# Phase 28: the owner's steps

Claude ran none of the commands in this file. A step marked as taken from a record carries that record's text unchanged, between the two comment lines of its verbatim region.

| Step | Where | Needs | Source |
|------|-------|-------|--------|
| OS-6 | real server | nothing | deploy/BACKUP-TIMER-RUN.md O-1 |

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
