---
phase: quick-261026-bon
plan: 01
subsystem: blocks, media
tags: [blocks, bildtext, media, upload, staging, css, i18n]
requires: []
provides:
  - "Image position (left, right, above, below) on the Image-and-text block"
  - "Scale-down question after an upload whose longest side is over 2560 px"
affects: [internal/block, internal/media, internal/admin, public themes]
key-files:
  created:
    - internal/block/imageposition_test.go
    - internal/bundle/blocks_position_test.go
    - internal/media/scale.go
    - internal/media/scale_test.go
    - internal/media/staging.go
    - internal/media/staging_test.go
    - internal/admin/media_scale.go
    - internal/admin/media_scale_test.go
    - cmd/holzcloud/templates/admin/media_scale.html
  modified:
    - internal/block/block.go
    - internal/block/render.go
    - internal/admin/media.go
    - internal/admin/handler.go
    - internal/web/render.go
    - cmd/holzcloud/main.go
    - cmd/holzcloud/assets/bausteine.css
    - cmd/holzcloud/assets/admin.css
    - cmd/holzcloud/templates/public/holzcloud/style.css
    - cmd/holzcloud/templates/public/holzcloud/layout.html
    - cmd/holzcloud/templates/public/rudel/style.css
    - cmd/holzcloud/templates/public/weide/style.css
decisions:
  - "Image position is the existing Block.Variant (rechts, oben, unten; empty = left); no migration, old markup byte-identical"
  - "Staging lives on disk under data/media-staging/{website}/{token}, not in SQLite"
  - "keepUpload split into checkUpload + keepChecked; the AI path keeps calling keepUpload and never asks"
metrics:
  tasks: 3
  completed: 2026-10-26
status: complete
actuals:
  tokens: 36000
  tasks: 3
  commits: 3
plan_head_before: 298152fa1552a21ff0d95b599f4a6fcb3eb26eab
plan_head_after: 165966e0f21813623111a5a6724f74b076cf185b
---

# Phase quick-261026-bon Plan 01: Image position and scale-down question Summary

Image-and-text blocks can put the image left, right, above or below the text in all eight shipped themes, and an upload whose longest side is over 2560 px is held back and the operator is asked on a plain-form page whether to scale it down.

## What was built

**Task 1 (8846285), image position.** `Block.ImagePositionClass` mints `hc-bildtext--rechts|oben|unten` from the constants `ImageRight`, `ImageAbove`, `ImageBelow`. `Set.Clean` keeps only those three on a bildtext block, so the form, the AI tool and the archive all meet one check. The renderer appends the class; markup for "" and "rechts" is byte-identical to before (pinned by test). The editor select offers four options. In `bausteine.css`, oben/unten become one column inside the 45em block and `unten` gets `order: 2` outside any media query. In holzcloud, rudel and weide the stacked modifiers are excluded from the wide slot with `:where(:not(...))` (specificity unchanged), and in holzcloud also from the whole heading-spanning `:has()` family. Spec passage in TEMPLATE-SPEC.md section 4, AI hint, catalogue in four languages plus regenerated de-CH.

**Task 2 (ec99f50), scale-down question.** `media.ScaledSize`, `Intake.Oversize` (header-only, megapixel budget, single-frame GIF) and `Intake.Scaled` (JPEG stays JPEG, PNG stays PNG, WebP to JPEG or PNG by transparency, still GIF to PNG, CatmullRom, through the existing `resizeSlots` throttle). `media.Staging` holds the pictures on disk per website and user: token is 32 lower-case hex characters checked before any path is built, `Take` renames atomically so a double submit gets "expired", `Prune` with `StagingRetention` of one hour. `HandleMediaUpload` stores everything exempt at once, stages the oversize rasters and redirects to `GET /admin/websites/{id}/media/scale?token=...`; one page and one decision cover the whole upload. `POST` re-runs `media.Check` on the staged bytes, then scales or keeps. Job `media-staging-prune` every 15 minutes with `RunAtStart`.

**Task 3 (165966e), docs** in content-model, publishing and configuration, plus the gates and a run of the real binary.

## Deviations from Plan

**1. [Rule 3 - Blocking] Stylesheet version hash.** `cmd/holzcloud/stylesheet_version_test.go` fails when the holzcloud `style.css` changes without `layout.html` being updated. Updated `?v=` in `templates/public/holzcloud/layout.html` (97020d8fd131 to 890c509ea77f). File not on the plan's list. Commit 8846285.

**2. [Rule 3 - Blocking] Template registry.** `internal/web/render.go` keeps a list of admin page names (`layoutPageNames`); `media_scale` had to be added or `ParseAdminTemplates` refuses the new template. Commit ec99f50.

**3. [Rule 2] Small admin.css block** for `.scale-files` and `.scale-question .form-actions` (the plan allowed it "only if needed"; `form-actions` has no rule anywhere, so two buttons would not have been spaced). Plain CSS, kebab-case, inside `@layer components`.

**4. Worktree base.** The worktree started at ee3c9ed (main with PR 47 merged), not at the plan commit 298152f, so `merge-base` did not match and the plan file was absent. The tree was clean and the branch my own, so I ran `git reset --hard 298152f` as the startup check allows. Everything below is on top of 298152f. The orchestrator should be aware that PR 47 (ee3c9ed) is not in this branch's history.

**5. TDD order.** Tests for Task 1 were written in the same pass as the code, not run red first. They pin the behaviour list; I did not demonstrate each one failing first.

## Verification

- `gofmt -l internal cmd tools`: empty.
- `go vet ./...`: clean.
- `go run ./tools/i18n`: 2067 strings, de/fr/es/it at 0 open, 0 orphaned; de-CH regenerated (106 deviations).
- `go run ./tools/themewords -check`: green.
- `go run ./tools/english`: "no German in the Go source outside the catalogues". It is green here; the plan expected one pre-existing finding in build/devseed/main.go, which does not exist at the plan commit.
- `go test ./... -timeout 30m`: exit 0 (the admin package alone takes about 380 s).

## Browser check: not done through Playwright, done over HTTP instead

No Playwright MCP tool was available to this agent, so **the visual part of the plan's browser check was not performed**: nothing was looked at at 1280 px or 390 px, no screenshots exist, and I have not seen the four positions rendered in the default theme, holzcloud, rudel or weide. The CSS was reviewed by reading and brace-counted; that is all. This should be checked by eye (or by a later Playwright run) before release.

What I did instead: built with `CGO_ENABLED=0`, ran the binary against a fresh data directory on port 18457, completed setup and the compulsory 2FA step, created a website, and drove it with a Python/urllib script (plain HTTP, no JavaScript, which is the no-JS path by construction):

- Upload of a 3000x1800 JPEG plus a 400x300 PNG in one request: 303 to the scale page, the PNG in the library at once, the JPEG not; page lists only the JPEG, "3000 x 1800 px, becomes 2560 x 1536 px", plain POST form to `/admin/websites/1/media/scale`, no `hx-post`; one staging directory.
- Answer No: original stored at 3000x1800, staging empty. Same JPEG again with Yes: stored at 2560x1536 (read with `sips`), flash "File uploaded - scaled down to fit within 2560 px - please still enter an image description".
- Reusing the answered token: 303 back to the library (expired flash), no error.
- A page with four Image-and-text blocks (left, rechts, oben, unten) served on a domain: the public HTML carries `hc-bildtext`, `... --rechts`, `... --oben`, `... --unten` in that order.

## Known limits

- EXIF orientation is not applied when scaling. The existing upload path already drops the EXIF block (including orientation) and the existing scaled copies ignore it too, so a scaled copy behaves like the original as stored. Not changed.
- The colour profile is not carried over into a scaled copy (documented in publishing.md).
- No CHANGELOG entry and no version bump, per the plan: the release of milestone v0.1.1 owns both. The owner can add an entry if a release is cut earlier.
- New block markup only appears on pages saved after this change (content_html is frozen at save).

## Known Stubs

None.

## Threat Flags

None beyond the plan's register (T-bon-01 to T-bon-09 are implemented as written: token form check before path building, per-website directory plus per-user manifest check, `media.Check` re-run on staged bytes, DecodeConfig before decode and the existing throttle, 0700/0600 permissions plus prune, class minted from constants, route behind the CSRF mux and listed in the route table, atomic `Take`, re-encode drops metadata and a test asserts the marker string is gone).

## Self-Check: PASSED

Created files exist (internal/block/imageposition_test.go, internal/bundle/blocks_position_test.go, internal/media/scale.go, scale_test.go, staging.go, staging_test.go, internal/admin/media_scale.go, media_scale_test.go, cmd/holzcloud/templates/admin/media_scale.html) and commits 8846285, ec99f50, 165966e are ancestors of HEAD.
