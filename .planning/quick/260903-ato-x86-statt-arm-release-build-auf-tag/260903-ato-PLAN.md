---
phase: quick-260903-ato
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .github/workflows/ci.yml
  - .github/workflows/release.yml
  - .github/workflows/security.yml
  - README.md
  - CLAUDE.md
  - CONTRIBUTING.md
  - deploy/DEPLOY.md
autonomous: true
requirements: [DEP-01]

estimate:
  tokens: 45000
  raw_tokens: 45000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "Pushing a v* tag produces a GitHub Release with a linux/amd64 binary and its checksum attached"
    - "A push to main produces a downloadable linux/amd64 binary artifact with a checksum"
    - "The weekly security audit finishes its race-detector step instead of being killed by a timeout"
    - "No file outside .planning/ describes this project as targeting Raspberry Pi hardware"
    - "Both workflows are enabled on GitHub and a real run on main is green"
  artifacts:
    - ".github/workflows/release.yml"
    - ".github/workflows/ci.yml (amd64, no hardware-named job)"
    - ".github/workflows/security.yml (explicit -timeout on the race step)"
  key_links:
    - "fetch-depth: 0 -> git describe --tags -> -X main.Version -> holzcloud version output"
    - "push tag v* -> release job with contents: write -> gh release create -> attached binary + .sha256"
    - "gh workflow enable (before push) -> next push actually triggers a run"
---

<objective>
Retarget the whole repository from Raspberry Pi / linux-arm64 to linux/amd64, give a
version tag a working path to a GitHub Release, and unblock the security audit that
has been timing out since 2026-08-23.

Purpose: the user runs this on an x86 Kubernetes cluster now. The Pi framing is dead
weight, a tag currently produces nothing at all (no tag trigger exists anywhere), and
both workflows are switched off on GitHub so the last ten pushes went unchecked.

Output: three correct workflow files, honestly rewritten docs and code comments, and
two re-enabled workflows with a green run to prove it.
</objective>

<execution_context>
@/Users/holz/.claude/gsd-core/workflows/execute-plan.md
@/Users/holz/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@/Users/holz/Projects/holzcloud-cms/CLAUDE.md
@/Users/holz/Projects/holzcloud-cms/.github/workflows/ci.yml
@/Users/holz/Projects/holzcloud-cms/.github/workflows/security.yml
@/Users/holz/Projects/holzcloud-cms/deploy/DEPLOY.md
</context>

<facts_already_verified>
Do not re-derive these. They were measured before planning.

- `gh workflow list --all`: CI (258602985) and Security Audit (258602986) are both
  `disabled_manually`. Dependabot is active. Last CI run on main: 2026-08-22.
- Security run 32608395932 (2026-08-23) failed with
  `FAIL github.com/holzcloud/cms/internal/plugin 600.060s` — Go's default 10-minute
  test timeout. `go test -race ./internal/plugin/` measures **297 s** on the
  developer's machine. The module was later renamed to `holzcloud-cms`; the failure
  predates the rename and is unrelated to it.
- `ci.yml` has `push: branches:[main]` and `pull_request: branches:[main]` only.
  **No tag trigger, no release job** — that is why a tag produces nothing.
- Both ci.yml jobs use `fetch-depth: 0` because `git describe` needs history, and
  `-ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)"`.
  `main.Version` / `main.Commit` are declared in `cmd/holzcloud/main.go` (~line 60).
- ci.yml sets `permissions: contents: read` at file level.
- Tags v1.0–v1.3 live on `archive/gsd-v1.1-dead` and are not ancestors of main.
  The first tag on main is `v1.4`. `git describe --tags` on main returns `v1.4`.
- Go toolchain: `go 1.26.2`, module `github.com/holzcloud/holzcloud-cms`.
- `gh` 2.95.0 is installed and authenticated. `python3 -c "import yaml"` works.
- i18n baseline, captured just now:
  `en.json / es.json / fr.json / it.json` each report `1126 übersetzt, 0 offen, 0 verwaist`.
  (de-CH 51, fr-CH 4, it-CH 9 Abweichungen — pre-existing, not caused by this work.)
</facts_already_verified>

<scope_boundaries>
**MUTABLE SCOPE.** The set of files carrying a hardware reference is live state. Task 2
re-derives it with its own grep at execution time. The table below is investigation
guidance only — do not treat it as a frozen authorization list, and do not run a blind
`sed` across the tree.

**HARD EXCLUSION — `internal/i18n/locales/it.json`.** It had 26 apparent hits. Every one
is a false positive: Italian words ("più", "pi") matching `\bPi\b`. It is a translation
catalogue. **Do not touch it.** `go run ./tools/i18n` must still report
`0 offen, 0 verwaist` for en/es/fr/it afterwards.

**HARD EXCLUSION — Argon2id cost parameters.** `internal/auth/password.go` sets
`Memory: 64*1024, Iterations: 1, Parallelism: 2`. These are env-configurable
(`HOLZCLOUD_ARGON2_MEMORY` / `_ITERATIONS` / `_PARALLELISM`). Changing a password-hashing
cost is a security decision for the user, not a side effect of a docs cleanup. **Only the
comment that justifies the default by the hardware may be rewritten.**

**HARD EXCLUSION — `.planning/**`.** Milestone archives, research notes and roadmap
entries are a historical record of what was decided when. Rewriting them would be
falsifying history. Do not stage anything under `.planning/` except this plan's own
SUMMARY.

**OUT OF SCOPE — Kubernetes and containers.** The user said "erst mal" about the cluster
and did not ask for manifests. Commit `015611f` ("Container und Cluster wieder heraus")
deliberately removed an earlier Kubernetes deploy; a `deploy/kubernetes` branch still
holds it. **Do not add a Dockerfile or Kubernetes manifests.** systemd + Caddy stay —
they are valid for a plain amd64 Linux host.

**No new dependencies.** This work adds none. No npm/pip/cargo install occurs, so the
package legitimacy gate does not apply.
</scope_boundaries>

<tasks>

<task type="tracer">
  <name>Task 1: A tag reaches a Release — the whole build path, amd64, end to end</name>
  <files>.github/workflows/ci.yml, .github/workflows/release.yml, .github/workflows/security.yml</files>
  <read_first>.github/workflows/ci.yml, .github/workflows/security.yml</read_first>
  <action>
Wire one path from a pushed version tag all the way to a published GitHub Release, and
fix the two existing workflows around it. Three files, one coherent shape.

**Design decision A — the second ci.yml job is dissolved, not renamed.** The `test` job
already runs on `ubuntu-latest`, which *is* linux/amd64, and already builds with the
production ldflags. A second job cross-compiling from amd64 to amd64 would build the
identical binary twice and buy nothing. What the second job genuinely bought was the
*uploaded artifact* — a per-commit binary to roll back to. So: delete the second job and
move the artifact production into `test`. Change its Build step to write to
`-o holzcloud-linux-amd64`, add `sha256sum holzcloud-linux-amd64 > holzcloud-linux-amd64.sha256`
and echo it, then add an `actions/upload-artifact@v7` step uploading both files with
`retention-days: 30`. Keep `fetch-depth: 0`, keep `CGO_ENABLED: "0"`, keep the existing
ldflags verbatim. Also add `workflow_dispatch:` to the `on:` block so the workflow can be
fired by hand — without it there is no way to trigger a check other than a fresh push,
which is exactly the corner this repo is in right now.

**Design decision B — the release lives in its own file, not in ci.yml.** A separate
`.github/workflows/release.yml` keeps `ci.yml` at file-level `permissions: contents: read`
(least privilege — a workflow that never publishes never gets a write token), keeps the
tag trigger out of a file whose triggers are branch-shaped, and makes the release path
readable as one unit. Give it: `name: Release`; `on: push: tags: ['v*']`;
file-level `permissions: contents: read`; `env: CGO_ENABLED: "0"`; one job `release`
on `ubuntu-latest` that overrides `permissions: contents: write` **at job level** so the
write scope exists only where it is used. Steps, in order: `actions/checkout@v7` with
`fetch-depth: 0` (leave it out and `git describe` silently degrades to a bare commit hash
again — that is the exact bug the v1.4 tag was created to fix); `actions/setup-go@v7`
with `go-version-file: go.mod` and `cache: true`; `go test ./...` as a gate, so a broken
tag cannot become an official Release; the build, `GOOS: linux` / `GOARCH: amd64`, same
ldflags as ci.yml, `-o holzcloud-linux-amd64`; `sha256sum` into
`holzcloud-linux-amd64.sha256`; and finally `gh release create "${GITHUB_REF_NAME}"
--generate-notes --title "${GITHUB_REF_NAME}"` attaching both files, with
`env: GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`.

Use the `gh` CLI, which is preinstalled on the runner, rather than a third-party release
action. It is one less unpinned third party in the publish path, and it matches this
project's stdlib-first posture. Only first-party `actions/*` steps appear anywhere.

**Design decision C — security.yml gets an explicit timeout with the measurement
recorded.** The race step inherits Go's 10-minute default and the wasm plugin host
crosses it. Set `-timeout 30m` on the `go test -race ./...` invocation. Write a YAML
comment directly above the step giving the number and the date — that
`go test -race ./internal/plugin/` measured 297 s locally on 2026-09-03, that a hosted
runner is slower, and that 30m is deliberate headroom over a genuinely slow
wazero-under-race package. Without that note the next person reads "30m" as paranoia and
lowers it back to the value that was already proven to fail. Change nothing else in that
file; the module-verify and vet steps are fine.

Do not leave the old hardware target name behind in a YAML comment anywhere in
`.github/workflows/`. The acceptance grep for this task is comment-stripped, so a stray
mention would not fail the gate — it would just quietly re-establish the thing being
removed.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms && python3 -c "import yaml,sys; [yaml.safe_load(open(f)) for f in sys.argv[1:]]; print('yaml ok')" .github/workflows/ci.yml .github/workflows/release.yml .github/workflows/security.yml && test "$(grep -rniE 'raspberry|raspi|arm64|aarch64' .github/workflows/ | grep -v '^[^:]*:[0-9]*: *#' | wc -l | tr -d ' ')" = "0" && grep -q 'amd64' .github/workflows/ci.yml && grep -q 'amd64' .github/workflows/release.yml && grep -q "tags:" .github/workflows/release.yml && grep -q 'contents: write' .github/workflows/release.yml && grep -q 'fetch-depth: 0' .github/workflows/release.yml && grep -q 'timeout 30m' .github/workflows/security.yml && grep -q 'workflow_dispatch' .github/workflows/ci.yml && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)" -o /tmp/holzcloud-linux-amd64 ./cmd/holzcloud && file /tmp/holzcloud-linux-amd64 | grep -q 'x86-64' && sha256sum /tmp/holzcloud-linux-amd64 && echo "TRACER OK"</automated>
  </verify>
  <done>
All three workflow files parse as YAML. `ci.yml` builds and uploads a linux/amd64
binary plus checksum from the `test` job, has no second job, and accepts a manual
dispatch. `release.yml` exists, triggers on `v*` tags, holds `contents: write` at job
level only, keeps `fetch-depth: 0`, gates on `go test ./...`, and calls `gh release
create` with the binary and its `.sha256`. `security.yml` carries `-timeout 30m` on the
race step with the 297 s measurement recorded in a comment above it. No workflow file
mentions the old hardware target outside a comment, and the exact build command from the
workflows reproduces an x86-64 ELF locally.
  </done>
  <reversibility rating="reversible">Workflow files are plain YAML under version control; a revert restores the previous behaviour with no external state left behind.</reversibility>
</task>

<task type="auto">
  <name>Task 2: Remove the Pi framing from prose and code comments, honestly</name>
  <files>README.md, CLAUDE.md, CONTRIBUTING.md, deploy/DEPLOY.md, plus the .go and .md files the live grep returns</files>
  <read_first>README.md, CLAUDE.md, CONTRIBUTING.md, deploy/DEPLOY.md</read_first>
  <!-- planner-discipline-allow: Raspberry, Pi, Raspi -->
  <!-- The acceptance grep below excludes .planning/, where this plan file lives, so
       naming the literals here cannot self-invalidate the gate. The allowlist marker is
       for the planner lint only. -->
  <action>
**Step 1 — re-derive the live scope yourself.** Run this and work from its output, not
from any list written down earlier:

  grep -rnIE '\bPi\b|[Rr]aspberry|[Rr]aspi|arm64|aarch64|ARM64' --include='*.go' --include='*.md' --include='*.sh' --include='*.yml' . | grep -v '^\./\.planning/' | grep -v '^\./\.git/'

Investigation found roughly twenty sites at planning time — README.md, CLAUDE.md,
CONTRIBUTING.md, deploy/DEPLOY.md, docs/vergleich-statamic.md, sites/README.md,
cmd/holzcloud/assets/VENDOR.md, and one-line comments in internal/auth/password.go,
internal/web/diskspace_other.go, internal/media/variants.go, internal/bundle/export.go,
internal/plugin/{manifest,package,manager,runtime}.go, internal/public/plugins.go,
internal/wxr/wxr.go, internal/payrexx/payrexx.go, internal/ai/mcp.go,
plugins/kontaktformular/main.go, tools/mkbundle/main.go. Treat that as a starting map.
The grep is the authority. If it returns a file not listed, handle it; if a listed file
no longer matches, skip it.

**Step 2 — classify every hit before editing it.** There are exactly two kinds, and they
get opposite treatment.

*Framing.* Statements that the deployment target is a specific single-board computer, and
cross-compile instructions naming the 64-bit ARM target. These go, or become linux/amd64.
Concretely: README.md line 3 and CLAUDE.md line 3 (the one-line project description);
README.md "Build" section (the production build line becomes
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ...`); README.md "Deployment" heading and its
`scp` example (a generic `user@your-server:/opt/holzcloud/` reads better than a
hardware-specific hostname); README.md deploy-table row for DEPLOY.md; README.md Tech
Stack row for the SQLite driver — the driver's actual virtue is that it is CGO-free and
therefore builds as a static binary with no C toolchain, which is true and useful
independent of architecture, so say *that*; CLAUDE.md lines 49 and 139 for the same
reasons; CONTRIBUTING.md line 3 and line 46; and all of deploy/DEPLOY.md.

deploy/DEPLOY.md is the heaviest single file (19 hits, 303 lines). It becomes a guide for
a plain amd64 Debian/Ubuntu server: retitle it, replace the ARM cross-compile lines
(31, 187) with the amd64 equivalent, drop the SD-card fsync warning and the USB-SSD
advice (22, 90) — on a server the storage question is a normal one and does not need a
flash-card caveat — generalise the transfer/SSH steps (36-45, 189-192), and rewrite the
prerequisites (7-18) and the memory-footprint note (229). Keep the systemd unit, the
Caddy config, the backup/restore scripts and all the hardening guidance: none of it was
hardware-specific and all of it still applies.

*Reasoning that happens to name the hardware.* Several comments explain a real number or
a real design choice, and the small machine is the evidence they cite. Deleting the
sentence deletes the justification and leaves a magic constant. Rewrite the reason
honestly instead — a modest cluster node is not a workstation either, so in most cases
the constraint survives and only the example changes. This covers
`internal/media/variants.go` (why 400/800 and why not four sizes),
`internal/bundle/export.go` and `internal/wxr/wxr.go` (why streaming, why the size
limit — holding a gigabyte in memory is bad on any small node),
`internal/plugin/package.go`, `manager.go` and `runtime.go` and
`internal/public/plugins.go` (memory ceilings, per-request cost, disk reads),
`tools/mkbundle/main.go` (why already-compressed photos skip deflate), and
`internal/web/diskspace_other.go` (the real reason is that the syscall is Linux-only and
the target is Linux — the hardware was never the point). For
`internal/plugin/manifest.go` and `internal/ai/mcp.go` the argument is about *static
cross-compilation without cgo*, which remains exactly true — just drop the architecture
name and keep the argument. For `internal/auth/password.go` line 22: rewrite the comment
to state that the defaults are sized for a small single-tenant server and are
env-tunable; **do not touch the parameter values on the following lines.**

`plugins/kontaktformular/main.go:77`, `docs/vergleich-statamic.md`, `sites/README.md` and
`cmd/holzcloud/assets/VENDOR.md` are German prose making the same "small machine" point —
rewrite in German, same register, keep the argument.

**Step 3 — leave the excluded things alone.** No edit to `internal/i18n/locales/*.json`.
No edit under `.planning/`. No change to the Argon2 numbers. Do not stage
`.planning/REQUIREMENTS.md` or `.planning/state.json`, which already carry unrelated
uncommitted changes.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms && test "$(grep -rnIE '\bPi\b|[Rr]aspberry|[Rr]aspi|arm64|aarch64|ARM64' --include='*.go' --include='*.md' --include='*.sh' --include='*.yml' . | grep -v '^\./\.planning/' | grep -v '^\./\.git/' | wc -l | tr -d ' ')" = "0" && git diff --quiet -- internal/i18n/locales/ && grep -q '64 \* 1024' internal/auth/password.go && grep -q 'Iterations:  1,' internal/auth/password.go && test -z "$(gofmt -l .)" && go build ./... && go vet ./... && go test ./... 2>&1 | tail -5 && test "$(go run ./tools/i18n 2>&1 | grep -cE '^(en|es|fr|it)\.json .*0 offen, 0 verwaist')" = "4" && echo "DOCS OK"</automated>
  </verify>
  <done>
The live grep returns nothing outside `.planning/`. `internal/i18n/locales/` is
untouched per `git diff --quiet`. The Argon2 default values are byte-identical. The tree
is gofmt-clean, `go build ./...`, `go vet ./...` and `go test ./...` all pass with no
FAIL line, and `go run ./tools/i18n` still reports `0 offen, 0 verwaist` for all four of
en/es/fr/it. Every rewritten comment still explains *why* its number or design choice is
what it is — no justification was deleted along with the hardware name.
  </done>
</task>

<task type="auto">
  <name>Task 3: Turn the workflows back on and prove a run is green</name>
  <files>(no file edits — repo-side GitHub state and one commit)</files>
  <precondition>`gh auth status` succeeds and the authenticated account has write access to the origin repository.</precondition>
  <action>
Order matters here, and it is not the obvious order.

**First commit locally.** Stage only what this plan touched — the three workflow files
and the doc/comment files from Task 2 — using explicit pathspecs. The tree was clean at planning time, but a
bare `git add -A` would still sweep in whatever another workflow leaves behind mid-run. Commit with a message
describing the retarget to linux/amd64, the tag-driven release path, and the audit
timeout fix.

**Then enable, then push.** Run `gh workflow enable ci.yml` and
`gh workflow enable security.yml`. Enabling does not itself fire a run — it only restores
the trigger — so doing it before the push is what makes the push actually get checked.
Reversed, the push lands against still-disabled workflows and nothing runs; `ci.yml` has
no dispatch trigger today to rescue that (Task 1 adds one, but only once it is on the
remote). Enabling before the commit is landed would be the other failure: the trigger
would come back while the old hardware-shaped file was still the version on the remote.
Local commit, then enable, then push, in that sequence.

**Then push and watch.** `git push origin main`, then confirm with `gh run list --limit 3`
that a CI run started against the new head, and follow it with `gh run watch` until it
concludes. If it fails, read the log with `gh run view --log-failed`, fix the cause, and
push again — a red run means this task is not done. Report the final conclusion and the
run URL in the summary.

Do not create a version tag as part of this task. The release path is proven by the
workflow being syntactically valid and by its build command reproducing locally (Task 1);
cutting `v1.5` is the user's decision about when to release, not a verification step.
Note the exact command they will use — `git tag v1.5 && git push origin v1.5` — in the
summary so it is ready when they want it.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms && gh workflow list --all | grep -E 'CI|Security Audit' | grep -vc 'disabled' | grep -q '^2$' && git status --porcelain -- .github/ README.md CLAUDE.md CONTRIBUTING.md deploy/ internal/ docs/ sites/ cmd/ tools/ plugins/ | wc -l | tr -d ' ' | grep -q '^0$' && test -z "$(git log origin/main..HEAD --oneline)" && gh run list --limit 1 --json conclusion,headSha,workflowName | grep -q '"conclusion":"success"' && echo "SHIPPED OK"</automated>
  </verify>
  <done>
`gh workflow list --all` shows neither CI nor Security Audit as disabled. The plan's
files are committed with nothing left dirty and nothing unrelated swept in, `main` is
pushed (no commits ahead of `origin/main`), and the most recent GitHub run concluded
`success`. The summary records the run URL and the two commands for cutting the first
release tag.
  </done>
  <reversibility rating="costly">Pushing to `main` and enabling repo workflows are both visible outside the working tree. `gh workflow disable` reverses the enable exactly; undoing the push takes a further commit. Neither destroys anything.</reversibility>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| pushed git ref → GitHub Actions runner | A tag or branch push starts privileged automation |
| Actions runner → GitHub Releases API | The release job publishes an artifact the world downloads |
| third-party Action → runner filesystem and token | Any `uses:` step executes code in the token's scope |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-ATO-01 | Elevation of Privilege | `release.yml` publish token | high | mitigate | `contents: write` is declared at **job** level only; both files keep `contents: read` at file level, so no other job in the repo ever receives a write token |
| T-ATO-02 | Tampering | tag trigger → official Release | medium | mitigate | The release job runs `go test ./...` before `gh release create`, so a tag on broken code cannot become a published Release. Pushing a `v*` tag already requires repo write access, which is the intended authorization boundary |
| T-ATO-03 | Tampering | release binary in transit | medium | mitigate | A `sha256sum` file is produced beside the binary and attached to the same Release, so a downloader can verify what they got |
| T-ATO-04 | Tampering | third-party Actions supply chain | high | mitigate | Only first-party `actions/checkout`, `actions/setup-go` and `actions/upload-artifact` appear. Publishing uses the runner's preinstalled `gh` CLI with `secrets.GITHUB_TOKEN` rather than a third-party release action, removing one unpinned publisher from the path |
| T-ATO-05 | Information Disclosure | build artifacts on a public repo | low | accept | The uploaded binary is the same code the repository already publishes as source; a 30-day retention artifact leaks nothing the Release would not |
| T-ATO-SC | Tampering | npm/pip/cargo installs | n/a | n/a | No package-manager install occurs anywhere in this plan. The legitimacy gate does not apply; no `[ASSUMED]`/`[SUS]` package is introduced |
</threat_model>

<verification>
Run from `/Users/holz/Projects/holzcloud-cms`:

1. `python3 -c "import yaml,sys; [yaml.safe_load(open(f)) for f in sys.argv[1:]]" .github/workflows/*.yml` — every workflow parses.
2. `go build ./... && go vet ./... && go test ./...` — baseline is 38 packages, 0 FAIL.
3. `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)" -o /tmp/h ./cmd/holzcloud && file /tmp/h` — reports an x86-64 ELF, and `git describe` resolves to `v1.4` rather than a bare hash.
4. `go run ./tools/i18n` — en/es/fr/it each still `0 offen, 0 verwaist`.
5. `git diff --quiet -- internal/i18n/locales/` — the translation catalogues are untouched.
6. Live grep for the hardware name across `*.go`/`*.md`/`*.sh`/`*.yml`, excluding `.planning/` and `.git/` — returns nothing.
7. `gh workflow list --all` — neither CI nor Security Audit is disabled.
8. `gh run list --limit 1` — the run against the new head concluded `success`.
</verification>

<success_criteria>
- A `v*` tag push has a defined path to a GitHub Release carrying `holzcloud-linux-amd64` and `holzcloud-linux-amd64.sha256`, with `main.Version` populated from `git describe` rather than a bare commit hash.
- A push to main uploads the same pair as a 30-day artifact, from a single job.
- The security audit's race step carries `-timeout 30m` and a comment recording the 297 s measurement that justifies it.
- Nothing outside `.planning/` describes this project as targeting Raspberry Pi hardware, and no comment lost its reasoning in the process.
- `internal/i18n/locales/*.json` and the Argon2id cost parameters are bit-identical to before.
- Both workflows are enabled and a real run on main is green.
</success_criteria>

<notes>
**Follow-ups, deliberately not done here:**

- `.planning/REQUIREMENTS.md` DEP-01 and FND-01 still specify `linux/arm64`. They are now
  factually stale, but `.planning/` is excluded from this plan's edit scope. Worth a
  separate pass.
- `.planning/codebase/STACK.md` names `linux/arm64` as the primary target. It is a
  generated map — regenerate with `/gsd-map-codebase` rather than hand-editing.
- A container image would suit a Kubernetes deployment, but commit `015611f` removed one
  on purpose and the user did not ask for it back. Recorded as an option, not an action.
</notes>

<output>
Create `.planning/quick/260903-ato-x86-statt-arm-release-build-auf-tag/260903-ato-SUMMARY.md` when done
</output>
