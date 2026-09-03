-- +goose Up

-- Scheduled publishing. Deliberately NOT a new status value: the status CHECK
-- is inline on a STRICT table and changing it would need the twelve-step table
-- rebuild, on the only copy of the customer's data.
--
-- Instead the schedule is a predicate evaluated at read time. A Pi that was
-- switched off over the publication moment comes up already correct, and there
-- is no tick to miss and no clock drift to reconcile.
ALTER TABLE pages ADD COLUMN publish_at TEXT;
ALTER TABLE pages ADD COLUMN unpublish_at TEXT;
ALTER TABLE pages ADD COLUMN review_state TEXT NOT NULL DEFAULT 'none'
    CHECK (review_state IN ('none', 'pending'));

CREATE INDEX idx_pages_publish_at ON pages(website_id, status, publish_at);

-- Redirects, so renaming a page does not break every link pointing at it. The
-- inline title editor rewrites the slug from the title, which means a live URL
-- can change by accident.
CREATE TABLE redirects (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    from_path  TEXT    NOT NULL,
    to_path    TEXT    NOT NULL,
    code       INTEGER NOT NULL DEFAULT 301 CHECK (code IN (301, 302)),
    source     TEXT    NOT NULL DEFAULT 'auto' CHECK (source IN ('auto', 'manual')),
    hits       INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (website_id, from_path)
) STRICT;

-- Reusable text blocks. The address and the opening hours are the content that
-- changes most often and had to be copied into every page.
CREATE TABLE snippets (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id       INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    key              TEXT    NOT NULL,
    name             TEXT    NOT NULL,
    content_markdown TEXT    NOT NULL DEFAULT '',
    content_html     TEXT    NOT NULL DEFAULT '',
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (website_id, key)
) STRICT;

-- Full-text search. FTS5 is compiled into the pinned pure-Go driver, so this
-- costs no new dependency and no CGO.
--
-- A virtual table cannot be STRICT — an SQLite limitation, not a departure from
-- the convention. content_markdown is indexed rather than content_html so no
-- tags end up in the index, and remove_diacritics 2 is what makes MATCH 'mobel'
-- find "Möbel".
-- +goose StatementBegin
CREATE VIRTUAL TABLE pages_fts USING fts5(
    title,
    content_markdown,
    content='pages',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
-- +goose StatementEnd

-- External-content tables are not kept in sync automatically; the delete form
-- with VALUES('delete', ...) is what FTS5 requires to remove the old terms.
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

-- Backfill for rows that existed before the index did.
INSERT INTO pages_fts(rowid, title, content_markdown)
SELECT id, title, content_markdown FROM pages;

-- +goose Down
DROP TRIGGER IF EXISTS pages_fts_update;
DROP TRIGGER IF EXISTS pages_fts_delete;
DROP TRIGGER IF EXISTS pages_fts_insert;
DROP TABLE IF EXISTS pages_fts;
DROP TABLE IF EXISTS snippets;
DROP TABLE IF EXISTS redirects;
DROP INDEX IF EXISTS idx_pages_publish_at;
ALTER TABLE pages DROP COLUMN review_state;
ALTER TABLE pages DROP COLUMN unpublish_at;
ALTER TABLE pages DROP COLUMN publish_at;
