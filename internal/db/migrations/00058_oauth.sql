-- +goose Up

-- Signing an assistant in with OAuth instead of a key pasted by hand.
--
-- Claude, ChatGPT and the other hosted assistants connect to an MCP server only
-- through OAuth: they register themselves, send the operator to a page on this
-- server to agree, and receive a key in return. The key they receive is an
-- ordinary row in ai_tokens — same scope, same revocation, same line on the key
-- screen — so there is still exactly one place that decides what an assistant
-- may do.

-- A client is an assistant that introduced itself (RFC 7591). Registering costs
-- nothing and grants nothing: until an administrator agrees on the consent
-- page, a client can do no more than exist.
CREATE TABLE oauth_clients (
    client_id     TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    -- One address per line. A code is only ever sent to one of these, compared
    -- as whole strings, never as prefixes.
    redirect_uris TEXT NOT NULL,
    -- NULL for a public client, which proves itself with PKCE alone.
    secret_hash   TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

-- An authorisation code lives for minutes and is used once. Only its hash is
-- kept, like every other secret here.
CREATE TABLE oauth_codes (
    code_hash    TEXT PRIMARY KEY,
    client_id    TEXT NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    challenge    TEXT NOT NULL,
    website_id   INTEGER REFERENCES websites(id) ON DELETE CASCADE,
    can_write    INTEGER NOT NULL CHECK (can_write IN (0, 1)),
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at   TEXT NOT NULL
) STRICT;

-- A key handed out through OAuth belongs to its client, and carries a refresh
-- secret: the key itself lives an hour, the refresh secret renews it and is
-- replaced every time it is used.
--
-- client_id is a plain column and not a foreign key: SQLite cannot drop a
-- column that is one, and the way down has to exist. A client is only ever
-- pruned while it has no keys, so nothing is left pointing nowhere.
ALTER TABLE ai_tokens ADD COLUMN client_id TEXT;
ALTER TABLE ai_tokens ADD COLUMN refresh_hash TEXT;
ALTER TABLE ai_tokens ADD COLUMN refresh_expires_at TEXT;
CREATE UNIQUE INDEX idx_ai_tokens_refresh ON ai_tokens(refresh_hash) WHERE refresh_hash IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_ai_tokens_refresh;
DELETE FROM ai_tokens WHERE client_id IS NOT NULL;
ALTER TABLE ai_tokens DROP COLUMN refresh_expires_at;
ALTER TABLE ai_tokens DROP COLUMN refresh_hash;
ALTER TABLE ai_tokens DROP COLUMN client_id;
DROP TABLE IF EXISTS oauth_codes;
DROP TABLE IF EXISTS oauth_clients;
