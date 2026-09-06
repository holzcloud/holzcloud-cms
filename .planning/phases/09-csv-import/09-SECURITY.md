---
phase: "09"
slug: "csv-import"
status: audited
# threats_open = count of OPEN threats at or above workflow.security_block_on (= high)
threats_open: 2
threats_total: 45
threats_closed: 43
asvs_level: 1
block_on: high
created: "2026-09-06"
register_authored_at_plan_time: true
---

# Phase 9 — Security

> Threat register, accepted risks and audit trail for the CSV import.

The register was written at planning time across six plans and checked against
the shipped code afterwards. **Fifty register rows, forty-five distinct threats**
— `T-09-SC` repeats once per plan and counts once. Forty-three closed with a
cited location. **Two open, both `high`, both the same underlying defect.**

The shortcut the workflow allows at `threats_open: 0` was not available, and
would not have been taken anyway: as in phases 7 and 8, a code review found nine
findings after the mitigation texts were written, four of them inside mechanisms
whose disposition already said `mitigate`.

---

## Trust Boundaries

| Boundary | Description | What crosses it |
|---|---|---|
| An uploaded file → `internal/csv` | Bytes from outside the installation. Nothing about their shape, size or encoding may be assumed | Untrusted structure and content, up to 10 MB |
| A staging token in a URL → 10 MB of somebody's file | The token is the whole of the address, and the address is copyable, bookmarkable and logged | A capability |
| One admin → another admin's staged upload | `requireAdmin` admits every administrator of the installation; it does not say which import is whose | Ownership |
| A cell → the public site's HTML | The body is Markdown from a file and reaches a visitor's browser | Untrusted content in an HTML context |
| The row loop → the single write connection | `SetMaxOpenConns(1)`. Anything held across rows blocks admin and public alike | Availability of the whole machine |
| A generated example CSV → a spreadsheet on someone's own machine | A cell beginning `=` is executed by Excel and LibreOffice when the file is opened | Formula injection |
| Five new routes → the authorization chain | `requireAdmin` and the `adminOnly` table are two separate acts, and only the first is in `main.go` | Privilege |

---

## Threat Register

| Severity | Total | Closed | Open |
|---|---|---|---|
| high | 26 | 24 | **2** |
| medium | 11 | 11 | 0 |
| low | 8 | 8 | 0 |

### Open — blocking

#### T-09-14 · Denial of Service · high — a transaction spanning more than one row
#### T-09-30 · Denial of Service · high — the write loop on the single write connection

Two register rows, one property, one defect. Both mitigations assert the same
sentence:

> *"No transaction spans more than one row … `grep -rln BeginTx internal/csv/
> internal/csvimport/ internal/admin/csvimport.go` measures 0, and every store
> the loop calls opens and closes its own inside one row. Other requests
> interleave between rows."*

**The row loop keeps that promise. The term pre-pass does not.**

`internal/admin/csvimport.go:802-822` harvests every term name the *whole file*
mentions and hands the list to `term.EnsureNames`, which opens one transaction at
`internal/term/store.go:330` and inserts every name inside it before committing
at `:349`. The write pool admits one connection (`internal/db/db.go:41`), so that
transaction holds the machine's only write connection for its full duration.

The grep gate that stands as proof is scoped to `internal/csv/`,
`internal/csvimport/` and `internal/admin/csvimport.go`. The transaction is in
`internal/term/`, so the gate measures 0 and the property still fails. This is a
gate that proves a proxy rather than the claim.

**Nothing bounds the size of that transaction.** `csv.MaxRows = 5000` bounds
rows, not names. `term.MaxPerPage = 12` is applied by `RowTerms`
(`internal/csvimport/row.go:142`) but **not** by `TermNames`
(`:160-200`), which calls `add(termNames(cell))` uncapped. The pre-pass also runs
*before* `CheckRow`, so rows that will be refused — no title, invalid address,
oversized cell, too wide — still contribute every name they carry.

**Measured, not argued.** On this machine (Apple silicon, NVMe), against a real
migrated database, using the project's own `db.Open`:

| | Terms in one transaction | Elapsed | Worst wait for one competing write |
|---|---|---|---|
| Term pre-pass, 10.0 MB file, 583 data rows | 1 166 000 | **10.75 s** | **10.75 s** |
| Plain 5000-row write loop, no terms | — | 1.10 s | **3.2 ms** |

The contrast is the finding. The loop the mitigation describes is clean —
5000 pages written while a competing writer never waited more than three
milliseconds. The pre-pass the mitigation does not mention stalled **every write
on the machine for ten and a half seconds** from a single 10 MB upload. The
deployment target is a small linux/amd64 server, where this is plausibly tens of
seconds to minutes.

The blast radius is machine-wide, not admin-only: `auth.NewSQLiteStore(database.Write)`
(`cmd/holzcloud/main.go:175`) puts **session writes** on the same single
connection, alongside contact-form submissions, shop orders and the outbox. This
is exactly the anti-feature IMP-10 and criterion 5 were written to forbid.

**Two further consequences of the same uncapped pre-pass**, stated here rather
than filed as separate threats because they share one root:

1. The terms are created **before** any row is validated and are never removed if
   every row is then skipped. `internal/term/store.go` has no sweep — the only
   `DELETE FROM terms` is a manual per-id delete at `:286`. A 10 MB upload can
   leave 1.17 million permanent rows in `terms` that nothing points at.
2. The dry run does not run the pre-pass at all (`if write` at
   `internal/admin/csvimport.go:802`), so the screen whose entire purpose is to
   show what will happen shows nothing of this and issues no warning.

**Not remotely reachable:** the commit is `requireAdmin` and needs a CSRF token.
The realistic path is an accident, not malice — a supplier's product export whose
tag column carries thousands of distinct values.

**What to fix first:** cap the pre-pass. `TermNames` should apply
`term.MaxPerPage` per row the way `RowTerms` already does, and `EnsureNames`
should be called in batches rather than in one transaction. Either alone reduces
the stall by orders of magnitude; both together close the threat. The row loop
needs no change.

### Closed — a selection with its evidence

The full table is below; these are the ones the audit weighed first.

| Threat | Sev | Evidence |
|---|---|---|
| T-09-01 · unbounded reader | high | `MaxRows = 5000` (`internal/csv/csv.go:49`) enforced in `Next` (`:244`); `MaxCellBytes = 100000` (`:67`) per cell per row (`:274-283`); `MaxColumns = 100` (`:58`) in `New` (`:223`) — a fourth bound the plan did not name. `TestRowLimit`, `TestColumnLimit`, `TestCellLimitInBytes` |
| T-09-02 · NUL byte | medium | `CheckBytes` refuses `\x00` on the raw bytes (`internal/csv/csv.go:110`), called at `internal/admin/csvimport.go:288` **before** `Stage` at `:369`. Refused and not cleaned. `TestCSVDegenerateFilesAreRefused` asserts `staged rows = 0` for every refusal |
| T-09-03 / T-09-25 · formula injection | high | `defuse` (`internal/csv/example.go:63`) prefixes an apostrophe to `= + - @ 0x09 0x0d`, applied to the **header** (`:27`) and every sample row (`:31`). Byte-for-byte identical to `plugins/kontaktformular/csv.go:117`; each copy names the other. Headers: `attachment` (`internal/admin/csvimport.go:1090`), `nosniff` (`:1089`), `no-store` (`:1094`). The filename passes `page.Slugify` → `[a-z0-9-]` only |
| T-09-07 / T-09-09 · the token | high / medium | 16 bytes from `crypto/rand` as hex (`internal/csvimport/store.go:115-119`) = 128 bits. Only `hashToken(token)` is inserted (`:131`); the lookup is `WHERE token_hash = $1` (`:152`). `TestTokenIsNotInTheDatabase` |
| T-09-06 / T-09-23 / T-09-35 · ownership | high | `Get` compares `u.UserID != userID` → `ErrForeign` (`store.go:161-163`) in the store, not the handlers. `staged` (`internal/admin/csvimport.go:390-405`) is the single place either error becomes a response, and all three token-bearing screens reach it (`:633`, `:880` via `csvPrepare`). `TestCSVForeignTokenIsNotFound` asserts the 404 **and** that the body carries no cell of the other admin's file; the browser pass drove it with a second real account |
| T-09-15 · stored XSS | high | `page.RenderMarkdown` (`internal/csvimport/row.go:328`) → goldmark then `sanitizer.Sanitize` (`internal/page/markdown.go:81-84`). `grep template.HTML internal/csv/ internal/csvimport/ internal/admin/csvimport.go` measures 0 outside one comment |
| T-09-21 · byte cap | high | `http.MaxBytesReader(w, r.Body, csvMaxUpload)` is the first statement of `HandleCSVImport` (`internal/admin/csvimport.go:264`). `TestCSVTenMegabyteLimit` proves the boundary and one step either side **and** asserts the staged row count does not move when the cap fires |
| T-09-22 / T-09-29 · authorization | high | Five routes, not the three the plan foresaw. All five `requireAdmin` (`cmd/holzcloud/main.go:889-896`); all five in `TestRouteAuthorization`'s table (`main_test.go:173-177`), which drives `newRouter`, so a future pattern conflict fails a test rather than a deploy |
| T-09-31 · a refresh importing twice | high | **Implemented differently and more strongly than declared** — see below |
| T-09-34 · operator text in a reason | medium | Every arm of `csv_reason.html` is `{{t}}` or `{{tf}}`; `tf` returns `string` (`internal/web/render.go:99`), which `html/template` escapes contextually. `{{th}}`, which returns `template.HTML` (`:95`), appears **zero** times in `csv_reason.html`, `csv_dryrun.html` and `csv_report.html` |
| T-09-38 · a reason with no arm | low | `{{else}}` fallback plus `TestCSVEveryReasonHasASentence`, which reads both files and fails on a missing arm, a duplicate arm **and** an arm nothing in Go produces |
| T-09-SC · package install | low | `go.mod` and `go.sum` appear in none of the 63 phase-9 commits. The legitimacy gate correctly does not fire |

**Complete closed set:** T-09-01 … T-09-13, T-09-15 … T-09-29, T-09-31 … T-09-44,
T-09-SC. Forty-three of forty-five.

---

## Accepted Risks Log

> The auditor could verify the substance of each of these in the code but could
> check them against no record, because there was none. Phase 8's sentence still
> holds: *an assumption that is never written down is an assumption nobody made.*
> Here they are, made explicitly.

| Risk | Reference | Argument, evidenced in code | Accepted | Date |
|---|---|---|---|---|
| `ErrExpired` and `ErrForeign` are distinguishable | T-09-08 · low | A correct guess of a foreign token answers 404 while a wrong guess answers the expiry screen, so the pair distinguishes "exists" from "does not exist". The token is 128 bits from `crypto/rand` (`internal/csvimport/store.go:115`), so reaching that oracle requires already holding the answer. One answer for both would tell an operator whose own upload the sweep took that they had done something wrong, which D-33 forbids in as many words. The argument is written out at `store.go:56-70` | Plan 09-02 | 2026-09-06 |
| A translation may say what the German does not | T-09-44 · low | `tools/i18n` checks that every key has a non-empty value, not that the value means the same thing. The same exposure every string in this project already carries; the German source is the original a reader can compare against. Five catalogues, 1271 strings, 0 open | Plan 09-06 | 2026-09-06 |
| No package installation in this phase | T-09-SC (×6) · low | `git log --name-only` over the 63 phase-9 commits returns neither `go.mod` nor `go.sum`. Every symbol is standard library or already present | Audit run | 2026-09-06 |
| **`RequireWebsiteAccess` reads the website id from the URL *path*, so five routes taking it from a form value escape the middleware** | new, not in the register · accepted | `auth.RequireWebsiteAccess` (`internal/auth/middleware.go:102`) resolves the id through `websiteIDInPath` (`:118-131`), so `GET /admin/csv-vorlage?website=<id>` and `POST /admin/websites/import-csv` never reach the check. **Verified harmless today, and only today:** both routes are `requireAdmin` (`main.go:889`, `:896`), `RequireAuth` re-reads the role from the database on every request (`middleware.go:41-58`), and `NewWebsiteAccessLookup` returns `true` unconditionally for role `admin` (`internal/admin/handler.go:168-170`) — `user_websites` restricts editors only. So an admin escaping the middleware gains nothing an admin does not already have. The day a route of this shape is opened to an editor, that stops being true. `internal/admin/csvimport.go:1040-1049` records this in the handler itself. This is *not* the shape of the cross-website hole fixed in the menu handlers today (`de4a1ce`, `5e453a9`): that one was reachable by an editor | Audit run | 2026-09-06 |
| `Get` materialises the file before the ownership comparison | T-09-23 · low | `internal/csvimport/store.go:149-163` scans `daten` in the same `SELECT` that finds the row, so a foreign token costs one read of up to 10 MB before `ErrForeign` is returned. Nothing is rendered — `staged` answers `http.NotFound` before the upload is returned to any caller, and `TestCSVForeignTokenIsNotFound` asserts the body. Splitting the query into an ownership probe and a fetch would be two round trips on the read pool to save a discarded buffer | Audit run | 2026-09-06 |
| `ReasonRowUnreadable` carries `encoding/csv`'s own `ParseError` text | T-09-04 · low | That text names a **line** index, which is what T-09-04 says never reaches a screen. It is a deliberate exemption, documented at `internal/csv/csv.go:27-30` and `:159-166`. It is also currently unreachable: `LazyQuotes = true` (`:207`) suppresses `ErrBareQuote`/`ErrQuote` and `FieldsPerRecord = -1` (`:211`) suppresses `ErrFieldCount`, which is every `ParseError` the reader can raise. The oversized-cell sentence never reaches a screen either, because `CheckRow` measures the cells itself (`internal/csvimport/row.go:274-281`) **before** it looks at `row.Error` (`:290`). If `LazyQuotes` is ever removed, this becomes live | Audit run | 2026-09-06 |

---

## Where the implementation departs from its mitigation text

Not behaviour errors. They read wrong, and belong corrected if these plans are
ever used as a source again.

| Threat | What the text says | What the code does |
|---|---|---|
| T-09-31 | "The staging row is removed **after** the loop and before the render … a process that dies mid-write leaves the row intact and the operator can retry" | `Store.Claim` deletes it **before** the first write (`internal/admin/csvimport.go:966-972`), and only the request whose `DELETE` affected a row imports. **Stronger than declared:** the declared design left two overlapping commits both passing `staged()`, both reaching the loop, and on the "new website" path both calling `CreateWebsite` — two websites from one double-click, on a form htmx never processes, so `hx-disabled-elt` does not fire. The trade is written out at `internal/csvimport/store.go:180-190`. `TestCSVReloadDoesNotImportTwice`, `TestClaimSucceedsExactlyOnce` |
| T-09-21 | "exactly 10 MB accepted, 10 MB + 1 refused" | `http.MaxBytesReader` bounds the **whole request body**, so the multipart envelope is part of what the cap counts and a file of exactly 10 MB does not fit. Measured and recorded as a correction to D-08 in `09-04-SUMMARY.md:50` and `:263-276`; `wordpress.go:25` has always had the same property. The test measures the envelope and probes the real boundary |
| T-09-22 | "All **three** are wrapped in `requireAdmin`" | Five shipped. All five are wrapped and all five are in the table |
| T-09-24 | "`?zeile=`" | The parameter is `?row=` (`internal/admin/csvimport.go:545`). Clamped as described; `TestCSVSampleRowSteppingIsClamped` |
| T-09-14, T-09-30 | "No transaction spans more than one row" | False for `term.EnsureNames`. This is the open finding above, not a documentation error |

Several mitigations also name tests by German identifiers
(`TestFremdeMarkeWirdAbgelehnt`, `TestNeuladenImportiertNichtZweimal`,
`TestJaNeinKennsteinGeschlossenesVokabular`, `TestBenutzerLoeschenRaeumtAblageMit`,
`TestProbeUndStartVerdictenGleich`). All exist under English names
(`TestForeignTokenIsRefused`, `TestCSVReloadDoesNotImportTwice`,
`TestBoolKnowsAClosedVocabulary`, `TestDeletingAUserTakesTheUploadWithIt`,
`TestCSVProbeAndStartAgreeOnEveryVerdict`) and assert what was promised.

---

## Areas examined and found clean

Named, so "clean" is a statement about something and not an absence of effort.

- **The staging table.** `user_id … ON DELETE CASCADE` and `website_id … ON DELETE
  SET NULL` are database facts (`internal/db/migrations/00049_csv_imports.sql:59`,
  `:68`), each with a test (`TestDeletingAUserTakesTheUploadWithIt`,
  `TestDeletingAWebsiteLeavesTheUploadStanding`). The sweep really deletes:
  `Prune` (`store.go:220-229`) with `TestPruneSweepsOnlyTheOld`, wired at
  `cmd/holzcloud/main.go:523-551` every 6 h at 24 h age with **no `RunAtStart`**,
  so a deploy does not sweep a running import. `modus` and `kollision` are closed
  vocabularies in `CHECK` constraints, not merely in the handler. `STRICT` table.
- **Logging.** `grep 'slog\.|log\.|fmt.Print'` over `internal/csv/`,
  `internal/csvimport/` and `internal/admin/csvimport.go` matches nothing but one
  comment. Neither the token nor a cell is logged anywhere. The browser pass
  recorded 0 `ERROR` lines and two `WARN` lines, both expected 404s.
- **CSRF.** All three POSTs carry the hidden input:
  `website_list.html:56`, `csv_mapping.html:61`, `csv_dryrun.html:63`. The admin
  chain is wrapped by `gorilla/csrf` at `cmd/holzcloud/main.go:1036`. The two
  `csv-vorlage` forms are GETs and need none.
- **Scripting.** No `.js`, no `<script>`, no `on*` attribute, no `javascript:` URL
  on any of the five new templates. The only `hx-` attribute used is
  `hx-disabled-elt`. All four screens were driven with `javaScriptEnabled: false`
  and every control worked. CSP `default-src 'self'` is the second layer; 0
  violations in the pass.
- **The layout, and therefore the CSRF body attribute.** All four new pages are in
  `layoutPageNames` (`internal/web/render.go:51`), counted 48/4 against the tree.
- **The 5000-row write loop itself.** Measured: 5000 pages in 1.10 s while a
  competing writer's worst wait was 3.2 ms. `CreatePage` autocommits, `SetForPage`
  opens and commits its own inside one row, and other requests genuinely
  interleave. T-09-30's claim is true of everything except the term pre-pass.
- **`?row=` clamping.** `[1, n]`, negative and over-large both land on a valid row
  (`internal/admin/csvimport.go:545`, `:566-575`); no arithmetic reaches a slice
  out of range.
- **Per-row integrity.** `field.CheckAll` before the first write; compensation on
  the create arm through the ordinary `TrashPage` → `PurgePage`; the update arm
  deliberately **not** rolled back, asserted by `TestUpdateArmIsNotRolledBack`.

---

## Warning — process, not code

`09-06-SUMMARY.md` carries no `## Threat Flags` section. The other five summaries
do. Nothing in plan 09-06's work appears to have opened new attack surface — it is
catalogues, CSS and four browser-found display defects — but the absence was not
declared, so the audit had to establish it rather than read it.

---

## Audit Trail

### Security Audit 2026-09-06

| Metric | Count |
|---|---|
| Register rows across six plans | 50 |
| Distinct threats | 45 |
| Closed | 43 |
| Open (blocking, ≥ `high`) | **2** |
| Open (non-blocking, below `high`) | 0 |
| Risks accepted and now recorded | 6 |
| Documentation errors without behavioural consequence | 5 |
| Unregistered flags | 1 (missing `## Threat Flags` in `09-06-SUMMARY.md`) |

**Method:** all six `<threat_model>` blocks, all five `## Threat Flags` sections,
the code review, the fix report, `deferred-items.md` and phase 8's audit read,
then every register row checked against the code. Two claims were measured rather
than read, against a real migrated database in an isolated copy of the tree:
the term pre-pass (10.75 s, one transaction, 1.17 M rows) and the plain 5000-row
loop (1.10 s, worst competing wait 3.2 ms). **No implementation file was
modified**; `git status` was clean before and after.
`go test -count=1 ./internal/csv/ ./internal/csvimport/ ./internal/admin/
./cmd/holzcloud/` — all green.

**Next:** cap `TermNames` at `term.MaxPerPage` per row and batch `EnsureNames`,
then re-run `/gsd-secure-phase`. Nothing else in the phase blocks ship.
