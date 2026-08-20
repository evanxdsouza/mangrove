package orchestrator

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/evanxdsouza/mangrove/internal/config"
	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/secrets"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// fakeTemplateExecutor satisfies executor.Executor without touching Docker,
// so InstallTemplate's own logic (row creation, placeholder resolution,
// per-deployment ordering) can be tested without a daemon. Every container
// is reported healthy immediately.
type fakeTemplateExecutor struct {
	builds         int
	buildSpecs     []executor.BuildSpec // every BuildSpec Build() was called with, in order -- so tests can assert on git-based builds without a real clone
	runSpecs       []executor.RunSpec   // every RunSpec Run() was called with, in order -- so tests can assert on what actually would have hit `docker run`, not just that Deploy() returned success
	removedRefs    []string             // every container ref Remove() was called with
	removedVolumes []string             // every volume name RemoveVolume() was called with
	removedImages  []string             // every image tag RemoveImage() was called with
	stoppedRefs    []string             // every container ref Stop() was called with
	restartedRefs  []string             // every container ref Restart() was called with
	execCalls      []execCall           // every (ref, cmd) Exec() was called with
	execResult     executor.ExecResult  // scripted return value for every Exec() call
	execErr        error                // scripted error for every Exec() call
}

type execCall struct {
	ref string
	cmd []string
}

func (f *fakeTemplateExecutor) Build(ctx context.Context, spec executor.BuildSpec, logs io.Writer) (executor.BuildResult, error) {
	f.builds++
	f.buildSpecs = append(f.buildSpecs, spec)
	return executor.BuildResult{ImageTag: spec.ImageRef}, nil
}
func (f *fakeTemplateExecutor) Run(ctx context.Context, spec executor.RunSpec) (executor.RunResult, error) {
	f.runSpecs = append(f.runSpecs, spec)
	return executor.RunResult{ContainerID: "container-" + spec.ContainerName, ContainerAddr: "10.0.0.1:1234"}, nil
}
func (f *fakeTemplateExecutor) Stop(ctx context.Context, ref string, timeout time.Duration) error {
	f.stoppedRefs = append(f.stoppedRefs, ref)
	return nil
}
func (f *fakeTemplateExecutor) Restart(ctx context.Context, ref string, timeout time.Duration) error {
	f.restartedRefs = append(f.restartedRefs, ref)
	return nil
}
func (f *fakeTemplateExecutor) Exec(ctx context.Context, ref string, cmd []string) (executor.ExecResult, error) {
	f.execCalls = append(f.execCalls, execCall{ref: ref, cmd: cmd})
	return f.execResult, f.execErr
}
func (f *fakeTemplateExecutor) Remove(ctx context.Context, ref string) error {
	f.removedRefs = append(f.removedRefs, ref)
	return nil
}
func (f *fakeTemplateExecutor) HealthCheck(ctx context.Context, ref string, cfg executor.HealthCheckSpec) (executor.HealthStatus, error) {
	return executor.HealthStatus{Healthy: true, StatusCode: 200}, nil
}
func (f *fakeTemplateExecutor) Logs(ctx context.Context, ref string, opts executor.LogOptions) (io.ReadCloser, error) {
	return nil, nil
}
func (f *fakeTemplateExecutor) Stats(ctx context.Context, ref string) (executor.ResourceStats, error) {
	return executor.ResourceStats{MemUsageMB: 1}, nil
}
func (f *fakeTemplateExecutor) Prune(ctx context.Context, opts executor.PruneOptions) (executor.PruneResult, error) {
	return executor.PruneResult{}, nil
}
func (f *fakeTemplateExecutor) RemoveVolume(ctx context.Context, name string) error {
	f.removedVolumes = append(f.removedVolumes, name)
	return nil
}
func (f *fakeTemplateExecutor) RemoveImage(ctx context.Context, ref string) error {
	f.removedImages = append(f.removedImages, ref)
	return nil
}
func (f *fakeTemplateExecutor) ContainerAddr(ctx context.Context, ref string, port int) (string, error) {
	return "10.0.0.1:1234", nil
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newTestOrchestrator(t *testing.T) (*Orchestrator, *store.Store, int64) {
	t.Helper()
	dir := t.TempDir()

	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.New(db)

	key, err := secrets.LoadOrCreateMasterKey(filepath.Join(dir, "master.key"))
	if err != nil {
		t.Fatalf("load master key: %v", err)
	}
	box, err := secrets.NewBox(key)
	if err != nil {
		t.Fatalf("new box: %v", err)
	}

	proj, err := st.CreateProject(context.Background(), 1, "Template Test", "template-test", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	o := &Orchestrator{
		Store:   st,
		Exec:    &fakeTemplateExecutor{},
		Secrets: box,
		Config: config.Config{
			NetworkName:               "mangrove-test-net",
			PortRangeMin:              20000,
			PortRangeMax:              21000,
			DeploymentMemoryCeilingMB: 4096,
			BaseDomain:                "example.test",
		},
		Log: discardLogger(),
	}
	return o, st, proj.ID
}

func TestInstallTemplateStandaloneCreatesRowsAndDeploys(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "postgres", "mydb", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	if len(result.Deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(result.Deployments))
	}
	if result.Deployments[0].Slug != "mydb" {
		t.Errorf("expected slug 'mydb', got %q", result.Deployments[0].Slug)
	}
	if result.Deployments[0].DeployError != "" {
		t.Errorf("expected deploy to succeed, got error: %s", result.Deployments[0].DeployError)
	}

	dep, err := st.GetDeployment(ctx, result.Deployments[0].DeploymentID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.Status != "running" {
		t.Errorf("expected deployment status 'running', got %q", dep.Status)
	}
	if dep.ImageRef != "postgres:16-alpine" {
		t.Errorf("expected image_ref postgres:16-alpine, got %q", dep.ImageRef)
	}

	services, err := st.ListServices(ctx, dep.ID)
	if err != nil || len(services) != 1 {
		t.Fatalf("ListServices: %v (got %d)", err, len(services))
	}
	svc := services[0]
	if !svc.IsInternalOnly {
		t.Error("expected postgres service to be internal-only")
	}

	vols, err := st.ListVolumesForService(ctx, svc.ID)
	if err != nil || len(vols) != 1 {
		t.Fatalf("ListVolumesForService: %v (got %d)", err, len(vols))
	}
	if vols[0].MountPath != "/var/lib/postgresql/data" {
		t.Errorf("unexpected mount path %q", vols[0].MountPath)
	}

	// The generated POSTGRES_PASSWORD should come back as a shown-once
	// credential, and never in plaintext through the normal env listing.
	found := false
	for label, val := range result.Credentials {
		if label == "PostgreSQL: POSTGRES_PASSWORD" {
			found = true
			if val == "" {
				t.Error("expected non-empty generated password")
			}
		}
	}
	if !found {
		t.Errorf("expected a credential for PostgreSQL: POSTGRES_PASSWORD, got %v", result.Credentials)
	}

	rows, err := st.ListEnvVarRows(ctx, svc.ID)
	if err != nil {
		t.Fatalf("ListEnvVarRows: %v", err)
	}
	for _, r := range rows {
		if r.KeyName == "POSTGRES_PASSWORD" && !r.IsSecret {
			t.Error("expected POSTGRES_PASSWORD to be stored as a secret")
		}
	}
}

// TestInstallTemplateWiresVolumesAndCommandThroughToRun asserts on what the
// executor's Run() actually received, not just that Deploy() reported
// success -- a fake executor that only checks "did Build/Run return no
// error" can't catch a field that's set on the template/DB side but never
// copied into RunSpec before Run() is called (redis's --requirepass
// Command override was exactly this bug: wired into executor.RunSpec and
// the templates schema, but never read out of the service row in
// orchestrator.Deploy(), so Run() got called with an empty Command).
func TestInstallTemplateWiresVolumesAndCommandThroughToRun(t *testing.T) {
	o, _, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	if _, err := o.InstallTemplate(ctx, projectID, "redis", "cache", nil, nil); err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}

	fake := o.Exec.(*fakeTemplateExecutor)
	if len(fake.runSpecs) != 1 {
		t.Fatalf("expected exactly 1 Run() call, got %d", len(fake.runSpecs))
	}
	spec := fake.runSpecs[0]

	wantCommand := []string{"sh", "-c", `redis-server --requirepass "$REDIS_PASSWORD"`}
	if len(spec.Command) != len(wantCommand) {
		t.Fatalf("Command not passed through to RunSpec: got %v, want %v", spec.Command, wantCommand)
	}
	for i := range wantCommand {
		if spec.Command[i] != wantCommand[i] {
			t.Errorf("Command[%d] = %q, want %q", i, spec.Command[i], wantCommand[i])
		}
	}

	if len(spec.Volumes) != 1 || spec.Volumes[0].MountPath != "/data" {
		t.Errorf("expected 1 volume mounted at /data, got %v", spec.Volumes)
	}
	if spec.Env["REDIS_PASSWORD"] == "" {
		t.Error("expected REDIS_PASSWORD to be set in the container env")
	}
	if spec.NetworkAlias == "" {
		t.Error("expected a stable NetworkAlias to be set")
	}
}

// TestInstallTemplateGiteaDeploysStandalone verifies the git-client
// template installs as a single internal-only deployment with the
// GitHub-like Git service pre-configured: sqlite-backed, HTTPS clone URLs
// derived from the deployment's public domain, SSH off, and self-registration
// disabled so only the admin can create users and set their passwords.
func TestInstallTemplateGiteaDeploysStandalone(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "gitea", "code", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	if len(result.Deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(result.Deployments))
	}
	if result.Deployments[0].Slug != "code" {
		t.Errorf("expected slug 'code', got %q", result.Deployments[0].Slug)
	}
	if result.Deployments[0].DeployError != "" {
		t.Errorf("expected deploy to succeed, got error: %s", result.Deployments[0].DeployError)
	}

	dep, err := st.GetDeployment(ctx, result.Deployments[0].DeploymentID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.ImageRef != "gitea/gitea:latest" {
		t.Errorf("expected image_ref gitea/gitea:latest, got %q", dep.ImageRef)
	}

	services, err := st.ListServices(ctx, dep.ID)
	if err != nil || len(services) != 1 {
		t.Fatalf("ListServices: %v (got %d)", err, len(services))
	}
	svc := services[0]
	if !svc.IsInternalOnly {
		t.Error("expected gitea service to start internal-only")
	}
	if svc.InternalPort != 3000 {
		t.Errorf("expected internal_port 3000, got %d", svc.InternalPort)
	}

	vols, err := st.ListVolumesForService(ctx, svc.ID)
	if err != nil || len(vols) != 1 {
		t.Fatalf("ListVolumesForService: %v (got %d)", err, len(vols))
	}
	if vols[0].MountPath != "/data" {
		t.Errorf("unexpected mount path %q", vols[0].MountPath)
	}

	rows, err := st.ListEnvVarRows(ctx, svc.ID)
	if err != nil {
		t.Fatalf("ListEnvVarRows: %v", err)
	}
	values := map[string]string{}
	for _, r := range rows {
		if r.IsSecret {
			t.Errorf("unexpected secret env var %q on gitea template", r.KeyName)
			continue
		}
		values[r.KeyName] = r.ValuePlain.String
	}

	if got := values["GITEA__server__ROOT_URL"]; got != "https://code.example.test/" {
		t.Errorf("expected ROOT_URL to resolve to the deployment's public URL, got %q", got)
	}
	if got := values["GITEA__service__DISABLE_REGISTRATION"]; got != "true" {
		t.Errorf("expected DISABLE_REGISTRATION true so only the admin creates users, got %q", got)
	}
	if got := values["GITEA__database__DB_TYPE"]; got != "sqlite3" {
		t.Errorf("expected sqlite3 DB type, got %q", got)
	}
	if got := values["GITEA__database__PATH"]; got != "/data/gitea/gitea.db" {
		t.Errorf("expected DB path on the /data volume, got %q", got)
	}
	if _, ok := values["GITEA__security__INSTALL_LOCK"]; ok {
		t.Error("expected INSTALL_LOCK to be left unset so the one-time setup page can run and create the admin")
	}
}

func TestInstallTemplateLinkedDependencyResolvesAliasAndGeneratedValue(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "wordpress", "blog", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	if len(result.Deployments) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(result.Deployments))
	}

	// Dependency first, primary last -- both deployed successfully.
	if result.Deployments[0].Slug != "blog-mysql" {
		t.Errorf("expected first deployment slug 'blog-mysql', got %q", result.Deployments[0].Slug)
	}
	if result.Deployments[1].Slug != "blog" {
		t.Errorf("expected second deployment slug 'blog', got %q", result.Deployments[1].Slug)
	}
	for _, d := range result.Deployments {
		if d.DeployError != "" {
			t.Errorf("deployment %q failed to deploy: %s", d.Slug, d.DeployError)
		}
	}

	mysqlDep, _ := st.GetDeployment(ctx, result.Deployments[0].DeploymentID)
	wpDep, _ := st.GetDeployment(ctx, result.Deployments[1].DeploymentID)

	mysqlSvcs, _ := st.ListServices(ctx, mysqlDep.ID)
	wpSvcs, _ := st.ListServices(ctx, wpDep.ID)
	wpSvc := wpSvcs[0]

	wantAlias := "mangrove-blog-mysql-app" // stable container/network-alias name for the mysql sibling
	rows, err := st.ListEnvVarRows(ctx, wpSvc.ID)
	if err != nil {
		t.Fatalf("ListEnvVarRows: %v", err)
	}
	var dbHost, dbPassword string
	var dbPasswordSecret bool
	for _, r := range rows {
		switch r.KeyName {
		case "WORDPRESS_DB_HOST":
			dbHost = r.ValuePlain.String
		case "WORDPRESS_DB_PASSWORD":
			dbPasswordSecret = r.IsSecret
			if r.IsSecret {
				plaintext, err := o.Secrets.Open([]byte("env_vars:"+strconv.FormatInt(wpSvc.ID, 10)+":WORDPRESS_DB_PASSWORD"), r.ValueEncrypted, r.ValueNonce)
				if err != nil {
					t.Fatalf("decrypt WORDPRESS_DB_PASSWORD: %v", err)
				}
				dbPassword = string(plaintext)
			}
		}
	}

	if dbHost != wantAlias+":3306" {
		t.Errorf("expected WORDPRESS_DB_HOST %q, got %q", wantAlias+":3306", dbHost)
	}
	if !dbPasswordSecret {
		t.Fatal("expected WORDPRESS_DB_PASSWORD to be stored as a secret")
	}

	// The password WordPress got must be the exact same one generated for
	// MySQL's MYSQL_PASSWORD -- that's the whole point of {{generated:...}}.
	mysqlRows, _ := st.ListEnvVarRows(ctx, mysqlSvcs[0].ID)
	var mysqlUserPassword string
	for _, r := range mysqlRows {
		if r.KeyName == "MYSQL_PASSWORD" {
			plaintext, err := o.Secrets.Open([]byte("env_vars:"+strconv.FormatInt(mysqlSvcs[0].ID, 10)+":MYSQL_PASSWORD"), r.ValueEncrypted, r.ValueNonce)
			if err != nil {
				t.Fatalf("decrypt MYSQL_PASSWORD: %v", err)
			}
			mysqlUserPassword = string(plaintext)
		}
	}
	if mysqlUserPassword == "" {
		t.Fatal("expected MYSQL_PASSWORD to be set on the mysql deployment")
	}
	if dbPassword != mysqlUserPassword {
		t.Errorf("WordPress's WORDPRESS_DB_PASSWORD (%q) should equal MySQL's generated MYSQL_PASSWORD (%q)", dbPassword, mysqlUserPassword)
	}
}

// TestInstallTemplateFailsFastOnMissingRequiredPromptedEnv verifies that a
// missing required prompt value is rejected before any rows are created --
// not partway through, after the linked Postgres dependency already exists.
func TestInstallTemplateFailsFastOnMissingRequiredPromptedEnv(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	_, err := o.InstallTemplate(ctx, projectID, "nephthys", "helper", nil, nil)
	if err == nil {
		t.Fatal("expected InstallTemplate to fail when required prompted env vars are missing")
	}

	deployments, err := st.ListDeployments(ctx, projectID)
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(deployments) != 0 {
		t.Errorf("expected no deployment rows created when required prompt validation fails upfront, got %d", len(deployments))
	}
}

// TestInstallTemplateDockerfileBuildWithPromptedEnv exercises the full
// nephthys install path: a git+Dockerfile build for the primary deployment,
// and prompted env overrides substituted in the same way memory overrides
// are, never stored as part of the template itself.
func TestInstallTemplateDockerfileBuildWithPromptedEnv(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	envOverrides := map[string]map[string]string{
		"": {
			"SLACK_BOT_TOKEN":         "xoxb-test",
			"SLACK_USER_TOKEN":        "xoxp-test",
			"SLACK_SIGNING_SECRET":    "sig-test",
			"SLACK_HEARTBEAT_CHANNEL": "C1",
			"SLACK_TICKET_CHANNEL":    "C2",
			"SLACK_BTS_CHANNEL":       "C3",
			"SLACK_HELP_CHANNEL":      "C4",
			"SLACK_MAINTAINER_ID":     "U1",
			"HACK_CLUB_AI_API_KEY":    "sk-hc-test",
		},
	}

	result, err := o.InstallTemplate(ctx, projectID, "nephthys", "helper", nil, envOverrides)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	if len(result.Deployments) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(result.Deployments))
	}
	for _, d := range result.Deployments {
		if d.DeployError != "" {
			t.Errorf("deployment %q failed to deploy: %s", d.Slug, d.DeployError)
		}
	}

	primaryDep, err := st.GetDeployment(ctx, result.Deployments[1].DeploymentID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if primaryDep.BuildStrategy != "dockerfile" {
		t.Errorf("expected build_strategy 'dockerfile', got %q", primaryDep.BuildStrategy)
	}
	if primaryDep.GitBranch != "main" {
		t.Errorf("expected git_branch 'main', got %q", primaryDep.GitBranch)
	}

	fake := o.Exec.(*fakeTemplateExecutor)
	if len(fake.buildSpecs) != 2 {
		t.Fatalf("expected 2 Build() calls (db image-tag + nephthys dockerfile), got %d", len(fake.buildSpecs))
	}
	dockerfileBuild := fake.buildSpecs[1]
	if dockerfileBuild.Context.GitURL != "https://github.com/hackclub/nephthys.git" {
		t.Errorf("expected BuildSpec.Context.GitURL to be the nephthys repo, got %q", dockerfileBuild.Context.GitURL)
	}
	if dockerfileBuild.Context.GitRef != "main" {
		t.Errorf("expected BuildSpec.Context.GitRef 'main', got %q", dockerfileBuild.Context.GitRef)
	}
	if dockerfileBuild.Context.AuthToken != "" {
		t.Errorf("expected no AuthToken for a public template repo build, got %q", dockerfileBuild.Context.AuthToken)
	}

	primarySvcs, _ := st.ListServices(ctx, primaryDep.ID)
	rows, err := st.ListEnvVarRows(ctx, primarySvcs[0].ID)
	if err != nil {
		t.Fatalf("ListEnvVarRows: %v", err)
	}
	values := map[string]string{}
	for _, r := range rows {
		if r.IsSecret {
			plaintext, err := o.Secrets.Open([]byte("env_vars:"+strconv.FormatInt(primarySvcs[0].ID, 10)+":"+r.KeyName), r.ValueEncrypted, r.ValueNonce)
			if err != nil {
				t.Fatalf("decrypt %s: %v", r.KeyName, err)
			}
			values[r.KeyName] = string(plaintext)
		} else {
			values[r.KeyName] = r.ValuePlain.String
		}
	}

	if values["SLACK_BOT_TOKEN"] != "xoxb-test" {
		t.Errorf("expected prompted SLACK_BOT_TOKEN to be substituted in, got %q", values["SLACK_BOT_TOKEN"])
	}
	if values["BASE_URL"] != "helper.example.test" {
		t.Errorf("expected BASE_URL to resolve {{slug}}.{{base_domain}}, got %q", values["BASE_URL"])
	}
	if values["PORT"] != "3000" {
		t.Errorf("expected literal PORT default '3000', got %q", values["PORT"])
	}
}

// TestInstallTemplateSupabaseResolvesFilesAliasesAndJWTs exercises the
// Supabase template end-to-end through the fake executor, asserting on the
// three things that make it more than a pile of containers: the init-SQL
// files actually reach the -db container's RunSpec with the generated
// password substituted, the sibling aliases land in the gateway's env, and
// the anon/service_role keys are real HS256 JWTs signed with the shared
// JWT_SECRET (PostgREST verifies them, so a malformed key would break the
// REST API at first request).
func TestInstallTemplateSupabaseResolvesFilesAliasesAndJWTs(t *testing.T) {
	o, _, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "supabase", "sup", nil, nil)
	if err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}
	if len(result.Deployments) != 7 {
		t.Fatalf("expected 7 deployments, got %d", len(result.Deployments))
	}
	if result.Deployments[0].Slug != "sup-db" || result.Deployments[6].Slug != "sup" {
		t.Errorf("expected -db dependency first and primary gateway last, got first=%q last=%q", result.Deployments[0].Slug, result.Deployments[6].Slug)
	}
	for _, d := range result.Deployments {
		if d.DeployError != "" {
			t.Errorf("deployment %q failed to deploy: %s", d.Slug, d.DeployError)
		}
	}

	fake := o.Exec.(*fakeTemplateExecutor)
	if len(fake.runSpecs) != 7 {
		t.Fatalf("expected 7 Run() calls, got %d", len(fake.runSpecs))
	}

	// The gateway (the one nginx RunSpec) must have gotten its sh -c
	// config-writer command and the sibling aliases in its env.
	var gw, db executor.RunSpec
	for _, s := range fake.runSpecs {
		switch s.InternalPort {
		case 8000:
			gw = s
		case 5432:
			db = s
		}
	}
	if gw.ImageRef != "nginx:alpine" {
		t.Fatalf("expected gateway image nginx:alpine, got %q", gw.ImageRef)
	}
	if len(gw.Command) != 3 || gw.Command[0] != "sh" || gw.Command[1] != "-c" {
		t.Fatalf("expected sh -c gateway command, got %v", gw.Command)
	}
	wantAliases := map[string]string{
		"NGINX_AUTH_HOST":     "mangrove-sup-auth-app",
		"NGINX_REST_HOST":     "mangrove-sup-rest-app",
		"NGINX_REALTIME_HOST": "mangrove-sup-realtime-app",
		"NGINX_META_HOST":     "mangrove-sup-meta-app",
		"NGINX_STUDIO_HOST":   "mangrove-sup-studio-app",
	}
	for key, want := range wantAliases {
		if gw.Env[key] != want {
			t.Errorf("expected gateway env %s=%q, got %q", key, want, gw.Env[key])
		}
	}
	if !strings.Contains(gw.Command[2], "proxy_pass http://$NGINX_AUTH_HOST:9999/") ||
		!strings.Contains(gw.Command[2], "map \\$http_upgrade \\$connection_upgrade") ||
		!strings.Contains(gw.Command[2], "exec nginx -g 'daemon off;'") {
		t.Error("gateway command should render a self-configuring nginx.conf with escaped nginx vars")
	}

	// The -db RunSpec carries the init-SQL file mounts, with the generated
	// password substituted into the role ALTERs.
	postgresPassword := result.Credentials["Supabase Database: POSTGRES_PASSWORD"]
	jwtSecret := result.Credentials["Supabase Database: JWT_SECRET"]
	if postgresPassword == "" || jwtSecret == "" {
		t.Fatalf("expected DB password and JWT secret in shown-once credentials, got %v", result.Credentials)
	}
	if len(db.Files) != 3 {
		t.Fatalf("expected 3 init-SQL files on the -db RunSpec, got %d", len(db.Files))
	}
	var rolesContent, jwtFileContent string
	seenPaths := map[string]string{}
	for _, f := range db.Files {
		seenPaths[f.Path] = string(f.Content)
	}
	rolesContent = seenPaths["/docker-entrypoint-initdb.d/init-scripts/99-roles.sql"]
	jwtFileContent = seenPaths["/docker-entrypoint-initdb.d/init-scripts/99-jwt.sql"]
	if rolesContent == "" || jwtFileContent == "" {
		t.Fatalf("expected roles.sql and jwt.sql mounts, got paths %v", seenPaths)
	}
	if !strings.Contains(rolesContent, "ALTER ROLE authenticator WITH LOGIN PASSWORD '"+postgresPassword+"'") {
		t.Error("roles.sql should set the authenticator role to the generated password")
	}
	if !strings.Contains(jwtFileContent, "ALTER DATABASE postgres SET \"app.settings.jwt_secret\" TO '"+jwtSecret+"'") {
		t.Error("jwt.sql should record the generated JWT secret")
	}

	// The anon key must be an HS256 JWT with role "anon", signed with the
	// shared JWT secret -- otherwise PostgREST would reject it.
	anonKey := result.Credentials["Supabase Studio: SUPABASE_ANON_KEY"]
	serviceKey := result.Credentials["Supabase Studio: SUPABASE_SERVICE_KEY"]
	if anonKey == "" || serviceKey == "" {
		t.Fatalf("expected anon/service keys in credentials, got %v", result.Credentials)
	}
	checkSupabaseJWT(t, anonKey, jwtSecret, "anon")
	checkSupabaseJWT(t, serviceKey, jwtSecret, "service_role")

	// Studio's public URLs must be the primary gateway's public URL, not an
	// alias-resolved internal name -- {{base_slug}} should expand to the
	// user's chosen base slug.
	for _, s := range fake.runSpecs {
		if s.InternalPort == 3000 && s.ImageRef == "supabase/studio:2026.08.03-sha-022b374" {
			if s.Env["SUPABASE_URL"] != "https://sup.example.test" {
				t.Errorf("expected SUPABASE_URL to use the base slug public URL, got %q", s.Env["SUPABASE_URL"])
			}
			if s.Env["STUDIO_PG_META_URL"] != "http://mangrove-sup-meta-app:8080" {
				t.Errorf("expected STUDIO_PG_META_URL to use the meta alias, got %q", s.Env["STUDIO_PG_META_URL"])
			}
		}
	}
}

func checkSupabaseJWT(t *testing.T, token, secret, wantRole string) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected a 3-part JWT, got %d parts in %q", len(parts), token)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)); sig != parts[2] {
		t.Errorf("JWT signature does not verify against the shared JWT secret (role %q)", wantRole)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode JWT payload: %v", err)
	}
	var claims struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal JWT claims: %v", err)
	}
	if claims.Role != wantRole {
		t.Errorf("expected JWT role %q, got %q", wantRole, claims.Role)
	}
}

// TestGenerateSupabaseJWTValidatesSignature verifies the jwt generate kind
// produces a signature that verifies with the referenced secret -- the same
// check PostgREST performs against PGRST_JWT_SECRET.
func TestGenerateSupabaseJWTValidatesSignature(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	token, err := generateValue("jwt:anon:{{generated:jwt_secret}}", map[string]string{"jwt_secret": secret})
	if err != nil {
		t.Fatalf("generateValue: %v", err)
	}
	checkSupabaseJWT(t, token, secret, "anon")

	if _, err := generateValue("jwt:anon:{{generated:missing}}", map[string]string{}); err == nil {
		t.Error("expected generateValue to reject a jwt referencing an unknown secret")
	}
	if _, err := generateValue("jwt:bogus:{{generated:jwt_secret}}", map[string]string{"jwt_secret": secret}); err == nil {
		t.Error("expected generateValue to reject a jwt with an unknown role")
	}
}

// TestGenerateHex64 verifies the hex64 generate kind returns 64 lowercase
// hex chars (32 random bytes), the shape HS256 JWT secrets expect.
func TestGenerateHex64(t *testing.T) {
	v, err := generateValue("hex64", nil)
	if err != nil {
		t.Fatalf("generateValue: %v", err)
	}
	if len(v) != 64 {
		t.Errorf("expected 64 chars, got %d (%q)", len(v), v)
	}
	for _, c := range v {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("hex64 value %q contains non-hex char %q", v, c)
		}
	}
}

// TestInstallTemplateRunsPostDeployCommandsForOptionalExtraAdmins exercises
// pocketbase.json's real post_deploy_commands: a supplied second-admin
// email/password pair should reach the exec'd command as literal argv
// (never re-interpreted by a shell), and a blank third-admin slot should
// still run its guarded command harmlessly (empty args, exit 0) rather than
// being skipped or failing the install.
func TestInstallTemplateRunsPostDeployCommandsForOptionalExtraAdmins(t *testing.T) {
	o, _, projectID := newTestOrchestrator(t)
	ctx := context.Background()
	fake := o.Exec.(*fakeTemplateExecutor)
	fake.execResult = executor.ExecResult{ExitCode: 0}

	overrides := map[string]map[string]string{
		"": {
			"PB_ADMIN_EMAIL":      "owner@example.com",
			"PB_ADMIN_PASSWORD":   "ownerpassword1",
			"PB_ADMIN_EMAIL_2":    "second@example.com",
			"PB_ADMIN_PASSWORD_2": "secondpassword2",
			// PB_ADMIN_EMAIL_3 / PB_ADMIN_PASSWORD_3 left unset on purpose.
		},
	}

	if _, err := o.InstallTemplate(ctx, projectID, "pocketbase", "mypb", nil, overrides); err != nil {
		t.Fatalf("InstallTemplate: %v", err)
	}

	if len(fake.execCalls) != 2 {
		t.Fatalf("expected 2 post-deploy exec calls (admin 2 and admin 3 slots), got %d: %+v", len(fake.execCalls), fake.execCalls)
	}
	got2 := fake.execCalls[0].cmd
	if got2[len(got2)-2] != "second@example.com" || got2[len(got2)-1] != "secondpassword2" {
		t.Errorf("expected admin 2's exec command to carry the resolved email/password, got %+v", got2)
	}
	got3 := fake.execCalls[1].cmd
	if got3[len(got3)-2] != "" || got3[len(got3)-1] != "" {
		t.Errorf("expected admin 3's exec command to carry empty email/password (unset, skipped by its own guard), got %+v", got3)
	}
}

// TestInstallTemplateFailsOnNonzeroPostDeployCommandExit verifies a failed
// seed command (e.g. pocketbase superuser upsert rejecting a bad password)
// fails the whole install, the same way a bad Deploy() call does.
func TestInstallTemplateFailsOnNonzeroPostDeployCommandExit(t *testing.T) {
	o, _, projectID := newTestOrchestrator(t)
	ctx := context.Background()
	fake := o.Exec.(*fakeTemplateExecutor)
	fake.execResult = executor.ExecResult{ExitCode: 1, Output: "boom"}

	overrides := map[string]map[string]string{
		"": {
			"PB_ADMIN_EMAIL":      "owner@example.com",
			"PB_ADMIN_PASSWORD":   "ownerpassword1",
			"PB_ADMIN_EMAIL_2":    "second@example.com",
			"PB_ADMIN_PASSWORD_2": "secondpassword2",
		},
	}
	if _, err := o.InstallTemplate(ctx, projectID, "pocketbase", "mypb", nil, overrides); err == nil {
		t.Error("expected InstallTemplate to fail when a post-deploy command exits nonzero")
	}
}
