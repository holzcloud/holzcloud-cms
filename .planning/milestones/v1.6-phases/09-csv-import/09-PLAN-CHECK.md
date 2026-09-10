# Phase 9: CSV Import — Plan Check

**Checked:** 2026-09-06, before execution
**Plans:** `09-01-PLAN.md` … `09-06-PLAN.md`
**Verdict:** **ISSUES FOUND** — 6 blockers, 6 warnings. Five of the six blockers
sit in one place: the hand edit that moved the example CSV off the staging token
(D-37) landed in the prose of `09-04` but not in three of its executable gates,
and `09-05`'s two CSS gates were written from numbers nobody had measured.

**Verdict per plan**

| Plan | Wave | Verdict | Blockers | Warnings |
|---|---|---|---|---|
| 09-01 | 1 | **PASS** — no defect found | 0 | 0 |
| 09-02 | 2 | **PASS with a warning** | 0 | 1 |
| 09-03 | 3 | **PASS with warnings** | 0 | 2 |
| 09-04 | 4 | **BLOCKED** — D-37 leftovers in three gates and a task-ordering defect | 4 | 2 |
| 09-05 | 5 | **BLOCKED** — two counting gates cannot pass as written | 2 | 1 |
| 09-06 | 6 | **PASS** — no defect found | 0 | 0 |

---

## 1. Baselines: every number in `09-CONTEXT.md` re-measured, all correct

Measured against the working tree before writing this file. The "Baseline counts"
table is trustworthy and the plans may rest on it:

| What | CONTEXT says | Measured | |
|---|---|---|---|
| migrations | 48, highest `00048_snippet_group_namespace.sql` | 48, same file | ✓ |
| `adminProtectedMux.Handle` | 145 | 145 | ✓ |
| `adminOnly` rows | 14 | 14 | ✓ |
| `<details` panels | 2 | 2 | ✓ |
| `jobs.Job{` | 11 | 11 | ✓ |
| admin templates | 61 | 61 | ✓ |
| `layoutPageNames` | 44 | 44 | ✓ |
| packages under `internal/` | 38 | 38 | ✓ |
| files using `BeginTx` | 14 | 14 | ✓ |
| admin CSS files | 2 | 2 | ✓ |
| `encoding/csv` importers | 1 | 1 | ✓ |
| strings in source | 1158, `0 offen, 0 verwaist` | 1158, `0 offen, 0 verwaist` ×4 | ✓ |

Two numbers the plans use that were **not** in that table, measured here for the
first time — see blockers B4 and B5:

| What | Measured now |
|---|---|
| `grep -c 'badge--' cmd/holzcloud/assets/admin.css` | **6** |
| `grep -c '@layer components' cmd/holzcloud/assets/admin.css` | **15** |

---

## 2. Coverage

### IMP-01 … IMP-10 — all ten owned

Union of the six plans' `requirements:` frontmatter covers IMP-01 through IMP-10
with no gap. Each also has a task that actually delivers it, not merely a
mention:

| Req | Owning plan(s) / task |
|---|---|
| IMP-01 | 09-03 T1 (`Spalten`, index-addressed), 09-04 T1 (the mapping screen) |
| IMP-02 | 09-03 T3 (`SchreibeZeile` → `page.CreatePage`, no second path), 09-05 T2 |
| IMP-03 | 09-01 T1 (`Zeilennummer`), 09-05 T1 (`Fasse`), 09-05 T3 (the report) |
| IMP-04 | 09-04 T1 step 8 (the two radio choices), 09-03 T3 step 11, 09-05 T2 |
| IMP-05 | 09-05 T2 (`csvLauf(schreiben:false)`, `TestProbeSchreibtNichts`) |
| IMP-06 | 09-03 T1 (`FaltKopf`, `Automatisch`) |
| IMP-07 | 09-01 T2 (`Beispiel`), 09-04 T2 (`HandleCSVBeispiel`) |
| IMP-08 | 09-04 T1 step 5 (the clamped `?zeile=` stepper), 09-02 (staging makes it buildable) |
| IMP-09 | 09-01 T1 (rows/cells/BOM/NUL), 09-04 T1 step 3 (the 10 MB pair) |
| IMP-10 | 09-01/02/03/04/05/06 — the `BeginTx` gate, plus 09-03 T3's compensation |

**Nothing vanished.** No requirement is covered only by prose.

### The 27 `resolved`/`explicit` edge resolutions — all 27 become checkable assertions

Traced one by one. Each lands as a `<behavior>` bullet with a named test, or as a
grep gate, or both:

- IMP-01 adjacency/empty/ordering/concurrency → 09-03 T1, 09-04 T1, 09-02 T2
- IMP-02 idempotency/concurrency → 09-03 T3 `TestDateizweimalEingelesen`, 09-05 T2
- IMP-03 boundary/concurrency → 09-01 T1 `TestZeilennummerIstDieDerTabelle`, 09-05 T2
- IMP-04 adjacency/empty/ordering/concurrency → 09-03 T3, 09-04 T1, 09-05 T2
- IMP-05 concurrency → 09-02, 09-05 T2 (same `a.Daten` both passes)
- IMP-06 adjacency/empty/encoding/ordering → 09-03 T1, all four with named subtests
- IMP-07 concurrency → 09-04 T2 (`Cache-Control: no-store` + the snapshot sentence)
- IMP-08 adjacency/empty/ordering → 09-04 T1 `TestZeileBlaetternIstBeschnitten` et al.
- IMP-09 boundary/adjacency/empty/ordering/precision → 09-01 T1 (4 of the 6 boundary
  cases) + 09-04 T1 (the 10 MB pair) = **six cases, as the resolution demands**
- IMP-10 concurrency → the mechanical `BeginTx` gate in five of six plans

The 7 `dismissed` edges are correctly not built against.

### PROJECT/ROADMAP success criteria 1–6 — each has an owning, gated task

Criteria 1, 2, 3, 4, 6 all have owning tasks with verifiable gates. Criterion 5's
two halves are the strongest-gated in the phase: "no transaction spans more than
one row" is `grep -rln BeginTx` over three paths, and the compensation is
`grep -c 'PurgePage'` on comment-stripped source plus
`TestZeileWirdZurueckgenommen`. **No criterion is owned by prose alone.**

---

## 3. The specific checks requested

### `layoutPageNames` — the gate is correct, exactly four templates

Read out of the plans rather than out of the gate: `csv_mapping.html` and
`csv_abgelaufen.html` (09-04 T1 step 6), `csv_probe.html` and `csv_report.html`
(09-05 T3). **Exactly four, each named, each appended to the slice by the plan
that creates it.** 44 + 4 = 48, and the intermediate 46 in 09-04 is right rather
than a shortcut — 09-04's gate explicitly rejects 48 because a name whose template
does not exist yet is a `RenderAdmin` error at request time. The admin-template
count agrees independently: 61 + 4 = 65. **No defect here.**

### D-32, the translation trap — honoured, and gated

`09-03` T2 defines `Urteil{Zeile int; Ausgang; Grund; Args []string}` — a code plus
arguments — and `09-05` T3 renders it through one `{{define "csv-grund"}}` block of
`{{tf}}` literals. I confirmed against `tools/i18n/main.go:51-65` that the collector
reads exactly eight Go functions (`SetFlashError`, `SetFlashSuccess`,
`SetFlashWarning`, `Add`, `NewLayoutData`, `Titlef`, `T`, `N`). **No plan builds a
user-visible sentence in Go outside those eight.** `urteil.go` and `gruppen.go` are
both gated to not import `fmt` at all, and `09-06` T1 gates that the 1158 count
actually moved — which is the check that catches the failure D-32 describes.
**No blocker.** Two heuristic softnesses are recorded as W2 and W3.

### IMP-10 — both halves owned

The `BeginTx` gate is present in **five of six plans**, in both forms: the narrow
`grep -rln BeginTx internal/csv/ internal/csvimport/ internal/admin/csvimport.go`
= 0, and the global `internal/` = 14 (never 15). `09-06` re-measures both against
the finished tree. The compensation path (D-02) is owned by `09-03` T3 with a
`PurgePage` gate and a named test that forces `SetForPage` to fail. **No blocker.**

### Wave ordering — correct

Dependencies are a clean chain `01 ← 02 ← 03 ← 04 ← 05 ← 06`, waves 1–6, acyclic,
no forward references. Every plan is alone in its wave, so the file-overlap
question (`internal/admin/csvimport.go` in 04, 05 and 06's orbit) is satisfied
trivially — **no two plans in one wave write any file.** `09-06` carries the
review-fix precondition as a `<precondition>` element on Task 2, not as prose:
"Do not drive the browser before this: in Phase 7 and Phase 8 the verifier caught
user-visible changes landing after the pass was signed off." **The Phase 7/8
failure is closed.**

### Migration `00049` — correct

`09-02` is the only plan that touches `internal/db/migrations/`. It uses `00049`,
verified as the next free number (tree runs to `00048`), and gates
`git diff --name-only -- internal/db/migrations/ | grep -c -v '00049_csv_imports.sql'`
= 0 so no released migration can be edited. `09-04`'s table asserts the count stays
at 49. **No plan edits a released migration.**

---

## 4. BLOCKERS (must fix before execution)

### B1 — `09-04`, Task 1 step 9: the `adminOnly` row is a D-37 leftover, and the real new route gets no row

The plan tells the executor to add:

```
{"GET", "/admin/csv-import/abc/beispiel"},
```

That route no longer exists — D-37 moved the example to `GET /admin/csv-vorlage`,
and the same task's route block (three lines above) correctly registers
`csv-vorlage`. Two consequences, and the second is the dangerous one:

1. `TestRouteAuthorization` asserts `editor got 403` for every table row
   (`main_test.go:174-176`). An unregistered five-segment path answers 404, not
   403, so **the test fails deterministically.**
2. The path of least resistance for whoever meets that failure is to delete the
   row — which leaves `GET /admin/csv-vorlage` with **no row in the table at all**.
   That table's own comment says it exists because "website deletion shipped
   reachable by any editor". D-06 requires every new route in it.

**Fix:** replace the third row with `{"GET", "/admin/csv-vorlage"},` and say in the
action that the row must survive, not merely that the test must pass.

### B2 — `09-04`, Task 1 step 9: registers a route whose handler Task 2 writes

Step 9 registers all three routes, including
`GET /admin/csv-vorlage → HandleCSVBeispiel`. `HandleCSVBeispiel` is created in
**Task 2**. Task 1's own verify runs `go build ./...` and asserts
`adminProtectedMux.Handle` = 148 — so Task 1 cannot satisfy its own gates.

**Fix:** move the `csv-vorlage` registration (and its `adminOnly` row) into Task 2,
and set Task 1's gates to 147 `Handle` / 16 `adminOnly` rows; or state explicitly
that Task 1 also writes `HandleCSVBeispiel`'s signature. The arithmetic in Task 3's
table must follow whichever is chosen.

### B3 — `09-04`, Task 1 verify: `grep -c 'csv-import' main_test.go` ≥ 3 is unreachable

The three correct rows are `/admin/websites/import-csv`, `/admin/csv-import/abc`
and `/admin/csv-vorlage`. Only the middle one contains the literal `csv-import`
(the first contains `import-csv`, the third contains neither). The gate can
measure at most **1**. It was already wrong before the D-37 edit — the old third
row would have made 2 — so the edit did not cause it, but the edit did not fix it
either.

**Fix:** count the three routes explicitly, e.g.
`grep -Ec 'websites/import-csv|csv-import/|csv-vorlage' cmd/holzcloud/main_test.go`
expecting 3.

### B4 — `09-05`, Task 3 verify: the `badge--` gate ships a guessed number, and it is wrong

```
grep -c 'badge--' cmd/holzcloud/assets/admin.css
fails_when: the printed count is not 8
```

**Measured: 6.** The plan adds no badge modifier, so the count stays 6 and the gate
fails deterministically. The `fails_when` prose does say "Re-measure before trusting
this number" — but prose does not run; the assertion does, and it carries a guess.
This is the Phase 8 failure in the shape `09-CONTEXT.md` warns about, inside the
plan that quotes the warning.

**Fix:** change the expected value to **6** and record it in the "Baseline counts"
table so a later plan does not re-guess it.

### B5 — `09-05`, Task 3 verify: the `@layer components` gate cannot fail

```
<automated>grep -c '@layer components' cmd/holzcloud/assets/admin.css</automated>
<fails_when>the printed count did not increase by exactly 1 against the
pre-change tree — measure it before the edit and record both numbers</fails_when>
```

The command prints one number and the condition needs two. Nothing in the gate
knows the baseline, so no automated run can decide pass or fail — **the criterion
is owned by prose.** Measured baseline is **15**, so the post-change number is 16.

**Fix:** `fails_when: the printed count is not 16 — 15 at baseline plus this plan's
one appended block.`

### B6 — `09-04`, Task 2: token-scoped leftovers contradict the plan's own action

Three places in Task 2 still assume the pre-D-37 shape, and each contradicts the
action text or the acceptance criteria a few lines away:

- `<behavior>` bullet 1: "`GET /admin/csv-import/<token>/beispiel` answers 200 with
  …". The action says `GET /admin/csv-vorlage?website={id}` and the acceptance
  criterion says "It does **not** call `ablage`: it is not token-bearing."
- `<action>`: "For a target website that does not exist yet, use the pending name
  from the staging row through the same call." There is no staging row on this
  path — the handler never resolves a token. D-37 settles it the other way: a
  website that does not exist yet gets the four fixed columns and the panel says so.
- Test name `TestBeispielFremdeMarkeIstNichtGefunden` — "fremde Marke" is a foreign
  *token*. The behaviour it should assert is a website id this admin cannot reach.

**Fix:** rewrite the behaviour bullet to `GET /admin/csv-vorlage?website=<id>`;
delete the pending-name sentence and replace it with the four-fixed-columns rule;
rename the test to `TestBeispielFremdeWebsiteIstNichtGefunden`.

*(Checked and clean in the same edit: the gate `grep -c 'beispiel' cmd/holzcloud/main.go`
expecting 0 does pass, because `HandleCSVBeispiel` capitalises the B. It survives on
case alone — see W6.)*

---

## 5. WARNINGS (worth fixing)

**W1 — `09-02` is `autonomous: false` and opens with a `checkpoint:decision`.**
`09-CONTEXT.md` opens by recording that "the developer asked for the whole
milestone to be carried out autonomously", and every other plan is `autonomous:
true`. The checkpoint is defensible — a released migration is a one-way door — but
it will halt an unattended run at wave 2 of 6. Either pre-approve
`wie-vorgeschlagen` in the plan or accept that the phase pauses there.

**W2 — `09-05` T3's arm-count gate counts lines, not occurrences.**
`grep -c "eq .Grund"` against `grep -c 'Grund = "'` is exact only if each
`{{else if eq .Grund …}}` sits on its own line. A template that puts two arms on
one line silently under-counts and the gate passes while a reason renders the
`{{else}}` fallback. Use `grep -o … | wc -l` on both sides.

**W3 — the "no German `fmt.Sprintf`" gates are keyword heuristics.**
`grep -rn 'Sprintf' internal/csvimport/ | grep -c 'ä\|ö\|ü\|ß\|Zeile \|Spalte \|nicht \|konnte '`
(09-03 T2, and a variant in 09-05 T1) misses a German sentence containing none of
those tokens. The `no "fmt" import` gates on `urteil.go` and `gruppen.go` are the
real defence and they are exact; `zeile.go` has neither. Consider adding
`grep -c '"fmt"' internal/csvimport/zeile.go` = 0 if the row function genuinely
needs no formatting.

**W4 — `09-04` T2's `files_modified` omits `website_list.html`.**
Its acceptance criterion gates "The **panel** carries the download as a second GET
form … the panel is where IMP-07 is actually satisfied", but the panel edit is
described in Task 1 step 8 and Task 2 lists only `csvimport.go`, `csvimport_test.go`
and `csv_mapping.html`. Harmless in a sequential plan; confusing to the executor.

**W5 — `09-04` does not name `SetFlashError` for screen 1's refusals.**
The action says "three distinguishable flashes" without naming the function. Only
`SetFlashError`/`SetFlashSuccess`/`SetFlashWarning` are collected by `tools/i18n`;
a flash set any other way is invisible to QUAL-01, which is precisely D-32's hole
one layer down. Name the function in the action.

**W6 — `09-04` T2's `grep -c 'beispiel' cmd/holzcloud/main.go` = 0 passes on case
alone.** It is intended to prove no `/beispiel` route survived, and it works only
because `HandleCSVBeispiel` capitalises the B. Make it `grep -c '/beispiel'`.

---

## The single most dangerous thing found

**B1** — `09-04`'s `adminOnly` row still names `/admin/csv-import/abc/beispiel`, a
route D-37 deleted.

Not because it fails — everything here fails loudly — but because of *how* it will
be made to pass. `TestRouteAuthorization` will break on a row pointing at nothing,
and the cheapest repair is to delete the row. Do that, and `GET /admin/csv-vorlage`
— the one genuinely new, genuinely reachable route the D-37 edit created — ends the
phase with no entry in the table whose own comment records that website deletion
once shipped reachable by any editor. The failure is loud; the wrong fix is silent,
and it removes exactly the safety net D-06 exists to hold.

