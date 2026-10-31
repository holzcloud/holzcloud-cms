# Phase 22: Backup and restore, measured - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-12
**Phase:** 22-Backup and restore, measured
**Areas discussed:** none interactively — all delegated to Claude

---

## Gray area selection

Four areas were offered (empty system and measurement; proof of integrity and content; observing the timer run; changes to backup.sh/restore.sh). The first attempt came back with no selection; on the retry the user answered "Entscheide du alles".

| Option | Description | Selected |
|--------|-------------|----------|
| Leeres System und Messung | Fresh container vs wiped data dir; source site; seed size vs inflated media | delegated |
| Beweis von Integrität und Inhalt | Row counts, media hashes, login, `check`; MANIFEST checksums or not | delegated |
| Timer-Lauf beobachten | Near-term OnCalendar drop-in vs clock change | delegated |
| Änderungen an backup.sh/restore.sh | How far scripts may change if the run shows defects | delegated |

**User's choice:** "Entscheide du alles"
**Notes:** Same delegation as Phase 21. Claude decided each area; see CONTEXT.md D-01..D-18.

## Claude's Discretion

All four areas. Alternatives considered and not taken: restoring in place after wiping the data directory (weaker: leaves state behind); changing the container clock for the timer (moves journal, retention and `Persistent=true` bookkeeping); adding `--yes` to `restore.sh` (new surface); adding checksums to MANIFEST up front (feature without a demonstrated defect); measuring only the tiny seed site (dominated by service start).

## Deferred Ideas

- Tested off-site copy via `REMOTE_TARGET`
- Atomic DB + media snapshot
- Non-interactive restore / binary restore subcommand
- Real-server test of the 03:15 window, `Persistent=true`, `RandomizedDelaySec`
