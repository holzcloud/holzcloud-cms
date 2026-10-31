---
phase: quick-260928-l3g
plan: 01
subsystem: public-theme-holzcloud
status: complete
tags: [css, theme, holzcloud, auftakt, bildtext, has-selector, grid]
requires: [quick-260928-klh]
provides:
  - "Phone frame for Vorspann + Bild + Bild in the holzcloud theme"
  - "Heading above image for heading-led bildtext blocks (holzcloud theme)"
affects:
  - cmd/holzcloud/templates/public/holzcloud/style.css
  - CHANGELOG.md
tech-stack:
  added: []
  patterns:
    - "Adjacent-sibling sequence selectors (.hc-eigen--vorspann + .hc-bild + .hc-bild) as layout triggers"
    - ":has()-guarded grid with display: contents on the text wrapper, trailing 1fr row absorbing image height"
key-files:
  created: []
  modified:
    - cmd/holzcloud/templates/public/holzcloud/style.css
    - CHANGELOG.md
decisions:
  - "Dropped the optional camera-island ::after: the holzcloud.ch phone screenshots have no status bar, so the pill sat over the app header (documented in the CSS comment)"
  - "Karten selectors extended as selector lists, not :is(), so the original selectors keep their exact specificity"
  - "Bildtext change stays CSS-only (save-time rendering + shared block-HTML contract); render.go, TEMPLATE-SPEC.md, bausteine.css and the other seven themes are untouched"
metrics:
  duration: "~47 min"
  completed: 2026-09-28
actuals:
  tokens: 2600
  tasks: 3
  commits: 3
plan_head_before: a6d9e4205af12fffbf9dddd257a047a0ff42e555
plan_head_after: 2216bbedb33286eb79c63df2f7c934425e815487
---

# Phase quick-260928-l3g Plan 01: Holzcloud theme — phone frame after hero, bildtext heading above image — Summary

The holzcloud theme now changes two layouts, in its CMS-layer CSS only. When a second image follows the hero window, it is drawn as a phone: a bezel built from theme tokens, lying over the lower right of the window. When a bildtext block's text starts with an h2 or h3, a `:has()`-guarded grid puts that heading across the full width above the image. Below it, image and text sit side by side, top-aligned, from 45em up and stack below that.

## Tasks

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Phone frame after the hero (harness, baseline, CSS, probe) | ec0881f | style.css |
| 2 | Bildtext heading above the image (CSS-only, :has-guarded grid) | 18986b2 | style.css |
| 3 | CHANGELOG 0.0.6, full gate, clean up | 2216bbe | CHANGELOG.md |

## What was built

- **Phone** (`.page-content > .hc-eigen--vorspann + .hc-bild + .hc-bild`):
  - Sizing and position: `clamp(7rem, 21%, 18rem)` wide, `justify-self: end` with a 4% right inset, pulled up by `min(24vh, 16rem, 28%)`, `position: relative; z-index: 1`.
  - Frame: the bezel is `--hc-pane-sunken` over `--hc-ground`, so it follows `data-seite` and has no colour literal. It adds a `--hc-pane-edge` border, a brass glow in `color-mix(oklch)` and `--hc-shadow-lift`.
  - Screenshot: the img keeps its natural aspect, capped at `100cqi * 2.3` and cut only at the bottom (`object-position: top`).
  - Caption: the figcaption is hidden with the `.sr-only` clip pattern, not `display: none`.
  - Karten: all five Auftakt Karten rules, including the one in the 30em query, also match `… + .hc-bild + .hc-bild + .hc-karten`.
- **Bildtext** (guard `.hc-bildtext:has(> .hc-bildtext__bild + .hc-bildtext__text > :is(h2, h3):first-child)`):
  - Text wrapper: `display: contents`, heading in `grid-row: 1`, `row-gap: 0`.
  - From 45em: named lines `bild` / `lauf`, rows `auto repeat(15, auto) 1fr`, `align-items: start`. The image spans `2 / -1` and the text sits in `lauf`, with the first paragraph's top margin set to 0.
  - `--rechts` mirrors the columns and keeps the image at three fifths.
- **CHANGELOG**: two paragraphs in 0.0.6, placed before the `thumbnails -force` instruction. Swiss spelling, „…“ quotes.

## Verification (all green)

- `probe.js hero`: passes at 1440x900 and 390x844, dark and light, on /holzkube-manager (brass) and /hauscloud (blue).
  - Phone/hero ratio is 0.210 at 1440. The phone is 112px wide at 390.
  - The phone top lies inside the window below the title bar, and its right edge is inset.
  - The image is not clipped; its aspect matches 2532/1170 within 2px.
  - A hit test shows the phone paints above the window.
  - The Karten start at least 8px below the phone, and their style matches the holzcloud-cms Karten.
  - Background and shadow are identical in dark and light.
  - /holzcloud-cms hero and Karten rects equal baseline.json within 0.5px.
- `probe.js bildtext`: passes for all 32 guarded blocks (/, /holzkube-manager, /holzcloud-cms, /hauscloud) at both viewports and both schemes. The heading-less guard block on home equals its baseline, measured relative to the block itself.
- Sanity: against the unmodified build the same probes fail (113 hero, 321 bildtext assertions), while the cms and guard regression checks pass. So the probes tell the two builds apart.
- `holzcloud template check`: no problems found. `go test ./internal/template/ ./internal/block/` pass.
- The verbatim region is byte-identical to a6d9e42. `git diff --quiet` from the base commit is clean over render.go, TEMPLATE-SPEC.md, bausteine.css and the seven other themes.
- Gate:
  - `gofmt -l .` prints nothing.
  - `go vet ./...` passes.
  - `tools/english` passes.
  - `tools/i18n` reports 0 open and 0 orphaned for every catalogue.
  - `themewords -check` passes.
  - `cites -check` reports 0 citations.
  - `go test -timeout 40m ./...` passes all 53 packages; internal/admin took 1442s.
- Clean-up: the scratch server is stopped and port 18766 is free. `git status --short` shows nothing, and holzcloud-sites is unchanged.

## Deviations from Plan

1. **[Rule 3 - Blocking] Harness env.** `HOLZCLOUD_LISTEN` takes an IP only, so the harness sets the port in `HOLZCLOUD_PORT=18766`. Node's `fetch` cannot resolve `*.localhost`, so the freshness guard uses `http.get` to 127.0.0.1 with a Host header. The website edit page is `/admin/websites/{id}`, not `/edit`. All three stay in the scratch harness only.
2. **[Rule 1 - Probe] Natural-size check → aspect check.** After the variants were regenerated, the phone img serves a srcset. Its natural size is then the chosen candidate (1120x2423 or 390x843), not 1170x2532. The probe now checks the natural aspect ratio instead, plus the rendered height = width × 2532/1170 within 2px.
3. **Harness hero images.** The real `holzkube-aufmacher.webp` and `hauscloud-aufmacher.webp` are laptop-plus-phone composites. To judge the composition, the scratch data dir got the desktop-only screenshots (`phones/*-desktop.png` → webp) under the same file names, and `holzcloud thumbnails -force` was run. This happened in the scratch data dir only; holzcloud-sites is untouched. The holzcloud-cms page and its hero were left alone, so the regression baseline still holds.
4. **Guard bildtext baseline measured relative to the block.** The heading-led bildtext blocks above it on home legitimately change height, so absolute page positions would shift. The check compares block size and left edge, plus child rects relative to the block.
5. **Camera island dropped.** The plan made it optional. It covered app-header content in these screenshots, which have no status bar.
6. **Scratch import quirk.** Pages in the manifest carry no `locale`, so the import de-duplicates slugs (`holzkube-manager`, `-2` … `-5`). The probes use the base slugs. The final screenshots use `/holzkube-manager-5`, the German page.

## Final screenshots

- /tmp/claude-1000/-home-holz-Projects-holzcloud-ch-website-local/80edae85-ded8-4ea5-8431-fbb76e1f4c6b/scratchpad/theme-check/final/holzkube-manager-hero-phone-1440x900.png
- /tmp/claude-1000/-home-holz-Projects-holzcloud-ch-website-local/80edae85-ded8-4ea5-8431-fbb76e1f4c6b/scratchpad/theme-check/final/holzkube-manager-hero-phone-390x844.png
- /tmp/claude-1000/-home-holz-Projects-holzcloud-ch-website-local/80edae85-ded8-4ea5-8431-fbb76e1f4c6b/scratchpad/theme-check/final/holzkube-manager-bildtext-1440x900.png
- /tmp/claude-1000/-home-holz-Projects-holzcloud-ch-website-local/80edae85-ded8-4ea5-8431-fbb76e1f4c6b/scratchpad/theme-check/final/holzkube-manager-bildtext-390x844.png

## Notes for the operator

- On holzcloud.ch the current hero images already show a phone. With a second (phone) image added, the page would show two phones. Replacing the hero with a desktop-only screenshot, as the harness did, gives the intended pair.
- At 1440x900 the phone reaches about 380px below the window, which pushes the Karten below the first screen. This follows from the planned 18–28% width and the uncropped screenshot.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: cmd/holzcloud/templates/public/holzcloud/style.css (modified)
- FOUND: CHANGELOG.md (modified)
- FOUND commits: ec0881f, 18986b2, 2216bbe
- FOUND: 4 final screenshots
