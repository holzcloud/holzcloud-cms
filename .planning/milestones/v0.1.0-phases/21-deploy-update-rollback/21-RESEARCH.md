# Phase 21: Deploy, update, rollback - Research

**Researched:** 2026-10-12
**Domain:** Operating a single-binary Go/SQLite service under systemd: a literal install from a document, a schema-changing update, a snapshot rollback; a bash/Docker harness that plays them
**Confidence:** HIGH for what the code does (read this session and played on the host); MEDIUM for how it behaves inside the Debian/amd64 container (not yet played there)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

The user delegated all decisions for this phase to Claude ("du kannst alles selber entscheiden").
Everything below is therefore Claude's call, made to be conservative and evidence-first.

#### Fresh install (DEP-01)
- **D-01:** Play the install from `TARGET=systemd-base` (Phase 20 hand-over): DEPLOY.md is followed **step by step, literally**, each step through `run.sh exec`; `scp` steps become `docker cp`, `sudo` commands are pasted unchanged. The agent must not use knowledge that is not in the document — a step that only works because of outside knowledge is a deviation.
- **D-02:** Every deviation found (missing step, wrong path, wrong order, unexplained prerequisite, command that fails) is **fixed in DEPLOY.md in the same phase** and listed in a findings record (`deploy/test/DEPLOY-RUN.md` or section in the phase summary) with: step, what the doc said, what happened, fix.
- **D-03:** Steps that cannot run in the container (Caddy/HTTPS with a public domain, DNS, real mount for `RequiresMountsFor`) are **named as deviations, not skipped silently**. Caddy is verified at most as `caddy validate` of `Caddyfile.example` if installable; the HTTP check is against `127.0.0.1:8080` inside the container. Reuse the container-only drop-in that `run.sh start` applies.
- **D-04:** "Works" = unit active, `/healthz`-style endpoint (see DEPLOY.md "Health checks") answers, First Run section completes (admin user created, login possible).

#### Update across a migration (DEP-02)
- **D-05:** Use **two real builds**: an older tagged release as "old" and the current HEAD as "new", built with different `VERSION`/`COMMIT` build args (Phase 20 recipe). Choose the old tag so that **at least one migration lies between the two** (the planner picks the tag and states the migration count; more is better than one). Synthetic migrations are not used.
- **D-06:** Seed data on the old version (website, domain, published page, uploaded image, a user with login, plugin installed — reuse `seed`; add a login user via `holzcloud user`). "No data loss" is shown by **before/after comparison**: row counts per table that exist in both schemas, content hash of the page and media file, successful login, page renders publicly. Raw outputs committed.
- **D-07:** The update procedure played is exactly the DEPLOY.md "Updating" section; the pre-upgrade snapshot the service takes (`internal/db/maintain.go`, `pre-upgrade-<version>-<ts>.sqlite`) is observed and its name/location recorded. Fix the section where it is silent (e.g. back-up-first as a concrete command, order stop → replace → start, what to look for in the journal).

#### Rollback (DEP-03)
- **D-08:** The **documented rollback path is: stop → restore the pre-upgrade snapshot over the live database → put the old binary back → start.** This is the supported route because it is the only one that restores both binary and data consistently. It is played after the update from D-05 and the result (old version runs, pre-update data present, data written after the update is gone) is recorded.
- **D-09:** State the **migration limit explicitly in DEPLOY.md**: after a newer binary has migrated the schema, an older binary must not be started against that database; going back means restoring the snapshot and **losing everything written since the update**. Also play the negative case once — old binary against the migrated database — and record the actual behaviour (refuses vs. misbehaves) so the doc states fact, not assumption.
- **D-10:** `goose down` is **not** offered as the operator rollback path. Whether Down sections exist (60 of 60) is mentioned at most as a developer note; it is not tested as a supported route in this phase. Rationale: untested data-destroying Down migrations are a worse promise than a snapshot.
- **D-11:** Rollback documentation lives as a new "Rolling back" section next to "Updating" (not only under Recovery), and the existing "A failed migration" recovery text is cross-linked and kept consistent.

#### Evidence and automation
- **D-12:** Played runs are committed as **repeatable scripts** under `deploy/test/` (new subcommands or steps for `run.sh`, keeping build/start/exec separable as in D-02 of Phase 20) plus the raw logs under the existing `deploy/test/out/` convention. Reason: Phase 22 and the 0.1.0 beta criteria benefit from a rerunnable proof. Scripts must not become a second installer: DEPLOY.md remains the source of truth and the script only mirrors its steps.
- **D-13:** Platform stays **linux/amd64 via emulation** (Phase 20 D-01); container-vs-server differences relevant to install/update/rollback are named in the findings.

### Claude's Discretion
All of the above, plus: choice of old tag, exact file names of records, how row counts/hashes are collected, whether Caddy validation is feasible, wording of doc fixes (English, matching DEPLOY.md's style).

### Deferred Ideas (OUT OF SCOPE)
- Supporting `goose down` as an operator rollback command — would be a new capability with its own test burden.
- Automatic rollback on failed health check after update — new feature, own phase.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DEP-01 | Eine frische Installation nach `deploy/DEPLOY.md` gelingt Schritt für Schritt; gefundene Abweichungen sind in der Doku korrigiert. | "Predicted deviations" list (DEPLOY.md vs. what the container/code do), the First-Run curl recipe verified on the host, harness design (`doc_cmd` drift guard), Environment Availability (gpg/rsync/fuser missing in the image) |
| DEP-02 | Ein Update auf eine neue Version samt Migrationen gelingt ohne Datenverlust. | Old tag `v0.0.10` (2 migrations, 58 to 60, one data-touching), snapshot behaviour observed, before/after comparison method (`ATTACH` of the pre-upgrade snapshot), CLI-migrates-silently pitfall |
| DEP-03 | Ein Rollback auf die Vorversion (Binär und Datenbank) ist gespielt und dokumentiert, einschließlich der Grenze bei Migrationen. | Snapshot rollback played on the host; negative case observed (old binary starts silently on a newer schema, and the next order insert fails); WAL/SHM hazard with SQLite citation; old binary is overwritten by the documented update |
</phase_requirements>

## Summary

Nothing needs to be installed or invented for this phase: no new Go packages, no new test framework. The work is (1) following `deploy/DEPLOY.md` literally in the Phase 20 container started from `TARGET=systemd-base`, (2) a real update `v0.0.10` to HEAD that crosses two migrations, (3) a snapshot rollback and the negative case, and (4) rewriting `Updating` and adding `Rolling back`. The harness is `deploy/test/run.sh` (bash 3.2-safe, subcommand interface that must be extended, not renamed).

The most valuable findings are the ones that change what the doc must say, all of them reproduced on the host this session with two real binaries (`v0.0.10` and HEAD) against a throw-away data directory:

1. **The old binary does not refuse a newer schema.** Started against a database at schema version 60, `v0.0.10` logs no migration, answers `/healthz` and `/readyz` with `ok`, and runs. The migration limit is therefore not enforced by the program; it is a promise the document has to make, and the failure shows up later (see 2).
2. **What breaks is the first write the old code does not know about.** Migration 00060 adds `orders.token TEXT NOT NULL DEFAULT ''` with a `UNIQUE` index. An `INSERT` that does not name `token` (what `v0.0.10` does) succeeds once and fails the second time with `UNIQUE constraint failed: orders.token`. That is the concrete "misbehaves, silently, later" fact for the doc.
3. **The documented update destroys the way back.** `Updating` overwrites `/opt/holzcloud/holzcloud`; nothing keeps the old binary. A rollback that "puts the old binary back" has nothing to put back unless the doc says to copy it aside first.
4. **The snapshot is named after the NEW version and holds the OLD schema.** `pre-upgrade-<Version>-<ts>.sqlite` uses the `Version` of the binary that took it (the new one). It is written only when migrations are pending, and a fresh install takes none (observed: a first start on an empty directory left no snapshot).
5. **Every CLI subcommand migrates.** `openForCLI` runs `RunMigrations` with no snapshot. `holzcloud user list`, `check`, `backup`, `migrate status` with the NEW binary against the old database migrate it silently. "Back up first" must be done with the OLD binary, before it is replaced.
6. **The restore command printed by the program and quoted in DEPLOY.md is incomplete.** It copies the snapshot over `holzcloud.sqlite` without removing `-wal`/`-shm` (SQLite lists "Overwriting a database file with another without also deleting any hot journal associated with the original database" as a corruption cause) and without putting the old binary back, so the restarted service would run the new binary and fail the same migration again.

**Primary recommendation:** Pick `v0.0.10` as the old build, cross-compile both binaries with the literal DEPLOY.md `go build` line plus `-X main.Version/Commit`, `docker cp` them into the container, and make `run.sh` gain `install`, `update`, `rollback` (and a `lifecycle` chain) whose every command line is checked with `grep -F` against DEPLOY.md before it runs, so the script can only mirror the document and never drift from it.

## Architectural Responsibility Map

This is an operations phase, so the "tiers" are the operator's layers rather than web tiers.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Build the binary, version stamping | Operator workstation (`go build`) | CI release workflow | DEPLOY.md builds on the dev machine; `release.yml` stamps `-X main.Version=$(git describe --tags --always) -X main.Commit=…` |
| Install, replace binary, keep previous binary | Server filesystem + `sudo` (documented commands) | — | Nothing in the program does this; the doc owns it |
| Process lifecycle, restart limits | systemd (`holzcloud.service`) | — | `Restart=on-failure`, `StartLimitBurst=5`; a failing migration becomes a restart loop |
| Pre-upgrade snapshot | Service binary (`cmd/holzcloud/main.go`, `internal/db/maintain.go`) | — | Taken at start, only if migrations are pending |
| Schema migration | Service binary (goose, embedded) | CLI subcommands (`openForCLI`) | Both the server start and every CLI call migrate |
| Rollback of data | Operator (copy snapshot over the database) | — | The only consistent route (D-08); the program has no rollback command |
| Rollback of the binary | Operator (copy `.prev` back) | — | Requires a copy kept by the update step |
| Liveness / identity of the running build | `/healthz`, `/readyz` (`version`, `commit`, `integrity`) | `holzcloud version` | `/readyz` JSON carries `version` and `commit`: use it to prove which binary runs |
| Evidence collection, replay | `deploy/test/run.sh` + Docker (`systemd-base`/`unit-test`) | `deploy/DEPLOY-RUN.md` record | Phase 20 recipe; amd64 under emulation |
| TLS, Caddy | Not testable in the container (named deviation) | `caddy validate` at most | D-03 |

## Standard Stack

No new packages. Everything is already in the repository or on the host.

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| `deploy/test/run.sh` | in repo (Phase 20) | Build, up, start, exec, seed, check against a systemd container | The agreed recipe; Phase 22 reuses it. Extend, never rename (README: "the interface below is stable") |
| Docker | 29.8.2, aarch64 host `[VERIFIED: docker info]` | The systemd test container, `--platform linux/amd64` | Phase 20 D-01/D-13 |
| Go toolchain (host) | go1.27.2 darwin/arm64 `[VERIFIED: go version]`; `go.mod` says `go 1.26.8` `[VERIFIED: go.mod:3]` | Cross-compile both binaries literally as DEPLOY.md's Build step does | The old tag's `go.mod` says `go 1.26.6` `[VERIFIED: old tree go.mod]`; `GOTOOLCHAIN=auto` is the local setting `[VERIFIED: go env GOTOOLCHAIN]` |
| `git archive <tag> \| tar -x -C <dir>` | git | Extract the old tree without touching the work tree or `.git` | A `git worktree` would write metadata into `.git`; `archive` does not |
| `sqlite3` CLI | in the image `[VERIFIED: command -v inside the image]`; on the host too | Row counts, `ATTACH` comparison, schema version | `run.sh check` already uses it with `-readonly` |
| `curl`, `jq`, `sha256sum`, `runuser` | in the image | HTTP probes, hashes, running CLI as the service user | Already used by `run.sh seed`/`check` |

### Supporting
| Tool | Purpose | When to use |
|------|---------|-------------|
| Existing `run.sh seed` | Website, domain, five plugins, page, image, admin API key | On the OLD binary, before the update (D-06) |
| `holzcloud user create -email … -role admin` (password on stdin) | The login user `seed` does not create | After `seed`, as `runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data …` |
| `apt` packages `gnupg`, `rsync`, `psmisc` | Needed by DEPLOY.md steps that the image lacks (see Pitfalls); `caddy` for D-03 | Install them only as a recorded deviation or, better, add them to DEPLOY.md's prerequisites |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Host `go build` for both binaries | Two `run.sh build` runs with `VERSION`/`COMMIT` (CONTEXT D-05 wording) | The old tag has no `deploy/test/Dockerfile`, so a Docker build of the old tag needs the current Dockerfile overlaid on the old tree. Host `go build` is literally DEPLOY.md's Build step (a free test of that section) and needs no overlay. If the planner prefers the Docker route, build HEAD with `VERSION`/`COMMIT` and take the binary with `docker create` + `docker cp` |
| `docker cp` of the binaries | A bind mount | Phase 20 Safety: "No host path is mounted"; keep it |

**Installation:** none (no `npm`/`go get`).

**Version verification:** not applicable for libraries. Tag facts, verified with git this session: tags `v0.0.1` through `v0.0.25` exist; `internal/db/migrations/` has 58 files at `v0.0.1` through `v0.0.10`, 59 at `v0.0.15` and `v0.0.20`, 60 at `v0.0.22` onward; `git diff --name-status v0.0.10 HEAD -- internal/db/migrations` shows exactly `A 00059_website_mail.sql` and `A 00060_order_token.sql`; `v0.0.21` already equals HEAD in migrations `[VERIFIED: git ls-tree / git diff]`. `v0.0.10` = commit `ec365d6`, `v0.0.25` = `7878f57`, HEAD at research time `d86f480` `[VERIFIED: git log / rev-parse]`.

## Package Legitimacy Audit

No external packages are added to the project by this phase (no `go get`, no npm). `gsd-tools package-legitimacy` was therefore not run.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

Operating-system packages that a played run may install inside the throw-away container (`gnupg`, `rsync`, `psmisc`, `caddy` from the Caddy apt repository named in DEPLOY.md) are Debian/vendor packages, not language-ecosystem packages. They are tagged `[ASSUMED]` as to their current repository state: the planner should gate the Caddy install behind the D-03 judgement (install only if network access works; otherwise name it a deviation).

## Architecture Patterns

### System Architecture Diagram

```
operator host (macOS, Docker Desktop, linux/amd64 emulation)
  |
  | 1. git archive v0.0.10 -> tmp tree ; go build (literal DEPLOY.md line + -X Version/Commit)
  |    go build HEAD (same line)                      -> holzcloud-old , holzcloud-new
  v
run.sh build up (TARGET=systemd-base | unit-test)  -> privileged systemd container, no ports, no mounts
  |
  |  A. INSTALL  (systemd-base)        B. UPDATE + ROLLBACK (unit-test image, old binary swapped in)
  |  docker cp holzcloud + deploy/       docker cp holzcloud-old -> /opt/holzcloud/holzcloud
  |  doc_cmd: useradd, mkdir, cp,        run.sh start ; seed ; user create          [state S0, schema 58]
  |  chown, cp unit, daemon-reload,      record: counts, hashes, login, page, /readyz version
  |  enable, start                       DEPLOY.md "Updating": cp .prev ; backup ; stop ; cp new ; start
  |        |                                  |
  |        v                                  v
  |  start crashes (Rosetta/ProcSubset)  service: HasPendingMigrations -> pre-upgrade-<new>-<ts>.sqlite
  |  -> run.sh start adds drop-in             -> RunMigrations 59,60 -> serve         [state S1, schema 60]
  |  First Run via curl (Referer+token)  record: same probes + ATTACH diff old snapshot vs live
  |  /healthz /readyz, login 303              |
  |                                           +--> NEGATIVE: stop ; cp old binary ; start (DB stays at 60)
  |                                           |     record: starts silently; 2nd order INSERT fails UNIQUE token
  |                                           +--> ROLLBACK: stop ; cp .prev ; rm -wal -shm ; cp snapshot ; start
  |                                                 record: /readyz version old, schema 58, pre-update data present,
  |                                                         post-update write gone
  v
deploy/DEPLOY.md (fixed in place)  +  deploy/DEPLOY-RUN.md (findings + raw output pasted)  +  run.sh subcommands
```

### Recommended Project Structure
```
deploy/
├── DEPLOY.md                 # fixed in place (Prerequisites, Build, Install, First Run, Updating, new Rolling back, Recovery cross-links)
├── DEPLOY-RUN.md             # NEW: findings table (step / doc said / happened / fix) + raw run output
└── test/
    ├── run.sh                # + install, update, rollback, lifecycle subcommands; helpers doc_cmd, step
    ├── README.md             # + rows for the new subcommands/variables, hand-over to 22 updated
    └── out/                  # gitignored scratch; raw logs are pasted into DEPLOY-RUN.md
```

`deploy/test/DEPLOY-RUN.md` (CONTEXT's name) or `deploy/DEPLOY-RUN.md` are both allowed (Claude's discretion on file names). Phase 20's equivalent, `deploy/SECURITY-SCORE.md`, sits beside DEPLOY.md and pastes the raw files; follow that.

### Pattern 1: Mirror with a drift guard (keeps DEPLOY.md the source of truth, D-12)
**What:** Each script step names the command exactly as DEPLOY.md prints it, and the script refuses to run a command that is not a line of DEPLOY.md.
**When to use:** every `install`/`update`/`rollback` step that mirrors a documented command.
**Example:**
```bash
# Source: pattern for deploy/test/run.sh (bash 3.2-safe: no arrays, no mapfile)
doc_cmd() { # $1 = the command exactly as DEPLOY.md prints it
  grep -Fqx -- "$1" deploy/DEPLOY.md \
    || die "DEPLOY.md has no line '$1': the document and the script have drifted apart"
  say "STEP: $1"
  # steps run as root in the container, sudo is installed there, so the line is pasted unchanged
  printf '%s\n' "$1" | dxi bash -s
}
```
Lines that DEPLOY.md does not contain on their own (an `scp`, a comment, a multi-line `\` continuation) are the deviations: a `docker cp` replaces `scp`, and the script names that as `DEVIATION[scp]` in its output, as `start` already does for `DEVIATION[dropin]`.

### Pattern 2: One container per scenario, one OUT_DIR per scenario
`up` runs `rm -rf "$OUT_DIR"` and refuses an existing container name (`run.sh` `cmd_up`). Install, and update-plus-rollback, are different starting points (`systemd-base` versus `unit-test`), so give each its own `NAME`, `OUT_DIR` and (automatic) `IMAGE` tag, e.g. `NAME=holzcloud-install OUT_DIR=deploy/test/out/amd64-install`. Never chain a second scenario after the first in the same `OUT_DIR`.

### Pattern 3: Prove which binary runs from the server itself
`/readyz` returns `version`, `commit`, `integrity` `[VERIFIED: internal/web/health.go json tags "version", "commit", "integrity"]` and observed on the host: `{"status":"ok","version":"v0.0.10","commit":"old","integrity":"ok"}`. Use `jq -c '{status,version,commit,integrity}'` after every start; also `holzcloud version` prints `holzcloud <Version> (<Commit>)` without opening the database `[VERIFIED: cmd/holzcloud/cli.go "version" case]`.

### Pattern 4: Compare before/after against the snapshot itself (D-06)
The pre-upgrade snapshot is, by construction, a verified copy of the old state. After the update, stop the service (clean stop: no WAL left) and compare it to the live database in one `sqlite3` session:
```bash
# Source: sqlite3 ATTACH; names of the tables come from sqlite_master, nothing is hand-listed
sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite <<'SQL'
ATTACH '/opt/holzcloud/data/pre-upgrade-<version>-<ts>.sqlite' AS old;
SELECT m.name, (SELECT 1 FROM old.sqlite_master o WHERE o.type='table' AND o.name=m.name) AS in_old
FROM main.sqlite_master m WHERE m.type='table' AND m.name NOT LIKE 'sqlite_%';
SQL
# then generate one "SELECT 'T', (SELECT count(*) FROM main.T), (SELECT count(*) FROM old.T)" per common table
```
Tables that legitimately move with traffic (sessions, activity log, outbox, plugin_store, the goose table) are named in the record as expected differences; the content tables (websites, domains, pages, media, users, orders, plugins) must be equal. Tables present only in the new schema (`website_mail`) are listed, not compared. Content hash of the page: pipe the stored text through `sha256sum` inside the container (SQLite has no built-in hash); media: `sha256sum` of the file under `/opt/holzcloud/data/media`. Column names of `pages`/`media` were not read this session: discover them with `.schema` first.

### Anti-Patterns to Avoid
- **Looking at the old database with the new binary.** `holzcloud user list`, `check`, `backup`, `migrate status` all call `openForCLI`, which migrates with no snapshot `[VERIFIED: cmd/holzcloud/cli.go openForCLI → db.RunMigrations]`. Inspect with `sqlite3 -readonly` instead.
- **Seeding after the swap.** `seed` creates its key with whatever binary is on disk (`holzcloud ai key create`). Seed while the OLD binary is installed.
- **Treating `systemctl start` success as a pass.** `Type=simple` returns before the process can crash; the Rosetta crash comes 8 to 10 seconds into emulation. Poll `/healthz` (`wait`), never sleep.
- **Restoring a database file without its WAL/SHM.** See Pitfall 4.
- **Committing `deploy/test/out/`.** It is gitignored (`.gitignore` line 33 `/deploy/test/out/`), CONTEXT's "raw logs committed" is met by pasting them into `DEPLOY-RUN.md` as Phase 20 did, proven with a `grep -F` loop.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| A consistent copy of a live SQLite database | `cp` of the file | `holzcloud backup <path>` (`VACUUM INTO` + `quick_check`) or the service's own pre-upgrade snapshot | `cp` of a WAL database in use is exactly the corruption case SQLite documents |
| Proving which build runs | Parsing the journal | `/readyz` `version` + `commit` | Already in the JSON |
| Waiting for the service | `sleep N` | `run.sh wait` (polls `/healthz`) | Emulated start takes 8 to 10 s, varies |
| Setting up the amd64-emulation crash workaround | A new drop-in in the new subcommands | `run.sh start` (applies `ProcSubset=all` only on the `mmap_min_addr` signature) | One definition of the container-only deviation |
| An installer | A shell script that "does the install" | `doc_cmd` mirroring DEPLOY.md lines | D-12: DEPLOY.md stays the truth |
| A rollback command / `goose down` | `holzcloud migrate down` | The documented snapshot copy | D-10, deferred |
| Seeding data | New tables of fixtures | `run.sh seed` plus `holzcloud user create` | Already proven in Phase 20 |

**Key insight:** every consistency guarantee in this phase already exists in a tool the repository ships (snapshot, backup, health JSON, seed). The phase's output is a procedure and evidence for it, not new mechanism.

## Runtime State Inventory

Not a rename/refactor/migration-of-names phase. Omitted deliberately. The one relevant piece of "state that survives a binary swap", for the rollback section: `csrf.key` (not touched by any migration), sessions (stored in SQLite, so restored to their pre-update contents by the snapshot), media and templates on disk (not covered by the snapshot; files added after the update remain, harmlessly orphaned).

## Common Pitfalls

### Pitfall 1: The documented update throws away the old binary
**What goes wrong:** `Updating` runs `sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud` over the only copy. D-08's "put the old binary back" is then impossible unless an operator rebuilds an old tag.
**Why it happens:** the section was written for roll-forward only.
**How to avoid:** the fixed section copies it aside first (`sudo cp -p /opt/holzcloud/holzcloud /opt/holzcloud/holzcloud.prev`) and records `holzcloud version` before replacing. A released binary also exists: `release.yml` publishes `holzcloud-linux-amd64` with a `.sha256` `[VERIFIED: .github/workflows/release.yml "go build … -o holzcloud-linux-amd64"]`; the doc may mention it as an alternative source of the old binary.
**Warning signs:** the rollback step has no file to copy.

### Pitfall 2: The old binary accepts the newer schema (observed)
**What goes wrong:** `v0.0.10` against a version-60 database: no refusal, no log line, `/healthz` and `/readyz` `ok`, `HasPendingMigrations` false so no snapshot either. Later, the first order insert that does not set `orders.token` is the first one to succeed, the second fails with `UNIQUE constraint failed: orders.token`.
**Why it happens:** goose's `Up` treats "database ahead of the embedded migrations" as nothing to do. This is a property of the goose provider version in `go.mod` `[VERIFIED by running it, not by reading goose]`.
**How to avoid:** the document states the limit as fact and as a rule (older binary + newer schema is unsupported; going back means restoring the snapshot). Do not describe it as "the old binary will refuse to start": it does not.
**Warning signs:** an operator reading the limit as "the program will protect me".
**How to play it in the container:** old binary on the migrated database (a) start and `/readyz` ok; (b) the `INSERT INTO orders(website_id,number,audience,email,name,street,postal_code,city,items_net,items_tax,items_gross,total_net,total_tax,total_gross,created_at,updated_at) VALUES(…)` that a pre-00060 binary issues, twice, through `runuser -u holzcloud -- sqlite3`. Host result: first row ok, second `Error: stepping, UNIQUE constraint failed: orders.token (19)` `[VERIFIED: run on host against a v0.0.25 migration of a v0.0.10 database]`. Say in the record that this is the statement shape of the old code, issued by hand, unless a real order can be placed through the shop flow (preferable if cheap).

### Pitfall 3: The snapshot's name and scope surprise
**What goes wrong:** the file is `pre-upgrade-<NEW version>-<ts>.sqlite` and holds the OLD schema (observed: `pre-upgrade-v0.0.25+-20261012-105446.sqlite`, goose version 58; live database afterwards 60). It covers the database only, not media/templates. Only the newest five are kept. A fresh install writes none; an update without pending migrations writes none.
**How to avoid:** the doc tells the operator to take the path from the journal line `pre-upgrade snapshot written` (`path=…`) rather than guess a name, to copy it out of the data directory if more than five updates may happen before a rollback, and to say that "no snapshot" means "no migration".
Quoted from the code (read this session), `cmd/holzcloud/main.go` lines 121-143:
```go
if pending, err := db.HasPendingMigrations(database.Write); err != nil {
…
name := fmt.Sprintf("pre-upgrade-%s-%s.sqlite", Version, time.Now().UTC().Format("20060102-150405"))
snapshot, err := db.Backup(context.Background(), database, filepath.Join(cfg.DataDir, name))
…
slog.Info("pre-upgrade snapshot written", "path", snapshot)
if err := db.PruneSnapshots(cfg.DataDir, 5); err != nil {
…
slog.Error("restore with: systemctl stop holzcloud && cp " +
	preUpgradeSnapshot + " " + cfg.DBPath + " && systemctl start holzcloud")
```

### Pitfall 4: Restoring over a live WAL file
**What goes wrong:** the quoted restore command copies the snapshot over `holzcloud.sqlite` and leaves `holzcloud.sqlite-wal` and `-shm`. After a clean stop the WAL is empty (the shutdown path runs `PRAGMA wal_checkpoint(TRUNCATE)` `[VERIFIED: internal/db/maintain.go Maintain]`, and the server calls it on shutdown), but after the migration failure path (`os.Exit(1)`, deferred `Close` does not run) a non-empty WAL can remain. SQLite: "Overwriting a database file with another without also deleting any hot journal associated with the original database" is a listed way to corrupt a database `[CITED: https://www.sqlite.org/howtocorrupt.html §1.4]`.
**How to avoid:** `sudo rm -f /opt/holzcloud/data/holzcloud.sqlite-wal /opt/holzcloud/data/holzcloud.sqlite-shm` after the stop and before the copy, in the doc and in the played run. Observed after a snapshot restore on the host: the old binary started, `/readyz` `{"status":"ok","version":"v0.0.10","commit":"old","integrity":"ok"}`, goose version 58, no `migration applied` line.

### Pitfall 5: The recovery text for a failed migration restores data but not the binary
**What goes wrong:** with a failing NEW binary, copying the snapshot and `systemctl start` brings back the same new binary, which fails the same migration. Meanwhile systemd restarts it (`Restart=on-failure`, `RestartSec=5`, `StartLimitBurst=5` in 300 s `[VERIFIED: deploy/holzcloud.service]`) until the start limit refuses further starts.
**How to avoid:** D-11: the "A failed migration" text links to "Rolling back" and says "binary first, then data"; mention `systemctl reset-failed holzcloud` (already a documented pitfall in `deploy/test/README.md`). Changing the log message in `main.go` is a code change; CONTEXT allows code changes only for a defect that blocks the criteria. Record the incomplete message as a finding and fix the document; leave the code unless the planner judges otherwise.

### Pitfall 6: First Run over plain HTTP fails with 403 (observed)
**What goes wrong:** the shipped unit sets `HOLZCLOUD_SECURE=true`. A `curl` POST to `/admin/setup` over `http://127.0.0.1:…` is answered `403`; the server logs `CSRF validation failed … reason:"referer not supplied"`; with a referer but an HTML-escaped token it logs `CSRF token invalid` `[VERIFIED: run on host, HOLZCLOUD_SECURE=true, gorilla/csrf v1.7.3 per go.mod]`.
**How to avoid / how to play First Run headlessly (verified on the host with curl 8.7.1):**
```bash
H=http://127.0.0.1:8080
gettok(){ curl -s -c jar -b jar "$H$1" | grep -o 'name="gorilla.csrf.Token" value="[^"]*"' | head -1 \
  | sed 's/.*value="//;s/"$//;s/&#43;/+/g;s/&#61;/=/g;s/&#47;/\//g'; }   # html/template escapes + = /
tok=$(gettok /admin/setup)
curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' -c jar -b jar \
  -H "Referer: https://127.0.0.1:8080/admin/setup" \
  --data-urlencode "gorilla.csrf.Token=$tok" -d email=a@example.com \
  -d password=… -d password_confirm=… "$H/admin/setup"
# host result: 303 http://127.0.0.1:18081/admin/
# GET /admin/ afterwards: 303 …/admin/2fa/einrichten ; -L ends 200 there
```
Form field names `email`, `password`, `password_confirm` and the hidden field `gorilla.csrf.Token` are from `cmd/holzcloud/templates/admin/setup.html` lines 18-29 `[VERIFIED: read]`. The same recipe logs in (`POST /admin/login`, fields `email`, `password`): host result `303 …/admin/`. Whether Debian's curl 7.88.1 sends the `Secure` cookie to `http://127.0.0.1` the way macOS curl 8.7.1 did is `[ASSUMED]`; fallback is to pass the `_gorilla_csrf` cookie value with `-H 'Cookie: …'`.
**Doc consequence:** (a) First Run says "visit https://your-domain/admin", but nothing before it tells the operator that with the shipped unit (`HOLZCLOUD_SECURE=true`) the admin works only through the TLS proxy; (b) After the setup form the admin is sent to `/admin/2fa/einrichten`: two-factor enrolment is compulsory for administrators and First Run does not mention it. Both are DEP-01 findings. Do not hit the throttle while probing (ten failures per address block 15 minutes, per Recovery section).

### Pitfall 7: The image is not a bare server
`systemd-base` pre-installs `sudo curl ca-certificates jq procps sqlite3` (Dockerfile stage `systemd-base`, apt line read this session) and `openssl` is present `[VERIFIED: command -v inside the image]`. DEPLOY.md's Prerequisites name only `sqlite3`. A step that works only because of a pre-installed tool is, by D-01, a deviation. Verified missing in the image: `gpg` (needed by the Caddy install lines), `rsync` (needed by `backup.sh`/`restore.sh`), `fuser` (Troubleshooting) `[VERIFIED: command -v inside the image, 2026-10-12]`. A real VPS image may ship some of them; the doc should name them rather than rely on that.

### Pitfall 8: Under emulation the doc's `systemctl start` crashes (known, Phase 20)
The unchanged unit crashes at start with `ProcSubset=pid` (status 5/TRAP, `mmap_min_addr`). The literal DEPLOY.md `sudo systemctl start holzcloud` therefore ends in a restart loop in this container. Play the line literally, record the failure, then `run.sh start` (it re-issues `systemctl start`, detects the signature, writes `/etc/systemd/system/holzcloud.service.d/container.conf`, `reset-failed`, restart, waits). Name it `DEVIATION[dropin]` exactly as Phase 20 did. The drop-in survives binary swaps, so update and rollback need nothing extra.

### Pitfall 9: `up` resets, `seed` refuses, `check` needs the key
`up` deletes `OUT_DIR`; `seed` dies on an already seeded container (`/root/.hc-key` exists); `check` and `mdwe-control` need `/root/.hc-key`, which lives outside the data directory and is not part of any snapshot (README, hand-over to 22). After a rollback to a pre-seed state the key file would still exist while its database row is gone. Seed once, on the old binary, and let the rollback return to a state that contains it (the snapshot is taken after seeding, so the key row exists in S0).

### Pitfall 10: The first-hop comparison counts volatile tables
Sessions, activity log, mail outbox and plugin store change with every login and request. A before/after equality over every table fails for the wrong reason. Compare content tables; list the volatile ones as expected differences in the record (Pattern 4).

## Code Examples

### Old build, literally DEPLOY.md's line plus version stamping
```bash
# Source: deploy/DEPLOY.md "Build" line + .github/workflows/release.yml ldflags
T=$(mktemp -d); git archive v0.0.10 | tar -x -C "$T"
( cd "$T" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags="-s -w -X main.Version=v0.0.10 -X main.Commit=$(git -C "$OLDPWD" rev-parse --short v0.0.10)" \
    -o holzcloud-old ./cmd/holzcloud )
```
The host build of the same tree for darwin completed this session (go modules downloaded; toolchain auto-selected) `[VERIFIED]`; the linux/amd64 cross-build was not run. DEPLOY.md's own Build line carries no `-X` flags, so a binary built exactly by the doc reports `Version=dev`, and then every snapshot is named `pre-upgrade-dev-<ts>.sqlite` and `/readyz` cannot tell old from new. Finding: the Build section should tell the operator to stamp the version (`main.go` already documents the flag in a comment at lines 61-63 `[VERIFIED: read]`).

### The three server-side blocks the fixed doc should contain (shape, not final wording)
```bash
# Updating (before replacing anything)
sudo cp -p /opt/holzcloud/holzcloud /opt/holzcloud/holzcloud.prev
/opt/holzcloud/holzcloud version
sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups    # old binary, still in place
# …stop, replace, start as today…
journalctl -u holzcloud --since "10 minutes ago" | grep -E 'pre-upgrade snapshot written|migration applied'

# Rolling back
sudo systemctl stop holzcloud
sudo cp -p /opt/holzcloud/holzcloud.prev /opt/holzcloud/holzcloud
sudo rm -f /opt/holzcloud/data/holzcloud.sqlite-wal /opt/holzcloud/data/holzcloud.sqlite-shm
sudo -u holzcloud cp /opt/holzcloud/data/pre-upgrade-<version>-<ts>.sqlite /opt/holzcloud/data/holzcloud.sqlite
sudo systemctl reset-failed holzcloud; sudo systemctl start holzcloud
curl -s http://127.0.0.1:8080/readyz | jq -c '{status,version,commit,integrity}'
```
The exact wording belongs to the executor after the run; the `backup.sh` path in the second block must be the one the played install produced (Pitfall 11).

### Pitfall 11 (doc consistency, resolves a Phase 20 hand-over item): which backup path?
The install block copies `/tmp/deploy/backup.sh` to `/opt/holzcloud/backup.sh`; the Backups section and `holzcloud-backup.service` (`ExecStart=/opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups`) use `/opt/holzcloud/deploy/backup.sh`; `restore.sh` is never copied anywhere; `/opt/holzcloud/backups` is created but not chowned to `holzcloud`, while the backup unit runs as `User=holzcloud`; `backup.sh` needs `rsync`. A literal install therefore cannot run the "Backups" section, and the "back up first" command in `Updating` depends on it. Resolving the minimal set is a DEP-01 fix (install the whole `deploy/` directory to `/opt/holzcloud/deploy/`, chown `backups`, name `rsync`); proving backup and restore stays with Phase 22. Update `deploy/test/README.md` "Hand-over to phase 22" in the same commit so the two phases do not fix the same thing twice or contradict.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `sqlite3 .backup` in `backup.sh` | `holzcloud backup` (`VACUUM INTO` + `quick_check`) | already shipped | DEPLOY.md Prerequisites still say `sqlite3` is for backups (stale) |
| Operator guesses recovery commands | Program logs the restore command after a failed migration | already shipped | Incomplete (Pitfall 5) |

**Deprecated/outdated in DEPLOY.md (candidates, to be confirmed by the played run):** "Go 1.22 or later" (go.mod says 1.26.8 `[VERIFIED: go.mod:3]`; older Go versions fetch the toolchain themselves when allowed `[ASSUMED: Go toolchain documentation, not re-read this session]`); "`sqlite3` CLI (for backups)"; the container image section says "from 1.10 on" for `HOLZCLOUD_LISTEN` (outside this phase).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Debian's curl 7.88.1 sends a `Secure` cookie to `http://127.0.0.1` like macOS curl 8.7.1 did | Pitfall 6 | First Run scripting needs the explicit `Cookie:` header fallback; low |
| A2 | The container has working outbound network and the Caddy apt repository is reachable (needed for `caddy validate`) | D-03 / Environment Availability | Caddy validation is named a deviation instead; low |
| A3 | A real VPS image may ship `gpg`/`rsync`/`curl`; the doc should name them regardless | Pitfall 7 | Doc says more than needed; harmless |
| A4 | Go auto-downloads the toolchain for a `go.mod` newer than the installed Go (so "Go 1.22 or later" is merely stale, not blocking) | State of the Art | Doc wording; low |
| A5 | An update with no pending migration can be rolled back by swapping the binary alone | Rolling back wording | If wrong, the doc over-promises; play one no-migration hop (`v0.0.21` to HEAD, same schema) before writing it, or leave the sentence out |
| A6 | A cross-compiled linux/amd64 build of the `v0.0.10` tree succeeds exactly like the darwin build did | Code Examples | Falls back to Docker build with the current Dockerfile overlaid; low |
| A7 | The five HEAD plugin archives install and run on the `v0.0.10` binary (archives differ only by 2 bytes since the tag, `sdk` and the `/ai` tool and argument names are the same) | Seeding the old state | If `seed` fails on the old binary, seed on `v0.0.20` instead (1 migration) and say so |

## Open Questions

1. **Old tag: `v0.0.10` (two migrations) or something nearer?**
   - Known: `v0.0.10` crosses 00059 (new table `website_mail`, STRICT, FK to `websites`) and 00060 (`ALTER TABLE orders ADD COLUMN token`, `UPDATE orders SET token = lower(hex(randomblob(16)))`, unique index): one structure-only and one data-touching migration; ideal for "no data loss". `v0.0.1` to `v0.0.10` all sit at 58, so any of them gives two. `v0.0.15`/`v0.0.20` give one.
   - Unclear: whether `seed` works end to end on `v0.0.10` (A7).
   - Recommendation: `v0.0.10`; fall back to `v0.0.20` (one migration, the data-touching one) only if seeding fails, and state the count either way.
2. **Does an order exist in the seeded state?** The orders table is the only place migration 00060 changes data. `seed` creates none.
   - Recommendation: add one order row to the old state (real shop flow if the plugin makes it cheap, otherwise an `INSERT` through `sqlite3` as the service user, named as a direct insert), so the after-update check can show `token` filled for an existing row and its count unchanged.
3. **Play the no-migration hop?** A5. Cheap: `v0.0.21` already equals HEAD in migrations. Recommended as one extra short run because the migration limit is worded as "after a newer binary has migrated the schema".
4. **Scope overlap with Phase 22.** Pitfall 11. Recommendation: Phase 21 fixes install-path and prerequisite text that a literal install trips over; Phase 22 proves backup and restore and owns `backup.sh`/`restore.sh` changes. `restore.sh` is interactive (`read -r -p … [yes/NO]`) and needs `rsync`: not usable as the rollback in this phase.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker daemon | all runs | yes | 29.8.2, host aarch64 `[VERIFIED]` | none |
| Go toolchain (host) | both builds | yes | go1.27.2 darwin/arm64, `GOTOOLCHAIN=auto` `[VERIFIED]` | Docker build stage |
| git | `git archive` of the old tag | yes | — | — |
| Local images | rebuild anyway for HEAD | `holzcloud-unit-test:amd64-debian-bookworm-slim` present `[VERIFIED: docker image ls]` | built in Phase 20 | `run.sh build` |
| `systemd-base` image | install run | not built yet (`TARGET=systemd-base` gives tag `…-systemd-base`) | — | `run.sh build` |
| Old container left over | `up` refuses an existing name | none present `[VERIFIED: docker ps -a]` | — | `run.sh down` |
| `gpg`, `rsync`, `fuser` inside the image | Caddy install lines, `backup.sh`, Troubleshooting | **no** `[VERIFIED]` | — | `apt install gnupg rsync psmisc` as a named deviation, or add to DEPLOY.md prerequisites |
| Outbound network from the container, Caddy apt repo | `caddy validate` (D-03) | unknown | — | name as deviation |
| `sqlite3`, `curl`, `jq`, `sudo`, `openssl`, `runuser` in the image | probes | yes (`sudo curl ca-certificates jq procps sqlite3` from the Dockerfile; `openssl` verified) | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** `gpg`, `rsync`, `fuser`, network for Caddy.

(Validation Architecture section omitted: `workflow.nyquist_validation` is `false` in `.planning/config.json` `[VERIFIED: read]`. The requirement-to-evidence mapping the planner needs is in the table below.)

### Requirement to evidence map (acceptance, in place of a test-framework section)

| Req | Observable pass | Evidence file |
|-----|-----------------|---------------|
| DEP-01 | Every DEPLOY.md step ran in `systemd-base`; unit `active`; `/healthz` and `/readyz` ok; setup POST `303 …/admin/`, login `303 …/admin/`, then `…/admin/2fa/einrichten`; findings table complete; DEPLOY.md re-played after the fixes shows zero remaining deviations (second pass) | `DEPLOY-RUN.md` |
| DEP-02 | `/readyz` version old then new; journal shows `pre-upgrade snapshot written` and `migration applied` 59 and 60; goose version 58 to 60; content tables equal (Pattern 4); page hash and media hash equal; login `303`; public page 200; `orders.token` filled | `DEPLOY-RUN.md` |
| DEP-03 | After rollback `/readyz` version old, goose version 58, pre-update rows present, a row written after the update gone; negative case recorded (starts, `/readyz` ok, 2nd order insert fails); migration-limit paragraph in DEPLOY.md quotes the observed behaviour | `DEPLOY-RUN.md`, `DEPLOY.md` |
| D-12 | `run.sh` usage text and `deploy/test/README.md` list the new subcommands; `bash -n` and `shellcheck` (if present) clean; interface of the old subcommands unchanged | repo |

Quick gates after editing docs and scripts (all runnable in seconds): `go test ./cmd/holzcloud -run 'Deploy|Caddyfile'` (it asserts DEPLOY.md still contains `2.11.2`, `CVE-2026-30851`, `systemctl edit caddy`, `EnvironmentFile=/etc/holzcloud-sso.env` and none of three forbidden phrases `[VERIFIED: cmd/holzcloud/deploy_docs_test.go read]`), `go run ./tools/cites -check` (scans `.go` files only, currently 0 unresolvable `[VERIFIED: run]`), `gofmt -l .`, `go vet ./...`.

## Security Domain

`security_enforcement` is not set to `false` in `.planning/config.json` `[VERIFIED: read]`, so this applies.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (First Run, login probe, `user create`) | Argon2id via the program; password only on stdin (`readPassword`), never as an argument; test passwords are throw-away |
| V3 Session Management | marginal | Sessions live in SQLite: a rollback restores the sessions table of the snapshot (post-update logins vanish); say so in the doc |
| V4 Access Control | yes (file modes) | `UMask=0077`; snapshot and data directory readable by `holzcloud` only; restored files must stay `holzcloud`-owned (`sudo -u holzcloud cp`, not root-owned new files) |
| V5 Input Validation | no | no new input surface |
| V6 Cryptography | no | none added; `HOLZCLOUD_SECRET_KEY` and the CSRF key are not touched by a rollback (key file outside the DB) |
| V14 Configuration | yes | Do not leak the admin API key or any test password into committed records (`seed` already keeps the key in a root-only file; apply the same to the login user's password: set it from an environment variable inside the container, never echo it into `DEPLOY-RUN.md`) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Restoring a stale or foreign database over the live one | Tampering | Verify the snapshot (`holzcloud check` on a copy, as `restore.sh` does) before copying; keep a copy of the pre-rollback state (`cp` the migrated database aside as `….after-update`) so the rollback is itself reversible |
| Corruption by replacing the DB under a WAL | Tampering/DoS | Stop the service, remove `-wal`/`-shm` (Pitfall 4) |
| World-readable snapshot/backups holding password hashes | Information disclosure | Files are created `0600` by the service (`UMask=0077`); the doc keeps `backups` `700` (already in Backups) |
| Secrets in committed run logs | Information disclosure | `seed` strips the one-time upload link and never writes the key; keep that rule in the new subcommands; scan `DEPLOY-RUN.md` for `hc_` and test passwords before commit (Phase 20 did a "no key-shaped string" check) |
| Container is privileged | Elevation | Throw-away, no ports, no mounts (Phase 20 Safety); keep that for the new containers |
| Third-party fetch at runtime | — | CLAUDE.md "Nothing loads at runtime" applies to the application, not to the deploy-time `apt`/Caddy repository of the doc |

## Project Constraints (from CLAUDE.md)

- **GSD workflow enforcement:** file edits happen through the GSD execute path for this phase; no ad-hoc edits outside it.
- **Docs language:** DEPLOY.md is English; match its style (CONTEXT agrees). The operator-facing catalogue rule (`go run ./tools/i18n`) does not apply to Markdown; it applies only if a `.go` file under `internal/` or `cmd/holzcloud/templates/admin` changes. No such change is planned, so the gate should stay `0 open, 0 orphaned`.
- **Hard stack:** Go 1.22+ statement in CLAUDE.md versus `go 1.26.8` in `go.mod`; do not "fix" CLAUDE.md here, but DEPLOY.md should state what `go.mod` requires.
- **Nothing loads at runtime:** untouched; the Caddy apt repository in DEPLOY.md is a deploy-time download by the operator, not part of the program.
- **Build:** `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud` is the canonical line; the version stamping adds `-X` flags to it.
- **Database conventions:** WAL, dual pool, `STRICT` tables, goose embedded migrations: all consistent with the findings (snapshot via `VACUUM INTO`).
- **Code changes:** none expected. If a played run exposes a blocking defect (CONTEXT scope), the usual gates apply (`go vet`, `gofmt`, i18n gate, `tools/english`, `tools/cites -check`, the relevant tests).
- **Git (from user memory):** commits as `holzcloud.ch` (noreply), set locally; attribution trailer required on commits. Questions to the user go through `AskUserQuestion` with options.

## Sources

### Primary (HIGH confidence)
- Repository files read this session: `deploy/DEPLOY.md`, `deploy/holzcloud.service`, `deploy/restore.sh`, `deploy/backup.sh` (lines 1-80), `deploy/holzcloud-backup.service` (grep), `deploy/test/run.sh`, `deploy/test/README.md`, `deploy/test/Dockerfile`, `deploy/test/Dockerfile.dockerignore`, `.dockerignore`, `internal/db/maintain.go`, `internal/db/db.go` (lines 60-120), `cmd/holzcloud/main.go` (lines 61-143, 185-215), `cmd/holzcloud/cli.go` (usage, `runCLI`, `openForCLI`, `user`, `backup`, `migrate`), `internal/admin/setup.go`, `cmd/holzcloud/templates/admin/setup.html` (grep), `cmd/holzcloud/deploy_docs_test.go`, `.github/workflows/release.yml` (lines 195-215), `.planning/phases/20-…/20-03-SUMMARY.md`, `deploy/SECURITY-SCORE.md` (Container vs. server)
- Played on this host with two real binaries (`v0.0.10` from `git archive`, and HEAD, both `go build` darwin/arm64, throw-away data directories in the session scratchpad): fresh start, update 58 to 60 with snapshot, old binary against schema 60, snapshot restore, `/admin/setup` and `/admin/login` via curl with `HOLZCLOUD_SECURE=true`, the duplicate-token insert
- `git ls-tree`, `git diff --name-status`, `git rev-parse` on tags `v0.0.1` to `v0.0.25`
- `docker run` on `holzcloud-unit-test:amd64-debian-bookworm-slim`: `command -v` for gpg, rsync, openssl, sudo, curl, jq, sqlite3, fuser, git

### Secondary (MEDIUM confidence)
- https://www.sqlite.org/howtocorrupt.html §1.2 to §1.4 (fetched this session): replacing a database file without deleting the hot journal/WAL of the original is a corruption cause; a WAL must travel with its database

### Tertiary (LOW confidence)
- Go toolchain auto-download behaviour, curl's treatment of `Secure` cookies on loopback: training knowledge, tagged `[ASSUMED]` (A1, A4)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, nothing new; every tool verified present or named missing
- Architecture (procedure and harness): HIGH for the host-played behaviour; MEDIUM for the in-container replay (emulation timing, Debian curl, apt/network)
- Pitfalls: HIGH for 1 to 6 (each reproduced or read), MEDIUM for 7 to 11 (verified facts, wording of fixes is the executor's)

**Research date:** 2026-10-12
**Valid until:** 2026-11-11 (stable; invalid sooner if migrations 00061+ land before the run, because the migration count to HEAD then changes)
