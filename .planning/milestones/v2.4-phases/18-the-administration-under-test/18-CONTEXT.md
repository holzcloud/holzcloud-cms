# Phase 18 — The Administration Under Test

Opened 2026-09-17, when phase 17 closed content-complete.

Carries TEST-01, TEST-02 and TEST-03 — the whole of what v2.4 has left.

---

## §1 What is being measured, and from where

| Package | Before | Why it is on the list |
|---|---|---|
| `internal/admin` | **35.9 %** over 171 routes | the largest surface an operator ever touches, and the least covered package that matters |
| `internal/branding` | **0.0 %** | the only package under `internal/` with no test file at all |

Both numbers come from `17-COVERAGE.txt`, measured on 2026-09-15 across 54
packages.

## §2 The trap, restated because it is the whole phase

**A coverage number is not a goal.** The way to move 35.9 % is to write tests
that execute lines, and the way to do that badly is to write tests that execute
lines. What earns a place is a test that would go red if the screen stopped
working.

TEST-03 is the instrument: every test driven red before it is driven green. Not
as ceremony — 18-01 measured what it is worth. Fifteen mutations of
`internal/branding`, and **three tests that were already green passed for the
wrong reason**: `Load` applies the same trim-and-fall-back that `clean` does, so
a `Save` that stored `"   "` still produced the right brand on screen and every
test that went through both was satisfied. The dirty row would have sat in the
database unseen. Two guards are right; tested only through each other, only one
of them was really held.

That is the failure mode a coverage sweep produces by default, and it does not
show up as a low number. It shows up as a high one.

## §3 How a test is driven red here

Three ways, in order of preference:

1. **Revert the fix.** For a test written with a change, the change comes out
   and the test goes red. This is what 17-03 did for both gaps.
2. **Mutate the code under it.** For a test written against code that already
   works — which is all of a coverage phase — a mutation is the only honest
   substitute. 18-01 ran fifteen and recorded them.
3. **Say why not.** A mutation that leaves behaviour identical cannot be
   caught by anything, and calling that a test weakness is a lie. 18-01 has two,
   both measured rather than argued: `> max` → `>= max` slices the whole value
   again at the bound, and removing `WriteLogo`'s empty-folder guard leaves
   `os.MkdirAll("")`, which fails with ENOENT anyway.

## §4 Order

**18-01 — `internal/branding` (TEST-02).** Done. 0.0 % → 95.7 %. Small, self
contained, and the right place to establish the method before applying it to a
package with 171 routes.

**18-02 — `internal/admin` (TEST-01).** The screens an operator uses every day,
driven through `newTestAdmin`. Every test using it today was written for one
specific defect rather than for the screen; this is the other half.

## §5 What 17 handed over

Phase 17 found four things and fixed three of them in place: the citations
(DOC-01), the stale claims (DOC-02/03), the exported surface (SURF-01/02) and
the two recorded limitations (GAP-01/02). The fourth is this phase, and it was
always going to be: it is the only one of the four that cannot be done by a
script and checked by a tool.
