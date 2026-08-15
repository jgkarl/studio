package clients

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/auth"
	"studio/internal/settings"
	"studio/internal/web"
)

type Service struct {
	Pool *sql.DB
	Auth *auth.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /clients", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /clients/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /clients", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /clients/{id}", svc.Auth.RequireUser(svc.handleDetail))
	// /clients/edit/{id}, not /clients/{id}/edit - the latter is ambiguous with commerce's
	// /clients/quotes/{id} and /clients/orders/{id} under Go 1.22's ServeMux (same segment
	// count, wildcard in a different position - it refuses to register and panics at startup).
	mux.HandleFunc("GET /clients/edit/{id}", svc.Auth.RequireUser(svc.handleEditForm))
	mux.HandleFunc("POST /clients/{id}/update", svc.Auth.RequireUser(svc.handleUpdate))
	mux.HandleFunc("POST /clients/{id}/tags", svc.Auth.RequireUser(svc.handleAddTag))
	mux.HandleFunc("POST /clients/{id}/tags/{assignmentId}/delete", svc.Auth.RequireUser(svc.handleRemoveTag))
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
	typeLabels, err := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierClientType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, ListPage(chromeFor(r, user, "/clients"), rows, typeLabels))
}

func (svc *Service) handleNewForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	clientTypes, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierClientType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	contactMethods, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierContactMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/clients"), clientTypes, contactMethods))
}

func formInput(r *http.Request) Input {
	return Input{
		Type:                   strings.TrimSpace(r.FormValue("type")),
		Name:                   strings.TrimSpace(r.FormValue("name")),
		Email:                  strings.TrimSpace(r.FormValue("email")),
		Phone:                  strings.TrimSpace(r.FormValue("phone")),
		Address:                strings.TrimSpace(r.FormValue("address")),
		City:                   strings.TrimSpace(r.FormValue("city")),
		PostalCode:             strings.TrimSpace(r.FormValue("postalCode")),
		Country:                strings.TrimSpace(r.FormValue("country")),
		Notes:                  strings.TrimSpace(r.FormValue("notes")),
		OrganizationName:       strings.TrimSpace(r.FormValue("organizationName")),
		ContactPerson:          strings.TrimSpace(r.FormValue("contactPerson")),
		TaxID:                  strings.TrimSpace(r.FormValue("taxId")),
		PreferredContactMethod: strings.TrimSpace(r.FormValue("preferredContactMethod")),
		ReferralSource:         strings.TrimSpace(r.FormValue("referralSource")),
	}
}

func (svc *Service) handleCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	in := formInput(r)
	if in.Name == "" {
		http.Error(w, "Name is required.", http.StatusBadRequest)
		return
	}
	if in.Type == "" {
		in.Type = "individual"
	}
	id, err := Create(r.Context(), svc.Pool, in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/"+id, http.StatusSeeOther)
}

func (svc *Service) handleDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	client, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if client == nil {
		http.NotFound(w, r)
		return
	}

	assets, err := ListAssetsForClient(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	quotes, err := ListQuotesForClient(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	orders, err := ListOrdersForClient(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	clientTypes, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierClientType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	contactMethods, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierContactMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	typeLabel, err := settings.GetClassifierLabel(ctx, svc.Pool, settings.ClassifierClientType, client.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tagAssignments, err := settings.GetTagAssignments(ctx, svc.Pool, "Client", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeHTML(w, r, DetailPage(chromeFor(r, user, "/clients"), *client, typeLabel, assets, quotes, orders, clientTypes, contactMethods, tagAssignments))
}

func (svc *Service) handleEditForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	client, err := GetByID(ctx, svc.Pool, r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if client == nil {
		http.NotFound(w, r)
		return
	}
	clientTypes, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierClientType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	contactMethods, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierContactMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, EditPage(chromeFor(r, user, "/clients"), *client, clientTypes, contactMethods))
}

func (svc *Service) handleUpdate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := Update(r.Context(), svc.Pool, id, formInput(r)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/"+id, http.StatusSeeOther)
}

func (svc *Service) handleAddTag(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := settings.AddTagToEntity(r.Context(), svc.Pool, "Client", id, r.FormValue("name")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/clients/"+id, http.StatusSeeOther)
}

func (svc *Service) handleRemoveTag(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	if err := settings.RemoveTagAssignment(r.Context(), svc.Pool, r.PathValue("assignmentId")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/"+id, http.StatusSeeOther)
}
