package commerce

import (
	"context"
	"testing"
	"time"

	studiodb "studio/internal/db"
	"studio/internal/testutil"
)

func TestRound2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		// 1.005*100 isn't exactly representable in float64 (it's ~100.49999999999999), so this
		// rounds down — same behavior as the original's Math.round(x*100)/100 in lib/domain/
		// pricing.ts, not a Go-specific quirk. Documenting the actual (inherited) behavior here
		// rather than an idealized one.
		{1.005, 1.0},
		{1.004, 1.0},
		{0, 0},
		{123.456, 123.46},
		{-2.345, -2.35},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"short", 60, "short"},
		{"", 10, ""},
		{"exactly10c", 10, "exactly10c"},
		{"this is longer than ten characters", 10, "this is lo"},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestEstimateProjectCost(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	userID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO User (id, name, email, provider, role) VALUES (?, ?, ?, ?, ?)",
		userID, "Test Conservator", "conservator@test.local", "dev", "conservator")

	assetTypeID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO Classifier (id, type, code, title, sequence, updatedAt) VALUES (?, 'asset_type', 'painting', 'Painting', 0, ?)`,
		assetTypeID, time.Now())

	// A billable activity type (defaultRate set) and a non-billable one (no rate — e.g. a
	// waiting period) — EstimateProjectCost must skip the latter entirely, not price it at 0.
	billableTypeID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO Classifier (id, type, code, title, sequence, data, updatedAt) VALUES (?, 'activity_type', 'cleaning', 'Surface Cleaning', 0, ?, ?)`,
		billableTypeID, `{"defaultRate": 60}`, time.Now())
	waitingTypeID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO Classifier (id, type, code, title, sequence, data, updatedAt) VALUES (?, 'activity_type', 'drying', 'Drying', 1, ?, ?)`,
		waitingTypeID, `{"defaultRate": 0, "isWaitingPeriod": true}`, time.Now())

	clientID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Client (id, name, updatedAt) VALUES (?, ?, ?)", clientID, "Test Client", time.Now())

	assetID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Asset (id, clientId, referenceCode, assetTypeId, updatedAt) VALUES (?, ?, ?, ?, ?)",
		assetID, clientID, "A-0001", assetTypeID, time.Now())

	projectID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Project (id, assetId, title, updatedAt) VALUES (?, ?, ?, ?)",
		projectID, assetID, "Test Workflow", time.Now())

	// 90 minutes billable at 60/hr = 1.5h * 60 = 90.00
	mustExec(t, pool, `INSERT INTO Activity (id, projectId, activityTypeId, userId, description, startedAt, durationMinutes) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		studiodb.NewID(), projectID, billableTypeID, userID, "Removed surface grime with a dry sponge across the whole front panel", time.Now(), 90)
	// A second billable entry: 30 minutes at 60/hr = 30.00
	mustExec(t, pool, `INSERT INTO Activity (id, projectId, activityTypeId, userId, description, startedAt, durationMinutes) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		studiodb.NewID(), projectID, billableTypeID, userID, "Second pass", time.Now(), 30)
	// Waiting period: has duration but defaultRate 0 — must not appear in items or total.
	mustExec(t, pool, `INSERT INTO Activity (id, projectId, activityTypeId, userId, description, startedAt, durationMinutes) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		studiodb.NewID(), projectID, waitingTypeID, userID, "Left to dry", time.Now(), 480)
	// No duration logged at all — must be skipped, not treated as a zero-cost line item.
	mustExec(t, pool, `INSERT INTO Activity (id, projectId, activityTypeId, userId, description, startedAt) VALUES (?, ?, ?, ?, ?, ?)`,
		studiodb.NewID(), projectID, billableTypeID, userID, "Notes only, no time logged", time.Now())

	items, total, err := EstimateProjectCost(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("EstimateProjectCost: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d line items, want 2 (waiting period and no-duration activity should be excluded): %+v", len(items), items)
	}
	if total != 120 {
		t.Errorf("total = %v, want 120 (90.00 + 30.00)", total)
	}
	if items[0].Amount != 90 || items[0].EstimatedHours != 1.5 || items[0].Rate != 60 {
		t.Errorf("items[0] = %+v, want Amount=90 EstimatedHours=1.5 Rate=60", items[0])
	}
	if items[1].Amount != 30 || items[1].EstimatedHours != 0.5 {
		t.Errorf("items[1] = %+v, want Amount=30 EstimatedHours=0.5", items[1])
	}

	// Description truncates the activity title + description to a readable line-item label.
	if got := items[0].Description; got == "" {
		t.Errorf("items[0].Description is empty")
	}
}

func mustExec(t *testing.T, q studiodb.Querier, query string, args ...any) {
	t.Helper()
	if _, err := studiodb.Execute(context.Background(), q, query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
