# Phase 23: Repository tidy and beta criteria - Pattern Map

**Mapped:** 2026-10-14
**Files analyzed:** 5 new/modified (+1 generated check script, scratch only)
**Analogs found:** 5 / 5 (all documents; no Go code is expected)

All analog paths below were checked with `git ls-files` and are tracked. Line numbers for README refer to `origin/main:README.md` (after the mandatory `git fetch && git merge origin/main`); the local README is stale.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `docs/beta.md` | doc (contract) | citation-to-record (grep -F checkable) | `deploy/test/README.md` "Hand-over to phase 23" (lines 278-306) + `deploy/BACKUP-RUN.md` Summary / *Not tested* | role-match (best available) |
| `docs/branches.md` | doc (audit table + manual steps) | generated from git/gh | `deploy/BACKUP-RUN.md` table style (Summary, Measurements) + `docs/security.md` house style | partial |
| `CHANGELOG.md` | release notes | parsed by `release.yml` awk and `internal/changelog` | top entries `## 0.0.25` (lines 17-41 of the file) | exact |
| `README.md` | front door | static | `origin/main:README.md` badge (line 6), `> [!WARNING]` (lines 21-24), `## Documentation` list (98-109) | exact (edit in place) |
| `docs/deployment.md` | doc | static | itself, "Versioning" section (~line 166-172) | exact (edit in place) |

## Pattern Assignments

### `docs/beta.md` (doc, citation-to-record)

**Analog:** `deploy/test/README.md` lines 278-306 (what may and may not be claimed) and `deploy/BACKUP-RUN.md` (Summary table, `Evidence commit`, *Not tested*).

**Voice and shape to copy.** English, whole sentences, the record states what was shown and what was not. Two lists of equal weight: *promises* (each with a backticked citation token that occurs verbatim in a record) and *non-promises* (each named). Bind each claim to its evidence commit, as BACKUP-RUN does:

```markdown
Evidence commit: bf74496
```
(`deploy/BACKUP-RUN.md:15`; the DEP promises bind to `2a11eeb` per `DEPLOY-RUN.md`, the UNIT promises to `f1428d2` per `SECURITY-SCORE.md:19-20`.)

**Promise tokens (verbatim, for the grep -F loop)** (RESEARCH "evidence inventory"):
- `deploy/BACKUP-RUN.md`: `Evidence commit: bf74496`, `BAK-01 | PASS`-style rows from the Summary table (row text starts `| BAK-01 | PASS.`), and the `summary.txt` words `BAK-01 PASS`, `BAK-02 PASS`, `BAK-03 PASS`, `RESTORE-NEGATIVE PASS`, `STEPS: 78 played, 0 failed`, `LEAK-CHECK OK`. Confirm each token's exact bytes in the record before writing it (some live only in the Appendix).
- `deploy/DEPLOY-RUN.md`: `33 steps played, 0 failed`, `LEAK-CHECK OK`, `MIGRATIONS: old=58 new=60 pending=2`, `MODE: snapshot`, `MODE: binary`.
- `deploy/SECURITY-SCORE.md`: `Overall exposure level 1.3 online and 1.4 offline on systemd 252`, `SETTLED, and the unit keeps MemoryDenyWriteExecute=true`.

**Non-promise list to copy (D-05)** from `deploy/test/README.md:295-304` (the exact list the hand-over authorises), copied nearly verbatim:

```markdown
- The real 03:15 window: the timer was moved a minute ahead by a container-only drop-in.
- `Persistent=true` after a missed run, and `RandomizedDelaySec` (set to 0 by the drop-in).
- The off-site `REMOTE_TARGET` copy, and retention pruning (no directory was older than the retention).
- An atomic database-plus-media snapshot: the pair is taken one after the other with the site running, and the window was not provoked.
- A backup older than the binary that restores it, in the final set.
- A restore on a real server, with a separate data volume, a cold cache and a real network copy.
- Native amd64 timing: every number is an emulation number on an aarch64 host, and the extrapolation in the record is an estimate, not a measurement.
- Protection against bit rot in a complete backup: a flipped byte inside a data page passes both integrity readers.
```
Add from `BACKUP-RUN.md` *Not tested* (lines ~221-237): disk nearly full above `MIN_FREE_MB`, losing `HOLZCLOUD_SECRET_KEY`, a restore through a browser. Add Phase 20/21 non-promises from `SECURITY-SCORE.md` (`:68` "No native amd64 run was made and none is planned (user decision).", *Container vs. server* table `:230-250`, *Not measured* `:252-257`) and `DEPLOY-RUN.md` *Not measured* (`:459-473`). State the asymmetry: seccomp enforcement observed on native arm64 only; target is linux/amd64.

**What no backup holds** (`HAND-REBUILT`, `BACKUP-RUN.md:197`): quote as written, including `HOLZCLOUD_SECRET_KEY` and the unit file/drop-ins.

**Also include** (D-07, Pitfall 4): "beta means safe to run with backups, not feature-frozen; migrations/schema not promised stable; rollback limit as in DEPLOY.md" and a short paragraph "Where the word alpha still appears" (admin banner from `web.IsAlpha`, `deploy/DEPLOY.md:677`).

**Rerun rule to copy** (`deploy/test/README.md:306`): "A change to `deploy/DEPLOY.md`, `backup.sh`, `restore.sh`, the units or `run.sh` after the evidence commit means the set is made again before the record is cited for the changed text."

**Verification pattern (grep -F loop)** (RESEARCH Pattern 1, from 21-06/22-05 plans):
```bash
while IFS='|' read -r file token; do
  grep -qF -- "$token" "$file" || { echo "NOT IN RECORD: $file: $token" >&2; bad=1; }
done <<'EOF'
deploy/BACKUP-RUN.md|Evidence commit: bf74496
EOF
test -z "${bad:-}"
git diff --quiet bf74496 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh
```

---

### `CHANGELOG.md` (release notes, format contract)

**Analog:** the `## 0.0.25 — 2026-10-11` entry at the top of `CHANGELOG.md` (line 17 onward).

**Heading and shape** (copy exactly: `## ` + number + space + em dash + ISO date; one-sentence lead; bold lead-in paragraphs, no second `## ` or `###` inside the entry; German operator prose in whole sentences):
```markdown
## 0.0.25 — 2026-10-11

Menüeinträge lassen sich bearbeiten, und die Eintragsliste ist auf dem Handy lesbar.

**Einträge bearbeiten statt löschen und neu anlegen.** Jeder Eintrag im Menüeditor
hat jetzt die Schaltfläche „Bearbeiten“. ...
```
New entry `## 0.1.0 — <real date>` goes above line 17 and must be the first version heading (release.yml uses `grep -m1`). Do not edit anything below. Use an absolute URL `https://github.com/holzcloud/holzcloud-cms/blob/main/docs/beta.md` (relative links do not resolve on the release page). No raw HTML, no "Unreleased" heading. The upgrade note (copy `deploy/` directory to `/opt/holzcloud/deploy`, chmod the two scripts, install `rsync`) lives inside the entry because the release body is exactly this section.

**Format contract / checks:** `release.yml` snippets (RESEARCH "Rehearse release.yml"):
```bash
version=$(grep -m1 -oE '^## [0-9]+\.[0-9]+(\.[0-9]+)?' CHANGELOG.md | sed 's/^## //')
awk -v v="$version" '/^## / { if (found) exit; if ($2 == v) { found = 1; next } } found { print }' CHANGELOG.md
```
Tests: `go test . ./internal/changelog/ ./internal/admin/` (`TestEveryReleaseHasANumberAndAnEntry`, `TestTheNewestReleaseIsFirst`, `TestNoScriptSurvivesIntoAnEntry`, `TestTheChangelogIsEmbedded`).

---

### `README.md` (front door, edit in place after merge)

**Analog:** itself at `origin/main`.

Badge (line 6): change `status-alpha-e3a36b` to a beta slug and `alt="Status: alpha"` to `alt="Status: beta"`:
```html
<img src="https://img.shields.io/badge/status-alpha-e3a36b?style=flat-square&labelColor=14201c" alt="Status: alpha">
```
Warning block (lines 21-24), keep the `> [!WARNING]` form, rewrite text to match `docs/beta.md` and promise no more than it:
```markdown
> [!WARNING]
> **Alpha software under heavy development.** Features, settings, the template
> contract and the AI tools can change from one release to the next. Back up the
> data directory before every update and read the [changelog](CHANGELOG.md).
```
Documentation list (lines 98-109): add one bullet in the existing `- [Name](path): description` form, e.g. `- [Beta criteria](docs/beta.md): what 0.1.0 promises and what it does not`.

---

### `docs/deployment.md` (edit in place)

**Analog:** its own "Versioning" paragraph (~line 166-168):
```markdown
holzcloud-CMS is **alpha**, and the tags count from **`v0.0.1`**. Until 1.0 a
release may break what the one before it did: back up before every update and
read the changelog first. Every 0.x release is published as a pre-release.
```
Reword "alpha" to beta, keep "Every 0.x release is published as a pre-release" (release.yml `v0.*` rule, DEPLOY.md says the same) and link `beta.md`. This file is NOT in the Phase 22 evidence set.

---

### `docs/branches.md` (audit table, generated)

**Analog:** table style of `deploy/BACKUP-RUN.md` Summary (pipe table with a verdict column and a reason column) and the heading/bullet house style of `docs/security.md` (`# Title`, linked list at top, `## Section`).

**Generation (Pattern 2, RESEARCH):**
```bash
git fetch origin --prune
for b in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin | grep -v -x -e origin -e origin/HEAD -e origin/main); do
  printf '%s %s %s\n' "$b" "$(git log -1 --format='%cs %h' "$b")" "$(git rev-list --left-right --count origin/main..."$b")"
  git cherry origin/main "$b"
done
gh pr list --state all --limit 100 --json number,headRefName,state,title
```
Columns: branch, last commit date and tip SHA, main-only/branch-only, cherry, PR, verdict, reason. Print generation date and `origin/main` SHA in the document. Verdict rule: only all-`-` or 0 branch-only is "safe"; any `+` is "review first" with commit subject; `release` is "keep" (the button in `release.yml`). Starting verdicts are in RESEARCH "Verdict table" (8 safe, `backfill-releases` and `tagfix` review first, with the push-triggered `contents: write` workflow facts). Steps: GitHub repo page, branch dropdown, View all branches, trash icon; PR page "Restore branch" only for branches that had a PR (A1 assumed, verify wording); `git push origin <sha>:refs/heads/<name>` (A2 assumed). Keep the note that the proxy answers 403 to ref deletion.

## Shared Patterns

### Evidence-commit rule (D-06)
**Source:** `deploy/test/README.md:306`, `deploy/BACKUP-RUN.md:15`.
**Apply to:** `docs/beta.md`, and the final plan check. Touch none of: `deploy/DEPLOY.md`, `deploy/backup.sh`, `deploy/restore.sh`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer`, `deploy/test/run.sh`. Final check: `git diff --quiet bf74496 HEAD -- <those six>` exits 0.

### Claims equal the record, shown by grep -F
**Source:** Phase 21/22 plans (`21-06-PLAN.md`, `22-05-PLAN.md`) via RESEARCH Pattern 1.
**Apply to:** every promise in `docs/beta.md`, the README wording, the CHANGELOG entry.

### Human-only steps listed with exact commands, never attempted
**Source:** RESEARCH "The owner's closing steps"; `release.yml` comments; `deploy/DEPLOY.md` "Making a release".
**Apply to:** the last plan and `docs/branches.md`.
```bash
git push origin main            # publishes the local commits and the merge
git push origin main:release    # fast-forward; release.yml mints v0.1.0
# or Actions > Release > Run workflow, no inputs (main must be pushed first)
```
Verify afterwards: `git ls-remote --tags origin v0.1.0`, `gh release view v0.1.0 --json isPrerelease,body`. Do not push tags; do not edit `.github/workflows/release.yml`.

### Gates held (QUAL-01, D-18)
**Apply to:** every plan's verify step; run first (baseline) and last.
```bash
go run ./tools/i18n && go run ./tools/themewords -check && go run ./tools/english
go test . ./internal/changelog/ ./internal/admin/
```
i18n prints one line per catalogue (`N translated, 0 open, 0 orphaned`). Add no operator string; do not reword the three alpha catalogue keys (leaves orphans in de/es/fr/it).

### Language and style
`docs/` and `deploy/` are English whole sentences; the CHANGELOG is German operator prose; no external subresources in docs beyond existing shields.io badges.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `docs/beta.md` (as a "contract") | doc | promise/non-promise | No existing page states promises; the hand-over in `deploy/test/README.md` is the nearest, and it is a restriction list, not a model page. Structure comes from D-03 to D-07 |
| `docs/branches.md` | doc | generated audit | No prior branch audit in `docs/`; use the git/gh script above |

## Metadata

**Analog search scope:** `docs/`, `deploy/`, `deploy/test/README.md`, `CHANGELOG.md`, `README.md` (origin/main), `.github/workflows/release.yml` (via RESEARCH).
**Files scanned:** about 10 read or listed; RESEARCH already carried the verified details.
**Pattern extraction date:** 2026-10-14
