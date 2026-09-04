# Codebase Structure

**Analysis Date:** 2026-09-04

*Corrected surgically against the working tree in Phase 6 (plan `06-04`) — not regenerated.*

## Directory Layout

```
Holzcloud_CMS/
├── cmd/holzcloud/            # Entry point, route table, CLI, embedded assets/templates
│   ├── main.go               # main() + newRouter() — all wiring and routes
│   ├── cli.go                # Subcommands (user, backup, migrate, compact, check, …)
│   ├── main_test.go          # Route-table test
│   ├── assets/               # admin.css, bausteine.css, htmx.min.js, favicon.svg (embed)
│   └── templates/            # admin/ + public/<theme>/ Go templates (embed)
├── internal/                 # All application packages (38 dirs)
│   └── db/migrations/        # 45 goose .sql files, to 00045_pages_locale_unique (embed)
│   └── i18n/locales/         # de-CH, en, es, fr, fr-CH, it, it-CH JSON catalogs (embed)
├── sdk/                      # Public Go SDK for writing WASM plugins (own go.mod)
├── plugins/                  # First-party plugins, each its own module + .wasm
├── sites/                    # Example site bundles (holzcloud.json + media/)
├── tools/                    # Build-time helpers (i18n extraction, mkbundle)
├── deploy/                   # systemd units, backup/restore scripts, Caddyfile example
├── build/                    # Dev seed program and prebuilt example bundles
├── docs/                     # Working notes
├── data/                     # Runtime state — gitignored (DB, WAL, csrf.key, media, logs)
├── Dockerfile
├── CLAUDE.md                 # Hard stack constraints and conventions
└── README.md
```

## Directory Purposes

**`cmd/holzcloud/`:**
- Purpose: the only `package main`; composition root
- Contains: `main.go` (startup + `newRouter`), `cli.go`, embedded `assets/` and `templates/`
- Key files: `cmd/holzcloud/main.go`, `cmd/holzcloud/cli.go`

**`internal/` — one package per concept:**
- Infrastructure: `config/`, `db/`, `web/`, `jobs/`, `mail/`, `outbox/`, `auth/`, `totp/`, `sharelink/`, `activity/`, `textdiff/`
- Content model: `page/`, `menu/`, `media/`, `snippet/`, `term/`, `field/`, `kind/`, `block/`, `structured/`
- Commerce: `shop/`, `money/`, `payrexx/`
- Tenancy & presentation: `domain/`, `template/`, `tmplmgr/`, `tmplspec/`, `design/`, `branding/`, `i18n/`, `locale/`
- Interfaces: `admin/` (49 non-test files, largest), `public/`, `ai/` (MCP), `plugin/` (wazero)
- Data movement: `bundle/` (export/import), `wxr/` (WordPress import)
- Users: `user/`

**`sdk/`:**
- Purpose: importable ABI for third-party plugin authors; separate `go.mod`
- Key files: `sdk/plugin.go`, `sdk/abi_wasm.go`, `sdk/abi_other.go`

**`plugins/`:**
- Purpose: reference/first-party plugins (`bestellung`, `jahreszahl`, `kontaktformular`, `nicht-gefunden`, `suche`)
- Each contains: `go.mod`, `main.go`, `plugin.json`, built `plugin.wasm`; four of the five also ship a distributable `.zip`, while `kontaktformular` ships a `migrations/` directory instead

**`data/`:**
- Purpose: all runtime state — `holzcloud.sqlite` (+ `-wal`/`-shm`), `csrf.key`, logs, media, user-uploaded templates, i18n and branding overrides
- Generated: yes. Committed: no.

## Key File Locations

**Entry Points:**
- `cmd/holzcloud/main.go`: startup, DI, jobs, HTTP server
- `cmd/holzcloud/main.go` (`newRouter`): complete route table
- `cmd/holzcloud/cli.go`: subcommands

**Configuration:**
- `internal/config/config.go`: env vars (`HOLZCLOUD_*`), validation, logger
- `Dockerfile`, `deploy/holzcloud.service`: deployment config

**Core Logic:**
- `internal/db/db.go`: dual-pool SQLite, migrations, DSN pragmas
- `internal/domain/resolver.go`: host → website multi-tenancy
- `internal/public/handler.go`: public page rendering
- `internal/admin/handler.go`: admin handler struct and `ErrHandler`
- `internal/template/loader.go`: public template resolution and cache
- `internal/auth/middleware.go`: auth/role/website guards
- `internal/plugin/runtime.go`: wasm sandbox

**Templates & Assets:**
- `cmd/holzcloud/templates/admin/*.html`: admin screens (`base.html` is the shell)
- `cmd/holzcloud/templates/public/<theme>/`: `default`, `holzcloud`, `journal`, `magazine`, `midnight`, `rudel`, `schlicht`, `weide`
- `cmd/holzcloud/assets/`: `admin.css`, `bausteine.css`, `htmx.min.js`, `favicon.svg`

**Testing:**
- Co-located `*_test.go` beside implementation in every package
- End-to-end style: `internal/plugin/hofladen_e2e_test.go`, `internal/plugin/sdk_e2e_test.go`, `internal/public/formular_e2e_test.go`, `internal/template/traversal_e2e_test.go`
- Fixtures: `internal/plugin/testdata/echo/`

**CI/CD:**
- `.github/workflows/ci.yml`, `image.yml`, `release.yml`, `security.yml`

## Naming Conventions

**Files:**
- Go: `snake_case.go`, one concept per file (`store.go`, `model.go`/`models.go`, `handler.go`, `render.go`, `upload.go`)
- Tests: `<file>_test.go` next to the source; broader ones `<feature>_e2e_test.go`
- Templates: `snake_case.html` (`page_form.html`, `two_factor_setup.html`)
- Migrations: `000NN_snake_case.sql`, goose-annotated, zero-padded to five digits (next is `00039_`)

**Directories:**
- Short, lowercase, single word: `page`, `menu`, `tmplmgr`, `sharelink`

**Symbols:**
- Handlers: `func (h *Handler) HandleEntityAction(w, r) error`
- Constructors: `NewStore`, `NewHandler`, `NewResolver`, `NewLoader`
- Optional deps set after construction: `SetPlugins`, `SetMail`, `SetAITokens`, `SetTermStore`
- SQL: `snake_case` tables/columns; CSS: `kebab-case`
- German is used for user-facing route segments and some identifiers (`/vorschau/`, `/freischalten`, `Felder`, `Uebersetzungen`) — match the surrounding file.

## Where to Add New Code

**New content concept (e.g. a new entity):**
- Package: `internal/<concept>/` with `model.go` + `store.go` (+ `render.go` if it renders)
- Migration: `internal/db/migrations/000NN_<concept>.sql`
- Admin screens: `internal/admin/<concept>.go` + `cmd/holzcloud/templates/admin/<concept>_list.html`
- Wiring: construct the store in `cmd/holzcloud/main.go`, add it to `routerDeps` and to `admin.NewHandler` (or a `SetX` setter if optional)
- Routes: register in `newRouter` under the correct middleware chain

**New route:**
- `cmd/holzcloud/main.go` `newRouter` only — admin routes on `adminProtectedMux` (or `adminPublicMux` for unauthenticated), public routes on `publicMux`
- Add a case to the route-table test in `cmd/holzcloud/main_test.go`

**New admin screen:**
- Handler: `internal/admin/<name>.go`, wrapped in `adminHandler.ErrHandler`
- Template: `cmd/holzcloud/templates/admin/<name>.html` composing `base.html`
- Nav entry: `internal/web/layoutdata.go` / nav middleware in `cmd/holzcloud/main.go`

**New public theme:**
- `cmd/holzcloud/templates/public/<slug>/` with at least `page.html`; register in `tmpl.BuiltinTemplates` (`internal/template/`), which seeds it into the DB at startup

**New periodic task:**
- Add a `jobs.Job{Name, Every, Fn}` to the `jobs.New(...)` list in `cmd/holzcloud/main.go`

**New CLI subcommand:**
- Add a `case` in `runCLI` (`cmd/holzcloud/cli.go`) and a line to its help text

**New plugin:**
- `plugins/<name>/` with its own `go.mod` importing `github.com/holzcloud/holzcloud-cms/sdk`, plus `plugin.json`; build to `plugin.wasm` and zip

**New translation string:**
- `internal/i18n/locales/*.json` for every locale; extract with `tools/i18n`

**Shared helpers:**
- `internal/web/` (HTTP-adjacent: forms, flash, headers, layout data, asset helpers)

## Special Directories

**`data/`:** runtime state, generated, not committed (mode 0700).
**`cmd/holzcloud/assets/` and `cmd/holzcloud/templates/`:** committed, embedded via `//go:embed assets templates` in `cmd/holzcloud/main.go`; vendored third-party files documented in `cmd/holzcloud/assets/VENDOR.md`.
**`internal/db/migrations/`:** committed, embedded, append-only — never edit an applied migration.
**`plugins/*/plugin.wasm` and `*.zip`:** committed build artifacts.
**`sdk/`, `plugins/*/`:** separate Go modules, excluded from the root module build.
**`.playwright-mcp/`:** browser-tooling scratch output, not part of the build.

---

*Structure analysis: 2026-09-04*
