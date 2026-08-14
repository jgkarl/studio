package settings

import (
	"net/http"

	"studio/internal/auth"
	studiodb "studio/internal/db"
)

func isAssignableRole(role string) bool {
	for _, r := range assignableRoles {
		if string(r) == role {
			return true
		}
	}
	return false
}

func (svc *Service) handleUsersIndex(w http.ResponseWriter, r *http.Request, user *auth.User) {
	users, err := auth.ListUsers(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, UsersPage(chromeFor(r, user, "/settings"), users))
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
	http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
}
