---
phase: 08-snippets-carry-fields
plan: 04
subsystem: ui
tags: [go, html-template, admin, snippets, field-values, xss, contextual-escaping, i18n]

# Dependency graph
requires:
  - phase: 08-01
    provides: "snippet.Store.SetFields, Snippet.Fields und field.Store.OfSnippet — der Speicher und der Leser, auf denen dieses Formular steht"
  - phase: 08-02
    provides: "validate's Textbaustein-Arm: gilt_fuer auf beides gestellt, Bedingung geleert — der Grund, warum hier kein field.For nötig ist"
  - phase: 08-03
    provides: "die Definitionshälfte — der vierte Modus des Feldbildschirms und snippet_fields_test.go, das dieser Plan erweitert"
provides:
  - "admin.SnippetValues.Fields, gefüllt aus fieldsFromRequest"
  - "admin.SnippetListData.FieldViews, .Media, .RefPages, .RefTerms und pool()"
  - "(*admin.Handler).snippetFieldDefs — der eine Ladeweg der Definitionen dieses Bildschirms"
  - "Der Speicherweg groupAction → field.CheckAll → field.Clean → field.Encode → snippets.SetFields"
  - "Die ?edit=-Vorfüllung über field.Decode"
  - "Der Feldteil von snippet_list.html, gebaut aus field_top"
  - "Vier weitere Prüfungen in internal/admin/snippet_fields_test.go, alle drei mutationsgeprüft"
  - "T-07-26 geschlossen, mit der Massnahme auf field.CheckAll zurückgeschrieben"
affects: [08-05, 09-csv-import]

actuals:
  tokens: 6100
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "Ein zweiter Träger benutzt den Parser und den Bauplatz des ersten, statt einen zweiten zu prägen"
    - "Eine Sicherheitszusage wird bewiesen, indem zwei Träger als gleich behauptet werden — nicht, indem der Maskierer nachgeprüft wird"
    - "Eine falsche Massnahme wird korrigiert, nicht gebaut, und der Grund steht im Eintrag"

key-files:
  created: []
  modified:
    - internal/admin/snippet.go
    - cmd/holzcloud/templates/admin/snippet_list.html
    - internal/admin/snippet_fields_test.go
    - .planning/phases/07-field-kinds/07-SECURITY.md
    - internal/i18n/locales/en.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json

key-decisions:
  - "groupAction steht VOR values.validate und nicht dahinter, wie die Schrittliste des Plans sagt: die Zeilenknöpfe tragen formnovalidate, damit ein leeres Pflichtfeld das Hinzufügen einer Zeile nicht blockiert — hinter dem Errors.Any()-Tor wäre genau das passiert"
  - "Der ?edit=-Zweig liest die Werte, BEVOR snippetListData läuft: die Feldeingaben werden aus values.Fields gebaut, und eine Vorfüllung danach hätte leere Kästchen gezeichnet"
  - "Die Sanierungsprüfung liegt in internal/admin über field.Resolve und dieselbe Vorlage über beide Träger, nicht über die echte öffentliche Route: deren Vorrichtung wohnt in den _test-Dateien von internal/public und ist nicht importierbar; die echte Route hält seit 08-01 TestBausteinfelderErreichenDasTheme"
  - "Zusammenfassungstabelle und Frontmatter-Zähler von 07-SECURITY.md nachgezogen (medium 11→12 geschlossen, 2→1 offen): einen Eintrag zu schliessen und die Zähler stehen zu lassen hiesse, dasselbe Dokument eine neue Unwahrheit behaupten zu lassen"
  - "Das Zähltor grep -c 'T-07-26' misst 2 statt 1 — eine zu lang, Ursache ist eine vorbestehende Fremdnennung in W-4; gemeldet, nicht passend gemacht"

patterns-established:
  - "Mutationsprobe für eine Sicherheitszusage: den Guss einbauen, den das Tor verbietet, und nachsehen, dass die Prüfung auf beiden Trägern rot wird"
  - "Ein Zähltor, das eine Fremdnennung mitzählt, wird gemeldet und nicht durch Streichen der Fremdnennung grün gemacht"

requirements-completed: [SNIP-01, SNIP-03, SNIP-05]  # in REQUIREMENTS.md abgehakt: SNIP-01. SNIP-03 und SNIP-05 werden von Plan 08-05 ebenfalls geführt und bleiben bis zu dessen Zusammenfassung offen (Tor auf geteilte Kennungen).

coverage:
  - id: D1
    description: "Ein Feldwert eines Textbausteins übersteht Speichern und Wiederöffnen: getippt, durch fieldsFromRequest geparst, durch CheckAll geprüft, durch Clean gesäubert, durch Encode geschrieben, durch SetFields abgelegt — und im wiedergeöffneten Formular unter demselben Namen wieder da"
    requirement: "SNIP-01"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestSnippetFeldRundlauf"
        status: pass
      - kind: other
        ref: "Mutationsprobe: SetFields entfernt → drei Fälle fallen; Fieldset aus der Vorlage entfernt → zwei fallen"
        status: pass
    human_judgment: false
  - id: D2
    description: "Ein abgewiesenes Speichern schreibt nichts — auch nicht halb: der getippte Wert des anderen Feldes kommt zurück, ein Grund steht daneben, und der Speicher trägt noch den vorherigen Wert"
    requirement: "SNIP-01"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestSnippetFeldPflichtWirdAbgewiesen"
        status: pass
    human_judgment: false
  - id: D3
    description: "Ein <script> in einem langtext-Feld eines Textbausteins überlebt nicht, und derselbe Wert im selben Feldart auf einer Seite ergibt dasselbe — beide über den default:-Arm von field.Resolve und die kontextabhängige Maskierung von html/template"
    requirement: "SNIP-03"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestSnippetFeldSanierung"
        status: pass
      - kind: other
        ref: "Mutationsprobe: ein Guss nach template.HTML im KindLong-Arm von field.Resolve → die Prüfung fällt auf BEIDEN Trägern, danach wiederhergestellt"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/admin/snippet.go | grep -c 'template.HTML\\|RenderMarkdown(' → 1 (der eine erlaubte Treffer: die Kette des Rumpfes)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Der Rumpf des Textbausteins behält seine eigene, andere Kette (goldmark → bluemonday) und ist von dieser Phase unberührt; Kennung und Rumpf überstehen ein Speichern ohne Feldwerte"
    requirement: "SNIP-05"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestSnippetFeldSanierung"
        status: pass
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestSnippetFeldRundlauf"
        status: pass
    human_judgment: false
  - id: D5
    description: "Das Seiteneditor-Formular trägt weder die Beschriftung noch den Formularnamen eines Textbausteinfeldes — die im Browser sichtbare Hälfte von D-03"
    requirement: "SNIP-05"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestSnippetFeldStehtNichtImSeitenformular"
        status: pass
    human_judgment: false
  - id: D6
    description: "Ein Textbaustein ohne Definitionen zeigt kein Fieldset, keine Überschrift und keinen leeren Kasten; ein neuer, den es noch nicht gibt, ebenso wenig"
    requirement: "SNIP-01"
    verification:
      - kind: other
        ref: "{{if .FieldViews}} in cmd/holzcloud/templates/admin/snippet_list.html; snippetFieldDefs gibt für snippetID == 0 nil zurück"
        status: pass
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestTextbausteinModusOeffnetSichFuerDeneigenen (der leere Fall, aus 08-03)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Kein zweiter Parser, kein zweiter Bauplatz, keine zweite Prägestelle, kein JavaScript"
    requirement: "SNIP-01"
    verification:
      - kind: other
        ref: "grep -c 'func fieldsFromRequest|func fieldViews|func groupAction' page_form.go page_fields.go → 1 / 2"
        status: pass
      - kind: other
        ref: "grep -c 'name=\"feld_' snippet_list.html → 0; grep -ci 'script|onclick|onchange|javascript:' → 0"
        status: pass
    human_judgment: false
  - id: D8
    description: "T-07-26 ist geschlossen: die Massnahme nennt field.CheckAll statt field.Hidden, der Eintrag steht in einem Geschlossen-Abschnitt und ist aus der Liste der nächsten Schritte gestrichen; kein Verhalten geändert"
    verification:
      - kind: other
        ref: "grep -c 'field.CheckAll' 07-SECURITY.md → 2; git diff -- internal/field/field.go → leer"
        status: pass
    human_judgment: false
  - id: D9
    description: "Der Bildschirm im Browser: das Aussehen des Feldteils unter dem Markdown-Kasten, das Register der Legende, die Bedienbarkeit einer Gruppenzeile ohne Skript"
    verification: []
    human_judgment: true
    rationale: "Optik, Register und die Frage, ob sich eine Gruppenzeile am echten Bildschirm angenehm hinzufügen lässt, sind Urteilsfragen. Die Browserhälfte der Phase ist Plan 08-05 zugewiesen; hier steht das Formular samt seinem Speicherweg, gefahren über die echten Vorlagen von der Platte."

# Metrics
duration: 10 min
completed: 2026-09-06
status: complete
---

# Phase 8 Plan 04: Das Wertformular des Textbausteins Summary

**Das Textbaustein-Formular trägt jetzt seine eigenen Felder — durch den Parser, den Bauplatz, die Prüfung und die Vorlagen des Seiteneditors, ohne eine einzige zweite Prägestelle; und SNIP-03 ist bewiesen, indem eine Prüfung behauptet, dass ein `<script>` am Textbaustein und dasselbe `<script>` an einer Seite Zeichen für Zeichen dasselbe ergeben, weil `KindLong` in `field.Resolve` keinen eigenen Arm hat und `html/template` beide beim Drucken maskiert.**

## Performance

- **Duration:** 10 min
- **Started:** 2026-09-06T10:14:27Z
- **Completed:** 2026-09-06T10:25:11Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- **Komposition statt Nachbau, und die Zähltore beweisen es.** `fieldsFromRequest` bleibt mit **1** Definition der eine Parser, `fieldViews` mit **1** der eine Bauplatz, `groupAction` mit dem zweiten Treffer in `page_fields.go` bei **2**. Der Textbaustein-Bildschirm ruft alle drei und schreibt keinen davon ab. Das ist die Form, in der Phase 7 zweimal einen Kritischen ausgeliefert hat, hier vorweggenommen.
- **Der Speicherweg ist die Fünferfolge einer Seite, in derselben Reihenfolge.** `groupAction` → `field.CheckAll` → `field.Clean` → `field.Encode` → `snippets.SetFields`. Geprüft wird über die Definitionen, die der **Server** über `OfSnippet` geladen hat, nie über die, die das Formular behauptet. Kein `field.For`: der Filter verengt nach „gilt für", was `validate` an einem Textbaustein ohnehin auf „beides" stellt — der Grund steht als Kommentar an beiden Aufrufstellen.
- **Die Ablehnung ist ein neu gezeichnetes Formular und kein Flash.** `web.RenderFormError` mit `web.FormErrors`, die Redewendung, die dieser Bildschirm schon benutzt. Ein Flash überlebt eine Umleitung, aber nicht den getippten Inhalt, und das Wertformular eines Textbausteins ist lang.
- **SNIP-03 ist bewiesen, ohne den Maskierer nachzuprüfen.** `TestSnippetFeldSanierung` legt `<script>alert(1)</script>` in ein `langtext`-Feld eines Textbausteins **und** in ein `langtext`-Feld einer Seite, löst beide über `field.Resolve` auf und druckt beide durch dieselbe Vorlage: keine lebende Marke, und die zwei Ausgaben stimmen Zeichen für Zeichen überein. Die Prüfung sagt in ihrem Doc-Kommentar ausdrücklich, dass **keine** Markdown-Kette auf einem Feldwert läuft — die Kette gehört dem Rumpf und wird nur genannt, um die Linie zu ziehen.
- **Drei Mutationsproben, bevor die Prüfungen als Beweis geführt werden.** `SetFields` entfernt → drei Fälle fallen. Das Fieldset aus der Vorlage entfernt → zwei fallen. Ein Guss nach `template.HTML` im `KindLong`-Arm von `field.Resolve` eingebaut → die Sanierungsprüfung fällt auf **beiden** Trägern mit der überlebenden Marke im Fehlertext. Danach jeweils wiederhergestellt und grün.
- **T-07-26 geschlossen, indem die Behauptung korrigiert wird.** Der genannte Mechanismus existiert nicht: `field.Hidden` steht auf keinem Speicherweg, und `field.Clean` behält den Wert eines versteckten Feldes ausdrücklich. Gebaut wurde nichts, aus drei aufgeschriebenen Gründen; was schützt, ist `field.CheckAll` über die geladenen Definitionen. `internal/field/field.go` steht in keinem Diff dieses Plans.
- **Eine neue sichtbare Zeichenkette, übersetzt in ihrem eigenen Commit.** „Angaben zu diesem Textbaustein" — das Gegenstück zu „Angaben zu dieser Seite", im selben Register. `-write`, dann `-schweiz` (ohne Änderung an `de-CH`), dann en/es/fr/it von Hand; `go run ./tools/i18n` meldet für alle vier `0 offen, 0 verwaist`.

## Task Commits

1. **Task 1: Das Wertformular trägt Felder** — `2b61428` (feat)
2. **Task 1, zweite Hälfte: T-07-26** — `52a2490` (docs, eigener Commit wie vom Plan verlangt)
3. **Task 2: Der Feldteil des Formulars** — `ce211da` (feat)
4. **Die neue Zeichenkette in en, es, fr, it** — `f8c6ffc` (chore)
5. **Task 3: Die vier Prüfungen** — `3cd179c` (test)

**Plan metadata:** der `docs(08-04)`-Commit dieses Plans

## Files Created/Modified

- `internal/admin/snippet.go` — `SnippetValues.Fields`; `SnippetListData.FieldViews`, `.Media`, `.RefPages`, `.RefTerms` und `pool()`; `snippetFieldDefs`; die Vorräte und die Feldeingaben in `snippetListData`; `groupAction`, `CheckAll`, `Clean`, `Encode` und `SetFields` in `handleSnippetSave`; die `?edit=`-Vorfüllung über `field.Decode`
- `cmd/holzcloud/templates/admin/snippet_list.html` — ein `<fieldset class="own-fields">` zwischen Markdown-Kasten und Speichern-Knopf, gewickelt in `{{if .FieldViews}}`, gerendert über `field_top`; kein `name`-Attribut, keine neue CSS-Regel, kein Skript
- `internal/admin/snippet_fields_test.go` — vier weitere Fälle und fünf Helfer; die Doc-Prosa nennt den Mechanismus, der wirklich schützt, und beschreibt, wie ein übersehener Ort aussähe, wenn nur die zählenden Tore liefen
- `.planning/phases/07-field-kinds/07-SECURITY.md` — T-07-26 umgeschrieben und geschlossen, aus der Liste der nächsten Schritte gestrichen, Zusammenfassungstabelle und Frontmatter-Zähler nachgezogen
- `internal/i18n/locales/{en,es,fr,it}.json` — je eine neue Übersetzung

## Decisions Made

- **`groupAction` steht vor `values.validate`, nicht dahinter.** Siehe Abweichung 1.
- **Der `?edit=`-Zweig liest die Werte vor `snippetListData`.** Die Feldeingaben werden aus `values.Fields` gebaut; die Vorfüllung danach in `data.Values` zu schreiben — wie es vorher stand — hätte ein wiedergeöffnetes Formular mit gefülltem Markdown-Kasten und leeren Feldkästchen ergeben. Die vom Plan verlangte Zeile `Fields: field.Decode(sn.Fields)` steht unverändert im `SnippetValues`-Literal jenes Zweiges.
- **`web.RenderFormError` auch für den Zeilenknopf**, wie der Plan es schreibt — obwohl der Seiteneditor an derselben Stelle `web.RenderAdmin` mit 200 benutzt. Das Ergebnis ist ein 422 für ein erfolgreiches Zeilenhinzufügen. Der Rumpf ist derselbe, der Browser zeichnet ihn gleich, und die Anweisung des Plans ist ausdrücklich; notiert, damit die Abweichung vom Vorbild sichtbar bleibt, falls sie später stört.
- **Die Definitionen werden zweimal geladen** — einmal in `snippetListData` für die Eingaben, einmal im Speicherweg für die Prüfung. Genau so macht es der Seiteneditor (`newPageFormData` und `handlePageCreatePost` rufen beide `fieldDefs`); eine gemeinsame Zwischenspeicherung wäre ein dritter Weg gewesen, den nichts verlangt.
- **`SetFields` läuft auch beim Anlegen**, mit leerer Zeichenkette. Ein neuer Textbaustein hat keine Definitionen, `Encode` einer leeren `field.Data` gibt `""`, und das UPDATE ist ein Nichts. Die Alternative wäre ein Zweig gewesen, der die Zusage „ein leeres `field.Data` wird als Zeichenkette abgelegt, die `Decode` als leer zurückliest" nicht ausübt.
- **Die Zähler von `07-SECURITY.md` sind nachgezogen.** Der Plan verlangt, keinen **anderen Eintrag** zu ändern; die Zusammenfassungstabelle und die Frontmatter-Zähler sind Buchhaltung und kein Eintrag. Einen Punkt zu schliessen und `threats_open_below_threshold: 2` stehen zu lassen hiesse, dasselbe Dokument eine neue Unwahrheit behaupten zu lassen — die Fehlerklasse, die dieser Plan gerade korrigiert.

## Deviations from Plan

### Auto-fixed Issues

**1. [Regel 1 — Fehler] `groupAction` steht vor `values.validate`, nicht dahinter**

- **Found during:** Task 1
- **Issue:** Die Schrittliste des Plans setzt den Zeilenknopf „after the existing `values.validate(data.Errors)`". Unmittelbar nach `validate` steht aber das Tor `if data.Errors.Any() { return web.RenderFormError(…) }`. Wer eine Gruppenzeile hinzufügt, während Name oder Kennung noch leer sind, hätte statt der neuen Zeile eine Fehlermeldung bekommen — und die Zeilenknöpfe tragen in `field_top.html` ausdrücklich `formnovalidate`, genau damit das nicht geschieht („damit ein leeres Pflichtfeld weiter unten das Hinzufügen einer Zeile nicht blockiert").
- **Fix:** `groupAction` läuft als Erstes im Speicherweg, vor `snippetListData` und vor `validate` — dieselbe Stelle, die `page.go:419` benutzt.
- **Files modified:** `internal/admin/snippet.go`
- **Verification:** `go test ./internal/admin/` grün; die vom Plan verlangte Reihenfolge `groupAction` → `CheckAll` → `Clean` → `Encode` → `SetFields` ist unverändert eingehalten.
- **Committed in:** `2b61428`

**2. [Regel 3 — Blockierend] Die Sanierungsprüfung druckt in `internal/admin`, nicht über die echte öffentliche Route**

- **Found during:** Task 3
- **Issue:** Der Plan verlangt „Render both to the public output". Die Vorrichtung dafür — `bausteinVorrichtung`, `bausteinFS`, `seedWebsite` — wohnt in den `_test`-Dateien von `internal/public` und ist aus `internal/admin` nicht importierbar. Sie nachzubauen hiesse, ein zweites Theme-Gerüst zu unterhalten, das vom ersten wegdriften kann.
- **Fix:** Beide Träger werden über `field.Resolve` aufgelöst und durch **dieselbe** `html/template`-Vorlage gedruckt; behauptet wird, dass keine lebende Marke überlebt **und** dass die zwei Ausgaben übereinstimmen. Das ist genau die Aussage, die der Plan verlangt („assert the two agree"), über genau den Mechanismus, den er benennt. Der Weg bis in ein echtes Theme auf einer echten öffentlichen Adresse steht seit 08-01 in `internal/public/bausteinfelder_test.go#TestBausteinfelderErreichenDasTheme`; der Doc-Kommentar der neuen Prüfung sagt, welche Hälfte wo liegt.
- **Files modified:** `internal/admin/snippet_fields_test.go`
- **Verification:** Mutationsprobe — ein `case KindLong: out[d.Key] = template.HTML(raw)` in `field.Resolve` eingebaut: die Prüfung fällt auf beiden Trägern und druckt die überlebende Marke in den Fehlertext. Danach wiederhergestellt, `git status` sauber, grün.
- **Committed in:** `3cd179c`

### Ein Zähltor misst 2 statt 1 — gemeldet, nicht passend gemacht

| Gatter | Plan | Gemessen | Richtung | Ursache |
|---|---|---|---|---|
| `grep -c 'T-07-26' 07-SECURITY.md` | < 2 | **2** | eine **zu lang** | eine vorbestehende Fremdnennung |
| `grep -c 'field.CheckAll' 07-SECURITY.md` | > 0 | **2** | ✅ | Eintrag + W-1 |
| `grep -c 'func fieldsFromRequest\|func fieldViews\|func groupAction'` | 1 / 2 | **1 / 2** | ✅ | — |
| `grep -c 'template.HTML\|RenderMarkdown('` (ohne Kommentare) | ≤ 1 | **1** | ✅ | die Kette des Rumpfes |
| `grep -c 'field.For('` (ohne Kommentare) | 0 | **0** | ✅ | — |
| `git diff -- internal/field/field.go` | leer | **leer** | ✅ | — |
| `grep -c 'field_top' snippet_list.html` | > 0 | **1** | ✅ | — |
| `grep -c 'name="feld_' snippet_list.html` | 0 | **0** | ✅ | — |
| `grep -ci 'script\|onclick\|onchange\|javascript:'` | 0 | **0** | ✅ | „Skript" im Kommentar trifft nicht |
| `git diff -- cmd/holzcloud/assets/admin.css` | leer | **leer** | ✅ | die Klassen des Seiteneditors reichen |
| `go test -run 'TestTextbaustein\|TestSnippetFeld' -v` | ≥ 7 `--- PASS` | **8** | ✅ | 4 aus 08-03, 4 aus 08-04 |

Zur einen abweichenden Zahl: `07-SECURITY.md:183` steht in **W-4** („Die
Bezeichnungsmenge ist von 12 auf 32 MiB gewachsen") und schreibt „Die Massnahme
zu T-07-26 (Plan 05)". T-07-26 ist aber die Information Disclosure aus **Plan
06** über `field.Hidden`; W-4 redet über die Prägung von Bezeichnungen aus Plan
05. Die Nummer dort ist eine Verwechslung, und sie stand vor diesem Plan schon
da — der Plan hat beim Auszählen von „Eintrag plus Liste = 2" nur zwei der drei
Vorkommen gesehen.

Nicht angefasst: Plan 08-04 verlangt ausdrücklich, keinen anderen Eintrag jenes
Dokuments zu ändern, und welche Nummer in W-4 richtig wäre, liesse sich nur
raten. Die Fremdnennung zu streichen, um das Tor grün zu bekommen, wäre genau
das Passendmachen, das die Lehre der Wellen 1 und 2 verbietet. Notiert in
`.planning/phases/08-snippets-carry-fields/deferred-items.md` und im
Fensterbuch (`.planning/WINDOWS.md`, Eintrag 4), zu prüfen beim nächsten
`/gsd-secure-phase 7`.

---

**Total deviations:** 2 auto-fixed (1 × Regel 1, 1 × Regel 3), dazu eine
gemeldete Gatterzahl.
**Impact on plan:** Kein Zuwachs am Umfang. Abweichung 1 verhindert einen
Bedienfehler, den der Plan nicht vorhergesehen hat, und lässt die vom Plan
verlangte Aufrufreihenfolge unberührt. Abweichung 2 ist eine Frage des Ortes,
nicht der Aussage, und ist mutationsgeprüft. Keine Zahl wurde passend gemacht.

## Issues Encountered

- **`snippetListData` baute die Feldeingaben aus leeren Werten**, weil `HandleSnippetList` die `?edit=`-Vorfüllung erst danach in `data.Values` schrieb. Beim Schreiben aufgefallen und durch Umstellen der Reihenfolge behoben, bevor irgendetwas commitet wurde — kein grüner Lauf hätte es gemeldet, weil erst Task 3 ein wiedergeöffnetes Formular liest.
- **Umlaute in den neuen Kommentaren** waren zuerst in ASCII-Umschrift getippt (`erklaert`, `Praegestelle`) und wurden vor dem Commit auf die Schreibweise des restlichen Baumes gestellt.

## Known Stubs

Keine. Jeder Zweig dieses Plans trägt echte Daten: die Feldeingaben kommen aus
`OfSnippet`, die Werte gehen über `SetFields` in die Datenbank und über
`field.Decode` zurück ins Formular. Kein Platzhaltertext, kein leerer
Vorrat, keine noch nicht verdrahtete Anzeige.

## Threat Flags

Keine neue sicherheitsrelevante Fläche über den `<threat_model>` des Plans
hinaus. Zu den einzelnen Punkten:

- **T-08-18** (ein Feldwert erreicht ein Theme): gemindert. `KindLong` hat in `field.Resolve` keinen eigenen Arm — im Baum nachgesehen und durch die Mutationsprobe belegt —, der Wert fällt in den `default:`-Arm und wird von `html/template` beim Drucken maskiert. Auf dem Feldweg des Textbausteins steht **kein** `template.HTML`-Guss; das Tor misst 1, und der eine Treffer ist die Markdown-Kette des Rumpfes.
- **T-08-18B** (der Rumpf erreicht ein Theme): unverändert. `page.RenderMarkdown` steht genau da, wo es stand, samt seinem Kommentar; dieser Plan hat keine zweite Kette angelegt, und die Prüfung sieht nach, dass `<strong>da</strong>` im gespeicherten HTML steht.
- **T-08-19** (`fieldsFromRequest` auf dem Textbaustein-Formular): gemindert. Der Parser ist der des Seiteneditors, gerufen und nicht abgeschrieben; geprüft und gesäubert wird gegen die Definitionen, die der Server über `OfSnippet` geladen hat.
- **T-08-20** (ein Formularname zweimal geprägt): gemindert. `grep -c 'name="feld_'` misst 0 in `snippet_list.html`, und die Prüfungen bauen ihre Schlüssel aus `field.Def.FieldName` — eine Vorlage, die einen eigenen Namen prägte, fiele durch, statt unter dem falschen Namen grün zu sein.
- **T-08-21** (der Wert eines versteckten Feldes): angenommen wie im Plan, und die Annahme ist jetzt aufgeschrieben — sie ist es, die T-07-26 schliesst.
- **T-08-22** (grosse Nutzlast): unverändert gedeckt. `field.CheckAll` setzt `MaxValueBytes` und `MaxRows` durch, `MaxFields` zählt seit 08-02 auf den Träger.
- **T-08-23** (Spoofing auf die POST-Route): angenommen wie im Plan. Keine neue Route, das Formular trägt den CSRF-Token bereits.
- **T-08-SC**: kein Paketinstallationsschritt in diesem Plan; das Legitimitätstor feuert korrekt nicht. `go.mod` und `go.sum` stehen in keinem Diff.

## User Setup Required

Keine — kein externer Dienst berührt.

## Next Phase Readiness

Bereit für **Plan 08-05**, die letzte Welle: die Browserhälfte, `MinimalData`s
leerer Zwilling für `Bausteinfelder`/`Bausteinliste` samt der Umschreibung des
Satzes „no menus, no labels, no snippets", und die §7-Prosa der
`TEMPLATE-SPEC.md`. Beide Punkte stehen seit 08-01 offen und sind ausdrücklich
diesem Plan zugewiesen.

Was 08-05 vorfindet: ein Formular, das seine Werte trägt, ein Speicherweg mit
denselben fünf Aufrufen wie eine Seite, und acht Prüfungen der Familie
`TestTextbaustein*` / `TestSnippetFeld*`, drei davon mutationsgeprüft.

**Kein Kontrolltor in diesem Plan:** alle drei Aufgaben trugen `type="auto"`, es
gab keinen `checkpoint:*` und damit auch keine gewählte Option zu melden.

Keine Blockierer. `.planning/phases/11-galerie/` ist unberührt geblieben.

---
*Phase: 08-snippets-carry-fields*
*Completed: 2026-09-06*

## Self-Check: PASSED

Alle vier geänderten Quelldateien liegen auf der Platte, und die fünf Commits
`2b61428`, `52a2490`, `ce211da`, `f8c6ffc` und `3cd179c` stehen im Verlauf.
Jedes zugesagte Symbol ist im Baum: `SnippetValues.Fields`,
`SnippetListData.FieldViews`, `(SnippetListData).pool`,
`(*Handler).snippetFieldDefs`, `h.snippets.SetFields` im Speicherweg und
`Fields: field.Decode(sn.Fields)` im `?edit=`-Zweig. `go build ./...`,
`go vet ./...`, `gofmt -l .` und `go test ./...` sind grün, und
`go run ./tools/i18n` meldet für en/es/fr/it je `0 offen, 0 verwaist`.
