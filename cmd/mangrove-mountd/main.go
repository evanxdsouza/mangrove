// Command mangrove-mountd is the privileged half of Mangrove's storage/NAS
// feature: it owns mounting and unmounting removable drives so the main
// mangrove.service process never needs mount capabilities itself. Runs as
// its own systemd unit (deploy/systemd/mangrove-mountd.service), typically
// as root, and talks to the main process over a local Unix domain socket
// only -- see internal/mountd and docs/storage.md.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/evanxdsouza/mangrove/internal/mountd"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &mountd.Server{
		SocketPath:  getEnv("MANGROVE_MOUNTD_SOCKET", "/run/mangrove-mountd.sock"),
		MountRoot:   getEnv("MANGROVE_MOUNTD_ROOT", "/var/lib/mangrove-drives"),
		SocketGroup: getEnv("MANGROVE_MOUNTD_GROUP", "mangrove-mount"),
		Log:         log,
	}

	log.Info("mangrove-mountd listening", "socket", srv.SocketPath, "mount_root", srv.MountRoot)
	if err := srv.ListenAndServe(ctx); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
