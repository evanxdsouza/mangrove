package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/evanxdsouza/mangrove/internal/store"
)

const (
	SessionCookieName = "mangrove_session"
	SessionTTL        = 30 * 24 * time.Hour
	tokenBytes        = 32
)

// NewSessionToken generates an opaque random token. Only its SHA-256 hash
// is ever persisted (in sessions.token_hash) -- the raw value lives only in
// the client's cookie, so a leaked database can't be used to forge sessions.
func NewSessionToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession issues and persists a new session for userID, returning the
// raw token to set as a cookie value.
func CreateSession(ctx context.Context, st *store.Store, userID int64, ip, userAgent string) (string, error) {
	token, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(SessionTTL)
	if err := st.CreateSession(ctx, userID, hashToken(token), expiresAt, ip, userAgent); err != nil {
		return "", fmt.Errorf("persist session: %w", err)
	}
	return token, nil
}

// ValidateSession looks up the user a raw cookie token belongs to, or
// store.ErrNotFound if it's missing/expired.
func ValidateSession(ctx context.Context, st *store.Store, token string) (int64, error) {
	userID, expiresAt, err := st.GetSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		return 0, err
	}
	if time.Now().After(expiresAt) {
		return 0, store.ErrNotFound
	}
	return userID, nil
}

func DeleteSession(ctx context.Context, st *store.Store, token string) error {
	return st.DeleteSessionByTokenHash(ctx, hashToken(token))
}
