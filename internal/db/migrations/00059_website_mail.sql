-- +goose Up

-- A website's own mail account.
--
-- One installation serves several websites, and until now all of them sent
-- through the one account in the environment. A shop confirmation from
-- velowerkstatt.example then came from whatever address the installation had,
-- and a receiver checking SPF and DKIM for velowerkstatt.example rightly
-- thought it suspicious. With a row here, everything about this website — the
-- enquiry notifications, the copy to the sender, the order mails — goes out
-- through its own server under its own address. Without one, nothing changes.
CREATE TABLE website_mail (
    website_id INTEGER PRIMARY KEY REFERENCES websites(id) ON DELETE CASCADE,
    host       TEXT NOT NULL,
    port       INTEGER NOT NULL DEFAULT 587 CHECK (port BETWEEN 1 AND 65535),
    username   TEXT NOT NULL DEFAULT '',
    -- AES-GCM under a key derived from HOLZCLOUD_SECRET_KEY, with the website
    -- id as additional data so a value copied into another row does not open.
    -- Empty for a relay that needs no password. The key is never stored here:
    -- this file is what goes into a backup.
    password   TEXT NOT NULL DEFAULT '',
    from_addr  TEXT NOT NULL,
    from_name  TEXT NOT NULL DEFAULT '',
    tls        TEXT NOT NULL DEFAULT 'starttls' CHECK (tls IN ('starttls', 'tls', 'none')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

-- +goose Down
DROP TABLE IF EXISTS website_mail;
