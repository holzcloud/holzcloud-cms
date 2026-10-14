---
phase: 23-repository-tidy-and-beta-criteria
verified: 2026-10-14T14:40:00Z
status: human_needed
score: 3/4 must-haves verified
covered_files:
  - .planning/phases/23-repository-tidy-and-beta-criteria/23-01-PLAN.md
  - .planning/phases/23-repository-tidy-and-beta-criteria/23-01-SUMMARY.md
  - .planning/phases/23-repository-tidy-and-beta-criteria/23-02-PLAN.md
  - .planning/phases/23-repository-tidy-and-beta-criteria/23-02-SUMMARY.md
  - .planning/phases/23-repository-tidy-and-beta-criteria/23-03-PLAN.md
  - .planning/phases/23-repository-tidy-and-beta-criteria/23-03-SUMMARY.md
  - .planning/phases/23-repository-tidy-and-beta-criteria/23-04-PLAN.md
  - .planning/phases/23-repository-tidy-and-beta-criteria/23-04-SUMMARY.md
  - CHANGELOG.md
  - README.md
  - docs/beta.md
  - docs/branches.md
  - docs/deployment.md
covered_digest: "v3:sha256:8d21275fb57b56fd380a227303e836098c71022f0d6663d7719fb646fff2057c"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Push main, then push main:release (or Actions, Release, Run workflow), as written in 23-OWNER-STEPS.md sections 1 and 2"
    expected: "git ls-remote --tags origin v0.1.0 returns a tag and the GitHub release page carries the 0.1.0 CHANGELOG section as its body"
    why_human: "ROADMAP criterion 2 says 'the tag is set'. v0.1.0 exists neither locally nor on origin. The development proxy refuses refs/tags, and CONTEXT D-09 assigns the tag to the owner. The release input (heading, notes) is verified locally; the tag itself cannot be."
---

# Phase 23: Repository tidy and beta criteria Verification Report

**Phase Goal:** 0.1.0 says what it promises and the repository carries nothing stray.
**Verified:** 2026-10-14
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Beta criteria are written down | VERIFIED | `docs/beta.md` (93 lines) has exactly the five required level-two headings in order. Ten promise rows P-1..P-10: I re-ran a byte-exact check of every backticked cited word against the single record named in each row (SECURITY-SCORE / DEPLOY-RUN / BACKUP-RUN); 0 missing. Non-promises name 03:15, Persistent=true, RandomizedDelaySec, REMOTE_TARGET, retention, atomic snapshot, older-than-binary, real server, native amd64, bit rot, seccomp, Not measured. States the schema is not promised stable and that an older binary must not run against a migrated database. Binds to f1428d2 / 2a11eeb / bf74496. Notes where the word alpha remains (`IsAlpha`). |
| 2 | CHANGELOG and release notes for 0.1.0 exist and the tag is set | PARTIAL: content VERIFIED, tag is a human step | CHANGELOG first version heading is `## 0.1.0 — 2026-10-14` above 0.0.25. release.yml's own grep and awk, run locally, give version 0.1.0 and a 4676-byte notes section containing the absolute link to docs/beta.md and to deploy/DEPLOY.md, with no second `##` heading, no raw HTML and no relative link. The upgrade note for operators on 0.0.25 is in the entry. README badge is `status-beta`, the notice links docs/beta.md, and a Documentation bullet exists. docs/deployment.md Versioning says beta. Facts in the entry hold: last migration is 00060, no file outside planning/deploy/docs/CHANGELOG/README/.gitignore differs from v0.0.25, deploy/holzcloud.service changed in comment lines only. **The tag `v0.1.0` does not exist locally or on origin.** D-09 deliberately leaves this to the owner, since the proxy refuses tags. |
| 3 | Orphan branches are listed with GitHub-UI deletion steps | VERIFIED | `docs/branches.md` lists all 10 remote branches. I recomputed against the live remote: tip SHA, only-in-main/only-in-branch counts and git cherry evidence match every row. Verdicts follow the stated rule: `release` is Keep (route 2 of release.yml), `backfill-releases` and `tagfix` are Review first (cherry `+`), the other seven are Safe to delete (ancestor or all `-`). It has the UI steps (branch dropdown, View all branches, trash icon; PR Delete branch route), the proxy-403 note, the restore routes with route B marked unverified, and the push-triggered workflow facts for the two Review first branches. |
| 4 | `go run ./tools/i18n` reports `0 open, 0 orphaned` | VERIFIED | Run now: de, es, fr, it all `2056 translated, 0 open, 0 orphaned`. `tools/themewords -check` and `tools/english` exit 0. `go test ./...` has no failures. |

**Score:** 3/4 truths fully verified; truth 2 is verified except for the tag, which only the owner can create.

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `docs/beta.md` | VERIFIED | Substantive and cited; not a stub |
| `CHANGELOG.md` | VERIFIED | 0.1.0 entry complete, German prose, works as release input |
| `README.md`, `docs/deployment.md` | VERIFIED | Beta badge, notice, bullet, Versioning section |
| `docs/branches.md` | VERIFIED | Matches git, see above |
| `23-OWNER-STEPS.md` | VERIFIED | Ordered push, release, check, tidy steps; veto points; `git push origin main:release`; no command creates a tag |

### Key Link Verification

| From | To | Status |
|------|----|--------|
| beta.md promise rows | the three records | WIRED (grep -F check, 0 missing) |
| CHANGELOG 0.1.0 | docs/beta.md and deploy/DEPLOY.md | WIRED (absolute blob/main URLs) |
| CHANGELOG first heading | release.yml | WIRED (snippets yield 0.1.0) |
| branches.md `release` row | release.yml route 2 | WIRED |

### Requirements Coverage

| Requirement | Plans | Status | Evidence |
|-------------|-------|--------|----------|
| REPO-01 | 23-01, 23-02 | SATISFIED | docs/beta.md, README notice and bullet |
| REPO-02 | 23-01, 23-02, 23-04 | SATISFIED for entry and notes; tag NEEDS HUMAN | CHANGELOG entry; v0.1.0 tag absent by design (D-09) |
| REPO-03 | 23-03, 23-04 | SATISFIED | docs/branches.md matches git |
| QUAL-01 | 23-01, 23-04 | SATISFIED | i18n gate 0 open, 0 orphaned on four languages |

No orphaned requirements: REQUIREMENTS.md maps exactly REPO-01, REPO-02, REPO-03 and QUAL-01 to Phase 23, and all four are claimed by plans.

### Constraint Checks

| Check | Result |
|-------|--------|
| `git diff --quiet bf74496` over the six Phase 22 evidence files | exit 0 |
| Go files, go.mod, go.sum, .github, internal differ from 9db196b | no diff |
| Anything outside .planning, deploy, docs, CHANGELOG.md, README.md, .gitignore differs from v0.0.25 | no diff |
| Docs, CHANGELOG, README clean in the working tree | yes |
| Nothing pushed, no tag, no branch deleted | confirmed: origin main is still 9cc7bcb, no v0.1.0 tag |

### Anti-Patterns Found

None. Docs-only phase; no TBD/FIXME markers introduced in the covered files. Note that the README badge image URL is a README-only shields.io link that predates this phase and is not served by the application, so the CLAUDE.md runtime-fetch rule does not apply.

### Human Verification Required

**1. Publish and tag 0.1.0**

**Test:** Follow `23-OWNER-STEPS.md`: `git push origin main`, then `git push origin main:release` (or run the Release workflow from Actions), then `git ls-remote --tags origin v0.1.0`.
**Expected:** Tag `v0.1.0` exists on origin; the release page body is the 0.1.0 CHANGELOG section.
**Why human:** The roadmap wording "the tag is set" cannot be met by an agent here, since the proxy refuses `refs/tags` and D-09 reserves this step to the owner. The workflow input is proven locally.

**2. Branch deletion (optional follow-through)**

REPO-03 only requires the list and the instructions, which exist. Deleting the seven Safe-to-delete branches is the owner's step (section 5 of OWNER-STEPS); the two Review first branches carry push-triggered workflows and need the owner's decision.

### Gaps Summary

No code or documentation gaps. Everything the phase could deliver is present and matches the repository and the live remote. The one open item is the tag push, which is a recorded human step. Status is `human_needed` rather than `passed` for that reason only.

---

_Verified: 2026-10-14_
_Verifier: Claude (gsd-verifier)_
