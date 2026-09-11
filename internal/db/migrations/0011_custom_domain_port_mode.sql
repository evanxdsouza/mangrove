-- Custom domains, port-routing mode: an alternative to the default
-- automatic-HTTPS/:80/:443 routing (internal/proxy/caddy.go's srv_public)
-- for boxes that can't be reached on :80/:443 at all -- e.g. Hack Club
-- Nest, where a box only receives traffic on whatever port the owner
-- registers against a domain in Nest's own dashboard, which then handles
-- HTTPS termination itself. DNS TXT ownership verification (the default
-- mode's anti-hijack check) doesn't apply either: proving you can register
-- a domain+port pair in your own Nest dashboard already proves you control
-- where that domain's traffic goes, the same trust boundary a plain
-- deployment's own port already relies on.
--
-- MANGROVE_CUSTOM_DOMAIN_MODE=port (internal/config) switches every new
-- domain into this mode: no verification_token wait, a dedicated port is
-- allocated immediately from the same pool as service ports, and Mangrove
-- programs a normal per-port Caddy route (PutRoute/PutFileServerRoute --
-- the exact same calls a deployment's own base route uses) instead of a
-- host-matched srv_public route. See internal/orchestrator/domains.go.
ALTER TABLE custom_domains ADD COLUMN routing_mode TEXT NOT NULL DEFAULT 'auto_tls';
ALTER TABLE custom_domains ADD COLUMN port INTEGER;

-- SQLite can't ALTER a CHECK constraint in place, so allocation_type's enum
-- is extended by rebuilding port_registry (see db.go's applyMigration,
-- which runs this with foreign_keys off around the DROP).
CREATE TABLE port_registry_new (
    id INTEGER PRIMARY KEY,
    port INTEGER NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'allocated' CHECK (status IN ('allocated','reserved','free')),
    allocation_type TEXT NOT NULL CHECK (allocation_type IN ('service_public','service_direct_publish','system','manual_external','custom_domain_port')),
    service_id INTEGER,
    note TEXT,
    reserved_by_user_id INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO port_registry_new SELECT * FROM port_registry;

DROP TABLE port_registry;
ALTER TABLE port_registry_new RENAME TO port_registry;
