package media

import "testing"

func TestParseRegionFull(t *testing.T) {
	r, err := parseRegion("full", 800, 600)
	if err != nil || r != nil {
		t.Fatalf("parseRegion(full) = %+v, %v, want nil, nil", r, err)
	}
}

func TestParseRegionSquare(t *testing.T) {
	r, err := parseRegion("square", 800, 600)
	if err != nil {
		t.Fatalf("parseRegion(square): %v", err)
	}
	if r.Width != 600 || r.Height != 600 {
		t.Errorf("square region = %+v, want a 600x600 crop of the shorter dimension", r)
	}
	if r.Left != 100 || r.Top != 0 {
		t.Errorf("square region offset = (%d,%d), want centered (100,0)", r.Left, r.Top)
	}
}

func TestParseRegionPixel(t *testing.T) {
	r, err := parseRegion("10,20,300,400", 800, 600)
	if err != nil {
		t.Fatalf("parseRegion: %v", err)
	}
	if r.Left != 10 || r.Top != 20 || r.Width != 300 || r.Height != 400 {
		t.Errorf("region = %+v, want {10 20 300 400}", r)
	}
}

func TestParseRegionPercent(t *testing.T) {
	r, err := parseRegion("pct:10,10,50,50", 1000, 800)
	if err != nil {
		t.Fatalf("parseRegion: %v", err)
	}
	if r.Left != 100 || r.Top != 80 || r.Width != 500 || r.Height != 400 {
		t.Errorf("region = %+v, want {100 80 500 400}", r)
	}
}

func TestParseRegionClampsToImageBounds(t *testing.T) {
	// A region requesting more width/height than remains past its offset must clamp, not error.
	r, err := parseRegion("700,500,500,500", 800, 600)
	if err != nil {
		t.Fatalf("parseRegion: %v", err)
	}
	if r.Width != 100 || r.Height != 100 {
		t.Errorf("region = %+v, want clamped to the remaining 100x100", r)
	}
}

func TestParseRegionRejectsOutOfBounds(t *testing.T) {
	cases := []string{"800,0,10,10", "0,600,10,10", "0,0,0,10", "0,0,10,0"}
	for _, region := range cases {
		if _, err := parseRegion(region, 800, 600); err == nil {
			t.Errorf("parseRegion(%q) = nil error, want an out-of-bounds error", region)
		}
	}
}

func TestParseRegionRejectsMalformed(t *testing.T) {
	cases := []string{"1,2,3", "a,b,c,d", "-1,0,10,10"}
	for _, region := range cases {
		if _, err := parseRegion(region, 800, 600); err == nil {
			t.Errorf("parseRegion(%q) = nil error, want a parse error", region)
		}
	}
}

func TestParseSizeFullMax(t *testing.T) {
	for _, in := range []string{"full", "max"} {
		s, err := parseSize(in, 400, 300)
		if err != nil {
			t.Fatalf("parseSize(%q): %v", in, err)
		}
		if s.Width != 0 || s.Height != 0 {
			t.Errorf("parseSize(%q) = %+v, want unconstrained (0,0)", in, s)
		}
	}
}

func TestParseSizePercent(t *testing.T) {
	s, err := parseSize("pct:50", 400, 300)
	if err != nil {
		t.Fatalf("parseSize: %v", err)
	}
	if s.Width != 200 || s.Height != 150 || s.Fit != "fill" {
		t.Errorf("parseSize(pct:50) = %+v, want {200 150 fill}", s)
	}
}

func TestParseSizePixelWidthOnly(t *testing.T) {
	s, err := parseSize("200,", 400, 300)
	if err != nil {
		t.Fatalf("parseSize: %v", err)
	}
	if s.Width != 200 || s.Height != 0 || s.Fit != "" {
		t.Errorf("parseSize(200,) = %+v, want {200 0 \"\"} (height derived downstream from aspect ratio)", s)
	}
}

func TestParseSizeExactFill(t *testing.T) {
	s, err := parseSize("200,150", 400, 300)
	if err != nil {
		t.Fatalf("parseSize: %v", err)
	}
	if s.Width != 200 || s.Height != 150 || s.Fit != "fill" {
		t.Errorf("parseSize(200,150) = %+v, want {200 150 fill}", s)
	}
}

func TestParseSizeBestFitInside(t *testing.T) {
	s, err := parseSize("!200,150", 400, 300)
	if err != nil {
		t.Fatalf("parseSize: %v", err)
	}
	if s.Width != 200 || s.Height != 150 || s.Fit != "inside" {
		t.Errorf("parseSize(!200,150) = %+v, want {200 150 inside}", s)
	}
}

func TestParseSizeRejectsMalformed(t *testing.T) {
	cases := []string{"", ",", "abc", "pct:0", "pct:-5", "-10,10"}
	for _, size := range cases {
		if _, err := parseSize(size, 400, 300); err == nil {
			t.Errorf("parseSize(%q) = nil error, want a parse error", size)
		}
	}
}

func TestParseRotation(t *testing.T) {
	cases := []struct {
		in      string
		degrees float64
		mirror  bool
	}{
		{"0", 0, false},
		{"90", 90, false},
		{"180", 180, false},
		{"360", 360, false},
		{"!90", 90, true},
		{"45.5", 45.5, false},
	}
	for _, c := range cases {
		r, err := parseRotation(c.in)
		if err != nil {
			t.Fatalf("parseRotation(%q): %v", c.in, err)
		}
		if r.Degrees != c.degrees || r.Mirror != c.mirror {
			t.Errorf("parseRotation(%q) = %+v, want {%v %v}", c.in, r, c.degrees, c.mirror)
		}
	}
}

func TestParseRotationRejectsOutOfRange(t *testing.T) {
	cases := []string{"-1", "361", "abc", ""}
	for _, in := range cases {
		if _, err := parseRotation(in); err == nil {
			t.Errorf("parseRotation(%q) = nil error, want an error", in)
		}
	}
}

func TestClamp(t *testing.T) {
	cases := []struct{ v, lo, hi, want float64 }{
		{5, 0, 10, 5},
		{-5, 0, 10, 0},
		{15, 0, 10, 10},
	}
	for _, c := range cases {
		if got := clamp(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clamp(%v, %v, %v) = %v, want %v", c.v, c.lo, c.hi, got, c.want)
		}
	}
}
