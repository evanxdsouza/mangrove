package api

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// TestDispatchDeployRoutesStaticToDeployStatic guards against the bug that
// made GitHub auto-deploy silently fail for every static-strategy
// deployment: the webhook handler used to two-way-switch on
// "compose" vs everything-else, so a static deployment fell into Deploy()
// -- written for container-based strategies -- which tries to run a
// container from an image that was never built (static builds produce a
// directory, not an image) and fails. dispatchDeploy's three-way switch
// (mirroring triggerDeploy/redeployDeployment) is what every deploy
// trigger, including the webhook, now goes through.
//
// This exercises dispatchDeploy directly against a local git repo (no
// GitHub/network dependency) so the test is deterministic: a static
// deployment should build (copy the repo's files) and succeed with no
// container ever started.
func TestDispatchDeployRoutesStaticToDeployStatic(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	repoDir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repoDir, "index.html"), []byte("<h1>hi</h1>"), 0644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q")
	run("config", "user.email", "t@t.com")
	run("config", "user.name", "t")
	run("add", "index.html")
	run("commit", "-q", "-m", "initial")

	proj, err := env.store.CreateProject(ctx, 1, "Dispatch Test", "dispatch-test", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dep, err := env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: proj.ID, Name: "site", Slug: "dispatch-test-site", BuildStrategy: "static",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if _, err := env.store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: dep.ID, Name: "site", ContainerName: "mangrove-dispatch-test-site",
		IsInternalOnly: true, NoContainer: true,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}

	req := orchestrator.DeployRequest{
		DeploymentID: dep.ID,
		TriggeredBy:  "push",
		GitURL:       repoDir,
	}
	historyID, err := env.server.dispatchDeploy(ctx, dep, req)
	if err != nil {
		t.Fatalf("dispatchDeploy: %v", err)
	}

	history, err := env.store.GetDeployHistory(ctx, historyID)
	if err != nil {
		t.Fatalf("GetDeployHistory: %v", err)
	}
	if history.Status != "success" {
		t.Errorf("expected static deploy to succeed with no container, got status=%q error=%q", history.Status, history.ErrorMessage)
	}
}
