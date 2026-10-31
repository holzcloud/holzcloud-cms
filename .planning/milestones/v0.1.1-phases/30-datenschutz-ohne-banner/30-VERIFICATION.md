---
phase: 30-datenschutz-ohne-banner
verified: 2026-10-31T11:00:00Z
status: passed
score: 7/7 requirements verified in code and tests; card UAT driven; journald real-host check deferred (see 30-HUMAN-UAT.md)
covered_files:
  - .planning/phases/30-datenschutz-ohne-banner/30-01-PLAN.md
  - .planning/phases/30-datenschutz-ohne-banner/30-01-SUMMARY.md
  - .planning/phases/30-datenschutz-ohne-banner/30-02-PLAN.md
  - .planning/phases/30-datenschutz-ohne-banner/30-02-SUMMARY.md
  - .planning/phases/30-datenschutz-ohne-banner/30-03-PLAN.md
  - .planning/phases/30-datenschutz-ohne-banner/30-03-SUMMARY.md
  - .planning/phases/30-datenschutz-ohne-banner/30-04-PLAN.md
  - .planning/phases/30-datenschutz-ohne-banner/30-04-SUMMARY.md
  - .planning/phases/30-datenschutz-ohne-banner/30-05-PLAN.md
  - .planning/phases/30-datenschutz-ohne-banner/30-05-SUMMARY.md
  - .planning/phases/30-datenschutz-ohne-banner/30-06-PLAN.md
  - .planning/phases/30-datenschutz-ohne-banner/30-06-SUMMARY.md
  - cmd/holzcloud/access_log_ip_test.go
  - cmd/holzcloud/public_cookies_test.go
  - cmd/holzcloud/templates/admin/website_form.html
  - deploy/journald-holzcloud.conf
  - internal/admin/privacy_draft.go
  - internal/admin/privacy_features.go
  - internal/admin/starter.go
  - internal/media/intake_exif_test.go
  - internal/media/upload_guard_test.go
  - internal/privacy/cookies.go
  - internal/privacy/text.go
  - internal/shop/pricing.go
  - internal/web/logging.go
  - internal/web/logip.go
covered_digest: "v4:sha256:d3bb0f62b68febaa9e2e43c787739624880c816d1772e62da32baa7554471fd6"
behavior_unverified: 1
overrides_applied: 0
behavior_unverified_items:
  - truth: "DSV-02: the host deletes request-log lines after 14 days (journald honours MaxRetentionSec=14d / MaxFileSec=1d)"
    test: "On the production host install deploy/journald-holzcloud.conf, restart systemd-journald, run `systemd-analyze cat-config systemd/journald.conf`"
    expected: "Output lists MaxRetentionSec=14d and MaxFileSec=1d; journalctl --list-boots / oldest entry is not older than ~15 days after two weeks"
    why_human: "The drop-in file and its coupling to the constant and the text are test-held, but the deletion itself is journald behaviour; the dev machine has no systemd (the drop-in's own header says so)."
human_verification:
  - test: "Website settings screen (Playwright UAT, not done): open /admin/websites/{id}/edit for (a) a website without a page slug datenschutz, (b) one with a published datenschutz page, (c) one with a draft starter page carrying the template notice; click 'Create the template as a draft'"
    expected: "(a) 'no privacy page yet' text; (b) 'published and is left exactly as it is', no template warning; (c) additionally the bold 'do not publish before it is filled in'. The button posts, redirects to the editor of a new draft titled 'Datenschutzerklaerung (neue Vorlage)' with slug datenschutz-vorlage; the original page is byte-identical; the card works with JavaScript disabled and in each admin language."
    why_human: "Visual layout and the clicked flow were not driven in a browser; server-side behaviour is covered by Go tests only."
  - test: "DSV-02 on a real host (see behavior_unverified_items)"
    expected: "check command lists both values"
    why_human: "No systemd on the development machine."
---

# Phase 30: Datenschutz ohne Banner - Verification Report

**Phase Goal:** The CMS is GDPR-compliant without a cookie banner. The public site sets no cookie without the visitor doing something; what is set is necessary or visitor-triggered and listed in the statement. No legal advice: texts are marked as templates.
**Verified:** 2026-10-31
**Status:** human_needed
**Re-verification:** No, initial verification

All seven requirements are present, substantive, wired and test-held in the codebase. No FAILED truth, no blocker. Two items remain for a human: the Playwright UAT of the settings card and the real-host check that journald enforces the drop-in.

## Gate runs (executed in this verification)

| Gate | Result |
| ---- | ------ |
| `go build ./...` | clean |
| `go run ./tools/i18n` | 2077 strings, de/es/fr/it: 0 open, 0 orphaned |
| `go test ./internal/privacy ./internal/web ./internal/media ./internal/shop ./internal/public ./internal/admin ./cmd/holzcloud` | all `ok` (admin 500s) |
| `go run ./tools/english` | exit 1: "2 German lines in build/devseed/main.go". `/build/` is gitignored (`.gitignore:24`) and untracked, so this is a local scratch file outside the phase, not a repo or phase defect. Warning only: remove the stray file or the gate stays red on this machine. |

## Observable Truths (per requirement)

| # | Requirement | Status | Evidence |
| - | ----------- | ------ | -------- |
| 1 | DSV-01 request log writes /24 and /48, non-IP as `unknown`; rate limit keeps full address in memory; activity untouched | VERIFIED | `internal/web/logip.go` `TruncateIP` (netip, Unmap, mask to `LogIPv4Prefix=24` / `LogIPv6Prefix=48`, parse failure returns `UnknownIP`, never echoes input). `internal/web/logging.go:154` is `slog.String("ip", TruncateIP(ip))`; the full address stays in a local variable. `ClientIP` unchanged (`TestClientIPStaysWhole`). `internal/activity` not modified in the phase commits. Tests: `TestTruncateIP*` (number, shape, garbage, forged newline, idempotence), `cmd/holzcloud/access_log_ip_test.go` runs the real router and asserts the log line holds `ip=203.0.113.0` and not `.77`, v6 `2001:db8:abcd::`, and the no-proxy case. |
| 2 | DSV-02 `deploy/` journald drop-in `MaxRetentionSec=14d`; tests hold drop-in, constant, text together | VERIFIED (file and coupling); enforcement is a human item | `deploy/journald-holzcloud.conf` has `MaxRetentionSec=14d` and `MaxFileSec=1d` with install and check instructions and stated limits. `web.LogRetentionDays = 14`; `privacy/text.go` prints it from the constant; `TestRetentionComesFromTheDropIn` parses the file and compares with the constant, `TestLogSentencesComeFromTheConstants` ties the sentence. Real journald behaviour not observable here: behavior_unverified item 1. |
| 3 | DSV-03 test fails when a public GET sends `Set-Cookie`; real router, all eight themes, listed paths | VERIFIED | `cmd/holzcloud/public_cookies_test.go`: `TestNoPublicGetSetsACookie` seeds one website per `tmpl.BuiltinTemplates` (start page, page, protected page, 12 posts for archive page 2, tag, search plugin, shop with 2 products) and runs a matrix through `testRouterWith` (real session middleware): `/`, page, `/geheim` (401 gate), archive, tag, `/suche`, 404, `/shop`, product, `/warenkorb`, `/kasse`, feed, sitemap, robots, llms, `/t/style.css`, order and payment-return 404s. Exact statuses, body words checked, so a vacuous 404 cannot pass. `TestEveryShippedThemeIsInTheMatrix` ties the matrix to the theme directories; `TestCookieDetectorFlagsASetCookieOnAGet` and `TestVisitorTriggeredCookiesAreVisibleToTheDetector` prove the detector sees the three real cookies on POST. Passes. |
| 4 | DSV-04 price-view cookie 30 days, `Secure` behind a TLS proxy as the basket cookie, listed as visitor-triggered | VERIFIED | `shop.AudienceLifetime = 30*24h` is the only source of `MaxAge` in `SetAudienceCookie`. `internal/public/shop.go:303` passes `h.secure` (installation setting, same as `SetCartCookie` at `cart.go:180`), not `r.TLS`. `TestPriceViewCookieEndToEnd` runs secure=true over plain HTTP and asserts `Max-Age=2592000`, `Secure`, `HttpOnly`, `SameSite=Lax`; `TestAudienceCookieLastsThirtyDays`. Register `privacy.Cookies` reads name and lifetime from the shop constants with `TriggerPriceView`; the table row starts "Wird gesetzt, wenn du die Preisansicht wechselst". The route only exists when `Display == both`, which is exactly the `PriceChoice` condition that lists the row. |
| 5 | DSV-05 regression through `Intake.Keep`: GPS and camera EXIF removed, orientation kept; source test that no other code stores an upload | VERIFIED | `internal/media/intake_exif_test.go`: `Check` then `Keep` on multipart-parsed uploads (JPEG LE and BE, PNG, WebP; EXIF, XMP, IPTC, comment, GPS markers), fixture self-check `mustCarry`, stored bytes read back from disk, `mustNotLeak`, orientation 6 preserved by `orientationOf`; scaled copy has orientation 1 with pixels turned; byte size recorded is the stored size. `internal/media/upload_guard_test.go` `TestOnlyIntakeKeepStoresAnUpload` walks the whole tree with go/parser (floor of 150 files) and allows `PrepareUpload`/`StoreFile` only in `intake.go` and `StripMetadata` only in `upload.go`; `TestStoringCallsDetectionSeesACall` proves the detector sees calls; staging dir pinned outside `data/media`. Passes. |
| 6 | DSV-06 starter `datenschutz` composed from blocks per function, cookie table (name, purpose, lifetime, legal basis), processor table, still marked as template, no legal advice | VERIFIED | `internal/privacy/text.go` `Compose(Features)`: opens with `TemplateNotice` ("Diese Seite ist eine Vorlage und noch nicht ausgefüllt."), `(eintragen)` markers for operator facts, closes with `NoLegalAdvice`. Blocks: log, cookies/processors, orders (Shop), payment (Payrexx, AV-Vertrag), protected pages, forms, rights. Cookie table header `Name / Zweck / Laufzeit / Rechtsgrundlage`; processor table `Empfänger / Aufgabe / Übermittelte Daten / Vertrag`. Cookies come from `privacy.Cookies` fed by constants; `TestEveryCookieWriterIsAccountedFor` source-walks for unlisted cookie writers. `internal/admin/starter.go` uses `compose: privacy.Compose` fed by `privacyFeatures` (shop, price choice, payment config, protected pages, form plugins assigned to the website); the old "sets no cookies" literal is gone; starter stays draft, second in the footer menu. Text is deterministic, no operator string, no raw HTML. i18n gate: 0 open. Note: at website creation a new site normally has no shop yet, so the starter is mostly the base text; the DSV-07 offer covers later changes. This matches D-04 ("active functions") and is not a gap. |
| 7 | DSV-07 existing privacy page never changed; settings screen shows hint and offers the current template as a new draft | VERIFIED (code and tests); UAT open | `internal/admin/privacy_draft.go` `HandlePrivacyDraft` composes from current features and calls only `CreatePage` (status `draft`, slug `datenschutz-vorlage`, `CreatePage` appends `-N` on collision at `page/store.go:505`); a source test holds the list of page-store calls. `privacyHint` reads state, id and published flag only and the template never prints the stored title or text. Route `POST /admin/websites/{id}/datenschutz-vorlage` registered at `cmd/holzcloud/main.go:1011` behind the protected admin mux (CSRF token in the form). `website_form.html` card "Privacy statement": three states (none / exists / template notice), plain `<form method="POST" action=...>` without `hx-post`, `hx-disabled-elt`. Catalogue keys present in de, es, fr, it. Only the browser walk-through is missing: human item 1. |

**Score:** 7/7 requirements verified in code; 1 behavior-dependent truth (journald enforcement) present but not exercised on a real host.

## ROADMAP goal check

- No cookie without interaction: DSV-03 matrix, over all eight themes, plus the writer-guard in `privacy/cookies_test.go`.
- What is set is necessary or visitor-triggered and listed: three cookies (basket, price view, unlock) in the register, each with a trigger, lifetime from the code's constants.
- Template, no legal advice: notice sentence, `(eintragen)` markers, closing sentence, drafts only.
- Nothing from third parties introduced: the phase adds no subresources (CSP unchanged).

## Assessment of the accepted D-02 exceptions

Four other call sites log a full address: `internal/admin/login.go:45` (login rate-limited), `internal/admin/oidc.go:250` (SSO refusal), `internal/admin/forwardauth.go:884` (forward-auth refusal), and `internal/web/forwardauth.go:165` (`r.RemoteAddr` of a trusted proxy peer sending a wrong shared secret).

- Does this contradict a requirement? No. DSV-01 is scoped to "the request log ... in its `ip` field", and `AccessLog` is the only writer of that field (grep of `"ip"` shows `logging.go:154` as the sole truncated writer; the other three write a differently named record). Roadmap audit point 2 is likewise about `internal/web/logging.go`.
- Does it contradict the phase goal? No, because the sites are admin-only (staff login, SSO) and the fourth logs a proxy peer, not a visitor. The public visitor never reaches them. The privacy text discloses it in `writeLog`: the full address is written on a wrong admin login or a refused SSO sign-in, kept as long, concerning only people working on the site. All four sit in the same 14-day journal.
- Info, not a gap: the text wording covers "wrong login or refused single sign-on" and does not name the rate-limit line or the forward-auth wrong-secret line separately; the first falls under "falsch anmeldet", the second logs a proxy address. If the owner wants the statement exact, truncate these four with `web.TruncateIP` in a follow-up. Not required by DSV-01..07.

## Required Artifacts

| Artifact | Status | Details |
| -------- | ------ | ------- |
| `internal/web/logip.go` | VERIFIED | substantive, used by `logging.go` and tests |
| `deploy/journald-holzcloud.conf` | VERIFIED | read by `privacy/text_test.go` |
| `internal/privacy/{cookies,text}.go` | VERIFIED | used by `admin/starter.go`, `admin/privacy_draft.go` |
| `internal/admin/{privacy_features,privacy_draft}.go` | VERIFIED | wired from `starter.go`, `website.go`, route in `main.go` |
| `cmd/holzcloud/templates/admin/website_form.html` card | VERIFIED (static) | rendered from `PrivacyHint` set in `internal/admin/website.go` |
| test files for DSV-01, 03, 05 | VERIFIED | executed, pass |

## Anti-Patterns

Scan of phase files for TBD/FIXME/XXX: no unreferenced debt markers in the phase's production files. No stubs, no static fallbacks: cookie lifetimes and log numbers flow from constants into code, drop-in test and text.

## Human Verification Required

1. **Settings card in a browser (Playwright UAT, not yet done).** Drive `/admin/websites/{id}/edit` in the three states, click the button, confirm the new draft, the untouched original, no-JS posting and the translated labels (see frontmatter).
2. **Journald on a real host.** Install the drop-in, restart journald, run `systemd-analyze cat-config systemd/journald.conf`; confirm both values after two weeks. This is the owner confirmation the drop-in's header already asks for.

## Gaps Summary

No gaps. Warnings: (a) `go run ./tools/english` is red locally because of an untracked, gitignored `build/devseed/main.go` containing two German lines; outside the phase and not in CI's tree; (b) the four D-02 full-address log lines are accepted and disclosed in the text, optional follow-up to truncate them for exactness.

---

_Verified: 2026-10-31_
_Verifier: Claude (gsd-verifier)_

## UAT result (Playwright, 2026-10-31)

Item 1, the privacy card on the website settings screen, was driven against a dev server on a fresh data directory (new website, starter privacy page still carrying the template notice):

- Card state "page still carries the notice" renders; "Open the page" links to the existing draft.
- "Create the template as a draft" (plain POST, CSRF token, no `hx-*`) created `datenschutz-vorlage` as a new draft; the original `datenschutz` page kept its slug, draft status and body.
- No external subresource on the page.

Not driven: the other two card states (no privacy page, filled-in page; covered by `TestWebsiteSettingsShowThePrivacyCard`), the no-JavaScript and per-language passes.

Still open: item 2, journald on a real host (`systemd-analyze cat-config systemd/journald.conf`).
