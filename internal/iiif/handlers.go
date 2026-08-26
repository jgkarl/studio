package iiif

import (
	"encoding/json"
	"net/http"
	"strings"

	"studio/internal/auth"
	"studio/internal/media"
)

type Service struct {
	Media *media.Service
}

// Mount registers the IIIF Image API routes — public (unauthenticated), same convention as
// GET /api/media/{id} (internal/media/handlers.go): media IDs are unguessable UUIDs, and IIIF
// services are conventionally public by design anyway (any IIIF-aware tool should be able to
// resolve one without a session).
func Mount(mux *http.ServeMux, svc *Service, authSvc *auth.Service) {
	mux.HandleFunc("GET /api/iiif/{id}/info.json", svc.handleInfoJSON(authSvc))
	mux.HandleFunc("GET /api/iiif/{id}/{region}/{size}/{rotation}/{qualityFormat}", svc.handleTransform)
}

func (svc *Service) handleInfoJSON(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := media.GetByID(r.Context(), svc.Media.Pool, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if m == nil || m.Kind != media.KindImage || !m.Width.Valid || !m.Height.Valid {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/ld+json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // IIIF Image API §5 requires CORS-open info.json
		_ = json.NewEncoder(w).Encode(BuildInfoJSON(id, int(m.Width.Int64), int(m.Height.Int64), authSvc.Origin(r)))
	}
}

func (svc *Service) handleTransform(w http.ResponseWriter, r *http.Request) {
	qualityFormat := r.PathValue("qualityFormat")
	dot := strings.LastIndex(qualityFormat, ".")
	if dot < 0 {
		http.Error(w, "invalid quality.format", http.StatusBadRequest)
		return
	}
	params := Params{
		Region:   r.PathValue("region"),
		Size:     r.PathValue("size"),
		Rotation: r.PathValue("rotation"),
		Quality:  qualityFormat[:dot],
		Format:   qualityFormat[dot+1:],
	}

	buf, contentType, err := TransformImage(r.Context(), svc.Media, r.PathValue("id"), params)
	if err != nil {
		status := http.StatusInternalServerError
		if iiifErr, ok := err.(*Error); ok {
			status = iiifErr.Status
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(buf)
}
