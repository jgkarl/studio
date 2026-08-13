// Package dashboard is the landing page after sign-in: counts, active workflows, open orders.
// Queries are inline here (not routed through the Assets/Workflows/Commerce domain packages
// those modules will add later) - same structure as the original app's dashboard page, which
// only ever imported the Classifiers label-map helper and otherwise queried directly.
package dashboard

import (
	"database/sql"
	"net/http"

	"studio/internal/auth"
	studiodb "studio/internal/db"
	"studio/internal/settings"
	"studio/internal/web"
)

type Service struct {
	Pool *sql.DB
	Auth *auth.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /{$}", svc.Auth.RequireUser(svc.handleIndex))
}

type activeProject struct {
	ID, Title, Stage, AssetTitle, AssetReferenceCode string
}

func scanActiveProject(rows *sql.Rows) (activeProject, error) {
	var p activeProject
	var assetTitle sql.NullString
	err := rows.Scan(&p.ID, &p.Title, &p.Stage, &assetTitle, &p.AssetReferenceCode)
	p.AssetTitle = assetTitle.String
	return p, err
}

type openOrder struct {
	ID, OrderNumber, Status, ClientName string
}

func scanOpenOrder(rows *sql.Rows) (openOrder, error) {
	var o openOrder
	err := rows.Scan(&o.ID, &o.OrderNumber, &o.Status, &o.ClientName)
	return o, err
}

func scanCount(rows *sql.Rows) (int, error) {
	var n int
	err := rows.Scan(&n)
	return n, err
}

func (svc *Service) handleIndex(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()

	clientCount, err := studiodb.QueryOne(ctx, svc.Pool, "SELECT COUNT(*) AS n FROM Client", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assetCount, err := studiodb.QueryOne(ctx, svc.Pool, "SELECT COUNT(*) AS n FROM Asset", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	activeProjects, err := studiodb.Query(ctx, svc.Pool,
		`SELECT p.id, p.title, p.stage, a.title, a.referenceCode FROM Project p JOIN Asset a ON a.id = p.assetId
		 WHERE p.stage != ? ORDER BY p.updatedAt DESC LIMIT 6`, scanActiveProject, "handover_done")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	openOrders, err := studiodb.Query(ctx, svc.Pool, `
		SELECT o.id, o.orderNumber, o.status, c.name FROM "Order" o JOIN Client c ON c.id = o.clientId
		WHERE o.status NOT IN ('completed', 'archived') ORDER BY o.updatedAt DESC LIMIT 6`, scanOpenOrder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	draftReports, err := studiodb.QueryOne(ctx, svc.Pool, "SELECT COUNT(*) AS n FROM Report WHERE status = ?", scanCount, "draft")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	orderStatusLabels, err := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierOrderStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	chrome := web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), "/")
	page := Page(chrome, deref(clientCount), deref(assetCount), activeProjects, openOrders, deref(draftReports), orderStatusLabels)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(ctx, w)
}

func deref(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}
