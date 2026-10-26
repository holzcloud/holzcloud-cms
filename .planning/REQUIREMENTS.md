# Requirements: v0.1.1 — Beta belegt

**Defined:** 2026-10-20
**Core Value:** One small binary runs several websites without dependency soup.

Opened on 2026-10-20 after v0.1.0 (released 2026-10-18, the first beta). 0.1.0 wrote
down what it does **not** promise, each limit named, in `docs/beta.md`. This
milestone takes that list and plays what can be played, measures what can be
measured, and hands over what only a real server can show as numbered owner steps
with the output to expect. A line may move from *does not promise* to *promises*
only together with the record that proves it. No feature is built.

---

## v0.1.1 Requirements

### Backup timer (BKP)

- [x] **BKP-01**: The backup timer fires from the **shipped** calendar entry (03:15), with no drop-in that moves it; a record shows the unit starting by itself and the backup it made.
- [x] **BKP-02**: After a missed run (host off at 03:15), `Persistent=true` makes the backup run when the host comes up; a record shows it.
- [x] **BKP-03**: `RandomizedDelaySec` is played with its shipped 300 seconds, not 0; a record shows the delay inside that window.
- [x] **BKP-04**: With `REMOTE_TARGET` set, the copy to a second host runs, and a restore from that copy brings the site back.
- [x] **BKP-05**: Retention pruning deletes backups older than the retention and keeps younger ones; a record shows both kinds.

### Consistency and compatibility (CON)

- [x] **CON-01**: A backup taken while pages are written and media is added gives a database and a media directory that agree, or `docs/beta.md` names the exact window in which they may not, with the run that provoked it.
- [x] **CON-02**: A backup made by the previous release (0.1.0) is restored by this one; a record shows the pages, the login and the migration level afterwards.
- [x] **CON-03**: A disk that is nearly full but above `MIN_FREE_MB` is provoked, and the record says what the backup does.

### Real server (SRV)

- [ ] **SRV-01**: The five plugins run under `MemoryDenyWriteExecute=true` on **native amd64** (a GitHub-hosted runner counts), and native backup and restore timings replace the emulation figures.
- [ ] **SRV-02**: A restore on a real server, from a backup made there, to a different path, is handed over as owner steps and the result is recorded when the owner has made it.
- [ ] **SRV-03**: A reboot with the service and timer units enabled brings the site back; same hand-over and recording as SRV-02.

### The promise in writing (DOC)

- [ ] **DOC-01**: Every line of `docs/beta.md` that moves from *does not promise* to *promises* carries a sentence of a record, checked byte for byte as 0.1.0 did; every line that stays is still named with its reason.
- [ ] **DOC-02**: `OWNER-STEPS` for this milestone lists each step that needs a real server or the GitHub UI: the command, the output to expect, and what a different output means.

### Repository (TIDY)

- [ ] **TIDY-01**: Each remote branch in `docs/branches.md` is decided: the seven Safe-to-delete and the two Review-first branches are removed or kept on purpose, with the decision written down.
- [ ] **TIDY-02**: STATE.md carries one current position; the history blocks are collapsed so `state.advance-plan` no longer refuses with `ambiguous_position_phase`.
- [ ] **TIDY-03**: v0.1.0 is archived: `milestones/v0.1.0-ROADMAP.md`, `-REQUIREMENTS.md`, its audit, and a one-line entry in ROADMAP.md as for v2.5.
- [ ] **TIDY-04**: The local backup branch from 2026-10-20 and the stray branch in the shared checkout are retired.

---

## Future Requirements

- **FUT-01**: A checksum file inside a backup, so bit rot in a complete backup is caught. A feature, not a measurement.
- **FUT-02**: A database-plus-media snapshot that is atomic. Becomes a requirement only if CON-01 shows the window cannot be named and lived with.
- **FUT-03**: Certificate issuance, DNS and the proxy in front of the service played on a real domain.
- **FUT-04**: The single sign-on paths (Authentik forward authentication, OpenID Connect) played.
- **FUT-05**: Systemd versions other than 252 and 257 measured (Ubuntu 24.04 ships 255).

## Out of Scope

| Feature | Reason |
|---------|--------|
| New CMS functions | This milestone proves what 0.1.0 said; it adds nothing a person sees. |
| Changing the shipped `backup.sh` timer calendar or retention to make a run easier | The point is to play what ships. A change is a finding, written as one. |
| Tuning the unit for a better `systemd-analyze security` score | 0.1.0 states "measured, not tuned"; that stays. |
| Merging the archived parallel lineage | It is a second implementation of the same product; features are ported one by one if ever. |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| BKP-01 | Phase 24 | Complete |
| BKP-02 | Phase 24 | Complete |
| BKP-03 | Phase 24 | Complete |
| BKP-04 | Phase 24 | Complete |
| BKP-05 | Phase 24 | Complete |
| CON-01 | Phase 25 | Complete |
| CON-02 | Phase 25 | Complete |
| CON-03 | Phase 25 | Complete |
| SRV-01 | Phase 26 | Pending |
| SRV-02 | Phase 26 | Pending |
| SRV-03 | Phase 26 | Pending |
| TIDY-01 | Phase 27 | Pending |
| TIDY-02 | Phase 27 | Pending |
| TIDY-03 | Phase 27 | Pending |
| TIDY-04 | Phase 27 | Pending |
| DOC-01 | Phase 28 | Pending |
| DOC-02 | Phase 28 | Pending |

**Coverage:**
- v0.1.1 requirements: 17 total
- Mapped to phases: 17
- Unmapped: 0 ✓

---
*Requirements defined: 2026-10-20*
*Last updated: 2026-10-20 after milestone v0.1.1 started*
