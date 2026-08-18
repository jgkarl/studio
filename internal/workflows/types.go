// Package workflows is the Projects module (route-facing name "Projects" — see cmd/server/
// main.go and internal/web/navbar.templ; the Go package keeps its original name): a Project
// moves an Asset through a fixed 5-stage pipeline (project_stage Classifier: inquiry, queue,
// working, review, completed) shown as a kanban board. Stage — not CompletedAt — is the single
// source of truth for open/closed; CompletedAt is still set (once) when a Project first reaches
// the "completed" stage, kept only as a timestamp for display/export, not read for that check.
package workflows

import (
	"database/sql"
	"time"
)

type Project struct {
	ID               string
	AssetID          string
	Title            string
	Stage            string
	Priority         string
	TargetReviewDate sql.NullTime
	AssignedToUserID sql.NullString
	StartedAt        sql.NullTime
	CompletedAt      sql.NullTime
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        sql.NullTime
}

type ListRow struct {
	ID                 string
	Title              string
	Stage              string
	AssetTitle         sql.NullString
	AssetReferenceCode string
	ClientName         string
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
