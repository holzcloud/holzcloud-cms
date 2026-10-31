# Architecture Research — v1.6 Integration Map

**Domain:** Multi-site self-hosted CMS (Go + htmx + plain CSS + SQLite), mature codebase
**Milestone:** v1.6 "Inhaltsmodell und Zugang" — Phases 6–10
**Researched:** 2026-09-03
**Confidence:** HIGH — every claim below was read out of the working tree at commit `c58ceb0`; the two claims that could only be settled by running something (the i18n writer's output, the plugin-wasm skips) were settled by running it.

> **Mode:** this is *integration* research, not greenfield. Every section names the file
> and, where one applies, the line. New-vs-modified is explicit. Build order inside each
> phase respects real dependencies.

---

## 0. Corrections to the existing map

`.planning/codebase/ARCHITECTURE.md` (dated 2026-08-22) has drifted. Verified against the tree:

| Claim in the old map | Reality |
|---|---|
| "38 goose `.sql` files" | **45** (`00001`…`00045_pages_locale_unique.sql`) |
| "`newRouter`, line ~572" | **`cmd/holzcloud/main.go:630`** (`routerDeps` is at `:561`) |
| "`main()` — `cmd/holzcloud/main.go:59`" | **`:64`** |
| "`internal/admin/` (47 files)" | **49** non-test `.go` files (55 with tests) |
| "Admin templates ... `templates/admin/`" | actually **`cmd/holzcloud/templates/admin/`**; assets are `cmd/holzcloud/assets/` |
| Admin chain description | correct in order, exact line is **`main.go:968`** |

`docs/offene-punkte.md` also says "Migrationen laufen bis `00044`" — they run to `00045`.
`.planning/ROADMAP.md` and `.planning/REQUIREMENTS.md` are still written for a milestone
called **v1.5 with phases 6–8**, while `.planning/PROJECT.md` is on **v1.6 with phases 6–10**.
That inconsistency *is* part of Phase 6's scope.

---

## 1. System overview — where v1.6 attaches

```
┌───────────────────────────────────────────────────────────────────────────┐
│  cmd/holzcloud/main.go — one binary, one route table (newRouter, :630)     │
│                                                                           │
│  RequestID(:1069) → AccessLog(:1068) → Recoverer → SecureHeadersWith(:1066)│
│                          → sm.LoadAndSave                                  │
│      │                                    │                    │           │
│      ▼ /admin/ (:968)                     ▼ / (:1055)          ▼ /ai(:696) │
│  AdminHeaders → csrf → setupGuard    domainResolver →      ai.Server       │
│    → requireAuth → requireSecondFactor  LocaleMiddleware   (Bearer token,  │
│    → withLang → requireWebsite          → PluginMiddleware  no session,    │
│    → withNav → adminProtectedMux        → ShopRoutes        no CSRF)       │
│                                          → publicMux            ▲          │
│         ▲ PHASE 10 inserts here                                 │          │
│         │ (forward-auth, between setupGuard and requireAuth)    │ PHASE 10 │
│         │                                                       │ precedent│
├─────────┴───────────────────────────────────────────────────────┴──────────┤
│  Domain packages                                                           │
│   internal/field  ◄── PHASE 7 (kinds)   internal/snippet ◄── PHASE 8       │
│   internal/term   ◄── PHASE 7 (chooser) internal/wxr     ◄── PHASE 9 model │
│   internal/user   ◄── PHASE 10 (rights) internal/auth    ◄── PHASE 10      │
├────────────────────────────────────────────────────────────────────────────┤
│  SQLite dual pool (write 1 conn, read 5), WAL — internal/db/db.go           │
│  page_field_defs is the ONE field-definition table (00028/00029/00037/00038)│
└────────────────────────────────────────────────────────────────────────────┘
```

The single load-bearing structural fact of this milestone: **`page_field_defs` is already a
three-namespace table** — page fields (`parent_id IS NULL AND block_type_id IS NULL`), group
sub-fields (`parent_id IS NOT NULL`), and block-kind fields (`block_type_id IS NOT NULL`).
Phase 8 adds a fourth namespace, and migration `00038` is a line-for-line template for how.

---

## 2. Phase 6 — Aufräumen

### 2.1 `writeCatalog`: the premise is false, and that is the finding

**Where.** `tools/i18n/main.go` — doc comment at **:287–288**, `func writeCatalog` at
**:289**, body **:290–309**. It sorts the keys (`:294`), writes `{\n` (`:297`), then for each
key writes `quote(k)` (`:299`) **at column zero**, `": "` (`:300`), `quote(v)` (`:301`), a
comma unless last (`:302–304`), a newline, and closes with `}\n` (`:307`). `quote` (`:277–285`)
is `json.Encoder` with `SetEscapeHTML(false)` and the trailing newline trimmed.

**What it emits:** flush-left, one key per line, sorted byte-wise, no indentation,
HTML characters unescaped, trailing newline at EOF.

**What is committed — measured, not assumed:**

| File | Lines | Lines matching `^  "` | Sorted | Ends `\n` |
|---|---|---|---|---|
| `de-CH.json` | 53 | **0** | yes | yes |
| `en.json` | 1130 | **0** | yes | yes |
| `es.json` | 1130 | **0** | yes | yes |
| `fr-CH.json` | 6 | **0** | yes | yes |
| `fr.json` | 1130 | **0** | yes | yes |
| `it-CH.json` | 11 | **0** | yes | yes |
| `it.json` | 1130 | **0** | yes | yes |

**No catalog is committed with two-space indent. None.** And running the tool proves the
output already matches byte-for-byte:

```
$ go run ./tools/i18n            → 1128 Zeichenketten; en/es/fr/it: 1128 übersetzt, 0 offen, 0 verwaist
$ go run ./tools/i18n -write     → git diff internal/i18n/locales  ⇒ EMPTY
$ go run ./tools/i18n -schweiz   → git diff internal/i18n/locales  ⇒ EMPTY
```

**So the smallest change that makes the output match the committed files is: none.**
What is actually wrong is the **doc comment at `tools/i18n/main.go:287`**, which claims the
file is written "sorted and **indented**". It is not indented. Two honest options:

- **(a) Fix the comment (one line).** Delete the word "indented" / replace with "flush-left,
  one key per line". Zero risk, zero diff in the catalogs.
- **(b) Actually indent (one line + a reflow commit).** Insert `b.WriteString("  ")` before
  `:299`, then run `-write` once and commit the seven reflowed files as a separate,
  whitespace-only commit. Cost: ~5 500 changed lines, no behaviour change. `readCatalog`
  (`:258–268`) is `json.Unmarshal`, so it reads either form.

**Recommendation: (a).** Nothing downstream reads these files as text — `readCatalog` parses
JSON, and the runtime loader embeds them. The indent would buy readability only, at the price
of a 5 500-line diff that will hide the next real change.

**One genuine gap worth folding into Phase 6:** the writer never touches `fr-CH.json` and
`it-CH.json`. `main` (`:104–114`) `continue`s past any filename containing a `-` *before*
reaching the `-write` block (`:148–152`), and `-schweiz` only rebuilds `de-CH.json`
(`:83–87`, `writeSwiss` at `:228–256`). Those two files are hand-maintained and only ever
checked. That is deliberate and documented (`:99–103`) — but it is not documented anywhere a
planner would find it, and a phase that "makes the writer's format match" would otherwise be
tempted to start writing them.

### 2.2 The silent test skips

Five tests skip themselves when a build artefact is missing:

| File:line | Missing artefact |
|---|---|
| `internal/plugin/sdk_e2e_test.go:23` | `plugins/jahreszahl/plugin.wasm` |
| `internal/plugin/hofladen_e2e_test.go:25` | `plugins/bestellung/plugin.wasm` |
| `internal/plugin/runtime_test.go:26` | `internal/plugin/testdata/echo.wasm` |
| `internal/public/formular_e2e_test.go:160` | `plugins/kontaktformular/plugin.wasm` |
| `internal/public/suche_e2e_test.go:28` | `plugins/suche/plugin.wasm` |

**Verified: all five currently run and pass.** All six `.wasm` files are tracked by git
(`git ls-files | grep wasm`), so today the skip never fires:

```
--- PASS: TestHofladenLaeuftDurch (3.20s)
--- PASS: TestBeispielPluginLaeuftDurch (0.79s)
--- PASS: TestKontaktformularPluginNimmtNachrichtenAn (1.17s)
--- PASS: TestSuchePluginBeantwortetSuche (0.79s)
```

The real risk is **staleness, not absence**: the `.wasm` blobs are committed build products
of `plugins/*/main.go`, and nothing rebuilds them. Editing `plugins/suche/main.go` and
running `go test ./...` tests the *old* module and reports PASS.

**Is there an established way to make a skip loud? No — there is nothing to copy.** Verified:

- **No `Makefile`, no `justfile`, no `Taskfile`, no `scripts/`.** The only shell scripts in
  the repo are `deploy/backup.sh` and `deploy/restore.sh`.
- **No CI step builds the plugins.** `.github/workflows/ci.yml` runs gofmt → `go mod tidy`
  → `go vet ./...` → `go build ./cmd/holzcloud` → `go test ./...` → upload artefact. Nothing
  enters `plugins/`.
- **No build tag** guards any of the five tests.
- The **only precedent** is a `//go:generate` line at `internal/plugin/runtime_test.go:13`,
  for `testdata/echo.wasm` only — and its own comment (`:19–21`) states the project's position:
  *"Es liegt gebaut im Repository, damit die Tests ohne zweite Werkzeugkette laufen — ein
  Test, der einen Compiler-Lauf braucht, wird irgendwann übersprungen und dann nie wieder
  ausgeführt."*
- The build command is documented in prose only: `plugins/README.md` §"Ein Plugin bauen".

**Recommended shape (this phase invents the convention, so pick one and write it down):**
add a `plugins/build.sh` that loops the five directories with the README's exact command,
plus a CI step `- name: Build plugins` before `go test`, plus an env switch — e.g.
`HOLZCLOUD_TEST_REQUIRE_WASM=1` — that turns each `t.Skipf` into `t.Fatalf`, set in CI.
That makes the skip loud in CI and forgiving locally, which is the property the existing
comment asks for.

### 2.3 Stale planning notes

- `.planning/ROADMAP.md` and `.planning/REQUIREMENTS.md` describe **v1.5 / phases 6–8**;
  `.planning/PROJECT.md` describes **v1.6 / phases 6–10**. The old Phases 6/7/8 became 7/8/9.
- `.planning/codebase/ARCHITECTURE.md` — the six drifted facts in §0 above.
- `docs/offene-punkte.md` — "Migrationen laufen bis `00044`" (now 45); point 7 (Dependabot)
  is fully closed and could move to a "done" section; points 1, 2, 3, 4 and 6 are the
  substance of Phases 7–9 and should be cross-referenced, not duplicated.

### Phase 6: new vs modified

| | |
|---|---|
| **NEW** | `plugins/build.sh` (or a `Makefile`); a CI step that runs it; an env switch that promotes the five skips to failures |
| **MODIFIED** | `tools/i18n/main.go:287–288` (comment); `.planning/ROADMAP.md`; `.planning/REQUIREMENTS.md`; `.planning/codebase/ARCHITECTURE.md`; `docs/offene-punkte.md`; the five `_test.go` skip sites |
| **UNCHANGED** | `writeCatalog` itself, and all seven catalogs |

**Build order:** ① planning-doc corrections (no code risk, unblocks the roadmapper) →
② `tools/i18n` comment → ③ plugin build script → ④ CI step → ⑤ skip-promotion switch
(needs ③ and ④ to exist first, or CI goes red on the same commit that introduces it).

---

## 3. Phase 7 — Field Kinds

### 3.1 How a kind is declared and dispatched — five places, all Go-side

A field kind is a plain string constant. Adding one means touching exactly these:

| # | Where | Line | What |
|---|---|---|---|
| 1 | `internal/field/field.go` | `26–49` | the `Kind*` constant |
| 2 | `internal/field/field.go` | `70–82` | an entry in `Kinds` (the admin menu, with `i18n.N` label + hint) |
| 3 | `internal/field/field.go` | `507–553` | a `case` in `Check` (validation, German user-facing message) |
| 4 | `internal/field/render.go` | `102–158` | a `case` in `Resolve` (typed value for the theme); `default` at `:156` returns the raw string |
| 5 | `cmd/holzcloud/templates/admin/field_input.html` | `24–63` | an `{{else if .Is "…"}}` branch |

Plus, as needed: `SubKinds()` (`:87–95`, excludes group+section), `BlockKinds()` (`:105–115`,
also excludes `verweis`), `Def.MayControl()` (`:212–218` — decides whether other fields may
hang on this one), `admin.switchOf` (`internal/admin/page_fields.go:186–195`),
`admin.oneView` (`:211–229` — only `janein`, `bild`, `verweis` need per-kind view work),
`field.List`'s type switch (`internal/field/render.go:214–245`) and `field.Filled` (`:253–287`).

**Migration needed? No.** Confirmed by reading the SQL:
`internal/db/migrations/00029_field_groups.sql:32` declares `art TEXT NOT NULL` with **no
CHECK**, and `00028_page_fields.sql:25–27` states the reason outright: *"art ist eine der acht
Eingabearten. **Kein CHECK**: eine neue Art wäre sonst wieder ein Tabellenumbau, und geprüft
wird ohnehin beim Speichern."* The gate is `KnownKind` (`field.go:139–146`), enforced in
`validate` (`store.go:419–421`). **Phase 7 needs zero migrations.**

`Choices` is likewise free: the column is `auswahl TEXT NOT NULL DEFAULT ''` (`00029:36`),
read by `SplitChoices` and written by `JoinChoices`.

### 3.2 `SplitChoices` — location and shape

`internal/field/field.go:613–622`:

```go
func SplitChoices(raw string) []string   // splits on "\n", TrimSpace, drops empties
func JoinChoices(choices []string) string // strings.Join(choices, "\n")   :625
```

Round-trip points: `store.scanDef` sets `d.Choices = SplitChoices(auswahl)`
(`internal/field/store.go:166`); `store.Create`/`Update` write `JoinChoices(d.Choices)`
(`:242`, `:284`); the admin form reads `field.SplitChoices(r.FormValue("auswahl"))`
(`internal/admin/field.go:126`) from the textarea at
`cmd/holzcloud/templates/admin/field_list.html:132–138`.

**Reuse it for `bereich`.** A range field needs a lower and an upper bound and has nowhere
else to put them without a migration. Two lines in `auswahl` (`min`, `max`) read by
`SplitChoices` costs nothing and needs no schema change. `validate` (`store.go:405–485`)
gains a bounds check beside the existing `KindChoice` one at `:422–424`.

### 3.3 How `field.Data` stores a value, and what a list costs

```go
type Values map[string]string                     // field.go:360
type Data struct {                                // field.go:367–371
    Values Values                `json:"werte,omitempty"`
    Rows   map[string][]Values   `json:"gruppen,omitempty"`
}
```

Stored as JSON in one column, `pages.fields` (`00028:50`). `Decode` (`:389–412`) tolerates
both the current shape and the pre-groups flat shape; `Encode` (`:416–425`) returns `""`
when empty. `trimTo` (`:494–500`) caps a value at `MaxValueBytes = 4000` (`:159`).

**A multi-value is genuinely the first value that is not one string, and there are exactly
three candidate designs.** The seams that constrain the choice:

- **`field.Values` is `map[string]string`** — changing it to `map[string][]string` is a
  breaking change across `internal/bundle` (export `:404–427`, `:431`; import `:413–429`,
  `:438`), `internal/ai/tools.go` (`:386–387`, `:477–478`, `:411`, `:517`),
  `internal/public/pluginhost.go:145`, `internal/public/pagedata.go:71` and every admin path.
- **`Hidden` (`field.go:256–284`) reads `values[d.Condition]` as a string** and treats `""`
  and `"0"` as empty.
- **`fieldsFromRequest` (`internal/admin/page_form.go:141–177`) takes `values[0]` and throws
  the rest away** — at `:150` for a top-level field, `:163` for a group sub-field. This is the
  single line where a browser's repeated `feld_x=a&feld_x=b` is currently lost.
- **`Clean` (`:433–479`) and `cleanRow` (`:484–492`) drop anything that trims to `""`.**

**Recommended: newline-joined inside the existing string.** This is what
`docs/offene-punkte.md` point 1 proposes and it is right — it reuses the encoding
`SplitChoices` already reads, keeps `Values` a `map[string]string`, keeps the stored JSON
shape, and leaves bundle/AI/plugin paths untouched. Concretely:

1. `field.SplitValues(raw) []string` and `field.JoinValues([]string) string` beside
   `SplitChoices`/`JoinChoices` at `field.go:613–625` (same body, different name so the
   two concepts can diverge later).
2. `page_form.go:150` becomes `out.Values[key] = field.JoinValues(values)` — one line;
   `:163` the same for group rows.
3. `Check` gains a `KindMultiChoice` case that splits and validates each line against
   `d.Choices`.
4. `Resolve` gains a case returning `[]string` — a theme then writes
   `{{range .Page.Felder.sorten}}`.
5. `Entry` (`render.go:169–187`) gains `Values []string`; `List`'s type switch (`:214–245`)
   gains a `case []string`; `Filled` (`:253–287`) gains the same.
6. `MaxValueBytes` (4000) now bounds the joined list, which is the right unit.

**Decide this before Phase 9 starts** — a CSV column mapped to a multi-choice field must
write exactly this encoding.

### 3.4 The reference chooser, end to end — the template for `KindTerm`

**Admin side (choose):**

| Step | File:line |
|---|---|
| `PageChoice{ID,Title,Slug,Draft}` | `internal/admin/page_fields.go:98–105` |
| `refPages()` — this website's pages, `Locale:"*"`, `Sort:"title"`, `PerPage:500` | `:238–255` |
| `PageFormData.RefPages` field | `internal/admin/page.go:79–81` |
| filled once per form render | `internal/admin/page.go:339` |
| carried into `pool{media,pages}` | `page_fields.go:112–118` |
| `oneView` sets `v.Pages`, `v.PageID`, `v.RefDraft` | `:219–226` |
| `switchOf("verweis") == "auswahl"` | `:186–195` |
| the `<select>` | `field_input.html:49–54` |
| validation: "is it a positive int" only | `field.go:539–546` |

**Public side (resolve):**

| Step | File:line |
|---|---|
| `type Ref{Title,URL,Kind}` | `internal/field/render.go:36–44` |
| `type RefLookup func(id int64)(Ref,bool)` | `:52` |
| `type Links{Image Lookup; Page RefLookup}` | `:57–60` |
| built per request | `internal/public/pagedata.go:72` |
| `fieldRefs` — exists **and** published **and** this website **and** not password-gated | `:84–114`, checks at `:94–95` |
| consumed in `Resolve` | `render.go:140–154`; nil target ⇒ `(*Ref)(nil)` so `{{with}}` skips |
| admin's parallel image lookup (the caching idiom) | `internal/admin/page_fields.go:333–357` |

**`KindTerm` copies this exactly, with two simplifications:** a term has no draft state and
no publish gate, so `TermLookup` only has to check that the term belongs to this website.
The source list already exists: `term.Store.ListAll(ctx, websiteID)`
(`internal/term/store.go:234–256`) returns `[]Term{ID,Slug,Name,Count}` ordered
`name COLLATE NOCASE`. `Term.URL()` is `/tag/<slug>` (`:34`), which is what the theme should
link to. `h.terms` is already on the admin handler (`internal/admin/handler.go:50`) and
`h.termStore` on the public handler (`internal/public/pagedata.go:138–148`), so **no new
wiring in `main.go` is required.**

`Links` grows a third member — `Term TermLookup` — and every construction site must set it:
`internal/public/pagedata.go:72` (public) and any admin/preview path that builds `Links`.
Passing `nil` degrades to "resolves to nothing", the documented contract at `render.go:55–56`.

### 3.5 The trap nobody will see coming: conditional-field CSS

`Def.MayControl()` (`field.go:212–218`) decides which fields may carry dependants, and
`admin.switchOf` (`page_fields.go:186–195`) names the CSS rule. The rules live in
**`cmd/holzcloud/assets/admin.css:1096–1110`**:

```css
.feld-schalter--kreuz:has(> .form-group input[type="checkbox"]:checked) > .feld-abhaengig
.feld-schalter--auswahl:has(> .form-group option[value=""]:checked)    > .feld-abhaengig
.feld-schalter--text:has(> .form-group input:placeholder-shown), … textarea:placeholder-shown
```

Consequences for the new kinds, all script-free:

- **Button row (FIELD-01).** `switchOf` currently returns `"auswahl"` for `KindChoice`
  (`:190`), and the `--auswahl` rule requires an `<option>`. Radio inputs have none, so a
  Choice rendered as a button row **silently breaks any field that hangs on it**. Either add
  a `.feld-schalter--knopfreihe` rule keyed on an explicit empty radio, or return a new
  switch name from `switchOf`. Either way it is a CSS + `switchOf` change, not a Go-model one.
- **Multiple choice (FIELD-02).** Checkboxes: `--kreuz` matches `input[type=checkbox]:checked`
  and would fire on *any* box in the group, which is arguably correct ("something is chosen").
  Verify in the browser; QUAL-02 exists for this.
- **`bereich` (FIELD-05).** `<input type=range>` always has a value and never shows a
  placeholder, so it can never read as "empty". **`MayControl()` must return `false` for it**,
  exactly as it already does for `KindDate` (`:214`). Otherwise a dependent field is
  permanently visible with no explanation.
- **`code` (FIELD-06).** Must never touch `goldmark`. The `default` branch of `Resolve`
  (`render.go:156–157`) already returns the raw string and `html/template` escapes it on
  output — so a `<script>` typed into a Code field prints verbatim and cannot execute, which
  is the success criterion, and it comes for free. The only work is a monospace `<textarea>`
  branch in `field_input.html` and a `.form-input--code` class.
- **`zeit` (FIELD-04).** `<input type="time">`, `Check` parses `15:04`, `Resolve` may keep it
  a string (a theme formats it) — do **not** return a `*time.Time`, because `formatDate` in
  the theme contract expects a date.

### 3.6 The theme contract is test-enforced

Anything added to `field.Entry` or to the `Page`/`Site` structs must also land in the
fixtures and the spec, or the suite fails:

- structs: `internal/template/loader.go` (`Felder` at `:461`, `Feldliste` at `:467`)
- fixtures: `internal/template/sample.go:74–81` (`SampleData`) and `MinimalData`
- spec: `internal/tmplspec/TEMPLATE-SPEC.md` (`.Page.Felder` §236, `.Page.Feldliste` §237, §523–544)
- the tie: `internal/template/sample_test.go:17` (`TestSampleDataFillsEveryField`),
  `:83` (`TestMinimalDataLeavesOptionalFieldsEmpty`),
  `internal/tmplspec/spec_test.go:18` (`TestSpecDocumentsEveryFieldOfTheContract`, by reflection)

### Phase 7: new vs modified

| | |
|---|---|
| **NEW** | `field.KindMultiChoice`, `KindTerm`, `KindTime`, `KindRange`, `KindCode` constants; `field.SplitValues`/`JoinValues`; `field.TermRef` + `field.TermLookup`; `admin.TermChoice` + `Handler.refTerms`; `.feld-schalter--knopfreihe` CSS; a "Darstellung" toggle for Choice |
| **MODIFIED** | `field/field.go` (`Kinds`, `Check`, `MayControl`, `Entry`-adjacent helpers); `field/render.go` (`Links`, `Resolve`, `Entry`, `List`, `Filled`); `field/store.go:405–485` (`validate` bounds for `bereich`); `admin/page_fields.go` (`FieldView`, `oneView`, `switchOf`, `pool`); `admin/page.go:79–81,:339`; `admin/page_form.go:150,:163`; `admin/field.go:126`; `field_input.html`; `field_list.html:121–138`; `admin.css:1096–1110`; `public/pagedata.go:72`; `template/loader.go`, `sample.go`, `TEMPLATE-SPEC.md` |
| **NO MIGRATION** | `art` has no CHECK (`00028:25–27`, `00029:32`) |

**Build order (dependencies are real):**

1. **Multi-value encoding first.** `SplitValues`/`JoinValues`, `page_form.go:150`,
   `Values []string` on `Entry`, `Resolve`/`List`/`Filled`. Everything else and all of
   Phase 9 depend on this decision being settled.
2. **`zeit`, `bereich`, `code`** — an hour each, no new plumbing. `bereich` carries the
   `MayControl() == false` fix and the `auswahl`-as-bounds decision.
3. **`KindMultiChoice`** — first consumer of step 1; checkbox markup + `--kreuz` verification.
4. **Choice as a button row** — the `switchOf` + CSS change; last of the choice work so the
   conditional-field rule is only touched once.
5. **`KindTerm`** — copies §3.4 wholesale; independent of 1–4, can run in parallel.
6. **Contract + i18n + browser gate** — fixtures, `TEMPLATE-SPEC.md`,
   `go run ./tools/i18n` ⇒ `0 offen, 0 verwaist`, and a browser pass per kind.

---

## 4. Phase 8 — Snippets Carry Fields

### 4.1 What a snippet stores today

Table `snippets` — `internal/db/migrations/00010_scheduling_search_redirects.sql:34–44`:

```sql
CREATE TABLE snippets (
    id, website_id REFERENCES websites(id) ON DELETE CASCADE,
    key TEXT NOT NULL, name TEXT NOT NULL,
    content_markdown TEXT NOT NULL DEFAULT '',
    content_html     TEXT NOT NULL DEFAULT '',
    created_at, updated_at,
    UNIQUE (website_id, key)
) STRICT;
```

`internal/snippet/store.go`: `Snippet` struct `:31–41`; column list `:51`; `scan` `:53–63`;
`Create(ctx, websiteID, key, name, markdown, html)` `:98–110`;
`Update(ctx, id, key, name, markdown, html)` `:113–128`; `Rendered{HTML, LatestUpdate}`
`:145–151`; `LoadRendered` `:155–178`; `Expand` `:202–215`. Admin screen:
`internal/admin/snippet.go` — `HandleSnippetList` `:80–120`, `snippetListData` `:122`,
`handleSnippetSave` `:145`, template `cmd/holzcloud/templates/admin/snippet_list.html`.

### 4.2 The index question, answered from `00029` and `00038`

Current state of `page_field_defs`, after applying all four migrations:

| Index | Definition | Introduced |
|---|---|---|
| `idx_page_field_defs_website` | `(website_id, parent_id, position)` | `00029:53` |
| `idx_page_field_defs_kennung_oben` | UNIQUE `(website_id, kennung) WHERE parent_id IS NULL AND block_type_id IS NULL` | created `00029:58–59`, **replaced** `00038:52–56` |
| `idx_page_field_defs_kennung_gruppe` | UNIQUE `(parent_id, kennung) WHERE parent_id IS NOT NULL` | `00029:60–61` |
| `idx_page_field_defs_kennung_baustein` | UNIQUE `(block_type_id, kennung) WHERE block_type_id IS NOT NULL` | `00038:58–60` |

**It is an index swap, not a table rebuild — and `00038` already proved it.** The v1.5
planning note that named `idx_page_field_defs_kennung_oben` as
`(website_id, kennung) WHERE parent_id IS NULL` was reading `00029` and missing `00038`,
which had already loosened it once for exactly this reason. `00038:44–46` says so in the
file: *"Die Eindeutigkeit wird neu gezogen. Sie steht seit 00029 als eigener Index da und
nicht am Tabellenkopf — deshalb ist das hier ein **Austausch von zwei Indizes und kein
Tabellenneubau**."*

**Migration `00046_snippet_fields.sql` is a transcription of `00038:41–60`:**

```sql
-- MUST default to NULL: SQLite refuses ALTER TABLE ADD COLUMN with REFERENCES
-- and any other default. NULL = "belongs to the page, not to a snippet",
-- which is true of every existing row. (00038:36–40)
ALTER TABLE page_field_defs
    ADD COLUMN snippet_id INTEGER REFERENCES snippets(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;
CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung)
    WHERE parent_id IS NULL AND block_type_id IS NULL AND snippet_id IS NULL;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_baustein_snippet
    ON page_field_defs(snippet_id, kennung) WHERE snippet_id IS NOT NULL;
```

And a `+goose Down` that mirrors `00038:62–69`.

For the *values*: the cheap, symmetric move is `ALTER TABLE snippets ADD COLUMN fields TEXT
NOT NULL DEFAULT ''` — exactly what `00028:50` did to `pages`. `snippets` is `STRICT`, but
`ADD COLUMN` with a constant default is legal on a STRICT table (`00033:20` does it to a
STRICT-adjacent table; `00037:22` does it to `page_field_defs` itself, which *is* STRICT).

### 4.3 What `snippet_id` collides with in Go

**`internal/field/store.go:53` — `WHERE website_id = $1 AND block_type_id IS NULL`.** This is
the page-fields query and it does **not** exclude snippet fields. Without `AND snippet_id IS
NULL` every snippet field would appear in the page editor and in every theme's
`.Page.Feldliste`. This one line is the highest-consequence edit of the phase.

Other collision points, all in `internal/field/store.go`:

| Line | Today | Needs |
|---|---|---|
| `:48–81` `List` | page fields | `AND snippet_id IS NULL` at `:53` |
| `:84–104` `Sub` | `WHERE website_id AND parent_id = $2` | fine as-is (a sub-field inherits its parent's namespace) |
| `:107–127` `OfBlockType` | pattern to copy | add `OfSnippet(ctx, websiteID, snippetID)` |
| `:133–153` `OfBlockTypes` | pattern to copy | add `OfSnippets(ctx, websiteID)` for one-query loading |
| `:155–168` `scanDef` | 13 columns | scan `COALESCE(snippet_id,0)` into a new `Def.SnippetID` |
| `:216–219` `Create` count | `COUNT(*) WHERE website_id` | `MaxFields = 60` (`field.go:153`) would now be shared between pages and snippets — decide whether to scope the count per namespace |
| `:234–242` `Create` INSERT | `$11 = block_type_id`; position via `COALESCE(parent_id,0)`+`COALESCE(block_type_id,0)` | add `snippet_id` as `$12` and to the position sub-select |
| `:258–290` `Update` | preserves `ParentID`/`BlockTypeID` (`:265–266`) | preserve `SnippetID` too |
| `:353–403` `Move` | switches on `ParentID`/`BlockTypeID` (`:360–368`) | fourth case for `SnippetID` |
| `:405–485` `validate` | `BlockTypeID > 0` clears `Condition`, forces `Required=false`, `AppliesTo=ForBoth` (`:446–459`) | a snippet field **can** be required and has no `gilt_fuer` — clear `AppliesTo`, keep `Required` |

`field.Def` (`internal/field/field.go:162–189`) gains `SnippetID int64` beside `BlockTypeID`
(`:183`) with the same comment shape.

The admin screen already has a three-mode shape to extend:
`internal/admin/field.go:217–260` (`fieldListData`) switches on `blockType != nil` /
`group != nil` / default, choosing the query, the title and the kind list. A fourth case
`snippet != nil` slots straight in, with `field.SubKinds()` as its kind list (a group inside a
snippet is over-engineering; a section is arguably fine). `fieldPath` (`:264–274`) gains
`?baustein=` sibling `?textbaustein=`. `FieldListData` (`:21–45`) gains `Snippet *snippet.Snippet`
and `Simple()` (`:64`) is extended.

### 4.4 How a theme reads a page's fields — and the smallest snippet equivalent

```
public.pageContent (pagedata.go:30–51)
  └─ ownFields (:58–74)
       ├─ h.fieldStore.List(ctx, websiteID)        :62
       ├─ field.For(defs, pg.KindValue())          :70
       ├─ field.Decode(pg.Fields)                  :71
       ├─ field.Links{Image: fieldImages, Page: fieldRefs}  :72
       └─ field.Resolve(...) , field.List(...)     :73
  → tmpl.PageContent{Felder: …, Feldliste: …}      :48–49
```

Snippets today reach a theme only as HTML: `Site.Snippets map[string]template.HTML`
(`internal/template/loader.go:372–377`), filled from `loadSnippets`
(`internal/public/pagedata.go:187–198` → `snippet.LoadRendered`) at nine call sites
(`public/handler.go:186–187,:272–273`, `access.go:72–73,:178–179`, `archive.go:48–49`,
`feed.go:76`, `search.go:34–35`, `cart.go:74`, `checkout.go:267,:332`, `shop.go:159,:218`,
`pluginhost.go:180`).

**Smallest surface that lets a theme read a snippet's fields the same way:** a second map
beside the first, not a change to the first.

```go
// internal/template/loader.go, beside Snippets at :377
// Bausteinfelder are each snippet's own fields by key, resolved the same way
// .Page.Felder is: {{ index .Site.Bausteinfelder "kontakt" }}.
Bausteinfelder map[string]map[string]any
Bausteinliste  map[string][]field.Entry
```

Filled inside `loadSnippets` so all nine call sites get it for free — extend
`snippet.Rendered` (`store.go:145–151`) with the raw `fields` JSON per key, resolve in
`pagedata.go` with the *same* `field.Links` the page uses, and set both maps beside
`site.Snippets`. Rationale for a parallel map rather than turning `Snippets` into a struct:
`Site.Snippets` is a published contract (`TEMPLATE-SPEC.md:212`, §649–653) that user templates
already index with `{{index .Site.Snippets "footer-kontakt"}}`; changing its element type
breaks every installed theme. Both new maps must be added to `sample.go`/`MinimalData` and
`TEMPLATE-SPEC.md` or `internal/tmplspec/spec_test.go:18` fails.

**Sanitisation (SNIP-03) is free if nothing new is written.** A snippet's long-text field
value goes through the *same* `field.Resolve` `default` branch as a page's, and the same
`html/template` escaping. The Markdown body keeps its existing
goldmark → bluemonday → `template.HTML` path (`store.go:170–172` documents why the cast is
safe). No second pipeline — that is the criterion, and the way to meet it is to add no
rendering code at all.

Also touch: `internal/bundle/format.go:39` (`Snippets`), `:48`/`:81` (`Fields`) and the
importer at `internal/bundle/import.go:298–370` — a snippet's fields must travel in a bundle,
or an export/import round-trip silently drops them.

### Phase 8: new vs modified

| | |
|---|---|
| **NEW** | `internal/db/migrations/00046_snippet_fields.sql`; `field.Store.OfSnippet`/`OfSnippets`; `field.Def.SnippetID`; `snippet.Snippet.Fields`; `Site.Bausteinfelder` + `Site.Bausteinliste`; the snippet-field admin sub-screen |
| **MODIFIED** | `field/store.go` (`:53`, `scanDef`, `Create`, `Update`, `Move`, `validate`); `field/field.go:162–189`; `snippet/store.go` (`columns :51`, `scan`, `Create`, `Update`, `Rendered`, `LoadRendered`); `admin/snippet.go`; `admin/field.go:217–274`; `snippet_list.html`; `field_list.html`; `public/pagedata.go` (`ownFields`, `loadSnippets`); `template/loader.go`, `sample.go`, `TEMPLATE-SPEC.md`; `bundle/format.go`, `export.go`, `import.go` |

**Build order:**

1. **Migration `00046`** — column + index swap. Nothing compiles against it yet; land it alone
   so a rollback is one file.
2. **`field.Store` namespace-awareness** — `:53` first (it is the leak), then `Def.SnippetID`,
   `scanDef`, `Create`, `Update`, `Move`, `validate`. Existing tests must stay green: this
   step must be invisible to pages and blocks.
3. **`snippet` store carries `fields`** — column in `columns`/`scan`, `Create`/`Update`
   signatures, `LoadRendered` returns it.
4. **Admin screen** — the fourth mode of `fieldListData`, the values form on `snippet_list.html`.
   Half the phase's effort, per `docs/offene-punkte.md`.
5. **Theme surface** — `Site.Bausteinfelder`/`Bausteinliste`, fixtures, spec.
6. **Bundle round-trip**, then i18n + browser gate.

---

## 5. Phase 9 — CSV Import

### 5.1 `internal/wxr` as the template — including where not to copy it

**It does not stream.** `wxr.Parse` (`internal/wxr/wxr.go:125–192`) calls
`dec.Decode(&f)` at `:132`, materialising the whole document into `feed` before the item
loop at `:142`. The bound is `MaxItems = 2000` (`:30`), applied *after* decoding (`:165–168`,
setting `Export.Truncated`). Its own comment (`:27–29`) admits the memory cost. The upload is
separately capped at 10 MB by `http.MaxBytesReader` at `internal/admin/wordpress.go:25`.

**A CSV importer should do better, and can:** `encoding/csv.Reader.Read()` returns one record
at a time, so `internal/csv` can genuinely stream — read the header row, then loop `Read()`
with a row cap, never holding the file. Keep the same *shape* (`MaxRows` constant, a
`Truncated` flag, a per-row report) and improve the mechanism.

**Validate / report / create — the three-layer split to copy verbatim:**

| Layer | WXR | CSV equivalent |
|---|---|---|
| Pure parse, no DB, no HTTP | `wxr.Parse` → `Export{SiteTitle, Items, MediaURLs, Skipped, Truncated}` (`:52–65`) | `csv.Parse` → `Table{Header []string, Rows [][]string, Skipped, Truncated}` |
| One-row create, returns a *reason string* or `""` | `importWordPressItem` (`admin/wordpress.go:107–153`) | `importCSVRow` |
| Handler: cap → parse → create website → loop → report | `HandleWordPressImport` (`:22–99`) | `HandleCSVImport` |

**The one-row function is the whole contract.** `importWordPressItem`:
- title fallback `:108–111`
- `page.Slugify(item.Slug)` then fall back to the title `:113–116`, `page.ValidateSlug` `:117–119`
- in-import duplicate suffixing against a `seen map[string]bool` `:122–126` — note this is
  *within* the import; `page.CreatePage` separately retries `-2`, `-3`… against the database
  (`internal/page/store.go:467–491`, `maxSlugAttempts = 100` at `:406`)
- `page.RenderMarkdown(item.HTML)` `:128–131` — the sanitised pipeline
- `status` draft/published `:133–136`
- **`h.pages.CreatePage(r.Context(), page.PageCreate{…})` `:138–142`** — the ordinary path,
  which is IMP-02
- terms via `h.terms.SetForPage` `:147–151`

`page.PageCreate` (`internal/page/store.go:409–438`) already carries `Fields string` (`:419`)
— so a CSV column mapped to a custom field is `field.Encode(field.Clean(defs, data))`
handed in there, using **the encoding Phase 7 settles**. That is the only dependency between
the two phases.

**Reporting.** `bundle.Report` (`internal/bundle/import.go:41–51`) is
`{WebsiteID, Pages, Media, Menus, Snippets, Terms, Warnings []string}`, rendered by
`cmd/holzcloud/templates/admin/import_report.html`. WXR reuses it
(`admin/wordpress.go:52`), appending one warning per failed row plus summary warnings
(`:67–89`). IMP-03 wants *every* row named, created **or** skipped — `Warnings []string`
would do it but reads oddly for successes. Cleanest: keep `bundle.Report` for the counts and
add `Report.Rows []RowResult{Line int, Slug, Reason string}` plus a block in
`import_report.html`, so both importers can use it.

### 5.2 Where it hangs off the admin screen

`cmd/holzcloud/templates/admin/website_list.html` has two `<details class="card">` panels
above the table: the bundle import at **`:4–22`** and the WordPress import at **`:24–45`**;
`{{template "website_list-content" .}}` follows at `:47`. Each is a plain
`<form method="POST" enctype="multipart/form-data">` with a hidden
`gorilla.csrf.Token`, a file input, a name input and a `hx-disabled-elt="this"` button.
**A third `<details>` block, copied from `:24–45`, is the whole UI hook.**

Routes, per the project's own anti-pattern rule ("register the route in `newRouter`, never in
a feature package"): `cmd/holzcloud/main.go:827–828`

```go
adminProtectedMux.Handle("POST /admin/websites/import",           requireAdmin(...HandleWebsiteImport))
adminProtectedMux.Handle("POST /admin/websites/import-wordpress", requireAdmin(...HandleWordPressImport))
```

CSV needs **two** routes, because the mapping screen is a second step:
`POST /admin/websites/import-csv` (upload → mapping screen) and
`POST /admin/websites/import-csv/ausfuehren` (confirm → create). Both `requireAdmin`.
Add both to `TestRouteAuthorization`'s `adminOnly` table
(`cmd/holzcloud/main_test.go:158–173`) — that table exists precisely because a missing
`requireAdmin` once shipped.

**Carrying the file between the two steps** is the one design decision the mapping screen
forces. Do **not** put the parsed table in the session (SCS stores sessions in SQLite as
blobs; a 2 MB CSV in a session row is a bad day). Two honest options: re-submit the file with
the mapping, or write it to a temp file under `cfg.DataDir` with a random name and pass the
name. Re-submitting is simpler and needs no cleanup job.

### 5.3 Upload guards to copy

| Guard | Where | Value |
|---|---|---|
| `http.MaxBytesReader` on `r.Body` | `admin/wordpress.go:25` | `10<<20` |
| | `admin/media.go:103` | `max(cfg.MaxMediaSize, cfg.MaxVideoSize)` |
| | `admin/template.go:98` | `cfg.MaxTemplateSize` |
| | `admin/bundle.go:83` | `cfg.MaxMediaSize*10` |
| | `admin/plugin.go:87` | `plugin.MaxTotalBytes` |
| `r.ParseMultipartForm` | `admin/template.go:100` | same cap |
| Magic-byte MIME check | `media.ValidateMIME` `internal/media/upload.go:34–64` | reads 512 bytes, `http.DetectContentType`, seeks back; allow-list at `:17–28`; SVG needs both extension *and* an `<svg` root (`:58–61`) |
| Config defaults | `internal/config/config.go:121–123` | template 10 MB, media 5 MB, video 64 MB |

For CSV: `MaxBytesReader` at a fixed cap (5–10 MB, matching WXR's posture — CSV is text and
carries no images), a **row cap** (`csv.MaxRows`, reported not silently applied, mirroring
`wxr.MaxItems` at `:30`), and a **"is it text" check** before parsing. `ValidateMIME`'s
allow-list will *reject* CSV, so do not reuse it directly — reuse its *technique*: read the
first 512 bytes, require `http.DetectContentType` to return a `text/` type, and reject
anything with a NUL byte. Also set `csv.Reader.FieldsPerRecord` and `LazyQuotes=false` so a
malformed file is an error, not a silent shift of every column.

### Phase 9: new vs modified

| | |
|---|---|
| **NEW** | `internal/csv/` (`csv.go` + `csv_test.go`) — `Parse`, `Table`, `MaxRows`; `internal/admin/csvimport.go` — `HandleCSVImport`, `HandleCSVImportRun`, `importCSVRow`, `CSVMappingData`; `cmd/holzcloud/templates/admin/csv_mapping.html` |
| **MODIFIED** | `cmd/holzcloud/main.go` (+2 routes near `:828`); `main_test.go:158–173` (+2 rows); `website_list.html` (+1 `<details>` after `:45`); `bundle.Report` (`import.go:41–51`, +`Rows`); `import_report.html`; `internal/i18n/locales/*.json` |
| **UNCHANGED** | `page.CreatePage`, `page.Slugify`, `page.RenderMarkdown`, `field.Encode` — the point of IMP-02 is that none of them change |

**Build order:**

1. **`internal/csv` parser + tests** — pure, no DB, no HTTP. Fully testable alone.
2. **`importCSVRow`** — one row → `page.CreatePage`, returns a reason string. Reuses Phase 7's
   multi-value encoding; this is the only place the two phases meet.
3. **Upload + mapping screen** (htmx preview swap, full-page fallback mandatory).
4. **Confirm + report** — extend `bundle.Report` with `Rows`, extend `import_report.html`.
5. **Routes + `main_test.go` table + `website_list.html` panel.**
6. **Milestone gates:** `go run ./tools/i18n` ⇒ `0 offen, 0 verwaist`, browser pass.

**Note:** Phase 9 depends on Phase 7 **only** for the multi-value encoding, and not on
Phase 8 at all. If Phase 7 step 1 lands early, Phase 9 can run in parallel with Phase 8.

---

## 6. Phase 10 — Authentik Forward-Auth

### 6.1 The exact chain that guards the admin

**`cmd/holzcloud/main.go:968`** — one line, read outside-in:

```go
mux.Handle("/admin/",
  web.AdminHeaders(          // internal/web/headers.go:97   — adminCSP, X-Frame-Options, no-store
   csrfMiddleware(           // gorilla/csrf
    setupGuard(              // main.go:334–360 — first-run bootstrap, latched atomic.Bool :333
     requireAuth(            // auth.RequireAuth,        main.go:650, internal/auth/middleware.go:33–64
      requireSecondFactor(   // auth.RequireSecondFactor, main.go:654, internal/auth/twofactor.go:50–78
       withLang(             // i18n.Middleware,         main.go:965
        requireWebsite(      // auth.RequireWebsiteAccess, main.go:661, middleware.go:99–115
         withNav(            // web.WithNav,             main.go:956
          adminProtectedMux)))))))))
```

Outside that, at `main.go:1066–1069`: `RequestID` → `AccessLog` → `Recoverer` →
`SecureHeadersWith` → `sm.LoadAndSave`.

The **public** admin branch is `main.go:936`:
`i18n.Middleware(nil)(web.AdminHeaders(csrfMiddleware(setupGuard(adminPublicMux))))`, mounted
on `/admin/login`, `/admin/login/`, `/admin/2fa` (exact), `/admin/activate/`, `/admin/reset/`,
`/admin/setup`, `/admin/setup/` (`:937–944`). Routes registered at `:700–719`.

`auth.Chain` (`internal/auth/middleware.go:17–22`) exists as a helper but **`newRouter` does
not use it** — the chain at `:968` is hand-nested. A forward-auth middleware therefore goes in
by editing that one line, not by appending to a list.

**`RequireAuth` (`middleware.go:33–64`)** reads `sm.GetInt64(ctx, SessionKeyUserID)` (`:36`);
zero ⇒ 303 to `/admin/login` (`:38`). It then **re-validates against the database on every
request** via `UserLookup` (`:42–59`, implemented at `internal/admin/handler.go:131`): a
deleted user has its session destroyed (`:50`) and a changed role is written back into the
session (`:56–58`).

**This is the seam.** A forward-auth middleware placed *between* `setupGuard` and
`requireAuth` need only put a valid `user_id` into the session; `RequireAuth` then validates
it, `RequireAdmin` reads `SessionKeyUserRole`, and `RequireWebsiteAccess` reads
`SessionKeyUserID`. Nothing downstream changes.

### 6.2 `completeLogin` — the single funnel

`internal/admin/login.go:106–125`:

```go
h.sm.Remove(ctx, auth.SessionKeyPendingUserID)   // :107
h.sm.Put(ctx, auth.SessionKeyUserID,    id)      // :108
h.sm.Put(ctx, auth.SessionKeyUserRole,  role)    // :109
h.sm.Put(ctx, auth.SessionKeyUserEmail, email)   // :110
h.users.RecordLogin(ctx, id)                     // :114 — logged, never fatal
h.LogActivity(r, activity.Entry{                 // :118–124
    UserID: &id, ActorEmail: email,
    Action: activity.ActionAuthLoginSuccess, EntityType: "user", EntityID: id})
```

Its comment (`:102–105`) states the design: *"It is the single place a session becomes signed
in, so the password path and the second-factor path cannot drift apart."* Three callers today:

| Caller | Path |
|---|---|
| `internal/admin/login.go:97` | password, no TOTP |
| `internal/admin/twofactor.go:118` | password + TOTP verified |
| `internal/admin/setup.go:121` | first-run bootstrap |

**A forward-auth handler must be the fourth caller and nothing else.** Any code path that
writes `SessionKeyUserID` directly bypasses the activity log, `RecordLogin`, and the pending-id
cleanup — and the log line is what makes an SSO sign-in auditable.

Note the surrounding discipline `completeLogin` does *not* do and its callers do:
`h.sm.RenewToken(ctx)` **before** setting values — `login.go:80` ("prevents session
fixation") and again at `twofactor.go:115–117` ("a second rotation: the session that carried
the pending id must not be the session that ends up signed in"). **A forward-auth handler must
call `RenewToken` first too.**

### 6.3 The trust boundary — answered from the code

**Yes. The existing client-IP code validates the PEER address.**

`internal/web/clientip.go`:

```go
type ClientIPResolver struct{ trusted []netip.Prefix }        // :16–18
func NewClientIPResolver(trusted []netip.Prefix) *…            // :21–23

func (r *ClientIPResolver) ClientIP(req *http.Request) string { // :32–45
    peer := hostOnly(req.RemoteAddr)                            // :33  ← the TCP peer
    if !r.isTrusted(peer) { return peer }                       // :34  ← peer check FIRST
    for _, c := range reverse(splitForwarded(
            req.Header.Values("X-Forwarded-For"))) {            // :38  right-to-left
        if !r.isTrusted(c) { return c }                         // :39
    }
    return peer                                                 // :44
}

func (r *ClientIPResolver) IsTrustedPeer(req *http.Request) bool { // :50–52
    return r.isTrusted(hostOnly(req.RemoteAddr))
}
func (r *ClientIPResolver) isTrusted(host string) bool {           // :54–66
    addr, err := netip.ParseAddr(host); if err != nil { return false }
    addr = addr.Unmap()                       // :59 — ::ffff:1.2.3.4 cannot dodge a v4 prefix
    for _, p := range r.trusted { if p.Contains(addr) { return true } }
    return false
}
```

`req.RemoteAddr` is set by `net/http` from the accepted connection and cannot be spoofed by a
client. So `IsTrustedPeer` is a **real** trust check, not header parsing. It is
`*web.ClientIPResolver`, constructed at **`cmd/holzcloud/main.go:257`** from
`cfg.TrustedProxies`, injected into `admin.NewHandler` as its last argument
(`main.go:260`, field at `internal/admin/handler.go:79`) and into `routerDeps.clientIP`
(`main.go:415`, declared `:587`).

Config: `Config.TrustedProxies []netip.Prefix` (`internal/config/config.go:45–47`), parsed
from `HOLZCLOUD_TRUSTED_PROXIES` at `:163`, **defaulting to
`"127.0.0.1/32,::1/128"`** (`defaultTrustedProxies`, `:98`) — the documented Caddy-on-the-same-host
deployment. Logged at `:197–200`.

**Header-trust is therefore safe — provided the gate is `IsTrustedPeer`, and there is already
a working precedent for exactly that shape.** `internal/web/logging.go:31–45`:

```go
func RequestID(resolver *ClientIPResolver) func(http.Handler) http.Handler {
    …
    id := ""
    if resolver != nil && resolver.IsTrustedPeer(r) {              // :35  ← the gate
        id = sanitizeRequestID(r.Header.Get("X-Request-ID"))       // :36  ← then sanitise
    }
    if id == "" { id = newRequestID() }                            // :38–40
```

Its comment (`:28–30`): *"An incoming X-Request-ID is only adopted from a trusted proxy;
otherwise a client could poison the logs by choosing its own."* `sanitizeRequestID` (`:57–…`)
caps length at 64 and requires printable characters. **Copy this function's structure exactly:**
gate on `IsTrustedPeer`, then sanitise, then use.

**Two operational cautions the planner must carry:**

1. **The default trusts loopback.** If a forward-auth middleware ships enabled-by-default,
   any process on the host that can reach `localhost:PORT` can assert any identity. The
   feature must require an explicit opt-in — e.g. `HOLZCLOUD_FORWARD_AUTH_HEADER` empty ⇒ the
   middleware is a no-op, matching the project's established "optional dependencies are
   nil-able" pattern.
2. **The proxy must strip the header on the way in.** Caddy's `forward_auth` with
   `copy_headers` sets it; the Caddyfile must also delete any client-supplied copy. That is a
   `deploy/Caddyfile.example` change, not a Go change.

### 6.4 The TOTP gate, and where "administrators must have a second factor" actually lives

| Constant / function | File:line | Value / effect |
|---|---|---|
| `SessionKeyPendingUserID` | `internal/auth/twofactor.go:19` | `"pending_user_id"` — deliberately *not* `user_id`, so a half-authenticated session has no user at all (`:15–18`) |
| `SetupPath` | `:22` | `/admin/2fa/einrichten` |
| `VerifyPath` | `:25` | `/admin/2fa` |
| **`MustHaveSecondFactor(role)`** | **`:44`** | **`return role == "admin"`** — this one function is the whole rule |
| `RequireSecondFactor` | `:50–78` | enforcement: skips 2FA paths (`:53`), skips when no user (`:58–62`), looks up state (`:64`), and at **`:70–72`** redirects to `SetupPath` when `MustHaveSecondFactor(role) && !Enabled` |
| `isSecondFactorPath` | `:85–87` | `/admin/2fa*` and `/admin/logout` stay reachable |
| `SecondFactorLookup` impl | `internal/admin/twofactor.go:342–358` | one query: `SELECT role, totp_confirmed_at FROM users WHERE id = $1`; `Enabled = confirmed.Valid` |
| Wired | `cmd/holzcloud/main.go:654` | inside `requireAuth` (`:651–653` explains why) |

**Flow:** `HandleLogin` verifies the password, calls `RenewToken` (`login.go:80`), then reads
`h.users.GetTwoFactor` (`:87`). If enabled it writes **only** `SessionKeyPendingUserID`
(`:92`) and redirects to `VerifyPath` (`:93`); `RequireAuth` would bounce that session, which
is why `GET/POST /admin/2fa` sit on the **public** mux (`main.go:703–707`, comment explains).
`HandleTwoFactorVerify` throttles the code (`twofactor.go:81–88`), verifies
(`:92–105`), rotates again (`:115–117`), then calls `completeLogin` (`:118`).

**The Phase 10 policy question this exposes:** an administrator arriving via Authentik has
already presented a second factor *at the IdP*. Making them also pass local TOTP is either
correct defence-in-depth or a usability wall, but the decision has exactly one home:
**`auth.MustHaveSecondFactor` at `twofactor.go:44`**. Change it there or nowhere. A likely
shape is `MustHaveSecondFactor(role string, viaSSO bool) bool` with a session key recording
how the session was established.

### 6.5 `internal/user/rights.go` — the shape a group mapping must produce

```go
type Rights struct {                    // :26–32
    MayPublish bool                     // false = writes and submits
    Websites   []int64                  // EMPTY MEANS ALL   :30
}
func Everything() Rights                // :36  — {MayPublish: true}
func (r Rights) MayUse(id int64) bool   // :39–49 — len==0 ⇒ true
func (r Rights) Limited() bool          // :53
func (s *Store) Rights(ctx, id) (Rights, error)      // :59–86
func (s *Store) SetRights(ctx, id, Rights) error     // :93–121 — replaces wholesale (:90–92)
```

Storage: `users.may_publish INTEGER NOT NULL DEFAULT 1`
(`internal/db/migrations/00033_user_rights.sql:20`) and
`user_websites(user_id, website_id) PRIMARY KEY (…)` STRICT (`:28–32`), index on `website_id`
(`:35`). `Store.Rights` short-circuits: **an admin always gets `Everything()`** (`:67–69`) —
*"a site an administrator may not enter would be a site nobody could repair."*

**Constraints on a group → rights mapping:**

- **Only two roles exist.** `user.RoleAdmin = "admin"`, `user.RoleEditor = "editor"`
  (`internal/user/store.go:22–23`), `ValidRole` at `:80`, and — the hard part —
  `users.role` carries a **table-level CHECK** `CHECK (role IN ('admin','editor'))`
  (`00001_initial.sql:7`). Per the `00029` lesson, loosening a table-head CHECK in SQLite
  means a full table rebuild, and `users` has foreign-key children (`user_websites`,
  `page.created_by/updated_by`, activity log). **Do not invent a third role.** Map Authentik
  groups onto `admin` / `editor` + `Rights`.
- **The natural mapping is three-part:** one group name ⇒ `role = admin`; one group name ⇒
  `MayPublish`; one group **per website** ⇒ a row in `user_websites`. Empty must keep meaning
  "all websites" (`:21–23` of the migration says a migration that silently takes access away
  is one after which nobody can work on Monday) — so an SSO user with **no** matching website
  group should get **no** `user_websites` rows only if that is what the operator configured;
  otherwise they get everything. Make this an explicit, documented setting.
- **`SetRights` replaces wholesale** (`:102–105` deletes then re-inserts). Re-applying it on
  every SSO sign-in is safe and idempotent — that is the right place to sync group membership.
- **Provisioning a user needs a password.** `users.password TEXT NOT NULL`
  (`00001_initial.sql:6`) has no CHECK, so an SSO-provisioned account can store an
  unguessable random Argon2id hash rather than needing a schema change. `user.Store.Create`
  (`store.go:171`) hashes what it is given — pass 32 random bytes and never disclose them.

### 6.6 `HandleLogout`, and the CSP trap nobody expects

`internal/admin/login.go:128–136`:

```go
h.LogActivity(r, activity.Entry{Action: activity.ActionAuthLogout, EntityType: "user"})  // :130 BEFORE destroy
h.sm.Destroy(r.Context())                                                                // :131
http.Redirect(w, r, "/admin/login", http.StatusSeeOther)                                 // :134
```

`sm.Destroy` removes the whole SCS session row — `user_id`, `user_role`, `user_email`,
`pending_user_id` (`auth/twofactor.go:19`), `elevated_at` (`auth/elevate.go:24`) and the
flash keys — and clears the cookie. Route: `POST /admin/logout`
(`cmd/holzcloud/main.go:723`), on the **protected** mux but explicitly allowed through the
2FA gate (`auth/twofactor.go:86`).

**A redirect to an IdP end-session endpoint goes at `login.go:134`** — replace the target
when a configured sign-out URL exists, keeping the local `Destroy` above it (the local
session must die whether or not the IdP round-trip succeeds).

**And here is the trap.** `adminCSP` (`internal/web/headers.go:11–20`) contains
**`form-action 'self'`** at `:17`. Logout is a **POST**, and a redirect issued in response to
a form submission is checked against `form-action` by some browsers — Safari among them. The
codebase already learned this the expensive way, in the Payrexx comment at `headers.go:41–53`:
*"a redirect after a form submission is checked against form-action by some browsers — Safari
among them. Without this the TWINT payment on an iPhone dies silently at the moment of the
handover, and nothing in the server log says why."*

**So a POST-logout that redirects to `https://auth.example.ch/…/end-session` will die silently
on Safari unless `adminCSP` allows that origin in `form-action`.** The fix has a precedent to
copy exactly: `PublicCSP(extraFormAction ...string)` (`headers.go:59–67`) +
`SecureHeadersWith(csp)` (`:82`), wired from config at `main.go:1062–1066`. Phase 10 needs the
same pair for the admin: `web.AdminCSP(extraFormAction ...string)` and
`web.AdminHeadersWith(csp)`, replacing the constant-only `AdminHeaders` (`:97–106`) at both
`main.go:936` and `main.go:968`. Keep `frame-ancestors 'none'`, `X-Frame-Options: DENY` and
`Cache-Control: no-store` — `cmd/holzcloud/main_test.go:220–234` asserts all three.

An alternative that avoids the CSP change entirely: answer the POST with a local
`/admin/abgemeldet` page carrying a plain `<a>` to the IdP. Navigation by link is not checked
against `form-action`, and CLAUDE.md already blesses outbound hyperlinks as content.

### 6.7 Is there a precedent for trusting an inbound header for identity?

**`/ai` is a precedent for authenticating outside the session, but *not* for believing a
header's claim.** `cmd/holzcloud/main.go:695–697` mounts `d.aiServer` on `/ai` **outside** the
admin chain — no CSRF, no session — and the comment at `:687–694` explains: a browser form
cannot set a request header, so the CSRF gap does not exist there.

`internal/ai/mcp.go`: `ServeHTTP` `:121–169`; POST-only `:127–132`; `authenticate` `:134`;
`WWW-Authenticate: Bearer` + 401 with a deliberately identical message for missing and wrong
(`:136–140`); body capped by `MaxRequestBytes` `:143`. `authenticate` (`:172–178`) reads
`Authorization`, requires the `bearer ` prefix, and hands the rest to
`ai.Store.Verify` (`internal/ai/token.go:144–174`) — which looks the value up **by SHA-256
hash** (`:157–159`, *"a wrong key is a miss in an index rather than a comparison this code
could get wrong"*), checks expiry (`:163–167`) and returns a `Scope{TokenID, Name, WebsiteID,
CanWrite}` with `MayWrite`/`MaySee` guards (`token.go:74–91`).

**The distinction that matters:** the `/ai` header carries a **secret** the server can verify
independently. A forward-auth header carries a **claim** whose only guarantee is who the peer
is. So `/ai` is the precedent for *"a route may live outside the session chain"*, and
**`web.RequestID` (`logging.go:31–45`) is the precedent for *"a header may be believed when
the peer is trusted"***. Phase 10 needs both, and must not confuse them.

Exhaustive check of every other header read in non-test code
(`grep r.Header.Get`, excluding HX-Request / Accept-Language / Content-Type / conditional /
Range / Origin / Referer / User-Agent):

| Site | Purpose | Trusted? |
|---|---|---|
| `internal/web/logging.go:36` | `X-Request-ID` | **only via `IsTrustedPeer`** (`:35`) |
| `internal/public/plugins.go:101` | forwards an allow-listed set of headers to a plugin | read-only pass-through, no identity |
| `internal/ai/mcp.go:173` | `Authorization: Bearer` | verified secret |

**Nothing in the codebase today trusts a header's *claim* for identity.** Phase 10 introduces
the first one, which is why the `IsTrustedPeer` gate is not optional.

### Phase 10: new vs modified

| | |
|---|---|
| **NEW** | `internal/auth/forwardauth.go` — `ForwardAuthConfig{Header, EmailHeader, GroupsHeader, AdminGroup, PublishGroup, WebsiteGroupPrefix}` + `ForwardAuth(sm, resolver, cfg, provision) Middleware`; a provisioning/sync function on `internal/admin` (fourth `completeLogin` caller); `web.AdminCSP` + `web.AdminHeadersWith`; `Config` fields for `HOLZCLOUD_FORWARD_AUTH_*` incl. the sign-out URL; a `<a href>` sign-in button on `login.html`; docs in `deploy/Caddyfile.example` + `deploy/DEPLOY.md`; a new `_test.go` proving an untrusted peer's header is ignored |
| **MODIFIED** | `cmd/holzcloud/main.go:936` and `:968` (insert the middleware between `setupGuard` and `requireAuth`; swap `AdminHeaders` → `AdminHeadersWith`); `internal/config/config.go` (new env vars beside `:163`); `internal/admin/login.go:134` (`HandleLogout` redirect target); `internal/web/headers.go:11–20,:97–106`; possibly `internal/auth/twofactor.go:44` (`MustHaveSecondFactor`); `cmd/holzcloud/main_test.go` |
| **UNCHANGED — deliberately** | `auth.RequireAuth`, `RequireAdmin`, `RequireWebsiteAccess`, `RequireSecondFactor`, `completeLogin`'s body, `user.Rights`/`SetRights`, `users` schema. If any of these need changing, the design is wrong. |
| **NO MIGRATION expected** | `users.password` is `NOT NULL` without a CHECK (a random hash suffices); a third role would need a table rebuild (`00001:7`) and must be avoided |

**Build order (each step is independently shippable and reversible):**

1. **Config + no-op middleware.** Env vars; `ForwardAuth` returns `next` unchanged when the
   header name is empty. Land the wiring at `main.go:968` while it does nothing.
2. **Peer-gated header read.** Copy `web.RequestID`'s shape: `IsTrustedPeer` → sanitise →
   use. Test first that an untrusted peer's header is ignored. **This is the security core;
   do it before anything can rely on it.**
3. **Look up an existing user by e-mail; call `completeLogin`.** No provisioning yet — SSO
   works for accounts that already exist. `RenewToken` first.
4. **Provisioning.** Create a missing user with a random password hash + `RoleEditor`.
5. **Group → rights sync.** Map groups to `role` and to `user.SetRights`; re-apply on every
   sign-in. Depends on 4.
6. **TOTP policy.** Decide and implement in `MustHaveSecondFactor` alone. Depends on 3.
7. **Sign-out.** `HandleLogout` redirect + the `AdminCSP`/`AdminHeadersWith` pair (or the
   interstitial-page alternative). Depends on nothing above; can run in parallel from step 2.
8. **Deployment docs + browser pass**, including a run with forward-auth *disabled* to prove
   the password path is untouched.

---

## 7. Cross-phase dependency graph and execution order

```
  Phase 6 (Aufräumen)
     │  planning docs + i18n comment + plugin build/CI
     │  (blocks nothing technically; unblocks the roadmapper and makes
     │   Phases 7–9's "0 offen, 0 verwaist" gate meaningful)
     ▼
  Phase 7 (Field Kinds)
     │  step 1: multi-value encoding  ──────────────┐
     │  steps 2–6                                   │
     ▼                                              │
  Phase 8 (Snippets Carry Fields)                   │  (only dependency)
     reuses page_field_defs; wants the palette      │
     complete before snippets store a value         ▼
                                          Phase 9 (CSV Import)
                                          independent of Phase 8

  Phase 10 (Authentik) — independent of 7, 8, 9. Touches auth/, web/, config/,
  main.go only. Can be executed at any point; nothing in 6–9 reads or writes it.
```

**Recommended order: 6 → 7 → (8 ∥ 9) → 10**, with Phase 10 movable earlier if a second
worker is available, because its file set is disjoint from every other phase's.

---

## 8. Anti-patterns this milestone is most likely to hit

### 8.1 A second field-definition table

**What people do:** add `snippet_field_defs` because `page_field_defs` "is for pages".
**Why it's wrong:** `00029:10–13` and `00038:11–15` both record the same lesson — a second
table needs the same columns, the same validation, the same ordering logic and the same input
templates, and drifts. SNIP-02 is written to forbid it.
**Do this instead:** `snippet_id` beside `block_type_id`, and add `AND snippet_id IS NULL` to
`internal/field/store.go:53`.

### 8.2 Turning `field.Values` into `map[string][]string`

**What people do:** make the type match the data.
**Why it's wrong:** eight packages destructure it — `internal/bundle` (6 sites),
`internal/ai/tools.go` (6 sites), `internal/public`, `internal/admin`. Multi-choice is one
field kind; it should not cost a type change across the codebase.
**Do this instead:** newline-encode inside the existing string, exactly as `SplitChoices`
already reads options (`field.go:613–622`).

### 8.3 A CSV importer with its own INSERT

**What people do:** write straight to `pages` for speed.
**Why it's wrong:** it skips slug uniquing (`store.go:467–491`), sanitisation
(`page.RenderMarkdown`), revisions, activity logging and the FTS triggers
(`00010:56–90`). IMP-02 exists because of this.
**Do this instead:** `page.CreatePage(ctx, page.PageCreate{…})`, as
`admin/wordpress.go:138–142` does.

### 8.4 Believing a forward-auth header without checking the peer

**What people do:** `r.Header.Get("X-Authentik-Username")`.
**Why it's wrong:** with the binary on `localhost:PORT` and the proxy in front, anyone who can
reach the port directly becomes any user. `HOLZCLOUD_TRUSTED_PROXIES` exists precisely to make
this checkable, and the peer check is already implemented.
**Do this instead:** gate on `resolver.IsTrustedPeer(r)` first, then sanitise, then use —
`internal/web/logging.go:31–45` is the working copy.

### 8.5 Writing `SessionKeyUserID` outside `completeLogin`

**What people do:** the forward-auth middleware sets the session key itself.
**Why it's wrong:** skips `RecordLogin`, skips the activity-log entry, skips the pending-id
cleanup — an SSO sign-in becomes invisible in `/admin/protokoll`.
**Do this instead:** call `completeLogin` (`login.go:106`), after `RenewToken`.

### 8.6 Registering a route inside a feature package

The existing rule (`.planning/codebase/ARCHITECTURE.md`, and the reason `newRouter` is
testable at all — `main.go:626–629`): every route is declared in `newRouter` and covered in
`cmd/holzcloud/main_test.go`. Phase 9's two CSV routes and any Phase 10 route go there, and
into `TestRouteAuthorization`'s table (`main_test.go:158–173`).

### 8.7 Adding a contract field without the fixture and the spec

`internal/template/sample_test.go:17` and `internal/tmplspec/spec_test.go:18` fail the build
if a new field on `PageContent`/`Site` is missing from `SampleData`/`MinimalData` or from
`TEMPLATE-SPEC.md`. Phases 7 and 8 both add contract fields.

---

## 9. Confidence

| Question | Confidence | Basis |
|---|---|---|
| i18n writer output already matches the committed files | **HIGH** | ran `-write` and `-schweiz`; `git diff` empty both times; counted indented lines per file |
| No catalog is two-space indented | **HIGH** | `grep -c '^  "'` = 0 on all seven |
| No build tag / Makefile / CI step for `plugins/*/plugin.wasm` | **HIGH** | read `ci.yml` in full; `find`ed for build scripts; `grep`ed for build tags |
| The five wasm tests currently pass, not skip | **HIGH** | `go test -v -run …` output captured |
| `page_field_defs.art` has no CHECK ⇒ Phase 7 needs no migration | **HIGH** | `00028:25–27` and `00029:32` read directly |
| `snippet_id` is an index swap, not a rebuild | **HIGH** | `00038:41–60` is the same operation, already performed for `block_type_id`; its own comment says so at `:44–46` |
| `field/store.go:53` is the leak a `snippet_id` column creates | **HIGH** | query read in full |
| `ClientIPResolver` validates the peer, not just a header | **HIGH** | `clientip.go:32–34`, `:50–52` |
| `web.RequestID` is the pattern to copy | **HIGH** | `logging.go:31–45` |
| `adminCSP` `form-action 'self'` will break an external POST-logout redirect | **MEDIUM-HIGH** | the mechanism is documented in this same file (`headers.go:41–53`) from a real Safari incident; not re-tested here for the admin origin |
| `MustHaveSecondFactor` is the only place the admin-2FA rule lives | **HIGH** | `grep`ed; one definition (`twofactor.go:44`), one caller (`:70`) |
| A third role would need a table rebuild | **HIGH** | `00001_initial.sql:7` table-head CHECK; the `00029` lesson applies |

## Sources

All primary — the working tree at `c58ceb0`, plus commands run against it:
`go run ./tools/i18n [-write|-schweiz]`, `git diff`, `go test -v -run …`,
`git ls-files | grep wasm`, `grep -c '^  "'` per catalog.
No external sources were needed; every question was answerable from the code.

---
*Integration research for: Holzcloud CMS v1.6, Phases 6–10*
*Researched: 2026-09-03*
