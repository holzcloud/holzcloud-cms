---
phase: quick-260903-eyv
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  # Stable scope — authorised and known to exist today. Line numbers anywhere in
  # this plan are from the planner's reading on 2026-09-03 and are hints only;
  # the executor reads the file and takes what is actually there.
  - tools/mkbundle/main.go
  - tools/mkbundle/pack_test.go
  - internal/bundle/format.go
  - sites/beispiel/holzcloud.json
  - internal/public/formular_e2e_test.go
autonomous: true
# QF-01 and QF-02 are local to this quick task: the two defects recorded on
# 2026-09-03, one in .planning/quick/260903-ceo-.../deferred-items.md, one
# measured in the same session. Neither is an ID in .planning/REQUIREMENTS.md.
requirements: [QF-01, QF-02]

estimate:
  tokens: 50000
  raw_tokens: 50000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "A bundle whose term name is not identical to its own slug packs: `go run ./tools/mkbundle sites/beispiel` exits 0 while the example carries a term named `Laufräder` under the slug `laufraeder`"
    - "A page that spells a term as a slug where slug and name differ is refused by mkbundle, with a message that names the spelling to use instead of only saying the entry is missing"
    - "Export, import and mkbundle agree that `page.terms` holds names: export.go still writes `t.Name`, import.go is untouched, and mkbundle's lookup set is keyed by the name"
    - "No bundle already exported changes meaning — the wire format is unchanged, and internal/bundle/import.go carries no diff"
    - "`go test -count=400 -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff ./internal/public/` passes, and reports a real run rather than a skip"
    - "The failing-first evidence exists for both defects: the mkbundle regression test fails before the validator is changed, and the form test fails at a high -count before the assertion is scoped"
    - "Every negative whole-document assertion in internal/public/formular_e2e_test.go has been surveyed and each one that could match a random value is scoped; the survey result is written into the summary even where nothing had to change"
    - "`go build ./...`, `go vet ./...` and `go test ./...` are green, 0 FAIL"
  artifacts:
    - "tools/mkbundle/main.go — checkReferences resolves page terms against the declared names"
    - "tools/mkbundle/pack_test.go — a regression test for a manifest whose term name differs from its slug, one that pins the slug-spelling error, and one that keeps the example carrying that case"
    - "internal/bundle/format.go — the decision written down where the format is defined: page.terms holds names"
    - "sites/beispiel/holzcloud.json — a terms list and at least one page carrying a label, the name/slug distinction included on purpose"
    - "internal/public/formular_e2e_test.go — the composed-form assertion scoped to where the prefix can actually appear"
  key_links:
    - "internal/bundle/export.go (writes t.Name into page.terms) -> internal/bundle/import.go (term.Parse over those names) -> tools/mkbundle checkReferences — the three places that must agree after this plan"
    - "internal/page/Slugify -> the slug the importer derives from a name -> the slug the example manifest declares; the two must match in the example"
    - "plugins/kontaktformular/eigenes.go feldPraefix -> the rendered field-name attribute -> the assertion in formular_e2e_test.go"
---

<objective>
Fix the two defects found and deliberately deferred on 2026-09-03.

QF-01 — mkbundle validates a page's terms against the term slugs while export writes,
and import reads, the term names. A bundle exported from a running site whose label is
spelled with a capital or an umlaut cannot be packed. This plan settles the spelling in
favour of the name, moves the validator to it, writes the decision down in the format,
and lets the example bundle finally carry the `terms` it was denied.

QF-02 — `TestMarkeMitUnbekanntemArgumentBleibtDerBetreff` searches the whole rendered
document for a two-character field prefix. The document also carries a base64url HMAC,
whose alphabet contains both characters, so the test fails about once in eighty runs
with nothing broken. This plan scopes the assertion to the place the prefix can actually
appear and surveys the file for the same shape elsewhere.

Purpose: a green CI that stays green, and a bundle format whose three handlers say the
same thing.
Output: five files touched, two regression tests that fail before their fix, one example
manifest that demonstrates the case which used to be unpackable.
</objective>

<execution_context>
@/Users/holz/.claude/gsd-core/workflows/execute-plan.md
@/Users/holz/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@/Users/holz/Projects/holzcloud-cms/CLAUDE.md
@/Users/holz/Projects/holzcloud-cms/internal/term/store.go
@/Users/holz/Projects/holzcloud-cms/internal/bundle/format.go
@/Users/holz/Projects/holzcloud-cms/tools/mkbundle/main.go
@/Users/holz/Projects/holzcloud-cms/tools/mkbundle/pack_test.go
@/Users/holz/Projects/holzcloud-cms/plugins/kontaktformular/eigenes.go
</context>

<mutable_scope>
The planner read the repository on 2026-09-03. Everything below is a finding, not an
instruction to trust blindly. Every line number is a hint; the executor confirms each
claim in the named file at execution time and widens scope if reality has moved.
Authorised scope: `tools/mkbundle/`, `internal/bundle/`,
`internal/public/formular_e2e_test.go`, `sites/beispiel/`.

**Finding A — the three places, as they stood.**
`internal/bundle/export.go` (~256) appends `t.Name` to a page's terms and
`exportTerms` (~312) writes the pair `{Slug, Name}` into the top-level list.
`internal/bundle/import.go` (~547) reads a page's terms back through
`term.Parse(strings.Join(p.Terms, ", "))` into `Terms.SetForPage`.
`tools/mkbundle/main.go` `checkReferences` (~166) builds its lookup set from the slug
field of the top-level list and compares a page's entries against it. Only the third
place uses the slug. Confirm all three before changing anything.

**Finding B — the slug is the database identity; the name is not structurally unique.**
`internal/db/migrations/00015_terms.sql` declares `UNIQUE (website_id, slug)` on
`terms`, and nothing on `name`. `term.Store.SetForPage` and `SetForProduct` derive the
slug from the name with `page.Slugify` and insert `ON CONFLICT (website_id, slug) DO
NOTHING`, so on that path a name always lands on exactly one row. The one way two rows
can end up sharing a name is `term.Store.Rename`, which changes the name and leaves the
slug alone deliberately, without a collision check — `internal/admin/term.go`
`HandleTermRename` only refuses an empty name. **So a name is not guaranteed unique
within a website.** D-01 explains why the direction survives that anyway; the executor
re-reads `internal/term/store.go` and the migration and stops the task if either has
changed since.

**Finding C — the importer never reads the declared slug.** The only thing
`internal/bundle/import.go` does with `m.Terms` is `report.Terms = len(m.Terms)` (~581).
Terms are created as a side effect of the names carried by pages. Two consequences the
executor should confirm rather than assume: a declared term that no page carries is not
created on import, and a renamed term's slug does not survive a round trip because the
importer re-derives it from the name. Both are pre-existing, both are out of scope here,
and both belong in the summary as known limits — not in the diff.

**Finding D — where the two characters come from in the form test.**
`plugins/kontaktformular/eigenes.go` declares `const feldPraefix = "f_"` and emits it
only as a form field name — `name := feldPraefix + fe.Kennung`, written out as a quoted
`name=` attribute. `plugins/kontaktformular/main.go` `zeichnen` and `zeichnenEigen` both
write a hidden `gestellt` field whose value is `base64.RawURLEncoding` of an HMAC
(~344); that alphabet contains `f` and `_`, which is the whole flake. A quoted attribute
prefix cannot occur inside a base64url value, because base64url has no quote character.

**Finding E — the two renderers differ in more than the field prefix.** `zeichnenEigen`
opens with `class="contact-form contact-form--<kennung>"` and always writes the hidden
`formular` field; `zeichnen` writes neither, and its opening tag carries the bare
`contact-form` class. This matters because the fixture `formularAufbau` defines no
composed form, so a regression that wrongly took the composed branch could render a form
with zero fields and therefore zero prefixed names — the field prefix alone is not a
sufficient discriminator. Confirm both renderers before writing the assertion.

**Finding F — the form test skips itself when the plugin is not built.**
`formularAufbau` calls `t.Skipf` when `plugins/kontaktformular/plugin.wasm` is missing.
The file is committed and was present on 2026-09-03 (4.7 MB). A skipped test passes
vacuously, so the 400-run gate proves nothing unless the run is a real one. See the
precondition on task 3.

**Finding G — the example carries no terms today.** `sites/beispiel/holzcloud.json` has
the keys `site`, `pages`, `menus`, `snippets`, `media` and nothing else; a grep for
`"terms"` returns nothing. `readManifest` decodes with `DisallowUnknownFields`, so the
key set remains exactly the json tags in `internal/bundle/format.go` — read them there,
never from this plan.
</mutable_scope>

<decisions>
Four questions the briefing left open. Each is answered here so the executor does not
reopen them, and each answer goes into the summary.

**D-01 — the name wins, and the fix lands in the validator alone.**
The briefing asked for the direction to be confirmed rather than assumed, and the
confirmation is Finding B: the slug, not the name, is what the database holds unique. On
its own that argues for the slug. It does not win, for three reasons that outrank it.

First, the bundle format has already settled it on both sides that matter. Export writes
the name into `page.terms` and import reads a name — and import derives the slug from
that name, so the declared slug is not load-bearing anywhere in the reading path
(Finding C). The name is the identity *of the format*, whatever the identity of the table
happens to be. Only the validator, the youngest of the three, disagreed.

Second, a manifest exists to be read and repaired by hand. `Möbel` is what an editor
typed and what the archive page shows; `moebel` is machine spelling. Turning the readable
side into the machine side to satisfy a check is the tail wagging the dog.

Third, switching export and import to slugs would change the wire format and break every
bundle already handed to a customer — for a defect that costs one line in a dev tool.

The non-uniqueness is not thereby dismissed: two terms that share a name after a rename
merge into one on import. That is a property of the format as it stands today, it
predates this task, no validator can repair it, and it is recorded as a known limit in
the summary rather than quietly fixed here.

**D-02 — compatibility: nothing on the reading side changes, and it must not.**
The wire format does not move. `page.terms` held names yesterday and holds names
tomorrow; `internal/bundle/import.go` carries no diff, so every archive already in the
wild imports exactly as before. The reading side is deliberately *not* taught to accept
both spellings: a slug accepted there would create a term whose name really is `moebel`
— lowercase, hyphenated — written into the customer's database, which is precisely the
wrong spelling this task exists to prevent. Accepting both would also make the two
spellings drift apart permanently, since nothing would ever complain again.

One class of artefact does change status: a manifest hand-written to satisfy the buggy
check, with `page.terms` spelled as slugs where slug and name differ. Such a file packs
today and imports as a term named after its slug — already wrong, just silently. After
this fix mkbundle refuses it and names the spelling to use. That refusal happens at build
time, never at run time, and nothing in this repository is such a file (Finding G).
Where a page's entry matches no name but does match a declared slug, the error says so
explicitly, so the person who wrote it is told what to write instead of being told the
entry is missing while they can see it sitting there.

The match is exact, not case-folded. Machine-written bundles always match exactly,
because export writes the same string into both places; a hand-written manifest whose
page says `möbel` while its terms list says `Möbel` would import a term named `möbel` and
leave the declaration a lie, so being told about it is the useful outcome.

**D-03 — the example gains terms, and gains the hard case on purpose.**
It was left out only because of this bug, and with the bug gone the example is where the
format is shown. It gets at least one label whose name is not its own slug —
`Laufräder` under `laufraeder`, since `internal/page/slug.go` folds `ä` to `ae` — because
that is exactly the shape that could not be packed before. The example then does double
duty: `TestTheExampleStillPacks` packs it on every `go test ./...`, so a future revert of
the validator reddens the suite through the shipped example and not only through a
synthetic fixture.

One rule binds the example that does **not** bind mkbundle: every declared
`terms[].slug` must equal what `page.Slugify` derives from the name, because that is the
slug the importer will actually create (Finding C). An example declaring a slug the
importer would not produce would teach a fiction. A genuine export from a renamed term
legitimately violates that equality, which is why mkbundle must not turn it into an
error — the example holds itself to a standard the validator cannot impose.

**D-04 — the form test asserts the shape of the form, not the presence of two
characters.** The test's own name says the marker with an unknown argument stays a
prefilled subject. What it means by its second assertion is that the composed renderer
did not run. Two characters anywhere in the document is a proxy for that, and a poor one
in both directions: it collides with a random HMAC (Finding D), and it would miss a
composed form rendered with no fields (Finding E). The replacement asserts the classic
form's own opening tag is there and the composed renderer's fingerprints are not — the
`contact-form--` class, the hidden `formular` field, and the field prefix in the only
position it can occupy, a quoted `name=` attribute.
</decisions>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: page terms resolve by name, end to end, with a test that fails first</name>
  <files>tools/mkbundle/pack_test.go, tools/mkbundle/main.go, internal/bundle/format.go</files>
  <read_first>
    Read, in this order, and stop the task if any of them contradicts Finding A, B or C:
    `internal/term/store.go` (what a term is, and what `Rename` does to the name),
    `internal/db/migrations/00015_terms.sql` (which column is unique),
    `internal/bundle/export.go` around the page loop and `exportTerms`,
    `internal/bundle/import.go` around the page terms and the report,
    `tools/mkbundle/main.go` `checkReferences` and `join`,
    `tools/mkbundle/pack_test.go` in full — its language is English and every test in it
    carries a comment saying why it exists; match both.
  </read_first>
  <behavior>
    Tests to add to `tools/mkbundle/pack_test.go`, calling `checkReferences` directly on
    a `*bundle.Manifest` built in the test (the file is `package main`, so no temp
    directory and no packing is needed for these):
    - A manifest whose terms list declares a label whose name is not its own slug
      (`Laufräder` / `laufraeder`), with a page whose terms name that label the way
      export writes it, produces no error. This is the regression guard for QF-01 and
      MUST fail before the validator is changed — record the failure text in the summary.
    - The same manifest with the page spelling the label as its slug produces an error,
      and the error text names the spelling that should have been used, so the message
      is useful rather than merely correct. Assert on the substring that carries the
      correct spelling, not on the whole sentence.
    - A page naming a label that is in neither the names nor the slugs still produces an
      error. This keeps the check a check: it would be trivially satisfiable if the fix
      widened the set instead of moving it.
  </behavior>
  <action>
    Write the three tests first and watch the first two fail, then change
    `checkReferences` so the lookup set is keyed by the declared name field rather than
    the declared slug field, per D-01. Build a second map from slug to name alongside it
    and consult it only on the failure path, so a page entry that turns out to be a slug
    is reported as a spelling to correct rather than as a missing entry, per D-02. Keep
    the existing failure wording for an entry that matches nothing at all; keep the
    behaviour of `join` (nil when there are no problems, deduplicated and sorted
    otherwise) untouched. Do not add a check that the declared slug agrees with what
    Slugify would derive — D-03 explains why that would refuse legitimate exports.
    Update the doc comment above `checkReferences` so it states which spelling a page's
    entries carry; the comment currently talks about references by name in general.
    Then write the decision down where the format lives: in `internal/bundle/format.go`,
    extend the comment on the page field that holds the labels to say it carries the
    names as they are shown, matching the name field of the declared entries, and extend
    the comment on the declared-entry struct to say what the importer does with the
    declared slug today (Finding C, confirmed by reading, not copied from this plan).
    Two or three sentences each, in the voice of the surrounding comments — this is a
    format decision, and the next person meets it there rather than in a plan file.
    Touch no file under `internal/bundle/` other than `format.go`: export and import are
    already correct and a diff in either would break bundles already exported.
  </action>
  <verify>
    <automated>go -C /Users/holz/Projects/holzcloud-cms test ./tools/mkbundle/ ./internal/bundle/ && go -C /Users/holz/Projects/holzcloud-cms vet ./tools/mkbundle/ ./internal/bundle/ && grep -v '^\s*//' /Users/holz/Projects/holzcloud-cms/tools/mkbundle/main.go | grep -c 'terms\[t.Name\]' && test 0 -eq "$(grep -v '^\s*//' /Users/holz/Projects/holzcloud-cms/tools/mkbundle/main.go | grep -c 'terms\[t.Slug\]')" && test -z "$(git -C /Users/holz/Projects/holzcloud-cms diff --name-only -- internal/bundle/export.go internal/bundle/import.go)"</automated>
  </verify>
  <done>
    The two new guards failed before the change and pass after it; a page term that
    matches nothing still fails; the lookup set is keyed by the name and no longer by the
    slug; `internal/bundle/export.go` and `internal/bundle/import.go` are unmodified; the
    format file states which spelling a page's terms carry.
  </done>
  <reversibility rating="reversible">
    A validator in a dev tool and two doc comments. Nothing is written to a database and
    no archive already built is affected.
  </reversibility>
</task>

<task type="auto">
  <name>Task 2: the example bundle finally carries its labels</name>
  <files>sites/beispiel/holzcloud.json, tools/mkbundle/pack_test.go</files>
  <read_first>
    `internal/bundle/format.go` for the json tags of the declared-entry struct and of the
    page field — the accepted key set is what is written there, never what is written
    here. `internal/page/slug.go` for how `Slugify` folds the umlaut. The whole of
    `sites/beispiel/holzcloud.json`, including the page bodies: a label has to be
    justified by what the page actually says.
  </read_first>
  <action>
    Add a top-level terms list to the example and attach labels to the pages that earn
    them, per D-03. Use two labels at most. One of them must be a name that is not its
    own slug — `Laufräder` under `laufraeder` is the planner's suggestion and the page
    about the workshop talks about wheels being rebuilt, but the executor picks from what
    the pages really say. Avoid a label whose slug collides with an existing page slug in
    the example, so a reader is not left wondering how `/service` and the archive of the
    same word relate. Every declared slug must equal what `Slugify` derives from its
    name — verify each one by reading `internal/page/slug.go`, not by eye. Write the page
    entries the way export writes them: the name as shown. Keep the file's existing
    formatting — two-space indent, key order matching the other pages, no trailing
    whitespace. Add no key that is not a json tag in the format file; `readManifest`
    refuses unknown keys and the example is the file people copy.
    Do this task after task 1 and not before: the example would fail to pack against the
    old validator, which is the point, but it would also redden the suite for a reason
    that has nothing to do with the example.
    Then pin what the example is now for, with one more test in
    `tools/mkbundle/pack_test.go` beside the two already there: read the example manifest
    through `readManifest` and assert it declares at least one label whose name is not its
    own slug, that at least one page carries a label, and that every label a page carries
    is declared. Written in Go rather than as a shell one-liner because this repository
    has no second language in it, and because the guard has to outlive this task: without
    it the next person tidying the example could drop the umlaut and quietly remove the
    only shipped case that exercises the fix. Comment it the way the neighbouring tests
    are commented — with the reason it exists.
  </action>
  <verify>
    <automated>go -C /Users/holz/Projects/holzcloud-cms run ./tools/mkbundle sites/beispiel && go -C /Users/holz/Projects/holzcloud-cms test -v ./tools/mkbundle/ && go -C /Users/holz/Projects/holzcloud-cms vet ./tools/mkbundle/ && test -z "$(git -C /Users/holz/Projects/holzcloud-cms status --porcelain sites/beispiel.zip)"</automated>
  </verify>
  <done>
    The example declares at least one label whose name is not its slug, at least one page
    carries a label spelled as a name, every page label has a declared entry, the built
    archive is written without a complaint, and `sites/beispiel.zip` stays out of
    `git status` (it is ignored).
  </done>
</task>

<task type="auto">
  <name>Task 3: the form test asserts the form, not two characters in a random value</name>
  <files>internal/public/formular_e2e_test.go</files>
  <precondition>
    `plugins/kontaktformular/plugin.wasm` exists and loads — otherwise `formularAufbau`
    skips and the 400-run gate proves nothing. Confirm the file is present before
    starting, and confirm at the end that the gate run reports a real pass rather than a
    skip.
  </precondition>
  <read_first>
    `plugins/kontaktformular/eigenes.go` (how the composed renderer opens its form, what
    it writes as a hidden field, and how the field prefix reaches the markup) and
    `plugins/kontaktformular/main.go` (the classic renderer, the hidden `gestellt` field
    and where its value comes from). Then the whole of
    `internal/public/formular_e2e_test.go` — the survey below needs the whole file, not
    the one test.
  </read_first>
  <action>
    First reproduce: run the gate command from `<verification>` unchanged and record the
    failure count in the summary. A single green run proves nothing here.
    Then rewrite the second assertion of `TestMarkeMitUnbekanntemArgumentBleibtDerBetreff`
    per D-04. Assert positively that the classic renderer ran — its opening tag with the
    bare class, exactly as `zeichnen` writes it — and negatively that none of the
    composed renderer's fingerprints are present: the modified class it opens with, the
    hidden field that says which composed form was sent, and the field prefix in the only
    position it can occupy, inside a quoted `name=` attribute. Take all four strings from
    the two renderer files, not from this plan. Keep the existing subject assertion.
    Leave the German comment above the test and its failure messages in the voice they
    are written in; extend the failure message so it says what was found rather than
    dumping the document alone, if the surrounding style allows it.
    Then survey the file: list every `strings.Contains` in it and judge each negative one
    against the question "could this substring occur by chance in a value the test does
    not control". The planner's reading found the others to be markup fragments and
    script-ish literals that cannot collide, so the expected outcome of the survey is that
    nothing else needs changing — **do not churn assertions that are already immune.**
    Record the survey and its outcome in the summary either way, including the case where
    the answer is "nothing else".
  </action>
  <verify>
    <automated>go -C /Users/holz/Projects/holzcloud-cms test -count=400 -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff ./internal/public/ && test 0 -eq "$(grep -c 'seite, "f_"' /Users/holz/Projects/holzcloud-cms/internal/public/formular_e2e_test.go)" && go -C /Users/holz/Projects/holzcloud-cms test -v -count=1 -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff ./internal/public/ | grep -q -- '--- PASS'</automated>
  </verify>
  <done>
    The test failed at a high `-count` before the change and passes 400 consecutive runs
    after it; the run is a pass and not a skip; the whole-document search for the two
    characters is gone; the assertion would still fail if the composed renderer ran, with
    or without fields; the file has been surveyed and the outcome is in the summary.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| hand-written or exported manifest -> mkbundle -> zip -> admin import | Untrusted-ish content crosses into a customer's database. mkbundle is the only gate between a manifest and an import that cannot be undone. |
| plugin-rendered HTML -> public page | Unchanged by this plan; the test that observes it is what changes. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-eyv-01 | Tampering | tools/mkbundle checkReferences | medium | mitigate | The check moves from the slug to the name; it must stay a check. Task 1 keeps a guard asserting that a label matching neither names nor slugs is still refused, so the fix cannot degrade into accepting everything. |
| T-eyv-02 | Tampering | internal/bundle/import.go | high | mitigate | The reading side is not widened to accept both spellings (D-02); a slug accepted there would write a machine-spelled label into a customer's database. Task 1's verify asserts import.go carries no diff. |
| T-eyv-03 | Information disclosure | sites/beispiel/holzcloud.json | low | accept | The example is public placeholder content about a fictional workshop; the labels added carry no real-world data. |
| T-eyv-04 | Denial of service | tools/mkbundle | low | accept | A refused pack is a build-time failure with a clear message and nothing to undo — the tool's stated preference. |
| T-eyv-SC | Tampering | package-manager installs | n/a | n/a | No dependency is added. `go.mod` and `go.sum` carry no diff; nothing is fetched at build or run time, per CLAUDE.md. Asserted in `<verification>`. |
</threat_model>

<verification>
Run from the repository root (or with `go -C /Users/holz/Projects/holzcloud-cms` as
written). Package counts are a hint, not a gate — `go list ./...` reported 41 packages on
2026-09-03; what counts is 0 FAIL.

```bash
# 1 — the baseline, green
go -C /Users/holz/Projects/holzcloud-cms build ./...
go -C /Users/holz/Projects/holzcloud-cms vet ./...
go -C /Users/holz/Projects/holzcloud-cms test ./... 2>&1 | tail -60   # 0 FAIL

# 2 — the discriminating gate for QF-02. Run it BEFORE task 3's edit and record the
#     failure count; it fails today at this count and at -count=60. Allow several
#     minutes: every iteration instantiates the plugin module afresh.
go -C /Users/holz/Projects/holzcloud-cms test -count=400 \
  -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff ./internal/public/

# 3 — the run is real, not a skip
go -C /Users/holz/Projects/holzcloud-cms test -v -count=1 \
  -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff ./internal/public/ | grep -- '--- '

# 4 — the discriminating gate for QF-01. Run the new guards BEFORE the validator moves
#     and record the failure text; a fix whose test never failed has proved nothing.
go -C /Users/holz/Projects/holzcloud-cms test -v ./tools/mkbundle/

# 5 — the example, which is now the shipped regression guard
go -C /Users/holz/Projects/holzcloud-cms run ./tools/mkbundle sites/beispiel
git -C /Users/holz/Projects/holzcloud-cms status --porcelain sites/   # only the manifest

# 6 — the reading side is untouched, and no dependency moved
git -C /Users/holz/Projects/holzcloud-cms diff --name-only -- \
  internal/bundle/export.go internal/bundle/import.go go.mod go.sum   # empty

# 7 — translations, confirmatory: no app-facing string moves in this plan, so each of
#     en/es/fr/it must still report 0 offen, 0 verwaist
go -C /Users/holz/Projects/holzcloud-cms run ./tools/i18n
```
</verification>

<success_criteria>
- `go build ./...`, `go vet ./...`, `go test ./...` green, 0 FAIL.
- Both defects have failing-first evidence recorded in the summary: the mkbundle guards
  and the form test at a high `-count`.
- `go test -count=400 -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff ./internal/public/`
  passes, and the single-run check shows a pass rather than a skip.
- `go run ./tools/mkbundle sites/beispiel` exits 0 with the example carrying a label
  whose name is not its own slug.
- `internal/bundle/export.go`, `internal/bundle/import.go`, `go.mod` and `go.sum` carry
  no diff.
- The spelling decision and its compatibility consequences (D-01, D-02) are written into
  `internal/bundle/format.go`, not only into the summary.
- The survey of `internal/public/formular_e2e_test.go` is reported, including the case
  where nothing further needed changing.
- The known limits confirmed while reading — a declared label no page carries is not
  created on import, a renamed label's slug does not survive a round trip, two labels
  sharing a name merge on import — are recorded in the summary as limits, not fixed.
</success_criteria>

<output>
Create `.planning/quick/260903-eyv-zwei-abgelegte-fehler/260903-eyv-SUMMARY.md` when done.
Commit the work as soon as the plan is finished — the two defects are one changeset.
</output>
