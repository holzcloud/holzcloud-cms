-- +goose Up

-- The German column names become English.
--
-- This is LANG-05 of milestone v2.0, and it is the half of "the codebase
-- speaks English" that a reader meets without opening a Go file: a schema dump
-- is the first thing anybody looks at, and until now it answered in German.
--
-- No released migration is edited. 00028, 00029, 00036, 00038, 00046, 00047,
-- 00048 and 00049 keep their text and their comments exactly as they were
-- written — they are the record of why each column exists, and a record that
-- gets rewritten later stops being one. This file is the correction, as 00048
-- is for 00047 and for the same stated reason.
--
-- RENAME COLUMN rather than a table rebuild. SQLite carries the new name into
-- every index, trigger, view, CHECK constraint and REFERENCES clause that
-- mentions it, which is the whole reason this is one short file and not
-- twenty-four table rebuilds on an SD card.
--
-- What does NOT change: the values. A field kind is still 'langtext', a
-- collision rule is still 'uebergehen', a content kind is still whatever the
-- operator typed. Those sit in every existing database and travel in every
-- exported bundle, and .planning/phases/12-codebase-speaks-english/12-CONTEXT.md
-- §2 argues the case. .planning/GLOSSARY.md carries the rule this rests on and
-- the runtime bug that taught it: translating a stored value compiles cleanly
-- and is refused by SQLite when a CHECK constraint meets it.

-- page_field_defs — twelve of the twenty-four.
ALTER TABLE page_field_defs RENAME COLUMN kennung TO key;
ALTER TABLE page_field_defs RENAME COLUMN beschriftung TO label;
ALTER TABLE page_field_defs RENAME COLUMN art TO kind;
ALTER TABLE page_field_defs RENAME COLUMN pflicht TO required;
ALTER TABLE page_field_defs RENAME COLUMN hinweis TO hint;
ALTER TABLE page_field_defs RENAME COLUMN auswahl TO choices;
ALTER TABLE page_field_defs RENAME COLUMN gilt_fuer TO applies_to;
ALTER TABLE page_field_defs RENAME COLUMN bedingung TO condition;
ALTER TABLE page_field_defs RENAME COLUMN darstellung TO display;
ALTER TABLE page_field_defs RENAME COLUMN max_werte TO max_values;
ALTER TABLE page_field_defs RENAME COLUMN min_wert TO range_min;
ALTER TABLE page_field_defs RENAME COLUMN max_wert TO range_max;

ALTER TABLE block_types RENAME COLUMN kennung TO key;
ALTER TABLE block_types RENAME COLUMN hinweis TO hint;

-- content_types. Three of these four are not in the phase's criterion list at
-- all: it names twelve columns, and the schema carries twenty-four. Counted
-- rather than trusted, on 2026-09-11, by walking every column of every table.
ALTER TABLE content_types RENAME COLUMN kennung TO key;
ALTER TABLE content_types RENAME COLUMN mehrzahl TO plural;
ALTER TABLE content_types RENAME COLUMN archiv TO archive;
ALTER TABLE content_types RENAME COLUMN sortierung TO sort_order;

ALTER TABLE csv_imports RENAME COLUMN modus TO mode;
ALTER TABLE csv_imports RENAME COLUMN kollision TO collision;
ALTER TABLE csv_imports RENAME COLUMN dateiname TO filename;
ALTER TABLE csv_imports RENAME COLUMN daten TO data;
ALTER TABLE csv_imports RENAME COLUMN erstellt_am TO created_at;

-- pages.art is the one column in this file that does NOT become `kind`, and the
-- reason is worth the four lines it costs.
--
-- `pages` has carried an English `kind` since 00014 and it means something
-- else: 'page' or 'post'. `art`, added in 00036, holds the key of the website's
-- own content kind — 'produkt', 'termin' — and is a reference by key into
-- content_types. Two different questions, and the obvious translation of the
-- second collides head-on with the first.
--
-- So it is `content_kind`, which says which of the two it is without a lookup.
-- The glossary's rule (one German term, exactly one English word) is bent here
-- knowingly and is recorded there as bent, because the alternative — renaming
-- the older, already-correct `kind` to free the name — churns more code to make
-- a worse schema.
ALTER TABLE pages RENAME COLUMN art TO content_kind;

-- The index NAMES are German too, and SQLite has no ALTER INDEX RENAME, so
-- these six are dropped and rebuilt. RENAME COLUMN above has already carried
-- the new column names into their definitions; what is left is the name a
-- reader sees in an EXPLAIN QUERY PLAN.
--
-- Each definition below is the one that was in the database at this point,
-- re-typed with the English names. In particular the snippet index keeps
-- 00048's narrower form (snippet_id IS NOT NULL AND parent_id IS NULL) and not
-- 00047's — copying the wrong one of those two is exactly the mistake 00048
-- wrote a paragraph about.
DROP INDEX IF EXISTS idx_page_field_defs_kennung_gruppe;
CREATE UNIQUE INDEX idx_page_field_defs_key_group
    ON page_field_defs(parent_id, key) WHERE parent_id IS NOT NULL;

DROP INDEX IF EXISTS idx_page_field_defs_kennung_baustein;
CREATE UNIQUE INDEX idx_page_field_defs_key_block_type
    ON page_field_defs(block_type_id, key)
    WHERE block_type_id IS NOT NULL;

DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;
CREATE UNIQUE INDEX idx_page_field_defs_key_top
    ON page_field_defs(website_id, key)
    WHERE parent_id IS NULL AND block_type_id IS NULL AND snippet_id IS NULL;

DROP INDEX IF EXISTS idx_page_field_defs_kennung_textbaustein;
CREATE UNIQUE INDEX idx_page_field_defs_key_snippet
    ON page_field_defs(snippet_id, key)
    WHERE snippet_id IS NOT NULL AND parent_id IS NULL;

DROP INDEX IF EXISTS idx_pages_art;
CREATE INDEX idx_pages_content_kind ON pages(website_id, content_kind, published_at DESC)
    WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS idx_csv_imports_alter;
CREATE INDEX idx_csv_imports_created_at ON csv_imports(created_at);

-- +goose Down

-- Written with the care 00048 documents, which here means one thing above all:
-- the down half must put back what was there BEFORE this file ran, not a
-- tidied-up version of it. Every name below is the German one, and the four
-- indexes are rebuilt under their German names with their German columns.
--
-- This rollback cannot lose data. A column rename is reversible by definition —
-- no value is read, written or compared, so there is no case where the trip
-- back finds rows it cannot fit. That is unusually comfortable and it is why
-- this file is a rename and not a rebuild.
--
-- The order is the mirror of the up half: indexes first, because they are
-- rebuilt from the column names, then the columns.
DROP INDEX IF EXISTS idx_page_field_defs_key_group;
DROP INDEX IF EXISTS idx_page_field_defs_key_block_type;
DROP INDEX IF EXISTS idx_page_field_defs_key_top;
DROP INDEX IF EXISTS idx_page_field_defs_key_snippet;
DROP INDEX IF EXISTS idx_pages_content_kind;
DROP INDEX IF EXISTS idx_csv_imports_created_at;

ALTER TABLE pages RENAME COLUMN content_kind TO art;

ALTER TABLE csv_imports RENAME COLUMN created_at TO erstellt_am;
ALTER TABLE csv_imports RENAME COLUMN data TO daten;
ALTER TABLE csv_imports RENAME COLUMN filename TO dateiname;
ALTER TABLE csv_imports RENAME COLUMN collision TO kollision;
ALTER TABLE csv_imports RENAME COLUMN mode TO modus;

ALTER TABLE content_types RENAME COLUMN sort_order TO sortierung;
ALTER TABLE content_types RENAME COLUMN archive TO archiv;
ALTER TABLE content_types RENAME COLUMN plural TO mehrzahl;
ALTER TABLE content_types RENAME COLUMN key TO kennung;

ALTER TABLE block_types RENAME COLUMN hint TO hinweis;
ALTER TABLE block_types RENAME COLUMN key TO kennung;

ALTER TABLE page_field_defs RENAME COLUMN range_max TO max_wert;
ALTER TABLE page_field_defs RENAME COLUMN range_min TO min_wert;
ALTER TABLE page_field_defs RENAME COLUMN max_values TO max_werte;
ALTER TABLE page_field_defs RENAME COLUMN display TO darstellung;
ALTER TABLE page_field_defs RENAME COLUMN condition TO bedingung;
ALTER TABLE page_field_defs RENAME COLUMN applies_to TO gilt_fuer;
ALTER TABLE page_field_defs RENAME COLUMN choices TO auswahl;
ALTER TABLE page_field_defs RENAME COLUMN hint TO hinweis;
ALTER TABLE page_field_defs RENAME COLUMN required TO pflicht;
ALTER TABLE page_field_defs RENAME COLUMN kind TO art;
ALTER TABLE page_field_defs RENAME COLUMN label TO beschriftung;
ALTER TABLE page_field_defs RENAME COLUMN key TO kennung;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_gruppe
    ON page_field_defs(parent_id, kennung) WHERE parent_id IS NOT NULL;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_baustein
    ON page_field_defs(block_type_id, kennung)
    WHERE block_type_id IS NOT NULL;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung)
    WHERE parent_id IS NULL AND block_type_id IS NULL AND snippet_id IS NULL;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_textbaustein
    ON page_field_defs(snippet_id, kennung)
    WHERE snippet_id IS NOT NULL AND parent_id IS NULL;

CREATE INDEX idx_pages_art ON pages(website_id, art, published_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_csv_imports_alter ON csv_imports(erstellt_am);
