package notify

import (
	"context"
	"os"
	"testing"
)

// TestLiveSend is a manual, opt-in check against the real Resend API --
// skipped unless MANGROVE_LIVE_EMAIL_TEST=1 is set, so it never runs as
// part of the normal test suite (it costs real API quota and sends a real
// email). Run it once after wiring a new API key to confirm the
// integration actually works end-to-end, not just against a mock:
//
//	MANGROVE_LIVE_EMAIL_TEST=1 MANGROVE_RESEND_API_KEY=... MANGROVE_TEST_TO_EMAIL=... \
//	  go test ./internal/notify/ -run TestLiveSend -v
func TestLiveSend(t *testing.T) {
	if os.Getenv("MANGROVE_LIVE_EMAIL_TEST") != "1" {
		t.Skip("set MANGROVE_LIVE_EMAIL_TEST=1 to run this against the real Resend API")
	}
	apiKey := os.Getenv("MANGROVE_RESEND_API_KEY")
	to := os.Getenv("MANGROVE_TEST_TO_EMAIL")
	if apiKey == "" || to == "" {
		t.Fatal("MANGROVE_RESEND_API_KEY and MANGROVE_TEST_TO_EMAIL must both be set")
	}

	c := NewResendClient(apiKey, "")
	err := c.SendDeploySuccess(context.Background(), to, DeploySuccessParams{
		AppName:         "mangrove-live-test",
		Port:            20001,
		SuggestedDomain: "mangrove-live-test.evanxdsouza.hackclub.app",
		CommitSHA:       "livetest0000000",
	})
	if err != nil {
		t.Fatalf("live send failed: %v", err)
	}
}
