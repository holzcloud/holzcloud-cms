# Holzcloud CMS

A minimal, self-hosted CMS for a small linux/amd64 server. Single Go binary. Manages multiple websites with multiple domains. Admin UI via htmx. Public site via server-rendered templates. SQLite storage.

## Quick Start

```bash
# Build
go build ./cmd/holzcloud

# Run
./holzcloud

# Open http://localhost:8080/admin — creates the first admin account
```

## Features

- **Multi-Site** — manage multiple websites with multiple domains from one instance
- **Eigene Felder** — jede Website bestimmt selbst, woraus ihre Seiten bestehen:
  neun Eingabearten plus wiederholbare Gruppen, Abschnitte und Bedingungen
  („Sonderpreis“ erst, wenn „Im Angebot“ angekreuzt ist), im Editor und im Theme, ohne
  neue Programmversion. Darunter der **Verweis**: eine Seite dieser Website,
  ausgewählt statt eingetippt — er folgt einer Umbenennung und verschwindet,
  wenn das Ziel verschwindet
- **Eigene Bausteinarten** — der Bausteineditor kann neun Arten; eine zehnte —
  Rezeptschritt, Öffnungszeiten-Kasten, Preiszeile — legt jede Website selbst
  an, mit denselben Feldern wie eine Seite und einer CSS-Klasse fürs Theme
- **Eigene Inhaltsarten** — neben *Seite* und *Beitrag* legt jede Website ihre
  eigenen an: Produkt, Termin, Rezept, Tier. Mit eigenem Namen im ganzen
  Bildschirm, eigener Übersichtsseite, eigenen Feldern und eigenem Filter
- **Hofladen** — ein Bestellformular über die eigenen Produkte, als Plugin; ein
  Produkt ist eine Seite mit einem Preisfeld
- **Mehrsprachigkeit** — eine Website in mehreren Sprachen: die Hauptsprache
  behält ihre Adressen, jede weitere liegt unter ihrem Kürzel (`/fr/contact`).
  Seiten, Menüs, Archiv und Feed je Sprache, mit `hreflang` und Sprachwahl
- **Übersetzte Verwaltung** — die Verwaltung selbst spricht Deutsch, Englisch,
  Französisch, Italienisch und Spanisch, je Person einstellbar; eine weitere
  Sprache ist eine Datei im Ordner `data/sprachen`, ohne Neubau und ohne Neustart
- **Schweizer Fassungen** — `de-CH`, `fr-CH` und `it-CH` liegen neben ihren
  Grundsprachen: kein ß, «spitze Anführungszeichen», Natel statt Handy. Eine
  Regionalfassung trägt nur die Abweichungen, der Rest kommt aus der
  Grundsprache — und ein Schweizer Browser bekommt sie von allein
- **Host-Based Routing** — incoming requests are routed by `Host` header to the correct website
- **Markdown Pages** — author content in Markdown, rendered via goldmark and sanitized via bluemonday
- **Template System** — upload custom `.zip` template archives per website, with a built-in default template
- **Hierarchical Menus** — create nested navigation menus per website with up/down reordering
- **Media Management** — upload images and files per website with MIME validation, paginated
- **SEO** — per-site `sitemap.xml` and `robots.txt`, generated from published pages
- **User Management** — admin and editor roles with Argon2id password hashing
- **Rechte je Person** — wer welche Websites betreten darf, und wer
  veröffentlichen darf. Beides innerhalb der Rolle, durchgesetzt vor jedem
  Zugang und nicht bloss in der Menüführung
- **Two-factor authentication** — compulsory for administrators, with printed
  recovery codes and a command-line way back in
- **Protected pages and preview links** — a page behind a password, and signed
  links that show a draft to someone without an account
- **Export and import** — a whole website as one readable archive, so a site is
  not a hostage to the machine it runs on
- **Per-site design** — colours, typeface, text width and corner rounding,
  validated to a shape the CSS can read only one way
- **Structured data** — schema.org JSON-LD with the business address, opening
  hours and telephone number, so a search result shows more than a blue link
- **Single Binary** — all templates, assets, and migrations embedded via `embed.FS`
- **SQLite** — pure-Go SQLite via `modernc.org/sqlite`, no CGO required
- **htmx** — progressive enhancement with htmx 2.0; works fully without JavaScript

## Configuration

All settings via environment variables (prefix `HOLZCLOUD_`):

| Variable | Default | Description |
|----------|---------|-------------|
| `HOLZCLOUD_PORT` | `8080` | HTTP listen port |
| `HOLZCLOUD_DATA_DIR` | `data` | Directory for SQLite DB, media, and templates |
| `HOLZCLOUD_LOG_LEVEL` | `INFO` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `HOLZCLOUD_SECURE` | `false` | Set `true` behind TLS (enables Secure cookies) |
| `HOLZCLOUD_MAX_TEMPLATE_SIZE` | `10485760` | Max template archive size in bytes (10 MB) |
| `HOLZCLOUD_MAX_MEDIA_SIZE` | `5242880` | Max media file size in bytes (5 MB) |
| `HOLZCLOUD_MAX_VIDEO_SIZE` | `67108864` | Grösste Videodatei in Bytes (64 MB) |
| `HOLZCLOUD_MAX_MEGAPIXELS` | `24` | Größte Bildfläche, die für verkleinerte Fassungen dekodiert wird |
| `HOLZCLOUD_ARGON2_MEMORY` | `65536` | Argon2id memory cost in KB (64 MB) |
| `HOLZCLOUD_ARGON2_ITERATIONS` | `1` | Argon2id time cost |
| `HOLZCLOUD_ARGON2_PARALLELISM` | `2` | Argon2id parallelism |
| `HOLZCLOUD_SMTP_HOST` | — | Mailserver. Leer heisst: es werden keine E-Mails verschickt |
| `HOLZCLOUD_SMTP_PORT` | `587` | Port des Mailservers |
| `HOLZCLOUD_SMTP_USER` | — | Anmeldename, leer für einen Relay ohne Anmeldung |
| `HOLZCLOUD_SMTP_PASSWORD` | — | Passwort dazu |
| `HOLZCLOUD_SMTP_FROM` | — | Absenderadresse. Zusammen mit `_HOST` zu setzen |
| `HOLZCLOUD_SMTP_FROM_NAME` | — | Anzeigename des Absenders |
| `HOLZCLOUD_SMTP_TLS` | `starttls` | `starttls`, `tls` oder `none` |

### E-Mail

Der Versand ist aus, solange `HOLZCLOUD_SMTP_HOST` und `HOLZCLOUD_SMTP_FROM`
nicht beide gesetzt sind. Dann verhält sich alles wie zuvor: ein Einladungs-
oder Passwortlink wird einmal auf dem Bildschirm gezeigt und von Hand
weitergegeben.

Mit Versand geht dieser Link zusätzlich per Mail hinaus — er steht trotzdem auf
dem Bildschirm, weil E-Mail verzögert oder gefiltert werden kann. Ist bei einer
Website unter *Einstellungen* eine Benachrichtigungsadresse hinterlegt, meldet
das Kontaktformular-Plugin dorthin jede neue Anfrage.

Das ist die einzige Stelle, an der dieser Server von sich aus nach aussen wählt,
und zwar zu genau dem einen Server, der hier eingetragen ist. Für den Browser
eines Besuchers ändert sich nichts: jede Ressource kommt weiterhin von diesem
Server, die Content-Security-Policy bleibt unverändert. Verschickt wird reiner
Text, nie HTML — Zählpixel und nachgeladene Bilder haben in einem Postfach so
wenig zu suchen wie auf einer Seite.

Der Zustand steht in der Verwaltung unter *E-Mail*: dort liegt auch ein
Testversand an die eigene Adresse.

### Video aus der eigenen Mediathek

Ein Bausteintyp **Video** zeigt ein hochgeladenes MP4 in einem `<video>`-Element
— mit `controls`, ohne `autoplay`, mit `preload="metadata"`, also ohne den Film
selbst zu laden, bevor jemand darauf drückt. Dazu wahlweise ein Vorschaubild
und eine Bildunterschrift.

Kein YouTube und kein Vimeo: ein eingebetteter Rahmen ist genau das, was die
Regel „nichts von Dritten zur Laufzeit" verbietet — und der Grund, warum diese
Seiten ohne Cookie-Banner auskommen. Eine eigene Datei braucht beides nicht.

MP4 bis 64 MB (`HOLZCLOUD_MAX_VIDEO_SIZE`), Bilder bleiben bei 5 MB. Und wie
bei Fotos gilt: **die Aufnahme verliert ihre Metadaten**, bevor sie auf der
Platte liegt. Ein MP4 verweist mit absoluten Byte-Positionen auf sich selbst,
also wird nichts herausgeschnitten — die Box mit dem Aufnahmeort heisst danach
`free` und ist mit Nullen gefüllt. Gleiche Länge, gleiche Positionen, keine
Koordinaten.

### Von WordPress umziehen

Ob jemand überhaupt ankommt, entscheidet sich vor der ersten Anmeldung. Unter
*Websites → Von WordPress umziehen* nimmt das CMS eine WXR-Datei entgegen
(in WordPress: *Werkzeuge → Daten exportieren → Alle Inhalte*):

- Seiten und Beiträge mit Titel, Adresse, Text, Kurzfassung, Zustand,
  Datum und Schlagwörtern
- Shortcodes (`[gallery …]`) werden entfernt statt geraten
- alles, was nicht veröffentlicht war, kommt als **Entwurf** an
- Anhänge, Menüpunkte und Papierkorb werden übergangen und gezählt
- es entsteht immer eine **neue** Website, wie beim eigenen Import

**Die Bilder bleiben zurück** — bewusst. Sie liegen auf dem alten Server, und
dieser hier holt von sich aus nichts von Dritten. Der Bericht zählt die
Adressen auf; die Dateien kommen unter *Medien* herüber. Zwanzig Dateien von
Hand sind besser als eine gebrochene Regel.

Der Inhalt kommt als HTML herein und wird als Quelltext der Seite gespeichert.
Das ist kein Kompromiss: Markdown lässt HTML durch, und alles geht durch
denselben Filter wie jeder andere Text — ein `<script>`, das WordPress
mitgeführt hat, überlebt die erste Ausgabe nicht.

### Marke der Verwaltung

Unter *Marke* bekommt die Verwaltung den Namen der Anlage: das Wort in der
Ecke, die zweite Hälfte jedes Fenstertitels und die Überschrift auf dem
Anmeldebildschirm. Dazu ein Zeichen (ein oder zwei Buchstaben für das Quadrat)
und wahlweise ein Logo — PNG, WebP oder SVG, höchstens 512 KB.

Ein SVG ist ein Dokument, das Skript tragen kann, und es wird aus diesem Server
ausgeliefert. Darum wird es **geprüft und nicht entschärft**: was `<script>`,
`onload=` oder `javascript:` enthält, wird abgelehnt statt gesäubert. Und die
Prüfung sieht auf die Bytes, nicht auf den Dateinamen.

Das betrifft nur die Verwaltung. Wie die Websites selbst aussehen, steht unter
*Design*.

### Passwort noch einmal, vor dem Unwiderruflichen

Eine Sitzung ist einen Arbeitstag lang, weil alles Kürzere dazu führt, dass
Passwörter auf Zetteln stehen. Der Preis ist der offene Laptop im geteilten
Büro. Die Antwort darauf ist keine kürzere Sitzung, sondern **eine Frage vor
den wenigen Knöpfen, die etwas zerstören**:

- eine Website löschen
- einen Benutzer löschen
- einen KI-Schlüssel ausstellen
- ein Plugin entfernen

Dann kommt ein Bildschirm, der nach dem Passwort fragt; danach ist man 15
Minuten bestätigt und landet wieder dort, wo man war. Der Knopf wird nicht
heimlich nachgeholt — man drückt ihn noch einmal, und diesmal geht er. Das
spart eine Sitzung voller zwischengelagerter Formulare und ist ehrlicher.

Absichtlich vier Knöpfe. Eine Rückfrage, die überall kommt, liest niemand mehr,
und dann schützt sie nichts.

### Rechte je Person

Zwei Rollen bleiben, weil zwei richtig sind: **Administrator** führt die Anlage,
**Redakteur** die Inhalte. Was in der Praxis gefehlt hat, sind nicht mehr Rollen,
sondern zwei Grenzen *innerhalb* der einen — beide unter *Benutzer → Bearbeiten*:

| Recht | Bedeutung |
|---|---|
| **Websites** | Nichts angekreuzt: alle. Sonst genau diese — überall, auch über eine von Hand eingetippte Adresse. |
| **Darf veröffentlichen** | Ohne dieses Recht wird geschrieben und zur Prüfung eingereicht; online stellt jemand anderes. |

Ein Administrator hat keine Grenzen: die Rolle *ist* das Recht, die Anlage zu
führen, und eine Website, die ein Administrator nicht betreten darf, wäre eine,
die niemand reparieren kann.

**Wie es durchgesetzt wird.** Die Websiteprüfung ist *eine* Middleware über der
ganzen Verwaltung und liest die Adresse: alles unter `/admin/websites/<Nummer>`
gehört zu dieser Website, ganz gleich, welche Route dahinter liegt. Der
Unterschied ist nicht Bequemlichkeit — es gibt über sechzig solche Routen, und
eine Regel, die an sechzig Stellen steht, ist eine Regel, die an einer davon
fehlt. Genau so ist in diesem Projekt schon einmal ein fehlendes `requireAdmin`
beim Löschen einer Website durchgerutscht.

Das Veröffentlichungsrecht sitzt an den drei Stellen, an denen ein Zustand
wechselt: im Editor, im Zeilenmenü und in der Sammelaktion. Wer es nicht hat,
ändert den Zustand einer Seite überhaupt nicht — auch nicht nach unten. Ein
stilles Zurückstufen wäre schlimmer als ein abgelehnter Knopf: eine laufende
Seite verschwände beim Korrigieren eines Tippfehlers.

Was jemand nicht darf, wird auch nicht gezeigt: der Umschalter, die
Website-Liste und die Übersicht führen nur die eigenen Websites, samt Zahlen.
Eine Liste, die beim Anklicken 403 sagt, ist keine Liste, sondern eine Falle.

### Eigene Felder

Eine Seite hat Titel, Adresse und Text. Was darüber hinaus zu einer Seite
*dieser* Website gehört, legt der Betreiber selbst fest — unter *Felder*:

| Art | Wofür |
|---|---|
| Kurzer Text | ein Name, eine Menge, eine Sorte |
| Langer Text | mehrere Zeilen ohne Formatierung |
| Zahl | ein Preis, ein Gewicht, ein Jahrgang |
| Datum | ein Tag, ohne Uhrzeit |
| Ja/Nein | verfügbar, vergriffen |
| Auswahl | eine Liste von Möglichkeiten |
| Bild | ein Bild aus der Mediathek dieser Website |
| Link | eine eigene Seite oder eine fremde Adresse |
| Verweis | eine Seite dieser Website, ausgewählt statt eingetippt — siehe unten |
| Gruppe | mehrere Felder, mehrfach ausgefüllt — siehe unten |
| Abschnitt | keine Eingabe, sondern eine Überschrift über den folgenden Feldern |

Ein Feld gilt wahlweise für Seiten, für Beiträge, für beides — oder für genau
eine eigene Inhaltsart. Ein Preis gehört an ein Produkt, ein Autor an einen
Beitrag; beides überall abzufragen macht das Formular länger, ohne dass es mehr
kann.

#### Abschnitte und Bedingungen

Zwanzig Felder untereinander sind eine Wand. Zwei Angaben machen daraus ein
Formular, das jemand ausfüllt:

Ein **Abschnitt** ist eine Überschrift zwischen den Feldern — er ist selbst ein
Feld, nur eines ohne Eingabe. Alles darunter steht bis zum nächsten Abschnitt
unter dieser Überschrift.

Eine **Bedingung** lässt ein Feld erst erscheinen, wenn ein anderes ausgefüllt
ist: *Sonderpreis* erst, wenn *Im Angebot* angekreuzt ist. Solange das Feld
nicht erscheint,

- wird es **nicht verlangt**, auch wenn es ein Pflichtfeld ist,
- gibt das Theme es **nicht aus** — weder unter seiner Kennung noch in
  `.Feldliste`,
- **bleibt sein Wert stehen.** Ein versehentlich entferntes Häkchen kostet
  niemanden seine Eingabe; ein zweiter Klick, und alles ist wieder da.

Das Ein- und Ausblenden im Browser ist reines CSS — kein Skript, wie überall
hier. Das bestimmt, was eine Bedingung kann: **ausgefüllt oder nicht**, mehr
nicht. Auf einen bestimmten Wert zu prüfen bräuchte ein Stylesheet je Website,
also eine zweite Stelle, an der dieselbe Regel steht und mit der Zeit anders
lauten würde. Aus demselben Grund lässt sich eine Bedingung nicht an jedes Feld
hängen: an einem Datum kann der Browser nicht ablesen, ob es ausgefüllt ist. Der
Bildschirm bietet nur die Felder an, an denen es geht — die Regel muss niemand
kennen.

Geprüft wird ohnehin ein zweites Mal auf dem Server, mit derselben Funktion für
Verwaltung, Import und KI-Zugang. Ein Browser ohne `:has()` zeigt darum
schlimmstenfalls ein Feld zu viel und nie ein falsches Ergebnis.

#### Verweise auf eigene Seiten

Ein Link ist eine getippte Adresse. Sie geht in dem Moment ins Leere, in dem
jemand die Zielseite umbenennt, und niemand merkt es. Ein **Verweis** ist
stattdessen eine Auswahl aus dem eigenen Bestand:

```gotemplate
{{ with .Page.Felder.gehoert_zu }}
  Gehört zu <a href="{{ .URL }}">{{ .Title }}</a>
{{ end }}
```

Gespeichert wird die Seite, nicht ihre Adresse. Daraus folgt alles Weitere:

- **Umbenennen schadet nicht** — der Verweis zeigt weiter auf dieselbe Seite,
  unter ihrer neuen Adresse.
- **Titel und Adresse sind immer aktuell**, weil sie beim Ausliefern geholt
  werden und nicht beim Auswählen kopiert wurden.
- **Ein Entwurf bleibt ein Entwurf.** Zeigt ein Verweis auf eine unveröffentlichte,
  gelöschte oder mit Passwort geschützte Seite, sieht das Theme *nichts* — ein
  `{{ with }}` lässt den Block einfach weg. Der Editor sagt es vorher: „Diese
  Seite ist nicht veröffentlicht."
- **Im Export reist die Adresse**, nicht die Nummer. Auch ein Verweis auf eine
  Seite, die im Archiv erst später kommt, findet nach dem Import wieder sein Ziel.

Für mehrere Ziele: eine **Gruppe** mit einem Verweis darin — dann steht auch die
Reihenfolge fest und lässt sich mit den Pfeilen ändern.

#### Wiederholbare Gruppen

Ein einzelnes Feld reicht für einen Preis. Es reicht nicht für Öffnungszeiten,
Teammitglieder oder eine Preisstaffel. Dafür gibt es die Art **Gruppe**: man
legt sie an, bestimmt danach, woraus eine Zeile besteht, und füllt sie im
Editor so oft aus, wie es Zeilen gibt.

Genau eine Ebene — eine Gruppe in einer Gruppe wird abgelehnt. Verschachtelung
kostet einen Editor, der sich selbst aufruft, und ein Formular, in dem sich
niemand mehr zurechtfindet.

Die Knöpfe *Zeile hinzufügen*, *Entfernen* und die Pfeile sind gewöhnliche
Absenden-Knöpfe: der Server baut das Formular neu auf. Wie der Bausteineditor
funktioniert das ohne eine Zeile JavaScript.

```gotemplate
{{ range .Page.Felder.preisstaffel }}
  ab {{ .ab_menge }} {{ .einheit }}: CHF {{ .preis }}
{{ end }}
```

Die **Kennung** wird einmal aus der Beschriftung gebildet und steht danach
fest. Sie ist der Name, unter dem das Theme das Feld anspricht und unter dem
jeder gespeicherte Wert steht; würde sie sich mit der Beschriftung ändern, wäre
nach jeder Umformulierung alles Ausgefüllte still verschwunden.

Im Theme auf zwei Arten erreichbar:

```gotemplate
{{/* namentlich — für ein Theme, das für diese eine Website geschrieben ist */}}
{{ with .Page.Felder.preis_pro_kilo }}<p class="preis">CHF {{ . }}</p>{{ end }}
{{ if .Page.Felder.direkt_bestellbar }}<p>Direkt ab Hof</p>{{ end }}

{{/* als Liste — für ein mitgeliefertes Theme, das die Namen nicht kennen kann */}}
{{ range .Page.Feldliste }}<dt>{{ .Label }}</dt><dd>{{ .Text }}</dd>{{ end }}
```

Die Werte kommen typisiert an: eine Zahl als Zahl (und gedruckt so, wie sie
getippt wurde), ein Datum als `*time.Time` für `formatDate`, ein Ja/Nein als
`bool`, ein Bild als Struktur mit Adresse, Alt-Text und Fokuspunkt — oder `nil`,
wenn das Bild gelöscht wurde. Eine Gruppe kommt als Liste von Zeilen an, jede
Zeile mit ihren eigenen Feldern. Alle mitgelieferten Themes drucken die Liste
unter dem Text; ein eigenes Theme entscheidet selbst.

Geprüft wird beim Speichern, und zwar überall gleich: in der Verwaltung, beim
Import und über den KI-Zugang. Ein Preis, der keine Zahl ist, wird abgelehnt;
eine Auswahlmöglichkeit, die nicht in der Liste steht, ebenso — auch wenn
jemand das `<select>` umgeht. Bei einer Gruppe steht die Zeilennummer in der
Meldung: „Preisstaffel, Zeile 2: Preis muss eine Zahl sein.“

Ein gelöschtes Feld nimmt die Werte nicht mit. Sie stehen an den Seiten, bis
diese das nächste Mal gespeichert werden — wer sich vertan hat, legt das Feld
einfach wieder an.

Export und Import tragen beides mit: die Definitionen und die Werte. Ein
Bildfeld reist als Dateiname, nicht als Kennzahl, weil eine Kennzahl auf der
Zielmaschine nichts bedeutet.

### Eigene Inhaltsarten

Eine Website hat von Haus aus zwei Arten: die **Seite**, die für sich steht, und
den **Beitrag**, der im Archiv nach Datum liegt. Wer Produkte, Termine, Rezepte
oder Tiere führt, hat ein drittes Ding — und behilft sich sonst damit, es als
Seite zu führen und in Gedanken auseinanderzuhalten.

Unter *Inhaltsarten* legt jede Website ihre eigenen an:

| Angabe | Wofür |
|---|---|
| Name (Einzahl) | steht auf den Knöpfen: „Neues Produkt“ |
| Mehrzahl | steht über der Liste und im Filter |
| Kennung | fürs Theme; wird aus dem Namen gebildet und steht dann fest |
| Übersichtsseite | Adresse, unter der alle Einträge aufgezählt werden — `/hofladen`. Leer lassen für eine Art ohne Übersicht |
| Sortierung | neueste zuerst oder nach Titel |

Danach ist die Art überall da, wo sie hingehört: im Editor zur Wahl, als Spalte
und Filter in der Liste, als Ziel für ein Feld („gilt nur für Produkte“), als
öffentliche Übersicht und in der Sitemap. Im Theme:

```gotemplate
{{/* die Übersichtsseite benutzt list.html, wie das Archiv */}}
{{ range .Archive.Entries }}<h2><a href="{{ .URL }}">{{ .Title }}</a></h2>{{ end }}

{{/* an einem Eintrag: .Page.Art ist die Kennung, leer bei Seite und Beitrag */}}
{{ if eq .Page.Art "produkt" }}<p>CHF {{ .Page.Felder.preis }}</p>{{ end }}
```

Zwei Dinge ändert eine Art **nicht**, und das ist Absicht:

- **Die Adressen.** Ein Produkt wohnt unter `/wollpaket-gross`, nicht unter
  `/produkte/wollpaket-gross`. Eine Art zu vergeben oder wieder zu entfernen
  bricht deshalb keinen Link.
- **Den Bestand.** Wird eine Art entfernt, bleiben ihre Einträge stehen und
  tragen weiter ihre Kennung. Wer sich vertan hat, legt die Art wieder an.

Export, Import und der KI-Zugang tragen die Art mit.

### Eigene Bausteinarten

Der Bausteineditor bringt neun Arten mit — Text, Bild, Bild und Text, Galerie,
Karten, Zitat, Aufruf, Video, Trennlinie. Eine zehnte war bis jetzt eine neue
Programmfassung. Unter *Bausteinarten* legt jede Website ihre eigenen an:

1. **Art anlegen** — Name („Rezeptschritt"), Hinweis. Die Kennung entsteht aus
   dem Namen und steht danach fest.
2. **Felder bestimmen** — dieselben Felder wie bei einer Seite: Text, langer
   Text, Zahl, Datum, Ja/Nein, Auswahl, Bild, Link.
3. **Benutzen** — die Art steht im Editor hinter den eingebauten im Menü.

Im Theme kommt sie als Auszeichnung an, die man mit CSS anspricht:

```html
<div class="hc-block hc-eigen hc-eigen--rezeptschritt hc-ja--hervorheben">
  <p class="hc-eigen__zeile hc-eigen__zeile--nummer">Schritt 1</p>
  <div class="hc-eigen__text hc-eigen__text--anleitung"><h3>Teig ansetzen</h3>…</div>
  <figure class="hc-eigen__bild hc-eigen__bild--bild"><img …></figure>
</div>
```

Drei Regeln, die den Rest erklären:

- **Ein langer Text wird als Markdown ausgegeben.** Das ist der Weg zu einer
  Überschrift oder einer Aufzählung im Baustein — und der Grund, warum es keine
  Vorlage zum Selberschreiben braucht.
- **Ein Ja/Nein gibt nichts aus**, sondern setzt `hc-ja--kennung` am Baustein.
  Damit macht das Theme denselben Baustein hervorgehoben oder gewöhnlich.
- **Einen Verweis gibt es hier nicht.** Ein Verweis folgt einer Umbenennung;
  ein Baustein wird beim Speichern der Seite ein für alle Mal in HTML
  verwandelt und könnte das nicht halten. Ein Link sagt, was er ist.

Warum keine eigene HTML-Vorlage je Art: die wäre eine Vorlagensprache in einem
Textfeld, also ein Weg, ein `<script>` durch die Vordertür auf eine Seite zu
bringen. Das ganze Programm steht auf dem Versprechen, dass es so einen Weg
nicht gibt.

Export und Import tragen die Arten samt ihren Feldern mit — und die Bausteine
auf den Seiten dazu. Ein Bild darin reist als Dateiname, nicht als Kennzahl,
und die Seite wird auf der anderen Maschine neu gesetzt.

### Hofladen

Ein Bestellformular über die eigenen Produkte, als Plugin (`plugins/bestellung`).
Es bringt kein Produktmodell mit: **ein Produkt ist eine Seite, die ein
Preisfeld ausgefüllt hat.** Welches Feld das ist, steht auf dem Bildschirm des
Plugins — vorbelegt mit `preis`, `einheit` und `verfuegbarkeit`.

`[[bestellung]]` in einer Seite wird zur Produktliste mit Mengenfeldern und den
Feldern für Name, E-Mail, Telefon, Adresse und Bemerkung. Die Bestellung geht an
`/bestellung`, landet im Speicher des Plugins und löst eine E-Mail an die
Benachrichtigungsadresse aus — mit der Adresse des Bestellers als Antwortadresse,
damit ein Klick auf *Antworten* reicht.

Drei Entscheidungen, die anders sind als bei einem gewöhnlichen Shop:

- **Kein Warenkorb.** Wer bei einem Hof bestellt, wählt einmal aus und schickt
  ab. Ein Warenkorb bräuchte eine Sitzung je Besucher, ein Plätzchen und eine
  zweite Seite — und alles davon müsste stimmen, bevor die erste Bestellung
  ankommt.
- **Keine Bezahlung.** Ein Zahlungsanbieter wäre ein Aufruf nach draussen zur
  Laufzeit, also genau das, was dieses CMS ohne Cookie-Banner auskommen lässt.
  Bezahlt wird bei der Übergabe oder per Rechnung; der Hinweis darüber steht in
  den Einstellungen des Plugins.
- **Preise kommen aus der Seite, nicht aus dem Formular.** Aus dem Formular
  kommt die Menge und sonst nichts — sonst bestimmte der Besucher, was etwas
  kostet. Gespeichert wird der Preis von heute, damit eine Änderung morgen eine
  alte Bestellung nicht umschreibt.

Gegen Spam dasselbe wie beim Kontaktformular: ein Honigtopf, eine signierte
Zeitmarke (schneller als zwei Sekunden war kein Mensch, älter als zwölf Stunden
ist eine Seite mit womöglich alten Preisen) und eine Stundengrenze.

### Mehrsprachigkeit

Eine Website kann in mehreren Sprachen erscheinen. In den Einstellungen steht
die **Hauptsprache** und darunter das Feld **Weitere Sprachen** – Kürzel durch
Komma getrennt, `fr, it` oder `fr-CH`, höchstens acht.

Die eine Entscheidung, an der alles andere hängt:

> **Die Hauptsprache behält ihre Adressen.** `/kontakt` bleibt `/kontakt`, wenn
> jemand Französisch einschaltet. Jede weitere Sprache liegt unter ihrem
> Kürzel: `/fr/contact`.

Kein bestehender Link bricht, keine Weiterleitung wird nötig, und eine Website
mit einer Sprache merkt von der ganzen Sache nichts – kein Präfix, keine
Sprachwahl, kein zusätzliches Feld im Formular.

**Eine Fassung anlegen.** Im Seitenformular steht unter *Fassungen in anderen
Sprachen*, welche Sprachen es von dieser Seite schon gibt und welche fehlen.
*Fassung anlegen* kopiert die Seite als **Entwurf** in die andere Sprache –
mit Text, Bausteinen, eigenen Feldern und Vorschaubild. Übersetzt werden dann
Titel, Text und Adresse; die Adresse ist zunächst `kontakt-fr` und wird zu
`contact`. Ein Entwurf, weil eine halb übersetzte Seite nicht ins Netz gehört.

Die Fassungen bilden einen **Stern**: alle zeigen auf dieselbe Seite in der
Hauptsprache. Keine Kette – sonst wäre die Reihenfolge des Anlegens dauerhaft
eingebaut, und das Löschen einer mittleren Sprache zerrisse die Gruppe. Wer die
deutsche Seite löscht, löscht nicht auch die französische; die steht dann für
sich.

**Was je Sprache existiert:** Seiten, Menüs (dasselbe Hauptmenü noch einmal auf
Französisch), das Archiv, die Schlagwortseiten und der Feed unter
`/fr/feed.xml`. Fehlt in einer Sprache ein Menü, wird das der Hauptsprache
genommen – je Stelle einzeln, damit „Hauptmenü zuerst, Fussmenü später“
funktioniert. Schlagwörter selbst sind gemeinsam: ein Thema ist auf Französisch
dasselbe Thema, nur der Inhalt darunter ist ein anderer.

**Was eine fehlende Übersetzung tut:** `/fr/kontakt` antwortet 404, es zeigt
nicht die deutsche Seite. Ehrlich statt verwirrend – und die Sprachwahl bietet
nur an, was es wirklich gibt. Eine Sprachwahl, die Französisch anbietet und
dann 404 antwortet, ist schlechter als gar keine.

**Für Suchmaschinen** trägt jede Seite ihren eigenen `canonical` mit Präfix,
dazu `hreflang` für jede Fassung, die es wirklich gibt. Die Sitemap führt alle
Sprachen mit ihren Präfixen und zusätzlich jede Startseite.

**Für eine Vorlage** gibt es zwei Felder: `.Site.Sprachen` ist die
Sprachwahl – die Fassungen dieser Seite, wo es welche gibt, sonst die
Startseiten der Sprachen. `.Page.Uebersetzungen` sind nur die echten Fassungen,
daraus wird `hreflang`. Die Namen stehen in der Sprache selbst („Français“),
denn gelesen wird die Sprachwahl von jemandem, der die Seite nicht versteht,
auf der er gerade steht.

Eine Einschränkung, die man kennen sollte: **Adressen sind je Website eindeutig,
über alle Sprachen hinweg.** Es kann also `/kontakt` und `/fr/contact` geben,
aber nicht zweimal denselben Slug in zwei Sprachen. In der Praxis ist das kaum
je störend – eine französische Seite heisst ohnehin anders als die deutsche.

### Sprache der Verwaltung

Mitgeliefert werden fünf Sprachen: **Deutsch, Englisch, Französisch,
Italienisch und Spanisch**, alle vollständig, dazu die **Schweizer Fassung** der
drei Landessprachen (siehe unten). Unter **Mein Konto → Sprache der
Verwaltung** wählt jede Person ihre eigene; ohne Wahl entscheidet der Browser
(`Accept-Language`), und das gilt auch für den Anmeldebildschirm – dort, wo eine
unlesbare Sprache am schlimmsten wäre, weil von da kein Weg zu einer Einstellung
führt.

Die Sprache gehört zum Menschen, nicht zur Website: auf derselben Website
arbeiten eine deutsche Redaktorin und ein englischsprachiger Entwickler, und
beide sehen ihre eigene Verwaltung. Mit den Sprachen der Website selbst (siehe
*Mehrsprachigkeit*) hat das nichts zu tun.

**Wie es gebaut ist.** Der deutsche Satz ist der Schlüssel:

```html
<label>{{t "Titel"}}</label>
<p>{{tf "Seite %d von %d" .Page .TotalPages}}</p>
<p>{{th "Ohne Favicon fragt jeder Browser <code>/favicon.ico</code> an."}}</p>
```

`t` übersetzt, `tf` übersetzt die Vorlage und füllt sie dann (damit eine Sprache
die Teile umstellen kann), `th` ist für einen Satz, der eigenes Markup trägt –
sonst zerfiele er in drei Bruchstücke, mit denen kein Übersetzer etwas anfangen
kann. Alle drei sehen ausschliesslich Zeichenketten aus dem Quelltext, nie
Benutzerdaten.

Zwei Dinge folgen daraus, und beide sind mehr wert als ein ordentliches
Schlüsselschema:

- **Eine fehlende Übersetzung fällt auf Deutsch zurück** – auf einen Satz, den
  jemand lesen kann, statt auf `page.saved`, was ein Fehler auf dem Bildschirm
  wäre.
- **Die Vorlagen bleiben lesbar.** Man sieht, was ein Bildschirm sagt, ohne
  irgendwo nachzuschlagen – der Unterschied zwischen einer Übersetzung, die
  gepflegt wird, und einer, die verrottet.

Meldungen aus Go brauchen nichts: `SetFlashError` übersetzt beim Weglegen,
`NewLayoutData` übersetzt den Titel, und Formularfehler werden beim Zeichnen
übersetzt. Nur ein zusammengesetzter Satz braucht `web.Titlef(r, "Seiten – %s",
name)`, damit das Gerüst nachschlagbar bleibt. Beschriftungen, die beim Start
gebaut werden (Bausteinarten, Feldarten, Startbereitschaft), markiert `i18n.N`
und die Vorlage übersetzt sie mit `{{t .Name}}`.

#### Eine Sprache hinzufügen – ohne Neubau

Sprachen sind Dateien, wie Vorlagen: **was auf der Platte liegt, gilt über dem,
was im Programm steckt.** Der Ordner ist `data/sprachen/`, eine Datei je Sprache,
benannt nach dem Kürzel (`nl.json`, `fr-CH.json`).

In der Verwaltung unter **Sprachen**:

1. **Vorlage herunterladen** – eine JSON-Datei mit allen 917 deutschen Sätzen
   als Schlüssel und leeren Werten.
2. Werte ausfüllen. Was leer bleibt, erscheint auf Deutsch; eine halbe
   Übersetzung ist brauchbar.
3. **Hochladen** – oder die Datei von Hand in den Ordner kopieren und
   *Neu einlesen* drücken. Ein Neustart ist nicht nötig.

Löschen geht genauso: Datei entfernen oder in der Verwaltung auf *Entfernen*.
Wer die Sprache eingestellt hatte, bekommt danach wieder die seines Browsers.

Dieselbe Mechanik korrigiert eine mitgelieferte Sprache: eine `de.json` mit
zehn Zeilen darin ändert zehn Formulierungen, alles andere kommt weiter aus dem
Programm.

Beim Einlesen wird geprüft, und zwar bevor die Datei im Ordner landet:

- Sie muss aus Text-Paaren bestehen — deutscher Satz zu Übersetzung.
- Die **Platzhalter müssen passen**. Ein fehlendes `%s` verschluckt sonst still
  einen Namen; solche Zeilen werden übergangen, der deutsche Satz bleibt.
- **Auszeichnung wird gefiltert.** Erlaubt sind `<code>`, `<strong>` und ein
  paar weitere für den Text selbst; alles andere wird entfernt. Eine
  Sprachdatei ist damit kein Weg, Fremdes in die Verwaltung zu bekommen.

#### Regionalfassungen – `de` und `de-CH`

Eine Sprache mit Region ist eine Sprachdatei wie jede andere, mit **einer Regel
darüber: was sie nicht sagt, sagt ihre Grundsprache.** `de-CH.json` enthält die
knapp vier Dutzend Sätze, die in der Schweiz anders geschrieben werden; die
übrigen 880 kommen aus dem Deutschen. Deshalb bleibt eine Regionalfassung
pflegbar: ein neuer Satz im Programm erscheint in ihr, sobald er einmal
übersetzt ist, und niemand hält zwei fast gleiche Dateien von Hand im Gleichlauf.

Mitgeliefert sind die drei Landessprachen in ihrer Schweizer Fassung:

| Datei | Was darin steht |
|---|---|
| `de-CH.json` | kein ß (`gross`, `heisst`, `Strasse`), «spitze Anführungszeichen» statt „…“, Natel statt Handy |
| `fr-CH.json` | *natel* statt *téléphone*, *laptop* statt *portable* |
| `it-CH.json` | *formulario* statt *modulo*, *natel* statt *cellulare* |

Rätoromanisch ist die vierte Landessprache und hat keine zweite Fassung, von
der es sich unterscheiden müsste. Ein `rm.json` im Ordner genügt, und die
Verwaltung führt es als *Rumantsch* – geschrieben ist es nicht.

**Der Browser findet sie von allein.** Ein Schweizer Browser schickt
`Accept-Language: de-CH,de;q=0.9`; gesucht wird zuerst das ganze Kürzel, dann
die Sprache ohne Region. `de-CH` trifft also die Schweizer Fassung, `de-DE`
trifft Deutsch, und ein Kürzel, für das es nichts gibt, endet bei Deutsch statt
bei nichts.

Die deutsche Rechtschreibung der Schweiz ist eine Regel, kein Urteil: kein ß,
Anführungszeichen nach aussen. Sie wird deshalb gerechnet, nicht getippt:

```bash
go run ./tools/i18n -schweiz     # baut de-CH.json aus dem deutschen Quelltext
```

Was das Werkzeug nach Regel erzeugt, schreibt es neu; Zeilen von Hand – ein Wort
wie *Natel*, das keine Rechtschreibfrage ist – bleiben stehen und bekommen die
Regel zusätzlich. Ein `go test ./internal/i18n/` prüft, dass keine Schweizer
Fassung ein ß enthält und dass jeder ihrer Schlüssel ein Satz ist, den es im
Programm wirklich gibt.

Eine Sprache, die mit dem Programm ausgeliefert werden soll, kommt statt dessen
nach `internal/i18n/locales/` und wird eingebaut:

```bash
cp internal/i18n/locales/en.json internal/i18n/locales/nl.json
go run ./tools/i18n              # meldet, was fehlt und was verwaist ist
go run ./tools/i18n -write       # trägt neue Zeichenketten leer nach
```

Kein Go-Code, keine Schlüsselliste. Das Werkzeug liest die `{{t}}`-Aufrufe der
Vorlagen und die Meldungen im Go-Quelltext; `go test ./internal/i18n/` prüft,
dass keine mitgelieferte Übersetzung leer ist und dass die Platzhalter zur
deutschen Fassung passen.

### KI-Zugang

Ein Betreiber kann seinen eigenen KI-Assistenten an das laufende CMS anschliessen
und damit Inhalte schreiben. Die Richtung ist wichtig: **dieser Server ruft keine
KI auf.** Hier liegt kein Zugang zu einem Anbieter, hier wird nichts nachgeladen,
und die Regel, dass zur Laufzeit nichts von Dritten geholt wird, bleibt
unangetastet. Der Assistent läuft, wo der Betreiber ihn laufen lässt, und meldet
sich von aussen an.

Gesprochen wird MCP — das Protokoll, das Claude, ChatGPT und die Editoren
benutzen: JSON-RPC 2.0 unter `POST /ai`, angemeldet mit
`Authorization: Bearer <Schlüssel>`. Die Adresse liegt ausserhalb des
CSRF-Schutzes und ausserhalb der Sitzung, weil sich hier kein Browser anmeldet;
ein Formular kann diesen Kopf nicht setzen, also gibt es die Lücke nicht, gegen
die CSRF sonst schützt.

Schlüssel werden in der Verwaltung unter *KI-Zugang* ausgestellt. Ein Schlüssel
ist genau einmal zu sehen — gespeichert ist nur sein SHA-256-Abdruck, dieselbe
Disziplin wie beim Einladungslink. Er gilt wahlweise für alle Websites oder für
eine, wahlweise nur lesend oder auch schreibend, wahlweise unbegrenzt oder bis zu
einem Datum. Wann er zuletzt benutzt wurde, steht in der Liste; das ist die Spur,
an der auffällt, wenn einer benutzt wird, von dem man das nicht erwartet hätte.

Was ein Assistent tun kann, tut er durch dieselben Stores wie die Verwaltung:
gleiche Adressregeln, gleiche Fassungen, gleiche Prüfungen. Zwei Regeln stehen
darüber:

- **Neues ist immer ein Entwurf.** Veröffentlicht wird nur mit einem eigenen,
  ausdrücklichen Aufruf. Ein Assistent, der aus Versehen veröffentlicht, stellt
  etwas Halbfertiges ins Netz, und gemerkt wird das erst, wenn jemand es gelesen
  hat.
- **Ändern ist nicht Veröffentlichen.** Eine veröffentlichte Seite bleibt
  veröffentlicht, ein Entwurf bleibt Entwurf, und der alte Stand bleibt als
  Fassung erhalten und lässt sich zurückholen.

| Werkzeug | Was es tut | Schreibt |
|---|---|---|
| `websites_auflisten` | die Websites dieser Installation | nein |
| `seiten_auflisten` | Seiten einer Website, ohne Text | nein |
| `seite_lesen` | eine Seite samt Markdown | nein |
| `seiten_durchsuchen` | Volltextsuche, Entwürfe eingeschlossen | nein |
| `medien_auflisten` | Bilder und Dateien mit fertigem Markdown-Verweis | nein |
| `felder_auflisten` | die eigenen Felder dieser Website samt Gruppen | nein |
| `seite_anlegen` | neue Seite, immer als Entwurf; nimmt eigene Felder entgegen | ja |
| `seite_aendern` | Titel, Text oder einzelne Felder; Zustand bleibt | ja |
| `seite_veroeffentlichen` | öffentlich schalten oder zurücknehmen | ja |

Ein nur lesender Schlüssel bekommt die drei schreibenden Werkzeuge gar nicht erst
zu sehen — und wird abgewiesen, falls er sie doch aufruft. Eine Seite aus
Bausteinen lässt sich nicht mit Markdown überschreiben; der Assistent bekommt
gesagt, warum, und dass das in der Verwaltung zu machen ist.

## Versionen

Dieses Repository beginnt bei **`v1.4`**. Das ist sein erster Tag, und der
einzige Commit darunter ist der Anfang der öffentlichen Aufzeichnung, nicht der
Anfang des Projekts. Entwickelt wurde es vorher in einem privaten Repository;
was hier liegt, ist der Stand, ab dem es öffentlich weitergeht. Wer sich
wundert, dass ein fertig aussehendes Projekt aus einem einzigen Commit besteht:
das ist der Grund, und mehr steckt nicht dahinter.

Dass überhaupt ein Tag da sein muss, hat einen praktischen Grund. Die Fassung
wird beim Bauen fest ins Binär geschrieben:

```
-ldflags "-X main.Version=$(git describe --tags --always) ..."
```

`git describe` findet nur Tags, die von `HEAD` aus erreichbar sind. Liegt keiner
dort, greift das `--always`, und statt einer Fassung steht ein blosser
Commit-Hash im Binär. Mit `v1.4` steht dort eine.

Was ein laufendes Binär von sich selbst hält, fragen Sie es direkt:

```bash
./holzcloud version        # ebenso -version und --version
```

Die Meilensteine unter `.planning/` teilen dieselbe Nummerierung: auf den Tag
`v1.4` folgt der Meilenstein `v1.5`.

## Build

```bash
# Local development
go build ./cmd/holzcloud

# Production (linux/amd64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o holzcloud ./cmd/holzcloud
```

The binary is fully self-contained (~13 MB). No external files needed at runtime.

## Deployment (linux/amd64)

Detailed instructions in [`deploy/DEPLOY.md`](deploy/DEPLOY.md). Summary:

```bash
# 1. Copy binary to the server
scp holzcloud user@your-server:/opt/holzcloud/

# 2. Install systemd service
sudo cp deploy/holzcloud.service /etc/systemd/system/
sudo systemctl enable --now holzcloud

# 3. Set up Caddy for HTTPS (optional)
sudo cp deploy/Caddyfile.example /etc/caddy/Caddyfile
# Edit domains, then: sudo systemctl reload caddy

# 4. Set up backups
sudo cp deploy/backup.sh /opt/holzcloud/
crontab -e
# 0 3 * * * /opt/holzcloud/backup.sh
```

The `deploy/` directory contains:

| File | Purpose |
|------|---------|
| `holzcloud.service` | systemd unit with security hardening (`ProtectSystem`, `NoNewPrivileges`, etc.) |
| `Caddyfile.example` | Caddy reverse proxy with automatic HTTPS |
| `backup.sh` | WAL-safe SQLite backup + media/template rsync |
| `DEPLOY.md` | Step-by-step setup guide for a Debian/Ubuntu amd64 server |

## Usage

### First Run

1. Start the binary: `./holzcloud`
2. Open `http://localhost:8080/admin`
3. You'll be redirected to the setup form — create your admin account
4. You're in the dashboard

### Managing Websites

1. Go to **Websites** in the sidebar
2. Create a website (name + description)
3. Add one or more domains to the website
4. Point DNS for those domains to your server
5. Requests matching the domain's `Host` header are routed to this website

Domains are normalised on save: a pasted URL, an explicit port, a trailing dot
or uppercase letters are all reduced to the bare hostname, and an
internationalised name is converted to punycode (`möbel.de` is stored as
`xn--mbel-5qa.de`, which is what a browser actually sends). The admin UI shows
the readable form. Unchecking **Active** takes the site offline immediately.

### Creating Pages

1. Navigate to a website, then **Pages**
2. Create a page with title, slug, and Markdown content
3. The Markdown is rendered to HTML and sanitized automatically
4. Set status to **Published** to make it visible on the public site
5. Draft pages return 404 publicly

### Menus

1. Navigate to a website, then **Menus**
2. Create a menu with a location key (e.g., `main`, `footer`)
3. Add items: link to published pages, external URLs, or plain text
4. Nest items up to 3 levels deep
5. Reorder with up/down buttons
6. Menus render as nested `<ul>` lists in the public template

### Templates

1. Go to **Templates** in the sidebar
2. Upload a `.zip` archive containing at least `layout.html` and `page.html`
3. Optionally include `home.html`, `404.html`, and an `assets/` directory
4. Activate the template for a specific website
5. The public site immediately uses the new template (no restart needed)

`layout.html` is the document and is what gets rendered. Each view file —
`home.html`, `page.html`, `404.html` — supplies the `{{define "content"}}` block
that `layout.html` pulls in, so a view is parsed together with the layout into
its own template set. A built-in default template is always available as
fallback, and any file the archive omits falls back to it.

### Media

1. Navigate to a website, then **Medien**
2. Upload images (JPEG, PNG, GIF, WebP, SVG) or PDFs
3. Files are stored in `data/media/{website_id}/` with unique filenames
4. Insert them from the editor (*Bild aus der Mediathek einfügen*) or write the
   Markdown yourself: `![Beschreibung](/media/{website_id}/{filename})`
5. Served with immutable cache headers
6. The library is paginated at 24 files per page and can be filtered by name,
   description, kind and "unused"

**Kamera-Angaben werden beim Hochladen entfernt.** Ein Handyfoto trägt die
GPS-Koordinaten des Aufnahmeorts im EXIF-Block; die Datei wird öffentlich und
unverändert ausgeliefert, ein zuhause fotografiertes Produktfoto würde also die
Privatadresse veröffentlichen. Entfernt werden EXIF, XMP, IPTC und Kommentare —
per Byte-Chirurgie, nicht durch Neukodieren: das Bild bleibt bitgenau dasselbe.
Farbprofil (ICC), Pixeldichte (JFIF) und Farbtransformation (Adobe) bleiben
erhalten, weil ihr Verlust das Bild sichtbar verändern würde.

Eine Bildbeschreibung (Alt-Text) lässt sich zu jeder Datei speichern und wird
beim Einfügen automatisch übernommen. Die Mediathek zeigt, wie viele Bilder noch
keine haben. Wird eine Datei gelöscht, die noch auf einer Seite steht, nennt die
Fehlermeldung die betroffenen Seiten, statt die Bilder dort stillschweigend
kaputtgehen zu lassen.

### Suche, Textbausteine und Zeitsteuerung

Die Volltextsuche läuft über SQLite FTS5, das im reinen Go-Treiber bereits
einkompiliert ist — kein zusätzlicher Dienst, kein CGO. Diakritika werden
gefaltet, `mobel` findet also „Möbel“. Öffentlich unter `/suche`, im Admin über
das Suchfeld der Seitenliste.

Textbausteine sind wiederverwendbare Blöcke (Adresse, Öffnungszeiten). Ein
`[[snippet:kennung]]` im Seitentext wird beim Ausliefern ersetzt, nicht beim
Speichern — geändert wird also genau an einer Stelle. Ein Theme kann einen
Baustein zusätzlich über `{{index .Site.Snippets "kennung"}}` platzieren.

Eine Seite kann ein „sichtbar ab“ und ein „sichtbar bis“ haben. Beides ist eine
Bedingung beim Lesen, kein Hintergrundjob: es gibt keinen Zeitpunkt, der
verpasst werden kann, und ein Gerät, das über den Termin aus war, kommt bereits
richtig hoch. Der Feed liegt unter `/feed.xml`.

Wird eine Seite umbenannt, entsteht automatisch eine Weiterleitung von der alten
Adresse. Unter *Weiterleitungen* lassen sich zusätzlich die Adressen einer alten
Website eintragen; die Trefferzähler zeigen, welche davon noch Verkehr bringen.

### SEO

Every website automatically serves, under each of its domains:

- `/sitemap.xml` — the homepage plus every published page, with a `lastmod`
  date. Drafts are excluded by the query, so an unpublished page is not
  disclosed here either.
- `/robots.txt` — allows the public site, disallows `/admin/`, and points at
  the sitemap.

Absolute URLs use the request's `Host` and the scheme implied by
`HOLZCLOUD_SECURE`. A forwarded scheme header is deliberately ignored: these
URLs go to search engines, so a client must not be able to influence them.

### Zugänge, Weiterleitungen und kaputte Links

Ein Admin muss kein Passwort mehr ausdenken und durchsagen: unter *Benutzer* gibt
es **Reset-Link** und **Einladungslink**. Der Link wird genau einmal angezeigt,
läuft ab (72 h für Einladungen, 1 h für Resets) und funktioniert einmal.
Gespeichert ist nur seine Prüfsumme — dieselbe Disziplin wie beim CSRF-Schlüssel.

Bewusst ohne E-Mail-Versand: SMTP wäre der erste Code hier, der zur Laufzeit nach
außen wählt. Gib den Link über den Weg weiter, den ihr ohnehin benutzt; anders als
ein durchgesagtes Passwort bleibt er kein dauerhaft geteiltes Geheimnis.

*Ausgesperrt?* Wenn niemand mehr hineinkommt, hilft weiterhin nur die CLI auf dem
Gerät (`holzcloud user passwd`) — ohne Egress ist das unvermeidbar.

Unter **Nicht gefunden** stehen die Adressen, die Besucher angefragt haben und die
es nicht gibt: die kaputten eingehenden Links, über die die Weiterleitungsliste
nichts sagen kann, weil dort nur steht, wofür schon jemand eine Weiterleitung
angelegt hat. Jede Zeile lässt sich mit einer Auswahl zur Weiterleitung machen.
Offensichtliches Scanner-Rauschen (`.php`, `/wp-`, `/.env`) wird nicht mitgezählt,
und Treffer werden im Speicher gesammelt und einmal pro Minute geschrieben — ein
Schreibvorgang pro 404 würde durch den Single-Writer-Pool laufen und echte
Seitenaufrufe blockieren. Auf demselben Screen steht, welche Links im eigenen Text
ins Leere führen, mit der Seite, auf der sie stehen.

### Users

1. Go to **Users** in the sidebar (admin only)
2. Create users with **admin** or **editor** role
3. Editors can manage content (pages, menus, media) but not users or site settings
4. Changing your own password requires entering the current password
5. Admins can reset any user's password without the current password

## Architecture

```
cmd/holzcloud/main.go           Entry point, route wiring, middleware
internal/
  config/                       Environment-based configuration
  db/                           SQLite dual-pool (write pool MaxOpenConns=1), WAL mode
  db/migrations/                Goose SQL migrations (embedded, run at startup)
  auth/                         Argon2id hashing, SCS sessions, CSRF, middleware
  domain/                       Website/domain models, store, host resolver middleware
  page/                         Page models, store, goldmark+bluemonday pipeline, slug generation
  admin/                        Admin handlers: dashboard, websites, pages, menus, templates, media, users
  public/                       Public site handlers with ETag/Last-Modified caching
  template/                     Template loader (disk-first, embedded fallback)
  tmplmgr/                      Template archive upload (zip-slip safe), activation per website
  menu/                         Hierarchical menu store, nested HTML renderer
  media/                        Media upload with magic-byte MIME validation, disk storage
  web/                          Flash messages, layout data, template rendering helpers
  bundle/                       One website as a zip: export, import, manifest format
sites/                          Fertige Websites als Quelltext (Manifest + Bilder), siehe sites/README.md
tools/mkbundle/                 Packt ein sites/-Verzeichnis zum importierbaren Zip und prüft es
```

### Database

SQLite with WAL mode and dual connection pools:

- **Write pool** — `MaxOpenConns=1` to prevent `SQLITE_BUSY`
- **Read pool** — higher concurrency for queries
- **Pragmas** applied per connection: `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`
- **Migrations** run automatically at startup via goose with embedded SQL files

### Schema

| Migration | Tables |
|-----------|--------|
| `00001_initial.sql` | `users` |
| `00002_sessions.sql` | `sessions` |
| `00003_websites.sql` | `websites`, `website_domains` |
| `00004_pages.sql` | `pages` |
| `00005_templates_menus_media.sql` | `templates`, `website_templates`, `menus`, `menu_items`, `media` |
| `00006_user_name.sql` | Adds `name` column to `users` |

### Security

- **Argon2id** password hashing (tunable to the server's performance), minimum 8 characters
  everywhere a password is set
- **CSRF** protection on all state-changing requests via gorilla/csrf
- **htmx integration** — CSRF token in `hx-headers` on `<body>` for AJAX requests
- **Session rotation** on login to prevent fixation
- **Session revalidation** — every admin request re-checks the user against the
  database, so deleting a user or changing a role takes effect immediately
- **Session revocation** — changing a password ends that user's other sessions
- **Login throttling** — 10 failures per client address and 100 per account
  within 15 minutes; the account limit is deliberately loose so it cannot be
  used to lock someone out
- **Nothing loads at runtime** — a `default-src 'self'` CSP on every response
  (stricter still on `/admin`), and template archives are rejected at upload if
  they reference an external stylesheet, script, font or image. See below.
- **Security headers** — `nosniff`, `Referrer-Policy`, `X-Frame-Options`,
  `Cross-Origin-Opener-Policy`
- **bluemonday** HTML sanitization on all Markdown output
- **Zip-slip prevention** and an enforced uncompressed-size budget on template upload
- **Magic-byte MIME validation** on media upload; SVG must really be SVG, and
  SVG/PDF are served with a sandboxing CSP
- **Path validation** on template assets — `http.ServeMux` cleans the *escaped*
  URL, so `%2e%2e` reaches handlers as `..` and must be rejected explicitly
- **Draft isolation** — draft pages never leak to public queries

### No Runtime Dependencies on Third Parties

Nothing is fetched from another server while the application runs — no CDN
scripts, no external stylesheets, no web fonts, no remote images, no analytics.
Every subresource comes from this server's own origin, which is what makes the
`default-src 'self'` CSP possible and keeps a Holzcloud instance fully
self-contained and offline-capable.

Only two things may be downloaded **at build time**:

1. **Go modules** — normal `go get` / `go mod tidy`
2. **Fonts** — fetched once, committed to the repository, embedded via
   `embed.FS` and served from `/assets/`. A font is never referenced by URL.

Uploaded templates must follow the same rule: bundle CSS, images and fonts into
the archive and reference them with relative or root-relative paths
(`/t/style.css`, `/t/fonts/inter.woff2`). An archive that references another
origin is refused with the offending URL named in the error. Ordinary outbound
hyperlinks (`<a href="https://…">`) are content, not subresources, and remain
allowed — as do `rel="canonical"` and `rel="alternate"` links.

#### A Holzcloud site needs no cookie banner

This follows from the rule above and is worth stating plainly. The public side
sets **no cookies at all**: the session manager is only ever touched by admin
handlers, and the CSRF cookie is scoped to `Path("/admin")`. It also loads
nothing from a third party, so there is no embedded service that could set one
on your behalf and nothing to obtain consent for under § 25 TDDDG.

Consent is about storing or reading information on the visitor's device and
about passing data to third parties. A Holzcloud site does neither, so the
banner has nothing to ask about. You still need an imprint (§ 5 DDG) and a
privacy notice (Art. 13 DSGVO) — a new website is created with both as drafts,
linked from the footer menu, ready to fill in.

This stops being true the moment a feature is added that stores something in
the browser or talks to another server. Any such feature has to say so here.

**The contact form is the one feature that collects personal data**, so it gets
its entry. Placing `[[formular]]` on a page adds a form whose submissions are
stored in the site's own database and read in the admin under *Nachrichten*.

`[[formular:Rohwolle]]` places the same form with *Rohwolle* already in the
subject line. That is for a page that sells or offers one thing: without it
every enquiry from every product page arrives with an empty subject, and
whoever reads them has to open each one to find out which product it is about.
The visitor can still change the field, and a subject they typed themselves is
never overwritten.

What that does and does not change:

- It still sets **no cookie**. The spam protection is a honeypot field and a
  signed timestamp in the form itself, so nothing is stored on the visitor's
  device and the banner still has nothing to ask about.
- It still talks to **no third party**. Nothing is mailed; there is no SMTP
  server, no captcha service, no delivery provider.
- It **does** store what the visitor typed — name, address, subject, message —
  under Art. 6(1)(b)/(f) DSGVO. The sender's IP address and user agent are
  deliberately *not* stored.
- Your privacy notice therefore has to mention the form: what is collected, why,
  and for how long you keep it. Nothing else in Holzcloud needs an entry there.

Delete answered enquiries when you no longer need them; nothing expires them
automatically, because how long an enquiry stays relevant is a decision only the
operator can make.

**A password-protected page sets one cookie**, and only that page does. It
remembers that the visitor entered the password, is scoped to that page's path,
lasts twelve hours and contains a signed token rather than anything about the
person. It is strictly necessary for a service the visitor explicitly asked for
— seeing the page they just unlocked — and therefore consent-exempt under
§ 25(2) TDDDG. A site with no protected page still sets no cookie at all.

### Export and import

`Einstellungen → Website herunterladen` writes one website as a zip: a JSON
manifest plus the media files. It exists so a site is not a hostage to the
machine it runs on — a database file is a backup of the whole installation,
this is one website, in a form somebody can read, diff and repair in a text
editor when everything else has gone wrong.

What travels: settings **including the site's further languages**, design
tokens, pages and posts as **Markdown**, pages built from **blocks** with the
blocks themselves, the website's own **fields**, **content kinds** and **block
kinds**, menus (nested, pointing at pages by slug, one per language), snippets,
labels and every media file with its description and a SHA-256.

Anything that names a picture or a page names it the way the other machine can
read: a **file name**, never an id — in the preview image, in a picture field,
inside a block, and inside a block's own fields. An id means nothing on the
machine a bundle lands on, and one that was silently kept would point at
somebody else's photo.

What deliberately does not:

- **Domains.** A bundle is meant to land somewhere else; carrying the host names
  would either collide with the site already serving them or quietly claim a
  domain the new machine does not own. An imported site therefore has none, and
  the report says so.
- **Passwords.** Neither account passwords nor page passwords. A hash in a file
  that gets emailed around is a hash somebody can attack offline. A page that
  was protected arrives unprotected — and the report names it, rather than
  leaving a price list publicly readable with nobody told.
- **Rendered HTML.** Only the Markdown and the blocks travel; the HTML is
  derived, it roughly doubles the archive, and a bundle written by an older
  renderer would otherwise import markup that no longer matches what this
  version produces. A page of blocks is therefore set again on the way in.

Import always creates a **new** website rather than merging. Merging would need
an answer for every collision — same address, different text — and the honest
answer at this size is a second site the operator can compare and then delete.
Everything read out of the archive goes through the same validators as the
forms: a bundle is a file anyone can edit, so it is exactly as untrusted as a
text field.

### Per-site design

Between "pick one of four themes" — too coarse, because everyone wants their own
colour — and "upload your own template" — too much, because it means maintaining
a copy of a theme through every future change — sits a handful of values that
every shipped theme picks up: text colour, background, accent, typeface, text
width, corner rounding.

Each is validated to a shape the CSS can interpret only one way: a hex colour, a
typeface from a fixed list, or an integer in range. A value that does not fit is
dropped and the theme answers for it, rather than the whole form being refused
over one mistyped colour. The typeface list is fixed for a second reason: a name
typed by hand is either not installed — silently falling back to something else
— or a URL, which would load a font from another server.

### Structured data

Every page carries a schema.org graph as JSON-LD: the organisation behind the
site, the page or article itself, and a breadcrumb trail. What that buys is the
difference between a search result with an address, opening hours and a
telephone number and one with a blue link — which for a workshop that lives on
being findable is not a small difference.

The fields are the ones that belong in the imprint anyway; the settings screen
only asks them in a form a search engine understands rather than guesses at.
Leaving the business type empty emits no organisation block at all, because a
personal blog should not claim to be a shop.

It is built in Go rather than left to the theme. A theme author who gets one
field wrong produces a result that looks perfectly fine and is silently ignored,
and there is nothing on the site itself to show it.

### Protected pages and preview links

Two small answers to two questions every site eventually asks.

A page can be put **behind a password** — the trade price list a joiner sends to
dealers and does not want in a search index. It is not a substitute for
accounts: everyone who gets the word gets in, and there is no record of who did.
What it does guarantee is that the page never appears in a listing, an archive,
the search, the feed or the sitemap, because the title and the excerpt are
exactly what the password was meant to hold back.

A **preview link** shows an unpublished page to someone without an account. The
alternative people reach for otherwise is publishing a draft "just for a minute"
so a customer can look at it, which is how a half-finished price list ends up in
a search index for a year. The link is an HMAC over the page id and an expiry, so
a signature cannot be moved to another page or stretched to a later date, and the
page it opens carries a visible banner saying it is a preview.

### Two-factor authentication

Administrators **must** use a second factor; editors may. The reasoning is that
an administrator can change every password on the installation, upload a
template and reach every site, so a guessed password there costs everything.

It is ordinary TOTP (RFC 6238) — any authenticator app works, and the
implementation is checked against the RFC's published test vectors so it agrees
with the apps rather than merely with itself. The QR code is generated and
drawn as inline SVG; nothing is fetched at page load, and the shared secret is
also printed in blocks of four for anyone whose camera will not focus.

Making it compulsory would turn the phone into a single point of failure for the
whole installation, so there are two ways back:

1. **Recovery codes.** Ten single-use codes, shown once at setup and never
   again — only their hashes are stored. `Mein Konto` counts down how many are
   left and issues a fresh list on request.
2. **The command line.** `holzcloud user 2fa disable -email …` removes the
   second factor from an account. It needs shell access to the machine, which is
   exactly the thing an attacker with the password but not the phone does not
   have. `holzcloud user 2fa status` shows who has one and how many recovery
   codes they have left.

A code is refused once it has been used, even inside the thirty seconds it stays
arithmetically valid: that window is long enough for someone who read the digits
over a shoulder to type them in afterwards.

### Frontend

- **htmx 2.0** — the only JavaScript (self-hosted, no CDN)
- **Plain CSS** — OKLCH color tokens, `@layer` cascade, container queries
- **View Transitions** — progressive enhancement for admin navigation
- **No-JS fallback** — every admin action works with JavaScript disabled

### Die Verwaltung

Oben eine dunkle Leiste, links die Abschnitte, rechts der Inhalt auf grauem
Grund in weissen Karten. Die Leiste trägt, **wo** man ist, die Seitenleiste,
**was** man dort tun kann — das ist die Aufteilung, an der man sich
zurechtfindet, ohne sie erklärt zu bekommen.

- **Website-Wechsler** in der Kopfleiste. Wer mehrere Websites betreut,
  wechselt an einer Stelle statt über Umwege durch die Website-Liste.
- **Die Abschnitte bleiben stehen.** Die Seitenleiste zeigt Seiten, Medien,
  Menüs und den Rest auch dort, wo die Adresse keine Website nennt: gemerkt
  wird, woran zuletzt gearbeitet wurde. Vorher verschwand das halbe Menü auf
  dem Weg über die Benutzerliste.
- **Plugin-Bildschirme stehen im Menü**, zwischen dem Übrigen. Wer Anfragen
  aus dem Kontaktformular liest, sucht sie beim Inhalt und nicht unter
  „Plugins“ — die vorher der einzige Weg dorthin waren, und nur für
  Administratoren zu öffnen.
- **Eine sichtbare Aktion pro Zeile**, der Rest hinter drei Punkten. Fünf
  gleich laute Knöpfe mal zehn Zeilen sind fünfzig, und dann sieht man keinen
  mehr.
- **Hell und dunkel**, nach der Einstellung des Betriebssystems. Getauscht
  werden nur die Farbwerte; es gibt keine zweite Regelmenge, die auseinander
  laufen könnte.

Aufklappmenüs sind `<details>`/`<summary>` — kein Skript, und die Tastatur
bedient sie von sich aus. Was sie nicht können: sich schliessen, wenn man
daneben klickt. Das ist der Preis dafür, dass hier keine Zeile JavaScript für
Menüs steht.

Ein Vergleich der Funktionen und Feldtypen mit Statamic — samt Vorschlag, was
als Nächstes fehlt — steht in [`docs/vergleich-statamic.md`](docs/vergleich-statamic.md).

## Data Directory

All runtime data lives in one directory (default `data/`):

```
data/
  holzcloud.sqlite          SQLite database
  holzcloud.sqlite-wal      WAL file
  csrf.key                  CSRF encryption key (32 bytes, auto-generated)
  media/{website_id}/       Uploaded media files
  templates/{slug}/         Uploaded template archives (extracted)
```

Back up this entire directory to preserve everything. See `deploy/backup.sh` for an automated approach.

## Tech Stack

| Component | Choice | Reason |
|-----------|--------|--------|
| Language | Go 1.22+ | Single binary, stdlib-first |
| Database | SQLite via `modernc.org/sqlite` | Pure Go, no CGO — builds as a static binary with no C toolchain |
| Sessions | `alexedwards/scs/v2` | Server-side sessions in SQLite |
| CSRF | `gorilla/csrf` | Middleware with htmx header integration |
| Migrations | `pressly/goose/v3` | Embedded SQL files, auto-run at startup |
| Markdown | `yuin/goldmark` | CommonMark compliant |
| Sanitizer | `microcosm-cc/bluemonday` | Prevents XSS from user content |
| Frontend | htmx 2.0 + plain CSS | No build tools, no npm, no bundlers |

## License

[GNU AGPL-3.0](LICENSE) — the full text is in `LICENSE`.
