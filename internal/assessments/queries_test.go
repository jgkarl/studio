package assessments_test

import (
	"context"
	"database/sql"
	"testing"

	"stuudio/internal/assessments"
	"stuudio/internal/assets"
	"stuudio/internal/clients"
	"stuudio/internal/settings"
	"stuudio/internal/testutil"
	"stuudio/internal/workflows"
)

func createFixtureProject(t *testing.T, ctx context.Context, pool *sql.DB) (projectID, assetID string) {
	t.Helper()
	clientID, err := clients.Create(ctx, pool, clients.Input{Type: "individual", Name: "Owner"})
	if err != nil {
		t.Fatalf("clients.Create: %v", err)
	}
	assetTypeID, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "assessments_test_type", Title: "Test Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}
	assetID, err = assets.Create(ctx, pool, clientID, assetTypeID, "AS-REF-1", assets.Input{Title: "Fixture Asset"})
	if err != nil {
		t.Fatalf("assets.Create: %v", err)
	}
	projectID, err = workflows.Create(ctx, pool, assetID, "Fixture Project")
	if err != nil {
		t.Fatalf("workflows.Create: %v", err)
	}
	return projectID, assetID
}

// Create resolves AssetID server-side from the Project (see queries.go's doc comment) and also
// updates Asset.currentStateId to "fixate" the new assessment as the asset's current state — both
// side effects are worth pinning down with a test.
func TestCreateResolvesAssetAndFixatesCurrentState(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID, assetID := createFixtureProject(t, ctx, pool)

	id, err := assessments.Create(ctx, pool, assessments.Input{ProjectID: projectID, Condition: "good", Description: "Looks fine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	asset, err := assets.GetByID(ctx, pool, assetID)
	if err != nil {
		t.Fatalf("assets.GetByID: %v", err)
	}
	if !asset.CurrentStateID.Valid || asset.CurrentStateID.String != id {
		t.Fatalf("got Asset.CurrentStateID=%+v, want valid %s", asset.CurrentStateID, id)
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	projectID, _ := createFixtureProject(t, ctx, pool)

	id, err := assessments.Create(ctx, pool, assessments.Input{ProjectID: projectID, Condition: "fair", Description: "Original"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := assessments.Update(ctx, pool, id, assessments.UpdateInput{Condition: "poor", Description: "Updated"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	rows, err := assessments.ListByProject(ctx, pool, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Condition != "poor" || rows[0].Description != "Updated" {
		t.Fatalf("got %+v, want Condition=poor Description=Updated", rows[0])
	}
	if rows[0].ProjectID != projectID {
		t.Fatalf("got ProjectID=%q, want %q", rows[0].ProjectID, projectID)
	}
}
