package assets

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

const assetColumns = `id, clientId, referenceCode, assetTypeId, title, artist, creationPeriod, dimensions,
	description, medium, signatureMarks, weight, provenance, acquisitionDate, estimatedValue, isInsured,
	locationInStudio, currentStateId, createdAt, updatedAt`

func scanAsset(rows *sql.Rows) (Asset, error) {
	var a Asset
	err := rows.Scan(&a.ID, &a.ClientID, &a.ReferenceCode, &a.AssetTypeID, &a.Title, &a.Artist, &a.CreationPeriod,
		&a.Dimensions, &a.Description, &a.Medium, &a.SignatureMarks, &a.Weight, &a.Provenance, &a.AcquisitionDate,
		&a.EstimatedValue, &a.IsInsured, &a.LocationInStudio, &a.CurrentStateID, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func GetByID(ctx context.Context, q studiodb.Querier, id string) (*Asset, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+assetColumns+" FROM Asset WHERE id = ?", scanAsset, id)
}

func scanListRow(rows *sql.Rows) (ListRow, error) {
	var r ListRow
	err := rows.Scan(&r.ID, &r.Title, &r.ReferenceCode, &r.AssetTypeCode, &r.AssetTypeTitle, &r.ClientName, &r.CurrentStateCondition,
		&r.ProjectCount, &r.ThumbnailMediaID)
	return r, err
}

func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT a.id, a.title, a.referenceCode, at.code, at.title, c.name, cs."condition",
		       (SELECT COUNT(*) FROM Project p WHERE p.assetId = a.id AND p.deletedAt IS NULL) AS projectCount,
		       (SELECT mr.mediaId FROM MediaReference mr
		          JOIN Assessment ast ON ast.id = mr.referencingId AND mr.referencingType = 'Assessment'
		          JOIN Media m ON m.id = mr.mediaId AND m.kind = 'image'
		          WHERE ast.assetId = a.id
		          ORDER BY mr.createdAt DESC, mr.sortOrder ASC LIMIT 1) AS thumbnailMediaId
		FROM Asset a
		JOIN Classifier at ON at.id = a.assetTypeId
		JOIN Client c ON c.id = a.clientId
		LEFT JOIN Assessment cs ON cs.id = a.currentStateId
		ORDER BY a.createdAt DESC`, scanListRow)
}

type Input struct {
	Title            string
	Artist           string
	CreationPeriod   string
	Dimensions       string
	Description      string
	Medium           string
	SignatureMarks   string
	Weight           string
	Provenance       string
	AcquisitionDate  any // string "YYYY-MM-DD" or nil
	EstimatedValue   any // float64 or nil
	IsInsured        any // bool or nil (tri-state: unknown/true/false)
	LocationInStudio string
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func Create(ctx context.Context, q studiodb.Querier, clientID, assetTypeID, referenceCode string, in Input) (string, error) {
	id := studiodb.NewID()
	_, err := studiodb.Execute(ctx, q, `
		INSERT INTO Asset (id, clientId, assetTypeId, referenceCode, title, artist, creationPeriod, dimensions,
			description, medium, signatureMarks, weight, provenance, acquisitionDate, estimatedValue, isInsured,
			locationInStudio, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, clientID, assetTypeID, referenceCode, nullIfEmpty(in.Title), nullIfEmpty(in.Artist), nullIfEmpty(in.CreationPeriod),
		nullIfEmpty(in.Dimensions), nullIfEmpty(in.Description), nullIfEmpty(in.Medium), nullIfEmpty(in.SignatureMarks),
		nullIfEmpty(in.Weight), nullIfEmpty(in.Provenance), in.AcquisitionDate, in.EstimatedValue, in.IsInsured,
		nullIfEmpty(in.LocationInStudio), time.Now())
	return id, err
}

func UpdateProfile(ctx context.Context, q studiodb.Querier, id string, in Input) error {
	_, err := studiodb.Execute(ctx, q, `
		UPDATE Asset SET title = ?, artist = ?, creationPeriod = ?, dimensions = ?, description = ?, medium = ?,
			signatureMarks = ?, weight = ?, provenance = ?, acquisitionDate = ?, estimatedValue = ?, isInsured = ?,
			locationInStudio = ?, updatedAt = ?
		WHERE id = ?`,
		nullIfEmpty(in.Title), nullIfEmpty(in.Artist), nullIfEmpty(in.CreationPeriod), nullIfEmpty(in.Dimensions),
		nullIfEmpty(in.Description), nullIfEmpty(in.Medium), nullIfEmpty(in.SignatureMarks), nullIfEmpty(in.Weight),
		nullIfEmpty(in.Provenance), in.AcquisitionDate, in.EstimatedValue, in.IsInsured, nullIfEmpty(in.LocationInStudio), time.Now(), id)
	return err
}

// --- Projects summary ---------------------------------------------------------------------

func scanProjectSummary(rows *sql.Rows) (ProjectSummary, error) {
	var p ProjectSummary
	err := rows.Scan(&p.ID, &p.Title, &p.Stage)
	return p, err
}

func ListProjectsForAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ProjectSummary, error) {
	return studiodb.Query(ctx, q, `
		SELECT p.id, p.title, p.stage FROM Project p
		WHERE p.assetId = ? AND p.deletedAt IS NULL ORDER BY p.createdAt DESC`, scanProjectSummary, assetID)
}
