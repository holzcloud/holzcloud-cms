---
schema_version: 1
open_count: 3
waived_count: 1
fixed_count: 1
total_count: 5
last_updated: 2026-09-06T10:55:06.562Z
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
| 5 | 08 | deviation | cmd/holzcloud/templates/admin/field_list.html | 16 | field_list.html druckt &#8592; als Text statt als Pfeil (alle drei Rueckwege); Aenderung verwaist drei Katalogschluessel, darum zurueckgestellt | open |  | 2026-09-06T10:55:06.562Z |  |

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
    "description": "field_list.html druckt &#8592; als Text statt als Pfeil (alle drei Rueckwege); Aenderung verwaist drei Katalogschluessel, darum zurueckgestellt",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-06T10:55:06.562Z",
    "resolved_at": null
  }
]
````
