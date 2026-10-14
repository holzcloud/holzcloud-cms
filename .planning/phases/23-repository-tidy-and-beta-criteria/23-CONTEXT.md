# Phase 23: Repository tidy and beta criteria - Context

**Gathered:** 2026-10-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Close milestone v0.1.0 (REPO-01, REPO-02, REPO-03, QUAL-01): write down what "0.1.0" promises and
what it does not, write the CHANGELOG entry and release notes for 0.1.0 and set up the tag,
list the orphaned remote branches with GitHub-UI deletion steps, and hold the i18n gate at
`0 open, 0 orphaned`. Deliverables are documents and one version bump; no new operator-visible
features. Code changes only where a gate (QUAL-01) or the version stamp requires them.

</domain>

<decisions>
## Implementation Decisions

The user was offered all four gray areas (criteria form, version/tag, release notes, branches) and
delegated every one to Claude ("du entscheidest"), as in Phases 21 and 22. Everything below is
Claude's call, conservative and evidence-first.

### Beta criteria (REPO-01)
- **D-01:** One file, `docs/beta.md`, linked from `README.md` and from the 0.1.0 CHANGELOG entry. Not a README section: the README is a front door, this is a contract that must be diffable on its own.
- **D-02:** Language: **English**, like `deploy/` and the other `docs/` pages that are not operator wording. Operator-facing UI strings are untouched.
- **D-03:** Two lists with equal weight: **what 0.1.0 promises** and **what it does not**. Each promise cites a recorded proof and nothing else; each non-promise is named, not implied.
- **D-04:** Allowed citations are exactly those the hand-overs release: `deploy/DEPLOY-RUN.md` (Phase 21), `deploy/BACKUP-RUN.md` (Phase 22: Summary table, `Evidence commit`, the `summary.txt` words, the shown list, `HAND-REBUILT`), `deploy/SECURITY-SCORE.md` (Phase 20), and the Phase 20 plugin/`MemoryDenyWriteExecute` verdict. Read each hand-over section in `deploy/test/README.md` before citing; claim nothing outside them.
- **D-05:** The non-promises **must include**, copied from Phase 22's *Not tested*: the real 03:15 window, `Persistent=true` and `RandomizedDelaySec`, the off-site `REMOTE_TARGET` copy, retention pruning, an atomic database-plus-media snapshot, a backup older than its restoring binary, a restore on a real server, native amd64 timing, and bit-rot in a complete backup. Phase 20/21 equivalents (container-vs-server differences, seccomp not enforced under emulation) are added from their own records.
- **D-06:** The evidence-commit rule is carried forward: a change to `deploy/DEPLOY.md`, `backup.sh`, `restore.sh`, the units or `deploy/test/run.sh` after the evidence commit means the set is rerun before the record is cited for the changed text. If Phase 23 touches any of them, the plan must say so and rerun or drop the citation.
- **D-07:** "Beta" is defined as: safe to run for real with backups, **not** feature-frozen. State plainly whether the 0.1.0 migrations/schema are promised stable (default: not promised; rollback limit as documented in DEPLOY.md).

### Version and tag (REPO-02)
- **D-08:** The release is **0.1.0**, tag `v0.1.0`. The CHANGELOG heading is `## 0.1.0 — 2026-10-xx` with the date of the actual tag day; follow the file's whole-sentence style and its German-vs-English convention as the existing entries do (they are German operator prose).
- **D-09:** Claude prepares CHANGELOG, release notes, README badge/wording and the version references, commits them, and **does not push a tag**. The proxy refuses `refs/tags`; the documented route is `release` branch fast-forward or the Actions button (`.github/workflows/release.yml` reads the CHANGELOG). The plan ends with the exact one-line command or button steps for the owner. "The tag is set" (roadmap criterion 2) is satisfied when the owner has run it; the plan records it as a human step and verifies the workflow's expected input (CHANGELOG heading) locally instead of claiming the tag exists.
- **D-10:** README: badge `status-alpha` → `status-beta` and the "alpha software under heavy development" notice rewritten to match `docs/beta.md`. The wording must not promise more than D-03/D-05 allow.
- **D-11:** Release notes = the CHANGELOG 0.1.0 entry **plus** a short upgrade note for operators coming from 0.0.25 (what changed in `backup.sh`/`restore.sh`/`DEPLOY.md`, hardened unit, what to do after updating). No separate notes file unless `release.yml` requires one; check what it publishes as the body.
- **D-12:** The 0.1.0 entry covers what is new **since 0.0.25** (Phases 20–22: unit proven, deploy/update/rollback played, backup/restore measured and scripts repaired). It does not retell 0.0.1–0.0.25.

### Orphan branches (REPO-03)
- **D-13:** The list is generated from the repository, not from memory. Current remote branches besides `main`: `backfill-releases`, `claude/holzcloud-projects-update-0t9epb`, `claude/project-open-tasks-lwuj02`, `claude/task-phases-milestones-open-mt1qz0`, `feat/holzwave-farben`, `fix/tables-mobile`, `holzbar-colours`, `redirects-keep-language`, `release`, `tagfix`. (Measured 2026-10-14 against `origin/main`: ahead-of-main counts 1, 0, 4, 0, 1, 1, 0, 0, 0, 1 respectively.)
- **D-14:** **`release` is not an orphan** and is listed as "keep", with the reason (it is the button in `release.yml`).
- **D-15:** Each candidate is checked before it is called deletable: ahead commits are tested for patch-equivalence in `main` (`git cherry`, or a diff of content) because squash merges hide them. A branch whose commits are **not** in `main` is marked "review first" with the commit subject, never "safe to delete". Claude does not delete anything and does not decide on unmerged work.
- **D-16:** The deliverable lives in `docs/branches.md` (or a clearly named section in `docs/beta.md` if shorter): a table (branch, last commit date, ahead/behind, verdict, reason) and the GitHub-UI steps (Code → Branches → trash icon, or the branch list page; note the restore link GitHub offers after deletion). The note that the proxy answers 403 to ref deletion stays, since it is why this is manual.

### i18n gate (QUAL-01)
- **D-17:** `go run ./tools/i18n` must print `0 open, 0 orphaned` at the end of the phase. Run it first, before any edit, to learn whether the baseline is already clean. Any operator-visible string the phase adds goes through the catalogue (`t`/`th`/`tf`/`web.T`); this phase should add almost none, since it is documents.
- **D-18:** Also keep green: `go run ./tools/themewords -check`, and the repository's existing tests that tie the CHANGELOG to the version (`changelog_test.go`, `changelog.go` at repo root). Read them before editing CHANGELOG.md: they are the real format contract.

### Claude's Discretion
- Exact wording and section order of `docs/beta.md`, as long as D-03–D-07 hold.
- Whether the branch table sits in its own file or in `docs/beta.md`.
- Whether the upgrade note is a CHANGELOG paragraph or a short `docs/` page, provided the release body shows it.
- Order of plans and waves.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and scope
- `.planning/ROADMAP.md` §"Phase 23" — goal and four success criteria
- `.planning/REQUIREMENTS.md` — REPO-01, REPO-02, REPO-03, QUAL-01 and the "Out of Scope" table
- `.planning/STATE.md` §"Where we are" — release is 0.0.25; orphan branch list; proxy 403 on ref deletion

### Evidence the criteria may cite
- `deploy/test/README.md` — "Hand-over to phase 23" (what may and may not be claimed), and the Phase 21/20 hand-overs
- `deploy/BACKUP-RUN.md` — Phase 22 record (Summary table, `Evidence commit`, Measurements, *Not tested*, `HAND-REBUILT`)
- `deploy/DEPLOY-RUN.md` — Phase 21 record
- `deploy/SECURITY-SCORE.md` — Phase 20 score and container-vs-server differences
- `deploy/DEPLOY.md` — install, update, rollback (with migration limit), backups
- `.planning/phases/22-backup-and-restore-measured/22-CONTEXT.md`, `22-VERIFICATION.md` — prior decisions on evidence-first records

### Release machinery
- `CHANGELOG.md` — format and house style; read the top entries
- `changelog.go`, `changelog_test.go` (repo root) — what the tests demand of the CHANGELOG
- `.github/workflows/release.yml` — how a tag/`release` push/dispatch becomes a release, what it reads and publishes
- `cmd/holzcloud/main.go` — `Version`/`Commit` injected at build time (`-ldflags -X`)
- `README.md` — alpha badge (line ~11) and alpha notice (line ~60)
- `CONTRIBUTING.md`, `SECURITY.md` — check for version/alpha wording that must change

### Gates
- `tools/i18n` — `go run ./tools/i18n` must report `0 open, 0 orphaned`
- `tools/themewords` — `go run ./tools/themewords -check`
- `CLAUDE.md` §Translation — the catalogue rules

No external specs beyond these.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `deploy/BACKUP-RUN.md` and `deploy/DEPLOY-RUN.md`: the records the criteria cite; their grep-F proof pattern (Phase 21/22) is the model for any "claim equals record" check.
- `deploy/test/run.sh` chains (`fresh`, `lifecycle`, `backup-chain`): the way to rerun evidence if a cited file changes (D-06).
- `changelog_test.go`: executable format contract for CHANGELOG entries.

### Established Patterns
- Records state what was shown and what was not, with deviations named; claims outside the record are not made.
- Docs under `deploy/` and `docs/` are English; CHANGELOG entries are German operator prose.
- Human-only steps (GitHub UI, tag push) are listed with exact steps, never attempted through the proxy.

### Integration Points
- `release.yml` reads the CHANGELOG heading to name the release; the heading must match what the workflow parses.
- README badge and notice must agree with `docs/beta.md`.
- Version stamp comes from `git describe --tags`; no hard-coded version in Go is expected, but grep before assuming.

</code_context>

<specifics>
## Specific Ideas

- The beta page is a contract for readers who trust it: a promise without a citation does not go in.
- Branch verdicts are about evidence in `main`, never about age.

</specifics>

<deferred>
## Deferred Ideas

- Real-server and native-amd64 runs (restore timing, seccomp enforcement, the 03:15 window, `Persistent=true`): need their own run, a candidate for a later milestone.
- Checksums or an atomic database-plus-media snapshot in `backup.sh` (bit-rot protection): a feature, not tidy-up.
- Deleting the orphan branches: manual, in the GitHub UI, by the owner.

</deferred>

---

*Phase: 23-repository-tidy-and-beta-criteria*
*Context gathered: 2026-10-14*
