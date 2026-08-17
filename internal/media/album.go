package media

import (
	"context"
	"database/sql"
	"time"

	studiodb "studio/internal/db"
)

// AlbumItem is one Media row annotated with which Asset/Project/Client it belongs to — every
// photo/video/document uploaded anywhere in the app, in one place. Workflow (Project) grouping
// works from day one here even though the Workflows module (8) hasn't landed yet - the schema
// and MediaReference→Activity/AssetState resolution chain already fully support it.
type AlbumItem struct {
	ID              string
	Kind            Kind
	MimeType        string
	Width           sql.NullInt64
	Height          sql.NullInt64
	DurationSeconds sql.NullInt64
	CreatedAt       time.Time
	Role            sql.NullString
	UploadedByName  string
	AssetID         sql.NullString
	AssetTitle      sql.NullString
	ProjectID       sql.NullString
	ProjectTitle    sql.NullString
	ClientName      sql.NullString
}

type mediaWithUploader struct {
	ID, MimeType, UploadedByName string
	Kind                         Kind
	Width, Height, Duration      sql.NullInt64
	CreatedAt                    time.Time
}

type refRow struct {
	ID, MediaID, ReferencingID string
	CreatedAt                  time.Time
	ReferencingType            ReferencingType
	Role                       sql.NullString
}

// GetAllMediaWithContext resolves MediaReference's polymorphic target (Activity/AssetState/
// Report/Asset/Treatment) down to a concrete Asset/Project/Client via a few batched queries and
// in-memory joins - there's no single JOINable FK for a polymorphic association like this.
func GetAllMediaWithContext(ctx context.Context, q studiodb.Querier) ([]AlbumItem, error) {
	scanMediaU := func(rows *sql.Rows) (mediaWithUploader, error) {
		var m mediaWithUploader
		err := rows.Scan(&m.ID, &m.Kind, &m.MimeType, &m.Width, &m.Height, &m.Duration, &m.CreatedAt, &m.UploadedByName)
		return m, err
	}
	mediaRows, err := studiodb.Query(ctx, q, `
		SELECT m.id, m.kind, m.mimeType, m.width, m.height, m.durationSeconds, m.createdAt, u.name
		FROM Media m JOIN User u ON u.id = m.uploadedByUserId ORDER BY m.createdAt DESC`, scanMediaU)
	if err != nil {
		return nil, err
	}

	scanRef := func(rows *sql.Rows) (refRow, error) {
		var r refRow
		err := rows.Scan(&r.ID, &r.MediaID, &r.ReferencingType, &r.ReferencingID, &r.Role, &r.CreatedAt)
		return r, err
	}
	refs, err := studiodb.Query(ctx, q,
		"SELECT id, mediaId, referencingType, referencingId, role, createdAt FROM MediaReference ORDER BY createdAt ASC", scanRef)
	if err != nil {
		return nil, err
	}

	// One reference per media (the first created) - the upload flow attaches media exactly once
	// in practice.
	refByMediaID := map[string]refRow{}
	for _, r := range refs {
		if _, ok := refByMediaID[r.MediaID]; !ok {
			refByMediaID[r.MediaID] = r
		}
	}

	var assetStateIDs, activityIDs, reportIDs, treatmentIDs []string
	for _, r := range refs {
		switch r.ReferencingType {
		case RefAssetState:
			assetStateIDs = append(assetStateIDs, r.ReferencingID)
		case RefActivity:
			activityIDs = append(activityIDs, r.ReferencingID)
		case RefReport:
			reportIDs = append(reportIDs, r.ReferencingID)
		case RefTreatment:
			treatmentIDs = append(treatmentIDs, r.ReferencingID)
		}
	}

	type assetStateRow struct {
		ID, AssetID string
		ProjectID   sql.NullString
	}
	assetStates, err := queryByIDs(ctx, q, "SELECT id, assetId, projectId FROM AssetState", "id", assetStateIDs,
		func(rows *sql.Rows) (assetStateRow, error) {
			var s assetStateRow
			err := rows.Scan(&s.ID, &s.AssetID, &s.ProjectID)
			return s, err
		})
	if err != nil {
		return nil, err
	}
	type activityRow struct{ ID, ProjectID string }
	activities, err := queryByIDs(ctx, q, "SELECT id, projectId FROM Activity", "id", activityIDs,
		func(rows *sql.Rows) (activityRow, error) {
			var a activityRow
			err := rows.Scan(&a.ID, &a.ProjectID)
			return a, err
		})
	if err != nil {
		return nil, err
	}
	type reportRow struct {
		ID, AssetID string
		ProjectID   sql.NullString
	}
	reports, err := queryByIDs(ctx, q, "SELECT id, assetId, projectId FROM Report", "id", reportIDs,
		func(rows *sql.Rows) (reportRow, error) {
			var r reportRow
			err := rows.Scan(&r.ID, &r.AssetID, &r.ProjectID)
			return r, err
		})
	if err != nil {
		return nil, err
	}
	type treatmentRow struct{ ID, AssetID string }
	treatments, err := queryByIDs(ctx, q, "SELECT id, assetId FROM Treatment", "id", treatmentIDs,
		func(rows *sql.Rows) (treatmentRow, error) {
			var t treatmentRow
			err := rows.Scan(&t.ID, &t.AssetID)
			return t, err
		})
	if err != nil {
		return nil, err
	}

	assetStateByID := map[string]assetStateRow{}
	for _, s := range assetStates {
		assetStateByID[s.ID] = s
	}
	activityByID := map[string]activityRow{}
	for _, a := range activities {
		activityByID[a.ID] = a
	}
	reportByID := map[string]reportRow{}
	for _, r := range reports {
		reportByID[r.ID] = r
	}
	treatmentByID := map[string]treatmentRow{}
	for _, t := range treatments {
		treatmentByID[t.ID] = t
	}

	projectIDSet := map[string]bool{}
	var directAssetIDs []string
	for _, r := range refs {
		if r.ReferencingType == RefAsset {
			directAssetIDs = append(directAssetIDs, r.ReferencingID)
		}
	}
	for _, a := range activities {
		if a.ProjectID != "" {
			projectIDSet[a.ProjectID] = true
		}
	}
	for _, s := range assetStates {
		if s.ProjectID.Valid {
			projectIDSet[s.ProjectID.String] = true
		}
	}
	for _, r := range reports {
		if r.ProjectID.Valid {
			projectIDSet[r.ProjectID.String] = true
		}
	}
	var allProjectIDs []string
	for id := range projectIDSet {
		allProjectIDs = append(allProjectIDs, id)
	}

	type projectRow struct{ ID, Title, AssetID string }
	projects, err := queryByIDs(ctx, q, "SELECT id, title, assetId FROM Project", "id", allProjectIDs,
		func(rows *sql.Rows) (projectRow, error) {
			var p projectRow
			err := rows.Scan(&p.ID, &p.Title, &p.AssetID)
			return p, err
		})
	if err != nil {
		return nil, err
	}
	projectByID := map[string]projectRow{}
	for _, p := range projects {
		projectByID[p.ID] = p
	}

	assetIDSet := map[string]bool{}
	for _, id := range directAssetIDs {
		assetIDSet[id] = true
	}
	for _, s := range assetStates {
		assetIDSet[s.AssetID] = true
	}
	for _, r := range reports {
		assetIDSet[r.AssetID] = true
	}
	for _, p := range projects {
		assetIDSet[p.AssetID] = true
	}
	for _, t := range treatments {
		assetIDSet[t.AssetID] = true
	}
	var allAssetIDs []string
	for id := range assetIDSet {
		allAssetIDs = append(allAssetIDs, id)
	}

	type assetRow struct {
		ID, ReferenceCode, ClientName string
		Title                         sql.NullString
	}
	assetRows, err := queryByIDs(ctx, q, "SELECT a.id, a.title, a.referenceCode, c.name FROM Asset a JOIN Client c ON c.id = a.clientId", "a.id", allAssetIDs,
		func(rows *sql.Rows) (assetRow, error) {
			var a assetRow
			err := rows.Scan(&a.ID, &a.Title, &a.ReferenceCode, &a.ClientName)
			return a, err
		})
	if err != nil {
		return nil, err
	}
	assetByID := map[string]assetRow{}
	for _, a := range assetRows {
		assetByID[a.ID] = a
	}

	resolveContext := func(ref *refRow) (assetID, assetTitle, projectID, projectTitle, clientName sql.NullString) {
		var aID, pID string
		if ref != nil {
			switch ref.ReferencingType {
			case RefAsset:
				aID = ref.ReferencingID
			case RefAssetState:
				if st, ok := assetStateByID[ref.ReferencingID]; ok {
					aID = st.AssetID
					pID = st.ProjectID.String
				}
			case RefActivity:
				if act, ok := activityByID[ref.ReferencingID]; ok {
					pID = act.ProjectID
					if p, ok := projectByID[pID]; ok {
						aID = p.AssetID
					}
				}
			case RefReport:
				if rep, ok := reportByID[ref.ReferencingID]; ok {
					aID = rep.AssetID
					pID = rep.ProjectID.String
				}
			case RefTreatment:
				if t, ok := treatmentByID[ref.ReferencingID]; ok {
					aID = t.AssetID
				}
			}
		}
		if aID != "" {
			assetID = sql.NullString{String: aID, Valid: true}
			if a, ok := assetByID[aID]; ok {
				title := a.ReferenceCode
				if a.Title.Valid && a.Title.String != "" {
					title = a.Title.String
				}
				assetTitle = sql.NullString{String: title, Valid: true}
				clientName = sql.NullString{String: a.ClientName, Valid: true}
			}
		}
		if pID != "" {
			projectID = sql.NullString{String: pID, Valid: true}
			if p, ok := projectByID[pID]; ok {
				projectTitle = sql.NullString{String: p.Title, Valid: true}
			}
		}
		return
	}

	items := make([]AlbumItem, len(mediaRows))
	for i, m := range mediaRows {
		ref, hasRef := refByMediaID[m.ID]
		var refPtr *refRow
		if hasRef {
			refPtr = &ref
		}
		assetID, assetTitle, projectID, projectTitle, clientName := resolveContext(refPtr)
		items[i] = AlbumItem{
			ID: m.ID, Kind: m.Kind, MimeType: m.MimeType, Width: m.Width, Height: m.Height,
			DurationSeconds: m.Duration, CreatedAt: m.CreatedAt, UploadedByName: m.UploadedByName,
			Role: ref.Role, AssetID: assetID, AssetTitle: assetTitle, ProjectID: projectID,
			ProjectTitle: projectTitle, ClientName: clientName,
		}
	}
	return items, nil
}

// queryByIDs runs baseSQL + "WHERE <idColumn> IN (...)" for a batch of IDs, or returns nil
// without querying at all when ids is empty (matches the original's
// `ids.length ? query(...) : []`). idColumn should be table-qualified (e.g. "a.id") whenever
// baseSQL joins another table with its own id column, to avoid an ambiguous-column error.
func queryByIDs[T any](ctx context.Context, q studiodb.Querier, baseSQL, idColumn string, ids []string, scan studiodb.ScanFunc[T]) ([]T, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := ""
	args := make([]any, len(ids))
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = id
	}
	return studiodb.Query(ctx, q, baseSQL+" WHERE "+idColumn+" IN ("+placeholders+")", scan, args...)
}
