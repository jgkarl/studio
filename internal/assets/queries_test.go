package assets_test

import (
	"context"
	"testing"

	"studio/internal/assets"
	"studio/internal/clients"
	"studio/internal/settings"
	"studio/internal/testutil"
)

func TestCreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	clientID, err := clients.Create(ctx, pool, clients.Input{Type: "individual", Name: "Owner"})
	if err != nil {
		t.Fatalf("clients.Create: %v", err)
	}
	assetTypeID, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "assets_test_type", Title: "Test Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}

	id, err := assets.Create(ctx, pool, clientID, assetTypeID, "REF-100", assets.Input{Title: "A Painting", Artist: "Unknown"})
	if err != nil {
		t.Fatalf("assets.Create: %v", err)
	}

	got, err := assets.GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil for a just-created asset")
	}
	if !got.Title.Valid || got.Title.String != "A Painting" {
		t.Fatalf("got Title=%+v, want valid 'A Painting'", got.Title)
	}
	if got.ReferenceCode != "REF-100" || got.ClientID != clientID || got.AssetTypeID != assetTypeID {
		t.Fatalf("got %+v, unexpected FK/reference values", got)
	}
}

func TestListIncludesJoinedClientAndType(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	clientID, err := clients.Create(ctx, pool, clients.Input{Type: "individual", Name: "Jane Owner"})
	if err != nil {
		t.Fatalf("clients.Create: %v", err)
	}
	assetTypeID, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "assets_test_list_type", Title: "Listed Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}
	if _, err := assets.Create(ctx, pool, clientID, assetTypeID, "REF-200", assets.Input{Title: "Listed Asset"}); err != nil {
		t.Fatalf("assets.Create: %v", err)
	}

	rows, err := assets.List(ctx, pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
}

func TestListProjectsForAssetEmpty(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	clientID, err := clients.Create(ctx, pool, clients.Input{Type: "individual", Name: "Owner"})
	if err != nil {
		t.Fatalf("clients.Create: %v", err)
	}
	assetTypeID, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "assets_test_empty_type", Title: "Empty Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}
	id, err := assets.Create(ctx, pool, clientID, assetTypeID, "REF-300", assets.Input{Title: "No Projects Yet"})
	if err != nil {
		t.Fatalf("assets.Create: %v", err)
	}

	projects, err := assets.ListProjectsForAsset(ctx, pool, id)
	if err != nil {
		t.Fatalf("ListProjectsForAsset: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("got %d projects for a brand-new asset, want 0", len(projects))
	}
}
