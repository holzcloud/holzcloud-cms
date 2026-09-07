-- +goose Up

-- An album: a named set of pictures that a page can carry by name.
--
-- The shape is menus (00005): a parent row that belongs to one website, and an
-- ordered child table beneath it. An album is exactly that with pictures
-- instead of links, and the reason to copy that shape rather than to invent one
-- is that everything downstream of it already exists — the cascade, the
-- website scoping, the swap of two sort orders that reorders a list without
-- rewriting it.
--
-- What an album buys over a gallery block: the block's picture list lives in
-- one page's blocks column and cannot be reused. An album is written once and
-- named from as many pages as want it, and correcting a caption corrects it
-- everywhere at once.
--
-- Three things this file does NOT have, each on purpose:
--
-- No location key. menus carries one because a menu is mounted somewhere in a
-- theme and the theme asks for it by that key. Nobody mounts an album; a page
-- names it.
--
-- No nesting column pointing at a row of the same table. An album is FLAT.
-- Everything in internal/menu/store.go:280-349 — buildTree, createsCycle,
-- materialize — exists only because menus nest, and one of those three exists
-- because a bug was found in it. An album that copied the column would inherit
-- the hazard without having any use for it.
--
-- No closed vocabulary. There is no kind column and no display column here, so
-- this migration nails down no stored word in any language.
CREATE TABLE albums (
    -- AUTOINCREMENT, and the two candidates in this tree disagree about it:
    -- menus (00005) has the bare form and predates the house style, while
    -- snippets (00010), terms (00015) and csv_imports (00049) all carry it.
    -- Where an older and a newer analog disagree, the newer one wins. What it
    -- buys is that a deleted album's id is never handed to the next one, so a
    -- stale link in somebody's browser history cannot land on a different
    -- album than the one it named.
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,

    -- slug is the stable key; name is what an editor typed, with its capitals
    -- and umlauts. The pair is the one terms (00015) already carries, and an
    -- album needs it for the same reason plus one of its own: a gallery block
    -- stores the slug when it names this album, and the bundle re-derives it on
    -- import from the name it carried across (internal/bundle/import.go:573-583
    -- does exactly that for a label).
    --
    -- Which is why a rename must move the name and NOT the slug. Correcting a
    -- typo in "Werkstatt 2024" would otherwise take the album away from every
    -- page that carries it, silently, at the moment of the correction.
    slug TEXT NOT NULL,
    name TEXT NOT NULL,

    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    -- Unique per website and not globally: two websites on one installation may
    -- both have an album called "Referenzen", and they are two albums.
    UNIQUE (website_id, slug)
) STRICT;

-- The pictures of one album, in order.
--
-- This table has no website_id of its own, exactly as menu_items has none: it
-- reaches the website through its parent. That is a decision with a cost, and
-- the cost is paid in internal/album/store.go, where every item statement
-- scopes itself through a subquery on albums instead of naming a column here.
-- The alternative — a second website_id, denormalised — is a column that can
-- disagree with its parent, and a row on which the two disagree is a row no
-- reader can classify.
CREATE TABLE album_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,

    -- The cascade here means something concrete: deleting a picture from the
    -- media library removes it from every album that used it, silently and
    -- correctly, in the same way it already vanishes from a page's block. An
    -- album row pointing at a file that no longer exists would render as a
    -- broken image on a public page, which is worse than one picture fewer.
    --
    -- Note what the foreign key does NOT prove: it proves the media row exists,
    -- not whose it is. A media id reaching this table comes out of a form, so
    -- the store checks the picture's own website_id before inserting, the way
    -- internal/admin/page_blocks.go:105-115 already does for a block picture.
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,

    -- These two, with media_id above, are the same three things block.Item
    -- carries (internal/block/block.go:220-227), and they are the same three on
    -- purpose. GAL-07 asks for one mechanism, and one mechanism means both
    -- lists feed the SAME []block.Item into the SAME gallery renderer. A fourth
    -- column here would be a second shape, and a second shape is the first line
    -- of a second renderer.
    alt TEXT NOT NULL DEFAULT '',
    caption TEXT NOT NULL DEFAULT '',

    -- Order is an integer that two rows swap, not a position that a rewrite of
    -- the whole list maintains. internal/menu/store.go:196-222 is the idiom.
    sort_order INTEGER NOT NULL DEFAULT 0,

    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

-- Every page that carries an album asks exactly this question on every request:
-- the pictures of one album, in order. Without the index that is a scan of
-- every picture row of every album on the installation, on the read path of a
-- public page — and it gets slower for everybody each time anybody anywhere
-- adds a photo.
CREATE INDEX idx_album_items_album_sort ON album_items(album_id, sort_order);

-- +goose Down

-- Three statements, and that is the complete rollback rather than the
-- abbreviated one.
--
-- 00049's tail draws the distinction this paragraph inherits: 00047 and 00048
-- are index swaps, so their rollback has to restore a previous shape and can
-- restore the wrong one without anything noticing. This migration creates and
-- changes nothing. There is therefore no earlier state it could restore too far
-- or too little — it takes away what it made, and everything it made.
--
-- The order matters and is the child before the parent. DROP TABLE albums
-- first would leave album_items standing with a foreign key naming a table that
-- is gone; SQLite permits that, and it is exactly the half-state a rollback
-- exists to avoid. The index goes first for the same reason it is named first
-- in 00049: it is the object a rollback that stopped at the tables would leave
-- behind.
DROP INDEX IF EXISTS idx_album_items_album_sort;
DROP TABLE IF EXISTS album_items;
DROP TABLE IF EXISTS albums;
