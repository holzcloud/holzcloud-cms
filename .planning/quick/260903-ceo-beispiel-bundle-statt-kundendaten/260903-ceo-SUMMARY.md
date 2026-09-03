---
phase: quick-260903-ceo
plan: 01
subsystem: bundle
tags: [open-source-readiness, bundle-format, testdaten, datenschutz]
status: complete

requires: []
provides:
  - "sites/beispiel — ein erfundenes Bundle, das jeden Teil des Formats einmal zeigt"
  - "tools/mkbundle/pack_test.go — packt das Beispiel bei jedem go test"
affects:
  - "sites/README.md als Einstieg ins Bundle-Format"
  - "Testfixtures in internal/mail, internal/i18n, internal/plugin, internal/public, internal/admin"

tech-stack:
  added: []
  patterns:
    - "Platzhalterbilder aus der Standardbibliothek (image/jpeg), Generator bewusst nicht eingecheckt"
    - "Regressionstest gegen ein handgeschriebenes Manifest über DisallowUnknownFields"

key-files:
  created:
    - sites/beispiel/holzcloud.json
    - sites/beispiel/media/werkstatt-01.jpg
    - sites/beispiel/media/velo-01.jpg
    - tools/mkbundle/pack_test.go
  modified:
    - sites/README.md
    - tools/mkbundle/main.go
    - internal/mail/mail.go
    - internal/mail/mail_test.go
    - internal/web/layoutdata.go
    - internal/i18n/i18n_test.go
    - internal/plugin/store_test.go
    - internal/plugin/manager_test.go
    - internal/plugin/sdk_e2e_test.go
    - internal/public/suche_e2e_test.go
    - internal/public/formular_e2e_test.go
    - internal/admin/page_blocks_test.go
    - cmd/holzcloud/templates/public/weide/layout.html
  deleted:
    - "die zwei Kundenbündel unter sites/ — 101 Dateien, 33 MB"

decisions:
  - "D-01 Der Bildgenerator bleibt ein Wegwerfprogramm; das Rezept steht als Prosa im README"
  - "D-02 Der Kennungsdurchgang geht bis zum Doc-Kommentar in internal/mail/mail.go"
  - "D-03 Das Beispiel wird über mkbundle geprüft, nicht über einen echten Import"
  - "Kein terms im Beispiel, solange mkbundle Namen gegen Slugs prüft (siehe deferred-items.md)"

metrics:
  duration: "8 Minuten"
  completed: 2026-09-03

actuals:
  tokens: 6760
  tasks: 3
  commits: 3
---

# Quick 260903-ceo: Beispiel-Bundle statt Kundendaten — Summary

Unter `sites/` lagen zwei echte Betriebe; jetzt liegt dort eine erfundene
Velowerkstatt, die `tools/mkbundle` vollständig durchläuft und jeden Teil des
Formats genau einmal zeigt.

## Was gemacht wurde

**Task 1 — `sites/beispiel` (`d2ade94`).** Zuerst der dünne Pfad: Verzeichnis,
zwei Bilder, ein Minimalmanifest mit einer Seite, `mkbundle` laufen lassen. Erst
nach dem grünen Durchlauf die Prosa. Das Beispiel hat vier veröffentlichte
Seiten (`home`, `werkstatt`, `service`, `kontakt`), ein Menü `Hauptmenü` an
`main` mit einem Unterpunkt (`service` unter `werkstatt`), je einen Punkt vom
Typ `url` und `page`, den Textbaustein `oeffnungszeiten`, zwei Medieneinträge
mit Beschreibung und Bildunterschrift, ein Titelbild und zwei Pfade der Form
`/media/0/`. Die Schlüssel stammen aus den json-Tags in
`internal/bundle/format.go`, gelesen zur Ausführungszeit — nicht aus einer Liste
im Plan und nicht aus einem der entfernten Manifeste.

Erfunden ist alles: `kontakt@example.com` (RFC 2606), `Musterweg 1`,
Postleitzahl `0000` (in der Schweiz ungültig), `Musterhausen`. Zwei Seiten sagen
im eigenen Text, dass es diese Werkstatt nicht gibt.

**Task 2 — die Entfernung (`45aff40`).** `git rm -r` auf beide
Kundenverzeichnisse: 101 Dateien, 33 MB Fotos. Danach der Durchgang durch die
zwölf Dateien, die der eigene grep zur Ausführungszeit fand — die elf aus
Finding C plus das Nutzungsbeispiel im Paketkommentar von `mkbundle`, das auf
ein Verzeichnis zeigte, das es nicht mehr gibt. `sites/README.md` ist um das
Beispiel herum neu geschrieben.

**Task 3 — `tools/mkbundle/pack_test.go` (`c7513ae`).** Zwei Tests, beide
absichtlich zum Scheitern gebracht, bevor sie blieben.

## Die zwei Fixtures, die keine blinde Ersetzung vertrugen

- `internal/mail/mail_test.go` prüft, dass ein Anzeigename mit Komma in
  Anführungszeichen gesetzt wird — sonst spaltet das Komma die Adressliste.
  der Kundenname → `Velowerkstatt, Musterhausen`, Komma erhalten, und die
  erwartete Kopfzeile in derselben Datei mitgezogen. Ohne das zweite hätte der
  Test auf einen Namen geprüft, den niemand mehr übergibt.
- `internal/i18n/i18n_test.go` prüft, dass `Tf` den **Rahmen** übersetzt, bevor
  er ihn füllt. Der Gedankenstrich in `"Seiten – %s"` gehört zum Rahmen und ist
  der Katalogschlüssel — er blieb unangetastet. Gewechselt hat nur das
  Argument, und mit ihm beide Erwartungen (`Pages – Velowerkstatt` und die
  deutsche für die unbekannte Sprache).

Überall sonst war der Wechsel mechanisch: Fixture-Namen, ein Testhost
(der Kunden-Testhost → `velowerkstatt.test`), ein Markdown-Fixture und zwei
Doc-Kommentare. Der Kommentar in `weide/layout.html` nannte eine Schwesterseite
beim Namen und beschreibt jetzt die Lage allgemein, mit denselben zwei
Begründungen.

## Dass die Tests Zähne haben, wurde nachgewiesen

Ein Regressionstest, der nicht scheitern kann, ist nichts wert. Beide wurden
vor dem Commit gebrochen und wiederhergestellt:

| Probe | Ergebnis |
|---|---|
| `opening_hours` in `internal/bundle/format.go` umbenannt | beide Tests FAIL — `json: unknown field "opening_hours"` |
| `mime_type` von `velo-01.jpg` auf `image/png` gelogen | FAIL — `velo-01.jpg is jpeg data but the manifest declares image/png` |

Die zweite schliesst die Lücke, die `mkbundle` offen lässt: es glaubt dem
`mime_type` und schaut die Bytes nie an.

## Entscheidungen

**D-01 — der Bildgenerator ist nicht eingecheckt.** Er hätte genau eine
Aufgabe, die sich nie wiederholt, und ein zweiter Lauf würde die Bytes und
damit jede Prüfsumme still neu schreiben. Ein Bild ist Inhalt, kein
Bauergebnis. Das Rezept steht als Prosa im README: zwei flächige JPEGs,
1200×800, nur `image`, `image/color`, `image/draw`, `image/jpeg`, Qualität 80.

**D-02 — der Durchgang ging bis zum letzten Kommentar.** Ein Firmenname in
einem Doc-Kommentar ist keine Personendate und wäre für sich vertretbar
gewesen. Aber die Abnahmehürde ist ein leerer grep, und zehn von elf Stellen
hätten die elfte der nächsten Prüfung überlassen.

**D-03 — geprüft über `mkbundle`, nicht über einen echten Import.** Ein Test,
der wirklich importiert, bräuchte eine Datenbank, den vollen `Stores`-Satz und
eine Kopie entweder der Packlogik oder von `newStores`. `mkbundle` dekodiert in
dieselbe `bundle.Manifest` wie der Import, mit `DisallowUnknownFields` obendrauf.
Der ungeprüfte Rest ist, dass beide Seiten dieselbe Struktur verwenden — das
garantiert der Compiler.

## Abweichungen vom Plan

**[Regel 3 — blockierend] Kein `terms` im Beispiel-Bundle.** Der Plan nennt
`terms` als Beweis dafür, dass die Schlüsselliste aus der Quelle kommen muss,
verlangt es aber nicht im Beispiel. Beim Schreiben zeigte sich, warum es besser
draussen bleibt: `export.go:256` schreibt in `page.terms` den **Namen**,
`import.go:547` liest ihn als Namen, aber `mkbundle` prüft gegen die **Slugs**.
Beide möglichen Schreibweisen wären im Beispiel falsch gewesen — die eine
scheitert an `mkbundle`, die andere importiert ein kleingeschriebenes
Schlagwort. Ein Beispiel wird kopiert, also kam keine hinein. Der Befund ist
vorbestehend und nicht von dieser Aufgabe verursacht; er steht in
`deferred-items.md` statt im Diff.

`.gitignore` blieb unangetastet: Finding D bestätigt sich, `/sites/*.zip`
deckt das Archiv ab (`git check-ignore -v` nennt Zeile 40).

## Die Geschichte wurde nicht umgeschrieben

**`git rm` entfernt die Dateien aus HEAD, sonst nichts.** Die 33 MB Fotos, die
Texte, die Mail- und die Postadresse der beiden Betriebe sind über die alten
Commits weiterhin erreichbar — `git log` und jeder vorhandene Klon haben sie.
Wer dieses Repository klont, lädt sie mit herunter. Das ist so entschieden
(T-ceo-03, angenommen): ein `filter-repo`-Durchgang steht vorerst nicht an.
Niemand sollte diese Zusammenfassung so lesen, als seien die Daten weg. Weg
sind sie aus dem Arbeitsbaum.

## Verifikation

| Prüfung | Ergebnis |
|---|---|
| `go run ./tools/mkbundle sites/beispiel` | exit 0, schreibt `sites/beispiel.zip` |
| `git check-ignore sites/beispiel.zip` | ignoriert via `.gitignore:40` |
| `go build ./...` | ok |
| `go vet ./...` | ok |
| `go test ./...` | **39 ok / 0 FAIL** (Basis 38, plus `tools/mkbundle`) |
| `go run ./tools/i18n` | en/es/fr/it je 1128 übersetzt, 0 offen, 0 verwaist |
| grep der fünf Kennungen ausserhalb `.planning/` | leer (exit 1) |
| `git status --porcelain` | leer |

## Bekannte Grenzen

- Das Beispiel ist über `mkbundle` bewiesen, nicht über einen laufenden Import
  (D-03).
- `terms` kommt im Beispiel nicht vor, solange der Slug-gegen-Name-Befund offen
  ist (`deferred-items.md`).
- Die Bilder zeigen nichts. Sie sind Rechtecke auf einer Fläche und sollen nur
  belegen, dass Medien mitreisen und der Typ stimmt.

## Self-Check: PASSED

Alle vier erzeugten Dateien liegen auf der Platte, alle drei Commits sind in
`git log` erreichbar (`d2ade94`, `45aff40`, `c7513ae`), und die zwei
Kundenverzeichnisse sind aus dem Arbeitsbaum verschwunden.
