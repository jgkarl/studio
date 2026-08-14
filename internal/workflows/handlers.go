package workflows

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"studio/internal/assets"
	"studio/internal/auth"
	"studio/internal/clients"
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
	mux.HandleFunc("GET /workflows", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /workflows/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /workflows", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /workflows/{id}", svc.Auth.RequireUser(svc.handleDetail))
	mux.HandleFunc("POST /workflows/{id}/activities", svc.Auth.RequireUser(svc.handleLogActivity))
	mux.HandleFunc("POST /workflows/{id}/advance", svc.Auth.RequireUser(svc.handleAdvanceStage))
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
	writeHTML(w, r, ListPage(chromeFor(r, user, "/workflows"), rows))
}

func (svc *Service) handleNewForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	options, err := ListAssetOptions(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/workflows"), options, r.URL.Query().Get("assetId")))
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
	http.Redirect(w, r, "/workflows/"+id, http.StatusSeeOther)
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
	activities, err := ListActivities(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	activityMedia := map[string][]media.ReferenceWithMedia{}
	for _, a := range activities {
		refs, err := svc.Media.GetReferencedMedia(ctx, media.RefActivity, a.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		activityMedia[a.ID] = refs
	}
	activityTypes, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierActivityType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeHTML(w, r, DetailPage(chromeFor(r, user, "/workflows"), *project, asset, client, activities, activityMedia, activityTypes, conditions))
}

func (svc *Service) handleLogActivity(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	projectID := r.PathValue("id")

	activityTypeID := r.FormValue("activityTypeId")
	description := strings.TrimSpace(r.FormValue("description"))
	if activityTypeID == "" || description == "" {
		http.Error(w, "Activity type and description are required.", http.StatusBadRequest)
		return
	}

	activityType, err := settings.GetClassifierByID(ctx, svc.Pool, activityTypeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if activityType == nil {
		http.Error(w, "Unknown activity type.", http.StatusBadRequest)
		return
	}
	var typeData struct {
		IsStateFixation bool `json:"isStateFixation"`
	}
	if activityType.Data.Valid {
		_ = json.Unmarshal([]byte(activityType.Data.String), &typeData)
	}

	startedAt := time.Now()
	if raw := r.FormValue("startedAt"); raw != "" {
		if t, err := time.Parse("2006-01-02T15:04", raw); err == nil {
			startedAt = t
		}
	}
	var durationMinutes any
	if raw := strings.TrimSpace(r.FormValue("durationMinutes")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			durationMinutes = n
		}
	}
	var materialsUsed any
	if raw := strings.TrimSpace(r.FormValue("materialsUsed")); raw != "" {
		encoded, err := json.Marshal(raw)
		if err == nil {
			materialsUsed = string(encoded)
		}
	}

	activityID, err := LogActivity(ctx, svc.Pool, projectID, LogActivityInput{
		ActivityTypeID: activityTypeID, UserID: user.ID, Description: description,
		StartedAt: startedAt, DurationMinutes: durationMinutes, MaterialsUsed: materialsUsed,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Same file list attached twice on purpose when this activity also fixates state (below) -
	// once against the Activity itself, once against the AssetState snapshot - matching the
	// original app's behavior exactly (two independent Media rows, not a shared attachment).
	if _, err := svc.Media.UploadAllAndAttach(ctx, media.FilesFromForm(r, "photos"), user.ID, media.RefActivity, activityID, "photo"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Activity types marked isStateFixation (in Classifier.data) prompt a linked AssetState.
	if typeData.IsStateFixation {
		project, err := GetByID(ctx, svc.Pool, projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if project == nil {
			http.Error(w, "Workflow not found.", http.StatusNotFound)
			return
		}
		condition := r.FormValue("condition")
		if condition == "" {
			condition = "stable"
		}
		stateID, err := assets.RecordState(ctx, svc.Pool, project.AssetID, condition, description, &projectID, &activityID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := svc.Media.UploadAllAndAttach(ctx, media.FilesFromForm(r, "photos"), user.ID, media.RefAssetState, stateID, "photo"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/workflows/"+projectID, http.StatusSeeOther)
}

func (svc *Service) handleAdvanceStage(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	nextStage := Stage(r.FormValue("nextStage"))

	project, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.NotFound(w, r)
		return
	}
	allowed := StageTransitions[project.Stage]
	ok := false
	for _, s := range allowed {
		if s == nextStage {
			ok = true
			break
		}
	}
	if !ok {
		http.Error(w, "Cannot move from \""+string(project.Stage)+"\" to \""+string(nextStage)+"\".", http.StatusBadRequest)
		return
	}

	if err := AdvanceStage(ctx, svc.Pool, id, nextStage); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/workflows/"+id, http.StatusSeeOther)
}
