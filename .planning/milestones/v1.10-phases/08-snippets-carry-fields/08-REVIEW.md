---
phase: 08-snippets-carry-fields
reviewed: 2026-09-06T13:15:00Z
depth: standard
files_reviewed: 31
files_reviewed_list:
  - cmd/holzcloud/templates/admin/field_list.html
  - cmd/holzcloud/templates/admin/snippet_list.html
  - internal/admin/field.go
  - internal/admin/snippet.go
  - internal/admin/snippet_fields_test.go
  - internal/bundle/bundle_test.go
  - internal/bundle/export.go
  - internal/bundle/format.go
  - internal/bundle/import.go
  - internal/db/migrations/00047_snippet_fields.sql
  - internal/db/migrations_down_test.go
  - internal/field/field.go
  - internal/field/store.go
  - internal/field/store_test.go
  - internal/public/access.go
  - internal/public/archive.go
  - internal/public/bausteinfelder_test.go
  - internal/public/cart.go
  - internal/public/checkout.go
  - internal/public/handler.go
  - internal/public/pagedata.go
  - internal/public/pluginhost.go
  - internal/public/search.go
  - internal/public/shop.go
  - internal/public/tag.go
  - internal/public/typearchive.go
  - internal/snippet/store.go
  - internal/template/loader.go
  - internal/template/sample.go
  - internal/template/sample_test.go
  - internal/tmplspec/TEMPLATE-SPEC.md
findings:
  critical: 1
  warning: 8
  info: 4
  total: 13
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-09-06T13:15:00Z
**Depth:** standard
**Files Reviewed:** 31
**Status:** issues_found

## Summary

The fourth namespace itself is the strongest part of the phase and I could not
break it. All **seven** `SELECT` column lists in `internal/field/store.go` are
byte-for-byte identical and agree with `scanDef`'s positional scan (verified
mechanically, not by eye); `List` carries `AND block_type_id IS NULL AND
snippet_id IS NULL`; `Sub` deliberately carries no snippet clause and is right
to; `Update` pins `SnippetID` from the stored row and a test proves a form
cannot push a field across namespaces. D-05's per-carrier `MaxFields` is
correctly scoped in all four arms and `TestBausteinNamensraumFeldvorrat` proves
the part that distinguishes the decision from merely raising the number. The
`ON DELETE CASCADE` fires (probed). `fillSnippets` really is the only writer of
the three snippet members — 14 call sites, and `feed.go` was correctly left
alone. `go vet` is clean, the i18n catalogues report `0 offen, 0 verwaist`, and
the whole suite is green.

Three things are not right.

The first is structural and cannot be edited away: **migration `00047`'s new
partial unique index is missing `AND parent_id IS NULL`**, so a snippet's group
sub-fields share the snippet's top-level key namespace. Two groups on the same
snippet cannot both carry a sub-field `tag`; a top-level field cannot share a
key with a sub-field. Both are legal on a page. I proved this against the
migrated schema. `00038`'s block-kind index has the same shape and gets away
with it only because a block kind cannot carry a group — a snippet can, which
is exactly what this phase decided (`admin/field.go:316-322`), so the template
was copied one clause short. The remedy is `00048`.

The second is the bundle path, which the brief flagged and which repaid the
attention. `cleanSnippetValues` skips `field.Clean` **and** `field.CheckAll`
whenever the snippet has no definitions — so a manifest that carries `values`
and omits `fields` writes its payload straight into `snippets.fields`
unvalidated. I imported a hand-written archive and 200 KB landed in the column
with zero warnings, on the very path whose doc comment claims that leaving
`CheckAll` out "would re-open on the snippet exactly the hole 07-04 closed on
the page". It is the same guard `importFieldValues` has carried since Phase 7,
so this is an inherited pattern rather than a new invention — but it is now
reachable through a second carrier and the comment above it asserts the
opposite. Beside it, `importSnippetFields` dereferences `created.ID` where the
admin handler carefully guards the same value, and image/reference/label field
values do not survive the round trip at all while the admin screen actively
offers those kinds.

The third is smaller but user-visible: the field screen tells the operator to
write `{{.Site.Bausteinfelder.footer-kontakt.telefon}}`, which is a hard Go
template parse error for any snippet key containing a hyphen — and hyphens are
explicitly permitted by `validKey`, and `footer-kontakt` is the project's own
fixture key.

Structural findings were not supplied for this review; the sections below are
narrative only.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: `00047`'s snippet index has no `parent_id IS NULL`, so a snippet's groups share its key namespace

**File:** `internal/db/migrations/00047_snippet_fields.sql:48-50`

**Issue:**

```sql
CREATE UNIQUE INDEX idx_page_field_defs_kennung_textbaustein
    ON page_field_defs(snippet_id, kennung)
    WHERE snippet_id IS NOT NULL;
```

Since the 08-05 fix, a group's sub-fields on a snippet carry **both**
`parent_id` and `snippet_id` (`internal/admin/field.go:194-202`,
`internal/bundle/import.go:833-846`). The predicate above therefore covers those
sub-field rows as well, which folds three namespaces that `00029` deliberately
kept apart into one.

Measured against the migrated schema:

```
PAGE:    two groups may both carry sub-field 'tag'                — accepted
PAGE:    top-level field 'tag' beside group sub-field 'tag'       — accepted
SNIPPET: second group could NOT reuse sub key 'tag'               — ErrDuplicateKey
SNIPPET: top-level field could NOT reuse group sub key 'tag'      — ErrDuplicateKey
```

Consequences:

1. An operator defining `Öffnungszeiten{tag, von}` and `Ferien{tag, von}` on one
   snippet is refused on the second group with *"Ein Feld mit dieser Kennung
   gibt es schon"* — a sentence that is not true of any other carrier, and that
   directly contradicts the copy this phase added at `field_list.html:20`:
   *"Ein Textbaustein bekommt jede Art von Feld, die auch eine Seite bekommt —
   Gruppen eingeschlossen."*
2. `00047`'s own comment (`:34-37`) claims *"Vier Namensräume, und keiner von
   ihnen trifft einen anderen"*. The fourth namespace collides with its own
   groups' namespaces.
3. On the archive path the collision is a **silent partial loss**:
   `importSnippetFields` reports the refused sub-field as one warning line among
   others and imports the rest, so a bundle round-trips into a snippet whose
   second group is missing a column.

`idx_page_field_defs_kennung_gruppe` from `00029`
(`ON page_field_defs(parent_id, kennung) WHERE parent_id IS NOT NULL`) already
guarantees uniqueness within each group, so the extra coverage buys nothing.

**Fix:** a released migration is never edited — ship `00048`:

```sql
-- +goose Up
-- 00047 zog den Teilindex ohne "parent_id IS NULL". Ein Textbaustein kann,
-- anders als eine Bausteinart, eine Gruppe tragen, und ihre Unterfelder tragen
-- seither snippet_id — sie fielen damit in denselben Namensraum wie die Felder
-- der obersten Ebene. Auf einer Seite ist das seit 00029 getrennt; hier war es
-- eine Klausel zu wenig.
DROP INDEX IF EXISTS idx_page_field_defs_kennung_textbaustein;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_textbaustein
    ON page_field_defs(snippet_id, kennung)
    WHERE snippet_id IS NOT NULL AND parent_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_page_field_defs_kennung_textbaustein;
CREATE UNIQUE INDEX idx_page_field_defs_kennung_textbaustein
    ON page_field_defs(snippet_id, kennung)
    WHERE snippet_id IS NOT NULL;
```

Add the regression to `internal/field/store_test.go` in the shape of the probe
above: two groups on one snippet, both carrying `tag`, plus a top-level `tag`.

## Warnings

### WR-01: `cleanSnippetValues` skips `Clean` and `CheckAll` when the snippet has no definitions

**File:** `internal/bundle/import.go:929-951` (the `if len(defs) > 0 {` guard)

**Issue:** `importSnippetFields` returns early only when `sn.Values` **and**
`sn.ValueGroups` are both empty. A manifest that carries values and omits
`fields` therefore reaches `cleanSnippetValues` with `defs == nil`, the guard
skips both `field.Clean` and `field.CheckAll`, and `field.Encode(data)` stores
the manifest's payload verbatim.

Probed with a hand-written archive (`values` present, `fields` absent):

```
Warnungen: []
gespeicherte fields-Spalte: 200091 Bytes
```

200 KB of keys no definition carries, stored without one warning line. The
function's own doc comment says the opposite:

> `field.CheckAll` ist, wo das Byte-Budget und die Regeln je Art leben; es hier
> wegzulassen risse auf dem Textbaustein genau das Loch wieder auf, das 07-04
> auf der Seite geschlossen hat.

The identical guard sits in `importFieldValues` (`import.go:502`), so this is an
inherited pattern and not a new invention — but it is now reachable through a
second carrier, and the mitigation the comment names does not fire on the one
manifest shape that is entirely attacker-authored.

**Fix:** run both unconditionally. With no definitions, `Clean` correctly drops
everything, which is the right outcome — a value under no definition is not
renderable by anything.

```go
-	if len(defs) > 0 {
-		data = field.Clean(defs, data)
-		for key := range field.CheckAll(defs, data) {
-			...
-		}
-		sort.Strings(dropped)
-	}
+	data = field.Clean(defs, data)
+	for key := range field.CheckAll(defs, data) {
+		...
+	}
+	sort.Strings(dropped)
```

Apply the same change at `import.go:502` so the two paths stay one decision.

### WR-02: `importSnippetFields` dereferences a snippet that may be nil

**File:** `internal/bundle/import.go:793-800`

**Issue:**

```go
created, err := s.Snippets.Create(ctx, websiteID, sn.Key, sn.Name, markdown, html)
if err != nil { ...; continue }
report.Snippets++
importSnippetFields(ctx, s, websiteID, created.ID, sn, report)
```

`snippet.Store.Create` ends in `return s.Get(ctx, id)`, and `Get` returns
`(nil, nil)` when the row is not found (`internal/snippet/store.go:92-94`) —
note also that `Get` reads through `s.DB.Read`, a different pool from the one
that wrote. Before this phase the return value was discarded (`if _, err :=
…`); the new line makes `(nil, nil)` a nil-pointer panic that takes down the
import request. `internal/admin/snippet.go:307-310` guards the very same value
(`if created != nil { id = created.ID }`), so the two call sites now disagree
about whether the value can be nil.

**Fix:**

```go
if created == nil {
	report.Warnings = append(report.Warnings,
		fmt.Sprintf("Textbaustein %q konnte nicht zurückgelesen werden.", sn.Key))
	continue
}
report.Snippets++
importSnippetFields(ctx, s, websiteID, created.ID, sn, report)
```

### WR-03: a snippet's picture, reference and label values are silently lost on the archive round trip

**File:** `internal/bundle/export.go:325-341`, `internal/bundle/format.go:278-300`

**Issue:** `exportSnippets` writes `field.Decode(sn.Fields)` out as-is. A page's
values go through `exportFieldValues`/`translateIn`, which turn a media id into
a file name and a page id into an address; a snippet's do not. On import the raw
id is stored, and `fieldImages`/`fieldRefs` reject it because it belongs to
another website (`internal/public/pagedata.go:190`, `:157`) — so the field
arrives and the picture does not, with **no warning in the report**.

This is documented as "a recorded limitation" in `format.go:293-300`, but two
things make it a finding rather than a disposition:

1. The admin screen deliberately offers exactly those kinds — `kinds =
   field.Kinds` at `internal/admin/field.go:316-322`, with the comment "a
   reference and a label field can keep their promise here, so they are
   offered" — and `field_list.html:20` repeats the promise to the operator. The
   promise is true at render time and false at export time.
2. It is not in `.planning/phases/08-snippets-carry-fields/deferred-items.md`
   and not in `.planning/WINDOWS.md`, so nothing outside a code comment tracks
   it. Phase 7 shipped this exact class of bug on this exact path.

**Fix (minimum, this phase):** make the loss loud rather than silent, and record
it. In `exportSnippets`, after loading `defs`:

```go
for _, d := range defs {
	switch d.Kind {
	case field.KindImage, field.KindRef, field.KindTerm:
		if _, ok := data.Values[d.Key]; ok {
			// Nicht übersetzbar auf diesem Weg: der Wert ist eine Nummer
			// dieser Maschine. Er reist mit, damit nichts still verschwindet,
			// und der Bericht sagt, dass er auf der anderen Seite ins Leere
			// zeigt.
			m.Notes = append(m.Notes, fmt.Sprintf(
				"Textbaustein %q, Feld %q: der Wert zeigt auf eine Nummer dieser "+
					"Installation und muss nach dem Import neu gewählt werden.", sn.Key, d.Key))
		}
	}
}
```

and add the entry to `deferred-items.md`. **Fix (proper):** hoist
`translateOut`/`translateIn` out of the page path and reuse them here — that is
a phase of its own and should be a roadmap item, not a silent comment.

### WR-04: three themed public routes never fill the snippet surface

**File:** `internal/public/handler.go:391`, `internal/public/access.go:212`,
`internal/public/pagedata.go:413`

**Issue:** `fillSnippets`' doc comment states it is *"der einzige Ort im Baum"*
and that a gate proves no assignment survives outside it. That gate cannot see
the opposite failure — a route that calls neither. Three do:

- `serve404` (`handler.go:391`) — `h.loader.Render404(…, h.siteData(r, website))`
- `serveShareError` (`access.go:212`) — `site := h.siteData(r, website)`, then
  `RenderPage(…, "404.html", data)`
- `HandleMaintenance` (`pagedata.go:413`) — `RenderMaintenance(…, site, …)`

All three render the theme's real layout, so a shipped footer built on
`.Site.Snippets` and now on `.Site.Bausteinfelder` renders empty there. I
verified this degrades rather than panics — `index` over a nil map in
`html/template` yields the empty string, and `range` over one loops zero times —
but the 404 page and the maintenance page are exactly the pages where the
contact block matters most, and `TEMPLATE-SPEC.md:212` documents both members
unconditionally.

The `.Site.Snippets` half is pre-existing; the two new members inherit the gap
on the day they are introduced.

**Fix:** each of the three already has a `*domain.Website`, so:

```go
site := h.siteData(r, website)
h.fillSnippets(r, &site, website.ID, h.loadSnippets(r, website.ID))
```

`HandleMaintenance` is a deliberate judgement call — a deactivated website
arguably should not run a snippet query — but if it is left alone, say so in the
comment rather than leaving it looking like an omission.

### WR-05: the field screen tells the operator to write a template expression that will not parse

**File:** `cmd/holzcloud/templates/admin/field_list.html:17`

**Issue:**

```html
<code>&#123;&#123;.Site.Bausteinfelder.{{.Snippet.Key}}.kennung&#125;&#125;</code>
```

A snippet key may contain `-` (`internal/admin/snippet.go:104` permits it
explicitly) and the project's own fixture key is `footer-kontakt`
(`internal/template/sample.go:74`). Verified:

```
{{.Site.Bausteinfelder.footer-kontakt.telefon}}  -> template: t:1: bad character U+002D '-'
{{.Site.Bausteinfelder.kontakt.telefon}}         -> <nil>
```

A theme author following the on-screen advice gets their upload rejected by
`template.Check` with a raw Go parser error naming a character, not a cause.
`TEMPLATE-SPEC.md:735-746` gets this right and uses `index` throughout — the
admin screen is the one place that contradicts the specification.

**Fix:** use the same form the specification uses, so the two agree:

```html
<code>&#123;&#123;index .Site.Bausteinfelder "{{.Snippet.Key}}" "kennung"&#125;&#125;</code>
```

### WR-06: `snippet.Store.SetFields` does not touch `updated_at`, which is the site-wide cache validator

**File:** `internal/snippet/store.go:140-146`

**Issue:** `Rendered.LatestUpdate` is documented at `store.go:172-175` as the
reason a page's `Last-Modified` accounts for snippets — *"or a conditional
request answers 304 with the old opening hours baked in"* — and
`contentModTime` (`internal/public/pagedata.go:327-332`) uses it for **every**
page of the site. This phase makes a snippet's field values part of what a page
renders (`fillSnippets`), and the only function that writes them does not bump
`updated_at`.

It works today only because `handleSnippetSave` always calls `Update` (which
bumps it) before `SetFields`. That is an ordering coupling between two
functions in different packages with nothing enforcing it, on the code path
whose own comment explains why the field must not go stale. The import path
gets away with it because `Create` sets the timestamp.

`page.Store.SetFields` has the same shape, but a page has no site-wide cache
validator hanging off it, so the precedent does not carry.

**Fix:**

```go
func (s *Store) SetFields(ctx context.Context, id int64, raw string) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE snippets SET fields = $1,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = $2`, raw, id)
	...
}
```

### WR-07: `field.Store.Create` never checks that `SnippetID` belongs to `WebsiteID`, and a comment claims it does

**File:** `internal/field/store.go:344-449`, `internal/bundle/import.go:812-816`

**Issue:** `Create` takes `d.SnippetID` on trust. `internal/admin/field.go:207`
guards it with `snippetOf`, and the bundle path supplies a freshly created id —
both correct today. But `importSnippetFields`' doc comment asserts a guarantee
the store does not provide:

> The manifest never supplies an id, so a bundle cannot name another website's
> snippet, and **Create's own website scoping is the second layer under that.**

There is no such second layer. `REFERENCES snippets(id)` proves the snippet
exists; nothing proves it belongs to `d.WebsiteID`, and
`idx_page_field_defs_kennung_textbaustein` is keyed on `snippet_id` alone, so
the database will happily store a cross-website definition. The next caller that
believes the comment writes the hole. `BlockTypeID` has carried the same gap
since `00038` — which is an argument for closing both, not for leaving a false
comment in place.

**Fix:** either delete the sentence, or make it true in `Create` (which is
cheap, runs once per definition, and covers every present and future caller):

```go
if d.SnippetID > 0 {
	var n int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM snippets WHERE id = $1 AND website_id = $2`,
		d.SnippetID, d.WebsiteID).Scan(&n); err != nil || n == 0 {
		return nil, ErrNoGroup // oder ein eigenes ErrNoSnippet
	}
}
```

### WR-08: `scanDef`'s comment still counts five column lists; there are seven

**File:** `internal/field/store.go:304-307`

**Issue:**

```go
// Die Reihenfolge hier ist die der fünf SELECT-Spaltenlisten, Zeichen für
// Zeichen. Die fünf sind Abschriften voneinander und müssen es bleiben:
// eine Liste, die von den anderen abweicht, lädt ein Feld still mit einem
// Nullwert, und nichts schlägt fehl.
```

This phase added `OfSnippet` and `OfSnippets`, so `List`, `Sub`, `OfBlockType`,
`OfBlockTypes`, `OfSnippet`, `OfSnippets` and `Get` are **seven** copies. All
seven are currently byte-identical and agree with the scan order (I extracted
and compared them mechanically), so nothing is broken — but this comment is the
only thing in the file that tells the next author how many places to change, and
it is on the hazard the phase named as its most dangerous. `store_test.go:100`
carries the same stale arithmetic ("fünf SELECTs, das INSERT und das UPDATE").

**Fix:** say seven in both places, and say what makes it seven, so the number is
re-derivable:

```go
// Die Reihenfolge hier ist die der sieben SELECT-Spaltenlisten — List, Sub,
// OfBlockType, OfBlockTypes, OfSnippet, OfSnippets und Get —, Zeichen für
// Zeichen. Die sieben sind Abschriften voneinander und müssen es bleiben: …
```

## Info

### IN-01: `deferred-items.md` says the orphaned sub-fields have no read path; `Sub` and `Get` return them

**File:** `.planning/phases/08-snippets-carry-fields/deferred-items.md` (Plan 08-05 section)

**Issue:** The note says of sub-fields written with `snippet_id NULL` under a
snippet group: *"Sie schaden nichts — kein Leseweg gibt sie heraus."*
`Store.Sub` (`internal/field/store.go:105-127`) selects by `website_id` and
`parent_id` only, with no snippet clause, so those rows **are** returned on the
`?gruppe=<id>` screen and by `Get`. Only `OfSnippet`/`OfSnippets` miss them,
which is what makes them invisible on the snippet's own form.

No shipped installation can be affected — 08-03 and 08-05 land in the same
phase — so the disposition ("no repair migration") stands. The reasoning behind
it does not.

**Fix:** correct the sentence to "kein Leseweg des Textbausteins gibt sie
heraus; der Gruppenbildschirm zeigt sie weiterhin".

### IN-02: the group screen loses the snippet it was opened from

**File:** `cmd/holzcloud/templates/admin/field_list.html:81`, `internal/admin/field.go:301-353`

**Issue:** The "Felder der Gruppe" link is
`…/felder?gruppe={{.ID}}` with no `&textbaustein=`. On that screen `.Snippet` is
nil, so the back link reads "← Alle Felder" and goes to the page-field list —
the operator leaves the snippet's context with no way back to it except the
snippet overview. Creating a sub-field still works (the carrier is inherited
from the stored parent), so this is navigation only.

**Fix:** carry the carrier in the link and let `HandleFieldList` keep it:

```html
{{if .IsGroup}}<a class="btn btn--sm" href="/admin/websites/{{$d.WebsiteID}}/felder?gruppe={{.ID}}{{if $d.Snippet}}&amp;textbaustein={{$d.Snippet.ID}}{{end}}">…</a>{{end}}
```

and in `HandleFieldList`, when both `gruppe` and `textbaustein` are present,
keep `snip` for the heading and the back link while `group` still selects the
list.

### IN-03: `?textbaustein=X&aendern=<id>` opens any of the website's fields in the snippet form

**File:** `internal/admin/field.go:141-146`

**Issue:** `data.Edit` is filled from `h.fields.Get(websiteID, id)` with no check
that the field belongs to the mode the screen is in. `?textbaustein=X&aendern=`
a page field's id renders that page field inside the snippet screen. The write
is safe — `Update` pins the carrier from the stored row — so the effect is a
confusing edit, not a namespace escape, and the same looseness already exists
for `?gruppe`.

**Fix:** ignore an `aendern` whose carrier does not match the mode:

```go
if def.SnippetID != snippetIDOf(snip) || def.BlockTypeID != blockTypeIDOf(blockType) {
	def = nil
}
```

### IN-04: per-carrier `MaxFields` removes the per-website ceiling on `page_field_defs`

**File:** `internal/field/field.go:210-220`, `internal/field/store.go:377-412`

**Issue:** D-05 is correctly implemented and correctly reasoned — the budget
exists so one *form* stays usable. Worth recording that it also removes the
table's only bound: nothing limits how many snippets or block kinds a website
has, so `page_field_defs` is now bounded by `60 × (1 + block kinds + snippets)`
rather than by 60. `fillSnippets` reads every snippet definition of the website
on every public page render (`OfSnippets`). Behind admin authentication and out
of v1 performance scope, but it is the invariant that quietly went away.

**Fix:** none required. If it ever binds, bound the number of snippets rather
than re-sharing the field budget.

---

_Reviewed: 2026-09-06T13:15:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
