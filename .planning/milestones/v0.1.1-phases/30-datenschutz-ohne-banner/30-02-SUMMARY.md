---
phase: 30-datenschutz-ohne-banner
plan: 02
subsystem: shop-cookies
tags: [cookies, dsgvo, secure-flag, regression-test]
requires: []
provides:
  - shop.AudienceCookieName
  - shop.AudienceLifetime
  - shop.CartCookieName
  - routerTweaks.shop / .secure / .seed / .searchPlugin
  - setCookies / cookieFindings / seedThemeSites / publicGetMatrix
affects: [30-04, 30-06]
tech-stack:
  added: []
  patterns: [matrix test through the real router over every shipped theme]
key-files:
  created:
    - cmd/holzcloud/public_cookies_test.go
  modified:
    - internal/shop/pricing.go
    - internal/shop/pricing_test.go
    - internal/shop/cart.go
    - internal/public/shop.go
    - cmd/holzcloud/main_test.go
decisions:
  - "/kasse is asserted as exactly 303 (empty basket redirects), not 200-or-303"
status: complete
commits: 3
plan_head_before: b0995b9
plan_head_after: 9b19bbe6e5c9bce5e69d8ef9997707d285b6cd31
actuals:
  tokens: 19000
  tasks: 3
  commits: 3
---

# Phase 30 Plan 02: Cookie lifetime, Secure flag and the no-cookie-on-GET matrix Summary

The price-view cookie `hc_preise` now lives 30 days (one constant, `AudienceLifetime`) and takes its Secure flag from the installation, and a test through the real router proves for all eight shipped themes that no public GET of 19 addresses sets a cookie.

## Tasks

| Task | Commit | Content |
| ---- | ------ | ------- |
| 1 (tracer) | 13ab848 | `AudienceCookieName`, `AudienceLifetime` (30 d), `HandlePriceSwitch` uses `h.secure`; unit test `TestAudienceCookieLastsThirtyDays`; router test `TestPriceViewCookieEndToEnd`; `routerTweaks.shop`, `.secure` |
| 2 | fa0badc | `routerTweaks.seed`, `.searchPlugin`; `seedThemeSites`, `publicGetMatrix`, `TestNoPublicGetSetsACookie`, `TestEveryShippedThemeIsInTheMatrix` |
| 3 | 9b19bbe | `shop.CartCookieName`; `TestVisitorTriggeredCookiesAreVisibleToTheDetector`, `TestCookieDetectorFlagsASetCookieOnAGet` |

## Matrix: status per address (identical on all eight themes)

| Address | Status |
| ------- | ------ |
| `/` | 200 |
| `/ueber-uns` | 200 |
| `/geheim` (gate) | 401 |
| `/aktuelles` | 200 |
| `/aktuelles?seite=2` | 200 |
| `/tag/werkstatt` | 200 |
| `/suche?q=Gartenzwerg` (search plugin, hit title in body) | 200 |
| `/gibt-es-nicht` | 404 |
| `/shop` | 200 |
| `/shop/tisch` | 200 |
| `/warenkorb` | 200 |
| `/kasse` | 303 |
| `/feed.xml` | 200 |
| `/sitemap.xml` | 200 |
| `/robots.txt` | 200 |
| `/llms.txt` | 200 |
| `/t/style.css` | 200 |
| `/bestellung/unbekannt` | 404 |
| `/zahlung/zurueck/unbekannt` | 404 |

Every response has an empty Set-Cookie header.

## Recorded red runs

- Router test with the handler reverted to `r.TLS != nil`: `TestPriceViewCookieEndToEnd` failed with `cookie "hc_preise=business; Path=/; Max-Age=2592000; HttpOnly; SameSite=Lax" lacks "Secure"`; restored.
- Matrix with a deliberate `shop.SetAudienceCookie(w, true, shop.Business)` at the top of `HandleHome`: `TestNoPublicGetSetsACookie` failed on all eight themes, each with `<theme> GET / sets a cookie without the visitor doing anything: ["hc_preise=business; ..."]`; the line was removed (handler.go is unchanged against HEAD).

## Deviations from Plan

- **`/kasse` tightened to exactly 303** rather than "200 or 303": the fixture basket is empty and the checkout sends the visitor back to the basket. `/warenkorb` covers the page that shows the empty basket. Not a hole; recorded in a comment in the matrix.
- **Search hit assertion uses "Geschichte"** (occurs only in the hit's title) instead of the query word, which is echoed on the page and would pass without a hit.
- **`.planning/` is gitignored** since b0995b9, so this SUMMARY cannot be committed in the worktree branch; it lives at `.planning/phases/30-datenschutz-ohne-banner/30-02-SUMMARY.md` in the worktree only (untracked, ignored).
- The `searchPlugin` tweak reads the module with `os.ReadFile` and fails with `t.Fatalf` when missing (no `wasmtest.Module`, which skips without `HOLZCLOUD_TEST_REQUIRE_WASM`).

No other deviations. No stubs. No new dependencies (`go.mod`/`go.sum` unchanged against e2513b1). No new threat surface.

## Verification

- `go test ./internal/shop/ ./internal/public/ ./cmd/holzcloud/ -count=1`: ok (shop 5.6s, public 35s, cmd 24s)
- `go build ./...`, `go vet` of the three packages, `gofmt -l internal cmd`: clean
- `go run ./tools/i18n`: 0 open, 0 orphaned; `go run ./tools/english` finds 0 offenders in the five trees
- `grep -rn 'audienceCookie\|cartCookie'` over internal and cmd: 0; `grep -c Skip` in the new test file: 0; eight passing theme subtests

## Self-Check: PASSED
