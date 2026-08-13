package assets

import (
	"context"
	"database/sql"

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
	err := rows.Scan(&r.ID, &r.Title, &r.ReferenceCode, &r.AssetTypeTitle, &r.ClientName, &r.CurrentStateCondition, &r.ProjectCount)
	return r, err
}

func List(ctx context.Context, q studiodb.Querier) ([]ListRow, error) {
	return studiodb.Query(ctx, q, `
		SELECT a.id, a.title, a.referenceCode, at.title, c.name, cs.condition,
		       (SELECT COUNT(*) FROM Project p WHERE p.assetId = a.id) AS projectCount
		FROM Asset a
		JOIN Classifier at ON at.id = a.assetTypeId
		JOIN Client c ON c.id = a.clientId
		LEFT JOIN AssetState cs ON cs.id = a.currentStateId
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3))`,
		id, clientID, assetTypeID, referenceCode, nullIfEmpty(in.Title), nullIfEmpty(in.Artist), nullIfEmpty(in.CreationPeriod),
		nullIfEmpty(in.Dimensions), nullIfEmpty(in.Description), nullIfEmpty(in.Medium), nullIfEmpty(in.SignatureMarks),
		nullIfEmpty(in.Weight), nullIfEmpty(in.Provenance), in.AcquisitionDate, in.EstimatedValue, in.IsInsured,
		nullIfEmpty(in.LocationInStudio))
	return id, err
}

func UpdateProfile(ctx context.Context, q studiodb.Querier, id string, in Input) error {
	_, err := studiodb.Execute(ctx, q, `
		UPDATE Asset SET title = ?, artist = ?, creationPeriod = ?, dimensions = ?, description = ?, medium = ?,
			signatureMarks = ?, weight = ?, provenance = ?, acquisitionDate = ?, estimatedValue = ?, isInsured = ?,
			locationInStudio = ?, updatedAt = NOW(3)
		WHERE id = ?`,
		nullIfEmpty(in.Title), nullIfEmpty(in.Artist), nullIfEmpty(in.CreationPeriod), nullIfEmpty(in.Dimensions),
		nullIfEmpty(in.Description), nullIfEmpty(in.Medium), nullIfEmpty(in.SignatureMarks), nullIfEmpty(in.Weight),
		nullIfEmpty(in.Provenance), in.AcquisitionDate, in.EstimatedValue, in.IsInsured, nullIfEmpty(in.LocationInStudio), id)
	return err
}

// --- Materials -------------------------------------------------------------------------------

func scanMaterial(rows *sql.Rows) (Material, error) {
	var m Material
	err := rows.Scan(&m.ID, &m.MaterialID, &m.Role, &m.Title)
	return m, err
}

func ListMaterials(ctx context.Context, q studiodb.Querier, assetID string) ([]Material, error) {
	return studiodb.Query(ctx, q, `
		SELECT am.id, am.materialId, am.role, c.title FROM AssetMaterial am JOIN Classifier c ON c.id = am.materialId
		WHERE am.assetId = ?`, scanMaterial, assetID)
}

func AddMaterial(ctx context.Context, q studiodb.Querier, assetID, materialID, role string) error {
	existing, err := studiodb.QueryOne(ctx, q, "SELECT id FROM AssetMaterial WHERE assetId = ? AND materialId = ?", scanID, assetID, materialID)
	if err != nil {
		return err
	}
	if existing != nil {
		_, err := studiodb.Execute(ctx, q, "UPDATE AssetMaterial SET role = ? WHERE id = ?", nullIfEmpty(role), *existing)
		return err
	}
	_, err = studiodb.Execute(ctx, q, "INSERT INTO AssetMaterial (id, assetId, materialId, role) VALUES (?, ?, ?, ?)",
		studiodb.NewID(), assetID, materialID, nullIfEmpty(role))
	return err
}

func scanID(rows *sql.Rows) (string, error) {
	var id string
	err := rows.Scan(&id)
	return id, err
}

func RemoveMaterial(ctx context.Context, q studiodb.Querier, assetMaterialID string) error {
	_, err := studiodb.Execute(ctx, q, "DELETE FROM AssetMaterial WHERE id = ?", assetMaterialID)
	return err
}

func AttachMaterialsOnCreate(ctx context.Context, q studiodb.Querier, assetID string, materialIDs []string) error {
	for _, mid := range materialIDs {
		if mid == "" {
			continue
		}
		if _, err := studiodb.Execute(ctx, q, "INSERT INTO AssetMaterial (id, assetId, materialId) VALUES (?, ?, ?)",
			studiodb.NewID(), assetID, mid); err != nil {
			return err
		}
	}
	return nil
}

// --- Condition states (AssetState) ------------------------------------------------------------

func scanState(rows *sql.Rows) (State, error) {
	var s State
	err := rows.Scan(&s.ID, &s.Condition, &s.Description, &s.RecordedAt)
	return s, err
}

func ListStates(ctx context.Context, q studiodb.Querier, assetID string) ([]State, error) {
	return studiodb.Query(ctx, q, "SELECT id, `condition`, description, recordedAt FROM AssetState WHERE assetId = ? ORDER BY recordedAt DESC", scanState, assetID)
}

// RecordState inserts a condition snapshot and updates Asset.currentStateId - the Notebook's
// "fixate state" concept, usable both directly from the Asset page (no project) and later from a
// Workflow's activity log (module 8, which passes projectID/recordedViaActivityID).
func RecordState(ctx context.Context, pool *sql.DB, assetID, condition, description string, projectID, recordedViaActivityID *string) (string, error) {
	return studiodb.WithTransaction(ctx, pool, func(tx *sql.Tx) (string, error) {
		id := studiodb.NewID()
		if _, err := studiodb.Execute(ctx, tx,
			"INSERT INTO AssetState (id, assetId, projectId, recordedViaActivityId, `condition`, description) VALUES (?, ?, ?, ?, ?, ?)",
			id, assetID, ptrToAny(projectID), ptrToAny(recordedViaActivityID), condition, description); err != nil {
			return "", err
		}
		if _, err := studiodb.Execute(ctx, tx, "UPDATE Asset SET currentStateId = ? WHERE id = ?", id, assetID); err != nil {
			return "", err
		}
		return id, nil
	})
}

func ptrToAny(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// --- Workflows summary (Project) --------------------------------------------------------------

func scanProjectSummary(rows *sql.Rows) (ProjectSummary, error) {
	var p ProjectSummary
	err := rows.Scan(&p.ID, &p.Title, &p.Stage, &p.OrderNumber)
	return p, err
}

func ListProjectsForAsset(ctx context.Context, q studiodb.Querier, assetID string) ([]ProjectSummary, error) {
	return studiodb.Query(ctx, q, `
		SELECT p.id, p.title, p.stage, o.orderNumber FROM Project p LEFT JOIN `+"`Order`"+` o ON o.id = p.orderId
		WHERE p.assetId = ? ORDER BY p.createdAt DESC`, scanProjectSummary, assetID)
}
