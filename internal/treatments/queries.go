package treatments

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

const treatmentColumns = "id, projectId, assetId, method, title, notes, performedByUserId, performedAt, createdAt, updatedAt, deletedAt"

func scanTreatment(rows *sql.Rows) (Treatment, error) {
	var t Treatment
	err := rows.Scan(&t.ID, &t.ProjectID, &t.AssetID, &t.Method, &t.Title, &t.Notes, &t.PerformedByUserID,
		&t.PerformedAt, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt)
	return t, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Treatment, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+treatmentColumns+" FROM Treatment WHERE id = ? AND deletedAt IS NULL", scanTreatment, id)
}

// Unlink soft-deletes the treatment — assetId is required (NOT NULL), so unlike Media there's no
// "detach" possible; this hides it from every query while leaving the row in the database.
func Unlink(ctx context.Context, q studiodb.Querier, id string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE Treatment SET deletedAt = ?, updatedAt = ? WHERE id = ?", time.Now(), time.Now(), id)
	return err
}

func scanDetailRow(rows *sql.Rows) (DetailRow, error) {
	var d DetailRow
	err := rows.Scan(&d.ID, &d.ProjectID, &d.AssetID, &d.Method, &d.Title, &d.Notes, &d.PerformedByUserID,
		&d.PerformedAt, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt, &d.ProjectTitle, &d.AssetTitle, &d.AssetReferenceCode, &d.ClientID, &d.ClientName)
	return d, err
}

const treatmentDetailColumns = `t.id, t.projectId, t.assetId, t.method, t.title, t.notes, t.performedByUserId, t.performedAt,
	t.createdAt, t.updatedAt, t.deletedAt`

func GetDetailByID(ctx context.Context, q studiodb.Querier, id string) (*DetailRow, error) {
	return studiodb.QueryOne(ctx, q, `
		SELECT `+treatmentDetailColumns+`, p.title, a.title, a.referenceCode, c.id, c.name
		FROM Treatment t
		JOIN Project p ON p.id = t.projectId
		JOIN Asset a ON a.id = t.assetId
		JOIN Client c ON c.id = a.clientId
		WHERE t.id = ? AND t.deletedAt IS NULL`, scanDetailRow, id)
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Title, &r.Method, &r.PerformedAt, &r.AssetTitle, &r.AssetReferenceCode, &r.ClientName)
	return r, err
}

const treatmentListSelect = `
	SELECT t.id, t.title, t.method, t.performedAt, a.title, a.referenceCode, c.name
	FROM Treatment t
	JOIN Asset a ON a.id = t.assetId
	JOIN Client c ON c.id = a.clientId`

// List is every Treatment across every Asset, newest-performed first — the module's landing
// page.
func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, treatmentListSelect+` WHERE t.deletedAt IS NULL ORDER BY t.performedAt DESC`, scanListRow)
}

// ListByAsset is every Treatment for one Asset across all its Projects, newest-performed first —
// the Asset detail page's "Treatments" section.
func ListByAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ListRow, error) {
	return studiodb.Query(ctx, q, treatmentListSelect+` WHERE t.assetId = ? AND t.deletedAt IS NULL ORDER BY t.performedAt DESC`, scanListRow, assetID)
}

// ListByProject is every Treatment for one Project, newest-performed first — the Project detail
// page's "Treatments" section.
func ListByProject(ctx context.Context, q studiodb.Querier, projectID string) ([]ListRow, error) {
	return studiodb.Query(ctx, q, treatmentListSelect+` WHERE t.projectId = ? AND t.deletedAt IS NULL ORDER BY t.performedAt DESC`, scanListRow, projectID)
}

// ListByProjectLimit is ListByProject capped to the N most recent — the Dashboard's active-project
// cards.
func ListByProjectLimit(ctx context.Context, q studiodb.Querier, projectID string, limit int) ([]ListRow, error) {
	return studiodb.Query(ctx, q, treatmentListSelect+` WHERE t.projectId = ? AND t.deletedAt IS NULL ORDER BY t.performedAt DESC LIMIT ?`, scanListRow, projectID, limit)
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
// first — the "pick a project" select on the standalone new-treatment form.
func ListProjectOptions(ctx context.Context, q studiodb.Querier) ([]ProjectOption, error) {
	return studiodb.Query(ctx, q, projectOptionSelect+` ORDER BY p.createdAt DESC`, scanProjectOption)
}

// ListProjectOptionsForAsset scopes ListProjectOptions to one Asset's own Projects — the Asset
// detail page's "+Add" modal, where the project picker shouldn't offer every project in the app.
func ListProjectOptionsForAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ProjectOption, error) {
	return studiodb.Query(ctx, q, projectOptionSelect+` AND p.assetId = ? ORDER BY p.createdAt DESC`, scanProjectOption, assetID)
}

type Input struct {
	ProjectID         string
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

// Create inserts a Treatment (AssetID resolved server-side from the Project via a subquery —
// every Project pins exactly one Asset, so the form only ever asks for a Project).
func Create(ctx context.Context, q studiodb.Querier, in Input) (string, error) {
	id := studiodb.NewID()
	now := time.Now()
	performedAt := in.PerformedAt
	if performedAt.IsZero() {
		performedAt = now
	}
	_, err := studiodb.Execute(ctx, q, `
		INSERT INTO Treatment (id, projectId, assetId, method, title, notes, performedByUserId, performedAt, updatedAt)
		SELECT ?, p.id, p.assetId, ?, ?, ?, ?, ?, ? FROM Project p WHERE p.id = ?`,
		id, in.Method, in.Title, in.Notes, nullIfEmpty(in.PerformedByUserID), performedAt, now, in.ProjectID)
	return id, err
}

type UpdateInput struct {
	Method string
	Title  string
	Notes  string
}

func Update(ctx context.Context, q studiodb.Querier, id string, in UpdateInput) error {
	_, err := studiodb.Execute(ctx, q,
		"UPDATE Treatment SET method = ?, title = ?, notes = ?, updatedAt = ? WHERE id = ?",
		in.Method, in.Title, in.Notes, time.Now(), id)
	return err
}
