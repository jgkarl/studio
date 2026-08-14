package seed

import (
	"context"
	"database/sql"
	"testing"

	studiodb "studio/internal/db"
	"studio/internal/testutil"
)

func scanCount(rows *sql.Rows) (int, error) {
	var n int
	err := rows.Scan(&n)
	return n, err
}

func countRows(t *testing.T, pool *sql.DB, query string, args ...any) int {
	t.Helper()
	row, err := studiodb.QueryOne(context.Background(), pool, query, scanCount, args...)
	if err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	if row == nil {
		return 0
	}
	return *row
}

func TestSeedAllClassifiers(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	if err := SeedAllClassifiers(ctx, pool); err != nil {
		t.Fatalf("SeedAllClassifiers: %v", err)
	}

	// Every classifier type the app's forms depend on must have at least one option, or those
	// <select> elements render broken/empty — this is the whole point of seeding.
	types := []string{
		"client_type", "contact_method", "asset_type", "material", "condition_state",
		"activity_type", "order_status", "quote_status", "invoice_status",
	}
	for _, typ := range types {
		n := countRows(t, pool, "SELECT COUNT(*) AS n FROM Classifier WHERE type = ?", typ)
		if n == 0 {
			t.Errorf("classifier type %q has zero rows after seeding", typ)
		}
	}

	total := countRows(t, pool, "SELECT COUNT(*) AS n FROM Classifier")

	// Seeding again must be a no-op (INSERT OR IGNORE against the (type, code) unique index) —
	// this is what makes it safe to run on every server boot rather than a one-time migration.
	if err := SeedAllClassifiers(ctx, pool); err != nil {
		t.Fatalf("SeedAllClassifiers (second run): %v", err)
	}
	totalAfterRerun := countRows(t, pool, "SELECT COUNT(*) AS n FROM Classifier")
	if totalAfterRerun != total {
		t.Errorf("re-seeding changed Classifier row count: %d -> %d, want idempotent", total, totalAfterRerun)
	}
}

func TestBootstrapAdmin(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	if err := BootstrapAdmin(ctx, pool, "", ""); err != nil {
		t.Fatalf("BootstrapAdmin with empty args: %v", err)
	}
	if n := countRows(t, pool, "SELECT COUNT(*) AS n FROM User"); n != 0 {
		t.Fatalf("BootstrapAdmin with empty name/email created %d users, want 0", n)
	}

	if err := BootstrapAdmin(ctx, pool, "Ada Admin", "ada@studio.local"); err != nil {
		t.Fatalf("BootstrapAdmin: %v", err)
	}
	if n := countRows(t, pool, "SELECT COUNT(*) AS n FROM User WHERE email = ?", "ada@studio.local"); n != 1 {
		t.Fatalf("got %d User rows for ada@studio.local, want 1", n)
	}
	if n := countRows(t, pool, "SELECT COUNT(*) AS n FROM User WHERE email = ? AND role = 'admin' AND provider = 'dev'", "ada@studio.local"); n != 1 {
		t.Fatalf("bootstrapped user isn't role=admin, provider=dev")
	}

	// Re-running with the same email must not create a second row.
	if err := BootstrapAdmin(ctx, pool, "Ada Admin", "ada@studio.local"); err != nil {
		t.Fatalf("BootstrapAdmin (second run): %v", err)
	}
	if n := countRows(t, pool, "SELECT COUNT(*) AS n FROM User"); n != 1 {
		t.Fatalf("got %d User rows after re-running BootstrapAdmin, want 1", n)
	}
}
