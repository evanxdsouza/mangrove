package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// seedWorkspace creates a workspace (owned by ownerID, who becomes its
// admin automatically per store.CreateWorkspace) plus one project,
// deployment, and service inside it -- the minimum fixture every
// workspace-role test below needs, none of which touches s.Orchestrator
// (same constraint as roleTestEnv's existing tests).
func (env *roleTestEnv) seedWorkspace(t *testing.T, ownerID int64, slug string) (workspaceID, projectID, deploymentID, serviceID int64) {
	t.Helper()
	ctx := context.Background()

	ws, err := env.store.CreateWorkspace(ctx, slug, slug, ownerID)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	proj, err := env.store.CreateProject(ctx, ws.ID, slug, slug, "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dep, err := env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: proj.ID, Name: slug, Slug: slug, BuildStrategy: "dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	svc, err := env.store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-" + slug + "-web", InternalPort: 8080,
	})
	if err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	return ws.ID, proj.ID, dep.ID, svc.ID
}

// setRole replaces memberID's workspace_members role in workspaceID
// ("" removes membership entirely), so a single test can walk through
// viewer -> editor -> admin against the same fixture.
func (env *roleTestEnv) setRole(t *testing.T, workspaceID, memberID int64, role string) {
	t.Helper()
	ctx := context.Background()
	_ = env.store.RemoveWorkspaceMember(ctx, workspaceID, memberID) // ignore "wasn't a member"
	if role == "" {
		return
	}
	if err := env.store.AddWorkspaceMember(ctx, workspaceID, memberID, role); err != nil {
		t.Fatalf("AddWorkspaceMember(%s): %v", role, err)
	}
}

// TestNonexistentWorkspaceResourceReturns404NotLeaked confirms a caller
// with no access to a resource that doesn't exist gets the same 404 they'd
// get if it existed but they lacked access -- see auth.RequireWorkspaceRole's
// doc comment on why that distinction matters.
func TestNonexistentWorkspaceResourceReturns404NotLeaked(t *testing.T) {
	env := newRoleTestEnv(t)
	memberCookie, _ := env.cookieFor(t, "member@example.com", "member")

	for _, path := range []string{"/api/projects/99999", "/api/deployments/99999", "/api/services/99999"} {
		rec := env.do(http.MethodGet, path, memberCookie, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: expected 404, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}

// TestMemberWithNoWorkspaceMembershipBlocked is the headline case: a
// member who exists on the box but has no workspace_members row in a
// given workspace gets 403 on that workspace's resources -- including,
// critically, exec/terminal, which before this pass had no scoping at all.
func TestMemberWithNoWorkspaceMembershipBlocked(t *testing.T) {
	env := newRoleTestEnv(t)
	_, ownerID := env.cookieFor(t, "owner@example.com", "owner")
	memberCookie, _ := env.cookieFor(t, "member@example.com", "member")

	_, projectID, deploymentID, serviceID := env.seedWorkspace(t, ownerID, "acme")

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"get project", http.MethodGet, "/api/projects/" + itoa(projectID)},
		{"get deployment", http.MethodGet, "/api/deployments/" + itoa(deploymentID)},
		{"get service", http.MethodGet, "/api/services/" + itoa(serviceID)},
		{"deploy", http.MethodPost, "/api/deployments/" + itoa(deploymentID) + "/deploy"},
		{"exec", http.MethodPost, "/api/services/" + itoa(serviceID) + "/exec"},
		{"terminal", http.MethodGet, "/api/services/" + itoa(serviceID) + "/terminal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := env.do(c.method, c.path, memberCookie, nil)
			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 for a member with no membership in this workspace on %s %s, got %d: %s", c.method, c.path, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestWorkspaceRoleThresholds walks the same member through
// viewer -> editor -> admin in one workspace, checking each tier can do
// what it should and nothing more.
func TestWorkspaceRoleThresholds(t *testing.T) {
	env := newRoleTestEnv(t)
	_, ownerID := env.cookieFor(t, "owner@example.com", "owner")
	memberCookie, memberID := env.cookieFor(t, "member@example.com", "member")

	workspaceID, projectID, deploymentID, serviceID := env.seedWorkspace(t, ownerID, "acme")

	t.Run("viewer can view but not act", func(t *testing.T) {
		env.setRole(t, workspaceID, memberID, "viewer")

		rec := env.do(http.MethodGet, "/api/projects/"+itoa(projectID), memberCookie, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 viewing project as viewer, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = env.do(http.MethodGet, "/api/deployments/"+itoa(deploymentID), memberCookie, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 viewing deployment as viewer, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = env.do(http.MethodPost, "/api/deployments/"+itoa(deploymentID)+"/deploy", memberCookie, nil)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 deploying as viewer, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = env.do(http.MethodPost, "/api/services/"+itoa(serviceID)+"/exec", memberCookie, map[string]any{"command": []string{"ls"}})
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 exec'ing as viewer, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("editor can act but not delete or set secrets", func(t *testing.T) {
		env.setRole(t, workspaceID, memberID, "editor")

		// createDeployment is pure store (no Orchestrator dependency), so
		// this is a genuine end-to-end success check, unlike deploy/exec
		// above which this test env can only verify get *rejected*
		// correctly (a real deploy/exec needs Docker -- verified manually
		// end-to-end separately, same as the backup/restore pass).
		rec := env.do(http.MethodPost, "/api/projects/"+itoa(projectID)+"/deployments", memberCookie, map[string]any{
			"name": "worker", "slug": "worker", "build_strategy": "dockerfile",
			"service": map[string]any{"name": "web", "internal_port": 8080},
		})
		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201 creating a deployment as editor, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = env.do(http.MethodDelete, "/api/projects/"+itoa(projectID), memberCookie, nil)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 deleting a project as editor, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = env.do(http.MethodPut, "/api/services/"+itoa(serviceID)+"/env/API_KEY", memberCookie, map[string]any{"value": "x", "is_secret": true})
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 setting a secret env var as editor, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = env.do(http.MethodPost, "/api/deployments/"+itoa(deploymentID)+"/access", memberCookie, map[string]any{"is_public": true})
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 setting access control as editor, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("admin can delete and set secrets -- the devolved global-owner powers", func(t *testing.T) {
		env.setRole(t, workspaceID, memberID, "admin")

		rec := env.do(http.MethodPut, "/api/services/"+itoa(serviceID)+"/env/API_KEY", memberCookie, map[string]any{"value": "x", "is_secret": true})
		if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusNoContent {
			// StatusInternalServerError is acceptable here: s.Secrets is
			// nil in this lightweight test env (no master key configured),
			// so a *permitted* secret write fails downstream at encryption,
			// not at the role check -- what this test actually verifies is
			// that it got past the 403 boundary, i.e. did NOT get 403.
			t.Errorf("expected the role check to pass (200/204 or a downstream 500, not 403) setting a secret as workspace-admin, got %d: %s", rec.Code, rec.Body.String())
		}

		// setDeploymentAccess and deleteProject both call into
		// s.Orchestrator (Caddy gate config / container teardown), which
		// is nil in this lightweight test env -- same reasoning as the
		// secret-env-var check above: a *permitted* action panics
		// downstream (chi's Recoverer turns that into a 500), which still
		// proves the role check passed. Real success is covered by this
		// session's manual end-to-end smoke test against a live instance.
		rec = env.do(http.MethodPost, "/api/deployments/"+itoa(deploymentID)+"/access", memberCookie, map[string]any{"is_public": true})
		if rec.Code == http.StatusForbidden {
			t.Errorf("expected the role check to pass setting access control as workspace-admin, got 403: %s", rec.Body.String())
		}

		rec = env.do(http.MethodDelete, "/api/projects/"+itoa(projectID), memberCookie, nil)
		if rec.Code == http.StatusForbidden {
			t.Errorf("expected the role check to pass deleting a project as workspace-admin, got 403: %s", rec.Body.String())
		}
	})
}

// TestWorkspaceAdminScopedNotGlobal proves admin-devolution (workspace
// admins can delete/set-secrets within their own workspace) didn't
// accidentally become global -- an admin of workspace A has no special
// power over workspace B.
func TestWorkspaceAdminScopedNotGlobal(t *testing.T) {
	env := newRoleTestEnv(t)
	_, ownerID := env.cookieFor(t, "owner@example.com", "owner")
	memberCookie, memberID := env.cookieFor(t, "member@example.com", "member")

	workspaceA, _, _, _ := env.seedWorkspace(t, ownerID, "acme")
	_, projectB, _, _ := env.seedWorkspace(t, ownerID, "globex")

	env.setRole(t, workspaceA, memberID, "admin")

	rec := env.do(http.MethodDelete, "/api/projects/"+itoa(projectB), memberCookie, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 deleting a project in a workspace this admin doesn't belong to, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestWorkspaceMembershipManagementEndpoints covers the new
// /api/workspaces/{id}/members routes: admin-only, add-by-email (not a
// roster picker), role validation, and the not-a-member/duplicate cases.
func TestWorkspaceMembershipManagementEndpoints(t *testing.T) {
	env := newRoleTestEnv(t)
	_, ownerID := env.cookieFor(t, "owner@example.com", "owner")
	editorCookie, editorID := env.cookieFor(t, "editor@example.com", "member")
	_, colleagueID := env.cookieFor(t, "colleague@example.com", "member")

	workspaceID, _, _, _ := env.seedWorkspace(t, ownerID, "acme")
	env.setRole(t, workspaceID, editorID, "editor")

	membersPath := "/api/workspaces/" + itoa(workspaceID) + "/members"

	t.Run("non-admin forbidden", func(t *testing.T) {
		rec := env.do(http.MethodGet, membersPath, editorCookie, nil)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 listing members as editor, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = env.do(http.MethodPost, membersPath, editorCookie, map[string]any{"email": "colleague@example.com", "role": "viewer"})
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 adding a member as editor, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	env.setRole(t, workspaceID, editorID, "admin")

	t.Run("admin can manage members", func(t *testing.T) {
		rec := env.do(http.MethodPost, membersPath, editorCookie, map[string]any{"email": "colleague@example.com", "role": "viewer"})
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 adding a member as admin, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = env.do(http.MethodPost, membersPath, editorCookie, map[string]any{"email": "colleague@example.com", "role": "viewer"})
		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 re-adding an existing member, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = env.do(http.MethodPost, membersPath, editorCookie, map[string]any{"email": "nobody@example.com", "role": "viewer"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 adding a nonexistent account, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = env.do(http.MethodPost, membersPath, editorCookie, map[string]any{"email": "someone@example.com", "role": "superuser"})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for an invalid role, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = env.do(http.MethodPut, membersPath+"/"+itoa(colleagueID), editorCookie, map[string]any{"role": "editor"})
		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204 changing a member's role as admin, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = env.do(http.MethodGet, membersPath, editorCookie, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 listing members as admin, got %d: %s", rec.Code, rec.Body.String())
		}

		rec = env.do(http.MethodDelete, membersPath+"/"+itoa(colleagueID), editorCookie, nil)
		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204 removing a member as admin, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = env.do(http.MethodDelete, membersPath+"/"+itoa(colleagueID), editorCookie, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 removing an already-removed member, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
