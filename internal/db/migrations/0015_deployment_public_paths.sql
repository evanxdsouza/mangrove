-- Paths on a password-protected deployment that skip the gate entirely
-- (a landing page, a health endpoint, a public docs section). Stored
-- newline-separated; each entry is an exact path ("/pricing") or a prefix
-- when it ends in "*" ("/docs/*"). Empty means the whole deployment is gated.
ALTER TABLE deployments ADD COLUMN public_paths TEXT NOT NULL DEFAULT '';
