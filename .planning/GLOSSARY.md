# Glossar — die eine deutsch-englische Wortliste dieses Projekts

**Zweck.** Das Projekt ist quelloffen, also spricht der Code Englisch. Diese
Datei ist die **einzige** Stelle, an der festgelegt ist, welches englische Wort
für welchen deutschen Begriff steht. Ohne sie wählen zwei Bearbeiter zwei Wörter
für dieselbe Sache, und genau das ist der Schaden, den die Umstellung beheben
soll.

**Regel.** Wer einen hier gelisteten Begriff übersetzt, nimmt die rechte Spalte —
nicht ein Synonym, das im Satz besser klingt. Wer einen Begriff übersetzt, der
hier fehlt, **trägt ihn ein**, im selben Commit.

**Was ausdrücklich NICHT übersetzt wird:** nichts. Der Entwickler hat am
2026-09-06 den vollen Umfang gewählt — Kommentare, Bezeichner, Testnamen, den
Vorlagen-Vertrag (harter Bruch mit Fassungssprung) und die SQL-Spalten samt
Katalogschlüsseln.

---

## Die Kernbegriffe des Inhaltsmodells

| Deutsch | Englisch | Anmerkung |
|---|---|---|
| Kennung | `key` | Der Schlüssel eines Feldes. SQL-Spalte `kennung` → `key` |
| Beschriftung | `label` | Was an einem Feld steht. **Nicht** `label` für Schlagwort — siehe unten |
| Art | `kind` | `art` als SQL-Spalte → `kind`. Nicht `type`: `type` ist in Go belegt |
| Bausteinart | `block kind` / `block_type` | Die Tabelle heisst schon `block_types` |
| Baustein | `block` | |
| Textbaustein | `snippet` | Das Paket heisst schon `snippet`. **Nie** `text block` |
| Schlagwort | `term` | Das Paket heisst schon `term`. **Nie** `label`, **nie** `tag` |
| Feld | `field` | |
| Felder (Vertrag) | `Fields` | `.Page.Felder` → `.Page.Fields` |
| Feldliste | `FieldList` | |
| Bausteinfelder | `SnippetFields` | `.Site.Bausteinfelder` → `.Site.SnippetFields` |
| Bausteinliste | `SnippetList` | `.Site.Bausteinliste` → `.Site.SnippetList` |
| Übersetzungen | `Translations` | `.Page.Uebersetzungen` → `.Page.Translations` |
| Gruppe | `group` | |
| Abschnitt | `section` | |
| Auswahl | `choice` / `choices` | Einzahl die Art, Mehrzahl die Möglichkeiten |
| Mehrfachauswahl | `multi-choice` | Die Art heisst `mehrfachauswahl` → `multi` |
| Möglichkeit | `option` | |
| Wert / Werte | `value` / `values` | |
| Pflicht | `required` | |
| Hinweis | `hint` | |
| Bedingung | `condition` | |
| Darstellung | `display` | |
| gilt für | `applies_to` | |
| Bereich | `range` | Die Feldart. `min_wert`/`max_wert` → `range_min`/`range_max` |
| Verweis | `ref` | Auf eine eigene Seite |
| Link | `link` | Auf eine beliebige Adresse |

## Seiten, Websites, Vorlagen

| Deutsch | Englisch | Anmerkung |
|---|---|---|
| Seite | `page` | |
| Beitrag | `post` | |
| Website | `website` | Bleibt. **Nicht** `site` — `Site` ist der Vorlagen-Vertrag |
| Vorlage (Theme) | `theme` | Was eine Website anzieht |
| Vorlage (Muster) | `template` | Eine Go-Vorlage oder eine Beispieldatei |
| Entwurf | `draft` | |
| Veröffentlicht | `published` | |
| Adresse (Slug) | `slug` | |
| Mediathek | `media library` | |

## Der Einlesevorgang (Phase 9)

| Deutsch | Englisch | Anmerkung |
|---|---|---|
| Zeile | `row` | Eine Datenzeile. **Nicht** `line` — Zeilennummer ≠ Zeilennummer der Datei |
| Zeilennummer | `RowNumber` | Die des Tabellenprogramms: Kopfzeile ist 1 |
| Spalte | `column` | |
| Kopfzeile | `header` | |
| Leser | `Reader` | |
| Zuordnung | `mapping` | Spalte → Ziel |
| Urteil | `verdict` | Was mit einer Zeile geschieht |
| Grund | `reason` | Warum. Als **Code** plus Argumente, nie als fertiger Satz (D-32) |
| Probelauf | `dry run` | |
| Ablage | `staging` | Der Vorgang und die zwischengelagerte Datei. Der **Typ**, der eine einzelne davon ist, heisst `Upload` — `csvimport.Ablage` → `csvimport.Upload` |
| Marke | `token` | |
| Beispieldatei | `example` | |
| entschärfen | `defuse` | Gegen Formelauswertung im Tabellenprogramm |
| abgeschnitten | `truncated` | Passend zu `wxr.Export.Truncated` |
| Bericht | `report` | |
| übergehen | `skip` | |
| anlegen | `create` | |
| aktualisieren | `update` | |
| Zelle | `cell` | |
| Nullbyte | `NUL byte` | Das Byte heisst NUL, nicht null |
| Grenze | `limit` | `MaxSpalten` → `MaxColumns`, `Zellengrenze` → cell limit |
| Fehler | `error` | Auch als Feld: `Zeile.Fehler` → `Row.Error` |
| Abwehr | `defence` | Britische Schreibung, wie der Paketkommentar sie schon führt |
| Wache | `guard` | Was ein Test über eine Datei von aussen hält |
| Musterzeile | `sample row` | Die Zeilen in der Beispieldatei |
| Überschrift | `heading` | Die einzelne Spaltenüberschrift; die ganze Zeile ist `header` |
| Trennzeichen | `separator` | |
| Anführungszeichen | `quote` / `quoting` | |
| Zeilenumbruch | `line break` | |
| Tabellenprogramm | `spreadsheet program` | Excel und LibreOffice |
| Ausfertigung (einer Regel) | `copy` | Die zweite Niederschrift derselben Regel |
| Angriffsfläche | `attack surface` | |
| Bediener / Bedienerin | `operator` | Wer den Import bedient |
| Modus | `Mode` | SQL-Spalte `modus` bleibt bis Phase 12 stehen |
| Kollision | `Collision` | SQL-Spalte `kollision` bleibt bis Phase 12 stehen |
| Dateiname | `Filename` | SQL-Spalte `dateiname` bleibt bis Phase 12 stehen |
| Daten | `Data` | SQL-Spalte `daten` bleibt bis Phase 12 stehen |
| erstellt am | `CreatedAt` | SQL-Spalte `erstellt_am` bleibt bis Phase 12 stehen |
| fremd | `foreign` | `ErrFremd` → `ErrForeign`. Die Person dazu ist ein `stranger` |
| abgelaufen | `expired` | `ErrAbgelaufen` → `ErrExpired` |
| Benutzer | `user` | |
| Konto | `account` | |

## Vorgänge und Werkzeuge

| Deutsch | Englisch | Anmerkung |
|---|---|---|
| Wanderung | `migration` | |
| Prüfung / Tor | `check` / `gate` | |
| Archiv (Bundle) | `bundle` | Das Paket heisst schon `bundle` |
| Sicherung | `backup` | |
| Aufräumen | `prune` / `sweep` | `prune` für einen Auftrag, `sweep` für den Vorgang |
| Rückstand | `deferred item` | |
| Fenster (kaputtes) | `broken window` | `.planning/WINDOWS.md` |
| Katalog | `catalogue` | Die Übersetzungsdateien |
| Stehendes Tor | `standing gate` | |
| Rückwärtshälfte | `down half` | Der `-- +goose Down`-Teil einer Wanderung |
| Rücknahme | `rollback` | Das Fahren dieser Hälfte |
| Rückfahrt | `trip back up` | Das erneute Anwenden danach |
| Aufbewahrung | `retention` | Wie lange ein Aufräumlauf etwas stehen lässt |
| Gegenstand (im Schema) | `object` | Tabelle oder Index in `sqlite_master` |

## Wörter, die eine Falle sind

- **`Label`** übersetzt **`Beschriftung`**, nie `Schlagwort`. Der bestehende
  Katalog wirft beide auf `Label` — das ist ein Übersetzungsfehler, der beim
  Umdrehen der Quellsprache auffällt und dort behoben wird.
- **`Time`** übersetzt **`Uhrzeit`**; `Zeitpunkt` heisst `point in time` oder
  wird umformuliert. Auch das wirft der Katalog heute zusammen.
- **`Crop`** übersetzt **`Zuschnitt`** (das Ergebnis); `Zuschneiden` ist `crop`
  als Tätigkeit und `Ausschnitt` ist `crop area`.
- **`Type`** ist in Go belegt. `Art` heisst deshalb `kind`, durchgehend.
- **`Site`** ist der Name des Vorlagen-Vertrags (`.Site.…`). Eine Website heisst
  im Code `website`, nie `site`.

---

## Herkunft

Angelegt 2026-09-06, als der Entwickler die Umstellung auf Englisch beauftragt
hat. Die Kollisionsliste unten stammt aus einer Messung von `en.json`: von 1158
deutschen Schlüsseln fallen genau **neun** auf einen gemeinsamen englischen Wert
zusammen, und drei davon sind vorbestehende Übersetzungsfehler.

| Englisch | Deutsche Schlüssel, die darauf fallen |
|---|---|
| `%s only` | `Nur %s`, `nur %s` |
| `.` | `an.`, `fest.` — **beides kaputt** |
| `Applies to` | `Gilt für`, `Wirkt auf` |
| `Apply` | `Anwenden`, `Übernehmen` |
| `Crop` | `Ausschnitt`, `Zuschneiden`, `Zuschnitt` |
| `Label` | `Beschriftung`, `Schlagwort` — **falsch zusammengeworfen** |
| `Reset link` | `Link zum Zurücksetzen`, `Reset-Link` |
| `Time` | `Uhrzeit`, `Zeitpunkt` |
| `– none –` | `– keines –`, `– keins –` |
