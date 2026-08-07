package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignatureAccepts(t *testing.T) {
	secret := []byte("my-webhook-secret")
	body := []byte(`{"ref":"refs/heads/main"}`)
	header := sign(secret, body)

	if !VerifySignature(secret, body, header) {
		t.Error("expected valid signature to be accepted")
	}
}

func TestVerifySignatureRejectsWrongSecret(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	header := sign([]byte("correct-secret"), body)

	if VerifySignature([]byte("wrong-secret"), body, header) {
		t.Error("expected signature computed with a different secret to be rejected")
	}
}

func TestVerifySignatureRejectsTamperedBody(t *testing.T) {
	secret := []byte("my-webhook-secret")
	header := sign(secret, []byte(`{"ref":"refs/heads/main"}`))

	if VerifySignature(secret, []byte(`{"ref":"refs/heads/evil"}`), header) {
		t.Error("expected signature to be rejected when body doesn't match what was signed")
	}
}

func TestVerifySignatureRejectsMalformedHeader(t *testing.T) {
	secret := []byte("my-webhook-secret")
	body := []byte(`{}`)

	cases := []string{"", "not-even-close", "sha1=abcd", "sha256=not-hex-zz"}
	for _, h := range cases {
		if VerifySignature(secret, body, h) {
			t.Errorf("expected header %q to be rejected", h)
		}
	}
}

func TestBranchFromRef(t *testing.T) {
	cases := map[string]string{
		"refs/heads/main":       "main",
		"refs/heads/feat/thing": "feat/thing",
		"refs/tags/v1.0.0":      "",
		"garbage":               "",
	}
	for ref, want := range cases {
		if got := BranchFromRef(ref); got != want {
			t.Errorf("BranchFromRef(%q) = %q, want %q", ref, got, want)
		}
	}
}
