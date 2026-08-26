package reporter_test

import (
	"context"
	"database/sql"
	"testing"

	"studio/internal/assets"
	"studio/internal/auth"
	"studio/internal/clients"
	"studio/internal/reporter"
	"studio/internal/settings"
	"studio/internal/testutil"
	"studio/internal/workflows"
)

func createFixtureProjectAndAuthor(t *testing.T, ctx context.Context, pool *sql.DB) (projectID, authorID string) {
	t.Helper()
	clientID, err := clients.Create(ctx, pool, clients.Input{Type: "individual", Name: "Owner"})
	if err != nil {
		t.Fatalf("clients.Create: %v", err)
	}
	assetTypeID, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "reporter_test_type", Title: "Test Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}
	assetID, err := assets.Create(ctx, pool, clientID, assetTypeID, "RP-REF-1", assets.Input{Title: "Fixture Asset"})
	if err != nil {
		t.Fatalf("assets.Create: %v", err)
	}
	projectID, err = workflows.Create(ctx, pool, assetID, "Fixture Project")
	if err != nil {
		t.Fatalf("workflows.Create: %v", err)
	}
	authorID, err = auth.CreateUser(ctx, pool, "Report Author", "author@example.com", "hash")
	if err != nil {
		t.Fatalf("auth.CreateUser: %v", err)
	}
	return projectID, authorID
}

func TestCreateStartsAsDraft(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID, authorID := createFixtureProjectAndAuthor(t, ctx, pool)

	id, err := reporter.Create(ctx, pool, projectID, "Condition Report", authorID, reporter.Sections{
		ConditionFindings: "Stable", TreatmentPerformed: "None yet",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := reporter.GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil for a just-created report")
	}
	if got.Status != "draft" {
		t.Fatalf("got Status=%q, want draft", got.Status)
	}
	if !got.AuthorID.Valid || got.AuthorID.String != authorID {
		t.Fatalf("got AuthorID=%+v, want valid %s", got.AuthorID, authorID)
	}
	if !got.ConditionFindings.Valid || got.ConditionFindings.String != "Stable" {
		t.Fatalf("got ConditionFindings=%+v, want valid Stable", got.ConditionFindings)
	}
}

func TestUpdateSections(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID, authorID := createFixtureProjectAndAuthor(t, ctx, pool)

	id, err := reporter.Create(ctx, pool, projectID, "Report", authorID, reporter.Sections{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := reporter.UpdateSections(ctx, pool, id, reporter.SectionsInput{
		Summary: "New summary", ConditionFindings: "Updated findings",
	}); err != nil {
		t.Fatalf("UpdateSections: %v", err)
	}

	got, err := reporter.GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.Summary.Valid || got.Summary.String != "New summary" {
		t.Fatalf("got Summary=%+v, want valid 'New summary'", got.Summary)
	}
	if !got.ConditionFindings.Valid || got.ConditionFindings.String != "Updated findings" {
		t.Fatalf("got ConditionFindings=%+v, want valid 'Updated findings'", got.ConditionFindings)
	}
}

func TestListByProject(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID, authorID := createFixtureProjectAndAuthor(t, ctx, pool)

	if _, err := reporter.Create(ctx, pool, projectID, "Report A", authorID, reporter.Sections{}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rows, err := reporter.ListByProject(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].ProjectID != projectID || rows[0].Status != "draft" {
		t.Fatalf("got %+v, unexpected values", rows[0])
	}
}
