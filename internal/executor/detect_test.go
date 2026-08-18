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
