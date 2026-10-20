# Phase 23: Repository tidy and beta criteria - Research

**Researched:** 2026-10-14
**Domain:** release engineering for a documents-only milestone close (Markdown contracts, CHANGELOG/release workflow, git branch audit, standing i18n gate)
**Confidence:** HIGH (everything below was read or run in this session; the few `[ASSUMED]` items are listed in the Assumptions Log)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

The user was offered all four gray areas (criteria form, version/tag, release notes, branches) and
delegated every one to Claude ("du entscheidest"), as in Phases 21 and 22. Everything below is
Claude's call, conservative and evidence-first.

**Beta criteria (REPO-01)**
- **D-01:** One file, `docs/beta.md`, linked from `README.md` and from the 0.1.0 CHANGELOG entry. Not a README section: the README is a front door, this is a contract that must be diffable on its own.
- **D-02:** Language: **English**, like `deploy/` and the other `docs/` pages that are not operator wording. Operator-facing UI strings are untouched.
- **D-03:** Two lists with equal weight: **what 0.1.0 promises** and **what it does not**. Each promise cites a recorded proof and nothing else; each non-promise is named, not implied.
- **D-04:** Allowed citations are exactly those the hand-overs release: `deploy/DEPLOY-RUN.md` (Phase 21), `deploy/BACKUP-RUN.md` (Phase 22: Summary table, `Evidence commit`, the `summary.txt` words, the shown list, `HAND-REBUILT`), `deploy/SECURITY-SCORE.md` (Phase 20), and the Phase 20 plugin/`MemoryDenyWriteExecute` verdict. Read each hand-over section in `deploy/test/README.md` before citing; claim nothing outside them.
- **D-05:** The non-promises **must include**, copied from Phase 22's *Not tested*: the real 03:15 window, `Persistent=true` and `RandomizedDelaySec`, the off-site `REMOTE_TARGET` copy, retention pruning, an atomic database-plus-media snapshot, a backup older than its restoring binary, a restore on a real server, native amd64 timing, and bit-rot in a complete backup. Phase 20/21 equivalents (container-vs-server differences, seccomp not enforced under emulation) are added from their own records.
- **D-06:** The evidence-commit rule is carried forward: a change to `deploy/DEPLOY.md`, `backup.sh`, `restore.sh`, the units or `deploy/test/run.sh` after the evidence commit means the set is rerun before the record is cited for the changed text. If Phase 23 touches any of them, the plan must say so and rerun or drop the citation.
- **D-07:** "Beta" is defined as: safe to run for real with backups, **not** feature-frozen. State plainly whether the 0.1.0 migrations/schema are promised stable (default: not promised; rollback limit as documented in DEPLOY.md).

**Version and tag (REPO-02)**
- **D-08:** The release is **0.1.0**, tag `v0.1.0`. The CHANGELOG heading is `## 0.1.0 — 2026-10-xx` with the date of the actual tag day; follow the file's whole-sentence style and its German-vs-English convention as the existing entries do (they are German operator prose).
- **D-09:** Claude prepares CHANGELOG, release notes, README badge/wording and the version references, commits them, and **does not push a tag**. The proxy refuses `refs/tags`; the documented route is `release` branch fast-forward or the Actions button (`.github/workflows/release.yml` reads the CHANGELOG). The plan ends with the exact one-line command or button steps for the owner. "The tag is set" (roadmap criterion 2) is satisfied when the owner has run it; the plan records it as a human step and verifies the workflow's expected input (CHANGELOG heading) locally instead of claiming the tag exists.
- **D-10:** README: badge `status-alpha` → `status-beta` and the "alpha software under heavy development" notice rewritten to match `docs/beta.md`. The wording must not promise more than D-03/D-05 allow.
- **D-11:** Release notes = the CHANGELOG 0.1.0 entry **plus** a short upgrade note for operators coming from 0.0.25 (what changed in `backup.sh`/`restore.sh`/`DEPLOY.md`, hardened unit, what to do after updating). No separate notes file unless `release.yml` requires one; check what it publishes as the body.
- **D-12:** The 0.1.0 entry covers what is new **since 0.0.25** (Phases 20–22: unit proven, deploy/update/rollback played, backup/restore measured and scripts repaired). It does not retell 0.0.1–0.0.25.

**Orphan branches (REPO-03)**
- **D-13:** The list is generated from the repository, not from memory. Current remote branches besides `main`: `backfill-releases`, `claude/holzcloud-projects-update-0t9epb`, `claude/project-open-tasks-lwuj02`, `claude/task-phases-milestones-open-mt1qz0`, `feat/holzwave-farben`, `fix/tables-mobile`, `holzbar-colours`, `redirects-keep-language`, `release`, `tagfix`. (Measured 2026-10-14 against `origin/main`: ahead-of-main counts 1, 0, 4, 0, 1, 1, 0, 0, 0, 1 respectively.)
- **D-14:** **`release` is not an orphan** and is listed as "keep", with the reason (it is the button in `release.yml`).
- **D-15:** Each candidate is checked before it is called deletable: ahead commits are tested for patch-equivalence in `main` (`git cherry`, or a diff of content) because squash merges hide them. A branch whose commits are **not** in `main` is marked "review first" with the commit subject, never "safe to delete". Claude does not delete anything and does not decide on unmerged work.
- **D-16:** The deliverable lives in `docs/branches.md` (or a clearly named section in `docs/beta.md` if shorter): a table (branch, last commit date, ahead/behind, verdict, reason) and the GitHub-UI steps (Code → Branches → trash icon, or the branch list page; note the restore link GitHub offers after deletion). The note that the proxy answers 403 to ref deletion stays, since it is why this is manual.

**i18n gate (QUAL-01)**
- **D-17:** `go run ./tools/i18n` must print `0 open, 0 orphaned` at the end of the phase. Run it first, before any edit, to learn whether the baseline is already clean. Any operator-visible string the phase adds goes through the catalogue (`t`/`th`/`tf`/`web.T`); this phase should add almost none, since it is documents.
- **D-18:** Also keep green: `go run ./tools/themewords -check`, and the repository's existing tests that tie the CHANGELOG to the version (`changelog_test.go`, `changelog.go` at repo root). Read them before editing CHANGELOG.md: they are the real format contract.

### Claude's Discretion
- Exact wording and section order of `docs/beta.md`, as long as D-03–D-07 hold.
- Whether the branch table sits in its own file or in `docs/beta.md`.
- Whether the upgrade note is a CHANGELOG paragraph or a short `docs/` page, provided the release body shows it.
- Order of plans and waves.

### Deferred Ideas (OUT OF SCOPE)
- Real-server and native-amd64 runs (restore timing, seccomp enforcement, the 03:15 window, `Persistent=true`): need their own run, a candidate for a later milestone.
- Checksums or an atomic database-plus-media snapshot in `backup.sh` (bit-rot protection): a feature, not tidy-up.
- Deleting the orphan branches: manual, in the GitHub UI, by the owner.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REPO-01 | The beta criteria are written down: what "0.1.0" promises and what it does not. | The citable record is fully read (see "What the beta page may cite"); the non-promise list is assembled below from the three records' own *Not tested* / *Not measured* / *Container vs. server* sections. Two contradictions with the word "beta" were found (in-app notice, `DEPLOY.md:677`) and are in Open Questions. |
| REPO-02 | CHANGELOG entry and release notes for 0.1.0, tag `v0.1.0`. | `release.yml` was read end to end: heading regex, awk extraction, pre-release flag, tag minting. No Go code changed since 0.0.25, so the entry is honest only if it describes documents, scripts and the unit's comments. The upgrade-note facts for operators coming from 0.0.25 are extracted from the diff. |
| REPO-03 | The orphaned remote branches are listed, with instructions for deleting them in the GitHub UI. | Every branch was fetched, cherry-checked against local and remote `main`, cross-checked against GitHub PR state, and two of them carry push-triggered workflows that are a live hazard. Verdict table below. |
| QUAL-01 | `go run ./tools/i18n` reports `0 open, 0 orphaned`. | Baseline run this session: already clean on all four translated catalogues. The gate only needs to be held; the one way to break it is changing an existing catalogue key (see Pitfall 5). |
</phase_requirements>

## Summary

Phase 23 is a documents-and-release-plumbing phase, and the research finding that shapes it most is that **the binary of 0.1.0 is functionally the binary of 0.0.25**. `git diff v0.0.25 main` outside `.planning/` touches `deploy/`, `docs/`, `.gitignore` and one new test-harness file (`deploy/test/canary/main.go`); not one `.go` file under `cmd/`, `internal/`, `plugins/`, `sdk/` or `tools/` changed, and the only change to `deploy/holzcloud.service` is seven comment lines. So the CHANGELOG entry must be about what operators can now rely on and about the repaired scripts and documents, not about features, and the "version bump" is purely a CHANGELOG heading: the version in the binary comes from `git describe --tags` via `-ldflags -X main.Version`, and no version string is hard-coded in Go.

Three repository facts the CONTEXT does not know about change the plan. (1) **Local `main` has diverged from `origin/main`**: 77 local commits (all of Phases 20 to 23 context) are not pushed, and `origin/main` has one commit local lacks (`9cc7bcb`, 2026-10-13, "slim the README"), which rewrote `README.md` almost entirely. A trial merge is clean, but the README line numbers in CONTEXT are stale (badge now at line 6, warning at lines 21 to 24, a Documentation list at 98 to 109). The first task must be `git fetch && git merge origin/main`. (2) **The product itself says "alpha"**: `internal/web/layoutdata.go` `IsAlpha()` is true for every `0.x` build, so the admin sidebar, login page and dashboard banner will say "alpha software" on 0.1.0; `deploy/DEPLOY.md:677` also says "alpha software", and that file is inside the Phase 22 evidence set. Changing either is expensive (catalogue orphans; evidence rerun). The recommended default is to change neither and to say so plainly in `docs/beta.md`. (3) **Two branches carry push-triggered workflows with `contents: write`** (`tagfix` force-moves tags; `backfill-releases` mints releases v2.0 to v2.2, which the CHANGELOG says were deleted), which gives the "review first" verdicts a concrete hazard line.

The QUAL-01 gate is already green (`0 open, 0 orphaned` on `de`, `es`, `fr`, `it`; `themewords -check` and `tools/english` clean; changelog tests pass), so that requirement is "do not break it". The only ways this phase can break it are changing an existing catalogue key (leaves an orphan, because the tool never deletes keys) or adding an operator string outside the three scanned roots.

**Primary recommendation:** Merge `origin/main` first; write `docs/beta.md` with every promise carrying a grep-checkable citation into the three records; add a `## 0.1.0 — <real date>` heading at the very top of the CHANGELOG with an absolute-URL link to `docs/beta.md` and an upgrade note for 0.0.25 operators (the deploy directory must be re-copied); build `docs/branches.md` from a script that reads git and `gh`; touch none of the six evidence-set files; end with the owner's two commands (`git push origin main`, then `git push origin main:release`).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Beta criteria (`docs/beta.md`) | Repository documents | README (front door link) | A contract that must be diffable on its own (D-01); cites records under `deploy/`. |
| Version/tag creation | GitHub Actions (`release.yml`) | Owner's shell / Actions button | The proxy refuses `refs/tags`; the workflow mints the tag through the Releases API and reads the CHANGELOG heading. |
| Release page text | `release.yml` awk over `CHANGELOG.md` | GitHub `--generate-notes` appended | The page body is the CHANGELOG section verbatim; no separate notes file. |
| Version shown to operators | Binary (`-ldflags -X main.Version`) + embedded `CHANGELOG.md` | Admin screen `/admin/neuerungen` | The embedded file is parsed by `internal/changelog`; the sidebar version comes from `git describe`. |
| "Alpha" label in the product | Binary (`web.IsAlpha`, admin templates, catalogues) | — | Derived from the major version number, not from any document. Untouched by default (see Open Question 1). |
| Orphan branch list | Repository documents (`docs/branches.md`) | GitHub web UI (deletion, owner) | Deletion is a human step; the proxy answers 403 to ref deletion. |
| i18n gate | CI + `tools/i18n` | — | Standing gate; scans three roots only. |

## Standard Stack

No packages are installed in this phase and no `go.mod` change is expected. The "stack" is existing project tooling, all present and run this session.

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| `go run ./tools/i18n` | in-repo | The QUAL-01 gate; baseline already `0 open, 0 orphaned` on de/es/fr/it | Project standing gate `[VERIFIED: ran this session, exit 0]` |
| `go run ./tools/themewords -check` | in-repo | Shipped-theme catalogue gate | Output: `every shipped theme's catalogue matches tools/themewords/words.json` `[VERIFIED: ran this session]` |
| `go run ./tools/english` | in-repo | CI step "No German in the Go source" | Output: `no German in the Go source outside the catalogues` `[VERIFIED: ran this session]` |
| `git` (`cherry`, `rev-list --left-right --count`, `merge-tree`, `merge-base --is-ancestor`) | system | Branch audit and merge trial | Patch-equivalence is the only check that sees through squash merges (D-15) |
| `gh` CLI | system (authenticated in this environment) | PR state per branch, release list | `gh pr list --state all` returned all 42 PRs `[VERIFIED: ran this session]` |

### Supporting
| Tool | Purpose | When to Use |
|------|---------|-------------|
| `.github/workflows/release.yml` | Turns a `release` branch push or button press into tag + release + binary | Owner step; Claude only verifies its inputs locally |
| `internal/changelog` + `changelog_test.go` (root) + `internal/admin/changelog_test.go` | Format contract of `CHANGELOG.md` | Run after editing the CHANGELOG |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Separate release-notes file | CHANGELOG section as the body | `release.yml` already publishes exactly the CHANGELOG section (D-11); a second file would be a second thing to forget |
| Hand-typed branch table | Script reading git and `gh` | D-13 requires it be generated; the branch `claude/holzcloud-projects-update-0t9epb` moved during this research (see Pitfall 2), so a hand list goes stale within a day |

**Installation:** none.

**Version verification:** not applicable (no packages). Go toolchain on this machine is `go1.27.2 darwin/arm64`; `go.mod`/CI pin Go 1.26 `[VERIFIED: go version output, README badge]`.

## Package Legitimacy Audit

No external packages are installed or recommended in this phase.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                     LOCAL (Claude, this phase)                          OWNER (human steps)
 ┌──────────────────────────────────────────────────────────┐   ┌───────────────────────────────────────┐
 │ 0. git fetch; git merge origin/main   (README rewritten)  │   │                                       │
 │ 1. baseline gates: i18n, themewords, english, changelog   │   │  a. git push origin main              │
 │ 2. docs/beta.md  <── cites ── deploy/{SECURITY-SCORE,     │   │  b. git push origin main:release      │
 │                               DEPLOY-RUN,BACKUP-RUN}.md   │   │       │   (or Actions > Release > Run) │
 │ 3. CHANGELOG.md: new top heading "## 0.1.0 — DATE"        │   │       ▼                               │
 │      ├─ links absolute URL -> docs/beta.md                │   │  release.yml                           │
 │      └─ upgrade note for operators on 0.0.25              │   │   grep -m1 '^## N.N(.N)' CHANGELOG.md  │
 │ 4. README: badge + notice + link to docs/beta.md          │   │   -> tag v0.1.0 (does not exist yet)   │
 │ 5. docs/deployment.md "Versioning" alpha -> beta wording  │   │   tests, wasm check, build amd64       │
 │ 6. docs/branches.md  <── script: git cherry + gh pr list  │   │   awk: CHANGELOG section -> notes      │
 │ 7. local rehearsal of release.yml's two shell snippets    │   │   gh release create --prerelease       │
 │ 8. final gate run; commit                                 │   │   (v0.* is always a pre-release)       │
 └──────────────────────────────────────────────────────────┘   │  c. GitHub UI: delete branches per     │
                                                                 │     docs/branches.md                   │
                                                                 └───────────────────────────────────────┘
```

### Recommended file set for the phase

```
docs/beta.md                 NEW  promises / non-promises, each promise with a citation
docs/branches.md             NEW  branch table + GitHub-UI steps (or a section in beta.md)
CHANGELOG.md                 EDIT new top entry only; nothing below it is touched
README.md                    EDIT badge line, warning block, Documentation list (after the merge)
docs/deployment.md           EDIT "Versioning" section (says "alpha", line ~168)
```

Files that must NOT be edited (they are the Phase 22 evidence set, D-06): `deploy/DEPLOY.md`, `deploy/backup.sh`, `deploy/restore.sh`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer`, `deploy/test/run.sh`. `deploy/test/README.md` is not in that set, but it is the hand-over document; leave it alone too.

### Pattern 1: A promise is a citation, checkable by `grep -F`

**What:** Every sentence under "What 0.1.0 promises" carries a backticked token that occurs verbatim in a record, and the plan's verification loops over those tokens (the Phase 21/22 pattern: a loop of `grep -F` proving claim equals record).
**When to use:** all of `docs/beta.md`'s promise list.
**Example:**
```bash
# every token in a "cites:" list must exist, byte for byte, in the record it names
while IFS='|' read -r file token; do
  grep -qF -- "$token" "$file" || { echo "NOT IN RECORD: $file: $token" >&2; bad=1; }
done <<'EOF'
deploy/BACKUP-RUN.md|Evidence commit: bf74496
deploy/BACKUP-RUN.md|BAK-02 | PASS
deploy/SECURITY-SCORE.md|Overall exposure level 1.3 online and 1.4 offline on systemd 252
EOF
test -z "${bad:-}"
# Source: pattern from .planning/phases/21-deploy-update-rollback/21-06-PLAN.md and 22-05-PLAN.md [VERIFIED: read]
```

### Pattern 2: Branch verdicts come from evidence in `main`, never from age

**What:** For each remote branch, record tip SHA, last commit date, `git rev-list --left-right --count origin/main...BRANCH` (left = only in main, right = only in the branch), `git cherry origin/main BRANCH` (`-` = patch-equivalent exists in main, `+` = not in main), and `gh pr list --state all` state.
**Rule:** only "right = 0" or "all `-`" may be called safe; any `+` is "review first" with the commit subject.
**Example:**
```bash
git fetch origin --prune
for b in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin \
           | grep -v -x -e origin -e origin/HEAD -e origin/main); do
  printf '%s %s %s\n' "$b" "$(git log -1 --format='%cs %h' "$b")" \
         "$(git rev-list --left-right --count origin/main..."$b")"
  git cherry origin/main "$b"
done
gh pr list --state all --limit 100 --json number,headRefName,state,title
# Source: commands run in this session [VERIFIED]
```
Note the column order of `--left-right --count A...B`: **first number = commits only in A (here: main), second = only in B (the branch)**. The CONTEXT's "ahead counts" are the second number.

### Anti-Patterns to Avoid
- **Verdict by age or by name:** `claude/*` looks disposable and is not uniformly so (`claude/holzcloud-projects-update-0t9epb` was pushed on 2026-10-13).
- **Editing the CHANGELOG body below the new entry:** the admin screen and the release page both read the whole file; history entries stay as written.
- **Putting a relative link in the CHANGELOG entry:** the entry is shown on the GitHub release page, where `docs/beta.md` does not resolve; use the absolute `https://github.com/holzcloud/holzcloud-cms/blob/main/docs/beta.md`. The admin screen renders the same text through goldmark + bluemonday, where an outbound hyperlink is allowed content `[VERIFIED: internal/changelog/changelog.go parse path; CLAUDE.md "An outbound hyperlink is content"]`.
- **Adding an "Unreleased" heading at the top of the CHANGELOG:** `release.yml` skips it (its regex needs digits) but `internal/changelog` would list it as a release.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Is this branch's work already in main? | A `git log` eyeball or an age cutoff | `git cherry origin/main BRANCH` plus `gh pr list --state all` | Squash merges (#39, #40) rewrite SHAs; only patch-ids and PR state see through them |
| Release page text | A hand-written notes file | The CHANGELOG section, extracted by `release.yml`'s own awk | The workflow already does it; a second copy diverges |
| Tag creation | `git tag` + push from the proxy session | `release.yml` via `git push origin main:release` or the Actions button | The proxy answers 403 to `refs/tags`, by git and by API `[CITED: .github/workflows/release.yml comments, DEPLOY.md "Making a release"]` |
| Version string | A constant in Go or a VERSION file | `git describe --tags` through `-ldflags -X main.Version` | Already the mechanism (`cmd/holzcloud/main.go:61-67`); a second source would disagree |
| Proof that a doc quote is real | Reading it again | The `grep -F` loop (Pattern 1) | Reading is what produced the Phase 21/22 defects |
| i18n completeness | A count of keys | `go run ./tools/i18n` (the gate) and its `-write` then `git diff --exit-code` CI twin | The tool owns the definition of open/orphaned |

**Key insight:** this phase invents nothing. Every mechanism it needs (release, changelog parse, gates, evidence rule) exists; the work is to feed each its correct input and to refuse to claim anything a record does not.

## What the beta page may cite (the evidence inventory)

All lines below were read in this session. Quoted tokens are verbatim and are the ones the `grep -F` loop should use.

### Phase 20, `deploy/SECURITY-SCORE.md` (D-04, D-05)
- Summary rows `[VERIFIED: deploy/SECURITY-SCORE.md:8-14]`: `UNIT-01: the unit starts and the site answers` PASS (the unchanged unit started; the first start on the emulated platform crashed and needed a container-only drop-in); `UNIT-02: MemoryDenyWriteExecute=true against wazero` is `SETTLED, and the unit keeps MemoryDenyWriteExecute=true. All five plugins load and run on wazero's interpreter fallback with the directive enforced. The evidence comes from native arm64 only. On amd64 the verdict is VOID`; `UNIT-03` is `Overall exposure level 1.3 online and 1.4 offline on systemd 252; 1.3 online and 1.3 offline on systemd 257. Measured, not tuned.`
- Commit of the measurement: `2026-10-12, f1428d2`; `deploy/holzcloud.service` "has not changed since the comment-only commit `dd38a10`" `[VERIFIED: SECURITY-SCORE.md:19-20]`.
- Non-promises from its own record: `No native amd64 run was made and none is planned (user decision).` (`:68`), the whole *Container vs. server* seccomp table (`:230-250`: `MemoryDenyWriteExecute=true`, `RestrictAddressFamilies`, `SystemCallFilter`, ... "no" on amd64), and *Not measured* (`:252-257`): native amd64; Ubuntu 24.04 with systemd 255 and other systemd versions than 252 and 257; Debian 12 on real hardware; QEMU emulation.
- Note the asymmetry the page must state honestly: seccomp enforcement was observed on **native arm64 only**, and only `MemoryDenyWriteExecute` and the address-family directive were probed there; the target platform is linux/amd64.

### Phase 21, `deploy/DEPLOY-RUN.md`
- Summary rows `[VERIFIED: DEPLOY-RUN.md:16-22]`: `DEP-01 PASS` (`33 steps played, 0 failed`, `CADDY: validated`, `LEAK-CHECK OK`); `DEP-02` PASS across migrations 58 to 60 (`MIGRATIONS: old=58 new=60 pending=2`, 52 PASS and no BAD against the pre-update state); `DEP-03 PASS` in both situations (snapshot: `MODE: snapshot`; binary only: `MODE: binary`) and "The limit, that an older binary must not be started against a migrated database, leads the section and was observed".
- **The record's text is bound to build `2a11eeb`** (`v0.0.25-42-g2a11eeb`), "recorded on top of 83b4348". `DEPLOY.md` was edited by Phase 22 afterwards (see Pitfall 3), so the page must say "as of `2a11eeb`", not "as of the current DEPLOY.md".
- Non-promises: *Not measured* (`:459-473`): native amd64; a real VPS image (Ubuntu 24.04); certificate issuance, DNS, ACME and a browser over https; five or more successive updates and snapshot pruning; a rollback of media and templates; the single sign-on paths; "A restore from a backup" (Phase 22 owns it); a reboot with the units enabled; the compulsory two-factor enrolment and the shop and form flows through a browser; a rollback after a failed migration; a production-size database. And the *Container vs. server* table (`:431-457`), whose **assumed** rows are the ones to name.

### Phase 22, `deploy/BACKUP-RUN.md` and the "Hand-over to phase 23" in `deploy/test/README.md:278-306`
- Permitted citations `[VERIFIED: deploy/test/README.md:280-284]`: the Summary table and the line `Evidence commit: bf74496` (`BACKUP-RUN.md:15`); the words of `summary.txt`: `BAK-01 PASS`, `BAK-02 PASS`, `BAK-03 PASS`, `RESTORE-NEGATIVE PASS`, `STEPS: 78 played, 0 failed`, `LEAK-CHECK OK`, and the two `POINT` lines "with the numbers in *Measurements*, which are emulation numbers: a restore of about eight seconds, a backup of about a second, both at 20 MB and at 320 MB"; the statements the record makes as shown; and the `HAND-REBUILT` list.
- Numbers, quoted as the record writes them `[VERIFIED: BACKUP-RUN.md:160-169]`: backup `BACKUP-BYTES` 20548955 and 335125851; `BACKUP-WALL-S` 0.692 and 1.015; `RESTORE-WALL-S` 8.126 and 8.296; `START-READY-S` 7.397 and 7.379.
- The non-promise list is already written: `deploy/test/README.md:295-304` ("They must not claim, copied from the record's *Not tested*") and `BACKUP-RUN.md:221-237`. D-05's nine items map one-to-one onto lines there; the record has four more worth carrying: a disk nearly full but above `MIN_FREE_MB`, the consequence of losing `HOLZCLOUD_SECRET_KEY`, a restore through a browser, and "the extrapolation in *Measurements* is an estimate".
- `HAND-REBUILT` (what no backup holds) `[VERIFIED: BACKUP-RUN.md:197]`: "the unit file and its drop-ins in `/etc/systemd/system`, environment files, `HOLZCLOUD_SECRET_KEY` and the payment and mail credentials that live in an environment file, the proxy configuration and the certificate state, and the pre-upgrade snapshots in the data directory."

### The evidence-commit rule holds today
`git diff --quiet bf74496 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh` exits 0 at the current `HEAD` `[VERIFIED: ran this session, "EVIDENCE RULE HOLDS at HEAD"]`. The plan must re-run it as its last check; if it ever prints non-zero, D-06 applies.

## Runtime State Inventory

Not a rename/refactor/migration phase; omitted. One related fact for the planner: no version string is hard-coded outside records and examples. A grep for `0.0.25` outside `.planning/`, `CHANGELOG.md` and `deploy/test/out` finds only recorded evidence (`deploy/DEPLOY-RUN.md`, one example snapshot name at `deploy/DEPLOY.md:736`, `deploy/test/run.sh:28` using `v0.0.21` as an old tag) and an unrelated `go.mod` indirect dependency `v0.0.24` `[VERIFIED: grep this session]`. `README.md` at the old line numbers carried no version number either.

## Common Pitfalls

### Pitfall 1: Local `main` and `origin/main` have diverged, and the README was rewritten upstream
**What goes wrong:** Editing `README.md` at the line numbers in CONTEXT (badge ~11, notice ~60) edits a file that no longer exists upstream; the owner's push then conflicts or silently drops the upstream rewrite.
**Why it happens:** `origin/main` gained `9cc7bcb` ("docs: slim the README, put four screenshots in it, move the admin tour to publishing.md", 2026-10-13) after the local branch point `32e05ea`; local carries 77 unpushed commits (`git rev-list --left-right --count origin/main...main` = `1 77`) `[VERIFIED: ran this session]`.
**How to avoid:** Task 0 of the first plan: `git fetch origin && git merge origin/main`. `git merge-tree --write-tree main origin/main` reported a clean merge, and the local side never touched `README.md`, `docs/publishing.md` or `CHANGELOG.md` since the merge base `[VERIFIED]`. Use a merge, not a rebase of 77 commits. Then re-read `README.md`: after the merge the status badge is at line 6 (`status-alpha`), the `> [!WARNING]` block at lines 21 to 24 ("Alpha software under heavy development. Features, settings, the template contract and the AI tools can change from one release to the next. Back up the data directory before every update and read the [changelog](CHANGELOG.md)."), and the `## Documentation` list at lines 98 to 109 (add the `docs/beta.md` link there) `[VERIFIED: git show origin/main:README.md]`.
**Warning signs:** `git status -sb` shows `ahead N, behind 1`; a README diff of ~250 lines.

### Pitfall 2: The branch list is a moving target
**What goes wrong:** `claude/holzcloud-projects-update-0t9epb` moved while this research ran (`git fetch` printed `b168e85..9cc7bcb`); a table typed from memory, or from CONTEXT's counts, is wrong by the time it is read.
**How to avoid:** Generate the table at execution time (Pattern 2) and print the generation date and the `origin/main` SHA in the document. State that verdicts hold for that SHA. Record each branch's tip SHA so a deleted branch can be restored (see Pitfall 7).
**Warning signs:** a `claude/*` branch with a commit dated today.

### Pitfall 3: DEPLOY.md changed after the Phase 21 evidence, and the beta page can over-claim
**What goes wrong:** The page says "a fresh install, update and rollback by `DEPLOY.md` were played" and a reader takes it for the current text.
**Fact:** `DEPLOY-RUN.md` is bound to `2a11eeb`. Since then `DEPLOY.md` changed in: Prerequisites (adds `rsync`), the Backups section (+33 lines), two wording sentences in Updating and Rolling back (the snapshot "covers the database only: media, templates, plugins, language files and the logo are not in it"), and *Restore from a backup* `[VERIFIED: git diff -U0 2a11eeb HEAD -- deploy/DEPLOY.md]`. No command in Install, Updating or Rolling back changed. The Backups and Restore blocks were re-played by Phase 22 against `bf74496`.
**How to avoid:** Word the DEP promises "as played against `2a11eeb`" and the BAK promises "as played against `bf74496`"; do not write "the current document was played end to end".

### Pitfall 4: The product and DEPLOY.md say "alpha"; the README will say "beta"
**What goes wrong:** An operator reads "beta" in the README and the release page, installs, and the admin dashboard says "holzcloud-CMS is alpha software. It is still being developed heavily..." with a sidebar tag "alpha" and a login-page line.
**Fact:** `web.IsAlpha(version)` returns true for any major number below 1 and for any non-version `[VERIFIED: internal/web/layoutdata.go:160-174, quoted: "return n < 1"]`; the test `TestIsAlpha` fixes `"v0.9.2-4-gabc1234": true` `[VERIFIED: internal/web/alpha_test.go]`. Three templates use it (`cmd/holzcloud/templates/admin/base.html:131`, `login.html:35`, `dashboard.html:6-8`) and three catalogue keys carry the wording: `"Alpha software: updates can still bring breaking changes."`, `"alpha"`, `"holzcloud-CMS is alpha software."` `[VERIFIED: grep this session]`. `deploy/DEPLOY.md:677` says "holzcloud-CMS is alpha software: until 1.0, an update may change or remove features, settings and the template contract without a transition period." `[VERIFIED: Read]`, and `docs/deployment.md:168` says "holzcloud-CMS is **alpha**".
**How to avoid:** See Open Question 1. Default: do not change the product or `DEPLOY.md`; change `docs/deployment.md` (not in the evidence set); and make `docs/beta.md` carry a short "Where the word alpha still appears, and why" paragraph. The sentences themselves ("an update may change or remove features...") are exactly what D-07 says beta means, so the substance agrees and only the label differs.

### Pitfall 5: Changing a catalogue key breaks `0 orphaned`
**What goes wrong:** Rewording the three alpha strings makes the tool report the old keys as `orphaned` in all four catalogues, because "A key that disappears from the source is reported but never deleted" `[VERIFIED: tools/i18n/main.go header comment]`; and CI runs `go run ./tools/i18n -write` then `-schweiz` then `git diff --exit-code -- internal/i18n/locales/`, so a new key must be translated by hand into de, es, fr, it (and de-CH regenerates).
**How to avoid:** If Open Question 1 is answered "change the product", the plan needs: new keys translated in four files, old keys deleted by hand in four files, `-schweiz` re-run, `IsAlpha` test intact, and the gate re-run. Treat it as a separate plan with its own verification. If it is answered "leave it" (the default), add no operator string at all and QUAL-01 is trivially held.

### Pitfall 6: The release workflow reads the CHANGELOG in specific ways
**Facts (all read in `.github/workflows/release.yml`):**
- With no input, the version is `grep -m1 -oE '^## [0-9]+\.[0-9]+(\.[0-9]+)?' CHANGELOG.md | sed 's/^## //'` and the tag is `v$version`. The **first** matching heading wins, so the 0.1.0 heading must be the first `## ` version heading in the file (above `## 0.0.25 — 2026-10-11`, which is at line 17 today).
- The notes are `awk -v v="$version" '/^## / { if (found) exit; if ($2 == v) { found = 1; next } } found { print }' CHANGELOG.md`. `$2` is the second whitespace field, so `## 0.1.0 — 2026-10-14` yields `0.1.0`. Do not put a second line starting with `## ` inside the entry; use `**bold**` lead-ins as the existing entries do (a `###` line is safe, but the file's style is bold).
- A tag that already exists on the remote makes the `release` branch route only refresh the notes and every other route refuse: remote tags today stop at `v0.0.25` and `v0.1.*` does not exist `[VERIFIED: git ls-remote --tags origin 'v0.1*' 'v0.0.25']`.
- `case "${TAG}" in v0.*) prerelease=(--prerelease)` makes 0.1.0 a **pre-release** on GitHub. Leave it: `docs/deployment.md` ("Every 0.x release is published as a pre-release") and `DEPLOY.md` ("Every release before 1.0 is published as a pre-release") both say so, and `DEPLOY.md` must not be edited. The comment in the workflow says "the software is alpha"; it is a comment, and editing a workflow file may also be refused by the proxy (the workflow notes a missing `.github/workflows` scope for the Actions token; whether the dev proxy refuses it is `[ASSUMED]`). Do not touch `release.yml` in this phase.
- The release body is the CHANGELOG section followed by GitHub's generated notes (`--notes-file ... --generate-notes`), so **the upgrade note must live inside the CHANGELOG entry** (D-11), and the CHANGELOG text is also what the owner's operators read in the admin ("What is new").
- The heading date is free text to the workflow and to `splitHeading` (`internal/changelog/changelog.go:161-169` splits on ` — `, ` – ` or ` - `). A literal `2026-10-xx` would parse and would be shown in the admin and embedded in the binary. Prefer a real date: the day the entry is committed (the file's own rule: "a heading typed on the day it is written"), and tell the owner that the entry's date is the planned tag day.
- Operationally the job runs `go run ./tools/wasm -check` and `go test -timeout 20m ./...` before it publishes; a red CI on the merge blocks the release. The `holzcloud-ch` job afterwards writes the version into holzcloud.ch pages through `.github/cms-version.py` (needs the `CMS_TOKEN` secret; without it, it exits 0). Pages on holzcloud.ch outside this repository may still say "Alpha" in their status boxes; that is an owner follow-up outside the repository `[VERIFIED: cms-version.py docstring mentions "Alpha 0.2.2" status text]`.

### Pitfall 7: "The restore link GitHub offers after deletion" exists only for branches with a pull request
**What goes wrong:** D-16 asks the steps to "note the restore link"; for a branch that never had a PR (`backfill-releases`, `claude/task-phases-milestones-open-mt1qz0`, `tagfix`, `release`) GitHub offers none.
**Fact:** The GitHub documentation page for deleting a branch lists the steps (repository main page, branch dropdown, **View all branches**, trash icon next to the branch, confirm if an open pull request exists) and says that deleting a branch with an open pull request closes it; it does **not** describe restoring `[CITED: docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-and-deleting-branches-within-your-repository]`. The "Restore branch" button on a closed or merged pull request page is real but is documented elsewhere (`Deleting and restoring branches in a pull request`) and was not fetched `[ASSUMED]`.
**How to avoid:** In `docs/branches.md` give both routes: the PR-page button for branches that had a PR (merged PRs #13, #33, #34, #39, #40 and the `claude/project-open-tasks-lwuj02` set), and for every branch the tip SHA in the table so it can be recreated with `git push origin <sha>:refs/heads/<name>` from any clone that still has the object `[ASSUMED]: standard git; GitHub keeps unreachable objects for some time, how long is not documented here`. Open PRs at research time: none (`gh pr list --state open` returned `[]`), so no deletion closes a live PR.

### Pitfall 8: Two branches are live hazards, not merely stale
**Facts `[VERIFIED: git show of both workflow files]`:**
- `origin/tagfix` carries `.github/workflows/tagfix.yml` with `on: push: branches: [ tagfix ]` and `permissions: contents: write`. It recreates every tag listed in `.tagfix/list.txt` with `git tag -f` and pushes them with `+refs/tags/...` (force) refspecs, then runs `git push origin --delete tagfix`. A push to that branch would re-run it. The file says it deletes the branch afterwards; the branch still exists, so that step never completed or failed.
- `origin/backfill-releases` carries `.github/workflows/backfill.yml` with `on: push: branches: [ 'backfill-releases' ]`, `contents: write`, to "Publish v2.0, v2.1 and v2.2 at the commits that closed them". `CHANGELOG.md`'s own preamble says those releases and their tags "were deleted when the count started again".
**How to use it:** both are `+` in `git cherry` (not in main), so by D-15 they are **"review first"**, never "safe to delete"; the document states the facts above as the reason a human should look, and does not decide. Both have no pull request.

### Pitfall 9: Wrong inference from `git cherry` direction
`git cherry origin/main BRANCH` marks a commit `-` when main has a patch-equivalent one and `+` when it does not. Results this session: `claude/project-open-tasks-lwuj02` four `-`; `feat/holzwave-farben` one `-`; `fix/tables-mobile` one `-`; `backfill-releases` one `+`; `tagfix` one `+` `[VERIFIED]`. The same results hold against local `main`. For the branches with a squash merge (#39, #40) `git rev-list` alone would say "1 commit ahead"; only the `-` plus the merged PR makes "safe".

## Verdict table, measured 2026-10-14 (starting point; regenerate at execution)

`origin/main` was `9cc7bcb` at measurement. Columns: last commit date and short SHA (tip), left/right = only in main / only in branch, cherry, PR state.

| Branch | Tip (date, sha10) | main-only / branch-only | cherry | PRs | Verdict | Reason |
|--------|-------------------|------------------------|--------|-----|---------|--------|
| `release` | 2026-10-11 `7878f57ff0` | 2 / 0 | none ahead | none | **Keep** | The button in `release.yml` (route 2); an ancestor of main by design, fast-forwarded to release |
| `claude/holzcloud-projects-update-0t9epb` | 2026-10-13 `9cc7bcb7bd` | 0 / 0 | none ahead | #13 merged | Safe to delete | Tip equals `origin/main`; a Claude session pushes here, so check none is running (it was pushed to on 2026-10-13) |
| `claude/project-open-tasks-lwuj02` | 2026-10-11 `58b0963a49` | 6 / 4 | 4 x `-` | 21 PRs merged, #12 closed | Safe to delete | All four commits patch-equivalent in main (#41, #42 squash) |
| `claude/task-phases-milestones-open-mt1qz0` | 2026-09-14 `044d51c1d9` | 165 / 0 | none ahead | none | Safe to delete | Ancestor of main (0 branch-only) |
| `feat/holzwave-farben` | 2026-10-10 `88e9450a9d` | 11 / 1 | 1 x `-` | #40 merged | Safe to delete | Squash-merged as `2740cc3` |
| `fix/tables-mobile` | 2026-10-09 `40c9422fd9` | 12 / 1 | 1 x `-` | #39 merged | Safe to delete | Squash-merged as `fa55c0b` |
| `holzbar-colours` | 2026-10-02 `dce6ed2029` | 40 / 0 | none ahead | #33 merged | Safe to delete | Ancestor of main |
| `redirects-keep-language` | 2026-10-02 `42c4c20da6` | 38 / 0 | none ahead | #34 merged | Safe to delete | Ancestor of main |
| `backfill-releases` | 2026-09-15 `4b14672344` | 158 / 1 | 1 x `+` | none | **Review first** | Unmerged commit "One-shot, second push: finish v2.1 and v2.2" adds a push-triggered release workflow; the releases it would mint were deleted per CHANGELOG preamble |
| `tagfix` | 2026-09-26 `98791fbc4b` | 107 / 1 | 1 x `+` | none | **Review first** | Unmerged commit "ci: one-off tagfix to point tags at the rewritten history" adds a push-triggered workflow that force-pushes tags |

Eight of ten are fully contained in `main`; two need a human look. The CONTEXT's D-13 "ahead counts 1, 0, 4, 0, 1, 1, 0, 0, 0, 1" were reproduced exactly (the branch-only column).

## Operator upgrade facts for the 0.1.0 entry (D-11, D-12)

All from `git diff v0.0.25 main` outside `.planning/`; no `.go` file changed `[VERIFIED]`.

- **What did not change:** the binary's behaviour. Migrations stand at `00060_order_token.sql`; none added `[VERIFIED: ls internal/db/migrations | tail -3]`. Say so: updating from 0.0.25 migrates nothing.
- **The hardened unit:** `deploy/holzcloud.service` gained seven comment lines explaining `MemoryDenyWriteExecute=true` against wazero; no directive changed `[VERIFIED: git diff]`. Phase 20 settled the question: the unit keeps the directive and plugins run on the interpreter fallback (native arm64 evidence).
- **Backups were incomplete before and are not now (F-01):** the 0.0.25 `backup.sh` copied the database, the CSRF key, `media` and `templates` only. The repaired script and `restore.sh` also carry `plugins`, `sprachen` and `marke` (`for name in media templates plugins sprachen marke`), `restore.sh` refuses a directory without `MANIFEST` and sets mode 600 on the restored database `[VERIFIED: git diff v0.0.25 main -- deploy/backup.sh deploy/restore.sh]`.
- **Why an update does not fix this by itself:** the scripts and units are not in the binary; they live in `/opt/holzcloud/deploy` on the server. An operator who installed from the 0.0.25 `DEPLOY.md` copied only `backup.sh`, to `/opt/holzcloud/backup.sh`, never installed `restore.sh`, and the shipped backup unit's `ExecStart` is `/opt/holzcloud/deploy/backup.sh` `[VERIFIED: git show v0.0.25:deploy/DEPLOY.md lines 89-90; deploy/holzcloud-backup.service:10; DEPLOY-RUN.md Findings row "DEP-01 | Install"]`. So the upgrade action is: copy the repository's `deploy` directory to the server as `/opt/holzcloud/deploy` (the document's Install block does it with `scp -r deploy/ ...` then `sudo cp -r /tmp/deploy /opt/holzcloud/deploy`), make the two scripts executable, and install `rsync` (new named prerequisite: both scripts call it). Take the exact commands from the current `DEPLOY.md` Install and Backups sections rather than retyping them.
- **Documents corrected by the plays** (Phase 21 found ten defects in the install pass; the Updating section gained a pre-update backup step, `holzcloud.prev`, a readiness check and a stamped build line; Rolling back is new, with the migration limit first) and `docs/deployment.md` and `docs/configuration.md` now say what a backup holds and lacks.
- **Honest framing sentence for the entry:** this release adds no feature; it adds proof, repairs, and the written beta criteria.

## Code Examples

### Rehearse `release.yml` against the edited CHANGELOG, locally
```bash
# Source: the workflow's own two snippets, copied from .github/workflows/release.yml [VERIFIED: read]
version=$(grep -m1 -oE '^## [0-9]+\.[0-9]+(\.[0-9]+)?' CHANGELOG.md | sed 's/^## //')
test "$version" = "0.1.0" || { echo "first heading is $version, not 0.1.0" >&2; exit 1; }
awk -v v="$version" '
  /^## / { if (found) exit; if ($2 == v) { found = 1; next } }
  found { print }
' CHANGELOG.md > /tmp/release-notes.md       # use the session scratchpad instead of /tmp
test -s /tmp/release-notes.md && wc -l /tmp/release-notes.md
git ls-remote --tags origin v0.1.0 | grep -q . && echo "TAG EXISTS - release.yml would refuse" || echo "tag free"
```

### The gates, in the order to run them
```bash
go run ./tools/i18n            # expect on de, es, fr, it: "N translated, 0 open, 0 orphaned"; exit 0
go run ./tools/themewords -check
go run ./tools/english
go test ./internal/changelog/ ./internal/admin/ -run 'Changelog|TestEvery|TestNewest|TestFind|TestPreamble|TestNoScript|TestMajor' .
go test .                       # root package: embed + .dockerignore keeps CHANGELOG.md
```
The i18n tool prints one line per catalogue (`de.json   2056 translated, 0 open, 0 orphaned`), not a single combined line; the combined phrase in the requirement is the sum of those lines `[VERIFIED: ran this session]`.

### The owner's closing steps (to be written at the end of the plan, as the only way "the tag is set")
```bash
git push origin main                 # publishes the 77+ local commits and the merge
git push origin main:release         # fast-forward; starts release.yml (route 2), which mints v0.1.0
# or: GitHub > Actions > Release > Run workflow, no inputs (reads the CHANGELOG, builds from the default branch)
```
Order matters: route 3 (button) builds from the head of the **default branch** on GitHub, so `main` must be pushed first; route 2 uploads the objects itself but leaves `origin/main` behind. Verification afterwards: `git ls-remote --tags origin v0.1.0` and `gh release view v0.1.0 --json isPrerelease,body`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| "Tag by hand" | `release` branch push or Actions button; the Releases API mints the tag | 2026-09-14/15 (measured, per workflow comments) | The proxy cannot write `refs/tags`; no tag is ever pushed from this environment |
| Release page = GitHub's generated "Full Changelog" link | Page = CHANGELOG section + generated notes | v2.9 (per workflow comment) | The CHANGELOG entry is the release notes |
| One README with everything | Slim README plus `docs/*.md` | `9cc7bcb`, 2026-10-13 | Beta page is a `docs/` file linked from the README's Documentation list |

**Deprecated/outdated:** the CONTEXT's README line numbers (badge ~11, notice ~60): stale after the upstream rewrite.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A "Restore branch" button exists on a merged or closed PR page for a deleted head branch | Pitfall 7 | Low: `docs/branches.md` would mention a button that is not there; the SHA route covers it. Verify by reading GitHub's "Deleting and restoring branches in a pull request" article before writing the sentence |
| A2 | `git push origin <sha>:refs/heads/<name>` recreates a deleted branch for as long as GitHub retains the objects | Pitfall 7 | Low-medium: the table's tip SHAs would not restore anything after garbage collection; state "as long as the object is still available" or drop the claim |
| A3 | The dev proxy may refuse a push that modifies `.github/workflows/*` | Pitfall 6 | None if `release.yml` stays untouched (the recommendation) |
| A4 | Leaving the in-app "alpha" label and `DEPLOY.md:677` unchanged in 0.1.0 is acceptable to the owner | Pitfall 4, Open Question 1 | Medium: the owner may want the product to say beta; the cost of doing so is in Pitfall 5 |
| A5 | The holzcloud.ch project pages (outside this repository) show "Alpha" status text that the release job's `cms-version.py` will rewrite only for the number | Pitfall 6 | Low: owner follow-up, not in scope |
| A6 | `gh` authentication stays available at execution time (it worked here) | Standard Stack | Low: without it the PR column is dropped and `git cherry` alone is used; say so in the document |

## Open Questions

1. **Does 0.1.0 keep saying "alpha" inside the product and in `DEPLOY.md`?**
   - What we know: `IsAlpha` is `major < 1` (`internal/web/layoutdata.go:166-174`), so every 0.x admin shows the sidebar tag, the login line and the dashboard banner; `DEPLOY.md:677` and `docs/deployment.md:168` say alpha. CONTEXT D-10 changes only README (badge and warning). CONTEXT also says "Code changes only where a gate (QUAL-01) or the version stamp requires them." Neither requires this.
   - What's unclear: whether the owner wants the label changed now.
   - Recommendation (default, needs only owner veto): change nothing in the product and nothing in `DEPLOY.md`; edit `docs/deployment.md`'s Versioning paragraph; add to `docs/beta.md` a short paragraph "Where the word alpha still appears" naming the admin banner (derived from the major version, goes away at 1.0) and the `DEPLOY.md` Updating notice, and saying that their substance (an update may change or remove features) is what beta means here. Reason: changing either triggers an evidence rerun (`DEPLOY.md`) or a catalogue key swap with manual orphan cleanup in four files (product), and both are outside "documents plus a version bump". If the owner overrides: product change is one plan (new keys in de/es/fr/it, old keys removed by hand, `-schweiz`, `go test ./internal/web ./internal/admin`), `DEPLOY.md` change means rerun `backup-chain` (about 238 s, needs the Docker VM) and `fresh` and `lifecycle`, then re-record `Evidence commit`.

2. **What date goes in the heading?**
   - What we know: the workflow ignores the date; the admin screen and the embedded CHANGELOG show it; D-08 says "date of the actual tag day" and offers `2026-10-xx`.
   - Unclear: the tag day is the owner's choice.
   - Recommendation: write the commit day (2026-10-14 unless execution slips) and tell the owner in the closing steps that a different tag day means editing that one line before `git push origin main:release`. Do not commit the literal `xx`.

3. **Should the upgrade note also go in `docs/` as a page?** CONTEXT leaves it to discretion provided the release body shows it. Recommendation: CHANGELOG paragraph only; the body is exactly that section, a `docs/` page would not appear on the release page.

4. **Which GitHub steps wording for `claude/holzcloud-projects-update-0t9epb`?** It is in active use by a session; recommend the table says "safe to delete; first make sure no session is working on it".

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `go` | gates, tests | yes | go1.27.2 darwin/arm64 (project pins 1.26) | — |
| `git` | branch audit, merge, evidence rule | yes | system | — |
| `gh` (authenticated) | PR state per branch, `release list` | yes | worked this session | drop the PR column; `git cherry` still decides |
| Network to `origin` | `git fetch`, `ls-remote --tags` | yes | worked this session | state the generation date and SHA; do not claim current |
| Docker | only if D-06 forces an evidence rerun (not planned) | client present, daemon not probed | 29.8.2 client | n/a under the recommended default |
| `jq`, `awk`, `grep` | rehearsal and the `grep -F` loop | yes | system | — |

**Missing dependencies with no fallback:** none for the recommended plan.

## Validation Architecture

Skipped: `workflow.nyquist_validation` is `false` in `.planning/config.json` `[VERIFIED: Read]`. The executable checks the planner should wire as each task's verify step are nonetheless listed under Code Examples (gates, release rehearsal, `grep -F` citation loop) and the evidence rule above; none needs a new test file. Existing coverage of the CHANGELOG: `TestTheChangelogIsEmbedded`, `TestTheDockerContextKeepsTheChangelog` (root), `TestEveryReleaseHasANumberAndAnEntry`, `TestTheNewestReleaseIsFirst`, `TestFindAnswersOnlyForVersionsThatExist`, `TestThePreambleIsNotAnEntry`, `TestNoScriptSurvivesIntoAnEntry` (`internal/changelog`), and the admin screen tests in `internal/admin/changelog_test.go`. These are format smoke tests; **no existing test ties the CHANGELOG to a version number**, so D-18's "tests that tie the CHANGELOG to the version" is, strictly, the release workflow's heading parse, which the rehearsal snippet covers. No test reads `README.md` or `docs/*.md` (`grep` of `*_test.go`); `cmd/holzcloud/deploy_docs_test.go` reads only `deploy/DEPLOY.md` and `deploy/Caddyfile.example` for Caddy/SSO strings and is unaffected by this plan as long as `DEPLOY.md` is not edited.

## Security Domain

`security_enforcement` is not set to false, so it applies; this phase handles no user input and adds no endpoint, so most ASVS categories are out of scope.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no (one repository-level item below) | Branch deletion is by the owner in the GitHub UI |
| V5 Input Validation | partly | The CHANGELOG is rendered in the admin through goldmark + bluemonday (`page.RenderMarkdown`); keep the entry plain Markdown, no raw HTML (`TestNoScriptSurvivesIntoAnEntry` holds it) |
| V6 Cryptography | no | — |
| V10 Malicious code / V14 Config (supply chain) | yes | Do not change `release.yml` or any pinned action; no new packages; no external subresource in `docs/` or README beyond existing shields.io badges (README is not served by the app) |

### Known Threat Patterns for this phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Over-claiming security or durability in release text (a promise the evidence does not hold) | Repudiation / Information disclosure | Every promise carries a `grep -F` checkable citation; every non-promise is named (D-03, D-05) |
| A push-triggered workflow with `contents: write` left on a stale branch (`tagfix`, `backfill-releases`) | Tampering / Elevation | Document as "review first" with the trigger and the write scope; owner decides; Claude deletes nothing |
| A stale claim that a record covers the current `DEPLOY.md` | Repudiation | Bind each claim to its evidence commit (`2a11eeb`, `bf74496`) and re-run the `git diff --quiet` evidence rule at the end |
| Pushing a tag from an agent session | Tampering | Not done; `release.yml` mints it; owner runs the one command |
| A backup advertised as complete | Information disclosure / Denial of service | State what no backup holds (`HAND-REBUILT`) and the bit-rot limit (a flipped byte inside a data page passes both integrity readers) |

## Project Constraints (from CLAUDE.md)

- **GSD workflow enforcement:** file-changing work starts through a GSD command; this phase runs under `/gsd-execute-phase`. No direct edits outside it.
- **Translation:** every operator-read word goes through the catalogue; `go run ./tools/i18n` is the gate (`0 open, 0 orphaned`). The collector reads only `cmd/holzcloud/templates/admin`, `internal` and `plugins/`; a string added anywhere else is unreported. A plugin or a theme has its own route. This phase should add no operator string.
- **Public themes:** the eight shipped themes are generated from `tools/themewords/words.json`; `go run ./tools/themewords -check` is a CI step and must stay green.
- **Nothing loads at runtime:** nothing in this phase may add a third-party subresource to any template or asset. README badges are outside the served application.
- **Build:** `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud`; `Version`/`Commit` are injected with `-X`.
- **Conventions:** Go files `snake_case.go`; docs under `deploy/` and `docs/` are English; CHANGELOG entries are German operator prose in whole sentences (`CHANGELOG.md` preamble).
- **Git identity (user memory):** commits are made as `holzcloud.ch` with the noreply address; `git config` here already reads `holzcloud.ch` and `6815936+holzcloud@users.noreply.github.com` `[VERIFIED]`. Commit messages end with the attribution lines the session supplies.
- **User preference (memory):** questions to the user go through `AskUserQuestion` with options; relevant if Open Question 1 is escalated rather than defaulted.

## Sources

### Primary (HIGH confidence, read or run this session)
- `/Users/cheidenreich/privat/holzcloud-cms/.github/workflows/release.yml` (whole file) - heading regex, awk extraction, prerelease flag, tag minting, `holzcloud-ch` job.
- `/Users/cheidenreich/privat/holzcloud-cms/changelog.go`, `changelog_test.go`, `internal/changelog/changelog.go`, `internal/changelog/changelog_test.go`, `internal/admin/changelog_test.go` - format contract.
- `CHANGELOG.md` lines 1 to 140; `README.md` (local lines 1 to 120, and `origin/main:README.md`); `docs/deployment.md` 160 to 200; `deploy/DEPLOY.md` 673 to 682 and 1015 to 1080; `deploy/test/README.md` 200 to 353; `deploy/BACKUP-RUN.md` 1 to 240; `deploy/DEPLOY-RUN.md` 1 to 28, 140 to 175, 431 to 474; `deploy/SECURITY-SCORE.md` 8 to 69, 106 to 135, 211 to 268.
- `internal/web/layoutdata.go:150-174`, `internal/web/alpha_test.go`, `cmd/holzcloud/templates/admin/{base,login,dashboard}.html`, `internal/i18n/locales/*.json` (alpha keys), `tools/i18n/main.go` header, `.github/workflows/ci.yml` gates.
- git (fetch, cherry, rev-list, merge-tree, diff, show, ls-remote), `gh pr list`, `gh release list`, and the gate runs listed under Standard Stack.

### Secondary (MEDIUM confidence)
- GitHub Docs, "Creating and deleting branches within your repository" (fetched 2026-10-14): steps and open-PR behaviour; silent on restoring.

### Tertiary (LOW confidence, `[ASSUMED]`)
- The restore-branch button and the retention of unreachable objects (A1, A2).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - nothing new; each tool run this session.
- Architecture: HIGH - the release path and CHANGELOG parsing were read from source, and the branch audit was run.
- Pitfalls: HIGH for 1 to 6, 8 and 9 (observed); MEDIUM for 7 (partly from a docs page that is silent on restore).

**Research date:** 2026-10-14
**Valid until:** about 2026-10-21 for the branch table and the `origin/main` divergence (they move daily); 30 days for everything else.
