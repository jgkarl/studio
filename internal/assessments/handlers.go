package assessments

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/auth"
	"studio/internal/media"
	"studio/internal/settings"
	"studio/internal/web"
)

type Service struct {
	Pool  *sql.DB
	Auth  *auth.Service
	Media *media.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /assessments", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /assessments/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /assessments", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /assessments/{id}", svc.Auth.RequireUser(svc.handleDetail))
	mux.HandleFunc("GET /assessments/{id}/edit", svc.Auth.RequireUser(svc.handleEditForm))
	mux.HandleFunc("POST /assessments/{id}/update", svc.Auth.RequireUser(svc.handleUpdate))
	mux.HandleFunc("POST /assessments/{id}/unlink", svc.Auth.RequireUser(svc.handleUnlink))
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
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, ListPage(chromeFor(r, user, "/assessments"), rows, conditions))
}

func (svc *Service) handleNewForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	projects, err := ListProjectOptions(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/assessments"), projects, conditions, r.URL.Query().Get("projectId")))
}

func (svc *Service) handleCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	projectID := r.FormValue("projectId")
	if projectID == "" {
		http.Error(w, "Project is required.", http.StatusBadRequest)
		return
	}
	condition := r.FormValue("condition")
	if condition == "" {
		condition = "other"
	}
	description := strings.TrimSpace(r.FormValue("description"))
	if description == "" {
		http.Error(w, "Description is required.", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	id, err := Create(ctx, svc.Pool, Input{ProjectID: projectID, Condition: condition, Description: description})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := svc.Media.UploadAllAndAttach(ctx, media.FilesFromForm(r, "photos"), user.ID, media.RefAssessment, id, "photo"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assessments/"+id, http.StatusSeeOther)
}

func (svc *Service) handleDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	assessment, err := GetDetailByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if assessment == nil {
		http.NotFound(w, r)
		return
	}
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	photos, err := svc.Media.GetReferencedMedia(ctx, media.RefAssessment, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, DetailPage(chromeFor(r, user, "/assessments"), *assessment, conditions, photos))
}

func (svc *Service) handleEditForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	assessment, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if assessment == nil {
		http.NotFound(w, r)
		return
	}
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, EditPage(chromeFor(r, user, "/assessments"), *assessment, conditions))
}

func (svc *Service) handleUpdate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := Update(r.Context(), svc.Pool, id, UpdateInput{
		Condition:   r.FormValue("condition"),
		Description: strings.TrimSpace(r.FormValue("description")),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assessments/"+id, http.StatusSeeOther)
}

// handleUnlink soft-deletes the assessment (see Unlink's doc comment) and redirects back to the
// asset it belonged to, matching the asset detail page's "unlink" action for this section.
func (svc *Service) handleUnlink(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	assessment, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if assessment == nil {
		http.NotFound(w, r)
		return
	}
	if err := Unlink(ctx, svc.Pool, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+assessment.AssetID, http.StatusSeeOther)
}
