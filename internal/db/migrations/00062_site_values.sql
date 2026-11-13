-- +goose Up

-- The values of a website: a named line of text that a page writes as
-- [[wert:key]] inside a sentence.
--
-- VAL-01 of milestone 0.1.5. The case that asked for it is a version number:
-- holzcloud.ch names the released version of each project in a dozen sentences
-- and link addresses, and every release rewrote those pages one by one. A value
-- is kept once, and every sentence that names it reads it.
--
-- Not called "values": VALUES is SQL, and a table named after a keyword has to
-- be quoted in every statement that ever touches it.
--
-- One text per website, for every language (D-04 of phase 33). Words that
-- differ by language remain the page's own text or the wording catalogue's; a
-- per-language text later means a column here and a resolver that knows the
-- page's language, and that cost was accepted.
--
-- The text is resolved when a page is delivered, never frozen into
-- pages.content_html at save (VAL-04). Changing a value therefore changes every
-- page that names it without touching any of them, which is the whole point —
-- and it is why the public side folds updated_at into its caching validator.
--
-- The unique pair is also the index the per-request read uses: a page that
-- names a value asks for every value of one website, which is that key's
-- prefix.
CREATE TABLE site_values (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id  INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    key         TEXT    NOT NULL,
    value       TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (website_id, key)
) STRICT;

-- +goose Down

-- Down drops every value, and there is nowhere else for them to go. A page
-- that names one keeps its marker in the stored text; a binary from before this
-- migration does not know the marker and prints it as it stands.
DROP TABLE IF EXISTS site_values;
