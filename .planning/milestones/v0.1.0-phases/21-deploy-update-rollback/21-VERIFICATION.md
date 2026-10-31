---
phase: 21-deploy-update-rollback
verified: 2026-10-12T15:30:00Z
status: passed
score: 3/3 must-haves verified
covered_files:
  - .planning/phases/21-deploy-update-rollback/21-01-PLAN.md
  - .planning/phases/21-deploy-update-rollback/21-01-SUMMARY.md
  - .planning/phases/21-deploy-update-rollback/21-02-PLAN.md
  - .planning/phases/21-deploy-update-rollback/21-02-SUMMARY.md
  - .planning/phases/21-deploy-update-rollback/21-03-PLAN.md
  - .planning/phases/21-deploy-update-rollback/21-03-SUMMARY.md
  - .planning/phases/21-deploy-update-rollback/21-04-PLAN.md
  - .planning/phases/21-deploy-update-rollback/21-04-SUMMARY.md
  - .planning/phases/21-deploy-update-rollback/21-05-PLAN.md
  - .planning/phases/21-deploy-update-rollback/21-05-SUMMARY.md
  - .planning/phases/21-deploy-update-rollback/21-06-PLAN.md
  - .planning/phases/21-deploy-update-rollback/21-06-SUMMARY.md
  - deploy/DEPLOY-RUN.md
  - deploy/DEPLOY.md
  - deploy/test/README.md
  - deploy/test/run.sh
  - docs/deployment.md
covered_digest: "v3:sha256:043532dd1b39ee35532928ac7146230920eed3f0711b630800c6cfca256569ec"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 21: Deploy, update, rollback Verification Report

**Phase Goal:** Install, update and rollback are played through and the docs match what happened.
**Verified:** 2026-10-12
**Status:** passed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths (ROADMAP success criteria, merged with plan must-haves)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A fresh install by `deploy/DEPLOY.md` alone works; every deviation found is fixed in the doc (DEP-01) | VERIFIED | `deploy/test/out/amd64-fresh/summary.txt`: `DEP-01 PASS`, `STEPS: 33 played, 0 failed`, `CADDY: validated`, `LEAK-CHECK OK`. `deviations.txt` holds only the named set (scp x2, dropin, mount, browser, caddy). Pass-1 findings (ten defects) have fix rows in DEPLOY-RUN.md Findings, and the fixes are in DEPLOY.md (apt update, gnupg/psmisc/openssl prerequisites, version stamping in Build, `/opt/holzcloud/deploy` copy and chmod, chown of data and backups, First Run TLS/two-factor text). |
| 2 | An update across at least one migration keeps all data (DEP-02) | VERIFIED | `amd64-lifecycle`: `MIGRATIONS: old=58 new=60 pending=2`, `SCHEMA: 58 -> 60`, `COMPARE: pass=52 bad=0`, `ATTACH: pass=50 bad=0` against the service's own snapshot, journal shows `pre-upgrade snapshot written` then `migration applied` 59 and 60. I re-counted PASS/BAD in `compare-s0-s1.txt` (52/0) and `attach-s1.txt` (50/0). The comparison can fail: `compare-selftest.txt` shows 2 BAD on a tampered copy and a SELFTEST OK line. |
| 3 | A rollback is played and documented, with the migration limit stated (DEP-03) | VERIFIED | DEPLOY.md `## Rolling back` (line 724) opens with the limit in bold. Snapshot mode (`amd64-lifecycle/rollback.txt`): old version back, schema 58, marker page gone, owner/mode `holzcloud 600`, no migration on restart, `compare-s0-s2` 52 PASS / 0 BAD. Binary mode (`amd64-lifecycle-hop`, v0.0.21 to HEAD, same schema 60, no snapshot): `DEP-03 PASS`, 53 PASS / 0 BAD. Negative case observed and documented: `NEGATIVE-RESULT: starts`, then `UNIQUE constraint failed: orders.token` at the second order. Rollback steps are played from the document's own blocks (`doc_play "Rolling back" 1|2`). Recovery "A failed migration" is cross-linked. |
| 4 | The docs match what happened: DEPLOY-RUN.md record complete, evidence pasted verbatim | VERIFIED | I independently parsed all 29 appendix blocks of `deploy/DEPLOY-RUN.md` and compared each against `deploy/test/out/...`: 29 files, 0 differing. Searched the record for secrets: only content sha256 hashes appear. |
| 5 | Evidence was made against the final document | VERIFIED | `git diff 2a11eeb HEAD -- deploy/DEPLOY.md` is empty; `deploy/test/run.sh` changed only by the chains commit 2a40bd3, which is what produced the evidence. Evidence summaries show build `v0.0.25-42-g2a11eeb`. |
| 6 | `docs/deployment.md` and `deploy/test/README.md` agree with DEPLOY.md | VERIFIED | `docs/deployment.md` uses `/opt/holzcloud/deploy`, `holzcloud.prev`, links Updating and Rolling back. README documents `fresh`, `lifecycle`, `OLD_TAG`, `INSTALL_FROM`, `scenario.txt`, `summary.txt` and a hand-over to phase 22. |
| 7 | No Go source changed, no operator-visible text changed, gates pass | VERIFIED | `git diff 783027b HEAD -- '*.go'` is empty; `go vet ./...` clean, `gofmt -l` clean, `go run ./tools/i18n` reports 0 open, 0 orphaned; `bash -n deploy/test/run.sh` ok; full `go test ./...` reported green by the caller. |

**Score:** 3/3 roadmap success criteria verified (7/7 including plan-level truths), 0 behavior-unverified.

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `deploy/test/run.sh` | VERIFIED | 2469 lines; `doc_play`, `cmd_update`, `cmd_rollback`, `cmd_lifecycle`, `compare_states`, chains `fresh` and `lifecycle` present and exercised by the evidence. |
| `deploy/DEPLOY.md` | VERIFIED | `## Rolling back`, `holzcloud.prev`, `two-factor` present; Updating rewritten (back-up-first, journal check). |
| `deploy/DEPLOY-RUN.md` | VERIFIED | 1420 lines; Summary, How this was run, Deviations, Findings, DEP-01/02/03 sections, Container vs. server, Not measured, Appendix. |
| `deploy/test/README.md` | VERIFIED | Subcommands, variables, output files, hand-over to phase 22. |
| `docs/deployment.md` | VERIFIED | Consistent with DEPLOY.md paths. |

### Key Link Verification

| From | To | Status | Details |
|------|----|--------|---------|
| run.sh | DEPLOY.md | WIRED | `doc_play "Install" / "First Run" / "Updating" / "Rolling back" 1|2` at lines 1163, 1305, 1988, 2118, 2120. |
| DEPLOY.md Updating and Recovery | Rolling back | WIRED | `(#rolling-back)` links present in both. |
| DEPLOY-RUN.md Appendix | scratch evidence | WIRED | 29/29 blocks match byte for byte. |
| README | run.sh | WIRED | `fresh`, `lifecycle`, `negative`, `rollback` documented. |

### Requirements Coverage

| Requirement | Source Plans | Status | Evidence |
|-------------|--------------|--------|----------|
| DEP-01 | 21-01, 21-02, 21-06 | SATISFIED | `fresh` chain `DEP-01 PASS`; findings fixed in DEPLOY.md. |
| DEP-02 | 21-02, 21-03, 21-06 | SATISFIED | 58 to 60 migrations, 52/0 and 50/0 comparisons. |
| DEP-03 | 21-04, 21-05, 21-06 | SATISFIED | Rolling back section, snapshot and binary modes played, negative case observed, limit stated first. |

REQUIREMENTS.md maps exactly DEP-01 to DEP-03 to phase 21 (lines 54-56); all three are claimed by plans. No orphaned requirements.

### Anti-Patterns Found

No TBD/FIXME/XXX debt markers introduced; no Go files touched. Observations only (Info):

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `deploy/test/run.sh` `steps_failed_since` | Excludes every `systemctl status` step from the failure count | Info | Step 19 of every run ends `rc=3` (the emulated first start crashes, the known `dropin` deviation), so the exclusion is what lets `0 failed` hold. It is justified and the following readiness `curl` step would still catch a really dead service, but a literal reader of `0 failed` should know one step has rc=3. |

### Behavioral Spot-Checks

Re-counted PASS/BAD from the committed-locally evidence files (no containers started): fresh, lifecycle and hop summaries read as claimed; selftest proves the comparison can fail. `bash -n` on run.sh passes.

### Probe Execution

Step 7c: SKIPPED. The phase's probes are the container chains (`fresh`, `lifecycle`), which need Docker with amd64 emulation and minutes of runtime; their recorded output was checked instead.

### Human Verification Required

None blocking. Context for the owner (not gaps): the evidence comes from an emulated amd64 container, not a real server. DEPLOY-RUN.md names every difference under *Container vs. server* and *Not measured* (e.g. real mount, ACME/Caddy reload, timer firing, failed-migration path read from code only).

### Gaps Summary

None. The goal, "install, update and rollback are played through and the docs match what happened", is backed by raw evidence that I re-read and re-counted, by an appendix proven identical to that evidence, and by a DEPLOY.md that was not edited after the final evidence set was made.

---

_Verified: 2026-10-12_
_Verifier: Claude (gsd-verifier)_
