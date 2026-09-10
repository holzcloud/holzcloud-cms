---
phase: 06-aufr-umen
reviewed: 2026-09-04T00:00:00Z
depth: standard
range: ef82dfe..HEAD
files_reviewed: 12
files_reviewed_list:
  - tools/wasm/main.go
  - tools/i18n/main.go
  - tools/i18n/main_test.go
  - internal/plugin/wasmtest/wasmtest.go
  - internal/plugin/runtime_test.go
  - internal/plugin/sdk_e2e_test.go
  - internal/plugin/hofladen_e2e_test.go
  - internal/public/formular_e2e_test.go
  - internal/public/suche_e2e_test.go
  - .github/workflows/ci.yml
  - .github/workflows/security.yml
  - .github/workflows/release.yml
  - docs/offene-punkte.md
  - plugins/README.md
  - internal/plugin/testdata/README.md
findings:
  critical: 0
  high: 3
  medium: 6
  low: 9
  total: 18
status: issues_found
severity_mapping:
  blocker: [H-01, H-02, H-03]
  warning: [M-01, M-02, M-03, M-04, M-05, M-06]
---

# Phase 06 (Aufräumen): Code Review Report

**Reviewed:** 2026-09-04
**Depth:** standard, with targeted execution of the new tool
**Range:** `ef82dfe..HEAD`
**Status:** issues_found

## Summary

The phase does what it set out to do: `tools/wasm -check` reproduces all ten
committed artifacts byte-for-byte on this machine (verified, exit 0), the zip
packer is genuinely deterministic, the i18n rewrite is byte-identical against
all seven real catalogues, and none of the five promoted call sites inverted its
condition. Four of the seven concerns raised for targeted attention are
**refuted** — see the section at the end; that is a real result and it is stated
explicitly rather than left silent.

What the phase did **not** get right is the one thing everything else rests on:
the environment control in `tools/wasm/bauen`. Two build-affecting variables are
missing from the filter list, and — worse — the mitigation chosen for the two
that *are* listed (`GOFLAGS=`, `GOEXPERIMENT=`) does not do what the comment
claims it does, because `cmd/go` treats an empty environment value as *unset*
and falls through to the `go env -w` configuration file. Both were reproduced
with measured hashes below. Every one of these produces the exact failure the
tool's own doc comment says must not happen: "the comparison is red for a reason
nobody caused" — or, in the write direction, non-canonical bytes committed into
the repository.

A third structural problem: `packen` hardcodes a two-entry archive while
`internal/plugin/package.go` accepts four kinds of entry. The one plugin that
would expose this today was excluded by hand instead of guarded, so the next
plugin that grows an `assets/` or `migrations/` directory gets a silently lossy
archive that the new blocking CI gate then locks in.

---

## HIGH

### H-01: `GOWASM` and `GOFIPS140` are not in the controlled-variable list

**File:** `tools/wasm/main.go:142` (`var gesteuert`), used at `tools/wasm/main.go:486-503`

**Issue:** `gesteuert` lists `GOOS GOARCH CGO_ENABLED GOTOOLCHAIN GOFLAGS
GOEXPERIMENT`. Two further variables in the running toolchain change the
produced bytes and are passed straight through to the child:

Measured on this machine (`go build -o wasmtool ./tools/wasm`, then
`./wasmtool -print-hashes echo`, all with the same source and the same pinned
`GOTOOLCHAIN`):

| environment | sha256 (prefix) | size |
|---|---|---|
| clean | `82c0da9e5b9dbca8…` | 3 253 624 |
| `GOWASM=satconv,signext` | `62ad3aa2ced6d608…` | 3 253 650 |
| `GOFIPS140=latest` | `8310af5a42d06ae6…` | 3 253 706 |

**Failure scenario:** a contributor with `GOWASM` or `GOFIPS140` exported (both
are legitimate, documented Go settings; `GOFIPS140` is a first-class `go env`
key in this toolchain — `go env GOFIPS140` returns `off`) runs
`go run ./tools/wasm -check` and gets six red targets with no explanation, or
runs the default mode, commits the differing modules, and turns CI red for
everyone else. `GOWASM` is the acute case: it is *specifically* the wasm knob,
and every one of these six targets is a wasm build.

**Missing:** `GOWASM` and `GOFIPS140` in `gesteuert`, and pinned to explicit
values in `cmd.Env` alongside `GOOS`/`GOARCH` — see H-02 for why "set to empty"
is not sufficient. A test asserting that `gesteuert` covers every variable the
build reads would keep the list from drifting again.

### H-02: filtering the process environment does not neutralise `go env -w`; `GOFLAGS=` is a no-op against it

**File:** `tools/wasm/main.go:486-503`, comment at `tools/wasm/main.go:136-141` and `498-500`

**Issue:** Two separate defects in the same block.

1. `cmd/go` reads its configuration from *two* places: the process environment
   and the env file (`$GOENV`, defaulting to `os.UserConfigDir()/go/env` —
   written by `go env -w`). The tool only touches the first. `GOENV` itself is
   not in `gesteuert` and is not set.
2. `cfg.Getenv` in `cmd/go` returns the process value only when it is
   **non-empty**, and otherwise falls back to the env file. So
   `"GOFLAGS="` and `"GOEXPERIMENT="` at lines 501-502 do not override a
   `go env -w GOFLAGS=…`; they leave it fully in force. The comment at
   lines 498-500 ("Emptied rather than inherited: a GOFLAGS or GOEXPERIMENT in a
   contributor's shell changes the produced bytes") asserts a protection the
   code does not provide.

Reproduced (same prebuilt tool, same target `echo`):

| environment | sha256 (prefix) | size |
|---|---|---|
| clean | `82c0da9e5b9dbca8…` | 3 253 624 |
| env file containing `GOFLAGS=-tags=zzz` | `ed821c3ea7877352…` | 3 253 642 |
| env file containing `GOWASM=satconv,signext` | `62ad3aa2ced6d608…` | 3 253 650 |

The reproduction points `GOENV` at a scratch file, which is the same code path
`cmd/go` uses for the default `go env -w` location; nothing about the fallback
differs between the two.

**Failure scenario:** `go env -w GOFLAGS=-mod=mod` (a common workaround a
contributor keeps for months and forgets) silently changes every guest module
this tool builds. `-check` goes red on their machine for a setting they cannot
see in `env`, and the tool's diagnostic — which names the two hashes and the two
embedded Go versions — reports *identical* Go versions, pointing them at "the
source changed" when neither cause applies.

**Missing:** `GOENV=off` in the child environment (which disables the env file
outright), plus explicit non-empty values for anything that must be neutral.
Neither the code nor the doc comment acknowledges the env file exists.

### H-03: `packen` silently drops `assets/` and `migrations/` from a plugin archive

**File:** `tools/wasm/main.go:345-370` (`packen`), entry list at `348-354`; format owner at `internal/plugin/package.go:105-129` and `internal/plugin/manifest.go:31-40`

**Issue:** `packen` writes exactly two entries, `plugin.json` and `plugin.wasm`.
The package format `ReadPackage` accepts four kinds of entry: the manifest, the
module, `assets/…` (`MaxAssetBytes`), and `migrations/…` (`MigrationDir`,
applied at install time by `internal/plugin/store.go:376-412`). `packen` never
looks at the plugin directory; it cannot know that anything else is there.

The comment at `tools/wasm/main.go:93-98` shows the authors knew: `kontaktformular`
is excluded *by hand* because it carries `migrations/`, "which the flat two-entry
layout of the other four would silently drop". That is a correct diagnosis with
the wrong remedy — the hazard was documented as an exception instead of guarded.

**Failure scenario:** someone adds `plugins/suche/assets/suche.css` or
`plugins/suche/migrations/0001_index.sql`. `go run ./tools/wasm` overwrites the
committed, correct `plugins/suche/suche.zip` with a version missing that file.
Nothing errors. `-check` then reports the lossy archive as *aktuell* and, from
the next commit onwards, the blocking CI gate enforces it. An operator installing
`suche.zip` through the admin gets a plugin whose stylesheet 404s, or whose table
was never created — and the plugin source next to it looks correct.

**Missing:** either a scan of the plugin directory that packs everything the
format admits, or a hard refusal when the directory contains an entry `packen`
cannot represent. Also missing: any test that feeds a packed archive back
through `plugin.ReadPackage` and asserts the result matches the source directory
— which would have caught this and is cheap, since both packages are in this
module.

---

## MEDIUM

### M-01: a failed build leaves the working tree partially rewritten

**File:** `tools/wasm/main.go:272-291` (`jeArtefakt`), `373-382` (`inDenBaum`); comment at `267-271`

**Issue:** `jeArtefakt` calls `fn` for each artifact *inside* the per-target
loop. In the default mode `fn` is the tree write. If target 4 of 6 fails to
compile, targets 1-3 (and their archives) have already been replaced on disk and
the command exits 1.

The comment at 267-271 claims the temp-directory build means "a build that fails
then cannot leave a truncated file behind" — true, and a different property from
the one a reader will take away. Individual files are atomic
(`atomarSchreiben`); the *set* is not.

**Failure scenario:** an SDK change breaks `plugins/suche`. `go run ./tools/wasm`
rewrites bestellung, jahreszahl, kontaktformular and their zips, then fails.
`git status` now shows five modified binaries mixed in with the source change,
and the obvious recovery (`git checkout -- plugins/`) also discards work.

**Missing:** collect every artifact first, write only when all targets built —
`jeArtefakt` already holds them in memory, so this is a reordering, not a
redesign. Or, at minimum, a comment that says the set is not atomic.

### M-02: `ziele` is a hardcoded allowlist with no cross-check against `plugins/`

**File:** `tools/wasm/main.go:99-106`

**Issue:** The six targets are a literal table. Nothing compares it against the
contents of `plugins/`. The CI gate at `.github/workflows/ci.yml:67` therefore
covers exactly the plugins somebody remembered to add to this slice.

**Failure scenario:** `plugins/kalender/` is added in a later phase with a
committed `plugin.wasm`. `-check` stays green forever, `plugins/kalender/plugin.wasm`
goes stale against its own source, and the phase's central claim — "the committed
file and its source are comparable again" — is quietly false for that plugin.
The failure is invisible: green CI, no warning, no listing of what was skipped.

**Missing:** a check that every directory under `plugins/` containing a
`plugin.json` appears in `ziele`, failing loudly when one does not.

### M-03: `tools/wasm` has no tests, though it is now a blocking CI gate

**File:** `tools/wasm/` (544 lines, no `_test.go`)

**Issue:** `go test ./tools/...` reports `? github.com/holzcloud/holzcloud-cms/tools/wasm [no test files]`.
Both sibling commands have tests — `tools/i18n/main_test.go` was added in this
same phase, and `tools/mkbundle/pack_test.go` predates it. The untested command
is the one that can block every push and the one that overwrites committed
binaries.

Several units are pure and trivially testable: `kleiner`/`zahlen` (version
comparison, see L-06 for a defect in them), `waehlen` (unknown and duplicate
target names), `bodenPruefen` (the go.mod floor), `beschreiben`, and — most
valuably — `packen`, where a test could assert both byte-stability across two
calls and that `plugin.ReadPackage` round-trips the result (H-03).

**Missing:** a `tools/wasm/main_test.go` in the shape of the one this phase
wrote for `tools/i18n`.

### M-04: three documented build routes produce bytes the new blocking gate rejects

**Files:** `internal/plugin/runtime_test.go:19` (`//go:generate`), `plugins/README.md:47-48`, `internal/plugin/testdata/README.md:7`

**Issue:** All three were updated in this phase to add `-buildvcs=false`, which
was the known blocker. None of them pins `GOTOOLCHAIN`, clears `GOFLAGS`/
`GOEXPERIMENT`/`GOWASM`, or produces a deterministic archive. `plugins/README.md:48`
still documents `zip -j jahreszahl.zip plugin.json plugin.wasm` as the packing
step — `zip -j` stamps the file's current mtime and uses its own deflate, so the
archive it produces cannot match `packen`'s fixed 1980-01-01 header. The tool's
own comment at `tools/wasm/main.go:109-110` states the committed archives are
"what the `zip -j` line in plugins/README.md produces", which is true of the
*contents* and false of the *bytes* that CI now compares.

**Failure scenario:** a contributor follows `plugins/README.md` verbatim on a Go
1.25 machine, commits, and CI reports four red targets. The README frames
`go run ./tools/wasm` as a convenience ("Wer den Schalter nicht im Kopf behalten
will"), not as the only route that satisfies the gate.

**Missing:** the manual commands should be marked as illustrative-only, or
removed in favour of the tool. The `//go:generate` line has the same problem and
carries a comment that half-acknowledges it without saying "this will not match".

### M-05: `t.Helper()` was deleted from `echoModul`, misattributing every skip and failure

**File:** `internal/plugin/runtime_test.go:27-29`

**Issue:** The pre-change `echoModul` called `t.Helper()`. The rewrite dropped
it and delegates to `wasmtest.Modul`, which does call `t.Helper()` on itself.
Because `echoModul` is not marked, the testing package stops unwinding there:
every skip and every failure from a missing `testdata/echo.wasm` is reported at
`runtime_test.go:28`, regardless of which of the many tests in the package
called it.

`internal/public/formular_e2e_test.go:158` kept its `t.Helper()` on the
equivalent wrapper, so the two indirect call sites now behave differently.

**Failure scenario:** on a runner, `HOLZCLOUD_TEST_REQUIRE_WASM=1` with the
module absent produces N identical failures all pointing at one helper line —
exactly the diagnostic loss the shared helper was introduced to prevent.

**Missing:** `t.Helper()` as the first statement of `echoModul`.

### M-06: the package-format constants are duplicated instead of imported

**File:** `tools/wasm/main.go:111-114` versus `internal/plugin/manifest.go:31,34`

**Issue:** `manifestName = "plugin.json"` and `modulName = "plugin.wasm"` are
redeclared in `tools/wasm`, while `plugin.ManifestName` and `plugin.ModuleName`
are exported from the package that *defines* the format and are importable from
`tools/` (same module). Two sources of truth for the same contract, in a tool
whose entire purpose is to stop a generated artifact from drifting from its
source. Related to H-03: the tool also silently lacks `plugin.MigrationDir` and
the `assets/` prefix.

**Missing:** import the constants from `internal/plugin`.

---

## LOW

### L-01: `-out=""` silently falls through to the tree-writing mode

**File:** `tools/wasm/main.go:152-156` and `183`

Mode selection tests `*out != ""`. `go run ./tools/wasm -out=""` (or `-out ""`,
or a shell variable that expanded to nothing) is therefore indistinguishable
from no flag at all, and overwrites the committed artifacts instead of erroring.
Missing: reject an explicitly-supplied empty `-out` via `flag.Visit`.

### L-02: `HOLZCLOUD_TEST_REQUIRE_WASM=0` and `=false` mean "required"

**File:** `internal/plugin/wasmtest/wasmtest.go:47`

The non-empty test is justified in the package comment (lines 13-16) for the
`true`/`yes` direction, and that reasoning is sound. It does not cover the
opposite direction: `0`, `false` and `no` are the natural ways to turn a flag
off, and all three turn it *on*. Someone reproducing a runner failure locally
and then trying to disable it again gets the strict behaviour. Missing: either
handle the false-y spellings, or say in the comment that they are deliberately
not handled.

### L-03: every read error is reported as "fehlt"

**File:** `internal/plugin/wasmtest/wasmtest.go:43-50`

`os.ReadFile` failing for `EACCES`, `EISDIR` or an I/O error produces
"`%s` fehlt" and the "build it with `go run ./tools/wasm`" hint, which is wrong
advice for all three. The underlying error *is* printed, so this is a message
problem rather than a lost signal. Missing: distinguish `errors.Is(err, fs.ErrNotExist)`
from other errors, as `vergleichen` already does at `tools/wasm/main.go:421`.

### L-04: interrupted writes leave untracked dot-files inside `plugins/`

**File:** `tools/wasm/main.go:451-469`, `.gitignore`

`atomarSchreiben` creates `.plugin.wasm.tmpNNNNN` next to the destination.
`SIGINT` during a build leaves these behind in `plugins/*/` and
`internal/plugin/testdata/`, and none of the `.gitignore` patterns matches them.
Missing: a `.gitignore` entry, or a cleanup sweep at start-up.

### L-05: `tools/wasm` mixes English and German in user-facing output

**File:** `tools/wasm/main.go:176`, `379`, `423-441` (German) versus `197 (line 203: "unknown target %q")`, `529-538`, `542` (English)

The project convention is English comments in `tools/`, German on stdout.
`tools/i18n` follows it. `tools/wasm` reports results in German ("aktuell",
"ist nicht aktuell", "Datei(en) sind nicht aktuell") but prints its usage block,
`unknown target %q` and the `error:` prefix in English. Missing: pick one for
the user-facing surface; the German report lines are already the majority.

### L-06: `zahlen` discards `strconv.Atoi`'s error, so a pre-release pin is rejected with a wrong reason

**File:** `tools/wasm/main.go:257-265`, used by `kleiner` at `240-255` and `bodenPruefen` at `230`

`strconv.Atoi(t)` errors are dropped and the component becomes 0. Setting
`goToolchain = "go1.27rc1"` yields `[1, 0]`, which compares below the go.mod
directive `1.26.6`, so `bodenPruefen` refuses to run and reports "die feste
Werkzeugkette go1.27rc1 ist älter als die Vorgabe »go 1.26.6«" — a statement
that is the opposite of the truth. Missing: return an error from `zahlen`, or
handle the `rcN`/`betaN` suffix explicitly.

### L-07: `docs/offene-punkte.md` contradicts itself and breaks its own wrap

**File:** `docs/offene-punkte.md:116` versus `:140`

Line 116 now says migrations run to `00045` (correct — 45 files, last is
`00045_pages_locale_unique.sql`). Ten lines later, the relocated Dependabot
paragraph says "Bei 1.57.0 geprüft: 44 Wanderungen, 48 Tabellen". They describe
different moments in time but sit in the same section and read as a
contradiction. Line 140 is also 103 columns, against a file that otherwise wraps
near 80 — the paragraph move re-joined it. Missing: date the measurement, and
re-wrap.

### L-08: `writeCatalog` truncates in place, and CI now runs `-write` on every push

**File:** `tools/i18n/main.go:317`

`os.WriteFile` truncates before writing. An interrupted run destroys a
catalogue. This is pre-existing behaviour, but two things changed around it in
this phase: `tools/wasm` in the same commit range went to the trouble of writing
through a temp file and renaming (`atomarSchreiben`), and
`.github/workflows/ci.yml:79-80` now runs `-write` and `-schweiz` on every push,
so the mutating path executes far more often than before. Missing: the same
write-then-rename treatment `tools/wasm` gives its artifacts.

### L-09: the new environment variable is documented only inside `.planning/`

**Files:** `internal/plugin/wasmtest/wasmtest.go:10-16`, `.github/workflows/{ci,security,release}.yml`

Outside the three workflow files and the package doc comment, the only
description of `HOLZCLOUD_TEST_REQUIRE_WASM` is in `.planning/codebase/TESTING.md`
— a planning artifact, not developer documentation. `docs/offene-punkte.md`
gained a paragraph about `tools/wasm` in this phase but says nothing about the
variable. Missing: one line where a contributor debugging a CI-only test failure
would look.

---

## Concerns raised for targeted attention — verdicts

Four of the seven are **refuted**. Stated explicitly, because a refuted concern
is a result.

1. **`tools/wasm` correctness — subprocess errors, false `-check` match, temp
   cleanup.** *Refuted for these three.* `bauen` returns `cmd.Run()`'s error
   directly and streams the compiler's own stderr through
   (`tools/wasm/main.go:504-507`); every caller wraps it with the target name and
   propagates. `-check` cannot report a false match: it compares the *same*
   in-memory slice the default mode would write (`artefakt.inhalt`, doc comment
   at `125-127` is accurate), it treats a missing file as a difference rather
   than a crash (`421-428`), and it reports all mismatches before exiting
   (`vergleichen` returns a count, `main` exits 1 at `175-178`). Temp cleanup is
   correct on both levels: `defer os.RemoveAll(tmp)` at `277` and
   `defer os.Remove(name)` at `457`. All required build flags are present at
   `475-503`, including `-buildvcs=false` and the pinned `GOTOOLCHAIN`. **Not
   refuted:** the partial-write property (M-01) and everything under H-01/H-02.

2. **Environment construction.** *Confirmed as a defect, twice over* — see H-01
   and H-02. The *technique* is sound and was verified working for the variables
   it covers: `GOOS=linux GOARCH=amd64 CGO_ENABLED=1 GOTOOLCHAIN=go1.25.0
   ./wasmtool -print-hashes echo` and `GOFLAGS=-tags=foo ./wasmtool -print-hashes
   echo` both reproduced the clean-environment hash exactly
   (`82c0da9e5b9dbca8…`). The list is what is wrong, not the mechanism — and the
   `GOFLAGS=`/`GOEXPERIMENT=` half of the mechanism is a no-op against the env
   file.

3. **Determinism of the zip packing — was `mkbundle`'s `now()` stamp inherited?**
   *Refuted.* `packen` sets `Modified: archivZeit` (1980-01-01 UTC,
   `tools/wasm/main.go:123, 356`), fixes the entry order (manifest then module,
   `348-354`), and uses `zip.Deflate` consistently. `tools/mkbundle/main.go:346`
   uses the bare `zw.Create`, which is the mistake — it was not copied. Proven
   end to end: `go run ./tools/wasm -check` reported all four archives *aktuell*
   against the committed files, i.e. a fresh pack reproduced them byte for byte.
   The residual exposure is honestly documented in the comment at `338-344`: the
   archive bytes depend on `compress/flate` in whichever toolchain runs *this*
   tool, which is not the pinned one. That is a real gap but a disclosed one.

4. **`go run` collapsing the child exit code — does CI test for `== 2`?**
   *Refuted.* `.github/workflows/ci.yml:67` is `run: go run ./tools/wasm -check`
   with no exit-code comparison anywhere, so GitHub Actions fails the step on any
   non-zero status. No `== 2` test exists in any of the four workflows. The
   `usage()` exit-2 path (`tools/wasm/main.go:538`) is unreachable from the CI
   invocation regardless.

5. **`wasmtest` placement and behaviour.** *Partly confirmed.* Placement is
   correct and justified (a non-test package is genuinely required for five call
   sites across three packages; nothing shipped imports it — verified, the only
   five importers are `_test.go` files). Both branches name
   `go run ./tools/wasm` through the shared `bauhinweis` constant, so they cannot
   drift — good. The env-var reading is defensible but has an unhandled
   direction (L-02), the error message over-claims (L-03), and the executor's own
   flag about the missing `t.Helper()` is **correct and confirmed** (M-05) — it
   is `echoModul` in `runtime_test.go`, and the `t.Helper()` was actively deleted
   by this diff.

6. **The i18n writer — can `encoding/json` change output bytes?** *Refuted for
   every realistic input.* Verified directly:
   - **Key ordering:** `encoding/json` sorts map keys byte-wise, identical to the
     deleted `sort.Strings`. Probed with `{"B","a","Ä","0"}` → emitted `0, B, a, Ä`.
   - **HTML escaping:** `SetEscapeHTML(false)` survives the subsequent
     `json.Indent`; `<`, `>` and `&` come out literal. This is load-bearing and
     has teeth: four of the seven catalogues contain 49 `<` and 5 `&` each.
   - **Trailing newline:** `Encoder.Encode`'s `\n` is preserved through
     `json.Indent`; all seven committed files still end `"\n}\n"`.
   - **Shape:** `json.Indent(dst, src, "", "")` yields flush-left, one entry per
     line, with `": "` after each key — matching the committed files.
   - **Escapes:** `\n`, `\t`, control characters and U+2028/U+2029 are escaped
     identically to the deleted `quote()`, which used the same encoder settings.
   The new `TestCatalogsSurviveTheRoundTrip` locks all of this against the seven
   real files and passes.
   **The one divergence is real and correctly documented:** the empty map yields
   `{}\n` where the hand-rolled writer produced `{\n}\n`
   (`tools/i18n/main_test.go:30-33`). It is unreachable here — the smallest
   catalogue, `fr-CH.json`, has six lines — so the decision not to test it is
   defensible.

7. **The five promoted call sites — did any invert the condition?** *Refuted.*
   All five now delegate to `wasmtest.Modul` and none contains its own branch:
   `internal/plugin/sdk_e2e_test.go:22`, `internal/plugin/hofladen_e2e_test.go:24`,
   `internal/plugin/runtime_test.go:28`, `internal/public/formular_e2e_test.go:159`,
   `internal/public/suche_e2e_test.go:27`. A repository-wide grep for `.wasm` in
   `_test.go` files confirms no direct `os.ReadFile` of a module survives (the
   only other hit, `internal/plugin/manager_test.go:318`, is a path-traversal
   fixture and unrelated). `HOLZCLOUD_TEST_REQUIRE_WASM=1 go test
   ./internal/plugin/... ./internal/public/... ./tools/...` passes.
   Workflow placement is as claimed: the variable is at workflow level in
   `ci.yml:22`, `security.yml:17`, `release.yml:19`, absent from `image.yml`, and
   `security.yml`'s step-level `CGO_ENABLED: "1"` override at line 50 merges
   rather than replaces, so it survives into the race run. Note as an
   observation, not a finding: only `ci.yml` runs `tools/wasm -check` — a tagged
   release can therefore still ship on top of stale committed modules, since the
   env var catches a *missing* module but not a *stale* one.

---

_Reviewed: 2026-09-04_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard, with the new command built and executed under six environments_
