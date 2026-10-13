package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/proxy"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// fakeCaddy is a Caddy admin API that starts empty (as after a restart
// without --resume) and records every config write.
type fakeCaddy struct {
	mu     sync.Mutex
	writes []string
}

func (f *fakeCaddy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Method == http.MethodGet {
		w.Write([]byte(`{}`))
		return
	}
	f.writes = append(f.writes, r.Method+" "+r.URL.Path)
}

func TestRestoreMissingRoutes(t *testing.T) {
	o, st, projectID := newTestOrchestrator(t)
	ctx := context.Background()
	caddy := &fakeCaddy{}
	srv := httptest.NewServer(caddy)
	defer srv.Close()
	o.Proxy = proxy.NewClient(srv.URL)

	newDep := func(slug, status string) int {
		dep, err := st.CreateDeployment(ctx, store.CreateDeploymentParams{ProjectID: projectID, Name: slug, Slug: slug, BuildStrategy: "image", ImageRef: "nginx"})
		if err != nil {
			t.Fatal(err)
		}
		svc, err := st.CreateService(ctx, store.CreateServiceParams{DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-" + slug + "-web", InternalPort: 80})
		if err != nil {
			t.Fatal(err)
		}
		port, err := portregistry.AllocateForService(ctx, st.DB, svc.ID, 20000, 21000)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.UpdateServiceRuntime(ctx, svc.ID, "nginx", "cid-"+slug, "running"); err != nil {
			t.Fatal(err)
		}
		if err := st.UpdateDeploymentStatus(ctx, dep.ID, status); err != nil {
			t.Fatal(err)
		}
		return port
	}
	running := newDep("live", "running")
	stopped := newDep("off", "stopped")

	n, err := o.RestoreMissingRoutes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("restored %d deployments, want 1", n)
	}
	want := "/config/apps/http/servers/" + proxy.PortServerName(running)
	found := false
	for _, w := range caddy.writes {
		if w == "PATCH "+want || w == "PUT "+want {
			found = true
		}
		if w == "PATCH /config/apps/http/servers/"+proxy.PortServerName(stopped) {
			t.Errorf("stopped deployment's route was restored: %v", caddy.writes)
		}
	}
	if !found {
		t.Fatalf("expected a write to %s, got %v", want, caddy.writes)
	}
}
