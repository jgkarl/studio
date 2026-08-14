package export

import (
	"net/http"
	"regexp"
	"strings"

	"studio/internal/auth"
)

func Mount(mux *http.ServeMux, svc *Service, authSvc *auth.Service) {
	mux.HandleFunc("GET /api/export/{type}/{id}", authSvc.RequireUser(svc.handleExport))
}

var filenameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func filenameBase(title string) string {
	base := strings.ToLower(filenameSanitizer.ReplaceAllString(title, "-"))
	base = strings.Trim(base, "-")
	if base == "" {
		return "export"
	}
	return base
}

func (svc *Service) handleExport(w http.ResponseWriter, r *http.Request, _ *auth.User) {
	exportType := r.PathValue("type")
	id := r.PathValue("id")
	if !IsValidType(exportType) {
		http.Error(w, "Unknown exportable type \""+exportType+"\".", http.StatusBadRequest)
		return
	}

	var doc *Doc
	var err error
	switch Type(exportType) {
	case TypeAsset:
		doc, err = svc.GetAssetExportData(r.Context(), id)
	case TypeProject:
		doc, err = svc.GetProjectExportData(r.Context(), id)
	case TypeOrder:
		doc, err = svc.GetOrderExportData(r.Context(), id)
	case TypeReport:
		doc, err = svc.GetReportExportData(r.Context(), id)
	}
	if err != nil || doc == nil {
		http.Error(w, "Not found.", http.StatusNotFound)
		return
	}

	base := filenameBase(doc.Title)

	if r.URL.Query().Get("format") == "pdf" {
		data, err := svc.RenderPDF(r.Context(), doc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.pdf"`)
		_, _ = w.Write(data)
		return
	}

	htmlOut := svc.RenderHTML(r.Context(), doc)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.html"`)
	_, _ = w.Write([]byte(htmlOut))
}
