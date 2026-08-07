package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendDeploySuccessSendsExpectedRequest(t *testing.T) {
	var gotReq sendRequest
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&gotReq)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"test-id"}`))
	}))
	defer srv.Close()

	c := NewResendClient("test-api-key", "Mangrove <test@example.com>")
	c.HTTPClient = srv.Client()
	prev := resendAPIURL
	resendAPIURL = srv.URL
	defer func() { resendAPIURL = prev }()

	err := c.SendDeploySuccess(context.Background(), "evan@example.com", DeploySuccessParams{
		AppName: "my-app", Port: 20001, SuggestedDomain: "my-app.example.hackclub.app", CommitSHA: "abcdef1234567890",
	})
	if err != nil {
		t.Fatalf("SendDeploySuccess: %v", err)
	}

	if gotAuth != "Bearer test-api-key" {
		t.Errorf("expected Authorization header 'Bearer test-api-key', got %q", gotAuth)
	}
	if gotReq.From != "Mangrove <test@example.com>" {
		t.Errorf("unexpected From: %q", gotReq.From)
	}
	if len(gotReq.To) != 1 || gotReq.To[0] != "evan@example.com" {
		t.Errorf("unexpected To: %v", gotReq.To)
	}
	if gotReq.Subject != "my-app deployed successfully" {
		t.Errorf("unexpected Subject: %q", gotReq.Subject)
	}
}

func TestSendDeploySuccessPropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"invalid API key"}`))
	}))
	defer srv.Close()

	c := NewResendClient("bad-key", "")
	c.HTTPClient = srv.Client()
	prev := resendAPIURL
	resendAPIURL = srv.URL
	defer func() { resendAPIURL = prev }()

	err := c.SendDeploySuccess(context.Background(), "evan@example.com", DeploySuccessParams{AppName: "app", Port: 1})
	if err == nil {
		t.Fatal("expected an error for a 401 response, got nil")
	}
}

func TestEnabledReflectsAPIKeyPresence(t *testing.T) {
	if (&ResendClient{}).Enabled() {
		t.Error("expected Enabled() to be false with no API key")
	}
	if !(&ResendClient{APIKey: "x"}).Enabled() {
		t.Error("expected Enabled() to be true with an API key set")
	}
	var nilClient *ResendClient
	if nilClient.Enabled() {
		t.Error("expected Enabled() to be false on a nil client")
	}
}

func TestNewResendClientDefaultsFromAddress(t *testing.T) {
	c := NewResendClient("key", "")
	if c.FromEmail != "Mangrove <onboarding@resend.dev>" {
		t.Errorf("expected default from address, got %q", c.FromEmail)
	}
}
