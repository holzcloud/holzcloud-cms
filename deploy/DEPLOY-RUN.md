# Install, update and rollback by the document alone: measured results

This document is the permanent record of phase 21. It records what happened when
`deploy/DEPLOY.md` was played literally, command by command, in a bare systemd Debian container, what
that play cannot say about a real server, and which defects it found in the document and how they were
fixed. Nothing was tuned to look better: a defect is fixed in `deploy/DEPLOY.md` and listed here with the
output that showed it. A deviation is only ever a thing the container cannot do, never a defect that was
renamed.

Later plans of the phase append their own sections after the DEP-01 section and add rows to the Summary and to
Findings.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| DEP-01: a fresh install by the document alone | PASS after repair. The first pass found ten defects in the document. All are fixed in `deploy/DEPLOY.md`. The second pass, from a fresh container, installs, sets up, audits and validates Caddy, and shows only the named deviations. | linux/amd64 (emulated on an arm64 host), Debian 12, systemd 252 | 2026-10-12, 58ae902 with the repair of plan 21-02 |
| DEP-02: an update across migrations | PASS after repair. v0.0.10 (58 migrations) updated to the current build (60) through the Updating section: 46 content tables with equal row counts, equal page and media hashes, login, public page, five running plugins and a filled order token, and an independent comparison against the service's own pre-upgrade snapshot, with no BAD line. The as-shipped section was silent in five places. All are fixed, and the second play from a fresh container shows the old binary kept, a backup taken and the snapshot recorded. | linux/amd64 (emulated on an arm64 host), Debian 12, systemd 252 | 2026-10-12, b7a6c8d with the repair of plan 21-03 |

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

A second word sits beside DEVIATION: **SCENARIO**. A deviation replaces a line of the document that the container
cannot play. A scenario is a step the harness performs that the document must not contain, because it plays what
the document tells nobody to do: the older binary started against a migrated database (`SCENARIO[negative]`), and the
new binary put back afterwards so that the chain can go on (`SCENARIO[restore-new]`). Scenarios go to `scenario.txt`,
never to `deviations.txt`, and the result of the first is `negative.txt`, which decides what DEPLOY.md may say about
that case.

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

**Result of the second play** (`deploy/test/out/amd64-update`):

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

`compare-s0-s1.txt` has 52 PASS lines and no BAD, `attach-s1.txt` 50 PASS lines and no BAD. The raw files stay in the
gitignored scratch directory, with the log `deploy/test/out/amd64-update.log`; they are reproduced by the command under
*How this was run*.

**The pre-upgrade snapshot, observed.** The service wrote it itself, before the first migration, and said so in the journal:
`pre-upgrade snapshot written` with `path` `/opt/holzcloud/data/pre-upgrade-v0.0.25-33-gb7a6c8d-20261012-121929.sqlite`, followed by
`migration applied` for versions 59 and 60. The file name carries the version of the new binary (`v0.0.25-33-gb7a6c8d`), it lies in the
data directory, it has mode 600 and belongs to `holzcloud`, and it holds schema 58 while the live database is at 60. The comparison
was made independently of the capture: one `sqlite3 -readonly` session as the service user on the live database with the snapshot
attached counted every table that both have. It covers the database only, which is why the backup of step 1, with media and
templates, is a separate thing. An update without pending migrations writes no snapshot.

**What the played section left out, and the fix.** The first play (pass 1, and pass 1b with the same result) showed `PREV: absent`,
`BACKUP: none`, the unstamped build line and no word about the snapshot or the check after the start. The five rows in *Findings* list
them. The repaired section has three bash blocks: keep the old binary, print its version and back up with the old binary; build,
transfer, stop, copy, start; check `/readyz` and read the journal. The second play shows `PREV: present`, the backup directory
`20261012-121928` (mode 700, written by the service user), `/readyz` with the new version and commit, and the journal lines of the
snapshot and the two migrations.

**What a subcommand of the new binary would have done.** `openForCLI` migrates the database it opens and takes no snapshot, so a back-up
made with the new binary, or a `holzcloud user list`, would have moved the schema first and left no copy of the old state. This
is read from the code, not played; it is why the section says to back up with the old binary and the harness never runs a
subcommand of the new binary against the old database.

**What this does not show.** That the way back works (plan 21-04 plays it, starting from the marker page `after-update` that `update`
writes after the comparison and captures as state `s1m`), how long a larger database takes to migrate, and that the real server's
clock, disk and mounts behave like the container's (DEVIATION[mount] stands for the mount).
