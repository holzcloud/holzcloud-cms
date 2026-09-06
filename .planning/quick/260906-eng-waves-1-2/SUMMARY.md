---
task: Translate Phase 9 waves 1 and 2 from German to English
date: 2026-09-06
status: complete
commits:
  - 631d881 refactor(09-01) internal/csv speaks English
  - ba7074d refactor(09-02) internal/csvimport speaks English, and the four callers of wave 1
---

# Waves 1 and 2 in English

## Why this was cheap today

Waves 1 and 2 landed hours ago and nothing imports their names yet: `grep -rn
'internal/csv"'` returned nothing at all, and `internal/csvimport` was reached
from exactly three places, two of which only spell `NewStore` and `Prune`.
Waves 3 to 6 are about to spell `Reader`, `Row`, `Next`, `RowNumber`, `Get` and
`Delete` across four handlers, a dry run and a report. One commit now, four
commits later.

## What changed

**`internal/csv`** (commit `631d881`) — the pure half.

| was | is |
|---|---|
| `Leser` / `Zeile` | `Reader` / `Row` |
| `Kopf()` `Naechste()` `Abgeschnitten()` | `Header()` `Next()` `Truncated()` |
| `Neu` `PruefeBytes` `Zeilennummer` `MaxSpalten` | `New` `CheckBytes` `RowNumber` `MaxColumns` |
| `fuellen` `spaltenname` `istParseFehler` | `fill` `columnName` `isParseError` |
| `ErrLeer` `ErrNurBOM` `ErrNullbyte` | `ErrEmpty` `ErrOnlyBOM` `ErrNULByte` |
| `ErrKeineKopfzeile` `ErrZuVieleSpalten` | `ErrNoHeader` `ErrTooManyColumns` |
| `Zeile.Nummer/.Zellen/.Fehler` | `Row.Number/.Cells/.Error` |
| `beispiel.go` `Beispiel` `entschaerfen` | `example.go` `Example` `defuse` |

Both renames went through `git mv`. Sixteen test functions kept the German
names' habit of naming the assertion rather than the function:
`TestZeilennummerIstDieDerTabelle` → `TestRowNumberIsTheSpreadsheetsOwn`,
`TestZweiTabsUeberlebenEinander` → `TestTwoTabsSurviveEachOther`.

The `Reader` struct's `r *stdcsv.Reader` field became `cr`, because the receiver
is now `r` and the two would have collided.

**`internal/csvimport`** (commit `ba7074d`) — the half with the database.
`Ablage`→`Upload`, `Holen`→`Get`, `Loeschen`→`Delete`, `hashMarke`→`hashToken`,
`ErrAbgelaufen`→`ErrExpired`, `ErrFremd`→`ErrForeign`, and the five fields
`Modus`/`Kollision`/`Dateiname`/`Daten`/`ErstelltAm` →
`Mode`/`Collision`/`Filename`/`Data`/`CreatedAt`. Test helpers `aufbau`,
`benutzer`, `musterAblage` → `setup`, `user`, `sampleUpload`.

Also in that commit, because each names wave 1's code and would otherwise point
at something that no longer exists: the `csv-import-prune` comment block in
`cmd/holzcloud/main.go`, `TestMigration00049RunterUndRauf` →
`TestMigration00049DownAndUp` in `internal/db/migrations_down_test.go`, and the
four-line cross-reference in `plugins/kontaktformular/csv.go` — which now points
at `internal/csv/example.go`.

Comments were translated, not compressed. Every paragraph that carried an
argument still carries the same argument at the same length: why `MaxCellBytes`
counts bytes and not runes, why the row number is not a line number, why
`user/token.go:59-62` was deliberately *not* copied, why the sweep has no
`RunAtStart`, why the defusing rule is written down twice.

## Gates — literal output

```
$ go build ./... && go vet ./... && gofmt -l .
exit=0                                    (no output from any of the three)

$ go test ./internal/csv/ ./internal/csvimport/ -v
--- PASS: TestBOMIsStrippedOnce (0.00s)
--- PASS: TestRowNumberIsTheSpreadsheetsOwn (0.00s)
--- PASS: TestRowLimit (0.00s)
--- PASS: TestColumnLimit (0.00s)
--- PASS: TestCellLimitInBytes (0.00s)
--- PASS: TestShortRowDoesNotShift (0.00s)
--- PASS: TestStrayQuoteDoesNotEatTheRest (0.00s)
--- PASS: TestQuotedCellWithSeparatorAndLineBreak (0.00s)
--- PASS: TestBlankLineDoesNotCount (0.00s)
--- PASS: TestCheckBytesRefuses (0.00s)
--- PASS: TestHeaderOnlyIsNotAnError (0.00s)
--- PASS: TestNoHeaderIsAnError (0.00s)
--- PASS: TestDefuseTakesTheMeaningOutOfFormulas (0.00s)
--- PASS: TestExampleWritesTheBOM (0.00s)
--- PASS: TestExampleDoesNotDoItsOwnQuoting (0.00s)
--- PASS: TestExampleWithoutSampleRows (0.00s)
ok  	github.com/holzcloud/holzcloud-cms/internal/csv
--- PASS: TestUploadComesBackByteForByte (0.09s)
--- PASS: TestForeignTokenIsRefused (0.07s)
--- PASS: TestUnknownTokenIsExpired (0.07s)
--- PASS: TestTwoTabsSurviveEachOther (0.07s)
--- PASS: TestTokenIsNotInTheDatabase (0.07s)
--- PASS: TestDeleteTakesExactlyOne (0.07s)
--- PASS: TestPruneSweepsOnlyTheOld (0.07s)
--- PASS: TestDeletingAUserTakesTheUploadWithIt (0.07s)
--- PASS: TestDeletingAWebsiteLeavesTheUploadStanding (0.07s)
--- PASS: TestUploadWithoutAWebsiteIsAllowed (0.07s)
ok  	github.com/holzcloud/holzcloud-cms/internal/csvimport

$ go test ./...
exit=0, no FAIL line

$ grep -rn '[äöüÄÖÜß]' internal/csv/ internal/csvimport/
exit=1                                    (nothing printed)

$ go run ./tools/i18n
1158 Zeichenketten im Quelltext
de-CH.json   55 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1158 übersetzt, 0 offen, 0 verwaist
es.json      1158 übersetzt, 0 offen, 0 verwaist
fr-CH.json   4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1158 übersetzt, 0 offen, 0 verwaist
it-CH.json   9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1158 übersetzt, 0 offen, 0 verwaist

$ go list -deps ./internal/csv/ | grep holzcloud-cms | grep -v 'internal/csv$' | wc -l
0
```

Test counts were measured before the work started: 16 in `internal/csv`, 10 in
`internal/csvimport`. Both numbers are unchanged, and the numbers in the task
were right.

## Glossary

Twenty-nine entries added across the two commits, plus one existing row
sharpened.

Commit 1: `Zelle`→cell, `Nullbyte`→NUL byte, `Grenze`→limit, `Fehler`→error,
`Abwehr`→defence, `Wache`→guard, `Musterzeile`→sample row,
`Überschrift`→heading, `Trennzeichen`→separator, `Anführungszeichen`→quote,
`Zeilenumbruch`→line break, `Tabellenprogramm`→spreadsheet program,
`Ausfertigung`→copy, `Angriffsfläche`→attack surface, `Bediener/in`→operator.

Commit 2: `Modus`→Mode, `Kollision`→Collision, `Dateiname`→Filename,
`Daten`→Data, `erstellt am`→CreatedAt, `fremd`→foreign, `abgelaufen`→expired,
`Benutzer`→user, `Konto`→account, `Rückwärtshälfte`→down half,
`Rücknahme`→rollback, `Rückfahrt`→trip back up, `Aufbewahrung`→retention,
`Gegenstand (im Schema)`→object.

**The row that was sharpened.** `Ablage | staging` now reads: the concept is
`staging`, but the *type* that is one of them is `Upload`. See below.

## Where the naming map needed a second look

**1. `Ablage` → `Upload` and the glossary both stand, at different scopes.**
The task's map says the type becomes `Upload`; the glossary says `Ablage` is
`staging`. Read as a contradiction one of them has to lose. Read as scope they
both hold, and the code already assumed it: `Stage`, `NewStore`, and every
English comment wave 2 shipped already say "staged upload" and "staging store".
So: the operation and the concept are `staging`, the struct holding one file is
`Upload`. The glossary row now says this in one clause, because the next person
translating `Zwischenablage` needs it settled and not re-derived.

**2. The map is silent on the four values that a CHECK constraint pins.**
`modus IN ('neu','bestehend')` and `kollision IN ('uebergehen','aktualisieren')`
live in migration 00049, which is out of scope and correctly so. The glossary
lists `übergehen`→`skip` and `aktualisieren`→`update`, which reads like an
instruction to translate them — and translating either one produces a Go
program that writes a row SQLite refuses, at runtime, on a path no unit test in
this phase exercises. They are data, not identifiers. Left as they are, and a
paragraph at the head of `csvimport` now says so, along with which German
column each English field maps to. This is the one thing here that could have
become a production bug rather than a compile error.

**3. Fixture strings had to turn too, which the map does not mention.**
The umlaut gate forces it: `"Grösse"` in `TestBeispielSchreibtDieBOM` and
`"Prüfsite"` in `aufbau` cannot survive `grep -rn '[äöüÄÖÜß]'`. Having to turn
two of them, I turned all of them for consistency — `Titel`→`Title`,
`Stuhl`→`Chair`, `zwei,drei`→`two,three`, `produkte.csv`→`products.csv`.

**4. One comment lost a concrete example and gained a general one.**
`beispiel.go` explained the BOM with "without it a Ü becomes an Ãœ". Both
characters are umlauts and would fail the gate. It now reads "an accented
character arrives as mojibake" — same reasoning, same length, one degree less
vivid. Flagging it because it is the only place where the gate cost something.

**5. `internal/admin/handler.go` needed nothing.** The two lines wave 2 added
(`csvImports *csvimport.Store` and its comment) were already English.

## One thing to know about the history

`git mv` was used for both renames, but git detects renames by content
similarity at read time, and these files changed by more than half. At the
default 50% threshold `git log --follow internal/csv/example.go` stops at this
commit; at `-M30%` it reaches back to `5a1b5f1`, where the file was created:

```
$ git log --follow -M30% --oneline -- internal/csv/example.go
631d881 refactor(09-01): internal/csv speaks English
5a1b5f1 feat(09-01): die Beispieldatei, und die zweite Ausfertigung einer Regel
```

The lineage is intact, it just needs a lower threshold to be seen. A
rename-only commit followed by a content commit would have avoided this; the
two-commit shape asked for here does not allow it. Worth knowing for Phase 12,
which renames far more files than two: **rename in one commit, translate in the
next**, and `--follow` works at any threshold.

## Explicitly not done

`internal/db/migrations/00049_csv_imports.sql` was not touched. It is a
released migration that has already run on a database; editing it would give
two installations two different schemas under one version number. Its German
comments, its German column names and its four German CHECK values are Phase
12's work, behind the version jump the roadmap already schedules for exactly
this. The head comment of `internal/csvimport` now names that phase, so the
mismatch between `Mode` and `modus` reads as a scheduled state rather than as
an oversight.

`internal/db/migrations_down_test.go` still holds `TestMigration00047RunterUndRauf`
and `TestMigration00048RunterUndRauf` beside the English
`TestMigration00049DownAndUp`. That file is now mixed-language, which is
correct for this task's scope and wrong as an end state — Phase 12 closes it.
