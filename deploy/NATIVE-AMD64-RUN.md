# Native amd64 and the real server: hand-over

This document is the record of phase 26. It hands over three requirements that this machine cannot play: SRV-01 (the five plugins under `MemoryDenyWriteExecute=true` and the backup timings, on native amd64), SRV-02 (a restore on a real server, from a backup made there, to a different path) and SRV-03 (a reboot with the service and timer units enabled). It is not a result: until the owner's output is pasted and `bash deploy/test/native-amd64.sh record-check` accepts the `recorded` form, nothing in it is evidence for any of the three.

What exists is the instrument and the steps. The instrument is a button, `.github/workflows/native-amd64.yml` (manual dispatch only, a read-only token, no secret, the three official actions `ci.yml` already pins), the verdict rules and the checker in `deploy/test/native-amd64.sh`, and the harness `deploy/test/run.sh`, unchanged. The steps are O-1 to O-3 below; the expected output of O-2 and O-3 was observed in a container rehearsal and is quoted there as such. What does not exist is any native amd64 figure and any result from a real server. This record says what was prepared, what was seen while preparing it, and what is not claimed.

`deploy/SECURITY-SCORE.md` says "No native amd64 run was made and none is planned (user decision)". The milestone that opened on 2026-10-20 reversed that decision (SRV-01). The sentence, and the matching lines of `docs/beta.md`, stay as they are written: `docs/beta.md` is phase 28's, and `deploy/SECURITY-SCORE.md` is cited byte for byte by it. The hand-over is at the end of this record.

Every requirement is in one of two states, `pending owner` or `recorded`. `bash deploy/test/native-amd64.sh record-check` accepts the first without a claim and refuses the second unless the evidence for it is in the record; `bash deploy/test/native-amd64.sh status-check` holds `.planning/REQUIREMENTS.md` to the record. The state of this record is `pending owner` for all three.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| SRV-01 | PENDING OWNER. The button and the verdict rules exist and were exercised as far as an arm64 machine allows: the guard that keeps an arm64 run from reading as amd64 evidence ended `SRV-01 NOT-NATIVE` with exit 3 in the rehearsal of this plan. No native amd64 run has been made, no native figure replaces the emulation figures, and the plugins have not been seen under `MemoryDenyWriteExecute=true` on amd64. Outstanding: the owner presses the button (O-1) and the result is recorded | GitHub-hosted runner, linux/amd64, a privileged container with systemd 252 (not yet run) | not run |
| SRV-02 | PENDING OWNER. Owner step O-2 exists, with its expected output observed once in a container rehearsal. No restore has been made on a real server. Outstanding: the owner runs O-2 and the result is recorded | The owner's server (not yet run) | not run |
| SRV-03 | PENDING OWNER. Owner step O-3 exists, with its expected output observed once in a container rehearsal, where a container restart stood in for the reboot. No real server has been rebooted. Outstanding: the owner runs O-3 and the result is recorded | The owner's server (not yet run) | not run |

Evidence commit: none (pending owner)

The line above is the only one of its kind outside the appendix. It becomes the sha of the run's commit (the `COMMIT:` line of the pasted summary) in the commit that records SRV-01 as `recorded`, and in no other.

## How this was prepared

Three files exist because of this phase: `.github/workflows/native-amd64.yml`, `deploy/test/native-amd64.sh` (subcommands `selftest`, `run`, `summarize`, `down`, `record-check`, `status-check` and `appendix`) and this record. The commands below were run on 2026-10-26 on the development machine (an arm64 Mac, Docker Desktop 29.3.1) and nowhere else.

- `bash deploy/test/native-amd64.sh selftest` ends `SELFTEST OK (862 cases)`: the verdict functions on the real R1 and R2 texts of `deploy/SECURITY-SCORE.md` and the real chain summary of `deploy/BACKUP-RUN.md`, the summary writer, the evidence bundle with its refusals, and (this plan) `summary_check`, `record-check` in every refusal and in the recorded form of all three requirements, `status-check` and `appendix`. The summaries that the new cases need are made by `summarize` over a synthetic native set (the R2 texts with `aarch64` turned into `x86_64`), labelled synthetic in the code; no such set exists outside the temporary directory of the selftest.
- `bash deploy/test/nightly.sh selftest` still ends `SELFTEST OK (542 cases)`, and `record-check` and `record-check-con` of that file still end `RECORD-CHECK OK 29 blocks` and `RECORD-CHECK OK 27 blocks` (against the scratch files of phases 24 and 25). The file is not edited by this phase.
- `bash -n deploy/test/native-amd64.sh` is clean. `yq` reads the workflow as: one trigger, `workflow_dispatch`; `permissions: contents: read`; `runs-on: ubuntu-latest`; `timeout-minutes: 90`; three `uses:` lines, the same three pins as `ci.yml`. `actionlint` is not installed here and the workflow has not been run on GitHub.
- The arm64 rehearsal of the guard, `PLATFORM=linux/arm64 NATIVE_PREFIX=rehearsal-arm64-native NATIVE_STAGES=mdwe bash deploy/test/native-amd64.sh run`, from 16:11:44 to 16:12:22 CET on the host clock: `run.sh mdwe` ended with status 0 and `MDWE-WORD: SETTLED` on an aarch64 host and container with the canary enforcing, and `native-summary.txt` read `SRV-01-MDWE NOT-NATIVE`, `SRV-01-TIMING NOT-RUN`, `SRV-01 NOT-NATIVE`, ending `NATIVE-AMD64 DONE`. The script exited 3, the bundle held nine files, `down` left no container. An arm64 `SETTLED` did not read as amd64 evidence. The timing stage was not played there.
- The container rehearsal of O-2 and O-3 (quoted in the Owner steps): a `fresh`-style install without the proxy in a Debian 12 container with systemd 252 (`PLATFORM=linux/amd64 TARGET=systemd-base ... run.sh build up install firstrun postinstall`, from 15:57:27 to 15:58:02 CET), with the production unit enabled, the backup timer enabled by `postinstall`, one manual backup and an administrator account from `firstrun`. O-2 was played at 15:00:21Z and again as the refused variant at 15:01:52Z; O-3 was played at 15:01:27Z and again, with the output saved to files as the step now says, at 15:12:37Z. Deviations: amd64 under emulation on an aarch64 host; the container-only drop-in `ProcSubset=all` on the production unit and, for O-2, on the throwaway unit; a container restart for a reboot, with the start time of PID 1 compared instead of the boot id; the restore question read through `script(1)`; a few fixture files in `media`, `templates` and `sprachen`. One command block had to be corrected while rehearsing: the first O-3 draft kept its output only on the terminal, which does not survive a reboot, so the blocks now `tee` into the owner's home (the first draft was played at 15:01:27Z, the final text at 15:12:37Z). Every other command ran as first written.
- The frozen files are unchanged: `git diff --quiet 05fc076 -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud.service deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/nightly.sh` exited 0, and the same command for the earlier records `deploy/SECURITY-SCORE.md deploy/DEPLOY-RUN.md deploy/BACKUP-RUN.md deploy/BACKUP-TIMER-RUN.md deploy/BACKUP-CONSISTENCY-RUN.md` exited 0.
- Nothing was pushed, nothing was dispatched on GitHub, nothing was run on a server, and the rehearsal container is removed.

## SRV-01: the five plugins under MemoryDenyWriteExecute=true on native amd64, and native backup timings

**What it will show.** Whether the five shipped plugins (`kontaktformular`, `bestellung`, `nicht-gefunden`, `suche`, `jahreszahl`) load and run under the shipped `holzcloud.service` with `MemoryDenyWriteExecute=true` on a native x86_64 kernel that enforces seccomp, and which wazero engine runs them (`ENGINE-SHIPPED`); and what `backup.sh` and `restore.sh` cost on native amd64, as two data points (the seed site, and the same site with 300 MB of synthetic media), set against the emulation figures below. The work is done by the existing harness: `run.sh mdwe` (build, up, start, canary, seed, check, mdwe-control, verdict) and `run.sh backup-chain` with `BULK_MB=300`, the recipe that made the emulation figures. The new rule is on top: `deploy/test/native-amd64.sh` decides the word.

**What it does not show.** The runner's own systemd (Ubuntu 24.04, systemd 255) is not exercised; the work is done in a container with Debian's systemd 252, the recipe already proven on native arm64 (R2 of `deploy/SECURITY-SCORE.md`). A runner is a virtual machine on shared hardware with a cold cache, not the owner's server, and two readings are two readings, not a rate. Only two directives were probed on arm64 and only the plugin runtime is exercised here, not the whole of `systemd-analyze security`.

The figures the native ones are set against, from the `POINT` lines of `deploy/BACKUP-RUN.md` (amd64 under emulation on an aarch64 host):

| Figure | Emulation, seed point | Native (pending owner) | Emulation, bulk point (300 MB) | Native (pending owner) |
|--------|-----------------------|------------------------|--------------------------------|------------------------|
| `BACKUP-WALL-S` | 0.692 | | 1.015 | |
| `RESTORE-WALL-S` | 8.126 | | 8.296 | |
| `START-READY-S` | 7.397 | | 7.379 | |

```text
POINT seed: BACKUP-BYTES=20548955 BACKUP-WALL-S=0.692 RESTORE-WALL-S=8.126 VERIFY-S=0.083 STOP-MOVE-COPY-S=0.455 START-READY-S=7.397
POINT bulk: BACKUP-BYTES=335125851 BACKUP-WALL-S=1.015 RESTORE-WALL-S=8.296 VERIFY-S=0.088 STOP-MOVE-COPY-S=0.631 START-READY-S=7.379
```

The verdict rule, as the script applies it and as `record-check` recomputes it from the lines of the pasted summary: PASS only when the Docker host and the container are x86_64, `run.sh`'s own word is SETTLED (the canary says the directive is enforced, all assertions hold under the shipped unit, and the control with the directive off runs on the compiler), and the chain summary has `BAK-01`, `BAK-02`, `BAK-03` and `RESTORE-NEGATIVE` as PASS, `LEAK-CHECK OK` and numeric `POINT seed:` and `POINT bulk:` lines. A PASS says that the five plugins run on wazero's interpreter fallback under the shipped unit, which is what the unit's comment says. The other words: VOID (the canary reports `Seccomp: 0`, so the plugin run proves nothing about the directive, and the requirement stays open; exit 3), NOT-NATIVE (the Docker host or the container is not x86_64, decided first so that an arm64 SETTLED never reads as amd64 evidence; exit 3), FAIL (the plugins do not load under the shipped unit, or a chain line is missing; exit 1) and NOT-RUN (a stage was not played; exit 1). Only PASS can be recorded.

Status: pending owner
Result (owner): pending

```text
(nothing pasted yet)
```

## SRV-02: a restore on a real server, from a backup made there, to a different path

**What it will show.** That a backup made on the owner's server by the shipped `backup.sh`, restored by the shipped `restore.sh` to a path other than the production data directory, starts as a service of its own (a second unit with its own data path and port), answers `/readyz` with `"integrity":"ok"`, and holds the same trees as the backup (`media`, `templates`, `plugins`, `sprachen`, `marke`, each if the backup has it). The copy is a second unit on the same server, not a spare machine. That is the strongest thing the shipped `restore.sh` allows without being edited: it stops and starts the unit named in `SERVICE`, replaces `DATA_DIR`, and polls the production `/readyz` on port 8080 whatever `DATA_DIR` says. `deploy/DEPLOY.md` warns that a restore pointed at a test directory proves nothing; O-2 answers that by checking the copy on its own port and its own tree, which the script's closing line does not do.

**What it does not show.** A spare machine, a real network copy, a cold cache, a data directory on its own mount, the proxy and the certificates in front of the copy, and the timing of a restore on the production port. The throwaway unit has no payment, mail or secret-key lines, so what the copy can do outside the machine is not played either.

Status: pending owner
Result (owner): pending

```text
(nothing pasted yet)
```

## SRV-03: a reboot with the service and timer units enabled

**What it will show.** That on the owner's server, with `holzcloud.service` and `holzcloud-backup.timer` enabled, a reboot brings the site back without a hand on it: PID 1 has a new start time, both units are active, `holzcloud.service` has not been restarted on the way (`NRestarts=0`, `Result=success`), `/readyz` answers `"integrity":"ok"`, the timer is armed with a next run, and the plugins that are installed are loaded in the journal of this boot.

**What it does not show.** A reboot here is a clean one (`systemctl reboot`), not a power loss; the catch-up of a missed 03:15 run after a power cycle is not played (it needs the clock to pass 03:15); the proxy, the certificate and DNS after the boot are shown only by the optional last line; the ordering of the network, the clock and the mount against the service is what the machine did, not something measured.

Status: pending owner
Result (owner): pending

```text
(nothing pasted yet)
```

## Container vs. server

| Aspect | In the container or on the runner | On the owner's server | Measured or assumed |
|--------|-----------------------------------|-----------------------|---------------------|
| The platform of SRV-01 | A GitHub-hosted runner: an Azure virtual machine with an Ubuntu image, its own systemd 255 not exercised; the work is done in a privileged container with Debian 12 and systemd 252 | The owner's server, Debian or Ubuntu with its own systemd | Not measured; the runner path is assumed to boot systemd in a privileged container with the default flags (the `docker_run_args` input is the hook if it does not) |
| A privileged container | `--privileged`, a cgroup namespace the runner's Docker decides, no published port | A plain host | Measured on arm64 (R2), assumed on the runner |
| Emulation against native | The rehearsals here ran amd64 under emulation on aarch64; the native run has x86_64 hardware and a kernel that enforces seccomp | The same hardware class as the runner or better | The emulation figures are measured (`deploy/BACKUP-RUN.md`); the native ones are pending owner |
| The second unit against a spare machine | O-2 runs the copy as a second unit on the same host, on port 8081, with its own data path | The same: the copy lives next to production for the length of the step | Measured in the container rehearsal; on the server pending owner |
| A container restart against a reboot | `docker restart`: the Docker VM's kernel stays, `uptime -s` and the boot id do not change, `journalctl -b` does not separate the starts | A new kernel, a new boot id, the unit started by the boot target | Measured in the rehearsal as a restart (PID 1's start time changed); a reboot is pending owner |
| Journal and boot ordering | Whatever the container's journal holds; no network target, mount or clock synchronisation to wait for | The machine's own | Assumed |
| Warm cache | A container started minutes before its backup, the data in the page cache | A cold cache after a reboot, and a restore from a backup that may be days old | Not measured |
| The unit drop-in | `ProcSubset=all` on the units in the emulated rehearsals (the crash it answers is an emulation crash); the runner path starts the unit as shipped unless the harness meets the same crash | The unit as shipped | Measured on emulation; native pending owner |

## Not tested

- A spare machine across a real network, and the pull of an off-site copy back to a rebuilt server (O-3 and O-4 of `deploy/BACKUP-TIMER-RUN.md` as they stand).
- A power loss and the catch-up of a missed 03:15 backup after it (O-2 of `deploy/BACKUP-TIMER-RUN.md`); the reboot of SRV-03 is a clean one.
- Ubuntu 24.04 with systemd 255 on the host itself, outside a container, and systemd versions other than 252 (FUT-05).
- The proxy and the certificates after the reboot (FUT-03), and sign-in through the identity service (FUT-04).
- A cold cache, and a data directory on its own mount.
- Whether a backup made by a server with its payment, mail and secret-key lines restores into a copy that has none of them: the copy is meant to run without them, and nothing here sends anything.

## Owner steps

Each step is for the owner, with the commands in fenced blocks, the output to expect, what a different output means, and where the result is pasted. `deploy/BACKUP-TIMER-RUN.md` says that phase 26 takes its O-2 (the catch-up after a real power cycle) and its O-4 (the restore on a rebuilt box) into its own owner steps. O-3 below covers the part of that O-2 that does not need the clock to pass 03:15, a reboot with the timer enabled and armed afterwards, and O-2 below covers the restore on the real server; the catch-up of a missed 03:15 run and the pull of an off-site copy stay named in *Not tested*. The expected output of O-2 and O-3 was observed once in a **container rehearsal** (a Debian 12 container with systemd 252, amd64 under emulation on an aarch64 host, the `fresh` install path without the proxy); it is quoted below as *Container rehearsal, not a result*, with the deviations named next to it. A rehearsal shows what the commands print when everything works, nothing more.

Nothing in these steps prints a password, a key or a token. The domain name and the host name are fine to paste. Nothing else needs removing; if an output ever shows a secret, stop and ask before pasting it anywhere.

**O-1. Native amd64: the five plugins under the shipped unit and the backup timings (SRV-01).** Press the button. It runs the existing harness on a GitHub-hosted amd64 runner (`runs-on: ubuntu-latest`, a privileged container with systemd 252 and the unit unchanged), needs no secret, and changes nothing in the repository. From a terminal with the GitHub CLI, or with Actions, native-amd64, Run workflow in the browser:

```bash
gh workflow run native-amd64.yml --ref main
gh run list --workflow native-amd64.yml --limit 1
gh run watch
```

When the run has finished, take its number from the list and unpack the evidence artifact so that `amd64-native/`, `amd64-native-mdwe/` and the other directories sit directly under `deploy/test/out/` of your checkout:

```bash
gh run download RUN-ID --name native-amd64-evidence --dir deploy/test/out
```

Expect the job to end green. The Summary page of the run shows one block from `---- native amd64 summary ----` to `NATIVE-AMD64 DONE`, and in it `HOST-ARCH: x86_64`, `CONTAINER-ARCH: x86_64`, `CANARY: enforced`, `ENGINE-SHIPPED: interpreter` (the plugins run on wazero's interpreter fallback under the directive, which is what the unit's comment says and what native arm64 showed), `MDWE-WORD: SETTLED`, `BAK-01 PASS`, `BAK-02 PASS`, `BAK-03 PASS`, `RESTORE-NEGATIVE PASS`, the two lines `POINT seed:` and `POINT bulk:` with their `BACKUP-WALL-S=`, `RESTORE-WALL-S=` and `START-READY-S=` figures, `LEAK-CHECK OK`, the three verdict lines and, as the last verdict line, `SRV-01 PASS`. The shape is this; every value in it that is not a fixed word is a placeholder, and none of it is a result (derived from the script and from the arm64 rehearsal of plan 26-02, not observed on amd64):

```text
---- native amd64 summary ----
COMMIT: <40 hex characters of the commit the run used>
RUN: <URL of the run> attempt <n>
HOST-ARCH: x86_64
CONTAINER-ARCH: x86_64
CANARY: enforced
ENGINE-SHIPPED: interpreter
ENGINE-CONTROL: compiler
ASSERTIONS-SHIPPED: 17
ASSERTIONS-CONTROL: 17
MDWE-WORD: SETTLED
MDWE-STATUS: 0
TIMING-STATUS: 0
BAK-01 PASS
BAK-02 PASS
BAK-03 PASS
RESTORE-NEGATIVE PASS
POINT seed: BACKUP-BYTES=<n> BACKUP-WALL-S=<s> RESTORE-WALL-S=<s> ...
POINT bulk: BACKUP-BYTES=<n> BACKUP-WALL-S=<s> RESTORE-WALL-S=<s> ...
LEAK-CHECK OK
SRV-01-MDWE PASS
SRV-01-TIMING PASS
SRV-01 PASS
NATIVE-AMD64 DONE
```

A different output has a name, and only `SRV-01 PASS` can be recorded. `SRV-01 VOID` with `CANARY: not-enforced` (and `Seccomp: 0` in the files) means the runner's kernel or Docker installed no seccomp filter, so the plugin run proves nothing about the directive; run it once more, and if it repeats, the requirement stays open and the run is itself a finding. `SRV-01 NOT-NATIVE` means the runner or the Docker host was not x86_64; nothing is recorded. `MDWE-WORD: FAILS` and `SRV-01 FAIL` mean the plugins do not load under the shipped unit on native amd64: that is a finding for the owner (the unit's own comment names the change, `MemoryDenyWriteExecute=false`; it is not made here). `systemd did not reach running or degraded within 90s` in the log means the container's systemd did not boot with the default flags: run the workflow again with the input `--cgroupns=host -v /sys/fs/cgroup:/sys/fs/cgroup:rw`. A red job with no artifact means the evidence bundle was refused because a file looked like a secret; read the log line, and publish nothing.

Paste the block from `---- native amd64 summary ----` to `NATIVE-AMD64 DONE` into the `Result (owner)` block of SRV-01 in this record, unchanged. The artifact stays on your disk; its files become the appendix (see *Recording a result*).

**O-2. A restore on this server, from a backup made here, to a different path (SRV-02).** Run on the server that carries the site, as a user who may use `sudo`, all blocks in one shell session (the variables `T`, `P` and `B` carry from one block to the next). The restore goes to a second, throwaway unit with its own data path and its own port, so the production data is never replaced. Two things about the shipped `restore.sh` shape the steps. First, it stops and starts the unit named in `SERVICE` and replaces `DATA_DIR`, so both are handed to it with `sudo env`. Second, it polls the **production** `http://127.0.0.1:8080/readyz` whatever `DATA_DIR` says, so its closing line `=== Restore complete ===` is not about the copy; the copy is checked on its own port and by comparing its tree with the backup. The script is not edited.

First look, and change `T` or `P` if the path exists or the port answers (`copy port ... 000` means nothing is listening, which is what is wanted):

```bash
T=/srv/holzcloud-restore-test
P=8081
test -e "$T" && echo "STOP: $T exists, choose another path" || echo "$T is free"
systemctl is-active holzcloud
curl -s -o /dev/null -w "production %{http_code}\n" http://127.0.0.1:8080/readyz
curl -s -o /dev/null -w "copy port $P %{http_code}\n" http://127.0.0.1:$P/readyz || true
```

Make a backup with the document's own line and keep its directory in `B`:

```bash
B=$(sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups | tee /dev/stderr | sed -n 's/^=== Backup complete: \(.*\) ===$/\1/p')
echo "B=$B"
test -n "$B" || echo "STOP: no backup directory"
```

Make the path and the throwaway unit. The unit is derived from the unit that is installed, with the data path and the port replaced and every line removed that could give the copy access to payment, mail or a secret key, so that the copy cannot send anything. Read the lines the `grep` shows before going on:

```bash
sudo install -d -o holzcloud -g holzcloud -m 700 "$T"
sed -e "s#/opt/holzcloud/data#$T/data#g" -e "s#^Environment=HOLZCLOUD_PORT=8080#Environment=HOLZCLOUD_PORT=$P#" \
  -e '/^EnvironmentFile=/d' -e '/^Environment=HOLZCLOUD_PAYREXX/d' -e '/^Environment=HOLZCLOUD_SMTP/d' -e '/^Environment=HOLZCLOUD_SECRET/d' \
  /etc/systemd/system/holzcloud.service | sudo tee /etc/systemd/system/holzcloud-restore-test.service > /dev/null
grep -nE '^(Environment|EnvironmentFile|ReadWritePaths|RequiresMountsFor)=' /etc/systemd/system/holzcloud-restore-test.service
grep -c -e '/opt/holzcloud/data' -e '^EnvironmentFile' /etc/systemd/system/holzcloud-restore-test.service || true
sudo systemctl daemon-reload
```

Now the restore itself. Read the question before answering:

```bash
sudo env DATA_DIR="$T/data" SERVICE=holzcloud-restore-test /opt/holzcloud/deploy/restore.sh "$B"
```

The question must read `Stop holzcloud-restore-test and replace /srv/holzcloud-restore-test/data? [yes/NO]` (with your `T`). Answer `yes` only then. If the question names `holzcloud` and `/opt/holzcloud/data`, the variables were lost on the way and the script would replace production: answer `no` (or press Ctrl-C); nothing has been stopped or moved at that point.

Then check the copy on its own port and its own tree, the checks that `restore.sh` cannot make. The `curl: (7) Failed to connect` lines are the retries while the copy starts; `YOUR-DOMAIN` is a domain of one of your websites:

```bash
R=$(curl -fsS --retry 30 --retry-connrefused --retry-delay 1 "http://127.0.0.1:$P/readyz"); echo "$R"
case "$R" in *'"integrity":"ok"'*) echo "SRV-02 INSTANCE READY" ;; esac
curl -s -o /dev/null -w "setup on the copy: %{http_code}\n" "http://127.0.0.1:$P/admin/setup"
curl -s -o /dev/null -w "/ on production: %{http_code}\n" -H "Host: YOUR-DOMAIN" http://127.0.0.1:8080/
curl -s -o /dev/null -w "/ on the copy: %{http_code}\n" -H "Host: YOUR-DOMAIN" http://127.0.0.1:$P/
sudo env B="$B" T="$T" sh -c '
ok=1
for d in media templates plugins sprachen marke; do
  if [ -d "$B/$d" ]; then
    if diff -r "$B/$d" "$T/data/$d" > /dev/null; then echo "equal   $d"; else echo "DIFFERS $d"; ok=0; fi
  else
    echo "absent  $d (not in this backup)"
  fi
done
[ "$ok" = 1 ] && echo "SRV-02 TREES EQUAL"
'
```

Last, clean up, and show that production is untouched and that nothing is left behind:

```bash
sudo systemctl stop holzcloud-restore-test
sudo rm /etc/systemd/system/holzcloud-restore-test.service
sudo systemctl daemon-reload
sudo rm -rf -- "${T:?}"
systemctl is-active holzcloud
ls -d /opt/holzcloud/data.before-restore-* 2> /dev/null || echo "no data.before-restore-* directory"
test -e "$T" && echo "STOP: $T is still there" || echo "$T is gone"
curl -s -o /dev/null -w "copy port $P %{http_code}\n" http://127.0.0.1:$P/readyz || true
```

Expect, block by block: `STOP` appears nowhere; the first block prints `active`, `production 200` and `copy port 8081 000`; the backup prints its progress and ends `=== Backup complete: /opt/holzcloud/backups/YYYYMMDD-HHMMSS ===`, and `B=` names that directory; the `grep` shows the data path, the port and `ReadWritePaths` of the copy and none of the removed lines, and the count is `0`; the restore prints the manifest, `integrity: ok`, the question, one `Restoring ...` line per directory the backup holds, `=== Restore complete ===` and the **production** readiness answer; the copy answers `"integrity":"ok"` (`SRV-02 INSTANCE READY`), `setup on the copy` is `404` once an account exists, the two `/` codes are equal, every present directory is `equal`, the last line is `SRV-02 TREES EQUAL`; the clean-up prints `active`, `no data.before-restore-* directory`, `... is gone` and `copy port 8081 000`.

Container rehearsal, not a result (2026-10-26, 15:00:21Z to 15:00:27Z; the transcript is in the gitignored scratch directory and is not pasted anywhere). The question as the script prints it with a terminal, then the lines of the rehearsal that carry the markers:

```text
Stop holzcloud-restore-test and replace /srv/holzcloud-restore-test/data? [yes/NO]
=== Backup complete: /opt/holzcloud/backups/20261026-150021 ===
B=/opt/holzcloud/backups/20261026-150021
0
=== Restore complete ===
{"status":"ok","version":"v0.1.0-53-g8359caa","commit":"8359caa","uptime_s":149,"integrity":"ok","disk_free_bytes":1817958727680,"database":{"freelist_count":0,"page_count":155,"size_bytes":4096,"wal_bytes":2356672}}
{"status":"ok","version":"v0.1.0-53-g8359caa","commit":"8359caa","uptime_s":0,"integrity":"ok","disk_free_bytes":1817959329792,"database":{"freelist_count":0,"page_count":154,"size_bytes":630784,"wal_bytes":0}}
SRV-02 INSTANCE READY
setup on the copy: 404
/ on production: 404
/ on the copy: 404
equal   media
equal   templates
equal   plugins
equal   sprachen
absent  marke (not in this backup)
SRV-02 TREES EQUAL
active
no data.before-restore-* directory
/srv/holzcloud-restore-test is gone
copy port 8081 000
```

The first JSON line is what `restore.sh` printed after `=== Restore complete ===`: it is the production service (`uptime_s` 149), not the copy. The second is the copy (`uptime_s` 0, a freshly started process on the restored database). That difference is the trap in one pair of lines. The rehearsal's deviations: the container is amd64 emulated on an aarch64 host, so the copy's unit got the same container-only drop-in as the production unit (`ProcSubset=all`, see `deploy/test/README.md`, Pitfalls), and the owner steps contain no such line; the answer was fed on standard input through `script(1)` so that the question appears as it does with a terminal (piped input prints no question); the data directory was a fresh install plus three small fixture files (`media`, `templates`, `sprachen`), so the trees compared are small, the site has no domain and `/` answers 404 on both ports (equal, which is all the check asks); the shell was root's, where the owner's is a user with `sudo`.

A different output means: a question that names `holzcloud` or `/opt/holzcloud/data` is the lost variable above (answer `no`; the rehearsal of 15:01:52Z answered `no` to exactly that question and the script ended `aborted` with production `active` and no `data.before-restore-*` directory: the `Stop holzcloud and replace /opt/holzcloud/data? [yes/NO]` line). A `/readyz` of the copy that never answers means the copy did not start: read `journalctl -u holzcloud-restore-test -n 50 --no-pager`; the usual causes are a port already in use, a unit directive that differs on your server, or a data path the unit cannot write. `"integrity"` anything but `ok`, or a `DIFFERS` line (list the differences with `sudo diff -rq "$B/media" "$T/data/media"`), means the backup or the restore did not reproduce the data: keep `$T` as it is and report it; it is a finding, not a retry. `setup on the copy: 200` means the copy has no administrator account, so the backup does not hold the users. Two `/` codes that differ mean the copy does not serve what production serves. A `data.before-restore-...` name with today's time in the clean-up means production was replaced: do not delete anything, stop here and ask.

Paste everything the blocks printed, from the first block to the clean-up, into the `Result (owner)` block of SRV-02 in this record. The lines `=== Backup complete: `, `=== Restore complete ===`, `SRV-02 INSTANCE READY` and `SRV-02 TREES EQUAL` have to be in it; the checker looks for them at the beginning of a line.

**O-3. A reboot with the service and the backup timer enabled (SRV-03).** Choose the time (the site is down for the length of the boot) and keep a console or a second route to the machine in reach. The blocks save their output in your home directory, because the terminal does not survive the reboot, and `/tmp` is emptied at boot. Before the reboot:

```bash
{
  systemctl is-enabled holzcloud holzcloud-backup.timer
  test "$(systemctl is-enabled holzcloud holzcloud-backup.timer | sort -u)" = enabled && echo "SRV-03 UNITS ENABLED"
  systemctl is-active holzcloud holzcloud-backup.timer
  ps -o lstart= -p 1 | sed 's/^ *//' | tee ~/srv03-boot-before.txt
  curl -fsS http://127.0.0.1:8080/readyz
  echo
} 2>&1 | tee ~/srv03-before.txt
sudo systemctl reboot
```

Log in again when the machine is back, and run the second block; give the machine a minute if `systemctl is-system-running` still says `starting`, and run the block again (it overwrites its own file):

```bash
{
  systemctl is-system-running
  cat ~/srv03-boot-before.txt
  ps -o lstart= -p 1 | sed 's/^ *//'
  test "$(ps -o lstart= -p 1 | sed 's/^ *//')" != "$(cat ~/srv03-boot-before.txt)" && echo "SRV-03 BOOT CHANGED"
  systemctl is-active holzcloud holzcloud-backup.timer
  test "$(systemctl is-active holzcloud holzcloud-backup.timer | sort -u)" = active && echo "SRV-03 UNITS ACTIVE"
  systemctl show holzcloud -p NRestarts -p Result -p ActiveEnterTimestamp
  R=$(curl -fsS --retry 30 --retry-connrefused --retry-delay 1 http://127.0.0.1:8080/readyz)
  echo "$R"
  case "$R" in *'"integrity":"ok"'*) echo "SRV-03 SITE READY" ;; esac
  systemctl list-timers holzcloud-backup.timer --no-pager
  test -n "$(systemctl show holzcloud-backup.timer -p NextElapseUSecRealtime --value)" && echo "SRV-03 TIMER ARMED"
  journalctl -u holzcloud -b --no-pager | grep -c 'plugin loaded'
} 2>&1 | tee ~/srv03-after.txt
cat ~/srv03-before.txt ~/srv03-after.txt
```

For a site behind the proxy, one more line, with your domain, shows the whole path back (optional, and the first status line is all that is asked for): `curl -fsSI https://YOUR-DOMAIN/ | head -n 1`.

Expect `enabled` twice and `SRV-03 UNITS ENABLED` before; after the boot `running` (or `degraded` with `systemctl --failed` naming something that is not ours), two different start times of PID 1 with `SRV-03 BOOT CHANGED`, `active` twice and `SRV-03 UNITS ACTIVE`, `Result=success` and `NRestarts=0` with an `ActiveEnterTimestamp` that is after the boot, a readiness answer with `"integrity":"ok"` and `SRV-03 SITE READY`, one timer row whose NEXT column is a time of day (03:15 plus up to 300 s of random delay) and `SRV-03 TIMER ARMED`, and a last number that is one `plugin loaded` line per installed plugin in this boot's journal (`0` if none is installed).

Container rehearsal, not a result (2026-10-26, 15:12:37Z to 15:12:41Z; a container restart stands in for the reboot). The lines of the rehearsal:

```text
enabled
enabled
SRV-03 UNITS ENABLED
active
active
Mon Oct 26 15:01:27 2026
running
Mon Oct 26 15:01:27 2026
Mon Oct 26 15:12:37 2026
SRV-03 BOOT CHANGED
active
active
SRV-03 UNITS ACTIVE
Result=success
NRestarts=0
ActiveEnterTimestamp=Mon 2026-10-26 15:12:39 UTC
{"status":"ok","version":"v0.1.0-53-g8359caa","commit":"8359caa","uptime_s":0,"integrity":"ok","disk_free_bytes":1817617743872,"database":{"freelist_count":0,"page_count":157,"size_bytes":643072,"wal_bytes":0}}
SRV-03 SITE READY
NEXT                        LEFT     LAST                        PASSED    UNIT                   ACTIVATES
Tue 2026-10-27 03:19:04 UTC 12h left Mon 2026-10-26 14:58:00 UTC 14min ago holzcloud-backup.timer holzcloud-backup.service
SRV-03 TIMER ARMED
0
```

The deviations of the rehearsal: `docker restart` shares the kernel of the Docker VM, so there is no new kernel, `uptime -s` and the boot id do not change and `journalctl -b` does not separate the two starts; this is why the step compares the start time of PID 1, which does change in the container as it does on a machine (the rehearsal waited for `systemctl is-system-running` before the second block); the timer's row shows a LAST time because `postinstall` of the harness had made one manual backup and the timer's stamp is in the container's writable layer; no plugin was installed, so the count is `0`; no proxy was installed, so the optional line was not played; the clock of the container is the VM's.

A different output means: `SRV-03 UNITS ENABLED` missing before the reboot means one unit is not enabled (`sudo systemctl enable --now holzcloud holzcloud-backup.timer`, then start again from the first block); the reboot would prove nothing. The two start times of PID 1 being equal means the machine did not reboot, so there is no `SRV-03 BOOT CHANGED` and the result is not recordable. `activating` or `failed` for `holzcloud` after the minute is up means the service did not come back by itself: `journalctl -u holzcloud -b --no-pager | tail -n 50` names the reason (a data directory on a mount that was not ready is the one the unit's `RequiresMountsFor` guards against), and that failure is itself the finding. `NRestarts` above `0` means the service started, failed and was restarted: look at the journal before calling it a success. A timer with an empty NEXT column, or `inactive`, means the timer is not armed. If the site does not come back at all, use the console: nothing here changes the machine except the reboot.

Paste the output of the last `cat` (before and after together) into the `Result (owner)` block of SRV-03 in this record. The lines `SRV-03 UNITS ENABLED`, `SRV-03 BOOT CHANGED`, `SRV-03 UNITS ACTIVE`, `SRV-03 SITE READY` and `SRV-03 TIMER ARMED` have to be in it; the checker looks for them at the beginning of a line.

**Recording a result.** One commit records one result, and it changes: the pasted block, `Status:` and `Result (owner):` of that requirement (both read `recorded` and the paste block holds the result instead of `(nothing pasted yet)`), the Summary row of that requirement (no longer `PENDING OWNER`), for SRV-01 only the `Evidence commit:` line (the sha of the run's `COMMIT:` line, at least 7 characters, a commit this repository knows) and the appendix (`bash deploy/test/native-amd64.sh appendix`, with the artifact unpacked under `deploy/test/out`, appended below the appendix heading), and the requirement in `.planning/REQUIREMENTS.md` (the box `[x]` and the traceability cell `Complete`). Then `bash deploy/test/native-amd64.sh record-check` and `bash deploy/test/native-amd64.sh status-check` must both end `OK`; whoever cannot make them end OK has not recorded a result. A run that ended `VOID`, `FAIL`, `NOT-NATIVE` or `NOT-RUN` is a finding, written down as one, and leaves the status at `pending owner`.

## Hand-over to phase 28

Phase 28 (DOC-01, DOC-02) cites this record. `docs/beta.md` is not edited by this phase: `git diff --quiet f1ba5a2 -- docs/beta.md` exited 0. These are the five lines of that file that this record bears on, each as it stands:

```text
- A restore on a real server was not made: every restore went into a throwaway container, with no separate data volume, a warm cache and no real network copy.
- Native amd64 timing was not measured: every number is an emulation number on an aarch64 host, and the extrapolation in the record is an estimate.
- A reboot with the units enabled was not made.
- No native amd64 run was made and none is planned (a user decision); the assumption stated for `MemoryDenyWriteExecute` on amd64 stays an assumption.
- Seccomp enforcement was observed on native arm64 only, and only `MemoryDenyWriteExecute` and the address-family directive were probed there, while the target platform is linux/amd64.
```

| Line | Answered by | What phase 28 may do with it |
|------|-------------|------------------------------|
| A restore on a real server was not made | SRV-02 (O-2) | Move it when the `Status:` of SRV-02 is `recorded` |
| Native amd64 timing was not measured | SRV-01 (O-1), the timing half | Move it when the `Status:` of SRV-01 is `recorded`; the figures to quote are the `POINT` lines of the pasted summary |
| A reboot with the units enabled was not made | SRV-03 (O-3) | Move it when the `Status:` of SRV-03 is `recorded` |
| No native amd64 run was made and none is planned | SRV-01 (O-1) | The milestone reversed the decision (see the opening of this record); reword it when SRV-01 is `recorded`, and keep it while it is `pending owner`, because it is true until then |
| Seccomp enforcement was observed on native arm64 only | SRV-01 (O-1), the plugin half | Only the part about `MemoryDenyWriteExecute` on amd64 can change, and only when SRV-01 is `recorded`; the address-family directive was not probed by this phase |

The rule is the same for all five: a line moves only when the matching `Status:` is `recorded` and `bash deploy/test/native-amd64.sh record-check` ends `OK` (and `status-check` with it). While any of O-1, O-2 and O-3 is `pending owner`, DOC-02's OWNER-STEPS lists it, with the steps of this record. The sentence of `deploy/SECURITY-SCORE.md` that says no native amd64 run is planned (user decision) is superseded by SRV-01 and is not edited: `docs/beta.md` cites that file byte for byte, so phase 28 decides whether and how the citation changes.

## Appendix: raw output

This appendix is empty until SRV-01 is recorded. It is assembled with `bash deploy/test/native-amd64.sh appendix`: a fixed list of thirteen files, each as a level-four heading `DIRECTORY/FILE` and a text block that holds the file unchanged, taken from the evidence artifact unpacked under `deploy/test/out`. It is pasted below this paragraph in the commit that records SRV-01. `record-check` then compares every block with its file byte for byte (a missing final newline is normalised) and the block of `amd64-native/native-summary.txt` with the `Result (owner)` block of SRV-01. The files that are never pasted are `step-NN.txt`, `seed.txt`, logs, cookie jars and key files; the evidence bundle drops them before the artifact is made.
