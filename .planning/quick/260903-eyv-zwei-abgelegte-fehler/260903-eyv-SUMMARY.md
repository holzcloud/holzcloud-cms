---
phase: quick-260903-eyv
plan: 01
subsystem: bundle-format, mkbundle, plugin-tests
tags: [bundle, mkbundle, terms, flaky-test, kontaktformular]
status: complete
requires:
  - internal/bundle (Manifest, Term, Page.Terms)
  - internal/page (Slugify)
  - plugins/kontaktformular (zeichnen, zeichnenEigen)
provides:
  - "mkbundle resolves a page's terms against the declared names"
  - "a manifest spelled with slugs is refused with the correct spelling named"
  - "the example bundle ships the name/slug divergence as a regression guard"
  - "TestMarkeMitUnbekanntemArgumentBleibtDerBetreff is deterministic"
affects:
  - tools/mkbundle
  - sites/beispiel
  - internal/public (test only)
tech-stack:
  added: []
  patterns:
    - "the format decision lives in the struct comment, not in a plan file"
    - "assert the shape of the thing, never a two-character proxy for it"
key-files:
  created: []
  modified:
    - tools/mkbundle/main.go
    - tools/mkbundle/pack_test.go
    - internal/bundle/format.go
    - sites/beispiel/holzcloud.json
    - internal/public/formular_e2e_test.go
decisions:
  - "D-01: page.terms holds the name; the fix lands in the validator alone"
  - "D-02: the reading side is not widened to accept both spellings"
  - "D-03: the example carries the awkward case (Laufräder / laufraeder) on purpose"
  - "D-04: the form test asserts the classic form's shape, not two characters"
metrics:
  duration: ~35 min
  completed: 2026-09-03
  tasks: 3
  commits: 3
  files: 5
actuals:
  tokens: 3467
  tasks: 3
  commits: 3
---

# Quick 260903-eyv: Zwei abgelegte Fehler Summary

mkbundle now resolves a page's labels against the declared names rather than the
declared slugs, so a bundle exported from a real site packs even when a label was
typed with a capital or an umlaut; and the contact-form test asserts the classic
renderer's own opening tag instead of searching the whole document for two
characters that a random base64url HMAC contains about once in eighty runs.

## What Was Built

**QF-01 — the three handlers of `page.terms` now agree.**
`internal/bundle/export.go` writes `t.Name` into a page's terms and
`internal/bundle/import.go` reads a name back through `term.Parse`. Only
`tools/mkbundle`'s `checkReferences`, the youngest of the three, compared those
entries against the declared *slugs*. The lookup set is now keyed by
`t.Name`; a second map from slug to name is consulted only on the failure path,
so a page entry that turns out to be a slug is told which spelling belongs there
instead of being told the entry is missing. `join`'s behaviour is untouched, and
no check for slug/`Slugify` agreement was added — see the limits below.

**The decision is written where the format is defined.** `internal/bundle/format.go`
now states on `Page.Terms` that it carries names as shown, and on `Term` that the
name is the load-bearing half while the declared slug is documentation. The next
person meets the decision there, not in a plan file.

**QF-02 — the form test asserts the form.** `TestMarkeMitUnbekanntemArgumentBleibtDerBetreff`
now asserts positively that `zeichnen`'s opening tag is present, and negatively
that none of `zeichnenEigen`'s three fingerprints are: the `contact-form--`
class, the hidden `name="formular"` field, and the field prefix inside a quoted
`name="f_` attribute — the only position it can occupy, and one a base64url value
can never reach because base64url has no quote character.

## Failing-First Evidence

Both fixes were seen red before the code moved. Neither guard was written after
the fact.

**QF-01 — `go test -v ./tools/mkbundle/` before the validator changed:**

```
=== RUN   TestAPageMayCarryALabelSpelledAsItsName
    pack_test.go:141: a page naming a declared label was refused: 1 problem(s):
          - page "werkstatt": term "Laufräder" has no entry in terms
--- FAIL: TestAPageMayCarryALabelSpelledAsItsName (0.00s)
=== RUN   TestALabelSpelledAsASlugSaysWhatToWriteInstead
    pack_test.go:155: a page spelling a label as its slug was accepted
--- FAIL: TestALabelSpelledAsASlugSaysWhatToWriteInstead (0.00s)
=== RUN   TestALabelThatIsNeitherNameNorSlugIsStillRefused
--- PASS: TestALabelThatIsNeitherNameNorSlugIsStillRefused (0.00s)
FAIL	github.com/holzcloud/holzcloud-cms/tools/mkbundle	0.438s
```

The third passing from the start is the point of it: it is the guard that the
fix did not degrade into accepting everything, and it must be green in both
worlds.

**The example, measured against the old slug-keyed check** (a throwaway test
reproducing the old logic against the new manifest, then deleted) — 3 problems:

```
page "werkstatt": term "Laufräder" has no entry in terms
page "werkstatt": term "Unterhalt" has no entry in terms
page "service": term "Unterhalt" has no entry in terms
```

So the shipped example is now itself a regression guard: a revert of the
validator reddens `go test ./...` through `TestTheExampleStillPacks`.

**QF-02 — `go test -count=400 -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff ./internal/public/`
before the assertion was scoped:** `FAIL`, 431.850s, one failure in 400 runs at
`formular_e2e_test.go:414`:

```
es wurde ein zusammengestelltes Formular gezeichnet:
```

The colliding value, taken from the failure dump:

```
name="gestellt" value="1788426524.5i0nyESyRQRxhsQpCC5Ne3XgqP5aP28MoYf_p0Bm-K8"
```

— `…MoY` **`f_`** `p0Bm-K8`. Nothing was broken; the HMAC simply landed on those
two characters.

**After the fix:** `ok github.com/holzcloud/holzcloud-cms/internal/public 439.640s`
for the same 400 runs, and the single-run check reports
`--- PASS: TestMarkeMitUnbekanntemArgumentBleibtDerBetreff (1.13s)` — a real pass,
not a skip.

**The new assertion still bites.** A throwaway test built a composed form through
the admin screens and confirmed all three fingerprints are present in it and the
classic opening tag is not, so the replacement would still fail if `zeichnenEigen`
ever ran here. Deleted afterwards; the neighbouring `TestEigenesFormularVonEndeZuEnde`
already pins `name="f_dein-name"` permanently.

## Survey of `internal/public/formular_e2e_test.go`

Every negative `strings.Contains` in the file, judged against "could this
substring occur by chance in a value the test does not control". **Outcome:
nothing else needed changing.** No assertion was churned.

| Line | Substring | Verdict |
|------|-----------|---------|
| 43 | `"<p><form"`, `"<p></p>"` | Immune. Both contain `<` and `>`, which base64url has not got. |
| 148 | `"<script"`, `"<img"` | Immune, same reason. |
| 413 (was) | `"f_"` | **The flake.** Two characters, both in the base64url alphabet. Fixed. |
| 468 | `"<script"`, `"javascript:"` | Immune — `<` and `:` are outside base64url. |
| 468 | `"onclick"` | Immune. Seven fixed lowercase letters against an admin screen that carries no random value; even against a 43-character base64url string the odds are about 1 in 10¹¹. |

## The Example Bundle

`sites/beispiel/holzcloud.json` gained two labels and no new key that is not a
json tag in `internal/bundle/format.go`:

- `Laufräder` under `laufraeder` — the awkward case, because `internal/page/slug.go`
  folds `ä` to `ae`. Justified by the text: a wheel is rebuilt on the workshop page
  and wheels are trued on the service page.
- `Unterhalt` under `unterhalt` — the ordinary case, differing only in case.

`werkstatt` carries both, `service` the second; `home` and `kontakt` carry none,
because the field is optional and an example should show that too. Neither slug
collides with a page slug. Both slugs were confirmed by **running** `page.Slugify`
in a throwaway test rather than by eye:

```
"Laufräder" -> "laufraeder"
"Unterhalt" -> "unterhalt"
```

`TestTheExampleStillCarriesALabelWorthShowing` pins all of it, including the rule
that binds the example and deliberately not mkbundle: every declared slug must be
the one the import would derive from the name.

## Deviations from Plan

None — the plan executed exactly as written. All three findings (A, B, C) and
findings D, E, F, G were confirmed by reading before anything was changed, and
none contradicted the plan.

## Known Limits (confirmed while reading, deliberately not fixed)

These predate this task, no validator can repair them, and none is in the diff:

1. **A declared label that no page carries is not created on import.**
   `internal/bundle/import.go` does exactly one thing with `m.Terms`:
   `report.Terms = len(m.Terms)`. Labels come into being as a side effect of the
   names carried by pages.
2. **A renamed label's slug does not survive a round trip.** `term.Store.Rename`
   changes the name and leaves the slug alone on purpose; the importer re-derives
   the slug from the name, so the original address is lost. This is precisely why
   mkbundle must *not* insist a declared slug equals `Slugify(name)` — a genuine
   export of a renamed label legitimately breaks that equality.
3. **Two labels sharing a name merge on import.** The database holds
   `UNIQUE (website_id, slug)` and nothing on `name`, and `Rename` has no
   collision check (`internal/admin/term.go` `HandleTermRename` only refuses an
   empty name). Two rows can therefore share a name, and on import they become
   one. A property of the format as it stands, not something this task created.

## Note for whoever moves the .wasm modules out of the repository

**`internal/public/formular_e2e_test.go` depends on a built
`plugins/kontaktformular/plugin.wasm` being present, and it skips *silently*
without one.** `formularAufbau` calls `t.Skipf` when the file is missing, so the
whole suite — including the 400-run gate that proves QF-02 is fixed — reports
`ok` while proving nothing at all.

The file was present for this work (4.7 MB, committed). A decision was taken
today to move the shipped `.wasm` modules out of the repository and build them at
release time. Unless something builds them before `go test` runs, this suite goes
quiet rather than red, and the flake this task removed could be reintroduced
without a single failing test. Either build the modules in CI before the test
step, or turn the `t.Skipf` into a `t.Fatalf` when an environment variable says
the modules were meant to be there.

## Verification

| # | Gate | Result |
|---|------|--------|
| 1 | `go build ./...` | pass |
| 2 | `go vet ./...` | pass |
| 3 | `go test ./...` | 39 ok, 0 FAIL |
| 4 | `go test -count=400 -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff ./internal/public/` | `ok … 439.640s`; single run shows `--- PASS`, not a skip |
| 5 | `go run ./tools/mkbundle sites/beispiel` | exit 0, `sites/beispiel.zip` stays ignored |
| 6 | Packing test for name ≠ slug | seen FAIL before the fix, PASS after |
| 7 | `go run ./tools/i18n` | en/es/fr/it each `1128 übersetzt, 0 offen, 0 verwaist` |
| 8 | `internal/bundle/export.go`, `import.go`, `go.mod`, `go.sum` | no diff |

## Commits

| Task | Commit | Subject |
|------|--------|---------|
| 1 | `44977af` | Die Marken einer Seite tragen ihren Namen, nicht ihre Adresse |
| 2 | `d343a58` | Das Beispiel bekommt endlich die Marken, die es nicht tragen durfte |
| 3 | `8b697fc` | Der Formulartest prüft die Form und nicht zwei Zeichen im Rauschen |

## Self-Check: PASSED

All five modified files exist on disk, all three commit hashes resolve in
`git log --all`, and the three commits delete no tracked file. Working tree
clean apart from this summary, which the orchestrator commits.
