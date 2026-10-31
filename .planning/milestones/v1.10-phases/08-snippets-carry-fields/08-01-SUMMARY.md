---
phase: 08-snippets-carry-fields
plan: 01
subsystem: database
tags: [sqlite, goose, go, html-template, field-definitions, snippets]

# Dependency graph
requires:
  - phase: 07-field-kinds
    provides: "die vier neuen Spalten an page_field_defs, field.Entry samt Vertragswächtern, TestNeueSpalten als Form für ein Tor, das zurückliest statt zu zählen"
  - phase: 03-content
    provides: "die snippets-Tabelle, LoadRendered und .Site.Snippets als veröffentlichter Vertrag"
provides:
  - "Wanderung 00047: page_field_defs.snippet_id mit Fremdschlüssel, der Indextausch, der vierte Teilindex, snippets.fields"
  - "field.Def.SnippetID und der Namensraum-Diskriminator in allen sechs handgeschriebenen Anweisungen"
  - "(*field.Store).OfSnippet — Spaltenliste von OfBlockType, Baumbau von List"
  - "snippet.Snippet.Fields, (*snippet.Store).SetFields, snippet.Rendered.Fields und .IDs"
  - "template.SiteData.Bausteinfelder und .Bausteinliste als Theme-Vertrag"
  - "(*public.Handler).fillSnippets als einzige Prägestelle der drei Textbaustein-Mitglieder"
affects: [08-02, 08-03, 08-04, 08-05, 09-csv-import]

actuals:
  tokens: 12395
  tasks: 3
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Der vierte Namensraum an page_field_defs, nach Seite, Gruppe und Bausteinart"
    - "Jede Leseabfrage nennt jeden fremden Namensraum ausdrücklich (D-09); kein Zweig bedeutet „was übrig bleibt“"
    - "Eine Prägestelle für ein Vertragsmitglied, das an vierzehn Routen gesetzt wird"

key-files:
  created:
    - internal/db/migrations/00047_snippet_fields.sql
    - internal/db/migrations_down_test.go
    - internal/public/bausteinfelder_test.go
  modified:
    - internal/field/field.go
    - internal/field/store.go
    - internal/field/store_test.go
    - internal/snippet/store.go
    - internal/template/loader.go
    - internal/template/sample.go
    - internal/tmplspec/TEMPLATE-SPEC.md
    - internal/public/pagedata.go
    - internal/public/handler.go

key-decisions:
  - "Tor 1 und Tor 2 als `beide-wie-vorgeschlagen` beantwortet: Wanderung 00047 in der Form von 00038, mit REFERENCES snippets(id) ON DELETE CASCADE und ohne Vorgabewert; .Site.Bausteinfelder / .Site.Bausteinliste als Namenspaar neben .Site.Snippets, das seinen Typ behält"
  - "OfSnippet ist ein sechster Leser und trägt darum eine sechste Spaltenliste — die Zählgatter des Plans (6/5) waren gegen fünf Leser gerechnet und messen nun 7/6, symmetrisch zum Bausteinart-Vorbild, das in derselben Datei ebenfalls 7 misst"
  - "TestMigration00047RunterUndRauf liegt in package db statt in db_test.go, weil migrationProvider unexportiert ist und RunMigrations nur den Weg nach oben kennt"
  - "SampleData und die zwei .Site-Zeilen der TEMPLATE-SPEC.md wurden vorgezogen: die vier Reflexionswächter aus Phase 7 brechen die Übersetzung in dem Moment, in dem SiteData ein Mitglied bekommt"

patterns-established:
  - "Vierter Träger in einer Tabelle: Spalte ohne Vorgabewert, Teilindex je Namensraum, Diskriminator in jeder Leseabfrage, Träger beim Update aus dem Gespeicherten gepinnt"
  - "Das Tor auf einen Namensraum ist ein Test, der über jeden Leseweg zurückliest — nicht die Zahl der Vorkommen einer Zeichenkette"

requirements-completed: [SNIP-02, SNIP-03, SNIP-04, SNIP-05]

coverage:
  - id: D1
    description: "page_field_defs trägt einen vierten Namensraum; die drei davor sind unverändert"
    requirement: "SNIP-02"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraum"
        status: pass
      - kind: integration
        ref: "internal/field/store_test.go#TestNeueSpalten"
        status: pass
    human_judgment: false
  - id: D2
    description: "Dieselbe Kennung steht einmal an der Seite und einmal am Textbaustein derselben Website, ohne zu kollidieren; ein zweites Mal am selben Textbaustein wird mit ErrDuplicateKey abgewiesen"
    requirement: "SNIP-02"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraum"
        status: pass
    human_judgment: false
  - id: D3
    description: "field.Store.List gibt nie ein Feld eines Textbausteins heraus — der gefährliche Schnitt dieser Phase"
    requirement: "SNIP-04"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraum"
        status: pass
      - kind: e2e
        ref: "internal/public/bausteinfelder_test.go#TestBausteinfelderErreichenDasTheme"
        status: pass
    human_judgment: false
  - id: D4
    description: "Ein Feldwert eines Textbausteins erreicht ein Theme über .Site.Bausteinfelder auf einer echten öffentlichen Adresse, während der Markdown-Rumpf weiterhin erscheint"
    requirement: "SNIP-03"
    verification:
      - kind: e2e
        ref: "internal/public/bausteinfelder_test.go#TestBausteinfelderErreichenDasTheme"
        status: pass
    human_judgment: false
  - id: D5
    description: ".Site.Snippets behält map[string]template.HTML; ein Textbaustein ohne Definitionen bekommt eine leere und keine nil-Karte"
    requirement: "SNIP-05"
    verification:
      - kind: integration
        ref: "internal/public/bausteinfelder_test.go#TestSnippetsBleibtTemplateHTML"
        status: pass
    human_judgment: false
  - id: D6
    description: "Die Rückwärtshälfte von 00047 nimmt genau so viel zurück, wie 00047 gegangen ist — Seiten-, Gruppen- und Bausteinartfelder bleiben stehen, und der wiederhergestellte Index trägt 00038s Form"
    requirement: "SNIP-02"
    verification:
      - kind: integration
        ref: "internal/db/migrations_down_test.go#TestMigration00047RunterUndRauf"
        status: pass
    human_judgment: false
  - id: D7
    description: "Der vierte Namensraum ist für Seiten und Bausteinarten unsichtbar: die bestehende Prüffolge ist grün, ohne dass ein Textbaustein in einer Vorrichtung steht"
    requirement: "SNIP-04"
    verification:
      - kind: integration
        ref: "go test ./..."
        status: pass
    human_judgment: false
  - id: D8
    description: "Der Bearbeitungsbildschirm eines Textbausteins und die Felddefinitionsmaske in ihrem vierten Modus"
    requirement: "SNIP-01"
    verification: []
    human_judgment: true
    rationale: "Nicht Teil dieses Plans — Plan 08-03 und 08-04 bauen die Bildschirme, und ihre Beurteilung im Browser gehört dorthin. Hier steht nur der Weg unter ihnen."

duration: 10 min
completed: 2026-09-06
status: complete
---

# Phase 8 Plan 01: Der vierte Namensraum Summary

**Ein Feld eines Textbausteins reist von der Wanderung `00047` über `field.Def.SnippetID`, `OfSnippet` und `snippets.fields` bis in `.Site.Bausteinfelder` und wird von einer Vorlage auf einer echten öffentlichen Adresse gedruckt — während `field.Store.List` es nie zu sehen bekommt.**

## Performance

- **Duration:** 10 min
- **Started:** 2026-09-06T09:21:58Z
- **Completed:** 2026-09-06T09:32:46Z
- **Tasks:** 3 (ein Entscheidungstor, zwei ausführende)
- **Files modified:** 12 (3 neu, 9 geändert)

## Accomplishments

- **Wanderung `00047`** legt `page_field_defs.snippet_id` mit `REFERENCES snippets(id) ON DELETE CASCADE` und ohne Vorgabewert an, tauscht `idx_page_field_defs_kennung_oben` gegen die Fassung mit `snippet_id IS NULL`, legt `idx_page_field_defs_kennung_textbaustein` über `(snippet_id, kennung)` an und gibt `snippets` die Spalte `fields`. Die Rückwärtshälfte stellt **`00038`s** Indexform wieder her, nicht `00029`s — und ein Test fährt sie wirklich.
- **Der gefährliche Schnitt ist geschnitten.** `internal/field/store.go` nennt in `List`, `OfBlockType` und `OfBlockTypes` jetzt `AND snippet_id IS NULL`; `Sub` bleibt bewusst ohne, weil eine Gruppe an einem Textbaustein stehen darf und ihre Unterfelder dann beides tragen. `Update` behält seine trägerlose `WHERE` und pinnt stattdessen `d.SnippetID = existing.SnippetID`.
- **`OfSnippet`** ist der eine neue Leser: Spaltenliste und Fehlerhülle von `OfBlockType`, Baumbau von `List` — denn ein Textbaustein hat ein eigenes Formular und trägt darum auch Gruppen, was eine Bausteinart nicht kann.
- **Der Theme-Vertrag** wächst um `.Site.Bausteinfelder map[string]map[string]any` und `.Site.Bausteinliste map[string][]field.Entry`, angehängt hinter `Snippets`, das seinen Typ behält. SNIP-05 gilt damit durch Bauart und nicht durch eine Prüfung.
- **`fillSnippets`** ist die einzige Stelle im Baum, an der eines der drei Textbaustein-Mitglieder von `SiteData` geschrieben wird; die zwei Seitenrouten gehen jetzt hindurch, die übrigen zwölf bleiben für Plan 08-02 stehen.
- **Das Tor ist eine Lesung, keine Zählung.** `TestBausteinNamensraum` legt ein Seitenfeld und ein Textbausteinfeld mit **derselben Kennung** an und liest über `List`, `OfSnippet`, `Sub`, `OfBlockType`, `OfBlockTypes` und `Get` zurück. `bausteinfelder_test.go` druckt den Wert durch eine echte öffentliche Adresse und prüft im selben Atemzug, dass `.Page.Feldliste` leer bleibt.

## Task Commits

1. **Task 1: Die zwei Türen ohne Rückweg** — kein Commit; ein `checkpoint:decision`, im Auto-Modus mit der empfohlenen Option beantwortet (siehe unten)
2. **Task 2: Wanderung 00047** — `9fd3e2d` (feat)
3. **Task 3: Ein Feld von der Tabelle bis ins Theme** — `93f1194` (feat, tracer)

**Plan metadata:** siehe letzten Commit dieses Plans (docs)

## Das Entscheidungstor (Task 1)

**Gewählt: `beide-wie-vorgeschlagen`** — die Empfehlung, im Auto-Modus automatisch bestätigt.

`.planning/config.json` trägt `mode: yolo` und `workflow.auto_advance: true`, und die Aufgabe trägt kein `gate="blocking-human"`. Nach Regel 5 des Checkpoint-Protokolls wählt ein `checkpoint:decision` dann die erste Option — und die Planer stellen die empfohlene voran. Der Entwickler hat den Meilenstein delegiert und angeordnet, dass die Tore selbsttätig laufen.

Warum die Wahl auch inhaltlich die richtige ist, gegen den Baum geprüft und nicht aus Vorliebe:

- **Tür 1, die Wanderung.** `00038:42` trägt `REFERENCES block_types(id) ON DELETE CASCADE`, und sein eigener Kommentar bei `:36-39` nennt die tatsächliche Regel: SQLite verweigert `ADD COLUMN` mit `REFERENCES` **zusammen mit** einem Vorgabewert ungleich NULL, nicht `REFERENCES` an sich. `snippet_id` bekommt keinen Vorgabewert, also ist die Klausel zu haben — und `migration-ohne-fremdschluessel` hätte eine Zusicherung ohne Not an Go abgegeben.
- **Tür 2, der Name.** `loader.go:355-357` verbietet Umbenennen und Entfernen in so vielen Worten: ein hochgeladenes Theme ist gegen diese Struktur geschrieben, und ein fehlendes Mitglied ist ein Parse-Fehler auf der Anfrage eines Besuchers. `snippets-typ-aendern` hätte jedes installierte Theme genau dort gebrochen und SNIP-05 aus einer Eigenschaft, die durch Bauart gilt, in eine gemacht, die man nicht prüfen kann.

## Files Created/Modified

- `internal/db/migrations/00047_snippet_fields.sql` *(neu)* — die Spalte, der Indextausch, der vierte Teilindex, `snippets.fields`; Rückwärtshälfte auf `00038`s Stand
- `internal/db/migrations_down_test.go` *(neu)* — `TestMigration00047RunterUndRauf`, fährt `Down` und liest die Index-SQL aus `sqlite_master` zurück
- `internal/public/bausteinfelder_test.go` *(neu)* — der Weg von Ende zu Ende plus die SNIP-05-Gegenprobe
- `internal/field/field.go` — `Def.SnippetID` neben `Def.BlockTypeID`
- `internal/field/store.go` — sechs Spaltenlisten, die Positions-Unterabfrage, drei `IS NULL`-Klauseln, `scanDef`, `Update`s Pin, `OfSnippet`
- `internal/field/store_test.go` — `TestBausteinNamensraum`
- `internal/snippet/store.go` — `Fields`, `columns`, `scan`, `SetFields`, `Rendered.Fields`, `Rendered.IDs`, `LoadRendered` in **einer** Abfrage
- `internal/template/loader.go` — `SiteData.Bausteinfelder`, `SiteData.Bausteinliste`
- `internal/template/sample.go` — `SampleData` trägt beide (Abweichung 2)
- `internal/tmplspec/TEMPLATE-SPEC.md` — zwei `.Site`-Zeilen (Abweichung 2)
- `internal/public/pagedata.go` — `fillSnippets`, `leereBausteine`
- `internal/public/handler.go` — zwei Routen gehen durch `fillSnippets`

## Decisions Made

Über das Entscheidungstor hinaus:

- **`OfSnippet` baut den Baum, `OfBlockType` nicht.** `BlockKinds()` lässt an einer Bausteinart keine Gruppe zu (`field.go:167-176`), ein Textbaustein hat aber ein eigenes Formular und trägt darum alles, was ein Seitenformular trägt. Im Doc-Kommentar steht, welche Hälfte von welchem der beiden Vorbilder stammt.
- **`Sub` bleibt ohne Diskriminator, und der Grund steht als Kommentar darüber**, weil der Mustervergleich hier genau der falsche Zug wäre: eine Gruppe an einem Textbaustein käme sonst leer zurück. Ihr Namensraum ist die Gruppennummer, und die ist innerhalb der Website eindeutig — im Sinne von D-09 also bereits ausdrücklich genannt.
- **Ein Textbaustein ohne eine einzige Definition bekommt trotzdem seinen Eintrag** in `Bausteinfelder`: eine leere Karte, kein fehlender Schlüssel. Das ist die Zusage aus den `must_haves` und der Fall, an dem ein Theme sonst auf der Anfrage eines Besuchers scheitert.
- **Nicht gebaut, und das mit Absicht:** `OfSnippets`, der vierte Arm von `Move`, der Textbaustein-Arm in `validate` und die trägerweise `MaxFields`-Zählung. Alle vier gehören zu Plan 08-02; sie herauszuhalten ist es, was diese Aufgabe auf einem Pfad hält.

## Deviations from Plan

### Auto-fixed Issues

**1. [Regel 3 — Blockierend] `TestMigration00047RunterUndRauf` liegt in `package db`, nicht in `db_test.go`**

- **Found during:** Task 2
- **Issue:** Der Plan verlangt den Test „in `internal/db/db_test.go`, in the package, using `migrationProvider` directly“. Diese Datei ist aber `package db_test`, und `migrationProvider` wie `migrationsFS` sind unexportiert — der Test lässt sich dort nicht schreiben, ohne die API zu verbreitern.
- **Fix:** Neue Datei `internal/db/migrations_down_test.go` mit `package db`. Das ist die wörtliche Lesart von „in the package“, führt `migrationProvider` unmittelbar und exportiert nichts Neues. Ein exportierter Test-Helfer in `db.go` wäre die Alternative gewesen und hätte die öffentliche Fläche für einen Test vergrössert.
- **Files modified:** `internal/db/migrations_down_test.go`
- **Verification:** `go test ./internal/db/ -run 'TestRunMigrations|TestMigration00047' -v` → drei `--- PASS`
- **Committed in:** `9fd3e2d`

**2. [Regel 3 — Blockierend] `SampleData` und zwei Zeilen der `TEMPLATE-SPEC.md` vorgezogen aus Plan 08-05**

- **Found during:** Task 3, Schritt 5
- **Issue:** In dem Moment, in dem `SiteData` die zwei Mitglieder bekommt, fallen `TestSampleDataFillsEveryField` und `TestSpecDocumentsEveryFieldOfTheContract`. Das ist kein Unfall, sondern der Zweck der vier Reflexionswächter aus Phase 7 — `08-CONTEXT.md` sagt es voraus: „TEMPLATE-SPEC.md + SampleData + MinimalData → or the suite fails.“ Das Gatter von Task 3 verlangt aber `go test ./...` grün.
- **Fix:** `SampleData` trägt `Bausteinfelder` und `Bausteinliste` für den bereits benutzten Schlüssel `footer-kontakt`, beide so gefüllt, dass sie übereinstimmen. `TEMPLATE-SPEC.md` bekommt die zwei `.Site`-Zeilen mit der zweistufigen Indizierung.
- **Bewusst nicht mitgenommen:** `MinimalData`s leerer Zwilling und die §7-Prosa. Kein Test verlangt sie, die Artefaktliste der Phase weist beide Plan 08-05 zu, und der leere Zwilling zieht die Umschreibung des `MinimalData`-Kommentars („no snippets“) nach sich. Als Lücke für 08-05 notiert, nicht als erledigt geführt.
- **Files modified:** `internal/template/sample.go`, `internal/tmplspec/TEMPLATE-SPEC.md`
- **Verification:** `go test ./...` grün
- **Committed in:** `93f1194`

### Zwei Zählgatter messen 7 und 6 statt 6 und 5 — und das ist die richtige Zahl

Kein Fehler in der Ausführung, sondern eine Rechnung im Plan, die einen Leser zu wenig zählt. Weil der Plan diese Gatter ausdrücklich als einmal korrigiert und besonders heikel führt, hier die Herleitung im Ganzen:

| Gatter | Plan | Gemessen | Warum |
|---|---|---|---|
| `COALESCE(snippet_id, 0)` gesamt | 6 | **7** | sechs SELECT-Spaltenlisten + eine Unterabfrage |
| `COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)` | 5 | **6** | eine je Spaltenliste |
| `COALESCE(snippet_id, 0) = COALESCE` | 1 | **1** | ✅ unverändert |
| `snippet_id IS NULL` | ≥ 3 | **3** | ✅ `List`, `OfBlockType`, `OfBlockTypes` |

Die Rechnung des Plans („`:57`, `:94`, `:118`, `:145`, `:193` sind die Listen, `:257` ist die Unterabfrage“) ist gegen **fünf** Leser gestellt. Dieselbe Aufgabe verlangt in Schritt 3 aber `OfSnippet` — einen **sechsten** Leser, der dieselbe Spaltenliste tragen muss, weil `scanDef` sie positionsweise liest und die Listen laut der deutschen Warnung bei `scanDef` Abschriften voneinander bleiben müssen.

Die Gegenprobe steht in derselben Datei und ist entscheidend: `COALESCE(block_type_id, 0)` misst nach diesem Plan ebenfalls **7**. Das Bausteinart-Vorbild und der neue Namensraum sind Zeichen für Zeichen symmetrisch — genau das, was die Gatter sichern sollen. Wäre die Zahl auf 6 gedrückt worden, hätte das eine von zwei Formen gehabt: `OfSnippet` ohne `COALESCE` um `snippet_id` (der Bruch der Abschriftenregel, vor dem `scanDef` warnt) oder das Herausziehen der Spaltenliste in eine gemeinsame Konstante (ein Umbau aller fünf bestehenden Anweisungen, der jedes Zählgatter dieses Plans wertlos macht).

Die Warnung des Plans — „eine zu kurze Zählung bedeutet immer eine fehlende Spaltenliste, nie eine überflüssige Unterabfrage“ — ist eingehalten: die Zählung ist **zu lang**, nicht zu kurz, und die Unterabfrage steht unangetastet bei genau 1.

---

**Total deviations:** 2 auto-fixed (2 × Regel 3 blockierend), dazu eine dokumentierte Korrektur zweier Gatterzahlen.
**Impact on plan:** Kein Zuwachs am Umfang. Abweichung 1 ist eine Dateiwahl, die die Go-Sichtbarkeit erzwingt. Abweichung 2 ist das Minimum, das die Übersetzung wieder grün macht, und lässt die Hälfte, die kein Test verlangt, ausdrücklich bei Plan 08-05. Die Gatterzahlen sind an ihrem eigenen Vorbild in derselben Datei nachgewiesen.

## Issues Encountered

- **`field.Values` ist `map[string]string`, nicht `map[string][]string`.** Beim Schreiben der Vorrichtung angenommen, vom Übersetzer sofort gemeldet, korrigiert.
- **`HandlePage` liest die Adresse aus `r.PathValue("slug")`.** Die erste Fassung des Endzu-Ende-Tests lief in ein 404; der Baum zeigt bei `handler_test.go:411` das eingeführte `r.SetPathValue`, und der Test benutzt jetzt dasselbe.
- **Deutsche Anführungszeichen in Go-Zeichenketten.** `„telefon"` schliesst mit einem geraden Anführungszeichen und beendet damit das Literal. Auf `„telefon“` gestellt.

## Known Stubs

Keine. Jede Stelle, die dieser Plan anfasst, trägt echte Daten; die zwölf noch nicht umgestellten Zuweisungsstellen in `internal/public` sind unverändert und nicht halb umgebaut — sie setzen `site.Snippets` weiterhin selbst, genau wie vor diesem Plan, und Plan 08-02 stellt sie zusammen mit dem Gatter um, das ihre Übereinstimmung nachweist.

## Threat Flags

Keine neue sicherheitsrelevante Fläche über den `<threat_model>` des Plans hinaus. Zu T-08-05 ausdrücklich: auf dem Weg eines Textbausteinfeldes steht **kein** neuer `template.HTML`-Guss. Die Werte gehen durch `field.Resolve` und `field.List`, dieselben zwei Aufrufe wie bei `.Page.Felder`, und `html/template` maskiert sie kontextabhängig. Der einzige bestehende Guss, `Rendered.HTML`, ist unangetastet und weiterhin durch die Kette goldmark → bluemonday gedeckt, die vor dem Speichern gelaufen ist.

## User Setup Required

Keine — kein externer Dienst berührt.

## Next Phase Readiness

Bereit für **Plan 08-02**. Die Grundlage steht und ist von Ende zu Ende bewiesen; 08-02 hat vier klar umrissene Aufgaben, die dieser Plan bewusst liegen gelassen hat:

1. `OfSnippets` — der Massenleser, eine Abfrage je Website statt einer je Textbaustein
2. `fillSnippets` an den **übrigen zwölf** Zuweisungsstellen (`access.go:73,179`, `archive.go:49`, `search.go:35`, `tag.go:66`, `typearchive.go:65`, `feed.go:76`, `cart.go:74`, `checkout.go:267,332`, `pluginhost.go:180`, `shop.go:159,218`) — samt dem Gatter, das ihre Übereinstimmung nachweist
3. `Move`s vierter Arm und der Textbaustein-Arm in `validate` (wobei `Required` an einem Textbaustein bedeutungsvoll bleibt — die Verengung der Bausteinart bei `:530-536` ist hier ein Gegenbeispiel und kein Vorbild)
4. die trägerweise `MaxFields`-Zählung nach D-05

Offen und ausdrücklich Plan 08-05 zugewiesen: `MinimalData`s leerer Zwilling für `Bausteinfelder` samt der Umschreibung des Satzes „no menus, no labels, no snippets“, und die §7-Prosa der `TEMPLATE-SPEC.md`. Bis dahin druckt die Hochladeprüfung den gefüllten Fall, nicht den leeren.

Keine Blockierer. `.planning/phases/11-galerie/` ist unberührt.

---
*Phase: 08-snippets-carry-fields*
*Completed: 2026-09-06*

## Self-Check: PASSED

Alle drei neuen Dateien liegen auf der Platte, beide Commits stehen im Verlauf,
und jedes im Vertrag zugesagte Symbol ist im Baum: `OfSnippet`,
`snippet.Store.SetFields`, `fillSnippets`, `SiteData.Bausteinfelder` — und
`SiteData.Snippets` trägt unverändert `map[string]template.HTML`.
