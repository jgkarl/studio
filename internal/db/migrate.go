package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Migrate applies every *.sql file in migrationsFS that isn't already recorded in
// schema_migrations, in filename order (hence the 0001_, 0002_... prefixes) — no external
// migration framework. migrationsFS is db/migrations' embed.FS (see db/migrations/embed.go) in
// production and in tests (internal/testutil) — the binary/test process never reads
// db/migrations/ off disk. Each file is split into individual statements and executed one at a
// time: unlike the MySQL driver this app used to run on, SQLite has no "run several
// ;-separated statements in one Exec" mode, so this driver doesn't either. Splitting on a bare
// ";" is safe here because every migration file is hand-written DDL this app controls — no
// semicolons inside string literals to worry about.
func Migrate(ctx context.Context, pool *sql.DB, migrationsFS fs.FS) error {
	if _, err := pool.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT NOT NULL PRIMARY KEY,
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

	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
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
		sqlBytes, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			return fmt.Errorf("reading migration %q: %w", name, err)
		}
		for _, stmt := range splitStatements(string(sqlBytes)) {
			if _, err := pool.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("applying migration %q: %w", name, err)
			}
		}
		if _, err := pool.ExecContext(ctx, "INSERT INTO schema_migrations (filename) VALUES (?)", name); err != nil {
			return fmt.Errorf("recording migration %q: %w", name, err)
		}
	}
	return nil
}

// splitStatements splits a .sql file's content into individual, trimmed, non-empty statements on
// top-level ";" boundaries, dropping full-line "--" comments first (SQLite would otherwise choke
// on a comment-only trailing fragment after the last real statement).
func splitStatements(sqlText string) []string {
	var kept []string
	for _, line := range strings.Split(sqlText, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		kept = append(kept, line)
	}
	cleaned := strings.Join(kept, "\n")

	var out []string
	for _, part := range strings.Split(cleaned, ";") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
