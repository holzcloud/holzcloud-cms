---
phase: quick-260928-n6m
plan: 01
subsystem: public-theme-holzcloud
tags: [css, grid, holzcloud-theme, auftakt]
status: complete
requires: [quick-260928-l3g]
provides: ["Karten beside the phone in the Auftakt from 60em"]
affects: [cmd/holzcloud/templates/public/holzcloud/style.css, CHANGELOG.md]
tech-stack:
  added: []
  patterns: ["explicit shared grid row (grid-row: 3) guarded by :first-child", "shared custom properties resolved at the point of use"]
key-files:
  created: []
  modified:
    - cmd/holzcloud/templates/public/holzcloud/style.css
    - CHANGELOG.md
decisions:
  - "Phone and Karten share grid row 3 from 60em. This works only when .hc-eigen--vorspann is :first-child; otherwise the page stays stacked."
  - "--hc-handy-breite and --hc-handy-einzug are declared on both phone and Karten, so the Karten's margin-inline-end reserves exactly the phone's width plus its inset plus 24px."
  - "The breakpoint stays at 60em: the 960x800 probe shows 3 card columns."
metrics:
  duration: 8min
  completed: 2026-09-28
  tasks: 2
  files: 2
actuals:
  tokens: 1400
  tasks: 2
  commits: 2
plan_head_before: 24ee695e6e6e523a846d03c6fd7cf6fbb342490c
plan_head_after: 15b09103f76d3e10ea3088e037c7d255be9e5484
---

# Quick 260928-n6m Plan 01: Karten neben dem Telefon im Auftakt Summary

From 60em up, the phone and the strengths cards in the holzcloud theme share one explicit grid row directly under the hero window. The cards reserve the phone's width, inset and a 24px gap through shared custom properties. At 1440x900 they now start at y 857 on /holzkube-manager and y 824 on /hauscloud; before, they started at y 1220 and y 1187.

## What changed

- **style.css, CMS layer only:**
  - A new selector list declares `--hc-handy-breite` and the new `--hc-handy-einzug` for both the phone and the following `.hc-karten`.
  - The phone's `margin-inline` now uses `var(--hc-handy-einzug)`, the same expression as before.
  - The phone comment is updated: from 60em the cards stand beside the phone, below that they stand under it.
  - A new `@media (min-width: 60em)` block sets `grid-row: 3; align-self: start` on both elements, guarded by `.hc-eigen--vorspann:first-child`. It gives the phone `margin-block-end: var(--hc-space-7)` and the cards `margin-inline-end: calc(var(--hc-handy-breite) + var(--hc-handy-einzug) + var(--hc-space-5))`.
  - German comments explain why.
- **CHANGELOG.md:** new `## 0.0.7 — 2026-09-28` section above 0.0.6.

## RED / GREEN

- **RED** (unmodified build at 24ee695, SCRATCH/red-n6m.log): 22 FAILs, all on the phone pages at wide viewports:
  - `cards.top` ≠ hero.bottom + 32 + 24: 10 failures
  - `cards.top` ≥ 900 at 1440x900: 2 failures
  - cards end right of the phone's left edge, with gaps from −336px down to −221px: 10 failures

  /holzcloud-cms, 390x844, the hero/phone-rect checks, the hit tests and horizontal scroll all passed.
- **GREEN** (SCRATCH/green-n6m.log): `ALL OK` at all six viewports on the first build after the CSS change. The CSS values needed no adjustment.

Measured in green-n6m.log:

| Page | Viewport | cards.top | hero.bottom | gap to phone | cols | next.top − phone.bottom | next.top − cards.bottom |
|---|---|---|---|---|---|---|---|
| holzkube-manager | 1920x1080 | 948.0 | 892.0 | 24.0 | 4 | 72.0 | 261.5 |
| hauscloud | 1920x1080 | 915.5 | 859.5 | 24.0 | 4 | 72.0 | 261.5 |
| holzkube-manager | 1440x900 | 856.9 | 800.9 | 24.0 | 4 | 72.0 | 280.3 |
| hauscloud | 1440x900 | 824.4 | 768.4 | 24.0 | 4 | 72.0 | 280.3 |
| holzkube-manager | 1280x800 | 801.0 | 745.0 | 24.0 | 4 | 72.0 | 219.2 |
| hauscloud | 1280x800 | 768.5 | 712.5 | 24.0 | 4 | 72.0 | 214.3 |
| holzkube-manager | 1024x768 | 775.7 | 719.7 | 24.0 | 4 | 72.0 | 73.7 |
| hauscloud | 1024x768 | 743.2 | 687.2 | 24.0 | 4 | 72.0 | 73.7 |
| holzkube-manager | 960x800 | 828.3 | 772.3 | 24.0 | 3 | 200.3 | 72.0 |
| hauscloud | 960x800 | 795.8 | 739.8 | 24.0 | 3 | 205.2 | 72.0 |

- **Where the next block starts.** It starts 72px below whichever of the two ends lower: the phone's 48px bottom margin, or the cards' 48px bottom margin, plus the next block's 24px top margin.
- **Unchanged against the baseline.** The hero and phone rects match within 0.5px at every viewport. At 390x844, and on /holzcloud-cms at every viewport, every `.page-content` child rect matches within 0.5px.
- **No horizontal scroll** on any page at any viewport.

## Screenshots

Taken at scroll 0, SCRATCH = /tmp/claude-1000/-home-holz-Projects-holzcloud-ch-website-local/80edae85-ded8-4ea5-8431-fbb76e1f4c6b/scratchpad/theme-check:

- SCRATCH/final-n6m/holzkube-manager-1440x900.png (the main one)
- SCRATCH/final-n6m/holzkube-manager-auftakt-1440x900.png: full-page clip from the hero to the next block
- SCRATCH/final-n6m/holzkube-manager-1920x1080.png, -1280x800.png, -1024x768.png, -960x800.png, -390x844.png
- SCRATCH/final-n6m/hauscloud-1440x900.png, hauscloud-390x844.png
- SCRATCH/final-n6m/holzcloud-cms-1440x900.png

## Gate

- **Commands, all green:**
  - `gofmt -l .` printed nothing
  - `go vet ./...`
  - `go run ./tools/english`
  - `go run ./tools/i18n`: 0 open, 0 orphaned
  - `go run ./tools/themewords -check`
  - `go run ./tools/cites -check`: 0 citations, 0 unresolvable
  - `go run ./cmd/holzcloud template check`: no problems found
  - `go test ./internal/template/... ./internal/public/... ./cmd/...`
- **Full `go test ./...` skipped.** No Go code changed; internal/admin alone takes about 25 minutes on this Pi. For a CSS-only change to an embedded theme, the right scope is the template, public and cmd tests: theme loading, rendering and the embedded binary.
- **Byte-identical to the base:** the verbatim holzcloud-design region of style.css.
- **Only files changed:** style.css and CHANGELOG.md.
- **Cleanup:**
  - The scratch server is stopped, and port 18766 is free.
  - holzcloud-sites is unchanged.
  - `git status --short` is clean.

## Deviations from Plan

**1. [Rule 1 - Bug, in the probe] Hit test used smooth scrolling**
- **Found during:** Task 1, first RED run.
- **Issue:** The theme sets smooth scrolling, so `scrollIntoView` had not finished when `elementFromPoint` ran. All hit tests failed, even with the cards below the phone on the unmodified build.
- **Fix:** Call `scrollIntoView({ behavior: 'instant' })`, and report the element that was hit on a miss.
- **Result:** The RED log was re-run and now fails only on the intended assertions.
- **Scope:** scratch harness only, no repo change.

Otherwise the plan was executed as written.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: cmd/holzcloud/templates/public/holzcloud/style.css contains `.hc-eigen--vorspann:first-child + .hc-bild + .hc-bild + .hc-karten`
- FOUND: CHANGELOG.md contains `## 0.0.7 — 2026-09-28`
- FOUND: commit 89a265e (feat, style.css) and commit 15b0910 (docs, CHANGELOG.md)
- FOUND: 10 PNGs in SCRATCH/final-n6m/
