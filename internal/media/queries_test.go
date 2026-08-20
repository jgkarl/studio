package media

import (
	"context"
	"testing"

	"stuudio/internal/auth"
	"stuudio/internal/i18n"
	"stuudio/internal/testutil"
)

func createFixtureUser(t *testing.T, ctx context.Context, svc *Service) string {
	t.Helper()
	id, err := auth.CreateUser(ctx, svc.Pool, "Uploader", "uploader@example.com", "hash")
	if err != nil {
		t.Fatalf("auth.CreateUser: %v", err)
	}
	return id
}

func TestUpdateDescription(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	storage, err := NewLocalDiskAdapter(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDiskAdapter: %v", err)
	}
	svc := &Service{Pool: pool, Storage: storage}
	userID := createFixtureUser(t, ctx, svc)
	m, err := svc.UploadMedia(ctx, []byte("not a real image"), "application/octet-stream", userID)
	if err != nil {
		t.Fatalf("UploadMedia: %v", err)
	}

	if err := UpdateDescription(ctx, pool, m.ID, "A whole-image note"); err != nil {
		t.Fatalf("UpdateDescription: %v", err)
	}
	got, err := GetByID(ctx, pool, m.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.Description.Valid || got.Description.String != "A whole-image note" {
		t.Fatalf("got Description=%+v, want valid 'A whole-image note'", got.Description)
	}

	// Saving an empty string clears the note back to NULL, not an empty string — matches every
	// other nullIfEmpty-guarded text column in this app.
	if err := UpdateDescription(ctx, pool, m.ID, ""); err != nil {
		t.Fatalf("UpdateDescription (clear): %v", err)
	}
	got, err = GetByID(ctx, pool, m.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Description.Valid {
		t.Fatalf("got Description=%+v, want NULL after clearing with an empty string", got.Description)
	}
}

// RenderAnnotatedImage returns (nil, "", nil) for a media item with no drawn regions — callers
// (the "Download annotated" route, HTML/PDF export) rely on this to fall back to the plain file.
func TestRenderAnnotatedImageNoRegions(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	storage, err := NewLocalDiskAdapter(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDiskAdapter: %v", err)
	}
	svc := &Service{Pool: pool, Storage: storage}
	userID := createFixtureUser(t, ctx, svc)
	m, err := svc.UploadMedia(ctx, []byte("not a real image"), "image/png", userID)
	if err != nil {
		t.Fatalf("UploadMedia: %v", err)
	}

	png, mimeType, err := svc.RenderAnnotatedImage(ctx, m.ID, i18n.LocaleEN)
	if err != nil {
		t.Fatalf("RenderAnnotatedImage: %v", err)
	}
	if png != nil || mimeType != "" {
		t.Fatalf("got png=%v mimeType=%q, want nil/\"\" for a media item with no regions", png != nil, mimeType)
	}
}

func TestRenderAnnotatedImageMissingMedia(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)
	storage, err := NewLocalDiskAdapter(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDiskAdapter: %v", err)
	}
	svc := &Service{Pool: pool, Storage: storage}

	png, mimeType, err := svc.RenderAnnotatedImage(ctx, "does-not-exist", i18n.LocaleEN)
	if err != nil {
		t.Fatalf("RenderAnnotatedImage: %v", err)
	}
	if png != nil || mimeType != "" {
		t.Fatalf("got png=%v mimeType=%q, want nil/\"\" for a missing media id", png != nil, mimeType)
	}
}
