package web

import (
	"net/http"

	"studio/internal/i18n"
	"studio/internal/theme"
)

// Chrome is everything the persistent app shell (Navbar + page wrapper) needs to render, for
// every in-app page. web deliberately doesn't import internal/auth (would cycle back here via
// auth's own templ views) - callers pass the two auth.User fields the navbar actually needs.
type Chrome struct {
	Active       string
	UserName     string
	UserRole     string
	Theme        theme.Theme
	Locale       i18n.Locale
	Dict         i18n.Dictionary
	ShowSettings bool
}

// BuildChrome reads the theme/locale cookies off r and assembles a Chrome. active is the current
// request path (for nav-link highlighting); showSettings is the caller's own role check (kept
// out of this package to avoid an import cycle with internal/auth).
func BuildChrome(r *http.Request, userName, userRole string, showSettings bool, active string) Chrome {
	locale := i18n.GetLocale(r)
	return Chrome{
		Active:       active,
		UserName:     userName,
		UserRole:     userRole,
		Theme:        theme.Get(r),
		Locale:       locale,
		Dict:         i18n.GetDictionary(locale),
		ShowSettings: showSettings,
	}
}
