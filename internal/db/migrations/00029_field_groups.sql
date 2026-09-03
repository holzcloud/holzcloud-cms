-- +goose Up

-- Wiederholbare Feldgruppen.
--
-- Ein einzelnes Feld reicht für einen Preis. Es reicht nicht für die
-- Öffnungszeiten (mehrere Zeilen aus Tag, von, bis), für die Mitglieder eines
-- Teams oder für die Varianten eines Produkts. Dafür braucht es eine Gruppe von
-- Feldern, die man mehrfach ausfüllt.
--
-- Die Unterfelder stehen in derselben Tabelle und zeigen mit parent_id auf ihre
-- Gruppe. Eine zweite Tabelle hätte dieselben Spalten, dieselben Prüfungen und
-- dieselbe Reihenfolge-Logik noch einmal gebraucht — und wäre über kurz oder
-- lang auseinandergelaufen.
--
-- Genau eine Ebene: eine Gruppe in einer Gruppe wird beim Anlegen abgelehnt.
-- Verschachtelung kostet einen Editor, der sich selbst aufruft, und ein
-- Formular, in dem sich niemand mehr zurechtfindet.
--
-- Die Tabelle wird neu gebaut statt erweitert: der UNIQUE-Zwang über
-- (website_id, kennung) steht am Tabellenkopf aus 00028 und muss weg, denn
-- "von" darf es in den Öffnungszeiten und in den Kurszeiten geben. Eine
-- Bedingung am Tabellenkopf lässt sich in SQLite nicht nachträglich lockern.

-- +goose StatementBegin
CREATE TABLE page_field_defs_neu (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    parent_id  INTEGER REFERENCES page_field_defs_neu(id) ON DELETE CASCADE,

    kennung      TEXT NOT NULL,
    beschriftung TEXT NOT NULL,
    art          TEXT NOT NULL,

    pflicht   INTEGER NOT NULL DEFAULT 0 CHECK (pflicht IN (0, 1)),
    hinweis   TEXT NOT NULL DEFAULT '',
    auswahl   TEXT NOT NULL DEFAULT '',
    gilt_fuer TEXT NOT NULL DEFAULT 'beides',

    position   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;
-- +goose StatementEnd

INSERT INTO page_field_defs_neu
    (id, website_id, parent_id, kennung, beschriftung, art, pflicht, hinweis, auswahl, gilt_fuer, position, created_at)
SELECT id, website_id, NULL, kennung, beschriftung, art, pflicht, hinweis, auswahl, gilt_fuer, position, created_at
FROM page_field_defs;

DROP INDEX IF EXISTS idx_page_field_defs_website;
DROP TABLE page_field_defs;
ALTER TABLE page_field_defs_neu RENAME TO page_field_defs;

CREATE INDEX idx_page_field_defs_website ON page_field_defs(website_id, parent_id, position);

-- Eindeutig je Gruppe, nicht je Website. Zwei Teilausdrücke, weil NULL in
-- SQLite nie gleich NULL ist und ein einzelner Index die Felder auf oberster
-- Ebene deshalb gar nicht prüfen würde.
CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX idx_page_field_defs_kennung_gruppe
    ON page_field_defs(parent_id, kennung) WHERE parent_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_page_field_defs_kennung_gruppe;
DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;
DELETE FROM page_field_defs WHERE parent_id IS NOT NULL;
