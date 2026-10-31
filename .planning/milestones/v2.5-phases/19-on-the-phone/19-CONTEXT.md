# Phase 19 — On the Phone

Milestone v2.5. Opened 2026-09-17, directly after v2.4 closed.

Asked for in one sentence: **„die webapp soll komplett auch auf dem handy
nutzbar sein"**. The scope was settled the same day, with the measurement in
hand: all of it — no screen wider than the phone, every control at least
44 × 44, every field a person types into at least 16px. Tables stay tables and
scroll inside their card.

## The instrument

Chromium through Playwright at 390 × 844, an iPhone user agent, `isMobile`,
`hasTouch`, threefold pixel ratio, signed in as an administrator with a second
factor. Twenty-nine admin screens. Deliberately **not** in this repository —
the stack constraint says no Node, and a checking script is not an exception
somebody should have to argue about later. The numbers are committed instead,
the way `17-COVERAGE.txt` and `16-BENCH.txt` are.

Two things about the ruler had to be corrected before it could be believed, and
both were found by re-measuring rather than by reading:

1. **`window.innerWidth` grows with the fault.** Chrome's phone emulation
   rescales an overflowing page instead of letting it scroll, so a box measured
   against `innerWidth` is measured against a ruler the overflow itself
   stretched: at 390px the Users screen reported `innerWidth` 665. The honest
   ruler is `document.documentElement.clientWidth`, which stays 390. The first
   survey's `over` column already used it and is therefore comparable; its
   per-element lists did not.
2. **A box that sticks out is not a box a person has to chase.** A cell in a
   table that scrolls inside its own card ends past 390 and is perfectly
   reachable. Asking only "does this end past the edge" counted the cure as
   the disease. The question is whether a scrolling ancestor holds it — every
   scrolling ancestor, not just the nearest, because an `<svg>` clips its own
   contents and stopping at the first one reported a circle inside an icon
   inside a table that scrolled fine.

A third correction belongs to the same family: a closed `<details>` does not
give its contents `display: none`. Chromium hides them with
`content-visibility`, and they go on reporting a box — which is how the
Markdown cheat sheet was reported 24px too wide while folded shut.
`checkVisibility()` sees that; the two properties alone do not.

## The plans

- **19-01** — PHONE-01: no admin screen wider than the phone.
- **19-02** — PHONE-02: every control at least 44 × 44.
- **19-03** — PHONE-03: every typing field at least 16px.
- **19-04** — PHONE-04 and PHONE-05: measure again, commit it, and drive the
  result by hand once in a phone-shaped browser.
