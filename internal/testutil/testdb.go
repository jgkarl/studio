// Package testutil provides a real, migrated SQLite database for package tests that need one —
// no mocks: internal/db's generic Query/Execute helpers are thin enough that the only meaningful
// thing to test against is an actual database.
package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	studiodb "studio/internal/db"
)

// repoRoot resolves the module root from this file's own location, so tests work the same
// regardless of which package directory `go test` runs from.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// OpenTestDB opens a fresh, migrated SQLite database backed by a temp file (not ":memory:" —
// avoids the per-connection-fresh-database surprise an in-memory DSN has once the pool opens
// more than one connection) and registers cleanup to close it.
func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	pool, err := studiodb.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	if err := studiodb.Migrate(context.Background(), pool, filepath.Join(repoRoot(), "db", "migrations")); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return pool
}
