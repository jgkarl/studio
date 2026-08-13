package media

import (
	"context"
	"database/sql"

	studiodb "studio/internal/db"
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

func ListEditsOf(ctx context.Context, q studiodb.Querier, mediaID string) ([]Media, error) {
	return studiodb.Query(ctx, q, "SELECT "+mediaColumns+" FROM Media WHERE editedFromId = ?", scanMedia, mediaID)
}

const referenceColumns = "id, mediaId, referencingType, referencingId, role, sortOrder, createdAt"

func scanReference(rows *sql.Rows) (Reference, error) {
	var r Reference
	err := rows.Scan(&r.ID, &r.MediaID, &r.ReferencingType, &r.ReferencingID, &r.Role, &r.SortOrder, &r.CreatedAt)
	return r, err
}

func GetFirstReference(ctx context.Context, q studiodb.Querier, mediaID string) (*Reference, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+referenceColumns+" FROM MediaReference WHERE mediaId = ? LIMIT 1", scanReference, mediaID)
}
