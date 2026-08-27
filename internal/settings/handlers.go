package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/auth"
	studiodb "studio/internal/db"
	"studio/internal/web"
)

func chromeFor(r *http.Request, user *auth.User, active string) web.Chrome {
	return web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), active)
}

type Service struct {
	Pool *sql.DB
	Auth *auth.Service
}

// Mount registers every settings route on mux. Classifiers require a signed-in user
// (RequireUser, not RequireAdmin — the nav only shows a Settings link to admins, but
// conservators can still reach these routes directly); the Users tab
// (both viewing and its role-change mutation) is RequireAdmin-gated since it can grant admin
// access to any account.
func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /settings", svc.Auth.RequireUser(svc.handleClassifiers))
	mux.HandleFunc("GET /settings/features", svc.Auth.RequireUser(svc.handleFeatures))
	mux.HandleFunc("POST /settings/features", svc.Auth.RequireUser(svc.handleFeaturesUpdate))
	mux.HandleFunc("POST /settings/reportable", svc.Auth.RequireUser(svc.handleReportableUpdate))
	mux.HandleFunc("GET /settings/users", svc.Auth.RequireAdmin(svc.handleUsers))
	mux.HandleFunc("POST /settings/classifiers/{type}", svc.Auth.RequireUser(svc.handleClassifierCreate))
	mux.HandleFunc("POST /settings/classifiers/{type}/find-or-create", svc.Auth.RequireUser(svc.handleClassifierFindOrCreate))
	mux.HandleFunc("POST /settings/classifiers/{type}/{id}/update", svc.Auth.RequireUser(svc.handleClassifierUpdate))
	mux.HandleFunc("POST /settings/classifiers/{type}/{id}/delete", svc.Auth.RequireUser(svc.handleClassifierDelete))
	mux.HandleFunc("POST /settings/users/{id}/role", svc.Auth.RequireAdmin(svc.handleUpdateUserRole))
}

func writeHTML(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(r.Context(), w)
}

func (svc *Service) handleClassifiers(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	groups := make([]ClassifierGroup, 0, len(SettingsManagedTypes))
	for _, t := range SettingsManagedTypes {
		rows, err := GetAllClassifiers(ctx, svc.Pool, t)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		groups = append(groups, ClassifierGroup{Type: t, Label: ClassifierTypeLabels[t], Rows: rows})
	}

	writeHTML(w, r, ClassifiersPage(chromeFor(r, user, "/settings"), groups, user.HasRole(auth.RoleAdmin)))
}

func (svc *Service) handleFeatures(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	limits := LoadDashboardLimits(ctx, svc.Pool)
	reportableGroups := LoadReportableGroups(ctx, svc.Pool)
	writeHTML(w, r, FeaturesPage(chromeFor(r, user, "/settings"), limits, reportableGroups, user.HasRole(auth.RoleAdmin)))
}

func (svc *Service) handleFeaturesUpdate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	for _, l := range DashboardLimits {
		raw := strings.TrimSpace(r.FormValue(l.Key))
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			continue // leave unset/invalid values at their current (default-falling-back) value
		}
		if err := SetInt(ctx, svc.Pool, l.Key, n); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/settings/features", http.StatusSeeOther)
}

func (svc *Service) handleReportableUpdate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	for _, f := range ReportableFields {
		enabled := r.FormValue("reportable."+f.Model+"."+f.Field) == "on"
		if err := SetReportable(ctx, svc.Pool, f.Model, f.Field, enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/settings/features", http.StatusSeeOther)
}

func (svc *Service) handleUsers(w http.ResponseWriter, r *http.Request, user *auth.User) {
	users, err := auth.ListUsers(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, UsersPage(chromeFor(r, user, "/settings"), users))
}

// --- Classifiers -----------------------------------------------------------------------------

var slugNonWord = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a display title into a lowercase_with_underscores code — the flat Settings page
// only asks for a title, unlike the old per-type admin table's separate Code field.
func slugify(title string) string {
	s := slugNonWord.ReplaceAllString(strings.ToLower(strings.TrimSpace(title)), "_")
	return strings.Trim(s, "_")
}

func (svc *Service) handleClassifierCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	t := ClassifierType(r.PathValue("type"))
	if !IsSettingsManagedType(t) {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	titleEt := strings.TrimSpace(r.FormValue("titleEt"))
	code := slugify(title)
	if title == "" || code == "" {
		http.Error(w, "Title is required.", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	count, err := CountClassifiers(ctx, svc.Pool, []ClassifierType{t})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := ""
	if t == ClassifierAnnotationType {
		data = buildAnnotationTypeData(r.FormValue("hatch"), r.FormValue("color"))
	}
	if _, err := CreateClassifier(ctx, svc.Pool, ClassifierInput{
		Type: t, Code: code, Title: title, TitleEt: titleEt, Sequence: count, IsActive: true, Data: data,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// buildAnnotationTypeData JSON-encodes an annotation type's pattern-layer color + hatch direction
// (internal/media/annotations.go's AnnotationTypeData decodes the exact same {"hatch","color"}
// shape) - "" (no Data at all) when both are blank, so a plain classifier type untouched by this
// form still gets media's documented fallback (red diagonal hatch) rather than an empty object.
func buildAnnotationTypeData(hatch, color string) string {
	hatch = strings.TrimSpace(hatch)
	color = strings.TrimSpace(color)
	if hatch == "" && color == "" {
		return ""
	}
	if hatch == "" {
		hatch = "hatch-diagonal"
	}
	if color == "" {
		color = "#dc2626"
	}
	b, _ := json.Marshal(struct {
		Hatch string `json:"hatch"`
		Color string `json:"color"`
	}{hatch, color})
	return string(b)
}

// handleClassifierUpdate saves title/titleEt (and, for annotation types, the color/hatch pair -
// see buildAnnotationTypeData) for an existing classifier. Code/sequence/isActive aren't editable
// here yet.
func (svc *Service) handleClassifierUpdate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	t := ClassifierType(r.PathValue("type"))
	if !IsSettingsManagedType(t) {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	existing, err := GetClassifierByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing == nil || existing.Type != t {
		http.NotFound(w, r)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = existing.Title
	}
	titleEt := strings.TrimSpace(r.FormValue("titleEt"))
	if titleEt == "" {
		titleEt = existing.TitleEt.String
	}
	data := existing.Data.String
	if t == ClassifierAnnotationType {
		data = buildAnnotationTypeData(r.FormValue("hatch"), r.FormValue("color"))
	}
	if err := UpdateClassifier(ctx, svc.Pool, id, title, titleEt, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// handleClassifierFindOrCreate backs static/js/classifier-autocomplete.js: resolves a typed
// title against existing classifiers of the type (case-insensitive), creating one on the spot if
// nothing matches, so a ClassifierAutocomplete field never forces a trip to Settings first.
func (svc *Service) handleClassifierFindOrCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	t := ClassifierType(r.PathValue("type"))
	if !IsValidClassifierType(string(t)) {
		http.NotFound(w, r)
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		http.Error(w, "Title is required.", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	existing, err := GetClassifiers(ctx, svc.Pool, t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, c := range existing {
		if strings.EqualFold(c.Title, title) {
			writeJSON(w, c)
			return
		}
	}

	code := slugify(title)
	if code == "" {
		http.Error(w, "Title is required.", http.StatusBadRequest)
		return
	}
	count, err := CountClassifiers(ctx, svc.Pool, []ClassifierType{t})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, err := CreateClassifier(ctx, svc.Pool, ClassifierInput{
		Type: t, Code: code, Title: title, Sequence: count, IsActive: true,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, Classifier{ID: id, Type: t, Code: code, Title: title})
}

func writeJSON(w http.ResponseWriter, c Classifier) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		ID    string `json:"id"`
		Code  string `json:"code"`
		Title string `json:"title"`
	}{c.ID, c.Code, c.Title})
}

func (svc *Service) handleClassifierDelete(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := DeleteClassifier(r.Context(), svc.Pool, r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// --- Users (inlined, admin only) --------------------------------------------------------------

var assignableRoles = []auth.Role{auth.RolePending, auth.RoleConservator, auth.RoleAdmin}

func isAssignableRole(role string) bool {
	for _, r := range assignableRoles {
		if string(r) == role {
			return true
		}
	}
	return false
}

func (svc *Service) handleUpdateUserRole(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	targetID := r.PathValue("id")
	role := r.FormValue("role")
	if !isAssignableRole(role) {
		http.Error(w, "Invalid role.", http.StatusBadRequest)
		return
	}
	if _, err := studiodb.Execute(r.Context(), svc.Pool, "UPDATE User SET role = ? WHERE id = ?", role, targetID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
