package reporter

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

const reportColumns = `id, projectId, assetId, title, content, status, authorId,
	summary, conditionFindings, treatmentPerformed, materialsUsed, recommendations,
	coverMediaId, layoutStyle, showCover, showSummary, showCondition, showTreatment, showMaterials, showRecommendations,
	createdAt, updatedAt, deletedAt`

func scanReport(rows *sql.Rows) (Report, error) {
	var r Report
	err := rows.Scan(&r.ID, &r.ProjectID, &r.AssetID, &r.Title, &r.Content, &r.Status, &r.AuthorID,
		&r.Summary, &r.ConditionFindings, &r.TreatmentPerformed, &r.MaterialsUsed, &r.Recommendations,
		&r.CoverMediaID, &r.LayoutStyle, &r.ShowCover, &r.ShowSummary, &r.ShowCondition, &r.ShowTreatment, &r.ShowMaterials, &r.ShowRecommendations,
		&r.CreatedAt, &r.UpdatedAt, &r.DeletedAt)
	return r, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Report, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+reportColumns+" FROM Report WHERE id = ? AND deletedAt IS NULL", scanReport, id)
}

// Unlink soft-deletes the report — assetId is required (NOT NULL), so unlike Media there's no
// "detach" possible; this hides it from every query while leaving the row in the database.
func Unlink(ctx context.Context, q studiodb.Querier, id string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE Report SET deletedAt = ? WHERE id = ?", time.Now(), id)
	return err
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Title, &r.Status, &r.AssetTitle, &r.AssetReferenceCode, &r.ClientName, &r.AuthorName)
	return r, err
}

const reportListSelect = `
	SELECT r.id, r.title, r.status, a.title, a.referenceCode, c.name, u.name
	FROM Report r
	JOIN Asset a ON a.id = r.assetId
	JOIN Client c ON c.id = a.clientId
	LEFT JOIN User u ON u.id = r.authorId`

func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, reportListSelect+` WHERE r.deletedAt IS NULL ORDER BY r.updatedAt DESC`, scanListRow)
}

// ListByAsset returns every report for one asset across all its Projects, newest first - used by
// the asset detail page's Reports section.
func ListByAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ListRow, error) {
	return studiodb.Query(ctx, q, reportListSelect+` WHERE r.assetId = ? AND r.deletedAt IS NULL ORDER BY r.updatedAt DESC`, scanListRow, assetID)
}

// ListByProject is every Report for one Project, newest first — the Project detail page's
// "Reports" section.
func ListByProject(ctx context.Context, q studiodb.Querier, projectID string) ([]ListRow, error) {
	return studiodb.Query(ctx, q, reportListSelect+` WHERE r.projectId = ? AND r.deletedAt IS NULL ORDER BY r.updatedAt DESC`, scanListRow, projectID)
}

// ListByProjectLimit is ListByProject capped to the N most recent — the Dashboard's active-project
// cards.
func ListByProjectLimit(ctx context.Context, q studiodb.Querier, projectID string, limit int) ([]ListRow, error) {
	return studiodb.Query(ctx, q, reportListSelect+` WHERE r.projectId = ? AND r.deletedAt IS NULL ORDER BY r.updatedAt DESC LIMIT ?`, scanListRow, projectID, limit)
}

func scanProjectOption(rows *sql.Rows) (ProjectOption, error) {
	var p ProjectOption
	err := rows.Scan(&p.ID, &p.Title, &p.AssetTitle, &p.AssetRef, &p.ClientName)
	return p, err
}

const projectOptionSelect = `
	SELECT p.id, p.title, a.title, a.referenceCode, c.name
	FROM Project p
	JOIN Asset a ON a.id = p.assetId
	JOIN Client c ON c.id = a.clientId
	WHERE p.deletedAt IS NULL`

// ListProjectOptions is every open Project (with owning asset/client display fields), newest
// first — the "pick a project" select on the standalone new-report form.
func ListProjectOptions(ctx context.Context, q studiodb.Querier) ([]ProjectOption, error) {
	return studiodb.Query(ctx, q, projectOptionSelect+` ORDER BY p.createdAt DESC`, scanProjectOption)
}

// ListProjectOptionsForAsset scopes ListProjectOptions to one Asset's own Projects — the Asset
// detail page's "+Add" modal, where the project picker shouldn't offer every project in the app.
func ListProjectOptionsForAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ProjectOption, error) {
	return studiodb.Query(ctx, q, projectOptionSelect+` AND p.assetId = ? ORDER BY p.createdAt DESC`, scanProjectOption, assetID)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Create inserts a Report (AssetID resolved server-side from the Project via a subquery — every
// Project pins exactly one Asset, so the form only ever asks for a Project).
func Create(ctx context.Context, q studiodb.Querier, projectID, title, authorID string, sections Sections) (string, error) {
	id := studiodb.NewID()
	now := time.Now()
	_, err := studiodb.Execute(ctx, q, `
		INSERT INTO Report (id, assetId, projectId, title, content, status, authorId, conditionFindings, treatmentPerformed, createdAt, updatedAt)
		SELECT ?, p.assetId, p.id, ?, ?, 'draft', ?, ?, ?, ?, ? FROM Project p WHERE p.id = ?`,
		id, title, emptyDoc(), authorID,
		nullIfEmpty(sections.ConditionFindings), nullIfEmpty(sections.TreatmentPerformed), now, now, projectID)
	return id, err
}

func UpdateSections(ctx context.Context, q studiodb.Querier, id string, in SectionsInput) error {
	_, err := studiodb.Execute(ctx, q, `
		UPDATE Report SET summary = ?, conditionFindings = ?, treatmentPerformed = ?, materialsUsed = ?, recommendations = ?, updatedAt = ?
		WHERE id = ?`,
		nullIfEmpty(in.Summary), nullIfEmpty(in.ConditionFindings), nullIfEmpty(in.TreatmentPerformed),
		nullIfEmpty(in.MaterialsUsed), nullIfEmpty(in.Recommendations), time.Now(), id)
	return err
}

func UpdateLayout(ctx context.Context, q studiodb.Querier, id string, in LayoutInput) error {
	_, err := studiodb.Execute(ctx, q, `
		UPDATE Report SET layoutStyle = ?, coverMediaId = ?,
			showCover = ?, showSummary = ?, showCondition = ?, showTreatment = ?, showMaterials = ?, showRecommendations = ?, updatedAt = ?
		WHERE id = ?`,
		in.LayoutStyle, nullIfEmpty(in.CoverMediaID),
		in.ShowCover, in.ShowSummary, in.ShowCondition, in.ShowTreatment, in.ShowMaterials, in.ShowRecommendations, time.Now(), id)
	return err
}

func SetStatus(ctx context.Context, q studiodb.Querier, id, status string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE Report SET status = ?, updatedAt = ? WHERE id = ?", status, time.Now(), id)
	return err
}

// ExistsForProject reports whether at least one non-deleted Report already exists for a
// Project — the Finish-project flow's "auto-draft only if missing" check.
func ExistsForProject(ctx context.Context, q studiodb.Querier, projectID string) (bool, error) {
	n, err := studiodb.QueryOne(ctx, q, "SELECT COUNT(*) AS n FROM Report WHERE projectId = ? AND deletedAt IS NULL",
		func(rows *sql.Rows) (int, error) { var c int; scanErr := rows.Scan(&c); return c, scanErr }, projectID)
	if err != nil || n == nil {
		return false, err
	}
	return *n > 0, nil
}
