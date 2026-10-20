# Phase 22: Backup and restore, measured - Research

**Researched:** 2026-10-14
**Domain:** Proving a shell-script backup/restore of a single-binary SQLite CMS, under real systemd in an amd64-emulated container (evidence-first, no new features)
**Confidence:** HIGH for the mechanics (code read plus four probes run this session); MEDIUM for the one predicted defect (the backup does not cover `plugins/`, `sprachen/`, `marke/`), which is read from code and **not yet played**

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

The user delegated all decisions for this phase to Claude ("Entscheide du alles"), as in
Phase 21. Everything below is Claude's call, made conservative and evidence-first.

#### What "an empty system" is, and the source site (BAK-02)
- **D-01:** The restore target is a **fresh container**: `TARGET=systemd-base`, a new `NAME`, DEPLOY.md's install played through `install` (as in Phase 21), **no data carried over**. It is a different container from the one that was backed up, so the proof cannot lean on state that survived in place. The service of the new container may have started once and written an empty database; `restore.sh` moves that aside, which is the realistic situation on a rebuilt server.
- **D-02:** The site that is backed up is the populated one Phase 21 leaves: `INSTALL_FROM=old ... build up install baseline update` (website, published page, five plugins, image, login user, order, marker page, pre-upgrade snapshot). Reuse it; do not seed a second, different fixture.
- **D-03:** The backup that is **restored is the one the timer produced** (D-12), not a separate manual one. That makes BAK-03 and BAK-02 one chain: timer → backup → transfer → restore. A manual `backup.sh` run still happens for BAK-01's integrity assertions and is recorded separately.
- **D-04:** Transfer between the two containers goes **through the host with `docker cp`** (the same stand-in for `scp` that Phase 21 D-01 uses) and is named as a deviation from a real server, where it would be `rsync`/`scp`.
- **D-05:** `restore.sh` is run **unchanged and interactively as designed**: the answer `yes` comes from stdin. No `--yes` flag is added (see D-16).

#### Proof of integrity and content (BAK-01, BAK-02)
- **D-06:** "Passes its integrity check" is shown twice, by two tools: the binary's own check (`holzcloud check`, which `restore.sh` already runs on a copy) **and** an independent `sqlite3 -readonly ... PRAGMA integrity_check` on the backup file. A backup that two different readers accept is stronger than one that the writer vouches for.
- **D-07:** "Contains database and media" is shown by **comparison, not by listing**: row counts per table that exist in both, a `sha256` list of every file under `media/` (and `templates/`, `csrf.key`) in the source data directory against the backup directory and against the restored data directory. Raw outputs committed. An empty `media/` directory would pass a listing and must fail here, so the source is asserted non-empty first.
- **D-08:** "Working pages, media and login" after the restore = the public page renders and carries the marker content, the media file is served with the same hash, the login user logs in with the password it had before (session via the real login form, with the CSRF behaviour Phase 21 already documented), and `check` passes against the restored container. `/root/.hc-key` lives outside the data directory and is **not** part of a backup (README hand-over): the run states what is rebuilt by hand on the new box and what comes back from the backup.
- **D-09:** **Negative cases are played once and recorded:** (a) a backup whose `holzcloud.sqlite` has been damaged must be refused by `restore.sh` **before** the live data is touched (the data directory is unchanged afterwards); (b) a backup directory without `holzcloud.sqlite` is refused. The doc states facts, not the comment in the script.
- **D-10:** `backup.sh` gets **no new MANIFEST fields or checksum files** unless the played run shows that a restore cannot tell a truncated backup from a whole one. The comparison in D-07 is the proof; a feature is only added if it is the fix for a defect.

#### Timer run (BAK-03)
- **D-11:** The timer is brought into reach by a **container-only drop-in on `holzcloud-backup.timer`** that replaces `OnCalendar` with a time a few seconds to a minute ahead and sets `RandomizedDelaySec=0`. The unit files in `deploy/` are not edited; the drop-in is named as a **deviation** in the record (alongside the `ProcSubset=all` drop-in of Phase 20). The clock is not changed: it would also move the journal, the `-mtime` retention and the `Persistent=true` bookkeeping.
- **D-12:** "Observed" means three artefacts, all committed raw: `systemctl list-timers holzcloud-backup.timer` before and after (the NEXT/LAST columns moved), `journalctl -u holzcloud-backup.service` for that one invocation (started by the timer, not by hand, exit status 0), and the **new backup directory** it produced under `/opt/holzcloud/backups`, owned by the service user with mode 700. The hardening of `holzcloud-backup.service` (`ProtectSystem=strict`, `ReadWritePaths`) is exercised for real here, which is the point.
- **D-13:** What the drop-in cannot prove — the real 03:15 window, `Persistent=true` after a missed run, `RandomizedDelaySec` — is **named as not tested**, not claimed.

#### Measurement (BAK-02)
- **D-14:** Record, for the restore and for the backup: wall time (`date +%s.%N` around the step, not `time` of a pipeline), size of the backup directory (`du -sb` and `du -sh`), size of database and media separately, and the file counts. Two data points: the **seed site as it is**, and the same site with **synthetic media added** (order of 100–500 MB of non-compressible files, planner picks) so that the time is not dominated by service start and the per-MB rate can be read.
- **D-15:** Times come from the **amd64 emulation** (Phase 20 D-01/Phase 21 D-13) and say so in the record, next to the host architecture. Any extrapolation to a real server is written as an estimate with its assumption (rsync/disk-bound, linear in size), never as a measurement. The restore time is split: verify, stop+move+copy, start to `/readyz`.

#### Script changes and defects
- **D-16:** If the played run exposes a defect, **fix the script in this phase** and record step / what happened / fix, as Phase 21 D-02 does for the document. Likely candidates to *look at*, not to change in advance: stale `-wal`/`-shm` after the restore (`restore.sh` moves the whole data directory aside, so probably none — verify), `chown`/mode of the restored tree, `MIN_FREE_MB=1024` on the container disk (override via env and name it), the non-atomic pair "database snapshot first, media copied after" (DB rows never reference a file the backup lacks, extra media files are harmless — state this in DEPLOY.md if it is not there). Changes stay minimal and do not add options.
- **D-17:** `deploy/DEPLOY.md` is the source of truth. Its Backups section is extended or corrected so that a reader needs nothing from this phase's record: the restore command, the `yes` prompt, what comes back and what does not (`/root/.hc-key`-like material outside the data directory, anything in `/etc`), the measured size/time as an order of magnitude with the emulation caveat, and the non-atomic note from D-16. Wording in English, matching the document.
- **D-18:** Evidence follows Phase 21: repeatable steps as new `run.sh` subcommands or chain steps (build/up/start separable), raw logs under `deploy/test/out/`, and a committed record **`deploy/BACKUP-RUN.md`** that pastes them unchanged, with the same line-by-line `grep -F` proof as `DEPLOY-RUN.md`. The record lists deviations (D-04, D-11, emulation) first. Do not edit `deploy/DEPLOY.md` between the chains and the record.

### Claude's Discretion
All of the above, plus: exact names of the new subcommands, how row counts and hashes are collected, size of the synthetic media, the time offset of the timer drop-in, whether the damaged-backup case truncates or flips bytes, wording of doc fixes, and whether the target container is built on bookworm or trixie (`BASE_IMAGE`; keep the Phase 21 default).

### Deferred Ideas (OUT OF SCOPE)
- Off-site copy as a tested path (`REMOTE_TARGET` rsync): REQUIREMENTS lists an off-site backup *service* as out of scope; `backup.sh` keeps naming rsync as an operator decision. At most the document says it was not played.
- An atomic DB-plus-media snapshot (filesystem snapshot / quiesce): new capability, own phase if ever needed.
- A non-interactive `restore.sh --yes` or a restore subcommand in the binary: new surface with its own test burden.
- Testing the real 03:15 window, `Persistent=true` catch-up and `RandomizedDelaySec` on a real server: needs a real host and days of wall time.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BAK-01 | `backup.sh` erzeugt eine verifizierte Sicherung von SQLite und Medien. | The binary's own snapshot+check is `VACUUM INTO` + `PRAGMA quick_check(1)` (not `integrity_check`), so the independent second reader must be `integrity_check`, and it must read the WAL-mode backup file safely (copy, or `?immutable=1`). "Contains media" must be a tree-hash comparison; the comparison must cover the **whole** data directory, which is what exposes that `plugins/`, `sprachen/`, `marke/` are not backed up (Pitfall 1). |
| BAK-02 | `restore.sh` stellt auf einem leeren System her; Seiten, Medien und Login funktionieren. Dauer und Größe sind gemessen. | Reuse `capture_state` + `compare_states` across two containers (they already assert row counts, page hash, media hash, login, public page, 5/5 plugins, order token). Time and size must be taken **inside the container** (host is macOS: no `date +%N`, no `du -b`). Transfer by tar stream preserves modes, mtimes and case-distinct names (probe). The fixture secrets (`/root/.hc-login-pass`, `/root/.hc-key`, `/tmp/hc-upload.jpg`) must be carried by exec-pipe, never through a host file. |
| BAK-03 | Ein Lauf über `holzcloud-backup.timer` ist einmal real durchgespielt. | A drop-in with `OnCalendar=` (empty reset) + `OnCalendar=*-*-* HH:MM:SS` + `RandomizedDelaySec=0`, applied with `daemon-reload` only (no timer restart), fired on time in a probe; proof of "started by the timer" is `TriggeredBy=holzcloud-backup.timer` on the service plus the journal; the resulting directory is named by `=== Backup complete: DIR ===` in that journal. The chain must include `postinstall` (it installs and enables the units; `baseline update` do not). |
</phase_requirements>

## Summary

This phase is a proof phase: nothing new is built except test-harness subcommands, a record (`deploy/BACKUP-RUN.md`), and fixes for whatever the played run exposes. All of the machinery needed already exists from Phases 20/21: the `deploy/test/run.sh` harness (2469 lines, bash 3.2-compatible, doc-driven), `capture_state`/`compare_states` (state capture and comparison that already fail on a tampered copy), `leak_check`, `postinstall` (installs and enables the backup timer and runs one manual backup), and the populated-site chain `INSTALL_FROM=old ... build up install baseline update`. The work is to add a second container (the restore target), a timer drop-in, a bulk-media fixture, timing/size instrumentation, negative cases, and a record.

The most important finding is a **probable defect the phase was designed to find**: `backup.sh` backs up only `holzcloud.sqlite`, `csrf.key`, `media/` and `templates/`, but the data directory also holds `plugins/` (the WASM modules and assets of every installed plugin; the database keeps only manifest and sha256), `sprachen/` (operator language files) and `marke/` (the logo). After a restore from a `backup.sh` backup the plugin rows exist, are enabled, and the manager records "the module is missing from disk" and does not load them; the harness's `plugins running 5/5` assertion and `check` (17 assertions) will fail. A CONTEXT D-07 comparison restricted to `media/`, `templates/`, `csrf.key` would **not** catch it; hashing the whole data directory (minus the SQLite family and pre-upgrade snapshots) would. D-16 authorises the fix in this phase; it is the same `rsync` pattern the script already uses, twice more or in a loop, mirrored in `restore.sh`, with `docs/deployment.md` and `docs/configuration.md` (which both claim the script backs up "this whole directory") corrected.

Other facts that change the plan: the shell `read -p` prompt of `restore.sh` is **not printed** when stdin is not a terminal (the record's transcript will lack the `[yes/NO]` line; verified); the VACUUM INTO backup file is itself WAL-mode (header bytes `02 02`), so a plain `sqlite3 -readonly` can fail on newer sqlite versions (macOS 3.51: `unable to open database file (14)`; Debian 3.40.1 in the container worked); the binary's `check` is `quick_check`, not `integrity_check`; deterministic ways to damage a backup (truncate to half, smash the header, overwrite 4 KB mid-file) were all refused by `holzcloud check`; the timer mechanics (empty `OnCalendar=` reset, `daemon-reload` is enough, `TriggeredBy=`) were verified live in a throwaway systemd container.

**Primary recommendation:** Play the unchanged scripts first (pass 1) against a source container built by `INSTALL_FROM=old ... build up install baseline update postinstall`, with a **whole-data-directory** hash comparison and `compare_states` across two containers; expect the plugin/`sprachen`/`marke` gap; fix `backup.sh`+`restore.sh` minimally; then make the final evidence set from fresh containers against the final scripts and document (Phase 21's pass-1/repair/final-set pattern), and write `deploy/BACKUP-RUN.md` with the `grep -F` proof.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Consistent DB snapshot + integrity verdict | Binary CLI (`holzcloud backup`, `check`) | — | `VACUUM INTO` through the pure-Go driver plus `quick_check`; the script only calls it [VERIFIED: internal/db/maintain.go:68-109] |
| Copy of media/templates/other data files | Shell script (`backup.sh`, `restore.sh`, rsync) | — | Not the binary's job; rsync of directories |
| Scheduling a nightly backup | systemd (`holzcloud-backup.timer` → `.service`) | — | Hardened oneshot as the service user; the point of BAK-03 is to exercise that hardening |
| Independent integrity reader | `sqlite3` CLI inside the container | Harness on host orchestrates via `docker exec` | A second implementation (C sqlite) distinct from the Go driver |
| Time/size measurement | Inside the container (GNU `date`, `du`, `find`) | Host `run.sh` only collects text | Host is macOS: BSD `date` has no `%N`, BSD `du` has no `-b` |
| Transfer source → target | Host (`docker cp` tar stream) | — | Stand-in for `rsync`/`scp` (D-04, named deviation) |
| Evidence and record | Host repo (`deploy/test/out/` scratch, `deploy/BACKUP-RUN.md` committed) | — | Phase 21 pattern |

## Standard Stack

No external packages are installed by this phase. Everything is the existing toolchain; there is no `npm`/`pip`/`cargo` dependency to add.

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| `deploy/test/run.sh` | in repo (2469 lines) | Harness to extend: new subcommands/chain | README: "the interface below is stable: extend it, do not rename it" [VERIFIED: deploy/test/README.md:8] |
| `deploy/backup.sh`, `deploy/restore.sh` | in repo | Scripts under test | The deliverable under proof |
| `holzcloud-backup.service` / `.timer` | in repo | Units under test | BAK-03 |
| `sqlite3` | 3.40.1 in bookworm image | Independent integrity reader, row counts | Already in the `systemd-base` image [VERIFIED: probe `sqlite3 --version` in the image, this session] |
| `rsync` | from Prerequisites `apt install` | Used by both scripts | DEPLOY.md Prerequisites names it [VERIFIED: deploy/DEPLOY.md:18,24] |
| Docker Desktop | 29.8.2 on aarch64 host | Containers, `--platform linux/amd64` under emulation | Phase 20/21 recipe [VERIFIED: `docker --version`, `docker info`, this session] |

### Supporting
| Tool | Purpose | When to Use |
|------|---------|-------------|
| `systemctl list-timers`, `systemctl show -p TriggeredBy`, `journalctl -u holzcloud-backup.service` | Timer evidence | BAK-03 |
| `sha256sum`, `find`, `du -sb`, `date +%s.%N` (GNU, in container) | Hashes, sizes, wall time | BAK-01/02 measurement |
| `jq` | Reading `/readyz` | Existing pattern |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Drop-in with calendar time (D-11, locked) | `OnActiveSec=` | Not locked-in; D-11 is explicit. Calendar form was verified to work. |
| `docker cp A:dir hostdir` then `docker cp hostdir B:` | tar stream `docker cp A:dir - \| docker cp - B:dest` | Stream never lands on the macOS (case-insensitive) filesystem, no host disk for 300 MB, preserves modes/mtimes/uid numerically [VERIFIED: probe this session]. Still "through the host" so D-04 holds. |
| `sqlite3 -readonly FILE` on the backup | `sqlite3 "file:FILE?immutable=1"` or check a copy | `-readonly` failed on macOS sqlite 3.51 for the WAL-mode backup; both alternatives worked [VERIFIED: probes this session] |

**Installation:** none. **Version verification:** `docker`, `go`, `bash`, `sqlite3`, `systemd` versions probed in Environment Availability.

## Package Legitimacy Audit

No external packages are installed by this phase; the `systemd-base` image and Prerequisites already carry every tool. **Packages removed due to [SLOP] verdict:** none. **Packages flagged as suspicious [SUS]:** none. (No `gsd-tools package-legitimacy` run was needed: nothing is added.)

## Architecture Patterns

### System Architecture Diagram

```
                      host (macOS aarch64, Docker Desktop VM, amd64 emulation)
                      deploy/test/run.sh  (orchestrates only; takes text, never secrets)
                                |
        +-----------------------+---------------------------------------------+
        |                                                                     |
 SOURCE container  (NAME=src, OUT_DIR=out/src)                     TARGET container (NAME=tgt, OUT_DIR=out/tgt)
 build up install baseline update postinstall                      build up install     (service ran once: empty DB)
        |                                                                     |
 [fixture] bulk media (data point 2 only)                                     |
        |                                                                     |
 [BAK-01] manual backup.sh as service user  ---> BACKUP_A (timed, sized)      |
        |   checks: holzcloud check + sqlite3 integrity_check (immutable)     |
        |   content: whole-data-dir sha256 list  src == backup ; row counts   |
        |                                                                     |
 [BAK-03] timer drop-in (OnCalendar= reset, +60s, Randomized=0)               |
        |   daemon-reload ; list-timers BEFORE                                |
        |   timer fires -> holzcloud-backup.service (hardened, User=holzcloud)|
        |   list-timers AFTER ; TriggeredBy ; journal ; dir 700 owner         |
        +--> BACKUP_T (named by "=== Backup complete: DIR ===" in the journal)|
                         |                                                    |
                         |   tar stream via host (DEVIATION[transfer])        |
                         +--------------------------------------------------->|
                                                                              |
   secrets by exec-pipe: /root/.hc-login-pass, /root/.hc-key, /tmp/hc-upload.jpg  ------>|
                                                                              |
                                  [BAK-02] printf 'yes\n' | restore.sh BACKUP_T  (timestamped lines)
                                          verify copy -> stop -> mv aside -> copy -> chown -> start -> /readyz
                                                                              |
                                  assertions: tree hash tgt == backup == src ; capture_state+compare_states
                                              vs source ; run.sh check ; /admin/setup now 404
                                                                              |
                          negative cases (on target or a 3rd scratch dir): damaged DB refused before data touched;
                          no holzcloud.sqlite refused; answer "no" aborts
                                                                              |
                                  deploy/test/out/*  ->  deploy/BACKUP-RUN.md (pasted unchanged, grep -F proof)
```

### Recommended Project Structure
```
deploy/
├── backup.sh, restore.sh            # fix only what the played run exposes (D-16)
├── DEPLOY.md                        # Backups + "Restore from a backup" corrected (D-17)
├── BACKUP-RUN.md                    # NEW: the record (D-18), pastes deploy/test/out files unchanged
└── test/
    ├── run.sh                       # new subcommands + chain (names are discretion; extend, do not rename)
    ├── README.md                    # subcommand table, env vars, output files, "Phase 22" section
    └── out/                         # gitignored scratch [VERIFIED: .gitignore:33 "/deploy/test/out/"]
docs/deployment.md, docs/configuration.md   # claims "back up this whole directory ... backup.sh does it" must agree with the fixed script
```

### Pattern 1: Two containers, one harness
**What:** `cmd_up` and every `dx`/`dxi` helper read the globals `NAME` and `OUT_DIR`. A chain that needs source and target switches the globals between phases (or adds `TARGET_NAME`/`TARGET_OUT_DIR` variables, documented in the README). `up` resets only its own `OUT_DIR`; `BIN_DIR` is shared (README Pitfalls). The `on_exit` trap removes only `NAME`, so a two-container chain must remove both unless `KEEP=1`.
**When to use:** the whole phase. Pin `NEW_REF` to a sha (not `HEAD`) for both containers so the binary is byte-identical even if a script fix is committed mid-phase (`bin_path new` names the binary by the resolved sha).
**Example:**
```bash
# Source: deploy/test/README.md (Quick start) + this research
PLATFORM=linux/amd64 TARGET=systemd-base INSTALL_FROM=old NEW_REF=<sha> \
  NAME=holzcloud-bak-src OUT_DIR=deploy/test/out/amd64-bak-src \
  ./deploy/test/run.sh build up install baseline update postinstall
PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=<sha> \
  NAME=holzcloud-bak-tgt OUT_DIR=deploy/test/out/amd64-bak-tgt \
  ./deploy/test/run.sh build up install
```
`postinstall` is required in the source chain: `baseline`/`update` do not install the backup units; `postinstall` plays DEPLOY.md's Backups block (copies the units from `/opt/holzcloud/deploy`, enables the timer, runs one manual backup as the service user) [VERIFIED: deploy/test/run.sh:1411-1474 and deploy/DEPLOY.md:617-625].

### Pattern 2: Measure inside the container
**What:** One `docker exec -i NAME bash -s` script that takes `date +%s.%N` before and after, and for `restore.sh` prefixes every output line with a timestamp, so the split verify / stop+move+copy / start-to-ready comes from the script's own progress lines (`Verifying snapshot...`, `Stopping holzcloud...`, `Restoring database...`, `Starting holzcloud...`, `Waiting for readiness...`, `=== Restore complete ===`). The readiness loop of `restore.sh` polls once a second (`seq 1 30` / `sleep 1`), so start-to-ready has ±1 s resolution; say so.
**Why:** host is macOS bash 5.3 / BSD tools [VERIFIED: `bash --version`, `go version` darwin/arm64 this session]; the BSD `date` cannot print nanoseconds.
**Example:**
```bash
# Source: this research; line prefixes are filled by the container's GNU date
printf 'yes\n' | dxi bash -c '
  t0=$(date +%s.%N)
  /opt/holzcloud/deploy/restore.sh "$1" 2>&1 | while IFS= read -r l; do printf "%s %s\n" "$(date +%s.%N)" "$l"; done
  t1=$(date +%s.%N); echo "WALL $t0 $t1"' _ /opt/holzcloud/backups/TS
```
(Caveat: `read -p` does not print its prompt with non-terminal stdin, so the `Stop holzcloud and replace ...? [yes/NO]` text will not appear in the transcript, and the pipe-through-`while` makes `$?` that of the loop; take the script's exit status from `PIPESTATUS`/`set -o pipefail`.)

### Pattern 3: Whole-data-directory hash comparison
**What:** One function `tree_hash DIR` run as root inside the container: `cd DIR && find . -type f ! -name 'holzcloud.sqlite*' ! -name 'pre-upgrade-*.sqlite' ! -name .readyz -print0 | sort -z | xargs -0 sha256sum`. Run on the source data dir, on the backup dir (minus `MANIFEST`), and on the restored data dir; diff the three lists. Assert the source list is non-empty and that `media/` contributes files (D-07's "empty must fail"). This is what exposes `plugins/`, `sprachen/`, `marke/`.
**Why not just `media/ templates/ csrf.key`:** that exact scope is what the script copies, so the comparison would pass while the restore is functionally broken (Pitfall 1).

### Pattern 4: Reuse the state comparison across containers
`capture_state LABEL` writes `state-LABEL.txt` in the current `OUT_DIR`; `compare_states A B OUT` and `run.sh compare A B` already do the cross-file comparison and already have a self-test showing it can fail [VERIFIED: deploy/test/run.sh:1590-1621, 1657-1768, 1911-1914]. For the target, `capture_state` needs in the target container: `/root/.hc-login-pass` (login), `/root/.hc-key` (`list_plugins`), `/tmp/hc-upload.jpg` (media source hash) and `ORDER_NO` (or the source's `baseline.txt` copied into the target `OUT_DIR`). Carry them by `docker exec SRC cat F | docker exec -i TGT bash -c 'umask 077; cat > F'`: nothing touches the host disk, and `leak_check` (which reads those files inside the container and greps `OUT_DIR`) keeps working.

### Pattern 5: Timer drop-in
```bash
# Source: verified live in a throwaway systemd 252 container this session (probe.timer = copy of holzcloud-backup.timer)
T=$(date -u -d '+60 seconds' '+%H:%M:%S')           # container clock, UTC
mkdir -p /etc/systemd/system/holzcloud-backup.timer.d
printf '[Timer]\nOnCalendar=\nOnCalendar=*-*-* %s\nRandomizedDelaySec=0\nAccuracySec=1s\n' "$T" \
  > /etc/systemd/system/holzcloud-backup.timer.d/container.conf
systemctl daemon-reload            # enough: NEXT moved to the new time without a restart
systemctl list-timers holzcloud-backup.timer --no-pager      # BEFORE: record
# ... poll until the service has an ExecMainStartTimestamp, never sleep ...
systemctl show holzcloud-backup.service -p TriggeredBy -p Result -p ExecMainStatus -p ExecMainStartTimestamp -p ExecMainExitTimestamp --no-pager
journalctl -u holzcloud-backup.service --no-pager -o short-precise
systemctl list-timers holzcloud-backup.timer --no-pager      # AFTER: NEXT is the next day, LAST is the fire time
```
`AccuracySec=1s` is added to the drop-in because the default is 1 min and the timer may fire up to that late [CITED: man systemd.timer(5), via manpages.debian.org: "AccuracySec= defaults to 1min"]; in the probe the unmodified default fired on the second, so it is a safety, not a necessity.

### Anti-Patterns to Avoid
- **Restricting the content comparison to what the script copies** (D-07 literal scope): proves nothing about completeness. Hash the whole data directory.
- **`systemctl restart holzcloud-backup.timer` after adding the drop-in:** a restart makes `LAST` show the time the timer was started (Persistent stamp), so a "LAST moved" claim becomes ambiguous (probe: after a restart LAST showed `07:10:41`, 5 s ago, with nothing fired). `daemon-reload` alone moved NEXT. Evidence of an actual run is the journal and `TriggeredBy`, with `LAST` as corroboration.
- **Running `holzcloud check`/`backup`/`user …` of a newer binary against an old database to "look":** they migrate silently (README Pitfalls). Look with `sqlite3`.
- **`docker cp` to a macOS directory for the backup tree:** case-insensitive filesystem can merge `A.jpg`/`a.jpg`; host disk use; use the tar stream (probe: both names survive, modes `700`/`600`/`644` and mtimes preserved, owner kept numerically).
- **Editing `deploy/DEPLOY.md` between the final chains and the record** (D-18; README): the evidence set is made against one text of the document.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Comparing two sites | A new diff of tables/hashes | `capture_state` + `compare_states` (+ `compare_selftest` idea) | Already handles volatile tables (sessions, activity_log, outbox, plugin_store, ...), page/media hashes, login, public page, plugin count, order token |
| Playing document commands | Typing commands into `run.sh` | `doc_play` / `doc_cmd` reading fenced bash blocks of `DEPLOY.md` | Drift guard; `doc_cmd` refuses a line not in the document |
| Secret handling | Writing passwords/keys to out files | `leak_check`, root-only files in the container, exec-pipe transfer | Phase 21 T-21-01 |
| Login with CSRF | A new curl dance | `form post /admin/login EMAIL PASSFILE JAR` (`FORM_SH`) | Handles the hidden CSRF field, entity decoding and the Referer the `HOLZCLOUD_SECURE=true` unit requires |
| Proving the record equals the output | Eyeballing | A `grep -Fxq` loop of every non-empty line of every pasted file against `BACKUP-RUN.md` | Phase 21's verification re-parsed 29 blocks; make it a repeatable command |
| Snapshot of a live SQLite DB | `cp` of the file | `holzcloud backup` (VACUUM INTO) — already used | WAL-safe |
| Retention / rotation / off-site | New logic | Out of scope (REQUIREMENTS); name as not played | Deferred ideas |

**Key insight:** every assertion this phase needs already exists in the harness as a function that works on one container; the new work is orchestration across two containers, timing, and a *complete* (not script-scoped) content comparison.

## Common Pitfalls

### Pitfall 1: The backup does not contain `plugins/`, `sprachen/`, `marke/` (probable defect)
**What goes wrong:** After `restore.sh`, the five plugin rows are enabled in the database but their module files are gone; the manager logs `plugin not loaded` and records `the module is missing from disk`; `[[jahr]]` shortcodes stay unrendered, `capture_state` shows `plugins: 0` and `compare_states` prints `BAD plugins running ... (5 expected)`; `run.sh check` fails. The operator logo (`marke/logo.*`) and operator language files (`sprachen/*.json`) are also lost silently.
**Why it happens:** `backup.sh` copies exactly four things. Quoted verbatim:
- `backup.sh:52-67`: `if [ -f "${DATA_DIR}/csrf.key" ]; then`, `if [ -d "${DATA_DIR}/media" ]; then`, `rsync -a "${DATA_DIR}/media/" "${BACKUP_DIR}/media/"`, `if [ -d "${DATA_DIR}/templates" ]; then` [VERIFIED: deploy/backup.sh:52-67]
- The data directory also contains: `func (m *Manager) root() string         { return filepath.Join(m.dataDir, "plugins") }` [VERIFIED: internal/plugin/manager.go:94]; `module, err := os.ReadFile(filepath.Join(m.dir(p.ID), ModuleName))` and `m.note(ctx, p.ID, fmt.Errorf("the module is missing from disk: %w", err))` [VERIFIED: internal/plugin/manager.go:117-119]; `const DirName = "marke"` [VERIFIED: internal/branding/branding.go:23]; `const DirName = "sprachen"` [VERIFIED: internal/i18n/disk.go:31]; the `plugins` table keeps `manifest` and `sha256` only [VERIFIED: internal/db/migrations/00021_plugins.sql:15-29].
- Both `docs/deployment.md` ("Back up the whole data directory; `/opt/holzcloud/deploy/backup.sh` does it WAL-safely") and `docs/configuration.md` ("Back up this whole directory and you have everything. `deploy/backup.sh` does it WAL-safely") claim more than the script does [VERIFIED: docs/deployment.md:118, docs/configuration.md:130-131]. `docs/configuration.md`'s tree omits `plugins/` and `marke/` too.
**How to avoid / fix (D-16, minimal, no new options):** in `backup.sh` and `restore.sh` extend the two `rsync` blocks to a loop over `media templates plugins sprachen marke` (each `[ -d ]`-guarded, same as today). Keep the top-level directory names so **older backups still restore** (restore handles each directory only if present). The pre-upgrade snapshots (`pre-upgrade-*.sqlite`) are deliberately not backed up (they are old-schema copies in the data directory); state it. Fix the two docs and DEPLOY.md.
**Warning signs:** the whole-directory tree-hash diff shows `plugins/...` only in the source; `check.txt` has `BAD loaded bestellung`; the journal of the target has `plugin not loaded`.
**Test-fixture consequence:** `sprachen/` and `marke/` are empty/absent on the seeded site; plant one small file in each in the source (a `DEVIATION[fixture]`) so the fix is exercised, or state they were read but not played.

### Pitfall 2: The independent integrity check can fail on a good backup
**What goes wrong:** `sqlite3 -readonly b/holzcloud.sqlite "PRAGMA integrity_check"` answered `Error: in prepare, unable to open database file (14)` on a good backup (macOS sqlite 3.51). The reason: the VACUUM INTO output is a WAL-mode file; bytes 18-19 of the header read `0202` for both the live and the backup file [VERIFIED: probe this session, `xxd -s 18 -l 2`]. On Debian bookworm sqlite 3.40.1 as root the same command printed `ok`.
**How to avoid:** use `sqlite3 "file:/path/holzcloud.sqlite?immutable=1" "PRAGMA integrity_check"` (worked on both 3.51 and 3.40.1; it never creates `-wal`/`-shm` next to the backup) or check a copy, like `restore.sh` does. Do not open the backup read-write: it would add `-wal`/`-shm` files into the backup directory and change the evidence.
**Also:** `holzcloud check` runs `PRAGMA quick_check(1)` [VERIFIED: internal/db/maintain.go:24 `` `PRAGMA quick_check(1)` ``, cmd/holzcloud/cli.go:290-306], and `db.Backup` runs the same, so the "second reader" is also a *stronger pragma* (`integrity_check`); say so in the record.

### Pitfall 3: Choosing the damaged-backup case (D-09a)
Three deterministic corruptions were all refused by `holzcloud check`, so any is a valid choice (measured natively on macOS with the HEAD binary built here, same code as the container's): truncate to half → `error: open database at …: verify WAL: database disk image is malformed (11)`; overwrite the first 16 bytes → `verify WAL: file is not a database (26)`; overwrite 4 KB at mid-file → `Tree 78 page 78: btreeInitPage() returns error code 11` then `error: database reports corruption — restore from a backup`. **Do not use a small text-byte flip inside a data page:** SQLite has no page checksums, so neither `quick_check` nor `integrity_check` would notice it, and the "refusal" would not occur (reasoned from SQLite's design; not played) [ASSUMED]. Prefer truncation (simplest, deterministic). Prove "before the live data is touched" by hashing the data directory (the SQLite family included) before and after the refused run and requiring equality, and that the service is still `active`, since `restore.sh` verifies before it stops the service (restore.sh:43-53 order: verify, `read`, `systemctl stop`).

### Pitfall 4: `read -p` shows no prompt without a terminal
**What goes wrong:** `echo yes | bash -c 'read -r -p "PROMPT? " c; echo got=$c'` printed only `got=yes` (bash 5.3, this session; the prompt is displayed only when input comes from a terminal). So the restore transcript has no `[yes/NO]` line, and the DEPLOY.md wording about the prompt must be written from the script, not from the captured transcript. The record should say this once [VERIFIED: probe this session]. To capture the prompt you would need a pty (`docker exec -t` with a tty stdin, or `script`); not needed for the proof.

### Pitfall 5: Timer evidence traps
- `LAST` is populated after a *restart* without a run (stamp file; probe) — rely on `TriggeredBy` + journal.
- `OnCalendar` is a list: the drop-in **must** start with an empty `OnCalendar=` line, else both the 03:15 and the new time apply [CITED: systemd.timer(5): "If the empty string is assigned to any of these options, the list of timers is reset" via manpages.debian.org; and VERIFIED live: the resulting `NextElapseUSecRealtime` was the drop-in time].
- The first `NEXT` after `postinstall` is the real `03:15 + up to 300 s` of tomorrow (probe: `Thu 2026-10-15 03:17:36 UTC`); running the harness within minutes of 03:15 UTC would collide with it — immaterial, but state the container clock.
- The proof of "started by the timer, not by hand": `systemctl show holzcloud-backup.service -p TriggeredBy` printed `TriggeredBy=probe.timer` for the timer-fired probe service [VERIFIED: probe this session]; a manual `sudo -u holzcloud backup.sh` run never appears in the unit's journal.
- Identify the produced directory from the journal line `=== Backup complete: /opt/holzcloud/backups/TS ===` (backup.sh:91), because the backups directory already holds the `postinstall` manual backup and the Updating-step-1 backup of the old binary (schema 58).
- Seccomp is not enforced under emulation (`Seccomp: 0`), so the hardened unit's `ProtectSystem=strict`/`ReadWritePaths`/`PrivateTmp` mount-namespace sandboxing is exercised but a syscall filter is not; the backup unit sets no `SystemCallFilter` anyway [VERIFIED: deploy/holzcloud-backup.service:18-22 shows `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`, `ReadWritePaths`, `PrivateTmp` only].

### Pitfall 6: Host tools and bash dialect
`run.sh` is written for bash 3.2 (no arrays, no `mapfile`, no `${v,,}`); GNU-only things run in the container via `docker exec` [VERIFIED: deploy/test/run.sh:55-56]. New code must keep that: no `date +%s.%N`, `du -b`, `stat -c`, `sha256sum` on the host. `docker cp` with a macOS bsdtar source needs `--no-xattrs` etc. (see the existing `scp` translation); the container-to-container stream does not.

### Pitfall 7: Secrets and files the restored site "needs" that are not in a backup
Not in `backup.sh` output, by design: the unit file and its drop-ins (`/etc/systemd/system`), `/etc/holzcloud.env`, `/etc/holzcloud-sso.env`, `HOLZCLOUD_SECRET_KEY` (without it stored website mail-account passwords no longer open: docs/configuration.md), Payrexx key, the Caddyfile, and the test-only `/root/.hc-key`/`/root/.hc-login-pass`. In the harness the two `/root` files are *test fixture* secrets that stand in for what the operator remembers; carry them by exec-pipe and name it `DEVIATION[fixture]`. The AI key itself is validated against the restored database (`token_hash` table `ai_tokens`), so a working `list_plugins` with the old key after restore also proves the key table came back [VERIFIED: internal/ai/oauth.go:470 shows `token_hash`; that the CLI-created key uses the same hashing is [ASSUMED]].

### Pitfall 8: `holzcloud.service` start after restore, and the setup form
A target whose first start wrote an empty database has the **open** setup form (`GET /admin/setup` → 200, README/firstrun). After a successful restore the restored database has users and the form must answer 404. Assert both (before: 200, after: 404): it is also the sharpest check that the *restored* database, not the empty one, is live. [VERIFIED for the behaviour: deploy/test/run.sh:1319-1340 `PASS setup form open before any account` / `closed afterwards`.]

### Pitfall 9: Ownership and permission after transfer and restore
The tar-stream transfer keeps the numeric uid and modes (probe: owner `UNKNOWN` on a box without that uid, modes `700`/`600`/`644` kept). `restore.sh` ends with `chown -R "${OWNER}" "${DATA_DIR}"` and `chmod 700 "${DATA_DIR}"` [VERIFIED: deploy/restore.sh:82-83], so ownership comes right regardless of uid; assert owner `holzcloud` and mode of `holzcloud.sqlite`, `csrf.key`, a media file and `plugins/` after the restore. `mkdir -p "${DATA_DIR}"` creates the directory root:root 755 first; the window until `chmod 700` is inside the stopped-service period.

### Pitfall 10: Defaults that could bite
- `MIN_FREE_MB=1024` is checked on `BACKUP_ROOT` with `df -Pm` *before* the backup directory is made (backup.sh:37-43); with the 300 MiB bulk set plus copies this is not near on the Docker VM (Phase 21 saw ~402 GB free). A cheap negative case: `MIN_FREE_MB=99999999 backup.sh` must exit 1 and create no new directory.
- `RETENTION_DAYS` 30 `find -mtime +30 … rm -rf`: not exercised (fresh containers); the timestamps the tar stream preserves are whole seconds.
- `restore.sh` hardcodes `http://127.0.0.1:8080/readyz` and a 30 s wait; Phase 21 saw 8-10 s starts under emulation; with 300 MiB the start does not read media, so no change expected.
- `/readyz` also fails on low free disk (`HOLZCLOUD_MIN_FREE_BYTES` default 512 MiB [VERIFIED: internal/config/config.go:317 region `envSize("HOLZCLOUD_MIN_FREE_BYTES", 512*1024*1024`]); irrelevant here, relevant if the container disk is small.

### Pitfall 11: Non-atomic backup (D-16 note)
`backup.sh` snapshots the database first and copies media afterwards, so a media file uploaded in between is in the backup without a database row (harmless), while a row can never reference a file the backup lacks as long as files are written before rows. Deleting media between the two steps can produce a row whose file is gone. State this in DEPLOY.md as D-16 says; this is reasoning, not a played case [ASSUMED for the delete ordering].

## Code Examples

### Independent integrity + row-count + hash proof on the backup (inside the container, as root)
```bash
# Source: this research; commands verified individually in probes (immutable URI, find|sha256sum is standard)
B=/opt/holzcloud/backups/TS
sqlite3 "file:$B/holzcloud.sqlite?immutable=1" "PRAGMA integrity_check;"          # expect: ok
sqlite3 "file:$B/holzcloud.sqlite?immutable=1" \
  "select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name" |
  while IFS= read -r t; do printf 'table %s %s\n' "$t" "$(sqlite3 "file:$B/holzcloud.sqlite?immutable=1" "select count(*) from \"$t\"")"; done
tree_hash() { ( cd "$1" && find . -type f ! -name 'holzcloud.sqlite*' ! -name 'pre-upgrade-*.sqlite' ! -name .readyz ! -name MANIFEST -print0 | sort -z | xargs -0 sha256sum ); }
tree_hash /opt/holzcloud/data   > /tmp/src.sha ; tree_hash "$B" > /tmp/bak.sha ; diff /tmp/src.sha /tmp/bak.sha && echo TREE-EQUAL
test "$(wc -l < /tmp/src.sha)" -gt 0 && grep -q ' ./media/' /tmp/src.sha || echo "BAD: source media empty"
```
(`/tmp` here is the harness's `docker exec` root shell, not a unit with `PrivateTmp`.)

### Bulk media fixture (data point 2), as the service user, outside the database
```bash
# Source: this research; 60 x 5 MiB = 300 MiB of incompressible files. DEVIATION[fixture].
runuser -u holzcloud -- bash -c 'umask 077; d=/opt/holzcloud/data/media/1/bulk; mkdir -p "$d"; for i in $(seq 1 60); do head -c 5242880 /dev/urandom > "$d/bulk-$i.bin"; done'
```
The database does not reference these files; the backup copies `media/` as a tree, so it is the right load for rsync/copy time. Choose the website id from `select id from websites order by id limit 1` as `insert_old_order` does (run.sh:1509), do not hardcode `1`.

### Sizes and counts (inside the container)
```bash
du -sb "$B"; du -sh "$B"; du -sb "$B/holzcloud.sqlite" "$B/media" "$B/templates" "$B/plugins" 2>/dev/null
find "$B" -type f | wc -l ; find "$B/media" -type f | wc -l
```
`du -sb` is apparent bytes and counts each directory as 4096; say so. The VACUUM INTO file is smaller than the live file plus WAL (a freshly migrated empty schema backup measured 630,784 bytes vs 634,880 live, native probe).

### Negative cases (restore.sh refuses before touching data)
```bash
# (a) damaged: truncate a COPY of the backup to half its size, transfer/run restore.sh on it; data dir hash and `systemctl is-active holzcloud` must be unchanged
# (b) no holzcloud.sqlite: restore.sh:33-36 prints "ERROR: <dir>/holzcloud.sqlite not found" and exits 1
# (c) answer "no": restore.sh:52-53 prints "aborted", exit 1, service untouched
```
Source for (b)/(c)/(a) ordering: [VERIFIED: deploy/restore.sh:33-36, 43-53].

### Proving the record is the output
```bash
# Source: this research (generalises DEPLOY-RUN.md's "line-by-line grep -F" sentence); -x makes it exact-line
for f in deploy/test/out/amd64-bak-*/{summary,backup,timer,restore}*.txt; do
  while IFS= read -r l; do [ -z "$l" ] || grep -Fxq -- "$l" deploy/BACKUP-RUN.md || echo "MISSING in record: $f: $l"; done < "$f"
done
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `backup.sh` shelling out to the `sqlite3` CLI, verifying nothing | `holzcloud backup` (VACUUM INTO via pure-Go driver) + `quick_check` before it counts | before this phase (backup.sh header comment) | The binary vouches for its own file; this phase adds an independent reader |
| Restore undocumented and unplayed | `restore.sh` + DEPLOY.md "Restore from a backup" | before this phase; "Restore is only proven if you have run it" (DEPLOY.md:895) | This phase is that proof |
| Timer never fired in any run | Container-only drop-in brings it into reach | this phase (D-11) | Hardened oneshot exercised once |

**Deprecated/outdated:** the DEPLOY.md sentence "Do it once on a test directory before you need it" is superseded by the played record; the `docs/*` claims about "the whole data directory" are outdated until the fix lands.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | After restoring a `backup.sh` backup, the five plugins are not loaded and `check`/`compare_states` fail (consequence read from `Sync`/`note`, not played) | Pitfall 1, Summary | Low: pass 1 plays it either way; if plugins somehow load, the finding downgrades to "operator files only" and the fix scope shrinks |
| A2 | A one-text-byte flip inside a data page is not detected by `quick_check`/`integrity_check` (SQLite has no page checksums) | Pitfall 3 | Low: only influences the choice of corruption; truncation is the recommended case |
| A3 | A CLI-created AI key (`ai key create`) is stored hashed in `ai_tokens` like the OAuth tokens, so a working `list_plugins` after restore also proves the key table | Pitfall 7 | Low: the check still proves plugins/login; the key claim in the record would be dropped |
| A4 | Emulated (Rosetta) `rsync`/`sha256sum` throughput is lower than native; the per-MB rate recorded here is an upper bound on a real server's time | Measurement, D-15 | Medium: extrapolation text must say "estimate"; wrong only in direction of conservatism |
| A5 | Deleting media between the DB snapshot and the media copy can leave a row without its file (ordering reasoning) | Pitfall 11 | Low: DEPLOY.md note only; reword if the admin delete order differs |
| A6 | Bulk media created directly on disk does not trigger any startup scan that would slow or break the restored service | Code Examples | Low: `media.Backfill` iterates DB rows (main.go:521 region); if wrong, put the files under a path the service ignores |

## Open Questions

1. **Fix shape for the missing directories: explicit loop vs. "everything except the SQLite family and snapshots".**
   - What we know: the script's pattern is one `rsync` block per directory; old backups use top-level `media/`, `templates/`. Future directories (e.g. another `DirName`) would be missed again by an explicit list.
   - What's unclear: whether the user prefers the explicit list (smallest diff) or a generic rsync of the data dir with excludes (robust, but changes backup layout for the database/`csrf.key` handling and restore of old backups).
   - Recommendation: explicit loop over `media templates plugins sprachen marke`, plus a one-line comment in `backup.sh` and a unit-test-like assertion in the harness (the whole-tree hash) that fails when a *new* directory appears in the data directory and is not in the backup. D-16 allows this under "fix the defect, add no options".

2. **Second data point: a second fresh target, or a second restore into the same target?** (Claude's discretion.)
   - What we know: `mv` of the data directory to `.before-restore-*` is a same-filesystem rename (no copy cost), so a second restore into the first target has the same timing; a second fresh target is cleaner "empty system" evidence at ~35 s cost.
   - Recommendation: second fresh target for the bulk data point (one `up install` more), so every restore in the record is onto an empty system.

3. **How to play the DEPLOY.md restore line, which carries a sample timestamp.**
   - What we know: the section's only line is `sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20260802-031500`; `doc_cmd` runs a line verbatim, and a placeholder-free sample name will simply fail (README: "A played block carries no angle-bracket value and no carried variable").
   - Recommendation: extend the section so the block lists the directories first (`ls -1 /opt/holzcloud/backups`) and keep the restore line; the harness plays it through a named translation like the `scp` one (a new `DEVIATION[backup-name]`: the sample timestamp replaced by the directory the timer made), with `yes` on stdin. The known-deviation vocabulary check (`A name outside the documented set fails the run`) needs the new name added where the set is defined, and to `deploy/test/README.md`.

4. **Cross-version restore (optional):** the source's backups directory also holds the schema-58 backup made by the old binary in Updating step 1. Restoring *that* into a fresh target with the new binary would show the migrate-on-start path after a restore. Cheap; not required by BAK-01..03; Claude's discretion.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker Desktop | all containers | yes | 29.8.2, host `aarch64`, 10 CPUs, ~8.3 GB RAM | none (blocking) |
| `--platform linux/amd64` image `holzcloud-unit-test:amd64-debian-bookworm-slim-systemd-base` | target/source | yes (cached, 170 MB) | systemd 252 (252.39-1~deb12u2) | `build` recreates it |
| Network (apt: `rsync` etc. from Prerequisites; Caddy not needed here) | `install` | not probed; Phase 21 saw it reachable | — | `DEVIATION[network]` exits 3 |
| Go toolchain on host | `bins` (build old/new binary) | yes | go1.27.2 darwin/arm64 (go.mod wants 1.26.8+) | cached binaries in `deploy/test/out/bin` (incl. `holzcloud-old-v0.0.10`, `holzcloud-old-v0.0.21`) |
| git | `bins` | yes | 2.54.0 | — |
| bash on host | `run.sh` | yes | 5.3.20 (script targets 3.2) | — |
| `sqlite3` in container | independent integrity check, counts | yes | 3.40.1 | — |
| `rsync` in container | both scripts | via Prerequisites `apt install` | — | — |
| Host disk | `deploy/test/out`, binaries | yes | ~55 GiB free of 460 GiB at last probe | — |
| Stale containers named `holzcloud-*` | `up` refuses an existing name | none present at probe (`docker ps -a` showed unrelated ones only) | — | `run.sh down` |

**Missing dependencies with no fallback:** none found. **Missing with fallback:** none. Do not run the source and target chains with the same `NAME`/`OUT_DIR`.

## Security Domain

(`security_enforcement` is absent in `.planning/config.json`, so it applies.)

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (login after restore) | Existing Argon2id hashes restored from the database; assert login with the old password |
| V3 Session Management | yes (CSRF key) | `csrf.key` restored (backup.sh comment: without it sessions/tokens are invalid); assert presence and mode |
| V4 Access Control | yes | Backup dir `700`, owned by the service user (D-12); restored data dir `700`, owner `holzcloud` (restore.sh:82-83); setup form must be closed after restore (404) |
| V5 Input Validation | limited | `restore.sh` takes a path argument and a `yes` answer only; the restored tree is trusted operator data |
| V6 Cryptography | limited | No new crypto; `HOLZCLOUD_SECRET_KEY` is not in the backup (document it) |
| V8 Data Protection | yes | Backups contain password hashes and the CSRF key (DEPLOY.md "Security note"); transfer by container-to-container stream, nothing on the host disk; `leak_check` over out files |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Secret (login password, AI key) leaking into committed evidence | Information disclosure | Keep in root-only files in the container; exec-pipe transfer; `leak_check` extended to the target's files; the pasted-output scan for key-shaped strings/hashes as in Phase 21 |
| Restoring a corrupt snapshot over a good database | Tampering / DoS | `restore.sh` verifies a copy before stopping the service; negative case (a) proves it |
| Backup directory readable by other users | Information disclosure | `chmod 700` in `backup.sh:44`; assert mode on the timer-made directory |
| Restore of a backup with symlinks (`rsync -a` keeps them) | Tampering | Backups are the operator's own trusted artifacts; name it, do not mitigate in this phase [ASSUMED] |
| Setup form left open on a rebuilt box | Elevation of privilege | Assert 200 before restore, 404 after (Pitfall 8) |

## Project Constraints (from CLAUDE.md)

- **GSD workflow:** file edits go through a GSD command (`/gsd-execute-phase` for this planned phase); no direct repo edits outside it.
- **Stack:** Go 1.22+ stdlib-first, plain CSS, htmx only JS, SQLite via `modernc.org/sqlite`, no npm/Node. This phase adds shell and Markdown only; if any Go changed, `go run ./tools/i18n` must report `0 open, 0 orphaned`, and `bash -n deploy/test/run.sh` is a gate Phase 21 used [VERIFIED: 21-VERIFICATION.md item 7 text].
- **Nothing loads at runtime from a third party:** untouched; Docker pulls base images at image build time only (README Safety).
- **Conventions:** snake_case file names; SQL named params `$name`; STRICT tables (no schema change expected); "No new operator-visible features". The i18n rules apply only to Go/template strings; `deploy/*.sh` messages are not collected by `tools/i18n` (reads `cmd/holzcloud/templates/admin`, `internal`, `plugins/` only) — operator-facing wording in the shell scripts is English, matching DEPLOY.md (D-17).
- **Project memory (user):** questions as choices (`AskUserQuestion` with options); git commits as `holzcloud.ch` (noreply), set locally; attribution lines per the session reminder.
- **Hard rule from README:** do not rename harness subcommands/variables/output files; extend only.

## Sources

### Primary (HIGH confidence)
- Repository files opened this session: `deploy/backup.sh`, `deploy/restore.sh`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer`, `deploy/holzcloud.service`, `deploy/DEPLOY.md` (Prerequisites, Install, Backups, Recovery), `deploy/DEPLOY-RUN.md` (structure and proof wording), `deploy/test/README.md`, `deploy/test/run.sh` (header, doc machinery, `install`, `postinstall`, `seed`, `capture_state`, `compare_states`, chains, dispatch), `deploy/test/Dockerfile`, `internal/db/maintain.go`, `cmd/holzcloud/cli.go`, `cmd/holzcloud/main.go`, `internal/plugin/manager.go`, `internal/branding/branding.go`, `internal/i18n/disk.go`, `internal/db/migrations/00021_plugins.sql`, `internal/config/config.go`, `docs/deployment.md`, `docs/configuration.md`.
- Probes run this session: (1) throwaway systemd 252 container with a copy of the backup timer: reset+new `OnCalendar`, `RandomizedDelaySec=0`, `daemon-reload` vs restart, fire on time, `TriggeredBy`, `LAST` semantics; (2) native build of HEAD and `check`/`backup` on a scratch dir: WAL header `0202`, `-readonly` failure on macOS sqlite 3.51, `?immutable=1` OK, three corruption variants refused; (3) the same sqlite probes inside the bookworm image (3.40.1); (4) container-to-container `docker cp - ` stream: modes, mtimes, case-distinct names, numeric owner kept; (5) `read -p` with piped stdin.

### Secondary (MEDIUM confidence)
- systemd.timer(5) man page via manpages.debian.org (search result extracts): `AccuracySec=` default 1 min; empty assignment resets the list; `Persistent=` semantics. (freedesktop.org returned 403 to WebFetch; the stamp-file location was observed directly in the probe, `/var/lib/systemd/timers/stamp-probe.timer`.)

### Tertiary (LOW confidence)
- none relied on.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages; everything is in-repo or already in the image.
- Architecture: HIGH — assembled from existing harness functions, each read this session.
- Pitfalls: HIGH for 2-10 (probed or code-read with quotes); MEDIUM for Pitfall 1 (mechanism read, outcome not yet played) and Pitfall 11 (reasoning).

**Research date:** 2026-10-14
**Valid until:** 2026-11-13 (stable; invalidated if `backup.sh`, `restore.sh`, `run.sh` or `DEPLOY.md` change before planning, or if Docker Desktop/Rosetta behaviour changes)
