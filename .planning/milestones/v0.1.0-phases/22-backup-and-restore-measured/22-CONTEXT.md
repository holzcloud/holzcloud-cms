# Phase 22: Backup and restore, measured - Context

**Gathered:** 2026-10-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Prove that a backup made by `deploy/backup.sh` can be restored by `deploy/restore.sh` onto an
empty system, and record what it cost (BAK-01, BAK-02, BAK-03): integrity check and content of
the backup, a restore that yields working pages, media and login with time and size recorded,
and one observed run through `holzcloud-backup.timer`. Deliverables are recorded evidence,
repeatable scripts under `deploy/test/`, and `deploy/DEPLOY.md` corrected where the played run
disagrees with it. No new operator-visible features; script or code changes only if the played
run exposes a defect that blocks a criterion.

</domain>

<decisions>
## Implementation Decisions

The user delegated all decisions for this phase to Claude ("Entscheide du alles"), as in
Phase 21. Everything below is Claude's call, made conservative and evidence-first.

### What "an empty system" is, and the source site (BAK-02)
- **D-01:** The restore target is a **fresh container**: `TARGET=systemd-base`, a new `NAME`, DEPLOY.md's install played through `install` (as in Phase 21), **no data carried over**. It is a different container from the one that was backed up, so the proof cannot lean on state that survived in place. The service of the new container may have started once and written an empty database; `restore.sh` moves that aside, which is the realistic situation on a rebuilt server.
- **D-02:** The site that is backed up is the populated one Phase 21 leaves: `INSTALL_FROM=old ... build up install baseline update` (website, published page, five plugins, image, login user, order, marker page, pre-upgrade snapshot). Reuse it; do not seed a second, different fixture.
- **D-03:** The backup that is **restored is the one the timer produced** (D-12), not a separate manual one. That makes BAK-03 and BAK-02 one chain: timer → backup → transfer → restore. A manual `backup.sh` run still happens for BAK-01's integrity assertions and is recorded separately.
- **D-04:** Transfer between the two containers goes **through the host with `docker cp`** (the same stand-in for `scp` that Phase 21 D-01 uses) and is named as a deviation from a real server, where it would be `rsync`/`scp`.
- **D-05:** `restore.sh` is run **unchanged and interactively as designed**: the answer `yes` comes from stdin. No `--yes` flag is added (see D-16).

### Proof of integrity and content (BAK-01, BAK-02)
- **D-06:** "Passes its integrity check" is shown twice, by two tools: the binary's own check (`holzcloud check`, which `restore.sh` already runs on a copy) **and** an independent `sqlite3 -readonly ... PRAGMA integrity_check` on the backup file. A backup that two different readers accept is stronger than one that the writer vouches for.
- **D-07:** "Contains database and media" is shown by **comparison, not by listing**: row counts per table that exist in both, a `sha256` list of every file under `media/` (and `templates/`, `csrf.key`) in the source data directory against the backup directory and against the restored data directory. Raw outputs committed. An empty `media/` directory would pass a listing and must fail here, so the source is asserted non-empty first.
- **D-08:** "Working pages, media and login" after the restore = the public page renders and carries the marker content, the media file is served with the same hash, the login user logs in with the password it had before (session via the real login form, with the CSRF behaviour Phase 21 already documented), and `check` passes against the restored container. `/root/.hc-key` lives outside the data directory and is **not** part of a backup (README hand-over): the run states what is rebuilt by hand on the new box and what comes back from the backup.
- **D-09:** **Negative cases are played once and recorded:** (a) a backup whose `holzcloud.sqlite` has been damaged must be refused by `restore.sh` **before** the live data is touched (the data directory is unchanged afterwards); (b) a backup directory without `holzcloud.sqlite` is refused. The doc states facts, not the comment in the script.
- **D-10:** `backup.sh` gets **no new MANIFEST fields or checksum files** unless the played run shows that a restore cannot tell a truncated backup from a whole one. The comparison in D-07 is the proof; a feature is only added if it is the fix for a defect.

### Timer run (BAK-03)
- **D-11:** The timer is brought into reach by a **container-only drop-in on `holzcloud-backup.timer`** that replaces `OnCalendar` with a time a few seconds to a minute ahead and sets `RandomizedDelaySec=0`. The unit files in `deploy/` are not edited; the drop-in is named as a **deviation** in the record (alongside the `ProcSubset=all` drop-in of Phase 20). The clock is not changed: it would also move the journal, the `-mtime` retention and the `Persistent=true` bookkeeping.
- **D-12:** "Observed" means three artefacts, all committed raw: `systemctl list-timers holzcloud-backup.timer` before and after (the NEXT/LAST columns moved), `journalctl -u holzcloud-backup.service` for that one invocation (started by the timer, not by hand, exit status 0), and the **new backup directory** it produced under `/opt/holzcloud/backups`, owned by the service user with mode 700. The hardening of `holzcloud-backup.service` (`ProtectSystem=strict`, `ReadWritePaths`) is exercised for real here, which is the point.
- **D-13:** What the drop-in cannot prove — the real 03:15 window, `Persistent=true` after a missed run, `RandomizedDelaySec` — is **named as not tested**, not claimed.

### Measurement (BAK-02)
- **D-14:** Record, for the restore and for the backup: wall time (`date +%s.%N` around the step, not `time` of a pipeline), size of the backup directory (`du -sb` and `du -sh`), size of database and media separately, and the file counts. Two data points: the **seed site as it is**, and the same site with **synthetic media added** (order of 100–500 MB of non-compressible files, planner picks) so that the time is not dominated by service start and the per-MB rate can be read.
- **D-15:** Times come from the **amd64 emulation** (Phase 20 D-01/Phase 21 D-13) and say so in the record, next to the host architecture. Any extrapolation to a real server is written as an estimate with its assumption (rsync/disk-bound, linear in size), never as a measurement. The restore time is split: verify, stop+move+copy, start to `/readyz`.

### Script changes and defects
- **D-16:** If the played run exposes a defect, **fix the script in this phase** and record step / what happened / fix, as Phase 21 D-02 does for the document. Likely candidates to *look at*, not to change in advance: stale `-wal`/`-shm` after the restore (`restore.sh` moves the whole data directory aside, so probably none — verify), `chown`/mode of the restored tree, `MIN_FREE_MB=1024` on the container disk (override via env and name it), the non-atomic pair "database snapshot first, media copied after" (DB rows never reference a file the backup lacks, extra media files are harmless — state this in DEPLOY.md if it is not there). Changes stay minimal and do not add options.
- **D-17:** `deploy/DEPLOY.md` is the source of truth. Its Backups section is extended or corrected so that a reader needs nothing from this phase's record: the restore command, the `yes` prompt, what comes back and what does not (`/root/.hc-key`-like material outside the data directory, anything in `/etc`), the measured size/time as an order of magnitude with the emulation caveat, and the non-atomic note from D-16. Wording in English, matching the document.
- **D-18:** Evidence follows Phase 21: repeatable steps as new `run.sh` subcommands or chain steps (build/up/start separable), raw logs under `deploy/test/out/`, and a committed record **`deploy/BACKUP-RUN.md`** that pastes them unchanged, with the same line-by-line `grep -F` proof as `DEPLOY-RUN.md`. The record lists deviations (D-04, D-11, emulation) first. Do not edit `deploy/DEPLOY.md` between the chains and the record.

### Claude's Discretion
All of the above, plus: exact names of the new subcommands, how row counts and hashes are collected, size of the synthetic media, the time offset of the timer drop-in, whether the damaged-backup case truncates or flips bytes, wording of doc fixes, and whether the target container is built on bookworm or trixie (`BASE_IMAGE`; keep the Phase 21 default).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Backup tooling under test
- `deploy/backup.sh` — snapshot via `holzcloud backup`, csrf.key, media, templates, MANIFEST, retention, optional rsync
- `deploy/restore.sh` — verifies a copy, asks `[yes/NO]`, stops service, moves data aside, restores, waits for `/readyz`
- `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer` — units (03:15, `Persistent=true`, `RandomizedDelaySec=300`, hardened oneshot)
- `deploy/DEPLOY.md` — "Backups" section (and Prerequisites naming `rsync`, Install, Updating/Rolling back); the document under correction
- `deploy/DEPLOY-RUN.md` — Phase 21 record; model for the new record and the `grep -F` proof

### Test recipe (Phase 20/21 hand-over)
- `deploy/test/README.md` — "Hand-over to phase 22" and Pitfalls (WAL/SHM, CLI as service user, `/root/.hc-key`, login via `holzcloud user create`, `up` resets OUT_DIR, polling `/healthz`)
- `deploy/test/run.sh`, `deploy/test/Dockerfile` — recipe to extend (`postinstall` already enables the timer and runs one manual backup)
- `deploy/SECURITY-SCORE.md` — container-vs-server differences and the `ProcSubset=all` drop-in
- `.planning/phases/20-hardened-unit-under-real-systemd/20-CONTEXT.md` — D-01..D-03a (amd64 emulation, container shape)
- `.planning/phases/21-deploy-update-rollback/21-CONTEXT.md` — D-01..D-13 (literal doc playing, findings record, scripts + raw logs)

### Code
- `internal/db/maintain.go` — `Backup` (VACUUM INTO + integrity check), pre-upgrade snapshot
- `cmd/holzcloud/cli.go` — `cmdBackup`, `check` and `user create` subcommands

### Planning
- `.planning/REQUIREMENTS.md` — BAK-01..03; out of scope: off-site backup service, replication
- `.planning/ROADMAP.md` §Phase 22 — success criteria

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `run.sh` targets and chains: `build up install postinstall`, and `INSTALL_FROM=old ... build up install baseline update` for a populated, updated site; `lifecycle` with `KEEP=1`.
- `postinstall` already copies the backup units, enables the timer and runs one manual backup as the service user.
- `holzcloud user create -email ... -role admin` for the login user; `check` (17 checks) as an after-restore assertion.
- `restore.sh` already verifies the snapshot on a copy before touching live data and moves the old data aside.

### Established Patterns
- Play DEPLOY.md literally; deviations named, never skipped silently.
- Raw logs under `deploy/test/out/`, committed record that pastes them unchanged and proves it with `grep -F`.
- Run the CLI as the service user with `HOLZCLOUD_DATA_DIR` set; poll, never sleep.

### Integration Points
- The timer has never fired in any run; only a drop-in or an equivalent brings it into reach.
- A second container is new for the harness (backup source ≠ restore target): `NAME`/`OUT_DIR` pairs can share `BIN_DIR` (README Pitfalls).
- Phase 23's beta criteria will cite the record.

</code_context>

<specifics>
## Specific Ideas

- "A backup that has been restored is the only backup" (ROADMAP goal): the restore into a *different, empty* container is the heart of the phase.
- The chain timer → backup → docker cp → restore → login is the single story the record should tell.

</specifics>

<deferred>
## Deferred Ideas

- Off-site copy as a tested path (`REMOTE_TARGET` rsync): REQUIREMENTS lists an off-site backup *service* as out of scope; `backup.sh` keeps naming rsync as an operator decision. At most the document says it was not played.
- An atomic DB-plus-media snapshot (filesystem snapshot / quiesce): new capability, own phase if ever needed.
- A non-interactive `restore.sh --yes` or a restore subcommand in the binary: new surface with its own test burden.
- Testing the real 03:15 window, `Persistent=true` catch-up and `RandomizedDelaySec` on a real server: needs a real host and days of wall time.

</deferred>

---

*Phase: 22-Backup and restore, measured*
*Context gathered: 2026-10-12*
