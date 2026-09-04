---
phase: 06-aufr-umen
plan: 07
subsystem: testing
tags: [wasm, testing, ci, github-actions, env-var, documentation, i18n, goose, migrations]

requires:
  - phase: 06-04
    provides: "the stale-note pass over everything that predated this phase, and the four anchors it deliberately reserved because Phase 6 was about to change what they describe"
  - phase: 06-06
    provides: "the rebuild of all ten artifacts and `go run ./tools/wasm -check` as a blocking CI step ahead of `go test ./...` — the precondition without which promoting the skips hands a contributor a red suite they cannot fix (D-12)"
  - phase: 06-03
    provides: "`writeCatalog` through `encoding/json`, the deletion of the indentation claim and of `quote()`, and the tool's own statement about which regional catalogues it writes — the facts that let the ROADMAP note be retired rather than re-pointed"
  - phase: 06-05
    provides: "`tools/wasm` complete, and the measured note that `go run` collapses the child exit code so CI must treat non-zero as the signal"
provides:
  - "`internal/plugin/wasmtest` — one non-test package deciding skip-or-fail for five tests across three Go packages, with the build command in a single package-level constant"
  - "`HOLZCLOUD_TEST_REQUIRE_WASM: \"1\"` at workflow level in `ci.yml`, `security.yml` and `release.yml`; absent from `image.yml`, which runs no tests"
  - "The four documentation anchors this phase created or invalidated, each checked against the tree at the moment it was written"
  - "The Phase 6 i18n planning note retired — both sub-clauses stamped closed, every line pointer into `tools/i18n/main.go` removed"
  - "The standing gate recorded with its evidence, including the half that did not happen"
  - "A migration finding raised while setting the gate up and then REFUTED by measurement: all three released tags carry the `00008` column, the earliest is 2026-09-03, and the one affected database applied `00008` on 2026-04-25 from a pre-release tree — no released installation can reach the failing state"
affects: [phase-07, ci, plugins, testing]

actuals:
  tokens: 4774
  tasks: 4
  commits: 4

tech-stack:
  added: []
  patterns:
    - "A non-test helper package under `internal/` for a helper three test packages must share — the first of its kind in this repository, shaped after `net/http/httptest`"
    - "One environment variable, compared against emptiness rather than a literal, separates a forgiving development machine from a strict runner"
    - "A planning note whose subject a phase has just reshaped is retired with its finding stamped closed, not re-pointed at a fresh line number"
    - "A gate records what did not happen with its reason, and compensating evidence is labelled compensating"

key-files:
  created:
    - internal/plugin/wasmtest/wasmtest.go
  modified:
    - internal/plugin/runtime_test.go
    - internal/plugin/sdk_e2e_test.go
    - internal/plugin/hofladen_e2e_test.go
    - internal/public/formular_e2e_test.go
    - internal/public/suche_e2e_test.go
    - .github/workflows/ci.yml
    - .github/workflows/security.yml
    - .github/workflows/release.yml
    - .planning/codebase/TESTING.md
    - .planning/codebase/STRUCTURE.md
    - docs/offene-punkte.md
    - .planning/ROADMAP.md

key-decisions:
  - "The failing branch was proven by reproducing the false pass first: with the module moved aside and HOLZCLOUD_TEST_REQUIRE_WASM=1, the suite printed `ok … 0.449s` and exited 0 BEFORE the change. A promotion whose old behaviour was never observed is a claim, not a measurement."
  - "Exit codes were measured without a pipe. The plan's own `<verify>` writes `echo \"exit=$?\"` after `| tail`, which reports the exit status of `tail`, not of `go test` — the same class of defect as the `go run` collapse 06-05 recorded. Real values: unset 0, `=1` 1, `=true` 1, `=yes` 1."
  - "`echoModul` became a one-line wrapper exactly as the plan and 06-RESEARCH specify, which means it no longer calls `t.Helper()` itself. Consequence named rather than hidden: for that one indirect call site the message is attributed to the wrapper line instead of the calling test. The four direct sites are unaffected and the `--- SKIP:` / `--- FAIL:` line names the test either way."
  - "STRUCTURE.md received a second one-line edit beyond the `tools/` inventory line — the committed-artifact line under Special Directories now names `go run ./tools/wasm`. Without it the plan's own `<verify>` command `grep -c 'tools/wasm' STRUCTURE.md` cannot pass, because the tree line names the helper without its path."
  - "The four visibly-rendering guests were NOT driven through a browser, and no database rows were written by hand to manufacture a pass. Writing plugin rows directly would have faked exactly the code path the gate exists to observe."
  - "The migration finding raised during gate setup was escalated as an upgrade-path defect and then refuted: `git show <tag>:…/00008…` shows all three released tags carry the column, and the earliest tag (2026-09-03) postdates by four months the date the one affected database applied that migration (2026-04-25). It is a local development artifact, needs no plan, and the escalation is left visible in the body rather than deleted, so the reasoning error stays legible."

patterns-established:
  - "Prove the RED of a promotion by reproducing the false pass, not by asserting that one existed"
  - "Measure an exit code without a pipe in between, or you are measuring the last command in the pipeline"
  - "A gate that could not run half of itself says so in the same breath as the half that passed"

requirements-completed: [MAINT-04, MAINT-05]

coverage:
  - id: D1
    description: "One shared helper decides skip-or-fail for five tests across three Go packages, both of its branches executed, and the failure names the command that fixes it"
    requirement: MAINT-04
    verification:
      - kind: integration
        ref: "BEFORE the change, module moved aside, HOLZCLOUD_TEST_REQUIRE_WASM=1 => `ok … 0.449s`, exit 0 — the false pass reproduced"
        status: pass
      - kind: integration
        ref: "AFTER, module moved aside: unset => --- SKIP naming path + `go run ./tools/wasm`, exit 0; =1 => --- FAIL naming path + error + `go run ./tools/wasm`, exit 1; =true => exit 1; =yes => exit 1 (all measured unpiped)"
        status: pass
      - kind: other
        ref: "grep -c t.Skipf over a .wasm in the five test files => 0; grep -c os.ReadFile over a .wasm => 0; five of five import wasmtest; `go list -f '{{.Imports}}' ./internal/plugin/wasmtest` => `os testing` exactly"
        status: pass
      - kind: integration
        ref: "go build ./... && go vet ./... && gofmt -l . => exit 0, no output; go test ./internal/plugin/ ./internal/public/ => ok; file restored, go test ./... exit 0, go run ./tools/wasm -check exit 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every workflow that runs a test declares HOLZCLOUD_TEST_REQUIRE_WASM at workflow level; the one that runs none declares nothing"
    requirement: MAINT-04
    verification:
      - kind: other
        ref: "yaml.safe_load over all four workflows: d['env']['HOLZCLOUD_TEST_REQUIRE_WASM'] == '1' (str) in ci/security/release; absent from image.yml; absent from every job-level and step-level env in all three"
        status: pass
      - kind: other
        ref: "grep -c HOLZCLOUD_TEST_REQUIRE_WASM => 1/1/1/0 (ci/security/release/image); grep -c 'uses:' => 3/2/2/5, identical before and after; a four-line German comment sits immediately above the variable in each of the three"
        status: pass
      - kind: integration
        ref: "HOLZCLOUD_TEST_REQUIRE_WASM=1 go test ./... => exit 0, 40 packages ok, no FAIL and no SKIP — the suite is green under the value CI will set"
        status: pass
    human_judgment: false
  - id: D3
    description: "The three anchors this phase created and the one it invalidated describe the finished tree, and the ROADMAP note is retired rather than re-pointed"
    requirement: MAINT-05
    verification:
      - kind: other
        ref: "TESTING.md step order vs yaml.safe_load(ci.yml) step names: formatting < tidy < vet < `tools/wasm -check` < catalogues < build < test — matches; grep -c HOLZCLOUD_TEST_REQUIRE_WASM TESTING.md => 2"
        status: pass
      - kind: other
        ref: "ls -d tools/*/ | wc -l => 3; the tools/ inventory line names i18n, mkbundle and wasm; grep -c 'tools/wasm' STRUCTURE.md => 1"
        status: pass
      - kind: other
        ref: "awk over `## Beim Weiterarbeiten` in docs/offene-punkte.md: `tools/wasm` => 2, `GOTOOLCHAIN` => 1, `go.mod` => 2, `eigenen Commit` => 1"
        status: pass
      - kind: other
        ref: "precondition: grep -c 'sorted and indented' tools/i18n/main.go => 0; inside ### Phase 6: grep -c 'main\\.go:[0-9]' => 0, grep -c 'tools/wasm' => 5, no line asserting the doc comment claims indented output; git diff -U0 ROADMAP.md => one hunk at line 135, Phase 7 starts at 145, so Phases 7–10 untouched"
        status: pass
      - kind: other
        ref: "grep -r '2026-08-22' .planning/codebase/ => no match; plan 06-04's date stamps survived"
        status: pass
    human_judgment: false
  - id: D4
    description: "Standing gate, translation half: the catalogue report is a measured fact rather than an assumption"
    requirement: QUAL-01
    verification:
      - kind: integration
        ref: "go run ./tools/i18n, 2026-09-04 => en/es/fr/it each `1128 übersetzt, 0 offen, 0 verwaist`; de-CH 51 Abweichungen 0 ohne Gegenstück; fr-CH 4; it-CH 9 — quoted verbatim below"
        status: pass
      - kind: integration
        ref: "at gate time: go test ./... => exit 0, 40 packages ok, 0 `--- SKIP`; go run ./tools/wasm -check => exit 0"
        status: pass
    human_judgment: false
  - id: D5
    description: "Standing gate, browser half: the rebuilt guest modules seen working in the running application"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "binary built CGO_ENABLED=0 -trimpath, started on an empty data directory, migrations 0 → 45 clean; /admin/login served, real sign-in, compulsory TOTP enrolment completed, /admin/ overview rendered — screenshots captured"
        status: pass
      - kind: automated_ui
        ref: "the four visibly-rendering guests (suche, kontaktformular, jahreszahl, bestellung) on a public page — NOT PERFORMED, see `## The half of the gate that did not happen`"
        status: unknown
      - kind: e2e
        ref: "COMPENSATING ONLY, not a substitute: go test ./internal/plugin/ ./internal/public/ -v => 0 skips; TestHofladenLaeuftDurch, TestKontaktformularPluginNimmtNachrichtenAn, TestEigenesFormularVonEndeZuEnde, TestFormulareditorUeberstehtDenFilterDesHosts, TestSuchePluginBeantwortetSuche, TestOhneSuchePluginKeineSuche all PASS through the real wazero host"
        status: pass
    human_judgment: true
    rationale: "Half of this deliverable was not verified. The application was seen to start, authenticate, enforce its second factor and render admin screens; the four rebuilt guests were never seen on a public page, because a fresh database has no plugins and the available browser tooling could not perform the .zip upload that installs one. The suite proves the guests execute and produce their expected output — which is exactly the check docs/offene-punkte.md warns is not sufficient on its own. A human must decide whether this carries into Phase 7 as an open item or blocks here."

duration: 18 min
completed: 2026-09-04
status: complete
---

# Phase 6 Plan 07: The Promotion, the Documents and the Standing Gate Summary

**The five tests that used to skip themselves now fail on a runner and stay forgiving on a development machine, decided in one place by one environment variable that three workflows set — and the phase's closing gate is recorded with both halves, including the browser half that could not be run.**

## Performance

- **Duration:** 18 min
- **Started:** 2026-09-04T11:40:00Z
- **Completed:** 2026-09-04T11:58:00Z
- **Tasks:** 4 (three executed here, the standing gate run by the orchestrator)
- **Files modified:** 13 (1 created, 12 modified)

## Accomplishments

- **The false pass was reproduced before it was removed.** With `plugins/suche/plugin.wasm`
  moved aside and `HOLZCLOUD_TEST_REQUIRE_WASM=1` already set, the suite printed
  `ok  github.com/holzcloud/holzcloud-cms/internal/public  0.449s` and exited **0**.
  That is precisely the green-that-checks-nothing MAINT-04 exists to remove, observed
  rather than assumed. After the change the same command exits **1**.
- **One function decides for five tests in three packages.**
  `internal/plugin/wasmtest` is a non-test package importing exactly `os` and
  `testing` (`go list -f '{{.Imports}}'` → `os testing`). It has to be a non-test
  package: the five call sites live in `package plugin`, `package plugin_test` and
  `package public`, and a helper declared in a test file under one of them cannot be
  seen by the other two. Duplicating it would have put D-11's message in two places,
  and MAINT-04 is a requirement about one consistent message.
- **The build command is a package-level constant**, `bauhinweis`, so the skipping
  and the failing branch cannot drift apart. Both carry it on its own line.
- **`true` and `yes` behave like `1`.** The comparison is against emptiness, not
  against the literal value — a contributor reproducing a runner failure locally
  reaches for `true`, and a strict comparison would have silently kept skipping,
  which is the same hole one level down (T-06-22).
- **Three workflows set the variable, one deliberately does not.** `ci.yml`,
  `security.yml` and `release.yml` carry it as a workflow-level `env:` beside
  `CGO_ENABLED`. `image.yml` runs no tests and is untouched — asserted by a
  criterion so nobody "fixes" it later. `security.yml` is in even though it runs on
  a schedule: anything on a runner is CI, and it is the one place the race detector
  runs, measured at 297 s for `./internal/plugin/` alone.
- **`uses:` counts are unchanged**: 3 / 2 / 2 / 5 before and after. No action, no
  step, no per-step override was added.
- **The four documentation anchors read true against the finished tree**, each with
  its checking command recorded below.
- **The i18n planning note is retired, not re-pointed.** `06-03` deleted the
  indentation claim and the `quote()` helper above it, so the note's sub-clause (a)
  asserted something that no longer exists and pointed at a line that had moved. A
  fresh line number into a file this phase just reshaped would only start drifting
  again — which is the same defect class MAINT-05 exists to remove, committed by the
  phase that removes it. Both sub-clauses are stamped closed and every
  `main.go:<line>` pointer inside `### Phase 6` is gone.
- **The translation half of the gate is green and now says something.** Beyond the
  count, the three regional lines carry `06-03`'s self-description
  (`wird von -schweiz erzeugt` / `nur gelesen, von Hand gepflegt`), so **D-17 is
  satisfied by observation**: MAINT-02's plain statement is visible in the report a
  person actually runs, not only in the source.

## The standing gate (QUAL-01, QUAL-02)

### Translation half — green

`go run ./tools/i18n`, verbatim, 2026-09-04:

```
1128 Zeichenketten im Quelltext
de-CH.json   51 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1128 übersetzt, 0 offen, 0 verwaist
es.json      1128 übersetzt, 0 offen, 0 verwaist
fr-CH.json   4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1128 übersetzt, 0 offen, 0 verwaist
it-CH.json   9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
```

All four full catalogues report `0 offen, 0 verwaist`. This phase adds no
user-visible string, so a green line was expected; it was run anyway, because that
is what makes the number mean something when Phases 7 through 10 begin adding
strings.

Two checks at the moment the gate was recorded: `go test ./...` exit 0, 40 packages
`ok`, **0 `--- SKIP`**; `go run ./tools/wasm -check` exit 0.

### Browser half — what was exercised

Binary built with `CGO_ENABLED=0 go build -trimpath -o <scratch>/holzcloud
./cmd/holzcloud`, started with `HOLZCLOUD_DATA_DIR=<scratch>/gate/data
HOLZCLOUD_PORT=51775` against an empty directory. **Migrations applied cleanly
0 → 45 at startup** — which incidentally confirms at runtime the migration number
this phase corrected in four codebase maps.

| # | Page | Outcome |
|---|---|---|
| 1 | `/admin/login` | Sign-in page served. On an empty install `/admin` 303s to `/admin/setup`; after creating an account with `holzcloud user create` the login page renders normally. Screenshot captured. |
| 2 | sign-in | Completed with a throwaway account (`gate@example.test`). |
| 3 | TOTP enrolment | Compulsory for administrators. QR page rendered, code accepted, recovery-codes page rendered. Screenshot captured. |
| 4 | `/admin/` overview | Renders with counters at 0/0/0 and the empty-state prompt. Screenshot captured. |

So the application built from this phase's tree starts, serves, authenticates,
enforces its second factor and renders admin screens.

### The half of the gate that did not happen

**The four visibly-rendering guests — `suche`, `kontaktformular`, `jahreszahl` and
`bestellung` — were not exercised in a browser.** Stated plainly, because a gate
that quietly rounds a partial pass up to a pass is worth less than no gate.

Two reasons, both real:

1. A fresh database has no plugins. Installing one means uploading a `.zip` through
   the admin — exactly the "content that does not exist in a fresh database" case
   the plan's own action text anticipates.
2. The browser tooling available in the session could not perform that upload. The
   Playwright MCP browser was already held by another process and refused a second
   instance; the fallback in-app browser pane has no file-upload capability at all.
   The "New website" form could not be submitted through it either — no website was
   created, confirmed against the scratch database
   (`select count(*) from websites` → 0).

No plugin rows were written into the database by hand to work around this. That
would have manufactured a pass for the exact code path the gate exists to observe.

**Compensating evidence — recorded as compensating, not as a substitute:**

```
--- PASS: TestHofladenLaeuftDurch (3.21s)
--- PASS: TestKontaktformularPluginNimmtNachrichtenAn (1.14s)
--- PASS: TestEigenesFormularVonEndeZuEnde (1.14s)
--- PASS: TestFormulareditorUeberstehtDenFilterDesHosts (1.14s)
--- PASS: TestSuchePluginBeantwortetSuche (0.80s)
--- PASS: TestOhneSuchePluginKeineSuche (0.07s)
```

`go test ./internal/plugin/ ./internal/public/ -v` reports **0 skips**. These
exercise the rebuilt `.wasm` artifacts through the real wazero host, so the
recompiled guests demonstrably execute and produce their expected output — but
through the suite, which is precisely the check `docs/offene-punkte.md` warns is not
sufficient on its own. The honest statement: **the guests are proven to run; they
have not been seen on a public page.**

This carries forward as an open item for whoever picks up Phase 7, which has real UI
surface and will need a working browser path regardless.

## A finding that was escalated and then refuted: migration 00036

Found while setting the gate up, escalated as an upgrade-path defect, and then
**disproved by measurement.** Outside MAINT-01…05 either way.

`holzcloud user create` was run once **without** `HOLZCLOUD_DATA_DIR` set. It opened
the repository's own gitignored development database at `data/holzcloud.sqlite`,
applied migrations 18–35 to it, and failed at 36:

```
goose up: partial migration error (type:sql,version:36): SQL logic error: no such column: deleted_at (1)
```

The accident is disclosed: that database now sits at 35, wedged at 36. Its content is
intact (2 websites, 3 pages, 1 user, 1 menu — throwaway test data),
`pragma integrity_check` returns `ok`, and there is no backup.

The failure itself is **pre-existing and worth more than the accident**:

| Evidence | Observation |
|---|---|
| `internal/db/migrations/00036_content_types.sql:60-62` | creates a partial index ending `WHERE deleted_at IS NULL` |
| `internal/db/migrations/00008_revisions_locking_trash.sql:15` | is where the column is added |
| a **fresh** database migrated 0 → 45 | has `pages.deleted_at`, and 36 applies cleanly — verified against the scratch instance during this gate |
| the development database | does **not** have the column, although `goose_db_version` records version 8 as applied on 2026-04-25 |
| grep for `CREATE TABLE pages_new`, `ALTER TABLE pages RENAME`, `CREATE TABLE new_pages` | nothing — no migration rebuilds the `pages` table |

**Hypothesis at the time of writing:** migration `00008` was edited after it had
already been applied somewhere, so an installation that applied the pre-edit version
permanently lacks the column and cannot upgrade past 36 — a live upgrade-path defect
for a self-hosted CMS with real deployments.

### That hypothesis was tested afterwards and is REFUTED. No released version is affected.

Checked against the tree and the tags, not reasoned about:

| Evidence | Observation |
|---|---|
| `git tag` | `v1.4`, `v1.5`, `v1.6` — there are no others |
| `git show <tag>:internal/db/migrations/00008_revisions_locking_trash.sql` | **all three tags carry the `ADD COLUMN deleted_at` line** |
| earliest tag date (`git log -1 --format=%ci v1.4`) | **2026-09-03** |
| when the development database applied migration 8 | **2026-04-25** — four months earlier |
| repository history | squashed into `0e6d7af`; the `archiv` remote that held the pre-squash lineage returns 404, so git cannot show when the line was added |

So the development database applied migration `00008` as it stood in April, from a
tree still under development, months before any release existed. Editing a migration
in place before it has ever shipped is ordinary practice, not a defect. **Every
released version has always carried the column**, so no installation created from a
release can reach the state that fails.

**Scope of the real problem: exactly one database — this repository's own gitignored
`data/holzcloud.sqlite`**, which grew alongside development. It is a local artifact,
not an upgrade-path defect, and it needs no separate plan.

The escalation above was written before that check. It is left standing rather than
deleted because a summary that silently swaps a wrong claim for a right one teaches
nobody where the reasoning went wrong: the fresh-install path being green was
correctly identified as a blind spot, but the conclusion drawn from one anomalous
database ran ahead of the evidence. The cheap check — *do the released tags carry the
line?* — was available the whole time and settles it in one command.

**What survives as worth doing:** a test that exercises upgrade-from-an-old-snapshot
rather than only fresh-install would still be worth having. Nothing observed here
demonstrates a defect it would catch, so it is a hardening idea, not a fix — and it
belongs in the backlog, not in a remediation plan.

## Task Commits

1. **Task 1: One shared helper decides skip-or-fail for all five call sites** — `41ce89e` (feat)
2. **Task 2: `HOLZCLOUD_TEST_REQUIRE_WASM` in the three workflows that run tests** — `3491a2b` (ci)
3. **Task 3: The documentation loop on every fact this phase created or invalidated** — `b445db8` (docs)
4. **Task 4: Standing gate** — records evidence, changes no file; no commit of its own

## Files Created/Modified

- `internal/plugin/wasmtest/wasmtest.go` — **new.** The one place that decides what a
  missing guest module means. German doc comment; `Modul(t, pfad)`; the build
  instruction as a package-level constant.
- `internal/plugin/runtime_test.go` — `echoModul` is a one-line wrapper; the `os`
  import fell away with it
- `internal/plugin/sdk_e2e_test.go`, `internal/plugin/hofladen_e2e_test.go`,
  `internal/public/formular_e2e_test.go`, `internal/public/suche_e2e_test.go` —
  four-line read-and-skip block replaced by one call each; variable names unchanged,
  neighbouring manifest reads untouched
- `.github/workflows/ci.yml`, `security.yml`, `release.yml` — the variable at
  workflow level with a German comment
- `.planning/codebase/TESTING.md` — the CI line now lists the `test` job step for
  step in running order and names the variable; the security line names it too; the
  run-commands block gained `go run ./tools/wasm` and `-check`
- `.planning/codebase/STRUCTURE.md` — the `tools/` inventory line names all three
  helpers; the committed-artifact line names the tool that builds them
- `docs/offene-punkte.md` — the toolchain-pin standing instruction in
  `## Beim Weiterarbeiten`
- `.planning/ROADMAP.md` — the i18n planning note retired

## The four anchors, with the command that checks each

| Claim written | Command | Result |
|---|---|---|
| TESTING.md names the variable | `grep -c HOLZCLOUD_TEST_REQUIRE_WASM .planning/codebase/TESTING.md` | 2 |
| its step order matches `ci.yml` | `yaml.safe_load(ci.yml)` step names in order | formatting < tidy < vet < wasm-check < catalogues < build < test — matches the prose |
| three helpers under `tools/` | `ls -d tools/*/ \| wc -l` | 3 |
| STRUCTURE.md names the tool | `grep -c 'tools/wasm' .planning/codebase/STRUCTURE.md` | 1 |
| the standing instruction landed in the standing-instructions section | `awk '/^## Beim Weiterarbeiten/{f=1} f' docs/offene-punkte.md \| grep -c 'tools/wasm'` | 2 |
| …and names the pin and the floor | same awk, `grep -c GOTOOLCHAIN` / `grep -c 'go\.mod'` | 1 / 2 |
| …and the own-commit rule | same awk, `grep -c 'eigenen Commit'` | 1 |
| the indentation claim is gone from the source | `grep -c 'sorted and indented' tools/i18n/main.go` | **0** (the precondition for retiring rather than correcting) |
| no line pointer survives in the note | `sed -n '/### Phase 6/,/### Phase 7/p' ROADMAP.md \| grep -c 'main\.go:[0-9]'` | **0** |
| 06-01's wasm notes were not collateral damage | same sed, `grep -c 'tools/wasm'` | 5 |
| Phases 7–10 untouched | `git diff -U0 -- .planning/ROADMAP.md \| grep '^@@'` | one hunk, line 135; `### Phase 7` begins at 145 |
| 06-04's date stamps survived | `grep -r '2026-08-22' .planning/codebase/` | no match |

## Decisions Made

1. **The RED was a reproduction, not an assertion.** The plan asked for both branches
   to be executed after the change. The false pass *before* the change was measured
   as well — `ok … 0.449s`, exit 0 with the variable already set — because "a helper
   whose failing branch was never executed is not a promotion" cuts both ways: a
   promotion whose old behaviour was never observed is a claim.
2. **Exit codes were measured unpiped.** The plan's `<verify>` block writes
   `echo "exit=$?"` after `| tail -20`, which reports `tail`'s status and would have
   printed `exit=0` for a failing test run. This is the same class of defect `06-05`
   recorded for `go run` collapsing a child's exit code, one layer out. The four real
   values are unset → 0, `=1` → 1, `=true` → 1, `=yes` → 1.
3. **`echoModul` lost its own `t.Helper()`.** The plan and `06-RESEARCH.md` both
   prescribe a one-line wrapper and that is what was written. The consequence is
   named rather than hidden: for that single indirect call site the skip or failure
   is attributed to the wrapper's line rather than the calling test. The four direct
   sites are unaffected — the recorded message points at `suche_e2e_test.go:27` — and
   `--- SKIP:` / `--- FAIL:` names the test in every case, so nothing is lost from
   the output. Worth a deliberate line rather than a silent divergence.
4. **STRUCTURE.md got a second, one-line edit.** The plan scopes the file to its
   `tools/` inventory line, but its own `<verify>` requires
   `grep -c 'tools/wasm' STRUCTURE.md` to be non-zero, and the tree line names the
   helper without its path. The committed-artifact line under `## Special
   Directories` now reads "built and verified by `go run ./tools/wasm` (`-check` in
   CI)" — the same fact this phase created, in the file's own register.
5. **The German register was continued.** `06-06` decided the two new `ci.yml`
   comments would be German against the file's English majority. The three comments
   added here follow that, which the plan's acceptance criterion demands explicitly.
   The register split in `ci.yml` noted by `06-06` therefore persists and remains a
   whole-file question for a later deliberate pass.
6. **No database rows were hand-written to complete the browser half.** See the gate
   section. A manufactured pass on the one code path the gate exists to observe is
   worse than a recorded gap.
7. **The migration defect is recorded, not fixed.** It is outside MAINT-01…05 and a
   fix without a test that migrates from an old snapshot would close the symptom and
   leave the class open.

## Deviations from Plan

### Auto-fixed and corrections

**1. [Rule 3 - Blocking] The `tools/wasm` mention STRUCTURE.md's verify command requires**
- **Found during:** Task 3
- **Issue:** The plan scopes STRUCTURE.md to its `tools/` inventory line, but that
  line names the helpers without paths, so the task's own
  `<automated>grep -c 'tools/wasm' STRUCTURE.md</automated>` would have reported 0
  and failed the task.
- **Fix:** One additional line — the committed-artifact entry under
  `## Special Directories` now names `go run ./tools/wasm`.
- **Files modified:** `.planning/codebase/STRUCTURE.md`
- **Verification:** `grep -c 'tools/wasm' .planning/codebase/STRUCTURE.md` → 1
- **Commit:** `b445db8`

### Plan-internal inconsistencies observed (not acted on beyond reporting)

**2. Task 3's `<verify>` omits `.planning/ROADMAP.md` from its allowed file list.**
Its `git diff --name-only HEAD` check fails when any path outside `TESTING.md`,
`STRUCTURE.md` and `docs/offene-punkte.md` appears, yet ROADMAP.md is in the same
task's `<files>`, in three of its `<acceptance_criteria>` and in the plan
frontmatter. Verified against the four-file list the criteria name, and the ROADMAP
diff separately confirmed to be a single hunk inside `### Phase 6`.

**3. Task 1's `<verify>` reads `$?` through a pipe.** See decision 2. Both branches
were measured correctly instead.

**Total deviations:** 1 auto-fixed (Rule 3), 2 plan-internal inconsistencies reported.
**Impact:** none on scope or behaviour. The auto-fix is one documentation line that
the plan's own verification command required.

## Verification Results

| Check | Result |
|---|---|
| `go build ./... && go vet ./...` | exit 0 |
| `gofmt -l .` | no output |
| `go test ./...` | exit 0, 40 packages ok |
| `HOLZCLOUD_TEST_REQUIRE_WASM=1 go test ./...` | exit 0, no FAIL, **no SKIP** |
| `go run ./tools/wasm -check` | exit 0 |
| `go run ./tools/i18n` | four full catalogues `0 offen, 0 verwaist` |
| module aside, variable unset | `--- SKIP`, message names path + `go run ./tools/wasm`, exit 0 |
| module aside, `=1` / `=true` / `=yes` | `--- FAIL`, message names path + error + `go run ./tools/wasm`, exit 1 / 1 / 1 |
| `t.Skipf` over a `.wasm` in the five files | 0 |
| `os.ReadFile` over a `.wasm` in the five files | 0 |
| direct imports of `internal/plugin/wasmtest` | `os`, `testing` — exactly two |
| workflow `env:` parse (all four files) | `ok` — `'1'` at top level in three, absent from `image.yml`, absent from every job and step |
| `grep -c 'uses:'` per workflow | 3 / 2 / 2 / 5, unchanged |
| `grep -c 'sorted and indented' tools/i18n/main.go` | 0 |
| `main\.go:[0-9]` inside `### Phase 6` | 0 |
| `tools/wasm` inside `### Phase 6` | 5 |
| ROADMAP diff scope | one hunk at line 135; Phases 7–10 untouched |
| `grep -r '2026-08-22' .planning/codebase/` | no match |
| deletions across the three commits | none (`git diff --diff-filter=D HEAD~3 HEAD` empty) |

## Requirements

- **MAINT-04 → Complete.** Declared by this plan alone. The five tests fail loudly on
  a runner, stay forgiving locally, share one message naming the build command, and
  were promoted after the rebuild landed (`06-06` first, D-12 satisfied by
  construction).
- **MAINT-05 → Complete.** Declared by `06-01`, `06-04` and `06-07`; all three now
  have a SUMMARY. `06-04` corrected everything that predated the phase; this plan
  corrected the three anchors the phase created and the one it invalidated.

## Issues Encountered

- **The browser half of the standing gate could not be completed.** See
  `## The half of the gate that did not happen`. Not a code defect; a tooling
  limitation, recorded rather than papered over.
- **A development database was wedged at migration 35.** Disclosed above. Throwaway
  content, `integrity_check` ok, no backup. The underlying defect is pre-existing and
  is written up as its own finding.

## Known Stubs

None in code. One **unrun verification**: the browser pass over the four
visibly-rendering guests, tracked above and carried to Phase 7.

## Deferred / Out of Scope

- **The browser pass over `suche`, `kontaktformular`, `jahreszahl` and
  `bestellung`** — needs a working file-upload-capable browser path. Phase 7 has real
  UI surface and will need one anyway.
- **The migration `00036` upgrade-path defect** — its own plan, with a test that
  migrates from an old snapshot rather than from zero.
- **The register split in `ci.yml`** (English steps, now five German comment lines
  across three files) — carried from `06-06`. A whole-file decision, made
  deliberately in one commit, if wanted at all.
- **Archive bytes still depend on `compress/flate`** in the toolchain running
  `tools/wasm`, not on the pinned guest compiler — carried from `06-05`/`06-06`. The
  new standing instruction in `docs/offene-punkte.md` covers the raise-the-pin half;
  the flate half remains the documented residual risk.
- **`go build ./tools/wasm` still drops an un-ignored native `./wasm` binary in the
  repository root**, as `./i18n` and `./mkbundle` do. Pre-existing, unrelated.

## User Setup Required

None.

## Next Phase Readiness

Phase 6 is complete: 7 of 7 plans executed. Three things to carry into Phase 7:

1. **CI is now strict about guest modules.** Any workflow that runs a test sets
   `HOLZCLOUD_TEST_REQUIRE_WASM=1`, so a missing `.wasm` fails the run. Locally the
   suite still skips, and `go run ./tools/wasm` is the one command that fixes it —
   named in the failure message itself.
2. **The browser half of the closing gate is unfinished for Phase 6 and will be
   needed properly in Phase 7.** Establish a working, upload-capable browser path
   before the phase's own gate, not during it.
3. **The migration `00036` finding is open and unowned.** It is an upgrade-path
   defect on a self-hosted product, invisible to a fresh-install test suite. It
   should be scheduled rather than left in this SUMMARY.

## Self-Check: PASSED

`internal/plugin/wasmtest/wasmtest.go` verified present on disk; all twelve modified
files verified present. Commits `41ce89e`, `3491a2b` and `b445db8` verified in
`git log`. Plan-level verification re-run at close-out: `go build`/`go vet`/`gofmt`
clean, `go test ./...` exit 0, `HOLZCLOUD_TEST_REQUIRE_WASM=1 go test ./...` exit 0
with no skips, `go run ./tools/wasm -check` exit 0, all four workflow files parse
with the variable at top level in three and absent from the fourth.

---
*Phase: 06-aufr-umen*
*Completed: 2026-09-04*
