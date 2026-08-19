package clients

import (
	"context"
	"database/sql"
	"time"

	studiodb "stuudio/internal/db"
)

const clientColumns = `id, type, name, email, phone, address, city, postalCode, country, notes,
	organizationName, contactPerson, taxId, preferredContactMethod, referralSource, createdAt, updatedAt`

func scanClient(rows *sql.Rows) (Client, error) {
	var c Client
	err := rows.Scan(&c.ID, &c.Type, &c.Name, &c.Email, &c.Phone, &c.Address, &c.City, &c.PostalCode, &c.Country,
		&c.Notes, &c.OrganizationName, &c.ContactPerson, &c.TaxID, &c.PreferredContactMethod, &c.ReferralSource,
		&c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Client, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+clientColumns+" FROM Client WHERE id = ?", scanClient, id)
}

// ListAllSortedByName is every Client, name-ordered - the option list for the Asset "pick a
// client" select.
func ListAllSortedByName(ctx context.Context, q studiodb.Querier) ([]Client, error) {
	return studiodb.Query(ctx, q, "SELECT "+clientColumns+" FROM Client ORDER BY name ASC", scanClient)
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Name, &r.Email, &r.Type, &r.AssetCount)
	return r, err
}

func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT c.id, c.name, c.email, c.type,
		       (SELECT COUNT(*) FROM Asset a WHERE a.clientId = c.id) AS assetCount
		FROM Client c ORDER BY c.createdAt DESC`, scanListRow)
}

type Input struct {
	Type                   string
	Name                   string
	Email                  string
	Phone                  string
	Address                string
	City                   string
	PostalCode             string
	Country                string
	Notes                  string
	OrganizationName       string
	ContactPerson          string
	TaxID                  string
	PreferredContactMethod string
	ReferralSource         string
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func Create(ctx context.Context, q studiodb.Querier, in Input) (string, error) {
	id := studiodb.NewID()
	_, err := studiodb.Execute(ctx, q, `
		INSERT INTO Client (id, name, type, email, phone, address, city, postalCode, country,
			organizationName, contactPerson, taxId, preferredContactMethod, referralSource, notes, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.Name, in.Type, nullIfEmpty(in.Email), nullIfEmpty(in.Phone), nullIfEmpty(in.Address),
		nullIfEmpty(in.City), nullIfEmpty(in.PostalCode), nullIfEmpty(in.Country),
		nullIfEmpty(in.OrganizationName), nullIfEmpty(in.ContactPerson), nullIfEmpty(in.TaxID),
		nullIfEmpty(in.PreferredContactMethod), nullIfEmpty(in.ReferralSource), nullIfEmpty(in.Notes), time.Now())
	return id, err
}

func Update(ctx context.Context, q studiodb.Querier, id string, in Input) error {
	_, err := studiodb.Execute(ctx, q, `
		UPDATE Client SET name = COALESCE(NULLIF(?, ''), name), type = ?, email = ?, phone = ?, address = ?, city = ?,
			postalCode = ?, country = ?, organizationName = ?, contactPerson = ?, taxId = ?, preferredContactMethod = ?,
			referralSource = ?, notes = ?, updatedAt = ?
		WHERE id = ?`,
		in.Name, in.Type, nullIfEmpty(in.Email), nullIfEmpty(in.Phone), nullIfEmpty(in.Address),
		nullIfEmpty(in.City), nullIfEmpty(in.PostalCode), nullIfEmpty(in.Country),
		nullIfEmpty(in.OrganizationName), nullIfEmpty(in.ContactPerson), nullIfEmpty(in.TaxID),
		nullIfEmpty(in.PreferredContactMethod), nullIfEmpty(in.ReferralSource), nullIfEmpty(in.Notes), time.Now(), id)
	return err
}

// FindOrCreateByEmail looks up a Client by email, creating a minimal one (email only, type
// "individual") if none exists — the entry point for any intake path where a prospect isn't
// already a Client record. Email isn't unique in the schema (institutions may share a contact
// inbox), so this matches the first existing record rather than relying on a DB constraint.
func FindOrCreateByEmail(ctx context.Context, q studiodb.Querier, email, name string) (*Client, error) {
	existing, err := studiodb.QueryOne(ctx, q, "SELECT "+clientColumns+" FROM Client WHERE email = ? LIMIT 1", scanClient, email)
	if err != nil || existing != nil {
		return existing, err
	}
	displayName := name
	if displayName == "" {
		displayName = email
	}
	id := studiodb.NewID()
	if _, err := studiodb.Execute(ctx, q, "INSERT INTO Client (id, email, name, type, updatedAt) VALUES (?, ?, ?, ?, ?)",
		id, email, displayName, "individual", time.Now()); err != nil {
		return nil, err
	}
	return GetByID(ctx, q, id)
}

// --- Detail-page read-only summaries (Assets) -----------------------------------------------

func scanAssetSummary(rows *sql.Rows) (AssetSummary, error) {
	var a AssetSummary
	err := rows.Scan(&a.ID, &a.Title, &a.ReferenceCode)
	return a, err
}

func ListAssetsForClient(ctx context.Context, q studiodb.Querier, clientID string) ([]AssetSummary, error) {
	return studiodb.Query(ctx, q, "SELECT id, title, referenceCode FROM Asset WHERE clientId = ? ORDER BY createdAt DESC", scanAssetSummary, clientID)
}
