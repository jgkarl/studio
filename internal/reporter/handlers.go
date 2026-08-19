package reporter

import (
	"database/sql"
	"net/http"
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
	rows, err := List(r.Context(), svc.Pool)
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
	refs, err := svc.Media.GetReferencedMedia(ctx, media.RefReport, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	gallery, err := BuildGallery(ctx, svc.Media, svc.Pool, report.ProjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	infoPanel, err := BuildInfoPanel(ctx, svc.Pool, report.AssetID, report.ProjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, DetailPage(chromeFor(r, user, "/reports"), *report, refs, projectAssessments, projectTreatments, gallery, infoPanel))
}

func (svc *Service) handleSetCaption(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := media.SetCaption(r.Context(), svc.Pool, r.PathValue("refId"), strings.TrimSpace(r.FormValue("caption"))); err != nil {
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
		Summary:            strings.TrimSpace(r.FormValue("summary")),
		ConditionFindings:  strings.TrimSpace(r.FormValue("conditionFindings")),
		TreatmentPerformed: strings.TrimSpace(r.FormValue("treatmentPerformed")),
		MaterialsUsed:      strings.TrimSpace(r.FormValue("materialsUsed")),
		Recommendations:    strings.TrimSpace(r.FormValue("recommendations")),
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
		ShowSummary:         r.FormValue("showSummary") == "on",
		ShowCondition:       r.FormValue("showCondition") == "on",
		ShowTreatment:       r.FormValue("showTreatment") == "on",
		ShowMaterials:       r.FormValue("showMaterials") == "on",
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

func (svc *Service) handleAddAttachments(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if _, err := svc.Media.UploadAllAndAttach(r.Context(), media.FilesFromForm(r, "photos"), user.ID, media.RefReport, id, "attachment"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports/"+id, http.StatusSeeOther)
}
