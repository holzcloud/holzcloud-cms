---
phase: 07-field-kinds
plan: 07
subsystem: testing
tags: [go, html-template, i18n, playwright, template-spec, field-kinds, csp, accessibility]

# Dependency graph
requires:
  - phase: 07-field-kinds
    provides: "07-01's KindMulti, Entry.Values, die Kaestchengruppe und ihr verborgener Wachposten"
  - phase: 07-field-kinds
    provides: "07-02's Migration 00046 und die vier Eigenschaften darstellung, max_werte, min_wert, max_wert am Definitionsformular"
  - phase: 07-field-kinds
    provides: "07-03's KindTime, KindRange und KindCode samt placeholder=\" \" am Zahlenfeld — die Annahme, die dieser Plan im Browser geprueft hat"
  - phase: 07-field-kinds
    provides: "07-04's Meldung statt stiller Kuerzung und die Mehrwert-Obergrenze, deren Ablehnung hier im Browser gesehen wurde"
  - phase: 07-field-kinds
    provides: "07-05's KindTerm, sein <select> und die Aufloesung auf den aktuellen Namen"
  - phase: 07-field-kinds
    provides: "07-06's switchOf(field.Def), der Schalter knopfreihe und die sechs steuernden Arten, deren Verhalten hier beobachtet wurde"
provides:
  - "TEMPLATE-SPEC.md dokumentiert alle fuenf neuen Arten, dazu .Values, .Term und das bislang undokumentierte .Yes"
  - "SampleData traegt eine gefuellte Zeile je neuer Art, MinimalData den leeren Zwilling jeder davon — die Grundlage, gegen die die Upload-Pruefung rendert"
  - "vier neue Reflexionswaechter: jedes Mitglied von field.Entry und jede Art aus field.Kinds muss im Dokument stehen, und beide Sichten auf dieselben Fixture-Daten muessen uebereinstimmen"
  - "23 neue deutsche Quellsaetze in en, es, fr und it — die Kataloge in einem Commit fuer sich"
  - "docs/offene-punkte.md: Punkt 1, 2 und 6 tragen ein Gebaut daneben statt geloescht zu sein"
  - "D-08 durch Beobachtung geschlossen: MayControl() bleibt unveraendert, KindRange bleibt steuernd"
  - "kein Vorwaertsverweis mehr im Baum auf einen Browserdurchgang, der stattgefunden hat"
affects: [08-snippets, 09-csv-import, 10-authentik, 11-galerie]

actuals:
  tokens: 23698
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Ein Vertrag, den drei Dateien zugleich buchstabieren, wird durch Reflexion zusammengehalten statt durch Aufmerksamkeit: ein neues Mitglied an field.Entry, das im Dokument fehlt, faellt im Test auf"
    - "Der leere Fall gehoert in die Fixture: MinimalData ist das Einzige, was eine Vorlage abfaengt, die ein Feld fuer immer gefuellt haelt"
    - "Eine mechanische Neuerzeugung ueber fuenf Katalogdateien bekommt einen Commit fuer sich, damit git log -S eine echte Aenderung daneben noch findet"
    - "Eine offene Frage wird durch eine Beobachtung geschlossen, und die Beobachtung wird dort aufgeschrieben, wo die Frage stand — ein Kommentar, der auf einen erledigten Durchgang vorausweist, ist eine Falle fuer den naechsten Leser"

key-files:
  created: []
  modified:
    - internal/tmplspec/TEMPLATE-SPEC.md
    - internal/tmplspec/spec_test.go
    - internal/template/sample.go
    - internal/template/sample_test.go
    - internal/i18n/locales/de-CH.json
    - internal/i18n/locales/en.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json
    - docs/offene-punkte.md
    - internal/field/field.go
    - internal/field/field_test.go
    - internal/admin/page_fields_switch_test.go

key-decisions:
  - "D-08 ist beantwortet und die Umkehr wurde NICHT angewendet: ein <input type=\"number\"> trifft :placeholder-shown, .feld-schalter--text greift daran, das abhaengige Feld erschien und verschwand. MayControl() steht, wie 07-03 es hinterlassen hat"
  - "Der leere Fall der neuen Arten liegt in MinimalDatas Felder-Karte, nicht in Feldliste: field.List laesst jeden leeren Eintrag aus, eine Liste mit leerem Eintrag gibt es also gar nicht"
  - "Der Browserdurchgang wurde vom Orchestrator gefahren, nicht vom Executor: die Vorbedingung von Schritt 3 verlangt ein Browserwerkzeug, das dem Executor nicht zur Verfuegung steht. Halten war richtig — die Arbeit wurde geteilt statt in eine Checkliste zurueckverwandelt"
  - "Die Sprache der Ablehnungsgruende wird hier nicht repariert: field.go baut sie durch Zeichenkettenverkettung ohne i18n.N, das ist vorbestehend und eigene Arbeit. Als offenes Fenster eingetragen statt still gelassen"

patterns-established:
  - "Ein Kommentar, der einen kuenftigen Durchgang ankuendigt, wird nach dem Durchgang umgeschrieben — sonst liest der naechste eine offene Frage, die geschlossen ist"
  - "Eine Beobachtung wird als Beobachtung notiert, mit Datum, Werkzeug und dem, was auf dem Bildschirm stand — nicht als Erwartung, die eingetroffen sei"

requirements-completed: [FIELD-01, FIELD-02, FIELD-03, FIELD-04, FIELD-05, FIELD-06, FIELD-07, FIELD-08]

coverage:
  - id: D1
    description: "TEMPLATE-SPEC.md dokumentiert mehrfachauswahl, schlagwort, zeit, bereich und code je in einem eigenen Absatz, dazu .Values, .Term und .Yes — und ein neues Mitglied an field.Entry oder eine neue Art in field.Kinds, die im Dokument fehlt, faellt im Test auf"
    requirement: FIELD-01
    verification:
      - kind: unit
        ref: "internal/tmplspec/spec_test.go#TestSpecDocumentsEveryFieldOfTheContract"
        status: pass
      - kind: unit
        ref: "internal/tmplspec/spec_test.go#TestSpecExamplesPassTheChecker"
        status: pass
    human_judgment: false
  - id: D2
    description: "SampleData traegt eine gefuellte Feldliste-Zeile je neuer Art samt passendem Schluessel in Felder; MinimalData traegt den leeren Zwilling jeder davon — die Upload-Pruefung rendert damit jede Vorlage auch gegen eine Seite, auf der jedes neue Feld leer ist (T-07-28)"
    requirement: FIELD-01
    verification:
      - kind: unit
        ref: "internal/template/sample_test.go#TestSampleDataFillsEveryField"
        status: pass
      - kind: unit
        ref: "internal/template/sample_test.go#TestMinimalDataLeavesOptionalFieldsEmpty"
        status: pass
      - kind: unit
        ref: "internal/template/sample_test.go#TestShippedThemesPassTheCheck"
        status: pass
    human_judgment: false
  - id: D3
    description: "Jede neue deutsche Zeichenkette dieser Phase hat eine nicht-leere Uebersetzung in en, es, fr und it; die beiden handgepflegten Kataloge fr-CH und it-CH wurden nicht geschrieben (QUAL-01)"
    verification:
      - kind: other
        ref: "go run ./tools/i18n — en/es/fr/it je '1151 uebersetzt, 0 offen, 0 verwaist'"
        status: pass
      - kind: other
        ref: "git show --stat b03ef1f — nur Pfade unter internal/i18n/locales/, fr-CH.json und it-CH.json unberuehrt"
        status: pass
    human_judgment: false
  - id: D4
    description: "Die vier neuen Bedienelemente am Definitionsformular (Darstellung, Hoechstzahl, untere und obere Grenze) sind vorhanden, uebersetzt und mit ihrem Nur-fuer-diese-Art-Hinweis versehen; eine untere Grenze ueber der oberen wird abgelehnt und die Definition nicht gespeichert"
    requirement: FIELD-05
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05, Definitionsbildschirm: 'Display' (Drop-down list / Row of buttons), 'Maximum number of values', 'Limits' als role=group mit aufloesendem aria-labelledby; unten=12 / oben=4 abgelehnt mit 'The lower limit is above the upper one — no number would fit between them. Swap the two values.', Definition nicht gespeichert"
        status: pass
    human_judgment: false
  - id: D5
    description: "bereich ist ein Zahlenfeld und kein Schieber, und die gewaehlte Zahl ist vor dem Speichern als Text sichtbar — ohne eine Zeile JavaScript (FIELD-05, D-07)"
    requirement: FIELD-05
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: <input type=\"number\" min=\"2\" max=\"12\" step=\"any\" placeholder=\" \"> im gezeichneten Markup, die getippte Zahl als Inhalt sichtbar"
        status: pass
    human_judgment: false
  - id: D6
    description: "zeit haelt eine Uhrzeit ohne Zeitzone und unterscheidet leer von Mitternacht; code ist ein festbreites Textfeld (FIELD-06)"
    requirement: FIELD-06
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: <input type=\"time\">, 07:30 ueberlebt Speichern und Neuladen, leer bleibt von 00:00 unterscheidbar; <textarea class=\"form-input form-code\"> mit errechneter font-family 'ui-monospace, \"Cascadia Code\", \"Fira Code\", monospace'"
        status: pass
    human_judgment: false
  - id: D7
    description: "Eine Mehrfachauswahl nimmt mehrere Werte in einem Speichern an, zeigt dieselben nach dem Neuladen wieder angekreuzt, unterscheidet geleert von nie mitgeschickt, und lehnt eine Ueberschreitung der Hoechstzahl serverseitig ab (FIELD-02, FIELD-07, D-11)"
    requirement: FIELD-02
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: verborgener Wachposten zuerst, dann vier Kaestchen, alle mit name=feld_ausstattung[]; drei angekreuzt -> gesendet [\"\",\"Schublade\",\"Griff\",\"Schloss\"] -> gespeichert \"Schublade\\nGriff\\nSchloss\" -> alle drei nach dem Neuladen angekreuzt; alle abgewaehlt -> gesendet [\"\"] (Schluessel da, Wert leer = geleert); vier gegen Hoechstzahl 3 -> HTTP 422"
        status: pass
    human_judgment: false
  - id: D8
    description: "Ein Schlagwortfeld bietet die Schlagwoerter dieser Website und keiner anderen, speichert das Kuerzel und druckt den Namen; eine Umbenennung aendert, was eine bereits veroeffentlichte Seite zeigt, ohne dass die Seite angefasst wird (FIELD-04)"
    requirement: FIELD-04
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: <select> mit '– no label –', Ahorn, Eiche, Nussbaum; 'Geheimholz' der zweiten Website fehlt; gespeichert wird 'eiche'; Eiche -> 'Eichenholz massiv' umbenannt, Kuerzel und gespeicherter Wert unveraendert, Seite nie neu gespeichert, oeffentliche Seite druckt danach 'Eichenholz massiv'"
        status: pass
    human_judgment: false
  - id: D9
    description: "Eine Auswahl als Knopfreihe zeichnet sich als Radioreihe in getippter Reihenfolge mit dem ausdruecklichen Leerknopf voran, nicht als Klappliste (FIELD-01, D-11)"
    requirement: FIELD-01
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: vier Radioknoepfe in getippter Reihenfolge, der erste mit value=\"\""
        status: pass
    human_judgment: false
  - id: D10
    description: "Die beiden Gruppenbedienelemente tragen kein for=, das ins Leere zeigt, und nennen ihre Beschriftung ueber ein aufloesendes aria-labelledby — der Browser bestaetigt, was 07-06 am Markup behauptet hat"
    requirement: FIELD-01
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: alle for= und alle id= im gezeichneten Formular eingesammelt und verglichen — null verwaiste for=; Kaestchengruppe und Knopfreihe je role=group mit aufloesendem aria-labelledby"
        status: pass
      - kind: integration
        ref: "internal/admin/page_fields_switch_test.go#TestSchalter/jedes_for_zeigt_auf_eine_Kennung,_die_es_gibt"
        status: pass
    human_judgment: false
  - id: D11
    description: "Ein abhaengiges Feld erscheint und verschwindet fuer jede der sechs steuernden Arten — Knopfreihe, Klappliste, Mehrfachauswahl, Schlagwort, Code und Bereich (FIELD-08, D-08, D-10)"
    requirement: FIELD-08
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: sechs Abhaengige, mit leerem Steuerfeld alle auf display: none; jedes Steuerfeld der Reihe nach gefuellt -> alle sechs auf display: block, einschliesslich bereich nach getippter 6 und der Knopfreihe nach einem echten Klick auf einen Radioknopf"
        status: pass
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: die Bedingungs-Klappliste bietet sitzplaetze an, abholzeit nicht — die KindTime-Ausnahme aus 07-03 im laufenden Programm bestaetigt"
        status: pass
    human_judgment: false
  - id: D12
    description: "HTML, das in ein code-Feld getippt wird, erscheint auf der veroeffentlichten oeffentlichen Seite als Text und fuehrt nichts aus; die Werte einer Mehrfachauswahl erscheinen als lesbare verbundene Zeichenkette (FIELD-06, FIELD-02, T-07-29)"
    requirement: FIELD-06
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05, oeffentliches Impressum (page.html): Abbundzeile = &lt;balken laenge=&#34;240&#34;&gt;Eiche &amp; Co&lt;/balken&gt;&lt;script&gt;alert(1)&lt;/script&gt; — null lebendes <script>alert in den ausgelieferten Bytes, maskierte Form vorhanden, nichts ausgefuehrt; Ausstattung = 'Schublade, Griff, Schloss'"
        status: pass
    human_judgment: false
  - id: D13
    description: "Kein Unterresource wird von einer fremden Herkunft geladen, und keine CSP-Verletzung tritt auf (T-07-32)"
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: Browserkonsole ueber die ganze Sitzung — drei 422-Ressourcenfehler (die drei absichtlichen Ablehnungsproben) und zwei 'Transition was skipped'-Hinweise, keine CSP-Verletzung, keine Parse-Beanstandung, keine Barrierefreiheitswarnung; Serverprotokoll null ERROR, null CSP, drei WARN (dieselben 422)"
        status: pass
    human_judgment: false
  - id: D14
    description: "Alles, was ein Mensch sieht, ist in seiner Sprache — die Haelfte von QUAL-02, die kein Test misst"
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05 mit Accept-Language en: alle vier neuen Bedienelemente englisch beschriftet, kein untersetzter Schluessel auf irgendeinem Bildschirm"
        status: pass
    human_judgment: true
    rationale: "Teilweise erfuellt, und die Luecke ist gemessen statt uebergangen: die Beschriftungen und Hinweise sind uebersetzt, aber die Ablehnungsgruende aus internal/field/field.go erschienen bei englischer Oberflaeche auf Deutsch. Vorbestehend (gegen 60ff5b2 geprueft) und ausserhalb des Auftrags dieses Plans; als offenes Fenster Nr. 3 in .planning/WINDOWS.md eingetragen. Ein Mensch muss vor dem Ausliefern entscheiden, ob das die Phase blockiert."
  - id: D15
    description: "D-08 ist durch Beobachtung geschlossen: KindRange bleibt steuernd, MayControl() unveraendert, und kein Kommentar im Baum weist noch auf einen Durchgang voraus, der stattgefunden hat"
    requirement: FIELD-08
    verification:
      - kind: automated_ui
        ref: "Playwright-MCP-Durchgang 2026-09-05: abhaengiges Feld an einem bereich-Feld display: none bei leerem Zahlenfeld, display: block nach getippter 6"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestWoranEineBedingungHaengenDarf"
        status: pass
      - kind: other
        ref: "grep -rn '07-07' internal cmd docs tools — kein Vorwaertsverweis mehr"
        status: pass
    human_judgment: false

# Metrics
duration: 37 min
completed: 2026-09-05
status: complete
---

# Phase 7 Plan 7: Der Vertrag, die Kataloge und der Browserdurchgang — Summary

**Die fünf neuen Arten stehen in der Spezifikation und in beiden Fixtures, die vier Kataloge sind grün und in einem Commit für sich, und D-08 ist nicht mehr offen: ein Zahlenfeld trifft `:placeholder-shown`, im Browser gesehen — `MayControl()` bleibt, wie es war.**

## Performance

- **Duration:** 37 min
- **Started:** 2026-09-05T15:38:00Z
- **Completed:** 2026-09-05T16:15:00Z
- **Tasks:** 3
- **Files modified:** 13

## Accomplishments

- **Der per-Art-Zoll ist bezahlt.** Jede der fünf neuen Arten hat einen Absatz in `TEMPLATE-SPEC.md`, eine gefüllte Zeile in `SampleData` und ihren leeren Zwilling in `MinimalData`. Dazu `.Values`, `.Term` — und `.Yes`, das nie dokumentiert war und beim Zählen auffiel.
- **Der Vertrag hält sich jetzt selbst.** Vier neue Wächter: jedes Mitglied von `field.Entry` und jede Art aus `field.Kinds` muss im Dokument stehen, jeder Fixture-Eintrag ist so geformt, wie `field.List` ihn formt, und beide Sichten auf dieselben Daten stimmen überein. Die bisherigen Reflexionsläufe kamen an einem fremden Struct nicht vorbei und sagten zu beidem nichts.
- **23 neue deutsche Quellsätze in vier Sprachen**, in einem Commit, der nichts sonst anfasst. `go run ./tools/i18n` meldet `0 offen, 0 verwaist` auf vier Zeilen; `fr-CH.json` und `it-CH.json` blieben unberührt.
- **Der Browserdurchgang ist ganz gelaufen, nicht halb.** Jede neue Art, jedes neue Bedienelement, jede Gruppe, alle sechs steuernden Arten, die öffentliche Seite und die Konsole — gegen einen frisch gebauten Binary mit eigener Wegwerf-Datenbank, gefahren mit dem Playwright-MCP-Server. Was gesehen wurde, steht unten; was nicht gefahren werden konnte, gibt es nicht.
- **D-08 ist beantwortet und die Umkehr wurde nicht angewendet.** Das abhängige Feld an einem Bereichsfeld verschwand und erschien korrekt. `MayControl()` steht unverändert.
- **Ein Befund, der nicht dieser Phase gehört**, ist gemessen und eingetragen statt still gelassen: die Ablehnungsgründe in `field.go` sind nie übersetzt.

## Task Commits

1. **Schritt 1: die fünf Arten im Vertrag** — `9e2f46d` (feat)
2. **Schritt 2: die Kataloge, allein** — `b03ef1f` (chore)
3. **Schritt 2 (zweiter Teil): die Arbeitsliste** — `e96bcb8` (docs)
4. **Schritt 3: D-08 beobachtet, nicht mehr angekündigt** — `352753a` (docs)

Schritte 1 und 2 wurden von einem früheren Executor-Lauf gefahren und lagen bei Beginn dieses Laufs bereits committet vor; sie wurden nicht wiederholt.

## Wie Schritt 3 gefahren wurde

Schritt 3 trägt eine `<precondition>`: die Anwendung muss lokal startbar und **ein browsertreibendes Werkzeug in der Sitzung verfügbar** sein, und die Aufgabe darf ausdrücklich nicht in eine Checkliste zurückverwandelt werden, die dem Entwickler zurückgegeben wird.

Das Werkzeug stand dem Executor nicht zur Verfügung — der Executor-Werkzeugkasten trägt keine Browserwerkzeuge. **Halten war die richtige Antwort**, und die Arbeit wurde geteilt statt umgedeutet: **der Orchestrator hat den Durchgang mit dem Playwright-MCP-Server selbst gefahren**, gegen eine Wegwerf-Instanz (frisches temporäres Datenverzeichnis, `HOLZCLOUD_PORT=8137`, alle 46 Wanderungen einschliesslich `00046`, Verwalter über `/admin/setup` angelegt, Website mit der Domain `localhost`, Schlagwörter Eiche/Nussbaum/Ahorn, dazu eine **zweite Website** mit dem eigenen Schlagwort `Geheimholz`; sieben Felddefinitionen und sechs abhängige Felder, eines je steuernder Art). Danach alles abgeräumt; der Projektbaum blieb unberührt. Dieser Lauf hat die Beobachtungen entgegengenommen, sie in den Baum eingetragen und den Plan geschlossen.

**Die Oberfläche stand auf Englisch** (Accept-Language `en` im Browser). Das machte den Durchgang zur schärferen Probe auf Schritt 2: `en.json` war einer der vier Kataloge mit 23 offenen Sätzen, und englische Beschriftungen an den neuen Bedienelementen beweisen, dass die Übersetzungen angekommen sind, statt dass Deutsch durchfällt.

## Was gesehen wurde

Beobachtet, nicht erwartet:

**Definitionsbildschirm.** Alle vier neuen Bedienelemente vorhanden und übersetzt: „Display" (Drop-down list / Row of buttons), „Maximum number of values", „Limits" (unten/oben), jedes mit seinem „For the "X" kind only"-Hinweis. Das Grenzenpaar zeichnet sich als `role="group"` mit einem auflösenden `aria-labelledby`. Speichern mit unten=12 / oben=4 wurde **abgelehnt** — *„The lower limit is above the upper one — no number would fit between them. Swap the two values."* — und die Definition **nicht** gespeichert.

**Seiteneditor, je Art.**

- `bereich` → `<input type="number" min="2" max="12" step="any" placeholder=" ">`. Zahlenfeld, kein Schieber; die gewählte Zahl ist vor dem Speichern als Text sichtbar. Das `placeholder=" "` steht im gezeichneten Markup.
- `zeit` → `<input type="time">`; `07:30` überlebte Speichern und Neuladen; leer bleibt von `00:00` unterscheidbar.
- `code` → `<textarea class="form-input form-code">`, errechnete `font-family` = `ui-monospace, "Cascadia Code", "Fira Code", monospace` — festbreit an der errechneten Darstellung bestätigt, nicht am Klassennamen.
- `mehrfachauswahl` → verborgener Wachposten **zuerst**, dann vier Kästchen, alle mit `feld_ausstattung[]`. Drei angekreuzt → gesendet `["", "Schublade", "Griff", "Schloss"]` → gespeichert `"Schublade\nGriff\nSchloss"` → **alle drei nach dem Neuladen wieder angekreuzt**. Alle vier abgewählt → gesendet genau `[""]`, Schlüssel da, Wert leer: *geleert*, unterscheidbar von *nicht da*. Alle vier gegen Höchstzahl 3 → abgelehnt mit **HTTP 422** und `Ausstattung: höchstens 3 Werte, ausgewählt sind 4.`
- `schlagwort` → `<select>` mit `– no label –`, Ahorn, Eiche, Nussbaum. **`Geheimholz` der zweiten Website fehlt.** Gespeichert wird das Kürzel `eiche`.
- Knopfreihe → vier Radioknöpfe in getippter Reihenfolge, der erste mit `value=""`. Keine Klappliste.

**Gruppenbedienelemente.** Null verwaiste `for=` im gezeichneten Formular (jedes `for` und jedes `id` eingesammelt und verglichen). Kästchengruppe und Knopfreihe tragen je `role="group"` mit einem auflösenden `aria-labelledby`.

**Bedingte Felder — die Antwort auf D-08.** Zugewiesene Schalter: `sitzplaetze`→`text`, `abbundzeile`→`text`, `ausstattung`→`kreuz`, `material`→`auswahl`, `zustand`→`knopfreihe`, `klappliste`→`auswahl`. Mit leerem Steuerfeld errechneten alle sechs Abhängigen `display: none`. Jedes Steuerfeld der Reihe nach gefüllt: **alle sechs wechselten auf `display: block`** — einschliesslich `bereich` nach einer getippten `6` und einschliesslich der Knopfreihe nach einem echten Klick auf einen Radioknopf.

Ausserdem: die Bedingungs-Klappliste am Definitionsbildschirm bietet `sitzplaetze` an, `abholzeit` aber **nicht** — die `KindTime`-Ausnahme aus Welle 3 im laufenden Programm bestätigt.

**Öffentliche Seite.** Die Startseite benutzt `home.html` und hat keine `Feldliste`-Schleife, deshalb wurde stattdessen das Impressum veröffentlicht (gezeichnet von `page.html`). Es druckte: `Sitzplaetze = 6`, `Abholzeit = 07:30`, `Zustand = gebraucht`, `Ausstattung = Schublade, Griff, Schloss` (mehrwertig als lesbare verbundene Zeichenkette), `Material = Eiche` (der **Name** des Schlagworts, während gespeichert das Kürzel liegt) und `Abbundzeile = &lt;balken laenge=&#34;240&#34;&gt;Eiche &amp; Co&lt;/balken&gt;&lt;script&gt;alert(1)&lt;/script&gt;`. Ein echtes `<script>alert(1)</script>` war in das Code-Feld getippt worden: **null lebendes `<script>alert` in den ausgelieferten Bytes, maskierte Form vorhanden.** Wortwörtlich, und es führt nichts aus.

**Die Umbenennung, ohne die Seite anzufassen.** Schlagwort `Eiche` in `Eichenholz massiv` umbenannt. Kürzel blieb `eiche`, der gespeicherte Wert der Seite blieb `eiche`, die Seite wurde nie neu gespeichert — und die öffentliche Seite druckte danach `Eichenholz massiv`.

**Konsole und Serverprotokoll.** Ganze Sitzung: drei 422-Ressourcenfehler (alle drei die absichtlichen Ablehnungsproben) und zwei „Transition was skipped"-Hinweise der View-Transitions. **Keine CSP-Verletzung, keine Parse-Beanstandung, keine Barrierefreiheitswarnung.** Serverprotokoll: null ERROR-Zeilen, null CSP-Einträge, drei WARN-Zeilen, die dieselben absichtlichen 422 sind.

## Was nicht gefahren wurde

- **Das Code-Feld innerhalb eines Blocks** auf der öffentlichen Seite. Der Auftrag verlangt die Probe zweimal — auf einer schlichten Seite und im Block, weil ein Block beim Speichern zu HTML einfriert und Maskierung im Theme dort zu spät käme. Gefahren wurde die schlichte Seite. Der Blockpfad ist im Test gedeckt (`internal/block` prüft `renderOwn`s `case field.KindCode` und die `PlainText`-Whitelist, 07-03), aber **nicht im Browser gesehen**. Das ist eine offene Zeile der Browserhälfte, nicht ein bestandener Punkt.

## Entscheidung: D-08

**Die Umkehr wurde nicht angewendet.** `MayControl()` steht unverändert, `KindRange` bleibt steuernd.

Die Beobachtung ist eindeutig: ein `type="number"`-Feld **trifft** `:placeholder-shown`, die Regel `.feld-schalter--text` **greift** an `bereich`, und das abhängige Feld erschien und verschwand korrekt. Die Notiz im Fahrplan, `KindRange` neben `KindDate` in die Ausnahme zu nehmen, ruhte auf der Schieber-Annahme, die D-07 verworfen hat. Auch der zweite Zweig des Auftrags — „prüfe erst, ob das Attribut überhaupt im Markup steht" — ist beantwortet: `placeholder=" "` steht im gezeichneten Markup.

Geändert wurde deshalb nicht das Verhalten, sondern die Aktenlage. Drei Kommentare kündigten einen Durchgang an, der inzwischen stattgefunden hat:

- `internal/admin/page_fields_switch_test.go` — „der Browserdurchgang in Plan 07-07 ist das Einzige, was sie beantwortet" → sagt jetzt, was am 5. September 2026 gesehen wurde, und dass `MayControl()` deshalb nicht angefasst wurde.
- `internal/field/field_test.go` — „Der Browserdurchgang in Plan 07-07 entscheidet das endgültig" → „im Browserdurchgang zu Plan 07-07 beobachtet und nicht bloss erschlossen".
- `internal/field/field.go`, `MayControl()`s Kommentar — die Begründung „es ist ein Zahlenfeld und trägt sehr wohl einen Platzhalter" trägt jetzt den Zusatz, dass genau das im Browser nachgesehen wurde.

Es gibt keinen Vorwärtsverweis auf diesen Durchgang mehr im Baum: `grep -rn '07-07' internal cmd docs tools` findet nichts.

## Decisions Made

- **D-08 geschlossen, ohne `MayControl()` zu ändern** — begründet oben.
- **Der leere Fall der neuen Arten liegt in `MinimalData`s `Felder`-Karte, nicht in `Feldliste`.** `field.List` lässt jeden leeren Eintrag aus; eine Liste mit einem leeren Eintrag gibt es also gar nicht, und ihn dort zu verlangen hiesse, eine Form zu fordern, die der Code nie erzeugt.
- **Der Durchgang wurde geteilt statt umgedeutet.** Die Vorbedingung war nicht erfüllt, der Executor hat gehalten, der Orchestrator hat gefahren. Die Alternative — eine Checkliste zurückgeben — ist genau das, was der Auftrag verbietet, und was die Browserhälfte in Phase 6 halb hat liegen lassen.
- **Die Sprache der Ablehnungsgründe wird hier nicht repariert.** Vorbestehend, ausserhalb des Auftrags, und die Reparatur ist eigene Arbeit. Als offenes Fenster eingetragen.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Der Reflexionslauf sah `field.Entry` und `field.Kinds` nicht**

- **Found during:** Schritt 1
- **Issue:** Die bestehenden Wächter in `tmplspec` prüften die Struktur des Dokuments, aber kein Lauf kam an einem fremden Struct vorbei. Ein neues Mitglied an `field.Entry` oder eine neue Art in `field.Kinds`, die im Dokument fehlt, wäre stillschweigend durchgegangen — genau die Lücke, die der per-Art-Zoll schliessen soll.
- **Fix:** Zwei neue Wächter in `tmplspec` (jedes `Entry`-Mitglied und jede Art muss im Dokument stehen) und zwei in `template` (jeder Fixture-Eintrag ist so geformt, wie `field.List` ihn formt; beide Sichten auf dieselben Daten stimmen überein).
- **Files modified:** `internal/tmplspec/spec_test.go`, `internal/template/sample_test.go`
- **Verification:** `go test ./internal/tmplspec/ ./internal/template/` grün; `.Yes` fiel dabei als undokumentiert auf und wurde ergänzt
- **Committed in:** `9e2f46d`

**2. [Rule 2 - Missing Critical] `docs/offene-punkte.md` Punkt 2 stand als offen, obwohl er seit 07-05 gebaut ist**

- **Found during:** Schritt 2
- **Issue:** Der Auftrag nennt Punkt 1 und Punkt 6. Punkt 2 (Schlagwort als Feldart) stand nicht darin, war aber seit Plan 07-05 fertig. Eine Arbeitsliste, die einen gebauten Punkt als offen führt, ist schlechter als keine.
- **Fix:** Punkt 2 mit demselben Gebaut-Vermerk versehen wie 1 und 6; ausserdem die Wanderungsangabe von `00045` auf `00046` nachgezogen.
- **Files modified:** `docs/offene-punkte.md`
- **Verification:** Punkt 1, 2 und 6 tragen ein Gebaut daneben, nichts wurde gelöscht
- **Committed in:** `e96bcb8`

**3. [Rule 3 - Blocking] Drei Kommentare wiesen auf einen Durchgang voraus, der stattgefunden hatte**

- **Found during:** Schritt 3
- **Issue:** `page_fields_switch_test.go`, `field_test.go` und `field.go` sagten allesamt, D-08 sei offen und werde vom Browserdurchgang in 07-07 entschieden. Nach dem Durchgang ist das falsch, und ein späterer Leser hätte eine geschlossene Frage für offen gehalten — die Aktenlage wäre der Grund gewesen, die Prüfung ein zweites Mal zu fahren.
- **Fix:** Alle drei sagen jetzt, was beobachtet wurde, mit Datum, Werkzeug und dem, was auf dem Bildschirm stand. `MayControl()` selbst blieb unverändert.
- **Files modified:** `internal/admin/page_fields_switch_test.go`, `internal/field/field_test.go`, `internal/field/field.go`
- **Verification:** `grep -rn '07-07' internal cmd docs tools` findet keinen Vorwärtsverweis mehr; `go test ./...` grün, `gofmt -l .` still, `go vet ./...` sauber
- **Committed in:** `352753a`

---

**Total deviations:** 3 auto-fixed (2 fehlende kritische Absicherung, 1 blockierende Aktenlage)
**Impact on plan:** Kein Scope-Zuwachs. Zwei betreffen den Beweis und die Notizen, der dritte ist der Auftrag von Schritt 3 selbst. Kein Verhalten geändert; `MayControl()` ist byteweise dasselbe Prädikat wie nach 07-03.

## Issues Encountered

**Die Ablehnungsgründe sind nie übersetzt — vorbestehend, gemessen, zurückgestellt.**

Die Höchstzahl-Ablehnung erschien **auf Deutsch, während die Oberfläche auf Englisch stand**: `Ausstattung: höchstens 3 Werte, ausgewählt sind 4.` `internal/field/field.go` baut seine Prüfmeldungen durch blosse Zeichenkettenverkettung (`rangeReason` bei 674–682, die Längen- und Mehrwertmeldungen bei 710 und 778–781) ohne `i18n.N`. Sie werden deshalb nie extrahiert und nie übersetzt — während dieselbe Datei ihre Artnamen und Hinweise sehr wohl in `i18n.N` fasst, weshalb die Art-Klappliste übersetzt war.

**Kein Regress dieser Phase.** Gegen `60ff5b2` geprüft: `field.go` gab dort schon rohes Deutsch zurück (`"das muss mit / beginnen (eigene Seite) oder mit https:// (fremde Adresse)."`). Phase 7 ist der bestehenden Konvention gefolgt und hat die Fläche verbreitert.

Es zählt trotzdem, weil `go run ./tools/i18n` `0 offen, 0 verwaist` melden kann, während einer Person, die kein Deutsch liest, deutsche Ablehnungen gezeigt werden: **der QUAL-01-Zähler sieht diese Zeichenketten überhaupt nicht.**

Eingetragen als offenes Fenster Nr. 3 in `.planning/WINDOWS.md`, nicht hier repariert: jede Validierungsrückgabe in `field.go` umzuschreiben ist eigene Arbeit und gehört nicht in den Auftrag dieses Plans.

**Kein Checkpoint-Gate.** Schritt 3 trägt eine `<precondition>`, kein `checkpoint:*`. Sie war beim Executor unerfüllt (kein Browserwerkzeug), was ein Halten und einen `blocking-human`-Bericht bedeutet — und genau so ist es gelaufen, bevor dieser Lauf begann. Innerhalb dieses Laufs wurde kein Gate erreicht und keines automatisch bestätigt. Auto-Modus war aktiv (`workflow.auto_advance: true`) und blieb ohne Wirkung.

## Known Stubs

Keine. Kein hartkodierter Leerwert, kein Platzhaltertext, kein TODO in den geänderten Dateien. `placeholder=" "` am Zahlenfeld ist kein Stub, sondern das Element, an dem `.feld-schalter--text` greift — in diesem Durchgang im Browser bestätigt.

## Threat Flags

Keine neue sicherheitsrelevante Oberfläche. Kein Endpunkt, kein Authentifizierungspfad, kein Dateizugriff, keine Schemaänderung — dieser Plan hat Dokumente, Fixtures, Kataloge und Kommentare angefasst.

Vier Bedrohungen des Registers wurden hier abgeschlossen: **T-07-28** (`MinimalData` deckt jetzt jeden neuen leeren Fall), **T-07-29** (`code` erscheint als Text auf der öffentlichen Seite und führt nichts aus — auf der schlichten Seite beobachtet, im Block nur im Test gedeckt, siehe *Was nicht gefahren wurde*), **T-07-31** (D-08 durch Beobachtung entschieden) und **T-07-32** (keine CSP-Verletzung, kein fremdes Unterresource in der ganzen Sitzung). **T-07-30** war eingehalten: Wegwerf-Datenbank unter einem temporären Verzeichnis, keine echten Daten, keine echten Zugangsdaten.

## User Setup Required

Keine — kein externer Dienst, keine neue Umgebungsvariable, keine Migration in diesem Plan.

## Next Phase Readiness

- **Phase 7 ist abgeschlossen.** Alle sieben Pläne sind ausgeführt, alle acht Anforderungen FIELD-01 bis FIELD-08 sind erfüllt, und alle sechs Erfolgskriterien des Fahrplans sind erreicht — Kriterium 6 mit einer benannten Einschränkung: die Übersetzungshälfte ist grün, die Browserhälfte ist gefahren, und die eine Zeile, die nicht gefahren wurde (`code` innerhalb eines Blocks auf der öffentlichen Seite), steht oben als nicht gefahren und nicht als bestanden.
- **Zwei offene Punkte reisen weiter:** die nie übersetzten Ablehnungsgründe (Fenster Nr. 3) und die eine ungefahrene Browserzeile. Beide gehören der Entscheidung vor dem Ausliefern, nicht diesem Plan.
- **Phase 8 kann anfangen.** Das Schnipsel erbt eine vollständige Feldpalette, eine Spezifikation, die alle Arten dokumentiert, und zwei Fixtures, die den gefüllten und den leeren Fall jeder Art tragen — die Upload-Prüfung wird eine Vorlage, die ein neues Feld für immer gefüllt hält, ab jetzt abfangen.
- **Phase 9 erbt `SplitValues`/`JoinValues`** als die eine Schreibweise, statt eine dritte zu erfinden. Das war der Zweck von FIELD-07 und ist eingelöst.

## Self-Check: PASSED

- `internal/tmplspec/TEMPLATE-SPEC.md` — FOUND
- `internal/template/sample.go` — FOUND
- `internal/field/field.go` — FOUND
- `internal/field/field_test.go` — FOUND
- `internal/admin/page_fields_switch_test.go` — FOUND
- `docs/offene-punkte.md` — FOUND
- Commits `9e2f46d`, `b03ef1f`, `e96bcb8`, `352753a` — alle vier im Log
- `go test ./...` — grün, keine FAIL-Zeile
- `gofmt -l .` — still
- `go vet ./...` — sauber
- `go run ./tools/i18n` — `0 offen, 0 verwaist` auf vier Zeilen (en, es, fr, it)
- `grep -rn '07-07' internal cmd docs tools` — kein Vorwärtsverweis

---
*Phase: 07-field-kinds*
*Completed: 2026-09-05*
