# 17-INVENTORY — what has accumulated, measured

Taken 2026-09-15 against `eb596ea`, the commit `v2.3` names. Nothing here is a
proposal; it is the stock-take a cleanup milestone has to start from, the way
13-00 and 16-00 started theirs.

**430 Go files, 120 953 lines, 1 367 test functions, 54 packages with a
measured coverage, suite green.**

---

## §1 What is NOT wrong

Worth stating first, because a cleanup milestone that only lists faults gives a
false picture of the tree it is cleaning.

- **Zero** `TODO`, `FIXME`, `XXX` or `HACK` in the entire Go source. Not one.
- **Zero** skipped tests. The single `t.Skip` is conditional and legitimate
  (a changelog with one release has no second one to choose between).
- The window ledger is empty: 0 open, 25 fixed, 9 waived.
- Every standing gate is green, and the two that were red this week —
  `tools/wasm -check` and the image build — are green and now guarded.
- The documented invariants still hold: 22 `hx-confirm` attributes, exactly as
  CLAUDE.md asserts; 8 shipped themes; 56 migrations; every catalogue at 1 830
  strings with 0 open and 0 orphaned.

## §2 Test coverage, by package

The whole suite, instrumented. 54 packages.

**The bottom, and it is where the operator is:**

| package | coverage | what it is |
|---|---:|---|
| `internal/branding` | **0.0 %** | no test file at all |
| `internal/kind` | 28.9 % | content kinds |
| `cmd/holzcloud` | 32.2 % | wiring, `main`, the router |
| **`internal/admin`** | **35.9 %** | **the whole administration — 171 routes** |
| `internal/menu` | 36.7 % | menus |
| `internal/i18n` | 44.8 % | the catalogue itself |
| `internal/user` | 54.2 % | accounts |
| `internal/public` | 56.0 % | what a visitor gets |

**The top:** `internal/tmplspec` and `internal/jobs` at 100 %, `design` 95.7 %,
`textdiff` 95.6 %, `structured` 95.5 %, `payrexx` 94.9 %, `locale` 94.8 %,
`totp` 92.9 %, `money` 92.5 %, `changelog` 92.5 %, `wxr` 92.2 %,
`csvimport` 90.6 %.

The shape is clear and it is the usual one: **the small, pure packages are
covered and the big, wired ones are not.** `internal/admin` is the largest
surface an operator ever touches and the least covered of the packages that
matter. That is not an accident of neglect — it is hard to test a handler that
needs a database, a session and a template — but `newTestAdmin` exists and makes
it possible, and every test that uses it today was written for one specific
defect rather than for the screen.

The raw output is `17-COVERAGE.txt` beside this file.

## §3 Two functions that are not functions

| | lines | |
|---|---:|---|
| `main` | **573** | `cmd/holzcloud/main.go:71` |
| `newRouter` | **513** | `cmd/holzcloud/main.go:788` |

Both are wiring and neither has a decision in it that a test could catch, which
is why `cmd/holzcloud` sits at 32 %. `newRouter`'s own doc comment says why it
exists at all — *"the missing requireAdmin on website deletion shipped unnoticed
precisely because there was no test here that could see the route table"* — so
the route table is already understood as the thing worth holding. It is held by
`TestEveryAdminRouteIsClassified`, which caught the two changelog routes this
week.

The five largest files: `internal/admin/csvimport.go` 1 372,
`cmd/holzcloud/main.go` 1 327, `internal/page/store.go` 1 277,
`internal/bundle/import.go` 1 238, `internal/field/field.go` 1 140.

## §4 Exported surface

**1 312** exported identifiers in `internal/`. **301** of them are used nowhere
outside their own package.

That number is an **upper bound and not a finding**: it counts Go source only,
so it cannot see a field an `html/template` reads by name, a symbol a test in
another package uses, or a type that is exported because it is part of a
contract somebody else writes against (`plugin.Manifest`, `sdk`). A real pass
would have to look at each one.

What it does say is that there is a real question here worth an afternoon: this
project's own rule is that a name is exported when somebody outside needs it,
and `hasAlbumMarker` was deliberately un-exported for exactly that reason, with
the reasoning written down. 301 is the size of the drift from that rule.

## §5 The comments, which in this project are the documentation

This repository puts its reasoning in comments rather than in a wiki, so a
comment that has stopped being true is not a cosmetic problem — it is a document
that lies to the next reader.

- **108** comments cite a `file.go:line`.
- **1** of those cites a line that cannot exist: `internal/admin/csvimport.go:1187`
  points at `media.go:377`, and that file has 322 lines.
- The other 107 point at lines that exist. Whether they point at what they
  *claim* has not been checked, and cannot be checked mechanically.
- At least **one** comment describes a defect that has since been fixed as
  though it were still open: `internal/csvimport/verdict.go:104` says
  `field.Check`'s reasons are *"hard-coded German and invisible to tools/i18n
  today"*. They go through `i18n.N` via `reasonf` and have done for some time.

## §6 Two limitations that are recorded and still true

Both are written down at their own site, which is this project's habit, and
neither has a ledger entry.

1. **A snippet's image, reference and term fields do not survive a bundle.**
   `internal/bundle/import.go:1008`. Ids are translated into filenames and
   addresses on the *page* path and not on the snippet path, so on the other
   machine the id belongs to a different website, `fieldImages`/`fieldRefs`
   refuse it, and the field arrives with its picture missing. The value travels
   so nothing disappears, and the operator has to choose again.

2. **An album marker is replaced without knowing whether it sits in text or in
   an attribute.** `internal/block/render.go:494`. Recorded rather than fixed
   because the fix is a change to the mechanism. Reachable only by an
   authenticated editor on their own website.

## §7 One package with no test at all

`internal/branding`. Everything else under `internal/` has at least one test
file.
