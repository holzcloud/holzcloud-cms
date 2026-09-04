---
gsd_state_version: 1.0
milestone: v1.6
milestone_name: Inhaltsmodell und Zugang
current_phase: 6
current_phase_name: Aufräumen
status: executing
stopped_at: Completed 06-02-PLAN.md
last_updated: "2026-09-04T11:15:35.000Z"
last_activity: 2026-09-04
state_head: 6a5bc2f1d378e928784b4fa26100c5662ad9a4fb
progress:
  total_phases: 5
  completed_phases: 0
  total_plans: 7
  completed_plans: 4
  percent: 0
---

## State: Holzcloud CMS

### Project Reference

- Core value: One Go binary runs several websites without dependency soup
- Current focus: Phase 6 — Aufräumen (v1.6 Inhaltsmodell und Zugang)
- Constraints: Go + htmx + plain CSS + SQLite only — no deviations without explicit user approval
- Stack is a hard mandate: modernc.org/sqlite (pure-Go), html/template, log/slog, embed.FS, gorilla/csrf, alexedwards/scs, pressly/goose, goldmark, bluemonday
- Nothing loads at runtime: no CDN, no web fonts by URL, no third-party subresource of any kind

### Current Position

Phase: 6 — Aufräumen (in progress, 4 / 7 plans complete)
Plan: 06-02 complete — Welle 2 ist fertig: `tools/wasm -print-hashes` baut die sechs Wasm-Module reproduzierbar, und D-05 ist gemessen statt zitiert (PASS, Lauf 33866318077). Welle 3 (06-05) ist frei
Status: Executing Phase 6
Last activity: 2026-09-04

### Milestone Map

**v1.6 — Inhaltsmodell und Zugang.** Phases 6–10. Numbering continues from v1.0
and never restarts; the five v1.0 phase directories are archived under
`.planning/milestones/v1.0-phases/`. There is no v1.5 milestone shell — its three
phases were renumbered into this one as 7, 8 and 9.

| Phase | Name | Requirements | Count | Status |
|-------|------|--------------|-------|--------|
| 6 | Aufräumen | MAINT-01…05 | 5 | In Progress (4/7 plans) |
| 7 | Field Kinds | FIELD-01…08 | 8 | Not started |
| 8 | Snippets Carry Fields | SNIP-01…05 | 5 | Not started |
| 9 | CSV Import | IMP-01…10 | 10 | Not started |
| 10 | Authentik Forward-Auth | SSO-01…11, QUAL-01, QUAL-02 | 13 | Not started |

**Execution order: 6 → 7 → (8 ∥ 9) → 10.** The one real dependency inside the
milestone is that Phase 9 needs Phase 7's multi-value encoding (Phase 7's build
step ①). Once that lands, Phases 8 and 9 are independent of each other and may
run in parallel. Phase 10's file set is disjoint from every other phase's and
can move anywhere; it is scheduled last because it carries the milestone's
close-out gate.

**UI phases:** 7, 8 and 9 all involve admin screen work. Phase 9's mapping and
dry-run screens are the largest UI surface in the milestone. Phases 6 and 10 are
not UI phases.

**Research flags:** Phase 7 wants a UI-SPEC for `bereich` alone (the
invisible-slider problem has three candidate answers and the choice is a design
question). Phase 9's dry-run report screen is a real information-design problem.
Phase 10 wants `/gsd-discuss-phase` for the whole phase — not for lack of
research but because every remaining question there is a policy decision.
Phases 6 and 8 follow standard patterns.

**Migration numbers claimed:** Phase 7 takes `00046`, Phase 8 takes `00047`.
Phases 6, 9 and 10 need none.

Coverage: 41 / 41 requirements mapped. Orphans 0, duplicates 0.

### Performance / Quality Notes

- Hard stack: Go 1.26 + htmx 2.x + plain CSS + SQLite (modernc.org/sqlite) — no exceptions
- UI aesthetic: schlicht/modern (Linear/Ghost/Vercel feel), OKLCH color tokens, @layer cascade, 8px spacing scale, system font stack, view transitions
- Target: linux/amd64 single binary (retargeted from arm64/Pi on 2026-09-03)
- Go patterns: 1.22+ stdlib ServeMux, slog structured logging, embed.FS for all assets/templates/migrations
- SQLite: dual-pool (write pool MaxOpenConns=1, read pool higher), WAL + busy_timeout=5000 + foreign_keys=ON on every connection
- Migrations stand at 00045 (`00045_pages_locale_unique.sql`); Phase 7 takes 00046 (`darstellung`, `max_werte`, `bereich` bounds), Phase 8 takes 00047 (`snippet_id` + index swap + `snippets.fields`). Phases 6, 9 and 10 need none

### Accumulated Context

#### Decisions

- Stack pivot from PHP + SQLite to Go + htmx + SQLite; PHP implementation preserved on legacy/php-stack branch, not ported file-by-file
- modernc.org/sqlite chosen over mattn/go-sqlite3 for pure-Go cross-compilation (no CGO)
- alexedwards/scs chosen for server-side sessions stored in SQLite (no Redis dependency)
- gorilla/csrf chosen for CSRF; htmx integration requires hx-headers on <body> element — hidden form fields are NOT sent by htmx AJAX requests
- goldmark + bluemonday pipeline for Markdown: goldmark renders to raw HTML, bluemonday sanitizes before template.HTML cast (never skip sanitization)
- goose for migrations: SQL files embedded via embed.FS, run automatically at startup
- v1.0: 5-phase coarse roadmap, Foundation → Auth+Shell → Multi-Site+Pages+Public → Templates+Menus+Media → Polish+Users+Deploy
- v1.6: 5-phase coarse roadmap, Aufräumen → Field Kinds → Snippets Carry Fields → CSV Import → Authentik Forward-Auth. The middle three follow the milestone goal as PROJECT.md states it: every field kind, in every carrier, then content as a table
- v1.6: QUAL-01 (five languages) and QUAL-02 (browser pass) are standing gates, not deliverables. Counted once in the last phase for traceability; repeated verbatim as the final success criterion of every other phase so no phase can close without them
- v1.6: snippet fields reuse `page_field_defs` with a `snippet_id` column — explicitly not a third field table
- v1.6: `darstellung`, `max_werte` and the `bereich` bounds get their own columns rather than riding in `auswahl` — a line that is not an option is exactly the ambiguity the one-value-per-line encoding was careful to avoid. Phase 7 therefore ships a migration
- v1.6: a multi-valued field is stored one value per line in the existing string slot, and crosses a CSV cell as a pipe. The delimiter is already illegal inside a value because `SplitChoices` reads options one per line — correct for closed vocabularies, which is all v1.6 has
- v1.6: an Authentik session satisfies the second factor unconditionally (operator's decision, 2026-09-03). One home: `auth.MustHaveSecondFactor` at `internal/auth/twofactor.go:44`. The dependency must be stated in DEPLOY.md and shown in the admin
- v1.6: the CSV import offers both an existing website and a new one, chosen on screen 1 (operator's decision, 2026-09-03) — a deliberate departure from `wordpress.go`'s always-a-new-website rule, answered by the update-or-skip choice plus the dry run
- README's `## License` said MIT while LICENSE carries the full GNU AGPL-3.0; corrected to AGPL-3.0 with a link to LICENSE. A documentation-defect fix, not a relicensing — revert commit d089e3d if MIT was ever the intent
- CHANGELOG.md follows the Keep a Changelog skeleton but writes entries as full sentences, matching the register of SECURITY.md and CONTRIBUTING.md; the choice is stated at the top of the file so the next entry does not revert to bullets
- The public record begins at v1.4 and no pre-1.4 releases are invented. README, CONTRIBUTING and CHANGELOG all say development happened in a private repository first; none of them names that repository or its visibility

#### Known Risks

- FIELD-02 (multiple choice) is the first field value that is not a single string. The encoding chosen in Phase 7 is load-bearing for Phase 9's importer — decide it before either phase writes code
- A CHECK constraint at the table head cannot be loosened in SQLite without a full table rebuild, and `pages` has foreign-key children. `page_field_defs.art` carries no CHECK, so a new field *kind* needs no migration — but Phase 7 ships 00046 anyway, because `darstellung`, `max_werte` and the `bereich` bounds were given their own columns rather than being squeezed into `auswahl`. `users.role` DOES carry a table-level CHECK at `00001:7`: Phase 10 must not invent a third role
- Phase 8's `snippet_id` column collides with the partial unique index `idx_page_field_defs_kennung_oben` — an index swap, not a rebuild. **That index was already replaced by `00038:52–56`, not left as `00029` wrote it.** Read 00029 AND 00038 before writing 00047; 00038 is a line-for-line template and its own comment explains the operation
- No JavaScript beyond htmx: the button row, the multiple choice and the slider must all work as plain form controls with a full-page fallback
- Every new user-visible string must land in de/en/es/fr/it; `go run ./tools/i18n` must say `0 offen, 0 verwaist`
- Draft page leakage: always include AND status='published' in every public page query — applies to a Term or Ref field resolving on the public site too
- Vary: HX-Request header required on any handler returning different content based on HX-Request
- The defects this project actually shipped were found in the browser, not by the suite — the QUAL-02 pass is not optional

#### Todos

- Phase 6 re-aiming is already done and must not be re-discovered: the i18n catalogues already match the tool's output (verified twice, quick task `260903-bsk`, `.planning/WINDOWS.md`), and all five `plugin.wasm` files are committed with all five tests passing. The defects are the doc comment at `tools/i18n/main.go:287`, the undocumented fact that `fr-CH.json`/`it-CH.json` are never written, and CI never rebuilding the wasm files
- Phase 6 ordering: rebuild-and-hash-compare in CI first, promote the test skips second. Any catalogue reformat is its own commit, proven with a `jq -S` semantic diff
- Phase 8 must edit `internal/field/store.go:53` first — the missing `AND snippet_id IS NULL` puts every snippet field on every page's edit form, silently
- Phase 10 must gate on the existing `web.ClientIPResolver.IsTrustedPeer` (`internal/web/clientip.go:49–52`), not a second peer check; the middleware goes in at `cmd/holzcloud/main.go:968` between `setupGuard` and `requireAuth`

#### Blockers

- (none)

### Quick Tasks Completed

| Datum | Aufgabe | Ergebnis |
|---|---|---|
| 2026-08-30 | uebergabe-kundenwebsite-abschliessen | Eine Kundenwebsite in zwei Bundles aufgeteilt, Übergabe und Zeiger in CLAUDE.md entfernt |
| 2026-09-02 | i18n-kataloge-sauber | en/es/fr/it auf 0 offen, 0 verwaist; 24 Sätze aus Laden und Bestellungen übersetzt, zwei unsichtbare Quelltext-Literale wieder extrahierbar, de-CH nachgezogen |
| 2026-09-02 | dependabot-13-14 | modernc.org/sqlite 1.56.0→1.57.0 und golang.org/x/net 0.57.0→0.58.0 gemergt; go.mod-Konflikt aufgelöst, 44 Wanderungen gegen echte DB-Datei geprüft (wal/5000/1/1, integrity ok), Punkt 7 der Arbeitsliste abgeschrieben |
| 2026-09-03 | theme-holzcloud | Achte eingebaute Vorlage „Holzcloud": das Design von holzcloud.ch als Theme, 15 Dateien plus vier woff2 (Manrope, JetBrains Mono, SIL OFL), Design-System byte-gleich übernommen, `template check` ohne Befund |
| 2026-09-03 | theme-holzcloud-sichtpruefung | Im Browser durchgeklickt (Start, 404, schmal): Design trifft, Menü ohne JS. Ein Fehler gefunden und behoben — home.html druckte .Page.Excerpt im Aufmacher, der Text stand dadurch zweimal auf jeder Seite |
| 2026-09-03 | x86-statt-arm | Raspberry Pi und arm64 aus Bauplan und Beschreibung entfernt, CI baut linux/amd64; neue release.yml veröffentlicht auf `v*`-Tag mit Binär und Prüfsumme; Race-Timeout im Security-Audit auf 30m; beide Workflows wieder aktiv, CI erstmals seit 22. August grün |
| 2026-09-03 | haertung-nach-der-freigabepruefung | Go auf 1.26.6 (govulncheck: 8 aufgerufene Lücken → 0), govulncheck als Schritt in security.yml, AGPL-§13-Hinweis mit Fassung und Quellverweis in der Verwaltung (in fünf Sprachen, im Browser geprüft), zwei versehentliche Bauergebnisse aus dem Arbeitsbaum |
| 2026-09-03 | beispiel-bundle-statt-kundendaten | Die zwei Kundenwebsites in ein eigenes privates Repository ausgezogen; an ihrer Stelle das erfundene Bundle sites/beispiel (Velowerkstatt, vier Seiten, zwei selbst erzeugte Bilder), sites/ von 33 MB auf 108 KB; Kundendaten in elf weiteren Dateien mitgeräumt; neuer Packtest. Historie unverändert — die Blobs bleiben erreichbar |
| 2026-09-03 | kleinigkeiten-aus-der-freigabepruefung | CODE_OF_CONDUCT, Issue- und PR-Vorlagen, Zeile zu den KI-verfassten Commits in CONTRIBUTING, alle drei Actions auf Commit-SHA gepinnt, fremde Domain holzbau.ch in neun Dateien durch example.* ersetzt. Dabei ein flatternder Bestandstest gefunden (1,25 %), abgelegt statt behoben |
| 2026-09-03 | zwei-abgelegte-fehler | mkbundle prüft die Schlagwörter jetzt gegen die Namen statt die Kennungen — der Import las die Kennung ohnehin nie; das Beispiel-Bundle trägt endlich Schlagwörter, darunter eines mit Name ≠ Kennung als stehende Absicherung. Der flatternde Formulartest prüft jetzt die Form statt zwei Zeichen im Rauschen, 400 Läufe grün. Vermerk: die Formular-Testreihe überspringt sich stumm ohne gebautes plugin.wasm |
| 2026-09-03 | historienbereinigung | `git filter-repo` hat die zwei Kundenwebsites unter sites/ aus allen Commits getilgt; ein Commit, der nur Fotos enthielt, fiel als leer weg (336→335). Repository 261 MB → 20 MB, E-Mail in 0 Blobs, Tests grün. force-Push auf main, v1.1 und v1.4 mitgeschrieben, die zwei claude/*-Zweige gelöscht. **Offen: GitHub liefert die alten Objekte weiter aus, bis es aufräumt** |
| 2026-09-03 | dokumente-fuer-den-umzug | README-Abschnitt „Versionen" und CONTRIBUTING-Abschnitt zur KI-Autorschaft auf das frische Repository umgeschrieben, CHANGELOG.md mit 1.4 als erster öffentlicher Fassung angelegt. Nebenbei berichtigt: README nannte unter „License" MIT, während LICENSE, CONTRIBUTING und VENDOR.md AGPL-3.0 sagen |
| 2026-09-03 | dokumente-fuer-den-umzug-ins-frische-rep | README „Versionen" erklärt jetzt den einen Commit statt des toten Zweigs archive/gsd-v1.1-dead und der Tags v1.0–v1.3; CONTRIBUTING trägt die KI-Aussage ohne den Zählbefehl, der in einem Ein-Commit-Repository eine Zeile ausgibt; neues CHANGELOG.md (deutsch, Keep-a-Changelog-Gerüst, Sätze statt Stichworte) mit 1.4 als erstem und einzigem Eintrag. Dabei korrigiert: README nannte MIT, während LICENSE die AGPL-3.0 trägt |
| 2026-09-03 | vorlage-weide-neu-gestalten | Die Vorlage `weide` ins Helle übersetzt: warmes Papier #FAF6EF, Erdbraun #6E4D32, Manrope aus dem Repository statt Systemschrift. style.css von 2332 auf 1657 Zeilen neu (vier @layer, dann die schichtlose CMS-Schicht, dann .Site.Design), alle zwölf Ansichten in der Bauteil-Sprache .hc-*, die neun Bausteinarten über die zehn Zeilen der Brücke. Kontraststufe auf hellem Papier neu gemessen: 64 % statt der 52 % aus holzcloud. Drei Fehler nebenbei behoben — fehlendes Druck-Stylesheet, durchgeisternde Kopfleiste, ein langer Betriebsname sprengte die Leiste auf dem Telefon. Nach der Sichtprüfung zwei Befunde nachgezogen: die Navigation unter 1000 px war ein Treppenmuster (oberste Liste blieb eine Flex-ZEILE, ein <li> mit Untermenü riss sie auf) — jetzt beide Ebenen als Spalte, dazu der Checkbox-Umschalter zurück, oberhalb der Schwelle auf display: none; und ein Bild allein im Absatz spannt bis breit-ende, weil eine Seite aus reinem Markdown sonst 62 % der Breite leer lässt. Nach dem Umstellen des Inhalts auf Bausteine zwei weitere Fehler: Überschriften in Bausteinen blieben klein (Kindselektor traf sie nicht — eine Überschrift über einem `bildtext` kann nur im Markdown des Bausteins stehen, weil es dort gar kein Titelfeld gibt), und `.hc-karten` fehlte in der Breit-Regel, weshalb drei Karten in der 705-px-Textspalte auf 2+1 umbrachen. Danach derselbe Befund ein drittes Mal für `.hc-bildtext` (Bild neben Text auf 2 x 340 statt 2 x 577 px); der Kommentar nennt seither die Regel statt der Liste — breit ist, was mehrspaltig ist oder ein Bild in natürlicher Grösse zeigt, schmal bleibt, was gelesen wird, und `.hc-aufruf` bleibt absichtlich schmal. Zuletzt die Bildgrösse selbst: kein Hochrechnen mehr (eine 235-px-Bildmarke wurde 2,4-fach gezogen) und eine Höhengrenze (ein Hochformat stand auf 569 x 1012), für die drei Bildarten, die nicht zugeschnitten werden — Zuschnitt scheidet aus, weil render.go das object-position nur für Karte und Galerie liefert. bausteine.css blieb unangetastet. Zum Schluss zwei Befunde von der Schwestervorlage: ein Aufruf mit vier Absätzen stand zentriert (jetzt linksbündig ab dem zweiten Absatz oder einer Liste, der kurze Zuruf bleibt mittig), und `.hc-bild--breit` mass 810 statt 1120 px, weil der Prozentüberzug des Kerns gegen das Raster kämpfte — für ein Bild hat breit jetzt drei benannte Stufen. Dabei ein eigener Fehler berichtigt: die Höhengrenze traf auch `--voll`. Zuletzt der schwerste Befund: das Untermenü des vorletzten Hauptpunktes ragte über den rechten Rand und liess damit JEDE Seite waagrecht scrollen, ohne dass jemand das Menü öffnete — ein absolut gesetztes Element zählt auch mit `visibility: hidden` zur Scrollbreite. Die Untermenüs der letzten drei Punkte hängen jetzt nach links; in headless Chrome gegen die echten Dateien gemessen, 1280/1100/1024 px je 0 px Überlauf, beide Enden mit einem langen und einem kurzen Prüfmenü nachgewiesen |
| 2026-09-03 | vorlage-rudel-neu-gestalten | Die Vorlage `rudel` als gruene Schwester der hellen `weide`: Papier #F7F6F0, Waldgruen #325737, Manrope, Rundung 10 px und Textmass 66 ch nach dem Manifest der Website statt nach dem Entwurf. Kontrast selbst nachgerechnet (Tinte 15.16, schwaechste Stufe 64 % = 4.80, Marke 7.59 auf Papier und 7.72 auf einer Karte). Alle vier Befunde aus der Sichtpruefung von `weide` sind hier im ersten Wurf drin statt nachtraeglich geerbt: Ueberschriften in Bausteinen, Breit-Regel fuer `.hc-karten` und `.hc-bildtext`, Hoehengrenze im Bildtext, kein Hochrechnen kleiner Bilder. Dazu eine Besonderheit dieser Website: ihr Markdown enthaelt handgeschriebenes HTML — `<div><section>` als Kartenreihe, `<aside>` als Aufruf —, das bluemonday durchlaesst; die Vorlage kleidet diese Uebergangsform mit, damit die Startseite nach der Neugestaltung nicht schlechter aussieht als vorher. Zwoelf der zwanzig Seiten sind Tierportraets, deshalb ist die Hoehengrenze hier die tragende Regel und nicht die Vorsichtsmassnahme |

### Performance Metrics

**Velocity:**

- v1.0: 13 plans across 5 phases, all complete 2026-04-14
- v1.6: 0 plans complete

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase quick-260903-ato P01 | 35m | 3 tasks | 35 files |
| Phase quick-260903-hkh P01 | 7m | 3 tasks | 3 files |
| Phase quick-260903-rqq P01 | 121m | 3 tasks | 18 files |
| Phase quick-260903-t0s P01 | 26m | 3 tasks | 18 files |
| Phase 6 P01 | 6 min | 1 tasks | 2 files |
| Phase 06 P03 | 8 min | 3 tasks | 2 files |
| Phase 06 P04 | 27 min | 3 tasks | 11 files |
| Phase 06 P02 | 13 min | 2 tasks | 2 files |

### Session Continuity

To resume: read `.planning/ROADMAP.md` for the v1.6 phase structure (Phases 6–10)
and its *Standing Gates* section. Requirement IDs are in
`.planning/REQUIREMENTS.md`; the working list most of them came from, with the
size and location of each item, is `docs/offene-punkte.md`.

Next command: `/gsd-discuss-phase 6`

**Last session:** 2026-09-04T11:15:35.000Z
**Stopped at:** Completed 06-02-PLAN.md
**Resume file:** None

## Decisions

- [Phase 6]: Phase 6: the i18n pointer tools/i18n/main.go:287 stands as written — D-17a is WITHDRAWN and a fresh grep confirms 287; what the note now says is that the claim lives in the writeCatalog doc comment, not the package doc comment at :1-15
- [Phase 6]: 06-03: writeCatalog is encoding/json end to end (SetEscapeHTML(false) + json.Indent with two empty strings); all seven committed catalogues are byte-unchanged, proven by test and by jq -S against HEAD
- [Phase 6]: 06-03: the round-trip test locks the FORMAT, not the key set — a cleanly deleted key still round-trips. The guard for a deleted key is the -write + git diff pair that 06-06 installs as CI (D-15)
- [Phase 6]: 06-03: a lock test on already-canonical behaviour cannot fail before the implementation exists, so the RED gate was proven by corrupting a real catalogue (reindent, stripped newline) instead of by absence
- [Phase 6]: Phase 6: MAINT-03 and success criterion 3 cover all six committed .wasm modules including internal/plugin/testdata/echo.wasm, and the four .zip archives are repacked by the same tool (D-07, D-23)
- [Phase 6]: Phase 6: MAINT-05 and success criterion 5 cover all seven codebase maps and point at 06-RESEARCH.md MAINT-05 Correction Inventory instead of naming a count that can itself go stale (D-18, D-19)
- [Phase 6]: Phase 6: the wasm build is tools/wasm, a Go command run as 'go run ./tools/wasm' (D-06); GOTOOLCHAIN is pinned inside the tool with a floor at the root go.mod go directive (D-03, D-03a); -buildvcs=false is mandatory in all four documented invocations (D-02a)
- [Phase 06]: Corrections to the codebase maps are derived by re-running each proof command against current HEAD, never copied from a research document measured at an older commit
- [Phase 06]: CONCERNS.md keeps every judgement; only its numbers moved. The Go-version-mismatch finding stays open because CLAUDE.md:7 still says "Go 1.22+" while go.mod says 1.26.6
- [Phase 06]: A resolved deferred finding is closed by a dated stamp above its text, not by deletion — the reasoning is what a later phase needs
- [Phase 06]: 06-02: D-05 is PASS — the six wasip1 guests hash identically on darwin/arm64 and ubuntu-latest (run 33866318077, 2026-09-04). The byte comparison can be made blocking; the D-05 fallback (-out plus HOLZCLOUD_WASM_DIR) is not needed and must not be planned for
- [Phase 06]: 06-02: tools/wasm forces GOTOOLCHAIN on every build subprocess, proven against ambient local/go1.26.7/go1.27.0 — the guest bytes depend on the goToolchain constant alone, not on setup-go or go.mod. The real trap is D-03a floor: bumping go.mod go directive above the pin breaks the echo build loudly, and only echo
