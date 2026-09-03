# Was noch fehlt

Stand: August 2026, nach den eigenen Bausteinarten. Eine Arbeitsliste, keine
Wunschliste: jeder Punkt sagt, **was fehlt**, **wo es hingehört** und **wie
gross es ist**. Was bewusst nicht gebaut wird, steht ganz unten — damit
niemand es zweimal vorschlägt.

Der Vergleich mit Statamic, aus dem die meisten Punkte stammen, steht in
`vergleich-statamic.md`. Die eine strukturelle Lücke von damals ist zu: eine
Website bestimmt ihr Inhaltsmodell inzwischen selbst — eigene Felder, Gruppen,
Inhaltsarten, Abschnitte, Bedingungen und Bausteinarten. Was hier steht, ist
Einzelarbeit, keine Bauart.

---

## 1. Auswahl auch als Knopfreihe und als Mehrfachauswahl

**Fehlt:** Die Feldart *Auswahl* (`field.KindChoice`) ist immer eine
Klappliste. Für drei Möglichkeiten nebeneinander ist das die falsche Form, und
„mehrere davon" gibt es gar nicht.

**Wo:** `internal/field/field.go` (zwei neue Arten neben `KindChoice`),
`internal/field/render.go` (eine Mehrfachauswahl ist eine Liste, kein String —
das ist die eigentliche Entscheidung), `cmd/holzcloud/templates/admin/field_input.html`.

**Grösse:** Die Knopfreihe ist ein Nachmittag. Die Mehrfachauswahl ist mehr:
sie ist der erste Feldwert, der kein einzelner String ist, also braucht
`field.Data` einen zweiten Speicherweg oder eine Kodierung, auf die man sich
festlegt. Vorschlag: eine Zeile je Wert im selben String, wie `SplitChoices`
die Möglichkeiten schon liest.

## 2. Schlagwörter als Feldart

**Fehlt:** Schlagwörter gibt es an jeder Seite (`internal/term`), aber man kann
kein Feld „Sorte" anlegen, das aus ihnen wählt.

**Wo:** neue Art `KindTerm` in `internal/field`, ein Chooser wie der von
`KindRef` (siehe `refPages` in `internal/admin/page_fields.go`), Auflösung in
`internal/field/render.go` über einen `TermLookup` neben `Links.Page`.

**Grösse:** Ein Tag. Das Muster steht komplett beim Verweis; abzuschreiben ist
es einmal.

## 3. Textbausteine können nur Text

**Fehlt:** Ein Textbaustein (`internal/snippet`) ist ein Markdown-Feld. Eine
globale Telefonnummer mit Prüfung, ein globales Bild, eine globale Zahl gibt es
nicht — Statamics *Globals* tragen jeden Feldtyp.

**Wo:** `internal/snippet`. Der ehrliche Weg ist derselbe wie bei den
Bausteinarten: die Felder aus `page_field_defs` wiederverwenden, mit einer
Spalte `snippet_id` daneben — nicht eine dritte Feldtabelle.

**Grösse:** Zwei Tage, davon die Hälfte Bildschirm.

## 4. CSV-Import

**Fehlt:** Es gibt den eigenen Bündel-Import und WordPress (WXR). Eine Tabelle
mit Titel, Text und ein paar eigenen Feldern kann man nicht einlesen.

**Wo:** neben `internal/wxr` ein `internal/csv`, angehängt an denselben
Bildschirm (`cmd/holzcloud/templates/admin/website_list.html`).

**Grösse:** Ein Tag. Die Zuordnung Spalte → Feld ist die ganze Arbeit; alles
danach ist `page.CreatePage`.

## 5. Statischer Export

**Fehlt:** Eine Website als reine HTML-Dateien.

**Wo:** ein neuer Befehl neben den anderen in `runCLI`, der den öffentlichen
Handler gegen einen Ordner laufen lässt.

**Grösse:** Ein Tag für Seiten, Archiv, Feed, Sitemap und Medien. **Vorher
überlegen:** es ist eine zweite Betriebsart neben der einen, die funktioniert —
Formulare, Suche und geschützte Seiten kann sie nicht. Wenn, dann als
ausdrücklich abgespeckte Ausgabe.

## 6. Feldtypen, die einzeln fehlen

Klein, jeder für sich eine Stunde, alle in `internal/field`:

- `zeit` — eine Uhrzeit. Gibt es nur in der Zeitsteuerung.
- `bereich` — eine Zahl zwischen zwei Grenzen, als Schieber.
- `code` — ein Textfeld ohne Markdown, mit fester Schrift.

## 7. Dependabot: erledigt, und wie es das nächste Mal geht

`#3`–`#8` sind am 30. August eingegangen, `#13` (`modernc.org/sqlite`
1.56 → 1.57) und `#14` (`golang.org/x/net` 0.57 → 0.58) am 2. September.
Zurzeit steht keiner offen.

**Vorgehen beim nächsten Mal:** einer nach dem anderen, `go build ./...`,
`go vet ./...` und `go test ./...` dazwischen. Der Konflikt in `go.mod` ist
der Normalfall, sobald zwei PRs von derselben Fassung ausgehen — beide
Anhebungen behalten, `go mod tidy`, fertig.

Bei `modernc.org/sqlite` zusätzlich ein Lauf gegen eine echte Datenbankdatei;
das ist der eine Baustein, bei dem ein stiller Unterschied teuer wäre. Binär
bauen, gegen ein leeres Datenverzeichnis starten und nachsehen, ob alle
Wanderungen durchlaufen und die Pragmas stehen:

```
HOLZCLOUD_DATA_DIR=/tmp/dbtest HOLZCLOUD_PORT=18099 ./holzcloud
```

Erwartet: `journal_mode=wal`, `busy_timeout=5000`, `foreign_keys=1`,
`synchronous=1`, dazu `PRAGMA foreign_key_check` ohne Zeile und
`PRAGMA integrity_check` gleich `ok`. Bei 1.57.0 geprüft: 44 Wanderungen,
48 Tabellen, alles sauber.

---

## Was bewusst nicht gebaut wird

Damit es nicht wiederkommt:

- **Befehlspalette (⌘K) und Passkeys** — beides braucht JavaScript. Der Gewinn
  wiegt die Ausnahme nicht auf.
- **YouTube- und Vimeo-Einbettung** — die Regel „nichts von Dritten zur
  Laufzeit" ist der Grund, warum dieses CMS ohne Cookie-Banner auskommt. Ein
  Einbettungscode kostet genau das. Ein eigenes MP4 gibt es als Baustein.
- **GraphQL, OAuth, Git-Automatik** — jedes davon ist eine zweite Betriebsart
  neben der einen, die funktioniert.
- **Eine HTML-Vorlage je Bausteinart** — das wäre eine Vorlagensprache in einem
  Textfeld, also ein Weg, ein `<script>` durch die Vordertür auf eine Seite zu
  bringen. Das ganze Programm steht auf dem Versprechen, dass es so einen Weg
  nicht gibt. Wo eine CSS-Klasse nicht reicht, ist das lange Textfeld der
  Ausweg: es läuft durch den Markdown-Renderer und dieselbe Reinigung wie jeder
  andere Text.
- **Ein Verweis in einem Baustein** — ein Verweis überlebt eine Umbenennung;
  ein Baustein wird beim Speichern der Seite ein für alle Mal in HTML
  verwandelt und könnte das nicht halten. Der Link tut dieselbe Arbeit und sagt,
  was er ist.

---

## Beim Weiterarbeiten

- **Migrationen** laufen bis `00044`. Eine neue Wanderung, die eine bestehende
  Tabelle ändert, zuerst gegen `internal/db/migrations/00029` und `00031` lesen:
  eine CHECK-Bedingung am Tabellenkopf lässt sich in SQLite nur mit einem
  vollständigen Neubau lockern, und `pages` hat Fremdschlüsselkinder. Ein
  eigener Index dagegen — wie die Eindeutigkeit der Feldkennung — ist ein
  Austausch von zwei Zeilen.
- **Nach jeder Änderung an Texten:** `go run ./tools/i18n` zeigt, was in den
  fünf Sprachen fehlt, `-write` legt die Schlüssel an, `-schweiz` baut
  `de-CH.json` neu. Der Lauf muss „0 offen, 0 verwaist" sagen.
- **Geprüft wird im Browser.** Die Fehler dieser Woche — ein Beitrag, der beim
  Bildeinfügen zur Seite wurde; Bausteine, die im Bündel fehlten; Menüs, die
  beim Import zusammenstiessen — hat keiner der Tests gefunden, sondern ein
  Durchlauf durch die laufende Anwendung.
