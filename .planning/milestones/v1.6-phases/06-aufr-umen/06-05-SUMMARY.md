---
phase: 06-aufr-umen
plan: 05
subsystem: tooling
tags: [go, archive-zip, reproducible-builds, buildvcs, gotoolchain, sha256, documentation]

requires:
  - phase: 06-02
    provides: "`tools/wasm` with the target table, the `goToolchain` pin, the build function, `-print-hashes`, and the measured D-05 PASS that lets this plan execute as written"
provides:
  - "`go run ./tools/wasm` — write mode: builds the six guests and packs the four archives into the working tree, each installed atomically through a temporary file in the destination's own directory"
  - "`go run ./tools/wasm -check` — compares a fresh build against all ten committed artifacts, prints both hashes, both sizes and both embedded Go versions per mismatch, reports every one before exiting non-zero"
  - "`go run ./tools/wasm -out <dir>` — builds out of tree (the D-05 fallback, implemented although the gate passed)"
  - "deterministic archive packing for `bestellung`, `jahreszahl`, `nicht-gefunden` and `suche`: fixed `Modified` stamp, two flat entries, two runs byte-identical"
  - "`bodenPruefen` — a startup guard comparing `goToolchain` against the root `go.mod` `go` directive, the trap 06-02 flagged as this plan's scope"
  - "`-buildvcs=false` in all four places the guest build command is written down, so a contributor following a README produces bytes the comparison accepts"
affects: [06-06, 06-07, ci]

actuals:
  tokens: 4400
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "One in-memory artifact set feeding every mode: write, -check, -print-hashes and -out consume the same byte slices, so a mode cannot compare something other than what another mode would install"
    - "Deterministic zip: an explicit fixed FileHeader.Modified, one compression method, entries in a fixed order, built in a bytes.Buffer and written once"
    - "A build that never opens its destination: the compiler writes into a temporary directory and the result is installed by rename, so a failed build cannot truncate a committed artifact"

key-files:
  created: []
  modified:
    - tools/wasm/main.go
    - plugins/README.md
    - internal/plugin/testdata/README.md
    - internal/plugin/runtime_test.go

key-decisions:
  - "All four modes are built on one artifact set produced by `erzeugen`, rather than each mode running its own build. What `-check` compares is byte for byte what write mode would install, because it is the same slice — the two cannot drift apart as the tool grows."
  - "The build always happens in a temporary directory, in every mode including the one that writes into the tree; installation is a separate atomic step (temporary file in the destination's own directory, then rename). This satisfies both reasons the plan gave — no cross-filesystem rename, no truncated file — and adds a third: a failed build never touches the tree at all."
  - "Archives are `zip.Deflate`, not `zip.Store`. A wasm guest compresses to under 30 %; storing uncompressed would add ~21 MB to git on top of D-04's rebuild. The price, recorded in a comment beside the packer: archive bytes depend on `compress/flate` — on the toolchain running the tool, not on the pinned one that builds the guests. If a Go release ever changes flate output, the fix is a repack in the commit that raises the version, the same rule the compiler pin already carries."
  - "The fixed archive timestamp is 1980-01-01T00:00:00Z, the earliest instant the zip format can represent. It reads as 'deliberately none' rather than as a date somebody might mistake for provenance — the same reasoning `-buildvcs=false` already applies to the modules."
  - "`bodenPruefen` was added although no task asked for it (Rule 2). 06-02 flagged the `go.mod` floor as this plan's scope; the guard names both versions in one sentence instead of letting `echo` alone fail deep inside a build and turning the comparison half red for a reason unrelated to the guests."
  - "The stale duplicate build command in `runtime_test.go`'s `echoModul` doc comment was removed rather than given the flag. It named neither `-trimpath` nor the linker flags and therefore contradicted the `//go:generate` line three rows above it; the task's own acceptance criterion (three `GOOS=wasip1` lines, all carrying the flag) requires exactly this."

patterns-established:
  - "A report line that names a command a human can run verbatim: the rebuild hint carries the target's positional name, never the artifact label — `go run ./tools/wasm bestellung`, never `… bestellung.zip`"
  - "An exception in a table carries the reason beside it, so it does not read as an oversight (kontaktformular has no archive because migrations/ would be flattened)"

requirements-completed: []
requirements-advanced: [MAINT-03]

coverage:
  - id: D1
    description: "`go run ./tools/wasm` builds the selected guests into the working tree without ever leaving a truncated artifact behind, and `-out <dir>` does the same outside the tree"
    requirement: MAINT-03
    verification:
      - kind: integration
        ref: "go run ./tools/wasm echo => rebuilds only internal/plugin/testdata/echo.wasm (b13b065c… -> 82c0da9e…); git status --porcelain -- plugins empty; restored with git checkout"
        status: pass
      - kind: integration
        ref: "simulated failed build (jahreszahl pointed at tools/, no Go package) => 'no Go files in …', exit non-zero, plugins/jahreszahl/plugin.wasm sha256 b3c00c92… before and after, git status clean"
        status: pass
      - kind: integration
        ref: "go run ./tools/wasm -out $(mktemp -d) => exit 0, 6 modules (10 files once archives landed); git status --porcelain -- plugins internal/plugin/testdata empty"
        status: pass
    human_judgment: false
  - id: D2
    description: "`-check` compares all ten committed artifacts, reports every mismatch with both hashes, both sizes and both embedded Go versions plus the command that fixes it, and exits non-zero"
    requirement: MAINT-03
    verification:
      - kind: integration
        ref: "go run ./tools/wasm -check => 10 'ist nicht aktuell' blocks, exit 1; the six module blocks carry two distinct 64-hex hashes, two sizes and two go1.* versions (go1.24.7 ×4, go1.26.2, go1.26.4 vs go1.26.6)"
        status: pass
      - kind: integration
        ref: "go run ./tools/wasm -check | grep -c 'go run ./tools/wasm' => 11; every hint names a valid positional target"
        status: pass
      - kind: integration
        ref: "go run ./tools/wasm -check | grep -c '\\.zip' => 8 (four archives, each named in its header and its hint line)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Archive packing is deterministic: two runs from the same source produce byte-identical archives, each holding exactly two flat entries, manifest first, module second"
    requirement: MAINT-03
    verification:
      - kind: integration
        ref: "two -out runs into fresh directories; diff -r D1 D2 => no difference across all ten artifacts; cmp per archive => equal"
        status: pass
      - kind: integration
        ref: "unzip -l suche.zip => exactly 2 entries, plugin.json then plugin.wasm, no '/' in either name, dated 1980-01-01"
        status: pass
      - kind: integration
        ref: "unzip of jahreszahl.zip; cmp extracted plugin.wasm against jahreszahl.wasm from the same -out run => identical"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' tools/wasm/main.go | grep -c 'Modified' => 1; plugins/kontaktformular produces no archive in any mode and the reason is a comment beside the table"
        status: pass
    human_judgment: false
  - id: D4
    description: "The tool and the four written build invocations agree flag for flag, so a contributor following any of them produces bytes the comparison accepts"
    requirement: MAINT-03
    verification:
      - kind: other
        ref: "grep -rn 'GOOS=wasip1' plugins/README.md internal/plugin/testdata/README.md internal/plugin/runtime_test.go => 3 lines, 0 of them without -buildvcs=false; flag order -buildmode=c-shared, -trimpath, -buildvcs=false, -ldflags matches tools/wasm/main.go:476-482"
        status: pass
      - kind: unit
        ref: "go test ./internal/plugin/ => ok 22.485s; go vet ./internal/plugin/ => 0; gofmt -l internal/plugin/runtime_test.go => empty"
        status: pass
      - kind: other
        ref: "plugins/README.md names go run ./tools/wasm and go run ./tools/wasm -check and explains in German why the flag is mandatory; internal/plugin/testdata/README.md names go run ./tools/wasm echo"
        status: pass
    human_judgment: false
  - id: D5
    description: "Repository-wide hygiene: the extended tool builds, vets and formats clean, and nothing else in the tree moved"
    requirement: MAINT-03
    verification:
      - kind: other
        ref: "gofmt -l . => empty; go vet ./... => 0; git status --porcelain => only .planning bookkeeping"
        status: pass
    human_judgment: false

duration: 21 min
completed: 2026-09-04
status: complete
---

# Phase 6 Plan 05: `tools/wasm` Complete — Four Modes, Ten Artifacts Summary

**`tools/wasm` now writes, compares, prints and builds out of tree from one shared artifact set, packs the four plugin archives byte-reproducibly, and reports all ten committed artifacts as stale — which is the correct answer until plan 06-06 rebuilds them.**

## What the tool does now

| Mode | Command | Effect |
|---|---|---|
| write | `go run ./tools/wasm [ziel…]` | builds the guests and packs their archives into the tree, atomically |
| compare | `go run ./tools/wasm -check` | ten artifacts, every mismatch reported, non-zero exit |
| report | `go run ./tools/wasm -print-hashes` | ten lines: hash, size, embedded compiler; writes nothing |
| out of tree | `go run ./tools/wasm -out <dir>` | ten files into `<dir>`, tree untouched (D-05 fallback) |

Exactly one mode at a time; anything else is a usage error with exit 2 and a
line naming all four. Positional target names select a subset in every mode, and
a target carries its archive with it.

### The mismatch report, verbatim

```
plugins/jahreszahl/plugin.wasm ist nicht aktuell
  im Repository: b3c00c92bd930a8eadd9da45f7f227712555eca9217e1e29a532c0feb25abccd   3097425 Bytes  gebaut mit go1.24.7
  neu gebaut:    629574a3b612c073f1bb7ea111f605f979cc97820a0cefe278b0b94a99fd7b82   3313021 Bytes  gebaut mit go1.26.6
  Neu bauen mit: go run ./tools/wasm jahreszahl
```

Both embedded Go versions are printed because a mismatch has exactly two causes
— the pin moved, or the source changed — and the block should settle which one
without a second command. Here it settles it immediately: `go1.24.7` against
`go1.26.6` is the compiler drift the phase set out to remove, not a source
change.

The full run reports **ten** such blocks (six modules, four archives) and then
`10 Datei(en) sind nicht aktuell. Neu bauen mit: go run ./tools/wasm`. The
committed hashes match `06-RESEARCH.md`'s measured inventory row for row, and
the fresh hashes match `06-02-SUMMARY.md`'s cross-host set row for row — the
tool did not quietly change what it builds while it grew three modes.

## Accomplishments

- **One artifact set, four modes.** `erzeugen` builds a target and returns its
  artifacts in memory; `jeArtefakt` walks the selection and hands each one to a
  mode-specific function. What `-check` compares is byte for byte what write
  mode would install, because it is the same slice. This is the structural
  answer to the failure the plan warns about — a `-check` that passes while
  checking something other than what gets written.
- **A failed build cannot damage the tree.** The compiler always writes into a
  temporary directory; installation is a second step through a temporary file in
  the destination's own directory followed by a rename. Proven by pointing a
  target at a directory with no Go package: the build fails loudly and
  `plugins/jahreszahl/plugin.wasm` keeps its hash to the byte.
- **Deterministic archives (D-23).** Four archives, two flat entries each,
  `plugin.json` before `plugin.wasm`, `zip.Deflate` throughout, and an explicit
  `Modified` of 1980-01-01T00:00:00Z. Two `-out` runs into fresh directories are
  byte-identical across all ten artifacts (`diff -r` clean). The module inside a
  produced archive is the module produced by the same run, not the file lying in
  the tree — the archive cannot describe a guest other than the one beside it.
- **`kontaktformular` is an exception with a reason attached.** It carries a
  `migrations/` directory that the flat two-entry layout would silently drop, so
  it gets no archive, and the comment beside the table says so. `.gitignore`
  already ignores `plugins/kontaktformular/kontaktformular.zip` for the same
  reason — the tool and the ignore file now agree, and neither reads as an
  oversight.
- **`-out` names files after the target, not the destination's basename.** Five
  of the six destinations are called `plugin.wasm`; basenames would collide in a
  single directory. Archives keep their basename, which is already unique.
- **The `go.mod` floor is a guard now, not a trap** (see below).
- **Four written build invocations, one flag set.** `plugins/README.md`,
  `internal/plugin/testdata/README.md` and the `//go:generate` line all carry
  `-buildvcs=false` in the same position the tool uses, and both READMEs now say
  in German why it is a condition rather than a preference and point at
  `go run ./tools/wasm` as the way not to have to remember it.

## The `go.mod` floor guard — 06-02's inheritance, closed

`06-02-SUMMARY.md` measured that the toolchain pin is genuinely forced on the
build subprocess and then flagged the one real trap next door: if `go.mod`'s `go`
directive is raised above the `goToolchain` constant, `echo` alone fails — it
lives in the root module — deep inside a build, and the comparison goes half red
for a reason that has nothing to do with the guests. It named a startup guard as
this plan's scope.

`bodenPruefen` now runs before any mode does anything: it reads the root
`go.mod`, extracts the `go` directive with one regexp, compares it component by
component against `goToolchain`, and refuses with a single sentence naming both
numbers and the constant to raise. It is deliberately narrow — a missing
directive is not this tool's problem and returns nil — so it cannot turn a clear
error into a vague one. Today `goToolchain = go1.26.6` against `go 1.26.6`: the
floor is satisfied and the guard is invisible.

## Task Commits

| Task | Commit | What |
|---|---|---|
| 1 | `fcaadee` | write, `-check` and `-out`; mode exclusivity; `bodenPruefen` |
| 2 | `2d8ec3e` | deterministic archive packing, wired into all four modes |
| 3 | `6964c8a` | `-buildvcs=false` in the three documents and the generate directive |

## Deviations from Plan

**1. [Rule 2 — missing critical functionality] `bodenPruefen`, the `go.mod` floor guard.**
- **Found during:** Task 1
- **Issue:** No task asked for it, but `06-02-SUMMARY.md` explicitly assigned the guard to "06-05/06-06 to handle", and 06-06 makes the comparison blocking. Without it, a `go.mod` bump produces a five-green/one-red run whose message is about a module the caller never mentioned.
- **Fix:** ~35 lines: read `go.mod`, compare the directive against `goToolchain`, refuse with one German sentence naming both. Narrow by construction — no directive found means no opinion.
- **Files:** `tools/wasm/main.go`
- **Commit:** `fcaadee`

**2. [Rule 1 — bug] The rebuild hint named an artifact label instead of a target.**
- **Found during:** Task 2
- **Issue:** Once archives entered the report, the hint read `go run ./tools/wasm bestellung.zip` — not a valid positional target, so the one command D-11 requires the message to carry would have failed for four of ten artifacts.
- **Fix:** `artefakt` carries the target it came from (`quelle`); the hint uses that. Verified: the six distinct hints are exactly the six target names.
- **Commit:** `2d8ec3e`

**3. [Rule 3 — blocking] The stale duplicate build command in `runtime_test.go` was removed, not amended.**
- **Found during:** Task 3
- **Issue:** The plan says "the only addition is `-buildvcs=false`", but Task 3's acceptance criterion requires `grep -rn 'GOOS=wasip1'` over the three files to return **three** lines. There were four: the `//go:generate` line plus a second, older copy inside `echoModul`'s doc comment that named neither `-trimpath` nor the linker flags.
- **Fix:** Removed the duplicate; the comment now points at `testdata/README.md` and at the generate line above it. Keeping a fourth divergent copy is the exact defect D-02a exists to prevent.
- **Files:** `internal/plugin/runtime_test.go`
- **Commit:** `6964c8a`

**4. [Note, not a code change] `go run` collapses the exit code.**
- **Found during:** Task 1 acceptance
- **Issue:** Two criteria call for exit 2 from `go run ./tools/wasm …`. `go run` prints `exit status 2` and itself exits **1**, for any non-zero child status.
- **Resolution:** Verified against a compiled binary — `go build -o /tmp/wasmtool ./tools/wasm && /tmp/wasmtool -check -out /tmp/x` exits **2**, and so does an unknown target. The program's exit codes are correct; the wrapper hides them. **Plan 06-06 must treat non-zero as the CI signal and must not test for a specific code through `go run`.**

**Total deviations:** 3 code (1 Rule 1, 1 Rule 2, 1 Rule 3) plus 1 recorded observation. **Impact:** none negative; every plan acceptance criterion passes, and two of the three deviations close traps that plan 06-06 would otherwise have inherited.

## Requirements

**MAINT-03 stays `Pending`.** It is declared by `06-01`, `06-02`, `06-05` and
`06-06`; `requirements ready-ids` reports `0/1 requirement(s) ready to mark
complete`, so `REQUIREMENTS.md` was left untouched. MAINT-03 closes when 06-06
rebuilds the artifacts and makes the comparison blocking.

## Issues Encountered

None. No auth gates, no package installs, no architectural decisions.

## Deferred / Out of Scope

- **The rebuild itself and the CI wiring** — plan 06-06, deliberately not done
  here. No `.wasm` and no `.zip` was committed by this plan; `-check` is red by
  design and that redness is the evidence the tool works.
- **The temporary `ci.yml` hash step** from 06-02 is untouched; 06-06 removes it.
- **Archive bytes depend on `compress/flate`**, i.e. on the toolchain running the
  tool rather than on the pinned guest compiler. Documented in a comment beside
  the packer. Low risk — Go has not changed flate output in years — and the
  remedy is the same one the compiler pin already carries: repack in the commit
  that raises the version. Worth a glance when 06-06 makes `-check` blocking.
- **`go build ./tools/wasm` still drops a native `./wasm` binary in the
  repository root**, un-ignored, exactly as `./i18n` and `./mkbundle` do.
  Pre-existing, unrelated, left alone (carried over from 06-02).

## Next Phase Readiness

`06-06` is unblocked and has everything it needs: a tool that rebuilds all ten
artifacts in one command (`go run ./tools/wasm`), a comparison that covers all
ten (`go run ./tools/wasm -check`), and a documented build invocation that agrees
with both. Two things to carry into it: treat **non-zero**, not `2`, as the CI
failure signal, and remember that the artifact-only commit is ten files — six
modules **and** four archives.

## Self-Check: PASSED

Modified files verified on disk: `tools/wasm/main.go`, `plugins/README.md`,
`internal/plugin/testdata/README.md`, `internal/plugin/runtime_test.go`, and this
SUMMARY. Commits `fcaadee`, `2d8ec3e`, `6964c8a` verified in `git log`.
Plan-level verification re-run at close-out: `gofmt -l .` empty, `go vet ./...`
exit 0, `go run ./tools/wasm -check` exit 1 with ten mismatch blocks, two `-out`
runs `diff -r` clean, all three `GOOS=wasip1` lines carrying `-buildvcs=false`,
`go test ./internal/plugin/` ok in 22.485 s.
