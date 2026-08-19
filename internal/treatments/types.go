// Package treatments is the Treatments module: the sole way to record conservation work
// performed on an Asset (the Activity Notebook — internal/workflows's old activity log — is
// retired; Projects now only track pipeline stage). Asset-scoped only, no Project relation,
// matching the design artifact's Treatment data.
package treatments

import (
	"database/sql"
	"time"
)

type Treatment struct {
	ID                string
	ProjectID         string
	AssetID           string
	Method            string
	Title             string
	Notes             string
	PerformedByUserID sql.NullString
	PerformedAt       time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         sql.NullTime
}

type ListRow struct {
	ID                 string
	Title              string
	Method             string
	PerformedAt        time.Time
	AssetTitle         sql.NullString
	AssetReferenceCode string
	ClientName         string
	ProjectID          string
	ProjectTitle       string
}

// ProjectOption is one entry in the "pick a project" select on the new-treatment form — every
// Project pins exactly one Asset, so this doubles as the asset picker.
type ProjectOption struct {
	ID         string
	Title      string
	AssetTitle sql.NullString
	AssetRef   string
	ClientName string
}

// DetailRow is a Treatment plus its owning Asset/Client's display fields, denormalized via a
// join rather than importing internal/assets or internal/clients — internal/assets imports this
// package (to render a "Treatments" section on the Asset detail page), so this package can't
// import back without a cycle.
type DetailRow struct {
	Treatment
	ProjectTitle       string
	AssetTitle         sql.NullString
	AssetReferenceCode string
	ClientID           string
	ClientName         string
}
