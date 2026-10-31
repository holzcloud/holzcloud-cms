---
phase: quick-260928-f2n
plan: 01
subsystem: api
tags: [mcp, ai, media, upload, go]
status: complete

requires: []
provides:
  - "create_upload_link MCP tool, registered right after upload_media"
  - "PUT/POST /ai/upload/<secret> mounted in the production router next to /ai"
  - "aiConnection helper building /ai and the upload handler from one ai.Deps"
  - "Router-level and package-level tests for every boundary of the upload link"
  - "CHANGELOG 0.0.5 entry and docs/ai-access.md section on upload links"
affects: [ai, media, router, docs, release-0.0.5]

actuals:
  tokens: 12700
  tasks: 3
  commits: 3
plan_head_before: 5c9c1a63b129c24a78e24e5909ff39ee96f20624
plan_head_after: d83c2ad830ffa6bf67ec99727926766ef681b5f2

tech-stack:
  added: []
  patterns:
    - "One Deps value feeds both an MCP tool and the HTTP handler that completes it (aiConnection)"
    - "Method-less prefix mount so the handler answers 405 itself instead of the public catch-all"

key-files:
  created:
    - internal/ai/upload_link.go
    - internal/ai/upload_link_test.go
    - cmd/holzcloud/upload_link_test.go
  modified:
    - internal/ai/tools.go
    - internal/ai/tools_media.go
    - cmd/holzcloud/main.go
    - cmd/holzcloud/main_test.go
    - CHANGELOG.md
    - docs/ai-access.md

key-decisions:
  - "upload_link.go and the storeUpload extraction were kept unchanged: no test revealed a defect"
  - "ai.UploadPath mounted without a method pattern so a GET gets the handler's JSON 405, not the public site's 404 page"
  - "upload_media description now steers assistants to create_upload_link for anything above a few kilobytes"

requirements-completed: [QUICK-260928-f2n]

metrics:
  duration: 39min
  completed: 2026-09-28
---

# Quick 260928-f2n: MCP upload link for large media files Summary

**`create_upload_link` hands an assistant a one-time `/ai/upload/<secret>` address (256-bit secret stored only as a hash, valid once for 15 minutes, with the key re-checked when the file arrives). The raw file is PUT there and goes through the same `storeUpload` → admin `OpUploadMedia` intake as `upload_media`.**

## Performance

- **Duration:** about 39 min. Most of it was test runtime on a heavily loaded shared Raspberry Pi.
- **Tasks:** 3/3
- **Files:** 9 (3 created, 6 modified)

## Accomplishments

- `Deps.Uploads` added. `create_upload_link` is registered after `upload_media`, and the `upload_media` description now points to it for screenshots and photographs.
- `cmd/holzcloud/main.go`: `aiConnection(cfg, deps)` builds the MCP server and `ai.UploadHandler` from one Deps value carrying one `*ai.Uploads`. `routerDeps.aiUpload` is added, and `newRouter` mounts `ai.UploadPath` next to `/ai`, with a comment on why it sits outside CSRF and session and why the mount has no method.
- Tracer test `TestAnUploadLinkGoesThroughTheRouter`: the link is opened over `/ai` with a real content key. A real 64x48 PNG is PUT through the production router into the real `admin.Handler.OpUploadMedia` and answers 201 with alt text "Der Hof", `mime` image/png, and the media row on the right website. Reusing the link gives 404 (handler JSON, not the public HTML page), and a GET gives 405 with `Allow: PUT, POST`.
- 13 package tests in `internal/ai/upload_link_test.go` cover:
  - issuing over http and https
  - which keys see the tool in tools/list
  - a 201 with the exact bytes, alt text and activity entry by "KI: schreibend"
  - POST accepted
  - a used link (404) and an expired link (404)
  - refusals for another website and for a read-only key
  - a key revoked or made read-only after the link was opened (403)
  - 413 with and without a Content-Length header
  - a GET answering 405 without using up the link
  - an unknown link (404 JSON)
  - the limit of 200 open links, the sweep of expired links, and storage by hash only
  - "not available" when the installation cannot upload
- CHANGELOG `## 0.0.5 — 2026-09-28` (German, Swiss spelling) and a docs/ai-access.md update covering the files rule, the tool table and a "Large files: upload links" subsection with an answer table.

## Task Commits

1. **Task 1 (tracer): end-to-end wiring plus router test**: `49e1225` (feat). This commit also carries the previously uncommitted `internal/ai/upload_link.go` and the `storeUpload` extraction in `internal/ai/tools_media.go`, as the task brief required.
2. **Task 2: package tests for every boundary**: `9f18f92` (test)
3. **Task 3: CHANGELOG 0.0.5 and docs**: `d83c2ad` (docs)

## Changes to the pre-existing upload_link.go / storeUpload

None. Every test passed against the code as it stood. A spot mutation check confirmed the tests catch real faults:
- If `take` no longer deletes the ticket, `TestAnUploadLinkWorksOnce` fails.
- If `stillValid` is skipped, `TestAnUploadLinkDiesWithItsKey` fails.

Removing the Content-Length pre-check is *not* caught, because `MaxBytesReader` still answers 413 and the stand-in still receives nothing. The pre-check only saves reading the body. It is not a separate boundary.

## TDD note

Task 2 is marked `tdd="true"`, but the implementation already existed as the starting point of this task, so there could be no RED phase. The tests went green on their first run. The mutation check above stands in for RED evidence.

## Deviations from Plan

None in the code. The plan was executed as written.

## Gate results

- `gofmt -l .`: clean
- `go mod tidy`: no diff to go.mod or go.sum
- `go vet ./...`: clean
- `go run ./tools/english`: "no German in the Go source outside the catalogues"
- `go run ./tools/cites -check`: 0 citations, 0 unresolvable
- `go run ./tools/i18n`: 1946 strings. de, es, fr and it each report 1946 translated, 0 open, 0 orphaned.
- `go test ./...`: 52 packages passed on the full run. `internal/admin` hit Go's default 10-minute test timeout while it was in the middle of `TestASavedViewIsRememberedCorrectedAndForgotten`, on a shared machine with load around 10 on 4 cores. It is unrelated to this change: no file in internal/admin was touched. Rerun alone with `-timeout 40m`, it passed (`ok internal/admin 1210s`). Result: 53/53 packages pass and there are 0 failing tests.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model. The new surface, `/ai/upload/`, is exactly what T-q260928-01..08 describe.

## Self-Check: PASSED

- FOUND: internal/ai/upload_link.go, internal/ai/upload_link_test.go, cmd/holzcloud/upload_link_test.go
- FOUND commits: 49e1225, 9f18f92, d83c2ad
