package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// TestUpdateDeploymentBuildConfig covers the fix for a deployment whose
// build_strategy was mis-detected by "Deploy from GitHub" at creation time
// (e.g. a Vite frontend guessed as nixpacks, which fails every build with
// "no start command could be found") -- before this endpoint existed there
// was no way to change it short of deleting and recreating the deployment.
func TestUpdateDeploymentBuildConfig(t *testing.T) {
	env := newRoleTestEnv(t)
	ownerCookie, _ := env.cookieFor(t, "owner@example.com", "owner")
	ctx := context.Background()

	proj, err := env.store.CreateProject(ctx, 1, "Static Fix Test", "static-fix-test", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dep, err := env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: proj.ID, Name: "web", Slug: "web", BuildStrategy: "nixpacks",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	rec := env.do(http.MethodPost, "/api/deployments/"+itoa(dep.ID)+"/build-config", ownerCookie, map[string]any{
		"build_strategy":       "static",
		"root_path":            ".",
		"static_build_command": "npm run build",
		"static_output_dir":    "dist",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	got, err := env.store.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if got.BuildStrategy != "static" {
		t.Errorf("BuildStrategy = %q, want %q", got.BuildStrategy, "static")
	}
	if got.StaticBuildCommand != "npm run build" {
		t.Errorf("StaticBuildCommand = %q, want %q", got.StaticBuildCommand, "npm run build")
	}
	if got.StaticOutputDir != "dist" {
		t.Errorf("StaticOutputDir = %q, want %q", got.StaticOutputDir, "dist")
	}
}

func TestUpdateDeploymentBuildConfigRejectsUnknownStrategy(t *testing.T) {
	env := newRoleTestEnv(t)
	ownerCookie, _ := env.cookieFor(t, "owner@example.com", "owner")
	ctx := context.Background()

	proj, err := env.store.CreateProject(ctx, 1, "Bad Strategy Test", "bad-strategy-test", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dep, err := env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: proj.ID, Name: "web", Slug: "web", BuildStrategy: "nixpacks",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	rec := env.do(http.MethodPost, "/api/deployments/"+itoa(dep.ID)+"/build-config", ownerCookie, map[string]any{
		"build_strategy": "not-a-real-strategy",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown strategy, got %d: %s", rec.Code, rec.Body.String())
	}
}
