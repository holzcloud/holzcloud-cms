# Phase 27: Repository tidy - Context

**Gathered:** 2026-10-20
**Status:** Ready for planning

<domain>
## Phase Boundary

No stray branch, one current position in STATE.md, v0.1.0 archived. Requirements TIDY-01 to TIDY-04. Nothing in the program changes; this phase touches `.planning/`, `docs/branches.md` and the remote's branch list only.

</domain>

<decisions>
## Implementation Decisions

### Remote branches (TIDY-01)
- **D-01:** Deletion is tried from here with the owner's own `gh`/`git`, first on **one** Safe-to-delete branch (`holzbar-colours`). The 403 in `docs/branches.md` belongs to the development proxy, not to this Mac. If that deletion works, the other six Safe-to-delete branches follow; if it answers 403, the phase writes the GitHub-UI click path as an owner step and deletes nothing. — **Reversibility:** costly — a deleted branch loses its tip unless the SHA is noted; each deletion is therefore recorded with its tip SHA in `docs/branches.md` before it happens.
- **D-02:** `backfill-releases` and `tagfix` (one commit each, push-triggered workflow with write access, not in main) are **deleted**. Both were one-off repairs that are done. The two commits' subjects and tip SHAs are written into `docs/branches.md` first so a restore is possible. — **Reversibility:** costly — same reason as D-01.
- **D-03:** `release` stays (the button of route 2 in `release.yml`). `claude/*` branches are judged by the table's rule like any other; the table is regenerated against the current `origin/main` before anything is deleted, because it was made against `9cc7bcb7bd` and main has moved.
- **D-04:** `fix/bausteine-befunde` appeared on origin on 2026-10-20 and belongs to another running session. It is **not** deleted by this phase and is not part of the table's verdicts; it is listed as "in use by a session" until that session is finished.

### STATE.md (TIDY-02)
- **D-05:** The history blocks (`Current Position (v1.10 …)`, `The milestone before this one (v2.1 / v2.0)`, the old Milestone Map, Performance Metrics and Session Continuity of closed milestones) move **verbatim** to `.planning/milestones/STATE-HISTORY.md`. STATE.md keeps one `Current Position`, Decisions, Operator Next Steps and Deferred Items. Nothing is rewritten, only moved.
- **D-06:** Done when `state.advance-plan` stops refusing with `ambiguous_position_phase` and the frontmatter progress counters read the real totals (5 phases for v0.1.1). That command is the acceptance test.

### Archive of v0.1.0 (TIDY-03)
- **D-07:** The archive is complete like v2.0 to v2.5: `milestones/v0.1.0-ROADMAP.md` (the phase 20 to 23 detail taken out of ROADMAP.md), `milestones/v0.1.0-REQUIREMENTS.md` (already written 2026-10-20), and a **retrospective** `milestones/v0.1.0-MILESTONE-AUDIT.md` written from the existing records (four VERIFICATION files, 23-UAT, the release page). It says plainly that it was written after the release. ROADMAP.md keeps one line for v0.1.0.

### Local branches (TIDY-04)
- **D-08:** `backup/lokal-vor-origin-abgleich` (the old local history, 489 commits, unrelated to origin/main) is deleted at the **end** of the phase, after the owner has confirmed that nothing is missing. It is local only; deleting it is the one irreversible step and is last.

### Claude's Discretion
- Order of work inside the phase, the exact layout of `STATE-HISTORY.md`, the wording of the retrospective audit.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Branches
- `docs/branches.md` - the table, the verdict rule, the GitHub-UI steps, the restore routes. Regenerate before use (commands are in the file).
- `.github/workflows/release.yml` - why `release` stays.

### Planning state
- `.planning/STATE.md` - the 644-line file to collapse.
- `.planning/ROADMAP.md` - phase 20 to 23 detail to archive; v2.5's one-line entry is the model.
- `.planning/milestones/v2.5-ROADMAP.md`, `v2.5-REQUIREMENTS.md`, `v2.5-MILESTONE-AUDIT.md` - the shape of a finished archive.
- `.planning/milestones/v0.1.0-REQUIREMENTS.md` - already archived.
- `.planning/milestones/v0.1.0-phases/` - the four VERIFICATION files and 23-UAT the retrospective audit rests on.

### Requirements
- `.planning/REQUIREMENTS.md` - TIDY-01 to TIDY-04.

</canonical_refs>

<code_context>
## Existing Code Insights

No program code is involved.

### Established Patterns
- Evidence before action: `docs/branches.md` says its verdicts rest on evidence in `main` and nothing else; a deletion without that evidence regenerated is not allowed.
- A published tag is never moved; branches carry no tag and are free to delete.

### Integration Points
- `gsd-tools query state.advance-plan` is the acceptance test for D-06.
- `gsd-tools query audit-open` should report all clear after the archive.

</code_context>

<specifics>
## Specific Ideas

- Try one deletion first, then the rest (D-01).
- The retrospective audit must say it was written after the release.

</specifics>

<deferred>
## Deferred Ideas

- Deleting `fix/bausteine-befunde` once its session is finished.
- Merging the old local lineage: never (see memory: parallel lineage, port single features).

</deferred>

---

*Phase: 27-repository-tidy*
*Context gathered: 2026-10-20*
