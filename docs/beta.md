# What 0.1.1 promises, and what it does not

## What beta means here

Beta here means that you can run 0.1.1 for real as long as you keep backups, and that it is not feature-frozen: features, settings and the template contract may still change or go away from one release to the next. The schema and the migrations are not promised stable either: an update may add migrations, and an older binary must not be started against a migrated database, which is the limit that leads the *Rolling back* section of `deploy/DEPLOY.md` and was observed (see P-5). The tables below are the whole promise, and each promise is a sentence of a recorded proof; anything that is not in them is not promised. A restore you have tried somewhere other than production is worth more than any sentence here, so try one before you need it.

## What 0.1.1 promises

Every row below was shown in a Debian 12 container with systemd as PID 1, amd64 under emulation on an arm64 host, except where a row says otherwise; no row is about a real server. The rows P-11 onward rest on `deploy/BACKUP-TIMER-RUN.md` and `deploy/BACKUP-CONSISTENCY-RUN.md`. Each row names the record it rests on and the words it quotes from it, and the words can be found in that record with `grep -F`.

| ID | Promise | Record | Cited words |
|----|---------|--------|-------------|
| P-1 | The hardened unit starts under real systemd 252 and the site answers. On the emulated platform the first start crashed, and the unit needed a container-only drop-in there. | `deploy/SECURITY-SCORE.md` | `UNIT-01: the unit starts and the site answers`, `The unchanged unit started`, `the first start crashed and needed a container-only drop-in` |
| P-2 | `MemoryDenyWriteExecute=true` stays in the unit: all five plugins load and run under it on wazero's interpreter fallback. The evidence is native arm64 only, and on amd64 the verdict is void. | `deploy/SECURITY-SCORE.md` | `SETTLED, and the unit keeps`, `All five plugins load and run on wazero's interpreter fallback with the directive enforced`, `The evidence comes from native arm64 only`, `On amd64 the verdict is VOID` |
| P-3 | The exposure level that `systemd-analyze security` reports for the unit was measured, and the unit was not tuned to improve it. | `deploy/SECURITY-SCORE.md` | `Overall exposure level 1.3 online and 1.4 offline on systemd 252`, `1.3 online and 1.3 offline on systemd 257`, `Measured, not tuned` |
| P-4 | A fresh install by the text of `deploy/DEPLOY.md` as it stood at build 2a11eeb works, played command by command, with the Caddyfile validated and no secret leaked. | `deploy/DEPLOY-RUN.md` | `DEP-01: a fresh install by the document alone`, `33 steps played, 0 failed`, `CADDY: validated`, `LEAK-CHECK OK` |
| P-5 | An update across two migrations and a rollback were played, in both situations a real update produces (with a snapshot, and binary only). An older binary must not be started against a migrated database; that limit was observed. | `deploy/DEPLOY-RUN.md` | `DEP-02: an update across migrations`, `MIGRATIONS: old=58 new=60 pending=2`, `52 PASS and no BAD against the state before the update`, `DEP-03: rolling back, and the migration limit`, `MODE: snapshot`, `MODE: binary`, `The limit, that an older binary must not be started against a migrated database, leads the section and was observed` |
| P-6 | A backup of the database and the media, made by hand and again by the timer, is whole, and it carries media, templates, plugins, language file, logo and the CSRF key. | `deploy/BACKUP-RUN.md` | `BAK-01 PASS`, `A backup of the database and the media, made by hand and again by the timer, is whole`, `media, templates, plugins, language file, logo and the CSRF key` |
| P-7 | A backup that the timer's service made restores with `restore.sh` into a different, empty system with working pages, media, login and plugins, and the restore passes its own checks. Four refusals leave the live data untouched. | `deploy/BACKUP-RUN.md` | `BAK-02 PASS`, `restored by the document's own restore line into a different, empty container`, `pages and the marker page answer`, `RESTORE-NEGATIVE PASS`, `STEPS: 78 played, 0 failed`, `Four refusals (damaged database, no database, answer no, too little disk) leave the live data untouched` |
| P-8 | One backup run was started by `holzcloud-backup.timer` through the hardened service and succeeded. | `deploy/BACKUP-RUN.md` | `BAK-03 PASS`, `TriggeredBy=holzcloud-backup.timer`, `Result=success`, `=== Backup complete ===` |
| P-9 | On an emulated amd64 container a backup took about a second and a restore about eight seconds, at 20 MB and at 320 MB of backup. These are emulation figures and not a promise of speed. | `deploy/BACKUP-RUN.md` | `BACKUP-WALL-S=0.692`, `BACKUP-WALL-S=1.015`, `RESTORE-WALL-S=8.126`, `RESTORE-WALL-S=8.296`, `every number is an emulation number on an aarch64 host` |
| P-10 | The documents say what no backup holds, so that you rebuild it by hand: the unit file and its drop-ins, environment files, `HOLZCLOUD_SECRET_KEY` with the payment and mail credentials, the proxy configuration and the certificate state, and the pre-upgrade snapshots in the data directory. | `deploy/BACKUP-RUN.md` | `HAND-REBUILT`, `the payment and mail credentials that live in an environment file`, `the proxy configuration and the certificate state, and the pre-upgrade snapshots in the data directory` |
| P-11 | BKP-01: the shipped `holzcloud-backup.timer` starts the shipped, hardened `holzcloud-backup.service` at its own 03:15 entry, with no drop-in and no person, and the backup it leaves is whole. To make 03:15 arrive within two minutes the time zone of the container was moved; the zone of a real server is not shown. | `deploy/BACKUP-TIMER-RUN.md` | "The shipped entry fires the shipped, hardened service without a person, a drop-in or a manual start, and the backup it leaves is whole.", `OnCalendar=*-*-* 03:15:00`, `START-LOCAL: 03:15:19` |

## What 0.1.1 does not promise

These are the limits, each one named. They come from the *Not tested* lists of the records, from the hand-over of each record, from the limits of 0.1.0 that stay, and from the findings of the later phases, and none is softened here. If a use of 0.1.1 needs one of them, it needs a run of its own first.

### The nightly backup and a restore

- The clock and the zone of a real server were not in play: the zone of the container was moved so that 03:15 arrived within two minutes, and daylight saving was not tested. Why: the record lists the real wall-clock 03:15 on a server's own zone as not tested. Record: deploy/BACKUP-TIMER-RUN.md "The real wall-clock 03:15 on a server's own zone, and daylight saving." Owner step OS-6.
- `Persistent=true` after a missed run was not exercised: the timer had never fired before the one observed run, and nothing was powered off.
- `RandomizedDelaySec` was not exercised: the drop-in set it to 0, so the shipped 300 seconds of random delay were never in play.
- The off-site `REMOTE_TARGET` copy did not run: the variable was not set.
- Retention pruning did not delete anything: no directory was older than the retention of 30 days.
- An atomic database-plus-media snapshot is not promised: the pair is taken one after the other with the site running, and the window in which a file is added or deleted in between was not provoked.
- A backup made by an older binary than the one that restores it was not played in the final set.
- A restore on a real server was not made: every restore went into a throwaway container, with no separate data volume, a warm cache and no real network copy.
- Native amd64 timing was not measured: every number is an emulation number on an aarch64 host, and the extrapolation in the record is an estimate.
- Bit rot in a complete backup is not caught: a flipped byte inside a data page passes both integrity readers, and no checksum file exists.
- A disk that is nearly full but above `MIN_FREE_MB` was not provoked; the floor was only shown to refuse an impossible value.
- The consequence of losing `HOLZCLOUD_SECRET_KEY` is stated in `deploy/DEPLOY.md` and was never played.
- A restore through a browser was not seen: the login and the pages were checked by `curl`, by status codes, hashes and counts.

### Install, update and rollback

- A real VPS image and its package set, such as Ubuntu 24.04, were not played: the install ran on Debian 12 only.
- Certificate issuance and everything behind it were not played: DNS, ACME, the proxy reaching the service, and a browser over https.
- Five or more successive updates and the pruning of the pre-upgrade snapshots were not played.
- A rollback of media and templates is not promised: the snapshot holds the database only.
- The single sign-on paths (Authentik forward authentication and OpenID Connect) were not played.
- A reboot with the units enabled was not made.
- The compulsory two-factor enrolment and the shop and form flows through a browser were not played; orders were written straight into the database.
- A rollback after a failed migration was not played: the update migrated cleanly, and that path is read from the code.
- A database of production size was not used: the two migrations took milliseconds on test data.

### The unit and the platform

- No native amd64 run was made and none is planned (a user decision); the assumption stated for `MemoryDenyWriteExecute` on amd64 stays an assumption.
- The seccomp-based directives (`MemoryDenyWriteExecute`, `RestrictAddressFamilies`, `SystemCallFilter` and the rest) are not enforced under the amd64 emulation, so a green run there says nothing about them.
- Seccomp enforcement was observed on native arm64 only, and only `MemoryDenyWriteExecute` and the address-family directive were probed there, while the target platform is linux/amd64.
- Systemd versions other than 252 and 257 were not measured, such as systemd 255 on Ubuntu 24.04.
- Debian 12 on real hardware was not measured.
- The privileged shared-kernel container hides what the resource directives do on a server: they were not tested for effect.
- A data directory without a separate mount was the only case: `RequiresMountsFor` was not exercised.

### The product itself

- Features, settings and the template contract may change or be removed between releases without a transition period.
- The schema and the migrations are not promised stable, and an older binary must not be started against a migrated database.

## What changed since 0.1.0

A limit of 0.1.0 that a record now answers is listed here, with what 0.1.1 says and the promise rows that answer it. Every other limit of 0.1.0 stays above, with its reason.

| 0.1.0 said | 0.1.1 says | Promise rows |
|------------|------------|--------------|
| The real 03:15 window was not exercised: the timer was moved a minute ahead by a container-only drop-in, so the shipped calendar entry never fired. | The shipped calendar entry fired the shipped service without a drop-in and without a person, in a container whose zone was moved so that 03:15 arrived within two minutes. A real server's own zone and clock are still not shown. | P-11 |

## What the evidence is bound to

Each promise rests on a run made against one commit, and the commit is part of the claim.

| ID | Evidence | Record | Cited words |
|----|----------|--------|-------------|
| E-1 | P-1 to P-3, the unit measurements, were made from the repository state of that commit. | `deploy/SECURITY-SCORE.md` | `2026-10-12, f1428d2`, `The commit is the repository state the runs were made from` |
| E-2 | P-4 and P-5, install, update and rollback, were played against build 2a11eeb. | `deploy/DEPLOY-RUN.md` | `build 2a11eeb (v0.0.25-42-g2a11eeb)`, `recorded on top of 83b4348` |
| E-3 | P-6 to P-10, backup and restore, were played against evidence commit bf74496. | `deploy/BACKUP-RUN.md` | `Evidence commit: bf74496`, `build v0.0.25-68-gbf74496` |

P-4 and P-5 are about the text of `deploy/DEPLOY.md` at build 2a11eeb. Since then `deploy/DEPLOY.md` has changed, and `git diff -U0 2a11eeb HEAD -- deploy/DEPLOY.md` shows where: in *Prerequisites* (the line about `rsync` names the data directories it copies), in *Backups* (rewritten: what a backup holds and does not hold, that the pair is not atomic, `MIN_FREE_MB`), in two sentences of *Updating* and *Rolling back* that say what the pre-upgrade snapshot covers (the database only; media, templates, plugins, language files and the logo are not in it), and in *Restore from a backup* (rewritten: the steps of the script in order, what comes back, what you rebuild by hand, how long it takes). No command of *Install*, *Updating* or *Rolling back* changed; the changes there are sentences. The *Backups* and *Restore from a backup* text was played by Phase 22 against bf74496, which is why P-6 to P-10 are bound to that commit.

The rule of `deploy/test/README.md` stands: a change to `deploy/DEPLOY.md`, `backup.sh`, `restore.sh`, the units or `deploy/test/run.sh` after the evidence commit means the evidence set is made again before the record is cited for the changed text. The command that shows the rule holds is this one, and it exits 0 for 0.1.0:

```text
git diff --quiet bf74496 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh
```

## Where the word alpha still appears

The admin screen still labels every 0.x build alpha: a tag in the sidebar, a line on the login page and a notice on the dashboard. The label is derived from the version number, not written by hand: `web.IsAlpha` in `internal/web/layoutdata.go` is true for every build below 1.0, and it goes away with 1.0. The *Updating* section of `deploy/DEPLOY.md` also still calls the software alpha software.

Neither was reworded for 0.1.1. Changing the label would orphan catalogue keys in four languages, and an edit of `deploy/DEPLOY.md` belongs to the evidence set and would force it to be made again. The substance of both sentences, that an update may change or remove features, settings and the template contract, is what beta means on this page, so only the label differs.
