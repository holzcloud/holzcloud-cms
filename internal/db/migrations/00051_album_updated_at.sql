-- +goose Up

-- When an album last changed, because a page that carries one is cached
-- against that answer.
--
-- 00050 gave albums and album_items a created_at and nothing else, and that was
-- enough for as long as nobody asked when the album had last moved. Something
-- does ask, on every public request: contentModTime
-- (internal/public/pagedata.go) is the Last-Modified a page is served with, and
-- its own doc comment states the rule 00050 broke —
--
--   "It has to account for the snippets, not just the page: editing the opening
--   hours changes what the page renders without touching pages.updated_at, and
--   a browser holding an If-Modified-Since would keep the old text
--   indefinitely."
--
-- An album is the second such source. Adding a picture to it changes every page
-- that names it and touches none of them: pages.content_html carries the marker
-- and nothing about the pictures, which is the whole of GAL-03. Without a
-- validator that moves with the album, a browser that has been on the site
-- before answers its own request out of its cache and keeps the old gallery.
--
-- Why the default is the empty string and not a timestamp: SQLite refuses a
-- parenthesised expression as the DEFAULT of ADD COLUMN, so the form 00050 uses
-- on created_at is not available here. TEXT NOT NULL DEFAULT '' is the shape
-- 00047 already had to take on snippets.fields for the same reason, and every
-- existing row is filled from created_at immediately below. The consequence is
-- that internal/album/store.go must NAME this column in its INSERTs rather than
-- leave it to the table; it does, and Store's own comment says why.
--
-- Nothing in 00050 is edited. That file is released.
ALTER TABLE albums ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';

-- Every album that exists was last changed when it was made — which is what its
-- editor would say too, and it is never later than the truth, so a cache built
-- on it can only be too eager to revalidate and never too willing to keep.
UPDATE albums SET updated_at = created_at;

-- The child carries its own stamp as well, and the distinction between the two
-- is worth writing down because only one of them is the validator.
--
-- albums.updated_at is what a public page is cached against, and it is moved by
-- EVERY change to the album — its name, and the addition, the edit, the removal
-- and the reordering of any of its pictures (internal/album/store.go's
-- touchAlbum). It has to be the parent's, because a DELETED picture leaves no
-- row behind to carry a timestamp: a MAX over the children can only ever go up,
-- and removing the last picture from an album is exactly the change a visitor
-- must not be served the old answer for.
--
-- album_items.updated_at is provenance for one row — when this caption was last
-- corrected — and it is what makes the child table honest on its own terms
-- rather than only through its parent. It is deliberately NOT the thing the
-- validator reads.
ALTER TABLE album_items ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';

UPDATE album_items SET updated_at = created_at;

-- +goose Down

-- Two columns away, child before parent, the same order 00050's rollback gives
-- and for a weaker version of the same reason: nothing here depends on the
-- other, but a rollback that stopped halfway should stop having taken the
-- narrower thing rather than the wider one.
--
-- This migration adds and backfills; it changes no existing shape. So there is
-- no earlier state its rollback could restore too far, which is the hazard
-- 00049's tail names for the index swaps.
ALTER TABLE album_items DROP COLUMN updated_at;
ALTER TABLE albums DROP COLUMN updated_at;
