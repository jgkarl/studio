package commerce

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"

	studiodb "studio/internal/db"
)

// EstimateProjectCost sums logged Activity time x the activity type Classifier's
// data.defaultRate for a project - the "suggested" estimate surfaced when drafting a Quote/
// Invoice. Staff review and confirm the number; nothing here auto-bills.
func EstimateProjectCost(ctx context.Context, q studiodb.Querier, projectID string) (items []LineItem, total float64, err error) {
	type activityRow struct {
		Description     string
		DurationMinutes sql.NullInt64
		ActivityTitle   string
		ActivityData    sql.NullString
	}
	scan := func(rows *sql.Rows) (activityRow, error) {
		var a activityRow
		err := rows.Scan(&a.Description, &a.DurationMinutes, &a.ActivityTitle, &a.ActivityData)
		return a, err
	}
	activities, err := studiodb.Query(ctx, q, `
		SELECT a.description, a.durationMinutes, c.title, c.data
		FROM Activity a JOIN Classifier c ON c.id = a.activityTypeId
		WHERE a.projectId = ?`, scan, projectID)
	if err != nil {
		return nil, 0, err
	}

	for _, a := range activities {
		if !a.DurationMinutes.Valid || a.DurationMinutes.Int64 == 0 {
			continue
		}
		var data struct {
			DefaultRate float64 `json:"defaultRate"`
		}
		if a.ActivityData.Valid {
			_ = json.Unmarshal([]byte(a.ActivityData.String), &data)
		}
		if data.DefaultRate == 0 {
			continue
		}
		hours := round2(float64(a.DurationMinutes.Int64) / 60)
		desc := a.ActivityTitle + " — " + truncate(a.Description, 60)
		amount := round2(hours * data.DefaultRate)
		items = append(items, LineItem{Description: desc, EstimatedHours: hours, Rate: data.DefaultRate, Amount: amount})
		total += amount
	}
	return items, round2(total), nil
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
