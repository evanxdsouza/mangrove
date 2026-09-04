-- Storage/NAS shares: a service that bind-mounts a specific host directory
-- (a drive internal/mountd mounted) and publishes a port directly on the
-- host's real network interface instead of going through Caddy -- SMB
-- isn't HTTP, so Caddy (which only reverse-proxies/file-serves HTTP) can't
-- route it the way every other public deployment is routed. See
-- internal/orchestrator/storage.go and docs/storage.md.
--
-- direct_publish_port is deliberately NOT a port_registry FK the way
-- services.host_port is: port_registry's range is for Caddy-routed
-- deployments (see internal/portregistry), and a NAS share's port (445,
-- the standard SMB port) is fixed and published straight onto the host,
-- not allocated from that pool.
ALTER TABLE services ADD COLUMN direct_publish_port INTEGER;
ALTER TABLE services ADD COLUMN host_mount_source TEXT;
ALTER TABLE services ADD COLUMN host_mount_target TEXT;
