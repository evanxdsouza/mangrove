package scheduler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// DDNSUpdater keeps a home-server operator's domain pointed at this box's
// current public IP -- see docs/deployment.md's "Home server / DDNS"
// section and setup.sh's "home" install mode. Only needed when the box
// sits behind a router without a static IP; a VPS/Nest install leaves
// Domain empty and this job is never started (see cmd/mangrove/main.go).
//
// Provider is a fixed, small set rather than a generic pluggable
// interface: DuckDNS's update API (a single GET request, empty ip= param
// auto-detecting the caller's public IP) is free, simple, and exactly
// what the mocinno/hackclub audience this feature targets already
// reaches for -- adding pluggability before a second provider is actually
// needed would be speculative.
const duckDNSUpdateURL = "https://www.duckdns.org/update"

type DDNSUpdater struct {
	Domain   string // DuckDNS subdomain, without the .duckdns.org suffix
	Token    string
	Provider string // only "duckdns" is currently supported
	Log      *slog.Logger
	HTTP     *http.Client
	interval time.Duration
	// duckdnsURL is duckDNSUpdateURL in production; tests override it to
	// point at an httptest server instead of making a real network call.
	duckdnsURL string
}

func NewDDNSUpdater(domain, token, provider string, log *slog.Logger) *DDNSUpdater {
	return &DDNSUpdater{
		Domain:     domain,
		Token:      token,
		Provider:   provider,
		Log:        log,
		HTTP:       &http.Client{Timeout: 10 * time.Second},
		interval:   5 * time.Minute,
		duckdnsURL: duckDNSUpdateURL,
	}
}

// Run blocks, ticking until ctx is canceled. Updates once immediately on
// start (rather than waiting a full interval) so a restart after the
// box's IP already changed doesn't leave the domain stale for up to 5
// minutes.
func (d *DDNSUpdater) Run(ctx context.Context) {
	d.tick(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.tick(ctx)
		}
	}
}

func (d *DDNSUpdater) tick(ctx context.Context) {
	if err := d.update(ctx); err != nil {
		d.Log.Warn("ddns update failed", "provider", d.Provider, "domain", d.Domain, "error", err)
	}
}

func (d *DDNSUpdater) update(ctx context.Context) error {
	if d.Provider != "duckdns" {
		return fmt.Errorf("unsupported ddns provider %q (only \"duckdns\" is supported)", d.Provider)
	}

	url := fmt.Sprintf("%s?domains=%s&token=%s&ip=", d.duckdnsURL, d.Domain, d.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("reach duckdns: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// DuckDNS always answers 200 with "OK"/"KO" in the body, not an HTTP
	// error status, to keep its API scriptable with plain curl.
	if string(body) != "OK" {
		return fmt.Errorf("duckdns returned %q (check MANGROVE_DDNS_DOMAIN/MANGROVE_DDNS_TOKEN)", body)
	}
	return nil
}
