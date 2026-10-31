---
phase: 08-snippets-carry-fields
fixed_at: 2026-09-06T14:05:00Z
review_path: .planning/phases/08-snippets-carry-fields/08-REVIEW.md
iteration: 1
findings_in_scope: 9
fixed: 9
skipped: 0
status: all_fixed
---

# Phase 8: Code Review Fix Report

**Fixed at:** 2026-09-06T14:05:00Z
**Source review:** `.planning/phases/08-snippets-carry-fields/08-REVIEW.md`
**Iteration:** 1
**Scope:** critical + warning (CR-01, WR-01 … WR-08). The four Info findings
were out of scope; IN-01 was carried along because it is one wrong sentence in
a file WR-03 had to edit anyway.

**Summary:**

- Findings in scope: 9
- Fixed: 9
- Skipped: 0

**Verification recorded where it ran:** every gate below was run **inside the
isolated worktree** (`.claude/worktrees/rf-08-…`), not in the main checkout.
This project has no `node_modules` and no build tooling — a Go worktree runs
the real gates — so the numbers reproduce from the main checkout after the
fast-forward.

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go test -count=1 ./...` | 40 packages, all `ok` (uncached) |
| `gofmt -l .` | no output |
| `go vet ./...` | clean |
| `go run ./tools/i18n` | `1158 übersetzt, 0 offen, 0 verwaist` for en, es, fr, it |

**No new visible string was added**, so no `-write`/`-schweiz` pass and no
translation commit was needed. The two new German sentences are import-report
warnings; `internal/bundle/import.go` contains zero `i18n` references — every
report line there is a raw `fmt.Sprintf`, and the new ones follow it.

**Every fix has a gate that was watched failing first.** What was actually seen
is recorded per finding below — not what was expected.

## Fixed Issues

### CR-01: `00047`'s snippet index has no `parent_id IS NULL`

**Files modified:** `internal/db/migrations/00048_snippet_group_namespace.sql`
(new), `internal/field/store_test.go`, `internal/db/migrations_down_test.go`
**Commit:** `4367eb3`

`00047` copied `00038`'s partial index, where the shape is correct because a
block kind cannot carry a group. A snippet can, and since 08-05 a group's
sub-fields inherit `snippet_id` — so they fell into the snippet's top-level key
namespace. A released migration is never edited; the correction is `00048`,
which swaps the index for `WHERE snippet_id IS NOT NULL AND parent_id IS NULL`.
Uniqueness inside a group is unaffected: `idx_page_field_defs_kennung_gruppe`
from `00029` covers every sub-field regardless of its group's carrier.

`00048`'s `Down` restores **`00047`'s** form (no `parent_id`), which is the trap
this project has now walked three times. The comment in the file says so, and
says that a rollback may legitimately fail if two groups have since been given
the same sub-key — the failure would be the truth, not a bug.

**Seen failing:**

```
--- FAIL: TestBausteinGruppenNamensraum
    store_test.go:1239: Textbaustein, „tag“ in der zweiten Gruppe: dieses Feld
    gibt es schon — was die Seite trägt, muss der Textbaustein auch tragen
```

The test runs the same four steps on a page and on a snippet; the page half
passed throughout, which is what makes the snippet half a defect rather than a
rule. A second gate, `TestMigration00048RunterUndRauf`, reads the index text
after the rollback. I deliberately broke the `Down` half (restoring the *new*
shape) and watched it fail — `migrations_down_test.go:173` — then restored it.

### WR-01: `Clean` and `CheckAll` skipped when the carrier has no definitions

**Files modified:** `internal/bundle/import.go`, `internal/bundle/bundle_test.go`
**Commit:** `6402d46`

Both guards hung on `if len(defs) > 0`, on **both** carriers. A manifest that
brings `values` and omits `fields` — the one shape that is entirely hand-written
— passed neither guard and went into the column verbatim. Removed the condition
in `importFieldValues` and `cleanSnippetValues` alike, so the two paths stay one
decision. `field.Clean` with no definitions correctly drops everything (verified
by reading `field.go:576-616`: `simple` and `groups` stay empty, so every value
is skipped), which is the right outcome — a value under no definition is
renderable by nothing.

The doc comment above `cleanSnippetValues` claimed the opposite of what the code
did. Both comments now record why the condition is gone.

**Seen failing:** five lines from
`TestWerteOhneDefinitionWerdenAufBeidenTraegernVerworfen` — 4001 bytes stored on
the page, 4001 on the snippet, one group row each under no definition, and a raw
`fields` column of 4069 bytes. The raw column is measured separately from the
decoded map on purpose: a value can fall out of `Decode` and still sit in the
database.

### WR-02: `importSnippetFields` dereferences a snippet that may be nil

**Files modified:** `internal/bundle/import.go`, `internal/bundle/bundle_test.go`
**Commit:** `09b7436`

`snippet.Store.Create` ends in `Get`, which returns `(nil, nil)` for a missing
row — through the **read** pool, a different pool from the one that wrote.
Before 08-05 the value was discarded; reading `created.ID` made it a crash.
`internal/admin/snippet.go:307-310` guarded the same value, so the two call
sites disagreed. Now both say the same thing, and the report names the snippet
that did not arrive.

**Seen failing:** not a failed assertion — a crash.

```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV] … bundle.importSnippets … import.go:806
```

The condition is staged through the pools rather than a fake: write pool on the
real database, read pool on a second migrated-but-empty one. That is exactly
what `Get` sees when its row is not there, and it needs no interface extraction.

### WR-03: picture, reference and label values lost silently on the round trip

**Files modified:** `internal/bundle/import.go`, `internal/bundle/format.go`,
`internal/bundle/bundle_test.go`,
`.planning/phases/08-snippets-carry-fields/deferred-items.md`
**Commit:** `7cfbccd`
**Status: fixed as the review's "minimum", deliberately not as the "proper" fix.**

Only the loudness changed; the data path did not. `ortsgebundeneWerte` walks the
definitions (top level and group sub-fields) and names every image/reference/
label value in the import report, so the operator re-picks it instead of finding
the gap. The value still travels — nothing disappears.

**The review's literal suggestion did not apply**: it appends to `m.Notes`, and
`Manifest` has no `Notes` member. Adding one is a format change, and the same
file's comment explains that `omitempty` is what keeps a manifest byte-identical
to a pre-phase one. The import report is in any case where the person it
concerns is standing. Adapted rather than skipped.

**The second half of the finding is now satisfied**: the limitation is recorded
in `deferred-items.md` with what was fixed, what stays open, and why hoisting
`translateOut`/`translateIn` out of the page path is a roadmap item. Before this
it was tracked only by a code comment that called itself "a recorded
limitation".

**Seen failing:** `TestBildwertEinesTextbausteinsWirdBeimImportGemeldet` found
an empty warnings list for a snippet carrying `logo: "42"` under a
`field.KindImage` definition.

**Carried along (IN-01, out of scope):** the same `deferred-items.md` said of
the orphaned sub-fields *"kein Leseweg gibt sie heraus"*. `Store.Sub` selects by
`website_id` and `parent_id` only, so the group screen does return them; only
the snippet's own form does not. Sentence corrected. The disposition — no repair
migration — is untouched and still stands, because 08-03 and its fix are in the
same phase.

### WR-04: three themed public routes never filled the snippet surface

**Files modified:** `internal/public/handler.go`, `internal/public/access.go`,
`internal/public/pagedata.go`, `internal/public/bausteinfelder_test.go`
**Commit:** `cbedf98`

`renderNotFound`, `serveShareError` and `HandleMaintenance` called neither
`loadSnippets` nor `fillSnippets`. All three now do. These are precisely the
pages where a visitor is looking for the contact block: the address they found
is wrong, the preview link expired, the site is away.

**`HandleMaintenance` was the judgement call and it is decided rather than
passed over**: the maintenance page is the only page a visitor sees while the
site is down, and the line they want is the phone number. One query on a page
nobody requests often. The reason is in the comment, so it does not read like an
omission either way.

`fillSnippets`' doc comment now records the gap its counting gate structurally
cannot see — a route that does neither — and names the test that covers it
instead.

**Seen failing:** three sub-tests, three empty elements.

```
--- FAIL: …/404                      <p class="telefon"></p>
--- FAIL: …/abgelaufener_Vorschaulink <p class="telefon"></p>
--- FAIL: …/Wartung                   <p class="telefon"></p>
```

The fixture theme's `404.html` gained the line and a `maintenance.html` was
added, so the assertion reads a rendered body rather than a struct — which is
the only way this class of gap is visible at all.

### WR-05: the field screen advised a template expression that will not parse

**Files modified:** `cmd/holzcloud/templates/admin/field_list.html`,
`internal/admin/snippet_fields_test.go`
**Commit:** `306bd12`

Now `{{index .Site.Bausteinfelder "<key>" "kennung"}}`, the form
`TEMPLATE-SPEC.md` already uses. Confirmed the failure independently before
fixing: `{{.Site.Bausteinfelder.footer-kontakt.telefon}}` →
`template: t:1: bad character U+002D '-'`; the `index` form parses.

**Scope checked, not assumed:** the group branch on the same screen renders
`{{range .Page.Felder.{{.Group.Key}}}}`, which is *not* affected —
`internal/field/store.go:807-819` restricts a field key to `a-z`, `0-9` and `_`.
Only the snippet branch is exposed, because `internal/admin/snippet.go:97-110`
permits `-` in a snippet key.

**Seen failing:**

```
snippet_fields_test.go:663: der Rat auf dem Bildschirm ist kein übersetzbarer Ausdruck:
  {{.Site.Bausteinfelder.footer-kontakt.kennung}}
  template: rat:1: bad character U+002D '-'
```

The test renders the real screen for a snippet keyed `footer-kontakt`, pulls the
`<code>` block out of the body, un-escapes the entities and feeds it to the Go
parser. It asserts the property — *this parses* — and not the wording, so
rephrasing the advice later will not make it red for the wrong reason.

### WR-06: `SetFields` did not touch `updated_at`

**Files modified:** `internal/snippet/store.go`, `internal/snippet/store_test.go`
**Commit:** `c43f3e7`

`Rendered.LatestUpdate` is what `contentModTime` uses for **every** page of the
site. This phase made a snippet's field values part of what a page renders, and
their only writer left the validator alone. It worked only because
`handleSnippetSave` happens to call `Update` first — an ordering coupling
between two packages that nothing enforces. The comment records why
`page.Store.SetFields` is not a precedent here: no site-wide validator hangs off
a page.

**Seen failing:**

```
store_test.go:190: updated_at = 2020-01-01T00:00:00Z, unverändert seit
2020-01-01T00:00:00Z — ohne den Stempel antwortet jede Seite der Website 304
mit den alten Feldwerten
```

The test rewinds the stamp instead of sleeping: the clock has second resolution,
so a waiting test would measure its own runtime rather than the guard.

### WR-07: `Create` never checked that the carrier belongs to the website

**Files modified:** `internal/field/store.go`, `internal/field/store_test.go`,
`internal/bundle/import.go`
**Commit:** `2907cae`

Chose to make the comment true rather than delete it, per the review's second
option: `Create` now verifies the carrier row against `d.WebsiteID` before
writing. `REFERENCES` only proves the row exists, and both partial indexes are
keyed on the carrier column alone, so the database would have stored a
cross-website definition without complaint. Two new errors, `ErrNoSnippet` and
`ErrNoBlockType`. The helper interpolates a table name — the two call sites pass
fixed literals from inside the same file, never anything from outside, and both
ids stay bound.

`block_type_id` has carried the same gap since `00038` and is closed in the same
line, which is what the review argued for. The false sentence in
`importSnippetFields`' doc comment now says when it became true and what it was
before.

**Seen failing:** both halves, and the test could not even compile until the two
error values existed — so the errors were added first and the guard second, to
watch the runtime failure:

```
store_test.go:1303: ein Feld an einem Textbaustein einer fremden Website wurde angelegt
store_test.go:1311: ein Feld an einer Bausteinart einer fremden Website wurde angelegt
```

The test also creates a field on the website's *own* snippet and block kind, so
a guard that turned into a blanket refusal would be caught.

### WR-08: `scanDef`'s comment counted five column lists; there are seven

**Files modified:** `internal/field/store.go`, `internal/field/store_test.go`
**Commit:** `2c62d1b`

Both comments corrected — `store.go` now names all seven readers (`List`, `Sub`,
`OfBlockType`, `OfBlockTypes`, `OfSnippet`, `OfSnippets`, `Get`), and
`store_test.go:100` says nine SQL sites (seven `SELECT`s, the `INSERT`, the
`UPDATE`) instead of the old arithmetic.

Went beyond the review's comment-only fix, because this phase's own lesson is
about counts written against the tree and then trusted:
`TestSpaltenlistenSindAbschriften` extracts the column lists from `store.go`,
asserts there are seven, asserts they are byte-identical, and asserts the list
ends on `COALESCE(snippet_id, 0)` — the position `scanDef` scans last. A fifth
carrier makes it red, which is the point: the author has to walk past `scanDef`.

**Verified the gate bites:** swapping `block_type_id` and `snippet_id` in one
list makes it fail; restored afterwards.

**One thing worth reporting in the direction it went:** my first version of that
test counted 21 columns where `scanDef` scans 18, because it counted the commas
inside `COALESCE(parent_id, 0)`. That is the same species of counting error the
comment itself carried. I did not adjust the expected number to match — I fixed
the counter to track paren depth, and recorded the incident in the test's own
comment so the next reader knows why the depth is tracked.

## Skipped Issues

None. All nine in-scope findings were applied.

Two were adapted rather than applied literally, both recorded above:

- **WR-03** — the suggested `m.Notes` append targets a `Manifest` member that
  does not exist; the import report was used instead, and the "proper" fix
  (hoisting `translateOut`/`translateIn`) is explicitly left open and written
  down in `deferred-items.md` as a roadmap item rather than a comment.
- **WR-04** — `HandleMaintenance` was the option the review left open. It is
  filled, with the reason in the code, rather than left looking accidental.

## Notes for the verifier

- **Two fixes are semantic and deserve a human eye**, even though both carry a
  test that was red first: **CR-01** changes a unique index (`00048`), and
  **WR-07** adds a refusal to a write path (`ErrNoSnippet`/`ErrNoBlockType`). A
  passing suite proves neither is wrong; it proves nothing in the tree depended
  on the old behaviour.
- **`00048` is a real migration.** Any database already carrying two groups with
  a colliding sub-key under one snippet cannot roll it back — by design, and
  documented in the file. No shipped installation can be in that state, because
  the collision was impossible before `00048` existed.
- **`fillSnippets` now has 17 call sites**, up from 14. If a plan gate counts
  them, the direction is *up by three* and the cause is WR-04 — do not resolve a
  long count by removing one of the three routes.

---

_Fixed: 2026-09-06T14:05:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
