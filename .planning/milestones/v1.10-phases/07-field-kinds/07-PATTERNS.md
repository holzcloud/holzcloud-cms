# Phase 7: Field Kinds - Pattern Map

**Mapped:** 2026-09-05
**Files analyzed:** 12 (all modified; 1 created)
**Analogs found:** 12 / 12 — every change in this phase has an in-tree precedent.

> Every path below is git-tracked source (verified with `git ls-files`). No
> build or install mirror is named.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/field/field.go` (kinds, `SplitValues`/`JoinValues`, `FieldName`, `CheckAll`, `MayControl`) | model / domain | transform | itself — `KindRef` rows, `SplitChoices`/`JoinChoices` (`:614`,`:625`) | exact (self-analog) |
| `internal/field/render.go` (`Resolve`, `List`, `Filled`, `Entry`) | service | transform | `case KindRef` (`:140`) and `case KindImage` in the same switch | exact |
| `internal/field/store.go` (four new columns) | model / store | CRUD | its own `auswahl` column round trip (`:159–166`, `:235`, `:280`) | exact |
| `internal/db/migrations/00046_*.sql` **(new)** | migration | schema | `00044_revision_labels.sql`; `00037_field_conditions.sql:22` | exact |
| `internal/admin/page_form.go` (`fieldsFromRequest`) | controller | request-response | itself, `:150`/`:163` | exact (self-analog) |
| `internal/admin/page_fields.go` (`FieldView`, `oneView`, `switchOf`, `pool`, term list) | controller / view model | request-response | `case field.KindRef` in `oneView` (`:216–223`) + `refPages` (`:236–260`) | exact |
| `cmd/holzcloud/templates/admin/field_input.html` (controls for `zeit`, `bereich`, `code`, `mehrfachauswahl`, button row, `schlagwort`) | template | request-response | the `{{else if .Is "auswahl"}}` and `{{else if .Is "verweis"}}` branches | exact |
| `cmd/holzcloud/assets/admin.css` (`.feld-schalter--knopf`) | config / stylesheet | n/a | `.feld-schalter--auswahl` at `:1104` | exact |
| `internal/block/render.go` (`renderOwn` escaping for `code`) | service | transform | `default:` branch at `:320–322`, `case KindDate` at `:310` | exact |
| `internal/template/sample.go` (`SampleData`, `MinimalData`) | test fixture | n/a | the existing `Entries` rows at `:79–80` | exact |
| `internal/tmplspec/TEMPLATE-SPEC.md` | doc contract | n/a | its own per-kind rows | exact |
| `internal/field/field_test.go` | test | n/a | existing table tests in the same file | exact |

## Pattern Assignments

### `internal/field/field.go` — new kinds + `SplitValues`/`JoinValues` (model, transform)

**Kind declaration pattern** (`field.go:26–49`) — a const with a German value and, where the choice is not obvious, a comment that says *why* it exists rather than what it is:

```go
	// KindRef points at another page of this website — chosen from a list, not
	// typed. The difference from KindLink is what happens afterwards: a typed
	// address goes stale the moment the target is renamed, a reference follows
	// it, and when the target is deleted the theme sees nothing instead of a
	// link into the void.
	KindRef = "verweis"
```

New: `KindTime = "zeit"`, `KindRange = "bereich"`, `KindCode = "code"`, `KindMulti = "mehrfachauswahl"`, `KindTerm = "schlagwort"`. Each also needs a row in the `Kinds` slice with `i18n.N(...)` label and hint — same shape as `{KindSection, i18n.N("Abschnitt"), i18n.N("Keine Eingabe, …")}`.

**Subtractive exclusion pattern** (`BlockKinds()`, `:105`) — D-06 adds `KindTerm` to this `switch`, and *only* `KindTerm`:

```go
func BlockKinds() []Kind {
	out := make([]Kind, 0, len(Kinds)-3)
	for _, k := range Kinds {
		switch k.Kind {
		case KindGroup, KindSection, KindRef:
			continue
		}
		out = append(out, k)
	}
	return out
}
```

Note the `len(Kinds)-3` capacity hint — it tracks the number of exclusions and should become `-4`. `SubKinds()` at `:87` uses the same `if k.Kind != … && …` shape; new kinds stay in it.

**`MayControl` exclusion list** (`:212`) — the shape D-08 may have to extend after the browser pass:

```go
func (d Def) MayControl() bool {
	switch d.Kind {
	case KindGroup, KindSection, KindDate:
		return false
	}
	return d.ParentID == 0
}
```

**The split/join pair to sit beside** (`:614–625`) — `SplitValues`/`JoinValues` copy this exactly, including the drop-empty-lines behaviour D-11 relies on:

```go
// SplitChoices reads the options as the admin types them: one per line.
func SplitChoices(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// JoinChoices writes them back for the textarea.
func JoinChoices(choices []string) string { return strings.Join(choices, "\n") }
```

**`FieldName` — the single minting point** (`:346`), where D-03's `[]` suffix goes:

```go
func (d Def) FieldName() string { return "feld_" + d.Key }
```

**The silent truncation D-13 must replace** (`trimTo`, `:496`):

```go
func trimTo(val string) string {
	val = strings.TrimSpace(val)
	if len(val) > MaxValueBytes {
		val = val[:MaxValueBytes]
	}
	return val
}
```

**`CheckAll` error-reporting pattern** (`:633`) — `max_werte` (D-05) and the shared byte budget (D-13) follow this: keyed by field key, message written for the person filling the form, guarded by `hidden[def.Key]`:

```go
	errs := map[string]string{}
	hidden := Hidden(defs, d.Values)
	for _, def := range defs {
		if !def.HoldsValue() || hidden[def.Key] {
			continue
		}
		if !def.IsGroup() {
			if reason := Check(def, d.Values[def.Key]); reason != "" {
				errs[def.Key] = reason
			}
			continue
		}
```

---

### `internal/field/render.go` — `Resolve`/`List`/`Filled` for `[]string` and `KindTerm` (service, transform)

**Analog:** `case KindRef` (`render.go:140`) — the exact template D-09 copies: parse the stored identity, bail to a typed nil when the lookup fails, never a broken value.

```go
		case KindRef:
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || links.Page == nil {
				out[d.Key] = (*Ref)(nil)
				continue
			}
			ref, ok := links.Page(id)
			if !ok {
				// Deleted, moved to another website, or still a draft on the
				// public site. Nil, so a theme's {{ with }} leaves the link
				// out rather than pointing at a page that is not there.
				out[d.Key] = (*Ref)(nil)
				continue
			}
			out[d.Key] = &ref
```

`KindTerm` differs in one way only: it stores a **slug**, not an id, so no `ParseInt` — the lookup takes the raw string. The lookup itself is injected through `Links` (`render.go:57–60`), which gains a third member:

```go
type Links struct {
	Image Lookup
	Page  RefLookup
}
```

**`List` must gain a `case []string`** beside the existing type switch (`render.go:~205`), which currently handles `string`, `Number`, `bool`, `*time.Time`, `*Image`, `*Ref`. Each case's first act is the skip-when-empty guard — for a slice that is `len(v) == 0`:

```go
		switch v := resolved[d.Key].(type) {
		case string:
			if v == "" {
				continue
			}
			e.Text = v
```

**`Filled`** (`:253`) is the same type switch inverted (`return true`), and needs the same `case []string`. Missing it means a theme's whole field panel disappears when a page carries only multi-valued fields.

---

### `internal/admin/page_form.go` — the `[]` suffix and the sentinel (controller, request-response)

**Analog:** itself. The two lines D-02/D-03 change, in context — note the doc comment already explains *why* the prefix read happens before definitions load, so the fix must stay inside this loop:

```go
		if key, ok := strings.CutPrefix(name, "feld_"); ok && key != "" {
			out.Values[key] = values[0]
			continue
		}
```

and the group-row twin at `:163`:

```go
		rows[group][index][sub] = values[0]
```

The `parseRowName`/`ok` guard-clause style above it is the shape the `[]`-suffix test should follow: `strings.CutSuffix(key, "[]")` → branch, no early definition load.

---

### `internal/admin/page_fields.go` — the Term chooser and `switchOf` (controller, request-response)

**Analog for the chooser:** `KindRef`, in three places.

1. **The choice type** (`:97–105`) — `PageChoice`; `TermChoice` is its twin, carrying `Slug` and `Name` (no `Draft` equivalent exists for a term):

```go
type PageChoice struct {
	ID    int64
	Title string
	Slug  string
	Draft bool
}
```

2. **`pool`** (`:107–118`) — the doc comment already anticipates D-09; add `terms []TermChoice` here and extend `PageFormData.pool()`, and no call site changes:

```go
type pool struct {
	media []media.Media
	pages []PageChoice
}

func (d PageFormData) pool() pool { return pool{media: d.Media, pages: d.RefPages} }
```

3. **`oneView`** (`:211–229`) — the per-kind hydration switch. `case field.KindTerm` copies the `KindRef` arm minus the `ParseInt`; `case field.KindMulti` sets `v.Selected = field.SplitValues(value)`:

```go
	switch d.Kind {
	case field.KindBool:
		v.Checked = value != "" && value != "0"
	case field.KindImage:
		v.Media = p.media
		v.MediaID, _ = strconv.ParseInt(value, 10, 64)
	case field.KindRef:
		v.Pages = p.pages
		v.PageID, _ = strconv.ParseInt(value, 10, 64)
		for _, c := range p.pages {
			if c.ID == v.PageID {
				v.RefDraft = c.Draft
			}
		}
	}
```

4. **The loader** — `refPages` (`:236–260`) is the analog for a `siteTerms` helper: nil store → nil slice, error → nil slice (never a failed request), bounded, mapped into the view type. `internal/term/store.go:234` `ListAll(ctx, websiteID)` already returns exactly what is needed (`Term{ID, Slug, Name}`), so the new helper is shorter than `refPages`:

```go
func (h *Handler) refPages(ctx context.Context, websiteID int64) []PageChoice {
	if h.pages == nil {
		return nil
	}
	list, _, err := h.pages.ListPages(ctx, websiteID, page.ListFilter{...})
	if err != nil {
		return nil
	}
	...
}
```

**`switchOf`** (`:186–195`) — D-10's fifth case goes here. The comment above it is the reason the new name is required:

```go
func switchOf(kind string) string {
	switch kind {
	case field.KindBool:
		return "kreuz"
	case field.KindChoice, field.KindImage, field.KindRef:
		return "auswahl"
	default:
		return "text"
	}
}
```

Careful: `switchOf` takes only a `string` kind. A button row is a `darstellung` on `KindChoice`, so this signature must widen to take the `field.Def` (or a second argument) — that is a real, small refactor and its only call site is `viewOf` at `:180`:

```go
	if len(v.Dependent) > 0 {
		v.Switch = switchOf(d.Kind)
	}
```

`FieldView.Is` (`:94`) is what the template branches on and stays as is; a button row needs a second predicate (e.g. `func (v FieldView) Buttons() bool`) rather than a fake kind.

---

### `cmd/holzcloud/templates/admin/field_input.html` — new controls (template, request-response)

**Analog:** the `auswahl` branch (list with an explicit empty option — D-11's radio row is the same idea in radios) and the `verweis` branch (identity → label).

```html
    {{else if .Is "auswahl"}}
    <select id="{{.Name}}" name="{{.Name}}" class="form-input form-select">
        <option value="">{{if .Def.Required}}{{t "bitte wählen"}}{{else}}{{t "– keine Angabe –"}}{{end}}</option>
        {{$v := .Value}}
        {{range .Def.Choices}}<option value="{{.}}"{{if eq . $v}} selected{{end}}>{{.}}</option>{{end}}
    </select>
```

Conventions this file enforces and every new branch must keep:
- `{{else if .Is "<kind>"}}` chained inside one `{{if}}`; `janein` is special-cased above the label because a checkbox carries its own label.
- Every user-visible string through `{{t "…"}}`.
- `id="{{.Name}}" name="{{.Name}}" class="form-input"`.
- **`{{if eq .Switch "text"}}placeholder=" "{{end}}`** on any typed control — the comment at the top of the file explains it, and D-08's `bereich` number input needs it for `.feld-schalter--text` to work at all.
- The `verweis` branch's `{{$id := .PageID}}` … `{{if eq .ID $id}} selected{{end}}` is the shape for the Term `<select>` with `{{$slug := .Value}}`.
- Trailing `{{if .RefDraft}}…{{end}}` shows how a kind-specific note is appended after the control.

For `mehrfachauswahl` (D-12) the checkbox group has no analog in this file; the closest markup precedent is the `janein` `<label class="form-check">` wrapper above. D-11's sentinel is new:
`<input type="hidden" name="{{.Name}}" value="">` immediately before the group.

---

### `cmd/holzcloud/assets/admin.css` — `.feld-schalter--knopf` (config, n/a)

**Analog:** the rule D-10 says a button row breaks, `admin.css:1096–1110`:

```css
    .feld-schalter--kreuz > .feld-abhaengig {
        display: none;
    }

    .feld-schalter--kreuz:has(> .form-group input[type="checkbox"]:checked) > .feld-abhaengig {
        display: block;
    }

    .feld-schalter--auswahl:has(> .form-group option[value=""]:checked) > .feld-abhaengig {
        display: none;
    }

    .feld-schalter--text:has(> .form-group input:placeholder-shown) > .feld-abhaengig,
    .feld-schalter--text:has(> .form-group textarea:placeholder-shown) > .feld-abhaengig {
        display: none;
    }
```

Two shapes are on offer: `kreuz` is show-on-match (default hidden), `auswahl`/`text` are hide-on-match (default shown). The button row copies **`auswahl`**, swapping the selector per D-10:

```css
    .feld-schalter--knopf:has(> .form-group input[type="radio"][value=""]:checked) > .feld-abhaengig {
        display: none;
    }
```

Indentation is four spaces because these rules live inside an `@layer` block — match the surrounding file, do not reindent.

---

### `internal/db/migrations/00046_*.sql` — four columns on `page_field_defs` (migration, schema)

**Analog:** `00044_revision_labels.sql` in full (shape, brevity, a `Down` that reverses) and `00037_field_conditions.sql:22` (the same table, the same `TEXT NOT NULL DEFAULT ''` idiom):

```sql
-- +goose Up
-- Eine Beschriftung für eine Fassung. Ein Verlauf aus zwanzig Zeitstempeln
-- beantwortet nicht, welche davon die war, die vor dem Umbau galt — und genau
-- diese sucht man, wenn man den Verlauf überhaupt öffnet.
ALTER TABLE page_revisions ADD COLUMN label TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE page_revisions DROP COLUMN label;
```

```sql
ALTER TABLE page_field_defs ADD COLUMN bedingung TEXT NOT NULL DEFAULT '';
```

Conventions: comments in German, stating the reason not the mechanics; one `ALTER` per line; no `-- +goose StatementBegin` needed for a plain `ALTER` (that wrapper appears only around `CREATE TABLE`/`TRIGGER` in `00045`). D-14's no-CHECK precedent is the `art` column comment in `00028:25–27`:

```sql
    -- art ist eine der acht Eingabearten. Kein CHECK: eine neue Art wäre sonst
    -- wieder ein Tabellenumbau, und geprüft wird ohnehin beim Speichern.
    art TEXT NOT NULL,
```

(That count "acht" is now stale and is a natural thing for the D-01 docs task to leave alone — it is a migration and migrations are never edited. Do not touch `00028`.)

---

### `internal/field/store.go` — reading and writing the new columns (model, CRUD)

**Analog:** the `auswahl` column, which is the exact precedent for a text column that is a list on the Go side (`store.go:159–166`):

```go
		auswahl string
	...
		&pflicht, &d.Hint, &auswahl, &d.AppliesTo, &d.Position, &d.Condition, &d.BlockTypeID); err != nil {
	...
	d.Choices = SplitChoices(auswahl)
```

The column list appears **five times** (`:50`, `:86`, `:109`, `:135`, `:177`) plus the `INSERT` (`:235`) and `UPDATE` (`:280`). Every new column must be added to all seven or a field silently loads with a zero value — this is the highest-risk mechanical edit in the phase.

---

### `internal/block/render.go` — `code` escaping in the frozen HTML (service, transform)

**Analog:** the `default:` arm of `renderOwn`'s per-kind switch (`:320–322`) — this is where a `code` field would land today, and `html.EscapeString` is already the house rule:

```go
		default:
			fmt.Fprintf(&inner, `<p class="hc-eigen__zeile hc-eigen__zeile--%s">%s</p>`,
				key, html.EscapeString(value))
```

D-06's requirement (HTML typed into a `code` field appears verbatim and does not execute, **including inside a block**) is satisfied by an explicit `case field.KindCode` emitting `<pre><code …>` with the same `html.EscapeString(value)`. Follow `case field.KindDate` (`:310–318`) for the shape: a kind-specific element, a `hc-eigen__<ding>--<key>` class pair, everything escaped.

Also check `PlainText` (`:420`), whose whitelist decides what the site search indexes:

```go
				switch d.Kind {
				case field.KindText, field.KindLong, field.KindChoice:
					add(b.Fields[d.Key])
				}
```

`KindMulti` and `KindCode` are candidates here; `KindTerm` is excluded from blocks entirely by D-06.

---

### `internal/template/sample.go` + `internal/tmplspec/TEMPLATE-SPEC.md` — the per-kind tax (fixture / doc)

**Analog:** the existing `Entries` rows (`sample.go:79–80`) and the `Felder` map above them (`:74`):

```go
				{Key: "holzart", Label: "Holzart", Kind: "text", Value: "Eiche", Text: "Eiche"},
				{Key: "lieferzeit", Label: "Lieferzeit", Kind: "text", Value: "4 Wochen", Text: "4 Wochen"},
```

Every new kind needs a row here **and** in `MinimalData` (the empty-everything fixture that catches `{{.Page.Next.URL}}`-class bugs), **and** a section in `TEMPLATE-SPEC.md`. Three tests tie the three together — this is the per-kind tax, paid once per kind, five kinds this phase.

## Shared Patterns

### Guard clauses and early `continue`
**Source:** `internal/field/render.go:140`, `internal/block/render.go:265–272`, `internal/admin/page_fields.go:236`
**Apply to:** every new switch arm. A lookup that fails yields a typed nil / a skipped entry, never an error to the caller and never a half-rendered value.

### Escape at the boundary
**Source:** `internal/block/render.go` — `html.EscapeString` on every interpolated value, `key` escaped once at `:273`
**Apply to:** the `code` block arm, and any new `renderOwn` case. Blocks build raw HTML strings, so nothing is escaped for you.

### German comments that state the reason
**Source:** `00044_revision_labels.sql`, `00028_page_fields.sql:18–27`, `internal/field/field.go:26–49`
**Apply to:** migration `00046`, every new kind const, `SplitValues`/`JoinValues`. The house style explains *why this and not the obvious alternative* — D-05, D-07 and D-12 each supply that sentence already.

### `{{t "…"}}` on every string in a template
**Source:** `cmd/holzcloud/templates/admin/field_input.html` throughout
**Apply to:** the new controls, including D-11's "— keine Angabe —" radio label.

### One place mints, everywhere reads
**Source:** `Def.FieldName()` (`field.go:346`), `pool` (`page_fields.go:107`)
**Apply to:** D-03's `[]` marker and D-09's term list — neither may be spelled out at a call site.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| checkbox group + hidden sentinel in `field_input.html` (D-11, D-12) | template | request-response | No multi-valued control exists in the tree; the nearest markup precedent is the single `janein` `.form-check` label. Sentinel semantics (present-but-empty = cleared, absent = untouched) have no precedent at all — `blocksFromRequest`/`fieldsFromRequest` currently never distinguish the two. |
| radio-row rendering of `auswahl` (D-04) | template | request-response | Every existing choice control is a `<select>`; no radio group exists in the admin templates. |

## Metadata

**Analog search scope:** `internal/field/`, `internal/admin/`, `internal/block/`, `internal/term/`, `internal/template/`, `internal/db/migrations/`, `cmd/holzcloud/templates/admin/`, `cmd/holzcloud/assets/`
**Files scanned:** 14
**Pattern extraction date:** 2026-09-05
