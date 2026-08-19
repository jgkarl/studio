package reporter

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	studiodb "studio/internal/db"
)

// emptyDoc is the (now-unused-by-new-reports) empty TipTap document, kept only so Report.Content
// stays valid JSON for any code that still expects that — see types.go's Content field doc.
func emptyDoc() string {
	b, _ := json.Marshal(map[string]any{"type": "doc", "content": []any{}})
	return string(b)
}

// Sections is a suggested starting point for a new Report's structured fields, built from a
// Project's Assessment (condition) history and Treatment records — the conservator edits from
// there; nothing here is final. Summary/MaterialsUsed/Recommendations have no automatic source
// and are left for the conservator to fill in.
type Sections struct {
	ConditionFindings  string
	TreatmentPerformed string
}

// BuildSuggestedOutline pre-fills a new Report's structured sections from the Project's existing
// data: condition findings from its Assessment history (first vs. most recent), treatment
// performed from its Treatment records — both now Project-scoped (Phase 3 of the Project-scope
// refactor), not the whole Asset's lifetime history, so a report drafted for one project doesn't
// pull in another project's unrelated work on the same object.
func BuildSuggestedOutline(ctx context.Context, q studiodb.Querier, projectID string) (Sections, error) {
	type assessmentRow struct {
		ID          string
		Description string
	}
	assessmentRows, err := studiodb.Query(ctx, q,
		`SELECT id, description FROM Assessment WHERE projectId = ? AND deletedAt IS NULL ORDER BY recordedAt ASC`,
		func(rows *sql.Rows) (assessmentRow, error) {
			var s assessmentRow
			err := rows.Scan(&s.ID, &s.Description)
			return s, err
		}, projectID)
	if err != nil {
		return Sections{}, err
	}

	type treatmentRow struct {
		MethodTitle string
		Notes       string
	}
	treatmentRows, err := studiodb.Query(ctx, q, `
		SELECT c.title, t.notes FROM Treatment t
		JOIN Classifier c ON c.type = 'treatment_method' AND c.code = t.method
		WHERE t.projectId = ? AND t.deletedAt IS NULL ORDER BY t.performedAt ASC`,
		func(rows *sql.Rows) (treatmentRow, error) {
			var tr treatmentRow
			err := rows.Scan(&tr.MethodTitle, &tr.Notes)
			return tr, err
		}, projectID)
	if err != nil {
		return Sections{}, err
	}

	var conditionFindings string
	if len(assessmentRows) > 0 {
		lines := []string{"On arrival: " + assessmentRows[0].Description}
		if len(assessmentRows) > 1 {
			lines = append(lines, "Most recent: "+assessmentRows[len(assessmentRows)-1].Description)
		}
		conditionFindings = strings.Join(lines, "\n")
	}

	var treatmentPerformed string
	if len(treatmentRows) > 0 {
		lines := make([]string, len(treatmentRows))
		for i, tr := range treatmentRows {
			lines[i] = tr.MethodTitle + ": " + tr.Notes
		}
		treatmentPerformed = strings.Join(lines, "\n")
	}

	return Sections{ConditionFindings: conditionFindings, TreatmentPerformed: treatmentPerformed}, nil
}

// CreateAutoDraft creates a Report for a Project with a system-chosen title and its suggested
// outline pre-filled — used both by the "New Report" form's smart defaults (via handleCreate,
// which calls BuildSuggestedOutline itself) and by the Finish-project flow (internal/workflows),
// which needs a Report to exist as a side effect of finishing rather than a user filling out the
// New Report form.
func CreateAutoDraft(ctx context.Context, pool *sql.DB, projectID, projectTitle, authorID string) (string, error) {
	sections, err := BuildSuggestedOutline(ctx, pool, projectID)
	if err != nil {
		return "", err
	}
	return Create(ctx, pool, projectID, "Report — "+projectTitle, authorID, sections)
}
