-- +goose Up

-- Optimistic locking. updated_at deliberately is not used as the token:
-- strftime('%Y-%m-%dT%H:%M:%SZ') is second-resolution, so two saves within the
-- same second would carry the same value and the conflict would go unnoticed.
ALTER TABLE pages ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

-- Authorship. ON DELETE SET NULL so removing a user does not remove their work.
ALTER TABLE pages ADD COLUMN created_by INTEGER REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE pages ADD COLUMN updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL;

-- Soft delete. A page in the trash keeps its row; the slug is rewritten on the
-- way in so it stops colliding with the inline UNIQUE(website_id, slug) that
-- cannot be dropped without a full table rebuild.
ALTER TABLE pages ADD COLUMN deleted_at TEXT;
ALTER TABLE media ADD COLUMN deleted_at TEXT;

CREATE INDEX idx_pages_deleted ON pages(website_id, deleted_at);
CREATE INDEX idx_media_deleted ON media(website_id, deleted_at);

-- Revisions store Markdown only. content_html is derivable and storing it would
-- roughly double the table on an SD card.
CREATE TABLE page_revisions (
    id               INTEGER PRIMARY KEY,
    page_id          INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    user_id          INTEGER REFERENCES users(id) ON DELETE SET NULL,
    title            TEXT NOT NULL,
    slug             TEXT NOT NULL,
    content_markdown TEXT NOT NULL,
    status           TEXT NOT NULL,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE INDEX idx_page_revisions_page ON page_revisions(page_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_page_revisions_page;
DROP TABLE IF EXISTS page_revisions;
DROP INDEX IF EXISTS idx_media_deleted;
DROP INDEX IF EXISTS idx_pages_deleted;
ALTER TABLE media DROP COLUMN deleted_at;
ALTER TABLE pages DROP COLUMN deleted_at;
ALTER TABLE pages DROP COLUMN updated_by;
ALTER TABLE pages DROP COLUMN created_by;
ALTER TABLE pages DROP COLUMN version;
