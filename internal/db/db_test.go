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
	if count != 1 {
		t.Errorf("expected 1 applied migration, got %d", count)
	}
}
