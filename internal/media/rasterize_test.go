package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"math"
	"math/rand"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"

	"studio/internal/i18n"
	"studio/internal/settings"
	"studio/internal/testutil"
)

// noisyPNG builds a losslessly-encoded, high-entropy grayscale PNG - noise so it barely
// compresses, giving a file whose base64 (as embedded in the bake SVG's <image> data: URI)
// comfortably exceeds librsvg/libxml2's ~10 MB single-node parse limit. That limit is exactly what
// broke a real bake in prod ("XML parse error ... Extra content at the end of the document"),
// so the regression test needs an image big enough to hit it.
func noisyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(1))
	if _, err := rng.Read(img.Pix); err != nil {
		t.Fatalf("rng.Read: %v", err)
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// TestRasterizeSVGUnlimited is the direct regression test for the prod bake 500: an annotation SVG
// whose embedded base64 base image is larger than librsvg/libxml2's ~10 MB per-node parse limit
// must still rasterize (rasterizeSVG sets svgload's "unlimited" flag). The plain loader is checked
// too, only to document that it's the one that rejects this input.
func TestRasterizeSVGUnlimited(t *testing.T) {
	raw := noisyPNG(t, 3000, 3000) // ~9M px grayscale noise -> PNG ~9 MB -> base64 ~12M chars, over the ~10 MB limit
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
	if len(dataURI) < 10_000_000 {
		t.Fatalf("test image too small: data URI is %d bytes, need > 10 MB to exercise the limit", len(dataURI))
	}
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="200" height="200">`+
			`<image xlink:href="%s" x="0" y="0" width="200" height="200" preserveAspectRatio="none"/>`+
			`<rect x="10" y="10" width="50" height="50" fill="red" fill-opacity="0.4"/></svg>`,
		dataURI)

	if _, err := vips.NewImageFromBuffer([]byte(svg)); err == nil {
		t.Log("note: plain vips.NewImageFromBuffer accepted the oversized SVG on this libvips build")
	} else {
		t.Logf("plain loader rejects it as expected: %v", firstLine(err.Error()))
	}

	ref, err := rasterizeSVG(svg)
	if err != nil {
		t.Fatalf("rasterizeSVG rejected an SVG with a >10 MB embedded image: %v", err)
	}
	defer ref.Close()
	if ref.Width() != 200 || ref.Height() != 200 {
		t.Errorf("rasterized size = %dx%d, want 200x200", ref.Width(), ref.Height())
	}
}

func firstLine(s string) string {
	if i := bytes.IndexByte([]byte(s), '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ptOnA4 is the inverse of bakedTextPx for a landscape/square canvas (one that fills the A4
// content width): given a font size in the baked canvas's pixel space, what point size does it
// render at on the page.
func ptOnA4(px float64, canvasW int) float64 {
	mmPerPx := bakeA4ContentWidthMM / float64(canvasW)
	return px * mmPerPx / bakeMMPerPoint
}

func TestBakedTextPx(t *testing.T) {
	// A mid-size landscape photo's note lands right on the target point size on A4.
	if got := ptOnA4(bakedTextPx(3000, 2000, bakeNotePt), 3000); math.Abs(got-bakeNotePt) > 0.01 {
		t.Errorf("3000x2000 note renders at %.2fpt on A4, want %.2fpt", got, bakeNotePt)
	}
	// Same target point size regardless of how many megapixels the source was - that's the point.
	for _, w := range []int{600, 1200, 2400, 4000, 9000} {
		if got := ptOnA4(bakedTextPx(w, w*2/3, bakeNotePt), w); math.Abs(got-bakeNotePt) > 0.01 {
			t.Errorf("%dpx-wide note renders at %.2fpt on A4, want %.2fpt", w, got, bakeNotePt)
		}
	}
	// Bigger canvas -> bigger pixel font (so it stays constant on paper).
	if bakedTextPx(4000, 3000, bakeNotePt) <= bakedTextPx(1000, 750, bakeNotePt) {
		t.Errorf("expected a wider canvas to get a larger pixel font")
	}
	// The 20pt ceiling and 8pt floor are hard: an absurd request is clamped before scaling.
	if got := ptOnA4(bakedTextPx(3000, 2000, 40), 3000); math.Abs(got-bakeTextMaxPt) > 0.01 {
		t.Errorf("40pt request renders at %.2fpt on A4, want the %.0fpt ceiling", got, bakeTextMaxPt)
	}
	if got := ptOnA4(bakedTextPx(3000, 2000, 3), 3000); math.Abs(got-bakeTextMinPt) > 0.01 {
		t.Errorf("3pt request renders at %.2fpt on A4, want the %.0fpt floor", got, bakeTextMinPt)
	}
	// The legend label sits above the note size.
	if !(bakedTextPx(3000, 2000, bakeNotePt) < bakedTextPx(3000, 2000, bakeLegendLabelPt)) {
		t.Errorf("legend label should be larger than the note")
	}
	// A tall portrait image is shown narrower than full width (it hits the page height first), so
	// its caption gets a larger pixel font - sized so it still lands near the target once displayed
	// at that narrower width, rather than ballooning past the ceiling.
	if !(bakedTextPx(2000, 5000, bakeNotePt) > bakedTextPx(2000, 1333, bakeNotePt)) {
		t.Errorf("a portrait canvas should get a larger pixel font than a landscape one of the same width")
	}
	// That portrait, displayed at its height-bound width, still renders at the target point size.
	portraitDisplayMM := bakeA4ContentHeightMM * 2000.0 / 5000.0
	gotPt := bakedTextPx(2000, 5000, bakeNotePt) * (portraitDisplayMM / 2000.0) / bakeMMPerPoint
	if math.Abs(gotPt-bakeNotePt) > 0.01 {
		t.Errorf("portrait note renders at %.2fpt when shown full-page, want %.2fpt", gotPt, bakeNotePt)
	}
}

// TestBakeAnnotatedVersionRealImage runs the whole save-an-annotated-image path end to end on a
// real photo-sized image whose grayscale base, embedded in the composite SVG, is large enough to
// have tripped the librsvg parse limit before rasterizeSVG's "unlimited" flag: upload original ->
// CreateAnnotatedVersion -> draw a region + set a note -> BakeAnnotatedVersion, then decode the
// persisted file to prove it's a valid image whose canvas grew to fit the legend/note beneath the
// (unstretched) photo.
func TestBakeAnnotatedVersionRealImage(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	storage, err := NewLocalDiskAdapter(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDiskAdapter: %v", err)
	}
	svc := &Service{Pool: pool, Storage: storage}
	userID := createFixtureUser(t, ctx, svc)

	// The annotation_type classifiers are seeded by migration 0013, so a migrated test DB already
	// has them.
	types, err := settings.GetClassifiers(ctx, pool, settings.ClassifierAnnotationType)
	if err != nil || len(types) == 0 {
		t.Fatalf("GetClassifiers(annotation_type): %v (n=%d)", err, len(types))
	}

	const srcW, srcH = 3000, 3000
	original, err := svc.UploadMedia(ctx, noisyPNG(t, srcW, srcH), "image/png", userID)
	if err != nil {
		t.Fatalf("UploadMedia: %v", err)
	}

	version, err := svc.CreateAnnotatedVersion(ctx, *original, userID)
	if err != nil {
		t.Fatalf("CreateAnnotatedVersion: %v", err)
	}
	if _, err := CreateRegion(ctx, pool, version.ID, types[0].ID, 10, 10, 25, 25, "lower-edge losses"); err != nil {
		t.Fatalf("CreateRegion: %v", err)
	}
	if err := UpdateDescription(ctx, pool, version.ID, "Losses along the lower edge; retouching test note that wraps across more than one line."); err != nil {
		t.Fatalf("UpdateDescription: %v", err)
	}

	version, err = GetByID(ctx, pool, version.ID)
	if err != nil {
		t.Fatalf("GetByID(version): %v", err)
	}
	if err := svc.BakeAnnotatedVersion(ctx, *version, i18n.LocaleEN); err != nil {
		t.Fatalf("BakeAnnotatedVersion: %v", err)
	}

	baked, err := storage.Get(version.StorageKey)
	if err != nil {
		t.Fatalf("reading baked file: %v", err)
	}
	ref, err := vips.NewImageFromBuffer(baked)
	if err != nil {
		t.Fatalf("baked file is not a decodable image: %v", err)
	}
	defer ref.Close()
	if ref.Width() != srcW {
		t.Errorf("baked width = %d, want %d (photo must not be stretched)", ref.Width(), srcW)
	}
	if ref.Height() <= srcH {
		t.Errorf("baked height = %d, want > %d (canvas should grow for the legend/note)", ref.Height(), srcH)
	}

	got, err := GetByID(ctx, pool, version.ID)
	if err != nil {
		t.Fatalf("GetByID after bake: %v", err)
	}
	if !got.IsBaked() {
		t.Errorf("version.IsBaked() = false after BakeAnnotatedVersion, want true")
	}
}
