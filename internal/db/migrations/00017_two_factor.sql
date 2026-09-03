-- +goose Up

-- The shared secret of an account's authenticator app.
--
-- Empty means two-factor is not set up. It is stored in the clear because it
-- has to be: verifying a code means computing it, which needs the secret
-- itself. Hashing it would make verification impossible, and encrypting it with
-- a key that sits in the same data directory protects against nothing.
ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT '';

-- The secret of an enrolment that has not been confirmed yet.
--
-- Separate from totp_secret so that starting a new enrolment cannot break the
-- factor that currently works: someone who merely opens the setup page — or
-- gets halfway through moving to a new phone — must still be able to sign in
-- with the old app until the new one has proved itself.
ALTER TABLE users ADD COLUMN totp_pending_secret TEXT NOT NULL DEFAULT '';

-- NULL until the account has proved the app works by entering one code.
-- Without this an account could lock itself out by scanning the code, closing
-- the app and being asked for a number nobody can produce.
ALTER TABLE users ADD COLUMN totp_confirmed_at TEXT;

-- The counter of the last code accepted.
--
-- A code stays valid for thirty seconds. Without remembering which one was
-- used, anyone who reads the six digits over a shoulder can sign in for the
-- rest of that window.
ALTER TABLE users ADD COLUMN totp_last_step INTEGER NOT NULL DEFAULT 0;

-- The way back in when the phone is gone.
--
-- Making two-factor mandatory without these would make the phone a single point
-- of failure for the whole site. They are hashed the same way tokens are: the
-- plain code is shown once, at setup, and never stored.
CREATE TABLE user_recovery_codes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT    NOT NULL,
    used_at    TEXT,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE INDEX idx_user_recovery_codes_user ON user_recovery_codes(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_user_recovery_codes_user;
DROP TABLE IF EXISTS user_recovery_codes;
ALTER TABLE users DROP COLUMN totp_last_step;
ALTER TABLE users DROP COLUMN totp_confirmed_at;
ALTER TABLE users DROP COLUMN totp_pending_secret;
ALTER TABLE users DROP COLUMN totp_secret;
