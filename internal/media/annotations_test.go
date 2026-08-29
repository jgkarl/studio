package media

import (
	"context"
	"strings"
	"testing"
)

func TestPolygonPoints(t *testing.T) {
	got := PolygonPoints(`[{"x":1,"y":2},{"x":50.5,"y":99.25}]`)
	want := "1.000,2.000 50.500,99.250"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPolygonPointsInvalidJSON(t *testing.T) {
	if got := PolygonPoints("not json"); got != "" {
		t.Fatalf("expected empty string for invalid pathData, got %q", got)
	}
}

func TestCreateFreehandRegionRejectsInvalidJSON(t *testing.T) {
	// Validation happens before any database access, so a nil Querier is safe here — these cases
	// must fail before ever reaching studiodb.Execute.
	if _, err := CreateFreehandRegion(context.Background(), nil, "m1", "t1", "not json", ""); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCreateFreehandRegionRejectsTooFewPoints(t *testing.T) {
	if _, err := CreateFreehandRegion(context.Background(), nil, "m1", "t1", `[{"x":1,"y":1},{"x":2,"y":2}]`, ""); err == nil {
		t.Fatal("expected error for fewer than 3 points")
	} else if !strings.Contains(err.Error(), "3 points") {
		t.Fatalf("unexpected error: %v", err)
	}
}
