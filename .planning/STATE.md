---
gsd_state_version: 1.0
milestone: v1.6
milestone_name: Inhaltsmodell und Zugang
status: planning
last_updated: "2026-09-03T20:19:50.109Z"
last_activity: 2026-09-03
progress:
  total_phases: 0
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
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

Phase: Not started (defining requirements)
Plan: —
Status: Defining requirements
Last activity: 2026-09-03 — Milestone v1.6 started

### Milestone Map

(filled in when the roadmap is created)

### Performance / Quality Notes

- Hard stack: Go 1.26 + htmx 2.x + plain CSS + SQLite (modernc.org/sqlite) — no exceptions
- UI aesthetic: schlicht/modern (Linear/Ghost/Vercel feel), OKLCH color tokens, @layer cascade, 8px spacing scale, system font stack, view transitions
- Target: linux/amd64 single binary (retargeted from arm64/Pi on 2026-09-03)
- Go patterns: 1.22+ stdlib ServeMux, slog structured logging, embed.FS for all assets/templates/migrations
- SQLite: dual-pool (write pool MaxOpenConns=1, read pool higher), WAL + busy_timeout=5000 + foreign_keys=ON on every connection
- Migrations stand at 00045 (`00045_pages_locale_unique.sql`); v1.6's first new migration is 00046

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
- README's `## License` said MIT while LICENSE carries the full GNU AGPL-3.0; corrected to AGPL-3.0 with a link to LICENSE. A documentation-defect fix, not a relicensing — revert commit d089e3d if MIT was ever the intent
- CHANGELOG.md follows the Keep a Changelog skeleton but writes entries as full sentences, matching the register of SECURITY.md and CONTRIBUTING.md; the choice is stated at the top of the file so the next entry does not revert to bullets
- The public record begins at v1.4 and no pre-1.4 releases are invented. README, CONTRIBUTING and CHANGELOG all say development happened in a private repository first; none of them names that repository or its visibility

#### Known Risks

- FIELD-02 (multiple choice) is the first field value that is not a single string. The encoding chosen in Phase 7 is load-bearing for Phase 9's importer — decide it before either phase writes code
- A CHECK constraint at the table head cannot be loosened in SQLite without a full table rebuild, and `pages` has foreign-key children. Read `internal/db/migrations/00029` and `00031` before altering an existing table. (`page_field_defs.art` carries no CHECK, so new field kinds need no migration at all)
- Phase 8's `snippet_id` column collides with the partial unique index `idx_page_field_defs_kennung_oben` on `(website_id, kennung) WHERE parent_id IS NULL` — an index swap, not a rebuild
- No JavaScript beyond htmx: the button row, the multiple choice and the slider must all work as plain form controls with a full-page fallback
- Every new user-visible string must land in de/en/es/fr/it; `go run ./tools/i18n` must say `0 offen, 0 verwaist`
- Draft page leakage: always include AND status='published' in every public page query — applies to a Term or Ref field resolving on the public site too
- Vary: HX-Request header required on any handler returning different content based on HX-Request
- The defects this project actually shipped were found in the browser, not by the suite — the QUAL-02 pass is not optional

#### Todos

- (none open — v1.6 requirements and roadmap are being written)

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

### Session Continuity

To resume: read `.planning/ROADMAP.md` for the v1.6 phase structure (Phases 6–10)
and its *Standing Gates* section. Requirement IDs are in
`.planning/REQUIREMENTS.md`; the working list most of them came from, with the
size and location of each item, is `docs/offene-punkte.md`.

Next command: `/gsd-discuss-phase 6`

**Last session:** 2026-09-03
**Stopped at:** Meilenstein v1.6 begonnen — PROJECT.md und STATE.md umgestellt, Anforderungen und Roadmap folgen
**Resume file:** None
