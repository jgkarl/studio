// Package settings holds the admin-only management modules: the generic Classifier manager (one
// UI for every "pick from a list" field in the app) and Tags. Users admin lands here too in
// module 12.
package settings

import (
	"database/sql"
	"time"
)

type ClassifierType string

const (
	ClassifierClientType     ClassifierType = "client_type"
	ClassifierContactMethod  ClassifierType = "contact_method"
	ClassifierAssetType      ClassifierType = "asset_type"
	ClassifierMaterial       ClassifierType = "material"
	ClassifierConditionState ClassifierType = "condition_state"
	ClassifierActivityType   ClassifierType = "activity_type"
	ClassifierOrderStatus    ClassifierType = "order_status"
	ClassifierQuoteStatus    ClassifierType = "quote_status"
	ClassifierInvoiceStatus  ClassifierType = "invoice_status"
)

// ClassifierTypes is the display order for the /settings/classifiers index.
var ClassifierTypes = []ClassifierType{
	ClassifierClientType,
	ClassifierContactMethod,
	ClassifierAssetType,
	ClassifierMaterial,
	ClassifierConditionState,
	ClassifierActivityType,
	ClassifierOrderStatus,
	ClassifierQuoteStatus,
	ClassifierInvoiceStatus,
}

var ClassifierTypeLabels = map[ClassifierType]string{
	ClassifierClientType:     "Client Types",
	ClassifierContactMethod:  "Contact Methods",
	ClassifierAssetType:      "Asset Types",
	ClassifierMaterial:       "Materials",
	ClassifierConditionState: "Condition States",
	ClassifierActivityType:   "Activity Types (Notebook)",
	ClassifierOrderStatus:    "Order Statuses (Orders kanban)",
	ClassifierQuoteStatus:    "Quote Statuses",
	ClassifierInvoiceStatus:  "Invoice Statuses",
}

func IsValidClassifierType(t string) bool {
	_, ok := ClassifierTypeLabels[ClassifierType(t)]
	return ok
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

type Tag struct {
	ID       string
	Name     string
	Category sql.NullString
	Sequence int
}

// TagUsage is a Tag plus its TagAssignment counts, grouped by taggableType — the Settings → Tags
// overview.
type TagUsage struct {
	Tag
	ByType map[string]int
	Total  int
}
