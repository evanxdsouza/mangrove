package gateauth

import (
	"testing"
	"time"

	"github.com/evanxdsouza/mangrove/internal/secrets"
)

func testSigner(t *testing.T) *Signer {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	box, err := secrets.NewBox(key)
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	return NewSigner(box)
}

func TestCookieRoundTrip(t *testing.T) {
	s := testSigner(t)
	token, err := s.IssueCookie(42)
	if err != nil {
		t.Fatalf("IssueCookie: %v", err)
	}
	if !s.VerifyCookie(token, 42) {
		t.Fatal("VerifyCookie: expected valid for the deployment it was issued for")
	}
	if s.VerifyCookie(token, 43) {
		t.Fatal("VerifyCookie: expected invalid for a different deployment")
	}
	if s.VerifyCookie("not-a-real-token", 42) {
		t.Fatal("VerifyCookie: expected invalid for garbage input")
	}
}

func TestHandoffRoundTrip(t *testing.T) {
	s := testSigner(t)
	token, err := s.IssueHandoff(7, "https://myapp.example.com/dashboard?x=1")
	if err != nil {
		t.Fatalf("IssueHandoff: %v", err)
	}
	id, returnURL, err := s.OpenHandoff(token)
	if err != nil {
		t.Fatalf("OpenHandoff: %v", err)
	}
	if id != 7 {
		t.Errorf("deployment id = %d, want 7", id)
	}
	if returnURL != "https://myapp.example.com/dashboard?x=1" {
		t.Errorf("return url = %q, want the original", returnURL)
	}
}

func TestCallbackRoundTrip(t *testing.T) {
	s := testSigner(t)
	token, err := s.IssueCallback(9, "https://myapp.example.com/foo")
	if err != nil {
		t.Fatalf("IssueCallback: %v", err)
	}
	returnURL, err := s.OpenCallback(token, 9)
	if err != nil {
		t.Fatalf("OpenCallback: %v", err)
	}
	if returnURL != "https://myapp.example.com/foo" {
		t.Errorf("return url = %q, want the original", returnURL)
	}
	if _, err := s.OpenCallback(token, 10); err == nil {
		t.Fatal("OpenCallback: expected error for a callback token replayed against a different deployment")
	}
}

func TestOpenRejectsWrongAAD(t *testing.T) {
	s := testSigner(t)
	token, err := s.seal("aad-a", struct{ X int }{X: 1}, time.Minute)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	var out struct{ X int }
	if err := s.open(token, "aad-b", &out); err == nil {
		t.Fatal("expected error opening a token with the wrong aad")
	}
}

func TestOpenRejectsExpiredToken(t *testing.T) {
	s := testSigner(t)
	token, err := s.seal("aad", struct{ X int }{X: 1}, -time.Minute)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	var out struct{ X int }
	if err := s.open(token, "aad", &out); err == nil {
		t.Fatal("expected error opening an already-expired token")
	}
}

func TestOpenRejectsTamperedToken(t *testing.T) {
	s := testSigner(t)
	token, err := s.IssueCookie(1)
	if err != nil {
		t.Fatalf("IssueCookie: %v", err)
	}
	tampered := token[:len(token)-1] + "x"
	if s.VerifyCookie(tampered, 1) {
		t.Fatal("expected a tampered token to fail verification")
	}
}
