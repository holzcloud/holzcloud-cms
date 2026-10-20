---
phase: 21-deploy-update-rollback
plan: 05
subsystem: infra
tags: [deploy, docs, bash, docker, systemd, rollback, no-migration, chains, evidence]

requires:
  - phase: 21-deploy-update-rollback
    provides: "rollback and negative subcommands, the Rolling back section with the migration limit (plan 21-04); update, states and comparisons (plan 21-03)"
provides:
  - "run.sh update accepts a hop with no pending migration (SNAPSHOT: none, schema unchanged, PASS no snapshot in attach-s1.txt); rollback MODE binary played"
  - "deploy/DEPLOY.md: final Rolling back section (both cases, observed), the Updating check line no longer fails on a healthy no-migration update"
  - "deploy/DEPLOY-RUN.md: DEP-03 section, Summary row, five DEP-03 findings rows, scenario table"
  - "run.sh chains fresh (DEP-01) and lifecycle (DEP-02, DEP-03), each with summary.txt and LEAK-CHECK OK"
  - "the evidence set amd64-fresh, amd64-lifecycle, amd64-lifecycle-hop, made once against the final DEPLOY.md"
affects: [21-06, 22-backup-restore]

actuals:
  tokens: 6650
  tasks: 2
  commits: 2
plan_head_before: e249f08b5ed2ee30b7b7fdc171f8df8879c62ff6
plan_head_after: 2a40bd3e064a42607bda404a7559de329fdd66e5

tech-stack:
  added: []
  patterns:
    - "A claim of the document that only a play can settle (assumption A5) is written after the play, and the text names the pair of builds it was seen with and what the play cannot show"
    - "A chain is a function over the existing subcommands that writes one summary.txt in a fixed vocabulary; it refuses any TARGET but systemd-base"
    - "A check that compares against the previous state (token kept) instead of expecting a constant"

key-files:
  created: []
  modified:
    - deploy/test/run.sh
    - deploy/DEPLOY.md
    - deploy/DEPLOY-RUN.md
    - deploy/test/README.md

key-decisions:
  - "A no-migration update is accepted by update as a different kind of pass (pending=0: no snapshot, no new pre-upgrade file, no migration applied, schema unchanged), recorded as PASS no snapshot in attach-s1.txt in place of the ATTACH comparison"
  - "The claim in Rolling back block 2 is limited to what was played (v0.0.21 to HEAD, same schema) and says that equal schema does not tell what the newer code wrote into rows"
  - "Summary vocabulary fixed for phase 22 and the beta criteria: DEP-01 PASS, DEP-02 PASS, DEP-02 not applicable, DEP-03 PASS, NEGATIVE-RESULT, MODE, LEAK-CHECK OK"

patterns-established:
  - "OUT_DIR/summary.txt of `fresh`: DEP-01 PASS, CADDY verdict line, STEPS N played F failed, DEVIATIONS kinds, LEAK-CHECK OK"
  - "OUT_DIR/summary.txt of `lifecycle`: OLD, NEW, MIGRATIONS, SNAPSHOT, MODE, NEGATIVE-RESULT (when played), DEP-02 line, DEP-03 line, LEAK-CHECK OK"

requirements-completed: [DEP-03]

coverage:
  - id: D1
    description: "An update between two builds with the same schema (v0.0.21 to HEAD, 60 migration files each) writes no snapshot and applies no migration, and putting the old binary alone back is a complete rollback: version old, schema 60, the marker page still served, the order token as the update left it, 53 PASS and no BAD against the state after the update, restart migrates nothing, integrity ok"
    requirement: DEP-03
    verification:
      - kind: integration
        ref: "run.sh build up install baseline update rollback with OLD_TAG=v0.0.21 (amd64-hop): UPDATE OK, ROLLBACK OK, SNAPSHOT: none, pending=0, PASS no snapshot, MODE: binary, PASS marker kept, PASS version old, no BAD in compare-s0-s1.txt, compare-s1m-s2.txt, rollback.txt"
        status: pass
    human_judgment: false
  - id: D2
    description: "The Rolling back section is final: limit sentence first, both cases explained with what was observed (v0.0.21 and HEAD for the no-migration case), no angle-bracket value in a played line; DEP-03 is recorded with the verbatim quotes of its findings"
    requirement: DEP-03
    verification:
      - kind: command
        ref: "section greps (no migration, v0.0.21), DEP-03 heading, five DEP-03 rows, SCENARIO[negative] named, every backtick quote of a DEP-03 row found verbatim in the scratch output, no AI-key-shaped string; go test ./cmd/holzcloud -run 'Deploy|Caddyfile'"
        status: pass
    human_judgment: false
  - id: D3
    description: "Two chains replay the proof from one command each: fresh ends in DEP-01 PASS with CADDY validated, 33 steps played and 0 failed, LEAK-CHECK OK; lifecycle with v0.0.10 ends in DEP-02 PASS, DEP-03 PASS, MODE snapshot, NEGATIVE-RESULT starts; lifecycle with v0.0.21 ends in DEP-02 not applicable, DEP-03 PASS, MODE binary, SNAPSHOT none and skips the negative case"
    requirement: DEP-03
    verification:
      - kind: integration
        ref: "evidence.sh in the scratchpad: three runs from fresh containers, each exit 0; greps of the plan's automated verifies on summary.txt, steps.txt, compare and rollback files"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-10-12
status: complete
---

# Phase 21 Plan 05: No-migration rollback played, DEP-03 recorded, two replay chains Summary

**An update between two builds with the same schema (v0.0.21 to HEAD) writes no snapshot, and putting the old binary alone back is observed to be a complete rollback (53 PASS, no BAD against the post-update state); the Rolling back section and DEPLOY-RUN.md record both cases, and `fresh` and `lifecycle` replay install, update and rollback from one command each against the final document.**

## Performance

- **Duration:** about 35 min of tool time (the container runs under amd64 emulation dominate)
- **Completed:** 2026-10-12
- **Tasks:** 2
- **Files modified:** 4 (deploy/test/run.sh +187, deploy/DEPLOY.md +22, deploy/DEPLOY-RUN.md +127, deploy/test/README.md +8)

## Accomplishments

- **No-migration hop in `update`.** When `MIGRATIONS:` says `pending=0`, `UPDATE OK` requires the schema unchanged (`SCHEMA: 60 -> 60`), `SNAPSHOT: none`, no new `pre-upgrade-*.sqlite` file and no `migration applied` line; the result is `PASS no snapshot` (plus three more PASS lines) in `attach-s1.txt`. `compare-s0-s1.txt` is unchanged and shows 53 PASS. `negative` keeps refusing in this case.
- **Rollback MODE binary played** (v0.0.21 to HEAD, `amd64-hop`): `PASS version old v0.0.21`, `PASS schema 60`, `PASS marker kept`, `PASS token kept`, `PASS no migration on restart`, `PASS integrity ok`, `compare-s1m-s2.txt` 53 PASS and no BAD. Assumption A5 of the research is settled for this pair of builds, and the text says only that.
- **Rolling back final.** Limit sentence first and unchanged; block 2 is explained as the case with the same schema (the old binary alone is a complete rollback, undoes nothing, loses nothing, data stays as the new version left it); a paragraph states what was observed with v0.0.21 and what the play cannot show (that equal schema says nothing about the rows the newer code wrote; the CHANGELOG is to be read). Every fact of plan 21-04 is kept.
- **DEPLOY-RUN.md.** Summary row for DEP-03; five DEP-03 findings rows (the Updating silence about the way back, the old binary against a newer schema, the incomplete restore command of the program, the WAL and SHM hazard, the failing Updating check line); the scenario table (`SCENARIO[negative]`, `SCENARIO[restore-new]`) in Deviations named; the section "DEP-03: rolling back, and the migration limit" with the played sequence, the rollback check tables for both cases, the negative result verbatim, what a rollback undoes and does not cover, why the snapshot route and not `goose down`, and the container-versus-server differences. Fragments from run output are verbatim in backticks (checked mechanically); source-file fragments are marked inferred and carry no backticks.
- **Chains `fresh` and `lifecycle`.** Both die unless `TARGET=systemd-base`, set `CLEANUP_DOWN=1`, call `leak_check` before teardown and write `summary.txt`. `fresh` tolerates exit 3 of Caddy, never exit 1. `lifecycle` plays `negative` only when `pending` is at least 1 and otherwise records `SCENARIO[negative]: skipped` with the reason, and prints `DEP-02 not applicable (no migration between these builds)`.
- **Evidence set, made once, against the final DEPLOY.md** (`git diff --quiet -- deploy/DEPLOY.md` is clean from before the set until the end), each from a fresh container with its own NAME and OUT_DIR:
  - `amd64-fresh`: `DEP-01 PASS`, `CADDY: validated`, `STEPS: 33 played, 0 failed`, `DEVIATIONS: browser caddy dropin mount scp`, `LEAK-CHECK OK`.
  - `amd64-lifecycle` (v0.0.10): `MIGRATIONS: old=58 new=60 pending=2`, `MODE: snapshot`, `NEGATIVE-RESULT: starts`, `DEP-02 PASS`, `DEP-03 PASS`, `LEAK-CHECK OK`.
  - `amd64-lifecycle-hop` (v0.0.21): `pending=0`, `SNAPSHOT: none`, `MODE: binary`, `DEP-02 not applicable`, `DEP-03 PASS`, `LEAK-CHECK OK`.

## Task Commits

1. **Task 1: the no-migration hop end to end, the final Rolling back text and the DEP-03 record (tracer)** - `2a11eeb` (feat)
2. **Task 2: chains fresh and lifecycle, the evidence set** - `2a40bd3` (feat)

The tracer gate was honoured: Task 2 started only after the hop run passed end to end (third pass; the first two found the two issues below).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The Updating check line fails on a healthy no-migration update**
- **Found during:** Task 1, hop pass 1 (`amd64-hop-pass1`)
- **Issue:** Step 3 of Updating ends in `journalctl ... | grep -E 'pre-upgrade snapshot written|migration applied'`. With no migration there is nothing to match, grep exits 1, and the played step reads `STEP 29 rc=1`. A script, or an operator chaining with `&&`, would stop on a healthy update.
- **Fix:** the line ends with `|| echo 'no migration, no snapshot'`, as the last line of Rolling back block 1 already did; the text says what the line prints when nothing migrates. Recorded as a DEP-03 finding.
- **Files modified:** deploy/DEPLOY.md, deploy/DEPLOY-RUN.md
- **Commit:** 2a11eeb

**2. [Rule 1 - Bug] The harness expected a 32-character order token after a binary-only rollback**
- **Found during:** Task 1, hop pass 1b (`amd64-hop-pass1b`): `BAD token kept: the order line of state-s2 is '2026-9001 0'`
- **Issue:** v0.0.21 already has the `orders.token` column, so the seeded order was written by the old code without a token (length 0, in every state). The check assumed 32.
- **Fix:** the check compares the token length of `s2` with that of `s1m` (what the update left), so putting the binary back must change nothing. It is a wrong expectation in the check, not a defect of the document, and is stated as such in the record.
- **Files modified:** deploy/test/run.sh
- **Commit:** 2a11eeb

### Judgment calls beyond the plan text

- **Findings quote check extended.** The plan's automated quote check names the scratch directories amd64-update-pass1(b), amd64-rollback-pass1(b) and amd64-hop. The finding for deviation 1 quotes `amd64-hop-pass1`, which is the evidence of the failure, so I kept that directory (renamed from amd64-hop) and ran the same check with `amd64-hop-pass1` and `amd64-hop-pass1b` added; with the plan's exact list the row would be reported as missing. The same check, so extended, passes with no missing quote.
- **Two small extra edits:** a paragraph in deploy/test/README.md and one in "How this was run" of DEPLOY-RUN.md name the chains and the evidence set. Neither touches DEPLOY.md.
- **Evidence set and commit order.** The set was made after Task 1 was committed and with the `fresh`/`lifecycle` code of Task 2 in the working tree (the chains were not yet committed); deploy/DEPLOY.md was the committed final text throughout. The NEW version in the summaries is therefore `v0.0.25-42-g2a11eeb`.

**Total deviations:** 2 auto-fixed (Rule 1), 3 judgment calls. **Impact:** none on scope; no Go source and no schema file changed (`git diff --quiet e249f08 -- cmd internal plugins sdk tools` exits 0).

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None: no endpoint, auth path or schema change. T-21-19: the no-migration claim was written only after the hop run passed and names the pair of builds. T-21-20: both chains call `leak_check` before teardown and print `LEAK-CHECK OK` (all three summaries). T-21-21: the set was made once, each run from a fresh container (`up` resets OUT_DIR), against the final DEPLOY.md.

## Issues Encountered

None open. Scratch directories `amd64-hop`, `amd64-hop-pass1`, `amd64-hop-pass1b`, `amd64-fresh`, `amd64-lifecycle` and `amd64-lifecycle-hop` under `deploy/test/out` are gitignored.

## Next Phase Readiness

Plan 21-06 can paste from `deploy/test/out/amd64-fresh`, `amd64-lifecycle` and `amd64-lifecycle-hop` (summary.txt and the files named in the plan's artifacts section). DEPLOY.md must not change before it does, or the set has to be made again.

## Self-Check: PASSED

- FOUND: deploy/test/run.sh (`cmd_fresh`, `cmd_lifecycle`, `nomigration_check`), deploy/DEPLOY.md (`## Rolling back`), deploy/DEPLOY-RUN.md (`## DEP-03: rolling back, and the migration limit`), deploy/test/out/amd64-fresh/summary.txt, amd64-lifecycle/summary.txt, amd64-lifecycle-hop/summary.txt
- FOUND: commits 2a11eeb and 2a40bd3 (ancestors of HEAD); `git rev-list --count e249f08..HEAD` is 2
- `bash -n` passes with bash and /bin/bash; `go test ./cmd/holzcloud -run 'Deploy|Caddyfile'` passes
