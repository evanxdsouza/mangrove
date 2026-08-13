// Package config centralizes Mangrove's runtime configuration, loaded from
// environment variables (see deploy/systemd/mangrove.service's
// EnvironmentFile) with sensible defaults for local development.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/evanxdsouza/mangrove/internal/portregistry"
)

type Config struct {
	DataDir string
	DBPath  string
	// StaticSitesDir holds the output directories for static-strategy
	// deploys, one subdirectory per deploy history entry -- see
	// executor.DockerExecutor.StaticSitesDir and proxy.PutFileServerRoute.
	//
	// Deliberately NOT nested under DataDir: DataDir (and its parent,
	// mangrove.service's WorkingDirectory in production) is locked down to
	// 0700 for the sqlite DB and master key, which the caddy user has no
	// business reading -- but caddy DOES need to read every static site's
	// output, since that's the whole point of file_server serving it
	// publicly. A production install must point this at its own
	// world-readable directory (see deploy/systemd/mangrove.service) rather
	// than relying on the default, which assumes an unsandboxed dev box.
	StaticSitesDir string

	// APIPort is the single fixed port Mangrove's own API/dashboard/webhook
	// receiver listens on -- never allocated to a deployment.
	APIPort int

	PortRangeMin int
	PortRangeMax int

	NetworkName string

	// CgroupParent, when set, is passed to every deployment container so it
	// runs under mangrove-deployments.slice's hard memory ceiling (see
	// deploy/systemd/mangrove-deployments.slice). Left empty by default so
	// local development doesn't require the slice units to be installed.
	CgroupParent string

	// CaddyAdminAddr is Caddy's local admin API. Empty uses the client's
	// built-in default (http://127.0.0.1:2019).
	CaddyAdminAddr string

	// DeploymentMemoryCeilingMB mirrors mangrove-deployments.slice's
	// MemoryMax; admission control rejects a deploy that would push the sum
	// of configured service memory limits past this.
	DeploymentMemoryCeilingMB int

	ResendAPIKey  string
	NotifyToEmail string

	// BaseDomain is used only to compose the suggested-domain-slug shown in
	// deploy-success emails -- Mangrove never registers it (Nest has no
	// public domain API).
	BaseDomain string

	// GithubOAuthClientID/Secret enable the "Deploy from GitHub" flow (OAuth
	// App, not a GitHub App -- see internal/github/oauth.go). Both empty
	// (the default) disables the feature entirely; handlers check this
	// rather than crashing on a missing credential.
	GithubOAuthClientID     string
	GithubOAuthClientSecret string

	// PublicURL, when set, overrides request-derived scheme+host detection
	// for building absolute URLs Mangrove hands to GitHub (the OAuth
	// redirect_uri, and an auto-registered webhook's callback URL). Needed
	// behind an edge that terminates HTTPS but doesn't forward
	// X-Forwarded-Proto to this box's own dashboard route (see
	// internal/proxy/caddy.go's PutRoute comment -- that header rewrite is
	// only applied to deployment routes, not Mangrove's own). No trailing
	// slash, e.g. "https://mangrove.example.com".
	PublicURL string
}

// GithubOAuthEnabled reports whether an OAuth App has been registered via
// MANGROVE_GITHUB_OAUTH_CLIENT_ID/_SECRET.
func (c Config) GithubOAuthEnabled() bool {
	return c.GithubOAuthClientID != "" && c.GithubOAuthClientSecret != ""
}

func Load() Config {
	dataDir := getEnv("MANGROVE_DATA_DIR", "./data")
	return Config{
		DataDir:                   dataDir,
		DBPath:                    filepath.Join(dataDir, "mangrove.db"),
		StaticSitesDir:            getEnv("MANGROVE_STATIC_SITES_DIR", "./mangrove-static"),
		APIPort:                   getEnvInt("MANGROVE_PORT", 7777),
		PortRangeMin:              getEnvInt("MANGROVE_PORT_RANGE_MIN", portregistry.DefaultRangeMin),
		PortRangeMax:              getEnvInt("MANGROVE_PORT_RANGE_MAX", portregistry.DefaultRangeMax),
		NetworkName:               getEnv("MANGROVE_NETWORK", "mangrove-net"),
		CgroupParent:              os.Getenv("MANGROVE_CGROUP_PARENT"),
		CaddyAdminAddr:            os.Getenv("MANGROVE_CADDY_ADMIN_ADDR"),
		DeploymentMemoryCeilingMB: getEnvInt("MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB", 1536),
		ResendAPIKey:              os.Getenv("MANGROVE_RESEND_API_KEY"),
		NotifyToEmail:             os.Getenv("MANGROVE_NOTIFY_EMAIL"),
		BaseDomain:                getEnv("MANGROVE_BASE_DOMAIN", "evanxdsouza.hackclub.app"),
		GithubOAuthClientID:       os.Getenv("MANGROVE_GITHUB_OAUTH_CLIENT_ID"),
		GithubOAuthClientSecret:   os.Getenv("MANGROVE_GITHUB_OAUTH_CLIENT_SECRET"),
		PublicURL:                 strings.TrimRight(os.Getenv("MANGROVE_PUBLIC_URL"), "/"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
