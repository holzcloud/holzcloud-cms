# 19-01 — PHONE-01: no admin screen is wider than the phone

Measured 2026-09-17 at 390 × 844. Before and after, twenty-nine screens.

## What the survey had assumed, and what was actually true

The survey's reading was: *"six of the seven for one reason — `table.table` is
wider than 390px and the `@container (max-width: 600px)` rule that would make
its card scroll never applies."*

The first half is right, the second is not, and the difference matters because
it points at a different repair. Asked in the browser, the container query
**does** apply: `.main-content` carries `container-type: inline-size`, is 390px
wide, the rule matches, `.card` computes `overflow-x: auto`, and the card really
does scroll — `scrollWidth` 636 against `clientWidth` 356 on the Websites
screen. The table was contained the whole time.

What ran out of the card was a **`.sr-only`**. It is `position: absolute`, and
with no positioned ancestor its containing block is the page rather than the
card, so it is not clipped by the card and it lands at its static position —
far out in the wide part of the table. The page then reaches exactly as far
right as that invisible label stands:

| screen | over, before | the `.sr-only`'s right edge |
|---|---|---|
| Users | 275px | 665 |
| Pages | 187px | 577 |
| Languages | 97px | 487 |
| Websites | 50px | 440 |
| Menus | 41px | 431 |

Adding `position: relative` beside the `overflow-x: auto` — one line — took all
five to 0. Nothing else changed.

The sixth table screen, the **Activity log** (338px over), was the case the
survey had described: its table is not a child of the card but of the
`#activity-list` container htmx swaps, and the rule named the containers it
knew one by one — `.card:has(> .table)`, `.card:has(> #page-list)` — and had
never been told about this one. Asking for a descendant instead of a child
covers all three shapes and cannot be forgotten again.

The seventh screen, **Website settings** (59px over), is unrelated to tables:
`.domain-item` is a flex row of a domain name and two buttons with no
`flex-wrap`, so a long name pushed the row — and the button hanging off the
right-hand edge was, of all of them, „Entfernen".

One more was found while measuring rather than in the survey: the **Markdown
cheat sheet** is 24px too wide when it is open, which is the only state anybody
reads it in. It scrolls in itself now; its `code` cells must not break
mid-token, because `![Bild](/media/1/foto.jpg)` is only usable in one piece.

## The measurement

`over` is `documentElement.scrollWidth − documentElement.clientWidth`: how far
the page can be pushed sideways. `wide` counts boxes ending past the edge that
**no** scrolling ancestor holds.

| screen | over before | over after | wide before | wide after |
|---|---|---|---|---|
| Websites | 51 | 0 | 11 | 0 |
| Website settings | 59 | 0 | 2 | 0 |
| Pages | 187 | 0 | 22 | 0 |
| Menus | 41 | 0 | 10 | 0 |
| Languages | 97 | 0 | 8 | 0 |
| Activity log | 338 | 0 | 8 | 0 |
| Users | 275 | 0 | 13 | 0 |
| the other 22 | 0 | 0 | 0 | 0 |

All twenty-nine screens: `over` 0, `wide` 0. The full reading is
`19-01-AFTER.json`, beside the first one in
`.planning/milestones/v2.5-PHONE-SURVEY.txt`.

## What this cost

Four rules, all in `cmd/holzcloud/assets/admin.css`, no template changed:

- `position: relative` on the scrolling card.
- `.card:has(.table)` instead of two named containers.
- `flex-wrap: wrap` and `overflow-wrap: anywhere` on the domain row.
- `overflow-x: auto` on the Markdown cheat sheet.

## The lesson

The survey named a cause from reading the stylesheet, and it was the wrong one
on six screens out of seven. The rule it accused was working. What the reading
could not see is that an element which is *invisible* still has a position, and
that `overflow` clips a descendant only when the descendant's containing block
is inside it. Both of those are questions for the browser, and the browser
answers them in one call.
