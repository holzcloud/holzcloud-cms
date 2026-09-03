-- +goose Up
CREATE TABLE pages (
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
    UNIQUE(website_id, slug)
) STRICT;

CREATE INDEX idx_pages_website_status ON pages(website_id, status);
CREATE INDEX idx_pages_website_slug ON pages(website_id, slug);

-- +goose Down
DROP TABLE IF EXISTS pages;
