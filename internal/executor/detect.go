package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// DetectedEnvVar is one key the "Deploy from GitHub" wizard should prompt
// the user for -- Secret is only a heuristic default (based on the key's
// name), the caller lets the user flip it before saving.
type DetectedEnvVar struct {
	Key    string `json:"key"`
	Secret bool   `json:"secret"`
}

type DetectionResult struct {
	Strategy       BuildStrategy    `json:"build_strategy"`
	DockerfilePath string           `json:"dockerfile_path,omitempty"`
	ComposePath    string           `json:"compose_path,omitempty"`
	EnvVars        []DetectedEnvVar `json:"env_vars,omitempty"`
}

// DetectBuildStrategy shallow-clones src (the same materialize() step a
// real build already does) and inspects rootPath -- a subdirectory within
// the clone, "." for the repo root -- to guess a build strategy and the env
// vars the app expects. Best-effort by design: the wizard that calls this
// always lets the user review and override the result before deploying, so
// a wrong guess costs a form edit, not a broken deploy.
func DetectBuildStrategy(ctx context.Context, src ContextSource, rootPath string) (DetectionResult, error) {
	dir, cleanup, err := materialize(ctx, src)
	if err != nil {
		return DetectionResult{}, err
	}
	defer cleanup()

	base := dir
	if rootPath != "" && rootPath != "." {
		base = filepath.Join(dir, rootPath)
	}

	result := detectStrategy(base)
	result.EnvVars = detectEnvVars(base)
	return result, nil
}

// detectStrategy checks, in order, for the strongest unambiguous signal
// first: a compose file beats a Dockerfile beats "no server-side code at
// all" beats the generic buildpack fallback.
func detectStrategy(dir string) DetectionResult {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if isFile(filepath.Join(dir, name)) {
			return DetectionResult{Strategy: StrategyCompose, ComposePath: name}
		}
	}
	if isFile(filepath.Join(dir, "Dockerfile")) {
		return DetectionResult{Strategy: StrategyDockerfile, DockerfilePath: "Dockerfile"}
	}
	// A plain HTML site with no package.json (i.e. no build step, nothing
	// server-side) -- anything with a package.json is left to nixpacks
	// below, since "static frontend" vs. "Node server" can't be told apart
	// from file presence alone.
	if isFile(filepath.Join(dir, "index.html")) && !isFile(filepath.Join(dir, "package.json")) {
		return DetectionResult{Strategy: StrategyStatic}
	}
	// Generic fallback: nixpacks is Mangrove's "no Dockerfile needed"
	// buildpack strategy, and handles most common stacks (Node, Python, Go,
	// Ruby, ...) without further guessing.
	return DetectionResult{Strategy: StrategyNixpacks}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// detectEnvVars parses .env.example-style files at the repo root -- the
// closest thing to a de facto convention for "here are the env vars this
// app needs" that doesn't require guessing at framework-specific idioms.
func detectEnvVars(dir string) []DetectedEnvVar {
	for _, name := range []string{".env.example", ".env.sample", ".env.dist"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		return parseEnvExample(string(data))
	}
	return nil
}

var secretKeyHints = []string{"SECRET", "TOKEN", "PASSWORD", "PRIVATE", "CREDENTIAL", "AUTH", "_KEY", "APIKEY"}

// parseEnvExample reads KEY=value lines (value ignored -- .env.example
// values are usually placeholders, not real defaults worth carrying over).
func parseEnvExample(content string) []DetectedEnvVar {
	var out []DetectedEnvVar
	seen := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" || seen[key] || !isEnvKeyName(key) {
			continue
		}
		seen[key] = true
		out = append(out, DetectedEnvVar{Key: key, Secret: looksSecret(key)})
	}
	return out
}

func isEnvKeyName(key string) bool {
	for _, r := range key {
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func looksSecret(key string) bool {
	upper := strings.ToUpper(key)
	for _, hint := range secretKeyHints {
		if strings.Contains(upper, hint) {
			return true
		}
	}
	return false
}
