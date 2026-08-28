package reporter

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/assessments"
	"studio/internal/auth"
	"studio/internal/media"
	"studio/internal/treatments"
	"studio/internal/web"
)

type Service struct {
	Pool  *sql.DB
	Auth  *auth.Service
	Media *media.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /reports", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /reports/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /reports", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /reports/{id}", svc.Auth.RequireUser(svc.handleDetail))
	mux.HandleFunc("POST /reports/{id}/sections", svc.Auth.RequireUser(svc.handleUpdateSections))
	mux.HandleFunc("POST /reports/{id}/layout", svc.Auth.RequireUser(svc.handleUpdateLayout))
	mux.HandleFunc("POST /reports/{id}/status", svc.Auth.RequireUser(svc.handleSetStatus))
	mux.HandleFunc("POST /reports/{id}/attachments", svc.Auth.RequireUser(svc.handleAddAttachments))
	mux.HandleFunc("POST /reports/{id}/media/{refId}/caption", svc.Auth.RequireUser(svc.handleSetCaption))
	mux.HandleFunc("POST /reports/{id}/gallery/reorder", svc.Auth.RequireUser(svc.handleReorderGallery))
	mux.HandleFunc("POST /reports/{id}/gallery/columns", svc.Auth.RequireUser(svc.handleSetGalleryColumns))
	mux.HandleFunc("POST /reports/{id}/unlink", svc.Auth.RequireUser(svc.handleUnlink))
}

func writeHTML(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(r.Context(), w)
}

func chromeFor(r *http.Request, user *auth.User, active string) web.Chrome {
	return web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), active)
}

func (svc *Service) handleList(w http.ResponseWriter, r *http.Request, user *auth.User) {
	rows, err := ListIncludingRemoved(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, ListPage(chromeFor(r, user, "/reports"), rows))
}

func (svc *Service) handleNewForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	projects, err := ListProjectOptions(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/reports"), projects, r.URL.Query().Get("projectId")))
}

func (svc *Service) handleCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	projectID := r.FormValue("projectId")
	title := strings.TrimSpace(r.FormValue("title"))
	if projectID == "" || title == "" {
		http.Error(w, "Project and title are required.", http.StatusBadRequest)
		return
	}
	sections, err := BuildSuggestedOutline(ctx, svc.Pool, projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := Create(ctx, svc.Pool, projectID, title, user.ID, sections)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports/"+id, http.StatusSeeOther)
}

func (svc *Service) handleDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	report, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if report == nil {
		http.NotFound(w, r)
		return
	}
	projectAssessments, err := assessments.ListByProject(ctx, svc.Pool, report.ProjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projectTreatments, err := treatments.ListByProject(ctx, svc.Pool, report.ProjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	gallery, err := BuildGallery(ctx, svc.Media, svc.Pool, report.ProjectID, report.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	infoPanel, err := BuildInfoPanel(ctx, svc.Pool, report.AssetID, report.ProjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, DetailPage(chromeFor(r, user, "/reports"), *report, projectAssessments, projectTreatments, gallery, infoPanel))
}

// handleSetCaption saves one gallery image's caption and its "stretch to column width" flag
// together, in the same request - the whole point of putting both in a single form/row (see
// internal/reporter/views.templ's Image gallery section) is one Save click for both.
func (svc *Service) handleSetCaption(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	refID := r.PathValue("refId")
	if err := media.SetCaption(r.Context(), svc.Pool, refID, strings.TrimSpace(r.FormValue("caption"))); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := SetGalleryItemStretch(r.Context(), svc.Pool, id, refID, r.FormValue("stretch") == "on"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports/"+id, http.StatusSeeOther)
}

func (svc *Service) handleUpdateSections(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	in := SectionsInput{
		Description:     strings.TrimSpace(r.FormValue("description")),
		Summary:         strings.TrimSpace(r.FormValue("summary")),
		Recommendations: strings.TrimSpace(r.FormValue("recommendations")),
	}
	id := r.PathValue("id")
	if err := UpdateSections(r.Context(), svc.Pool, id, in); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports/"+id, http.StatusSeeOther)
}

// handleUnlink soft-deletes the report (see Unlink's doc comment) and redirects back to the
// asset it belonged to, matching the asset detail page's "unlink" action for this section.
func (svc *Service) handleUnlink(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	report, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if report == nil {
		http.NotFound(w, r)
		return
	}
	if err := Unlink(ctx, svc.Pool, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+report.AssetID, http.StatusSeeOther)
}

func (svc *Service) handleUpdateLayout(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	in := LayoutInput{
		LayoutStyle:         r.FormValue("layoutStyle"),
		CoverMediaID:        r.FormValue("coverMediaId"),
		ShowCover:           r.FormValue("showCover") == "on",
		ShowDescription:     r.FormValue("showDescription") == "on",
		ShowSummary:         r.FormValue("showSummary") == "on",
		ShowRecommendations: r.FormValue("showRecommendations") == "on",
	}
	if in.LayoutStyle == "" {
		in.LayoutStyle = "standard"
	}
	id := r.PathValue("id")
	if err := UpdateLayout(r.Context(), svc.Pool, id, in); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports/"+id, http.StatusSeeOther)
}

// handleReorderGallery persists a drag-drop reorder of the report's own image gallery -
// orderedRefIds is a repeated form field, one MediaReference.id per image, in their new order
// (see static/js/report-gallery.js).
func (svc *Service) handleReorderGallery(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := ReorderGallery(r.Context(), svc.Pool, id, r.Form["refId"]); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (svc *Service) handleSetGalleryColumns(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	columns, err := strconv.Atoi(r.FormValue("columns"))
	if err != nil || (columns != 1 && columns != 2) {
		columns = 2
	}
	id := r.PathValue("id")
	if err := SetGalleryColumns(r.Context(), svc.Pool, id, columns); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports/"+id, http.StatusSeeOther)
}

func (svc *Service) handleSetStatus(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	status := r.FormValue("status")
	if status != "draft" && status != "final" {
		http.Error(w, "Invalid status.", http.StatusBadRequest)
		return
	}
	if err := SetStatus(r.Context(), svc.Pool, id, status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports/"+id, http.StatusSeeOther)
}

// handleAddAttachments uploads directly into the report's own image gallery (the old separate
// "Attachments" card is gone - see internal/reporter/views.templ's Image gallery section, which
// is what this now posts from). Image-only: the gallery never shows video (BuildGallery already
// filters it out on read), so a stray video in the upload is rejected here too rather than
// accepted and then silently invisible.
func (svc *Service) handleAddAttachments(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	files := media.FilesFromForm(r, "photos")
	images := files[:0]
	for _, f := range files {
		if strings.HasPrefix(f.MimeType, "image/") {
			images = append(images, f)
		}
	}
	if _, err := svc.Media.UploadAllAndAttachWithCaption(r.Context(), images, user.ID, media.RefReport, id, "gallery", r.FormValue("photosCaption")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports/"+id, http.StatusSeeOther)
}
