# 18 — Verification

Measured 2026-09-17, at the end of the phase.

---

## TEST-01 — `internal/admin` is materially better covered than 35.9 %

**59.3 %.** Measured with `go test ./internal/admin/ -cover`; the per-function
output is committed beside this file as `18-COVERAGE.txt`.

| | before | after |
|---|---|---|
| statements covered | 35.9 % | **59.3 %** |
| functions at 0 % | 232 | **108** |
| handlers at 0 % | 104 | **40** |

The number is not the point and the requirement says so. What was covered is the
screens an operator uses every day: the brand, the five media screens, menus and
labels, the website and its domains, the six template screens, the page list and
the selection list, the activity log, the dashboard, the six language screens,
the account screens, the two kind screens, the bundle screens, the wording
screen, the list's own furniture, the outbox, the media picker and the preview
link. What is left at zero is the long tail — the shop, two-factor enrolment,
the CSV importer's inner panels, WordPress import, setup.

**Eleven defects, every one found by writing a test rather than by reading the
code.** That is the requirement's real argument, so they are listed:

1. `looksLikeSVG` was a prefix test and four forbidden substrings. `onerror=`,
   `onmouseover=`, `onload =` and `javascript&#58;` all walked past it, as did
   an RSS feed and a truncated file. It parses now.
2. The menu location-key rule was applied on create and not on rename, so
   "haupt" could become "Haupt Menü", be answered "Menu saved", and take the
   navigation off the site.
3. `HandleDomainRemove` passed the store a domain id and nothing else, so
   website A's screen could take a domain off website B — a customer's site off
   the air while the screen said "Domain removed".
4. `kind.Store.Count` matched the `kind` column, and an entry of a website's own
   kind is stored as a PAGE with the key beside it. Three products counted as
   zero products and as three pages; the delete screen's warning never fired.
5. The import report carried TWO headings, the second hard-coding "WordPress
   import finished" — for a file this program wrote itself.
6. `message := "Website angelegt"` reached a flash as a variable, so the
   catalogue gate reported neither open nor orphaned about it.
7. `src.Title + " (Kopie)"` — a German suffix glued on in Go, so every
   installation in every language titled its copies that way.
8. Seven more sentences built with `fmt.Sprintf`, two still German.
   `tools/assembled` is the gate for that shape and CI runs it.
9. `linkTitle` returned two bare German titles into a translated slot.
10. `lookupTerm`'s doc comment claimed a website check it does not perform.
11. `exportFilename`'s empty-slug fallback is dead code — `page.Slugify` answers
    "untitled" and never "".

**Three faults in the test harness itself**, each of which had made a whole
class of test impossible without anybody noticing:

- `newTestAdmin` built the loader with a nil template resolver, so the loader in
  every admin test fell back to the shipped theme and could not see which theme
  a website runs.
- It passed a zero `Argon2Params`, so every path that hashes a password
  **panicked** rather than failing. It had been broken as long as it was unused.
- It built the loader without a wording resolver, so a word saved on that screen
  could never reach a rendered page.

## TEST-02 — `internal/branding` has tests

**0.0 % → 95.7 %.** Eleven tests. What is left uncovered is a driver fault in
`rows.Scan`, a file that disappears between `LogoPath` and `Stat`, and a write
that fails after `MkdirAll` succeeded — three races and a hardware failure.

## TEST-03 — each new test is driven red before it is driven green

**164 mutations run, 158 caught.** The six survivors are recorded at their own
tests, each with the measurement that shows it is equivalent rather than missed:

- `> max` → `>= max` slices the whole value again at the bound.
- Removing `WriteLogo`'s empty-folder guard leaves `os.MkdirAll("")`, which
  fails with ENOENT anyway.
- `depth == 0` in `looksLikeSVG` is unreachable because the decoder errors on a
  truncated document first.
- The empty-filename guard in `HandleMediaServe` leaves a lookup for the file
  called "", which no website has.
- Passing nil instead of the fallback FS changes only what the acceptance check
  does with views an archive omits; held in `internal/template`'s own tests.
- An archive carries no domains, so the resolver cache can hold nothing about a
  website that has just been imported.

**The lesson this phase actually taught**, because it caught the work fourteen
times: *a second guard one layer down, or a catch-all arm, makes the first
guard's removal invisible.* `Load` re-trims what `clean` trimmed.
`NormalizeDomain` re-refuses what the screen refused. `time.Parse("")` fails
where the "no date" guard already did. "Saving failed." answers for every
unrecognised error. Each time the test passed while the guard was gone, and each
time the fix was the same: assert the sentence the operator reads, or the number
they see — never "an error came back".

## QUAL-01 — the standing gates

All green: `tools/i18n` reports `0 open, 0 orphaned` on four catalogues (1840
strings), `tools/english`, `tools/assembled` (new this phase), `tools/cites`,
`tools/themewords -check`, `tools/wasm -check`. `go vet ./...` and `gofmt -l`
clean. The full suite passes.

## QUAL-02 — every screen driven once through the running application

**Done, and nothing found** — the first milestone where that is true.

Thirty addressable admin screens driven in a real browser at 1280 × 900, signed
in as an administrator with a second factor, checking for: more than one `<h1>`,
a CSS custom property used and never defined, a link with an empty or `#`
target, an unresolved marker on the page, a console error, a status of 400 or
more.

Then the four screens an operator only reaches by DOING something, because that
is where all three of v2.3's faults were: the import report (after a real export
and import), the access link, the preview link, and the changelog behind the
version number.

The v2.3 faults are confirmed gone, and the import report's second heading —
found by a test this phase — is confirmed gone in the browser too.

## QUAL-03 — the suite does not get slower than it is useful

`internal/admin` went from 265 s to 498 s, which is the cost of 89 new tests
that each open a database and parse the admin templates. The whole suite is
still under fifteen minutes. This is the number to watch in v2.5: the next
phase that adds tests at this rate should make the fixture cheaper first.
