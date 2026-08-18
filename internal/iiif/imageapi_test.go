package iiif

import "testing"

func TestParseRegionFull(t *testing.T) {
	r, err := parseRegion("full", 800, 600)
	if err != nil || r != nil {
		t.Fatalf("full region should be nil/no-op, got %+v, err %v", r, err)
	}
}

func TestParseRegionSquare(t *testing.T) {
	r, err := parseRegion("square", 800, 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Width != 600 || r.Height != 600 || r.Left != 100 || r.Top != 0 {
		t.Fatalf("expected centered 600x600 square, got %+v", r)
	}
}

func TestParseRegionPixel(t *testing.T) {
	r, err := parseRegion("10,20,100,50", 800, 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Left != 10 || r.Top != 20 || r.Width != 100 || r.Height != 50 {
		t.Fatalf("unexpected region: %+v", r)
	}
}

func TestParseRegionPercent(t *testing.T) {
	r, err := parseRegion("pct:10,10,50,50", 1000, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Left != 100 || r.Top != 20 || r.Width != 500 || r.Height != 100 {
		t.Fatalf("unexpected region: %+v", r)
	}
}

func TestParseRegionClampsOverflow(t *testing.T) {
	// A region requesting more than what's left of the image past its own origin must be
	// clamped to the image bounds, not rejected outright — this is the IIIF spec's own behavior.
	r, err := parseRegion("700,500,500,500", 800, 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Left != 700 || r.Top != 500 || r.Width != 100 || r.Height != 100 {
		t.Fatalf("expected clamp to image bounds, got %+v", r)
	}
}

func TestParseRegionOutOfBounds(t *testing.T) {
	if _, err := parseRegion("900,10,10,10", 800, 600); err == nil {
		t.Fatal("expected error for x beyond image width")
	}
}

func TestParseRegionInvalidPartCount(t *testing.T) {
	if _, err := parseRegion("10,10,10", 800, 600); err == nil {
		t.Fatal("expected error for wrong number of region parts")
	}
}

func TestParseSizeFullMax(t *testing.T) {
	for _, s := range []string{"full", "max"} {
		ps, err := parseSize(s, 800, 600)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", s, err)
		}
		if ps.Width != 0 || ps.Height != 0 {
			t.Fatalf("%s: expected unconstrained size, got %+v", s, ps)
		}
	}
}

func TestParseSizePercent(t *testing.T) {
	ps, err := parseSize("pct:50", 800, 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.Width != 400 || ps.Height != 300 || ps.Fit != "fill" {
		t.Fatalf("unexpected size: %+v", ps)
	}
}

func TestParseSizeWidthOnly(t *testing.T) {
	ps, err := parseSize("400,", 800, 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.Width != 400 || ps.Height != 0 || ps.Fit != "" {
		t.Fatalf("expected width-only auto-height, got %+v", ps)
	}
}

func TestParseSizeExactDistorts(t *testing.T) {
	ps, err := parseSize("400,400", 800, 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.Width != 400 || ps.Height != 400 || ps.Fit != "fill" {
		t.Fatalf("expected exact fill (distorting), got %+v", ps)
	}
}

func TestParseSizeBestFit(t *testing.T) {
	ps, err := parseSize("!400,400", 800, 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.Width != 400 || ps.Height != 400 || ps.Fit != "inside" {
		t.Fatalf("expected best-fit inside, got %+v", ps)
	}
}

func TestParseSizeInvalid(t *testing.T) {
	cases := []string{"", "0,0", "-10,20", "abc,def"}
	for _, c := range cases {
		if _, err := parseSize(c, 800, 600); err == nil {
			t.Fatalf("%q: expected error", c)
		}
	}
}

func TestParseRotationPlain(t *testing.T) {
	r, err := parseRotation("90")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Degrees != 90 || r.Mirror {
		t.Fatalf("unexpected rotation: %+v", r)
	}
}

func TestParseRotationMirror(t *testing.T) {
	r, err := parseRotation("!180")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Degrees != 180 || !r.Mirror {
		t.Fatalf("unexpected rotation: %+v", r)
	}
}

func TestParseRotationArbitraryDegree(t *testing.T) {
	r, err := parseRotation("45.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Degrees != 45.5 {
		t.Fatalf("unexpected rotation: %+v", r)
	}
}

func TestParseRotationOutOfRange(t *testing.T) {
	cases := []string{"-10", "361", "abc"}
	for _, c := range cases {
		if _, err := parseRotation(c); err == nil {
			t.Fatalf("%q: expected error", c)
		}
	}
}
