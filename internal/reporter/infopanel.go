package reporter

import (
	"context"
	"database/sql"
	"sort"
	"strconv"

	studiodb "stuudio/internal/db"
	"stuudio/internal/media"
	"stuudio/internal/settings"
)

// clientInfoRow/assetInfoRow/projectInfoRow are local, denormalized reads of exactly the fields
// the Client/Asset/Project "reportable fields" registry (internal/settings/reportable.go) can
// toggle — plain raw SQL rather than importing internal/clients/assets/workflows, which already
// import this package (for their own Reports sections) and would cycle back.
type clientInfoRow struct {
	ID                                                                   string
	Name, OrganizationName, ContactPerson, Email, Phone, Address, Notes sql.NullString
}

type assetInfoRow struct {
	Title, Artist, CreationPeriod, Dimensions, Medium, Description, Provenance, SignatureMarks sql.NullString
	EstimatedValue                                                                             sql.NullFloat64
}

type projectInfoRow struct {
	Title            string
	Priority         string
	TargetReviewDate sql.NullTime
	AssignedToName   sql.NullString
}

// InfoField is one label/value pair already filtered to reportable-enabled fields — ready to
// render directly in the Report's Client/Asset/Project info panel.
type InfoField struct {
	Label, Value string
}

func fieldsFor(model string, enabled map[string]bool, get func(field string) string) []InfoField {
	var out []InfoField
	for _, f := range settings.ReportableFields {
		if f.Model != model || !enabled[f.Field] {
			continue
		}
		if v := get(f.Field); v != "" {
			out = append(out, InfoField{Label: f.Label, Value: v})
		}
	}
	return out
}

func clientField(row clientInfoRow, field string) string {
	switch field {
	case "name":
		return row.Name.String
	case "organizationName":
		return row.OrganizationName.String
	case "contactPerson":
		return row.ContactPerson.String
	case "email":
		return row.Email.String
	case "phone":
		return row.Phone.String
	case "address":
		return row.Address.String
	case "notes":
		return row.Notes.String
	default:
		return ""
	}
}

func assetField(row assetInfoRow, field string) string {
	switch field {
	case "title":
		return row.Title.String
	case "artist":
		return row.Artist.String
	case "creationPeriod":
		return row.CreationPeriod.String
	case "dimensions":
		return row.Dimensions.String
	case "medium":
		return row.Medium.String
	case "description":
		return row.Description.String
	case "provenance":
		return row.Provenance.String
	case "signatureMarks":
		return row.SignatureMarks.String
	case "estimatedValue":
		if !row.EstimatedValue.Valid {
			return ""
		}
		return strconv.FormatFloat(row.EstimatedValue.Float64, 'f', 2, 64)
	default:
		return ""
	}
}

func projectField(row projectInfoRow, field string) string {
	switch field {
	case "title":
		return row.Title
	case "priority":
		return row.Priority
	case "targetReviewDate":
		if !row.TargetReviewDate.Valid {
			return ""
		}
		return row.TargetReviewDate.Time.Format("2006-01-02")
	case "assignedTo":
		return row.AssignedToName.String
	default:
		return ""
	}
}

// InfoPanel is the Report's Client/Asset/Project info blocks, already filtered to whichever
// fields Settings -> Features -> "Reportable fields" has enabled.
type InfoPanel struct {
	Client   []InfoField
	ClientID string
	Asset    []InfoField
	Project  []InfoField
}

func BuildInfoPanel(ctx context.Context, q studiodb.Querier, assetID, projectID string) (InfoPanel, error) {
	groups := settings.LoadReportableGroups(ctx, q)
	enabled := map[string]map[string]bool{}
	for _, g := range groups {
		m := map[string]bool{}
		for _, f := range g.Fields {
			m[f.Field] = f.Enabled
		}
		enabled[g.Model] = m
	}

	asset, err := studiodb.QueryOne(ctx, q, `
		SELECT title, artist, creationPeriod, dimensions, medium, description, provenance, signatureMarks, estimatedValue
		FROM Asset WHERE id = ?`,
		func(rows *sql.Rows) (assetInfoRow, error) {
			var a assetInfoRow
			err := rows.Scan(&a.Title, &a.Artist, &a.CreationPeriod, &a.Dimensions, &a.Medium, &a.Description, &a.Provenance, &a.SignatureMarks, &a.EstimatedValue)
			return a, err
		}, assetID)
	if err != nil {
		return InfoPanel{}, err
	}

	client, err := studiodb.QueryOne(ctx, q, `
		SELECT c.id, c.name, c.organizationName, c.contactPerson, c.email, c.phone, c.address, c.notes
		FROM Client c JOIN Asset a ON a.clientId = c.id WHERE a.id = ?`,
		func(rows *sql.Rows) (clientInfoRow, error) {
			var c clientInfoRow
			err := rows.Scan(&c.ID, &c.Name, &c.OrganizationName, &c.ContactPerson, &c.Email, &c.Phone, &c.Address, &c.Notes)
			return c, err
		}, assetID)
	if err != nil {
		return InfoPanel{}, err
	}

	project, err := studiodb.QueryOne(ctx, q, `
		SELECT p.title, p.priority, p.targetReviewDate, u.name
		FROM Project p LEFT JOIN User u ON u.id = p.assignedToUserId WHERE p.id = ?`,
		func(rows *sql.Rows) (projectInfoRow, error) {
			var p projectInfoRow
			err := rows.Scan(&p.Title, &p.Priority, &p.TargetReviewDate, &p.AssignedToName)
			return p, err
		}, projectID)
	if err != nil {
		return InfoPanel{}, err
	}

	panel := InfoPanel{}
	if client != nil {
		panel.Client = fieldsFor("client", enabled["client"], func(f string) string { return clientField(*client, f) })
		panel.ClientID = client.ID
	}
	if asset != nil {
		panel.Asset = fieldsFor("asset", enabled["asset"], func(f string) string { return assetField(*asset, f) })
	}
	if project != nil {
		panel.Project = fieldsFor("project", enabled["project"], func(f string) string { return projectField(*project, f) })
	}
	return panel, nil
}

// GalleryItem is one image/video in the Report's timestamp-ordered gallery, spanning every
// Assessment/Treatment/Report/Project-direct upload the Project has — not just this Report's own
// attachments.
type GalleryItem struct {
	media.ReferenceWithMedia
}

// BuildGallery collects every Media reference reachable from a Project (its own direct uploads,
// plus every Assessment/Treatment/Report's own attachments), oldest first — the report's "image
// gallery order by timestamp" requirement.
func BuildGallery(ctx context.Context, mediaSvc *media.Service, q studiodb.Querier, projectID string) ([]GalleryItem, error) {
	var items []GalleryItem

	addRefs := func(refType media.ReferencingType, refID string) error {
		refs, err := mediaSvc.GetReferencedMedia(ctx, refType, refID)
		if err != nil {
			return err
		}
		for _, r := range refs {
			items = append(items, GalleryItem{r})
		}
		return nil
	}

	if err := addRefs(media.RefProject, projectID); err != nil {
		return nil, err
	}

	assessmentIDs, err := studiodb.Query(ctx, q, "SELECT id FROM Assessment WHERE projectId = ? AND deletedAt IS NULL",
		func(rows *sql.Rows) (string, error) { var id string; scanErr := rows.Scan(&id); return id, scanErr }, projectID)
	if err != nil {
		return nil, err
	}
	for _, id := range assessmentIDs {
		if err := addRefs(media.RefAssessment, id); err != nil {
			return nil, err
		}
	}

	treatmentIDs, err := studiodb.Query(ctx, q, "SELECT id FROM Treatment WHERE projectId = ? AND deletedAt IS NULL",
		func(rows *sql.Rows) (string, error) { var id string; scanErr := rows.Scan(&id); return id, scanErr }, projectID)
	if err != nil {
		return nil, err
	}
	for _, id := range treatmentIDs {
		if err := addRefs(media.RefTreatment, id); err != nil {
			return nil, err
		}
	}

	reportIDs, err := studiodb.Query(ctx, q, "SELECT id FROM Report WHERE projectId = ? AND deletedAt IS NULL",
		func(rows *sql.Rows) (string, error) { var id string; scanErr := rows.Scan(&id); return id, scanErr }, projectID)
	if err != nil {
		return nil, err
	}
	for _, id := range reportIDs {
		if err := addRefs(media.RefReport, id); err != nil {
			return nil, err
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}
