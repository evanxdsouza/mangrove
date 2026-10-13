package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWithin(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "web", "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "web"), filepath.Join(base, "alias")); err != nil {
		t.Fatal(err)
	}

	for _, ok := range []string{"", ".", "web", "web/dist", "./web/../web/dist", "alias/dist"} {
		if _, err := resolveWithin(base, ok, "root_path"); err != nil {
			t.Errorf("resolveWithin(%q): unexpected error %v", ok, err)
		}
	}
	for _, bad := range []string{"..", "../x", "web/../../x", "/etc", "escape", "escape/sub", "missing"} {
		if p, err := resolveWithin(base, bad, "root_path"); err == nil {
			t.Errorf("resolveWithin(%q) = %q, want error", bad, p)
		}
	}
}

func TestRemoveEscapingSymlinks(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"leak.html": secret,
		"rel-leak":  "../../../../../../etc/passwd",
		"dangling":  filepath.Join(dir, "nope"),
		"chain":     "leak.html",
		"home.html": "index.html",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := removeEscapingSymlinks(dir); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"leak.html", "rel-leak", "dangling", "chain"} {
		if _, err := os.Lstat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s: expected removed, got %v", gone, err)
		}
	}
	for _, kept := range []string{"index.html", "home.html"} {
		if _, err := os.Lstat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s: expected kept, got %v", kept, err)
		}
	}
}
