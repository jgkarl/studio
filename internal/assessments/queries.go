package assessments

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

const assessmentColumns = `id, projectId, assetId, "condition", description, recordedAt, updatedAt, deletedAt`

func scanAssessment(rows *sql.Rows) (Assessment, error) {
	var a Assessment
	err := rows.Scan(&a.ID, &a.ProjectID, &a.AssetID, &a.Condition, &a.Description, &a.RecordedAt, &a.UpdatedAt, &a.DeletedAt)
	return a, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Assessment, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+assessmentColumns+" FROM Assessment WHERE id = ? AND deletedAt IS NULL", scanAssessment, id)
}

// Unlink soft-deletes the assessment — projectId/assetId are required (NOT NULL), so unlike Media
// there's no "detach" possible; this hides it from every query while leaving the row in the
// database.
func Unlink(ctx context.Context, q studiodb.Querier, id string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE Assessment SET deletedAt = ?, updatedAt = ? WHERE id = ?", time.Now(), time.Now(), id)
	return err
}

const assessmentDetailColumns = `a.id, a.projectId, a.assetId, a."condition", a.description, a.recordedAt, a.updatedAt, a.deletedAt`

func scanDetailRow(rows *sql.Rows) (DetailRow, error) {
	var d DetailRow
	err := rows.Scan(&d.ID, &d.ProjectID, &d.AssetID, &d.Condition, &d.Description, &d.RecordedAt, &d.UpdatedAt, &d.DeletedAt,
		&d.ProjectTitle, &d.AssetTitle, &d.AssetReferenceCode, &d.ClientID, &d.ClientName)
	return d, err
}

func GetDetailByID(ctx context.Context, q studiodb.Querier, id string) (*DetailRow, error) {
	return studiodb.QueryOne(ctx, q, `
		SELECT `+assessmentDetailColumns+`, p.title, ast.title, ast.referenceCode, c.id, c.name
		FROM Assessment a
		JOIN Project p ON p.id = a.projectId
		JOIN Asset ast ON ast.id = a.assetId
		JOIN Client c ON c.id = ast.clientId
		WHERE a.id = ? AND a.deletedAt IS NULL`, scanDetailRow, id)
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Condition, &r.Description, &r.RecordedAt, &r.AssetTitle, &r.AssetReferenceCode, &r.ClientName, &r.ProjectID, &r.ProjectTitle)
	return r, err
}

const assessmentListSelect = `
	SELECT a.id, a."condition", a.description, a.recordedAt, ast.title, ast.referenceCode, c.name, p.id, p.title
	FROM Assessment a
	JOIN Asset ast ON ast.id = a.assetId
	JOIN Client c ON c.id = ast.clientId
	JOIN Project p ON p.id = a.projectId`

// List is every Assessment across every Asset, newest-recorded first — the module's landing page.
func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, assessmentListSelect+` WHERE a.deletedAt IS NULL ORDER BY a.recordedAt DESC`, scanListRow)
}

// ListByProject is every Assessment for one Project, newest-recorded first — the Project detail
// page's "Assessments" section.
func ListByProject(ctx context.Context, q studiodb.Querier, projectID string) ([]ListRow, error) {
	return studiodb.Query(ctx, q, assessmentListSelect+` WHERE a.projectId = ? AND a.deletedAt IS NULL ORDER BY a.recordedAt DESC`, scanListRow, projectID)
}

// ListByProjectLimit is ListByProject capped to the N most recent — the Dashboard's active-project
// cards.
func ListByProjectLimit(ctx context.Context, q studiodb.Querier, projectID string, limit int) ([]ListRow, error) {
	return studiodb.Query(ctx, q, assessmentListSelect+` WHERE a.projectId = ? AND a.deletedAt IS NULL ORDER BY a.recordedAt DESC LIMIT ?`, scanListRow, projectID, limit)
}

// ListByAsset is every Assessment for one Asset across all its Projects, newest-recorded first —
// the Asset detail page's "Assessments" section.
func ListByAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ListRow, error) {
	return studiodb.Query(ctx, q, assessmentListSelect+` WHERE a.assetId = ? AND a.deletedAt IS NULL ORDER BY a.recordedAt DESC`, scanListRow, assetID)
}

func scanProjectOption(rows *sql.Rows) (ProjectOption, error) {
	var p ProjectOption
	err := rows.Scan(&p.ID, &p.Title, &p.AssetTitle, &p.AssetRef, &p.ClientName)
	return p, err
}

const projectOptionSelect = `
	SELECT p.id, p.title, ast.title, ast.referenceCode, c.name
	FROM Project p
	JOIN Asset ast ON ast.id = p.assetId
	JOIN Client c ON c.id = ast.clientId
	WHERE p.deletedAt IS NULL`

// ListProjectOptions is every open Project (with owning asset/client display fields), newest
// first — the "pick a project" select on the standalone new-assessment form.
func ListProjectOptions(ctx context.Context, q studiodb.Querier) ([]ProjectOption, error) {
	return studiodb.Query(ctx, q, projectOptionSelect+` ORDER BY p.createdAt DESC`, scanProjectOption)
}

// ListProjectOptionsForAsset scopes ListProjectOptions to one Asset's own Projects — the Asset
// detail page's "+Add" modal, where the project picker shouldn't offer every project in the app.
func ListProjectOptionsForAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ProjectOption, error) {
	return studiodb.Query(ctx, q, projectOptionSelect+` AND p.assetId = ? ORDER BY p.createdAt DESC`, scanProjectOption, assetID)
}

type Input struct {
	ProjectID   string
	Condition   string
	Description string
}

// Create inserts an Assessment (AssetID resolved server-side from the Project via a subquery —
// every Project pins exactly one Asset, so the form only ever asks for a Project) and updates
// Asset.currentStateId - the same "fixate state" concept the old assets.RecordState had, now
// requiring a Project.
func Create(ctx context.Context, pool *sql.DB, in Input) (string, error) {
	return studiodb.WithTransaction(ctx, pool, func(tx *sql.Tx) (string, error) {
		id := studiodb.NewID()
		now := time.Now()
		if _, err := studiodb.Execute(ctx, tx, `
			INSERT INTO Assessment (id, projectId, assetId, "condition", description, recordedAt, updatedAt)
			SELECT ?, p.id, p.assetId, ?, ?, ?, ? FROM Project p WHERE p.id = ?`,
			id, in.Condition, in.Description, now, now, in.ProjectID); err != nil {
			return "", err
		}
		if _, err := studiodb.Execute(ctx, tx,
			"UPDATE Asset SET currentStateId = ? WHERE id = (SELECT assetId FROM Project WHERE id = ?)",
			id, in.ProjectID); err != nil {
			return "", err
		}
		return id, nil
	})
}

type UpdateInput struct {
	Condition   string
	Description string
}

func Update(ctx context.Context, q studiodb.Querier, id string, in UpdateInput) error {
	_, err := studiodb.Execute(ctx, q,
		`UPDATE Assessment SET "condition" = ?, description = ?, updatedAt = ? WHERE id = ?`,
		in.Condition, in.Description, time.Now(), id)
	return err
}
