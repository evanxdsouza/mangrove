package store

import (
	"context"
	"path/filepath"
	"testing"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
)

// TestListDeploymentsReturnsFullFields locks in the single-query rewrite
// of ListDeployments/ListAllDeployments: both used to SELECT id then call
// GetDeployment per row (an N+1), so a regression back to that pattern
// wouldn't be caught by a test that only checks IDs/count.
func TestListDeploymentsReturnsFullFields(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	proj, err := st.CreateProject(ctx, 1, "Widgets", "widgets", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	created, err := st.CreateDeployment(ctx, CreateDeploymentParams{
		ProjectID:      proj.ID,
		Name:           "api",
		Slug:           "api",
		BuildStrategy:  "dockerfile",
		GitBranch:      "main",
		RootPath:       "services/api",
		DockerfilePath: "Dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	list, err := st.ListDeployments(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(list))
	}
	got := list[0]
	if got.ID != created.ID || got.Name != "api" || got.Slug != "api" ||
		got.BuildStrategy != "dockerfile" || got.GitBranch != "main" ||
		got.RootPath != "services/api" || got.DockerfilePath != "Dockerfile" ||
		got.ProjectID != proj.ID || got.Status == "" || got.CreatedAt.IsZero() {
		t.Fatalf("ListDeployments returned incomplete row: %+v", got)
	}

	all, err := st.ListAllDeployments(ctx)
	if err != nil {
		t.Fatalf("ListAllDeployments: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(all))
	}
	gotAll := all[0]
	if gotAll.ID != created.ID || gotAll.Name != "api" || gotAll.Slug != "api" ||
		gotAll.BuildStrategy != "dockerfile" || gotAll.RootPath != "services/api" {
		t.Fatalf("ListAllDeployments returned incomplete row: %+v", gotAll)
	}
}

// TestSweepStuckDeploysAlsoFailsTheDeployment locks in the fix for a
// deployment left showing "building" forever in the UI after a crash or
// restart: SweepStuckDeploys used to only touch the deploy_history row
// (correctly marking it "failed"), leaving deployments.status -- set to
// "building" at the start of the same deploy and only otherwise flipped
// by the deploy that finishes it -- stuck on "building" with nothing left
// working on it.
func TestSweepStuckDeploysAlsoFailsTheDeployment(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	proj, err := st.CreateProject(ctx, 1, "Widgets", "widgets", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dep, err := st.CreateDeployment(ctx, CreateDeploymentParams{
		ProjectID:     proj.ID,
		Name:          "api",
		Slug:          "api",
		BuildStrategy: "dockerfile",
		GitBranch:     "main",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	// Simulate what orchestrator.Deploy does at the start of a deploy,
	// then simulate a crash/restart before it finishes: the deployment
	// and its deploy_history are both left mid-flight.
	historyID, err := st.CreateDeployHistory(ctx, dep.ID, "push", "", "", "main")
	if err != nil {
		t.Fatalf("CreateDeployHistory: %v", err)
	}
	if err := st.UpdateDeploymentStatus(ctx, dep.ID, "building"); err != nil {
		t.Fatalf("UpdateDeploymentStatus: %v", err)
	}
	if err := st.UpdateDeployHistoryStatus(ctx, historyID, "building", ""); err != nil {
		t.Fatalf("UpdateDeployHistoryStatus: %v", err)
	}

	if _, err := st.SweepStuckDeploys(ctx); err != nil {
		t.Fatalf("SweepStuckDeploys: %v", err)
	}

	gotDep, err := st.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if gotDep.Status != "failed" {
		t.Fatalf("deployment status = %q, want %q", gotDep.Status, "failed")
	}

	gotHist, err := st.GetDeployHistory(ctx, historyID)
	if err != nil {
		t.Fatalf("GetDeployHistory: %v", err)
	}
	if gotHist.Status != "failed" {
		t.Fatalf("deploy_history status = %q, want %q", gotHist.Status, "failed")
	}
}
