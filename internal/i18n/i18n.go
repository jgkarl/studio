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
	Clients, Assets, Projects, Reports, Media, Settings string
}

type DashboardDict struct {
	Title, Subtitle                                             string
	ClientsLabel, AssetsLabel, ProjectsLabel, ReportsLabel      string
	ActiveLabel, AllLabel, DraftLabel, FinalLabel               string
	ActiveProjectsHeading, NothingInProgress                    string
	AssessmentsCardLabel, TreatmentsCardLabel, ReportsCardLabel string
	NoneYet                                                     string
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
			Clients: "Kliendid", Assets: "Esemed", Projects: "Projektid",
			Reports: "Aruanded", Media: "Meedia", Settings: "Seaded",
		},
		Dashboard: DashboardDict{
			Title: "Töölaud", Subtitle: "Ülevaade käimasolevast tööst.",
			ClientsLabel: "Kliendid", AssetsLabel: "Esemed", ProjectsLabel: "Projektid", ReportsLabel: "Aruanded",
			ActiveLabel: "aktiivsed", AllLabel: "kokku", DraftLabel: "mustandid", FinalLabel: "valmis",
			ActiveProjectsHeading: "Aktiivsed projektid", NothingInProgress: "Hetkel pole käimasolevaid töid.",
			AssessmentsCardLabel: "Hinnangud", TreatmentsCardLabel: "Töötlused", ReportsCardLabel: "Aruanded",
			NoneYet: "Pole veel.",
		},
		Common: CommonDict{
			SwitchToEnglish: "Switch to English", SwitchToEstonian: "Vaheta eesti keelele",
			SwitchToLightTheme: "Lülitu heledale teemale", SwitchToDarkTheme: "Lülitu tumedale teemale",
			SignOut: "Logi välja", Menu: "Menüü",
		},
	},
	LocaleEN: {
		Nav: NavDict{
			Clients: "Clients", Assets: "Assets", Projects: "Projects",
			Reports: "Reports", Media: "Media", Settings: "Settings",
		},
		Dashboard: DashboardDict{
			Title: "Dashboard", Subtitle: "An overview of work in progress.",
			ClientsLabel: "Clients", AssetsLabel: "Assets", ProjectsLabel: "Projects", ReportsLabel: "Reports",
			ActiveLabel: "active", AllLabel: "all", DraftLabel: "draft", FinalLabel: "final",
			ActiveProjectsHeading: "Active projects", NothingInProgress: "Nothing in progress right now.",
			AssessmentsCardLabel: "Assessments", TreatmentsCardLabel: "Treatments", ReportsCardLabel: "Reports",
			NoneYet: "None yet.",
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
