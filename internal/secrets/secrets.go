// Package secrets encrypts values at rest (PATs, webhook secrets, env vars
// marked as secret) with AES-256-GCM under a single master key.
//
// The master key file is the primary mechanism for this deployment target:
// headless Ubuntu servers generally have no Secret Service/libsecret daemon,
// so an OS-keyring is not relied on. Losing the key file makes every secret
// unrecoverable — back it up alongside the SQLite file.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const keySize = 32 // AES-256

// Box seals and opens values under a single master key.
type Box struct {
	aead cipher.AEAD
}

// NewBox constructs a Box from a raw 32-byte key.
func NewBox(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("master key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new GCM: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext, returning ciphertext and the nonce used.
// aad (additional authenticated data) should bind the ciphertext to the row
// it belongs to (e.g. "service:42:key_name") so a copied ciphertext blob
// cannot be silently reattached to a different row.
func (b *Box) Seal(aad, plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext = b.aead.Seal(nil, nonce, plaintext, aad)
	return ciphertext, nonce, nil
}

// Open decrypts ciphertext produced by Seal. aad must match exactly what was
// passed to Seal, or decryption fails (returns an error, never garbage data).
func (b *Box) Open(aad, ciphertext, nonce []byte) ([]byte, error) {
	plaintext, err := b.aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// LoadOrCreateMasterKey reads a 32-byte key from path, generating and
// persisting a new random one (mode 0600) if the file does not exist.
func LoadOrCreateMasterKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) != keySize {
			return nil, fmt.Errorf("master key file %s has wrong length: expected %d bytes, got %d", path, keySize, len(data))
		}
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read master key: %w", err)
	}

	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create key dir: %w", err)
	}
	// Write to a temp file and rename so a crash mid-write can never leave a
	// truncated/partial key on disk.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, key, 0600); err != nil {
		return nil, fmt.Errorf("write master key: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, fmt.Errorf("finalize master key: %w", err)
	}
	return key, nil
}
