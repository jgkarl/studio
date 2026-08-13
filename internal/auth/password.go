package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// scrypt (stdlib-adjacent — golang.org/x/crypto, no cgo, no native binary) rather than
// bcrypt/argon2, matching the original app's reasoning for using Node's built-in scrypt: no
// extra native dependency, and scrypt being CPU/memory-heavy is the point (brute-force
// resistance), not a bug to work around. Params are Go's own choice, not required to bit-match
// the original Node app's scryptSync output — this is a separate app/database, not a shared
// user table.
const (
	scryptN      = 1 << 15 // 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32
	saltLen      = 16
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	hash, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash), nil
}

func VerifyPassword(password, stored string) bool {
	saltHex, hashHex, ok := strings.Cut(stored, ":")
	if !ok {
		return false
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}
	candidate, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(candidate, expected) == 1
}
