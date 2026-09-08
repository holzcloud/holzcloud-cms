---
phase: 06-aufr-umen
audited: 2026-09-08
audited_at: 24fa2ee
status: OPEN_THREATS
threats_total: 27
threats_closed: 25
threats_open_nonblocking: 2
threats_open_blocking: 0
block_on: high
note: >-
  Nachgeholt. Phase 6 war die einzige Phase des Meilensteins ohne
  Sicherheitsbericht, obwohl sie 27 Bedrohungszeilen trägt.
---

# Phase 6 — Sicherheitsprüfung (nachgeholt)

## Der Befund, der alles andere überwiegt

Der Prüfer hat das Tor **gefahren** statt darüber zu lesen:

```
$ go run ./tools/wasm -check
plugins/kontaktformular/plugin.wasm ist nicht aktuell
  im Repository: 1bde1f71…  gebaut mit go1.26.6
  neu gebaut:    0800580d…  gebaut mit go1.26.6
2 Datei(en) sind nicht aktuell.        exit 1
```

Drei Dinge folgen daraus, alle in derselben Sitzung geprüft:

1. **Die Abweichung war echt, kein Reproduzierbarkeitsartefakt.** Elf Artefakte
   zweimal in getrennte Verzeichnisse gebaut, alle elf Paare byte-gleich. Eine
   Abweichung ist also Signal. `plugins/kontaktformular/csv.go` wurde am
   2026-09-06 von Phase 9 geändert und das Modul nie neu gebaut — das
   eingecheckte Binärartefakt bürgte für Quelltext, aus dem es nicht gebaut war.
   Genau `T-06-01`, `T-06-12`, `T-06-16`.
2. **Das Tor hat auch auf dem Laufer angeschlagen.** Derselbe Befund im letzten
   CI-Lauf (`d4ca500`), auf ubuntu-latest/amd64 gegen darwin/arm64 hier — die
   rechnerübergreifende Byte-Gleichheit (D-05) hält also weiter, und der Schritt
   hat den Auftrag mit exit 1 abgebrochen.
3. **Es hat nur niemand gehandelt.** `origin/main` steht rot auf `d4ca500`.

**Das ist keine Lücke von Phase 6 — es ist der Grund, aus dem sie sichtbar war.**
Behoben am 2026-09-08 (`4580143`, danach `3aca298`).

## Geschlossen (25 von 27)

Die tragenden: `T-06-01`/`T-06-16` (CI baut neu und vergleicht, ohne
`continue-on-error`, vor `Build` und `Test`), `T-06-03` (die Bauumgebung wird
gesteuert und nicht ergänzt — `GOENV=off`, leere `GOFLAGS`/`GOEXPERIMENT`/
`GOWASM`/`GOFIPS140`), `T-06-12` (fester Zeitstempel 1980-01-01, in den
eingecheckten Archiven nachgesehen), `T-06-13` (`atomarSchreiben` legt die
Zwischendatei im Zielverzeichnis an und benennt um), `T-06-21`/`T-06-22`
(`HOLZCLOUD_TEST_REQUIRE_WASM` lässt einen fehlenden Gast scheitern statt
überspringen, in drei Arbeitsabläufen gesetzt), `T-06-07` (Katalogformat durch
Rundlauftest plus `git diff --exit-code` in CI).

`T-06-02` ist ein **Transfer**, und er wurde in der Go-Quelle nachgeprüft statt
in der Dokumentation: `modfetch/sumdb.go` erzwingt die Prüfsummendatenbank für
`golang.org/toolchain` auch bei `GOSUMDB=off`.

## Offen, nicht blockierend (2)

**T-06-06 — die akzeptierte Begründung stimmt nicht.** Sie lautet: „die
Zeichenketten erreichen eine Seite über `html/template`, das beim Rendern
maskiert". Für `th` (`internal/web/render.go:95`) ist das falsch — der
Katalogwert wird nach `template.HTML` gecastet und gerade **nicht** maskiert,
an über zehn lebenden Aufrufstellen. Das Restrisiko bleibt klein (ein Katalog
ist eingecheckter Quelltext, und `script-src 'self'` verbietet Ausführung), und
`render.go:88-94` trägt ein **richtiges** Vertrauensargument. Aber die Annahme
steht auf einer Prämisse, der der Code widerspricht, und ist so nicht gültig.

**T-06-14 — die Minderung hat zwei Stellen nicht erfasst.** Die drei genannten
Dateien sind sauber; es gibt eine **vierte** niedergeschriebene Bauform
(`sdk/plugin.go`) und eine Anleitung mit `zip -j` (`plugins/README.md`), die
nicht die Bytes erzeugen kann, die das Tor vergleicht, und ein
`migrations/`-Verzeichnis still verliert. **Behoben am 2026-09-08** (`539a3ca`).

## Warnungen — nicht registrierte Fläche

- **W-1**: `T-06-02`s Transfer hat zwei geerbte Umgebungslücken —
  `GOPROXY=file://…` und `GIT_HTTP_USER_AGENT` umgehen die Prüfsummendatenbank,
  und beide stehen nicht in `gesteuert`.
- **W-2**: `image.yml` pinnt fünf fremde Actions per **Tag**, während `ci.yml`
  danebenschreibt, dass ein Tag verschoben werden kann. Der Auftrag hält
  `packages: write`. Vorbestehend, von keiner Bedrohungszeile abgedeckt.
- **W-3**: ein Abnahmekriterium von Phase 6 wurde nie gemessen — „höchstens 6
  geänderte Zeilen" gegen tatsächlich **32**. Die Substanz hält (der Diff wurde
  gelesen), die Zahl wurde still gerissen. Sechste Instanz von „ein Tor muss
  messen, was sein Name behauptet" — in der Phase, die es reparieren sollte.
- **W-4**: drei Zahlen im Doku-Kommentar von `tools/wasm` sind falsch (vier statt
  fünf Archive, zehn statt elf Artefakte, „flach" für ein Archiv mit
  `migrations/`). Und die am 2026-09-04 berichtigten Codekarten sind schon
  wieder abgedriftet: 45 Migrationen dort, **51** auf der Platte. `T-06-09`/
  `T-06-10` haben eine punktuelle Reparatur gekauft, keinen Wächter.
- **W-6**: `ziele` in `tools/wasm` ist eine Literaltabelle ohne Abgleich gegen
  `plugins/` — ein sechstes Verzeichnis stünde still ausserhalb des Tors.
- **W-7**: `tools/i18n` schreibt mit `os.WriteFile` in place, während CI den
  schreibenden Pfad bei jedem Push fährt; das Schwesterwerkzeug im selben
  Commitbereich benutzt `atomarSchreiben`.
- **W-8**: sechs von sieben Berichten tragen gar keinen Abschnitt
  `## Threat Flags`. Es fehlte nichts — aber die ausführerseitige Hälfte der
  Bedrohungsschleife lief für sechs von sieben Plänen nicht.

## Archivbehandlung

Phase 6 hat die Leseseite **nicht** geweitet — der Diff berührt keinen
Importeur. Alle drei Zip-Türen sind geprüft und geschützt:
`tmplmgr/upload.go` (Clean + Präfixeinschluss), `plugin/package.go`
(`safeRelative`), `bundle/import.go` (`safeName`). Was die Phase auf der
Archivseite geändert hat, machte den **Schreib**pfad strenger: `packen` liest
seither ein Verzeichnis statt zweier fester Einträge, weshalb
`kontaktformular.zip` seine Migration überhaupt trägt.
