-- +goose Up

-- Scaled-down copies of an uploaded image.
--
-- A phone on a slow connection should not have to download a 12-megapixel photo
-- to show it 400 pixels wide. The sanitizer already lets srcset through; this is
-- what fills it.
--
-- Variants are rows rather than columns because the set of widths is a decision
-- that can change, and a fixed thumb/medium/large triple would need a migration
-- every time.
CREATE TABLE media_variants (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id   INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    -- label is "thumb", "medium", … and is what the admin grid asks for.
    label      TEXT    NOT NULL,
    filename   TEXT    NOT NULL,
    width      INTEGER NOT NULL,
    height     INTEGER NOT NULL,
    size_bytes INTEGER NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (media_id, label)
) STRICT;

CREATE INDEX idx_media_variants_media ON media_variants(media_id);

-- The original's pixel dimensions, so a theme can emit width/height and the
-- browser can reserve the space before the image arrives.
ALTER TABLE media ADD COLUMN width INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media ADD COLUMN height INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE media DROP COLUMN height;
ALTER TABLE media DROP COLUMN width;
DROP INDEX IF EXISTS idx_media_variants_media;
DROP TABLE IF EXISTS media_variants;
