# Phase 9: CSV Import - Pattern Map

**Mapped:** 2026-09-06
**Files analyzed:** 16 (12 new, 4 modified)
**Analogs found:** 13 / 16
**Central finding:** **there is no multi-screen wizard anywhere in this tree.**
Every admin flow is either one screen (GET renders, POST redirects) or one screen
that re-renders itself. Phase 9's four-screen flow **invents a pattern**, and the
plan must fix its shape deliberately rather than "copy the wizard" — because
there is none to copy. Details in §A.

---

## A. The finding the prompt asked for first: no multi-step admin flow exists

Searched exhaustively: every `Handle*` in `internal/admin` (`grep -n "func (h
\*Handler) Handle"` across `setup.go`, `media_crop.go`, `template.go`, `user.go`,
`twofactor.go`, `page_bulk.go`, `confirm.go`, `bundle.go`, `wordpress.go`,
`plugin.go`), plus the shop checkout in `internal/public/checkout.go` and
`internal/shop`.

What exists, and why none of it is the analog:

| Candidate | Shape | Why it is not a wizard |
|---|---|---|
| Bundle import `internal/admin/bundle.go:80` | one POST → renders `import_report` | **One** POST. Upload and report, nothing between. |
| WordPress import `internal/admin/wordpress.go:22` | one POST → renders `import_report` | Same. This is the *closest* import analog and it is single-step. |
| Media crop `internal/admin/media_crop.go:60, :89` | GET screen, POST saves, redirect back to the GET | The screen is reached from a **list**, not from a previous POST. No state is carried between them beyond the media id in the path. |
| Template upload `internal/admin/template.go:84` | GET form (`:45`) then POST | The GET is a plain form; the POST ends in a flash + redirect. |
| Password confirm `internal/admin/confirm.go:27` | GET/POST on one handler, `?weiter=` carries the destination | **The nearest thing to a step.** But it is an *interstitial*, not a step: it carries one URL (`auth.SafeReturn`), not a payload, and it hands control back rather than forward. |
| Two-factor enrolment `internal/admin/twofactor.go:138 → :173 confirmTwoFactor → :247 showRecoveryCodes` | GET setup screen → POST → **renders a second screen** | **The only POST in the tree that leads to a different screen.** But the state is carried in the *database against the user id* (`EnsurePendingSecret`, `:154`), there is no token in any URL, and it is two screens, not four. |
| Public checkout `internal/public/checkout.go:28` | GET/POST on one handler, then `redirect /bestellung/{number}` | Two screens, and the carried state is the **cart cookie** (`shop.TokenFrom(r)`, `:34`). Cookie-carried, not URL-carried. |
| User invitation `internal/admin/user.go:178` + `internal/user/token.go:39` | POST creates → mails a link → the *recipient* opens a token URL | The two halves are separated by an e-mail and by a different person. Not a flow. |

**Verdict for the planner.** The four-screen flow of D-01 has **no analog**. Two
half-analogs exist and the plan should name which of them it is copying, because
they disagree:

- **`twofactor.go:138-207` is the model for "a POST renders the next screen"** —
  in particular `confirmTwoFactor`'s error arm (`:184-201`), which re-renders the
  *same* screen with the state re-fetched from the store rather than redirecting.
  Screens 2 and 3 of this phase need exactly that arm: a bad mapping must come
  back on the mapping screen with the mapping intact, and the only way to get the
  mapping back is to re-read the staged row from the store.
- **`confirm.go:27-73` is the model for one handler serving GET and POST** — the
  `if r.Method == http.MethodPost { … }` block first, the GET render last. Route
  table `cmd/holzcloud/main.go:830-832` shows the house style for registering
  both verbs on one path against one handler.

**What NOT to copy from either:** `twofactor.go` keys its half-finished state on
the *user id*, so a user can only ever have one enrolment in flight. A staging
token must key on the token and *check* the user (D-06), not key on the user —
otherwise a second upload silently destroys the first.

**Consequence for the plan:** the shape decisions — where the token lives in the
URL, whether screen 2 is GET or POST, what happens on a token that has been swept
— are **new**, and should be written down in the plan as decisions with reasons,
in the voice `media_crop.go:14-24` uses for its own invented pattern (a comment
block at the top of the file that explains why the shape is what it is, before
any code).

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/csv/csv.go` (new) | parser | streaming | `internal/wxr/wxr.go:1-63` | role-match |
| `internal/csv/beispiel.go` (new) | writer | file-I/O | `plugins/kontaktformular/csv.go:23-98` | exact |
| `internal/csv/entschaerfen` (in the above) | utility | transform | `plugins/kontaktformular/csv.go:113-128` | exact (verbatim) |
| `internal/csv/csv_test.go` (new) | test | — | `internal/wxr/wxr_test.go:48, :100` | role-match |
| `internal/db/migrations/00049_csv_imports.sql` (new) | migration | batch DDL | `internal/db/migrations/00042_outbox.sql:13-60` + `00012:12` | exact |
| staging store — **`internal/csv/store.go`** (new) | store | CRUD | `internal/user/token.go:39-121` | role-match |
| `internal/admin/csvimport.go` (new) | handler | request-response | `internal/admin/wordpress.go:22-97` (write) + `twofactor.go:138-207` (flow) | partial — see §A |
| `internal/admin/csvimport_test.go` (new) | test | — | `internal/admin/snippet_fields_test.go:1-60` over `page_handler_test.go:33-96` | exact |
| `templates/admin/csv_mapping.html` (new) | template | — | `translation_matrix.html:32-62` + `field_input.html` | role-match |
| `templates/admin/csv_probe.html` (new) | template | — | `import_report.html` (shape only) | partial |
| `templates/admin/csv_report.html` (new) | template | — | **no analog** — D-25's grouping has no precedent | none |
| `templates/admin/csv_upload.html` (new, if screen 1 needs more than the panel) | template | — | `website_list.html:4-22` | exact |
| `cmd/holzcloud/assets/admin.css` (modified) | style | — | `admin.css:2050-2083` (`matrix`) | exact |
| `cmd/holzcloud/templates/admin/website_list.html` (modified) | template | — | its own `:24-45` panel | exact |
| `cmd/holzcloud/main.go` (modified) | config | — | `:851-853` routes, `:498-520` prunes | exact |
| `cmd/holzcloud/main_test.go` (modified) | test | — | `:158-172` table | exact |
| **`internal/web/render.go:46`** (modified) | config | — | the list itself | exact — **easy to forget, see §K** |

---

## Pattern Assignments

### `internal/csv/` — the pure package

**Analog:** `internal/wxr/wxr.go:1-63`. Copy the **package doc-comment voice**
above everything: `wxr.go:1-12` states *why the package exists*, then states what
it deliberately does **not** do ("What is deliberately *not* done here: fetching
the pictures"). `internal/csv`'s doc comment owes the same two paragraphs, the
second naming the anti-features of D-16 (the second copy of `entschaerfen`) and
the deferred separator sniffing.

**Cap constant + reported truncation** (`wxr.go:26-30` and `:60-63`):

```go
// MaxItems bounds one import.
//
// A WordPress site with more entries than this exists, but importing it in one
// go means holding all of it in memory at once, which a small node does not
// have to spare. The limit is reported rather than silently applied.
const MaxItems = 2000
…
	// Truncated says the file held more than MaxItems entries.
	Truncated bool
```

D-09's `MaxRows = 5000` and D-10's `MaxCellBytes = 100000` copy this exactly:
a constant with a two-sentence *reason*, and a `Truncated bool` on the result
struct rather than an error.

**What NOT to copy from `wxr`:** `wxr.Parse` materialises the whole document into
`Export.Items` (`:52-55`). D-03 says `internal/csv` streams. So the *struct shape*
`Export{Items, Skipped, Truncated}` is a template for the **summary** the reader
returns, not for a slice of every row. The plan should say which of the two the
type is, because the field `Items []Item` is the single most copyable and the
single most wrong thing in the analog.

**The German-comment/English-doc split** is real and consistent: `wxr.go` and
`page/store.go` doc comments are English; `wordpress.go:15-19` and
`migrations/*.sql` are German. `field.go:965-973` mixes them in one file. Rule
observed: **exported doc comments in English, in-body "why" comments in German.**

**`entschaerfen` — copy verbatim, plus a cross-reference.** Source
`plugins/kontaktformular/csv.go:113-128`, with its comment at `:104-112`. It is
16 lines and already carries the full rationale. D-16 requires *both* copies to
name the other; the plugin file must gain that sentence too, which makes
`plugins/kontaktformular/csv.go` a **modified** file the build order does not
currently list.

**The BOM** — `plugins/kontaktformular/csv.go:31-33` is the proof-of-normal-case
D-11 rests on:

```go
	// Die Byte-Order-Mark, damit Excel die Datei als UTF-8 öffnet. Ohne sie
	// wird aus einem Ü ein Ãœ, und dann tippt doch wieder jemand ab.
	buf.WriteString("﻿")
```

The example CSV writer (IMP-07) should write it for the same reason; the reader
strips it for the mirror reason. One sentence in each, naming the other.

### `internal/csv/csv_test.go`

**Analog:** `internal/wxr/wxr_test.go:48 TestParse`, `:100
TestParseLehntUnsinnAb`. Two tests, German names, one happy path and one hostile
path. The hostile-file checklist D-08…D-16 is nine named defences; the plan
should name each subtest after the defence rather than after the input, so a
failing line says which rule broke.

**Contrast worth naming:** `internal/admin/snippet_fields_test.go:22-31` shows
the *newer* house style — a paragraph-length comment above the test file
explaining what the file is the gate on. For a nine-defence checklist that is the
better model than `wxr_test.go`'s bare tests.

---

### `internal/db/migrations/00049_csv_imports.sql` (new)

**Next free number confirmed:** the tree runs `…00046, 00047, 00048`. `00049` is
free.

**Analog:** `internal/db/migrations/00042_outbox.sql:1-60` — a whole new table
with a `CHECK`ed status column and per-column German paragraphs. Its head comment
(`:1-12`) is the exact voice: *what the table is for, and what goes wrong without
it*.

```sql
-- +goose Up

-- Der Postausgang.
--
-- E-Mail wird nicht im Anfrage-Zyklus verschickt. Ein Mailserver, der drei
-- Sekunden für die Begrüssung braucht, hängt sonst an der Bestellung der
-- Kundin, …
CREATE TABLE outbox (
    id INTEGER PRIMARY KEY,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    …
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed')),
```

**The `ON DELETE CASCADE` from `users` D-01 asks for** has its analog at
`internal/db/migrations/00012_account_tokens_and_404_log.sql:12` (`user_tokens`)
— the only other table in the tree keyed to a user rather than to a website.
**This is the one place where two candidate analogs disagree, and it matters:**
every other table in `migrations/` cascades from `websites`. A `csv_imports` row
belongs to a *user* (D-06's ownership check) and only optionally to a website (a
new-website import has no website yet). `00012` wins; the plan must say so in the
migration's comment, because a reader who has seen the other forty migrations
will read `website_id INTEGER` with no `NOT NULL` as an oversight.

**STRICT:** `CLAUDE.md` says STRICT where possible. A BLOB column is fine under
STRICT (`BLOB` is one of the five STRICT types). Verify against `00041` /`00042`
whether those tables carry `STRICT` — if the recent ones do, `00049` does too.

**What NOT to copy:** `00047`/`00048` are *index-swap* migrations
(`00048:29-35`). Their shape — `DROP INDEX` / `CREATE UNIQUE INDEX` — is not this
migration's, and their headline lesson ("a released migration is never edited; a
correction is its own file", `00048:26-28`) applies but their body does not.

---

### The staging store — recommendation: **`internal/csv/store.go`**

The prompt asks for an argument. Three candidates:

| Placement | For | Against |
|---|---|---|
| **`internal/csv`** | one package per feature is the tree's rule (`internal/wxr`, `internal/term`, `internal/sharelink`); the handler then takes one dependency, not two | **breaks D-03's "pure: no database"**, which is stated in the context as a property of the *package* |
| `internal/csvimport` (new, second package) | keeps `internal/csv` pure exactly as D-03 words it | a package containing one struct and four methods; nothing else in the tree splits parse and store this way — `internal/wxr` has no store because WXR has no staging |
| `internal/admin` | the handler owns it; `internal/admin/pagination.go` is precedent for a non-handler type living here | `internal/admin` is at 14.7 % coverage and is where authorisation lives; putting SQL there means the staging queries are only testable through HTTP |

**Recommendation: a second package, `internal/csvimport`.** D-03's word "pure"
is load-bearing — it is what makes the hostile-file checklist testable with no
database, and it is build-order step 1. Merging the store in makes step 1
untestable without `db.Open`. The two-package split has a precedent in shape if
not in name: `internal/shop` holds `cart.go` (store) beside `pricing.go` (pure),
in **one** package — so the tree's own precedent argues the other way. The plan
must **pick and justify**; this document's recommendation is the split, and the
counter-evidence (`internal/shop`) is named so the planner can overrule it
knowingly.

**Whichever is chosen, the store's analog is `internal/user/token.go:39-121`**,
and it answers question (b) of the prompt.

### (b) Where a per-request random token is minted and checked

Two candidates, and they disagree. **`internal/user/token.go` wins.**

**`internal/user/token.go:49-56`** — the model:

```go
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate token: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().UTC().Add(lifetime)
```

- **Randomness:** `crypto/rand.Read` into a `[]byte`. This is the tree's single
  idiom — `internal/shop/cart.go:145` (24 bytes), `internal/web/logging.go:49`
  (8 bytes), `internal/totp/totp.go:50`, `internal/ai/token.go:110` all do it.
  D-07 asks for 128 bits: `make([]byte, 16)`.
- **Encoding:** `base64.RawURLEncoding` in `user`, `shop` and `ai`;
  `hex.EncodeToString` in `web/logging.go:53` and in `hashToken`
  (`token.go:152-155`). D-07 says hex, which matches `logging.go` and
  `hashToken`. Either is defensible; hex is what D-07 chose and hex is what a
  path segment reads most safely.
- **Comparison:** `token.go:76-80` looks the row up **by the hash**, so the
  comparison is the SQL equality on `token_hash`; `token.go:159-161` provides
  `SameToken` (`crypto/subtle.ConstantTimeCompare`) for the case where the caller
  already holds the value. `internal/sharelink/sharelink.go:66-69` uses
  `hmac.Equal` with a comment saying why constant time matters.
- **Lifetime + purge:** `token.go:23-27` (two named lifetimes with a one-line
  reason each) and `token.go:114-124` `PurgeExpiredTokens` — which is *exactly*
  the `csv-import-prune` job body, returning `(int64, error)`.

**What NOT to copy:** `token.go:59-62` deletes the previous token for the same
user and purpose so an old link stops working. A staging token must **not** do
this — an admin with two browser tabs would lose the first import. This is the
one line that must be consciously dropped, and the plan should say so.

**Why `internal/sharelink` loses.** It is *stateless*: an HMAC over
`pageID-expiry` (`sharelink.go:52-56`), verified without a database. That works
because the payload is one integer. A staging token points at 10 MB of bytes that
must live somewhere anyway, so the row exists regardless and a signature buys
nothing. `sharelink` remains the right model for **one** thing: the comment at
`:64-66` on why comparison is constant-time, and the ordering discipline at
`:79-82` (check the signature *before* the expiry, so an expired-but-genuine
token and a forged one get different answers). For this phase: check ownership
before expiry, so "this import is not yours" and "this import was swept" are told
apart honestly.

---

### `internal/admin/csvimport.go` — the handlers

**Name:** `csvimport.go` is right — the tree names handler files after the
feature (`wordpress.go`, `bundle.go`, `media_crop.go`), not after the package.

**Analog for the write half: `internal/admin/wordpress.go:22-97` + `:107-149`.**

**The cap, with its comment** (`wordpress.go:24-26`) — D-08 copies this
character for character, swapping the reason:

```go
	// Eine WXR-Datei ist Text und wird ohne Bilder exportiert; zehn Megabyte
	// sind mehr als jede davon und wenig genug für einen kleinen Server.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
```

**The failed-upload arm** (`:28-32`) — note it is a flash + redirect, not a 422:

```go
	file, _, err := r.FormFile("wxr")
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Datei zu groß oder nicht ausgewählt")
		return h.redirect(w, r, "/admin/websites")
	}
	defer file.Close()
```

**The one-row function returning a reason string** (`:102-149`) — D-22's
`pruefeZeile` is this signature. What to copy: the `return fmt.Sprintf(…)` for
every failure and the bare `return ""` at the end; the German user-facing
sentences quoting the offending value with `%q`; the doc comment that explains
the *content* decision, not the mechanics:

```go
// importWordPressItem creates one page, or says why it could not.
//
// The content arrives as HTML and is stored as the page's source. That is not a
// compromise: Markdown passes block HTML through, …
func (h *Handler) importWordPressItem(r *http.Request, websiteID int64, item wxr.Item, seen map[string]bool) string {
```

**What NOT to copy:** three things.

1. `:113-119` re-invents slug de-duplication with a `seen map[string]bool`.
   D-23 says the CSV importer must let `page.CreatePage` do it
   (`internal/page/store.go:469-489`, `maxSlugAttempts = 100` at `:406`) and
   report `created.Slug != wanted`. Copying `seen` would hide exactly what
   criterion 4 asks to be reported.
2. `:88` `report.Warnings = append(…)` on `bundle.Report` — D-24 forbids reusing
   that struct.
3. `wordpress.go:15-19`'s stated rule that an import always creates a new
   website. This phase departs from it (IMP-04), and the roadmap requires the
   departure to be written into the plan as a decision.

**The compensation path D-02 asks for** — `internal/page/store.go:801`
`TrashPage`, then `:864` `PurgePage`. No new store method. Confirmed both exist
and both are single statements.

**Handler dependency:** everything needed is already on `Handler`
(`internal/admin/handler.go:37-79`): `pages`, `terms`, `fields`, `domains`,
`resolver`, `db`, `cfg`, `sm`, `templates`. Only the staging store is new. Follow
the nil-tolerant style of `mail`/`aiTokens`/`activityStore` (`:56-65`) **only if**
the store can be absent — it cannot, so it is a plain non-nil member like
`fields`.

**Route registration** (`cmd/holzcloud/main.go:851-853`):

```go
	adminProtectedMux.Handle("POST /admin/websites/import", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteImport))))
	adminProtectedMux.Handle("POST /admin/websites/import-wordpress", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWordPressImport))))
```

Five new lines in this shape. `main.go:830-832` shows GET and POST on one path
registered as two lines against one handler — that is screen 2 if it is
GET-and-POST.

**Error style:** handlers return `error`; `ErrHandler`
(`internal/admin/handler.go:201-207`) logs and writes a bare 500. Anything a
person should read is a flash or a re-render, never a returned error.

---

### (e) How an admin handler streams a file download — for IMP-07

Five downloads exist. **`internal/admin/language.go:71-79` is the closest**: a
generated text file with a fixed name, two headers and a `w.Write`.

```go
func (h *Handler) HandleLanguageTemplate(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="sprache-vorlage.json"`)
	_, err := w.Write(i18n.Starter())
	return err
}
```

For a per-website filename, `internal/admin/bundle.go:46-51` is the model, and
`exportFilename` (`:71-77`) is the escaping:

```go
	filename := exportFilename(ws.Name)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	// The archive is built while it is written, so nothing may cache it and
	// nothing can know its length in advance.
	w.Header().Set("Cache-Control", "no-store")
…
func exportFilename(siteName string) string {
	slug := page.Slugify(siteName)
	if slug == "" {
		slug = "website"
	}
	return fmt.Sprintf("%s-%s.holzcloud.zip", slug, time.Now().UTC().Format("2006-01-02"))
}
```

**How the filename is escaped — two different answers in the tree, and they
disagree:**

- `bundle.go` runs the website name through `page.Slugify`
  (`internal/page/slug.go:108`), which yields `[a-z0-9-]` only. Safe by
  construction.
- `internal/admin/plugin.go:387-406` `safeDownloadName` strips control
  characters and `/ \ " ;`, caps at 100 bytes, strips leading dots, and falls
  back to `"export.txt"`. Used because a *plugin* supplies the name.

**`page.Slugify` wins for IMP-07**, because the name comes from the website, not
from an untrusted source — same provenance as `bundle.go`'s. Suggested:
`fmt.Sprintf("%s-vorlage.csv", slug)`.

**Also copy from `plugin.go:369-374`:** `X-Content-Type-Options: nosniff` with its
German comment, and the deliberate `attachment` over `inline`. A CSV served
inline is a file a browser may try to render.

**Content type:** `text/csv; charset=utf-8`. The BOM is written into the body
(`plugins/kontaktformular/csv.go:33`), not signalled in the header.

**Note on the route:** D-06's table registers `POST …/beispiel`. Every existing
download in the tree is a **GET** (`bundle.go` export, `language.go` ×2,
`template.go:358`). A POST download is a departure — defensible if the mapping
form's current state must be posted along to generate the columns, but the plan
should say that in one sentence or the reviewer will read it as a slip.

---

### (c) The i18n contract — exact steps

**The tool:** `tools/i18n/main.go`. Its own doc comment (`:1-25`) is the
contract. Read it before adding a single string.

**What it collects** (`main.go:65-77`):

1. `collectTemplates("cmd/holzcloud/templates/admin")` — every `{{t "…"}}`,
   `{{th "…"}}`, `{{tf "…" …}}`, matched by `callsInTemplates`
   (`main.go:44`). **The literal must be a Go string literal in the template**;
   it is read back with `strconv.Unquote`, so a string built by concatenation is
   invisible to the tool and will render untranslated.
2. `collectGo("internal")` — first-string arguments of the functions in
   `goFuncs` (`main.go:51-62`): `SetFlashError`/`SetFlashSuccess`/
   `SetFlashWarning` (arg 2), `Errors.Add` (arg 1), `NewLayoutData` (arg 2),
   `Titlef` (arg 1), `T` (arg 1), `N` (arg 0). **A user-visible string built any
   other way — e.g. `fmt.Sprintf` into a report warning — is never collected and
   never translated.** `wordpress.go:66-84` builds its warnings with
   `fmt.Sprintf` and they are consequently untranslated today. Phase 9's report
   is *all* such strings, so this is the phase's largest i18n hazard: **every
   per-row reason must go through `web.Titlef`/`web.T`, or through a `{{tf}}` in
   the template with the numbers passed as data.** Prefer the latter — it keeps
   the sentence in the catalogue and the numbers out of it.

**The catalogues:** `internal/i18n/locales/` — `en.json`, `es.json`, `fr.json`,
`it.json` (full translations, written by `-write`) and `de-CH.json`,
`fr-CH.json`, `it-CH.json` (deviation lists, `main.go:104-129`: **never filled**,
only checked; `de-CH.json` is *generated* by `-schweiz`).

**The exact steps for a new German string:**

1. Write the German in a `{{t}}`/`{{th}}`/`{{tf}}` in the template, or pass it to
   one of the eight `goFuncs`.
2. `go run ./tools/i18n` — reports `N offen` for each of `en/es/fr/it`.
3. `go run ./tools/i18n -write` — appends the key with an **empty** value to
   `en.json`, `es.json`, `fr.json`, `it.json`. **This does not close the gate**;
   empty counts as `offen` (`main.go:139-141`).
4. Fill the four values by hand.
5. If any German string changed spelling, `go run ./tools/i18n -schweiz` to
   rebuild `de-CH.json`.
6. `go run ./tools/i18n` again — QUAL-01 needs `0 offen, 0 verwaist` on all four.
   `verwaist` (`main.go:145-149`) means a key in a catalogue with no sentence in
   the source: renaming or rewording a German string creates one, and the tool
   **never deletes it** (`main.go:22-24`) — it must be removed by hand.

**Files that change per string:** the template or Go file, plus
`internal/i18n/locales/{en,es,fr,it}.json`. Plus `de-CH.json` if `-schweiz` is
run. That is 5–6 files touched by a string change, which is why the plan should
batch the i18n pass as **one task at the end of each wave**, not per template.

**Trap specific to this phase:** the report groups rows and names counts
("17 Zeilen: …"). Do **not** build that sentence in Go with `fmt.Sprintf` — use
`{{tf "%d Zeilen: %s" .Count .Reason}}` so `%d Zeilen: %s` is the catalogue key.
`translation_matrix.html:41` is the existing example: `{{tf "%d fehlen" .Missing}}`.

---

### (d) Admin CSS for a wide table with a per-row status

**One stylesheet only:** `cmd/holzcloud/assets/admin.css` (2732 lines), linked
once at `cmd/holzcloud/templates/admin/base.html:9`. `bausteine.css` is not
loaded by the admin layout. New CSS therefore appends to `admin.css` as its own
`@layer components { … }` block, which is how every recent section was added.

**Closest existing screen: the translation matrix.** `translation_matrix.html`
is pages × languages with a per-cell status — structurally the same problem as
rows × columns with a per-row verdict. Its CSS is `admin.css:2050-2083`, and the
whole block is copyable:

```css
/* ── Übersetzungsraster ──
   Seiten mal Sprachen. Die erste Spalte bleibt beim seitwärts Rollen stehen,
   sonst weiss man nach der dritten Sprache nicht mehr, in welcher Zeile man
   ist. */
@layer components {
    .matrix-scroll {
        overflow-x: auto;
        /* Der Kasten rollt, nicht die Seite: sonst wandert die Navigation mit
           nach links aus dem Bild. */
        max-width: 100%;
    }
    .matrix { min-width: max-content; }
    .matrix th[scope="row"] {
        position: sticky; left: 0; z-index: 1;
        background: var(--color-surface);
        text-align: start; font-weight: 500; white-space: nowrap;
    }
    .matrix-cell { vertical-align: top; white-space: nowrap; }
    .matrix-cell--missing { background: var(--color-warning-subtle); }
}
```

The sticky first column is exactly what a mapping screen with many columns needs,
and `--missing`'s subtle background is the per-cell status idiom.

**Tokens and classes that already exist and must be reused, not re-invented:**

| Class / token | Where | Use in this phase |
|---|---|---|
| `.card` | used throughout `website_list.html` | every panel and screen section |
| `.table`, `.table th/td`, `.table tbody tr:hover` | `admin.css:959-989` | the mapping and report tables |
| `.table-actions` | `:991-1007` | right-aligned row controls; note its comment forbids `display:flex` on a cell |
| `.badge`, `.badge--published`, `.badge--draft`, `.badge--primary` | `:1008-1032` | **the per-row verdict.** `anlegen` → `--published`, `übergehen` → `--draft`, `aktualisieren` → `--primary`. **No new badge modifier is needed**, and adding one should be argued for. |
| `--space-1 … --space-8` | `:71-78` | the 8px scale |
| `--color-warning-subtle`, `--color-success-subtle`, `--color-accent-subtle`, `--color-sunken`, `--color-border`, `--color-text-muted` | tokens layer `:27-136` | row shading by verdict |
| `.callout callout--warning` | used at `import_report.html:12` | the "nothing has been written" notice on the dry-run screen |
| `.text-muted`, `.btn`, `.btn--primary`, `.btn--sm`, `.form-group`, `.form-label`, `.form-input`, `.sr-only` | throughout | forms and controls |
| `.data-table tr.is-unread td` | `:2388-2390` | a precedent for a whole-row state expressed as a `tr` modifier, which is the shape D-25's report wants |
| the mobile table rule | `:432-443` (`.card:has(> .table)`, table scrolls rather than shrinking) | already handles narrow screens; do not re-solve |

**A real gap found, and it is a warning not an invitation:**
`import_report.html:6` and `:26` use `.import-summary` and `.import-warnings`,
and **neither class has a single rule anywhere in `admin.css`** (verified by
grep across `cmd/` and `internal/`). They are dead classes on an unstyled list.
Do not copy that screen's CSS approach — there is none. This is a second,
independent reason D-24's "its own template" is right.

**View transitions:** `CLAUDE.md` and the memory note call for `@view-transition`
on admin navigation. A four-screen flow is the best candidate in the codebase for
`view-transition-name` on the step indicator; the plan should decide explicitly
whether to spend that, since it is the milestone's largest UI surface.

---

### `cmd/holzcloud/templates/admin/website_list.html` — the third panel

**Analog:** its own second panel at `:24-45`. Exact structure to copy — `<details
class="card" style="margin-bottom: var(--space-3);">`, a `<summary>` with a
`{{th}}` bold title, one or two `<p class="text-muted">` explaining the rules,
then the form:

```html
    <form method="POST" action="/admin/websites/import-wordpress" enctype="multipart/form-data">
        <input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">
        <div class="form-group">
            <label class="form-label" for="wxr">{{t "WordPress-Datei (.xml)"}}</label>
            <input type="file" id="wxr" name="wxr" class="form-input" accept=".xml,text/xml,application/xml" required>
        </div>
        …
        <button type="submit" class="btn btn--primary" hx-disabled-elt="this">{{t "Einspielen"}}</button>
    </form>
```

Copy: the hidden CSRF input, `enctype="multipart/form-data"`, `accept=`,
`required`, and `hx-disabled-elt="this"` on the submit button (a `CLAUDE.md`
rule, honoured here).

**What differs (D-05):** the button leads to a *screen*, so its label is not
"Einspielen". And screen 1 needs the existing-vs-new-website choice (IMP-04),
which neither sibling has — that is new markup with no analog; the nearest
control idiom is the radio/select pairs in `page_form.html`.

---

### The four new templates — registration and shape

**`internal/web/render.go:46` `layoutPageNames` is a hand-maintained list.**
Every template that supplies a `{{define "content"}}` block to `base.html` must
be named there. `csv_mapping`, `csv_probe`, `csv_report` and any `csv_upload` go
into it. A template that is omitted parses fine and renders as a fragment with no
layout — it fails silently in the browser and **not** in the test suite unless a
handler test asserts on layout markup. Flag this as its own plan step.

**Data-struct + template pairing:** `newTestAdmin`
(`internal/admin/page_handler_test.go:47-50`) parses the **real templates from
disk**, so a template referencing a field its data struct lacks fails at test
time. That is the safety net; use it by writing a handler test per screen.

**`csv_report.html` has no analog.** D-25's grouped-by-reason shape exists
nowhere. Nearest structural cousins: `import_report.html`'s counters block
(unstyled, see above) and `activity_log.html`. The plan should give this screen
its own task with its own layout decision, as the roadmap's research flag asks.

---

### `internal/admin/csvimport_test.go`

**Analog:** `internal/admin/snippet_fields_test.go:1-60`, which is the newest and
best example of using the `page_handler_test.go` harness.

**What to copy from the harness** (`page_handler_test.go`):

- `newTestAdmin(t)` at `:33` → `(*Handler, *scs.SessionManager, *db.DB,
  *domain.Website)` over a real migrated SQLite in `t.TempDir()` and the real
  on-disk templates.
- `serve(t, h, sm, h.HandleX, req)` at `:75` — runs the handler inside
  `sm.LoadAndSave`, because handlers read and write the session.
- `postForm(target, values, pathValues)` at `:89` — sets the `SetPathValue`
  entries the mux would.
- `seedPage(...)` at `:98` for fixtures.

**What to copy from `snippet_fields_test.go`:**

- The file-head paragraph (`:22-31`) explaining *what this test file is the gate
  on* — for Phase 9 that is: the ownership check on the staging token, and the
  fact that the dry run writes nothing.
- Small German-named per-screen helpers (`feldBildschirm` `:33`, `feldAnlegen`
  `:44`, `zweiteWebsite` `:55`) rather than inline request construction.
- `zweiteWebsite` is the **direct template for the D-06 ownership test**: a
  second website/second user as the counterparty of every authorisation
  assertion.

**The must-have assertions, in the harness's own idiom:** the dry run leaves
`SELECT COUNT(*) FROM pages` unchanged (read back through the store, as
`page_handler_test.go:143-146` reads back after a rejected edit); another admin's
token yields 403/404; a swept token yields the "start again" screen, not a 500.

---

### `cmd/holzcloud/main.go` — the prune job

**Analog:** `:515-521` `token-purge`, which is the shortest of the three and the
closest fit (it wraps a store method returning `(int64, error)`):

```go
		jobs.Job{
			Name:  "token-purge",
			Every: 6 * time.Hour,
			Fn: func(ctx context.Context) error {
				_, err := user.NewStore(database, argon2Params).PurgeExpiredTokens(ctx)
				return err
			},
		},
```

`outbox-prune` (`:498-507`) shows the house style of a **German comment inside
the job** saying what is deliberately *not* pruned:

```go
			Name:  "outbox-prune",
			Every: 24 * time.Hour,
			Fn: func(ctx context.Context) error {
				// Nur Zugestelltes. Ein Fehlschlag bleibt stehen, bis jemand
				// hingesehen hat.
				_, err := outboxStore.Prune(ctx, 90*24*time.Hour)
				return err
			},
```

`csv-import-prune` needs that comment: a *finished* import's staging row and an
*abandoned* one are both dropped after a day, and the sentence should say why a
day is right (an operator who left the tab open over lunch still finds the
import; one who left it over a weekend does not, and re-uploading a file is
cheap). `Every: 6 * time.Hour` matches `token-purge`.

**What NOT to copy:** `media-backfill` (`:453-478`) sets `RunAtStart: true`. A
staging prune should not — sweeping on every restart would kill an import that is
in flight during a deploy.

### `cmd/holzcloud/main_test.go:158-172` — the authorization table

```go
	adminOnly := []struct{ method, path string }{
		{"GET", "/admin/websites/new"},
		{"POST", "/admin/websites/new"},
		{"POST", "/admin/websites/1/delete"},
		…
	}
```

Five rows appended, with a concrete token in the path segment (the table uses
literal `1` for ids; a literal like `abc` for `{token}` is the equivalent). The
loop asserts **403 for the editor and not-403 for the admin** (`:175-182`), so the
handler must return something other than 403 for an admin holding a nonexistent
token — a 404 or a redirect, not a 403. **This constrains the "unknown token"
answer** and is the kind of thing that only shows up when the table row is
written; the plan should decide it up front. Note the comment at `:150-152` that
explains why the table exists — leave it, and do not add a second table.

---

## Shared Patterns

### Flash + redirect for anything an operator caused
**Source:** `internal/admin/wordpress.go:28-32`, `internal/admin/media_crop.go:104-107`
**Apply to:** every refusal on screens 1–4 that is not a form-validation error.
```go
		web.SetFlashError(h.sm, r.Context(), "Datei zu groß oder nicht ausgewählt")
		return h.redirect(w, r, "/admin/websites")
```
For a *form* error that must keep what was typed, `internal/web/forms.go:45`
`RenderFormError` re-renders with **422** — `page_handler_test.go:118-128` is the
test that exists because a flash-and-redirect once discarded a long article. The
mapping screen must use the 422 path, not the flash path.

### CSRF and htmx
**Source:** `website_list.html:9`, `:31`
**Apply to:** every form on screens 1–4.
```html
<input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">
…
<button type="submit" class="btn btn--primary" hx-disabled-elt="this">
```

### Render entry points
**Source:** `internal/web/render.go:251` `RenderAdmin`, `:258` `RenderAdminStatus`
(sets `Vary: HX-Request`), `internal/web/forms.go:45` `RenderFormError`
**Apply to:** all four screens. Note `RenderAdminStatus` already sets `Vary`, so a
handler does not repeat it.

### Data-struct shape
**Source:** `internal/admin/media_crop.go:26-44` — embeds `web.LayoutData`, then
per-screen members each carrying a comment saying *why the server has to compute
it*. `internal/admin/confirm.go:18-23` additionally embeds `web.FormState` for a
screen that can be rejected. Screens 2 and 3 need both embeds.

### Terms: name in, slug derived
**Source:** `internal/bundle/import.go:289-329` (`importTerms`, `EnsureNames`
before any page) and `:573-583` (the field value stored as `page.Slugify(val)`),
with `internal/term/store.go:66` `Normalize` and `:329` `EnsureNames`.
**Apply to:** D-20. Copy the ordering — collect and `EnsureNames` once before the
row loop — and copy the comment explaining why the two derivations agree by
construction.

### Multi-value encoding
**Source:** `internal/field/field.go:973` `JoinValues`, whose doc comment
(`:965-972`) names this caller in as many words.
**Apply to:** every multi-value cell. Never build the newline string directly.

### Key matching
**Source:** `internal/field/field.go:876` `SlugifyKey` — folds `ä ö ü ß`,
lowercases, collapses space/`-`/`_` to `_`.
**Apply to:** D-17. `SlugifyKey(header) == def.Key || SlugifyKey(header) ==
SlugifyKey(def.Label)`.

---

## No Analog Found

| File / concern | Role | Why there is no analog |
|---|---|---|
| **The four-screen flow itself** | handler | **No multi-screen wizard exists anywhere in the tree.** §A above, searched exhaustively. `twofactor.go:138-207` (one POST → a second screen) and `confirm.go:27` (GET+POST on one handler) are the only half-analogs, and neither carries a payload forward. |
| `templates/admin/csv_report.html` | template | D-25's group-by-reason report has no precedent. `import_report.html` is a flat `<ul>` whose two CSS classes are not styled at all. |
| Screen 1's existing-vs-new-website choice | template | No importer offers it; `wordpress.go:15-19` explicitly rules it out. IMP-04 overrules that, and the control is new markup. |
| A `POST` that returns a file download | handler | All five existing downloads are GET (`bundle.go:40`, `language.go:71,:81`, `template.go:358`, `plugin.go:358`). D-06's `POST …/beispiel` is a departure. |
| A staging table keyed to a **user** rather than a website | migration | Only `user_tokens` (`00012:12`) is; every other table cascades from `websites`. `00012` is the analog, but it is a lone one. |

---

## Metadata

**Analog search scope:** `internal/` (all 35 packages, with targeted reads in
`admin`, `wxr`, `user`, `sharelink`, `shop`, `public`, `field`, `page`, `term`,
`web`, `db/migrations`), `cmd/holzcloud/` (`main.go`, `main_test.go`, `assets/`,
`templates/admin/`), `plugins/kontaktformular/`, `tools/i18n/`.
**Files read in full or in targeted ranges:** 34
**Pattern extraction date:** 2026-09-06
