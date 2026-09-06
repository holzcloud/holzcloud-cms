---
phase: 09-csv-import
plan: 02
wave: 2
subsystem: csv-import
status: complete
tags: [csv, staging, migration, one-way-door, token-hash, prune, concurrency]

requires:
  - "09-01 — internal/csv stays pure; this wave is where the database goes (D-35)"
provides:
  - "migration 00049 — csv_imports, STRICT, cascading from users, holding the raw upload as a BLOB"
  - "csvimport.Store — Stage, Holen, Loeschen, Prune"
  - "csvimport.Ablage — the staged upload as a Go value"
  - "csvimport.ErrAbgelaufen / ErrFremd — two distinguishable refusals (D-33)"
  - "admin.Handler.csvImports — constructed inside NewHandler, never nil"
  - "the csv-import-prune job in cmd/holzcloud/main.go, twelfth of twelve"
affects:
  - "internal/admin/handler.go — one struct member, one constructor line, one import"
  - "internal/db/migrations_down_test.go — TestMigration00049RunterUndRauf"

decision:
  id: wie-vorgeschlagen
  what: "Hash-only staging token, and no zuordnung column — the mapping travels as form state"
  decided_by: orchestrator
  decided: 2026-09-06
  where: "09-02-PLAN.md <context>, before task 1 began"
  rejected: [klartext-marke, zuordnung-als-spalte]

tech-stack:
  added: []
  patterns:
    - "a token is minted from crypto/rand and stored only as its SHA-256 (user/token.go:152-155, 00012:1-11)"
    - "a line deliberately NOT copied from its analog gets a comment naming the analog and a test asserting its absence"
    - "a nullable foreign key that is nullable on purpose says so in the column's own comment"
    - "a Down half that only drops says why it is short, because its neighbours are long"

key-files:
  created:
    - internal/db/migrations/00049_csv_imports.sql
    - internal/csvimport/store.go
    - internal/csvimport/store_test.go
  modified:
    - internal/db/migrations_down_test.go
    - internal/admin/handler.go
    - cmd/holzcloud/main.go

decisions:
  - "wie-vorgeschlagen: the token is stored as a hash and there is no zuordnung column — recorded before task 1, not re-opened"
  - "TestMigration00049RunterUndRauf went into migrations_down_test.go and not db_test.go: db_test.go is package db_test and cannot reach the unexported migrationProvider"
  - "Stage writes erstellt_am explicitly rather than leaning on the column DEFAULT, matching IssueToken; the DEFAULT stays as a safety net for a hand-written row"
  - "Holen parses erstellt_am leniently — an unparseable timestamp yields the zero time rather than failing the fetch, because the sweep and not the fetch is what that column serves"
  - "Two tests beyond the plan's list: TestWebsiteLoeschenLaesstAblageStehen (the ON DELETE SET NULL half of the behavior block) and TestAblageOhneWebsiteIstErlaubt (the modus='neu' row)"

metrics:
  duration: "~14 min"
  completed: 2026-09-06
  tasks: 2
  commits: 2
  files_created: 3
  files_modified: 3

actuals:
  tokens: 9562
  tasks: 2
  commits: 2
---

# Phase 9 Plan 02: The staging that makes IMP-05 true and IMP-08 buildable Summary

An uploaded CSV now survives four screens as raw bytes in its own `STRICT`
table, fetched back byte for byte by the person who staged it, refused for
anyone else, told apart from one the sweep already took, and dropped after a day
by a job that deliberately does not fire on restart.

## The one-way door, as decided

**Chosen: `wie-vorgeschlagen`** — hash-only token, no `zuordnung` column.
Decided by the orchestrator on 2026-09-06 and written into `09-02-PLAN.md`'s
`<context>` with both alternatives (`klartext-marke`, `zuordnung-als-spalte`)
and the evidence, **before task 1 began**. It was not re-opened during
execution and no checkpoint was raised. Both halves are in the shipped
migration:

- `token_hash TEXT NOT NULL UNIQUE`, and the lookup in `Holen` is
  `WHERE token_hash = $1`. The token is never in the database.
- No `zuordnung` column exists. The row is written once by `Stage` and no method
  in the package issues an `UPDATE`.

## What was built

**`internal/db/migrations/00049_csv_imports.sql`** (116 lines, German
throughout, in `00042`'s voice). The head comment carries the three sentences a
later reader needs: a CSV cannot ride in the SCS session because sessions live
in the same SQLite file and a two-megabyte session row is read and written on
every request of that account; a server cannot fill a file input, so screens 2
to 4 carry a token and no file field; and what is stored is the raw bytes, never
a parsed table.

Three columns carry a decision rather than a description:

| Column | What the comment has to say |
|---|---|
| `user_id` | **The one cascade in this tree that is not from `websites`.** Names `00012`/`user_tokens` as the precedent explicitly, because a reader who has seen the other forty-seven migrations would otherwise read the line as a mistake. It is what makes "one admin cannot resume another's import" a database fact rather than a handler convention. |
| `website_id` | Nullable **on purpose** — an import creating a new website has none yet. `ON DELETE SET NULL` and not `CASCADE`, so a target website deleted mid-wizard ends the wizard with a message (D-29) rather than with the operator's file vanishing. |
| `daten BLOB` | The raw upload, capped at 10 MB by the handler before it arrives. Written once, read three times, and `BLOB` is one of the five types `STRICT` admits. |

`modus` and `kollision` each carry a `CHECK (… IN (…))`. The `Down` half is
`DROP INDEX` then `DROP TABLE` and nothing else — with a paragraph saying why it
is two statements where `00047`'s is six: `00047` and `00048` are index
*exchanges*, so their rollback restores a prior state and can restore the wrong
one unnoticed (`00048`'s own `Down` comment spends a paragraph on exactly that
trap); `00049` only creates, so there is no prior state to overshoot.

**`internal/csvimport/store.go`** (197 lines). `Ablage`, `Store`, `NewStore`,
and four methods. `Stage` mints `make([]byte, 16)` through `crypto/rand.Read`,
renders with `hex.EncodeToString`, and inserts the SHA-256. `Holen` finds by
hash, then compares ownership — the ordering is what makes the two answers
distinguishable, and the check lives in the store rather than in each of the
four handlers so a fifth screen added later cannot forget it. `Loeschen` is one
`DELETE`; `Prune` is one `DELETE … WHERE erstellt_am < $1` returning
`RowsAffected`, matching `PurgeExpiredTokens`' signature so the job body is
three lines. **No `BeginTx` anywhere** — every method is a single statement,
which is what makes IMP-10 true by construction rather than by care.

The two lines deliberately *not* copied are both commented at the site of their
absence and both asserted by a test:

- `user/token.go:59-62` deletes the previous token of the same user. Copied
  here it would mean an admin with two tabs loses the first import, silently and
  with the file already gone.
- `twofactor.go:154` keys half-finished state on the user id. Nothing in this
  package is keyed on a user id.

**`internal/csvimport/store_test.go`** (400 lines, German test names). Opens
with a paragraph naming the two sentences it guards. Ten tests over a real
migrated SQLite in `t.TempDir()`.

**`cmd/holzcloud/main.go`** — the `csv-import-prune` job, `Every: 6 * time.Hour`
like `token-purge`, store built inside the closure from the `database` already
in scope, no `RunAtStart`. Its German comment answers the roadmap note that
argued against staging because it "needs no cleanup job", where a reader of the
code will meet it.

## Behaviour block → assertion

| Plan's behaviour line | Test |
|---|---|
| token is 32 hex chars, only its SHA-256 stored | `TestAblageKommtByteFuerByteZurueck` (length), `TestMarkeStehtNichtInDerDatenbank` (absent from every column; `token_hash` is 64 chars) |
| bytes come back byte for byte, BOM and binary tail included | `TestAblageKommtByteFuerByteZurueck` — BOM plus every byte `0x80`–`0xFF` |
| unknown token → `ErrAbgelaufen` | `TestUnbekannteMarkeIstAbgelaufen`, which also asserts it is **not** `ErrFremd` |
| foreign token → `ErrFremd`, distinguishable with `errors.Is` | `TestFremdeMarkeWirdAbgelehnt`, which also asserts the owner still gets through |
| two `Stage` calls leave two rows, first token still works | `TestZweiTabsUeberlebenEinander` — counts the rows *and* re-fetches both |
| `Prune` takes the two-day row, leaves the one-hour row, returns the count | `TestPruneRaeumtNurAltes` |
| `Loeschen` removes exactly one; the token then reads as expired | `TestLoeschenNimmtGenauEine` |
| deleting the user cascades the row away | `TestBenutzerLoeschenRaeumtAblageMit` |
| deleting the website leaves the row with NULL `website_id` | `TestWebsiteLoeschenLaesstAblageStehen` (added — the plan's behaviour block names it, its test list did not) |
| a `modus='neu'` row has no website at all | `TestAblageOhneWebsiteIstErlaubt` (added) |

## Verification gate output

Every gate run against the post-change tree, literal output recorded. One
diverges; it is a gate defect, not a code problem, and nothing was adjusted to
hide it.

### Task 1

```
go build ./... && go vet ./internal/db/ ./internal/csvimport/ ./internal/admin/
  && gofmt -l internal/db/ internal/csvimport/ internal/admin/   → exit=0, no output

go test ./internal/db/ -run 'TestRunMigrations|TestMigration00049' -v
  --- PASS: TestMigration00049RunterUndRauf (0.09s)
  --- PASS: TestRunMigrations (0.07s)
  --- PASS: TestRunMigrationsIdempotent (0.07s)
  PASS   ok  …/internal/db   0.654s
  (no "testing: warning: no tests to run", no "--- FAIL")

go test ./internal/csvimport/ -v
  --- PASS: TestAblageKommtByteFuerByteZurueck (0.09s)
  --- PASS: TestFremdeMarkeWirdAbgelehnt (0.07s)
  --- PASS: TestUnbekannteMarkeIstAbgelaufen (0.07s)
  --- PASS: TestZweiTabsUeberlebenEinander (0.07s)
  --- PASS: TestMarkeStehtNichtInDerDatenbank (0.10s)
  --- PASS: TestLoeschenNimmtGenauEine (0.07s)
  --- PASS: TestPruneRaeumtNurAltes (0.07s)
  --- PASS: TestBenutzerLoeschenRaeumtAblageMit (0.07s)
  --- PASS: TestWebsiteLoeschenLaesstAblageStehen (0.07s)
  --- PASS: TestAblageOhneWebsiteIstErlaubt (0.07s)
  PASS   ok  …/internal/csvimport   1.102s

ls internal/db/migrations/*.sql | wc -l                    → 49   (expects 49 ✓)
ls internal/db/migrations/ | sort | tail -1                → 00049_csv_imports.sql ✓
git diff --name-only -- internal/db/migrations/ | grep -c -v '00049…'  → 0 (expects 0 ✓ — but see note)
grep -rln BeginTx internal/csv/ internal/csvimport/ | wc -l → 0    (expects 0 ✓)
grep -rln BeginTx internal/ --include='*.go' | grep -v _test | wc -l → 14 (expects 14 ✓)
ls -d internal/*/ | wc -l                                  → 40   (expects 40 ✓)
grep -c 'csvimport.NewStore' internal/admin/handler.go     → 1    (expects 1 ✓)
go test ./...                                              → exit=0, 43 ok, 0 FAIL
```

### Task 2

```
go build ./... && go vet ./cmd/holzcloud/ && gofmt -l cmd/holzcloud/  → exit=0, no output
grep -c 'jobs.Job{' cmd/holzcloud/main.go        → 12  (expects 12 ✓)
grep -c 'csv-import-prune' cmd/holzcloud/main.go → 1   (expects 1 ✓)
sed -n "/csv-import-prune/,/^\t\t},/p" … | grep -c 'RunAtStart' → 1  (expects 0 ✗ — divergence 1)
go test ./cmd/holzcloud/ -run TestRouteAuthorization -v
  --- PASS: TestRouteAuthorization (0.39s)   PASS   ok  …/cmd/holzcloud  0.936s
go build ./... && go test ./...  → exit=0, 43 ok, 0 FAIL
git diff --stat cmd/holzcloud/main.go → 31 insertions, 0 deletions (the job and its import only)
```

### The counting table, baseline measured on the pre-change tree

| What | Command | Baseline | Plan adds | Predicted | **Measured** |
|---|---|---|---|---|---|
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | 48 | 1 | 49 | **49** ✓ |
| highest migration | `ls … \| sort \| tail -1` | `00048_…` | — | `00049_csv_imports.sql` | **`00049_csv_imports.sql`** ✓ |
| packages under `internal/` | `ls -d internal/*/ \| wc -l` | 39 | 1 | 40 | **40** ✓ |
| `BeginTx` files in `internal/` | `grep -rln … \| grep -v _test \| wc -l` | 14 | 0 | 14 | **14** ✓ |
| `BeginTx` in the two new packages | `grep -rln BeginTx internal/csv/ internal/csvimport/ \| wc -l` | 0 | 0 | 0 | **0** ✓ |
| `jobs.Job{` in `main.go` | `grep -c 'jobs.Job{' …` | 11 | 1 | 12 | **12** ✓ |
| `csvimport.NewStore` in `handler.go` | `grep -c …` | 0 | 1 | 1 | **1** ✓ |
| `RunAtStart` in the new job's block | `sed … \| grep -c 'RunAtStart'` | — | 0 | 0 | **1** ✗ — divergence 1 |
| `internal/csv` purity (wave 1's corrected form) | `go list -deps … \| grep holzcloud-cms \| grep -v 'internal/csv$' \| wc -l` | 0 | 0 | 0 | **0** ✓ |

Wave 1's corrected purity gate was used, not the original form its summary
identified as off by the package itself. It still measures 0 after this wave —
`internal/csvimport` did not leak into `internal/csv`.

## Divergences between the plan and the tree

### 1. `grep -c 'RunAtStart'` in the job block measures 1, not 0 — the gate counts mentions, not the field

**Direction:** measured one higher than the plan expects.
**Cause:** the gate is a text grep over the block, and the property it protects
is *the field not being set*. The single match is a **comment line** the plan's
own action text asked for:

```
// Kein RunAtStart, anders als bei media-backfill. Ein
// Aufraeumlauf beim Start wuerde genau den Import erwischen,
// der waehrend eines Deploys gerade laeuft.
```

The plan says in as many words: "**No `RunAtStart`** — `media-backfill` sets it
and this must not: sweeping on every restart would kill an import that is in
flight during a deploy", and asks the German comment to "carry the decision".
Carrying that decision requires naming the flag. The gate and the instruction
are in direct conflict.

The property itself holds, and the honest gate says so:

```
sed -n "/csv-import-prune/,/^\t\t},/p" cmd/holzcloud/main.go | grep -c 'RunAtStart:'   → 0
```

**The comment was not reworded to move the count.** Wave 1's summary states the
rule this follows: "rewriting prose to move a count is exactly the 'quiet
adjustment' this wave was told not to make." **The correct gate for a later wave
is `grep -c 'RunAtStart:'`** (with the colon — the field-set form), expecting 0.

This is the same class of defect wave 1 found twice, and it is now three times
in two waves: a counting gate whose command measures text where its name
measures structure.

### 2. `TestMigration00049RunterUndRauf` cannot live in `db_test.go` — the plan named the wrong file

**Direction:** a file the plan lists under `files_modified` was not modified;
another was modified instead.
**Cause:** `internal/db/db_test.go` is `package db_test` (external), and the
harness needs the **unexported** `migrationProvider`. The plan's own
`<read_first>` names `TestMigration00047RunterUndRauf` as the shape to copy —
and that test lives in `internal/db/migrations_down_test.go`, which is
`package db` precisely for this reason and says so in its head comment:

> *„Der Test liegt in package db und nicht in package db_test, weil er
> migrationProvider braucht: RunMigrations kennt nur den Weg nach oben."*

The alternative — exporting `migrationProvider` to satisfy the plan's file list
— would widen a package's API for a test's convenience. The test went beside its
two named siblings instead. `internal/db/db_test.go` is untouched.

### 3. `integrity-check` also writes `RunAtStart`, which the plan did not mention

The plan says `media-backfill` "is the one thing not to copy" and treats it as
the sole writer of the flag. There is a second: `cmd/holzcloud/main.go:572`
carries `RunAtStart: false` on the `integrity-check` job — an explicit false
rather than an omission. So "absent" is not the tree's only idiom for "does not
run at start"; writing it explicitly is also house style.

The field was **omitted** here, per the plan's acceptance criterion ("does
**not** carry `RunAtStart`"). Recorded because the choice was between two
existing idioms and the plan only knew about one.

### 4. `COALESCE(website_id, 0)` — right idiom, wrong attribution

The plan says to scan through `COALESCE(website_id, 0)` "the way the field store
already does it". The field store does not do it. The idiom is real and is used
in the tree — `internal/page/store.go:922` scans `COALESCE(u.email, '')` — so
the instruction was followed; only its citation was wrong.

### 5. `go test ./...` reports 43 packages `ok`, where wave 1's summary said 44

**Direction:** measured one *lower* than the previous wave, after adding a
package.
**Cause:** wave 1's number was itself off. The tree has 46 packages
(`go list ./... | wc -l` → 46), of which 3 print `[no test files]`, leaving 43
`ok`. Before this wave there were 45 packages and 42 `ok`. The gate's actual
criterion — non-zero exit or any `FAIL` line — is met: `exit=0`, `FAIL` count 0.
Noted so a later wave does not chase a phantom regression.

## Deviations from plan

### One process deviation

**The `internal/db/db_test.go` entry in `files_modified` was not honoured**;
`internal/db/migrations_down_test.go` was modified instead. See divergence 2 —
the plan's target file cannot reach the symbol its own acceptance criterion
requires.

### Two tests beyond the plan's list

The plan's `<behavior>` block ends with *"deleting the website leaves the row
with a NULL `website_id` rather than deleting it"*, but its list of test names
does not cover it. `TestWebsiteLoeschenLaesstAblageStehen` was added so the
`ON DELETE SET NULL` half is asserted and not merely written;
`TestAblageOhneWebsiteIstErlaubt` covers the `modus='neu'` row that has no
website at all. Both are Rule 2 (a stated guarantee with no gate is not a
guarantee).

### No auto-fixes were needed

Nothing in the existing tree was broken by this wave and nothing was fixed
outside it. `go test ./...` was green before and is green after.

### Nothing was committed that this plan does not own

Both commits were made with explicit pathspecs on **both** `git add` and
`git commit … -- <files>`. No `git add -A`, no `git commit -a`.
`git show --stat` for each commit lists only this plan's files, with **zero
deletions**:

```
d808942  internal/admin/handler.go                     |   5 +
         internal/csvimport/store.go                   | 197 +
         internal/csvimport/store_test.go              | 400 +
         internal/db/migrations/00049_csv_imports.sql  | 116 +
         internal/db/migrations_down_test.go           | 103 +
         5 files changed, 821 insertions(+)

ee870bc  cmd/holzcloud/main.go | 31 +
         1 file changed, 31 insertions(+)
```

The developer's `README.md`, `docs/`, `.github/`, `CONTRIBUTING.md` and
`SECURITY.md` never entered a commit. A pre-existing `git stash` entry
(`stash@{0}: WIP on gsd-reviewfix/07-97067`) was left untouched.

### On the "released migrations untouched" gate

`git diff --name-only -- internal/db/migrations/` measures 0, but it would
measure 0 whether or not anything had been done: `00049` was untracked at that
moment, and the gate reads only the working-tree diff of *tracked* files. The
assertion that actually carries weight was run after both commits:

```
git diff --name-only HEAD~2 HEAD -- internal/db/migrations/
  internal/db/migrations/00049_csv_imports.sql        ← the only one
```

`00046`, `00047` and `00048` are byte-identical to what they were. No correction
migration was written, so the count stayed at 49 and not 50.

## Known Stubs

None. Every symbol this plan declares is implemented and exercised by a test.

`admin.Handler.csvImports` is constructed and not yet read — that is by design
and stated in the plan: plan 09-04 is the first reader. An unused struct field
is not a compile error, and the alternative would put the constructor edit in
the middle of a handler task.

## Threat Flags

None beyond the plan's own register. The surfaces this wave opens are exactly
T-09-06 (ownership check, asserted by `TestFremdeMarkeWirdAbgelehnt`), T-09-07
(128-bit token from `crypto/rand`), T-09-09 (hash-only at rest, asserted by
`TestMarkeStehtNichtInDerDatenbank`), T-09-11 (`RunAtStart` absent), T-09-12
(cascade, asserted by `TestBenutzerLoeschenRaeumtAblageMit`) and T-09-13
(`BeginTx` measures 0). T-09-08 stays accepted as written.

The package opens no socket and fetches nothing. The 10 MB cap (T-09-10) is
plan 09-04's half and is **not** held by this wave.

## Commits

| Task | Commit | What |
|---|---|---|
| 1 | `d808942` | `feat(09-02): eine hochgeladene Tabelle ueberlebt vier Bildschirme` |
| 2 | `ee870bc` | `feat(09-02): der Aufraeumlauf, neben den drei Prunes die es schon gibt` |

## What wave 3 and later inherit

- **`00049` is released.** It is never edited; a correction is `00050` on every
  database that already ran it. `TestMigration00049RunterUndRauf` runs its
  `Down` half, which nothing else in the tree does.
- **The mapping travels as form state**, not in a column. Screen 2 is one
  `<form method="POST" action="…/probe">` and the row stepper is a submit button
  inside it with `formmethod="GET" formaction="/admin/csv-import/{token}"` —
  the idiom `page_form.html:136-137` and `:174` already use. **Adding a
  `zuordnung` column later would end IMP-08's concurrency dismissal**, which
  rests on the row being write-once.
- **Nothing may `UPDATE` a `csv_imports` row.** `Stage` writes it, `Holen`
  reads it, `Loeschen` and `Prune` remove it. That is the whole vocabulary, and
  D-30 stands on it.
- `admin.Handler.csvImports` is wired and non-nil. **Plan 09-04 reads it; no
  constructor argument is needed** and `NewHandler`'s signature is unchanged.
- Plan 09-05 calls `Loeschen` **after** the write loop and before the report
  renders, so a refresh finds no token. Deleting after rather than before is
  deliberate: a process that dies mid-write leaves the row intact.
- `ErrAbgelaufen` renders the named "this upload has expired, please start
  again" screen. `ErrFremd` is a refusal. **A handler that collapses them into
  one 404 undoes D-33.**
- The 10 MB `http.MaxBytesReader` cap and `MaxSpalten`'s refusal message on
  screen 1 are both still **plan 09-04's** and are not held yet.
- `BeginTx` in `internal/` is still **14** files, and `jobs.Job{` is now **12**.
- **The `RunAtStart` gate needs the colon** (`grep -c 'RunAtStart:'`) if it is
  re-run. See divergence 1.

## Self-Check: PASSED

```
FOUND: internal/db/migrations/00049_csv_imports.sql
FOUND: internal/csvimport/store.go
FOUND: internal/csvimport/store_test.go
FOUND: internal/db/migrations_down_test.go
FOUND: internal/admin/handler.go
FOUND: cmd/holzcloud/main.go
FOUND: d808942
FOUND: ee870bc
```
