---
phase: 08-snippets-carry-fields
plan: 03
subsystem: ui
tags: [go, html-template, admin, authorization, snippets, field-definitions, i18n]

# Dependency graph
requires:
  - phase: 08-01
    provides: "field.Def.SnippetID, OfSnippet und die Wanderung 00047 — der vierte Namensraum, den dieser Bildschirm bedient"
  - phase: 08-02
    provides: "validate's Textbaustein-Arm (Pflicht bleibt bedeutungsvoll, Feldarten unverengt), Move's vierten Arm und die traegerweise MaxFields-Zaehlung"
provides:
  - "(*admin.Handler).snippetOf — die eine Eigentumspruefung, drei Nennungen, zwei Aufrufstellen"
  - "admin.FieldListData.Snippet und das auf drei Traeger erweiterte Simple()"
  - "Der ?textbaustein=<id>-Modus von GET /admin/websites/{id}/felder — ohne neue Route"
  - "admin.fieldPath's vierter Parameter und vierter Arm, an allen vier Aufrufstellen"
  - "Die sieben {{else if .Snippet}}-Geschwister in field_list.html"
  - "Die „Felder\"-Schaltflaeche je Textbausteinzeile in snippet_list.html"
  - "internal/admin/snippet_fields_test.go — vier Pruefungen ueber die echten Vorlagen, mutationsgeprueft"
affects: [08-04, 08-05, 09-csv-import]

actuals:
  tokens: 11400
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Ein vierter Modus eines bestehenden Bildschirms statt einer vierten Route"
    - "Eine Eigentumspruefung als Funktion, weil der Speicher sie nicht selbst traegt — GET und POST stellen dieselbe Frage"
    - "Eine Mutationsprobe belegt, dass eine Berechtigungspruefung rot werden kann"

key-files:
  created:
    - internal/admin/snippet_fields_test.go
  modified:
    - internal/admin/field.go
    - cmd/holzcloud/templates/admin/field_list.html
    - cmd/holzcloud/templates/admin/snippet_list.html
    - internal/i18n/locales/en.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json
    - internal/i18n/locales/de-CH.json

key-decisions:
  - "Die Eigentumspruefung steht vor Create UND vor Update, nicht nur vor Create — der Plan verlangt „before the Create\", und eine Stelle frueher deckt beide Zweige, ohne einen legitimen Fall abzuweisen"
  - "Der Kopfabsatz nennt Name und Kennung des Textbausteins und druckt den Theme-Pfad .Site.Bausteinfelder.<kennung>.kennung, nach dem Vorbild des Gruppen-Arms — nicht nach dem des Bausteinart-Arms, der keine Kennung zeigt"
  - "Die Pruefungen behaupten die Abwesenheit ueber unuebersetzte Beschriftungen („Seitenpreis\", „Telefonnummer\") statt ueber uebersetzte Saetze — ein Katalogwechsel darf eine Berechtigungspruefung nicht faerben"
  - "Eine Mutationsprobe gefahren, bevor die Pruefungen als Beweis gefuehrt werden: ohne sn.WebsiteID != websiteID fallen genau die zwei Berechtigungsfaelle"

patterns-established:
  - "Ein Modus mehr an einem Bildschirm: ein Abfrageparameter, ein Strukturmitglied, je ein Arm an jedem positiven Geschwister — und die Verneinungen bleiben unangetastet"
  - "Neue sichtbare Zeichenketten bekommen ihren eigenen Commit, nach -write und -schweiz, mit 0 offen / 0 verwaist als Tor"

requirements-completed: [SNIP-01, SNIP-02, SNIP-04]

coverage:
  - id: D1
    description: "Der Feldbildschirm hat einen vierten Modus, erreicht durch ?textbaustein=<id>, ohne neue Route; er nennt den Textbaustein beim Namen und listet dessen Felder"
    requirement: "SNIP-01"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestTextbausteinModusOeffnetSichFuerDeneigenen"
        status: pass
    human_judgment: false
  - id: D2
    description: "Ein Textbaustein ohne Definitionen zeigt denselben Bildschirm mit leerer Liste und dem Satz, der das sagt"
    requirement: "SNIP-01"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestTextbausteinModusOeffnetSichFuerDeneigenen"
        status: pass
    human_judgment: false
  - id: D3
    description: "Ein Textbaustein einer anderen Website oeffnet den Modus nicht: der Bildschirm faellt auf die eigenen Seitenfelder zurueck und traegt den fremden Namen nicht"
    requirement: "SNIP-02"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestTextbausteinFremderWebsiteOeffnetDenModusNicht"
        status: pass
      - kind: other
        ref: "Mutationsprobe: sn.WebsiteID != websiteID entfernt -> genau dieser Fall faellt, danach wiederhergestellt"
        status: pass
    human_judgment: false
  - id: D4
    description: "Ein POST mit dem Textbaustein einer anderen Website antwortet 404 und legt nichts an — belegt ueber den Speicher, nicht ueber den Status allein"
    requirement: "SNIP-02"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestTextbausteinFremderWebsiteWirdBeimSpeichernAbgewiesen"
        status: pass
      - kind: other
        ref: "Mutationsprobe: ohne die Pruefung antwortet derselbe Aufruf 303 statt 404"
        status: pass
    human_judgment: false
  - id: D5
    description: "Der Seitenbildschirm bleibt unveraendert: GET …/felder ohne Abfrageparameter listet mit einem Textbausteinfeld in der Datenbank genau die eigenen Felder der Seite"
    requirement: "SNIP-04"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestTextbausteinfeldErscheintNichtAufDemSeitenbildschirm"
        status: pass
    human_judgment: false
  - id: D6
    description: "Der Modus bietet field.Kinds in voller Breite, Gruppen eingeschlossen; „Gilt fuer\" verschwindet, „Pflicht\" bleibt"
    requirement: "SNIP-01"
    verification:
      - kind: integration
        ref: "internal/admin/snippet_fields_test.go#TestTextbausteinModusOeffnetSichFuerDeneigenen"
        status: pass
      - kind: other
        ref: "grep -v '{{/*' field_list.html | grep -c 'not .*BlockType}}' -> 3 (keine Pflicht-Verneinung verbreitert)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Kein JavaScript auf den beiden Bildschirmen; jede Bedienung ist eine gewoehnliche Formularabgabe"
    requirement: "SNIP-01"
    verification:
      - kind: other
        ref: "grep -ci 'script|onclick|onchange|javascript:' field_list.html snippet_list.html -> je 0"
        status: pass
    human_judgment: false
  - id: D8
    description: "Der Bildschirm im Browser: Register der neuen deutschen Saetze, Bedienbarkeit der Feldliste, Aussehen des vierten Modus neben den drei bestehenden"
    verification: []
    human_judgment: true
    rationale: "Optik und Register sind Urteilsfragen. Die Browserhaelfte der Phase ist Plan 08-05 zugewiesen; hier steht der Bildschirm samt seiner Berechtigung, gefahren ueber die echten Vorlagen von der Platte."

# Metrics
duration: 9 min
completed: 2026-09-06
status: complete
---

# Phase 8 Plan 03: Der vierte Modus des Feldbildschirms Summary

**`?textbaustein=<id>` macht den einen Feldbildschirm zu seinem vierten Modus — ohne neue Route, mit der vollen Feldartenliste und mit `snippetOf` als einziger Eigentumsprüfung, die auf dem GET **und** auf dem POST dieselbe Frage stellt; belegt durch vier Prüfungen über die echten Vorlagen und durch eine Mutationsprobe, die zeigt, dass genau die zwei Berechtigungsfälle rot werden können.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-09-06T09:58:25Z
- **Completed:** 2026-09-06T10:07:00Z
- **Tasks:** 2
- **Files modified:** 9 (1 neu, 8 geändert)

## Accomplishments

- **`snippetOf` ist die eine Prüfung, und ihr Doc-Kommentar sagt, warum sie überhaupt existiert.** `blockTypes.Get` nimmt die Websitenummer und findet eine Bausteinart einer anderen Seite schlicht nicht; `snippets.Get` nimmt nur eine Nummer. Diese Asymmetrie ist der ganze Grund, dass hier eine Funktion steht statt zweier Vergleiche, die auseinanderlaufen können. `h.snippets.Get` steht in `internal/admin/field.go` genau **einmal** — im Rumpf des Helfers.
- **Der vierte Modus ist ein Abfrageparameter, kein Bildschirm.** `FieldListData.Snippet` neben `BlockType`, `Simple()` prüft jetzt alle drei Träger, `fieldListData` bekommt einen vierten `case`, der über `OfSnippet` liest und `field.Kinds` **in voller Breite** anbietet — mit dem einzeiligen Grund daneben: die vier Ausschlüsse von `BlockKinds()` bestehen, weil ein Baustein beim Speichern zu HTML erstarrt, und die Werte eines Textbausteins werden beim Anzeigen aufgelöst.
- **`fieldPath` hat seinen vierten Arm, und alle vier Aufrufstellen füttern ihn.** `HandleFieldDelete` und `HandleFieldMove` lesen `SnippetID` zusammen mit `ParentID` und `BlockTypeID` vom Gespeicherten zurück, damit die Umleitung auf dem Bildschirm landet, auf dem der Bediener stand. Die Löschmeldung — die absichtlich sagt, was *nicht* geschehen ist — bleibt unverändert und gilt damit auch für ein Textbausteinfeld.
- **Sieben Arme, nicht zehn.** `field_list.html` trägt zehn `BlockType}}`-Vorkommen; sieben sind positiv und bekamen ein `{{else if .Snippet}}`-Geschwister, drei sind `{{if not .BlockType}}`-Verneinungen und blieben unangetastet. Das ist keine Zählübung: die drei Verneinungen sind die drei Orte der „Pflicht"-Spalte, und eine davon zu verbreitern hätte dem Bediener die einzige Möglichkeit genommen, ein Textbausteinfeld als Pflicht zu markieren. Beide Tore messen genau die Zahlen des Plans, **7** und **3**.
- **Der Weg hinein und die Prüfungen, die die Berechtigung halten.** Jede Textbausteinzeile trägt eine „Felder"-Schaltfläche nach dem Vorbild von `blocktype_list.html:58`. Vier Prüfungen über `newTestAdmin` — der eigene Modus samt leerem Fall, die fremde Website per GET, die fremde Website per POST, und der Seitenbildschirm, der sauber bleibt.
- **Die Prüfungen sind mutationsgeprüft.** `sn.WebsiteID != websiteID` einmal aus `snippetOf` entfernt: der GET-Fall meldete den fremden Namen auf dem Bildschirm, der POST-Fall antwortete **303 statt 404**. Danach wiederhergestellt und erneut grün. Drei grüne Fälle beweisen nichts, solange nicht feststeht, dass sie rot werden können.
- **Sechs neue sichtbare Sätze, übersetzt in ihrem eigenen Commit.** `-write`, dann `-schweiz`, dann en/es/fr/it von Hand; `go run ./tools/i18n` meldet für alle vier `0 offen, 0 verwaist`. `de-CH` bekam den einen Eintrag, den die Anführungszeichen erzwingen.

## Task Commits

1. **Task 1: Der vierte Modus des Feldbildschirms** — `48e5b1d` (feat)
2. **Task 2: Der Weg hinein, und die Prüfungen** — `a14723e` (feat)
3. **Die sechs neuen Zeichenketten in en, es, fr, it** — `23b01af` (chore)

**Plan metadata:** der `docs(08-03)`-Commit dieses Plans

## Files Created/Modified

- `internal/admin/snippet_fields_test.go` *(neu)* — vier Prüfungen und drei Helfer über `newTestAdmin`; die Doc-Prosa sagt, warum diese Prüfungen in `internal/admin` und nicht im Speicher liegen
- `internal/admin/field.go` — `snippetOf`; `FieldListData.Snippet`; `Simple()` auf drei Träger; die dritte Modus-Lesung in `HandleFieldList`; `textbaustein` im Formular von `HandleFieldSave` samt der 404-Abweisung; `SnippetID` in `HandleFieldDelete` und `HandleFieldMove`; `fieldListData`s vierter `case`; `fieldPath`s vierter Parameter und Arm
- `cmd/holzcloud/templates/admin/field_list.html` — sieben `{{else if .Snippet}}` / `{{if .Snippet}}`-Arme; die drei `{{if not .BlockType}}`-Verneinungen unverändert
- `cmd/holzcloud/templates/admin/snippet_list.html` — die „Felder"-Schaltfläche je Zeile, nur diese eine Zeile
- `internal/i18n/locales/{en,es,fr,it}.json` — je sechs neue Übersetzungen
- `internal/i18n/locales/de-CH.json` — der eine Eintrag, den `-schweiz` erzeugt

## Decisions Made

- **Die 404-Abweisung steht vor `id > 0`, also vor `Create` **und** vor `Update`.** Der Plan verlangt sie „before the `Create`". Eine Stelle früher deckt beide Zweige; ein Textbaustein der eigenen Website läuft unverändert durch, und ein Seitenfeld trägt gar kein `textbaustein`, sodass die Prüfung dort übersprungen wird. `Update` pinnt `SnippetID` ohnehin vom Gespeicherten (Plan 08-01, T-08-14) — die frühere Stelle nimmt dem Bedrohungsmodell nichts weg und schliesst den Zweig zusätzlich.
- **Der Kopfabsatz folgt dem Gruppen-Arm, nicht dem Bausteinart-Arm.** Er nennt Name *und* Kennung und druckt den Theme-Pfad `.Site.Bausteinfelder.<kennung>.kennung`, so wie der Gruppen-Arm `{{range .Page.Felder.<kennung>}}` druckt. Der Bausteinart-Arm zeigt keine Kennung, weil im Bausteineditor niemand eine tippt; bei einem Textbaustein ist die Kennung genau das, was ein Theme-Autor braucht.
- **Der zweite Absatz sagt, warum die Feldartenliste hier vollständig ist.** Damit steht der Grund für `field.Kinds` an drei Orten in derselben Formulierung: im `validate`-Kommentar (08-02), im `fieldListData`-Arm und auf dem Bildschirm selbst.
- **Die Prüfungen behaupten Anwesenheit und Abwesenheit über unübersetzte Beschriftungen** — „Seitenpreis", „Telefonnummer", „Fremder Kontaktblock" — und nicht über übersetzte Sätze. Ein Katalogwechsel darf eine Berechtigungsprüfung nicht einfärben. Die einzige Ausnahme ist der Nachweis des leeren Falls, der auf dem Teilstring „noch keine Felder" steht.
- **Der Rückweg des vierten Modus geht auf `/snippets` und nicht auf `/bausteinarten`.** Das ist die Liste, von der die „Felder"-Schaltfläche kommt.

## Deviations from Plan

Keine. Der Plan wurde ausgeführt, wie er geschrieben steht.

Insbesondere sind **beide Zähltore des Plans genau eingetroffen** — anders als in den Wellen 1 und 2, deren Gatter gegen den Baum *vor* ihrer eigenen Änderung gerechnet waren:

| Gatter | Plan | Gemessen | |
|---|---|---|---|
| `grep -v '{{/*' field_list.html \| grep -c 'Snippet}}'` | ≥ 7 | **7** | ✅ |
| `grep -v '{{/*' field_list.html \| grep -c 'not .*BlockType}}'` | genau 3 | **3** | ✅ |
| `grep -v '^\s*//' field.go \| grep -c 'snippetOf('` | ≥ 3 | **3** | ✅ |
| `grep -c 'h.snippets.Get' field.go` | ≤ 1 | **1** | ✅ |
| `grep -ci 'script\|onclick\|onchange\|javascript:'` (beide Vorlagen) | 0 | **0 / 0** | ✅ |
| `grep -c 'func newTestAdmin' page_handler_test.go` | 1 | **1** | ✅ |
| `grep -c 'textbaustein' snippet_list.html` | > 0 | **1** | ✅ |
| `go test -run 'TestTextbaustein\|TestSnippetFeld' -v` | ≥ 3 `--- PASS` | **4** | ✅ |

Der Grund, dass die Zahlen diesmal stimmen: der Plan hat die zehn `BlockType}}`-Vorkommen von `field_list.html` Zeile für Zeile ausgezählt und in eine Tabelle geschrieben, statt sie zu schätzen. Gegen den Baum nachgemessen, bevor die erste Zeile geändert wurde — die zehn Zeilennummern des Plans (`:7`, `:32`, `:37`, `:59`, `:72`, `:92`, `:100`, `:105`, `:223`, `:236`) stimmen alle.

**Total deviations:** 0.
**Impact on plan:** Kein Zuwachs am Umfang, keine Zahl passend gemacht.

## Issues Encountered

- **Die Testhelfer hatten zunächst einen erfundenen Typ und ein erfundenes Feld** (`sm sitzung`, `h.db`). Beim ersten `gofmt`/`vet`-Lauf gemeldet und auf `*scs.SessionManager` und einen durchgereichten `*db.DB` gestellt, bevor irgendetwas commitet wurde.
- **Die Übersetzungskataloge werden von einem Formattest gehalten** (06-03: `writeCatalog` ist `encoding/json` von Ende zu Ende). Nach dem Schreiben der vier Kataloge wurde `go test ./internal/i18n/` gefahren und `git diff --stat` gelesen: genau die 6 + 6 + 6 + 6 neuen Zeilen und die eine in `de-CH`, keine Umformatierung.

## Known Stubs

Keine. Jeder Arm dieses Plans trägt echte Daten; der vierte Modus liest über `OfSnippet` und schreibt über `field.Store.Create` — dieselben zwei Wege, die die drei bestehenden Modi gehen.

## Threat Flags

Keine neue sicherheitsrelevante Fläche über den `<threat_model>` des Plans hinaus.

- **T-08-12** (`?textbaustein=`-Lesung): gemindert. `snippetOf` gibt für eine fremde Nummer `nil` zurück, der Modus öffnet sich nicht, und der Bildschirm fällt auf die eigenen Seitenfelder zurück. Die Prüfung behauptet die **Abwesenheit** des fremden Namens, nicht nur den Status.
- **T-08-13** (verstecktes `textbaustein`-Eingabefeld): gemindert. Derselbe Helfer läuft auf dem POST-Pfad, vor jedem Schreiben; die Prüfung sieht danach durch den Speicher nach, unter **beiden** Websitenummern.
- **T-08-14** (Träger über das Änderungsformular verschieben): unverändert gedeckt durch `Update`s Pin aus 08-01. Der Ändern-Link dieses Bildschirms trägt den Modus nur mit, um den Bediener zur richtigen Liste zurückzubringen.
- **T-08-15** (Spoofing auf die POST-Route): angenommen wie im Plan. Keine neue Route; `requireAdmin` und `gorilla/csrf` gelten unverändert, und das versteckte `textbaustein`-Feld steht in demselben Formular, das den Token bereits ausgibt.
- **T-08-16** (Skripting): gemindert. Beide Vorlagen messen 0 bei `script|onclick|onchange|javascript:`; die Ordnungsknöpfe sind unverändert gewöhnliche Formularabgaben, und `hx-disabled-elt` ist ein htmx-Attribut, kein Ereignisattribut.
- **T-08-17** (Löschmeldung): unverändert übernommen. Sie nennt weder eine andere Website noch eine Nummer.

## User Setup Required

Keine — kein externer Dienst berührt.

## Next Phase Readiness

Bereit für **Plan 08-04**, den Wertebildschirm: die **Definitions**hälfte steht, und 08-04 findet `OfSnippet` für die Liste und `snippet.Store.SetFields` für das Speichern bereits vor. Der Plan hat ausdrücklich nur die eine Zeile in `snippet_list.html` angefasst und das Wertformular gar nicht berührt, damit nicht zwei Pläne einer Welle in derselben Datei stehen.

Unverändert offen und ausdrücklich Plan 08-05 zugewiesen (aus 08-01 und 08-02 übernommen): `MinimalData`s leerer Zwilling für `Bausteinfelder`/`Bausteinliste` samt der Umschreibung des Satzes „no menus, no labels, no snippets", und die §7-Prosa der `TEMPLATE-SPEC.md`.

Kein Kontrolltor in diesem Plan: beide Aufgaben trugen `type="auto"`, es gab keinen `checkpoint:*` und damit auch keine gewählte Option zu melden.

Keine Blockierer. `.planning/phases/11-galerie/` ist unberührt.

---
*Phase: 08-snippets-carry-fields*
*Completed: 2026-09-06*

## Self-Check: PASSED

`internal/admin/snippet_fields_test.go` liegt auf der Platte; die drei Commits
`48e5b1d`, `a14723e` und `23b01af` stehen im Verlauf. Jedes zugesagte Symbol ist
im Baum: `(*Handler).snippetOf`, `FieldListData.Snippet`, `case snippetID > 0:`
in `fieldPath`, `h.fields.OfSnippet` im vierten `case` von `fieldListData` — und
`h.snippets.Get` steht in der Datei genau einmal. `go build ./...`,
`go vet ./...`, `gofmt -l .` und `go test ./...` sind grün, und
`go run ./tools/i18n` meldet für en/es/fr/it je `0 offen, 0 verwaist`.
