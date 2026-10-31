---
phase: 30-datenschutz-ohne-banner
plan: 01
subsystem: logging
tags: [privacy, access-log, journald, ip-truncation]
requires: []
provides:
  - web.TruncateIP, LogIPv4Prefix (24), LogIPv6Prefix (48), LogRetentionDays (14), UnknownIP
  - deploy/journald-holzcloud.conf
  - logSink test helper (cmd/holzcloud/access_log_ip_test.go)
affects: [30-04, 30-06]
key-files:
  created:
    - internal/web/logip.go
    - internal/web/logip_test.go
    - cmd/holzcloud/access_log_ip_test.go
    - cmd/holzcloud/deploy_journald_test.go
    - deploy/journald-holzcloud.conf
  modified:
    - internal/web/logging.go
    - docs/deployment.md
requirements: [DSV-01, DSV-02]
status: complete
commits: 3
plan_head_before: b0995b91fe3a3130ac11e2328a9d27e010531e8f
plan_head_after: 2d80ef97e2816f8fdb6b4d1c46eb638f969f58d2
actuals:
  tasks: 3
  commits: 3
---

# Phase 30 Plan 01: Request log without full addresses Summary

The request log writes the client address cut to /24 (IPv4) or /48 (IPv6), non-addresses as `unknown`, and a journald drop-in enforces 14 days of retention, tied by test to the constant the privacy text will use.

## Tasks

| Task | Commit | What |
|------|--------|------|
| 1 (tracer) | a7bfa13 | `TruncateIP` + `AccessLog` uses it + end-to-end test through the real router |
| 2 | 3d2a548 | Table, never-echoes, idempotence and "ClientIP stays whole" tests |
| 3 | 2d80ef9 | `deploy/journald-holzcloud.conf`, tying test, deployment page row and "Log retention." paragraph |

## Red runs (proof the tests pin what they claim)

- Task 1: `AccessLog` logging the resolved address as it was: all three subtests FAIL; restored.
- Task 2: `LogIPv4Prefix` = 32: `TestTruncateIP` FAIL (203.0.113.77 not cut, 127.0.0.1, 255.255.255.255, ::ffff:203.0.113.77 and the port form). `LogIPv6Prefix` = 64: `TestTruncateIP` FAIL (the two IPv6 cases keep `1234`). Both restored to 24 and 48.

## Check command for the owner on a real host

```
sudo install -D -m 0644 deploy/journald-holzcloud.conf /etc/systemd/journald.conf.d/holzcloud.conf
sudo systemctl restart systemd-journald
systemd-analyze cat-config systemd/journald.conf
```

The last command must list `MaxRetentionSec=14d` and `MaxFileSec=1d`. Not played on a real host (no systemd on the development machine).

## Other log call sites that still write a full address (T-30-04, accepted, D-02)

- `internal/admin/login.go:45` login rate-limit warning (`ip`)
- `internal/admin/oidc.go:250` OpenID Connect refusal (`ip`)
- `internal/admin/forwardauth.go:884` single-sign-on refusal (`ip`)
- `internal/web/forwardauth.go:165` forward-auth wrong-secret warning (`peer`, `r.RemoteAddr`)

## Deviations from Plan

**1. Frozen gate red at baseline (not caused by this plan).** `bash .planning/phases/28-the-promise-in-writing/28-check.sh frozen` fails Rules A, B and D, and the pinned diff `git diff --quiet 05fc076 -- deploy/DEPLOY.md deploy/holzcloud.service` exits non-zero. Cause: commit `05fc076` is not an ancestor of this branch and `deploy/DEPLOY.md` differs from it by 39 deleted lines; no commit after b0995b9 touches `deploy/` other than adding `journald-holzcloud.conf` (`git diff b0995b9 HEAD -- deploy` lists only that addition). DEPLOY.md, holzcloud.service and every bound file are byte for byte as at b0995b9. The plan's verification premise ("baseline prints an ok: frozen line") does not hold in this branch history; the owner/orchestrator should check the gate on the branch where the evidence commits live. Verified instead: `git diff --quiet e2513b1 -- go.mod go.sum`, `clientip.go` and `internal/activity` untouched.

**2. `.planning/` is gitignored (commit b0995b9).** This SUMMARY is written to the phase directory of the worktree but cannot be committed without `git add -f`, which is forbidden. A copy of the main checkout's `.planning/` was placed in the worktree (ignored) to run the gate.

**3. AccessLog shape.** To satisfy the acceptance criteria literally, `TruncateIP` is called inside the `slog.String("ip", TruncateIP(ip))` attribute rather than on a reassigned variable.

## Verification

- `go test ./internal/web/ ./cmd/holzcloud/ -count=1`: ok
- `go build ./...`, `go vet`, `gofmt -l`: clean
- `go run ./tools/i18n`: 0 open, 0 orphaned (no operator-facing strings added)
- `go run ./tools/english`: 0 hits for the path pattern

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

All created files exist and commits a7bfa13, 3d2a548, 2d80ef9 are on the branch.
