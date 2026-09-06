---
phase: 08-snippets-carry-fields
verified: 2026-09-06
status: human_needed
score: 5/6 criteria verified
behavior_unverified: 1
requirements: [SNIP-01, SNIP-02, SNIP-03, SNIP-04, SNIP-05]
human_verification:
  - test: "On a snippet, define one field of each kind Phase 7 added — mehrfachauswahl, zeit, bereich, code, schlagwort — fill each on the snippet screen, save, reload."
    expected: "Every value comes back in its own control; the schlagwort picker lists this website's terms; the bereich shows both bounds."
    why_human: "Criterion 1 names these kinds explicitly. The palette is offered (field.Kinds in full) and the mechanism is shared with the page editor, but no test and no browser step exercises any of the five on a snippet through the form. Round trip is proven for text, langtext and gruppe only."
  - test: "Re-drive the four user-visible things the review-fix round changed AFTER the wave-5 browser pass."
    expected: "(a) field_list.html now prints {{index .Site.Bausteinfelder \"key\" \"kennung\"}} — screenshot 01b captured the old, broken dot-notation advice; (b) two groups on one snippet, both carrying a sub-field named the same, are both accepted (migration 00048); (c) a 404 page, the maintenance page and a share-error page carry .Site.Bausteinfelder; (d) a bundle import whose snippet holds a bild/verweis/schlagwort value shows the new warning line in the report."
    why_human: "Criterion 6 asks that everything a person can see has been driven once through the running application. 08-REVIEW-FIX.md contains no browser or Playwright section; each of these four is covered by a passing test only."
warnings:
  - "WINDOWS.md entry 5 is open and visible: cmd/holzcloud/templates/admin/field_list.html:16 — the snippet back link this phase added prints the literal text '&#8592; Alle Textbausteine'. Correctly recorded and correctly deferred (the string is the catalogue key), but it ships."
  - "WR-03 is recorded only in the phase's own deferred-items.md. Its own commit message (7cfbccd) says it 'gehört in die Roadmap, nicht in einen Kommentar'; ROADMAP.md and the planning root contain no entry for it."
  - "The wave-5 browser screenshots no longer exist — the scratchpad and the throwaway data directory were deleted. The pass is corroborated by two artifacts in the tree instead (see criterion 3), not by images."
---

# Phase 8: Snippets Carry Fields — Verification

**Goal:** a text snippet stops being one Markdown box and becomes a small content model of its own.
**Verified:** 2026-09-06 · goal-backward, from the six ROADMAP success criteria.

Tree state confirmed by command, not by SUMMARY: `go build ./...`, `go vet ./...`, `gofmt -l .` all silent; `go test -count=1 ./...` — 41 packages `ok`, zero failures.

---

## Criterion 1 — any field kind on a snippet, filled and surviving a reload

**Verdict: VERIFIED for the mechanism and for the kinds exercised; one clause routed to human.**

- The palette is offered in full. `internal/admin/field.go:322` — the snippet arm of `fieldListData` sets `kinds = field.Kinds`, not `field.BlockKinds()`. `BlockKinds()` (`internal/field/field.go:167–177`) drops `gruppe`, `abschnitt`, `verweis`, `schlagwort`; the snippet arm drops nothing. The comment at `:318–321` states why.
- The value path is the page editor's own, call for call. `internal/admin/snippet.go:91` `fieldsFromRequest(r)` → `:243` `groupAction` → `:274` `field.CheckAll` → `:292` `field.Encode(field.Clean(defs, values.Fields))` → `:333` `h.snippets.SetFields`, and back in through `:156` `field.Decode(sn.Fields)`. The form is drawn by `fieldViews(defs, values.Fields, data.pool(), …)` at `:229`/`:276` over the existing `field_top` block (`snippet_list.html:67`).
- The pickers are fed: `internal/admin/snippet.go:217–221` fills `Media`, `RefPages` (`h.refPages`) and `RefTerms` (`h.siteTerms`), which `pool()` at `:47–49` hands to `fieldViews`. So a `bild`, `verweis` or `schlagwort` field on a snippet gets the same choices it gets on a page.
- Round trip proven: `TestSnippetFeldRundlauf` (`internal/admin/snippet_fields_test.go:375`) — PASS. It saves `text` and `langtext` through the real handler, reads the stored column back, then re-opens the form and asserts both values and both `FieldName()`s are in the body. `TestGruppeAmTextbausteinTraegtIhreUnterfelder` — PASS — covers `gruppe`.

**What is not proven.** No test and no browser step fills a `mehrfachauswahl`, `zeit`, `bereich`, `code` or `schlagwort` field **on a snippet** and reloads it. `grep` over `snippet_fields_test.go`, `bausteinfelder_test.go`, `bundle_test.go` and `field/store_test.go` returns no such case; the wave-5 browser pass drove `text`, `langtext` and one `gruppe`. The criterion names these kinds by hand ("including every kind Phase 7 added"), so the shared-mechanism argument is strong but is an argument, not evidence. Routed to human verification rather than passed.

## Criterion 2 — one field table, `snippet_id` beside `website_id`, no key collision

**Verdict: VERIFIED.**

- One table, not a third. `internal/db/migrations/00047_snippet_fields.sql` contains no `CREATE TABLE`: `ALTER TABLE page_field_defs ADD COLUMN snippet_id INTEGER REFERENCES snippets(id) ON DELETE CASCADE`, an index swap, and `ALTER TABLE snippets ADD COLUMN fields TEXT NOT NULL DEFAULT ''`. `00048` adds no table either.
- Four namespaces, drawn as partial unique indexes: `idx_page_field_defs_kennung_oben` on `(website_id, kennung) WHERE parent_id IS NULL AND block_type_id IS NULL AND snippet_id IS NULL`; `idx_page_field_defs_kennung_textbaustein` on `(snippet_id, kennung) WHERE snippet_id IS NOT NULL AND parent_id IS NULL` after `00048`.
- **Both collision directions run and pass.**
  - `TestBausteinNamensraum` (`internal/field/store_test.go:539`) — PASS. Creates `telefon` on the page and `telefon` on a snippet of the same website; both succeed. Then asserts `List` returns exactly the page field, `OfSnippet` exactly the snippet field, `Sub`/`OfBlockType`/`OfBlockTypes` nothing, and — the part a count cannot fake — that `Get` writes `SnippetID` into the `Def`.
  - `TestBausteinGruppenNamensraum` (`:1187`) — PASS. This is CR-01's area. It walks the page's four steps (two groups, `tag` in each, `tag` at top level) and then the identical four on a snippet, then asserts the namespace is still *narrow*: a second top-level `tag` and a second `tag` in the same group each return `ErrDuplicateKey`. Both directions, in one test.
- `00048`'s Down deliberately restores `00047`'s wider form and says so in the file; `internal/db/migrations_down_test.go` gained coverage in the same round.

## Criterion 3 — a snippet's fields never leak into the other three namespaces

**Verdict: VERIFIED.**

Every reader in `internal/field/store.go` names the foreign namespaces:

| reader | line | WHERE |
|---|---|---|
| `List` (page fields) | 72 | `website_id = $1 AND block_type_id IS NULL AND snippet_id IS NULL` |
| `OfBlockType` | 144 | `… AND block_type_id = $2 AND snippet_id IS NULL` |
| `OfBlockTypes` | 174 | `… AND block_type_id IS NOT NULL AND snippet_id IS NULL` |
| `OfSnippet` | 211 | `… AND snippet_id = $2` |
| `OfSnippets` | 269 | `… AND snippet_id IS NOT NULL` |
| `MaxFields` count | 441/444/452/457 | one branch per carrier, no "whatever is left" arm |
| `Create` position subquery | 489–491 | `COALESCE(parent_id/block_type_id/snippet_id, 0) = COALESCE($…, 0)` |

`Sub` (`:112–118`) is the one deliberate exception and carries its reason at `:105–111`: a group's sub-fields inherit `snippet_id` from the group, so `AND snippet_id IS NULL` would return every snippet group empty; the namespace there is `parent_id`, which is unique within the website. Sound, and documented in the file rather than in a plan.

`.Page.Feldliste` and the page edit form both read `field.Store.List` (`internal/admin/field.go:328`, `internal/admin/page_fields.go:413`, `internal/public/pagedata.go:49`), so all three surfaces the criterion names are covered by the one clause.

**Confirmed in a browser, not only in a test.** The screenshots are gone with the throwaway directory, but two artifacts in the tree could only have come from a real run:
- commit `3ad28ba` — "Im Browserdurchgang gefunden, nicht im Test": the group screen carries `?gruppe=<id>` and knew nothing of a snippet one level down, so it wrote sub-fields with `snippet_id` NULL and `OfSnippet` returned "Gruppe (0)". A green suite cannot produce that finding. Fixed by inheriting the carrier from the stored parent (`internal/admin/field.go:200`).
- `WINDOWS.md` entry 5 — the literal `&#8592;` on the back link, visible only when rendered.

`TestTextbausteinfeldErscheintNichtAufDemSeitenbildschirm` and `TestSnippetFeldStehtNichtImSeitenformular` — both PASS — hold the drawn-template half.

## Criterion 4 — same pipeline, same sanitisation, one chain

**Verdict: VERIFIED — and the criterion's own wording is met by a different mechanism than it names. Stated plainly:**

`internal/field/render.go` has arms for `KindBool`, `KindNumber`/`KindRange`, `KindTime`, `KindDate`, `KindImage`, `KindRef`, `KindTerm`, `KindMulti` and then `default:` at `:227`. **There is no `KindLong` arm.** A `langtext` value on either carrier falls to `default:` and reaches the theme as a plain Go string; what protects it is `html/template`'s contextual escaping — the identical mechanism protecting the identical value on a page. So the promise ("sanitised away exactly as it is on a page") holds; the words ("one goldmark → bluemonday chain") describe the snippet **body**, not the field value.

- Counted from git, not claimed: `git diff <phase-base>^ HEAD -- internal/ cmd/` adds **6** `RenderMarkdown` lines and removes 0 — all six are in `_test.go` files. Added lines matching `goldmark|bluemonday` are **comments only**. The phase adds no production chain. `page.RenderMarkdown` on the snippet body sits at `internal/admin/snippet.go:299`.
- `TestSnippetFeldSanierung` (`internal/admin/snippet_fields_test.go:497`) — PASS — is the right shape: it defines `hinweis`/`langtext` on *both* a snippet and a page, posts `<script>alert(1)</script>` through both real handlers, resolves both through `field.Resolve`, prints both through the *same* theme template, and asserts (a) neither output contains `<script` and (b) the two outputs are byte-identical. It then re-reads the snippet and asserts the body still carries `<strong>da</strong>` — the body's own chain untouched.

## Criterion 5 — existing snippets keep working untouched

**Verdict: VERIFIED.**

- `internal/template/loader.go:377` — `Snippets map[string]template.HTML`, type unchanged. `Bausteinfelder map[string]map[string]any` (`:390`) and `Bausteinliste map[string][]field.Entry` (`:400`) are appended **after** it. `TEMPLATE-SPEC.md:212–214` documents all three as separate rows; §7 (`:725–763`) spells out the two-level indexing and the deliberate asymmetry between the two new maps.
- `fillSnippets` (`internal/public/pagedata.go:102`) assigns `site.Snippets = rendered.HTML` first and unchanged, then the two new maps. It is the only assignment left in the tree: `grep -rn "Snippets = |Snippets:" internal/public/*.go` returns exactly that one line, against 18 call sites across `handler.go`, `access.go`, `archive.go`, `cart.go`, `checkout.go`, `pluginhost.go`, `search.go`, `shop.go`, `tag.go`, `typearchive.go`. `TestBausteinfelderAufDenRoutenOhneSeite` — PASS — covers the three page-less routes WR-04 added.
- A website with no snippet pays no query (`:112–114`) and gets empty maps rather than nil ones.
- No migration step for the admin: `internal/db/db.go:88` runs `goose up` at startup, as it always has. `snippets.fields` is `TEXT NOT NULL DEFAULT ''`; `field.Decode("")` yields an empty `Data`.
- Old bundles import unchanged: `Snippet.Fields`, `.Values`, `.ValueGroups` are all `omitempty` (`internal/bundle/format.go`), and `TestAeltererTextbausteinImportiertUnveraendert` — PASS — proves it.
- Key and body survive a save: asserted at the end of `TestSnippetFeldRundlauf`.

**Scope call on WR-03.** A snippet's `bild`/`verweis`/`schlagwort` value is a local id that `translateOut`/`translateIn` do not reach, so it points at a foreign row after a bundle trip. Against these six criteria this is a **correctly-scoped deferral, not a gap**: criterion 1 says "after reload" (the admin screen, which works), criterion 5 says existing snippets keep working (they carry no fields). The value still travels — nothing is deleted — and `ortsgebundeneWerte` (`internal/bundle/import.go`) now names it in the import report; `TestBildwertEinesTextbausteinsWirdBeimImportGemeldet` — PASS. Flagged as a warning only because the deferral was never promoted out of the phase directory.

## Criterion 6 — the standing gate

**Verdict: PARTIAL → human_needed. First half verified by command; second half short by four items.**

**i18n — VERIFIED.** Ran it:

```
1158 Zeichenketten im Quelltext
en.json      1158 übersetzt, 0 offen, 0 verwaist
es.json      1158 übersetzt, 0 offen, 0 verwaist
fr.json      1158 übersetzt, 0 offen, 0 verwaist
it.json      1158 übersetzt, 0 offen, 0 verwaist
```

de-CH / fr-CH / it-CH report `0 ohne Gegenstück`, as designed.

**Browser — incomplete.** The wave-5 pass is real and thorough (seven steps, JavaScript off for three of them, and it caught the `snippet_id` NULL bug four green suites missed). But the review-fix round landed **after** it and changed four things a person can see, and `08-REVIEW-FIX.md` contains no browser or Playwright section at all:

| after the pass | commit | seen in a browser? |
|---|---|---|
| `field_list.html:19` — the theme advice corrected from dot notation to `index` | `306bd12` | **No.** Screenshot `01b` captured the *old*, broken string. |
| Migration `00048` — two groups on one snippet may share a sub-key | `4367eb3` | **No.** The pass drove one group. |
| `renderNotFound` / `serveShareError` / `HandleMaintenance` now carry the snippet surface | `cbedf98` | **No.** |
| The new import-report warning line | `7cfbccd` | **No.** |

Each is covered by a passing test. None has been through the running application. That is exactly the distinction criterion 6 draws, so it is not passed here.

---

## Requirements

| Req | Status | Evidence |
|---|---|---|
| SNIP-01 | Satisfied, one clause unproven | `field.Kinds` in full at `admin/field.go:322`; round trip PASS for `text`/`langtext`/`gruppe`; the five Phase 7 kinds untested on a snippet |
| SNIP-02 | Satisfied | `00047` + `00048`; `TestBausteinNamensraum`, `TestBausteinGruppenNamensraum` both PASS |
| SNIP-03 | Satisfied | `TestSnippetFeldSanierung` PASS; zero production goldmark/bluemonday added (git-counted) |
| SNIP-04 | Satisfied | seven explicit discriminators; two page-form negatives PASS; browser step 4 |
| SNIP-05 | Satisfied | `.Site.Snippets` type unchanged; `omitempty`; `TestAeltererTextbausteinImportiertUnveraendert` PASS |

## Anti-patterns

`git diff` over the phase's production files: no `TODO`, `FIXME`, `XXX` or `HACK` introduced. No `.js` file, no inline `<script>`, no `on*` attribute in `field_list.html` or `snippet_list.html` — the two JavaScript prohibitions in plans 03 and 04 hold, and browser step 7 drove the group row buttons with scripting off.

One open, recorded, visible defect: `cmd/holzcloud/templates/admin/field_list.html:16` prints `&#8592;` literally on the snippet back link this phase added (two of the three occurrences pre-date the phase). `WINDOWS.md` entry 5, status `open`, with the reason — the string is the catalogue key, and changing it orphans three keys in four catalogues. Correctly deferred.

## Summary

Five of six criteria hold against the tree, on evidence I ran rather than read. The schema is one table with four namespaces and the collision question settled in both directions; the page form is clean at the query, at the template and in the browser; the sanitisation criterion holds by a mechanism its own wording misnames, and the code says so out loud; nothing that already worked changed type or needed a migration step.

What stops a `passed`: criterion 1 names five field kinds that nothing exercises on a snippet, and criterion 6's browser half was not re-run over the four visible things the review fixes changed. Both are short, concrete browser tasks — neither suggests the phase goal was missed.
