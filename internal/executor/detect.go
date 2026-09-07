package executor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	// SuggestedPort, when > 0, is the port the app looks like it listens
	// on (currently: a Dockerfile's last EXPOSE instruction) -- the
	// wizard's blind default of 3000 is wrong for most repos otherwise,
	// since it has no relationship to the port the app actually binds.
	SuggestedPort int `json:"suggested_port,omitempty"`
	// StaticBuildCommand and StaticOutputDir are only set when Strategy ==
	// static because a package.json build produced no start command -- see
	// detectNixpacksStaticFallback.
	StaticBuildCommand string `json:"static_build_command,omitempty"`
	StaticOutputDir    string `json:"static_output_dir,omitempty"`
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

	result := detectStrategy(ctx, base)
	result.EnvVars = detectEnvVars(base)
	return result, nil
}

// detectStrategy checks, in order, for the strongest unambiguous signal
// first: a compose file beats a Dockerfile beats "no server-side code at
// all" beats the generic buildpack fallback.
func detectStrategy(ctx context.Context, dir string) DetectionResult {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if isFile(filepath.Join(dir, name)) {
			return DetectionResult{Strategy: StrategyCompose, ComposePath: name}
		}
	}
	if path := filepath.Join(dir, "Dockerfile"); isFile(path) {
		return DetectionResult{Strategy: StrategyDockerfile, DockerfilePath: "Dockerfile", SuggestedPort: detectExposedPort(path)}
	}
	// A plain HTML site with no package.json (i.e. no build step, nothing
	// server-side) -- anything with a package.json is left to nixpacks
	// below, since "static frontend" vs. "Node server" can't be told apart
	// from file presence alone.
	if isFile(filepath.Join(dir, "index.html")) && !isFile(filepath.Join(dir, "package.json")) {
		return DetectionResult{Strategy: StrategyStatic}
	}
	// Some package.json apps are static frontends nixpacks can't recognize
	// as such (a bare `bun run build`, a framework it has no SSR provider
	// for): they build fine but have no start command, and a real deploy
	// would fail at `nixpacks build` with "No start command could be
	// found". Ask nixpacks itself (rather than re-implementing its
	// framework-detection matrix) and fall back to Mangrove's Static
	// strategy when that's the case.
	if isFile(filepath.Join(dir, "package.json")) {
		if result, ok := detectNixpacksStaticFallback(ctx, dir); ok {
			return result
		}
	}
	// Generic fallback: nixpacks is Mangrove's "no Dockerfile needed"
	// buildpack strategy, and handles most common stacks (Node, Python, Go,
	// Ruby, ...) without further guessing.
	return DetectionResult{Strategy: StrategyNixpacks}
}

// nixpacksPlanOutput is the subset of `nixpacks plan`'s JSON fields
// detectNixpacksStaticFallback needs.
type nixpacksPlanOutput struct {
	Phases map[string]struct {
		Cmds []string `json:"cmds"`
	} `json:"phases"`
	Start *struct {
		Cmd string `json:"cmd"`
	} `json:"start"`
}

// detectNixpacksStaticFallback runs `nixpacks plan` (static analysis only --
// it doesn't run installs or need network/Nix) against dir and reports
// whether nixpacks found no start command but did find a build command,
// meaning the app is really a static site nixpacks doesn't recognize as
// one. ok is false when nixpacks found a start command (the ordinary
// nixpacks strategy is correct), when it found neither a start nor a build
// command (nothing to build a static fallback out of), or when `nixpacks
// plan` itself failed (e.g. the CLI isn't installed) -- in every one of
// those cases the caller keeps its original generic nixpacks guess.
func detectNixpacksStaticFallback(ctx context.Context, dir string) (DetectionResult, bool) {
	out, err := exec.CommandContext(ctx, "nixpacks", "plan", dir).Output()
	if err != nil {
		return DetectionResult{}, false
	}
	var plan nixpacksPlanOutput
	if err := json.Unmarshal(out, &plan); err != nil {
		return DetectionResult{}, false
	}
	if plan.Start != nil {
		return DetectionResult{}, false
	}
	build, ok := plan.Phases["build"]
	if !ok || len(build.Cmds) == 0 {
		return DetectionResult{}, false
	}
	return DetectionResult{
		Strategy:           StrategyStatic,
		StaticBuildCommand: strings.Join(build.Cmds, " && "),
		StaticOutputDir:    guessStaticOutputDir(dir),
	}, true
}

// guessStaticOutputDir guesses the directory a static build writes to --
// "dist" covers the large majority of bundlers (Vite, Rollup, esbuild,
// Astro, ...); Create React App's "react-scripts build" is the one common
// holdout that uses "build" instead, so it gets special-cased. The wizard
// that surfaces this always lets the user correct it before deploying.
func guessStaticOutputDir(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err == nil && strings.Contains(string(data), `"react-scripts"`) {
		return "build"
	}
	return "dist"
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

var exposeRe = regexp.MustCompile(`(?im)^\s*EXPOSE\s+(\d+)`)

// detectExposedPort returns a Dockerfile's last EXPOSE port (the last one
// wins, matching how multi-stage Dockerfiles are read top to bottom;
// EXPOSE's optional "/tcp"/"/udp" suffix and any additional ports on the
// same line are ignored -- picking the first port on the line is enough
// for the common single-port case this is meant to cover). 0 means no
// EXPOSE instruction was found, and the caller falls back to its own
// default.
func detectExposedPort(dockerfilePath string) int {
	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return 0
	}
	matches := exposeRe.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		return 0
	}
	port, err := strconv.Atoi(matches[len(matches)-1][1])
	if err != nil {
		return 0
	}
	return port
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
