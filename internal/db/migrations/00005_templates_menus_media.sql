-- +goose Up

CREATE TABLE templates (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE TABLE website_templates (
    id          INTEGER PRIMARY KEY,
    website_id  INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    template_id INTEGER NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
    is_active   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(website_id, template_id)
) STRICT;

CREATE UNIQUE INDEX idx_website_active_template
    ON website_templates(website_id) WHERE is_active = 1;

CREATE TABLE menus (
    id           INTEGER PRIMARY KEY,
    website_id   INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    location_key TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(website_id, location_key)
) STRICT;

CREATE TABLE menu_items (
    id         INTEGER PRIMARY KEY,
    menu_id    INTEGER NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
    parent_id  INTEGER REFERENCES menu_items(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    item_type  TEXT NOT NULL DEFAULT 'url',
    url        TEXT NOT NULL DEFAULT '',
    page_id    INTEGER REFERENCES pages(id) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE TABLE media (
    id            INTEGER PRIMARY KEY,
    website_id    INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL,
    mime_type     TEXT NOT NULL,
    size_bytes    INTEGER NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE INDEX idx_media_website ON media(website_id);

-- +goose Down
DROP TABLE IF EXISTS media;
DROP TABLE IF EXISTS menu_items;
DROP TABLE IF EXISTS menus;
DROP TABLE IF EXISTS website_templates;
DROP TABLE IF EXISTS templates;
