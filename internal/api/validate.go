package api

import (
	"fmt"
	"regexp"
	"strings"
)

// slugPattern is what web/src/lib/slug.ts's slugify produces: lowercase
// alphanumeric runs joined by single hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// servicePattern additionally allows "_" and "." (compose service names);
// it still has to be lowercase since it ends up in an image tag.
var servicePattern = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)

// Deployment slugs become a DNS label (<slug>.<base domain>, max 63) and
// get suffixes appended for staging ("-staging"), PR previews
// ("-pr-<n>"), and multi-deployment templates ("-realtime"), so they're
// capped well below 63.
const (
	maxSlugLen           = 63
	maxDeploymentSlugLen = 40
)

// validateSlug rejects anything that isn't safe to put in a container
// name, a hostname, an image tag, and a filesystem path. Before this,
// slugs were stored as typed -- "My App!" or "../../etc" were accepted and
// only failed (or worse, didn't) at deploy time.
func validateSlug(field, v string, maxLen int) error {
	if len(v) > maxLen {
		return fmt.Errorf("%s must be at most %d characters", field, maxLen)
	}
	if !slugPattern.MatchString(v) {
		return fmt.Errorf("%s must be lowercase letters, digits, and single hyphens (e.g. \"my-app\")", field)
	}
	return nil
}

// envKeyPattern is the POSIX shape plus "." and "-", which some runtimes
// (Spring, .NET) use and Docker passes through fine.
var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// validateEnvVar rejects what would break every later deploy rather than
// fail now: env vars reach `docker run` as "-e KEY=VALUE" arguments, where
// a NUL byte can't be passed at all and a key containing "=" silently
// becomes a different variable.
func validateEnvVar(key, value string) error {
	if len(key) > 256 || !envKeyPattern.MatchString(key) {
		return fmt.Errorf("env var name must start with a letter or underscore and contain only letters, digits, '_', '.', or '-' (got %q)", key)
	}
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("env var value can't contain a NUL byte")
	}
	return nil
}

func validateServiceName(v string) error {
	if len(v) > maxSlugLen {
		return fmt.Errorf("service name must be at most %d characters", maxSlugLen)
	}
	if !servicePattern.MatchString(v) {
		return fmt.Errorf("service name must be lowercase letters and digits, optionally separated by '-', '_', or '.'")
	}
	return nil
}
