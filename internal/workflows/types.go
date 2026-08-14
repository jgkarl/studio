// Package workflows is the Workflows (Notebook) module: a Project moves an Asset through a fixed
// conservation-work stage machine, with an Activity log (the Notebook) as its spine — state
// history, and eventually pricing/report outlines (modules 9, 10), are derived from these logged
// entries rather than re-entered elsewhere.
package workflows

import (
	"database/sql"
	"time"
)

type Stage string

const (
	StageIngest             Stage = "ingest"
	StageDescribe           Stage = "describe"
	StageFixateCurrentState Stage = "fixate_current_state"
	StageTreatment          Stage = "treatment"
	StageWaiting            Stage = "waiting"
	StageFixateNewState     Stage = "fixate_new_state"
	StageHandoverDone       Stage = "handover_done"
)

// Stages is the fixed display order every stage-badge trail renders in.
var Stages = []Stage{
	StageIngest, StageDescribe, StageFixateCurrentState, StageTreatment,
	StageWaiting, StageFixateNewState, StageHandoverDone,
}

var StageLabels = map[Stage]string{
	StageIngest:             "Ingest",
	StageDescribe:           "Describe / Assess",
	StageFixateCurrentState: "Fixate Current State",
	StageTreatment:          "Treatment",
	StageWaiting:            "Waiting / Drying",
	StageFixateNewState:     "Fixate New State",
	StageHandoverDone:       "Handover / Done",
}

// StageTransitions are the allowed forward moves. "treatment" and "waiting" can repeat
// (self-loop) before moving on — this is a transition graph (real workflow logic), not a flat
// pickable option list, so it's kept as code rather than a Classifier.
var StageTransitions = map[Stage][]Stage{
	StageIngest:             {StageDescribe},
	StageDescribe:           {StageFixateCurrentState},
	StageFixateCurrentState: {StageTreatment},
	StageTreatment:          {StageTreatment, StageWaiting, StageFixateNewState},
	StageWaiting:            {StageTreatment, StageWaiting, StageFixateNewState},
	StageFixateNewState:     {StageHandoverDone},
	StageHandoverDone:       {},
}

type Project struct {
	ID               string
	OrderID          sql.NullString
	AssetID          string
	Title            string
	Stage            Stage
	AssignedToUserID sql.NullString
	StartedAt        sql.NullTime
	CompletedAt      sql.NullTime
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ListRow struct {
	ID                 string
	Title              string
	Stage              Stage
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
