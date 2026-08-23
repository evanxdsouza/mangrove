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
	"regexp"
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

// commitSHAPattern matches a full or abbreviated commit hash. `git clone
// --branch` only resolves refs/heads/<name> or refs/tags/<name> on the
// remote -- it never accepts a raw object ID, even one the remote would
// happily serve via `git fetch <sha>` (GitHub allows fetching any reachable
// commit by SHA). GitRef is documented as "branch, tag, or commit SHA" (see
// ContextSource), so a SHA-shaped ref needs the fetch-by-object-id path in
// gitFetchSHA below rather than gitClone's --branch flag, which would fail
// with "Remote branch <sha> not found in upstream origin". This is what
// makes promote-to-production-by-exact-commit work: promoting deploys the
// exact commit staging is running, not just "whatever main's tip is now".
var commitSHAPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

func looksLikeCommitSHA(ref string) bool {
	return commitSHAPattern.MatchString(ref)
}

// gitClone materializes src.GitRef into dir: a shallow branch/tag clone for
// an ordinary ref, or a fetch-by-object-id for a commit SHA (see
// looksLikeCommitSHA). The auth token, when present, is passed via
// GIT_CONFIG_KEY_n/GIT_CONFIG_VALUE_n environment variables rather than as
// a CLI flag or embedded in the URL — env is not visible to other users via
// `ps`, whereas argv is.
func gitClone(ctx context.Context, src ContextSource, dir string) error {
	if src.GitRef != "" && looksLikeCommitSHA(src.GitRef) {
		return gitFetchSHA(ctx, src, dir)
	}

	args := []string{"clone", "--depth", "1", "--single-branch"}
	if src.GitRef != "" {
		args = append(args, "--branch", src.GitRef)
	}
	args = append(args, src.GitURL, dir)

	out, err := runGitCmd(ctx, dir, src.AuthToken, args...)
	if err != nil {
		return fmt.Errorf("%w: %s", err, redactOutput(out, src.AuthToken))
	}
	return nil
}

// gitFetchSHA checks out an exact commit that need not be any branch's
// current tip: init an empty repo, add the remote, fetch just that object
// (depth 1 -- GitHub serves a reachable SHA without needing history behind
// it), then check it out. `git clone --branch <sha>` cannot do this (see
// looksLikeCommitSHA); `git fetch <url> <sha>` can, because fetch resolves
// against whatever the remote's upload-pack advertises it'll serve, not
// just its ref list.
func gitFetchSHA(ctx context.Context, src ContextSource, dir string) error {
	steps := [][]string{
		{"init"},
		{"remote", "add", "origin", src.GitURL},
		{"fetch", "--depth", "1", "origin", src.GitRef},
		{"checkout", "FETCH_HEAD"},
	}
	for _, args := range steps {
		out, err := runGitCmd(ctx, dir, src.AuthToken, args...)
		if err != nil {
			return fmt.Errorf("git %s: %w: %s", args[0], err, redactOutput(out, src.AuthToken))
		}
	}
	return nil
}

// runGitCmd runs `git <args...>` with cwd set to dir (needed for the
// init/remote/fetch/checkout sequence, where dir is the repo itself rather
// than clone's destination argument) and the auth header injected the same
// way for every git invocation.
func runGitCmd(ctx context.Context, dir, authToken string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	if authToken != "" {
		header := "AUTHORIZATION: basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+authToken))
		cmd.Env = append(cmd.Env,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.extraHeader",
			"GIT_CONFIG_VALUE_0="+header,
		)
	}

	return cmd.CombinedOutput()
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
