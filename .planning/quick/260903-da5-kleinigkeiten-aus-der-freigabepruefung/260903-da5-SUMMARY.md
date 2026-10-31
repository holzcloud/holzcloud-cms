---
phase: quick-260903-da5
plan: 01
subsystem: repository-hygiene
tags: [community-docs, supply-chain, fixtures, ci]
status: complete

requires: []
provides:
  - "CODE_OF_CONDUCT.md im Wurzelverzeichnis"
  - "Issue-Formulare fehler.yml und funktion.yml plus config.yml"
  - ".github/PULL_REQUEST_TEMPLATE.md"
  - "CONTRIBUTING.md Abschnitt „Wer diesen Code geschrieben hat\""
  - "SHA-gepinnte Aktionen in allen drei Arbeitsabläufen"
affects:
  - ".github/workflows/"
  - "internal/template/sample.go (Produktivcode)"

tech-stack:
  added: []
  patterns:
    - "GitHub-Aktionen auf 40-stellige Commit-Kennung gepinnt, Fassung als Kommentar dahinter"
    - "example.ch / example.de als hausübliche Fixture-Domains"

key-files:
  created:
    - CODE_OF_CONDUCT.md
    - .github/ISSUE_TEMPLATE/fehler.yml
    - .github/ISSUE_TEMPLATE/funktion.yml
    - .github/ISSUE_TEMPLATE/config.yml
    - .github/PULL_REQUEST_TEMPLATE.md
  modified:
    - CONTRIBUTING.md
    - .github/workflows/ci.yml
    - .github/workflows/release.yml
    - .github/workflows/security.yml
    - internal/template/sample.go
    - internal/admin/orderdoc_test.go
    - internal/outbox/outbox_test.go
    - internal/payrexx/payrexx_test.go
    - internal/public/payment_test.go
    - internal/template/render_test.go
    - internal/bundle/bundle_test.go
    - internal/structured/structured_test.go
    - internal/totp/totp_test.go

decisions:
  - "D-01: eigener kurzer Verhaltenskodex statt Contributor Covenant"
  - "D-02: Meldeweg ist der bestehende private Advisory-Kanal, keine neue Adresse"
  - "D-03: Pin auf Commit-Kennung, Fassung als Kommentar, Begründung nur in ci.yml"
  - "D-04: erfundener Firmenname „Holzbau Schmidt\" bleibt stehen"
  - "D-05: die neuen Gemeinschafts-Dokumente sind auf Deutsch"

metrics:
  duration: ~35 min
  completed: 2026-09-03

actuals:
  tokens: 21000
  tasks: 3
  commits: 3
---

# Quick 260903-da5: Kleinigkeiten aus der Freigabeprüfung — Zusammenfassung

Vier leichte Befunde der Freigabeprüfung abgeräumt: die neun Fixture-Adressen, die
einer echten Schreinerei gehören könnten, auf `example.*` umgestellt; die sieben
`uses:`-Zeilen der drei Arbeitsabläufe von der beweglichen Marke `@v7` auf feste
Commit-Kennungen genagelt; dem Repository Verhaltenskodex, Issue-Formulare und
PR-Vorlage gegeben; und in CONTRIBUTING.md ausgesprochen, dass ein grosser Teil des
Verlaufs von einem KI-Agenten stammt.

## Commits

| Task | Commit    | Was                                                        |
| ---- | --------- | ---------------------------------------------------------- |
| 1    | `2dc0b93` | Die Fixture-Adressen gehören keiner fremden Werkstatt mehr  |
| 2    | `f551ccd` | Die drei Aktionen hängen an einer Kennung statt an einer Marke |
| 3    | `39658dc` | Die Gemeinschafts-Dokumente, die dem Repository noch fehlten |

## Die eigene Trefferliste aus Task 1

Der eigene `grep -rIn "holzbau\|Holzbau" --exclude-dir=.git --exclude-dir=.planning .`
bestätigte Befund A **vollständig**: dieselben neun Dateien, dazu die zwei bewusst
ausgenommenen (`internal/db/migrations/00042_outbox.sql`,
`cmd/holzcloud/templates/admin/shop_settings.html`). Keine Datei kam hinzu, keine fiel
weg. Eine Abweichung im Detail:

- **Befund A nannte „sechs Stellen" für die Payrexx-Kennung in
  `internal/payrexx/payrexx_test.go`; es waren acht** (Zeilen 108, 118, 120, 143, 163,
  224, 343, 369). Alle acht wurden umgestellt, einschliesslich der Zusicherung auf den
  Anfrageparameter `instance` in Zeile 224. Hätte man sich auf die Zahl statt auf den
  eigenen grep verlassen, wären zwei Vorkommen stehen geblieben.

`internal/tmplspec/TEMPLATE-SPEC.md` zitiert keinen der Werte wörtlich (mit eigenem grep
geprüft), die Vorlagen-Beschreibung musste also nicht mitwandern.

Ersetzt wurde ausschliesslich, was auflösen und Post annehmen kann:

| Alt | Neu | Wo |
| --- | --- | --- |
| `holzbau.ch` (Domain, `bestellungen@`, `post@`) | `example.ch` | outbox, orderdoc, payrexx |
| `holzbau-schmidt.de` | `example.de` | sample.go, render, bundle, structured |
| `holzbau.de` | `example.de` | totp |
| Payrexx-Instanz `holzbau` | `example` | payrexx, payment |

Aufbau und Zusicherung wanderten überall zusammen. Zwei Stellen verdienen die
ausdrückliche Erwähnung, weil ein halber Austausch dort einen Test still entwertet
hätte:

- **`internal/bundle/bundle_test.go` dreht die Richtung um.** Zeile 292 legt die Domain
  an, Zeile 300 sichert zu, dass das Archiv sie **nicht** enthält. Wäre nur eine der
  beiden umgestellt worden, suchte die Prüfung nach einer Zeichenkette, die nie
  eingetragen wurde — der Test bliebe für immer grün, ohne noch etwas zu prüfen.
- **`internal/outbox/outbox_test.go`** benutzte in derselben Testfunktion schon
  `anna@example.ch` für die Kundin. Nach dem Austausch bleiben Kundin
  (`anna@example.ch`) und Betrieb (`bestellungen@example.ch`) verschieden, die
  Zusicherung auf die Antwortadresse unterscheidet also weiterhin etwas.

`internal/template/sample.go` ist dabei **kein Test**, sondern der Produktivcode, gegen
den `holzcloud template check` hochgeladene Vorlagen rendert.

## D-01 in einem Satz

Ein eigener kurzer Kodex statt des Contributor Covenant, weil der Covenant „community
leaders", eine vierstufige Durchsetzungsleiter und ein Beschwerdeverfahren verspricht,
die eine einzelne Person in ihrer Freizeit nicht führen kann — und ein solches Versprechen
direkt neben SECURITY.md, das offen sagt „ich kann keine Fristen zusichern", würde das
ehrliche Dokument unglaubwürdig machen.

## D-04 in einem Satz

Der erfundene Firmenname `Holzbau Schmidt` bleibt, weil ein Name nichts auflöst und keine
Post annimmt, und weil er an zwei Stellen steht, an denen ein Austausch teurer wäre als
das Problem: im Kommentar einer bereits angewendeten Migration (CONTRIBUTING verbietet das
Anfassen ohne Ausnahme für Kommentare) und als sichtbarer Platzhalter im Admin-Formular für
die Zahlungsangaben. Ein späterer vollständiger Austausch kostet danach genau **eine**
Ersetzung über rund fünfzehn Zusicherungen in sechs Dateien, plus die Entscheidung über die
bewusst ausgenommene Migrationszeile — eine eigene Aufgabe, kein Nachtrag zu dieser.

**Die Prüfliste filtert ihn deshalb ausdrücklich heraus.** Das Gate lautet
`grep -rIn "holzbau" … | grep -vF "Holzbau Schmidt"`. Der Filter ist Absicht nach D-04 und
**keine übersehene Stelle** — ohne ihn würde die Prüfung an den zwei bewusst behaltenen
Vorkommen fehlschlagen. Wer diese Zeile später liest, soll sie nicht für Nachlässigkeit
halten.

## Ausserhalb des Umfangs — geprüft, nichts geändert

- **Maschinen-Hostnamen in der Commit-Autorschaft.** Geprüft: `git config user.email` steht
  auf `6815936+holz41289@users.noreply.github.com`, und **alle 28 Commits vom 2026-09-03**
  benutzen sie, die drei dieser Arbeit eingeschlossen. Für die Zukunft ist das erledigt.
  Die alten Einträge sind Geschichte; sie zu entfernen hiesse, den Verlauf umzuschreiben.
  **Nichts geändert.**
- **`.planning/` ist mitveröffentlicht.** Geprüft: 72 Dateien sind verfolgt, das
  Verzeichnis steht nicht in `.gitignore` (nur `.planning/state.json` ist ausgenommen).
  Das ist eine bereits getroffene Entscheidung — `commit_docs: true`, und das Herausnehmen
  bräche den GSD-Arbeitsablauf. **Nichts geändert.**

## Abweichungen vom Plan

Keine, ausser der oben genannten Zählkorrektur (acht statt sechs Payrexx-Stellen), die der
Plan durch seinen `<mutable_scope>` ausdrücklich vorgesehen hat.

## Zurückgestellt

**Ein flatternder Test, nicht von dieser Arbeit verursacht.**
`TestMarkeMitUnbekanntemArgumentBleibtDerBetreff` in `internal/public/formular_e2e_test.go`
schlug im ersten vollen Lauf fehl. Die Datei steht nicht im Umfang dieses Plans und wurde
nicht angefasst; die einzige Änderung im Paket `internal/public` betraf zwei
Zeichenketten in `payment_test.go`.

Ursache: die Zusicherung `!strings.Contains(seite, "f_")` sucht im **ganzen** Dokument nach
`f_`, während auf derselben Seite ein zufälliger base64url-Wert im Feld `gestellt` steht —
dessen Alphabet enthält `f` und `_`. Gemessen: `-count=400` ergab 5 Fehlschläge (~1,25 %),
was zur Rechnung passt (≈39 Positionen × 1/64 × 1/64 ≈ 0,95 %).

Notiert in `deferred-items.md` mit Vorschlag zur Behebung. **Nicht behoben** — ausserhalb
des Umfangs nach der Regel, dass nur Fehler angefasst werden, die die eigene Änderung
verursacht hat.

## Verifikation

Alle sieben Punkte aus `<verification>`:

| # | Prüfung | Ergebnis |
| - | ------- | -------- |
| 1 | `go build ./...`, `go vet ./...` | beide still |
| 2 | `go test ./...` | 39 ok, **0 FAIL**, 2 ohne Tests; `go list` = 41 = Grundlinie |
| 3 | gefilterter `grep` auf den Wortstamm | leer |
| 4 | ungepinnte `uses:`-Zeilen | `0` (alle sieben gepinnt) |
| 5 | YAML aller Abläufe **und** Formulare | `alles einlesbar` |
| 6 | `go run ./tools/i18n` | en/es/fr/it je `0 offen, 0 verwaist` |
| 7 | `git status --short` | nur `deferred-items.md`, keine Streuung |

Zusätzlich geprüft: `gofmt -l internal/ cmd/` leer; die Issue-Formulare gegen die
Strukturregeln von GitHub validiert (fehler.yml 4 Blöcke / 3 Pflichtfelder, funktion.yml
3 Blöcke / 1 Pflichtfeld, config.yml mit `blank_issues_enabled: true` und 2 contact_links);
`git diff` über `.github/workflows/` zeigt genau die sieben Pins plus den einen
Kommentarblock in `ci.yml`, alle vorhandenen Kommentare unverändert.

Die Gemeinschafts-Prüfliste ist vollständig: README, LICENSE, CONTRIBUTING, SECURITY,
CODE_OF_CONDUCT, PR-Vorlage, zwei Issue-Formulare und config.yml liegen alle vor.

## Known Stubs

Keine.

## Self-Check: PASSED
