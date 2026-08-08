-- Adds an optional command override for a service's container, stored as
-- a JSON array of argv strings (e.g. '["sh","-c","redis-server --requirepass \"$REDIS_PASSWORD\""]').
-- Needed by templates whose image requires flags the image's default CMD
-- doesn't set (e.g. Redis's --requirepass) -- see executor.RunSpec.Command.
ALTER TABLE services ADD COLUMN command TEXT;
