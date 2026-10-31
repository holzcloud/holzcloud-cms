---
phase: 30-datenschutz-ohne-banner
plan: 05
subsystem: privacy-text
tags: [privacy, starter-content, feature-detection, dsgvo]
requires: [30-04]
provides:
  - page.Store.HasProtected
  - admin.Handler.privacyFeatures
  - admin.formPluginIDs
  - starter datenschutz page composed by privacy.Compose
affects: [30-06]
key-files:
  created:
    - internal/admin/privacy_features.go
    - internal/admin/privacy_features_test.go
    - internal/admin/starter_test.go
  modified:
    - internal/page/access.go
    - internal/page/access_test.go
    - internal/admin/starter.go
requirements: [DSV-06]
status: complete
commits: 2
plan_head_before: a6144055a927253cc28ba81ef1c97f8cdeca1f0b
plan_head_after: eb6d3fc
actuals:
  tasks: 2
  commits: 2
---

# Phase 30 Plan 05: Which functions a website uses, and the composed starter privacy page Summary

`Handler.privacyFeatures` answers which functions of a website the privacy text tells apart (shop, price choice, payment, protected pages, forms), and the starter `datenschutz` page of a new website is now `privacy.Compose` of exactly those functions, still a draft and still second in the footer menu.

## Tasks

| Task | Commit | What |
|------|--------|------|
| 1 | 6f51df5 | `page.Store.HasProtected` (live page, password access, any status, wastebasket excluded); `Handler.privacyFeatures` and `formsEnabled`; `formPluginIDs` = kontaktformular, bestellung |
| 2 | eb6d3fc | `starterPage.compose`; `createStarterContent` reads the website back and detects its functions; the old "keine Cookies" literal is gone; three tests |

## States of the detection that are tested

`TestHasProtected` (page): no page; unprotected draft; protected draft (counts); another website's protected page (does not count); protected page in the wastebasket (does not count); protected published page; protection switched off.

`TestPrivacyFeaturesFollowTheWebsite` (admin, real stores, the shipped manifest and wasm module of the plugins; 19 subtests): fresh website (zero Features); shop address set with private prices; both price modes (PriceChoice); business-only prices (no PriceChoice); payment with instance and secret; instance only; secret only; credentials without a shop (no Payment); protected published page; protected draft; protected page in the wastebasket; contact form installed, enabled, assigned; order form installed, enabled, assigned; contact form assigned to another website only; contact form disabled; order form disabled; plugin manager present but no form plugin installed; no plugin manager at all; another website's protected page.

Starter tests: `TestStarterPrivacyPageIsTheComposedTemplate` (zero Features, stored HTML is the rendering, draft, footer menu impressum then datenschutz), `TestStarterPrivacyPageFollowsTheWebsiteFunctions` (shop + both prices + protected page gives the three cookies named in the text), `TestStarterLegalPagesAreUnchanged`.

## Gaps the detection does not cover (for the owner to decide)

- Only `kontaktformular` and `bestellung` count as form plugins. Any other installed plugin that stores visitor input (a guestbook, newsletter sign-up, a third-party plugin) does not switch `Forms` on. The list is `formPluginIDs`; a new form plugin must be added there.
- A form plugin that is enabled and assigned but not running (load error) still counts; the detection reads installed/enabled/assigned state, not the running state. That errs on the side of disclosure.
- The payment condition is the configuration (instance and secret in the environment), not whether any page actually offers payment at the moment.
- A shop that is later switched on, a page protected later, or a form plugin installed later does not change an already created privacy page (D-06); plan 30-06 offers the current template as a draft for that.

## Deviations from Plan

**1. Worktree base was stale.** The branch started at b0995b9, not an ancestor of `main`; reset to `main` (a614405) before any work.

**2. SUMMARY location.** The sandbox refused a write to the main checkout path, so this file is written under the worktree's `.planning/` (same relative path); the orchestrator copies/commits it. STATE.md and ROADMAP.md were not touched.

**3. Commit messages via `-F`.** Written to the scratchpad and committed with `-F`, since the sandbox refuses compound git commands.

Otherwise the plan was executed as written.

## Verification

- `go test ./internal/page/ ./internal/admin/ -count=1`: ok (page 7.4s, admin 474s; the run was made before a gofmt-only alignment fix to the struct literal in starter.go; the three new starter tests were re-run afterwards and pass)
- `go build ./...`, `go vet ./internal/page/ ./internal/admin/`: clean; `gofmt -l internal`: empty after the fix
- `go run ./tools/i18n`: 2067 translated, 0 open, 0 orphaned
- `go run ./tools/english`: 0 lines naming internal, cmd, tools, plugins or sdk
- `grep -c 'Diese Website setzt keine Cookies' internal/admin/starter.go`: 0; `grep -c 'privacy.Compose'`: 1
- `go.mod` / `go.sum` unchanged against a614405

## Known Stubs

None. The `(eintragen)` markers inside the composed text are intentional template placeholders (D-04); the page stays a draft.

## Threat Flags

None. The detection only reads; no existing page of any website is edited, published or deleted.

## Self-Check: PASSED

internal/admin/{privacy_features,privacy_features_test,starter_test}.go exist; commits 6f51df5 and eb6d3fc are on branch worktree-agent-af33585ba0b857fa1.
