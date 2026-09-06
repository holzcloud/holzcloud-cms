# Phase 8: Snippets Carry Fields - Context

**Gathered:** 2026-09-06
**Status:** Ready for planning

> Written in English to match `ROADMAP.md` and `REQUIREMENTS.md`, the documents
> downstream agents read alongside this one.
>
> **The developer asked for the whole milestone to be carried out autonomously.**
> Every decision below is therefore Claude's, taken against the tree rather than
> from preference, and each one names the evidence it rests on. The four
> questions that would otherwise have been put to the developer are listed under
> *Claude's Discretion* with how each was settled. Where a decision overrides a
> note in `ROADMAP.md` or `REQUIREMENTS.md`, the override and its evidence are
> stated.

<domain>
## Phase Boundary

A text snippet stops being one Markdown box and becomes a small content model of
its own: any field kind, defined in the field-definition table that already
exists, rendered through the pipeline and the sanitisation that already exist,
with nothing that already works changing.

Requirements: SNIP-01 … SNIP-05

**Not in this phase:** the CSV importer (Phase 9), the gallery (Phase 11),
single sign-on (Phase 10). Per-locale snippet values are deferred (`V2-15`), and
revisions on snippets are deferred (`V2-17`) — neither is reopened here.

</domain>

<decisions>
## Implementation Decisions

### The migration

- **D-01:** Phase 8 ships migration **`00047`** — verified: the tree now runs to
  `00046_field_kinds.sql`, which Phase 7 added. Contents, in one file so a
  rollback is one file:
  1. `ALTER TABLE page_field_defs ADD COLUMN snippet_id` — **must default to
     NULL and must carry no `REFERENCES` clause**. SQLite refuses `ADD COLUMN`
     that carries a foreign-key reference together with any other default; the
     column is a plain `INTEGER` and the relationship is enforced in Go, the way
     `block_type_id` already is.
  2. The index swap.
  3. `ALTER TABLE snippets ADD COLUMN fields TEXT NOT NULL DEFAULT ''` — the
     same shape `pages.fields` uses, so `field.Encode`/`Decode` work unchanged.
  — **Reversibility:** one-way — a released migration is never edited; a
  correction is `00048`.

- **D-02:** **The stale note in the older planning material is corrected before
  it is followed.** `idx_page_field_defs_kennung_oben` is **not** still as
  `00029` wrote it: `00038:52-56` already replaced it, and it reads
  `ON page_field_defs(website_id, kennung) WHERE parent_id IS NULL AND
  block_type_id IS NULL`. Verified in the tree. `00047` therefore extends
  **`00038`'s** version, not `00029`'s:
  - the top-level index gains `AND snippet_id IS NULL`
  - a fourth partial index covers snippet fields:
    `ON page_field_defs(snippet_id, kennung) WHERE snippet_id IS NOT NULL`
  Read **`00029` and `00038`** before writing `00047`. `00038:44-46` explains
  the operation in the file itself — *„ein Austausch von zwei Indizes und kein
  Tabellenneubau"* — and this is the third time that pattern is walked.

### The carrier discriminator — the phase's one dangerous edit

- **D-03:** **`internal/field/store.go:59` is written first, before anything
  else in the phase.** It reads
  `WHERE website_id = $1 AND block_type_id IS NULL`. Without
  `AND snippet_id IS NULL` it puts **every snippet field on every page's edit
  form** and into every theme's `.Page.Feldliste`. Silent, and visible only in a
  browser.
  Measured, by extracting the backtick SQL literals rather than by counting
  grep lines: the carrier discriminator lives in **six** hand-written statements
  in `internal/field/store.go` — five `SELECT`s and one `INSERT` — carrying
  **eleven** `block_type_id` occurrences between them (`:57`, `:59`, `:94`,
  `:118-119`, `:145-147`, `:193`, `:250`, `:257`). Line 59 is the one that
  "already works", which is why it is the one that ships broken.

  Note the `UPDATE` at `:299-306` carries **no** discriminator: it updates by
  `id` and `website_id`. That is correct today and stays correct — but it means
  the count of statements to edit is not the same as the count of statements
  that touch this table. Check each one; do not pattern-match.
  **This step must be invisible to pages and blocks**: after it, the existing
  suite must be green with no snippet in the database.
  — **Reversibility:** reversible — but a miss is silent, so it gets its own
  task and its own test, not a comment.

- **D-04:** **Phase 7's lesson is applied here directly.** In Phase 7 the same
  shape appeared twice: a mechanism correct at every known site and wrong at one
  overlooked site — the block editor's form-name minting (CR-01) and the
  per-page parser applied to a whole archive (CR-02). The counting acceptance
  criterion is **not** sufficient on its own; a behavioural test that reads back
  through every query path is what catches a missed site. Phase 7's
  `TestNeueSpalten` is the shape to copy.

### The namespace budget — the roadmap's open question, settled

- **D-05:** **`MaxFields` is counted per carrier, not per website.**
  `internal/field/store.go:232` counts
  `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1` with no namespace
  filter, and `MaxFields = 60` (`internal/field/field.go:215`) is therefore one
  budget shared by three carriers today and four after this phase.
  **Decision: scope the count to the carrier being written** — a page-field
  create counts page fields, a block-kind field create counts that block kind's
  fields, a snippet-field create counts that snippet's fields.
  Rationale, from what the limit is for: the budget exists so that **one form**
  does not become unusable, and a form only ever renders one carrier's fields.
  A shared budget lets one carrier's usage silently deny another — an operator
  adding a second field to a snippet is told „mehr Felder gehen nicht" because
  block kinds ate the budget, which is exactly the kind of unexplained refusal
  this project avoids elsewhere. The guard keeps its purpose and stops lying
  about its reason.
  `MaxFields` keeps its value of 60. The error stays `ErrTooMany`.
  — **Reversibility:** reversible — it loosens a limit; nothing stored becomes
  invalid.

### The theme contract

- **D-06:** **`.Site.Snippets map[string]template.HTML` does not change type.**
  It is a published contract (`internal/template/loader.go:377`,
  `TEMPLATE-SPEC.md:212` and `:722-726`) that every installed theme indexes.
  Field values arrive through a **parallel** map beside it, not through it.
  The snippet's Markdown body stays exactly as it is: a snippet becomes body +
  optional fields, precisely as a page is content + optional fields. **Zero
  migration for existing content**, and SNIP-05's „snippets that already exist
  keep working untouched" is satisfied by construction rather than by testing
  for it afterwards.
  — **Reversibility:** one-way — the parallel map's name becomes a published
  contract the moment a theme uses it.

- **D-07:** The parallel map is named **`.Site.Bausteinfelder`**, keyed by
  snippet key, each value being the same `map[string]any` shape a page's
  `.Page.Felder` carries — so a theme author who has read the field section of
  `TEMPLATE-SPEC.md` already knows how to read it. A second entry
  `.Site.Bausteinliste` carries the ordered `[]field.Entry` per snippet, mirroring
  `.Page.Feldliste`. Two shapes, the same two shapes pages already have; no third
  spelling. This is the same reasoning D-02 of Phase 7 applied to values.

### What must not change

- **D-08:** Rendering goes through the **existing** pipeline: goldmark →
  bluemonday → `template.HTML`. One chain, not a second. A `<script>` in a
  snippet's long-text field is sanitised exactly as it is on a page, because it
  is the same code — SNIP-04 is met by reuse, and the test proves the reuse
  rather than re-testing goldmark.

- **D-09:** `page_field_defs` is already a **three-namespace** table (page
  fields `parent_id IS NULL AND block_type_id IS NULL`, group sub-fields, and
  block-kind fields). This phase adds the fourth and must not disturb the first
  three. Every query that selects a carrier's fields **excludes the other three
  namespaces explicitly** — no implicit "whatever is left" branch, because that
  is what breaks when a fifth carrier arrives.

### Build order

- **D-10:** The roadmap's order is adopted unchanged, because each step is
  provable before the next begins:
  1. **The migration alone** (`00047`) — a rollback is one file.
  2. **`internal/field/store.go:59` and its ten siblings** — the discriminator,
     before anything else. Invisible to pages and blocks when done.
  3. `Def.SnippetID`, `scanDef`, `Create`, `Update`, `Move`, `validate`.
  4. The `snippet` store carries `fields` (`internal/snippet/store.go`).
  5. The admin screen (`internal/admin/snippet.go`) — **half the phase's
     effort**.
  6. Theme surface + fixtures + spec + i18n + the browser pass.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition
- `.planning/ROADMAP.md` §"Phase 8: Snippets Carry Fields" — six success
  criteria, the build order, the named dangerous edit.
- `.planning/REQUIREMENTS.md` — SNIP-01…SNIP-05.
- `.planning/phases/07-field-kinds/07-CONTEXT.md` — Phase 7's locked decisions;
  the field system this phase extends was shaped by them.
- `.planning/phases/07-field-kinds/07-SECURITY.md` — **T-07-26 (Plan 06) is
  still open**: the mitigation names `field.Hidden` as dropping hidden values on
  save, and `Hidden` is called on no save path. Phase 8 touches `CheckAll` and
  `Clean`'s callers; if the wording is corrected anywhere, it is here.

### The field system
- `internal/field/store.go` — the eleven `block_type_id` occurrences across
  seven statements; `:59` is D-03's subject; `:232` is D-05's count.
- `internal/field/field.go` — `MaxFields` (`:215`), `Def` (`:232`),
  `maxKeyBytes` (`:223`).
- `internal/db/migrations/00029*.sql` and `00038*.sql` — read both before
  writing `00047`; `00038:52-60` is the current index shape.
- `internal/db/migrations/00028:25-27` — the precedent for
  `ALTER TABLE ADD COLUMN` without a CHECK.

### The snippet system
- `internal/snippet/store.go` — gains `fields`.
- `internal/admin/snippet.go` — the screen, half the phase.
- `internal/db/migrations/00010_scheduling_search_redirects.sql:34-44` — the
  `snippets` table: `STRICT`, `UNIQUE (website_id, key)`.

### Contracts that fail the build if a carrier is missing from them
- `internal/tmplspec/TEMPLATE-SPEC.md` (`:212`, `:722-726` for `.Site.Snippets`)
- `internal/template/loader.go:372-377` — the `Snippets` field and its comment
- `internal/template/sample.go` — `SampleData` **and** `MinimalData`
- `internal/tmplspec/spec_test.go` and `internal/template/sample_test.go` —
  Phase 7 added `TestSpecDocumentsEveryFieldEntryMember`,
  `TestSpecDocumentsEveryFieldKind`,
  `TestSampleFieldsAreShapedLikeTheRendererProducesThem` and
  `TestMinimalDataCarriesTheEmptyValueOfEveryOwnField`. A new `Site` member
  must satisfy them.

### Constraints
- `CLAUDE.md` — Go stdlib-first; **htmx is the only JavaScript and enhancement
  only**; plain CSS; goose migrations, never edited once released; nothing
  fetched from a third party at runtime; every resource scoped to exactly one
  website.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **The block-kind namespace is a complete template.** `block_type_id` was added
  to `page_field_defs` the same way `snippet_id` now will be, with a partial
  unique index of its own (`00038:58-60`). Copy that shape; do not invent one.
- **`pages.fields`** is the precedent for `snippets.fields`: a `TEXT` column
  holding `field.Encode`'s JSON. The encoder, decoder and every reader work
  unchanged.
- **Phase 7's contract tests** already enumerate every field kind and every
  `Entry` member. A new carrier inherits that enforcement for free — and fails
  the build if it is added to the structs and not to the spec.

### Established Patterns
- **Partial unique indexes per namespace**, not one index with a discriminator
  column — `00038` establishes it.
- **The relationship is enforced in Go, not by a foreign key**, because SQLite's
  `ADD COLUMN` cannot carry `REFERENCES` alongside a default. `block_type_id`
  already lives this way.
- **A released migration is never edited.**
- **The stored value is always a string**, converted to its meaning once on the
  way out in `Resolve`.

### Integration Points
- `internal/field/store.go` → the fourth namespace, at seven statements.
- `internal/snippet/store.go` → `fields` column, encode/decode.
- `internal/admin/snippet.go` → the screen.
- `internal/template/loader.go` → `Site.Bausteinfelder` + `Site.Bausteinliste`.
- `TEMPLATE-SPEC.md` + `SampleData` + `MinimalData` → or the suite fails.

</code_context>

<specifics>
## Specific Ideas

- **Verified against the tree during this analysis**, so the planner need not
  re-derive: migrations run to `00046`; `idx_page_field_defs_kennung_oben` is
  `00038`'s version and carries `parent_id IS NULL AND block_type_id IS NULL`;
  `snippets` is `STRICT` with `UNIQUE (website_id, key)`; `MaxFields = 60` is
  counted without a namespace filter at `store.go:232`;
  `Site.Snippets` is `map[string]template.HTML` at `loader.go:377`;
  `internal/snippet` contains only `store.go` and `store_test.go`;
  `internal/admin/snippet.go` exists.
- **`internal/admin` sits at low coverage and is where authorisation lives.**
  Reuse `internal/admin/page_handler_test.go` as the harness; do not invent a
  second one.

</specifics>

<deferred>
## Deferred Ideas

- **Per-locale snippet values** (`V2-15`). Both Statamic and Craft localise
  their globals; here it meets the star-shaped locale model in ways not visible
  from outside, and it wants its own research. Not reopened.
- **Revisions on snippets** (`V2-17`). Untouched.
- **Raising `MaxFields`.** D-05 scopes the count rather than raising the number.
  If 60 per carrier ever binds, that is a separate, evidenced decision.

</deferred>

---

## Claude's Discretion

The developer delegated the whole milestone. The four areas that would otherwise
have been put to them, and how each was settled:

| Area | Settled by |
|---|---|
| `MaxFields`: shared budget or per namespace? — the roadmap's explicit open question | **D-05** — the limit exists so one *form* stays usable, and a form renders one carrier; a shared budget makes one carrier deny another with a reason that reads false |
| How do field values reach a theme without changing `.Site.Snippets`? | **D-06/D-07** — a parallel pair mirroring `.Page.Felder` / `.Page.Feldliste`, so a theme author who knows the page contract already knows this one |
| Does `snippet_id` get a foreign key? | **D-01** — no; SQLite refuses `ADD COLUMN` with `REFERENCES` plus a default, and `block_type_id` already lives this way |
| Which index does `00047` extend? | **D-02** — `00038`'s, not `00029`'s; the older planning note was one migration behind, verified in the tree |

---

*Phase: 8-Snippets Carry Fields*
*Context gathered: 2026-09-06*
