-- +goose Up

-- One-time links for inviting a colleague and for resetting a password.
--
-- Deliberately no SMTP: sending mail would be the first code in this project
-- that dials out at runtime, which the "nothing loads at runtime" rule exists to
-- prevent. An admin-generated link needs no egress at all — it is copied out of
-- the admin and handed over by whatever channel the operator already uses.
--
-- Only the hash is stored, the same discipline the CSRF key follows: a stolen
-- database must not yield working links.
CREATE TABLE user_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT    NOT NULL UNIQUE,
    purpose    TEXT    NOT NULL CHECK (purpose IN ('invite', 'reset')),
    expires_at TEXT    NOT NULL,
    used_at    TEXT,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE INDEX idx_user_tokens_user ON user_tokens(user_id, purpose);

-- A password an admin typed and passed on stays the real password forever.
-- must_change_password ends that at the next login.
ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0
    CHECK (must_change_password IN (0, 1));
ALTER TABLE users ADD COLUMN last_login_at TEXT;

-- Which addresses visitors ask for and do not get.
--
-- The redirect table can only count hits on redirects that already exist; it can
-- say nothing about a URL nobody ever wrote a redirect for, which is the entire
-- population of broken inbound links. Aggregated per path rather than one row
-- per event, so it cannot fill the card.
CREATE TABLE not_found_log (
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    path       TEXT    NOT NULL,
    hits       INTEGER NOT NULL DEFAULT 1,
    referrer   TEXT    NOT NULL DEFAULT '',
    first_seen TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_seen  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (website_id, path)
) STRICT, WITHOUT ROWID;

-- +goose Down
DROP TABLE IF EXISTS not_found_log;
ALTER TABLE users DROP COLUMN last_login_at;
ALTER TABLE users DROP COLUMN must_change_password;
DROP INDEX IF EXISTS idx_user_tokens_user;
DROP TABLE IF EXISTS user_tokens;
