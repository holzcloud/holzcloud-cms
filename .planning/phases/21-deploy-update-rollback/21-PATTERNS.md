# Phase 21: Deploy, update, rollback - Pattern Map

**Mapped:** 2026-10-12
**Files analyzed:** 7 (all verified git-tracked)
**Analogs found:** 7 / 7 (no Go code planned; docs and bash only)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `deploy/test/run.sh` (modify: add `install`, `update`, `rollback`, `lifecycle`, helpers `doc_cmd`, `step`) | harness script | batch / request-response via `docker exec` | itself: `cmd_start`, `cmd_seed`, `cmd_all`, dispatch at lines 775-794 | exact |
| `deploy/DEPLOY.md` (modify: Prerequisites, Build, Transfer, Install, First Run, Updating; new "Rolling back"; Recovery cross-links) | doc | n/a | itself (style) plus Backups section lines 560-595 | exact |
| `deploy/DEPLOY-RUN.md` (new; or `deploy/test/DEPLOY-RUN.md`) | evidence record | n/a | `deploy/SECURITY-SCORE.md` (Summary, How measured, Container vs. server table, Appendix raw output) | exact |
| `deploy/test/README.md` (modify: subcommand rows, output files table, hand-over to 22) | doc | n/a | itself: "Output files" table and "Hand-over to phase 22" | exact |
| old/new build helper (inside `run.sh`, or documented commands) | build step | batch | `cmd_build` lines 113-124 plus `release.yml` ldflags | role-match |
| Login-user seeding step (in `update` flow) | harness step | request-response | `cmd_seed` key-creation block lines 404-413 | role-match |
| `cmd/holzcloud/deploy_docs_test.go` (only read, must keep passing) | test | n/a | itself lines 1-30 | guard |

## Pattern Assignments

### `deploy/test/run.sh` new subcommands (harness, batch)

**Analog:** `deploy/test/run.sh` itself. Bash 3.2: no arrays, no mapfile, no `${v,,}`.

**Helpers already there** (lines 57-60), reuse and do not redefine:
```bash
say() { printf '%s\n' "$*" >&2; }
die() { say "run.sh: $*"; exit 1; }
dx() { docker exec "$NAME" "$@"; }
dxi() { docker exec -i "$NAME" "$@"; }
```

**Drift-guard helper to add (RESEARCH Pattern 1)**, `doc_cmd` sits beside `say/die`:
```bash
doc_cmd() { # $1 = the command exactly as DEPLOY.md prints it
  grep -Fqx -- "$1" deploy/DEPLOY.md \
    || die "DEPLOY.md has no line '$1': the document and the script have drifted apart"
  say "STEP: $1"
  printf '%s\n' "$1" | dxi bash -s
}
```
`scp` lines become `docker cp` and are announced as `DEVIATION[scp]` using the same `say` format as `DEVIATION[dropin]` (lines 214-215).

**Per-scenario isolation** (lines 50-55, 126-136): `NAME`, `IMAGE`, `OUT_DIR` are env-overridable. `cmd_up` does `rm -rf "$OUT_DIR"` and refuses an existing container name, so install and update/rollback each get their own, e.g. `NAME=holzcloud-install OUT_DIR=deploy/test/out/amd64-install`. Never chain two scenarios in one `OUT_DIR`.

**Starting the service after literal `systemctl start`** (lines 184-237): `cmd_start` runs `dx systemctl start holzcloud || true`, polls `/healthz`, and on the `mmap_min_addr` signature writes the drop-in, `daemon-reload`, `reset-failed`, `restart`, `wait_ready`. The install flow plays the DEPLOY.md `sudo systemctl start holzcloud` line literally, records the crash, then calls `cmd_start`. After every swap use `wait_ready` (lines 99-111), never `sleep`.

**Guard clauses at the top of each command** (lines 396-399, 621):
```bash
dx systemctl is-active --quiet holzcloud \
  || die "holzcloud is not active; run 'run.sh start' first"
if dx test -e /root/.hc-key; then die "this container is already seeded ..."; fi
```

**Running the CLI as the service user** (lines 410-413), the pattern for `holzcloud user create`, `version`, `backup`:
```bash
dx bash -c 'set +x; umask 077
  out=$(runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data /opt/holzcloud/holzcloud ai key create -name hc-unit-test -level admin) || { echo "ai key create failed" >&2; exit 1; }
  ...' || die "could not create the admin key"
```
For `user create`, the password goes through stdin from an in-container env var and is never echoed to the host or the record (RESEARCH V14).

**Evidence capture** (lines 172-179, 226-235): brace group redirected into `"$OUT_DIR/<name>.txt"`. Probe pattern for which binary runs (RESEARCH Pattern 3):
```bash
dx curl -fsS http://127.0.0.1:8080/readyz | dxi jq -c '{status,version,commit,integrity}'
```
Assertions use `check_pass` / `check_bad` (lines 483-484) with `PASS`/`BAD` prefixes; counting via `count_pass` (line 673).

**Direct sqlite reads** (lines 560-561): `runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "..."`. Use this, never `holzcloud user list`/`check`/`migrate status` on a pre-update DB with the new binary (those call `openForCLI`, cmd/holzcloud/cli.go:103, which migrates silently).

**Chain command pattern** (`cmd_all`, lines 751-773; `cmd_mdwe`, 710-722):
```bash
cmd_all() {
  CLEANUP_DOWN=1
  cmd_build; cmd_up; ...
  say "---- summary ($PLATFORM) ----"
}
```
`lifecycle` follows this shape: set `CLEANUP_DOWN=1`, run steps, end with a summary of `PASS` lines.

**Cleanup trap** (lines 62-78): single `on_exit`; a rollback that has a temporary state (e.g. stopped service) should register restore the same way `MDWE_DROPIN`/`mdwe_restore` do (lines 631-647).

**Dispatch and usage to extend** (lines 80-95 usage text; 775-794 dispatcher): add the new names to both the `usage()` list and the `case` pattern at line 786. Do not rename existing subcommands (header lines 14-16: interface is stable).
```bash
case "$sub" in
  exec | logs) "cmd_$sub" "$@"; exit $? ;;
  build | up | start | wait | canary | down | smoke | seed | check | verdict | mdwe | score | all) "cmd_$sub" ;;
  mdwe-control) cmd_mdwe_control ;;
  *) usage ;;
esac
```

**Old/new build** (lines 113-124): `cmd_build` forwards `VERSION`/`COMMIT` as `--build-arg`. Per research the old tag has no Dockerfile, so build both with host `go build` (literal DEPLOY.md line + `-X main.Version=… -X main.Commit=…`; `git archive v0.0.10 | tar -x -C "$T"`) and `docker cp` into the container, or overlay the current Dockerfile on the old tree. README "Two binaries" requires distinct `IMAGE` per build.

**Seed on OLD binary only** (`cmd_seed`, lines 395-469): reuse unchanged; it dies if `/root/.hc-key` exists (line 398). One-time upload link stays stripped from the log (lines 449-450).

---

### `deploy/DEPLOY.md` (doc, English, matching existing style)

**Analog:** its own Updating (lines 597-617) and Recovery (688-696) sections; Backups (560-595) for command-block style: fenced `bash` blocks with `# comment` lines, `sudo` in front of every privileged command, paths `/opt/holzcloud/...`.

**Current Updating block to rewrite** (lines 604-615):
```bash
# Build new binary on dev machine
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud
# Transfer to the server
scp holzcloud user@your-server:/tmp/holzcloud
# On the server: stop, replace, start
sudo systemctl stop holzcloud
sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
sudo systemctl start holzcloud
```
Target shape (RESEARCH "three server-side blocks"): keep `.prev` copy, `holzcloud version`, backup with the OLD binary via `/opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups`, then stop/replace/start, then `journalctl -u holzcloud --since "10 minutes ago" | grep -E 'pre-upgrade snapshot written|migration applied'`. New "Rolling back" section goes right after Updating (D-11).

**Existing failed-migration text to cross-link** (lines 688-696): the restore command printed by the program copies only the snapshot. Keep consistent: link to "Rolling back", binary first then data, `rm -f` of `-wal`/`-shm`, `systemctl reset-failed holzcloud`.

**Known path inconsistencies to fix** (Pitfall 11): Install copies `/tmp/deploy/backup.sh` to `/opt/holzcloud/backup.sh` (lines 89-90) while Backups and `holzcloud-backup.service` use `/opt/holzcloud/deploy/backup.sh` (line 579); `backups` dir not chowned; `restore.sh` never installed; `rsync` not in Prerequisites; Prerequisites line 10 names `sqlite3` "for backups" (stale); line 15 "Go 1.22 or later" vs `go.mod` 1.26.8; Build line 29 has no `-X main.Version/Commit`; First Run (lines 553-558) omits `HOLZCLOUD_SECURE=true` consequence and compulsory 2FA setup.

**Guard:** `cmd/holzcloud/deploy_docs_test.go` reads `deploy/DEPLOY.md` via `readDeploy` (lines 14-21) and asserts `2.11.2`, `CVE-2026-30851`, `systemctl edit caddy`, `EnvironmentFile=/etc/holzcloud-sso.env` remain and three forbidden phrases stay absent. Run `go test ./cmd/holzcloud -run 'Deploy|Caddyfile'` after edits. Every command line that `run.sh` mirrors must remain one exact line (the `grep -Fx` guard), so do not wrap those with `\` continuations.

**Snapshot facts to quote** (`cmd/holzcloud/main.go` 121-143, `internal/db/maintain.go` 39-41, 247-267): name `pre-upgrade-<NEW Version>-<UTC ts>.sqlite`, written only when migrations are pending, newest 5 kept (`PruneSnapshots(cfg.DataDir, 5)`), journal line `pre-upgrade snapshot written` with `path=`.

---

### `deploy/DEPLOY-RUN.md` (evidence record)

**Analog:** `deploy/SECURITY-SCORE.md`, tracked, sits beside DEPLOY.md.

Sections to copy the shape of: `## Summary` (line 8), `## How this was measured` (22), `## Deviation from D-01` (45), per-requirement sections (`## UNIT-01 …` 70), `## Container vs. server` (211) with the table header `| Aspect | Measured here | Real server | Status |` and Status vocabulary **measured / inferred / assumed**, `## Not measured` (252), `## Appendix: raw output` (269) with `### R1, amd64, bookworm: <file>` subheadings pasting `OUT_DIR` files unchanged.

Findings table required by D-02: `| Step | DEPLOY.md said | What happened | Fix |`. Deviations named as in Phase 20: `DEVIATION[dropin]`, `DEVIATION[scp]`, plus Caddy/DNS/`RequiresMountsFor`. `deploy/test/out/` is gitignored (`.gitignore` line 33), so "raw logs committed" means pasted here. Before commit grep the file for `hc_` and test passwords.

---

### `deploy/test/README.md` (doc)

**Analog:** itself. Add rows to the "Output files" table (`| File | Written by | Content |`, around lines 106-120) for each new file under `OUT_DIR`; update "Hand-over to phase 21" (now done) and "Hand-over to phase 22" bullets "Paths" and "backup.sh and what the image lacks" so they match the DEPLOY.md fixes (Pitfall 11). Keep "Deviations" and "Pitfalls" lists; add new pitfalls there in the same one-line bullet style.

## Shared Patterns

### Run the CLI as the service user with data dir
**Source:** `deploy/test/run.sh` lines 410-413 and `deploy/test/README.md` Pitfalls. **Apply to:** every harness step that calls `holzcloud ...` or writes into the data directory. Root-owned DB files stop the service from opening them.
```bash
runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data /opt/holzcloud/holzcloud <subcommand>
```

### Poll, never sleep; reset failed state between crash experiments
**Source:** `run.sh` `wait_ready` 99-111, `reset-failed` at 212, 636, 658. **Apply to:** every start/restart in install, update, rollback.

### Secrets stay inside the container
**Source:** `run.sh` 404-413 (key to root-only file), 449-450 (upload URL stripped from the log). **Apply to:** login-user password and any seeded key; never into `OUT_DIR` or `DEPLOY-RUN.md`.

### Which binary runs
**Source:** `/readyz` JSON (`version`, `commit`, `integrity`); `holzcloud version` (cmd/holzcloud/cli.go:67) opens no database. **Apply to:** after each swap in update/rollback.

### Raw evidence, deviations named
**Source:** `deploy/SECURITY-SCORE.md` structure; `run.sh` `say "DEVIATION[...]"`. **Apply to:** `DEPLOY-RUN.md` and run output.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| before/after DB comparison step (`ATTACH` of the pre-upgrade snapshot, counts per common table, page and media sha256) | harness step | batch | No existing step compares two databases. Use RESEARCH Pattern 4; run via `dx runuser -u holzcloud -- sqlite3 -readonly`, with the inline-SQL call style of `run.sh` lines 560-561. Discover `pages`/`media` columns with `.schema` first. |
| Negative case (old binary on migrated DB, duplicate `orders.token` insert) | harness step | request-response | Nothing similar exists; RESEARCH Pitfall 2 gives the statement and observed result. |
| Headless First Run (CSRF token + `Referer` + `--data-urlencode`) | harness step | request-response | No existing script posts a form; use RESEARCH Pitfall 6 recipe (`http_code`/`http_body` at run.sh lines 487-488 are the nearest curl helpers). |

## Metadata

**Analog search scope:** `deploy/`, `deploy/test/`, `cmd/holzcloud/` (cli.go, main.go, deploy_docs_test.go), `internal/db/maintain.go`
**Tracked-source check:** `git ls-files` confirmed deploy/*, deploy/test/*, cmd/holzcloud/{cli,main,deploy_docs_test}.go, internal/db/maintain.go. No mirror paths used.
**Files scanned:** about 8 read in full or in part
**Pattern extraction date:** 2026-10-12
