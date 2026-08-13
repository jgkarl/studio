package auth

// authMessages maps stable codes (never raw text in a redirect URL) to display copy. English
// only for now — internal/i18n (module 4) will generalize this the same way the rest of the app
// picks up locale switching.
var authMessages = map[string]string{
	"incorrectCredentials": "Incorrect email or password.",
	"emailNotVerified":     "Please confirm your email address before signing in — check your inbox for the verification link.",
	"pendingApproval":      "Your account is awaiting admin approval.",
	"passwordTooShort":     "Password must be at least 8 characters.",
	"passwordMismatch":     "Passwords don't match.",
	"resetLinkInvalid":     "This reset link is invalid or has expired.",
	"passwordUpdated":      "Password updated — sign in with your new password.",
}

func authMessage(code string) string {
	if msg, ok := authMessages[code]; ok {
		return msg
	}
	return code
}
