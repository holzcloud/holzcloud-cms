# 19-02 and 19-03 — PHONE-02 and PHONE-03

Measured 2026-09-17 at 390 × 844, twenty-nine admin screens.

## PHONE-02: every control at least 44 × 44

44 CSS pixels is the figure Apple's guidelines and WCAG 2.2 SC 2.5.5 arrive at
independently. Below it a fingertip hits the neighbour.

**Hung on `(pointer: coarse)`, not on a width.** The number is about fingers,
not about screens: a phone held sideways is 926px wide and still needs it, a
mouse on a 390px window does not. Measured the same day: the survey's context
and a phone both match `(pointer: coarse)`; a desk does not, so the admin stays
as dense as it is where a pointer is fine.

Every rule is a **minimum**. None changes something already big enough, and
none sets a colour, a spacing or a typeface.

| what | before | after |
|---|---|---|
| `.topbar-hamburger` | 32 × 32 | 44 × 44 |
| `.topbar-brand` | 26 × 26 | 44 × 44 |
| `.topbar-user` | 36 × 32 | 44 × 44 |
| `.switcher-current` | 154 × 31 | 44 tall |
| `.switcher-item` | 226 × 35 | 44 tall |
| `.nav-item` | 233 × 35 | 44 tall |
| `.rowmenu-trigger`, `.rowmenu-item` | 28, 35 | 44 tall |
| `.btn` (with `--sm`, `--danger`, `--primary`) | 28–39 | 44 tall |
| `.form-input` (text, number, date, file, select) | 25–43 | 44 tall |
| `input[type=file]` without the class | 25 | 44 tall |
| `.website-card__name`, `.activity-item__title` | 24, 16 | 44 tall |
| `summary` | 24 | 44 tall |
| tick box and radio | 13 × 13 | 24 × 24, in a 44px row |

Result: **0 controls under the minimum on all twenty-nine screens**, from
13–33 per screen.

### The one place 44 is the wrong number

A tick box the size of a button is not a tick box any more. So it is held to
two figures instead: the box itself to **24 × 24**, which is what WCAG 2.2
SC 2.5.8 sets as the minimum and is nearly four times the area of the measured
13 × 13; and the **row** it shares with its label to 44, because tapping the
label does exactly what tapping the box does.

Two boxes keep only the 24: the row-select boxes in the page list, whose label
is `.sr-only`. There is no row that switches with them. That is written down
here rather than quietly excluded.

### Three faults in the ruler, each found by re-measuring

1. **A `<label>` is a control when it is the only handle.** The hamburger *is*
   one — a label over a 1 × 1 checkbox. Leaving labels out meant the one
   control every screen carries was never measured, while the hidden checkbox
   it drives was reported as too small on all twenty-nine. Counting every
   label was the opposite error: a caption above a text field is not a target,
   the field beneath it is. The rule is: a label counts when the thing it
   drives is hidden.
2. **A minimum a flex row takes back is not a minimum.** With `inline-size:
   24px` set, the measured tick box was 14px wide: it is a flex item in an
   `inline-flex` row and shrank. `flex: none` is what makes the figure hold.
3. **A tick box is not a typing field.** The first survey's own closing note
   said so and left them in the list anyway. They are out.

## PHONE-03: every typing field at least 16px

Below 16px iOS zooms the page when a field is focused and does not zoom back,
which turns one tap into a pinch and a scroll.

After the tick boxes came out of the list, exactly one field was left:
`.form-textarea--tall`, the page editor's own writing box, at `--text-sm` =
14px. Of every field in the admin, the one somebody spends an afternoon in.
It is 1rem now; the fixed character width stays, because an indent has to
remain an indent.

Result: **0 fields under 16px on all twenty-nine screens.**

## One template changed

`field_list.html` said `class="checkbox-label"` — a name no stylesheet defines,
used once, where `.form-check` was the name for the same thing everywhere else.
The screen it is on was the last one left failing PHONE-02. Two names for one
idea is how a rule comes to cover all but one place.

## The whole reading

`19-AFTER.json`, beside the first one in
`.planning/milestones/v2.5-PHONE-SURVEY.txt`. Twenty-nine screens, all 200,
`over` 0, `wide` 0, `small` 0, `tiny` 0.
