// Package reporter is the Reports module: composes the final conservation write-up as five plain
// structured sections (Summary, Condition findings, Treatment performed, Materials used,
// Recommendations) plus a customize-layout sidebar (layout style + per-section show/hide,
// including the cover). The old TipTap rich-text editor is retired — Content still exists on
// Report (old data, never lost) but new reports don't write to it.
package reporter

import (
	"database/sql"
	"time"
)

type Report struct {
	ID        string
	ProjectID sql.NullString
	AssetID   string
	Title     string
	Content   string // raw TipTap JSON doc from before the structured-sections rework — opaque, unused by new reports
	Status    string
	AuthorID  sql.NullString

	Summary             sql.NullString
	ConditionFindings   sql.NullString
	TreatmentPerformed  sql.NullString
	MaterialsUsed       sql.NullString
	Recommendations     sql.NullString
	CoverMediaID        sql.NullString
	LayoutStyle         string
	ShowCover           bool
	ShowSummary         bool
	ShowCondition       bool
	ShowTreatment       bool
	ShowMaterials       bool
	ShowRecommendations bool

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt sql.NullTime
}

type ListRow struct {
	ID                 string
	Title              string
	Status             string
	AssetTitle         sql.NullString
	AssetReferenceCode string
	ClientName         string
	AuthorName         sql.NullString
}

type AssetOption struct {
	ID            string
	Title         sql.NullString
	ReferenceCode string
	ClientName    string
}

type ProjectOption struct {
	ID    string
	Title string
}

// SectionsInput is the five structured-section textareas, saved together as one form post.
type SectionsInput struct {
	Summary            string
	ConditionFindings  string
	TreatmentPerformed string
	MaterialsUsed      string
	Recommendations    string
}

// LayoutInput is the "Customize layout" sidebar: layout style plus per-section visibility.
type LayoutInput struct {
	LayoutStyle         string
	CoverMediaID        string
	ShowCover           bool
	ShowSummary         bool
	ShowCondition       bool
	ShowTreatment       bool
	ShowMaterials       bool
	ShowRecommendations bool
}
