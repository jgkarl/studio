// Package assessments is the Assessments module: condition-status records (renamed from the old
// AssetState "condition status" quick-add) — the sole way to record an Asset's condition. Every
// Assessment belongs to a Project (which pins exactly one Asset), matching Treatments/Reports;
// still filterable by Asset directly since AssetID is denormalized onto the row.
package assessments

import (
	"database/sql"
	"time"
)

type Assessment struct {
	ID          string
	ProjectID   string
	AssetID     string
	Condition   string
	Description string
	RecordedAt  time.Time
	UpdatedAt   time.Time
	DeletedAt   sql.NullTime
}

type ListRow struct {
	ID                 string
	Condition          string
	Description        string
	RecordedAt         time.Time
	AssetTitle         sql.NullString
	AssetReferenceCode string
	ClientName         string
}

type ProjectOption struct {
	ID         string
	Title      string
	AssetTitle sql.NullString
	AssetRef   string
	ClientName string
}

// DetailRow is an Assessment plus its owning Project/Asset/Client's display fields, denormalized
// via a join rather than importing internal/workflows or internal/assets — both of those import
// this package (to render an "Assessments" section on their own detail pages), so this package
// can't import back without a cycle.
type DetailRow struct {
	Assessment
	ProjectTitle       string
	AssetTitle         sql.NullString
	AssetReferenceCode string
	ClientID           string
	ClientName         string
}
