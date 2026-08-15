package store

import (
	"context"
	"path/filepath"
	"testing"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
)

func TestWorkspacesAndReplicaFields(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	// Workspace creation and listing with project counts.
	ws, err := st.CreateWorkspace(ctx, "Production", "production")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if ws.ID == 0 || ws.Name != "Production" || ws.Slug != "production" {
		t.Fatalf("CreateWorkspace returned bad row: %+v", ws)
	}

	// New projects can be placed in a specific workspace.
	proj, err := st.CreateProject(ctx, ws.ID, "API", "api", "the api")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if proj.WorkspaceID != ws.ID {
		t.Fatalf("expected project in workspace %d, got %d", ws.ID, proj.WorkspaceID)
	}

	counts, err := st.ListWorkspaceProjectCounts(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaceProjectCounts: %v", err)
	}
	if len(counts) != 2 { // default (id 1) + the new one
		t.Fatalf("expected 2 workspaces, got %d", len(counts))
	}
	found := false
	for _, c := range counts {
		if c.Workspace.ID == ws.ID && c.ProjectCount == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("new workspace not listed with its project count: %+v", counts)
	}

	// Projects listing by workspace.
	byWS, err := st.ListProjectsByWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("ListProjectsByWorkspace: %v", err)
	}
	if len(byWS) != 1 || byWS[0].ID != proj.ID || byWS[0].WorkspaceName != "Production" {
		t.Fatalf("ListProjectsByWorkspace returned wrong rows: %+v", byWS)
	}

	// Moving a project between workspaces.
	if err := st.SetProjectWorkspace(ctx, proj.ID, 1); err != nil {
		t.Fatalf("SetProjectWorkspace: %v", err)
	}
	moved, err := st.GetProject(ctx, proj.ID)
	if err != nil || moved.WorkspaceID != 1 {
		t.Fatalf("project not moved to default workspace: %+v err=%v", moved, err)
	}

	// Deleting a workspace moves its projects back to default rather than orphaning.
	ws2, err := st.CreateWorkspace(ctx, "Staging", "staging")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if _, err := st.CreateProject(ctx, ws2.ID, "Web", "web", ""); err != nil {
		t.Fatalf("CreateProject in staging: %v", err)
	}
	if err := st.DeleteWorkspace(ctx, ws2.ID); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}
	all, err := st.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	for _, p := range all {
		if p.Slug == "web" && p.WorkspaceID != 1 {
			t.Fatalf("project not rescued to default workspace on workspace delete: %+v", p)
		}
	}

	// Replica count on deployments (defaulting to 1 when omitted).
	dep, err := st.CreateDeployment(ctx, CreateDeploymentParams{
		ProjectID:     proj.ID,
		Name:          "api",
		Slug:          "api",
		BuildStrategy: "dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if dep.Replicas != 1 {
		t.Fatalf("expected default replicas=1, got %d", dep.Replicas)
	}
	if err := st.UpdateDeploymentReplicas(ctx, dep.ID, 3); err != nil {
		t.Fatalf("UpdateDeploymentReplicas: %v", err)
	}
	dep2, _ := st.GetDeployment(ctx, dep.ID)
	if dep2.Replicas != 3 {
		t.Fatalf("expected replicas=3, got %d", dep2.Replicas)
	}

	// Service replica container IDs round-trip, keeping container_id_current = primary.
	svc, err := st.CreateService(ctx, CreateServiceParams{
		DeploymentID:  dep.ID,
		Name:          "web",
		ContainerName: "mangrove-api-web",
		InternalPort:  8080,
	})
	if err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	if err := st.UpdateServiceReplicas(ctx, svc.ID, []string{"aaa", "bbb", "ccc"}); err != nil {
		t.Fatalf("UpdateServiceReplicas: %v", err)
	}
	svc2, err := st.GetService(ctx, svc.ID)
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if svc2.ContainerIDCurrent != "aaa" {
		t.Fatalf("expected primary container_id_current=aaa, got %q", svc2.ContainerIDCurrent)
	}
	if len(svc2.ReplicaContainerIDs) != 3 || svc2.ReplicaContainerIDs[0] != "aaa" || svc2.ReplicaContainerIDs[2] != "ccc" {
		t.Fatalf("expected replica ids [aaa bbb ccc], got %v", svc2.ReplicaContainerIDs)
	}

	// A single-element (or empty) set clears the replica list.
	if err := st.UpdateServiceReplicas(ctx, svc.ID, []string{"only"}); err != nil {
		t.Fatalf("UpdateServiceReplicas single: %v", err)
	}
	svc3, _ := st.GetService(ctx, svc.ID)
	if svc3.ContainerIDCurrent != "only" || len(svc3.ReplicaContainerIDs) != 0 {
		t.Fatalf("expected single-element set to clear replicas, got %+v", svc3)
	}
}
