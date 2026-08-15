// Package i18n is a small hand-rolled dictionary + cookie-toggle locale switch — no i18n
// framework, mirrors the original app's lib/i18n.ts. Estonian (et) is the default; only an
// explicit toggle switches to English (en). The dictionary only covers what the modules built so
// far actually render (nav + dashboard); auth/settings still use hardcoded English strings from
// their own modules — folding those in is a follow-up, not a blocker for this module.
package i18n

import (
	"net/http"
)

type Locale string

const (
	LocaleET Locale = "et"
	LocaleEN Locale = "en"
)

const cookieName = "studio_locale"

func GetLocale(r *http.Request) Locale {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value != string(LocaleEN) {
		return LocaleET
	}
	return LocaleEN
}

func SetLocale(w http.ResponseWriter, l Locale) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    string(l),
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(365 * 24 * 60 * 60),
	})
}

type NavDict struct {
	Dashboard, Clients, Assets, Workflows, Orders, Reporter, Album, Settings string
}

type DashboardDict struct {
	Title, Subtitle                                string
	Clients, Assets, ActiveWorkflows, DraftReports string
	ActiveWorkflowsHeading, NothingInProgress      string
	OpenOrdersHeading, NoOpenOrders                string
}

type CommonDict struct {
	SwitchToEnglish, SwitchToEstonian     string
	SwitchToLightTheme, SwitchToDarkTheme string
	SignOut, Menu                         string
}

type Dictionary struct {
	Nav       NavDict
	Dashboard DashboardDict
	Common    CommonDict
}

var dictionaries = map[Locale]Dictionary{
	LocaleET: {
		Nav: NavDict{
			Dashboard: "Töölaud", Clients: "Kliendid", Assets: "Esemed", Workflows: "Töövood",
			Orders: "Tellimused", Reporter: "Aruanded", Album: "Album", Settings: "Seaded",
		},
		Dashboard: DashboardDict{
			Title: "Töölaud", Subtitle: "Ülevaade käimasolevast tööst.",
			Clients: "Kliendid", Assets: "Esemed", ActiveWorkflows: "Aktiivsed töövood", DraftReports: "Mustandaruanded",
			ActiveWorkflowsHeading: "Aktiivsed töövood", NothingInProgress: "Hetkel pole käimasolevaid töid.",
			OpenOrdersHeading: "Avatud tellimused", NoOpenOrders: "Avatud tellimusi pole.",
		},
		Common: CommonDict{
			SwitchToEnglish: "Switch to English", SwitchToEstonian: "Vaheta eesti keelele",
			SwitchToLightTheme: "Lülitu heledale teemale", SwitchToDarkTheme: "Lülitu tumedale teemale",
			SignOut: "Logi välja", Menu: "Menüü",
		},
	},
	LocaleEN: {
		Nav: NavDict{
			Dashboard: "Dashboard", Clients: "Clients", Assets: "Assets", Workflows: "Workflows",
			Orders: "Orders", Reporter: "Reporter", Album: "Album", Settings: "Settings",
		},
		Dashboard: DashboardDict{
			Title: "Dashboard", Subtitle: "An overview of work in progress.",
			Clients: "Clients", Assets: "Assets", ActiveWorkflows: "Active workflows", DraftReports: "Draft reports",
			ActiveWorkflowsHeading: "Active workflows", NothingInProgress: "Nothing in progress right now.",
			OpenOrdersHeading: "Open orders", NoOpenOrders: "No open orders.",
		},
		Common: CommonDict{
			SwitchToEnglish: "Switch to English", SwitchToEstonian: "Switch to Estonian",
			SwitchToLightTheme: "Switch to light theme", SwitchToDarkTheme: "Switch to dark theme",
			SignOut: "Sign out", Menu: "Menu",
		},
	},
}

func GetDictionary(l Locale) Dictionary {
	return dictionaries[l]
}
