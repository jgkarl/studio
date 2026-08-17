package media

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/a-h/templ"

	"studio/internal/auth"
	"studio/internal/web"
)

type HandlerService struct {
	*Service
	Auth *auth.Service
}

// Mount registers the media serving route (public - no session required: media IDs are
// unguessable UUIDs) plus the Media module (session required).
func Mount(mux *http.ServeMux, svc *HandlerService) {
	mux.HandleFunc("GET /api/media/{id}", svc.handleServeMedia)

	mux.HandleFunc("GET /media", svc.Auth.RequireUser(svc.handleAlbum))
	mux.HandleFunc("GET /media/view/{mediaId}", svc.Auth.RequireUser(svc.handleMediaView))
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

// --- Media (session required) ------------------------------------------------------------------

func (svc *HandlerService) handleAlbum(w http.ResponseWriter, r *http.Request, user *auth.User) {
	items, err := GetAllMediaWithContext(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, AlbumPage(chromeFor(r, user, "/media"), items))
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
	writeHTML(w, r, MediaViewPage(chromeFor(r, user, "/media"), *m, uploadedByName, reference))
}

func userName(ctx context.Context, pool *sql.DB, userID string) (string, error) {
	u, err := auth.GetUserByID(ctx, pool, userID)
	if err != nil || u == nil {
		return "unknown", err
	}
	return u.Name, nil
}
