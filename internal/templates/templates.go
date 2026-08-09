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
}

// Template is one entry in the gallery.
type Template struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Category    string       `json:"category"`
	Deployments []Deployment `json:"deployments"`
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

var registry map[string]Template
var ordered []Template

func init() {
	entries, err := dataFS.ReadDir("data")
	if err != nil {
		panic(fmt.Sprintf("templates: read embedded data dir: %v", err))
	}

	registry = make(map[string]Template, len(entries))
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
}

var generatedPlaceholderRe = regexp.MustCompile(`\{\{generated:([^}]+)\}\}`)

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
			for _, m := range generatedPlaceholderRe.FindAllStringSubmatch(ev.Value, -1) {
				if !seenGenerateKeys[m[1]] {
					return fmt.Errorf("template %q deployment %d: env %q references undefined generate_key %q", t.Key, i, ev.Key, m[1])
				}
			}
			if ev.GenerateKey != "" {
				seenGenerateKeys[ev.GenerateKey] = true
			}
		}
	}
	if primaryCount != 1 {
		return fmt.Errorf("template %q: expected exactly 1 deployment with slug_suffix \"\", got %d", t.Key, primaryCount)
	}
	return nil
}

// List returns every built-in template, sorted by name, for the gallery.
func List() []Template {
	out := make([]Template, len(ordered))
	copy(out, ordered)
	return out
}

// Get looks up a template by key.
func Get(key string) (Template, bool) {
	t, ok := registry[key]
	return t, ok
}
