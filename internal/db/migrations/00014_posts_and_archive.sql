-- +goose Up

-- A second kind of content: the dated entry.
--
-- A carpenter's site has an "Über uns" that changes twice a decade and a
-- "Neues aus der Werkstatt" that gets an entry a month. Both are pages today,
-- which means the second kind has no archive, no ordering by date and no way to
-- keep it out of the main navigation.
--
-- One column on the existing table rather than a second table: a post has the
-- same fields, the same revisions, the same search index and the same
-- scheduling. Splitting them would duplicate all of that to express one
-- difference in how they are listed.
ALTER TABLE pages ADD COLUMN kind TEXT NOT NULL DEFAULT 'page'
    CHECK (kind IN ('page', 'post'));

-- Listing an archive means "the posts of this site, newest first".
CREATE INDEX idx_pages_kind_published
    ON pages(website_id, kind, published_at DESC)
    WHERE deleted_at IS NULL;

-- Where the archive lives. A German site wants /aktuelles, an English one
-- /news, and a site with no posts wants neither — an empty value switches the
-- archive route off entirely.
ALTER TABLE websites ADD COLUMN blog_base TEXT NOT NULL DEFAULT 'aktuelles';

-- How many entries one archive page shows before it pages.
ALTER TABLE websites ADD COLUMN posts_per_page INTEGER NOT NULL DEFAULT 10;

-- +goose Down
ALTER TABLE websites DROP COLUMN posts_per_page;
ALTER TABLE websites DROP COLUMN blog_base;
DROP INDEX IF EXISTS idx_pages_kind_published;
ALTER TABLE pages DROP COLUMN kind;
