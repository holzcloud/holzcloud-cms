# Phase 25: Consistent and compatible backups - Context

**Gathered:** 2026-10-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Verification phase, no feature. On the phase 22/24 harness (`deploy/test/`, `nightly.sh`): (CON-01) a backup taken while pages are saved and media is uploaded restores with database and media in agreement, or the disagreement is reproduced and its window named; (CON-02) a backup made by 0.1.0 (tag `v0.1.0`) restores with this release: same pages, same login, migration level as the record states; (CON-03) a disk nearly full but above `MIN_FREE_MB` is provoked and the outcome recorded, not guessed. Requirements CON-01..03. FUT-02 (atomic snapshot) becomes a requirement only if CON-01 shows the window cannot be named.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All choices at Claude's discretion: pure verification phase. Constraints carried from phase 24: shipped files (backup.sh, restore.sh, units, DEPLOY.md, run.sh) change only on a measurement-proven defect, in a separate `fix(25-..)` commit, never loosening a sandbox directive (if that is the only fix: stop and hand the decision to the owner); results recorded with raw output in a record file in the style of `deploy/BACKUP-TIMER-RUN.md` (with nightly.sh `record-check` style byte-for-byte proofs where practical); container limits named, not claimed; `docs/beta.md` is not edited here (phase 28). The 0.1.0 binary is built from tag `v0.1.0`.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `deploy/test/nightly.sh` (selftest, fire, delay, catchup, prune, offsite, offsite-restore, chain, record-check), `deploy/test/run.sh`, `deploy/backup.sh`, `deploy/restore.sh`, `deploy/BACKUP-TIMER-RUN.md`, `deploy/BACKUP-RUN.md`.

### Established Patterns
- Evidence in gitignored `deploy/test/out/`, record pastes it as numbered blocks, a claim without recorded output is not allowed.

### Integration Points
- Phase 24 summaries and findings (F-1..F-8), phase 26 reuses the harness natively.

</code_context>

<specifics>
## Specific Ideas

No specific requirements.

</specifics>

<deferred>
## Deferred Ideas

None.

</deferred>
