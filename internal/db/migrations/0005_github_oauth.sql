-- GitHub OAuth: an OAuth-obtained token is stored as just another row in
-- github_pats (source='oauth' instead of 'pat'), sealed the same way a
-- pasted PAT already is. That means linkProjectRepo, the webhook receiver,
-- and the deploy pipeline need zero changes to accept it -- see plan's
-- GitHub auto-deploy section.

ALTER TABLE github_pats ADD COLUMN source TEXT NOT NULL DEFAULT 'pat';
ALTER TABLE github_pats ADD COLUMN github_login TEXT;

-- Short-lived, single-use CSRF state for the OAuth redirect dance. Rows are
-- deleted once consumed; expired-but-unconsumed rows are just ignored (no
-- background sweep needed, the table stays tiny).
CREATE TABLE github_oauth_states (
    id INTEGER PRIMARY KEY,
    state TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    redirect_uri TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);
