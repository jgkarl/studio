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

// CreateAnnotatedVersion starts a new annotation session on an original image: inserts a draft
// Media row (EditedFromID pointing at the original, DerivedLabel computed via NextDerivedLabel) so
// the drag-to-draw editor has something to attach regions to via POST /media/{id}/annotations
// immediately - it has no real file/thumbnail on disk yet (SizeBytes 0, Width/Height NULL; see
// Media.IsBaked) until the first BakeAnnotatedVersion call, which happens when the editor closes
// (static/js/lightbox.js). Every MediaReference the original has is copied onto the new draft too,
// so the annotated version shows up in the same Asset/Project/etc. context the original does, not
// just reachable by drilling in from the original's own page.
func (s *Service) CreateAnnotatedVersion(ctx context.Context, original Media, uploadedByUserID string) (*Media, error) {
	label, err := NextDerivedLabel(ctx, s.Pool, original.ID)
	if err != nil {
		return nil, err
	}

	id := studiodb.NewID()
	placeholderKey := id + "/original.png"
	if _, err := studiodb.Execute(ctx, s.Pool,
		`INSERT INTO Media (id, storageKey, kind, mimeType, sizeBytes, checksum, uploadedByUserId, editedFromId, derivedLabel)
		 VALUES (?, ?, ?, 'image/png', 0, '', ?, ?, ?)`,
		id, placeholderKey, KindImage, uploadedByUserID, original.ID, label); err != nil {
		return nil, err
	}

	refs, err := studiodb.Query(ctx, s.Pool, "SELECT "+referenceColumns+" FROM MediaReference WHERE mediaId = ?", scanReference, original.ID)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if err := s.AttachMediaReference(ctx, id, ref.ReferencingType, ref.ReferencingID, ref.Role.String, ref.SortOrder); err != nil {
			return nil, err
		}
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
	return s.AttachMediaReferenceWithCaption(ctx, mediaID, refType, refID, role, "", sortOrder)
}

// AttachMediaReferenceWithCaption is AttachMediaReference plus an upfront caption - lets an
// upload set its own name/caption in the same step instead of always needing a separate
// after-the-fact edit (see web.FileInput's optional caption field).
func (s *Service) AttachMediaReferenceWithCaption(ctx context.Context, mediaID string, refType ReferencingType, refID, role, caption string, sortOrder int) error {
	_, err := studiodb.Execute(ctx, s.Pool,
		"INSERT INTO MediaReference (id, mediaId, referencingType, referencingId, role, caption, sortOrder) VALUES (?, ?, ?, ?, ?, ?, ?)",
		studiodb.NewID(), mediaID, refType, refID, nullIfEmptyStr(role), nullIfEmptyStr(caption), sortOrder)
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
	return s.UploadAndAttachWithCaption(ctx, file, uploadedByUserID, refType, refID, role, "")
}

// UploadAndAttachWithCaption is UploadAndAttach plus an upfront caption for the new reference -
// see AttachMediaReferenceWithCaption.
func (s *Service) UploadAndAttachWithCaption(ctx context.Context, file *UploadedFile, uploadedByUserID string, refType ReferencingType, refID, role, caption string) (*Media, error) {
	if file == nil || len(file.Data) == 0 {
		return nil, nil
	}
	m, err := s.UploadMedia(ctx, file.Data, file.MimeType, uploadedByUserID)
	if err != nil {
		return nil, err
	}
	if err := s.AttachMediaReferenceWithCaption(ctx, m.ID, refType, refID, role, caption, 0); err != nil {
		return nil, err
	}
	return m, nil
}

// UploadAllAndAttach is UploadAndAttach for every file in a multi-file <input>.
func (s *Service) UploadAllAndAttach(ctx context.Context, files []*UploadedFile, uploadedByUserID string, refType ReferencingType, refID, role string) ([]*Media, error) {
	return s.UploadAllAndAttachWithCaption(ctx, files, uploadedByUserID, refType, refID, role, "")
}

// UploadAllAndAttachWithCaption is UploadAllAndAttach, giving every file in the batch the same
// caption - the one web.FileInput's optional name/caption field collects for the whole picker,
// not a separate per-file name (a multi-file <input> has no way to solicit one per file).
func (s *Service) UploadAllAndAttachWithCaption(ctx context.Context, files []*UploadedFile, uploadedByUserID string, refType ReferencingType, refID, role, caption string) ([]*Media, error) {
	var out []*Media
	for _, f := range files {
		m, err := s.UploadAndAttachWithCaption(ctx, f, uploadedByUserID, refType, refID, role, caption)
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
