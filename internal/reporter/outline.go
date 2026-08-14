package reporter

import (
	"context"
	"database/sql"
	"encoding/json"

	studiodb "studio/internal/db"
)

func heading(text string, level int) map[string]any {
	return map[string]any{
		"type":    "heading",
		"attrs":   map[string]any{"level": level},
		"content": []any{map[string]any{"type": "text", "text": text}},
	}
}

func paragraph(text string) map[string]any {
	content := []any{}
	if text != "" {
		content = []any{map[string]any{"type": "text", "text": text}}
	}
	return map[string]any{"type": "paragraph", "content": content}
}

func emptyDoc() string {
	b, _ := json.Marshal(map[string]any{"type": "doc", "content": []any{}})
	return string(b)
}

// BuildSuggestedOutline pre-fills a Report's TipTap JSON content from the project's logged
// Notebook data - intake description -> condition on arrival -> treatment performed -> condition
// after treatment. The conservator edits from there; nothing here is final.
func BuildSuggestedOutline(ctx context.Context, q studiodb.Querier, projectID string) (string, error) {
	type projectRow struct {
		Title   string
		AssetID string
	}
	project, err := studiodb.QueryOne(ctx, q, "SELECT title, assetId FROM Project WHERE id = ?",
		func(rows *sql.Rows) (projectRow, error) {
			var p projectRow
			err := rows.Scan(&p.Title, &p.AssetID)
			return p, err
		}, projectID)
	if err != nil {
		return "", err
	}
	if project == nil {
		return emptyDoc(), nil
	}

	type assetRow struct {
		Title         sql.NullString
		ReferenceCode string
	}
	asset, err := studiodb.QueryOne(ctx, q, "SELECT title, referenceCode FROM Asset WHERE id = ?",
		func(rows *sql.Rows) (assetRow, error) {
			var a assetRow
			err := rows.Scan(&a.Title, &a.ReferenceCode)
			return a, err
		}, project.AssetID)
	if err != nil {
		return "", err
	}
	assetName := project.AssetID
	if asset != nil {
		assetName = asset.ReferenceCode
		if asset.Title.Valid && asset.Title.String != "" {
			assetName = asset.Title.String
		}
	}

	type activityRow struct {
		Description       string
		ActivityTypeTitle string
	}
	activities, err := studiodb.Query(ctx, q, `
		SELECT a.description, c.title FROM Activity a JOIN Classifier c ON c.id = a.activityTypeId
		WHERE a.projectId = ? ORDER BY a.startedAt ASC`,
		func(rows *sql.Rows) (activityRow, error) {
			var a activityRow
			err := rows.Scan(&a.Description, &a.ActivityTypeTitle)
			return a, err
		}, projectID)
	if err != nil {
		return "", err
	}

	type stateRow struct {
		ID          string
		Description string
	}
	states, err := studiodb.Query(ctx, q, `SELECT id, description FROM AssetState WHERE projectId = ? ORDER BY recordedAt ASC`,
		func(rows *sql.Rows) (stateRow, error) {
			var s stateRow
			err := rows.Scan(&s.ID, &s.Description)
			return s, err
		}, projectID)
	if err != nil {
		return "", err
	}

	content := []any{
		heading("Conservation Report — "+assetName, 1),
		paragraph("Workflow: " + project.Title),
	}

	var intake *stateRow
	if len(states) > 0 {
		intake = &states[0]
		content = append(content, heading("Condition on arrival", 2), paragraph(intake.Description))
	}

	if len(activities) > 0 {
		content = append(content, heading("Treatment performed", 2))
		for _, a := range activities {
			content = append(content, paragraph(a.ActivityTypeTitle+": "+a.Description))
		}
	}

	if len(states) > 0 {
		final := states[len(states)-1]
		if intake == nil || final.ID != intake.ID {
			content = append(content, heading("Condition after treatment", 2), paragraph(final.Description))
		}
	}

	b, err := json.Marshal(map[string]any{"type": "doc", "content": content})
	return string(b), err
}
