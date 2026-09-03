-- +goose Up

CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password   TEXT    NOT NULL,
    role       TEXT    NOT NULL DEFAULT 'editor' CHECK (role IN ('admin', 'editor')),
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- +goose Down
DROP TABLE IF EXISTS users;
