---
phase: 11-galerie
plan: 02
subsystem: storage
tags: [album, migration, website-scoping, sqlite, gal-05]
status: complete

requires:
  - internal/db (Open, RunMigrations, dual pool)
  - internal/page (Slugify)
  - internal/block (Item)
provides:
  - migration 00050 — albums, album_items, idx_album_items_album_sort
  - internal/album — Album, Picture, MaxNameLength, MaxItems
  - internal/album.Store — website-scoped CRUD, ordered pictures, SwapSortOrder
  - named errors — ErrNotFound, ErrDuplicateName, ErrNoName, ErrTooManyItems, ErrForeignMedia
affects:
  - 11-03 (admin screens call this store; nine routes, none in this wave)
  - 11-06 (bundle round trip calls Create and Rename; GAL-04)
  - 11-05/11-07 (a page that names an album renders Items through the gallery arm)

tech-stack:
  added: []
  patterns:
    - "website_id in the WHERE clause of every statement, term-shaped, not menu-shaped"
    - "an item statement reaches the website through a subquery on albums"
    - "RowsAffected() == 0 is a named error, in all seven mutating places"
    - "order changes by swapping two rows in one transaction, never by rewriting the list"
    - "a UNIQUE violation is detected by SQLite's extended result code, not by matching its message"

key-files:
  created:
    - internal/db/migrations/00050_albums.sql
    - internal/album/album.go
    - internal/album/store.go
    - internal/album/store_test.go
  modified:
    - internal/db/migrations_down_test.go

decisions:
  - "An album has a slug, and Rename never moves it — GAL-04's round trip needs a stable key on the stored side"
  - "Delete is stricter than term.Delete: RowsAffected() == 0 is an error, because a silent no-op reports success for a delete that never happened"
  - "The picture list is []block.Item and not a parallel type — GAL-07's one mechanism means one list, not two that agree"
  - "MaxItems = 120: five times a block's cap, because an album is reusable and expanded on the read path of every page that names it"
  - "ErrNotFound does not distinguish 'no such album' from 'another website's album' — which of the two it was is itself information about another website"
  - "Pictures() was added beside Items(): a block.Item carries no row id, and 11-03's reorder and delete controls need one"

metrics:
  duration: ~50 min
  completed: 2026-09-07
  tasks: 3
  commits: 3

actuals:
  tokens: 13034
  tasks: 3
  commits: 3
---

# Phase 11 Plan 02: Migration 00050 and internal/album Summary

A website-scoped album store where GAL-05 is a fact of the SQL rather than a
convention among handlers: `website_id` stands in the WHERE clause of all
sixteen statements, and zero affected rows is a named error in all seven
mutating places.

## What was built

**`internal/db/migrations/00050_albums.sql`** — `albums` (website-scoped parent,
`UNIQUE (website_id, slug)`, `INTEGER PRIMARY KEY AUTOINCREMENT`, `STRICT`) and
`album_items` (flat ordered child carrying exactly the three things `block.Item`
carries, plus identity and `sort_order`), with one index on
`album_items(album_id, sort_order)` — the single question every page carrying an
album asks on every request. The down half takes the index, then the child, then
the parent, and its paragraph says why that order and not the other.

**`internal/album`** — `Album`, `Picture`, `MaxNameLength = 60`,
`MaxItems = 120`, and a `Store` with term-shaped signatures (`websiteID`
immediately after the context on every method) and menu-shaped ordering
(`SwapSortOrder`: `BeginTx`, two reads, two crossed writes, `defer tx.Rollback()`,
`return tx.Commit()`).

The four item methods reach the website through a subquery on `albums`, because
`album_items` has no `website_id` of its own — which is precisely the reason
`internal/menu/store.go:14-33` gives for not having been fixed the safer way
after the 2026-09-06 cross-website write. It is paid here while the table is new.
Four copies of one clause, deliberately, and the package comment says so: a
builder would hide the one thing a reader must see at a glance, which is that
the clause is there every time.

## Task-by-task

| Task | What | Commit |
|---|---|---|
| 1 | Migration 00050 + `TestMigration00050DownAndUp` | `a3e5780` |
| 2 RED | Fourteen failing tests, `album.go` model and bounds | `25eb648` |
| 2 GREEN | `store.go` — the whole store, all gates green on the first run | `1ad5d11` |
| 3 | The counting table below. No code; produced this file. | — (uncommitted per instruction) |

## The counting gates, measured against the post-change tree

Baselines were re-measured on this tree before any change and matched the plan
exactly, all seven.

| What | Baseline | Plan predicted | Measured after | Verdict |
|---|---|---|---|---|
| migrations | 49 | 50 | **50** | as predicted |
| packages under `internal/` | 40 | 41 | **41** | as predicted |
| files using `BeginTx` | 14 | 15 | **15** | as predicted, and on purpose |
| admin templates | 66 | 66 | **66** | unchanged, as required |
| `layoutPageNames` entries | 48 | 48 | **48** | unchanged, as required |
| `adminProtectedMux.Handle` | 150 | 150 | **150** | unchanged, as required |
| strings in source (`tools/i18n`) | 1277 | 1277 | **1280** | **diverged, +3 — cause below** |

**The `BeginTx` row moved on purpose.** One transaction, in `SwapSortOrder`,
opened and committed in the same function. The count is not the gate for it: the
gate is `TestSwapDoesNotHoldTheWriteConnection`, which runs forty swaps, forty
refused swaps and forty ordinary writes under a two-second deadline — well below
the 5000 ms `busy_timeout`. Phase 9 shipped a `BeginTx` count that read zero
while a ten-second write-connection stall sat one package away, and a count
proves a proxy. This one was mutation-tested: removing `defer tx.Rollback()`
makes it fail in 2.09 s at exactly the predicted line, `the write after swap 0
did not get the connection: context deadline exceeded`. It was then restored and
re-run green.

### The one divergence: 1280 instead of 1277, +3

**Direction:** three more strings than the plan predicted.

**Cause: not this plan.** Measured directly rather than argued — moving
`internal/album` out of the tree and re-running the tool gives **1280 either
way**, so this package contributes zero strings. The three are plan 11-01's
lightbox control labels, added to `internal/block` in commits `c432bba` and
`a3b49c9` while both wave-1 plans ran sequentially in this same working tree:

```
i18n.N("Vorheriges Bild")
i18n.N("Nächstes Bild")
i18n.N("Grossansicht schliessen")
```

**What the plan got wrong, and it is worth writing down:** the gate's intent —
"`internal/album` mints no sentence an operator reads, because the wording
belongs in plan 11-03's templates" — holds exactly, and is now proved more
directly than the gate could have. The gate's *threshold* was unsatisfiable as
written, because it names a whole-tree number while two plans of the same wave
run in one tree. A number that is only correct in isolation cannot be asserted
about a shared tree. The honest form of this gate is the isolation measurement:
*removing this package must not change the count.* Nothing was adjusted to make
1277 appear; the measurement stands as it came out.

`tools/i18n` reads only the arguments of `goFuncs` (`SetFlashError/Success/Warning`,
`Add`, `NewLayoutData`, `Titlef`, `T`, `N`), and `internal/album` calls none of
them — its errors are `errors.New` and `fmt.Errorf` values for developers.

## Verification — literal output

```
$ ls internal/db/migrations/*.sql | wc -l                                    50
$ git status --porcelain internal/db/migrations/ | grep -v 00050 | wc -l      0
$ grep -cE 'INTEGER PRIMARY KEY AUTOINCREMENT' .../00050_albums.sql           2
$ python3 ... parent_id|location_key (comment lines stripped)                 0
$ grep -c 'STRICT' .../00050_albums.sql                                       2
$ python3 ... album_items before DROP TABLE IF EXISTS albums;              True
$ go test ./internal/db/ -run 'Migration|RunMigrations|CurrentVersion' -v
    --- PASS: TestMigration00047RunterUndRauf / 00048 / 00049DownAndUp
    --- PASS: TestMigration00050DownAndUp
    --- PASS: TestRunMigrations / Idempotent / CurrentVersionMatchesMigrations
$ go test ./internal/db/ -run TestMigration00050DownAndUp -v | grep -c PASS    1

$ go build ./... && go vet ./internal/album/ && gofmt -l internal/album/   exit 0, no paths
$ go test ./internal/album/ -v          14 tests, --- PASS on every one, ok 1.273s
$ python3 ... SQL statements / unscoped                                   16 0  []
$ python3 ... RowsAffected()                                                   7
$ python3 ... page.Slugify(                                                    1
$ python3 ... UPDATE albums SET slug                                           0
$ python3 ... BeginTx( , tx.Commit()                                         1 1
$ go test ./internal/album/ -run TestSwapDoesNotHoldTheWriteConnection -v
    --- PASS: TestSwapDoesNotHoldTheWriteConnection (0.09s)

$ go build ./... && go vet ./... && gofmt -l . && go test ./...
    build OK, vet OK, gofmt printed nothing, test exit 0 — no FAIL line
```

Sixteen statements, zero unscoped. Seven `RowsAffected()` where the plan asked
for at least six: `Rename`, `Delete`, `AddItem`, `UpdateItem`, `DeleteItem`, and
both writes of the swap.

## Deviations from Plan

**1. [Rule 2 — missing critical functionality] `requireOwnMedia`, and
`ErrForeignMedia` beside it**

- **Found during:** Task 2, reading the threat register before writing `AddItem`.
- **Issue:** T-11-07 assigns `mitigate` to "AddItem with a media id from another
  website's library" — the foreign key on `media_id` proves the file exists and
  never whose it is, and the id comes out of a form. The method list in the
  action did not spell the check out.
- **Fix:** `AddItem` and `UpdateItem` both call `requireOwnMedia` first;
  `TestAddItemRefusesAnotherWebsitesMedia` holds both directions (adding a
  foreign picture, and repointing an own picture at a foreign one).
- **Files:** `internal/album/store.go`, `internal/album/store_test.go`
- **Commit:** `1ad5d11`

**2. [Rule 2 — missing critical functionality] `Pictures()` beside `Items()`**

- **Found during:** Task 2, writing the swap test.
- **Issue:** the plan's signature list exposes the picture list only as
  `[]block.Item`, and a `block.Item` carries no row id. Plan 11-03's reorder and
  delete controls need one for every row, and so does any test that asserts an
  order rather than a sequence. Without it, 11-03 would have to write a second
  query over `album_items` — the second spelling GAL-07 forbids.
- **Fix:** `Pictures(ctx, websiteID, albumID) ([]Picture, error)` is the single
  query; `Items` is a projection of it and not a second statement, so the two
  cannot come back in a different order. `Picture` carries `block.Item` rather
  than repeating its three fields.
- **Files:** `internal/album/album.go`, `internal/album/store.go`
- **Commit:** `1ad5d11`

**3. [documented choice] Two tests beyond the twelve the plan names**

`TestAddItemRefusesAnotherWebsitesMedia` (deviation 1) and
`TestNameOfOnlySpacesIsRefused` (the last line of the plan's `<behavior>` list,
which had no test named for it). All twelve named tests exist, English, one
property each.

**4. [instruction, not a deviation] No commit for task 3, and STATE.md untouched**

Task 3 produces no code; its output is the table above. Per the executor
instruction this summary stays uncommitted, and `.planning/STATE.md` was not
written at all — two plans of one wave ran sequentially in this tree, and both
advancing the plan counter would double-count. The orchestrator owns that.

## What the plan got wrong about the tree

1. **The i18n gate's threshold**, above. Everything else in the counting table
   matched to the digit, including all seven baselines.
2. **`internal/menu/menu.go` does not exist** — the plan's `read_first` for task
   2 names it as "the model file beside a store, for the shape of `album.go`".
   The file is `internal/menu/model.go`. Read that instead; no consequence.
3. **`internal/block/block.go:165-175`** is `MaxBlocks` and `MaxItems` at
   `:165-173`, and `Item` is at `:219-226` rather than `:220-227`. Off by a line
   or two in the same paragraph; the reference was unambiguous.

## Known Stubs

None. No placeholder, no `TODO`, no `t.Skip`, no unrun `<verify>`. Every gate in
the plan was run and its literal output is above.

## Threat Flags

None. `internal/album` opens no route, reads no request and imports no network
package. The two surfaces the register named — a media id from a form (T-11-07)
and a transaction on the single write connection (T-11-09) — are mitigated and
tested, and `go.mod` is untouched (T-11-SC does not fire).

## Self-Check: PASSED

```
FOUND: internal/db/migrations/00050_albums.sql
FOUND: internal/album/album.go
FOUND: internal/album/store.go
FOUND: internal/album/store_test.go
FOUND: internal/db/migrations_down_test.go
FOUND commit: a3e5780  feat(11-02): migration 00050 ...
FOUND commit: 25eb648  test(11-02): failing tests ...
FOUND commit: 1ad5d11  feat(11-02): the album store ...
```

Each commit touches only this plan's `files_modified`, confirmed by
`git show --stat`: `a3e5780` two files, `25eb648` two files, `1ad5d11` one file.
Nothing of plan 11-01's, nothing of the developer's.

## TDD Gate Compliance

Task 2 carried `tdd="true"` and the gate sequence is in the log in order:
`test(11-02)` at `25eb648` (RED — the suite failed to build against a package
with no `Store`, and `go build ./...` stayed green so the parallel executor's
tree was not broken), then `feat(11-02)` at `1ad5d11` (GREEN — fourteen tests
passing). No REFACTOR commit: nothing needed cleaning up.
