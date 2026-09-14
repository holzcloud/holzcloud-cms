# Context: Phase 16 — Every word on a page belongs to whoever reads it

Opened 2026-09-14, the day v2.2 closed.

---

## §1 The sentence three milestones have been writing

- **v2.0** turned the source language round: the English sentence is the
  catalogue key, and German became a translation like any other.
- **v2.1** gave the public side a channel at all — a theme carries its own
  catalogue, the operator can override any key, and a plugin answers a visitor
  in the **page's** language rather than the operator's. PUB-01 existed because
  v2.0 had got that exact question the wrong way round.
- **v2.2** worked the ledger to one entry. That entry, window 34, is PUB-01's
  question one level down: an album gallery answers in the **website's**
  language rather than the page's.

So this phase is not a new subject. It is the last room in a house three
milestones have been building, and the reason it was left is written in window
8's own text: the fix "moves the boundary between what a gallery freezes at save
and what it resolves at request — an architecture question".

## §2 What is actually frozen

`block.Render` writes a page's blocks to HTML **once, on save**, into
`pages.content_html`. `internal/admin/page_blocks.go` says so at its own site,
and `internal/bundle/import.go` does the same on import. Five strings this
program mints ride along:

| | |
|---|---|
| `Previous image` | lightbox control |
| `Next image` | lightbox control |
| `Close large view` | lightbox control |
| `Your browser cannot play this video.` | between the `<video>` tags |
| `Gallery` | the scrolling region's accessible name |

`blockSet` translates them from `ws.Locale` at save. So they carry the language
the website had **when somebody last pressed Save** — and a website that changes
its language re-renders them only as each page is saved again, which is what
window 8 measured and what v2.2 half-fixed.

Half-fixed, precisely: window 8 made an album gallery's wrapper late along with
its tiles, so the two halves of one region agree. It did **not** make them the
page's language, because of the reason `albumsFor` states in its own comment —
an inline gallery and an album gallery on the same page must not end up in two
languages, and the inline one is frozen.

That comment is right, and it is why WORD-01 cannot be done alone. Fixing only
the album half would replace one inconsistency with another.

## §3 The trap to plan around, before choosing anything

**Most sites have one language.** For them these five words are correct today,
were correct yesterday, and cost nothing: they are bytes in a column, already
rendered.

Any mechanism that makes them late costs something on **every** page view of
**every** site — a marker scan, an expansion, a second pass over the body. The
album marker already pays that price, and `hasAlbumMarker` exists precisely to
make a page that names no album cost nothing: *"the cheap question, asked before
the expensive one"*.

So WORD-04 comes first and alone: **measure before choosing.** How many stored
pages carry these words; what each candidate costs a page that has one language
and no gallery; and whether the cheap question can be asked here too. A
mechanism that is elegant and taxes the common case is the wrong one, and this
project has the measurement habit to settle it rather than argue it.

## §4 What v2.2 taught that applies directly

**The optional marker tail.** Window 8 changed a stored format without a
migration by making the new fields optional: `[[album:slug:at]]` still parses,
an old page's wrapper is still in its stored HTML, and the page heals on its
next save. WORD-03 asks for the same property, and that is the shape that
delivers it.

**Narrow beats broad.** Twice in v2.2 the conservative-looking change was the
wrong one — adding `Cookie` to every answer, asking whether the account was
linked. Both would have looked safe and cost the common case. The same instinct
applies here: "resolve everything late" is the broad answer, and it is probably
not the right one.

## §5 The two ledger entries

**WIN-07** is window 34 itself, which WORD-01 and WORD-02 close together. The
ledger then reaches zero for the first time since it was opened.

**WIN-08** is the older one: a plugin's `plugin.json` name and description are
not translated, carried as "deliberately open" through v2.0, v2.1 and v2.2. It
is visible as the one untranslated sentence on `/admin/plugins`. Three
milestones of deferral is long enough that the entry should say *decided*
rather than *deferred* — either it is translated, or the reason it cannot be
stands where a plugin author reads it.
