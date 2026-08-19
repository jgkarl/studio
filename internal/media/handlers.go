package media

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/a-h/templ"

	"stuudio/internal/auth"
	"stuudio/internal/settings"
	"stuudio/internal/web"
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
	mux.HandleFunc("POST /media/{id}/annotations", svc.Auth.RequireUser(svc.handleCreateAnnotation))
	mux.HandleFunc("POST /media/{id}/annotations/{regionId}/delete", svc.Auth.RequireUser(svc.handleDeleteAnnotation))
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

	chrome := chromeFor(r, user, "/media")
	var regions []AnnotationRegion
	var annotationTypes []AnnotationTypeOption
	if m.Kind == KindImage {
		regions, err = ListRegionsForMedia(ctx, svc.Pool, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		classifiers, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierAnnotationType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		annotationTypes = BuildAnnotationTypeOptions(classifiers, chrome.Locale)
	}
	writeHTML(w, r, MediaViewPage(chrome, *m, uploadedByName, reference, regions, annotationTypes))
}

// --- Pattern-layer annotation regions (session required) ---------------------------------------

func (svc *HandlerService) handleCreateAnnotation(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	typeID := r.FormValue("annotationTypeId")
	if typeID == "" {
		http.Error(w, "annotationTypeId is required", http.StatusBadRequest)
		return
	}

	if r.FormValue("shape") == "freehand" {
		if _, err := CreateFreehandRegion(r.Context(), svc.Pool, id, typeID, r.FormValue("points")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		x, xErr := strconv.ParseFloat(r.FormValue("xPct"), 64)
		y, yErr := strconv.ParseFloat(r.FormValue("yPct"), 64)
		width, wErr := strconv.ParseFloat(r.FormValue("widthPct"), 64)
		height, hErr := strconv.ParseFloat(r.FormValue("heightPct"), 64)
		if xErr != nil || yErr != nil || wErr != nil || hErr != nil {
			http.Error(w, "xPct/yPct/widthPct/heightPct must be numbers", http.StatusBadRequest)
			return
		}
		if _, err := CreateRegion(r.Context(), svc.Pool, id, typeID, x, y, width, height); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// The drag-to-draw UI (static/js/pattern-layer.js) posts via fetch and reloads the page
	// itself on success — a redirect response body would just be discarded. A plain form post
	// (no JS) still gets sent back to the media view, same convention as handleSetStage.
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/media/view/"+id, http.StatusSeeOther)
}

func (svc *HandlerService) handleDeleteAnnotation(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	if err := DeleteRegion(r.Context(), svc.Pool, r.PathValue("regionId")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/media/view/"+id, http.StatusSeeOther)
}

func userName(ctx context.Context, pool *sql.DB, userID string) (string, error) {
	u, err := auth.GetUserByID(ctx, pool, userID)
	if err != nil || u == nil {
		return "unknown", err
	}
	return u.Name, nil
}
