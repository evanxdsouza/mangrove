package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
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
	runSpecs       []executor.RunSpec // every RunSpec Run() was called with, in order -- so tests can assert on what actually would have hit `docker run`, not just that Deploy() returned success
	removedRefs    []string           // every container ref Remove() was called with
	removedVolumes []string           // every volume name RemoveVolume() was called with
	removedImages  []string           // every image tag RemoveImage() was called with
}

func (f *fakeTemplateExecutor) Build(ctx context.Context, spec executor.BuildSpec, logs io.Writer) (executor.BuildResult, error) {
	f.builds++
	return executor.BuildResult{ImageTag: spec.ImageRef}, nil
}
func (f *fakeTemplateExecutor) Run(ctx context.Context, spec executor.RunSpec) (executor.RunResult, error) {
	f.runSpecs = append(f.runSpecs, spec)
	return executor.RunResult{ContainerID: "container-" + spec.ContainerName, ContainerAddr: "10.0.0.1:1234"}, nil
}
func (f *fakeTemplateExecutor) Stop(ctx context.Context, ref string, timeout time.Duration) error {
	return nil
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

	proj, err := st.CreateProject(context.Background(), "Template Test", "template-test", "")
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

	result, err := o.InstallTemplate(ctx, projectID, "postgres", "mydb", nil)
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

	if _, err := o.InstallTemplate(ctx, projectID, "redis", "cache", nil); err != nil {
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

func TestInstallTemplateLinkedDependencyResolvesAliasAndGeneratedValue(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()

	result, err := o.InstallTemplate(ctx, projectID, "wordpress", "blog", nil)
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
