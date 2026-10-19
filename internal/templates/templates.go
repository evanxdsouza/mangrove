// Package templates holds Mangrove's built-in one-click deployment
// templates. They ship embedded in the binary as plain JSON (not database
// rows) specifically so adding one is "add a file, rebuild" -- no
// migration, no seed script, no admin UI to manage them.
//
// A template describes one or more deployments to create together (in
// order): most are a single deployment, but a template can declare a
// linked dependency (e.g. WordPress's MySQL, Umami's Postgres) as a second
// deployment in the same project, resolved via a stable Docker network
// alias rather than folded into one multi-container deployment -- see
// internal/orchestrator/templates.go, which is where these get turned into
// actual deployment/service/volume/env rows and deployed through the exact
// same Deploy() path (and so the same admission control and port registry)
// as anything a user configures by hand.
package templates

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed data/*.json
var dataFS embed.FS

// EnvVar describes one environment variable on a template deployment's
// service. Exactly one of Value, Generate, or Prompt is set:
//   - Generate ("password" or "hex32") creates a random value at install
//     time and stores it under GenerateKey, so any deployment later in the
//     same template can reference it (see Value's {{generated:...}} below).
//   - Value is a literal, optionally containing placeholders resolved at
//     install time: "{{alias:<slug_suffix>}}" (a sibling deployment's
//     stable container/network-alias name), "{{generated:<generate_key>}}"
//     (a value produced by an earlier deployment's Generate), "{{slug}}"
//     (this deployment's own final slug), or "{{base_domain}}"
//     (Config.BaseDomain). Placeholders can appear inline within a larger
//     string (e.g. a DATABASE_URL combining a generated password with a
//     sibling's alias), not just as the whole value.
//   - Prompt asks the installing user for this value in the install form
//     instead of deriving it from the template itself -- for things a
//     template can't sensibly default (API tokens, webhook secrets). Label
//     is optional UI copy for the prompt; Required, if set, blocks install
//     until a non-empty value is supplied (see InstallTemplate). Prompted
//     values are never stored as part of the template -- they're passed in
//     by the caller at install time and substituted the same way a
//     generated or literal value would be.
type EnvVar struct {
	Key         string `json:"key"`
	Value       string `json:"value,omitempty"`
	Generate    string `json:"generate,omitempty"`
	GenerateKey string `json:"generate_key,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
	Prompt      bool   `json:"prompt,omitempty"`
	Label       string `json:"label,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Volume describes one named volume to create and mount into the service.
type Volume struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
}

// File is one inline file a template deployment mounts into its container
// (read-only) before it first starts -- e.g. a Postgres
// entrypoint-initdb.d script Supabase's image needs but that a template
// can't express any other way. Content may use the same placeholders as an
// env var's value ({{slug}}, {{base_slug}}, {{base_domain}},
// {{alias:<slug_suffix>}}, {{generated:<generate_key>}}), resolved at
// install time. Files are only materialized during the template install
// itself (see executor.FileMount); a later plain redeploy of the resulting
// deployment carries no files, which is fine because they exist to seed
// first boot (e.g. role-password setup that only runs on an empty database).
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Deployment describes one deployment a template creates. Most built-in
// templates use BuildStrategy "image" (they deploy a pre-built image, no
// build step). BuildStrategy "dockerfile" instead builds from GitURL (and
// optional GitBranch) the same way a hand-created git-backed deployment
// does -- see InstallTemplate, which threads these into the same Deploy()
// call a normal dockerfile-strategy deployment uses, including an empty
// AuthToken (an unauthenticated clone), so this only works for a public
// repo.
type Deployment struct {
	// SlugSuffix is appended to the user-chosen base slug to form this
	// deployment's slug ("" for the template's primary/only deployment,
	// e.g. "-mysql" for a linked dependency). Exactly one deployment in a
	// template must have SlugSuffix == "", and it must be last -- see
	// Registry's validation.
	SlugSuffix string `json:"slug_suffix"`
	NameSuffix string `json:"name_suffix"`
	// BuildStrategy defaults to "image" if empty. "image" requires ImageRef;
	// "dockerfile" requires GitURL (GitBranch is optional -- an empty
	// GitBranch clones the repo's actual default branch, same as leaving
	// GitRef empty on a normal deploy).
	BuildStrategy string `json:"build_strategy"`
	ImageRef      string `json:"image_ref,omitempty"`
	GitURL        string `json:"git_url,omitempty"`
	GitBranch     string `json:"git_branch,omitempty"`
	InternalPort  int    `json:"internal_port"`
	// ForceInternalOnly is true for anything speaking a raw TCP protocol
	// (Postgres, MySQL, MongoDB, Redis) rather than HTTP: Caddy's routing
	// only reverse-proxies/file-serves HTTP (see internal/proxy), so there
	// is no working way to expose these publicly through it, and they are
	// meant to be reached over the Docker network by a sibling deployment
	// instead. Deployments without this set still default to internal-only
	// at creation (matching every other Mangrove deployment's default) but
	// remain user-togglable afterward via the existing access-control
	// endpoint.
	ForceInternalOnly bool     `json:"force_internal_only"`
	MemoryLimitMB     int      `json:"memory_limit_mb"`
	CPULimitCores     float64  `json:"cpu_limit_cores"`
	HealthCheckPath   string   `json:"health_check_path,omitempty"`
	Command           []string `json:"command,omitempty"`
	Volumes           []Volume `json:"volumes,omitempty"`
	Env               []EnvVar `json:"env,omitempty"`
	Files             []File   `json:"files,omitempty"`
	// PostDeployCommands are exec-form commands (like Command, but run via
	// `docker exec` rather than as the container's CMD) executed against
	// this deployment's own container immediately after it deploys
	// successfully -- e.g. seeding extra admin accounts an image's startup
	// script has no way to create on its own (see pocketbase.json). Each
	// token is resolved for the same placeholders a Files entry supports,
	// plus "{{env:<key>}}" -- this deployment's own resolved value for that
	// env key (literal, generated, or prompted), so an optional prompted
	// value can be threaded into a seed command. Not persisted anywhere;
	// exists only during InstallTemplate, same as Files. A nonzero exit
	// fails the install the same way a bad Deploy() does, so write each
	// command defensively (e.g. skip cleanly when an optional prompted
	// value was left empty).
	PostDeployCommands [][]string `json:"post_deploy_commands,omitempty"`
}

// Template is one entry in the gallery.
type Template struct {
	Key          string       `json:"key"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Category     string       `json:"category"`
	Deployments  []Deployment `json:"deployments"`
	SourceRepo   string       `json:"-"`
	SourceCommit string       `json:"-"`
}

// TotalMemoryMB is the combined memory_limit_mb across every deployment
// this template creates -- shown in the gallery so a user can see whether
// it'll fit their remaining admission-control budget before installing.
func (t Template) TotalMemoryMB() int {
	total := 0
	for _, d := range t.Deployments {
		total += d.MemoryLimitMB
	}
	return total
}

var defaultRegistry *Registry

// Registry is an immutable-after-startup catalog of built-in and
// operator-configured community templates.
type Registry struct {
	entries map[string]Template
	ordered []Template
}

// DefaultRegistry returns the process-wide built-in catalog. Runtime
// community catalogs should be constructed with LoadCommunitySources.
func DefaultRegistry() *Registry { return defaultRegistry }

func NewRegistry() *Registry {
	entries := make(map[string]Template, len(defaultRegistry.entries))
	ordered := make([]Template, len(defaultRegistry.ordered))
	copy(ordered, defaultRegistry.ordered)
	for key, value := range defaultRegistry.entries {
		entries[key] = value
	}
	return &Registry{entries: entries, ordered: ordered}
}

// List returns every template sorted by display name, then key.
func (r *Registry) List() []Template {
	if r == nil {
		return nil
	}
	out := make([]Template, len(r.ordered))
	copy(out, r.ordered)
	return out
}

// Get looks up a template by key.
func (r *Registry) Get(key string) (Template, bool) {
	if r == nil {
		return Template{}, false
	}
	t, ok := r.entries[key]
	return t, ok
}

// AddCommunity validates a repository's complete set before adding any of
// it. Remote keys are namespaced to avoid collisions with built-ins and each
// other; the original key is retained only in the source manifest.
func (r *Registry) AddCommunity(owner, repo, commit string, incoming []Template) error {
	if r == nil {
		return fmt.Errorf("nil template registry")
	}
	if len(incoming) == 0 || len(incoming) > maxCommunityTemplates {
		return fmt.Errorf("repository must provide between 1 and %d templates", maxCommunityTemplates)
	}
	prefix := "community-" + slugPart(owner) + "-" + slugPart(repo) + "-"
	pending := make([]Template, 0, len(incoming))
	seen := make(map[string]bool, len(incoming))
	for _, t := range incoming {
		if err := validate(t); err != nil {
			return fmt.Errorf("template %q: %w", t.Key, err)
		}
		if !communityKeyRE.MatchString(t.Key) {
			return fmt.Errorf("template key %q must contain only lowercase letters, digits, and hyphens", t.Key)
		}
		t.Key = prefix + t.Key
		if len(t.Key) > 63 || !communityKeyRE.MatchString(t.Key) {
			return fmt.Errorf("namespaced template key %q is invalid or exceeds 63 characters", t.Key)
		}
		if _, exists := r.entries[t.Key]; exists || seen[t.Key] {
			return fmt.Errorf("duplicate template key %q", t.Key)
		}
		seen[t.Key] = true
		t.SourceRepo = owner + "/" + repo
		t.SourceCommit = strings.ToLower(commit)
		pending = append(pending, t)
	}
	for _, t := range pending {
		r.entries[t.Key] = t
		r.ordered = append(r.ordered, t)
	}
	sort.Slice(r.ordered, func(i, j int) bool {
		if r.ordered[i].Name == r.ordered[j].Name {
			return r.ordered[i].Key < r.ordered[j].Key
		}
		return r.ordered[i].Name < r.ordered[j].Name
	})
	return nil
}

func slugPart(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func init() {
	registry := make(map[string]Template)
	ordered := make([]Template, 0)

	entries, err := dataFS.ReadDir("data")
	if err != nil {
		panic(fmt.Sprintf("templates: read embedded data dir: %v", err))
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := dataFS.ReadFile("data/" + e.Name())
		if err != nil {
			panic(fmt.Sprintf("templates: read %s: %v", e.Name(), err))
		}
		var t Template
		if err := json.Unmarshal(raw, &t); err != nil {
			panic(fmt.Sprintf("templates: parse %s: %v", e.Name(), err))
		}
		if err := validate(t); err != nil {
			panic(fmt.Sprintf("templates: invalid %s: %v", e.Name(), err))
		}
		if _, dup := registry[t.Key]; dup {
			panic(fmt.Sprintf("templates: duplicate key %q (from %s)", t.Key, e.Name()))
		}
		registry[t.Key] = t
		ordered = append(ordered, t)
	}

	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	defaultRegistry = &Registry{entries: registry, ordered: ordered}
}

var generatedPlaceholderRe = regexp.MustCompile(`\{\{generated:([^}]+)\}\}`)
var envPlaceholderRe = regexp.MustCompile(`\{\{env:([^}]+)\}\}`)

// validate enforces the invariants InstallTemplate relies on: exactly one
// deployment is the template's primary (SlugSuffix == ""), it's last (so
// every dependency it might reference via {{alias:...}} or
// {{generated:...}} has already been created and deployed), every
// deployment has a build source (ImageRef for "image", GitURL for
// "dockerfile"), and every {{generated:key}} placeholder points at a
// GenerateKey that actually exists earlier in the list.
func validate(t Template) error {
	if t.Key == "" {
		return fmt.Errorf("missing key")
	}
	if len(t.Deployments) == 0 {
		return fmt.Errorf("template %q has no deployments", t.Key)
	}

	seenGenerateKeys := map[string]bool{}
	primaryCount := 0
	for i, d := range t.Deployments {
		if d.SlugSuffix == "" {
			primaryCount++
			if i != len(t.Deployments)-1 {
				return fmt.Errorf("template %q: primary deployment (slug_suffix \"\") must be last", t.Key)
			}
		}
		buildStrategy := d.BuildStrategy
		if buildStrategy == "" {
			buildStrategy = "image"
		}
		switch buildStrategy {
		case "dockerfile":
			if d.GitURL == "" {
				return fmt.Errorf("template %q deployment %d: build_strategy \"dockerfile\" requires git_url", t.Key, i)
			}
		default:
			if d.ImageRef == "" {
				return fmt.Errorf("template %q deployment %d: missing image_ref", t.Key, i)
			}
		}
		if d.MemoryLimitMB <= 0 {
			return fmt.Errorf("template %q deployment %d: memory_limit_mb must be positive", t.Key, i)
		}
		for _, ev := range d.Env {
			set := 0
			if ev.Generate != "" {
				set++
			}
			if ev.Value != "" {
				set++
			}
			if ev.Prompt {
				set++
			}
			if set > 1 {
				return fmt.Errorf("template %q deployment %d: env %q must set only one of generate, value, or prompt", t.Key, i, ev.Key)
			}
			if ev.Prompt {
				continue // resolved from caller-supplied overrides at install time, not from the template
			}
			// A generate kind can carry its own {{generated:...}} reference
			// (e.g. a Supabase "jwt:anon:{{generated:jwt_secret}}" key signing
			// with an earlier-generated secret), so check those too.
			for _, m := range generatedPlaceholderRe.FindAllStringSubmatch(ev.Generate, -1) {
				if !seenGenerateKeys[m[1]] {
					return fmt.Errorf("template %q deployment %d: env %q generate references undefined generate_key %q", t.Key, i, ev.Key, m[1])
				}
			}
			for _, m := range generatedPlaceholderRe.FindAllStringSubmatch(ev.Value, -1) {
				if !seenGenerateKeys[m[1]] {
					return fmt.Errorf("template %q deployment %d: env %q references undefined generate_key %q", t.Key, i, ev.Key, m[1])
				}
			}
			if ev.GenerateKey != "" {
				seenGenerateKeys[ev.GenerateKey] = true
			}
		}
		for _, f := range d.Files {
			if f.Path == "" || f.Content == "" {
				return fmt.Errorf("template %q deployment %d: file must have both a path and content", t.Key, i)
			}
			for _, m := range generatedPlaceholderRe.FindAllStringSubmatch(f.Content, -1) {
				if !seenGenerateKeys[m[1]] {
					return fmt.Errorf("template %q deployment %d: file %q references undefined generate_key %q", t.Key, i, f.Path, m[1])
				}
			}
		}
		envKeys := map[string]bool{}
		for _, ev := range d.Env {
			envKeys[ev.Key] = true
		}
		for _, cmd := range d.PostDeployCommands {
			if len(cmd) == 0 {
				return fmt.Errorf("template %q deployment %d: post_deploy_commands entry must not be empty", t.Key, i)
			}
			for _, tok := range cmd {
				for _, m := range envPlaceholderRe.FindAllStringSubmatch(tok, -1) {
					if !envKeys[m[1]] {
						return fmt.Errorf("template %q deployment %d: post_deploy_commands references undefined env key %q", t.Key, i, m[1])
					}
				}
				for _, m := range generatedPlaceholderRe.FindAllStringSubmatch(tok, -1) {
					if !seenGenerateKeys[m[1]] {
						return fmt.Errorf("template %q deployment %d: post_deploy_commands references undefined generate_key %q", t.Key, i, m[1])
					}
				}
			}
		}
	}
	if primaryCount != 1 {
		return fmt.Errorf("template %q: expected exactly 1 deployment with slug_suffix \"\", got %d", t.Key, primaryCount)
	}
	return nil
}

// List returns every built-in template, sorted by name, for legacy callers.
func List() []Template { return defaultRegistry.List() }

// Get looks up a built-in template by key for legacy callers.
func Get(key string) (Template, bool) { return defaultRegistry.Get(key) }
