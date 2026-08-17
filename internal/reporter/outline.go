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

// Sections is a suggested starting point for a new Report's structured fields, built from an
// Asset's AssetState (condition) history and Treatment records — the conservator edits from
// there; nothing here is final. Summary/MaterialsUsed/Recommendations have no automatic source
// and are left for the conservator to fill in.
type Sections struct {
	ConditionFindings  string
	TreatmentPerformed string
}

// BuildSuggestedOutline pre-fills a new Report's structured sections from the Asset's existing
// data: condition findings from its AssetState history (intake vs. most recent), treatment
// performed from its Treatment records (internal/treatments — the Activity Notebook this used to
// read from is retired, see Phase 3).
func BuildSuggestedOutline(ctx context.Context, q studiodb.Querier, assetID string) (Sections, error) {
	type stateRow struct {
		ID          string
		Description string
	}
	states, err := studiodb.Query(ctx, q,
		`SELECT id, description FROM AssetState WHERE assetId = ? ORDER BY recordedAt ASC`,
		func(rows *sql.Rows) (stateRow, error) {
			var s stateRow
			err := rows.Scan(&s.ID, &s.Description)
			return s, err
		}, assetID)
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
		WHERE t.assetId = ? ORDER BY t.performedAt ASC`,
		func(rows *sql.Rows) (treatmentRow, error) {
			var tr treatmentRow
			err := rows.Scan(&tr.MethodTitle, &tr.Notes)
			return tr, err
		}, assetID)
	if err != nil {
		return Sections{}, err
	}

	var conditionFindings string
	if len(states) > 0 {
		lines := []string{"On arrival: " + states[0].Description}
		if len(states) > 1 {
			lines = append(lines, "Most recent: "+states[len(states)-1].Description)
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
