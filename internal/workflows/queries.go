package workflows

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

const projectColumns = "id, assetId, title, stage, priority, targetReviewDate, assignedToUserId, startedAt, completedAt, createdAt, updatedAt, deletedAt"

func scanProject(rows *sql.Rows) (Project, error) {
	var p Project
	err := rows.Scan(&p.ID, &p.AssetID, &p.Title, &p.Stage, &p.Priority, &p.TargetReviewDate, &p.AssignedToUserID,
		&p.StartedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	return p, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Project, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+projectColumns+" FROM Project WHERE id = ? AND deletedAt IS NULL", scanProject, id)
}

// Unlink soft-deletes the project — assetId is required (NOT NULL), so unlike Media there's no
// "detach" possible; this hides it from every query while leaving the row in the database.
func Unlink(ctx context.Context, q studiodb.Querier, id string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE Project SET deletedAt = ?, updatedAt = ? WHERE id = ?", time.Now(), time.Now(), id)
	return err
}

func UpdateTitle(ctx context.Context, q studiodb.Querier, id, title string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE Project SET title = ?, updatedAt = ? WHERE id = ?", title, time.Now(), id)
	return err
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Title, &r.Stage, &r.AssetTitle, &r.AssetReferenceCode, &r.ClientName)
	return r, err
}

// List is every Project, newest-updated first — the kanban board groups these client-side by
// Stage (see views.templ).
func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT p.id, p.title, p.stage, a.title, a.referenceCode, c.name
		FROM Project p
		JOIN Asset a ON a.id = p.assetId
		JOIN Client c ON c.id = a.clientId
		WHERE p.deletedAt IS NULL
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

// SetStage moves a Project to a new project_stage Classifier code — the kanban drag-and-drop
// handler and the detail page's stage-advance control both call this. Reaching "completed" also
// stamps completedAt once (COALESCE, never overwritten); moving off "completed" again doesn't
// clear it — CompletedAt is a historical timestamp only, Stage is what open/closed reads.
func SetStage(ctx context.Context, q studiodb.Querier, id, stage string) error {
	now := time.Now()
	if stage == "completed" {
		_, err := studiodb.Execute(ctx, q,
			"UPDATE Project SET stage = ?, completedAt = COALESCE(completedAt, ?), updatedAt = ? WHERE id = ?",
			stage, now, now, id)
		return err
	}
	_, err := studiodb.Execute(ctx, q, "UPDATE Project SET stage = ?, updatedAt = ? WHERE id = ?", stage, now, id)
	return err
}

func scanActivity(rows *sql.Rows) (Activity, error) {
	var a Activity
	err := rows.Scan(&a.ID, &a.Description, &a.StartedAt, &a.DurationMinutes, &a.ActivityTypeTitle, &a.UserName)
	return a, err
}

// ListActivities is historical Notebook data (activity logging is retired — Treatments is now
// the sole way to record conservation work — but any rows a pre-retirement database already has
// still render here, in the project export).
func ListActivities(ctx context.Context, q studiodb.Querier, projectID string) ([]Activity, error) {
	return studiodb.Query(ctx, q, `
		SELECT a.id, a.description, a.startedAt, a.durationMinutes, c.title, u.name
		FROM Activity a
		JOIN Classifier c ON c.id = a.activityTypeId
		JOIN User u ON u.id = a.userId
		WHERE a.projectId = ? ORDER BY a.startedAt DESC`, scanActivity, projectID)
}
