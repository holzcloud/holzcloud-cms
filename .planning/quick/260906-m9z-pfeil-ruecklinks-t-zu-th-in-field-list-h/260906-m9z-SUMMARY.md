---
phase: quick-260906-m9z
plan: 01
subsystem: admin-ui, i18n, planning-akte
tags: [i18n, html-template, admin, windows-ledger, deferred-items]
status: complete
requires: []
provides:
  - "field_list.html reicht seine drei Rueckwege-Zeichenketten als template.HTML durch"
  - "WINDOWS.md Eintrag 5 geschlossen, open_count 2"
affects:
  - cmd/holzcloud/templates/admin/field_list.html
tech-stack:
  added: []
  patterns:
    - "th statt t fuer eine Katalogzeichenkette, die eigenes Markup traegt — dieselbe Bedingung wie website_list.html:5/:7"
key-files:
  created:
    - .planning/quick/260906-m9z-pfeil-ruecklinks-t-zu-th-in-field-list-h/260906-m9z-SUMMARY.md
  modified:
    - cmd/holzcloud/templates/admin/field_list.html
    - .planning/WINDOWS.md
    - .planning/phases/08-snippets-carry-fields/deferred-items.md
decisions:
  - "Der Flick wechselt die aufrufende Funktion, nicht die Zeichenkette — deshalb verwaist kein Katalogschluessel und kein Katalog wurde angefasst"
  - "Die falsche Aufschubbegruendung wird berichtigt, nicht geloescht; die ueberholte Schrittfolge bleibt durchgestrichen stehen"
metrics:
  duration: 13 min
  completed: 2026-09-06
actuals:
  tokens: 3500
  tasks: 2
  commits: 2
---

# Quick 260906-m9z: Pfeil auf den Ruecklegen des Feldbildschirms Summary

Drei Zeichen in einer Vorlage — `{{t` wird `{{th` — und zwei Akteneintraege,
deren Aufschubbegruendung auf einer falschen Annahme ruhte, sagen jetzt selbst,
worin die Annahme falsch war.

## Was gebaut wurde

`cmd/holzcloud/templates/admin/field_list.html` druckte auf den Zeilen 8, 16 und
25 woertlich `&#8592; Alle Bausteinarten` / `&#8592; Alle Textbausteine` /
`&#8592; Alle Felder`, weil `t` sein Ergebnis kontextabhaengig maskiert und das
`&` der Entitaet dabei selbst zu `&amp;` wird. Die drei Aufrufe stehen jetzt auf
`th`, das seinen Wert als `template.HTML` zurueckgibt und deshalb nicht
maskiert.

Der ganze Kern der Sache ist, was **nicht** angefasst wurde: die Zeichenkette
hinter dem Funktionsnamen ist byte-fuer-byte dieselbe geblieben. Sie **ist** der
Katalogschluessel; sie zu aendern — etwa die Entitaet durch das Zeichen `←` zu
ersetzen, wie `deferred-items.md` es vorschlug — haette drei Schluessel in vier
Katalogen verwaisen lassen. Genau deshalb war der Befund ein Jahr lang
aufgeschoben, und genau deshalb war der Aufschub unnoetig.

## Die tatsaechliche Ausgabe der drei Tore

Das 1158-Tor ist der einzige Beweis dafuer, dass der Schluessel unangetastet
blieb, deshalb steht seine Ausgabe hier woertlich — **vor** der Aenderung
gemessen und **nach** der Aenderung noch einmal, beide Male identisch:

```
1158 Zeichenketten im Quelltext
de-CH.json   55 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1158 übersetzt, 0 offen, 0 verwaist
es.json      1158 übersetzt, 0 offen, 0 verwaist
fr-CH.json   4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1158 übersetzt, 0 offen, 0 verwaist
it-CH.json   9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1158 übersetzt, 0 offen, 0 verwaist
```

Die Zahl hat sich nicht bewegt. `git status --porcelain internal/i18n/locales/`
gab **keine Zeile** aus — kein Katalog wurde angefasst.

Die beiden Zaehlungen in der Vorlage:

```
-- Tor 3a: die drei neuen Aufrufstellen, erwartet 3 --
3
-- Tor 3b: keine maskierende Aufrufstelle mehr, erwartet 0 --
0
```

Die Reihe:

```
-- Tor 2a: die Vorlage ist noch zerlegbar --
ok  	github.com/holzcloud/holzcloud-cms/internal/web	3.601s
-- Tor 2b: die ganze Reihe --
go test ./...  → 41 Pakete ok, 3 [no test files], 0 Fehlschlaege (EXIT=0)
     darunter: cmd/holzcloud 2.386s, internal/admin 26.384s,
               internal/tmplmgr 0.462s, internal/web (cached)
-- Tor 2c: gofmt -l .  → keine Ausgabe
-- Tor 2d: go vet ./... → keine Ausgabe
```

`internal/web` ist dabei das Tor, das wirklich etwas ueber diese Aenderung sagt:
`render_test.go:19` zerlegt das echte Verzeichnis
`cmd/holzcloud/templates/admin`, die Vorlage muss also nach dem Flick noch
zerlegbar sein.

Die Tore aus Task 2:

```
-- offene Fenster, vorher 3, erwartet 2 --   → 2
-- Status des Eintrags 5, erwartet fixed --  → fixed
-- 260906-m9z in der Beschreibung --         → 1
-- Stempel in deferred-items.md --           → 2
-- als ueberholt markiert --                 → 1
-- der alte Text ist NICHT geloescht --      → 1
-- WR-03 unangetastet --                     → 1
```

## Sicherheit

`th` giesst nach `template.HTML` und maskiert deshalb nicht. Der Inhalt an
diesen drei Stellen ist ausschliesslich ein Zeichenkettenliteral aus einer in
das Binaerprogramm eingebetteten Katalogdatei dieses Repositoriums — nie eine
Benutzereingabe, nie ein Datenbankwert, nie ein Anfrageparameter. Genau das
nennt der Kommentar bei `internal/web/render.go:84-89` als Bedingung der
Funktion, und `website_list.html:5`/`:7` nutzen sie schon unter derselben
(T-M9Z-01, accept). T-M9Z-02 ist durch das 1158-Tor und die leere
`git status`-Gegenprobe erledigt, T-M9Z-03 dadurch, dass beide Commits
ausschliesslich benannte Dateien tragen.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blockierend] Die Reihenfolge in Task 2A war falsch herum**

- **Gefunden bei:** Task 2, erster Lauf von `windows fixed 5`
- **Befund:** Der Plan sagt, `windows fixed 5` lese den von Hand berichtigten
  JSON-Block, ziehe die Zaehler nach und **zeichne die Markdown-Tabelle neu** —
  die Tabellenzeile bekomme die berichtigte Beschreibung dadurch von selbst.
  Das Werkzeug tut das nicht. Es **prueft zuerst**, ob Tabelle und JSON-Block
  uebereinstimmen, und bricht ab, wenn sie es nicht tun:
  `Error: Ledger table in .../WINDOWS.md disagrees with the fenced JSON entries
  (the sole source of truth) for row id(s): 5.` Nach der reinen JSON-Aenderung
  war genau dieser Zustand hergestellt.
- **Behebung:** Die Beschreibungszelle der Tabellenzeile 5 wurde vor dem
  Werkzeuglauf aus dem JSON-Block heraus angeglichen (dieselbe Zeichenkette,
  maschinell uebernommen statt ein zweites Mal getippt). Danach lief
  `windows fixed 5` durch und hat Status, `resolved_at`, die Kopfzaehler und
  die Tabelle wie vorgesehen gesetzt.
- **Dateien:** `.planning/WINDOWS.md`
- **Commit:** 202fce5

### Beobachtungen, nicht behoben

**Der Arbeitsbaum war ein anderer als der Plan ihn gemessen hatte, und er hat
sich waehrend der Ausfuegung weiter bewegt.**

Der Plan hat gegen HEAD `da6c2fb` gemessen und `.planning/config.json`
(geaendert) sowie `.planning/milestone.lock` (unversioniert) im Baum gesehen.
Tatsaechlich stand HEAD bei Beginn auf `3255c7d`; beide genannten Dateien waren
bereits weg. Statt ihrer lagen `docs/screenshots/` und das 33-MB-Bauergebnis
`./holzcloud` unversioniert da. **Waehrend** der Ausfuegung kamen dazu:
geaenderte `README.md` und `.planning/phases/09-csv-import/09-04-PLAN.md`, dazu
unversioniert `09-PLAN-CHECK.md`, `docs/configuration.md`,
`docs/content-model.md`, `docs/multilingual.md`, `docs/publishing.md`.

Nichts davon ist in einen der beiden Commits geraten — die Regel des Plans
(ausschliesslich benannte Dateien, nie `-a`) hat genau das verhindert, wofuer
sie geschrieben wurde, und sie war noetig: eine Liste haette hier nicht
gereicht, weil der Baum sich waehrend des Laufs veraendert hat. Das Bauergebnis
`./holzcloud` wurde weder committet noch geloescht.

**`windows status --pick 'ledger.entries[4].description'` funktioniert nicht.**
Die Picks `ledger.open_count` (→ `2`) und `ledger.entries[4].status` (→
`fixed`) liefern sauber. Der Pick auf `description` gibt stattdessen das ganze
Ledger-JSON auf stdout **und** stderr aus und meldet
`Error: --pick "ledger.entries[4].description": command output was not JSON`.
Das Tor selbst ist trotzdem gruen — `grep -c '260906-m9z'` misst 1, weil die
Beschreibung im ausgegebenen JSON steht —, aber es misst nicht, was es zu
messen glaubt. Gegengeprueft wurde die Beschreibung deshalb direkt aus der
Datei: Eintrag 5 fuehrt `status: fixed`, `resolved_at:
2026-09-06T14:14:47.478Z`, und die Beschreibung enthaelt `260906-m9z` genau
einmal. Ein Werkzeugbefund, kein Befund dieses Repositoriums; nicht behoben,
weil er ausserhalb des Auftrags liegt.

## Known Stubs

Keine.

## Was noch offen ist

**QUAL-02 gehoert nicht dem Executor.** Der Pfeil muss in der laufenden
Anwendung gesehen werden, bevor die Aufgabe zaehlt — Feldliste einer Website,
dieselbe mit `?baustein=`, dieselbe mit `?textbaustein=`. Diesen Durchgang
faehrt der Orchestrator nach der Ausfuegung. Kein Browserschritt und keine
Checkliste sind hier eingebaut worden.

## Commits

| Commit | Betreff | Dateien |
|---|---|---|
| `899d7d0` | `fix(quick-260906-m9z-01): der Rueckweg ueber dem Feldbildschirm zeigt endlich einen Pfeil` | `cmd/holzcloud/templates/admin/field_list.html` |
| `202fce5` | `docs(quick-260906-m9z-01): der Aufschub ruhte auf einer falschen Annahme, und das gehoert auf die Akte` | `.planning/WINDOWS.md`, `.planning/phases/08-snippets-carry-fields/deferred-items.md` |

## Self-Check: PASSED

Alle vier Dateien liegen auf der Platte, beide Commit-Kennungen (`899d7d0`,
`202fce5`) sind in `git log` auffindbar, `status: complete` steht im Kopf.

---

## Nachtrag: der Browserdurchgang (QUAL-02), vom Orchestrator gefahren

**Alle drei Stellen gesehen, nicht nur die eine, die der Plan verlangt hat.**
Frische Datenbank unter dem Kratzverzeichnis, Port 8147, 48 Wanderungen
angewandt, Wegwerf-Administrator über `holzcloud user create`, Zweitfaktor
eingerichtet (er ist für Administratoren Pflicht und lässt sich nicht umgehen).

| Bildschirm | Adresse | Was dasteht |
|---|---|---|
| Gruppe | `/admin/websites/1/felder?gruppe=1` | **`← All fields`** |
| Textbaustein | `/admin/websites/1/felder?textbaustein=1` | **`← All snippets`** |
| Bausteinart | `/admin/websites/1/felder?baustein=1` | **`← All block kinds`** |

Ein echter Pfeil an allen drei Stellen. Vorher stand dort wörtlich
`&#8592; Alle Felder`.

**Die Oberfläche stand dabei auf Englisch**, und das ist mehr wert als der
deutsche Fall: `{{th}}` übersetzt zuerst und gibt das Ergebnis dann als HTML
aus, also beweist der englische Durchgang, dass der Flick **durch den
Übersetzungsweg hindurch** hält und nicht nur am unübersetzten Ausgangstext.

Browser-Konsole: **0 Fehler.** Serverprotokoll: **0 ERROR-Zeilen, 0
CSP-Einträge.**

### Ein Fehlalarm, der keiner war — und warum er hier steht

Zwischendurch sah es so aus, als nähme das Textbaustein-Formular eine Eingabe
an, ohne etwas anzulegen: die Felder waren gefüllt, nach dem Klick war das
Formular leer und die Liste sagte weiterhin „No snippets yet". Das wäre ein
schwerer Fehler gewesen. Nachgemessen statt gemeldet: **die Datenbank direkt
abgefragt** (`SELECT … FROM snippets` — leer), dann dasselbe Formular über
`requestSubmit()` abgeschickt — „Snippet saved", Zeile da. Also hatten die
Mausklicks des Werkzeugs den Knopf verfehlt; die Anwendung war nie im Unrecht.

Festgehalten, weil die Versuchung gross war, das als Befund zu melden. Der
Unterschied zwischen „das Formular ist kaputt" und „mein Klick ging daneben"
kostete eine Abfrage.

**Eine benannte Grenze dieses Durchgangs:** die drei Objekte (Gruppe kam per
echtem Klick zustande, Textbaustein und Bausteinart per `requestSubmit()`).
Für die Pfeil-Prüfung ist das gleichwertig — geprüft wird gerenderter Text,
nicht der Knopf —, aber es ist kein Beleg dafür, dass die Speichern-Knöpfe
dieser beiden Formulare unter der Maus funktionieren. Das war nie Gegenstand
dieser Aufgabe und ist an anderer Stelle gedeckt.
