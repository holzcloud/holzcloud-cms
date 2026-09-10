---
phase: 06-aufr-umen
plan: 04
subsystem: docs
tags: [planning-maps, documentation-drift, maint-05, codebase-map, deferred-items]

requires:
  - phase: 06-aufr-umen (plan 06-01)
    provides: the ground-truth amendment that widened MAINT-05 from one map to all seven
provides:
  - Seven `.planning/codebase/` maps whose countable facts and file references reproduce against the working tree
  - A `docs/offene-punkte.md` whose numbered list holds only open work, with the Dependabot upgrade procedure kept as a standing instruction
  - Three `deferred-items.md` that read as closed at a glance, with their reasoning intact word for word
affects: [06-07, phase-07, phase-08, phase-09, phase-10, plan-phase, map-codebase]

actuals:
  tokens: 10200
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Surgical map correction: every changed value is derived by re-running its proof command against current HEAD, never copied from a research document measured at an older commit"
    - "Closure by stamp, not deletion: a resolved deferred finding gets a dated blockquote above its text; the text stays for the phase that needs the reasoning"

key-files:
  created: []
  modified:
    - .planning/codebase/ARCHITECTURE.md
    - .planning/codebase/STACK.md
    - .planning/codebase/STRUCTURE.md
    - .planning/codebase/INTEGRATIONS.md
    - .planning/codebase/CONCERNS.md
    - .planning/codebase/TESTING.md
    - .planning/codebase/CONVENTIONS.md
    - docs/offene-punkte.md
    - .planning/quick/260902-cml-i18n-kataloge-sauber-0-offen-0-verwaist/deferred-items.md
    - .planning/quick/260903-ceo-beispiel-bundle-statt-kundendaten/deferred-items.md
    - .planning/quick/260903-da5-kleinigkeiten-aus-der-freigabepruefung/deferred-items.md

key-decisions:
  - "The tree beat the research inventory in every disagreement; four `k8s/` citations and three workflow-list claims the inventory had not flagged were corrected because the plan's own verification gate is repo-wide over `.planning/codebase/`"
  - "`CONCERNS.md` kept every judgement, including the still-open Go-version-mismatch finding — only its number moved from 1.26.2 to 1.26.6, because `CLAUDE.md:7` still states `Go 1.22+`"
  - "The dangling `docs/offene-punkte.md §7` pointer inside `CONCERNS.md` was repaired because this plan's own task 2 is what invalidated it"

patterns-established:
  - "Proof-beside-value: a corrected number in a planning map is recorded in the SUMMARY next to the command that reproduces it, so D-19's 'provable line by line' is checkable rather than asserted"

requirements-completed: [MAINT-05]

coverage:
  - id: D1
    description: "All seven `.planning/codebase/` maps carry today's Analysis Date and no stale 2026-08-22 stamp"
    requirement: "MAINT-05"
    verification:
      - kind: other
        ref: "grep -rn '2026-08-22' .planning/codebase/ (exit 1, no output)"
        status: pass
    human_judgment: false
  - id: D2
    description: "No map describes infrastructure that was deleted or a target that was retargeted — no `k8s/`, no `deploy.yml`, no Raspberry Pi, no `linux/arm64`"
    requirement: "MAINT-05"
    verification:
      - kind: other
        ref: "grep -rn 'k8s/\\|deploy\\.yml\\|Raspberry Pi\\|linux/arm64' .planning/codebase/ (exit 1, no output)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The migration count and last migration name in ARCHITECTURE, STACK, STRUCTURE and CONCERNS equal what the tree reports"
    requirement: "MAINT-05"
    verification:
      - kind: other
        ref: "ls internal/db/migrations/*.sql | wc -l = 45; tail -1 = 00045_pages_locale_unique.sql; grep -rl '00045_pages_locale_unique' .planning/codebase/ | wc -l = 4"
        status: pass
    human_judgment: false
  - id: D4
    description: "Twenty-seven drifted file-and-line references in ARCHITECTURE, INTEGRATIONS and CONVENTIONS now land on the symbol they name"
    requirement: "MAINT-05"
    verification:
      - kind: other
        ref: "sed -n '<n>p' <file> spot-check for all 27 corrected references — table in ## Proof Tables below"
        status: pass
    human_judgment: false
  - id: D5
    description: "`docs/offene-punkte.md`'s numbered list holds only open work; the Dependabot procedure survives as a standing instruction with its dated measurement intact"
    requirement: "MAINT-05"
    verification:
      - kind: other
        ref: "grep -n '^## [0-9]' docs/offene-punkte.md lists 1-6 only; awk from '## Beim Weiterarbeiten' contains go vet, go test, HOLZCLOUD_DATA_DIR, journal_mode=wal, integrity_check"
        status: pass
    human_judgment: false
  - id: D6
    description: "All three `deferred-items.md` carry exactly one closure stamp and zero deleted lines"
    requirement: "MAINT-05"
    verification:
      - kind: other
        ref: "grep -c '^> \\*\\*Erledigt am ' = 1 per file; git diff --numstat = 2/0 per file"
        status: pass
    human_judgment: false

duration: 27 min
completed: 2026-09-04
status: complete
---

# Phase 6 Plan 04: MAINT-05 — Notes That Read True Summary

**Seven codebase maps, one working list and three deferred findings corrected in place against a re-verified working tree: 45 migrations not 38, Go 1.26.6 not 1.26.2, 27 relocated line references, and every trace of a deleted Kubernetes directory, a deleted deployment workflow and a retired arm64 target removed — with `CONCERNS.md`'s judgements untouched.**

## Performance

- **Duration:** 27 min
- **Started:** 2026-09-04T10:28:00Z
- **Completed:** 2026-09-04T10:55:17Z
- **Tasks:** 3
- **Files modified:** 11 (132 insertions, 117 deletions)

## Accomplishments

- Every countable fact in the seven `.planning/codebase/` maps now reproduces from a command run against `f9ec0ea` — migration count and name, Go version, package inventories, theme list, file counts, six dependency versions, deployment target.
- All 27 drifted file-and-line references were re-derived with `grep -n` against current `HEAD` and then spot-checked one at a time with `sed -n '<n>p'`. Not one number was copied from `06-RESEARCH.md`, which was measured at `8edcd9cb`.
- `INTEGRATIONS.md`, the most drifted map, no longer describes a Kubernetes directory, a `workflow_run`-driven `deploy.yml` with `kubectl rollout status`, or an arm64 cross-compile job. It names what actually publishes: `image.yml` to GHCR and `release.yml` on a `v*` tag.
- `docs/offene-punkte.md` lost its finished Dependabot item from the numbered list — cleanly, since it was the highest-numbered heading — while the whole upgrade procedure, including the fenced database check and the dated 1.57.0 measurement, moved verbatim into `## Beim Weiterarbeiten`.
- The three closed findings read as closed on the first screen without a single word of their reasoning being lost.

## Task Commits

1. **Task 1: Correct the seven codebase maps surgically** — `dde9f4a` (docs)
2. **Task 2: Retire the finished Dependabot item, keep its procedure** — `d447912` (docs)
3. **Task 3: Stamp the three `deferred-items.md` as closed** — `f895b63` (docs)

## Proof Tables

Every changed value with the command that reproduces it. All commands were run against `f9ec0ea` before the value was written.

### `ARCHITECTURE.md`

| Was | Now | Proof |
|-----|-----|-------|
| `Analysis Date: 2026-08-22` (3 places) | `2026-09-04` | — |
| 18-name domain package list | + `activity design money outbox payrexx shop structured totp textdiff wxr tmplspec` | `ls internal` → 38 dirs |
| `newRouter`, line ~572 (2 places) | 630 | `grep -n 'func newRouter' cmd/holzcloud/main.go` |
| Admin handlers (47 files) | 49 non-test files | `ls internal/admin/*.go \| grep -v _test \| wc -l` |
| migrations (38 `.sql`) | 45, through `00045_pages_locale_unique.sql` | `ls internal/db/migrations/*.sql \| wc -l` / `\| tail -1` |
| nine jobs | ten | `grep -c 'jobs.Job{' cmd/holzcloud/main.go` |
| outer chain `main.go:941-945` | `1066-1069` | `grep -n 'RequestID(\|AccessLog(\|Recoverer(\|SecureHeaders'` |
| `HandlePage` `public/handler.go:193` | 209 | `grep -n 'func (h \*Handler) HandlePage\b'` |
| `RenderPage` `loader.go:483` | 702 | `grep -n 'func (l \*Loader) RenderPage'` |
| `serveCached` `public/handler.go:351` | 387 | `grep -n 'func.*serveCached'` |
| admin chain `main.go:882` | 722 | `grep -n 'adminProtectedMux'` |
| `ErrHandler` `admin/handler.go:187` (2 places) | 201 | `grep -n 'func.*ErrHandler' internal/admin/handler.go` |
| `ErrHandler` `public/handler.go:102` | 118 | `grep -n 'func.*ErrHandler' internal/public/handler.go` |
| `cacheKey` `loader.go:394`, `:543` | 613, 770 | `grep -n 'cacheKey\|InvalidateTemplateCache'` |
| sharelink signer `main.go:202` | 223 | `grep -n 'sharelink\.'` |
| markdown contract `markdown.go:23-47` | `76-78` | `sed -n '76,78p' internal/page/markdown.go` |
| published query `public/handler.go:220` | 241 | `grep -rn 'GetPublishedPageIn' internal/public/` |
| "on a Raspberry Pi" | "on a small single linux/amd64 server" | STATE.md; `ci.yml:56-58` |
| `usersExist` `main.go:297` | 333 | `grep -n 'usersExist'` |
| `WithSettings` `main.go:266`, `:904` | 309, `991-993` | `grep -n 'WithSettings\|WithPages\|WithRender\|WithNotify'` |

Six rows the inventory marked correct (`db.go:50`, `resolver.go:48`, `runtime.go` limits, write pool, `variants.go:91`, `branding.go:83`) were left byte-unchanged.

### `STACK.md`

| Was | Now | Proof |
|-----|-----|-------|
| Go 1.26.2 | 1.26.6 | `grep '^go ' go.mod` |
| 38 migrations … `00038_block_types.sql` | 45 … `00045_pages_locale_unique.sql` | `ls internal/db/migrations/*.sql \| wc -l`, `\| tail -1` |
| themes: 7 | 8, `holzcloud` added | `ls cmd/holzcloud/templates/public` |
| Primary target `linux/arm64` (Pi 5) | `linux/amd64` | `.github/workflows/ci.yml:56-58`; `Dockerfile:9` |
| `modernc.org/sqlite` v1.48.2 | v1.57.0 | `grep sqlite go.mod` |
| `pressly/goose/v3` v3.27.0 | v3.27.3 | `grep goose go.mod` |
| `golang.org/x/crypto` v0.54.0 | v0.55.0 | `grep x/crypto go.mod` |
| `yuin/goldmark` v1.8.2 | v1.8.5 | `grep goldmark go.mod` |
| `golang.org/x/image` v0.44.0 | v0.45.0 | `grep x/image go.mod` |
| `golang.org/x/net` v0.57.0 | v0.58.0 | `grep x/net go.mod` |
| `golang:1.26-bookworm` pinned to `$BUILDPLATFORM` | `golang:1.26`, `linux/amd64` | `grep FROM Dockerfile` |
| Kubernetes Secret (`k8s/10-secret.example.yaml`) | removed | `ls` — no `k8s/` at top level |
| `.github/workflows/{ci,deploy,security}.yml` | `{ci,image,release,security}.yml` | `ls .github/workflows/` |
| Production: Pi 5 + Kubernetes PVC | small `linux/amd64` server, or the GHCR image as a single instance | `ls deploy/`; `.github/workflows/image.yml` |

Five rows marked correct (scs v2.9.0, csrf v1.7.3, wazero v1.12.0 + limits, bluemonday v1.0.27, `rsc.io/qr` v0.2.0) were re-checked against `go.mod` and left alone.

### `STRUCTURE.md`

| Was | Now | Proof |
|-----|-----|-------|
| `internal/` (33 dirs) | 38 | `ls internal \| wc -l` |
| 38 goose `.sql` | 45, to `00045_pages_locale_unique` | `ls internal/db/migrations/*.sql \| wc -l` |
| `k8s/` tree entry | `build/` (dev seed + prebuilt example bundles) | `ls -d */`; `ls -R build` |
| package inventory | + `outbox activity textdiff shop money payrexx tmplspec` (38 total) | `ls internal` |
| `admin/` (47 files) | 49 non-test files | `ls internal/admin/*.go \| grep -v _test \| wc -l` |
| every plugin ships a `.zip` | four of five; `kontaktformular` ships `migrations/` | `ls plugins/*/*.zip`; `ls plugins/kontaktformular` |
| `Dockerfile`, `k8s/20-app.yaml`, `deploy/holzcloud.service` | `Dockerfile`, `deploy/holzcloud.service` | `ls` — no `k8s/` |
| themes: 7 | 8, `holzcloud` added | `ls cmd/holzcloud/templates/public` |
| `.github/workflows/ci.yml`, `deploy.yml`, `security.yml` | `ci.yml`, `image.yml`, `release.yml`, `security.yml` | `ls .github/workflows/` |

The `tools/` inventory line is byte-unchanged — plan 06-07 owns it (`git diff` shows no `tools/  ` hunk).

### `INTEGRATIONS.md`

| Was | Now | Proof |
|-----|-----|-------|
| Argon2id "tunable for the Pi" | "tunable for the target server" | retarget 2026-09-03 |
| secrets note citing `k8s/10-secret.example.yaml` | data-directory wording, citation dropped | `ls` — no `k8s/` |
| logs "cluster log pipeline (Kubernetes)" | "whatever collects container stdout" | no manifests in repo |
| `/healthz` `main.go:610` | 671 | `grep -n '/healthz' cmd/holzcloud/main.go` |
| `/readyz` `main.go:616` | 677 | `grep -n '/readyz' cmd/holzcloud/main.go` |
| Hosting: Pi 5; Kubernetes single-replica with three manifests | small `linux/amd64` server, or the image as a single instance; no manifests here | `ls`, `ls deploy/` |
| registry via `deploy.yml` | `image.yml` | `ls .github/workflows/` |
| `ci.yml` "plus a `linux/arm64` cross-compile job" | one job (`test`) on the amd64 runner, which is the deployment target | `grep -n '^  [a-z-]*:' .github/workflows/ci.yml` → one `test:` |
| `deploy.yml` (`workflow_run`, `kubectl rollout status`) | `image.yml` (push/tag/PR/dispatch → GHCR) and `release.yml` (`v*` tag → binary via `gh`) | `sed -n '13,18p' image.yml`; `sed -n '3,5p' release.yml` |
| Secrets: Kubernetes `Secret`, `k8s/README.md`; "On the Pi" | systemd unit lines; orchestration secrets live in the infrastructure repo | `ls deploy/` |

Left alone as the plan required: the `security.yml` description, the backup story, the environment-variable list, the `/ai` MCP endpoint.

### `CONCERNS.md` — three planned corrections plus four dead citations

| Was | Now | Proof |
|-----|-----|-------|
| `refreshed:` / `Analysis Date:` / audit footer `2026-08-22` | `2026-09-04` | — |
| migrations (38 files, through `00038_block_types.sql`) | 45, through `00045_pages_locale_unique.sql` | `ls internal/db/migrations/*.sql` |
| "Go 1.26.2 in `go.mod` vs. Go 1.22+ in `CLAUDE.md`" | "Go 1.26.6 …" — **finding kept open** | `grep '^go ' go.mod` = 1.26.6; `CLAUDE.md:7` still says "Go 1.22+" |
| `k8s/20-app.yaml:49` as throttle mitigation | "the deployment runs a single process" | `ls` — no `k8s/` |
| `k8s/20-app.yaml:49-51` pins `replicas: 1` | "exactly one instance against one data directory (`deploy/holzcloud.service`)" | `ls deploy/` |
| "The comment at `k8s/20-app.yaml:4` already says this" | "It is recorded here because …" | `ls` — no `k8s/` |
| "one PVC (`k8s/20-app.yaml:14-18`)" | "one volume (`HOLZCLOUD_DATA_DIR`)" | `ls` — no `k8s/` |
| "`docs/offene-punkte.md` §7 should be corrected to say eight" | points at the retirement done in this plan's task 2 | `grep -n '^## [0-9]' docs/offene-punkte.md` → 1-6 |

Every judgement, every risk statement and every recommendation is byte-unchanged. No finding was opened, closed or reworded.

### `TESTING.md` and `CONVENTIONS.md`

| File | Was | Now | Proof |
|------|-----|-----|-------|
| TESTING | `Analysis Date` / footer `2026-08-22` | `2026-09-04` | — |
| TESTING | Go 1.26.2 | 1.26.6 | `grep '^go ' go.mod` |
| CONVENTIONS | `Analysis Date` / footer `2026-08-22` | `2026-09-04` | — |
| CONVENTIONS | setters `admin/handler.go:196-211` | `210-225` | `grep -n 'func (h \*Handler) Set' internal/admin/handler.go` → 218/222/225, doc opens at 210 |
| CONVENTIONS | `ErrHandler` `admin/handler.go:187` | 201 | `grep -n 'func.*ErrHandler'` |
| CONVENTIONS | route wiring `main.go:686` | 747 | `grep -n 'GET /admin/websites/{id}/pages"'` |
| CONVENTIONS | rationale comment `admin/handler.go:196-204` | `210-218` | `sed -n '210,218p' internal/admin/handler.go` |

`TESTING.md` lines 25-26 describing `ci.yml` and `security.yml` are byte-unchanged — `git diff` contains no line matching `workflows/`. Plan 06-07 rewrites them after this phase's CI changes land.

### `docs/offene-punkte.md`

| Was | Now | Proof |
|-----|-----|-------|
| `## 7. Dependabot: erledigt …` in the numbered list | gone; list runs 1-6 | `grep -n '^## [0-9]'` — 7 was the highest, so nothing renumbered |
| the procedure inside the numbered list | one `- **Dependabot-Anhebungen:**` bullet under `## Beim Weiterarbeiten` | `awk '/^## Beim Weiterarbeiten/{f=1} f'` contains `go vet`, `go test`, `HOLZCLOUD_DATA_DIR`, `journal_mode=wal`, `integrity_check` |
| "Migrationen laufen bis `00044`" | `00045` | `ls internal/db/migrations/*.sql \| tail -1` |

The dated measurement — `Bei 1.57.0 geprüft: 44 Wanderungen, 48 Tabellen, alles sauber.` — is present byte-for-byte (`grep -cF` = 1). It was re-wrapped onto one line so the sentence stays a contiguous, unindented-by-the-move substring inside the new bullet.

### The three `deferred-items.md`

| File | Stamp names | Closure confirmed by |
|------|-------------|----------------------|
| `260902-cml-…/deferred-items.md` | quick task `260903-bsk`, route 1 taken, waived entry 1 in `.planning/WINDOWS.md` | `sed -n '2p'` on all seven catalogues — every one flush left; `.planning/WINDOWS.md` entry 1 `status: waived` |
| `260903-ceo-…/deferred-items.md` | quick task `260903-zwei-abgelegte-fehler`, `tools/mkbundle/main.go:184` | `grep -n 't\.Name\|t\.Slug' tools/mkbundle/main.go` → `terms[t.Name] = true` at 184; `t.Slug` survives only as a lookup for the error message |
| `260903-da5-…/deferred-items.md` | same quick task, 400 runs green | `grep -n 'f_' internal/public/formular_e2e_test.go` → the bare `strings.Contains(seite, "f_")` is gone; the assertion is now `<form class="contact-form" method="POST"` plus three negative traces, with the rationale at `:415-422` |

All three: `git diff --numstat` reports `2 0` — two added lines, zero deletions, per file.

## Files Created/Modified

- `.planning/codebase/ARCHITECTURE.md` — 6 countable facts, 20 line references, package box, three date stamps
- `.planning/codebase/STACK.md` — Go version, migrations, themes, target, six dependency versions, workflow list, production section
- `.planning/codebase/STRUCTURE.md` — directory counts, tree entry, package inventory, plugin archive claim, themes, workflow list
- `.planning/codebase/INTEGRATIONS.md` — the whole CI/CD and hosting section rewritten against the four workflows that exist
- `.planning/codebase/CONCERNS.md` — three dates, migration count, Go version, four dead `k8s/` citations, one dangling section pointer; all judgements intact
- `.planning/codebase/TESTING.md` — date stamps and the Go version only
- `.planning/codebase/CONVENTIONS.md` — date stamps and four line references
- `docs/offene-punkte.md` — numbered item 7 retired, procedure relocated, migration number corrected
- three `.planning/quick/*/deferred-items.md` — one closure stamp each, nothing removed

## Decisions Made

- **The tree wins over the research inventory, always.** Every line reference was re-derived at `f9ec0ea`, and the values matched `06-RESEARCH.md`'s "actual" column in all cases — the inventory was accurate where it looked. Where it had not looked at all, the tree still decided.
- **`CONCERNS.md` keeps the Go-version concern open.** `CLAUDE.md:7` still says "Go 1.22+" while `go.mod` says 1.26.6, so the mismatch it describes is real. Only the number moved.
- **The historical 1.57.0 measurement was preserved, not "corrected".** It is a correctly dated observation of a specific driver version; rewriting it would destroy the only record of when that check was last run.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Four dead `k8s/` citations in `CONCERNS.md` were outside the inventory but inside the verification gate**

- **Found during:** Task 1
- **Issue:** `06-RESEARCH.md` §E lists three corrections for `CONCERNS.md` and says "touch nothing else", and the acceptance criteria cap the file at "no more than 6 changed lines". But the plan's own task-level and plan-level verification require `grep -rn 'k8s/' .planning/codebase/` to print nothing, and `CONCERNS.md` carried four citations of `k8s/20-app.yaml` at lines 123, 208, 213 and 222. The two instructions cannot both be satisfied. The research pass had checked `CONCERNS.md` only for the items it expected to find there.
- **Fix:** Corrected the four citations surgically — each names the same operational fact (one process, one data directory, one volume) without the deleted manifest path. Every judgement, risk line and recommendation is byte-unchanged. Followed the standing rule from the phase notes: the tree wins, and the disagreement is recorded.
- **Files modified:** `.planning/codebase/CONCERNS.md`
- **Verification:** `grep -rn 'k8s/' .planning/codebase/` → exit 1. `CONCERNS.md` still carries the Go-version concern and every other finding.
- **Committed in:** `dde9f4a`
- **Cost:** `git diff --stat -- CONCERNS.md` reports 17 insertions / 15 deletions, above the plan's "no more than 6 changed lines". The overage is entirely the four citations plus the §7 pointer below; the three planned corrections plus the surgery note account for six.

**2. [Rule 3 - Blocking] The `docs/offene-punkte.md §7` pointer inside `CONCERNS.md` was invalidated by this plan's own task 2**

- **Found during:** Task 2 planning
- **Issue:** `CONCERNS.md` said "`docs/offene-punkte.md` §7 should be corrected to say eight". Task 2 removes §7 entirely, which would have left this plan creating exactly the kind of dangling anchor the phase exists to remove.
- **Fix:** Replaced that one sentence with one recording that the numbered item was retired in this plan and only its procedure survives, under `## Beim Weiterarbeiten`. The finding itself — that the PR list was stale — is unchanged.
- **Files modified:** `.planning/codebase/CONCERNS.md`
- **Verification:** `grep -n '^## [0-9]' docs/offene-punkte.md` lists 1-6; the CONCERNS sentence now points at a section that exists.
- **Committed in:** `dde9f4a`

**3. [Rule 1 - Bug] Three workflow-list claims and one Dockerfile claim the inventory had not flagged**

- **Found during:** Task 1
- **Issue:** `STACK.md:79` said `.github/workflows/{ci,deploy,security}.yml` and `STRUCTURE.md:89` said `ci.yml, deploy.yml, security.yml`. Neither matches `ls .github/workflows/` (`ci image release security`). `STACK.md:44` described the builder as `golang:1.26-bookworm` pinned to `$BUILDPLATFORM`; `Dockerfile:11` is `FROM golang:1.26` with no platform pin, and `Dockerfile:9` states the build is `linux/amd64`.
- **Fix:** Corrected all four against `ls` and `grep FROM Dockerfile`.
- **Files modified:** `.planning/codebase/STACK.md`, `.planning/codebase/STRUCTURE.md`
- **Verification:** `grep -rn 'deploy\.yml' .planning/codebase/` → exit 1.
- **Committed in:** `dde9f4a`

**4. [Rule 2 - Missing Critical] `STRUCTURE.md`'s top-level tree omitted `build/`**

- **Found during:** Task 1
- **Issue:** Removing the `k8s/` entry would have left the tree listing nine of the ten top-level directories. `build/` (a dev seed program and five prebuilt example bundles) has never been in it.
- **Fix:** The `k8s/` line became a `build/` line in the same position — one deleted claim replaced by one true one, tree height unchanged.
- **Files modified:** `.planning/codebase/STRUCTURE.md`
- **Verification:** `ls -d */` matches the tree entries.
- **Committed in:** `dde9f4a`

---

**Total deviations:** 4 auto-fixed (2 bugs, 1 blocking, 1 missing critical)
**Impact on plan:** All four are the same class of defect MAINT-05 exists to remove — a planning note asserting something the tree does not support. Three of them are required by the plan's own verification gate; the fourth is repair of an anchor this plan itself broke. No judgement in `CONCERNS.md` was altered and no map was regenerated, so D-19 holds. The only cost is that `CONCERNS.md` exceeds its stated 6-changed-line cap, which was written on the assumption that only three things in it were wrong.

## Out of Scope — Recorded, Not Fixed

- **`INTEGRATIONS.md`'s "Webhooks & Callbacks" says "No payment … webhook receivers"**, while `internal/payrexx/` exists. This is a claim about an integration surface, not a countable fact or a line reference, and correcting it needs someone to read what `internal/payrexx` actually exposes. Left for a later map pass — it is not part of MAINT-05's inventory and inventing an answer here would be the same failure mode this plan is fixing.
- **`CONCERNS.md`'s "Eight open Dependabot PRs"** finding is resolved by the tree (every named bump is in `go.mod`: crypto 0.55.0, sqlite 1.57.0, goldmark 1.8.5, goose 3.27.3, image 0.45.0, net 0.58.0). Closing a finding is a judgement change, which D-19 forbids here. Only its dangling section pointer was repaired.

## Issues Encountered

None. The three tasks ran without a failed verification.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Planners for Phases 7-10 can now read `.planning/codebase/` without silently inheriting six weeks of drift.
- Plan 06-07's four deferred anchors are untouched and still true to defer: `STRUCTURE.md`'s `tools/` line, `TESTING.md`'s `ci.yml`/`security.yml` description, the toolchain-pin instruction in `docs/offene-punkte.md`, and the Phase 6 note in `.planning/ROADMAP.md`.
- No blockers.

## Self-Check: PASSED

- All 11 modified files exist on disk and appear in `git diff --name-only dde9f4a~1..HEAD` — no path outside `files_modified`.
- All three task commits found: `dde9f4a`, `d447912`, `f895b63`.
- Plan-level verification re-run after the last commit: `grep -rn '2026-08-22' .planning/codebase/` → exit 1; `grep -rn 'k8s/\|deploy\.yml\|Raspberry Pi\|linux/arm64' .planning/codebase/` → exit 1; `grep -n '^## [0-9]' docs/offene-punkte.md` → 1-6; `git diff --numstat` on the three `deferred-items.md` → `2 0` each.

---
*Phase: 06-aufr-umen*
*Completed: 2026-09-04*
