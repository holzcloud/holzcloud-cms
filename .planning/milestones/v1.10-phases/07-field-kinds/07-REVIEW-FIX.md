---
phase: 07-field-kinds
review_path: .planning/phases/07-field-kinds/07-REVIEW.md
fixed: 2026-09-05
fix_scope: critical+warning
iteration: 1
status: partial
findings:
  addressed: 4
  remaining: 8
  out_of_scope: 3
---

# Phase 7 — was aus dem Code-Review behoben wurde

Der Fixer lief in einem eigenen Worktree und wurde vom Nutzungslimit
abgebrochen, nachdem er vier Befunde committet hatte. Die Arbeit lag
vollständig auf `gsd-reviewfix/07-97067`, wurde geprüft und ist auf `main`
übernommen. Dieser Bericht wurde vom Orchestrator nachgetragen — der Fixer kam
nicht mehr dazu.

## Behoben (4 von 12 im Umfang)

| Befund | Commit | Was geändert wurde |
|---|---|---|
| **CR-01** Mehrfachauswahl im Baustein verwirft jedes Häkchen | `bf4abdd` | `page_blocks.go:249` prägt den Namen jetzt mit `d.NameSuffix()` statt `"[]"` von Hand. Der Bausteineditor war der dritte Ort, an dem Formularnamen entstehen, und der einzige, der die Markierung nicht kannte. D-03s Regel „eine Prägestelle" bleibt damit gewahrt. Regressionstest: `TestMehrfachauswahlInEigenerBausteinartUeberlebtDasSpeichern`. |
| **CR-02** Bundle-Import verliert jedes Schlagwort ab dem dreizehnten | `8111a76` | Neue Funktion `term.Normalize` — die Hälfte von `Parse`, die von einem einzelnen Namen handelt. Der Archivpfad normalisiert jeden Manifest-Namen für sich, statt die ganze Liste durch den Seiten-Leser zu schicken. `term.Parse` und `MaxPerPage = 12` bleiben unangetastet: die Konstante ist für ihren eigenen Zweck richtig. |
| **WR-01** `MaxValueBytes` galt für kein Feld, das `CheckAll` überspringt | `60cd2e3` | `field.Clean` sieht in `admin/page.go` jetzt dieselbe Feldmenge wie `CheckAll`. Der KI-Pfad hatte das schon richtig, der Admin-Pfad nicht. |
| **WR-02** Mehrfachauswahl ohne Möglichkeiten blockiert jedes Speichern | `f63bc6e` | `validate` verlangt Möglichkeiten auch für `KindMulti`, und der Hinweis am Feld nennt jetzt beide Arten statt nur „Auswahl". |
| *(Folgearbeit)* | `7d76b98` | Die Kataloge nach WR-02s erweitertem Hinweis: alter Satz verwaist, neuer offen, beides in en/es/fr/it nachgezogen, `de-CH` neu abgeleitet und der verwaiste Schweizer Eintrag entfernt. |

## Offen (8, nicht angefasst)

Der Lauf brach nach WR-02 ab. Diese acht sind **nicht** geprüft und **nicht**
verworfen — sie stehen unverändert in `07-REVIEW.md`:

- **WR-03** `bereich` nimmt serverseitig ein Dezimalkomma an, das sein eigenes Steuerelement nicht tragen kann
- **WR-04** `Def.Key` wird nie auf seine Form geprüft, die `[]`-Markierung kann nach einem Bundle-Import kollidieren
- **WR-05** Die Unterscheidung „geleert gegen unberührt", für die der Wächter existiert, überlebt nicht bis in den Speicher
- **WR-06** Ein abgelehnter Gruppenwert lässt eine leere Zeile in der importierten Seite zurück
- **WR-07** Die Spezifikation verspricht eine `bereich`-Grenze, die das Programm nicht hält
- **WR-08** `renderOwn` bekam keinen `KindMulti`-Zweig, obwohl `PlainText` einen bekam
- **WR-09** `report.Terms` ist nicht die Zahl der angelegten Schlagwörter
- **WR-10** `JoinValues` verteidigt das Trennzeichen nicht, das ihm gehört

**WR-04, WR-05 und WR-10 hängen zusammen** und berühren die Zentralmechanik
aus D-02/D-03/D-11. Sie verdienen eher eine geplante Runde als einen Fix —
insbesondere WR-05, weil Phase 9 angewiesen ist, die Unterscheidung zu erben,
die es laut Befund gar nicht bis in den Speicher schafft.

## Ausserhalb des Umfangs

IN-01 bis IN-03 sind Hinweise und waren nie im Fix-Umfang (`critical+warning`).

## Prüfstand nach der Übernahme

```
go build ./...        sauber
go test ./...         0 Fehlschläge
gofmt -l .            still
go vet ./...          still
go run ./tools/i18n   en/es/fr/it je 1151 übersetzt, 0 offen, 0 verwaist
                      de-CH 54 Abweichungen, 0 ohne Gegenstück
```
