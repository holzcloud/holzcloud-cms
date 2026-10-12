---
phase: 20-hardened-unit-under-real-systemd
verified: 2026-10-12T15:00:00Z
status: passed
score: 3/3 must-haves verified
covered_files:
  - .gitignore
  - .planning/phases/20-hardened-unit-under-real-systemd/20-01-PLAN.md
  - .planning/phases/20-hardened-unit-under-real-systemd/20-01-SUMMARY.md
  - .planning/phases/20-hardened-unit-under-real-systemd/20-02-PLAN.md
  - .planning/phases/20-hardened-unit-under-real-systemd/20-02-SUMMARY.md
  - .planning/phases/20-hardened-unit-under-real-systemd/20-03-PLAN.md
  - .planning/phases/20-hardened-unit-under-real-systemd/20-03-SUMMARY.md
  - deploy/SECURITY-SCORE.md
  - deploy/holzcloud.service
  - deploy/test/Dockerfile
  - deploy/test/Dockerfile.dockerignore
  - deploy/test/README.md
  - deploy/test/canary/go.mod
  - deploy/test/canary/main.go
  - deploy/test/run.sh
covered_digest: "v3:sha256:b65a2e8817fb7dfc388f5dcc5748ea767493fd89ac95629fd8695b6e8b59210a"
behavior_unverified: 0
overrides_applied: 0
re_verification: none
---

# Phase 20: Hardened unit under real systemd Verification Report

**Phase Goal:** The hardened `holzcloud.service` is proven to run, plugins included, under a real systemd.
**Verified:** 2026-10-12
**Status:** passed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The unit starts in a systemd container and the site answers HTTP (UNIT-01) | VERIFIED | `deploy/SECURITY-SCORE.md` UNIT-01 section pastes `/healthz` = `{"status":"ok"}` and `/readyz` status ok + integrity ok. Scratch `deploy/test/out/amd64/http.txt` and `unit-start.txt` exist. Native arm64 started the unchanged unit with no drop-in (no `unit-start-unchanged.txt` in `out/arm64`). On amd64 under emulation the first start crashed (`mmap_min_addr`, status 5/TRAP, saved as `unit-start-unchanged.txt`) and a container-only drop-in `ProcSubset=all` was needed; this is disclosed, gated on the crash signature, and the shipped unit is untouched. |
| 2 | Every shipped wasm plugin loads under the unit; the MDWE vs. wazero question is settled with evidence and the unit adjusted if needed (UNIT-02) | VERIFIED | `plugins/` holds exactly five plugins (bestellung, jahreszahl, kontaktformular, nicht-gefunden, suche). `out/arm64/check.txt`: 17 PASS lines, all five `loaded`, `running`, `invoke`, `stable`; `engine: interpreter (0 anonymous executable mappings)`; `Seccomp: 2`, 24 filters. `verdict.txt` arm64 = SETTLED, amd64 = VOID (Seccomp 0, canary not-enforced), so the amd64 pass is not misquoted as MDWE evidence. wazero v1.12.0 source confirms the mechanism: `NewRuntimeConfig()` is `engineKindAuto`, which picks the compiler only if `platform.CompilerSupports` (including `executableMmapSupported`) holds, else the interpreter; `internal/plugin/runtime.go:112` uses `NewRuntimeConfig()`. Decision "unit keeps `MemoryDenyWriteExecute=true`" is in the unit as a comment-only diff (`git diff daf651f -- deploy/holzcloud.service` shows only added `#` lines; directives unchanged); `git diff daf651f -- internal plugins` is empty. |
| 3 | `systemd-analyze security` score measured and committed; container-vs-server differences named (UNIT-03) | VERIFIED | `deploy/SECURITY-SCORE.md` carries four raw `Overall exposure level for holzcloud.service` lines (1.3 online / 1.4 offline on systemd 252; 1.3 / 1.3 on 257). I confirmed by string match that the appendix blocks equal the scratch files (`amd64/score-online`, `score-offline`, `amd64-trixie/score-online`, `arm64/check`, both `canary.txt`). Section "Container vs. server" has a table of 10 aspects, each labelled measured / inferred / assumed, plus a seccomp-directive table and a "Not measured" section. Score was measured on the unit as shipped (score runs before the drop-in start). |

**Score:** 3/3 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `deploy/test/Dockerfile` | stages build, canary, systemd-base, unit-test; copies unit unchanged | VERIFIED | all four stages present; `COPY deploy/holzcloud.service /etc/systemd/system/holzcloud.service` (line 81), not enabled |
| `deploy/test/Dockerfile.dockerignore` | overrides root ignore | VERIFIED | tracked in git |
| `deploy/test/run.sh` | subcommands build/up/start/wait/canary/exec/logs/down/smoke/seed/check/mdwe/score/all | VERIFIED | executable, `bash -n` clean, 33 KB, committed |
| `deploy/test/canary/{main.go,go.mod}` | stdlib-only probe, own module | VERIFIED | `go vet` in the module passes; own module `hc-canary` |
| `deploy/SECURITY-SCORE.md` | record + raw output + container-vs-server | VERIFIED | nine sections plus appendix; raw blocks match scratch |
| `deploy/test/README.md` | hand-over to phases 21 and 22 | VERIFIED | sections "Hand-over to phase 21" and "Hand-over to phase 22", subcommand and variable tables, pitfalls |
| `.gitignore` | `/deploy/test/out/` ignored | VERIFIED | `git check-ignore` confirms |
| `deploy/holzcloud.service` | MDWE decision explained, directive unchanged | VERIFIED | seven added comment lines, no directive changed |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| run.sh | Dockerfile | `docker build -f deploy/test/Dockerfile --target` | WIRED (summaries and smoke logs on both archs) |
| Dockerfile | deploy/holzcloud.service | COPY, unchanged | WIRED |
| run.sh | hc-canary | systemd-run with the unit's sandbox properties | WIRED (`canary.txt` and `canary.verdict` exist for both archs) |
| unit comment | deploy/SECURITY-SCORE.md | reference | WIRED (file now exists) |

### Behavioral Spot-Checks

| Behavior | Check | Result | Status |
|----------|-------|--------|--------|
| run.sh parses | `bash -n deploy/test/run.sh` | syntax ok | PASS |
| Canary compiles | `go vet` in `deploy/test/canary` | ok | PASS |
| Plugin behavior under enforced MDWE | read `out/arm64/check.txt`, `verdict.txt` (scratch from the executor's run; Docker not re-run, per instructions) | 17/17 PASS, interpreter engine, Seccomp 2, SETTLED | PASS (evidence read, not re-executed) |
| i18n gate | `go run ./tools/i18n` | `0 open, 0 orphaned` on the catalogues shown | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| UNIT-01 | 20-01 (and 20-03) | Unit runs under real systemd, starts, answers HTTP | SATISFIED | Truth 1 |
| UNIT-02 | 20-02 | All plugins (wazero) load under the hardened unit; MDWE suspicion decided with evidence, unit adjusted if needed | SATISFIED | Truth 2 |
| UNIT-03 | 20-03 | `systemd-analyze security` measured and committed | SATISFIED | Truth 3 |

All three IDs appear in PLAN frontmatter, are marked `[x]` and "Complete" in `.planning/REQUIREMENTS.md`, and no further requirement is mapped to Phase 20 (no orphans).

### Anti-Patterns Found

None. No `TBD`/`FIXME`/`XXX` in any file the phase modified. No stub files.

### Notes and Residual Risks (advisory, not blocking)

- **MDWE evidence is native arm64 only; the production target is linux/amd64.** The documents state this plainly ("assumed" for amd64 behavior, "Not measured: native amd64"), and the arm64-only refinement of D-01 was a recorded user decision on 2026-10-12. The wazero source check above supports the assumption that the probe is architecture independent. The document also gives the operator a two-command confirmation on a real server (`journalctl ... plugin loaded`, `grep Seccomp`).
- **UNIT-01 on amd64 needed a container-only drop-in** (`ProcSubset=all`) because of the emulation layer; the shipped unit is not changed, and native arm64 started it unchanged. Named in the record as an emulation artefact.
- Docker evidence under `deploy/test/out/` is gitignored scratch and was not re-run; claims were judged against the committed docs, `run.sh`, summaries, and the scratch files that are present (which match the pasted appendix).
- Outside this phase's scope (handed to phase 22 in README): `backup.sh` path and `sqlite3`/`rsync` mismatches.

### Human Verification Required

None.

### Gaps Summary

No gaps. All three roadmap success criteria are backed by committed documentation that matches the measured scratch output, a unit that is unchanged apart from explanatory comments, and a reusable `deploy/test/` recipe handed over to phases 21 and 22.

---

_Verified: 2026-10-12_
_Verifier: Claude (gsd-verifier)_
