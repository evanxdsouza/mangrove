package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectExposedPort(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"no expose", "FROM node:20\nCMD [\"node\", \"index.js\"]\n", 0},
		{"single expose", "FROM golang:1.22\nEXPOSE 80\nCMD [\"./app\"]\n", 80},
		{"expose with protocol suffix", "FROM nginx\nEXPOSE 8080/tcp\n", 8080},
		{"last expose wins (multi-stage)", "FROM node AS build\nEXPOSE 3000\nFROM nginx\nEXPOSE 8080\n", 8080},
		{"indented expose", "FROM alpine\n    EXPOSE 9000\n", 9000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "Dockerfile")
			if err := os.WriteFile(path, []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			got := detectExposedPort(path)
			if got != tc.want {
				t.Errorf("detectExposedPort() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDetectStrategyDockerfileSuggestsPort(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\nEXPOSE 4000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result := detectStrategy(dir)
	if result.Strategy != StrategyDockerfile {
		t.Fatalf("Strategy = %v, want dockerfile", result.Strategy)
	}
	if result.SuggestedPort != 4000 {
		t.Errorf("SuggestedPort = %d, want 4000", result.SuggestedPort)
	}
}

func writePackageJSON(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html>"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectStrategyViteIsStatic(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
		"scripts": {"dev": "vite", "build": "vite build", "preview": "vite preview"},
		"devDependencies": {"vite": "^8.0.0"}
	}`)
	result := detectStrategy(dir)
	if result.Strategy != StrategyStatic {
		t.Fatalf("Strategy = %v, want static", result.Strategy)
	}
	if result.StaticBuildCommand != "npm run build" {
		t.Errorf("StaticBuildCommand = %q, want %q", result.StaticBuildCommand, "npm run build")
	}
	if result.StaticOutputDir != "dist" {
		t.Errorf("StaticOutputDir = %q, want %q", result.StaticOutputDir, "dist")
	}
}

func TestDetectStrategyCRAIsStatic(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
		"scripts": {"start": "react-scripts start", "build": "react-scripts build"},
		"dependencies": {"react-scripts": "5.0.1"}
	}`)
	result := detectStrategy(dir)
	if result.Strategy != StrategyStatic {
		t.Fatalf("Strategy = %v, want static", result.Strategy)
	}
	if result.StaticOutputDir != "build" {
		t.Errorf("StaticOutputDir = %q, want %q", result.StaticOutputDir, "build")
	}
}

func TestDetectStrategySSRFrameworkStaysNixpacks(t *testing.T) {
	// SvelteKit depends on vite directly but runs its own SSR server --
	// staticBundlerOutputDir must not win over an ssrFrameworkHints match.
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
		"scripts": {"build": "vite build"},
		"devDependencies": {"vite": "^5.0.0", "@sveltejs/kit": "^2.0.0"}
	}`)
	result := detectStrategy(dir)
	if result.Strategy != StrategyNixpacks {
		t.Fatalf("Strategy = %v, want nixpacks (SSR framework present)", result.Strategy)
	}
}

func TestDetectStrategyNodeServerStaysNixpacks(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
		"scripts": {"start": "node server.js", "build": "tsc"},
		"dependencies": {"express": "^4.18.0"}
	}`)
	result := detectStrategy(dir)
	if result.Strategy != StrategyNixpacks {
		t.Fatalf("Strategy = %v, want nixpacks (no static bundler dependency)", result.Strategy)
	}
}
