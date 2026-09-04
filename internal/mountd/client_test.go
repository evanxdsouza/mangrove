package mountd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestClientServerRoundTrip exercises the real Unix-socket wire protocol
// end to end (encode/decode, connection-per-request, error propagation)
// without touching real block devices -- it only hits request shapes that
// fail validation before any exec.Command runs (missing uuid, unknown
// action), which is exactly the boundary between "protocol works" and
// "device enumeration works" (covered separately by TestFilterDrives_*
// against fixture lsblk output).
func TestClientServerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "mountd.sock")
	srv := &Server{
		SocketPath: sockPath,
		MountRoot:  filepath.Join(dir, "drives"),
		Log:        slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()

	// Wait for the socket to appear rather than a fixed sleep.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(sockPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("socket never appeared at %s", sockPath)
		}
		time.Sleep(10 * time.Millisecond)
	}

	client := NewClient(sockPath)

	t.Run("mount without uuid is rejected", func(t *testing.T) {
		_, err := client.call(context.Background(), Request{Action: ActionMount})
		if err == nil {
			t.Fatal("expected an error for a mount request with no uuid")
		}
	})

	t.Run("unknown action is rejected", func(t *testing.T) {
		_, err := client.call(context.Background(), Request{Action: "reformat-everything"})
		if err == nil {
			t.Fatal("expected an error for an unrecognized action")
		}
	})

	t.Run("unmount of an unknown uuid fails cleanly", func(t *testing.T) {
		err := client.Unmount(context.Background(), "not-a-real-uuid")
		if err == nil {
			t.Fatal("expected an error unmounting a uuid that was never mounted")
		}
	})
}

func TestClient_Unavailable(t *testing.T) {
	client := NewClient("/nonexistent/path/to/a/socket.sock")
	_, err := client.List(context.Background())
	if err != ErrUnavailable {
		t.Fatalf("List() against a missing socket = %v, want ErrUnavailable", err)
	}
}
