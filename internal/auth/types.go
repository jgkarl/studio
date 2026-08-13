package auth

import (
	"database/sql"
	"time"
)

// Role — same string-union-as-VARCHAR approach as the Node app's lib/types.ts: validated at the
// application layer, not a native DB enum.
type Role string

const (
	RolePending     Role = "pending"
	RoleAdmin       Role = "admin"
	RoleConservator Role = "conservator"
)

type User struct {
	ID              string
	Name            string
	Email           string
	AvatarURL       sql.NullString
	Provider        sql.NullString
	PasswordHash    sql.NullString
	EmailVerifiedAt sql.NullTime
	Role            Role
	CreatedAt       time.Time
}

func (u *User) HasRole(allowed ...Role) bool {
	for _, r := range allowed {
		if u.Role == r {
			return true
		}
	}
	return false
}

type VerificationTokenType string

const (
	TokenEmailVerify   VerificationTokenType = "email_verify"
	TokenPasswordReset VerificationTokenType = "password_reset"
)

type VerificationToken struct {
	ID        string
	UserID    string
	TokenHash string
	Type      VerificationTokenType
	ExpiresAt time.Time
	UsedAt    sql.NullTime
	CreatedAt time.Time
}
