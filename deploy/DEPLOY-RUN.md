# Install, update and rollback by the document alone: measured results

This document is the permanent record of phase 21. It records what happened when
`deploy/DEPLOY.md` was played literally, command by command, in a bare systemd Debian container, what
that play cannot say about a real server, and which defects it found in the document and how they were
fixed. Nothing was tuned to look better: a defect is fixed in `deploy/DEPLOY.md` and listed here with the
output that showed it. A deviation is only ever a thing the container cannot do, never a defect that was
renamed.

The record rests on one evidence set: three runs from fresh containers, made once against the final `deploy/DEPLOY.md`
(see *How this was run*). Their raw output is the Appendix, pasted unchanged and checked line by line against the scratch
files. The Findings quote the output of the earlier passes, the ones made before the document was repaired; that output
showed the defects and is not pasted again, the Appendix shows the document after the repair. What the container cannot say about a
real server is in *Container vs. server*, and what was not measured at all is in *Not measured*.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| DEP-01: a fresh install by the document alone | PASS. `fresh` from a bare systemd image ends in `DEP-01 PASS`: 33 steps played, 0 failed, `CADDY: validated` (Caddy 2.11.7), `LEAK-CHECK OK`. The first pass of the document found ten defects; all are fixed in `deploy/DEPLOY.md`, and the final run shows only the named deviations (browser, caddy, dropin, mount, scp). | linux/amd64 (emulated on an arm64 host), Debian 12, systemd 252 (252.39-1~deb12u2) | 2026-10-12, build 2a11eeb (v0.0.25-42-g2a11eeb), recorded on top of 83b4348 |
| DEP-02: an update across migrations | PASS. `lifecycle` with the old tag v0.0.10 (58 migrations) to the current build (60), two migrations between the builds (`MIGRATIONS: old=58 new=60 pending=2`), through the Updating section: 52 PASS and no BAD against the state before the update (46 content tables with equal row counts, equal page and media hashes, login, public page, five running plugins, a filled order token), and 50 PASS and no BAD against the service's own pre-upgrade snapshot, which the journal and the data directory both show. The as-shipped section was silent in five places; all are fixed. (The v0.0.21 run is the no-migration hop that belongs to DEP-03: it prints `DEP-02 not applicable` because no migration lies between those builds.) | linux/amd64 (emulated on an arm64 host), Debian 12, systemd 252 (252.39-1~deb12u2) | 2026-10-12, build 2a11eeb (v0.0.25-42-g2a11eeb), recorded on top of 83b4348 |
| DEP-03: rolling back, and the migration limit | PASS in both situations a real update produces, `DEP-03 PASS` in both summaries. With a snapshot (v0.0.10 to the current build, schema 58 to 60, `MODE: snapshot`): the old version runs again, the data of before the update is back (52 PASS, no BAD against the pre-update state), the page written after the update is gone, the migrated database is kept as `holzcloud.sqlite.after-update`, and the restart migrates nothing. Without a snapshot (v0.0.21 to the current build, the same schema 60, `MODE: binary`): the old binary alone is a complete rollback, it undoes nothing and loses nothing (53 PASS, no BAD against the state after the update). The limit, that an older binary must not be started against a migrated database, leads the section and was observed: the old binary starts, answers ok and fails at the second order. The Updating check line was found to end with a failure status when nothing migrates, and was fixed. | linux/amd64 (emulated on an arm64 host), Debian 12, systemd 252 (252.39-1~deb12u2) | 2026-10-12, build 2a11eeb (v0.0.25-42-g2a11eeb), recorded on top of 83b4348 |

The build under test is the NEW of all three summaries, `v0.0.25-42-g2a11eeb 2a11eeb`; the chains `fresh` and `lifecycle` themselves
are the ones committed in 2a40bd3. The old tags are v0.0.10 (`ec365d6`, for DEP-02 and the DEP-03 case with a snapshot) and v0.0.21 (`76d99c3`,
the no-migration hop of DEP-03).

## How this was run

Every command comes out of `deploy/DEPLOY.md` itself: `deploy/test/run.sh` reads the fenced bash blocks of a
section and refuses a line the document does not contain, so the play cannot drift from the text. Each step
leaves `step-NN.txt` and one `STEP nn rc=R` line behind, a deviation leaves a `DEVIATION[name]` line.

Pass 1 (plan 21-01, the document as it was, nothing fixed):

```text
PLATFORM=linux/amd64 TARGET=systemd-base OUT_DIR=deploy/test/out/amd64-install-pass1 ./deploy/test/run.sh build up install
PLATFORM=linux/amd64 TARGET=systemd-base OUT_DIR=deploy/test/out/amd64-install-pass1b ./deploy/test/run.sh build up install firstrun
PLATFORM=linux/amd64 TARGET=systemd-base OUT_DIR=deploy/test/out/amd64-install-pass1b ./deploy/test/run.sh caddy
PLATFORM=linux/amd64 TARGET=systemd-base OUT_DIR=deploy/test/out/amd64-install-pass1b ./deploy/test/run.sh postinstall
```

Pass 2 (this plan, the repaired document, from a fresh container each time):

```text
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-install OUT_DIR=deploy/test/out/amd64-install ./deploy/test/run.sh build up install firstrun
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-install OUT_DIR=deploy/test/out/amd64-install ./deploy/test/run.sh postinstall
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-install OUT_DIR=deploy/test/out/amd64-install ./deploy/test/run.sh caddy
```

Pass 2 was made from commit 58ae902 (`git rev-parse --short HEAD`) with the changes of this plan in the working
tree, on 2026-10-12 (`date -u +%F`). With the image build cached, the calls took 21 s (`build up install firstrun`), 2 s
(`postinstall`) and 16 s (`caddy`); the emulated first start of the service needs about ten seconds of that.

The update runs (DEP-02), each from a fresh container started from the same bare image, with the old release installed
by the document and seeded, and then the Updating section played:

```text
PLATFORM=linux/amd64 TARGET=systemd-base INSTALL_FROM=old OLD_TAG=v0.0.10 NAME=holzcloud-update-pass1 OUT_DIR=deploy/test/out/amd64-update-pass1 ./deploy/test/run.sh build up install baseline update
PLATFORM=linux/amd64 TARGET=systemd-base INSTALL_FROM=old OLD_TAG=v0.0.10 NAME=holzcloud-update-pass1b OUT_DIR=deploy/test/out/amd64-update-pass1b ./deploy/test/run.sh build up install baseline update
PLATFORM=linux/amd64 TARGET=systemd-base INSTALL_FROM=old OLD_TAG=v0.0.10 NAME=holzcloud-update OUT_DIR=deploy/test/out/amd64-update ./deploy/test/run.sh build up install baseline update
```

Pass 1 (plan 21-02) played the Updating section as it was shipped and recorded what it left out. Pass 1b (plan 21-03,
still the shipped section) added the complete comparison to the harness and proved it works. Pass 2 is the repaired section.
The old and the new binary are built by the document's own Build line, in a clone of the repository at the old tag and at
HEAD, and are cached by ref.

The whole proof replays from one command per chain (plan 21-05), each from a fresh container started from the bare image, each with its own `NAME`
and `OUT_DIR`, and each ending in `summary.txt` and a `LEAK-CHECK OK`:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-fresh OUT_DIR=deploy/test/out/amd64-fresh ./deploy/test/run.sh fresh
PLATFORM=linux/amd64 TARGET=systemd-base OLD_TAG=v0.0.10 NAME=holzcloud-lifecycle OUT_DIR=deploy/test/out/amd64-lifecycle ./deploy/test/run.sh lifecycle
PLATFORM=linux/amd64 TARGET=systemd-base OLD_TAG=v0.0.21 NAME=holzcloud-lifecycle-hop OUT_DIR=deploy/test/out/amd64-lifecycle-hop ./deploy/test/run.sh lifecycle
```

These three runs are the evidence set of the record. They were made once, against the final `deploy/DEPLOY.md` (which was not edited afterwards), and
`fresh` ends in `DEP-01 PASS`, `lifecycle` with v0.0.10 in `DEP-02 PASS` and `DEP-03 PASS` (mode snapshot), and `lifecycle` with v0.0.21 in `DEP-02 not
applicable` and `DEP-03 PASS` (mode binary, no snapshot).

The three runs were made one after the other on 2026-10-12 with the image build and both binaries cached (`bins` had built `holzcloud-old-v0.0.10`,
`holzcloud-old-v0.0.21` and `holzcloud-new-2a11eeb` by the Build line of the document, in clones of the repository). From the end of `up` to the
last line of `summary.txt`, read from the file times of the output directories, `fresh` took 35 s, `lifecycle` with v0.0.10 75 s and
`lifecycle` with v0.0.21 about 60 s. The set was made against the `deploy/DEPLOY.md` of commit 2a11eeb, which is the text in the repository now:
the document was not edited after the set was made, and the repository gates of plan 21-06 do not touch it. The three `versions.txt` files are identical
and the host table below is read from them.

| | Value |
|---|---|
| Host architecture | aarch64 |
| Host | Docker Desktop VM, Docker 29.8.2 |
| Host kernel | 7.0.14-linuxkit |
| Container uname | Linux 7.0.14-linuxkit x86_64 |
| Container systemd | 252 (252.39-1~deb12u2) |
| Container OS | Debian 12 (bookworm) |

## Deviations named

These are the things the container cannot do the way a server does. They are named in the output of every run
(`deviations.txt`), and a deviation outside this set fails the run. They are not defects of the document.

| Deviation | Why it exists | What a real server does instead |
|-----------|---------------|---------------------------------|
| DEVIATION[scp] | There is no ssh and no second machine in the container. The two `scp` lines are played as `docker cp` of the built binary and of the working tree's `deploy` directory. | `scp` from the development machine to the server |
| DEVIATION[dropin] | Under amd64 emulation the unchanged unit crashes at its first start, because `ProcSubset=pid` hides a file the emulation layer opens. The harness adds a container-only drop-in that sets `ProcSubset=all` after recognising exactly that signature. `deploy/holzcloud.service` is untouched. | Nothing: the unit starts as shipped on a real kernel |
| DEVIATION[browser] | First Run is a browser dialogue. It is played with `curl`: the CSRF field is read out of the page, and a Referer with the https origin is added, because the shipped unit sets `HOLZCLOUD_SECURE=true`. | An operator opens `https://your-domain/admin` through the TLS proxy |
| DEVIATION[caddy] | `systemctl reload caddy` is replaced by `caddy validate --config /etc/caddy/Caddyfile`: the container has no public domain, no DNS and no ACME, and a real reload would start certificate issuance for example.com. | The reload, with a Caddyfile that names the real domain |
| DEVIATION[mount] | The unit's `RequiresMountsFor` names `/opt/holzcloud/data`, a plain directory on the container's root filesystem. No separate mount exists, so that line is not exercised. | A data directory on its own volume, mounted before the service starts |
| DEVIATION[fixture] | The plugin archives, the test image, the login user and the order row stand in for work an operator does in the admin and in the shop, not for steps of the document. The archives and the image are copied in from the working tree, the login user is created by the old binary's `holzcloud user create` with a password generated in the container, and the order row is written straight into the database in the statement shape of the old code, because the shop flow needs a browser. | The operator installs plugins, uploads images, creates users and receives orders in the admin and on the site |
| DEVIATION[network] | Printed only when a step fails because no package repository or download host can be reached. The run then exits 3 ("ran, but this platform cannot give the evidence") and not 1. It did not occur in either pass. | The server has a network |

Which of them the three runs of the evidence set show, counted from their `deviations.txt`:

| Run | Deviation lines |
|-----|-----------------|
| `amd64-fresh` | DEVIATION[browser] once, DEVIATION[caddy] once, DEVIATION[dropin] once, DEVIATION[mount] once, DEVIATION[scp] twice (the binary and the deploy directory) |
| `amd64-lifecycle` (v0.0.10) | DEVIATION[dropin] once, DEVIATION[fixture] once, DEVIATION[mount] once, DEVIATION[scp] three times (the old binary, the deploy directory, and the new binary of the update) |
| `amd64-lifecycle-hop` (v0.0.21) | the same set as the run with v0.0.10 |

DEVIATION[network] occurs in none of them: every package repository and the Caddy repository were reachable. DEVIATION[browser] and DEVIATION[caddy]
are absent from the two lifecycle runs because those runs do not play First Run or the Caddy block.

A second word sits beside DEVIATION: **SCENARIO**. A deviation replaces a line of the document that the container
cannot play. A scenario is a step the harness performs that the document must not contain, because it plays what
the document tells nobody to do: the older binary started against a migrated database (`SCENARIO[negative]`), and the
new binary put back afterwards so that the chain can go on (`SCENARIO[restore-new]`). Scenarios go to `scenario.txt`,
never to `deviations.txt`, and the result of the first is `negative.txt`, which decides what DEPLOY.md may say about
that case.

| Scenario step | What it is | Why it is not in the document |
|---------------|------------|-------------------------------|
| SCENARIO[negative] | After an update that migrated, the old binary (`holzcloud.prev`) is copied over the live binary and started against the migrated database, with no snapshot restored. Then two orders are inserted in the statement shape of the old code. The result is `negative.txt` (`NEGATIVE-RESULT: starts` or `refuses`). | It is the thing *Rolling back* says not to do. It is played once so that the document states what happens and does not assume it. It cannot run when the update wrote no snapshot, because no migration lies between the builds, and the chain `lifecycle` then says so and skips it. |
| SCENARIO[restore-new] | The new binary is put back (`docker cp` to `/tmp/holzcloud`, stop, copy, start), and `/readyz` must report the new version again, so that the rollback that follows starts from the real post-update state. | It undoes the scenario above; an operator who never runs the scenario has nothing to undo. |

What a real server does instead, for both: nothing. An operator never starts the old binary against a migrated database (that is the case *Rolling back* warns about),
so there is no new binary to put back. In the evidence set `scenario.txt` of the v0.0.10 run holds both lines, and that of the v0.0.21 run holds only
`SCENARIO[negative]: skipped, no migration lies between these builds (pending=0)`, with its reason, because there is no migrated schema for the old binary to meet.

## Findings

One row per defect that pass 1 showed. The text in backticks in the fourth column is the decisive output, quoted
verbatim from the scratch files of pass 1 (`deploy/test/out/amd64-install-pass1`, `amd64-install-pass1b`, `amd64-update-pass1` and their
logs). Later plans add their own rows.

| Requirement | Step | DEPLOY.md said | What happened | Fix |
|-------------|------|----------------|---------------|-----|
| DEP-01 | Prerequisites | Names only `sqlite3`, with a one-line `apt install` that has no `apt update` and no other package. | `sqlite3 is already the newest version (3.40.1-2+deb12u2).` The image has sqlite3; the tools a later step uses were missing: `TOOL rsync absent`, `TOOL gpg absent`, `TOOL fuser absent` | A fenced block with `sudo apt update` and one `apt install -y` line naming every package the later steps use, each with its reason: ca-certificates, curl, jq, sqlite3, rsync, psmisc, gnupg, openssl. The Go version is the one on the `go` line of `go.mod`. |
| DEP-01 | Build | One `go build` line with `-ldflags="-s -w"`, no version. | The service starts and calls itself `dev`: `readyz: {"status":"ok","version":"dev","commit":"unknown","integrity":"ok"}` | The line stamps `-X main.Version` and `-X main.Commit`, as the release workflow does, with a sentence on why: two builds must be told apart in `/readyz` and in the name of the pre-upgrade snapshot. |
| DEP-01 | Install | Copies only `backup.sh`, to `/opt/holzcloud/backup.sh`; `restore.sh` is never installed. Backups, the backup unit and restore all use `/opt/holzcloud/deploy`. | `BAD missing /opt/holzcloud/deploy/backup.sh` and `BAD missing /opt/holzcloud/deploy/restore.sh` | Install copies the whole transferred deploy directory to `/opt/holzcloud/deploy` and makes both scripts executable. |
| DEP-01 | Backups | `sudo cp deploy/holzcloud-backup.{service,timer} /etc/systemd/system/`, a relative path that exists on the development machine only. | `cp: cannot stat 'deploy/holzcloud-backup.service': No such file or directory` and so `Failed to enable unit: Unit file holzcloud-backup.timer does not exist.` and `sudo: /opt/holzcloud/deploy/backup.sh: command not found` | The block copies from `/opt/holzcloud/deploy/`, which Install now creates. |
| DEP-01 | Caddy | `apt install -y debian-keyring ...` as the first command, with no package list update. | `E: Unable to locate package debian-keyring` | The block starts with `sudo apt update`. |
| DEP-01 | Caddy | Unpacks the repository key by piping it into `gpg`, which no prerequisite installs. | `sudo: gpg: command not found` and then the repository fails its signature check: `The following signatures couldn't be verified because the public key is not available: NO_PUBKEY ABA1F9B8875A6661` | `gnupg` and `curl` are named in the block and in Prerequisites; the final `apt install` carries `-y`, so nobody has to answer a question that a script cannot. |
| DEP-01 | Caddy | Copies the Caddyfile to `/etc/caddy/Caddyfile` and reloads. | Caddy was never installed, so the directory does not exist: `cp: cannot create regular file '/etc/caddy/Caddyfile': No such file or directory` | Consequence of the two rows above; fixed with them. After the repair the block installs Caddy 2.11.7 and `caddy validate` answers `Valid configuration`. |
| DEP-01 | First Run | Says to visit the domain and use the setup form; says nothing about the shipped unit's `HOLZCLOUD_SECURE=true`. | The forms work only as if they came through TLS: `The shipped unit sets HOLZCLOUD_SECURE=true, so the form is only accepted as if it came through the TLS proxy` | The section says the admin works through the TLS proxy, and that a form posted over plain HTTP to port 8080 is answered `403 Forbidden - CSRF validation failed` (measured in pass 2). |
| DEP-01 | First Run | Says nothing about who may use the setup form, nor about the second factor. | `PASS setup form open before any account: GET /admin/setup answers 200` and `PASS first request after the login: 303 to /admin/2fa/einrichten (compulsory two-factor enrolment)` | The section says the setup form is open to the first visitor until an account exists (create it before the domain is public, 404 afterwards), that the next request after the login is the compulsory two-factor enrolment, and points to the section on a locked-out administrator. Its verification commands are now a fenced block that `firstrun` plays. |
| DEP-01 | Troubleshooting | `sudo fuser /opt/holzcloud/data/holzcloud.sqlite` | `TOOL fuser absent` | The line names the package `psmisc`, which Prerequisites installs. |
| DEP-02 | Updating | Says "Back up first" and names no command. | The play of the section took no backup: `BACKUP: none` | Step 1 of the section: `sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups`, taken while the OLD binary is still in place, with the reason (every subcommand of a new binary migrates the database on the way, and takes no snapshot). |
| DEP-02 | Updating | Overwrites `/opt/holzcloud/holzcloud` with `sudo cp`, the only copy of the running version. | Nothing of the old version is left on the server: `PREV: absent` | Step 1 copies the running binary to `/opt/holzcloud/holzcloud.prev` with `sudo cp -p` and prints its version with `/opt/holzcloud/holzcloud version`. |
| DEP-02 | Updating | Carries its own copy of the build line, without the version flags that the Build section got in plan 21-02. | `STEP 20 rc=0 SKIP[dev-machine] CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud` | Step 2 carries the stamped line of the Build section. Without the stamp the new binary calls itself `dev`, and the pre-upgrade snapshot is named after `dev`. |
| DEP-02 | Updating | Says that migrations run automatically, and nothing about the snapshot the service takes before them or where it lies. | The journal holds it, and nothing in the section said to look: `"msg":"pre-upgrade snapshot written","path":"/opt/holzcloud/data/pre-upgrade-v0.0.25-30-gb5b0393-20261012-120647.sqlite"` | Step 3 greps the journal since the start for the two lines. The text states what was observed about the snapshot: name with the version of the new binary, old schema, only when migrations are pending, database only, newest five kept, mode 600 for the service user, and that the path is the one in the journal line. |
| DEP-02 | Updating | Ends at `systemctl start`, with no check that the service came up or that it is the new build. | The last step of the section returns at once, long before the service is ready: `STEP 24 rc=0 sudo systemctl start holzcloud` | Step 3 starts with `/readyz` through `curl --retry 30 --retry-connrefused --retry-delay 1` reduced with `jq` to status, version, commit and integrity, so the line waits for the start itself. The text names `StartLimitBurst` and sends a failed start to Recovery. |
| DEP-03 | Updating (as shipped before plan 21-03) | Ended at the sentence that migrations run automatically, and said nothing about going back: not that the old binary is needed, not that a snapshot exists, not what a rollback loses. | Inferred from the source file as it stood before plan 21-03, not from run output. The last step the play could take was the start of the service, the procedure ended there (the DEP-02 row on `STEP 24 rc=0 sudo systemctl start holzcloud`), and an operator whose update went wrong had no instruction. | A section *Rolling back* (plan 21-04), linked from *Updating* and from *Recovery*, with the limit as its first sentence. |
| DEP-03 | Rolling back | Not written. The assumption of the research was that a newer schema stops an older binary. | `NEGATIVE-RESULT: starts`, then `readyz: {"status":"ok","version":"v0.0.10","commit":"ec365d6","integrity":"ok"}`, `journal: none`, and at the second order `insert-2: Runtime error near line 2: UNIQUE constraint failed: orders.token (19)`. The old binary does not refuse a database that is two migrations ahead, and it says nothing about it. | The limit is the first paragraph of the section, and states this behaviour as observed, with the pair of versions it was seen with. |
| DEP-03 | Recovery, A failed migration | Quoted the command that the program prints when a migration fails, and left it at that. | Inferred from the source file cmd/holzcloud/main.go, not from run output: the program logs restore with: systemctl stop holzcloud && cp PATH DBPATH && systemctl start holzcloud, with the snapshot path and the database path filled in. It restores data only, runs under the NEW binary that has just failed, leaves the write-ahead files of the replaced database in place and ignores the start limit. | The text keeps the printed command as quoted, says that it restores data only, sends the reader to *Rolling back* from its first block (old binary first), and names `reset-failed` and `StartLimitBurst`. The program is unchanged: the message is incomplete, not wrong. |
| DEP-03 | Rolling back, restore | Not written. A plain copy of the snapshot over `holzcloud.sqlite` is the obvious restore. | Inferred from how the database is opened (write-ahead mode, see the pragmas in internal/db), not from run output: a write-ahead or shared-memory file that belongs to the replaced database stays next to the restored file, and SQLite reads a restored file together with a log that is not its own. The play removes them before the restore, and every restored database passed its check afterwards: `PASS integrity ok`. | Block 1 stops the service and then runs `sudo rm -f /opt/holzcloud/data/holzcloud.sqlite-wal /opt/holzcloud/data/holzcloud.sqlite-shm` before the move and the restore, and the text says why. |
| DEP-03 | Updating, check | The last line of step 3, the journal grep, as the check that a migration ran. | In the no-migration update (v0.0.21 to the current build) the line ends with a failure status, because grep finds nothing: `STEP 29 rc=1 sudo journalctl -u holzcloud --since` (hop pass 1, scratch directory deploy/test/out/amd64-hop-pass1). The update itself had worked. A script, or an operator who chains the steps with the shell AND operator, would stop on a healthy update. | The line ends with an `echo` fallback, `no migration, no snapshot`, joined with the shell's OR operator, as the last line of *Rolling back* block 1 already did, and the text says what the line prints when nothing migrated. |

**What the final set added to this table: nothing.** The three runs of the evidence set show no failing step (`STEPS: 33 played, 0 failed` in the
first, no `STEP` line with a status other than 0 in the other two except `systemctl status`, whose exit status reports the unit state and is
never counted), no BAD line in any comparison and a `LEAK-CHECK OK` in each summary. The one status 3 in each `steps.txt` is `sudo systemctl status holzcloud`
right after the first start, which under the emulation finds the unit in an automatic restart after the crash that DEVIATION[dropin] answers. A
defect found by a later run would have been fixed in `deploy/DEPLOY.md` and the whole set made again; none was.

### Changed without a failing line in pass 1

Two things in the repair did not fail in pass 1 and are recorded as what they are, reasoned from the files and
confirmed only by pass 2:

- `/opt/holzcloud/backups` is now owned by the service user and has mode 700. The backup unit runs as `holzcloud`
  with `ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups`, and pass 1 never got as far as a backup. In pass 2 the
  manual backup run as `holzcloud` into that directory succeeds.
- `ca-certificates` and `openssl` are named in Prerequisites. The image has both, so nothing failed; the first is what
  `curl` needs for the HTTPS downloads of the Caddy block, and the second is used by the single sign-on section. The
  document names every tool the image ships and a step uses, so no step works here by luck.
- Step 1 of Updating says why the backup is taken with the old binary: `openForCLI` in `cmd/holzcloud/cli.go` runs
  `db.RunMigrations` and takes no snapshot, so `holzcloud user list`, `check`, `backup` and `migrate status` of the new binary
  would migrate the old database silently. This was read in the code and was not played here; no step of the harness runs
  a subcommand of the new binary against the old database (the harness reads with `sqlite3 -readonly` instead).

## DEP-01: a fresh install by the document alone

What was played, in this order, each block exactly as it stands in `deploy/DEPLOY.md`:

1. *Prerequisites*: `apt update` and the `apt install -y` line.
2. *Build*, *Transfer to the server*, *Install*, *systemd Service*: the Build line is recorded as skipped on the
   server side, because `bins` had already played that same line in a clone of the repository to make the binary; the two
   `scp` lines are DEVIATION[scp]; the unit starts as shipped.
3. *First Run*: the verification block, then the dialogue headlessly.
4. *Caddy (Reverse Proxy + HTTPS)*: the install block, then the Caddyfile and a validation instead of the reload.
5. *Backups*: the unit and timer, the timer enabled, and one manual backup run as the service user.

Result of the second pass, from containers that had nothing but systemd:

- `install` ended in `INSTALL OK`: every counted step has status 0, the unit is active, `/healthz` and `/readyz` answer
  `ok` with `integrity` `ok`, and `/readyz` reports a stamped version and commit, not `dev`.
- `firstrun` ended in `FIRSTRUN OK` with five PASS lines: the setup form is open before an account exists (200), the setup
  post creates the account (303 to `/admin/`), the setup form is gone afterwards (404), the login works (303 to
  `/admin/`) and the first request after the login leads to `/admin/2fa/einrichten`. The generated password is in a file
  readable by root inside the container only; `LEAK-CHECK OK` says that no copy of it, and nothing shaped like an AI
  key, is in any output file.
- `postinstall` ended in `POSTINSTALL OK`: the Backups block ran without a failing step, a backup directory with a
  verified database was written, and every path of the form `/opt/holzcloud/deploy/NAME` that the document names exists
  and, for the scripts, is executable.
- `caddy` exited 0 with `CADDY: validated`: Caddy 2.11.7 from the repository installs and accepts the shipped Caddyfile.
- `deviations.txt` holds only DEVIATION[scp], DEVIATION[dropin], DEVIATION[browser], DEVIATION[caddy] and DEVIATION[mount].

**The final run** (`fresh`, `amd64-fresh`, Appendix R1), made against the final document in one command from a bare container:
`summary.txt` reads `DEP-01 PASS`, `CADDY: validated`, `STEPS: 33 played, 0 failed`, `DEVIATIONS: browser caddy dropin mount scp` and `LEAK-CHECK OK`.
`install.txt` shows the service answering with the stamped build (`v0.0.25-42-g2a11eeb`, commit `2a11eeb`, `integrity` `ok`) and the unit active;
`firstrun.txt` has its five PASS lines; `paths.txt` has seven PASS lines for the paths the document names under `/opt/holzcloud` and seven tools present (rsync,
gpg, fuser, curl, jq, sqlite3, openssl); `caddy.txt` shows Caddy 2.11.7 installed from the repository and `Valid configuration`. Step 26 is the
manual backup as the service user and ends in `backup written and verified`. The unit file, the document and the harness are the same in all three
runs of the set.

What this does not show: that a real server's DNS, firewall and certificate issuance work (DEVIATION[caddy] and
DEVIATION[network] stand for them), that the host kernel enforces the unit's syscall filter (phase 20 measured that),
and that a restore from the backup works. Phase 22 owns the proof of backup and restore; this phase only made the paths
and prerequisites that an install trips over correct.

## DEP-02: an update across migrations

**The old release.** `v0.0.10`, built by the Build line of the document in a clone at that tag. It has 58 migration files, the
current build 60, so two migrations lie between them:

- `00059_website_mail.sql` is structure only: it adds the table `website_mail`; nothing that exists is touched.
- `00060_order_token.sql` touches data: it adds `orders.token`, fills it for every existing order with
  `lower(hex(randomblob(16)))`, and adds a unique index `orders_token_idx`.

Seeding worked on `v0.0.10` as it stands, so no later tag was needed as a fallback. The old build was installed through the
repaired document (`INSTALL_FROM=old`), so the update starts from a server that the document made.

**State S0** (`baseline`, on the old binary): one website, Test, with the domain `test.local`; the five plugins `bestellung`,
`jahreszahl`, `kontaktformular`, `nicht-gefunden` and `suche` installed and enabled through the `/ai` interface by `seed`; the
published page `plugintest` that uses three of the plugins through their shortcodes; one uploaded image with its scaled variants; an admin-level AI key made by
`seed`, and a login user `login@example.test`; one order, `2026-9001`. What is a fixture and not a step of the document is
named DEVIATION[fixture] (see *Deviations named*): the plugin archives, the image, the login user and the order row. The login
password is generated inside the container, kept in a root-only file and passed on stdin; `LEAK-CHECK OK` after `baseline` and after
`update` says it occurs in no output file.

**The comparison** (`capture_state`, `compare_states`). Both states are written as lines of the form `KEY: value`, counts and
content hashes only, never a row of the users table, the sessions table or any table that holds a credential. It compares, between
S0 and S1:

- the row count of every table that exists in both schemas, with the table names read from `sqlite_master`, nothing hand-listed;
- the sha256 of the stored Markdown and HTML of the page `plugintest`, computed inside the container;
- the sha256 of the uploaded image file under `/opt/holzcloud/data/media`, which must also equal the sha256 of the file that was staged
  for the upload;
- a login through the form, `HTTP 303 /admin/`;
- `GET /plugintest` and `GET /` with `Host: test.local` answering 200, and the page title in the body;
- the number of plugins that `list_plugins` shows running with an empty `last_error`;
- the token of the existing order: absent as a column at schema 58, 32 characters at schema 60.

Tables whose rows move with traffic or with the update itself are not compared for equality. They are printed as `INFO volatile`:

| Table | Why its count may differ |
|-------|--------------------------|
| `sessions` | one row per login, deleted at expiry; the capture itself logs in |
| `activity_log` | the audit trail, one row per admin action |
| `mail_outbox`, `outbox` | queues that a background run drains |
| `plugin_store` | written by the plugins at run time, the 404 hook stores the missing path |
| `goose_db_version` | one row per applied migration, so it grows by exactly the migrations of the update (59 to 61 rows) |
| `oauth_codes` | an authorisation code lives for minutes and is used once |

A difference in any other table is a BAD and an investigation; none was found. `website_mail` exists only in the new schema and
is printed as `INFO new-only`.

**The comparison can fail.** The harness changes a copy of `state-s1.txt` (the count of `pages` plus one, the page hash with four
digits in front) and requires `compare_states` to report both. `compare-selftest.txt` holds the result: `BAD rows pages a=4 b=5`
and `BAD page sha256`, so a clean `compare-s0-s1.txt` means something.

**Result of the final run** (`lifecycle` with v0.0.10, `deploy/test/out/amd64-lifecycle`, Appendix R2; the second play of plan 21-03, from a fresh container as well, showed the same table):

| What was compared | Result |
|-------------------|--------|
| Row counts, 46 tables in both schemas and not volatile | 46 times `PASS rows`, no BAD |
| Page content (Markdown and HTML) | `PASS page sha256` |
| Media file, and against the staged source | `PASS media sha256` |
| Login | `PASS login` (`HTTP 303 /admin/` before and after) |
| Public page | `PASS public page` (`plugintest=200 root=200 title=yes` before and after) |
| Plugins | `PASS plugins running 5/5` |
| Order token | `PASS order token`: no token column at S0, 32 characters at S1 (the migration backfilled it) |
| Snapshot against S0 and against the live database, 46 tables | 46 times `PASS snapshot rows` |
| Schema of the snapshot and of the live database | `PASS snapshot schema 58`, `PASS live schema 60` |
| Snapshot file | `PASS snapshot mode holzcloud 600` |
| Backup directory of step 1 | `PASS backup mode holzcloud 700` |

`compare-s0-s1.txt` has 52 PASS lines and no BAD, `attach-s1.txt` 50 PASS lines and no BAD, and `update.txt` says `COMPARE: pass=52 bad=0` and
`ATTACH: pass=50 bad=0`. The files are in the Appendix (R2), unchanged; the scratch directories stay gitignored, and the command under
*How this was run* makes them again.

**The pre-upgrade snapshot, observed.** The service wrote it itself, before the first migration, and said so in the journal:
`pre-upgrade snapshot written` with `path` `/opt/holzcloud/data/pre-upgrade-v0.0.25-42-g2a11eeb-20261012-124115.sqlite`, followed by
`migration applied` for versions 59 and 60 (`update.txt`). The file name carries the version of the new binary (`v0.0.25-42-g2a11eeb`), it lies in the
data directory, it has mode 600 and belongs to `holzcloud`, and it holds schema 58 while the live database is at 60. The comparison
was made independently of the capture: one `sqlite3 -readonly` session as the service user on the live database with the snapshot
attached counted every table that both have. It covers the database only, which is why the backup of step 1, with media and
templates, is a separate thing. An update without pending migrations writes no snapshot.

**What the played section left out, and the fix.** The first play (pass 1, and pass 1b with the same result) showed `PREV: absent`,
`BACKUP: none`, the unstamped build line and no word about the snapshot or the check after the start. The five rows in *Findings* list
them. The repaired section has three bash blocks: keep the old binary, print its version and back up with the old binary; build,
transfer, stop, copy, start; check `/readyz` and read the journal. The final run shows `PREV: present`, the backup directory
`20261012-124114` (mode 700, written by the service user), `/readyz` with the new version and commit, and the journal lines of the
snapshot and the two migrations.

**What a subcommand of the new binary would have done.** `openForCLI` migrates the database it opens and takes no snapshot, so a back-up
made with the new binary, or a `holzcloud user list`, would have moved the schema first and left no copy of the old state. This
is read from the code, not played; it is why the section says to back up with the old binary and the harness never runs a
subcommand of the new binary against the old database.

**What this does not show.** That the way back works (plan 21-04 plays it, starting from the marker page `after-update` that `update`
writes after the comparison and captures as state `s1m`), how long a larger database takes to migrate, and that the real server's
clock, disk and mounts behave like the container's (DEVIATION[mount] stands for the mount).

## DEP-03: rolling back, and the migration limit

**What was played, in this order**, on the container that the update left behind:

1. The Updating section (as in DEP-02), then a marker page `after-update` written through the new binary and captured as state `s1m`.
2. With a snapshot: `SCENARIO[negative]` (the old binary against the migrated database, once), `SCENARIO[restore-new]` (the new binary put
   back), then block 1 of *Rolling back*, exactly as it stands in `deploy/DEPLOY.md`.
3. Without a snapshot: block 2 of *Rolling back* straight after the update. The harness chooses the block by the journal of the update: a
   `SNAPSHOT:` path in `update.txt` means block 1, `SNAPSHOT: none` means block 2.

The two earlier passes were played step by step (the final set is made by the two `lifecycle` commands under *How this was run*, which chain exactly these subcommands):

```text
PLATFORM=linux/amd64 TARGET=systemd-base INSTALL_FROM=old OLD_TAG=v0.0.10 NAME=holzcloud-rollback-pass1b OUT_DIR=deploy/test/out/amd64-rollback-pass1b ./deploy/test/run.sh build up install baseline update negative rollback
PLATFORM=linux/amd64 TARGET=systemd-base INSTALL_FROM=old OLD_TAG=v0.0.21 NAME=holzcloud-hop OUT_DIR=deploy/test/out/amd64-hop ./deploy/test/run.sh build up install baseline update rollback
```

**The rollback with a snapshot** (v0.0.10 to the current build, schema 58 to 60, the final run `deploy/test/out/amd64-lifecycle`, Appendix R2; the earlier pass `amd64-rollback-pass1b` showed the same table). The state after the
rollback (`s2`) is compared with the state before the update (`s0`), with the same comparison as for DEP-02:

| Check | Result |
|-------|--------|
| The old version runs again | `PASS version old v0.0.10` |
| The schema is the old one | `PASS schema 58` |
| The page written after the update is gone | `PASS marker gone (GET /after-update 404)` |
| The order is as it was before the migration | `PASS no token column (the order of the old schema, as it was)` |
| A page that existed before the update is served | `PASS pre-update page (GET /plugintest 200)` |
| The restored database belongs to the service user, mode 600 | `PASS owner and mode holzcloud 600` |
| The migrated database is kept and can be put back | `PASS set-aside copy holzcloud.sqlite.after-update at schema 60` |
| The restart migrates nothing and writes no snapshot | `PASS no migration on restart` |
| The service reports the database sound | `PASS integrity ok` |
| Everything of the update comparison, against `s0` | `compare-s0-s2.txt`: 52 PASS, no BAD |

**The negative case, verbatim** (`negative.txt` of the final run, identical to that of the earlier pass): the old binary was started against the database that the new one had
migrated, no snapshot restored.

```text
NEGATIVE-RESULT: starts
readyz: {"status":"ok","version":"v0.0.10","commit":"ec365d6","integrity":"ok"}
schema: 60 
journal: none
insert-1: ok
insert-2: Runtime error near line 2: UNIQUE constraint failed: orders.token (19)
orders: 2 
RESTORE: ok
```

It starts, says it is healthy and logs nothing about the schema. It fails at the second order, because migration 00060 made `orders.token` unique
and the old code does not write it. The inserts were direct, in the statement shape of the old code; the shop flow was not played. The result
is for this pair of versions. The document says so, and states the limit as a rule, because another pair can fail somewhere else, or nowhere
visible. This is why the limit is the first paragraph of *Rolling back*.

**The rollback without a snapshot** (v0.0.21 to the current build, the final run `deploy/test/out/amd64-lifecycle-hop`, Appendix R3; the earlier pass `amd64-hop` showed the same table). Both builds have 60 migration files, so
`update.txt` says `MIGRATIONS: old=60 new=60 pending=0`, `APPLIED: 0` and `SNAPSHOT: none`, and `attach-s1.txt` holds four PASS lines instead of the
ATTACH comparison, which needs a snapshot: no snapshot line in the journal since the start, no new pre-upgrade file in the data directory, no
migration applied, schema 60 before and after. The comparison of the update against the baseline has 53 PASS and no BAD. Then block 2 of
*Rolling back*, and `rollback.txt` with `MODE: binary`:

| Check | Result |
|-------|--------|
| The old version runs again | `PASS version old v0.0.21` |
| The schema is unchanged | `PASS schema 60` |
| The page written after the update is still there | `PASS marker kept (GET /after-update 200)` |
| The order token is as the update left it | `PASS token kept (0 characters, as after the update)` |
| The restart migrates nothing, and the database is sound | `PASS no migration on restart`, `PASS integrity ok` |
| Everything, against the state after the update (`s1m`) | `compare-s1m-s2.txt`: 53 PASS, no BAD |

So, with no migration between the builds, putting the old binary back alone undoes nothing and loses nothing; this is what assumption A5 of
the research claimed, and it is now observed for one pair of builds and no further. Two things the play cannot show: that the old code
reads rows that the newer code wrote in a new shape (the schema is the same, the meaning of a row need not be), and an update with more than
one release between the builds. The document says the first. The token length is 0 in that run, not 32: v0.0.21 already has the column, so the
seeded order was written by the old code without a token, and the check compares with the state after the update instead of expecting 32. An
earlier version of the harness expected 32 and reported that as BAD (`deploy/test/out/amd64-hop-pass1b`); that was a wrong expectation in the
check, not a defect of the document, and it is the only check that changed.

**What a rollback by snapshot undoes, and does not cover.** It brings back users, sessions and API keys as the snapshot had them: a password
changed since the update is the old password again, a revoked session or key is valid again. The counts of the restored state are in
`rollback.txt` (`INFO credentials`), and no credential is written to any output file (`LEAK-CHECK OK`). Sessions are rows of the database, so a
restart does not clear them; the document tells the operator to re-apply revocations in the admin. It does not cover files: media and
templates added after the update stay on disk and the restored database does not know them; files removed after the update are not brought
back, and the backup of step 1 of *Updating* is the source for those.

**Why the snapshot route and not `goose down`** (D-10). Every migration file has a `Down` section (60 of 60, counted with grep), but
`holzcloud migrate` offers `status` and `up` only: no command runs them. A rollback that the program cannot do is not offered. A `Down` that
nobody has run against production data is a second, untested way to lose data, while the snapshot is the program's own verified copy of the
state before the first migration. The document carries one developer-note sentence about the `Down` sections and no more.

**A trap that the play showed.** The first start of a fresh install writes a snapshot of an empty database (`pre-upgrade-v0.0.10-...sqlite`,
about 12 KB). The restore line of block 1 takes the newest snapshot by modification time, so after an update that wrote none it would restore
that empty one, or an older update's. The text says to run the first line of block 1 on its own and read it, and what the name and the size of
the right file look like.

**What differs from a real server, and matters to a rollback:**

- The container is a privileged systemd container; a server runs the unit under the host's own systemd. The commands of the section are the
  same. The start limit and `reset-failed` were exercised as the unit sets them (`StartLimitBurst`), but no run drove the service into the limit,
  so that a restart after a limit has been hit works is read from the unit and not observed.
- Under amd64 emulation the unit needs a container-only drop-in (DEVIATION[dropin]), and a start takes about ten seconds. The document's check line waits for the start itself, so this does not change the text, but the timing in the runs is not a server's.
- The data directory is a plain directory (DEVIATION[mount]), not a mount. Whether the restore, with a service stopped before a volume is
  touched, holds on a separate volume is not shown here.
- The journal of a fresh container holds everything since its start. A server may have rotated the line `pre-upgrade snapshot written` away
  by the time somebody rolls back; the file in the data directory, named after the version it was written for, is the second source, and the
  text names both.
- The database is small (test data only). How long a snapshot of a large database takes to write and to restore is not measured.

## Container vs. server

Everything above was played in a privileged Debian 12 container with systemd as PID 1, on an arm64 Docker Desktop VM, under amd64 emulation. This table names each
difference that matters to an install, an update or a rollback, with what the runs showed here and what a real server does.

The last column says how the claim that the result carries over to a real server is known. **measured**: the thing itself was observed in the runs of the evidence
set, and the server does not differ in kind. **inferred**: read from the code, the unit or another phase, or reasoned, and not observed in these runs. **assumed**: nothing in
these runs speaks for it; it is taken on trust, and it is the place to look first if a server behaves differently.

| Aspect | Measured here | Real server | Status |
|--------|---------------|-------------|--------|
| Architecture and the emulation layer | linux/amd64 containers on an aarch64 host (`versions.txt`: host architecture aarch64, container uname x86_64). The unit as shipped crashes at its first start under the emulation (the `systemctl status` of step 19 exits with 3 and shows an automatic restart after a TRAP signal), and runs with the container-only drop-in that sets `ProcSubset=all` (DEVIATION[dropin]) | A native x86-64 kernel. The unit is expected to start as shipped, without the drop-in; phase 20 saw it start unchanged on native arm64, and no native amd64 run exists | inferred |
| Seccomp and the unit's syscall filter | `Seccomp:` and `Seccomp_filters:` read 0 in the process status of the started service (`unit-start.txt`): the filter of the unit is not enforced under this emulation, so a green play says nothing about it | The kernel enforces the filter. Phase 20 measured that on native arm64 for `MemoryDenyWriteExecute` only | inferred |
| The data directory mount | A plain directory on the container's root filesystem, no mount of its own (DEVIATION[mount]); the unit's `RequiresMountsFor` line was not exercised, and a restore on a separate volume was not played | The data directory may lie on its own volume that must be mounted before the service starts, and a restore touches that volume with the service stopped | assumed |
| Package repositories and package lists | `apt update` and the one `apt install -y` line of Prerequisites succeed on Debian 12 (bookworm) from a bare image, and the Caddy repository is reachable (steps 1, 2, 27 to 31). The image ships `sqlite3`, `curl`, `jq` and `sudo` already | A VPS image has its own preinstalled set and may be another distribution, for example Ubuntu 24.04, with other package versions; the document names every package a later step uses, so a missing one is installed | inferred |
| Caddy installation and the Caddyfile | Caddy 2.11.7 installs from the repository, and `caddy validate` answers `Valid configuration` for the shipped Caddyfile (`CADDY: validated`) | The same install, then `systemctl reload caddy` | measured |
| Caddy, TLS, DNS and ACME | None of them exists here: no public domain, no DNS, no certificate issuance; the reload is replaced by the validation (DEVIATION[caddy]) | The reload with a Caddyfile that names the real domain starts certificate issuance, which needs DNS pointing at the server and open ports | assumed |
| First Run over plain http with `HOLZCLOUD_SECURE=true` | The shipped unit sets `HOLZCLOUD_SECURE=true`, so the forms are accepted only as if they came through TLS: the harness posts with a Referer that carries the https origin and the CSRF field read out of the page (DEVIATION[browser]); the plain post without it was answered 403 in pass 2 (see *Findings*) | The browser reaches the admin through the TLS proxy, with no Referer trick | inferred |
| The browser steps, replaced by curl | The setup, the login and the redirect after it are played with `curl` and judged by status codes and `Location` (`firstrun.txt`, five PASS lines); what the pages look like and how htmx behaves in them was not seen | An operator clicks through the same forms in a browser | assumed |
| Compulsory two-factor enrolment | The first request after the login answers a redirect to `/admin/2fa/einrichten` (`firstrun.txt`); the enrolment itself, with an authenticator app, was not played | The administrator enrols and keeps the recovery codes | assumed |
| The systemd version of the image | systemd 252 (252.39-1~deb12u2), from `versions.txt` | The version of the server's distribution; the commands of the document are plain `systemctl` and `journalctl` calls and none of them is tied to a version, which was read and not played on another version | assumed |
| Timing | Emulated, not a benchmark: the first start takes about ten seconds including the plugin compile, and the three runs took 35, 75 and about 60 seconds from `up` to the summary | A native start is expected to be faster. The `curl` of the check lines retries 30 times one second apart, which covers the emulated start with room; a slower disk could need more | inferred |
| Free disk for `backup.sh` | `/readyz` reports `disk_free_bytes` of about 402 GB (`http.txt`), the free space of the Docker VM, so the 1024 MB threshold of `backup.sh` (`MIN_FREE_MB`) was never near | On a small VPS the free space may be below the threshold, and then `backup.sh` refuses with an error; that was read in the script and not triggered | inferred |
| Journal retention | The journal of a fresh container holds everything since its start, so the update's `pre-upgrade snapshot written` line is always found; `SystemMaxUse=200M` was not applied | A server may have rotated the line away. The snapshot file in the data directory, named after the version it was written for, is the second source, and *Rolling back* names both | inferred |
| The privileged shared-kernel container and cgroups | A privileged container on the kernel of the Docker Desktop VM (7.0.14-linuxkit), with systemd as PID 1; memory and CPU figures in the status output are that VM's cgroup, not a server's | A server runs the unit under its own systemd on its own kernel and cgroup tree; the unit's resource directives were not tested for effect here | assumed |
| The backup timer | Enabled with `enable --now` and listed by `systemctl list-timers` (steps 24 and 25); it never fired in a run, and the backup was run by hand as the service user (step 26) | The timer fires at 03:15 plus a random delay and starts the backup unit; the unit run as a service by the timer was not observed | assumed |
| Size of the database | Test data only: `/readyz` reports a 4096-byte database file (`http.txt`). The two migrations of the update took 2 and 3 milliseconds (`update.txt`); the time of a restore was not measured | A production database is larger, and the time for a migration, a snapshot and a restore grows with it | assumed |

## Not measured

What no run of this phase observed, so that nobody reads the record as saying more:

- A native amd64 run: every run is amd64 under emulation on an arm64 host.
- A real VPS image and its package set, for example Ubuntu 24.04: the install was played on Debian 12 only.
- Certificate issuance and everything behind it: DNS, ACME, the TLS proxy reaching the service, a browser over https.
- Five or more successive updates and the pruning of the pre-upgrade snapshots (the newest five are kept; that was read, and no run made a sixth).
- A rollback of media and templates: the snapshot covers the database only; files added after the update stay on disk, and files removed are not brought back.
- The single sign-on paths (Authentik forward auth and OpenID Connect): the sections were not played here.
- A restore from a backup: `restore.sh` was not used in this phase, and the backup of step 1 of *Updating* was written and listed but never restored. Phase 22 owns that proof.
- A reboot of the container with the units enabled: the units were enabled and started, and no run restarted the machine.
- The compulsory two-factor enrolment itself, and the shop and form flows through a browser: orders were written straight into the database in the statement shape of the code.
- A rollback after a failed migration (a migration that stops half way): the update played here migrated cleanly, and the failed-migration path of *Recovery* is read from the code.
- A database of production size.

## Appendix: raw output

The files of the evidence set, pasted unchanged: every line is the line of the scratch file, with nothing retyped, trimmed or re-wrapped. Under each heading stands the scratch
directory and the command that makes it again; each file is a fenced block under its own path. Not pasted, on purpose: `seed.txt`, any `.log` file, any
`step-NN.txt` and the cookie jar. The generated passwords and the key exist only inside the throwaway containers, and `LEAK-CHECK OK` in each summary says that none of
them is in any output file. The pasted text was searched for key-shaped strings, 24-character hexadecimal words, cookie headers and password-hash prefixes
before it was committed. A line-by-line `grep -F` of every non-empty line of the 29 files against this document found no line missing.

### R1 fresh install

Scratch directory `deploy/test/out/amd64-fresh`, made by:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-fresh OUT_DIR=deploy/test/out/amd64-fresh ./deploy/test/run.sh fresh
```

#### amd64-fresh/summary.txt

```text
---- summary (linux/amd64) ----
DEP-01 PASS
CADDY: validated
STEPS: 33 played, 0 failed
DEVIATIONS: browser caddy dropin mount scp
LEAK-CHECK OK
```

#### amd64-fresh/steps.txt

```text
STEP 01 rc=0 sudo apt update
STEP 02 rc=0 sudo apt install -y ca-certificates curl jq sqlite3 rsync psmisc gnupg openssl
STEP 03 rc=0 SKIP[dev-machine] CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)" ./cmd/holzcloud
STEP 04 rc=0 scp holzcloud user@your-server:/tmp/holzcloud
STEP 05 rc=0 scp -r deploy/ user@your-server:/tmp/deploy/
STEP 06 rc=0 sudo useradd -r -s /usr/sbin/nologin holzcloud
STEP 07 rc=0 sudo mkdir -p /opt/holzcloud/data
STEP 08 rc=0 sudo mkdir -p /opt/holzcloud/backups
STEP 09 rc=0 sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
STEP 10 rc=0 sudo chmod 755 /opt/holzcloud/holzcloud
STEP 11 rc=0 sudo cp -r /tmp/deploy /opt/holzcloud/deploy
STEP 12 rc=0 sudo chmod 755 /opt/holzcloud/deploy/backup.sh /opt/holzcloud/deploy/restore.sh
STEP 13 rc=0 sudo chown -R holzcloud:holzcloud /opt/holzcloud/data /opt/holzcloud/backups
STEP 14 rc=0 sudo chmod 700 /opt/holzcloud/backups
STEP 15 rc=0 sudo cp /tmp/deploy/holzcloud.service /etc/systemd/system/holzcloud.service
STEP 16 rc=0 sudo systemctl daemon-reload
STEP 17 rc=0 sudo systemctl enable holzcloud
STEP 18 rc=0 sudo systemctl start holzcloud
STEP 19 rc=3 sudo systemctl status holzcloud
STEP 20 rc=0 sudo systemctl status holzcloud
STEP 21 rc=0 curl -fsS --retry 30 --retry-connrefused --retry-delay 1 http://127.0.0.1:8080/readyz | jq '{status, version, commit, integrity}'
STEP 22 rc=0 sudo cp /opt/holzcloud/deploy/holzcloud-backup.{service,timer} /etc/systemd/system/
STEP 23 rc=0 sudo systemctl daemon-reload
STEP 24 rc=0 sudo systemctl enable --now holzcloud-backup.timer
STEP 25 rc=0 systemctl list-timers holzcloud-backup.timer
STEP 26 rc=0 sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
STEP 27 rc=0 sudo apt update
STEP 28 rc=0 sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https gnupg curl
STEP 29 rc=0 curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
STEP 30 rc=0 curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
STEP 31 rc=0 sudo apt update && sudo apt install -y caddy
STEP 32 rc=0 sudo cp /tmp/deploy/Caddyfile.example /etc/caddy/Caddyfile
STEP 33 rc=0 sudo systemctl reload caddy
```

#### amd64-fresh/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-2a11eeb to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[browser]: the document's First Run is played with curl and not in a browser. The shipped unit sets HOLZCLOUD_SECURE=true, so the form is only accepted as if it came through the TLS proxy: a post over plain http needs a Referer header that carries the https origin, and the hidden CSRF field is read out of the page
DEVIATION[caddy]: 'sudo systemctl reload caddy' was replaced by 'caddy validate --config /etc/caddy/Caddyfile': the container has no public domain, no DNS and no ACME, and a real reload would start certificate issuance for example.com
```

#### amd64-fresh/versions.txt

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```

#### amd64-fresh/install.txt

```text
readyz: {"status":"ok","version":"v0.0.25-42-g2a11eeb","commit":"2a11eeb","integrity":"ok"}
binary: holzcloud v0.0.25-42-g2a11eeb (2a11eeb)
unit: active
```

#### amd64-fresh/http.txt

```text
== /healthz ==
{"status":"ok"}
== /readyz ==
{"status":"ok","version":"v0.0.25-42-g2a11eeb","commit":"2a11eeb","uptime_s":0,"integrity":"ok","disk_free_bytes":402084728832,"database":{"freelist_count":0,"page_count":155,"size_bytes":4096,"wal_bytes":2241312}}
```

#### amd64-fresh/firstrun.txt

```text
PASS setup form open before any account: GET /admin/setup answers 200
PASS setup POST creates the account: 303 to /admin/
PASS setup form closed afterwards: GET /admin/setup answers 404
PASS login POST: 303 to /admin/
PASS first request after the login: 303 to /admin/2fa/einrichten (compulsory two-factor enrolment)
```

#### amd64-fresh/caddy.txt

```text
== step 27 ==
$ sudo apt update

WARNING: apt does not have a stable CLI interface. Use with caution in scripts.

Hit:1 http://deb.debian.org/debian bookworm InRelease
Hit:2 http://deb.debian.org/debian bookworm-updates InRelease
Hit:3 http://deb.debian.org/debian-security bookworm-security InRelease
Reading package lists...
Building dependency tree...
Reading state information...
1 package can be upgraded. Run 'apt list --upgradable' to see it.
STEP 27 rc=0 sudo apt update

== step 28 ==
$ sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https gnupg curl

WARNING: apt does not have a stable CLI interface. Use with caution in scripts.

Reading package lists...
Building dependency tree...
Reading state information...
debian-archive-keyring is already the newest version (2023.3+deb12u2).
debian-archive-keyring set to manually installed.
gnupg is already the newest version (2.2.40-1.1+deb12u2).
curl is already the newest version (7.88.1-10+deb12u15).
The following NEW packages will be installed:
  apt-transport-https debian-keyring
0 upgraded, 2 newly installed, 0 to remove and 1 not upgraded.
Need to get 30.5 MB of archives.
After this operation, 32.0 MB of additional disk space will be used.
Get:1 http://deb.debian.org/debian bookworm/main amd64 apt-transport-https all 2.6.1 [25.2 kB]
Get:2 http://deb.debian.org/debian bookworm/main amd64 debian-keyring all 2022.12.24 [30.5 MB]
debconf: delaying package configuration, since apt-utils is not installed
Fetched 30.5 MB in 3s (11.9 MB/s)
Selecting previously unselected package apt-transport-https.
(Reading database ... (Reading database ... 5%(Reading database ... 10%(Reading database ... 15%(Reading database ... 20%(Reading database ... 25%(Reading database ... 30%(Reading database ... 35%(Reading database ... 40%(Reading database ... 45%(Reading database ... 50%(Reading database ... 55%(Reading database ... 60%(Reading database ... 65%(Reading database ... 70%(Reading database ... 75%(Reading database ... 80%(Reading database ... 85%(Reading database ... 90%(Reading database ... 95%(Reading database ... 100%(Reading database ... 8564 files and directories currently installed.)
Preparing to unpack .../apt-transport-https_2.6.1_all.deb ...
Unpacking apt-transport-https (2.6.1) ...
Selecting previously unselected package debian-keyring.
Preparing to unpack .../debian-keyring_2022.12.24_all.deb ...
Unpacking debian-keyring (2022.12.24) ...
Setting up apt-transport-https (2.6.1) ...
Setting up debian-keyring (2022.12.24) ...
STEP 28 rc=0 sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https gnupg curl

== step 29 ==
$ curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
STEP 29 rc=0 curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg

== step 30 ==
$ curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
# Source: Caddy
# Site: https://github.com/caddyserver/caddy
# Repository: Caddy / stable
# Description: Fast, multi-platform web server with automatic HTTPS


deb [signed-by=/usr/share/keyrings/caddy-stable-archive-keyring.gpg] https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main

deb-src [signed-by=/usr/share/keyrings/caddy-stable-archive-keyring.gpg] https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main
STEP 30 rc=0 curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list

== step 31 ==
$ sudo apt update && sudo apt install -y caddy

WARNING: apt does not have a stable CLI interface. Use with caution in scripts.

Hit:1 http://deb.debian.org/debian bookworm InRelease
Get:2 https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version InRelease [14.8 kB]
Hit:3 http://deb.debian.org/debian bookworm-updates InRelease
Hit:4 http://deb.debian.org/debian-security bookworm-security InRelease
Get:5 https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version/main amd64 Packages [5081 B]
Fetched 19.9 kB in 1s (15.7 kB/s)
Reading package lists...
Building dependency tree...
Reading state information...
1 package can be upgraded. Run 'apt list --upgradable' to see it.

WARNING: apt does not have a stable CLI interface. Use with caution in scripts.

Reading package lists...
Building dependency tree...
Reading state information...
The following NEW packages will be installed:
  caddy
0 upgraded, 1 newly installed, 0 to remove and 1 not upgraded.
Need to get 18.1 MB of archives.
After this operation, 52.0 MB of additional disk space will be used.
Get:1 https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version/main amd64 caddy amd64 2.11.7 [18.1 MB]
debconf: delaying package configuration, since apt-utils is not installed
Fetched 18.1 MB in 2s (11.9 MB/s)
Selecting previously unselected package caddy.
(Reading database ... (Reading database ... 5%(Reading database ... 10%(Reading database ... 15%(Reading database ... 20%(Reading database ... 25%(Reading database ... 30%(Reading database ... 35%(Reading database ... 40%(Reading database ... 45%(Reading database ... 50%(Reading database ... 55%(Reading database ... 60%(Reading database ... 65%(Reading database ... 70%(Reading database ... 75%(Reading database ... 80%(Reading database ... 85%(Reading database ... 90%(Reading database ... 95%(Reading database ... 100%(Reading database ... 8578 files and directories currently installed.)
Preparing to unpack .../caddy_2.11.7_amd64.deb ...
Unpacking caddy (2.11.7) ...
Setting up caddy (2.11.7) ...
Created symlink /etc/systemd/system/multi-user.target.wants/caddy.service → /lib/systemd/system/caddy.service.
/usr/sbin/policy-rc.d returned 101, not running 'start caddy.service'
STEP 31 rc=0 sudo apt update && sudo apt install -y caddy

== step 32 ==
$ sudo cp /tmp/deploy/Caddyfile.example /etc/caddy/Caddyfile
STEP 32 rc=0 sudo cp /tmp/deploy/Caddyfile.example /etc/caddy/Caddyfile

== step 33 ==
$ sudo systemctl reload caddy
{"level":"info","ts":1791808841.0301318,"msg":"using config from file","file":"/etc/caddy/Caddyfile"}
{"level":"info","ts":1791808841.0403218,"msg":"adapted config to JSON","adapter":"caddyfile"}
{"level":"info","ts":1791808841.042904,"logger":"tls.cache.maintenance","msg":"started background certificate maintenance","cache":"0x305230a50500"}
{"level":"info","ts":1791808841.0445275,"logger":"http.auto_https","msg":"server is listening only on the HTTPS port but has no TLS connection policies; adding one to enable TLS","server_name":"srv0","https_port":443}
{"level":"info","ts":1791808841.0450346,"logger":"http.auto_https","msg":"enabling automatic HTTP->HTTPS redirects","server_name":"srv0"}
{"level":"info","ts":1791808841.047815,"logger":"http","msg":"servers shutting down with eternal grace period"}
{"level":"info","ts":1791808841.0487406,"logger":"tls.cache.maintenance","msg":"stopped background certificate maintenance","cache":"0x305230a50500"}
Valid configuration
STEP 33 rc=0 sudo systemctl reload caddy

CADDY: validated
```

#### amd64-fresh/paths.txt

```text
PASS exists /opt/holzcloud/holzcloud
PASS exists /opt/holzcloud/data
PASS exists /opt/holzcloud/backups
PASS exists /opt/holzcloud/deploy/backup.sh
PASS exists /opt/holzcloud/deploy/holzcloud-backup.service
PASS exists /opt/holzcloud/deploy/holzcloud-backup.timer
PASS exists /opt/holzcloud/deploy/restore.sh
TOOL rsync present
TOOL gpg present
TOOL fuser present
TOOL curl present
TOOL jq present
TOOL sqlite3 present
TOOL openssl present
```

### R2 lifecycle with v0.0.10

Scratch directory `deploy/test/out/amd64-lifecycle`, made by:

```text
PLATFORM=linux/amd64 TARGET=systemd-base OLD_TAG=v0.0.10 NAME=holzcloud-lifecycle OUT_DIR=deploy/test/out/amd64-lifecycle ./deploy/test/run.sh lifecycle
```

#### amd64-lifecycle/summary.txt

```text
---- summary (linux/amd64) ----
OLD: v0.0.10 ec365d6
NEW: v0.0.25-42-g2a11eeb 2a11eeb
MIGRATIONS: old=58 new=60 pending=2
SNAPSHOT: /opt/holzcloud/data/pre-upgrade-v0.0.25-42-g2a11eeb-20261012-124115.sqlite
MODE: snapshot
NEGATIVE-RESULT: starts
DEP-02 PASS
DEP-03 PASS
LEAK-CHECK OK
```

#### amd64-lifecycle/steps.txt

```text
STEP 01 rc=0 sudo apt update
STEP 02 rc=0 sudo apt install -y ca-certificates curl jq sqlite3 rsync psmisc gnupg openssl
STEP 03 rc=0 SKIP[dev-machine] CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)" ./cmd/holzcloud
STEP 04 rc=0 scp holzcloud user@your-server:/tmp/holzcloud
STEP 05 rc=0 scp -r deploy/ user@your-server:/tmp/deploy/
STEP 06 rc=0 sudo useradd -r -s /usr/sbin/nologin holzcloud
STEP 07 rc=0 sudo mkdir -p /opt/holzcloud/data
STEP 08 rc=0 sudo mkdir -p /opt/holzcloud/backups
STEP 09 rc=0 sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
STEP 10 rc=0 sudo chmod 755 /opt/holzcloud/holzcloud
STEP 11 rc=0 sudo cp -r /tmp/deploy /opt/holzcloud/deploy
STEP 12 rc=0 sudo chmod 755 /opt/holzcloud/deploy/backup.sh /opt/holzcloud/deploy/restore.sh
STEP 13 rc=0 sudo chown -R holzcloud:holzcloud /opt/holzcloud/data /opt/holzcloud/backups
STEP 14 rc=0 sudo chmod 700 /opt/holzcloud/backups
STEP 15 rc=0 sudo cp /tmp/deploy/holzcloud.service /etc/systemd/system/holzcloud.service
STEP 16 rc=0 sudo systemctl daemon-reload
STEP 17 rc=0 sudo systemctl enable holzcloud
STEP 18 rc=0 sudo systemctl start holzcloud
STEP 19 rc=3 sudo systemctl status holzcloud
STEP 20 rc=0 sudo cp -p /opt/holzcloud/holzcloud /opt/holzcloud/holzcloud.prev
STEP 21 rc=0 /opt/holzcloud/holzcloud version
STEP 22 rc=0 sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
STEP 23 rc=0 SKIP[dev-machine] CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)" ./cmd/holzcloud
STEP 24 rc=0 scp holzcloud user@your-server:/tmp/holzcloud
STEP 25 rc=0 sudo systemctl stop holzcloud
STEP 26 rc=0 sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
STEP 27 rc=0 sudo systemctl start holzcloud
STEP 28 rc=0 curl -fsS --retry 30 --retry-connrefused --retry-delay 1 http://127.0.0.1:8080/readyz | jq '{status, version, commit, integrity}'
STEP 29 rc=0 sudo journalctl -u holzcloud --since "$(systemctl show -p ActiveEnterTimestamp --value holzcloud)" -o cat | grep -E 'pre-upgrade snapshot written|migration applied' || echo 'no migration, no snapshot'
STEP 30 rc=0 ls -l /opt/holzcloud/data/pre-upgrade-*.sqlite
STEP 31 rc=0 sudo systemctl stop holzcloud
STEP 32 rc=0 sudo cp -p /opt/holzcloud/holzcloud.prev /opt/holzcloud/holzcloud
STEP 33 rc=0 sudo rm -f /opt/holzcloud/data/holzcloud.sqlite-wal /opt/holzcloud/data/holzcloud.sqlite-shm
STEP 34 rc=0 sudo mv /opt/holzcloud/data/holzcloud.sqlite /opt/holzcloud/data/holzcloud.sqlite.after-update
STEP 35 rc=0 sudo -u holzcloud cp "$(ls -t /opt/holzcloud/data/pre-upgrade-*.sqlite | head -n 1)" /opt/holzcloud/data/holzcloud.sqlite
STEP 36 rc=0 sudo systemctl reset-failed holzcloud
STEP 37 rc=0 sudo systemctl start holzcloud
STEP 38 rc=0 curl -fsS --retry 30 --retry-connrefused --retry-delay 1 http://127.0.0.1:8080/readyz | jq '{status, version, commit, integrity}'
STEP 39 rc=0 sudo journalctl -u holzcloud --since "$(systemctl show -p ActiveEnterTimestamp --value holzcloud)" -o cat | grep -E 'pre-upgrade snapshot written|migration applied' || echo 'no migration, no snapshot'
```

#### amd64-lifecycle/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-old-v0.0.10 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[fixture]: the plugin archives, the test image, the login user and the order row are test fixtures that stand in for operator work in the admin, not steps of the document
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-2a11eeb to /tmp/holzcloud in the container (there is no ssh and no second machine here)
```

#### amd64-lifecycle/scenario.txt

```text
SCENARIO[negative]: the OLD binary (holzcloud.prev) is started against the database the NEW binary migrated, with no snapshot restored. The document tells nobody to do this; it is played once to find out what happens
SCENARIO[restore-new]: the NEW binary is put back (docker cp to /tmp/holzcloud, stop, copy, start), so the update state is as the update left it, plus the order rows the old code wrote
```

#### amd64-lifecycle/versions.txt

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```

#### amd64-lifecycle/baseline.txt

```text
OLD_TAG: v0.0.10
VERSION: holzcloud v0.0.10 (ec365d6)
WEBSITE: 1
DOMAIN: test.local
PAGE: 4 plugintest (published)
PLUGINS: bestellung jahreszahl kontaktformular nicht-gefunden suche
USER: login@example.test (admin, created with holzcloud user create on the old binary)
ORDER: 2026-9001 (a direct INSERT in the statement shape of the old code, not the shop flow)
```

#### amd64-lifecycle/update.txt

```text
OLD: v0.0.10 ec365d6
NEW: v0.0.25-42-g2a11eeb 2a11eeb
MIGRATIONS: old=58 new=60 pending=2
APPLIED: 2
SNAPSHOT: /opt/holzcloud/data/pre-upgrade-v0.0.25-42-g2a11eeb-20261012-124115.sqlite
SCHEMA: 58 -> 60
PREV: present
BACKUP: 20261012-124114
COMPARE: pass=52 bad=0
ATTACH: pass=50 bad=0
JOURNAL: {"time":"2026-10-12T12:41:15.98078268Z","level":"INFO","msg":"pre-upgrade snapshot written","path":"/opt/holzcloud/data/pre-upgrade-v0.0.25-42-g2a11eeb-20261012-124115.sqlite"}
JOURNAL: {"time":"2026-10-12T12:41:15.99293768Z","level":"INFO","msg":"migration applied","version":59,"type":"sql","duration_ms":2}
JOURNAL: {"time":"2026-10-12T12:41:15.993759972Z","level":"INFO","msg":"migration applied","version":60,"type":"sql","duration_ms":3}
MARKER: ok
```

#### amd64-lifecycle/state-s0.txt

```text
identity: {"status":"ok","version":"v0.0.10","commit":"ec365d6","integrity":"ok"}
binary: holzcloud v0.0.10 (ec365d6)
schema: 58
table activity_log 18
table ai_tokens 1
table album_items 0
table albums 0
table app_settings 0
table block_types 0
table cart_items 0
table carts 0
table content_types 0
table csv_imports 0
table form_messages 0
table goose_db_version 59
table mail_outbox 0
table media 1
table media_usage 0
table media_variants 2
table menu_items 3
table menus 2
table oauth_clients 0
table oauth_codes 0
table order_items 0
table orders 1
table outbox 0
table page_field_defs 0
table page_revisions 0
table page_terms 0
table pages 4
table pages_fts 4
table pages_fts_config 1
table pages_fts_data 7
table pages_fts_docsize 4
table pages_fts_idx 5
table plugin_migrations 1
table plugin_store 0
table plugin_websites 5
table plugins 5
table product_media 0
table product_terms 0
table products 0
table redirects 0
table saved_views 0
table sessions 0
table snippets 0
table templates 8
table terms 0
table theme_wording 0
table user_recovery_codes 0
table user_tokens 0
table user_websites 0
table users 1
table website_domains 1
table website_templates 1
table websites 1
page: 722cb2f1d7d8ab8fb9c875251cf546d261f0225b006bb68d898d0791b1d27092 ae73077bf91d27d2a9d8dd51f505a309f5b99e12047552819fef5adeab1e2804
media: 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577 source 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
order: 2026-9001 no-token-column
owner: holzcloud 600
public: plugintest=200 root=200 title=yes
plugins: 5
login: HTTP 303 /admin/
```

#### amd64-lifecycle/state-s1.txt

```text
identity: {"status":"ok","version":"v0.0.25-42-g2a11eeb","commit":"2a11eeb","integrity":"ok"}
binary: holzcloud v0.0.25-42-g2a11eeb (2a11eeb)
schema: 60
table activity_log 19
table ai_tokens 1
table album_items 0
table albums 0
table app_settings 0
table block_types 0
table cart_items 0
table carts 0
table content_types 0
table csv_imports 0
table form_messages 0
table goose_db_version 61
table mail_outbox 0
table media 1
table media_usage 0
table media_variants 2
table menu_items 3
table menus 2
table oauth_clients 0
table oauth_codes 0
table order_items 0
table orders 1
table outbox 0
table page_field_defs 0
table page_revisions 0
table page_terms 0
table pages 4
table pages_fts 4
table pages_fts_config 1
table pages_fts_data 7
table pages_fts_docsize 4
table pages_fts_idx 5
table plugin_migrations 1
table plugin_store 2
table plugin_websites 5
table plugins 5
table product_media 0
table product_terms 0
table products 0
table redirects 0
table saved_views 0
table sessions 1
table snippets 0
table templates 8
table terms 0
table theme_wording 0
table user_recovery_codes 0
table user_tokens 0
table user_websites 0
table users 1
table website_domains 1
table website_mail 0
table website_templates 1
table websites 1
page: 722cb2f1d7d8ab8fb9c875251cf546d261f0225b006bb68d898d0791b1d27092 ae73077bf91d27d2a9d8dd51f505a309f5b99e12047552819fef5adeab1e2804
media: 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577 source 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
order: 2026-9001 32
owner: holzcloud 600
public: plugintest=200 root=200 title=yes
plugins: 5
login: HTTP 303 /admin/
```

#### amd64-lifecycle/state-s2.txt

```text
identity: {"status":"ok","version":"v0.0.10","commit":"ec365d6","integrity":"ok"}
binary: holzcloud v0.0.10 (ec365d6)
schema: 58
table activity_log 19
table ai_tokens 1
table album_items 0
table albums 0
table app_settings 0
table block_types 0
table cart_items 0
table carts 0
table content_types 0
table csv_imports 0
table form_messages 0
table goose_db_version 59
table mail_outbox 0
table media 1
table media_usage 0
table media_variants 2
table menu_items 3
table menus 2
table oauth_clients 0
table oauth_codes 0
table order_items 0
table orders 1
table outbox 0
table page_field_defs 0
table page_revisions 0
table page_terms 0
table pages 4
table pages_fts 4
table pages_fts_config 1
table pages_fts_data 7
table pages_fts_docsize 4
table pages_fts_idx 5
table plugin_migrations 1
table plugin_store 2
table plugin_websites 5
table plugins 5
table product_media 0
table product_terms 0
table products 0
table redirects 0
table saved_views 0
table sessions 1
table snippets 0
table templates 8
table terms 0
table theme_wording 0
table user_recovery_codes 0
table user_tokens 0
table user_websites 0
table users 1
table website_domains 1
table website_templates 1
table websites 1
page: 722cb2f1d7d8ab8fb9c875251cf546d261f0225b006bb68d898d0791b1d27092 ae73077bf91d27d2a9d8dd51f505a309f5b99e12047552819fef5adeab1e2804
media: 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577 source 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
order: 2026-9001 no-token-column
owner: holzcloud 600
public: plugintest=200 root=200 title=yes
plugins: 5
login: HTTP 303 /admin/
```

#### amd64-lifecycle/compare-s0-s1.txt

```text
# compare-states A=deploy/test/out/amd64-lifecycle/state-s0.txt B=deploy/test/out/amd64-lifecycle/state-s1.txt
INFO volatile activity_log a=18 b=19
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version a=59 b=61
INFO volatile mail_outbox a=0 b=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes a=0 b=0
PASS rows order_items
PASS rows orders
INFO volatile outbox a=0 b=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
PASS rows pages
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store a=0 b=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions a=0 b=1
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_templates
PASS rows websites
INFO new-only website_mail b=0
PASS page sha256
PASS media sha256
PASS login
PASS public page
PASS plugins running 5/5
PASS order token (the migration backfilled a 32-character token)
INFO binary a='holzcloud v0.0.10 (ec365d6)' b='holzcloud v0.0.25-42-g2a11eeb (2a11eeb)'
INFO schema a=58 b=60
```

#### amd64-lifecycle/attach-s1.txt

```text
PASS snapshot schema 58 (the old schema, as captured in S0)
PASS live schema 60 (the new schema, ahead of the snapshot)
PASS snapshot mode holzcloud 600 (user and octal mode, T-21-10)
INFO snapshot volatile activity_log snapshot=19 live=20
PASS snapshot rows ai_tokens
PASS snapshot rows album_items
PASS snapshot rows albums
PASS snapshot rows app_settings
PASS snapshot rows block_types
PASS snapshot rows cart_items
PASS snapshot rows carts
PASS snapshot rows content_types
PASS snapshot rows csv_imports
PASS snapshot rows form_messages
INFO snapshot volatile goose_db_version snapshot=59 live=61
INFO snapshot volatile mail_outbox snapshot=0 live=0
PASS snapshot rows media
PASS snapshot rows media_usage
PASS snapshot rows media_variants
PASS snapshot rows menu_items
PASS snapshot rows menus
PASS snapshot rows oauth_clients
INFO snapshot volatile oauth_codes snapshot=0 live=0
PASS snapshot rows order_items
PASS snapshot rows orders
INFO snapshot volatile outbox snapshot=0 live=0
PASS snapshot rows page_field_defs
PASS snapshot rows page_revisions
PASS snapshot rows page_terms
PASS snapshot rows pages
PASS snapshot rows pages_fts
PASS snapshot rows pages_fts_config
PASS snapshot rows pages_fts_data
PASS snapshot rows pages_fts_docsize
PASS snapshot rows pages_fts_idx
PASS snapshot rows plugin_migrations
INFO snapshot volatile plugin_store snapshot=2 live=2
PASS snapshot rows plugin_websites
PASS snapshot rows plugins
PASS snapshot rows product_media
PASS snapshot rows product_terms
PASS snapshot rows products
PASS snapshot rows redirects
PASS snapshot rows saved_views
INFO snapshot volatile sessions snapshot=1 live=2
PASS snapshot rows snippets
PASS snapshot rows templates
PASS snapshot rows terms
PASS snapshot rows theme_wording
PASS snapshot rows user_recovery_codes
PASS snapshot rows user_tokens
PASS snapshot rows user_websites
PASS snapshot rows users
PASS snapshot rows website_domains
PASS snapshot rows website_templates
PASS snapshot rows websites
PASS backup mode holzcloud 700 (20261012-124114)
```

#### amd64-lifecycle/negative.txt

```text
NEGATIVE-RESULT: starts
readyz: {"status":"ok","version":"v0.0.10","commit":"ec365d6","integrity":"ok"}
schema: 60 
journal: none
insert-1: ok
insert-2: Runtime error near line 2: UNIQUE constraint failed: orders.token (19)
orders: 2 
RESTORE: ok
```

#### amd64-lifecycle/rollback.txt

```text
MODE: snapshot
PASS version old v0.0.10
PASS schema 58
PASS marker gone (GET /after-update 404)
PASS no token column (the order of the old schema, as it was)
PASS pre-update page (GET /plugintest 200)
PASS owner and mode holzcloud 600
PASS set-aside copy holzcloud.sqlite.after-update at schema 60
PASS no migration on restart
PASS integrity ok
INFO credentials users=1 sessions=1 ai_tokens=1 user_tokens=0 (counts in the restored state; state-s0 had users=1 ai_tokens=1)
INFO compare deploy/test/out/amd64-lifecycle/compare-s0-s2.txt pass=52 bad=0
```

#### amd64-lifecycle/compare-s0-s2.txt

```text
# compare-states A=deploy/test/out/amd64-lifecycle/state-s0.txt B=deploy/test/out/amd64-lifecycle/state-s2.txt
INFO volatile activity_log a=18 b=19
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version a=59 b=59
INFO volatile mail_outbox a=0 b=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes a=0 b=0
PASS rows order_items
PASS rows orders
INFO volatile outbox a=0 b=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
PASS rows pages
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store a=0 b=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions a=0 b=1
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_templates
PASS rows websites
PASS page sha256
PASS media sha256
PASS login
PASS public page
PASS plugins running 5/5
PASS order token (no token column in either schema, nothing migrated)
INFO binary a='holzcloud v0.0.10 (ec365d6)' b='holzcloud v0.0.10 (ec365d6)'
INFO schema a=58 b=58
```

### R3 lifecycle hop with v0.0.21

Scratch directory `deploy/test/out/amd64-lifecycle-hop`, made by:

```text
PLATFORM=linux/amd64 TARGET=systemd-base OLD_TAG=v0.0.21 NAME=holzcloud-lifecycle-hop OUT_DIR=deploy/test/out/amd64-lifecycle-hop ./deploy/test/run.sh lifecycle
```

#### amd64-lifecycle-hop/summary.txt

```text
---- summary (linux/amd64) ----
OLD: v0.0.21 76d99c3
NEW: v0.0.25-42-g2a11eeb 2a11eeb
MIGRATIONS: old=60 new=60 pending=0
SNAPSHOT: none
MODE: binary
DEP-02 not applicable (no migration between these builds)
DEP-03 PASS
LEAK-CHECK OK
```

#### amd64-lifecycle-hop/update.txt

```text
OLD: v0.0.21 76d99c3
NEW: v0.0.25-42-g2a11eeb 2a11eeb
MIGRATIONS: old=60 new=60 pending=0
APPLIED: 0
SNAPSHOT: none
SCHEMA: 60 -> 60
PREV: present
BACKUP: 20261012-124232
COMPARE: pass=53 bad=0
ATTACH: pass=4 bad=0
MARKER: ok
```

#### amd64-lifecycle-hop/attach-s1.txt

```text
PASS no snapshot (the journal has no 'pre-upgrade snapshot written' line since the start of the update)
PASS no new pre-upgrade file in the data directory (1 before, 1 after)
PASS no migration applied in the journal
PASS schema unchanged (60 -> 60)
```

#### amd64-lifecycle-hop/rollback.txt

```text
MODE: binary
PASS version old v0.0.21
PASS schema 60
PASS marker kept (GET /after-update 200)
PASS token kept (0 characters, as after the update)
PASS pre-update page (GET /plugintest 200)
PASS owner and mode holzcloud 600
PASS no migration on restart
PASS integrity ok
INFO credentials users=1 sessions=3 ai_tokens=1 user_tokens=0 (counts in the restored state; state-s0 had users=1 ai_tokens=1)
INFO compare deploy/test/out/amd64-lifecycle-hop/compare-s1m-s2.txt pass=53 bad=0
```

#### amd64-lifecycle-hop/compare-s1m-s2.txt

```text
# compare-states A=deploy/test/out/amd64-lifecycle-hop/state-s1m.txt B=deploy/test/out/amd64-lifecycle-hop/state-s2.txt
INFO volatile activity_log a=20 b=21
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version a=61 b=61
INFO volatile mail_outbox a=0 b=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes a=0 b=0
PASS rows order_items
PASS rows orders
INFO volatile outbox a=0 b=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
PASS rows pages
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store a=2 b=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions a=2 b=3
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_mail
PASS rows website_templates
PASS rows websites
PASS page sha256
PASS media sha256
PASS login
PASS public page
PASS plugins running 5/5
PASS order token (length 0 in both, as it was)
INFO binary a='holzcloud v0.0.25-42-g2a11eeb (2a11eeb)' b='holzcloud v0.0.21 (76d99c3)'
INFO schema a=60 b=60
```
