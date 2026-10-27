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

- [x] **DOC-01**: Every line of `docs/beta.md` that moves from *does not promise* to *promises* carries a sentence of a record, checked byte for byte as 0.1.0 did; every line that stays is still named with its reason.
- [x] **DOC-02**: `OWNER-STEPS` for this milestone lists each step that needs a real server or the GitHub UI: the command, the output to expect, and what a different output means.

### Repository (TIDY)

- [x] **TIDY-01**: Each remote branch in `docs/branches.md` is decided: the seven Safe-to-delete and the two Review-first branches are removed or kept on purpose, with the decision written down.
- [x] **TIDY-02**: STATE.md carries one current position; the history blocks are collapsed so `state.advance-plan` no longer refuses with `ambiguous_position_phase`.
- [x] **TIDY-03**: v0.1.0 is archived: `milestones/v0.1.0-ROADMAP.md`, `-REQUIREMENTS.md`, its audit, and a one-line entry in ROADMAP.md as for v2.5.
- [x] **TIDY-04**: The local backup branch from 2026-10-20 and the stray branch in the shared checkout are retired.

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

**Note, 2026-10-27.** The row "New CMS functions" held for the requirements of 0.1.1. In the same session the owner nevertheless asked for three small product changes. They are not requirements of this milestone and are named openly in the CHANGELOG 0.1.1 entry:
the image position option of the "Bild und Text" block (Bildlage); the scale-down question for pictures over 2560 px on upload (quick tasks 261026-bon, 261026-rrh for EXIF orientation and ICC profile on scale-down, 261026-sxy for the orientation stub kept by `StripMetadata`); and the layout fixes of PR 47.

The owner also decided on 2026-10-26 to change the shipped `backup.sh`: retry rsync status 24 once, then fail loudly; a failed night removes its own directory, so none is left without a MANIFEST. This touches the row about changing the shipped `backup.sh` and will be a new phase with a new evidence set. The row stands as the milestone's default; this decision is the recorded exception.

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
| SRV-01 | Phase 26 | Pending owner |
| SRV-02 | Phase 26 | Pending owner |
| SRV-03 | Phase 26 | Pending owner |
| TIDY-01 | Phase 27 | Complete |
| TIDY-02 | Phase 27 | Complete |
| TIDY-03 | Phase 27 | Complete |
| TIDY-04 | Phase 27 | Complete |
| DOC-01 | Phase 28 | Complete |
| DOC-02 | Phase 28 | Complete |

`Pending owner` means the instrument and the owner steps exist and the owner's results are not recorded (phase 26, `deploy/NATIVE-AMD64-RUN.md`). The cell becomes `Complete` and the box `[x]` only in the commit that records the result and sets that requirement's `Status: recorded` in the record. `bash deploy/test/native-amd64.sh status-check` fails if the two files disagree. The value `Pending owner` is deliberate: it is not one that the workflow tools mark complete by themselves (phase 26, D-06).

TIDY-01 and TIDY-04 read `Pending owner` for a different reason than SRV-01 to SRV-03: what is left is the deletion of remote branches and of the old local branch, which needs the owner's credentials and explicit go at that moment (plans 27-04 and 27-05; the steps are in `.planning/phases/27-repository-tidy/27-OWNER-STEPS.md`). The cell becomes `Complete` and the box `[x]` only in the commit that records the deletions. `bash .planning/phases/27-repository-tidy/27-check.sh requirements` holds the file to `docs/branches.md` and to git. TIDY-01 became `Complete` on 2026-10-26, when the nine remote branches of the Decisions table were deleted with the owner's go (plan 27-04); TIDY-04 became `Complete` on 2026-10-26, when the two local branches were retired with the owner's go (plan 27-05; the old lineage lives on only in the verified bundle recorded in the owner steps).

**Coverage:**
- v0.1.1 requirements: 17 total
- Mapped to phases: 17
- Unmapped: 0 ✓

---
*Requirements defined: 2026-10-20*
*Last updated: 2026-10-26 after the phase 26 hand-over, SRV-01 to SRV-03 pending owner*
