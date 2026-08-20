package templates

import "testing"

func TestAllBuiltInTemplatesLoad(t *testing.T) {
	all := List()
	if len(all) != 15 {
		names := make([]string, len(all))
		for i, tpl := range all {
			names[i] = tpl.Key
		}
		t.Fatalf("expected 15 built-in templates, got %d: %v", len(all), names)
	}

	wantKeys := []string{
		"postgres", "redis", "mysql", "mongodb", "n8n", "ghost",
		"wordpress", "uptime-kuma", "vaultwarden", "umami", "nephthys",
		"nocodb", "pocketbase", "supabase", "gitea",
	}
	for _, k := range wantKeys {
		if _, ok := Get(k); !ok {
			t.Errorf("expected template %q to be registered", k)
		}
	}
}

// TestForceInternalOnlyOnRawTCPServices guards a real correctness
// constraint, not just a style choice: Caddy only reverse-proxies/serves
// HTTP (see internal/proxy), so a Postgres/MySQL/MongoDB/Redis deployment
// must never be user-togglable public -- there is no working way to expose
// it through Caddy, only a confusing broken one.
func TestForceInternalOnlyOnRawTCPServices(t *testing.T) {
	rawTCP := map[string]string{
		"postgres": "",
		"redis":    "",
		"mysql":    "",
		"mongodb":  "",
	}
	for key := range rawTCP {
		tpl, ok := Get(key)
		if !ok {
			t.Fatalf("template %q not found", key)
		}
		for _, d := range tpl.Deployments {
			if !d.ForceInternalOnly {
				t.Errorf("template %q deployment (slug_suffix %q) speaks raw TCP but is not force_internal_only", key, d.SlugSuffix)
			}
		}
	}

	// The linked DB dependencies inside wordpress/umami/nephthys must be forced too.
	for _, key := range []string{"wordpress", "umami", "nephthys"} {
		tpl, _ := Get(key)
		for _, d := range tpl.Deployments {
			if d.SlugSuffix != "" && !d.ForceInternalOnly {
				t.Errorf("template %q dependency deployment (slug_suffix %q) must be force_internal_only", key, d.SlugSuffix)
			}
		}
	}
}

// TestEachTemplateFitsDefaultCeiling checks every template's total memory
// footprint against the documented 2GB-box default
// (MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB=1536) with room to spare for
// Mangrove/Caddy/dockerd's own reserved floor -- a template that alone
// can't fit would be a broken "one-click deploy".
func TestEachTemplateFitsDefaultCeiling(t *testing.T) {
	const defaultCeilingMB = 1536
	for _, tpl := range List() {
		if total := tpl.TotalMemoryMB(); total > defaultCeilingMB {
			t.Errorf("template %q needs %dMB, exceeds the default %dMB ceiling on its own", tpl.Key, total, defaultCeilingMB)
		}
	}
}

func TestLinkedTemplatesHaveDependencyFirst(t *testing.T) {
	for _, key := range []string{"wordpress", "umami", "nephthys"} {
		tpl, _ := Get(key)
		if len(tpl.Deployments) != 2 {
			t.Fatalf("template %q: expected 2 deployments, got %d", key, len(tpl.Deployments))
		}
		if tpl.Deployments[0].SlugSuffix == "" {
			t.Errorf("template %q: dependency deployment should be first, got primary first", key)
		}
		if tpl.Deployments[1].SlugSuffix != "" {
			t.Errorf("template %q: primary deployment should be last", key)
		}
	}
}

func TestValidateRejectsPrimaryNotLast(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1},
			{SlugSuffix: "-dep", ImageRef: "x", MemoryLimitMB: 1},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject a template whose primary deployment isn't last")
	}
}

func TestValidateRejectsUnknownGeneratedReference(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "X", Value: "{{generated:nonexistent}}"},
			}},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject a {{generated:...}} reference with no matching generate_key")
	}
}

func TestValidateRejectsGenerateAndValueTogether(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "X", Value: "literal", Generate: "password"},
			}},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject an env var setting both generate and value")
	}
}

func TestValidateAcceptsForwardGeneratedReferenceWithinSameDeployment(t *testing.T) {
	// A deployment referencing its own generate_key (defined earlier in
	// its own Env slice, same deployment) is fine -- only cross-deployment
	// ordering matters.
	ok := Template{
		Key: "ok",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "PASSWORD", Generate: "password", GenerateKey: "pw"},
				{Key: "URL", Value: "postgres://{{generated:pw}}@host"},
			}},
		},
	}
	if err := validate(ok); err != nil {
		t.Errorf("expected same-deployment forward reference to validate, got: %v", err)
	}
}

func TestValidateRejectsDockerfileStrategyWithoutGitURL(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", BuildStrategy: "dockerfile", MemoryLimitMB: 1},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject a dockerfile-strategy deployment with no git_url")
	}
}

func TestValidateAcceptsDockerfileStrategyWithGitURLAndNoImageRef(t *testing.T) {
	ok := Template{
		Key: "ok",
		Deployments: []Deployment{
			{SlugSuffix: "", BuildStrategy: "dockerfile", GitURL: "https://github.com/owner/repo.git", MemoryLimitMB: 1},
		},
	}
	if err := validate(ok); err != nil {
		t.Errorf("expected dockerfile-strategy deployment with git_url to validate without image_ref, got: %v", err)
	}
}

func TestValidateRejectsPromptWithGenerateOrValue(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "TOKEN", Prompt: true, Value: "literal"},
			}},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject an env var setting both prompt and value")
	}

	bad2 := Template{
		Key: "bad2",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "TOKEN", Prompt: true, Generate: "password"},
			}},
		},
	}
	if err := validate(bad2); err == nil {
		t.Error("expected validate to reject an env var setting both prompt and generate")
	}
}

func TestValidateAcceptsPromptEnvVar(t *testing.T) {
	ok := Template{
		Key: "ok",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "TOKEN", Prompt: true, Label: "API token", Required: true},
			}},
		},
	}
	if err := validate(ok); err != nil {
		t.Errorf("expected a bare prompt env var to validate, got: %v", err)
	}
}

func TestValidateAcceptsTemplateWithFiles(t *testing.T) {
	ok := Template{
		Key: "ok",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "PW", Generate: "password", GenerateKey: "pw"},
			}, Files: []File{
				{Path: "/docker-entrypoint-initdb.d/99-setup.sql", Content: "ALTER ROLE x PASSWORD '{{generated:pw}}';\n"},
			}},
		},
	}
	if err := validate(ok); err != nil {
		t.Errorf("expected a template with a placeholder-bearing file to validate, got: %v", err)
	}
}

func TestValidateRejectsFileReferencingUnknownGeneratedKey(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Files: []File{
				{Path: "/init.sql", Content: "{{generated:nope}}"},
			}},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject a file referencing an undefined generate_key")
	}
}

func TestValidateRejectsEmptyFile(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Files: []File{
				{Path: "", Content: "content"},
				{Path: "/ok.sql", Content: ""},
			}},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject a file with an empty path or content")
	}
}

func TestValidateAcceptsJWTGenerateReferencingEarlierSecret(t *testing.T) {
	ok := Template{
		Key: "ok",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "JWT_SECRET", Generate: "hex64", GenerateKey: "jwt_secret"},
				{Key: "ANON_KEY", Generate: "jwt:anon:{{generated:jwt_secret}}", GenerateKey: "anon"},
			}},
		},
	}
	if err := validate(ok); err != nil {
		t.Errorf("expected a jwt generate referencing an earlier-generated secret to validate, got: %v", err)
	}
}

func TestValidateRejectsJWTGenerateReferencingUnknownSecret(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "ANON_KEY", Generate: "jwt:anon:{{generated:missing}}"},
			}},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject a jwt generate referencing an undefined secret")
	}
}

func TestValidateAcceptsPostDeployCommandReferencingOwnEnvKey(t *testing.T) {
	ok := Template{
		Key: "ok",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, Env: []EnvVar{
				{Key: "EXTRA_EMAIL", Prompt: true},
			}, PostDeployCommands: [][]string{
				{"sh", "-c", "echo \"$1\"", "--", "{{env:EXTRA_EMAIL}}"},
			}},
		},
	}
	if err := validate(ok); err != nil {
		t.Errorf("expected a post_deploy_commands entry referencing its own deployment's env key to validate, got: %v", err)
	}
}

func TestValidateRejectsPostDeployCommandReferencingUnknownEnvKey(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, PostDeployCommands: [][]string{
				{"sh", "-c", "echo \"$1\"", "--", "{{env:NOPE}}"},
			}},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject a post_deploy_commands entry referencing an undefined env key")
	}
}

func TestValidateRejectsEmptyPostDeployCommand(t *testing.T) {
	bad := Template{
		Key: "bad",
		Deployments: []Deployment{
			{SlugSuffix: "", ImageRef: "x", MemoryLimitMB: 1, PostDeployCommands: [][]string{
				{},
			}},
		},
	}
	if err := validate(bad); err == nil {
		t.Error("expected validate to reject an empty post_deploy_commands entry")
	}
}
