package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	maxCommunitySources   = 20
	maxCommunityTemplates = 100
	maxManifestBytes      = 1 << 20
	communityManifest     = "mangrove-templates.json"
)

var (
	githubRepoPartRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
	gitCommitRE      = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	communityKeyRE   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

type communityManifestFile struct {
	Templates []Template `json:"templates"`
}

type communityPin struct {
	Owner  string
	Repo   string
	Commit string
}

// parseCommunityPin accepts only owner/repo@<full commit SHA>. Branch names,
// tags, and arbitrary URLs are rejected so a configured catalog is immutable
// and cannot turn this fetcher into an SSRF primitive.
func parseCommunityPin(value string) (communityPin, error) {
	var pin communityPin
	parts := strings.Split(value, "@")
	if len(parts) != 2 || !gitCommitRE.MatchString(parts[1]) {
		return pin, fmt.Errorf("expected owner/repo@<40-character commit SHA>")
	}
	repoParts := strings.Split(parts[0], "/")
	if len(repoParts) != 2 || !githubRepoPartRE.MatchString(repoParts[0]) || !githubRepoPartRE.MatchString(repoParts[1]) {
		return pin, fmt.Errorf("expected a GitHub owner/repo name")
	}
	pin.Owner, pin.Repo, pin.Commit = repoParts[0], repoParts[1], strings.ToLower(parts[1])
	return pin, nil
}

// LoadCommunitySources builds a registry from the embedded templates and
// loads each configured, commit-pinned public GitHub catalog. A broken source
// is returned in warnings and skipped; it never prevents built-ins from
// loading. Community manifests are data only, but their deployment templates
// can still ask Docker to run images, commands, and mounted files, so the
// operator must explicitly trust every configured repository.
func LoadCommunitySources(ctx context.Context, sources []string) (*Registry, []error) {
	return loadCommunitySources(ctx, sources, &http.Client{Timeout: 8 * time.Second}, "https://raw.githubusercontent.com")
}

func loadCommunitySources(ctx context.Context, sources []string, client *http.Client, rawBase string) (*Registry, []error) {
	registry := NewRegistry()
	warnings := make([]error, 0)
	if len(sources) > maxCommunitySources {
		warnings = append(warnings, fmt.Errorf("community template sources: configured %d sources, maximum is %d; ignoring sources after the limit", len(sources), maxCommunitySources))
		sources = sources[:maxCommunitySources]
	}
	for _, source := range sources {
		pin, err := parseCommunityPin(strings.TrimSpace(source))
		if err != nil {
			warnings = append(warnings, fmt.Errorf("community template source %q: %w", source, err))
			continue
		}
		url := fmt.Sprintf("%s/%s/%s/%s/%s", strings.TrimRight(rawBase, "/"), pin.Owner, pin.Repo, pin.Commit, communityManifest)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: %w", pin.Owner, pin.Repo, err))
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mangrove-community-templates")
		resp, err := client.Do(req)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: fetch failed: %w", pin.Owner, pin.Repo, err))
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes+1))
		closeErr := resp.Body.Close()
		if readErr != nil {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: read failed: %w", pin.Owner, pin.Repo, readErr))
			continue
		}
		if closeErr != nil {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: close failed: %w", pin.Owner, pin.Repo, closeErr))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: GitHub returned HTTP %d", pin.Owner, pin.Repo, resp.StatusCode))
			continue
		}
		if len(body) > maxManifestBytes {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: manifest exceeds %d bytes", pin.Owner, pin.Repo, maxManifestBytes))
			continue
		}
		var manifest communityManifestFile
		if err := json.Unmarshal(body, &manifest); err != nil {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: invalid manifest: %w", pin.Owner, pin.Repo, err))
			continue
		}
		if len(manifest.Templates) == 0 || len(manifest.Templates) > maxCommunityTemplates {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: manifest must contain between 1 and %d templates", pin.Owner, pin.Repo, maxCommunityTemplates))
			continue
		}
		if err := registry.AddCommunity(pin.Owner, pin.Repo, pin.Commit, manifest.Templates); err != nil {
			warnings = append(warnings, fmt.Errorf("community template source %s/%s: %w", pin.Owner, pin.Repo, err))
		}
	}
	return registry, warnings
}
