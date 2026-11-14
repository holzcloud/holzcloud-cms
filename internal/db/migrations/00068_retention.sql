-- +goose Up

-- Retention: how long a website keeps what it no longer needs, and the record
-- of the job that deletes it (Phase 33.2, DSR-04, D-04, D-05, D-06, D-09).
--
-- retention_periods holds the periods an operator chose, per website and kind
-- (D-04). A missing row means the default. The defaults live in internal/dsr
-- and not here, so a default that changes in a later release reaches every
-- website that never chose its own period, and only those. The kinds are the
-- record kinds of internal/dsr that belong to one website: form submissions,
-- privacy requests and orders. The server log is not among them: journald
-- keeps it for the whole server (deploy/journald-holzcloud.conf), and the
-- screen shows that period read-only. Six hundred months is fifty years, more
-- than any period the research found; a period of zero would delete
-- everything at the next run.
CREATE TABLE retention_periods (
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('form_submission', 'request', 'order')),
    months     INTEGER NOT NULL CHECK (months BETWEEN 1 AND 600),
    PRIMARY KEY (website_id, kind)
) STRICT;

-- One row per run of the retention job (D-05, D-09). The job ticks every hour
-- and runs once a day; this table is how it knows when it last succeeded, so
-- the gate survives a restart, and how the Retention screen and the 48-hour
-- warning of the dashboard know whether it runs at all. A failure is a row
-- with status 'failed' and its error, not only a line in the journal that
-- nobody reads. 'running' is a run in progress, or one a process left behind
-- when it stopped; the next run closes it.
CREATE TABLE retention_runs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at  TEXT    NOT NULL,
    finished_at TEXT,
    status      TEXT    NOT NULL CHECK (status IN ('running', 'ok', 'failed')),
    deleted     INTEGER NOT NULL DEFAULT 0 CHECK (deleted >= 0),
    error       TEXT    NOT NULL DEFAULT '' CHECK (length(error) <= 4000)
) STRICT;

-- The last successful run, and the last run of any status.
CREATE INDEX retention_runs_by_status ON retention_runs (status, started_at);

-- How much one run deleted per website and kind, and the error of a kind that
-- failed. A failing kind does not stop the others, so the run's own row can
-- only say that something failed; this row says what. website_id is NULL for
-- the installation-wide kinds (staff accounts, failed mail), which belong to
-- no website.
CREATE TABLE retention_deletions (
    run_id     INTEGER NOT NULL REFERENCES retention_runs(id) ON DELETE CASCADE,
    website_id INTEGER REFERENCES websites(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL,
    deleted    INTEGER NOT NULL DEFAULT 0 CHECK (deleted >= 0),
    error      TEXT    NOT NULL DEFAULT '' CHECK (length(error) <= 2000)
) STRICT;

-- The screen reads the latest run of one website.
CREATE INDEX retention_deletions_by_website ON retention_deletions (website_id, run_id);
-- Pruning a run deletes its rows through the cascade, which looks them up
-- by run.
CREATE INDEX retention_deletions_by_run ON retention_deletions (run_id);

-- An erased order is reduced to what the law asks to keep and stays until
-- retain_until (D-07, written by the erasure). The job reads both, so it never
-- deletes an erased order before the date its person was told, even when the
-- website's period is shortened later.
ALTER TABLE orders ADD COLUMN erased_at TEXT;
ALTER TABLE orders ADD COLUMN retain_until TEXT;

-- The moment from which "no successful run for 48 hours" can be judged on an
-- installation that never ran the job: without it a fresh installation would
-- warn at once, and an old one that never ran would have no date to count
-- from.
INSERT OR IGNORE INTO app_settings (key, value)
VALUES ('retention.since', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

-- +goose Down
-- The periods an operator chose and the record of past runs are forgotten.
-- Nothing that was deleted comes back.
DELETE FROM app_settings WHERE key = 'retention.since';
ALTER TABLE orders DROP COLUMN retain_until;
ALTER TABLE orders DROP COLUMN erased_at;
DROP INDEX IF EXISTS retention_deletions_by_run;
DROP INDEX IF EXISTS retention_deletions_by_website;
DROP TABLE IF EXISTS retention_deletions;
DROP INDEX IF EXISTS retention_runs_by_status;
DROP TABLE IF EXISTS retention_runs;
DROP TABLE IF EXISTS retention_periods;
