// Package settings holds the admin-only management modules: the generic Classifier manager (one
// UI for every "pick from a list" field in the app) and Users admin.
package settings

import (
	"database/sql"
	"time"

	"studio/internal/i18n"
	"studio/internal/web"
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
	ClassifierAnnotationType  ClassifierType = "annotation_type"
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
	ClassifierAnnotationType,
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
	ClassifierAnnotationType:  "Annotation Types (Pattern Layer)",
}

func IsValidClassifierType(t string) bool {
	_, ok := ClassifierTypeLabels[ClassifierType(t)]
	return ok
}

// ClassifierLabel is the single call site every display of a classifier's name should route
// through — falls back to the (English) Title when TitleEt is empty and locale is "et", so
// partially-translated data never renders blank.
func ClassifierLabel(c Classifier, locale i18n.Locale) string {
	if locale == i18n.LocaleET && c.TitleEt.Valid && c.TitleEt.String != "" {
		return c.TitleEt.String
	}
	return c.Title
}

// ClassifierFilterOptions projects Classifier rows to web.FilterOption for
// web.EntityListFilterSelect — Value is the stable code (matches a list row's data-el-* filter
// attribute), Label is locale-resolved via ClassifierLabel.
func ClassifierFilterOptions(classifiers []Classifier, locale i18n.Locale) []web.FilterOption {
	out := make([]web.FilterOption, len(classifiers))
	for i, c := range classifiers {
		out[i] = web.FilterOption{Value: c.Code, Label: ClassifierLabel(c, locale)}
	}
	return out
}

// ClassifierAutocompleteOptions projects Classifier rows to web.ClassifierOption for
// web.ClassifierAutocomplete — Title is locale-resolved via ClassifierLabel, ID/Code carried
// through so the field can bind by either depending on what the target column stores.
func ClassifierAutocompleteOptions(classifiers []Classifier, locale i18n.Locale) []web.ClassifierOption {
	out := make([]web.ClassifierOption, len(classifiers))
	for i, c := range classifiers {
		out[i] = web.ClassifierOption{ID: c.ID, Code: c.Code, Title: ClassifierLabel(c, locale)}
	}
	return out
}

// SettingsManagedTypes is the flat Settings screen's chip-group order: the design artifact's 5
// headline groups (Asset Types, Condition States, Treatment Methods, Project Stages, Priority)
// first, then Client Types and Contact Methods — real, still-used picklists (the New Client
// form's Type/Preferred-contact-method fields) that aren't part of the mockup's 5 but would lose
// their only admin UI if dropped entirely — and finally Annotation Types, whose color/hatch pair
// (internal/media/annotations.go's AnnotationTypeData) is only actually editable through this
// page's annotationTypePill/annotationSwatchFields (see views.templ). activity_type is
// deliberately excluded: the Activity Notebook it fed is retired (Treatments replaced it), so
// nothing creates activity_type rows through the app anymore.
var SettingsManagedTypes = []ClassifierType{
	ClassifierAssetType,
	ClassifierConditionState,
	ClassifierTreatmentMethod,
	ClassifierProjectStage,
	ClassifierPriority,
	ClassifierClientType,
	ClassifierContactMethod,
	ClassifierAnnotationType,
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
	ID            string
	Type          ClassifierType
	Code          string
	Sequence      int
	Title         string
	TitleEt       sql.NullString
	Description   sql.NullString
	DescriptionEt sql.NullString
	Data          sql.NullString // raw JSON text, stored/read verbatim - never decoded server-side
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
