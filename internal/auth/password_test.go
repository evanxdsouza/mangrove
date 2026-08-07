package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("expected correct password to verify")
	}

	ok, err = VerifyPassword("wrong password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword (wrong password): %v", err)
	}
	if ok {
		t.Error("expected incorrect password to fail verification")
	}
}

func TestHashPasswordProducesUniqueSalts(t *testing.T) {
	h1, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	h2, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if h1 == h2 {
		t.Error("expected two hashes of the same password to differ (random salt)")
	}
}

func TestVerifyPasswordRejectsGarbageHash(t *testing.T) {
	if _, err := VerifyPassword("anything", "not-a-real-hash"); err == nil {
		t.Error("expected an error for a malformed hash string")
	}
}
