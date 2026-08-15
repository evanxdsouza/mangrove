-- Adds horizontal scaling to a deployment: a `replicas` count on
-- deployments and a per-service record of the full set of running
-- replica container IDs.
--
-- A service still has a single "primary" container
-- (services.container_id_current) -- what the health-check scheduler,
-- logs, stats, and `docker exec` operate on -- but a deployment with
-- replicas > 1 runs that many containers of the same image behind one
-- load-balanced Caddy route. The extra container IDs live in
-- services.replica_container_ids (a JSON array, primary first) so stop/
-- restart/teardown can act on every replica, not just the primary.
--
-- Both are plain ADD COLUMN, so no table rebuild is needed.

ALTER TABLE deployments ADD COLUMN replicas INTEGER NOT NULL DEFAULT 1;
ALTER TABLE services ADD COLUMN replica_container_ids TEXT;
