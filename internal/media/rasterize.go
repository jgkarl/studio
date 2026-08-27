package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidbyttow/govips/v2/vips"

	studiodb "studio/internal/db"
	"studio/internal/i18n"
	"studio/internal/settings"
)

const (
	legendRowHeight  = 34 // taller than a plain 16px color swatch to fit legendSwatchSize's pattern preview
	legendSwatchSize = 22 // was a flat 16x16 solid-color square - big enough now to actually show the hatch, not just the color
	legendColGap     = 24
)

// legendMarkup renders the "used annotation types" legend as SVG fragments - shared by
// RenderAnnotatedImage's on-demand flatten and BakeAnnotatedVersion's saved composite so both
// present regions the same way: each swatch is its own small tiled-pattern rect (not a flat color
// fill) so the legend actually distinguishes types by pattern as well as color, matching what's
// drawn on the image itself. Lays out two columns once there's more than one type to list (a
// single column would waste half the available width), one otherwise. containerWidth/sidePadding
// describe the horizontal space the legend has to fill; height is the vertical space it took up,
// for the caller to lay out whatever comes next.
func legendMarkup(usedTypes []AnnotationTypeOption, containerWidth, sidePadding int) (markup string, height int) {
	if len(usedTypes) == 0 {
		return "", 0
	}
	cols := 1
	if len(usedTypes) > 1 {
		cols = 2
	}
	colWidth := containerWidth - 2*sidePadding
	if cols == 2 {
		colWidth = (colWidth - legendColGap) / 2
	}
	rows := (len(usedTypes) + cols - 1) / cols
	height = legendRowHeight/2 + rows*legendRowHeight

	var defs, items strings.Builder
	for i, t := range usedTypes {
		col, row := i%cols, i/cols
		x := sidePadding + col*(colWidth+legendColGap)
		y := row * legendRowHeight
		patID := fmt.Sprintf("legend-pattern-%d", i)
		fmt.Fprintf(&defs,
			`<pattern id="%s" width="4" height="4" patternUnits="userSpaceOnUse" patternTransform="rotate(%d)">`+
				`<line x1="0" y1="1" x2="4" y2="1" stroke="%s" stroke-width="1.4"/></pattern>`,
			patID, HatchAngle(t.Hatch), html.EscapeString(t.Color))
		fmt.Fprintf(&items,
			`<g transform="translate(%d, %d)">`+
				`<rect width="%d" height="%d" rx="5" fill="url(#%s)" fill-opacity="0.8"/>`+
				`<rect x="1" y="1" width="%d" height="%d" rx="4" fill="none" stroke="%s" stroke-width="1.5"/>`+
				`<text x="%d" y="%d" font-family="sans-serif" font-size="15" fill="#111111">%s</text>`+
				`</g>`,
			x, y,
			legendSwatchSize, legendSwatchSize, patID,
			legendSwatchSize-2, legendSwatchSize-2, html.EscapeString(t.Color),
			legendSwatchSize+8, legendSwatchSize-6, html.EscapeString(t.Label))
	}
	return "<defs>" + defs.String() + "</defs>" + items.String(), height
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

	// The "web" variant (not the original) — same file the lightbox/viewer display, so the
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
	legend, legendHeight := legendMarkup(usedTypes, width, 16)

	dataURI := "data:" + file.MimeType + ";base64," + base64.StdEncoding.EncodeToString(file.Data)
	totalHeight := height + legendHeight

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d">`+
		`<rect width="%d" height="%d" fill="#ffffff"/>`+
		`<svg x="0" y="0" width="%d" height="%d" viewBox="0 0 100 100" preserveAspectRatio="none">`+
		`<image xlink:href="%s" x="0" y="0" width="100" height="100"/>%s</svg>`+
		`<g transform="translate(0, %d)">%s</g></svg>`,
		width, totalHeight, width, totalHeight, width, height, dataURI, shapes.String(), height, legend)

	rendered, err := vips.NewImageFromBuffer([]byte(svg))
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
// divider (inset from both edges by sidePadding, with its own vertical breathing room above/below),
// then the region-type legend, then the whole-image note as wrapped text - see the "Annotated
// versions" design in internal/media/views.templ for the corresponding read-only rendering.
const (
	bakeMaxDimension  = 4000 // cap on the long edge - a real deliverable, not a thumbnail, but still bounded against pathological raw-camera-file memory use
	bakeSidePadding   = 24
	bakeDividerGap    = 16 // breathing room both above the divider (before it) and below the legend (after it) - the same gap on both sides of that section
	bakeDividerHeight = 1  // a hairline rule, not a thick color bar - see BakeAnnotatedVersion
	bakeNoteFontSize  = 15
	bakeNoteLineGap   = 6
	bakeNoteCharWidth = 8 // rough average glyph advance at bakeNoteFontSize, for word-wrap width estimation
)

// BakeAnnotatedVersion renders the *current* full set of regions (internal/media/annotations.go)
// on `target` (an annotated-version Media row created by CreateAnnotatedVersion, or re-baked again
// after further edits) into one complete PNG and persists it as that Media's own file - unlike
// RenderAnnotatedImage above (which is a pure on-demand, never-saved flatten, still used by
// "download annotated" on media with regions attached the old way and by internal/export), this is
// the "edit session finished" save: the original image, converted to grayscale (matching what the
// editor showed while drawing - see static/js/lightbox.js's IIIF tileQuality=gray), at its own full
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

	original, err := s.Storage.Get(source.StorageKey)
	if err != nil {
		return fmt.Errorf("reading source original: %w", err)
	}
	img, err := vips.NewImageFromBuffer(original)
	if err != nil {
		return fmt.Errorf("decoding source original: %w", err)
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

	var shapes bytes.Buffer
	if err := regionsAndDefsFragment(regions, annotationTypes).Render(ctx, &shapes); err != nil {
		return err
	}

	usedTypes := UsedTypeOptions(regions, annotationTypes)
	legend, legendHeight := legendMarkup(usedTypes, imgW, bakeSidePadding)
	// The same gap that separates the image from the divider ("top of legend") also separates the
	// legend from whatever follows ("bottom of legend") - symmetric breathing room around the
	// whole legend block, not just above it.
	legendBottomPad := 0
	if legendHeight > 0 {
		legendBottomPad = bakeDividerGap
	}

	noteLines := wrapText(target.Description.String, (imgW-2*bakeSidePadding)/bakeNoteCharWidth)
	noteHeight := 0
	var note bytes.Buffer
	if len(noteLines) > 0 {
		noteHeight = len(noteLines)*(bakeNoteFontSize+bakeNoteLineGap) + bakeNoteLineGap
		for i, line := range noteLines {
			fmt.Fprintf(&note, `<text x="%d" y="%d" font-family="sans-serif" font-size="%d" fill="#333333">%s</text>`,
				bakeSidePadding, bakeNoteLineGap+(i+1)*(bakeNoteFontSize+bakeNoteLineGap)-bakeNoteLineGap, bakeNoteFontSize, html.EscapeString(line))
		}
	}

	dividerY := imgH + bakeDividerGap
	belowDividerY := dividerY + bakeDividerHeight + bakeDividerGap
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
		divider = fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="#000000"/>`,
			bakeSidePadding, dividerY, imgW-2*bakeSidePadding, bakeDividerHeight)
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d">`+
		`<rect width="%d" height="%d" fill="#ffffff"/>`+
		`<svg x="0" y="0" width="%d" height="%d" viewBox="0 0 100 100" preserveAspectRatio="none">`+
		`<image xlink:href="%s" x="0" y="0" width="100" height="100"/>%s</svg>`+
		`%s<g transform="translate(0, %d)">%s</g><g transform="translate(0, %d)">%s</g></svg>`,
		imgW, totalHeight, imgW, totalHeight, imgW, imgH, dataURI, shapes.String(),
		divider, belowDividerY, legend, noteY, note.String())

	rendered, err := vips.NewImageFromBuffer([]byte(svg))
	if err != nil {
		return fmt.Errorf("rasterizing baked image: %w", err)
	}
	defer rendered.Close()
	baked, _, err := rendered.ExportPng(vips.NewPngExportParams())
	if err != nil {
		return fmt.Errorf("exporting baked PNG: %w", err)
	}

	// Preserve whatever was there from an earlier bake, rather than silently discarding it.
	if existing, err := s.Storage.Get(target.StorageKey); err == nil && len(existing) > 0 {
		backupKey := fmt.Sprintf("%s-old-%d", strings.TrimSuffix(target.StorageKey, filepath.Ext(target.StorageKey)), time.Now().UnixNano()) + filepath.Ext(target.StorageKey)
		if err := s.Storage.Put(backupKey, existing); err != nil {
			log.Printf("media: backing up previous bake of %s: %v", target.ID, err)
		}
	}
	if err := s.Storage.Put(target.StorageKey, baked); err != nil {
		return fmt.Errorf("storing baked image: %w", err)
	}
	s.storeImageVariants(target.ID, baked)

	sum := sha256.Sum256(baked)
	_, err = studiodb.Execute(ctx, s.Pool,
		"UPDATE Media SET mimeType = 'image/png', sizeBytes = ?, width = ?, height = ?, checksum = ? WHERE id = ?",
		len(baked), imgW, totalHeight, hex.EncodeToString(sum[:]), target.ID)
	return err
}

// wrapText is a plain word-wrap for the note baked under the legend - no font metrics available
// server-side, so it estimates each line's capacity from bakeNoteCharWidth's rough average glyph
// advance rather than measuring exactly. Good enough for a short caption; not meant for long prose.
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
