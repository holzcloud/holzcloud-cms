-- +goose Up

-- Messages a visitor sent through a contact form.
--
-- They are stored, not mailed. Sending mail would mean an SMTP server, its
-- credentials in the configuration, a queue for when it is unreachable, and a
-- delivery reputation to maintain — on a machine in someone's hallway. A
-- message that sits in the admin until it is read cannot bounce, cannot be
-- rejected as spam and needs nothing configured.
--
-- Deliberately absent: the sender's IP address and user agent. Neither is
-- needed to answer a message, and storing them would turn "this site sets no
-- cookies and collects nothing" from true into a sentence needing footnotes.
CREATE TABLE form_messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    -- page_id says which page the form stood on. SET NULL rather than CASCADE:
    -- deleting a page must not delete the enquiries that came through it.
    page_id    INTEGER REFERENCES pages(id) ON DELETE SET NULL,
    name       TEXT    NOT NULL,
    email      TEXT    NOT NULL,
    subject    TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    -- read_at is NULL until someone opens it, which is what the unread badge
    -- counts.
    read_at    TEXT
) STRICT;

CREATE INDEX idx_form_messages_website ON form_messages(website_id, created_at DESC);

-- The address enquiries should reach, shown next to the form so a visitor who
-- would rather write an ordinary mail can.
ALTER TABLE websites ADD COLUMN contact_email TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE websites DROP COLUMN contact_email;
DROP INDEX IF EXISTS idx_form_messages_website;
DROP TABLE IF EXISTS form_messages;
