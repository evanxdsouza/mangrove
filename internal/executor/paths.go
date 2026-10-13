package executor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// resolveWithin joins rel (a repo-relative path from a deployment's config,
// e.g. RootPath or StaticOutputDir) onto base and refuses anything that
// lands outside base: "..", an absolute path, or a symlink committed to the
// repo that points elsewhere on the host. Without this, a deployment
// configured with static_output_dir "../../../root/mangrove/data" copied
// the host's master key into a publicly served static site.
//
// Empty and "." resolve to base itself.
func resolveWithin(base, rel, what string) (string, error) {
	if rel == "" || rel == "." {
		return base, nil
	}
	clean := filepath.Clean(rel)
	if !filepath.IsLocal(clean) {
		return "", fmt.Errorf("%s %q must be a relative path inside the repository", what, rel)
	}
	joined := filepath.Join(base, clean)
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", what, err)
	}
	real, err := filepath.EvalSymlinks(joined)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s %q does not exist in the repository", what, rel)
		}
		return "", fmt.Errorf("resolve %s %q: %w", what, rel, err)
	}
	if !isWithin(realBase, real) {
		return "", fmt.Errorf("%s %q resolves outside the repository", what, rel)
	}
	return joined, nil
}

func isWithin(base, p string) bool {
	return p == base || strings.HasPrefix(p, base+string(os.PathSeparator))
}

// removeEscapingSymlinks deletes every symlink under dir whose target
// resolves outside dir (or doesn't resolve at all). Static output is served
// straight off disk by Caddy's file_server, which follows symlinks, so a
// repo committing `index.html -> /etc/passwd` would otherwise publish that
// host file. Symlinks between files inside the site are left alone.
func removeEscapingSymlinks(dir string) error {
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		target, err := filepath.EvalSymlinks(p)
		if err != nil || !isWithin(realDir, target) {
			if rmErr := os.Remove(p); rmErr != nil {
				return rmErr
			}
		}
		return nil
	})
}
