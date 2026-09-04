# Testing Patterns

**Analysis Date:** 2026-09-04

*Corrected surgically against the working tree in Phase 6 (plan `06-04`) — not regenerated.*

## Test Framework

**Runner:**
- Go stdlib `testing` (Go 1.26.6, `go.mod`)
- No config file — no testify, no gomock, no ginkgo. Zero test dependencies.

**Assertion Library:**
- None. Hand-written `if got != want { t.Errorf(...) }` comparisons.
- `errors.Is` for sentinel comparison, `strings.Contains` for rendered HTML.

**Run Commands:**
```bash
go test ./...                      # Run all tests
go test ./internal/page/           # One package
go test -run TestUpdatePage ./...  # One test
go test -race ./...                # Race detector (needs CGO_ENABLED=1)
go test -cover ./...               # Coverage (not enforced anywhere)
go test ./internal/i18n/           # Translation catalogue guard
```

CI (`.github/workflows/ci.yml`) runs `gofmt -l`, `go mod tidy` diff, `go vet ./...`, build, then `go test ./...` with `CGO_ENABLED=0`.
`.github/workflows/security.yml` runs weekly: `go mod verify`, `go list -m -u all`, and `go test -race ./...` with `CGO_ENABLED=1` — the one place the race detector runs.

## Test File Organization

**Location:**
- Co-located with the code under test, same package (internal tests, not `_test` packages). `internal/page/store.go` ↔ `internal/page/store_test.go`.

**Naming:**
- `<subject>_test.go` mirroring the source file
- `<feature>_e2e_test.go` for full-chain tests: `internal/plugin/sdk_e2e_test.go`, `internal/public/suche_e2e_test.go`, `internal/public/formular_e2e_test.go`, `internal/template/traversal_e2e_test.go`

**Structure:**
```
internal/page/
├── store.go
├── store_test.go
├── access.go
├── access_test.go
└── ...
internal/plugin/testdata/       # the only testdata dir: echo.wasm + source
```

80 test files, 566 `Test*` functions. Every `internal/` package has tests except `internal/branding/`.

## Test Structure

**Suite Organization:** Flat top-level functions, no suites. Test names are full English (or German, in plugin/public packages) sentences describing the guarantee:

```go
// Two editors open the same page. The second save must be refused rather than
// silently dropping the first editor's paragraphs.
func TestUpdatePageRefusesAStaleVersion(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Titel", "titel", "erster text", "draft")

	stale := p.Version
	// ... first save succeeds ...

	err := s.UpdatePage(ctx, p.ID, PageUpdate{ /* ... */ ExpectedVersion: stale})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("second save with a stale version: got %v, want ErrConflict", err)
	}
}
```
`internal/page/store_test.go:51`

**Patterns:**
- Every test opens its own environment via a package-local `newTestX(t)` helper; no shared global state
- `t.Helper()` in every helper; `t.TempDir()` for filesystem isolation; `t.Cleanup(...)` for teardown
- `t.Fatalf` for setup failures and preconditions, `t.Errorf` for assertions that should keep the test running
- Failure messages state what was got and what was wanted, in prose
- Each test carries a comment naming the real scenario it defends
- Table-driven tests are rare (2 files) and subtests rarer (9 `t.Run` sites) — the house style is one named function per behaviour
- No `t.Parallel()` anywhere: tests own real SQLite files, so serial execution is deliberate

## Mocking

**Framework:** None.

**Philosophy — prefer the real thing.** From `internal/page/store_test.go:13`:
> "newTestStore opens a real migrated SQLite database. The interesting parts of the store are transactions and constraints, which a fake would not have."

The store fixture:
```go
func newTestStore(t *testing.T) (*Store, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil { t.Fatalf("db.Open: %v", err) }
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	// seed a website, return its id
}
```

The handler fixture wires the real admin templates off disk so a template that drifts from its data struct fails in `go test`, not in the browser (`internal/admin/page_handler_test.go:31`):
```go
templates, err := web.ParseAdminTemplates(os.DirFS("../../cmd/holzcloud/templates/admin"))
sm := scs.New()
sm.Store = memstore.New()   // the one substitution: in-memory session store
```

**What to Mock:**
- Session storage — `scs/v2/memstore` instead of the SQLite session table
- Genuinely unreachable collaborators, via a tiny hand-written struct. The single example in the tree is `type stubResolver struct{ slug string }` (`internal/template/loader_test.go:52`)
- Optional subsystems are passed as `nil` (plugins, mail, AI tokens) — nil means "this build has no such screen"

**What NOT to Mock:**
- The database. Always a real migrated SQLite file in `t.TempDir()`.
- Templates. Parsed from the real `cmd/holzcloud/templates/` tree.
- HTTP. `net/http/httptest` throughout (22 test files) — `httptest.NewRequest` / `httptest.NewRecorder`, driven through the real middleware.
- The WASM plugin runtime. `internal/plugin/runtime_test.go` and the `*_e2e_test.go` files instantiate wazero and run real modules.

## Fixtures and Factories

**Test Data:** Small typed seed helpers per package, always taking `*testing.T` first and failing fast:
```go
func seed(t *testing.T, s *Store, websiteID int64, title, slug, body, status string) *Page {
	t.Helper()
	p, err := s.CreatePage(context.Background(), PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug,
		Markdown: body, HTML: "<p>" + body + "</p>", Status: status,
	})
	if err != nil { t.Fatalf("CreatePage: %v", err) }
	return p
}
```
`internal/page/store_test.go:36`. Siblings: `seedWebsite`, `seedPage`, `loadPlugin` in `internal/public/suche_e2e_test.go`.

**Location:**
- Helpers live at the top of the `_test.go` file that needs them, or in the package's principal test file (`newTestAdmin` in `internal/admin/page_handler_test.go`, `newTestHandler` in `internal/public/handler_test.go`)
- Binary fixtures: `internal/plugin/testdata/` (`echo.wasm` plus its source and a README)
- Built plugin artifacts under `plugins/*/plugin.wasm` are treated as optional fixtures

**Skipping on missing artifacts** — the only sanctioned skip:
```go
modul, err := os.ReadFile("../../plugins/suche/plugin.wasm")
if err != nil {
	t.Skipf("plugins/suche/plugin.wasm fehlt: %v", err)
}
```
`internal/public/suche_e2e_test.go:26`. Keeps `go test ./...` green on a checkout where the WASM plugins have not been built, while the e2e tests still run in a full build.

## Coverage

**Requirements:** None enforced. No coverage gate in CI, no `codecov`, no threshold.

**View Coverage:**
```bash
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

Coverage is steered by risk rather than percentage: the densest suites sit on security and data-integrity boundaries — `internal/tmplmgr` (zip-slip, oversize, forged sizes, external subresources), `internal/auth` (password, session, rate limit, elevate, middleware), `internal/page` (version conflicts, revisions, draft leakage, access).

## Test Types

**Unit Tests:**
- Pure logic with no I/O: `internal/design/tokens_test.go` (colour/CSS sanitising), `internal/config/config_test.go`, `internal/locale/locale_test.go`, `internal/web/clientip_test.go`
- Store tests, which are unit-shaped but hit a real SQLite file

**Integration Tests:**
- Handler tests: real DB + real templates + real session manager, one handler driven through `serve(t, h, sm, fn, req)` (`internal/admin/page_handler_test.go:74`)
- Routing/authorisation tests over the real mux: `cmd/holzcloud/main_test.go` — `TestRouteAuthorization`, `TestAdminRoutesRequireASession`, `TestAdminResponsesCarrySecurityHeaders`, `TestUnknownHostGets404`

**E2E Tests:**
- `*_e2e_test.go` run a real compiled WASM plugin through the real wazero runtime, the real middleware chain and the real theme, asserting on the HTTP response. See `internal/public/suche_e2e_test.go`, `internal/public/formular_e2e_test.go`, `internal/plugin/hofladen_e2e_test.go`, `internal/plugin/sdk_e2e_test.go`.
- No browser driver. `.playwright-mcp/` is scratch output, not a test suite.

**Not present:** benchmarks, fuzz targets, golden-file tests.

## Common Patterns

**HTTP request through real middleware:**
```go
rec := httptest.NewRecorder()
req := httptest.NewRequest("GET", "http://beispiel.test/suche?q=Wolle", nil)
req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	t.Error("die Anfrage lief am Plugin vorbei bis zum Kern")
})).ServeHTTP(rec, req)

if rec.Code != http.StatusOK {
	t.Fatalf("Status %d, Rumpf: %s", rec.Code, rec.Body.String())
}
```
`internal/public/suche_e2e_test.go:48` — the fall-through handler doubles as an assertion that the request never reached the core.

**Session-backed handler invocation:** wrap the call in `sm.LoadAndSave` so the handler sees a live session, exactly as the production middleware chain provides it (`internal/admin/page_handler_test.go:74`).

**Error Testing:**
```go
if !errors.Is(err, ErrConflict) {
	t.Fatalf("second save with a stale version: got %v, want ErrConflict", err)
}
```
Always `errors.Is` against the package sentinel; never a message-string match.

**Async Testing:** No goroutine-coordination helpers, no `time.Sleep` polling. Background work (mail queue, jobs) is tested by calling the worker step directly — `internal/mail/queue_test.go`.

**Assert on effects, not calls:** after asserting the error, tests re-read the row and check the surviving state (`after.ContentMarkdown != "editor A"`), rather than verifying that a method was invoked.

## Writing a New Test

1. Put it in `<subject>_test.go` beside the code, same package.
2. Reuse the package's `newTestX(t)` fixture; add a `seedX` helper if you need new data.
3. Name the function after the guarantee, in a full sentence, and write a comment giving the real-world scenario.
4. Use the real database and real templates; substitute only session storage.
5. Assert with `errors.Is` for errors and by re-reading state for effects.
6. Run `gofmt -l .`, `go vet ./...`, `go test ./...` before committing — CI enforces all three.

---

*Testing analysis: 2026-09-04*
