package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"log/slog"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidbyttow/govips/v2/vips"

	studiodb "studio/internal/db"
	"studio/internal/i18n"
	"studio/internal/settings"
)

// --- Baked-caption typography -----------------------------------------------------------------
//
// The legend and whole-image note under a baked photo are drawn in the image's own pixel space.
// A fixed pixel size therefore looks enormous on an 800px photo and dwindles to nothing on a
// 24-megapixel one. Instead the text is sized for the export that actually matters: the annotated
// image placed full-width on an A4 page. bakedTextPx maps a target point size to the pixel size
// that renders at (about) that point size once the baked canvas is scaled down to
// bakeA4ContentWidthMM. Point sizes are clamped to [bakeTextMinPt, bakeTextMaxPt] so an unusual
// image can't yield unreadably small or oversized text — the 20pt ceiling is the hard requirement.
//
// Image DPI/resolution metadata is deliberately not consulted: the printed width is fixed by the
// page layout, not by whatever density the file claims (phone photos routinely lie), so the pixel
// width of the baked canvas is the only input that determines on-paper size.
const (
	bakeA4ContentWidthMM  = 170.0       // A4 (210mm) less ~20mm margin each side — "typical full-width"
	bakeA4ContentHeightMM = 250.0       // A4 (297mm) less ~24mm top+bottom — the height a full-page image gets
	bakeMMPerPoint        = 25.4 / 72.0 // 1pt in mm

	bakeNotePt        = 11.0 // whole-image note: comfortable body text on A4
	bakeLegendLabelPt = 12.0 // legend labels: the key, a touch larger than the note
	bakeTextMinPt     = 8.0  // never smaller than this on paper, however large the source
	bakeTextMaxPt     = 20.0 // never larger than this on paper, however small the source
)

// bakedTextPx is the pixel font-size for a baked-caption element of point size pt, chosen so it
// renders near pt when the composite is placed as large as it goes on an A4 page. pt is clamped to
// [bakeTextMinPt, bakeTextMaxPt] first.
//
// A landscape/square composite fills the content width (bakeA4ContentWidthMM); a portrait one hits
// the page height first and is shown narrower than that, so its displayed width — and thus the
// px→pt scale — is driven by the height instead. Sizing for that narrower width keeps a tall
// image's caption from blowing past the 20pt ceiling once it's actually on the page.
func bakedTextPx(canvasW, canvasH int, pt float64) float64 {
	pt = math.Max(bakeTextMinPt, math.Min(bakeTextMaxPt, pt))
	displayWidthMM := bakeA4ContentWidthMM
	if canvasH > 0 {
		if heightBound := bakeA4ContentHeightMM * float64(canvasW) / float64(canvasH); heightBound < displayWidthMM {
			displayWidthMM = heightBound
		}
	}
	return pt * bakeMMPerPoint * (float64(canvasW) / displayWidthMM)
}

// legendMarkup renders the "used annotation types" legend as SVG fragments - shared by
// RenderAnnotatedImage's on-demand flatten and BakeAnnotatedVersion's saved composite so both
// present regions the same way: each swatch is its own small tiled-pattern rect (not a flat color
// fill) so the legend actually distinguishes types by pattern as well as color, matching what's
// drawn on the image itself. Lays out two columns once there's more than one type to list (a
// single column would waste half the available width), one otherwise. containerWidth/sidePadding
// describe the horizontal space the legend has to fill; labelPx is the (already A4-scaled) label
// font size every other dimension here is derived from; height is the vertical space it took up,
// for the caller to lay out whatever comes next.
func legendMarkup(usedTypes []AnnotationTypeOption, containerWidth, sidePadding int, labelPx float64) (markup string, height int) {
	if len(usedTypes) == 0 {
		return "", 0
	}
	swatch := int(math.Round(labelPx * 1.5))
	rowH := int(math.Round(labelPx * 2.5))
	colGap := int(math.Round(labelPx * 1.8))
	stroke := math.Max(1, labelPx*0.09)
	tile := math.Max(3, labelPx*0.3) // legend-swatch hatch tile, so the pattern reads at any swatch size

	cols := 1
	if len(usedTypes) > 1 {
		cols = 2
	}
	colWidth := containerWidth - 2*sidePadding
	if cols == 2 {
		colWidth = (colWidth - colGap) / 2
	}
	rows := (len(usedTypes) + cols - 1) / cols
	height = rowH/2 + rows*rowH

	var defs, items strings.Builder
	for i, t := range usedTypes {
		col, row := i%cols, i/cols
		x := sidePadding + col*(colWidth+colGap)
		y := row * rowH
		patID := fmt.Sprintf("legend-pattern-%d", i)
		fmt.Fprintf(&defs,
			`<pattern id="%s" width="%.2f" height="%.2f" patternUnits="userSpaceOnUse" patternTransform="rotate(%d)">`+
				`<line x1="0" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.2f"/></pattern>`,
			patID, tile, tile, HatchAngle(t.Hatch), tile*0.25, tile, tile*0.25, html.EscapeString(t.Color), tile*0.35)
		fmt.Fprintf(&items,
			`<g transform="translate(%d, %d)">`+
				`<rect width="%d" height="%d" rx="%.1f" fill="url(#%s)" fill-opacity="0.8"/>`+
				`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.1f" fill="none" stroke="%s" stroke-width="%.2f"/>`+
				`<text x="%d" y="%.1f" font-family="sans-serif" font-size="%.1f" fill="#111111">%s</text>`+
				`</g>`,
			x, y,
			swatch, swatch, labelPx*0.25, patID,
			stroke, stroke, float64(swatch)-2*stroke, float64(swatch)-2*stroke, labelPx*0.2, html.EscapeString(t.Color), stroke,
			swatch+int(math.Round(labelPx*0.5)), float64(swatch)*0.5+labelPx*0.34, labelPx, html.EscapeString(t.Label))
	}
	return "<defs>" + defs.String() + "</defs>" + items.String(), height
}

// rasterizeSVG renders one of this file's composed annotation SVGs (the base photo embedded as a
// base64 data: URI, plus the vector region/legend/note markup) to a libvips image.
//
// It sets svgload's "unlimited" flag. librsvg/libxml2 otherwise refuse to parse any single XML
// node larger than ~10 MB, and the base64 of a full-resolution conservation photo in the
// <image xlink:href="data:..."> attribute sails right past that — libxml2 stops mid-attribute and
// librsvg reports `XML parse error ... Extra content at the end of the document` (seen in prod as
// a bake 500, "cannot save annotated image", for every real upload while tiny CI/demo images were
// fine). The SVG is built entirely from our own trusted bytes, so the untrusted-SVG
// decompression-bomb guard that flag disables isn't protecting anything here.
func rasterizeSVG(svg string) (*vips.ImageRef, error) {
	params := vips.NewImportParams()
	params.SvgUnlimited.Set(true)
	return vips.LoadImageFromBuffer([]byte(svg), params)
}

// RenderAnnotatedImage flattens a Media image's drawn regions (see annotations.go) and their
// legend into the image itself: builds one standalone SVG (the "web" variant as a data: URI, the
// same region shapes and legend the live editor/viewer render) and rasterizes it to PNG via
// libvips/librsvg. Used by the "Download annotated" button and by report exports
// (internal/export) so a shared image carries its marked damage/loss areas baked in, not just live
// on the page. Returns (nil, "", nil) when the media has no regions — callers should fall back to
// the plain file in that case, there's nothing to flatten.
func (s *Service) RenderAnnotatedImage(ctx context.Context, mediaID string, locale i18n.Locale) ([]byte, string, error) {
	m, err := GetByID(ctx, s.Pool, mediaID)
	if err != nil || m == nil || m.Kind != KindImage {
		return nil, "", err
	}
	regions, err := ListRegionsForMedia(ctx, s.Pool, mediaID)
	if err != nil || len(regions) == 0 {
		return nil, "", err
	}
	classifiers, err := settings.GetClassifiers(ctx, s.Pool, settings.ClassifierAnnotationType)
	if err != nil {
		return nil, "", err
	}
	annotationTypes := BuildAnnotationTypeOptions(classifiers, locale)

	// The "web" variant (not the original) — same file the editor/viewer display, so the
	// percentage-coordinate regions land in exactly the same place a conservator drew them, and a
	// guaranteed-JPEG format librsvg can always decode inside an embedded <image>.
	file, err := s.ReadMediaFile(ctx, mediaID, "web")
	if err != nil || file == nil {
		return nil, "", err
	}
	baseImg, err := vips.NewImageFromBuffer(file.Data)
	if err != nil {
		return nil, "", fmt.Errorf("decoding base image for annotation rasterization: %w", err)
	}
	width, height := baseImg.Width(), baseImg.Height()
	baseImg.Close()

	var shapes bytes.Buffer
	if err := regionsAndDefsFragment(regions, annotationTypes).Render(ctx, &shapes); err != nil {
		return nil, "", err
	}

	usedTypes := UsedTypeOptions(regions, annotationTypes)
	legend, legendHeight := legendMarkup(usedTypes, width, 16, bakedTextPx(width, height, bakeLegendLabelPt))

	dataURI := "data:" + file.MimeType + ";base64," + base64.StdEncoding.EncodeToString(file.Data)
	totalHeight := height + legendHeight

	// The inner <svg>'s viewBox="0 0 100 100" is percent-of-image space (regions are stored that
	// way - see annotations.go) stretched non-uniformly onto the real width x height box so a
	// region's percent coordinates land in the right place regardless of the photo's own aspect
	// ratio. The <image> itself must carry the same preserveAspectRatio="none" (SVG's default for
	// <image> is "meet", which aspect-fits/letterboxes *before* that outer stretch is applied) or
	// the two non-uniform scales compose into a visibly stretched/squashed photo instead of
	// canceling out back to the original proportions.
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d">`+
		`<rect width="%d" height="%d" fill="#ffffff"/>`+
		`<svg x="0" y="0" width="%d" height="%d" viewBox="0 0 100 100" preserveAspectRatio="none">`+
		`<image xlink:href="%s" x="0" y="0" width="100" height="100" preserveAspectRatio="none"/>%s</svg>`+
		`<g transform="translate(0, %d)">%s</g></svg>`,
		width, totalHeight, width, totalHeight, width, height, dataURI, shapes.String(), height, legend)

	rendered, err := rasterizeSVG(svg)
	if err != nil {
		return nil, "", fmt.Errorf("rasterizing annotated image: %w", err)
	}
	defer rendered.Close()

	png, _, err := rendered.ExportPng(vips.NewPngExportParams())
	if err != nil {
		return nil, "", fmt.Errorf("exporting annotated PNG: %w", err)
	}
	return png, "image/png", nil
}

// Layout constants for BakeAnnotatedVersion's composited image: the original photo, a solid black
// divider spanning the image's full width, then the region-type legend, then the whole-image note
// as wrapped text - see the "Annotated versions" design in internal/media/views.templ for the
// corresponding read-only rendering. The type sizes (legend labels, note) are computed per-image
// by bakedTextPx above; the divider gap and word-wrap width below are derived from the note size
// so the caption block keeps proportional breathing room whether the photo is 0.5 or 25 megapixels.
const (
	bakeMaxDimension  = 4000 // cap on the long edge - a real deliverable, not a thumbnail, but still bounded against pathological raw-camera-file memory use
	bakeSidePadding   = 24   // left/right inset for the legend/note *text* only - the divider itself runs edge to edge
	bakeDividerHeight = 1    // a hairline rule, not a thick color bar - see BakeAnnotatedVersion
)

// BakeAnnotatedVersion renders the *current* full set of regions (internal/media/annotations.go)
// on `target` (an annotated-version Media row created by CreateAnnotatedVersion, or re-baked again
// after further edits) into one complete PNG and persists it as that Media's own file - unlike
// RenderAnnotatedImage above (which is a pure on-demand, never-saved flatten, still used by
// "download annotated" on media with regions attached the old way and by internal/export), this is
// the "edit session finished" save: the original image, converted to grayscale (matching what the
// editor showed while drawing - see static/js/media-editor.js's IIIF tileQuality=gray), at its own full
// resolution (capped at bakeMaxDimension) rather than the "web" thumbnail, canvas width kept
// exactly equal to the image's own width so the photo itself is never stretched - only the
// (taller) canvas grows to fit the divider/legend/note beneath it. If `target` already has a
// baked file from an earlier session, that file is preserved on disk first (a timestamped
// "-old-<unix>" sibling key) before being overwritten, never silently discarded.
func (s *Service) BakeAnnotatedVersion(ctx context.Context, target Media, locale i18n.Locale) error {
	if !target.EditedFromID.Valid {
		return fmt.Errorf("bake annotated version: %s is not an annotated version (no editedFromId)", target.ID)
	}
	source, err := GetByID(ctx, s.Pool, target.EditedFromID.String)
	if err != nil {
		return err
	}
	if source == nil {
		return fmt.Errorf("bake annotated version: source media %s not found", target.EditedFromID.String)
	}

	regions, err := ListRegionsForMedia(ctx, s.Pool, target.ID)
	if err != nil {
		return err
	}
	classifiers, err := settings.GetClassifiers(ctx, s.Pool, settings.ClassifierAnnotationType)
	if err != nil {
		return err
	}
	annotationTypes := BuildAnnotationTypeOptions(classifiers, locale)
	slog.DebugContext(ctx, "bake: inputs loaded", "media_id", target.ID, "source_id", source.ID,
		"source_storage_key", source.StorageKey, "regions", len(regions), "annotation_types", len(annotationTypes),
		"has_note", target.Description.String != "", "locale", string(locale), "category", "media", "event", "bake_step")

	original, err := s.Storage.Get(source.StorageKey)
	if err != nil {
		return fmt.Errorf("reading source original: %w", err)
	}
	slog.DebugContext(ctx, "bake: read source original", "media_id", target.ID, "bytes", len(original),
		"source_mime", source.MimeType, "category", "media", "event", "bake_step")
	img, err := vips.NewImageFromBuffer(original)
	if err != nil {
		return fmt.Errorf("decoding source original (%s, %d bytes): %w", source.MimeType, len(original), err)
	}
	defer img.Close()
	if err := img.AutoRotate(); err != nil {
		return fmt.Errorf("auto-rotating: %w", err)
	}
	if err := img.ToColorSpace(vips.InterpretationBW); err != nil {
		return fmt.Errorf("converting to grayscale: %w", err)
	}
	if w, h := img.Width(), img.Height(); w > bakeMaxDimension || h > bakeMaxDimension {
		scale := math.Min(float64(bakeMaxDimension)/float64(w), float64(bakeMaxDimension)/float64(h))
		if err := img.Resize(scale, vips.KernelAuto); err != nil {
			return fmt.Errorf("resizing to bake bounds: %w", err)
		}
	}
	imgW, imgH := img.Width(), img.Height()
	grayPng, _, err := img.ExportPng(vips.NewPngExportParams())
	if err != nil {
		return fmt.Errorf("exporting grayscale base: %w", err)
	}
	slog.DebugContext(ctx, "bake: grayscale base prepared", "media_id", target.ID, "img_w", imgW, "img_h", imgH,
		"gray_png_bytes", len(grayPng), "category", "media", "event", "bake_step")

	var shapes bytes.Buffer
	if err := regionsAndDefsFragment(regions, annotationTypes).Render(ctx, &shapes); err != nil {
		return err
	}

	// Type sizes scaled for this image's pixel width (see bakedTextPx); the divider gap and the
	// note's line spacing / word-wrap width are all derived from the note size so the whole caption
	// block scales together.
	notePx := bakedTextPx(imgW, imgH, bakeNotePt)
	gap := int(math.Round(notePx * 0.4)) // vertical breathing room around the divider/legend/note
	noteLineGap := notePx * 0.42
	noteCharWidth := notePx * 0.5 // ~average glyph advance for a sans-serif at notePx

	usedTypes := UsedTypeOptions(regions, annotationTypes)
	legend, legendHeight := legendMarkup(usedTypes, imgW, bakeSidePadding, bakedTextPx(imgW, imgH, bakeLegendLabelPt))
	// `gap` after the legend mirrors the gap above the divider - symmetric breathing room around
	// the whole legend block, not just above it.
	legendBottomPad := 0
	if legendHeight > 0 {
		legendBottomPad = gap
	}

	noteLines := wrapText(target.Description.String, int(float64(imgW-2*bakeSidePadding)/noteCharWidth))
	noteHeight := 0
	var note bytes.Buffer
	if len(noteLines) > 0 {
		noteHeight = int(math.Round(float64(len(noteLines))*(notePx+noteLineGap) + noteLineGap))
		for i, line := range noteLines {
			baseline := float64(i+1) * (notePx + noteLineGap)
			fmt.Fprintf(&note, `<text x="%d" y="%.1f" font-family="sans-serif" font-size="%.1f" fill="#333333">%s</text>`,
				bakeSidePadding, baseline, notePx, html.EscapeString(line))
		}
	}

	dividerY := imgH + gap
	belowDividerY := dividerY + bakeDividerHeight + 2*gap
	noteY := belowDividerY + legendHeight + legendBottomPad
	totalHeight := noteY + noteHeight
	if legendHeight == 0 && noteHeight == 0 {
		// Nothing to caption - just the (grayscale) image itself, no divider floating with
		// nothing underneath it.
		totalHeight = imgH
	}

	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(grayPng)
	var divider string
	if legendHeight > 0 || noteHeight > 0 {
		// Full width, edge to edge - not inset by bakeSidePadding like the legend/note text above
		// and below it.
		divider = fmt.Sprintf(`<rect x="0" y="%d" width="%d" height="%d" fill="#000000"/>`,
			dividerY, imgW, bakeDividerHeight)
	}

	// <image preserveAspectRatio="none"> - see the matching comment in RenderAnnotatedImage above;
	// without it this composite came out visibly stretched/squashed instead of matching the
	// original photo's proportions, with only the canvas height growing for the divider/legend/note.
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d">`+
		`<rect width="%d" height="%d" fill="#ffffff"/>`+
		`<svg x="0" y="0" width="%d" height="%d" viewBox="0 0 100 100" preserveAspectRatio="none">`+
		`<image xlink:href="%s" x="0" y="0" width="100" height="100" preserveAspectRatio="none"/>%s</svg>`+
		`%s<g transform="translate(0, %d)">%s</g><g transform="translate(0, %d)">%s</g></svg>`,
		imgW, totalHeight, imgW, totalHeight, imgW, imgH, dataURI, shapes.String(),
		divider, belowDividerY, legend, noteY, note.String())

	slog.DebugContext(ctx, "bake: SVG composed", "media_id", target.ID, "svg_bytes", len(svg),
		"img_w", imgW, "img_h", imgH, "total_height", totalHeight, "legend_height", legendHeight,
		"note_lines", len(noteLines), "used_types", len(usedTypes), "shapes_bytes", shapes.Len(),
		"category", "media", "event", "bake_step")

	rendered, err := rasterizeSVG(svg)
	if err != nil {
		// The embedded base64 image is stripped from svg_markup - it's huge and never the cause;
		// what's left is the legend/note/region markup that actually varies and can be malformed
		// or reference a font librsvg can't resolve.
		slog.ErrorContext(ctx, "bake: rasterizing composed SVG failed", "err", err, "media_id", target.ID,
			"svg_bytes", len(svg), "svg_markup", svgForLog(svg), "category", "media", "event", "bake_rasterize_failed")
		return fmt.Errorf("rasterizing baked image: %w", err)
	}
	defer rendered.Close()
	baked, _, err := rendered.ExportPng(vips.NewPngExportParams())
	if err != nil {
		return fmt.Errorf("exporting baked PNG: %w", err)
	}
	slog.DebugContext(ctx, "bake: composite rasterized", "media_id", target.ID, "baked_png_bytes", len(baked),
		"category", "media", "event", "bake_step")

	// Preserve whatever was there from an earlier bake, rather than silently discarding it.
	if existing, err := s.Storage.Get(target.StorageKey); err == nil && len(existing) > 0 {
		backupKey := fmt.Sprintf("%s-old-%d", strings.TrimSuffix(target.StorageKey, filepath.Ext(target.StorageKey)), time.Now().UnixNano()) + filepath.Ext(target.StorageKey)
		if err := s.Storage.Put(backupKey, existing); err != nil {
			slog.ErrorContext(ctx, "backing up previous bake", "err", err, "media_id", target.ID, "category", "media", "event", "bake_backup_failed")
		}
	}
	if err := s.Storage.Put(target.StorageKey, baked); err != nil {
		return fmt.Errorf("storing baked image: %w", err)
	}
	s.storeImageVariants(ctx, target.ID, baked)

	sum := sha256.Sum256(baked)
	if _, err := studiodb.Execute(ctx, s.Pool,
		"UPDATE Media SET mimeType = 'image/png', sizeBytes = ?, width = ?, height = ?, checksum = ? WHERE id = ?",
		len(baked), imgW, totalHeight, hex.EncodeToString(sum[:]), target.ID); err != nil {
		return fmt.Errorf("updating Media row after bake: %w", err)
	}
	slog.InfoContext(ctx, "bake: annotated version saved", "media_id", target.ID, "regions", len(regions),
		"img_w", imgW, "total_height", totalHeight, "baked_png_bytes", len(baked), "category", "media", "event", "bake_saved")
	return nil
}

// svgForLog replaces the (large, base64) embedded image data URI in a composed bake SVG with a
// short placeholder so the rest of the markup - the legend/note/region shapes that actually vary -
// can be logged when a rasterize fails without dumping a megabyte of base64.
func svgForLog(svg string) string {
	const marker = `xlink:href="data:`
	start := strings.Index(svg, marker)
	if start < 0 {
		return svg
	}
	rest := start + len(marker)
	end := strings.IndexByte(svg[rest:], '"')
	if end < 0 {
		return svg
	}
	return svg[:rest] + "…omitted…" + svg[rest+end:]
}

// wrapText is a plain word-wrap for the note baked under the legend - no font metrics available
// server-side, so the caller estimates each line's capacity from a rough average glyph advance
// (~0.5em) rather than measuring exactly. Good enough for a short caption; not meant for long prose.
func wrapText(text string, maxCharsPerLine int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxCharsPerLine < 10 {
		maxCharsPerLine = 10
	}
	words := strings.Fields(text)
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() > 0 && cur.Len()+1+len(w) > maxCharsPerLine {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}
