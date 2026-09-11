# What is still missing

As of August 2026, after the website's own block kinds; carried forward in
September 2026, after the field kinds. A working list, not a wish list: every
point says **what is missing**, **where it belongs** and **how big it is**. What
is deliberately not being built is right at the bottom — so that nobody proposes
it twice.

The comparison with Statamic, which most of these points come from, is in
`vergleich-statamic.md`. The one structural gap from back then is closed: a
website now decides its own content model — its own fields, groups, content
kinds, sections, conditions and block kinds. What is here is individual work, not
a change of construction.

---

## 1. Choice as a button row and as multiple choice — built

**Built:** September 2026, phase 7. Both halves are in place. `mehrfachauswahl`
is a kind of its own beside `auswahl` — one value per line in the same string, via
`field.SplitValues`/`JoinValues`, with the multiplicity in the form name
(`feld_<key>[]`). The button row is not a kind but the display mode
`darstellung` on `auswahl` itself, together with an explicit empty button and a
rule of its own, `.feld-schalter--knopfreihe`, without which every dependent field
below it would silently have stopped appearing.

**Was missing:** the field kind *Choice* (`field.KindChoice`) is always a dropdown.
For three possibilities side by side that is the wrong form, and "several of them"
did not exist at all.

**Where:** `internal/field/field.go` (**one** new kind `mehrfachauswahl` beside
`KindChoice`, plus a display mode `darstellung` on `KindChoice` itself — the button
row is not a kind of its own but the same choice in another form: same
possibilities, same stored value, same contract to the template),
`internal/field/render.go` (a multiple choice is a list, not a string — that is the
real decision), `cmd/holzcloud/templates/admin/field_input.html`.

**Size:** the button row is an afternoon. The multiple choice is more: it is the
first field value that is not a single string, so `field.Data` needs either a
second storage path or an encoding you commit to. Proposal: one line per value in
the same string, the way `SplitChoices` already reads the possibilities.

## 2. Labels as a field kind — built

**Built:** September 2026, phase 7. The kind is called `schlagwort`, the picker is
the one copied from `KindRef`, and resolution goes through a `TermLookup` beside
`Links.Page`. What is stored is the slug, what is printed is the name as it
currently stands — so a rename changes every page without a single page being
touched. Inside a block the kind is excluded, for exactly the same reason as the
reference.

**Was missing:** labels exist on every page (`internal/term`), but you could not
create a field "Variety" that picks from them.

**Where:** a new kind `KindTerm` in `internal/field`, a chooser like the one for
`KindRef` (see `refPages` in `internal/admin/page_fields.go`), resolution in
`internal/field/render.go` through a `TermLookup` beside `Links.Page`.

**Size:** a day. The pattern is complete at the reference; it has to be copied
once.

## 3. Snippets can only do text — built

**Built:** September 2026, phase 8. Exactly the way this entry proposed, which
is worth saying because the entry was written before anybody knew: the fields
come from `page_field_defs` with a `snippet_id` column beside them
(`00047_snippet_fields.sql`), and there is no third field table. A snippet now
carries every kind a page carries — a telephone number with validation, a
picture, a number, a group.

**What the entry did not foresee.** `snippet_id` collided with the partial
unique index that guards a top-level field's key, so 00047 had to swap the
index rather than only add a column; and a snippet's own fields have to reach
the theme through *two* views that must agree — `SnippetFields` (every defined
field, filled or not) and `SnippetList` (only the filled ones), which is the
same asymmetry a page has and the same one a fixture can get backwards.

**Also settled here, and it is a real limit:** a picture, a reference or a label
inside a snippet value travels through the archive **untranslated** — the value
is a number, and a number means nothing on the machine the archive lands on.
Recorded as `V2-18`, not fixed.

## 4. CSV import — built

**Built:** September 2026, phase 9. It became two packages rather than one:
`internal/csv` reads the bytes (delimiter, encoding, quoting, the caps) and
`internal/csvimport` decides what each row means. The screen is a four-step
wizard of its own under `/admin/csv-import/{token}`, not a button on the website
list.

**"A day. Mapping column to field is the whole job."** That was wrong, and the
way it was wrong is the useful part. Mapping was the small half. The large half
was everything that has to be true of a run over five thousand rows a person
cannot check by hand: a dry run whose verdict per row is the same verdict the
write will reach (they are one function, because two would drift); a cap on
every dimension a file can grow in; a term pre-pass that does not hold the one
write connection for the length of the file; and — the one that took longest to
get right — the rule that **a blank cell says nothing**, because a table cannot
express the difference between "this is empty now" and "this file says nothing
about this". Reading that difference is how forty blank cells demote forty live
pages to drafts with no line in the report.

**Still not expressible from a CSV update:** clearing a value. That is done on
the page form, one page at a time, where the person doing it can see what they
are emptying.

## 5. Static export

**Missing:** a website as plain HTML files.

**Where:** a new command beside the others in `runCLI`, running the public handler
against a directory.

**Size:** a day for pages, archive, feed, sitemap and media. **Think first:** it is
a second mode of operation beside the one that works — it cannot do forms, search
or protected pages. If at all, then as an explicitly reduced output.

## 6. Field types missing individually — built

**Built:** September 2026, phase 7. All three, along with the columns they need
(migration `00046`). The hour per kind was right for the building; the second hour
per kind went to the contracts — `TEMPLATE-SPEC.md`, `SampleData`, `MinimalData`
and the catalogues — and that was planned for.

**Were missing**, small, an hour each, all in `internal/field`:

- `zeit` — a time of day. Exists only in scheduling.
- `bereich` — a number between two bounds, as a bounded number field
  (`<input type="number" min max step>`). Not a slider: a slider's chosen number
  appears nowhere as text, and the readout that would make it legible needs
  JavaScript — exactly the pattern `internal/tmplmgr/script.go` refuses.
- `code` — a text field without Markdown, in a fixed-width face.

---

## What is deliberately not being built

So that it does not come round again:

- **A command palette (⌘K) and passkeys** — both need JavaScript. The gain does
  not outweigh the exception.
- **YouTube and Vimeo embedding** — the rule "nothing from third parties at
  runtime" is the reason this CMS does without a cookie banner. An embed code
  costs exactly that. An MP4 of your own is available as a block.
- **GraphQL, OAuth, git automation** — each of them is a second mode of operation
  beside the one that works.
- **An HTML template per block kind** — that would be a templating language in a
  text box, and therefore a way to bring a `<script>` onto a page through the
  front door. The whole program rests on the promise that no such way exists.
  Where a CSS class is not enough, the long text field is the way out: it goes
  through the Markdown renderer and the same sanitising as any other text.
- **A reference inside a block** — a reference survives a rename; a block is turned
  into HTML once and for all when the page is saved and could not keep that
  promise. The link does the same work and says what it is.

---

## When carrying on

- **Migrations** run to `00048`. For a new migration that changes an existing
  table, read `internal/db/migrations/00029` and `00031` first: a CHECK constraint
  at the head of a table can only be relaxed in SQLite by rebuilding the table
  completely, and `pages` has foreign-key children. An index of your own, on the
  other hand — like the uniqueness of a field key — is a swap of two lines.
- **After every change to text:** `go run ./tools/i18n` shows what is missing in
  the five languages, `-write` creates the keys, `-schweiz` rebuilds `de-CH.json`.
  The run has to say "0 open, 0 orphaned".
- **Dependabot bumps:** one after another, with `go build ./...`, `go vet ./...`
  and `go test ./...` in between. The conflict in `go.mod` is the normal case as
  soon as two PRs start from the same version — keep both bumps, `go mod tidy`,
  done.

  For `modernc.org/sqlite` additionally a run against a real database file; that
  is the one component where a silent difference would be expensive. Build the
  binary, start it against an empty data directory and check that all migrations
  run through and the pragmas are in place:

  ```
  HOLZCLOUD_DATA_DIR=/tmp/dbtest HOLZCLOUD_PORT=18099 ./holzcloud
  ```

  Expected: `journal_mode=wal`, `busy_timeout=5000`, `foreign_keys=1`,
  `synchronous=1`, plus `PRAGMA foreign_key_check` with no rows and
  `PRAGMA integrity_check` equal to `ok`. Checked at 1.57.0: 44 migrations, 48
  tables, all clean.
- **When raising the Go version:** `go run ./tools/wasm` builds the six guest
  modules, and the tool pins the toolchain itself — `goToolchain` as a constant in
  `tools/wasm/main.go`, set as `GOTOOLCHAIN` during the build so that a run here
  produces the same bytes as one on the runner. That pin has a floor: it must never
  be below the `go` directive in the root `go.mod`. The test guest
  `internal/plugin/testdata/echo` lives in the root module and cannot carry a
  `toolchain` line of its own, and the go command refuses to load a module
  demanding a newer version than the running chain. Raising from `go 1.26.6`
  therefore means: raise the pin as well, rebuild all six guests — and the rebuild
  belongs in a commit of its own containing nothing but the build artefacts,
  otherwise it buries the actual change and `git log -S` no longer finds it.
  Without this paragraph, the person who notices is whoever merges the next bump
  and watches all six targets go red at once.
- **Checking happens in the browser.** This week's bugs — a post that became a page
  when an image was inserted; blocks missing from the bundle; menus colliding on
  import — were found by none of the tests but by a run through the running
  application.
