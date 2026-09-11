---
phase: 08-snippets-carry-fields
plan: 02
subsystem: api
tags: [go, sqlite, html-template, field-definitions, snippets, public-render]

# Dependency graph
requires:
  - phase: 08-01
    provides: "Wanderung 00047, field.Def.SnippetID, OfSnippet, snippets.fields, SiteData.Bausteinfelder/.Bausteinliste und fillSnippets als Prägestelle mit zwei von vierzehn Aufrufen"
  - phase: 07-field-kinds
    provides: "die vier neuen Spalten, field.Resolve/field.List als Weg nach draussen, und die Lehre aus zwei Kritischen, dass ein Zählgatter kein Verhaltensbeweis ist"
provides:
  - "(*field.Store).OfSnippets — der Massenleser, eine Abfrage je Website"
  - "Move's vierter Arm und ein default-Arm, der die Seite nennt statt „was übrig bleibt“"
  - "validate's Textbaustein-Arm — gilt_fuer gestellt, Bedingung geleert, Pflicht und Feldarten ausdrücklich unangetastet"
  - "die trägerweise MaxFields-Zählung (D-05) als vierarmiger Schalter"
  - "fillSnippets als einzige Zuweisungsstelle der drei Textbaustein-Mitglieder in ganz internal/public/"
  - "der Mehrroutentest über tag.go, search.go und shop.go"
affects: [08-03, 08-04, 08-05, 09-csv-import]

actuals:
  tokens: 11600
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Ein Vertragsmitglied hat genau eine Prägestelle, und ein Gatter weist nach, dass ausserhalb davon keine Zuweisung überlebt"
    - "Der Vorrat wird auf den Träger gezählt, damit die Ablehnung einen wahren Grund nennt"
    - "Ein Massenleser neben dem Einzelleser, mit einem Test, der beide Element für Element gleichsetzt"

key-files:
  created: []
  modified:
    - internal/field/store.go
    - internal/field/field.go
    - internal/field/store_test.go
    - internal/public/pagedata.go
    - internal/public/access.go
    - internal/public/archive.go
    - internal/public/cart.go
    - internal/public/checkout.go
    - internal/public/pluginhost.go
    - internal/public/search.go
    - internal/public/shop.go
    - internal/public/tag.go
    - internal/public/typearchive.go
    - internal/public/bausteinfelder_test.go

key-decisions:
  - "fillSnippets liest über OfSnippets statt über OfSnippet je Textbaustein — sonst hätte der Massenleser dieses Plans keinen einzigen Aufrufer, und T-08-11 („kein Umlauf je Textbaustein“) wäre eine Zusage ohne Deckung"
  - "Eine Website ohne einen einzigen Textbaustein zahlt gar keine Abfrage: len(rendered.IDs) == 0 kehrt vor OfSnippets um, was die SNIP-03-Zusage „no additional query“ hält"
  - "Der Textbaustein-Arm der MaxFields-Zählung steht über dem Gruppenarm, weil eine Gruppe an einem Textbaustein stehen darf und ihre Unterfelder Zeilen desselben Formulars sind"
  - "Die neuen Prüfungen heissen TestBausteinNamensraum*, damit das Plangatter `-run TestBausteinNamensraum` sie wirklich fährt statt an ihnen vorbei"
  - "Zwei Zählgatter des Plans messen 1 statt ≥2 und 12 statt 11 — beide gemeldet und hergeleitet, keines durch eine künstliche Zeile oder eine Streichung passend gemacht"

patterns-established:
  - "Mutationsprobe für ein Verhaltensgatter: eine einzelne Route zurückdrehen und nachsehen, dass genau ihr Untertest fällt — sonst ist der Test Zierde"
  - "Ein Kommentar darf die Zeichenkette nicht enthalten, die ein Zählgatter zählt; sonst misst das Gatter Prosa"

requirements-completed: [SNIP-03, SNIP-04]

coverage:
  - id: D1
    description: "OfSnippets gibt für jeden Textbaustein Element für Element und Sub für Sub das heraus, was OfSnippet für denselben herausgibt — auch bei zwei Feldern auf derselben Position"
    requirement: "SNIP-04"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraumMassenleser"
        status: pass
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraumGruppe"
        status: pass
    human_judgment: false
  - id: D2
    description: "Eine Website ohne Textbausteinfelder bekommt eine leere und keine nil-Karte, und ohne Textbaustein läuft gar keine zusätzliche Abfrage"
    requirement: "SNIP-03"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraumMassenleser/ohne_Textbausteinfelder_eine_leere_Karte"
        status: pass
      - kind: integration
        ref: "internal/public/bausteinfelder_test.go#TestSnippetsBleibtTemplateHTML"
        status: pass
    human_judgment: false
  - id: D3
    description: "Der Feldvorrat wird auf den Träger gezählt: das einundsechzigste Feld eines Textbausteins wird abgelehnt, während das erste eines zweiten, ein Seitenfeld und ein Bausteinartfeld angenommen werden (D-05)"
    requirement: "SNIP-04"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraumFeldvorrat"
        status: pass
    human_judgment: false
  - id: D4
    description: "validate lässt Pflicht und Feldartenauswahl an einem Textbaustein unangetastet, stellt gilt_fuer auf beides und leert die Bedingung — und der Bausteinart-Arm bleibt, wie er war"
    requirement: "SNIP-04"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraumValidate"
        status: pass
    human_judgment: false
  - id: D5
    description: "Move verschiebt ein Textbausteinfeld nur innerhalb seines eigenen Textbausteins; die Positionen der Seitenfelder bleiben stehen, und das erste Feld hat nach oben nichts zu tauschen"
    requirement: "SNIP-04"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestBausteinNamensraumMove"
        status: pass
    human_judgment: false
  - id: D6
    description: "Ausserhalb von fillSnippets überlebt in internal/public/ keine Zuweisung an die drei Textbaustein-Mitglieder einer SiteData"
    requirement: "SNIP-03"
    verification:
      - kind: other
        ref: "grep -rn '\\.Snippets = ' internal/public/*.go | grep -v 'pagedata\\.go:' → 0 Treffer"
        status: pass
    human_judgment: false
  - id: D7
    description: "Drei Routen verschiedenen Zuschnitts — Schlagwortarchiv, Suche und Katalog — tragen den Feldwert eines Textbausteins durch den echten Handler bis ins Theme"
    requirement: "SNIP-03"
    verification:
      - kind: e2e
        ref: "internal/public/bausteinfelder_test.go#TestBausteinfelderAufMehrerenRouten"
        status: pass
    human_judgment: false
  - id: D8
    description: "internal/public/feed.go bleibt unverändert — eine Ladestelle ist keine Zuweisungsstelle"
    requirement: "SNIP-03"
    verification:
      - kind: other
        ref: "git diff --name-only -- internal/public/feed.go → leer"
        status: pass
    human_judgment: false
  - id: D9
    description: "Der Bearbeitungsbildschirm für Textbausteinfelder"
    verification: []
    human_judgment: true
    rationale: "Nicht Teil dieses Plans — 08-03 und 08-04 bauen die Bildschirme, und die Beurteilung im Browser gehört dorthin. Hier steht nur der Speicher und der öffentliche Weg darunter."

duration: 12 min
completed: 2026-09-06
status: complete
---

# Phase 8 Plan 02: Der Rest des Namensraums, und eine Prägestelle statt vierzehn Summary

**`OfSnippets` liest die Felder aller Textbausteine einer Website in einer Abfrage, `MaxFields` zählt auf den Träger statt auf die Website — und die zwölf verbliebenen Zuweisungen an `site.Snippets` gehen durch `fillSnippets`, sodass in ganz `internal/public/` genau eine überlebt, bewiesen von einem Gatter und von drei Routen im Test.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-09-06T09:40:09Z
- **Completed:** 2026-09-06T09:53:00Z
- **Tasks:** 2
- **Files modified:** 14

## Accomplishments

- **`OfSnippets`**, der Massenleser: eine Abfrage je Website statt einer je Textbaustein, `WHERE website_id = $1 AND snippet_id IS NOT NULL ORDER BY snippet_id, position, id`, Karte vor der Abfrage angelegt und nie nil, Baumbau wie `OfSnippet`. Ein Test setzt beide Wege Element für Element und `Sub` für `Sub` gleich — auch für zwei Felder, denen von Hand dieselbe Position gegeben wurde, wo allein die Nummer den Gleichstand bricht.
- **`fillSnippets` liest jetzt über `OfSnippets`.** Ohne diesen Schritt hätte der Massenleser dieses Plans keinen einzigen Aufrufer gehabt, und T-08-11 — „`fillSnippets` issues no per-snippet round trip" — wäre eine Zusage ohne Deckung geblieben. Eine Website ohne einen einzigen Textbaustein kehrt vor der Abfrage um und zahlt weiterhin nichts.
- **Der Feldvorrat hört auf zu lügen (D-05).** Ein vierarmiger Schalter in derselben Hausform wie `Move`s; jeder Arm nennt seinen Namensraum mit einer ausdrücklichen SQL-Bedingung, keiner zählt `WHERE website_id = $1` allein. Der Textbaustein-Arm steht **über** dem Gruppenarm, weil eine Gruppe an einem Textbaustein stehen darf und ihre Unterfelder Zeilen desselben Formulars sind.
- **`validate` bekommt den einen Arm, der absichtlich nicht sein Vorbild ist.** `gilt_fuer` wird auf `beides` gestellt und die Bedingung geleert; `Required` wird **nicht** gezwungen und die Feldarten werden **nicht** verengt. Beide Gründe stehen im Kommentar: ein Textbaustein hat ein eigenes Formular, auf dem sich ein Pflichtfeld zurückweisen lässt, und seine Werte erstarren nicht zu HTML, also gilt der Grund für `BlockKinds()` hier nicht. Ein Test legt `verweis` und `schlagwort` an einem Textbaustein an und weist im selben Atemzug nach, dass die Bausteinart sie weiterhin mit `ErrNotInBlock` abweist.
- **Zwölf Zuweisungen in neun Dateien umgestellt**, `feed.go` unberührt. Danach steht `.Snippets = ` in ganz `internal/public/` genau einmal, in `fillSnippets`, und der Doc-Kommentar dort sagt mit Zahl und Grund, warum das eine Eigenschaft der Bauart und keine Gedächtnisleistung ist.
- **Das Verhaltensgatter ist mit einer Mutationsprobe belegt.** `TestBausteinfelderAufMehrerenRouten` fährt Schlagwortarchiv, Suche und Katalog durch den echten Handler. Zur Kontrolle wurde `tag.go` einmal auf die alte Zuweisung zurückgedreht: **genau der Untertest „Schlagwortarchiv" fiel, kein anderer** — danach wiederhergestellt. Ein Test, der auch ohne die Änderung grün ist, wäre Zierde gewesen.

## Task Commits

1. **Task 1: Massenleser, vierter Move-Arm, validate's Textbaustein-Arm, trägerweiser Vorrat** — `61f2b8d` (feat)
2. **Task 2: Eine Prägestelle für die Textbaustein-Fläche des Themes** — `92dc3eb` (feat)

**Plan metadata:** der `docs(08-02)`-Commit dieses Plans

## Files Created/Modified

- `internal/field/store.go` — `OfSnippets`; `Move`s vierter Arm und der erklärte `default:`; `validate`s Textbaustein-Arm und die erweiterte Bedingungsklausel; die trägerweise Zählung
- `internal/field/field.go` — der `MaxFields`-Doc-Kommentar nennt jetzt den Träger und nicht mehr die Website
- `internal/field/store_test.go` — fünf neue Prüfungen der Familie `TestBausteinNamensraum*` samt drei Helfern
- `internal/public/pagedata.go` — `fillSnippets` liest über `OfSnippets`, kehrt ohne Textbaustein früh um, und sein Doc-Kommentar trägt die Einwegzusage
- `internal/public/access.go`, `archive.go`, `search.go`, `tag.go`, `typearchive.go` — sechs Stellen, an denen der `Rendered` schon in der Hand war
- `internal/public/cart.go`, `checkout.go`, `pluginhost.go`, `shop.go` — sechs Stellen, die einmal in eine lokale Grösse laden und diese weiterreichen
- `internal/public/bausteinfelder_test.go` — die geteilte Vorrichtung `bausteinVorrichtung`, drei neue Ansichten im Theme und `TestBausteinfelderAufMehrerenRouten`

## Decisions Made

- **`fillSnippets` über `OfSnippets`.** Der Plan schreibt es an keiner Stelle wörtlich vor; sein Ziel („der Massenleser, den `.Site.Bausteinfelder` braucht") und T-08-11 verlangen es beide. Ohne diesen Schritt bliebe `OfSnippets` toter Code.
- **Die frühe Umkehr bei `len(rendered.IDs) == 0`.** Die `must_haves` sagen „no additional query runs when the website has no snippet". Der Schleifenkörper lief vorher schon nie, `OfSnippets` liefe ohne diese Zeile aber doch — die Zeile hält eine Zusage, die der Umbau sonst gebrochen hätte.
- **Die neuen Prüfungen heissen `TestBausteinNamensraum…`.** Das Verify-Gatter des Plans fährt `-run TestBausteinNamensraum`; unter einem anderen Namen wäre es an allen neuen Fällen vorbeigelaufen und hätte trotzdem grün gemeldet. Der Plan verlangt „extend `TestBausteinNamensraum` … rather than starting a second file" — dieselbe Datei, dieselbe Prüffamilie, aber fünf Funktionen statt einer einzigen von fünfhundert Zeilen.
- **Kein Eintrag in `.planning/WINDOWS.md`.** Es gibt keinen Stummel, keine übersprungene Prüfung und kein ungefahrenes `<verify>`. Die zwei abweichenden Gatterzahlen sind eine Rechnung im Plan und kein Mangel im Baum; 08-01 hat denselben Fall ebenso im SUMMARY geführt und nicht im Fensterbuch.

## Deviations from Plan

### Auto-fixed Issues

**1. [Regel 3 — Blockierend] Ein Kommentar enthielt die Zeichenkette, die ein Zählgatter zählt**

- **Found during:** Task 1
- **Issue:** Der Kommentar am Textbaustein-Arm begann mit „Kein `d.Required = false`: …". Das Abnahmekriterium verlangt, dass `grep -c 'd.Required = false' internal/field/store.go` **genau** die Zahl von vorher (2) zurückgibt; mit dem Kommentar mass es 3. Das Gatter hätte Prosa gezählt und nicht Zuweisungen — und die Zahl 3 hätte im Verlauf ausgesehen wie ein zweiter Träger, der Pflicht erzwingt.
- **Fix:** Umformuliert zu „Pflicht wird hier nicht auf falsch gezwungen: …", inhaltlich unverändert, ohne die gezählte Zeichenkette.
- **Files modified:** `internal/field/store.go`
- **Verification:** `grep -c 'd.Required = false'` → 2 (Ausgangswert), `grep -c 'blockKind('` → 1 (Ausgangswert)
- **Committed in:** `61f2b8d`

**2. [Regel 2 — Fehlendes Kritisches] `fillSnippets` liest über `OfSnippets` statt je Textbaustein**

- **Found during:** Task 2
- **Issue:** `fillSnippets` rief in einer Schleife `OfSnippet` je Textbaustein auf — ein Umlauf je Textbaustein auf jedem öffentlichen Seitenaufbau. T-08-11 des Plans führt als Minderung „`fillSnippets` issues no per-snippet round trip", und Task 1s Massenleser hätte sonst keinen Aufrufer gehabt.
- **Fix:** Eine Abfrage über `OfSnippets`, danach Zugriff über die Karte; dazu die frühe Umkehr, wenn die Website keinen Textbaustein hat.
- **Files modified:** `internal/public/pagedata.go`
- **Verification:** `go test ./internal/public/` grün, `TestSnippetsBleibtTemplateHTML` weiterhin grün (die leere Karte bleibt eine leere Karte, weil `field.Resolve` bei nil-Definitionen `make(map, 0)` herausgibt)
- **Committed in:** `92dc3eb`

### Zwei Zählgatter messen andere Zahlen als der Plan — beide gemeldet, keines passend gemacht

Kein Fehlgriff in der Ausführung. Beide Zahlen sind gegen den Baum hergeleitet, und in beiden Fällen wäre das „Passendmachen" schlechter als die Meldung gewesen.

| Gatter | Plan | Gemessen | Richtung | Ursache |
|---|---|---|---|---|
| `grep -v '^\s*//' internal/field/store.go \| grep -c 'OfSnippets'` | ≥ 2 | **1** | eine **zu kurz** | ein Massenleser trägt seinen Namen genau einmal |
| `grep -rln 'fillSnippets' internal/public/*.go \| wc -l` | ≥ 11, „12 oder mehr" ist verdächtig | **12** | eine **zu lang** | `*.go` fasst `bausteinfelder_test.go` mit |
| `grep -rn '\.Snippets = ' … \| grep -vc ':\s*//'` | 0 | **0** | ✅ | — |
| `grep -c 'blockKind('` / `'d.Required = false'` | wie vorher | **1 / 2** | ✅ | nach Abweichung 1 |
| `git diff --name-only -- internal/public/feed.go` | leer | **leer** | ✅ | — |

**Zum ersten Gatter.** Die Gegenprobe steht in derselben Datei und ist entscheidend: `OfBlockTypes`, das der Plan selbst als Vorbild für `OfSnippets` benennt, misst mit demselben Befehl ebenfalls **1** — die Deklarationszeile, und sonst nichts. Ein Massenleser hat in `store.go` keinen Aufrufer; sein einziger Aufrufer ist `fillSnippets` in `internal/public/pagedata.go`, und das Gatter sieht nur `store.go`. Auf 2 zu kommen hätte eines von zweien verlangt: eine Zeile ohne Zweck, die den Namen ein zweites Mal nennt, oder einen Aufruf, den es nicht zu tun gibt. Der Massenleser **ist** vollständig da — Deklaration, Rumpf, Abfrage, Baumbau —, und drei Prüfungen lesen durch ihn hindurch. Gemeldet und nicht durch eine Attrappe erfüllt.

**Zum zweiten Gatter.** Die elf Dateien, die der Plan aufzählt, sind vollzählig und `feed.go` ist nicht darunter — nachgezählt: `grep -rln 'fillSnippets' internal/public/*.go | grep -v '_test\.go$' | wc -l` gibt **11**. Die zwölfte ist `bausteinfelder_test.go`, die `fillSnippets` schon **vor** dieser Aufgabe aufrief (`git show HEAD:… | grep -c 'fillSnippets'` → 1, geschrieben von Plan 08-01) und die dieser Plan ausdrücklich zu erweitern verlangt. Die Warnung des Gatters — „12 oder mehr heisst, eine `loadSnippets`-Stelle wurde für eine Zuweisungsstelle gehalten" — trifft hier nicht zu: `feed.go` ist unverändert und in keiner Liste. Das Muster `*.go` fasst schlicht Prüfdateien mit.

---

**Total deviations:** 2 auto-fixed (1 × Regel 3 blockierend, 1 × Regel 2 fehlendes Kritisches), dazu zwei gemeldete Gatterzahlen.
**Impact on plan:** Kein Zuwachs am Umfang. Abweichung 1 ist eine Umformulierung, die ein Gatter wieder messen lässt, was es messen soll. Abweichung 2 ist es, was den Massenleser aus Task 1 überhaupt in Betrieb nimmt und eine Minderung des Bedrohungsmodells deckt. Keine Zahl wurde durch Streichen oder durch eine künstliche Zeile passend gemacht.

## Issues Encountered

- **`data.Site` gegen `site`.** Sechs der zwölf Stellen halten den `Rendered` bereits, sechs nicht, und drei davon schreiben in eine `SiteData`, die schon in einem `PageData`-Literal liegt. Jede Stelle wurde vor der Änderung gelesen, wie der Plan es verlangt; die drei letzten bekamen `&data.Site` statt `&site`.
- **Ein Gatter, das seinen eigenen Kommentar zählt.** Siehe Abweichung 1 — bemerkt, weil die Abnahmekriterien nach der Aufgabe gefahren wurden und nicht erst am Ende.
- **Die Mutationsprobe war nicht verlangt, aber nötig.** Drei grüne Untertests beweisen nichts, solange nicht feststeht, dass sie rot werden können. `tag.go` wurde einmal zurückgedreht, genau ein Untertest fiel, danach wiederhergestellt — geprüft mit `grep -n fillSnippets internal/public/tag.go` und einem erneuten grünen Lauf.

## Known Stubs

Keine. Jede der zwölf umgestellten Stellen trägt echte Daten und ist vollständig umgebaut; keine ist auskommentiert stehen geblieben, was das `.Snippets = `-Gatter mit seinem Kommentarfilter ausdrücklich nachweist.

## Threat Flags

Keine neue sicherheitsrelevante Fläche über den `<threat_model>` des Plans hinaus.

- **T-08-07** (Websitenummer je Route): jede der zwölf Stellen reicht dieselbe Nummer weiter, die sie vorher an `loadSnippets` gab — `website.ID` beziehungsweise `websiteID` aus dem Auflöser. Keine Stelle prägt die Zuweisung mehr selbst.
- **T-08-08** (`OfSnippets`): die Abfrage verengt auf `website_id = $1`, **bevor** sie `snippet_id IS NOT NULL` fordert; die Karte kann keine Definition einer anderen Website tragen.
- **T-08-09** (gelockerter Vorrat): wie im Plan angenommen. Jeder Träger bleibt bei sechzig, und kein Aufruf zeichnet je mehr als einen Träger.
- **T-08-11**: durch Abweichung 2 tatsächlich gedeckt statt nur zugesagt — eine Abfrage je Aufbau, keine je Textbaustein, und ohne Textbaustein gar keine.
- Kein neuer `template.HTML`-Guss auf dem Weg eines Textbausteinfeldes; `field.Resolve` und `field.List` sind dieselben zwei Aufrufe wie bei `.Page.Felder`.

## User Setup Required

Keine — kein externer Dienst berührt.

## Next Phase Readiness

Bereit für **Plan 08-03**. Der Speicher ist vollständig: vier Namensräume, vier Leser, vier Arme in `Move`, vier Arme im Vorrat, und die öffentliche Fläche hat genau eine Prägestelle. Was 08-03 und 08-04 bauen, sind die Bildschirme — und sie finden `OfSnippet` für die Einzelansicht und `OfSnippets` für die Liste bereits vor.

Unverändert offen und ausdrücklich Plan 08-05 zugewiesen (aus 08-01 übernommen): `MinimalData`s leerer Zwilling für `Bausteinfelder`/`Bausteinliste` samt der Umschreibung des Satzes „no menus, no labels, no snippets", und die §7-Prosa der `TEMPLATE-SPEC.md`.

Keine Blockierer. `.planning/phases/11-galerie/` ist unberührt.

---
*Phase: 08-snippets-carry-fields*
*Completed: 2026-09-06*

## Self-Check: PASSED

Alle vierzehn geänderten Dateien liegen auf der Platte, beide Aufgabencommits
stehen im Verlauf, und jedes zugesagte Symbol ist im Baum: `OfSnippets`,
`Move`s `case current.SnippetID > 0`, `validate`s `if d.SnippetID > 0`, der
vierarmige Vorrat — und `fillSnippets` liest wirklich über den Massenleser.
`go build ./...`, `go vet`, `gofmt -l`, `go test ./...` sind grün, und
`go run ./tools/i18n` meldet für en/es/fr/it je `0 offen, 0 verwaist`.
