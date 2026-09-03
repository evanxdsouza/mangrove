package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestListWorkspacesJSONShape locks in the wire shape the frontend actually
// reads (WorkspacesPage.tsx: w.workspace.id, w.project_count) -- lowercase,
// snake_case. store.WorkspaceProjectCount previously had no json tags at
// all, so encoding/json fell back to the Go field names ("Workspace",
// "ProjectCount"), which every row silently failed to match on the
// frontend and broke the whole page (see ErrorBoundary.tsx). Asserting on
// the raw decoded map, not a re-declared Go struct, is what would have
// caught that: a struct with the same accidental mismatch would decode
// "successfully" into zero values instead of failing.
func TestListWorkspacesJSONShape(t *testing.T) {
	env := newRoleTestEnv(t)
	cookie, _ := env.cookieFor(t, "owner@example.com", "owner")

	rec := env.do(http.MethodGet, "/api/workspaces", cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/workspaces: %d %s", rec.Code, rec.Body.String())
	}

	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(rows) != 1 { // the always-present default workspace
		t.Fatalf("expected 1 workspace (the default), got %d: %s", len(rows), rec.Body.String())
	}

	row := rows[0]
	wsRaw, ok := row["workspace"]
	if !ok {
		t.Fatalf(`response row missing "workspace" key, got keys %v`, keysOf(row))
	}
	if _, ok := row["project_count"]; !ok {
		t.Fatalf(`response row missing "project_count" key, got keys %v`, keysOf(row))
	}

	var ws map[string]json.RawMessage
	if err := json.Unmarshal(wsRaw, &ws); err != nil {
		t.Fatalf("decode workspace object: %v", err)
	}
	for _, key := range []string{"id", "name", "slug"} {
		if _, ok := ws[key]; !ok {
			t.Errorf(`workspace object missing %q key, got keys %v`, key, keysOf(ws))
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
