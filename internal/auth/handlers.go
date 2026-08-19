package auth

import (
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/mail"
)

// Mount registers every auth route on mux. Public — no session required to reach any of these
// (that's the point of them).
func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /login", svc.handleLoginPage)
	mux.HandleFunc("POST /login", svc.handleLogin)
	mux.HandleFunc("POST /logout", svc.handleLogout)

	mux.HandleFunc("GET /register", svc.handleRegisterPage)
	mux.HandleFunc("POST /register", svc.handleRegister)
	mux.HandleFunc("GET /register/check-email", svc.handleRegisterCheckEmail)

	mux.HandleFunc("GET /forgot-password", svc.handleForgotPasswordPage)
	mux.HandleFunc("POST /forgot-password", svc.handleForgotPassword)
	mux.HandleFunc("GET /forgot-password/check-email", svc.handleForgotPasswordCheckEmail)

	mux.HandleFunc("GET /reset-password", svc.handleResetPasswordPage)
	mux.HandleFunc("POST /reset-password", svc.handleResetPassword)

	mux.HandleFunc("GET /verify-email", svc.handleVerifyEmail)
	mux.HandleFunc("POST /verify-email/resend", svc.handleResendVerification)
}

func writeHTML(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Render(r.Context(), w); err != nil {
		log.Printf("render error: %v", err)
	}
}

// --- Login -------------------------------------------------------------------------------

func (s *Service) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if user, _ := s.CurrentUser(r.Context(), r); user != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	writeHTML(w, r, LoginPage(r.URL.Query().Get("error"), r.URL.Query().Get("notice")))
}

func loginFailure(w http.ResponseWriter, r *http.Request, reason string) {
	http.Redirect(w, r, "/login?error="+url.QueryEscape(reason), http.StatusSeeOther)
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	user, err := GetUserByEmail(r.Context(), s.Pool, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Same generic message whether the account doesn't exist or the password is wrong —
	// avoids confirming which emails have accounts.
	if user == nil || user.Provider.String != "email" || !user.PasswordHash.Valid || !VerifyPassword(password, user.PasswordHash.String) {
		loginFailure(w, r, "incorrectCredentials")
		return
	}
	if !user.EmailVerifiedAt.Valid {
		loginFailure(w, r, "emailNotVerified")
		return
	}
	if user.Role == RolePending {
		loginFailure(w, r, "pendingApproval")
		return
	}

	s.Sessions.Create(w, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Sessions.Destroy(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// --- Register ------------------------------------------------------------------------------

var emailRE = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func (s *Service) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	writeHTML(w, r, RegisterPage())
}

func (s *Service) handleRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirmPassword")

	if name == "" || email == "" || password == "" {
		http.Error(w, "Name, email, and password are required.", http.StatusBadRequest)
		return
	}
	if !emailRE.MatchString(email) {
		http.Error(w, "Enter a valid email address.", http.StatusBadRequest)
		return
	}
	if len(password) < 8 {
		http.Error(w, "Password must be at least 8 characters.", http.StatusBadRequest)
		return
	}
	if password != confirmPassword {
		http.Error(w, "Passwords don't match.", http.StatusBadRequest)
		return
	}

	existing, err := GetUserByEmail(r.Context(), s.Pool, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "An account with this email already exists — try signing in instead.", http.StatusConflict)
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userID, err := CreateUser(r.Context(), s.Pool, name, email, hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.sendVerificationEmail(r, userID, name, email); err != nil {
		log.Printf("sending verification email: %v", err)
	}

	http.Redirect(w, r, "/register/check-email", http.StatusSeeOther)
}

func (s *Service) sendVerificationEmail(r *http.Request, userID, name, email string) error {
	token, err := CreateVerificationToken(r.Context(), s.Pool, userID, TokenEmailVerify)
	if err != nil {
		return err
	}
	link := s.Origin(r) + "/verify-email?token=" + url.QueryEscape(token)
	return s.Mailer.Send(applyName(mail.VerifyEmailMessage(name, link), email))
}

// applyName sets the To address on a message built from a template call — kept as a tiny
// helper so the two send call sites (register, resend) don't repeat the same three lines.
func applyName(msg mail.Message, to string) mail.Message {
	msg.To = to
	return msg
}

func (s *Service) handleRegisterCheckEmail(w http.ResponseWriter, r *http.Request) {
	writeHTML(w, r, CheckEmailPage("Check your email",
		"We've sent a confirmation link to your email address. Click it to finish setting up your account — an admin will still need to approve it before you can sign in."))
}

// --- Verify email --------------------------------------------------------------------------

func (s *Service) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	var verified *User
	if token != "" {
		user, err := ConsumeVerificationToken(r.Context(), s.Pool, token, TokenEmailVerify)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if user != nil {
			if err := MarkEmailVerified(r.Context(), s.Pool, user.ID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			verified = user
		}
	}
	writeHTML(w, r, VerifyEmailPage(verified))
}

func (s *Service) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	user, err := GetUserByEmail(r.Context(), s.Pool, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Same response regardless of whether the account exists/is already verified — avoids
	// leaking which emails have accounts.
	if user != nil && user.Provider.String == "email" && !user.EmailVerifiedAt.Valid {
		if err := s.sendVerificationEmail(r, user.ID, user.Name, user.Email); err != nil {
			log.Printf("resending verification email: %v", err)
		}
	}
	http.Redirect(w, r, "/register/check-email", http.StatusSeeOther)
}

// --- Forgot / reset password ----------------------------------------------------------------

func (s *Service) handleForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	writeHTML(w, r, ForgotPasswordPage())
}

func (s *Service) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	user, err := GetUserByEmail(r.Context(), s.Pool, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Same response regardless of whether the account exists — avoids leaking which emails
	// have accounts.
	if user != nil && user.Provider.String == "email" {
		token, err := CreateVerificationToken(r.Context(), s.Pool, user.ID, TokenPasswordReset)
		if err != nil {
			log.Printf("creating reset token: %v", err)
		} else {
			link := s.Origin(r) + "/reset-password?token=" + url.QueryEscape(token)
			if err := s.Mailer.Send(applyName(mail.ResetPasswordMessage(user.Name, link), user.Email)); err != nil {
				log.Printf("sending reset email: %v", err)
			}
		}
	}
	http.Redirect(w, r, "/forgot-password/check-email", http.StatusSeeOther)
}

func (s *Service) handleForgotPasswordCheckEmail(w http.ResponseWriter, r *http.Request) {
	writeHTML(w, r, CheckEmailPage("Check your email",
		"If an account exists for that address, we've sent a link to reset your password."))
}

func (s *Service) handleResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	writeHTML(w, r, ResetPasswordPage(r.URL.Query().Get("token"), r.URL.Query().Get("error")))
}

func resetFailure(w http.ResponseWriter, r *http.Request, token, reason string) {
	http.Redirect(w, r, "/reset-password?token="+url.QueryEscape(token)+"&error="+url.QueryEscape(reason), http.StatusSeeOther)
}

func (s *Service) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	token := r.FormValue("token")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirmPassword")

	if len(password) < 8 {
		resetFailure(w, r, token, "passwordTooShort")
		return
	}
	if password != confirmPassword {
		resetFailure(w, r, token, "passwordMismatch")
		return
	}

	user, err := ConsumeVerificationToken(r.Context(), s.Pool, token, TokenPasswordReset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		resetFailure(w, r, token, "resetLinkInvalid")
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := SetPasswordHash(r.Context(), s.Pool, user.ID, hash); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Note: this doesn't invalidate any other active session for the account — the
	// signed-cookie session has no server-side session table to revoke from. A known
	// limitation carried over from the original app.
	http.Redirect(w, r, "/login?notice="+url.QueryEscape("passwordUpdated"), http.StatusSeeOther)
}
