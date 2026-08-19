package workflows

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"stuudio/internal/assessments"
	"stuudio/internal/assets"
	"stuudio/internal/auth"
	"stuudio/internal/clients"
	"stuudio/internal/i18n"
	"stuudio/internal/media"
	"stuudio/internal/reporter"
	"stuudio/internal/settings"
	"stuudio/internal/treatments"
	"stuudio/internal/web"
)

type Service struct {
	Pool  *sql.DB
	Auth  *auth.Service
	Media *media.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /projects", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /projects/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /projects", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /projects/{id}", svc.Auth.RequireUser(svc.handleDetail))
	mux.HandleFunc("POST /projects/{id}/stage", svc.Auth.RequireUser(svc.handleSetStage))
	mux.HandleFunc("POST /projects/{id}/finish", svc.Auth.RequireUser(svc.handleFinish))
	mux.HandleFunc("GET /projects/{id}/edit", svc.Auth.RequireUser(svc.handleEditForm))
	mux.HandleFunc("POST /projects/{id}/update", svc.Auth.RequireUser(svc.handleUpdate))
	mux.HandleFunc("POST /projects/{id}/unlink", svc.Auth.RequireUser(svc.handleUnlink))
	mux.HandleFunc("POST /projects/{id}/media", svc.Auth.RequireUser(svc.handleAddMedia))
	mux.HandleFunc("POST /projects/{id}/media/{refId}/unlink", svc.Auth.RequireUser(svc.handleUnlinkMedia))
}

func writeHTML(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(r.Context(), w)
}

func chromeFor(r *http.Request, user *auth.User, active string) web.Chrome {
	return web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), active)
}

func (svc *Service) handleList(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	rows, err := List(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stages, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierProjectStage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, ListPage(chromeFor(r, user, "/projects"), rows, stages))
}

func (svc *Service) handleNewForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	options, err := ListAssetOptions(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/projects"), options, conditions, r.URL.Query().Get("assetId")))
}

func (svc *Service) handleCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	assetID := r.FormValue("assetId")
	title := strings.TrimSpace(r.FormValue("title"))
	if assetID == "" || title == "" {
		http.Error(w, "Asset and title are required.", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	id, err := Create(ctx, svc.Pool, assetID, title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Optional "initial assessment" sub-block: keeps the old one-step asset-intake feel now that
	// Assessments require a Project — if the conservator filled in a condition description here,
	// record it as this Project's first Assessment right away.
	if description := strings.TrimSpace(r.FormValue("assessmentDescription")); description != "" {
		condition := r.FormValue("assessmentCondition")
		if condition == "" {
			condition = "other"
		}
		assessmentID, err := assessments.Create(ctx, svc.Pool, assessments.Input{
			ProjectID: id, Condition: condition, Description: description,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := svc.Media.UploadAllAndAttach(ctx, media.FilesFromForm(r, "photos"), user.ID, media.RefAssessment, assessmentID, "photo"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}

func (svc *Service) handleDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	project, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.NotFound(w, r)
		return
	}
	asset, err := assets.GetByID(ctx, svc.Pool, project.AssetID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var client *clients.Client
	if asset != nil {
		client, err = clients.GetByID(ctx, svc.Pool, asset.ClientID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	stages, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierProjectStage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assignedToName := ""
	if project.AssignedToUserID.Valid {
		assignedTo, err := auth.GetUserByID(ctx, svc.Pool, project.AssignedToUserID.String)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if assignedTo != nil {
			assignedToName = assignedTo.Name
		}
	}

	projectAssessments, err := assessments.ListByProject(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projectTreatments, err := treatments.ListByProject(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	locale := i18n.GetLocale(r)
	treatmentMethodLabels, err := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierTreatmentMethod, locale)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	treatmentMethods, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierTreatmentMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projectReports, err := reporter.ListByProject(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projectMedia, err := svc.Media.GetReferencedMedia(ctx, media.RefProject, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	damageMappingCount, err := media.CountRegionsForProject(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeHTML(w, r, DetailPage(chromeFor(r, user, "/projects"), *project, asset, client, stages, assignedToName,
		projectAssessments, conditions, projectTreatments, treatmentMethodLabels, treatmentMethods,
		projectReports, projectMedia, damageMappingCount))
}

func (svc *Service) handleSetStage(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	stage := r.FormValue("stage")
	if stage == "" {
		http.Error(w, "Stage is required.", http.StatusBadRequest)
		return
	}
	if err := SetStage(r.Context(), svc.Pool, id, stage); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// The kanban board's drag-and-drop updates the DOM itself and only needs a non-error
	// response; the detail page's stage-advance <select> is a normal form post that expects a
	// redirect back. Same distinction static/js/kanban.js's fetch() already relies on.
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}

// handleFinish is the "Finish project" action: it guarantees a Report exists before the Project
// is marked completed, auto-drafting one from the Project's own Assessments/Treatments/media if
// none exists yet (the user is free to have already created one manually — this only fills the
// gap, never duplicates).
func (svc *Service) handleFinish(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	project, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.NotFound(w, r)
		return
	}
	hasReport, err := reporter.ExistsForProject(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !hasReport {
		reportID, err := reporter.CreateAutoDraft(ctx, svc.Pool, id, project.Title, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := SetStage(ctx, svc.Pool, id, "completed"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/reports/"+reportID+"?draftedToFinish=1", http.StatusSeeOther)
		return
	}
	if err := SetStage(ctx, svc.Pool, id, "completed"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}

func (svc *Service) handleEditForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	project, err := GetByID(r.Context(), svc.Pool, r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, r, EditPage(chromeFor(r, user, "/projects"), *project))
}

func (svc *Service) handleUpdate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "Title is required.", http.StatusBadRequest)
		return
	}
	if err := UpdateTitle(r.Context(), svc.Pool, id, title); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}

// handleUnlink soft-deletes the project (see Unlink's doc comment) and redirects back to the
// asset it belonged to, matching the asset detail page's "unlink" action for this section.
func (svc *Service) handleUnlink(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	project, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.NotFound(w, r)
		return
	}
	if err := Unlink(ctx, svc.Pool, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+project.AssetID, http.StatusSeeOther)
}

// handleAddMedia attaches media directly to the Project itself (the project detail view's Media
// section's "+Add") — distinct from an Assessment/Treatment/Report's own photo uploads, which
// attach to that record instead. Mirrors internal/assets' handleAddMedia.
func (svc *Service) handleAddMedia(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if _, err := svc.Media.UploadAllAndAttach(r.Context(), media.FilesFromForm(r, "photos"), user.ID, media.RefProject, id, "photo"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}

// handleUnlinkMedia removes just the MediaReference join row (see media.Service.UnlinkReference's
// doc comment) — the Media itself is shared library content and is never touched by this.
func (svc *Service) handleUnlinkMedia(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	if err := svc.Media.UnlinkReference(r.Context(), r.PathValue("refId")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}
