// Package media is the Media module: local-disk file storage, upload with a web-sized thumbnail
// variant (govips), a lightbox viewer (rotate + brightness/contrast CSS filters, no server round
// trip), and the media grid (every Media row, annotated with which Asset/Project/Client it
// belongs to via the polymorphic MediaReference table).
package media

import (
	"database/sql"
	"time"
)

type Kind string

const (
	KindImage    Kind = "image"
	KindVideo    Kind = "video"
	KindDocument Kind = "document"
)

func KindFromMime(mime string) Kind {
	switch {
	case len(mime) >= 6 && mime[:6] == "image/":
		return KindImage
	case len(mime) >= 6 && mime[:6] == "video/":
		return KindVideo
	default:
		return KindDocument
	}
}

var extByMime = map[string]string{
	"image/jpeg":      "jpg",
	"image/png":       "png",
	"image/webp":      "webp",
	"image/heic":      "heic",
	"image/gif":       "gif",
	"video/mp4":       "mp4",
	"video/quicktime": "mov",
	"video/webm":      "webm",
}

func extFromMime(mime string) string {
	if ext, ok := extByMime[mime]; ok {
		return ext
	}
	return "bin"
}

type Media struct {
	ID              string
	StorageKey      string
	Kind            Kind
	MimeType        string
	SizeBytes       int64
	Width           sql.NullInt64
	Height          sql.NullInt64
	DurationSeconds sql.NullInt64
	Checksum        string
	UploadedByID    string
	EditedFromID    sql.NullString
	CreatedAt       time.Time
}

// ReferencingType — the polymorphic MediaReference.referencingType values (matches lib/types.ts).
type ReferencingType string

const (
	RefActivity   ReferencingType = "Activity"
	RefAssessment ReferencingType = "Assessment" // was "AssetState" pre-Project-scope refactor
	RefReport     ReferencingType = "Report"
	RefAsset      ReferencingType = "Asset"
	RefProject    ReferencingType = "Project"
	RefTreatment  ReferencingType = "Treatment"
)

type Reference struct {
	ID              string
	MediaID         string
	ReferencingType ReferencingType
	ReferencingID   string
	Role            sql.NullString
	SortOrder       int
	Caption         sql.NullString
	CreatedAt       time.Time
}

type ReferenceWithMedia struct {
	Reference
	Media Media
}
