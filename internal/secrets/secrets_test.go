package secrets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testBox(t *testing.T) *Box {
	t.Helper()
	key := make([]byte, keySize)
	for i := range key {
		key[i] = byte(i)
	}
	box, err := NewBox(key)
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	return box
}

func TestSealOpenRoundTrip(t *testing.T) {
	box := testBox(t)
	plaintext := []byte("ghp_supersecrettoken")
	aad := []byte("github_pats:1")

	ciphertext, nonce, err := box.Seal(aad, plaintext)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext must not equal plaintext")
	}

	got, err := box.Open(aad, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestOpenFailsOnWrongAAD(t *testing.T) {
	box := testBox(t)
	ciphertext, nonce, err := box.Seal([]byte("service:1:API_KEY"), []byte("secret-value"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Simulates a ciphertext blob copied to a different row.
	if _, err := box.Open([]byte("service:2:API_KEY"), ciphertext, nonce); err == nil {
		t.Fatal("expected decryption to fail under mismatched AAD, got nil error")
	}
}

func TestOpenFailsOnTamperedCiphertext(t *testing.T) {
	box := testBox(t)
	aad := []byte("service:1:API_KEY")
	ciphertext, nonce, err := box.Seal(aad, []byte("secret-value"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	ciphertext[0] ^= 0xFF

	if _, err := box.Open(aad, ciphertext, nonce); err == nil {
		t.Fatal("expected decryption to fail on tampered ciphertext, got nil error")
	}
}

func TestLoadOrCreateMasterKeyGeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "master.key")

	key1, err := LoadOrCreateMasterKey(path)
	if err != nil {
		t.Fatalf("first LoadOrCreateMasterKey: %v", err)
	}
	if len(key1) != keySize {
		t.Fatalf("expected %d-byte key, got %d", keySize, len(key1))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected key file mode 0600, got %o", perm)
	}

	key2, err := LoadOrCreateMasterKey(path)
	if err != nil {
		t.Fatalf("second LoadOrCreateMasterKey: %v", err)
	}
	if !bytes.Equal(key1, key2) {
		t.Error("expected second call to load the same persisted key, got a different one")
	}
}

func TestLoadOrCreateMasterKeyRejectsWrongLength(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "master.key")
	if err := os.WriteFile(path, []byte("too-short"), 0600); err != nil {
		t.Fatalf("write bad key file: %v", err)
	}

	if _, err := LoadOrCreateMasterKey(path); err == nil {
		t.Fatal("expected error loading a wrong-length key file, got nil")
	}
}
