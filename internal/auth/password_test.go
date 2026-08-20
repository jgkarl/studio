package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "correct-horse-battery-staple" {
		t.Fatalf("got hash=%q, want a non-empty, non-plaintext hash", hash)
	}
	if !VerifyPassword("correct-horse-battery-staple", hash) {
		t.Fatal("VerifyPassword rejected the correct password")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatal("VerifyPassword accepted an incorrect password")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of the same password matched — expected a per-call random salt")
	}
	if !VerifyPassword("same-password", a) || !VerifyPassword("same-password", b) {
		t.Fatal("both salted hashes must still verify the same plaintext password")
	}
}

func TestVerifyPasswordRejectsGarbageHash(t *testing.T) {
	if VerifyPassword("anything", "not-a-real-hash") {
		t.Fatal("VerifyPassword accepted a malformed stored hash")
	}
}
