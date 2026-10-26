# deploy/test: the systemd test container

## What this is

A Debian container with systemd as PID 1 that runs the unit under test, `deploy/holzcloud.service`, unchanged. `deploy/test/run.sh`
builds the image, starts a privileged and throwaway container from it, and drives the checks through `docker exec`. It was built in
phase 20 to answer three questions (UNIT-01, UNIT-02, UNIT-03); the answers are in `deploy/SECURITY-SCORE.md`. Phase 21 (install, update, rollback) used the same recipe with
other steps and recorded its results in `deploy/DEPLOY-RUN.md`; phase 22 (backup and restore) did the same and recorded its results in `deploy/BACKUP-RUN.md`, and phase 23 reuses the harness too, so the interface below is stable: extend it, do not rename it.

Files:

- `deploy/test/run.sh`: the driver.
- `deploy/test/nightly.sh`: the phase 24 driver, a sibling that leaves `run.sh` untouched and reaches it only through its public subcommands (see Phase 24: the nightly run as shipped).
- `deploy/test/Dockerfile`, `deploy/test/Dockerfile.dockerignore`: the image and its build-context filter.
- `deploy/test/canary/`: a stdlib-only probe, its own Go module, that asks whether this platform enforces seccomp.
- `deploy/test/out/`: scratch output, gitignored. The committed records are `deploy/SECURITY-SCORE.md` (phase 20), `deploy/DEPLOY-RUN.md` (phase 21), `deploy/BACKUP-RUN.md` (phase 22) and `deploy/BACKUP-TIMER-RUN.md` (phase 24), which paste these files unchanged.

## Quick start

Needs Docker (Docker Desktop is enough) and bash. Run from anywhere; the script changes into the repository root itself.

```text
# The whole recipe on the project's target architecture: build, up, score, start, canary, seed, check, mdwe-control, verdict.
PLATFORM=linux/amd64 ./deploy/test/run.sh all

# The narrow native run for the MemoryDenyWriteExecute evidence (the only arm64 use, see Deviations).
PLATFORM=linux/arm64 ./deploy/test/run.sh mdwe

# Phase 21: the whole proof from a bare systemd image, one command per chain (each has its own summary.txt, see Phase 21: install, update, rollback).
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-fresh OUT_DIR=deploy/test/out/amd64-fresh ./deploy/test/run.sh fresh
PLATFORM=linux/amd64 TARGET=systemd-base OLD_TAG=v0.0.10 NAME=holzcloud-lifecycle OUT_DIR=deploy/test/out/amd64-lifecycle ./deploy/test/run.sh lifecycle

# Phase 22: the whole backup and restore proof from fresh containers (a source, two targets), one command, ending in OUT_DIR/summary.txt (see Phase 22: backup, timer and restore).
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-bak OUT_DIR=deploy/test/out/amd64-bak TGT_NAME=holzcloud-bak-tgt TGT_OUT_DIR=deploy/test/out/amd64-bak-tgt ./deploy/test/run.sh backup-chain

# A stepwise session that keeps the container for your own commands.
KEEP=1 ./deploy/test/run.sh build up start
./deploy/test/run.sh exec -- systemctl status holzcloud
./deploy/test/run.sh exec -- curl -fsS http://127.0.0.1:8080/healthz
./deploy/test/run.sh down
```

`all`, `mdwe`, `fresh`, `lifecycle` and `backup-chain` remove the container when they end (`backup-chain` all three or four of them: the source and both targets), unless `KEEP=1`. `build`, `up`, `start` and the single phase 21 and phase 22 steps never do; `down` does.

## Subcommands

Subcommands can be chained in one call (`run.sh build up start canary`) and run in the order given. `exec` and `logs` take the rest of the
command line and end the call.

| Subcommand | What it does and what it needs |
|---|---|
| `build` | `docker build --platform PLATFORM --target TARGET` of `deploy/test/Dockerfile`, build arguments `BASE_IMAGE`, `VERSION` and `COMMIT`, tagged `IMAGE`. Needs a Docker daemon. |
| `up` | Refuses when a container named `NAME` exists. Resets `OUT_DIR`, starts the privileged container, waits for `systemctl is-system-running` to say `running` or `degraded`, writes `versions.txt`. Needs `build`. |
| `start` | Starts `holzcloud` unchanged and polls `/healthz`. On the known emulation crash (the `mmap_min_addr` signature) it saves `unit-start-unchanged.txt` and adds the container-only drop-in `ProcSubset=all`; any other failure ends with exit 1. Needs `up` and a `unit-test` image. |
| `wait` | Polls `/healthz` once a second for up to `READY_TIMEOUT` seconds. Needs a running unit. |
| `canary` | Runs the probe under the unit's own sandbox properties, once with `MemoryDenyWriteExecute=yes` and once with `no` as a control, and writes `canary.txt` and `canary.verdict`. Needs `start` first. Ends the whole call with exit 3 when seccomp is not enforced, so put it last in a chain or run it in a subshell. |
| `seed` | Creates an admin API key inside the container, then website `Test`, domain `test.local`, the five plugins (installed, bound, enabled), the published page `plugintest` and one uploaded image, all through `/ai`. Needs `start`. Refuses a container that is already seeded. |
| `check` | Restarts the unit, waits, then makes 17 assertions (five plugins loaded, no load failure, five running, five invoked over HTTP, stable) and prints `engine:`, four `LATENCY` medians and the `Seccomp` lines. Needs `seed`. Prints `CHECK OK`. |
| `mdwe-control` | The same checks with a drop-in `MemoryDenyWriteExecute=no`; the drop-in is removed and the unit restarted afterwards, also when the run fails. Needs `seed`. |
| `verdict` | Reads `canary.verdict`, `check.txt` and `mdwe-control.txt` and prints SETTLED, VOID, FAILS or INCONSISTENT; exit 3 for VOID. Needs `canary`, `check` and `mdwe-control`. |
| `score` | `systemd-analyze security`, online and offline, written raw to `score-online.txt`, `score-offline.txt` and `score-unit.txt`. Needs `up`. Run it before `start`, so no container-only drop-in is in the measurement. |
| `exec` | Runs the rest of the command line in the container (`docker exec -i`); a leading `--` is dropped. |
| `logs` | `journalctl -u holzcloud --no-pager` with the rest of the command line as extra arguments. |
| `down` | `docker rm -f` of the container. Safe to run when there is none. |
| `smoke` | `build`, `up`, `start`, then the UNIT-01 assertions on `/healthz` and `/readyz` (writes `http.txt`), prints `UNIT-01 PASS`. Removes the container unless `KEEP=1`. |
| `mdwe` | `build`, `up`, `start`, `canary`, `seed`, `check`, `mdwe-control`, `verdict`. Removes the container unless `KEEP=1`. Exit 3 means the verdict is VOID. |
| `all` | `build`, `up`, `score`, `start`, the UNIT-01 assertions, `canary`, `seed`, `check`, `mdwe-control`, `verdict`, then a summary. Removes the container unless `KEEP=1`. Exits 0 when nothing really failed, also when the canary or the verdict said 3. |
| `bins` | Builds the binaries under `BIN_DIR` without a container: `holzcloud-new-SHA` for `NEW_REF`, and `holzcloud-old-OLD_TAG` too when `INSTALL_FROM=old` or `WITH_OLD=1`. Each is built in a clone of the repository by the Build line that it reads out of `DEPLOY_DOC`, named by ref, reused unless `REBUILD=1`, and described in a `.info` file and in `bins.txt`. Needs git and a Go toolchain on the host. |
| `install` | Plays the Prerequisites, Build, Transfer, Install and systemd Service sections of `deploy/DEPLOY.md` out of the document, in a container that has none of it. Needs `up` on a `TARGET=systemd-base` image and refuses a container that already has `/opt/holzcloud/holzcloud`. Runs `bins`, stages the binary that `INSTALL_FROM` names, and adds the container-only drop-in when the first start crashes. Writes `steps.txt`, `deviations.txt`, `install.txt`, `http.txt`, `unit-start.txt`; prints `INSTALL OK`. |
| `firstrun` | The document's First Run section over `curl`: its verification block, then the setup form, the login and the redirect to the compulsory two-factor enrolment, with a password made and kept inside the container. Needs `install`; refuses a container that has been through First Run. Writes `firstrun.txt`; prints `FIRSTRUN OK`. |
| `caddy` | The document's Caddy section: install block, then the Caddyfile, with the reload replaced by `caddy validate`. Needs the container of `install` and a network. Writes `caddy.txt` and a `CADDY:` verdict line. Exits 3 when only the network or a missing prerequisite prevented the evidence, 1 when Caddy is installed and rejects the Caddyfile. |
| `postinstall` | The document's Backups block (the unit and timer copied, the timer enabled, one manual backup as the service user), then an audit that every `/opt/holzcloud/deploy/NAME` path the document names exists. Needs `install`. Writes `paths.txt`; prints `POSTINSTALL OK`. |
| `baseline` | On a container installed with `INSTALL_FROM=old`: stages the fixtures, runs `seed`, creates a login user with `holzcloud user create` of the old binary, writes one order row, and captures the state `s0`. Needs `install` with `INSTALL_FROM=old` and the matching `OLD_TAG`. Writes `baseline.txt`, `state-s0.txt`; prints `BASELINE OK`. |
| `update` | Plays the document's Updating section, builds both binaries, captures the state `s1`, compares it with `s0` (`compare-s0-s1.txt`) and with the service's own pre-upgrade snapshot (`attach-s1.txt`), writes a marker page and captures `s1m`. Accepts an update with no migration between the builds (`SNAPSHOT: none`). Needs `baseline`. Writes `update.txt`; prints `UPDATE OK`. |
| `negative` | After `update`: starts the old binary against the migrated database once, which the document tells nobody to do, and records what happens in `negative.txt` (`NEGATIVE-RESULT: starts` or `refuses`); the new binary is put back. Needs `update` with a snapshot, and refuses when none was written. Writes `scenario.txt`; prints `NEGATIVE OK`. |
| `rollback` | After `update`: plays the document's Rolling back section (block 1 when `update.txt` names a snapshot, block 2 when it says `SNAPSHOT: none`), captures `s2`, and compares it with `s0` or with `s1m`. Needs `update`. Writes `rollback.txt`, `compare-s0-s2.txt` or `compare-s1m-s2.txt`; prints `ROLLBACK OK`. |
| `compare` | `run.sh compare state-A.txt state-B.txt [OUTFILE]`: the comparison alone, on two state files, without a container. Exit 1 when a line is BAD. |
| `backup` | Phase 22, in the source container (`NAME`): captures the state `src`, plays the one line of the document's Backups block that names `/backup.sh`, timed with the container's clock, and reads the directory from `=== Backup complete: DIR ===`. Then `backup_verify`: `holzcloud check` on a copy and `sqlite3` `PRAGMA integrity_check` on an immutable URI, the MANIFEST keys, owner and mode `holzcloud 700`, the sha256 of the whole data directory against the backup, the row count of every table against the state `src`, and a self-test that an altered list is reported. Needs `postinstall` and `baseline` (`INSTALL_FROM=old`). Writes `backup.txt`, `backup-run.txt`, `verify-backup.txt`, `tree-src-backup.txt`, `tree-backup.txt`, `rows-backup.txt`, `selftest-tree.txt`, `state-src.txt`; prints `BACKUP OK`, or `BACKUP FAILED` with exit 1 after every file is written. |
| `transfer` | Phase 22: the backup as a tar stream from the source into `/opt/holzcloud/backups` of the target `TGT_NAME` (`docker cp SOURCE:dir - \| docker cp - TARGET:dir`), plus the fixture files (`/root/.hc-login-pass`, `/root/.hc-key`, `/tmp/hc-upload.jpg`) by exec-pipe and a sha256 of every file on both sides. The directory is `BACKUP_DIR_PATH`, else the `DIR:` of `timer.txt` (the backup the timer made), else that of `backup.txt`. Needs `backup` and a target brought up with `up install`. Writes `TGT_OUT_DIR/transfer.txt`; prints `TRANSFER OK`. |
| `restore` | Phase 22, in the target: the setup form must answer 200 (the empty database of the first start), the document's restore line is played with the answer `RESTORE_ANSWER` on stdin and every output line stamped with the container's clock, then `/readyz`, the login with the old password (303), the setup form (404), the time split, size, set-aside directory, owner and mode, the whole-directory comparison, the media file over HTTP, the marker page, the state comparison with the source and `check`. Needs `transfer`. Writes `TGT_OUT_DIR/restore.txt`, `restore-run.txt`, `tree-restored.txt`, `state-tgt.txt`, `compare-src-tgt.txt`, `compare-selftest.txt`, `check.txt`; prints `RESTORE OK`, or `RESTORE FAILED` with exit 1. |
| `backup-timer` | Phase 22, in the source: adds a container-only drop-in to `holzcloud-backup.timer` that fires `TIMER_OFFSET` seconds ahead of the container clock, runs `systemctl daemon-reload` and nothing else, polls until the hardened service has run, and writes the raw evidence (`timer-before.txt`, `timer-after.txt`, `timer-journal.txt`, `timer-dropin.txt`) and `timer.txt` with `TRIGGERED-BY`, `RESULT`, `EXIT-STATUS` and the directory the journal names. Then `backup_verify` on that directory (`verify-timer.txt` and the `-timer` lists), which is the backup `transfer` carries. Needs `postinstall` and `backup`, and a unit that has never run. Prints `BACKUP-TIMER OK` or `BACKUP-TIMER FAILED` (exit 1). |
| `restore-negative` | Phase 22, in the target after `restore`: plays four refusals once, with the live data's invariants taken before and after each (inode of the data directory, whole-tree hash without the SQLite family, the service's InvocationID and ActiveEnterTimestamp, the number of `data.before-restore-*` directories, `is-active`, the `/readyz` status, `integrity_check` of the live database): a truncated database (after proving the damage real), a directory without `holzcloud.sqlite`, the answer `no`, and `backup.sh` with a `MIN_FREE_MB` no disk has. Needs `restore`. Writes `TGT_OUT_DIR/restore-negative.txt` and `scenario.txt`; ends `RESTORE-NEGATIVE OK` or `RESTORE-NEGATIVE FAILED` (exit 1). |
| `backup-fixture` | Phase 22, in the source as the service user: plants one small valid language file (`data/sprachen/nl.json`) and one SVG logo (`data/marke/logo.svg`), so the data directory holds every directory a backup must carry; with `BULK_MB` above zero also `BULK_MB / 5` files of 5 MiB from `/dev/urandom` under `data/media/WEBSITE/bulk`, outside the database. Dies when `BULK_MB * 4 + 1024` megabytes are not free. Writes `bulk.txt` when `BULK_MB` is above zero. |
| `backup-chain` | BAK-01, BAK-02 and BAK-03 in one command: pins `NEW_REF` to a sha, then the source (`build up install baseline update postinstall backup-fixture`), a first target (`up install`), `backup`, `backup-timer`, `transfer`, `restore`, `restore-negative`, and a second data point (the source with `BULK_MB` of synthetic media, its own directory `OUT_DIR-bulk`, a second fresh target `TGT_NAME-bulk`: `backup-fixture`, `backup`, `transfer`, `restore`). Every stage keeps its status, so one failure does not hide the rest. Writes `OUT_DIR/summary.txt`, then ends with exit 1 when a verdict is FAIL. Needs `TARGET=systemd-base` (exit 1 before any docker call otherwise) and removes all its containers unless `KEEP=1`. |
| `fresh` | DEP-01 in one command: `build`, `up`, `install`, `firstrun`, `postinstall`, `caddy`, then `summary.txt`. Needs `TARGET=systemd-base` and refuses any other target. Tolerates exit 3 of `caddy`, never exit 1. Removes the container unless `KEEP=1`. |
| `lifecycle` | DEP-02 and DEP-03 in one command: `build`, `up`, `install` with the old release, `baseline`, `update`, `negative` when a migration lies between the builds, `rollback`, then `summary.txt`. Needs `TARGET=systemd-base` and `OLD_TAG`. Removes the container unless `KEEP=1`. |

## Environment variables

All are optional.

| Variable | Default and meaning |
|---|---|
| `PLATFORM` | `linux/amd64`. Also `linux/arm64`. Passed to `docker build` and `docker run`. |
| `BASE_IMAGE` | `debian:bookworm-slim`. `debian:trixie-slim` gives systemd 257. |
| `TARGET` | `unit-test`. Also `systemd-base`, see Images and targets. |
| `IMAGE` | `holzcloud-unit-test:ARCH-BASETAG`, with `-TARGET` appended when `TARGET` is not `unit-test` (for example `holzcloud-unit-test:amd64-debian-bookworm-slim`). |
| `NAME` | `holzcloud-unit-test-ARCH`. One container per architecture; a second `up` with the same name is refused. |
| `OUT_DIR` | `deploy/test/out/ARCH`. Removed and recreated by `up`. |
| `KEEP` | `0`. `1` keeps the container after `smoke`, `mdwe` and `all`. |
| `READY_TIMEOUT` | `120`. Seconds `wait` polls `/healthz`. |
| `DOCKER_RUN_ARGS` | Empty. Extra `docker run` flags, split at spaces on purpose. |
| `VERSION` | Not set. When set, passed to the build as `--build-arg VERSION`; the Dockerfile default is `dev`. |
| `COMMIT` | Not set. When set, passed to the build as `--build-arg COMMIT`; the Dockerfile default is `unknown`. |
| `OLD_TAG` | `v0.0.10`. The older release that `baseline`, `update` and `lifecycle` start from; built by `bins` with `WITH_OLD=1`. `v0.0.21` has the same schema as the current build. |
| `NEW_REF` | `HEAD`. The ref whose binary is installed and updated to. |
| `BIN_DIR` | `deploy/test/out/bin`. Where `bins` puts the binaries, outside `OUT_DIR` on purpose: `up` resets `OUT_DIR` and must not throw the binaries away. |
| `INSTALL_FROM` | `new`. Which binary `install` stages: `new` (`NEW_REF`) or `old` (`OLD_TAG`). `lifecycle` sets `old` itself. |
| `DEPLOY_DOC` | `deploy/DEPLOY.md`. The document every played command is read out of. |
| `WITH_OLD` | `0`. `1` makes `bins` build the old binary as well. `update` sets it itself. |
| `REBUILD` | `0`. `1` makes `bins` build a binary again although a file for that ref exists. |
| `TGT_NAME` | `NAME-tgt`. Phase 22: the second, fresh container a backup is restored onto. `NAME` and `OUT_DIR` keep meaning the source; `on_target` switches them to the target for one step and refuses a `TGT_NAME` equal to `NAME`. |
| `TGT_OUT_DIR` | `OUT_DIR-tgt`. The target's directory of evidence (`transfer.txt`, `restore.txt`, `restore-negative.txt`, ...). |
| `RESTORE_DIR` | Not set. The backup directory inside the target that the document's restore line is given; `restore` sets it from `transfer.txt`. |
| `RESTORE_ANSWER` | `yes`. What is typed at restore.sh's question. |
| `BACKUP_DIR_PATH` | Not set. An explicit backup directory in the source for `transfer`; without it the `DIR:` of `timer.txt`, then of `backup.txt`. |
| `TIMER_OFFSET` | `60`. Seconds ahead of the container's clock at which the timer drop-in makes the timer fire (10 or more). |
| `TIMER_WAIT` | `TIMER_OFFSET` plus 180. Seconds `backup-timer` polls for the run before it dies and prints the unit's journal. |
| `BULK_MB` | `0` for `backup-fixture`, `300` in `backup-chain` (60 files of 5 MiB). Megabytes of synthetic media in the second data point. |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success. |
| 1 | A real failure: a step failed, an assertion was `BAD`, the canary was invalid, or the verdict was FAILS or INCONSISTENT. |
| 2 | Usage: no subcommand, an unknown one, or `exec` without a command. The usage text is printed. |
| 3 | The step ran, but this platform cannot give the evidence. The canary returns it when seccomp is not enforced (so amd64 under emulation), and `verdict` returns it for VOID. `mdwe` passes it on. `all` does not: it tolerates 3 from both and exits 0. |

`caddy` returns 3 when only the network or a missing prerequisite prevented the Caddy evidence (`CADDY: not run (reason)`); `fresh` tolerates that and writes the verdict into
its summary. A real failure of the Caddy block, including a Caddyfile that Caddy rejects, is still 1.

In a chain such as `run.sh build up start canary seed`, the `canary` step ends the call with 3 on a platform that does not enforce seccomp, and
`seed` never runs. Use `mdwe` or `all`, which know the convention. The same holds for `caddy`: a chain that exits 3 there ends the call, so put `caddy` last or use `fresh`.

## Images and targets

The Dockerfile has four stages; the last, `unit-test`, is the default target.

| Target | Content |
|---|---|
| `systemd-base` | Debian with systemd as PID 1 and `systemd-sysv`, `dbus`, `curl`, `ca-certificates`, `jq`, `procps`, `sqlite3`, `sudo`. No `holzcloud` user, no `/opt/holzcloud`, no binary, no unit. `systemd-modules-load.service` is masked, because it fails in a container. |
| `unit-test` | `systemd-base` plus the `holzcloud` user (the line from `deploy/DEPLOY.md`), `/opt/holzcloud/data` owned by that user, the binary `/opt/holzcloud/holzcloud` built from this tree, the unit in `/etc/systemd/system/` (copied unchanged, not enabled), the five plugin archives in `/opt/holzcloud/plugin-zips`, and the probe `/usr/local/bin/hc-canary`. |

Fresh install, update and rollback start from `systemd-base` and play the document through `install`: they never start from `unit-test`, whose user, directory, binary and unit would hide what
the document leaves out, and `fresh`, `lifecycle` and `backup-chain` refuse any other target. Phase 22 starts from `systemd-base` too: the populated source is `INSTALL_FROM=old run.sh build up install baseline update postinstall`, and the restore target is a second fresh `systemd-base` container brought up with `up install` (a second one for the bulk data point). The old binary is not in either
image: `bins` builds it from a clone of the tag with the Build line of `deploy/DEPLOY.md`, and `install` copies it in.

`BASE_IMAGE=debian:trixie-slim` switches the base of either target to Debian 13 and systemd 257; the apt list is the same.
The image tag carries the base, so bookworm and trixie images do not overwrite each other.

## Output files

All under `OUT_DIR` (default `deploy/test/out/ARCH`), except `bins.txt`, which lives in `BIN_DIR`. The directory is gitignored and is reset by `up`. A `backup-chain` run writes four such directories: `OUT_DIR` (the source, seed data point) with `summary.txt`, `TGT_OUT_DIR` (the first target), `OUT_DIR-bulk` (the same source with synthetic media; only `baseline.txt`, `bulk.txt`, `backup.txt`, `verify-backup.txt` and their lists) and `TGT_OUT_DIR-bulk` (the second target). The committed record of
phase 20 is `deploy/SECURITY-SCORE.md`, that of phase 21 is `deploy/DEPLOY-RUN.md` and that of phase 22 is `deploy/BACKUP-RUN.md`; all three paste these files unchanged. The second proves that with a line-by-line `grep -F`, the third with a block-by-block byte comparison of every appendix block against the scratch file named in its heading (the command is under *Phase 22: backup, timer and restore*).

| File | Written by | Content |
|---|---|---|
| `versions.txt` | `up` | Host architecture, host OS, host kernel, container uname, systemd version, container OS. |
| `score-online.txt` | `score` | `systemd-analyze security holzcloud`, raw, with the `Overall exposure level` line. |
| `score-offline.txt` | `score` | The same for the unit file alone, `--offline=true`. |
| `score-unit.txt` | `score` | `systemctl cat holzcloud`: shows whether a drop-in existed at measurement time. |
| `unit-start-unchanged.txt` | `start` | Only when the first start crashed: `systemctl status` and the journal of the failed start. |
| `unit-start.txt` | `start` | Status, `MainPID`, `NRestarts`, `Result` and the `Seccomp` lines of the started unit. |
| `http.txt` | `smoke`, `all` | The `/healthz` and `/readyz` answers. |
| `canary.txt` | `canary` | The probe table for `yes` and for the `no` control. |
| `canary.verdict` | `canary` | One word: `enforced` or `not-enforced`. |
| `seed.txt` | `seed` | The tool answers of the seeding. Not committed anywhere. The one-time upload link is stripped; the API key is never written. |
| `check.txt` | `check` | The `PASS` and `BAD` lines, `engine:`, `LATENCY`, `Seccomp`, `NRestarts`, `Result` and the `plugin loaded` journal lines. |
| `mdwe-control.txt` | `mdwe-control` | The same for the run with `MemoryDenyWriteExecute=no`. |
| `verdict.txt` | `verdict` | The one-line MDWE verdict. |
| `steps.txt` | `install`, `firstrun`, `caddy`, `postinstall`, `update`, `rollback` | One line `STEP nn rc=R command` per played command of the document. A `systemctl status` line never counts as failed: its status reports the unit state. |
| `step-NN.txt` | the same | The output of step NN. Not committed anywhere. |
| `deviations.txt` | the same | One `DEVIATION[name]` line for each thing the container cannot do the way a server does. The harness does not check the names: the chain's `summary.txt` lists the distinct kinds on its `DEVIATIONS:` line, and whoever reads the evidence compares that list with the documented set. |
| `scenario.txt` | `negative`, `lifecycle` | The `SCENARIO[...]` lines: steps the harness performs that the document must not contain, or the line saying the negative case was skipped. |
| `install.txt` | `install` | `/readyz` after the install, the binary's `version` line, and whether the unit is active. |
| `firstrun.txt` | `firstrun` | The five PASS lines of First Run. The generated password is never written. |
| `caddy.txt` | `caddy` | The output of each Caddy step and the `CADDY:` verdict. |
| `paths.txt` | `postinstall` | `PASS exists` for each path the document names and `TOOL x present` for each tool its Prerequisites install. |
| `baseline.txt` | `baseline` | The old version, the fixtures (website, domain, page, plugins, login user, order) and `OLD_TAG`. |
| `state-LABEL.txt` | `baseline` (`s0`), `update` (`s1`, `s1m`), `rollback` (`s2`) | One `KEY: value` line per fact of a captured state: the schema, row counts per table, content hashes. Counts and hashes only, no credential. |
| `compare-A-B.txt` | `update`, `rollback`, `compare` | The comparison of two state files: `PASS`, `INFO` and `BAD` lines. `compare-selftest.txt` shows that the comparison fails when it should. |
| `attach-s1.txt` | `update` | The comparison of the state with the pre-upgrade snapshot (attached read-only), or the four PASS lines of an update with no snapshot. |
| `update.txt` | `update` | `OLD`, `NEW`, `MIGRATIONS`, `APPLIED`, `SNAPSHOT`, `SCHEMA`, `PREV`, `BACKUP`, the two comparison counts, the journal lines and `MARKER`. |
| `negative.txt` | `negative` | `NEGATIVE-RESULT: starts` or `refuses`, with the answers of the old binary against the migrated database. |
| `rollback.txt` | `rollback` | `MODE: snapshot` or `binary` and the PASS lines of the rollback checks. |
| `summary.txt` | `fresh`, `lifecycle` | The verdict lines of a chain in a fixed vocabulary: `DEP-01 PASS`, `DEP-02 PASS` or `DEP-02 not applicable`, `DEP-03 PASS`, `MODE:`, `NEGATIVE-RESULT:`, `LEAK-CHECK OK`. |
| `backup.txt` | `backup` | `DIR:`, `WALL-S:`, `FREE-MB:`, `DIR-OWNER-MODE:`, `BYTES:`, `SIZE:`, `DB-BYTES:`, `MEDIA-BYTES:`, `FILES:`, `MEDIA-FILES:`, `HOST-ARCH:`, `CONTAINER-ARCH:`, `MEDIA-SERVED:`: the KEY lines of the manual backup. Source directory. |
| `backup-run.txt` | `backup` | The raw output of the document's backup line with the container's `WALL t0 t1` line. |
| `verify-backup.txt`, `verify-timer.txt` | `backup`, `backup-timer` | `PASS`, `INFO` and `BAD` lines about a backup: two integrity readers, MANIFEST keys, owner and mode, the whole-directory comparison, the row counts. |
| `tree-src-backup.txt`, `tree-backup.txt`, `tree-src-timer.txt`, `tree-timer.txt` | `backup`, `backup-timer` | `sha256  ./path` for every file of the source data directory and of the backup, except the SQLite family, `pre-upgrade-*.sqlite*` and `MANIFEST`. |
| `rows-backup.txt`, `rows-timer.txt` | `backup`, `backup-timer` | `table NAME N`: the row count of every table of the backup database. Counts only. |
| `selftest-tree.txt` | `backup` | The BAD lines an altered, an empty and a media-less list produce, and `SELFTEST OK`. Never read for BAD. |
| `state-src.txt`, `state-tgt.txt` | `backup`, `restore` | The captured state of the source just before the backup and of the restored target. |
| `transfer.txt` | `transfer` | `DIR:`, `SOURCE:`, `FILES:` and `TRANSFER: equal`. Target directory. |
| `restore.txt` | `restore` | `DIR:`, `SETUP-BEFORE:`, `READYZ:`, `LOGIN:`, `SETUP-AFTER:`, `VERIFY-S:`, `STOP-MOVE-COPY-S:`, `START-READY-S:`, `WALL-S:`, `DATA-BYTES:`, `BACKUP-BYTES:`, `MEDIA-SERVED:`, `COMES-BACK:`, `HAND-REBUILT:` and the `PASS`, `INFO`, `BAD` lines. Target directory. |
| `restore-run.txt` | `restore` | The raw output of the document's restore line, every line stamped with the container's clock, and the `WALL t0 t1` line. The `[yes/NO]` question is not in it. |
| `tree-restored.txt` | `restore` | The tree list of the restored data directory. |
| `compare-src-tgt.txt`, `compare-selftest.txt` | `restore` | The comparison of source and target state, and the proof that it fails when it should. |
| `timer.txt` | `backup-timer` | `TRIGGERED-BY:`, `RESULT:`, `EXIT-STATUS:`, `START:`, `END:`, `DROPIN-TIME:`, `CLOCK:`, `NEXT-BEFORE:`, `NEXT-AFTER:`, `LAST-BEFORE:`, `LAST-AFTER:`, `DIR:`, `DIR-OWNER-MODE:`, `HOST-ARCH:`, `CONTAINER-ARCH:`, the assertions, then the KEY lines of `backup_verify`. |
| `timer-before.txt`, `timer-after.txt`, `timer-journal.txt`, `timer-dropin.txt` | `backup-timer` | `systemctl list-timers holzcloud-backup.timer` before and after the run, `journalctl -u holzcloud-backup.service` (`-o short-precise`) of the one invocation, and the text of the drop-in. Raw. |
| `restore-negative.txt` | `restore-negative` | Per case `CASE x NAME: exit=N`, the output of the script, `PASS` and `BAD` lines for the exit status, the output and each invariant, `NEGATIVE-RESULT: refused`, and `RESTORE-NEGATIVE OK`. Target directory. |
| `bulk.txt` | `backup-fixture` | `BULK-MB:`, `FILES:`, `BYTES:` (`du -sb`, a directory counts 4096) and `MEDIA-BYTES:` of the synthetic media. |
| `summary.txt` | `backup-chain` | `BAK-01`, `BAK-02`, `BAK-03` and `RESTORE-NEGATIVE` each `PASS` or `FAIL`, `POINT seed:` and `POINT bulk:` with sizes, wall seconds and the restore split, `TIMER:`, `STEPS:`, `DEVIATIONS:`, `LEAK-CHECK OK`. |
| `bins.txt`, `holzcloud-*` | `bins` | In `BIN_DIR`: the built binaries and a description of each (version, commit, sha256, number of migration files). `up` resets `OUT_DIR` and not `BIN_DIR`. |

## Phase 21: install, update, rollback

Phase 21 played a fresh install by `deploy/DEPLOY.md` alone, an update across two migrations and a rollback; the results, the findings and every difference from a real
server are in `deploy/DEPLOY-RUN.md`.

- **Rerun the proof.** Two commands, each from a fresh container, each with its own `NAME` and `OUT_DIR`, each ending in `OUT_DIR/summary.txt`:
  - `fresh` (DEP-01): `build up install firstrun postinstall caddy`. The summary has `DEP-01 PASS`, the `CADDY:` verdict, `STEPS: N played, F failed`, `DEVIATIONS:` and `LEAK-CHECK OK`.
  - `lifecycle` (DEP-02, DEP-03): `build up install baseline update`, then `negative` when `update.txt` says `pending=` one or more (otherwise `scenario.txt` records it as skipped), then
    `rollback`. Needs `OLD_TAG`. The summary has `OLD:`, `NEW:`, `MIGRATIONS:`, `SNAPSHOT:`, `MODE:`, `NEGATIVE-RESULT:` (when played), `DEP-02 PASS` or
    `DEP-02 not applicable (no migration between these builds)`, `DEP-03 PASS` and `LEAK-CHECK OK`. `OLD_TAG=v0.0.10` crosses migrations 59 and 60 (`MODE: snapshot`); `OLD_TAG=v0.0.21` has the
    same schema as HEAD (`MODE: binary`, no snapshot).
  - The same steps can be called one by one on a container that is kept (`KEEP=1`, or no chain): `build up install firstrun postinstall caddy`, or `INSTALL_FROM=old build up install baseline update negative rollback`.
- **The document is the source of truth, the script only mirrors it.** `doc_play` reads the commands out of the fenced bash blocks of a section of `DEPLOY_DOC`, `doc_cmd` refuses a line that is not in the document, and no
  command of the proof is typed into `run.sh`. To change what is played, change the document. The harness reads the headings `Prerequisites`, `Build`, `Transfer to the server`, `Install`, `systemd Service`,
  `First Run`, `Caddy (Reverse Proxy + HTTPS)`, `Backups`, `Updating` and `Rolling back`; a heading is a `##` or `###` line with exactly that text. A missing heading ends the run with an error that names it, and for the sections `install` needs (`Build`, `Transfer to the server`, `Install`, `systemd Service`) so does a missing bash block, both before Docker is touched; `Prerequisites` and `First Run` are skipped when they have no bash block.
- **A played block carries no angle-bracket value and no carried variable.** Each line is played in a shell of its own, so a value the reader would fill in (`<tag>`) or a variable set by an earlier line
  cannot be played. A block that needs one is written so that it does not, or the harness answers the step itself and names it as a deviation or a scenario.
- **A played block must wait for the service itself.** The harness adds no wait after a step: the check lines use `curl --retry 30 --retry-connrefused --retry-delay 1`.
- **Do not edit `deploy/DEPLOY.md` between the chains and the record.** The evidence set is made against one text of the document; an edit means the set is made again.

## Phase 22: backup, timer and restore

Phase 22 proved that a backup made by `deploy/backup.sh` restores onto an empty system with `deploy/restore.sh`, that the nightly timer starts the hardened backup service, and what the restore costs.
The record is `deploy/BACKUP-RUN.md`; this section says how to repeat the proof.

- **Rerun the proof.** One command, from nothing but the image, ending in `OUT_DIR/summary.txt`:

  ```text
  PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-bak OUT_DIR=deploy/test/out/amd64-bak \
    TGT_NAME=holzcloud-bak-tgt TGT_OUT_DIR=deploy/test/out/amd64-bak-tgt BULK_MB=300 ./deploy/test/run.sh backup-chain
  ```

  It pins `NEW_REF` to a sha at its start, so the source and both targets get the byte-identical binary. The summary has `BAK-01`, `BAK-02`, `BAK-03` and `RESTORE-NEGATIVE`, one `POINT` line per data point (the seed site as it is, and the same site with `BULK_MB` of synthetic media),
  `TIMER:`, `STEPS:`, `DEVIATIONS:` and `LEAK-CHECK OK`. A FAIL ends the run with exit 1 after the summary is written. Four directories of evidence are made: `OUT_DIR`, `TGT_OUT_DIR`, `OUT_DIR-bulk` and `TGT_OUT_DIR-bulk`.
- **The separable steps, in order.** On a source `INSTALL_FROM=old run.sh build up install baseline update postinstall` (the populated site with the backup units installed), on a target `run.sh up install` under its own `NAME` and `OUT_DIR`; then, with `NAME`, `OUT_DIR`, `TGT_NAME` and `TGT_OUT_DIR` given on every call:
  `backup-fixture`, `backup`, `backup-timer`, `transfer`, `restore`, `restore-negative`. A BAD line makes a step exit 1 after it has written every file it owns; the calls can be chained (`run.sh backup backup-timer transfer restore`), which goes on after a 1 and ends with 1.
  `backup-timer` needs a service that has never run, so it is played once per source container, and `transfer` then carries the backup the timer made, not the manual one.
- **The document is still the source of the commands.** The manual backup line and the restore line are read out of `deploy/DEPLOY.md` (the sections *Backups* and *Restore from a backup*) and played through `doc_cmd`. The restore line carries a sample timestamp; its last word is replaced by the transferred directory and the replacement is named `DEVIATION[backup-name]`.
  The scripts `backup.sh` and `restore.sh`, the two backup units and `DEPLOY.md` are not edited by the harness, and the harness never hides a step that fails.
- **Where the proof is carried by something else than the document.** `DEVIATION[transfer]`: the backup crosses as a tar stream through the host instead of `rsync` or `scp`. `DEVIATION[timer]`: the container-only drop-in on the timer, and what it cannot show (the real 03:15 window, `Persistent=true` after a missed run, `RandomizedDelaySec`).
  `DEVIATION[fixture]` covers the plugin archives, the image, the login user and the order row of phase 21, and now also the language file, the logo and the synthetic media, and the secrets that cross from container to container by exec-pipe.
  `DEVIATION[backup-name]`: the sample timestamp of the restore line.
- **Scenario names**, in `scenario.txt` of the target: `SCENARIO[restore-damaged]` (a truncated database, the damage proven real first), `SCENARIO[restore-no-sqlite]` (a directory with a MANIFEST and no database), `SCENARIO[restore-answer-no]` (the answer `no`) and `SCENARIO[backup-low-disk]` (`backup.sh` with `MIN_FREE_MB=99999999`). The script path is the document's, the arguments are new.
- **What the recipe taught** (read these before extending it):
  - `read -p` prints no prompt without a terminal, so the `[yes/NO]` line is absent from a piped transcript (`restore-run.txt`); the wording about the prompt comes from the script, not from the transcript.
  - The file `VACUUM INTO` writes is itself WAL-mode, and a plain `sqlite3 -readonly` of it failed on a newer sqlite. For an independent reader open it as `file:PATH?immutable=1`, which also never creates `-wal` and `-shm` files next to a backup. The binary's `check` opens the database read-write, so it runs on a copy, the way `restore.sh` does it,
    and it is `quick_check`; the second reader runs the stronger `integrity_check`.
  - Restarting the timer after adding the drop-in stamps `LAST` with the restart time and hides the evidence. `daemon-reload` is enough to move `NEXT`; the proof of a run is `TriggeredBy=holzcloud-backup.timer` on the service and the journal, and the drop-in starts with an empty `OnCalendar=`, because the option is a list and only the empty value resets it.
    The directory a timer run made is the one the journal's `=== Backup complete: DIR ===` names, never the newest: the backups directory already holds the `postinstall` and the Updating backups.
  - The backup crosses between containers as a tar stream (`docker cp SOURCE:dir - | docker cp - TARGET:dir`), not through a host directory: the host filesystem is case-insensitive, and 300 MiB of media should not land on the host disk. Modes, times and names survive.
  - `du -sb` counts apparent bytes and each directory as 4096, so a size of a tree with directories is a little more than the sum of its files; `bulk.txt` and the `*-BYTES:` lines say which is meant.
  - The fixture secrets (`/root/.hc-login-pass`, `/root/.hc-key`) cross by exec-pipe between two `docker exec` calls and are never written to a host file; `leak_check` runs on all four directories.
  - A comparison limited to `media/`, `templates/` and `csrf.key` would pass while the restore is broken: the comparison hashes the whole data directory and must find a file below `./media/` in its reference list, or it fails.
- **Hand-over facts that still hold.** `postinstall` plays the Backups block of the document (the units copied from `/opt/holzcloud/deploy`, the timer enabled, one manual backup as the service user) and checks the paths; DEPLOY.md installs the deploy directory to `/opt/holzcloud/deploy`, makes the data and backups directories belong to the service user (the backups directory has mode 700) and names `rsync` in Prerequisites.
  `check` (17 assertions) reads the key file `/root/.hc-key`, which is outside the data directory and so in no backup; the key itself lives in the database. `seed` reads the plugin archives from `/opt/holzcloud/plugin-zips`, which `baseline` stages (`DEVIATION[fixture]`).
  Login: `holzcloud user create -email ... -role admin`, run as the service user with `HOLZCLOUD_DATA_DIR` set and the password on stdin; `seed` alone creates an API key and no login user.
- **A document number that is wrong by more than a factor of two against the final set is corrected and the whole set is made again, never patched.** The first complete final set measured a restore of about seven seconds against the two seconds `DEPLOY.md` still quoted from pass 1, because the five plugin modules had joined the backup (20 MB, not under one) and the service loads each plugin at start, about a second each under emulation. The set was thrown away, the paragraph corrected and the chain run again; the record pastes only the set of the evidence commit. Do not paste from a discarded set.
- **The plugins dominate the seed point.** A restore is about eight seconds at both data points because `START-READY-S` is about 7.4 seconds either way; 300 MiB of media added a fraction of a second to the copy. Read `START-READY-S` and `STOP-MOVE-COPY-S` separately before quoting a rate, and say that the rate rests on two single readings.
- **`set -u` and a status that is only set on failure.** `restore_into` set `ccrc` only in its `|| ccrc=$?` branch, so the first restore whose `check` passed ended on `ccrc: unbound variable`, exit 1, with a right result behind it. Pass 1 never reached it because `check` failed there. A variable that a green path reads must be initialised.
- **Proof of the record, one command.** The appendix of `deploy/BACKUP-RUN.md` has one level-four heading per file, exactly `#### DIRECTORY/FILE` with `DIRECTORY` the scratch directory below `deploy/test/out`, and one fenced block of the file under it. The comparison extracts every block and compares it with its scratch file, byte for byte (a missing final newline is normalised); it prints the number of blocks it checked and names every block that differs:

  ```bash
  R=deploy/BACKUP-RUN.md; E=deploy/test/out; T=$(mktemp -d); : > $T/index
  awk -v d="$T" '/^## Appendix: raw output$/{app=1;next} !app{next} /^#### /{name=$2;want=1;next} want && /^```/{ if(!inb){inb=1;n++;f=d "/" n;print n " " name >> (d "/index");printf "" > f;next} else {inb=0;want=0;close(f);next} } inb{print >> f}' $R
  bad=0; while read -r n name; do test -f "$E/$name" || { echo "no scratch file for $name" >&2; bad=1; continue; }; awk 1 "$T/$n" > "$T/$n.a"; awk 1 "$E/$name" > "$T/$n.b"; cmp -s "$T/$n.a" "$T/$n.b" || { echo "block differs from the scratch file: $name" >&2; bad=1; }; done < $T/index
  test "$bad" -eq 0 && echo "$(wc -l < $T/index | tr -d ' ') blocks match their scratch files"
  ```

  It needs the scratch directories, so it can be run only where the chain was run. A scratch file with a line that begins with three backticks, with `#### ` or with `## ` cannot be pasted unchanged (the comparison and the heading list would misread it): check that first. Never paste a `step-NN.txt`, `seed.txt`, a `.log` file, a cookie jar or a password file.
- **Do not edit `deploy/DEPLOY.md`, `backup.sh` or `restore.sh` between the final chains and the record.** The evidence set is made against one text of each; an edit means the set is made again. The record names the evidence commit on a line of its own (`Evidence commit: SHA`) and `deploy/test/out/amd64-bak-evidence-commit.txt` holds the same sha; `git diff --quiet SHA HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh` exits 0 while that holds.
- **`sed -i` with a line address differs on macOS (BSD sed).** Edit a file with the editor tool or a scripted rewrite, not with `sed -i` and a line address, on the development machine.

## Hand-over to phase 23

The beta criteria may cite `deploy/BACKUP-RUN.md`, and only the following of it:

- **The Summary table** (BAK-01, BAK-02 and BAK-03, each PASS) and the line `Evidence commit` that names the commit the evidence set was made on, which is the commit whose `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, units and `deploy/test/run.sh` the record describes.
- **The words of `summary.txt`** of that set: `BAK-01 PASS`, `BAK-02 PASS`, `BAK-03 PASS`, `RESTORE-NEGATIVE PASS`, `STEPS: 78 played, 0 failed`, `LEAK-CHECK OK`, and the two `POINT` lines with the numbers in *Measurements*, which are emulation numbers: a restore of about eight seconds, a backup of about a second, both at 20 MB and at 320 MB.
- **What the record states as shown**: a backup made by the timer's service restores by `restore.sh` into a different, empty system with working pages, media, login and plugins; the four refusals leave the live data untouched; the scripts carry and return media, templates, plugins, language files and the logo; and the list of what no backup holds (`HAND-REBUILT`).

How to rerun, from nothing but the image (names as in the final set; give the same `NAME`, `OUT_DIR`, `TGT_NAME` and `TGT_OUT_DIR` so the evidence lands where the record expects it, or other names and a new record):

```text
PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=$(git rev-parse --short HEAD) \
  NAME=holzcloud-bak-src OUT_DIR=deploy/test/out/amd64-bak-src \
  TGT_NAME=holzcloud-bak-tgt TGT_OUT_DIR=deploy/test/out/amd64-bak-tgt \
  ./deploy/test/run.sh backup-chain      # BULK_MB left at the chain's default of 300
```

They must not claim, copied from the record's *Not tested*:

- The real 03:15 window: the timer was moved a minute ahead by a container-only drop-in.
- `Persistent=true` after a missed run, and `RandomizedDelaySec` (set to 0 by the drop-in).
- The off-site `REMOTE_TARGET` copy, and retention pruning (no directory was older than the retention).
- An atomic database-plus-media snapshot: the pair is taken one after the other with the site running, and the window was not provoked.
- A backup older than the binary that restores it, in the final set.
- A restore on a real server, with a separate data volume, a cold cache and a real network copy.
- Native amd64 timing: every number is an emulation number on an aarch64 host, and the extrapolation in the record is an estimate, not a measurement.
- Protection against bit rot in a complete backup: a flipped byte inside a data page passes both integrity readers.

If a beta criterion needs one of these, it needs its own run; none of them follows from the record. A change to `deploy/DEPLOY.md`, `backup.sh`, `restore.sh`, the units or `run.sh` after the evidence commit means the set is made again before the record is cited for the changed text.

## Phase 24: the nightly run as shipped

Phase 22 moved the backup timer a minute ahead with a container-only drop-in. Phase 24 played the shipped `holzcloud-backup.timer` and `holzcloud-backup.service` with no drop-in on either: the 03:15 entry, the 300 second random delay, the catch-up of a missed window, the pruning of old directories, the off-site copy to a second host and a restore from that copy
(BKP-01 to BKP-05). The record is `deploy/BACKUP-TIMER-RUN.md`; this section says how to repeat the proof. `nightly.sh` is a sibling of `run.sh` on purpose: `docs/beta.md` binds `run.sh` to its evidence commit, so `run.sh` is not edited, and `nightly.sh` reaches the harness through `run.sh`'s public subcommands only.

- **Rerun the proof.** One command, from nothing but the image, ending in `OUT_DIR/summary.txt` (about 15 minutes with the image and the Go build cached):

  ```text
  PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=$(git rev-parse --short HEAD) bash deploy/test/nightly.sh chain
  ```

  It refuses to start when `TARGET` is not `systemd-base` or when either container exists, pins `NEW_REF` to a sha so the source and the second host get the byte-identical binary, and plays source, second host, `fire`, `delay`, `catchup`, `prune`, `run.sh backup`, `offsite` and `offsite-restore`. Each stage is a child process of its own and its status is kept, so one failure hides nothing
  (a function called from the left of `||` runs with `set -e` off, which is why the stages are processes). The verdict of each requirement comes from the files the stage wrote: `BKP-01` to `BKP-05` are `PASS` only when the stage ended 0, the file ends with its ok line and no line begins with `BAD`; a stage that ended with 3 (the platform could not give the evidence) is `FAIL` with the reason on the line.
  The summary then has one `KEY:` line each for `TIMER`, `DELAY`, `CATCHUP`, `PRUNE`, `OFFSITE` and `RESTORE`, `STEPS: N played, F failed`, `DEVIATIONS:` and `LEAK-CHECK OK`. A FAIL ends the run with exit 1 after the summary. The containers are removed at the end unless `KEEP=1`.
- **The subcommands**, which can be called one by one on kept containers (`KEEP=1`) and chained in one call:

  | Subcommand | What it does | Result file |
  |---|---|---|
  | `selftest` | the pure functions (zone arithmetic, window and sample judgements, catch-up, prune expectation, list equality, restore lines, `verdict_of`, `record_check`) against a table of cases, failing cases included; no Docker | `SELFTEST OK` |
  | `source` | `run.sh build up install baseline update postinstall backup-fixture`: the populated site whose postinstall enables the shipped timer | the files of phase 22 |
  | `target` | `run.sh up install` for the second, fresh container | the files of phase 22 |
  | `fire` | BKP-01: the zone is moved so that the shipped 03:15 entry comes due in about 90 seconds; the unit starts by itself | `fire.txt`, `verify-fire.txt`, `units-shipped.txt` |
  | `delay` | BKP-03: the shipped 300 second delay, 12 re-armings sampled, and the draw of the real run | `delay.txt` |
  | `catchup` | BKP-02: the container is stopped across the missed 03:15 and started again; a control timer with `Persistent=false` shows the run is the catch-up | `catchup.txt`, `verify-catchup.txt` |
  | `prune` | BKP-05: five directories at 45 d, 31 d 1 h, 30 d 12 h, 29 d and 1 d are planted, one run of the shipped service | `prune.txt` |
  | `offsite` | BKP-04: `REMOTE_TARGET` uncommented, the copy to the second host; credentials in the home (experiment A) and outside it (experiment B) | `offsite.txt`, `offsite-natural.txt` |
  | `offsite-restore` | BKP-04: the document's restore line, played on the second host against the copy the unit made | `offsite-restore.txt`; host 2: `restore.txt` |
  | `chain` | everything above from fresh containers | `summary.txt` |
  | `record-check` | the proof of the record against the scratch files (below) | `RECORD-CHECK OK N blocks` |
  | `down` | removes the source and the second host; safe when either is absent | |

- **Variables**, all optional: `NAME`, `OUT_DIR` (default `holzcloud-t24-src`, `deploy/test/out/amd64-t24-src`); `TGT_NAME`, `TGT_OUT_DIR` (`holzcloud-t24-host2`, `deploy/test/out/amd64-t24-host2`); `ZONE_LEAD` (90 s before 03:15:00 local at which `fire` sets the zone, 60 to 150); `FIRE_WAIT` (540 s that `fire` waits for the run);
  `DELAY_SAMPLES` (12); `CATCHUP_LEAD` (150 s) and `CATCHUP_WAIT` (480 s, after the start); `KEEP` (1 keeps the containers of `chain`); `NEW_REF`; and for `record-check` `R` (`deploy/BACKUP-TIMER-RUN.md`) and `E` (`deploy/test/out`).
- **Output files** (`OUT_DIR` is the source, `TGT_OUT_DIR` the second host): `summary.txt`, `versions.txt`, `deviations.txt`; `units-shipped.txt` (sha256 of the installed units against `deploy/`, and that no drop-in exists); `fire.txt`, `fire-before.txt`, `fire-after.txt`, `fire-journal.txt`, `fire-armed-log.txt` (PID 1's debug lines about the timer), `verify-fire.txt`;
  `delay.txt`; `catchup.txt`, `catchup-before.txt`, `catchup-after.txt`, `catchup-docker.txt`, `catchup-journal.txt`, `verify-catchup.txt`; `prune.txt`, `prune-before.txt`, `prune-after.txt`, `prune-journal.txt`; `offsite.txt`, `offsite-natural.txt` (the credential experiments, raw), `offsite-journal.txt`, `offsite-journal-b.txt`, `offsite-unit-diff.txt`, `offsite-sha-source.txt`; `offsite-restore.txt`;
  on the second host `offsite-host2.txt`, `offsite-sha-host2.txt`, `offsite-modes.txt`, `transfer.txt`, `restore.txt`, `restore-run.txt`, `check.txt`. `selftest-tree.txt` and `compare-selftest.txt` carry `BAD` lines on purpose (run.sh's proof that its comparison fails when it should); they are not read for `BAD` and are not pasted.
- **Deviation names**, in `deviations.txt`: `zone` (the time zone is moved, never the clock), `restart` (a container stop and start for a power cycle), `control-timer` (the `Persistent=false` control, new files and no drop-in), `ssh-packages` (the document installs `rsync`, which only suggests an ssh client and server), `offsite-host` (the second host is a container on the same bridge),
  `hosts` (`nas` is an `/etc/hosts` line), `remote-target` (the one line the unit's own comment says to uncomment), `credentials` (the key lives in the source only, the host key was taken on first use), `fixture` (the logins and the test image carried by exec-pipe), `backup-name` (the sample timestamp of the restore line), and from phase 21 `scp`, `dropin` (`ProcSubset=all` on `holzcloud.service` only) and `mount`.
  Neither backup unit has a drop-in; `units-shipped.txt` shows it.
- **What the recipe taught** (read these before extending it):
  - Move the time zone, never the clock. The container shares the kernel clock of the Docker VM, so a clock setter would move every container on the daemon; a fixed-offset zone compiled with `zic` and copied to `/etc/localtime` changes what 03:15 means and nothing else. The catch-up zone is a whole calendar day away from the first, or `backup.sh` names two backups alike (it names them by local date and second).
  - The stamp file `/var/lib/systemd/timers/stamp-holzcloud-backup.timer` is what `Persistent=true` reads. systemd may create it when the timer is started, so a catch-up test needs an older stamp than the missed instant, and the stage reads it before and after.
  - A timer is re-armed, with a new random draw, whenever the realtime clock steps (the Docker VM steps it about every 30 seconds). The armed time of a run is therefore the last draw PID 1 logged before the start, read from its debug log, not a polled `NextElapseUSecRealtime`.
  - The journal survives `docker stop` in this image (`/var/log/journal` exists) and the kernel boot id is the VM's, so `journalctl -b` does not separate the two starts; a run's own lines are found by `_SYSTEMD_INVOCATION_ID`. Unit states, including `ExecMainStartTimestamp`, do start from nothing, because `/run` is a tmpfs. Evidence is saved before the stop anyway.
  - `AccuracySec` of the shipped timer is systemd's default of 1 minute, inside the 300 second window; the allowances in the verdicts are named in the script and written to the file as `SLACK-S`.
  - `find -mtime +N` counts whole 24 hour units, rounded down, so `backup.sh` deletes a directory from `RETENTION_DAYS + 1` days of age; a directory 30 days and 12 hours old stays. `prune_expect` encodes that and the stage plants a directory on each side of the boundary.
  - `rsync` only suggests an ssh client, and `rsync -a --delete` makes the off-site copy a mirror of the backup root: what the box prunes, the copy loses.
  - `ProtectHome=true` hides `/home` from the service, and the service user `holzcloud` has no home directory. A key and `known_hosts` in `~holzcloud/.ssh` fail in the unit (`Host key verification failed`, status 255) and work outside the sandbox. A key under `/opt/holzcloud/offsite-ssh` named by `/etc/ssh/ssh_config.d/holzcloud-backup.conf` works with the unit unchanged; `deploy/DEPLOY.md` now says so.
  - `/tmp` is emptied at every boot (the tmpfiles.d rule `D /tmp 1777 root root -`; it has no mount of its own), so the test image that `run.sh baseline` stages in `/tmp/hc-upload.jpg` is gone after the `catchup` restart. `catchup` keeps it under `/root` for the stop and puts it back, and says so in `catchup.txt` (`TEST-IMAGE-*`, `TMP-AT-BOOT`); the restore on the second host reads it.
  - A shell function called from the left of `||` runs with `set -e` off for its whole body. `chain` runs its stages as child processes for that reason.
  - The chain takes about 15 minutes, longer than a foreground shell command may run on some machines: start it in the background (a small wrapper script that writes the host date before and after into a log) and read only `summary.txt` and the stage files it names when it ends.
- **Proof of the record, one command.** `bash deploy/test/nightly.sh record-check` reproduces the appendix proof of `deploy/BACKUP-RUN.md` (every `#### DIRECTORY/FILE` block after `## Appendix: raw output` equals `deploy/test/out/DIRECTORY/FILE` byte for byte, a missing final newline normalised) and adds four checks: every token `amd64-t24-src/FILE` or `amd64-t24-host2/FILE` outside the appendix has a block
  (no sentence cites output that is not pasted), the required headings exist, there is exactly one line `Evidence commit: SHA` that `git cat-file` resolves, and no private key marker occurs. It prints `RECORD-CHECK OK N blocks` or names every failure and exits 1. It needs the scratch directories, so it can be run only where the chain was run. The appendix is assembled by script, not by hand;
  a scratch file with a line that begins with three backticks, with `#### ` or with `## ` cannot be pasted unchanged. Never paste a `step-NN.txt`, `seed.txt`, a `.log` file, a cookie jar or a key file.
- **The evidence set is made again after any edit of a bound file.** The bound files are `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, the two backup units, `deploy/test/run.sh` and `deploy/test/nightly.sh`. The record names the evidence commit on a line of its own, `deploy/test/out/amd64-t24-evidence-commit.txt` holds the same sha, and
  `git diff --quiet SHA HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/nightly.sh` exits 0 while that holds. The README and the record are not bound files.

## Pitfalls

- `ProcSubset=pid` crashes the amd64 emulation at process start: `status=5/TRAP` and `Unable to open /proc/sys/vm/mmap_min_addr`. The drop-in `ProcSubset=all` is the named container-only answer (see `deploy/SECURITY-SCORE.md`); `start` applies it. On native arm64 the unit starts without it.
- Seccomp is not enforced under the emulation (`Seccomp: 0`), so a green plugin run there proves nothing about `MemoryDenyWriteExecute`. The canary decides, and `verdict` says VOID.
- wazero falls back to its interpreter silently, so "all plugins loaded" does not say which engine ran. Read the `engine:` line of `check.txt`.
- `/tmp` is private to a transient unit (`PrivateTmp=true`), so helpers that a transient unit must run live in `/usr/local/bin`.
- Never pass a megabyte of base64 through a command-line argument ("Argument list too long", and the tool then reports an empty file). Pipe it: `seed` does, through `jq -Rs`.
- Online and offline `systemd-analyze security` differ (1.3 against 1.4 on systemd 252). Record which one a number is.
- Run `systemctl reset-failed holzcloud` between experiments that crash the unit: `StartLimitBurst=5` refuses the next start for the wrong reason otherwise.
- Run the CLI as the service user with `HOLZCLOUD_DATA_DIR` set (`runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data /opt/holzcloud/holzcloud ...`), or the database files end up owned by root and the service cannot open them.
- Do not publish port 8080. The unit sets no `HOLZCLOUD_LISTEN`, so the service binds `127.0.0.1` inside the container; use `exec` and `curl`.
- Poll `/healthz`, never sleep: the service needs 8 to 10 seconds under emulation, the plugin compile included.
- `systemctl is-system-running` may say `degraded` on trixie. The failing `systemd-modules-load` is masked in the image, and `up` accepts `degraded` as well as `running`.
- Give `--platform` to `docker build` and to `docker run`. A locally built image started without it failed with a pull error. `run.sh` does both.
- Every `holzcloud` CLI subcommand that opens the database (`user list`, `check`, `backup`, `migrate status`) migrates it silently, so use `sqlite3 -readonly` to look into a database that must stay as it is. The old binary does the same to nothing: it does not refuse a newer schema (`NEGATIVE-RESULT: starts`).
- Restoring a database by copying a file over `holzcloud.sqlite` needs the `holzcloud.sqlite-wal` and `holzcloud.sqlite-shm` of the replaced database removed first, or SQLite reads the restored file together with a log that is not its own.
- `up` resets `OUT_DIR` and not `BIN_DIR`. A binary there is named by its ref and reused until `REBUILD=1`; two `NAME` and `OUT_DIR` pairs can share `BIN_DIR`.
- With `HOLZCLOUD_SECURE=true`, a post to `/admin` over plain http needs a `Referer` header that carries the https origin, or it is answered `403 Forbidden - CSRF validation failed`; `firstrun` adds it and names it DEVIATION[browser].
- A played block must wait for the service itself (`curl --retry 30 --retry-connrefused --retry-delay 1`); the harness adds no wait between steps.
- A step in a chain that exits 3 ends the call. `fresh` handles that for `caddy`; a chain of your own has to as well.
- The newest `pre-upgrade-*.sqlite` is not always the snapshot of the update: a fresh install writes one of an empty database at its first start, and an update with no migration writes none.

## Safety

- The container is privileged and throwaway. `down` removes it; `run.sh` removes it on exit for `smoke`, `mdwe`, `all`, `fresh`, `lifecycle` and `backup-chain` (the last one all of its containers, the targets too).
- No host path is mounted and no port is published. The one file that goes in is the test image, through `docker cp`.
- The admin API key is created inside the container and stays in a root-only file there. It never reaches the host or any output file.
- `up` removes and recreates `OUT_DIR` and refuses `/`, `.`, `..` and the repository root.
- A stale container with the same `NAME` is never reused; `up` refuses it.
- The repository rule that nothing loads from a third party at runtime is untouched: nothing here ships in the binary. Docker pulls base images at image build time only.

## Deviations

Phase 20's are recorded with their evidence in `deploy/SECURITY-SCORE.md`, phase 21's in `deploy/DEPLOY-RUN.md` (section *Deviations named*, with what a real server does instead).

- The container-only drop-in `ProcSubset=all`, which the harness adds only after the known emulation crash. The shipped unit is untouched. Named `DEVIATION[dropin]`.
- D-01: one narrow native arm64 run, as MemoryDenyWriteExecute evidence only. UNIT-01 and UNIT-03 are measured on linux/amd64. No native amd64 run was made.
- Seccomp is not enforced on amd64 under the emulation, so seccomp-based directives are not exercised there; the document marks each as measured or inferred.
- Phase 21, named in the output of every run (`deviations.txt`): `DEVIATION[scp]` (the two `scp` lines played as `docker cp`), `DEVIATION[browser]` (First Run played with `curl` and a Referer), `DEVIATION[caddy]` (the reload replaced by `caddy validate`),
  `DEVIATION[mount]` (a plain directory where the unit's `RequiresMountsFor` expects a mount), `DEVIATION[fixture]` (the plugin archives, image, login user and order row), and `DEVIATION[network]`, printed only when a step fails for want of a network and then ending the call with exit 3.
  The harness also knows `DEVIATION[prompt]` (an `apt` question answered on stdin) and `DEVIATION[preinstalled]` (a tool of the image that the document does not name); neither occurred in the final runs.
- Phase 22, named in the output of every run (`deviations.txt`): `DEVIATION[transfer]` (the backup crosses as a tar stream through the host, not by `rsync` or `scp`), `DEVIATION[timer]` (the container-only drop-in `holzcloud-backup.timer.d/container.conf`: an empty `OnCalendar=`, a time `TIMER_OFFSET` seconds ahead, `RandomizedDelaySec=0`, `AccuracySec=1s`; the real 03:15 window, `Persistent=true` after a missed run and `RandomizedDelaySec` are not tested),
  `DEVIATION[backup-name]` (the sample timestamp of the document's restore line replaced by the transferred directory, the answer typed on stdin), and `DEVIATION[fixture]`, which now also covers the planted language file and logo, the synthetic bulk media and the secrets carried by exec-pipe.
  Phase 22 scenario steps (`scenario.txt` of the target): `SCENARIO[restore-damaged]`, `SCENARIO[restore-no-sqlite]`, `SCENARIO[restore-answer-no]` and `SCENARIO[backup-low-disk]`.
- Phase 21 scenario steps, which the document must not contain and which go to `scenario.txt`: `SCENARIO[negative]` (the old binary against the migrated database) and `SCENARIO[restore-new]` (the new binary put back afterwards).
