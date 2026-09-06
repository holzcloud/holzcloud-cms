---
gsd_state_version: 1.0
milestone: v1.6
milestone_name: Inhaltsmodell und Zugang
current_phase: 7
current_phase_name: Snippets Carry Fields
status: executing
stopped_at: Completed 07-07-PLAN.md
last_updated: "2026-09-06T09:19:53.421Z"
last_activity: 2026-09-06
state_head: 0376a46ffbfe99574cb2f7a721e771edfc6cafb0
progress:
  total_phases: 6
  completed_phases: 1
  total_plans: 19
  completed_plans: 14
  percent: 17
---

## State: Holzcloud CMS

### Project Reference

- Core value: One Go binary runs several websites without dependency soup
- Current focus: Phase 7 — Field Kinds abgeschlossen; als Nächstes Phase 8 — Snippets Carry Fields (v1.6 Inhaltsmodell und Zugang)
- Constraints: Go + htmx + plain CSS + SQLite only — no deviations without explicit user approval
- Stack is a hard mandate: modernc.org/sqlite (pure-Go), html/template, log/slog, embed.FS, gorilla/csrf, alexedwards/scs, pressly/goose, goldmark, bluemonday
- Nothing loads at runtime: no CDN, no web fonts by URL, no third-party subresource of any kind

### Current Position

Phase: 7 — Field Kinds (7 / 7 Pläne ausgeführt, abgeschlossen)
Plan: 07-07 complete — der per-Art-Zoll ist bezahlt, das stehende Tor ist gefahren, und **D-08 ist durch Beobachtung geschlossen**. `TEMPLATE-SPEC.md` dokumentiert alle fünf neuen Arten samt `.Values`, `.Term` und dem bislang undokumentierten `.Yes`; `SampleData` trägt je eine gefüllte Zeile, `MinimalData` den leeren Zwilling jeder davon — die Grundlage, gegen die die Upload-Prüfung rendert (T-07-28). Vier neue Reflexionswächter halten den Vertrag jetzt selbst zusammen: ein neues Mitglied an `field.Entry` oder eine neue Art in `field.Kinds`, die im Dokument fehlt, fällt im Test auf. 23 neue deutsche Quellsätze in en/es/fr/it, in einem Commit für sich; `fr-CH.json` und `it-CH.json` unberührt. **Die Browserhälfte wurde vom Orchestrator mit dem Playwright-MCP-Server gefahren**, nicht vom Executor: Schritt 3 trägt eine `<precondition>`, die ein Browserwerkzeug verlangt, das dem Executor fehlt — Halten war richtig, die Arbeit wurde geteilt statt in eine Checkliste zurückverwandelt. Gefahren gegen eine Wegwerf-Instanz mit eigener Datenbank, **auf Englisch**, was die schärfere Probe auf die Übersetzungen war. Gesehen: alle vier neuen Definitionsbedienelemente übersetzt und die Grenzenablehnung wirksam; `bereich` als Zahlenfeld mit vor dem Speichern lesbarer Zahl; `zeit` unterscheidet leer von `00:00`; `code` festbreit an der errechneten `font-family`; Mehrfachauswahl mit Wachposten, drei Werte über Speichern und Neuladen, geleert unterscheidbar von nicht-da, Überschreitung als HTTP 422; Schlagwortwähler ohne die Schlagwörter der zweiten Website; Knopfreihe als Radioreihe. Null verwaiste `for=`, beide Gruppen mit auflösendem `aria-labelledby`. **Alle sechs steuernden Arten** blendeten ihr abhängiges Feld korrekt ein und aus — einschliesslich `bereich`: **`MayControl()` bleibt unverändert, `KindRange` bleibt steuernd**, und die Fahrplan-Notiz ruhte auf der Schieber-Annahme, die D-07 verworfen hat. Öffentlich: getipptes `<script>alert(1)</script>` erscheint maskiert und führt nichts aus, Mehrfachwerte als lesbare verbundene Zeichenkette, und eine Schlagwort-Umbenennung änderte, was eine nie neu gespeicherte Seite druckt. Konsole und Serverprotokoll: keine CSP-Verletzung, keine Barrierefreiheitswarnung, drei absichtliche 422. Drei Kommentare, die auf diesen Durchgang vorauswiesen, sagen jetzt, was gesehen wurde — kein Vorwärtsverweis mehr im Baum
Status: Phase 07 abgeschlossen — FIELD-01…08 erfüllt, Abnahme offen
Offen aus dem stehenden Tor (Phase 6): die Übersetzungshälfte ist grün, die **Browserhälfte nur zur Hälfte gelaufen**. Die vier sichtbar rendernden Gäste (`suche`, `kontaktformular`, `jahreszahl`, `bestellung`) wurden **nicht** auf einer öffentlichen Seite gesehen — eine frische Datenbank kennt kein Plugin, und das verfügbare Browserwerkzeug konnte den `.zip`-Upload nicht ausführen
Offen aus dem stehenden Tor (Phase 7): **eine Zeile ungefahren** — `code` innerhalb eines Blocks auf der öffentlichen Seite. Der Blockpfad ist im Test gedeckt (`internal/block`, 07-03), aber nicht im Browser gesehen; als nicht gefahren geführt, nicht als bestanden. Dazu **Fenster Nr. 3**: die Ablehnungsgründe aus `internal/field/field.go` erschienen bei englischer Oberfläche auf Deutsch — vorbestehend, gegen `60ff5b2` geprüft, in `.planning/WINDOWS.md` eingetragen
Last activity: 2026-09-06

### Milestone Map

**v1.6 — Inhaltsmodell und Zugang.** Phases 6–10. Numbering continues from v1.0
and never restarts; the five v1.0 phase directories are archived under
`.planning/milestones/v1.0-phases/`. There is no v1.5 milestone shell — its three
phases were renumbered into this one as 7, 8 and 9.

| Phase | Name | Requirements | Count | Status |
|-------|------|--------------|-------|--------|
| 6 | Aufräumen | MAINT-01…05 | 5 | Plans 7/7 — Abnahme offen (Browserhälfte des Tors) |
| 7 | Field Kinds | FIELD-01…08 | 8 | Plans 7/7 — Abnahme offen (eine Browserzeile, Fenster Nr. 3) |
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
- **The upgrade path from an old installation is broken and no test can see it (found 2026-09-04, own task pending).** Migration `00036_content_types.sql:60-62` builds a partial index on `pages.deleted_at`; the column is added by `00008_revisions_locking_trash.sql:15`. A fresh database migrates 0 → 45 cleanly, but an installation whose recorded version 8 predates an edit to `00008` lacks the column, dies at 36 with `no such column: deleted_at`, and the server refuses to start. goose never re-runs a recorded version and no migration rebuilds `pages`. The fix needs a test that migrates from an old snapshot — a suite that only ever migrates from zero is structurally blind to this. Evidence in `.planning/phases/06-aufr-umen/06-07-SUMMARY.md`
- The defects this project actually shipped were found in the browser, not by the suite — the QUAL-02 pass is not optional
- **Die Ablehnungsgründe in `internal/field/field.go` sind nie übersetzt und der QUAL-01-Zähler sieht sie nicht** (beobachtet 2026-09-05, Fenster Nr. 3). `rangeReason` (674–682) und die Längen- und Mehrwertmeldungen (710, 778–781) werden durch blosse Zeichenkettenverkettung ohne `i18n.N` gebaut; bei englischer Oberfläche erschien `Ausstattung: höchstens 3 Werte, ausgewählt sind 4.` auf Deutsch. Vorbestehend, gegen `60ff5b2` geprüft — Phase 7 ist der Konvention gefolgt und hat die Fläche verbreitert. `go run ./tools/i18n` kann deshalb `0 offen, 0 verwaist` melden, während eine Person deutsche Ablehnungen liest. Jede Validierungsrückgabe in `field.go` umzuschreiben ist eigene Arbeit

#### Todos

- Phase 6 is done and none of it must be re-discovered. The i18n catalogues already matched the tool's output (quick task `260903-bsk`, `.planning/WINDOWS.md`); all three former defects are closed — `06-03` deleted the indentation claim from the `writeCatalog` doc comment (do **not** go looking for `tools/i18n/main.go:287`; the line and the claim are both gone, and `06-07` retired the ROADMAP note that pointed at them) and made the tool state which regional catalogues it only reads, and `06-06` made CI rebuild and compare all ten artifacts before any test runs
- Phase 6 ordering: rebuild-and-hash-compare in CI first, promote the test skips second. Any catalogue reformat is its own commit, proven with a `jq -S` semantic diff
- Vor dem Ausliefern zu entscheiden: die eine ungefahrene Browserzeile aus Phase 7 (`code` innerhalb eines Blocks auf der öffentlichen Seite — im Test gedeckt, im Browser nicht gesehen) und Fenster Nr. 3 (unübersetzte Ablehnungsgründe). Beide stehen als offen, nicht als bestanden
- Phase 8 must edit `internal/field/store.go:53` first — the missing `AND snippet_id IS NULL` puts every snippet field on every page's edit form, silently
- Phase 10 must gate on the existing `web.ClientIPResolver.IsTrustedPeer` (`internal/web/clientip.go:49–52`), not a second peer check; the middleware goes in at `cmd/holzcloud/main.go:968` between `setupGuard` and `requireAuth`

#### Blockers

- (none)

### Quick Tasks Completed

| Datum | Aufgabe | Ergebnis |
|---|---|---|
| 2026-08-30 | uebergabe-kundenwebsite-abschliessen | Eine Kundenwebsite in zwei Bundles aufgeteilt, Übergabe und Zeiger in CLAUDE.md entfernt |
| 2026-09-06 | phase-7-abschluss-vier-befunde | Feldschlüssel wird auf seine Form geprüft (T-07-02), die Bytegrenze erreicht auch ein verstecktes Feld (W-1), `JoinValues` verteidigt sein Trennzeichen (W-3); Kriterium 1 sichtbar gesenkt statt die Datenstruktur umgebaut |
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
| Phase 06 P05 | 21 min | 3 tasks | 4 files |
| Phase 06 P07 | 18 min | 4 tasks | 13 files |
| Phase 07 P01 | 22 min | 3 tasks | 10 files |
| Phase 07 P02 | 21 min | 3 tasks | 11 files |
| Phase 07 P03 | 17 min | 3 tasks | 12 files |
| Phase 07 P04 | 22 min | 3 tasks | 7 files |
| Phase 07 P05 | 19 min | 4 tasks | 13 files |
| Phase 07 P06 | 9 min | 3 tasks | 4 files |
| Phase 07 P07 | 37 min | 3 tasks | 13 files |

### Session Continuity

To resume: read `.planning/ROADMAP.md` for the v1.6 phase structure (Phases 6–10)
and its *Standing Gates* section. Requirement IDs are in
`.planning/REQUIREMENTS.md`; the working list most of them came from, with the
size and location of each item, is `docs/offene-punkte.md`.

Next command: `/gsd-execute-phase 7` — 07-07 ist der letzte Plan der Phase: er traegt die drei gebündelten Restposten (Übersetzungen, Dokumentensteuer je Art, Browserhälfte) und entscheidet D-08 im Browser — ob ein `bereich`-Feld seine abhängigen Felder wirklich ein- und ausblendet, oder ob `KindRange` neben `KindDate` in die `MayControl()`-Ausnahme gehört. Weiterhin offen und unabhängig davon: `/gsd-verify-work 6` — die Browserhälfte des stehenden Tors gehört in die Abnahme, nicht in einen neuen Plan

**Last session:** 2026-09-05T16:18:01.777Z
**Stopped at:** Completed 07-07-PLAN.md
**Resume file:** None

## Decisions

- [Phase 6]: Phase 6: the i18n pointer tools/i18n/main.go:287 stands as written — D-17a is WITHDRAWN and a fresh grep confirms 287; what the note now says is that the claim lives in the writeCatalog doc comment, not the package doc comment at :1-15
- [Phase 6]: 06-03: writeCatalog is encoding/json end to end (SetEscapeHTML(false) + json.Indent with two empty strings); all seven committed catalogues are byte-unchanged, proven by test and by jq -S against HEAD
- [Phase 6]: 06-03: the round-trip test locks the FORMAT, not the key set — a cleanly deleted key still round-trips. The guard for a deleted key is the -write + git diff pair that 06-06 installs as CI (D-15)
- [Phase 6]: 06-03: a lock test on already-canonical behaviour cannot fail before the implementation exists, so the RED gate was proven by corrupting a real catalogue (reindent, stripped newline) instead of by absence
- [Phase 6]: Phase 6: MAINT-03 and success criterion 3 cover all six committed .wasm modules including internal/plugin/testdata/echo.wasm, and the four .zip archives are repacked by the same tool (D-07, D-23)
- [Phase 6]: 06-05: all four tools/wasm modes work from one in-memory artifact set, so -check compares byte for byte what the write mode would install — the modes cannot drift apart
- [Phase 6]: 06-05: the archives are Deflate with a fixed 1980-01-01 Modified stamp; two runs are byte-identical, and the residual dependency is compress/flate in the toolchain running the tool, not the pinned guest compiler
- [Phase 6]: 06-05: the go.mod floor that 06-02 flagged is now a startup guard (bodenPruefen) — goToolchain below go.mod's go directive refuses with one sentence naming both, instead of letting echo alone fail deep inside a build
- [Phase 6]: 06-05: `go run` collapses any non-zero exit to 1, so the tool's exit 2 for a usage error is only visible to a compiled binary. CI must treat non-zero as the signal, not the specific code
- [Phase 6]: Phase 6: MAINT-05 and success criterion 5 cover all seven codebase maps and point at 06-RESEARCH.md MAINT-05 Correction Inventory instead of naming a count that can itself go stale (D-18, D-19)
- [Phase 6]: Phase 6: the wasm build is tools/wasm, a Go command run as 'go run ./tools/wasm' (D-06); GOTOOLCHAIN is pinned inside the tool with a floor at the root go.mod go directive (D-03, D-03a); -buildvcs=false is mandatory in all four documented invocations (D-02a)
- [Phase 06]: Corrections to the codebase maps are derived by re-running each proof command against current HEAD, never copied from a research document measured at an older commit
- [Phase 06]: CONCERNS.md keeps every judgement; only its numbers moved. The Go-version-mismatch finding stays open because CLAUDE.md:7 still says "Go 1.22+" while go.mod says 1.26.6
- [Phase 06]: A resolved deferred finding is closed by a dated stamp above its text, not by deletion — the reasoning is what a later phase needs
- [Phase 06]: 06-02: D-05 is PASS — the six wasip1 guests hash identically on darwin/arm64 and ubuntu-latest (run 33866318077, 2026-09-04). The byte comparison can be made blocking; the D-05 fallback (-out plus HOLZCLOUD_WASM_DIR) is not needed and must not be planned for
- [Phase 06]: 06-02: tools/wasm forces GOTOOLCHAIN on every build subprocess, proven against ambient local/go1.26.7/go1.27.0 — the guest bytes depend on the goToolchain constant alone, not on setup-go or go.mod. The real trap is D-03a floor: bumping go.mod go directive above the pin breaks the echo build loudly, and only echo
- [Phase 07]: 07-01: D-05 als own-kind bestaetigt — mehrfachauswahl ist eine eigene Art neben auswahl, nicht auswahl mit max_werte>1. Gemessen: render.go Resolve hat keinen case KindChoice, eine Auswahl erreicht ein Theme also als String — Auswahl mehrwertig zu machen wuerde still umtypisieren, was jedes bestehende Theme auf jeder bestehenden Website liest — ohne Fehler an irgendeiner Stelle. Die Entscheidung wurde AUTOMATISCH gewaehlt (workflow.auto_advance: true) und ist einwegig; sie deckt sich mit 07-CONTEXT D-05, sollte aber einmal bestaetigt werden
- [Phase 07]: 07-01: SplitValues/JoinValues sind das eine Paar fuer mehrwertige Werte; SplitChoices/JoinChoices bleiben getrennt und keines ruft das andere auf — Zwei Namen fuer zwei Bedeutungen: eine spaetere Aenderung daran, wie eine Moeglichkeitenliste gelesen wird, darf nicht still aendern, wie ein Wert gespeichert wird. Phase 9 erbt die beiden ausgefuehrten Funktionen
- [Phase 07]: 07-01: Mehrwertigkeit steht im Namen des Formularfeldes (feld_<key>[]), gepraegt allein in Def.FieldName(); der versteckte Waechter haelt geleert von abwesend unterscheidbar — fieldsFromRequest liest bewusst per Praefix, bevor die Definitionen geladen sind — diese Eigenschaft musste erhalten bleiben. Eine Kennung ist slug-artig und kann keine Klammer enthalten, die Markierung kann also nicht kollidieren
- [Phase 07]: 07-02: D-14 als four-columns bestaetigt — darstellung, max_werte, min_wert und max_wert bekommen eigene Spalten auf page_field_defs, keine Pruefregel am Spaltenkopf, ein Down das alle vier zuruecknimmt. AUTOMATISCH gewaehlt (workflow.auto_advance: true) und einwegig: eine freigegebene Wanderung wird nie bearbeitet, eine Korrektur waere 00047. Deckt sich mit 07-CONTEXT D-14 und REQUIREMENTS.md:317, sollte aber einmal bestaetigt werden
- [Phase 07]: 07-02: die beiden Bereichsgrenzen sind Text und keine Zahlen — 'keine Grenze' und 'die Grenze ist null' sind zwei verschiedene Tatsachen, und eine Zahlenspalte mit Vorgabewert koennte sie nicht auseinanderhalten
- [Phase 07]: 07-02: die fuenf SELECT-Spaltenlisten in internal/field/store.go bleiben zeichengleiche Abschriften voneinander und werden als eine Ersetzung geaendert; die vier neuen Spalten stehen zwischen bedingung und COALESCE(block_type_id, 0). Das Tor darauf ist ein Lesetest ueber jeden Leseweg, nicht eine Zaehlung der Vorkommen — ein SELECT kann eine Spalte nennen und sie trotzdem nie in den Def schreiben
- [Phase 07]: 07-02: der Plan widersprach sich selbst — keine Art behaelt Display UND MaxValues, weil validate leert, was zur Art nicht passt. Die Leerregel gewinnt (sie entschaerft T-07-06); der Lesetest nimmt je Ebene ein Auswahl- und ein Mehrfachauswahl-Feld, zusammen decken die beiden alle vier Spalten auf jedem Leseweg
- [Phase 07]: 07-03: die Suchindex-Liste in internal/block/render.go PlainText nimmt KindCode und KindMulti auf und sonst nichts. Ausdruecklich festgehalten, weil weder Fahrplan noch Kontext diese Entscheidung getroffen hatten: ein Codefeld haelt Worte (Adresse, Einstellungszeile) und eine Seite aus Bausteinen waere sonst fuer ihre eigene Suche gerade dort unsichtbar, wo der Verfasser sich am meisten Muehe gab. Eine Mehrfachauswahl kommt mit Leerzeichen verbunden ueber field.SplitValues hinein, nicht als gespeicherte Zeilenspalte. zeit und bereich bleiben draussen, aus demselben Grund wie Bildnummer und Datum; schlagwort kann hier nie auftauchen
- [Phase 07]: 07-03: field.ParseNumber und field.ParseTimeOfDay sind die je eine Lesart. Check misst den eingegebenen Wert UND beide Grenzen damit, und validate in store.go wurde auf ParseNumber umgestellt: vorher las validate mit blankem strconv.ParseFloat, waehrend Check das Komma vorher ersetzt — "0,5"/"0,2" war damit ein verdrehtes Paar, das validate nicht sehen konnte und Check dann rueckwaerts durchgesetzt haette. Eine Uhrzeit wird ueber die Zeichenlaenge gelesen und nicht ueber time.Parse, dessen Stundenfeld auch einstellig annimmt: "9:30" kaeme sonst still als halb zehn durch
- [Phase 07]: 07-03: D-08 wurde hier bewusst NICHT entschieden — MayControl bleibt true fuer bereich, und der Platzhalterzweig im Zahlenfeld haelt die Praemisse ueberhaupt pruefbar. Der Browserdurchgang in 07-07 entscheidet, ob :placeholder-shown an einem Zahlenfeld greift, und traegt den geschriebenen Rueckweg. zeit dagegen ist raus, aus demselben Grund wie datum; MayControl wurde dafuer in zwei switch-Faelle geteilt, damit die gemeinsame Begruendung neben dem Paar steht, das sie erklaert
- [Phase 07]: 07-03: die dritte Leerregel in validate (beide Grenzen fuer jede Art ausser bereich) hat drei bestehende Testreihen zerbrochen — store_test, admin/field_defs_test und bundle_test hingen die Grenzen mangels Konstante an Auswahl, Mehrfachauswahl und Textfeld. Die Grenzen zogen in allen dreien an ein Bereichsfeld um; der Lesetest gewann dabei Deckung, weil jede Spalte jetzt an der Art sitzt, der sie gehoert. Am schlimmsten war die T-07-05-Pruefung auf gebundene Parameter: an einem Textfeld waere der boshafte Wert geleert worden und die Pruefung haette weiter gruen gemeldet, ohne noch irgendetwas zu beweisen
- [Phase 07]: 07-04: MaxValueBytes wird gemeldet und nicht mehr angewendet (D-13 erledigt). trimTo kuerzt nicht mehr, ein Bytewaechter steht vor dem Verteiler in Check und gilt damit jeder Art; gemessen wird der ganze verbundene Wert eines Feldes einschliesslich der Umbrueche, in Byte und nicht in Runen — Ein gekuerzter Wert sieht aus wie einer, den jemand so getippt hat. Seit das Budget allen Werten eines Feldes zusammen gehoert, haette das Kuerzen einen Wert halbiert und das Bruchstueck als echten Wert abgelegt. Die Datenbank zaehlt Byte, also zaehlt die Grenze Byte; die Begruendung sagt dazu, dass Umlaute doppelt zaehlen, statt 4000 Zeichen zu versprechen, die sie nicht halten kann. Eintrag 2 in .planning/WINDOWS.md ist fixed, offen sind null
- [Phase 07]: 07-04: max_werte gilt serverseitig in Check, gezaehlt ueber SplitValues; null heisst ohne Grenze — Eine Haekchengruppe laesst sich in HTML ohne JavaScript nicht begrenzen, und dieses Programm traegt keines ausser htmx (D-05). Null ist das, was jedes vor dieser Phase angelegte Feld traegt — eine erzwungene Null waere eine Verhaltensaenderung an jedem bestehenden Feld
- [Phase 07]: 07-04: parseRowName bekam eine vierte Wache — ein Unterfeldname, der nach dem Abschneiden der Markierung leer waere, wird abgelehnt — Der Plan nannte drei zu erhaltende Wachen; keine davon lehnt gruppe.<kennung>.<nummer>. oder gruppe.<kennung>.<nummer>.[] ab. Beide legten bisher einen Zeileneintrag unter dem leeren Schluessel an; cleanRow warf ihn spaeter weg, gespeichert wurde also nie etwas. Der Weg oben auf der Seite uebergeht feld_[] seit 07-01 — die beiden Namenswege stimmen jetzt ueberein, und die Ablehnungstabelle weist es nach
- [Phase 07]: 07-05: Check prueft ein Schlagwortkuerzel mit page.Slugify(value) != value und NICHT mit page.ValidateSlug — ValidateSlug traegt reservedSlugs, eine Reservierung der Router-Pfade; ein Schlagwort namens "admin" ist unter /tag/admin voellig erreichbar und waere abgelehnt worden. Slugify(v)==v ist genau die hier gestellte Frage: ist das schon die normalisierte Schreibweise. Dafuer importiert internal/field erstmals internal/page — zyklenfrei geprueft, weil internal/page nur auth und db zieht und keines von beiden field kennt
- [Phase 07]: 07-05: Der Wert eines Schlagwortfeldes reist im Archiv als NAME und nicht als Kuerzel, internal/bundle/format.go blieb unangetastet. Rename behaelt das Kuerzel absichtlich, damit Links nicht brechen — ein als Kuerzel reisender Wert haette auf der Zielmaschine still ins Leere gezeigt. format.go:152-159 und :274-284 sind als Entscheidung gelesen worden und nicht als Beschreibung: an der Schreibweise zu drehen wuerde die Bedeutung jedes bereits ausgehaendigten Archivs aendern. Folge und kein Fehler: die Adresse einer Beschriftung darf sich ueber eine Rundreise bewegen (moebel kommt als moebelbau an), das gemeinte Schlagwort nicht — der Test sagt das im Kommentar, damit es spaeter niemand als Fehler meldet
- [Phase 07]: 07-05: term.EnsureNames und importTerms legen jedes Schlagwort an, das ein Manifest nennt; report.Terms zaehlt Angelegtes statt Behauptetes, an genau einer Stelle gesetzt. Bisher entstand ein Schlagwort nur als Nebenwirkung einer Seite, die es traegt — eines, das nur ein Schlagwortfeld nennt, kam nirgends an. EnsureNames leitet das Kuerzel mit derselben page.Slugify ab wie SetForPage; liefen die beiden auseinander, waere ein Schlagwort zwei Zeilen
- [Phase 07]: 07-05: Der offene Blocker aus 07-04 ist ENTSCHIEDEN und behoben statt vertagt — importFieldValues laeuft jetzt durch field.Clean und field.CheckAll (Rule 2, fehlende Eingabepruefung an einer Vertrauensgrenze). Ein Archiv ist genauso unvertraut wie ein Formularfeld, alle anderen Schreibwege waren gedeckt, und seit 07-04 kuerzt trimTo nichts mehr — CheckAll ist damit die einzige verbliebene Stelle, an der das Bytebudget gilt. Die Definitionen werden ueber s.Fields.List GELESEN und nicht aus dem Manifest nachgebaut, damit gegen das geprueft wird, was diese Website hat. Ein beanstandeter Wert wird benannt und entfernt, nie die ganze Seite verworfen; gemeldet nur im ersten Durchgang, nicht noch einmal im Verweisdurchgang
- [Phase 07]: 07-05: Der beweisende Rundreisetest benennt das Schlagwort VOR dem Export um, und das ist nicht optional — im RED-Lauf bestand der Untertest ohne Umbenennung gegen den unreparierten Baum, waehrend der mit Umbenennung in drei Behauptungen fiel. Ein Schlagwort, dessen Name noch zu seinem Kuerzel passt, reist auch ohne die Uebersetzung heil und bewiese gar nichts
- [Phase 07]: 07-05: KEIN Kontrollpunkt in diesem Plan — alle vier Tasks type=auto, kein tracer, kein precondition. Es wurde nichts automatisch genehmigt und nichts automatisch gewaehlt; anders als 07-01 und 07-02 gibt es hier keine einwegige Auswahl nachzubestaetigen
- [Phase 07]: v1.6 Phase 7: switchOf nimmt die ganze field.Def statt einer blossen Art — „eine Auswahl als Knopfreihe" ist mit einer Zeichenkette nicht auszudruecken, weil die Darstellung Teil der Antwort ist. Eine einzige Aufrufstelle, viewOf — Ein zweiter Parameter oder eine blosse Darstellungszeichenkette waeren dieselbe Entscheidung an zwei Stellen geschrieben
- [Phase 07]: v1.6 Phase 7: .feld-schalter--knopfreihe kopiert die Form der auswahl-Regel (Vorgabe sichtbar, verbergen bei Treffer) und nicht die des kreuz — eine beantwortete Auswahl zeigt ihre Abhaengigen, eine offene verbirgt sie. Selektor: input[type="radio"][value=""]:checked
- [Phase 07]: v1.6 Phase 7: eine Mehrfachauswahl und ein Schlagwortfeld meldeten vor 07-06 den Schalter „text", dessen Regel einen Platzhalter sucht, den beide nicht tragen — jedes Feld daran blieb stumm sichtbar. Beide fielen in switchOfs default-Zweig; 07-06 hat sie auf „kreuz" und „auswahl" gestellt
- [Phase 07]: v1.6 Phase 7: MayControl() blieb in 07-06 unangetastet — KindTime ausgeschlossen, KindRange drin. D-08 gehoert dem Browserdurchgang in 07-07 und darf nicht aus dem Markup entschieden werden
- [Phase 07]: v1.6: D-08 ist durch Beobachtung geschlossen und die Umkehr NICHT angewendet — `MayControl()` bleibt unverändert, `KindRange` bleibt steuernd. Ein `<input type="number">` trifft `:placeholder-shown`, `.feld-schalter--text` greift daran, das abhängige Feld erschien und verschwand (Playwright, 2026-09-05) — Die Fahrplan-Notiz, `KindRange` neben `KindDate` auszuschliessen, ruhte auf der Schieber-Annahme, die D-07 verworfen hat. Auch der zweite Zweig des Auftrags ist beantwortet: `placeholder=" "` steht im gezeichneten Markup, es fehlte also nicht. Drei Kommentare, die auf den Durchgang vorauswiesen, sagen jetzt, was gesehen wurde.
- [Phase 07]: v1.6: eine `<precondition>`, die dem Executor fehlt, wird gehalten und die Arbeit geteilt — nicht in eine Checkliste zurückverwandelt. Schritt 3 von 07-07 verlangte ein Browserwerkzeug; der Executor hielt, der Orchestrator fuhr den Durchgang mit dem Playwright-MCP-Server — Eine Checkliste zurückzugeben ist genau das, was der Auftrag verbietet, und was die Browserhälfte in Phase 6 halb hat liegen lassen. Das Muster gilt für Phase 8, 9, 10 und 11 weiter.
- [Phase 07]: v1.6: der leere Fall einer neuen Feldart liegt in `MinimalData`s `Felder`-Karte, nicht in `Feldliste` — `field.List` lässt jeden leeren Eintrag aus, eine Liste mit leerem Eintrag gibt es also gar nicht — Ihn in `Feldliste` zu verlangen hiesse, eine Form zu fordern, die der Code nie erzeugt. Gilt für jede Art, die Phase 8 und 9 hinzufügen.

## Accumulated Context

### Roadmap Evolution

- Phase 11 added: Galerie — Vergroessern/Lightbox (:target, kein JS), Album als wiederverwendbares Ding, Diashow via CSS scroll-snap. Haengt an Phase 7 (Mehrwert-Kodierung FIELD-07), nicht an Phase 10. Passt nicht ins v1.6-Meilensteinziel; per Entwicklerentscheid trotzdem hier.

### Blockers

- ~~07-04 gemeldet, nicht behoben: internal/bundle/import.go importFieldValues schreibt Feldwerte mit field.Encode direkt, ohne CheckAll und ohne Clean.~~ **ERLEDIGT in 07-05 (7cb09f4).** Entschieden wurde gedeckt und nicht vertagt: ein Archiv ist eine Datei, die jeder bearbeiten kann, alle anderen Schreibwege sind gedeckt, und seit 07-04 kuerzt trimTo nichts mehr — CheckAll ist damit die einzige Stelle, an der das Bytebudget ueberhaupt noch gilt. importPages liest die tatsaechlich angelegten Definitionen ueber s.Fields.List und reicht sie in importFieldValues; dort laufen field.Clean und field.CheckAll, ein beanstandeter Wert wird entfernt und namentlich in den Bericht geschrieben, nie die ganze Seite verworfen. TestArchivwerteGehenDurchDieselbePruefung beweist es, Gegenprobe mit deaktivierter Wache gefuehrt (vier Behauptungen fallen). Offene Blocker: keine
