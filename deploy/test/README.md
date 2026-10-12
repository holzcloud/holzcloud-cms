# deploy/test: the systemd test container

## What this is

A Debian container with systemd as PID 1 that runs the unit under test, `deploy/holzcloud.service`, unchanged. `deploy/test/run.sh`
builds the image, starts a privileged and throwaway container from it, and drives the checks through `docker exec`. It was built in
phase 20 to answer three questions (UNIT-01, UNIT-02, UNIT-03); the answers are in `deploy/SECURITY-SCORE.md`. Phases 21 (deploy, update, rollback)
and 22 (backup and restore) reuse the same recipe with other steps, so the interface below is stable: extend it, do not rename it.

Files:

- `deploy/test/run.sh`: the driver.
- `deploy/test/Dockerfile`, `deploy/test/Dockerfile.dockerignore`: the image and its build-context filter.
- `deploy/test/canary/`: a stdlib-only probe, its own Go module, that asks whether this platform enforces seccomp.
- `deploy/test/out/`: scratch output, gitignored.

## Quick start

Needs Docker (Docker Desktop is enough) and bash. Run from anywhere; the script changes into the repository root itself.

```text
# The whole recipe on the project's target architecture: build, up, score, start, canary, seed, check, mdwe-control, verdict.
PLATFORM=linux/amd64 ./deploy/test/run.sh all

# The narrow native run for the MemoryDenyWriteExecute evidence (the only arm64 use, see Deviations).
PLATFORM=linux/arm64 ./deploy/test/run.sh mdwe

# A stepwise session that keeps the container for your own commands.
KEEP=1 ./deploy/test/run.sh build up start
./deploy/test/run.sh exec -- systemctl status holzcloud
./deploy/test/run.sh exec -- curl -fsS http://127.0.0.1:8080/healthz
./deploy/test/run.sh down
```

`all` and `mdwe` remove the container when they end, unless `KEEP=1`. `build`, `up` and `start` never do; `down` does.

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

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success. |
| 1 | A real failure: a step failed, an assertion was `BAD`, the canary was invalid, or the verdict was FAILS or INCONSISTENT. |
| 2 | Usage: no subcommand, an unknown one, or `exec` without a command. The usage text is printed. |
| 3 | The step ran, but this platform cannot give the evidence. The canary returns it when seccomp is not enforced (so amd64 under emulation), and `verdict` returns it for VOID. `mdwe` passes it on. `all` does not: it tolerates 3 from both and exits 0. |

In a chain such as `run.sh build up start canary seed`, the `canary` step ends the call with 3 on a platform that does not enforce seccomp, and
`seed` never runs. Use `mdwe` or `all`, which know the convention.

## Images and targets

The Dockerfile has four stages; the last, `unit-test`, is the default target.

| Target | Content |
|---|---|
| `systemd-base` | Debian with systemd as PID 1 and `systemd-sysv`, `dbus`, `curl`, `ca-certificates`, `jq`, `procps`, `sqlite3`, `sudo`. No `holzcloud` user, no `/opt/holzcloud`, no binary, no unit. `systemd-modules-load.service` is masked, because it fails in a container. |
| `unit-test` | `systemd-base` plus the `holzcloud` user (the line from `deploy/DEPLOY.md`), `/opt/holzcloud/data` owned by that user, the binary `/opt/holzcloud/holzcloud` built from this tree, the unit in `/etc/systemd/system/` (copied unchanged, not enabled), the five plugin archives in `/opt/holzcloud/plugin-zips`, and the probe `/usr/local/bin/hc-canary`. |

`BASE_IMAGE=debian:trixie-slim` switches the base of either target to Debian 13 and systemd 257; the apt list is the same.
The image tag carries the base, so bookworm and trixie images do not overwrite each other.

## Output files

All under `OUT_DIR` (default `deploy/test/out/ARCH`). The directory is gitignored and is reset by `up`. The committed record of
phase 20 is `deploy/SECURITY-SCORE.md`, which pastes these files unchanged.

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

## Hand-over to phase 21

Phase 21 plays through a fresh install by `deploy/DEPLOY.md` alone, an update across a migration and a rollback.

- **Fresh install.** Start from `TARGET=systemd-base` (`build up` with that target): it has no user, no directories, no binary and no unit, which is what DEPLOY.md
  starts from. DEPLOY.md's `scp` steps become `docker cp` into the container. Its `sudo` commands run unchanged, because `sudo` is installed and the
  steps run as root through `run.sh exec`. Each DEPLOY.md step is one `run.sh exec -- ...` call, so every deviation between the document and the
  result is visible. The binary to install can be taken from a `unit-test` image (`/opt/holzcloud/holzcloud`) or built by the Dockerfile.
- **Two binaries.** The old and the new binary come from two `build` runs with different `VERSION` and `COMMIT` build arguments. The default `IMAGE` tag does not
  contain them, so give each run its own `IMAGE`, or the second build replaces the first.
- **Update and rollback.** The `unit-test` target has the user, the directory, the binary and the unit pre-installed. It is the starting point for update and
  rollback from the second step on, when the install itself is no longer the subject.
- **Starting the unit.** If a step starts the unit itself under amd64 emulation, it needs the same container-only drop-in that `start` applies
  (`ProcSubset=all`), or it should call `start`. Without it the first start crashes, see Pitfalls.
- `start` needs the unit and the binary to exist, so on `systemd-base` it fails until the install steps are done.
- **Two chains replay the whole proof** (plan 21-05), both from `TARGET=systemd-base` and both refusing to start from anything else. Each removes its
  container at the end (`KEEP=1` keeps it) and writes `OUT_DIR/summary.txt`; give each run its own `NAME` and `OUT_DIR`:
  - `fresh` (DEP-01): `build up install firstrun postinstall caddy`. The summary has `DEP-01 PASS`, the `CADDY:` verdict, `STEPS: N played, F failed`,
    `DEVIATIONS:` and `LEAK-CHECK OK`.
  - `lifecycle` (DEP-02, DEP-03): `build up install baseline update`, then `negative` when `update.txt` says `pending=` one or more (otherwise the
    scenario is recorded as skipped), then `rollback`. Needs `OLD_TAG`. The summary has `OLD:`, `NEW:`, `MIGRATIONS:`, `SNAPSHOT:`, `MODE:`,
    `NEGATIVE-RESULT:` (when played), `DEP-02 PASS` or `DEP-02 not applicable (no migration between these builds)`, `DEP-03 PASS` and `LEAK-CHECK OK`.
    `OLD_TAG=v0.0.10` crosses migrations 59 and 60 (MODE snapshot); `OLD_TAG=v0.0.21` has the same schema as HEAD (MODE binary, no snapshot).

## Hand-over to phase 22

Phase 22 proves that a backup can be restored.

- **A populated site.** `unit-test` plus `seed` gives a website with a domain, a published page, five plugins and one image: something worth backing up
  and restoring. `check` is a ready-made after-restore assertion (17 checks on the plugins and the page). It restarts the unit and reads
  the key file `/root/.hc-key`, which is outside the data directory and so not part of a backup; after a restore into a new container, `check` needs the key file again, and
  the key itself lives in the database.
- **`backup.sh` and what the image lacks.** `deploy/backup.sh` takes its snapshot with the binary (`holzcloud backup`) and copies media and templates with `rsync`.
  The image does not install `rsync`, and does not contain `deploy/backup.sh`, `deploy/restore.sh` or the two backup units, so phase 22 copies them in (as DEPLOY.md does with `scp`) and
  decides about `rsync`. DEPLOY.md's prerequisites still name the `sqlite3` CLI for backups, which the current `backup.sh` does not call; that is a documentation question for phase 22. The image does have `sqlite3`, and `check` uses it.
- **The backup directory.** `holzcloud-backup.service` writes to `/opt/holzcloud/backups`, and its own `ReadWritePaths` covers that path. The directory must exist and belong to the service
  user: DEPLOY.md creates it (`sudo mkdir -p /opt/holzcloud/backups`), the image does not.
- **Paths.** The unit's `ExecStart` is `/opt/holzcloud/deploy/backup.sh`; DEPLOY.md copies the script to `/opt/holzcloud/backup.sh` in the install steps and uses the `deploy/` path in the backup section. Check which one the run needs.
- **The timer.** Real systemd runs timers inside the container, so `holzcloud-backup.timer` can be observed for real. Its schedule is `OnCalendar=*-*-* 03:15:00` with
  `Persistent=true` and `RandomizedDelaySec=300`. How to bring that schedule into reach (a drop-in with a nearer time, or a changed clock) is phase 22's decision and must be named there
  as a deviation from the shipped timer.
- **Login.** `seed` creates an API key and no login user. The `holzcloud user create -email ... -role admin` command is the route phase 22 needs for BAK-02's login check; run it as the service user with `HOLZCLOUD_DATA_DIR` set.

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

## Safety

- The container is privileged and throwaway. `down` removes it; `run.sh` removes it on exit for `smoke`, `mdwe` and `all`.
- No host path is mounted and no port is published. The one file that goes in is the test image, through `docker cp`.
- The admin API key is created inside the container and stays in a root-only file there. It never reaches the host or any output file.
- `up` removes and recreates `OUT_DIR` and refuses `/`, `.`, `..` and the repository root.
- A stale container with the same `NAME` is never reused; `up` refuses it.
- The repository rule that nothing loads from a third party at runtime is untouched: nothing here ships in the binary. Docker pulls base images at image build time only.

## Deviations

All are recorded with their evidence in `deploy/SECURITY-SCORE.md`.

- The container-only drop-in `ProcSubset=all`, which the harness adds only after the known emulation crash. The shipped unit is untouched.
- D-01: one narrow native arm64 run, as MemoryDenyWriteExecute evidence only. UNIT-01 and UNIT-03 are measured on linux/amd64. No native amd64 run was made.
- Seccomp is not enforced on amd64 under the emulation, so seccomp-based directives are not exercised there; the document marks each as measured or inferred.
