# Deferred items — 260902-cml

> **Erledigt am 2026-09-03 — Schnellaufgabe `260903-bsk`:** der erste der beiden hier vorgeschlagenen Wege wurde genommen; die vier grossen Kataloge liegen jetzt bündig links wie alle sieben, das Format von `writeCatalog` gilt, festgehalten als abgenommener Eintrag 1 in `.planning/WINDOWS.md`.

## tools/i18n schreibt die Kataloge anders, als sie im Repository liegen

`writeCatalog` in `tools/i18n/main.go` schreibt jede Zeile ohne Einrückung.
`en.json`, `es.json`, `fr.json` und `it.json` liegen aber mit zwei Leerzeichen
Einrückung im Repository, `de-CH.json`, `fr-CH.json` und `it-CH.json` ohne.
Ein `go run ./tools/i18n -write` formatiert die vier grossen Dateien deshalb
komplett um: 4408 gelöschte und 4504 eingefügte Zeilen für 24 neue Sätze.

Nicht in dieser Aufgabe angefasst — das ist eine Entscheidung über das
Dateiformat, nicht über Übersetzungen. Zwei mögliche Wege:

1. Die vier Dateien einmal bewusst mit `-write` normalisieren, in einem
   eigenen Commit, der nichts anderes tut.
2. `writeCatalog` zwei Leerzeichen einrücken lassen und `de-CH.json`,
   `fr-CH.json`, `it-CH.json` einmal mitziehen.

Solange beides offen ist, muss jeder, der `-write` benutzt, wissen, dass die
Umformatierung im Diff mitkommt.
