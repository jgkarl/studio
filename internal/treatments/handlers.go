package treatments

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

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
	mux.HandleFunc("GET /treatments", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /treatments/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /treatments", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /treatments/{id}", svc.Auth.RequireUser(svc.handleDetail))
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
	methodLabels, err := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierTreatmentMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, ListPage(chromeFor(r, user, "/treatments"), rows, methodLabels))
}

func (svc *Service) handleNewForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	options, err := ListAssetOptions(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	methods, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierTreatmentMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/treatments"), options, methods, r.URL.Query().Get("assetId")))
}

func (svc *Service) handleCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	assetID := r.FormValue("assetId")
	method := r.FormValue("method")
	title := strings.TrimSpace(r.FormValue("title"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	if assetID == "" || method == "" || title == "" || notes == "" {
		http.Error(w, "Asset, method, title, and notes are required.", http.StatusBadRequest)
		return
	}
	performedAt := time.Now()
	if raw := r.FormValue("performedAt"); raw != "" {
		if t, err := time.Parse("2006-01-02T15:04", raw); err == nil {
			performedAt = t
		}
	}

	ctx := r.Context()
	id, err := Create(ctx, svc.Pool, Input{
		AssetID: assetID, Method: method, Title: title, Notes: notes,
		PerformedByUserID: user.ID, PerformedAt: performedAt,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := svc.Media.UploadAllAndAttach(ctx, media.FilesFromForm(r, "photos"), user.ID, media.RefTreatment, id, "photo"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/treatments/"+id, http.StatusSeeOther)
}

func (svc *Service) handleDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	treatment, err := GetDetailByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if treatment == nil {
		http.NotFound(w, r)
		return
	}
	methodLabels, err := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierTreatmentMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	performedByName := ""
	if treatment.PerformedByUserID.Valid {
		performedBy, err := auth.GetUserByID(ctx, svc.Pool, treatment.PerformedByUserID.String)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if performedBy != nil {
			performedByName = performedBy.Name
		}
	}
	photos, err := svc.Media.GetReferencedMedia(ctx, media.RefTreatment, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeHTML(w, r, DetailPage(chromeFor(r, user, "/treatments"), *treatment, methodLabels, performedByName, photos))
}
