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
	// StaticBuildCommand and StaticOutputDir are only set alongside
	// Strategy == static when detectStaticFrontendBuild recognized a known
	// static-only bundler (e.g. Vite, Create React App) -- the wizard
	// pre-fills its build-command/output-dir fields from these but still
	// lets the user review and change them before deploying.
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
	// server-side).
	if isFile(filepath.Join(dir, "index.html")) && !isFile(filepath.Join(dir, "package.json")) {
		return DetectionResult{Strategy: StrategyStatic}
	}
	// A package.json alone can't tell "static frontend" apart from "Node
	// server" by presence alone -- but a handful of well-known build tools
	// (Vite, Create React App, Parcel, ...) never run a production server of
	// their own, so a repo built by one of them is static regardless of
	// having a package.json. Handing these to nixpacks instead reliably
	// fails the first deploy: nixpacks' plan detection finds no start
	// command (there was never going to be one) and errors out only after
	// the build has already run.
	if buildCmd, outputDir, ok := detectStaticFrontendBuild(dir); ok {
		return DetectionResult{Strategy: StrategyStatic, StaticBuildCommand: buildCmd, StaticOutputDir: outputDir}
	}
	// Generic fallback: nixpacks is Mangrove's "no Dockerfile needed"
	// buildpack strategy, and handles most common stacks (Node, Python, Go,
	// Ruby, ...) without further guessing.
	return DetectionResult{Strategy: StrategyNixpacks}
}

// staticBundlerOutputDir maps a small set of well-known frontend build tools
// that never run a production server of their own to their default build
// output directory. Deliberately narrow and allowlist-based: tools with an
// SSR/server mode of their own (Next, Nuxt, SvelteKit, Astro, ...) are
// excluded via ssrFrameworkHints below even when one of these also appears
// as a transitive dependency (e.g. SvelteKit apps depend on vite directly),
// since guessing wrong there means a working server-rendered app silently
// gets deployed as a static file dump instead of falling through to the
// nixpacks path the user can review and correct.
var staticBundlerOutputDir = map[string]string{
	"vite":          "dist",
	"react-scripts": "build", // create-react-app
	"parcel":        "dist",
	"@parcel/core":  "dist",
}

var ssrFrameworkHints = []string{
	"next", "nuxt", "nuxt3", "@remix-run/dev", "@remix-run/react",
	"@sveltejs/kit", "astro", "@builder.io/qwik-city", "solid-start",
	"express", "fastify", "koa", "@nestjs/core", "@hapi/hapi",
}

type packageJSONForDetect struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// detectStaticFrontendBuild reports whether dir's package.json belongs to a
// static-only frontend build (a "build" script, one of staticBundlerOutputDir's
// tools as a dependency, and none of ssrFrameworkHints present), returning
// the install+build command to run and the tool's default output directory.
func detectStaticFrontendBuild(dir string) (buildCmd, outputDir string, ok bool) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", "", false
	}
	var pkg packageJSONForDetect
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", "", false
	}
	if pkg.Scripts["build"] == "" {
		return "", "", false
	}

	deps := make(map[string]bool, len(pkg.Dependencies)+len(pkg.DevDependencies))
	for name := range pkg.Dependencies {
		deps[name] = true
	}
	for name := range pkg.DevDependencies {
		deps[name] = true
	}

	for _, hint := range ssrFrameworkHints {
		if deps[hint] {
			return "", "", false
		}
	}
	for tool, outDir := range staticBundlerOutputDir {
		if deps[tool] {
			return "npm run build", outDir, true
		}
	}
	return "", "", false
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
