---
phase: "23"
slug: "repository-tidy-and-beta-criteria"
status: verified
threats_open: 0
asvs_level: 1
created: "2026-10-15"
---

# Phase 23 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Docs-only phase: no Go file, template, catalogue, go.mod or go.sum changed. Mitigations were checked at ASVS L1 depth against git on 2026-10-15.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| records to docs/beta.md | claims must equal the Phase 22 records | public documentation |
| origin/main to local main | upstream README rewrite must survive the merge | repository content |
| CHANGELOG.md to release.yml and admin screen | heading shape decides tag and notes; text is rendered through goldmark + bluemonday | Markdown |
| remote branches to docs/branches.md | table must equal git | branch metadata |
| agent session to remote | agent pushes no tag and deletes nothing | git refs |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-23-01 | Repudiation | docs/beta.md promises vs records | high | mitigate | cited-words loop (plan 23-01 verify), evidence commits named | closed |
| T-23-02 | Tampering | six evidence-set files | high | mitigate | `git diff --quiet bf74496` over the six files: clean on 2026-10-15 | closed |
| T-23-03 | Tampering | merge of origin/main | medium | mitigate | origin/main is an ancestor of HEAD (verified) | closed |
| T-23-04 | Tampering | tag or push from agent | high | mitigate | no local v0.1.0 tag, none on origin, no `git tag` in 23-OWNER-STEPS.md | closed |
| T-23-05 | Repudiation | CHANGELOG/README over-promise | high | mitigate | per-fact verify commands in plan 23-02 | closed |
| T-23-06 | Tampering | release.yml parse of entry | medium | mitigate | first heading is `## 0.1.0 — 2026-10-14`, single section | closed |
| T-23-07 | Tampering | raw HTML in entry | low | mitigate | no HTML tag in the 0.1.0 section (verified) | closed |
| T-23-08 | Information Disclosure | beta in README vs alpha in admin | low | accept | owner assumption, see log | closed |
| T-23-09 | Elevation of Privilege | tagfix, backfill-releases workflows | high | mitigate | both marked Review first in docs/branches.md; nothing deleted or pushed | closed |
| T-23-10 | Repudiation | stale branch table | medium | mitigate | generated from git with date and SHA; Review-first rows present (table not recomputed in this audit) | closed |
| T-23-11 | Tampering | branch deleted under active session | medium | mitigate | verdicts from patch-equivalence; Tip column enables restore | closed |
| T-23-12 | Repudiation | evidence-set drift at end state | high | mitigate | same diff as T-23-02, clean | closed |
| T-23-13 | Tampering | i18n gate broken | medium | mitigate | no Go, template or catalogue change since 9db196b | closed |
| T-23-14 | Elevation of Privilege | push order / tag from agent | medium | mitigate | 23-OWNER-STEPS.md: `push origin main` (line 14) before `main:release` (line 22); no tag command | closed |
| T-23-SC | Tampering | package installs | high | accept | none in any plan; go.mod and go.sum unchanged | closed |

*Status: open · closed · open — below high threshold (non-blocking)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-23-01 | T-23-08 | the product's alpha label stays; docs/beta.md explains it | plan 23-02 (owner may veto) | 2026-10-15 |
| AR-23-02 | T-23-SC | no package-manager install in the phase | plans 23-01..23-04 | 2026-10-15 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-15 | 15 | 15 | 0 | Claude (inline L1 check, no auditor agent) |

Note: UAT test 1 (v0.1.0 on origin) is still pending; T-23-04 and T-23-14 hold for the agent session and do not depend on it.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-15
