---
phase: 11-galerie
plan: 07
subsystem: i18n
tags: [i18n, catalogue, glossary, browser-pass, lightbox, slideshow, album, accessibility]

requires:
  - phase: 11-01
    provides: "the :target lightbox and its three i18n.N control names in internal/block/render.go"
  - phase: 11-02
    provides: "internal/album with the store, the migration and the two admin screens"
  - phase: 11-03
    provides: "the album admin screens and their layoutPageNames entries"
  - phase: 11-04
    provides: "the late album expansion — the marker, and internal/album/expand.go"
  - phase: 11-05
    provides: "the responsive pipeline running after the expansion, and the slideshow CSS"
  - phase: 11-06
    provides: "export and import of albums through the manifest"
  - phase: 11-review-fix
    provides: "the four Critical and seven Warning fixes this pass had to run after, not before"
provides:
  - "41 German strings of the gallery translated into en, es, fr and it; `go run ./tools/i18n` reports 0 offen, 0 verwaist on all four"
  - "de-CH.json rebuilt with -schweiz (byte-identical: none of the 41 carries a sharp s or German quotation marks)"
  - "four glossary terms — Album, Diashow, Grossansicht, Lichtkasten — and the measured Weiter/Zurück collision"
  - "a twelve-step browser pass against the running binary, after the review fixes, with every outcome written in words"
  - "a Critical the review round missed: album.Store.Create could mint a second album with an existing name after a rename"
  - "one open finding recorded in WINDOWS.md rather than half-fixed at phase close"
affects: [12-umbenennung, verify-work, ship]

actuals:
  tokens: 24231
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "A browser pass driven through a stdlib-only CDP client rather than through a screenshot: every claim below is a number or a string that can be re-measured, because .playwright-mcp/ is gitignored and prose about pictures nobody can check is not a gate"
    - "Create and Rename check a name in the same write transaction, because the UNIQUE constraint on the slug stops seeing a duplicate the moment the slug and the name come apart"

key-files:
  created: []
  modified:
    - internal/i18n/locales/en.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json
    - .planning/GLOSSARY.md
    - internal/album/store.go
    - internal/admin/album.go
    - internal/bundle/album_collision_test.go
    - .planning/WINDOWS.md

key-decisions:
  - "The lightbox's three control names stay their own strings. Measured, not assumed: the catalogue translates Weiter as Continue and Zurück as Back, and both are the wrong word on a picture."
  - "ErrDuplicateSlug is split off from ErrDuplicateName and gets its own sentence, because after a rename an address collision and a name collision are different events with different remedies — and 'an album with this name already exists' sends the operator to a list in which that name does not appear."
  - "The mixed-language slideshow region label is recorded in WINDOWS.md and NOT fixed here: the fix moves the boundary between what a gallery freezes at save and what it resolves at request, which is a Rule 4 decision and not phase-close work."
  - "The Spanish half of the lightbox gate was driven out of band, because the website language select offers only de and en (internal/template/dates.go:63). Said rather than skipped."

patterns-established:
  - "A counting table measured once against the finished tree at phase close, so a count that drifted between five waves is visible before the phase closes"
  - "A browser finding is closed by a test with two mutation probes, the way the rest of this phase closed its findings"

requirements-completed: [QUAL-01, QUAL-02, GAL-01, GAL-02, GAL-03, GAL-06]

coverage:
  - id: D1
    description: "Every German string this phase added ships in en, es, fr and it; de-CH rebuilt by -schweiz; fr-CH and it-CH untouched"
    requirement: QUAL-01
    verification:
      - kind: other
        ref: "go run ./tools/i18n | grep -c '0 offen, 0 verwaist' -> 4"
        status: pass
      - kind: other
        ref: "git status --porcelain internal/i18n/locales/fr-CH.json internal/i18n/locales/it-CH.json | wc -l -> 0"
        status: pass
      - kind: other
        ref: "python3: len(en)==len(es)==len(fr)==len(it)==1319"
        status: pass
    human_judgment: false
  - id: D2
    description: "The lightbox's three control names are translated WHEN RENDERED, not merely present in the catalogue"
    requirement: QUAL-01
    verification:
      - kind: automated_ui
        ref: "browser step 12: website language en -> 'Next image | Previous image | Close large view'; locale es -> 'Imagen siguiente | Imagen anterior | Cerrar la vista grande'"
        status: pass
    human_judgment: false
  - id: D3
    description: "The unstyled case: a gallery page with /assets/bausteine.css blocked is a readable list of pictures with working anchors"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "browser step 8: Fetch.failRequest on bausteine.css; .hc-galerie computes display:block, every gallery element static/visible/in flow, tiles then large views in document order, anchors move the viewport (scrollY 0 -> 3845 -> 5489)"
        status: pass
    human_judgment: false
  - id: D4
    description: "A slideshow adds no horizontal scroll to the page around it, measured at 1280, 1100 and 1024"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "browser step 7: documentElement scrollWidth == clientWidth at all three widths, with and without a large view open, and on all nine pages of the weide site"
        status: pass
    human_judgment: false
  - id: D5
    description: "Both new admin screens render inside the base layout"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "browser steps 1 and 2: 25 nav-item anchors, 'Albums' active, hx-headers on <body>, #flash-area present, <h1> — on the list screen and on the edit screen"
        status: pass
      - kind: unit
        ref: "layoutPageNames parsed: 50 entries, 2 of them album_"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every screen works with JavaScript switched off"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "browser step 9: Emulation.setScriptExecutionDisabled — album create/add/reorder/rename by real mouse and keyboard input, the lightbox's three controls, the block editor's htmx buttons round-tripping as name=bausteinaktion submits, the slideshow scrolling to its maximum"
        status: pass
    human_judgment: false
  - id: D7
    description: "GAL-03 seen: an album changed once changed both pages carrying it, with neither page saved"
    requirement: GAL-03
    verification:
      - kind: automated_ui
        ref: "browser step 6: both page rows byte-identical (sha256 5d324394cea78b85, fa420de03eb33765; updated_at and version unchanged) while both served pages gained a fifth picture and a new ETag"
        status: pass
    human_judgment: false
  - id: D8
    description: "The lightbox: fragment in the URL, absent controls at both ends, back button walking the sequence"
    requirement: GAL-01
    verification:
      - kind: automated_ui
        ref: "browser step 4: #hc-b3-p1..p3; first has no previous, last has no next; close -> #hc-zu matches nothing; six history.back() walk close -> p2 -> p3 -> p2 -> p1 -> grid"
        status: pass
    human_judgment: false
  - id: D9
    description: "Two galleries on one page open the picture that was clicked"
    requirement: GAL-02
    verification:
      - kind: automated_ui
        ref: "browser step 5: hc-b3-p1..p3 and hc-b11-p1..p2, all five ids distinct; clicking the second gallery's first tile resolved :target to hc-b11-p1 inside gallery index 1"
        status: pass
    human_judgment: false
  - id: D10
    description: "album.Store.Create refuses a name a renamed album already carries"
    requirement: GAL-06
    verification:
      - kind: unit
        ref: "internal/bundle/album_collision_test.go#TestCreateRefusesANameARenamedAlbumAlreadyHas"
        status: pass
      - kind: automated_ui
        ref: "browser step 3 re-driven: 'An album with this name already exists' / 'Another album already has the address this name produces' / 'Album created', no SQL text, no duplicate row"
        status: pass
    human_judgment: false
  - id: D11
    description: "The four glossary terms and the Weiter/Zurück collision entered in the commit that first uses them"
    verification:
      - kind: other
        ref: "grep -cE 'Diashow|Grossansicht|Lichtkasten' .planning/GLOSSARY.md -> 3"
        status: pass
    human_judgment: false
  - id: D12
    description: "Every mechanical gate of the whole phase measured once against the finished tree"
    verification:
      - kind: other
        ref: "the counting table below — 19 rows, 18 matching, 1 divergence named and explained"
        status: pass
    human_judgment: false
  - id: D13
    description: "A single album slideshow renders its lightbox controls in the visitor's language and its scroll-region name in German"
    verification: []
    human_judgment: true
    rationale: "Open finding, recorded in WINDOWS.md entry 8 and NOT fixed here. The remedy moves the save-time/request-time boundary of the gallery wrapper, which is an architectural decision (Rule 4) and needs the developer, not a phase-close commit."

duration: 40 min
completed: 2026-09-08
status: complete
---

# Phase 11 Plan 07: The standing gate, and the half no test can hold — Summary

**Forty-one gallery strings translated into four languages and four words entered in the glossary; then twelve browser steps against the running binary that found a Critical the code review had missed and one open defect that every green gate in this repository is blind to.**

## Performance

- **Duration:** ~40 min
- **Started:** 2026-09-08T04:31:00Z
- **Completed:** 2026-09-08T05:11:00Z
- **Tasks:** 2 of 2
- **Files modified:** 9 (4 catalogues, the glossary, 3 Go files, the ledger)

## Accomplishments

- `go run ./tools/i18n` reports **0 offen, 0 verwaist** on en, es, fr and it — 1319 strings each, catalogues of identical length by construction.
- The lightbox's three control names are **translated when rendered**, proved in a browser in two non-German languages, which is the half a catalogue entry cannot prove.
- Twelve browser steps driven against a scratch installation, after the review fixes: the large view with its back button, two galleries minting distinct ids, an album changing two pages nobody saved, a slideshow measured at three widths, a gallery page with its stylesheet blocked, and every screen with scripting switched off.
- **A Critical the review round did not find:** `album.Store.Create` would mint a second album with an existing visible name once the first had been renamed — the exact state `internal/bundle/album_collision_test.go` exists to forbid, reached through the door CR-02 did not close. Fixed, tested, both mutations driven red.
- One open defect recorded rather than half-fixed: an album slideshow shows English (or Spanish) lightbox controls beside a German scroll-region name.

## Task Commits

1. **Task 1: the catalogues and the glossary** — `322f853` (i18n)
2. **Task 2: the browser pass** — `a3a5fe8` (fix), the finding it produced

**Plan metadata:** see the final `docs(11-07)` commit.

_The browser pass itself produces no source commit; its output is this document._

## Task 1 — the catalogues

### The tool's own four steps, in its order

```
go run ./tools/i18n        1318 strings, 41 offen on each of en, es, fr, it
go run ./tools/i18n -write 41 keys added with EMPTY values (not the German source)
   ... translated by hand ...
go run ./tools/i18n -schweiz  de-CH.json  70 nach Regel, 3 von Hand
go run ./tools/i18n        en/es/fr/it  1318 übersetzt, 0 offen, 0 verwaist
```

A second round followed the Task 2 fix, which minted one more string: **1319** in all four.

`-write` leaves an empty value, not the German source — so the "a key holding its German source counts as translated" trap the plan warns about cannot arise from the tool. It can only arise from a translator, and it did not: every one of the 41 was written by hand.

### The three that needed care

`internal/block/render.go` carries **four** `i18n.N` literals, not the three the plan's gate predicted:

```
['Vorheriges Bild', 'Nächstes Bild', 'Grossansicht schliessen', 'Galerie']
all present in the catalogue: True     among them Weiter or Zurück: []
```

- All three control names are present, and **neither is `Weiter` nor `Zurück`** — measured against the catalogue, which translates those two as `Continue` and `Back`.
- The fourth, `textGallery = i18n.N("Galerie")`, is a **deliberate reuse** added by an earlier wave for the slideshow's scroll-region name. Reusing it was right on the sense (a gallery is a gallery) and is what makes it cost no fifth translation. It is also the string that produced this plan's open finding — see below.
- `Galerie` reused: yes, same sense. `Darstellung` **not** reused: the display select's own label already exists as `Darstellung` → `Display`, and the two *option* names are new strings (`Raster` → `Grid`, `Diashow` → `Slideshow`) because neither existed. `Bilder` → `Images` reused throughout. `Bild` → `Image` reused for every flash sentence.

Translations:

| German | en | es | fr | it |
|---|---|---|---|---|
| Vorheriges Bild | Previous image | Imagen anterior | Image précédente | Immagine precedente |
| Nächstes Bild | Next image | Imagen siguiente | Image suivante | Immagine successiva |
| Grossansicht schliessen | Close large view | Cerrar la vista grande | Fermer la vue agrandie | Chiudi la vista grande |
| Diashow | Slideshow | Diapositivas | Diaporama | Presentazione |
| Raster | Grid | Cuadrícula | Grille | Griglia |

### de-CH, and the two files that are only read

`-schweiz` ran and produced a **byte-identical** `de-CH.json` — the file does not appear in `git status`. That is the correct result and not a skipped step: none of the 41 new German strings carries a sharp s or German quotation marks (`Grossansicht schliessen` is already spelled the Swiss way), so the rule has nothing to change. `de-CH.json` still reads `73 Abweichungen, 0 ohne Gegenstück`.

```
git status --porcelain internal/i18n/locales/fr-CH.json internal/i18n/locales/it-CH.json | wc -l
0
```

### One measurement that moved, named rather than adjusted

The gate "no en value equals its German key" reads **27 → 28**. The one new entry is:

```
'Album %s – %s'
```

the page title of the album edit screen. English and German agree on the word *Album*, and any other value would be wrong. The catalogue already carries 27 such entries (`Design – %s`, `Website`, `Text`, `Video`, …), so this is the existing shape and not a new one. Named here because the gate asked for the list and the list is the evidence.

### The glossary

Four terms into the content-model table, with the distinction the phase actually needs:

| Deutsch | Englisch | Anmerkung |
|---|---|---|
| Album | `album` | **Not** `gallery` — the gallery is the block, the album is the stock it shows |
| Diashow | `slideshow` | **Not** `carousel` — nothing turns by itself |
| Grossansicht | `large view` | What the visitor sees |
| Lichtkasten | `lightbox` | The mechanism behind it: `:target` on a `<figure>`, no `<dialog>`, no JavaScript. **Not** a synonym for Grossansicht |

And one line into *Wörter, die eine Falle sind*: `Weiter` and `Zurück` are taken, translated as `Continue` and `Back` since long before this phase, and anyone building a stepper anywhere mints their own strings. Measured at the catalogue on 2026-09-08, not supposed.

## Task 2 — the browser pass

### The precondition

Checked before anything was driven. `git log --oneline` shows the review-fix commits (`932baa7` the report, `2e02be3` CR-01, `fb32335` CR-04, `23e0208` WR-07, `a4af157` WR-06, `6efb3ba` WR-01), and `11-REVIEW.md` and `11-REVIEW-FIX.md` are both committed.

**One honest qualification.** `git status --porcelain` was not empty at the moment the pass started: `internal/admin/forwardauth_test.go` was dirty, uncommitted work of the Phase 10 wave-4 agent running in the same tree. It belongs to no part of Phase 11's user-visible surface and I did not touch it. The condition the precondition is actually about — *no Phase 11 change landed after the pass* — held: the last Phase 11 commit before the pass was `932baa7`, and the only Phase 11 commits after it are my own two.

### The counting table, measured against the finished tree

| What | Measured | Expected |
|---|---|---|
| migrations | **51** | 50 — **divergence, see below** |
| packages under `internal/` | 41 | 41 |
| admin templates | 68 | 68 |
| `layoutPageNames` entries (`album_` of them) | 50 (2) | 50 (2) |
| `adminProtectedMux.Handle` | 159 | 159 |
| `adminOnly` rows | 19 | 19 |
| editor-open rows | 6 | 6 |
| `{{define "icon-` blocks | 27 | 27 |
| `nav-item` anchors | 27 | 27 |
| files using `BeginTx` | 15 | 15 |
| `bundle.Stores{` sites | 1 | 1 |
| fields on `block.Block` / `bundle.Block` / `bundle.Manifest` | 16 / 16 / 13 | 16 / 16 / 13 |
| `/assets/bausteine.css` in the spec | 4 | ≥3 |
| themes linking it | 8 | 8 |
| `bausteine.css`: fixed/absolute, `visibility:`, `:target`, `scroll-snap-type` | 0, 0, 2, 1 | 0, 0, >0, >0 |
| `internal/shop` files changed | 0 | 0 |
| strings in source | 1319 | measured |
| `0 offen, 0 verwaist` lines | 4 | 4 |

**The one divergence, and it is Phase 11's own.** 51 migrations, not 50: the code-review fix round added `00051_album_updated_at.sql` (commit `2e02be3`, CR-01 — the album's timestamp had to reach the page's cache validators). The plan's table was written before that fix existed. Not a drift from another phase, not a number to adjust: `ls internal/db/migrations/*.sql | tail -2` shows `00050_albums.sql` and `00051_album_updated_at.sql`, and `git log` attributes each.

`files using BeginTx` still reads 15 after my own fix added a transaction to `album.Store.Create` — the gate counts *files*, and `internal/album/store.go` was already among them (`Rename`, `SwapSortOrder`).

### The instrument

Chrome 152 headless, driven over the DevTools Protocol through a WebSocket client written in Python stdlib only (no npm, no Node in the loop). Scratch installation: `HOLZCLOUD_DATA_DIR` under the session scratch path, migrations 0 → 51, two archives seeded (`seehof-seewen.zip` on `weide`, `delnahida.zip` on `rudel`), one admin account, sign-in with the compulsory TOTP. **No real data directory was used and the scratch one was removed at the end** (`data dir gone: yes`, `port 8099: free`).

Screenshots were captured for each step into the session scratch directory. They are **not** the evidence: `.playwright-mcp/` and the scratch path are both outside the repository, so every claim below is a number or a string that can be re-measured. That is deliberate and answers this project's standing criticism of browser gates.

The media the pass used were chosen for shape, so the missing-large-variant case at `media/variants.go` was actually exercised: `baechlein-01.jpg` 1200×1600 (portrait), `haus-01.jpg` 1453×817 (landscape), `milchschafe-03.jpg` 900×1600, `milchschafe-04.jpg` 1600×900, and **`kontakt-01.jpg` 235×258 with no variants at all**.

---

### Step 1 — the album screens

Reached from the navigation: `a.nav-item[href$='/albums']`, labelled `Albums`, sitting between Media and Menus with its icon.

```
URL      /admin/websites/1/albums
h1       Albums – Milchschäferei Seehof
nav-item anchors      25
active nav item       Albums
<body hx-headers>     {"X-CSRF-Token": "1jLxBgw…"}
flash region          #flash-area
```

Four pictures added, two with captions. Reordering, and the arrows at the ends:

```
before          1:Ein Bach  2:Das Hofhaus  3:Kleines Bild  4:Milchschafe
up(3)   ->      1:Ein Bach  3:Kleines Bild  2:Das Hofhaus  4:Milchschafe   flash "Order changed"
down(2) ->      1:Ein Bach  3:Kleines Bild  4:Milchschafe  2:Das Hofhaus   flash "Order changed"
first picture's ↑ button  disabled: True
last  picture's ↓ button  disabled: True     (still true AFTER reordering)
```

Renamed to `Sommer 2026 am Seehof`; flash `Album saved`; and the address held: *"The album’s address stays sommer-2026. Pages carrying it do not lose it when it is renamed."* — which is my own new translation of a sentence the template splits around a `<code>` element, rendering correctly.

### Step 2 — the edit screen's layout, as its own observation

The second of the two `layoutPageNames` entries, and the failure mode is silent, so it is recorded separately:

```
URL      /admin/websites/1/albums/1
title    Album Sommer 2026 am Seehof – Milchschäferei Seehof — Holzcloud
nav-item anchors      25
active nav item       Albums
<body hx-headers>     present
flash region          #flash-area
actions block         "Back to the albums"
```

Both screens render inside the base layout. Confirmed by eye in the screenshots and by these five probes, because neither alone can see it.

### Step 3 — the duplicate name

**This is the step that found a Critical.** Driven first, it produced:

```
"Sommer 2026"           -> "An album with this name already exists"   (refused)
"Sommer 2026 am Seehof" -> "Album created"                            ← two albums, one name
```

The second is exactly the state `internal/bundle/album_collision_test.go` exists to forbid. CR-02 had closed `Rename` and its comment justified leaving `Create` alone: *"Create refuses a duplicate through the UNIQUE constraint on the slug."* That sentence stops being true at the first rename — the slug deliberately does not move (GAL-04), so the **old name** is free again under a **different** slug and the `INSERT` walks past the constraint. Fixed in `a3a5fe8`; see *Deviations* below.

Re-driven after the fix:

```
"Sommer 2026 am Seehof" -> "An album with this name already exists"
"Sommer 2026"           -> "Another album already has the address this name produces"
"Herbst 2026"           -> "Album created"
rows afterwards: Herbst 2026 | Sommer 2026 am Seehof
SQL / UNIQUE / constraint text anywhere on the page: none
```

Two different sentences for two different events, and an ordinary create still an ordinary create.

### Step 4 — the inline gallery and the large view

Three pictures on `/hof`, then clicked.

```
grid           hash ""                target none
click tile 1   hash "#hc-b3-p1"       target hc-b3-p1   caption "Das Hofbaechlein"
               controls: [#hc-b3-p2 "Nächstes Bild", #hc-zu "Grossansicht schliessen"]
                                                        ← NO previous control at the first
next -> p2     hash "#hc-b3-p2"       controls: previous, next, close
next -> p3     hash "#hc-b3-p3"       controls: [previous, close]
                                                        ← NO next control at the last
               img currentSrc /media/1/kontakt-01.jpg   ← the 235×258 original, no variants,
                                                          served whole rather than broken
prev -> p2     hash "#hc-b3-p2"
close          hash "#hc-zu"          target none       ← matches nothing, the grid is back
```

The fragment is in the address bar at every step — the evidence that this is `:target` and nothing else. Absent controls at the ends, never disabled ones.

**The back button, six presses:**

```
#hc-zu -> #hc-b3-p2 -> #hc-b3-p3 -> #hc-b3-p2 -> #hc-b3-p1 -> (grid) -> off the page
```

Exactly the sequence in reverse, one step at a time, and only then does the browser leave the page.

### Step 5 — two galleries on one page

```
G0  tiles #hc-b3-p1,#hc-b3-p2,#hc-b3-p3    views hc-b3-p1,hc-b3-p2,hc-b3-p3
G1  tiles #hc-b11-p1,#hc-b11-p2            views hc-b11-p1,hc-b11-p2
all five ids unique: True
```

The two fragment id prefixes are **`hc-b3-`** and **`hc-b11-`** — quoted because D-03's whole reason is that they differ. Clicking the **second** gallery's first tile:

```
hash #hc-b11-p1   :target hc-b11-p1   caption "Zweite Galerie, Bild eins"
containing gallery index: 1            alt "Das Hofhaus, zweite Galerie"
```

That picture, in that gallery.

**One incident worth recording.** The first attempt read `galleries: 1` and I nearly wrote down "the second gallery does not render". The database said otherwise (`gross ids: hc-b3-p1..p3, hc-b11-p1, hc-b11-p2`), and the cause was the browser's own HTTP cache: the public page carries `Cache-Control: public, max-age=300`, so a five-minute freshness window had not expired. Not a defect and not Phase 11's — but the reason every public-page measurement after this point ran with `Network.setCacheDisabled`.

### Step 6 — the album on two pages, and nobody saved a page

The album was placed on `/` (home) and on `/milchschafe`. Both showed its four pictures in the reordered order.

Then **one picture was added to the album and nothing else was touched**:

```
                       BEFORE                                    AFTER
page 1 content_html    sha256 5d324394cea78b85                   sha256 5d324394cea78b85
page 1 updated_at      2026-09-08T04:49:38Z                      2026-09-08T04:49:38Z
page 1 version         2                                         2
page 3 content_html    sha256 fa420de03eb33765                   sha256 fa420de03eb33765
page 3 updated_at      2026-09-08T04:49:43Z                      2026-09-08T04:49:43Z

served /              ETag "b3e2a46abb998ade474c642c42f4ed0e" -> "91215eff4eb7811bc6a515cb002ce525"
served /milchschafe   ETag "c9d9d8a600f8acca78f08a6129b763a9" -> "b7d6db603032ac604cddc76afb0485b7"
Last-Modified on both moved to 04:50:21 — the album's timestamp, not the page's

in the browser, both pages:
  ids   hc-b10-p1..p5   and   hc-b8-p1..p5
  alts  Ein Bach / Kleines Bild / Milchschafe / Das Hofhaus / Huehner, im Album ergaenzt
```

Both halves, and the second is the one that matters: the same rows in the database, two different pages on the net.

**CR-01 driven at the same time,** because a warm browser is the case that would make GAL-03 false anyway:

```
GET / with the OLD ETag              -> 200   (fresh content)
GET / with only the OLD date         -> 200
GET / with the CURRENT ETag          -> 304
```

### Step 7 — the slideshow, and the width

Switched to `Diashow` in the admin, hint rendered from my own new translation. The rendered container:

```
class     hc-block hc-galerie hc-spalten-3 hc-galerie--diashow
display   grid          grid-auto-flow  column
overflowX auto          scroll-snap-type  inline mandatory
children  <figure> with scroll-snap-align: start
role      region        tabindex 0        aria-label "Galerie"
```

**The measurement, against the real files, the way `.planning/STATE.md:209` was measured:**

| width | `documentElement.scrollWidth` | `clientWidth` | excess |
|---|---|---|---|
| 1280, large view closed | 1280 | 1280 | **0** |
| 1100, large view closed | 1100 | 1100 | **0** |
| 1024, large view closed | 1024 | 1024 | **0** |
| 1280, large view open | 1280 | 1280 | **0** |
| 1100, large view open | 1100 | 1100 | **0** |
| 1024, large view open | 1024 | 1024 | **0** |

And every page of the `weide` site at 1024, because STATE.md:209's finding was about a whole theme:

```
/ 1024/1024   /hof 1024/1024   /milchschafe 1024/1024   /alles-vom-schaf 1024/1024
/wolle 1024/1024   /fleisch-und-milch 1024/1024   /zuchttiere 1024/1024
/schulklassen 1024/1024   /kontakt 1024/1024
```

Excess 0 everywhere. The tiles inside the track do stick out past `clientWidth` — that is the scroll container doing its job, and the document not moving is what proves the difference.

**Movement and snapping,** with real input events:

```
ArrowRight ×1   scrollLeft 0 -> 376     distance to nearest snap point: 0
ArrowRight ×2   scrollLeft   -> 678     (678 = 1856 − 1178, the maximum; 74 from a snap point,
                                         which is what a snap track does at its end)
trackpad push deltaX=350   scrollLeft -> 376   snap distance 0   ← 350 pushed, 376 landed
touch drag −400px          scrollLeft -> 376   snap distance 0   ← 400 dragged, 376 landed
```

The last two lines are the snap as a number: the gesture was 350 and 400 pixels, the track came to rest at 376 both times.

### Step 8 — the stylesheet blocked

`/assets/bausteine.css` failed at the network layer (`Fetch.failRequest`, `BlockedByClient`), verified by what the page could still reach:

```
sheet rule counts     bausteine.css=BLOCKED   style.css=277   inline=1
.hc-galerie           computed display: block        (grid when the sheet loads)
.hc-galerie__gross    computed display: block        (the large views are simply visible)
```

**In a sentence: the page becomes an ordinary document.** The three tiles as figures, then the same three pictures larger, each under its own caption, stacked one below the other down the page — a list you scroll, not a stack of boxes over anything.

```
gallery children in document order   0:bild 1:bild 2:bild 3:gross 4:gross 5:gross
every gallery element                static, visible, in the normal flow — none positioned,
                                     none display:none, none visibility:hidden, none opacity 0,
                                     none with a z-index
captions                             "Das Hofbaechlein" | "Das Wollschwein" | "Das kleine Original"
                                     | "Zweite Galerie, Bild eins" | "Zweite Galerie, Bild zwei"
tops of the large views              3960 < 5500 < 6491 < 9947 < 10641   (strictly increasing)
page scrollWidth/clientWidth         1280/1280
```

**And the anchors work with no block stylesheet at all:**

```
scrollY 0 -> click tile 1 -> hash #hc-b3-p1, scrollY 3845, target 115px below the top
            -> next        -> hash #hc-b3-p2, scrollY 5489, target  11px below the top
            -> close       -> hash #hc-zu, no target
```

The slideshow page unstyled degrades further and correctly: `overflow-x: visible`, `scroll-snap-type: none`, `scrollWidth == clientWidth` on the container itself — no sideways track at all, just five tiles and five large views down the page.

### Step 9 — scripting off

`Emulation.setScriptExecutionDisabled`. Everything below was driven with **real mouse and keyboard events**, not with `element.click()`, because with no script engine there is nothing to call.

**The lightbox (step 4 repeated):**

```
htmx present in the executing page   False        <script> tags   1 (the ld+json data block)
real click on tile 1     URL -> /hof#hc-b3-p1
real click on "next"     URL -> /hof#hc-b3-p2     ← the selector only resolves if :target opened
real click on "close"    URL -> /hof#hc-zu
```

**The album screens (step 1 repeated):**

```
album list, no scripting     25 nav items, hx-headers on <body>: True
type a name + press Create   flash "Album created", row "Ohne Skript 2026" appears
edit screen, no scripting    25 nav items, #flash-area present
select a picture + submit    flash "Image added", 6 picture forms
press ↑ on the last picture  flash "Order changed"
rename + submit              flash "Album saved", the <h1> follows
```

**The block editor,** which is the htmx-heaviest screen in the phase. Its buttons turn out to be honest submits:

```
<button type="submit" name="bausteinaktion" value="neu:galerie"  hx-post=… hx-target="#bausteine">
<button type="submit" name="bausteinaktion" value="neu-e:2"      hx-post=… hx-target="#bausteine">
<button type="submit" name="bausteinaktion" value="weg-e:2:0"    hx-post=… hx-target="#bausteine">

real click on "Gallery" with no scripting:  blocks 11 -> 12, full page round trip
```

htmx swaps a fragment; without it the same POST round-trips the whole page. Enhancement only, as the convention requires.

**The slideshow (step 7 repeated):** two `ArrowRight` presses and one sideways push, dispatched as raw input with no script engine running, moved the track to `scrollLeft = 1846` of a maximum of `1846`. Native CSS scroll-snap, nothing else.

**The album expansion (step 6 repeated):** the page carried six large views and six ids with no scripting at all, and `[[album:` appears nowhere in the rendered text — the expansion is server-side, so scripting was never in the path.

### Step 10 — a second website

```
/admin/websites/2/albums   h1 "Albums – Delnahida"   rows 0
                           "No albums yet. Create the first one above."
any of website 1's album names visible:  False
```

The foreign album, as an authenticated administrator of website 2, holding a **valid CSRF token taken from website 2's own rendered page**:

```
GET  /admin/websites/2/albums/1              -> 404, 19 bytes, leaks nothing
POST /admin/websites/2/albums/1/update       -> 404, leaks nothing
POST /admin/websites/2/albums/1/delete       -> 404, leaks nothing
POST /admin/websites/2/albums/1/pictures     -> 404, leaks nothing
POST without a CSRF token                    -> 403 ("CSRF token not found in request")
album 1 afterwards: name "Sommer 2026 am Seehof", 6 pictures — untouched
```

GAL-05, seen: the store's WHERE clauses, the handler's checks and the route authorisation exercised together in one browser for the only time.

### Step 12 — the lightbox in a language that is not German

This is the step the whole i18n half of the phase exists for, and it produced **two results, one of them a defect**.

Before, website language `de`:

```
/      (album gallery, rendered per request)  Vorheriges Bild | Nächstes Bild | Grossansicht schliessen
/hof   (inline gallery, frozen at save)       Vorheriges Bild | Nächstes Bild | Grossansicht schliessen
```

Language switched to **English through the settings form**:

```
/      Previous image | Next image | Close large view          ← immediately, html lang="en"
/hof   Vorheriges Bild | Nächstes Bild | Grossansicht schliessen
```

The inline gallery stays German because block HTML is rendered once at save — which `internal/admin/page_blocks.go` documents in those words ("a website that changes its language re-renders its blocks on the next save of each page — not before"). Confirmed by doing it: re-saving `/hof` with no other change turned it into `Next image | Close large view | Previous image`.

And in Spanish, to prove the catalogue and not just the mechanism:

```
html lang: es
lightbox controls: Imagen anterior | Imagen siguiente | Cerrar la vista grande
```

**Those are my own translations, on a live public page.** A German screen with a translated catalogue entry — the failure this gate exists to catch — did not happen for the three control names.

**It did happen for the fourth string.** On the same page, in the same render:

```
lightbox controls        Next image | Previous image | Close large view      (English)
slideshow aria-label     "Galerie"                                            (German)

and in Spanish:
lightbox controls        Imagen siguiente | Imagen anterior | Cerrar la vista grande
slideshow aria-label     "Galerie"
```

See *Deviations → not fixed* below.

### Step 11 — the round trip, by hand

Exported through the UI link *Download the website*:

```
attachment; filename="milchschaeferei-seehof-2026-09-08.holzcloud.zip"   6 880 104 bytes, 15 entries
manifest albums:
  [{"name":"Herbst 2026"}, {"name":"Ohne Skript 2026"},
   {"name":"Sommer 2026 am Seehof","items":[
     {"media":"baechlein-01.jpg","alt":"Ein Bach im Sommer","caption":"Der Bach hinter dem Hof"},
     {"media":"kontakt-01.jpg","alt":"Kleines Bild ohne Varianten"},
     {"media":"milchschafe-04.jpg","alt":"Milchschafe auf der Weide"},
     {"media":"haus-01.jpg","alt":"Das Hofhaus","caption":"Das Hofhaus von vorn"},
     {"media":"seehof-02.jpg","alt":"Ohne Skript hinzugefuegt"},
     {"media":"huehner-02.jpg","alt":"Huehner, im Album ergaenzt","caption":"Nachtraeglich hinzugefuegt"}]}]
```

Imported as a new website through the form:

```
Import finished — 9 pages and posts, 14 files, 1 menus, 2 snippets, 3 albums
                                                             ↑ my new "%d Alben" -> "%d albums"
warnings: none
```

The imported album carried its pictures, and the address was **re-derived** rather than copied — which is the design (`sommer-2026` on the source, whose name had moved; `sommer-2026-am-seehof` on the target, derived from the travelling name):

```
album 7 on website 3, 6 items, in order, with alt texts and captions intact
markers on the imported pages:  page 31 home        [[album:sommer-2026-am-seehof:9]]
                                page 32 milchschafe [[album:sommer-2026-am-seehof:7]]
```

Both public pages of the new website:

```
/              6 large views, ids hc-b10-p1..p6
/milchschafe   6 large views, ids hc-b8-p1..p6
image sources  /media/3/baechlein-01.jpg … /media/3/huehner-02.jpg    ← the NEW website's library
each of the six fetched directly: 200
raw marker visible: False
```

The five images reading `naturalWidth == 0` on the home page carry `loading="lazy"` and sit below the fold; all six answer 200 when fetched. That is laziness, not a missing file, and it was checked rather than assumed.

## Files Created/Modified

- `internal/i18n/locales/en.json` — 42 entries added (41 in Task 1, 1 after the Task 2 fix)
- `internal/i18n/locales/es.json` — the same 42
- `internal/i18n/locales/fr.json` — the same 42
- `internal/i18n/locales/it.json` — the same 42
- `.planning/GLOSSARY.md` — four content-model terms, one collision note
- `internal/album/store.go` — `Create` checks the name in a write transaction; `ErrDuplicateSlug` split off; `Rename`'s comment corrected where it asserted something false
- `internal/admin/album.go` — `albumSaid` maps the new error to its own sentence
- `internal/bundle/album_collision_test.go` — `TestCreateRefusesANameARenamedAlbumAlreadyHas`
- `.planning/WINDOWS.md` — entry 8, the open finding

## Decisions Made

1. **`Galerie` reused, `Darstellung` reused, `Raster` and `Diashow` minted.** The first two carry the same sense they already had; the two display-option names did not exist. Recorded because the plan asked which reuses were made and why.
2. **`ErrDuplicateSlug` gets its own sentence.** Folding it into `ErrDuplicateName` would tell an operator a name is taken while the list in front of them shows no such name — the one answer that cannot be acted on, which is the argument `album.Store.Delete`'s own comment already makes for its strictness. A mutation probe holds it.
3. **The mixed-language region label is recorded, not fixed.** Rule 4: the remedy moves the boundary between what a gallery freezes at save and what it resolves at request. Doing that at phase close, in a tree with another agent committing to it, would be worse than the defect.
4. **The Spanish evidence is out of band, and says so.** The website language select offers `de` and `en` only (`internal/template/dates.go:63`). English satisfies the gate as written ("a website whose language is not German", "any of the four the catalogues carry"). Spanish was reached by writing `locale='es'` into the scratch database and restarting — clearly marked, because a reader must be able to tell which claim went through the UI and which did not.

## Deviations from Plan

### Auto-fixed

**1. [Rule 1 — Bug] `album.Store.Create` minted a second album with an existing name after a rename**

- **Found during:** Task 2, browser step 3.
- **Issue:** CR-02 closed `Rename` and justified leaving `Create` alone with *"Create refuses a duplicate through the UNIQUE constraint on the slug"*. That holds only while a name still derives its album's slug. The slug deliberately does not move on rename (GAL-04 needs the address to stand still), so the old name becomes free again under a different slug and the `INSERT` passes the constraint. Driven: rename `Sommer 2026` → `Sommer 2026 am Seehof`, create `Sommer 2026 am Seehof`, and the list shows the name twice. That is the state `internal/bundle/album_collision_test.go` exists to forbid — two indistinguishable manifest entries, one album's pictures lost on import, every gallery bound to the survivor.
- **Fix:** `Create` now checks the name in the same write transaction as its insert, for `Rename`'s reason (the read pool is a different WAL snapshot). `ErrDuplicateSlug` split off from `ErrDuplicateName` so an address collision keeps its own sentence. `Rename`'s comment corrected where it asserted the false thing.
- **Files modified:** `internal/album/store.go`, `internal/admin/album.go`, `internal/bundle/album_collision_test.go`, the four catalogues.
- **Verification:** `TestCreateRefusesANameARenamedAlbumAlreadyHas`, with two mutation probes driven red and each restored:
  ```
  MUTATION A: the name check is made and then ignored
    album_collision_test.go:111: Create produced a second album called "Werkstatt 2025" —
        an archive of this website can no longer say which of them a gallery means
  MUTATION B: the slug error folded back into the name error
    album_collision_test.go:132: a slug collision refused with an album with this name
        already exists: "Werkstatt 2024"; want the named ErrDuplicateSlug
    album_collision_test.go:135: a slug collision was reported as a name collision —
        the list shows no such name
  ```
  Then re-driven in the browser: three names, three correct answers, no SQL text, no duplicate row.
- **Committed in:** `a3a5fe8`.

**2. [Rule 3 — Blocker] one new catalogue string**

- **Found during:** Task 2, as a consequence of fix 1.
- **Issue:** the new error needs a sentence, and `go run ./tools/i18n` immediately read `1 offen`.
- **Fix:** `Ein anderes Album hat schon die Adresse, die aus diesem Namen entsteht` translated into all four; `-schweiz` re-run; back to `0 offen, 0 verwaist` on 1319 strings.
- **Committed in:** `a3a5fe8`.

### Found and deliberately not fixed

**3. [Rule 4 — Architectural] one gallery, two languages**

- **Found during:** Task 2, browser step 12.
- **Issue:** an album gallery in slideshow display renders its lightbox controls in the visitor's language and its scroll-region name in German, on the same page, in the same render pass. Measured in two languages:
  ```
  website language en:  Next image | Previous image | Close large view    aria-label="Galerie"
  locale es:            Imagen siguiente | Imagen anterior | …            aria-label="Galerie"
  ```
- **Cause, both call sites:** `internal/block/render.go:212` writes the region name with `s.text` — `block.Set.T`, which `internal/admin/page_blocks.go:105` wires **only at save time**. The controls inside reach `GalleryItems` through `internal/album/expand.go:133`, which passes `set.t`, the **request-time** translator. For an inline gallery both halves are `s.text` and agree; for an album gallery they disagree, because only the marker's contents are substituted at request while the wrapper `<div>` is stored HTML.
- **Why it matters beyond one attribute:** this is precisely the class of failure no gate in this repository can see. `textGallery` is marked with `i18n.N`, collected, translated in four catalogues, and `go run ./tools/i18n` reports `0 offen, 0 verwaist` — and the string is printed in German to every visitor anyway. Its own comment claims the reuse "costs nothing, because the key is translated in en, es, fr and it today". It is never translated when rendered.
- **Blast radius:** the `aria-label` of a slideshow's scroll region. Screen-reader-only text. Real, and narrow.
- **Why not fixed here:** the remedy moves the boundary between what a gallery freezes at save and what it resolves at request — either the wrapper is re-rendered per request, or the marker grows to carry it, or the album path is frozen like the inline one (which would undo the album's live translation and is worse). That is an architectural decision, not phase-close work, in a tree another agent is committing to.
- **Recorded in:** `.planning/WINDOWS.md` entry 8 (`kind: deviation`, `internal/block/render.go:212`, `status: open`), so it blocks `/gsd-ship` rather than scrolling out of context.

---

**Total deviations:** 2 auto-fixed (1 × Rule 1, 1 × Rule 3), 1 recorded under Rule 4.
**Impact on plan:** no scope creep. The Rule 1 fix is a Critical of the same class as CR-02 and was found by the very gate this plan exists to run; the Rule 3 fix is its cost in the catalogue. The Rule 4 finding is written down and left for the developer.

## Issues Encountered

**1. The Phase 10 wave-4 agent had a red tree while this plan ran.** `go test ./...` failed to build `internal/admin` on `internal/admin/forwardauth_test.go` (`undefined: provisionSSOUser`, `randomSecret`) — a deliberate TDD RED commit of plan 10-03/10-05, in a package this plan never touched. Diagnosed rather than worked around: the file was reproduced at HEAD in a throwaway copy of the tree (`git archive HEAD | tar -x`) and the suite ran green with that one file removed. No `git worktree`, no `git stash`, no `git clean`, no blanket reset was used at any point. By the time the final gates ran, the other agent had reached 10-05 and the whole suite is green as recorded below.

**2. `Network.setBlockedURLs` silently did not block.** It reported success and the stylesheet still loaded and still applied. Caught because I measured the consequence (`getComputedStyle(.hc-galerie).display` still read `grid`) instead of trusting the API. Redone through `Fetch.enable` + `Fetch.failRequest`, which produced `bausteine.css=BLOCKED` and `display: block`. Recorded because "I blocked it" is a claim and "the rules are unreachable and the computed style changed" is a measurement.

**3. The browser's own HTTP cache nearly produced a false finding in step 5** — see the incident note there. The public page's `max-age=300` is pre-existing and correct; every measurement after that ran with the cache disabled, and the cache path itself was driven separately in step 6.

## Not driven

Recorded as *not driven* rather than as *passed*, because this project has been bitten by both mistakes:

- **A Spanish, French or Italian website through the settings form.** `internal/template/dates.go:63` lists `de` and `en` and nothing else, so a public website cannot be set to any of the other three through the UI. The gate as written is satisfied (English is one of the four catalogues), and the Spanish render was driven out of band and labelled as such. Whether the website language select should offer the languages the installation actually ships is a question for a later phase, not a Phase 11 defect.
- **A drag with a real finger.** The touch drag was `Input.synthesizeScrollGesture` with `gestureSourceType: "touch"`, which is Chrome's own synthesis and not a hand on glass. It moved the track and it snapped; a physical device was not used.
- **A screenshot of the Spanish gallery.** One was captured against a stale server and would have shown English; it was deleted rather than left to mislead. The Spanish evidence in this document is the rendered HTML, which is stronger and can be re-measured.
- **No `11-UI-SPEC.md` was written.** The plan flagged this as a deliberate substitution and it stands: the accessible shape lives in plan 11-01's constraints and in browser step 4.

## Final gates, on the finished tree

```
git status --porcelain     (only .planning/ changes of this plan)
go build ./...             clean
go vet ./...               silent
gofmt -l .                 silent
go test ./...              0 failures
go run ./tools/i18n        1319 strings
                           de-CH.json   73 Abweichungen, 0 ohne Gegenstück
                           en.json      1319 übersetzt, 0 offen, 0 verwaist
                           es.json      1319 übersetzt, 0 offen, 0 verwaist
                           fr-CH.json    4 Abweichungen — nur gelesen
                           fr.json      1319 übersetzt, 0 offen, 0 verwaist
                           it-CH.json    9 Abweichungen — nur gelesen
                           it.json      1319 übersetzt, 0 offen, 0 verwaist
```

## Coordination

Another agent executed Phase 10 wave 4 and 5 in this working tree throughout. **Every commit here was staged file by file** — no `git add .`, no `git add -A` — and `git show --stat` on both shows only this plan's files:

```
322f853  .planning/GLOSSARY.md, internal/i18n/locales/{en,es,fr,it}.json          5 files
a3a5fe8  internal/admin/album.go, internal/album/store.go,
         internal/bundle/album_collision_test.go, internal/i18n/locales/*.json    7 files
```

No `git clean`, no `git stash`, no blanket reset, no `git checkout -- .` was run at any point.

## Next Phase Readiness

Phase 11 is complete: seven plans, seven summaries. One open finding sits in `.planning/WINDOWS.md` (entry 8) for the developer to decide on, and it is narrow enough not to block the phase.

Phase 12 inherits two things worth carrying: the `Weiter`/`Zurück` collision is now in the glossary's trap list, and the `SupportedLocales` gap (four catalogues, two selectable website languages) is named here for the first time.

## Self-Check: PASSED

- All nine modified files present on disk.
- Both task commits present in `git log` (`322f853`, `a3a5fe8`), each staged file by file.
- `TestCreateRefusesANameARenamedAlbumAlreadyHas` present and green.
- `grep -cE 'Diashow|Grossansicht|Lichtkasten' .planning/GLOSSARY.md` → 3.
- `.planning/WINDOWS.md` entry 8 present: `deviation / 11 / internal/block/render.go:212 / open`.
- `go build ./...` clean, `go vet ./...` silent, `gofmt -l .` silent, `go test ./...` 0 failures.
- `go run ./tools/i18n` → 4 lines reading `0 offen, 0 verwaist`.
