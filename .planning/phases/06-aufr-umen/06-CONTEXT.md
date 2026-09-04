# Phase 6: Aufräumen - Context

**Gathered:** 2026-09-04
**Status:** Ready for planning

> Written in English to match `ROADMAP.md` and `REQUIREMENTS.md`, which are the
> documents downstream agents read alongside this one. The discussion itself was
> held in German; `06-DISCUSSION-LOG.md` keeps it verbatim.

<domain>
## Phase Boundary

Repair the milestone's own ground truth before it adds a single string:

1. `tools/i18n` writes through the standard library and a round-trip test locks
   the format so a later hand-translation pass cannot reintroduce drift.
2. CI rebuilds every committed `.wasm` and compares it against the file in the
   tree, so an SDK or ABI change can no longer be validated against a stale
   binary — and only then are the five self-skipping tests promoted to failures.
3. Every planning note that Phases 7–10 are planned against reads true against
   the working tree.

Requirements: MAINT-01, MAINT-02, MAINT-03, MAINT-04, MAINT-05

**Not in this phase:** any new user-visible string, any field kind, any admin
screen. The standing gate's translation half is expected to be trivially green;
it is run anyway, because that is what makes the number mean something in
Phases 7–10.

</domain>

<decisions>
## Implementation Decisions

### Ground truth first (ordering)

- **D-01:** The **first** task of the phase amends `REQUIREMENTS.md` and
  `ROADMAP.md` to match the decisions below, in its own docs commit, before any
  code is touched: MAINT-05 widened to all seven codebase maps (D-14), the
  planning note `plugins/build.sh` corrected to `tools/wasm` (D-06), and the
  toolchain pin (D-02) added as a planning note. A phase whose purpose is
  correct ground truth must not leave its own two source documents contradicting
  its context file.

### Wasm rebuild and comparison (MAINT-03, MAINT-04)

- **D-02:** The byte comparison from success criterion 3 is **kept**, and made
  stable by pinning the toolchain that builds the guests. Measured during
  discussion: two builds with the same Go produce an identical hash, but the
  committed modules carry `go1.24.7` in the binary while a local rebuild with
  `go1.26.4` differs by ~215 KB. The hash tracks the compiler, so without a pin
  the comparison is red on day one and red again on every Dependabot bump —
  ~21 MB of forced rebuild commits per bump, for no signal about the code.
  — **Reversibility:** reversible — the fallback (D-05) is a CI-file change.

  > **D-02a — CORRECTION FROM RESEARCH, and the precondition for all of D-02.**
  > `-buildvcs=false` **must** be part of every build invocation. All six
  > committed modules carry `vcs.revision`, `vcs.time` and `vcs.modified=true`,
  > stamped by the default `-buildvcs=auto`. The revision is the git SHA at
  > build time, so a CI rebuild happens at a different commit *by construction*
  > and can never match the committed bytes. Measured in `06-RESEARCH.md`: the
  > same source at two commits gave two hashes; with `-buildvcs=false` it gave
  > one. **No toolchain pin rescues this.** Every place the build command is
  > written down carries the flag: `tools/wasm`, `plugins/README.md`,
  > `internal/plugin/testdata/README.md`, and the `//go:generate` line at
  > `internal/plugin/runtime_test.go:13`.

- **D-03:** The pin lives **inside the build tool**, as `GOTOOLCHAIN` with an
  exact patch level — not in CI's `setup-go` and not as a `toolchain` line in
  `go.mod`. A contributor running the tool locally must produce the same bytes
  as the runner, otherwise the fragility has only moved from `go.mod` to CI. It
  also solves `echo.wasm`, which lives in the root module and therefore cannot
  carry its own `toolchain` line. The guest compiler and the host binary's
  compiler are deliberately decoupled — they share no reason to match.

  > **D-03a — CONSTRAINT FROM RESEARCH.** `GOTOOLCHAIN` cannot be set *lower*
  > than the root `go.mod`'s `go` directive (currently `go 1.26.6`), because
  > `echo` lives in the root module. The pin therefore has a floor, and the
  > discussion's premise that the committed set is uniformly `go1.24.7` is
  > wrong: four files are, `echo.wasm` is `go1.26.2`, and
  > `plugins/kontaktformular/plugin.wasm` is `go1.26.4`. Five of six also still
  > stamp the pre-rename import path `github.com/holzcloud/cms/sdk`. D-04's
  > one-time rebuild resolves all of this in a single stroke.
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
- **D-23:** The four committed `.zip` archives beside the plugins (3.7 MB, each
  holding a byte-identical copy of the `plugin.wasm` next to it) are **packed by
  `tools/wasm` too**, deterministically via `archive/zip` with fixed timestamps
  so the archive is byte-comparable like the module. Raised by research: D-04's
  rebuild would desynchronise them, recreating MAINT-03's own defect one layer
  up — self-inflicted, which is why it is in scope rather than deferred. No Go
  code reads them, so nothing else changes.

### D-05 resolved by research: RELIABLE WITH CONDITIONS

`06-RESEARCH.md` settles the cross-platform question. Location, build-cache
contents and git state were measured to have no effect **once `-buildvcs=false`
is set** (D-02a). The host axis itself (darwin/arm64 vs. linux/amd64) is cited
to go.dev/blog/rebuild — `CGO_ENABLED=0 go build -trimpath` is host-independent
since Go 1.21 — but was **not** empirically falsified; the research container
never got past its registry pull, and the researcher reported no observation
rather than asserting an absence. The plan must therefore carry the one-CI-run
falsification gate the research prescribes (`-print-hashes` on `ubuntu-latest`)
before the comparison is treated as load-bearing, with the D-05 fallback ("build
fresh in CI and run the plugin tests against the fresh build") as the documented
answer if it fails.

### i18n format lock and self-description (MAINT-01, MAINT-02)

- **D-13:** `writeCatalog` goes through the standard library as
  `json.NewEncoder` with `SetEscapeHTML(false)` over the whole map, then
  `json.Indent(dst, src, "", "")`. **Measured during discussion — this
  reproduces today's format byte for byte:** flush-left, HTML unescaped, keys
  sorted byte-wise (Go's encoder sorts map keys exactly as `sort.Strings` does).
  `quote()` disappears entirely; it existed only to dodge `MarshalIndent`'s
  escaping. Both roadmap warnings were re-confirmed by measurement:
  `json.MarshalIndent` escapes HTML to `<` with no switch, and
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
  looks: the doc comment at the head of `tools/i18n/main.go` and the
  per-regional-file report line at `main.go:113`, which crosses the screen on
  every run.

  > **D-17a — CORRECTION FROM RESEARCH.** The indentation claim MAINT-02 asks to
  > remove is at **`tools/i18n/main.go:288`** (the `writeCatalog` doc comment,
  > "writes the file sorted and indented"), not in the package doc at `:1–15`,
  > which makes no such claim. The roadmap's own pointer at `main.go:287` lands
  > inside `quote()`. D-01 corrects that pointer along with the rest.

### Stale notes (MAINT-05)

- **D-18:** All **seven** codebase maps are corrected surgically, not just
  `ARCHITECTURE.md`. Research widened this further still: `ARCHITECTURE.md` has
  six countable-fact drifts **plus sixteen drifted line references**, and
  `INTEGRATIONS.md` still documents a `k8s/` directory, a `deploy.yml` workflow
  and an arm64 cross-compile job that no longer exist. `06-RESEARCH.md` carries
  the complete line-referenced correction list; a plan task executes it
  mechanically. MAINT-05 understates the problem: every map carries
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

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition
- `.planning/ROADMAP.md` §"Phase 6: Aufräumen" — goal, six success criteria,
  the ordering constraint, the CI-budget note and the `encoding/json` traps.
  Note D-01: two of its notes are corrected as the phase's first task.
- `.planning/REQUIREMENTS.md` — MAINT-01…05 verbatim (lines 113–117) and the
  traceability table. Also corrected by D-01.
- `.planning/WINDOWS.md` — entry 1, waived: why the tool's flush-left format is
  canonical and the two-space indent in the four large catalogues was drift.

### i18n
- `tools/i18n/main.go` — `:1–16` doc comment (claims indented output, MAINT-02);
  `:99–114` the regional-file branch and its rationale; `:113` the report line;
  `:213–255` `swissSpelling` and `writeSwiss`; `:277–287` `quote()`;
  `:288–307` `writeCatalog`.
- `internal/i18n/locales/` — seven catalogues; `de-CH`, `fr-CH`, `it-CH` are
  deviation lists, the other four are full translations.
- `.github/workflows/ci.yml:47–50` — the `go mod tidy` + `git diff --exit-code`
  pattern the i18n step is modelled on (D-15).

### Plugins and wasm
- `plugins/README.md` §"Ein Plugin bauen" — the canonical build invocation.
- `internal/plugin/testdata/README.md` — why `echo.wasm` is committed, and its
  build command.
- `internal/plugin/runtime_test.go:13` — an existing `//go:generate` line that
  already builds `echo.wasm` with the right flags. `go generate ./...` does not
  descend into `plugins/*` (separate modules), so it cannot be the whole answer.
- The five self-skipping tests: `internal/plugin/sdk_e2e_test.go:23`,
  `internal/plugin/hofladen_e2e_test.go:25`, `internal/plugin/runtime_test.go:26`,
  `internal/public/formular_e2e_test.go:160`, `internal/public/suche_e2e_test.go:28`.
- `.github/workflows/ci.yml`, `security.yml:36–47`, `release.yml:40` — the three
  workflows that run tests. `security.yml`'s comment carries the 297 s
  measurement that the CI budget must respect.

### Stale notes
- `docs/offene-punkte.md` — `:87–105` the finished Dependabot section, `:138–152`
  "Beim Weiterarbeiten", `:140` the `00044`/`00045` claim. The file stays the
  project's source of truth for what is missing.
- `.planning/codebase/*.md` — all seven maps, all dated 2026-08-22.
- `.planning/quick/260902-cml-…/deferred-items.md`,
  `.planning/quick/260903-ceo-…/deferred-items.md`,
  `.planning/quick/260903-da5-…/deferred-items.md` — all three verified closed.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`tools/i18n` and `tools/mkbundle`** — the established shape for this repo's
  own tooling: a Go command under `tools/`, run as `go run ./tools/<name>`.
  `tools/wasm` follows it (D-06).
- **`ci.yml:47–50`** — `go mod tidy` followed by `git diff --exit-code` is
  already the repo's idiom for "the generated artefact is in step with the
  source". The i18n step is its twin (D-15).
- **`internal/plugin/runtime_test.go:13`** — a working `//go:generate` line with
  the exact build flags for a wasip1 guest. Reuse the invocation, not the
  mechanism.
- **`quote()` in `tools/i18n/main.go:277`** — already uses `json.Encoder` with
  `SetEscapeHTML(false)`. The stdlib route is a generalisation of what the file
  does per string, not a new dependency.

### Established Patterns
- **Nothing is deleted, only reported.** The i18n tool never removes an orphaned
  key ("a sentence often comes back one commit later"). D-20 applies the same
  instinct to `deferred-items.md`.
- **A format change is its own commit.** Roadmap PITFALLS #25; applied to the
  `.wasm` rebuild in D-04.
- **Plugins are separate Go modules** (`plugins/*/go.mod`, `go 1.24`, with a
  `replace` to `../../sdk`). `go build ./...` from the root does not reach them
  and `go generate ./...` does not either. `echo` is the exception — it lives in
  the root module.
- **CI actions are pinned to a commit, not a tag**, with the version in a
  comment and Dependabot bumping both. Any new action follows this.

### Integration Points
- `tools/wasm` → six module directories, five under `plugins/*` plus
  `internal/plugin/testdata/echo`.
- `ci.yml` `test` job → a `tools/wasm -check` step before `go test ./...`, and an
  i18n step modelled on `go mod tidy`.
- `ci.yml`, `security.yml`, `release.yml` → workflow-level
  `HOLZCLOUD_TEST_REQUIRE_WASM=1`.
- Five test files → their `t.Skipf` calls become failures, guarded by that
  variable, naming `go run ./tools/wasm`.

</code_context>

<specifics>
## Specific Ideas

- **Measured during discussion, not assumed.** Two rebuilds of
  `plugins/jahreszahl` with the same Go produced identical hashes
  (`2508d6f5…`); the committed file (`b3c00c92…`, 3 097 425 B, `go1.24.7`
  embedded) differs from a `go1.26.4` rebuild (3 312 864 B). Total committed
  wasm in the repo: 20.7 MB across six files.
- **The three stdlib JSON routes were run, not recalled.**
  `Encoder(SetEscapeHTML=false)` + `json.Indent("","")` reproduces today's
  format exactly; `MarshalIndent` emits `<b>`; `SetIndent("","")`
  emits a single line.
- The scratchpad probe for the JSON routes is at
  `/private/tmp/claude-501/-Users-holz-Projects-holzcloud-cms/2f12cd57-770f-4d7f-b8a9-29d503defd7d/scratchpad/jsontest/`
  — reproduce it rather than trusting the numbers above if anything looks off.

</specifics>

<deferred>
## Deferred Ideas

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

</deferred>

---

*Phase: 6-Aufräumen*
*Context gathered: 2026-09-04*
