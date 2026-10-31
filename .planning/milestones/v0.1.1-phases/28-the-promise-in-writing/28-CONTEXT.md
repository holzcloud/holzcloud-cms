# Phase 28: The promise in writing - Context

**Gathered:** 2026-10-26
**Status:** Ready for planning
**Source:** The orchestrator's brief for this phase (no discuss-phase was held), the ROADMAP entry of phase 28, REQUIREMENTS DOC-01 and DOC-02, and the records of phases 24 to 27, read in full except their appendices.

<domain>
## Phase Boundary

Writing phase. Requirements DOC-01 and DOC-02. It changes three files and adds one: `docs/beta.md` (0.1.0 promised ten things and listed 31 limits; this phase moves exactly those limits that a record now answers, and keeps the others with their reason), `CHANGELOG.md` (a 0.1.1 entry in the shape `.github/workflows/release.yml` extracts), a new `.planning/phases/28-the-promise-in-writing/28-OWNER-STEPS.md` (every step of this milestone that needs a real server or the GitHub UI), and the checker `.planning/phases/28-the-promise-in-writing/28-check.sh`, which the planner wrote and committed together with the plans (the phase 27 precedent: executors use the checker, they do not loosen it).

The phase proves nothing new. It moves a line to *promises* only where a sentence of a record proves it and the checker proves that the sentence is quoted byte for byte. SRV-01, SRV-02 and SRV-03 are **pending owner** (`Status: pending owner` in `deploy/NATIVE-AMD64-RUN.md`, `bash deploy/test/native-amd64.sh status-check` agrees): they stay under *does not promise*, each with its reason and its owner step.

What this phase does not do: set a version number, a tag or a release (the milestone completion step owns the number, the date and the tag; a published tag is never moved), push anything, edit `deploy/backup.sh`, `deploy/restore.sh`, the units, `deploy/test/run.sh`, `deploy/test/nightly.sh`, `deploy/DEPLOY.md`, `.github/workflows/release.yml` or any record under `deploy/`, or change the program.

The facts the plans rest on were measured on 2026-10-26 at HEAD 85fa486 and are in the plans' tasks; the ones that decide design are in the next section.
</domain>

<decisions>
## Implementation Decisions

### Locked (from the orchestrator's brief for this phase)

- **D-01 (the promise rule):** A line moves to *promises* only if a sentence of a record proves it. A moved row cites one record, quotes at least one whole sentence of it between straight double quotes, and may add backticked tokens. The checker (`28-check.sh beta quotes`) finds every quote and every token in the record with `grep -F` and requires each quoted sentence to end with a full stop and to begin and end where a sentence of the record begins and ends. The method of 0.1.0 (`grep -F` on the cited words) is kept; 0.1.0 ran it once, at plan time, with no tool. This phase leaves the tool behind.
- **D-02 (SRV stays a limit):** SRV-01, SRV-02 and SRV-03 do not move. Each stays under *does not promise* with its reason, the sentence `Pending owner: SRV-0n`, `Record: deploy/NATIVE-AMD64-RUN.md` and the number of the owner step that answers it (OS-5, OS-13, OS-14). The checker (`beta srv`) refuses a promise row that cites the native record or names an SRV requirement while any Status there reads `pending owner`, and it fails on purpose when a Status reads `recorded`, because then the page is stale and the line moves by the hand-over rule of that record.
- **D-03 (the 25 findings, honestly):** The page says what the beta does and does not promise about them. Promised: the shipped timer, the delay, the catch-up, the retention rule, the off-site copy and the restore from it; the window in which database and media disagree is named and was reproduced in both directions; a backup of 0.1.0 restores with this release; a full disk produces a failed unit, a partial directory that `restore.sh` refuses, and no false completion. Not promised, each as its own limit with `Why:`, `Record:` and a quoted sentence: the pair is not atomic (files without a row in 41 of 80 runs, rows without a file in 11 of 40); a restore does not report a row whose file the backup lacks (the only symptom is a 404); a file that vanishes during the media copy ends the night with `rsync` status 24 and no `MANIFEST`; a failed night leaves its directory and the guard then refuses the following nights until it is removed by hand; the retention step is skipped on such nights; a key in the service user's home does not work; the off-site copy is a mirror; a failed copy leaves the new backup on the box; a failed removal in the retention step is silent; the sixth restore within five minutes meets the start limit.
- **D-04 (frozen files; DEPLOY.md is not edited):** `deploy/DEPLOY.md` and `docs/beta.md` are bound files of earlier evidence (docs/beta.md baseline f1ba5a2; the six files bound by b501b1b and e86d490). `deploy/DEPLOY.md` is not edited: the two sentences it lacks (a vanished file ends the night; a failed night's directory blocks the following nights) and the third gap found in phase 25 (a restore does not report a row whose file is missing) are written in `docs/beta.md` and in the owner steps instead. An edit of DEPLOY.md would stale the evidence bound by e86d490 and force a second evidence set; whether to do that is the owner's decision (see Deferred).
- **D-05 (the shape of the page):** `docs/beta.md` keeps its 0.1.0 structure and its rows: P-1 to P-10 and E-1 to E-3 stay byte-identical; the four groups of *does not promise* stay in their order; a section *What changed since 0.1.0* is added between *does not promise* and *What the evidence is bound to*. Every one of the 31 limits of the baseline is either **kept** (the baseline bullet verbatim, followed by ` Why: ...` and, where a record or an owner step bears on it, `Record:` and `Owner step OS-n`) or **moved** (the baseline bullet verbatim as the first cell of a ledger row, with what 0.1.1 says in the second cell and the promise rows that answer it in the third, or `none (reworded)`). The checker (`beta baseline`) refuses a limit that is neither, and a limit that is both.
- **D-06 (the version):** The page is `0.1.1`. The title line carries it; the headings of the promises and does-not-promise sections carry it; `28-check.sh changelog extract` requires the first heading of `CHANGELOG.md` and release.yml's own extraction to read the same number. The number and the date in the heading (`## 0.1.1 — 2026-10-26`) are the draft of the milestone completion step; owner step OS-15 is where the owner confirms or changes the date.
- **D-07 (evidence):** The page gains E-4 (`deploy/BACKUP-TIMER-RUN.md`, evidence commit b501b1b) and E-5 (`deploy/BACKUP-CONSISTENCY-RUN.md`, evidence commit e86d490). The rule paragraph and its command are rewritten, because the 0.1.0 command, which includes `deploy/DEPLOY.md`, no longer exits 0: DEPLOY.md has 39 added lines against bf74496 (the credentials paragraph of phase 24, F-2). The page states that difference ("39 added lines"), shows five git commands that exit 0 (the three bound-file commands name HEAD; the two program-identity commands are pinned to 85fa486), and says that `cmd/holzcloud/assets/bausteine.css` and the weide `style.css` differ by PR 47 after the evidence builds. The checker runs the commands it requires the page to show.
- **D-08 (OWNER-STEPS):** One file, `28-OWNER-STEPS.md`, an index table and `### OS-n: title` steps. Every step has `Source:` (a record and its step, or `new`), `Where:` (development machine, GitHub UI, real server), `Needs:` (earlier steps or `nothing`), at least one fenced command block, `Expect begins at: "phrase"` and `A different output is explained at: "phrase"` (both phrases found in the step). A step taken from a record carries the record's own text in one region between `<!-- verbatim: SOURCE -->` and `<!-- end verbatim -->`, copied by `28-check.sh extract` and compared with `cmp` by `28-check.sh owner-steps steps`. A `new` step carries a `Rehearsal:` line saying what was played and where. Outputs are never invented: they are the record's, or were seen in a rehearsal in a throwaway repository, or were read from GitHub with a read-only `gh` call, and the step says which.
- **D-09 (the steps the records do not hold):** The push of `main` (local `main` is ahead of `origin/main` and also behind it: three commits of PR 47 are only on GitHub), triggering the release of 0.1.1 through `release.yml`, and keeping the bundle file of phase 27 are owner steps of their own (OS-1 to OS-4, OS-15 to OS-19). The ruleset on `main` (no deletion, no force push, a pull request with the green check `Build and test`, bypass for the Admin role) was read on 2026-10-26 and decides how the push is explained.
- **D-10 (CHANGELOG):** The entry is `## 0.1.1 — 2026-10-26` directly above `## 0.1.0 — 2026-10-14`, in German whole sentences with bold leads, absolute GitHub links to `docs/beta.md` and `deploy/DEPLOY.md`, and no `## ` line inside it. It says what 0.1.1 proves and where the proof is, what it does not, what the owner still has to do, and what changed in the program since 0.1.0 (two small features from quick task 261026-bon and PR 47). `28-check.sh changelog extract` runs release.yml's own version line and notes step on the file.
- **D-11 (the checker is part of the plans):** `28-check.sh` is committed with the plans. Its selftest (58 cases, each negative case asserting the reason it fails) must stay green. An executor who finds a rule wrong reports the deviation in the SUMMARY and changes the rule in a commit of its own; an executor does not loosen a rule to make a page pass.
- **D-12 (nothing earlier may break):** Phase 24, 25 and 26 checkers keep passing: `nightly.sh selftest` (542 cases, read through `2>&1`), `record-check` (29 blocks), `E=<repo>/deploy/test/out record-check-con` (27 blocks), `native-amd64.sh selftest` (862 cases), `status-check`, `record-check` (0 blocks, pending SRV-01 to SRV-03). `28-check.sh frozen` shows that no bound file, no record and no program file changed.

### Claude's Discretion

- **D-13 (the line "none is planned (a user decision)"):** The 0.1.0 limit "No native amd64 run was made and none is planned (a user decision); ..." is half false since phase 26: the milestone reversed the decision and a button exists. `deploy/NATIVE-AMD64-RUN.md` tells phase 28 to keep the line while SRV-01 is pending "because it is true until then". It is true that no run was made; it is not true that none is planned. This plan moves the baseline line to the ledger with `none (reworded)` and replaces it by a limit that says no native amd64 run was made *yet*, that one is planned as a button, and that the result is pending the owner (SRV-01, OS-5). The owner can overrule this by keeping the baseline bullet and adding a `Why:` that corrects it; the checker accepts both.
- **D-14 (wording):** The wording of the promise column, of the `Why:` tails, of the residual limits, of the new owner steps and of the CHANGELOG paragraphs is the executor's, inside the rules above. Prose is plain English on the page and in the owner steps (as in 0.1.0), German in the CHANGELOG (as in 0.1.0).
- **D-15 (tracer):** The phase leads with one line moved end to end (BKP-01, the 03:15 line): a title and heading change, one promise row, one ledger row, one residual limit, one owner step and one CHANGELOG heading with a first paragraph, all checked by the slices of `28-check.sh` that are true at that point. The other seven promise rows, the other 22 limits, the other 18 steps and the rest of the entry expand from it.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- `docs/beta.md` at f1ba5a2 (the baseline, `git show f1ba5a2:docs/beta.md`): 13 rows (P-1 to P-10, E-1 to E-3), 31 limits in four groups, the rule paragraph and its command, the alpha section.
- `deploy/BACKUP-TIMER-RUN.md` (evidence b501b1b): BKP-01 to BKP-05, findings F-1 to F-8, Not tested, owner steps O-1 to O-5. `deploy/BACKUP-CONSISTENCY-RUN.md` (evidence e86d490): CON-01 to CON-03, findings F-1 to F-7, Not tested, two owner steps. `deploy/NATIVE-AMD64-RUN.md`: SRV-01 to SRV-03 pending owner, owner steps O-1 to O-3.
- `.planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md` (steps 0 to 4: heading date, publish, release, check, afterwards) and `.planning/phases/27-repository-tidy/27-OWNER-STEPS.md` (the bundle: E-2 and E-5).
- `.github/workflows/release.yml`: the version is `grep -m1 -oE '^## [0-9]+\.[0-9]+(\.[0-9]+)?' CHANGELOG.md | sed 's/^## //'`; the notes are the awk of the step "Write the release notes from the CHANGELOG" (`/^## / { if (found) exit; if ($2 == v) { found = 1; next } } found { print }`); routes: push to the `release` branch (route 2), the Actions button (route 3); every `v0.*` is a pre-release.
- `.planning/phases/27-repository-tidy/27-check.sh`: the style of a phase checker (bash, `set -u`, fail/ok, subcommands, `all`).

### Established Patterns

- Records quote themselves: a record's `Hand-over to phase 28` names the baseline lines it bears on, as they stand, and the rule for moving them.
- `docs/beta.md` rows name one record and cite words that `grep -F` finds. The rule command of the evidence section is run, not read.
- The checker's environment: macOS, BWK awk 20200816 (no gensub, no interval expressions), bash 3.2-safe, `jq`, `gh` (authenticated, read-only calls only), `go`. The Bash tool is zsh: run `$VAR` word-splitting under `bash`.

### Integration Points

- `internal/changelog` and the root `changelog.go` embed and parse `CHANGELOG.md`: `go test ./internal/changelog/ -count=1`, `go test . -run Changelog -count=1` and `go test ./internal/admin -run Changelog -count=1` read the new entry.
- The program is identical to the evidence builds b501b1b and e86d490 outside `.planning`, `docs`, `deploy`, `.github` at 85fa486; PR 47 (commits 78efbce, fa696f6, merge ee3c9ed) changes `cmd/holzcloud/assets/bausteine.css` and `cmd/holzcloud/templates/public/weide/style.css` on `origin/main`.
</code_context>

<specifics>
## Specific Ideas

- Remote facts read on 2026-10-26 (all read-only): `git ls-remote --heads origin` gives `main` ee3c9ed, `release` 4d238b9 (= tag v0.1.0), `fix/bausteine-befunde` fa696f6. Local `main` is 67 commits ahead and 3 behind `origin/main`, so a plain `git push origin main` is refused as non-fast-forward until the three are merged; `git merge-tree --write-tree main origin/main` was clean. The ruleset "main: kein Force-Push, kein Loeschen, PR mit gruener Pruefung" (id 22933928) holds the rule types `deletion`, `non_fast_forward`, `pull_request` (0 approvals) and `required_status_checks` (`Build and test`); its only bypass actor is the repository role Admin (id 5), mode `always`; the account `gh` is signed in with has `permissions.admin: true`. The workflow `native-amd64` is not yet listed on GitHub (`gh workflow list`), so the button of SRV-01 exists only after `main` is pushed. CI takes about 13.5 minutes (`gh run list --workflow CI`).
- The bundle of phase 27 is at `/Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle`, 26349797 bytes, sha256 `bdcd2fd9ff164b1d1cd59e6ed68a36f7509e5dba00155ea07ead512cfa5b2924`, `git bundle verify` prints `is okay` and the ref `ef4873ad07e30ac27b02e5829ff0f1a20307ca9f refs/heads/backup/lokal-vor-origin-abgleich`.
- Product changes since v0.1.0 that the entry must state: quick task 261026-bon (the Bild-und-Text block gained "Bildlage", commit 8846285; uploads of rasters over 2560 px ask "Passend verkleinern auf höchstens %d px?", commit ec99f50), PR 47 (four layout findings in the blocks' core: small images are not scaled up, the call-out centres a single sentence, `--breit` can be adapted to the grid by property, and the image text aligns at the top), module bumps (goose v3.29.0, x/image v0.47.0, x/crypto, x/net), no migration (the highest is still 60). `backup.sh`, `restore.sh` and both units are byte-identical to v0.1.0.
- REQUIREMENTS lists "New CMS functions" as out of scope for this milestone; the two features predate the evidence builds and are in the program the records measured. The entry states them as what they are and the handback flags it for the owner.
</specifics>

<deferred>
## Deferred Ideas

- Editing `deploy/DEPLOY.md` to document the three gaps. It would stale the evidence bound by b501b1b and e86d490 (the six bound files are compared with `git diff --quiet` against those commits) and the evidence sets would be made again. The owner decides; if yes, it is a plan of its own that names what it re-binds.
- Fixing what phase 25 found: whether `rsync` status 24 should end the night, whether a failed night should remove its own directory, whether retention should run before the guard and the copy (the record leaves all three to the owner). FUT-02 (an atomic snapshot) stays future: every disagreement was named.
- Recording SRV-01, SRV-02 and SRV-03: the owner's results go in by the rule of `deploy/NATIVE-AMD64-RUN.md` ("Recording a result"); the page then moves the matching limits in a later release.
- The milestone completion: the release number, the date, the tag, the push and the Release run. Owner steps OS-15 to OS-18 are the text for it; this phase executes none of them.
- Reaching Ubuntu 24.04 or systemd 255, a real certificate, a browser restore, bit rot: unchanged limits, outside this milestone.
</deferred>

---

*Phase: 28-the-promise-in-writing*
*Context gathered: 2026-10-26 from the orchestrator's brief and the records of phases 24 to 27*
