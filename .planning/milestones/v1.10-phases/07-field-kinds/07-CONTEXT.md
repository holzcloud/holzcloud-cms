# Phase 7: Field Kinds - Context

**Gathered:** 2026-09-05
**Status:** Ready for planning

> Written in English to match `ROADMAP.md` and `REQUIREMENTS.md`, the documents
> downstream agents read alongside this one.
>
> **The developer asked for no questions: "entscheide alles du. keine fragen."**
> Every decision below is therefore Claude's, taken against the tree rather than
> from preference. Where a decision overrides a note in `ROADMAP.md`,
> `REQUIREMENTS.md` or `docs/offene-punkte.md`, the override and its evidence are
> stated — and D-01 corrects the note itself, so a later reader is not left with
> two documents disagreeing.

<domain>
## Phase Boundary

A website's content model reaches every field kind this milestone names — a
Choice that can render as a button row, a genuinely multi-valued field, terms as
a field kind, and the three small kinds still missing (`zeit`, `bereich`,
`code`) — and each one survives save, reload, public render and the bundle round
trip.

Requirements: FIELD-01 … FIELD-08

**Not in this phase:** snippets carrying fields (Phase 8), the CSV importer
(Phase 9) — though FIELD-07 exists precisely so Phase 9 inherits this phase's
encoding rather than inventing a third one.

</domain>

<decisions>
## Implementation Decisions

### Ground truth first

- **D-01:** The **first** task amends the three documents that currently
  disagree with each other or with the tree, in its own docs commit, before any
  code is touched. Same shape as Phase 6's D-01, and for the same reason: this
  phase is planned against these notes.
  1. **`REQUIREMENTS.md:295`** reads "``code`` and `KindTerm` are excluded from
     block kinds where the reason applies". Its stated reason is that a block
     freezes to HTML on save, so a value that must follow a later rename cannot
     live in one. That reason applies to `KindTerm` and **not** to `code` — and
     ROADMAP success criterion 5 requires the opposite for `code`: HTML typed
     into it "appears verbatim on the public page and does not execute,
     **including when the field sits inside a block**". Correct the row to name
     `KindTerm` only. See D-06.
  2. **`docs/offene-punkte.md` §1** says "zwei neue Arten neben `KindChoice`"
     (two new kinds beside Choice). That is the older sketch and is wrong on
     one half: the button row is a display mode, not a kind (D-04). Correct it
     to "one new kind, plus a display mode on the existing one".
  3. **`docs/offene-punkte.md` §6** describes `bereich` as "eine Zahl zwischen
     zwei Grenzen, **als Schieber**". A slider cannot satisfy FIELD-05's
     no-JavaScript readability constraint (D-07). Correct the sketch to the
     control that can.
  — **Reversibility:** reversible — three prose edits.

### The multi-value mechanism (FIELD-07) — build this first

Everything else in this phase, and all of Phase 9, depends on it. The roadmap's
build order is adopted unchanged: encoding first, then the three small kinds,
then multiple choice, then the button row, then `KindTerm` (parallelizable),
then fixtures + spec + i18n + browser pass.

- **D-02:** `SplitValues` / `JoinValues` are added beside `SplitChoices` /
  `JoinChoices` in `internal/field/field.go` and are the **only** pair. One
  value per line in the string slot that already exists, per
  `REQUIREMENTS.md:291` — the delimiter is already illegal inside a value
  because the option list is read one per line. Exported, so Phase 9's importer
  inherits them instead of inventing a third spelling.
  `page_form.go:150` and `:163` become `field.JoinValues(values)`.
  The CSV pipe separator (`REQUIREMENTS.md:292`) is a **boundary conversion in
  the importer**, not a second mode of these functions.
  — **Reversibility:** costly — the stored format reaches the database, the
  bundle round trip and Phase 9; changing it later means a data migration.

- **D-03:** Multi-valuedness is encoded in the **form field name**, minted in
  the one place that mints names — `Def.FieldName()` at `field.go:346` — as
  **`feld_<key>[]`**.
  Rationale: `fieldsFromRequest` (`page_form.go:150`) deliberately reads by
  prefix *before* definitions are loaded, and that property is load-bearing; the
  fix must not be "load the defs here". After `CutPrefix("feld_")` the handler
  tests a `[]` suffix: present → multi-value, key is the trimmed remainder,
  `out.Values[key] = field.JoinValues(values)`; absent → today's behaviour
  unchanged. A field key is slug-like and cannot contain `[`, so the marker
  cannot collide with a key. One prefix family rather than two.

### The field kinds

- **D-04:** **The button row is a display mode of `auswahl`, not a new kind.**
  `darstellung` (its own column, per `REQUIREMENTS.md:293`) selects list or
  button row. FIELD-01's own wording is "configure a Choice field to render as a
  row of buttons" — same options, same stored value, same theme contract, only
  the control changes. A second kind would pay the full per-kind tax
  (`Kinds`, `SubKinds`, `BlockKinds`, `field_input.html`, `CheckAll`, `Resolve`,
  `TEMPLATE-SPEC.md`, `SampleData`, `MinimalData`, i18n, bundle, AI round trip)
  for no semantic difference.

- **D-05:** **Multiple choice IS its own kind** (`mehrfachauswahl`), *not*
  `auswahl` with `max_werte > 1`.
  Measured, not assumed: `internal/field/render.go`'s `Resolve` switch has **no
  `case KindChoice`** — a Choice falls through to the default branch and reaches
  a theme as a plain string. Making `auswahl` multi-valued would silently change
  the type every existing theme already reads, on every existing site, with no
  error anywhere. A new kind is purely additive and lets `auswahl` keep its
  contract permanently.
  `max_werte` therefore belongs to `mehrfachauswahl` alone and is enforced
  **server-side in `CheckAll`** — a checkbox count cannot be capped in markup
  without JavaScript.
  — **Reversibility:** one-way — once a site stores values under a kind key,
  merging the two kinds later needs a migration and a theme-contract change.

- **D-06:** `BlockKinds()` excludes **`KindTerm` only**, beside the existing
  `KindRef`. `code` stays available inside a block, and its escaping happens in
  the **block render path** — blocks freeze to HTML when the page is saved, so
  escaping in the theme would be too late. This is the easy miss the roadmap
  names; it must be a task, not a comment.
  Both exclusions are silent if missed, because `SubKinds()` and `BlockKinds()`
  are **subtractive** — verified in `field.go:87` and `:105`, a new kind is in
  unless explicitly excluded.

- **D-07:** **`bereich` is `<input type="number" min max step>`, not a slider.**
  FIELD-05's binding constraint is that "the chosen number is readable **before**
  saving, without JavaScript". Candidates weighed against it:

  | Candidate | Verdict |
  |---|---|
  | `<input type=range>` alone | the value is never textually visible — fails |
  | `<input type=range>` + `<output>` | `<output>` only updates via script; `internal/tmplmgr/script.go` rejects exactly this pattern in uploaded templates — fails |
  | `<input type=range>` + `<datalist>` ticks | shows the scale, not the chosen number — fails |
  | `<input type=number min max step>` | the chosen number *is* the visible content; bounds enforced natively; no script — **passes** |

  FIELD-05 says "Range field bounded below and above", never "slider"; only the
  older sketch in `offene-punkte.md` §6 does, and D-01 corrects it. The slider
  affordance is the acknowledged loss and is recorded under Deferred.

- **D-08:** **`MayControl()` stays `true` for `bereich`** — the roadmap's note to
  return `false` rests on a premise D-07 removes. Its stated reason is "an
  `<input type=range>` never matches `:placeholder-shown`"; a number input does
  match it, so the existing `.feld-schalter--text` rule
  (`cmd/holzcloud/assets/admin.css:1108`) should work unchanged.
  **This one is conditional and must be confirmed in the browser pass**, which
  criterion 6 demands anyway: if a `bereich` field does not correctly show and
  hide its dependants, add `KindRange` to the `MayControl()` exclusion beside
  `KindDate` and record that the note was right after all. Do not assume either
  way from reading.

- **D-09:** `KindTerm` copies `KindRef` wholesale — chooser at
  `page_fields.go:98–255`, resolution in `render.go` (`case KindRef`, `:140`).
  Stores the term's **slug** (`REQUIREMENTS.md:294`), prints the term's **name**,
  so a rename changes what the page shows without touching the page. The site's
  existing terms come from `internal/admin/term.go`. Independent of D-02…D-08 and
  parallelizable.

### The two named traps

- **D-10:** `switchOf` (`page_fields.go:186–195`) gains a case for a Choice
  rendered as a button row, returning a **new** switch name with a matching CSS
  rule beside `admin.css:1104`. The existing rule is
  `:has(> .form-group option[value=""]:checked)` and a radio row has no
  `<option>`, so without this **every field hanging on that Choice silently
  stops appearing and disappearing**. The new rule matches the radio instead:
  `:has(> .form-group input[type="radio"][value=""]:checked)`.
  Scheduled **last** in the build order so the conditional-field rule is touched
  exactly once (FIELD-08).

- **D-11:** Two shapes that are five lines each and multi-hour bugs if missed:
  - An **optional button row renders an explicit "— keine Angabe —" radio**
    with `value=""`. A radio group cannot be un-set in plain HTML, and that
    option is also what D-10's CSS rule matches.
  - A **hidden sentinel precedes every checkbox group**, sharing its name:
    `<input type="hidden" name="feld_<key>[]" value="">`. An all-unchecked group
    otherwise submits nothing and the server cannot tell "cleared" from "this
    form never carried the field". With the sentinel: present but empty →
    cleared; absent from `r.Form` → untouched. `JoinValues` drops empty entries,
    so a partly-ticked group is unaffected.

- **D-12:** Multiple choice is rendered as **checkboxes**, not
  `<select multiple>`. A multiple select needs ctrl-click, offers no affordance
  that it is multi-valued, and is hostile on touch. Checkboxes also make D-11's
  cleared-vs-absent distinction expressible in markup.

- **D-13:** `MaxValueBytes` (4000, `field.go:158`) is shared across **all**
  selected values of one field. `field.go:496–497` currently truncates silently
  (`val = val[:MaxValueBytes]`) — for a multi-valued field that would cut a
  value in half. `CheckAll` must **report** the overflow instead.

### Migration

- **D-14:** Phase 7 ships migration **`00046`** — verified: the tree runs to
  `00045_pages_locale_unique.sql`. `darstellung`, `max_werte` and the `bereich`
  bounds get their **own columns** on `page_field_defs`, not a reuse of the
  `auswahl` column, which is read one option per line and would reintroduce
  exactly the ambiguity D-02 avoids. `ALTER TABLE ADD COLUMN`, and explicitly
  **no CHECK**, per `00028:25–27`. `page_field_defs.art` itself needs nothing:
  it is already `TEXT NOT NULL` with no CHECK — verified against the schema.
  — **Reversibility:** one-way — a released migration is never edited; a
  correction is a new migration.

### Claude's Discretion

The developer delegated every decision ("entscheide alles du. keine fragen").
The four areas that were about to be put to them, and how each was settled:

| Area | Settled by |
|---|---|
| Multiple choice: own kind or a mode of `auswahl`? | D-05 — `Resolve` has no `case KindChoice`, so a Choice reaches themes as a string; making it multi-valued breaks every existing theme silently |
| `bereich` readable without JavaScript | D-07 — three of four candidates fail the constraint outright; the fourth is a bounded number input |
| `code` inside a block: REQUIREMENTS vs. criterion 5 | D-06 + D-01 — criterion 5 wins, the requirement row is corrected |
| The multi-value name scheme, control and sentinel | D-03, D-11, D-12 |

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition
- `.planning/ROADMAP.md` §"Phase 7: Field Kinds" — six success criteria, the
  build order, the per-kind tax, the two named pitfalls with addresses. Note
  D-01 corrects nothing here; the roadmap's own notes are sound apart from the
  `MayControl()` premise D-08 revises.
- `.planning/REQUIREMENTS.md` — FIELD-01…08, and the decision table at
  `:291–295`. Row `:295` is corrected by D-01.
- `.planning/phases/06-aufr-umen/06-CONTEXT.md` — the previous phase's working
  rules that still apply: notes must read true, measure rather than assume, the
  standing gate is run even when it is expected to be green.

### The field system
- `internal/field/field.go` — `Kinds` (`:26–49`), `SubKinds()` (`:87`),
  `BlockKinds()` (`:105`), `MayControl()` (`:212`), `MaxValueBytes` (`:158`),
  the silent truncation (`:496`), `FieldName()` (`:346`),
  `SplitChoices`/`JoinChoices` (`:614`, `:625`), `CheckAll` (`:633`).
- `internal/field/render.go` — `Resolve`'s switch (`:102`), `KindRef`
  resolution (`:140`), `List` (`:193`), `Filled` (`:253`). **`KindChoice` has
  no case here** — that absence is the evidence behind D-05.
- `internal/admin/page_form.go:150` and `:163` — the two lines that keep only
  the first value of a group.
- `internal/admin/page_fields.go:98–255` — the `KindRef` chooser, the template
  `KindTerm` copies. `switchOf` at `:186–195`.
- `cmd/holzcloud/assets/admin.css:1096–1110` — the four conditional-field rules;
  `:1104` is the one a button row breaks.
- `cmd/holzcloud/templates/admin/field_input.html` — every kind's control.
- `internal/admin/term.go` — the site's terms, for D-09's chooser.

### Contracts that fail the build if a kind is missing from them
- `internal/tmplspec/TEMPLATE-SPEC.md`
- `internal/template/sample.go` — `SampleData` and `MinimalData`
- Three tests tie those to the loader structs; a new field missing from one
  fails the suite. This is the per-kind tax the roadmap says to budget once.

### Constraints
- `CLAUDE.md` — hard stack mandate; **no JavaScript beyond htmx**.
- `internal/tmplmgr/script.go` — rejects `<script>`, inline handlers and
  `javascript:` in uploaded templates. This is what rules out a live slider
  readout (D-07).
- `docs/offene-punkte.md` §1 and §6 — the original sketches; both corrected by
  D-01.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`KindRef` is a complete end-to-end template** for `KindTerm`: chooser,
  storage of a stable identity, resolution to a display value at render time,
  and exclusion from `BlockKinds()`. D-09 copies it wholesale.
- **`SplitChoices`/`JoinChoices`** already establish the one-value-per-line
  convention D-02 extends.
- **`switchOf` + the four `.feld-schalter--*` CSS rules** are an existing,
  working no-JS conditional-field mechanism. D-10 adds a fifth case, not a new
  mechanism.
- **`pool`** (`page_fields.go`) already carries what the choosers need and its
  doc comment says a third kind of chooser should not mean touching every call
  site — which is exactly what D-09 is.

### Established Patterns
- **Subtractive kind lists.** `SubKinds()` and `BlockKinds()` filter *out*, so a
  new kind is available everywhere by default. Both exclusions in D-06 are
  invisible if forgotten.
- **The stored value is always a string**, converted to its meaning once on the
  way out in `Resolve`. D-02 keeps that: multi-value is a joined string in
  storage and a `[]string` only after `Resolve`.
- **A released migration is never edited.** `00028:25–27` is the precedent for
  `ALTER TABLE ADD COLUMN` without a CHECK.
- **No JavaScript beyond htmx**; htmx is enhancement only and every control must
  submit as a plain form.

### Integration Points
- `field_input.html` → one control per kind, plus `darstellung` branching for
  `auswahl` (D-04).
- `page_form.go` → the `[]` suffix test (D-03) and the sentinel (D-11).
- `render.go` → `case []string` in `Resolve`, `List` and `Filled`; `Entry` gains
  `Values []string`.
- `CheckAll` → `max_werte` (D-05) and the shared byte budget (D-13).
- Migration `00046` → four new columns on `page_field_defs` (D-14).
- `TEMPLATE-SPEC.md` + `SampleData` + `MinimalData` → every new kind, or the
  suite fails.

</code_context>

<specifics>
## Specific Ideas

- **Verified against the tree during this discussion**, so the planner need not
  re-derive: `FieldName()` really is the single minting point; `page_form.go:150`
  really does keep `values[0]`; `SubKinds`/`BlockKinds` really are subtractive;
  `MayControl()` really does exclude `KindDate`; `admin.css:1104` really does
  match `<option value="">`; migrations really do run to `00045`;
  `page_field_defs.art` really is `TEXT NOT NULL` with no CHECK; `Resolve`
  really has no `case KindChoice`.
- The one thing the roadmap got wrong is small and would not have hurt: its
  `MayControl()` note assumes a slider. D-08 revises it and asks the browser
  pass to confirm.

</specifics>

<deferred>
## Deferred Ideas

- **A real slider for `bereich`.** Ruled out by FIELD-05's no-JavaScript
  readability constraint (D-07), not by taste. It would need a scripted readout,
  which `internal/tmplmgr/script.go` rejects by design. Revisit only if that
  rule ever changes — and per `docs/offene-punkte.md` §"Was bewusst nicht gebaut
  wird", it will not.
- **Merging `auswahl` and `mehrfachauswahl` behind one kind with `max_werte`.**
  Rejected for this milestone by D-05 because it silently retypes what every
  existing theme reads. If it is ever wanted, it needs a migration and a
  theme-contract change, not a refactor.
- **Nesting a group inside a group.** Untouched here; `SubKinds()` still
  excludes `KindGroup` on purpose.

</deferred>

---

*Phase: 7-Field Kinds*
*Context gathered: 2026-09-05*
