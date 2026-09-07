package executor

import (
	"context"
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
	result := detectStrategy(context.Background(), dir)
	if result.Strategy != StrategyDockerfile {
		t.Fatalf("Strategy = %v, want dockerfile", result.Strategy)
	}
	if result.SuggestedPort != 4000 {
		t.Errorf("SuggestedPort = %d, want 4000", result.SuggestedPort)
	}
}

func TestDetectStrategyFallsBackToStaticWithNoStartCommand(t *testing.T) {
	requireNixpacks(t)
	dir := t.TempDir()
	pkgJSON := `{"name":"readme","scripts":{"build":"echo building"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectStrategy(context.Background(), dir)
	if result.Strategy != StrategyStatic {
		t.Fatalf("Strategy = %v, want static", result.Strategy)
	}
	if result.StaticBuildCommand == "" {
		t.Error("expected a non-empty StaticBuildCommand")
	}
	if result.StaticOutputDir != "dist" {
		t.Errorf("StaticOutputDir = %q, want %q", result.StaticOutputDir, "dist")
	}
}

func TestDetectStrategyKeepsNixpacksWithStartCommand(t *testing.T) {
	requireNixpacks(t)
	dir := t.TempDir()
	pkgJSON := `{"name":"app","scripts":{"build":"echo building","start":"node index.js"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectStrategy(context.Background(), dir)
	if result.Strategy != StrategyNixpacks {
		t.Fatalf("Strategy = %v, want nixpacks", result.Strategy)
	}
}

func TestGuessStaticOutputDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react-scripts":"5.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := guessStaticOutputDir(dir); got != "build" {
		t.Errorf("guessStaticOutputDir() = %q, want %q", got, "build")
	}
}
