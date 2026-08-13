package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/auth"
	"studio/internal/web"
)

type HandlerService struct {
	*Service
	Auth *auth.Service
}

// Mount registers the media serving routes (public - no session required, same as the original
// app: media IDs are unguessable UUIDs, and IIIF services are conventionally public/CORS-open)
// plus the Album module (session required).
func Mount(mux *http.ServeMux, svc *HandlerService) {
	mux.HandleFunc("GET /api/media/{id}", svc.handleServeMedia)
	mux.HandleFunc("GET /api/iiif/{id}/info.json", svc.handleInfoJSON)
	mux.HandleFunc("GET /api/iiif/{id}/{region}/{size}/{rotation}/{qualityFormat}", svc.handleIIIFTransform)

	mux.HandleFunc("GET /album", svc.Auth.RequireUser(svc.handleAlbum))
	mux.HandleFunc("GET /album/view/{mediaId}", svc.Auth.RequireUser(svc.handleMediaView))
	mux.HandleFunc("POST /album/view/{mediaId}/edit", svc.Auth.RequireUser(svc.handleSaveEditedMedia))
}

func writeHTML(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(r.Context(), w)
}

func chromeFor(r *http.Request, user *auth.User, active string) web.Chrome {
	return web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), active)
}

// --- Serving routes (public) -----------------------------------------------------------------

func (svc *HandlerService) handleServeMedia(w http.ResponseWriter, r *http.Request) {
	variant := "web"
	if r.URL.Query().Get("variant") == "original" {
		variant = "original"
	}
	file, err := svc.ReadMediaFile(r.Context(), r.PathValue("id"), variant)
	if err != nil || file == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	_, _ = w.Write(file.Data)
}

func (svc *HandlerService) handleInfoJSON(w http.ResponseWriter, r *http.Request) {
	m, err := GetByID(r.Context(), svc.Pool, r.PathValue("id"))
	if err != nil || m == nil || m.Kind != KindImage || !m.Width.Valid || !m.Height.Valid {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	origin := requestOrigin(r)
	body := BuildInfoJSON(m.ID, int(m.Width.Int64), int(m.Height.Int64), origin)

	w.Header().Set("Content-Type", `application/ld+json;profile="http://iiif.io/api/image/3/context.json"`)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(body)
}

func requestOrigin(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
		if r.TLS != nil {
			proto = "https"
		}
	}
	return proto + "://" + r.Host
}

// handleIIIFTransform implements the canonical IIIF Image API request URL:
// /{id}/{region}/{size}/{rotation}/{quality}.{format}. qualityFormat is one route segment (a
// single {name} can't itself split on a dot) - split on the *last* dot, since quality names never
// contain one.
func (svc *HandlerService) handleIIIFTransform(w http.ResponseWriter, r *http.Request) {
	qualityFormat := r.PathValue("qualityFormat")
	dot := strings.LastIndex(qualityFormat, ".")
	if dot == -1 {
		http.Error(w, "Malformed quality.format segment", http.StatusBadRequest)
		return
	}
	params := IIIFParams{
		Region:   r.PathValue("region"),
		Size:     r.PathValue("size"),
		Rotation: r.PathValue("rotation"),
		Quality:  qualityFormat[:dot],
		Format:   qualityFormat[dot+1:],
	}
	buf, contentType, err := svc.TransformImage(r.Context(), r.PathValue("id"), params)
	if err != nil {
		if iiifErr, ok := err.(*IIIFError); ok {
			http.Error(w, iiifErr.Message, iiifErr.Status)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(buf)
}

// --- Album (session required) ------------------------------------------------------------------

func (svc *HandlerService) handleAlbum(w http.ResponseWriter, r *http.Request, user *auth.User) {
	items, err := GetAllMediaWithContext(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, AlbumPage(chromeFor(r, user, "/album"), items))
}

func (svc *HandlerService) handleMediaView(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("mediaId")
	m, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if m == nil {
		http.NotFound(w, r)
		return
	}
	uploadedByName, err := userName(ctx, svc.Pool, m.UploadedByID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reference, err := GetFirstReference(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	edits, err := ListEditsOf(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, MediaViewPage(chromeFor(r, user, "/album"), *m, uploadedByName, reference, edits))
}

func userName(ctx context.Context, pool *sql.DB, userID string) (string, error) {
	u, err := auth.GetUserByID(ctx, pool, userID)
	if err != nil || u == nil {
		return "unknown", err
	}
	return u.Name, nil
}

func (svc *HandlerService) handleSaveEditedMedia(w http.ResponseWriter, r *http.Request, user *auth.User) {
	sourceID := r.PathValue("mediaId")
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No edited image provided.", http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	m, err := svc.SaveEditedImage(r.Context(), sourceID, data, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/album/view/"+m.ID, http.StatusSeeOther)
}
