// Package theme is a cookie-toggle dark/light switch — mirrors lib/theme.ts. Default is always
// light; only an explicit toggle (never OS preference) switches to dark.
package theme

import "net/http"

type Theme string

const (
	Light Theme = "light"
	Dark  Theme = "dark"
)

const cookieName = "stuudio_theme"

func Get(r *http.Request) Theme {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value != string(Dark) {
		return Light
	}
	return Dark
}

func Set(w http.ResponseWriter, t Theme) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    string(t),
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(365 * 24 * 60 * 60),
	})
}
