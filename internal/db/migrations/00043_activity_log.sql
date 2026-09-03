-- +goose Up
-- Die Prüfspur. user_id und website_id lösen auf SET NULL auf, damit ein
-- gelöschter Benutzer die Einträge nicht mitnimmt: gerade dann will man
-- nachlesen können, was vorher geschah. Die Adresse steht deshalb zusätzlich
-- als actor_email in der Zeile — sie überlebt das Konto.
CREATE TABLE activity_log (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER REFERENCES users(id) ON DELETE SET NULL,
    actor_email  TEXT    NOT NULL DEFAULT '',
    action       TEXT    NOT NULL,
    entity_type  TEXT    NOT NULL DEFAULT '',
    entity_id    INTEGER NOT NULL DEFAULT 0,
    website_id   INTEGER REFERENCES websites(id) ON DELETE SET NULL,
    metadata     TEXT    NOT NULL DEFAULT '{}',
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
) STRICT;

-- Vier Indizes, weil der Bildschirm vier Filter hat und jeder davon nach
-- Zeit sortiert ausgibt. Ohne den zusammengesetzten Index liest SQLite die
-- ganze Tabelle, sobald das Protokoll ein paar Monate alt ist.
CREATE INDEX idx_activity_log_created_at ON activity_log(created_at DESC);
CREATE INDEX idx_activity_log_user       ON activity_log(user_id, created_at DESC);
CREATE INDEX idx_activity_log_website    ON activity_log(website_id, created_at DESC);
CREATE INDEX idx_activity_log_action     ON activity_log(action, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS activity_log;
