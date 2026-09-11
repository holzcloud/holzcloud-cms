---
phase: 09-csv-import
round: gaps
closed: 2026-09-06
gaps_addressed: [T-09-14, T-09-30, criterion-3, IMP-05, IMP-08]
gaps_closed: 3
gaps_open: 0
found_while_closing: 1
commits: [7c4f034, 2ce851f, 776b194, 3beace1]
---

# Phase 9 — the gap round

Three gaps were named: the blocking one from the security audit and criterion 5,
the dry run's lie from criterion 3, and the default with no control from
warning 2. All three are closed. A fourth defect of the same family was found by
driving the screens afterwards and is closed too.

Nothing in this round is uncommitted except this file and the new §5 of
`deferred-items.md`.

---

## Gap 1 — the term pre-pass on the single write connection

**Closed.** `7c4f034`.

The audit's diagnosis was right in every part, including that the root cause is a
correctness bug and that fixing it is what closes the denial of service.

**The cap.** `TermNames` now calls `RowTerms`, so the two cannot carry the cap
separately and cannot drift again. Writing `term.MaxPerPage` a second time would
have fixed today's number and left tomorrow's; the doc comment on both functions
says that in as many words. A target is also harvested once even when two columns
are pointed at it, because `cellFor` reads only the first — the same defect one
step smaller.

**The batch.** `csvEnsureTerms` hands the names to `term.EnsureNames` in chunks of
`csvTermChunk = 500`. The number is measured, not chosen for looking round: an
`INSERT` of a name costs about 10 µs on this machine, so 500 is about 5 ms of held
connection, which is the same order as one row's own writes — and the row loop
already proved that grain of interleaving imperceptible (5000 pages, worst
competing wait 3.2 ms). Smaller batches buy nothing measurable and pay one
transaction each.

**Measured, same file, same method as the audit** — 10.00 MB, 398 data rows, a
competing writer polling the write pool every millisecond:

| | before | after |
|---|---|---|
| elapsed | 12.18 s | **3.54 s** |
| worst wait for one competing write | 8.08 s | **3.64 ms** |
| rows in `terms` | 796 000 | **4 776** = 398 × `MaxPerPage` |

Re-measured on the final tree after all four commits: 3.64 s / 3.86 ms / 4 776.
The stall is down by a factor of about 2 200 and now sits inside the band the
clean write loop occupies.

**Numbers differing from the audit's.** The audit reported 583 rows / 1 166 000
names / 10.75 s for its 10 MB file; mine is 398 rows / 796 000 names / 12.18 s.
Same shape, different fixture — the audit's rows evidently carried fewer names
each. The contrast it drew is reproduced exactly.

**The orphans.** The cap prevents the unbounded kind and cannot prevent all of
them. Two bounded shapes survive: a row refused *after* its names were harvested
(the pre-pass runs before `CheckRow`), and a label a later row takes away on the
update arm — the second being ordinary `SetForPage` behaviour and not the
importer's at all. Existing installations **can** already carry the unbounded
kind, because every import run before this fix harvested uncapped. No migration
was written: a sweep cannot tell an orphan from a label an editor made by hand
and has not used yet, so it belongs as an admin action a person confirms.
Written up as `deferred-items.md` §5 with the measurement behind each claim.

---

## Gap 2 — the dry run predicting what the write does not do

**Closed.** `2ce851f`.

The honest answer turned out to be that the dry run **can** know, so it now does
rather than apologising. It walks the file in order, so it remembers the addresses
rows above the current one will take and hands `CheckRow` the page that row is
about to create. Only `Slug` is read from that stand-in, because nothing else
about a page that does not exist yet could be said honestly, and the address is
recorded from the **verdict** rather than from the mapping, so a row refused for
its title leaves the address free for the row below it.

`TestCSVProbeAndStartAgreeOnEveryVerdict` carries the duplicate address now — row
6 repeating row 2 — and runs on **both** collision answers, because the two arms
answer a taken address differently and only one of them had ever been measured.

The screen says it too, since "1 anlegen / 1 aktualisieren" on a website that does
not exist yet is otherwise unexplainable.

**The term pre-pass in the dry run.** It ran only on the write arm (`if write`), so
the screen that promises to show what will happen showed none of the labels. The
harvest now runs on both arms and the creation on one: the dry run counts them,
says how many, and `TestCSVDryRunNamesTheLabelsAndTheRepeatedAddress` asserts the
`terms` table is still empty afterwards. The count is deliberately "as far as they
do not exist yet" — knowing which already exist would cost a query per name, and
that is the honest sentence a run which reads nothing can write.

---

## Gap 3 — a default the test proves and the operator cannot reach

**Closed.** `776b194`. **The tree corrected me on the shape of the fix.**

I set out to add the body control and to take the fallback away from title and
address, on the argument that a default there gives every row one identity.
`TestRowWithoutATitleIsSkipped` then failed: it asserts a title default applies
and cites IMP-08 while doing it. Removing the fallback would have narrowed an
existing green test to fit a missing control — the exact move the gap brief
forbids, arrived at from the other direction.

So every target but "nichts" takes a default, `csvimport.TakesDefault` is the one
rule, and its two readers are `cellFor` and the mapping screen. The card now
carries **Titel, Adresse, Text, Zustand, Schlagwörter** and one box per field.
The foot-gun is stated in a hint beside the two boxes rather than prevented, and
the dry run now predicts its consequence correctly, so the operator is shown what
will happen instead of being protected from a box the phase says should work.

`TestCSVMappingShowsEveryDefaultItAccepts` walks every target and fails in **both**
directions — a target that accepts a default with no box, and a box for a target
that cannot use one — so the next target added cannot repeat this.

**One existing test was reworked, and it is worth naming.**
`TestCSVMappingCutsAnOversizedSample` measured "the response is smaller than the
cell", which is only the same statement as its own title while the page stays
under 10 kB; four more boxes put it at 10 066 bytes for a 10 000-byte cell. It now
measures the difference between the same screen rendered with a huge cell and with
a short one. Verified still to catch the defect it was written for: with the cut
removed it reports 9 996 bytes of growth. This is a test measuring what it names
more closely than before, not a test narrowed to fit.

---

## Found while closing, not in either report

**Closed.** `3beace1`. Found by driving the screens, which is why that pass is not
optional.

A file with no `Schlagwörter` column and `importiert|hofladen` typed into the
Vorgaben box gap 3 had just added: the dry run promised two labels and four were
created. Two halves of one rule, each broken the other way:

1. `TermNames` walked the mapping's **columns**, and a default-only target has
   none — while `RowTerms` goes through `cellFor`, which falls back to the default
   whether a column exists or not. The same two functions disagreeing again, one
   step past where gap 1's cap fixed them.
2. `setTerms` then discarded what the operator typed, because it asked only
   whether a **column** was mapped. That is the right question for a blank cell —
   a file that says nothing about terms must not clear them — and the wrong one
   for a value stated on the screen. A default is the file speaking, which is the
   rule the status, the body and every field already follow.

Fixing only the first half would have created the labels and attached them to
nothing: an orphan made on purpose, the very defect the audit named, manufactured
by its own fix.

---

## What the two reports got wrong

Little, and nothing that changed a verdict.

- **`09-SECURITY.md`, "cap the pre-pass … either alone reduces the stall by
  orders of magnitude."** Only half true as stated. The cap alone reduces the
  *work*; the batch alone reduces the *held connection*. On the measured file the
  cap takes 796 000 names to 4 776 and the batch takes what is left from one
  transaction to ten — both are needed to write down "no transaction spans more
  than one row" without a footnote, which is what the audit's own "both together
  close the threat" says two sentences later.
- **`09-VERIFICATION.md`, criterion 3, "the dry run cannot know about pages the
  same run is about to create."** It can. It walks the file in order and had
  simply not been asked to remember. `deferred-items.md` §4 repeats the same
  claim and is now superseded by §5's neighbour — the divergence it describes no
  longer exists.
- **`09-VERIFICATION.md`, warning 2, "defensible against the wording (each FIELD
  can carry a default)."** The narrower reading does not survive the tree:
  `TestRowWithoutATitleIsSkipped` asserts a **title** default applies and cites
  IMP-08. The phase had already decided the broad reading; only the screen had
  not been told.
- **Measurement fixtures differ** (see gap 1). Not an error, but the audit's
  583 rows / 1 166 000 names is not reproducible from the file shape it
  describes; the contrast is.

Everything else in both documents checked out against the code, including every
line number cited.

---

## Gates

```
go build ./...   silent
go vet ./...     silent
gofmt -l .       silent
go test ./...    exit 0
go run ./tools/i18n
  1277 Zeichenketten im Quelltext
  de-CH.json   73 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
  en.json      1277 übersetzt, 0 offen, 0 verwaist
  es.json      1277 übersetzt, 0 offen, 0 verwaist
  fr-CH.json   4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
  fr.json      1277 übersetzt, 0 offen, 0 verwaist
  it-CH.json   9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
  it.json      1277 übersetzt, 0 offen, 0 verwaist
```

1271 → 1277 is six new sentences, all translated; `-schweiz` was run and de-CH
needs no variant for any of them (none carries an ß).

---

## What was driven, and what was not

Driven in a real browser — the Chromium Playwright had already cached, over the
DevTools protocol, against the built binary on a fresh database — twice: once
while finding the fourth defect and once from an empty database afterwards, so
the screenshots below are of the shipped tree and not of an intermediate one.

| screen | seen |
|---|---|
| 1 — the panel | `g-01`, `01-panel` |
| 2 — the mapping, new website | `02-mapping` — the Vorgaben card carries Titel, Adresse, Text, Zustand, Schlagwörter |
| 2 — the mapping, existing website with two fields | `05-mapping-with-fields` — all seven boxes, `default_title … default_field:kategorie` |
| 3 — the dry run | `03-dryrun` — `2 anlegen / 1 aktualisieren / 2 übergehen`, "5 Schlagwörter werden dabei angelegt", and the repeated-address sentence |
| 3 — the dry run with defaults | `06-dryrun-defaults` — "4 Schlagwörter" after the fourth fix, "2" before it |
| 4 — the report | `04-report` — `2 angelegt / 1 aktualisiert / 2 übergangen`, the same two reason groups in the same order |
| 4 — the report after the defaults | `07-report-defaults` |

**The two screens now agree, driven end to end.** The dry run said
`2 anlegen / 1 aktualisieren / 2 übergehen` and the write did
`2 angelegt / 1 aktualisiert / 2 übergangen` on the exact file shape the
verification measured as `5 / 2` against `4 / 3`.

**Checked in the database afterwards, not only on the screen:**
- the body default reached the pages — `zwetschge` and `quitte` carry
  `Aus der Vorgabe`, through a box that did not exist before this round;
- the field default reached them — `{"werte":{"farbe":"unbekannt", …}}`;
- the terms default reached them — `importiert` and `hofladen` each point at two
  pages, which is the fourth fix proved rather than asserted;
- nine labels exist, and the only ones nothing points at are `gemuese` (from the
  refused `Gurke` row) and `rot` (taken away by the update) — the two bounded
  shapes `deferred-items.md` §5 describes, and no third one.

**Not driven:** the 10 MB contention case. It is a measurement, not a screen —
run as a Go harness against a real migrated database through `db.Open`, the same
way the audit ran it, and the harness was deleted rather than committed. The
human-verification item in `09-VERIFICATION.md` asked for a second browser
context during a large-vocabulary commit; what stands in for it is the 3.86 ms
worst-wait number, which is a stronger statement than "the page felt responsive"
and is reproducible.

**Also not driven:** the four non-German catalogues. The six new sentences were
translated by hand and the gate reports them present; no screen was rendered in
en, es, fr or it this round, unlike the phase's own browser pass.
