-- Custom domains a user points at one of their deployments -- the
-- "Domains dashboard" feature. Unlike deployments.slug/BaseDomain (which
-- are purely cosmetic, see internal/config/config.go's BaseDomain
-- comment), a verified row here is actually programmed into Caddy as a
-- host-matched route on the shared :443/:80 public server block, with
-- Caddy's automatic HTTPS provisioning the certificate. See
-- internal/orchestrator/domains.go and internal/proxy/caddy.go's
-- PutDomainRoute/DeleteDomainRoute.
CREATE TABLE custom_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id INTEGER NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    hostname TEXT NOT NULL,
    -- Required as a DNS TXT record (mangrove-domain-verification=<token>)
    -- on hostname before Mangrove will program a live route for it --
    -- otherwise anyone who points a domain's DNS at this box's IP could
    -- silently steal traffic meant for someone else's deployment.
    verification_token TEXT NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_custom_domains_hostname ON custom_domains(hostname);
CREATE INDEX idx_custom_domains_deployment_id ON custom_domains(deployment_id);
