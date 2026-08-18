// Package dashboard is the landing page after sign-in: counts, active workflows, draft reports.
// Queries are inline here (not routed through the Assets/Workflows domain packages those modules
// will add later) - same structure as the original app's dashboard page, which only ever imported
// the Classifiers label-map helper and otherwise queried directly.
package dashboard

import (
	"database/sql"
	"net/http"

	"studio/internal/auth"
	studiodb "studio/internal/db"
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
	ID, Title, AssetTitle, AssetReferenceCode string
}

func scanActiveProject(rows *sql.Rows) (activeProject, error) {
	var p activeProject
	var assetTitle sql.NullString
	err := rows.Scan(&p.ID, &p.Title, &assetTitle, &p.AssetReferenceCode)
	p.AssetTitle = assetTitle.String
	return p, err
}

type draftReport struct {
	ID, Title, AssetTitle, AssetReferenceCode string
}

func scanDraftReport(rows *sql.Rows) (draftReport, error) {
	var rep draftReport
	var assetTitle sql.NullString
	err := rows.Scan(&rep.ID, &rep.Title, &assetTitle, &rep.AssetReferenceCode)
	rep.AssetTitle = assetTitle.String
	return rep, err
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
	treatmentCount, err := studiodb.QueryOne(ctx, svc.Pool, "SELECT COUNT(*) AS n FROM Treatment WHERE deletedAt IS NULL", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	activeProjects, err := studiodb.Query(ctx, svc.Pool,
		`SELECT p.id, p.title, a.title, a.referenceCode FROM Project p JOIN Asset a ON a.id = p.assetId
		 WHERE p.stage != 'completed' AND p.deletedAt IS NULL ORDER BY p.updatedAt DESC LIMIT 6`, scanActiveProject)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	draftReportRows, err := studiodb.Query(ctx, svc.Pool, `
		SELECT r.id, r.title, a.title, a.referenceCode FROM Report r JOIN Asset a ON a.id = r.assetId
		WHERE r.status = 'draft' AND r.deletedAt IS NULL ORDER BY r.updatedAt DESC LIMIT 6`, scanDraftReport)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	draftReportCount, err := studiodb.QueryOne(ctx, svc.Pool, "SELECT COUNT(*) AS n FROM Report WHERE status = ? AND deletedAt IS NULL", scanCount, "draft")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	chrome := web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), "/")
	page := Page(chrome, deref(clientCount), deref(assetCount), deref(treatmentCount), activeProjects, draftReportRows, deref(draftReportCount))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(ctx, w)
}

func deref(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}
