---
phase: 21-deploy-update-rollback
plan: 01
subsystem: infra
tags: [bash, docker, systemd, deploy, harness, caddy, first-run]

requires:
  - phase: 20-hardened-unit-under-real-systemd
    provides: deploy/test/run.sh (build, up, start, exec, assert_http), the systemd-base image target, the container-only ProcSubset drop-in
provides:
  - "run.sh subcommands bins, install, firstrun, caddy, postinstall"
  - "doc_lines, doc_blocks, doc_preflight, doc_cmd, doc_play: the document is the only source of the played commands, and a line it does not contain is refused"
  - "bin_path and bins: binaries named by ref (holzcloud-new-SHORTSHA, holzcloud-old-OLD_TAG) built by the document's own Build line in a clone"
  - "FORM_SH and leak_check: headless First Run with no secret in OUT_DIR"
  - "raw pass-1 evidence of the unfixed DEPLOY.md under deploy/test/out/amd64-install-pass1 and amd64-install-pass1b"
affects: [21-02, 21-03, 21-04, 21-05, 21-06, 22-backup-restore]

actuals:
  tokens: 7720
  tasks: 2
  commits: 2
plan_head_before: 87d376d2ac270ff4d9e9f5a62df67f92e046db97
plan_head_after: 43a62afa874f89404f85843ef1e4b255832fab62

tech-stack:
  added: []
  patterns:
    - "Mirror with a drift guard: the harness reads every played command out of the document and dies on a line the document lacks"
    - "A named DEVIATION[...] line (screen and deviations.txt) for everything the container cannot do as written"
    - "Secrets stay in a root-only file inside the container; the host prints status codes and redirect paths only"

key-files:
  created: []
  modified:
    - deploy/test/run.sh

key-decisions:
  - "HEADING of doc_lines is the heading text without the hashes, matched as a plain string against a ## or ### line; a section ends at the next heading of the same or a higher level"
  - "The Build section is also played by install, where doc_cmd records it as SKIP[dev-machine], so the step list shows that the line was run by bins from the same text"
  - "A line with apt install and no -y is played with y on stdin and named DEVIATION[prompt]; otherwise the Caddy block would end at a question nobody can answer in a container"
  - "firstrun needs no explicit Cookie header: Debian's curl 7.88.1 sends the Secure CSRF cookie to the http loopback URL (assumption A1 settled)"
  - "install writes DEVIATION[preinstalled] once: the image ships sudo, curl, jq, sqlite3 and openssl, the document names only sqlite3"

patterns-established:
  - "OUT_DIR/step-NN.txt holds the command and the output of every played step; steps.txt holds one STEP nn rc=R LINE per step"
  - "Exit 3 from caddy means: ran, but the network or a missing prerequisite prevented the evidence"

requirements-completed: [DEP-01]

coverage:
  - id: D1
    description: "run.sh install plays Build, Transfer, Install and systemd Service out of DEPLOY.md from TARGET=systemd-base, refuses a document without the sections before docker is touched, and ends with an active unit and /healthz and /readyz ok"
    requirement: DEP-01
    verification:
      - kind: integration
        ref: "PLATFORM=linux/amd64 TARGET=systemd-base run.sh build up install (INSTALL OK, 16 STEP lines, no counted rc!=0, integrity ok)"
        status: pass
      - kind: integration
        ref: "DEPLOY_DOC=deploy/test/out/empty-doc.md run.sh install exits 1, names the file, creates no container"
        status: pass
    human_judgment: false
  - id: D2
    description: "firstrun completes the First Run section headlessly: setup form 200, setup POST 303, setup closed 404, login 303, compulsory two-factor redirect; no secret reaches OUT_DIR"
    requirement: DEP-01
    verification:
      - kind: integration
        ref: "run.sh build up install firstrun (FIRSTRUN OK, five PASS lines, LEAK-CHECK OK, generated password absent from every file and log)"
        status: pass
    human_judgment: false
  - id: D3
    description: "caddy and postinstall detect where the unfixed document and the installed system differ (caddy not installable by the document, deploy/backup.sh and restore.sh missing, backup block failing)"
    requirement: DEP-01
    verification:
      - kind: integration
        ref: "run.sh caddy exits 3 with a CADDY: verdict; run.sh postinstall exits 1 with BAD missing /opt/holzcloud/deploy/backup.sh"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-10-12
status: complete
---

# Phase 21 Plan 01: Play DEPLOY.md literally, pass 1 Summary

**A document-mirroring harness in deploy/test/run.sh that plays DEPLOY.md's install, First Run, Caddy and Backups blocks in a bare systemd container, refuses any command the document does not contain, and leaves the raw evidence of what the unfixed document gets wrong.**

## Performance

- **Duration:** 8 min of tool time (builds and emulated container runs included)
- **Started:** 2026-10-12T11:47:22Z
- **Completed:** 2026-10-12T11:56:18Z
- **Tasks:** 2
- **Files modified:** 1 (deploy/test/run.sh, +656 lines)

## Accomplishments

- `doc_awk`/`doc_lines`/`doc_blocks`/`doc_preflight` read the fenced bash blocks of a named `##` or `###` section out of `DEPLOY_DOC` (POSIX awk, bash 3.2 safe, backslash continuations joined, comments and blank lines dropped, plain-string heading comparison). `doc_cmd` is the only place a command runs: it dies when the line is not in the document, dies on an angle-bracket placeholder, and classifies `scp` (docker cp or a tar stream), the `go build` line (SKIP[dev-machine]), `systemctl reload caddy` (caddy validate), and `apt install` without `-y` (answer supplied). Every step lands in `steps.txt` as `STEP nn rc=R LINE`, with its output in `step-NN.txt`.
- `bins` builds `NEW_REF` (and `OLD_TAG` with `WITH_OLD=1`) from a `git clone --no-checkout` plus checkout of the resolved sha, by playing the document's own Build line with `bash -c`; files are named by ref under `deploy/test/out/bin`, with sha256, describe and migration count recorded in `bins.txt`. Assumption A6 holds: `v0.0.10` (58 migrations) cross-compiles with the document's line exactly as HEAD (60) does.
- `install` on `TARGET=systemd-base`, linux/amd64: all counted steps rc 0, the literal `systemctl start` crashes as in Phase 20, `cmd_start` applies the container-only drop-in, then `active`, `/healthz` and `/readyz` ok (`status ok`, `integrity ok`; version `dev`, commit `unknown` because the document's Build line carries no `-X` flags).
- `firstrun` completes headlessly with five PASS lines: GET /admin/setup 200, setup POST 303 /admin/, GET /admin/setup 404, login POST 303 /admin/, GET /admin/ 303 /admin/2fa/einrichten. `leak_check` found no secret.
- `caddy` and `postinstall` exit 3 and 1 on the unfixed document, which is the proof that the detectors can fail.

## Pass-1 evidence for plan 21-02 (raw, nothing fixed)

Scratch output (gitignored) in `deploy/test/out/amd64-install-pass1` (`install`) and `amd64-install-pass1b` (`install firstrun`, `caddy`, `postinstall`), with `*.log`, `-prereq.log`, `-caddy.log`, `-post.log`. What the document said against what happened:

| Where | The document says | What happened |
|-------|-------------------|---------------|
| Build | one `go build` line, no `-X` | binary reports version `dev`, commit `unknown`; `/readyz` cannot tell old from new |
| Install | `sudo cp /tmp/deploy/backup.sh /opt/holzcloud/backup.sh` | works, but Backups and `holzcloud-backup.service` use `/opt/holzcloud/deploy/backup.sh`, which the install never creates; `restore.sh` is never installed either (paths.txt: `BAD missing` for both) |
| Install | `mkdir -p /opt/holzcloud/backups`, no chown | backup unit runs as `holzcloud` and writes there (not exercised in pass 1; to be settled in 21-02) |
| systemd Service | `systemctl start`, then `status` | start returns 0, process dies 8 to 10 s later under emulation (known Phase 20 crash, DEVIATION[dropin]); `systemctl status` exit code 3 is not a failure |
| Prerequisites | `sudo apt install sqlite3` | prints "already the newest version" (image pre-installs it; DEVIATION[preinstalled]); the document names no rsync, gpg or fuser, which the image lacks (TOOL lines) |
| Caddy block | `apt install -y debian-keyring ...` first | `E: Unable to locate package debian-keyring` (rc 100): no `apt update` before the first install on a bare server |
| Caddy block | `curl ... gpg.key \| sudo gpg --dearmor ...` | `sudo: gpg: command not found`; the repository then fails `apt update` with NO_PUBKEY, caddy is never installed, `/etc/caddy` does not exist, `cp` of the Caddyfile fails; CADDY: not run |
| Backups | `sudo cp deploy/holzcloud-backup.{service,timer} /etc/systemd/system/` | the relative `deploy/` resolves against the working directory, and the document put the files in `/tmp/deploy`: rc 1, then `enable --now holzcloud-backup.timer` rc 1 and the manual `backup.sh` run rc 1 |
| First Run | "Visit https://your-domain/admin" | nothing says the shipped unit's `HOLZCLOUD_SECURE=true` makes the admin work only through the TLS proxy; after the setup form the next request goes to the compulsory two-factor enrolment, which the section does not mention |

## Task Commits

1. **Task 1: install from systemd-base out of the document** - `efee0ac` (feat)
2. **Task 2: First Run over curl, Caddy validation, post-install audit** - `43a62af` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] macOS tar xattr broke the directory scp translation**
- **Found during:** Task 1, first install run
- **Issue:** `docker cp -` of the tar stream failed with `lsetxattr /tmp/deploy: xattr "com.apple.provenance": operation not supported`; `COPYFILE_DISABLE=1` alone does not keep that attribute out of the archive
- **Fix:** `tar --no-xattrs --no-acls --no-mac-metadata` (bsdtar 3.5.3), with `COPYFILE_DISABLE=1` kept
- **Files modified:** deploy/test/run.sh
- **Commit:** efee0ac

### Judgment calls beyond the plan text (no deviation from a stated requirement)

- `install` also plays the Build section, so the `SKIP[dev-machine]` step is a visible, numbered record (this makes 16 STEP lines, the plan asked for at least 15).
- `DEVIATION[prompt]` (apt without `-y` is answered with `y` on stdin) and a generic `DEVIATION[network]` detector inside `doc_cmd` were added; the plan named the network deviation but not the prompt.
- `DEVIATION[preinstalled]` is printed once by `install` to make the pre-installed tools explicit evidence.
- `FORM_SH` is assigned with `read -r -d ''` and a quoted heredoc instead of a single-quoted string, because its sed expressions need single quotes and bash 3.2 mishandles quotes in a heredoc inside `$(...)`.
- The usage, header and README-facing interface: `deploy/test/README.md` was not touched (not in this plan's `files_modified`); the new subcommands and variables are documented in the run.sh header and usage text. README rows belong to a later plan of this phase.

**Total deviations:** 1 auto-fixed (Rule 3). **Impact:** none on scope.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None: the plan adds a test harness only; no endpoint, auth path or schema change.

## Issues Encountered

None open. `deploy/DEPLOY.md` and `deploy/holzcloud.service` are unchanged (`git diff --quiet` exits 0); no schema was pushed or changed.

## Next Phase Readiness

Ready for 21-02: the evidence table above is the input for the DEPLOY.md fixes (Build `-X` stamping, Prerequisites with rsync, gpg and psmisc, apt update before the first apt install, one consistent `/opt/holzcloud/deploy` path with a chown of `backups`, First Run note on `HOLZCLOUD_SECURE` and two-factor enrolment), and `run.sh install firstrun caddy postinstall` is the replay for the second pass. The built binaries are cached in `deploy/test/out/bin` (`WITH_OLD=1 run.sh bins` adds `v0.0.10`).

## Self-Check: PASSED

- FOUND: deploy/test/run.sh (functions doc_lines, doc_blocks, doc_preflight, doc_cmd, doc_play, bin_path, cmd_bins, cmd_install, cmd_firstrun, cmd_caddy, cmd_postinstall, leak_check, constant FORM_SH)
- FOUND: commit efee0ac and commit 43a62af (ancestors of HEAD)
- FOUND: deploy/test/out/amd64-install-pass1/steps.txt and amd64-install-pass1b/{firstrun,caddy,paths}.txt
- `bash -n` passes with bash and /bin/bash; guard run exits 1 and names the document; deploy/DEPLOY.md unchanged
