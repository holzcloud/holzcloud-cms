---
phase: 22-backup-and-restore-measured
plan: 03
subsystem: infra
tags: [backup, restore, findings, sqlite, systemd, docker, measurement]

requires:
  - phase: 22-backup-and-restore-measured
    provides: "run.sh backup-chain: the whole proof from fresh containers, summary.txt in a fixed vocabulary (plans 22-01 and 22-02)"
provides:
  - "22-FINDINGS.md: eleven findings F-01 to F-11, each with step, quote, planned fix and one disposition"
  - "D-10 decided: the restore tells a damaged database from a whole one, and cannot tell an interrupted backup (no MANIFEST) from a complete one; the minimal fix needs no new field"
  - "A1, A2, A3 and A6 confirmed, A5 unchanged (reasoning), each with its evidence file"
  - "pass-1 numbers at both data points with the emulation named, and a per-MiB estimate"
  - "scratch evidence deploy/test/out/amd64-bak-pass1-* and amd64-bak-inv3-* for plans 22-04 and 22-05"
affects: [22-04, 22-05]

actuals:
  tokens: 6795
  tasks: 2
  commits: 1

tech-stack:
  added: []
  patterns:
    - "a question that the played files cannot answer is asked in a second, kept chain under other names (inv3), so the pass-1 directories stay exactly what the chain wrote"

key-files:
  created:
    - .planning/phases/22-backup-and-restore-measured/22-FINDINGS.md
  modified: []

key-decisions:
  - "D-10: no new MANIFEST field or checksum file. restore.sh refuses a directory without MANIFEST, which backup.sh writes last; this covers an interrupted backup and not bit rot in a complete one"
  - "F-08 (restored database mode 644 against 600) is a one-line fix script; F-09 (the set-aside directory is never removed) and the MIN_FREE_MB floor are document sentences, not script changes"
  - "No harness repair was needed: the chain ran to summary.txt on the first run, so pass 1 is the only pass-1 run"

requirements-completed: [BAK-01, BAK-02, BAK-03]

coverage:
  - id: D1
    description: "The unchanged scripts and document were played end to end once from three fresh containers at both data points (BULK_MB 300), with the raw evidence on disk and the specimen byte-identical to commit 5de2b17 before and after"
    requirement: "BAK-01"
    verification:
      - kind: integration
        ref: "task 1 verify: git diff --quiet 5de2b17 HEAD and HEAD over the six deploy files; summary.txt with BAK-01 FAIL, BAK-02 FAIL, BAK-03 PASS, RESTORE-NEGATIVE PASS, both POINT lines, LEAK-CHECK OK; the 15 named evidence files non-empty; no holzcloud-bak-pass1 container; no AI-key-shaped string"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every BAD line of pass 1 is pasted in the findings and each belongs to a row with exactly one disposition; the D-16 candidates, D-10 and the assumptions A1, A2, A3, A5, A6 are each decided from evidence"
    requirement: "BAK-02"
    verification:
      - kind: other
        ref: "task 2 verify: seven headings in order, 11 F rows each ending in one of the five dispositions, the named candidates and assumptions present, both POINT lines, estimate and emulat, every distinct BAD line of the four directories found in the file, no key-shaped string, no 24-character hex word, specimen unchanged"
        status: pass
    human_judgment: true
    rationale: "Whether each disposition is the right call (F-08 and F-09 especially) and whether the D-10 verdict is the right trade is a judgment the repair plan and the owner confirm"
  - id: D3
    description: "The numbers DEPLOY.md will quote (plan 22-04) exist with the emulation named: backup under a second and restore under two seconds at 300 MiB, roughly one second per GiB, estimate"
    requirement: "BAK-03"
    verification:
      - kind: integration
        ref: "summary.txt POINT seed and POINT bulk of amd64-bak-pass1-src"
        status: pass
    human_judgment: true
    rationale: "Two single readings under an amd64 emulation: an order of magnitude, not a benchmark"

duration: 14min
completed: 2026-10-14
status: complete
plan_head_before: b687738222385d31bd4904c96f5a3e67b045da3d
plan_head_after: 02364738dcd1dba4e8ac3838c39bd7478f1b1154
commits: 1
---

# Phase 22 Plan 03: Pass 1 on the unchanged scripts, findings and numbers Summary

**The unchanged backup.sh, restore.sh and DEPLOY.md were played end to end once from three fresh containers at both data points; 22-FINDINGS.md records eleven differences between what they say and what happened, each with its fix decided, D-10 decided from the damaged-backup case, and the numbers DEPLOY.md will quote (backup under a second and restore under two at 300 MiB, about one second per GiB, on an amd64 emulation).**

## Performance

- **Duration:** about 14 min (the chain itself took 249 s, the investigation chain about as long)
- **Started:** 2026-10-14T10:31:41Z
- **Completed:** 2026-10-14T10:45Z
- **Tasks:** 2 of 2 (task 1 changed no file, so it has no commit of its own)
- **Files:** 1 created (22-FINDINGS.md, 163 lines); deploy/test/run.sh untouched

## Accomplishments

- **Pass 1, once, unrepaired.** `backup-chain` with `NEW_REF=b687738`, `BULK_MB` at its default 300, exit 1 with `BAK-01 FAIL`, `BAK-02 FAIL`, `BAK-03 PASS`, `RESTORE-NEGATIVE PASS`, `STEPS: 76 played, 0 failed`, `LEAK-CHECK OK`. The six specimen files equal commit 5de2b17 and HEAD before and after. No harness defect stopped the chain, so no repair and no second pass-1 run.
- **One defect class, no others in the BAD lines.** 62 distinct BAD lines, all from F-01 (backup.sh copies four of the five directories: the plugin tree, `sprachen/nl.json` and `marke/logo.svg` are only in the source), F-02 (restore.sh has the same four) and F-03 (the restored site: five plugins enabled and not running, 1 of 17 `check` assertions, while `/readyz` says ok).
- **Not only BAD lines.** Eight more rows come from the comparison of what the script and the document say with what happened: the docs that claim a whole-directory backup (F-04), the DEPLOY.md paragraphs (F-05, F-06), the prompt that is a harness fact (F-07), the restored database at mode 644 against 600 (F-08), set-aside directories that stay and hold the media (F-09), a restore that accepts a directory with no MANIFEST and a missing file (F-10), the README claim about deviation names that the harness never checks (F-11).
- **D-10 decided.** Truncation of the database is refused by the integrity check before the live data is touched (case a). A flipped byte inside a data page is not detected by either reader (E2). An interrupted backup (no MANIFEST, a media file missing) restores with exit 0 and loses the file (E3b). Fix without a new field: refuse a directory without MANIFEST, which backup.sh writes last.
- **Assumptions settled.** A1, A2, A3 and A6 confirmed with files, A5 unchanged and labelled reasoning.
- **Numbers.** seed 715976 bytes, backup 0.509 s, restore 1.651 s; bulk 315292872 bytes, backup 0.722 s, restore 1.902 s, start to ready 1.150 s and 1.162 s. Estimate from the difference: about 0.7 ms per MiB of backup and 0.8 ms per MiB of restore, roughly a second per GiB, good to a factor of two.

## Task Commits

1. **Task 1 (tracer): pass 1, the whole chain against the unchanged specimen** - no commit (evidence is gitignored scratch; run.sh unchanged). Tracer gate in auto mode: the verify passed end to end, so expansion went ahead.
2. **Task 2: 22-FINDINGS.md** - `0236473` (docs)

**Plan metadata:** committed with this SUMMARY (docs).

## Files Created/Modified

- `.planning/phases/22-backup-and-restore-measured/22-FINDINGS.md` - the seven sections the plan names, in order.
- Scratch, gitignored: `deploy/test/out/amd64-bak-pass1.log`, `amd64-bak-pass1-src`, `-tgt`, `-src-bulk`, `-tgt-bulk`; and the investigation run `amd64-bak-inv3.log`, `amd64-bak-inv3-src`, `-tgt`, `-src-bulk`, `-tgt-bulk`, `amd64-bak-inv3-investigation.txt` (E1 to E5).

## Decisions Made

- D-10: restore.sh refuses a directory without MANIFEST; no new field, no digest file. Bit rot in a complete backup stays uncovered by decision.
- The `MIN_FREE_MB` floor and the non-atomic pair become sentences in DEPLOY.md and the script headers, not script changes (no new option is allowed).
- F-08 is a script fix (one `chmod 600`), F-09 a document fix.

## Deviations from Plan

### Plan-text departures (named, each small)

- **A second chain, `amd64-bak-inv3-*`, for questions pass 1 could not answer.** The plan allows a rerun with `KEEP=1` and `run.sh exec` for investigations. It was run with `BULK_MB=5` under other names and directories, so the pass-1 directories are exactly what the first chain wrote, and its containers were removed with `run.sh down` (one call per name). The investigation file is scratch like the rest; the findings quote it by E-number, and plan 22-05 must treat it as evidence of the same kind.
- **E3 first run was a no-op.** It removed a `bulk` directory that the seed target does not have; E3b repeated it with a real media file removed. Both are in the investigation file, unedited.

**Total deviations:** none under Rules 1 to 4. **Impact:** none on scope.

## Findings for later plans

- **For 22-04:** the repair list is `fix script` rows F-01, F-02, F-03 (closes with the first two), F-08, F-10; `fix document` rows F-04, F-05, F-06, F-09; `fix harness` row F-11; `no action` row F-07. Two things read from the scripts and not played are listed under the table in the findings: the "test directory" advice of DEPLOY.md (restore.sh stops the production unit whatever DATA_DIR says) and the fixed `MIN_FREE_MB` floor.
- **Interaction with the harness:** F-10's repair (refuse a directory without MANIFEST) must not break `restore-negative` case b (a directory with a MANIFEST and no database is still refused for the missing database) or the documented restore line; the harness's own backups always carry a MANIFEST.
- **For 22-05:** the quotes in the table are verbatim from the scratch files, except the paraphrase-free E-number quotes, which are in `amd64-bak-inv3-investigation.txt`. The BAD block of the findings is the pass-1 set, and it will not be the final evidence set (the final set must have none).
- **Pre-existing, out of scope (unchanged):** `deferred-items.md` still lists the STATE.md position ambiguity, the shared requirement IDs and the README deviation claim (now also F-11).

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path or trust boundary: the plan added one document of counts, hashes of content and version strings. `leak_check` passed in both chains, the findings and the investigation file hold no `hc_`-shaped string and no 24-character hex word.

## Next Phase Readiness

Plan 22-04 can repair backup.sh, restore.sh, DEPLOY.md and the two docs from a written list, watch `BAK-01` and `BAK-02` turn PASS with the same command, and quote the numbers from the findings.

## Self-Check: PASSED

- `22-FINDINGS.md` exists, has the seven headings, 11 F rows, 62 BAD lines pasted; `deploy/test/out/amd64-bak-pass1-src/summary.txt` and the other three directories exist.
- Commit `0236473` is an ancestor of HEAD.
- `git diff --quiet 5de2b17 HEAD` and `git diff --quiet HEAD` over backup.sh, restore.sh, DEPLOY.md and the three unit files exit 0; no `holzcloud-bak` container remains.
