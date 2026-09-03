-- +goose Up

-- What the site is, as a machine reads it.
--
-- A joiner's website lives or dies on being findable, and the difference
-- between a search result with an address, opening hours and a telephone number
-- and one with a blue link is not a small one. All of it is data the operator
-- already puts in the imprint; this only says it in a form a search engine
-- understands rather than guesses at.
--
-- org_type is a schema.org type. Left empty, no business block is emitted at
-- all — a personal blog should not claim to be a shop.
ALTER TABLE websites ADD COLUMN org_type TEXT NOT NULL DEFAULT '';

ALTER TABLE websites ADD COLUMN street TEXT NOT NULL DEFAULT '';
ALTER TABLE websites ADD COLUMN postal_code TEXT NOT NULL DEFAULT '';
ALTER TABLE websites ADD COLUMN city TEXT NOT NULL DEFAULT '';
ALTER TABLE websites ADD COLUMN country TEXT NOT NULL DEFAULT 'DE';
ALTER TABLE websites ADD COLUMN phone TEXT NOT NULL DEFAULT '';

-- Opening hours in the schema.org shorthand, one rule per line:
--   Mo-Fr 08:00-17:00
--   Sa 09:00-12:00
--
-- Free text rather than a table of days: a workshop's hours are two or three
-- lines, and a table would need its own screen to edit what fits in a textarea.
ALTER TABLE websites ADD COLUMN opening_hours TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE websites DROP COLUMN opening_hours;
ALTER TABLE websites DROP COLUMN phone;
ALTER TABLE websites DROP COLUMN country;
ALTER TABLE websites DROP COLUMN city;
ALTER TABLE websites DROP COLUMN postal_code;
ALTER TABLE websites DROP COLUMN street;
ALTER TABLE websites DROP COLUMN org_type;
