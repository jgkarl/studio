package reporter

import (
	"context"
	"strings"
	"testing"
	"time"

	studiodb "studio/internal/db"
	"studio/internal/testutil"
)

func mustExec(t *testing.T, q studiodb.Querier, query string, args ...any) {
	t.Helper()
	if _, err := studiodb.Execute(context.Background(), q, query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func TestBuildSuggestedOutlineUnknownAsset(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	sections, err := BuildSuggestedOutline(context.Background(), pool, "does-not-exist")
	if err != nil {
		t.Fatalf("BuildSuggestedOutline: %v", err)
	}
	if sections.ConditionFindings != "" || sections.TreatmentPerformed != "" {
		t.Errorf("expected empty sections for an unknown asset, got %+v", sections)
	}
}

func TestBuildSuggestedOutline(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	assetTypeID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO Classifier (id, type, code, title, sequence, updatedAt) VALUES (?, 'asset_type', 'painting', 'Painting', 0, ?)`,
		assetTypeID, time.Now())
	methodID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO Classifier (id, type, code, title, sequence, updatedAt) VALUES (?, 'treatment_method', 'surface_cleaning', 'Surface cleaning', 0, ?)`,
		methodID, time.Now())

	clientID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Client (id, name, updatedAt) VALUES (?, ?, ?)", clientID, "Test Client", time.Now())

	assetID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Asset (id, clientId, referenceCode, assetTypeId, updatedAt) VALUES (?, ?, ?, ?, ?)",
		assetID, clientID, "A-0001", assetTypeID, time.Now())

	mustExec(t, pool, `INSERT INTO AssetState (id, assetId, "condition", description, recordedAt) VALUES (?, ?, ?, ?, ?)`,
		studiodb.NewID(), assetID, "fair", "Surface grime, minor tears at edges.", time.Now().Add(-2*time.Hour))

	mustExec(t, pool, "INSERT INTO Treatment (id, assetId, method, title, notes, performedAt, updatedAt) VALUES (?, ?, ?, ?, ?, ?, ?)",
		studiodb.NewID(), assetID, "surface_cleaning", "Surface cleaning", "Removed surface grime with dry sponge.", time.Now().Add(-time.Hour), time.Now())

	mustExec(t, pool, `INSERT INTO AssetState (id, assetId, "condition", description, recordedAt) VALUES (?, ?, ?, ?, ?)`,
		studiodb.NewID(), assetID, "good", "Grime removed, tears stable.", time.Now())

	sections, err := BuildSuggestedOutline(ctx, pool, assetID)
	if err != nil {
		t.Fatalf("BuildSuggestedOutline: %v", err)
	}

	if !strings.Contains(sections.ConditionFindings, "On arrival: Surface grime, minor tears at edges.") {
		t.Errorf("ConditionFindings missing intake line: %q", sections.ConditionFindings)
	}
	if !strings.Contains(sections.ConditionFindings, "Most recent: Grime removed, tears stable.") {
		t.Errorf("ConditionFindings missing most-recent line: %q", sections.ConditionFindings)
	}
	if !strings.Contains(sections.TreatmentPerformed, "Surface cleaning: Removed surface grime with dry sponge.") {
		t.Errorf("TreatmentPerformed missing treatment line: %q", sections.TreatmentPerformed)
	}
}

func TestBuildSuggestedOutlineSingleStateHasNoMostRecentLine(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	assetTypeID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO Classifier (id, type, code, title, sequence, updatedAt) VALUES (?, 'asset_type', 'painting', 'Painting', 0, ?)`,
		assetTypeID, time.Now())
	clientID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Client (id, name, updatedAt) VALUES (?, ?, ?)", clientID, "Test Client", time.Now())
	assetID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Asset (id, clientId, referenceCode, assetTypeId, updatedAt) VALUES (?, ?, ?, ?, ?)",
		assetID, clientID, "A-0002", assetTypeID, time.Now())

	mustExec(t, pool, `INSERT INTO AssetState (id, assetId, "condition", description, recordedAt) VALUES (?, ?, ?, ?, ?)`,
		studiodb.NewID(), assetID, "good", "Only one state logged so far.", time.Now())

	sections, err := BuildSuggestedOutline(ctx, pool, assetID)
	if err != nil {
		t.Fatalf("BuildSuggestedOutline: %v", err)
	}
	if strings.Contains(sections.ConditionFindings, "Most recent:") {
		t.Errorf("expected no 'Most recent:' line when only one AssetState is logged, got %q", sections.ConditionFindings)
	}
	if !strings.Contains(sections.ConditionFindings, "On arrival: Only one state logged so far.") {
		t.Errorf("expected the single state as the 'On arrival' line, got %q", sections.ConditionFindings)
	}
}
