# Requirements: v2.3 — Every Word on a Page Belongs to Whoever Reads It

Milestone opened 2026-09-14, the same day v2.2 closed.

It finishes a sentence three milestones have been writing. v2.0 turned the
source language round. v2.1 gave the public side a translation channel and made
a plugin answer in the page's language. v2.2 closed the ledger to one entry —
and that entry is the same question one level down: an album gallery answers in
the **website's** language rather than the **page's**.

Behind it sits the larger half, which window 8 expected to be the whole of it:
**the words this program mints into a page are frozen at save time.** Block HTML
is rendered once and stored, so a gallery's five words carry the language the
website had when somebody last pressed Save. On a monolingual site nobody
notices. On a site with a second language, every one of them is wrong for half
the visitors.

**Milestone goal:** every word this program writes into a page a visitor reads
is resolved in the language of that page, at the moment it is delivered. And the
ledger reaches zero.

---

## The words the renderer mints

- [x] **WORD-01**: **An album gallery answers in the page's language.** Window
      34. `internal/public/pagedata.go`'s `albumsFor` builds its translator from
      `ws.Locale`, so a French page on a German website reads German lightbox
      controls. Its own comment gives the reason — an inline gallery and an
      album gallery on one page must not speak two languages — which is true and
      is solved by WORD-02 rather than by leaving both wrong.
- [x] **WORD-02**: **A gallery's words are resolved at delivery, not frozen at
      save.** Five strings today (`Previous image`, `Next image`, `Close large
      view`, `Your browser cannot play this video.`, `Gallery`), written into
      `pages.content_html` by `block.Render` through `admin/page_blocks.go` and
      `bundle/import.go`. The album half already resolves late through its
      marker; the inline half does not, and that asymmetry is what forced
      WORD-01's compromise.
      *Done 2026-09-15 (16-01), all three together:* `internal/block/words.go`.
      `block.Render` writes `[[w:<key>]]` for all five, and one guarded pass at
      delivery resolves them in the page's language. `block.Set.T`, the `t`
      parameters of `GalleryItems` and `GalleryWrapper` and `album.Set`'s
      translator are gone — where there is no second translator, two cannot
      disagree, which is what WORD-01's compromise was protecting against.
- [x] **WORD-03**: **A page written before this milestone keeps working, and
      heals.** The same rule window 8 followed: no migration, no broken page, no
      re-save required for a page to render. Whatever mechanism WORD-02 chooses
      must read old stored HTML unchanged.
      *Done:* a body written before this milestone carries the words and no
      marker, the guard skips it and it is returned untouched
      (`TestAPageStoredBeforeTheMarkersIsServedUnchanged`).
- [x] **WORD-04**: **The measurement is written down before the mechanism is
      chosen.** How many stored pages carry frozen words, in how many
      installations' shapes, and what each candidate mechanism costs a
      monolingual site — which is most of them, and which must not pay for this.
      *Done 2026-09-15:* `phases/16-every-word/16-MEASUREMENT.md`, the raw
      numbers in `16-BENCH.txt`, the bench itself in
      `internal/block/frozen_bench_test.go`. Only a page with a gallery or a
      video carries any of the five words at all. Searching a body is free
      (358 ns, no allocation on a 23 KB page); building a second one is what
      costs (37 µs and 49 KB unguarded). Re-rendering the blocks per request —
      the broad answer — costs 124 µs and 215 KB and cannot be guarded, so it
      is out on measurement rather than on instinct. A guarded one-pass
      substitution costs the common case 311 ns and nothing at all.

## The ledger

- [x] **WIN-07**: **The ledger reaches zero open entries**, or any entry that
      remains carries a reason a reader can check. It stands at one.
      *Done 2026-09-15:* **0 open, 25 fixed, 9 waived, 34 total** — zero for the
      first time since the ledger was opened. Entry 34 was the one left, and
      WORD-01/02 closed it.
- [x] **WIN-08**: **A plugin's `plugin.json` name and description are
      translated, or the reason they are not is written where a reader meets
      it.** Carried unchanged through v2.0, v2.1 and v2.2 as "deliberately
      open", which after three milestones is a decision that should be stated
      rather than deferred again.
      *Done 2026-09-15 (16-02):* **translated.** A plugin carries its own
      catalogue in `plugin.json` under `"lang"`, keyed by tag then by the
      source sentence — the theme rule, in the manifest because there are three
      strings and the host reads them before the module exists. The five
      shipped plugins were rewritten with English sources and de, es, fr and it
      catalogues; four of them had said their own name in German since v2.0.
      `sdk/plugin.go` documents it for plugin authors. Driven in the browser:
      an English operator reads "Year", a German one "Jahreszahl", from one
      installed package.

## Standing gates

- [x] **QUAL-01**: `go run ./tools/i18n` reports `0 open, 0 orphaned` on every
      catalogue; `tools/english` and `tools/themewords -check` stay green.
      *Green 2026-09-15:* 1830 strings, 0 open, 0 orphaned in de, es, fr and it;
      `tools/english`, `tools/themewords -check` and `tools/wasm -check` all
      pass — the last of those was red for two days before this milestone
      noticed.
- [x] **QUAL-02**: Every screen touched is driven once through the running
      application. The blast radius here is a page with both kinds of gallery,
      in two languages, before and after a re-save.
      *Done:* the changelog screen, the plugin list and the plugin screen were
      each driven in a browser against a running binary, the last two in two
      operator languages. Three faults came out of it that no test had: a
      heading printed twice on three screens, a CSS custom property this project
      does not define, and the plugin screen's back link sitting outside the
      place base.html keeps for it.
