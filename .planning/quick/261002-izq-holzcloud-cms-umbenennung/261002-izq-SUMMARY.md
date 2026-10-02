---
phase: quick-261002-izq
plan: 01
requirements-completed: [RENAME-01]
completed: 2026-10-02
---

# Quick 261002-izq: Rename remainder to holzcloud-CMS

All operator-facing mentions of the program now read "holzcloud-CMS"; CHANGELOG 0.0.17 added.

## Commits
- 3a63a2c docs: README, docs/*, deploy/*, default theme comment (anchor in docs/security.md updated)
- 07dd568 feat: admin templates, mail subjects, TOTP issuer, DefaultName, locale catalogues, tests
- c109e47 docs: CHANGELOG 0.0.17

## Deviation
[Rule 1] Brand initial: the "H" mark is a separate constant (DefaultMark), not derived from DefaultName. But `IsDefault()` (which decides mark SVG vs letter square) compares Name == DefaultName, and a save with an empty field had stored "Holzcloud" in app_settings. Without a fix such installations would flip from the CMS mark to an "H" square after upgrade. Added `legacyDefaultName` mapping in `branding.Load` plus test `TestTheOldStoredDefaultNameStillCountsAsTheDefault`; CHANGELOG mentions it.

## Gates (all green)
gofmt clean, go vet, tools/english, tools/cites -check, tools/assembled -check, tools/themewords -check, tools/i18n (1981 strings, 0 open, 0 orphaned for de/es/fr/it; de-CH regenerated via -schweiz, 0 without counterpart). Tests: cmd/holzcloud, internal/web, i18n, branding, totp, admin (Mail|TwoFactor|Brand|Dashboard|Account and Changelog|Check), root changelog test, internal/template: all ok. Stylesheet version test passed without change.

## i18n approach
Catalogue keys are the English sentences; renamed in de/fr/it/es/de-CH by textual swap, then `-write` (re-sorted, no open/orphaned) and `-schweiz`.

## Leftover "Holzcloud" (intentional)
mcp.go Bearer realm, loader.go theme entry, public/holzcloud/style.css header (theme), code comments (ai/token.go, bundle/format.go, sdk/plugin.go, migration 00042), test name TestHolzcloudStylesheetVersion..., legacyDefaultName and its test.

## Orchestrator follow-up
docs/brand/holzcloud-social.png (and possibly docs/brand/banner.png) still show the old name; untouched. docs/brand/README.md only describes them. README image alt text updated.
