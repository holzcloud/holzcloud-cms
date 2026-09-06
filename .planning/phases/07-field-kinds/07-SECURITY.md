---
phase: "07"
slug: "field-kinds"
status: audited
# threats_open = count of OPEN threats at or above workflow.security_block_on (= high)
threats_open: 0
threats_total: 36
threats_closed: 34
threats_open_below_threshold: 2
asvs_level: 1
block_on: high
created: "2026-09-06"
audited_at: "e044fb1"
---

# Phase 7 — Security

> Bedrohungsregister, akzeptierte Risiken und Prüfspur für die Feldarten.

Das Register wurde zur Planungszeit geschrieben (`register_authored_at_plan_time: true`)
und nach der Ausführung gegen den Code geprüft. Die Abkürzung, die der Workflow
bei `threats_open: 0` und ASVS 1 erlaubt, wurde **bewusst nicht genommen**: nach
den Plänen fand ein Code-Review zwei kritische Fehler in genau den Mechanismen,
deren Massnahmen „mitigate" behaupteten. Ein Durchwinken hätte den Schritt
wertlos gemacht.

---

## Trust Boundaries

| Grenze | Beschreibung | Was sie kreuzt |
|---|---|---|
| Browser → Seitenformular | Wiederholte Formularschlüssel `feld_<key>[]`, vom Absender frei wählbar in Anzahl und Inhalt | Redaktorinhalt, unbegrenzt wiederholbar |
| Browser → Definitionsbildschirm | Artname, Grenzen, Höchstzahl, Darstellung | Konfiguration, die spätere Prüfungen steuert |
| Gespeicherter Wert → öffentliche Seite | Insbesondere `code`, dessen Zweck wörtliche Wiedergabe ist | Redaktorinhalt in HTML-Kontext |
| Gespeicherter Wert → eingefrorener Baustein | Ein Baustein wird beim Speichern zu HTML; Maskierung im Theme wäre zu spät | Redaktorinhalt, dauerhaft |
| Archivdatei → Datenbank | Manifest eines fremden Rechners, Feldschlüssel und Werte wörtlich übernommen | Nicht vertrauenswürdige Struktur *und* Inhalt |
| Schlagwort-Slug → Begriffssuche | Entscheidet, welche Begriffe einer Website eine Seite erreicht | Websitegrenze |

---

## Threat Register

36 eigenständige Bedrohungen über sieben Pläne (42 Zeilen; `T-07-SC` wiederholt
sich je Plan). **Alle 16 mit Schweregrad `high` sind geschlossen, jede mit
belegter Fundstelle.** Die vollständige Belegtabelle steht im Prüfbericht dieser
Sitzung; hier die Zusammenfassung und die beiden offenen Punkte im Wortlaut.

| Schweregrad | Gesamt | Geschlossen | Offen |
|---|---|---|---|
| high | 16 | 16 | 0 |
| medium | 13 | 11 | 2 |
| low | 13 | 13 | 0 |

### Offen — unterhalb der Blockschwelle, nicht blockierend

#### T-07-02 · Tampering · medium · `Def.Key` wird nie auf seine Form geprüft

**Behauptete Massnahme:** „Ein Feldschlüssel ist slug-artig und kann keine
Klammer enthalten, also kann ein gebastelter Name kein anderes Feld adressieren."

**Befund:** Die Prüfhälfte ist da (`internal/admin/page_form.go:157-174`), die
**Schlüsselform ist aber Dokumentation und kein Code**. `validKey` wird auf
`Condition` (`internal/field/store.go:534`) und `AppliesTo` (`:546`) angewandt,
**nie auf `d.Key`**; `validate` leitet einen Schlüssel nur ab, wenn er leer ist.
`internal/bundle/import.go:351`, `:369`, `:414` übernehmen `Key: f.Key` wörtlich
aus einem hochgeladenen Manifest.

Konkret: ein einwertiges Feld mit Schlüssel `farbe[]` prägt den Formularnamen
`feld_farbe[]`, den `fieldsFromRequest` als mehrwertigen Schlüssel `farbe` liest —
kollidierend mit einem echt so benannten Feld. Eines überschreibt still das andere.

**Warum medium und nicht high:** Schlüssel reisen als gebundene `$n`-Parameter
und erreichen HTML nur innerhalb `name="{{…}}"`, was `html/template` im
Attributkontext maskiert. Die Reichweite bleibt innerhalb einer Website
(`importFields` schreibt die aufgelöste `websiteID`). Es ist keine Injektion,
sondern die **Falsifikation der tragenden Prämisse einer erklärten Massnahme**.

**Behebung:** ein Aufruf `validKey(d.Key)` in `validate`. `SlugifyKey` erzeugt
ohnehin nur `[a-z0-9_]`, kein vom Admin angelegtes Feld ist betroffen.

#### T-07-26 (Plan 06) · Information Disclosure · medium · Der genannte Mechanismus existiert nicht

**Behauptete Massnahme:** „`field.Hidden` verwirft ihre Werte serverseitig beim
Speichern, ein verstecktes Feld kann also keinen Wert einschmuggeln."

**Befund:** `Hidden` wird an genau zwei Stellen aufgerufen —
`internal/field/field.go:409` innerhalb `Effective` und `:937` innerhalb
`CheckAll` — **keine davon ist ein Speicherweg**. `Effective` wird nur aus
`internal/field/render.go:106` und `:273` gerufen. `Clean`, das *der*
Speicherweg ist, behält versteckte Werte ausdrücklich
(`internal/field/field.go:568-573`: „Ein Feld, dessen Bedingung nicht erfüllt
ist, behält seinen Wert.").

**Die andere Hälfte der Massnahme ist wahr** und ist die eigentliche Antwort auf
die Kategorie: die Felder gehören zu demselben Formular, das die Redaktorin
ohnehin sehen darf. Es wird also nichts offengelegt, was sie nicht schon sieht.

**Behebung:** die Massnahme auf das zurückschreiben, was tatsächlich schützt,
statt einen Mechanismus zu nennen, den es nicht gibt. Siehe die Warnung unten —
dort liegt die Substanz.

---

## Warnungen — nicht im Register, beim Prüfen gefunden

### W-1 · Der Ausnahmefall bei bedingten Feldern ist unkartierte Angriffsfläche, und diese Phase hat ihn verbreitert

`CheckAll` überspringt ein Feld, dessen Bedingung nicht erfüllt ist
(`internal/field/field.go:941`). `Clean` behält seinen Wert
(`:568-573`). Vor Plan 07-04 kürzte `trimTo` unbedingt, `MaxValueBytes` galt für
diese Klasse also trotzdem. **D-13 hat diese Kürzung entfernt.**

Ergebnis auf **allen drei** Schreibwegen gleichermassen: der Wert eines bedingt
versteckten Feldes wird **ohne Bytegrenze und ohne Artregel** gespeichert —
einschliesslich einer `mehrfachauswahl`, deren Werte nie gegen ihre geschlossene
Liste geprüft wurden.

Das schränkt die Verdikte zu **T-07-01, T-07-03, T-07-15 und T-07-17** ein, deren
Massnahmentexte alle „vor dem Schreiben" sagen.

**Einstufung: medium.** Braucht eine angemeldete Redaktorin und ein von Hand
gebautes POST (oder ein von Hand bearbeitetes Manifest); der Wert wird beim
Rendern von `Effective` wieder verworfen; er wird maskiert, falls die Bedingung
später erfüllt wird; die äussere 10-MB-Grenze aus `ParseForm` gilt weiter.
Speicherballast und eine umgangene Prüfung, keine Rechteausweitung.

Das ist die **unbehobene zweite Hälfte von WR-01** — `60cd2e3` hat nur die
Filterhälfte behoben.

### W-2 · `07-05-SUMMARY.md` trägt keinen `## Threat Flags`-Abschnitt

Ausgerechnet der Plan mit den meisten hohen Bedrohungen (T-07-19, T-07-20,
T-07-22, T-07-25). Alle vier wurden unabhängig im Code geprüft, kein Verdikt
ändert sich — aber die Selbstauskunft des Ausführenden fehlt für diesen Plan.

### W-3 · `JoinValues` verteidigt sein Trennzeichen nicht — heute Korrektheit, morgen Sicherheit

`["a\nb","c"]` kommt als drei Werte zurück. Heute unschädlich: die einzige Art,
die das Paar benutzt, ist `KindMulti`, und `Check` prüft jeden geteilten Wert
gegen die geschlossene Optionsliste (`internal/field/field.go:751-765`).

Aber: es ist ein **Trennzeichen-Injektionsfehler in einer exportierten Funktion,
deren erklärter Zweck ist, von Phase 9s CSV-Importeur geerbt zu werden** (D-02).
Was ihn heute entschärft, ist die Optionsliste — **und eine CSV-Spalte ist kein
geschlossenes Vokabular.** Er wird an dem Tag zur Sicherheitslücke, an dem dieser
Aufrufer existiert.

**Zu beheben, bevor Phase 9 geplant wird, nicht danach** — zusammen mit der
Frage aus Verifikations-Gap 1 (`Clean` löscht die Unterscheidung
geleert/unberührt, die Phase 9 erben soll). Dieselbe Entwurfsrunde.

### W-4 · Die Bezeichnungsmenge ist von 12 auf 32 MiB gewachsen

CR-02s Behebung ersetzte `term.Parse` durch `term.Normalize` auf dem Archivweg —
richtig, denn `Parse` teilte an Kommas und deckelte bei `MaxPerPage = 12`, was
beides Eigenschaften *eines Eintrags* sind und nicht eines Archivs. Die Massnahme
zu T-07-26 (Plan 05) hält in der Substanz: Leerraum wird gefaltet, `MaxNameLength`
schneidet, ein leer slugifizierender Name fällt weg, und der Name reist als
gebundener Parameter.

Die wirksame Obergrenze für die *Anzahl* Bezeichnungen ist damit aber von 12 auf
das gewandert, was in `MaxManifestBytes` (32 MiB, `internal/bundle/import.go:62`,
durchgesetzt via `io.LimitReader` bei `:139`) passt. Die wörtliche Behauptung der
Massnahme („kann keine *unbegrenzte* Liste prägen") gilt weiter, und der Akteur
ist eine angemeldete Betreiberin — deshalb nicht offen. Aber es ist eine echte
Verbreiterung und gehört notiert.

---

## Accepted Risks Log

| Risiko | Bezug | Begründung | Angenommen von | Datum |
|---|---|---|---|---|
| Keine Paketinstallation in dieser Phase | T-07-SC (×7) | `git diff ba23dbd..HEAD -- go.mod go.sum` ist über 69 Commits leer. Jedes Symbol ist Go-Standardbibliothek oder stand schon in `go.mod`. Das Paket-Legitimitätstor feuert korrekt nicht. | Prüflauf 2026-09-06 | 2026-09-06 |
| Werte werden im Definitionsformular zurückgespiegelt | T-07-04 | `field_input.html:120-126` gibt sie über `{{$wahl}}` im Attributkontext aus; keine `template.HTML`- oder `safeHTML`-Verwendung in der Datei. | Plan 07-02 | 2026-09-05 |
| Migration `00046` ist ohne Prüfspur umkehrbar | T-07-09 | `migrations/00046_field_kinds.sql:30-33` — das `Down` kehrt alle vier Spalten um. Ein Rückbau ist nachvollziehbar, auch ohne eigenen Protokolleintrag. | Plan 07-02 | 2026-09-05 |

---

## Audit Trail

### Security Audit 2026-09-06

| Metrik | Anzahl |
|---|---|
| Bedrohungen im Register | 36 |
| Geschlossen | 34 |
| Offen (unter der Schwelle) | 2 |
| Offen (blockierend, ≥ `high`) | **0** |
| Warnungen ausserhalb des Registers | 4 |

**Verfahren:** alle sieben `<threat_model>`-Blöcke, alle sieben
`## Threat Flags`, das Code-Review, der Behebungsbericht und die Verifikation
gelesen, dann jede Registerzeile gegen den Code bei `e044fb1` geprüft. Keine
Implementierungsdatei verändert. `go test` für `field`, `block`, `admin`,
`template`, `public`, `bundle`, `term` — alle grün bei HEAD.

**Nächste Schritte, in dieser Reihenfolge:**

1. `validKey(d.Key)` in `validate` ergänzen (T-07-02, eine Zeile plus Test).
2. T-07-26 (Plan 06) entscheiden: den Mechanismus bauen oder die Behauptung auf
   das zurückschreiben, was wirklich schützt.
3. W-3 (`JoinValues`) und Verifikations-Gap 1 zusammen entscheiden — **vor**
   Phase 9.
4. Danach `/gsd-secure-phase 7` erneut fahren.
