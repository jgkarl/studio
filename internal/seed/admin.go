package seed

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"studio/internal/auth"
	studiodb "studio/internal/db"
)

func scanUserID(rows *sql.Rows) (string, error) {
	var id string
	err := rows.Scan(&id)
	return id, err
}

// BootstrapAdmin creates a single, real, already-active admin User — provider "email", a real
// (hashed) password, emailVerifiedAt set, role admin — if name/email/password are all set and no
// User with that email exists yet. This is the only way to get a first admin account on a brand
// new database without a database console: set BOOTSTRAP_ADMIN_NAME/EMAIL/PASSWORD and restart;
// the app has no other login path (no dev-login picker), so this account is how every deploy,
// dev or production, actually signs in for the first time.
func BootstrapAdmin(ctx context.Context, q studiodb.Querier, name, email, password string) error {
	if name == "" || email == "" || password == "" {
		return nil
	}
	existing, err := studiodb.QueryOne(ctx, q, "SELECT id FROM User WHERE email = ?", scanUserID, email)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing bootstrap admin password: %w", err)
	}
	_, err = studiodb.Execute(ctx, q,
		"INSERT INTO User (id, name, email, provider, passwordHash, emailVerifiedAt, role) VALUES (?, ?, ?, ?, ?, ?, ?)",
		studiodb.NewID(), name, email, "email", hash, time.Now(), string(auth.RoleAdmin))
	return err
}
