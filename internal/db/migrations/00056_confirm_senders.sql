-- +goose Up

-- Whether a person who writes through a form gets told that it arrived.
--
-- FORM-02 of milestone v2.1. A plugin may ask for one copy of its notification
-- to go to the address that notification already carries as its ReplyTo — the
-- person who wrote. Asking is not being allowed: this column is the operator's
-- own decision, per website, and the host sends nothing without it.
--
-- Default 0, and that is not timidity. Switching it on means this website
-- starts sending e-mail to addresses that visitors type into it, which is a
-- thing an operator should do knowingly: a form being filled in by a robot with
-- somebody else's address turns every submission into a message that person did
-- not ask for. The operator who reads the setting and switches it on has
-- weighed that; an upgrade that switched it on for them has not.
ALTER TABLE websites ADD COLUMN confirm_senders INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Dropping it means nobody is told any more, which is the state before this
-- migration and is not lossy in any way that matters: no message is lost, only
-- the courtesy of a receipt.
ALTER TABLE websites DROP COLUMN confirm_senders;
