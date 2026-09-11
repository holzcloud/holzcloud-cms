---
phase: 11-galerie
verified: 2026-09-08T07:40:00Z
status: gaps_found
score: 5/6 must-haves verified
score_at_verification: 4/6 must-haves verified
amended: "2026-09-10 — beim Meilenstein-Audit: GAL-03 durch c113c22/f95e864 geschlossen; QUAL-01 bleibt offen"
behavior_unverified: 0
overrides_applied: 0
tree_verified: 062c002 (source), re-confirmed at 2061e7e — the only commit between the two
  touches .planning/audits/v1.6-I18N-828.md and no Go source, so every gate run below stands
gaps:

  - truth: "SC2 / GAL-03 — changing an album changes every page that carries it, without touching those pages"
    status: partial
    reason: >-
      A fifth unexpanded path exists. The four the phase found and fixed (unconditional GET,
      conditional request CR-01, Atom feed CR-03, admin preview WR-03) are genuinely closed and
      each is held by a named behavioural test. But internal/public/pluginhost.go serves
      p.ContentHTML raw on the plugin pages API — no album.Expand anywhere in the file. Proved by
      running a probe, not by reading: a page whose content_html carries [[album:sommer:0]] comes
      back from PagesForPlugin/OpPagesGet as "<p>[[album:sommer:0]]</p>". Neither fixed nor
      recorded in WINDOWS.md. Same defect class as CR-03, which the review round rated Critical.
    artifacts:
      - path: "internal/public/pluginhost.go"
        issue: "line 83 `info.HTML = p.ContentHTML` — unexpanded; grep -c album on the file is 0"
    missing:
      - "Expand album markers on the OpPagesGet HTML, or decide explicitly that the plugin pages API serves stored HTML and record that decision where the next reader will find it"
      - "A behavioural test on the plugin pages API of the shape internal/public/feed_album_test.go already has"
  - truth: "SC6 / QUAL-01 — every new string in all five languages"
    status: partial
    reason: >-
      The numeric half reproduces exactly (re-run in this session: 1319 strings, 0 offen /
      0 verwaist on en, es, fr, it). The first clause does not hold for Phase 11's own new strings.
      Plan 11-06 (5f7709e), extended by the CR-02 fix (ca85c63), minted five operator-facing German
      sentences in internal/bundle/import.go built with fmt.Sprintf. All five return 0 hits in
      en.json, and import_report.html:28 renders them straight onto the operator's screen. The
      collector cannot see a sentence in that shape, so the gate reads green over them by
      construction — the B-proxy-gate family. Recorded open as WINDOWS.md entry 6; recorded is not
      closed, and the criterion says "every new string".
    artifacts:
      - path: "internal/bundle/import.go"
        issue: "importAlbums: five fmt.Sprintf German warnings, none in any catalogue"
      - path: "internal/block/render.go"
        issue: "line 212 — textGallery is in all four catalogues and always renders German (WINDOWS entry 8)"
    missing:
      - "The five import warnings moved into the code-plus-arguments shape .planning/GLOSSARY.md already prescribes for csvimport (D-32), then translated"
      - "A decision on WINDOWS entry 8 — the string is marked, collected, translated four times and printed in German"

deferred:

  - truth: "828 operator-facing strings are in no catalogue at all"
    addressed_in: "Phase 12"
    evidence: >-
      Phase 12 success criterion 9 names the audit by file and owns the work verbatim. The audit
      itself (.planning/audits/v1.6-I18N-828.md) concludes "QUAL-01 bleibt erfüllbar und bleibt
      sinnvoll — es prüft die Fläche, auf der es steht, und auf der ist es scharf." Only Phase 11's
      own five additions are charged to this phase; the pre-existing 823 are not.
  - truth: "GAL-07's wording still names FIELD-07's exported pair, which the album does not call"
    addressed_in: "Phase 12"
    evidence: "Phase 12 SC 2 rewrites identifiers and the glossary; the wording correction belongs with it"
coincidental_reliance_items:

  - truth: "SC5 — the gallery block and the album read their image list through one mechanism"
    reason: undeclared-precondition
    harden: >-
      SC5 holds through block.GalleryItems being the single renderer, not through
      SplitValues/JoinValues, which the ROADMAP's Depends-on paragraph and REQUIREMENTS.md GAL-07
      both name and which no album code calls. The reasoning is recorded in migration 00050's
      comment; it is not recorded in either document a reader checks first.
audit_acknowledged:
  milestone: v1.6
  at: 2026-09-10
  status: gaps_found
---

# Phase 11: Galerie — Verification Report

**Phase Goal:** A picture stops being a dead end. A visitor can enlarge one and page through the rest; an editor assembles an album once and uses it on several pages; and a gallery can be paged through instead of scrolled past — with no JavaScript anywhere, on a public page the template rules already forbid it on.

**Verified:** 2026-09-08
**Status:** gaps_found
**Re-verification:** No — initial verification

## Which tree these numbers came from

The suite and the gates were run at `062c002` with `git status --short` empty. Mid-session HEAD moved to `2061e7e`; the single intervening commit adds `.planning/audits/v1.6-I18N-828.md` and touches no Go source (`git diff --stat 062c002..HEAD` → 1 file, `.planning/`). Every run quoted below therefore describes the source tree as it stands now. The Phase 10 agent's red-tree problem that plan 11-07 had to work around did not recur: `go test ./...` exited 0 across all 47 packages in one run.

```
go build ./...   clean
go vet ./...     silent
gofmt -l .       silent
go test ./...    exit 0, 47 packages, no FAIL line
go run ./tools/i18n
                 1319 Zeichenketten im Quelltext
                 en.json  1319 übersetzt, 0 offen, 0 verwaist
                 es.json  1319 übersetzt, 0 offen, 0 verwaist
                 fr.json  1319 übersetzt, 0 offen, 0 verwaist
                 it.json  1319 übersetzt, 0 offen, 0 verwaist
```

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | Enlarge a picture with its caption, back button gets you back, next/previous from the large view, no JavaScript | ✓ VERIFIED | `render.go:612-657` — tile is `<a href="#hc-bN-pM">`, large view is `<figure id="hc-bN-pM" tabindex="-1">`, prev/next are `<a>` to sibling ids, close is `#hc-zu`; caption emitted on both tile and large view. `bausteine.css:173-187` carries `:target`. Driven with scripting disabled and real input events (11-07 step 9): tile → `#hc-b3-p1`, next → `#hc-b3-p2`, close → `#hc-zu`. `<script>` count on a public page is 1, the `ld+json` data block. `tmplspec/spec_test.go:284` fails any shipped theme that does not link `/assets/bausteine.css`. |
| 2 | A named album, placed on several pages; changing it changes them all without touching them; scoped to one website | ✗ FAILED (partial) | Late binding is real and I confirmed the mechanism in code: `render.go:184` writes `AlbumMarker(slug, at)` into stored block HTML, `pagedata.go:63-65` expands per request. Website scoping is solid — 33 `website_id` occurrences in `album/store.go`, 10 tests in `admin/album_scope_test.go`. The four broken paths are closed and held by named tests. **A fifth is open:** `pluginhost.go:83` serves the raw marker (probe below). |
| 3 | An album survives the bundle round trip, including after a rename | ✓ VERIFIED | `bundle_test.go:1032 TestAlbumRoundTripAfterRename` — read in full, not taken on trust. It renames to a name that slugifies *differently* and asserts that guard itself (`if newSlug == oldSlug { t.Fatalf("the test would prove nothing") }`), asserts the slug did not move, asserts the manifest carries the new name **and** that the old slug appears nowhere, then imports into a fresh website and asserts the block re-derives to `newSlug`, the album exists under the new name, and its picture belongs to the *new* website. Property, not proxy. Run in this session: PASS. |
| 4 | Slideshow instead of grid: side by side, snapping, keyboard and touch | ✓ VERIFIED | `bausteine.css:417-467` — `overflow-x: auto`, `scroll-snap-type: inline mandatory` on the container, `scroll-snap-align: start` on every direct child; `render.go:211` adds `tabindex="0" role="region"`. Same markup, same `GalleryItems`, two stylesheets. Browser-measured (11-07 step 7 and step 9): two `ArrowRight` presses and one sideways push with no script engine moved the track to `scrollLeft = 1846` of a maximum of 1846. |
| 5 | One mechanism for the image list; existing `hc-galerie` markup, `srcset`/`sizes`, focus cropping, own aspect ratio unchanged; album is a new source, not a new renderer | ✓ VERIFIED (coincidental-reliance) | Both sources feed `[]block.Item` into the one `block.GalleryItems`: `render.go:188` for the inline gallery, `album/expand.go:133` for the album. `GalleryItems` is the only caller-visible renderer (grep confirms exactly two non-test call sites). The grid arm is untouched. **But** it holds through a different mechanism than the one the documents name — see the note below. |
| 6 | Standing gate: `0 offen, 0 verwaist`, and everything a person can see driven once in a browser | ✗ FAILED (partial) | QUAL-02 verified and unusually strong. QUAL-01's numeric half reproduces exactly. QUAL-01's first clause — *every new string in all five languages* — is false for five sentences this phase itself added. |

**Score:** 4/6 truths verified (0 present, behavior-unverified)

---

## The fifth GAL-03 path

The phase's own history is four paths on which GAL-03 was silently false, each found later than the last. I looked for a fifth rather than assuming there was none. I enumerated every reader of `content_html` in the tree (`grep -rn "ContentHTML\|content_html" --include="*.go"`), then eliminated them one at a time:

| Reader | Verdict |
|---|---|
| `public/pagedata.go:41-73` — the page | expands (`album.Expand` at :65) |
| `public/feed.go:117` — the Atom feed | expands (`expandForFeed`, CR-03) |
| `admin/preview.go:270` — the preview | expands (WR-03) |
| `public/pagedata.go:301-304` — the **plugin content hook** | safe: `album.Expand` runs at :65, *before* `filterByPlugins` at :69, and the comment says why |
| `public/search.go`, `sitemap.go`, `structured.go`, `archive.go` | carry no `content_html` |
| `page.Excerpt` | derives from **markdown**, not HTML — no marker can reach it |
| `admin/page_bulk.go:138`, `page_locale.go:201`, `csvimport/row.go:710` | copy the marker forward, which is correct: the copy stays late-bound |
| `ai/tools.go:554` | a write path that carries the value through unchanged |
| **`public/pluginhost.go:83` — the plugin pages API** | **does not expand** |

`grep -c album internal/public/pluginhost.go` is 0. `info.HTML = p.ContentHTML` is unconditional for `OpPagesGet`, and `plugin/abi.go:185` documents that field as "the full body". I did not stop at reading. In a throwaway copy of the tree (`git archive HEAD | tar -x`, no file in this repository modified):

```
zz_verify_plugin_album_test.go:26: HTML handed to the plugin: "<p>[[album:sommer:0]]</p>\n"
zz_verify_plugin_album_test.go:28: the plugin pages API served the RAW album marker
--- FAIL: TestVerifierPluginPagesGetAlbumMarker
```

Two consequences, and they are the same two the review wrote against CR-03: the album's pictures are absent, and the album's internal address leaks to third-party code.

**Weighing it honestly.** No shipped plugin calls the pages API — `grep` across `plugins/` finds `in.HTML` (the content hook, which is safe) and nothing else. So this is narrower in reach than the feed. It is *not* narrower than the admin preview, which the phase rated WR-03 and fixed. What makes it a gap rather than an observation is that it was neither fixed nor recorded: WINDOWS.md carries entries 6 and 8 for two other Phase 11 findings, and this one is in neither the review, the fix report, nor either evidence file.

---

## Where the two evidence files stand up, and where they stop

Both were treated as claims. Both survive.

**`11-LAUFENDE-ANWENDUNG.md`** — its central GAL-03 measurement (same page row byte for byte, different page on the net) is consistent with everything I found in code, and its own "Was hier NICHT geprüft ist" section correctly deferred the unstyled case, the slideshow and the admin screens to 11-07, all three of which 11-07 then drove. Its incident note — an apparent cache finding that turned out to be a malformed form post, caught by reading the database before interpreting — is the reason I trust the rest of it.

**`11-KRITISCH-NACHPRUEFUNG.md`** — the four mutation probes are real and each names a distinct promise. Its self-correction is the most useful sentence in the phase's paperwork: *"Eine Mutationsprobe misst, ob ein Wächter trägt. Sie kann nicht messen, ob er an der richtigen Stelle steht, und sie stellt die Frage gar nicht, ob es eine zweite Tür gibt."* I applied exactly that question to GAL-03 and found the second door above. I applied it to CR-02 as well: `TestCreateRefusesANameARenamedAlbumAlreadyHas` exists, is at `album_collision_test.go:100`, and passes — the door that file describes is now shut.

## Data-Flow Trace (Level 4)

| Value | Source | Flows | Status |
|---|---|---|---|
| Album pictures on a public page | `album.Store.LoadFor` → `album.Expand` → `block.GalleryItems` | yes, per request | ✓ FLOWING |
| Album pictures in the Atom feed | `h.albumsFor` → `expandForFeed` | yes | ✓ FLOWING |
| Album pictures in the admin preview | `h.previewAlbums` → `album.Expand` | yes | ✓ FLOWING |
| Page body on the plugin content hook | expanded before `filterByPlugins` | yes | ✓ FLOWING |
| `PageInfo.HTML` on the plugin pages API | `p.ContentHTML`, stored | no | ✗ DISCONNECTED |
| `Last-Modified` / `ETag` for an album change | `album.Set.Latest` → `contentModTime`; `web.MatchesETag` | yes | ✓ FLOWING |
| Slideshow region name (`aria-label`) | `s.text` = save-time translator, never the request one | no | ⚠️ STATIC (WINDOWS 8) |

## Behavioural Spot-Checks

| Behaviour | Command | Result | Status |
|---|---|---|---|
| The catalogue gate | `go run ./tools/i18n` | 1319 strings, `0 offen, 0 verwaist` × 4 | ✓ PASS |
| Whole suite | `go test ./...` | exit 0, no FAIL | ✓ PASS |
| Round trip after rename | `go test ./internal/bundle/ -run TestAlbumRoundTripAfterRename` | PASS | ✓ PASS |
| Manifest carries no slug | `-run TestManifestCarriesNoAlbumSlug` | PASS | ✓ PASS |
| CR-02's second door | `-run TestCreateRefusesANameARenamedAlbumAlreadyHas` | PASS | ✓ PASS |
| Plugin pages API vs. the marker | probe in a throwaway tree | serves `[[album:sommer:0]]` | ✗ FAIL |

## Requirements Coverage

| Requirement | Status | Evidence |
|---|---|---|
| GAL-01 large view, caption, back button, no JS | ✓ SATISFIED | `render.go:612-657`, `bausteine.css:173-187`, browser pass step 9 |
| GAL-02 next/previous as sibling links | ✓ SATISFIED | `render.go:645-653`, absent at each end rather than wrapping |
| GAL-03 album changes every page untouched | ✗ BLOCKED | true on four paths, false on `pluginhost.go:83` |
| GAL-04 round trip after rename | ✓ SATISFIED | `TestAlbumRoundTripAfterRename`, run and read |
| GAL-05 one website, invisible from every other | ✓ SATISFIED **but mis-ledgered** | 33 `website_id` in `album/store.go`, 10 tests in `admin/album_scope_test.go`, browser step 10. REQUIREMENTS.md:180 still reads `- [ ]` and :304 still reads `Pending` |
| GAL-06 slideshow, scroll-snap, keyboard and touch | ✓ SATISFIED | `bausteine.css:417-467`, measured `scrollLeft 1846/1846` |
| GAL-07 one mechanism inherited from FIELD-07's exported pair | ⚠️ SATISFIED IN SPIRIT, NOT IN WORDING | see below |
| QUAL-01 | ✗ BLOCKED | five new uncollectable strings |
| QUAL-02 | ✓ SATISFIED | twelve-step browser pass, every outcome a number or a string |

**No orphaned requirements** — REQUIREMENTS.md maps GAL-01…07 to Phase 11 and all seven are claimed by plans.

### GAL-07: the criterion holds, the sentence describing it does not

The ROADMAP's Depends-on paragraph says the album "must read and write that list through Phase 7's `SplitValues`/`JoinValues` (FIELD-07)", and REQUIREMENTS.md GAL-07 says "inherited from FIELD-07's **exported pair**". I checked. `grep -rn "SplitValues\|JoinValues" --include="*.go" internal/` returns eleven non-test call sites and **not one of them is in `internal/album/`**. The album stores its items as rows in `album_items`.

This is not a miss, and it is not a defect. `SplitValues` encodes one string per line; an album item carries `media_id`, `alt` and `caption`, which that pair cannot express. The original sentence rested on a wrong premise about the shape of the data. The phase noticed and reinterpreted the requirement — `00050_albums.sql` says so in the comment over `album_items`: *"GAL-07 asks for one mechanism, and one mechanism means both lists feed the SAME []block.Item into the SAME gallery renderer."* That reinterpretation is right, and it is what ROADMAP SC 5 (the text I verify against) actually asks for, which is why SC 5 is VERIFIED.

What is left undone is bookkeeping with teeth: REQUIREMENTS.md still carries the superseded wording and marks it Complete, so the next reader who greps for `SplitValues` in `internal/album/` finds nothing and has to re-derive this whole argument. Flagged as `coincidental-reliance` (advisory — it changes no score and no status).

## Anti-Patterns Found

| File | Pattern | Severity | Impact |
|---|---|---|---|
| — | `TBD` / `FIXME` / `XXX` / `HACK` / `PLACEHOLDER` across all 92 files changed in `b5ce7bd..HEAD` | — | none found; debt-marker gate clean |

## The standing gate, examined rather than counted

**QUAL-01, the number: true.** I re-ran it. 1319 strings, four catalogues at `0 offen, 0 verwaist`.

**QUAL-01, the sentence: not true for this phase's own strings.** `internal/bundle/import.go`'s `importAlbums` builds five operator-facing German sentences with `fmt.Sprintf`:

```
"%d Alben konnten nicht angelegt werden."
"Album %d %q konnte nicht angelegt werden: %v"
"Album %q: das Bild %q ist nicht im Archiv."
"Album %q: das Bild %q kam nicht an: %v"
"Das Archiv nennt zwei Alben %q. …"
```

All five return **0 hits** in `en.json`. `import_report.html:28` renders `Report.Warnings` straight onto the operator's screen. So an operator running the admin in English who imports an archive with an album problem is shown German — while the gate reports `0 offen, 0 verwaist`, because the collector cannot see a sentence in that shape. `git log -S` puts the first four in `5f7709e` (plan 11-06) and the fifth in `ca85c63` (the CR-02 fix). These are Phase 11's own new strings, not inherited ones.

WINDOWS.md entry 6 records exactly this, `status: open`, with the correct remedy named (`.planning/GLOSSARY.md`'s D-32 shape: code plus arguments, not a finished sentence). Recording is the right call for a Rule 4 decision; it is not the same as the criterion being met.

**On the 828.** `.planning/audits/v1.6-I18N-828.md` — which landed in this tree while I was verifying — measures 828 operator-readable strings in no catalogue at all. I am **not** charging those to Phase 11. The audit's own conclusion is that QUAL-01 "bleibt erfüllbar und bleibt sinnvoll — es prüft die Fläche, auf der es steht, und auf der ist es scharf", and ROADMAP Phase 12 SC 9 owns the work by name. What is charged to Phase 11 is the five sentences it added to that pile after the shape was already documented as a known trap in CLAUDE.md.

**WINDOWS entry 8, confirmed in code.** `render.go:212` writes the slideshow's `aria-label` with `s.text` — `block.Set.T`, wired only at save — while the controls inside reach `GalleryItems` through `expand.go:133` with the request-time translator. One gallery, two languages, in one render pass. The blast radius is one screen-reader-only attribute, which is narrow and real. The file now documents the class in its own words at `render.go:398`: *"A string that is marked and never injected is collected, translated into four catalogues and printed in German anyway."* Correctly deferred as a Rule 4 architectural decision.

**QUAL-02, the browser half: met, and the two "not driven" items do not defeat it.** They are recorded as *not driven* and not as passed, which is what you asked me to check — `11-07-SUMMARY.md:779-790`. Judging them:

- *No Spanish/French/Italian website through the settings form.* `internal/template/dates.go:63` offers `de` and `en` only, so the UI cannot express it. The gate says "a website whose language is not German"; English was driven through the form and Spanish was driven out of band with the deviation labelled. **Criterion survives** — the gap is in `SupportedLocales`, not in Phase 11.
- *No drag with a real finger.* `Input.synthesizeScrollGesture` with `gestureSourceType: "touch"` is Chrome's synthesis. GAL-06 asks that the slideshow be "reachable by keyboard and by touch"; the keyboard half is native input, the touch half moved the track and it snapped. **Criterion survives**, with the honest caveat standing.
- The deleted stale screenshot and the missing `11-UI-SPEC.md` are the other two. The UI-SPEC was a ROADMAP *Research flag* recommendation ("Worth a UI-SPEC"), not a success criterion; the plan recorded the substitution deliberately. Noted, not charged.

The rest of the pass is the strongest browser evidence in this project's recent phases: the stylesheet blocked at the network layer and the consequence *measured* rather than the API trusted (`display: grid` → `block`, anchors still working, `scrollWidth == clientWidth`); the whole thing re-driven with the script engine disabled and real input events; a foreign website's albums attacked with a valid CSRF token from the attacker's own page. And it found a Critical — the `Create`-after-rename door — two hours after CR-02 was signed off.

## Human Verification Required

None blocking. Two decisions for the developer, both already written down:

1. **WINDOWS entry 6** — the five uncollectable import warnings. Fixing them is the mechanical D-32 move and is small; leaving them means QUAL-01 stays formally unmet for this phase.
2. **WINDOWS entry 8** — the mixed-language slideshow region name. Genuinely architectural; deferring was right.

## Gaps Summary

Phase 11 delivered its goal on every surface a visitor touches. The lightbox is `:target` and nothing else, and it survives both a blocked stylesheet and a disabled script engine — which is more than the criterion asked for. The album is a properly scoped, properly migrated, properly bundled resource, and `TestAlbumRoundTripAfterRename` is the rare case where the proving test proves the thing rather than a nearby thing: it renames to a differently-slugifying name and then asserts that its own guard is meaningful. The slideshow is one block of CSS over the same markup. Nothing here is a stub, and the phase found and fixed four separate silent failures of its hardest criterion before I arrived.

Two things keep it from passing.

**GAL-03 has a fifth door.** The phase's own remediation standard — fix it, or at minimum record it in WINDOWS.md — was applied to the admin preview, which is narrower in reach than this. `internal/public/pluginhost.go` hands a plugin the stored `content_html`, markers and all. I proved it by running it, not by reading it. It is the same shape as CR-03 and it is in none of the phase's paperwork.

**QUAL-01 is met by the number and not by the sentence.** The gate is green, and I re-ran it to be sure it is green here and not only in a summary. But this phase minted five German operator-facing strings in a shape CLAUDE.md already documents as invisible to the collector, and they print on a real admin screen. The phase found this itself and filed it open; that is good practice and it is not closure. This is precisely the failure family the phase's own `render.go:398` comment now names — *marked, collected, translated four times, and printed in German anyway* — and it is worth saying plainly that a criterion reading "0 offen, 0 verwaist" cannot, in principle, be the whole of "every new string in all five languages".

Both gaps are small in code and specific in remedy. Neither undoes the phase.

---

_Verified: 2026-09-08_
_Verifier: Claude (gsd-verifier)_

## Nachtrag 2026-09-10 — was sich seit dem Bericht bewegt hat

Nachgetragen beim Meilenstein-Audit von v1.6. Der Bericht oben beschreibt den
Baum `062c002`/`2061e7e` und bleibt stehen; die Lücken im Kopf sind die von
damals. Der Status bleibt `gaps_found`, weil eine der zwei Lücken offen ist.

| Punkt | Damals | Jetzt | Beleg, beim Audit nachgefahren |
|---|---|---|---|
| SC2 / GAL-03, der fünfte Pfad | ✗ `pluginhost.go:83` gab `ContentHTML` roh an ein Plugin | ✓ geschlossen | Rotbeweis `f95e864`, Flick `c113c22` (beide 2026-09-08): `expandForPlugin` in `internal/public/pluginhost.go:65`, aufgerufen auf `:126`. `go test ./internal/public/ -run 'Marker\|Plugin' -count=1` → `TestPagesGetHandsNoRawMarkerToAPlugin` PASS, dazu die vier Markertests für Feed, gelöschtes Album, fehlenden Speicher und fehlschlagende Abfrage |
| GAL-05 im Hauptbuch | `REQUIREMENTS.md:180` stand auf `- [ ]` | ✓ `- [x]`, Tabelle „Complete" | `REQUIREMENTS.md:180` und `:303` |
| SC6 / QUAL-01, fünf Sätze | ✗ fünf `fmt.Sprintf`-Warnungen in `importAlbums` | ✗ **unverändert offen** | dieselben fünf `fmt.Sprintf` in `internal/bundle/import.go` gezählt; Fensterbuch 6 steht auf `open` |
| Fensterbuch 8 (`textGallery` immer Deutsch) | offen | offen | `internal/block/render.go:212`, Fensterbuch 8 `open` |
| GAL-07, Wortlaut | an Phase 12 übergeben | an Phase 12 übergeben — **Phase 12 liegt seit `48865f2` in v2.0** | `ROADMAP.md` |

**Neuer Stand: 5 von 6.** Die verbleibende Lücke ist QUAL-01 im ersten Satz
(„every new string"), nicht in der Zahl. Sie geht als benannte technische Schuld
in den Meilenstein-Abschluss: der Flick ist die Form Code plus Argumente, die
`.planning/GLOSSARY.md` für csvimport schon vorschreibt (D-32), und gehört mit
den 828 unkatalogisierten Zeichenketten zu Phase 12, Kriterium 9.
