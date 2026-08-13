package media

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/davidbyttow/govips/v2/vips"
)

// A practical implementation of the IIIF Image API (https://iiif.io/api/image/3/) — region,
// size, rotation, quality, and format transforms applied live via govips against the stored
// original file. Level 2-*equivalent* feature set, same as the original app's sharp-based
// implementation (lib/iiif/imageApi.ts) — a from-scratch port of that file's parsing logic onto
// govips ops, not run through the official IIIF validator either.
//
// One deliberate gap vs. the original: govips has no direct 1-bit threshold operation (sharp's
// `.threshold()`), so IIIF quality=bitonal is approximated as grayscale rather than true
// bilevel. Nothing in this app's own UI requests bitonal — it's spec surface for third-party IIIF
// clients, not app-invoked.

type IIIFError struct {
	Message string
	Status  int
}

func (e *IIIFError) Error() string { return e.Message }

func NewIIIFError(message string, status int) *IIIFError {
	return &IIIFError{Message: message, Status: status}
}

type IIIFParams struct {
	Region   string
	Size     string
	Rotation string
	Quality  string
	Format   string
}

type pixelRegion struct {
	Left, Top, Width, Height int
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
		return nil, NewIIIFError("Invalid region: "+region, 400)
	}
	nums := make([]float64, 4)
	for i, p := range parts {
		n, err := strconv.ParseFloat(p, 64)
		if err != nil || n < 0 {
			return nil, NewIIIFError("Invalid region: "+region, 400)
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
		return nil, NewIIIFError("Region out of bounds: "+region, 400)
	}

	x = clamp(x, 0, float64(imgW-1))
	y = clamp(y, 0, float64(imgH-1))
	w = clamp(w, 1, float64(imgW)-x)
	h = clamp(h, 1, float64(imgH)-y)

	return &pixelRegion{Left: int(math.Round(x)), Top: int(math.Round(y)), Width: int(math.Round(w)), Height: int(math.Round(h))}, nil
}

type parsedSize struct {
	Width, Height int    // 0 means unconstrained on that axis
	Fit           string // "fill" | "inside" | ""
}

var sizeRE = regexp.MustCompile(`^(\d+)?,(\d+)?$`)

func parseSize(size string, regionW, regionH int) (parsedSize, error) {
	if size == "full" || size == "max" {
		return parsedSize{}, nil
	}
	if strings.HasPrefix(size, "pct:") {
		n, err := strconv.ParseFloat(size[4:], 64)
		if err != nil || n <= 0 {
			return parsedSize{}, NewIIIFError("Invalid size: "+size, 400)
		}
		return parsedSize{
			Width:  maxInt(1, int(math.Round(float64(regionW)*n/100))),
			Height: maxInt(1, int(math.Round(float64(regionH)*n/100))),
			Fit:    "fill",
		}, nil
	}

	best := strings.HasPrefix(size, "!")
	spec := size
	if best {
		spec = size[1:]
	}
	match := sizeRE.FindStringSubmatch(spec)
	if match == nil || (match[1] == "" && match[2] == "") {
		return parsedSize{}, NewIIIFError("Invalid size: "+size, 400)
	}
	var width, height int
	if match[1] != "" {
		width, _ = strconv.Atoi(match[1])
	}
	if match[2] != "" {
		height, _ = strconv.Atoi(match[2])
	}
	if width < 0 || height < 0 {
		return parsedSize{}, NewIIIFError("Invalid size: "+size, 400)
	}

	bothGiven := width != 0 && height != 0
	fit := ""
	if bothGiven {
		if best {
			fit = "inside"
		} else {
			fit = "fill"
		}
	}
	return parsedSize{Width: width, Height: height, Fit: fit}, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
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
		return parsedRotation{}, NewIIIFError("Invalid rotation: "+rotation, 400)
	}
	return parsedRotation{Degrees: degrees, Mirror: mirror}, nil
}

var iiifQualities = map[string]bool{"default": true, "color": true, "gray": true, "bitonal": true}
var iiifFormats = map[string]string{"jpg": "image/jpeg", "png": "image/png", "webp": "image/webp"}

func (s *Service) TransformImage(ctx context.Context, mediaID string, params IIIFParams) ([]byte, string, error) {
	m, err := GetByID(ctx, s.Pool, mediaID)
	if err != nil {
		return nil, "", err
	}
	if m == nil || m.Kind != KindImage {
		return nil, "", NewIIIFError("Not an image", 404)
	}

	file, err := s.ReadMediaFile(ctx, mediaID, "original")
	if err != nil {
		return nil, "", err
	}
	if file == nil {
		return nil, "", NewIIIFError("Original file not found", 404)
	}

	img, err := vips.NewImageFromBuffer(file.Data)
	if err != nil {
		return nil, "", fmt.Errorf("decoding image: %w", err)
	}
	defer img.Close()

	imgW, imgH := int(m.Width.Int64), int(m.Height.Int64)
	if imgW == 0 {
		imgW = img.Width()
	}
	if imgH == 0 {
		imgH = img.Height()
	}
	if imgW == 0 || imgH == 0 {
		return nil, "", NewIIIFError("Unknown image dimensions", 500)
	}

	region, err := parseRegion(params.Region, imgW, imgH)
	if err != nil {
		return nil, "", err
	}
	regionW, regionH := imgW, imgH
	if region != nil {
		if err := img.ExtractArea(region.Left, region.Top, region.Width, region.Height); err != nil {
			return nil, "", fmt.Errorf("extracting region: %w", err)
		}
		regionW, regionH = region.Width, region.Height
	}

	size, err := parseSize(params.Size, regionW, regionH)
	if err != nil {
		return nil, "", err
	}
	if size.Width != 0 || size.Height != 0 {
		if err := resizeTo(img, regionW, regionH, size); err != nil {
			return nil, "", fmt.Errorf("resizing: %w", err)
		}
	}

	rotation, err := parseRotation(params.Rotation)
	if err != nil {
		return nil, "", err
	}
	if rotation.Mirror {
		if err := img.Flip(vips.DirectionHorizontal); err != nil {
			return nil, "", fmt.Errorf("mirroring: %w", err)
		}
	}
	if rotation.Degrees != 0 {
		// Similarity handles arbitrary angles (incl. exact 90/180/270) in one op, with a solid
		// background for the corners a non-90x rotation exposes - same as the original's
		// sharp .rotate(degrees, { background: "#ffffff" }).
		bg := &vips.ColorRGBA{R: 255, G: 255, B: 255, A: 255}
		if err := img.Similarity(1.0, rotation.Degrees, bg, 0, 0, 0, 0); err != nil {
			return nil, "", fmt.Errorf("rotating: %w", err)
		}
	}

	if !iiifQualities[params.Quality] {
		return nil, "", NewIIIFError("Invalid quality: "+params.Quality, 400)
	}
	if params.Quality == "gray" || params.Quality == "bitonal" {
		if err := img.ToColorSpace(vips.InterpretationBW); err != nil {
			return nil, "", fmt.Errorf("converting to grayscale: %w", err)
		}
	}

	contentType, ok := iiifFormats[params.Format]
	if !ok {
		return nil, "", NewIIIFError("Invalid format: "+params.Format, 400)
	}

	var buf []byte
	switch params.Format {
	case "jpg":
		p := vips.NewJpegExportParams()
		p.Quality = 90
		buf, _, err = img.ExportJpeg(p)
	case "webp":
		p := vips.NewWebpExportParams()
		p.Quality = 90
		buf, _, err = img.ExportWebp(p)
	default: // png
		buf, _, err = img.ExportPng(vips.NewPngExportParams())
	}
	if err != nil {
		return nil, "", fmt.Errorf("exporting %s: %w", params.Format, err)
	}
	return buf, contentType, nil
}

func resizeTo(img *vips.ImageRef, regionW, regionH int, size parsedSize) error {
	switch {
	case size.Width == 0: // height-only - scale proportionally
		return img.Resize(float64(size.Height)/float64(regionH), vips.KernelAuto)
	case size.Height == 0: // width-only
		return img.Resize(float64(size.Width)/float64(regionW), vips.KernelAuto)
	case size.Fit == "inside":
		scale := minFloat(float64(size.Width)/float64(regionW), float64(size.Height)/float64(regionH))
		return img.Resize(scale, vips.KernelAuto)
	default: // "fill" - exact dimensions, distorts aspect ratio if needed
		return img.ResizeWithVScale(float64(size.Width)/float64(regionW), float64(size.Height)/float64(regionH), vips.KernelAuto)
	}
}

// BuildInfoJSON is the IIIF Image API 3.0 `info.json` descriptor —
// see https://iiif.io/api/image/3/#5-image-information.
func BuildInfoJSON(mediaID string, width, height int, origin string) map[string]any {
	return map[string]any{
		"@context":  "http://iiif.io/api/image/3/context.json",
		"id":        origin + "/api/iiif/" + mediaID,
		"type":      "ImageService3",
		"protocol":  "http://iiif.io/api/image",
		"profile":   "level2",
		"width":     width,
		"height":    height,
		"maxWidth":  width,
		"maxHeight": height,
		// Documents exactly what TransformImage above actually implements - not a claim of
		// official validator-certified conformance.
		"extraFeatures": []string{
			"regionByPx", "regionByPct", "regionSquare", "sizeByW", "sizeByH", "sizeByWh",
			"sizeByPct", "sizeByConfinedWh", "rotationArbitrary", "mirroring",
		},
		"extraFormats":   []string{"png", "webp"},
		"extraQualities": []string{"color", "gray", "bitonal"},
	}
}
