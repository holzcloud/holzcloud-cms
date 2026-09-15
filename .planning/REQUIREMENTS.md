# Requirements: v2.4 — What Has Accumulated

Milestone opened 2026-09-15, the same day v2.3 closed.

**Not a feature milestone, and deliberately so.** Three milestones in a row
changed what the program says; this one changes nothing an operator can see. It
exists because the stock-take in `17-INVENTORY.md` found four things that a
working tree accumulates quietly, and because the window ledger is empty for the
first time — which is exactly the moment to look at what the ledger was never
watching.

What it is **not**: a rewrite, an architecture change, or a hunt for defects that
nobody has met. Every requirement below names a number that is different at the
end, or a claim that is true at the end and is not true now.

The measurement it rests on is `phases/17-what-has-accumulated/17-INVENTORY.md`,
with the raw coverage output committed beside it.

---

## The administration under test

- [ ] **TEST-01**: **`internal/admin` is materially better covered than 35.9 %.**
      It is the largest surface an operator ever touches — 171 routes — and the
      least covered package that matters. The target is not a number for its own
      sake: it is that the screens an operator uses every day are driven by a
      test that would notice if they stopped working. `newTestAdmin` already
      makes this possible; every test using it today was written for one
      specific defect rather than for the screen.
- [ ] **TEST-02**: **`internal/branding` has tests.** It is the only package
      under `internal/` with no test file at all. It decides what an operator's
      own installation looks like, which is not nothing.
- [ ] **TEST-03**: **Each new test is driven red before it is driven green.**
      A test written after the code, against code that already works, proves
      only that it runs. The habit this project already has for fixes applies to
      coverage work as well, and where a test cannot be driven red, it says so
      and says why.

## The comments, which are this project's documentation

- [ ] **DOC-01**: **No comment cites a line that does not exist.** One does
      today: `internal/admin/csvimport.go:1187` points at `media.go:377`, in a
      file of 322 lines. There are 108 such citations in the tree.
- [ ] **DOC-02**: **No comment describes a defect as open that has been fixed.**
      At least one does: `internal/csvimport/verdict.go:104` says `field.Check`'s
      reasons are hard-coded German and invisible to `tools/i18n`. They go
      through `i18n.N` and have for some time. A reader who believed that comment
      would re-do work that is done.
- [ ] **DOC-03**: **The citations are checked mechanically from here on, or the
      reason they cannot be is written down.** A comment that rots silently is
      worse than no comment, and this repository puts its reasoning in comments
      rather than in a wiki. If a tool can hold the file-and-line form, it
      should; if it cannot hold whether a citation still points at what it
      claims, that limit belongs in the tool's own documentation.

## The exported surface

- [ ] **SURF-01**: **Every exported identifier in `internal/` is exported
      because something outside its package needs it.** 301 of 1 312 are used
      nowhere else. That number is an upper bound, not a finding — it cannot see
      a field an `html/template` reads, a test in another package, or a contract
      somebody else writes against — so the work is to look at each one and
      either use it, un-export it, or record why it stays.
- [ ] **SURF-02**: **The count is reported rather than recounted by hand.**
      Whatever the number ends at, the next person should be able to produce it
      in one command instead of writing the script again.

## The two limitations that are recorded and still true

- [ ] **GAP-01**: **A snippet's image, reference and term fields survive a
      bundle, or the limit is in the ledger and in front of the operator.**
      `internal/bundle/import.go:1008`. Ids are translated on the page path and
      not on the snippet path, so on the other machine the value is refused and
      the field arrives with its picture missing. Today the operator finds out by
      looking.
- [ ] **GAP-02**: **An album marker is replaced knowing whether it stands in
      text or in an attribute, or the limit is in the ledger.**
      `internal/block/render.go:494`. Recorded rather than fixed because the fix
      is a change to the mechanism; reachable only by an authenticated editor on
      their own website. Either it is fixed or it is an entry with a reason a
      reader can check — what it must not stay is a comment nobody counts.

## Standing gates

- [ ] **QUAL-01**: `go run ./tools/i18n` reports `0 open, 0 orphaned` on every
      catalogue; `tools/english`, `tools/themewords -check` and
      `tools/wasm -check` stay green, and CI and the image build stay green on
      `main`.
- [ ] **QUAL-02**: Every screen touched is driven once through the running
      application. The blast radius of a coverage milestone is small by design,
      but a test that passes against a handler nobody ran is the exact thing
      this requirement exists for.
- [ ] **QUAL-03**: **The suite does not get slower than it is useful.** It runs
      in about seven minutes today. Coverage work adds tests by definition, and
      a suite nobody waits for is a suite nobody runs.
