// Package dashboard is the landing page after sign-in: Client/Asset/Project/Report counters
// (active vs. all, draft vs. final), and a card per active Project showing its latest
// Assessments/Treatments/Reports. List caps are configurable per module via Settings → Features
// (internal/settings' AppSetting-backed GetInt) rather than hardcoded.
package dashboard

import (
	"database/sql"
	"net/http"

	"stuudio/internal/assessments"
	"stuudio/internal/auth"
	studiodb "stuudio/internal/db"
	"stuudio/internal/reporter"
	"stuudio/internal/settings"
	"stuudio/internal/treatments"
	"stuudio/internal/web"
)

type Service struct {
	Pool *sql.DB
	Auth *auth.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /{$}", svc.Auth.RequireUser(svc.handleIndex))
}

type projectRow struct {
	ID, Title, AssetTitle, AssetReferenceCode string
}

func scanProjectRow(rows *sql.Rows) (projectRow, error) {
	var p projectRow
	var assetTitle sql.NullString
	err := rows.Scan(&p.ID, &p.Title, &assetTitle, &p.AssetReferenceCode)
	p.AssetTitle = assetTitle.String
	return p, err
}

func scanCount(rows *sql.Rows) (int, error) {
	var n int
	err := rows.Scan(&n)
	return n, err
}

// projectCard is one Active Projects card: the project itself plus its latest
// Assessments/Treatments/Reports, each capped per Settings → Features.
type projectCard struct {
	Project     projectRow
	Assessments []assessments.ListRow
	Treatments  []treatments.ListRow
	Reports     []reporter.ListRow
}

func (svc *Service) handleIndex(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	pool := svc.Pool

	clientCountAll, err := studiodb.QueryOne(ctx, pool, "SELECT COUNT(*) AS n FROM Client", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	clientCountActive, err := studiodb.QueryOne(ctx, pool, `
		SELECT COUNT(*) AS n FROM Client c WHERE EXISTS (
			SELECT 1 FROM Asset a JOIN Project p ON p.assetId = a.id
			WHERE a.clientId = c.id AND p.stage != 'completed' AND p.deletedAt IS NULL)`, scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assetCountAll, err := studiodb.QueryOne(ctx, pool, "SELECT COUNT(*) AS n FROM Asset", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assetCountActive, err := studiodb.QueryOne(ctx, pool, `
		SELECT COUNT(*) AS n FROM Asset a WHERE EXISTS (
			SELECT 1 FROM Project p WHERE p.assetId = a.id AND p.stage != 'completed' AND p.deletedAt IS NULL)`, scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projectCountAll, err := studiodb.QueryOne(ctx, pool, "SELECT COUNT(*) AS n FROM Project WHERE deletedAt IS NULL", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projectCountActive, err := studiodb.QueryOne(ctx, pool,
		"SELECT COUNT(*) AS n FROM Project WHERE stage != 'completed' AND deletedAt IS NULL", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reportCountDraft, err := studiodb.QueryOne(ctx, pool,
		"SELECT COUNT(*) AS n FROM Report WHERE status = 'draft' AND deletedAt IS NULL", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reportCountFinal, err := studiodb.QueryOne(ctx, pool,
		"SELECT COUNT(*) AS n FROM Report WHERE status = 'final' AND deletedAt IS NULL", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	activeProjectsLimit := settings.GetInt(ctx, pool, "dashboard.active_projects.limit", 5)
	assessmentsLimit := settings.GetInt(ctx, pool, "dashboard.assessments.limit", 5)
	treatmentsLimit := settings.GetInt(ctx, pool, "dashboard.treatments.limit", 5)
	reportsLimit := settings.GetInt(ctx, pool, "dashboard.reports.limit", 5)

	activeProjectRows, err := studiodb.Query(ctx, pool, `
		SELECT p.id, p.title, a.title, a.referenceCode FROM Project p JOIN Asset a ON a.id = p.assetId
		WHERE p.stage != 'completed' AND p.deletedAt IS NULL ORDER BY p.updatedAt DESC LIMIT ?`,
		scanProjectRow, activeProjectsLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cards := make([]projectCard, len(activeProjectRows))
	for i, p := range activeProjectRows {
		a, err := assessments.ListByProjectLimit(ctx, pool, p.ID, assessmentsLimit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		t, err := treatments.ListByProjectLimit(ctx, pool, p.ID, treatmentsLimit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rep, err := reporter.ListByProjectLimit(ctx, pool, p.ID, reportsLimit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cards[i] = projectCard{Project: p, Assessments: a, Treatments: t, Reports: rep}
	}

	chrome := web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), "/")
	page := Page(chrome,
		deref(clientCountActive), deref(clientCountAll),
		deref(assetCountActive), deref(assetCountAll),
		deref(projectCountActive), deref(projectCountAll),
		deref(reportCountDraft), deref(reportCountFinal),
		cards)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(ctx, w)
}

func deref(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}
