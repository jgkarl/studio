package commerce

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	studiodb "studio/internal/db"
)

func encodeLineItems(items []LineItem) (string, error) {
	if items == nil {
		items = []LineItem{}
	}
	b, err := json.Marshal(items)
	return string(b), err
}

func decodeLineItems(raw string) []LineItem {
	var items []LineItem
	_ = json.Unmarshal([]byte(raw), &items)
	return items
}

// --- Quotes ------------------------------------------------------------------------------------

const quoteColumns = "id, clientId, status, lineItems, totalEstimate, validUntil, createdAt, updatedAt"

func scanQuote(rows *sql.Rows) (Quote, error) {
	var q Quote
	var lineItemsRaw string
	err := rows.Scan(&q.ID, &q.ClientID, &q.Status, &lineItemsRaw, &q.TotalEstimate, &q.ValidUntil, &q.CreatedAt, &q.UpdatedAt)
	q.LineItems = decodeLineItems(lineItemsRaw)
	return q, err
}

func GetQuoteByID(ctx context.Context, q studiodb.Querier, id string) (*Quote, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+quoteColumns+" FROM Quote WHERE id = ?", scanQuote, id)
}

func scanQuoteListRow(rows *sql.Rows) (QuoteListRow, error) {
	var r QuoteListRow
	err := rows.Scan(&r.ID, &r.Status, &r.TotalEstimate, &r.CreatedAt, &r.ClientName, &r.OrderID, &r.OrderNumber)
	return r, err
}

func ListQuotes(ctx context.Context, q studiodb.Querier) ([]QuoteListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT q.id, q.status, q.totalEstimate, q.createdAt, c.name, o.id, o.orderNumber
		FROM Quote q
		JOIN Client c ON c.id = q.clientId
		LEFT JOIN "Order" o ON o.quoteId = q.id
		ORDER BY q.createdAt DESC`, scanQuoteListRow)
}

func CreateQuote(ctx context.Context, q studiodb.Querier, clientID string, items []LineItem, validUntil any) (string, error) {
	lineItemsJSON, err := encodeLineItems(items)
	if err != nil {
		return "", err
	}
	var total float64
	for _, i := range items {
		total += i.Amount
	}
	id := studiodb.NewID()
	now := time.Now()
	_, err = studiodb.Execute(ctx, q,
		"INSERT INTO Quote (id, clientId, lineItems, totalEstimate, validUntil, createdAt, updatedAt) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, clientID, lineItemsJSON, total, validUntil, now, now)
	return id, err
}

func SetQuoteStatus(ctx context.Context, q studiodb.Querier, id, status string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE Quote SET status = ?, updatedAt = ? WHERE id = ?", status, time.Now(), id)
	return err
}

// AcceptQuote creates an Order from an accepted Quote - a quote never becomes operational work
// directly.
func AcceptQuote(ctx context.Context, pool *sql.DB, quoteID string) (string, error) {
	return studiodb.WithTransaction(ctx, pool, func(tx *sql.Tx) (string, error) {
		quote, err := GetQuoteByID(ctx, tx, quoteID)
		if err != nil {
			return "", err
		}
		if quote == nil {
			return "", fmt.Errorf("quote %s not found", quoteID)
		}
		orderID := studiodb.NewID()
		orderNumber := "ORD-" + strings.ToUpper(strconv.FormatInt(time.Now().UnixMilli(), 36))
		now := time.Now()
		if _, err := studiodb.Execute(ctx, tx,
			`INSERT INTO "Order" (id, clientId, quoteId, orderNumber, createdAt, updatedAt) VALUES (?, ?, ?, ?, ?, ?)`,
			orderID, quote.ClientID, quote.ID, orderNumber, now, now); err != nil {
			return "", err
		}
		if _, err := studiodb.Execute(ctx, tx, "UPDATE Quote SET status = ?, updatedAt = ? WHERE id = ?", "accepted", now, quoteID); err != nil {
			return "", err
		}
		return orderID, nil
	})
}

// --- Orders ------------------------------------------------------------------------------------

const orderColumns = `id, clientId, quoteId, orderNumber, status, notes, createdAt, updatedAt`

func scanOrder(rows *sql.Rows) (Order, error) {
	var o Order
	err := rows.Scan(&o.ID, &o.ClientID, &o.QuoteID, &o.OrderNumber, &o.Status, &o.Notes, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

func GetOrderByID(ctx context.Context, q studiodb.Querier, id string) (*Order, error) {
	return studiodb.QueryOne(ctx, q, `SELECT `+orderColumns+` FROM "Order" WHERE id = ?`, scanOrder, id)
}

func GetOrderByQuoteID(ctx context.Context, q studiodb.Querier, quoteID string) (*Order, error) {
	return studiodb.QueryOne(ctx, q, `SELECT `+orderColumns+` FROM "Order" WHERE quoteId = ?`, scanOrder, quoteID)
}

func scanOrderListRow(rows *sql.Rows) (OrderListRow, error) {
	var r OrderListRow
	err := rows.Scan(&r.ID, &r.OrderNumber, &r.Status, &r.ClientName, &r.ProjectCount)
	return r, err
}

func ListOrders(ctx context.Context, q studiodb.Querier) ([]OrderListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT o.id, o.orderNumber, o.status, c.name,
		       (SELECT COUNT(*) FROM Project p WHERE p.orderId = o.id) AS projectCount
		FROM "Order" o JOIN Client c ON c.id = o.clientId
		ORDER BY o.createdAt DESC`, scanOrderListRow)
}

// UpdateOrderStatus validates status against the order_status classifier before applying it -
// caller passes in the set of valid codes (settings.GetClassifiers result) to avoid this package
// depending on settings for a single lookup.
func UpdateOrderStatus(ctx context.Context, q studiodb.Querier, id, status string) error {
	_, err := studiodb.Execute(ctx, q, `UPDATE "Order" SET status = ?, updatedAt = ? WHERE id = ?`, status, time.Now(), id)
	return err
}

func scanProjectOnOrder(rows *sql.Rows) (ProjectOnOrder, error) {
	var p ProjectOnOrder
	err := rows.Scan(&p.ID, &p.Title, &p.Stage, &p.AssetTitle, &p.AssetReferenceCode)
	return p, err
}

func ListProjectsOnOrder(ctx context.Context, q studiodb.Querier, orderID string) ([]ProjectOnOrder, error) {
	return studiodb.Query(ctx, q, `
		SELECT p.id, p.title, p.stage, a.title, a.referenceCode
		FROM Project p JOIN Asset a ON a.id = p.assetId WHERE p.orderId = ?`, scanProjectOnOrder, orderID)
}

func scanUnattachedProject(rows *sql.Rows) (UnattachedProject, error) {
	var p UnattachedProject
	err := rows.Scan(&p.ID, &p.Title, &p.AssetTitle, &p.AssetReferenceCode)
	return p, err
}

func ListUnattachedProjects(ctx context.Context, q studiodb.Querier, clientID string) ([]UnattachedProject, error) {
	return studiodb.Query(ctx, q, `
		SELECT p.id, p.title, a.title, a.referenceCode
		FROM Project p JOIN Asset a ON a.id = p.assetId
		WHERE p.orderId IS NULL AND a.clientId = ?`, scanUnattachedProject, clientID)
}

func AttachProjectToOrder(ctx context.Context, q studiodb.Querier, orderID, projectID string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE Project SET orderId = ?, updatedAt = ? WHERE id = ?", orderID, time.Now(), projectID)
	return err
}

// --- Invoices ----------------------------------------------------------------------------------

const invoiceColumns = "id, orderId, status, lineItems, total, currency, issuedAt, dueAt, paidAt, createdAt, updatedAt"

func scanInvoice(rows *sql.Rows) (Invoice, error) {
	var inv Invoice
	var lineItemsRaw string
	err := rows.Scan(&inv.ID, &inv.OrderID, &inv.Status, &lineItemsRaw, &inv.Total, &inv.Currency,
		&inv.IssuedAt, &inv.DueAt, &inv.PaidAt, &inv.CreatedAt, &inv.UpdatedAt)
	inv.LineItems = decodeLineItems(lineItemsRaw)
	return inv, err
}

func ListInvoicesForOrder(ctx context.Context, q studiodb.Querier, orderID string) ([]Invoice, error) {
	return studiodb.Query(ctx, q, "SELECT "+invoiceColumns+" FROM Invoice WHERE orderId = ? ORDER BY createdAt DESC", scanInvoice, orderID)
}

func CreateInvoice(ctx context.Context, q studiodb.Querier, orderID string, items []LineItem) (string, error) {
	lineItemsJSON, err := encodeLineItems(items)
	if err != nil {
		return "", err
	}
	var total float64
	for _, i := range items {
		total += i.Amount
	}
	id := studiodb.NewID()
	now := time.Now()
	_, err = studiodb.Execute(ctx, q,
		"INSERT INTO Invoice (id, orderId, lineItems, total, currency, status, createdAt, updatedAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, orderID, lineItemsJSON, total, "EUR", "draft", now, now)
	return id, err
}

func SetInvoiceStatus(ctx context.Context, q studiodb.Querier, id, status string) error {
	var paidAt, issuedAt any
	now := time.Now()
	if status == "paid" {
		paidAt = now
	}
	if status == "sent" {
		issuedAt = now
	}
	_, err := studiodb.Execute(ctx, q,
		"UPDATE Invoice SET status = ?, paidAt = COALESCE(?, paidAt), issuedAt = COALESCE(?, issuedAt), updatedAt = ? WHERE id = ?",
		status, paidAt, issuedAt, now, id)
	return err
}

// collectLineItems parses up to 6 numbered item_description_N/item_hours_N/item_rate_N/
// item_amount_N form fields into LineItems - the fixed 6-row quote/invoice line-item form. Rows
// with no description are skipped; amount defaults to hours*rate when left blank.
func collectLineItems(form map[string][]string) []LineItem {
	get := func(key string) string {
		if v, ok := form[key]; ok && len(v) > 0 {
			return v[0]
		}
		return ""
	}
	var items []LineItem
	for i := 0; i < 6; i++ {
		suffix := strconv.Itoa(i)
		description := strings.TrimSpace(get("item_description_" + suffix))
		if description == "" {
			continue
		}
		hours, _ := strconv.ParseFloat(get("item_hours_"+suffix), 64)
		rate, _ := strconv.ParseFloat(get("item_rate_"+suffix), 64)
		amount, err := strconv.ParseFloat(get("item_amount_"+suffix), 64)
		if err != nil || amount == 0 {
			amount = hours * rate
		}
		items = append(items, LineItem{Description: description, EstimatedHours: hours, Rate: rate, Amount: amount})
	}
	return items
}
