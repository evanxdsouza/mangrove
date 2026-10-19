package templates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func validCommunityManifest(key string) string {
	return `{"templates":[{"key":"` + key + `","name":"Community ` + key + `","description":"test","category":"test","deployments":[{"slug_suffix":"","image_ref":"example/app:1","internal_port":80,"memory_limit_mb":64}]}]}`
}

func TestParseCommunityPinRequiresFullCommitSHA(t *testing.T) {
	pin, err := parseCommunityPin("Example-Org/mangrove-templates@" + strings.ToUpper(testCommit))
	if err != nil {
		t.Fatalf("expected valid pin, got %v", err)
	}
	if pin.Owner != "Example-Org" || pin.Repo != "mangrove-templates" || pin.Commit != testCommit {
		t.Fatalf("unexpected pin: %+v", pin)
	}

	for _, bad := range []string{
		"owner/repo",
		"owner/repo@main",
		"owner/repo@v1.0.0",
		"owner/repo@0123456789abcdef",
		"owner/repo/extra@" + testCommit,
		"https://github.com/owner/repo@" + testCommit,
		"owner/../repo@" + testCommit,
	} {
		if _, err := parseCommunityPin(bad); err == nil {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
}

func TestLoadCommunitySourcesLoadsPinnedManifest(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(validCommunityManifest("app")))
	}))
	defer server.Close()

	registry, warnings := loadCommunitySources(context.Background(), []string{"acme/templates@" + testCommit}, server.Client(), server.URL)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	wantPath := "/acme/templates/" + testCommit + "/mangrove-templates.json"
	if gotPath != wantPath {
		t.Fatalf("fetched %q, want %q", gotPath, wantPath)
	}
	tpl, ok := registry.Get("community-acme-templates-app")
	if !ok {
		t.Fatal("expected namespaced community template to be registered")
	}
	if tpl.SourceRepo != "acme/templates" || tpl.SourceCommit != testCommit {
		t.Fatalf("expected provenance to be retained, got repo=%q commit=%q", tpl.SourceRepo, tpl.SourceCommit)
	}
	if _, ok := registry.Get("postgres"); !ok {
		t.Fatal("expected built-ins to remain available")
	}
	if _, ok := DefaultRegistry().Get("community-acme-templates-app"); ok {
		t.Fatal("community templates must not mutate the default built-in registry")
	}
}

func TestLoadCommunitySourcesSkipsBadSourcesWithoutDroppingBuiltIns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/bad/") {
			_, _ = w.Write([]byte(`{"templates":`))
			return
		}
		if strings.Contains(r.URL.Path, "/missing/") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(validCommunityManifest("good")))
	}))
	defer server.Close()

	registry, warnings := loadCommunitySources(context.Background(), []string{
		"acme/bad@" + testCommit,
		"acme/missing@" + testCommit,
		"acme/good@" + testCommit,
		"acme/main",
	}, server.Client(), server.URL)
	if len(warnings) != 3 {
		t.Fatalf("expected three warnings, got %d: %v", len(warnings), warnings)
	}
	if _, ok := registry.Get("community-acme-good-good"); !ok {
		t.Fatal("expected the valid source to survive bad neighbors")
	}
	if _, ok := registry.Get("postgres"); !ok {
		t.Fatal("expected built-ins to survive bad sources")
	}
}

func TestAddCommunityRejectsBuiltInCollisionsAndDuplicates(t *testing.T) {
	registry := NewRegistry()
	err := registry.AddCommunity("acme", "templates", testCommit, []Template{validCommunityTemplate("postgres")})
	if err != nil {
		t.Fatalf("namespaced built-in-looking key should be accepted: %v", err)
	}
	err = registry.AddCommunity("acme", "templates", testCommit, []Template{validCommunityTemplate("app"), validCommunityTemplate("app")})
	if err == nil {
		t.Fatal("expected duplicate manifest keys to be rejected")
	}
	if _, ok := registry.Get("community-acme-templates-app"); ok {
		t.Fatal("a rejected manifest must not partially register templates")
	}
}

func TestAddCommunityRejectsInvalidKeysAndTemplates(t *testing.T) {
	registry := NewRegistry()
	bad := validCommunityTemplate("Bad_Key")
	if err := registry.AddCommunity("acme", "templates", testCommit, []Template{bad}); err == nil {
		t.Fatal("expected uppercase/underscore key to be rejected")
	}
	invalid := validCommunityTemplate("ok")
	invalid.Deployments[0].ImageRef = ""
	if err := registry.AddCommunity("acme", "templates", testCommit, []Template{invalid}); err == nil {
		t.Fatal("expected invalid deployment to be rejected")
	}
}

func validCommunityTemplate(key string) Template {
	return Template{
		Key:         key,
		Name:        "Community " + key,
		Description: "test",
		Category:    "test",
		Deployments: []Deployment{{SlugSuffix: "", ImageRef: "example/app:1", InternalPort: 80, MemoryLimitMB: 64}},
	}
}
