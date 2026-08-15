// Package workflows is the Workflows (Notebook) module: a Project moves an Asset through a fixed
// conservation-work stage machine, with an Activity log (the Notebook) as its spine — state
// history, and eventually pricing/report outlines (modules 9, 10), are derived from these logged
// entries rather than re-entered elsewhere.
package workflows

import (
	"database/sql"
	"time"
)

// A Project is a single unit of conservation work on an Asset. It has no granular workflow
// stage — just a Notebook (Activity log, see below) and a simple open/completed status driven by
// CompletedAt (set once, via CompleteProject — see queries.go).
type Project struct {
	ID               string
	OrderID          sql.NullString
	AssetID          string
	Title            string
	AssignedToUserID sql.NullString
	StartedAt        sql.NullTime
	CompletedAt      sql.NullTime
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ListRow struct {
	ID                 string
	Title              string
	CompletedAt        sql.NullTime
	AssetTitle         sql.NullString
	AssetReferenceCode string
	ClientName         string
	OrderNumber        sql.NullString
}

type AssetOption struct {
	ID            string
	Title         sql.NullString
	ReferenceCode string
	ClientName    string
}

type Activity struct {
	ID                string
	Description       string
	StartedAt         time.Time
	DurationMinutes   sql.NullInt64
	ActivityTypeTitle string
	UserName          string
}
