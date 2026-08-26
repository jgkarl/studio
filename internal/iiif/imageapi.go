// Package iiif is a practical implementation of the IIIF Image API
// (https://iiif.io/api/image/3/) — region, size, rotation, quality, and format transforms applied
// live via govips against the stored original file (internal/media). Level 2-*equivalent* feature
// set (documented as such in BuildInfoJSON) — this is a from-scratch implementation of the spec's
// transform semantics, not run through the official IIIF validator.
package iiif

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/davidbyttow/govips/v2/vips"

	"studio/internal/media"
)

// Error carries an HTTP status alongside the message — handlers.go uses it to pick the right
// response code (400 for a malformed parameter, 404 for a missing/non-image media row, 500 for a
// transform failure).
type Error struct {
	Message string
	Status  int
}

func (e *Error) Error() string { return e.Message }

func newError(status int, format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...), Status: status}
}

// Params is one IIIF Image API request's four path segments.
type Params struct {
	Region   string
	Size     string
	Rotation string
	Quality  string
	Format   string
}

type pixelRegion struct {
	Left, Top, Width, Height int
}

func parseRegion(region string, imgW, imgH int) (*pixelRegion, error) {
	if region == "full" {
		return nil, nil
	}
	if region == "square" {
		side := imgW
		if imgH < side {
			side = imgH
		}
		return &pixelRegion{Left: (imgW - side) / 2, Top: (imgH - side) / 2, Width: side, Height: side}, nil
	}

	isPct := strings.HasPrefix(region, "pct:")
	raw := region
	if isPct {
		raw = region[4:]
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return nil, newError(400, "invalid region: %s", region)
	}
	nums := make([]float64, 4)
	for i, p := range parts {
		n, err := strconv.ParseFloat(p, 64)
		if err != nil || n < 0 {
			return nil, newError(400, "invalid region: %s", region)
		}
		nums[i] = n
	}
	x, y, w, h := nums[0], nums[1], nums[2], nums[3]
	if isPct {
		x = x / 100 * float64(imgW)
		y = y / 100 * float64(imgH)
		w = w / 100 * float64(imgW)
		h = h / 100 * float64(imgH)
	}
	if x >= float64(imgW) || y >= float64(imgH) || w <= 0 || h <= 0 {
		return nil, newError(400, "region out of bounds: %s", region)
	}

	// Clamp to image bounds — the spec requires this rather than erroring on a region that
	// partially overlaps the image.
	x = math.Max(0, math.Min(x, float64(imgW-1)))
	y = math.Max(0, math.Min(y, float64(imgH-1)))
	w = math.Max(1, math.Min(w, float64(imgW)-x))
	h = math.Max(1, math.Min(h, float64(imgH)-y))

	return &pixelRegion{
		Left: int(math.Round(x)), Top: int(math.Round(y)),
		Width: int(math.Round(w)), Height: int(math.Round(h)),
	}, nil
}

// parsedSize: 0 means that axis is unconstrained (govips scales it proportionally on its own).
// Fit only means something once both Width and Height are pinned down — "" (not "fill") when only
// one axis is constrained, same reasoning as the TS version's ParsedSize.fit comment.
type parsedSize struct {
	Width, Height int
	Fit           string // "fill" | "inside" | ""
}

func parseSize(size string, regionW, regionH int) (parsedSize, error) {
	if size == "full" || size == "max" {
		return parsedSize{}, nil
	}

	if strings.HasPrefix(size, "pct:") {
		n, err := strconv.ParseFloat(size[4:], 64)
		if err != nil || n <= 0 {
			return parsedSize{}, newError(400, "invalid size: %s", size)
		}
		return parsedSize{
			Width:  int(math.Max(1, math.Round(float64(regionW)*n/100))),
			Height: int(math.Max(1, math.Round(float64(regionH)*n/100))),
			Fit:    "fill",
		}, nil
	}

	// "!w,h" — best-fit within the box, preserving aspect ratio. "w,h" — exact dimensions
	// (distorts aspect ratio if needed). "w," / ",h" — one dimension, the other auto.
	best := strings.HasPrefix(size, "!")
	spec := size
	if best {
		spec = size[1:]
	}
	parts := strings.SplitN(spec, ",", 2)
	if len(parts) != 2 {
		return parsedSize{}, newError(400, "invalid size: %s", size)
	}
	var width, height int
	if parts[0] != "" {
		n, err := strconv.Atoi(parts[0])
		if err != nil || n <= 0 {
			return parsedSize{}, newError(400, "invalid size: %s", size)
		}
		width = n
	}
	if parts[1] != "" {
		n, err := strconv.Atoi(parts[1])
		if err != nil || n <= 0 {
			return parsedSize{}, newError(400, "invalid size: %s", size)
		}
		height = n
	}
	if width == 0 && height == 0 {
		return parsedSize{}, newError(400, "invalid size: %s", size)
	}

	ps := parsedSize{Width: width, Height: height}
	if width > 0 && height > 0 {
		if best {
			ps.Fit = "inside"
		} else {
			ps.Fit = "fill"
		}
	}
	return ps, nil
}

func resizeTo(img *vips.ImageRef, size parsedSize, regionW, regionH int) error {
	switch {
	case size.Width > 0 && size.Height > 0:
		wScale := float64(size.Width) / float64(regionW)
		hScale := float64(size.Height) / float64(regionH)
		if size.Fit == "inside" {
			// Best-fit within the box: same scale on both axes, the smaller of the two so
			// neither dimension overflows.
			return img.Resize(math.Min(wScale, hScale), vips.KernelAuto)
		}
		return img.ResizeWithVScale(wScale, hScale, vips.KernelAuto)
	case size.Width > 0:
		return img.Resize(float64(size.Width)/float64(regionW), vips.KernelAuto)
	case size.Height > 0:
		return img.Resize(float64(size.Height)/float64(regionH), vips.KernelAuto)
	}
	return nil
}

type parsedRotation struct {
	Degrees float64
	Mirror  bool
}

func parseRotation(rotation string) (parsedRotation, error) {
	mirror := strings.HasPrefix(rotation, "!")
	raw := rotation
	if mirror {
		raw = rotation[1:]
	}
	degrees, err := strconv.ParseFloat(raw, 64)
	if err != nil || degrees < 0 || degrees > 360 {
		return parsedRotation{}, newError(400, "invalid rotation: %s", rotation)
	}
	return parsedRotation{Degrees: degrees, Mirror: mirror}, nil
}

var qualities = map[string]bool{"default": true, "color": true, "gray": true, "bitonal": true}
var formats = map[string]string{"jpg": "image/jpeg", "png": "image/png", "webp": "image/webp"}

// TransformImage runs one IIIF Image API request end to end: decode the stored original, auto-
// orient per EXIF (region/size math below assumes display orientation), region -> size ->
// rotation -> quality, then export.
func TransformImage(ctx context.Context, mediaSvc *media.Service, mediaID string, p Params) ([]byte, string, error) {
	m, err := media.GetByID(ctx, mediaSvc.Pool, mediaID)
	if err != nil {
		return nil, "", err
	}
	if m == nil || m.Kind != media.KindImage {
		return nil, "", newError(404, "not an image")
	}

	file, err := mediaSvc.ReadMediaFile(ctx, mediaID, "original")
	if err != nil {
		return nil, "", err
	}
	if file == nil {
		return nil, "", newError(404, "original file not found")
	}

	img, err := vips.NewImageFromBuffer(file.Data)
	if err != nil {
		return nil, "", newError(500, "decoding image: %v", err)
	}
	defer img.Close()

	if err := img.AutoRotate(); err != nil {
		return nil, "", newError(500, "auto-rotating: %v", err)
	}
	// Normalize to a standard 3-band colorspace regardless of the source (already-grayscale
	// scans, indexed/palette PNGs, CMYK, ...) — Similarity's backgroundColor below is always a
	// 4-component RGBA value, and libvips errors ("linear: vector must have 1 or 2 elements") if
	// it doesn't match the image's actual band count, e.g. a single-band grayscale source hitting
	// an arbitrary-degree rotation. quality=gray/bitonal still converts to true grayscale at the
	// end of the pipeline regardless of this internal working colorspace.
	if err := img.ToColorSpace(vips.InterpretationSRGB); err != nil {
		return nil, "", newError(500, "normalizing colorspace: %v", err)
	}

	imgW, imgH := img.Width(), img.Height()
	if imgW == 0 || imgH == 0 {
		return nil, "", newError(500, "unknown image dimensions")
	}

	region, err := parseRegion(p.Region, imgW, imgH)
	if err != nil {
		return nil, "", err
	}
	regionW, regionH := imgW, imgH
	if region != nil {
		if err := img.ExtractArea(region.Left, region.Top, region.Width, region.Height); err != nil {
			return nil, "", newError(500, "extracting region: %v", err)
		}
		regionW, regionH = region.Width, region.Height
	}

	size, err := parseSize(p.Size, regionW, regionH)
	if err != nil {
		return nil, "", err
	}
	if size.Width > 0 || size.Height > 0 {
		if err := resizeTo(img, size, regionW, regionH); err != nil {
			return nil, "", newError(500, "resizing: %v", err)
		}
	}

	rotation, err := parseRotation(p.Rotation)
	if err != nil {
		return nil, "", err
	}
	if rotation.Mirror {
		if err := img.Flip(vips.DirectionHorizontal); err != nil {
			return nil, "", newError(500, "mirroring: %v", err)
		}
	}
	if rotation.Degrees != 0 {
		// White background for the corners a non-90°-multiple rotation exposes — matches common
		// reference IIIF server behavior.
		bg := &vips.ColorRGBA{R: 255, G: 255, B: 255, A: 255}
		if err := img.Similarity(1, rotation.Degrees, bg, 0, 0, 0, 0); err != nil {
			return nil, "", newError(500, "rotating: %v", err)
		}
	}

	if !qualities[p.Quality] {
		return nil, "", newError(400, "invalid quality: %s", p.Quality)
	}
	if p.Quality == "gray" || p.Quality == "bitonal" {
		if err := img.ToColorSpace(vips.InterpretationBW); err != nil {
			return nil, "", newError(500, "converting to grayscale: %v", err)
		}
	}
	if p.Quality == "bitonal" {
		// Best-effort hard threshold at the midpoint — govips has no dedicated bitonal/threshold
		// op (unlike sharp's .threshold()). Linear1(a,b) computes a*x+b per pixel; a steep slope
		// collapses everything below/above 128 to 0/255 once the uchar output clamps. Approximate,
		// not spec-certified — same caveat the original TS implementation documents about itself.
		if err := img.Linear1(1000, -127500); err != nil {
			return nil, "", newError(500, "thresholding: %v", err)
		}
	}

	contentType, ok := formats[p.Format]
	if !ok {
		return nil, "", newError(400, "invalid format: %s", p.Format)
	}

	var buf []byte
	switch p.Format {
	case "jpg":
		params := vips.NewJpegExportParams()
		params.Quality = 90
		buf, _, err = img.ExportJpeg(params)
	case "webp":
		params := vips.NewWebpExportParams()
		params.Quality = 90
		buf, _, err = img.ExportWebp(params)
	default: // "png"
		buf, _, err = img.ExportPng(vips.NewPngExportParams())
	}
	if err != nil {
		return nil, "", newError(500, "exporting %s: %v", p.Format, err)
	}
	return buf, contentType, nil
}
