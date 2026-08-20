package clients

import (
	"context"
	"testing"

	"stuudio/internal/testutil"
)

func TestCreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := Create(ctx, pool, Input{Type: "individual", Name: "Eleanor Vance", Email: "eleanor@example.com"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil for a just-created client")
	}
	if got.Name != "Eleanor Vance" || got.Type != "individual" {
		t.Fatalf("got %+v, want Name=Eleanor Vance Type=individual", got)
	}
	if !got.Email.Valid || got.Email.String != "eleanor@example.com" {
		t.Fatalf("got Email=%+v, want valid eleanor@example.com", got.Email)
	}
}

func TestGetByIDMissing(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	got, err := GetByID(context.Background(), pool, "does-not-exist")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil for a missing client", got)
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := Create(ctx, pool, Input{Type: "individual", Name: "Original Name"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := Update(ctx, pool, id, Input{Type: "institution", Name: "Updated Name", City: "Tallinn"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Updated Name" || got.Type != "institution" {
		t.Fatalf("got %+v, want Name=Updated Name Type=institution", got)
	}
	if !got.City.Valid || got.City.String != "Tallinn" {
		t.Fatalf("got City=%+v, want valid Tallinn", got.City)
	}
}

// Update's name column falls back to the existing value when passed an empty string
// (COALESCE(NULLIF(?, ''), name)) — this guards against a blank submit accidentally wiping the
// one field every list/detail page uses as the client's display name.
func TestUpdateBlankNameKeepsExisting(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := Create(ctx, pool, Input{Type: "individual", Name: "Keep Me"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := Update(ctx, pool, id, Input{Type: "individual", Name: ""}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := GetByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Keep Me" {
		t.Fatalf("got Name=%q, want Name to survive a blank update", got.Name)
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	if _, err := Create(ctx, pool, Input{Type: "individual", Name: "A"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := Create(ctx, pool, Input{Type: "individual", Name: "B"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rows, err := List(ctx, pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	for _, r := range rows {
		if r.AssetCount != 0 {
			t.Fatalf("got AssetCount=%d for a client with no assets, want 0", r.AssetCount)
		}
	}
}
