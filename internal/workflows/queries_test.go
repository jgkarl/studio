package workflows_test

import (
	"context"
	"database/sql"
	"testing"

	"stuudio/internal/assets"
	"stuudio/internal/clients"
	"stuudio/internal/settings"
	"stuudio/internal/testutil"
	"stuudio/internal/workflows"
)

func createFixtureAsset(t *testing.T, ctx context.Context, pool *sql.DB) string {
	t.Helper()
	clientID, err := clients.Create(ctx, pool, clients.Input{Type: "individual", Name: "Owner"})
	if err != nil {
		t.Fatalf("clients.Create: %v", err)
	}
	assetTypeID, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "workflows_test_type", Title: "Test Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}
	assetID, err := assets.Create(ctx, pool, clientID, assetTypeID, "WF-REF-1", assets.Input{Title: "Fixture Asset"})
	if err != nil {
		t.Fatalf("assets.Create: %v", err)
	}
	return assetID
}

func TestCreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	assetID := createFixtureAsset(t, ctx, pool)

	id, err := workflows.Create(ctx, pool, assetID, "Conservation Pass 1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := workflows.GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil for a just-created project")
	}
	if got.Title != "Conservation Pass 1" || got.AssetID != assetID {
		t.Fatalf("got %+v, want Title=Conservation Pass 1 AssetID=%s", got, assetID)
	}
}

func TestUpdateTitle(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	assetID := createFixtureAsset(t, ctx, pool)

	id, err := workflows.Create(ctx, pool, assetID, "Original Title")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := workflows.UpdateTitle(ctx, pool, id, "Renamed Title"); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}
	got, err := workflows.GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Renamed Title" {
		t.Fatalf("got Title=%q, want Renamed Title", got.Title)
	}
}

// Unlink soft-deletes: GetByID (which filters deletedAt IS NULL) must stop returning the row
// afterward, matching the "hidden from every query, row stays in the database" doc comment.
func TestUnlinkSoftDeletes(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	assetID := createFixtureAsset(t, ctx, pool)

	id, err := workflows.Create(ctx, pool, assetID, "To Be Unlinked")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := workflows.Unlink(ctx, pool, id); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	got, err := workflows.GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil for a soft-deleted project", got)
	}
}

func TestListJoinsAssetAndClient(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	assetID := createFixtureAsset(t, ctx, pool)

	if _, err := workflows.Create(ctx, pool, assetID, "Listed Project"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rows, err := workflows.List(ctx, pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Title != "Listed Project" {
		t.Fatalf("got Title=%q, want Listed Project", rows[0].Title)
	}
}
