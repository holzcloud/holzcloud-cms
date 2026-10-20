# Phase 20: Hardened unit under real systemd - Context

**Gathered:** 2026-10-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Prove that `deploy/holzcloud.service` runs under a real systemd (in a Linux container),
that the site answers HTTP, that every shipped wasm plugin loads under the unit, and
record the `systemd-analyze security` score. Settle the `MemoryDenyWriteExecute=true`
vs. wazero question with evidence. Deliver a reusable test-container recipe for phases 21 and 22.
No new operator-visible features.

</domain>

<decisions>
## Implementation Decisions

### Container architecture
- **D-01:** Measure under **linux/amd64 via emulation** only (target arch of the project). No arm64 counter-run. The emulation caveat (JIT/mprotect/seccomp behaviour may differ from real hardware) MUST be named in the findings, together with other container-vs-server differences.

### Container recipe & hand-over
- **D-02:** Recipe lives in **`deploy/test/`** as a `Dockerfile` plus a `run.sh`. Systemd-capable base image (Debian-style), binary and unit copied in, run with `--privileged --cgroupns=host` (or the minimum that works). `run.sh` builds, starts, waits for systemd, runs the checks. Phases 21 and 22 reuse it with different steps, so keep build/start/exec separable.

### MDWE vs wazero
- **D-03:** **Measure both variants, then decide.** First run the unit unchanged (`MemoryDenyWriteExecute=true`), record the plugin-load result; on failure measure with `MemoryDenyWriteExecute=false`. Adjust the unit only if the evidence forces it. Switching wazero to the interpreter engine is a code change and only a last resort. No pre-emptive removal. Evidence = every shipped plugin (`plugins/bestellung`, `jahreszahl`, `kontaktformular`, `nicht-gefunden`, `suche`) loads and is invoked under the unit.

### Security score
- **D-03a:** Commit the **raw `systemd-analyze security` output** (full table + overall value) to the repo in `deploy/SECURITY-SCORE.md` (or `docs/`), with a section "Container vs. server" naming directives the container does not enforce. **No target value** is imposed; the score is measured, not tuned.

### Claude's Discretion
- Base image choice, exact Docker flags, how readiness is polled, how plugin loading is exercised, file naming of the score document.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Unit and deploy
- `deploy/holzcloud.service` — the hardened unit under test
- `deploy/DEPLOY.md` — install doc; later phases correct it
- `deploy/backup.sh`, `deploy/restore.sh`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer` — reused by phase 22 via the recipe
- `Dockerfile` — existing distroless image (linux/amd64 build flags to copy)

### Planning
- `.planning/REQUIREMENTS.md` — UNIT-01..03; test environment decision (container, deviations named)
- `.planning/ROADMAP.md` §Phase 20 — success criteria
- `.planning/quick/*9c4*/` SUMMARY — origin of the MDWE/wazero suspicion and the unit hardening
- `plugins/README.md` — plugin list and loading

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Root `Dockerfile`: builds the amd64 binary with `CGO_ENABLED=0`, `-trimpath`, ldflags — reuse the build stage.
- Five shipped plugins under `plugins/*` with prebuilt `plugin.wasm`.

### Established Patterns
- Config via `HOLZCLOUD_*` env vars set in the unit; data dir `/opt/holzcloud/data`; port 8080.
- Unit requires mounted `/opt/holzcloud/data` (`RequiresMountsFor`), user `holzcloud`.

### Integration Points
- Host is an arm64 Mac with Docker Desktop (cgroup v2, aarch64): amd64 runs under emulation.
- `deploy/test/` is new; phases 21/22 build on it.

</code_context>

<specifics>
## Specific Ideas

The container differs from a real server (kernel, cgroups, emulated CPU, sandbox directives that
a container may not enforce). Every such difference is named, never hidden.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 20-Hardened unit under real systemd*
*Context gathered: 2026-10-12*
