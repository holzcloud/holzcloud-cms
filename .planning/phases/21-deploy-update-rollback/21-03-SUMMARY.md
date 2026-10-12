---
phase: 21-deploy-update-rollback
plan: 03
subsystem: infra
tags: [deploy, docs, bash, docker, systemd, update, migrations, snapshot, sqlite]

requires:
  - phase: 21-deploy-update-rollback
    provides: "baseline and update subcommands, the v0.0.10 seeded install and the pass-1 evidence of the Updating section as shipped (plan 21-02)"
provides:
  - "run.sh capture_state complete (page, media, public, plugins, order, login, owner, binary), compare_states with a reasoned VOLATILE list, compare_selftest, the compare subcommand, attach_check against the pre-upgrade snapshot, update_marker"
  - "deploy/DEPLOY.md Updating section in three blocks: old binary kept as holzcloud.prev and back-up taken with the OLD binary, build/transfer/stop/copy/start, readyz plus journal check, with the observed facts about the snapshot"
  - "deploy/DEPLOY-RUN.md: DEP-02 section, five DEP-02 findings rows with verbatim pass-1 output, Summary row, DEVIATION[fixture]"
  - "scratch evidence under deploy/test/out/amd64-update-pass1b and amd64-update: state-s0, state-s1, state-s1m, compare-s0-s1, attach-s1, update.txt with the MARKER line"
affects: [21-04, 21-05, 21-06, 22-backup-restore]

actuals:
  tokens: 11160
  tasks: 2
  commits: 2
plan_head_before: 292526d565381975a8ff571055937cdf7929d955
plan_head_after: 469d9ef8ba247122ea9771b10e3a3a3b6350983d

tech-stack:
  added: []
  patterns:
    - "A comparison is only believed after it was shown to fail: compare_selftest alters a copy of the state file and requires BAD"
    - "Independent second proof: the service's own pre-upgrade snapshot attached read-only to the live database, counts compared with S0 and with the live tables"
    - "Counts and content hashes only: no row of a credential table reaches any output file"
    - "State-file keys and PASS/BAD/INFO vocabulary are the interface for later plans (rollback compares s1 or s1m against a restored state)"

key-files:
  created: []
  modified:
    - deploy/test/run.sh
    - deploy/DEPLOY.md
    - deploy/DEPLOY-RUN.md

key-decisions:
  - "VOLATILE (not compared for equality, printed INFO volatile): sessions, activity_log, mail_outbox, outbox, plugin_store, goose_db_version, oauth_codes, each with its reason in a comment and in DEPLOY-RUN.md; a difference in any other table is a BAD and an investigation"
  - "compare_states also prints INFO a-only for a table that only A has, so the rollback comparison of plan 21-04 (new schema against restored old schema) can use the same function"
  - "The self-test writes its deliberate BAD lines to compare-selftest.txt, never into compare-s0-s1.txt, so the real result files stay free of BAD"
  - "attach_check requires, per non-volatile table, snapshot == S0 and live == snapshot; the PASS vocabulary of the plan is unchanged"
  - "The page hash covers both stored columns (content_markdown and content_html), printed as two digests on one page: line"
  - "The CLI-migrates-silently fact is read from cmd/holzcloud/cli.go openForCLI and said so in DEPLOY-RUN.md; it was not played, no harness step runs a subcommand of the new binary on the old database"

patterns-established:
  - "OUT_DIR/state-LABEL.txt keys: identity, binary, schema, table NAME N, page, media, order, owner, public, plugins, login"
  - "OUT_DIR/update.txt keys: OLD, NEW, MIGRATIONS, APPLIED, SNAPSHOT, SCHEMA, PREV, BACKUP, COMPARE, ATTACH, JOURNAL, MARKER"
  - "Marker page: slug after-update on website Test, published, 200 for Host test.local, captured as state s1m for plan 21-04 to remove by rollback"

requirements-completed: [DEP-02]

coverage:
  - id: D1
    description: "An update of a v0.0.10 install across two migrations (58 to 60) through the Updating section loses no data: equal row counts of 46 content tables, equal page and media hashes, login, public page, five running plugins, filled order token, and the service's own pre-upgrade snapshot agrees with S0 and the live tables"
    requirement: DEP-02
    verification:
      - kind: integration
        ref: "run.sh build up install baseline update from fresh containers, pass 1b and pass 2: UPDATE OK, compare-s0-s1.txt 52 PASS 0 BAD, attach-s1.txt 50 PASS 0 BAD (snapshot schema 58, live schema 60, snapshot mode holzcloud 600, backup mode holzcloud 700), MARKER: ok, LEAK-CHECK OK"
        status: pass
      - kind: integration
        ref: "compare-selftest.txt: an altered copy of state-s1.txt yields BAD rows pages and BAD page sha256; a manual run of run.sh compare with a changed pages count reports BAD"
        status: pass
    human_judgment: false
  - id: D2
    description: "The Updating section can be followed literally: it keeps the old binary, takes the backup with the OLD binary, tells what the service does before it migrates and what to look for in the journal"
    requirement: DEP-02
    verification:
      - kind: integration
        ref: "pass 2 replay: PREV: present, BACKUP: a directory of mode 700, SNAPSHOT: /opt/holzcloud/data/pre-upgrade-..., steps.txt no counted failure, deviations.txt only the named set including fixture"
        status: pass
      - kind: unit
        ref: "go test ./cmd/holzcloud -run 'Deploy|Caddyfile'; section greps for holzcloud.prev, holzcloud version, backup.sh, pre-upgrade snapshot written, migration applied, StartLimitBurst, OLD binary, three bash blocks"
        status: pass
      - kind: command
        ref: "grep -F loop over the backtick quotes of the five DEP-02 findings rows against the pass-1 scratch files: none missing"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-10-12
status: complete
---

# Phase 21 Plan 03: No data lost across the update, Updating section repaired Summary

**An update of a seeded v0.0.10 install across migrations 00059 and 00060 is shown to lose nothing by a state comparison that was proven able to fail and by an independent ATTACH comparison against the service's own pre-upgrade snapshot; the Updating section now keeps the old binary, backs up with the OLD binary and says what the service does before it migrates.**

## Performance

- **Duration:** about 12 min of tool time (builds and emulated container runs included)
- **Started:** about 2026-10-12T12:09Z
- **Completed:** 2026-10-12T12:21Z
- **Tasks:** 2
- **Files modified:** 3 (deploy/test/run.sh +379, deploy/DEPLOY.md +69, deploy/DEPLOY-RUN.md +122)

## Accomplishments

- **Complete state capture.** `capture_state` adds `binary`, `page` (sha256 of the stored Markdown and of the stored HTML of `plugintest`, hashed inside the container), `media` (sha256 of the stored original next to the sha256 of the staged source), `public` (status of `/plugintest` and `/` for `test.local` and the page title in the body), `plugins` (running with an empty `last_error`, through `list_plugins`), `order` (token length or `no-token-column`), `login` (status and redirect only, through FORM_SH with the new password-file argument) and `owner`. Only counts and hashes leave the container.
- **A comparison that can fail.** `compare_states A B OUTFILE` writes PASS, BAD and INFO lines, with a VOLATILE list whose every entry has its reason; `compare_selftest` alters a copy (the `pages` count plus one, the page hash with digits in front) and demands a BAD for both, written to `compare-selftest.txt`. A `run.sh compare A B [OUT]` subcommand reruns the comparison on two state files without a container.
- **Independent proof.** `attach_check` runs one `sqlite3 -readonly` session as the service user on the live database with the snapshot attached: schema of both, owner and mode of the snapshot, and the counts of every table both have. For every non-volatile table the snapshot equals S0 and equals the live count.
- **Marker for the rollback.** `update_marker` creates and publishes the page `after-update` through the new binary, confirms `GET /after-update` is 200, and `s1m` is captured; plan 21-04's rollback must remove it again.
- **Updating section repaired** (three bash blocks): keep the running binary as `/opt/holzcloud/holzcloud.prev`, print its version, take the backup with `backup.sh` while the OLD binary is still in place (the reason is stated: every subcommand of a new binary migrates silently without a snapshot); build with the stamped line, transfer, stop, copy, start; check `/readyz` through `curl --retry` and read the journal for `pre-upgrade snapshot written` and `migration applied`. The prose states what was observed about the snapshot: name with the NEW version, old schema, only when migrations are pending, database only, newest five kept, mode 600 for the service user, and that the path is the one in the journal. `StartLimitBurst` and the pointer to Recovery are named; Rolling back is not mentioned (plan 21-04).
- **Result (pass 2, fresh container).** OLD `v0.0.10 ec365d6`, NEW `v0.0.25-33-gb7a6c8d`, 58 to 60 migration files (2 pending, 2 applied), SCHEMA 58 -> 60, PREV present, backup directory mode 700, snapshot `pre-upgrade-v0.0.25-33-gb7a6c8d-...sqlite` mode 600 owned by `holzcloud`, `compare-s0-s1.txt` 52 PASS and no BAD (46 table counts, page, media, login, public page, plugins 5/5, order token backfilled to 32 characters), `attach-s1.txt` 50 PASS and no BAD, MARKER ok.
- **DEPLOY-RUN.md.** DEP-02 Summary row, five findings rows (no backup command, no old binary kept, unstamped build line in Updating, nothing about the snapshot or the journal, no check after the start) with quotes proven present in the pass-1 scratch files, DEVIATION[fixture] in Deviations named, and the section `DEP-02: an update across migrations` (the old tag and the two migrations, S0 and its fixtures, the method and the VOLATILE table with reasons, the result table, the observed snapshot, what the new binary's CLI would have done, what is not shown).

## Task Commits

1. **Task 1: the no-data-loss proof end to end (tracer)** - `b7a6c8d` (feat)
2. **Task 2: Updating section repaired, DEP-02 recorded, pass 2 replayed** - `469d9ef` (fix)

The tracer's verify (pass 1b, section still as shipped) ran clean before Task 2 started: the proof worked end to end against the unrepaired document, then pass 2 against the repaired one.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The page title check was false in both states**
- **Found during:** Task 1, first run of the chain (`BAD public page ... title=no` although the page answered 200 and carries `<title>Plugintest — Test</title>`)
- **Issue:** `http_body | grep -q` under `set -o pipefail`: grep exits at the first match and the curl in front dies of SIGPIPE, so the pipeline reports failure
- **Fix:** the body goes into a variable first, then `grep -q '<title>Plugintest'` on it
- **Files modified:** deploy/test/run.sh
- **Commit:** b7a6c8d

### Judgment calls beyond the plan text

- `compare_states` also prints `INFO a-only TABLE` for a table only the first file has, so the comparison of plan 21-04 (new schema against restored old schema) works with the same function. Tables only in B still print `INFO new-only`.
- The `compare` subcommand (two state files, optional OUTFILE) was added for reruns and for the manual tamper check; `update.txt` has two more keys (`COMPARE`, `ATTACH`) with the PASS and BAD counts.
- A BAD from the self-test is written to `compare-selftest.txt`, never to the real comparison file.
- DEPLOY.md's step 3 also tells that `Failed to connect` lines while `curl` waits are the retries (observed in step-28.txt of the replay); the played lines are unchanged by it.
- The CLI-migrates-silently remark is read from `openForCLI` and labelled as read, not played.

**Total deviations:** 1 auto-fixed (Rule 1). **Impact:** none on scope.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None: no endpoint, auth path or schema change. No schema or migration file was edited. T-21-09: counts and content hashes only, `LEAK-CHECK OK`, and the login password, `argon2` and key-shaped strings occur in no scratch file or log. T-21-10: `attach-s1.txt` asserts snapshot `holzcloud 600` and backup directory `holzcloud 700`. T-21-11: Updating names the journal check, `StartLimitBurst` and Recovery, and the harness asserts `/readyz` after the start.

## Issues Encountered

None open. `deploy/holzcloud.service` is unchanged (`git diff --quiet` exits 0); no Go source file changed. The scratch directories `deploy/test/out/amd64-update-pass1b` and `amd64-update` are gitignored, as before.

## Next Phase Readiness

Ready for 21-04: the container image of pass 2 ends with the updated service, `holzcloud.prev` and a backup in place, a snapshot named in `update.txt` (`SNAPSHOT:`), and the marker page `after-update` captured as state `s1m`. A rollback (stop, remove `-wal` and `-shm`, copy the snapshot, put `holzcloud.prev` back, start) can be compared with `run.sh compare state-s0.txt state-rolled-back.txt`; the page `after-update` and the order token must be gone again, the session and activity counts are volatile. The state-file keys and the PASS names of this plan are the interface.

## Self-Check: PASSED

- FOUND: deploy/test/run.sh (compare_states, compare_selftest, attach_check, update_marker, is_volatile, cmd_compare), deploy/DEPLOY.md (Updating, three bash blocks), deploy/DEPLOY-RUN.md (heading `DEP-02: an update across migrations`, five DEP-02 findings rows)
- FOUND: commits b7a6c8d and 469d9ef (ancestors of HEAD); `git rev-list --count 292526d..HEAD` is 2
- FOUND: deploy/test/out/amd64-update/{compare-s0-s1.txt,attach-s1.txt,update.txt,state-s0.txt,state-s1.txt,state-s1m.txt} and amd64-update-pass1b/
- `bash -n` passes with bash and /bin/bash; `go test ./cmd/holzcloud -run 'Deploy|Caddyfile'` passes; `git diff --quiet -- deploy/holzcloud.service` exits 0
