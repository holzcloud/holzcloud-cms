---
phase: 09-csv-import
plan: 01
wave: 1
subsystem: csv-import
status: complete
tags: [csv, parser, hostile-input, bounds, formula-injection, purity]

requires: []
provides:
  - "csv.MaxRows, csv.MaxSpalten, csv.MaxCellBytes — the three bounds encoding/csv does not have"
  - "csv.PruefeBytes + ErrLeer / ErrNurBOM / ErrNullbyte — the three refusals before parsing"
  - "csv.Zeilennummer — the one place the operator's row number is minted"
  - "csv.Leser / csv.Zeile / csv.Neu / Kopf / Naechste / Abgeschnitten — the streaming reader"
  - "csv.Beispiel — the example-CSV writer, BOM first, every cell defused"
affects:
  - "plugins/kontaktformular/csv.go — comment only, the mirrored half of D-16"

tech-stack:
  added: []
  patterns:
    - "a cap is a named constant with two sentences of reason above it (internal/wxr/wxr.go:25-30)"
    - "a cap is reported, never silently applied (wxr.Export.Truncated, wxr.go:63)"
    - "a row carrying a Fehler still carries the cells that were read (wordpress.go:107's discipline)"
    - "German test names, one per defence, each naming its D-number"

key-files:
  created:
    - internal/csv/csv.go
    - internal/csv/csv_test.go
    - internal/csv/beispiel.go
    - internal/csv/beispiel_test.go
  modified:
    - plugins/kontaktformular/csv.go

decisions:
  - "The row number is minted by one exported helper, Zeilennummer(index) = index + 2, and by nothing else (D-26)"
  - "MaxCellBytes counts bytes via len() on the string; a rune cap would be a second definition of size (IMP-09 precision)"
  - "MaxSpalten = 100 refuses at Neu with its own sentinel, ErrZuVieleSpalten (D-38)"
  - "A data row longer than the header keeps every cell and is reported — nothing is dropped in silence (gap the plan left open)"
  - "TestBeispielZitiertSelbstNicht round-trips through this package's own Leser, not through a second stdcsv.Reader in the test file"

metrics:
  duration: "~8 min"
  completed: 2026-09-06
  tasks: 3
  commits: 2
  files_created: 4
  files_modified: 1

actuals:
  tokens: 6900
  tasks: 3
  commits: 2
---

# Phase 9 Plan 01: internal/csv — the half most likely to be wrong Summary

A pure, standard-library-only CSV reader that bounds rows, columns and cells,
refuses three degenerate files before parsing, strips the Excel BOM once at the
reader, and mints the operator's row number in exactly one place — plus the
example-CSV writer and this repository's second, cross-referenced copy of
`entschaerfen`.

## What was built

**`internal/csv/csv.go`** (282 lines). Package doc in two paragraphs in
`wxr.go:1-12`'s voice: why the package exists, then what it deliberately does
not do (no separator sniffing, no third-party fetch, no database and no HTTP).
Three bounds, each a constant with its own two-sentence reason:

| Constant | Value | Enforced | Reported |
|---|---|---|---|
| `MaxRows` | 5000 | in `Naechste` | `Abgeschnitten()` |
| `MaxSpalten` | 100 | in `Neu` | `ErrZuVieleSpalten` |
| `MaxCellBytes` | 100000 **bytes** | per cell in `Naechste` | `Zeile.Fehler`, naming the column |

`PruefeBytes` returns `ErrLeer`, `ErrNurBOM` or `ErrNullbyte` — three
distinguishable reasons on the raw bytes, before any parsing. `Zeilennummer` is
three lines and carries the longest comment in the file. `Neu` strips the BOM
once via `bufio.Reader.Peek`/`Discard` before the header record is read, sets
`LazyQuotes = true` and `FieldsPerRecord = -1` (both with their reason in a
comment), leaves `ReuseRecord` at `false`, and reads the header. `Naechste`
counts rows in its own counter, pads short rows to `len(Kopf())` so missing
cells are empty in position, and reports rather than truncates.

**`internal/csv/beispiel.go`** (79 lines). `Beispiel(kopf []string, zeilen
[][]string)` — plain strings, not `[]field.Def`, so no store-carrying package
gets in. BOM, then header and every sample row through `entschaerfen`, then
`Flush`/`Error`. `entschaerfen` is the plugin's implementation copied whole,
with the "where the content comes from" sentence adjusted and a paragraph added
naming `plugins/kontaktformular/csv.go` and why it cannot be imported.

**The tests** (398 lines over two files), one per defence, named after the
defence: `TestBOMWirdEinmalGestreift`, `TestZeilennummerIstDieDerTabelle`,
`TestZeilengrenze`, `TestSpaltengrenze`, `TestZellengrenzeInBytes`,
`TestKurzeZeileVerschiebtNicht`, `TestStrayQuoteFrisstDenRestNicht`,
`TestZitatMitTrennzeichenUndZeilenumbruch`, `TestLeerzeileZaehltNicht`,
`TestPruefeBytesLehntAb`, `TestNurKopfzeileIstKeinFehler`,
`TestOhneKopfzeileIstFehler`, `TestEntschaerfenNimmtFormelnDieBedeutung`,
`TestBeispielSchreibtDieBOM`, `TestBeispielZitiertSelbstNicht`,
`TestBeispielOhneZeilen`. No test opens a database, builds a request or uses
`t.TempDir()`.

## Edge resolutions turned into assertions

| Edge | Assertion |
|---|---|
| IMP-03 boundary | `Zeilennummer(0) == 2`, `Zeilennummer(1) == 3`, and the first read row's `Nummer == 2` |
| IMP-09 boundary (rows) | `MaxRows` rows → all read, `Abgeschnitten()` false; `MaxRows+1` → `MaxRows` read, `Abgeschnitten()` true |
| IMP-09 boundary (cells) | exactly `MaxCellBytes` accepted; `MaxCellBytes+1` reported with `Nummer == 3` and row 4 still read clean |
| IMP-09 boundary (columns, D-38) | header of exactly 100 accepted; 101 → `ErrZuVieleSpalten` |
| IMP-09 precision | `MaxCellBytes` bytes of four-byte runes accepted; `MaxCellBytes` *runes* of four bytes refused |
| IMP-09 adjacency | `a,"b,c",d` → three cells, middle `b,c`; a cell spanning two lines leaves the following row at `Nummer == 3` while it sits on file line 4 |
| IMP-09 empty | `PruefeBytes` told apart for empty, nil, BOM-only, NUL, NUL-behind-BOM, and two legitimate files |
| IMP-09 ordering | header of 5, row of 3 → five cells, cell 3 is `"3"`, cells 4 and 5 empty |
| IMP-01 empty | header with no data rows: `Neu` succeeds, no rows, not truncated |

## Verification gate output

Every gate below was run against the post-change tree and its literal output is
recorded. Two diverge from the plan; both are plan defects, not code problems,
and neither was quietly adjusted.

### Task 1

```
go build ./... && go vet ./internal/csv/ && gofmt -l internal/csv/   → exit=0, no output
go test ./internal/csv/ -v                                          → PASS, ok … 0.393s (16 tests, 0 FAIL)
go list -deps ./internal/csv/ | grep -c holzcloud-cms               → 1        (plan expects 0 — see divergence 1)
go list -f '{{TestImports}}{{XTestImports}}' … | grep -c 'holzcloud-cms\|net/http' → 0   (expects 0 ✓)
grep -v '^[[:space:]]*//' internal/csv/csv.go | grep -c 'MaxCellBytes\|MaxRows'    → 5   (expects ≥4 ✓)
grep -rln BeginTx internal/csv/ | wc -l                             → 0        (expects 0 ✓)
```

### Task 2

```
go build ./... && go vet ./internal/csv/ && gofmt -l internal/csv/ plugins/kontaktformular/  → exit=0, no output
go test ./internal/csv/ -run 'Beispiel|Entschaerfen' -v            → PASS, ok … 0.380s (0 FAIL)
grep -c 'internal/csv' plugins/kontaktformular/csv.go              → 1        (expects ≠0 ✓)
grep -c 'kontaktformular' internal/csv/beispiel.go                 → 1        (expects ≠0 ✓)
git diff -U0 -- plugins/kontaktformular/csv.go | grep -c '^[+-][^+-]' → 5, inspected by hand:
  +//
  +// Diese Regel steht zweimal in diesem Verzeichnisbaum. Die zweite Ausfertigung
  +// ist internal/csv/beispiel.go, für die Beispieldatei des CSV-Imports.
  +// Importieren lässt sich diese hier nicht: sie ist `package main` in einem
  +// wazero-Plugin. Eine Änderung an einer der beiden gehört in beide.
  All five are comment lines. No code line changed; `git show --stat` reads
  `plugins/kontaktformular/csv.go | 5 ++` with zero deletions.
```

### Task 3 — the counting table, baseline + what this plan adds

| What | Command | Baseline | Plan adds | Predicted | **Measured** |
|---|---|---|---|---|---|
| packages under `internal/` | `ls -d internal/*/ \| wc -l` | 38 | 1 | 39 | **39** ✓ |
| files matching `encoding/csv` | `grep -rl 'encoding/csv' --include='*.go' . \| wc -l` | 1 | 2 | 3 | **5** ✗ — divergence 2 |
| files using `BeginTx` | `grep -rln BeginTx internal/ --include='*.go' \| grep -v _test \| wc -l` | 14 | 0 | 14 | **14** ✓ |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | 48 | 0 | 48 | **48** ✓ |
| strings in source | `go run ./tools/i18n \| head -1` | 1158 | 0 | 1158 | **1158 Zeichenketten im Quelltext** ✓ |
| `layoutPageNames` entries | the slice at `internal/web/render.go:46` | 44 | 0 | 44 | **44** ✓ |

`go run ./tools/i18n` in full: `en.json 1158 übersetzt, 0 offen, 0 verwaist`,
same for `es.json`, `fr.json`, `it.json`; the three `-CH` files unchanged at 55 /
4 / 9 Abweichungen, `0 ohne Gegenstück`. No string a person reads was minted in
Go, which is what D-32 asks of this package.

```
go build ./... && go test ./...  → exit=0, 44 packages ok, 0 FAIL
```

The plugin lives in its own module and was checked there too:
`GOOS=wasip1 GOARCH=wasm go build ./...` ok, `go vet ./...` ok,
`go test ./...` → `ok kontaktformular 0.315s`.

## Divergences between the plan and the tree

### 1. `go list -deps … | grep -c holzcloud-cms` measures 1, not 0 — the gate is off by the package itself

**Direction:** measured one higher than the plan expects.
**Cause:** `go list -deps` includes the *listed package* in its own output. The
single match is `github.com/holzcloud/holzcloud-cms/internal/csv` itself. The
property the gate exists to protect holds exactly:

```
go list -deps ./internal/csv/ | grep holzcloud-cms
  github.com/holzcloud/holzcloud-cms/internal/csv          ← the package itself
go list -deps ./internal/csv/ | grep holzcloud-cms | grep -v 'internal/csv$' | wc -l
  0
```

Confirmed as a property of the command and not of this package: the same gate
over the equally pure `internal/wxr` also measures 1. **The correct gate for
later waves is `… | grep -v 'internal/csv$' | wc -l` expecting 0**, or
`go list -f '{{join .Deps "\n"}}'` which likewise excludes self. Not adjusted in
the plan file; recorded here.

### 2. `grep -rl 'encoding/csv' | wc -l` measures 5, not 3 — the gate counts mentions, not imports

**Direction:** measured two higher than the plan expects.
**Cause:** the command is a text grep over whole files, and the plan's arithmetic
("adds 2: `csv.go`, `beispiel.go`") counted *import statements*. The two extra
matches are prose, in comments the plan itself asked for:

- `internal/csv/csv_test.go:11` — "`encoding/csv` kennt keine Grenze: nicht für
  Zeilen, nicht für Spalten, nicht für Zellen" (the file-head paragraph the plan
  specifies for the test file)
- `internal/csv/beispiel_test.go:59` — "Das Anführungszeichen und das
  Trennzeichen erledigt encoding/csv"

Measured by import rather than by text, the number is exactly the predicted 3:
`plugins/kontaktformular/csv.go` plus the one Go package `internal/csv`, whose
import list `go list` reports as `bufio bytes encoding/csv errors fmt io
strings` — the alias `stdcsv` appears once per production file and nowhere in a
test. **The comments were not reworded to satisfy the grep**; rewriting prose to
move a count is exactly the "quiet adjustment" this wave was told not to make.
The honest gate for a later wave is
`grep -rl '"encoding/csv"\|stdcsv "encoding/csv"' --include='*.go' .`.

### 3. A gap the plan left open: a data row with more cells than the header

The plan specifies padding for short rows and says nothing about long ones.
Silently dropping the extra cells would violate "reported, never silently
applied", so `Naechste` keeps every cell and sets
`Fehler = "row has N cells, the header has M"`. The mapping reads by index
within the header, so the extra cells are unaddressable but not destroyed.
Recorded as a decision rather than left implicit.

### 4. `TestBeispielZitiertSelbstNicht` round-trips through `Neu`, not through a second `stdcsv.Reader`

The plan asks for the round trip "through a `stdcsv.Reader`". Written literally,
`beispiel_test.go` would need its own `encoding/csv` import — a fourth *real*
importer, moving a counting gate for cosmetic reasons. It round-trips through
this package's own `Leser` instead, which **is** a configured `stdcsv.Reader`
and proves strictly more: the writer's output is readable by this phase's own
reader, BOM included. The assertion the plan cares about is unchanged — a header
cell containing a comma and a quote, and a cell containing a newline, come back
identical.

## Deviations from plan

### One process deviation

**One commit per task instead of the TDD RED/GREEN pair.** The plan marks tasks
1 and 2 `tdd="true"`; the executing instruction for this wave says "one commit
per task, atomic". The RED/GREEN *cycle* was followed — each test file was
written first and its failure observed (`vet: internal/csv/csv_test.go:25:43:
undefined: Leser`, then `vet: internal/csv/beispiel_test.go:40:11: undefined:
entschaerfen`) before any implementation existed — but the two halves landed in
one commit per task rather than a `test(...)` commit followed by a `feat(...)`
one. No `refactor` gate was needed.

### No auto-fixes were needed

Nothing in the existing tree was broken by this wave and nothing was fixed
outside it. `go test ./...` was green before and is green after.

### Nothing was committed that this plan does not own

The developer was working in the same tree throughout and landed four commits
between this wave's two (`45ceca0`, `b2afb40`, `be6179d`, and one before the
wave started). Every commit here was made with an explicit pathspec —
`git add <named files>` then `git commit … -- <named files>` — so their staged
`.github/`, `CONTRIBUTING.md`, `SECURITY.md` and `README` work was never picked
up. `git show --stat` for both commits lists only the five files this plan owns,
with zero deletions.

## Known Stubs

None. Every symbol this plan declares is implemented and exercised by a test.

## Threat Flags

None. The three surfaces this plan touches are the ones the plan's threat model
already names (T-09-01 row/cell bounds, T-09-02 the NUL refusal, T-09-03 formula
injection); `MaxSpalten` adds a fourth bound in the same shape. The package
opens no socket, no file and no transaction.

## TDD Gate Compliance

RED was observed for both TDD tasks (compile failure naming the missing symbol).
GREEN was observed for both (`go test ./internal/csv/ -v` all PASS). The gates
are not separately visible in `git log` because of the one-commit-per-task
instruction recorded under Deviations.

## Commits

| Task | Commit | What |
|---|---|---|
| 1 | `83d73f2` | `feat(09-01): eine hochgeladene Tabelle wird gelesen, ohne ihr zu glauben` |
| 2 | `5a1b5f1` | `feat(09-01): die Beispieldatei, und die zweite Ausfertigung einer Regel` |
| 3 | — | arithmetic only; its output is the counting table above |

## What wave 2 inherits

- `internal/csv` is pure and must stay pure: `go list -deps` names nothing in
  this repository but the package itself. `internal/csvimport` is where the
  database goes (D-35).
- `csv.Zeilennummer` is the only source of a row number. The mapping screen, the
  sample-row stepper and the report all call it; none of them adds two of its
  own.
- `MaxSpalten` is declared and enforced at `Neu`; **plan 09-04 owns the refusal
  message on screen 1** (D-38).
- The 10 MB byte cap and its boundary pair are still plan 09-04's
  (`http.MaxBytesReader`), and are the two of IMP-09's six boundary cases this
  wave does not hold.
- `BeginTx` in `internal/` is still 14 files. It must not become 15.

## Self-Check: PASSED

```
FOUND: internal/csv/csv.go
FOUND: internal/csv/csv_test.go
FOUND: internal/csv/beispiel.go
FOUND: internal/csv/beispiel_test.go
FOUND: plugins/kontaktformular/csv.go
FOUND: 83d73f2
FOUND: 5a1b5f1
```
