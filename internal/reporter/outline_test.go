package reporter

import (
	"context"
	"encoding/json"
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

type outlineNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func TestBuildSuggestedOutlineUnknownProject(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	content, err := BuildSuggestedOutline(context.Background(), pool, "does-not-exist")
	if err != nil {
		t.Fatalf("BuildSuggestedOutline: %v", err)
	}
	var doc struct {
		Type    string        `json:"type"`
		Content []outlineNode `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Type != "doc" || len(doc.Content) != 0 {
		t.Errorf("expected an empty doc for an unknown project, got %+v", doc)
	}
}

func TestBuildSuggestedOutline(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	userID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO User (id, name, email, provider, role) VALUES (?, ?, ?, ?, ?)",
		userID, "Test Conservator", "conservator@test.local", "dev", "conservator")

	assetTypeID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO Classifier (id, type, code, title, sequence, updatedAt) VALUES (?, 'asset_type', 'painting', 'Painting', 0, ?)`,
		assetTypeID, time.Now())
	activityTypeID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO Classifier (id, type, code, title, sequence, updatedAt) VALUES (?, 'activity_type', 'cleaning', 'Surface Cleaning', 0, ?)`,
		activityTypeID, time.Now())

	clientID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Client (id, name, updatedAt) VALUES (?, ?, ?)", clientID, "Test Client", time.Now())

	assetID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Asset (id, clientId, referenceCode, assetTypeId, updatedAt) VALUES (?, ?, ?, ?, ?)",
		assetID, clientID, "A-0001", assetTypeID, time.Now())

	projectID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Project (id, assetId, title, updatedAt) VALUES (?, ?, ?, ?)",
		projectID, assetID, "Test Workflow", time.Now())

	intakeStateID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO AssetState (id, assetId, projectId, "condition", description, recordedAt) VALUES (?, ?, ?, ?, ?, ?)`,
		intakeStateID, assetID, projectID, "fair", "Surface grime, minor tears at edges.", time.Now().Add(-2*time.Hour))

	mustExec(t, pool, "INSERT INTO Activity (id, projectId, activityTypeId, userId, description, startedAt, durationMinutes) VALUES (?, ?, ?, ?, ?, ?, ?)",
		studiodb.NewID(), projectID, activityTypeID, userID, "Removed surface grime with dry sponge.", time.Now().Add(-time.Hour), 45)

	finalStateID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO AssetState (id, assetId, projectId, "condition", description, recordedAt) VALUES (?, ?, ?, ?, ?, ?)`,
		finalStateID, assetID, projectID, "good", "Grime removed, tears stable.", time.Now())

	content, err := BuildSuggestedOutline(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("BuildSuggestedOutline: %v", err)
	}

	var doc struct {
		Type    string        `json:"type"`
		Content []outlineNode `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatalf("unmarshal outline: %v", err)
	}

	var headings []string
	for _, n := range doc.Content {
		if n.Type == "heading" && len(n.Content) > 0 {
			headings = append(headings, n.Content[0].Text)
		}
	}

	want := []string{
		"Conservation Report — A-0001",
		"Condition on arrival",
		"Treatment performed",
		"Condition after treatment",
	}
	if len(headings) != len(want) {
		t.Fatalf("got headings %v, want %v", headings, want)
	}
	for i, h := range want {
		if headings[i] != h {
			t.Errorf("heading[%d] = %q, want %q", i, headings[i], h)
		}
	}
}

func TestBuildSuggestedOutlineSingleStateHasNoAfterSection(t *testing.T) {
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
	projectID := studiodb.NewID()
	mustExec(t, pool, "INSERT INTO Project (id, assetId, title, updatedAt) VALUES (?, ?, ?, ?)",
		projectID, assetID, "Solo State Workflow", time.Now())

	stateID := studiodb.NewID()
	mustExec(t, pool, `INSERT INTO AssetState (id, assetId, projectId, "condition", description, recordedAt) VALUES (?, ?, ?, ?, ?, ?)`,
		stateID, assetID, projectID, "good", "Only one state logged so far.", time.Now())

	content, err := BuildSuggestedOutline(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("BuildSuggestedOutline: %v", err)
	}
	var doc struct {
		Content []outlineNode `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, n := range doc.Content {
		if n.Type == "heading" && len(n.Content) > 0 && n.Content[0].Text == "Condition after treatment" {
			t.Errorf("expected no 'Condition after treatment' heading when only one AssetState is logged, got doc: %s", content)
		}
	}
}
