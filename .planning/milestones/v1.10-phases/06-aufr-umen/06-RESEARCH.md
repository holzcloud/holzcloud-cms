# Phase 6: Aufräumen — Research

**Researched:** 2026-09-04
**Domain:** Go build reproducibility (wasip1 guests), repo tooling under `tools/`, GitHub Actions cost, `encoding/json` format locking, planning-note drift repair
**Confidence:** HIGH for everything measured this session; MEDIUM for the one cross-host build that could not be completed in this environment (see §D-05 Verdict)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Ground truth first (ordering)**

- **D-01:** The **first** task of the phase amends `REQUIREMENTS.md` and
  `ROADMAP.md` to match the decisions below, in its own docs commit, before any
  code is touched: MAINT-05 widened to all seven codebase maps (D-14), the
  planning note `plugins/build.sh` corrected to `tools/wasm` (D-06), and the
  toolchain pin (D-02) added as a planning note. A phase whose purpose is
  correct ground truth must not leave its own two source documents contradicting
  its context file.

**Wasm rebuild and comparison (MAINT-03, MAINT-04)**

- **D-02:** The byte comparison from success criterion 3 is **kept**, and made
  stable by pinning the toolchain that builds the guests. Measured during
  discussion: two builds with the same Go produce an identical hash, but the
  committed modules carry `go1.24.7` in the binary while a local rebuild with
  `go1.26.4` differs by ~215 KB. The hash tracks the compiler, so without a pin
  the comparison is red on day one and red again on every Dependabot bump —
  ~21 MB of forced rebuild commits per bump, for no signal about the code.
  — **Reversibility:** reversible — the fallback (D-05) is a CI-file change.
- **D-03:** The pin lives **inside the build tool**, as `GOTOOLCHAIN=go1.26.x`
  with an exact patch level — not in CI's `setup-go` and not as a `toolchain`
  line in `go.mod`. A contributor running the tool locally must produce the same
  bytes as the runner, otherwise the fragility has only moved from `go.mod` to
  CI. It also solves `echo.wasm`, which lives in the root module and therefore
  cannot carry its own `toolchain` line. The guest compiler and the host
  binary's compiler are deliberately decoupled — they share no reason to match.
- **D-04:** Pin to the **current** Go patch level and rebuild all six modules
  once, rather than freezing `go1.24.7`. The phase starts from a deliberately
  set zero point instead of a preserved old state. That rebuild is **its own
  commit touching nothing but `.wasm` files** — the same rule the roadmap
  already applies to the catalogues (PITFALLS #25); ~21 MB otherwise buries the
  next real change and `git log -S` cannot recover it.
  — **Reversibility:** costly — ~21 MB enters git history permanently.
- **D-05:** ⚠️ **Verify before planning locks this in:** whether
  `GOOS=wasip1 GOARCH=wasm` with `-trimpath` produces byte-identical output on
  darwin/arm64 and on linux/amd64. Only one host was measured. If it does not
  hold, the byte comparison cannot carry and the fallback is "build fresh in CI
  and run the plugin tests against the fresh build" — which closes the same hole
  (an SDK change is never validated against a stale binary) at the cost of
  leaving the committed file's freshness unchecked.
- **D-06:** The rebuild lives in **`tools/wasm`, a Go command**, invoked as
  `go run ./tools/wasm` (and `-check` in CI). Deliberate deviation from the
  roadmap's `plugins/build.sh` note: the repo's own tooling is Go commands under
  `tools/` (`tools/i18n`, `tools/mkbundle`) — the form `docs/offene-punkte.md`
  already teaches — while shell exists only under `deploy/`. A Go program also
  crosses the module boundaries cleanly, sets `GOTOOLCHAIN`, and compares the
  hashes itself instead of `sha256sum` plumbing in YAML.
- **D-07:** **All six** committed `.wasm` files are covered, including
  `internal/plugin/testdata/echo.wasm`. The rule is memorable: no committed
  `.wasm` in this repo may go stale unchecked. `echo` does not import the SDK
  (it uses raw `//go:wasmimport`), so an SDK change cannot stale it — but a
  change to the host calling convention (`hc_call`, `hc_alloc`) can, and
  witnessing that convention is echo's only purpose. A stale echo attests an ABI
  that no longer exists: the same false pass, one layer down. Note MAINT-03's
  glob `plugins/*/plugin.wasm` does not literally reach it, while MAINT-04's
  "five tests" does — the requirements are asymmetric; D-01 fixes that.
- **D-08:** The comparison is a **step in the existing `test` job** in
  `ci.yml`, placed before `go test ./...`. It shares checkout, `setup-go` and
  the build cache, so it costs only the build itself. It sits on the critical
  path deliberately.
- **D-09:** **The tests keep reading the committed file.** Because the
  comparison proves the committed file equals a fresh build, MAINT-04 stays
  exactly as scoped and needs no rewiring.
- **D-10:** `HOLZCLOUD_TEST_REQUIRE_WASM=1` is set **repo-wide**, as a
  workflow-level `env:` in the three workflows that actually run tests:
  `ci.yml`, `security.yml` and `release.yml:40`. `image.yml` runs no tests and
  is untouched. Anything on a GitHub runner *is* CI; exempting `security.yml`
  would leave the hole open exactly where the plugin tests run most thoroughly
  (`-race`, measured at 297 s for `./internal/plugin/` alone).
- **D-11:** The promoted tests' failure message names **`go run ./tools/wasm`**
  as the command that builds the missing file.
- **D-12:** Ordering is non-negotiable and inherited from the roadmap: the
  rebuild-and-compare lands **first**, the skip promotion **second**.

**i18n format lock and self-description (MAINT-01, MAINT-02)**

- **D-13:** `writeCatalog` goes through the standard library as
  `json.NewEncoder` with `SetEscapeHTML(false)` over the whole map, then
  `json.Indent(dst, src, "", "")`. **Measured during discussion — this
  reproduces today's format byte for byte:** flush-left, HTML unescaped, keys
  sorted byte-wise (Go's encoder sorts map keys exactly as `sort.Strings` does).
  `quote()` disappears entirely; it existed only to dodge `MarshalIndent`'s
  escaping. Both roadmap warnings were re-confirmed by measurement:
  `json.MarshalIndent` escapes HTML to `&lt;` with no switch, and
  `Encoder.SetIndent("", "")` is a no-op that emits one line.
- **D-14:** The round-trip test lives in `tools/i18n` (which has **no test file
  at all** today) and covers **all seven catalogues** — including `fr-CH.json`
  and `it-CH.json`, which the tool never writes in normal operation. Reading
  each file and writing it back through `writeCatalog` into a buffer, compared
  byte for byte, puts every catalogue under the format lock without adding a
  special case to the write path.
- **D-15:** The `git diff` proof from success criterion 1 becomes a **permanent
  CI step**, modelled on the twin already in the same file: `ci.yml:47–50` runs
  `go mod tidy` and then `git diff --exit-code`. The i18n step runs `-write`,
  then `-schweiz`, then `git diff --exit-code -- internal/i18n/locales/`.
  **Scope boundary, deliberately drawn:** this proves the catalogues are in
  canonical format and in step with the source. It does **not** prove
  "0 offen" — `-write` creates a missing key with an empty value, after which
  the diff is clean and the translation is still absent. That is QUAL-01's human
  half and stays where the roadmap put it (see Deferred).
- **D-16:** `fr-CH.json` and `it-CH.json` keep their current behaviour — never
  written, only read and reported. The user first chose to extend `-schweiz` to
  all three and revised after the facts: `writeSwiss` works for `de-CH` because
  a mechanical rule exists (`main.go:221` — `ß→ss`, `„→«`, `“→»`) that derives
  the bulk of the entries from the German source keys. No such rule exists for
  the other two; their entries are pure vocabulary choice (*natel* for
  *portable*, *formulario*, *laptop*) and no `strings.NewReplacer` derives them.
- **D-17:** MAINT-02's plain statement goes in two places a person actually
  looks: the doc comment at the head of `tools/i18n/main.go` — which at the same
  time stops claiming indented output, MAINT-02's second half — and the
  per-regional-file report line at `main.go:113`, which crosses the screen on
  every run.

**Stale notes (MAINT-05)**

- **D-18:** All **seven** codebase maps are corrected surgically, not just
  `ARCHITECTURE.md`. MAINT-05 understates the problem: every map carries
  `Analysis Date: 2026-08-22`, and the same wrong numbers are copied across
  several of them — "38 goose migrations / through `00038_block_types.sql`"
  (actual: 45, through `00045_pages_locale_unique.sql`) in `ARCHITECTURE.md:112`,
  `CONCERNS.md:184`, `STACK.md:9` and `STRUCTURE.md:16`; "Go 1.26.2" (actual:
  `go.mod` says 1.26.6) in `CONCERNS.md:244`, `STACK.md:8` and `TESTING.md:8`.
  The phase goal — "no planning note that Phases 7–10 are planned against is
  still stale" — is the truer statement, and Phases 7–10 are planned against
  `STACK.md` and `STRUCTURE.md` just as much as against `ARCHITECTURE.md`.
- **D-19:** Surgical, **not** regenerated. Countable facts are corrected in
  place — migration count, Go version, analysis date, plus the six drifts named
  for `ARCHITECTURE.md` — and nothing else is touched. Each changed number must
  be provable against the working tree line by line. Re-running the mapper would
  produce a large, unreviewable diff and would silently rewrite the hand-written
  judgements in `CONCERNS.md`, the one file whose value is assessment rather
  than counting.
- **D-20:** The three `deferred-items.md` get a **stamp in the header, text
  unchanged**: one short `> **Erledigt am … — …**` line per finding, naming the
  commit or task. MAINT-05 asks that they "read as closed" — a deleted file
  reads as nothing, and each one carries the reasoning a later phase needs (the
  `page.terms` slug-vs-name finding bears on future bundle work). All three were
  verified closed during discussion: `tools/mkbundle` now checks via `t.Name`,
  the flaky `f_` assertion in `internal/public/formular_e2e_test.go` was
  rewritten, and the catalogue format was normalised by quick task
  `260903-bsk`.
- **D-21:** `docs/offene-punkte.md:87` — the finished Dependabot item is
  removed from the numbered "Was noch fehlt" list, and its procedure (`:93–105`
  — one at a time, `go build`/`go vet`/`go test` between them, plus a run
  against a real database file for `modernc.org/sqlite`) moves to
  `## Beim Weiterarbeiten`, the section the document already keeps for standing
  instructions and where the i18n and browser rules already live.
- **D-22:** Mechanical, no decision needed: `docs/offene-punkte.md:140` says
  migrations run to `00044`; they run to `00045`.

### Claude's Discretion

The user answered "Entscheide du" on four questions. The choices made, so no
downstream agent re-derives them:

- **Shape of the wasm comparison** → D-02, D-03, D-09. Byte comparison kept, made
  stable by a pin in the build tool; tests keep reading the committed file.
- **Where `HOLZCLOUD_TEST_REQUIRE_WASM=1` is set** → D-10. Repo-wide across the
  three testing workflows.
- **Reach of the `git diff` proof** → D-15. Permanent CI step, with the explicit
  scope boundary that it does not enforce "0 offen".
- **Form of "closed" in `deferred-items.md`** → D-20. Header stamp, text kept.

### Deferred Ideas (OUT OF SCOPE)

- **Enforce "0 offen, 0 verwaist" mechanically in CI.** Deliberately not in
  Phase 6. The `git diff` step (D-15) catches missing keys, not empty values; a
  hard "0 offen" check would turn CI red on a translation that is legitimately
  in progress. That is QUAL-01's own decision and QUAL-01 is formally assigned
  to Phase 10.
- **Bump `plugins/*/go.mod` from `go 1.24` to match the pinned compiler.**
  Cosmetic; the language version and the toolchain are independent, and D-03
  deliberately decouples them. Not needed for any success criterion.
- **A mechanical Swiss-French rule** (*septante/huitante/nonante*) for
  `fr-CH.json`, mirroring the `ß→ss` rule for German. Rejected for this phase:
  blind replacement inside translated sentences is risky, and a rule for one of
  the two languages would reintroduce the asymmetry D-16 settles.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description (REQUIREMENTS.md:113–117) | Research Support |
|----|---------------------------------------|------------------|
| MAINT-01 | `tools/i18n` writes its catalogues through the standard library, and a round-trip test locks the format… The committed catalogue files do not change — verified byte for byte, not assumed. | §i18n Format Lock — the proposed stdlib route was re-measured against all seven committed catalogues this session: 7/7 byte-identical, under both the default `encoding/json` and `GOEXPERIMENT=jsonv2`. One divergence found (empty map) that the round-trip test must not trip over. |
| MAINT-02 | `fr-CH.json` and `it-CH.json` are never written by the tool, only read… and `tools/i18n`'s own doc comment stops claiming its output is indented. | §i18n Self-Description — exact line anchors verified (`tools/i18n/main.go:1–15` doc comment, `:99–113` the regional branch, `:113` the report line, `:289` `writeCatalog`). **The doc comment does not in fact claim indented output — the false claim is at `:289`.** |
| MAINT-03 | CI rebuilds every `plugins/*/plugin.wasm` and compares the result against the committed file… | §D-05 Verdict + §`tools/wasm` Design + §CI Cost. **Critical:** `-buildvcs` (default `auto`) stamps the git revision into every guest, which makes the byte comparison mathematically impossible to satisfy without `-buildvcs=false`. Measured. |
| MAINT-04 | The five tests that today skip themselves… fail loudly in CI and stay forgiving on a contributor's machine, with a message that says how to build the missing file. | §MAINT-04 Promotion Mechanics — three different Go test packages are involved (`plugin`, `plugin_test`, `public`); the helper's placement follows from that. |
| MAINT-05 | The planning and repository notes that have gone stale are corrected… | §MAINT-05 Correction Inventory — a complete, line-referenced list verified against the working tree at `8edcd9c`. **The "six drifted facts" figure is itself stale: 20+ are drifted in `ARCHITECTURE.md` alone.** |
</phase_requirements>

---

## Project Constraints (from CLAUDE.md)

These are as binding as the locked decisions above. Every one of them constrains this phase.

| Directive | Consequence for Phase 6 |
|-----------|-------------------------|
| **Go stdlib-first; hard stack mandate** | `tools/wasm` uses `os/exec`, `crypto/sha256`, `os`, `path/filepath`, `flag`, `fmt` — nothing else. No new module. |
| **No npm, no Node, no bundlers, no other JS frameworks** | Not touched by this phase. |
| **`CGO_ENABLED=0`** for the host binary | Already the workflow-level `env:` in `ci.yml:15–17`, `security.yml:11–12`, `release.yml:12–14`. `CGO_ENABLED=0` is also *required* for the guest builds to be reproducible (see §D-05). |
| **Nothing loads at runtime from a third party** | Untouched. The `GOTOOLCHAIN` download (§D-03) is a **build-time module fetch**, which CLAUDE.md explicitly permits ("Only two things may be downloaded at build time: 1. Go modules"). A `GOTOOLCHAIN` toolchain *is* a Go module (`golang.org/toolchain@v0.0.1-goX.Y.Z-GOOS-GOARCH`) [CITED: go.dev/doc/toolchain]. |
| **Go files `snake_case.go`; packages short, lowercase, single-word** | `tools/wasm/main.go`. A helper package for MAINT-04 must be a single lowercase word. |
| **Build: `-trimpath -ldflags="-s -w"`** | Already the documented guest invocation; keep it. |
| **`slog.Error` for unexpected errors** | Does not apply to `tools/` — `tools/i18n` and `tools/mkbundle` both write to `os.Stderr` and `os.Exit(1)`. Follow that, not `slog`. |
| **GSD workflow enforcement** | Every edit in this phase goes through `/gsd-execute-phase`. |

---

## Summary

The phase has one live risk and it is not the one D-05 names. D-05 asks whether a
`GOOS=wasip1 GOARCH=wasm` build is byte-identical across hosts. It very probably is —
Go has guaranteed exactly this since 1.21 for `CGO_ENABLED=0 go build -trimpath`
[CITED: go.dev/blog/rebuild] — and every host-adjacent variable I *could* test on this
machine (build directory, `GOCACHE` contents, module-cache location) was measured to
have no effect. The real blocker is one flag nobody has looked at: **`-buildvcs`
defaults to `auto`, and all six committed `.wasm` files carry `vcs.revision`,
`vcs.time` and `vcs.modified=true` stamped into them.** The revision is the git SHA at
build time. A rebuild in CI therefore happens at a *different* commit than the one that
produced the committed file, by construction, so the two can never match. I measured
this directly: the same source at two different commits produced two different hashes
(`f04dae1e…` vs `62cb554d…`); with `-buildvcs=false` the same two commits produced the
same hash twice (`59815ce1…`). Without `-buildvcs=false`, D-02's byte comparison is red
on every commit forever, and no toolchain pin can rescue it.

Two further facts change the shape of the plan. First, the committed set is **already
inconsistent**: four files carry `go1.24.7`, `echo.wasm` carries `go1.26.2`, and
`kontaktformular` carries `go1.26.4`; four of them still stamp the pre-rename module
path `github.com/holzcloud/cms/sdk`. The staleness MAINT-03 fears is not hypothetical,
it is measurable in the bytes on disk. Second, the CI budget worry in the roadmap is
unfounded: six wasip1 guests built from a **completely empty** `GOCACHE` took 4.6 s of
wall time in total on this machine. The roadmap's 297 s figure is for `go test -race`,
which this step does not touch.

On the housekeeping half, the i18n change is safe and I re-verified it independently:
`json.NewEncoder`+`SetEscapeHTML(false)` then `json.Indent(dst, src, "", "")` reproduces
all seven committed catalogues byte for byte, including the trailing newline, the `": "`
separator and the key order — and it still does under `GOEXPERIMENT=jsonv2`. One edge
case diverges (an empty map: `{\n}\n` today vs `{}\n` from the stdlib route) and it is
unreachable in practice, but the round-trip test must not be written in a way that hits
it. On MAINT-05 the news is worse than the requirement says: `ARCHITECTURE.md` does not
have six drifted facts, it has more than twenty, and `INTEGRATIONS.md` is the most
broken map of the seven — it still describes a `k8s/` directory, a `deploy.yml`
workflow and a Raspberry Pi that no longer exist.

**Primary recommendation:** add `-buildvcs=false` to every guest build invocation — the
tool, `plugins/README.md`, `internal/plugin/testdata/README.md` and the `//go:generate`
line at `internal/plugin/runtime_test.go:13` — and treat that, not the toolchain pin, as
the change that makes D-02's byte comparison possible at all. Then pin
`GOTOOLCHAIN=go1.26.6` inside `tools/wasm`, rebuild all six in a `.wasm`-only commit,
and wire `go run ./tools/wasm -check` into the existing `test` job.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Building the six wasip1 guests | Build tooling (`tools/wasm`, host `go` toolchain) | — | Crosses six module boundaries and needs an environment the shell would have to re-derive per directory; D-06 already places it here. |
| Deciding *whether* a committed guest is current | CI (`ci.yml` `test` job) | Build tooling (`-check` does the comparison) | The tool computes and compares; the workflow decides the job fails. YAML holds no hashing logic (D-06). |
| Deciding whether a *missing* guest is fatal | Test code (a shared helper) | CI (`env:` supplies the signal) | The helper must behave differently on a contributor's machine and on a runner; only the process environment distinguishes them (D-10). |
| Catalogue serialisation format | `tools/i18n` (`writeCatalog`) | `encoding/json` stdlib | One writer, one format; the round-trip test in the same package is the lock (D-13, D-14). |
| Proving the catalogues are in step with the source | CI (`ci.yml`) | `tools/i18n` (`-write`, `-schweiz`) | Mirrors the `go mod tidy` + `git diff --exit-code` idiom already at `ci.yml:48–51` (D-15). |
| Recording what the codebase *is* | `.planning/codebase/*.md` (hand-corrected) | — | D-19 forbids regeneration; the maps are read by planners, not by code. |

---

## ⚠️ D-05 Verdict: cross-host reproducibility of the wasip1 build

### Verdict: **RELIABLE WITH CONDITIONS** — but the binding condition is `-buildvcs=false`, not the toolchain pin

D-05 asks about the host axis. That axis turns out not to be the problem. A different,
un-asked-about axis is, and it defeats the byte comparison unconditionally.

### The finding that changes the plan: VCS stamping

`go build`'s `-buildvcs` flag defaults to `auto`: *"version control information is
stamped into a binary if the main package, the main module containing it, and the
current directory are all in the same repository"*, and what is recorded is
`vcs.revision`, `vcs.time`, `vcs.modified` [CITED: pkg.go.dev/cmd/go]. All six
conditions hold for all six guests in this repository, and the stamps are present in the
committed bytes today.

Extracted verbatim from the committed `plugins/jahreszahl/plugin.wasm` build-info blob
[VERIFIED: plugins/jahreszahl/plugin.wasm, buildinfo blob at byte offset ~2306745, read this session]:

```
path	jahreszahl
mod	jahreszahl	v0.0.0-20260805121613-590af54e62b1+dirty
dep	github.com/holzcloud/cms/sdk	v0.0.0
=>	../../sdk	(devel)
build	-buildmode=c-shared
build	-compiler=gc
build	-trimpath=true
build	CGO_ENABLED=0
build	GOARCH=wasm
build	GOOS=wasip1
build	vcs=git
build	vcs.revision=590af54e62b1af6e180c6ecff858945e2bd57d8c
build	vcs.time=2026-08-05T12:16:13Z
build	vcs.modified=true
```

`vcs.revision` is the git SHA of `HEAD` when the file was built. The pseudo-version in
the `mod` line is derived from the same SHA. A CI rebuild necessarily runs at a
different commit than the one that produced the committed artifact, so those bytes
differ **by construction**. `vcs.modified=true` compounds it: all six were built from a
dirty tree, and a clean CI checkout would stamp `false`.

**Measured** [VERIFIED: local experiment, this session]:

| # | Experiment (all `plugins/jahreszahl`, `GOOS=wasip1 GOARCH=wasm -buildmode=c-shared -trimpath -ldflags="-s -w"`) | sha256 (16) | Result |
|---|---|---|---|
| E1 | Same working tree, built twice | `d5173858b320c215` both | deterministic |
| E2 | Isolated copy in a fresh git repo, at commit *one* | `f04dae1e4fd818e1` | — |
| E2 | Same copy, identical source, at commit *two* (`git commit --allow-empty`) | `62cb554d8790c5bd` | **DIFFERENT** |
| E3 | Commit *one*, `-buildvcs=false` | `59815ce1f7e3b6e1` | — |
| E3 | Commit *two*, `-buildvcs=false` | `59815ce1f7e3b6e1` | **IDENTICAL** |
| E4 | Real repo vs `/tmp` copy, both `-buildvcs=false` | `59815ce1f7e3b6e1` both, `cmp -l` = 0 differing bytes | location-independent |
| E5 | Fresh empty `GOCACHE`, `-buildvcs=false` | `59815ce1f7e3b6e1` | build-cache-independent |

Sizes: `-buildvcs=false` output is 3 312 731 B; with the stamps it is 3 312 864 B — 133
bytes of git metadata.

**Conclusion:** without `-buildvcs=false`, D-02 cannot be satisfied by any means. With
it, four of the five environment variables that could plausibly differ between a
developer machine and a runner (working directory, git revision, git dirtiness, build
cache contents) are proven not to matter.

### The host axis (what D-05 actually asked)

The Go project states the property directly [CITED: go.dev/blog/rebuild, "Perfectly
Reproducible, Verified Go Toolchains"]: a given target can be built *"on a
Linux/x86-64 host, or a Windows/ARM64 host, or a FreeBSD/386 host, or any other host
that supports Go"* and the result is the same; for ordinary Go programs reproducibility
is *"as simple as compiling with `CGO_ENABLED=0 go build -trimpath`"*, because
*"disabling cgo removes the host C toolchain as a relevant input"* and *"`-trimpath`
removes the current directory"*. Both conditions hold here: `CGO_ENABLED=0` is the
workflow-level `env:` in all three testing workflows and `-trimpath` is already in the
documented invocation.

I attempted a direct falsification — a `linux/amd64` container building the same module
with the same Go version and comparing against the darwin/arm64 hash — and it did not
complete inside this session's budget (image and toolchain download). **I therefore
report no observation on the host axis rather than a declared absence.** The claim
"host-independent" is `[CITED: go.dev/blog/rebuild]`, not `[VERIFIED]`.

### Conditions the plan must hold (all of them, or the comparison drifts)

1. `-buildvcs=false` on every guest build. **Mandatory** — measured above.
2. `CGO_ENABLED=0`. Already set workflow-wide; `tools/wasm` should set it explicitly in the child environment anyway, because a contributor's shell may differ.
3. `-trimpath`. Already in the documented invocation.
4. An exact toolchain, identical on both sides — this is D-03's `GOTOOLCHAIN=go1.26.6`.
5. `GOFLAGS` and `GOEXPERIMENT` cleared in the child environment. An inherited `GOFLAGS=-mod=vendor` or `GOEXPERIMENT=…` silently changes the output.
6. Identical module content, including the `replace … => ../../sdk` target. A dirty `sdk/` changes the guest bytes; that is desirable (it is the staleness signal MAINT-03 wants) but it means `-check` is a *content* check, not only a compiler check.

### The cheap empirical gate a plan task can run

This is the falsification the phase should own, and it costs one CI run:

> **Task: prove cross-host equality before the byte comparison is made blocking.**
> In the same commit that adds `tools/wasm`, add a temporary CI step (or use
> `workflow_dispatch`) that runs `go run ./tools/wasm -print-hashes` on the
> `ubuntu-latest` runner and prints the six sha256 values. Compare them against the six
> values produced locally by the identical command. If all six match, delete the
> temporary step and make `-check` blocking. If any differs, take the D-05 fallback.
>
> `-print-hashes` costs nothing extra to implement: it is `-check` without the
> comparison.

Rationale for doing it this way: it uses the real runner (the exact host the comparison
will live on), it needs no container, and it fails *loudly and once* rather than turning
`main` red.

### If it does not hold — the D-05 fallback, made concrete

Shape: *"build fresh in CI and run the plugin tests against the fresh build."*

- `tools/wasm` gains `-out <dir>`, writing the six guests into a directory instead of over the tree.
- `ci.yml` runs `go run ./tools/wasm -out "$RUNNER_TEMP/wasm"` before `go test ./...` and exports `HOLZCLOUD_WASM_DIR=$RUNNER_TEMP/wasm`.
- The shared test helper (see §MAINT-04) prefers `$HOLZCLOUD_WASM_DIR/<name>.wasm` when the variable is set and falls back to the committed path otherwise. This is the **only** change D-09 would have to give up, and it is one function.
- The committed files stay committed and stay read by default, so a contributor's experience is unchanged.
- What is lost: nothing proves the *committed* file is current. Mitigate by keeping the six hashes in a small committed text file that `tools/wasm` writes and CI compares — the hashes are host-dependent in this scenario, so that mitigation is only worth taking if the divergence turns out to be per-host-stable.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/exec` (stdlib) | Go 1.26.6 | Run `go build` per module from `tools/wasm` | The only way to cross a module boundary; `go/build` cannot drive a full link. [ASSUMED] |
| `crypto/sha256` (stdlib) | Go 1.26.6 | Compare built bytes to committed bytes | Already the repo's hash of record (`sha256sum` in `ci.yml:63` and `release.yml:50`). [VERIFIED: .github/workflows/ci.yml:63] |
| `encoding/json` (stdlib) | Go 1.26.6 | `writeCatalog` replacement | D-13; measured byte-exact this session. |
| `testing` (stdlib) | Go 1.26.6 | Round-trip test, shared wasm helper | The repo has zero test dependencies. [VERIFIED: .planning/codebase/TESTING.md:8–9 — "No config file — no testify, no gomock, no ginkgo. Zero test dependencies."] |
| `flag`, `fmt`, `os`, `path/filepath`, `sort`, `strings` (stdlib) | Go 1.26.6 | The rest of `tools/wasm` and `tools/i18n` | Exactly the import set `tools/i18n/main.go` already uses. [VERIFIED: tools/i18n/main.go:18–30] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `archive/zip` (stdlib) | Go 1.26.6 | Repack `plugins/*/*.zip` deterministically if the plan decides to keep them in step | Only if Open Question 1 is answered "repack". `tools/mkbundle` already uses it. [VERIFIED: tools/mkbundle/main.go:20 — `"archive/zip"`] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `tools/wasm` (Go) | `plugins/build.sh` (the roadmap's original note) | Rejected by D-06. Also: shell would have to re-derive the six directories, set `GOTOOLCHAIN` per invocation and shell out to `sha256sum`, which differs between macOS (`shasum -a 256`) and Linux. |
| `GOTOOLCHAIN` env in the tool | `toolchain go1.26.6` line in each `go.mod` | Rejected by D-03, and it cannot work: `internal/plugin/testdata/echo` is in the root module, whose `toolchain` line would then govern the host binary too. |
| `GOTOOLCHAIN=go1.26.6` (exact) | `GOTOOLCHAIN=go1.26.6+auto` | `+auto` defeats the pin — the go command *"selects and runs a newer Go version as needed"* [CITED: go.dev/doc/toolchain]. Use the bare form. |

**Installation:** none. This phase adds no external dependency of any kind.

**Version verification:**
```
$ go version
go version go1.26.6 darwin/arm64          # via GOTOOLCHAIN=auto + go.mod's `go 1.26.6`
$ GOTOOLCHAIN=local go version
go version go1.26.4 darwin/arm64          # the actually-installed toolchain
```
[VERIFIED: local shell, this session]

## Package Legitimacy Audit

**This phase installs no external packages.** Every import is either the Go standard
library or a package already in `go.mod`. No registry lookup is applicable and no
`checkpoint:human-verify` install gate is required.

| Package | Registry | Verdict | Disposition |
|---------|----------|---------|-------------|
| *(none)* | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

One adjacent supply-chain note that *is* in scope: `security.yml:63–66` installs
`golang.org/x/vuln/cmd/govulncheck@latest` at run time, and `image.yml:33,51,55` uses
`actions/checkout@v7`, `docker/setup-buildx-action@v3` and `docker/login-action@v3` by
**tag, not commit SHA** — contradicting the repo's own stated rule ("CI actions are
pinned to a commit, not a tag", `ci.yml:22–24`). Out of scope for Phase 6; worth a
`/gsd-capture` note. [VERIFIED: .github/workflows/image.yml:33,51,55]

---

## Architecture Patterns

### System Architecture Diagram

```text
                      ┌──────────────────────────────────────────┐
   contributor ──────►│  go run ./tools/wasm        (writes)     │
   CI `test` job ────►│  go run ./tools/wasm -check (compares)   │
                      └───────────────┬──────────────────────────┘
                                      │  for each of six targets
                     ┌────────────────┴────────────────┐
                     ▼                                 ▼
        ┌─────────────────────────┐        ┌────────────────────────────┐
        │ five plugin modules     │        │ one package in the ROOT    │
        │ plugins/<n>/go.mod      │        │ module                     │
        │   go 1.24               │        │ internal/plugin/testdata/  │
        │   replace → ../../sdk   │        │   echo   (go.mod: 1.26.6)  │
        └───────────┬─────────────┘        └─────────────┬──────────────┘
                    │                                    │
                    └────────────┬───────────────────────┘
                                 ▼
              exec: go build  cwd = <module dir>
              env: GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0
                   GOTOOLCHAIN=go1.26.6  GOFLAGS=  GOEXPERIMENT=
              args: -buildmode=c-shared -trimpath -buildvcs=false
                    -ldflags="-s -w" -o <tmp>/out.wasm .
                                 │
                    ┌────────────┴────────────┐
              write mode                  -check mode
                    │                          │
                    ▼                          ▼
        os.Rename → plugins/<n>/       sha256(tmp) vs sha256(committed)
          plugin.wasm                       │
        (and testdata/echo.wasm)      equal │ differ
                                            │      └──► report path, both
                                            │           hashes, both sizes,
                                            ▼           both embedded Go
                                          exit 0        versions → exit 1
                                                        "Neu bauen mit:
                                                         go run ./tools/wasm"
```

```text
   ci.yml  test job  (existing steps in grey)
   ─ checkout ─ setup-go(cache:true) ─ gofmt ─ go mod tidy+diff ─ vet
     └─► NEW: go run ./tools/wasm -check          (D-08: before go test)
     └─► NEW: go run ./tools/i18n -write
              go run ./tools/i18n -schweiz
              git diff --exit-code -- internal/i18n/locales/   (D-15)
   ─ build ─ go test ./...  (now with HOLZCLOUD_TEST_REQUIRE_WASM=1) ─ upload
```

### Recommended Project Structure

```
tools/
├── i18n/
│   ├── main.go            # writeCatalog rewritten (D-13), doc comment fixed (D-17)
│   └── main_test.go       # NEW — round-trip over all seven catalogues (D-14)
├── mkbundle/main.go       # untouched
└── wasm/
    └── main.go            # NEW — the whole tool, one file, like its two siblings

internal/plugin/
├── wasmtest/
│   └── wasmtest.go        # NEW — the shared skip/fail helper (MAINT-04)
└── testdata/
    ├── README.md          # build command gains -buildvcs=false
    └── echo.wasm          # rebuilt
```

### Pattern 1: A `tools/` command in this repo's house style

**What:** `package main` in one file under `tools/<name>/`, run as `go run ./tools/<name>`.
**When to use:** every build-time helper in this repository.
**The shape, taken from the two that exist** [VERIFIED: tools/i18n/main.go:1–16, tools/mkbundle/main.go:1–52]:

- A doc comment that opens with `// Command <name> …`, then an indented block of
  invocation lines, then *why* the thing exists in prose. `tools/mkbundle` runs to
  16 lines of rationale before `package main`. Match that register.
- Flags via `flag`, with a `-root` string flag defaulting to `"."` when the tool needs
  the repository root — `tools/i18n/main.go` does exactly this.
- Errors: `fmt.Fprintln(os.Stderr, "error:", err)` then `os.Exit(1)`, via a package-level
  `func fail(err error)`. Not `slog`, not `log.Fatal`.
  [VERIFIED: tools/i18n/main.go:309–312 — `func fail(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }`]
- Progress reported on stdout with `fmt.Printf` in German, one line per unit of work,
  e.g. `tools/i18n/main.go:112` prints `"%-12s %d Abweichungen, %d ohne Gegenstück\n"`.
- Comments inside `tools/` are written in **English**; comments inside `internal/` are
  written in **German**. `tools/i18n` and `tools/mkbundle` are both English throughout.
  [VERIFIED: tools/i18n/main.go:1–15, tools/mkbundle/main.go:1–16]

### Pattern 2: Driving a build in a foreign module from Go

**What:** one `exec.Cmd` per target, `Dir` set to the module directory, environment
built explicitly rather than inherited.
**When to use:** `tools/wasm`, and nowhere else in this repo.

```go
// Source: pattern derived from cmd/go's documented flag semantics; the flag set
// itself is copied verbatim from internal/plugin/runtime_test.go:13 plus the
// -buildvcs=false this research adds.
type ziel struct {
	dir  string // relative to the repository root
	out  string // relative to the repository root
}

var ziele = []ziel{
	{"plugins/bestellung", "plugins/bestellung/plugin.wasm"},
	{"plugins/jahreszahl", "plugins/jahreszahl/plugin.wasm"},
	{"plugins/kontaktformular", "plugins/kontaktformular/plugin.wasm"},
	{"plugins/nicht-gefunden", "plugins/nicht-gefunden/plugin.wasm"},
	{"plugins/suche", "plugins/suche/plugin.wasm"},
	{"internal/plugin/testdata/echo", "internal/plugin/testdata/echo.wasm"},
}

const goToolchain = "go1.26.6" // must be >= the `go` directive in ./go.mod — see Pitfall 3

func bauen(root string, z ziel, nach string) error {
	cmd := exec.Command("go", "build",
		"-buildmode=c-shared",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags=-s -w",
		"-o", nach, ".")
	cmd.Dir = filepath.Join(root, z.dir)
	// A built-from-scratch environment, not os.Environ() with additions: an
	// inherited GOFLAGS or GOEXPERIMENT changes the bytes and would make the
	// comparison depend on the contributor's shell.
	cmd.Env = append(os.Environ(),
		"GOOS=wasip1",
		"GOARCH=wasm",
		"CGO_ENABLED=0",
		"GOTOOLCHAIN="+goToolchain,
		"GOFLAGS=",
		"GOEXPERIMENT=",
	)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
```

Notes that matter:

- `-ldflags=-s -w` as a **single** `exec` argument. Writing `-ldflags="-s -w"` in Go
  source would pass literal quotes to the linker; the shell quoting in the READMEs does
  not survive `os/exec`.
- Later entries win in `cmd.Env` on Unix, so `append(os.Environ(), …)` is a legitimate
  override. Clearing `GOFLAGS`/`GOEXPERIMENT` by appending an empty assignment works for
  the same reason. [ASSUMED — documented `exec` behaviour on Unix; the repo targets linux and darwin only]
- `-o` must be an **absolute** path or one relative to `cmd.Dir`. Build to a temp file
  and `os.Rename` into place, so a failed build never leaves a truncated `.wasm`.
  `os.Rename` across filesystems fails, so put the temp file beside the target, not in
  `os.TempDir()`.
- The build output bytes do **not** depend on the `-o` path — measured (E4/E5).

### Pattern 3: A shared skip-or-fail helper across three test packages

The five call sites live in **three different Go packages** [VERIFIED: file headers read this session]:

| File | Package | Path it reads |
|------|---------|---------------|
| `internal/plugin/runtime_test.go:26` | `package plugin` | `testdata/echo.wasm` |
| `internal/plugin/sdk_e2e_test.go:23` | `package plugin_test` | `../../plugins/jahreszahl/plugin.wasm` |
| `internal/plugin/hofladen_e2e_test.go:25` | `package plugin_test` | `../../plugins/bestellung/plugin.wasm` |
| `internal/public/formular_e2e_test.go:160` | `package public` | `../../plugins/kontaktformular/plugin.wasm` |
| `internal/public/suche_e2e_test.go:28` | `package public` | `../../plugins/suche/plugin.wasm` |

Because `package public` cannot see identifiers declared in `internal/plugin`'s test
files, a test-only helper cannot serve all five. **Recommendation: a tiny non-test
package `internal/plugin/wasmtest`.** It imports only `os` and `testing`, nothing in
production imports it, so it adds nothing to the shipped binary — the same shape as
`net/http/httptest`.

Rejected alternative: declare an exported helper in an in-package `_test.go` under
`internal/plugin` (visible to `plugin_test` via the standard `export_test.go` trick) and
duplicate it in `internal/public`. That works, but it puts D-11's failure message in two
places, and MAINT-04 is precisely a requirement about one consistent message.

### Anti-Patterns to Avoid

- **Leaving `-buildvcs` at its default in any of the four places the build command is written down.** The tool, `plugins/README.md`, `internal/plugin/testdata/README.md` and `internal/plugin/runtime_test.go:13` must all agree, or a contributor who follows the README produces bytes `-check` rejects.
- **Putting the hash comparison in YAML.** D-06 settled this; `sha256sum` does not exist under that name on macOS, so a shell version would be developer-hostile.
- **Regenerating the codebase maps.** D-19, explicitly.
- **A round-trip test that constructs an empty catalogue.** See §i18n — the empty map is the one input where old and new writers disagree.
- **Adding `-race` or a second `go test` invocation to pay for the new step.** The wasm build is ~5 s; the expensive thing in this repo is `go test -race ./internal/plugin/` at 297 s [VERIFIED: .github/workflows/security.yml:38–42], and that step is not touched.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON string escaping without HTML escaping | `quote()` at `tools/i18n/main.go:277–287` | `json.NewEncoder` + `SetEscapeHTML(false)` over the whole map, then `json.Indent(dst, src, "", "")` | D-13. The existing `quote()` already *is* this, applied one string at a time; the whole-map form is a generalisation, not a new dependency. Measured byte-exact over all seven catalogues. |
| Sorting map keys before writing | `sort.Strings(keys)` plus a manual loop | Let `encoding/json` sort them | Go's encoder sorts `map[string]string` keys byte-wise, identical to `sort.Strings`. Proven over the real key sets: 1 128 keys × 4 files + 51 + 9 + 4, all byte-identical. |
| Making a build deterministic | A post-processing step that strips metadata from the `.wasm` | `-trimpath -buildvcs=false CGO_ENABLED=0` and a pinned `GOTOOLCHAIN` | The Go toolchain guarantees this directly [CITED: go.dev/blog/rebuild]; a stripper would have to understand the wasm custom-section layout and would silently break on the next Go release. |
| Acquiring a specific Go version | Downloading a tarball in CI, or a second `setup-go` step | `GOTOOLCHAIN=go1.26.6` | The go command downloads it as a module, proxied through `GOPROXY` and checksum-verified [CITED: go.dev/doc/toolchain]. This machine is already running such a toolchain: `GOROOT=/Users/holz/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64` [VERIFIED: `go env GOROOT`, this session]. |
| Deciding whether a test is "in CI" | Checking `os.Getenv("CI")`, `GITHUB_ACTIONS`, or a build tag | The explicit `HOLZCLOUD_TEST_REQUIRE_WASM` variable D-10 defines | An explicit signal is greppable and can be set locally to reproduce a CI failure. |

**Key insight:** every problem in this phase already has a first-party answer inside the
Go toolchain. The phase's whole risk is in *not knowing which flag* — which is exactly
the defect `-buildvcs` turned out to be.

---

## Runtime State Inventory

The `.wasm` half of this phase is a build-artifact migration, so the inventory applies.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| **Stored data** | None — verified. `tools/wasm` writes no database; the plugin `store` tables are untouched; the catalogue files change zero bytes by design (D-13). | none |
| **Live service config** | None — verified. No plugin is installed in any live database by this phase; the admin's *Plugins* screen reads uploaded zips, not `plugins/*/plugin.wasm`. | none |
| **OS-registered state** | None — verified. No systemd unit, task or timer references a `.wasm` path (`deploy/` holds only `holzcloud.service` and the backup timer). | none |
| **Secrets / env vars** | One **new** variable, `HOLZCLOUD_TEST_REQUIRE_WASM`, read only by test code. It is not a secret and needs no store. `internal/config/config.go` must **not** learn about it — it is not application configuration. | add to `ci.yml`, `security.yml`, `release.yml` as workflow-level `env:` (D-10) |
| **Build artifacts** | **Six committed `.wasm` files**, 20.7 MB, all of them stale in a demonstrable way (see the table below). **Plus four committed `.zip` archives**, 3.7 MB, each containing a byte-identical copy of the `.wasm` beside it — verified by extracting and hashing. Rebuilding the six desynchronises the four. | rebuild the six in a `.wasm`-only commit (D-04); **decide** what happens to the four zips (Open Question 1) |

### The committed `.wasm` inventory — measured

[VERIFIED: `git show HEAD:<path>` piped through `shasum -a 256` and `strings`, this session]

| File | sha256 (16) | Bytes | Go version embedded | Module path stamped |
|------|-------------|-------|---------------------|---------------------|
| `plugins/bestellung/plugin.wasm` | `5e92a872bbf8b829` | 4 230 537 | **go1.24.7** | `github.com/holzcloud/cms/sdk` (pre-rename) |
| `plugins/jahreszahl/plugin.wasm` | `b3c00c92bd930a8e` | 3 097 425 | **go1.24.7** | `github.com/holzcloud/cms/sdk` (pre-rename) |
| `plugins/kontaktformular/plugin.wasm` | `56a67bd858ce1423` | 4 702 759 | **go1.26.4** | `github.com/holzcloud/holzcloud-cms/sdk` (current) |
| `plugins/nicht-gefunden/plugin.wasm` | `958aaa8d7fe667f1` | 3 367 421 | **go1.24.7** | `github.com/holzcloud/cms/sdk` (pre-rename) |
| `plugins/suche/plugin.wasm` | `1c3c4d2b8f0f46b9` | 3 093 975 | **go1.24.7** | `github.com/holzcloud/cms/sdk` (pre-rename) |
| `internal/plugin/testdata/echo.wasm` | `b13b065c248d72d8` | 3 251 976 | **go1.26.2** | `mod github.com/holzcloud/cms` (pre-rename) |

All six carry `vcs.modified=true`. Three different compilers; five of six built against a
module path that no longer exists. **This corrects the discussion's premise that "the
committed modules carry `go1.24.7`" — it is true of four of them.**

### Cost of the D-04 rebuild — measured

Fresh builds (local `go1.26.4` via `GOTOOLCHAIN=auto`, `-buildvcs=false`) vs committed:

| Module | Committed | Fresh | Δ |
|--------|-----------|-------|---|
| jahreszahl | 3 097 425 | 3 312 731 | +215 306 |
| suche | 3 093 975 | 3 418 746 | +324 771 |
| nicht-gefunden | 3 367 421 | 3 587 325 | +219 904 |
| kontaktformular | 4 702 759 | 4 702 646 | −113 |
| bestellung | 4 230 537 | 4 572 822 | +342 285 |
| echo | 3 251 976 | 3 253 624 | +1 648 |
| **TOTAL** | **21 744 093 (20.7 MB)** | **22 847 894 (21.7 MB)** | **+1.1 MB** |

The commit adds ~21.7 MB of new blobs to history (wasm does not delta-compress).
Repacking the four zips would add ~3.7 MB more. D-04's estimate of "~21 MB" is confirmed.

---

## Common Pitfalls

### Pitfall 1: `-buildvcs=auto` makes the byte comparison impossible

**What goes wrong:** `tools/wasm -check` is red on the first commit after the rebuild, and on every commit after that, forever.
**Why it happens:** the guest binary embeds `vcs.revision` = the git SHA at build time. The rebuild in CI runs at a later commit than the one that produced the committed file.
**How to avoid:** `-buildvcs=false` in the tool and in all three places the command is documented.
**Warning signs:** a `-check` failure where the two files differ by ~130 bytes and `strings <file> | grep vcs.revision` shows two different SHAs.

### Pitfall 2: the pinned toolchain and the local toolchain are different by default, and *which* one you get depends on which directory you are in

**What goes wrong:** two builds from the same working tree produce different bytes.
**Why it happens:** `GOTOOLCHAIN=auto` upgrades only when a `go.mod` demands it. `plugins/*/go.mod` says `go 1.24`, so the *installed* toolchain is used; the root `go.mod` says `go 1.26.6`, so building `echo` switches to go1.26.6. On this machine those are `go1.26.4` and `go1.26.6` respectively.
**Measured** [VERIFIED: local shell, this session]:
```
$ GOTOOLCHAIN=local go version
go version go1.26.4 darwin/arm64
$ go version                      # from the repository root
go version go1.26.6 darwin/arm64
```
and the same source built `-buildvcs=false` gives `59815ce1…` (go1.26.4, from `plugins/jahreszahl`) versus `629574a3…` (go1.26.6, `GOTOOLCHAIN=go1.26.6`).
**How to avoid:** this is exactly what D-03's pin fixes. Set `GOTOOLCHAIN` explicitly for **all six** targets, including `echo`, and never rely on `auto`.
**Warning signs:** `strings plugin.wasm | grep -o 'go1\.[0-9.]*'` disagreeing between two files built in the same session.

### Pitfall 3: the pin cannot be lower than the root module's `go` directive

**What goes wrong:** `tools/wasm` fails outright, on every target, the day someone raises `go.mod`'s `go` line.
**Why it happens:** `echo` lives in the root module. *"The Go toolchain refuses to load a module or workspace that declares a minimum required Go version greater than the toolchain's own version"* [CITED: go.dev/doc/toolchain].
**Measured** [VERIFIED: local shell, this session] — with a `go 1.27.0` directive and the pin at `go1.26.6`:
```
go: go.mod requires go >= 1.27.0 (running go 1.26.6; GOTOOLCHAIN=go1.26.6)
```
**How to avoid:** the pin constant in `tools/wasm` must be ≥ `go.mod`'s `go` directive (`go 1.26.6` today, so `go1.26.6` is the floor *and* the natural D-04 value). Put a comment on the constant saying so, and note in `docs/offene-punkte.md` §"Beim Weiterarbeiten" that raising the `go` line means raising the pin and rebuilding the six in the same commit.
**Warning signs:** a Dependabot / Go-version PR that turns `tools/wasm` red on all six targets at once, rather than on one.

### Pitfall 4: the four committed `.zip` archives go stale the moment the `.wasm` files are rebuilt

**What goes wrong:** Phase 6 creates, one layer up, exactly the defect MAINT-03 exists to close.
**Why it happens:** `plugins/{bestellung,jahreszahl,nicht-gefunden,suche}/*.zip` each contain a copy of the `plugin.wasm` beside them — verified byte-identical this session by extracting and hashing. `plugins/kontaktformular` has no zip (it has `migrations/0001_uebernahme.sql`, which `zip -j` would flatten wrongly).
**How to avoid:** decide it in the plan, do not discover it in review. See Open Question 1.
**Warning signs:** none — nothing checks the zips today. That is the point.

### Pitfall 5: `Encoder.SetIndent("", "")` and `json.MarshalIndent` are both wrong here

**What goes wrong:** the catalogues reformat into one line, or every `<code>` becomes `<code>`.
**Why it happens:** `SetIndent("", "")` is a no-op; `MarshalIndent` HTML-escapes with no switch.
**How to avoid:** the exact pair D-13 names — `Encoder`+`SetEscapeHTML(false)`, then the free function `json.Indent(dst, src, "", "")`.
**Warning signs:** the `git diff --exit-code` step from D-15 going red with a 4 500-line diff.

### Pitfall 6: the round-trip test tripping on the empty map

**What goes wrong:** a table-driven round-trip test with an "empty catalogue" case fails.
**Why it happens:** today's writer emits `{\n}\n` for an empty map; the stdlib route emits `{}\n`. **Measured this session — this is the only divergence found across every input tried.**
**How to avoid:** the round-trip test reads the seven real files (D-14) and does not synthesise an empty one. A catalogue is never empty in practice.

### Pitfall 7: `security.yml` has no `fetch-depth`

**What goes wrong:** nothing in this phase — but note it before adding anything git-aware there.
**Why it happens:** `security.yml:20` checks out shallow; `ci.yml:26–28` and `release.yml:26–30` set `fetch-depth: 0` for `git describe`. D-10 only adds an `env:` line to `security.yml`, so this is safe as scoped.

---

## Code Examples

### The new `writeCatalog` (MAINT-01, D-13)

```go
// Source: derived from tools/i18n/main.go:289-307, measured byte-exact against
// all seven committed catalogues in this session.

// writeCatalog writes the file sorted and flush left, one key per line, so a
// change to one string is one line in a diff rather than a reshuffled file.
//
// The encoder is asked not to escape HTML: these files are dictionaries a
// translator reads, half the sentences contain a <code> or an &amp;, and
// "<code>" is not something anybody should have to decipher. The
// escaping is not needed either — nothing serves these files to a browser.
//
// json.Indent with an empty prefix and an empty indent is what produces the
// flush-left, one-entry-per-line shape. Encoder.SetIndent("", "") does NOT:
// it is a no-op that emits a single line. json.MarshalIndent is not usable
// either, because it always escapes HTML and offers no switch.
func writeCatalog(path string, catalog map[string]string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(catalog); err != nil {
		return err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, buf.Bytes(), "", ""); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}
```

`quote()` (`tools/i18n/main.go:277–287`) is deleted; `sort` may lose its import if nothing
else in the file uses it — `main()` at `:281` still calls `sort.Strings(sorted)`, so it stays.
[VERIFIED: tools/i18n/main.go:281 — `sort.Strings(sorted)`]

### The round-trip test (MAINT-01, D-14)

```go
// Source: tools/i18n/main_test.go — new file; tools/i18n has no test today.

// Jede Katalogdatei muss durch readCatalog und writeCatalog gehen, ohne dass
// sich ein Byte ändert. Das ist die Sperre gegen ein Format, das durch eine
// Übersetzungsrunde von Hand wegdriftet — genau das ist am 2. September
// passiert (siehe .planning/WINDOWS.md, Eintrag 1).
func TestKatalogeUeberlebenDenRundlauf(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "i18n", "locales")
	eintraege, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var gesehen int
	for _, e := range eintraege {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		gesehen++
		pfad := filepath.Join(dir, e.Name())
		wieAufPlatte, err := os.ReadFile(pfad)
		if err != nil {
			t.Fatal(err)
		}
		katalog, err := readCatalog(pfad)
		if err != nil {
			t.Fatal(err)
		}
		ziel := filepath.Join(t.TempDir(), e.Name())
		if err := writeCatalog(ziel, katalog); err != nil {
			t.Fatal(err)
		}
		neu, err := os.ReadFile(ziel)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(wieAufPlatte, neu) {
			t.Errorf("%s: writeCatalog liefert nicht die Datei zurück, die es gelesen hat (%d Bytes auf Platte, %d geschrieben)",
				e.Name(), len(wieAufPlatte), len(neu))
		}
	}
	if gesehen != 7 {
		t.Errorf("7 Kataloge erwartet, %d gefunden — eine neue Sprache gehört in denselben Rundlauf", gesehen)
	}
}
```

The `gesehen != 7` assertion is what makes the test a *lock* rather than a spot check:
a new locale added without thought fails it, which is the moment to look.

### The shared wasm helper (MAINT-04, D-10, D-11)

```go
// Source: internal/plugin/wasmtest/wasmtest.go — new package.

// Package wasmtest entscheidet an einer Stelle, was passiert, wenn ein
// gebautes Plugin-Modul fehlt.
//
// Auf dem Rechner eines Mitwirkenden ist ein fehlendes .wasm kein Fehler: er
// hat vielleicht nur einen Teil des Baums ausgecheckt. Auf einem Läufer ist es
// einer — ein Test, der sich selbst überspringt, meldet grün und prüft nichts.
// HOLZCLOUD_TEST_REQUIRE_WASM unterscheidet die beiden Fälle; die drei
// Arbeitsabläufe, die Tests ausführen, setzen es.
package wasmtest

import (
	"os"
	"testing"
)

// Modul liest ein gebautes .wasm für einen Test. pfad ist relativ zum
// Verzeichnis des Tests.
func Modul(t *testing.T, pfad string) []byte {
	t.Helper()
	b, err := os.ReadFile(pfad)
	if err == nil {
		return b
	}
	const bauen = "gebaut wird es mit: go run ./tools/wasm"
	if os.Getenv("HOLZCLOUD_TEST_REQUIRE_WASM") != "" {
		t.Fatalf("%s fehlt und HOLZCLOUD_TEST_REQUIRE_WASM ist gesetzt: %v\n%s", pfad, err, bauen)
	}
	t.Skipf("%s fehlt: %v\n%s", pfad, err, bauen)
	return nil
}
```

Call-site rewrites, all five of them one line each:

```go
// internal/plugin/runtime_test.go:19-26  (package plugin)
func echoModul(t *testing.T) []byte { return wasmtest.Modul(t, "testdata/echo.wasm") }

// internal/plugin/sdk_e2e_test.go:21-24  (package plugin_test)
modul := wasmtest.Modul(t, "../../plugins/jahreszahl/plugin.wasm")

// internal/plugin/hofladen_e2e_test.go:21-24
modul := wasmtest.Modul(t, "../../plugins/bestellung/plugin.wasm")

// internal/public/formular_e2e_test.go:158-161  (package public)
modul := wasmtest.Modul(t, "../../plugins/kontaktformular/plugin.wasm")

// internal/public/suche_e2e_test.go:21-24
modul := wasmtest.Modul(t, "../../plugins/suche/plugin.wasm")
```

`os.Getenv(...) != ""` rather than `== "1"`: D-10 sets `=1`, but a contributor
reproducing a CI failure will reach for `true` or `yes`, and the strict form would
silently keep skipping.

### The two new CI steps

```yaml
# .github/workflows/ci.yml — inserted after "Vet" and before "Build".
# Placement is D-08: on the critical path, before go test ./... .

    # Ein festgeschriebenes .wasm im Baum ist nur dann eine Zusage, wenn
    # jemand nachrechnet. Ohne diesen Schritt prüft eine SDK-Änderung gegen
    # ein Modul, das vor ihr gebaut wurde — ein Grün, das nichts bedeutet.
    - name: Verify the committed plugin modules are current
      run: go run ./tools/wasm -check

    # Zwilling zu "Verify go.mod is tidy" weiter oben: das erzeugte Erzeugnis
    # muss mit der Quelle Schritt halten. Beweist NICHT "0 offen" — -write legt
    # einen fehlenden Schlüssel mit leerem Wert an, danach ist der Diff sauber
    # und die Übersetzung fehlt weiterhin.
    - name: Verify the message catalogues are in step
      run: |
        go run ./tools/i18n -write
        go run ./tools/i18n -schweiz
        git diff --exit-code -- internal/i18n/locales/
```

And the workflow-level `env:` in all three (D-10), added beside the existing
`CGO_ENABLED: "0"` at `ci.yml:15–17`, `security.yml:11–12`, `release.yml:12–14`:

```yaml
env:
  CGO_ENABLED: "0"
  # Ein übersprungener Test meldet grün. Auf einem Läufer ist ein fehlendes
  # plugin.wasm ein Fehler, auf einem Entwicklungsrechner nicht.
  HOLZCLOUD_TEST_REQUIRE_WASM: "1"
```

---

## CI Cost (MAINT-03, D-08)

**Measured this session** on darwin/arm64 (M-series), with a *completely empty*
`GOCACHE`, `-buildvcs=false`, the full documented flag set:

| Build | Wall | User CPU | Note |
|-------|------|----------|------|
| `plugins/jahreszahl` (first, cold cache) | **1.81 s** | 5.54 s @ 371 % | includes the wasip1 standard-library packages it imports |
| `plugins/suche` | 0.26 s | 0.40 s | warm |
| `plugins/nicht-gefunden` | 0.15 s | 0.21 s | warm |
| `plugins/kontaktformular` | 0.44 s | 1.45 s | warm |
| `plugins/bestellung` | 0.17 s | 0.21 s | warm |
| `internal/plugin/testdata/echo` | **1.76 s** | 5.54 s @ 378 % | a second cold wave — see below |
| **Total, empty cache** | **≈ 4.6 s** | | resulting `GOCACHE`: **63 MB** |

Two things to understand from this:

1. **The roadmap's budget worry does not apply.** The 297 s figure at
   `security.yml:38–42` is `go test -race ./internal/plugin/`; it is the wazero host
   under the race detector, not a compile. Six wasip1 guests cost single-digit seconds.
   Even allowing a hosted `ubuntu-latest` runner to be 3–4× slower than this machine,
   the step lands around 15–20 s cold and 2–4 s warm.
2. **`echo` pays a second cold wave** even with the cache warm from the five plugins.
   The plugin modules declare `go 1.24` while the root module declares `go 1.26.6`;
   the language version feeds the `DefaultGODEBUG` string baked into the binary (visible
   in the committed `kontaktformular` build info), so the two sets of objects do not share
   cache entries. Budget for two cold waves, not one. Bumping `plugins/*/go.mod` to
   `1.26.6` would collapse them into one — but that is explicitly Deferred, and the saving
   is ~1.8 s, so leave it deferred.

### Does `actions/setup-go` `cache: true` help?

Yes, and no configuration change is needed — but for a reason worth writing down.

- `setup-go` caches *"Go modules and build outputs"*, and *"By default, the action looks
  for `go.mod` in the repository root and uses its hash as part of the cache key"*, with
  `cache-dependency-path` available *"when you have multiple dependency files, or when
  they're located in different subdirectories"* [CITED: github.com/actions/setup-go README].
  Note the v6 change recorded there: *"By default, the cache key for Go modules is based
  on `go.mod`. To use `go.sum`, configure the `cache-dependency-path` input."* The repo
  pins `actions/setup-go@b7ad1dad…  # v7.0.0` [VERIFIED: .github/workflows/ci.yml:31–36].
- **This repository has exactly one `go.sum` and it is at the root**
  [VERIFIED: `find . -name go.sum` → `./go.sum` only]. `sdk/go.mod` and the five
  `plugins/*/go.mod` have **no** `go.sum` at all, because their only requirement is the
  locally `replace`d SDK. So whichever of `go.mod`/`go.sum` the action keys on, adding
  the wasm step does not change the key, does not fragment the cache, and does not need
  `cache-dependency-path`.
- The cached directory that matters here is the **build cache** (`~/.cache/go-build`),
  and 63 MB of wasip1 objects is small next to what the amd64 build and test already put
  there.

**Recommendation: leave `cache: true` exactly as it is.** Do not add
`cache-dependency-path`. The one caveat to record in the plan: GitHub's cache is written
on a key *miss*, so the wasip1 objects first land in the cache on the first run after a
`go.mod`/`go.sum` change — i.e. the run that is cold anyway. That is the correct
behaviour and needs no work.

---

## MAINT-05 Correction Inventory

All line numbers and all "actual" values verified against the working tree at
`8edcd9cb0ac8350c7916f43c50882b3d10af1e1b`, this session.

> **This section contradicts a planning note, deliberately.** ROADMAP success criterion 5
> and REQUIREMENTS.md MAINT-05 both say `ARCHITECTURE.md` carries **six** drifted facts.
> It carries more than twenty. D-01 already opens both documents for amendment; the
> "six" should be amended with them, or the phase closes on a criterion that is itself
> a stale note.

### A. `.planning/codebase/ARCHITECTURE.md`

**Countable facts** (the class D-19 names explicitly):

| Line | Claim | Actual | Evidence |
|------|-------|--------|----------|
| 112 | `internal/db/migrations/` (**38** goose `.sql` files) | **45** | `ls internal/db/migrations/*.sql \| wc -l` = 45; last is `00045_pages_locale_unique.sql` |
| 63 | Admin handlers … (**47** files) | **49** non-test `.go` (55 including `_test.go`) | `ls internal/admin/*.go \| grep -v _test \| wc -l` |
| 166 | **nine** jobs registered in `cmd/holzcloud/main.go` | **ten** | ten `jobs.Job{` literals between `main.go:443` and `:530` |
| 212 | "every request pays for it **on a Raspberry Pi**" | target is **linux/amd64** since 2026-09-03 (quick task `x86-statt-arm`) | STATE.md "Target: linux/amd64 single binary (retargeted from arm64/Pi on 2026-09-03)"; `ci.yml:56–58` |
| 1, 4, 254 | `Analysis Date: 2026-08-22` (three places) | stale | — |
| 35–36 | domain-package list: `page menu media term snippet field kind block domain user tmplmgr plugin mail sharelink i18n branding locale bundle` | `internal/` holds **38** packages; missing from the list: `activity`, `design`, `money`, `outbox`, `payrexx`, `shop`, `structured`, `textdiff`, `totp`, `wxr`, `tmplspec` | `ls internal \| wc -l` = 38 |

**Drifted line references** (a second class; the numbers moved because code moved):

| Line | Says | Actual | How verified |
|------|------|--------|--------------|
| 57, 178 | `newRouter`, line ~572 | **630** | `grep -n 'func newRouter' cmd/holzcloud/main.go` |
| 170 | `main()` — `cmd/holzcloud/main.go:59` | **64** | `grep -n '^func main('` |
| 174 | `runCLI()` — `cmd/holzcloud/cli.go:56` | **53** | `grep -n 'func runCLI'` |
| 118 | outer chain at `main.go:941-945` | **1066–1069** | `grep -n 'RequestID(\|AccessLog(\|Recoverer(\|SecureHeaders'` |
| 122 | `HandlePage` — `internal/public/handler.go:193` | **209** | `grep -n 'func (h \*Handler) HandlePage\b'` |
| 126 | `RenderPage` — `internal/template/loader.go:483` | **702** | `grep -n 'func (l \*Loader) RenderPage'` |
| 127 | `serveCached` — `internal/public/handler.go:351` | **387** | `grep -n 'func.*serveCached'` |
| 131 | admin chain — `cmd/holzcloud/main.go:882` | **722** (`adminProtectedMux := http.NewServeMux()`) | `grep -n 'adminProtectedMux'` |
| 132, 154 | `internal/admin/handler.go:187` (`ErrHandler`) | **201** | `grep -n 'func.*ErrHandler'` |
| 154 | `internal/public/handler.go:102` (`ErrHandler`) | **118** | same |
| 158 | `loader.go:394` (`cacheKey`), `:543` (`InvalidateTemplateCache`) | **613** and **770** | `grep -n 'cacheKey\|InvalidateTemplateCache'` |
| 162 | sharelink signer wired at `main.go:202` | **223** | `grep -n 'sharelink\.'` |
| 201 | `internal/page/markdown.go:23-47` | the sanitizer is at **49**; the goldmark→bluemonday contract is documented at **76–78** | `grep -n 'bluemonday\|template.HTML' internal/page/markdown.go` |
| 207 | `internal/public/handler.go:220` | line 220 is now inside an unrelated comment about the archive route | `sed -n '218,222p'` |
| 213 | `usersExist atomic.Bool` at `main.go:297` | **333** | `grep -n 'usersExist'` |
| 225 | `WithSettings` etc. at `main.go:266`, `:904` | **309** (`WithSettings`), **991–993** (`WithPages`/`WithRender`/`WithNotify`) | `grep -n 'WithSettings\|WithPages'` |

**Verified still correct — do not touch:**
`:46` read pool max 5 (`internal/db/db.go:50`); `:119` `internal/domain/resolver.go:48`;
`:138` 2 s `CallTimeout`, 16 MB memory cap, 8 MB payload cap (`internal/plugin/runtime.go:24`,
`:31` `MemoryPages = 256`, `:34` `MaxPayloadBytes = 8 << 20`); `:188` write pool 1 conn
with `_txlock=immediate` (`internal/db/db.go:28,42`); `:189` `resizeSlots`
(`internal/media/variants.go:91`); `:213` `branding.Load` (`internal/branding/branding.go:83`).

### B. `.planning/codebase/STACK.md`

| Line | Claim | Actual |
|------|-------|--------|
| 3 | `Analysis Date: 2026-08-22` | stale |
| 8 | Go **1.26.2** (`go.mod`) | **1.26.6** |
| 9 | **38** goose migrations (`00001_*.sql` … `00038_block_types.sql`) | **45** … `00045_pages_locale_unique.sql` |
| 12 | themes `{default,journal,magazine,midnight,rudel,schlicht,weide}` | **eight** — add `holzcloud` |
| 20 | Primary target `linux/arm64` (Raspberry Pi 5) | **linux/amd64** |
| 52 | `modernc.org/sqlite` **v1.48.2** … "ARM64 cross-compilable" | **v1.57.0**; the ARM64 phrasing is now beside the point |
| 53 | `pressly/goose/v3` **v3.27.0** | **v3.27.3** |
| 54 | `golang.org/x/crypto` **v0.54.0** | **v0.55.0** |
| 56 | `yuin/goldmark` **v1.8.2** | **v1.8.5** |
| 60 | `golang.org/x/image` **v0.44.0** | **v0.45.0** |
| 61 | `golang.org/x/net` **v0.57.0** | **v0.58.0** |
| 88 | Raspberry Pi 5 (`linux/arm64`) via systemd | **linux/amd64** |

Correct and untouched: `:34` scs v2.9.0, `:35` csrf v1.7.3, `:55` wazero v1.12.0 and its
three limits, `:57` bluemonday v1.0.27, `:62` `rsc.io/qr` v0.2.0.

### C. `.planning/codebase/STRUCTURE.md`

| Line | Claim | Actual |
|------|-------|--------|
| 3 | `Analysis Date` | stale |
| 15 | `internal/` … (**33** dirs) | **38** |
| 16 | **38** goose `.sql` files | **45** |
| 23 | `k8s/  # Namespace, app, Caddy, RBAC manifests` | **the directory no longer exists** — top level is `build cmd data deploy docs internal plugins sdk sites tools` |
| 38–44 | package inventory | missing `activity`, `money`, `outbox`, `payrexx`, `shop`, `textdiff`, `tmplspec` |
| 42 | `admin/` (**47** files, largest) | **49** |
| 52 | each plugin has "…, distributable `.zip`, optional `migrations/`" | four of five have a `.zip`; `kontaktformular` has `migrations/` and no zip |
| 80 | themes list (seven) | **eight** — add `holzcloud` |

### D. `.planning/codebase/INTEGRATIONS.md` — the most drifted map of the seven

| Line | Claim | Actual |
|------|-------|--------|
| 3 | `Analysis Date` | stale |
| 53 | "Argon2id …, parameters tunable **for the Pi**" | linux/amd64 |
| 70 | `/healthz` — `cmd/holzcloud/main.go:610` | **671** |
| 71 | `/readyz` — `cmd/holzcloud/main.go:616` | **677** |
| 76 | Hosting: Raspberry Pi 5 via systemd behind Caddy | linux/amd64 |
| 77 | Kubernetes manifests `k8s/20-app.yaml`, `k8s/30-caddy-shared.yaml`, `k8s/40-deployer-rbac.yaml` | **`k8s/` does not exist** |
| 80 | Container registry via `.github/workflows/deploy.yml` | **`deploy.yml` does not exist**; the publisher is `image.yml` (GHCR) and `release.yml` (binary on a `v*` tag) |
| 83 | `ci.yml` … "**plus a `linux/arm64` cross-compile job**" | `ci.yml` has **one** job (`test`), building linux/amd64 |
| 84 | `.github/workflows/deploy.yml` — `workflow_run`, `kubectl rollout status` | **does not exist** |
| 101 | Kubernetes `Secret` … see `k8s/README.md`, `k8s/10-secret.example.yaml` | **`k8s/` does not exist** |
| 102 | "**On the Pi:** environment lines in `deploy/holzcloud.service`" | the unit exists; the Pi does not |

Correct and untouched: `:85` the `security.yml` description; `:88` the backup story;
`:93–98` the environment variables; `:108–109` the `/ai` MCP endpoint.

### E. `.planning/codebase/CONCERNS.md`

| Line | Claim | Actual |
|------|-------|--------|
| 1, 4 | `refreshed: 2026-08-22` / `Analysis Date: 2026-08-22` | stale |
| 184 | `internal/db/migrations/` (**38** files, through `00038_block_types.sql`) | **45**, through `00045_pages_locale_unique.sql` |
| 244 | "**Go 1.26.2** in `go.mod` vs. Go 1.22+ in `CLAUDE.md`" | `go.mod` says **1.26.6**. The *concern itself is still open* — `CLAUDE.md:7` still says "Go 1.22+". Correct the number, keep the finding. |

D-19 is exactly right about this file: it is the one map whose value is judgement.
Correct the two numbers and the date; touch nothing else.

### F. `.planning/codebase/TESTING.md`

| Line | Claim | Actual |
|------|-------|--------|
| 3 | `Analysis Date` | stale |
| 8 | Go stdlib `testing` (**Go 1.26.2**, `go.mod`) | **1.26.6** |

`:26–27`'s description of `ci.yml` and `security.yml` is accurate today and stays.
Note for the plan: this phase *changes* what `:26–27` describes (two new CI steps, one new
env var), so it should be updated **after** those steps land, not before.

### G. `.planning/codebase/CONVENTIONS.md`

| Line | Claim | Actual |
|------|-------|--------|
| 3 | `Analysis Date` | stale |
| 87 | `ErrHandler` … `internal/admin/handler.go:187` | **201** |
| 22 | `SetPlugins`, `SetMail`, `SetAITokens` (`internal/admin/handler.go:196-211`) | shifted with the same delta (+14) — re-derive rather than assume |
| 110 | `internal/admin/handler.go:196-204` | same shift |

No countable facts in this file are wrong. If D-19's scope is read strictly ("countable
facts"), CONVENTIONS.md needs only the date stamp. Recommendation: stamp the date, fix
`:87` because it is the same `ErrHandler` reference already being fixed in
`ARCHITECTURE.md:132`, and leave the rest.

### H. `docs/offene-punkte.md` (D-21, D-22)

| Line | Action | Detail |
|------|--------|--------|
| 87 | remove the item from the numbered list | `## 7. Dependabot: erledigt, und wie es das nächste Mal geht` is the **last** numbered heading (the list runs `## 1.` at :16 through `## 7.` at :87), so removing it needs **no renumbering** — a real convenience. |
| 93–110 | move to `## Beim Weiterarbeiten` | The procedure is longer than D-21's `:93–105` estimate: it runs from `:93` ("**Vorgehen beim nächsten Mal:** einer nach dem anderen …") through `:110`, and includes the fenced `HOLZCLOUD_DATA_DIR=/tmp/dbtest …` block at `:103–105` and the expected-pragma paragraph at `:107–110`. **Line `:109` also carries a stale count** — "Bei 1.57.0 geprüft: 44 Wanderungen, 48 Tabellen" — which is a *historical measurement*, correctly dated, and should be kept as written rather than "corrected". |
| 138 | destination | `## Beim Weiterarbeiten` |
| 140 | `00044` → `00045` | Verified: 45 files, last `00045_pages_locale_unique.sql`. |

**Add while there** (this phase creates the fact): a `## Beim Weiterarbeiten` bullet
saying that `tools/wasm` pins the guest compiler, that the pin must not fall below
`go.mod`'s `go` directive, and that raising the Go version means running
`go run ./tools/wasm` and committing the six `.wasm` files on their own. Without it,
Pitfall 3 is discovered by whoever merges the next Go bump.

### I. The three `deferred-items.md` (D-20)

All three verified closed this session by reading the files and the current code:

| File | Finding | Closed by |
|------|---------|-----------|
| `.planning/quick/260902-cml-i18n-kataloge-sauber-0-offen-0-verwaist/deferred-items.md` | `writeCatalog` writes flush-left while four catalogues carried a two-space indent | Route 1 of the two the note itself offers was taken: quick task `260903-bsk` normalised the four files; the tool's format is canonical, recorded as waived entry 1 in `.planning/WINDOWS.md`. Re-verified this session: 7/7 catalogues are byte-identical to what `writeCatalog` produces. |
| `.planning/quick/260903-ceo-beispiel-bundle-statt-kundendaten/deferred-items.md` | `tools/mkbundle:177` checked `page.terms` against `t.Slug` while export/import use `t.Name` | Quick task `260903-zwei-abgelegte-fehler` (STATE.md: "mkbundle prüft die Schlagwörter jetzt gegen die Namen statt die Kennungen"). |
| `.planning/quick/260903-da5-kleinigkeiten-aus-der-freigabepruefung/deferred-items.md` | flaky `strings.Contains(seite, "f_")` in `internal/public/formular_e2e_test.go` (~1.25 %) | Same quick task ("Der flatternde Formulartest prüft jetzt die Form statt zwei Zeichen im Rauschen, 400 Läufe grün"). |

The stamp goes on the line **after** the `#` heading of each file, per D-20 — i.e. after
`:1` in all three.

---

## i18n Format Lock and Self-Description

### Round-trip: re-measured independently this session

A probe program was written that implements both writers — the current
`writeCatalog` body copied verbatim from `tools/i18n/main.go:289–307`, and the proposed
stdlib route from D-13 — and ran both over every committed catalogue.

[VERIFIED: local probe, this session]

| File | Keys | writer == disk | proposed == disk | proposed == writer | Bytes |
|------|------|----------------|------------------|--------------------|-------|
| `de-CH.json` | 51 | true | **true** | true | 12 471 |
| `en.json` | 1 128 | true | **true** | true | 96 517 |
| `es.json` | 1 128 | true | **true** | true | 101 265 |
| `fr-CH.json` | 4 | true | **true** | true | 1 089 |
| `fr.json` | 1 128 | true | **true** | true | 103 465 |
| `it-CH.json` | 9 | true | **true** | true | 2 595 |
| `it.json` | 1 128 | true | **true** | true | 100 848 |

Identical results under `GOEXPERIMENT=jsonv2` — worth knowing, because Go 1.25+ ships a
second `encoding/json` implementation behind that experiment and a contributor who
enables it must not silently change the catalogue format.

**Specific questions from the brief, answered:**

- **Trailing newline** — preserved. `Encoder.Encode` appends `\n`; `json.Indent` copies it through. The proposed output ends `}\n`, byte-for-byte with the current `b.WriteString("}\n")`.
- **The `": "` separator** — `json.Indent` writes a space after a colon inside an object, matching the current `b.WriteString(": ")`. Confirmed by the byte-equality above, not by reading the implementation.
- **`sort.Strings` vs the encoder's map ordering** — identical over all seven real key sets, including 1 128-key catalogues containing German sentences with umlauts, guillemets, `<code>` markup and `&amp;`. Both sort byte-wise.

**Edge cases probed** (both writers, compared):

| Input | Current writer | Proposed | Equal |
|-------|----------------|----------|-------|
| control characters `\x01`, `\x1f` | `{\n"ab": "cd"\n}\n` | same | ✅ |
| newline inside key and value | `{\n"zeile\neins": "wert\nzwei"\n}\n` (escaped) | same | ✅ |
| HTML `<code>&amp;</code>` | unescaped | unescaped | ✅ |
| tab, quote, backslash | escaped identically | same | ✅ |
| `U+2028` / `U+2029` | ` ` / ` ` | same | ✅ |
| emoji + guillemets | literal UTF-8 | same | ✅ |
| invalid UTF-8 (`\xff`) | `�` | `�` | ✅ |
| **empty map** | `{\n}\n` | `{}\n` | ❌ **the only divergence** |

The empty map is unreachable: `writeCatalog` is called from `writeSwiss`
(`main.go:255`), which always has at least the rule-derived entries, and from the
`-write` path, which writes a full catalogue. But a table-driven round-trip test that
includes an "empty" case would fail on it, so **write the test against the seven real
files** (which D-14 already prescribes).

### MAINT-02: the doc comment does not say what the requirement thinks it says

Read this session [VERIFIED: tools/i18n/main.go:1–15]. The package doc comment is:

```
// Command i18n keeps the message catalogues in step with the source.
//
//	go run ./tools/i18n            # report what is missing
//	go run ./tools/i18n -write     # add the missing keys with empty values
//	go run ./tools/i18n -schweiz   # rebuild de-CH.json from the German
//
// It collects every German string that reaches a person: the {{t}}, {{th}} and
// {{tf}} calls in the admin templates, and the flash messages, form errors and
// page titles in the Go code. …
```

**It makes no claim about indentation.** The false claim is one function comment further
down:

```
// writeCatalog writes the file sorted and indented, so a change to one string
// is one line in a diff rather than a reshuffled file.
```
[VERIFIED: tools/i18n/main.go:288 — the line immediately above `func writeCatalog` at `:289`]

So MAINT-02's "the doc comment no longer claims its output is indented" resolves to
**`tools/i18n/main.go:288`**, not to the package doc at `:1–15`. ROADMAP's planning note
locates it at `main.go:287`, which is inside `quote()` — off by one function.
**D-01 should correct that pointer too.**

D-17's two placements, with exact anchors:

1. **Package doc comment, `tools/i18n/main.go:1–15`** — add a paragraph stating that `de-CH.json` is rebuilt by `-schweiz` from a mechanical rule, while `fr-CH.json` and `it-CH.json` are maintained by hand and are only ever read. Nothing to *remove* here.
2. **`writeCatalog`'s own comment at `:288`** — "sorted and indented" becomes "sorted and flush left, one key per line".
3. **The per-regional-file report line at `:112`** [VERIFIED: tools/i18n/main.go:112 — `fmt.Printf("%-12s %d Abweichungen, %d ohne Gegenstück\n", e.Name(), len(catalog), wrong)`]. D-17 cites `:113`; `:113` is the closing `continue`. The line to change is **`:112`**. Suggested: append `— von Hand gepflegt, wird nicht geschrieben` for a file the tool never writes, and `— aus dem Deutschen erzeugt (-schweiz)` for `de-CH.json`. The branch that reaches this line is at `:104` (`if strings.Contains(strings.TrimSuffix(e.Name(), ".json"), "-")`), with the rationale comment at `:99–103`.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| "Go builds are mostly reproducible if you try" | Guaranteed bit-for-bit for `CGO_ENABLED=0 go build -trimpath`, on any host, for any target | Go 1.21 [CITED: go.dev/blog/rebuild] | D-05's premise is answerable by documentation as well as measurement |
| Version pinned by the installed toolchain | `GOTOOLCHAIN`, with toolchains distributed as modules `golang.org/toolchain@v0.0.1-goX.Y.Z-GOOS-GOARCH` | Go 1.21 [CITED: go.dev/doc/toolchain] | D-03 is idiomatic, not a workaround |
| `go build` produced identical bytes regardless of git state | VCS stamping on by default (`-buildvcs=auto`), recording `vcs.revision`/`vcs.time`/`vcs.modified` | Go 1.18 [CITED: pkg.go.dev/cmd/go] | **This is what breaks D-02 and it has been true for eight releases** |
| `GOOS=wasip1` guests were command modules only | `-buildmode=c-shared` produces a reactor module with `//go:wasmexport` entry points | Go 1.24 | already the repo's invocation; nothing to change |
| One `encoding/json` | A second implementation behind `GOEXPERIMENT=jsonv2` | Go 1.25 | verified not to change the catalogue output |

**Deprecated / outdated in this repo's own notes:**
- `plugins/build.sh` (roadmap planning note): never existed; D-06 replaces it with `tools/wasm`.
- `k8s/` and `.github/workflows/deploy.yml` (INTEGRATIONS.md): gone from the tree.
- `linux/arm64` / Raspberry Pi as the primary target (STACK.md, INTEGRATIONS.md, ARCHITECTURE.md): retargeted 2026-09-03.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | everything | ✓ | `go1.26.6` via `GOTOOLCHAIN=auto`; `go1.26.4` installed locally | — |
| `GOTOOLCHAIN` module download | D-03's pin on a machine without go1.26.6 | ✓ | `GOROOT` is already a downloaded toolchain module (`golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64`) | — |
| `GOPROXY` reachability | the above | ✓ | default (`proxy.golang.org`) | vendor a note in `docs/offene-punkte.md`; `GOSUMDB=off` makes toolchain downloads **fail**, not succeed unverified [CITED: go.dev/doc/toolchain] |
| wasip1 target support | the six guest builds | ✓ | built six guests this session from an empty `GOCACHE` | — |
| `git` | `-buildvcs` behaviour, `git diff --exit-code` steps | ✓ | in `PATH` | — |
| Docker / a linux/amd64 host | the *optional* cross-host falsification | ✗ (daemon started, image pull did not complete in-session) | — | run the falsification on the `ubuntu-latest` runner instead — see §D-05's "cheap empirical gate", which is better anyway because it tests the real host |
| Playwright MCP | QUAL-02's browser half of the standing gate | not probed | — | the phase adds no user-visible string, so the browser pass is a formality; run it anyway per ROADMAP criterion 6 |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** a linux/amd64 build host — replaced by a one-off CI step.

---

## Security Domain

This phase writes no request-handling code and adds no user-visible surface, so most of
ASVS is not engaged. Two categories are, and both are supply-chain rather than
application security.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no | `tools/wasm` takes no untrusted input; its target list is a compile-time constant |
| V6 Cryptography | **yes, narrowly** | `crypto/sha256` for artifact comparison. It is an integrity check against accident, not against an adversary who can also edit the tool. Do not describe it as tamper-evidence in the commit message. |
| V14 Configuration / Build | **yes** | the `GOTOOLCHAIN` pin, `CGO_ENABLED=0`, and CI action pinning |

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| A committed binary artifact drifting from its source, so a change is validated against a stale build | Spoofing (the artifact attests to source it was not built from) | Exactly what MAINT-03 adds: rebuild and compare in CI |
| Toolchain substitution during `GOTOOLCHAIN` download | Tampering | Go's checksum database verifies toolchain module downloads; note that `GOSUMDB=off` makes them **fail** rather than proceed unverified [CITED: go.dev/doc/toolchain] |
| CI action tag moved to different code | Tampering | The repo's own rule (`ci.yml:22–24`): pin to a commit SHA. `image.yml` violates it today — out of scope, worth capturing |
| `govulncheck@latest` installed at run time in `security.yml:65` | Tampering | Accepted deliberately by the existing comment at `:61–62` ("a CI tool installed outside the module, so `@latest` never touches `go.mod` or `go.sum`"). Not this phase's business. |

**One security-relevant thing this phase must not do:** `-buildvcs=false` removes the git
revision from the shipped plugin binaries. That is a *reduction* in provenance metadata.
It is the right trade here — the guests are reproducible artifacts checked into the same
repository, so the commit that contains them *is* the provenance — but it should be said
out loud in the `tools/wasm` doc comment, so nobody re-enables stamping later "for
traceability" and silently breaks `-check`.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A `GOOS=wasip1 GOARCH=wasm` build is byte-identical on darwin/arm64 and linux/amd64 given the same toolchain, `CGO_ENABLED=0`, `-trimpath` and `-buildvcs=false`. **Cited from go.dev/blog/rebuild; the falsification run did not complete in this session.** | §D-05 Verdict | D-02's byte comparison fails on the first CI run and the phase falls back to D-05's alternative. Cost: one CI run to discover, one CI-file change to fix. Mitigated by the "cheap empirical gate" task. |
| A2 | `os/exec` on Unix lets a later `KEY=` entry in `cmd.Env` override an earlier one, so `append(os.Environ(), "GOFLAGS=")` clears an inherited `GOFLAGS`. | §Pattern 2 | `tools/wasm` inherits a contributor's `GOFLAGS` and produces different bytes than CI. Cheap to make robust: build the env slice by filtering `os.Environ()` instead of appending. Recommend doing that regardless. |
| A3 | Six wasip1 builds on `ubuntu-latest` cost roughly 3–4× the 4.6 s measured here, i.e. ~15–20 s cold. | §CI Cost | Only a scheduling estimate. The measurement itself (4.6 s from an empty cache on this machine) is verified. |
| A4 | `internal/plugin/testdata/echo` recompiles its stdlib dependencies separately from the plugin modules because of the `go 1.24` vs `go 1.26.6` language-version difference. | §CI Cost | The *timing* (1.76 s) is measured; the *explanation* is inferred. Wrong only in the explanation, not the number. |
| A5 | `actions/setup-go@v7` with `cache: true` keys on the single root `go.sum`/`go.mod` and therefore is unaffected by adding the wasm step. | §CI Cost | If the key does fragment, the cache misses more often and the step costs its cold price every run — ~20 s, not a budget problem. |
| A6 | ~~The `.zip` archives in `plugins/*` are consumed only by a human uploading them in the admin.~~ **Resolved to VERIFIED during research:** `grep -rn 'jahreszahl.zip\|suche.zip\|bestellung.zip\|nicht-gefunden.zip' --include='*.go' .` returns nothing — no Go code, test or fixture reads them. | Pitfall 4 / Open Question 1 | none — nothing automated breaks. Open Question 1 is purely a repository-hygiene decision. |
| A7 | `HOLZCLOUD_TEST_REQUIRE_WASM` is a name no other tool in the ecosystem uses. | §MAINT-04 | Collision is implausible; the `HOLZCLOUD_` prefix is the repo's own convention (`internal/config/config.go:171`). |

---

## Open Questions

1. **What happens to the four committed `.zip` archives when the six `.wasm` files are rebuilt?**
   - *What we know:* `plugins/{bestellung,jahreszahl,nicht-gefunden,suche}/*.zip` each contain a byte-identical copy of the `plugin.wasm` beside them — verified by extraction this session. Total 3.7 MB. `plugins/kontaktformular` has no zip. Nothing in CI checks them.
   - *What's unclear:* no D-xx covers them. D-04's rebuild silently desynchronises four artifacts and thereby recreates, one level up, the exact defect MAINT-03 closes.
   - *Recommendation:* **have `tools/wasm` repack them.** It is ~30 lines with `archive/zip` (the same import `tools/mkbundle` already uses), it makes them deterministic (fixed `Modified` timestamps, unlike `zip -j`), and `-check` then covers them for free. Cost: +3.7 MB in the rebuild commit. The alternative — deleting them, since `plugins/README.md` already documents `zip -j plugin.json plugin.wasm` — is defensible and cheaper, but it removes something a non-technical operator can download and use. Put this to the user before planning locks it in.

2. **Does the "six drifted facts" figure in ROADMAP criterion 5 and REQUIREMENTS MAINT-05 get amended?**
   - *What we know:* `ARCHITECTURE.md` has 6 countable-fact drifts *and* 16 drifted line references; the other six maps carry another ~35 corrections between them.
   - *What's unclear:* whether D-19's "countable facts … and nothing else" is meant to exclude the line references. A planning map whose every line reference is wrong is not much more useful than one whose counts are wrong.
   - *Recommendation:* D-01 is already opening both documents. Replace "six drifted facts" with "the drifted facts and file references listed in `06-RESEARCH.md` §MAINT-05", and let this document be the checklist. That keeps the criterion checkable and stops it from being stale on the day it is written.

3. **Is the wasip1 stdlib recompile for `echo` worth removing?**
   - *What we know:* it costs ~1.8 s of the ~4.6 s total; bumping `plugins/*/go.mod` to `go 1.26.6` would probably collapse the two waves into one.
   - *Recommendation:* leave it. The saving is trivial and the bump is explicitly Deferred.

4. **Should `tools/wasm -check` also verify the *embedded Go version* rather than only the hash?**
   - *What we know:* a hash mismatch already implies a version mismatch, but a hash mismatch alone reports nothing a human can act on.
   - *Recommendation:* no extra check, but the failure message should print both files' embedded `goX.Y.Z` (it is a `strings`-level scan of the wasm), because "the pin moved" and "the source changed" are the two very different causes and the message should distinguish them at a glance.

---

## Sources

### Primary (HIGH confidence)

- **The working tree at `8edcd9cb0ac8350c7916f43c50882b3d10af1e1b`**, read directly this session: `tools/i18n/main.go`, `tools/mkbundle/main.go`, `.github/workflows/{ci,security,release,image}.yml`, the five test call sites, `internal/plugin/runtime.go`, `internal/db/db.go`, `cmd/holzcloud/{main.go,cli.go}`, all seven `.planning/codebase/*.md`, `docs/offene-punkte.md`, the three `deferred-items.md`, `plugins/README.md`, `internal/plugin/testdata/README.md`, `plugins/*/go.mod`, `sdk/go.mod`, `go.mod`.
- **Local measurement, this session:** build reproducibility experiments E1–E5 (see §D-05), the `GOTOOLCHAIN` experiments (Pitfalls 2 and 3), the cold/warm build timings, the six committed `.wasm` build-info blobs, the four `.zip` extraction hashes, and the i18n writer probe over all seven catalogues plus eight edge cases.

### Secondary (MEDIUM confidence)

- [go.dev/blog/rebuild](https://go.dev/blog/rebuild) — "Perfectly Reproducible, Verified Go Toolchains". Cross-host reproducibility, the role of `CGO_ENABLED=0` and `-trimpath`.
- [go.dev/doc/toolchain](https://go.dev/doc/toolchain) — `GOTOOLCHAIN=<name>` semantics, the refusal to load a module requiring a newer version, toolchains as `golang.org/toolchain` modules, `GOSUMDB=off` behaviour.
- [pkg.go.dev/cmd/go](https://pkg.go.dev/cmd/go) — `-trimpath` and `-buildvcs` (default `auto`, what is stamped).
- [github.com/actions/setup-go](https://github.com/actions/setup-go) — `cache` / `cache-dependency-path` semantics and the v6 key change.

### Tertiary (LOW confidence)

- None. Everything in this document is either measured here or cited to an official Go / GitHub source.

---

## Metadata

**Confidence breakdown:**

- **Standard stack:** HIGH — stdlib only, no external package, every import already present in the repo.
- **Architecture (`tools/wasm`, the helper package, the CI steps):** HIGH — the module set, the packages, the flag set and the failure modes were all read or measured.
- **Reproducibility (D-05):** MEDIUM-HIGH — the decisive finding (`-buildvcs`) is VERIFIED by direct experiment; the host axis is CITED to the Go project and not falsified in this environment. The plan carries a cheap gate that settles it.
- **CI cost:** HIGH for the measurement, MEDIUM for the runner extrapolation.
- **i18n:** HIGH — re-measured over all seven real catalogues plus edge cases, twice (default and `jsonv2`).
- **MAINT-05 inventory:** HIGH — every line number and every "actual" value was checked against the tree in this session.
- **Pitfalls:** HIGH — five of the seven are direct measurements, including their exact error text.

**Research date:** 2026-09-04
**Valid until:** 2026-10-04 for the i18n and MAINT-05 findings (the line numbers drift with every commit — re-verify §MAINT-05 against `HEAD` before executing it). The `-buildvcs` and `GOTOOLCHAIN` findings are properties of the Go toolchain and are valid until the next Go minor release.
