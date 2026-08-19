package web

import (
	"net/http"

	"stuudio/internal/i18n"
	"stuudio/internal/theme"
)

// MountToggles registers the theme/locale toggle routes shared by every in-app page's Navbar.
// Ungated (no session required) - same as the original app's toggleTheme/toggleLocale actions,
// which read/set a plain cookie and don't touch anything user-specific.
func MountToggles(mux *http.ServeMux) {
	mux.HandleFunc("POST /theme/toggle", func(w http.ResponseWriter, r *http.Request) {
		next := theme.Light
		if theme.Get(r) == theme.Light {
			next = theme.Dark
		}
		theme.Set(w, next)
		redirectBack(w, r)
	})
	mux.HandleFunc("POST /locale/toggle", func(w http.ResponseWriter, r *http.Request) {
		next := i18n.LocaleET
		if i18n.GetLocale(r) == i18n.LocaleET {
			next = i18n.LocaleEN
		}
		i18n.SetLocale(w, next)
		redirectBack(w, r)
	})
}

func redirectBack(w http.ResponseWriter, r *http.Request) {
	dest := r.Header.Get("Referer")
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}
