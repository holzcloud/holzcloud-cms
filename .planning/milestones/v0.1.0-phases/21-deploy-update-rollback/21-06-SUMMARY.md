---
phase: 21-deploy-update-rollback
plan: 06
subsystem: infra
tags: [deploy, docs, record, evidence, readme, hand-over]

requires:
  - phase: 21-deploy-update-rollback
    provides: "the evidence set amd64-fresh, amd64-lifecycle, amd64-lifecycle-hop made once against the final DEPLOY.md (plan 21-05), the chains fresh and lifecycle, the DEP-01 to DEP-03 sections and findings rows of plans 21-02 to 21-05"
provides:
  - "deploy/DEPLOY-RUN.md complete: ten sections in the order the plan fixes, a Findings table with 10 DEP-01, 5 DEP-02 and 5 DEP-03 rows, Container vs. server (16 rows, each measured, inferred or assumed), Not measured (11 items), Appendix R1 to R3 with 29 scratch files pasted byte for byte"
  - "deploy/test/README.md documents every subcommand (the 27 and compare), all 18 variables, every output file, the section Phase 21: install, update, rollback and a hand-over to phase 22 that matches the fixed DEPLOY.md"
  - "docs/deployment.md Installing agrees with DEPLOY.md: /tmp for the transfer, /opt/holzcloud/deploy, the service user first, the backup timer in place of the cron entry"
  - "the repository gates clean after the change: i18n 0 open 0 orphaned, gofmt, vet, english, cites, assembled, the Deploy and Caddyfile tests, bash -n"
affects: [22-backup-restore, 23]

actuals:
  tokens: 18717
  tasks: 2
  commits: 2
plan_head_before: 83b4348bdbf0116ae7d4dec5ec35ea958584cd13
plan_head_after: 710a23a71380e692515809fa3087ade724d66264

tech-stack:
  added: []
  patterns:
    - "A raw appendix is written by a script that copies the scratch files without translating bytes (newline=''), then proven by a block-for-block comparison and by a line-by-line grep -F; prose is edited by hand"
    - "A status column that says how the claim 'this carries over to a real server' is known: measured, inferred or assumed"

key-files:
  created: []
  modified:
    - deploy/DEPLOY-RUN.md
    - deploy/test/README.md
    - docs/deployment.md

key-decisions:
  - "The Summary table names the build under test (2a11eeb, v0.0.25-42-g2a11eeb, the NEW of all three summary.txt files) and says the record was made on top of 83b4348; the commit column is not the HEAD of the record, because the evidence is about that build and the document text of 2a11eeb"
  - "The sections DEP-02 and DEP-03 now name the final run directories (amd64-lifecycle, amd64-lifecycle-hop) and their numbers; the quotes of the Findings stay the output of the earlier passes, because that output showed the defects and the final set shows the repaired document"
  - "Status vocabulary of Container vs. server: measured means the thing itself was observed and the server does not differ in kind, inferred means read or reasoned, assumed means nothing in these runs speaks for it"

patterns-established:
  - "Appendix layout of the record: ### scenario heading with the command that makes the directory again, #### path of each file, one fenced text block per file"

requirements-completed: [DEP-01, DEP-02, DEP-03]

coverage:
  - id: D1
    description: "deploy/DEPLOY-RUN.md is complete: ten headings in order, findings reconciled with the final set, a Container vs. server table of 16 rows each labelled measured, inferred or assumed, Not measured naming native amd64, certificate issuance and a restore from a backup, and the deviations table with the scenario steps"
    requirement: DEP-01
    verification:
      - kind: command
        ref: "plan 21-06 task 1 automated verify 1: record-structure-ok (headings, row counts, status column, deviation and scenario names)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The Appendix is the raw evidence of one fresh set: every non-empty line of the 29 named scratch files appears in the record, and each of the 29 pasted blocks equals its file byte for byte (including the carriage returns of dpkg progress lines in caddy.txt)"
    requirement: DEP-02
    verification:
      - kind: command
        ref: "plan 21-06 task 1 automated verify 2: raw-blocks-match; and a block-for-block comparison of the 29 fenced blocks: 29 blocks, 0 differ"
        status: pass
    human_judgment: false
  - id: D3
    description: "No secret, cookie, token, password or AI key in any committed file of the plan: no key-shaped string, no 24-character hex word, no Set-Cookie, no CSRF cookie value, no password-hash prefix; the record names both tags and the pre-upgrade snapshot"
    requirement: DEP-03
    verification:
      - kind: command
        ref: "plan 21-06 task 1 automated verify 3 (secrets-scan-ok) and task 2 verify 3 over the README and docs/deployment.md (docs-ok); the 29 scratch files were scanned first"
        status: pass
    human_judgment: false
  - id: D4
    description: "deploy/test/README.md has a row for each of the 27 subcommands that also appear in the usage of run.sh, a row for each of the 18 variables of run.sh, every new output file, the section Phase 21: install, update, rollback and an updated Hand-over to phase 22; docs/deployment.md names /opt/holzcloud/deploy and no tracked file outside .planning and the record names the old top-level backup script path"
    requirement: DEP-01
    verification:
      - kind: command
        ref: "plan 21-06 task 2 automated verify 1 (readme-ok) and verify 3 (docs-ok)"
        status: pass
    human_judgment: false
  - id: D5
    description: "No operator-visible text changed and the repository gates are clean: go run ./tools/i18n 0 open 0 orphaned for every catalogue, gofmt -l empty, go vet, tools/english, tools/cites -check, tools/assembled -check, go test ./cmd/holzcloud -run 'Deploy|Caddyfile', bash -n deploy/test/run.sh; deploy/DEPLOY.md and deploy/holzcloud.service unchanged; deploy/test/out untracked"
    requirement: DEP-03
    verification:
      - kind: command
        ref: "plan 21-06 task 2 automated verify 2; go test -count=1 ./cmd/holzcloud -run 'Deploy|Caddyfile' ok; git diff --quiet -- deploy/DEPLOY.md deploy/holzcloud.service exits 0"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-10-12
status: complete
---

# Phase 21 Plan 06: The record finished, the hand-over written Summary

**`deploy/DEPLOY-RUN.md` is now the complete record of the phase: a Summary from the three `summary.txt` files, findings reconciled with the final evidence set (which added none), 16 container-versus-server rows labelled measured, inferred or assumed, a Not measured list, and an Appendix that pastes 29 scratch files byte for byte; `deploy/test/README.md` and `docs/deployment.md` now say what DEPLOY.md and `run.sh` say.**

## Performance

- **Duration:** about 8 min of tool time (no container was run; the evidence set of plan 21-05 was the input)
- **Completed:** 2026-10-12
- **Tasks:** 2
- **Files modified:** 3 (deploy/DEPLOY-RUN.md +1068, deploy/test/README.md +151/-, docs/deployment.md +49/-)

## Accomplishments

- **Summary and How this was run.** Three rows from the three `summary.txt` files: DEP-01 `fresh` (33 steps, 0 failed, `CADDY: validated`), DEP-02 `lifecycle` with v0.0.10 (`MIGRATIONS: old=58 new=60 pending=2`, 52 and 50 PASS, no BAD), DEP-03 in both cases (`MODE: snapshot` with 52 PASS against the pre-update state, `MODE: binary` with the v0.0.21 hop and 53 PASS against the state after the update). The text says that the DEP-02 row is the v0.0.10 run and the v0.0.21 run is the no-migration hop that belongs to DEP-03. Durations read from the file times (35 s, 75 s, about 60 s from `up` to the summary), the three identical `versions.txt` checked with `md5`, and the sentence that the set was made against the document of 2a11eeb, which has not changed since.
- **Findings.** All 20 rows reconciled mechanically: every row has its four fields, every backtick quote in the What happened column was found verbatim in the scratch output, every fix is visible in the current DEPLOY.md. The final set added nothing to the table, and the record says why: no failing step (the only status 3 is `systemctl status` right after the first start under the emulation), no BAD line, `LEAK-CHECK OK` three times.
- **DEP-01, DEP-02, DEP-03 sections.** Moved from the numbers and directory names of the intermediate passes to those of the final set (snapshot `pre-upgrade-v0.0.25-42-g2a11eeb-20261012-124115.sqlite`, backup `20261012-124114`, `amd64-lifecycle`, `amd64-lifecycle-hop`); the negative case, verbatim, is identical to the earlier pass. A paragraph on the final `fresh` run was added to DEP-01.
- **Deviations named.** The table of plan 21-02 was complete; added a table of which run shows which deviation (counted from the `deviations.txt` files) and what a real server does for both scenario steps; `SCENARIO[negative]: skipped, no migration lies between these builds (pending=0)` is quoted from the hop run.
- **Container vs. server, Not measured.** 16 rows: architecture and emulation, seccomp, the data directory mount, package repositories, Caddy install, TLS and ACME, First Run over plain http, the browser replaced by curl, two-factor enrolment, the systemd version, timing, free disk against 1024 MB, journal retention, the privileged container and cgroups, the backup timer, database size. Each ends in one of measured, inferred, assumed, with the three words defined before the table. Not measured lists native amd64, a real VPS image, certificate issuance, five or more updates and pruning, rollback of media and templates, single sign-on, a restore from a backup (phase 22), a reboot, two-factor enrolment itself, a failed migration, a production-size database.
- **Appendix.** R1 (9 files), R2 (15) and R3 (5) pasted by a script that copies without translating bytes. The first attempt used Python's default newline handling and the `grep -F` loop caught it: `caddy.txt` holds 16 dpkg progress lines that end in a carriage return, which the first paste had turned into plain line ends. Redone with `newline=''`; all 29 blocks now equal their file byte for byte.
- **README.** Rows for `bins install firstrun caddy postinstall baseline update negative rollback fresh lifecycle` and for `compare`; rows for `OLD_TAG NEW_REF BIN_DIR INSTALL_FROM DEPLOY_DOC WITH_OLD REBUILD`; the exit 3 of `caddy` and that `fresh` tolerates it; the statement that the proof starts from `systemd-base` through `install` and that `unit-test` stays the start of phase 22's populated site; 17 new output-file rows; the section `Phase 21: install, update, rollback` in place of the old hand-over (rerun commands, the document as source of truth through `doc_play` and `doc_cmd`, the headings the harness reads, no angle-bracket value and no carried variable, the wait done by the played line itself); the hand-over to phase 22 rewritten (whole deploy directory in `/opt/holzcloud/deploy`, data and backups owned by the service user, rsync in Prerequisites, `postinstall` already plays the Backups block, `restore.sh` is interactive and was not used for the rollback, the `holzcloud user create` route for a login user) with the statements the fixes made untrue removed; seven pitfalls and the deviations and scenario steps of the phase.
- **docs/deployment.md.** The Installing block now uses `/tmp` for the transfer, creates the service user first, installs the whole deploy directory to `/opt/holzcloud/deploy`, runs the backup script from there and uses the timer where the cron line was; pointers to DEPLOY.md and DEPLOY-RUN.md; the table of files and the unit paragraph are kept.

## Task Commits

1. **Task 1: complete deploy/DEPLOY-RUN.md from the fresh evidence set (tracer)** - `072a6db` (docs)
2. **Task 2: README hand-over for phases 22 and 23, docs/deployment.md alignment, repository gates** - `710a23a` (docs)

The tracer gate: Task 1's three automated verifies passed (`record-structure-ok`, `raw-blocks-match`, `secrets-scan-ok`) before Task 2 started.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The first paste of the appendix altered carriage returns**
- **Found during:** Task 1, the `grep -F` loop (`not in record (amd64-fresh/caddy.txt): Selecting previously unselected package apt-transport-https.`)
- **Issue:** `caddy.txt` has 16 lines ending in a carriage return (dpkg progress output); reading with Python's default universal newlines turned them into plain line ends, so the pasted block was not the file.
- **Fix:** the appendix was removed and written again with `newline=''` on both reads and writes. A block-for-block comparison (29 blocks, 0 differ) was added to the plan's line-by-line loop.
- **Files modified:** deploy/DEPLOY-RUN.md
- **Commit:** 072a6db

### Judgment calls beyond the plan text

- **The appendix was generated by a script**, not typed or edited in with the Write tool, because a paste of 29 files cannot be both byte-exact and retyped. The script lives in the scratchpad; its output is proven by the plan's loop and by the block comparison. The prose of the record was edited with scoped edits as the plan asks.
- **Carriage returns stay in the record** (16 lines of `caddy.txt`). The plan says byte for byte and its own loop would reject anything else; the lines render normally in a fenced block.
- **The commit column of the Summary names the build under test, 2a11eeb**, and says the record was made on top of 83b4348, instead of putting `git rev-parse --short HEAD` there: HEAD is a documentation commit and says nothing about what was run.
- **Findings were not extended.** The plan asks for a row for anything the final runs found; they found nothing (see Accomplishments), and the record says so in a paragraph, not by a row.
- **DEP-02 and DEP-03 prose was brought to the final set's numbers and directory names**, which the plan describes as reconciling; the Findings quotes were left as the output of the earlier passes, since they are the evidence of the defects.
- **README: `compare` got a row** although it is not one of the 27 the plan counts, because the usage text of `run.sh` lists it. The README also names `DEVIATION[prompt]` and `DEVIATION[preinstalled]`, which the harness knows and which did not occur.

**Total deviations:** 1 auto-fixed (Rule 1), 6 judgment calls. **Impact:** none on scope; `deploy/DEPLOY.md`, `deploy/holzcloud.service`, `run.sh`, the schema and all Go sources are unchanged (`git diff --quiet 83b4348 -- deploy/DEPLOY.md deploy/holzcloud.service deploy/test/run.sh cmd internal plugins sdk tools` exits 0).

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None: no endpoint, auth path, file access pattern or schema change. T-21-22: the pasted files are the named ones only (never `seed.txt`, a log, a `step-NN.txt` or a cookie jar), the 29 files were scanned before pasting and the committed documents after it, and each summary says `LEAK-CHECK OK`. T-21-23: every Container vs. server row carries one of measured, inferred, assumed, Not measured lists the gaps, the loop and the block comparison prove the appendix, and DEPLOY.md was not edited (`git diff --quiet` exits 0). T-21-24: `docs/deployment.md` is aligned and `git grep '/opt/holzcloud/backup.sh'` outside `.planning` and the record finds nothing.

## Issues Encountered

None open. The scratch directories under `deploy/test/out` stay gitignored (`git ls-files deploy/test/out` prints nothing).

## Next Phase Readiness

The phase is ready for verification. Phase 22 starts from the Hand-over section of `deploy/test/README.md` (the Backups block is already played by `postinstall`) and from the Not measured list of the record, which names the restore as its open item.

## Self-Check: PASSED

- FOUND: deploy/DEPLOY-RUN.md (ten `##` headings, 29 appendix blocks), deploy/test/README.md, docs/deployment.md, .planning/phases/21-deploy-update-rollback/21-06-SUMMARY.md
- FOUND: commits 072a6db and 710a23a (ancestors of HEAD); `git rev-list --count 83b4348..HEAD` was 2 before this summary
- Plan verifies re-run after the last commit: record-structure-ok, raw-blocks-match, secrets-scan-ok, readme-ok, docs-ok; go run ./tools/i18n shows 0 open, 0 orphaned in every catalogue
