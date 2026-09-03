-- +goose Up

-- Free-form labels across content.
--
-- A menu says where something sits; a label says what it is about. The two are
-- not the same thing, and using the menu for both is what produces navigations
-- with forty entries.
--
-- One flat kind of term rather than the categories-and-tags pair most systems
-- ship: nobody running a workshop's website has ever been able to say where the
-- line between the two runs, and the second kind exists mostly to be explained.
CREATE TABLE terms (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    -- slug is the address under /tag/; name is what an editor typed, including
    -- its capitals and umlauts.
    slug       TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (website_id, slug)
) STRICT;

-- Which content carries which label.
--
-- WITHOUT ROWID: the whole row is its key, so the extra rowid would be pure
-- overhead on a table that is read on every archive page.
CREATE TABLE page_terms (
    page_id INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    term_id INTEGER NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    PRIMARY KEY (page_id, term_id)
) STRICT, WITHOUT ROWID;

-- The primary key already serves "which labels does this page have"; this index
-- serves the other direction, which is what /tag/<slug> asks.
CREATE INDEX idx_page_terms_term ON page_terms(term_id);

-- +goose Down
DROP INDEX IF EXISTS idx_page_terms_term;
DROP TABLE IF EXISTS page_terms;
DROP TABLE IF EXISTS terms;
