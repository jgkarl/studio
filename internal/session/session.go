// Package session implements a minimal signed-cookie session: no server-side session table, no
// external session library. Mirrors the original Node app's lib/auth.ts session handling exactly
// (HMAC-SHA256 over the user ID, constant-time compare) — every module just calls
// Manager.UserID(r) and doesn't care how the session was created.
package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const cookieName = "studio_session"

type Manager struct {
	secret []byte
}

func New(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) sign(userID string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(userID))
	return userID + "." + hex.EncodeToString(mac.Sum(nil))
}

func (m *Manager) verify(token string) (string, bool) {
	userID, sig, ok := strings.Cut(token, ".")
	if !ok || userID == "" || sig == "" {
		return "", false
	}
	expected := m.sign(userID)
	_, expectedSig, _ := strings.Cut(expected, ".")
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expectedSig)) != 1 {
		return "", false
	}
	return userID, true
}

// Create sets the session cookie for userID.
func (m *Manager) Create(w http.ResponseWriter, userID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    m.sign(userID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

// Destroy clears the session cookie.
func (m *Manager) Destroy(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// UserID returns the signed-in user ID from the request's session cookie, if any.
func (m *Manager) UserID(r *http.Request) (string, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return m.verify(c.Value)
}
