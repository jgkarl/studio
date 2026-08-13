package clients

import (
	"context"
	"database/sql"

	studiodb "studio/internal/db"
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

// ListAllSortedByName is every Client, name-ordered - the option list for the Asset/Quote
// "pick a client" selects (modules 6, 9).
func ListAllSortedByName(ctx context.Context, q studiodb.Querier) ([]Client, error) {
	return studiodb.Query(ctx, q, "SELECT "+clientColumns+" FROM Client ORDER BY name ASC", scanClient)
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Name, &r.Email, &r.Type, &r.AssetCount, &r.OrderCount)
	return r, err
}

func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT c.id, c.name, c.email, c.type,
		       (SELECT COUNT(*) FROM Asset a WHERE a.clientId = c.id) AS assetCount,
		       (SELECT COUNT(*) FROM `+"`Order`"+` o WHERE o.clientId = c.id) AS orderCount
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3))`,
		id, in.Name, in.Type, nullIfEmpty(in.Email), nullIfEmpty(in.Phone), nullIfEmpty(in.Address),
		nullIfEmpty(in.City), nullIfEmpty(in.PostalCode), nullIfEmpty(in.Country),
		nullIfEmpty(in.OrganizationName), nullIfEmpty(in.ContactPerson), nullIfEmpty(in.TaxID),
		nullIfEmpty(in.PreferredContactMethod), nullIfEmpty(in.ReferralSource), nullIfEmpty(in.Notes))
	return id, err
}

func Update(ctx context.Context, q studiodb.Querier, id string, in Input) error {
	_, err := studiodb.Execute(ctx, q, `
		UPDATE Client SET name = COALESCE(NULLIF(?, ''), name), type = ?, email = ?, phone = ?, address = ?, city = ?,
			postalCode = ?, country = ?, organizationName = ?, contactPerson = ?, taxId = ?, preferredContactMethod = ?,
			referralSource = ?, notes = ?, updatedAt = NOW(3)
		WHERE id = ?`,
		in.Name, in.Type, nullIfEmpty(in.Email), nullIfEmpty(in.Phone), nullIfEmpty(in.Address),
		nullIfEmpty(in.City), nullIfEmpty(in.PostalCode), nullIfEmpty(in.Country),
		nullIfEmpty(in.OrganizationName), nullIfEmpty(in.ContactPerson), nullIfEmpty(in.TaxID),
		nullIfEmpty(in.PreferredContactMethod), nullIfEmpty(in.ReferralSource), nullIfEmpty(in.Notes), id)
	return err
}

// FindOrCreateByEmail looks up a Client by email, creating a minimal one (email only, type
// "individual") if none exists — the entry point for any intake path (staff-entered quote,
// future public request form) where a prospect isn't already a Client record. Email isn't
// unique in the schema (institutions may share a contact inbox), so this matches the first
// existing record rather than relying on a DB constraint. Exported for the Commerce module
// (module 9) to reuse.
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
	if _, err := studiodb.Execute(ctx, q, "INSERT INTO Client (id, email, name, type, updatedAt) VALUES (?, ?, ?, ?, NOW(3))",
		id, email, displayName, "individual"); err != nil {
		return nil, err
	}
	return GetByID(ctx, q, id)
}

// --- Detail-page read-only summaries (Assets/Quotes/Orders/Invoices) -----------------------

func scanAssetSummary(rows *sql.Rows) (AssetSummary, error) {
	var a AssetSummary
	err := rows.Scan(&a.ID, &a.Title, &a.ReferenceCode)
	return a, err
}

func ListAssetsForClient(ctx context.Context, q studiodb.Querier, clientID string) ([]AssetSummary, error) {
	return studiodb.Query(ctx, q, "SELECT id, title, referenceCode FROM Asset WHERE clientId = ? ORDER BY createdAt DESC", scanAssetSummary, clientID)
}

func scanQuoteSummary(rows *sql.Rows) (QuoteSummary, error) {
	var qt QuoteSummary
	err := rows.Scan(&qt.ID, &qt.Status, &qt.TotalEstimate)
	return qt, err
}

func ListQuotesForClient(ctx context.Context, q studiodb.Querier, clientID string) ([]QuoteSummary, error) {
	return studiodb.Query(ctx, q, "SELECT id, status, totalEstimate FROM Quote WHERE clientId = ? ORDER BY createdAt DESC", scanQuoteSummary, clientID)
}

func ListOrdersForClient(ctx context.Context, q studiodb.Querier, clientID string) ([]OrderSummary, error) {
	type row struct {
		ID, OrderNumber, Status string
	}
	scan := func(rows *sql.Rows) (row, error) {
		var r row
		err := rows.Scan(&r.ID, &r.OrderNumber, &r.Status)
		return r, err
	}
	orderRows, err := studiodb.Query(ctx, q, "SELECT id, orderNumber, status FROM `Order` WHERE clientId = ? ORDER BY createdAt DESC", scan, clientID)
	if err != nil {
		return nil, err
	}
	out := make([]OrderSummary, len(orderRows))
	for i, o := range orderRows {
		count, err := studiodb.QueryOne(ctx, q, "SELECT COUNT(*) AS n FROM Invoice WHERE orderId = ?", scanCount, o.ID)
		if err != nil {
			return nil, err
		}
		n := 0
		if count != nil {
			n = *count
		}
		out[i] = OrderSummary{ID: o.ID, OrderNumber: o.OrderNumber, Status: o.Status, InvoiceCount: n}
	}
	return out, nil
}

func scanCount(rows *sql.Rows) (int, error) {
	var n int
	err := rows.Scan(&n)
	return n, err
}
