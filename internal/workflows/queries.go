package workflows

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

const projectColumns = "id, orderId, assetId, title, stage, assignedToUserId, startedAt, completedAt, createdAt, updatedAt"

func scanProject(rows *sql.Rows) (Project, error) {
	var p Project
	err := rows.Scan(&p.ID, &p.OrderID, &p.AssetID, &p.Title, &p.Stage, &p.AssignedToUserID,
		&p.StartedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Project, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+projectColumns+" FROM Project WHERE id = ?", scanProject, id)
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Title, &r.Stage, &r.AssetTitle, &r.AssetReferenceCode, &r.ClientName, &r.OrderNumber)
	return r, err
}

func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT p.id, p.title, p.stage, a.title, a.referenceCode, c.name, o.orderNumber
		FROM Project p
		JOIN Asset a ON a.id = p.assetId
		JOIN Client c ON c.id = a.clientId
		LEFT JOIN "Order" o ON o.id = p.orderId
		ORDER BY p.updatedAt DESC`, scanListRow)
}

func scanAssetOption(rows *sql.Rows) (AssetOption, error) {
	var a AssetOption
	err := rows.Scan(&a.ID, &a.Title, &a.ReferenceCode, &a.ClientName)
	return a, err
}

// ListAssetOptions is every Asset (with owning client name), newest first — the "pick an asset"
// select on the new-workflow form.
func ListAssetOptions(ctx context.Context, q studiodb.Querier) ([]AssetOption, error) {
	return studiodb.Query(ctx, q, `
		SELECT a.id, a.title, a.referenceCode, c.name FROM Asset a JOIN Client c ON c.id = a.clientId
		ORDER BY a.createdAt DESC`, scanAssetOption)
}

func Create(ctx context.Context, q studiodb.Querier, assetID, title string) (string, error) {
	id := studiodb.NewID()
	now := time.Now()
	_, err := studiodb.Execute(ctx, q,
		"INSERT INTO Project (id, assetId, title, startedAt, updatedAt) VALUES (?, ?, ?, ?, ?)",
		id, assetID, title, now, now)
	return id, err
}

// AdvanceStage moves a Project to nextStage — caller must have already validated the transition
// against StageTransitions. completedAt is set the first time a Project reaches handover_done.
func AdvanceStage(ctx context.Context, q studiodb.Querier, id string, nextStage Stage) error {
	var completedAt any
	if nextStage == StageHandoverDone {
		completedAt = time.Now()
	}
	_, err := studiodb.Execute(ctx, q,
		"UPDATE Project SET stage = ?, completedAt = COALESCE(?, completedAt), updatedAt = ? WHERE id = ?",
		nextStage, completedAt, time.Now(), id)
	return err
}

func scanActivity(rows *sql.Rows) (Activity, error) {
	var a Activity
	err := rows.Scan(&a.ID, &a.Description, &a.StartedAt, &a.DurationMinutes, &a.ActivityTypeTitle, &a.UserName)
	return a, err
}

func ListActivities(ctx context.Context, q studiodb.Querier, projectID string) ([]Activity, error) {
	return studiodb.Query(ctx, q, `
		SELECT a.id, a.description, a.startedAt, a.durationMinutes, c.title, u.name
		FROM Activity a
		JOIN Classifier c ON c.id = a.activityTypeId
		JOIN User u ON u.id = a.userId
		WHERE a.projectId = ? ORDER BY a.startedAt DESC`, scanActivity, projectID)
}

type LogActivityInput struct {
	ActivityTypeID  string
	UserID          string
	Description     string
	StartedAt       time.Time
	DurationMinutes any // int or nil
	MaterialsUsed   any // JSON text or nil
}

func LogActivity(ctx context.Context, q studiodb.Querier, projectID string, in LogActivityInput) (string, error) {
	id := studiodb.NewID()
	_, err := studiodb.Execute(ctx, q,
		"INSERT INTO Activity (id, projectId, activityTypeId, userId, description, startedAt, durationMinutes, materialsUsed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, projectID, in.ActivityTypeID, in.UserID, in.Description, in.StartedAt, in.DurationMinutes, in.MaterialsUsed)
	return id, err
}
