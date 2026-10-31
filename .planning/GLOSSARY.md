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
| Sprache / Sprachen | `language` / `Languages` | `.Site.Sprachen` → `.Site.Languages`. Gemessen 2026-09-11: der siebte und letzte deutsche Name im Vorlagen-Vertrag, in der Kriterienliste der Roadmap nicht genannt |
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
| Mehrzahl | `plural` | SQL-Spalte `mehrzahl` → `plural` (content_types). Am 2026-09-11 gefunden: die Kriterienliste nennt sie nicht |
| Archiv (einer Inhaltsart) | `archive` | SQL-Spalte `archiv` → `archive` (content_types). **Nicht** zu verwechseln mit `bundle`, dem Archiv als Datei |
| Sortierung | `sort_order` | SQL-Spalte `sortierung` → `sort_order` (content_types), passend zur gleichnamigen Spalte, die `album_items` und `menu_items` schon tragen |
| Verweis | `ref` | Auf eine eigene Seite |
| Link | `link` | Auf eine beliebige Adresse |
| Album | `album` | Eine Bilderreihe, die mehrere Seiten tragen können. **Nicht** `gallery` — die Galerie ist der Baustein, das Album ist der Vorrat, den er zeigt (Phase 11) |
| Diashow | `slideshow` | Die zweite Darstellung des Galerie-Bausteins: eine waagerechte Spur mit `scroll-snap`. **Nicht** `carousel` — es dreht sich nichts von selbst |
| Grossansicht | `large view` | Das grosse Bild, das ein Klick auf eine Kachel öffnet. Das, was der Besucher sieht |
| Lichtkasten | `lightbox` | Der Mechanismus dahinter: `:target` auf einer `<figure>`, kein `<dialog>`, kein JavaScript. **Nicht** synonym mit `Grossansicht` — das eine ist die Sache, das andere ihr Bauteil |

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
| Ziel | `Target` | Wohin eine Spalte zeigt. `nichts` ist eines davon und nicht dessen Fehlen |
| automatisch zuordnen | `AutoMap` | Der Vorschlag, den der Bildschirm überschreibt |
| Ausgang | `Outcome` | Was mit einer Zeile geschieht: `create`, `update`, `skip`. Nur im Speicher — die Werte in `kollision` bleiben deutsch |
| Anmerkung | `Note` | Warum eine Spalte unzugeordnet blieb. Code plus Argumente, wie ein Urteil |
| Gruppierungsschlüssel | `GroupKey` | Was zwei Zeilen mit derselben Aussage zu einer Zeile des Berichts macht |
| Schreiber | `Writer` | Der, der eine Zeile schreibt. **Nicht** `Store`: `csvimport.Store` ist schon die Ablage |
| vorhanden (Seite) | `existing` | Die Seite, die diese Adresse schon trägt |
| Wortschatz (geschlossen) | `closed vocabulary` | Eine Liste, ausserhalb derer gemeldet und nie geraten wird |
| kombinierendes Zeichen | `combining mark` | `unicode.Mn`. Das Setzen davor heisst `settleMarks` |
| Zustand (einer Seite) | `status` | Die gespeicherten Werte sind `draft` und `published`. Als Spaltenüberschrift erkennt der Einleser auch `Zustand` und `state` |
| zusammenfassen | `Summarize` | Aus Urteilen einen Bericht machen. **Nicht** `Aggregate` — der Vorgang fasst zusammen, er rechnet nicht |
| Durchgang / Lauf | `run` | Ein Gang durch die Datei. `csvLauf` → `csvRun` |
| Gesamt | `Total` | Wie viele Zeilen, in der Zusammenfassung wie in einer Gruppe |
| weitere (der Rest einer Liste) | `More` | Was eine gekürzte Aufzählung nicht mehr nennt. `Weitere` → `More` |
| Vorgabe | `default` | Was eingetragen wird, wenn die Zelle leer bleibt. Gehört dem **Ziel**, nicht der Spalte (IMP-08) |
| versteckte Felder | `hidden inputs` | Wie der Probelauf die Zuordnung an das Einlesen weitergibt |

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
- **`Type`** ist in Go belegt. `Art` heisst deshalb `kind` — mit **genau einer**
  aufgeschriebenen Ausnahme, und die steht in der Datenbank. `pages` trägt seit
  00014 ein englisches `kind`, und das heisst `page` oder `post`. Die Spalte
  `art` daneben (00036) hält die Kennung der **eigenen** Inhaltsart der Website
  — `produkt`, `termin` — und ist ein Verweis nach `content_types`. Zwei
  verschiedene Fragen, und die naheliegende Übersetzung der zweiten stösst
  frontal auf die erste. 00054 nennt sie deshalb `content_kind`. Die Regel
  „ein deutscher Begriff, genau ein englisches Wort" ist hier wissentlich
  gebogen; die Alternative wäre, die ältere und bereits richtige Spalte
  umzubenennen, um den Namen frei zu machen — mehr Code, schlechteres Schema.
  Im Vorlagen-Vertrag gibt es die Kollision nicht: dort heisst das Feld
  `.Page.Kind`, weil `PageContent` kein zweites `Kind` trägt.
- **`Site`** ist der Name des Vorlagen-Vertrags (`.Site.…`). Eine Website heisst
  im Code `website`, nie `site`.
- **`Weiter`** und **`Zurück`** sind vergeben. Der Katalog übersetzt sie seit
  langem als `Continue` und `Back` — die Wörter eines Assistenten, nicht die
  eines Bildes. Wer irgendwo ein Blättern baut, mintet eigene Zeichenketten:
  Phase 11 nahm `Vorheriges Bild` / `Nächstes Bild` (`Previous image` /
  `Next image`). Gemessen 2026-09-08 am Katalog, nicht vermutet. Hätte die
  Galerie `Weiter` wiederverwendet, stünde in vier Sprachen das falsche Wort
  auf einem Bild, und jedes Tor wäre grün geblieben.

---

## Herkunft

Angelegt 2026-09-06, als der Entwickler die Umstellung auf Englisch beauftragt
hat. Die Kollisionsliste unten stammt aus einer Messung von `en.json`: von 1158
deutschen Schlüsseln fielen genau **neun** auf einen gemeinsamen englischen Wert
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
| `Import finished` | `Einlesen abgeschlossen`, `Import abgeschlossen` | neu seit der Messung von 2026-09-06, gefunden am 2026-09-11 |
| `– none –` | `– keines –`, `– keins –` |

---

## Die eine Regel, die ein Laufzeitfehler gelehrt hat

**Ein deutsches Wort, das in der Datenbank *steht*, ist kein Bezeichner. Es ist
ein Wert.** Die Tabellen oben gelten fuer Bezeichner und Prosa. Sie gelten
**nicht** fuer eine Zeichenkette, die eine Zeile in der Datenbank ist oder in
einem Archiv reist.

Der Anlass, gefunden am 2026-09-06 beim Drehen von `internal/csvimport`: die
Wanderung `00049` nagelt zwei Wortschaetze fest --

```sql
modus     TEXT NOT NULL CHECK (modus     IN ('neu', 'bestehend')),
kollision TEXT NOT NULL CHECK (kollision IN ('uebergehen', 'aktualisieren')),
```

Das Glossar listet `uebergehen` -> `skip` und `aktualisieren` -> `update`. Wer
das als Anweisung liest, uebersetzt sie im Go-Code, **es baut sauber**, und dann
weist SQLite die Zeile zur Laufzeit zurueck -- auf einem Weg, den keine Pruefung
dieser Phase befaehrt. Das war die einzige Stelle, an der eine plausible Lesart
dieser Datei einen Produktionsfehler statt eines Compilerfehlers ergeben haette.

**Diese Werte bleiben deutsch, bis eine Wanderung sie aendert. Ihre Uebersetzung
ist eine Datenwanderung, keine Umbenennung.**

### Was sonst noch festgenagelt ist -- gemessen 2026-09-06

Ueber alle 49 Wanderungen tragen **nur zwei** CHECK-Klauseln deutsche Werte, und
beide stammen aus `00049`. Alles andere ist laengst englisch (`page`, `post`,
`draft`, `published`, `admin`, `editor`, `open`, `paid`, ...).

**Aber rund 25 deutsche Zeichenketten im Go-Code sind gespeicherte Werte**,
keine Bezeichner:

| Wortschatz | Werte | Wo sie liegen |
|---|---|---|
| Feldart (`page_field_defs.art`) | `text` `langtext` `code` `zahl` `bereich` `datum` `zeit` `janein` `auswahl` `mehrfachauswahl` `bild` `link` `verweis` `schlagwort` `gruppe` `abschnitt` | jede Felddefinition jeder Website |
| Geltung (`gilt_fuer`) | `beides` `seite` `beitrag` | dieselbe Tabelle |
| Darstellung | `knopfreihe` | dieselbe Tabelle |
| Bausteinart | `galerie` `karten` `zitat` `aufruf` `bildtext` `trenner` `video` | jede Seite mit Bausteinen |

Der Sprengradius der Bausteinarten reicht **ueber die Datenbank hinaus**: sie
erscheinen als CSS-Klassen in allen acht Themes -- `hc-aufruf` 20x, `hc-zitat`
13x, `hc-video` 13x, `hc-bildtext` 11x, `hc-karten` 7x, `hc-galerie` 6x,
`hc-trenner` 4x.

Diese Wortschaetze zu drehen ist deshalb **kein Teil des Umbenennens**, sondern
ein eigenes Vorhaben mit Datenwanderung, Archiv-Vertraeglichkeit und
Theme-Bruch. Phase 12 entscheidet es ausdruecklich -- sie darf es nicht nebenbei
tun und sie darf es nicht stillschweigend lassen.
