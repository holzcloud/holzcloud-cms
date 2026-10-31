# Verification: Phase 16 — Every word on a page belongs to whoever reads it

Measured 2026-09-15 against `ae726d1`, by running the gates, by reverting each
fix to see its test go red, and by driving a binary at `127.0.0.1:8080`.

**430 Go files, 120 953 lines, 1 367 test functions, 1 830 catalogue entries in
each of de, es, fr and it.** Five commits since the v2.2 tag; 65 files changed,
+2 608 / −217.

---

## WORD-04 — the measurement before the mechanism

`.planning/phases/16-every-word/16-MEASUREMENT.md`, with the raw output beside
it in `16-BENCH.txt` and the bench itself committed as
`internal/block/frozen_bench_test.go`, so the numbers can be produced again
rather than believed.

What it found, on a 23 KB page with no gallery:

| | ns/op | B/op |
|---|---:|---:|
| `strings.Contains` — the guard | 358 | 0 |
| a regular expression over the whole body | 409 | 0 |
| sentinel replace, unguarded | 37 439 | 49 176 |
| sentinel replace, guarded | 311 | 0 |
| render the blocks again | 123 842 | 214 657 |

Two readings, and both changed what was built. **Searching is free; building a
second body is what costs** — the expense was never the scan. And **the broad
answer is out on measurement**: re-rendering per request costs 400× the guarded
pass on the common case and cannot be guarded at all, because the render is the
question.

Also established, and it narrows everything after it: only a page with a gallery
or a video carries any of the five words. Two arms of `block.Render` write them
and no others.

**Verified.**

## WORD-01, WORD-02 — the words belong to the page

`internal/block/words.go`. `block.Render` writes `[[w:<key>]]`; one guarded pass
at delivery resolves every marker in the language of the page being read.

What was *removed* is the evidence: `block.Set.T`, the `t` parameters of
`GalleryItems` and `GalleryWrapper`, and `album.Set`'s translator are gone.
Window 34 closes by subtraction — where there is no second translator, two
cannot disagree. `albumsFor` is three lines shorter and no longer mentions a
locale.

Held by `TestAPageAnswersInItsOwnLanguageAndNotTheWebsites`: a German website
with a French page, one stored body planted in both, and the assertion is on a
control whose German and French differ. Reverting `pageWords` to `ws.Locale`
turns it red; reverting it to the operator's language turns it red as well,
because the request carries a German browser and a French page.

**Verified.**

## WORD-03 — an old page is served unchanged

A body written before this milestone carries the words themselves and no marker.
`HasWordMarker` is false, `ResolveWords` returns the identical string, and
`TestResolvingADocumentWithNoMarkerAllocatesNothing` asserts the allocation
count and not only the value — an equal string built fresh would pass the value
check and still cost the copy the guard exists to avoid.

`TestAPageStoredBeforeTheMarkersIsServedUnchanged` drives it through the public
handler with a v2.2-shaped body.

No migration was written and none is needed. A page heals on its next save,
which is window 8's rule applied a second time.

**Verified.**

## WIN-07 — the ledger reaches zero

**0 open, 25 fixed, 9 waived, 34 total** — zero for the first time since the
ledger was opened. Entry 34 was the last one, and WORD-01/02 closed it.

The three places that have to agree — the front matter, the table and the JSON —
were compared programmatically rather than by eye, because they had drifted by
four rows once before.

**Verified.**

## WIN-08 — a plugin's own words

Decided rather than deferred a fourth time: **translated.** `plugin.json` carries
`"lang"`, keyed by tag and then by the source sentence, and the host resolves
name, description and admin label in the operator's language.

The measurement that made it worth doing is not a benchmark but a screenshot:
four of the five shipped plugins had said their own name in German since v2.0
turned the source language round, on the first screen an operator meets.

Driven in a browser from one installed package, in two operator languages:

```
en: Year        = True   Jahreszahl = False
de: Jahreszahl  = True   Year       = False
```

**Verified.**

## QUAL-01 — the gates

```
gofmt -l .                     clean
go vet ./...                   clean
go run ./tools/english         no German in the Go source outside the catalogues
go run ./tools/themewords -check   every shipped theme's catalogue matches
go run ./tools/wasm -check     all current
go run ./tools/i18n            1830 translated, 0 open, 0 orphaned (de, es, fr, it)
go test ./...                  green, with HOLZCLOUD_TEST_REQUIRE_WASM=1
```

`tools/wasm -check` is the one worth naming: it had been **red for two days**
and nobody had read it. Four plugin modules were compiled against the SDK as it
stood before the contact-form work. That is not a v2.3 defect — it is a v2.1
defect that two milestones closed on top of — and it is why v2.1's back-filled
tag carries a written exemption and v2.2's does not sit on the commit that
closed it.

**Verified, and the finding is larger than the gate.**

## QUAL-02 — driven in a browser

Three screens, against a binary built from this tree, with a real database, a
real login and a real second factor:

- `/admin/neuerungen` — the changelog screen (a quick task, not a requirement)
- `/admin/plugins` — in English and in German, with a package installed through
  the upload form
- the sidebar, for the plugin's own menu entry

**Three faults came out of it that no test had**, and all three are fixed:

1. `plugin_list` and `plugin_screen` printed a heading `base.html` already
   prints, so two screens said "Plugins" twice. So did the new changelog screen
   before it was driven.
2. The changelog CSS reached for `--color-primary`; this project's accent is
   `--color-accent`. An unknown custom property fails silently, so the selected
   version would simply have had no outline and nothing would have said so.
3. The plugin screen's back link sat beside the heading instead of in the slot
   `base.html` keeps for actions.

None of them is subtle. All three needed a browser.

**Verified.**

---

## What this phase found that it did not plan for

**CI was red and nobody was reading it.** Two days, every push, in the step that
exists for exactly this. The release workflow did not run that check at all, so
a stale module could have travelled inside a tag; it runs it now.

**A comment in the SDK invalidates every plugin module.** Go's build id covers
comments, so editing `sdk/plugin.go`'s documentation changes every `.wasm` by a
few bytes at the same length. Harmless, and worth knowing before somebody spends
an afternoon on "non-deterministic builds".

**The three releases that had never been made.** v2.0, v2.1 and v2.2 had
CHANGELOG entries, close commits and no tags. They exist now, made by the
pipeline rather than by hand, each built and tested from the code it names.
v2.2 is tagged at `b2356f9` and not at its close commit, for the module reason
above; v2.1 is tagged at its close commit with the check waived and the reason
written into the workflow beside it.
