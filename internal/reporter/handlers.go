package reporter

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"studio/internal/auth"
	"studio/internal/media"
	"studio/internal/web"
)

type Service struct {
	Pool  *sql.DB
	Auth  *auth.Service
	Media *media.Service
}

func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /reporter", svc.Auth.RequireUser(svc.handleList))
	mux.HandleFunc("GET /reporter/new", svc.Auth.RequireUser(svc.handleNewForm))
	mux.HandleFunc("POST /reporter", svc.Auth.RequireUser(svc.handleCreate))
	mux.HandleFunc("GET /reporter/{id}", svc.Auth.RequireUser(svc.handleDetail))
	mux.HandleFunc("POST /reporter/{id}/content", svc.Auth.RequireUser(svc.handleSaveContent))
	mux.HandleFunc("POST /reporter/{id}/status", svc.Auth.RequireUser(svc.handleSetStatus))
	mux.HandleFunc("POST /reporter/{id}/image", svc.Auth.RequireUser(svc.handleUploadImage))
	mux.HandleFunc("POST /reporter/{id}/attachments", svc.Auth.RequireUser(svc.handleAddAttachments))
}

func writeHTML(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(r.Context(), w)
}

func chromeFor(r *http.Request, user *auth.User, active string) web.Chrome {
	return web.BuildChrome(r, user.Name, string(user.Role), user.HasRole(auth.RoleAdmin), active)
}

func (svc *Service) handleList(w http.ResponseWriter, r *http.Request, user *auth.User) {
	rows, err := List(r.Context(), svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, ListPage(chromeFor(r, user, "/reporter"), rows))
}

func (svc *Service) handleNewForm(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	assets, err := ListAssetOptions(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projects, err := ListProjectOptions(ctx, svc.Pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, NewPage(chromeFor(r, user, "/reporter"), assets, projects,
		r.URL.Query().Get("assetId"), r.URL.Query().Get("projectId")))
}

func (svc *Service) handleCreate(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	assetID := r.FormValue("assetId")
	title := strings.TrimSpace(r.FormValue("title"))
	if assetID == "" || title == "" {
		http.Error(w, "Asset and title are required.", http.StatusBadRequest)
		return
	}
	var projectID *string
	var content string
	if raw := r.FormValue("projectId"); raw != "" {
		projectID = &raw
		outline, err := BuildSuggestedOutline(ctx, svc.Pool, raw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		content = outline
	} else {
		content = emptyDoc()
	}

	id, err := Create(ctx, svc.Pool, assetID, projectID, title, content, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reporter/"+id, http.StatusSeeOther)
}

func (svc *Service) handleDetail(w http.ResponseWriter, r *http.Request, user *auth.User) {
	ctx := r.Context()
	id := r.PathValue("id")
	report, err := GetByID(ctx, svc.Pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if report == nil {
		http.NotFound(w, r)
		return
	}
	refs, err := svc.Media.GetReferencedMedia(ctx, media.RefReport, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeHTML(w, r, DetailPage(chromeFor(r, user, "/reporter"), *report, refs))
}

func (svc *Service) handleSaveContent(w http.ResponseWriter, r *http.Request, user *auth.User) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if !json.Valid(body) {
		http.Error(w, "Content must be valid JSON.", http.StatusBadRequest)
		return
	}
	if err := SaveContent(r.Context(), svc.Pool, r.PathValue("id"), string(body)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (svc *Service) handleSetStatus(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	status := r.FormValue("status")
	if status != "draft" && status != "final" {
		http.Error(w, "Invalid status.", http.StatusBadRequest)
		return
	}
	if err := SetStatus(r.Context(), svc.Pool, id, status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reporter/"+id, http.StatusSeeOther)
}

// handleUploadImage backs the editor toolbar's inline-image button - uploads one image and
// returns its servable URL as JSON, for the JS side to insert into the document at the cursor.
func (svc *Service) handleUploadImage(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	files := media.FilesFromForm(r, "file")
	if len(files) == 0 {
		http.Error(w, "No file provided.", http.StatusBadRequest)
		return
	}
	m, err := svc.Media.UploadAndAttach(r.Context(), files[0], user.ID, media.RefReport, id, "report_embed")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"mediaId": m.ID, "url": "/api/media/" + m.ID})
}

func (svc *Service) handleAddAttachments(w http.ResponseWriter, r *http.Request, user *auth.User) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if _, err := svc.Media.UploadAllAndAttach(r.Context(), media.FilesFromForm(r, "photos"), user.ID, media.RefReport, id, "attachment"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reporter/"+id, http.StatusSeeOther)
}
