# Hardened unit under real systemd: measured results

This document is the permanent record of phase 20. It records what was measured when
`deploy/holzcloud.service` was run under real systemd (a systemd Debian container), what that
measurement cannot say about a real server, and the raw `systemd-analyze security` output of the
unit as shipped. Nothing here is tuned: the unit was not changed to improve a number.

## Summary

| Requirement | Result | Platform | systemd | Date, commit |
|-------------|--------|----------|---------|--------------|
| UNIT-01: the unit starts and the site answers | PASS. The unchanged unit started and `/healthz` and `/readyz` answered. On this emulated platform the first start crashed and needed a container-only drop-in (see UNIT-01). | linux/amd64 (emulated on an arm64 host) | 252 (Debian 12) | 2026-10-12, f1428d2 |
| UNIT-02: `MemoryDenyWriteExecute=true` against wazero | SETTLED, and the unit keeps `MemoryDenyWriteExecute=true`. All five plugins load and run on wazero's interpreter fallback with the directive enforced. The evidence comes from native arm64 only. On amd64 the verdict is VOID, because the emulation installs no seccomp filter. | linux/arm64 native (evidence), linux/amd64 (functional evidence only) | 252 (Debian 12) | 2026-10-12, f1428d2 |
| UNIT-03: `systemd-analyze security` | Overall exposure level 1.3 online and 1.4 offline on systemd 252; 1.3 online and 1.3 offline on systemd 257. Measured, not tuned. | linux/amd64 container | 252 (Debian 12) and 257 (Debian 13) | 2026-10-12, f1428d2 |

No target value is imposed on the score: it is measured, not tuned (D-03a). The values above are what
the unit as shipped produced, and the full tables are in the appendix.

The commit is the repository state the runs were made from. `deploy/holzcloud.service` has not
changed since the comment-only commit `dd38a10`; the runs used that file.

## How this was measured

Three runs, all through `deploy/test/run.sh` (the recipe is described in `deploy/test/README.md`):

- R1: `PLATFORM=linux/amd64 OUT_DIR=deploy/test/out/amd64 ./deploy/test/run.sh all` (UNIT-01, UNIT-03, the plugin evidence).
- R2: `PLATFORM=linux/arm64 OUT_DIR=deploy/test/out/arm64 ./deploy/test/run.sh mdwe` (the one narrow native run, MDWE evidence only).
- R3: `PLATFORM=linux/amd64 BASE_IMAGE=debian:trixie-slim OUT_DIR=deploy/test/out/amd64-trixie ./deploy/test/run.sh build up score` (the same unit analysed by systemd 257).

The harness writes scratch files under `deploy/test/out/`, which is gitignored. The raw blocks in the
appendix are those files, pasted unchanged.

| | R1, amd64 | R2, arm64 | R3, amd64 trixie |
|---|---|---|---|
| Host architecture | aarch64 | aarch64 | aarch64 |
| Host | Docker Desktop VM | Docker Desktop VM | Docker Desktop VM |
| Host kernel | 7.0.14-linuxkit | 7.0.14-linuxkit | 7.0.14-linuxkit |
| Container uname | Linux 7.0.14-linuxkit x86_64 | Linux 7.0.14-linuxkit aarch64 | Linux 7.0.14-linuxkit x86_64 |
| Container systemd | 252 (252.39-1~deb12u2) | 252 (252.39-1~deb12u2) | 257 (257.13-1~deb13u1) |
| Container OS | Debian 12 (bookworm) | Debian 12 (bookworm) | Debian 13 (trixie) |

Both containers run as privileged containers with systemd as PID 1 and the unit copied in unchanged
and not enabled. The harness starts the service itself.

## Deviation from D-01

D-01 said: test on linux/amd64 through emulation only, with no arm64 counter-run. On 2026-10-12 the
user decided to refine this after the research. One narrow native arm64 run exists, and only as
the evidence for `MemoryDenyWriteExecute` against wazero. UNIT-01 and UNIT-03 stay measured on
linux/amd64.

The reason is what R1's canary printed. The research predicted that the amd64 emulation on this host
installs no seccomp filter; the run confirms it, it was not copied from the research:

- The service's main process shows `Seccomp: 0` and `Seccomp_filters: 0` on amd64 (R1, `unit-start.txt` and `check.txt`).
- The canary runs a probe under the unit's own sandbox properties with `MemoryDenyWriteExecute=yes`. On amd64
  `mprotect RW->RX` and `mmap RWX` are both `ALLOWED` under `yes`, and the `socket AF_NETLINK` probe is `ALLOWED` although the
  address-family list does not contain it. The probe shows the same answers with `MemoryDenyWriteExecute=no`.
- On native arm64 the same probe under `yes` answers `denied (operation not permitted)` for both memory operations and
  `denied (address family not supported by protocol)` for `AF_NETLINK`; with `no` both memory operations are `ALLOWED`. The
  main process has `Seccomp: 2` with 24 filters.

A plugin run on amd64 therefore proves nothing about `MemoryDenyWriteExecute`: no filter stands in the way, so wazero's
compiler works and the result is the opposite of what an enforcing system shows. The harness
reports such a run as VOID and exits with 3.

No native amd64 run was made and none is planned (user decision). Whatever is said about amd64 below was
measured under the emulation layer of this host, or is marked inferred or assumed.

## UNIT-01: the unit starts and the site answers

On linux/amd64 the unchanged unit crashed at its first start. R1 saved the evidence as
`unit-start-unchanged.txt` (the full file is not pasted here; these are the decisive lines):

```text
holzcloud[115]: assertion failed [!result.is_error]: Unable to open /proc/sys/vm/mmap_min_addr
holzcloud[115]: (VMAllocationTracker.cpp:318 init)
systemd[1]: holzcloud.service: Main process exited, code=killed, status=5/TRAP
systemd[1]: holzcloud.service: Failed with result 'signal'.
```

The emulation layer opens `/proc/sys/vm/mmap_min_addr` when a process starts. The unit has `ProcSubset=pid`, which hides
every `/proc` entry that is not a process directory, so the emulation aborts the process with a trap. This
is an artefact of the emulation, not of the program: the same binary and the same unit started without any change on native arm64.

The harness answers the known signature, and only that signature, with a container-only drop-in
`/etc/systemd/system/holzcloud.service.d/container.conf` that sets `ProcSubset=all`. The shipped unit stays untouched. With the drop-in
the service started (`Active: active (running)`, `NRestarts=0`, `Result=success`).
`score` runs before `start`, so the score measurements are of the unit without the drop-in (`score-unit.txt`, in the appendix, lists no drop-in).

On arm64 the unchanged unit started with no drop-in: R2 wrote no `unit-start-unchanged.txt`, `unit-start.txt` shows
`Active: active (running)` with no `Drop-In:` line, and all seventeen assertions of `check` passed on it.

R1 answered, from inside the container:

```text
== /healthz ==
{"status":"ok"}
== /readyz ==
{"status":"ok","version":"dev","commit":"unknown","uptime_s":0,"integrity":"ok","disk_free_bytes":402885558272,"database":{"freelist_count":0,"page_count":155,"size_bytes":4096,"wal_bytes":2241312}}
```

`/healthz` returned exactly `{"status":"ok"}`, and `/readyz` returned status `ok` and integrity `ok`. UNIT-01 is
measured on linux/amd64 as the plan and D-01 require.

## UNIT-02: MemoryDenyWriteExecute and wazero

The question: the plugin runtime wazero compiles WebAssembly to machine code and needs executable memory, and
`MemoryDenyWriteExecute=true` forbids it. Does the hardened unit break the plugins?

**What each platform enforces** (the canary, the full tables are in the appendix):

| Probe under the unit's sandbox properties | amd64, `MemoryDenyWriteExecute=yes` | arm64, `MemoryDenyWriteExecute=yes` | arm64, control `no` |
|---|---|---|---|
| `mprotect RW->RX` | ALLOWED | denied (operation not permitted) | ALLOWED |
| `mmap RWX` | ALLOWED | denied (operation not permitted) | ALLOWED |
| `socket AF_NETLINK` | ALLOWED | denied (address family not supported by protocol) | denied (address family not supported by protocol) |
| `stat /dev/kmsg` | denied (no such file or directory) | denied (no such file or directory) | denied (no such file or directory) |
| `write /etc` | denied (read-only file system) | denied (read-only file system) | denied (read-only file system) |
| Seccomp of the probe | 0 (0 filters) | 2 (12 filters) | 2 (10 filters) |

On amd64 the canary ran with `ProcSubset=all` because of the container-only drop-in, so its `stat /proc/sys` row answers
`ALLOWED` by construction and is not an enforcement result.

**The five plugins**, on both platforms, from `check.txt`, 17 assertions each, no `BAD` line:

| Plugin | loaded (amd64, arm64) | running (amd64, arm64) | invoke (amd64, arm64) |
|---|---|---|---|
| bestellung | PASS, PASS | PASS, PASS | PASS, PASS |
| jahreszahl | PASS, PASS | PASS, PASS | PASS, PASS |
| kontaktformular | PASS, PASS | PASS, PASS | PASS, PASS |
| nicht-gefunden | PASS, PASS | PASS, PASS | PASS, PASS |
| suche | PASS, PASS | PASS, PASS | PASS, PASS |

`PASS stable` appears on both platforms: `NRestarts=0`, `Result=success`, and no kill signature in the journal.

**Which engine ran** (the `engine:` line of `check.txt` and `mdwe-control.txt`, counted from `/proc/PID/maps`):

| Platform | Shipped unit (`MemoryDenyWriteExecute=true`) | Control (`MemoryDenyWriteExecute=no`) |
|---|---|---|
| linux/arm64 native | interpreter (0 anonymous executable mappings) | compiler (8 anonymous executable mappings) |
| linux/amd64 emulated | compiler (9 anonymous executable mappings) | compiler (10 anonymous executable mappings) |

**Latency**, median of ten requests in seconds, with the hook budget `CallTimeout` of 2 seconds
(`internal/plugin/runtime.go`):

| Route | arm64, interpreter (shipped) | arm64, compiler (control) | amd64, shipped | amd64, control |
|---|---|---|---|---|
| `/plugintest` | 0.011671 | 0.002179 | 0.009147 | 0.009656 |
| `/suche?q=Plugintest` | 0.007672 | 0.002084 | 0.008848 | 0.008233 |
| `/formular` | 0.000907 | 0.000260 | 0.005883 | 0.005748 |
| `/bestellung` | 0.000804 | 0.000209 | 0.005986 | 0.005695 |

These numbers are not benchmarks; they show the order of magnitude. On arm64 the interpreter is
slower than the compiler by a factor of about three to five, and all medians stay far inside the 2 second budget. On amd64 the emulation
overhead dominates and the two engines cannot be told apart by latency.

**The verdicts, exactly as `verdict.txt` printed them:**

```text
MDWE VERDICT: VOID - seccomp is not enforced on this platform (linux/amd64), so the plugin results prove nothing about MemoryDenyWriteExecute; 17/17 assertions passed, engine under the shipped unit: compiler
```

```text
MDWE VERDICT: SETTLED - MemoryDenyWriteExecute=true is enforced here (canary: enforced), all five plugins loaded and ran on wazero's interpreter fallback (17/17 assertions), the control with the directive off ran on the compiler engine (17/17), and the unit stays unchanged
```

In words. On amd64 the verdict is VOID: the platform enforces nothing, the plugins ran on the compiler under the shipped
unit, and so that run is not evidence about `MemoryDenyWriteExecute`. It is only evidence that the unit starts, the
plugins load and the site answers. A green plugin run there must never be quoted as the MDWE answer. On arm64 all five plugins load and
run on wazero's interpreter fallback with the directive enforced, and the control with the directive off shows the compiler.
So wazero probes at start whether it may map executable memory, falls back silently when it may not, and the plugins work either way.

**Decision.** The unit keeps `MemoryDenyWriteExecute=true`: the evidence did not force a change (D-03). The unit gained comment
lines only, above the directive, that explain the probe, the silent fallback and the cost. The tripwire is named there as well: if the
runtime configuration in `internal/plugin/runtime.go` is ever pinned to the compiler only (`NewRuntimeConfigCompiler`), the plugins
fail to load under this line, and the directive has to become `MemoryDenyWriteExecute=false` at that moment.

**The one assumption that remains:** that wazero's probe and the MDWE filter behave on native amd64 as they do on native arm64. The probe is
architecture independent in the wazero source and the seccomp filter that systemd installs for the directive covers x86-64 (the systemd.exec
documentation calls the directive fully available on x86-64), but this was not measured here. Status: assumed.

## UNIT-03: systemd-analyze security

`systemd-analyze security` was run for the unit as shipped, with no drop-in present (see `score-unit.txt`), without pager and
without terminal.

| Run | systemd | Mode | Overall exposure level |
|---|---|---|---|
| amd64, bookworm | 252 | online (the manager's loaded unit) | 1.3 OK |
| amd64, bookworm | 252 | offline (the file alone) | 1.4 OK |
| amd64, trixie | 257 | online | 1.3 OK |
| amd64, trixie | 257 | offline | 1.3 OK |

Findings still marked as exposed (the rows with a cross), with the tool's weight for each, in all four runs except where noted:

- `RemoveIPC=` 0.1
- `RootDirectory=/RootImage=` 0.1
- `RestrictAddressFamilies=~AF_UNIX` 0.1 (the unit allows local sockets)
- `RestrictAddressFamilies=~AF_(INET|INET6)` 0.3 (the unit allows Internet sockets, it needs them)
- `PrivateNetwork=` 0.5 (the service must reach the network)
- `PrivateUsers=` 0.2
- `DeviceAllow=` 0.1 (the tool reports a device ACL with `char-rtc:r`)
- `IPAddressDeny=` 0.2
- `PrivateMounts=` 0.2, in the offline analysis on systemd 252 only

Online and offline differ on systemd 252. The offline analysis reads the unit file alone and lists `PrivateMounts=` as
exposed; the online analysis asks the manager, which has already applied its mount-namespace settings, and the row passes. On systemd 257 the two tables are line
for line identical, and the online tables of 252 and 257 are identical as well. The full tables, every row including the passing ones, are in the appendix.

## Container vs. server

Every difference between this test environment and a real server that this phase found or expects is listed here.
Status is one of: **measured** (the harness observed it), **inferred** (concluded from something measured), **assumed** (neither observed nor concluded).

| Aspect | Measured here | Real server | Status |
|---|---|---|---|
| Architecture and emulation layer | amd64 binary under an emulation layer on an aarch64 host (Docker Desktop); the crash text names `VMAllocationTracker.cpp`, which points to Rosetta | native amd64 | measured (the arch and the crash); inferred (that the layer is Rosetta) |
| Kernel | 7.0.14-linuxkit, the kernel of the Docker Desktop VM (`versions.txt`) | the distribution's kernel (for example Debian 12 or Ubuntu 24.04) | measured |
| Seccomp-based directives (see the list below) | amd64: no filter at all, `Seccomp: 0`; arm64: filter installed, `Seccomp: 2` | filter installed | measured on both platforms |
| Mount-namespace directives (`ProtectSystem`, `PrivateDevices`, `PrivateTmp`, `ProcSubset`) | enforced on amd64 too: the canary's probes of `/dev/kmsg` (no such file or directory) and `write /etc` (read-only file system) were denied; `ProcSubset=pid` was enforced on arm64 (`stat /proc/sys` denied) | enforced | measured for `ProtectSystem` and `PrivateDevices` on both platforms, `ProcSubset` on arm64; inferred for `PrivateTmp` (helpers placed in `/usr/local/bin` because `/tmp` was not shared with the transient unit) |
| `ProcSubset=pid` with emulation | the emulation crashes at start with status 5/TRAP; the container-only drop-in sets `ProcSubset=all` | no emulation, `ProcSubset=pid` works (it did on native arm64) | measured |
| systemd versions | 252 (Debian 12) and 257 (Debian 13) | whatever the server runs; Ubuntu 24.04 has 255 | measured for 252 and 257; assumed for others |
| Privileged container and cgroups | `--privileged`, private cgroup namespace; cgroup v2 on Docker Desktop (not otherwise recorded) | systemd on the host, its own cgroup hierarchy, no privileged container | measured (that it worked); assumed (that there is no effect on the directives) |
| Audit log | no auditd in the container, so no kernel audit record of a seccomp kill; the harness substitutes `NRestarts`, `Result` and a journal scan for `code=killed`, `status=31` and `SIGSYS` | auditd or the kernel log records a seccomp kill | inferred |
| Network | loopback only, nothing published, no host mount; `IPAddressDeny` and `PrivateNetwork` appear only as static score lines | real interfaces, a reverse proxy in front | measured |
| Timing | emulated amd64: the form routes cost about 5.7 to 6.0 ms against 0.2 to 0.9 ms on arm64 | native | measured (not benchmarks) |
| Rosetta versus QEMU | Rosetta, inferred from the crash text; QEMU emulation was not tried | not applicable | assumed |

**Seccomp-based directives.** The directives the container did not enforce on amd64 are the ones that systemd implements with a seccomp filter. The systemd.exec
documentation (https://manpages.debian.org/trixie/systemd/systemd.exec.5.en.html, read on 2026-10-12) states it as follows. `SystemCallFilter=` names the Secure Computing Mode 2 interfaces
(seccomp filtering) explicitly. `MemoryDenyWriteExecute=` says that a system call filter is added (or an equivalent kernel check enabled with `prctl`), and
`SystemCallArchitectures=` says its architectures are included in the system call filter. `RestrictAddressFamilies=` and `RestrictNamespaces=` describe how they restrict
the `socket`, `unshare`, `clone` and `setns` system calls but do not use the word seccomp. `RestrictRealtime=`, `RestrictSUIDSGID=` and `LockPersonality=` are
described without naming the mechanism; that they use seccomp comes from the systemd source and is assumed here, not checked in the manual.

| Directive in the unit | Enforced on amd64 here | How known | Status |
|---|---|---|---|
| `MemoryDenyWriteExecute=true` | no | the canary: `mprotect RW->RX` and `mmap RWX` both ALLOWED under `yes` | measured |
| `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX` | no | the canary: `socket AF_NETLINK` ALLOWED; denied on arm64 | measured |
| `SystemCallFilter=@system-service` and `~@privileged @resources` | no | the main process shows `Seccomp: 0`, so no filter of any kind | inferred |
| `SystemCallArchitectures=native` | no | the same | inferred |
| `RestrictNamespaces=true` | no | the same | inferred |
| `RestrictRealtime=true` | no | the same | inferred |
| `RestrictSUIDSGID=true` | no | the same | inferred |
| `LockPersonality=true` | no | the same | inferred |

The consequence for this document: on amd64 the score is still the score of the directives (the tool reads what the unit says), but a run that shows the service working
there says nothing about whether the service survives these filters. Only the arm64 run is under a filter, and only the MDWE and address-family directives were probed
there. That the service also survives `SystemCallFilter=@system-service` on a real server rests on the arm64 run, where the main process ran with 24 filters installed and no kill (`PASS stable`). The system call tables of arm64 and amd64 differ, so that this carries over to amd64 is assumed.

## Not measured

- Native amd64: no run was made and none is planned (user decision). The assumption stated under UNIT-02 stays an assumption.
- Ubuntu 24.04 with systemd 255, and any other systemd version than 252 and 257.
- Debian 12 on real hardware.
- QEMU emulation, in place of the emulation layer that this host used.

To confirm on a real server, the operator runs two commands:

```text
journalctl -u holzcloud | grep 'plugin loaded'
grep Seccomp /proc/$(systemctl show -p MainPID --value holzcloud)/status
```

The first should list the five plugins once per start, the second should show `Seccomp: 2`. On a server whose kernel enforces seccomp, a plugin running on the interpreter
leaves no anonymous executable mapping in `/proc/PID/maps`; that is how the harness tells the engines apart.

## Appendix: raw output

Every block below is the content of the scratch file named in its heading, unchanged. Platform, base image and command are given for each.

### R1, amd64, bookworm: `score-online.txt`

Platform linux/amd64, base image debian:bookworm-slim, systemd 252. Command: `PLATFORM=linux/amd64 OUT_DIR=deploy/test/out/amd64 ./deploy/test/run.sh all` (the `score` step; `systemd-analyze security holzcloud --no-pager`).

```text
  NAME                                                        DESCRIPTION                                                                    EXPOSURE
✓ SystemCallFilter=~@swap                                     System call allow list defined for service, and @swap is not included          
✓ SystemCallFilter=~@resources                                System call allow list defined for service, and @resources is not included     
✓ SystemCallFilter=~@reboot                                   System call allow list defined for service, and @reboot is not included        
✓ SystemCallFilter=~@raw-io                                   System call allow list defined for service, and @raw-io is not included        
✓ SystemCallFilter=~@privileged                               System call allow list defined for service, and @privileged is not included    
✓ SystemCallFilter=~@obsolete                                 System call allow list defined for service, and @obsolete is not included      
✓ SystemCallFilter=~@mount                                    System call allow list defined for service, and @mount is not included         
✓ SystemCallFilter=~@module                                   System call allow list defined for service, and @module is not included        
✓ SystemCallFilter=~@debug                                    System call allow list defined for service, and @debug is not included         
✓ SystemCallFilter=~@cpu-emulation                            System call allow list defined for service, and @cpu-emulation is not included 
✓ SystemCallFilter=~@clock                                    System call allow list defined for service, and @clock is not included         
✗ RemoveIPC=                                                  Service user may leave SysV IPC objects around                                      0.1
✗ RootDirectory=/RootImage=                                   Service runs within the host's root directory                                       0.1
✓ User=/DynamicUser=                                          Service runs under a static non-root user identity                             
✓ RestrictRealtime=                                           Service realtime scheduling access is restricted                               
✓ CapabilityBoundingSet=~CAP_SYS_TIME                         Service processes cannot change the system clock                               
✓ NoNewPrivileges=                                            Service processes cannot acquire new privileges                                
✓ AmbientCapabilities=                                        Service process does not receive ambient capabilities                          
✓ CapabilityBoundingSet=~CAP_BPF                              Service may not load BPF programs                                              
✓ SystemCallArchitectures=                                    Service may execute system calls only with native ABI                          
✗ RestrictAddressFamilies=~AF_UNIX                            Service may allocate local sockets                                                  0.1
✗ RestrictAddressFamilies=~AF_(INET|INET6)                    Service may allocate Internet sockets                                               0.3
✓ ProtectSystem=                                              Service has strict read-only access to the OS file hierarchy                   
✓ ProtectProc=                                                Service has restricted access to process tree (/proc hidepid=)                 
✓ SupplementaryGroups=                                        Service has no supplementary groups                                            
✓ CapabilityBoundingSet=~CAP_SYS_RAWIO                        Service has no raw I/O access                                                  
✓ CapabilityBoundingSet=~CAP_SYS_PTRACE                       Service has no ptrace() debugging abilities                                    
✓ CapabilityBoundingSet=~CAP_SYS_(NICE|RESOURCE)              Service has no privileges to change resource use parameters                    
✓ CapabilityBoundingSet=~CAP_NET_ADMIN                        Service has no network configuration privileges                                
✓ CapabilityBoundingSet=~CAP_NET_(BIND_SERVICE|BROADCAST|RAW) Service has no elevated networking privileges                                  
✓ CapabilityBoundingSet=~CAP_AUDIT_*                          Service has no audit subsystem access                                          
✓ CapabilityBoundingSet=~CAP_SYS_ADMIN                        Service has no administrator privileges                                        
✓ PrivateTmp=                                                 Service has no access to other software's temporary files                      
✓ ProcSubset=                                                 Service has no access to non-process /proc files (/proc subset=)               
✓ CapabilityBoundingSet=~CAP_SYSLOG                           Service has no access to kernel logging                                        
✓ ProtectHome=                                                Service has no access to home directories                                      
✓ PrivateDevices=                                             Service has no access to hardware devices                                      
✗ PrivateNetwork=                                             Service has access to the host's network                                            0.5
✗ PrivateUsers=                                               Service has access to other users                                                   0.2
✗ DeviceAllow=                                                Service has a device ACL with some special devices: char-rtc:r                      0.1
✓ KeyringMode=                                                Service doesn't share key material with other services                         
✓ Delegate=                                                   Service does not maintain its own delegated control group subtree              
✗ IPAddressDeny=                                              Service does not define an IP address allow list                                    0.2
✓ NotifyAccess=                                               Service child processes cannot alter service state                             
✓ ProtectClock=                                               Service cannot write to the hardware clock or system clock                     
✓ CapabilityBoundingSet=~CAP_SYS_PACCT                        Service cannot use acct()                                                      
✓ CapabilityBoundingSet=~CAP_KILL                             Service cannot send UNIX signals to arbitrary processes                        
✓ ProtectKernelLogs=                                          Service cannot read from or write to the kernel log ring buffer                
✓ CapabilityBoundingSet=~CAP_WAKE_ALARM                       Service cannot program timers that wake up the system                          
✓ CapabilityBoundingSet=~CAP_(DAC_*|FOWNER|IPC_OWNER)         Service cannot override UNIX file/IPC permission checks                        
✓ ProtectControlGroups=                                       Service cannot modify the control group file system                            
✓ CapabilityBoundingSet=~CAP_LINUX_IMMUTABLE                  Service cannot mark files immutable                                            
✓ CapabilityBoundingSet=~CAP_IPC_LOCK                         Service cannot lock memory into RAM                                            
✓ ProtectKernelModules=                                       Service cannot load or read kernel modules                                     
✓ CapabilityBoundingSet=~CAP_SYS_MODULE                       Service cannot load kernel modules                                             
✓ CapabilityBoundingSet=~CAP_SYS_TTY_CONFIG                   Service cannot issue vhangup()                                                 
✓ CapabilityBoundingSet=~CAP_SYS_BOOT                         Service cannot issue reboot()                                                  
✓ CapabilityBoundingSet=~CAP_SYS_CHROOT                       Service cannot issue chroot()                                                  
✓ PrivateMounts=                                              Service cannot install system mounts                                           
✓ CapabilityBoundingSet=~CAP_BLOCK_SUSPEND                    Service cannot establish wake locks                                            
✓ MemoryDenyWriteExecute=                                     Service cannot create writable executable memory mappings                      
✓ RestrictNamespaces=~user                                    Service cannot create user namespaces                                          
✓ RestrictNamespaces=~pid                                     Service cannot create process namespaces                                       
✓ RestrictNamespaces=~net                                     Service cannot create network namespaces                                       
✓ RestrictNamespaces=~uts                                     Service cannot create hostname namespaces                                      
✓ RestrictNamespaces=~mnt                                     Service cannot create file system namespaces                                   
✓ CapabilityBoundingSet=~CAP_LEASE                            Service cannot create file leases                                              
✓ CapabilityBoundingSet=~CAP_MKNOD                            Service cannot create device nodes                                             
✓ RestrictNamespaces=~cgroup                                  Service cannot create cgroup namespaces                                        
✓ RestrictNamespaces=~ipc                                     Service cannot create IPC namespaces                                           
✓ ProtectHostname=                                            Service cannot change system host/domainname                                   
✓ CapabilityBoundingSet=~CAP_(CHOWN|FSETID|SETFCAP)           Service cannot change file ownership/access mode/capabilities                  
✓ CapabilityBoundingSet=~CAP_SET(UID|GID|PCAP)                Service cannot change UID/GID identities/capabilities                          
✓ LockPersonality=                                            Service cannot change ABI personality                                          
✓ ProtectKernelTunables=                                      Service cannot alter kernel tunables (/proc/sys, …)                            
✓ RestrictAddressFamilies=~AF_PACKET                          Service cannot allocate packet sockets                                         
✓ RestrictAddressFamilies=~AF_NETLINK                         Service cannot allocate netlink sockets                                        
✓ RestrictAddressFamilies=~…                                  Service cannot allocate exotic sockets                                         
✓ CapabilityBoundingSet=~CAP_MAC_*                            Service cannot adjust SMACK MAC                                                
✓ RestrictSUIDSGID=                                           SUID/SGID file creation by service is restricted                               
✓ UMask=                                                      Files created by service are accessible only by service's own user by default  

→ Overall exposure level for holzcloud.service: 1.3 OK :-)
```

### R1, amd64, bookworm: `score-offline.txt`

Platform linux/amd64, base image debian:bookworm-slim, systemd 252. Command: the `score` step of `all`; `systemd-analyze security --offline=true --no-pager /etc/systemd/system/holzcloud.service`.

```text
  NAME                                                        DESCRIPTION                                                                    EXPOSURE
✓ SystemCallFilter=~@swap                                     System call allow list defined for service, and @swap is not included          
✓ SystemCallFilter=~@resources                                System call allow list defined for service, and @resources is not included     
✓ SystemCallFilter=~@reboot                                   System call allow list defined for service, and @reboot is not included        
✓ SystemCallFilter=~@raw-io                                   System call allow list defined for service, and @raw-io is not included        
✓ SystemCallFilter=~@privileged                               System call allow list defined for service, and @privileged is not included    
✓ SystemCallFilter=~@obsolete                                 System call allow list defined for service, and @obsolete is not included      
✓ SystemCallFilter=~@mount                                    System call allow list defined for service, and @mount is not included         
✓ SystemCallFilter=~@module                                   System call allow list defined for service, and @module is not included        
✓ SystemCallFilter=~@debug                                    System call allow list defined for service, and @debug is not included         
✓ SystemCallFilter=~@cpu-emulation                            System call allow list defined for service, and @cpu-emulation is not included 
✓ SystemCallFilter=~@clock                                    System call allow list defined for service, and @clock is not included         
✗ RemoveIPC=                                                  Service user may leave SysV IPC objects around                                      0.1
✗ RootDirectory=/RootImage=                                   Service runs within the host's root directory                                       0.1
✓ User=/DynamicUser=                                          Service runs under a static non-root user identity                             
✓ RestrictRealtime=                                           Service realtime scheduling access is restricted                               
✓ CapabilityBoundingSet=~CAP_SYS_TIME                         Service processes cannot change the system clock                               
✓ NoNewPrivileges=                                            Service processes cannot acquire new privileges                                
✓ AmbientCapabilities=                                        Service process does not receive ambient capabilities                          
✓ CapabilityBoundingSet=~CAP_BPF                              Service may not load BPF programs                                              
✗ PrivateMounts=                                              Service may install system mounts                                                   0.2
✓ SystemCallArchitectures=                                    Service may execute system calls only with native ABI                          
✗ RestrictAddressFamilies=~AF_UNIX                            Service may allocate local sockets                                                  0.1
✗ RestrictAddressFamilies=~AF_(INET|INET6)                    Service may allocate Internet sockets                                               0.3
✓ ProtectSystem=                                              Service has strict read-only access to the OS file hierarchy                   
✓ ProtectProc=                                                Service has restricted access to process tree (/proc hidepid=)                 
✓ SupplementaryGroups=                                        Service has no supplementary groups                                            
✓ CapabilityBoundingSet=~CAP_SYS_RAWIO                        Service has no raw I/O access                                                  
✓ CapabilityBoundingSet=~CAP_SYS_PTRACE                       Service has no ptrace() debugging abilities                                    
✓ CapabilityBoundingSet=~CAP_SYS_(NICE|RESOURCE)              Service has no privileges to change resource use parameters                    
✓ CapabilityBoundingSet=~CAP_NET_ADMIN                        Service has no network configuration privileges                                
✓ CapabilityBoundingSet=~CAP_NET_(BIND_SERVICE|BROADCAST|RAW) Service has no elevated networking privileges                                  
✓ CapabilityBoundingSet=~CAP_AUDIT_*                          Service has no audit subsystem access                                          
✓ CapabilityBoundingSet=~CAP_SYS_ADMIN                        Service has no administrator privileges                                        
✓ PrivateTmp=                                                 Service has no access to other software's temporary files                      
✓ ProcSubset=                                                 Service has no access to non-process /proc files (/proc subset=)               
✓ CapabilityBoundingSet=~CAP_SYSLOG                           Service has no access to kernel logging                                        
✓ ProtectHome=                                                Service has no access to home directories                                      
✓ PrivateDevices=                                             Service has no access to hardware devices                                      
✗ PrivateNetwork=                                             Service has access to the host's network                                            0.5
✗ PrivateUsers=                                               Service has access to other users                                                   0.2
✗ DeviceAllow=                                                Service has a device ACL with some special devices: char-rtc:r                      0.1
✓ KeyringMode=                                                Service doesn't share key material with other services                         
✓ Delegate=                                                   Service does not maintain its own delegated control group subtree              
✗ IPAddressDeny=                                              Service does not define an IP address allow list                                    0.2
✓ NotifyAccess=                                               Service child processes cannot alter service state                             
✓ ProtectClock=                                               Service cannot write to the hardware clock or system clock                     
✓ CapabilityBoundingSet=~CAP_SYS_PACCT                        Service cannot use acct()                                                      
✓ CapabilityBoundingSet=~CAP_KILL                             Service cannot send UNIX signals to arbitrary processes                        
✓ ProtectKernelLogs=                                          Service cannot read from or write to the kernel log ring buffer                
✓ CapabilityBoundingSet=~CAP_WAKE_ALARM                       Service cannot program timers that wake up the system                          
✓ CapabilityBoundingSet=~CAP_(DAC_*|FOWNER|IPC_OWNER)         Service cannot override UNIX file/IPC permission checks                        
✓ ProtectControlGroups=                                       Service cannot modify the control group file system                            
✓ CapabilityBoundingSet=~CAP_LINUX_IMMUTABLE                  Service cannot mark files immutable                                            
✓ CapabilityBoundingSet=~CAP_IPC_LOCK                         Service cannot lock memory into RAM                                            
✓ ProtectKernelModules=                                       Service cannot load or read kernel modules                                     
✓ CapabilityBoundingSet=~CAP_SYS_MODULE                       Service cannot load kernel modules                                             
✓ CapabilityBoundingSet=~CAP_SYS_TTY_CONFIG                   Service cannot issue vhangup()                                                 
✓ CapabilityBoundingSet=~CAP_SYS_BOOT                         Service cannot issue reboot()                                                  
✓ CapabilityBoundingSet=~CAP_SYS_CHROOT                       Service cannot issue chroot()                                                  
✓ CapabilityBoundingSet=~CAP_BLOCK_SUSPEND                    Service cannot establish wake locks                                            
✓ MemoryDenyWriteExecute=                                     Service cannot create writable executable memory mappings                      
✓ RestrictNamespaces=~user                                    Service cannot create user namespaces                                          
✓ RestrictNamespaces=~pid                                     Service cannot create process namespaces                                       
✓ RestrictNamespaces=~net                                     Service cannot create network namespaces                                       
✓ RestrictNamespaces=~uts                                     Service cannot create hostname namespaces                                      
✓ RestrictNamespaces=~mnt                                     Service cannot create file system namespaces                                   
✓ CapabilityBoundingSet=~CAP_LEASE                            Service cannot create file leases                                              
✓ CapabilityBoundingSet=~CAP_MKNOD                            Service cannot create device nodes                                             
✓ RestrictNamespaces=~cgroup                                  Service cannot create cgroup namespaces                                        
✓ RestrictNamespaces=~ipc                                     Service cannot create IPC namespaces                                           
✓ ProtectHostname=                                            Service cannot change system host/domainname                                   
✓ CapabilityBoundingSet=~CAP_(CHOWN|FSETID|SETFCAP)           Service cannot change file ownership/access mode/capabilities                  
✓ CapabilityBoundingSet=~CAP_SET(UID|GID|PCAP)                Service cannot change UID/GID identities/capabilities                          
✓ LockPersonality=                                            Service cannot change ABI personality                                          
✓ ProtectKernelTunables=                                      Service cannot alter kernel tunables (/proc/sys, …)                            
✓ RestrictAddressFamilies=~AF_PACKET                          Service cannot allocate packet sockets                                         
✓ RestrictAddressFamilies=~AF_NETLINK                         Service cannot allocate netlink sockets                                        
✓ RestrictAddressFamilies=~…                                  Service cannot allocate exotic sockets                                         
✓ CapabilityBoundingSet=~CAP_MAC_*                            Service cannot adjust SMACK MAC                                                
✓ RestrictSUIDSGID=                                           SUID/SGID file creation by service is restricted                               
✓ UMask=                                                      Files created by service are accessible only by service's own user by default  

→ Overall exposure level for holzcloud.service: 1.4 OK :-)
```

### R1, amd64, bookworm: `score-unit.txt`

Platform linux/amd64, base image debian:bookworm-slim. Command: the `score` step of `all`; `systemctl cat holzcloud`. The unit as the manager had it at measurement time; no drop-in is listed.

```text
# /etc/systemd/system/holzcloud.service
[Unit]
Description=holzcloud-CMS
Documentation=https://github.com/holz/holzcloud
After=network.target
# Refuse to start on an unmounted data disk rather than creating an empty one.
RequiresMountsFor=/opt/holzcloud/data
StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
Type=simple
User=holzcloud
Group=holzcloud
WorkingDirectory=/opt/holzcloud
ExecStart=/opt/holzcloud/holzcloud
Restart=on-failure
RestartSec=5

# Environment — names must match internal/config.Load()
Environment=HOLZCLOUD_DATA_DIR=/opt/holzcloud/data
Environment=HOLZCLOUD_PORT=8080
Environment=HOLZCLOUD_LOG_LEVEL=INFO
# Set to true when the site is reached over HTTPS (Caddy terminates TLS).
# Controls the Secure flag on the session cookie and the CSRF cookie.
Environment=HOLZCLOUD_SECURE=true
# CIDRs whose X-Forwarded-For may be believed. Caddy on this host is covered by
# the default; without it every visitor would look like 127.0.0.1 and share one
# login-throttle bucket.
Environment=HOLZCLOUD_TRUSTED_PROXIES=127.0.0.1/32,::1/128

# Online payment. Both or neither — the service refuses to start with one.
# The key goes here and nowhere else: it must not end up in the database,
# because the database is what gets copied into every backup.
#
# Keep it out of this file if the file is in version control. Put it in
# /etc/holzcloud.env with mode 0600 and owner holzcloud, and uncomment:
#   EnvironmentFile=/etc/holzcloud.env
#Environment=HOLZCLOUD_PAYREXX_INSTANCE=meinladen
#Environment=HOLZCLOUD_PAYREXX_SECRET=

# Mailversand für die Bestellbestätigungen. Host und Absender zusammen, sonst
# startet der Dienst nicht. Das Passwort gehört wie der Zahlungsschlüssel in
# eine EnvironmentFile mit Rechten 0600, nicht in diese Datei.
#Environment=HOLZCLOUD_SMTP_HOST=mail.example.ch
#Environment=HOLZCLOUD_SMTP_PORT=587
#Environment=HOLZCLOUD_SMTP_USER=shop@example.ch
#Environment=HOLZCLOUD_SMTP_PASSWORD=
#Environment=HOLZCLOUD_SMTP_FROM=shop@example.ch
#
# Schlüssel für die Passwörter der Mailkonten einzelner Websites (Einstellungen
# einer Website, „Versand“). Ebenfalls in die EnvironmentFile, nie in die
# Datenbank: openssl rand -base64 32
#HOLZCLOUD_SECRET_KEY=…  (in der EnvironmentFile)
#
# Anmeldung über den Ausweisdienst: das gemeinsame Geheimnis gehört ebenfalls in
# eine EnvironmentFile mit Rechten 0600 — und dieselbe Datei braucht Caddy.
# deploy/DEPLOY.md, "The shared secret lives in two environments".
#EnvironmentFile=/etc/holzcloud-sso.env

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/holzcloud/data
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictNamespaces=true
SystemCallArchitectures=native
# wazero, the plugin runtime, probes at start whether it may map memory executable.
# Under this directive the probe fails and wazero silently falls back to its
# interpreter: every plugin still loads and runs (measured, see deploy/SECURITY-SCORE.md),
# slower than the compiler but far inside the per-call budget (CallTimeout).
# Tripwire: pinning the runtime to the compiler only (NewRuntimeConfigCompiler in
# internal/plugin/runtime.go) would make plugins fail under this line; it would then
# have to become MemoryDenyWriteExecute=false.
MemoryDenyWriteExecute=true
LockPersonality=true
RestrictRealtime=true
PrivateDevices=true
ProtectClock=true
ProtectHostname=true
ProtectKernelLogs=true
# The process sees only its own entries in /proc, and only the process part of it.
ProtectProc=invisible
ProcSubset=pid
# Only TCP/UDP and local sockets. The service listens on an unprivileged port
# (HOLZCLOUD_PORT, default 8080) and talks out over IPv4/IPv6 only (mail, payment,
# sign-in); it needs no raw or packet sockets, no netlink.
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
# A Go program with SQLite and an embedded WebAssembly runtime stays inside
# @system-service. Privileged calls (setuid, chown, mount ...) and resource
# tuning are cut off; a call outside the list ends the process.
SystemCallFilter=@system-service
SystemCallFilter=~@privileged @resources
# No capabilities at all: port 8080 needs none. If you move HOLZCLOUD_PORT below
# 1024, put CAP_NET_BIND_SERVICE into both lines below (or keep the proxy in front).
CapabilityBoundingSet=
AmbientCapabilities=
# Files the service creates (media, database, templates) are readable by the
# service user only. Caddy talks to the port and never reads the data directory.
UMask=0077

[Install]
WantedBy=multi-user.target
```

### R3, amd64, trixie: `score-online.txt`

Platform linux/amd64, base image debian:trixie-slim, systemd 257. Command: `PLATFORM=linux/amd64 BASE_IMAGE=debian:trixie-slim OUT_DIR=deploy/test/out/amd64-trixie ./deploy/test/run.sh build up score`; `systemd-analyze security holzcloud --no-pager`.

```text
  NAME                                                        DESCRIPTION                                                                    EXPOSURE
✓ SystemCallFilter=~@swap                                     System call allow list defined for service, and @swap is not included          
✓ SystemCallFilter=~@resources                                System call allow list defined for service, and @resources is not included     
✓ SystemCallFilter=~@reboot                                   System call allow list defined for service, and @reboot is not included        
✓ SystemCallFilter=~@raw-io                                   System call allow list defined for service, and @raw-io is not included        
✓ SystemCallFilter=~@privileged                               System call allow list defined for service, and @privileged is not included    
✓ SystemCallFilter=~@obsolete                                 System call allow list defined for service, and @obsolete is not included      
✓ SystemCallFilter=~@mount                                    System call allow list defined for service, and @mount is not included         
✓ SystemCallFilter=~@module                                   System call allow list defined for service, and @module is not included        
✓ SystemCallFilter=~@debug                                    System call allow list defined for service, and @debug is not included         
✓ SystemCallFilter=~@cpu-emulation                            System call allow list defined for service, and @cpu-emulation is not included 
✓ SystemCallFilter=~@clock                                    System call allow list defined for service, and @clock is not included         
✗ RemoveIPC=                                                  Service user may leave SysV IPC objects around                                      0.1
✗ RootDirectory=/RootImage=                                   Service runs within the host's root directory                                       0.1
✓ User=/DynamicUser=                                          Service runs under a static non-root user identity                             
✓ RestrictRealtime=                                           Service realtime scheduling access is restricted                               
✓ CapabilityBoundingSet=~CAP_SYS_TIME                         Service processes cannot change the system clock                               
✓ NoNewPrivileges=                                            Service processes cannot acquire new privileges                                
✓ AmbientCapabilities=                                        Service process does not receive ambient capabilities                          
✓ CapabilityBoundingSet=~CAP_BPF                              Service may not load BPF programs                                              
✓ SystemCallArchitectures=                                    Service may execute system calls only with native ABI                          
✗ RestrictAddressFamilies=~AF_UNIX                            Service may allocate local sockets                                                  0.1
✗ RestrictAddressFamilies=~AF_(INET|INET6)                    Service may allocate Internet sockets                                               0.3
✓ ProtectSystem=                                              Service has strict read-only access to the OS file hierarchy                   
✓ ProtectProc=                                                Service has restricted access to process tree (/proc hidepid=)                 
✓ SupplementaryGroups=                                        Service has no supplementary groups                                            
✓ CapabilityBoundingSet=~CAP_SYS_RAWIO                        Service has no raw I/O access                                                  
✓ CapabilityBoundingSet=~CAP_SYS_PTRACE                       Service has no ptrace() debugging abilities                                    
✓ CapabilityBoundingSet=~CAP_SYS_(NICE|RESOURCE)              Service has no privileges to change resource use parameters                    
✓ CapabilityBoundingSet=~CAP_NET_ADMIN                        Service has no network configuration privileges                                
✓ CapabilityBoundingSet=~CAP_NET_(BIND_SERVICE|BROADCAST|RAW) Service has no elevated networking privileges                                  
✓ CapabilityBoundingSet=~CAP_AUDIT_*                          Service has no audit subsystem access                                          
✓ CapabilityBoundingSet=~CAP_SYS_ADMIN                        Service has no administrator privileges                                        
✓ PrivateTmp=                                                 Service has no access to other software's temporary files                      
✓ ProcSubset=                                                 Service has no access to non-process /proc files (/proc subset=)               
✓ CapabilityBoundingSet=~CAP_SYSLOG                           Service has no access to kernel logging                                        
✓ ProtectHome=                                                Service has no access to home directories                                      
✓ PrivateDevices=                                             Service has no access to hardware devices                                      
✗ PrivateNetwork=                                             Service has access to the host's network                                            0.5
✗ PrivateUsers=                                               Service has access to other users                                                   0.2
✗ DeviceAllow=                                                Service has a device ACL with some special devices: char-rtc:r                      0.1
✓ KeyringMode=                                                Service doesn't share key material with other services                         
✓ Delegate=                                                   Service does not maintain its own delegated control group subtree              
✗ IPAddressDeny=                                              Service does not define an IP address allow list                                    0.2
✓ NotifyAccess=                                               Service child processes cannot alter service state                             
✓ ProtectClock=                                               Service cannot write to the hardware clock or system clock                     
✓ CapabilityBoundingSet=~CAP_SYS_PACCT                        Service cannot use acct()                                                      
✓ CapabilityBoundingSet=~CAP_KILL                             Service cannot send UNIX signals to arbitrary processes                        
✓ ProtectKernelLogs=                                          Service cannot read from or write to the kernel log ring buffer                
✓ CapabilityBoundingSet=~CAP_WAKE_ALARM                       Service cannot program timers that wake up the system                          
✓ CapabilityBoundingSet=~CAP_(DAC_*|FOWNER|IPC_OWNER)         Service cannot override UNIX file/IPC permission checks                        
✓ ProtectControlGroups=                                       Service cannot modify the control group file system                            
✓ CapabilityBoundingSet=~CAP_LINUX_IMMUTABLE                  Service cannot mark files immutable                                            
✓ CapabilityBoundingSet=~CAP_IPC_LOCK                         Service cannot lock memory into RAM                                            
✓ ProtectKernelModules=                                       Service cannot load or read kernel modules                                     
✓ CapabilityBoundingSet=~CAP_SYS_MODULE                       Service cannot load kernel modules                                             
✓ CapabilityBoundingSet=~CAP_SYS_TTY_CONFIG                   Service cannot issue vhangup()                                                 
✓ CapabilityBoundingSet=~CAP_SYS_BOOT                         Service cannot issue reboot()                                                  
✓ CapabilityBoundingSet=~CAP_SYS_CHROOT                       Service cannot issue chroot()                                                  
✓ PrivateMounts=                                              Service cannot install system mounts                                           
✓ CapabilityBoundingSet=~CAP_BLOCK_SUSPEND                    Service cannot establish wake locks                                            
✓ MemoryDenyWriteExecute=                                     Service cannot create writable executable memory mappings                      
✓ RestrictNamespaces=~user                                    Service cannot create user namespaces                                          
✓ RestrictNamespaces=~pid                                     Service cannot create process namespaces                                       
✓ RestrictNamespaces=~net                                     Service cannot create network namespaces                                       
✓ RestrictNamespaces=~uts                                     Service cannot create hostname namespaces                                      
✓ RestrictNamespaces=~mnt                                     Service cannot create file system namespaces                                   
✓ CapabilityBoundingSet=~CAP_LEASE                            Service cannot create file leases                                              
✓ CapabilityBoundingSet=~CAP_MKNOD                            Service cannot create device nodes                                             
✓ RestrictNamespaces=~cgroup                                  Service cannot create cgroup namespaces                                        
✓ RestrictNamespaces=~ipc                                     Service cannot create IPC namespaces                                           
✓ ProtectHostname=                                            Service cannot change system host/domainname                                   
✓ CapabilityBoundingSet=~CAP_(CHOWN|FSETID|SETFCAP)           Service cannot change file ownership/access mode/capabilities                  
✓ CapabilityBoundingSet=~CAP_SET(UID|GID|PCAP)                Service cannot change UID/GID identities/capabilities                          
✓ LockPersonality=                                            Service cannot change ABI personality                                          
✓ ProtectKernelTunables=                                      Service cannot alter kernel tunables (/proc/sys, …)                            
✓ RestrictAddressFamilies=~AF_PACKET                          Service cannot allocate packet sockets                                         
✓ RestrictAddressFamilies=~AF_NETLINK                         Service cannot allocate netlink sockets                                        
✓ RestrictAddressFamilies=~…                                  Service cannot allocate exotic sockets                                         
✓ CapabilityBoundingSet=~CAP_MAC_*                            Service cannot adjust SMACK MAC                                                
✓ RestrictSUIDSGID=                                           SUID/SGID file creation by service is restricted                               
✓ UMask=                                                      Files created by service are accessible only by service's own user by default  

→ Overall exposure level for holzcloud.service: 1.3 OK :-)
```

### R3, amd64, trixie: `score-offline.txt`

Platform linux/amd64, base image debian:trixie-slim, systemd 257. Command: the same run; `systemd-analyze security --offline=true --no-pager /etc/systemd/system/holzcloud.service`.

```text
  NAME                                                        DESCRIPTION                                                                    EXPOSURE
✓ SystemCallFilter=~@swap                                     System call allow list defined for service, and @swap is not included          
✓ SystemCallFilter=~@resources                                System call allow list defined for service, and @resources is not included     
✓ SystemCallFilter=~@reboot                                   System call allow list defined for service, and @reboot is not included        
✓ SystemCallFilter=~@raw-io                                   System call allow list defined for service, and @raw-io is not included        
✓ SystemCallFilter=~@privileged                               System call allow list defined for service, and @privileged is not included    
✓ SystemCallFilter=~@obsolete                                 System call allow list defined for service, and @obsolete is not included      
✓ SystemCallFilter=~@mount                                    System call allow list defined for service, and @mount is not included         
✓ SystemCallFilter=~@module                                   System call allow list defined for service, and @module is not included        
✓ SystemCallFilter=~@debug                                    System call allow list defined for service, and @debug is not included         
✓ SystemCallFilter=~@cpu-emulation                            System call allow list defined for service, and @cpu-emulation is not included 
✓ SystemCallFilter=~@clock                                    System call allow list defined for service, and @clock is not included         
✗ RemoveIPC=                                                  Service user may leave SysV IPC objects around                                      0.1
✗ RootDirectory=/RootImage=                                   Service runs within the host's root directory                                       0.1
✓ User=/DynamicUser=                                          Service runs under a static non-root user identity                             
✓ RestrictRealtime=                                           Service realtime scheduling access is restricted                               
✓ CapabilityBoundingSet=~CAP_SYS_TIME                         Service processes cannot change the system clock                               
✓ NoNewPrivileges=                                            Service processes cannot acquire new privileges                                
✓ AmbientCapabilities=                                        Service process does not receive ambient capabilities                          
✓ CapabilityBoundingSet=~CAP_BPF                              Service may not load BPF programs                                              
✓ SystemCallArchitectures=                                    Service may execute system calls only with native ABI                          
✗ RestrictAddressFamilies=~AF_UNIX                            Service may allocate local sockets                                                  0.1
✗ RestrictAddressFamilies=~AF_(INET|INET6)                    Service may allocate Internet sockets                                               0.3
✓ ProtectSystem=                                              Service has strict read-only access to the OS file hierarchy                   
✓ ProtectProc=                                                Service has restricted access to process tree (/proc hidepid=)                 
✓ SupplementaryGroups=                                        Service has no supplementary groups                                            
✓ CapabilityBoundingSet=~CAP_SYS_RAWIO                        Service has no raw I/O access                                                  
✓ CapabilityBoundingSet=~CAP_SYS_PTRACE                       Service has no ptrace() debugging abilities                                    
✓ CapabilityBoundingSet=~CAP_SYS_(NICE|RESOURCE)              Service has no privileges to change resource use parameters                    
✓ CapabilityBoundingSet=~CAP_NET_ADMIN                        Service has no network configuration privileges                                
✓ CapabilityBoundingSet=~CAP_NET_(BIND_SERVICE|BROADCAST|RAW) Service has no elevated networking privileges                                  
✓ CapabilityBoundingSet=~CAP_AUDIT_*                          Service has no audit subsystem access                                          
✓ CapabilityBoundingSet=~CAP_SYS_ADMIN                        Service has no administrator privileges                                        
✓ PrivateTmp=                                                 Service has no access to other software's temporary files                      
✓ ProcSubset=                                                 Service has no access to non-process /proc files (/proc subset=)               
✓ CapabilityBoundingSet=~CAP_SYSLOG                           Service has no access to kernel logging                                        
✓ ProtectHome=                                                Service has no access to home directories                                      
✓ PrivateDevices=                                             Service has no access to hardware devices                                      
✗ PrivateNetwork=                                             Service has access to the host's network                                            0.5
✗ PrivateUsers=                                               Service has access to other users                                                   0.2
✗ DeviceAllow=                                                Service has a device ACL with some special devices: char-rtc:r                      0.1
✓ KeyringMode=                                                Service doesn't share key material with other services                         
✓ Delegate=                                                   Service does not maintain its own delegated control group subtree              
✗ IPAddressDeny=                                              Service does not define an IP address allow list                                    0.2
✓ NotifyAccess=                                               Service child processes cannot alter service state                             
✓ ProtectClock=                                               Service cannot write to the hardware clock or system clock                     
✓ CapabilityBoundingSet=~CAP_SYS_PACCT                        Service cannot use acct()                                                      
✓ CapabilityBoundingSet=~CAP_KILL                             Service cannot send UNIX signals to arbitrary processes                        
✓ ProtectKernelLogs=                                          Service cannot read from or write to the kernel log ring buffer                
✓ CapabilityBoundingSet=~CAP_WAKE_ALARM                       Service cannot program timers that wake up the system                          
✓ CapabilityBoundingSet=~CAP_(DAC_*|FOWNER|IPC_OWNER)         Service cannot override UNIX file/IPC permission checks                        
✓ ProtectControlGroups=                                       Service cannot modify the control group file system                            
✓ CapabilityBoundingSet=~CAP_LINUX_IMMUTABLE                  Service cannot mark files immutable                                            
✓ CapabilityBoundingSet=~CAP_IPC_LOCK                         Service cannot lock memory into RAM                                            
✓ ProtectKernelModules=                                       Service cannot load or read kernel modules                                     
✓ CapabilityBoundingSet=~CAP_SYS_MODULE                       Service cannot load kernel modules                                             
✓ CapabilityBoundingSet=~CAP_SYS_TTY_CONFIG                   Service cannot issue vhangup()                                                 
✓ CapabilityBoundingSet=~CAP_SYS_BOOT                         Service cannot issue reboot()                                                  
✓ CapabilityBoundingSet=~CAP_SYS_CHROOT                       Service cannot issue chroot()                                                  
✓ PrivateMounts=                                              Service cannot install system mounts                                           
✓ CapabilityBoundingSet=~CAP_BLOCK_SUSPEND                    Service cannot establish wake locks                                            
✓ MemoryDenyWriteExecute=                                     Service cannot create writable executable memory mappings                      
✓ RestrictNamespaces=~user                                    Service cannot create user namespaces                                          
✓ RestrictNamespaces=~pid                                     Service cannot create process namespaces                                       
✓ RestrictNamespaces=~net                                     Service cannot create network namespaces                                       
✓ RestrictNamespaces=~uts                                     Service cannot create hostname namespaces                                      
✓ RestrictNamespaces=~mnt                                     Service cannot create file system namespaces                                   
✓ CapabilityBoundingSet=~CAP_LEASE                            Service cannot create file leases                                              
✓ CapabilityBoundingSet=~CAP_MKNOD                            Service cannot create device nodes                                             
✓ RestrictNamespaces=~cgroup                                  Service cannot create cgroup namespaces                                        
✓ RestrictNamespaces=~ipc                                     Service cannot create IPC namespaces                                           
✓ ProtectHostname=                                            Service cannot change system host/domainname                                   
✓ CapabilityBoundingSet=~CAP_(CHOWN|FSETID|SETFCAP)           Service cannot change file ownership/access mode/capabilities                  
✓ CapabilityBoundingSet=~CAP_SET(UID|GID|PCAP)                Service cannot change UID/GID identities/capabilities                          
✓ LockPersonality=                                            Service cannot change ABI personality                                          
✓ ProtectKernelTunables=                                      Service cannot alter kernel tunables (/proc/sys, …)                            
✓ RestrictAddressFamilies=~AF_PACKET                          Service cannot allocate packet sockets                                         
✓ RestrictAddressFamilies=~AF_NETLINK                         Service cannot allocate netlink sockets                                        
✓ RestrictAddressFamilies=~…                                  Service cannot allocate exotic sockets                                         
✓ CapabilityBoundingSet=~CAP_MAC_*                            Service cannot adjust SMACK MAC                                                
✓ RestrictSUIDSGID=                                           SUID/SGID file creation by service is restricted                               
✓ UMask=                                                      Files created by service are accessible only by service's own user by default  

→ Overall exposure level for holzcloud.service: 1.3 OK :-)
```

### R1, amd64: `canary.txt`

Platform linux/amd64, base image debian:bookworm-slim. Command: the `canary` step of `all`.

```text
== MemoryDenyWriteExecute=yes ==
mmap RW                ALLOWED
mprotect RW->RX        ALLOWED
mmap RWX               ALLOWED
socket AF_NETLINK      ALLOWED
socket AF_INET         ALLOWED
stat /proc/sys         ALLOWED
stat /dev/kmsg         denied (no such file or directory)
write /etc             denied (read-only file system)
Seccomp:	0
Seccomp_filters:	0

== MemoryDenyWriteExecute=no (control) ==
mmap RW                ALLOWED
mprotect RW->RX        ALLOWED
mmap RWX               ALLOWED
socket AF_NETLINK      ALLOWED
socket AF_INET         ALLOWED
stat /proc/sys         ALLOWED
stat /dev/kmsg         denied (no such file or directory)
write /etc             denied (read-only file system)
Seccomp:	0
Seccomp_filters:	0
```

### R1, amd64: `check.txt`

Platform linux/amd64, base image debian:bookworm-slim. Command: the `check` step of `all`.

```text
PASS loaded bestellung
PASS loaded jahreszahl
PASS loaded kontaktformular
PASS loaded nicht-gefunden
PASS loaded suche
PASS no-load-failures
PASS running bestellung
PASS running jahreszahl
PASS running kontaktformular
PASS running nicht-gefunden
PASS running suche
PASS invoke jahreszahl
PASS invoke kontaktformular
PASS invoke bestellung
PASS invoke suche
PASS invoke nicht-gefunden
PASS stable
engine: compiler (9 anonymous executable mappings)
# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests
LATENCY /plugintest median_s=0.009147 n=10
LATENCY /suche?q=Plugintest median_s=0.008848 n=10
LATENCY /formular median_s=0.005883 n=10
LATENCY /bestellung median_s=0.005986 n=10
Seccomp:	0
Seccomp_filters:	0
NRestarts=0
Result=success
{"time":"2026-10-12T10:41:01.789655629Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}
{"time":"2026-10-12T10:41:03.058772463Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}
{"time":"2026-10-12T10:41:04.107872422Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}
{"time":"2026-10-12T10:41:05.023770297Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}
{"time":"2026-10-12T10:41:05.947229964Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}
```

### R1, amd64: `mdwe-control.txt`

Platform linux/amd64, base image debian:bookworm-slim. Command: the `mdwe-control` step of `all` (drop-in `MemoryDenyWriteExecute=no`, then restored).

```text
PASS loaded bestellung
PASS loaded jahreszahl
PASS loaded kontaktformular
PASS loaded nicht-gefunden
PASS loaded suche
PASS no-load-failures
PASS running bestellung
PASS running jahreszahl
PASS running kontaktformular
PASS running nicht-gefunden
PASS running suche
PASS invoke jahreszahl
PASS invoke kontaktformular
PASS invoke bestellung
PASS invoke suche
PASS invoke nicht-gefunden
PASS stable
engine: compiler (10 anonymous executable mappings)
# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests
LATENCY /plugintest median_s=0.009656 n=10
LATENCY /suche?q=Plugintest median_s=0.008233 n=10
LATENCY /formular median_s=0.005748 n=10
LATENCY /bestellung median_s=0.005695 n=10
Seccomp:	0
Seccomp_filters:	0
NRestarts=0
Result=success
{"time":"2026-10-12T10:41:13.005876676Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}
{"time":"2026-10-12T10:41:14.305524468Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}
{"time":"2026-10-12T10:41:15.326119344Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}
{"time":"2026-10-12T10:41:16.252086094Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}
{"time":"2026-10-12T10:41:17.153804678Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}
```

### R1, amd64: `verdict.txt`

Platform linux/amd64, base image debian:bookworm-slim. Command: the `verdict` step of `all`.

```text
MDWE VERDICT: VOID - seccomp is not enforced on this platform (linux/amd64), so the plugin results prove nothing about MemoryDenyWriteExecute; 17/17 assertions passed, engine under the shipped unit: compiler
```

### R1, amd64: `versions.txt`

Platform linux/amd64, base image debian:bookworm-slim. Command: written by `up`.

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```

### R2, arm64: `canary.txt`

Platform linux/arm64 (native), base image debian:bookworm-slim. Command: `PLATFORM=linux/arm64 OUT_DIR=deploy/test/out/arm64 ./deploy/test/run.sh mdwe` (the `canary` step).

```text
== MemoryDenyWriteExecute=yes ==
mmap RW                ALLOWED
mprotect RW->RX        denied (operation not permitted)
mmap RWX               denied (operation not permitted)
socket AF_NETLINK      denied (address family not supported by protocol)
socket AF_INET         ALLOWED
stat /proc/sys         denied (no such file or directory)
stat /dev/kmsg         denied (no such file or directory)
write /etc             denied (read-only file system)
Seccomp:	2
Seccomp_filters:	12

== MemoryDenyWriteExecute=no (control) ==
mmap RW                ALLOWED
mprotect RW->RX        ALLOWED
mmap RWX               ALLOWED
socket AF_NETLINK      denied (address family not supported by protocol)
socket AF_INET         ALLOWED
stat /proc/sys         denied (no such file or directory)
stat /dev/kmsg         denied (no such file or directory)
write /etc             denied (read-only file system)
Seccomp:	2
Seccomp_filters:	10
```

### R2, arm64: `check.txt`

Platform linux/arm64 (native), base image debian:bookworm-slim. Command: the `check` step of `mdwe`.

```text
PASS loaded bestellung
PASS loaded jahreszahl
PASS loaded kontaktformular
PASS loaded nicht-gefunden
PASS loaded suche
PASS no-load-failures
PASS running bestellung
PASS running jahreszahl
PASS running kontaktformular
PASS running nicht-gefunden
PASS running suche
PASS invoke jahreszahl
PASS invoke kontaktformular
PASS invoke bestellung
PASS invoke suche
PASS invoke nicht-gefunden
PASS stable
engine: interpreter (0 anonymous executable mappings)
# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests
LATENCY /plugintest median_s=0.011671 n=10
LATENCY /suche?q=Plugintest median_s=0.007672 n=10
LATENCY /formular median_s=0.000907 n=10
LATENCY /bestellung median_s=0.000804 n=10
Seccomp:	2
Seccomp_filters:	24
NRestarts=0
Result=success
{"time":"2026-10-12T10:41:47.559763511Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}
{"time":"2026-10-12T10:41:47.637560803Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}
{"time":"2026-10-12T10:41:47.694508553Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}
{"time":"2026-10-12T10:41:47.745387636Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}
{"time":"2026-10-12T10:41:47.794530595Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}
```

### R2, arm64: `mdwe-control.txt`

Platform linux/arm64 (native), base image debian:bookworm-slim. Command: the `mdwe-control` step of `mdwe`.

```text
PASS loaded bestellung
PASS loaded jahreszahl
PASS loaded kontaktformular
PASS loaded nicht-gefunden
PASS loaded suche
PASS no-load-failures
PASS running bestellung
PASS running jahreszahl
PASS running kontaktformular
PASS running nicht-gefunden
PASS running suche
PASS invoke jahreszahl
PASS invoke kontaktformular
PASS invoke bestellung
PASS invoke suche
PASS invoke nicht-gefunden
PASS stable
engine: compiler (8 anonymous executable mappings)
# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests
LATENCY /plugintest median_s=0.002179 n=10
LATENCY /suche?q=Plugintest median_s=0.002084 n=10
LATENCY /formular median_s=0.000260 n=10
LATENCY /bestellung median_s=0.000209 n=10
Seccomp:	2
Seccomp_filters:	22
NRestarts=0
Result=success
{"time":"2026-10-12T10:41:50.982459638Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}
{"time":"2026-10-12T10:41:51.75384518Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}
{"time":"2026-10-12T10:41:52.292437764Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}
{"time":"2026-10-12T10:41:52.81347993Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}
{"time":"2026-10-12T10:41:53.309832875Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}
```

### R2, arm64: `verdict.txt`

Platform linux/arm64 (native), base image debian:bookworm-slim. Command: the `verdict` step of `mdwe`.

```text
MDWE VERDICT: SETTLED - MemoryDenyWriteExecute=true is enforced here (canary: enforced), all five plugins loaded and ran on wazero's interpreter fallback (17/17 assertions), the control with the directive off ran on the compiler engine (17/17), and the unit stays unchanged
```

### R2, arm64: `versions.txt`

Platform linux/arm64 (native), base image debian:bookworm-slim. Command: written by `up`.

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit aarch64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```
