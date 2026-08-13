package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/auth"
	studiodb "studio/internal/db"
)

type Service struct {
	Pool *sql.DB
	Auth *auth.Service
}

// Mount registers every settings route on mux — classifiers + tags here, users in module 12.
// Everything requires a signed-in user (RequireUser, not RequireAdmin — the nav only shows a
// Settings link to admins, but conservators can still reach these routes directly, same as the
// original app; only Users management is admin-gated).
func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /settings", svc.Auth.RequireUser(svc.handleIndex))

	mux.HandleFunc("GET /settings/classifiers", svc.Auth.RequireUser(svc.handleClassifiersIndex))
	mux.HandleFunc("GET /settings/classifiers/{type}", svc.Auth.RequireUser(svc.handleClassifierType))
	mux.HandleFunc("POST /settings/classifiers/{type}", svc.Auth.RequireUser(svc.handleClassifierCreate))
	mux.HandleFunc("POST /settings/classifiers/{type}/{id}/update", svc.Auth.RequireUser(svc.handleClassifierUpdate))
	mux.HandleFunc("POST /settings/classifiers/{type}/{id}/reorder", svc.Auth.RequireUser(svc.handleClassifierReorder))
	mux.HandleFunc("POST /settings/classifiers/{type}/{id}/delete", svc.Auth.RequireUser(svc.handleClassifierDelete))

	mux.HandleFunc("GET /settings/tags", svc.Auth.RequireUser(svc.handleTagsIndex))
	mux.HandleFunc("POST /settings/tags", svc.Auth.RequireUser(svc.handleTagCreate))
	mux.HandleFunc("POST /settings/tags/{id}/rename", svc.Auth.RequireUser(svc.handleTagRename))
	mux.HandleFunc("POST /settings/tags/{id}/reorder", svc.Auth.RequireUser(svc.handleTagReorder))
	mux.HandleFunc("POST /settings/tags/{id}/delete", svc.Auth.RequireUser(svc.handleTagDelete))
}

func writeHTML(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(r.Context(), w)
}

// --- Landing -------------------------------------------------------------------------------

func (svc *Service) handleIndex(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	classifierCount, err := CountClassifiers(ctx, svc.Pool, ClassifierTypes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tagCountRow, err := studiodb.QueryOne(ctx, svc.Pool, "SELECT COUNT(*) AS n FROM Tag", scanCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tagCount := 0
	if tagCountRow != nil {
		tagCount = *tagCountRow
	}
	pendingRow, err := studiodb.QueryOne(ctx, svc.Pool, "SELECT COUNT(*) AS n FROM User WHERE role = ?", scanCount, auth.RolePending)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pendingCount := 0
	if pendingRow != nil {
		pendingCount = *pendingRow
	}
	writeHTML(w, r, IndexPage(classifierCount, len(ClassifierTypes), tagCount, pendingCount))
}

// --- Classifiers -----------------------------------------------------------------------------

func (svc *Service) handleClassifiersIndex(w http.ResponseWriter, r *http.Request, user *auth.User) {
	counts := map[ClassifierType]int{}
	for _, t := range ClassifierTypes {
		n, err := CountClassifiers(r.Context(), svc.Pool, []ClassifierType{t})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		counts[t] = n
	}
	writeHTML(w, r, ClassifiersIndexPage(counts))
}

func (svc *Service) handleClassifierType(w http.ResponseWriter, r *http.Request, user *auth.User) {
	t := ClassifierType(r.PathValue("type"))
	if !IsValidClassifierType(string(t)) {
		http.NotFound(w, r)
		return
	}
	rows, err := GetAllClassifiers(r.Context(), svc.Pool, t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, ClassifierTypePage(t, rows, r.URL.Query().Get("edit")))
}

func parseClassifierData(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if !json.Valid([]byte(trimmed)) {
		return "", errInvalidJSON
	}
	return trimmed, nil
}

var errInvalidJSON = &jsonError{}

type jsonError struct{}

func (*jsonError) Error() string {
	return `Extra data must be valid JSON (e.g. {"defaultRate": 60}) or left blank.`
}

func (svc *Service) handleClassifierCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	t := ClassifierType(r.PathValue("type"))
	if !IsValidClassifierType(string(t)) {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(r.FormValue("code"))
	title := strings.TrimSpace(r.FormValue("title"))
	if code == "" || title == "" {
		http.Error(w, "Code and title are required.", http.StatusBadRequest)
		return
	}
	data, err := parseClassifierData(r.FormValue("data"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sequence, _ := strconv.Atoi(r.FormValue("sequence"))

	if _, err := CreateClassifier(r.Context(), svc.Pool, ClassifierInput{
		Type: t, Code: code, Title: title,
		Description: strings.TrimSpace(r.FormValue("description")),
		Sequence:    sequence,
		IsActive:    true,
		Data:        data,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/classifiers/"+string(t), http.StatusSeeOther)
}

func (svc *Service) handleClassifierUpdate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	t := r.PathValue("type")
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "Title is required.", http.StatusBadRequest)
		return
	}
	data, err := parseClassifierData(r.FormValue("data"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sequence, _ := strconv.Atoi(r.FormValue("sequence"))

	if err := UpdateClassifier(r.Context(), svc.Pool, id, ClassifierInput{
		Title:       title,
		Description: strings.TrimSpace(r.FormValue("description")),
		Sequence:    sequence,
		IsActive:    r.FormValue("isActive") == "on",
		Data:        data,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/classifiers/"+t, http.StatusSeeOther)
}

func (svc *Service) handleClassifierReorder(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if err := ReorderClassifier(r.Context(), svc.Pool, r.PathValue("id"), r.FormValue("direction")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/classifiers/"+r.PathValue("type"), http.StatusSeeOther)
}

func (svc *Service) handleClassifierDelete(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := DeleteClassifier(r.Context(), svc.Pool, r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/settings/classifiers/"+r.PathValue("type"), http.StatusSeeOther)
}

// --- Tags ------------------------------------------------------------------------------------

func (svc *Service) handleTagsIndex(w http.ResponseWriter, r *http.Request, user *auth.User) {
	tags, err := GetAllTagsWithUsage(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, TagsIndexPage(tags, r.URL.Query().Get("edit")))
}

func (svc *Service) handleTagCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if err := CreateTag(r.Context(), svc.Pool, r.FormValue("name"), r.FormValue("category")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/settings/tags", http.StatusSeeOther)
}

func (svc *Service) handleTagRename(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if err := RenameTag(r.Context(), svc.Pool, r.PathValue("id"), r.FormValue("name"), r.FormValue("category")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/settings/tags", http.StatusSeeOther)
}

func (svc *Service) handleTagReorder(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if err := ReorderTag(r.Context(), svc.Pool, r.PathValue("id"), r.FormValue("direction")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/tags", http.StatusSeeOther)
}

func (svc *Service) handleTagDelete(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := DeleteTag(r.Context(), svc.Pool, r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/tags", http.StatusSeeOther)
}
