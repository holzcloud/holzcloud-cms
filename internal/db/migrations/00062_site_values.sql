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
--
-- The CHECKs are a second layer under value.Store (CheckKey, CleanText), which
-- every writer goes through — the screen, the assistant, the bundle import.
-- They hold what the delivery side relies on: a key of a-z, 0-9, - and _ that
-- neither starts nor ends with - or _, at most 60 bytes; a text of 1 to 400
-- bytes with no square bracket (a bracket could complete a marker with the
-- brackets beside it) and no line break or tab. The finer rules — valid UTF-8,
-- no other control or invisible format character — stay in Go, where they can
-- be said in a sentence. Written into this migration and not a later one
-- because it had not been released when they were added (IN-04 of the phase
-- 33 review), and SQLite cannot add a CHECK to a table without rebuilding it.
CREATE TABLE site_values (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id  INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    key         TEXT    NOT NULL
                CHECK (length(key) BETWEEN 1 AND 60
                       AND key GLOB '[a-z0-9]*'
                       AND key GLOB '*[a-z0-9]'
                       AND key NOT GLOB '*[^a-z0-9_-]*'),
    value       TEXT    NOT NULL
                CHECK (length(CAST(value AS BLOB)) BETWEEN 1 AND 400
                       AND instr(value, '[') = 0 AND instr(value, ']') = 0
                       AND instr(value, char(10)) = 0 AND instr(value, char(13)) = 0
                       AND instr(value, char(9)) = 0),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (website_id, key)
) STRICT;

-- +goose Down

-- Down drops every value, and there is nowhere else for them to go. A page
-- that names one keeps its marker in the stored text; a binary from before this
-- migration does not know the marker and prints it as it stands.
DROP TABLE IF EXISTS site_values;
