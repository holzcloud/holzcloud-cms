---
phase: 22-backup-and-restore-measured
plan: 05
subsystem: infra
tags: [backup, restore, systemd, docker, evidence, record, measurement]

requires:
  - phase: 22-backup-and-restore-measured
    provides: "the final evidence set deploy/test/out/amd64-bak-* on commit bf74496 (plan 22-04), 22-FINDINGS.md (plan 22-03), backup-chain (plans 22-01, 22-02)"
provides:
  - "deploy/BACKUP-RUN.md: the permanent record of phase 22, twelve sections, 40 raw appendix blocks proven byte-equal to their scratch files, evidence commit bf74496 named on a line of its own"
  - "deploy/test/README.md: pointer to the record, the one-command block comparison, three lessons of the final set, and the Hand-over to phase 23 (what may be cited, how to rerun, what must not be claimed)"
affects: [23]

actuals:
  tokens: 28000
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "an evidence record whose appendix is appended by a script that copies each scratch file, then proven by an awk block extractor and cmp, rather than retyped"
    - "deviations before results; every Findings quote must be found verbatim in pass-1 output, the findings file, a script or the document as it stood at the phase's start commit"

key-files:
  created:
    - deploy/BACKUP-RUN.md
  modified:
    - deploy/test/README.md

key-decisions:
  - "The Appendix was generated with a scratch-directory shell script that copies each file into a fenced block (no retyping), instead of being written by hand with the Write tool; the body was written with Write and the appendix appended after it"
  - "The extrapolation (about 1.1 s per GiB of media for the backup, 0.6 s per GiB for the restore's copy, on top of a fixed restore cost of about eight seconds) is stated as an estimate with its assumption (copy bound, linear, two single readings, no spread measured), never as a measurement"
  - "Container vs. server holds fifteen rows; the claim that a result carries over is labelled measured, inferred or assumed, and where one aspect had a measured part and an unplayed part it was split into two rows rather than given a mixed label"

patterns-established:
  - "Phase 23 cites the Summary table, the Evidence commit line and the words of summary.txt; the Not tested list is copied into the README hand-over as what it must not claim"

requirements-completed: [BAK-01, BAK-02, BAK-03]

coverage:
  - id: D1
    description: "deploy/BACKUP-RUN.md records, from one clean evidence set against the final texts, that a timer-made backup was restored into a different empty container with working pages, media, login and plugins, what it cost at two sizes, what was fixed on the way and what remains untested"
    requirement: "BAK-01"
    verification:
      - kind: other
        ref: "task 1 verify (structure): record-structure-ok; twelve headings in order, Evidence commit line equals amd64-bak-evidence-commit.txt, 15 container-vs-server rows each measured/inferred/assumed, every POINT value quoted"
        status: pass
      - kind: other
        ref: "task 1 verify (proof of the paste): raw-blocks-match, 40 appendix blocks byte-equal to their scratch files, the twenty-one required files among them"
        status: pass
      - kind: other
        ref: "task 1 verify (findings and secrets): every F row of 22-FINDINGS.md present, 76 backtick quotes of 12 or more characters each found verbatim, no key-shaped string, 24-hex word, Set-Cookie, CSRF cookie value or password-hash prefix, scripts/document/units/run.sh unchanged since bf74496"
        status: pass
    human_judgment: false
  - id: D2
    description: "The measurements and the extrapolation say only what two single emulation readings can support, and Not tested names what the timer drop-in and the container cannot show"
    requirement: "BAK-03"
    verification: []
    human_judgment: true
    rationale: "Whether the estimate is worded conservatively enough, and whether the Container vs. server labels (measured, inferred, assumed) are the right ones, is a reading judgment no test asserts"
  - id: D3
    description: "deploy/test/README.md points to the record, carries the proof command and the hand-over to phase 23; the scripts, DEPLOY.md, the units and run.sh are unchanged since the evidence commit and the gates are clean"
    requirement: "BAK-02"
    verification:
      - kind: other
        ref: "task 2 verify: bash -n of run.sh, backup.sh, restore.sh; README headings and required words; no Go file differs from 5de2b17; git diff --quiet bf74496 HEAD over the frozen files"
        status: pass
      - kind: unit
        ref: "go test ./cmd/holzcloud -run 'Deploy|Caddyfile|Dockerfile' -count=1"
        status: pass
      - kind: other
        ref: "the README's own proof command extracted and run: 40 blocks match their scratch files"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-10-14
status: complete
plan_head_before: 756fedfb3c13f1912c2e71027e220770c65005d8
plan_head_after: 3f82e5803f3ba404a0c0e2289088a30871d1ad09
commits: 2
---

# Phase 22 Plan 05: Write the record of phase 22 and hand over to phase 23 Summary

**deploy/BACKUP-RUN.md records, from the one clean evidence set of commit bf74496, that a backup made by the timer's service restored into a different empty container (pages, media, login, five plugins, 17 of 17 checks, four refusals leaving live data untouched), at 20 MB and 320 MB in about eight seconds each under amd64 emulation, with eleven fixed findings, fifteen container-vs-server rows and thirteen untested items named; all 40 pasted blocks are proven byte-equal to their scratch files, and the README hands over to phase 23.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-10-14T11:05Z
- **Completed:** 2026-10-14T11:13Z
- **Tasks:** 2 of 2
- **Files modified:** 2 (one created, one edited)

## Accomplishments

- **The record.** `deploy/BACKUP-RUN.md` has exactly the twelve required level-two headings in order: Summary (one row per requirement, the platform, date and build, and `Evidence commit: bf74496`), How this was run (pass-1 and final commands with their names, the 238-second chain, the discarded `e4c3eb8` set, the `read -p` fact), Deviations named (transfer, timer, backup-name, fixture, scp, dropin, mount, the amd64 emulation beside the aarch64 host, and the four SCENARIO steps the document does not contain, all before any result), Findings (F-01 to F-11, each with step, the said text, the verbatim pass-1 quote and the visible fix), BAK-01, BAK-02, BAK-03, Measurements, What comes back and what does not, Container vs. server, Not tested and the Appendix.
- **The proof of the paste.** The Appendix holds 40 blocks (19 from the source, 14 from the target, 3 from the bulk source, 4 from the bulk target), appended by a script that copies each scratch file. The block extractor of the plan's verify and `cmp` find every block byte-equal to its scratch file (`raw-blocks-match`), and the README's own command, extracted and run, prints `40 blocks match their scratch files`. The plan's second check also passes: 76 backtick quotes of 12 or more characters in Findings, each found verbatim, no secret-shaped string, and the scripts, document, units and `run.sh` unchanged since the evidence commit.
- **The numbers, as the harness wrote them.** Seed point: backup 20548955 bytes in 0.692 s, restore 8.126 s (verify 0.083, stop-move-copy 0.455, start-to-ready 7.397). Bulk point: 335125851 bytes in 1.015 s, restore 8.296 s (0.088, 0.631, 7.379). The estimate (about 1.1 s per GiB of media for the backup, 0.6 s per GiB for the restore copy) is labelled an estimate, with its assumption and the missing spread.
- **The README hand-over.** The README names the record next to the two earlier ones, carries the one-command block comparison, and has the section `Hand-over to phase 23`: what the beta criteria may cite (the Summary table, the `Evidence commit` line, the words of `summary.txt`), how to rerun (`backup-chain` with its names), and what they must not claim (the real 03:15 window, `Persistent=true`, `RandomizedDelaySec`, the off-site `REMOTE_TARGET` copy, retention pruning, an atomic snapshot, a backup older than the binary, a restore on a real server, native amd64 timing, bit rot).

## Task Commits

1. **Task 1 (tracer): deploy/BACKUP-RUN.md from the final evidence set** - `ebb46d5` (docs). Tracer gate in auto mode: the three `<automated>` checks were re-run end to end before expansion (`record-structure-ok`, `raw-blocks-match`, the findings and secrets check with exit 0) and passed.
2. **Task 2: README pointer, proof command and hand-over, repository gates** - `3f82e58` (docs)

**Plan metadata:** committed with this SUMMARY (docs).

## Files Created/Modified

- `deploy/BACKUP-RUN.md` - the permanent record of phase 22 (new, 105 KB, 40 raw blocks in the Appendix)
- `deploy/test/README.md` - the record named in the opening, the file list and the output files; three lessons added to the Phase 22 section (a discarded set is never patched, the plugins dominate the seed point, `set -u` and a status set only on failure), the proof command, the macOS `sed -i` note, and the `Hand-over to phase 23` section

## Decisions Made

- The Appendix was appended by a script that copies the scratch files rather than written by hand. The plan asks for no retyping and a byte comparison; a script makes the first true by construction and leaves the second as a real check. The body of the record was written with the Write tool and the appendix appended after it, so nothing was overwritten.
- Aspects of Container vs. server that had a measured part and an unplayed part were split into two rows (the timer starting the service, measured, against the real schedule and `Persistent`, assumed; secrets outside every backup, measured, against the consequence of losing `HOLZCLOUD_SECRET_KEY`, assumed), so each row carries exactly one label.
- The three tree lists (`tree-src-backup.txt`, `tree-src-timer.txt`, `tree-restored.txt`) were pasted as well, since BAK-01 rests on the whole-directory comparison; no `step-NN`, seed, log, cookie or password file was pasted.

## Deviations from Plan

None - plan executed exactly as written. (The appendix was appended with a script, which the plan's "no retyping" requirement favours; it is listed under Decisions, not as a departure.)

## Issues Encountered

None. The first run of all three automated checks of Task 1 passed; no quote had to be reworded.

## Authentication Gates

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, auth path, file access pattern or schema change; only documentation. T-22-18 (a secret pasted into the committed record) is mitigated as planned: the pasted files and the body were scanned for key-shaped strings, 24-character hex words, `Set-Cookie`, CSRF cookie values and password-hash prefixes, and none is present; the sha256 of `csrf.key` in the hash lists belongs to a throwaway container. T-22-19 (a block that differs from its scratch file, a quote found nowhere) is mitigated by the byte comparison of 40 blocks and the verbatim search of 76 quotes. T-22-20 (the record describing other texts than the ones evidenced) is mitigated by the `Evidence commit` line, equal to `amd64-bak-evidence-commit.txt`, and by `git diff --quiet bf74496 HEAD` over the scripts, DEPLOY.md, the units and `run.sh` exiting 0. T-22-SC is accepted: no package was installed.

## Next Phase Readiness

Phase 23 can cite `deploy/BACKUP-RUN.md` (Summary table, `Evidence commit: bf74496`, the words of `summary.txt`), rerun the proof with the one `backup-chain` command in `deploy/test/README.md`, and knows from the same README what the record does not claim. Constraint it inherits: a change to `deploy/DEPLOY.md`, `backup.sh`, `restore.sh`, the two backup units or `run.sh` after `bf74496` means the evidence set is made again before the record is cited for the changed text. This is the last plan of phase 22.

## Self-Check: PASSED

- Created/modified files exist: deploy/BACKUP-RUN.md, deploy/test/README.md, deploy/test/out/amd64-bak-evidence-commit.txt (`bf74496`).
- Commits `ebb46d5` and `3f82e58` are ancestors of HEAD; `git rev-list --count 756fedf..HEAD` is 2 before this SUMMARY's own commit.
- Task 1 verifies re-run after the commit: `record-structure-ok`, `raw-blocks-match` and the findings-and-secrets check (exit 0); Task 2 verify re-run: README gates, `bash -n` of the three scripts, no Go file differs from 5de2b17, `go test ./cmd/holzcloud -run 'Deploy|Caddyfile|Dockerfile'` ok, `git diff --quiet bf74496 HEAD` over the frozen files exits 0.
