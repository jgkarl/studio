package auth

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

func scanUser(rows *sql.Rows) (User, error) {
	var u User
	err := rows.Scan(
		&u.ID, &u.Name, &u.Email, &u.AvatarURL, &u.Provider, &u.PasswordHash,
		&u.EmailVerifiedAt, &u.Role, &u.CreatedAt,
	)
	return u, err
}

const userColumns = "id, name, email, avatarUrl, provider, passwordHash, emailVerifiedAt, role, createdAt"

func GetUserByID(ctx context.Context, q studiodb.Querier, id string) (*User, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+userColumns+" FROM User WHERE id = ?", scanUser, id)
}

func GetUserByEmail(ctx context.Context, q studiodb.Querier, email string) (*User, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+userColumns+" FROM User WHERE email = ?", scanUser, email)
}

func ListUsers(ctx context.Context, q studiodb.Querier) ([]User, error) {
	return studiodb.Query(ctx, q, "SELECT "+userColumns+" FROM User ORDER BY createdAt ASC", scanUser)
}

// CreateUser inserts a new User row with role "pending" and returns its ID.
func CreateUser(ctx context.Context, q studiodb.Querier, name, email, passwordHash string) (string, error) {
	id := studiodb.NewID()
	_, err := studiodb.Execute(ctx, q,
		"INSERT INTO User (id, name, email, provider, passwordHash, role) VALUES (?, ?, ?, ?, ?, ?)",
		id, name, email, "email", passwordHash, RolePending,
	)
	return id, err
}

func SetPasswordHash(ctx context.Context, q studiodb.Querier, userID, hash string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE User SET passwordHash = ? WHERE id = ?", hash, userID)
	return err
}

func MarkEmailVerified(ctx context.Context, q studiodb.Querier, userID string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE User SET emailVerifiedAt = ? WHERE id = ?", time.Now(), userID)
	return err
}

func scanVerificationToken(rows *sql.Rows) (VerificationToken, error) {
	var t VerificationToken
	err := rows.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Type, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	return t, err
}

const tokenColumns = "id, userId, tokenHash, type, expiresAt, usedAt, createdAt"

func getVerificationTokenByHash(ctx context.Context, q studiodb.Querier, hash string) (*VerificationToken, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+tokenColumns+" FROM VerificationToken WHERE tokenHash = ?", scanVerificationToken, hash)
}

func insertVerificationToken(ctx context.Context, q studiodb.Querier, userID string, tokenType VerificationTokenType, tokenHash string, expiresAt any) error {
	_, err := studiodb.Execute(ctx, q,
		"INSERT INTO VerificationToken (id, userId, tokenHash, type, expiresAt) VALUES (?, ?, ?, ?, ?)",
		studiodb.NewID(), userID, tokenHash, tokenType, expiresAt,
	)
	return err
}

func markVerificationTokenUsed(ctx context.Context, q studiodb.Querier, id string) error {
	_, err := studiodb.Execute(ctx, q, "UPDATE VerificationToken SET usedAt = ? WHERE id = ?", time.Now(), id)
	return err
}
