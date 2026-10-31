# Context: Phase 17 — What has accumulated

Opened 2026-09-15, the day v2.3 closed.

---

## §1 Why now and not later

The window ledger is empty. That has not been true before — it carried fifteen
entries through v1.10 and v2.0, and two milestones closed on top of them.

An empty ledger is not a clean tree. It is a tree with nothing left that somebody
*wrote down*. This phase looks at what the ledger was never watching: coverage,
comments, exported surface, and two limitations that were recorded at their own
site and never given an entry.

## §2 What the stock-take actually found

`17-INVENTORY.md` has it in full. The short version, and the surprise is at the
top:

**The tree is in better shape than a cleanup milestone assumes.** Zero TODO,
zero FIXME, zero skipped tests, every documented invariant still true — 22
`hx-confirm` attributes, 8 themes, 56 migrations, 1 830 catalogue strings with
nothing open and nothing orphaned. Whoever wrote this did not leave rubbish
behind.

What it did leave is the four things a working tree accumulates without anybody
deciding to:

1. **Coverage where the wiring is.** `internal/admin` at 35.9 % over 171 routes.
   The pure packages are at 90 % and above; the wired ones are not, which is the
   usual shape and not a scandal — but the administration is the surface an
   operator lives on.
2. **Comments that have stopped being true.** 108 cite a file and a line; one
   cites a line that cannot exist; at least one describes a defect that was
   fixed since as though it were open. In a project that puts its reasoning in
   comments instead of a wiki, that is a document that lies.
3. **Exported names nobody outside needs.** 301 of 1 312, as an upper bound.
   The rule this project states for itself is the opposite — `hasAlbumMarker`
   was deliberately kept unexported, with the reasoning written down.
4. **Two limitations with no ledger entry.** A snippet's field values not
   surviving a bundle; an album marker replaced without knowing its context.

## §3 The trap to plan around

**A coverage number is not a goal.** The way to move `internal/admin` from 35.9 %
is to write tests that execute lines, and the way to do that badly is to write
tests that execute lines. What earns its place is a test that would go red if
the screen stopped working — which means driving it red first, which is TEST-03
and is the only thing keeping this phase honest.

The second trap is the mirror of it: **a comment pass that only touches
comments.** Two of the four findings here were produced by a script, and a
script can produce a list of 301 names just as easily as it can produce a fix
that is really a rename. SURF-01 says to look at each one, and means it.

## §4 What v2.3 taught that applies directly

**Measure before choosing.** WORD-04 made it a requirement rather than a habit
and it changed the answer: the obvious mechanism cost 400× the one that shipped.
This phase opens with the measurement already taken, which is the same move one
step earlier.

**A browser finds what a test cannot.** Three interface faults came out of
QUAL-02 in v2.3 and none of them was subtle: a heading printed twice on three
screens, a CSS custom property this project does not define, a link outside the
slot the layout keeps for it. All three were on screens whose handlers were
covered. That is the argument for QUAL-02 in a coverage milestone, not against
it.

**Green locally is not green.** `tools/wasm -check` was red on `main` for two
days and the image build failed on a file that was in every working copy. Both
were caught by something that runs somewhere other than here. QUAL-01 now names
CI and the image build explicitly.

## §5 Shape

Phase 17 takes DOC, SURF and the two GAPs — the work that is measurable and
finishes. The administration under test is large enough to be its own phase and
will be numbered 18 when this one closes.
