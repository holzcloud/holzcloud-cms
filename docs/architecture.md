# Architecture

How the code is laid out, and what it is built from.

```
cmd/holzcloud/            Entry point, route wiring, middleware, embedded assets
internal/
  config/                 Environment-based configuration
  db/                     SQLite dual pool (single writer), WAL, goose migrations
  auth/                   Argon2id, SCS sessions, CSRF, TOTP, middleware
  domain/                 Websites, domains, host resolver
  page/                   Pages, Markdown pipeline, slugs, versions
  block/  field/          The block editor and the websites' own fields
  admin/  public/         Admin handlers and the public site
  template/  tmplmgr/     Template loading and template-archive upload
  media/  album/  menu/    Media library, albums, menus
  shop/  payrexx/         Products, orders, payment
  bundle/  export/  wxr/  Website archives, static export, WordPress import
  ai/                     The MCP endpoint, keys and OAuth
  i18n/                   Admin translations, on disk and embedded
plugins/                  Bundled plugins, each its own Go module
sites/                    Complete example websites as readable source
tools/                    mkbundle, i18n and the other checks CI runs
```

| Component | Choice | Why |
|---|---|---|
| Language | Go 1.26 | Single binary, standard library first |
| Database | SQLite via `modernc.org/sqlite` | Pure Go — a static binary with no C toolchain |
| Sessions | `alexedwards/scs/v2` | Server-side sessions in SQLite |
| CSRF | `gorilla/csrf` | Middleware, integrated with htmx headers |
| Migrations | `pressly/goose/v3` | Embedded SQL, run at startup |
| Markdown | `yuin/goldmark` | CommonMark compliant |
| Sanitiser | `microcosm-cc/bluemonday` | Prevents XSS from user content |
| Frontend | htmx 2.0 and plain CSS | No build step, no npm, no bundler |
