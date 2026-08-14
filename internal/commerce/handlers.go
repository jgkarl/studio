package commerce

import (
	"database/sql"
	"net/http"
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
	mux.HandleFunc("GET /clients/quotes", svc.Auth.RequireUser(svc.handleQuotesList))
	mux.HandleFunc("GET /clients/quotes/new", svc.Auth.RequireUser(svc.handleNewQuoteForm))
	mux.HandleFunc("POST /clients/quotes", svc.Auth.RequireUser(svc.handleCreateQuote))
	mux.HandleFunc("GET /clients/quotes/{id}", svc.Auth.RequireUser(svc.handleQuoteDetail))
	mux.HandleFunc("POST /clients/quotes/{id}/status", svc.Auth.RequireUser(svc.handleSetQuoteStatus))
	mux.HandleFunc("POST /clients/quotes/{id}/accept", svc.Auth.RequireUser(svc.handleAcceptQuote))

	mux.HandleFunc("GET /clients/orders", svc.Auth.RequireUser(svc.handleOrdersKanban))
	mux.HandleFunc("GET /clients/orders/{id}", svc.Auth.RequireUser(svc.handleOrderDetail))
	mux.HandleFunc("POST /clients/orders/{id}/status", svc.Auth.RequireUser(svc.handleUpdateOrderStatus))
	mux.HandleFunc("POST /clients/orders/{id}/attach-project", svc.Auth.RequireUser(svc.handleAttachProject))
	mux.HandleFunc("POST /clients/orders/{id}/invoices", svc.Auth.RequireUser(svc.handleCreateInvoice))
	mux.HandleFunc("POST /clients/orders/{id}/invoices/{invoiceId}/status", svc.Auth.RequireUser(svc.handleSetInvoiceStatus))
}

func writeHTML(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(r.Context(), w)
}

func chromeFor(r *http.Request, user *auth.User, active string) web.Chrome {
	return web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), active)
}

// --- Quotes ------------------------------------------------------------------------------------

func (svc *Service) handleQuotesList(w http.ResponseWriter, r *http.Request, user *auth.User) {
	rows, err := ListQuotes(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, QuotesListPage(chromeFor(r, user, "/clients/orders"), rows))
}

func (svc *Service) handleNewQuoteForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	allClients, err := clients.ListAllSortedByName(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var suggested []LineItem
	if projectID := r.URL.Query().Get("projectId"); projectID != "" {
		items, _, err := EstimateProjectCost(ctx, svc.Pool, projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		suggested = items
	}
	writeHTML(w, r, NewQuotePage(chromeFor(r, user, "/clients/orders"), allClients, r.URL.Query().Get("clientId"), suggested))
}

func (svc *Service) handleCreateQuote(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	clientID := r.FormValue("clientId")
	email := strings.TrimSpace(r.FormValue("email"))
	if clientID == "" && email == "" {
		http.Error(w, "Select a client or provide an email address.", http.StatusBadRequest)
		return
	}

	var resolvedClientID string
	if clientID != "" {
		client, err := clients.GetByID(ctx, svc.Pool, clientID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if client == nil {
			http.Error(w, "Client not found.", http.StatusBadRequest)
			return
		}
		resolvedClientID = client.ID
	} else {
		client, err := clients.FindOrCreateByEmail(ctx, svc.Pool, email, strings.TrimSpace(r.FormValue("name")))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resolvedClientID = client.ID
	}

	items := collectLineItems(r.Form)
	var validUntil any
	if raw := strings.TrimSpace(r.FormValue("validUntil")); raw != "" {
		validUntil = raw
	}
	id, err := CreateQuote(ctx, svc.Pool, resolvedClientID, items, validUntil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/quotes/"+id, http.StatusSeeOther)
}

func (svc *Service) handleQuoteDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	quote, err := GetQuoteByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if quote == nil {
		http.NotFound(w, r)
		return
	}
	client, err := clients.GetByID(ctx, svc.Pool, quote.ClientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	order, err := GetOrderByQuoteID(ctx, svc.Pool, quote.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	statusLabel, err := settings.GetClassifierLabel(ctx, svc.Pool, settings.ClassifierQuoteStatus, quote.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, QuoteDetailPage(chromeFor(r, user, "/clients/orders"), *quote, client, order, statusLabel))
}

func (svc *Service) handleSetQuoteStatus(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := SetQuoteStatus(r.Context(), svc.Pool, id, r.FormValue("status")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/quotes/"+id, http.StatusSeeOther)
}

func (svc *Service) handleAcceptQuote(w http.ResponseWriter, r *http.Request, user *auth.User) {
	id := r.PathValue("id")
	orderID, err := AcceptQuote(r.Context(), svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/orders/"+orderID, http.StatusSeeOther)
}

// --- Orders ------------------------------------------------------------------------------------

func (svc *Service) handleOrdersKanban(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	orders, err := ListOrders(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	statuses, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierOrderStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, OrdersKanbanPage(chromeFor(r, user, "/clients/orders"), orders, statuses))
}

func (svc *Service) handleOrderDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	order, err := GetOrderByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if order == nil {
		http.NotFound(w, r)
		return
	}
	client, err := clients.GetByID(ctx, svc.Pool, order.ClientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var quote *Quote
	if order.QuoteID.Valid {
		quote, err = GetQuoteByID(ctx, svc.Pool, order.QuoteID.String)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	projects, err := ListProjectsOnOrder(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	invoices, err := ListInvoicesForOrder(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	unattached, err := ListUnattachedProjects(ctx, svc.Pool, order.ClientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	orderStatusLabels, err := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierOrderStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	invoiceStatuses, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierInvoiceStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var suggestedItems []LineItem
	var suggestedTotal float64
	for _, p := range projects {
		items, total, err := EstimateProjectCost(ctx, svc.Pool, p.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		suggestedItems = append(suggestedItems, items...)
		suggestedTotal += total
	}

	writeHTML(w, r, OrderDetailPage(chromeFor(r, user, "/clients/orders"), *order, client, quote, projects, invoices,
		unattached, orderStatusLabels, invoiceStatuses, suggestedItems, suggestedTotal))
}

func (svc *Service) handleUpdateOrderStatus(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	status := r.FormValue("status")

	valid, err := settings.GetClassifiers(ctx, svc.Pool, settings.ClassifierOrderStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ok := false
	for _, c := range valid {
		if c.Code == status {
			ok = true
			break
		}
	}
	if !ok {
		http.Error(w, "Invalid status.", http.StatusBadRequest)
		return
	}
	if err := UpdateOrderStatus(ctx, svc.Pool, id, status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// The kanban board's drag-and-drop calls this via fetch() and applies the move optimistically
	// client-side - no redirect needed, just acknowledge.
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/clients/orders", http.StatusSeeOther)
}

func (svc *Service) handleAttachProject(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	projectID := r.FormValue("projectId")
	if projectID == "" {
		http.Error(w, "Select a workflow.", http.StatusBadRequest)
		return
	}
	if err := AttachProjectToOrder(r.Context(), svc.Pool, id, projectID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/orders/"+id, http.StatusSeeOther)
}

func (svc *Service) handleCreateInvoice(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	items := collectLineItems(r.Form)
	if _, err := CreateInvoice(r.Context(), svc.Pool, id, items); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/orders/"+id, http.StatusSeeOther)
}

func (svc *Service) handleSetInvoiceStatus(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	orderID := r.PathValue("id")
	invoiceID := r.PathValue("invoiceId")
	if err := SetInvoiceStatus(r.Context(), svc.Pool, invoiceID, r.FormValue("status")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/clients/orders/"+orderID, http.StatusSeeOther)
}
