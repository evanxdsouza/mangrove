package executor

import (
	"archive/tar"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// materialize turns a ContextSource into a local directory the Docker CLI
// can build against. This is where the "no bare host paths crossing the
// Executor boundary" rule gets satisfied on the local implementation's
// side: the caller only ever hands over a git ref or a tarball stream.
func materialize(ctx context.Context, src ContextSource) (dir string, cleanup func(), err error) {
	tmpDir, err := os.MkdirTemp("", "mangrove-build-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp build dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(tmpDir) }

	switch {
	case src.Tarball != nil:
		if err := extractTar(src.Tarball, tmpDir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("extract tarball: %w", err)
		}
	case src.GitURL != "":
		if err := gitClone(ctx, src, tmpDir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("git clone: %w", err)
		}
	default:
		cleanup()
		return "", nil, fmt.Errorf("context source has neither GitURL nor Tarball set")
	}
	return tmpDir, cleanup, nil
}

// gitClone shallow-clones src.GitRef into dir. The auth token, when present,
// is passed via GIT_CONFIG_KEY_n/GIT_CONFIG_VALUE_n environment variables
// rather than as a CLI flag or embedded in the URL — env is not visible to
// other users via `ps`, whereas argv is.
func gitClone(ctx context.Context, src ContextSource, dir string) error {
	args := []string{"clone", "--depth", "1", "--single-branch"}
	if src.GitRef != "" {
		args = append(args, "--branch", src.GitRef)
	}
	args = append(args, src.GitURL, dir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = os.Environ()

	if src.AuthToken != "" {
		header := "AUTHORIZATION: basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+src.AuthToken))
		cmd.Env = append(cmd.Env,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.extraHeader",
			"GIT_CONFIG_VALUE_0="+header,
		)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, redactOutput(out, src.AuthToken))
	}
	return nil
}

// redactOutput strips a token from command output before it can end up in
// an error message that might be logged or displayed.
func redactOutput(out []byte, token string) string {
	if token == "" {
		return string(out)
	}
	return strings.ReplaceAll(string(out), token, "[REDACTED]")
}

func extractTar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("tar entry %q escapes destination directory", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
}
