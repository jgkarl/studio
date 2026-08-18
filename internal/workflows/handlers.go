package workflows

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/assets"
	"studio/internal/auth"
	"studio/internal/clients"
	"studio/internal/settings"
	"studio/internal/web"
)

type Service struct {
	Pool *sql.DB
	Auth *auth.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /projects", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /projects/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /projects", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /projects/{id}", svc.Auth.RequireUser(svc.handleDetail))
	mux.HandleFunc("POST /projects/{id}/stage", svc.Auth.RequireUser(svc.handleSetStage))
	mux.HandleFunc("GET /projects/{id}/edit", svc.Auth.RequireUser(svc.handleEditForm))
	mux.HandleFunc("POST /projects/{id}/update", svc.Auth.RequireUser(svc.handleUpdate))
	mux.HandleFunc("POST /projects/{id}/unlink", svc.Auth.RequireUser(svc.handleUnlink))
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
	options, err := ListAssetOptions(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/projects"), options, r.URL.Query().Get("assetId")))
}

func (svc *Service) handleCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	assetID := r.FormValue("assetId")
	title := strings.TrimSpace(r.FormValue("title"))
	if assetID == "" || title == "" {
		http.Error(w, "Asset and title are required.", http.StatusBadRequest)
		return
	}
	id, err := Create(r.Context(), svc.Pool, assetID, title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	writeHTML(w, r, DetailPage(chromeFor(r, user, "/projects"), *project, asset, client, stages, assignedToName))
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
