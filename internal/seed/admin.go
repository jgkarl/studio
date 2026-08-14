package seed

import (
	"context"
	"database/sql"

	studiodb "studio/internal/db"
)

func scanUserID(rows *sql.Rows) (string, error) {
	var id string
	err := rows.Scan(&id)
	return id, err
}

// BootstrapAdmin creates a single admin User (provider "dev", no password — usable only through
// the dev-login picker, same as db/seed-bootstrap.ts's upsertDevUser) if name and email are both
// set and no User with that email exists yet. This is the only way to get a first admin account
// on a brand new production database without a database console: set BOOTSTRAP_ADMIN_NAME and
// BOOTSTRAP_ADMIN_EMAIL, restart the app, sign in via the dev-login picker once, then use
// Settings -> Users to promote a real registered account and stop relying on it.
func BootstrapAdmin(ctx context.Context, q studiodb.Querier, name, email string) error {
	if name == "" || email == "" {
		return nil
	}
	existing, err := studiodb.QueryOne(ctx, q, "SELECT id FROM User WHERE email = ?", scanUserID, email)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	_, err = studiodb.Execute(ctx, q,
		"INSERT INTO User (id, name, email, provider, role) VALUES (?, ?, ?, ?, ?)",
		studiodb.NewID(), name, email, "dev", "admin")
	return err
}
