package seed

import (
	"context"
	"testing"

	"stuudio/internal/media"
	"stuudio/internal/testutil"
)

func newTestMediaService(t *testing.T) *media.Service {
	t.Helper()
	storage, err := media.NewLocalDiskAdapter(t.TempDir())
	if err != nil {
		t.Fatalf("media storage: %v", err)
	}
	media.InitImageProcessing()
	t.Cleanup(media.ShutdownImageProcessing)
	return &media.Service{Storage: storage}
}

func TestSeedDemoData(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	mediaSvc := newTestMediaService(t)
	mediaSvc.Pool = pool

	if err := SeedDemoData(ctx, pool, mediaSvc); err != nil {
		t.Fatalf("SeedDemoData: %v", err)
	}

	checks := []struct {
		label string
		query string
	}{
		{"User", "SELECT COUNT(*) AS n FROM User"},
		{"User with role conservator", "SELECT COUNT(*) AS n FROM User WHERE role = 'conservator' AND provider = 'email' AND passwordHash IS NOT NULL AND emailVerifiedAt IS NOT NULL"},
		{"Client", "SELECT COUNT(*) AS n FROM Client"},
		{"Asset", "SELECT COUNT(*) AS n FROM Asset"},
		{"Assessment", "SELECT COUNT(*) AS n FROM Assessment"},
		{"Project", "SELECT COUNT(*) AS n FROM Project"},
		{"Treatment", "SELECT COUNT(*) AS n FROM Treatment"},
		{"Report", "SELECT COUNT(*) AS n FROM Report"},
		{"Media", "SELECT COUNT(*) AS n FROM Media"},
		{"MediaReference", "SELECT COUNT(*) AS n FROM MediaReference"},
		{"MediaAnnotationRegion", "SELECT COUNT(*) AS n FROM MediaAnnotationRegion"},
	}
	want := map[string]int{
		"User":                       1, // conservator only — BootstrapAdmin isn't called in this test
		"User with role conservator": 1,
		"Client":                     2,
		"Asset":                      2,
		"Assessment":                 3,
		"Project":                    2,
		"Treatment":                  1,
		"Report":                     1,
		"Media":                      4,
		"MediaReference":             4,
		"MediaAnnotationRegion":      1,
	}

	for _, c := range checks {
		if got, exp := countRows(t, pool, c.query), want[c.label]; got != exp {
			t.Errorf("%s count = %d, want %d", c.label, got, exp)
		}
	}

	// Re-running must be a no-op (idempotent) — no duplicate rows on a second boot.
	if err := SeedDemoData(ctx, pool, mediaSvc); err != nil {
		t.Fatalf("SeedDemoData (second run): %v", err)
	}
	for _, c := range checks {
		if got, exp := countRows(t, pool, c.query), want[c.label]; got != exp {
			t.Errorf("after re-run: %s count = %d, want %d", c.label, got, exp)
		}
	}
}
