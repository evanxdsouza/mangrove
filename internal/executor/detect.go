package executor

import (
	"context"
	"encoding/json"
	"os"
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
	// StaticBuildCommand/StaticOutputDir are set alongside Strategy ==
	// static when detection recognized a package.json-driven static
	// frontend (see detectStaticFrontend) -- the wizard prefills the
	// static-strategy build command/output dir fields with these instead
	// of leaving StaticBuildCommand blank, which would skip the build
	// step entirely and ship raw, unbundled source.
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
	if path := filepath.Join(dir, "Dockerfile"); isFile(path) {
		return DetectionResult{Strategy: StrategyDockerfile, DockerfilePath: "Dockerfile", SuggestedPort: detectExposedPort(path)}
	}
	// A plain HTML site with no package.json (i.e. no build step, nothing
	// server-side) needs no further guessing.
	if isFile(filepath.Join(dir, "index.html")) && !isFile(filepath.Join(dir, "package.json")) {
		return DetectionResult{Strategy: StrategyStatic}
	}
	// A package.json alone doesn't say "Node server" -- a build-tool
	// frontend (Vite, CRA, Vue CLI, ...) has one too, but produces static
	// files and has no start script for nixpacks to run: see
	// detectStaticFrontend.
	if result, ok := detectStaticFrontend(dir); ok {
		return result
	}
	// Generic fallback: nixpacks is Mangrove's "no Dockerfile needed"
	// buildpack strategy, and handles most common stacks (Node, Python, Go,
	// Ruby, ...) without further guessing.
	return DetectionResult{Strategy: StrategyNixpacks}
}

// packageJSON is the subset of package.json fields detectStaticFrontend
// needs -- scripts to tell "has a server to start" from "just builds", and
// dependencies to name which bundler produced the build.
type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// staticBundlerOutputDirs maps a handful of common frontend build tools
// (identified by their package.json dependency name) to the output
// directory they write to by default. Deliberately excludes anything that
// can plausibly run as a Node *server* too (webpack alone, SvelteKit's
// adapter-node, Next.js) -- false-negative (falls through to nixpacks,
// same as before this existed) is the safe failure mode here, not
// false-positive (misclassifying a real server as static, which would
// silently ship a static site with no backend at all).
var staticBundlerOutputDirs = map[string]string{
	"vite":             "dist",
	"react-scripts":    "build",
	"@vue/cli-service": "dist",
	"@angular/cli":     "dist",
	"gatsby":           "public",
	"parcel":           "dist",
	"astro":            "dist",
	"@11ty/eleventy":   "_site",
}

// detectStaticFrontend recognizes a package.json-driven static frontend --
// a build script that runs one of staticBundlerOutputDirs' bundlers, and no
// start script (nixpacks would otherwise have nothing to run: "Error: No
// start command could be found"). ok is false when package.json is
// missing, unparseable, or doesn't match this pattern, leaving the caller's
// nixpacks fallback in place.
func detectStaticFrontend(dir string) (DetectionResult, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return DetectionResult{}, false
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return DetectionResult{}, false
	}
	if pkg.Scripts["start"] != "" || pkg.Scripts["build"] == "" {
		return DetectionResult{}, false
	}
	outputDir := ""
	for dep, out := range staticBundlerOutputDirs {
		if _, ok := pkg.Dependencies[dep]; ok {
			outputDir = out
			break
		}
		if _, ok := pkg.DevDependencies[dep]; ok {
			outputDir = out
			break
		}
	}
	if outputDir == "" {
		return DetectionResult{}, false
	}
	return DetectionResult{
		Strategy:           StrategyStatic,
		StaticBuildCommand: buildCommand(dir),
		StaticOutputDir:    outputDir,
	}, true
}

// buildCommand picks "npm run build" vs. the equivalent yarn/pnpm
// invocation based on which lockfile is present -- nixpacks (which
// actually runs the static build, see docker.go's StaticBuildCommand
// handling) needs the right package manager to install with too.
func buildCommand(dir string) string {
	switch {
	case isFile(filepath.Join(dir, "pnpm-lock.yaml")):
		return "pnpm run build"
	case isFile(filepath.Join(dir, "yarn.lock")):
		return "yarn build"
	default:
		return "npm run build"
	}
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
