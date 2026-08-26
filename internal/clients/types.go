// Package clients is the Client module: profile CRUD. The Assets section on the detail page
// queries that table directly rather than importing an assets domain module.
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
}

type AssetSummary struct {
	ID            string
	Title         sql.NullString
	ReferenceCode string
}
