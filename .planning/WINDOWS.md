---
schema_version: 1
open_count: 6
waived_count: 1
fixed_count: 2
total_count: 9
last_updated: 2026-09-08T05:24:33.848Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | quick-260903-bsk | deviation | internal/i18n/locales/en.json |  | tools/i18n writeCatalog emits flush-left JSON while the four full catalogues carried a two-space indent; -write reformatted ~2250 lines each. Tool format kept as canonical. | waived | Accepted during execution, not an open defect: the tool's flush-left format is canonical (de-CH, fr-CH and it-CH were already flush-left); the two-space indent in the four full catalogues was drift from a hand-translation pass. | 2026-09-03T06:49:26.839Z | 2026-09-03T06:49:41.839Z |
| 2 | 07 | deviation | internal/field/field.go |  | trimTo schneidet einen Wert bei MaxValueBytes still ab; bei einem mehrwertigen Feld halbiert das einen Wert. Vorbestehend, D-13, gehoert Plan 07-04 (melden statt abschneiden) | fixed |  | 2026-09-05T13:53:52.821Z | 2026-09-05T14:51:59.731Z |
| 3 | 07 | deviation | internal/field/field.go | 674 | Die Ablehnungsgruende in field.go (rangeReason 674-682, die Laengen- und Mehrwert-Meldungen in Check 710 und 778-781) werden durch blosse Zeichenkettenverkettung gebaut, ohne i18n.N. Sie werden deshalb nie extrahiert und nie uebersetzt: bei englischer UI erschien 'Ausstattung: hoechstens 3 Werte, ausgewaehlt sind 4.' auf Deutsch, waehrend die Artnamen daneben uebersetzt waren. Vorbestehend, kein Regress aus Phase 7 (gegen 60ff5b2 geprueft: field.go gab dort schon rohes Deutsch zurueck); Phase 7 ist der Konvention gefolgt und hat die Flaeche verbreitert. Relevant, weil go run ./tools/i18n '0 offen, 0 verwaist' meldet, ohne diese Zeichenketten je zu sehen — der QUAL-01-Zaehler misst sie nicht. Zurueckgestellt: jede Validierungsrueckgabe in field.go umzuschreiben ist eigene Arbeit. | open |  | 2026-09-05T16:13:58.341Z |  |
| 4 | 08 | deviation | .planning/phases/07-field-kinds/07-SECURITY.md | 183 | W-4 nennt 'T-07-26 (Plan 05)', die Nummer gehoert aber Plan 06 (Information Disclosure ueber field.Hidden). Vorbestehende Verwechslung, beim Schliessen von T-07-26 in Plan 08-04 gefunden und bewusst nicht angefasst (der Plan verbietet Aenderungen an anderen Eintraegen; welche Nummer richtig waere, liesse sich nur raten). Folge: das Zaehltor grep -c 'T-07-26' misst 2 statt 1. | open |  | 2026-09-06T10:24:52.821Z |  |
| 5 | 08 | deviation | cmd/holzcloud/templates/admin/field_list.html | 16 | field_list.html druckt &#8592; als Text statt als Pfeil (alle drei Rueckwege); Aenderung verwaist drei Katalogschluessel, darum zurueckgestellt — BERICHTIGT: diese Begruendung des Aufschubs ruhte auf einer falschen Annahme darueber, worin der Flick besteht. Sie gilt allein fuer den Flick, den deferred-items.md vorschlaegt (die Entitaet durch das Zeichen ersetzen). Der tatsaechlich gefahrene Flick wechselt an denselben drei Stellen nur die aufrufende Funktion von t auf die HTML-durchlassende Fassung th; die Zeichenkette bleibt byte-gleich, kein Schluessel verwaist, kein Katalog wurde angefasst, und der Zaehler stand vorher wie nachher auf 1158 Zeichenketten mit 0 offen, 0 verwaist fuer en/es/fr/it. Geschlossen im Schnellauftrag 260906-m9z am 2026-09-06. | fixed |  | 2026-09-06T10:55:06.562Z | 2026-09-06T14:14:47.478Z |
| 6 | 11 | deviation | internal/bundle/import.go |  | Report.Warnings baut jeden Satz mit fmt.Sprintf und rohem Deutsch. CLAUDE.md haelt seit 7e0c834 ausdruecklich fest, dass ein mit fmt.Sprintf gebauter Satz fuer tools/i18n unsichtbar ist. Vorbestehend: rund 30 solche Warnungen standen schon vor Plan 11-06 in dieser Datei; 11-06 hat vier weitere in derselben Form ergaenzt (importAlbums, missingAlbum), weil die Alternative den Locale des Bedieners durch bundle.Import zu faedeln waere und der Bericht sonst in der Sprache der importierten Website erschiene statt in der des Bedieners. Der richtige Flick ist die Form, die .planning/GLOSSARY.md fuer csvimport schon vorschreibt: Code plus Argumente statt fertigem Satz (D-32). Folge: der Zaehler go run ./tools/i18n sieht keine dieser Zeilen. | open |  | 2026-09-07T23:07:58.770Z |  |
| 7 | 10 | deviation | internal/admin/forwardauth.go |  | The plan 10-04 verify gate 'grep secret\|password \| grep -c slog.' reads a proxy: gofmt wraps slog.Info across lines, so a secret appended to a continuation line keeps the gate at 0. Held by TestTheProvisioningSecretAppearsInNoLogLine instead. | open |  | 2026-09-08T04:44:33.668Z |  |
| 8 | 11 | deviation | internal/block/render.go | 212 | Eine Album-Galerie mit Diashow-Darstellung zeigt ihre Lichtkasten-Bedienelemente in der Sprache des Besuchers und den Namen ihres Schiebefelds auf Deutsch — auf derselben Seite, im selben Durchgang. Im Browser gemessen am 2026-09-08: Website auf Englisch, /: 'Next image \| Previous image \| Close large view' neben aria-label="Galerie"; auf Spanisch: 'Imagen siguiente \| Imagen anterior \| Cerrar la vista grande' neben aria-label="Galerie". Ursache: render.go:212 uebersetzt den Regionsnamen mit s.text (block.Set.T, das internal/admin/page_blocks.go NUR beim Speichern setzt), waehrend die Bedienelemente in GalleryItems bei einer Album-Galerie ueber internal/album/expand.go:133 set.t bekommen, den Uebersetzer der Anfrage. Der Kommentar ueber textGallery behauptet, die Wiederverwendung von 'Galerie' koste nichts, weil der Schluessel 'in en, es, fr und it heute uebersetzt ist' — er wird nie uebersetzt gerendert. Genau die Klasse Fehler, die das i18n-Tor nicht sieht: markiert, gesammelt, viermal uebersetzt, 0 offen 0 verwaist, und trotzdem deutsch beim Besucher. Betrifft nur den Vorlese-Namen des Schiebefelds. Die Behebung verschiebt die Grenze zwischen dem, was eine Galerie beim Speichern einfriert, und dem, was sie bei der Anfrage aufloest — eine Architekturfrage (Regel 4), deshalb hier festgehalten und nicht am Phasenende gemacht. | open |  | 2026-09-08T05:10:24.057Z |  |
| 9 | 10 | deviation | internal/admin/forwardauth.go |  | 10-05: the plan's HasGroup counting gate counts its own explanatory comment (prints 3, wants 1); the corrected gate adds grep -v '//' and prints 1 | open |  | 2026-09-08T05:24:33.848Z |  |

````json
[
  {
    "id": 1,
    "kind": "deviation",
    "phase": "quick-260903-bsk",
    "file": "internal/i18n/locales/en.json",
    "line": null,
    "description": "tools/i18n writeCatalog emits flush-left JSON while the four full catalogues carried a two-space indent; -write reformatted ~2250 lines each. Tool format kept as canonical.",
    "status": "waived",
    "reason": "Accepted during execution, not an open defect: the tool's flush-left format is canonical (de-CH, fr-CH and it-CH were already flush-left); the two-space indent in the four full catalogues was drift from a hand-translation pass.",
    "recorded_at": "2026-09-03T06:49:26.839Z",
    "resolved_at": "2026-09-03T06:49:41.839Z"
  },
  {
    "id": 2,
    "kind": "deviation",
    "phase": "07",
    "file": "internal/field/field.go",
    "line": null,
    "description": "trimTo schneidet einen Wert bei MaxValueBytes still ab; bei einem mehrwertigen Feld halbiert das einen Wert. Vorbestehend, D-13, gehoert Plan 07-04 (melden statt abschneiden)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-05T13:53:52.821Z",
    "resolved_at": "2026-09-05T14:51:59.731Z"
  },
  {
    "id": 3,
    "kind": "deviation",
    "phase": "07",
    "file": "internal/field/field.go",
    "line": 674,
    "description": "Die Ablehnungsgruende in field.go (rangeReason 674-682, die Laengen- und Mehrwert-Meldungen in Check 710 und 778-781) werden durch blosse Zeichenkettenverkettung gebaut, ohne i18n.N. Sie werden deshalb nie extrahiert und nie uebersetzt: bei englischer UI erschien 'Ausstattung: hoechstens 3 Werte, ausgewaehlt sind 4.' auf Deutsch, waehrend die Artnamen daneben uebersetzt waren. Vorbestehend, kein Regress aus Phase 7 (gegen 60ff5b2 geprueft: field.go gab dort schon rohes Deutsch zurueck); Phase 7 ist der Konvention gefolgt und hat die Flaeche verbreitert. Relevant, weil go run ./tools/i18n '0 offen, 0 verwaist' meldet, ohne diese Zeichenketten je zu sehen — der QUAL-01-Zaehler misst sie nicht. Zurueckgestellt: jede Validierungsrueckgabe in field.go umzuschreiben ist eigene Arbeit.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-05T16:13:58.341Z",
    "resolved_at": null
  },
  {
    "id": 4,
    "kind": "deviation",
    "phase": "08",
    "file": ".planning/phases/07-field-kinds/07-SECURITY.md",
    "line": 183,
    "description": "W-4 nennt 'T-07-26 (Plan 05)', die Nummer gehoert aber Plan 06 (Information Disclosure ueber field.Hidden). Vorbestehende Verwechslung, beim Schliessen von T-07-26 in Plan 08-04 gefunden und bewusst nicht angefasst (der Plan verbietet Aenderungen an anderen Eintraegen; welche Nummer richtig waere, liesse sich nur raten). Folge: das Zaehltor grep -c 'T-07-26' misst 2 statt 1.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-06T10:24:52.821Z",
    "resolved_at": null
  },
  {
    "id": 5,
    "kind": "deviation",
    "phase": "08",
    "file": "cmd/holzcloud/templates/admin/field_list.html",
    "line": 16,
    "description": "field_list.html druckt &#8592; als Text statt als Pfeil (alle drei Rueckwege); Aenderung verwaist drei Katalogschluessel, darum zurueckgestellt — BERICHTIGT: diese Begruendung des Aufschubs ruhte auf einer falschen Annahme darueber, worin der Flick besteht. Sie gilt allein fuer den Flick, den deferred-items.md vorschlaegt (die Entitaet durch das Zeichen ersetzen). Der tatsaechlich gefahrene Flick wechselt an denselben drei Stellen nur die aufrufende Funktion von t auf die HTML-durchlassende Fassung th; die Zeichenkette bleibt byte-gleich, kein Schluessel verwaist, kein Katalog wurde angefasst, und der Zaehler stand vorher wie nachher auf 1158 Zeichenketten mit 0 offen, 0 verwaist fuer en/es/fr/it. Geschlossen im Schnellauftrag 260906-m9z am 2026-09-06.",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-06T10:55:06.562Z",
    "resolved_at": "2026-09-06T14:14:47.478Z"
  },
  {
    "id": 6,
    "kind": "deviation",
    "phase": "11",
    "file": "internal/bundle/import.go",
    "line": null,
    "description": "Report.Warnings baut jeden Satz mit fmt.Sprintf und rohem Deutsch. CLAUDE.md haelt seit 7e0c834 ausdruecklich fest, dass ein mit fmt.Sprintf gebauter Satz fuer tools/i18n unsichtbar ist. Vorbestehend: rund 30 solche Warnungen standen schon vor Plan 11-06 in dieser Datei; 11-06 hat vier weitere in derselben Form ergaenzt (importAlbums, missingAlbum), weil die Alternative den Locale des Bedieners durch bundle.Import zu faedeln waere und der Bericht sonst in der Sprache der importierten Website erschiene statt in der des Bedieners. Der richtige Flick ist die Form, die .planning/GLOSSARY.md fuer csvimport schon vorschreibt: Code plus Argumente statt fertigem Satz (D-32). Folge: der Zaehler go run ./tools/i18n sieht keine dieser Zeilen.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-07T23:07:58.770Z",
    "resolved_at": null
  },
  {
    "id": 7,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "The plan 10-04 verify gate 'grep secret|password | grep -c slog.' reads a proxy: gofmt wraps slog.Info across lines, so a secret appended to a continuation line keeps the gate at 0. Held by TestTheProvisioningSecretAppearsInNoLogLine instead.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T04:44:33.668Z",
    "resolved_at": null
  },
  {
    "id": 8,
    "kind": "deviation",
    "phase": "11",
    "file": "internal/block/render.go",
    "line": 212,
    "description": "Eine Album-Galerie mit Diashow-Darstellung zeigt ihre Lichtkasten-Bedienelemente in der Sprache des Besuchers und den Namen ihres Schiebefelds auf Deutsch — auf derselben Seite, im selben Durchgang. Im Browser gemessen am 2026-09-08: Website auf Englisch, /: 'Next image | Previous image | Close large view' neben aria-label=\"Galerie\"; auf Spanisch: 'Imagen siguiente | Imagen anterior | Cerrar la vista grande' neben aria-label=\"Galerie\". Ursache: render.go:212 uebersetzt den Regionsnamen mit s.text (block.Set.T, das internal/admin/page_blocks.go NUR beim Speichern setzt), waehrend die Bedienelemente in GalleryItems bei einer Album-Galerie ueber internal/album/expand.go:133 set.t bekommen, den Uebersetzer der Anfrage. Der Kommentar ueber textGallery behauptet, die Wiederverwendung von 'Galerie' koste nichts, weil der Schluessel 'in en, es, fr und it heute uebersetzt ist' — er wird nie uebersetzt gerendert. Genau die Klasse Fehler, die das i18n-Tor nicht sieht: markiert, gesammelt, viermal uebersetzt, 0 offen 0 verwaist, und trotzdem deutsch beim Besucher. Betrifft nur den Vorlese-Namen des Schiebefelds. Die Behebung verschiebt die Grenze zwischen dem, was eine Galerie beim Speichern einfriert, und dem, was sie bei der Anfrage aufloest — eine Architekturfrage (Regel 4), deshalb hier festgehalten und nicht am Phasenende gemacht.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T05:10:24.057Z",
    "resolved_at": null
  },
  {
    "id": 9,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "10-05: the plan's HasGroup counting gate counts its own explanatory comment (prints 3, wants 1); the corrected gate adds grep -v '//' and prints 1",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T05:24:33.848Z",
    "resolved_at": null
  }
]
````
