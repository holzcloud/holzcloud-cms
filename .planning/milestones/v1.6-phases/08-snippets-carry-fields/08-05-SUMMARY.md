---
phase: 08-snippets-carry-fields
plan: 05
subsystem: testing
tags: [go, html-template, theme-contract, bundle, zip, i18n, playwright, xss]

# Dependency graph
requires:
  - phase: 08-01
    provides: "SiteData.Bausteinfelder / .Bausteinliste, das vorgezogene SampleData und die zwei .Site-Zeilen der TEMPLATE-SPEC.md — samt der ausdrücklich offen gelassenen Hälfte"
  - phase: 08-02
    provides: "validate's Textbaustein-Arm (Bedingung geleert — darum trägt der Importweg keinen zweiten Durchgang), OfSnippets und die trägerweise MaxFields-Zählung"
  - phase: 08-03
    provides: "den vierten Modus des Feldbildschirms und die „Felder\"-Schaltfläche, die der Browserdurchgang gefahren hat"
  - phase: 08-04
    provides: "den Feldteil des Textbausteinformulars, groupAction und snippets.SetFields — was Schritt 3 und Schritt 7 im Browser bedienen"
  - phase: 07-field-kinds
    provides: "die vier Reflexionswächter über den Theme-Vertrag, deren Asymmetrie dieser Plan bedient"
provides:
  - "MinimalData.Site.Bausteinfelder — dieselben Kennungen am leeren Wert ihrer Feldart, und ausdrücklich keine Bausteinliste"
  - "SampleData.Site.Bausteinfelder trägt einen Wert, der keine Zeichenkette ist (*time.Time)"
  - "TestMinimalDataCarriesTheEmptyValueOfEverySnippetField — der Wächter, der die Asymmetrie in beide Richtungen festhält"
  - "TEMPLATE-SPEC.md §7: zweistufige Indizierung, das Durchlaufen der Liste, und der Satz über die zwei Karten"
  - "bundle.Snippet.Fields, .Values und .ValueGroups, alle drei omitempty"
  - "exportFieldDef — eine Abschrift statt zweier für exportFields und exportSnippets"
  - "importSnippetFields und cleanSnippetValues — jede Definition über field.Store.Create, jeder Wert durch Clean und CheckAll"
  - "Der Träger eines Unterfeldes wird aus der gespeicherten Gruppe gepinnt (Fehlerflick)"
  - "Sieben Browserschritte mit Schirmbild und Befund"
affects: [09-csv-import]

actuals:
  tokens: 9580
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Eine Vorrichtung, die den leeren Fall trägt, braucht einen Wächter, der beide Richtungen der Asymmetrie rot werden lässt"
    - "Ein neuer Träger im Archiv geht durch dieselben zwei Wächter wie der alte (Clean und CheckAll), oder er reisst wieder auf, was eine frühere Phase geschlossen hat"
    - "Ein Unterfeld erbt seinen Träger aus dem Gespeicherten, nicht aus dem Formular"

key-files:
  created: []
  modified:
    - internal/template/sample.go
    - internal/template/sample_test.go
    - internal/tmplspec/TEMPLATE-SPEC.md
    - internal/bundle/format.go
    - internal/bundle/export.go
    - internal/bundle/import.go
    - internal/bundle/bundle_test.go
    - internal/admin/field.go
    - internal/admin/snippet_fields_test.go

key-decisions:
  - "Die von 08-01 notierte Lücke ist hier geschlossen: MinimalData trägt Bausteinfelder am leeren Wert und keine Bausteinliste, und der Satz „no snippets\" ist auf das korrigiert, was jetzt wahr ist"
  - "Ein Wächter für die Asymmetrie wurde ergänzt, obwohl der Plan nur sample.go und TEMPLATE-SPEC.md nennt: kein bestehender Test sieht die Site-Hälfte an, und eine Zusage ohne Wächter ist eine Zusage, die beim nächsten Zug still bricht"
  - "bundle.Snippet bekam ein drittes Mitglied (ValueGroups): der Plan sagt, die Entscheidung der Seite zu Gruppenzeilen sei zu übernehmen — und die Seite trägt sie (Page.FieldGroups)"
  - "Der Importweg fährt field.CheckAll und nicht nur field.Clean: ohne ihn stünde auf dem Textbaustein wieder offen, was 07-04 auf der Seite geschlossen hat"
  - "Kennungen in einem Wert reisen NICHT übersetzt (Bild-, Verweis-, Schlagwortfelder): als Grenze im Doc-Kommentar von bundle.Snippet festgehalten, weil die dafür nötigen Nachschlagewerke im Seitenexport und -import eingeschlossen sind"
  - "Kein Katalog-Commit: dieser Plan fügt keine sichtbare deutsche Zeichenkette hinzu — 08-03 und 08-04 haben ihre schon übersetzt, das Tor misst 0 offen / 0 verwaist ohne einen Zug"

patterns-established:
  - "Der Browserdurchgang ist kein Abhaken: er hat den einen Fehler gefunden, den vier grüne Prüffolgen nicht gefunden haben"
  - "Ein Befund, dessen Flick ein Tor einer anderen Ebene fallen liesse, wird zurückgestellt und aufgeschrieben, nicht halb gemacht"

requirements-completed: [SNIP-03, SNIP-05]

coverage:
  - id: D1
    description: "SampleData trägt ein gefülltes .Site.Bausteinfelder und ein gefülltes .Site.Bausteinliste, beide nicht leer — walkContract meldet eine leere Karte als Nullwert, und die Hochladeprüfung soll beide mindestens einmal zeichnen"
    requirement: "SNIP-03"
    verification:
      - kind: unit
        ref: "internal/template/sample_test.go#TestSampleDataFillsEveryField"
        status: pass
      - kind: unit
        ref: "internal/template/sample_test.go#TestSampleFieldsAreShapedLikeTheRendererProducesThem"
        status: pass
    human_judgment: false
  - id: D2
    description: "MinimalData trägt .Site.Bausteinfelder mit jeder Kennung am leeren Wert ihrer Feldart und trägt KEINE .Site.Bausteinliste — die Asymmetrie, die field.List erzwingt"
    requirement: "SNIP-03"
    verification:
      - kind: unit
        ref: "internal/template/sample_test.go#TestMinimalDataCarriesTheEmptyValueOfEverySnippetField"
        status: pass
      - kind: other
        ref: "Mutationsprobe: Kennung entfernt → Fall 1 fällt; Bausteinliste erfunden → Fälle 2 und 3 fallen; danach wiederhergestellt"
        status: pass
      - kind: e2e
        ref: "holzcloud template check theme.zip → „no problems found\" für ein Theme, das {{index .Site.Bausteinfelder …}} und {{range index .Site.Bausteinliste …}} schreibt"
        status: pass
    human_judgment: false
  - id: D3
    description: "TEMPLATE-SPEC.md nennt beide Pfade in der .Site-Tabelle und erklärt in §7 die zweistufige Indizierung, das Durchlaufen der Liste und den Satz, dass ein Textbaustein ohne gefülltes Feld in der einen Karte steht und in der anderen fehlt"
    requirement: "SNIP-03"
    verification:
      - kind: unit
        ref: "internal/tmplspec/spec_test.go#TestSpecDocumentsEveryFieldOfTheContract"
        status: pass
      - kind: other
        ref: "grep -c 'Site.Bausteinliste' internal/tmplspec/TEMPLATE-SPEC.md → 5 (Tor: ≥ 2)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Ein Archiv trägt die Felddefinitionen eines Textbausteins und seine ausgefüllten Werte; Export und Import auf einer leeren Datenbank stellen beide wieder her — die Definitionen über field.Store.Create, die Werte über field.Clean und field.Encode, nie über ein eigenes INSERT"
    requirement: "SNIP-05"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestTextbausteinfelderUeberlebenDieArchivreise"
        status: pass
      - kind: other
        ref: "grep -c 'INSERT INTO page_field_defs' internal/bundle/export.go internal/bundle/import.go → 0 / 0"
        status: pass
      - kind: other
        ref: "Mutationsprobe: importSnippetFields entfernt → die Rundreise fällt mit „0 Definitionen\"; danach wiederhergestellt"
        status: pass
    human_judgment: false
  - id: D5
    description: "Ein Manifest von vor dieser Phase importiert unverändert: ein Textbaustein ohne fields und ohne values kommt mit Rumpf und ohne Definitionen an, und ein Textbaustein ohne Felder schreibt die drei neuen Schlüssel gar nicht erst"
    requirement: "SNIP-05"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestAeltererTextbausteinImportiertUnveraendert"
        status: pass
    human_judgment: false
  - id: D6
    description: "Eine abgewiesene Definition kostet ihr Feld und nicht den Import und wird gemeldet; ein Wert, den keine Definition trägt, wird von field.Clean weggenommen statt gespeichert"
    requirement: "SNIP-05"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestTextbausteinfelderAusDemArchivGehenDurchDieselbePruefung"
        status: pass
      - kind: other
        ref: "Mutationsprobe: field.Clean aus cleanSnippetValues entfernt → beide Fremdwerte werden gespeichert, die Prüfung fällt; danach wiederhergestellt"
        status: pass
    human_judgment: false
  - id: D7
    description: "go run ./tools/i18n meldet 0 offen, 0 verwaist für en, es, fr und it; fr-CH.json und it-CH.json sind unberührt"
    verification:
      - kind: other
        ref: "go run ./tools/i18n | grep -c '0 offen, 0 verwaist' → 4; git diff --exit-code -- fr-CH.json it-CH.json → 0"
        status: pass
    human_judgment: false
  - id: D8
    description: "Alle sieben Browserschritte wurden durch die laufende Anwendung gefahren: der Feldbildschirm im vierten Modus, zwei Felder mit „Pflicht\" und ohne „Gilt für\", das Textbausteinformular samt Neuladen, das saubere Seitenformular, die öffentliche Seite mit Rumpf und Feldwert, das nicht ausgeführte <script>, und dasselbe Formular mit abgeschaltetem JavaScript"
    verification:
      - kind: automated_ui
        ref: "playwright:shots/01b,02,03b,04,05,06,07d.png — je Schritt ein Schirmbild, Befunde unten"
        status: pass
    human_judgment: true
    rationale: "Die sieben Schritte sind gefahren und je mit einem Schirmbild belegt; die Beurteilung, ob die Bildschirme für einen Betreiber auch gut aussehen und sich richtig anfühlen, bleibt ein menschliches Urteil, das kein Skript abnimmt."

duration: 32 min
completed: 2026-09-06
status: complete
---

# Phase 8 Plan 05: Die Vorrichtungen, das Archiv und der Browser Summary

**`MinimalData` trägt jetzt den Fall, an dem ein Theme wirklich bricht — ein Textbaustein, dessen Felder definiert und leer sind, mit `Bausteinfelder` und ausdrücklich ohne `Bausteinliste`; ein Archiv trägt Definitionen und Werte eines Textbausteins durch `Create`, `Clean` und `CheckAll` heil hin und zurück; und der Browserdurchgang hat den einen Fehler gefunden, den vier grüne Prüffolgen nicht gefunden haben: eine Gruppe am Textbaustein, die keine einzige Zeile zeichnen konnte.**

## Performance

- **Duration:** 32 min
- **Started:** 2026-09-06T10:22:00Z
- **Completed:** 2026-09-06T10:56:00Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments

- **Die von 08-01 notierte Lücke ist geschlossen.** `MinimalData` trägt
  `Bausteinfelder` mit denselben drei Kennungen wie `SampleData`, jede am leeren
  Wert ihrer Feldart (`""`, `""`, `(*time.Time)(nil)`), und trägt **keine**
  `Bausteinliste`. Der Satz „no menus, no labels, no snippets" ist auf das
  korrigiert, was jetzt wahr ist: keine Baustein-*Rümpfe*, und ein Baustein,
  dessen Felder definiert und leer sind.
- **`SampleData` hat einen Wert bekommen, der keine Zeichenkette ist.** Es trug
  seit 08-01 zwei Texte; ein `*time.Time` (`oeffnet`) steht jetzt daneben, in
  beiden Karten und in derselben Schreibweise, in der `Page.Felder` ihn
  schreibt. Ohne ihn hätte die Hochladeprüfung einen getippten Wert auf einem
  Textbaustein nie ein einziges Mal gezeichnet.
- **Der Wächter, den es noch nicht gab.** Kein bestehender Test sieht die
  `Site`-Hälfte an — `TestMinimalDataCarriesTheEmptyValueOfEveryOwnField` prüft
  `Page`. `TestMinimalDataCarriesTheEmptyValueOfEverySnippetField` hält jetzt
  beide Richtungen fest und fährt zusätzlich die zwei Formen, die §7 einem
  Theme-Autor vorschreibt, gegen `MinimalData`: beide drucken nichts und keine
  scheitert.
- **Die Spezifikation erklärt die Asymmetrie, statt sie zu verschweigen.** §7
  bekommt die zweistufige Indizierung, das Durchlaufen einer Bausteinliste, den
  Satz „ein Textbaustein ist Rumpf plus optionale Felder, so wie eine Seite
  Inhalt plus optionale Felder ist" — und ausdrücklich den Satz, dass ein
  Textbaustein ohne gefülltes Feld einen Eintrag in `Bausteinfelder` hat und
  keinen in `Bausteinliste`.
- **Das Archiv trägt beide Hälften.** `bundle.Snippet` bekommt `Fields` (die
  Definitionen, wie `BlockType.Fields`), `Values` (die Antworten, wie
  `Page.Fields`) und `ValueGroups` (die Zeilen, wie `Page.FieldGroups`) — alle
  drei `omitempty`. Der Import legt **jede** Definition über `field.Store.Create`
  an, damit `validate` auf einer importierten so läuft wie auf einer getippten,
  und schickt die Werte durch `field.Clean` **und** `field.CheckAll`.
- **Sieben Browserschritte gefahren, nicht als Liste zurückgegeben** — gegen
  einen echten Server auf einem Wegwerf-Datenverzeichnis, mit Playwright, mit
  einem Schirmbild je Schritt. Der Durchgang hat einen echten Fehler gefunden
  und einen zweiten Befund zurückgestellt.

## Task Commits

1. **Task 1: Die Vorrichtungen und die Spezifikation** — `2ff3239` (feat)
2. **Task 2: Das Archiv trägt die Felder und die Werte** — `739c5ad` (feat)
3. **Task 3: Kataloge und Browserdurchgang** — kein eigener Commit für die
   Kataloge (nichts zu committen, siehe unten); der im Durchgang gefundene
   Fehler wurde als `3ad28ba` (fix) eingespielt.

**Plan metadata:** der `docs(08-05)`-Commit dieses Plans.

## Die sieben Browserschritte, und was jedes Schirmbild zeigt

Gefahren gegen `http://localhost:8099`, gestartet mit
`HOLZCLOUD_DATA_DIR` auf einem Wegwerf-Verzeichnis unterhalb des
Sitzungs-Kritzelpfads. Playwright kam aus dem bereits vorhandenen npx-Zwischenlager
— nichts wurde nachgeladen, und die Anwendung selbst bleibt eine einzelne
Go-Binärdatei. Die Schirmbilder liegen unter
`…/scratchpad/shots/`; das Datenverzeichnis ist danach gelöscht, der Server
gestoppt.

| # | Schirmbild | Was darauf zu sehen ist |
|---|---|---|
| 0 | `00-setup-fertig` | Konto angelegt, Zwei-Faktor eingerichtet (Pflicht) — der Weg zum eigentlichen Durchgang |
| 1a | `01a-textbausteinliste-mit-felder-knopf` | Die Textbausteinliste mit der „Felder"-Schaltfläche je Zeile; sie zeigt auf `…/felder?textbaustein=1` |
| 1b | `01b-feldbildschirm-vierter-modus` | Der Feldbildschirm im vierten Modus. Die Überschrift nennt den Textbaustein: **„Textbaustein „Fusszeile Kontakt" – Schreinerei Holz"**, der Kopfabsatz druckt `{{.Site.Bausteinfelder.footer-kontakt.kennung}}`, und der leere Fall trägt seinen Satz |
| 2 | `02-zwei-felder-pflicht-ja-giltfuer-nein` | Ein `text`- und ein `langtext`-Feld angelegt. Spalten: BESCHRIFTUNG, KENNUNG, ART, **PFLICHT** — **„Gilt für" fehlt**, was der sichtbare Unterschied zum Seitenmodus ist. Die Artenliste bietet alle fünfzehn Arten samt `gruppe` |
| 2b | `02b-seitenfeldbildschirm-ohne-textbausteinfelder` | Derselbe Bildschirm ohne Abfrageparameter: keine Tabelle, keine Zeile — die zwei Textbausteinfelder tauchen dort nicht auf |
| 3a | `03a-textbausteinformular-mit-feldteil` | Das Textbausteinformular mit dem Feldteil „Angaben zu diesem Textbaustein", nachweislich **unter** dem Markdown-Kasten (im DOM verglichen) |
| 3b | `03b-werte-ueberleben-das-neuladen` | Beide Werte getippt, gespeichert, Seite neu geladen — `07721 123456` und `Hinter dem Bahnhof, zweite Einfahrt links.` stehen wieder in ihren Kästchen |
| 4 | `04-seiteneditor-ohne-textbausteinfeld` | **Der wichtigste Schritt.** Das Formular für eine neue Seite trägt sechzehn Eingaben, **keine einzige mit `feld_`-Präfix**, und weder die Beschriftung „Telefon" noch „Anfahrt" noch „Angaben zu diesem Textbaustein" |
| 5 | `05-oeffentliche-seite-rumpf-und-feldwert` | Die öffentliche Seite `/kontakt` mit dem Marker `[[snippet:footer-kontakt]]`. Der Rumpf erscheint zweimal (im Seiteninhalt und über `.Site.Snippets`), der Feldwert über `{{index .Site.Bausteinfelder "footer-kontakt" "telefon"}}`, und die ganze Liste über `{{range index .Site.Bausteinliste "footer-kontakt"}}` mit Beschriftung und Wert |
| 6 | `06-script-erscheint-als-text` | `<script>alert(1)</script>` in das `langtext`-Feld getippt und gespeichert. Auf der öffentlichen Seite steht der Text **wörtlich sichtbar**; im Quelltext steht `&lt;script&gt;`; `document.querySelectorAll('script')` findet **null** Elemente mit Inhalt; **kein** `alert`-Dialog, **kein** Seitenfehler |
| 7a | `07a-gruppe-mit-unterfeldern-js-aus` | Mit **abgeschaltetem JavaScript**: eine Gruppe „Öffnungszeiten" und zwei Unterfelder angelegt, jede Bedienung eine gewöhnliche Formularabgabe |
| 7b–d | `07b/07c/07d-…-js-aus` | Ebenfalls ohne JavaScript: das Textbausteinformular mit dem Feldteil, „Zeile hinzufügen" (`name=gruppenaktion value=neu:oeffnungszeiten`, `formnovalidate`), die gefüllte Zeile 1 mit „Zeile nach oben", „Zeile nach unten" und „Entfernen" — gespeichert, neu geladen, `Montag` und `08:00` stehen noch da |
| 5b | `05b-oeffentliche-seite-endstand` | Der Endstand der öffentlichen Seite: Telefon, Anfahrt und die Gruppe „Öffnungszeiten" in `.Site.Bausteinliste` |

**Schritt 4 ausdrücklich:** Das Seitenformular trug kein einziges
Textbausteinfeld — weder als Eingabe (`feld_*`: leer) noch als Beschriftung noch
als Überschrift. Die im Browser sichtbare Hälfte des gefährlichen Schnitts hält.

**Schritt 6 ausdrücklich:** Das `<script>`-Element erschien als Text und wurde
nicht ausgeführt. Kein Dialog, kein Skript im Dokument, im Quelltext maskiert.

**Schritt 7 ausdrücklich:** Mit abgeschaltetem JavaScript funktionierten das
Anlegen von Feldern, das Anlegen einer Gruppe samt Unterfeldern, die
Zeilenknöpfe der Gruppe und das Speichern des Formulars vollständig.

## Files Created/Modified

- `internal/template/sample.go` — `oeffnet` als `*time.Time` in beiden
  `SampleData`-Karten; `MinimalData.Site.Bausteinfelder` am leeren Wert und ohne
  `Bausteinliste`; der korrigierte Doc-Satz
- `internal/template/sample_test.go` — `TestMinimalDataCarriesTheEmptyValueOfEverySnippetField`
- `internal/tmplspec/TEMPLATE-SPEC.md` — §7 mit den zwei Beispielen und dem
  Absatz über die Asymmetrie; `FieldEntry` nennt jetzt beide Listen
- `internal/bundle/format.go` — `Snippet.Fields`, `.Values`, `.ValueGroups` samt
  dem Doc-Kommentar, der die zwei Vorbilder benennt und die Grenze aufschreibt
- `internal/bundle/export.go` — der Textbaustein-Arm und `exportFieldDef`, das
  `exportFields` und `exportSnippets` teilen
- `internal/bundle/import.go` — `importSnippetFields` und `cleanSnippetValues`
- `internal/bundle/bundle_test.go` — drei Prüfungen: Rundreise,
  Rückwärtsverträglichkeit, bösartiges Manifest
- `internal/admin/field.go` — das Unterfeld erbt seinen Träger aus der
  gespeicherten Gruppe (Fehlerflick)
- `internal/admin/snippet_fields_test.go` — `TestGruppeAmTextbausteinTraegtIhreUnterfelder`

## Decisions Made

- **Ein dritter Manifest-Schlüssel (`ValueGroups`).** Der Plan nennt zwei
  Mitglieder, sagt aber im selben Absatz, die Entscheidung des Seitenexports zu
  Gruppen*zeilen* sei zu übernehmen — „und wenn Seiten keine Zeilen tragen,
  tragen Textbausteine auch keine". Seiten tragen sie (`Page.FieldGroups`), also
  tragen Textbausteine sie auch. Ein Textbaustein hat ein eigenes Formular und
  darf eine Gruppe tragen; ohne diesen Schlüssel hätte das Archiv genau die
  Zeilen verloren, die der Browserdurchgang in Schritt 7 eintippt.
- **`field.CheckAll` neben `field.Clean`.** Der Plan und `T-08-25` nennen nur
  `Clean`. Der Seitenweg fährt beide, und sein Kommentar sagt warum: `CheckAll`
  ist seit 07-04 der Ort, an dem der Byte-Vorrat und jede Regel je Feldart
  wohnen, „und dieser Weg war das letzte Loch". Auf dem Textbaustein nur `Clean`
  zu fahren hätte dasselbe Loch für einen neuen Träger wieder aufgemacht,
  während das Formular desselben Trägers (08-04) `CheckAll` fährt.
- **Kennungen in einem Wert reisen nicht übersetzt.** Ein Bildfeld hält eine
  Mediennummer, ein Verweisfeld eine Seitennummer; der Seitenexport macht daraus
  einen Dateinamen und eine Adresse. Der Textbaustein-Export tut das nicht — die
  dafür nötigen Nachschlagewerke werden **innerhalb** von `exportPages` und
  `importPages` gebaut, und sie von aussen erreichbar zu machen ist eine
  Änderung am Seitenweg, die dieser Plan nicht verlangt. Als **Grenze** im
  Doc-Kommentar von `bundle.Snippet` festgehalten, nicht verschwiegen: Text,
  Zahl, Datum, Auswahl und Ja/Nein reisen heil, ein Bild landet auf der anderen
  Maschine ohne sein Bild. Die Fläche ist eng — `field.Links` ist
  websitegebunden, ein baumelnder Verweis löst zu nichts auf und leckt nichts.
- **Der Wächter für die Asymmetrie wurde ergänzt.** Der Plan nennt in Task 1 nur
  `sample.go` und `TEMPLATE-SPEC.md` und behauptet, die bestehenden Prüfungen
  hielten die Asymmetrie fest. Sie tun es nicht: beide sehen `Page` an, keine
  `Site`. Eine Zusage, die kein Test rot werden lässt, ist keine — und die
  Mutationsprobe zeigt, dass dieser es tut.
- **Kein `MinimalData.Site.Snippets`.** Der Rumpf bleibt weg, obwohl die Felder
  da sind. Das ist der Fall aus dem Plan („no snippet bodies, and a snippet whose
  fields are defined and empty") und zugleich der härtere: ein Theme muss beide
  Hälften einzeln überleben.

## Zwei Zahlen, die vom Plan abweichen — beide gemeldet, keine passend gemacht

Die vier Wellen davor hatten je eine Zählung, die gegen den Baum von *vor* der
Änderung gerechnet war. Diese Welle hat zwei, und beide zeigen in dieselbe
Richtung: der Plan rechnete mit Arbeit, die frühere Pläne schon getan hatten.

| Tor | Plan | Gemessen | Ursache |
|---|---|---|---|
| `grep -c 'Site.Bausteinliste' TEMPLATE-SPEC.md` | ≥ 2 | **5** | 08-01 hatte die `.Site`-Zeile schon geschrieben; §7 fügt drei weitere Nennungen hinzu, und `FieldEntry` eine fünfte. Das Tor ist eine Untergrenze, die Zahl liegt darüber |
| `go run ./tools/i18n` neue Schlüssel | > 0, mit eigenem Commit | **0** | Der Plan erwartet „die deutschen Zeichenketten, die diese Phase auf `field_list.html` und `snippet_list.html` hinzugefügt hat". Die sind da — aber 08-03 und 08-04 haben sie **selbst schon** übersetzt und committet, jeweils als eigenen Commit nach ihrem Muster. `-write` und `-schweiz` liefen und schrieben nichts; das Tor misst `0 offen, 0 verwaist` auf vier Zeilen, `git status` auf `internal/i18n/locales/` ist leer, `fr-CH.json` und `it-CH.json` sind unberührt |

Für die zweite Zahl gilt ausdrücklich: **es wurde keine Zeichenkette erfunden**,
um einen Katalog-Commit zu erzeugen. Der Plan verlangt „Commit the catalogues on
their own" — wenn nichts wandert, ist der richtige Commit keiner.

Ein `grep`-Tor des Plans misst dagegen genau, was es soll:
`grep -v '^[[:space:]]*//' internal/template/sample.go | grep -c 'Bausteinliste'`
→ **1**, wie verlangt. Der `MinimalData`-Kommentar, der `Bausteinliste`
ausführlich bespricht, steht vollständig in Kommentarzeilen und wird
herausgefiltert — genau der Fall, den 08-02 als Muster aufgeschrieben hat.

## Deviations from Plan

### Auto-fixed Issues

**1. [Regel 2 — Fehlende kritische Funktion] Ein Wächter für die Vorrichtungs-Asymmetrie**

- **Found during:** Task 1
- **Issue:** Der Plan führt als Verhalten auf, dass
  `TestMinimalDataCarriesTheEmptyValueOfEveryOwnField` mit der neuen Vorrichtung
  „passt". Das tut sie — aber nur, weil sie ausschliesslich `Page.Felder` und
  `Page.Feldliste` ansieht. Keine Prüfung im Baum sieht die `Site`-Hälfte an.
  Die Asymmetrie, die dieser Plan als seine gefährlichste Stelle bezeichnet,
  wäre also unbewacht gewesen: ein späterer Zug, der `MinimalData` eine
  `Bausteinliste` gibt oder eine Kennung vergisst, wäre grün durchgelaufen.
- **Fix:** `TestMinimalDataCarriesTheEmptyValueOfEverySnippetField` in
  `sample_test.go`, benannt mit dem Präfix `TestMinimalData`, damit das
  `-run`-Tor des Plans ihn wirklich fährt. Er prüft beide Richtungen (keine
  Kennung fehlt, keine ist zu viel), vergleicht jeden Wert gegen `emptyValueOf`
  und fährt zusätzlich die drei Formen, die §7 vorschreibt, gegen `MinimalData`.
- **Files modified:** `internal/template/sample_test.go`
- **Verification:** Mutationsprobe in beide Richtungen: Kennung entfernt → Fall 1
  fällt; `Bausteinliste` erfunden → Fälle 2 und 3 fallen. Danach wiederhergestellt,
  Prüffolge grün. Das `-run`-Tor zählt jetzt **5** `--- PASS` statt der
  verlangten drei.
- **Committed in:** `2ff3239`

**2. [Regel 1 — Fehler] Eine Gruppe am Textbaustein zeichnete keine einzige Zeile**

- **Found during:** Task 3, Browserschritt 7 — **nicht** von der Prüffolge
- **Issue:** Der Gruppenbildschirm ist `…/felder?gruppe=<id>`, eine Ebene tiefer,
  und weiss von keinem Textbaustein. Sein Formular schickt `gruppe` und **kein**
  `textbaustein`, also legte `HandleFieldSave` das Unterfeld mit `snippet_id`
  NULL an, während seine Gruppe `snippet_id` trägt. `OfSnippet` fragt aber
  `WHERE snippet_id = $2` und gab die Gruppe danach **ohne ein einziges
  Unterfeld** heraus. Im Browser sichtbar als „Gruppe (0)" und als ein
  Textbausteinformular, auf dem die Gruppe keine Zeile zeichnen kann. In der
  Wegwerf-Datenbank nachgesehen: `id 3 → snippet_id 1`, `id 4,5 → snippet_id
  NULL`.
  Der Importweg dieses Plans setzt beides von Anfang an (der Plan schreibt es
  wörtlich vor) — Archiv und Bildschirm wären über dieselbe Gruppe uneins
  gewesen.
- **Fix:** Ein Unterfeld erbt seinen Träger aus der **gespeicherten** Gruppe und
  nicht aus dem Formular — dieselbe Form, in der `Update` seinen Träger pinnt
  (08-01), und was nicht aus dem Körper kommt, kann auch nicht gefälscht werden.
  Eine Gruppennummer, die keine Gruppe dieser Website nennt, antwortet 404.
- **Files modified:** `internal/admin/field.go`, `internal/admin/snippet_fields_test.go`
- **Verification:** `TestGruppeAmTextbausteinTraegtIhreUnterfelder`,
  mutationsgeprüft (ohne den Flick: „die Gruppe kommt ohne ihr Unterfeld zurück:
  []"), samt Gegenprobe, dass das Unterfeld nicht auf dem Seitenbildschirm
  auftaucht. Danach im Browser gegengefahren: „Gruppe (2)", Zeile hinzugefügt,
  gefüllt, gespeichert, neu geladen — **mit abgeschaltetem JavaScript**.
- **Committed in:** `3ad28ba`

**3. [Umfangsgrenze — nicht geflickt] `field_list.html` druckt `&#8592;` als Text**

- **Found during:** Task 3, Browserschritt 1
- **Issue:** Der Rückweg über dem Feldbildschirm liest wörtlich
  „&#8592; Alle Textbausteine" statt „← Alle Textbausteine". Die Zeichenkette
  steht als HTML-Entität im Quelltext, `{{t "…"}}` maskiert ihr Ergebnis, also
  erscheint die Entität selbst.
- **Warum nicht hier geflickt:** die Stelle steht **dreimal**
  (`field_list.html:8`, `:16`, `:25`), zwei der drei Arme sind älter als diese
  Phase, und die Zeichenkette **ist der Katalogschlüssel**. Sie zu ändern legt
  drei neue Schlüssel in vier Katalogen an und lässt drei alte verwaisen — das
  Tor `0 verwaist` dieses Plans fiele, bis die alten von Hand entfernt sind.
  Eine i18n-Aufräumarbeit mit eigenem Commit, keine Nebenwirkung eines
  Vorrichtungsplans.
- **Aufgeschrieben in:** `.planning/phases/08-snippets-carry-fields/deferred-items.md`
  (mit der genauen Schrittfolge) und im Fensterbuch `.planning/WINDOWS.md` als
  Eintrag 5, `status: open`.

---

**Total deviations:** 2 auto-fixed (1 × Regel 2, 1 × Regel 1), 1 bewusst
zurückgestellt und an zwei Stellen aufgeschrieben.
**Impact on plan:** Kein Zuwachs am Umfang. Abweichung 1 ist der Wächter, ohne
den die Zusage des Plans keine wäre. Abweichung 2 ist ein Fehler in der Fläche
dieser Phase, den Schritt 7 sonst gar nicht hätte fahren können — und er ist der
Beleg dafür, dass der Browserdurchgang kein Abhaken ist. Abweichung 3 ist genau
die Sorte Flick, die ein Tor einer anderen Ebene fallen liesse.

## Issues Encountered

- **Playwright statt Playwright-MCP.** Als Unteragent sehe ich keine
  MCP-Server, die im Projekt konfiguriert sind. Der Durchgang wurde stattdessen
  mit dem **bereits vorhandenen** Playwright aus dem npx-Zwischenlager gefahren
  (`require` auf einen Pfad, kein Download, kein `npx --yes`). Das Ergebnis ist
  dasselbe: ein echter Chromium, echte Schirmbilder, und für Schritt 7 ein
  Kontext mit `javaScriptEnabled: false`, was ein MCP-Werkzeug so nicht anbietet.
  Die Anwendung selbst bleibt eine einzelne Go-Binärdatei; Node ist hier
  Prüfwerkzeug und nichts im Auslieferungsstapel.
- **Zwei-Faktor ist für Administratoren Pflicht** und stand vor jedem
  Verwaltungsbildschirm. Im Durchgang mit einem selbst gerechneten TOTP-Code
  (HMAC-SHA1, 6 Stellen, 30 s) durchlaufen — kein Umgehungspfad benutzt, kein
  Code geändert.
- **Formularnamen sind deutsch** (`beschriftung`, `art`, `pflicht`,
  `gruppenaktion`), nicht englisch. Die ersten Skripte suchten `name=label` und
  liefen in einen Zeitablauf; am Baum nachgesehen und korrigiert.
- **`page.click('button[type=submit]')` traf den Abmelden-Knopf** in der
  Kopfleiste, weil er der erste im Dokument ist. Auf `getByRole('button', {name})`
  umgestellt.

## Known Stubs

Keine. Jede Stelle, die dieser Plan anfasst, trägt echte Daten. Was nicht
gebaut wurde, ist im Doc-Kommentar von `bundle.Snippet` als Grenze benannt (die
nicht übersetzten Kennungen in einem Wert) und nicht halb gebaut.

## Threat Flags

Keine neue Fläche über den `<threat_model>` des Plans hinaus. Zu den Einträgen
ausdrücklich:

- **T-08-24** ist gehalten und mit einem Tor belegt: `grep -c 'INSERT INTO
  page_field_defs' internal/bundle/*.go` → **0**, jede importierte Definition
  geht durch `field.Store.Create`, eine abgewiesene erzeugt eine Warnung und
  bricht den Import nicht ab (Prüfung
  `TestTextbausteinfelderAusDemArchivGehenDurchDieselbePruefung`).
- **T-08-25** ist gehalten und **über die Zusage hinaus**: neben `field.Clean`
  läuft `field.CheckAll`, siehe Entscheidungen.
- **T-08-26**: die `SnippetID` für `Create` ist die Nummer des Textbausteins,
  den dieser Import gerade angelegt hat; das Manifest liefert nie eine Nummer.
- **T-08-29**: der Server lief auf einem Wegwerf-Verzeichnis unterhalb des
  Sitzungspfades, das danach gelöscht wurde; kein echtes Datenverzeichnis
  berührt.
- **Neu und ausserhalb des Registers:** der Flick aus Abweichung 2 hat eine
  Berechtigungskante **verengt**, nicht erweitert — eine Gruppennummer, die zu
  keiner Gruppe dieser Website gehört, antwortet jetzt 404 statt ein Unterfeld
  unter einer fremden oder nicht existierenden Nummer anzulegen.

## User Setup Required

Keine — kein externer Dienst berührt.

## Next Phase Readiness

**Phase 8 ist fertig.** Alle fünf Pläne haben ihre Zusammenfassung, `go build
./...`, `go test ./...`, `gofmt -l .` und `go vet ./...` sind sauber, das
i18n-Tor meldet `0 offen, 0 verwaist` auf vier Zeilen, und alles, was ein Mensch
sehen kann, ist einmal in einem Browser gesehen worden — einschliesslich des
Seitenformulars, das sauber geblieben ist.

Offen und aufgeschrieben:

1. Der `&#8592;`-Befund in `field_list.html` (dreimal), mit Schrittfolge in
   `deferred-items.md` und als Eintrag 5 im Fensterbuch.
2. Die nicht übersetzten Kennungen in einem Textbausteinwert beim Archivweg —
   als Grenze im Code dokumentiert. Wer sie schliessen will, muss die
   Nachschlagewerke aus `exportPages`/`importPages` herausziehen; das ist eine
   Änderung am Seitenweg und gehört in einen eigenen Plan.

Keine Blockierer. `.planning/phases/11-galerie/` ist unberührt.

---
*Phase: 08-snippets-carry-fields*
*Completed: 2026-09-06*

## Self-Check: PASSED

Alle neun geänderten Dateien liegen auf der Platte, alle drei Commits stehen im
Verlauf (`2ff3239`, `739c5ad`, `3ad28ba`), jedes im Vertrag zugesagte Symbol ist
im Baum — `MinimalData().Site.Bausteinfelder`, `bundle.Snippet.Values`,
`importSnippetFields`, `cleanSnippetValues`, `exportFieldDef` —, und die fünfzehn
Schirmbilder liegen im Kritzelverzeichnis. Das Wegwerf-Datenverzeichnis ist
gelöscht, der Server gestoppt.
