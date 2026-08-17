// Package settings holds the admin-only management modules: the generic Classifier manager (one
// UI for every "pick from a list" field in the app) and Users admin.
package settings

import (
	"database/sql"
	"time"
)

type ClassifierType string

const (
	ClassifierClientType      ClassifierType = "client_type"
	ClassifierContactMethod   ClassifierType = "contact_method"
	ClassifierAssetType       ClassifierType = "asset_type"
	ClassifierConditionState  ClassifierType = "condition_state"
	ClassifierActivityType    ClassifierType = "activity_type"
	ClassifierProjectStage    ClassifierType = "project_stage"
	ClassifierPriority        ClassifierType = "priority"
	ClassifierTreatmentMethod ClassifierType = "treatment_method"
)

// ClassifierTypes is the display order for the /settings/classifiers index.
var ClassifierTypes = []ClassifierType{
	ClassifierClientType,
	ClassifierContactMethod,
	ClassifierAssetType,
	ClassifierConditionState,
	ClassifierActivityType,
	ClassifierProjectStage,
	ClassifierPriority,
	ClassifierTreatmentMethod,
}

var ClassifierTypeLabels = map[ClassifierType]string{
	ClassifierClientType:      "Client Types",
	ClassifierContactMethod:   "Contact Methods",
	ClassifierAssetType:       "Asset Types",
	ClassifierConditionState:  "Condition States",
	ClassifierActivityType:    "Activity Types (Notebook)",
	ClassifierProjectStage:    "Project Stages",
	ClassifierPriority:        "Priority",
	ClassifierTreatmentMethod: "Treatment Methods",
}

func IsValidClassifierType(t string) bool {
	_, ok := ClassifierTypeLabels[ClassifierType(t)]
	return ok
}

// SettingsManagedTypes is the flat Settings screen's chip-group order: the design artifact's 5
// headline groups (Asset Types, Condition States, Treatment Methods, Project Stages, Priority)
// first, then Client Types and Contact Methods — real, still-used picklists (the New Client
// form's Type/Preferred-contact-method fields) that aren't part of the mockup's 5 but would lose
// their only admin UI if dropped entirely. activity_type is deliberately excluded: the Activity
// Notebook it fed is retired (Treatments replaced it), so nothing creates activity_type rows
// through the app anymore.
var SettingsManagedTypes = []ClassifierType{
	ClassifierAssetType,
	ClassifierConditionState,
	ClassifierTreatmentMethod,
	ClassifierProjectStage,
	ClassifierPriority,
	ClassifierClientType,
	ClassifierContactMethod,
}

func IsSettingsManagedType(t ClassifierType) bool {
	for _, mt := range SettingsManagedTypes {
		if mt == t {
			return true
		}
	}
	return false
}

// ClassifierGroup is one chip-group on the flat Settings page.
type ClassifierGroup struct {
	Type  ClassifierType
	Label string
	Rows  []Classifier
}

type Classifier struct {
	ID          string
	Type        ClassifierType
	Code        string
	Sequence    int
	Title       string
	Description sql.NullString
	Data        sql.NullString // raw JSON text, stored/read verbatim - never decoded server-side
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
