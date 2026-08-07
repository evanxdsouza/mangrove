// Command mangrove is the control-plane daemon: HTTP API, deploy
// orchestration, and (from later commits) the embedded dashboard SPA,
// webhook receiver, and background schedulers all run in this one process.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/evanxdsouza/mangrove/internal/api"
	"github.com/evanxdsouza/mangrove/internal/config"
	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/portregistry"
	"github.com/evanxdsouza/mangrove/internal/proxy"
	"github.com/evanxdsouza/mangrove/internal/secrets"
	"github.com/evanxdsouza/mangrove/internal/store"
	"github.com/evanxdsouza/mangrove/internal/sysinfo"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := config.Load()
	if err := run(cfg, log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	if err := sysinfo.VerifyCgroupV2(); err != nil {
		log.Warn("resource-floor protection may not be active", "reason", err)
	}

	db, err := mangrovedb.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	keyPath := filepath.Join(cfg.DataDir, "master.key")
	masterKey, err := secrets.LoadOrCreateMasterKey(keyPath)
	if err != nil {
		return fmt.Errorf("load master key: %w", err)
	}
	box, err := secrets.NewBox(masterKey)
	if err != nil {
		return fmt.Errorf("init secrets box: %w", err)
	}

	if err := portregistry.RegisterSystemPort(ctx, db, cfg.APIPort, "mangrove API/dashboard/webhook (fixed, not reassignable)"); err != nil {
		return fmt.Errorf("register system port: %w", err)
	}

	dockerExec, err := executor.NewDockerExecutor(ctx, cfg.NetworkName)
	if err != nil {
		return fmt.Errorf("init docker executor: %w", err)
	}
	composeExec := &executor.ComposeExecutor{NetworkName: cfg.NetworkName}

	proxyClient := proxy.NewClient(cfg.CaddyAdminAddr)
	if err := proxyClient.EnsureBaseConfig(ctx); err != nil {
		log.Warn("caddy admin API not reachable; deployed services will not be publicly routed until it is", "error", err)
	}

	st := store.New(db)
	orch := &orchestrator.Orchestrator{
		Store:   st,
		Exec:    dockerExec,
		Compose: composeExec,
		Proxy:   proxyClient,
		Secrets: box,
		Config:  cfg,
		Log:     log,
	}

	server := &api.Server{
		Store:        st,
		Orchestrator: orch,
		Secrets:      box,
		Log:          log,
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.APIPort))
	httpServer := &http.Server{
		Addr:    addr,
		Handler: server.Router(),
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("mangrove listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
