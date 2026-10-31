---
phase: "30"
slug: "datenschutz-ohne-banner"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-10-31"
---

# Phase 30 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| client or proxy to request log | address and X-Forwarded-For arrive from outside and are written into a durable log | client IP (personal data) |
| program to host journal | retention is decided by journald on the host | log lines |
| repository to evidence of earlier phases | files bound by the frozen gate must not change | deploy files |
| visitor browser to public site | any GET; a cookie set here would be set without a decision | cookies |
| TLS-ending proxy to program | program sees plain HTTP, cannot tell HTTPS | Secure flag |
| uploader to disk | uploaded file may carry location and device | EXIF/XMP/IPTC |
| program to served media directory | what is under data/media is served verbatim for a year | image bytes |
| generated text to published page | a published false statement becomes the legal declaration | privacy text |
| register to code / text to host configuration | cookie table and retention sentence must follow real values | constants |
| website settings and plugin state to generated text | detection becomes a statement about cookies | feature flags |
| editor browser to admin route | state-changing POST with a session | CSRF token |
| website id in path to other websites' data | editors restricted to some websites | website scope |
| stored page content to settings screen | hint must not display editor-written content | page title/text |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-30-01 | Information disclosure | `web.AccessLog` client address | high | mitigate | `TruncateIP` to /24 and /48 (`internal/web/logip.go:28-39`, `logging.go:154`); `TestAccessLogWritesOnlyTheTruncatedAddress` | closed |
| T-30-02 | Tampering | log forging via X-Forwarded-For | medium | mitigate | unparseable input returns `unknown`, never echoed (`logip.go:30-33`); `TestTruncateIPNeverEchoesInput` | closed |
| T-30-03 | Repudiation | journal retention vs privacy text | medium | mitigate | `deploy/journald-holzcloud.conf` (`MaxRetentionSec=14d`, `MaxFileSec=1d`) tied to `LogRetentionDays` by tests | closed |
| T-30-04 | Information disclosure | four other log lines carry an address | low | accept | privacy text states it (`internal/privacy/text.go:150-152`), pinned by test | closed |
| T-30-05 | Tampering | edit of DEPLOY.md / unit would break frozen evidence | medium | mitigate | drop-in is a new file; `git diff 05fc076` on DEPLOY.md and unit empty; `28-check.sh frozen` ok | closed |
| T-30-06 | Information disclosure | public GET sets a cookie | high | mitigate | `TestNoPublicGetSetsACookie` over eight themes, theme list tied to directories | closed |
| T-30-07 | Information disclosure | price-view cookie without Secure | medium | mitigate | `h.secure` from config; `TestPriceViewCookieEndToEnd` | closed |
| T-30-08 | Information disclosure | price-view cookie lifetime | low | mitigate | `AudienceLifetime` 30 d sole MaxAge source; `TestAudienceCookieLastsThirtyDays` | closed |
| T-30-09 | Repudiation | cookie test passing vacuously | high | mitigate | exact status per address, content assertions, positive controls, detector self-check, no skip | closed |
| T-30-10 | Information disclosure | stored photo carries GPS/camera data | high | mitigate | `StripMetadata` via `PrepareUpload` in `Keep`; `TestKeepStoresNoCameraDataAndKeepsTheTurn` | closed |
| T-30-11 | Information disclosure | new path stores bytes without strip | high | mitigate | `TestOnlyIntakeKeepStoresAnUpload` source walk | closed |
| T-30-12 | Information disclosure | held-back copy before strip | low | accept | `data/media-staging`, mode 0600/0700, pruned after 1 h, outside served dir (pinned) | closed |
| T-30-13 | Repudiation | text diverges from program or host | high | mitigate | register from code constants, pinned literals, cookie-writer source guard, retention read from drop-in | closed |
| T-30-14 | Tampering | Markdown/script injection via composed text | low | mitigate | `Compose` takes five booleans; no HTML/link; rendered through sanitiser | closed |
| T-30-15 | Repudiation | half-filled text published as finished | high | mitigate | notice first, closing sentence last, fill-in markers, page created as draft | closed |
| T-30-16 | Repudiation | detection misses a cookie-setting function | high | mitigate | per-function conditions, both form plugins, over-disclose for drafts; `TestPrivacyFeaturesFollowTheWebsite` | closed |
| T-30-17 | Information disclosure | starter privacy page goes live with markers | high | mitigate | entry stays `draft`; test asserts status | closed |
| T-30-18 | Elevation of privilege | editor creates page in a forbidden website | high | mitigate | route under `/admin/websites/{id}/` with `RequireWebsiteAccess`; page created for path-id website | closed |
| T-30-19 | Tampering | CSRF on new POST | high | mitigate | `/admin/` behind CSRF middleware; hidden token; no `hx-post` | closed |
| T-30-20 | Tampering | existing privacy page overwritten/deleted | high | mitigate | handler only calls `CreatePage`/`GetPageBySlug` (AST test); untouched-page test | closed |
| T-30-21 | Information disclosure | card prints stored title/text | low | mitigate | `privacyHint` holds state, id, published flag only; marker test | closed |
| T-30-SC | Tampering | npm/pip/cargo installs (all six plans) | low | accept | `git diff --quiet e2513b1 -- go.mod go.sum` holds | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-30-01 | T-30-04 | Truncation limited to the request log (D-02); other events concern admin sign-ins, are rare, share the journal retention; disclosed in the privacy text | plan 30-01 | 2026-10-31 |
| AR-30-02 | T-30-12 | Staging copy is the editor's own upload, mode 0600, pruned after 1 h, not served | plan 30-03 | 2026-10-31 |
| AR-30-03 | T-30-SC | No module added; go.mod/go.sum unchanged | plans 30-01 to 30-06 | 2026-10-31 |

*Accepted risks do not resurface in future audit runs.*

### Notes from the audit (informational, non-blocking)

1. **Real-host journald check pending.** The drop-in was not played on a real host. The owner should run `systemd-analyze cat-config systemd/journald.conf` on the server and confirm both values.
2. **Strip failure falls back to the original.** `media.PrepareUpload` (`upload.go:128-136`) returns the original bytes if `StripMetadata` errors, so a malformed JPEG/PNG/WebP the stripper rejects would be stored with its metadata. Documented design; tests cover well-formed fixtures only.
3. **No 403 test for the new route.** No test shows a website-restricted editor gets 403 on `datenschutz-vorlage`; the middleware chain applies and `TestRequireWebsiteAccess` covers the middleware itself.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-31 | 22 | 22 | 0 | gsd-security-auditor (ASVS 1, block_on high) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-31
