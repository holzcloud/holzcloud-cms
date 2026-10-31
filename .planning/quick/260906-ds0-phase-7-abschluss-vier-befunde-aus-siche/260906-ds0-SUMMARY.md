---
phase: quick-260906-ds0
plan: 01
subsystem: api
tags: [go, validation, security, field-kinds, sqlite]

requires:
  - phase: 07-field-kinds
    provides: SplitValues/JoinValues, CheckAll, field.validate, der Archivimportweg
provides:
  - "T-07-02 geschlossen — validKey(d.Key) in validate; ein mitgebrachter Schlüssel wie farbe[] erreicht die Datenbank nicht mehr"
  - "Eine Obergrenze für den Feldschlüssel statt zweier (maxKeyBytes = 40); die still wegfallende Bedingung auf ein langes Feld ist Nebeneffekt-behoben"
  - "W-1 geschlossen — MaxValueBytes gilt auch für ein bedingt verstecktes Feld und für die Zeile einer bedingt versteckten Gruppe, während jede Artregel übersprungen bleibt"
  - "W-3 / WR-10 geschlossen — JoinValues faltet sein Trennzeichen; aus einem Eintrag kann nie ein zweiter Wert werden"
  - "ROADMAP Phase 7 Kriterium 1 liest wahr und trägt einen datierten Änderungsstempel"
affects: [09-import, bundle, field]

actuals:
  tokens: 3400
  tasks: 3
  commits: 7

tech-stack:
  added: []
  patterns:
    - "Eine Grenze, eine Konstante: SlugifyKey und validKey lesen maxKeyBytes, damit die Prüfung nicht ablehnt, was die Ableitung selbst erzeugt hat"
    - "tooLong als eigene Funktion — die Bytegrenze ist keine Frage an die Person vor dem Formular und gilt darum auch für ein verstecktes Feld"
    - "Normalisierung statt Maskierung an der exportierten Kodierung (D-02)"

key-files:
  created: []
  modified:
    - internal/field/field.go
    - internal/field/store.go
    - internal/field/field_test.go
    - internal/field/store_test.go
    - internal/field/values_test.go
    - internal/bundle/bundle_test.go
    - internal/admin/page_form.go
    - .planning/ROADMAP.md

key-decisions:
  - "Die beiden Schlüsselobergrenzen werden auf 40 vereinheitlicht statt auf 30 — nur so wird nichts abgelehnt, was heute angenommen wird (T-Q-01: accept)"
  - "W-1 wird durch Aufteilen des pauschalen continue in CheckAll geschlossen, nicht durch Wiederbeleben der Kürzung (D-13)"
  - "W-3 wird durch Falten des Trennzeichens innerhalb eines Wertes geschlossen — kein Fehlerweg (D-03), kein Maskierungsschema (D-02)"
  - "ROADMAP Kriterium 1 wird gesenkt statt die Datenstruktur umgebaut, weil IMP-08 dasselbe Problem auf der richtigen Ebene löst; die Senkung trägt einen sichtbaren Stempel"

patterns-established:
  - "Ein nachträglich geändertes Erfolgskriterium trägt einen Blockzitat-Stempel mit Datum, Messung, Beleg und Grund (Gestalt aus 06-CONTEXT D-20)"

requirements-completed: [QUICK-260906-ds0, FIELD-07]

coverage:
  - id: D1
    description: "Ein Feld mit dem Schlüssel farbe[] kommt nirgends mehr in die Datenbank; der Archivimport meldet es als Warnung und legt das echte farbe daneben trotzdem an (T-07-02)"
    requirement: FIELD-07
    verification:
      - kind: unit
        ref: "internal/field/store_test.go#TestFeldschluesselWirdAufSeineFormGeprueft"
        status: pass
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestArchivSchluesselMitKlammernWirdAbgelehnt"
        status: pass
    human_judgment: false
  - id: D2
    description: "Eine Beschriftung, deren Kennung 39 Zeichen lang wird, bleibt speicherbar und behält alle 39 Zeichen; eine Bedingung auf ein solches Feld fällt nicht mehr still weg"
    verification:
      - kind: unit
        ref: "internal/field/store_test.go#TestFeldschluesselWirdAufSeineFormGeprueft/eine_39_Zeichen_lange_Kennung_bleibt_speicherbar"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestKennungAusBeschriftung"
        status: pass
    human_judgment: false
  - id: D3
    description: "Der Wert eines bedingt versteckten Feldes und der einer bedingt versteckten Gruppenzeile sind an MaxValueBytes gebunden, während jede Artregel für sie weiterhin übersprungen wird (W-1)"
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestVerstecktesFeldBleibtAnDieBytegrenzeGebunden"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestVersteckteGruppeBleibtAnDieBytegrenzeGebunden"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestVerstecktesMehrwertigesFeldWirdNichtGeprueft"
        status: pass
    human_judgment: false
  - id: D4
    description: "SplitValues(JoinValues(v)) liefert nie mehr Werte, als v nicht-leere Einträge hat (W-3 / WR-10)"
    verification:
      - kind: unit
        ref: "internal/field/values_test.go#TestJoinValuesVerteidigtSeinTrennzeichen"
        status: pass
      - kind: unit
        ref: "internal/field/values_test.go#TestValuesRundreise"
        status: pass
    human_judgment: false
  - id: D5
    description: "ROADMAP Phase 7 Kriterium 1 liest wahr und trägt den datierten Änderungsstempel; page_form.go nennt die Grenze der Unterscheidung"
    verification:
      - kind: other
        ref: "grep -c 'Amended 2026-09-06' .planning/ROADMAP.md → 1"
        status: pass
    human_judgment: true
    rationale: "Ob der gesenkte Satz die Sache ehrlich fasst und der Stempel Grund und Beleg wirklich trägt, ist eine Leseentscheidung — grep zählt nur, dass er da ist."

duration: 24 min
completed: 2026-09-06
status: complete
---

# Quick 260906-ds0: Phase-7-Abschluss — vier Befunde aus der Sicherheitsprüfung Summary

**Der Feldschlüssel wird auf seine Form geprüft, die Bytegrenze gilt auch für ein Feld, das niemand sieht, JoinValues verteidigt sein eigenes Trennzeichen — und ein nachträglich gesenktes Erfolgskriterium sagt offen, dass es gesenkt wurde.**

## Performance

- **Duration:** 24 min
- **Tasks:** 3
- **Files modified:** 8
- **Commits:** 7 (3 × `test`, 3 × `fix`, 1 × `docs`)

## Accomplishments

- **T-07-02 geschlossen.** `validate` prüft jetzt `validKey(d.Key)`. Der einzige Weg, auf dem ein Schlüssel mitgebracht statt abgeleitet wird, ist der Archivweg (`internal/bundle/import.go:351`) — dort wird die Ablehnung zur Warnung im Bericht statt zum Abbruch des Imports.
- **Eine Obergrenze statt zweier.** `maxKeyBytes = 40` in `field.go`; `SlugifyKey` und `validKey` lesen dieselbe Zahl. Nebeneffekt: `validKey(d.Condition)` (`store.go`) löscht nicht mehr wortlos eine Bedingung, die auf ein Feld mit 31 bis 40 Zeichen langer Kennung zeigt.
- **W-1 geschlossen.** `tooLong` aus `Check` gehoben; `CheckAll` teilt den pauschalen `continue` auf, sodass ein bedingt verstecktes Feld — und jede Zeile einer bedingt versteckten Gruppe — noch durch die Bytegrenze läuft, aber durch keine einzige Artregel.
- **W-3 / WR-10 geschlossen.** `JoinValues` faltet jede Zeilenschaltung innerhalb eines Eintrags zu einem Leerzeichen. Die Zahl der zurückgelesenen Werte kann die Zahl der übergebenen nicht mehr übersteigen — die Zusage, die Phase 9s CSV-Importeur erbt (D-02).
- **Zwei Notizen lesen wieder wahr.** `page_form.go` nennt die Grenze der Unterscheidung geleert/nie-getragen und wer sie ab dort trägt; ROADMAP Phase 7 Kriterium 1 ist ehrlich gefasst und trägt einen datierten Änderungsstempel mit Messung, Beleg und Grund.

## Task Commits

1. **Task 1 (RED):** `a5194c6` — `test(quick-260906-ds0-01)`: der Feldschlüssel ohne Formprüfung
2. **Task 1 (GREEN):** `d3ecbcd` — `fix(quick-260906-ds0-01)`: der Feldschlüssel wird auf seine Form geprüft (T-07-02)
3. **Task 2 (RED):** `4656f76` — `test(quick-260906-ds0-01)`: das Loch in der Bytegrenze am versteckten Feld
4. **Task 2 (GREEN):** `7c3c54b` — `fix(quick-260906-ds0-01)`: die Bytegrenze gilt auch für ein verstecktes Feld (W-1)
5. **Task 3 (RED):** `19b11cc` — `test(quick-260906-ds0-01)`: JoinValues macht aus einem Eintrag zwei Werte
6. **Task 3 (GREEN):** `d5feaa1` — `fix(quick-260906-ds0-01)`: JoinValues verteidigt sein Trennzeichen (W-3)
7. **Task 3 (Notizen):** `85843f2` — `docs(quick-260906-ds0-01)`: zwei Notizen werden wieder wahr

Die Notizen sind ein eigener Docscommit, getrennt vom Codecommit — dieselbe Gestalt, die D-01 für die Grundwahrheiten vorgibt.

## Die fünf neuen Tests: der beobachtete Fehlschlag, wörtlich

Jeder Test wurde gegen den Baum **vor** seiner Codeänderung gefahren. Was dabei zu sehen war:

**1. `TestFeldschluesselWirdAufSeineFormGeprueft` (`internal/field/store_test.go`)**

```
--- FAIL: TestFeldschluesselWirdAufSeineFormGeprueft/Klammern_im_Schlüssel_werden_abgelehnt
    store_test.go:458: ein Schlüssel mit Klammern wurde angenommen
--- FAIL: TestFeldschluesselWirdAufSeineFormGeprueft/Grossbuchstaben_und_Punkte_werden_abgelehnt
    store_test.go:480: der Schlüssel "Farbe" wurde angenommen
    store_test.go:480: der Schlüssel "farbe.ton" wurde angenommen
    store_test.go:480: der Schlüssel "farbe-ton" wurde angenommen
    store_test.go:480: der Schlüssel "farbe ton" wurde angenommen
    store_test.go:480: der Schlüssel "fär be" wurde angenommen
--- FAIL: TestFeldschluesselWirdAufSeineFormGeprueft/eine_Bedingung_auf_ein_langes_Feld_fällt_nicht_mehr_still_weg
    store_test.go:516: Bedingung = "", wollte "sehr_langer_name_der_weit_ueber_vierzig"
```

Bemerkenswert am dritten Fehlschlag: er ist der latente Defekt, den der Plan benannt hat, und er war **vor** der Änderung tatsächlich rot. `validKey(d.Condition)` an `store.go:534` löschte die Bedingung wortlos, weil sie 39 Zeichen lang ist und `validKey` bei 30 abschnitt. Die Vereinheitlichung der Obergrenze schliesst ihn als Nebeneffekt — das ist gemessen, nicht angenommen.

Ebenso bemerkenswert: der Unterfall „eine 39 Zeichen lange Kennung bleibt speicherbar" war schon **vor** der Änderung grün. Er ist die Gegenprobe und musste grün bleiben, nicht grün werden — heute prüfte `validate` `d.Key` überhaupt nicht, also gab es nichts, woran eine 39-Zeichen-Kennung hätte scheitern können. Rot geworden wäre er erst durch einen naiven `validKey(d.Key)` ohne die Vereinheitlichung. Genau davor stand er Wache, und er hat nie geschlagen.

**2. `TestArchivSchluesselMitKlammernWirdAbgelehnt` (`internal/bundle/bundle_test.go`)**

```
--- FAIL: TestArchivSchluesselMitKlammernWirdAbgelehnt
    bundle_test.go:1511: angelegte Felder = [farbe[] farbe], wollte nur farbe
```

Beide Felder standen in der Datenbank, und der Bericht meldete nichts.

**3. `TestVerstecktesFeldBleibtAnDieBytegrenzeGebunden` (`internal/field/field_test.go`)**

```
--- FAIL: TestVerstecktesFeldBleibtAnDieBytegrenzeGebunden
    field_test.go:1190: der zu lange Wert eines versteckten Feldes wurde nicht gemeldet: map[]
```

`map[]` — die Fehlerkarte war leer, wie der Plan es vorhergesagt hat.

**4. `TestVersteckteGruppeBleibtAnDieBytegrenzeGebunden` (`internal/field/field_test.go`)**

```
--- FAIL: TestVersteckteGruppeBleibtAnDieBytegrenzeGebunden
    field_test.go:1229: der zu lange Wert in der Zeile einer versteckten Gruppe wurde nicht gemeldet: map[]
```

Dasselbe Loch eine Ebene tiefer, ebenfalls `map[]`.

**5. `TestJoinValuesVerteidigtSeinTrennzeichen` (`internal/field/values_test.go`)**

```
--- FAIL: TestJoinValuesVerteidigtSeinTrennzeichen
    values_test.go:226: aus zwei Einträgen wurden 3 Werte: []string{"a", "b", "c"}
```

Wörtlich der Befund aus W-3: zwei übergebene Einträge, drei zurückgelesene Werte.

## Der eine angefasste bestehende Test, und warum das keine Abschwächung ist

`TestVerstecktesMehrwertigesFeldWirdNichtGeprueft` (`field_test.go:978`) baute seine Vorlage aus
`JoinValues([]string{"Eiche", "Buche", strings.Repeat("x", MaxValueBytes)})` — ein Wert, der drei
Regeln auf einmal bricht: zu viele Werte bei `MaxValues: 1`, eine Option, die nicht auf der Liste
steht, **und** die Bytegrenze. Danach behauptete er `len(aus) == 0`.

**Was er schützte:** die Arthälfte. Von einer Person etwas zu verlangen, das sie nicht sehen kann,
ist der eine Weg, auf dem sich ein Formular nicht abschicken lässt, ohne zu sagen warum. Diese
Hälfte bleibt richtig und bleibt bewacht.

**Warum er im Weg stand:** die Bytehälfte war Beifang. Nach der Behebung wäre er rot geworden — aus
dem falschen Grund, nämlich weil die Bytegrenze jetzt korrekt feuert, und nicht weil eine Artregel
durchgesickert wäre.

**Was gemacht wurde:** die Vorlage wurde auf `JoinValues([]string{"Eiche", "Buche", "Ahorn"})`
**verengt**. Immer noch drei Werte bei `MaxValues: 1`, immer noch mit einer Option, die nicht auf
der Liste steht, immer noch `Required` — aber deutlich unter 4000 Byte.

**Warum das eine Verstärkung ist:** kein Fall wurde gestrichen, keine Zusicherung entfernt, keine
Behauptung abgeschwächt. Die Aussage wird *präziser*: ein Fehler unter dieser Vorlage bedeutet ab
jetzt eindeutig, dass eine **Artregel** gefeuert hat — vorher konnte es auch die Länge gewesen sein,
und der Test hätte eine zurückgeschmuggelte Artregel gar nicht von einer korrekt greifenden
Bytegrenze unterscheiden können. Die Bytehälfte wurde nicht entfernt, sondern in zwei eigene Tests
gehoben, die sie schärfer prüfen als die alte Vermengung es konnte. Der Doc-Kommentar des Tests sagt
jetzt ausdrücklich, wo die Grenze liegt: jede Artregel übersprungen, die Bytegrenze nicht.

`TestNichtsWirdMehrStillGekuerzt` und `TestKennungAusBeschriftung` wurden **nicht** angefasst und
sind beide unverändert grün. Kein Test wurde gelöscht.

## Gelockte Entscheidungen: keine überstimmt

- **D-02** — kein Maskierungsschema, keine zweite Kodierung. Die Faltung ist dieselbe Klasse
  Normalisierung, die `JoinValues` mit dem Trimmen und dem Wegfallen leerer Einträge ohnehin schon
  macht. `SplitValues` ist unangetastet, der `KindMulti`-Zweig von `Check` ebenso.
- **D-03** — `JoinValues` bekam keinen Fehlerweg. `fieldsFromRequest` läuft weiterhin per Präfix und
  **bevor** die Definitionen geladen sind; diese Eigenschaft ist unberührt.
- **D-13** — die Kürzung wurde nicht wiederbelebt. `trimTo` ist weiterhin ein reines `TrimSpace`,
  `tooLong` **meldet** nur. `TestNichtsWirdMehrStillGekuerzt` ist grün.
- **D-11** — der versteckte Wächter bleibt: leere Einträge fallen in `JoinValues` weiterhin weg, und
  `TestJoinValuesWaechterUndDoppelte` ist unverändert grün.

## Entscheidungen

- **Die Obergrenze wird auf 40 vereinheitlicht, nicht auf 30.** Der umgekehrte Weg hätte
  `TestKennungAusBeschriftung` gebrochen und jede Beschriftung unspeicherbar gemacht, deren Kennung
  zwischen 31 und 40 Zeichen lang wird. Ausschliesslich eine Erweiterung des Längenfensters bei
  unverändertem Zeichensatz `[a-z0-9_]` — genau die Verfügung, die der Plan als T-Q-01 mit
  `accept` eingetragen hat. `validKey` gilt auch für `d.Condition` und `d.AppliesTo`; auch dort ist
  die Änderung reine Erweiterung.
- **`faltZeilen` ist ein `strings.NewReplacer` mit `\r\n` an erster Stelle**, damit die
  Wagenrücklaufform ein Leerzeichen ergibt und nicht zwei. Ausdrücklich **keine** Funktion, die
  jeden Weissraum zusammenfasst — zwei Leerzeichen innerhalb eines gültigen Wertes bleiben zwei
  Leerzeichen, und `TestJoinValues` hätte das Gegenteil sofort gemeldet.
- **Die Ablehnung in `validate` steht nach der Ableitung**, damit der leere Fall seine eigene,
  hilfreichere Begründung behält.

## Deviations from Plan

None — plan executed exactly as written.

Der Plan hat drei Dinge vorhergesagt, die genau so eingetroffen sind: die leere Fehlerkarte `map[]`
bei den beiden versteckten Fällen, die drei statt zwei Werte bei `JoinValues`, und den latenten
Defekt der still wegfallenden Bedingung. Der einzige Punkt, an dem die Wirklichkeit vom Plantext
abwich, ist harmlos und oben bereits festgehalten: der 39-Zeichen-Unterfall war schon vor der
Änderung grün, weil `validate` `d.Key` bisher überhaupt nicht prüfte. Er ist eine Gegenprobe, kein
RED-Kandidat, und der Plan hat ihn auch als „Gegenprobe, die nicht rot werden darf" beschrieben.

## Issues Encountered

None.

## Standing Gate

Aus dem Projektwurzelverzeichnis, nach allen drei Aufgaben:

| Tor | Ergebnis |
|---|---|
| `go build ./...` | sauber |
| `go test ./...` | alle Pakete grün |
| `gofmt -l .` | still |
| `go vet ./...` | still |
| `grep -c 'Amended 2026-09-06' .planning/ROADMAP.md` | `1` |

**i18n:** `go run ./tools/i18n` meldet `1151 Zeichenketten im Quelltext` und für `en.json`,
`es.json`, `fr.json`, `it.json` jeweils `1151 übersetzt, 0 offen, 0 verwaist`. **Der Zähler hat
nichts Neues gesehen** — genau wie erwartet und im Plan begründet: die neue Ablehnung ist ein
`errors.New` in `internal/field`, das der Adminhandler als `err.Error()` weiterreicht, und
`tools/i18n` liest nur Literale an benannten Argumentstellen. Der wiederverwendete Wortlaut von
`tooLong` ist wörtlich aus `Check` gewandert, also auch dort keine neue Zeichenkette. **Es war kein
Katalogcommit nötig**, `-write`/`-schweiz` wurden nicht gefahren. Die `-CH`-Dateien zeigen
unverändert ihre bekannten Abweichungen (54 / 4 / 9) und `0 ohne Gegenstück`.

**Keine Migration.** `internal/db/migrations/` ist unberührt; diese Aufgabe ändert kein Schema.

**Keine neue Abhängigkeit.** `go.mod`/`go.sum` sind unberührt, jedes benutzte Symbol ist
Standardbibliothek. Das Paket-Legitimitätstor feuert korrekt nicht (T-07-SC).

## Was damit für Phase 7 offen bleibt

`07-SECURITY.md` führt vier nächste Schritte. Diese Aufgabe hat Schritt 1 (T-07-02) und Schritt 3
(W-3 zusammen mit Verifikations-Gap 1) erledigt und W-1 aus der Verifikation gleich mit.

**Schritt 2 — T-07-26 (Plan 06), „den Mechanismus bauen oder die Behauptung auf das zurückschreiben,
was wirklich schützt" — ist NICHT Teil dieser Aufgabe und bleibt offen.** Die Massnahmenformulierung
zu T-07-26 wurde nicht angefasst.

**Schritt 4 ist damit erreichbar:** T-07-02, W-1 und W-3 sind geschlossen, `/gsd-secure-phase 7` kann
erneut gefahren werden. Ob T-07-26 vorher entschieden sein sollte, ist eine Frage an die
Sicherheitsprüfung selbst — die Reihenfolge in `07-SECURITY.md` legt es nahe.

**Das Häkchen von Phase 7 wurde nicht gesetzt und der Status nicht angefasst** — das ist die
Entscheidung von `/gsd-verify-work`, nicht dieser Aufgabe.

## Next Phase Readiness

Phase 9s CSV-Importeur erbt `SplitValues`/`JoinValues` (D-02) jetzt mit einer durchgesetzten statt
einer nur behaupteten Prämisse: ein Eintrag kann keinen zweiten Wert prägen, auch ohne geschlossene
Möglichkeitenliste. Der Feldschlüssel aus einer fremden Datei ist auf seine Form geprüft. Beide
Grenzen sitzen an der einen exportierten Stelle, nicht beim Aufrufer.

## Self-Check: PASSED

- Alle sieben Commits im Verlauf gefunden: `a5194c6`, `d3ecbcd`, `4656f76`, `7c3c54b`, `19b11cc`, `d5feaa1`, `85843f2`.
- Alle acht geänderten Dateien auf der Platte gefunden.
- Alle fünf neuen Testfunktionen im Baum gefunden.
- Arbeitsverzeichnis sauber ausser den zwei Planungsordnern, die der Orchestrator festschreibt (`.planning/quick/260906-ds0-…/`) bzw. die einem anderen Ablauf gehören und nicht angefasst wurden (`.planning/phases/11-galerie/`).

---
*Quick task: 260906-ds0*
*Completed: 2026-09-06*
