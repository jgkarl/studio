package media

import (
	"io"
	"net/http"
)

// FilesFromForm reads every non-empty file from a parsed multipart form field into
// UploadedFile values, ready for Service.UploadAllAndAttach. r.ParseMultipartForm must already
// have been called. Shared by every module with a photo-upload form (Assets, Workflows,
// Reporter, ...) so each doesn't hand-roll its own copy.
func FilesFromForm(r *http.Request, fieldName string) []*UploadedFile {
	if r.MultipartForm == nil {
		return nil
	}
	var out []*UploadedFile
	for _, fh := range r.MultipartForm.File[fieldName] {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		out = append(out, &UploadedFile{MimeType: fh.Header.Get("Content-Type"), Data: data})
	}
	return out
}
