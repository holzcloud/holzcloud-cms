# Phase 04: Templates + Menus + Media - Research

**Researched:** 2026-04-14
**Domain:** File upload security, hierarchical data, media serving in Go
**Confidence:** HIGH

## Summary

This phase adds three distinct subsystems: template zip upload with security validation, hierarchical menu management, and media file upload/serving. All use Go stdlib exclusively -- `archive/zip` for extraction, `net/http.DetectContentType` for magic byte MIME sniffing, `os` for disk operations. No new dependencies needed.

The critical integration point is the template loader (`internal/template/loader.go`), which currently resolves templates by `data/templates/{websiteID}/` but must change to resolve via the active template's slug from the DB: `data/templates/{template_slug}/`. The loader already has `InvalidateTemplateCache()` and a stub `MenuItem` type ready for Phase 4.

**Primary recommendation:** Split into 3 plans: (1) migrations + stores for all three subsystems, (2) template upload + menu admin handlers, (3) media handlers + public menu rendering + loader integration.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Zip-slip prevention via path clean + prefix check; reject entire upload on escape
- D-02: Extension allow-list: .html, .css, .js, .svg, .png, .jpg, .jpeg, .gif, .webp, .ico, .woff, .woff2, .ttf
- D-03: 10MB template size cap (env var HOLZCLOUD_MAX_TEMPLATE_SIZE)
- D-04: Template must contain layout.html and page.html at root; optional home.html, 404.html, assets/
- D-05: Templates table + website_templates join table for per-website activation
- D-06: Templates stored at data/templates/{template_slug}/, metadata in DB
- D-07: Admin at /admin/templates with list, upload, activate/deactivate, delete
- D-08: Delete removes disk + DB; cannot delete active template
- D-09: Menus table with (website_id, location_key) unique constraint
- D-10: Menu items with parent_id self-reference and sort_order
- D-11: Menu editor at /admin/websites/{id}/menus with tree view
- D-12: Up/down reorder buttons, swap sort_order values
- D-13: Template helper {{menu .Menus "main"}} outputs nested ul/li, max 3 levels
- D-14: Menu item types: page link, external URL, custom text (no link)
- D-15: Media stored in data/media/{website_id}/ with UUID-prefixed filenames
- D-16: MIME allow-list validated by magic bytes, not Content-Type header
- D-17: 5MB per-file media limit (env var HOLZCLOUD_MAX_MEDIA_SIZE)
- D-18: Media table stores metadata; file on disk
- D-19: Admin at /admin/websites/{id}/media with grid/list, upload, delete
- D-20: Media served at /media/{website_id}/{filename} with immutable cache headers
- D-21: Delete removes disk + DB

### Claude's Discretion
- No thumbnail generation in v1 (serve originals)
- No menu item icons in v1
- Template validation beyond structure check -- recommend: skip deep HTML parsing
- Media list pagination threshold -- recommend: 50 items before paginating

### Deferred Ideas (OUT OF SCOPE)
- Drag-and-drop menu reordering (V2-10)
- Template preview before activation (V2-12)
- Image thumbnail generation
- Media insertion button in page editor
- Template versioning / rollback
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TPL-01 | Zip upload with zip-slip prevention, extension allow-list, size cap | Go archive/zip + filepath.Clean prefix check; extension map |
| TPL-02 | Activate one template per website for public rendering | website_templates join table; loader reads active slug from DB |
| TPL-03 | Template directory convention (layout.html, page.html, etc.) | Validate after extraction; reject if missing required files |
| TPL-04 | List and delete templates from admin | Store CRUD + os.RemoveAll for disk cleanup |
| MENU-01 | Multiple menus per website with location key | Menus table with UNIQUE(website_id, location_key) |
| MENU-02 | Hierarchical menu items with sort order | parent_id FK + sort_order; recursive CTE for tree query |
| MENU-03 | Reorder via up/down buttons | Swap sort_order of adjacent items in single transaction |
| MENU-04 | Public template renders menu by location key | FuncMap helper "menu" that filters by location and builds nested HTML |
| MEDIA-01 | Upload images/files per website | Multipart handler + UUID filename + disk write |
| MEDIA-02 | MIME validation via magic bytes + size limit | http.DetectContentType on first 512 bytes |
| MEDIA-03 | Serve with correct Content-Type and cache headers | DB lookup for mime_type; Cache-Control immutable |
| MEDIA-04 | Admin list with delete | Store listing + os.Remove for disk cleanup |
</phase_requirements>

## Standard Stack

### Core (already in project)
| Library | Purpose | Why |
|---------|---------|-----|
| `archive/zip` (stdlib) | Extract uploaded zip templates | Standard, handles all zip formats [VERIFIED: Go stdlib] |
| `net/http.DetectContentType` (stdlib) | Magic byte MIME sniffing | Implements mimesniff spec, reads first 512 bytes [VERIFIED: go doc] |
| `path/filepath` (stdlib) | Zip-slip path validation | `filepath.Clean` + `strings.HasPrefix` for escape detection [VERIFIED: Go stdlib] |
| `os` (stdlib) | Disk file operations | Create dirs, write files, remove on delete [VERIFIED: Go stdlib] |
| `crypto/rand` + `encoding/hex` (stdlib) | UUID-like filename prefix | 16 random bytes -> 32 hex chars for collision avoidance [VERIFIED: Go stdlib] |

### No New Dependencies
Everything needed is in Go stdlib. No new `go get` required.

## Architecture Patterns

### New Packages
```
internal/tmplmgr/          # Template upload/store (not "template" -- conflicts with stdlib)
  store.go                 # DB CRUD for templates + website_templates
  upload.go                # Zip extraction, validation, security
  model.go                 # Template, WebsiteTemplate structs
internal/menu/             # Menu system
  store.go                 # DB CRUD for menus + menu_items
  model.go                 # Menu, MenuItem structs
  render.go                # HTML rendering helper for public templates
internal/media/            # Media upload/serve
  store.go                 # DB CRUD for media
  model.go                 # Media struct
  upload.go                # File validation, disk write
```

### Migration: 00005_templates_menus_media.sql
```sql
-- Templates metadata
CREATE TABLE templates (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

-- Per-website template activation
CREATE TABLE website_templates (
  id INTEGER PRIMARY KEY,
  website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
  template_id INTEGER NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
  is_active INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  UNIQUE(website_id, template_id)
) STRICT;

-- Ensure only one active template per website
CREATE UNIQUE INDEX idx_website_active_template 
  ON website_templates(website_id) WHERE is_active = 1;

-- Menus
CREATE TABLE menus (
  id INTEGER PRIMARY KEY,
  website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  location_key TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  UNIQUE(website_id, location_key)
) STRICT;

-- Menu items (hierarchical)
CREATE TABLE menu_items (
  id INTEGER PRIMARY KEY,
  menu_id INTEGER NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
  parent_id INTEGER REFERENCES menu_items(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  item_type TEXT NOT NULL DEFAULT 'url',  -- 'page', 'url', 'custom'
  url TEXT NOT NULL DEFAULT '',
  page_id INTEGER REFERENCES pages(id) ON DELETE SET NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

-- Media files
CREATE TABLE media (
  id INTEGER PRIMARY KEY,
  website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
  filename TEXT NOT NULL,
  original_name TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE INDEX idx_media_website ON media(website_id);
```
[ASSUMED -- schema design follows established project patterns from migrations 00001-00004]

### Pattern: Zip-Slip Prevention
```go
// Source: OWASP zip-slip guidance + Go stdlib
func extractZip(zipPath, destDir string, maxSize int64) error {
    r, err := zip.OpenReader(zipPath)
    if err != nil { return err }
    defer r.Close()

    for _, f := range r.File {
        // Clean the path and check for escape
        cleanName := filepath.Clean(f.Name)
        destPath := filepath.Join(destDir, cleanName)
        
        // CRITICAL: zip-slip prevention
        if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
            return fmt.Errorf("zip-slip detected: %s", f.Name)
        }
        
        // Extension allow-list check
        if !f.FileInfo().IsDir() {
            ext := strings.ToLower(filepath.Ext(cleanName))
            if !allowedExtensions[ext] {
                return fmt.Errorf("disallowed file type: %s", ext)
            }
        }
        
        // Extract...
    }
    return nil
}
```
[VERIFIED: filepath.Clean + HasPrefix is the standard Go zip-slip pattern per OWASP]

### Pattern: Magic Byte MIME Validation
```go
// Source: Go stdlib net/http docs
func validateMIME(file multipart.File) (string, error) {
    buf := make([]byte, 512)
    n, err := file.Read(buf)
    if err != nil && err != io.EOF { return "", err }
    
    detected := http.DetectContentType(buf[:n])
    if !allowedMIME[detected] {
        return "", fmt.Errorf("disallowed MIME type: %s", detected)
    }
    
    // Reset reader for subsequent copy
    if seeker, ok := file.(io.Seeker); ok {
        seeker.Seek(0, io.SeekStart)
    }
    return detected, nil
}
```
[VERIFIED: http.DetectContentType reads first 512 bytes, implements mimesniff spec]

### Pattern: Menu Tree Query (Recursive CTE)
```sql
-- Get all menu items for a website's menu by location, ordered for tree building
SELECT mi.id, mi.menu_id, mi.parent_id, mi.title, mi.item_type, mi.url, 
       mi.page_id, mi.sort_order, p.slug as page_slug
FROM menu_items mi
JOIN menus m ON m.id = mi.menu_id
LEFT JOIN pages p ON p.id = mi.page_id AND p.status = 'published'
WHERE m.website_id = $1 AND m.location_key = $2
ORDER BY mi.parent_id NULLS FIRST, mi.sort_order;
```
Then build tree in Go by grouping by parent_id. No recursive CTE needed -- flat query + in-memory tree build is simpler and sufficient for max 3 levels.
[ASSUMED -- flat query + Go tree build is standard for shallow hierarchies]

### Pattern: Template Loader Integration

**Key change:** The current `loader.go` resolves templates at `data/templates/{websiteID}/`. With D-06, templates are stored at `data/templates/{template_slug}/`. The loader needs a way to look up the active template slug for a website.

Options:
1. Pass a callback/interface to the loader that resolves websiteID -> template slug
2. Store the active template slug path in a simple map updated on activation

Recommend option 1: add a `TemplateResolver` interface:
```go
type TemplateResolver interface {
    ActiveTemplateSlug(ctx context.Context, websiteID int64) (string, error)
}
```
The loader calls this instead of hardcoding `websiteID` in the path. The `tmplmgr.Store` implements this interface.

### Anti-Patterns to Avoid
- **Reading Content-Type header for MIME validation:** Trivially spoofed. Always use `http.DetectContentType` on file bytes. [VERIFIED: standard security practice]
- **Using os.MkdirAll without checking zip-slip first:** Must validate path BEFORE creating directories.
- **Storing media files with original filenames:** Collision risk + path traversal. UUID prefix required.
- **DELETE CASCADE on templates table to website_templates:** Use RESTRICT -- prevent deleting templates that are active.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| MIME detection | Custom magic byte tables | `http.DetectContentType` | Implements full mimesniff spec [VERIFIED] |
| UUID generation | math/rand strings | `crypto/rand` + hex encoding | Cryptographic randomness for filenames |
| Zip extraction | Manual byte-level parsing | `archive/zip` | Handles all compression methods, file modes |
| Path cleaning | String manipulation | `filepath.Clean` + `filepath.Join` | Handles ../, ./, platform separators correctly |

## Common Pitfalls

### Pitfall 1: Zip-Slip Path Traversal
**What goes wrong:** Malicious zip contains `../../etc/passwd`; naive extraction writes outside target dir.
**How to avoid:** `filepath.Clean` + `strings.HasPrefix(destPath, destDir+pathSep)` on every file before extraction. Reject entire archive on first violation.
**Warning signs:** Any extracted path not starting with destination prefix.

### Pitfall 2: Template Loader Cache Staleness
**What goes wrong:** Upload/activate a new template but public site still serves old cached version.
**How to avoid:** Call `InvalidateTemplateCache(websiteID)` after every template activation change. Already exists in loader.
**Warning signs:** Template changes not appearing on public site.

### Pitfall 3: Partial Zip Extraction on Error
**What goes wrong:** Zip extraction fails midway, leaving partial files on disk.
**How to avoid:** Extract to a temporary directory first, then `os.Rename` to final location atomically. On failure, clean up temp dir.
**Warning signs:** Broken template directories with missing files.

### Pitfall 4: SVG MIME Detection
**What goes wrong:** `http.DetectContentType` returns `text/xml` or `text/plain` for SVG files, not `image/svg+xml`.
**How to avoid:** For SVG specifically, also check file extension as a secondary signal. DetectContentType doesn't reliably identify SVG. [ASSUMED -- Go's sniffing is content-based and SVGs start with XML]
**Warning signs:** SVG uploads rejected despite being valid.

### Pitfall 5: Menu Sort Order Gaps
**What goes wrong:** After many reorders, sort_order values become large/sparse, making insert-between awkward.
**How to avoid:** For v1 with up/down buttons, just swap adjacent sort_order values. No gaps issue. Renormalize only if needed later.

### Pitfall 6: Multipart Memory Limits
**What goes wrong:** `r.ParseMultipartForm(maxMemory)` -- if maxMemory is too small, large files spool to temp disk; if too large, memory pressure on Pi.
**How to avoid:** Use `r.FormFile()` which calls ParseMultipartForm with 32MB default. For 10MB template / 5MB media limits, validate `Content-Length` header first as early rejection, then check actual size during read with `io.LimitReader`.

## Code Examples

### Template Upload Handler Pattern
```go
func (h *Handler) HandleTemplateUpload(w http.ResponseWriter, r *http.Request) error {
    // Limit request body
    r.Body = http.MaxBytesReader(w, r.Body, h.maxTemplateSize)
    
    file, header, err := r.FormFile("template")
    if err != nil {
        // MaxBytesReader returns specific error on overflow
        web.SetFlashError(h.sm, r.Context(), "File too large or missing")
        http.Redirect(w, r, "/admin/templates", http.StatusSeeOther)
        return nil
    }
    defer file.Close()
    
    // Save to temp file, extract, validate, move to final location
    // ...
}
```

### Media Serving Handler
```go
func (h *Handler) HandleMediaServe(w http.ResponseWriter, r *http.Request) error {
    websiteID, _ := strconv.ParseInt(r.PathValue("websiteID"), 10, 64)
    filename := r.PathValue("filename")
    
    // Look up in DB for mime_type
    m, err := h.store.GetByFilename(r.Context(), websiteID, filename)
    if err != nil || m == nil {
        http.NotFound(w, r)
        return nil
    }
    
    path := filepath.Join(h.dataDir, "media", strconv.FormatInt(websiteID, 10), filename)
    
    w.Header().Set("Content-Type", m.MimeType)
    w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
    w.Header().Set("Content-Disposition", "inline")
    http.ServeFile(w, r, path)
    return nil
}
```

### Menu FuncMap Helper
```go
// menuHTML renders a menu as nested <ul><li> by location key
func menuHTML(menus map[string][]MenuNode, locationKey string) template.HTML {
    items, ok := menus[locationKey]
    if !ok { return "" }
    return template.HTML(renderMenuLevel(items, 0, 3))
}

func renderMenuLevel(items []MenuNode, depth, maxDepth int) string {
    if depth >= maxDepth || len(items) == 0 { return "" }
    var sb strings.Builder
    sb.WriteString("<ul>")
    for _, item := range items {
        sb.WriteString("<li>")
        if item.URL != "" {
            sb.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, 
                template.HTMLEscapeString(item.URL), 
                template.HTMLEscapeString(item.Title)))
        } else {
            sb.WriteString(template.HTMLEscapeString(item.Title))
        }
        if len(item.Children) > 0 {
            sb.WriteString(renderMenuLevel(item.Children, depth+1, maxDepth))
        }
        sb.WriteString("</li>")
    }
    sb.WriteString("</ul>")
    return sb.String()
}
```

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Schema design follows project patterns | Migration | LOW -- consistent with existing migrations |
| A2 | Flat query + Go tree build sufficient for menus | Menu Tree Query | LOW -- max 3 levels is trivially shallow |
| A3 | SVG MIME detection unreliable via DetectContentType | Pitfall 4 | MEDIUM -- may need extension fallback for SVG uploads |
| A4 | Partial unique index (WHERE is_active=1) works in modernc.org/sqlite | Migration | MEDIUM -- if not supported, enforce in application code |

## Open Questions

1. **Partial unique index support in modernc.org/sqlite**
   - What we know: Standard SQLite supports `CREATE UNIQUE INDEX ... WHERE` since 3.8.0
   - What's unclear: Whether modernc.org/sqlite's pure-Go implementation supports it
   - Recommendation: Test during implementation; fallback to application-level enforcement if needed

2. **Template loader path change**
   - Current: `data/templates/{websiteID}/`
   - New: `data/templates/{template_slug}/`
   - This changes the loader's resolution logic. The loader needs a DB-backed resolver to map websiteID -> active template slug. This is the most complex integration point.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Existing auth middleware |
| V4 Access Control | yes | Admin-only routes for template/media management |
| V5 Input Validation | yes | Zip-slip, extension allow-list, MIME magic bytes, size limits |
| V6 Cryptography | no | N/A |
| V12 File Upload | yes | All three subsystems involve file upload |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Zip-slip path traversal | Tampering | filepath.Clean + prefix check per OWASP |
| MIME type spoofing | Spoofing | http.DetectContentType on file bytes, not Content-Type header |
| Filename injection | Tampering | UUID-prefixed filenames, never use raw user input in paths |
| DoS via large upload | Denial of Service | http.MaxBytesReader + Content-Length early check |
| XSS via SVG upload | Tampering | Serve SVGs with Content-Disposition: inline but from media subdomain/path; CSP headers recommended |
| Template injection | Elevation | Templates are Go html/template which auto-escapes; uploaded templates are parsed by html/template which is safe |

## Sources

### Primary (HIGH confidence)
- Go stdlib `archive/zip`, `net/http.DetectContentType`, `path/filepath` -- verified via go doc
- Existing codebase: loader.go, store.go, handler.go patterns -- verified via file read
- CONTEXT.md decisions D-01 through D-21 -- locked decisions

### Secondary (MEDIUM confidence)
- OWASP zip-slip prevention pattern (clean + prefix check)

### Tertiary (LOW confidence)
- SVG MIME detection behavior (A3) -- needs runtime verification

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all Go stdlib, no new deps
- Architecture: HIGH -- follows established project patterns exactly
- Pitfalls: HIGH -- zip-slip and MIME validation are well-documented domains
- Schema: MEDIUM -- partial unique index support in modernc.org/sqlite unverified

**Research date:** 2026-04-14
**Valid until:** 2026-05-14 (stable domain, no fast-moving deps)
