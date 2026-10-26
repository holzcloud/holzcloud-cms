# Phase 24: The backup timer as shipped - Context

**Gathered:** 2026-10-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Verification phase, no feature. The shipped `deploy/holzcloud-backup.timer` and `.service`, the delay, the catch-up (`Persistent=`), the off-site copy (`REMOTE_TARGET`) and the pruning are each seen working exactly as shipped (no drop-in), using the phase 22 harness in `deploy/test/`. Requirements BKP-01..BKP-05.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion: pure infrastructure/verification phase. Constraints: no product feature is built; scripts or units are changed only when a measurement proves a defect, and then the defect is named in the record. Results are written as records (like `deploy/BACKUP-RUN.md`); a line in `docs/beta.md` moves to *promises* only in phase 28. What a container cannot show is handed over as a numbered owner step, not claimed.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `deploy/test/` (Dockerfile, run.sh, canary) from phase 22; `deploy/backup.sh`, `restore.sh`, `holzcloud-backup.{service,timer}`; `deploy/BACKUP-RUN.md` as record format.

### Established Patterns
- Measured records as markdown next to the scripts; `docs/beta.md` lists promises vs. non-promises.

### Integration Points
- `.planning/REQUIREMENTS.md` BKP-01..05; phase 25/26 reuse the same harness.

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
