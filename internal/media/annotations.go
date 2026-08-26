package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	studiodb "studio/internal/db"
	"studio/internal/i18n"
	"studio/internal/settings"
)

// AnnotationRegion is one marked area on a Media item's "pattern layer" — see
// db/migrations/0012_media_annotation_regions.sql and 0014_media_annotation_shape.sql. Coordinates
// are percentages (0-100) of the image's own dimensions, not pixels, so a region stays correct
// regardless of which size the client happens to be viewing the image at. Shape is "rect" (the
// common case, XPct/YPct/WidthPct/HeightPct is the whole shape) or "freehand" (a brush-drawn area:
// those same four columns hold its bounding box, and PathData holds the actual outline as a JSON
// array of {"x","y"} percentage points — see PolygonPoints).
type AnnotationRegion struct {
	ID               string
	MediaID          string
	AnnotationTypeID string
	Shape            string
	XPct             float64
	YPct             float64
	WidthPct         float64
	HeightPct        float64
	PathData         sql.NullString
	CreatedAt        time.Time
}

const annotationRegionColumns = "id, mediaId, annotationTypeId, shape, xPct, yPct, widthPct, heightPct, pathData, createdAt"

func scanAnnotationRegion(rows *sql.Rows) (AnnotationRegion, error) {
	var a AnnotationRegion
	err := rows.Scan(&a.ID, &a.MediaID, &a.AnnotationTypeID, &a.Shape, &a.XPct, &a.YPct, &a.WidthPct, &a.HeightPct, &a.PathData, &a.CreatedAt)
	return a, err
}

// ListRegionsForMedia returns every region on one Media item, oldest first (draw order — later
// regions layer on top, matches the order a conservator most likely added them in).
func ListRegionsForMedia(ctx context.Context, q studiodb.Querier, mediaID string) ([]AnnotationRegion, error) {
	return studiodb.Query(ctx, q,
		"SELECT "+annotationRegionColumns+" FROM MediaAnnotationRegion WHERE mediaId = ? ORDER BY createdAt ASC",
		scanAnnotationRegion, mediaID)
}

// CreateRegion clamps x/y/width/height into [0, 100] and width/height into what's left of the box
// from x/y — a region can never claim to extend past the image, regardless of what a client sends
// (the drag-to-draw JS should never produce one, but this is the actual guarantee, not just a
// client-side nicety).
func CreateRegion(ctx context.Context, q studiodb.Querier, mediaID, annotationTypeID string, x, y, w, h float64) (string, error) {
	x = clampPct(x)
	y = clampPct(y)
	w = clampPct(w)
	if w > 100-x {
		w = 100 - x
	}
	h = clampPct(h)
	if h > 100-y {
		h = 100 - y
	}

	id := studiodb.NewID()
	_, err := studiodb.Execute(ctx, q,
		"INSERT INTO MediaAnnotationRegion (id, mediaId, annotationTypeId, xPct, yPct, widthPct, heightPct) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, mediaID, annotationTypeID, x, y, w, h)
	return id, err
}

// point is one coordinate of a freehand region's outline — percentages of the image's own
// dimensions, same convention as AnnotationRegion's XPct/YPct.
type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// CreateFreehandRegion stores a brush-drawn area: pointsJSON is a JSON array of {"x","y"}
// percentage points as posted by static/js/lightbox.js's freehand tool. Every point is
// clamped into [0,100] (same guarantee CreateRegion makes for rectangles), and the bounding box
// is computed and stored in the same XPct/YPct/WidthPct/HeightPct columns rectangles use, so a
// freehand region is still sortable/queryable like one — PathData carries the actual outline for
// rendering as an SVG <polygon> (see PolygonPoints).
func CreateFreehandRegion(ctx context.Context, q studiodb.Querier, mediaID, annotationTypeID, pointsJSON string) (string, error) {
	var raw []point
	if err := json.Unmarshal([]byte(pointsJSON), &raw); err != nil {
		return "", fmt.Errorf("invalid points: %w", err)
	}
	if len(raw) < 3 {
		return "", fmt.Errorf("a freehand region needs at least 3 points")
	}

	minX, minY := 100.0, 100.0
	maxX, maxY := 0.0, 0.0
	clamped := make([]point, len(raw))
	for i, p := range raw {
		x, y := clampPct(p.X), clampPct(p.Y)
		clamped[i] = point{X: x, Y: y}
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	pathBytes, err := json.Marshal(clamped)
	if err != nil {
		return "", err
	}

	id := studiodb.NewID()
	_, err = studiodb.Execute(ctx, q,
		"INSERT INTO MediaAnnotationRegion (id, mediaId, annotationTypeId, shape, xPct, yPct, widthPct, heightPct, pathData) VALUES (?, ?, ?, 'freehand', ?, ?, ?, ?, ?)",
		id, mediaID, annotationTypeID, minX, minY, maxX-minX, maxY-minY, string(pathBytes))
	return id, err
}

// PolygonPoints turns a freehand region's stored PathData JSON into an SVG
// <polygon points="x,y x,y ...">-ready string. Empty/invalid data (a corrupt row) renders an empty
// polygon rather than erroring the whole page.
func PolygonPoints(pathData string) string {
	var pts []point
	if err := json.Unmarshal([]byte(pathData), &pts); err != nil {
		return ""
	}
	parts := make([]string, len(pts))
	for i, p := range pts {
		parts[i] = strconv.FormatFloat(p.X, 'f', 3, 64) + "," + strconv.FormatFloat(p.Y, 'f', 3, 64)
	}
	return strings.Join(parts, " ")
}

func clampPct(n float64) float64 {
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
}

func DeleteRegion(ctx context.Context, q studiodb.Querier, regionID string) error {
	_, err := studiodb.Execute(ctx, q, "DELETE FROM MediaAnnotationRegion WHERE id = ?", regionID)
	return err
}

// AnnotationTypeOption is one "annotation_type" classifier resolved for the pattern-layer UI:
// locale-aware label plus the hatch direction + color its Classifier.Data JSON carries. This is
// the one place in the app that reads a classifier's Data column server-side (everywhere else
// it's opaque passthrough, per Classifier.Data's own doc comment) — the pattern layer needs the
// hatch/color to actually draw the SVG <pattern> defs and swatches.
type AnnotationTypeOption struct {
	ID    string
	Code  string
	Label string
	Hatch string
	Color string
}

type annotationTypeData struct {
	Hatch string `json:"hatch"`
	Color string `json:"color"`
}

// BuildAnnotationTypeOptions decodes each classifier's data JSON and resolves its locale-aware
// label. A missing/invalid data JSON falls back to a diagonal red hatch rather than failing the
// page — this is a display detail, not something worth a 500 over.
func BuildAnnotationTypeOptions(classifiers []settings.Classifier, locale i18n.Locale) []AnnotationTypeOption {
	out := make([]AnnotationTypeOption, 0, len(classifiers))
	for _, c := range classifiers {
		d := annotationTypeData{Hatch: "hatch-diagonal", Color: "#dc2626"}
		if c.Data.Valid {
			var parsed annotationTypeData
			if err := json.Unmarshal([]byte(c.Data.String), &parsed); err == nil {
				if parsed.Hatch != "" {
					d.Hatch = parsed.Hatch
				}
				if parsed.Color != "" {
					d.Color = parsed.Color
				}
			}
		}
		out = append(out, AnnotationTypeOption{
			ID: c.ID, Code: c.Code, Label: settings.ClassifierLabel(c, locale), Hatch: d.Hatch, Color: d.Color,
		})
	}
	return out
}

// OptionByID looks up one type's option by ID — a region's AnnotationTypeID, resolved for
// rendering its <rect fill="url(#pattern-...)">. Returns the zero value (blank label, empty
// hatch/color) if the type was deleted out from under an existing region; callers skip rendering
// a pattern def for a blank Hatch rather than erroring.
func OptionByID(opts []AnnotationTypeOption, id string) AnnotationTypeOption {
	for _, o := range opts {
		if o.ID == id {
			return o
		}
	}
	return AnnotationTypeOption{}
}

// UsedTypeOptions is the legend's row set: every distinct annotation type actually present in
// regions, in the same order as opts (not region order) so the legend stays stable as regions are
// added/removed.
func UsedTypeOptions(regions []AnnotationRegion, opts []AnnotationTypeOption) []AnnotationTypeOption {
	used := make(map[string]bool, len(regions))
	for _, r := range regions {
		used[r.AnnotationTypeID] = true
	}
	out := make([]AnnotationTypeOption, 0, len(opts))
	for _, o := range opts {
		if used[o.ID] {
			out = append(out, o)
		}
	}
	return out
}

// CountRegionsForProject counts MediaAnnotationRegion rows ("damage mappings") across every
// Media item reachable from a Project — directly uploaded to the Project, or attached to one of
// its Assessments/Treatments/Reports. Used by the Project detail page's summary card; a plain
// count rather than a full list since the Media album already has its own project-grouped browse
// view (see internal/media/views.templ's AlbumPage).
func CountRegionsForProject(ctx context.Context, q studiodb.Querier, projectID string) (int, error) {
	n, err := studiodb.QueryOne(ctx, q, `
		SELECT COUNT(*) AS n FROM MediaAnnotationRegion r
		WHERE r.mediaId IN (
			SELECT mr.mediaId FROM MediaReference mr WHERE mr.referencingType = 'Project' AND mr.referencingId = ?
			UNION
			SELECT mr.mediaId FROM MediaReference mr JOIN Assessment a ON a.id = mr.referencingId WHERE mr.referencingType = 'Assessment' AND a.projectId = ?
			UNION
			SELECT mr.mediaId FROM MediaReference mr JOIN Treatment t ON t.id = mr.referencingId WHERE mr.referencingType = 'Treatment' AND t.projectId = ?
			UNION
			SELECT mr.mediaId FROM MediaReference mr JOIN Report rp ON rp.id = mr.referencingId WHERE mr.referencingType = 'Report' AND rp.projectId = ?
		)`,
		func(rows *sql.Rows) (int, error) { var c int; scanErr := rows.Scan(&c); return c, scanErr },
		projectID, projectID, projectID, projectID)
	if err != nil || n == nil {
		return 0, err
	}
	return *n, nil
}

// hatchAngle maps a hatch-direction code to the rotation applied to a single base
// horizontal-line SVG <pattern> — one pattern shape, four directions, rather than four different
// line geometries.
func HatchAngle(hatch string) int {
	switch hatch {
	case "hatch-vertical":
		return 90
	case "hatch-diagonal":
		return 45
	case "hatch-antidiagonal":
		return 135
	default: // "hatch-horizontal"
		return 0
	}
}
