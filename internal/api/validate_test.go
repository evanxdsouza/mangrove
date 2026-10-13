package api

import (
	"strings"
	"testing"
)

func TestValidateSlug(t *testing.T) {
	for _, ok := range []string{"a", "my-app", "app2", "0x-1"} {
		if err := validateSlug("slug", ok, maxSlugLen); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
	for _, bad := range []string{"", "My-App", "my app", "-app", "app-", "a--b", "../etc", "a/b", "a.b", strings.Repeat("a", 64)} {
		if err := validateSlug("slug", bad, maxSlugLen); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
	if err := validateSlug("slug", strings.Repeat("a", 41), maxDeploymentSlugLen); err == nil {
		t.Error("expected deployment slug length cap")
	}
}

func TestValidateServiceName(t *testing.T) {
	for _, ok := range []string{"web", "api_v2", "db.primary", "worker-1"} {
		if err := validateServiceName(ok); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Web", "web server", "../x", "-web", "web_"} {
		if err := validateServiceName(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestValidateEnvVar(t *testing.T) {
	for _, ok := range []string{"DATABASE_URL", "_X", "spring.profiles.active", "a-b"} {
		if err := validateEnvVar(ok, "v\nmultiline is fine"); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
	for _, bad := range []string{"", "1ABC", "A=B", "bad key", "\ninject", strings.Repeat("K", 257)} {
		if err := validateEnvVar(bad, "v"); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
	if err := validateEnvVar("OK", "a\x00b"); err == nil {
		t.Error("expected NUL value to be rejected")
	}
}
