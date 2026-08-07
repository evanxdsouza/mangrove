// Package webhook implements GitHub webhook signature verification,
// independent of the HTTP handler that uses it (see internal/api/webhook.go)
// so the crypto-sensitive comparison logic can be unit tested directly.
package webhook

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// GenerateToken returns a random opaque token suitable for both the
// webhook URL path segment and the HMAC secret -- 32 random bytes,
// base64url-encoded so it's URL-safe without escaping.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// VerifySignature checks a GitHub X-Hub-Signature-256 header ("sha256=<hex>")
// against the raw request body, in constant time. secret must be the
// decrypted webhook secret for the specific repo the delivery claims to be
// for -- verification must always happen before any side effect.
func VerifySignature(secret []byte, body []byte, header string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	sigHex := strings.TrimPrefix(header, prefix)
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}

// BranchFromRef extracts "main" from "refs/heads/main"; returns "" for
// non-branch refs (tags, etc.) so callers can ignore them.
func BranchFromRef(ref string) string {
	const prefix = "refs/heads/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	return strings.TrimPrefix(ref, prefix)
}
