// Package config centralizes Mangrove's runtime configuration, loaded from
// environment variables (see deploy/systemd/mangrove.service's
// EnvironmentFile) with sensible defaults for local development.
package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/evanxdsouza/mangrove/internal/portregistry"
)

type Config struct {
	DataDir string
	DBPath  string

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
}

func Load() Config {
	dataDir := getEnv("MANGROVE_DATA_DIR", "./data")
	return Config{
		DataDir:                   dataDir,
		DBPath:                    filepath.Join(dataDir, "mangrove.db"),
		APIPort:                   getEnvInt("MANGROVE_PORT", 7777),
		PortRangeMin:              getEnvInt("MANGROVE_PORT_RANGE_MIN", portregistry.DefaultRangeMin),
		PortRangeMax:              getEnvInt("MANGROVE_PORT_RANGE_MAX", portregistry.DefaultRangeMax),
		NetworkName:               getEnv("MANGROVE_NETWORK", "mangrove-net"),
		CgroupParent:              os.Getenv("MANGROVE_CGROUP_PARENT"),
		DeploymentMemoryCeilingMB: getEnvInt("MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB", 1536),
		ResendAPIKey:              os.Getenv("MANGROVE_RESEND_API_KEY"),
		NotifyToEmail:             os.Getenv("MANGROVE_NOTIFY_EMAIL"),
		BaseDomain:                getEnv("MANGROVE_BASE_DOMAIN", "evanxdsouza.hackclub.app"),
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
