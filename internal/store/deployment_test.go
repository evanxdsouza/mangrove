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
