# Phase 21: Deploy, update, rollback - Context

**Gathered:** 2026-10-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Play through install, update and rollback in the Phase 20 test container and make
`deploy/DEPLOY.md` match what actually happened (DEP-01, DEP-02, DEP-03). Deliverables are
corrected documentation, a documented rollback including its migration limit, and recorded
evidence of each run. No new operator-visible features; code changes only if a played run
exposes a defect that blocks the criteria.

</domain>

<decisions>
## Implementation Decisions

The user delegated all decisions for this phase to Claude ("du kannst alles selber entscheiden").
Everything below is therefore Claude's call, made to be conservative and evidence-first.

### Fresh install (DEP-01)
- **D-01:** Play the install from `TARGET=systemd-base` (Phase 20 hand-over): DEPLOY.md is followed **step by step, literally**, each step through `run.sh exec`; `scp` steps become `docker cp`, `sudo` commands are pasted unchanged. The agent must not use knowledge that is not in the document — a step that only works because of outside knowledge is a deviation.
- **D-02:** Every deviation found (missing step, wrong path, wrong order, unexplained prerequisite, command that fails) is **fixed in DEPLOY.md in the same phase** and listed in a findings record (`deploy/test/DEPLOY-RUN.md` or section in the phase summary) with: step, what the doc said, what happened, fix.
- **D-03:** Steps that cannot run in the container (Caddy/HTTPS with a public domain, DNS, real mount for `RequiresMountsFor`) are **named as deviations, not skipped silently**. Caddy is verified at most as `caddy validate` of `Caddyfile.example` if installable; the HTTP check is against `127.0.0.1:8080` inside the container. Reuse the container-only drop-in that `run.sh start` applies.
- **D-04:** "Works" = unit active, `/healthz`-style endpoint (see DEPLOY.md "Health checks") answers, First Run section completes (admin user created, login possible).

### Update across a migration (DEP-02)
- **D-05:** Use **two real builds**: an older tagged release as "old" and the current HEAD as "new", built with different `VERSION`/`COMMIT` build args (Phase 20 recipe). Choose the old tag so that **at least one migration lies between the two** (the planner picks the tag and states the migration count; more is better than one). Synthetic migrations are not used.
- **D-06:** Seed data on the old version (website, domain, published page, uploaded image, a user with login, plugin installed — reuse `seed`; add a login user via `holzcloud user`). "No data loss" is shown by **before/after comparison**: row counts per table that exist in both schemas, content hash of the page and media file, successful login, page renders publicly. Raw outputs committed.
- **D-07:** The update procedure played is exactly the DEPLOY.md "Updating" section; the pre-upgrade snapshot the service takes (`internal/db/maintain.go`, `pre-upgrade-<version>-<ts>.sqlite`) is observed and its name/location recorded. Fix the section where it is silent (e.g. back-up-first as a concrete command, order stop → replace → start, what to look for in the journal).

### Rollback (DEP-03)
- **D-08:** The **documented rollback path is: stop → restore the pre-upgrade snapshot over the live database → put the old binary back → start.** This is the supported route because it is the only one that restores both binary and data consistently. It is played after the update from D-05 and the result (old version runs, pre-update data present, data written after the update is gone) is recorded.
- **D-09:** State the **migration limit explicitly in DEPLOY.md**: after a newer binary has migrated the schema, an older binary must not be started against that database; going back means restoring the snapshot and **losing everything written since the update**. Also play the negative case once — old binary against the migrated database — and record the actual behaviour (refuses vs. misbehaves) so the doc states fact, not assumption.
- **D-10:** `goose down` is **not** offered as the operator rollback path. Whether Down sections exist (60 of 60) is mentioned at most as a developer note; it is not tested as a supported route in this phase. Rationale: untested data-destroying Down migrations are a worse promise than a snapshot.
- **D-11:** Rollback documentation lives as a new "Rolling back" section next to "Updating" (not only under Recovery), and the existing "A failed migration" recovery text is cross-linked and kept consistent.

### Evidence and automation
- **D-12:** Played runs are committed as **repeatable scripts** under `deploy/test/` (new subcommands or steps for `run.sh`, keeping build/start/exec separable as in D-02 of Phase 20) plus the raw logs under the existing `deploy/test/out/` convention. Reason: Phase 22 and the 0.1.0 beta criteria benefit from a rerunnable proof. Scripts must not become a second installer: DEPLOY.md remains the source of truth and the script only mirrors its steps.
- **D-13:** Platform stays **linux/amd64 via emulation** (Phase 20 D-01); container-vs-server differences relevant to install/update/rollback are named in the findings.

### Claude's Discretion
All of the above, plus: choice of old tag, exact file names of records, how row counts/hashes are collected, whether Caddy validation is feasible, wording of doc fixes (English, matching DEPLOY.md's style).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Deploy docs and unit
- `deploy/DEPLOY.md` — the document under test (Install, systemd Service, First Run, Backups, Updating, Recovery → "A failed migration")
- `deploy/holzcloud.service` — hardened unit (unchanged by Phase 20)
- `deploy/Caddyfile.example` — reverse proxy example
- `deploy/backup.sh`, `deploy/restore.sh` — backup tooling referenced by "back up first"
- `deploy/SECURITY-SCORE.md` — Phase 20 findings, container-vs-server differences

### Test recipe (Phase 20 hand-over)
- `deploy/test/README.md` — "Hand-over to phase 21", subcommands, env vars, pitfalls
- `deploy/test/run.sh`, `deploy/test/Dockerfile` — recipe to extend
- `.planning/phases/20-hardened-unit-under-real-systemd/20-CONTEXT.md` — D-01..D-03a

### Migrations and snapshot
- `internal/db/maintain.go` — pre-upgrade snapshot logic
- `internal/db/migrations/` — 60 goose migrations (all with Down sections)
- `cmd/holzcloud/main.go` — migration on startup, Version/Commit injection

### Planning
- `.planning/REQUIREMENTS.md` — DEP-01..03
- `.planning/ROADMAP.md` §Phase 21 — success criteria

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `deploy/test/run.sh` subcommands (`build up start wait seed check exec logs down …`) and the `systemd-base` / `unit-test` targets.
- `seed` creates website, domain, page, five plugins, image and an API key (no login user).
- Pre-upgrade snapshot mechanism already in the service.

### Established Patterns
- Two builds with different `VERSION`/`COMMIT` build args; amd64 emulation drop-in for container start.
- Findings recorded raw, deviations named (Phase 20).

### Integration Points
- DEPLOY.md "Updating" currently has no backup command, no rollback and no migration limit.
- Phase 22 reuses `unit-test` + `seed`; keep scripts compatible.

</code_context>

<specifics>
## Specific Ideas

- The agent playing the install must treat DEPLOY.md as the only source of knowledge.
- The migration limit is the single most important sentence of the rollback section: older binary + newer schema is unsupported.

</specifics>

<deferred>
## Deferred Ideas

- Supporting `goose down` as an operator rollback command — would be a new capability with its own test burden.
- Automatic rollback on failed health check after update — new feature, own phase.

</deferred>

---

*Phase: 21-Deploy, update, rollback*
*Context gathered: 2026-10-12*
