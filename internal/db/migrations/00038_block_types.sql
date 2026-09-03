-- +goose Up

-- Eigene Bausteinarten je Website.
--
-- Der Bausteineditor kann neun Arten, und die stehen im Go-Code. Wer eine
-- zehnte braucht — einen Rezeptschritt, einen Öffnungszeiten-Kasten, eine
-- Preiszeile —, brauchte bisher eine neue Fassung des Programms. Das ist
-- derselbe Satz, mit dem die eigenen Felder angefangen haben, nur eine Ebene
-- tiefer.
--
-- Die Art steht hier, ihre Felder stehen in page_field_defs. Eine zweite
-- Feldtabelle hätte dieselben Spalten, dieselben Prüfungen, dieselbe
-- Reihenfolge-Logik und dieselben Eingabemasken noch einmal gebraucht — und
-- wäre über kurz oder lang auseinandergelaufen. Genau die Überlegung, aus der
-- in 00029 die Gruppen in dieselbe Tabelle gewandert sind.
CREATE TABLE block_types (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,

    -- kennung steht im gespeicherten Baustein jeder Seite und in der
    -- CSS-Klasse, die das Theme anspricht. Sie wird aus dem Namen gebildet und
    -- bleibt danach stehen.
    kennung TEXT NOT NULL,
    name    TEXT NOT NULL,
    hinweis TEXT NOT NULL DEFAULT '',

    position   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    UNIQUE (website_id, kennung)
) STRICT;

CREATE INDEX idx_block_types_website ON block_types(website_id, position);

-- Die Felder einer Bausteinart: dieselbe Tabelle, ein Verweis mehr.
--
-- Die Spalte darf keinen anderen Vorgabewert als NULL haben — SQLite lässt
-- ALTER TABLE ADD COLUMN mit REFERENCES sonst nicht zu. Das passt: NULL heisst
-- „gehört der Seite und keiner Bausteinart", und das trifft auf jede
-- bestehende Zeile zu.
ALTER TABLE page_field_defs
    ADD COLUMN block_type_id INTEGER REFERENCES block_types(id) ON DELETE CASCADE;

-- Die Eindeutigkeit wird neu gezogen. Sie steht seit 00029 als eigener Index
-- da und nicht am Tabellenkopf — deshalb ist das hier ein Austausch von zwei
-- Indizes und kein Tabellenneubau.
--
-- „titel" darf es in den Seitenfeldern einmal geben und in jeder Bausteinart
-- noch einmal: es sind getrennte Namensräume, und ein Rezeptschritt mit einem
-- Titel darf eine Website nicht daran hindern, auch ein Seitenfeld „Titel" zu
-- haben.
DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung)
    WHERE parent_id IS NULL AND block_type_id IS NULL;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_baustein
    ON page_field_defs(block_type_id, kennung)
    WHERE block_type_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_page_field_defs_kennung_baustein;
DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;
DELETE FROM page_field_defs WHERE block_type_id IS NOT NULL;
ALTER TABLE page_field_defs DROP COLUMN block_type_id;
CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung) WHERE parent_id IS NULL;
DROP INDEX IF EXISTS idx_block_types_website;
DROP TABLE IF EXISTS block_types;
