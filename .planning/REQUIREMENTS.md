# Requirements: v2.5 — On the Phone

Opened as a plan on 2026-09-17, to start when v2.4 closes. Asked for in one
sentence: **„die webapp soll komplett auch auf dem handy nutzbar sein"**.

The scope was settled the same day, with the measurement in hand: **all of it** —
no screen reaching past the edge of the phone, every control at least 44 × 44
pixels, every field a person types into at least 16px. Tables stay tables and
scroll inside their card; turning them into stacked cards was offered and not
chosen.

The measurement is committed beside this file as `v2.5-PHONE-SURVEY.txt`, so
the next reading is a comparison rather than a fresh opinion.

---

## What was measured, and how

Chromium through Playwright at **390 × 844** — an iPhone 14/15, at or below the
width most people hold — with `isMobile`, `hasTouch`, a threefold pixel ratio
and an iPhone user agent, signed in as an administrator with a second factor.
Twenty-nine admin screens, every one answering 200.

The tool is deliberately **not** in this repository. The stack constraint says
no Node, and a checking script is not an exception somebody should have to argue
about later; the numbers are committed instead, which is the same thing
`17-COVERAGE.txt` and `16-BENCH.txt` do.

**There is already a mobile layer**, and this is not a rewrite: the viewport
meta tag, a 900px breakpoint with an off-canvas sidebar and a hamburger, and a
container query meant to make tables scroll inside their card. What the survey
found is that some of it does not reach.

## The requirements

- [x] **PHONE-01**: **No admin screen is wider than the phone.** Seven were,
      and the reading above guessed the cause wrong on six of them. The
      container query applies; the card scrolls; what ran out of it was a
      `.sr-only`, which is `position: absolute` and, with no positioned
      ancestor, sits at its static position far out in the wide part of the
      table and drags the page out to meet it. `position: relative` beside the
      `overflow-x: auto` took Users (275), Pages (187), Languages (97),
      Websites (50) and Menus (41) to 0 in one line. The Activity log (338) was
      the one real case of the suspected cause: its table lives in
      `#activity-list`, and the rule listed the containers it knew by name.
      Website settings (59) was a flex row of a domain and two buttons with no
      `flex-wrap`. Found while measuring: the Markdown cheat sheet is 24px too
      wide when open, which is the only state anybody reads it in.
      *Done: all twenty-nine screens at 0 overflow and 0 unheld boxes. See
      `phases/19-on-the-phone/19-01-VERIFICATION.md`.*
- [ ] **PHONE-02**: **Every control is at least 44 × 44 pixels.** That is the
      figure Apple's guidelines and WCAG 2.2's target-size rule both land on.
      All twenty-nine screens fail it, and the same handful of elements is
      responsible on every one: the hamburger (32 × 32), the brand (26 × 26),
      the user menu (36 × 32), the website switcher's rows (35px) and the
      navigation items. Fix those five and twenty-nine screens improve at once.
      The buttons (`.btn`, `.btn--sm`) and the form fields are the second tier.
- [ ] **PHONE-03**: **Every field a person types into is at least 16px.** Below
      that iOS zooms the page on focus and does not zoom back, which turns one
      tap into a pinch and a scroll. The page editor's own textarea is on the
      list, which is the worst place for it. A radio button or a checkbox is
      not a typing field and is not covered.
- [ ] **PHONE-04**: **The measurement is repeated and committed.** The same
      twenty-nine screens, the same three numbers, beside the first reading. A
      requirement that says "it is better now" without a second measurement is
      an opinion.
- [ ] **PHONE-05**: **What is fixed is driven on a phone-shaped browser once,
      by hand.** QUAL-02's rule, which has found something in every milestone it
      has been applied to: a browser finds what a test cannot. Three interface
      faults came out of it in v2.3 and none of them was subtle.

## What this milestone is not

Not a redesign, not a second stylesheet, not a separate mobile application. The
admin is one set of templates and one stylesheet and stays that way — the
constraint against build tools has held for five milestones and is not being
spent on this.

Not the public themes. A visitor's side of the eight shipped themes is a
different question with a different audience; this is the administration.
