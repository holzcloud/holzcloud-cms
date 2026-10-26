# Native amd64 and the real server: hand-over

This document is the record of phase 26. It hands over three requirements that this machine cannot play: SRV-01 (the five plugins under `MemoryDenyWriteExecute=true` and the backup timings, on native amd64), SRV-02 (a restore on a real server, from a backup made there, to a different path) and SRV-03 (a reboot with the service and timer units enabled). It is not a result: until the owner's output is pasted and `bash deploy/test/native-amd64.sh record-check` accepts the `recorded` form, nothing in it is evidence for any of the three.

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
