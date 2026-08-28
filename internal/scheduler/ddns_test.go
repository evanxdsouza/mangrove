package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func testUpdater(t *testing.T, handler http.HandlerFunc) *DDNSUpdater {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &DDNSUpdater{
		Domain:     "myhome",
		Token:      "test-token",
		Provider:   "duckdns",
		Log:        discardLogger(),
		HTTP:       srv.Client(),
		duckdnsURL: srv.URL,
	}
}

func TestDDNSUpdateSendsExpectedParams(t *testing.T) {
	var gotQuery url.Values
	u := testUpdater(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte("OK"))
	})

	if err := u.update(context.Background()); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := gotQuery.Get("domains"); got != "myhome" {
		t.Errorf("domains = %q, want myhome", got)
	}
	if got := gotQuery.Get("token"); got != "test-token" {
		t.Errorf("token = %q, want test-token", got)
	}
}

func TestDDNSUpdateFailsOnKOResponse(t *testing.T) {
	u := testUpdater(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("KO"))
	})

	if err := u.update(context.Background()); err == nil {
		t.Fatal("expected error on KO response, got nil")
	}
}

func TestDDNSUpdateRejectsUnsupportedProvider(t *testing.T) {
	u := testUpdater(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make a request for an unsupported provider")
	})
	u.Provider = "no-ip"

	if err := u.update(context.Background()); err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
}
