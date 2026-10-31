---
phase: 30-datenschutz-ohne-banner
plan: 04
subsystem: privacy-text
tags: [privacy, cookies, dsgvo, template, source-guard]
requires: [30-01, 30-02]
provides:
  - privacy.Features, privacy.Cookies, privacy.Cookie, privacy.Trigger
  - privacy.Compose, LifetimeText, HasTemplateNotice, TemplateNotice, NoLegalAdvice, Fillin, HeadingCookies, DraftTitle, DraftSlug
  - page.UnlockCookiePrefix, page.UnlockLifetime
affects: [30-05, 30-06]
key-files:
  created:
    - internal/privacy/cookies.go
    - internal/privacy/cookies_test.go
    - internal/privacy/text.go
    - internal/privacy/text_test.go
  modified:
    - internal/page/access.go
    - internal/public/access.go
    - tools/english/main.go
requirements: [DSV-02, DSV-04, DSV-06]
status: complete
commits: 3
plan_head_before: 47f87dcb70a92e9830923d4f76461d9aec211cf3
plan_head_after: 250d84466b3d45870d6ebc4c381f2bc30aa8fd71
actuals:
  tasks: 3
  commits: 3
---

# Phase 30 Plan 04: Privacy text composer and cookie register Summary

`privacy.Compose(Features)` returns a deterministic German privacy template built from blocks for the functions a website uses; its cookie rows come from a register that reads the code's exported constants, its log numbers from `internal/web`, and tests tie it to the journald drop-in and to the set of files that write cookies.

## Tasks

| Task | Commit | What |
|------|--------|------|
| 1 | dee8637 | `page.UnlockCookiePrefix`/`UnlockLifetime` exported (public keeps local aliases); `privacy.Cookies` register; four tests incl. the cookie-writer source guard |
| 2 | 926d6d2 | `Compose` core (notice, responsible body, server log, cookies and processors with two tables, rights, closing), `LifetimeText`, `HasTemplateNotice`; allowlist entry in `tools/english`; six tests incl. drop-in coupling |
| 3 | 250d844 | Blocks for orders, online payment (with Payrexx row in the processor table), protected pages, forms; three tests |

## Red runs

- `web.LogRetentionDays` changed 14 to 15: `TestRetentionComesFromTheDropIn` failed twice ("the drop-in keeps 14 days but web.LogRetentionDays says 15 ..." and "the composed text does not say 14 Tage"); restored to 14.
- A scratch file `internal/web/zz_scratch.go` with `http.SetCookie`: `TestEveryCookieWriterIsAccountedFor` failed naming the file and telling to extend register, text and list; file removed.
- The `tools/english` gate saw the German in `text.go` before the allowlist entry (count above 0) and reports 0 after.

## Wording choices within CONTEXT.md discretion

- Du-form throughout, same as the existing starter pages; operator instructions and visitor statements share the voice.
- Section order: Datenschutz auf einen Blick, Verantwortliche Stelle, Server-Protokoll, Cookies und Auftragsverarbeiter (subsections Cookies, Auftragsverarbeiter und Empfänger), blocks (Bestellungen im Shop, Online-Zahlung, Geschützte Seiten, Formulare), Deine Rechte, rule, closing sentence.
- Legal basis column: all cookies "§ 25 Abs. 2 Nr. 2 TDDDG"; the basket cookie additionally "Art. 6 Abs. 1 lit. b DSGVO" (contract).
- With no cookie the section says "Allein durch das Öffnen einer Seite setzt diese Website kein Cookie." and the subsection "Diese Website setzt keine Cookies."; the "no banner needed" sentence appears only when cookies are listed.
- Trigger wording: "Wird gesetzt, wenn du etwas in den Warenkorb legst." / "... die Preisansicht wechselst." / "... das Passwort einer geschützten Seite eingibst."

## Statements left to a fill-in marker (not confirmable from code)

Responsible body; hosting provider (and its Art. 28 contract); every further recipient; retention of orders; payment provider name, address, link to its privacy statement and contract date; retention of form enquiries.

## Deviations from Plan

**1. Payment-off test narrowed.** Plan: with Payment off none of "Payrexx, Art. 28, customer data words, marker" occurs. The hosting row (required by the plan) names Art. 28 and the orders block names address data, so `TestPaymentBlockNamesWhatIsSent` asserts absence of Payrexx, "Zahlungsanbieter", "Zahlungsseite" and the payment heading instead.

**2. Guard skips `.claude`.** The cookie-writer walk skips `.claude` (worktrees hold whole tree copies), as 30-03 did for its guard.

**3. Worktree base was stale.** Branch started at b0995b9, not an ancestor of `main`; reset to `main` (47f87dc) before any work.

**4. Frozen gate not re-run.** Per 30-01 summary the `28-check.sh frozen` gate is red at baseline on this branch history for reasons unrelated to this plan; not repeated. `go.mod`/`go.sum`, `internal/admin` and `internal/public/payment.go` are unchanged against 47f87dc (empty diff).

**5. Message-file commits.** Commit messages were written to the scratchpad and committed with `-F` because the sandbox refused compound git commands.

## Verification

- `go test ./internal/privacy/ ./internal/page/ ./internal/web/ ./internal/public/ -count=1`: ok (13 tests in privacy)
- `go build ./...`, `go vet` of privacy, page, public, tools/english: clean; `gofmt -l internal tools cmd`: empty
- `go run ./tools/i18n`: 2067 translated, 0 open, 0 orphaned
- `go run ./tools/english`: 0 lines naming internal, cmd, tools, plugins or sdk

## Known Stubs

None. The `(eintragen)` markers are intentional template placeholders (D-04); 30-05 and 30-06 offer the text only as a draft.

## Threat Flags

None.

## Self-Check: PASSED

internal/privacy/{cookies,cookies_test,text,text_test}.go exist; commits dee8637, 926d6d2, 250d844 are on the branch.
