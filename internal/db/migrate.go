package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migrate applies every *.sql file in dir that isn't already recorded in schema_migrations, in
// filename order (hence the 0001_, 0002_... prefixes) — a direct port of the Node app's
// db/migrate.ts, same idea, no external migration framework. MySQL auto-commits DDL statements
// (CREATE/ALTER TABLE), so there's no real transactional atomicity to gain by wrapping each file
// in BEGIN/COMMIT; a failed migration is a stop-and-fix-by-hand situation either way.
func Migrate(ctx context.Context, pool *sql.DB, dir string) error {
	if _, err := pool.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename VARCHAR(255) NOT NULL PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("ensuring schema_migrations table: %w", err)
	}

	applied := map[string]bool{}
	rows, err := pool.QueryContext(ctx, "SELECT filename FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("reading schema_migrations: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("scanning schema_migrations: %w", err)
		}
		applied[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading migrations dir %q: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if applied[name] {
			continue
		}
		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("reading migration %q: %w", name, err)
		}
		if _, err := pool.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("applying migration %q: %w", name, err)
		}
		if _, err := pool.ExecContext(ctx, "INSERT INTO schema_migrations (filename) VALUES (?)", name); err != nil {
			return fmt.Errorf("recording migration %q: %w", name, err)
		}
	}
	return nil
}
