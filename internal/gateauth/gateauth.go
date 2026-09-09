// Package gateauth issues and verifies the short-lived signed tokens that
// drive a password-protected deployment's gate page (see
// internal/api/gate.go and docs/protected-deployments.md): the long-lived
// cookie that marks a visitor as let-through, and the two hops of the
// cross-domain "continue with your Mangrove account" handoff. Every token
// is a self-contained, tamper-evident blob sealed under the same master
// key already used for secrets at rest (internal/secrets) -- there is no
// new database table backing any of this.
package gateauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/evanxdsouza/mangrove/internal/secrets"
)

// HeaderDeploymentID is the request header Caddy sets (forcibly, overwriting
// any client-supplied value -- see internal/proxy/caddy.go's gateHandler)
// naming which deployment's gate a loopback-only request is for.
const HeaderDeploymentID = "X-Mangrove-Gate-Deployment"

// CookieName is the cookie a visitor holds once they've passed a
// deployment's gate (account sign-in or password).
const CookieName = "mangrove_gate"

const (
	cookieTTL   = 24 * time.Hour
	handoffTTL  = 5 * time.Minute
	callbackTTL = 2 * time.Minute
)

// Signer seals and opens gate tokens under the shared master key.
type Signer struct{ box *secrets.Box }

func NewSigner(box *secrets.Box) *Signer {
	return &Signer{box: box}
}

// envelope is the common shape every sealed token carries: an expiry plus
// whatever payload the specific token kind needs.
type envelope struct {
	ExpiresAt int64           `json:"exp"` // unix seconds
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// seal JSON-marshals payload into an envelope with the given TTL, encrypts
// it under aad, and returns a single opaque, URL-safe string.
func (s *Signer) seal(aad string, payload any, ttl time.Duration) (string, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	env := envelope{ExpiresAt: time.Now().Add(ttl).Unix(), Payload: payloadJSON}
	plaintext, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("marshal envelope: %w", err)
	}
	ciphertext, nonce, err := s.box.Seal([]byte(aad), plaintext)
	if err != nil {
		return "", fmt.Errorf("seal: %w", err)
	}
	// nonce is fixed-size (AEAD.NonceSize()) so this is unambiguous to split
	// back apart on open without a length prefix.
	blob := append(append([]byte{}, nonce...), ciphertext...)
	return base64.RawURLEncoding.EncodeToString(blob), nil
}

// open reverses seal: verifies aad and expiry, and decodes the payload into
// out (a pointer). Returns an error for a malformed, tampered, wrong-aad, or
// expired token -- callers should treat all of these identically ("this
// link/cookie is invalid or has expired"), never distinguish them to the
// visitor.
func (s *Signer) open(token, aad string, out any) error {
	blob, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("decode token: %w", err)
	}
	// GCM nonces are 12 bytes for the cipher.NewGCM default used by
	// secrets.NewBox; a token shorter than that plus some ciphertext/tag is
	// definitely not one of ours.
	const nonceSize = 12
	if len(blob) <= nonceSize {
		return fmt.Errorf("token too short")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	plaintext, err := s.box.Open([]byte(aad), ciphertext, nonce)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	var env envelope
	if err := json.Unmarshal(plaintext, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if time.Now().Unix() > env.ExpiresAt {
		return fmt.Errorf("token expired")
	}
	if out != nil && len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, out); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
	}
	return nil
}

func cookieAAD(deploymentID int64) string   { return fmt.Sprintf("gate-cookie:%d", deploymentID) }
func callbackAAD(deploymentID int64) string { return fmt.Sprintf("gate-callback:%d", deploymentID) }

// handoffAAD is fixed (not deployment-scoped): OpenHandoff's caller (the
// dashboard-domain /gate-auth handler) doesn't know which deployment a
// token is for until after decrypting it -- the deployment id travels
// inside the payload instead, same as ReturnURL.
const handoffAAD = "gate-handoff"

// IssueCookie/VerifyCookie: the long-lived "this visitor passed the gate"
// token, set as the mangrove_gate cookie once account sign-in or the
// password check succeeds.
func (s *Signer) IssueCookie(deploymentID int64) (string, error) {
	return s.seal(cookieAAD(deploymentID), struct{}{}, cookieTTL)
}

func (s *Signer) VerifyCookie(token string, deploymentID int64) bool {
	return s.open(token, cookieAAD(deploymentID), nil) == nil
}

type handoffPayload struct {
	DeploymentID int64  `json:"deployment_id"`
	ReturnURL    string `json:"return_url"`
}

// IssueHandoff/OpenHandoff: the first hop of "continue with your Mangrove
// account" -- minted on the protected deployment's own domain, redeemed on
// the dashboard's domain (which is why the deployment id has to ride along
// inside the token rather than the aad, see handoffAAD above).
func (s *Signer) IssueHandoff(deploymentID int64, returnURL string) (string, error) {
	return s.seal(handoffAAD, handoffPayload{DeploymentID: deploymentID, ReturnURL: returnURL}, handoffTTL)
}

func (s *Signer) OpenHandoff(token string) (deploymentID int64, returnURL string, err error) {
	var p handoffPayload
	if err := s.open(token, handoffAAD, &p); err != nil {
		return 0, "", err
	}
	return p.DeploymentID, p.ReturnURL, nil
}

type callbackPayload struct {
	ReturnURL string `json:"return_url"`
}

// IssueCallback/OpenCallback: the second hop -- minted by the dashboard once
// it's confirmed the visitor is signed in, redeemed back on the deployment's
// own domain to actually set the gate cookie.
func (s *Signer) IssueCallback(deploymentID int64, returnURL string) (string, error) {
	return s.seal(callbackAAD(deploymentID), callbackPayload{ReturnURL: returnURL}, callbackTTL)
}

func (s *Signer) OpenCallback(token string, deploymentID int64) (returnURL string, err error) {
	var p callbackPayload
	if err := s.open(token, callbackAAD(deploymentID), &p); err != nil {
		return "", err
	}
	return p.ReturnURL, nil
}
