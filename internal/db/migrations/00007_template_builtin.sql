-- +goose Up
ALTER TABLE templates ADD COLUMN is_builtin INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE templates DROP COLUMN is_builtin;
