package media

import (
	"context"
	"database/sql"
	"fmt"

	studiodb "studio/internal/db"
)

const mediaColumns = `id, storageKey, kind, mimeType, sizeBytes, width, height, durationSeconds,
	checksum, uploadedByUserId, editedFromId, derivedLabel, description, createdAt`

func scanMedia(rows *sql.Rows) (Media, error) {
	var m Media
	err := rows.Scan(&m.ID, &m.StorageKey, &m.Kind, &m.MimeType, &m.SizeBytes, &m.Width, &m.Height,
		&m.DurationSeconds, &m.Checksum, &m.UploadedByID, &m.EditedFromID, &m.DerivedLabel, &m.Description, &m.CreatedAt)
	return m, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Media, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+mediaColumns+" FROM Media WHERE id = ?", scanMedia, id)
}

// ListDerivedVersions returns every annotated version made from this original, oldest first (so
// "annotated", "annotated 2", ... reads in creation order) - empty for a Media that isn't a true
// original, or that has none yet.
func ListDerivedVersions(ctx context.Context, q studiodb.Querier, originalID string) ([]Media, error) {
	return studiodb.Query(ctx, q, "SELECT "+mediaColumns+" FROM Media WHERE editedFromId = ? ORDER BY createdAt ASC", scanMedia, originalID)
}

// NextDerivedLabel computes the label a new annotated version of this original would get -
// "annotated" for the first one, "annotated 2"/"annotated 3"/... after that. Computed once at
// creation (CreateAnnotatedVersion) and stored on the row rather than recomputed on every read, so
// it can't shift under a version if an earlier sibling is later deleted.
func NextDerivedLabel(ctx context.Context, q studiodb.Querier, originalID string) (string, error) {
	n, err := studiodb.QueryOne(ctx, q, "SELECT COUNT(*) AS n FROM Media WHERE editedFromId = ?",
		func(rows *sql.Rows) (int, error) { var c int; err := rows.Scan(&c); return c, err }, originalID)
	if err != nil {
		return "", err
	}
	count := 0
	if n != nil {
		count = *n
	}
	if count == 0 {
		return "annotated", nil
	}
	return fmt.Sprintf("annotated %d", count+1), nil
}

// UpdateDescription saves the media editor's whole-image note.
func UpdateDescription(ctx context.Context, q studiodb.Querier, mediaID, description string) error {
	var val any
	if description != "" {
		val = description
	}
	_, err := studiodb.Execute(ctx, q, "UPDATE Media SET description = ? WHERE id = ?", val, mediaID)
	return err
}

const referenceColumns = "id, mediaId, referencingType, referencingId, role, sortOrder, caption, createdAt"

func scanReference(rows *sql.Rows) (Reference, error) {
	var r Reference
	err := rows.Scan(&r.ID, &r.MediaID, &r.ReferencingType, &r.ReferencingID, &r.Role, &r.SortOrder, &r.Caption, &r.CreatedAt)
	return r, err
}

func GetFirstReference(ctx context.Context, q studiodb.Querier, mediaID string) (*Reference, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+referenceColumns+" FROM MediaReference WHERE mediaId = ? LIMIT 1", scanReference, mediaID)
}

// SetCaption updates one MediaReference's caption — the Report image gallery's per-image
// description field.
func SetCaption(ctx context.Context, q studiodb.Querier, referenceID, caption string) error {
	var arg any
	if caption != "" {
		arg = caption
	}
	_, err := studiodb.Execute(ctx, q, "UPDATE MediaReference SET caption = ? WHERE id = ?", arg, referenceID)
	return err
}
