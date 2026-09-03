---
phase: quick-260903-ato
plan: 01
subsystem: infra
tags: [github-actions, ci, release, amd64, cross-compile, docs]
status: complete

requires:
  - phase: v1.0 05-admin-polish-users-deployment
    provides: deploy/DEPLOY.md, systemd unit, Caddy config, backup scripts
provides:
  - "A v* tag push builds and publishes a GitHub Release with holzcloud-linux-amd64 and its .sha256"
  - "A push to main uploads the same binary + checksum pair as a 30-day artifact from one job"
  - "The weekly security audit's race step has 30m instead of Go's 10-minute default"
  - "CI and Security Audit are enabled on GitHub again after ~2 weeks of unchecked pushes"
  - "No file outside .planning/ describes this project as targeting Raspberry Pi hardware"
affects: [deployment, release-process, onboarding-docs]

actuals:
  tokens: 9700
  tasks: 3
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Publish-scope isolation: contents: write declared at job level only, never file level"
    - "Release publishing via the runner's preinstalled gh CLI rather than a third-party action"
    - "Measurement recorded in a comment beside any timeout constant that looks arbitrary"

key-files:
  created:
    - .github/workflows/release.yml
  modified:
    - .github/workflows/ci.yml
    - .github/workflows/security.yml
    - README.md
    - CLAUDE.md
    - CONTRIBUTING.md
    - deploy/DEPLOY.md
    - internal/auth/password.go
    - internal/media/variants.go
    - internal/bundle/export.go
    - internal/plugin/manager.go

key-decisions:
  - "The second ci.yml job was dissolved, not renamed: ubuntu-latest already is linux/amd64, so a cross-compile job would build the identical binary twice. What it genuinely bought — the uploaded artifact — moved into the test job."
  - "The release lives in its own release.yml so ci.yml keeps file-level contents: read. A workflow that never publishes never gets a write token."
  - "Comments that justified a constant by naming the hardware were rewritten, not deleted. A small cluster node is not a workstation either, so the constraint survives and only the example changes."
  - "Argon2id cost values left byte-identical. Only the comment justifying them was rewritten — changing a password-hashing cost is the user's security decision."
  - "No tag was cut. Releasing is the user's call about timing, not a verification step."

patterns-established:
  - "Hardware-shaped reasoning in comments: replace the example, keep the argument, never leave a magic constant bare"
  - "Timeout constants carry the measurement and date that justify them, so the next reader cannot mistake headroom for paranoia"

requirements-completed: [DEP-01]

coverage:
  - id: D1
    description: "A pushed v* tag builds linux/amd64 and publishes a GitHub Release with the binary and its checksum"
    requirement: DEP-01
    verification:
      - kind: other
        ref: "python3 yaml.safe_load .github/workflows/release.yml"
        status: pass
      - kind: other
        ref: "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ... && file → ELF 64-bit LSB executable, x86-64, statically linked"
        status: pass
      - kind: manual_procedural
        ref: "End-to-end publish is unproven until a tag is pushed — see 'What the user must do' below"
        status: unknown
  - id: D2
    description: "A push to main uploads holzcloud-linux-amd64 + .sha256 as a 30-day artifact"
    requirement: DEP-01
    verification:
      - kind: e2e
        ref: "GitHub run 33721382622 on 774b3ea — artifact holzcloud-linux-amd64, 9366692 bytes, expires 2026-10-03"
        status: pass
  - id: D3
    description: "The security audit's race step finishes instead of being killed by Go's default timeout"
    verification:
      - kind: other
        ref: "grep 'go test -race -timeout 30m' .github/workflows/security.yml"
        status: pass
      - kind: manual_procedural
        ref: "Next scheduled Sunday run, or gh workflow run security.yml — not fired here (a full -race pass costs ~30 CI-minutes)"
        status: unknown
  - id: D4
    description: "No file outside .planning/ frames this project as Raspberry Pi hardware, and no comment lost its reasoning"
    verification:
      - kind: other
        ref: "live grep over *.go/*.md/*.sh/*.yml excluding .planning and .git → 0 hits"
        status: pass
      - kind: unit
        ref: "go build ./... && go vet ./... && go test ./... → 38 ok, 0 FAIL"
        status: pass
      - kind: other
        ref: "go run ./tools/i18n → en/es/fr/it each 1126 übersetzt, 0 offen, 0 verwaist"
        status: pass
  - id: D5
    description: "Both workflows are enabled on GitHub and a real run on main is green"
    verification:
      - kind: e2e
        ref: "gh workflow list --all → CI active, Security Audit active; run 33721382622 conclusion success"
        status: pass

metrics:
  duration: ~35 min
  completed: 2026-09-03
---

# Quick Task 260903-ato: x86 statt ARM, Release-Build auf Tag — Summary

Retargeted the repository from Raspberry Pi / linux-arm64 to linux/amd64, gave a version
tag its first working path to a published GitHub Release, and unblocked the security audit
that had been dying on Go's default test timeout since 2026-08-23.

## What the user must do to actually see a release

**No release exists yet, and re-pushing `v1.4` will not produce one.**

`v1.4` was created and pushed on 2026-09-02, pointing at commit `2a48f36` — which predates
`release.yml` by two commits. GitHub Actions reads workflow files *from the tagged ref*, so
that tag had no release workflow to find when it was pushed, and it still has none today.
`gh release list` is empty. Deleting and force-re-pushing `v1.4` would not help either: the
commit it names still contains no `release.yml`.

The release path only exists on `a3d7ede` and later. To produce the first release, cut a new
tag on the current head:

```bash
git tag v1.5
git push origin v1.5
```

That triggers `Release`, which runs `go test ./...` as a gate, builds `holzcloud-linux-amd64`,
computes its `.sha256`, and publishes both to a GitHub Release named `v1.5` with generated
notes. Watch it with `gh run watch` and confirm with `gh release list`.

## Accomplishments

**Task 1 — the whole build path, amd64, end to end** (`a3d7ede`)

- `ci.yml`: the `cross-compile` job is gone. `ubuntu-latest` already *is* linux/amd64, so a
  second job cross-compiling amd64→amd64 would have built the identical binary twice. The one
  thing it genuinely contributed — an uploaded artifact to roll back to — moved into the `test`
  job, which now writes `-o holzcloud-linux-amd64`, computes `holzcloud-linux-amd64.sha256`,
  and uploads both with 30-day retention. `workflow_dispatch:` added, because without it the
  only way to get a check is a fresh push — which is exactly the corner this repo was in.
- `release.yml` (new): triggers on `v*` tags. File-level `permissions: contents: read`, with
  `contents: write` overridden at *job* level only, so the publish token exists nowhere else.
  Keeps `fetch-depth: 0` (without it `git describe` degrades to a bare commit hash — the exact
  defect `v1.4` was created to fix). Gates on `go test ./...` before publishing, so a tag on
  broken code cannot become an official release. Publishes via the runner's preinstalled `gh`
  CLI rather than a third-party release action.
- `security.yml`: `go test -race -timeout 30m ./...`, with a comment recording *why* — the
  2026-08-23 failure at `FAIL .../internal/plugin 600.060s`, and the 297 s local measurement
  for that package alone. Without the note, the next reader lowers 30m back to a value already
  proven to fail.

**Task 2 — the Pi framing removed honestly** (`774b3ea`)

46 sites across 32 files. Two kinds of hit, opposite treatment:

- *Framing* (README, CLAUDE.md, CONTRIBUTING.md, deploy/DEPLOY.md, docs, VENDOR.md): the
  project description, the `GOARCH=arm64` build lines, the `scp holzcloud pi@raspberrypi:`
  example, the deployment headings. `DEPLOY.md` (the heaviest, 16 hits) became a guide for a
  plain amd64 Debian/Ubuntu server — retitled, ARM cross-compile lines swapped for amd64,
  SD-card fsync warning and USB-SSD advice replaced with a normal storage paragraph, transfer
  and SSH steps generalised. The systemd unit, Caddy config, backup/restore scripts and all
  hardening guidance stayed untouched: none of it was ever hardware-specific.
- *Reasoning that happened to name the hardware* (~20 code comments): rewritten, not deleted.
  `variants.go` still explains why three sizes and not five, and why two concurrent decodes;
  `export.go` and `wxr.go` still explain why streaming and why the size limit; `plugin/*` and
  `public/plugins.go` still explain their memory ceilings. A modest node is not a workstation
  either, so the constraint survives and only the example changed. Where the argument was
  never really about hardware, the real reason is now stated: `diskspace_other.go` names the
  Linux-only syscall, `manifest.go` and `mcp.go` name static cross-compilation without cgo.

**Task 3 — workflows re-enabled, green run proven** (no file edits)

`gh workflow enable ci.yml` and `security.yml` ran *before* the push, which is what made the
push actually get checked — enabling restores the trigger but does not fire a run by itself.
Run [33721382622](https://github.com/holzcloud/holzcloud-cms/actions/runs/33721382622)
concluded **success** on `774b3ea` in 4m41s, with artifact `holzcloud-linux-amd64` (9.4 MB)
uploaded. First green CI on main since 2026-08-22.

## Deviations from Plan

**1. [Rule 1 — Bug] The plan's `.planning/` exclusion pattern does not match on this machine**

- **Found during:** Task 2 verification
- **Issue:** Both acceptance greps filter with `grep -v '^\./\.planning/'`, which assumes
  `grep -r .` emits paths prefixed `./`. This machine's `grep` is **ugrep 7.8.4**, which emits
  `.planning/PROJECT.md` with no prefix. The exclusion therefore never matched, and the check
  reported 154 hits — every one of them inside `.planning/`, i.e. inside the excluded scope.
  The work was compliant; the verification command was not.
- **Fix:** Ran the semantically-intended check, `grep -vE '^(\./)?\.planning/'`, which returns
  **0**. No source file was changed to accommodate this.
- **Files modified:** none (verification-command defect only)

**2. [Rule 2 — Missing critical correctness] A dangling cross-reference in DEPLOY.md**

- **Found during:** Task 2, after rewriting the storage section
- **Issue:** The Backups section read "a backup on the same disk does not survive the disk
  failure it exists for, **and this document itself calls that failure likely**" — a pointer to
  the SD-card corruption warning that the same task had just removed. Left as-is it would have
  referred to a paragraph that no longer exists.
- **Fix:** Rewrote the clause to stand on its own: "…and a backup that has never left the
  machine is not yet a backup." The argument for `REMOTE_TARGET` is preserved.
- **Files modified:** `deploy/DEPLOY.md`
- **Commit:** `774b3ea`

## Exclusions Honoured

| Exclusion | Evidence |
|-----------|----------|
| `internal/i18n/locales/**` never touched (26 hits are Italian "più"/"pi") | `git diff --quiet HEAD~2 HEAD -- internal/i18n/locales/` clean; `go run ./tools/i18n` still reports 1126 übersetzt / 0 offen / 0 verwaist for en, es, fr, it |
| Argon2id cost values unchanged | `Memory: 64 * 1024`, `Iterations: 1`, `Parallelism: 2` byte-identical. Only the comment above them was rewritten, to say the defaults are sized for a small single-tenant server and are env-tunable |
| No Dockerfile, no Kubernetes manifests | None added. Commit `015611f` removed those deliberately; the user said "erst mal" about the cluster and did not ask for them back |
| `.planning/**` not rewritten | Nothing under `.planning/` staged except this SUMMARY. Stale statements listed as follow-ups below |

## Follow-ups (deliberately not done here)

These are all inside `.planning/`, which this plan excluded. A shipped v1.0 requirement saying
`linux/arm64` was **true when it shipped** — rewriting it would falsify the record rather than
correct it. They want a separate, deliberate pass:

- `.planning/REQUIREMENTS.md` — **DEP-01** ("single self-contained binary for linux/arm64") and
  **FND-01** ("compiles with `CGO_ENABLED=0 GOARCH=arm64`") are now factually stale. DEP-01 is
  the requirement this plan completed, so its wording is the most misleading of the two.
- `.planning/codebase/STACK.md` line 20 and 88, and `.planning/codebase/INTEGRATIONS.md` line
  83 — these name `linux/arm64` as the primary target and describe ci.yml as having "a
  `linux/arm64` cross-compile job", which no longer exists. They are *generated* maps:
  regenerate with `/gsd-map-codebase` rather than hand-editing.
- `.planning/PROJECT.md`, `.planning/STATE.md`, `.planning/ROADMAP.md` — core-value statements
  and phase goals still name the Pi. These are narrative and low-harm, but a reader starting
  from `PROJECT.md` gets the wrong target.
- A container image would suit a Kubernetes deployment, but `015611f` removed one on purpose.
  Recorded as an option, not an action.

## Known Stubs

None. No hardcoded empty value, placeholder string, or unwired component was introduced.

## Threat Flags

None. No new network endpoint, auth path, file-access pattern, or schema change at a trust
boundary. The threat register's four `mitigate` dispositions were all implemented as planned:
job-level `contents: write` (T-ATO-01), `go test` gate before publish (T-ATO-02), `.sha256`
attached beside the binary (T-ATO-03), and first-party `actions/*` plus the runner's own `gh`
CLI as the only things in the publish path (T-ATO-04).

## Verification Results

| # | Check | Result |
|---|-------|--------|
| 1 | `python3 yaml.safe_load .github/workflows/*.yml` | OK — all three parse |
| 2 | `go build ./...` | OK |
| 3 | `go vet ./...` | OK |
| 4 | `go test ./...` | **38 ok, 0 FAIL** — matches baseline |
| 5 | `go run ./tools/i18n` | en/es/fr/it each `1126 übersetzt, 0 offen, 0 verwaist` |
| 6 | `git diff --quiet -- internal/i18n/locales/` | clean |
| 7 | `git describe --tags` | `v1.4-3-g774b3ea` — a real describe, not a bare hash |
| 8 | amd64 build + `file` | `ELF 64-bit LSB executable, x86-64, statically linked, stripped` |
| 9 | live hardware grep outside `.planning/` | 0 hits |
| 10 | `gh workflow list --all` | CI **active**, Security Audit **active** |
| 11 | `gh run list --limit 1` | conclusion **success**, head `774b3ea` |
| 12 | run artifacts | `holzcloud-linux-amd64`, 9366692 bytes, expires 2026-10-03 |

Pre-existing and unrelated: de-CH 51, fr-CH 4, it-CH 9 Abweichungen in the i18n report. These
were present in the recorded baseline and are not caused by this work.

## Self-Check: PASSED

- `.github/workflows/release.yml` — FOUND
- `.github/workflows/ci.yml` — FOUND
- `.github/workflows/security.yml` — FOUND
- commit `a3d7ede` — FOUND
- commit `774b3ea` — FOUND
