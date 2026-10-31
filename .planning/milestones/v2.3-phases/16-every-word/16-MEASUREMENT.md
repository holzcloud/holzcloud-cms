# 16-MEASUREMENT — What the five frozen words would cost to make late

WORD-04, answered before WORD-02 chooses anything. Measured 2026-09-15 on
`internal/block/frozen_bench_test.go`; the raw output is `16-BENCH.txt` beside
this file, and `go test ./internal/block/ -run '^$' -bench . -benchmem`
reproduces it.

Machine: Intel Xeon @ 2.10 GHz, linux/amd64, Go 1.26.6. The absolute numbers
belong to that machine; the ratios are what the decision rests on.

---

## §1 Which pages carry the words at all

Five strings this program mints are written into `pages.content_html` at save,
in the website's language at that moment. They are written by exactly two arms
of `block.Render`:

| Word | Written by | Appears on |
|---|---|---|
| `Your browser cannot play this video.` | the video arm (`render.go:141`) | a page with a **video** block |
| `Gallery` | `GalleryWrapper` (`render.go:640`) | a page with a **slideshow** gallery |
| `Previous image` | the lightbox (`render.go:761`) | a page with a gallery |
| `Next image` | the lightbox (`render.go:765`) | a page with a gallery |
| `Close large view` | the lightbox (`render.go:768`) | a page with a gallery |

No other arm calls `Set.text` or the wrapper's `t`. So a page with neither a
gallery nor a video carries **none** of them — which is most pages, and is the
case any mechanism must not tax.

That is a stronger statement than it looks: it means the set of bodies that need
work is exactly the set of bodies that contain a needle, and a needle can be
looked for without building anything.

## §2 The corpus

Three page bodies, built by `block.Render` from text blocks and, where the name
says so, one eight-picture slideshow:

| | bytes | bytes with a gallery |
|---|---|---|
| `small-2k` — a contact page, an imprint | 2 664 | 8 129 |
| `medium-20k` — an article, what most pages are | 23 643 | 29 138 |
| `large-200k` — a long report | 235 431 | 240 956 |

## §3 What each candidate costs, per request

`medium-20k`, the typical page, with no gallery — the common case:

| Candidate | ns/op | B/op | allocs |
|---|---:|---:|---:|
| **A** `strings.Contains` — the cheap question, already paid | **358** | **0** | **0** |
| **B** a regular expression over the whole body | 409 | 0 | 0 |
| **C** sentinel replace, unguarded | 37 439 | 49 176 | 3 |
| **C′** sentinel replace, guarded by A | **311** | **0** | **0** |
| **D** render the blocks again | 123 842 | 214 657 | 348 |

And on a page that *does* carry a gallery, where the work cannot be skipped:

| | ns/op | B/op | allocs |
|---|---:|---:|---:|
| **C′** sentinel replace, hit | 64 695 | 106 520 | 4 |
| **D** render the blocks again | 123 842 | 214 657 | 348 |
| album marker expansion, as shipped in v2.2 | 23 347 | 50 263 | 9 |

Small and large scale linearly; `16-BENCH.txt` has all nine rows of each.

## §4 What the numbers say

**1. Scanning is free. Rewriting is what costs.** The interesting pair is B
against C: a regular expression over 23 KB costs 409 ns and allocates nothing,
while a `strings.Replacer` over the same 23 KB costs 37 µs and allocates 49 KB —
ninety times the time and a garbage-collected copy of the whole page. The
expense was never the search. It is producing a second body.

This reframes the design question. It is not "how do we find the words
cheaply", it is "how do we avoid building a new string for a page that has
nothing to resolve".

**2. The guard is the whole decision.** C′ against C is 311 ns against 37 439 ns,
and 0 B against 49 176 B. Guarded, the mechanism costs the common case *less
than the album marker's own guard already costs it* — it is the same
`strings.Contains` pass, and the two can share one. Unguarded it puts a
kilobyte-per-kilobyte copy into the garbage collector on every view of every
page on the server.

`hasAlbumMarker`'s comment — *"the cheap question, asked before the expensive
one"* — is not a style preference. It is a factor of a hundred.

**3. Re-rendering is out, and now by measurement rather than instinct.**
16-CONTEXT §4 guessed that "resolve everything late" is the broad answer and
probably the wrong one. D costs 124 µs and 215 KB across 348 allocations on a
medium page — 400× the guarded sentinel on the common case, 5× the album
expansion on the case that has work to do, and that is with media lookups served
from a map. A real request would add a database query per picture. It also
cannot be guarded: there is no cheap question that says "this page would render
the same", because the render *is* the question.

**4. C′ is affordable by a standard this project has already accepted.** On a
page with a gallery it costs 65 µs and 107 KB. The album expansion shipped in
v2.2 costs 23 µs and 50 KB on the same page and nobody has called that
expensive. C′ is within the same order of magnitude, on strictly fewer pages
than "every page", and on a page that has both the two passes can be folded into
one.

## §5 What this does not settle

- **Whether the sentinel is the right shape.** These numbers say a guarded
  one-pass substitution is affordable; they say nothing about whether the
  substituted token should be `[[w:gallery]]`, a `data-` attribute, or something
  else. That is WORD-02's choice and it has other criteria: an old body must
  keep working (WORD-03), and a sentinel that leaks to a visitor is the defect
  `internal/album/expand.go` already names.
- **The lightbox's `id` attributes.** The controls are `<a href="#hc-b1-p2">`
  anchors; only their *text* is translated. A substitution that touched the
  fragment ids would break D-03's uniqueness rule, so whatever token is chosen
  must be unambiguous inside an attribute — the context-sensitivity that
  `albumMarkerPrefix`'s own comment records as an unfixed limitation of the
  marker mechanism.
- **Bodies not written by `block.Render`.** A page that is plain Markdown
  carries none of the five words, but `internal/bundle/import.go` renders blocks
  on import with the same frozen translator, and a body that arrives through it
  has to end up in the same state as one that was saved.
