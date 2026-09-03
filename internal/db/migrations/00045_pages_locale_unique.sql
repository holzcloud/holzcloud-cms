-- +goose Up

-- Eine Adresse je Sprache.
--
-- 00030 hat den Seiten eine Sprache gegeben und einen Index über
-- (website_id, locale, slug) gelegt. Der Zwang aus 00004 blieb, wie er war:
-- UNIQUE(website_id, slug). Eine Adresse kann es damit pro Website nur einmal
-- geben, und die französische Fassung von /holzcloud-cms bekommt beim Anlegen
-- still ein "-2" angehängt.
--
-- Das ist falsch, und die Verwaltung sagt es selbst: sie schlägt für eine neue
-- Übersetzung Titel und Adresse des Originals vor, weil "die Adresse das eine
-- ist, was über die Sprachen hinweg wiedererkennbar bleiben soll"
-- (admin/page.go, sourcePageFor). Ein Produktname wird nicht übersetzt. Für
-- die Menüs wurde derselbe Fehler in 00031 behoben; für die Seiten blieb er
-- stehen.
--
-- Der Zwang steht am Tabellenkopf und lässt sich in SQLite nicht ändern, die
-- Tabelle muss neu gebaut werden. Und weil DROP TABLE bei eingeschalteten
-- Fremdschlüsseln ein stilles DELETE FROM ausführt, muss alles mitgebaut
-- werden, was an pages hängt: page_revisions und media_usage und page_terms
-- mit CASCADE — die Fassungsgeschichte, die Bildzuordnung und die
-- Schlagwörter wären sonst weg —, dazu menu_items und form_messages, deren
-- page_id auf NULL gesetzt würde.
--
-- Der kürzere Weg, den SQLite dafür beschreibt — foreign_keys ausschalten und
-- die Tabelle umbenennen, damit die Verweise stehen bleiben —, trägt hier
-- nicht: modernc.org/sqlite schreibt die Fremdschlüsselklauseln beim
-- Umbenennen um, auch wenn foreign_keys nachweislich auf 0 steht. Gemessen am
-- 2026-09-03 gegen v1.34, mit foreign_keys(0) schon in der DSN. Genau darauf
-- baut dieser Weg stattdessen: die Verweise der _neu-Tabellen zeigen nach dem
-- letzten Umbenennen von selbst wieder auf die richtigen Namen.
--
-- Die neue Bedingung ist schwächer als die alte: was unter
-- UNIQUE(website_id, slug) erlaubt war, ist es unter
-- UNIQUE(website_id, locale, slug) erst recht. Bestehende Daten können sie
-- also nicht verletzen.

-- +goose StatementBegin
CREATE TABLE pages_neu (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id       INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    title            TEXT    NOT NULL,
    slug             TEXT    NOT NULL,
    content_markdown TEXT    NOT NULL DEFAULT '',
    content_html     TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_at     TEXT,
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    version          INTEGER NOT NULL DEFAULT 1,
    created_by       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    updated_by       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    deleted_at       TEXT,
    excerpt          TEXT    NOT NULL DEFAULT '',
    meta_description TEXT    NOT NULL DEFAULT '',
    featured_media_id INTEGER REFERENCES media(id) ON DELETE SET NULL,
    noindex          INTEGER NOT NULL DEFAULT 0 CHECK (noindex IN (0, 1)),
    publish_at       TEXT,
    unpublish_at     TEXT,
    review_state     TEXT    NOT NULL DEFAULT 'none' CHECK (review_state IN ('none', 'pending')),
    kind             TEXT    NOT NULL DEFAULT 'page' CHECK (kind IN ('page', 'post')),
    access           TEXT    NOT NULL DEFAULT 'public' CHECK (access IN ('public', 'password')),
    access_password  TEXT    NOT NULL DEFAULT '',
    access_hint      TEXT    NOT NULL DEFAULT '',
    blocks           TEXT    NOT NULL DEFAULT '',
    fields           TEXT    NOT NULL DEFAULT '',
    locale           TEXT    NOT NULL DEFAULT '',
    translation_of   INTEGER REFERENCES pages_neu(id) ON DELETE SET NULL,
    art              TEXT    NOT NULL DEFAULT '',
    UNIQUE(website_id, locale, slug)
) STRICT;
-- +goose StatementEnd

INSERT INTO pages_neu (
    id, website_id, title, slug, content_markdown, content_html, status, published_at,
    created_at, updated_at, version, created_by, updated_by, deleted_at, excerpt,
    meta_description, featured_media_id, noindex, publish_at, unpublish_at, review_state,
    kind, access, access_password, access_hint, blocks, fields, locale, translation_of, art)
SELECT
    id, website_id, title, slug, content_markdown, content_html, status, published_at,
    created_at, updated_at, version, created_by, updated_by, deleted_at, excerpt,
    meta_description, featured_media_id, noindex, publish_at, unpublish_at, review_state,
    kind, access, access_password, access_hint, blocks, fields, locale, translation_of, art
FROM pages;

-- +goose StatementBegin
CREATE TABLE page_revisions_neu (
    id               INTEGER PRIMARY KEY,
    page_id          INTEGER NOT NULL REFERENCES pages_neu(id) ON DELETE CASCADE,
    user_id          INTEGER REFERENCES users(id) ON DELETE SET NULL,
    title            TEXT NOT NULL,
    slug             TEXT NOT NULL,
    content_markdown TEXT NOT NULL,
    status           TEXT NOT NULL,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    blocks           TEXT NOT NULL DEFAULT '',
    label            TEXT NOT NULL DEFAULT ''
) STRICT;
-- +goose StatementEnd

INSERT INTO page_revisions_neu
    (id, page_id, user_id, title, slug, content_markdown, status, created_at, blocks, label)
SELECT id, page_id, user_id, title, slug, content_markdown, status, created_at, blocks, label
FROM page_revisions;

-- +goose StatementBegin
CREATE TABLE media_usage_neu (
    page_id  INTEGER NOT NULL REFERENCES pages_neu(id) ON DELETE CASCADE,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    PRIMARY KEY (page_id, media_id)
) STRICT, WITHOUT ROWID;
-- +goose StatementEnd

INSERT INTO media_usage_neu (page_id, media_id) SELECT page_id, media_id FROM media_usage;

-- +goose StatementBegin
CREATE TABLE page_terms_neu (
    page_id INTEGER NOT NULL REFERENCES pages_neu(id) ON DELETE CASCADE,
    term_id INTEGER NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    PRIMARY KEY (page_id, term_id)
) STRICT, WITHOUT ROWID;
-- +goose StatementEnd

INSERT INTO page_terms_neu (page_id, term_id) SELECT page_id, term_id FROM page_terms;

-- +goose StatementBegin
CREATE TABLE form_messages_neu (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    -- page_id says which page the form stood on. SET NULL rather than CASCADE:
    -- deleting a page must not delete the enquiries that came through it.
    page_id    INTEGER REFERENCES pages_neu(id) ON DELETE SET NULL,
    name       TEXT    NOT NULL,
    email      TEXT    NOT NULL,
    subject    TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    -- read_at is NULL until someone opens it, which is what the unread badge
    -- counts.
    read_at    TEXT
) STRICT;
-- +goose StatementEnd

INSERT INTO form_messages_neu
    (id, website_id, page_id, name, email, subject, body, created_at, read_at)
SELECT id, website_id, page_id, name, email, subject, body, created_at, read_at
FROM form_messages;

-- +goose StatementBegin
CREATE TABLE menu_items_neu (
    id         INTEGER PRIMARY KEY,
    menu_id    INTEGER NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
    parent_id  INTEGER REFERENCES menu_items_neu(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    item_type  TEXT NOT NULL DEFAULT 'url',
    url        TEXT NOT NULL DEFAULT '',
    page_id    INTEGER REFERENCES pages_neu(id) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;
-- +goose StatementEnd

INSERT INTO menu_items_neu
    (id, menu_id, parent_id, title, item_type, url, page_id, sort_order, created_at)
SELECT id, menu_id, parent_id, title, item_type, url, page_id, sort_order, created_at
FROM menu_items;

-- Erst die Kinder, dann die Eltern: an pages hängt danach nichts mehr, also
-- löscht DROP TABLE auch nichts mehr mit.
DROP INDEX IF EXISTS idx_page_revisions_page;
DROP INDEX IF EXISTS idx_media_usage_media;
DROP INDEX IF EXISTS idx_page_terms_term;
DROP INDEX IF EXISTS idx_form_messages_website;
DROP INDEX IF EXISTS idx_menu_items_menu;
DROP TABLE page_revisions;
DROP TABLE media_usage;
DROP TABLE page_terms;
DROP TABLE form_messages;
DROP TABLE menu_items;

DROP INDEX IF EXISTS idx_pages_website_status;
DROP INDEX IF EXISTS idx_pages_website_slug;
DROP INDEX IF EXISTS idx_pages_deleted;
DROP INDEX IF EXISTS idx_pages_publish_at;
DROP INDEX IF EXISTS idx_pages_kind_published;
DROP INDEX IF EXISTS idx_pages_locale;
DROP INDEX IF EXISTS idx_pages_translation;
DROP INDEX IF EXISTS idx_pages_art;
DROP TABLE pages;

-- Das Umbenennen zieht die Fremdschlüssel mit: die fünf Kinder zeigen danach
-- auf pages und nicht mehr auf pages_neu.
ALTER TABLE pages_neu RENAME TO pages;
ALTER TABLE page_revisions_neu RENAME TO page_revisions;
ALTER TABLE media_usage_neu RENAME TO media_usage;
ALTER TABLE page_terms_neu RENAME TO page_terms;
ALTER TABLE form_messages_neu RENAME TO form_messages;
ALTER TABLE menu_items_neu RENAME TO menu_items;

CREATE INDEX idx_pages_website_status ON pages(website_id, status);
CREATE INDEX idx_pages_website_slug ON pages(website_id, slug);
CREATE INDEX idx_pages_deleted ON pages(website_id, deleted_at);
CREATE INDEX idx_pages_publish_at ON pages(website_id, status, publish_at);
CREATE INDEX idx_pages_kind_published
    ON pages(website_id, kind, published_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_pages_locale ON pages(website_id, locale, slug);
CREATE INDEX idx_pages_translation ON pages(translation_of);
CREATE INDEX idx_pages_art ON pages(website_id, art, published_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_page_revisions_page ON page_revisions(page_id, created_at DESC);
CREATE INDEX idx_media_usage_media ON media_usage(media_id);
CREATE INDEX idx_page_terms_term ON page_terms(term_id);
CREATE INDEX idx_form_messages_website ON form_messages(website_id, created_at DESC);
CREATE INDEX idx_menu_items_menu ON menu_items(menu_id, sort_order);

-- Die drei Volltext-Trigger hingen an der alten Tabelle und sind mit ihr
-- gegangen. Der Index pages_fts selbst wird nicht angefasst: die ids bleiben
-- dieselben, also stimmt er weiter — ein DROP TABLE löst keine Trigger aus,
-- sonst hätte es ihn geleert.

-- +goose StatementBegin
CREATE TRIGGER pages_fts_insert AFTER INSERT ON pages BEGIN
    INSERT INTO pages_fts(rowid, title, content_markdown)
    VALUES (new.id, new.title, new.content_markdown);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER pages_fts_delete AFTER DELETE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, content_markdown)
    VALUES ('delete', old.id, old.title, old.content_markdown);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER pages_fts_update AFTER UPDATE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, content_markdown)
    VALUES ('delete', old.id, old.title, old.content_markdown);
    INSERT INTO pages_fts(rowid, title, content_markdown)
    VALUES (new.id, new.title, new.content_markdown);
END;
-- +goose StatementEnd

-- +goose Down
-- Der Zwang wird nicht zurückgebaut, aus demselben Grund wie in 00031: unter
-- der alten Bedingung dürfte es Seiten nicht geben, die es jetzt gibt, und
-- eine Rückwärtsmigration, die Inhalt löscht, ist keine.
SELECT 1;
