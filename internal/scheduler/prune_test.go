package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/store"
)

type fakePruneExecutor struct {
	executor.Executor
	pruneCalls []executor.PruneOptions
}

func (f *fakePruneExecutor) Prune(ctx context.Context, opts executor.PruneOptions) (executor.PruneResult, error) {
	f.pruneCalls = append(f.pruneCalls, opts)
	return executor.PruneResult{ImagesRemoved: 2, SpaceReclaimedMB: 100}, nil
}

// seedServiceWithArtifacts creates a project -> deployment (retention N) ->
// service, plus `count` deploy_history + artifact rows for that service.
func seedServiceWithArtifacts(t *testing.T, st *store.Store, retention, count int) int64 {
	t.Helper()
	ctx := context.Background()

	res, err := st.DB.ExecContext(ctx, `INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'p', 'prune-test')`)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	projectID, _ := res.LastInsertId()

	res, err = st.DB.ExecContext(ctx,
		`INSERT INTO deployments (project_id, name, slug, build_strategy, image_retention_count) VALUES (?, 'd', 'd', 'dockerfile', ?)`,
		projectID, retention)
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	deploymentID, _ := res.LastInsertId()

	res, err = st.DB.ExecContext(ctx, `INSERT INTO services (deployment_id, name, container_name, internal_port) VALUES (?, 'web', 'c', 80)`, deploymentID)
	if err != nil {
		t.Fatalf("insert service: %v", err)
	}
	serviceID, _ := res.LastInsertId()

	for i := 0; i < count; i++ {
		res, err := st.DB.ExecContext(ctx, `INSERT INTO deploy_history (deployment_id, triggered_by, status) VALUES (?, 'manual', 'success')`, deploymentID)
		if err != nil {
			t.Fatalf("insert deploy_history: %v", err)
		}
		historyID, _ := res.LastInsertId()
		if err := st.CreateDeployArtifact(ctx, historyID, serviceID, "tag", "img", "", ""); err != nil {
			t.Fatalf("CreateDeployArtifact: %v", err)
		}
	}

	return serviceID
}

func TestTickPrunesServiceOverRetention(t *testing.T) {
	st := testStore(t)
	seedServiceWithArtifacts(t, st, 5, 8) // 8 artifacts, retention 5 -> should prune

	fake := &fakePruneExecutor{}
	p := NewPruner(st, fake, discardLogger())
	p.tick(context.Background())

	if len(fake.pruneCalls) != 1 {
		t.Fatalf("expected 1 Prune call, got %d", len(fake.pruneCalls))
	}
	if len(fake.pruneCalls[0].KeepImageTags) != 5 {
		t.Errorf("expected 5 kept tags, got %d", len(fake.pruneCalls[0].KeepImageTags))
	}
}

func TestTickSkipsServiceUnderRetention(t *testing.T) {
	st := testStore(t)
	seedServiceWithArtifacts(t, st, 5, 3) // only 3 artifacts, retention 5 -> nothing to prune

	fake := &fakePruneExecutor{}
	p := NewPruner(st, fake, discardLogger())
	p.tick(context.Background())

	if len(fake.pruneCalls) != 0 {
		t.Errorf("expected no Prune call when under retention, got %d", len(fake.pruneCalls))
	}
}

// TestTickPrunesStaticOutputDirectories covers the static-strategy path,
// which has no image for p.Exec.Prune to touch -- pruning has to remove
// output directories from disk directly instead.
func TestTickPrunesStaticOutputDirectories(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	res, err := st.DB.ExecContext(ctx, `INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'p', 'prune-static-test')`)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	projectID, _ := res.LastInsertId()
	res, err = st.DB.ExecContext(ctx,
		`INSERT INTO deployments (project_id, name, slug, build_strategy, image_retention_count) VALUES (?, 'd', 'd', 'static', 2)`,
		projectID)
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	deploymentID, _ := res.LastInsertId()
	res, err = st.DB.ExecContext(ctx, `INSERT INTO services (deployment_id, name, container_name, internal_port) VALUES (?, 'web', 'c', 0)`, deploymentID)
	if err != nil {
		t.Fatalf("insert service: %v", err)
	}
	serviceID, _ := res.LastInsertId()

	dir := t.TempDir()
	var outputPaths []string
	for i := 0; i < 4; i++ {
		res, err := st.DB.ExecContext(ctx, `INSERT INTO deploy_history (deployment_id, triggered_by, status) VALUES (?, 'manual', 'success')`, deploymentID)
		if err != nil {
			t.Fatalf("insert deploy_history: %v", err)
		}
		historyID, _ := res.LastInsertId()
		outputPath := filepath.Join(dir, "out-"+string(rune('0'+i)))
		if err := os.MkdirAll(outputPath, 0755); err != nil {
			t.Fatalf("mkdir output path: %v", err)
		}
		if err := st.CreateDeployArtifact(ctx, historyID, serviceID, "", "", "", outputPath); err != nil {
			t.Fatalf("CreateDeployArtifact: %v", err)
		}
		outputPaths = append(outputPaths, outputPath)
	}

	fake := &fakePruneExecutor{}
	p := NewPruner(st, fake, discardLogger())
	p.tick(context.Background())

	if len(fake.pruneCalls) != 0 {
		t.Errorf("static strategy should never call Exec.Prune (no images), got %d calls", len(fake.pruneCalls))
	}
	// deploy_history.started_at has only second resolution, so which 2 of
	// the 4 (same-second) inserts count as "newest" is unspecified -- only
	// the retention count itself (keep 2 of 4) is asserted, matching how
	// TestTickPrunesServiceOverRetention avoids asserting exact identity too.
	remaining := 0
	for _, path := range outputPaths {
		if _, statErr := os.Stat(path); statErr == nil {
			remaining++
		} else if !os.IsNotExist(statErr) {
			t.Errorf("unexpected stat error for %s: %v", path, statErr)
		}
	}
	if remaining != 2 {
		t.Errorf("expected 2 of 4 output dirs to survive pruning (retention=2), got %d", remaining)
	}
}
