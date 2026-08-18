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

func writePackageJSON(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectStrategyStaticFrontend(t *testing.T) {
	// Reproduces evanxdsouza/eco-friendly-survival: a Vite project with a
	// build script and no start script, which nixpacks can't run
	// ("Error: No start command could be found").
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
		"scripts": {"dev": "vite", "build": "vite build", "preview": "vite preview"},
		"dependencies": {"three": "^0.185.1"},
		"devDependencies": {"vite": "^8.2.1"}
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

func TestDetectStrategyStaticFrontendPicksYarn(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"scripts": {"build": "react-scripts build"}, "dependencies": {"react-scripts": "5.0.1"}}`)
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	result := detectStrategy(dir)
	if result.Strategy != StrategyStatic {
		t.Fatalf("Strategy = %v, want static", result.Strategy)
	}
	if result.StaticBuildCommand != "yarn build" {
		t.Errorf("StaticBuildCommand = %q, want %q", result.StaticBuildCommand, "yarn build")
	}
	if result.StaticOutputDir != "build" {
		t.Errorf("StaticOutputDir = %q, want %q", result.StaticOutputDir, "build")
	}
}

func TestDetectStrategyNodeServerStaysNixpacks(t *testing.T) {
	// A real Node server (has a start script) must NOT be reclassified as
	// static -- that would silently ship a static site with no backend.
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"scripts": {"start": "node index.js", "build": "tsc"}, "dependencies": {"express": "^4.18.0"}}`)
	result := detectStrategy(dir)
	if result.Strategy != StrategyNixpacks {
		t.Fatalf("Strategy = %v, want nixpacks", result.Strategy)
	}
}

func TestDetectStrategyUnknownBuildToolStaysNixpacks(t *testing.T) {
	// A build script that isn't attributable to a known static bundler
	// falls through to nixpacks unchanged, rather than guessing an output
	// dir that's probably wrong.
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"scripts": {"build": "some-custom-tool build"}}`)
	result := detectStrategy(dir)
	if result.Strategy != StrategyNixpacks {
		t.Fatalf("Strategy = %v, want nixpacks", result.Strategy)
	}
}
