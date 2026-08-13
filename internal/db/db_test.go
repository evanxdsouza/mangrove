package db

import (
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrations(t *testing.T) {
	dir := t.TempDir()
	conn, err := Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	tables := []string{
		"organizations", "workspaces", "users", "workspace_members", "sessions",
		"nodes", "projects", "github_pats", "project_repos", "deployments",
		"services", "volumes", "port_registry", "deploy_history",
		"deploy_history_artifacts", "env_vars", "webhook_events",
		"health_checks", "notifications_log",
	}
	for _, tbl := range tables {
		var name string
		err := conn.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", tbl, err)
		}
	}

	var orgCount, wsCount, nodeCount int
	conn.QueryRow(`SELECT COUNT(*) FROM organizations`).Scan(&orgCount)
	conn.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&wsCount)
	conn.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&nodeCount)
	if orgCount != 1 || wsCount != 1 || nodeCount != 1 {
		t.Errorf("expected seed rows org=1 ws=1 node=1, got org=%d ws=%d node=%d", orgCount, wsCount, nodeCount)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mangrove.db")

	conn1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	conn1.Close()

	conn2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open (re-applying migrations should be a no-op): %v", err)
	}
	defer conn2.Close()

	var count int
	if err := conn2.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 applied migrations, got %d", count)
	}
}

// TestStaticStrategyMigrationPreservesExistingRows guards the 0002 rebuild
// of the deployments table (needed to add 'static' to build_strategy's
// CHECK constraint, which SQLite can't ALTER in place): a deployment and a
// dependent service row inserted before the rebuild must survive it with
// their foreign key intact, and the new build_strategy value plus the new
// static_* columns must both work afterward.
func TestStaticStrategyMigrationPreservesExistingRows(t *testing.T) {
	dir := t.TempDir()
	conn, err := Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'p', 'p')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO deployments (project_id, name, slug, build_strategy) VALUES (1, 'd', 'd', 'dockerfile')`); err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO services (deployment_id, name, container_name) VALUES (1, 'web', 'mangrove-d-web')`); err != nil {
		t.Fatalf("insert service: %v", err)
	}

	var svcCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM services WHERE deployment_id = 1`).Scan(&svcCount); err != nil {
		t.Fatalf("query services: %v", err)
	}
	if svcCount != 1 {
		t.Fatalf("expected the pre-existing service row to survive the deployments table rebuild, got %d rows", svcCount)
	}

	if _, err := conn.Exec(`INSERT INTO deployments (project_id, name, slug, build_strategy, static_build_command, static_output_dir) VALUES (1, 's', 's', 'static', 'npm run build', 'dist')`); err != nil {
		t.Errorf("insert static deployment: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO deployments (project_id, name, slug, build_strategy) VALUES (1, 'bad', 'bad', 'not-a-real-strategy')`); err == nil {
		t.Error("expected CHECK constraint to still reject an invalid build_strategy after the rebuild")
	}

	if _, err := conn.Exec(`INSERT INTO deploy_history (deployment_id, triggered_by, status) VALUES (1, 'manual', 'success')`); err != nil {
		t.Fatalf("insert deploy_history: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO deploy_history_artifacts (deploy_history_id, service_id, image_tag, output_path) VALUES (1, 1, '', '/data/static/d-1')`); err != nil {
		t.Errorf("insert deploy_history_artifacts with output_path: %v", err)
	}
}

// TestRedeployTriggerMigrationPreservesExistingRows guards the 0004 rebuild
// of deploy_history (needed to add 'redeploy' to triggered_by's CHECK
// constraint): a pre-existing deploy_history row, its dependent
// deploy_history_artifacts row, and its self-referencing
// rollback_of_deploy_history_id must all survive the rebuild with foreign
// keys intact, and the new 'redeploy' value must work afterward.
func TestRedeployTriggerMigrationPreservesExistingRows(t *testing.T) {
	dir := t.TempDir()
	conn, err := Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'p', 'p')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO deployments (project_id, name, slug, build_strategy) VALUES (1, 'd', 'd', 'dockerfile')`); err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO services (deployment_id, name, container_name) VALUES (1, 'web', 'mangrove-d-web')`); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO deploy_history (deployment_id, triggered_by, status) VALUES (1, 'manual', 'success')`); err != nil {
		t.Fatalf("insert deploy_history: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO deploy_history_artifacts (deploy_history_id, service_id, image_tag) VALUES (1, 1, 'img:1')`); err != nil {
		t.Fatalf("insert deploy_history_artifacts: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO deploy_history (deployment_id, triggered_by, status, rollback_of_deploy_history_id) VALUES (1, 'rollback', 'success', 1)`); err != nil {
		t.Fatalf("insert rollback deploy_history: %v", err)
	}

	var artifactCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM deploy_history_artifacts WHERE deploy_history_id = 1`).Scan(&artifactCount); err != nil {
		t.Fatalf("query deploy_history_artifacts: %v", err)
	}
	if artifactCount != 1 {
		t.Fatalf("expected the pre-existing artifact row to survive the deploy_history rebuild, got %d rows", artifactCount)
	}
	var rollbackTarget int64
	if err := conn.QueryRow(`SELECT rollback_of_deploy_history_id FROM deploy_history WHERE id = 2`).Scan(&rollbackTarget); err != nil {
		t.Fatalf("query rollback_of_deploy_history_id: %v", err)
	}
	if rollbackTarget != 1 {
		t.Errorf("expected the self-referencing FK to survive the rebuild, got %d", rollbackTarget)
	}

	if _, err := conn.Exec(`INSERT INTO deploy_history (deployment_id, triggered_by, status) VALUES (1, 'redeploy', 'success')`); err != nil {
		t.Errorf("insert deploy_history with triggered_by='redeploy': %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO deploy_history (deployment_id, triggered_by, status) VALUES (1, 'not-a-real-trigger', 'success')`); err == nil {
		t.Error("expected CHECK constraint to still reject an invalid triggered_by after the rebuild")
	}
}
