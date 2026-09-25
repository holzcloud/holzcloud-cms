-- +goose Up

-- The third level of an access key: besides reading and writing content, an
-- admin key may manage the installation itself — users, plugins, further keys.
--
-- Kept apart from can_write rather than folded into a level column, so every
-- query that already asks "may this key write?" keeps its answer. The CHECK
-- holds the one rule between the two: whoever may administer may also write,
-- and an admin key reaches every website, because a user or a plugin belongs
-- to the installation and not to one site.
ALTER TABLE ai_tokens ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0
    CHECK (is_admin IN (0, 1));

-- +goose Down
ALTER TABLE ai_tokens DROP COLUMN is_admin;
