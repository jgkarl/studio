package treatments

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

const treatmentColumns = "id, assetId, method, title, notes, performedByUserId, performedAt, createdAt, updatedAt"

func scanTreatment(rows *sql.Rows) (Treatment, error) {
	var t Treatment
	err := rows.Scan(&t.ID, &t.AssetID, &t.Method, &t.Title, &t.Notes, &t.PerformedByUserID,
		&t.PerformedAt, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Treatment, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+treatmentColumns+" FROM Treatment WHERE id = ?", scanTreatment, id)
}

func scanDetailRow(rows *sql.Rows) (DetailRow, error) {
	var d DetailRow
	err := rows.Scan(&d.ID, &d.AssetID, &d.Method, &d.Title, &d.Notes, &d.PerformedByUserID,
		&d.PerformedAt, &d.CreatedAt, &d.UpdatedAt, &d.AssetTitle, &d.AssetReferenceCode, &d.ClientID, &d.ClientName)
	return d, err
}

const treatmentDetailColumns = `t.id, t.assetId, t.method, t.title, t.notes, t.performedByUserId, t.performedAt,
	t.createdAt, t.updatedAt`

func GetDetailByID(ctx context.Context, q studiodb.Querier, id string) (*DetailRow, error) {
	return studiodb.QueryOne(ctx, q, `
		SELECT `+treatmentDetailColumns+`, a.title, a.referenceCode, c.id, c.name
		FROM Treatment t
		JOIN Asset a ON a.id = t.assetId
		JOIN Client c ON c.id = a.clientId
		WHERE t.id = ?`, scanDetailRow, id)
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Title, &r.Method, &r.PerformedAt, &r.AssetTitle, &r.AssetReferenceCode, &r.ClientName)
	return r, err
}

// List is every Treatment across every Asset, newest-performed first — the module's landing
// page.
func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT t.id, t.title, t.method, t.performedAt, a.title, a.referenceCode, c.name
		FROM Treatment t
		JOIN Asset a ON a.id = t.assetId
		JOIN Client c ON c.id = a.clientId
		ORDER BY t.performedAt DESC`, scanListRow)
}

// ListByAsset is every Treatment for one Asset, newest-performed first — the Asset detail
// page's "Treatments" section.
func ListByAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT t.id, t.title, t.method, t.performedAt, a.title, a.referenceCode, c.name
		FROM Treatment t
		JOIN Asset a ON a.id = t.assetId
		JOIN Client c ON c.id = a.clientId
		WHERE t.assetId = ?
		ORDER BY t.performedAt DESC`, scanListRow, assetID)
}

func scanAssetOption(rows *sql.Rows) (AssetOption, error) {
	var a AssetOption
	err := rows.Scan(&a.ID, &a.Title, &a.ReferenceCode, &a.ClientName)
	return a, err
}

// ListAssetOptions is every Asset (with owning client name), newest first — the "pick an asset"
// select on the new-treatment form.
func ListAssetOptions(ctx context.Context, q studiodb.Querier) ([]AssetOption, error) {
	return studiodb.Query(ctx, q, `
		SELECT a.id, a.title, a.referenceCode, c.name FROM Asset a JOIN Client c ON c.id = a.clientId
		ORDER BY a.createdAt DESC`, scanAssetOption)
}

type Input struct {
	AssetID           string
	Method            string
	Title             string
	Notes             string
	PerformedByUserID string
	PerformedAt       time.Time
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func Create(ctx context.Context, q studiodb.Querier, in Input) (string, error) {
	id := studiodb.NewID()
	now := time.Now()
	performedAt := in.PerformedAt
	if performedAt.IsZero() {
		performedAt = now
	}
	_, err := studiodb.Execute(ctx, q, `
		INSERT INTO Treatment (id, assetId, method, title, notes, performedByUserId, performedAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.AssetID, in.Method, in.Title, in.Notes, nullIfEmpty(in.PerformedByUserID), performedAt, now)
	return id, err
}
