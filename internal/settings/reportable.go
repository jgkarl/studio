package settings

import (
	"context"

	studiodb "studio/internal/db"
)

// ReportableField is one Client/Asset/Project description field that can be toggled on/off for
// inclusion in a Report's Client/Asset/Project info panel — a static, curated list (not every
// column; things like ids or timestamps aren't meaningful to toggle) rather than a fully dynamic
// reflection-based system.
type ReportableField struct {
	Model string // "client" | "asset" | "project" — matches reporter's info-panel lookup
	Field string
	Label string
}

var ReportableFields = []ReportableField{
	{Model: "client", Field: "name", Label: "Name"},
	{Model: "client", Field: "organizationName", Label: "Organization"},
	{Model: "client", Field: "contactPerson", Label: "Contact person"},
	{Model: "client", Field: "email", Label: "Email"},
	{Model: "client", Field: "phone", Label: "Phone"},
	{Model: "client", Field: "address", Label: "Address"},
	{Model: "client", Field: "notes", Label: "Notes"},

	{Model: "asset", Field: "title", Label: "Title"},
	{Model: "asset", Field: "artist", Label: "Artist"},
	{Model: "asset", Field: "creationPeriod", Label: "Creation period"},
	{Model: "asset", Field: "dimensions", Label: "Dimensions"},
	{Model: "asset", Field: "medium", Label: "Medium"},
	{Model: "asset", Field: "description", Label: "Description"},
	{Model: "asset", Field: "provenance", Label: "Provenance"},
	{Model: "asset", Field: "signatureMarks", Label: "Signature / marks"},
	{Model: "asset", Field: "estimatedValue", Label: "Estimated value"},

	{Model: "project", Field: "title", Label: "Title"},
	{Model: "project", Field: "priority", Label: "Priority"},
	{Model: "project", Field: "targetReviewDate", Label: "Target review date"},
	{Model: "project", Field: "assignedTo", Label: "Assigned to"},
}

// ReportableModels is ReportableGroup's display order on the Settings "Reportable fields"
// fieldset.
var ReportableModels = []struct{ Model, Label string }{
	{"client", "Client"},
	{"asset", "Asset"},
	{"project", "Project"},
}

func reportableKey(model, field string) string {
	return "reportable." + model + "." + field
}

// IsReportable reports whether one field should appear in a Report's info panel — defaults to
// true when no AppSetting row exists yet, so nothing silently disappears from reports on upgrade.
func IsReportable(ctx context.Context, q studiodb.Querier, model, field string) bool {
	return GetBool(ctx, q, reportableKey(model, field), true)
}

func SetReportable(ctx context.Context, q studiodb.Querier, model, field string, enabled bool) error {
	return SetBool(ctx, q, reportableKey(model, field), enabled)
}

// ReportableFieldState is one ReportableFields entry plus its current on/off state — the
// Settings "Reportable fields" fieldset renders one checkbox per entry.
type ReportableFieldState struct {
	ReportableField
	Enabled bool
}

type ReportableGroup struct {
	Model  string
	Label  string
	Fields []ReportableFieldState
}

func LoadReportableGroups(ctx context.Context, q studiodb.Querier) []ReportableGroup {
	groups := make([]ReportableGroup, len(ReportableModels))
	for i, m := range ReportableModels {
		var fields []ReportableFieldState
		for _, f := range ReportableFields {
			if f.Model != m.Model {
				continue
			}
			fields = append(fields, ReportableFieldState{ReportableField: f, Enabled: IsReportable(ctx, q, f.Model, f.Field)})
		}
		groups[i] = ReportableGroup{Model: m.Model, Label: m.Label, Fields: fields}
	}
	return groups
}
