package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"studio/internal/mail"
	"studio/internal/session"
)

type Service struct {
	Pool          *sql.DB
	Sessions      *session.Manager
	Mailer        *mail.Mailer
	AppURL        string
	AllowDevLogin bool
}

// CurrentUser returns the signed-in user for r, or nil if there isn't one.
func (s *Service) CurrentUser(ctx context.Context, r *http.Request) (*User, error) {
	userID, ok := s.Sessions.UserID(r)
	if !ok {
		return nil, nil
	}
	return GetUserByID(ctx, s.Pool, userID)
}

// Origin returns the absolute origin to build links in emails from — prefers the explicit
// APP_URL config (reliable behind a reverse proxy, where Host/X-Forwarded-Proto aren't always
// trustworthy) and falls back to the incoming request for local dev.
func (s *Service) Origin(r *http.Request) string {
	if s.AppURL != "" {
		return strings.TrimSuffix(s.AppURL, "/")
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1") {
			proto = "http"
		} else {
			proto = "https"
		}
	}
	return proto + "://" + r.Host
}

type ctxKey int

const userCtxKey ctxKey = 0

func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userCtxKey).(*User)
	return u
}

// RequireUser redirects to /login when no session is present; otherwise calls next with the
// user attached to the request context (retrieve via UserFromContext).
func (s *Service) RequireUser(next func(w http.ResponseWriter, r *http.Request, user *User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.CurrentUser(r.Context(), r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next(w, r.WithContext(ctx), user)
	}
}

// RequireAdmin is RequireUser plus a server-enforced role check — used for Settings → Users,
// since granting role changes to any signed-in user would let a conservator promote themselves.
func (s *Service) RequireAdmin(next func(w http.ResponseWriter, r *http.Request, user *User)) http.HandlerFunc {
	return s.RequireUser(func(w http.ResponseWriter, r *http.Request, user *User) {
		if !user.HasRole(RoleAdmin) {
			http.Error(w, "Admin access required.", http.StatusForbidden)
			return
		}
		next(w, r, user)
	})
}
