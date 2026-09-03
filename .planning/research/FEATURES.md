# Feature Research — v1.6 "Inhaltsmodell und Zugang"

**Domain:** Self-hosted CMS content model (field kinds, globals, tabular import)
**Researched:** 2026-09-03
**Confidence:** HIGH for the ecosystem survey (primary vendor docs for Statamic, Craft, ACF, Kirby, Directus, Feed Me, WP All Import, Statamic Importer). MEDIUM for the recommendations, which combine those docs with a read of this repository's own `internal/field`, `internal/term`, `internal/snippet`, `internal/admin/wordpress.go` and migrations 00028/00029/00038/00045.
**Scope note:** SSO / Authentik is deliberately absent — a separate researcher covers it.

> **This file supersedes the v1.0 FEATURES.md.** It is not a greenfield feature landscape; it is the
> expected-behaviour survey for the five content-model items scoped into v1.6.

---

## 0. Reading this file

Five features are covered. Each gets its own section with the same three buckets:

- **TABLE STAKES** — a user who has used any other CMS will call it broken if it is missing.
- **DIFFERENTIATOR** — nice, sets it apart, defensible to defer.
- **ANTI-FEATURE** — commonly built, and here either actively bad, inconsistent with a rule this
  project already enforces, or **impossible without JavaScript beyond htmx** (flagged `⛔ JS`).

Complexity is `S` (hours), `M` (a day), `L` (two days or more), sized against this codebase, not
against a blank page.

Section 6 answers the framing question on multi-value encoding. Section 7 is the dependency graph.
Section 8 is the recommended phase order.

**One global cost every field-kind item pays** — a planner must budget it once per new kind and it
is easy to forget:

| Obligation | Where | Why |
|---|---|---|
| Register the kind | `internal/field/field.go` `Kinds`, and decide `SubKinds()` (inside a group?) and `BlockKinds()` (inside a block kind?) | Both are subtractive lists; a new kind is in unless excluded |
| Admin input | `cmd/holzcloud/templates/admin/field_input.html` | The `{{else if .Is "…"}}` chain |
| Validation | `field.CheckAll` | One function shared by the admin form, the bundle import and the MCP write path — three copies would drift |
| Theme value | `field.Resolve` + `field.Entry` | The one place a stored string becomes a typed value |
| Contract | `internal/tmplspec/TEMPLATE-SPEC.md` **and** `SampleData` **and** `MinimalData` in `internal/template/sample.go` | Tests tie the three together; a kind missing from a fixture fails the suite |
| Translation | `go run ./tools/i18n -write` then `-schweiz` | Five languages plus three Swiss variants; the run must say "0 offen, 0 verwaist" |
| Round trip | `internal/bundle` export/import, `internal/ai` write path | A kind that does not survive export is a kind that loses data |

Budget `S` per kind for the kind itself and roughly the same again for that table.

---

## 1. Choice as a button row, and a genuine multiple choice

### What the neighbours call it

| System | Single, as buttons | Single, as radios | Multiple | Stored value (multiple) |
|---|---|---|---|---|
| **Statamic 6** | `button_group` — "a multiple choice input where you only get one choice" | `radio` | `checkboxes`; `select` with `multiple` | YAML **array**. Template loops yielding `value`, `label`, `key` |
| **Craft 5** | — (Dropdown/Radio only) | `Radio Buttons` | `Checkboxes`, `Multi-select` | `MultiOptionsFieldData` — iterable of `{label, value}`, with `.contains('x')`, `|length`, and `.options` exposing *all* options each with `.selected` |
| **ACF (WordPress)** | `Button Group` (since 5.6.3) | `Radio Button` | `Checkbox`, `Select` multiple | PHP **array**; `Return Format` picks Value / Label / Both (Both → array of `{value,label}`) |
| **Kirby 5** | `toggles` | `radio` | `checkboxes`, `multiselect`, `tags` | **Comma-separated string** in the flat content file (`"a, b, c"`), configurable `separator`; templates call `->split()` |
| **Directus** | — | radio interface | checkboxes interface | column is `csv` (comma-joined text) or `json` — the *interface* is chosen separately from the *type* |

Two things fall out of that table and they are the two decisions of this feature:

1. **A button row is not a new data shape.** Statamic's `button_group`, ACF's `Button Group` and
   Kirby's `toggles` all store a plain string, identical to the dropdown. Every one of them is a
   *presentation* setting on an existing single-choice field. That is a strong signal to model it
   the same way here.
2. **Multiple is a new data shape, and the neighbours do not agree on it.** Statamic/Craft/ACF
   expose an array of objects; Kirby stores a delimited string and hands the template a `split()`.
   Kirby's answer is the one that fits a flat-file store — and `pages.fields` **is** a flat store.
   See §6.

### Expected empty / none state — the sharpest no-JS finding in this section

Two concrete browser facts drive two table-stakes requirements that nobody writes down because
in a JS CMS they never come up:

- **A radio group cannot be un-set by a user.** Once any radio in a group is checked, plain HTML
  offers no way back to "nothing chosen"; Statamic's `select` has `clearable`, ACF's `Button Group`
  has `Allow Null`, and both implement it in JS. Therefore: **an optional (`pflicht = 0`) button row
  must render an explicit empty option** ("— keine Angabe —") as the first button, or the author
  clicks once and can never undo it. This is not polish; it is the difference between a working
  field and a trap.
- **An all-unchecked checkbox group submits nothing at all.** The browser sends no key for the
  field, so the server cannot distinguish "the author cleared it" from "the field was not on this
  form" (a hidden section, a condition not met). Therefore: **a hidden sentinel input must precede
  the checkboxes** (`<input type="hidden" name="feld_sorten" value="">`), or `Effective`/`Hidden`
  logic will silently resurrect old values. This is a five-line fix that is a multi-hour bug if
  missed.

Empty state in the theme: every comparable system makes empty falsy — Craft `|length` / `empty`,
ACF `if( $colors )`, Statamic an empty array. This matches the rule already written in
`field.Resolve`: *"An empty value is the zero of its kind."* A nil slice is falsy in Go templates,
so `{{if .Page.Felder.sorten}}` works with no extra code.

### Expected max / min selection count

| System | Max | Min |
|---|---|---|
| Statamic | `max_items` on `select`/`checkboxes` | — |
| Kirby | `max` | `min` |
| Craft | — | — |
| ACF | — | — |

**Verdict: max is a differentiator, min is not needed at all.** `pflicht` already means "at least
one" for a multiple field, which is every real min. A `max` has precedent inside this codebase
(`term.MaxPerPage = 12`, `field.MaxRows = 40`) and costs one integer column plus one line in
`CheckAll` — cheap enough to include, not worth a phase.

### Ordering of a multi-value — a free property worth writing down

Checkboxes submit in DOM order, so the stored order is always the **order the options are defined**,
never the order the author clicked. That makes the revision diff stable (re-checking the same three
boxes produces no diff) and the theme output stable. Say so in TEMPLATE-SPEC and do not sort.

### Buckets

| # | Item | Bucket | Complexity | Notes |
|---|---|---|---|---|
| 1.1 | Button row as a **display setting on `KindChoice`**, not a new kind | TABLE STAKES | S | Matches Statamic/ACF/Kirby exactly. A new `art` value would fork validation, export and TEMPLATE-SPEC for a CSS difference. Store `darstellung ∈ {liste, knopfreihe}` on `page_field_defs` |
| 1.2 | Explicit "keine Angabe" option when a button row is optional | TABLE STAKES | S | Otherwise unclearable — see above |
| 1.3 | Keyboard arrows move within the button row | TABLE STAKES | — | Free: a native radio group already does this. Do not intercept it |
| 1.4 | Segmented-control look via `:checked` + `:has()` | TABLE STAKES | S | Pure CSS; project already uses `:has()` and container queries |
| 1.5 | New kind `mehrfachauswahl` rendered as a checkbox group | TABLE STAKES | M | The data-shape decision is §6, not this row |
| 1.6 | Hidden sentinel so "cleared" is distinguishable from "absent" | TABLE STAKES | S | See above |
| 1.7 | Theme value is loopable **and** testable — `{{range}}` plus a `.Has "Bio"` method | TABLE STAKES | S | Craft `.contains()`, Statamic `contains()` modifier, ACF `in_array()`. Every system has this; a bare slice without a membership test forces `{{range}}{{if eq . "Bio"}}` in every theme |
| 1.8 | `{{.Page.Felder.sorten}}` printed bare yields something readable (comma-joined) | TABLE STAKES | S | A `String()` on the slice type. Otherwise Go prints `[a b c]` on the public site |
| 1.9 | `Entry.Text` for the admin field list joins with ", " | TABLE STAKES | S | The list view already prints `Text` |
| 1.10 | `max` selection count | DIFFERENTIATOR | S | Statamic + Kirby have it; Craft + ACF do not |
| 1.11 | Also expose the *unselected* options with a `.Selected` flag (Craft's `.options`) | DIFFERENTIATOR | S | Lets a theme render the whole vocabulary greyed out — a real pattern for filter UIs. Cheap because the defs are already in hand |
| 1.12 | Chips / inline appearance variants (Statamic's three `appearance` values) | DIFFERENTIATOR | S | Pure CSS, but three variants is two more support questions than one |
| 1.13 | Separate `value : Label` syntax in the options textarea | ANTI-FEATURE | — | ACF/Statamic have it; `SplitChoices` here reads one option per line and the label **is** the value. Splitting them means every stored value in the wild becomes ambiguous, and it destroys the escaping-free property that §6 depends on. If a label must change, `term`-style stable keys are the answer, not a colon |
| 1.14 | Searchable / typeahead multi-select (Statamic's Vue Select) | ANTI-FEATURE ⛔ JS | — | Needs a component. A `<select multiple>` is the no-JS fallback and is genuinely worse than checkboxes on touch |
| 1.15 | `taggable` / `push_tags` — invent a new option while filling the form | ANTI-FEATURE ⛔ JS | — | Statamic and ACF ("Allow Custom" / "Save Custom") both offer it; both need JS, and both make the option list mutate from the content screen, which means the vocabulary is no longer reviewable in one place. §2's term field is the honest version of this want |
| 1.16 | `cast_booleans` | ANTI-FEATURE | — | Statamic has it because YAML; there is no YAML here, and `KindBool` exists |
| 1.17 | Drag-to-reorder the selected values | ANTI-FEATURE ⛔ JS | — | And unnecessary: option order is already deterministic (see above) |

---

## 2. Terms (tags) as a field kind

### What is stored: id, slug, or name?

| System | Field | Stored | Survives rename? | On delete |
|---|---|---|---|---|
| **Statamic** | `terms`, *taxonomizing* mode (field handle = taxonomy handle) | the term's **slug** | Yes for the title; **no** if the slug is changed | resolves to nothing |
| **Statamic** | `terms`, *reference* mode | the **id**, formatted `taxonomy-handle/slug` | Yes | resolves to nothing |
| **Craft 5** | Categories / Tags field | **element id** in the `relations` table | Yes | relation disappears with the element |
| **ACF** | Taxonomy field | **term id** (`Return Value` = Term Object or Term ID) | Yes | dangling id; ACF returns `false`/null |
| **Kirby** | `tags` with `options: query` | the **plain string** written into the content file | **No** — the old name stays | nothing happens; the string persists |

The pattern: everything except Kirby stores a stable identifier and resolves it at render time, so
the printed name is always current. Kirby stores a copy and the copy goes stale. **Users expect the
Statamic/Craft/ACF behaviour**: rename the tag once, every page that carries it shows the new name.

### What the public template prints

All of them resolve to an object, not a string. Statamic: "term slugs will automatically be
converted to Term objects which means you will have all of the term's data available" — templates
reach `.title` and `.url`. Craft returns a Category/Tag element with `.title` and `.url`. ACF with
`Return Value = Term Object` gives a `WP_Term` plus `get_term_link()`.

**This is exactly the `KindRef` contract that already exists here.** `field.Ref` carries
`{Title, URL, Kind}` with the comment *"Title is the target's title as it is right now, not as it
was when somebody chose it"* and `RefLookup` is the place where "must belong to this website" and
"must be published" live. A term field is that pattern with `term.Term{ID, Slug, Name}` and
`Term.URL() = "/tag/" + slug` behind it. `docs/offene-punkte.md` already says so: *"Ein Tag. Das
Muster steht komplett beim Verweis; abzuschreiben ist es einmal."*

### The identifier decision, for this codebase specifically

**Recommendation: store the slug, not the id.** Three reasons, in order of weight:

1. `term.Store.Rename` is documented as *"changes the visible name of a label without moving its
   address"* and there is a test named `TestRenameKeepsTheAddress`. **In this codebase the slug
   already is the stable identity** — that is not true of Kirby, where the *name* is stored. Storing
   the slug therefore buys the Statamic/Craft rename behaviour with no indirection.
2. The bundle export and the WXR importer already carry terms **by name/slug**; `term.SetForPage`
   takes `names []string`. An id-valued field would need an id-remap table on bundle import — a
   problem the codebase does not currently have.
3. **A CSV importer cannot ask a human to type a term id.** A `sorte` column in a spreadsheet holds
   `Bio` or `bio`, not `47`. If the field stores an id, §5 needs a name→id resolution step on every
   row *and* a policy for names that do not resolve. If it stores a slug, the cell is the value.
   This is a hard dependency between §2 and §5 and it points one way.

The cost of choosing slug: if an operator ever gets the ability to *change* a term's slug, every
field value pointing at it breaks silently. Mitigation is one line: **do not offer slug editing**
(it is not offered today), and if it is ever added, rewrite the field values in the same
transaction. Note this in PITFALLS.

### Buckets

| # | Item | Bucket | Complexity | Notes |
|---|---|---|---|---|
| 2.1 | New kind `schlagwort` (`KindTerm`), chooser modelled on `refPages` in `internal/admin/page_fields.go` | TABLE STAKES | M | The whole point of the feature |
| 2.2 | Stores the slug; resolved through a `TermLookup` beside `Links.Page` | TABLE STAKES | S | See above |
| 2.3 | Rename follows: the theme prints the current name | TABLE STAKES | — | Free consequence of 2.2 |
| 2.4 | Delete degrades to nothing — the theme sees the zero value, the page still renders | TABLE STAKES | S | Identical to `KindRef`'s "when the target is deleted the theme sees nothing instead of a link into the void" |
| 2.5 | Theme gets `.Name` and `.URL` (the `/tag/<slug>` archive), not a bare string | TABLE STAKES | S | Every comparable system resolves to an object; a bare string cannot link to the archive |
| 2.6 | The chooser lists the website's existing vocabulary only, sorted, with counts | TABLE STAKES | S | `term.ListWithCounts` exists |
| 2.7 | **Not allowed inside a block kind** — add `KindTerm` to `BlockKinds()`' exclusion list next to `KindRef` | TABLE STAKES | S | Directly derivable from the existing comment: a block is frozen to HTML on save, so a rename could not follow. Offering it would be *"eine Zusage, die still bricht"*. Easy to miss; call it out in the plan |
| 2.8 | Empty state: no term chosen → theme sees nil, `{{with}}` is false | TABLE STAKES | — | Consistent with `KindRef` |
| 2.9 | Multi-value term field ("Sorten", several) | DIFFERENTIATOR | S *on top of §6* | Statamic/Craft/ACF all allow `max_items > 1`. **Only cheap if §6 has already landed** — it is the same encoding. If §6 slips, ship single-value and add multi later without a migration (§6's encoding makes a single value a one-line list) |
| 2.10 | The field's own vocabulary is filterable to a subset of tags | DIFFERENTIATOR | M | Statamic scopes by taxonomy; there is only one vocabulary per website here, so this would be a new concept. Defer |
| 2.11 | Create a new term from inside the page form | ANTI-FEATURE ⛔ JS *as an inline widget* | S *as a plain link* | Statamic (`create: true`) and ACF ("Create Terms") both do it inline with JS. The no-JS version — a link to *Schlagwörter* that returns to the form — is acceptable but costs the unsaved form. **Better: do not offer it in the field; the page's own tag box already creates terms freely, and a *field* whose point is choosing from a vocabulary should not also invent it** |
| 2.12 | Storing the term **id** | ANTI-FEATURE | — | For this codebase specifically: breaks CSV import (§5), breaks bundle portability, and buys nothing because the slug is already rename-stable here |
| 2.13 | Storing the term **name** (Kirby's model) | ANTI-FEATURE | — | Goes stale on rename — the one behaviour every user is surprised by |
| 2.14 | A term field that *also* taxonomizes the page (Statamic's dual mode) | ANTI-FEATURE | — | Two ways to put a tag on a page, with the archive counting one of them. Pick one: the page's tag box owns taxonomy, the field owns "which sort is this" |

---

## 3. The three small field kinds: `zeit`, `bereich`, `code`

### 3a. `zeit` — a time of day

**Expected behaviour.** Craft's Time field stores `H:i` (24-hour), returns a `DateTime` whose date
is implicitly today, or `null` when nothing was selected, and carries a loud warning: *"Changing the
system's timezone can cause previously-saved time field values to become inaccurate!"* That warning
is the design lesson — **a time of day is not an instant and must not carry a timezone.** Kirby's
`time` field has `step` (minute increment) and `display` (12h/24h). ACF's Time Picker has a display
format and a return format.

`<input type="time">` is native HTML with no JS at all, and the browser renders it 24h or 12h
according to the *user's* locale automatically — which is exactly right for a CMS with five UI
languages and Swiss variants. The one catch: Safari's `type="time"` has historically been a plain
text box; the value format the browser submits is nonetheless always `HH:MM`, so server-side
validation is the same either way.

| # | Item | Bucket | Complexity | Notes |
|---|---|---|---|---|
| 3.1 | `<input type="time">`, stored as a naked `HH:MM` string | TABLE STAKES | S | Native, no JS, locale-correct display for free |
| 3.2 | Resolved to a struct with `.Hour`/`.Minute` and a `String()` of `HH:MM` | TABLE STAKES | S | Mirror `field.Number{Value, Raw}` exactly — one precedent, not two |
| 3.3 | Empty is distinguishable from midnight | TABLE STAKES | S | `KindDate` already solved this with a `*time.Time`; use the same trick (pointer or an `Ok bool`) |
| 3.4 | **No timezone, ever** | TABLE STAKES | — | Craft's own warning. Do not reach for `time.Time` in a location |
| 3.5 | `step` / minute increment (e.g. 15) | DIFFERENTIATOR | S | Craft and Kirby both have it; `<input step="900">` is one attribute |
| 3.6 | min / max time | DIFFERENTIATOR | S | Craft has it; two attributes plus two lines in `CheckAll` |
| 3.7 | Seconds | ANTI-FEATURE | — | Nobody's opening hours need them; it doubles the parse surface |
| 3.8 | A combined "date + time" kind | ANTI-FEATURE | — | `datum` + `zeit` composed by the author covers it; a third kind would need its own storage format, its own validation and its own TEMPLATE-SPEC entry for no new capability |
| 3.9 | A JS time picker | ANTI-FEATURE ⛔ JS | — | The native control is better and free |

### 3b. `bereich` — a bounded number, as a slider

**Expected behaviour.** Statamic's `range` takes `min`, `max`, `step`, `append`/`prepend`, and
stores an int or a float depending on whether those are whole. The value is printed directly. ACF's
Range field is the same, plus `prepend`/`append` text.

**The one honest problem, and it is a real one.** `<input type="range">` is native and needs no JS —
but **its value is invisible**. Every implementation in the wild pairs it with a live readout, and
every one of those readouts is JavaScript (`oninput` into an `<output>`). CSS cannot read a form
control's numeric value; there is no `:has()` hook and no counter trick. So a slider without JS is a
control where the author drags a handle and cannot see what they picked until they save.

Three ways out, in order of preference:

1. **Few stops → reuse the button row from §1.** If `(max − min) / step + 1` is small (≤ 11 — a 1–5
   rating, a 0–10 scale, a T-shirt size), render it as a radio button row. The value is visible, it
   is keyboard-navigable, and **the renderer already exists once §1 ships.** This is the single
   best argument for ordering §1 before this item.
2. **Many stops → `<input type="range">` plus a paired `<input type="number">`**, both bounded by
   the same `min`/`max`/`step`. Without JS they do not track each other live, so the number input
   must be the authoritative one on submit and the range is an affordance for coarse dragging. This
   is honest and it degrades cleanly; it is also slightly odd-looking and should carry a hint.
3. **Many stops, minimal → `<input type="number" min max step>` only.** Loses the slider entirely
   but keeps every promise. Perfectly defensible; Statamic's own docs say of `range`, "Use the
   variable in your templates to display the value. That's pretty much it" — the *field* is a
   bounded number and the slider is decoration.

**Recommendation: 1 for ≤ 11 stops (automatic, not a setting), 2 above that.** Do not build 3
alone; a field named *Bereich* that is a spin box will be reported as a bug.

| # | Item | Bucket | Complexity | Notes |
|---|---|---|---|---|
| 3.10 | `min`, `max`, `step` on the def, enforced in `CheckAll` as well as in the browser | TABLE STAKES | S | Client attributes are a hint; the MCP write path and the CSV importer bypass the browser |
| 3.11 | The chosen value is **visible before saving** | TABLE STAKES | S–M | This is the requirement; the slider is one possible answer to it. See the three options above |
| 3.12 | Resolves to the existing `field.Number` type | TABLE STAKES | S | Do not invent a second numeric type. `Number{Value float64, Raw string}` already prints "8.50" correctly |
| 3.13 | Min/max shown as end labels on the control | TABLE STAKES | S | Pure CSS/markup; without them a slider is meaningless |
| 3.14 | Auto-switch to the button row at ≤ 11 stops | DIFFERENTIATOR | S | **Depends on §1.** Genuinely nicer than any JS slider for ratings |
| 3.15 | `prepend` / `append` unit text ("kg", "%") | DIFFERENTIATOR | S | Statamic and ACF both have it; it is a label, and it also improves the no-readout situation |
| 3.16 | `<datalist>` tick marks on the range input | DIFFERENTIATOR | S | Native, no JS; browser support is uneven and it is cosmetic |
| 3.17 | A live numeric readout beside the slider | ANTI-FEATURE ⛔ JS | — | The thing everyone will ask for. It is `oninput`, it is JavaScript, and `internal/tmplmgr/script.go` rejects exactly that pattern in uploaded templates — building it in the admin would be the project contradicting its own rule |
| 3.18 | An htmx round trip per drag to render the readout | ANTI-FEATURE | — | Technically within the rules; a request per pixel of drag on a small linux/amd64 box is grotesque. Named here so nobody proposes it as the clever way out |
| 3.19 | A two-handle range (from–to) | ANTI-FEATURE ⛔ JS | — | Not achievable with one native input. Two `bereich` fields, or a group |

### 3c. `code` — fixed-width, never Markdown, HTML shown verbatim

**Expected behaviour.** Statamic's `code` is CodeMirror with 25 languages, dark mode and vim
bindings; it stores a string, *or* an array `{code, mode}` when `mode_selectable` is on. Its docs
tell you to print it inside `<pre><code>` and reach for **Prism.js** for highlighting, and — the
detail that matters most — to use **unescaped output** (`{!! !!}`) in Blade. Craft has no built-in
code field; ACF has none either (`textarea` with a monospace class is the folk remedy). Kirby's
`writer`/`textarea` do not cover it.

**Is a language label expected?** Yes, but weakly — Statamic makes it opt-in (`mode_selectable`),
and its purpose is downstream: the language becomes a CSS class so a highlighter can find it. The
universal convention is `<pre><code class="language-go">`, which is what the HTML spec suggests and
what every highlighter reads.

**On escaping — this project starts from a better position than any system surveyed.** Statamic has
to tell Blade users `{!! !!}`; ACF users are told to `esc_html()`. Go's `html/template` escapes a
`string` printed in an HTML context **automatically and unconditionally**. So:

```gotemplate
<pre><code class="language-{{.Page.Felder.beispiel.Sprache}}">{{.Page.Felder.beispiel}}</code></pre>
```

renders `<script>` as `&lt;script&gt;` with no action from the template author. The requirement is
therefore not "escape it" — it is **"never cast it to `template.HTML`"**, which is the same rule
already written into CLAUDE.md for goldmark output. The one place that needs real care is a `code`
field **inside a block kind**, because a block is rendered to HTML on save (`internal/block`); that
render step must do the escaping itself rather than relying on the theme.

| # | Item | Bucket | Complexity | Notes |
|---|---|---|---|---|
| 3.20 | Monospace `<textarea>`, `white-space: pre`, tabs and leading whitespace preserved byte-for-byte | TABLE STAKES | S | This *is* the field. `spellcheck="false"`, `autocapitalize="off"`, `wrap="off"` |
| 3.21 | Never through goldmark, never through bluemonday, never cast to `template.HTML` | TABLE STAKES | S | `#` must stay a `#`, `<div>` must stay visible text |
| 3.22 | Theme guidance: print inside `<pre><code>` | TABLE STAKES | S | One paragraph in TEMPLATE-SPEC plus one line in the default theme |
| 3.23 | Escaping in the **block** render path, not only in the theme | TABLE STAKES | S | Blocks freeze to HTML on save; the escape must happen there. Easy to miss |
| 3.24 | `MaxValueBytes` (4000) applies and the limit is stated in the hint | TABLE STAKES | S | A code snippet is the field most likely to hit it |
| 3.25 | Optional language label from a **closed list**, emitted as `class="language-…"` | DIFFERENTIATOR | S | Statamic's `mode_selectable`. Closed list matters: a free-text language would be author-controlled text landing in a `class` attribute. **Do not store it as `{code, mode}` in one value** — store the language on the *definition* (all snippets in this field are Go) or as a sibling field; a two-part value would be the second thing in §6's slot and would fight the encoding |
| 3.26 | A "copy" button on the public page | ANTI-FEATURE ⛔ JS | — | Clipboard API |
| 3.27 | CodeMirror / line numbers / bracket matching / vim mode | ANTI-FEATURE ⛔ JS | — | Statamic's entire feature list here is a JS editor |
| 3.28 | Client-side highlighting (Prism.js, highlight.js) | ANTI-FEATURE ⛔ JS | — | And usually loaded from a CDN, which `internal/tmplmgr/external.go` rejects outright |
| 3.29 | Server-side syntax highlighting | ANTI-FEATURE | — | Would work and would be safe, but it is a large new dependency, it must emit HTML (re-opening the injection surface the escaping rule closes), and a theme can already style `<pre><code>` |
| 3.30 | An `html` kind whose content is *rendered* | ANTI-FEATURE | — | Already ruled out project-wide: *"Eine HTML-Vorlage je Bausteinart … wäre ein Weg, ein `<script>` durch die Vordertür auf eine Seite zu bringen."* `code` shows HTML; it never runs it. Say this explicitly in the field's hint, because the name invites the misunderstanding |

---

## 4. Text snippets that carry fields (Globals)

### What the neighbours do

| System | Name | Model | Editing surface | Template access |
|---|---|---|---|---|
| **Statamic** | Global Sets | Named sets, each *optionally* given a blueprint. Without one, YAML keys are plain text — **the blueprint is exactly what you add when you want a real fieldtype** | One CP screen per set, listed in the sidebar | Set handle is the scope: `{{ footer:copyright }}` / `$footer->copyright`. A default "Globals" set may be used unscoped |
| **Craft** | Globals | Global sets, each with a **field layout** — the same layout mechanism as entries | Settings → Globals to define; a "Globals" nav section with a sidebar of sets to edit | `{{ companyInfo.yearEstablished }}` |
| **WordPress/ACF** | Options Page | A field group with `location = options page` | A top-level admin menu page | `get_field('phone', 'option')` |
| **Kirby** | `site.txt` | The site blueprint's fields | The Panel's site view | `$site->phone()` |

Every one of them reuses the *same* field system as content. None of them has a second field
definition mechanism for globals. That is precisely the shape `docs/offene-punkte.md` already
proposes here: *"die Felder aus `page_field_defs` wiederverwenden, mit einer Spalte `snippet_id`
daneben — nicht eine dritte Feldtabelle."* Migration 00038 already did this once for
`block_type_id` (including the partial-unique-index swap), so `snippet_id` is a third instance of a
pattern that has been walked twice.

### The expected editing surface

Statamic and Craft both put globals on **their own screen, one per set, reached from a sidebar** —
not inline in a list. The important expectations:

- The fields appear **in the same form widget as everywhere else**, with the same sections,
  conditions and validation. A phone number with validation is the canonical example in the brief,
  and it should be validated by `field.CheckAll`, not by a second rule.
- A global's value is **not versioned separately** in Statamic; Craft globals do go through drafts
  in recent versions. Given `internal/page` already has revisions and snippets do not, matching the
  existing snippet behaviour (no revisions) is the consistent choice.
- Globals are **localized per site** in both. This project has per-website snippets already
  (`snippets.website_id`), which covers the multi-*website* case; the multi-*language* case
  (a website with locales) is a real open question — see 4.10.

### How a template reads one — and the constraint that decides it

Today `.Site.Snippets` is `map[string]template.HTML` and TEMPLATE-SPEC documents
`{{with index .Site.Snippets "footer-kontakt"}}<div>{{.}}</div>{{end}}`. Changing that map's value
type breaks every shipped and every uploaded template.

**Recommendation: leave `.Site.Snippets` untouched and add a parallel map of a struct**, mirroring
the page's own shape (`.Page.Felder` + `.Page.Feldliste`):

```gotemplate
{{with index .Site.Bausteine "kontakt"}}
  {{.HTML}}
  <a href="tel:{{.Felder.telefon}}">{{.Felder.telefon}}</a>
{{end}}
```

with `{HTML template.HTML; Felder map[string]any; Feldliste []field.Entry}`. Two properties this
buys: zero breakage for existing themes, and the `Felder`/`Feldliste` pair is a shape theme authors
(often an AI agent reading TEMPLATE-SPEC literally) have already met on the page.

**Keep the Markdown body.** A snippet today *is* a body; turning the body into "just another field"
would migrate every existing snippet and break every template that prints one. A snippet becomes
**body + optional fields**, exactly as a page is content + optional fields. That symmetry is the
whole argument and it is also zero migration for existing content.

### Buckets

| # | Item | Bucket | Complexity | Notes |
|---|---|---|---|---|
| 4.1 | Per-snippet field definitions reusing `page_field_defs` + a `snippet_id` column | TABLE STAKES | M | Third instance of the 00038 pattern. Partial unique index on `(snippet_id, kennung)` |
| 4.2 | Values as `field.Data` JSON in a `snippets.fields` column | TABLE STAKES | S | Symmetric with `pages.fields`; makes revisions/export/MCP uniform later |
| 4.3 | The existing Markdown body stays and stays default | TABLE STAKES | S | Otherwise every snippet and every theme migrates |
| 4.4 | Fields edited on the snippet's own screen, in the same form widget | TABLE STAKES | M | Reuse `field_input.html`; **this is the half of the two days that is screen work** |
| 4.5 | Validation through `field.CheckAll` | TABLE STAKES | S | "A phone number with validation" is the brief's own example |
| 4.6 | Every kind works, including image, reference and group | TABLE STAKES | M | The brief says "carries every field kind". Image needs the media picker on a non-page screen — check `media_picker.html` is not page-scoped |
| 4.7 | Theme access as `.Felder` / `.Feldliste` on a per-snippet struct, `.Site.Snippets` unchanged | TABLE STAKES | S | See above |
| 4.8 | Bundle export/import carries snippet fields | TABLE STAKES | S | `bundle.Report` already counts `Snippets`; the payload gains a `fields` key |
| 4.9 | The field list screen is reachable from the snippet, not from a global *Felder* screen | TABLE STAKES | S | Craft/Statamic both scope the layout to the set. A flat global list of every snippet's fields is unnavigable |
| 4.10 | Per-locale snippet values on a multilingual website | DIFFERENTIATOR | M | Both Statamic and Craft localize globals. Today snippets are per-website only. **Flag for phase-specific research** — it may be a bigger question than it looks, and it interacts with the star-shaped locale model |
| 4.11 | A snippet field usable inside page content (`{{baustein:kontakt.telefon}}`) | DIFFERENTIATOR | M | Statamic's scoping. Would need a Markdown-level shortcode, which is a new parsing surface. Defer |
| 4.12 | Revisions on snippets | DIFFERENTIATOR | M | Pages have them, snippets do not; adding both fields *and* revisions in one phase is two features |
| 4.13 | A second, snippet-only field definition table | ANTI-FEATURE | — | `offene-punkte.md` names it: *"nicht eine dritte Feldtabelle."* It would drift from `page_field_defs` in validation, ordering and conditions, exactly as migration 00029 predicted for groups |
| 4.14 | Reusing the snippet **key** as the field kennung namespace | ANTI-FEATURE | — | Keys are author-chosen and renameable; the def's `kennung` must be stable, as 00028 already argues for pages |
| 4.15 | "Global" snippets shared across websites | ANTI-FEATURE | — | Violates the project's website-isolation rule (all resources scoped to exactly one website, no sharing) |

---

## 5. CSV import of content

This is the item with the most user-visible design, so it gets the most detail. Everything below is
enough to draw the screens from.

### 5.1 How the four comparators do it

| | **WP All Import** | **Craft Feed Me** | **Statamic Importer** | **Directus** |
|---|---|---|---|---|
| Entry point | All Import → New Import | Feed Me section, one saved "feed" per import | Utilities → Importer | Collection sidebar → Import/Export |
| Steps | 5 numbered screens | 4 screens, saved as a reusable feed | 3 screens | 1 dialog |
| Mapping screen | **Drag & drop**: a floating panel of the file's columns; you drag each onto a target field | **A table**: rows are target fields; columns are `Field \| Feed Element (dropdown incl. "Don't import") \| Default Value`, plus a checkbox column marking the unique identifier | A table mapping blueprint fields to columns, with per-fieldtype extra settings ("Related Key", "Create when missing?", asset "Base URL") | **None** — headers must equal field keys or "the column will be skipped" |
| Preview | Step 2 shows the record count and **one record at a time** with next/prev navigation, plus row filters | No row preview; the Feed Element dropdown shows example values | Shows the file's columns | None |
| Duplicate key | "Unique Identifier", with **Auto-detect** | "Unique identifier" — a checkbox column, **may be more than one field** | "Unique Field" (required for entries) | Primary key only |
| Within-file duplicates | "only the first will be created and the others will be detected as duplicates" | not documented | not documented | not documented |
| Create vs update | "New Items" / "Existing Items" modes; on update you **explicitly choose which fields to update and which to ignore** ("If you choose to update all data without having data mapped, you risk erasing existing data") | Import strategy: add new / update existing / disable missing / **delete missing** / update search indexes; `Set Empty Values` decides whether a blank column clears an existing value | "update existing content and create new content as needed" | not documented |
| Transaction | Best-effort per row | Best-effort per row; optional **backup before each run** | Best-effort per row, queued for big files | **All-or-nothing**: errors accumulate to `MAX_IMPORT_ERRORS` and then "the import is cancelled" |
| Report | Summary of created / updated / skipped / deleted, plus History Logs | A run log | A summary | An error message |
| Multi-value cells | column per value, or a delimiter | delimiter | **pipe (`\|`)** — "values should be separated by a pipe" | comma (csv type) |

Two conclusions worth carrying forward:

- **Nobody except Directus skips the mapping screen, and Directus is the one everybody complains
  about.** The GitHub issue trail on Directus imports is almost entirely "my headers didn't match
  and the column was silently skipped". A mapping screen is table stakes.
- **Feed Me's orientation is the right one for this project.** WP All Import's drag & drop is
  ⛔ JS; Feed Me's table — one row per *target field*, a `<select>` of the file's headers — is
  plain HTML and is *also* the better information design, because the target field list is finite,
  ordered and known, while the column list is arbitrary.

### 5.2 The mapping screen, concretely

Four screens, each a plain form POST. No step needs JS.

**Screen 1 — Datei.** Upload; choose the target **website** (this importer writes into an existing
one — see 5.4); choose the **Inhaltsart** (Seite / Beitrag / a custom `art`); choose **Vorhandene
aktualisieren** or **Vorhandene überspringen**. Show the detected delimiter and row count back
before continuing.

**Screen 2 — Zuordnung.** A `<table>`, one row per target field:

| Feld | Spalte | Vorgabe | Schlüssel |
|---|---|---|---|
| Titel *(Pflicht)* | `<select>` of headers + "— nicht importieren —" | text input | (radio) |
| Adresse | `<select>` … | text input | ● |
| Inhalt (Markdown) | `<select>` … | — | |
| Status | `<select>` … | `entwurf` | |
| Schlagwörter | `<select>` … | — | |
| *Preis* (own field, `zahl`) | `<select>` … | text input | (radio) |
| *Sorten* (own field, multi) | `<select>` … | text input | |

- **Auto-mapping on arrival** — match each header against every field's `kennung` and
  `beschriftung`, case- and diacritic-insensitively, and preselect. This is the single largest UX
  win in the feature and it is roughly twenty lines. Every comparator does it or is criticised for
  not doing it.
- **"Vorgabe"** (Feed Me's Default Value) fills the field when the column is unmapped or the cell is
  empty. This is how "all 300 rows are drafts of kind *Produkt*" is expressed without a column.
- **"Schlüssel"** is a radio, so exactly one field is the match key. Feed Me allows several; one is
  enough here and one radio column is far easier to explain. Default it to **Adresse (slug)** — see
  5.4.
- **Above the table, a preview of one data row**, rendered as a definition list of `header → cell`,
  with `‹ Zeile 1 von 312 ›` next/prev links. WP All Import's step 2 is exactly this and it is the
  thing that makes people trust the delimiter detection. Next/prev is a `GET ?zeile=2` — no JS.
- The mapping must survive the round trip when the form comes back with an error. It is a plain
  POST, so this is just remembering to re-render selected states.

**Screen 3 — Probelauf (dry run).** Run the whole file through `field.CheckAll`, `page.ValidateSlug`
and the term/reference resolution, **writing nothing**, and show the report screen with `0 angelegt,
0 geändert` plus every problem, row-numbered. This is the differentiator, and it is cheap precisely
because `CheckAll` already exists and is already the one function shared by the admin form, the
bundle import and the MCP path.

**Screen 4 — Bericht.** Reuse `import_report.html` and `bundle.Report`, which already has
`Pages`, `Warnings` and the tone (*"Nichts davon hat den Import abgebrochen. Aufgelistet, damit
niemand zweihundert Seiten von Hand durchsehen muss, um es zu finden."*). Add `Created`, `Updated`,
`Skipped`. Cap the warning list the way `wordpress.go` caps its media URL list at 25 and then says
"… und N weitere".

### 5.3 Per-row result report — what it must contain

Row numbers, always, and the file's own row number (counting the header as row 1) so it matches what
the spreadsheet shows. One line per problem:

```
Zeile 47: „Preis" ist keine Zahl („CHF 8.50")
Zeile 51: die Adresse „Über uns!" ist nicht zulässig — importiert als „ueber-uns"
Zeile 88: Adresse „kontakt" kommt in dieser Datei zweimal vor — die zweite Zeile wurde übergangen
Zeile 92: das Schlagwort „Bio-Knospe" gibt es noch nicht — angelegt
Zeile 140: die Spalte „Bild" verweist auf „hof.jpg", das in der Mediathek dieser Website nicht liegt
```

Counters at the top: `angelegt / geändert / übersprungen`. Feed Me, WP All Import and Statamic
Importer all show that triple and users read it first.

### 5.4 Duplicates, create vs update, and the natural key

**Recommendation: the key is the slug (`Adresse`) by default.** `pages.slug` is already unique per
website per locale (migration 00045), so the lookup is an index hit and the uniqueness concept
already exists. Allowing an arbitrary own field as the key (Feed Me's model) means a JSON-blob scan
per row and a second uniqueness notion with no constraint behind it — list it as a differentiator.

- **Within the file:** the first row with a given key creates; every later row with the same key is
  **skipped and reported**, never silently merged. This is WP All Import's documented behaviour and
  it is the least surprising.
- **Against the website:** matched by key → **update** or **skip**, per the screen-1 choice. There
  is no third option.
- **On update, only mapped columns are written.** WP All Import's warning is worth heeding
  literally: *"If you choose to update all data without having data mapped, you risk erasing
  existing data."* An unmapped field keeps its value. An **empty cell in a mapped column** is the
  ambiguous case (Feed Me exposes it as `Set Empty Values`) — **recommendation: an empty cell in a
  mapped column clears the field**, because that is what a spreadsheet means, and say so on the
  screen. Do not add the setting; add the sentence.
- **Slug collisions on create** are resolved during the dry run by the same `-2`, `-3` numbering
  `importWordPressItem` already does, and each one is reported. This is what keeps write-time
  failures from appearing after validation passed.

### 5.5 Partially-bad file, and transactional vs best-effort

The field splits: Directus is all-or-nothing (and its issue tracker shows users hate the resulting
opacity); the three CMS importers are best-effort per row; this repository's own WXR importer is
best-effort per row with a warnings list.

**Recommendation: strict at the gate, best-effort at the till.**

1. **Validation is all-or-nothing and happens first.** Parse and validate every row before writing
   anything. If any row fails, screen 3 lists the failures and nothing has been written. The user
   fixes the spreadsheet and re-uploads. This is the property people actually want when they say
   "transactional" — *nothing half-imported* — and it costs one extra pass over a file that is
   already in memory.
2. **A "trotzdem importieren, fehlerhafte Zeilen überspringen" button on the dry-run screen** covers
   the case where three of 312 rows are junk and the user knows it. Best-effort, on request, with a
   record of what was skipped.
3. **Writing is one transaction per row, not one per file.** This is a hard constraint of the
   deployment, not a preference: the write pool is `SetMaxOpenConns(1)`, so a single transaction
   wrapping 500 page inserts holds the only writer for its whole duration and blocks every other
   request on the box, admin and public alike.
4. **An unexpected write error aborts and reports how far it got.** Do not attempt to roll back
   *n* already-committed rows; the report naming the last row is more useful, and the created pages
   are visible and deletable. Because step 1 already caught everything predictable, this path should
   be a database error, not a data error.

### 5.6 Cell formats — the rules a user has to be told exactly once

| Column kind | Format | Rationale |
|---|---|---|
| Any multi-value cell (multi-choice, multi-term, tags) | **pipe-separated**: `Bio\|Demeter\|Regional` | Statamic Importer's precedent, and — decisive — a comma or a newline inside a CSV cell requires quoting, which is exactly what Excel and Numbers users get wrong. One rule for the whole file |
| `janein` | `ja`/`nein`, `1`/`0`, `true`/`false`, empty = false | Every importer accepts a set; accept the set and say which |
| `zahl` / `bereich` | Accept both `8.50` and `8,50` | `field.Resolve` already does `ReplaceAll(",", ".")`. A Swiss spreadsheet writes a comma |
| `datum` | `YYYY-MM-DD` **and** `DD.MM.YYYY` | The second is what a Swiss/German locale exports. Accept both, store one |
| `zeit` | `HH:MM` | 24h only |
| `verweis` | the target page's slug | Same reasoning as §2: a human cannot type an id |
| `schlagwort` field | the term's slug or name | §2's slug storage makes this a lookup, not a remap |
| `bild` | the filename of a medium **already in this website's library** | See 5.11 |
| Encoding | UTF-8 required; strip a UTF-8 BOM silently; refuse invalid UTF-8 with "Speichere die Datei als CSV UTF-8" | One `utf8.Valid` call. Silent mojibake is worse than a refusal |
| Delimiter | auto-detect `,` `;` `\t` from the header line, and show what was detected on screen 1 | `;` is what a German/Swiss Excel writes by default. Getting this wrong is the #1 support ticket for every CSV importer |

### 5.7 Buckets

| # | Item | Bucket | Complexity | Notes |
|---|---|---|---|---|
| 5.1 | Header row required; explicit error if absent | TABLE STAKES | S | Statamic Importer requires it; without headers no mapping screen is possible |
| 5.2 | Delimiter auto-detection (`,` `;` tab) shown back to the user | TABLE STAKES | S | `;` is the Swiss Excel default |
| 5.3 | UTF-8 BOM stripped; invalid UTF-8 refused clearly | TABLE STAKES | S | |
| 5.4 | Column → field mapping table, one row per target field, `<select>` of headers, "— nicht importieren —" | TABLE STAKES | M | **This is the feature.** `offene-punkte.md`: *"Die Zuordnung Spalte → Feld ist die ganze Arbeit; alles danach ist `page.CreatePage`"* |
| 5.5 | Auto-mapping by `kennung`/`beschriftung`, case- and diacritic-insensitive | TABLE STAKES | S | ~20 lines, largest UX win per line in the milestone |
| 5.6 | One-row preview with `‹ ›` navigation above the mapping table | TABLE STAKES | S | Plain `GET ?zeile=n`. WP All Import step 2 |
| 5.7 | A match key (default: Adresse/slug) with create-or-update / create-or-skip | TABLE STAKES | M | Every comparator has one |
| 5.8 | Within-file duplicate keys: first wins, rest reported | TABLE STAKES | S | |
| 5.9 | On update, only mapped columns written | TABLE STAKES | S | WP All Import's own warning |
| 5.10 | Row-numbered problem report reusing `bundle.Report` + `import_report.html`, with created/updated/skipped counters | TABLE STAKES | S | The screen and the tone already exist |
| 5.11 | Slug collisions resolved with `-2`, `-3` and reported | TABLE STAKES | S | Copy `importWordPressItem` |
| 5.12 | Row and byte caps, mirroring `wxr.MaxItems` and the 10 MB `MaxBytesReader` | TABLE STAKES | S | A small server; a 200 MB CSV must be refused, not attempted |
| 5.13 | Draft by default unless a Status column says otherwise | TABLE STAKES | S | 300 pages appearing live on a public site is the failure mode people fear most about importers |
| 5.14 | **Probelauf / dry run** before anything is written | DIFFERENTIATOR | M | The best idea in this section. Cheap because `CheckAll` is already shared. Turns "is my file right?" from a destructive experiment into a click |
| 5.15 | "Vorgabe" (default value) per field for unmapped or empty cells | DIFFERENTIATOR | S | Feed Me's third column. Removes the need for constant-valued columns |
| 5.16 | Downloadable example CSV generated from the website's actual field definitions | DIFFERENTIATOR | S | Nobody surveyed does this and it eliminates most mapping questions. A `text/csv` response with one header row |
| 5.17 | "Trotzdem importieren, fehlerhafte Zeilen überspringen" on the dry-run screen | DIFFERENTIATOR | S | Best-effort, on request |
| 5.18 | Saved, re-runnable import profiles (Feed Me's whole model) | DIFFERENTIATOR | L | Real value for a recurring price list; a new table, a new list screen, and it invites scheduling. **Defer** |
| 5.19 | Arbitrary own field as the match key | DIFFERENTIATOR | M | Feed Me allows several; needs a JSON scan per row. Defer |
| 5.20 | Latin-1 / cp1252 transcoding | DIFFERENTIATOR | S | `golang.org/x/text/encoding/charmap`. Nice, but the refusal in 5.3 is honest and sufficient |
| 5.21 | Drag & drop mapping | ANTI-FEATURE ⛔ JS | — | WP All Import's signature UI. A `<select>` per row is plain HTML *and* better information design |
| 5.22 | **Delete missing** — remove pages absent from the file | ANTI-FEATURE | — | Feed Me has it and it is the single most destructive setting in that plugin. One mis-mapped key column deletes a website. Never offer it |
| 5.23 | Downloading images from URLs in the file | ANTI-FEATURE | — | WP All Import and Statamic Importer both do it. Here it is the runtime-third-party rule, and `wordpress.go` already refuses for exactly this reason and lists the URLs instead. Do the same |
| 5.24 | Creating a new website from a CSV | ANTI-FEATURE | — | That is what the bundle and WXR importers are for. A CSV is rows into a model that already exists |
| 5.25 | Importing repeatable groups or blocks from CSV | ANTI-FEATURE | — | Feed Me disables mapping for Matrix; Statamic Importer needs a custom transformer for Replicator. There is no honest flat encoding for a group with *n* rows of *m* fields. Say so on the mapping screen: unmappable fields are listed and greyed |
| 5.26 | Directus-style "headers must match field keys, no mapping screen" | ANTI-FEATURE | — | Silently skipping an unmatched column is the failure mode its issue tracker is full of |
| 5.27 | All-or-nothing write in one transaction | ANTI-FEATURE | — | Single-writer SQLite; a file-long transaction blocks every request on the box. §5.5's split gets the same guarantee without the lock |
| 5.28 | Background/queued import | ANTI-FEATURE *for v1.6* | — | Statamic Importer queues; there is a `internal/jobs` here so it is possible. But a synchronous import within the row cap keeps the report on the screen that started it, and progress reporting for a background job is either polling or ⛔ JS |
| 5.29 | Scheduled/recurring imports from a URL | ANTI-FEATURE | — | An outbound call at runtime. Same rule as always |
| 5.30 | Excel `.xlsx` upload | ANTI-FEATURE | — | A zip container plus an XML schema plus a dependency, for a format every spreadsheet can "Save as CSV" out of |

### 5.8 One departure from house rule, made deliberately

`internal/admin/wordpress.go` states the rule for the two existing importers: *"es entsteht immer
eine **neue** Website. Zusammenlegen bräuchte für jeden Zusammenstoss eine Antwort — gleiche
Adresse, anderer Text."* CSV import must break that rule, because rows into an existing content
model is the entire point.

The rule's stated reason is answered here rather than ignored: the collision question **does** get
an explicit answer (screen 1's update-or-skip plus the match key), and the dry run shows every
collision before anything is written. Worth writing into the phase plan as a decision, because a
future reader will otherwise see it as an inconsistency.

---

## 6. The framing question — how to encode a multiple-choice value

**This decision is load-bearing for §1, §2 and §5. Make it once, in the first phase that touches it.**

### The starting position

`pages.fields` holds one JSON blob per page: `{"werte": {"kennung": "wert"}, "gruppen": {"kennung":
[{...}, ...]}}`. `field.Values` is `map[string]string`; `field.Data.Rows` is
`map[string][]Values`. Migration 00028 states the reason plainly: *"Die Definition steht in einer
Tabelle, die Werte stehen als JSON an der Seite. Nicht andersherum: eine Spalte je Feld hiesse, dass
jedes neue Feld die Tabelle umbaut."* `field.Resolve` is the single place a stored string becomes a
typed value, and `MaxValueBytes = 4000` bounds one value.

That blob is also the unit of the revision snapshot, the bundle export payload and the MCP write
payload. Anything that changes its *shape* changes all four.

### The four candidates

| | **A — newline-separated in the same string** | **B — JSON array serialised into the same string slot** | **C — widen `Values` to `map[string]any`** | **D — a separate rows table** |
|---|---|---|---|---|
| Stored | `"Bio\nDemeter"` | `"[\"Bio\",\"Demeter\"]"` | `{"werte":{"sorten":["Bio","Demeter"]}}` | `page_field_multi(page_id, kennung, wert, position)` |
| Migration | **none** | none | none *(data)*, but a compile-wide change | one, plus a store |
| `Data`/`Decode` change | **none** | none | `Values`, `Rows`, `Effective`, `Hidden`, `CheckAll`, the flat-shape back-compat path — all of them | none, but the blob is no longer the whole value |
| Escaping rule needed | **none — see below** | JSON, doubly | none | none |
| Revision diff | one line per value, `textdiff` renders it well | `["Bio","Demeter"]` on one line | readable | **the diff no longer sees the value at all** |
| Readable in `sqlite3` / an export | yes | no (double-escaped) | yes | only via a join |
| **CSV importer must WRITE** | split the cell on `\|`, join with `\n`. **One `strings.Join`** | build and marshal a JSON array | assign a `[]string` | an insert per value, and a delete-then-insert on update |
| **Go template must READ** | `Resolve` returns a slice — identical for all of A/B/C | same | same | same, but `Resolve` now needs a DB call, or `Data` must be pre-joined everywhere |
| Blast radius | `Resolve`, `CheckAll`, `field_input.html` | same, plus every consumer must know which fields are JSON-in-string | ~every file that touches `field.Values` | a new store, list-query N+1, revisions, bundle, MCP |
| Inside a repeatable group? | works — a row is just `Values` | works | works | **has nowhere to key on** — group rows are `[]Values` with no persistent row id |

### Recommendation: **A — one value per line, in the existing string slot.**

Ranked reasons:

1. **The delimiter is already illegal inside a value, so no escaping rule is invented and none can
   be violated.** `SplitChoices` reads the option list one per line; a choice label therefore
   *cannot contain a newline*. For a term field the same holds: `page.Slugify` output has no
   newline. This is the property that makes A safe where a comma or a semicolon would not be —
   it is not "we hope nobody types a newline", it is "the alphabet of legal values excludes it".
   **This also bounds A's applicability: A is correct for closed vocabularies only.** If a
   multi-value *free-text* field is ever wanted, revisit — v1.6 has none.
2. **Zero migration and zero change to the JSON shape**, so revisions, `Decode`'s
   backward-compatible flat-shape path, bundle export/import, the WXR importer and the MCP write
   path all keep working untouched. Compare C, where `data.Values[k]` breaks at compile time in
   every file that touches it, for one field kind.
3. **The revision diff stays line-based and human-readable.** Adding one selected value produces a
   one-line diff in `internal/textdiff`, which is exactly what that package is good at. B and D
   both lose this.
4. **`Resolve` absorbs the whole cost.** It is already the documented single place where "the stored
   value is always a string … so the values are turned into the types they mean, once, on the way
   out." A multi-choice becomes ~15 lines there and every other code path stays string-typed.
5. `docs/offene-punkte.md` proposes exactly this (*"eine Zeile je Wert im selben String, wie
   `SplitChoices` die Möglichkeiten schon liest"*), so this is a confirmation with reasons rather
   than a new direction.

### What A costs, stated honestly

- **The CSV boundary needs a second delimiter.** A newline inside a CSV cell is legal RFC 4180 but
  requires quoting, is invisible in a spreadsheet cell, and is what Excel users get wrong. So the
  wire format is a **pipe** (Statamic Importer's choice) and the importer does
  `strings.Join(strings.Split(cell, "|"), "\n")` after trimming. One line, one sentence of
  documentation. This is the price and it is small — but it is a *user-visible* price, which is why
  it belongs in §5's cell-format table.
- **`MaxValueBytes` (4000) is now shared across all selected values.** At forty options of sixty
  characters that is not a real bound, but `CheckAll` should say so rather than truncate.
- **A single-value read of a multi-value field is a footgun.** `{{.Page.Felder.sorten}}` must not
  print `Bio\nDemeter`. Give the slice type a `String()` that joins with ", " (item 1.8).
- **Every consumer must consult the def to know whether to split.** True — but that is equally true
  of B and C, and the def is already in hand at every one of those call sites (`Resolve`,
  `CheckAll`, `Entry`).

### Consequences to write into the plan

- The **term field stores slugs, one per line**, using the same encoding — which is why §2's
  multi-value variant is nearly free once §1 lands, and why a single-value term field written today
  needs no migration to become multi-value later (one value is a one-line list).
- The **CSV importer's pipe rule applies to every multi-value column uniformly**, including the
  page's own tag column. `term.Parse` splits on commas because that is the *admin form's* contract;
  do not change it — the importer splits on `|` and passes the resulting names to
  `term.SetForPage`, which already creates terms from names.
- A **new `page_field_defs` column is still needed** for §1 (`darstellung`, and optionally
  `max_werte`) — that is an `ALTER TABLE ADD COLUMN`, which SQLite does cheaply, unlike the
  table rebuilds migrations 00029 and 00031 needed. Do not add a `CHECK` to it; 00028 already
  explains why (*"Kein CHECK: eine neue Art wäre sonst wieder ein Tabellenumbau"*).

---

## 7. Feature dependencies

```
[i18n writer + test-suite housekeeping]   (already scoped as the first phase)
    └──blocks──> everything with a new admin string, i.e. all five

[§1 Button row]  ── display setting on KindChoice, no data change
    └──enables──> [§3b `bereich` as a button row when steps <= 11]

[§6 MULTI-VALUE ENCODING]  ← decided inside §1, used by three phases
    ├──enables──> [§1 mehrfachauswahl]
    ├──enables──> [§2 multi-value term field]
    └──required-by──> [§5 CSV import writes it]

[§2 Term field]
    ├──copies──> [existing KindRef chooser: refPages, Links.Page, field.Ref]
    ├──requires──> [term slug is rename-stable]  (already true: term.Rename)
    ├──must-exclude-from──> [BlockKinds(), beside KindRef]
    └──required-by──> [§5 a term column in a CSV]

[§3 zeit / bereich / code]
    └──should-precede──> [§4 snippets with fields]
           (a snippet inherits every kind; landing kinds first means
            the snippet form is written once, not twice)

[§4 Snippets with fields]
    ├──copies──> [migration 00038's block_type_id pattern -> snippet_id]
    ├──reuses──> [field_input.html, field.CheckAll, media_picker.html]
    └──must-not-change──> [.Site.Snippets map[string]template.HTML]

[§5 CSV import]
    ├──requires──> [§6 encoding]      writes multi-value cells
    ├──requires──> [§2 term field]    a term column resolves to slugs
    ├──reuses──> [field.CheckAll, page.CreatePage, page.Slugify,
    │             bundle.Report, import_report.html, wordpress.go's -2 numbering]
    └──excludes──> [gruppe, baustein — no honest flat encoding]

[§5 CSV import] ──conflicts-with──> [wordpress.go's "always a new website" rule]
       resolved by an explicit update/skip choice + the dry run
```

### Dependency notes

- **§6 is decided inside §1 but paid for by §5.** If §1 ships the encoding and §5 slips, nothing
  breaks. If §5 ships first, it has to invent an encoding and §1 will inherit it. **§1 before §5,
  always.**
- **§2 before §5.** A CSV with a `Sorte` column is one of the two motivating examples in
  `offene-punkte.md`; if the term field does not exist yet, §5 either omits it or builds a
  throwaway path.
- **§3 before §4.** A snippet form that has to be revisited to add three kinds is the same screen
  work twice.
- **§1 before §3b.** The button-row renderer is what makes a short `bereich` usable without JS.
- **§2 must edit `BlockKinds()`.** This is the only cross-cutting change that is invisible from the
  feature's own description and is therefore the most likely to be missed.
- **No two of the five conflict.** They can be planned as five independent phases as long as the
  order §1 → §2 → §3 → §4 → §5 is respected.

---

## 8. Recommended phase order and prioritisation

| Order | Feature | User Value | Cost | Priority | Why here |
|---|---|---|---|---|---|
| 1 | §1 Button row + multiple choice | HIGH | M | P1 | Carries the encoding decision (§6) that two later phases depend on. The button row alone is an afternoon; the multiple is the real work |
| 2 | §2 Term field | HIGH | M | P1 | The pattern is fully written at `KindRef`; §5 wants it |
| 3 | §3 `zeit`, `bereich`, `code` | MEDIUM | S ×3 | P1 | Independent, small, and they must exist before §4 so the snippet form is built once. `bereich` is the only one with a real design question (the invisible-value problem) |
| 4 | §4 Snippets with fields | HIGH | L | P1 | The largest single screen job in the milestone; half of it is the form. Third instance of a migration pattern already walked twice |
| 5 | §5 CSV import | HIGH | L | P1 | Depends on §1's encoding and §2's field. The mapping screen and the dry run are the whole design |
| — | Saved import profiles (5.18) | MEDIUM | L | P3 | Invites scheduling, which invites an outbound call |
| — | Per-locale snippet values (4.10) | MEDIUM | M | P2 | **Flag for phase-specific research** — interacts with the star-shaped locale model |
| — | Arbitrary key field for CSV (5.19) | LOW | M | P3 | JSON scan per row |
| — | Latin-1 transcoding (5.20) | LOW | S | P3 | The refusal message is honest enough |

### Phases that likely need deeper research later

- **§3b `bereich`** — the "value must be visible without JS" constraint has three candidate answers
  and the choice is a UI-design question, not a technical one. Worth a UI-SPEC.
- **§4.10 per-locale snippet values** — the interaction with the star-shaped locale model is not
  obvious from the outside.
- **§5's dry-run screen** — the report layout for 312 rows with 40 problems is a real information-
  design problem and the one screen users will judge the feature by.

### Phases that need no further research

§1 (the neighbours agree, and the encoding is decided here), §2 (`KindRef` is a complete template),
§3a and §3c (native controls, and Go's escaping is already correct by default).

---

## Sources

Primary vendor documentation, fetched 2026-09-03. Confidence HIGH for behaviour claims sourced
directly from these; MEDIUM where a claim is inferred across systems.

- Statamic 6 fieldtypes: [button_group](https://statamic.dev/fieldtypes/button_group), [checkboxes](https://statamic.dev/fieldtypes/checkboxes), [select](https://statamic.dev/fieldtypes/select), [terms](https://statamic.dev/fieldtypes/terms), [code](https://statamic.dev/fieldtypes/code), [range](https://statamic.dev/fieldtypes/range)
- [Statamic Globals](https://statamic.dev/globals)
- [Statamic Importer — DOCUMENTATION.md](https://github.com/statamic/importer/blob/main/DOCUMENTATION.md) (unique field, pipe delimiter, "Create when missing?")
- Craft 5: [Checkboxes field](https://craftcms.com/docs/5.x/reference/field-types/checkboxes.html), [Time field](https://craftcms.com/docs/5.x/reference/field-types/time.html), [Globals](https://craftcms.com/docs/5.x/reference/element-types/globals.html)
- Craft Feed Me: [Field Mapping](https://docs.craftcms.com/feed-me/v4/feature-tour/field-mapping.html), [Creating your Feed](https://docs.craftcms.com/feed-me/v4/feature-tour/creating-your-feed.html)
- ACF: [Checkbox](https://www.advancedcustomfields.com/resources/checkbox/), [Button Group](https://www.advancedcustomfields.com/resources/button-group/), [Taxonomy](https://www.advancedcustomfields.com/resources/taxonomy/)
- Kirby: [checkboxes field](https://getkirby.com/docs/reference/panel/fields/checkboxes), [tags field](https://getkirby.com/docs/reference/panel/fields/tags)
- Directus: [Import & Export](https://directus.com/docs/guides/content/import-export) (`MAX_IMPORT_ERRORS`, headers must match field keys)
- WP All Import: [How to Import Posts](https://www.wpallimport.com/documentation/how-to-import-wordpress-posts/), [Define Unique Identifier Correctly](https://www.wpallimport.com/documentation/define-unique-identifier-correctly/), [Manual Record Matching](https://www.wpallimport.com/documentation/manual-record-matching/)

In-repository sources for the recommendations (HIGH confidence, read directly):

- `internal/field/field.go` (`Kinds`, `SubKinds`, `BlockKinds`, `SplitChoices`, `Data`, `Decode`, `MaxValueBytes`, `MaxRows`, `MaxFields`)
- `internal/field/render.go` (`Resolve`, `Links`, `Ref`, `Number`, `Entry`)
- `internal/term/store.go` (`Rename` keeps the address; `SetForPage` takes names; `Parse` splits on commas; `MaxPerPage`)
- `internal/snippet/store.go`, `internal/template/loader.go` (`.Site.Snippets map[string]template.HTML`)
- `internal/admin/wordpress.go` + `cmd/holzcloud/templates/admin/import_report.html` + `internal/bundle/import.go` (`Report`)
- Migrations `00028_page_fields.sql`, `00029_field_groups.sql`, `00038_block_types.sql`, `00045_pages_locale_unique.sql`
- `docs/offene-punkte.md`, `docs/vergleich-statamic.md`, `CLAUDE.md`

---
*Feature research for: Holzcloud CMS milestone v1.6 — Inhaltsmodell und Zugang*
*Researched: 2026-09-03*
