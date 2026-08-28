package settings

import (
	"context"
	"database/sql"

	studiodb "studio/internal/db"
)

// AssetTypeStat is one Asset Type classifier's usage count, for the Settings -> Statistics tab.
type AssetTypeStat struct {
	Classifier Classifier
	AssetCount int
}

// AnnotationTypeStat is one Annotation Type classifier's usage across the pattern-layer: how many
// regions have been drawn with it overall, and how many distinct Assets carry at least one.
type AnnotationTypeStat struct {
	Classifier  Classifier
	RegionCount int
	AssetCount  int
}

// LoadAssetTypeStats counts every Asset per Asset Type classifier (including inactive types, so a
// retired type with existing Assets still shows a real count rather than silently disappearing).
func LoadAssetTypeStats(ctx context.Context, q studiodb.Querier) ([]AssetTypeStat, error) {
	types, err := GetAllClassifiers(ctx, q, ClassifierAssetType)
	if err != nil {
		return nil, err
	}
	out := make([]AssetTypeStat, 0, len(types))
	for _, t := range types {
		n, err := studiodb.QueryOne(ctx, q, "SELECT COUNT(*) AS n FROM Asset WHERE assetTypeId = ?", scanCount, t.ID)
		if err != nil {
			return nil, err
		}
		count := 0
		if n != nil {
			count = *n
		}
		out = append(out, AssetTypeStat{Classifier: t, AssetCount: count})
	}
	return out, nil
}

type typeCount struct {
	TypeID string
	N      int
}

func scanTypeCount(rows *sql.Rows) (typeCount, error) {
	var c typeCount
	err := rows.Scan(&c.TypeID, &c.N)
	return c, err
}

// LoadAnnotationTypeStats resolves each MediaAnnotationRegion back to the Asset it was drawn on.
// Regions always live on an "annotated version" Media row (internal/media/rasterize.go); it's that
// version's true original (Media.editedFromId) that's actually linked to an Asset via
// MediaReference - directly (referencingType "Asset"), or through a Project/Assessment/Treatment/
// Report, all four of which carry their own denormalized assetId (see
// db/migrations/0015_project_scoped_records.sql) so no further hop through Project is needed for
// any of them.
func LoadAnnotationTypeStats(ctx context.Context, q studiodb.Querier) ([]AnnotationTypeStat, error) {
	types, err := GetAllClassifiers(ctx, q, ClassifierAnnotationType)
	if err != nil {
		return nil, err
	}

	regionRows, err := studiodb.Query(ctx, q,
		"SELECT annotationTypeId, COUNT(*) AS n FROM MediaAnnotationRegion GROUP BY annotationTypeId",
		scanTypeCount)
	if err != nil {
		return nil, err
	}
	regionByType := make(map[string]int, len(regionRows))
	for _, c := range regionRows {
		regionByType[c.TypeID] = c.N
	}

	assetRows, err := studiodb.Query(ctx, q, `
		WITH RegionAsset AS (
			SELECT r.annotationTypeId AS annotationTypeId,
				CASE mr.referencingType
					WHEN 'Asset' THEN mr.referencingId
					WHEN 'Project' THEN p.assetId
					WHEN 'Assessment' THEN asm.assetId
					WHEN 'Treatment' THEN tr.assetId
					WHEN 'Report' THEN rp.assetId
				END AS assetId
			FROM MediaAnnotationRegion r
			JOIN Media m ON m.id = r.mediaId
			JOIN Media orig ON orig.id = COALESCE(m.editedFromId, m.id)
			JOIN MediaReference mr ON mr.mediaId = orig.id
			LEFT JOIN Project p ON mr.referencingType = 'Project' AND p.id = mr.referencingId
			LEFT JOIN Assessment asm ON mr.referencingType = 'Assessment' AND asm.id = mr.referencingId
			LEFT JOIN Treatment tr ON mr.referencingType = 'Treatment' AND tr.id = mr.referencingId
			LEFT JOIN Report rp ON mr.referencingType = 'Report' AND rp.id = mr.referencingId
		)
		SELECT annotationTypeId, COUNT(DISTINCT assetId) AS n FROM RegionAsset WHERE assetId IS NOT NULL GROUP BY annotationTypeId
	`, scanTypeCount)
	if err != nil {
		return nil, err
	}
	assetByType := make(map[string]int, len(assetRows))
	for _, c := range assetRows {
		assetByType[c.TypeID] = c.N
	}

	out := make([]AnnotationTypeStat, 0, len(types))
	for _, t := range types {
		out = append(out, AnnotationTypeStat{Classifier: t, RegionCount: regionByType[t.ID], AssetCount: assetByType[t.ID]})
	}
	return out, nil
}
