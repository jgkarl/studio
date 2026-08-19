package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	studiodb "stuudio/internal/db"
)

var tokenTTL = map[VerificationTokenType]time.Duration{
	TokenEmailVerify:   24 * time.Hour,
	TokenPasswordReset: 1 * time.Hour,
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// CreateVerificationToken creates a token row and returns the raw token — the only time it's
// ever available in plaintext (only a SHA-256 hash is stored, same reasoning as password
// hashing: a database read alone should never yield a usable link).
func CreateVerificationToken(ctx context.Context, q studiodb.Querier, userID string, tokenType VerificationTokenType) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	rawHex := hex.EncodeToString(raw)
	expiresAt := time.Now().Add(tokenTTL[tokenType])
	if err := insertVerificationToken(ctx, q, userID, tokenType, hashToken(rawHex), expiresAt); err != nil {
		return "", err
	}
	return rawHex, nil
}

// ConsumeVerificationToken validates and single-use-consumes a token, returning the associated
// user or nil if invalid/expired/already used.
func ConsumeVerificationToken(ctx context.Context, q studiodb.Querier, rawToken string, tokenType VerificationTokenType) (*User, error) {
	record, err := getVerificationTokenByHash(ctx, q, hashToken(rawToken))
	if err != nil || record == nil {
		return nil, err
	}
	if record.Type != tokenType || record.UsedAt.Valid || record.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	if err := markVerificationTokenUsed(ctx, q, record.ID); err != nil {
		return nil, err
	}
	return GetUserByID(ctx, q, record.UserID)
}
