package auth

import (
	"context"
	"testing"

	"stuudio/internal/testutil"
)

func TestCreateUserAndGetByIDAndEmail(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := CreateUser(ctx, pool, "Ada Lovelace", "ada@example.com", "hashed-password")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	byID, err := GetUserByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if byID == nil || byID.Name != "Ada Lovelace" || byID.Email != "ada@example.com" {
		t.Fatalf("got %+v, want Name=Ada Lovelace Email=ada@example.com", byID)
	}

	byEmail, err := GetUserByEmail(ctx, pool, "ada@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if byEmail == nil || byEmail.ID != id {
		t.Fatalf("got %+v, want ID=%s", byEmail, id)
	}
}

func TestGetUserByEmailMissing(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	got, err := GetUserByEmail(context.Background(), pool, "nobody@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil for an unknown email", got)
	}
}

func TestSetPasswordHash(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := CreateUser(ctx, pool, "User", "user@example.com", "old-hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := SetPasswordHash(ctx, pool, id, "new-hash"); err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}
	got, err := GetUserByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.PasswordHash.String != "new-hash" {
		t.Fatalf("got PasswordHash=%q, want new-hash", got.PasswordHash.String)
	}
}

func TestMarkEmailVerified(t *testing.T) {
	ctx := context.Background()
	pool := testutil.OpenTestDB(t)

	id, err := CreateUser(ctx, pool, "User", "user2@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	before, err := GetUserByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if before.EmailVerifiedAt.Valid {
		t.Fatal("newly created user already has emailVerifiedAt set")
	}

	if err := MarkEmailVerified(ctx, pool, id); err != nil {
		t.Fatalf("MarkEmailVerified: %v", err)
	}
	after, err := GetUserByID(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if !after.EmailVerifiedAt.Valid {
		t.Fatal("MarkEmailVerified did not set emailVerifiedAt")
	}
}
