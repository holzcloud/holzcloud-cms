-- +goose Up

-- The requests people make about their data.
--
-- LEG-03 of phase 33.1. One table for every such request, told apart by its
-- kind: 'optout' comes from the "Do Not Sell or Share" page now, and the data
-- requests of Phase 33.2 (access, correction, erasure) arrive later in the
-- same table (33.2 D-09). That is why the kind is checked in Go
-- (internal/dsr) and not by a CHECK list here: a new kind is then a line of
-- Go, not a rebuild of a table that by then holds records which must be kept.
--
-- No IP address and no user agent. The request is used only to record it and
-- to answer it (CCPA regs § 7025(c)(2)), and the privacy page already promises
-- that a form stores neither.
--
-- Every field a person types is optional, but a row must say something: a
-- name, an address, or that the browser sent the Global Privacy Control
-- signal, which is a request in its own right.
--
-- handled_at records when the operator answered. Together with created_at it
-- is the record § 7101 asks a business to keep for 24 months (D-14).
--
-- Nothing in this phase deletes a row. Retention is Phase 33.2's, and it must
-- keep a US opt-out for at least 24 months.
CREATE TABLE privacy_requests (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id  INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    kind        TEXT    NOT NULL CHECK (kind <> '' AND length(kind) <= 40),
    locale      TEXT    NOT NULL DEFAULT '' CHECK (length(locale) <= 35),
    name        TEXT    NOT NULL DEFAULT '' CHECK (length(name) <= 200),
    email       TEXT    NOT NULL DEFAULT '' CHECK (length(email) <= 254),
    note        TEXT    NOT NULL DEFAULT '' CHECK (length(note) <= 2000),
    gpc         INTEGER NOT NULL DEFAULT 0 CHECK (gpc IN (0, 1)),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    handled_at  TEXT,
    CHECK (email <> '' OR name <> '' OR gpc = 1)
) STRICT;

-- The list per website and kind, newest first, and the hourly count the
-- opt-out form asks on every submission.
CREATE INDEX privacy_requests_by_site ON privacy_requests (website_id, kind, created_at);

-- +goose Down
-- Dropping the table forgets every request and when it was answered. A
-- rollback past this point is a decision to lose that record.
DROP INDEX IF EXISTS privacy_requests_by_site;
DROP TABLE IF EXISTS privacy_requests;
