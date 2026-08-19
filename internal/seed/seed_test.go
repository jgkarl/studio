package seed

import (
	"context"
	"database/sql"
	"testing"

	studiodb "stuudio/internal/db"
	"stuudio/internal/testutil"
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

// Classifier reference data now ships as SQL migrations (db/migrations/0009_seed_classifiers.sql)
// applied by internal/db.Migrate — see internal/testutil.OpenTestDB, which runs every migration
// against a fresh test database, so that migration's idempotency and coverage are exercised there
// rather than by a dedicated seed test here.

func TestBootstrapAdmin(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	if err := BootstrapAdmin(ctx, pool, "", "", ""); err != nil {
		t.Fatalf("BootstrapAdmin with empty args: %v", err)
	}
	if n := countRows(t, pool, "SELECT COUNT(*) AS n FROM User"); n != 0 {
		t.Fatalf("BootstrapAdmin with empty name/email/password created %d users, want 0", n)
	}

	if err := BootstrapAdmin(ctx, pool, "Ada Admin", "ada@stuudio.local", "correct-horse-battery"); err != nil {
		t.Fatalf("BootstrapAdmin with name/email but no password: %v", err)
	}
	if n := countRows(t, pool, "SELECT COUNT(*) AS n FROM User"); n != 1 {
		t.Fatalf("got %d User rows, want 1", n)
	}

	if n := countRows(t, pool,
		"SELECT COUNT(*) AS n FROM User WHERE email = ? AND role = 'admin' AND provider = 'email' AND passwordHash IS NOT NULL AND emailVerifiedAt IS NOT NULL",
		"ada@stuudio.local"); n != 1 {
		t.Fatalf("bootstrapped user isn't an active role=admin, provider=email account with a password")
	}

	// Re-running with the same email must not create a second row.
	if err := BootstrapAdmin(ctx, pool, "Ada Admin", "ada@stuudio.local", "correct-horse-battery"); err != nil {
		t.Fatalf("BootstrapAdmin (second run): %v", err)
	}
	if n := countRows(t, pool, "SELECT COUNT(*) AS n FROM User"); n != 1 {
		t.Fatalf("got %d User rows after re-running BootstrapAdmin, want 1", n)
	}
}

func TestBootstrapAdminRequiresPassword(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	if err := BootstrapAdmin(ctx, pool, "Ada Admin", "ada@stuudio.local", ""); err != nil {
		t.Fatalf("BootstrapAdmin with no password: %v", err)
	}
	if n := countRows(t, pool, "SELECT COUNT(*) AS n FROM User"); n != 0 {
		t.Fatalf("BootstrapAdmin with no password created %d users, want 0", n)
	}
}
