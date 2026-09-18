package store

import (
	"context"
	"path/filepath"
	"testing"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
)

func TestWorkspaceIDResolvers(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	ownerID, err := st.CreateUser(ctx, "owner@example.com", "hash", "owner")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	ws, err := st.CreateWorkspace(ctx, "Production", "production", ownerID)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	proj, err := st.CreateProject(ctx, ws.ID, "API", "api", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dep, err := st.CreateDeployment(ctx, CreateDeploymentParams{
		ProjectID: proj.ID, Name: "web", Slug: "web", BuildStrategy: "dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	svc, err := st.CreateService(ctx, CreateServiceParams{
		DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-api-web", InternalPort: 8080,
	})
	if err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	historyID, err := st.CreateDeployHistory(ctx, dep.ID, "manual", "", "", "")
	if err != nil {
		t.Fatalf("CreateDeployHistory: %v", err)
	}
	domain, err := st.CreateCustomDomain(ctx, dep.ID, "example.com", "verify-token")
	if err != nil {
		t.Fatalf("CreateCustomDomain: %v", err)
	}

	if got, err := st.WorkspaceIDForProject(ctx, proj.ID); err != nil || got != ws.ID {
		t.Errorf("WorkspaceIDForProject: got (%d, %v), want %d", got, err, ws.ID)
	}
	if got, err := st.WorkspaceIDForDeployment(ctx, dep.ID); err != nil || got != ws.ID {
		t.Errorf("WorkspaceIDForDeployment: got (%d, %v), want %d", got, err, ws.ID)
	}
	if got, err := st.WorkspaceIDForService(ctx, svc.ID); err != nil || got != ws.ID {
		t.Errorf("WorkspaceIDForService: got (%d, %v), want %d", got, err, ws.ID)
	}
	if got, err := st.WorkspaceIDForDeployHistory(ctx, historyID); err != nil || got != ws.ID {
		t.Errorf("WorkspaceIDForDeployHistory: got (%d, %v), want %d", got, err, ws.ID)
	}
	if got, err := st.WorkspaceIDForCustomDomain(ctx, domain.ID); err != nil || got != ws.ID {
		t.Errorf("WorkspaceIDForCustomDomain: got (%d, %v), want %d", got, err, ws.ID)
	}

	for name, fn := range map[string]func(context.Context, int64) (int64, error){
		"WorkspaceIDForProject":       st.WorkspaceIDForProject,
		"WorkspaceIDForDeployment":    st.WorkspaceIDForDeployment,
		"WorkspaceIDForService":       st.WorkspaceIDForService,
		"WorkspaceIDForDeployHistory": st.WorkspaceIDForDeployHistory,
		"WorkspaceIDForCustomDomain":  st.WorkspaceIDForCustomDomain,
	} {
		if _, err := fn(ctx, 999999); err != ErrNotFound {
			t.Errorf("%s(nonexistent): expected ErrNotFound, got %v", name, err)
		}
	}
}

func TestWorkspaceMembershipCRUD(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	ownerID, err := st.CreateUser(ctx, "owner@example.com", "hash", "owner")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	ws, err := st.CreateWorkspace(ctx, "Production", "production", ownerID)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	// The creator is automatically an admin of what they created.
	role, ok, err := st.WorkspaceRole(ctx, ownerID, ws.ID)
	if err != nil || !ok || role != "admin" {
		t.Fatalf("expected creator to be admin, got role=%q ok=%v err=%v", role, ok, err)
	}

	memberID, err := st.CreateUser(ctx, "member@example.com", "hash", "member")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// A new user auto-joins the default workspace (id 1) as editor, but
	// has no membership in a workspace they weren't explicitly added to.
	if role, ok, err := st.WorkspaceRole(ctx, memberID, 1); err != nil || !ok || role != "editor" {
		t.Errorf("expected new user to be editor of the default workspace, got role=%q ok=%v err=%v", role, ok, err)
	}
	if _, ok, err := st.WorkspaceRole(ctx, memberID, ws.ID); err != nil || ok {
		t.Errorf("expected no membership in a workspace never explicitly joined, got ok=%v err=%v", ok, err)
	}

	if err := st.AddWorkspaceMember(ctx, ws.ID, memberID, "viewer"); err != nil {
		t.Fatalf("AddWorkspaceMember: %v", err)
	}
	if err := st.AddWorkspaceMember(ctx, ws.ID, memberID, "viewer"); err != ErrDuplicate {
		t.Errorf("expected ErrDuplicate re-adding an existing member, got %v", err)
	}

	if err := st.SetWorkspaceMemberRole(ctx, ws.ID, memberID, "editor"); err != nil {
		t.Fatalf("SetWorkspaceMemberRole: %v", err)
	}
	if role, ok, err := st.WorkspaceRole(ctx, memberID, ws.ID); err != nil || !ok || role != "editor" {
		t.Errorf("expected role editor after update, got role=%q ok=%v err=%v", role, ok, err)
	}
	if err := st.SetWorkspaceMemberRole(ctx, ws.ID, 999999, "editor"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound updating a nonexistent membership, got %v", err)
	}

	members, err := st.ListWorkspaceMembers(ctx, ws.ID)
	if err != nil {
		t.Fatalf("ListWorkspaceMembers: %v", err)
	}
	if len(members) != 2 { // owner (admin) + memberID (editor)
		t.Fatalf("expected 2 members, got %d: %+v", len(members), members)
	}

	if err := st.RemoveWorkspaceMember(ctx, ws.ID, memberID); err != nil {
		t.Fatalf("RemoveWorkspaceMember: %v", err)
	}
	if err := st.RemoveWorkspaceMember(ctx, ws.ID, memberID); err != ErrNotFound {
		t.Errorf("expected ErrNotFound removing an already-removed member, got %v", err)
	}
}
