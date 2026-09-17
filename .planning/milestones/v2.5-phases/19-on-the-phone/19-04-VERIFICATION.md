# 19-04 — PHONE-04 and PHONE-05

## PHONE-04: the measurement is repeated and committed

The same twenty-nine screens, the same instrument, beside the first reading:

- `.planning/milestones/v2.5-PHONE-SURVEY.txt` — the reading the milestone
  started from.
- `.planning/milestones/v2.5-PHONE-SURVEY-AFTER.txt` — the second reading, in
  the same shape, with the two columns side by side.
- `19-AFTER.json` — the full reading, element by element.

| | before | after |
|---|---|---|
| pixels past the edge, all screens | 1048 | **0** |
| boxes no scrolling ancestor holds | 86 | **0** |
| controls under the minimum | 462 | **0** |
| fields a phone zooms for | 16 | **0** |

The ruler was corrected three times while this ran, each time by measuring
rather than by reading, and each correction is written up where it was made:
`window.innerWidth` grows with the fault; a box a scrolling ancestor holds is
not a box a person has to chase; a `<label>` is a control when it is the only
handle and not when it is a caption. The second reading says what the columns
now mean, so the two are read together rather than compared blind.

## PHONE-05: driven by hand in a phone-shaped browser

QUAL-02's rule, which has found something in every milestone it has been
applied to. Eight things a measurement cannot see, driven at 390 × 844 with
touch:

1. the hamburger, the drawer opening, a navigation entry inside it being
   reachable, and the drawer closing again with the same control;
2. the website switcher's menu staying inside the screen;
3. a row menu inside a table that scrolls in its card — opened on the first
   row and on the **last**, which is the one that opens at the card's bottom
   edge;
4. the editor: the writing box's size and font, the save button being
   uncovered, and text actually arriving when typed;
5. a wide table scrolling inside its card while the page stays put;
6. the domain row that used to hang off the edge;
7. the Markdown cheat sheet with the fold open;
8. a laptop with a touchscreen at 1280px.

It found two things, and both were real.

### The rule that had never run

`.rowmenu-menu { position: static }` sat in `@layer layout` inside the narrow
container query, with a comment explaining exactly why it had to exist: *"in a
scrolling card an opened menu would be cut off."* `.rowmenu-menu { position:
absolute }` sits in `@layer components`. Components comes after layout, and a
later layer beats an earlier one however specific the earlier rule is. The
override had never applied, on any screen, since it was written.

Measured on a phone: `overflow-x: auto` on the card makes `overflow-y` `auto`
as well, and the **last row's** menu reached 22px below the card's edge — it
appeared cut in half, and what is in it is publish, duplicate and delete. The
rule now lives in `@layer components`, where it wins. Both menus are clear of
the card's edge by 302px and 172px.

This is the shape of mistake a test does not see, because nothing fails: the
intent was written down correctly and put in the wrong place.

### A hamburger beside an open drawer

Found in the same pass, in something added an hour earlier by this phase.
`.topbar-hamburger { display: grid }` was written inside `@media (pointer:
coarse)` to centre the icon in its new 44 × 44 box. `display: none` for the
hamburger lives in `@media (max-width: 900px)` in `@layer layout` — so on a
laptop with a touchscreen at 1280px, the components rule won and the drawer
handle appeared next to a sidebar that was already open.

The lesson is narrow and worth keeping: **`(pointer: coarse)` may decide how
big a thing is, never whether it exists.** Anything that decides existence
belongs in the width query. The rule is bound to both now, and the check for
it is step 8 of the pass so the next one cannot be silent.

### The pass itself had two faults, both found by looking at the answer

A tool that reports "nothing found" is worth exactly as much as its questions.
Two of mine were wrong and said "ok" anyway:

- `form button[type="submit"]` found the **sign-out** button inside the folded
  user menu before the editor's save button, and reported it as covered. The
  selector is scoped to the editor's own form now. `login.js` had already
  written this trap down for the click path; it caught me on the query path.
- The row-menu check read `document.querySelector('.rowmenu-menu')` **without
  opening anything** — measuring a closed box and answering nothing. It said
  "on screen, entries 44px" while the real menu was 298px outside the card's
  visible area. It opens the first and the last row now and measures against
  the card.

Both are recorded in the script's own comments.

## The result

`19-PHONE-BROWSER.json` is empty: after the two fixes, the admin driven on a
phone turns up nothing.
