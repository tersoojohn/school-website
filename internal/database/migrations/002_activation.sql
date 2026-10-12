-- One-time account setup links. Only the SHA-256 of the token is stored.
-- A link is used once (the row is deleted when the person sets a password) and expires after 72 hours.
CREATE TABLE activation_tokens (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);
CREATE INDEX idx_activation_user ON activation_tokens(user_id);
