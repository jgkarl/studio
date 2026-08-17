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
	createdAt, updatedAt`

func scanReport(rows *sql.Rows) (Report, error) {
	var r Report
	err := rows.Scan(&r.ID, &r.ProjectID, &r.AssetID, &r.Title, &r.Content, &r.Status, &r.AuthorID,
		&r.Summary, &r.ConditionFindings, &r.TreatmentPerformed, &r.MaterialsUsed, &r.Recommendations,
		&r.CoverMediaID, &r.LayoutStyle, &r.ShowCover, &r.ShowSummary, &r.ShowCondition, &r.ShowTreatment, &r.ShowMaterials, &r.ShowRecommendations,
		&r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Report, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+reportColumns+" FROM Report WHERE id = ?", scanReport, id)
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Title, &r.Status, &r.AssetTitle, &r.AssetReferenceCode, &r.AuthorName)
	return r, err
}

func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT r.id, r.title, r.status, a.title, a.referenceCode, u.name
		FROM Report r
		JOIN Asset a ON a.id = r.assetId
		LEFT JOIN User u ON u.id = r.authorId
		ORDER BY r.updatedAt DESC`, scanListRow)
}

// ListByAsset returns every report for one asset, newest first - used by the asset detail page's
// Reports tab. Mirrors List() with an assetId filter.
func ListByAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT r.id, r.title, r.status, a.title, a.referenceCode, u.name
		FROM Report r
		JOIN Asset a ON a.id = r.assetId
		LEFT JOIN User u ON u.id = r.authorId
		WHERE r.assetId = ?
		ORDER BY r.updatedAt DESC`, scanListRow, assetID)
}

func scanAssetOption(rows *sql.Rows) (AssetOption, error) {
	var a AssetOption
	err := rows.Scan(&a.ID, &a.Title, &a.ReferenceCode, &a.ClientName)
	return a, err
}

func ListAssetOptions(ctx context.Context, q studiodb.Querier) ([]AssetOption, error) {
	return studiodb.Query(ctx, q, `
		SELECT a.id, a.title, a.referenceCode, c.name FROM Asset a JOIN Client c ON c.id = a.clientId
		ORDER BY a.createdAt DESC`, scanAssetOption)
}

func scanProjectOption(rows *sql.Rows) (ProjectOption, error) {
	var p ProjectOption
	err := rows.Scan(&p.ID, &p.Title)
	return p, err
}

func ListProjectOptions(ctx context.Context, q studiodb.Querier) ([]ProjectOption, error) {
	return studiodb.Query(ctx, q, "SELECT id, title FROM Project ORDER BY createdAt DESC", scanProjectOption)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func Create(ctx context.Context, q studiodb.Querier, assetID string, projectID *string, title, authorID string, sections Sections) (string, error) {
	id := studiodb.NewID()
	now := time.Now()
	var projectArg any
	if projectID != nil {
		projectArg = *projectID
	}
	_, err := studiodb.Execute(ctx, q, `
		INSERT INTO Report (id, assetId, projectId, title, content, status, authorId, conditionFindings, treatmentPerformed, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?)`,
		id, assetID, projectArg, title, emptyDoc(), authorID,
		nullIfEmpty(sections.ConditionFindings), nullIfEmpty(sections.TreatmentPerformed), now, now)
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
