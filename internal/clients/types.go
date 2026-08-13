// Package clients is the Client module: profile CRUD plus the polymorphic Tag assignment UI.
// Assets/Quotes/Orders sections on the detail page query those tables directly (same as the
// original app's page.tsx, which never imported an assets/commerce domain module either) -
// their own modules (6, 9) will add the write side; until then those links 404, which is an
// expected, temporary property of building module by module rather than all at once.
package clients

import (
	"database/sql"
	"time"
)

type Client struct {
	ID                     string
	Type                   string
	Name                   string
	Email                  sql.NullString
	Phone                  sql.NullString
	Address                sql.NullString
	City                   sql.NullString
	PostalCode             sql.NullString
	Country                sql.NullString
	Notes                  sql.NullString
	OrganizationName       sql.NullString
	ContactPerson          sql.NullString
	TaxID                  sql.NullString
	PreferredContactMethod sql.NullString
	ReferralSource         sql.NullString
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type ListRow struct {
	ID         string
	Name       string
	Email      sql.NullString
	Type       string
	AssetCount int
	OrderCount int
}

type AssetSummary struct {
	ID            string
	Title         sql.NullString
	ReferenceCode string
}

type QuoteSummary struct {
	ID            string
	Status        string
	TotalEstimate float64
}

type OrderSummary struct {
	ID           string
	OrderNumber  string
	Status       string
	InvoiceCount int
}
