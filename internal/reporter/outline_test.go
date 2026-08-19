package reporter

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	studiodb "stuudio/internal/db"
	"stuudio/internal/testutil"
)

func mustExec(t *testing.T, q studiodb.Querier, query string, args ...any) {
	t.Helper()
	if _, err := studiodb.Execute(context.Background(), q, query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// mustClassifierID looks up a Classifier row's id by (type, code) — used instead of inserting a
// fresh fixture row, since db/migrations/0009_seed_classifiers.sql now seeds every asset_type/
// treatment_method code these tests need as part of every testutil.OpenTestDB migration run, and
// a second INSERT would collide with Classifier's (type, code) unique index.
func mustClassifierID(t *testing.T, q studiodb.Querier, classifierType, code string) string {
	t.Helper()
	id, err := studiodb.QueryOne(context.Background(), q,
		"SELECT id FROM Classifier WHERE type = ? AND code = ?",
		func(rows *sql.Rows) (string, error) {
			var id string
			err := rows.Scan(&id)
			return id, err
		}, classifierType, code)
	if err != nil {
		t.Fatalf("looking up classifier %s/%s: %v", classifierType, code, err)
	}
	if id == nil {
		t.Fatalf("classifier %s/%s not found — expected it to be seeded by 0009_seed_classifiers.sql", classifierType, code)
	}
	return *id
}

// mustProject creates a Client/Asset/Project fixture chain and returns the Project's id — every
// Assessment/Treatment BuildSuggestedOutline reads is now Project-scoped.
func mustProject(t *testing.T, q studiodb.Querier, refCode string) string {
	t.Helper()
	assetTypeID := mustClassifierID(t, q, "asset_type", "painting")

	clientID := studiodb.NewID()
	mustExec(t, q, "INSERT INTO Client (id, name, updatedAt) VALUES (?, ?, ?)", clientID, "Test Client", time.Now())

	assetID := studiodb.NewID()
	mustExec(t, q, "INSERT INTO Asset (id, clientId, referenceCode, assetTypeId, updatedAt) VALUES (?, ?, ?, ?, ?)",
		assetID, clientID, refCode, assetTypeID, time.Now())

	projectID := studiodb.NewID()
	mustExec(t, q, "INSERT INTO Project (id, assetId, title, updatedAt) VALUES (?, ?, ?, ?)",
		projectID, assetID, "Test project", time.Now())
	return projectID
}

func TestBuildSuggestedOutlineUnknownProject(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	sections, err := BuildSuggestedOutline(context.Background(), pool, "does-not-exist")
	if err != nil {
		t.Fatalf("BuildSuggestedOutline: %v", err)
	}
	if sections.ConditionFindings != "" || sections.TreatmentPerformed != "" {
		t.Errorf("expected empty sections for an unknown project, got %+v", sections)
	}
}

func TestBuildSuggestedOutline(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID := mustProject(t, pool, "A-0001")

	mustExec(t, pool, `INSERT INTO Assessment (id, projectId, assetId, "condition", description, recordedAt, updatedAt)
		SELECT ?, id, assetId, ?, ?, ?, ? FROM Project WHERE id = ?`,
		studiodb.NewID(), "fair", "Surface grime, minor tears at edges.", time.Now().Add(-2*time.Hour), time.Now(), projectID)

	mustExec(t, pool, `INSERT INTO Treatment (id, projectId, assetId, method, title, notes, performedAt, updatedAt)
		SELECT ?, id, assetId, ?, ?, ?, ?, ? FROM Project WHERE id = ?`,
		studiodb.NewID(), "surface_cleaning", "Surface cleaning", "Removed surface grime with dry sponge.", time.Now().Add(-time.Hour), time.Now(), projectID)

	mustExec(t, pool, `INSERT INTO Assessment (id, projectId, assetId, "condition", description, recordedAt, updatedAt)
		SELECT ?, id, assetId, ?, ?, ?, ? FROM Project WHERE id = ?`,
		studiodb.NewID(), "good", "Grime removed, tears stable.", time.Now(), time.Now(), projectID)

	sections, err := BuildSuggestedOutline(ctx, pool, projectID)
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
	projectID := mustProject(t, pool, "A-0002")

	mustExec(t, pool, `INSERT INTO Assessment (id, projectId, assetId, "condition", description, recordedAt, updatedAt)
		SELECT ?, id, assetId, ?, ?, ?, ? FROM Project WHERE id = ?`,
		studiodb.NewID(), "good", "Only one state logged so far.", time.Now(), time.Now(), projectID)

	sections, err := BuildSuggestedOutline(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("BuildSuggestedOutline: %v", err)
	}
	if strings.Contains(sections.ConditionFindings, "Most recent:") {
		t.Errorf("expected no 'Most recent:' line when only one Assessment is logged, got %q", sections.ConditionFindings)
	}
	if !strings.Contains(sections.ConditionFindings, "On arrival: Only one state logged so far.") {
		t.Errorf("expected the single state as the 'On arrival' line, got %q", sections.ConditionFindings)
	}
}
