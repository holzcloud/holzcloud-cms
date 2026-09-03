-- +goose Up

-- Eigene Inhaltsarten je Website.
--
-- Bisher gab es genau zwei: eine Seite und einen Beitrag. Das ist die letzte
-- Stelle, an der dieses CMS dem Betreiber vorschreibt, woraus seine Website
-- besteht — wer Produkte, Termine, Rezepte oder Tiere führt, hat sie bisher
-- Beiträge genannt und die Bezeichnung überall ertragen.
--
-- Eine Art ist bewusst wenig: ein Name, eine Mehrzahl, wahlweise eine eigene
-- Übersichtsseite und eine Reihenfolge. Sie ändert nicht, wo ein Eintrag
-- wohnt: jeder Eintrag liegt weiterhin unter seiner eigenen Adresse. Ein
-- Präfix je Art hätte jede bestehende Adresse verschoben, sobald jemand eine
-- Art anlegt — und eine Adresse, die sich ändert, ist ein Link, der bricht.
CREATE TABLE content_types (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,

    -- kennung steht in pages.art. Sie steht fest, sobald es Einträge gibt:
    -- sie umzubenennen hiesse, jeden Eintrag umzuhängen.
    kennung TEXT NOT NULL,
    -- Einzahl und Mehrzahl, weil beide auf dem Bildschirm vorkommen: "Neues
    -- Produkt" und "Produkte". Eine Art, die nur die Einzahl kennt, schreibt
    -- "Produkt (3)" in die Liste.
    name     TEXT NOT NULL,
    mehrzahl TEXT NOT NULL,

    -- Die Adresse der Übersichtsseite, oder leer für eine Art ohne. Wie
    -- websites.blog_base ein reservierter Slug: eine Seite mit demselben
    -- Namen käme nie zum Zug, darum lehnt der Editor sie ab.
    archiv TEXT NOT NULL DEFAULT '',
    -- 'neueste' oder 'titel'. Ein Terminkalender liest sich nach Datum, eine
    -- Produktliste nach Namen.
    sortierung TEXT NOT NULL DEFAULT 'neueste',

    position   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    UNIQUE (website_id, kennung)
) STRICT;

CREATE INDEX idx_content_types_website ON content_types(website_id, position);

-- Welche eigene Art ein Eintrag hat. Leer heisst: eine der beiden eingebauten,
-- und die steht wie bisher in kind.
--
-- Eine zweite Spalte statt eines weiteren Werts in kind, und das hat einen
-- handfesten Grund: kind trägt ein CHECK (kind IN ('page','post')) aus
-- Migration 00014. Ein CHECK lässt sich in SQLite nur durch einen Neuaufbau
-- der Tabelle ändern — und an pages hängen die Fassungen, die Menüpunkte und
-- die Schlagwörter mit Fremdschlüsseln. Genau dieser Neuaufbau hat in 00031
-- schon einmal beinahe alle Menüeinträge gekostet. Eine zusätzliche Spalte
-- kostet nichts und niemanden.
--
-- Ein Eintrag einer eigenen Art bleibt deshalb kind='page': er wohnt unter
-- seiner Adresse, steht in keinem fremden Archiv und wird nur von seiner
-- eigenen Übersicht aufgezählt.
ALTER TABLE pages ADD COLUMN art TEXT NOT NULL DEFAULT '';

-- "Die Produkte dieser Website, nach Titel" ist die Abfrage der Übersicht.
CREATE INDEX idx_pages_art ON pages(website_id, art, published_at DESC)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_pages_art;
ALTER TABLE pages DROP COLUMN art;
DROP INDEX IF EXISTS idx_content_types_website;
DROP TABLE IF EXISTS content_types;
