# Phase 9 — deferred items

Things the browser pass of plan 09-06 found and deliberately did **not** fix,
each with the reason. Nothing here blocks the phase; all of it is written down
so the next person does not have to rediscover it.

---

## 1. `Transition was skipped` in the browser console — pre-existing, admin-wide

**What.** Chrome raises an unhandled promise rejection, `Transition was
skipped`, when a navigation is started while a cross-document view transition is
still running.

**Where it comes from.** `cmd/holzcloud/assets/admin.css:1807` —
`@view-transition { navigation: auto; }`. The rejection is the browser's own;
this project ships no JavaScript that could catch it (htmx is the only script,
and it does not drive these transitions).

**Why it is not phase 9's.** Reproduced on screens this phase never touched:
navigating `/admin/templates` → `/admin/users` by clicking the sidebar raises
it identically. It also does not appear at all when the transition is allowed
to finish before the next navigation, which is why the final pass reports zero.

**Out of scope** by the executor's scope boundary: not caused by this phase's
changes, and the file it lives in is not this phase's.

---

## 2. "1 Zeilen" survives in the three file-header sentences

**What.** A file of exactly one data row reads

- mapping screen — `Datei „x.csv“, 5 Spalten, 1 Zeilen.`
- dry run — `Datei „x.csv“, 1 Zeilen geprüft.`
- report — `Datei „x.csv“, 1 Zeilen gelesen, Website „y“.`

**What was fixed instead.** The *group* cell, which reads `Eine Zeile: 4` now —
see commit `66c27dc`. That one is a single count standing beside its noun, which
is the shape all six existing sites in this administration have
(`blocktype_list.html:41` and `:44`, `dashboard.html:31`, `media_list.html:43`,
`snippet_list.html:94`, `website_list.html:152`).

**Why the other three are not fixed.** They carry **two counts in one sentence**
(`%d Spalten, %d Zeilen`). A singular for those needs four spellings of one
sentence, per language — which is a plural mechanism, and this project has
deliberately not built one: every existing site handles exactly one count with
an `{{if eq .X 1}}`. Adding the mechanism is a decision about the whole
administration, not a repair to this phase.

**If it is ever taken up**, take it up for the whole admin at once, not for the
CSV screens alone.

---

## 3. `ReasonRenamed` (D-23) could not be driven in a browser

**What.** The report's "the address was already taken, the page got `x-2`"
group. Driven in the suite (`internal/csvimport/row_test.go`,
`groups.go:114`), **not** driven in the browser pass, and the reason is
structural rather than a gap in the driving.

**Three separate things close every route to it:**

1. The create-vs-update pre-check is **live and per row**
   (`internal/admin/csvimport.go:849-855`), so a duplicate address inside one
   file becomes an update or a skip long before `CreatePage` sees it. Measured:
   a file with two rows at `/doppelt` reported `5 anlegen / 2 aktualisieren` in
   the dry run and `4 angelegt / 3 aktualisiert` in the report — the second row
   became an update at write time.
2. The trash **frees the address**: a deleted page's slug becomes
   `trash-<id>-<slug>` (verified in the database), so a trashed page cannot hold
   a slug against a new one. `GetPageBySlug`'s `LivePredicate` gap is therefore
   not reachable either.
3. Two concurrent commits **serialise**: the write pool admits one connection,
   so the loser's pre-check sees the winner's page. Driven — two browser
   contexts committing the same address with `Promise.all` produced
   `1 angelegt` and `1 übergangen`, not a rename.

`ReasonRenamed` is the safety net for a read-to-INSERT race that this
architecture makes vanishingly rare. That is the right design; it just means the
group is not something a person can produce on purpose.

---

## 4. The dry run's counters and the report's counters can legitimately differ

**What.** For a file with two rows at the same new address, the dry run says
`5 anlegen / 2 aktualisieren / 4 übergehen` and the report says
`4 angelegt / 3 aktualisiert / 4 übergangen`.

**Why it is correct.** The dry run cannot know about pages the same run is about
to create. The second `/doppelt` row is a create at check time and an update at
write time, because by then the first row has written it.

**Why it is written down.** Nothing on either screen says the two may differ, and
an operator who compares them has no way to tell this apart from a bug. A
sentence on the report — "a row whose address a row above it has just taken is
counted here as it actually went" — would close it. Not added, because it is a
new sentence in five languages on a screen whose wording was settled in 09-05.
