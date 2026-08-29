package media

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/a-h/templ"

	"studio/internal/auth"
	"studio/internal/i18n"
	"studio/internal/settings"
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
	mux.HandleFunc("GET /media/edit/{mediaId}", svc.Auth.RequireUser(svc.handleMediaEdit))
	mux.HandleFunc("POST /media/{id}/annotations", svc.Auth.RequireUser(svc.handleCreateAnnotation))
	mux.HandleFunc("POST /media/{id}/annotations/{regionId}/delete", svc.Auth.RequireUser(svc.handleDeleteAnnotation))
	mux.HandleFunc("POST /media/{id}/description", svc.Auth.RequireUser(svc.handleUpdateDescription))
	mux.HandleFunc("POST /media/{id}/annotated-versions", svc.Auth.RequireUser(svc.handleCreateAnnotatedVersion))
	mux.HandleFunc("POST /media/{id}/bake", svc.Auth.RequireUser(svc.handleBakeAnnotatedVersion))
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
	id := r.PathValue("id")
	if r.URL.Query().Get("variant") == "annotated" {
		png, mimeType, err := svc.RenderAnnotatedImage(r.Context(), id, i18n.GetLocale(r))
		if err == nil && png != nil {
			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Content-Disposition", `attachment; filename="`+id+`-annotated.png"`)
			_, _ = w.Write(png)
			return
		}
		// No regions (or rendering failed) — fall through to the plain web variant below rather
		// than 404ing a media item that just has nothing to annotate.
	}

	variant := "web"
	if r.URL.Query().Get("variant") == "original" {
		variant = "original"
	}
	file, err := svc.ReadMediaFile(r.Context(), id, variant)
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
	var source *Media
	var derivedVersions []Media
	if m.Kind == KindImage {
		classifiers, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierAnnotationType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		annotationTypes = BuildAnnotationTypeOptions(classifiers, chrome.Locale)

		if m.IsAnnotatedVersion() {
			regions, err = ListRegionsForMedia(ctx, svc.Pool, id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			source, err = GetByID(ctx, svc.Pool, m.EditedFromID.String)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			derivedVersions, err = ListDerivedVersions(ctx, svc.Pool, id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	writeHTML(w, r, MediaViewPage(chrome, *m, uploadedByName, reference, regions, annotationTypes, source, derivedVersions))
}

// handleMediaEdit serves the pattern-layer annotation editor (MediaEditPage) at its own URL,
// /media/edit/{mediaId} - only meaningful for an annotated version (only those have their own
// region set to draw/edit); visiting it for a true original (nothing to annotate directly - see
// annotatedVersionsCard/CreateAnnotatedVersion) redirects back to that media's view page instead
// of 404ing, since it's a plausible bookmarked/back-button URL rather than a broken link.
func (svc *HandlerService) handleMediaEdit(w http.ResponseWriter, r *http.Request, user *auth.User) {
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
	if m.Kind != KindImage || !m.IsAnnotatedVersion() {
		http.Redirect(w, r, "/media/view/"+id, http.StatusSeeOther)
		return
	}

	chrome := chromeFor(r, user, "/media")
	classifiers, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierAnnotationType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	annotationTypes := BuildAnnotationTypeOptions(classifiers, chrome.Locale)
	regions, err := ListRegionsForMedia(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	source, err := GetByID(ctx, svc.Pool, m.EditedFromID.String)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, MediaEditPage(chrome, *m, regions, annotationTypes, source))
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
	// The drag-to-draw UI (static/js/media-editor.js) posts via fetch and reloads the page
	// itself on success — a redirect response body would just be discarded. A plain form post
	// (no JS) still gets sent back to the editor page, same convention as handleSetStage.
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/media/edit/"+id, http.StatusSeeOther)
}

// handleUpdateDescription saves the editor's whole-image note (see Media.Description) — same
// fetch-or-plain-form-post convention as handleCreateAnnotation.
func (svc *HandlerService) handleUpdateDescription(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if err := UpdateDescription(r.Context(), svc.Pool, id, r.FormValue("description")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/media/edit/"+id, http.StatusSeeOther)
}

// handleCreateAnnotatedVersion starts a new annotation session on a true original image (see
// CreateAnnotatedVersion) and redirects straight into that new version's own editor page, where
// its regions actually get drawn. There is no separate "continue" action - continuing an existing
// version is just opening its own editor page (its regions already belong to it), this route
// always starts a fresh one.
func (svc *HandlerService) handleCreateAnnotatedVersion(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	original, err := GetByID(r.Context(), svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if original == nil || original.Kind != KindImage {
		http.NotFound(w, r)
		return
	}
	version, err := svc.CreateAnnotatedVersion(r.Context(), *original, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/media/edit/"+version.ID, http.StatusSeeOther)
}

// handleBakeAnnotatedVersion is called by static/js/media-editor.js's Save button - flattens the
// current region set into a real file (BakeAnnotatedVersion) and responds 204, same fetch
// convention as handleCreateAnnotation.
func (svc *HandlerService) handleBakeAnnotatedVersion(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	target, err := GetByID(r.Context(), svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if target == nil || !target.IsAnnotatedVersion() {
		http.Error(w, "not an annotated version", http.StatusBadRequest)
		return
	}
	if err := svc.BakeAnnotatedVersion(r.Context(), *target, i18n.GetLocale(r)); err != nil {
		slog.ErrorContext(r.Context(), "baking annotated version", "err", err, "media_id", id, "category", "media", "event", "bake_failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (svc *HandlerService) handleDeleteAnnotation(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	if err := DeleteRegion(r.Context(), svc.Pool, r.PathValue("regionId")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/media/edit/"+id, http.StatusSeeOther)
}

func userName(ctx context.Context, pool *sql.DB, userID string) (string, error) {
	u, err := auth.GetUserByID(ctx, pool, userID)
	if err != nil || u == nil {
		return "unknown", err
	}
	return u.Name, nil
}
