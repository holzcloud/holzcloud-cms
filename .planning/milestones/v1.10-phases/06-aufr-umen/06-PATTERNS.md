# Phase 6: Aufräumen — Pattern Map

**Mapped:** 2026-09-04
**Files analyzed:** 22 (3 new code files, 8 modified code/CI files, 11 documentation files)
**Analogs found:** 9 / 11 code+CI files (documentation edits are in-place, no analog needed)

All analog paths below were verified git-tracked (`git ls-files`). No gitignored
mirror paths appear in this document.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `tools/wasm/main.go` (NEW) | tool (`package main`) | batch / subprocess + file-I/O | `tools/i18n/main.go` (shape, flags, `fail`) + `tools/mkbundle/main.go` (zip, sha256, per-unit progress) | role-match (exact for the shell; **no analog for `os/exec`** — see "No Analog Found") |
| `tools/i18n/main_test.go` (NEW) | test | file-I/O round-trip | `tools/mkbundle/pack_test.go` | exact |
| `internal/plugin/wasmtest/wasmtest.go` (NEW) | test helper (non-test package) | file-I/O | `internal/plugin/runtime_test.go:19–29` (`echoModul`, the helper being extracted) | role-match; **no `testutil`-style package exists in this repo today** |
| `tools/i18n/main.go` (`writeCatalog`, `quote`, `:1–15`, `:113`, `:288`) | tool | transform | itself — `quote()` at `:277–287` already is the stdlib route, per string | exact (in-file) |
| `.github/workflows/ci.yml` (2 new steps) | config (CI) | batch | `ci.yml:47–50` "Verify go.mod is tidy" | exact |
| `.github/workflows/ci.yml` / `security.yml` / `release.yml` (`env:`) | config (CI) | — | the existing `env: CGO_ENABLED: "0"` block in each of the three | exact |
| `internal/plugin/runtime_test.go:13` (`//go:generate`) | test / build directive | batch | itself; flags must match `tools/wasm` | exact |
| 5 × `t.Skipf` call sites | test | file-I/O guard | each other (identical five-line shape) | exact |
| `plugins/README.md:47`, `internal/plugin/testdata/README.md` | documentation | — | each other (same build line, two renderings) | exact |
| `plugins/*/{bestellung,jahreszahl,nicht-gefunden,suche}.zip` (repacked by `tools/wasm`) | build artifact | file-I/O | `tools/mkbundle/main.go:335–381` `writeZip` | role-match (needs one addition — see below) |
| `docs/offene-punkte.md`, 7 × `.planning/codebase/*.md`, 3 × `deferred-items.md`, `REQUIREMENTS.md`, `ROADMAP.md` | documentation | — | in-place surgical edits (D-19, D-20) | n/a |

---

## Pattern Assignments

### `tools/wasm/main.go` (NEW — tool, batch/subprocess)

**Primary analog:** `tools/i18n/main.go` (the house shape)
**Secondary analog:** `tools/mkbundle/main.go` (sha256, zip, per-unit progress line)

**Doc-comment shape** — copy the register from `tools/i18n/main.go:1–16`. Opens
`// Command <name> …`, then an indented invocation block, then prose *why*:

```go
// Command i18n keeps the message catalogues in step with the source.
//
//	go run ./tools/i18n            # report what is missing
//	go run ./tools/i18n -write     # add the missing keys with empty values
//	go run ./tools/i18n -schweiz   # rebuild de-CH.json from the German
//
// It collects every German string that reaches a person: …
package main
```

`tools/mkbundle/main.go:1–17` is the same shape with 16 lines of rationale.
**Comments inside `tools/` are English** (both siblings are English throughout);
comments inside `internal/` are German. `tools/wasm/main.go` is therefore English.

**Error exit** — `tools/i18n/main.go:309–313`, copy verbatim:

```go
func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
```

`tools/mkbundle/main.go:37–52` shows the second accepted form for an
argument/usage error — `fmt.Fprintln(os.Stderr, "usage: …")` + `os.Exit(2)` for
misuse, `os.Exit(1)` for a failure. Use `2` for a bad flag combination, `1` for
a failed build or a hash mismatch. **Not `slog`, not `log.Fatal`.**

**Per-unit progress on stdout, in German** — `tools/mkbundle/main.go:48–51`:

```go
		info, _ := os.Stat(out)
		fmt.Printf("%s -> %s (%.1f MB)\n", dir, out, float64(info.Size())/(1<<20))
```

and `tools/i18n/main.go:113`:

```go
			fmt.Printf("%-12s %d Abweichungen, %d ohne Gegenstück\n", e.Name(), len(catalog), wrong)
```

Note the split that must be preserved: **English comments, German output.** One
line per target (six lines), column-aligned with `%-…s` like the i18n line.

**sha256** — `tools/mkbundle/main.go:322–333` (`hashFile`) is the existing hash
helper; it uses `crypto/sha256` + `encoding/hex` + `io.Copy`, exactly the import
set `tools/wasm` needs. Reuse the function shape, not the function (different
package).

**Deterministic zip (D-23)** — `tools/mkbundle/main.go:335–381` `writeZip` is the
analog and it is *almost* what is needed. Build in memory, `os.WriteFile` once,
`Method: zip.Store` for already-compressed payload:

```go
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	…
		dst, err := zw.CreateHeader(&zip.FileHeader{Name: bundle.MediaDir + name, Method: zip.Store})
	…
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
```

Also copy `sort.Strings(names)` before the write loop — mkbundle already sorts
entries so the archive order is stable.

**The one deviation `tools/wasm` must add that mkbundle does not have:**
`writeZip` sets no `Modified`, so `archive/zip` stamps the current time and the
archive is not byte-comparable. `tools/wasm` must set an explicit fixed
`Modified` (and use `zip.Deflate` or `zip.Store` consistently) on each
`zip.FileHeader`. The committed archives contain exactly two entries,
`plugin.json` then `plugin.wasm` (verified: `unzip -l plugins/jahreszahl/jahreszahl.zip`),
flat, no directory prefix — matching `plugins/README.md:48`'s `zip -j`.
Four archives only: `bestellung`, `jahreszahl`, `nicht-gefunden`, `suche`.
`plugins/kontaktformular` has no zip (it carries `migrations/`).

**Flag handling** — `tools/i18n/main.go` uses `flag` with a `-root` string
defaulting to `"."`. `tools/wasm` needs `-check`, `-print-hashes` (research §D-05
gate) and the D-05-fallback `-out`; declare them all in `main()` with `flag`.

**The build invocation to copy** — `internal/plugin/runtime_test.go:13`, the one
working wasip1 line in the tree today:

```go
//go:generate sh -c "cd testdata/echo && GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -ldflags=\"-s -w\" -o ../echo.wasm ."
```

and its two prose twins, `plugins/README.md:47`:

```sh
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o plugin.wasm .
zip -j jahreszahl.zip plugin.json plugin.wasm
```

and `internal/plugin/testdata/README.md:5–8` (same line, `-o ../echo.wasm`).
**All four places gain `-buildvcs=false`** (D-02a). In Go source the ldflags
argument is a single `exec` arg — `"-ldflags=-s -w"`, never with literal quotes.

**Six targets** (D-07): `plugins/{bestellung,jahreszahl,kontaktformular,nicht-gefunden,suche}`
plus `internal/plugin/testdata/echo` → `internal/plugin/testdata/echo.wasm`.

---

### `tools/i18n/main.go` — `writeCatalog` rewrite (tool, transform)

**Analog:** the same file. `quote()` at `:277–287` already *is* the target
technique applied one string at a time — the rewrite generalises it to the map:

```go
// quote is json.Marshal for a string without the HTML escaping.
// …
func quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return `""`
	}
	return strings.TrimRight(b.String(), "\n")
}
```

The rationale paragraph inside `quote`'s doc comment ("these files are
dictionaries a translator reads, half the sentences contain a `<code>` …") is
the text to **move**, not rewrite, onto the new `writeCatalog`. Today's
`writeCatalog` doc comment at `:288–289` reads "writes the file sorted and
indented" — that is MAINT-02's false claim (D-17a); the corrected wording is
"sorted and flush left, one key per line".

**Keep:** `sort` stays imported — `main()` at `:281`-adjacent still calls
`sort.Strings(sorted)`. `os.WriteFile(path, …, 0o644)` is the existing perm.

**Report line to extend (D-17)** — `tools/i18n/main.go:113`, inside the regional
branch at `:99–114` whose comment already explains the deviation-list rationale:

```go
			fmt.Printf("%-12s %d Abweichungen, %d ohne Gegenstück\n", e.Name(), len(catalog), wrong)
```

---

### `tools/i18n/main_test.go` (NEW — test, file-I/O round-trip)

**Analog:** `tools/mkbundle/pack_test.go` — the only test beside a `tools/`
command, and the same "the committed artefact must still survive the tool" shape.

```go
package main

import (
	"archive/zip"
	…
	"testing"
	…
)

// beispiel is the example website kept in the repository as source.
func beispiel() string { return filepath.Join("..", "..", "sites", "beispiel") }

// TestTheExampleStillPacks is the reason the example does not quietly rot.
// …
// It packs into t.TempDir(): the archive is a build artefact, and a test that
// left one beside the source would put an ignored file into everybody's
// `git status` forever.
func TestTheExampleStillPacks(t *testing.T) {
	out := filepath.Join(t.TempDir(), "beispiel.zip")
	if err := pack(beispiel(), out); err != nil {
		t.Fatalf("pack: %v", err)
	}
```

Copy from it: `package main` (in-package test, so `writeCatalog`/`readCatalog`
are reachable), a `filepath.Join("..", "..", …)` locator function for the
repository path, `t.TempDir()` for every written byte, `t.Fatalf("<verb>: %v")`
for infrastructure failures and `t.Errorf` for assertion failures, and a long
doc comment that says *why the test exists* rather than what it does.

**Deviation:** `pack_test.go` is English (it is a `tools/` test). The research's
proposed `main_test.go` body is German. Follow `pack_test.go` — **English**, for
the same reason the two commands are English.

**Assertion the planner must keep:** the `gesehen != 7` count check, and the rule
that the test reads the seven real files and never synthesises an empty
catalogue (Pitfall 6 — `{\n}\n` vs `{}\n` is the one divergence).

---

### `internal/plugin/wasmtest/wasmtest.go` (NEW — test helper package)

**No `testutil`-style shared package exists anywhere in `internal/`** (verified:
no non-`_test.go` file in `internal/` declares a `func …(t *testing.T)`). This
package is the first of its kind; it therefore takes its shape from the helper
it replaces, `internal/plugin/runtime_test.go:15–29`:

```go
// echoModul ist in testdata/echo/ gebaut, siehe testdata/README.md:
//
//	cd testdata/echo && GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o ../echo.wasm .
//
// Es liegt gebaut im Repository, damit die Tests ohne zweite Werkzeugkette
// laufen — ein Test, der einen Compiler-Lauf braucht, wird irgendwann
// übersprungen und dann nie wieder ausgeführt.
func echoModul(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/echo.wasm")
	if err != nil {
		t.Skipf("testdata/echo.wasm fehlt: %v", err)
	}
	return b
}
```

Copy: `t.Helper()` first line, `os.ReadFile` + relative-to-test path, returning
`[]byte`, and the German comment register (this is `internal/`, so **German**,
unlike `tools/`). Package name is one lowercase word per CLAUDE.md — `wasmtest`.

**The five call sites, exactly as they read today** (all five are the identical
four-line shape; the planner replaces each with one `wasmtest.Modul(t, …)` line):

```go
// internal/plugin/runtime_test.go:22-27  (package plugin)
	b, err := os.ReadFile("testdata/echo.wasm")
	if err != nil {
		t.Skipf("testdata/echo.wasm fehlt: %v", err)
	}

// internal/plugin/sdk_e2e_test.go:21-24  (package plugin_test)
	modul, err := os.ReadFile("../../plugins/jahreszahl/plugin.wasm")
	if err != nil {
		t.Skipf("plugins/jahreszahl/plugin.wasm fehlt: %v", err)
	}

// internal/plugin/hofladen_e2e_test.go:23-26  (package plugin_test)
	modul, err := os.ReadFile("../../plugins/bestellung/plugin.wasm")
	if err != nil {
		t.Skipf("plugins/bestellung/plugin.wasm fehlt: %v", err)
	}

// internal/public/formular_e2e_test.go:158-161  (package public)
	modul, err := os.ReadFile("../../plugins/kontaktformular/plugin.wasm")
	if err != nil {
		t.Skipf("plugins/kontaktformular/plugin.wasm fehlt: %v", err)
	}

// internal/public/suche_e2e_test.go:26-29  (package public)
	modul, err := os.ReadFile("../../plugins/suche/plugin.wasm")
	if err != nil {
		t.Skipf("plugins/suche/plugin.wasm fehlt: %v", err)
	}
```

Three distinct Go packages (`plugin`, `plugin_test`, `public`) — which is why a
`_test.go`-local helper cannot serve all five and the non-test package is
required (research §Pattern 3).

---

### `.github/workflows/ci.yml` — two new steps

**Analog:** `ci.yml:47–50`, the step both new steps are modelled on (D-15):

```yaml
    - name: Verify go.mod is tidy
      run: |
        go mod tidy
        git diff --exit-code -- go.mod go.sum
```

Note the surrounding house style, all of it copied into the new steps:

- Step names are English sentences: `Verify formatting`, `Verify go.mod is tidy`, `Vet`, `Build`, `Test`, `Upload the binary`.
- A `#` comment **above** a step explains *why it exists*, in German, when the reason is not obvious — see `ci.yml:52–54` above `Build`, and `ci.yml:69–70` above `Upload the binary`.
- `sha256sum` is already the repo's hash of record in CI (`ci.yml:63`).
- Placement per D-08: the `tools/wasm -check` step goes after `Vet` and before `Build`/`Test`.

**Actions pinning** (`ci.yml:24–28`) — no new action is added by this phase, but
if one ever is, the rule is stated in-file:

```yaml
    # Actions are pinned to a commit, not a tag: a tag can be moved to point at
    # different code, a commit cannot. The version in the comment is what keeps
    # the pin readable, and Dependabot bumps it together with the hash monthly.
    - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
```

---

### Workflow-level `env:` in three workflows (D-10)

**Analog:** the identical existing block in each of the three. Add the new
variable beside `CGO_ENABLED`, with a German comment in the same register.

`ci.yml:15–17` and `release.yml:12–14` (identical, comment included):

```yaml
env:
  # Pure-Go SQLite driver: the binary must build without a C toolchain.
  CGO_ENABLED: "0"
```

`security.yml:11–12` (same block, no comment):

```yaml
env:
  CGO_ENABLED: "0"
```

Value is quoted (`"0"`), so `HOLZCLOUD_TEST_REQUIRE_WASM: "1"`.
`image.yml` is deliberately untouched (runs no tests).

`security.yml:34–44` also carries the 297 s measurement that the CI-budget note
refers to, and shows the per-step `env:` override form (`CGO_ENABLED: "1"`) —
proof that a workflow-level `env:` is overridable per step, should the plan ever
need to exempt one.

---

## Shared Patterns

### Errors and exit in `tools/`
**Source:** `tools/i18n/main.go:309–313`, `tools/mkbundle/main.go:37–52`
**Apply to:** `tools/wasm/main.go`
`fmt.Fprintln(os.Stderr, "error:", err)` + `os.Exit(1)`; `os.Exit(2)` for usage.
CLAUDE.md's `slog.Error` rule does **not** reach `tools/`.

### Comment language
**Source:** `tools/i18n/main.go`, `tools/mkbundle/main.go` (English throughout);
`internal/plugin/runtime_test.go`, all five e2e tests (German throughout)
**Apply to:** `tools/wasm/main.go` and `tools/i18n/main_test.go` → **English**;
`internal/plugin/wasmtest/wasmtest.go` → **German**.
User-facing stdout is German in both.

### "The generated artefact is in step with the source"
**Source:** `ci.yml:47–50`
**Apply to:** the new i18n CI step, and conceptually to `tools/wasm -check`.

### Write-in-memory, then one `os.WriteFile`
**Source:** `tools/mkbundle/main.go:335–381`, comment at `:336–338`
("a zip that was interrupted half-way still looks like a file")
**Apply to:** `tools/wasm`'s zip repack, and — via `os.Rename` from a temp file
*beside the target* — its `.wasm` writes.

### Doc comments state *why*, at length
**Source:** every file read for this map. `tools/mkbundle/main.go:1–16` (16
lines before `package main`), `pack_test.go:24–34`, `readManifest`'s comment at
`:86–93`, `runtime_test.go:15–21`.
**Apply to:** all three new files. A short doc comment is off-register here.

---

## No Analog Found

| File / concern | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `tools/wasm/main.go` — the `os/exec` half | tool | subprocess | **`os/exec` is imported nowhere in this repository** (verified by grep across all `*.go`). Use the code example in `06-RESEARCH.md` §"Pattern 2: Driving a build in a foreign module from Go" as the source: `cmd.Dir` set to the module dir, `cmd.Env = append(os.Environ(), …)` with `GOOS`/`GOARCH`/`CGO_ENABLED`/`GOTOOLCHAIN`/empty `GOFLAGS`/empty `GOEXPERIMENT`, `cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr`. |
| `internal/plugin/wasmtest/` | test helper package | — | No shared test-helper package exists in `internal/` today; this is the first. Shape taken from the `echoModul` helper it replaces, plus `net/http/httptest` as the naming precedent (research §Pattern 3). |
| Deterministic (fixed-timestamp) zip | build artifact | file-I/O | `tools/mkbundle`'s `writeZip` is the closest, but it sets no `Modified` and is therefore *not* byte-reproducible. The `Modified` field is a net addition with no in-repo precedent. |
| `.planning/codebase/*.md`, `docs/offene-punkte.md`, `deferred-items.md` edits | documentation | — | Surgical in-place edits (D-19/D-20/D-21); the line-referenced correction inventory in `06-RESEARCH.md` §MAINT-05 is the specification, not a pattern. |

## Metadata

**Analog search scope:** `tools/`, `internal/`, `.github/workflows/`, `plugins/`, `docs/`, `.planning/`
**Files read this session:** `tools/i18n/main.go`, `tools/mkbundle/main.go`, `tools/mkbundle/pack_test.go`, `internal/plugin/package.go` (grep only), the five test call sites, `.github/workflows/{ci,security,release}.yml`, `plugins/README.md`, `internal/plugin/testdata/README.md`
**Pattern extraction date:** 2026-09-04
