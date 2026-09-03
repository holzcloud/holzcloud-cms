---
phase: 05-admin-polish-users-deployment
plan: 03
subsystem: deployment
tags: [systemd, caddy, backup, deploy-guide, pi5]
dependency_graph:
  requires: [05-02]
  provides: [production-deployment]
  affects: []
tech_stack:
  added: [systemd, caddy]
  patterns: [sqlite3-backup, security-hardening]
key_files:
  created:
    - deploy/holzcloud.service
    - deploy/Caddyfile.example
    - deploy/backup.sh
    - deploy/DEPLOY.md
  modified: []
decisions:
  - "14 security hardening flags in systemd unit (beyond the 8 minimum)"
  - "Backup dir permissions 700 to protect password hashes"
  - "USB SSD recommendation prominently placed before build section"
metrics:
  duration: 66s
  completed: "2026-04-14"
  tasks_completed: 1
  tasks_total: 1
  files_created: 4
  files_modified: 0
---

# Phase 05 Plan 03: Deployment Configuration Summary

Production deployment files for Raspberry Pi 5: hardened systemd unit, Caddy reverse proxy with automatic HTTPS, WAL-safe SQLite backup script, and comprehensive setup guide.

## Task Results

### Task 1: Deployment files — systemd, Caddy, backup, guide

**Commit:** `920921b`

Created all four files in `deploy/`:

- **holzcloud.service** — systemd unit with 14 security hardening flags including `NoNewPrivileges=true`, `ProtectSystem=strict`, `ReadWritePaths` restricted to data dir, `MemoryDenyWriteExecute`, `PrivateDevices`, and more. Restart on failure with 5s delay.
- **Caddyfile.example** — Reverse proxy to localhost:8080 with automatic HTTPS. Includes commented second domain block and Caddy installation instructions.
- **backup.sh** — Uses `sqlite3 .backup` (WAL-safe, not raw file copy). Backs up media and templates via rsync. Timestamped directories with 700 permissions. Includes retention/pruning comments.
- **DEPLOY.md** — Full lifecycle guide: build, transfer, install, systemd setup, Caddy setup, environment variables, first run, backup cron, updating, and troubleshooting. USB SSD recommendation included prominently.

## Deviations from Plan

None — plan executed exactly as written.

## Threat Mitigations Applied

| Threat | Mitigation |
|--------|-----------|
| T-05-08 Privilege escalation | 14 systemd security flags, dedicated non-login user, restricted write paths |
| T-05-09 Backup info disclosure | Backup dir created with 700 permissions, documented in DEPLOY.md |
| T-05-10 Backup corruption | Uses `sqlite3 .backup` command which handles WAL correctly |

## Known Stubs

None.

## Self-Check: PASSED
