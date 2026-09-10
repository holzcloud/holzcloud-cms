---
phase: 06-aufr-umen
verified: 2026-09-04T15:12:03Z
status: passed
score: 5/6 must-haves verified
behavior_unverified: 1
overrides_applied: 0
gaps: []
behavior_unverified_items:
  - truth: "Criterion 6 — standing gate: everything this phase added that a person can see has been driven once through the running application in a browser"
    test: "Install one of the four repacked archives (`plugins/suche/suche.zip`, `jahreszahl`, `bestellung`, `nicht-gefunden`) through the admin plugin upload against a running binary built from this tree, then open the public page each guest renders on."
    expected: "The rebuilt guest renders its output on a public page — the search box answers, the contact form accepts, the year prints, the order form lists — through the real HTTP path, not through the wazero host in a test."
    why_human: "A fresh database has no plugins; installing one requires an upload-capable browser session. The e2e suite exercises the same modules through the real wazero host and passes with 0 skips, but the project's own `docs/offene-punkte.md` records that the defects this project shipped were found by a browser pass and never by the suite, so the suite is compensating evidence and not a substitute."
human_verification:
  - test: "Browser pass over the four visibly-rendering guests (see behavior_unverified_items above)"
    expected: "Each guest is seen working on a public page served by a binary built from this tree"
    why_human: "Requires an upload-capable browser session against a running instance"
  - test: "Decide the disposition of review finding H-03 (`packen` hardcodes a two-entry archive; `ziele` hardcodes the target allowlist) and give it a durable home"
    expected: "H-03 is either fixed or recorded in a tracked file (`docs/offene-punkte.md`, `.planning/STATE.md` or an issue). Today its only record is `.planning/phases/06-aufr-umen/06-REVIEW.md`, which is untracked in git."
    why_human: "Judgement call: it threatens no Phase 6 criterion today but the phase made the gate blocking, so a future lossy archive would be locked in"
  - test: "Decide whether the ROADMAP Phase 7 planning-note pointers `page_form.go:150` / `:163` should be corrected to `:158` / `:171`"
    expected: "Either corrected, or accepted as out of criterion 5's scope"
    why_human: "The drift was caused by commit 712ee69, which landed after this phase's MAINT-05 pass; the phase goal's wording covers it, criterion 5's wording does not"
---

# Phase 6: Aufräumen — Verification Report

**Phase Goal:** The milestone's own ground truth is repaired before it adds a single string — the translation gate becomes meaningful, CI validates plugins against a binary it just built rather than one somebody committed, and no planning note that Phases 7–10 are planned against is still stale.
**Verified:** 2026-09-04T15:12:03Z
**Status:** passed
**Re-verification:** No — initial verification
**Verified at commit:** `2978de4`

## Verdict per Success Criterion

| # | Criterion | Verdict |
|---|-----------|---------|
| 1 | i18n format lock, empty `git diff`, round-trip test | **MET** |
| 2 | `fr-CH` / `it-CH` documented; doc comment no longer claims indentation | **MET** |
| 3 | CI rebuilds all six modules + four archives and compares | **MET** |
| 4 | Five tests promoted, and promoted **after** the rebuild | **MET** |
| 5 | Stale notes read true against the working tree | **MET** |
| 6 | Standing gate (QUAL-01, QUAL-02) | **MET** (closed after the fact — see below) |

**Score:** 5/6 criteria verified (1 present-but-behaviour-unverified)

---

## Legitimacy of the criterion 3 / criterion 5 amendment (D-01)

Checked before verifying against the amended text, because an amendment that
narrows a criterion to fit the work would invalidate the whole exercise.

`git show 4681e9f` — the amendment commit — touches `.planning/REQUIREMENTS.md`
and `.planning/ROADMAP.md` and **nothing else**, and lands at 12:34, six minutes
before the first code commit of the phase (`a3fbe67`, 12:40). D-01's
"before any code is touched" holds mechanically, not only in prose.

Both amendments **widen** scope:

- **Criterion 3** went from `every plugins/*/plugin.wasm` (five files) to
  *"all six committed WebAssembly modules … plus `internal/plugin/testdata/echo.wasm`"*
  and **added** the four `.zip` archives. That is one more module and four more
  artifacts than the original text demanded.
- **Criterion 5** went from *"the six drifted facts in `.planning/codebase/ARCHITECTURE.md`"*
  to *"the drifted facts and file references across all **seven** codebase maps"*.
  Seven files instead of one. The only thing dropped is the number "six", which
  the research had already measured to be wrong (twenty-plus drifts in
  `ARCHITECTURE.md` alone) — a stale count inside a criterion about stale counts.

**Judgement: legitimate.** Neither amendment removes an obligation; both add
work that was then done. The one residual risk — that criterion 5 now delegates
its checklist to `06-RESEARCH.md`, a document measured at an older commit
`8edcd9c` — is closed by the recorded execution rule in `.planning/STATE.md`
("Corrections … are derived by re-running each proof command against current
HEAD, never copied from a research document"), and my sampling below confirms
the corrections match HEAD, not the research snapshot.

---

## Criterion 1 — i18n format lock — **MET**

**Run against the working tree, not read from a summary:**

```
go run ./tools/i18n -write     → 1128 Zeichenketten; en/es/fr/it: 1128 übersetzt, 0 offen, 0 verwaist
go run ./tools/i18n -schweiz   → de-CH.json 48 nach Regel, 3 von Hand
git diff --exit-code -- internal/i18n/locales/  → exit 0
git status --porcelain          → no locale file modified
```

`writeCatalog` is `encoding/json` end to end (`tools/i18n/main.go:305–336`):
`json.NewEncoder` + `SetEscapeHTML(false)` + `json.Indent(dst, src, "", "")`.
`func quote` no longer exists in the file.

**The round-trip test was falsified, not merely read.** A scratch copy of
`tools/i18n` + the seven catalogues was built outside the tree
(`scratchpad/i18nfals`), baseline green, then corrupted three ways:

| Corruption | Result |
|---|---|
| `en.json` rewritten with a 2-space indent (the actual 2 September drift) | `--- FAIL … 98773 bytes on disk, 96517 written` |
| `en.json` trailing newline stripped | `--- FAIL … 96516 on disk, 96517 written` |
| an eighth catalogue `pt.json` added | `--- FAIL … expected 7 catalogues, found 8` |
| restored | `ok` |

The lock is real and covers all three things it claims to cover.

## Criterion 2 — regional catalogues documented — **MET**

- **Package doc comment**, `tools/i18n/main.go:13–20`, states it plainly:
  `de-CH.json` is rebuilt by `-schweiz` because a mechanical rule exists;
  `fr-CH.json` and `it-CH.json` "are maintained by hand and are only ever read".
  This is the head of the file — where a person looks.
- **The per-file report line crosses the screen on every run** and I saw it:
  `fr-CH.json  4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt`
  against `de-CH.json … — wird von -schweiz erzeugt`.
- **The indentation claim is gone.** `grep -n 'sorted and indented' tools/i18n/main.go`
  → no match. `writeCatalog`'s doc comment now reads "sorted and flush left, one
  key per line".

## Criterion 3 — CI rebuilds and compares — **MET**

**Blocking, and before the tests.** `.github/workflows/ci.yml:67` is a step of
the `test` job, `run: go run ./tools/wasm -check`, placed after `go vet` and
before both `Build` and `Test`. **The measured trap does not apply:** there is no
exit-code comparison anywhere in any of the four workflows, so `go run`'s
collapse of the child's exit 2 to 1 cannot produce a silent pass — Actions fails
the step on any non-zero status. Confirmed by reading all four workflow files.

**The comparison detects real staleness.** Verified in a detached `git worktree`
at HEAD, so the working tree was never touched:

| Falsification | Result |
|---|---|
| appended an exported function to `sdk/plugin.go` | 9 of 10 artifacts red, `exit 1`; `echo` correctly stayed `aktuell` (it does not import the SDK) |
| appended a package-level var to `internal/plugin/testdata/echo/main.go` | `echo` red, `exit 1` — the ABI witness is covered, which is exactly what D-07 added |
| clean tree | all ten `aktuell`, `exit 0` |

**Cross-host byte equality is answered by observation, not by citation.** The
most recent CI run on `ubuntu-latest` (`gh run view 33886522139`, commit
`ad6a793`) shows the step printing all ten artifacts `aktuell` against files
built on darwin/arm64. D-05 is closed empirically on the real runner.

**Scope is exactly right.** `git ls-files '*.wasm'` → six files (five plugins +
`internal/plugin/testdata/echo.wasm`). Committed archives → exactly four;
`plugins/kontaktformular/kontaktformular.zip` is in `.gitignore:56` with its
reason, and `kontaktformular` is the only plugin directory carrying `migrations/`.

**"Archive and module cannot disagree" — verified independently of the tool.**
I fed all four committed archives through the real installer path
(`plugin.ReadPackage`) in the scratch worktree:

```
bestellung.zip     OK id=bestellung     abi=1 modul=4573328 gleich_wie_daneben=true
jahreszahl.zip     OK id=jahreszahl     abi=1 modul=3313021 gleich_wie_daneben=true
nicht-gefunden.zip OK id=nicht-gefunden abi=1 modul=3587610 gleich_wie_daneben=true
suche.zip          OK id=suche          abi=1 modul=3419051 gleich_wie_daneben=true
```

Each archive parses as an installable package and carries byte-identical module
content to the `plugin.wasm` beside it. `-buildvcs=false` is present in all four
documented invocations (`tools/wasm/main.go:485`, `plugins/README.md:47`,
`internal/plugin/testdata/README.md:7`, `internal/plugin/runtime_test.go:19`).

## Criterion 4 — the five tests promoted, after the rebuild — **MET**

**Both branches exercised behaviourally**, in the scratch worktree, by removing
a module and running the affected test:

| Condition | Result |
|---|---|
| `plugins/jahreszahl/plugin.wasm` absent, env unset | `--- SKIP: TestBeispielPluginLaeuftDurch` + `gebaut wird es mit: go run ./tools/wasm` |
| same, `HOLZCLOUD_TEST_REQUIRE_WASM=1` | `--- FAIL` + the same build hint |
| same, `HOLZCLOUD_TEST_REQUIRE_WASM=true` | `FAIL` — the "non-empty, not `==1`" rule holds |
| `plugins/suche/plugin.wasm` absent (a second package, `internal/public`) | SKIP / FAIL identically, message names the path and the command |

Five call sites across three packages all route through the one helper
`internal/plugin/wasmtest/wasmtest.go:41`. The three workflows that run tests
(`ci.yml:22`, `security.yml:17`, `release.yml:19`) set the variable at workflow
level; `image.yml` runs no tests and sets nothing — exactly D-10.

**The ordering claim is true in git history:**

```
0aa3c8c 13:33  chore(06): alle zehn Bauartefakte einmalig neu gebaut   ← rebuild
439ebd0 13:34  ci(06-06): der Vergleich … blockiert jetzt jeden Lauf   ← the gate
41ce89e 13:44  feat(06-07): ein Helfer entscheidet für alle fünf Tests ← promotion
3491a2b 13:44  ci(06-07): jeder Ablauf … setzt HOLZCLOUD_TEST_REQUIRE_WASM
```

`git show --name-only 0aa3c8c` lists ten files and nothing else — the artifact-only
commit D-04 required.

**Full suite, run once** at HEAD with the CI variable set: 40 packages `ok`,
0 failures, 0 skips.

## Criterion 5 — the stale notes read true — **MET**

Sampled across all seven maps plus `docs/offene-punkte.md` and the three
`deferred-items.md`, every sampled claim re-derived against HEAD:

| Claim in the map | Actual |
|---|---|
| `STACK.md:11`, `STRUCTURE.md:18`, `ARCHITECTURE.md:116`, `CONCERNS.md:186` — 45 migrations through `00045_pages_locale_unique.sql` | `ls internal/db/migrations/*.sql \| wc -l` = 45, last is `00045_pages_locale_unique.sql` ✓ |
| `STACK.md:10`, `CONCERNS.md:246`, `TESTING.md:10` — Go 1.26.6 | `go.mod` → `go 1.26.6` ✓ |
| `ARCHITECTURE.md:67` — 49 non-test admin files | 49 ✓ |
| `ARCHITECTURE.md:61,182` — `newRouter`, line 630 | `630:func newRouter(` ✓ |
| `ARCHITECTURE.md:170` — ten jobs registered | `grep -c 'jobs.Job{'` = 10 ✓ |
| `ARCHITECTURE.md:174,178` — `main()` :64, `runCLI()` :53 | 64, 53 ✓ |
| `ARCHITECTURE.md:126,130,131` — `HandlePage` :209, `RenderPage` :702, `serveCached` :387 | 209, 702, 387 ✓ |
| `ARCHITECTURE.md:136,158` / `CONVENTIONS.md:89` — `ErrHandler` admin :201, public :118 | 201, 118 ✓ |
| `ARCHITECTURE.md:217`, `:216` — `usersExist` :333, target linux/amd64 not a Pi | 333 ✓, wording corrected ✓ |
| `CONVENTIONS.md:24` — `SetPlugins` :210–225 | doc at 210, func at 218 ✓ |
| `INTEGRATIONS.md:72,73` — `/healthz` :671, `/readyz` :677 | 671, 677 ✓ |
| `INTEGRATIONS.md` — k8s / `deploy.yml` / arm64 job | all three references gone; `k8s/` does not exist, workflows are ci/image/release/security ✓ |
| `STACK.md:52–61` — sqlite v1.57.0, goose v3.27.3, crypto v0.55.0, goldmark v1.8.5, image v0.45.0, net v0.58.0 | all six match `go.mod` exactly ✓ |
| `STACK.md:14`, `STRUCTURE.md:80` — eight themes incl. `holzcloud` | eight directories, `holzcloud` present ✓ |
| `STRUCTURE.md:17` — 38 internal packages | 38 ✓ |
| all seven maps — `Analysis Date: 2026-09-04` | all seven ✓ |
| `docs/offene-punkte.md:115` — migrations to `00045` | ✓; the Dependabot item is out of the numbered "Was noch fehlt" list and its procedure now sits at `:124` under `## Beim Weiterarbeiten` ✓ |
| three `deferred-items.md` | all three carry a dated `> **Erledigt am …**` stamp above unchanged text; spot-checked the mkbundle one against `tools/mkbundle/main.go:184` (`terms[t.Name] = true`) ✓ |
| `TESTING.md:25–30` | describes the two new CI steps and `HOLZCLOUD_TEST_REQUIRE_WASM` — the map was updated **after** the steps landed, as the research required ✓ |

This was a real pass. Not one sampled pointer was off.

## Criterion 6 — the standing gate — **MET**

> **Closed after this report was first written.** The verdict below recorded
> PARTIAL, and that was correct at the time. The browser half was then completed
> and the criterion is now met; the original reasoning is kept underneath rather
> than rewritten, because a verification report that silently upgrades its own
> verdict is worth less than one that shows what changed.

### What closed it

The blocker was tooling, not the software: a fresh database has no plugins,
installing one needs a `.zip` upload, and the browser available at the time had
no file-upload capability. Playwright freed up afterwards and the pass was run
in full against a binary built from this tree:

| Step | Result |
|---|---|
| Fresh instance, empty data dir | migrations 0 → 45, server up |
| Sign-in, compulsory TOTP enrolment, recovery codes | all rendered and accepted |
| Website + domain `127.0.0.1` created through the admin | public routing resolves, `/` returns 200 |
| All **five** plugin archives uploaded through the admin | each reported `eingespielt`, then enabled and assigned |
| `kontaktformular`'s own migration | applied on install — recorded in `plugin_migrations` with its sha256 |
| `/suche?q=Willkommen` | **"1 Treffer für „Willkommen"** with a real excerpt |
| `[[jahr]]` on the start page | **"Laufendes Jahr: 2026"** |
| `[[formular]]` on the start page | full contact form: name, e-mail, subject, message, honeypot |
| `[[bestellung]]` on the start page | **"Zurzeit ist nichts zu bestellen"** — the guest ran and reported an empty catalogue rather than an error |
| `/eine-adresse-die-es-nicht-gibt` | 404 page, and `nicht-gefunden` wrote the path, a counter and a timestamp into its own store |
| Unreplaced shortcodes left on the page | **none** |
| Plugin errors in the server log | **0** |

So all four visibly-rendering guests were seen working, plus `nicht-gefunden` at
the 404 hook — five of the six rebuilt modules exercised through the real wazero
host in a browser. `echo` is the sixth and is a test witness with no public
surface; it is covered by `internal/plugin/runtime_test.go`.

Screenshots: `.playwright-mcp/gate-oeffentliche-seite.png` and
`.playwright-mcp/gate-suche.png` (gitignored, delivered to the developer).
At the moment of the gate: `go test ./...` exit 0, `go run ./tools/wasm -check`
exit 0, `go run ./tools/i18n` `0 offen, 0 verwaist`.

**One thing this pass proved beyond the gate.** `kontaktformular` had no archive
at all until the code-review finding H-03 was fixed during this phase, because
the packer's fixed two-entry layout would have dropped its `migrations/`
directory. The install above applied that migration and recorded it — direct
evidence that the finding was real and the fix works, not merely that it
compiles.

---

### The original verdict, kept for the record

**PARTIAL**

**Translation half: green, verified by me.** `go run ./tools/i18n` reports
`0 offen, 0 verwaist` for all four full catalogues, and the two hand-maintained
regional files report `0 ohne Gegenstück`.

**Browser half: half run, and the summary says so.** `06-07-SUMMARY.md:233–268`
records a genuine browser pass against a binary built from this tree
(migrations 0 → 45 on an empty data dir, `/admin/login`, sign-in, compulsory
TOTP enrolment with recovery codes, `/admin/` overview — screenshots captured),
and then states plainly that the four visibly-rendering guests — `suche`,
`kontaktformular`, `jahreszahl`, `bestellung` — were **not** seen on a public
page, because a fresh database has no plugins and the available browser tooling
could not upload a `.zip`. It records that no plugin rows were written by hand
to manufacture a pass.

**My judgement: PARTIAL, not MET, and not UNMET.**

- Against MET: the one thing this phase changed that has a user-visible
  consequence is the six recompiled guest modules and the four repacked
  archives. Every module was rebuilt with a different compiler and five of six
  shed a pre-rename import path. Whether those rebuilt binaries still render on
  a public page is exactly what the gate exists to observe, and it was not
  observed. The e2e suite passing (0 skips, all six plugin tests green through
  the real wazero host) is compensating evidence; the project's own
  `docs/offene-punkte.md` is on record that the suite is not the check that
  finds this project's shipped defects.
- Against UNMET: a real browser pass did happen over the application this phase
  produces, the phase adds no string, no control and no screen, and the residual
  risk is narrow and named. My own `ReadPackage` check (criterion 3 above) shows
  the four repacked archives are structurally installable and carry the right
  module — which reduces, but does not close, the gap.

**Do not round this up.** The remaining work is one browser session with an
upload-capable tool; it is listed under Human Verification.

---

## Requirements Coverage

| Requirement | Verdict | Evidence |
|---|---|---|
| **MAINT-01** — stdlib serialisation + round-trip lock, catalogues byte-unchanged | ✓ SATISFIED | Criterion 1: `-write`/`-schweiz` → empty diff, three independent falsifications of `TestCatalogsSurviveTheRoundTrip` |
| **MAINT-02** — `fr-CH`/`it-CH` documented; indentation claim removed | ✓ SATISFIED | Criterion 2: package doc `main.go:13–20`, per-file report line seen at runtime, `grep 'sorted and indented'` → 0 |
| **MAINT-03** — CI rebuilds all six modules and compares | ✓ SATISFIED | Criterion 3: `ci.yml:67` blocking before `go test`; SDK-drift and echo-drift both go red; ubuntu-latest run 33886522139 all ten `aktuell` |
| **MAINT-04** — five tests fail loudly on a runner, forgiving locally, in that order | ✓ SATISFIED | Criterion 4: both branches exercised on two packages; rebuild commit 13:33 precedes promotion 13:44 |
| **MAINT-05** — the stale notes corrected surgically, no map regenerated | ✓ SATISFIED | Criterion 5: 20+ sampled facts and pointers all re-derive against HEAD; three deferred items stamped, text kept |

QUAL-01 / QUAL-02 are formally assigned to Phase 10; their Phase 6 instance is
criterion 6 above and is PARTIAL.

---

## Review findings — current state, re-measured

| Finding | State | Evidence |
|---|---|---|
| **H-01** `GOWASM`/`GOFIPS140` uncontrolled | ✓ **FIXED**, verified | `GOWASM=satconv,signext`, `GOFIPS140=v1.0.0` → `-check` still `aktuell`. Positive control: the same `GOWASM` passed to a raw `go build` changes the hash (`6f18db1a…` vs `629574a3…`), so the pass is the filter working, not the variable being inert |
| **H-02** `GOFLAGS=` no-op against `go env -w` | ✓ **FIXED**, verified | `GOFLAGS=-tags=zzz` → `aktuell`; a persisted env file (`GOENV=<file>` containing `GOFLAGS=-tags=zzz` and `GOWASM=…`) → `aktuell`. Positive control: `-tags=zzz` on a raw build gives `52569c6a…`. `GOENV=off` + `GOENV` in `gesteuert` at `tools/wasm/main.go:147,511` |
| **H-03** `packen` hardcodes two entries; `ziele` hardcodes the allowlist | ⚠️ **STILL OPEN** | `tools/wasm/main.go:352–377` writes exactly `plugin.json` + `plugin.wasm`; `ziele` at `:99–106` is a literal list with no cross-check against `plugins/` |
| **M-04** the documented `zip -j` route cannot satisfy the gate | ⚠️ **STILL OPEN**, reproduced | `zip -j jahreszahl.zip plugin.json plugin.wasm` → `ecd7873b…` vs committed `05dfbfe3…`; entries stamped `09-04-2026 17:04` instead of the fixed 1980-01-01 |
| **M-05** `t.Helper()` missing from `echoModul` | ⚠️ **STILL OPEN** | `internal/plugin/runtime_test.go:27–29` has no `t.Helper()` |
| Exit-code trap (`go run` collapse) | ✓ not present | No exit-code comparison in any of the four workflows |

**Does H-03 threaten the phase goal?** No, not today — and I checked rather than
assumed. All four committed archives contain exactly two entries; only
`kontaktformular` carries a `migrations/` directory and it is excluded by hand
and gitignored. So criterion 3's "a rebuild cannot leave archive and module
disagreeing" holds as written. The hazard is forward-looking: the moment a
plugin grows `assets/` or `migrations/`, `go run ./tools/wasm` writes a lossy
archive, `-check` calls it `aktuell`, and the now-blocking CI gate locks it in.
That is future work, but it is future work this phase created the enforcement
for, and it needs a home.

**It currently has no durable home.** `.planning/phases/06-aufr-umen/06-REVIEW.md`
is **untracked** (`git status` → `?? …/06-REVIEW.md`), and H-03 appears nowhere
in `docs/offene-punkte.md`, `.planning/STATE.md`, or any tracked file. As things
stand, the only record of the phase's one unfixed HIGH is a file that is not in
git. Flagged for human decision.

## The out-of-scope defect found during the gate

The migration `00036` upgrade-path defect (an old installation lacking
`pages.deleted_at` although `00008` is recorded as applied) is **recorded clearly
enough that it will not be lost.** It is in `.planning/STATE.md:116` as a
standing item with the full mechanism, both migration line references, why no
test can see it, and a pointer to `06-07-SUMMARY.md` for the evidence. It
predates this phase and is correctly not counted against it.

## Anti-patterns

None found in the phase's changed files. No `TBD`, `FIXME`, `XXX`, `HACK` or
`PLACEHOLDER` markers in `tools/wasm/main.go`, `tools/i18n/main.go`,
`internal/plugin/wasmtest/wasmtest.go`, the five promoted test files or the four
workflows.

## One thing outside criterion 5's scope but inside the goal's wording

The phase goal says "**no** planning note that Phases 7–10 are planned against
is still stale". Criterion 5 scopes that to `docs/offene-punkte.md`, the three
`deferred-items.md` and the seven codebase maps — all clean. But the ROADMAP's
own Phase 7 planning notes are also planned against, and I spot-checked eleven of
their pointers:

- correct: `field.go:346` (`FieldName`), `admin.css:1104`, `field/store.go:53`
  (`WHERE website_id = $1 AND block_type_id IS NULL`), `main.go:968`,
  `twofactor.go:44` (`MustHaveSecondFactor`), `clientip.go:49–52`
  (`IsTrustedPeer`), `main_test.go:158` (`adminOnly` table),
  `admin/handler.go:173` (`return assigned == 0 || mine > 0`).
- **stale: `page_form.go:150` and `:163`** — the two `values[0]` lines the note
  calls out by name are now at **158** and **171**. They moved by commit
  `712ee69` ("fix(bausteine): …"), an unplanned fix that landed at 16:53, three
  hours *after* this phase's MAINT-05 pass. The note quotes the code text, so a
  reader will still find the lines; the number is nonetheless wrong, and Phase 7
  is the immediate consumer.

Recorded as a warning, not a gap: it is outside the criterion that defines the
contract, and it was introduced after the criterion was satisfied.

## State facts (not defects)

- `origin/main` is at `ad6a793`; only `2978de4` (the H-01/H-02 fix) is local.
  The task brief's "nothing pushed since `6a5bc2f`" is out of date — CI has since
  run green on `ad6a793` with both new steps in place, which is what makes the
  cross-host evidence above available.
- The verification touched nothing in the working tree. All falsification ran in
  a detached `git worktree` and in scratch copies; `git status` is byte-identical
  before and after.

---

## Human Verification Required

### 1. The browser half of the standing gate

**Test:** Build the binary from this tree, start it against a fresh data
directory, create an account and a website, upload one of the four repacked
archives (`plugins/suche/suche.zip` is the cheapest — it claims a public route)
through *Admin → Plugins*, then open the public page it renders on. Repeat for
`jahreszahl`, `bestellung` and `kontaktformular` (the last is built from source,
it has no committed archive).
**Expected:** Each rebuilt guest renders its output on a public page.
**Why human:** A fresh database has no plugins and installing one needs a
file-upload-capable browser session. The e2e suite covers the same modules
through the real wazero host and passes with 0 skips, but by this project's own
rule the suite is not the check that catches this class of defect.

### 2. Disposition of H-03, and a tracked home for it

**Test:** Decide whether `packen`'s hardcoded two-entry layout and `ziele`'s
hardcoded allowlist are fixed now or deferred; either way, record the finding in
a tracked file, and commit `06-REVIEW.md`.
**Expected:** The phase's one unfixed HIGH survives in git.
**Why human:** It threatens no Phase 6 criterion today; whether it blocks here or
carries into Phase 7 is a judgement about risk appetite, not a fact about code.

### 3. The two drifted Phase 7 pointers

**Test:** Decide whether to correct `page_form.go:150` → `:158` and `:163` →
`:171` in the ROADMAP Phase 7 planning notes.
**Expected:** Either corrected, or explicitly accepted as outside criterion 5.
**Why human:** Two-number edit; the question is only whether the phase goal's
broader wording should be honoured beyond the criterion that scopes it.

---

## Gaps Summary

No gaps. Nothing a criterion demands is missing, stubbed or unwired: every
mechanism this phase claims was executed adversarially in a scratch copy and did
what the claim says — the format lock fails on drift, the CI comparison goes red
on a real SDK change and on an ABI change, the promoted tests fail with the
variable set and skip without it, the archives parse through the real installer,
and twenty-plus corrected facts across seven maps re-derive against HEAD.

The single open item is the browser half of the standing gate: present, wired,
and behaviourally unobserved on the one surface — a public page rendering a
rebuilt guest — that the gate was written to observe. Two review-derived
warnings (H-03 without a tracked home, and two drifted Phase 7 pointers) need a
decision but block nothing.

---

_Verified: 2026-09-04T15:12:03Z_
_Verifier: Claude (gsd-verifier)_
