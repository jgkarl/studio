// Package reporter is the Reports module: composes the final conservation write-up as three plain
// structured sections (Description, Summary, Recommendations) plus a customize-layout sidebar
// (per-section show/hide including the cover, and the image gallery's own column/order layout).
// The old TipTap rich-text editor is retired — Content still exists on Report (old data, never
// lost) but new reports don't write to it; likewise conditionFindings/treatmentPerformed/
// materialsUsed stay in the schema (old report data isn't lost) but are no longer shown or
// editable — Description replaces them going forward.
package reporter

import (
	"database/sql"
	"time"
)

type Report struct {
	ID        string
	ProjectID string
	AssetID   string
	Title     string
	Content   string // raw TipTap JSON doc from before the structured-sections rework — opaque, unused by new reports
	Status    string
	AuthorID  sql.NullString

	Description sql.NullString

	Summary             sql.NullString
	ConditionFindings   sql.NullString // retired from the editing UI — see package doc comment
	TreatmentPerformed  sql.NullString // retired from the editing UI — see package doc comment
	MaterialsUsed       sql.NullString // retired from the editing UI — see package doc comment
	Recommendations     sql.NullString
	CoverMediaID        sql.NullString
	LayoutStyle         string
	GalleryColumns      int
	ShowCover           bool
	ShowDescription     bool
	ShowSummary         bool
	ShowCondition       bool // retired from the editing UI — see package doc comment
	ShowTreatment       bool // retired from the editing UI — see package doc comment
	ShowMaterials       bool // retired from the editing UI — see package doc comment
	ShowRecommendations bool

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt sql.NullTime
}

// IsRemoved reports whether Unlink has soft-deleted this report — still a real row (see Unlink's
// doc comment), just hidden from every list by default (ListPage's own "Removed" filter chip is
// the one place it can still be found).
func (r Report) IsRemoved() bool {
	return r.DeletedAt.Valid
}

type ListRow struct {
	ID                 string
	Title              string
	Status             string
	AssetTitle         sql.NullString
	AssetReferenceCode string
	ClientName         string
	AuthorName         sql.NullString
	ProjectID          string
	ProjectTitle       string
	Removed            bool
}

// ProjectOption is one entry in the "pick a project" select on the new-report form — every
// Project pins exactly one Asset, so this doubles as the asset picker.
type ProjectOption struct {
	ID         string
	Title      string
	AssetTitle sql.NullString
	AssetRef   string
	ClientName string
}

// SectionsInput is the three structured-section textareas, saved together as one form post.
type SectionsInput struct {
	Description     string
	Summary         string
	Recommendations string
}

// LayoutInput is the "Customize layout" card: export layout style, cover image, and per-section
// visibility. The image gallery's own layout (column count, per-image order/stretch) is set
// directly on the gallery itself, not here — see SetGalleryColumns/ReorderGallery/
// SetGalleryItemStretch.
type LayoutInput struct {
	LayoutStyle         string
	CoverMediaID        string
	ShowCover           bool
	ShowDescription     bool
	ShowSummary         bool
	ShowRecommendations bool
}
