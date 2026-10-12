# deploy/test: the systemd test container

## What this is

A Debian container with systemd as PID 1 that runs the unit under test, `deploy/holzcloud.service`, unchanged. `deploy/test/run.sh`
builds the image, starts a privileged and throwaway container from it, and drives the checks through `docker exec`. It was built in
phase 20 to answer three questions (UNIT-01, UNIT-02, UNIT-03); the answers are in `deploy/SECURITY-SCORE.md`. Phase 21 (install, update, rollback) used the same recipe with
other steps and recorded its results in `deploy/DEPLOY-RUN.md`; phases 22 (backup and restore) and 23 reuse it too, so the interface below is stable: extend it, do not rename it.

Files:

- `deploy/test/run.sh`: the driver.
- `deploy/test/Dockerfile`, `deploy/test/Dockerfile.dockerignore`: the image and its build-context filter.
- `deploy/test/canary/`: a stdlib-only probe, its own Go module, that asks whether this platform enforces seccomp.
- `deploy/test/out/`: scratch output, gitignored. The committed records are `deploy/SECURITY-SCORE.md` (phase 20) and `deploy/DEPLOY-RUN.md` (phase 21), which paste these files unchanged.

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

# A stepwise session that keeps the container for your own commands.
KEEP=1 ./deploy/test/run.sh build up start
./deploy/test/run.sh exec -- systemctl status holzcloud
./deploy/test/run.sh exec -- curl -fsS http://127.0.0.1:8080/healthz
./deploy/test/run.sh down
```

`all`, `mdwe`, `fresh` and `lifecycle` remove the container when they end, unless `KEEP=1`. `build`, `up`, `start` and the single phase 21 steps never do; `down` does.

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
the document leaves out, and `fresh` and `lifecycle` refuse any other target. The `unit-test` image stays the starting point of phase 22's populated site. The old binary is not in either
image: `bins` builds it from a clone of the tag with the Build line of `deploy/DEPLOY.md`, and `install` copies it in.

`BASE_IMAGE=debian:trixie-slim` switches the base of either target to Debian 13 and systemd 257; the apt list is the same.
The image tag carries the base, so bookworm and trixie images do not overwrite each other.

## Output files

All under `OUT_DIR` (default `deploy/test/out/ARCH`), except `bins.txt`, which lives in `BIN_DIR`. The directory is gitignored and is reset by `up`. The committed record of
phase 20 is `deploy/SECURITY-SCORE.md`, and that of phase 21 is `deploy/DEPLOY-RUN.md`; both paste these files unchanged, and the second proves that with a line-by-line `grep -F`.

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
| `deviations.txt` | the same | One `DEVIATION[name]` line for each thing the container cannot do the way a server does. A name outside the documented set fails the run. |
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

## Hand-over to phase 22

Phase 22 proves that a backup can be restored. DEPLOY.md now installs what it needs; the earlier text of this section, which listed what the image and the document lacked, no longer applies.

- **Where to start.** `postinstall` already plays the Backups block of the document (the units copied from `/opt/holzcloud/deploy`, the timer enabled, one manual backup run as the service user) and checks the paths, so phase 22
  starts from a container that `build up install postinstall` leaves behind and proves what is still open: integrity, content and the restore itself. `INSTALL_FROM=old ... build up install baseline update` leaves a populated and updated site (a website, a published page, five
  plugins, an image, a login user, an order, a marker page and a pre-upgrade snapshot), which is something worth backing up; `lifecycle` with `KEEP=1` leaves the same site after the rollback as well.
- **The paths and packages are fixed.** DEPLOY.md installs the whole deploy directory to `/opt/holzcloud/deploy` (`backup.sh`, `restore.sh`, the two backup units, `Caddyfile.example`), makes the data and backups directories
  belong to the service user (the backups directory has mode 700), and names `rsync` among the packages in Prerequisites. The unit's `ExecStart` for the backup is `/opt/holzcloud/deploy/backup.sh`, the same path as the manual run.
  Do not fix these texts a second time; read them in `deploy/DEPLOY.md` and `deploy/DEPLOY-RUN.md` first.
- **`restore.sh` is interactive.** It asks `[yes/NO]` before it stops the service and replaces the data directory, and the rollback of phase 21 did not use it: the rollback restores the pre-upgrade snapshot by hand with the commands of *Rolling back*.
  A restore proof needs the answer `yes` on stdin, and has to handle the `-wal` and `-shm` files of the replaced database (see Pitfalls).
- **`check` as an after-restore assertion.** `check` (17 checks on the plugins and the page) restarts the unit and reads the key file `/root/.hc-key`, which is outside the data directory and so not part of a backup; after a restore into a new container
  the key file has to be there again, and the key itself lives in the database. `seed` reads the plugin archives from `/opt/holzcloud/plugin-zips`: the `unit-test` image has them, and on a `systemd-base` install `baseline` stages them first
  (DEVIATION[fixture]). A container that `lifecycle` or `baseline` has seeded is refused by a second `seed`.
- **The timer.** Real systemd runs timers inside the container, and `postinstall` enables `holzcloud-backup.timer`; it never fired in a run of phase 21 (`OnCalendar=*-*-* 03:15:00`, `Persistent=true`, `RandomizedDelaySec=300`). How to bring the schedule into reach (a drop-in with a nearer time, or a changed clock) is phase 22's decision and must be named there as a deviation.
- **Login.** `baseline` shows the route for a login user: `holzcloud user create -email ... -role admin`, run as the service user with `HOLZCLOUD_DATA_DIR` set and the password on stdin. `seed` alone creates an API key and no login user.

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

- The container is privileged and throwaway. `down` removes it; `run.sh` removes it on exit for `smoke`, `mdwe` and `all`.
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
- Phase 21 scenario steps, which the document must not contain and which go to `scenario.txt`: `SCENARIO[negative]` (the old binary against the migrated database) and `SCENARIO[restore-new]` (the new binary put back afterwards).
