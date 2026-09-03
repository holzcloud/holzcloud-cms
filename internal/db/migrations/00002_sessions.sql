-- +goose Up
CREATE TABLE sessions (
    token  TEXT PRIMARY KEY,
    data   BLOB NOT NULL,
    expiry REAL NOT NULL
);
CREATE INDEX idx_sessions_expiry ON sessions(expiry);

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_expiry;
DROP TABLE IF EXISTS sessions;
