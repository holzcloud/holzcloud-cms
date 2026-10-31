# Project Research Summary

**Project:** Holzcloud CMS — milestone v1.6 "Inhaltsmodell und Zugang"
**Domain:** Self-hosted multi-site CMS (Go 1.26 + htmx 2.x + plain CSS + SQLite, single linux/amd64 binary, nothing loads from a third party at runtime)
**Researched:** 2026-09-03
**Confidence:** HIGH

> Detail lives in [STACK.md](STACK.md), [FEATURES.md](FEATURES.md), [ARCHITECTURE.md](ARCHITECTURE.md), [PITFALLS.md](PITFALLS.md). This file synthesizes; it does not replace them.

---

## Executive Summary

**The milestone needs zero new dependencies.** All four researchers arrive there independently: every one of the five phases is served by the standard library plus what is already in `go.mod`. `encoding/json` for the i18n catalogue, `encoding/csv` for the importer, four `r.Header.Get` calls for Authentik forward-auth (the cryptography lives in the outpost — that is the entire reason forward-auth was chosen over an OIDC client, which would require an outbound runtime call and break the project's founding rule). New field kinds need no migration at all, because `page_field_defs.art` is `TEXT NOT NULL` with no CHECK and migration `00028:25–27` says why. htmx stays at the vendored 2.0.10; every control this milestone adds is a plain HTML form control.

**Three premises the milestone was scoped on are wrong, and correcting them is this summary's most valuable output.** (1) The i18n indent problem no longer exists — settled by quick task `260903-bsk` on 2026-09-03 and recorded in `.planning/WINDOWS.md`, confirmed twice independently (byte-level round-trip of all seven catalogues, and an actual `-write` / `-schweiz` run producing an empty `git diff`). What *is* still wrong is the doc comment at `tools/i18n/main.go:287` claiming indented output, and the fact that `fr-CH.json` and `it-CH.json` are never written by the tool at all (`main.go:104–114` `continue`s before the write block). (2) The five `plugin.wasm` files are **committed**, so the test skips currently pass; the real defect is that CI never rebuilds them, so an SDK change validates against a stale binary — a false pass, worse than a skip. (3) The index Phase 8 must swap was already replaced by migration `00038`, not `00029` — the v1.5 planning note is one migration behind — and the next free migration number is **`00046`**, not `00045`.

**The risk is concentrated entirely in Phase 10.** Every other pitfall in this milestone costs a bug report; the forward-auth ones cost the installation. `cmd/holzcloud/main.go:425` binds `:8080` — every interface, IPv4 and IPv6 — so the moment the app trusts an identity header, anyone who can open a TCP connection to that port is an administrator. Compounding it, Caddy **CVE-2026-30851** (`forward_auth copy_headers` emits no delete for the client's own inbound header; v2.10.0–v2.11.1, fixed in 2.11.2) means the proxy demonstrably fails to strip what the app is about to believe. The mitigation is a four-layer ordering (§Critical Pitfalls) built on `web.ClientIPResolver.IsTrustedPeer` — which already exists, is already tested, and already has a working copy-me precedent in `web.RequestID` (`internal/web/logging.go:31–45`). Alongside that sit four latent defects with file:line addresses, each of which turns into a real bug the moment this milestone's code touches it.

---

## Key Findings

### Recommended Stack

Nothing new. Zero additions to `go.mod` across all five phases, no CSP change, no new asset, no new font, no build step. In each of the three places where a library is the usual reflex, the stdlib call is also the *shorter* one: the i18n writer becomes eleven lines that reproduce the committed files byte for byte; the CSV reader is `encoding/csv` with four fields set; the Authentik integration is four header reads.

**Core technologies (verified versions):**
- **Go 1.26.6** — every capability this milestone needs is in its stdlib. All `encoding/json` and `encoding/csv` behaviour claims in STACK.md were *executed* on this toolchain, not read from docs.
- **`modernc.org/sqlite` 1.57.0** — unchanged; `CGO_ENABLED=0`.
- **`pressly/goose/v3` 3.27.3** — Phase 8 needs exactly one new migration file, **`00046`**.
- **htmx 2.0.10 (vendored)** — untouched. Every new control is native HTML and degrades to a plain form.
- **stdlib reached for:** `encoding/json` (Phase 6), `net/http`/`strings`/`time`/`strconv`/`html/template` (Phase 7), `encoding/csv`/`unicode/utf8`/`bytes`/`io` (Phase 9), `crypto/subtle` + existing `net/netip` (Phase 10).

**Explicitly rejected:** `coreos/go-oidc` + `x/oauth2` (outbound runtime call, client secret in config — already ruled out in `docs/offene-punkte.md`); any JOSE library (only needed to verify `X-authentik-jwt`, which buys nothing behind a localhost listener); `gocarina/gocsv` / `csvutil` (struct-tag mapping when the mapping is chosen by the admin at runtime); `encoding/json/v2` (GOEXPERIMENT-gated, outside the Go 1 compatibility promise); any JS date/slider/tag widget.

**One `encoding/json` trap worth restating:** `Encoder.SetIndent("", "")` is a **no-op** — flush-left-with-newlines requires the free function `json.Indent`. And `json.MarshalIndent` *always* HTML-escapes with no switch, which is exactly why the hand-rolled `quote()` exists. Do not "simplify" to it (PITFALLS #25 names this as a content change disguised as a format change).

### Expected Features

Surveyed against Statamic 6, Craft 5, ACF, Kirby 5, Directus, Feed Me, WP All Import and the Statamic Importer — all primary vendor docs.

**Must have (table stakes):**
- **A button row is a *display setting* on `KindChoice`, not a new kind.** Statamic's `button_group`, ACF's `Button Group` and Kirby's `toggles` all store a plain string identical to the dropdown. Store `darstellung ∈ {liste, knopfreihe}`.
- **An optional button row must render an explicit "— keine Angabe —" option.** A radio group cannot be un-set in plain HTML; every other CMS solves this in JS. Without it the author clicks once and can never undo it.
- **A hidden sentinel input must precede a checkbox group.** An all-unchecked group submits *nothing*, so the server cannot distinguish "cleared" from "not on this form". Five-line fix, multi-hour bug if missed.
- **A term field stores the *slug*, not the id or the name** — resolved at render through a `TermLookup` beside `Links.Page`. `term.Store.Rename` already keeps the address (`TestRenameKeepsTheAddress`), so the slug *is* the stable identity here. Storing the id breaks CSV import (a human types `Bio`, not `47`) and bundle portability; storing the name goes stale on rename.
- **`KindTerm` must be excluded from `BlockKinds()`, beside `KindRef`** — a block freezes to HTML on save, so a rename could not follow. The most invisible cross-cutting change in the milestone.
- **`code` is never cast to `template.HTML`, never through goldmark, never through bluemonday.** Go's escaping is the feature. The one place needing care is a `code` field *inside* a block, where the block render path must escape.
- **`zeit` carries no timezone, ever** (Craft's own loud warning) and empty must be distinguishable from midnight — use `KindDate`'s pointer trick.
- **CSV: a mapping screen is table stakes.** One row per *target field*, a `<select>` of the file's headers, "— nicht importieren —" — Feed Me's orientation, which is plain HTML *and* better information design than WP All Import's JS drag & drop. Directus's "headers must match field keys" is the model everyone complains about.
- **CSV: auto-mapping** by `kennung`/`beschriftung`, case- and diacritic-insensitive — ~20 lines, the largest UX win per line in the milestone.
- **CSV: draft by default** unless a Status column says otherwise. 300 pages appearing live is the failure people fear most.
- **Snippet fields reuse `page_field_defs` + a `snippet_id` column** — not a third field table. Third instance of a pattern walked twice (`00037`, `00038`).
- **`.Site.Snippets map[string]template.HTML` must not change type** — it is a published contract in `TEMPLATE-SPEC.md:212` that every installed theme indexes. Add a parallel map (`Site.Bausteinfelder` / `Bausteinliste`) instead.
- **The snippet's Markdown body stays.** A snippet becomes body + optional fields, exactly as a page is content + optional fields. Zero migration for existing content.

**Should have (competitive):**
- **CSV dry run (Probelauf)** — the best idea in the research, and cheap because `field.CheckAll` is already the one shared validator. Turns "is my file right?" from a destructive experiment into a click.
- **Downloadable example CSV** generated from the website's own field definitions. Nobody surveyed does this; it eliminates most mapping questions.
- **"Vorgabe" (default value) per field** for unmapped or empty cells (Feed Me's third column).
- **One-row preview with `‹ ›` navigation** above the mapping table — a plain `GET ?zeile=n`.
- `max` selection count on multiple choice; `step` and min/max on `zeit`; `prepend`/`append` unit text on `bereich`; Craft-style `.Selected` flags exposing unselected options.

**Defer (v2+):**
- Saved, re-runnable import profiles (Feed Me's whole model) — invites scheduling, which invites an outbound call.
- **Per-locale snippet values** — both Statamic and Craft localize globals; here it interacts with the star-shaped locale model in ways not obvious from outside. **Flag for phase-specific research.**
- Arbitrary own field as the CSV match key (a JSON scan per row); Latin-1 transcoding (the refusal message is honest enough); revisions on snippets.

**Never (anti-features):**
- **CSV "delete missing"** — Feed Me's most destructive setting. One mis-mapped key column deletes a website.
- Downloading images from URLs in a CSV (the runtime-third-party rule; `wordpress.go` already refuses and lists URLs instead); all-or-nothing single-transaction write (single-writer SQLite: a file-long transaction blocks every request on the box); a live JS readout beside the `bereich` slider — `internal/tmplmgr/script.go` rejects exactly that pattern in uploaded templates, so building it in the admin would be the project contradicting its own rule.

### The multi-value encoding — load-bearing for Phases 7 and 9

**Both researchers who reached this question converged on the same answer, independently and with the same primary reason: one value per line, in the existing string slot, reusing `SplitChoices`' rule.**

Why it is safe rather than merely convenient: **the delimiter is already illegal inside a value.** `SplitChoices` (`internal/field/field.go:613–622`) reads the admin's option list one per line, so a configured choice *cannot contain a newline*; `page.Slugify` output has none either. This is not "we hope nobody types a newline" — the alphabet of legal values excludes it. That property bounds the recommendation: **it is correct for closed vocabularies only.** A multi-value *free-text* field would require revisiting; v1.6 has none.

The alternatives and why they lose: widening `field.Values` to `map[string][]string` breaks at compile time across eight packages (`internal/bundle` 6 sites, `internal/ai/tools.go` 6 sites, `internal/public`, `internal/admin`) for one field kind. A JSON array serialised into the string slot is double-escaped and unreadable in `sqlite3` or an export. A separate rows table has nowhere to key on inside a repeatable group (group rows are `[]Values` with no persistent row id) and removes the value from the revision diff entirely.

**Stated costs, carried forward honestly:**
- **The CSV boundary needs a second delimiter.** A newline inside a CSV cell is legal RFC 4180 but requires quoting and is invisible in a spreadsheet. So the wire format is a **pipe** (the Statamic Importer's choice) and the importer does `strings.Join(strings.Split(cell, "|"), "\n")`. One line, one sentence of user-facing documentation — but it *is* user-visible.
- **`MaxValueBytes` (4000) is now shared across all selected values.** `CheckAll` should say so rather than truncate.
- **A single-value read of a multi-value field is a footgun.** `{{.Page.Felder.sorten}}` must not print `Bio\nDemeter` — give the slice type a `String()` that joins with ", ".
- **Every consumer must consult the def to know whether to split** — equally true of every alternative, and the def is already in hand at `Resolve`, `CheckAll` and `Entry`.

Implementation shape: `field.SplitValues`/`JoinValues` beside `SplitChoices`/`JoinChoices` (same body, different name so the two concepts can diverge); `page_form.go:150` and `:163` become `field.JoinValues(values)`; `Entry` gains `Values []string`; `List` and `Filled` gain a `case []string`.

### Architecture Approach

This is *integration* research against the working tree at `c58ceb0`, not greenfield. The single load-bearing structural fact: **`page_field_defs` is already a three-namespace table** — page fields (`parent_id IS NULL AND block_type_id IS NULL`), group sub-fields, block-kind fields. Phase 8 adds a fourth namespace, and migration `00038` is a line-for-line template for how, including the partial-unique-index swap it already performed once.

**Major integration points:**
1. **`internal/field`** — five places declare a kind (constant, `Kinds`, `Check`, `Resolve`, `field_input.html`), plus `SubKinds()`, `BlockKinds()`, `MayControl()`, `switchOf`, `List`, `Filled`. **No migration**: `art` has no CHECK.
2. **`internal/field/store.go`** — the carrier discriminator lives in hand-written SQL at seven sites. Line **`:53`** is the page-fields query and is the one that "already works", so it is the one that ships broken.
3. **`cmd/holzcloud/main.go:968`** — the admin chain is *hand-nested*, not built with `auth.Chain`. Phase 10's middleware goes in by editing that one line, between `setupGuard` and `requireAuth`, so that `RequireAuth` → `RequireSecondFactor` → `RequireAdmin` → `RequireWebsiteAccess` all run unchanged.
4. **`internal/admin/login.go:106` `completeLogin`** — the single funnel where a session becomes signed in. A forward-auth handler must be its **fourth caller and nothing else**, preceded by `sm.RenewToken` (session fixation), or the SSO sign-in is invisible in `/admin/protokoll`.
5. **`internal/wxr` + `internal/admin/wordpress.go`** — the three-layer split (pure parse → one-row create returning a reason string → handler: cap, parse, loop, report) that Phase 9 copies verbatim. Copy the shape; improve the mechanism — `csv.Reader.Read()` genuinely streams where `wxr.Parse` materialises the whole document.
6. **The theme contract is test-enforced.** `sample_test.go:17` and `spec_test.go:18` fail the build if a new field on `PageContent`/`Site` is missing from `SampleData`/`MinimalData` or from `TEMPLATE-SPEC.md`. Phases 7 and 8 both add contract fields.

**The Authentik contract, verified from source** at tag `version/2026.8.1` (`internal/outpost/proxyv2/application/mode_common.go`, `getHeaders`), identical at `version-2025.8`:
- Headers: `X-authentik-username`, `-email`, `-name`, `-uid` (the OIDC `sub`, the stable key), `-groups`, `-entitlements`, `-jwt`, `-meta-jwks`, `-meta-outpost`, `-meta-provider`, `-meta-app`, `-meta-version`.
- **`X-authentik-groups` joins group names with `|` (U+007C).** From the source, not a blog. Empty header → no groups; guard against `[""]`. A group name containing a pipe is unrepresentable — authentik does not escape.
- Sign-out is **`/outpost.goauthentik.io/sign_out`**, routed on the *application's own domain*, therefore **same-origin** — no CSP consequence for the link itself.
- Always `r.Header.Get`, never map indexing: Go canonicalises header names.
- **Do not verify the JWT.** Verifying means fetching JWKS at runtime (the one rule this project does not break) or pinning a key (a rotation failure mode discovered at the worst moment); without a signing key authentik signs proxy tokens *symmetrically with the client secret*, so "verify" means two modes plus holding the secret — most of what an OIDC client costs, for none of its benefit. And the signature protects nothing the transport does not: if an attacker can set headers on that socket, they can talk to the CMS directly. **Do not implement a half version** — parsing a JWT without verifying its signature is strictly worse than not having it, because it looks like a defence.

### Critical Pitfalls

**The highest-severity latent defects, each with its address:**

1. **`internal/admin/handler.go:173` — `return assigned == 0 || mine > 0`.** "No assignment means every website" is correct for manual account creation and *inverts* under auto-provisioning: a freshly created SSO account has zero `user_websites` rows by construction, so the first stranger who authenticates at the IdP gets editor access to **every website in the installation**. This is the single most likely way this milestone ships a real vulnerability, because every part looks correct in isolation. **Do not change `handler.go:173`** — that would lock out every existing editor. Instead: auto-provisioning **off by default** (`HOLZCLOUD_SSO_AUTOCREATE=false`); if on, refuse to start without an explicit `HOLZCLOUD_SSO_DEFAULT_WEBSITE` (the project already refuses to start on a half-configured Payrexx pair).
2. **`internal/admin/page_form.go:150` — `out.Values[key] = values[0]`.** A checkbox group named `feld_sorten` submits three values; this keeps the first and drops the rest, silently. `:163` has the identical line for group rows. The fix is *not* "load the defs here" — the function deliberately reads by prefix before definitions are loaded, and that property is load-bearing. Encode multi-valuedness in the **form field name**, minted in the one place that mints names, `field.Def.FieldName()` (`field.go:346`).
3. **`cmd/holzcloud/main.go:425` — `Addr: ":" + cfg.Port`.** Every interface, IPv4 *and* IPv6. The IPv6 listener is the classic one: an operator writes an IPv4-only firewall rule, tests `curl http://<v4>:8080`, sees it refused, and concludes the port is closed. Add `HOLZCLOUD_BIND` (default `127.0.0.1`) and update `deploy/holzcloud.service` and `deploy/DEPLOY.md` in the same commit.
4. **`internal/field/store.go:53` — `WHERE website_id = $1 AND block_type_id IS NULL`.** Missing `AND snippet_id IS NULL` puts every snippet field on every page's edit form and in every theme's `.Page.Feldliste`. Silent; visible only in the browser. Highest-consequence single edit of Phase 8 — do it **first**, before anything else in that phase.
5. **`cmd/holzcloud/assets/admin.css:1104` — `.feld-schalter--auswahl:has(> .form-group option[value=""]:checked)`.** Field conditions are pure CSS. A Choice rendered as a radio button row has no `<option>`, so it **silently breaks every field that hangs on it**. `switchOf` (`page_fields.go:186–195`) must return a new switch name, with a matching `.feld-schalter--knopfreihe` rule. Related: `<input type=range>` never matches `:placeholder-shown`, so `MayControl()` must return **`false`** for `bereich`, exactly as it already does for `KindDate`.

**The forward-auth trust boundary — four layers, in this order.** They are not alternatives; each covers a different failure of the one before:

1. **Validate the peer address first, before reading any header.** `web.ClientIPResolver.IsTrustedPeer(req)` (`internal/web/clientip.go:49–52`) reads `req.RemoteAddr`, which `net/http` sets from the accepted connection and a client cannot spoof. It is fed by `cfg.TrustedProxies` (default `127.0.0.1/32,::1/128`) and already tested. **Reuse it — do not write a second trusted-peer check.** The nuance: an *untrusted* peer carrying no identity header must fall through to ordinary password login, never a 403, or break-glass dies with it. So: untrusted peer → strip every identity header and continue as anonymous.
2. **Strip inbound copies of every identity header unconditionally at the top of the chain — in the app, not only in Caddy** — including the underscore alias (`X_authentik_email`), because Go canonicalises `-` but not `_` and they are two distinct map keys. Three lines that cannot fail. This makes a wrong Caddyfile on someone else's server a misconfiguration rather than a bypass.
3. **A shared secret** the proxy adds, compared with `crypto/subtle.ConstantTimeCompare`, kept in the environment and out of the database (as `PayrexxSecret` already is — "the database is what gets copied into every backup"). This survives a peer-check mistake.
4. **A signed assertion — recommended *against* for v1.6**, with the reason recorded in code so it is not relitigated.

The precedent to copy for layers 1–2 is exact: **`web.RequestID` (`internal/web/logging.go:31–45`)** — `IsTrustedPeer` → sanitise → use. Note the distinction the codebase already draws: `/ai`'s `Authorization: Bearer` is a **secret the server verifies independently**; a forward-auth header is a **claim** whose only guarantee is who the peer is. Nothing in the codebase today trusts a header's claim for identity; Phase 10 introduces the first one.

**Caddy CVE-2026-30851 (GHSA-7r4p-vjf4-gxv4).** `forward_auth … { copy_headers X-Foo }` generates a *conditional* set that only fires when the auth service returns `X-Foo`, and **no delete** for the client's inbound `X-Foo`. When the outpost answers 200 without that header — anonymous route, user with no email, any header authentik chose not to emit — the client's own value reaches the backend verbatim. Affected **v2.10.0–v2.11.1**, fixed in **v2.11.2**; latent since PR #6608 in Nov 2024, so it is in whatever Caddy a typical operator has from the stable apt repo. `deploy/Caddyfile.example` needs an explicit `request_header -X-Authentik-…` line per copied header (plus the shared secret), and `DEPLOY.md` must state a minimum Caddy version of 2.11.2. Companion advisory GHSA-f59h-q822-g45g / CVE-2026-52845 is the underscore variant and is why layer 2 must cover aliases.

**The `form-action 'self'` trap.** `adminCSP` (`internal/web/headers.go:17`) carries `form-action 'self'`. `HandleLogout` is a **POST** (`internal/admin/login.go:128–136`, redirect at `:134`), and a redirect issued in response to a form submission is checked against `form-action` by some browsers — Safari among them. The codebase learned this the expensive way; the Payrexx comment at `headers.go:41–53` records it: *"the TWINT payment on an iPhone dies silently at the moment of the handover, and nothing in the server log says why."* The same-host `/outpost.goauthentik.io/sign_out` route keeps this same-origin and safe, but the moment a separate outpost host is supported it breaks silently. The fix has an exact precedent: `PublicCSP(extraFormAction ...string)` + `SecureHeadersWith(csp)` (`headers.go:59–67`, `:82`, wired at `main.go:1062–1066`) — mirror it as `web.AdminCSP` / `web.AdminHeadersWith` at both `main.go:936` and `:968`, keeping `frame-ancestors 'none'`, `X-Frame-Options: DENY` and `Cache-Control: no-store` (asserted by `main_test.go:220–234`). The alternative that avoids the CSP change entirely: answer the POST with a local `/admin/abgemeldet` page carrying a plain `<a>` — navigation by link is not checked against `form-action`, and CLAUDE.md already blesses outbound hyperlinks as content.

**Other Phase 10 pitfalls that must land in the plan:** logout that does not log out (the browser still holds the authentik cookie and is signed straight back in — redirect to the outpost's `sign_out`, build the target from the request's own host, never from a header, reusing `auth.backTo`/`auth.SafeReturn`; and use `HX-Redirect` + `Vary: HX-Request` because logout is an htmx POST); **CSRF stays on every route** — forward auth makes CSRF *more* relevant, not less, because it adds a second ambient credential the browser attaches automatically; group→role mapping must split on the delimiter and compare whole elements (naive `strings.Contains` matches `not-holzcloud-admins`), must run on **every** request so demotion works, must log every role change, and `HOLZCLOUD_SSO_ADMIN_GROUP` must have **no default**; email is not a stable identity — SQLite's `COLLATE NOCASE` folds ASCII only, so `Müller@` and `müller@` are two rows and two admins; and the password path must survive, because `auth.RequireFreshPassword` (`elevate.go:57`) guards website deletion, user deletion, AI key creation and plugin removal by asking for a password an SSO account does not have.

**Phase 9 pitfalls:** `encoding/csv` has **no limit anywhere** — cap bytes (`http.MaxBytesReader`, 10 MB, matching `wordpress.go:25`), rows, and cells (100 000 bytes; one 9 MB cell is legal within the byte cap); **the BOM is not stripped** — the first header cell arrives as `"﻿Titel"` and never matches a mapping, the number-one CSV import bug; `LazyQuotes = true` for untrusted uploads (a stray quote otherwise swallows the rest of the file into one field); `FieldsPerRecord = -1` so a short row is a *reported row* with a row number rather than a line number; blank lines are skipped silently and a quoted field may span lines, so **row count ≠ line count** — count rows yourself; a NUL byte passes straight through, so check for it; and `page.CreatePage` silently renames colliding slugs to `-2`, `-3` — report every one.

**Phase 6 pitfalls:** any catalogue reformat is **its own commit touching nothing but the catalogues**, proven mechanically with a `jq -S` semantic diff before committing — 4 590 lines otherwise bury the next real change and `git log -S` cannot recover it.

---

## Implications for Roadmap

The milestone's five phases are already fixed (6–10) and the research endorses that order, with one refinement: **Phase 9 depends on Phase 7 only for the multi-value encoding, and not on Phase 8 at all.** If Phase 7's step 1 lands early, 8 and 9 can run in parallel. Phase 10's file set is disjoint from every other phase's and can move anywhere.

### Phase 6: Aufräumen
**Rationale:** Blocks nothing technically, but unblocks the roadmapper and makes the `0 offen, 0 verwaist` translation gate meaningful for Phases 7–10. Must precede any reformat-adjacent work.
**Delivers:** Corrected planning docs; a correct `tools/i18n` doc comment; a plugin build script + CI step; loud-in-CI, forgiving-locally wasm test guards.
**Re-aim these two items — the premises were wrong:**
- **i18n:** the format already matches (verified twice). The work is (a) fix the doc comment at `tools/i18n/main.go:287` which still claims "indented", and (b) decide and document what to do about `fr-CH.json` / `it-CH.json`, which the tool **never writes** (`main.go:104–114` `continue`s past any filename containing `-` before reaching the write block at `:148–152`; `-schweiz` only rebuilds `de-CH.json`). That behaviour is deliberate and documented at `:99–103`, but nowhere a planner would find it.
- **wasm:** all five files are committed and all five tests currently pass. The defect is that `.github/workflows/ci.yml` never rebuilds them, so an SDK change validates against a stale binary — a false pass, strictly worse than a skip. Fix the *rebuild* first (build each plugin in CI, compare a hash against the committed file), *then* promote the skips. Note the cost: `security.yml:38–41` records `go test -race ./internal/plugin/` alone at **297 s**.
**Also:** `docs/offene-punkte.md` and `.planning/ROADMAP.md` both say migrations run to `00044`; they run to `00045`. `.planning/ROADMAP.md` and `REQUIREMENTS.md` still describe v1.5 / phases 6–8 while `PROJECT.md` is on v1.6 / phases 6–10. `.planning/codebase/ARCHITECTURE.md` has six drifted facts listed in ARCHITECTURE.md §0.
**Avoids:** Pitfall 25 (the 4 590-line reformat), Pitfall 26 (stale wasm false pass).

### Phase 7: Field Kinds
**Rationale:** Carries the multi-value encoding decision that Phase 9 depends on. Needs **no migration** — `art` has no CHECK.
**Delivers:** Button row as a `darstellung` setting on `KindChoice`; `mehrfachauswahl`; `KindTerm`; `zeit`, `bereich`, `code`.
**Build order (dependencies are real):** ① multi-value encoding (`SplitValues`/`JoinValues`, `page_form.go:150`/`:163`, `Entry.Values`, `Resolve`/`List`/`Filled`) — settle this first, everything and all of Phase 9 depend on it; ② `zeit`, `bereich`, `code` (an hour each, and `bereich` carries the `MayControl() == false` fix); ③ `mehrfachauswahl`; ④ button row + the `switchOf`/CSS change, last so the conditional-field rule is touched once; ⑤ `KindTerm` (copies `KindRef` wholesale, independent, parallelizable); ⑥ fixtures + `TEMPLATE-SPEC.md` + i18n + browser pass.
**Budget the per-kind tax once and remember it:** `Kinds`, `SubKinds()`/`BlockKinds()` (both *subtractive* — a new kind is in unless excluded), `field_input.html`, `CheckAll`, `Resolve`/`Entry`, `TEMPLATE-SPEC.md` + `SampleData` + `MinimalData` (tests tie all three), `tools/i18n -write` then `-schweiz`, and the bundle/AI round trip. Roughly as much again as the kind itself.
**Avoids:** Pitfalls 13, 14, 17, 18, 19, 20.

### Phase 8: Snippets Carry Fields
**Rationale:** Third instance of the `00038` pattern. Wants the kind palette complete first so the snippet form is built once, not twice.
**Delivers:** Migration `00046_snippet_fields.sql` (`ALTER TABLE page_field_defs ADD COLUMN snippet_id` — must default to NULL, SQLite refuses `ADD COLUMN` with `REFERENCES` and any other default — plus the index swap and `ALTER TABLE snippets ADD COLUMN fields`); namespace-aware `field.Store`; a fourth mode of `fieldListData`; `Site.Bausteinfelder` / `Bausteinliste`; bundle round-trip.
**Correct the stale note:** the index to swap, `idx_page_field_defs_kennung_oben`, was **already replaced by `00038:52–56`**, not left as `00029` wrote it. `00038:44–46` says so in the file: *"ein Austausch von zwei Indizes und kein Tabellenneubau."* Read `00029` and `00038` before writing `00046`.
**Build order:** ① migration alone (rollback is one file) → ② `field/store.go:53` **first**, then `Def.SnippetID`, `scanDef`, `Create`, `Update`, `Move`, `validate` — this step must be invisible to pages and blocks → ③ `snippet` store carries `fields` → ④ the admin screen (half the phase's effort) → ⑤ theme surface + fixtures + spec → ⑥ bundle round-trip.
**Avoids:** Pitfalls 15, 16, 24, 27 (`MaxFields = 60` is one budget shared by three — soon four — carriers, counted as `SELECT COUNT(*) … WHERE website_id = $1` at `store.go:217`).

### Phase 9: CSV Import
**Rationale:** Needs Phase 7's encoding and Phase 7's term field. Needs nothing from Phase 8.
**Delivers:** `internal/csv` (a genuinely streaming parser, unlike `wxr.Parse`); a four-screen flow (Datei → Zuordnung → Probelauf → Bericht); two routes registered in `newRouter` and added to `TestRouteAuthorization`'s table (`main_test.go:158–173`); a third `<details>` panel on `website_list.html` copied from `:24–45`.
**Posture:** strict at the gate, best-effort at the till — validate every row before writing anything, then a "trotzdem importieren" escape hatch, then **one transaction per row, never one per file** (the write pool is `SetMaxOpenConns(1)`; a file-long transaction blocks every request on the box, admin and public alike).
**Carrying the file between steps:** do **not** put the parsed table in the session (SCS stores sessions in SQLite; a 2 MB CSV in a session row is a bad day). Re-submit the file with the mapping — simpler, and needs no cleanup job.
**Deliberate departure from a house rule:** `internal/admin/wordpress.go` states that an importer always creates a *new* website. CSV must break that — rows into an existing content model is the point. The rule's stated reason (every collision needs an answer) is *answered* here by screen 1's update-or-skip choice plus the dry run. Write it into the plan as a decision, or a future reader sees an inconsistency.
**Avoids:** Pitfalls 10, 11, 12, 21, 22, 23.

### Phase 10: Authentik Forward-Auth SSO
**Rationale:** Independent of 7–9; touches only `auth/`, `web/`, `config/`, `main.go`. Movable earlier if a second worker is available. Highest stakes in the milestone by a wide margin.
**Delivers:** `internal/auth/forwardauth.go`; a fourth `completeLogin` caller; `web.AdminCSP` + `web.AdminHeadersWith`; `HOLZCLOUD_FORWARD_AUTH_*` config; `HOLZCLOUD_BIND`; a corrected `deploy/Caddyfile.example` + a DEPLOY.md SSO section; a test proving an untrusted peer's header is ignored.
**Build order (each step independently shippable and reversible):** ① config + a no-op middleware wired at `main.go:968` while it does nothing → ② **peer-gated header read — the security core, before anything can rely on it**; test first that an untrusted peer's header is ignored → ③ look up an existing user by e-mail, `RenewToken`, then `completeLogin` (no provisioning yet) → ④ provisioning (random Argon2id hash into `users.password`; no migration needed, `00001:6` has no CHECK) → ⑤ group → rights sync, re-applied every sign-in (`SetRights` replaces wholesale, so it is idempotent) → ⑥ TOTP policy, in `MustHaveSecondFactor` alone → ⑦ sign-out + the CSP pair (parallelizable from step 2) → ⑧ deployment docs + a browser pass **including a run with forward-auth disabled** to prove the password path is untouched.
**Deliberately unchanged:** `auth.RequireAuth`, `RequireAdmin`, `RequireWebsiteAccess`, `RequireSecondFactor`, `completeLogin`'s body, `user.Rights`/`SetRights`, the `users` schema. If any of these need changing, the design is wrong. **Do not invent a third role** — `users.role` carries a table-level `CHECK (role IN ('admin','editor'))` at `00001:7`, and loosening a table-head CHECK in SQLite means a full rebuild of a table with foreign-key children.
**Acceptance test for the whole phase, one command:** from a second machine, over IPv4 *and* IPv6, `curl -H 'X-authentik-email: …' http://<server>:8080/admin/` must return a login form, not a dashboard.

### Phase Ordering Rationale

- **6 → 7 → (8 ∥ 9) → 10.** Phase 6 makes the translation gate meaningful and corrects the notes the later phases are planned against. Phase 7 owns the encoding decision; if 7 ships it and 9 slips, nothing breaks — if 9 ships first, it invents an encoding 7 then inherits. **7 before 9, always.**
- **7 before 9 for a second reason:** a CSV with a `Sorte` column is one of the two motivating examples in `offene-punkte.md`. Without `KindTerm`, Phase 9 either omits it or builds a throwaway path.
- **7 before 8** so the snippet form is written once against a complete kind palette rather than revisited.
- **Inside Phase 7, the button row precedes `bereich`'s best answer** — for ≤ 11 stops a radio row is the honest no-JS solution to the invisible-slider problem, and the renderer already exists once the button row lands.
- **Phase 10 is disjoint** and can move; nothing in 6–9 reads or writes it.

### Research Flags

Phases likely needing `/gsd-plan-phase --research-phase` or `/gsd-discuss-phase`:
- **Phase 7 — `bereich` only.** "The value must be visible without JS" has three candidate answers and the choice is a UI-design question, not a technical one. Worth a UI-SPEC.
- **Phase 9 — the dry-run report screen.** 312 rows with 40 problems is a real information-design problem and the one screen users will judge the feature by.
- **Phase 10 — the whole phase.** Not for lack of research (the contract is verified from authentik's source) but because every open question below lands here and each is a policy decision, not a lookup.
- **Deferred, if ever promoted:** per-locale snippet values (FEATURES 4.10) — interacts with the star-shaped locale model in ways not obvious from outside.

Phases with standard patterns (skip research):
- **Phase 6** — housekeeping; the two live questions are already answered above.
- **Phase 7 — button row, multiple choice, `zeit`, `code`, `KindTerm`.** The neighbours agree, the encoding is decided, native controls do the work, Go's escaping is already correct by default, and `KindRef` is a complete end-to-end template (chooser at `page_fields.go:98–255`, resolution at `render.go:36–60` + `pagedata.go:84–114`).
- **Phase 8** — migration `00038` is a line-for-line template and its own comment explains the operation.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | **HIGH** | Load-bearing claims were *executed*, not read: the stdlib catalogue writer reproduces all seven committed files byte for byte (7/7 `identical=true`); `encoding/json` ordering, escaping and the `SetIndent("","")` no-op verified on Go 1.26.6; `encoding/csv` BOM / `ErrFieldCount` / LazyQuotes / NUL / blank-line behaviour verified against crafted inputs. Authentik headers read from primary source at two release tags. |
| Features | **HIGH / MEDIUM** | HIGH for the ecosystem survey (primary vendor docs for Statamic, Craft, ACF, Kirby, Directus, Feed Me, WP All Import, Statamic Importer); MEDIUM for the recommendations, which combine those with a read of `internal/field`, `internal/term`, `internal/snippet`, `wordpress.go` and migrations 00028/00029/00038/00045. |
| Architecture | **HIGH** | Every claim read out of the working tree at `c58ceb0`; the two that could only be settled by running something (the i18n writer's output, the wasm skips) were settled by running it. |
| Pitfalls | **HIGH / MEDIUM** | HIGH for everything grounded in this repository's own code (file:line throughout); MEDIUM for the Caddy/authentik advisories, read from the vendor advisory pages themselves. |

**Overall confidence:** HIGH

### Where the researchers agree (raises confidence)

- **Multi-value encoding** — STACK, FEATURES and ARCHITECTURE independently recommend one value per line in the existing string slot, and all three give the same primary reason: the delimiter is already illegal inside a value because `SplitChoices` reads options one per line.
- **The i18n premise is stale** — STACK proved it by byte-level round-trip of all seven catalogues; ARCHITECTURE proved it by running `-write` and `-schweiz` and getting an empty `git diff`. Two independent methods, same answer.
- **The wasm files are committed and the five tests currently pass** — STACK (`git ls-files`), ARCHITECTURE (captured `go test -v` PASS output) and PITFALLS all state it.
- **Phase 7 needs no migration** — STACK and ARCHITECTURE both cite `00028:25–27` and `00029:32`.
- **Do not verify the JWT** — STACK and PITFALLS reach the same recommendation by different routes (dependency + outbound-call cost vs. defence-in-depth accounting), and both add: do not implement a half version.
- **Gate on `IsTrustedPeer` first** — ARCHITECTURE and PITFALLS both name `web.RequestID` (`logging.go:31–45`) as the exact shape to copy.

### Where the researchers differ (the requirements step must choose)

1. **The i18n writer: replace it, or just fix the comment?** STACK recommends replacing the hand-rolled writer with an eleven-line stdlib equivalent plus a round-trip test that locks the format (so drift cannot return through a hand-translation pass). ARCHITECTURE recommends option (a): fix the doc comment only, change nothing else. Both agree the *catalogue files* must not change. The choice is whether Phase 6 buys the regression test. **Recommendation: take STACK's version** — the test is ~20 lines and it is the only mechanism that keeps the three regional files and the four full ones in one shape.
2. **The wasm skips: `t.Fatalf` outright, or rebuild-in-CI first?** STACK recommends promoting all five `t.Skipf` to `t.Fatalf` (with an opt-*out* env var). PITFALLS argues the promotion "changes almost nothing about coverage while introducing a new way for a contributor with a partial checkout to see a red suite they cannot fix", and that the *rebuild* is the change that closes the hole. ARCHITECTURE proposes a `plugins/build.sh` + CI step + `HOLZCLOUD_TEST_REQUIRE_WASM=1`. **Recommendation: PITFALLS' ordering — rebuild-and-hash-compare in CI first, then promote the skips, guarded so a local contributor is not blocked, with a message that says what to do.**
3. **`bereich` bounds: where do `min`/`max`/`step` live?** ARCHITECTURE proposes reusing the existing `auswahl TEXT` column via `SplitChoices` (zero schema change). FEATURES treats them as properties on the def. Both avoid a migration; **pick one and write it in TEMPLATE-SPEC**, because a theme author reads that document literally.
4. **`darstellung` / `max_werte`:** FEATURES calls for a new `page_field_defs` column (`ALTER TABLE ADD COLUMN`, cheap, and explicitly *no* CHECK per `00028`); ARCHITECTURE's Phase 7 table says "NO MIGRATION". These are reconcilable — a display setting could also ride in `auswahl` — but the requirements step must state which, because it determines whether Phase 7 ships a migration at all.

### Gaps to Address

Open questions that belong to the user or to `/gsd-discuss-phase`, gathered here so none is discovered mid-execution:

- **Must an admin arriving via Authentik still pass local TOTP?** The decision has exactly one home: **`auth.MustHaveSecondFactor` at `internal/auth/twofactor.go:44`** (`return role == "admin"` — one function, one caller at `:70`). Change it there or nowhere; a likely shape is `MustHaveSecondFactor(role string, viaSSO bool)` with a session key recording how the session was established. As it stands an SSO administrator lands on `/admin/2fa/einrichten` forever, because setting up TOTP for an account whose password nobody knows is at best confusing. STACK recommends treating an Authentik session as satisfying the second factor **only** when the operator has said so explicitly, default off. **User decision.**
- **Does CSV import target an existing website, or create a new one?** FEATURES recommends an existing one and names it a deliberate departure from `wordpress.go`'s "always a new website" rule; the rule's own reason is answered by the update/skip choice and the dry run. **Confirm with the user** — it is the difference between two different features.
- **Auto-provisioning: on or off, and with what default website?** Directly tied to `internal/admin/handler.go:173`. Research recommends off by default, with a start-time refusal if enabled without an explicit default website.
- **Does an SSO user with no matching website group get *all* websites or *none*?** Empty currently means "all" (`00033_user_rights.sql:21–23`: a migration that silently takes access away is one after which nobody can work on Monday). Must become an explicit, documented setting.
- **`MaxFields = 60` across four carriers** — scope the count per namespace, or keep one shared budget? Decide in Phase 8; the count is at `internal/field/store.go:217`.
- **`X-authentik-uid` format depends on the provider's Subject mode** (default: a hashed identifier). MEDIUM confidence — **verify in the operator's own instance before pinning the mapping to it**, or pin to username instead. Document that subject mode must not change after users are mapped.
- **The operator's installed Caddy version.** The advisory names v2.10.0–v2.11.1 affected, v2.11.2 fixed. Check it, and state the minimum in `DEPLOY.md`.
- **A `code` field inside a block kind** — blocks freeze to HTML on save, so the escaping must happen in the block render path, not in the theme. Easy to miss; confirm it is in the Phase 7 plan.
- **`internal/admin` is at 14.7 % coverage and is where authorisation lives.** Phases 8 and 10 both add handlers there. Reuse `internal/admin/page_handler_test.go` as the harness; do not invent a second.
- **A browser pass is a deliverable, not a courtesy** — the defects this project shipped were found in the browser, not by the test suite. Per-phase clicks are enumerated in PITFALLS §"Cross-cutting".

## Sources

### Primary (HIGH confidence)
- **The working tree at `c58ceb0`** — read directly at every cited line, plus commands run against it (`go run ./tools/i18n [-write|-schweiz]`, `git diff`, `go test -v -run …`, `git ls-files | grep wasm`, `grep -c '^  "'` per catalogue).
- **Local execution on Go 1.26.6** — all `encoding/json` and `encoding/csv` behaviour claims; round-trip against `internal/i18n/locales/*.json`.
- **[goauthentik/authentik](https://github.com/goauthentik/authentik)** at tags `version-2026.8` and `version-2025.8` — `internal/outpost/proxyv2/application/{mode_common,mode_forward,application,oauth,claims}.go`: header names, the `|` separator, `X-authentik-jwt` = raw token, `/outpost.goauthentik.io/sign_out`.
- **Vendor documentation** — Statamic 6 fieldtypes and Globals; Craft 5 Checkboxes/Time/Globals; Craft Feed Me field mapping; ACF Checkbox/Button Group/Taxonomy; Kirby checkboxes/tags; Directus Import & Export; WP All Import; Statamic Importer DOCUMENTATION.md.
- **`.planning/WINDOWS.md`** — the 2026-09-03 deviation record for quick task `260903-bsk` settling the catalogue format.

### Secondary (MEDIUM confidence)
- [Caddy GHSA-7r4p-vjf4-gxv4 / CVE-2026-30851](https://github.com/caddyserver/caddy/security/advisories/GHSA-7r4p-vjf4-gxv4) — `forward_auth copy_headers` emits no delete for the client's inbound header.
- [Caddy GHSA-f59h-q822-g45g / CVE-2026-52845](https://github.com/caddyserver/caddy/security/advisories/GHSA-f59h-q822-g45g) — the underscore-alias variant.
- [authentik GHSA-7jxf-mmg9-9hg7](https://github.com/goauthentik/authentik/security/advisories/GHSA-7jxf-mmg9-9hg7) — password auth bypass via `X-Forwarded-For`.
- authentik docs — [Proxy provider](https://docs.goauthentik.io/add-secure-apps/providers/proxy/), [Forward auth](https://docs.goauthentik.io/add-secure-apps/providers/proxy/forward_auth), [Caddy configuration](https://docs.goauthentik.io/add-secure-apps/providers/proxy/server_caddy/), [OAuth2 signing keys](https://docs.goauthentik.io/add-secure-apps/providers/oauth2/).
- [Caddy `forward_auth` directive](https://caddyserver.com/docs/caddyfile/directives/forward_auth) — the expanded form.

### Tertiary (needs validation at implementation time)
- **authentik subject-mode default and signing-key behaviour** — the docs page is partially incomplete. Verify `X-authentik-uid`'s format in the operator's own instance before relying on it.
- **`adminCSP`'s `form-action 'self'` breaking an external POST-logout redirect** — MEDIUM-HIGH. The mechanism is documented in this codebase from a real Safari incident (`internal/web/headers.go:41–53`) but was not re-tested for the admin origin.

---
*Research completed: 2026-09-03*
*Ready for roadmap: yes*
