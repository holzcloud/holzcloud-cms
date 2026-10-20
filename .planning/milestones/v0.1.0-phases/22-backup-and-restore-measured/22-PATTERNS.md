# Phase 22: Backup and restore, measured - Pattern Map

**Mapped:** 2026-10-14
**Files analyzed:** 9 (all analogs git-tracked, checked with `git ls-files` for deploy/*)
**Analogs found:** 9 / 9 (this is a shell/Markdown phase; no Go files expected)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `deploy/test/run.sh` (new subcommands: backup proof, timer, transfer, restore, negative, chain) | harness/test | batch, request-response (docker exec) | `deploy/test/run.sh` `cmd_postinstall` (1411-1474), `cmd_negative` (2240-2300), `cmd_lifecycle` (2396-2448) | exact |
| `deploy/test/run.sh` (state compare across two containers) | harness/test | transform | `capture_state` (1590-1621), `compare_states` (1657+) | exact |
| `deploy/test/run.sh` (leak check on target) | harness/test | file-I/O | `leak_check` (1274-1292) | exact |
| `deploy/backup.sh` (fix: add `plugins sprachen marke`) | script | file-I/O | itself, lines 57-67 (rsync blocks) | exact |
| `deploy/restore.sh` (mirror fix) | script | file-I/O | itself, lines 73-80 | exact |
| `deploy/DEPLOY.md` (Backups / Restore section) | doc | n/a | DEPLOY.md Backups block (played by `doc_play "Backups" 1`) | exact |
| `deploy/BACKUP-RUN.md` (record) | doc | n/a | `deploy/DEPLOY-RUN.md` (summary table, "How this was run", Appendix, `grep -F` sentence at line 481) | exact |
| `deploy/test/README.md` (subcommands, env vars, Phase 22 section) | doc | n/a | itself ("Hand-over to phase 22", Pitfalls) | exact |
| timer drop-in `/etc/systemd/system/holzcloud-backup.timer.d/container.conf` (container only, created by harness) | config | event-driven | `DEVIATION[dropin]` for `holzcloud.service.d/container.conf` in `cmd_install` (run.sh 1177) | role-match |
| `docs/deployment.md`, `docs/configuration.md` (correct "whole directory" claim) | doc | n/a | the lines themselves (deployment.md:118, configuration.md:130-131) | exact |

## Pattern Assignments

### `deploy/test/run.sh` new subcommands (harness, batch)

**Analog:** `cmd_postinstall`, `cmd_negative`, `cmd_lifecycle`.

**Helpers** (run.sh:89-90, 1269): all container access goes through
```bash
dx() { docker exec "$NAME" "$@"; }
dxi() { docker exec -i "$NAME" "$@"; }
form() { printf '%s\n' "$FORM_SH" | dxi bash -s -- "$@"; }
```
Two containers: switch the globals `NAME` / `OUT_DIR` per phase (or add documented `TARGET_NAME`/`TARGET_OUT_DIR`). Bash 3.2 only (no arrays, `mapfile`, `${v,,}`); GNU tools (`date +%s.%N`, `du -sb`, `sha256sum`) run inside the container via `dx`/`dxi`, never on the macOS host.

**Subcommand preamble pattern** (run.sh:1413-1416, 2243-2245):
```bash
dx test -e /opt/holzcloud/holzcloud || die "nothing is installed in container $NAME; run 'run.sh build up install' first"
mkdir -p "$OUT_DIR"
n0=$(steps_count)
doc_play "Backups" 1 || true
failed=$(steps_failed_since "$n0")
```
and precondition gating on a previous output file: `grep -q '^MARKER: ok' "$OUT_DIR/update.txt" || die "... run 'run.sh update' first"`.

**Result files pattern** (run.sh:2277-2288): `: > "$OUT_DIR/negative.txt"` then `echo "KEY: value" >> file`; later chains read them back with `sed -n 's/^KEY: //p' file | head -n 1`. Use the same `KEY: value` shape for backup.txt / timer.txt / restore.txt so summaries and the `grep -F` proof work.

**Scenario/deviation reporting:** `scenario "SCENARIO[negative]: ..."` (2257) and `deviation "DEVIATION[name]: ..."` (e.g. 271, 1489). New deviations needed: `DEVIATION[transfer]` (tar stream via host instead of rsync/scp), `DEVIATION[timer]` (drop-in), `DEVIATION[fixture]` (bulk media, `sprachen/`/`marke/` plants, exec-pipe of `/root/.hc-*`), `DEVIATION[backup-name]` (sample timestamp in the DEPLOY.md restore line). The known-deviation vocabulary check fails on an unknown name: add new names where the set is defined and to README.

**Doc-driven play:** `doc_cmd` (230-238) refuses a line not in DEPLOY.md and any `<placeholder>`; the restore line has a sample timestamp so a named translation (like `scp`, 262-280) is needed.

**Chain pattern** (cmd_fresh 2369-2392 / cmd_lifecycle 2396-2448): `chain_need_base NAME; CLEANUP_DOWN=1; cmd_build; cmd_up; cmd_install; ...; leak_check; { echo "---- summary ($PLATFORM) ----"; ...; echo "LEAK-CHECK OK"; } > "$OUT_DIR/summary.txt"; cat summary; die on failed requirement`. Register new names in the dispatch `case` (run.sh:2461) and usage; `on_exit` removes only `$NAME`, so a two-container chain must remove both unless `KEEP=1`.

**Polling, not sleeping** (run.sh:2263-2275): `while [ "$i" -le "$READY_TIMEOUT" ]; do dx curl -fsS --max-time 2 .../healthz ... && break; sleep 1; i=$((i+1)); done`. Use for the timer fire (poll `ExecMainStartTimestamp`) too.

**Timer drop-in snippet** (RESEARCH Pattern 5, probed): empty `OnCalendar=` first, then the time, `RandomizedDelaySec=0`, `AccuracySec=1s`, `daemon-reload` only (no timer restart); evidence = `TriggeredBy=holzcloud-backup.timer` + journal + `=== Backup complete: DIR ===` line.

**Source chain:** `INSTALL_FROM=old NEW_REF=<sha> build up install baseline update postinstall` (postinstall is required: installs and enables the units); target: `build up install` with its own `NAME`/`OUT_DIR`, sharing `BIN_DIR`.

### State comparison across containers

**Analog:** `capture_state` / `compare_states` (run.sh:1590-1621, 1657+), `st_get`/`st_count`, volatile-table list `is_volatile` (1631-1642: sessions, activity_log, mail_outbox, outbox, plugin_store, goose_db_version, oauth_codes). A difference in a non-volatile table is BAD, never a new entry. `capture_state` needs in the container: `/root/.hc-login-pass`, `/root/.hc-key`, `/tmp/hc-upload.jpg`, `order_number`. Carry them by `docker exec SRC cat F | docker exec -i TGT bash -c 'umask 077; cat > F'` (never via a host file). Comparison functions emit `PASS`/`INFO`/`BAD` through `cmp_pass/cmp_info/cmp_bad` (1651-1653); reuse them for the whole-tree hash diff.

### Leak check

**Analog:** `leak_check` (run.sh:1274-1292): greps `OUT_DIR` for the contents of `/root/.hc-admin-pass`, `/root/.hc-login-pass`, `/root/.hc-key` and `hc_[A-Za-z0-9_-]{20,}`; ends `say "LEAK-CHECK OK"`. Call it at the end of every new chain, once per container `OUT_DIR`.

### `deploy/backup.sh` (fix, D-16, only if pass 1 confirms the defect)

**Analog:** its own blocks (backup.sh:57-67):
```bash
if [ -d "${DATA_DIR}/media" ]; then
    echo "Backing up media..."
    rsync -a "${DATA_DIR}/media/" "${BACKUP_DIR}/media/"
fi
if [ -d "${DATA_DIR}/templates" ]; then
    echo "Backing up templates..."
    rsync -a "${DATA_DIR}/templates/" "${BACKUP_DIR}/templates/"
fi
```
Extend as a `for d in media templates plugins sprachen marke; do if [ -d "${DATA_DIR}/$d" ]; then echo "Backing up $d..."; rsync -a ...; fi; done` loop. Keep top-level names so older backups still restore; keep `set -euo pipefail` and `echo` progress style; no new options or MANIFEST fields (D-10). Header comment should say what is deliberately not backed up (`pre-upgrade-*.sqlite`).

### `deploy/restore.sh` (mirror fix)

**Analog:** restore.sh:73-80 (same `[ -d "${BACKUP_DIR}/X" ]` guards with `echo "Restoring X..."`). Same loop. Keep the verify-before-touch order, which the negative cases depend on:
```bash
echo "Verifying snapshot..."
VERIFY_DIR=$(mktemp -d); trap 'rm -rf "${VERIFY_DIR}"' EXIT
cp "${BACKUP_DIR}/holzcloud.sqlite" "${VERIFY_DIR}/holzcloud.sqlite"
if ! HOLZCLOUD_DATA_DIR="${VERIFY_DIR}" "${HOLZCLOUD_BIN}" check; then ... exit 1; fi
read -r -p "Stop ${SERVICE} and replace ${DATA_DIR}? [yes/NO] " confirm
[ "${confirm}" = "yes" ] || { echo "aborted"; exit 1; }
systemctl stop "${SERVICE}"   # then mv aside, mkdir, copy, chown -R, chmod 700, start, poll /readyz 30x
```
Progress lines usable for the timing split: `Verifying snapshot...`, `Stopping...`, `Restoring database...`, `Starting...`, `Waiting for readiness...`, `=== Restore complete ===`. `read -p` prints no prompt on non-tty stdin (state once in the record).

### `deploy/BACKUP-RUN.md`

**Analog:** `deploy/DEPLOY-RUN.md`: title + purpose paragraph, `## Summary` table (Requirement | Result | Platform | Date, commit), `## How this was run` with the literal commands (pass 1 / final set), Findings, Container vs. server, Not measured, Appendix pasted unchanged; proof sentence at line 481 ("A line-by-line `grep -F` of every non-empty line of the N files against this document found no line missing"). Deviations (transfer, timer drop-in, emulation, fixture) listed first (D-18). Do not edit DEPLOY.md between final chains and the record. Proof loop (RESEARCH): `while IFS= read -r l; do [ -z "$l" ] || grep -Fxq -- "$l" deploy/BACKUP-RUN.md || echo MISSING; done < file`.

### `deploy/test/README.md`, `deploy/DEPLOY.md`, `docs/*.md`

README: extend the subcommand table and env vars, add a Phase 22 section; never rename existing interface ("extend it, do not rename it", README:8). DEPLOY.md: English, matches the existing Backups block; the played blocks must contain no angle-bracket placeholders. `docs/deployment.md:118` and `docs/configuration.md:130-131` currently claim the script backs up the "whole directory"; must agree with the final script, and the configuration.md tree lacks `plugins/` and `marke/`.

## Shared Patterns

### Evidence discipline
**Source:** `run.sh` `say`, `die`, `deviation`, `scenario`, `leak_check`. Raw logs in `deploy/test/out/` (gitignored, `.gitignore:33`), committed record pastes them. Names `step-NN.txt`, `STEP nn rc=R`.

### Execution identity
CLI as service user with `HOLZCLOUD_DATA_DIR` set (`runuser -u holzcloud -- ...`, e.g. run.sh:1600, 2289); `sqlite3` readers use `file:PATH?immutable=1` for the WAL-mode backup (never open the backup read-write). Do not run a new binary's `check`/`backup` against an old DB to "look" (migrates silently).

### Whole-tree hash (content proof)
`( cd DIR && find . -type f ! -name 'holzcloud.sqlite*' ! -name 'pre-upgrade-*.sqlite' ! -name .readyz ! -name MANIFEST -print0 | sort -z | xargs -0 sha256sum )` on source data dir, backup dir and restored data dir; assert source non-empty and containing `./media/`. Covers `plugins/`, `sprachen/`, `marke/` (RESEARCH Pitfall 1), not just what the script copies.

### Pin the binary
Use `NEW_REF=<sha>` (not `HEAD`) for both containers so the binary is identical even if scripts are committed mid-phase.

### Pass structure
Pass 1 against unchanged scripts (expect plugin/`sprachen`/`marke` gap), fix minimally, then a final evidence set from fresh containers (Phase 21 pattern).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| container-to-container tar stream (`docker cp SRC:dir - \| docker cp - TGT:dest`) | harness | streaming | Only host-to-container `scp` translation exists (run.sh:271-279); use that `docker cp -` tar-stream idiom as the nearest reference |
| per-line timestamped timing of `restore.sh` and bulk-media fixture | harness | streaming | No timing code in run.sh; use RESEARCH Pattern 2 and the bulk fixture snippet (website id from `select id from websites order by id limit 1`, as `insert_old_order` run.sh:1518+ does) |
| negative cases on `restore.sh` (damaged DB, missing sqlite, answer "no") | harness | request-response | Nearest structure is `cmd_negative` (result file + `NEGATIVE-RESULT:` line); contents follow RESEARCH Pitfall 3 (truncate to half; hash data dir before/after; service still active) |

## Metadata

**Analog search scope:** `deploy/` (tracked files), `deploy/test/run.sh` by function index.
**Files scanned:** backup.sh, restore.sh, run.sh (targeted ranges), DEPLOY-RUN.md head, CONTEXT, RESEARCH.
**Pattern extraction date:** 2026-10-14
