-- +goose Up

-- Das 404-Protokoll ist ein Plugin geworden.
--
-- Die Sammlung war nie Kernaufgabe: Sie ist eine Aufzeichnung dessen, was
-- Besucher getan haben, und ob so etwas auf dem Server liegt, soll der
-- Betreiber entscheiden können — durch Installieren oder Nichtinstallieren,
-- nicht durch einen Schalter in einer Tabelle, die ohnehin mitläuft.
--
-- Die Zeilen werden nicht übernommen. Das Plugin hat einen eigenen, nach
-- Website getrennten Speicher, und ein stiller Umzug fremder Daten in dessen
-- Ablage wäre genau die Art von Nebenwirkung, die eine Migration nicht haben
-- soll. Wer die alte Liste braucht, hat sie im Vor-Upgrade-Abzug.
DROP TABLE IF EXISTS not_found_log;

-- +goose Down

CREATE TABLE not_found_log (
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    path       TEXT    NOT NULL,
    hits       INTEGER NOT NULL DEFAULT 1,
    referrer   TEXT    NOT NULL DEFAULT '',
    first_seen TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_seen  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (website_id, path)
) STRICT, WITHOUT ROWID;
