---
phase: 09-csv-import
fixed_at: 2026-09-06T20:30:00Z
review_path: .planning/phases/09-csv-import/09-REVIEW.md
iteration: 1
scope: critical_warning
findings_in_scope: 9
fixed: 9
skipped: 0
status: all_fixed
---

# Phase 9: Code Review Fix Report

**Fixed at:** 2026-09-06T20:30:00Z
**Source review:** `.planning/phases/09-csv-import/09-REVIEW.md`
**Iteration:** 1
**Scope:** CR-01 and WR-01 … WR-08. Info findings out of scope; IN-06 belongs to
plan 09-06 and was left alone. IN-01 was touched only where WR-08 made its dead
branch live — said below.

**Summary**

| | |
|---|---|
| Findings in scope | 9 |
| Fixed | 9 |
| Skipped | 0 |
| Commits | 9, one per finding |
| Files touched | 14, all of them phase 9's own |

Nothing outside phase 9 was staged. No `git add -A`, no `git commit -a`; every
commit named its files and was checked with `git show --stat` afterwards. The
developer's parallel work on `README.md`, `docs/`, `.github/`, `CONTRIBUTING.md`
and `SECURITY.md` was not touched — the full list of files across all nine
commits is at the end.

---

## Fixed

### CR-01 — a blank cell says nothing about a slot, on the whole update arm

**Commit:** `65897e9`
**Files:** `internal/csvimport/row.go`, `internal/csvimport/row_test.go`

Fixed as the orchestrator directed and **wider than the review states**. The
review named the status branch and treated the body as "arguably the documented
semantics"; the field arm it did not name at all. All three had the identical
shape, so all three are one rule now:

```go
// before
mapped := func(t Target) bool {
    return m.ColumnFor(t.Kind, t.Key) >= 0 || strings.TrimSpace(m.Defaults[t.String()]) != ""
}
// after
stated := func(t Target) bool { return cellFor(row, m, t) != "" }
```

`cellFor` already falls a blank cell back to the target's default, so "the
operator typed a default" counts as stated and "the file left the cell blank"
does not. `update()` takes the row now; `WriteRow` passes it.

The reversed decision is recorded rather than deleted. The old comment argued
that a blank cell clears its slot, "which is the difference between 'the file
says this is empty now' and 'the file says nothing about this'". That
distinction cannot be drawn from a CSV — a blank cell is exactly as much one as
the other — so the new doc comment states the consequence plainly: **clearing a
value is not expressible from a CSV update.** Emptying is done on the page form,
one page at a time, by somebody who can see what they are emptying.

The create arm is untouched. `parseStatus("")` → `draft` is correct there and
documented; `parseStatus`'s own comment now says the substitution is the create
arm's alone.

**The proving test, literally.** `TestBlankCellSaysNothingOnTheUpdateArm`
(`row_test.go`). Before the fix — `git checkout -- internal/csvimport/row.go`,
test file untouched:

```
--- FAIL: TestBlankCellSaysNothingOnTheUpdateArm (0.08s)
    row_test.go:706: a blank Zustand cell demoted a live page to "draft" — that is the bug this test exists for
    row_test.go:709: a blank Text cell wiped the page's markdown, leaving ""
    row_test.go:712: a blank field cell emptied the slot, leaving "", want rot
FAIL
FAIL	github.com/holzcloud/holzcloud-cms/internal/csvimport	0.518s
```

After:

```
--- PASS: TestBlankCellSaysNothingOnTheUpdateArm (0.10s)
PASS
ok  	github.com/holzcloud/holzcloud-cms/internal/csvimport	0.564s
```

All three destructive paths fail before and pass after, on one fixture. The test
also asserts the other half of the rule — a default set on the mapping screen
**does** apply to status, body and field alike — and that the create arm still
gives a blank `Zustand` cell the status `draft`. `TestUnmappedColumnsAreLeftAlone`
and `TestExistingPageIsUpdatedOrSkipped` cover none of this and still pass.

### WR-01 — one word yields one key at every accent

**Commit:** `127629b`
**Files:** `internal/csvimport/mapping.go`, `internal/csvimport/mapping_test.go`

**The review's patch would have introduced the bug it removes, one function
along.** It proposes dropping the base letter inside `settleMarks` — but
`settleMarks` has two consumers with two answers. `foldHeader` hands its output
to `field.SlugifyKey`, which drops the precomposed `é` whole, so there the base
must go. `foldCell` hands its output to `page.Transliterate`, which writes `é`
out as `e`, so there the base must stay. The review says as much in prose
("`foldCell` does not have the problem") and then patches the shared function
anyway.

So the loop is now parametrised and reached through two named entry points,
`settleHeaderMarks` (`dropMarkedBase`) and `settleMarks` (`keepMarkedBase`), and
both sides are asserted:

- `TestFoldHeaderAgreesOnEveryOtherAccent` — five accents (é, ñ, ç, å, š), NFC
  against NFD, each folding to the key `SlugifyKey` derives from the label.
  Before the fix, all five: `foldHeader(NFC) = "caf", foldHeader(NFD) = "cafe" —
  two keys for one word`. After: pass.
- `TestFoldCellKeepsTheBaseLetter` — the other consumer, so the day somebody
  flips the flag a whole status column does not silently fall outside its own
  closed vocabulary.

### WR-02 — a row wider than its header is a code and two numbers

**Commit:** `bf56af7`
**Files:** `internal/csv/csv.go`, `internal/csv/csv_test.go`,
`internal/csvimport/verdict.go`, `internal/csvimport/row.go`,
`internal/csvimport/row_test.go`, `cmd/holzcloud/templates/admin/csv_reason.html`

`csv.Row` gained `HeaderWidth`, `Next` no longer builds the English fragment, and
`CheckRow` answers the new `ReasonRowTooWide` with both numbers as arguments.
`HeaderWidth == 0` means "not known" (a hand-built row), not "no columns" — the
test asserts that too, because reading it the other way would refuse every row of
every file. `TestCSVEveryReasonHasASentence` picked the new arm up on its own.

`ReasonRowUnreadable` keeps `encoding/csv`'s own `ParseError` text and the
comment now says why that exemption is a decision rather than an oversight. The
sibling `cell in column %s is %d bytes` message stays: `CheckRow` measures cells
itself and answers `ReasonCellTooLong` with the column and the size before it
looks at `Error` at all, so it is unreachable on screen — `Row.Error`'s doc
comment now records both, so the day the cell loop moves the reader has been
told.

Fails-before is compile-level for the new symbols; the behavioural before is
recorded in the two tests' doc comments, quoting the sentence the report used to
render.

### WR-03 — the comment now says what the report actually prints

**Commit:** `466b0ea`
**Files:** `internal/csvimport/verdict.go`, `internal/admin/csvimport_test.go`

Behaviour kept, comment corrected. An operator who cannot see what the store said
cannot tell a duplicate address from a full disk, and the screen is behind an
admin session; the defect was never the printing, it was a comment stating a
guarantee the code does not keep, which the next person adding a reason would
have written against.

**This is the one fix with no fails-before test, and the reason is structural:**
correcting a false claim about unchanged behaviour cannot fail before. What is
added instead is a gate, `TestCSVStoreErrorReachesTheReport`, which holds both
arms to printing `(index .Args 1)` — so dropping the argument now has to change
the comment in the same commit. It passes before and after; it is a pin, not a
proof, and is reported as such.

### WR-04 — the update arm reports an encode failure

**Commit:** `8bfc470`
**File:** `internal/csvimport/row.go`

`update()` returns `(page.PageUpdate, error)`; `WriteRow` turns a non-nil error
into `ReasonNotWritten` before `UpdatePage` is called, which is what the create
arm already did for the identical call.

**No test, and the reason is that the branch cannot be reached.** `field.Encode`
is `json.Marshal` over a `Data` holding only `map[string]string` and
`map[string][]map[string]string`; `json.Marshal` cannot fail on those. There is
no seam to drive it through from the test package and inventing one would be a
second creation path, which is what IMP-02 forbids. The fix is a compile-level
guarantee that the two arms answer the same failure the same way, and the doc
comment says the branch is close to unreachable so the next reader does not go
looking for the test.

### WR-05 — the staged row is claimed before the loop

**Commit:** `e81cd98`
**Files:** `internal/csvimport/store.go`, `internal/csvimport/store_test.go`,
`internal/admin/csvimport.go`, `internal/admin/csvimport_test.go`

`Store.Claim` as the review wrote it. `HandleCSVStart` claims before
`CreateWebsite` and before the loop; the loser gets the expiry screen, which
`staged()` and the write path now reach through one `csvExpired` helper.
`Store.Delete` stays for the path that is not the write — the wizard ending
because its target website was deleted under it. The traded-away property ("a
process that dies mid-write leaves the row intact") is named in `Claim`'s comment
rather than left implied.

Two tests, and the second one fails before:

- `TestClaimSucceedsExactlyOnce` — two callers, one row, one winner, and the
  token expired afterwards.
- `TestCSVTwoOverlappingCommitsImportOnce` — two goroutines released off one
  `WaitGroup`. With the delete back after the loop, reproducibly:
  `websites = 3, want 2 … 4 pages, want 2 — the file was imported more than
  once`. After: pass, and clean under `-race`.

(The goroutines do not use `serveAs`: it calls `t.Fatalf`, which may only be
called from the test's own goroutine.)

### WR-06 — the mapping screen shows a sample of a cell, not the cell

**Commit:** `06e676c`
**Files:** `internal/admin/csvimport.go`, `internal/admin/csvimport_test.go`

`csvSampleBytes = 200` and `csvSample`, cutting at the view boundary and nowhere
else, **on a rune boundary** — the review's snippet slices bytes and would put
replacement glyphs on the screen; it asks for the rune boundary in prose and does
not write it. Ellipsis appended so a cut cell does not read as a short one.

`TestCSVMappingCutsAnOversizedSample` before the fix:
`the whole oversized cell was written into the response` /
`the response is 18660 bytes for a 10000-byte cell — the sample is not bounded`.
After: pass, plus `utf8.ValidString` on a cell of multi-byte characters.

### WR-07 — stepping the sample row carries the mapping (and `target_N`)

**Commit:** `8844607`
**Files:** `cmd/holzcloud/templates/admin/csv_mapping.html`,
`internal/admin/csvimport.go`, `internal/admin/csvimport_test.go`

Built as `09-02-PLAN.md`'s recorded `wie-vorgeschlagen` decision describes: one
form, and the stepper is a submit button inside it carrying
`formmethod="GET" formaction="/admin/csv-import/{token}" name="row"`, the idiom
`page_form.html:136` and `:174` already use. `HandleCSVMapping` reads the
submitted mapping through `csvSubmittedMapping` and passes it as `chosen`, which
`csvMappingData` already supported.

`csvSubmittedMapping` returns **nil** when the request carries no mapping control
at all, and that nil is load-bearing: an empty mapping would override the
automatic match with nothing, so a screen reached from screen 1 would come back
blank.

This commit also carries the orchestrator's directive 3, because the two are one
change: recognising "a request carrying a mapping" is what turned the prefix into
a named constant, and the constant is where the rename lands.

- `ziel_<N>` → `target_<N>`, wire name, `id` and `<label for>` alike.
- `default_field:<key>` was checked and is already English (`default_` +
  `field:` + the key); it is unchanged.
- `neu` / `bestehend` / `uebergehen` / `aktualisieren` untouched — 00049's CHECK
  pins them and translating one compiles cleanly and fails at runtime.

`TestCSVSteppingKeepsTheMapping` before the fix: `the stepper is still a plain
anchor` / `no stepping submit for row 2` / `column 1 came back with the automatic
match instead of the operator's „nothing“` / `the default the operator typed was
thrown away by the step`. After: pass.

One existing test needed its detection updated, not its assertion:
`TestCSVSampleRowSteppingIsClamped` looked for the "previous" control by
`href="?row=`, which no longer exists. It now looks for the control's own
wording, symmetrically with its "next" check.

### WR-08 — the example file has a path for a website that does not exist yet

**Commit:** `3e54f87`
**Files:** `internal/admin/csvimport.go`,
`cmd/holzcloud/templates/admin/website_list.html`,
`internal/admin/csvimport_test.go`

A website **named** and not there is still 404; a website **not named at all** is
now the fixed-column case. `TestCSVExampleUnknownWebsiteIsNotFound` keeps the 404
for a nonexistent id and hands `""`, `"0"` and `"nichts"` to the new
`TestCSVExampleWithoutAWebsiteIsTheFixedColumns`.

Two departures from the review's suggested fix, both forced:

1. **The panel gets its own second form, not a second submit button.** The review
   proposes `name="website" value="0"` inside the existing form. That does not
   work: the `<select>` is also named `website` and is serialised earlier, so
   `r.FormValue("website")` would read the select's value and never the zero. A
   form carrying no `website` field at all is unambiguous. It renders even when
   there are no websites to list, which is the moment the finding is about.
2. **`csvExampleFilename("")` did not "already yield `website-vorlage.csv`".**
   The review says it does, citing IN-01. It did not: `page.Slugify` returns
   `"untitled"` for a string it can make nothing of (`page/slug.go:118-120`), so
   the `if slug == ""` guard could never fire and the download would have arrived
   as `untitled-vorlage.csv` — a name that reads like a fault. The empty name is
   now answered before `Slugify`. This is IN-01's dead branch becoming live,
   which is exactly what IN-01 predicted would happen if WR-08 were taken; it is
   not scope creep, WR-08 is wrong without it.

**Directive 4** landed here too: `HandleCSVExample` now carries the warning
comment. Both halves were verified against the tree rather than taken on trust —
`auth.RequireWebsiteAccess` reads `websiteIDInPath(r.URL.Path)`
(`middleware.go:102`) and `NewWebsiteAccessLookup` returns `true` unconditionally
for `user.RoleAdmin` (`handler.go:167-169`), with `user_websites` consulted only
below that. The comment names the middleware, the admin short-circuit and the
fact that four other routes share the shape and are deliberately not touched.

---

## Where the review turned out to be wrong

Four things, none of them fatal to the finding they sit in.

1. **WR-01's patch would have reintroduced the bug it fixes.** Applied literally
   to `settleMarks`, it breaks `foldCell`: the decomposed `Café` would fold to
   `caf` while the composed one folds to `cafe`. The review's own prose says
   `foldCell` is fine; its diff does not respect that. Fixed by scoping the drop
   to the header fold and asserting both consumers.

2. **WR-08's filename claim is false.** `csvExampleFilename("")` yielded
   `untitled-vorlage.csv`, not `website-vorlage.csv`. See above.

3. **The summary's authorisation paragraph is wrong on one point.** It says "this
   codebase has no per-user website scoping at all". It has: `user_websites`,
   consulted by `NewWebsiteAccessLookup` for every non-admin role. What makes
   `GET /admin/csv-vorlage` harmless is the admin short-circuit plus
   `requireAdmin` on the route — not the absence of scoping. The distinction
   matters because the review's version reads as "there is nothing to escape",
   and directive 4's comment says the accurate thing instead.

4. **The `BeginTx` count is 15, not 14.** Non-test files tree-wide:
   `cmd/holzcloud/cli.go` plus 14 under `internal/`. The review's 14 is the
   `internal/`-only figure. Unchanged by this round either way, and still zero in
   `internal/csv`, `internal/csvimport` and `internal/admin/csvimport.go`.

Two further notes, not errors:

- **CR-01 was narrower in the review than in the tree.** The body and field arms
  carry the identical shape and the review named neither as a finding. The
  orchestrator's directive is what caught them; without it this round would have
  patched one of three trapdoors.
- **WR-06's snippet slices bytes** while asking for a rune boundary in prose.
  Implemented as asked, not as written.

---

## Gates

Run after every commit; the figures below are the final state.

```
go build ./...   clean
go vet ./...     clean
gofmt -l .       (no output)
go test ./...    exit 0
```

Verification ran **in the main working tree** — no worktree isolation, as
instructed — so every number here is reproducible from the checkout as it stands.

**Transactions.** `grep -rn BeginTx internal/csv internal/csvimport
internal/admin/csvimport.go` → 0 hits. Non-test files tree-wide carrying
`BeginTx`: **15**, unchanged (14 under `internal/`, plus `cmd/holzcloud/cli.go`).
See the correction above.

**Translation catalogues** (`go run ./tools/i18n`):

```
1271 Zeichenketten im Quelltext
en.json   1158 übersetzt, 113 offen, 0 verwaist
es.json   1158 übersetzt, 113 offen, 0 verwaist
fr.json   1158 übersetzt, 113 offen, 0 verwaist
it.json   1158 übersetzt, 113 offen, 0 verwaist
de-CH.json 55 Abweichungen, 0 ohne Gegenstück
fr-CH.json  4 Abweichungen, 0 ohne Gegenstück
it-CH.json  9 Abweichungen, 0 ohne Gegenstück
```

`offen` rose from 111 to 113 and `verwaist` stayed **0**, which is the constraint.
The two new strings are both {{tf}}/{{t}} literals the tool can see, which is the
point of D-32: `row_too_wide`'s sentence in `csv_reason.html`, and
"Beispieldatei für eine neue Website" in `website_list.html`. Plan 09-06 owns the
catalogues.

**No browser pass.** Plan 09-06 owns it.

---

## Files touched, all nine commits

```
cmd/holzcloud/templates/admin/csv_mapping.html
cmd/holzcloud/templates/admin/csv_reason.html
cmd/holzcloud/templates/admin/website_list.html
internal/admin/csvimport.go
internal/admin/csvimport_test.go
internal/csv/csv.go
internal/csv/csv_test.go
internal/csvimport/mapping.go
internal/csvimport/mapping_test.go
internal/csvimport/row.go
internal/csvimport/row_test.go
internal/csvimport/store.go
internal/csvimport/store_test.go
internal/csvimport/verdict.go
```

---

_Fixed: 2026-09-06T20:30:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
