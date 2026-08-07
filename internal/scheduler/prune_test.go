package scheduler

import (
	"context"
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
		if err := st.CreateDeployArtifact(ctx, historyID, serviceID, "tag", "img", ""); err != nil {
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
