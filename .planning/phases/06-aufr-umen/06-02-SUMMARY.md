---
phase: 06-aufr-umen
plan: 02
subsystem: tooling
tags: [go, os/exec, wasip1, reproducible-builds, gotoolchain, buildvcs, ci, sha256]

requires:
  - phase: 06-01
    provides: the amended ground truth (ROADMAP planning notes naming `tools/wasm`, the GOTOOLCHAIN pin with its go.mod floor, mandatory `-buildvcs=false`)
provides:
  - "`tools/wasm` — a Go command that builds any of the six committed wasip1 guest modules from source with a pinned compiler and no VCS stamp"
  - "`-print-hashes`: builds into a temporary directory and prints sha256, size and embedded Go version per module, leaving the working tree untouched"
  - "`-check` and `-out` declared and refused with exit 2, so plan 06-05 inherits a complete flag set rather than inventing one"
  - "the `goToolchain` constant (`go1.26.6`) — the single place the guest compiler is chosen"
  - "a temporary `ci.yml` step that prints the same six hashes on `ubuntu-latest`"
  - "**D-05 answered by observation, not citation: cross-host byte equality holds.** The six hashes are identical on darwin/arm64 and linux/amd64"
  - "measured proof that the pin is forced on the build subprocess regardless of the ambient `GOTOOLCHAIN`, which is what makes the comparison independent of `setup-go` and of `go.mod`"
affects: [06-05, 06-06, 06-07, ci]

actuals:
  tokens: 5100
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Driving a build in a foreign module from Go: one `exec.Cmd` per target with `cmd.Dir` set to the module directory"
    - "A child environment built by filtering the inherited one and then setting the controlled variables once, rather than appending over it"
    - "A tracer plan whose whole purpose is to falsify one cited assumption on real infrastructure before an expensive, hard-to-undo commit depends on it"

key-files:
  created:
    - tools/wasm/main.go
  modified:
    - .github/workflows/ci.yml

key-decisions:
  - "D-05 is PASS: all six sha256 values, all six byte sizes and all six embedded Go versions are identical on darwin/arm64 and on ubuntu-latest. Plans 06-05 and 06-06 execute as written; the D-05 fallback (`-out <dir>` plus `HOLZCLOUD_WASM_DIR`) is not needed."
  - "The coordinator's hypothesis that the hashes matched only because `setup-go` reads `go-version-file: go.mod` and lands on the same number the tool pins is FALSIFIED by experiment. The tool forces `GOTOOLCHAIN` on every build subprocess; the match is caused by the pin, not by the coincidence."
  - "The environment for the build subprocess is built by filtering `os.Environ()` and then appending the controlled variables, not by appending alone. `os/exec` does dedupe in favour of the later value, but a Go program reading its own environment keeps the FIRST mention of a key — filtering removes the dependence on which rule applies. This follows research assumption A2's own recommendation over the verbatim Pattern 2 snippet."
  - "The new `ci.yml` comment is English, not the German the plan asked for. All 11 comment blocks across the four workflow files are English; a lone German block would read as an accident."

patterns-established:
  - "A `tools/` command that shells out states its controlled variables in one named slice, so what the build depends on is a list a reader can check rather than a property of the code path"
  - "A reproducibility claim is recorded with the experiment that could have falsified it, not only with its result"

requirements-completed: []
requirements-advanced: [MAINT-03]

coverage:
  - id: D1
    description: "`go run ./tools/wasm -print-hashes [target]` builds a wasip1 guest from source with the pinned compiler and `-buildvcs=false`, and prints its sha256, size and embedded Go version without writing into the working tree"
    requirement: MAINT-03
    verification:
      - kind: integration
        ref: "go run ./tools/wasm -print-hashes jahreszahl => 1 line, 64-hex sha256, go1.26.6; exit 0"
        status: pass
      - kind: integration
        ref: "go run ./tools/wasm -print-hashes => exactly 6 lines, exit 0; two consecutive runs byte-identical (diff clean)"
        status: pass
      - kind: integration
        ref: "git status --porcelain -- plugins internal/plugin/testdata => empty after both runs"
        status: pass
      - kind: other
        ref: "non-comment source greps: -buildvcs=false => 1, GOFLAGS= => 1, GOEXPERIMENT= => 1, GOTOOLCHAIN => 3; const goToolchain = \"go1.26.6\" vs go.mod 'go 1.26.6' (floor satisfied)"
        status: pass
      - kind: other
        ref: "go build ./tools/wasm => 0; go vet ./tools/wasm => 0; gofmt -l tools/wasm/main.go => empty; repo-wide gofmt -l . empty and go vet ./... => 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "The identical command runs on `ubuntu-latest` in the `test` job, between `Vet` and `Build`, adding no GitHub Action"
    requirement: MAINT-03
    verification:
      - kind: other
        ref: "ci.yml: Vet at line 53, step at 61, run: at 62, Build at 67; grep -c 'uses:' => 3 before and after"
        status: pass
      - kind: integration
        ref: "GitHub Actions run 33866318077, job 'Build and test' green in 1m46s; the step itself ran 11:06:16Z–11:06:22Z (~6 s)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Cross-host byte equality of the wasip1 build is settled by observation on the host the comparison will live on (D-05)"
    requirement: MAINT-03
    verification:
      - kind: integration
        ref: "runner lines extracted from `gh run view 33866318077 --log` and diffed against the local set => IDENTICAL; both sets sha256 975d5843496675dfb04015a1652b89dcd768d2bcddff108979536c8f4d340a26"
        status: pass
      - kind: other
        ref: "run head SHA 6a5bc2f1d378e928784b4fa26100c5662ad9a4fb == local HEAD at the time of measurement; conclusion=success"
        status: pass
    human_judgment: false
  - id: D4
    description: "The pin is forced on the build subprocess regardless of the ambient GOTOOLCHAIN, GOFLAGS, GOEXPERIMENT — the guest bytes depend on the constant alone"
    requirement: MAINT-03
    verification:
      - kind: integration
        ref: "compiled tool run with ambient GOTOOLCHAIN=local on a host whose base toolchain is go1.26.4 => still go1.26.6, hash 629574a3…, 3313021 B (a non-forced build would have produced go1.26.4's 3312731 B)"
        status: pass
      - kind: integration
        ref: "ambient GOTOOLCHAIN=go1.27.0 + GOEXPERIMENT=greenteagc => identical hash; ambient GOFLAGS=-buildvcs=true => identical hash (the stamp would have changed it)"
        status: pass
    human_judgment: false

duration: 13 min
completed: 2026-09-04
status: complete
---

# Phase 6 Plan 02: The wasm Build Tracer and the D-05 Falsification Summary

**`tools/wasm -print-hashes` builds all six committed wasip1 guests from source with `GOTOOLCHAIN=go1.26.6`, `CGO_ENABLED=0`, `-trimpath` and `-buildvcs=false`, and the six hashes it prints on darwin/arm64 are byte-for-byte the same six it prints on `ubuntu-latest` — D-05 is answered by observation and plans 06-05 and 06-06 may execute as written.**

## Cross-host falsification

### Local reference set — darwin/arm64

`go run ./tools/wasm -print-hashes`, at commit `6a5bc2f`, 2026-09-04T11:04Z:

```
bestellung       7431486cf1c60ab544066b0296d958ee422422d456bc2218465dc572a0db6411   4573328 Bytes  gebaut mit go1.26.6
jahreszahl       629574a3b612c073f1bb7ea111f605f979cc97820a0cefe278b0b94a99fd7b82   3313021 Bytes  gebaut mit go1.26.6
kontaktformular  1bde1f7106739059548a8b6d2539bbc486878f37a4f1983becf77a438768a28d   4703040 Bytes  gebaut mit go1.26.6
nicht-gefunden   cb872953acb13ca25c7de3d986ecc65ed7898c0a13104eb13182fd29fb39bdd1   3587610 Bytes  gebaut mit go1.26.6
suche            b5533c1b302895423d43b5dc4e51c1a0bf20427e1465322fcfc28e5467b6d821   3419051 Bytes  gebaut mit go1.26.6
echo             82c0da9e5b9dbca826367884b25980a4efafb58e27a2b4fcb1f2cf75d6a6df0c   3253624 Bytes  gebaut mit go1.26.6
```

Two consecutive runs on this host produced an identical line set, so same-host determinism holds on top of the research's E1–E5.

### Runner set — `ubuntu-latest`, linux/amd64

GitHub Actions run **33866318077** (workflow `ci.yml`, step *Cross-host hash falsification (temporary — remove in plan 06-06)*, 2026-09-04T11:06:21–22Z, head SHA `6a5bc2f1d378e928784b4fa26100c5662ad9a4fb`), read with `gh run view 33866318077 --log`:

```
bestellung       7431486cf1c60ab544066b0296d958ee422422d456bc2218465dc572a0db6411   4573328 Bytes  gebaut mit go1.26.6
jahreszahl       629574a3b612c073f1bb7ea111f605f979cc97820a0cefe278b0b94a99fd7b82   3313021 Bytes  gebaut mit go1.26.6
kontaktformular  1bde1f7106739059548a8b6d2539bbc486878f37a4f1983becf77a438768a28d   4703040 Bytes  gebaut mit go1.26.6
nicht-gefunden   cb872953acb13ca25c7de3d986ecc65ed7898c0a13104eb13182fd29fb39bdd1   3587610 Bytes  gebaut mit go1.26.6
suche            b5533c1b302895423d43b5dc4e51c1a0bf20427e1465322fcfc28e5467b6d821   3419051 Bytes  gebaut mit go1.26.6
echo             82c0da9e5b9dbca826367884b25980a4efafb58e27a2b4fcb1f2cf75d6a6df0c   3253624 Bytes  gebaut mit go1.26.6
```

### Row-by-row comparison

Compared on the full 64-character values by `diff` over the two extracted sets, not by eye. The sha256 column below is truncated to 16 characters for reading only.

| Target | darwin/arm64 | ubuntu-latest | sha256 | Bytes | Go |
|--------|--------------|---------------|--------|-------|-----|
| `bestellung` | `7431486cf1c60ab5` | `7431486cf1c60ab5` | equal | equal (4 573 328) | equal (go1.26.6) |
| `jahreszahl` | `629574a3b612c073` | `629574a3b612c073` | equal | equal (3 313 021) | equal (go1.26.6) |
| `kontaktformular` | `1bde1f7106739059` | `1bde1f7106739059` | equal | equal (4 703 040) | equal (go1.26.6) |
| `nicht-gefunden` | `cb872953acb13ca2` | `cb872953acb13ca2` | equal | equal (3 587 610) | equal (go1.26.6) |
| `suche` | `b5533c1b30289542` | `b5533c1b30289542` | equal | equal (3 419 051) | equal (go1.26.6) |
| `echo` | `82c0da9e5b9dbca8` | `82c0da9e5b9dbca8` | equal | equal (3 253 624) | equal (go1.26.6) |

`diff` over the two six-line sets reports no difference; both sets hash to `975d5843496675dfb04015a1652b89dcd768d2bcddff108979536c8f4d340a26`.

### Verdict

**D-05: PASS (cross-host byte equality confirmed on ubuntu-latest, 2026-09-04, run 33866318077)**

The host axis was the one thing the research could cite but not falsify — its container never got past the registry pull. It is now measured. `GOOS=wasip1 GOARCH=wasm` with `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false` and a pinned toolchain produces the same bytes on darwin/arm64 and linux/amd64, for all six modules. Plans 06-05 and 06-06 execute as written; the D-05 fallback (`-out <dir>` in CI plus `HOLZCLOUD_WASM_DIR` in the shared test helper) is not needed and should not be planned for.

The step cost ~6 s of the job's 1m46s — the research's 15–20 s runner extrapolation (assumption A3) was pessimistic by a factor of three, and the CI budget question raised in the roadmap is closed.

## The `GOTOOLCHAIN` trap that plan 06-06 inherits — investigated, and narrower than it looks

Plan 06-06 makes this comparison **blocking**, so it inherits whatever makes the comparison fragile. One candidate was raised during the gate and is worth writing down properly, because the answer is not the obvious one.

**The worry.** The CI step's environment shows `GOTOOLCHAIN: local` (set by `actions/setup-go`), while `tools/wasm` pins `go1.26.6` internally. Both sides landed on `go1.26.6` — but perhaps only because `setup-go` reads `go-version-file: go.mod`, and `go.mod` currently says `go 1.26.6`, the same number the tool pins. If that were the cause, the agreement would be a coincidence, and a future bump of `go.mod`'s directive without a matching bump of the `goToolchain` constant would silently split the two.

**Measured, and the worry is falsified as stated.** This machine is an unusually good place to test it: its *base* toolchain is `go1.26.4`, and only `GOTOOLCHAIN=auto` selects `go1.26.6` on top. A build that ignored the pin would therefore produce go1.26.4's `jahreszahl` — 3 312 731 bytes, per the research's measurement — which is a different number from the pinned build's 3 313 021 and impossible to confuse.

| Experiment | Ambient environment | Result |
|---|---|---|
| A | `GOTOOLCHAIN=local`, via `go run` | parent died first: `go: go.mod requires go >= 1.26.6 (running go 1.26.4; GOTOOLCHAIN=local)` — the tool never started |
| B | `GOTOOLCHAIN=go1.26.7` | guest built with **go1.26.6**, hash unchanged |
| C | `GOFLAGS=-buildvcs=true` | hash unchanged — the stamp would have changed it, so `GOFLAGS` is genuinely stripped |
| E | `GOTOOLCHAIN=local`, compiled tool (parent cannot die first) | guest built with **go1.26.6**, 3 313 021 B, hash unchanged |
| F | `GOTOOLCHAIN=go1.27.0` + `GOEXPERIMENT=greenteagc` | hash unchanged |

**So: the tool does force `GOTOOLCHAIN` on the build subprocess, exactly as D-03 requires.** The guest bytes depend on the `goToolchain` constant alone — not on `setup-go`, not on `go.mod`, not on the contributor's shell. The cross-host match above is caused by the pin, not by a coincidence of version numbers.

**But there is a real trap next door, and it is D-03a's floor.** What `go.mod`'s directive controls is not the guest bytes, it is whether the build can happen at all:

- `internal/plugin/testdata/echo` lives in the **root module**. If `go.mod`'s `go` directive is bumped above `goToolchain`, the go command refuses to load it and the `echo` target fails with a message naming both versions — the exact error shape of experiment A. **Loud, not silent**, and only for `echo`; the five `plugins/*` modules have their own `go.mod` and would keep building and keep matching. A half-red comparison is the confusing outcome to expect, not a wrong-but-green one.
- Experiment A is also a live contributor trap on its own: on any machine whose base toolchain is below the root `go.mod` directive, `GOTOOLCHAIN=local` in the ambient environment kills `go run ./tools/wasm` before the tool starts. On the runner this is currently masked, because `setup-go` installs exactly the `go.mod` version.

**For 06-05/06-06 to handle — not fixed here, it is out of this plan's scope.** The cheap guard is for `tools/wasm` to compare its `goToolchain` constant against the root `go.mod` `go` directive at startup and refuse with a message naming both, instead of letting `echo` fail deep inside a build. That turns a puzzling one-of-six mismatch into one sentence. Whoever bumps `go.mod` in this repository must bump `goToolchain` in the same commit and rebuild the six artifacts.

## Accomplishments

- **`tools/wasm` exists and does one thing end to end.** 229 lines, `package main`, standard library only — `os/exec`, `crypto/sha256`, `encoding/hex`, `flag`, `regexp`, `slices`, `strings`, `os`, `path/filepath`, `fmt`. No new module, no addition to `go.mod`.
- **Six targets, not five** (D-07): the five `plugins/*` modules plus `internal/plugin/testdata/echo`, each carrying its module directory and the committed artifact path so 06-05's `-check` has them already.
- **`-print-hashes` never writes into the tree.** It builds into `os.MkdirTemp` and removes it; `git status --porcelain -- plugins internal/plugin/testdata` is empty after every run.
- **The output line is diagnostic, not decorative.** Name, sha256, size and the Go version scanned out of the built bytes, column-aligned in the register of `tools/i18n/main.go:112`. The version field exists because a mismatch has two causes — the pin moved, or the source changed — and open question 4 of the research asked for exactly this.
- **`-check` and `-out` are declared and refuse loudly** with exit 2 rather than being accepted and ignored, which is how a CI step passes while checking nothing. Plan 06-05 fills them in.
- **The committed set's compiler drift is now visible as a single number.** All six fresh builds report `go1.26.6`, against the committed set's `go1.24.7` ×4, `go1.26.2` and `go1.26.4`.

## Task Commits

| Task | Commit | What |
|---|---|---|
| 1 (tracer) | `6a5bc2f` | `tools/wasm/main.go` (new) + the temporary `ci.yml` step |
| 2 (checkpoint) | — | Records a verdict; changes no file. Closed by this SUMMARY. |

## Deviations from Plan

**1. [Style] The new `ci.yml` comment is English, not German.**
- **Found during:** Task 1
- **Issue:** The plan asked for "a German `#` comment in the register of `ci.yml:56–58`", but lines 56–58 are English, as are all 11 comment blocks across the four workflow files.
- **Fix:** Wrote the comment in English, keeping the register the plan pointed at (explain *why*, not *what*). The file's own uniform convention beat one word in the plan that contradicts the lines it cites.
- **Files:** `.github/workflows/ci.yml`
- **Commit:** `6a5bc2f`
- **Reversal cost:** one line, and the whole step disappears in 06-06 anyway.

**2. [Rule 2 — robustness] The subprocess environment is filtered, not only appended to.**
- **Found during:** Task 1
- **Issue:** Research Pattern 2 gives `cmd.Env = append(os.Environ(), "GOFLAGS=", …)` verbatim, resting on assumption A2 ("later entries win"). That is true of `os/exec`, which dedupes in favour of the later value — but it is *not* true of a Go program reading its own environment, where `syscall.Getenv` keeps the **first** mention of a key and clears duplicates. The verbatim shape therefore depends on which of two opposite rules happens to apply.
- **Fix:** Strip the six controlled keys from `os.Environ()` first, then set them once. This is what research assumption A2 itself recommended ("build the env slice by filtering … Recommend doing that regardless").
- **Verification:** Experiments C and F above — an ambient `GOFLAGS=-buildvcs=true`, which would re-stamp the git SHA and change the hash, leaves the output byte-identical.
- **Commit:** `6a5bc2f`

**Total deviations:** 2 (1 style, 1 robustness). **Impact:** none on the plan's outcome; both were verified by the same acceptance criteria the plan wrote.

## Requirements

**MAINT-03 stays `Pending`, deliberately.** It is declared by four plans — `06-01`, `06-02`, `06-05`, `06-06` — and the last two have no SUMMARY yet. `requirements ready-ids` confirms `0/1 requirement(s) ready to mark complete`, so `REQUIREMENTS.md` was left untouched. MAINT-03 closes when 06-06 makes the comparison blocking; this plan only proved the comparison is possible.

## Issues Encountered

None. No auth gates, no auto-fixes beyond deviation 2, no package installs (this phase installs none, by design).

## Deferred / Out of Scope

- **The `go.mod`-vs-`goToolchain` floor guard** described above — 06-05/06-06 scope, explicitly not fixed here.
- **`go build ./tools/wasm` drops a native `./wasm` binary in the repository root.** `.gitignore` already ignores this class of accident for `plugins/*` guests but not for `tools/*` outputs; `./i18n` and `./mkbundle` have the same hole today. Pre-existing, unrelated to this task, left alone.

## Next Phase Readiness

Wave 2 is complete (`06-02`, `06-03`, `06-04`). Wave 3 — `06-05`, `tools/wasm` finished with `-check`, `-out` and deterministic archive packing — is unblocked and may execute as written. Its `<precondition>` naming the D-05 verdict is satisfied by this document.

## Self-Check: PASSED

Created files verified on disk (`tools/wasm/main.go`, this SUMMARY). Task commit `6a5bc2f` verified in `git log`. Plan-level verification re-run at close-out: `gofmt -l .` empty, `go vet ./...` exit 0, `go run ./tools/wasm -print-hashes` prints 6 lines, `git status --porcelain -- plugins internal/plugin/testdata` empty, `06-01-SUMMARY.md` present.
