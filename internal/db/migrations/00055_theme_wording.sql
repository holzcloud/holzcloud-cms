-- +goose Up

-- The operator's own wording for a theme's words.
--
-- PUB-03 of milestone v2.1. Since PUB-02 a theme carries lang/<tag>.json and
-- its words come out of it. That is right for the theme author and not enough
-- for the operator: the author wrote "Warenkorb", the farm shop calls it
-- "Korb", and until now the only ways to change one word were to fork the
-- theme or to edit a file on the server. Both mean the next theme update either
-- overwrites the change or is never applied.
--
-- One row per website, language and key. The key is the theme author's own
-- sentence — the same string that stands in lang/<tag>.json and in the
-- template — because anything else would need a second name for every word and
-- a way to keep the two in step.
--
-- No foreign key to a theme. The wording belongs to the WEBSITE, not to the
-- theme it happens to use today: an operator who tries a second theme and comes
-- back must find their words still there. The cost is that switching to a theme
-- with different words leaves rows nobody reads, which the screen says out loud
-- rather than tidying away behind their back.
CREATE TABLE theme_wording (
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    -- The language tag exactly as the website is published in it: "de", "fr",
    -- "de-CH". Not normalised to the base language, because a site published in
    -- de-CH may well want one word different from the de it falls back to.
    locale     TEXT    NOT NULL,
    key        TEXT    NOT NULL,
    value      TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,
    PRIMARY KEY (website_id, locale, key)
) STRICT;

-- The render asks for every word of one website in one language at once, which
-- is exactly the prefix of the primary key; SQLite serves that from the key
-- itself. The index below is for the other direction — the screen that lists
-- what a website has overridden across all its languages — and for the delete
-- that follows a website.
CREATE INDEX idx_theme_wording_website ON theme_wording(website_id);

-- +goose Down

-- Down drops the operator's own words, and there is no way for it not to:
-- nothing else in the schema can hold them. Anybody rolling back past this
-- point loses every overridden word and gets the theme's own back — the site
-- still renders, in the theme author's wording. That is the same shape 00048
-- documents: a Down that cannot be lossless says so.
DROP INDEX IF EXISTS idx_theme_wording_website;
DROP TABLE IF EXISTS theme_wording;
