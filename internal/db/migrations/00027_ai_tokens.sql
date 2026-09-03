-- +goose Up

-- Zugangsschlüssel für eine KI, die der Betreiber anschliesst.
--
-- Gedacht ist es so: jemand verbindet seinen eigenen KI-Assistenten mit dem
-- laufenden CMS und schreibt damit Inhalte. Der Assistent ist ein Programm auf
-- einem fremden Rechner; er bekommt einen Schlüssel und sonst nichts.
--
-- Nur der Abdruck steht hier, nie der Schlüssel selbst. Wer die Datenbank in
-- die Hände bekommt — ein Abzug, ein Backup, ein verlorener Laptop —, findet
-- damit nichts, womit er sich anmelden könnte. Das ist derselbe Grund, aus dem
-- ein Passwort nicht im Klartext steht.
CREATE TABLE ai_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    token_hash TEXT    NOT NULL UNIQUE,

    -- website_id NULL heisst: alle Websites. Wer nur eine betreut, soll auch
    -- nur an diese eine herankommen.
    website_id INTEGER REFERENCES websites(id) ON DELETE CASCADE,

    -- can_write trennt Lesen von Schreiben. Ein Schlüssel, der nur liest, ist
    -- der, den man vergibt, wenn man erst einmal sehen will, was passiert —
    -- und es soll ihn geben, bevor jemand ihn vermisst.
    can_write INTEGER NOT NULL DEFAULT 0 CHECK (can_write IN (0, 1)),

    -- last_used_at ist die einzige Spur, an der ein Betreiber ablesen kann,
    -- ob ein Schlüssel noch benutzt wird — und ob einer benutzt wird, von dem
    -- er das nicht erwartet hätte.
    last_used_at TEXT,
    expires_at   TEXT,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE INDEX idx_ai_tokens_website ON ai_tokens(website_id);

-- +goose Down
DROP INDEX IF EXISTS idx_ai_tokens_website;
DROP TABLE IF EXISTS ai_tokens;
