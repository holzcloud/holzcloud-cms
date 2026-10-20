---
phase: 21-deploy-update-rollback
plan: 04
subsystem: infra
tags: [deploy, docs, bash, docker, systemd, rollback, snapshot, migrations, sqlite]

requires:
  - phase: 21-deploy-update-rollback
    provides: "the played update (update.txt with SNAPSHOT and MARKER, states s0, s1, s1m, compare_states, holzcloud.prev) of plan 21-03"
provides:
  - "run.sh subcommand rollback: plays the Rolling back section (block 1 with a snapshot, block 2 without), captures s2, compares with s0 (or s1m), writes rollback.txt with nine PASS checks"
  - "run.sh subcommand negative: the old binary against the migrated database, once, recorded in negative.txt, the new binary put back; SCENARIO lines in scenario.txt"
  - "deploy/DEPLOY.md: ## Rolling back with the migration limit as the first paragraph, the observed negative case, two pasteable blocks, what a rollback undoes and does not cover, goose Down developer note; Updating and Recovery cross-linked"
  - "scratch evidence under deploy/test/out/amd64-rollback-pass1 and amd64-rollback-pass1b: rollback.txt, negative.txt, compare-s0-s2.txt, state-s2.txt, scenario.txt"
affects: [21-05, 21-06, 22-backup-restore]

actuals:
  tokens: 5518
  tasks: 2
  commits: 2
plan_head_before: f7b7f80f76ba3b5ba277de521c99152952a5a2ec
plan_head_after: 50708cacc305fd9056758e619066b3aa08a27b14

tech-stack:
  added: []
  patterns:
    - "A procedure the document says not to follow is played as a SCENARIO, never read from the document, and its recorded result decides which sentence the document may contain"
    - "The play is done by the document's own lines (doc_play block N); the harness only asserts afterwards, and adds no wait inside the play"
    - "The set-aside copy is read with an immutable URI so no -wal or -shm file appears next to it"

key-files:
  created: []
  modified:
    - deploy/test/run.sh
    - deploy/DEPLOY.md
    - deploy/DEPLOY-RUN.md

key-decisions:
  - "Rollback route is the snapshot route only (D-08); goose down is neither offered nor played (D-10), the Down sections are one developer-note sentence (60 of 60, counted with grep)"
  - "The migration limit is the first paragraph of Rolling back, as a bold sentence containing 'must not'; the observed behaviour follows from negative.txt and nothing else"
  - "SCENARIO is added next to DEVIATION in DEPLOY-RUN.md: a step the harness performs that the document must not contain; it goes to scenario.txt, never to deviations.txt"
  - "The text does not claim that a restart clears stale sessions: sessions are rows of the database (the sessions table), so a rollback brings the snapshot's sessions back and a restart does not remove them; the operator re-applies revocations in the admin"

patterns-established:
  - "OUT_DIR/rollback.txt: MODE snapshot|binary, then PASS/BAD lines version old, schema, marker gone (or marker kept and token kept), no token column, pre-update page, owner and mode, set-aside copy, no migration on restart, integrity ok, then INFO lines"
  - "OUT_DIR/negative.txt keys: NEGATIVE-RESULT, readyz, schema, journal, insert-1, insert-2, orders, RESTORE (refusal lines when it refuses)"

requirements-completed: [DEP-03]

coverage:
  - id: D1
    description: "A rollback through the Rolling back section restores binary and data: the old version runs, the pre-update data is present (52 PASS, 0 BAD against state s0), the data written after the update (marker page, extra order rows) is gone, the live database is holzcloud 600 and reports integrity ok, the migrated database stays as holzcloud.sqlite.after-update at schema 60, and the restart migrates and snapshots nothing"
    requirement: DEP-03
    verification:
      - kind: integration
        ref: "run.sh build up install baseline update rollback (pass 1) and build up install baseline update negative rollback (pass 1b, against the final document): ROLLBACK OK, MODE: snapshot, compare-s0-s2.txt pass=52 bad=0, rollback.txt nine PASS, LEAK-CHECK OK"
        status: pass
    human_judgment: false
  - id: D2
    description: "The document states from observation what an older binary does against a migrated database: it does not refuse, answers /healthz and /readyz ok, logs nothing about the schema, and the second order insert fails with UNIQUE constraint failed: orders.token; the limit is the first paragraph"
    requirement: DEP-03
    verification:
      - kind: integration
        ref: "negative.txt of pass 1b: NEGATIVE-RESULT: starts, readyz ok v0.0.10, schema: 60, journal: none, insert-1: ok, insert-2: Runtime error near line 2: UNIQUE constraint failed: orders.token (19), RESTORE: ok"
        status: pass
      - kind: command
        ref: "section greps of the task 2 verify: first paragraph contains 'must not', 'does not refuse', 'UNIQUE constraint failed', goose, password; go test ./cmd/holzcloud -run 'Deploy|Caddyfile'"
        status: pass
    human_judgment: false
  - id: D3
    description: "Updating and the Recovery text 'A failed migration' link to #rolling-back; the recovery text says the printed command restores data only, the old binary comes first, the -wal and -shm files go, reset-failed clears the start limit"
    requirement: DEP-03
    verification:
      - kind: command
        ref: "awk section extraction of Updating and 'A failed migration' with grep for #rolling-back, reset-failed, binary"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-10-12
status: complete
---

# Phase 21 Plan 04: Rollback played, migration limit stated from observation Summary

**A snapshot rollback to v0.0.10 after an update across migrations 59 and 60 is played out of a new Rolling back section and restores binary and data (52 PASS, 0 BAD against the pre-update state), and the old binary is shown to start against the migrated schema without complaint and to fail only at the second order insert, which is why the migration limit leads the section.**

## Performance

- **Duration:** about 25 min of tool time (builds and emulated container runs included)
- **Completed:** 2026-10-12
- **Tasks:** 2
- **Files modified:** 3 (deploy/test/run.sh +289, deploy/DEPLOY.md +104, deploy/DEPLOY-RUN.md +7)

## Accomplishments

- **`rollback` subcommand.** `cmd_rollback` reads `SNAPSHOT:` from `update.txt`: a path selects MODE snapshot and plays block 1 of "Rolling back", `none` selects MODE binary and block 2. The play is the document's own lines through `doc_cmd`; the document's verification line waits for the start itself, the harness adds no wait. Afterwards it captures state `s2`, compares it with `s0` (snapshot) or `s1m` (binary) and writes `rollback.txt`: `version old`, `schema`, `marker gone` (or `marker kept` and `token kept`), `no token column`, `pre-update page`, `owner and mode` (`holzcloud 600`), `set-aside copy` (holzcloud.sqlite.after-update at schema 60, read immutable so no side file appears), `no migration on restart`, `integrity ok`, plus INFO lines with the counts of users, sessions and `ai_tokens` of the restored state. MODE binary is implemented and left to plan 21-05 to play.
- **`negative` subcommand.** Stops the service, copies `holzcloud.prev` over the live binary, starts, polls `/healthz`, records `negative.txt`, then puts the new binary back (`docker cp`, stop, copy, start, version check) and writes `RESTORE: ok`. The next `rollback` starts from the real post-update state plus the extra order row the old code wrote, and the comparison with `s0` is still clean.
- **Observed result of the negative case (pass 1b):** `NEGATIVE-RESULT: starts`; `readyz: {"status":"ok","version":"v0.0.10","commit":"ec365d6","integrity":"ok"}`; `schema: 60`; `journal: none`; `insert-1: ok`; `insert-2: Runtime error near line 2: UNIQUE constraint failed: orders.token (19)`; `orders: 2`. The old binary logs nothing about a newer schema.
- **Rolling back section.** The first paragraph is the limit (bold, "must not", snapshot restore loses everything written since). The second states what was observed, only that, with the reason (migration 00060 made `orders.token` unique, the old code leaves it empty) and that the inserts were direct, in the statement shape of the old code, not through the shop. Then which block to use and the trap in the restore line, two pasteable fenced blocks, and bullets: the set-aside copy and how to undo, ownership kept, credential changes undone (password, sessions, API keys), files not covered, newest five kept, and the one developer-note sentence about the goose Down sections (60 of 60, no command runs them).
- **Cross-links.** Updating gets one sentence with a link to `#rolling-back`. "A failed migration" keeps the program's printed command as quoted and adds that it restores data only while the NEW binary is what runs, sends the reader to Rolling back from its first block, and names `reset-failed` and `StartLimitBurst`.
- **Result (pass 1b, fresh container, against the final document):** `ROLLBACK OK`, `NEGATIVE OK`, `compare-s0-s2.txt` pass=52 bad=0, `rollback.txt` MODE snapshot, nine PASS and no BAD, `LEAK-CHECK OK`, `deviations.txt` holds only the named set, `scenario.txt` the two SCENARIO lines.

## Task Commits

1. **Task 1: the snapshot rollback end to end (tracer)** - `94f3dc9` (feat)
2. **Task 2: negative case, migration-limit text, Recovery cross-link** - `50708ca` (docs)

The tracer ran clean on its first pass (pass 1) before Task 2 started, and the whole chain with the negative case was replayed against the final document.

## Deviations from Plan

### Judgment calls beyond the plan text

- **`cmd_negative` landed in the Task 1 commit.** Both subcommands were written in one edit of `run.sh`, so `94f3dc9` already contains `negative` although only `rollback` was played at that point; the negative case was first played in Task 2 (pass 1b). The Task 1 commit message names only `rollback`.
- **Sessions and restart.** The plan asked the document to say "restart so no stale session survives". Sessions are rows of the database, so a restart clears nothing and the sentence would be false. The document says the opposite and tells the operator to re-apply revocations in the admin.
- **A warning about the restore line was added** to the paragraph that chooses the block: `ls -t` takes the newest snapshot by modification time, which is wrong when this update wrote none. Pass 1 showed a second snapshot, `pre-upgrade-v0.0.10-...sqlite` of about 12 KB, written by the first start of the fresh install (an empty database); block 1 after an update that wrote no snapshot would restore that or an older update's snapshot. The text names the sign (version in the name, time of the journal line) and the size hint.
- **Two extra checks** in `rollback.txt` beyond the eight named: `pre-update page` (the acceptance criterion of the plan) and, for MODE binary, `marker kept` and `token kept`.
- **`old_order_run`** was split out of `insert_old_order` so the negative case can read the error text instead of dying; `insert_old_order` behaves as before.

**Total deviations:** none of Rules 1-4. **Impact:** none on scope; no Go source or schema file changed (`git diff --quiet ebe7dea -- cmd internal plugins sdk tools` exits 0).

## Findings for plan 21-05's DEP-03 record

- **The incomplete restore message of the program** (`cmd/holzcloud/main.go`, `restore with: systemctl stop holzcloud && cp PATH DBPATH && systemctl start holzcloud`) restores data only, runs under the NEW binary, leaves the `-wal` and `-shm` files and ignores the start limit. Not a blocking defect, so main.go is unchanged; corrected in the Recovery text instead.
- **A first start writes a snapshot of an empty database** (`pre-upgrade-v0.0.10-...sqlite`, 12288 bytes in the play). It is the trap that the new warning in Rolling back names.
- **The old binary does not refuse a newer schema** (observed, not assumed), see above.
- **Harness artefact, not a document finding:** `attach_check` of plan 21-03 leaves `-shm` and `-wal` files next to the update's snapshot (the readonly ATTACH creates them); the glob `pre-upgrade-*.sqlite` of the document does not match them.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None: no endpoint, auth path or schema change. T-21-13/T-21-14: the play stops the service, removes `-wal` and `-shm`, sets the migrated database aside, and integrity is asserted after the restart. T-21-15: Rolling back names the password, session and API key reversal; the counts of users, sessions and `ai_tokens` of the restored state are in `rollback.txt`. T-21-16: restore as `holzcloud`, `owner and mode` asserted as `holzcloud 600`, the set-aside copy called out as holding password hashes. T-21-17: the limit leads the section and is backed by the played negative case. T-21-18: test data only, `LEAK-CHECK OK`.

## Issues Encountered

None open. `deploy/test/out/amd64-rollback-pass1` and `amd64-rollback-pass1b` are gitignored scratch.

## Next Phase Readiness

Ready for 21-05: `rollback` already implements MODE binary (block 2), to be played after an update that wrote no snapshot (a pair of builds with no migration between them); `negative.txt`, `rollback.txt` and the findings above are the material for the DEP-03 record in DEPLOY-RUN.md. After `negative` the chain still compares clean against `s0`.

## Self-Check: PASSED

- FOUND: deploy/test/run.sh (cmd_rollback, cmd_negative, scenario, old_order_run), deploy/DEPLOY.md (`## Rolling back`, two bash blocks, `#rolling-back` in Updating and in "A failed migration"), deploy/DEPLOY-RUN.md (SCENARIO paragraph)
- FOUND: commits 94f3dc9 and 50708ca (ancestors of HEAD); `git rev-list --count f7b7f80..HEAD` is 2
- FOUND: deploy/test/out/amd64-rollback-pass1b/{rollback.txt,negative.txt,compare-s0-s2.txt,state-s2.txt,scenario.txt} and amd64-rollback-pass1/
- `bash -n` passes with bash and /bin/bash; `go test ./cmd/holzcloud -run 'Deploy|Caddyfile'` passes
