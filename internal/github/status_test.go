package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostStatusSendsExpectedRequest(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := &StatusClient{HTTPClient: srv.Client(), BaseURL: srv.URL}
	err := c.PostStatus(context.Background(), "ghp_test123", "evanxdsouza", "myrepo", "abc123", StateSuccess, "Deployed successfully", "https://web.example.com")
	if err != nil {
		t.Fatalf("PostStatus: %v", err)
	}
	if gotAuth != "Bearer ghp_test123" {
		t.Errorf("Authorization header = %q, want Bearer ghp_test123", gotAuth)
	}
	if gotPath != "/repos/evanxdsouza/myrepo/statuses/abc123" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["state"] != "success" || gotBody["context"] != "mangrove" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestPostStatusRequiresSHA(t *testing.T) {
	c := NewStatusClient()
	if err := c.PostStatus(context.Background(), "tok", "o", "r", "", StatePending, "", ""); err == nil {
		t.Fatal("expected error for empty sha")
	}
}

func TestPostStatusPropagatesServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &StatusClient{HTTPClient: srv.Client(), BaseURL: srv.URL}
	if err := c.PostStatus(context.Background(), "bad-token", "o", "r", "sha", StateFailure, "x", ""); err == nil {
		t.Fatal("expected error for 401 response")
	}
}
