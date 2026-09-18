package db

import (
	"path/filepath"
	"testing"
)

// TestWorkspaceRoleBackfillMigration simulates the case the 0012 migration
// exists for: a database with users and workspaces that predate real
// per-workspace roles, upgrading onto them. Open already ran the migration
// once (a no-op on the empty fresh DB it creates), so this seeds
// "pre-upgrade" rows via raw SQL -- bypassing internal/store's CreateUser/
// CreateWorkspace, which already insert their own workspace_members rows
// and would mask whether the migration's backfill actually works -- then
// re-runs the migration's own SQL text (read from the embedded file, not a
// copy, so this can't drift out of sync with what actually ships) to prove
// it grants every existing user the right role in every existing workspace
// without clobbering a row that's already there.
func TestWorkspaceRoleBackfillMigration(t *testing.T) {
	dir := t.TempDir()
	conn, err := Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// Pre-upgrade state: a second workspace, an owner, and a member, none
	// of which have a workspace_members row anywhere (raw INSERT, not
	// CreateUser/CreateWorkspace).
	if _, err := conn.Exec(`INSERT INTO workspaces (id, org_id, name, slug) VALUES (2, 1, 'Staging', 'staging')`); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO users (id, org_id, email, password_hash, role) VALUES
		(100, 1, 'legacy-owner@example.com', 'x', 'owner'),
		(101, 1, 'legacy-member@example.com', 'x', 'member')`); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	// A pre-existing custom membership that must survive untouched --
	// INSERT OR IGNORE must not clobber it with the backfill's default.
	if _, err := conn.Exec(`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES (2, 100, 'viewer')`); err != nil {
		t.Fatalf("seed pre-existing membership: %v", err)
	}

	sql, err := migrationsFS.ReadFile("migrations/0012_workspace_role_backfill.sql")
	if err != nil {
		t.Fatalf("read migration file: %v", err)
	}
	if _, err := conn.Exec(string(sql)); err != nil {
		t.Fatalf("re-run backfill migration: %v", err)
	}

	cases := []struct {
		userID, workspaceID int64
		wantRole             string
	}{
		{100, 1, "admin"},  // legacy owner, default workspace: backfilled admin
		{100, 2, "viewer"}, // legacy owner, staging workspace: pre-existing row untouched, NOT overwritten to admin
		{101, 1, "editor"}, // legacy member, default workspace: backfilled editor
		{101, 2, "editor"}, // legacy member, staging workspace: backfilled editor
	}
	for _, c := range cases {
		var role string
		err := conn.QueryRow(`SELECT role FROM workspace_members WHERE user_id = ? AND workspace_id = ?`, c.userID, c.workspaceID).Scan(&role)
		if err != nil {
			t.Errorf("user %d workspace %d: %v", c.userID, c.workspaceID, err)
			continue
		}
		if role != c.wantRole {
			t.Errorf("user %d workspace %d: got role %q, want %q", c.userID, c.workspaceID, role, c.wantRole)
		}
	}
}
