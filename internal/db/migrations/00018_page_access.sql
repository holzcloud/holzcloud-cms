-- +goose Up

-- Who may see a page.
--
-- 'public' is everything as it was. 'password' puts a form in front of the page
-- for the price list a joiner sends to trade customers and does not want in a
-- search index.
--
-- Not a substitute for real accounts: everyone who gets the word gets in, and
-- there is no way to tell afterwards who did. It is the right size for the
-- problem it solves and no bigger.
ALTER TABLE pages ADD COLUMN access TEXT NOT NULL DEFAULT 'public'
    CHECK (access IN ('public', 'password'));

-- The Argon2id hash of the page's password, in the same PHC format as an
-- account password. Storing it in the clear would mean one careless database
-- copy exposes every protected page at once.
ALTER TABLE pages ADD COLUMN access_password TEXT NOT NULL DEFAULT '';

-- A line shown above the form — "the password is in our letter" — so a visitor
-- who has it knows they are in the right place and one who does not knows whom
-- to ask.
ALTER TABLE pages ADD COLUMN access_hint TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE pages DROP COLUMN access_hint;
ALTER TABLE pages DROP COLUMN access_password;
ALTER TABLE pages DROP COLUMN access;
