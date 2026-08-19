package settings

import (
	"context"

	studiodb "studio/internal/db"
)

// FeatureLimit is one Dashboard display cap configurable from the Settings "Features" fieldset —
// how many rows of each module show up in an active-project card, and how many active-project
// cards show up at all.
type FeatureLimit struct {
	Key     string
	Label   string
	Default int
}

var DashboardLimits = []FeatureLimit{
	{Key: "dashboard.active_projects.limit", Label: "Active projects shown", Default: 5},
	{Key: "dashboard.assessments.limit", Label: "Assessments per project card", Default: 5},
	{Key: "dashboard.treatments.limit", Label: "Treatments per project card", Default: 5},
	{Key: "dashboard.reports.limit", Label: "Reports per project card", Default: 5},
}

// FeatureLimitValue is one DashboardLimits entry plus its current (possibly still-default) value
// — the Settings "Features" fieldset renders one numeric input per entry.
type FeatureLimitValue struct {
	FeatureLimit
	Value int
}

func LoadDashboardLimits(ctx context.Context, q studiodb.Querier) []FeatureLimitValue {
	out := make([]FeatureLimitValue, len(DashboardLimits))
	for i, l := range DashboardLimits {
		out[i] = FeatureLimitValue{FeatureLimit: l, Value: GetInt(ctx, q, l.Key, l.Default)}
	}
	return out
}
