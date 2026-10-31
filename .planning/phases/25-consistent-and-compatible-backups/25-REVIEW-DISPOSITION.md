---
phase: 25
review: 25-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: open
    title: "A leftover MANIFEST-less directory in an otherwise AGREE run is flagged BAD, but its evidence is thrown away"
  - id: WR-02
    severity: warning
    disposition: open
    title: "`consist_run` is called as an `if` condition, so `set -e` is off for its entire 250-line body"
  - id: WR-03
    severity: warning
    disposition: open
    title: "`cf_run_unit` and the inline unit run in `capfull_run` never check that a new invocation started"
  - id: WR-04
    severity: warning
    disposition: open
    title: "Mechanism readings and document lines come from the working tree, not from the pinned `NEW_REF`"
  - id: WR-05
    severity: warning
    disposition: open
    title: "The record and the README state bound-file facts that phase 29 made false, with no pointer to the successor record"
  - id: WR-06
    severity: warning
    disposition: open
    title: "Destructive `rm -rf` on the \"target\" container with no `NAME != TGT_NAME` guard in `consist`, `window` and `compat`"
  - id: IN-01
    severity: info
    disposition: open
    title: "The first `usage()` is dead code, and the live one breaks under a relative invocation"
  - id: IN-02
    severity: info
    disposition: open
    title: "The header comment still documents `CHURN_WORKERS` as 2"
  - id: IN-03
    severity: info
    disposition: open
    title: "Step (4) of `capfull` can no longer run, which lost its coverage of `restore.sh` on a partial directory"
  - id: IN-04
    severity: info
    disposition: open
    title: "The unit-start sequence exists twice"
  - id: IN-05
    severity: info
    disposition: open
    title: "`consist_finish` runs `rm -rf` on a path taken from the transcript, and early returns leave `.work-*` directories behind"
open: 11
total: 11
recorded: 2026-10-31T00:00:00.000Z
---

# Phase 25: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| WR-01 | warning | open | - |
| WR-02 | warning | open | - |
| WR-03 | warning | open | - |
| WR-04 | warning | open | - |
| WR-05 | warning | open | - |
| WR-06 | warning | open | - |
| IN-01 | info | open | - |
| IN-02 | info | open | - |
| IN-03 | info | open | - |
| IN-04 | info | open | - |
| IN-05 | info | open | - |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
