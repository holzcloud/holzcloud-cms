# Phase 28: the owner's steps

Claude ran none of the commands in this file. A step marked as taken from a record carries that record's text unchanged, between the two comment lines of its verbatim region: the region is copied by a program (`28-check.sh extract`) and compared with `cmp`, so a step cannot drift from its record. The words outside the two comment lines are this file's own. A step whose Source is `new` has a Rehearsal line that says what was played in a throwaway repository, what was read from GitHub with a plain GET call and what was only read in a workflow file and not seen.

The order in which the steps are taken:

- **OS-1 to OS-4 first.** The button of OS-5 and the release of OS-16 need `main` on GitHub. The records under `deploy/`, `docs/beta.md` and the workflow `native-amd64` exist there only after OS-3, and the release body links to them.
- **OS-5** after OS-4: it is the button of the workflow `native-amd64` and runs on GitHub, so it needs the workflow to exist there.
- **OS-6 to OS-14** whenever the server runs the units that this release ships. They need no GitHub step, and none of them needs another in a particular order except where a Needs line says so.
- **OS-15 to OS-18 only after the milestone has been completed.** The number, the date and the tag belong to that step; the number 0.1.1 and the date 2026-10-26 that stand in `CHANGELOG.md` are drafts. The release is not made by the author of this file, and no step of it has been taken: there is no tag `v0.1.1`.
- **OS-19** at any time.

The real-server steps assume a server installed by `deploy/DEPLOY.md`: the program and the units under `/opt/holzcloud`, the units enabled, a service user. They give what the records give, and nothing more is promised for another layout.

| Step | Where | Needs | Source |
|------|-------|-------|--------|
| OS-1 | development machine | nothing | new |
| OS-2 | development machine | OS-1 | new |
| OS-3 | development machine | OS-2 | new |
| OS-4 | GitHub UI or development machine with gh | OS-3 | new |
| OS-5 | GitHub UI or development machine with gh | OS-3, OS-4 | deploy/NATIVE-AMD64-RUN.md O-1 |
| OS-6 | real server | nothing | deploy/BACKUP-TIMER-RUN.md O-1 |
| OS-7 | real server | nothing | deploy/BACKUP-TIMER-RUN.md O-2 |
| OS-8 | real server | nothing | deploy/BACKUP-TIMER-RUN.md O-3 |
| OS-9 | real server | OS-8 | deploy/BACKUP-TIMER-RUN.md O-4 |
| OS-10 | real server | nothing | deploy/BACKUP-TIMER-RUN.md O-5 |
| OS-11 | real server | nothing | deploy/BACKUP-CONSISTENCY-RUN.md item 1 |
| OS-12 | real server | nothing | deploy/BACKUP-CONSISTENCY-RUN.md item 2 |
| OS-13 | real server | nothing | deploy/NATIVE-AMD64-RUN.md O-2 |
| OS-14 | real server | nothing | deploy/NATIVE-AMD64-RUN.md O-3 |
| OS-15 | development machine | OS-3 | .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 0 |
| OS-16 | development machine | OS-3, OS-4, OS-15 | .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 2 |
| OS-17 | development machine | OS-16 | .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 3 |
| OS-18 | development machine | OS-17 | .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 4 |
| OS-19 | development machine | nothing | new |
| OS-20 | real server | nothing | deploy/BACKUP-ROBUST-RUN.md item 1 |
| OS-21 | real server | nothing | deploy/BACKUP-ROBUST-RUN.md item 2 |

### OS-1: Look at GitHub from here

Source: new
Where: development machine
Needs: nothing
Expect begins at: "The first prints nothing or `From` lines"
A different output is explained at: "A second number of 0 means"
Rehearsal: The first three commands were played in a throwaway pair of repositories made with `mktemp -d` (a bare repository standing in for GitHub, a clone four commits ahead of it, and four commits that exist only in the bare repository, one of them a merge); they printed a `From` line, the two numbers 4 and 4, and a list of four commits. The fourth command and the ruleset behind it were read from GitHub on 2026-10-26 with plain GET calls; nothing was written to GitHub, and `git ls-remote --heads origin` printed the same three heads (`main` ee3c9ed, `release` 4d238b9, `fix/bausteine-befunde` fa696f6) before and after.

```bash
git fetch origin
git rev-list --left-right --count main...origin/main
git log --oneline main..origin/main
gh api repos/holzcloud/holzcloud-cms/rules/branches/main --jq '.[].type'
```

The first prints nothing or `From` lines. The second prints two numbers on one line: the commits that are only here, then the commits that are only on GitHub. Read on 2026-10-26 it printed `71` and `3`; the first number grows with every commit made here, the second was 3. The third lists the commits that are only on GitHub; on 2026-10-26 they were `ee3c9ed` (the merge of pull request 47), `fa696f6` and `78efbce`. The fourth prints the types of the rules that apply to `main`, one per line; on 2026-10-26 they were `deletion`, `non_fast_forward`, `pull_request` and `required_status_checks`.

A second number of 0 means that there is nothing to bring in, and OS-2 is skipped. Any other commit than the three means that somebody pushed since, so read it before OS-2. No `pull_request` line means that the ruleset changed and OS-3 needs no bypass. An authentication error from `gh` means `gh auth login` first.

`main` carries a ruleset ("main: kein Force-Push, kein Loeschen, PR mit gruener Pruefung") whose only bypass is the Admin role, as read on 2026-10-26 with a plain GET call on the ruleset. It refuses a push that is not a fast-forward, it refuses deleting `main`, and it asks for a pull request with the green check `Build and test`; an Admin may bypass it. This is why OS-2 comes before OS-3.

This step does not answer a line of `docs/beta.md`; it tells what OS-2 and OS-3 start from.

### OS-2: Bring the commits that are only on GitHub into main

Source: new
Where: development machine
Needs: OS-1
Expect begins at: "The first prints one 40-character tree id"
A different output is explained at: "Exit 1 with file names from the first command means a conflict"
Rehearsal: All three commands were played in the throwaway pair of OS-1: the first printed one tree id and exit 0, the second printed `Merge made by the 'ort' strategy.` and the two files it brought in, the third listed those two files. A conflict was played too, with both sides changing one line of one file: the first command then printed three index lines, `CONFLICT (content): Merge conflict in app.txt` and exit 1. On the real repository only the first command was run on 2026-10-26 (it exits 0, so the three commits merge cleanly) and `git diff --stat` between the merge base and `origin/main`, which printed the two files named below; the merge itself was not made.

```bash
git merge-tree --write-tree main origin/main
git merge --no-edit origin/main
git diff --stat HEAD^1 HEAD
```

The first prints one 40-character tree id and exits 0; it changes nothing in the working copy. The second makes a merge commit and prints `Merge made by the 'ort' strategy.` and the files it brought in. The third lists exactly `cmd/holzcloud/assets/bausteine.css` and `cmd/holzcloud/templates/public/weide/style.css`, the two files of pull request 47; the same difference between the merge base and `origin/main` read `2 files changed, 22 insertions(+), 39 deletions(-)` on 2026-10-26.

Exit 1 with file names from the first command means a conflict: stop, and do not resolve it with a strategy option. Any file other than the two means that something besides pull request 47 came in since OS-1; read it first. Nothing is sent to GitHub by this step.

This is the owner's decision. The ruleset forbids a push that is not a fast-forward and forbids deleting the commits, so they have to come in; but bringing them in makes the shipped program differ from the evidence builds in those two files, which `docs/beta.md` states and the CHANGELOG entry names. Never overwrite pull request 47 on GitHub.

This step does not answer a line of `docs/beta.md`; it is the reason for the program-identity sentence of its evidence section.

### OS-3: Publish main

Source: new
Where: development machine
Needs: OS-2
Expect begins at: "The push output ends with a line that contains `main -> main`"
A different output is explained at: "A line that ends with `(non-fast-forward)` means"
Rehearsal: The push and the two reads were played on the throwaway pair of OS-1. A push before OS-1 was refused with `! [rejected]` and `main -> main (fetch first)`; after the fetch and before OS-2 the refusal ended with `(non-fast-forward)`; after OS-2 the push printed `e3a3990..32d1277  main -> main` and exit 0, and the two reads printed the same commit id. Not played, because they need GitHub: the `remote: Bypassed rule violations` lines, the `GH013` line and the pull request route; those are described as GitHub documents them and were not seen here.

```bash
git push origin main
git ls-remote origin refs/heads/main
git rev-parse main
```

The push output ends with a line that contains `main -> main`, in the form `OLD..NEW  main -> main`. GitHub may add lines that begin `remote: Bypassed rule violations`; they say that the Admin bypass of the ruleset was used, they are not an error, and they are documented by GitHub and were not seen here. The last two commands print the same commit id.

This comes first among the steps that touch GitHub: the records under `deploy/`, `docs/beta.md` and the workflow `native-amd64` exist on GitHub only after it, and the release body links to them.

A line that ends with `(non-fast-forward)` means that OS-1 and OS-2 were not done against the current GitHub: fetch again and repeat OS-2. A line with `GH013: Repository rule violations found` (documented, not seen) means that the signed-in account is not an Admin, so use a pull request instead. Push `main` to a branch with another name:

```bash
git push origin main:merge-main-0.1.1
```

Open the pull request from that branch into `main` on GitHub, wait for the check `Build and test`, and merge it with the option Create a merge commit (never Squash and never Rebase, which would rewrite the commits). Then take the merge back:

```bash
git fetch origin
git merge --ff-only origin/main
```

This step does not answer a line of `docs/beta.md`; every link that the page and the release body hold leads nowhere until it is done.

### OS-4: Read the result of CI on main

Source: new
Where: GitHub UI or development machine with gh
Needs: OS-3
Expect begins at: "The first line begins with `completed` and `success`"
A different output is explained at: "`in_progress` means wait"
Rehearsal: Read on 2026-10-26 with plain `gh` reads, before the push: `gh run list --workflow CI --branch main --limit 3` printed three lines that begin `completed` and `success` and took 13m37s, 13m31s and 13m38s, and `gh workflow list` printed seven workflows (Backfill the missed releases, CI, image, Release, Security Audit, tagfix, Dependabot Updates) and no `native-amd64`. What the commands print after the push was not seen, because the push is the owner's.

```bash
gh run list --workflow CI --branch main --limit 1
gh workflow list
```

The first line begins with `completed` and `success` and names the commit that was pushed. The three runs before took between 13m31s and 13m38s, so wait about that long after OS-3. `gh workflow list` now includes `native-amd64`, which it did not on 2026-10-26 before the push (GitHub lists a workflow with a `workflow_dispatch` trigger once its file is on the default branch; documented, not seen).

`in_progress` means wait (the exact word is documented by `gh`, not seen here). `completed` and `failure` means stop: read `gh run view RUN-ID --log-failed`, and take neither OS-5 nor OS-16. No `native-amd64` in the list means that the push did not reach the default branch.

This step does not answer a line of `docs/beta.md`; it tells whether OS-5 and the release may follow.

### OS-5: The button of the native amd64 run

Source: deploy/NATIVE-AMD64-RUN.md O-1
Where: GitHub UI or development machine with gh
Needs: OS-3, OS-4
Expect begins at: "Expect the job to end green."
A different output is explained at: "A different output has a name, and only `SRV-01 PASS` can be recorded."

<!-- verbatim: deploy/NATIVE-AMD64-RUN.md O-1 -->
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
<!-- end verbatim -->

The workflow exists on GitHub only after `main` has been pushed (OS-3), which is why this step needs OS-3 and OS-4. It answers SRV-01, the limit "Native amd64 timing was not measured" and the lines about seccomp on `docs/beta.md`. The result is recorded by the paragraph Recording a result of `deploy/NATIVE-AMD64-RUN.md`, in a commit of its own, and by nothing in this plan; until then `docs/beta.md` keeps the limit with the words Pending owner.

### OS-6: The shipped timer at its real 03:15

Source: deploy/BACKUP-TIMER-RUN.md O-1
Where: real server
Needs: nothing
Expect begins at: "Expect `TriggeredBy=holzcloud-backup.timer`"
A different output is explained at: "A start at another hour points at"
<!-- verbatim: deploy/BACKUP-TIMER-RUN.md O-1 -->
**O-1. The shipped timer at its real 03:15.** After the timer has been enabled for a night:

```bash
systemctl list-timers holzcloud-backup.timer
systemctl show holzcloud-backup.service -p TriggeredBy -p Result -p ExecMainStartTimestamp
journalctl -u holzcloud-backup.service --since yesterday -o short-precise | tail -n 20
timedatectl
```

Expect `TriggeredBy=holzcloud-backup.timer`, `Result=success`, an `ExecMainStartTimestamp` between 03:15:00 and about 03:21:00 local time, and a journal that ends with `=== Backup complete: /opt/holzcloud/backups/YYYYMMDD-HHMMSS ===`. A start at another hour points at the host's time zone (`timedatectl`), at a clock that is not synchronised (`System clock synchronized: no`), or at `AccuracySec` (the timer has the default of one minute). `TriggeredBy` empty means the last run was a manual one, not the timer's.
<!-- end verbatim -->

This step answers the limit about a real server's own zone on `docs/beta.md`; a result is not recorded in this repository by this phase.

### OS-7: The catch-up after a real power cycle

Source: deploy/BACKUP-TIMER-RUN.md O-2
Where: real server
Needs: nothing
Expect begins at: "Expect a start within about six minutes of boot"
A different output is explained at: "No run points at the stamp"

<!-- verbatim: deploy/BACKUP-TIMER-RUN.md O-2 -->
**O-2. The catch-up after a real power cycle.** Note the stamp, power the host off before 03:15, power it on after 03:21:

```bash
sudo stat -c '%y' /var/lib/systemd/timers/stamp-holzcloud-backup.timer
uptime -s
systemctl show holzcloud-backup.service -p TriggeredBy -p Result -p ExecMainStartTimestamp
journalctl -u holzcloud-backup.service -b -o short-precise
```

Expect a start within about six minutes of boot (the random delay of up to 300 seconds plus the accuracy; the service waits for `holzcloud.service`), `TriggeredBy=holzcloud-backup.timer`, `Result=success`. No run points at the stamp (it must be older than the missed instant; `systemctl show holzcloud-backup.timer -p Persistent` must say `yes`), at the zone, or at the timer not being enabled (`systemctl is-enabled holzcloud-backup.timer`).
<!-- end verbatim -->

This step answers the limit that a real power cycle and the boot ordering of the timer, the clock and the network were not exercised, because the container was stopped and started (Not tested in `deploy/BACKUP-TIMER-RUN.md`). O-3 of the native record (OS-14) can be taken in the same reboot.

### OS-8: The off-site copy to the real second host

Source: deploy/BACKUP-TIMER-RUN.md O-3
Where: real server
Needs: nothing
Expect begins at: "Expect `Result=success`, `ExecMainStatus=0`"
A different output is explained at: "Different outputs name their own cause"

<!-- verbatim: deploy/BACKUP-TIMER-RUN.md O-3 -->
**O-3. The off-site copy to your real second host, with your real key.** Follow the credentials paragraph in the Backups section of `deploy/DEPLOY.md` (the layout measured in F-2: key and `known_hosts` under `/opt/holzcloud/offsite-ssh`, a `Host` block in `/etc/ssh/ssh_config.d/holzcloud-backup.conf`). Compare the host key fingerprint by hand, then:

```bash
sudo -u holzcloud ssh-keygen -lf /opt/holzcloud/offsite-ssh/known_hosts
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub    # run this one ON the remote host and compare
sudo systemctl edit --full holzcloud-backup.service  # uncomment and set Environment=REMOTE_TARGET=user@host:/path
sudo systemctl start holzcloud-backup.service
systemctl show holzcloud-backup.service -p Result -p ExecMainStatus
ssh user@host ls -1 /path
```

Expect `Result=success`, `ExecMainStatus=0`, and the newest backup directory listed on the remote host with the same name. Different outputs name their own cause: `Permission denied (publickey)` is a public key that is not in the remote account's `authorized_keys`, or an `IdentityFile` that points elsewhere; `Host key verification failed` is a `known_hosts` that is missing, unreadable to the service (a file under `/home` is hidden by `ProtectHome=true`) or does not match; `rsync: Failed to exec ssh` or `ssh: command not found` is a server without an ssh client (`openssh-client`); `ExecMainStatus=255` in general is an ssh failure, and the new backup is on the box either way (F-6).
<!-- end verbatim -->

This step answers the limit about a real network between two real machines, the owner's credentials and a host key verified by fingerprint (Not tested in `deploy/BACKUP-TIMER-RUN.md`); the layout it asks for is the one `deploy/DEPLOY.md` describes in its credentials paragraph.

### OS-9: The copy back and the restore on a rebuilt box

Source: deploy/BACKUP-TIMER-RUN.md O-4
Where: real server
Needs: OS-8
Expect begins at: "Expect the transcript to end with"
A different output is explained at: "A refusal before anything is stopped"

<!-- verbatim: deploy/BACKUP-TIMER-RUN.md O-4 -->
**O-4. The copy back from the second host and the restore on a rebuilt box.** On a server rebuilt by the Install and systemd Service sections, with no account created in the setup form:

```bash
sudo rsync -a user@host:/path/ /opt/holzcloud/backups/
sudo ls -1 /opt/holzcloud/backups
sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/YYYYMMDD-HHMMSS
```

Answer `yes` at the question. Expect the transcript to end with `=== Restore complete ===`, `/admin/setup` to answer 404, and a login with the old password to work; `ls` must show the directories with mode `drwx------` (`rsync -a` keeps it). A refusal before anything is stopped (`MANIFEST` missing, snapshot failing its check) names the directory; a directory without a `MANIFEST` is an interrupted copy, and a different login means the wrong backup directory was chosen.
<!-- end verbatim -->

This step answers the limit about the pull of the copy back to a rebuilt server (Not tested in `deploy/BACKUP-TIMER-RUN.md`). It replaces the data of the server it is run on, so it is taken on a server that was rebuilt for it.

### OS-10: Retention on a real clock

Source: deploy/BACKUP-TIMER-RUN.md O-5
Where: real server
Needs: nothing
Expect begins at: "Expect no directory from the `find`"
A different output is explained at: "A surviving old directory points at"

<!-- verbatim: deploy/BACKUP-TIMER-RUN.md O-5 -->
**O-5. Retention on a real clock.** After more than 31 days of nightly runs:

```bash
ls -1 /opt/holzcloud/backups | wc -l
find /opt/holzcloud/backups -mindepth 1 -maxdepth 1 -type d -mtime +31
systemctl show holzcloud-backup.service -p Environment
```

Expect no directory from the `find` (nothing older than 31 days) and about 31 directories in the first count. A surviving old directory points at `RETENTION_DAYS` (check the third line; `0` switches pruning off), at the sandbox (`ReadWritePaths` must name `/opt/holzcloud/backups`), or at the script discarding the error of the removal (F-8: `2>/dev/null || true` hides a failure from the journal). A directory 30 days and a few hours old is expected to be there; it goes with the next run (F-3).
<!-- end verbatim -->

This step answers the limit about retention on elapsed real time rather than planted modification times (Not tested in `deploy/BACKUP-TIMER-RUN.md`).

### OS-11: The data against the free space

Source: deploy/BACKUP-CONSISTENCY-RUN.md item 1
Where: real server
Needs: nothing
Expect begins at: "The first number is an upper bound of what one backup writes"
A different output is explained at: "If the second is not clearly larger than the first"

```bash
sudo du -sm /opt/holzcloud/data
df -Pm /opt/holzcloud/backups | awk 'NR==2 {print $4}'
```

<!-- verbatim: deploy/BACKUP-CONSISTENCY-RUN.md item 1 -->
1. Compare the data a backup has to write with the space the guard looks at. Run `sudo du -sm /opt/holzcloud/data` and `df -Pm /opt/holzcloud/backups | awk 'NR==2 {print $4}'`. The first number is an upper bound of what one backup writes (it includes the pre-upgrade snapshots, which a backup does not carry), the second is what `backup.sh` reads as free space before it starts. If the second is not clearly larger than the first, a backup can pass the guard and still run out of space halfway (CON-03, F-4): set `MIN_FREE_MB` in the unit above the first number, as `deploy/DEPLOY.md` says, or make room.
<!-- end verbatim -->

This step answers the limit about a real disk that is nearly full but above `MIN_FREE_MB`, which was not provoked, and it carries the instruction to set `MIN_FREE_MB` above the first number.

### OS-12: A directory left by a failed night

Source: deploy/BACKUP-CONSISTENCY-RUN.md item 2
Where: real server
Needs: nothing
Expect begins at: "No output means every directory is a finished backup"
A different output is explained at: "A line means that night was interrupted"

```bash
for d in /opt/holzcloud/backups/*/; do [ -f "$d/MANIFEST" ] || echo "no MANIFEST: $d"; done
```

<!-- verbatim: deploy/BACKUP-CONSISTENCY-RUN.md item 2 -->
2. Look for a directory without a `MANIFEST`, the trace of a night that failed: `for d in /opt/holzcloud/backups/*/; do [ -f "$d/MANIFEST" ] || echo "no MANIFEST: $d"; done`. No output means every directory is a finished backup. A line means that night was interrupted (a full disk, F-4, or a file that vanished during the copy, F-3): `restore.sh` will refuse that directory, and if the filesystem is nearly full it is the directory that makes the next night's guard refuse (F-5). Check `systemctl status holzcloud-backup.service` for the failed unit, free the space or remove that one directory, and run `sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups` once by hand.
<!-- end verbatim -->

This step answers the limit about a failed night that leaves a directory which makes the following nights refuse; the sentence about it is on `docs/beta.md`, not in `deploy/DEPLOY.md`.

### OS-13: A restore on the real server, to a different path

Source: deploy/NATIVE-AMD64-RUN.md O-2
Where: real server
Needs: nothing
Expect begins at: "Expect, block by block:"
A different output is explained at: "A different output means:"

<!-- verbatim: deploy/NATIVE-AMD64-RUN.md O-2 -->
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
<!-- end verbatim -->

This step answers SRV-02 and the limit "A restore on a real server was not made". The result is recorded by the paragraph Recording a result of `deploy/NATIVE-AMD64-RUN.md`, in a commit of its own, and by nothing in this plan; until then `docs/beta.md` keeps the limit with the words Pending owner.

### OS-14: A reboot with the units enabled

Source: deploy/NATIVE-AMD64-RUN.md O-3
Where: real server
Needs: nothing
Expect begins at: "Expect `enabled` twice"
A different output is explained at: "A different output means:"

<!-- verbatim: deploy/NATIVE-AMD64-RUN.md O-3 -->
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
<!-- end verbatim -->

This step answers SRV-03 and the limit "A reboot with the units enabled was not made". The result is recorded by the paragraph Recording a result of `deploy/NATIVE-AMD64-RUN.md`, in a commit of its own, and by nothing in this plan; until then `docs/beta.md` keeps the limit with the words Pending owner. It can be taken in the same reboot as OS-7.

### OS-15: The date of the heading

Source: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 0 (adapted)
Where: development machine
Needs: OS-3
Expect begins at: "If the release is made on another day, edit that one line and commit it."
A different output is explained at: "The workflow ignores the date"
Rehearsal: The heading command was run on `CHANGELOG.md` of this repository on 2026-10-26 and printed the line named below; the edit and the commit it may lead to were not played.

```bash
grep -m1 '^## ' CHANGELOG.md
date +%Y-%m-%d
```

The first prints the heading of the entry, now `## 0.1.1 — 2026-10-26`; the second prints today's date. The number 0.1.1 and the date are drafts that the milestone completion step confirms. What to do when they differ on the day of the release is in the text below, taken from the earlier release.

<!-- verbatim: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 0 -->
The heading `## 0.1.1 — DATE` of `CHANGELOG.md` carries the day the entry was written. If the release is made on another day, edit that one line and commit it. The workflow ignores the date; the admin screen and the embedded changelog show it.

Read `docs/beta.md` once. It is the contract the release page links to.
<!-- end verbatim -->

This step answers no line of `docs/beta.md`; it settles the one line of the entry that the page's version number is read from.

### OS-16: Make the release

Source: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 2 (adapted)
Where: development machine
Needs: OS-3, OS-4, OS-15
Expect begins at: "The push starts the Release workflow"
A different output is explained at: "If the tests fail, nothing is tagged and nothing is published."

This is taken only after the milestone has been completed and the number and the date have been confirmed. The other steps of this file whose result is still pending do not hold it back, because the page ships them as limits.

<!-- verbatim: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 2 -->
```bash
git push origin main:release
```

This is route 2 of `.github/workflows/release.yml`. The `release` branch holds no work of its own and is only ever fast-forwarded. The push starts the Release workflow, which reads the first version heading of `CHANGELOG.md` (0.1.1), runs the whole test suite, builds linux/amd64, mints the tag `v0.1.1` through the Releases API after the tests have passed, and publishes the release as a pre-release, because every `v0.*` tag is one.

The alternative for a person: GitHub, Actions, Release, Run workflow, with no inputs (route 3), after OS-3. With no inputs it does the same thing at the head of the default branch.

The release body is the `CHANGELOG.md` section for 0.1.1 followed by GitHub's generated notes. There is no separate notes file. If the tests fail, nothing is tagged and nothing is published.
<!-- end verbatim -->

This step answers no line of `docs/beta.md`; the page and the release body are what it publishes.

### OS-17: Check the release

Source: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 3 (adapted)
Where: development machine
Needs: OS-16
Expect begins at: "The first prints one line."
A different output is explained at: "no line from `git ls-remote --tags` means that no tag exists yet"
Rehearsal: The explanation after the text below was read from `.github/workflows/release.yml` and was not played. On 2026-10-26 `git ls-remote --tags origin` listed `v0.1.0` and no `v0.1.1`, and the last three runs of the Release workflow had taken 14m12s, 14m7s and 13m50s.

<!-- verbatim: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 3 -->
```bash
git ls-remote --tags origin v0.1.1
gh release view v0.1.1 --json isPrerelease,body
```

The first prints one line. In the second, `isPrerelease` is `true` and the body opens with the text of the CHANGELOG entry.

The job `holzcloud-ch` writes the version into the holzcloud.ch pages only when the `CMS_TOKEN` secret exists. Without it the job prints a notice and succeeds without doing so.
<!-- end verbatim -->

Wait about 14 minutes after OS-16 before this. A different output, as the workflow gives it: no line from `git ls-remote --tags` means that no tag exists yet, so the Release run has not finished or its tests failed (`gh run list --workflow Release --limit 3` shows which); if the tests failed, nothing was published. `isPrerelease` of `false` means stop and read `.github/workflows/release.yml`, because every `v0.*` tag is a pre-release there.

This step answers no line of `docs/beta.md`; it checks that the release exists as the page says it does.

### OS-18: If something is wrong afterwards

Source: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 4 (adapted)
Where: development machine
Needs: OS-17
Expect begins at: "moves no tag"
A different output is explained at: "A published tag is never moved."
Rehearsal: Read from `.github/workflows/release.yml` and the earlier release; not played, because it needs a release that exists.

```bash
git push origin main:release
```

This is used only when the text of the entry must be corrected after the release exists: commit the corrected entry on `main` first, then run the push. The record's own words follow.

<!-- verbatim: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 4 -->
A push to `release` at a version that is already out moves no tag and rewrites only the release text from `CHANGELOG.md`. That is how a corrected entry reaches the page. A published tag is never moved.
<!-- end verbatim -->

This step answers no line of `docs/beta.md`.

### OS-19: Keep the bundle of phase 27

Source: new
Where: development machine
Needs: nothing
Expect begins at: "The size is 26349797 bytes"
A different output is explained at: "Another size or another checksum means"
Rehearsal: The three read-only commands were run on the real file on 2026-10-26 and printed what is written below. The command that brings the lineage back was played in a throwaway repository in phase 27 (E-5 of its owner steps); it was not run again here.

The bundle is `/Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle`, the only copy of the old lineage now that its branch is gone. Run the commands in any clone of the project; `git bundle verify` needs a repository.

```bash
ls -l /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle
shasum -a 256 /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle
git bundle verify /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle
```

The size is 26349797 bytes. The checksum is `bdcd2fd9ff164b1d1cd59e6ed68a36f7509e5dba00155ea07ead512cfa5b2924`. `git bundle verify` prints five lines: the first ends with `is okay`, then `The bundle contains this ref:`, `ef4873ad07e30ac27b02e5829ff0f1a20307ca9f refs/heads/backup/lokal-vor-origin-abgleich`, `The bundle records a complete history.` and `The bundle uses this hash algorithm: sha1`.

Another size or another checksum means that the file changed or was replaced: delete nothing, and compare it with a copy. A `verify` that fails means that the bundle is damaged.

To bring the lineage back, fetch it from the bundle into a branch of the same name (the command of E-5 of the phase 27 owner steps, with the real path):

```bash
git fetch /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle refs/heads/backup/lokal-vor-origin-abgleich:refs/heads/backup/lokal-vor-origin-abgleich
```

The old lineage is never merged into `main`: single features are ported, not the branch.

This step answers no line of `docs/beta.md`; it keeps what the repository no longer holds.

### OS-20: The first night of the changed backup.sh

Source: deploy/BACKUP-ROBUST-RUN.md item 1
Where: real server
Needs: nothing
Expect begins at: "expect the header line"
A different output is explained at: "means a file vanished and the copy was repeated"

```bash
sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups; echo "exit $?"
for d in /opt/holzcloud/backups/*/; do [ -f "$d/MANIFEST" ] || echo "no MANIFEST: $d"; done
```

<!-- verbatim: deploy/BACKUP-ROBUST-RUN.md item 1 -->
1. The first night after the update, by hand: run `sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups; echo "exit $?"` and then `for d in /opt/holzcloud/backups/*/; do [ -f "$d/MANIFEST" ] || echo "no MANIFEST: $d"; done`; expect the header line `=== holzcloud-CMS backup: TIMESTAMP ===`, the lines `Backing up ...`, the line `Pruning backups older than 30 days...`, the line `=== Backup complete: ... ===`, then `exit 0`, and no `no MANIFEST:` line for the new directory; a line `WARNING: rsync exited 24 (files vanished) while copying media; repeating it once` means a file vanished and the copy was repeated, which is the script working (the night still ends `exit 0` if the repeat succeeded), a line `ERROR: rsync exited 24 again while copying media; giving up` means it recurred and the night failed with `exit 24` and no directory, and a `no MANIFEST:` line names a directory of an earlier night of the old script, which `restore.sh` refuses and the new script does not remove, so remove that one directory by hand.
<!-- end verbatim -->

This step answers the limit of `docs/beta.md` that the changed `backup.sh` was played in a container and not on a real server: it is the first night of the script that this release ships, taken by hand. OS-12 still serves for a directory without a `MANIFEST` that a night of the old script left behind.

### OS-21: The morning after the first timer night

Source: deploy/BACKUP-ROBUST-RUN.md item 2
Where: real server
Needs: nothing
Expect begins at: "expect no output on a quiet night"
A different output is explained at: "means the night failed and removed its own directory"

```bash
journalctl -u holzcloud-backup.service --since yesterday --no-pager | grep -E 'rsync exited 24|backup did not finish'
systemctl status holzcloud-backup.service
```

<!-- verbatim: deploy/BACKUP-ROBUST-RUN.md item 2 -->
2. The morning after the first timer night: run `journalctl -u holzcloud-backup.service --since yesterday --no-pager | grep -E 'rsync exited 24|backup did not finish'` and expect no output on a quiet night; a line `WARNING: rsync exited 24 ... repeating it once` without a line `ERROR:` after it means a file vanished and the repeat completed, a line `ERROR: rsync exited 24 again ... giving up` or `ERROR: backup did not finish; removed ...` means the night failed and removed its own directory, so read `systemctl status holzcloud-backup.service`, look at the lines above it in the journal for the cause (a full disk shows as `No space left on device`) and run `sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups` once by hand in a quiet moment.
<!-- end verbatim -->

This step answers the limit of `docs/beta.md` that a status 24 and a failed night were provoked in a container (with a stand-in for `rsync` in the first case) and not seen under the real timer of a real server. OS-12 still serves for a directory without a `MANIFEST` that a night of the old script left behind.

## Veto points

These are the decisions the plans of this phase made for the owner. Each can be overruled; the cost is named.

**The merge of pull request 47 (OS-2).** `main` here is ahead of GitHub and also behind it, so the commits of pull request 47 have to come in before anything can be published. The cost: the shipped program then differs from the programs that the evidence builds measured, in `cmd/holzcloud/assets/bausteine.css` and in the `style.css` of the weide theme. `docs/beta.md` and the CHANGELOG entry say so in words; overruling this means publishing from a branch without those commits, which GitHub's ruleset does not allow as a push to `main`.

**The date of the heading (OS-15).** `2026-10-26` is the day the entry was written, as in 0.1.0. The milestone completion step confirms or changes it, and the workflow ignores it.

**The reworded native amd64 line.** The 0.1.0 limit "none is planned (a user decision)" is no longer true since the button exists; `docs/beta.md` says that no run was made yet and that one is planned. To keep the old wording instead, keep the baseline bullet and add a `Why:` that corrects it; the checker accepts both.

**`deploy/DEPLOY.md` is not edited.** Three gaps are written in `docs/beta.md` and in these steps instead: a vanished file ends the night, a failed night's directory blocks the following nights, and a restore does not report a row whose file is missing. An edit of `deploy/DEPLOY.md` would stale the evidence bound to it and force the runs to be made again; whether to pay that is the owner's.

**Two small product features ride in this release.** The block Bild und Text has a position for the picture, and an upload of a large raster asks whether to shrink it. They predate the evidence builds and were already in the program the records measured, but new CMS functions are out of scope for this milestone; the entry states them as what they are.
