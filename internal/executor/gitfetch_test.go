package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a local git repo with two commits and returns its
// path plus the SHA of the first commit -- enough to exercise both the
// ordinary branch-clone path and the fetch-by-commit-SHA path against a
// real `git` binary, no network required.
func initTestRepo(t *testing.T) (repoDir, firstSHA string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	run("init", "-q")
	run("config", "user.email", "t@t.com")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-q", "-m", "first")
	sha := strings.TrimSpace(run("rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "b.txt")
	run("commit", "-q", "-m", "second")

	return dir, sha
}

func TestGitCloneByBranch(t *testing.T) {
	repoDir, _ := initTestRepo(t)
	dst := t.TempDir()

	err := gitClone(context.Background(), ContextSource{GitURL: repoDir, GitRef: "master"}, dst)
	if err != nil {
		// Some environments default the initial branch to "main".
		err = gitClone(context.Background(), ContextSource{GitURL: repoDir, GitRef: "main"}, dst)
	}
	if err != nil {
		t.Fatalf("gitClone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "b.txt")); err != nil {
		t.Errorf("expected b.txt (tip commit) to exist: %v", err)
	}
}

func TestGitCloneByCommitSHA(t *testing.T) {
	repoDir, firstSHA := initTestRepo(t)
	dst := t.TempDir()

	if !looksLikeCommitSHA(firstSHA) {
		t.Fatalf("looksLikeCommitSHA(%q) = false, want true", firstSHA)
	}

	if err := gitClone(context.Background(), ContextSource{GitURL: repoDir, GitRef: firstSHA}, dst); err != nil {
		t.Fatalf("gitClone by SHA: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); err != nil {
		t.Errorf("expected a.txt (first commit) to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "b.txt")); err == nil {
		t.Errorf("b.txt should not exist when checked out at the first commit")
	}
}

func TestLooksLikeCommitSHA(t *testing.T) {
	cases := map[string]bool{
		"c878d67c84d08d0e3fb990b52dedc4092a1f79b7": true,
		"c878d67":   true,
		"main":      false,
		"feature/x": false,
		"v1.2.3":    false,
		"":          false,
	}
	for ref, want := range cases {
		if got := looksLikeCommitSHA(ref); got != want {
			t.Errorf("looksLikeCommitSHA(%q) = %v, want %v", ref, got, want)
		}
	}
}
