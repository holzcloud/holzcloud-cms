-- +goose Up

-- Per-website design overrides.
--
-- The gap this fills: choosing between four themes is too coarse — everyone
-- wants their own colour — and uploading a template is too much, because it
-- means maintaining a copy of a theme through every future change. A handful of
-- values sits between the two.
--
-- A fixed set of columns rather than a free key/value table on purpose. Every
-- one of these is validated against a shape the CSS can only interpret one way,
-- so no value an operator types can become a declaration. A table of arbitrary
-- names would either need the same whitelist anyway or would be a way to write
-- CSS into a page through a form.
--
-- Empty means "leave the theme alone", which is why none of them has a default
-- colour: a token with a value would override a theme that had a better answer.
ALTER TABLE websites ADD COLUMN token_ink TEXT NOT NULL DEFAULT '';
ALTER TABLE websites ADD COLUMN token_paper TEXT NOT NULL DEFAULT '';
ALTER TABLE websites ADD COLUMN token_brand TEXT NOT NULL DEFAULT '';

-- One of a few font stacks, never a font name typed by hand: a name that is not
-- installed silently falls back to something else, and a name that is a URL
-- would be a way to load a font from another server.
ALTER TABLE websites ADD COLUMN token_font TEXT NOT NULL DEFAULT '';

-- The width of the text column, in characters. 0 means the theme decides.
ALTER TABLE websites ADD COLUMN token_measure INTEGER NOT NULL DEFAULT 0;

-- Corner rounding in pixels. -1 means the theme decides, so that 0 can mean
-- "square corners, deliberately".
ALTER TABLE websites ADD COLUMN token_radius INTEGER NOT NULL DEFAULT -1;

-- +goose Down
ALTER TABLE websites DROP COLUMN token_radius;
ALTER TABLE websites DROP COLUMN token_measure;
ALTER TABLE websites DROP COLUMN token_font;
ALTER TABLE websites DROP COLUMN token_brand;
ALTER TABLE websites DROP COLUMN token_paper;
ALTER TABLE websites DROP COLUMN token_ink;
