package treatments_test

import (
	"context"
	"database/sql"
	"testing"

	"studio/internal/assets"
	"studio/internal/clients"
	"studio/internal/settings"
	"studio/internal/testutil"
	"studio/internal/treatments"
	"studio/internal/workflows"
)

func createFixtureProject(t *testing.T, ctx context.Context, pool *sql.DB) string {
	t.Helper()
	clientID, err := clients.Create(ctx, pool, clients.Input{Type: "individual", Name: "Owner"})
	if err != nil {
		t.Fatalf("clients.Create: %v", err)
	}
	assetTypeID, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "treatments_test_type", Title: "Test Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}
	assetID, err := assets.Create(ctx, pool, clientID, assetTypeID, "TR-REF-1", assets.Input{Title: "Fixture Asset"})
	if err != nil {
		t.Fatalf("assets.Create: %v", err)
	}
	projectID, err := workflows.Create(ctx, pool, assetID, "Fixture Project")
	if err != nil {
		t.Fatalf("workflows.Create: %v", err)
	}
	return projectID
}

func TestCreateResolvesAssetFromProject(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID := createFixtureProject(t, ctx, pool)

	if _, err := treatments.Create(ctx, pool, treatments.Input{ProjectID: projectID, Method: "cleaning", Title: "Surface Clean"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rows, err := treatments.ListByProject(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Method != "cleaning" || rows[0].Title != "Surface Clean" || rows[0].ProjectID != projectID {
		t.Fatalf("got %+v, unexpected values", rows[0])
	}
}

// Create defaults PerformedAt to "now" when the zero value is passed — the standalone
// new-treatment form doesn't always collect a date.
func TestCreateDefaultsPerformedAt(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID := createFixtureProject(t, ctx, pool)

	id, err := treatments.Create(ctx, pool, treatments.Input{ProjectID: projectID, Method: "consolidation", Title: "Consolidate"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rows, err := treatments.ListByProject(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	var found bool
	for _, r := range rows {
		if r.ID == id {
			found = true
			if r.PerformedAt.IsZero() {
				t.Fatalf("PerformedAt was left zero, want it defaulted to now")
			}
		}
	}
	if !found {
		t.Fatalf("created treatment %s not found in ListByProject", id)
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID := createFixtureProject(t, ctx, pool)

	id, err := treatments.Create(ctx, pool, treatments.Input{ProjectID: projectID, Method: "cleaning", Title: "Original"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := treatments.Update(ctx, pool, id, treatments.UpdateInput{Method: "repair", Title: "Updated", Notes: "done"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	rows, err := treatments.ListByProject(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if rows[0].Method != "repair" || rows[0].Title != "Updated" {
		t.Fatalf("got %+v, want Method=repair Title=Updated", rows[0])
	}
}
