---
phase: 21-deploy-update-rollback
plan: 02
subsystem: infra
tags: [deploy, docs, bash, docker, systemd, caddy, update, migrations]

requires:
  - phase: 21-deploy-update-rollback
    provides: "run.sh bins, install, firstrun, caddy, postinstall and the raw pass-1 evidence of the unfixed DEPLOY.md (plan 21-01)"
provides:
  - "deploy/DEPLOY.md repaired on the install side: Prerequisites block, stamped Build line, deploy directory at /opt/holzcloud/deploy, backups owned by the service user, First Run with TLS proxy, open setup form and compulsory two-factor, Caddy block with apt update and gnupg"
  - "deploy/DEPLOY-RUN.md: Summary, How this was run, Deviations named, Findings (ten DEP-01 rows with verbatim pass-1 output) and the DEP-01 section"
  - "run.sh baseline and update: an old-tag (v0.0.10) install seeded with real content, then the Updating section played as shipped, with update.txt, state-s0.txt, state-s1.txt"
  - "pass-1 update evidence for plan 21-03: deploy/test/out/amd64-update-pass1"
affects: [21-03, 21-04, 21-05, 21-06, 22-backup-restore]

actuals:
  tokens: 8900
  tasks: 2
  commits: 2
plan_head_before: 58ae902145a39cfcf58ca0f304b58de7965f7156
plan_head_after: 6bb12e475ec0ba6f37e11a3181e889e23d93b76b

tech-stack:
  added: []
  patterns:
    - "Repair the document, then replay it: the fix is proven by a second play from a fresh container, never by reading"
    - "A finding quotes the decisive output verbatim and a grep -F loop proves the quote exists in the scratch files"
    - "State is captured with counts only through sqlite3 -readonly as the service user; no holzcloud subcommand touches a database that the other binary owns"

key-files:
  created:
    - deploy/DEPLOY-RUN.md
  modified:
    - deploy/DEPLOY.md
    - deploy/test/run.sh

key-decisions:
  - "Install copies the whole transferred deploy directory to /opt/holzcloud/deploy (the path the Backups section, the backup unit and restore already used) instead of one script to /opt/holzcloud/backup.sh"
  - "DEVIATION[preinstalled] is emitted by install only for a tool of the image that the Prerequisites block does not name; with the repaired document it no longer appears, so the named set is exactly scp, dropin, browser, caddy, mount (network only when it happens)"
  - "update.txt carries one extra key, APPLIED (count of 'migration applied' journal lines), next to the keys the plan names; MIGRATIONS pending is new minus old from the .info files of the two built binaries"
  - "Old and new binaries are named by ref and carry the stamped version of the document's own Build line (v0.0.10 and v0.0.25-30-gb5b0393); the old binary built unstamped in plan 21-01 was removed and rebuilt"

patterns-established:
  - "OUT_DIR/update.txt is KEY: value lines: OLD, NEW, MIGRATIONS, APPLIED, SNAPSHOT, SCHEMA, PREV, BACKUP and repeated JOURNAL lines"
  - "OUT_DIR/state-LABEL.txt is identity, schema and one 'table NAME N' line per sqlite_master table"

requirements-completed: [DEP-01, DEP-02]

coverage:
  - id: D1
    description: "DEPLOY.md played literally from a bare systemd container installs a healthy, set-up service; every pass-1 deviation is fixed in the document and recorded in DEPLOY-RUN.md with its evidence"
    requirement: DEP-01
    verification:
      - kind: integration
        ref: "run.sh build up install firstrun, postinstall, caddy from fresh containers: INSTALL OK, FIRSTRUN OK, POSTINSTALL OK, CADDY: validated; steps.txt no failing counted step; deviations.txt only scp, dropin, browser, caddy, mount; /readyz version v0.0.25-29-g58ae902, not dev; paths.txt no BAD"
        status: pass
      - kind: unit
        ref: "go test ./cmd/holzcloud -run 'Deploy|Caddyfile'"
        status: pass
      - kind: command
        ref: "grep -F loop over the 19 backtick quotes of the DEP-01 findings rows against the pass-1 scratch files: none missing"
        status: pass
    human_judgment: false
  - id: D2
    description: "An installation of v0.0.10 is built through the repaired document, seeded with a website, domain, page, five plugins, an image, a login user and an order row, and the Updating section played as shipped moves it to the new build with a pre-upgrade snapshot and schema 58 to 60"
    requirement: DEP-02
    verification:
      - kind: integration
        ref: "run.sh build up install baseline update with INSTALL_FROM=old: BASELINE OK, UPDATE OK, LEAK-CHECK OK; update.txt SNAPSHOT under /opt/holzcloud/data/pre-upgrade-, MIGRATIONS old=58 new=60 pending=2, SCHEMA 58 -> 60, OLD v0.0.10 ec365d6, NEW v0.0.25-30-gb5b0393 b5b0393"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-10-12
status: complete
---

# Phase 21 Plan 02: DEPLOY.md repaired from the evidence, old install updated as shipped Summary

**The install side of deploy/DEPLOY.md is repaired from ten pass-1 findings and proven by a second play from fresh containers; DEPLOY-RUN.md records each finding with the verbatim output; run.sh now seeds a v0.0.10 install and plays the unedited Updating section against it (snapshot written, schema 58 to 60).**

## Performance

- **Duration:** about 10 min of tool time (builds and emulated container runs included)
- **Started:** about 2026-10-12T11:57Z
- **Completed:** 2026-10-12T12:08Z
- **Tasks:** 2
- **Files modified:** 3 (deploy/DEPLOY.md, deploy/DEPLOY-RUN.md new, deploy/test/run.sh)

## Accomplishments

- **Prerequisites** is now a fenced block (`sudo apt update`, one `apt install -y` line for ca-certificates, curl, jq, sqlite3, rsync, psmisc, gnupg, openssl) with a reason per package; the Go version is the one on the `go` line of go.mod (1.26.8, read, not remembered); `install` plays the block first.
- **Build** stamps `-X main.Version` and `-X main.Commit`, so `/readyz` and the pre-upgrade snapshot name tell builds apart (pass 2: `v0.0.25-29-g58ae902`, not `dev`).
- **Install** copies the whole deploy directory to `/opt/holzcloud/deploy`, makes `backup.sh` and `restore.sh` executable, and chowns data and backups to the service user (backups mode 700). **Backups** copies the unit and timer from that directory.
- **First Run** says that the shipped unit works only through the TLS proxy (a form posted to port 8080 over plain HTTP answers `403 Forbidden - CSRF validation failed`, measured), that the setup form belongs to the first visitor until an account exists (404 afterwards), and that two-factor enrolment follows the first login and is compulsory; its verification commands are a fenced block that `firstrun` plays. **Caddy** updates the package list first and names gnupg and curl; the final install carries `-y`. **Troubleshooting** names the package behind `fuser`.
- **Pass 2** from fresh containers (verified twice): `build up install firstrun` INSTALL OK and FIRSTRUN OK, `postinstall` POSTINSTALL OK, `caddy` CADDY: validated (Caddy 2.11.7, `Valid configuration`), no failing counted step, deviations only scp, dropin, browser, caddy, mount, no BAD in paths.txt. `deploy/holzcloud.service` unchanged.
- **DEPLOY-RUN.md** holds the five headings in order, a Deviations named table with six entries and ten DEP-01 findings rows; all 19 backtick quotes were proven present in the pass-1 scratch files by the grep -F loop. Two changes that no pass-1 line showed (backups ownership, ca-certificates and openssl) are recorded as reasoned, confirmed only by pass 2.
- **baseline and update** (run.sh): `stage_fixtures`, `insert_old_order`, `capture_state`, `cmd_baseline`, `cmd_update`. The old install is itself played out of the repaired document (`INSTALL_FROM=old`), the new build is HEAD, both stamped by the document's own Build line; S0 holds website Test, domain test.local, the published page plugintest, five plugins, one uploaded image, an admin login user (password generated in the container, on stdin) and one order row inserted directly in the old code's statement shape.

## Pass-1 update evidence (input for plan 21-03)

Scratch output in `deploy/test/out/amd64-update-pass1` (`update.txt`, `state-s0.txt`, `state-s1.txt`, `steps.txt`, `deviations.txt`, `baseline.txt`; log `amd64-update-pass1.log`). The Updating section was played exactly as it stands and every counted step returned 0:

| Fact | Value |
|------|-------|
| OLD / NEW | `v0.0.10 ec365d6` / `v0.0.25-30-gb5b0393 b5b0393` |
| Migration files / applied | old=58, new=60, pending=2; journal shows migration 59 and 60 applied |
| Snapshot | `/opt/holzcloud/data/pre-upgrade-v0.0.25-30-gb5b0393-20261012-120647.sqlite`, written by the service before the migrations |
| Schema | 58 to 60; tables 53 to 54 (website_mail new); row counts of the shared tables unchanged, the order row survived |
| What the section leaves out | no backup step before the stop (only the sentence "back up first"); no copy of the old binary (`PREV: absent`); no check after the start (no `/readyz`, no version comparison); the Build line is unstamped; no statement that the snapshot lands in the data directory and that `PruneSnapshots` keeps five; no way back |

## Task Commits

1. **Task 1: repair the install from the evidence, record DEP-01, prove it (pass 2)** - `b5b0393` (fix)
2. **Task 2: baseline and update, the Updating section played as shipped** - `6bb12e4` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] postinstall reported a false BAD for a brace-expanded path**
- **Found during:** Task 1, first pass-2 `postinstall`
- **Issue:** the path audit read `/opt/holzcloud/deploy/holzcloud-backup.{service,timer}` out of the Backups block as one literal path and reported it missing although both files existed
- **Fix:** the audit expands a brace name of the plain shape the way the shell that plays the line does, and tests each file
- **Files modified:** deploy/test/run.sh
- **Commit:** b5b0393

**2. [Rule 3 - Blocking] DEVIATION[preinstalled] would have failed the plan's own verify**
- **Found during:** Task 1, before pass 2
- **Issue:** `install` printed DEVIATION[preinstalled] unconditionally, and the verify allows only scp, dropin, browser, caddy, mount and network in deviations.txt
- **Fix:** `install` now plays the Prerequisites block and emits the deviation only for a tool of the image (ca-certificates, curl, jq, sqlite3, openssl) that the block does not name. With the repaired document none is missing, so the line is absent. This is a consequence of finding 1 of the findings table, not a way of hiding it: pass 1 still shows the deviation in its own files
- **Files modified:** deploy/test/run.sh
- **Commit:** b5b0393

**3. [Rule 1 - Bug] First Run section did not name "two-factor"**
- **Found during:** Task 1, second verify
- **Issue:** the first wording said "second factor"; the acceptance check and the plan name two-factor
- **Fix:** reworded in DEPLOY.md and DEPLOY-RUN.md, then pass 2 replayed (the played block was unchanged)
- **Commit:** b5b0393

### Judgment calls beyond the plan text

- Findings has ten rows, not the nine the first draft of the Summary said; the Summary text was corrected.
- `update.txt` has the additional key `APPLIED`; `PREV`, `BACKUP`, `SNAPSHOT`, `SCHEMA`, `MIGRATIONS`, `OLD`, `NEW` and `JOURNAL` are as specified (JOURNAL is one line per matching journal entry).
- `deploy/test/README.md` was not touched (not in this plan's `files_modified`); the new subcommands are in the run.sh header and usage text.
- Assumption A7 (seed on the old binary) held: `seed` ran on v0.0.10 and OLD_TAG stayed v0.0.10; no rerun at v0.0.20 was needed.

**Total deviations:** 3 auto-fixed (2 Rule 1, 1 Rule 3). **Impact:** none on scope.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None: no endpoint, auth path or schema change. No schema was pushed or edited; migrations 00059 and 00060 were played as they are against a real database. The new binary never touched the old database through a holzcloud subcommand; the harness reads with `sqlite3 -readonly` as the service user and the only CLI calls on the old binary were `ai key create` (inside `seed`) and `user create`. `LEAK-CHECK OK` held after baseline and update, and no AI-key-shaped string is in either document.

## Issues Encountered

None open. `deploy/holzcloud.service` is unchanged. The Updating section and `Rolling back` are untouched on purpose (plans 21-03 and 21-04 own them).

## Next Phase Readiness

Ready for 21-03: the pass-1 update evidence above is the input for repairing the Updating section, and `run.sh build up install baseline update` with `INSTALL_FROM=old` is the replay for pass 2. `capture_state` is a first version (identity, schema, table counts); 21-03 completes it.

## Self-Check: PASSED

- FOUND: deploy/DEPLOY-RUN.md, deploy/DEPLOY.md, deploy/test/run.sh (functions stage_fixtures, insert_old_order, capture_state, cmd_baseline, cmd_update)
- FOUND: commits b5b0393 and 6bb12e4 (ancestors of HEAD)
- FOUND: deploy/test/out/amd64-install/steps.txt, deploy/test/out/amd64-update-pass1/update.txt, state-s0.txt, state-s1.txt
- `bash -n` passes with bash and /bin/bash; `go test ./cmd/holzcloud -run 'Deploy|Caddyfile'` passes; `git diff --quiet -- deploy/holzcloud.service` exits 0
