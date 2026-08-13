package assets

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

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
	mux.HandleFunc("GET /assets", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /assets/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /assets", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /assets/{id}", svc.Auth.RequireUser(svc.handleDetail))
	mux.HandleFunc("POST /assets/{id}/update", svc.Auth.RequireUser(svc.handleUpdate))
	mux.HandleFunc("POST /assets/{id}/states", svc.Auth.RequireUser(svc.handleRecordState))
	mux.HandleFunc("POST /assets/{id}/materials", svc.Auth.RequireUser(svc.handleAddMaterial))
	mux.HandleFunc("POST /assets/{id}/materials/{materialId}/delete", svc.Auth.RequireUser(svc.handleRemoveMaterial))
	mux.HandleFunc("POST /assets/{id}/tags", svc.Auth.RequireUser(svc.handleAddTag))
	mux.HandleFunc("POST /assets/{id}/tags/{assignmentId}/delete", svc.Auth.RequireUser(svc.handleRemoveTag))
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
	materials, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierMaterial)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/assets"), allClients, assetTypes, materials, r.URL.Query().Get("clientId")))
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
	if err := r.ParseForm(); err != nil {
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

	if err := AttachMaterialsOnCreate(ctx, svc.Pool, id, r.Form["materialIds"]); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tags := strings.TrimSpace(r.FormValue("tags")); tags != "" {
		if err := settings.SetTags(ctx, svc.Pool, "Asset", id, tags); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if intake := strings.TrimSpace(r.FormValue("intakeDescription")); intake != "" {
		if _, err := RecordState(ctx, svc.Pool, id, "intake", intake, nil, nil); err != nil {
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
	materials, err := ListMaterials(ctx, svc.Pool, id)
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
	tagAssignments, err := settings.GetTagAssignments(ctx, svc.Pool, "Asset", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conditions, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierConditionState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allMaterials, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierMaterial)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	attached := map[string]bool{}
	for _, m := range materials {
		attached[m.MaterialID] = true
	}
	var available []settings.Classifier
	for _, m := range allMaterials {
		if !attached[m.ID] {
			available = append(available, m)
		}
	}
	conditionByCode := map[string]settings.Classifier{}
	for _, c := range conditions {
		conditionByCode[c.Code] = c
	}

	writeHTML(w, r, DetailPage(chromeFor(r, user, "/assets"), *asset, client, assetType, materials, available, states,
		projects, tagAssignments, conditions, conditionByCode))
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	condition := r.FormValue("condition")
	if condition == "" {
		condition = "other"
	}
	description := strings.TrimSpace(r.FormValue("description"))
	if _, err := RecordState(r.Context(), svc.Pool, id, condition, description, nil, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+id, http.StatusSeeOther)
}

func (svc *Service) handleAddMaterial(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	materialID := r.FormValue("materialId")
	if materialID == "" {
		http.Error(w, "Select a material.", http.StatusBadRequest)
		return
	}
	if err := AddMaterial(r.Context(), svc.Pool, id, materialID, r.FormValue("role")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+id, http.StatusSeeOther)
}

func (svc *Service) handleRemoveMaterial(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	if err := RemoveMaterial(r.Context(), svc.Pool, r.PathValue("materialId")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+id, http.StatusSeeOther)
}

func (svc *Service) handleAddTag(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := settings.AddTagToEntity(r.Context(), svc.Pool, "Asset", id, r.FormValue("name")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/assets/"+id, http.StatusSeeOther)
}

func (svc *Service) handleRemoveTag(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	if err := settings.RemoveTagAssignment(r.Context(), svc.Pool, r.PathValue("assignmentId")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/assets/"+id, http.StatusSeeOther)
}
