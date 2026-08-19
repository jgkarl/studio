package media

import (
	"context"
	"database/sql"

	studiodb "stuudio/internal/db"
)

const mediaColumns = `id, storageKey, kind, mimeType, sizeBytes, width, height, durationSeconds,
	checksum, uploadedByUserId, editedFromId, createdAt`

func scanMedia(rows *sql.Rows) (Media, error) {
	var m Media
	err := rows.Scan(&m.ID, &m.StorageKey, &m.Kind, &m.MimeType, &m.SizeBytes, &m.Width, &m.Height,
		&m.DurationSeconds, &m.Checksum, &m.UploadedByID, &m.EditedFromID, &m.CreatedAt)
	return m, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Media, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+mediaColumns+" FROM Media WHERE id = ?", scanMedia, id)
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
