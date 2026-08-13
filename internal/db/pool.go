// Package db is the only place raw SQL touches the wire. No ORM, no query builder — every
// module hand-writes its SQL and its own row-scanning functions using the generic helpers here.
// Mirrors the Node app's lib/db.ts (query/queryOne/execute/withTransaction/newId) on purpose:
// same shape, same discipline, ported to Go.
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// dsnFromURL converts a "mysql://user:pass@host:port/dbname" URL (what DATABASE_URL holds, same
// format the Node app used) into the "user:pass@tcp(host:port)/dbname?params" DSN the
// go-sql-driver/mysql package expects.
func dsnFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing DATABASE_URL: %w", err)
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":3306"
	}
	dbName := strings.TrimPrefix(u.Path, "/")

	// parseTime=true: scan DATETIME/TIMESTAMP columns straight into time.Time.
	// multiStatements=true: only needed so the migration runner can execute a whole *.sql file
	// (several ;-separated statements) in one Exec — application queries never use this to run
	// untrusted/concatenated SQL, they always go through the parameterized Query/Execute helpers
	// below.
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&multiStatements=true&charset=utf8mb4", user, pass, host, dbName), nil
}

func Open(databaseURL string) (*sql.DB, error) {
	dsn, err := dsnFromURL(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening db pool: %w", err)
	}
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(10)
	pool.SetConnMaxLifetime(5 * time.Minute)
	return pool, nil
}
