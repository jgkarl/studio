package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html"

	"github.com/davidbyttow/govips/v2/vips"

	"stuudio/internal/i18n"
	"stuudio/internal/settings"
)

const legendRowHeight = 26

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
	legendHeight := 0
	var legend bytes.Buffer
	if len(usedTypes) > 0 {
		legendHeight = legendRowHeight/2 + len(usedTypes)*legendRowHeight
		for i, t := range usedTypes {
			fmt.Fprintf(&legend, `<g transform="translate(16, %d)"><rect width="16" height="16" fill="%s"/><text x="24" y="13" font-family="sans-serif" font-size="15" fill="#111111">%s</text></g>`,
				i*legendRowHeight, html.EscapeString(t.Color), html.EscapeString(t.Label))
		}
	}

	dataURI := "data:" + file.MimeType + ";base64," + base64.StdEncoding.EncodeToString(file.Data)
	totalHeight := height + legendHeight

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d">`+
		`<rect width="%d" height="%d" fill="#ffffff"/>`+
		`<svg x="0" y="0" width="%d" height="%d" viewBox="0 0 100 100" preserveAspectRatio="none">`+
		`<image xlink:href="%s" x="0" y="0" width="100" height="100"/>%s</svg>`+
		`<g transform="translate(0, %d)">%s</g></svg>`,
		width, totalHeight, width, totalHeight, width, height, dataURI, shapes.String(), height, legend.String())

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
