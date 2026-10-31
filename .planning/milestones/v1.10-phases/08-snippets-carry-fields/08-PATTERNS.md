# Phase 8: Snippets Carry Fields - Pattern Map

**Mapped:** 2026-09-06
**Files analyzed:** 13 (1 new, 12 modified)
**Analogs found:** 13 / 13
**Central finding:** the block-kind namespace is expressed in **eight** places, not
one. Every one of them is a complete, working template for the snippet namespace.

---

## The claim, made concrete

`block_type_id` is not just a column. Phase 8's shape is already written down in
these eight places, and each is a line-for-line model for `snippet_id`:

| # | Where the block-kind namespace is expressed | File |
|---|---|---|
| 1 | The column, added without a default and with a partial index swap | `internal/db/migrations/00038_block_types.sql:35-60` |
| 2 | The struct member beside `ParentID` | `internal/field/field.go:249-253` |
| 3 | Five SELECT column lists + one INSERT + the `scanDef` scan order | `internal/field/store.go:57-206, 250-258` |
| 4 | A dedicated per-carrier reader `OfBlockType` and a bulk `OfBlockTypes` | `internal/field/store.go:113-160` |
| 5 | `Update` deliberately *not* carrying the discriminator | `internal/field/store.go:280-306` |
| 6 | `Move` switching on the carrier | `internal/field/store.go:380-390` |
| 7 | `validate` narrowing what a carrier may hold | `internal/field/store.go:525-537` |
| 8 | **The admin screen: a third *mode* of the existing field screen, not a new screen** | `internal/admin/field.go` + `cmd/holzcloud/templates/admin/field_list.html` |

**#8 is the measurement the prompt asked for.** There is **no** per-block-kind
field handler and **no** per-block-kind field template. `?baustein=<id>` on
`GET /admin/websites/{id}/felder` turns the one field screen into its third mode
(`internal/admin/field.go:79-86`, `:236-268`, `fieldPath` at `:271-281`;
`blocktype_list.html:40,58` links into it). The snippet field *definition* screen
is therefore a **fourth mode of the same screen** — roughly 40 lines of Go and a
handful of `{{if .Snippet}}` arms — and the real work of "half the phase" is the
snippet **value** form, whose analog is `internal/admin/page_fields.go`.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/db/migrations/00047_snippet_fields.sql` (new) | migration | batch DDL | `internal/db/migrations/00038_block_types.sql:35-60` | exact |
| `internal/field/field.go` | model | — | `Def.BlockTypeID` `:249-253`; `MaxFields` `:210` | exact |
| `internal/field/store.go` | store | CRUD | its own block-kind arms (same file) | exact |
| `internal/field/store_test.go` | test | — | `TestNeueSpalten` `:101-175` | exact |
| `internal/snippet/store.go` | store | CRUD | `internal/page/store.go` `fields` column (`:75, :186, :473, :610`) | exact |
| `internal/admin/field.go` | handler | request-response | its own `blockType` arms (same file) | exact |
| `cmd/holzcloud/templates/admin/field_list.html` | template | — | its own `{{if .BlockType}}` arms | exact |
| `internal/admin/snippet.go` | handler | request-response | `internal/admin/page_fields.go` + `page_form.go:149-204` | role-match |
| `cmd/holzcloud/templates/admin/snippet_list.html` | template | — | `field_input.html` (used by page form) | exact |
| `internal/template/loader.go` | contract | — | `PageContent.Felder`/`Feldliste` `:456-467` | exact |
| `internal/template/sample.go` | fixture | — | `SampleData` `:58-60`, `MinimalData` `:296-320` | exact |
| `internal/tmplspec/TEMPLATE-SPEC.md` | doc | — | `:212` (`.Site.Snippets` row), `:722-726` | exact |
| `internal/public/pagedata.go` | service | request-response | `ownFields` `:52-60` + `loadSnippets` `:232-244` | exact |
| `internal/admin/snippet_fields_test.go` (new) | test | — | `internal/admin/page_handler_test.go:33-60` harness | exact |

---

## Pattern Assignments

### `internal/db/migrations/00047_snippet_fields.sql` (new)

**Analog:** `internal/db/migrations/00038_block_types.sql:35-60` — the same
operation, third walk.

```sql
-- Die Spalte darf keinen anderen Vorgabewert als NULL haben — SQLite lässt
-- ALTER TABLE ADD COLUMN mit REFERENCES sonst nicht zu. Das passt: NULL heisst
-- „gehört der Seite und keiner Bausteinart", und das trifft auf jede
-- bestehende Zeile zu.
ALTER TABLE page_field_defs
    ADD COLUMN block_type_id INTEGER REFERENCES block_types(id) ON DELETE CASCADE;

-- Die Eindeutigkeit wird neu gezogen. Sie steht seit 00029 als eigener Index
-- da und nicht am Tabellenkopf — deshalb ist das hier ein Austausch von zwei
-- Indizes und kein Tabellenneubau.
DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung)
    WHERE parent_id IS NULL AND block_type_id IS NULL;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_baustein
    ON page_field_defs(block_type_id, kennung)
    WHERE block_type_id IS NOT NULL;
```

The `-- +goose Down` half is equally a template (`00038:62-69`): drop the new
index, drop the swapped one, `DELETE FROM page_field_defs WHERE <col> IS NOT NULL`,
`DROP COLUMN`, then recreate the *previous* version of `idx_..._kennung_oben`.
For `00047` that previous version is **`00038`'s** (with `AND block_type_id IS NULL`),
not `00029`'s — this is D-02 stated as a Down-migration requirement, and it is
the easiest thing in the phase to get wrong by copying `00038`'s Down verbatim.

**One discrepancy the planner must resolve, not assume:** D-01 says `snippet_id`
carries **no** `REFERENCES` clause, but `00038:35` demonstrably *does* carry one
(`REFERENCES block_types(id) ON DELETE CASCADE`) — because it carries no default.
Both statements are true of SQLite: `ADD COLUMN` may carry `REFERENCES` **or** a
non-NULL default, not both. Since `snippet_id` also has no default, the block-kind
form is available. Whichever is chosen, the comment in the file must say why.

**Column-add without a CHECK** — precedent `00028:25-27` and `00046:18` (whole
file is the current example of the house style: every `ALTER TABLE ADD COLUMN`
carries a German paragraph saying what the default means for existing rows).

**`snippets.fields`** — copy `pages.fields`: `TEXT NOT NULL DEFAULT ''`. The
`snippets` table it lands on (`00010_scheduling_search_redirects.sql:34-44`) is
`STRICT` with `UNIQUE (website_id, key)`, so `TEXT NOT NULL DEFAULT ''` is the
only shape that both satisfies STRICT and leaves existing rows valid.

---

### `internal/field/field.go` (model)

**Analog:** the member directly above the new one.

**Struct member pattern** (`:249-253`) — note the comment carries the *reason*
and a migration reference; a bare `SnippetID int64` would break the file's style:

```go
	// BlockTypeID names the block kind this field belongs to, or 0 for a field
	// of the page. The two are separate worlds sharing one table — see
	// migration 00038 — so a page field and a block kind's field may carry the
	// same key without meeting.
	BlockTypeID int64
```

`SnippetID` goes beside it, referencing migration 00047.

**`MaxFields`** (`:205-210`) — D-05 changes the *count*, not the constant. The
doc comment is the thing that must change, because it currently states the
scope that D-05 abolishes:

```go
// MaxFields bounds how many fields one website may define, groups and their
// contents counted together.
//
// Not a technical limit but an editorial one: a form with sixty extra fields
// is a form nobody fills in correctly.
const MaxFields = 60
```

The rationale sentence ("a form nobody fills in correctly") is D-05's own
argument already written in the tree — per-carrier counting makes the comment
true rather than aspirational.

---

### `internal/field/store.go` (store, CRUD) — the dangerous edit

**Analog:** the file itself. The eleven `block_type_id` occurrences, verified:

| Line | Statement | What `snippet_id` needs |
|---|---|---|
| `:57` | `List` column list | `COALESCE(snippet_id, 0)` appended |
| **`:59`** | `List` WHERE | **`AND snippet_id IS NULL`** — D-03 |
| `:94` | `Sub` column list | `COALESCE(snippet_id, 0)` appended |
| `:118-119` | `OfBlockType` list + WHERE | column added; WHERE already excludes by `block_type_id = $2` |
| `:145-147` | `OfBlockTypes` list + WHERE | column added |
| `:176` | `scanDef` scan order | `&d.SnippetID` appended — **must match all five lists character for character** |
| `:193` | `Get` column list | `COALESCE(snippet_id, 0)` appended |
| `:250, :257` | `Create` INSERT + position subquery | new column + `AND COALESCE(snippet_id, 0) = COALESCE($n, 0)` |

**The line that ships broken** (`:56-59`):

```go
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), kennung, beschriftung, art,
		        pflicht, hinweis, auswahl, gilt_fuer, position, bedingung,
		        darstellung, max_werte, min_wert, max_wert, COALESCE(block_type_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND block_type_id IS NULL ORDER BY position, id`, websiteID)
```

**`scanDef`'s own warning** (`:168-172`) is the standing instruction for this
edit and should be quoted into the plan:

```go
	// Die Reihenfolge hier ist die der fünf SELECT-Spaltenlisten, Zeichen für
	// Zeichen. Die fünf sind Abschriften voneinander und müssen es bleiben:
	// eine Liste, die von den anderen abweicht, lädt ein Feld still mit einem
	// Nullwert, und nichts schlägt fehl.
```

**The statement that must NOT be touched** (`:296-306`) — by `id` and
`website_id` only, no discriminator, correct today and after:

```go
	_, err = s.DB.Write.ExecContext(ctx,
		`UPDATE page_field_defs
		 SET beschriftung = $1, art = $2, pflicht = $3, hinweis = $4, auswahl = $5,
		     gilt_fuer = $6, bedingung = $7,
		     darstellung = $10, max_werte = $11, min_wert = $12, max_wert = $13
		 WHERE id = $8 AND website_id = $9`, …)
```

`Update` instead *pins* the carrier from what is stored (`:282-284`) — copy this
line for `SnippetID`, or an edit form could move a field between namespaces:

```go
	d.Key = existing.Key
	d.ParentID = existing.ParentID
	d.BlockTypeID = existing.BlockTypeID
```

**The INSERT's position subquery** (`:246-258`) — its comment already anticipates
this phase ("Three worlds in one table") and must become four:

```go
	var parent, blockType any
	if d.ParentID > 0 {
		parent = d.ParentID
	}
	if d.BlockTypeID > 0 {
		blockType = d.BlockTypeID
	}
	// The position counts within the level: the page's own fields, one group's
	// fields, or one block kind's fields. Three worlds in one table, and a
	// field must never be able to move out of its own.
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO page_field_defs (website_id, parent_id, block_type_id, kennung, …, position)
		 VALUES ($1, $2, $11, …,
		         COALESCE((SELECT MAX(position) + 1 FROM page_field_defs
		                   WHERE website_id = $1
		                     AND COALESCE(parent_id, 0) = COALESCE($2, 0)
		                     AND COALESCE(block_type_id, 0) = COALESCE($11, 0)), 0))`, …)
```

**The `MaxFields` count D-05 rewrites** (`:239-245`) — currently namespace-blind:

```go
	var count int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1`, d.WebsiteID).Scan(&count); err != nil {
		return nil, fmt.Errorf("felder zählen: %w", err)
	}
	if count >= MaxFields {
		return nil, ErrTooMany
	}
```

Per D-09 the replacement must name all four namespaces explicitly — no
"whatever is left" branch. The switch in `Move` (`:380-390`) is the house style
for exactly that, and gains a fourth arm:

```go
	var defs []Def
	switch {
	case current.ParentID > 0:
		defs, err = s.Sub(ctx, websiteID, current.ParentID)
	case current.BlockTypeID > 0:
		defs, err = s.OfBlockType(ctx, websiteID, current.BlockTypeID)
	default:
		defs, err = s.List(ctx, websiteID)
	}
```

**`OfBlockType` is the shape of `OfSnippet`** (`:112-120`) and `OfBlockTypes`
(`:137-160`) the shape of `OfSnippets` — the latter is what `Site.Bausteinfelder`
needs, one query for the whole website rather than one per snippet:

```go
// OfBlockTypes returns every block kind's fields of one website, keyed by kind.
//
// One query rather than one per kind: this runs on every save of a page and on
// every draw of the block editor.
```

**`validate`'s per-carrier narrowing** (`:523-537`) — the decision the plan owes
an answer to is whether a snippet field is narrowed the way a block field is
(`blockKind(d.Kind)`, `Required = false`, `AppliesTo = ForBoth`, no condition).
A snippet has a real form of its own, so the block-kind narrowing is a *contrast*
here, not a template — but `AppliesTo` is meaningless on a snippet and should be
forced to `ForBoth` the way `:535` does:

```go
	if d.ParentID > 0 || d.BlockTypeID > 0 {
		d.Condition = ""
	}
	if d.BlockTypeID > 0 {
		if !blockKind(d.Kind) {
			return ErrNotInBlock
		}
		d.Required = false
		d.AppliesTo = ForBoth
	}
```

---

### `internal/field/store_test.go` (test) — D-04's gate

**Analog:** `TestNeueSpalten` at `:101-175`. Its doc comment is D-04's argument
already in the tree, and the plan should extend this test rather than write a
new one:

```go
// TestNeueSpalten ist das eigentliche Tor auf die sieben SQL-Stellen.
//
// Die Spaltenliste von page_field_defs steht sieben Mal in store.go: fünf
// SELECTs, das INSERT und das UPDATE. Wird eine davon vergessen, lädt ein Feld
// still mit einem Nullwert — eine Knopfreihe erscheint als Klappliste, eine
// Grenze wird nicht durchgesetzt, und nirgends steht ein Fehler. Eine Zählung
// der Vorkommen fände das nicht: ein SELECT kann die Spalte nennen und sie
// trotzdem nie in den Def schreiben. Deshalb wird hier gelesen, nicht gezählt.
```

Its structure — create through the store, then read back through **every** path
(`Get`, `List`, `Sub`, `List`→`Sub`, `OfBlockType`, `OfBlockTypes`) with a
`pruefeX` helper per property and a `finde` lookup (`:88-99`) — is the shape for
D-03's own test: define a snippet field, then assert `List` does **not** return
it while `OfSnippet` does.

---

### `internal/snippet/store.go` (store, CRUD)

**Analog:** `internal/page/store.go` for the `fields` column; the file's own
`columns` const for the local style.

**The column list is a single const** (`internal/snippet/store.go:49`) shared by
`List` and `Get` — one edit, not four:

```go
const columns = `id, website_id, key, name, content_markdown, content_html, created_at, updated_at`
```

`scan` (`:51-62`) reads it positionally, so `Fields string` appends to both.
`Create` (`:96-108`) and `Update` (`:110-120`) take positional params
(`key, name, markdown, html`) — adding a fifth grows a signature that already
has four strings in a row. `page.Store` avoided this by giving fields their own
setter (`internal/page/store.go:186`):

```go
	_, err := s.DB.Write.ExecContext(ctx, `UPDATE pages SET fields = $1 WHERE id = $2`, raw, id)
```

Either extend the signature or copy that setter; the plan should pick one and say why.

**`LoadRendered` is the analog for the theme-side read** (`:144-176`) — one query
for the whole website, plus the sanitisation comment that D-08 rests on:

```go
// Rendered is the expansion map for one website: key to sanitised HTML.
type Rendered struct {
	HTML map[string]template.HTML
	// LatestUpdate is the newest change across all snippets. …
	LatestUpdate time.Time
}
…
		// The cast is safe for exactly the reason page content is: content_html
		// went through goldmark and then bluemonday before it was stored.
		out.HTML[key] = template.HTML(html)
```

`Rendered` is where `Bausteinfelder`/`Bausteinliste` belong: `LoadRendered`
already selects per website and already carries `LatestUpdate`, which snippet
field values must also feed (a changed field value changes the page's
`Last-Modified`, same argument as the existing comment).

---

### `internal/admin/field.go` + `field_list.html` (handler + template) — the fourth mode

**Analog:** the block-kind mode of the same files, arm for arm.

**Mode selection** (`internal/admin/field.go:79-86`):

```go
	// ?baustein=<id> narrows it to one block kind's fields, the same way.
	var blockType *block.Own
	if id, berr := strconv.ParseInt(r.URL.Query().Get("baustein"), 10, 64); berr == nil {
		if t, gerr := h.blockTypes.Get(r.Context(), websiteID, id); gerr == nil {
			blockType = t
			group = nil
		}
	}
```

**Data struct member + the `Simple()` predicate** (`:38-43`, `:64`) — `Simple`
governs which columns and which form rows appear, and a snippet mode is not simple:

```go
	// BlockType is the block kind whose fields are being edited, or nil for the
	// page's own fields. The third mode of this screen, after the top level and
	// a group.
	BlockType *block.Own
…
func (d FieldListData) Simple() bool { return d.Group == nil && d.BlockType == nil }
```

**Per-mode title, defs source and offered kinds** (`:236-256`):

```go
	switch {
	case blockType != nil:
		defs, err = h.fields.OfBlockType(r.Context(), websiteID, blockType.ID)
		title = "Baustein „" + blockType.Name + "“ – " + websiteName
		kinds = field.BlockKinds()
	case group != nil:
		defs, err = h.fields.Sub(r.Context(), websiteID, group.ID)
		title = "Gruppe „" + group.Label + "“ – " + websiteName
		kinds = field.SubKinds()
	default:
		defs, err = h.fields.List(r.Context(), websiteID)
	}
```

**The redirect target after every mutation** (`:271-281`) — save, delete and move
all end at `fieldPath`, and all three read the carrier back off the def first
(`:194`, `:220`). Four call sites, one function:

```go
func fieldPath(websiteID, parentID, blockTypeID int64) string {
	path := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/felder"
	switch {
	case blockTypeID > 0:
		path += "?baustein=" + strconv.FormatInt(blockTypeID, 10)
	case parentID > 0:
		path += "?gruppe=" + strconv.FormatInt(parentID, 10)
	}
	return path
}
```

**Form → Def** (`:115-133`) — the hidden inputs are read straight off the form:

```go
	parentID, _ := strconv.ParseInt(r.FormValue("gruppe"), 10, 64)
	blockTypeID, _ := strconv.ParseInt(r.FormValue("baustein"), 10, 64)
	def := field.Def{
		WebsiteID:   websiteID,
		ParentID:    parentID,
		BlockTypeID: blockTypeID,
		…
```

**Template arms** (`cmd/holzcloud/templates/admin/field_list.html`): the mode
shows up at `:7-14` (header + back-link), `:32` (list heading), `:37` +`:59`
(columns suppressed outside the simple mode), `:72` (edit link carries the mode),
`:92-93` (empty state), `:100` + `:104-105` (hidden input), `:223`, `:236`
(cancel link). Every one is an `{{if .BlockType}}` and gains an
`{{else if .Snippet}}` sibling.

```html
        {{if .Group}}<input type="hidden" name="gruppe" value="{{.Group.ID}}">{{end}}
        {{if .BlockType}}<input type="hidden" name="baustein" value="{{.BlockType.ID}}">{{end}}
```

**The link into the mode** (`blocktype_list.html:58`) is the model for the
"Felder" button on each snippet row:

```html
<a class="btn btn--sm" href="/admin/websites/{{$.WebsiteID}}/felder?baustein={{.ID}}">{{t "Felder"}}</a>
```

**Route:** no new route. `GET/POST /admin/websites/{id}/felder`
(`cmd/holzcloud/main.go:870-873`) already serves all modes.

---

### `internal/admin/snippet.go` (handler, request-response) — half the phase

**Analog:** `internal/admin/page_fields.go` (view building) + `page_form.go`
(request parsing) + the existing snippet handler's own form shape.

**The values struct grows a `field.Data`** — today (`snippet.go:34-49`):

```go
// SnippetValues is exactly what the form submitted.
type SnippetValues struct {
	ID       int64
	Key      string
	Name     string
	Markdown string
}
```

`page_form.go:68` is the precedent for the new member (`Fields field.Data`), and
`:299` for filling it from storage (`v.Fields = field.Decode(p.Fields)`).

**Request → `field.Data`** — reuse `fieldsFromRequest` (`page_form.go:149-204`)
rather than writing a second parser; it already handles group rows, multi-values
and the `GroupAction` row buttons. Its comment at `:172-175` explains why an
empty value and a missing key encode identically, which is the property that
makes `snippets.fields DEFAULT ''` safe.

**Def → form views** — `fieldViews` (`page_fields.go:165`) → `viewOf` (`:203`) →
`oneView` (`:278`) build `[]FieldBlock`/`FieldView`; the template side is
`field_input.html`, reusable unchanged. `FieldView`'s doc comment (`:28-33`) is
the reason not to branch in the template:

```go
// Built in Go rather than assembled in the template: which input a kind needs
// is a decision with eight branches, and eight branches in a template is where
// a missing `name` attribute hides until somebody notices their entry never
// saved.
```

**Validation and cleaning on save** — `checkFields` (`page_fields.go:422`) and
`field.Clean` / `field.CheckAll` (`internal/field/field.go:564`, `:984`).
Note the open security item **T-07-26**: `field.Hidden` (`field.go:368`) is named
as the mitigation for dropping hidden values on save and is called on no save
path. Phase 8 touches these callers; if the wording is corrected anywhere it is
in `07-SECURITY.md`.

**Error → flash mapping** — the existing snippet handler uses `web.FormErrors`
with re-render (`snippet.go:158-161`, `:185-188`), while the field screen uses
`web.SetFlashError` with a sentence per sentinel (`field.go:135-171`). The
snippet field *values* form belongs to the snippet screen and therefore uses
`FormErrors` + `web.RenderFormError`, not flashes:

```go
	values.validate(data.Errors)
	if data.Errors.Any() {
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}
```

**The rendering pipeline stays exactly as it is** (`snippet.go:163-168`) — D-08
is satisfied by not touching this:

```go
	// The same pipeline page content goes through, so the stored HTML carries
	// the same guarantee and can be cast in a template.
	html, err := page.RenderMarkdown(values.Markdown)
```

---

### `internal/template/loader.go` (contract)

**Analog:** `PageContent.Felder` / `Feldliste` at `:456-467` — D-07 says
`Site.Bausteinfelder` / `Site.Bausteinliste` mirror these two exactly, including
the comments' reasoning about German names:

```go
	// Felder are the website's own fields, resolved to the types they mean:
	// {{ .Page.Felder.preis }} prints a price, {{ if .Page.Felder.verfuegbar }}
	…
	Felder map[string]any
	// Feldliste are the same fields in their defined order, with their labels.
	…
	Feldliste []field.Entry
```

The German-name rule is stated at `:401`: *"German, like Page.Felder, because it
is a name a theme author types."*

**The member that must not change type** (`:372-377`):

```go
	// Snippets are the reusable blocks, by key, so a layout can place one
	// outside the page body: {{index .Site.Snippets "footer-kontakt"}}.
	//
	// The values are already sanitised — they went through the same
	// goldmark-then-bluemonday pipeline as page content.
	Snippets map[string]template.HTML
```

**The append-only rule** (`:355-357`) — the plan must add, never reorder:

```go
// Fields are only ever added, never renamed or removed: an uploaded theme is
// written against this struct and a missing field is a parse error at render
// time, on the visitor's request. New fields are zero-valued for old themes.
```

**Shape:** `Bausteinfelder map[string]map[string]any` and
`Bausteinliste map[string][]field.Entry`, both keyed by snippet key — one nesting
level deeper than `.Page`, which matters for `contractPaths`' `depth > 2` cut-off
(see the fixtures section below).

---

### `internal/public/pagedata.go` (service)

**Analog:** `ownFields` (`:52-60`) and `loadSnippets` (`:232-244`) in the same file.

```go
// ownFields resolves the website's own fields for a theme.
//
// Resolved on the way out rather than stored resolved: a picture chosen last
// month has to pick up this month's crop, and a field whose definition changed
// has to be read the new way without every page being saved again.
```

That paragraph applies unchanged to snippets and is the reason `Bausteinfelder`
is built here from `field.Resolve` (`internal/field/render.go:101`) rather than
stored resolved.

**Twelve assignment sites, all identical** — every public entry point already does
`site.Snippets = h.loadSnippets(r, website.ID).HTML`:
`access.go:73,179`, `handler.go:187,273`, `archive.go:49`, `search.go:35`,
`tag.go:66`, `typearchive.go:65`, `feed.go:76`, `cart.go:74`,
`checkout.go:267,332`, `pluginhost.go:180`, `shop.go:159,218`.
This is the phase's second silent-miss risk after `store.go:59`: a
`Bausteinfelder` filled at eleven of them is invisible on the twelfth. The nil
guard at `loadSnippets:234-236` (`h.snippetStore == nil` → empty map, never nil)
is the pattern that keeps a theme from seeing a nil map.

---

### `internal/template/sample.go` + the four reflection guards (fixtures)

Four tests will fail the build if a `Site` member is added and not fixtured/documented.

**`TestSampleDataFillsEveryField`** (`sample_test.go:20`) walks every exported
field and reports zero values; `walkContract` (`:36-59`) descends into maps and
slices and **fails on an empty one**:

```go
	case reflect.Map, reflect.Slice:
		if v.Len() == 0 {
			*missing = append(*missing, path)
			return
		}
```

So `SampleData` must carry at least one snippet in `Bausteinfelder` *and* one in
`Bausteinliste` — keyed by `"footer-kontakt"`, the key already used at `:58-60`:

```go
			Snippets: map[string]template.HTML{
				"footer-kontakt": "<p>Telefon 07721 123456</p>",
			},
```

**`TestSpecDocumentsEveryFieldOfTheContract`** (`spec_test.go:19-29`) requires the
literal path string in `TEMPLATE-SPEC.md`. Note `contractPaths`' `depth > 2`
cut-off (`:46`): `.Site.Bausteinfelder` is recorded, its inner map is not, so the
spec needs the two top-level paths — but a theme author needs the two-level
indexing spelled out anyway.

**`TestMinimalDataCarriesTheEmptyValueOfEveryOwnField`** (`sample_test.go:224`)
demands that every key in `SampleData().Page.Felder` exists in
`MinimalData().Page.Felder` with the kind's empty value — and that
`MinimalData().Page.Feldliste` is **empty**, because `field.List` never emits an
empty entry. The same asymmetry applies to `Bausteinliste`: `MinimalData` gets
`Bausteinfelder` with empty values and no `Bausteinliste`. The `MinimalData`
comment (`:281-283`) currently reads "no menus, no labels, **no snippets**" —
that sentence needs deciding against, since a website with a defined snippet
field and nothing filled in is precisely the case that breaks a theme.

**`TestSampleFieldsAreShapedLikeTheRendererProducesThem`** (`sample_test.go:123`)
ties the fixture's shapes to what `field.Resolve` actually returns — so
`Bausteinfelder`'s inner values must be built with the same
`field.Term`/`field.Number`/`*time.Time` spellings `SampleData:66-90` uses.

**TEMPLATE-SPEC.md** — two edits: a row in the `.Site` table beside `:212`
(`| `.Site.Snippets` | map | Reusable HTML blocks by key (§7) |`) and prose in §7
beside `:722-726`, whose existing example is the model:

```html
{{with index .Site.Snippets "footer-kontakt"}}<div>{{.}}</div>{{end}}
```

---

### `internal/admin/snippet_fields_test.go` (new)

**Analog:** `internal/admin/page_handler_test.go:33-60` — reuse `newTestAdmin`,
do not build a second harness. It already wires `snippet.NewStore` (`:25` import)
over a real migrated DB and the real on-disk admin templates:

```go
// newTestAdmin builds a handler over a real migrated database and the real
// on-disk admin templates, so a template that no longer matches its data struct
// fails here rather than in the browser.
func newTestAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, *domain.Website) {
```

Sibling tests for the field screen already exist and show the request shape:
`internal/admin/field_defs_test.go`, `page_fields_kinds_test.go`,
`page_fields_multi_test.go`, `page_fields_switch_test.go`.

---

## Shared Patterns

### Every query names its namespace explicitly (D-09)

**Source:** `internal/field/store.go:380-390` (`Move`'s switch)
**Apply to:** every new or edited statement in `store.go`, and the `MaxFields`
count. A `default:` arm that means "the page's own fields" is what a fifth
carrier breaks; each arm names its carrier.

### Website scoping is part of the lookup, not a check afterwards

**Source:** `internal/field/store.go:185-189`
```go
// The website is part of the lookup and not merely checked afterwards: the id
// comes out of an address, and without it an editor could reach another site's
// field by typing its number.
```
**Apply to:** `OfSnippet`, and the snippet mode's `?textbaustein=<id>` lookup.
The snippet handler already does the equivalent check explicitly
(`snippet.go:174-178`, `:211-214`: `if sn == nil || sn.WebsiteID != websiteID`) —
one of the two idioms, consistently.

### One query per website, not one per carrier

**Source:** `internal/field/store.go:137-141` (`OfBlockTypes`) and
`internal/snippet/store.go:153-156` (`LoadRendered`)
**Apply to:** `OfSnippets` and the `Bausteinfelder` build. Both run on every
public page render; the existing comments state the cost argument.

### Sanitisation by reuse, never a second pipeline (D-08)

**Source:** `internal/admin/snippet.go:163-165` and
`internal/snippet/store.go:170-172`
**Apply to:** every snippet field value that reaches a theme as HTML. The
`template.HTML` cast is justified by the pipeline having already run, and that
justification is written next to each cast.

### No JavaScript for ordering or for row buttons

**Source:** `internal/field/store.go:369-371`
```go
// Buttons rather than dragging, for the same reason as everywhere else here:
// dragging needs a script, and the order of eight fields is not worth one.
```
and `internal/admin/page_fields.go:19-24` (`GroupAction`).
**Apply to:** the snippet field screen and the snippet value form.

### Flash text says what did *not* happen

**Source:** `internal/admin/field.go:200-205`
```go
	// Deliberately says what did not happen: the values are still on the pages,
	// and someone who deleted the wrong field should know they can get it back.
```
**Apply to:** deleting a snippet field definition — the values stay in
`snippets.fields` until the snippet is next saved, exactly as on a page.

---

## No Analog Found

None. Every file in this phase has a working precedent in the tree; the phase is
a fourth walk of a path walked three times.

The two places where the analog is a **contrast** rather than a template, and the
plan should say so explicitly:

| File | Why the analog is a contrast |
|---|---|
| `internal/field/store.go` `validate` | A block field is forced non-required (`:530-536`) because a page save must not fail on a half-written block. A snippet has its own form, so `Required` is meaningful — do not copy that arm. |
| `internal/field/store.go` `Update` | Carries no discriminator by design (`:296-306`). Pattern-matching it into the change set would be the phase's other silent break. |

---

## Metadata

**Analog search scope:** `internal/field`, `internal/snippet`, `internal/admin`,
`internal/template`, `internal/tmplspec`, `internal/public`, `internal/page`,
`internal/db/migrations`, `cmd/holzcloud/templates/admin`, `cmd/holzcloud/main.go`
**Files scanned:** 24 (all git-tracked, verified via `git ls-files`)
**Pattern extraction date:** 2026-09-06
