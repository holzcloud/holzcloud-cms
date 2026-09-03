-- +goose Up

-- Alt text and caption. There was nowhere at all to describe an image, so the
-- public site could not emit an alt attribute even in principle — an
-- accessibility gap the operator had no way to close.
ALTER TABLE media ADD COLUMN alt_text TEXT NOT NULL DEFAULT '';
ALTER TABLE media ADD COLUMN caption TEXT NOT NULL DEFAULT '';

-- Content hash, so the same file uploaded twice is recognised instead of
-- silently occupying the card twice.
ALTER TABLE media ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';

-- (website_id, filename) is exactly the predicate every public image request
-- queries by, and nothing enforced it. One line, and the hot lookup becomes an
-- index probe.
CREATE UNIQUE INDEX idx_media_website_filename ON media(website_id, filename);
CREATE INDEX idx_media_hash ON media(website_id, content_hash);

-- Which page uses which file. Deleting a file used to break every page
-- embedding it, with no warning and no query that could have found the damage
-- afterwards.
CREATE TABLE media_usage (
    page_id  INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    PRIMARY KEY (page_id, media_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX idx_media_usage_media ON media_usage(media_id);

-- +goose Down
DROP INDEX IF EXISTS idx_media_usage_media;
DROP TABLE IF EXISTS media_usage;
DROP INDEX IF EXISTS idx_media_hash;
DROP INDEX IF EXISTS idx_media_website_filename;
ALTER TABLE media DROP COLUMN content_hash;
ALTER TABLE media DROP COLUMN caption;
ALTER TABLE media DROP COLUMN alt_text;
