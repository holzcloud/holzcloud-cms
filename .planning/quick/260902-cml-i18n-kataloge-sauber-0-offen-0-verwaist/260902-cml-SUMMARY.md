---
phase: quick-260902-cml
plan: 01
subsystem: i18n
status: complete
tags: [i18n, katalog, tooling, admin, shop]
requires:
  - tools/i18n (Extraktor und Katalog-Schreiber)
  - internal/i18n (Parse, verbs, sanitise, catalog_test.go)
provides:
  - "vollständige en/es/fr/it-Kataloge: 1126 übersetzt, 0 offen, 0 verwaist"
  - "zwei zuvor unsichtbare Quelltext-Sätze wieder im Zugriff des Extraktors"
affects:
  - internal/admin (Flash-Meldung beim Entfernen einer Inhaltsart)
  - cmd/holzcloud/templates/admin (Hinweis im Vorlagen-Upload)
tech-stack:
  added: []
  patterns:
    - "Ein übersetzbarer Satz steht als ein einziges, doppelt gequotetes Go-Literal da — kein Backtick, keine +-Verkettung, sonst sieht ihn tools/i18n nicht."
key-files:
  created: []
  modified:
    - cmd/holzcloud/templates/admin/template_upload.html
    - internal/admin/kind.go
    - internal/i18n/locales/en.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json
    - internal/i18n/locales/de-CH.json
decisions:
  - "Die drei gemeldeten Verwaisten wurden einzeln geprüft statt pauschal gelöscht: nur zwei waren wirklich tot, der dritte war ein lebender Satz in einer Form, die der Extraktor nicht liest. Vier funktionierende Übersetzungen blieben dadurch erhalten."
  - "Die 24 neuen Einträge wurden von Hand in die bestehende Dateiform eingefügt statt mit `-write` geschrieben: der Schreiber in tools/i18n rückt nicht ein, die vier Dateien im Repository schon, und ein `-write` hätte 4400 Zeilen umformatiert und die 24 Übersetzungen darin begraben."
  - "Die Terminologie folgt dem Katalog, nicht der alten Bruchstück-Übersetzung: Kennung ist key / identificador / identifiant / identificatore, Inhaltsart ist content kind / tipo de contenido / type de contenu / tipo di contenuto."
metrics:
  duration: "~25 min"
  completed: "2026-09-02"
actuals:
  tokens: 18900
  tasks: 3
  commits: 3
---

# Quick 260902-cml: i18n-Kataloge sauber (0 offen, 0 verwaist) Summary

`go run ./tools/i18n` meldet wieder `0 offen, 0 verwaist` für en, es, fr und it — erreicht, indem zwei lebende Sätze am Quelltext wieder extrahierbar gemacht wurden statt ihre Übersetzungen wegzuwerfen.

## Was gemacht wurde

### Task 1 — Die drei Verwaisten schliessen (Commit `0a25b76`)

Die Analyse im Plan hat sich Zeile für Zeile bestätigt:

| Gemeldet als verwaist | Befund | Behandlung |
|---|---|---|
| `Du musst keine Vorlage von Hand schreiben. …` | lebt, stand aber in Backticks | Literal auf doppelte Anführungszeichen umgestellt, die beiden inneren `"` des `href` als `\"` escapt. Vier Übersetzungen erhalten. |
| `Inhaltsart entfernt. Die %d Einträge … Kennung „%s“ — ` | Bruchstück einer `+`-Verkettung | Verkettung in `internal/admin/kind.go` zu einem Literal gefaltet; das Bruchstück aus allen vier Katalogen entfernt. |
| `Text dieser Fassung ansehen` | echte Umformulierung (6381da3) | aus allen vier Katalogen entfernt. |

Der Bericht nach Task 1 lautete exakt wie vorhergesagt: `0 verwaist`, `24 offen`.

### Task 2 — 24 Sätze übersetzen (Commit `20bf357`)

Der tatsächliche Worklist war nicht der im Objective vermutete (CSV-Download, Übersetzungsmatrix, Fassungsvergleich, Aktivitätsprotokoll — die sind längst übersetzt), sondern Laden und Bestellungen: Preis- und Betragsprüfung, Schweizer Steuersätze, Zahlungsabgleich beim Anbieter, Versandmeldung, wieder eingereihte Nachricht, Shop-Einstellungen, Produkt gespeichert/gelöscht. Dazu der in Task 1 zusammengefügte Satz.

Vor dem Übersetzen wurde die bestehende Terminologie aus dem Katalog gelesen (`Kennung`, `Inhaltsart`, `Laden`, `Editor`, `Einstellungen`, `Adresse`), damit die neuen Sätze nicht neben den alten stehen. Anrede folgt dem Bestand: Spanisch und Italienisch duzen (`Indica`, `Scegli`), Französisch siezt (`Veuillez indiquer`, `Choisissez`). Anführungszeichen folgen der Zielsprache: `“ ”` englisch, `« »` spanisch und italienisch, `« »` mit Innenabstand französisch.

Vor dem Schreiben geprüft, im Skript, für alle 96 Werte:
- kein leerer Wert, kein Wert gleich dem deutschen Schlüssel
- `verbs()` — Nachbau des Helfers aus `internal/i18n/disk.go` — identisch zur deutschen Fassung, also `%d` vor `%s` in derselben Reihenfolge
- kein `<` und kein `>` in irgendeinem neuen Wert (siehe Sicherheit unten)

### Task 3 — Schweizer Fassung nachziehen (Commit `d914492`)

`go run ./tools/i18n -schweiz` nach den Basis-Katalogen, wie der Plan es verlangt (`TestFassungKeysExistInTheSource` vergleicht gegen deren Vereinigung). Ergebnis: **eine** Zeile mehr — der Inhaltsart-Satz, dessen `„%s“` regelgemäss zu `«%s»` wird. Nichts von Hand entfernt, kein Wachstum zur Volldatei. `48 nach Regel, 3 von Hand` — die drei Handkorrekturen (Natelfoto, Velo, …) sind unverändert stehen geblieben.

## Sicherheit

`internal/web/render.go:90` castet einen `th`-Wert ohne Sanitisierung nach `template.HTML`, ein Katalogwert ist also vertrauenswürdiges Markup. Behandlung:

- **T-cml-01 (Tampering):** Kein einziger der 96 neuen Werte enthält `<` oder `>` — maschinell geprüft, nicht nur beabsichtigt. Keine neuen Tags, kein Attribut, nichts Skriptartiges.
- **T-cml-02 (Information disclosure):** Kein neuer Wert enthält eine URL oder einen Pfad. Der einzige Wert mit einem `href` ist der Hinweis aus Task 1(a) — dessen vier Übersetzungen wurden nicht angefasst, `/admin/templates/spec` steht dort unverändert.
- **T-cml-03 (Denial of service):** Platzhalter-Parität im Skript geprüft und danach von `TestPlaceholdersMatchTheGerman` bestätigt. Der einzige Satz mit Verben ist der Inhaltsart-Satz; `%d` steht in allen vier Sprachen vor `%s`.

Ergänzend beobachtet, nicht geändert: `sanitise` in `internal/i18n/disk.go:194` erlaubt nur Inline-Tags ohne Attribute — ein `<a href>` in einem *übersetzten* Wert wird beim Laden entfernt, während der deutsche Quelltext-Satz sein `<a>` behält. Das betrifft den Vorlagen-Upload-Hinweis und ist Bestand, kein Ergebnis dieser Aufgabe.

## Abweichungen vom Plan

**1. [Rule 3 — blockierend] `-write` nicht als Schreibweg benutzt**

- **Gefunden bei:** Task 2
- **Sachverhalt:** Der Plan sagt, `go run ./tools/i18n -write` schreibe die Dateien „in der kanonischen sortierten Form", so dass Formatierungsdrift verschwinde. Tatsächlich schreibt `writeCatalog` ohne Einrückung, die vier Dateien liegen aber mit zwei Leerzeichen Einrückung im Repository. Ein `-write` erzeugte einen Diff von **4408 gelöschten und 4504 eingefügten Zeilen** für 24 neue Übersetzungen.
- **Warum das blockiert:** Die Nebenbedingung dieser Aufgabe verlangt, dass jeder übersetzte Wert als vertrauenswürdiges Markup geprüft wird. In einem 4500-Zeilen-Diff ist das nicht prüfbar.
- **Behandlung:** `-write` einmal laufen lassen, um die fehlenden Schlüssel *auszulesen*, dann `git checkout` der vier Dateien. Die 24 Einträge wurden anschliessend sortiert in die bestehende Form eingefügt. Ergebnis: exakt 24 eingefügte Zeilen pro Datei, keine einzige geänderte.
- **Dateien:** en.json, es.json, fr.json, it.json
- **Commit:** `20bf357`
- **Nachwirkung:** in `deferred-items.md` festgehalten — die Formatuneinigkeit zwischen `writeCatalog` und den Dateien im Repository bleibt bestehen und gehört in einen eigenen Commit.

Keine weiteren Abweichungen. Keine Architekturentscheidung, keine Rückfrage nötig, keine neue Abhängigkeit, `go.mod` unberührt.

## Known Stubs

Keine. Kein leerer Wert, kein Platzhaltertext, kein TODO in einer der geänderten Dateien.

## Threat Flags

Keine. Kein neuer Endpunkt, kein neuer Auth-Pfad, kein Dateizugriff, keine Schemaänderung. Die Aufgabe hat Textwerte und zwei Literalformen geändert.

## Verifikation

| Prüfung | Ergebnis |
|---|---|
| `go run ./tools/i18n` | `en/es/fr/it: 1126 übersetzt, 0 offen, 0 verwaist` ✓ · `de-CH 51 / fr-CH 4 / it-CH 9 Abweichungen, je 0 ohne Gegenstück` ✓ |
| `go build ./...` | sauber ✓ |
| `go vet ./...` | sauber ✓ |
| `go test ./...` | 38 Pakete `ok`, 0 FAIL ✓ |
| `git diff --stat` gegen `6ca4e0e` | 7 Dateien, 99 eingefügt, 11 gelöscht ✓ |
| Löschungsprüfung je Commit | keine Datei gelöscht ✓ |

## Commits

| Commit | Betreff |
|---|---|
| `0a25b76` | Zwei Sätze, die der Extraktor verloren hatte, wieder sichtbar machen |
| `20bf357` | Die 24 offenen Sätze aus Laden und Bestellungen übersetzen |
| `d914492` | Die Schweizer Fassung nachziehen |

## Self-Check: PASSED

Alle sieben geänderten Dateien vorhanden, alle drei Commit-Hashes in `git log --all` gefunden.
