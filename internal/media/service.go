package media

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"

	"github.com/davidbyttow/govips/v2/vips"

	studiodb "studio/internal/db"
)

type Service struct {
	Pool    *sql.DB
	Storage StorageAdapter
}

// storeImageVariants stores a "web" thumbnail conversion (max 1600x1600, EXIF auto-rotated,
// JPEG q80) alongside the original — non-fatal on failure (an undecodable/corrupt image still
// keeps its original servable as-is).
func (s *Service) storeImageVariants(mediaID string, buf []byte) (width, height int) {
	img, err := vips.NewImageFromBuffer(buf)
	if err != nil {
		log.Printf("media: decoding image for variant generation: %v", err)
		return 0, 0
	}
	defer img.Close()

	if err := img.AutoRotate(); err != nil {
		log.Printf("media: auto-rotating image: %v", err)
	}

	width, height = img.Width(), img.Height()
	const maxDim = 1600
	if width > maxDim || height > maxDim {
		scale := 1.0
		if ws, hs := float64(maxDim)/float64(width), float64(maxDim)/float64(height); ws < hs {
			scale = ws
		} else {
			scale = hs
		}
		if err := img.Resize(scale, vips.KernelAuto); err != nil {
			log.Printf("media: resizing web variant: %v", err)
			return width, height
		}
	}

	params := vips.NewJpegExportParams()
	params.Quality = 80
	webBytes, _, err := img.ExportJpeg(params)
	if err != nil {
		log.Printf("media: exporting web variant: %v", err)
		return width, height
	}
	if err := s.Storage.Put(mediaID+"/web.jpg", webBytes); err != nil {
		log.Printf("media: storing web variant: %v", err)
	}
	return width, height
}

// UploadMedia stores one file's original bytes plus (for images) a web thumbnail variant.
func (s *Service) UploadMedia(ctx context.Context, data []byte, mimeType, uploadedByUserID string) (*Media, error) {
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])
	kind := KindFromMime(mimeType)

	id := studiodb.NewID()
	if _, err := studiodb.Execute(ctx, s.Pool,
		"INSERT INTO Media (id, storageKey, kind, mimeType, sizeBytes, checksum, uploadedByUserId) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, "", kind, mimeType, len(data), checksum, uploadedByUserID); err != nil {
		return nil, err
	}

	originalKey := id + "/original." + extFromMime(mimeType)
	if err := s.Storage.Put(originalKey, data); err != nil {
		return nil, err
	}

	var width, height int
	if kind == KindImage {
		width, height = s.storeImageVariants(id, data)
	}

	if _, err := studiodb.Execute(ctx, s.Pool, "UPDATE Media SET storageKey = ?, width = ?, height = ? WHERE id = ?",
		originalKey, nullIfZero(width), nullIfZero(height), id); err != nil {
		return nil, err
	}
	return GetByID(ctx, s.Pool, id)
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func (s *Service) AttachMediaReference(ctx context.Context, mediaID string, refType ReferencingType, refID, role string, sortOrder int) error {
	_, err := studiodb.Execute(ctx, s.Pool,
		"INSERT INTO MediaReference (id, mediaId, referencingType, referencingId, role, sortOrder) VALUES (?, ?, ?, ?, ?, ?)",
		studiodb.NewID(), mediaID, refType, refID, nullIfEmptyStr(role), sortOrder)
	return err
}

// UnlinkReference removes just the MediaReference join row — the Media itself is shared library
// content (may still be referenced elsewhere: an Assessment, an Activity, a Report cover) and is
// never touched by this.
func (s *Service) UnlinkReference(ctx context.Context, referenceID string) error {
	_, err := studiodb.Execute(ctx, s.Pool, "DELETE FROM MediaReference WHERE id = ?", referenceID)
	return err
}

func nullIfEmptyStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// UploadedFile is what handlers build from a parsed multipart form - decoupled from net/http so
// this package doesn't need to know about HTTP.
type UploadedFile struct {
	MimeType string
	Data     []byte
}

// UploadAndAttach uploads one file (if non-empty) and immediately attaches it to a target
// record.
func (s *Service) UploadAndAttach(ctx context.Context, file *UploadedFile, uploadedByUserID string, refType ReferencingType, refID, role string) (*Media, error) {
	if file == nil || len(file.Data) == 0 {
		return nil, nil
	}
	m, err := s.UploadMedia(ctx, file.Data, file.MimeType, uploadedByUserID)
	if err != nil {
		return nil, err
	}
	if err := s.AttachMediaReference(ctx, m.ID, refType, refID, role, 0); err != nil {
		return nil, err
	}
	return m, nil
}

// UploadAllAndAttach is UploadAndAttach for every file in a multi-file <input>.
func (s *Service) UploadAllAndAttach(ctx context.Context, files []*UploadedFile, uploadedByUserID string, refType ReferencingType, refID, role string) ([]*Media, error) {
	var out []*Media
	for _, f := range files {
		m, err := s.UploadAndAttach(ctx, f, uploadedByUserID, refType, refID, role)
		if err != nil {
			return nil, err
		}
		if m != nil {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Service) GetReferencedMedia(ctx context.Context, refType ReferencingType, refID string) ([]ReferenceWithMedia, error) {
	refs, err := studiodb.Query(ctx, s.Pool,
		"SELECT "+referenceColumns+" FROM MediaReference WHERE referencingType = ? AND referencingId = ? ORDER BY sortOrder ASC",
		scanReference, refType, refID)
	if err != nil || len(refs) == 0 {
		return nil, err
	}
	out := make([]ReferenceWithMedia, 0, len(refs))
	for _, r := range refs {
		m, err := GetByID(ctx, s.Pool, r.MediaID)
		if err != nil {
			return nil, err
		}
		if m == nil {
			continue
		}
		out = append(out, ReferenceWithMedia{Reference: r, Media: *m})
	}
	return out, nil
}

type FileVariant struct {
	Data     []byte
	MimeType string
}

// ReadMediaFile returns the "web" variant for images (falling back to the original if no
// variant was generated) or the original for everything else.
func (s *Service) ReadMediaFile(ctx context.Context, mediaID string, variant string) (*FileVariant, error) {
	m, err := GetByID(ctx, s.Pool, mediaID)
	if err != nil || m == nil {
		return nil, err
	}
	if variant != "original" && m.Kind == KindImage {
		if data, err := s.Storage.Get(mediaID + "/web.jpg"); err == nil {
			return &FileVariant{Data: data, MimeType: "image/jpeg"}, nil
		}
	}
	data, err := s.Storage.Get(m.StorageKey)
	if err != nil {
		return nil, err
	}
	return &FileVariant{Data: data, MimeType: m.MimeType}, nil
}
