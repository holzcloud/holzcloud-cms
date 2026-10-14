# What 0.1.0 promises, and what it does not

## What beta means here

Beta here means safe to run for real with backups, and it is not feature-frozen.

## What 0.1.0 promises

Every row below was shown in a Debian 12 container with systemd as PID 1, amd64 under emulation on an arm64 host; no row is about a real server.

| ID | Promise | Record | Cited words |
|----|---------|--------|-------------|
| P-1 | The hardened unit starts under real systemd and the site answers. | `deploy/SECURITY-SCORE.md` | `UNIT-01: the unit starts and the site answers`, `The unchanged unit started` |

## What 0.1.0 does not promise

- The real 03:15 window of the nightly backup was not exercised.

## What the evidence is bound to

| ID | Evidence | Record | Cited words |
|----|----------|--------|-------------|
| E-1 | P-1 is bound to the repository state the unit runs were made from. | `deploy/SECURITY-SCORE.md` | `2026-10-12, f1428d2` |

## Where the word alpha still appears

The admin screen still labels every 0.x build alpha.
