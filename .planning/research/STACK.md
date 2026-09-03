# Stack Research — Milestone v1.6 "Inhaltsmodell und Zugang"

**Domain:** self-hosted Go CMS, single binary, stdlib-first
**Researched:** 2026-09-03
**Confidence:** HIGH (see [Evidence and confidence](#evidence-and-confidence) — the load-bearing
claims were verified by running the project's own toolchain against the project's own files, and
by reading authentik's source at its current released tag)

---

## Verdict

**Nothing new is needed. Zero additions to `go.mod` for all five phases.**

Your prior is right in every case, and it is right for a stronger reason than "the stdlib is
adequate": in each of the three places where a library is the usual reflex, the stdlib call is
also the *shorter* one. The i18n writer becomes eleven lines that reproduce the committed files
byte for byte. The CSV reader is `encoding/csv` with four fields set. The Authentik integration
is four `r.Header.Get` calls, because the cryptography happens in the outpost — that is the whole
point of choosing forward-auth over an OIDC client.

| Phase | Needs | Package | New dependency |
|-------|-------|---------|----------------|
| 1 Aufräumen | catalogue writer, test gate, stale notes | `encoding/json`, `testing`, none | **No** |
| 2 Field Kinds | radio row, multi-value, terms, `zeit`/`bereich`/`code` | `net/http`, `strings`, `time`, `strconv`, `html/template` | **No** |
| 3 Snippets Carry Fields | fields on snippets | existing `internal/field`, `pressly/goose` migration | **No** |
| 4 CSV Import | upload, map, create, report | `encoding/csv`, `unicode/utf8`, `bytes`, `io` | **No** |
| 5 Authentik Forward-Auth | trust identity from the proxy | `net/http`, `crypto/subtle`, existing `internal/web.ClientIPResolver` | **No** |

No CSP change, no new asset, no new JavaScript, no new font, no build step. htmx stays at the
vendored **2.0.10** (`cmd/holzcloud/assets/htmx.min.js`) and is not touched: every control this
milestone adds is a plain HTML form control.

---

## Recommended Stack

### Core (unchanged, listed so the planner has the verified versions)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go toolchain | **1.26.6** (`go.mod`, verified `go version` locally) | everything | Mandate. Every capability below is in its stdlib. |
| `modernc.org/sqlite` | 1.57.0 | storage | Mandate, `CGO_ENABLED=0`. |
| `pressly/goose/v3` | 3.27.3 | migrations | Phase 3 needs one new file. |
| htmx | 2.0.10 (vendored) | admin interactivity | Enhancement only; every new control degrades to a plain form. |

### Standard-library packages this milestone reaches for

| Package | Phase | Exact use |
|---------|-------|-----------|
| `encoding/json` | 1 | `Encoder.SetEscapeHTML(false)` + `json.Indent` — the catalogue writer |
| `testing` | 1 | `t.Fatalf` instead of `t.Skipf` on a tracked build artefact |
| `net/http` | 2, 4, 5 | `r.PostForm["k"]` (multi-value), `http.MaxBytesReader`, `r.Header.Get` |
| `strings` | 2 | `Split`/`Join` for the multi-value encoding, reusing `SplitChoices`' rule |
| `time` | 2 | `time.Parse("15:04", v)` for `zeit` |
| `strconv` | 2 | `ParseFloat`/`Atoi` + clamp for `bereich` |
| `html/template` | 2 | `code` renders *without* a `template.HTML` cast — that is the whole feature |
| `encoding/csv` | 4 | the reader; `ParseError`, `FieldPos`, `InputOffset` for the per-row report |
| `unicode/utf8`, `bytes`, `io` | 4 | BOM strip, UTF-8 validation, NUL check, bounded read |
| `crypto/subtle` | 5 | `ConstantTimeCompare` for the proxy shared secret |
| `net/netip` | 5 | already used by `internal/web.ClientIPResolver` |

### Migration numbering — correct a stale note before planning

`internal/db/migrations/` already contains **`00045_pages_locale_unique.sql`**. Both
`docs/offene-punkte.md` ("Migrationen laufen bis `00044`") and `.planning/ROADMAP.md` ("migration
`00045`" for the snippet column) are one behind. **The snippet migration in Phase 3 is `00046`.**
This belongs in Phase 1's "stale planning notes" work.

---

## Phase 1 — Aufräumen

### Finding: the catalogue format mismatch is already resolved at HEAD

Verified by reading the working tree, not by trusting a note:

- All seven files in `internal/i18n/locales/` are **flush-left** (`grep -c '^  "'` = 0 in every
  one). None carries a two-space indent.
- `.planning/WINDOWS.md` records the deviation and its resolution on 2026-09-03 (quick task
  `260903-bsk`): "tool's flush-left format is canonical … the two-space indent in the four full
  catalogues was drift from a hand-translation pass", `-write` reformatted them, waived.
- **Round-trip proof:** a stdlib writer (below) fed the parsed content of each committed file
  reproduces it **byte for byte, all seven**:

  ```
  de-CH.json identical=true 12471   en.json identical=true 96517
  es.json    identical=true 101265  fr-CH.json identical=true 1089
  fr.json    identical=true 103465  it-CH.json identical=true 2595
  it.json    identical=true 100848
  ```

So Phase 1's i18n item is **not** "make the writer match the files". It is: **replace the
hand-rolled writer with the stdlib equivalent and lock the format with a test**, so the drift
cannot come back through a hand-translation pass. If the operator instead *wants* the two-space
indent, that is one line — but it reformats ~2 250 lines in each of the four full catalogues once,
and it is a taste decision, not a defect.

### The writer, in stdlib, verified byte-identical

```go
// writeCatalog writes the file sorted and flush left, one key per line, so a
// change to one string is one line in a diff.
func writeCatalog(path string, catalog map[string]string) error {
	var compact bytes.Buffer
	enc := json.NewEncoder(&compact)
	enc.SetEscapeHTML(false) // keeps <code> and & readable for a translator
	if err := enc.Encode(catalog); err != nil {
		return err
	}
	var out bytes.Buffer
	// prefix "" + indent "" == one element per line, no indentation.
	if err := json.Indent(&out, bytes.TrimRight(compact.Bytes(), "\n"), "", ""); err != nil {
		return err
	}
	out.WriteByte('\n')
	return os.WriteFile(path, out.Bytes(), 0o644)
}
```

This deletes `quote()` and the manual comma/newline bookkeeping. It keeps every property the
current comment claims: sorted, one line per key, no HTML escaping, trailing newline.

**If two-space indent is chosen instead**, the whole writer is:

```go
enc := json.NewEncoder(f)
enc.SetEscapeHTML(false)
enc.SetIndent("", "  ")
return enc.Encode(catalog)
```

### `encoding/json` facts a planner must not guess (all verified locally on Go 1.26.6)

| Behaviour | Verified result |
|-----------|-----------------|
| **Map key order** | Sorted, byte-wise on the UTF-8 key. Identical to `sort.Strings`. Observed: `"10" < "2" < "Z" < "_under" < "a" < "b" < "Ä"`. The current writer's `sort.Strings` and the encoder agree — that is why the round trip is byte-identical. |
| `json.MarshalIndent` | **Always HTML-escapes** (`<` → `<`, `&` → `&`). There is no switch. This is exactly why the hand-rolled `quote()` exists — do not "simplify" to `MarshalIndent`. |
| `Encoder.SetEscapeHTML(false)` | The only way to turn escaping off. Composes with `SetIndent`. |
| `Encoder.SetIndent("", "")` | **A no-op** — the encoder treats empty prefix *and* empty indent as "not indenting" and emits one compact line. Flush-left-with-newlines requires the free function `json.Indent`. This is the single trap in the whole phase. |
| `Encoder.Encode` | Always appends `"\n"`. `json.Indent` elides trailing whitespace from its source — hence the `TrimRight` + explicit `WriteByte('\n')` above. |
| U+2028 / U+2029 | Escaped **even with** `SetEscapeHTML(false)`. Harmless here; do not be surprised by it in a diff. |
| Invalid UTF-8 | Silently replaced with U+FFFD on marshal. Lossy — a reason the round-trip test earns its place. |
| Duplicate keys on unmarshal | Last one wins, silently. A hand-edited catalogue with a duplicated German sentence loses one on the next `-write`. The round-trip test catches this too. |

### The test that stops the drift

A table test over `internal/i18n/locales/*.json`: read the file, `json.Unmarshal`, `writeCatalog`
to a `t.TempDir()` copy, compare bytes with the committed file. Failure message: "run
`go run ./tools/i18n -write`". It costs ~20 lines and it is the only mechanism that keeps the
three regional files and the four full ones in one shape.

### `encoding/json/v2` — do not use

Verified locally: `go doc encoding/json/v2` fails on this toolchain and only resolves under
`GOEXPERIMENT=jsonv2`, where its own package doc says: *"This package (encoding/json/v2) is
experimental, and not subject to the Go 1 compatibility promise… Most users should use
encoding/json."* Building the release binary with a GOEXPERIMENT flag would put an experimental
encoder between the operator and their translations. **No.**

### The silently self-skipping test suite

The five `plugin.wasm` files are **tracked in git** (`git ls-files 'plugins/*/plugin.wasm'` returns
all five). Therefore a missing `plugins/kontaktformular/plugin.wasm` is not "not built yet" — it is
a broken checkout. The correct stdlib fix is one word:

```go
modul, err := os.ReadFile("../../plugins/kontaktformular/plugin.wasm")
if err != nil {
	t.Fatalf("plugins/kontaktformular/plugin.wasm fehlt: %v", err) // im Repo verfolgt
}
```

Five call sites share the pattern and should be decided together, so the rule is one rule:

| File:line | Artefact | Tracked? | Recommendation |
|-----------|----------|----------|----------------|
| `internal/public/formular_e2e_test.go:160` | `plugins/kontaktformular/plugin.wasm` | yes | `t.Fatalf` |
| `internal/public/suche_e2e_test.go:28` | `plugins/suche/plugin.wasm` | yes | `t.Fatalf` |
| `internal/plugin/sdk_e2e_test.go:23` | `plugins/jahreszahl/plugin.wasm` | yes | `t.Fatalf` |
| `internal/plugin/hofladen_e2e_test.go:25` | `plugins/bestellung/plugin.wasm` | yes | `t.Fatalf` |
| `internal/plugin/runtime_test.go:26` | `testdata/echo.wasm` | check separately | `t.Fatalf` if tracked |

There is no `go test` flag that turns a skip into a failure — the guard has to be in the test.
If a skip path must survive for a deliberately artefact-free tree, make it opt-**out** and loud:
`if os.Getenv("HOLZCLOUD_OHNE_WASM") == "1" { t.Skip(...) }`, default fail. Do not use
`testing.Short()`: `-short` is not set in this project's CI invocation, so it would restore the
silence it is meant to remove.

---

## Phase 2 — Field Kinds

Nothing new, and no migration: `page_field_defs.art` is `TEXT NOT NULL` with no CHECK (migration
`00029`), so a kind is a Go-side change.

| Kind | HTML control (no JS) | Server-side |
|------|----------------------|-------------|
| Choice as button row | `<input type=radio name=f_x>` per option, CSS-styled as a row | unchanged: one string |
| Multiple choice | `<input type=checkbox name=f_x value=…>` (repeatable) or `<select multiple>` | `r.PostForm["f_x"]` is already `[]string` — `net/http` does the parsing |
| Terms | `<select>` fed by `internal/term`, mirroring `refPages` in `internal/admin/page_fields.go` | resolve by name at render, like `Links.Page` |
| `zeit` | `<input type=time>` — submits `HH:MM` (or `HH:MM:SS` with a step) | `time.Parse("15:04", v)`, fall back to `"15:04:05"` |
| `bereich` | `<input type=range min max step>` | `strconv.ParseFloat` + clamp to the configured bounds server-side; a range input can be forged like any other |
| `code` | `<textarea>` rendered into `<pre><code>` | **no goldmark, no bluemonday, no `template.HTML` cast** — `html/template`'s own escaping is the feature. Casting would be the bug. |

**On the multi-value encoding (the one real decision).** `field.Data` is already persisted as JSON
(`encoding/json`, `Values map[string]string` + `Rows map[string][]Values`). Two stdlib-only options,
no dependency either way:

1. **Newline inside the existing string** — as `docs/offene-punkte.md` proposes, reusing
   `SplitChoices`/`JoinChoices`. Safe because an option *cannot* contain a newline: `SplitChoices`
   splits the admin's option list on newlines, so no configured value ever holds one. Every existing
   consumer (theme render, bundle export, WXR, the CSV importer of Phase 4, the WASM plugin SDK)
   keeps reading a `string` — one storage path, one encoding, no fan-out.
2. **A third map in `Data`** (`Mehrfach map[string][]string \`json:"mehrfach,omitempty"\``).
   Additive and back-compatible on read (an older binary ignores the unknown key — and silently
   drops it on the next save, which is the argument against it).

Recommendation: **(1)**, with the split/join pair exported from `internal/field` so exactly one
function knows the rule, and with a template helper so a theme gets a `[]string` to range over
rather than doing the split itself. Phase 4's importer then writes the same joined string.

---

## Phase 3 — Snippets Carry Fields

Nothing new. One goose migration — **`00046`**, not `00045` (see above) — adding `snippet_id` to
`page_field_defs` (`ALTER TABLE … ADD COLUMN`, the shape `00037` used) plus the index swap the
roadmap already identifies (`idx_page_field_defs_kennung_oben`). Rendering reuses the existing
goldmark → bluemonday chain; adding a second one is the thing to refuse.

---

## Phase 4 — CSV Import

`encoding/csv` and nothing else. **Do not add `gocarina/gocsv`, `jszwec/csvutil`, or a
delimiter-sniffing library** — struct-tag mapping is worthless here because the mapping is chosen
by the admin at runtime, which is the entire job.

### Recommended reader configuration

```go
r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // same posture as internal/admin/wordpress.go
file, _, err := r.FormFile("csv")
data, err := io.ReadAll(file)                    // bounded by the line above

data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // Excel "CSV UTF-8" writes a BOM
if !utf8.Valid(data) { /* "Bitte als CSV UTF-8 exportieren." */ }
if bytes.IndexByte(data, 0) >= 0 { /* not text — refuse */ }

cr := csv.NewReader(bytes.NewReader(data))
cr.Comma = comma          // sniffed, see below
cr.LazyQuotes = true      // an upload is not RFC 4180, it is whatever Excel produced
cr.FieldsPerRecord = -1   // own the length check, so a short row is a *reported row*
cr.TrimLeadingSpace = false // keep; trim the mapped value instead
```

### Behaviours verified by running them (Go 1.26.6)

| Behaviour | Verified result | Consequence |
|-----------|-----------------|-------------|
| **BOM** | **Not** stripped. The first header cell arrives as `"﻿Titel"`. | Strip the three bytes before parsing, or the first column never matches a mapping and the admin sees an unexplained "Titel" that is not "Titel". This is the number-one CSV import bug. |
| **`FieldsPerRecord = 0`** (default) | Locks to the header's field count; a short row returns `*csv.ParseError` with `Err` = `csv.ErrFieldCount`, and **`Read` is resumable** — the next call returns the following record. | Usable, but the error carries a *line* number, not a row number. With `-1` you get the row and can say "Zeile 7: 3 Spalten statt 5" in the report. Prefer `-1`. |
| **`LazyQuotes = false`** | `Hans "Peter",2` → `ParseError{Err: ErrBareQuote}`. A stray quote can swallow the rest of the file into one field. | Set `LazyQuotes = true` for untrusted uploads. With it, the same input parses as `Hans "Peter"`. |
| **Separator** | Not sniffed by the package. `cr.Comma = ';'` parses Swiss/German Excel output correctly. | Sniff it (below). |
| **CRLF** | Converted to `\n` everywhere, including inside quoted fields. | No `\r` handling needed. |
| **Blank lines** | Skipped silently. | Row numbers in the report must come from a counter, not from the line. |
| **NUL byte** | Passes straight through into a field (`"\x001"`). | Check for it explicitly; it is the cheapest "is this actually text" test after `utf8.Valid`. |
| **Quoted newline** | Supported: one record can span many lines. | **Row count ≠ line count.** Cap rows, and use `cr.FieldPos(0)` / `cr.InputOffset()` when you need the position in the file. |

### Separator detection (Swiss/German Excel)

Two rules, in this order:

1. **`sep=` line.** Excel writes and honours a first line of the form `sep=;`. If the first line
   matches `^sep=(.)\r?$`, take that rune as `Comma` and skip the line before handing the rest to
   the reader.
2. **Sniff the header line.** Count `;`, `,` and `\t` occurrences outside quotes in the first
   record; pick the highest; tie or zero → `,`. A German/Swiss Excel export uses `;` because the
   locale's decimal separator is the comma — this is the common case here, not the exotic one.

Show the detected separator on the mapping screen with a way to override it. That is honest and
it costs one `<select>`.

### Character encoding

Excel on Windows writes the ANSI code page (Windows-1252) unless the user picks "CSV UTF-8".
Recommendation: **refuse non-UTF-8 with a sentence that says what to do** ("Die Datei ist nicht
UTF-8. In Excel: Speichern unter → CSV UTF-8."). It is one `utf8.Valid` call and it never guesses
wrong. If transcoding is ever wanted, `golang.org/x/text/encoding/charmap` is already in the module
graph (indirect, `golang.org/x/text v0.41.0`) so it downloads nothing new — but it promotes an
indirect dependency to a direct one and it makes the importer guess at an encoding. Not for v1.6.

### Bounding an untrusted upload

Four limits, all stdlib, matching the posture the template and media uploads already take:

1. `http.MaxBytesReader` at 10 MB (the WXR importer's number — a CSV of pages is text).
2. A **row cap** (suggest 5 000) counted on records, checked inside the `Read` loop, reported as a
   refusal rather than a truncation.
3. A **per-cell cap** (suggest 100 000 bytes) — `encoding/csv` imposes none, and one 9 MB cell is
   legal within the byte cap.
4. Stream with `cr.Read()` in a loop; do **not** use `ReadAll` (it materialises the whole table,
   several times the file size in Go strings). Leave `ReuseRecord` at its default `false` — it is a
   micro-optimisation whose aliasing bug costs more than it saves at this size.

### Reporting every row

`errors.Is(err, io.EOF)` ends the loop. `errors.As(err, &parseErr)` gives `Line`, `Column`,
`StartLine` and `Err`; `errors.Is(parseErr.Err, csv.ErrFieldCount)` distinguishes the length case.
Reading continues after a `ParseError`, so a file mixing good and bad rows imports the good ones —
which is exactly what success criterion 3 demands. Keep a row counter for the report and use
`FieldPos` only when pointing at the file itself.

---

## Phase 5 — Authentik Forward-Auth

**Nothing new, and this is the strongest "no" of the five.** Forward-auth was chosen precisely so
that the token handling lives in the outpost. Reading four request headers needs `net/http`.
An OIDC client would need `golang.org/x/oauth2` + `coreos/go-oidc` + a JOSE library, a client
secret in the config, and an **outbound HTTP call at runtime** to the discovery and JWKS endpoints
— the one rule this project is built on. `docs/offene-punkte.md` already lists OAuth under "was
bewusst nicht gebaut wird". Nothing here reopens that.

### The exact contract

Verified in authentik's own source at the current release tag **`version/2026.8.1`** (published
2026-09-01), file `internal/outpost/proxyv2/application/mode_common.go`, function `getHeaders`,
and cross-checked against the official docs. Identical code at `version-2025.8`, so the contract is
stable across at least a year of releases.

```go
headers["X-authentik-username"]      = c.PreferredUsername
headers["X-authentik-groups"]        = strings.Join(c.Groups, "|")
headers["X-authentik-entitlements"]  = strings.Join(c.Entitlements, "|")
headers["X-authentik-email"]         = c.Email
headers["X-authentik-name"]          = c.Name
headers["X-authentik-uid"]           = c.Sub
headers["X-authentik-jwt"]           = c.RawToken
headers["X-authentik-meta-jwks"]     = a.endpoint.JwksUri
headers["X-authentik-meta-outpost"]  = a.outpostName
headers["X-authentik-meta-provider"] = a.proxyConfig.Name
headers["X-authentik-meta-app"]      = a.proxyConfig.AssignedApplicationSlug
headers["X-authentik-meta-version"]  = constants.UserAgentOutpost()
```

| Header | Contents | Notes for the middleware |
|--------|----------|--------------------------|
| `X-authentik-username` | authentik username (`preferred_username`) | The natural key to match against `users.email`/a new `users.extern_kennung`. |
| `X-authentik-email` | e-mail | May be empty if the user has none in authentik. Do not assume it is present. |
| `X-authentik-name` | full display name | Nice for the profile, never for authorisation. |
| `X-authentik-uid` | the OIDC `sub` | **The stable key.** Its format depends on the provider's *Subject mode* (default: a hashed user identifier). Document that the operator must not change subject mode after users are mapped, or pin the mapping to username instead. |
| `X-authentik-groups` | group names joined by **`|`** (U+007C pipe) | `strings.Split(v, "|")`. Empty header → no groups; guard against `[""]`. A group name containing a pipe is unrepresentable — authentik does not escape. This is the exact separator, from the source, not from a blog. |
| `X-authentik-entitlements` | same `|` format | Application entitlements. Ignore unless roles are mapped from them. |
| `X-authentik-jwt` | the raw token string | See "verify or trust" below. |
| `X-authentik-meta-jwks` | the JWKS **URL** | Present so an app *could* verify. Fetching it is an outbound runtime call — forbidden here. |
| `X-authentik-meta-outpost` / `-provider` / `-app` / `-version` | names of the outpost, provider, application slug, outpost version | Useful in a log line; `-app` can pin "this outpost is the one I expect". |

Go canonicalises header names, so `r.Header.Get("X-authentik-username")` matches whatever case
Caddy set. **Always use `r.Header.Get`, never `r.Header["X-authentik-username"]`.**

In 2026.8 the outpost additionally runs `removeDuplicateUnderscoreHeader`: a header whose name
contains `_` is dropped if the dash-form of the same name is also present. It does not affect these
names; it matters only if the operator adds custom headers via the `additionalHeaders` user
attribute (that mechanism exists and lets an operator inject arbitrary header names — another
reason for the app to read only the fixed set above).

### Endpoints (source: `mode_forward.go`, `application.go`, tag `version-2026.8`)

| Path | Purpose |
|------|---------|
| `/outpost.goauthentik.io/auth/caddy` | the forward-auth check Caddy calls (`uri`) |
| `/outpost.goauthentik.io/auth/nginx`, `/auth/traefik`, `/auth/envoy` | the same for other proxies |
| `/outpost.goauthentik.io/start` | begins the login flow |
| `/outpost.goauthentik.io/callback` | OAuth redirect target |
| **`/outpost.goauthentik.io/sign_out`** | **sign-out** |

There is also a query-string signature: `?X-authentik-logout=true` on any forwarded URL triggers
sign-out, and `?X-authentik-auth-callback=true` the callback (constants `LogoutSignature`,
`CallbackSignature` in `oauth.go`). Use the documented `/outpost.goauthentik.io/sign_out` path;
the query form is the fallback for domain-level setups.

Because Caddy routes `/outpost.goauthentik.io/*` on the **application's own domain** to the outpost,
the sign-out link is **same-origin**. No CSP consequence at all — and it is an `<a href>`, which is
content, not a subresource, either way.

### Caddyfile (official docs, verbatim; verified against Caddy v2.11.4)

```caddyfile
app.company {
    route {
        # always forward outpost path to actual outpost
        reverse_proxy /outpost.goauthentik.io/* http://outpost.company:9000

        # forward authentication to outpost
        forward_auth http://outpost.company:9000 {
            uri /outpost.goauthentik.io/auth/caddy
            # capitalization of the headers is important, otherwise they will be empty
            copy_headers X-Authentik-Username X-Authentik-Groups X-Authentik-Entitlements X-Authentik-Email X-Authentik-Name X-Authentik-Uid X-Authentik-Jwt X-Authentik-Meta-Jwks X-Authentik-Meta-Outpost X-Authentik-Meta-Provider X-Authentik-Meta-App X-Authentik-Meta-Version

            trusted_proxies private_ranges
        }

        # actual site configuration below, for example
        reverse_proxy localhost:8080
    }
}
```

Order matters: the `reverse_proxy /outpost.goauthentik.io/*` line must come **before**
`forward_auth`, or the login flow cannot reach the outpost.

What `forward_auth` expands to (Caddy docs, "Expanded form"):

```caddyfile
reverse_proxy <upstreams...> {
	method GET                       # so the incoming body is not consumed
	rewrite <to>                     # the uri subdirective
	header_up X-Forwarded-Method {method}
	header_up X-Forwarded-Uri {uri}
	@good status 2xx
	handle_response @good {
		request_header X-Authentik-Username {rp.header.X-Authentik-Username}
		# … one line per copy_headers field
	}
}
```

Three consequences for the planner:

1. **2xx → allowed**, and the listed headers are **set (replacing)** on the original request via
   `request_header`. Anything **not** in `copy_headers` is *not* replaced — a client-supplied
   `X-Authentik-Groups` would survive if that name were omitted from the list. Read only headers
   that are in the list, and see the trust model below.
2. **Non-2xx → the outpost's response goes to the browser** (a 302 into the authentik flow). The
   CMS never renders a login page in this mode.
3. The outpost rebuilds the original URL from `X-Forwarded-Proto` + `X-Forwarded-Host` +
   `X-Forwarded-Uri` (`getTraefikForwardUrl`, shared by the Caddy and Traefik handlers). Caddy's
   `reverse_proxy` sets the first two, `forward_auth` adds the third. If `trusted_proxies` is wrong,
   the redirect-back-after-login breaks — that is the symptom to recognise.

### Verify the JWT, or trust the header?

`X-authentik-jwt` carries the raw token, and `X-authentik-meta-jwks` the JWKS URL, so verification
is *possible*. **Recommendation: do not verify. Trust the header, and make the trust boundary
explicit and testable instead.** Reasons, in order:

- Verifying means fetching the JWKS at runtime → an outbound call from the CMS → the one rule this
  project does not break. Pinning the key in config avoids the call but adds a key-rotation failure
  mode the operator will discover at the worst moment.
- Without a signing key configured, authentik signs proxy-provider tokens **symmetrically with the
  client secret**; with one, asymmetrically. So "verify" means supporting two modes and holding the
  client secret in the CMS — which is most of what an OIDC client costs, for none of its benefit.
- Verifying a JWT by hand (alg confusion, `exp`, `aud`, `iss`) is the classic place where
  hand-rolled crypto goes wrong, and a JOSE library is a new dependency.
- The signature protects nothing that the transport does not already protect: the binary listens on
  localhost behind Caddy. If an attacker can set request headers on that socket, they can talk to
  the CMS directly and the JWT changes nothing.

**Trust model to build instead — all stdlib, three layers:**

1. **Off by default.** A new `HOLZCLOUD_FORWARD_AUTH=1` (naming to match the existing env style).
   With it off, the headers are ignored entirely — no behaviour change for existing installs.
2. **Peer must be the proxy.** `internal/web.ClientIPResolver` already exists and already has
   `IsTrustedPeer(*http.Request)`, driven by `HOLZCLOUD_TRUSTED_PROXIES`
   (default `127.0.0.1/32,::1/128`, `internal/config/config.go`). Reject the headers on any request
   that did not arrive through a trusted peer. **This is the integration point — it is already
   built and already tested (`internal/web/clientip_test.go`).**
3. **A shared secret the proxy adds.** `header_up X-Holzcloud-Forward-Auth {env.HOLZCLOUD_FA_SECRET}`
   in the site block, compared with `crypto/subtle.ConstantTimeCompare`. This closes the residual
   hole in layer 2 (anything else on the host that can reach localhost) and the hole in
   `copy_headers` (a header the operator forgot to list). Twelve lines, no dependency.

Then: strip all `X-authentik-*` headers from the request before it reaches any handler when the
check fails, so a downstream handler cannot accidentally read a spoofed value.

### Integration points in this codebase

| Concern | Where | Note |
|---------|-------|------|
| Middleware placement | `internal/auth/middleware.go`, in the `Chain` **before** `RequireAuth` | It should establish a session, then let the existing `RequireAuth` → `RequireSecondFactor` → `RequireAdmin` → `RequireWebsiteAccess` chain run unchanged. |
| Session | `alexedwards/scs/v2`, `SessionKeyUserID` / `SessionKeyUserRole` | On first arrival: look the user up, **rotate the session token** (the codebase already does this on password login), put the user id in. Every later request then costs one session read, not a header parse. |
| User mapping | `internal/user` + a migration | Match on `X-authentik-uid` (stable), fall back to e-mail; decide explicitly whether an unknown user is **created** (JIT provisioning) or **refused**. Refusing by default fits this project's posture — the operator adds the account, Authentik proves who it is. |
| Roles | `X-authentik-groups` (`|`-split) → the existing `admin`/editor roles | Keep the mapping in config or a small table; do not let a group name silently promote someone. |
| Second factor | `auth.MustHaveSecondFactor(role)` forces TOTP on admins | **A real decision for the planner:** if Authentik already enforces MFA, requiring a second, CMS-local TOTP is friction; skipping it weakens a rule that currently holds unconditionally. Recommendation: treat an Authentik-authenticated session as satisfying the second factor **only** when the operator has said so with an explicit setting, default off. |
| Logout | `/admin/logout` (already exempt in `isSecondFactorPath`) | Destroy the local session, then redirect to `/outpost.goauthentik.io/sign_out` — same origin, so no CSP question. |
| CSRF | `gorilla/csrf` | Untouched. Forward-auth changes who you are, not how a form is submitted. |
| Password login | unchanged | Must keep working: it is the way back in when the outpost is down. |
| CSP | `internal/web/headers.go` | **No change.** Nothing new is fetched; the sign-out link is same-origin. |

---

## What NOT to Use

| Avoid | Why | Use instead |
|-------|-----|-------------|
| `coreos/go-oidc`, `golang.org/x/oauth2` | An OIDC client is a second mode of operation, a new dependency, a client secret in config, and an **outbound runtime call** to discovery/JWKS. Already rejected in `docs/offene-punkte.md` and in PROJECT.md's Out of Scope. | Forward-auth headers (Phase 5 above) |
| `golang-jwt/jwt`, `lestrrat-go/jwx`, `go-jose` | Only needed to verify `X-authentik-jwt`, which buys nothing behind a localhost listener and costs a JWKS fetch or a pinned key. | Trust boundary: `IsTrustedPeer` + shared-secret header + `crypto/subtle` |
| `gocarina/gocsv`, `jszwec/csvutil` | Struct-tag mapping, when the mapping is chosen by the admin at runtime. Solves a problem this feature does not have. | `encoding/csv` |
| A delimiter-sniffing / "smart CSV" library | Twenty lines of counting on the header line, which you must review anyway. | Own sniffer + a visible override on the mapping screen |
| `goccy/go-json`, `bytedance/sonic`, `json-iterator/go` | Speed, for a file written by hand a few times a week. Sonic additionally means assembly and platform constraints. | `encoding/json` |
| `encoding/json/v2` | GOEXPERIMENT-gated on this toolchain; explicitly outside the Go 1 compatibility promise. | `encoding/json` |
| `nicksnyder/go-i18n`, `golang.org/x/text/message` | The catalogue format is German-sentence → translation, deliberately readable by a human. A message-ID framework would rewrite 1 128 keys for no gain. | `tools/i18n` + `encoding/json` |
| `go-playground/validator` for the new field kinds | `internal/field.CheckAll` already is the one place the rules live — that comment is load-bearing. | Extend `Check` with three cases |
| Any JS date/time/slider/tag widget | `input[type=time]`, `input[type=range]`, radios and checkboxes are native, work without JavaScript, and satisfy the "works with JS off" success criterion for free. | Plain HTML + CSS |
| Fetching the JWKS URL from `X-authentik-meta-jwks` | An outbound call at runtime. | Do not verify (above) |

---

## Version Compatibility

| Component | Verified version | Note |
|-----------|------------------|------|
| Go | 1.26.6 | `encoding/csv`, `encoding/json` behaviours in this document were executed on it |
| authentik | **2026.8.1** (released 2026-09-01) | Header set and endpoint paths verified in source at `version-2026.8`; identical at `version-2025.8` |
| Caddy | **v2.11.4** (released 2026-06-03) | `forward_auth` with `uri` + `copy_headers` as documented |
| htmx | 2.0.10 (vendored) | Unchanged; no new JavaScript in this milestone |
| goose | 3.27.3 | Next migration number is **00046** |

**Verification commands for the planner** (nothing here needs a browser):

```bash
go build ./... && go vet ./... && go test ./...
go run ./tools/i18n                 # must say 0 offen, 0 verwaist
ls internal/db/migrations | tail -1 # confirms the next migration number
```

---

## Evidence and confidence

The confidence seam (`gsd_run query classify-confidence`) rates raw `webfetch`/`websearch` as LOW
and MCP doc providers as MEDIUM; it has no tier for "executed locally". Reported honestly:

| Claim | How it was established | Confidence |
|-------|------------------------|------------|
| The stdlib catalogue writer reproduces all seven committed files byte for byte | Ran it against the actual files with the project's toolchain; 7/7 `identical=true` | **HIGH** (executed) |
| `encoding/json` map ordering, escaping, `SetIndent("","")` no-op, trailing newline | Executed on Go 1.26.6 | **HIGH** (executed) |
| `encoding/json/v2` unavailable without GOEXPERIMENT | `go doc` failed; succeeded with the flag | **HIGH** (executed) |
| `encoding/csv`: BOM not stripped, `ErrFieldCount` resumable, LazyQuotes effect, NUL passthrough, blank-line skip, CRLF folding | Executed against crafted inputs | **HIGH** (executed) |
| authentik header names, `|` separator, `X-authentik-jwt` = raw token | Primary source, `mode_common.go` at tag `version-2026.8` **and** `version-2025.8`; agrees with official docs | **HIGH** (primary source, cross-checked) |
| Outpost paths incl. `/outpost.goauthentik.io/sign_out`, logout query signature | Primary source, `mode_forward.go` / `application.go` / `oauth.go` at `version-2026.8` | **HIGH** (primary source) |
| Caddyfile snippet and `forward_auth` expansion | Official authentik and Caddy documentation | MEDIUM (docs only; the seam rates web fetch LOW — treat the exact `copy_headers` list as authoritative because it matches the header names found in source) |
| authentik subject-mode default, signing-key behaviour | Official docs, partially incomplete on the page | MEDIUM — **verify in the operator's own instance before relying on `X-authentik-uid`'s format** |
| The i18n indent drift is already resolved | Working tree inspection + `.planning/WINDOWS.md` deviation record | **HIGH** (local evidence) |
| Migrations already at `00045` | `ls internal/db/migrations` | **HIGH** (local evidence) |

## Sources

- Local execution, Go 1.26.6 — `encoding/json` and `encoding/csv` behaviour; round-trip against
  `internal/i18n/locales/*.json`
- Repository itself — `go.mod`, `tools/i18n/main.go`, `internal/field/`, `internal/web/clientip.go`,
  `internal/auth/`, `internal/config/config.go`, `internal/admin/wordpress.go`,
  `internal/db/migrations/`, `.planning/WINDOWS.md`
- <https://github.com/goauthentik/authentik> at tags `version-2026.8` and `version-2025.8` —
  `internal/outpost/proxyv2/application/{mode_common,mode_forward,application,oauth,claims}.go`
- <https://docs.goauthentik.io/add-secure-apps/providers/proxy/> — header list, `|` separator,
  sign-out path
- <https://docs.goauthentik.io/add-secure-apps/providers/proxy/forward_auth> — single-application
  vs domain-level modes
- <https://docs.goauthentik.io/add-secure-apps/providers/proxy/server_caddy/> — the Caddyfile
- <https://caddyserver.com/docs/caddyfile/directives/forward_auth> — directive and expanded form
- <https://docs.goauthentik.io/add-secure-apps/providers/oauth2/> — signing-key behaviour

---
*Stack research for: Holzcloud CMS v1.6 — Inhaltsmodell und Zugang*
*Researched: 2026-09-03*
