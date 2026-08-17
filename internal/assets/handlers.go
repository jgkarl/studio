package assets

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/auth"
	"studio/internal/clients"
	"studio/internal/media"
	"studio/internal/reporter"
	"studio/internal/settings"
	"studio/internal/treatments"
	"studio/internal/web"
)

type Service struct {
	Pool  *sql.DB
	Auth  *auth.Service
	Media *media.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /assets", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /assets/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /assets", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /assets/{id}", svc.Auth.RequireUser(svc.handleDetail))
	mux.HandleFunc("GET /assets/{id}/edit", svc.Auth.RequireUser(svc.handleEditForm))
	mux.HandleFunc("POST /assets/{id}/update", svc.Auth.RequireUser(svc.handleUpdate))
	mux.HandleFunc("POST /assets/{id}/states", svc.Auth.RequireUser(svc.handleRecordState))
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
	writeHTML(w, r, ListPage(chromeFor(r, user, "/assets"), rows))
}

func (svc *Service) handleNewForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	allClients, err := clients.ListAllSortedByName(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assetTypes, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierAssetType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/assets"), allClients, assetTypes, r.URL.Query().Get("clientId")))
}

func formInput(r *http.Request) Input {
	acquisitionDate := any(nil)
	if v := strings.TrimSpace(r.FormValue("acquisitionDate")); v != "" {
		acquisitionDate = v
	}
	var estimatedValue any
	if v := strings.TrimSpace(r.FormValue("estimatedValue")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			estimatedValue = f
		}
	}
	var isInsured any
	switch r.FormValue("isInsured") {
	case "true":
		isInsured = true
	case "false":
		isInsured = false
	}
	return Input{
		Title:            strings.TrimSpace(r.FormValue("title")),
		Artist:           strings.TrimSpace(r.FormValue("artist")),
		CreationPeriod:   strings.TrimSpace(r.FormValue("creationPeriod")),
		Dimensions:       strings.TrimSpace(r.FormValue("dimensions")),
		Description:      strings.TrimSpace(r.FormValue("description")),
		Medium:           strings.TrimSpace(r.FormValue("medium")),
		SignatureMarks:   strings.TrimSpace(r.FormValue("signatureMarks")),
		Weight:           strings.TrimSpace(r.FormValue("weight")),
		Provenance:       strings.TrimSpace(r.FormValue("provenance")),
		AcquisitionDate:  acquisitionDate,
		EstimatedValue:   estimatedValue,
		IsInsured:        isInsured,
		LocationInStudio: strings.TrimSpace(r.FormValue("locationInStudio")),
	}
}

func (svc *Service) handleCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	clientID := r.FormValue("clientId")
	assetTypeID := r.FormValue("assetTypeId")
	referenceCode := strings.TrimSpace(r.FormValue("referenceCode"))
	if clientID == "" || assetTypeID == "" || referenceCode == "" {
		http.Error(w, "Client, asset type, and reference code are required.", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	id, err := Create(ctx, svc.Pool, clientID, assetTypeID, referenceCode, formInput(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if intake := strings.TrimSpace(r.FormValue("intakeDescription")); intake != "" {
		stateID, err := RecordState(ctx, svc.Pool, id, "intake", intake, nil, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := svc.Media.UploadAllAndAttach(ctx, media.FilesFromForm(r, "photos"), user.ID, media.RefAssetState, stateID, "photo"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/assets/"+id, http.StatusSeeOther)
}

func (svc *Service) handleDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	asset, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if asset == nil {
		http.NotFound(w, r)
		return
	}

	client, err := clients.GetByID(ctx, svc.Pool, asset.ClientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assetType, err := settings.GetClassifierByID(ctx, svc.Pool, asset.AssetTypeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	states, err := ListStates(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projects, err := ListProjectsForAsset(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conditionByCode := map[string]settings.Classifier{}
	for _, c := range conditions {
		conditionByCode[c.Code] = c
	}

	stateMedia := map[string][]media.ReferenceWithMedia{}
	for _, st := range states {
		refs, err := svc.Media.GetReferencedMedia(ctx, media.RefAssetState, st.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		stateMedia[st.ID] = refs
	}

	reports, err := reporter.ListByAsset(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assetTreatments, err := treatments.ListByAsset(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	treatmentMethodLabels, err := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierTreatmentMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeHTML(w, r, DetailPage(chromeFor(r, user, "/assets"), *asset, client, assetType, states,
		projects, conditions, conditionByCode, stateMedia, reports, assetTreatments, treatmentMethodLabels))
}

func (svc *Service) handleEditForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	asset, err := GetByID(r.Context(), svc.Pool, r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if asset == nil {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, r, EditPage(chromeFor(r, user, "/assets"), *asset))
}

func (svc *Service) handleUpdate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := UpdateProfile(r.Context(), svc.Pool, id, formInput(r)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+id, http.StatusSeeOther)
}

func (svc *Service) handleRecordState(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	condition := r.FormValue("condition")
	if condition == "" {
		condition = "other"
	}
	description := strings.TrimSpace(r.FormValue("description"))
	stateID, err := RecordState(ctx, svc.Pool, id, condition, description, nil, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := svc.Media.UploadAllAndAttach(ctx, media.FilesFromForm(r, "photos"), user.ID, media.RefAssetState, stateID, "photo"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+id, http.StatusSeeOther)
}
