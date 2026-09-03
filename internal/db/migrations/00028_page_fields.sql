-- +goose Up

-- Eigene Felder je Website.
--
-- Bis hierher hatte eine Seite die Felder, die im Go-Code stehen: Titel,
-- Adresse, Inhalt, Kurzfassung, Vorschaubild. Wer ein Feld "Preis" brauchte,
-- brauchte eine neue Fassung des Programms. Das ist der Unterschied zwischen
-- einem CMS für diese Websites und einem CMS für Websites.
--
-- Die Definition steht in einer Tabelle, die Werte stehen als JSON an der
-- Seite. Nicht andersherum: eine Spalte je Feld hiesse, dass jedes neue Feld
-- die Tabelle umbaut — bei STRICT-Tabellen ein vollständiger Neuaufbau, und
-- das mitten im Betrieb auf einer SD-Karte.
CREATE TABLE page_field_defs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,

    -- kennung ist der Name im Theme: {{ .Page.Felder.preis }}. Sie wird beim
    -- Anlegen aus der Beschriftung erzeugt und bleibt danach stehen — würde
    -- sie sich mitändern, verlöre jede Seite bei jeder Umformulierung ihren
    -- Wert, und das Theme zeigte plötzlich nichts mehr an.
    kennung TEXT NOT NULL,
    beschriftung TEXT NOT NULL,

    -- art ist eine der acht Eingabearten. Kein CHECK: eine neue Art wäre sonst
    -- wieder ein Tabellenumbau, und geprüft wird ohnehin beim Speichern.
    art TEXT NOT NULL,

    pflicht INTEGER NOT NULL DEFAULT 0 CHECK (pflicht IN (0, 1)),
    hinweis TEXT NOT NULL DEFAULT '',

    -- auswahl sind die Möglichkeiten einer Auswahlliste, eine je Zeile.
    auswahl TEXT NOT NULL DEFAULT '',

    -- gilt_fuer trennt Seiten von Beiträgen: ein Preis gehört an ein Produkt,
    -- ein Autor an einen Beitrag, und beides gleichzeitig anzuzeigen macht das
    -- Formular länger, ohne dass es mehr kann.
    gilt_fuer TEXT NOT NULL DEFAULT 'beides',

    position   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    UNIQUE (website_id, kennung)
) STRICT;

CREATE INDEX idx_page_field_defs_website ON page_field_defs(website_id, position);

-- Die Werte, als JSON-Objekt {"kennung": "wert"}. Leer heisst: nichts
-- ausgefüllt, was für jede bestehende Seite zutrifft.
ALTER TABLE pages ADD COLUMN fields TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE pages DROP COLUMN fields;
DROP INDEX IF EXISTS idx_page_field_defs_website;
DROP TABLE IF EXISTS page_field_defs;
