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
| DEVIATION[network] | Printed only when a step fails because no package repository or download host can be reached. The run then exits 3 ("ran, but this platform cannot give the evidence") and not 1. It did not occur in either pass. | The server has a network |

## Findings

One row per defect that pass 1 showed. The text in backticks in the fourth column is the decisive output, quoted
verbatim from the scratch files of pass 1 (`deploy/test/out/amd64-install-pass1`, `amd64-install-pass1b` and their
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

### Changed without a failing line in pass 1

Two things in the repair did not fail in pass 1 and are recorded as what they are, reasoned from the files and
confirmed only by pass 2:

- `/opt/holzcloud/backups` is now owned by the service user and has mode 700. The backup unit runs as `holzcloud`
  with `ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups`, and pass 1 never got as far as a backup. In pass 2 the
  manual backup run as `holzcloud` into that directory succeeds.
- `ca-certificates` and `openssl` are named in Prerequisites. The image has both, so nothing failed; the first is what
  `curl` needs for the HTTPS downloads of the Caddy block, and the second is used by the single sign-on section. The
  document names every tool the image ships and a step uses, so no step works here by luck.

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
