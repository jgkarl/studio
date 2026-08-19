// Package db is the only place raw SQL touches the wire. No ORM, no query builder — every
// module hand-writes its SQL and its own row-scanning functions using the generic helpers here.
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// dsn builds the modernc.org/sqlite connection string for path — a plain filesystem path (e.g.
// "./data/stuudio.db"), with query-string pragmas applied to every connection the pool opens:
//
//   - foreign_keys(1): SQLite disables FK enforcement by default per-connection; every module's
//     schema relies on it (ON DELETE CASCADE/SET NULL/RESTRICT).
//   - journal_mode(WAL): lets concurrent readers proceed alongside a single writer instead of
//     blocking on each other.
//   - busy_timeout(5000): a writer that finds the database locked retries for up to 5s instead of
//     failing immediately with SQLITE_BUSY — matters once more than one request writes at once.
//
// _time_format=sqlite: writes Go time.Time values as "YYYY-MM-DD HH:MM:SS.SSS±HH:MM" (millisecond
// precision preserved) instead of the driver's default t.String() format — several modules order
// rows by createdAt to find "the first one" (e.g. MediaReference, TagAssignment) and second-only
// precision could tie-break wrong for rows created in the same request.
func dsn(path string) string {
	u := url.Values{}
	u.Add("_pragma", "foreign_keys(1)")
	u.Add("_pragma", "journal_mode(WAL)")
	u.Add("_pragma", "busy_timeout(5000)")
	u.Set("_time_format", "sqlite")
	return path + "?" + u.Encode()
}

func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}
	pool, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("opening db pool: %w", err)
	}
	// SQLite's single-writer model makes a large connection pool counterproductive — WAL mode
	// still allows concurrent reads through this same cap, and busy_timeout absorbs the rest.
	pool.SetMaxOpenConns(4)
	pool.SetConnMaxLifetime(0)
	return pool, nil
}
