---
phase: "24"
slug: "the-backup-timer-as-shipped"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-10-31"
---

# Phase 24 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| host shell to Docker daemon | nightly.sh drives a privileged, throwaway container; no port published, no host path mounted | commands, evidence files |
| container clock to shared VM kernel clock | a clock setter in a privileged container would move the clock of every container on the daemon | system time |
| scratch evidence to committed record | files under deploy/test/out are pasted into a record that is read as proof | evidence text (must hold no secret) |
| source container to second host | ssh and rsync channel between two throwaway containers on one bridge network | backup copies, public key |
| service user to key material | a private key lives on the source for the duration of the run | ssh private key |
| prune test to the real backups | rm -rf of planted directories under /opt/holzcloud/backups, inside the container only | backup directories |
| executor to shipped files | a fix could edit a hardened unit or the backup scripts | systemd units, backup.sh |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-24-01 | Tampering | clock of the Docker VM | medium | mitigate | Zone moved by files (`move_zone`, nightly.sh:1247-1262), no clock-setting command anywhere; CLOCK-SKEW-S recorded before/after and held within ±120 s (nightly.sh:1618, 2120) | closed |
| T-24-02 | Repudiation | a verdict typed instead of measured | high | mitigate | Pure verdict functions (`verdict_of`, `req_verdict`, `fire_window_ok`, `catchup_ok`, `prune_expect`); selftest with failing cases (646 green); byte-for-byte record paste | closed |
| T-24-03 | Information disclosure | password, AI key, private key in scratch output | high | mitigate | `leak_check` (nightly.sh:1176-1203) at the end of every subcommand over OUT_DIR and TGT_OUT_DIR | closed |
| T-24-04 | Elevation of privilege | privileged container with systemd as PID 1 | low | accept | Inherited from phase 20 harness; only `docker run` is run.sh:545, with no `-p` and no `-v` | closed |
| T-24-05 | Tampering | control units or moved zone left behind | medium | mitigate | `catchup_restore` idempotent, EXIT trap armed before the first change, `ZONE-RESTORED: yes` recorded | closed |
| T-24-06 | Denial of service | unbounded wait | medium | mitigate | Every poll bounded (FIRE_WAIT, CATCHUP_WAIT, 30 s re-arm, 120 s boot) and dies with evidence | closed |
| T-24-07 | Information disclosure | private ssh key on the source | high | mitigate | Generated in-container, mode 600, only the public key crosses by exec-pipe; `leak_ssh` and `leak_check` | closed |
| T-24-08 | Spoofing | host key taken by ssh-keyscan on first use | low | accept | Two throwaway containers on one bridge; `DEVIATION[credentials]`; real-host fingerprint compare is owner step O-3 | closed |
| T-24-09 | Tampering | prune test removes a backup that matters | medium | mitigate | Plants only under the container's backups root; MUST-STAY set asserted present afterwards (nightly.sh:2305-2318) | closed |
| T-24-10 | Tampering | rsync --delete mirrors deletions off-site | medium | accept | No feature built, backup.sh not edited in phase 24; behaviour observed (INFO MIRROR, finding F-4) and stated in DEPLOY.md:701 | closed |
| T-24-11 | Information disclosure | copied backups readable by others on host 2 | high | mitigate | Every copied directory asserted owner offsite, mode 700 (nightly.sh:2718-2725) | closed |
| T-24-12 | Repudiation | restore proof taken from a docker cp copy | high | mitigate | Host 2 backups root must be empty before restore; restore-run.txt names the copy path; `TRANSFER: equal` | closed |
| T-24-13 | Repudiation | a record sentence with no output behind it | high | mitigate | `record_check` (nightly.sh:491-580): byte-for-byte blocks, ledger, headings, evidence commit | closed |
| T-24-14 | Tampering | a fix loosens the sandbox | high | mitigate | Service and timer byte-identical to bf74496 at HEAD; diff pasted in the hand-over section | closed |
| T-24-15 | Information disclosure | secret pasted into the record | high | mitigate | `leak_check` before pasting; `record_check` refuses a private key marker; independent scan of the record found none | closed |
| T-24-16 | Repudiation | evidence made against a different text | medium | mitigate | One evidence commit (b501b1b) named and checked; bound files unchanged between it and the phase's last commit | closed |
| T-24-17 | Tampering | docs/beta.md edited ahead of its phase | low | mitigate | `git diff --quiet f1ba5a2` clean at the phase's last commit | closed |
| T-24-SC (plan 01) | Tampering | npm/pip/cargo installs | high | accept | Nothing installed | closed |
| T-24-SC (plan 02) | Tampering | npm/pip/cargo installs | high | accept | Only two Debian apt packages (openssh-client, openssh-server) in throwaway containers | closed |
| T-24-SC (plan 03) | Tampering | npm/pip/cargo installs | high | accept | Nothing installed | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-24-01 | T-24-04 | Privileged systemd container inherited from the phase 20 harness; throwaway, no published port, no host mount (re-verified at run.sh:545) | plan 24-01 register | 2026-10-31 |
| AR-24-02 | T-24-08 | ssh-keyscan on first use between two throwaway containers on one bridge; real-host comparison by hand is owner step O-3 | plan 24-02 register | 2026-10-31 |
| AR-24-03 | T-24-10 | rsync --delete mirrors an emptied backup root off-site; no feature built, behaviour observed and documented (DEPLOY.md:701) | plan 24-02 register | 2026-10-31 |
| AR-24-04 | T-24-SC (x3) | No npm, pip or cargo install; only two Debian apt packages in throwaway containers | plans 24-01, 24-02, 24-03 registers | 2026-10-31 |

---

## Non-blocking Observations (no change to the verdict)

- WR-01 (code review): the pure verdict functions return "ok" on empty or non-numeric input. Not reachable by the shipped stages; matters only for hand-edited evidence. Harden before the next evidence set.
- WR-02 (code review): `chain` ignores the `run.sh backup` status and the failed-step count. Latent false green in a future chain's exit code; the recorded set has 0 of 56 steps failed.
- `systemctl start holzcloud-backup.service` in `prune` and `offsite` is not a bounded poll and the oneshot has no `TimeoutStartSec`. Medium, below the block threshold.
- `leak_ssh` runs only in `offsite`; `offsite-restore` and `chain` run `leak_check` only. Independent grep found no host-key blob in evidence or record.
- `deploy/backup.sh` changed in phase 29 after the evidence commit b501b1b. Phase 28 must cite the record only for text that still matches, or re-run `nightly.sh chain`.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-31 | 20 | 20 | 0 | gsd-security-auditor (ASVS 1, block_on high) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-31
