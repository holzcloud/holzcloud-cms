---
phase: 07-field-kinds
plan: 05
subsystem: content-model
tags: [go, sqlite, htmx, terms, field-kinds, bundle, html-template]

requires:
  - phase: 07-field-kinds
    provides: "Die vier Arten aus 07-01..07-04 und die geteilten Dateien field.go, render.go, field_input.html in dem Zustand, in dem dieser Plan sie vorfand"
  - phase: 05-schlagworte
    provides: "internal/term — Term, ListAll, Rename, SetForPage, Parse und die Archivadresse /tag/<kuerzel>"
provides:
  - "KindTerm = \"schlagwort\": speichert das Kuerzel, druckt den Namen"
  - "field.Term, field.TermLookup und Links.Term — die Aufloesung samt der Websiteregel"
  - "BlockKinds() laesst Verweis und Schlagwort aus, mit dem Kapazitaetshinweis auf vier"
  - "admin.TermChoice, pool.terms, PageFormData.RefTerms und Handler.siteTerms — der Waehler"
  - "public.Handler.fieldTerms — eine Abfrage je Seite, auf eine Website begrenzt"
  - "term.Store.EnsureNames — legt Schlagwoerter an, ohne sie an etwas zu haengen"
  - "bundle: KindTerm-Arme in translateOut/translateIn, importTerms, report.Terms zaehlt Angelegtes"
  - "internal/bundle/import.go schreibt Feldwerte nur noch durch field.Clean und field.CheckAll"
affects: [07-06, 07-07, 09-csv-import, 11-galerie]

actuals:
  tokens: 16467
  tasks: 4
  commits: 7

tech-stack:
  added: []
  patterns:
    - "Eine Art, deren Wert einer spaeteren Umbenennung folgen muss, wird aus BlockKinds() ausgeschlossen — der Filter ist abziehend, der Ausschluss ist die einzige Stelle"
    - "Die Websiteregel einer nachgeschlagenen Art steht in der Nachschlagefunktion, nicht in Check: ein gespeicherter Wert sagt nichts darueber, welcher Website er gehoert"
    - "Ein Wert, dessen Identitaet auf der anderen Maschine nichts bedeutet, reist als seine uebertragbare Schreibweise und wird bei der Ankunft zurueckaufgeloest — dieselbe Form wie KindRef und KindImage"

key-files:
  created: []
  modified:
    - internal/field/field.go
    - internal/field/render.go
    - internal/field/field_test.go
    - internal/admin/page_fields.go
    - internal/admin/page.go
    - internal/admin/page_fields_kinds_test.go
    - internal/public/pagedata.go
    - internal/public/handler_test.go
    - cmd/holzcloud/templates/admin/field_input.html
    - internal/term/store.go
    - internal/bundle/export.go
    - internal/bundle/import.go
    - internal/bundle/bundle_test.go

key-decisions:
  - "Check misst mit page.Slugify(value) != value und nicht mit page.ValidateSlug: ValidateSlug traegt reservedSlugs, eine Reservierung der Router-Pfade, die fuer ein Schlagwort unter /tag/ nicht gilt — ein Schlagwort namens „admin“ ist dort erreichbar und darf nicht abgelehnt werden"
  - "internal/field importiert dafuer erstmals internal/page; zyklenfrei, weil internal/page nur auth und db zieht und keines davon field kennt"
  - "Der offene Blocker aus 07-04 ist ENTSCHIEDEN und behoben: importFieldValues laeuft jetzt durch field.Clean und field.CheckAll"
  - "Ein vom Import verworfener Wert wird benannt und nie die ganze Seite weggeworfen; gemeldet nur im ersten Durchgang, nicht noch einmal im Verweisdurchgang"
  - "report.Terms zaehlt angelegte Schlagwoerter statt der vom Manifest behaupteten — zwei verschiedene Tatsachen unter einer Zahl"
  - "Das Manifestformat blieb unangetastet: kein Schluessel kam dazu, keiner wurde umbenannt (format.go steht nicht in der Dateiliste dieses Plans)"
  - "siteTerms ist unbegrenzt, waehrend refPages bei 500 kappt: ListAll ist schon auf eine Website begrenzt, und ein weggefallenes Schlagwort waere ein Wert, den der Editor nicht mehr sieht"

patterns-established:
  - "Kapazitaetshinweis als Zaehlwerk: len(Kinds)-N in BlockKinds() traegt die Zahl der Ausschluesse und wird mit ihnen geaendert; ein Tor prueft, dass -3 nicht stehenbleibt"
  - "Der beweisende Test benennt vor dem Export um — ein Schlagwort, dessen Name noch zu seinem Kuerzel passt, reist auch ohne die Uebersetzung heil und bewiese nichts"

requirements-completed: [FIELD-03]

coverage:
  - id: D1
    description: "KindTerm ist eine Art, steht in SubKinds und fehlt in BlockKinds; der Kapazitaetshinweis zaehlt vier Ausschluesse und code bleibt in einem Baustein erlaubt"
    requirement: "FIELD-03"
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestSchlagwortStehtNichtInBausteinarten"
        status: pass
      - kind: other
        ref: "grep -n 'len(Kinds)-3' internal/field/field.go (kein Treffer)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Ein Schlagwortfeld loest ein gespeichertes Kuerzel zum aktuellen Namen auf, oder zu einem getippten nil — leer, ohne Nachschlagefunktion, geloescht, andere Schreibung"
    requirement: "FIELD-03"
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestSchlagwortWirdAufgeloest"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestSchlagwortOhneTrefferWirdNil"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestSchlagwortStehtMitNamenInDerListe"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestSchlagwortPruefung"
        status: pass
    human_judgment: false
  - id: D3
    description: "Der Seiteneditor bietet die Schlagwoerter dieser Website als Auswahl, zeigt das gespeicherte vorausgewaehlt und nennt keines einer fremden Website (T-07-20)"
    requirement: "FIELD-03"
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_kinds_test.go#TestSchlagwortfeldImSeiteneditor"
        status: pass
      - kind: unit
        ref: "internal/admin/page_fields_kinds_test.go#TestSchlagwortauswahlScheitertLeise"
        status: pass
    human_judgment: false
  - id: D4
    description: "Die oeffentliche Seite druckt den Namen; eine Umbenennung aendert, was sie zeigt, ohne Schreibzugriff auf die Seite; ein geloeschtes Schlagwort druckt nichts und die Seite bleibt stehen"
    requirement: "FIELD-03"
    verification:
      - kind: integration
        ref: "internal/public/handler_test.go#TestSchlagwortfeldDrucktDenAktuellenNamen"
        status: pass
      - kind: integration
        ref: "internal/public/handler_test.go#TestSchlagwortfeldOhneSchlagwortBleibtLeer"
        status: pass
    human_judgment: false
  - id: D5
    description: "Ein Schlagwort einer fremden Website loest sich beim Rendern zu nichts auf (T-07-19)"
    requirement: "FIELD-03"
    verification:
      - kind: integration
        ref: "internal/public/handler_test.go#TestSchlagwortfeldErreichtKeineFremdeWebsite"
        status: pass
    human_judgment: false
  - id: D6
    description: "Der Wert eines Schlagwortfeldes ueberlebt die Archivreise, auch wenn das Schlagwort vorher umbenannt wurde, und ein nur vom Feld getragenes Schlagwort wird beim Import angelegt statt gezaehlt"
    requirement: "FIELD-03"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestSchlagwortfeldRundreise/umbenannt_und_an_keiner_Seite"
        status: pass
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestSchlagwortfeldRundreise/auch_an_einer_Seite:_nur_einmal_angelegt"
        status: pass
    human_judgment: false
  - id: D7
    description: "Archivwerte gehen durch dieselbe Pruefung wie Formularwerte: ein Wert ueber dem Bytebudget, einer ausserhalb einer Auswahlliste und einer ohne Felddefinition erreichen den Speicher nicht mehr, und der Bericht nennt sie"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestArchivwerteGehenDurchDieselbePruefung"
        status: pass
    human_judgment: false
  - id: D8
    description: "Das Auswahlfeld sieht im Browser richtig aus und laesst sich ohne JavaScript bedienen — Aussehen, Abstaende, Verhalten der Auswahl neben den uebrigen Feldern"
    verification: []
    human_judgment: true
    rationale: "Die Auszeichnung ist geprueft, das Bild nicht. Die Browserhaelfte dieser Phase ist planmaessig 07-07 (STATE.md); ein Menschenblick auf das gerenderte Formular kann kein Test ersetzen."

duration: 19 min
completed: 2026-09-05
status: complete
---

# Phase 07 Plan 05: Terms als Feldart Summary

**`KindTerm` speichert das Kürzel eines Schlagworts und druckt dessen aktuellen Namen — eine Umbenennung ändert, was jede Seite zeigt, ohne dass eine Seite angefasst wird, und der Wert überlebt jetzt auch die Archivreise, weil er als Name reist statt als Kürzel.**

## Performance

- **Duration:** 19 min
- **Started:** 2026-09-05T16:55Z (Lesephase), erster Commit 17:01:29+02:00
- **Completed:** 2026-09-05T17:13:41+02:00 (+ ein Nachtrag 17:16)
- **Tasks:** 4
- **Files modified:** 13

## Accomplishments

- **`KindTerm` als vollständige Art** — Konstante, `Kinds`-Zeile, `Check`-Fall, Auflösung (`Term`, `TermLookup`, `Links.Term`, `Entry.Term`), `List`, `Filled`. `KindRef` war die Vorlage, und der eine Unterschied ist der ganze Punkt: gespeichert wird ein Kürzel, kein Bezeichner, also kein `ParseInt`.
- **Der eine Ausschluss (D-06)** — `BlockKinds()` lässt jetzt Gruppe, Abschnitt, Verweis **und** Schlagwort aus, und der Kapazitätshinweis zählt vier statt drei. `code` bleibt in einem Baustein erlaubt; ein Test hält beide Hälften fest.
- **Der Wähler im Seiteneditor** — `TermChoice`, `pool.terms`, `FieldView.Terms`, `oneView` und `siteTerms`, dazu der `schlagwort`-Zweig in `field_input.html`. Kein Aufrufer von `pool()` musste angefasst werden — genau das, wofür der Doc-Kommentar von `pool` gebaut war.
- **Die öffentliche Auflösung** — `fieldTerms` füllt seine Karte einmal je Seite aus einem `ListAll` genau der Website, die gerendert wird. Dieser eine Aufruf **ist** die Websiteregel; ein fremdes Kürzel steht schlicht nicht in der Karte.
- **Die Archivreise repariert** — der Wert reist als **Name**, der Import leitet das Kürzel mit `page.Slugify` selbst ab, und `importTerms` legt jedes Schlagwort an, das ein Manifest nennt. Das Manifestformat blieb dabei unverändert.
- **Der offene Blocker aus 07-04 geschlossen** — `importFieldValues` läuft jetzt durch `field.Clean` und `field.CheckAll`.

## Task Commits

1. **Task 1 (RED): das Schlagwortfeld, bevor es die Art gibt** — `fac7504` (test)
2. **Task 1 (GREEN): das Schlagwortfeld als Art, mit seinem einen Ausschluss** — `e6e86af` (feat)
3. **Task 2: der Schlagwortwähler im Seiteneditor** — `d1eb9b1` (feat)
4. **Task 3: die öffentliche Seite druckt den aktuellen Namen** — `38319a5` (feat)
5. **Task 4 (RED): die Rundreise eines Schlagwortfeldes, vor der Übersetzung** — `52889b1` (test)
6. **Task 4 (GREEN): das Schlagwortfeld überlebt die Archivreise** — `7cb09f4` (feat)
7. **Nachtrag: ein fremdes Schlagwort löst sich zu nichts auf (T-07-19)** — `052de1d` (test)

_Die beiden mit `tdd="true"` ausgezeichneten Tasks tragen je ein test- und ein feat-Commit; ein refactor war in keinem der beiden nötig._

## Files Created/Modified

- `internal/field/field.go` — `KindTerm`, seine `Kinds`-Zeile, der `Check`-Fall, die Erweiterung von `BlockKinds()` samt Kapazitätshinweis und neuem Doc-Kommentar; neuer Import von `internal/page`
- `internal/field/render.go` — `Term`, `TermLookup`, `Links.Term`, `Entry.Term`, `case KindTerm` in `Resolve`, `case *Term` in `List` und `Filled`
- `internal/field/field_test.go` — fünf deutsch benannte Tests, darunter die vier Wege ins Nichts und die Ausschluss-Bilanz von `BlockKinds()`
- `internal/admin/page_fields.go` — `TermChoice`, `pool.terms`, `FieldView.Terms`, `case field.KindTerm` in `oneView`, `siteTerms`
- `internal/admin/page.go` — `PageFormData.RefTerms` und sein Füllen neben `RefPages`
- `internal/admin/page_fields_kinds_test.go` — der Wähler im ausgelieferten HTML und die beiden leisen Fehlschläge von `siteTerms`
- `cmd/holzcloud/templates/admin/field_input.html` — der `schlagwort`-Zweig: Name als Text, Kürzel als Wert, `{{t}}` auf jeder sichtbaren Zeichenkette
- `internal/public/pagedata.go` — `fieldTerms` und das `Term:` in den `field.Links` von `ownFields`
- `internal/public/handler_test.go` — Umbenennung, Löschung, fremde Website; dazu `newFieldTestHandler` mit einer Vorlage, deren `{{with}}` die leere Kante überhaupt prüfbar macht
- `internal/term/store.go` — `EnsureNames`
- `internal/bundle/export.go` — `exportTerms` gibt eine Namenskarte zurück und läuft vor `exportPages`; der `KindTerm`-Arm in `translateOut`
- `internal/bundle/import.go` — `importTerms`, der `KindTerm`-Arm in `translateIn`, `report.Terms` an genau einer Stelle, und die beiden Wachen in `importFieldValues` samt `splitRowKey`
- `internal/bundle/bundle_test.go` — die Rundreise mit zwei Untertests und die Prüfung des Importwegs

## Decisions Made

**1. `Check` misst mit `page.Slugify(value) != value`, nicht mit `page.ValidateSlug`.**
Der Plan verlangte, die bestehende Kürzelregel wiederzuverwenden statt eine zweite Schreibweise zu erfinden. `ValidateSlug` wäre der naheliegende Griff und ist der falsche: es trägt `reservedSlugs`, eine Reservierung der Router-Pfade. Ein Schlagwort heißt sein Archiv `/tag/admin` und ist dort völlig erreichbar — `ValidateSlug` hätte es abgelehnt. `Slugify(v) == v` ist dagegen genau die Frage, die hier gestellt wird: ist das schon die normalisierte Schreibweise.

**2. `internal/field` importiert erstmals `internal/page`.**
Für 1 nötig. Vorher geprüft: `internal/page` zieht nur `internal/auth` und `internal/db`, und keines von beiden kennt `internal/field` — kein Zyklus. Es ist die erste Kante von der Modellschicht in die Seitenschicht und deshalb hier festgehalten, nicht weil sie problematisch wäre, sondern weil eine zweite ohne diesen Eintrag unbemerkt entstünde.

**3. Der Wert eines Schlagwortfeldes reist als Name, das Manifestformat bleibt unangetastet.**
`format.go:152–159` und `:274–284` sind als Entscheidung gelesen worden, nicht als Beschreibung: eine Beschriftung überquert das Manifest als **Name**, der Import leitet das Kürzel selbst ab, und daran zu drehen „würde die Bedeutung jedes bereits ausgehändigten Archivs ändern“. `format.go` steht deshalb nicht in der Dateiliste dieses Plans und wurde nicht angefasst — `git status` bezeugt es.

**4. Die Adresse einer Beschriftung darf sich über eine Rundreise bewegen, das gemeinte Schlagwort nicht.**
Ein Quellkürzel `moebel` kommt als `moebelbau` an, weil das Format die Adresse aus dem Namen ableitet. Der Test sagt das in einem Kommentar ausdrücklich, damit ein späterer Leser den Unterschied nicht als Fehler meldet.

**5. `report.Terms` zählt Angelegtes, nicht Behauptetes.**
Vorher `len(m.Terms)` in `importPages`, also die Zahl, die das Manifest nennt. Jetzt der Rückgabewert von `EnsureNames`, an genau einer Stelle gesetzt. Ein Bericht ist dazu da, geglaubt zu werden.

**6. `siteTerms` bleibt unbegrenzt, während `refPages` bei 500 kappt.**
Vom Plan so verlangt und hier bestätigt: `ListAll` ist schon auf eine Website begrenzt, eine Schlagwortliste ist keine Seitenliste, und ein abgeschnittenes Schlagwort wäre ein gespeicherter Wert, den der Editor nicht mehr anzeigen könnte.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 — Missing Critical] `importFieldValues` schrieb Feldwerte ohne `field.Clean` und ohne `field.CheckAll`**

- **Found during:** Task 4 (der Auftrag verlangte ausdrücklich eine Entscheidung zu diesem offenen Blocker aus 07-04)
- **Issue:** `internal/bundle/import.go` legte mit `field.Encode` ab, was im Manifest stand. Ein Archiv ist eine Datei, die jeder bearbeiten kann — genauso unvertraut wie ein Formularfeld —, und alle anderen Schreibwege sind gedeckt (`internal/admin/page.go:434`/`:597` über `checkFields`, `internal/ai/tools.go:670` über `CheckAll`). Seit 07-04 kürzt `trimTo` nicht mehr, `CheckAll` ist damit die **einzige** verbliebene Stelle, an der das Bytebudget überhaupt noch gilt. Vorbestehend, von diesem Plan nicht verursacht.
- **Decision:** **Gedeckt, nicht vertagt.** Rule 2: fehlende Eingabeprüfung an einer Vertrauensgrenze. `internal/bundle/format.go`s eigener Nachbar-Test sagt denselben Satz über die Designtokens — „ein Archiv ist genauso unvertraut wie ein Formularfeld und geht durch denselben Prüfer“ —, und der Feldwertweg war der eine, für den das nicht galt.
- **Fix:** `importPages` liest die tatsächlich angelegten Definitionen über `s.Fields.List` (gelesen, nicht aus dem Manifest nachgebaut, damit die Prüfung gegen das läuft, was diese Website hat, samt allem, was `validate` beim Anlegen geleert hat) und reicht sie in `importFieldValues`. Dort laufen `field.Clean` und dann `field.CheckAll`; ein beanstandeter Wert wird entfernt und namentlich in den Bericht geschrieben, nie die ganze Seite verworfen. Neues `splitRowKey` liest zurück, was `field.RowKey` schreibt, damit auch ein Wert in einer Gruppenzeile entfernt werden kann. Der Verweisdurchgang meldet nicht ein zweites Mal.
- **Files modified:** `internal/bundle/import.go`, `internal/bundle/bundle_test.go`
- **Verification:** `TestArchivwerteGehenDurchDieselbePruefung` — ein Wert über `MaxValueBytes`, einer außerhalb einer Auswahlliste und einer ohne Felddefinition erreichen den Speicher nicht mehr, ein gültiger daneben kommt an, und der Bericht nennt die verworfenen. Gegenprobe geführt: mit deaktivierter Wache schlägt der Test in allen vier Behauptungen fehl (4001 Byte abgelegt, „Zement“ abgelegt, Feld ohne Definition abgelegt, Bericht leer). Sämtliche bestehenden Rundreisen bleiben grün — die Wache wirft nichts Rechtmäßiges weg.
- **Committed in:** `7cb09f4`

**2. [Rule 2 — Missing Critical] Die Minderung T-07-19 war im Bedrohungsmodell benannt und unbewiesen**

- **Found during:** Abschlussprüfung gegen `<verification>` („No other website's term is reachable from either the chooser or the resolver“)
- **Issue:** Für den Wähler gab es die Behauptung (Task 2), für den Auflöser nicht. T-07-19 nennt einen Test ausdrücklich als seine Minderung, und Erfolgskriterium 7 verlangt beide Hälften.
- **Fix:** `TestSchlagwortfeldErreichtKeineFremdeWebsite` — das Kürzel eines Schlagworts der zweiten Website steht von Hand im Feld der ersten, und weder Name noch Kürzel erscheinen im gerenderten Dokument.
- **Files modified:** `internal/public/handler_test.go`
- **Verification:** grün; die Regel wirkt über die eine `ListAll`-Begrenzung in `fieldTerms`, nicht über `Check`
- **Committed in:** `052de1d`

---

**Total deviations:** 2 auto-fixed (beide Rule 2 — fehlende kritische Funktionalität an einer Vertrauensgrenze)
**Impact on plan:** Kein Scope-Creep. Die erste war eine ausdrücklich beauftragte Entscheidung und fiel in die Funktion, die Task 4 ohnehin anfasste; die zweite belegt eine Minderung, die das Bedrohungsmodell dieses Plans selbst fordert.

## Checkpoint Gates

**Dieser Plan trägt keinen Kontrollpunkt.** Alle vier Tasks sind `type="auto"`, keiner ist `type="tracer"`, kein `<precondition>` steht in irgendeinem. Es wurde daher **nichts automatisch genehmigt und nichts automatisch gewählt** — weder ein `gate="blocking"` noch ein `gate="blocking-human"` trat auf. (Zum Vergleich: 07-02 und 07-01 trugen je eine automatisch gewählte Entscheidung, die in STATE.md als bestätigungsbedürftig vermerkt ist. Hier gibt es nichts dergleichen nachzutragen.)

Ein Paketinstallationstor kam ebenfalls nicht vor: es wurde kein `go get` ausgeführt, jedes benutzte Symbol steht in der Standardbibliothek oder schon in `go.mod` (T-07-SC).

## Issues Encountered

- **Der RED-Beweis in Task 4 war die Mühe wert und bestätigte die Warnung des Plans wörtlich.** Von den beiden Untertests fiel im RED-Lauf nur der mit der Umbenennung durch (drei Behauptungen), während der ohne Umbenennung schon gegen den unreparierten Baum **bestand**. Ein Test ohne die Umbenennung hätte nichts bewiesen und den Fehler durchgelassen.
- **Ein Kommentar hätte fast ein Tor gerissen.** Das Abnahmekriterium von Task 3 ist `grep -c '"/tag/"' internal/public/pagedata.go` → 0. Die erste Fassung des Doc-Kommentars von `fieldTerms` schrieb die Adresse zur Erklärung aus und ließ die Zählung auf 1 stehen. Umformuliert, ohne die Aussage zu verlieren.

## User Setup Required

Keine — kein externer Dienst, keine neue Abhängigkeit, keine Wanderung. `internal/db/migrations/` ist unverändert.

## Next Phase Readiness

Bereit für **07-06** (die Knopfreihe, D-10/D-11). Dieser Plan hat `field.go`, `render.go` und `field_input.html` angefasst — dieselben drei Dateien, die 07-06 anfasst —, alle drei stehen sauber und formatiert.

Planmäßig offen und **nicht** von diesem Plan verursacht, alles drei in **07-07** (so schon in STATE.md nach 07-04 vermerkt):

1. **Die Dokumentensteuer je Art.** `internal/tmplspec/TEMPLATE-SPEC.md`, `SampleData` und `MinimalData` in `internal/template/sample.go` kennen `Entry.Term` noch nicht — genauso wenig, wie sie `zeit`, `bereich`, `code` und `mehrfachauswahl` aus 07-01…07-04 kennen. CLAUDE.md verlangt, dass die drei Stellen zusammengehalten werden; die Testreihe erzwingt es für ein neues `Entry`-Feld heute nicht, und die ganze Phase hat diese Steuer bewusst gebündelt.
2. **Die offenen Übersetzungen.** Dieser Plan fügt drei deutsche Quellzeichenketten hinzu (`Schlagwort` samt Hinweis in `Kinds`, `– kein Schlagwort –` in der Vorlage). Die Kataloge wurden nicht geschrieben — kein Plan dieser Phase hat `go run ./tools/i18n -write` ausgeführt, das gehört gebündelt in 07-07.
3. **Die Browserhälfte.** Das Auswahlfeld ist in der Auszeichnung geprüft, nicht im Bild (Deliverable D8, `human_judgment: true`).

**Ein Blocker weniger:** der Eintrag „07-04 gemeldet, nicht behoben: `internal/bundle/import.go` `importFieldValues` … ohne `CheckAll` und ohne `Clean`“ in STATE.md ist mit `7cb09f4` erledigt und wird geschlossen.

Nachgelagert erbt **Phase 9** (CSV-Import) genau die Eigenschaft, für die D-09 das Kürzel und nicht einen Bezeichner gewählt hat: `moebel` lässt sich in eine Tabellenzelle tippen, eine Zeilennummer nicht.

---
*Phase: 07-field-kinds*
*Completed: 2026-09-05*

## Self-Check: PASSED

Alle 13 als geändert genannten Dateien liegen auf der Platte, alle 7 Commit-Kürzel
stehen im Log, und die `<verification>`-Befehle des Plans wurden nach dem letzten
Commit noch einmal vollständig ausgeführt: `go build ./...` grün,
`go test ./...` über den ganzen Baum grün, `gofmt -l internal/ cmd/` ohne
Ausgabe. Keine Stummel, keine übersprungenen Tests, kein nicht ausgeführtes
`<verify>`.
