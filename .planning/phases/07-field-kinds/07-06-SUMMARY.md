---
phase: 07-field-kinds
plan: 06
subsystem: ui
tags: [go, html-template, css, has-selector, accessibility, field-kinds, conditional-fields]

# Dependency graph
requires:
  - phase: 07-field-kinds
    provides: "07-01's FieldView.Grouped() und die geteilte Beschriftung mit id + aria-labelledby — hier erweitert statt kopiert"
  - phase: 07-field-kinds
    provides: "07-02's darstellung-Spalte, Def.Display und Def.IsButtonRow() — das Praedikat, an dem switchOf und die Vorlage sich teilen"
  - phase: 07-field-kinds
    provides: "07-03's KindTime/KindRange/KindCode, der Platzhalter placeholder=\" \" an beiden Bedienelementen und MayControl in dem Zustand, den dieser Plan nicht anfasst"
  - phase: 07-field-kinds
    provides: "07-05's KindTerm und sein <select> mit leerer erster Moeglichkeit"
provides:
  - "switchOf(d field.Def) — von der blossen Art auf die ganze Definition erweitert, eine einzige Aufrufstelle"
  - "der neue Schaltername \"knopfreihe\" und die eine neue Regel .feld-schalter--knopfreihe in admin.css"
  - "die stillschweigend kaputten Faelle repariert: eine Mehrfachauswahl meldet jetzt \"kreuz\", ein Schlagwortfeld \"auswahl\""
  - "FieldView.Buttons() und ein um die Knopfreihe erweitertes FieldView.Grouped()"
  - "die Knopfreihe in field_input.html: Radioknoepfe mit ausdruecklichem Leerknopf, role=group und aria-labelledby, ohne eine Zeile JavaScript"
  - ".form-knopfreihe — eine umbrechende Flexreihe aus bestehenden Abstandstoken"
  - "internal/admin/page_fields_switch_test.go — je ein Fall pro steuernder Art, Klasse und Element zusammen, plus zwei allgemeine Waechter"
affects: [07-07, 08-snippets, 09-csv-import, 11-galerie]

actuals:
  tokens: 6772
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Ein Praedikat, das eine Regel traegt, wird erweitert und nie kopiert: Grouped() deckt jetzt zwei Gruppenarten ab, die Vorlage lernt nichts Neues"
    - "Eine Funktion, deren Antwort von der Darstellung abhaengt, bekommt die ganze Definition statt einer Zeichenkette — die Darstellung ist Teil der Antwort"
    - "Ein Vertrag zwischen Server und Stylesheet, den zur Laufzeit niemand prueft, wird im Test von beiden Seiten gemessen: die Klasse am Markup UND der Selektor in der Datei"
    - "Ein Test, der eine stumme Fehlfunktion abfangen soll, wird durch Mutation nachgewiesen — die Regel einmal falsch stellen, den Schalternamen einmal zurueckdrehen, beides muss rot werden"

key-files:
  created:
    - internal/admin/page_fields_switch_test.go
  modified:
    - internal/admin/page_fields.go
    - cmd/holzcloud/assets/admin.css
    - cmd/holzcloud/templates/admin/field_input.html

key-decisions:
  - "switchOf nimmt field.Def statt eines string-Kind — eine zweite Parameterzeile oder eine blosse Darstellungszeichenkette waeren beide Umschreibungen derselben Entscheidung an zwei Stellen"
  - "Die neue Regel kopiert die Form der Auswahl (verbergen bei Treffer, sichtbar als Vorgabe) und nicht die des Kreuzes — eine beantwortete Auswahl zeigt ihre Abhaengigen, eine offene verbirgt sie"
  - "Der Behaelter heisst .form-knopfreihe, in der Reihe von .form-check, .form-code, .form-select — nur Abstaende, keine Farbe, die kein Token ist"
  - "Der Leerknopf steht INNERHALB des Behaelters als erstes .form-check, damit er in derselben Flexreihe sitzt und trotzdem Nachfahre der .form-group bleibt, die der Selektor verlangt"
  - "MayControl() blieb unangetastet: KindTime ausgeschlossen, KindRange drin. D-08 ist die offene Frage des Browserdurchgangs in 07-07 und wurde hier nicht entschieden"
  - "Der Test wurde vor Schritt 1 und 2 geschrieben und rot festgehalten, wie Schritt 3 es ausdruecklich verlangt — die Reihenfolge der Commits ist test → feat → feat → test"

patterns-established:
  - "Signaturerweiterung statt Zusatzparameter, wenn die Frage die ganze Definition betrifft"
  - "Der Klasse-und-Element-Doppelgriff: nur die Klasse zu pruefen laesst eine veraltete Regel durch, nur das Element einen falschen Namen"
  - "Ein Waechter, der einen unbekannten Schalternamen meldet statt ihn zu uebergehen — die naechste Art kommt nicht stillschweigend vorbei"

requirements-completed: [FIELD-01, FIELD-08]

coverage:
  - id: D1
    description: "Eine Auswahl als Knopfreihe zeichnet sich als Reihe von Radioknoepfen: vier Knoepfe bei drei Moeglichkeiten, in getippter Reihenfolge, der leere zuerst; eine doppelte Zeile ergibt zwei Knoepfe mit demselben Wert; eine gewoehnliche Auswahl bleibt eine Klappliste"
    requirement: FIELD-01
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestKnopfreihe"
        status: pass
    human_judgment: false
  - id: D2
    description: "Der ausdrueckliche Leerknopf: ohne gespeicherten Wert angekreuzt, mit gespeichertem Wert nicht mehr, und der gespeicherte Wert stattdessen — eine Radiogruppe bleibt damit in reinem HTML abwaehlbar (D-11)"
    requirement: FIELD-01
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestKnopfreihe"
        status: pass
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestSchalter/eine_Auswahl_als_Knopfreihe"
        status: pass
    human_judgment: false
  - id: D3
    description: "Jede steuernde Art meldet einen Schalternamen, dessen Regel greifen kann, und traegt das Element, das diese Regel auswaehlt — Knopfreihe, Klappliste, Mehrfachauswahl, Schlagwort, Bereich, Code (D-10, FIELD-08)"
    requirement: FIELD-08
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestSchalter"
        status: pass
      - kind: other
        ref: "for s in kreuz auswahl text knopfreihe; do grep -c \"feld-schalter--$s\" cmd/holzcloud/assets/admin.css; done — 2 1 2 1, keine Null"
        status: pass
    human_judgment: false
  - id: D4
    description: "Zu jedem Schalternamen, den der Server senden kann, gibt es eine Regel in admin.css, und diese Regel greift am richtigen Selektor; ein unbekannter Name wird gemeldet statt uebergangen (T-07-24)"
    requirement: FIELD-08
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestSchalter/zu_jedem_Schalternamen_gibt_es_eine_Regel"
        status: pass
      - kind: other
        ref: "Mutationsnachweis: Regel auf option[value=\"\"] umgestellt → rot; switchOf auf \"auswahl\" zurueckgedreht → rot; beides wiederhergestellt"
        status: pass
    human_judgment: false
  - id: D5
    description: "Kein for= im Seitenformular nennt eine Kennung, die es nicht gibt; die beiden Gruppen — Kaestchengruppe und Knopfreihe — tragen kein for= und nennen ihre Beschriftung ueber aria-labelledby"
    requirement: FIELD-01
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestSchalter/jedes_for_zeigt_auf_eine_Kennung,_die_es_gibt"
        status: pass
    human_judgment: false
  - id: D6
    description: "Eine Uhrzeit wird nicht als Bedingung angeboten — die Ausnahme aus 07-03 kann nicht zurueckgenommen werden, ohne dass ein Test rot wird"
    requirement: FIELD-08
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestSchalter/an_einer_Uhrzeit_haengt_nichts"
        status: pass
    human_judgment: false
  - id: D7
    description: "Die Knopfreihe traegt keine Zeile JavaScript: kein <script>, kein onclick, kein javascript:, kein hx-Attribut, das sie zum Funktionieren braucht (T-07-27)"
    requirement: FIELD-01
    verification:
      - kind: other
        ref: "grep -c 'onclick\\|javascript:\\|<script' cmd/holzcloud/templates/admin/field_input.html = 0"
        status: pass
      - kind: integration
        ref: "internal/admin/page_fields_kinds_test.go#TestFeldartenTragenKeinJavaScript"
        status: pass
    human_judgment: false
  - id: D8
    description: "Eine Auswahl als Klappliste zeichnet sich unveraendert — die untere Haelfte des Zweiges ist byteweise dieselbe wie vorher"
    requirement: FIELD-01
    verification:
      - kind: other
        ref: "git diff cmd/holzcloud/templates/admin/field_input.html | grep '^-' — keine entfernte Zeile"
        status: pass
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestSchalter/eine_Auswahl_als_Klappliste"
        status: pass
    human_judgment: false
  - id: D9
    description: "Ob :has() und :placeholder-shown sich an einem echten Zahlenfeld im Browser so verhalten wie angenommen (D-08) — der Markup-Beweis sagt darueber nichts"
    verification: []
    human_judgment: true
    rationale: "D-08 ist ausdruecklich offen und wird durch den Browserdurchgang in Plan 07-07 entschieden, nicht hier. Ein gruener Testlauf ist kein bestaetigtes Browserverhalten; der Test sagt das in einem Kommentar an seinem Kopf ausdruecklich."

# Metrics
duration: 9 min
completed: 2026-09-05
status: complete
---

# Phase 7 Plan 6: Die Knopfreihe und ihre Falle — Summary

**Eine Auswahl als Reihe von Radioknöpfen mit ausdrücklichem Leerknopf, ein auf `field.Def` erweitertes `switchOf` mit dem neuen Schalter `knopfreihe`, und die Entdeckung, dass eine Mehrfachauswahl und ein Schlagwortfeld ihre abhängigen Felder schon vor diesem Plan nicht mehr ein- und ausblenden konnten.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-09-05T15:24:15Z
- **Completed:** 2026-09-05T15:33:29Z
- **Tasks:** 3
- **Files modified:** 4 (davon 1 neu)

## Accomplishments

- **`switchOf` nimmt die ganze Definition.** Ein `string`-Kind kann „eine Auswahl als Knopfreihe" nicht ausdrücken; die Darstellung gehört zur Antwort. Eine einzige Aufrufstelle, `viewOf`, hatte die `Def` ohnehin in der Hand.
- **Die Falle D-10 ist zu.** Eine Knopfreihe meldet jetzt `knopfreihe`, und dazu gibt es eine Regel in `admin.css`, die am angekreuzten leeren Radioknopf greift statt an einer gewählten `<option>`, die eine Radioreihe nicht hat.
- **Zwei stille Fehlfunktionen fielen dabei mit auf** — sie standen schon vor diesem Plan im Baum, und der rote Testlauf hat sie sichtbar gemacht (siehe *Issues Encountered*).
- **Die Knopfreihe selbst**: Radioknöpfe in getippter Reihenfolge mit einem ausdrücklichen Leerknopf davor, in einer umbrechenden Flexreihe, mit `role="group"` und `aria-labelledby` in derselben Schreibweise wie die Kästchengruppe — und ohne eine Zeile JavaScript.
- **Der Mechanismus wurde genau einmal angefasst.** Die vier bestehenden `.feld-schalter--*`-Regeln stehen unverändert; genau eine kam dazu.
- **Ein Test je steuernder Art**, der Klasse und Element zusammen behauptet, dazu zwei allgemeine Wächter: eine Regel für jeden Schalternamen (samt Selektor), und kein `for=`, das auf eine Kennung zeigt, die es nicht gibt.

## Task Commits

1. **Schritt 3 (rot vorweg): der Test** — `3eed72e` (test)
2. **Schritt 1: `switchOf`, `Buttons`, `Grouped`, die neue Regel** — `afae06a` (feat)
3. **Schritt 2: die Knopfreihe und `TestKnopfreihe`** — `8444d1d` (feat)
4. **Schritt 3 (grün, geschärft): der Selektor-Wächter** — `c9eb43a` (test)

Die Reihenfolge `test → feat → feat → test` ist Absicht und steht so in Schritt 3 des Plans: „Write … first, and watch it fail before Tasks 1 and 2 are finished." Der rote Lauf ist unten protokolliert.

## Files Created/Modified

- `internal/admin/page_fields.go` — `switchOf(d field.Def)` mit vier Fällen und erweitertem Kommentar; `FieldView.Buttons()`; `FieldView.Grouped()` um die Knopfreihe erweitert; die eine Aufrufstelle in `viewOf`
- `cmd/holzcloud/assets/admin.css` — `.feld-schalter--knopfreihe` (Form der Auswahl, Selektor auf den Radioknopf) und `.form-knopfreihe` (umbrechende Flexreihe aus `--space-1` / `--space-2`)
- `cmd/holzcloud/templates/admin/field_input.html` — der `auswahl`-Zweig teilt sich an `{{if .Buttons}}`; die untere Hälfte ist das `<select>` unverändert
- `internal/admin/page_fields_switch_test.go` *(neu)* — `TestSchalter` mit neun Unterfällen und `TestKnopfreihe`

## Decisions Made

- **`switchOf` bekommt `field.Def`, nicht einen zweiten Parameter.** Die Frage lautet „welche Regel gehört zu diesem Feld", und das Feld ist die Definition.
- **Die neue Regel kopiert die Form der `auswahl`, nicht die des `kreuz`.** Vorgabe sichtbar, verborgen bei Treffer: eine beantwortete Auswahl zeigt ihre Abhängigen, eine offene verbirgt sie. Die `kreuz`-Form (Vorgabe verborgen) wäre die falsche Leseart.
- **Der Leerknopf steht innerhalb des Behälters** als erstes `.form-check`, damit er in derselben Flexreihe sitzt. Er bleibt Nachfahre der `.form-group`, die der Selektor `> .form-group input[type="radio"][value=""]:checked` verlangt.
- **Keine neue Übersetzungszeichenkette.** Der Leerknopf trägt dieselben zwei Sätze, die die erste Möglichkeit der Klappliste schon benutzt (`bitte wählen` / `– keine Angabe –`), über dasselbe `{{t "…"}}`. Die i18n-Werkzeuge mussten deshalb nicht laufen.
- **`MayControl()` blieb unangetastet.** `KindTime` ausgeschlossen, `KindRange` drin. D-08 gehört dem Browserdurchgang in 07-07, und dieser Plan durfte ihn nicht vorwegnehmen.
- **`.form-knopfreihe` trägt nur Abstände.** Beschriftung und Knopf kommen von `.form-check`; keine Farbe, die kein Token ist.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] `TestKnopfreihe` hat gefehlt, obwohl Schritt 2 ihn im eigenen `<verify>` nennt**

- **Found during:** Schritt 2
- **Issue:** Schritt 2 verlangt in seinen Abnahmekriterien „vier Radioknöpfe in Optionsreihenfolge, der erste mit leerem Wert" und „eine doppelte Zeile ergibt zwei Knöpfe mit demselben Wert". Kein Test im Plan behauptete das — Schritt 3 deckt die Arten ab, nicht die Reihenfolge. Zugleich nennt Schritt 2's Prüfbefehl `-run 'TestKnopfreihe|TestFeldarten'` einen Test dieses Namens, den es nicht gab; der Befehl wäre grün gelaufen, ohne von der Knopfreihe ein Wort zu sagen.
- **Fix:** `TestKnopfreihe` geschrieben: drei Möglichkeiten ergeben vier Knöpfe in getippter Reihenfolge, der leere zuerst und ohne gespeicherten Wert angekreuzt; ein gespeicherter Wert ist angekreuzt und der leere dann nicht mehr; eine Liste mit doppelter Zeile ergibt zwei Knöpfe mit demselben Wert; eine gewöhnliche Auswahl trägt keinen einzigen Radioknopf.
- **Files modified:** `internal/admin/page_fields_switch_test.go`
- **Verification:** `go test ./internal/admin/ -run 'TestKnopfreihe|TestFeldarten' -v` — grün, keine „no tests to run"-Warnung
- **Committed in:** `8444d1d`

**2. [Rule 1 - Bug] Das Messfenster von `umFeld` blutete in das Nachbarfeld**

- **Found during:** Schritt 2 (beim Schreiben von `TestKnopfreihe`)
- **Issue:** Die Gegenprobe „eine gewöhnliche Auswahl ist keine Knopfreihe" las mit `umFeld` ein Fenster von ±600 Zeichen um `name="feld_glanz"`. Darin stand die Knopfreihe des Feldes davor, und der Test schlug fehl, obwohl die Klappliste völlig in Ordnung war — ein Messfehler, der als Befund erschienen wäre.
- **Fix:** Am Element selbst gemessen (`imTag`) und zusätzlich an den Radioknöpfen, die genau diesen Feldnamen tragen. Dasselbe Muster, das 07-03 schon für `min`/`max` eingeführt hat.
- **Files modified:** `internal/admin/page_fields_switch_test.go`
- **Verification:** `TestKnopfreihe` grün; die Behauptung ist jetzt an das gemeinte Element gebunden
- **Committed in:** `8444d1d`

**3. [Rule 2 - Missing Critical] Der Stylesheet-Wächter prüfte nur, DASS es eine Regel gibt**

- **Found during:** Schritt 3
- **Issue:** T-07-24 ist genau die Regel, die den richtigen Namen trägt und am falschen Element greift. Ein Wächter, der nur `strings.Contains(css, ".feld-schalter--knopfreihe")` prüft, hätte eine Regel durchgelassen, die weiter nach einem `<option>` sucht — also genau die stumme Fehlfunktion, um deretwillen dieser Plan zuletzt eingeplant wurde.
- **Fix:** Der Wächter kennt zu jedem Schalternamen den Selektor, den seine Regel tragen muss, und meldet zusätzlich einen Namen, den er noch nicht kennt.
- **Files modified:** `internal/admin/page_fields_switch_test.go`
- **Verification:** Mutationsnachweis — Regel auf `option[value=""]` umgestellt → rot; `switchOf` auf `"auswahl"` zurückgedreht → rot; beides wiederhergestellt, Suite grün
- **Committed in:** `c9eb43a`

---

**Total deviations:** 3 auto-fixed (2 fehlende kritische Absicherung, 1 Fehler im eigenen Test)
**Impact on plan:** Kein Scope-Zuwachs. Alle drei betreffen den Beweis, nicht das Verhalten: zwei schliessen Abnahmekriterien, die im Plan standen und von keinem Test getragen wurden, der dritte war ein Messfehler im frisch geschriebenen Test.

## Issues Encountered

**Zwei Arten konnten ihre abhängigen Felder schon vor diesem Plan nicht mehr ein- und ausblenden.** Der rote Lauf des Tests hat es gemessen, bevor eine Zeile Produktionscode geändert war:

- eine **Mehrfachauswahl** meldete `"text"` — deren Regel sucht `:placeholder-shown`, und eine Kästchengruppe trägt keinen Platzhalter
- ein **Schlagwortfeld** meldete ebenfalls `"text"` — obwohl sein Bedienelement ein `<select>` mit leerer erster Möglichkeit ist, also genau das, was die `auswahl`-Regel liest

Beide Arten wurden in 07-01 und 07-05 geboren und fielen in `switchOf`s `default`-Zweig; kein Fehler, kein Protokolleintrag, nur ein Formular, das die falschen Felder zeigt. Genau die Fehlfunktion, die D-10 für die Knopfreihe vorhersagt — sie war für zwei weitere Arten schon eingetreten. Schritt 1 des Plans hat beide Fälle vorgesehen und behoben. Das ist der Ertrag daraus, den Mechanismus einmal als Ganzes anzufassen statt je Art zu flicken.

**Kein Checkpoint in diesem Plan.** Alle drei Schritte sind `type="auto"`; es gab kein `checkpoint:*` und damit kein Gate, das automatisch bestätigt worden wäre. Auto-Modus war aktiv (`workflow.auto_advance: true`), blieb hier aber ohne Wirkung.

## Known Stubs

Keine. Kein hartkodierter Leerwert, kein Platzhaltertext, kein TODO in den geänderten Dateien. Der einzige `placeholder=" "` im Formular ist kein Stub, sondern das Element, an dem `.feld-schalter--text` greift.

## Threat Flags

Keine neue sicherheitsrelevante Oberfläche. Kein Endpunkt, kein Authentifizierungspfad, kein Dateizugriff, keine Schemaänderung. Die Werte der Knopfreihe laufen durch dasselbe `field.Check` mit `KindChoice`, das die Klappliste schon prüft (T-07-25), und der Zweig trägt keine Zeile JavaScript (T-07-27).

## User Setup Required

Keine — kein externer Dienst, keine neue Umgebungsvariable, keine Migration.

## Next Phase Readiness

- **FIELD-01 und FIELD-08 sind erfüllt**, und die vier bestehenden Bedingungsregeln wurden in dieser Phase genau einmal angefasst.
- **07-07 kann anfangen.** Der Browserdurchgang findet die Knopfreihe vor, an der er `:has()` beobachten soll, und die offene Frage D-08 ist unverändert offen gelassen: `MayControl()` gibt für ein Bereichsfeld weiter `true` zurück. Der Umkehrweg — `KindRange` neben `KindDate` in die Ausnahme — steht in 07-07 bereits geschrieben.
- **Ein Hinweis für 07-07:** der Markup-Beweis dieses Plans sagt nichts über das Verhalten eines echten Browsers. Der Test sagt das an seinem Kopf ausdrücklich, damit ein grüner Lauf nicht für ein bestätigtes `:placeholder-shown` an einem Zahlenfeld gehalten wird.

## Self-Check: PASSED

- `internal/admin/page_fields_switch_test.go` — FOUND
- `internal/admin/page_fields.go` — FOUND
- `cmd/holzcloud/assets/admin.css` — FOUND
- `cmd/holzcloud/templates/admin/field_input.html` — FOUND
- Commits `3eed72e`, `afae06a`, `8444d1d`, `c9eb43a` — alle vier im Log
- `go build ./...`, `go vet ./...`, `gofmt -l internal/ cmd/` — sauber
- `go test ./...` — grün

---
*Phase: 07-field-kinds*
*Completed: 2026-09-05*
