---
phase: 07-field-kinds
reviewed: 2026-09-05T00:00:00Z
depth: standard
files_reviewed: 34
files_reviewed_list:
  - cmd/holzcloud/assets/admin.css
  - cmd/holzcloud/templates/admin/field_input.html
  - cmd/holzcloud/templates/admin/field_list.html
  - docs/offene-punkte.md
  - internal/admin/field.go
  - internal/admin/field_defs_test.go
  - internal/admin/page.go
  - internal/admin/page_fields.go
  - internal/admin/page_fields_kinds_test.go
  - internal/admin/page_fields_multi_test.go
  - internal/admin/page_fields_switch_test.go
  - internal/admin/page_form.go
  - internal/ai/ai_test.go
  - internal/ai/tools.go
  - internal/block/block_test.go
  - internal/block/render.go
  - internal/bundle/bundle_test.go
  - internal/bundle/export.go
  - internal/bundle/format.go
  - internal/bundle/import.go
  - internal/db/migrations/00046_field_kinds.sql
  - internal/field/field.go
  - internal/field/field_test.go
  - internal/field/render.go
  - internal/field/store.go
  - internal/field/store_test.go
  - internal/field/values_test.go
  - internal/public/handler_test.go
  - internal/public/pagedata.go
  - internal/template/sample.go
  - internal/template/sample_test.go
  - internal/term/store.go
  - internal/tmplspec/TEMPLATE-SPEC.md
  - internal/tmplspec/spec_test.go
findings:
  critical: 2
  warning: 10
  info: 3
  total: 15
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-09-05
**Depth:** standard
**Files Reviewed:** 34
**Status:** issues_found

## Summary

The phase is faithful to D-01…D-14 in almost every place I could check by
tracing, and several of the traps the context named were closed properly: all
seven `page_field_defs` SQL sites agree in column and scan order
(`internal/field/store.go:57, 94, 118, 145, 193, 251, 301`); every
`switchOf` name has a matching `.feld-schalter--*` rule that selects markup the
control actually emits, and a test asserts the pair rather than the class alone;
the `code` field is escaped in the block render path with `html.EscapeString`
and never reaches `template.HTML`; the term lookup is website-scoped through
`ListAll` and has a cross-site test; `trimTo` really did stop truncating and
`Check` really does measure the joined value including separators; the
migration is additive with no CHECK; no JavaScript was added anywhere and the
new CSS uses a local `--font-mono` stack. `go build`, `go vet` and `go test ./...`
are all clean.

Two defects still ship, and both are silent by construction.

The first is a **multiple-choice field inside a website-defined block kind: it
always saves as empty.** The block editor builds its field names without
`Def.NameSuffix()`, so the `[]` marker D-03 depends on is missing, and
`block.FromForm` keeps `values[0]` — which is the hidden sentinel D-11 puts
*first*. Every tick the editor makes is discarded on every save, with no error.
Proven with a probe test against `FromForm`.

The second is in the bundle importer: `importTerms` funnels the whole manifest
term list through `term.Parse`, which is the **per-page** parser and stops at
`term.MaxPerPage` (12). A website with more than twelve labels loses every
label past the twelfth that no page happens to carry — which is exactly the case
`importTerms` was written to cover — and a `schlagwort` field pointing at one
resolves to nothing on the copy. Proven with a probe against `term.Parse`.

Beyond that, the removal of `trimTo`'s truncation (D-13) left `MaxValueBytes`
enforced only where `CheckAll` runs, and `CheckAll` deliberately skips two
classes of field that `field.Clean` still stores.

## Critical Issues

### CR-01: A multiple-choice field inside a block kind discards every tick

**File:** `internal/admin/page_blocks.go:243`, `internal/block/form.go:70`, `cmd/holzcloud/templates/admin/field_input.html:116`

**Issue:** `BlockKinds()` (`internal/field/field.go:167-177`) excludes only
`KindGroup`, `KindSection`, `KindRef` and `KindTerm`, so `mehrfachauswahl` is
offered inside a website-defined block kind. But `blockViews` builds the control
name as

```go
oneView(d, prefix+".f."+d.Key, b.Fields[d.Key], pool{media: items}, "")
```

with no `d.NameSuffix()`. The rendered markup is therefore a checkbox group
under `b0.f.hoelzer` — no `[]` — preceded by D-11's hidden sentinel under the
same name. `block.FromForm` reads `value := values[0]`, and because the sentinel
is emitted first (`field_input.html:116` before the `<div role="group">` on
:117), `values[0]` is the **empty string**. The field is stored as `""` on every
save, no matter what is ticked, and nothing reports it.

Proven:

```
form["b0.f.hoelzer"] = []string{"", "Eiche", "Buche"}
FromForm(form) → Fields:map[hoelzer:]      // both ticks lost
```

This is the exact pitfall D-03 was written to prevent; the marker is minted in
one place (`Def.FieldName`/`NameSuffix`) for the page form and the group row
(`page_fields.go:210, 271`) but the third naming site was not converted.

Note that even without the sentinel, `values[0]` would keep only the first
ticked option — so the sentinel is not the whole bug, it just turns "keeps one"
into "keeps none".

**Fix:** two halves; both are needed.

```go
// internal/admin/page_blocks.go:243
oneView(d, prefix+".f."+d.Key+d.NameSuffix(), b.Fields[d.Key], pool{media: items}, "")
```

```go
// internal/block/form.go — in setBlockField's "f." branch, mirror
// fieldsFromRequest instead of taking values[0]:
if key, ok := strings.CutPrefix(field, "f."); ok {
    if trimmed, multi := strings.CutSuffix(key, "[]"); multi {
        if trimmed == "" { return }
        b.Fields[trimmed] = field.JoinValues(values)   // pass the whole slice in
        return
    }
    ...
}
```

`FromForm` currently narrows to `values[0]` at line 70 before dispatching, so
the slice has to be threaded down to `setBlockField` (or the `f.` branch lifted
into `FromForm`). Add a test that drives a block kind's multi field through
`FromForm` **and** through the page-save handler — see IN-01.

---

### CR-02: Bundle import silently drops every label past the twelfth

**File:** `internal/bundle/import.go:313`

**Issue:**

```go
n, err := s.Terms.EnsureNames(ctx, websiteID, term.Parse(strings.Join(names, ", ")))
```

`term.Parse` (`internal/term/store.go:58-79`) is the parser for the **per-page**
comma-separated label field. It carries `MaxPerPage = 12` and breaks out of the
loop at that count. Joining the manifest's whole `Terms` list into one string
and handing it to `Parse` therefore caps the import at twelve labels.

Proven:

```
in=20 out=12 [awort bwort … lwort]
```

Consequences:

1. Labels 13+ that no page carries are never created — which is precisely the
   gap `importTerms` exists to close (its own doc comment says so).
2. A `schlagwort` field whose value slugifies to one of those labels resolves to
   `nil` on the copy, so `List` drops the entry and the theme prints nothing.
   The round trip this phase built is broken for any site with >12 labels.
3. `report.Terms` reports the truncated count with no warning, so the operator
   is told the import succeeded.

`Parse` also splits on commas, so a label whose name contains one is torn in two
(`Parse("Möbel, Bau, Eiche")` → `[Möbel Bau Eiche]`). Not reachable through the
admin UI today, but a manifest is a hand-editable file.

**Fix:** do not route a list through a parser for a single string. Normalise
each name on its own and drop the per-page cap:

```go
names := make([]string, 0, len(m.Terms))
for _, t := range m.Terms {
    // one name per entry: term.Parse folds a single field's comma list and
    // caps at MaxPerPage, neither of which applies to a site's whole archive.
    if n := term.Normalize(t.Name); n != "" {   // extract from Parse's body:
        names = append(names, n)                // Fields-join + MaxNameLength trim
    }
}
n, err := s.Terms.EnsureNames(ctx, websiteID, names)
```

`EnsureNames` already de-duplicates by slug through `ON CONFLICT`, so the
`seen` map in `Parse` is not needed here either. Add a test with 15 labels, of
which the last is carried by no page and is the target of a `schlagwort` field.

## Warnings

### WR-01: `MaxValueBytes` is no longer enforced on any value `CheckAll` skips

**File:** `internal/field/field.go:623`, `internal/field/field.go:943`, `internal/admin/page.go:447` and `:610`

**Issue:** Before this phase `trimTo` truncated every stored value to
`MaxValueBytes` inside `Clean` (`60ff5b2:internal/field/field.go:494-500`), so
the bound held unconditionally. D-13 correctly moved the *report* into `Check`
— but removed the last unconditional guard, and `CheckAll` skips two classes of
field on purpose:

- `field.go:943` — a field whose condition is not met (`hidden[def.Key]`).
- the caller filters: `checkFields` validates `field.For(defs, pageKind)` while
  the save at `page.go:447` / `:610` runs `field.Clean(defs, …)` over the
  **unfiltered** defs.

`Clean` keeps both (hidden values are kept deliberately; `simple[def.Key]` is
built from all defs), so a value in either class reaches `page_field_defs`'
JSON with no length bound and no per-kind check at all. A hand-built POST
carrying `feld_<key-of-a-post-only-field>=<10 MB>` on a page save is stored
verbatim. Nothing renders it (both `Resolve` and `ownFields` filter the same
way), so this is bloat and a check bypass rather than an XSS, but it is a
regression against `60ff5b2`.

Note the AI path does it right — `internal/ai/tools.go:669-673` uses the same
filtered `mine` for both `CheckAll` and `Clean`.

**Fix:** make the two agree, and keep a hard bound that no filter can skip.

```go
// internal/admin/page.go:447 and :610
storedFields, err := field.Encode(field.Clean(field.For(defs, values.KindValue()), values.Fields))
```

and in `field.Clean`, refuse rather than store an over-long value:

```go
for key, val := range d.Values {
    if !simple[key] || len(val) > MaxValueBytes {
        continue // Check reports it; Clean must not be the way around Check
    }
    ...
}
```

---

### WR-02: A `mehrfachauswahl` may be created with no options; if required it wedges every page save

**File:** `internal/field/store.go:442`, `cmd/holzcloud/templates/admin/field_list.html:136`

**Issue:** `validate` refuses a `KindChoice` with an empty option list but says
nothing about `KindMulti`:

```go
if d.Kind == KindChoice && len(d.Choices) == 0 {
    return errors.New("eine Auswahl braucht mindestens eine Möglichkeit")
}
```

A `mehrfachauswahl` with no options renders a group containing nothing but the
hidden sentinel, so it can never hold a value. If it is also marked required,
`Check` returns "… muss ausgefüllt werden." on every save of every page the
field applies to, and the form offers no control that could satisfy it — the
page becomes unsaveable until the definition is fixed, with the error pointing
at a field that shows no input.

The field-definition form makes this the likely path rather than the exotic one:
the "Möglichkeiten" hint at `field_list.html:136` reads *"Nur für die Art
„Auswahl“"*, which tells an operator defining a multiple choice that the box
does not apply to them.

**Fix:**

```go
if (d.Kind == KindChoice || d.Kind == KindMulti) && len(d.Choices) == 0 {
    return errors.New("eine Auswahl braucht mindestens eine Möglichkeit")
}
```

and correct the hint to name both kinds.

---

### WR-03: `bereich` accepts a comma decimal server-side that its own control cannot carry

**File:** `cmd/holzcloud/templates/admin/field_input.html:54-56`, `internal/field/field.go:724-737`, `internal/field/store.go:456-468`

**Issue:** `Check` parses a range value and both bounds with `ParseNumber`,
which treats `,` as a decimal separator (`field.go:639`). The control D-07
mandates is `<input type="number">`, which is locale-independent in the HTML
spec: a `value` attribute that is not a valid floating-point number leaves the
control's value empty, and a `min`/`max` that is not one is ignored.

So:

- `min="1,5"` and `max="9,5"` are rendered verbatim and **silently do not
  constrain the browser control** — the bound exists only server-side.
- A stored value of `1,5` — which `Check` accepts, and which the AI tools
  (`ai/tools.go:669`) and the bundle importer both can write — renders into
  `value="1,5"`, the control shows empty, and the next save of that page submits
  `""`. `Clean` drops it: the value is **lost without a word**.

Note `KindNumber` deliberately avoids this by using `type="text"
inputmode="decimal"` (`field_input.html:43`). The divergence is a consequence of
D-07, not a deviation from it, but the comma interaction was not carried across.

Separately, `validate` never rejects a bound that is not a number at all: it
only compares the pair when *both* parse (`store.go:458-468`), so `min="ungefähr
drei"` is stored and rendered into the attribute.

**Fix:** normalise on the way in and refuse a non-numeric bound.

```go
// internal/field/store.go validate(), for KindRange:
for _, p := range []*string{&d.RangeMin, &d.RangeMax} {
    if *p == "" { continue }
    n, ok := ParseNumber(*p)
    if !ok {
        return errors.New("eine Grenze muss eine Zahl sein")
    }
    *p = strconv.FormatFloat(n, 'f', -1, 64) // store the dot form the control needs
}
```

and do the same for the value in `Check`/`Clean`, or switch the control to the
`type="text" inputmode="decimal"` shape `zahl` already uses and enforce the
bounds server-side only.

---

### WR-04: `Def.Key` is never shape-checked, so the `[]` marker can collide after a bundle import

**File:** `internal/field/store.go:433-438`, `internal/bundle/import.go:342` and `:360`

**Issue:** `Def.FieldName`'s doc comment (`field.go:466-468`) states the
invariant the whole D-03 scheme rests on: *"A key is slug-like and cannot
contain a bracket, so the marker cannot collide with one."* Nothing enforces it.
`validate` derives a key from the label **only when it is empty**:

```go
if d.Key == "" { d.Key = SlugifyKey(d.Label) }
if d.Key == "" { return errors.New(...) }
```

There is no `validKey(d.Key)` call — `validKey` is applied to `Condition` and
`AppliesTo` only. `importFields` takes `Key: f.Key` straight out of an uploaded
manifest (`import.go:342`, and `:360` / `:405` for sub- and block-fields), which
is the one path a key can enter with an arbitrary shape.

Two collisions follow from that:

- a field keyed `farbe[]` mints `feld_farbe[][]`; another keyed `farbe` mints
  `feld_farbe`. `fieldsFromRequest` (`page_form.go:162`) cuts the suffix and
  writes both into `Values["farbe"]` — one field silently overwrites the other.
- a key containing `.` breaks `RowKey`/`parseRowName` and `splitRowKey`
  (`import.go:526`), all three of which assume exactly three dot-separated parts.

**Fix:** enforce the invariant where keys are minted, not only where they are
derived:

```go
// internal/field/store.go validate()
if d.Key == "" { d.Key = SlugifyKey(d.Label) }
if !validKey(d.Key) {
    return errors.New("eine Kennung besteht aus Kleinbuchstaben, Ziffern und Unterstrichen")
}
```

`SlugifyKey` already produces only `[a-z0-9_]`, so no existing admin-created
field is affected.

---

### WR-05: The "cleared vs. untouched" distinction the sentinel exists for does not survive to storage

**File:** `internal/admin/page_form.go:164-172`, `internal/field/field.go:579`

**Issue:** `fieldsFromRequest` documents, and D-11 specifies, that
`out.Values[trimmed] = ""` means *cleared* and an absent key means *the form
never carried this field*. Traced through, the two are indistinguishable:

- `field.Clean` (`field.go:579`) drops any value that trims to `""`, so a
  present-but-empty key and an absent key produce byte-identical stored JSON.
- `CheckAll` and `Hidden` read `d.Values[key]`, which returns `""` for a missing
  key too.

The sentinel is harmless — it does no damage and it correctly stops a partly
ticked group from noticing it, as `JoinValues` drops the empty entry — but it
buys nothing today, and the comment asserts a contract the pipeline does not
honour. FIELD-07 exists so Phase 9's CSV importer inherits this mechanism; an
importer written against that comment (e.g. "an absent column leaves the stored
value alone") would be wrong.

**Fix:** either implement the distinction (make `Clean` distinguish "key
present, empty" from "key absent", which means changing `field.Values` to carry
presence) or, cheaper and honest, correct the comment to say what the sentinel
actually guarantees — that the group's key is *always* present so a future
merge-style writer has something to key on — and note that the current save path
is a full replacement.

---

### WR-06: A rejected group value leaves an empty row in the imported page

**File:** `internal/bundle/import.go:494-511`

**Issue:** After `field.Clean`, `importFieldValues` walks `CheckAll`'s errors and
`delete(rows[i], sub)` for each rejected sub-value. `Clean` is not run again, so
a row whose only filled value was the rejected one is stored as an empty
`Values{}` inside `data.Rows[group]`. `field.Encode` writes it, and on render
`Resolve`/`List` produce an empty row that a theme's `{{range}}` will draw as a
blank table line — the one thing `cleanRow`'s doc comment says must not happen
("A row where everything was left blank disappears").

**Fix:** re-clean after the deletions.

```go
if len(dropped) > 0 {
    data = field.Clean(defs, data)
}
sort.Strings(dropped)
```

---

### WR-07: The spec promises a `bereich` bound the program does not keep

**File:** `internal/tmplspec/TEMPLATE-SPEC.md` (the `bereich` paragraph)

**Issue:** The new prose says the bounds

> are enforced before anything is stored, so a theme never has to check them and
> never sees a number outside them.

`Check` runs at save time against the bounds *as they are then*. An operator who
later tightens `min`/`max` on an existing field leaves every already-saved page
carrying an out-of-range value until that page is saved again — nothing
re-validates stored data. Per `CLAUDE.md` the spec is a contract that a template
author (often an AI agent) follows literally, so a theme written against this
sentence will index into a bounded array or size a bar chart without a guard.

**Fix:** state the actual guarantee — "checked against the bounds at the moment
the value is saved; a bound tightened afterwards does not rewrite pages already
stored, so guard if the number drives layout."

---

### WR-08: `renderOwn` gained no `KindMulti` arm although `PlainText` did

**File:** `internal/block/render.go:275-334` vs `:437-449`

**Issue:** `PlainText` was given an explicit `KindMulti` case that joins through
`field.SplitValues` with a space, with a comment explaining why the stored
newlines must not leak into an excerpt. `renderOwn` — the path that produces the
HTML a visitor actually sees — has no such case, so a multi value falls into the
`default` branch and is printed as one escaped blob with its stored newlines
inside a single `<p class="hc-eigen__zeile">`. HTML collapses the newlines, so
the values run together as `Eiche Buche` with no separator a theme can style
and no per-value element.

Currently masked by CR-01 (the value is always empty), which is why no test
caught it.

**Fix:** give it the same treatment `PlainText` got.

```go
case field.KindMulti:
    fmt.Fprintf(&inner, `<ul class="hc-eigen__werte hc-eigen__werte--%s">`, key)
    for _, v := range field.SplitValues(value) {
        fmt.Fprintf(&inner, `<li>%s</li>`, html.EscapeString(v))
    }
    inner.WriteString(`</ul>`)
```

---

### WR-09: `report.Terms` is not the number of labels created

**File:** `internal/term/store.go:305-329`, `internal/bundle/import.go:318-320`

**Issue:** `EnsureNames` increments `n` for every name that produced a usable
slug, including those the `ON CONFLICT … DO NOTHING` left untouched because the
label already existed. `importTerms` then writes it to `report.Terms` under the
comment *"Was angelegt wurde, nicht was das Archiv behauptet: eine Zahl, die ein
Bericht nennt, soll geglaubt werden können."* On an import into a site that
already has some of the labels, the number is larger than what was created — the
opposite of the stated intent, and combined with CR-02 the operator is given a
number that is wrong in both directions.

**Fix:** count rows actually inserted.

```go
res, err := tx.ExecContext(ctx, `INSERT INTO terms … ON CONFLICT … DO NOTHING`, …)
if err != nil { return 0, fmt.Errorf("create term %q: %w", name, err) }
if rows, _ := res.RowsAffected(); rows > 0 { n++ }
```

---

### WR-10: `JoinValues` does not defend the delimiter it owns

**File:** `internal/field/field.go:919-927`

**Issue:** `SplitValues`/`JoinValues` are exported specifically so Phase 9's CSV
importer inherits them (D-02), and their doc comment argues the newline is safe
because *"the options a value is drawn from are themselves read one per line."*
That argument holds for `KindMulti` values coming out of the option list — it
does not hold for the exported function, which trims and joins whatever it is
given. `JoinValues([]string{"a\nb"})` returns `"a\nb"`, and `SplitValues` turns
it back into two values. Round-trip identity, which the same comment promises
("saving the same form twice produces the same string byte for byte"), is lost
for exactly that input.

Reachable today only through a hand-built POST on a multi field, where `Check`
then rejects both halves against the option list — so no live corruption. It is
a trap laid for the next caller.

**Fix:** make the invariant the function's own, not the caller's.

```go
func JoinValues(values []string) string {
    out := make([]string, 0, len(values))
    for _, v := range values {
        // The separator cannot occur inside a value; a caller that hands one in
        // is spelling two values, and guessing which is meant is worse than
        // refusing to guess.
        if v = strings.TrimSpace(strings.ReplaceAll(v, "\n", " ")); v != "" {
            out = append(out, v)
        }
    }
    return strings.Join(out, "\n")
}
```

## Info

### IN-01: No test drives a multi field of a block kind through the form

**File:** `internal/block/block_test.go:637-641`, `internal/admin/page_fields_multi_test.go`

**Issue:** `TestPlainTextNimmtCodeUndMehrfachauswahl` sets
`Fields: map[string]string{"sorten": "Eiche\nBuche"}` directly — a value the
form path can never produce (CR-01). `page_fields_multi_test.go` covers the page
form and group rows thoroughly but never a block kind. This is the shape the
phase brief warned about: a test that stays green against a broken
implementation because it assembles the state the implementation fails to build.

**Fix:** add a case to `TestFormularWirdInBausteineGelesen` that posts
`b0.f.<key>[]` with a sentinel and two ticks and asserts both survive; and a
handler-level case in `admin` that saves a page whose block kind carries a
multi field and reads the stored JSON back.

### IN-02: `.Text` of a `zeit` entry is the raw stored value, not `HH:MM`

**File:** `internal/field/render.go:322-323`, `internal/tmplspec/TEMPLATE-SPEC.md` (`zeit` row)

**Issue:** `ParseTimeOfDay` accepts `15:04:05` because "manche Browser schicken
sie mit", but `List` sets `e.Text = strings.TrimSpace(data.Values[d.Key])` — the
stored string. A value submitted with seconds prints as `16:30:05`, while the
spec says `.Text` "is it as `HH:MM`". `SampleData` only exercises the `16:30`
case, so the suite agrees with the spec by accident.

**Fix:** format from the parsed time — `e.Text = t.Format("15:04")` — or widen
the spec sentence.

### IN-03: `pruefeFelder` returns an arbitrary one of several reasons

**File:** `internal/ai/tools.go:670-672`

**Issue:** `for _, reason := range field.CheckAll(mine, daten) { return "", reason, nil }`
returns whichever key Go's randomised map iteration reaches first, so an
assistant correcting a page with two bad values gets a different complaint on
each attempt. Pre-existing, but this phase widened `CheckAll`'s output (max
values, bounds, byte budget) and added `feldeigenschaften` so the assistant can
act on it.

**Fix:** collect and sort the reasons, or report the first by definition order.

---

_Reviewed: 2026-09-05_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
