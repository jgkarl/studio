// Package commerce is the Quote -> Order -> Invoice module — folded into the Clients module's
// URL space (/clients/quotes, /clients/orders) since every commercial document belongs to a
// client, same as the original app, even though it's its own Go package/module here.
package commerce

import (
	"database/sql"
	"time"
)

type LineItem struct {
	Description    string  `json:"description"`
	EstimatedHours float64 `json:"estimatedHours,omitempty"`
	Rate           float64 `json:"rate,omitempty"`
	Amount         float64 `json:"amount"`
}

type Quote struct {
	ID            string
	ClientID      string
	Status        string
	LineItems     []LineItem
	TotalEstimate float64
	ValidUntil    sql.NullTime
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type QuoteListRow struct {
	ID            string
	Status        string
	TotalEstimate float64
	CreatedAt     time.Time
	ClientName    string
	OrderID       sql.NullString
	OrderNumber   sql.NullString
}

type Order struct {
	ID          string
	ClientID    string
	QuoteID     sql.NullString
	OrderNumber string
	Status      string
	Notes       sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OrderListRow struct {
	ID           string
	OrderNumber  string
	Status       string
	ClientName   string
	ProjectCount int
}

type Invoice struct {
	ID        string
	OrderID   string
	Status    string
	LineItems []LineItem
	Total     float64
	Currency  string
	IssuedAt  sql.NullTime
	DueAt     sql.NullTime
	PaidAt    sql.NullTime
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProjectOnOrder struct {
	ID                 string
	Title              string
	Stage              string
	AssetTitle         sql.NullString
	AssetReferenceCode string
}

type UnattachedProject struct {
	ID                 string
	Title              string
	AssetTitle         sql.NullString
	AssetReferenceCode string
}
