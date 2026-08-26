package settings_test

import (
	"context"
	"strings"
	"testing"

	"studio/internal/assets"
	"studio/internal/clients"
	"studio/internal/settings"
	"studio/internal/testutil"
)

func TestCreateAndGetClassifierByID(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "test_type", Title: "Test Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}

	got, err := settings.GetClassifierByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetClassifierByID: %v", err)
	}
	if got == nil || got.Title != "Test Type" || got.Code != "test_type" {
		t.Fatalf("got %+v, want Title=Test Type Code=test_type", got)
	}
}

// GetClassifiers/GetAllClassifiers must agree on a freshly created, active-by-default classifier
// — both list functions filter by isActive = 1 in different ways (WHERE clause vs. none), so a
// row that's active by default should show up in both.
func TestGetClassifiersAndGetAllClassifiersAgreeWhenActive(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "queries_test_type", Title: "Queries Test Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}

	active, err := settings.GetClassifiers(ctx, pool, settings.ClassifierAssetType)
	if err != nil {
		t.Fatalf("GetClassifiers: %v", err)
	}
	all, err := settings.GetAllClassifiers(ctx, pool, settings.ClassifierAssetType)
	if err != nil {
		t.Fatalf("GetAllClassifiers: %v", err)
	}
	if len(active) != len(all) {
		t.Fatalf("got %d active vs %d total classifiers, want equal (nothing inactive was created)", len(active), len(all))
	}
	found := false
	for _, c := range active {
		if c.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("newly created classifier %s not present in GetClassifiers", id)
	}
}

func TestDeleteClassifierUnused(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "unused", Title: "Unused", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}
	if err := settings.DeleteClassifier(ctx, pool, id); err != nil {
		t.Fatalf("DeleteClassifier: %v", err)
	}
	got, err := settings.GetClassifierByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetClassifierByID: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil after delete", got)
	}
}

// DeleteClassifier blocks deleting an asset_type still referenced by an Asset row — deactivating
// is the correct path instead, so the caller must not be able to orphan the FK.
func TestDeleteClassifierBlockedWhenInUse(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	assetTypeID, err := settings.CreateClassifier(ctx, pool, settings.ClassifierInput{
		Type: settings.ClassifierAssetType, Code: "queries_test_in_use", Title: "In Use Type", Sequence: 1, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateClassifier: %v", err)
	}
	clientID, err := clients.Create(ctx, pool, clients.Input{Type: "individual", Name: "Owner"})
	if err != nil {
		t.Fatalf("clients.Create: %v", err)
	}
	if _, err := assets.Create(ctx, pool, clientID, assetTypeID, "REF-001", assets.Input{Title: "A Painting"}); err != nil {
		t.Fatalf("assets.Create: %v", err)
	}

	err = settings.DeleteClassifier(ctx, pool, assetTypeID)
	if err == nil {
		t.Fatal("expected DeleteClassifier to fail for a classifier still in use, got nil error")
	}
	if !strings.Contains(err.Error(), "still used by") {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := settings.GetClassifierByID(ctx, pool, assetTypeID)
	if err != nil {
		t.Fatalf("GetClassifierByID: %v", err)
	}
	if got == nil {
		t.Fatal("classifier was deleted despite being in use")
	}
}
