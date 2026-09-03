# Holzcloud neben Statamic

Statamic ist ein ausgewachsenes CMS mit über zehn Jahren Entwicklung, einem
Team dahinter und einem Markt für Erweiterungen. Holzcloud ist eine einzelne
Go-Datei auf einem kleinen Server. Der Vergleich ist trotzdem nützlich — nicht
um aufzuholen, sondern um zu sehen, welche Lücken **weh tun** und welche
bewusste Entscheidungen sind.

Stand: August 2026, Statamic v6.

---

## Die eine strukturelle Lücke — inzwischen geschlossen

Als dieses Papier entstand, stand hier der Unterschied in der Bauart:

> **Statamic lässt den Betreiber sein Inhaltsmodell selbst bestimmen.
> Holzcloud hat ein festes.**

In Statamic legt man *Blueprints* an: „Ein Produkt hat einen Namen, einen
Preis, ein Bild, eine Verfügbarkeit und drei Varianten." Dafür stehen über 40
Feldtypen bereit. In Holzcloud hatte eine Seite Titel, Adresse, Inhalt,
Kurzfassung, Vorschaubild, Schlagwörter und einen Zugang — und wer ein Feld
„Preis" brauchte, brauchte eine neue Go-Version.

Das war der Grund, warum wir den Hofladen zuerst als Markdown-Text gebaut
haben und nicht als Produktliste.

Diese Lücke ist zu: **eigene Felder** je Website (Punkt 1), **wiederholbare
Gruppen** (Punkt 2), **eigene Inhaltsarten** (Punkt 6) und **Abschnitte und
Bedingungen** im Formular (Punkt 7). Wer Produkte führt, legt die Art
*Produkt* an, hängt ihre Felder daran, teilt sie mit Überschriften ein, lässt
den Sonderpreis erst erscheinen, wenn das Angebot angekreuzt ist — und bekommt
eine Verwaltung, die von Produkten spricht. Mit Punkt 8 gilt dasselbe eine
Ebene tiefer: auch die Bausteine des Editors sind keine feste Liste mehr.

---

## Feldtypen: 47 gegen 15

Statamic hat 47 Feldtypen. Wir haben inzwischen elf, die eine Website selbst
vergeben kann — neun Eingabearten, die wiederholbare Gruppe und die Überschrift
(siehe README, *Eigene Felder*) — plus die fest eingebauten Eingabearten der
übrigen Bildschirme:

| Wo | Eingabearten |
|---|---|
| Seiteneditor | Text, Adresse, Markdown, Langtext, Bildauswahl, Ja/Nein, Auswahl, Datum+Zeit, Schlagwörter, Passwort |
| Bausteineditor | 9 Bausteinarten mit je eigenen Feldern (Markdown, Bild, Alt, Bildunterschrift, Breite/Variante, Titel, Text, Quelle, Linktext, Linkziel, wiederholbare Unterelemente) |
| Formular-Plugin | Kurze Antwort, Lange Antwort, E-Mail, Telefon, Zahl, Datum, Auswahl, Ankreuzfeld |
| Website-Einstellungen | Farbe, Schriftauswahl, Zahl, Text, E-Mail, Auswahl, Ja/Nein |

Der Unterschied ist nicht die Zahl. Es ist, dass unsere Eingabearten **nicht
kombinierbar** sind: das Formular-Plugin kann acht Arten, aber nur für
Formulare; der Bausteineditor kann Bilder, aber nur in Bausteinen.

### Was uns an einzelnen Feldtypen fehlt

Nach Nützlichkeit für dieses Projekt, nicht nach Statamics Reihenfolge:

| Statamic | Haben wir? | Anmerkung |
|---|---|---|
| `replicator`, `grid`, `group`, `array`, `list`, `table` | **ja, eine Art** | die Gruppe: wiederholbare Feldzeilen, eine Ebene tief |
| `entries` | **ja** | die Art „Verweis": eine Seite dieser Website, ausgewählt statt getippt |
| `terms`, `taxonomies`, `collections` | **teilweise** | Schlagwörter gibt es; als *Feld* wählbar sind sie nicht |
| `link` | **ja** | die Art „Link": interne Adresse, fremde Adresse oder Mailadresse |
| `code`, `html`, `yaml` | **nein** | für Redakteure selten, für Entwickler manchmal |
| `integer`, `float`, `range`, `time` | **teilweise** | Zahlen nur in Einstellungen, Zeit nur in der Zeitsteuerung |
| `radio`, `checkboxes`, `button_group` | **nein** | Auswahl gibt es, aber nur als Klappliste |
| `video` | **als Baustein** | eigenes MP4 in einem `<video>`; `icon` und `dictionary` fehlen |
| `assets` | **ja** | Medienauswahl mit Vorschau |
| `markdown`, `textarea`, `text`, `slug`, `toggle`, `select`, `date`, `color` | **ja** | |
| `bard` (Blockeditor) | **ja, eigener** | 9 eingebaute Arten plus die, die eine Website selbst anlegt |
| `users`, `user_groups`, `user_roles`, `sites`, `structures`, `navs`, `form`, `template` | **nein** | brauchen erst ein Feldsystem, um sinnvoll zu sein |
| `section`, `revealer` | **ja** | die Art „Abschnitt": eine Überschrift zwischen den Feldern, und die Bedingung „nur zeigen, wenn …" |
| `hidden`, `spacer`, `width` | **nein** | Kosmetik im Formularaufbau |

---

## Funktionen: nebeneinander

### Was wir haben

| Statamic | Holzcloud |
|---|---|
| Revisions & Content History | ✅ Fassungen mit Zurückholen |
| Asset Manager | ✅ Medien je Website, mit Alt-Text-Pflichtprüfung |
| Focal Point Editor | ✅ Bildausschnitt **und** Fokuspunkt |
| Dynamic Image Manipulation | ✅ Zuschnitt, Drehung, responsive Grössen |
| Block-Based Editor (Bard) | ✅ Bausteineditor, 9 eingebaute Arten plus eigene je Website |
| Live Preview | ✅ Vorschau im Editor, dazu Vollvorschau im Theme |
| Filter & Search | ✅ Suche, Status-, Art- und Sortierfilter |
| Collections | ✅ eigene Inhaltsarten je Website, mit Übersichtsseite und eigenen Feldern |
| Drag & Drop Nav Builder | ⚠️ Menüs ja, Verschieben mit Pfeiltasten (kein JS) |
| Forms | ✅ als Plugin, mit eigenem Formularbaukasten |
| Globals | ⚠️ Textbausteine — Text ja, andere Typen nein |
| Content Protection | ✅ Seite hinter Passwort, ganze Website offline |
| Two-Factor Authentication | ✅ TOTP, für Administratoren **Pflicht** |
| User Management | ✅ zwei Rollen plus Rechte je Person (Websites, Veröffentlichen); keine Gruppen |
| Dark Mode | ✅ seit heute |
| Importer | ✅ eigenes Format und WordPress (WXR); CSV nein |
| Content API | ✅ als MCP unter `/ai` — lesen und schreiben |
| Command Line Tools | ✅ Benutzer, Sicherung, Migration, Integrität |
| Helpful Utilities | ✅ E-Mail-Test, Integritätsprüfung, Papierkorb |
| Simple Commerce (Addon) | ✅ als Plugin: Produkte aus eigenen Feldern, Bestellformular, Bestellungen |
| Static Site Generator | ❌ |
| Multi-Site | ✅ mehrere Websites mit mehreren Domains, dazu Übersetzungen einer Website (Hauptsprache ohne Präfix) |

### Was Statamic hat und wir nicht

| | Was es ist | Für uns wert? |
|---|---|---|
| **Blueprints** | eigenes Inhaltsmodell mit Feldtypen | **gebaut** — eigene Felder, Gruppen, Inhaltsarten, Abschnitte, Bedingungen und Bausteinarten |
| **Mehrsprachigkeit** | dieselbe Seite in mehreren Sprachen | **gebaut** — Hauptsprache ohne Präfix, weitere unter `/fr/…` |
| **Übersetzte Verwaltung** | ~30 Sprachen | **gebaut** — fünf Sprachen mitgeliefert plus die Schweizer Fassung der drei Landessprachen, je Person; weitere als Datei im laufenden Betrieb |
| **Rollen und Rechte** | Rollen, Gruppen, einzelne Berechtigungen | **gebaut** — zwei Rollen plus zwei Rechte je Person: welche Websites, und veröffentlichen ja/nein. Keine Gruppen |
| **Eigene Spalten in Listen** | sichtbare Spalten wählen | **gebaut** — je Person, sechs Spalten zum Ankreuzen |
| **Gespeicherte Filter** | Ansichten speichern und wiederverwenden | **gebaut** — je Person und Website, als Chips über der Liste |
| **Elevated Sessions** | Passwort erneut abfragen vor heiklen Aktionen | **gebaut** — vor vier Knöpfen, 15 Minuten gültig |
| **Importer (WordPress, CSV)** | Umzug von anderswo | **gebaut** — WXR mit Seiten, Beiträgen und Schlagwörtern; Bilder bleiben bewusst zurück. Das eigene Format trägt alles: Sprachen, Felder, Inhalts- und Bausteinarten, Bausteine, Medien |
| **Statischer Export** | Website als reine HTML-Dateien | mittel — auf einem kleinen Knoten durchaus reizvoll |
| **Whitelabel** | eigenes Logo in der Verwaltung | **gebaut** — Name, Zeichen und Logo unter *Marke* |
| **Vollbildmodus** | ablenkungsfrei schreiben | **gebaut** — `?vollbild=1`, reines CSS |
| **Befehlspalette (⌘K)** | Tastaturzugriff auf alles | **passt nicht** — braucht JavaScript |
| **Passkeys** | Anmeldung ohne Passwort | **passt nicht** — braucht JavaScript |
| **Video-Embeds** | YouTube/Vimeo per Adresse einbetten | **passt nicht** — genau das, was die Regel „nichts von Dritten zur Laufzeit" verbietet |
| **GraphQL** | Headless-Schnittstelle | passt nicht — die Regel ist ein Server, der Seiten ausliefert |
| **OAuth** | Anmeldung über Google, GitHub … | passt nicht — jede Anmeldung wäre ein Aufruf nach draussen |
| **Git-Automatik** | jede Änderung als Commit | passt nicht — wir sind eine Datenbank, keine Dateien |
| **Laravel-Ökosystem** | ein ganzes Framework darunter | passt nicht — und will es nicht |

### Was wir haben und Statamic nicht

Der Vollständigkeit halber, weil ein Vergleich sonst schief steht:

- **Plugins laufen in einer Sandbox.** Ein Statamic-Addon ist PHP mit allen
  Rechten des Servers. Unsere Plugins sind WebAssembly: kein Dateisystem, kein
  Netz, nur die Host-Funktionen, die im Manifest stehen — und eine
  Endlosschleife wird nach zwei Sekunden abgebrochen.
- **Nichts wird zur Laufzeit von Dritten geholt.** Nicht als Empfehlung,
  sondern durchgesetzt: Content-Security-Policy auf jeder Antwort, und
  Theme-Archive werden beim Hochladen abgelehnt, wenn sie eine fremde Adresse
  laden.
- **Eine Datei.** Kein PHP, kein Composer, kein Node, kein Webserver davor
  nötig. `scp` und `systemctl restart`.
- **KI-Anschluss eingebaut.** MCP mit eigenem Schlüsselverwaltung, Lese- und
  Schreibrechten je Website.
- **Zwei-Faktor ist Pflicht**, nicht Option.
- **Design-Werte je Website** ohne Theme-Wechsel.

---

## Vorschlag: die nächsten Schritte

Nach Wirkung geordnet, mit dem, was ich tatsächlich bauen würde.

### 1. Eigene Felder je Website („Feldsätze") — **gebaut**

Das Fundament. Nicht Statamics volle Blueprints — eine kleinere Fassung, die
90 % davon kann:

- Je Website eine Liste zusätzlicher Felder für Seiten: Kennung, Beschriftung,
  Art, Pflicht ja/nein, Hinweis.
- Acht Arten zum Anfang: **Text, Langtext, Zahl, Datum, Ja/Nein, Auswahl,
  Bild, Link**. Genau die, die der Formularbaukasten schon kann — der Code
  dafür ist geschrieben und hat sich bewährt.
- Gespeichert als JSON an der Seite, wie die Bausteine. Kein Schema-Umbau je
  Feld.
- Im Theme erreichbar als `{{ .Felder.preis }}`.
- Optional gebunden an die Art: Felder nur für Beiträge, nur für Seiten, oder
  für eine eigene dritte Art.

**Warum zuerst:** Ohne das bleibt jede neue Inhaltsform eine Änderung am Kern.
Mit dem wird der Shop eine Anwendung statt einer Erweiterung.

*Umgesetzt.* Acht Arten, je Website definierbar, im Editor unter dem Inhalt, im
Theme als `.Page.Felder.kennung` und als `.Page.Feldliste`, geprüft in der
Verwaltung, beim Import und über den KI-Zugang, und in Export und Import
enthalten. Siehe README, Abschnitt *Eigene Felder*.

### 2. Wiederholbare Feldgruppen — **gebaut**

Der zweite Teil desselben Gedankens: eine Gruppe von Feldern, die man mehrfach
ausfüllt. Öffnungszeiten, Teammitglieder, Produktvarianten, Preisstaffeln.

*Umgesetzt.* Die Art „Gruppe“, eine Ebene tief, mit Zeilen zum Hinzufügen,
Entfernen und Verschieben — alles gewöhnliche Absenden-Knöpfe, ohne
JavaScript. Im Theme als `{{range .Page.Felder.preisstaffel}}`; in Export,
Import und KI-Zugang enthalten. Damit deckt unser Feldsystem ab, wofür Statamic
`replicator`, `grid`, `group` und `table` hat.

### 3. Shop auf dieser Grundlage — **gebaut**

Ein Produkt ist eine Seite mit Feldern: Preis, Einheit, Verfügbarkeit, Bild.

*Umgesetzt* als Plugin `plugins/bestellung`. Es kam ohne Warenkorb aus: wer bei
einem Hof bestellt, wählt einmal aus und schickt ab, und ein Warenkorb hätte
eine Sitzung je Besucher, ein Plätzchen und eine zweite Seite gebraucht.
Zahlung bleibt draussen — ein Zahlungsanbieter wäre ein Aufruf nach draussen
zur Laufzeit. Siehe README, Abschnitt *Hofladen*.

### 4. Mehrsprachigkeit — **gebaut**

Eine Seite in mehreren Sprachen, verbunden über eine gemeinsame Kennung, mit
`hreflang` im Kopf und einem Sprachumschalter im Theme.

*Umgesetzt.* Die tragende Entscheidung: die Hauptsprache behält ihre Adressen,
jede weitere liegt unter ihrem Kürzel (`/fr/contact`). Damit bricht kein
bestehender Link, wenn jemand eine zweite Sprache einschaltet, und eine
Website mit einer Sprache merkt von der ganzen Sache nichts.

Die Fassungen bilden einen Stern statt einer Kette — alle zeigen auf dieselbe
Seite in der Hauptsprache —, damit die Reihenfolge des Anlegens nicht dauerhaft
eingebaut ist und das Löschen einer mittleren Sprache die Gruppe nicht
zerreisst. *Fassung anlegen* kopiert die Seite samt Bausteinen und eigenen
Feldern als Entwurf in die andere Sprache. Siehe README, Abschnitt
*Mehrsprachigkeit*.

Anders als bei Statamic sind Adressen je Website eindeutig, über alle Sprachen
hinweg: der Zwang steht seit der ersten Fassung am Tabellenkopf von `pages`,
und ihn zu lockern hiesse, eine Tabelle mit Fremdschlüsselkindern in einer
Wanderung neu zu bauen. Der Preis dafür ist klein — eine französische Seite
heisst ohnehin anders als die deutsche.

### 5. Übersetzte Verwaltung — **gebaut**

Unsere Verwaltung war fest deutsch — jeder Text stand in der Vorlage. Für ein
CMS, das andere hosten sollen, war das eine harte Grenze.

*Umgesetzt.* Die tragende Entscheidung ist eine andere als bei Statamic (und
bei den meisten): **der deutsche Satz ist der Schlüssel.** Eine Vorlage schreibt
`{{t "Seite gespeichert"}}`, nicht `{{t "page.saved"}}`. Zwei Dinge folgen
daraus, und beide sind mehr wert als ein ordentliches Schlüsselschema:

- Eine fehlende Übersetzung fällt auf Deutsch zurück — auf einen Satz, den
  jemand lesen kann, statt auf einen Bezeichner, der ein Fehler auf dem
  Bildschirm ist.
- Die Vorlagen bleiben lesbar. Man sieht, was ein Bildschirm sagt, ohne
  nachzuschlagen. Das ist der Unterschied zwischen einer Übersetzung, die
  gepflegt wird, und einer, die verrottet.

1011 Zeichenketten, vollständig in **fünf Sprachen**: Deutsch, Englisch,
Französisch, Italienisch, Spanisch — dazu die **Schweizer Fassung** der drei
Landessprachen (`de-CH`, `fr-CH`, `it-CH`), die nur ihre Abweichungen trägt und
alles Übrige aus der Grundsprache holt. Ein Schweizer Browser bekommt sie von
allein. Die Sprache gehört zur Person, nicht zur
Website: unter *Mein Konto* wählt jeder seine eigene, ohne Wahl entscheidet
`Accept-Language` — auch auf dem Anmeldebildschirm, wo eine unlesbare Sprache am
schlimmsten wäre.

Und weiter als Statamic an einer Stelle: **eine Sprache ist eine Datei, die man
im laufenden Betrieb hineinlegt.** `data/sprachen/nl.json` — hochladen oder
hineinkopieren, *Neu einlesen* drücken, fertig; kein Neubau, kein Neustart.
Dieselbe Mechanik wie bei Vorlagen: die Platte gilt über dem Eingebauten, also
korrigiert eine `de.json` mit zehn Zeilen auch zehn unserer eigenen
Formulierungen. Geprüft wird vor dem Ablegen — Platzhalter müssen passen,
Auszeichnung wird gefiltert, und was fehlt, erscheint auf Deutsch. Siehe README,
*Sprache der Verwaltung*.

### 6. Eigene Inhaltsarten — **gebaut**

Der letzte Teil des Gedankens, mit dem diese Liste anfing. Eigene Felder sagen,
*was* an einer Seite steht; eine eigene Inhaltsart sagt, *was für ein Ding* die
Seite ist. Statamic nennt das *Collections*.

*Umgesetzt.* Unter *Inhaltsarten* legt eine Website ihre eigenen an — Produkt,
Termin, Rezept, Tier —, jede mit Einzahl, Mehrzahl, einer Kennung fürs Theme,
einer Übersichtsadresse und einer Sortierung. Danach:

- Der Editor bietet die Art zur Wahl, neben *Seite* und *Beitrag*.
- Ein Feld gilt wahlweise für alle, nur für Seiten, nur für Beiträge oder
  **nur für eine Art** — das Preisfeld steht am Produkt und sonst nirgends.
- Die Liste filtert danach und zeigt die Art als Spalte.
- Die Übersichtsadresse liefert öffentlich eine Seite, die alle Einträge der
  Art aufzählt (`/hofladen`), und steht in der Sitemap.
- Export, Import und der KI-Zugang tragen die Art mit.

Zwei Entscheidungen sind erwähnenswert. Die Art ist eine **eigene Spalte
`art`** und nicht ein dritter Wert in `kind`: `kind` trägt eine CHECK-Bedingung
in einer STRICT-Tabelle, und `pages` hat Fremdschlüsselkinder — der Wert wäre
nur über einen vollständigen Tabellenneubau zu erweitern, und genau der hat in
Wanderung 00031 beinahe die Menüs gekostet. Und die Adressen bleiben, wie sie
sind: ein Produkt wohnt unter `/wollpaket-gross`, nicht unter
`/produkte/wollpaket-gross`. Eine Art zu vergeben oder zu entfernen bricht so
keinen einzigen Link.

### 7. Abschnitte und Bedingungen im Formular — **gebaut**

Statamics Blueprints teilen ein Formular in *sections* und lassen ein Feld
über *conditions* erscheinen und verschwinden. Beides fehlte, und beides ist
das, was aus zwanzig Feldern untereinander ein Formular macht, das jemand
ausfüllt.

*Umgesetzt.* Ein **Abschnitt** ist eine Feldart ohne Eingabe — eine Überschrift,
unter der alles bis zum nächsten Abschnitt steht. Eine **Bedingung** hängt ein
Feld an ein anderes: „Sonderpreis" erscheint erst, wenn „Im Angebot" angekreuzt
ist. Solange es nicht erscheint, wird es nicht verlangt (auch als Pflichtfeld
nicht), das Theme gibt es nicht aus — und sein Wert bleibt trotzdem gespeichert,
sodass ein versehentlich entferntes Häkchen niemandem seine Eingabe kostet.

Der interessante Teil ist, was die Regel „kein JavaScript" hier bestimmt hat.
Das Ein- und Ausblenden im Browser ist eine Handvoll CSS-Regeln, und die
brauchen zwei Dinge: die abhängigen Felder stehen im HTML **neben** dem Feld,
an dem sie hängen — ein Stylesheet reicht dorthin und nirgendwo sonst hin —,
und die Bedingung prüft nur, was ein Browser ohne Skript ablesen kann,
nämlich **ausgefüllt oder nicht**.

Auf einen bestimmten Wert zu prüfen — Statamic kann das — hiesse, je Website
ein Stylesheet zu erzeugen: eine zweite Stelle, an der dieselbe Regel steht,
und die beiden liefen früher oder später auseinander. Der Verzicht kostet
wenig: fast jede Bedingung in einem Redaktionsformular ist ein Ja/Nein.

### 8. Eigene Bausteinarten — **gebaut**

Der letzte Satz aus der Überschrift dieses Papiers, eine Ebene tiefer: Der
Bausteineditor konnte neun Arten, und die standen im Go-Code. Eine zehnte —
ein Rezeptschritt, ein Öffnungszeiten-Kasten, eine Preiszeile — war eine neue
Fassung des Programms.

*Umgesetzt.* Unter *Bausteinarten* legt eine Website ihre eigenen an; woraus so
eine besteht, bestimmt sie mit **denselben Feldern wie eine Seite**. Genau das
ist der Grund, warum es wenig neuen Code gebraucht hat: die Bildauswahl, die
Klappliste, das Datumsfeld und ihre Prüfungen gibt es seit Punkt 1, und die
Felder einer Bausteinart wohnen in derselben Tabelle wie die einer Seite — mit
einem Verweis mehr und zwei ausgetauschten Indizes.

Zwei Entscheidungen, die den Rest erklären:

**Die Auszeichnung schreibt der Kern, das Aussehen macht das Theme.** Ein
eigener Baustein wird zu `<div class="hc-block hc-eigen hc-eigen--rezeptschritt">`
mit einer Klasse je ausgefülltem Feld. Statamic lässt den Betreiber ein
Template dazuschreiben; das wäre hier eine Vorlagensprache in einem Textfeld,
also ein Weg, ein `<script>` durch die Vordertür auf eine Seite zu bringen —
und dieses Programm steht auf dem Versprechen, dass es so einen Weg nicht gibt.
Wo eine Klasse nicht reicht, gibt es das **lange Textfeld**: es läuft durch den
Markdown-Renderer und durch dieselbe Reinigung wie jeder andere Text, also
schreibt ein Redakteur `### Schritt 3` und bekommt eine Überschrift.

**Keinen Verweis im Baustein.** Ein Verweis existiert, um eine Umbenennung zu
überleben; ein Baustein wird beim Speichern der Seite ein für alle Mal in HTML
verwandelt und könnte dieses Versprechen nicht halten. Lieber gar nicht
anbieten als eine Zusage, die still bricht — der Link tut dieselbe Arbeit und
sagt, was er ist.

### 9. Kleinigkeiten, die viel bringen

- **Eigene Spalten** in der Seitenliste (was sieht man beim Durchblättern).
- **Gespeicherte Filter** — „meine Entwürfe", „zur Prüfung".
- **Erneute Passwortabfrage** vor Löschen einer Website oder Anlegen eines
  KI-Schlüssels.
- **Vollbildmodus** im Editor — ein Ankreuzfeld und drei CSS-Regeln.
- **WordPress-Import** — WXR ist XML, das ist eine Nachmittagsarbeit für
  Seiten und Beiträge, länger für Medien.
- **Video-Baustein für eigene Dateien** — *gebaut*: hochgeladenes MP4 in
  `<video>`, ohne YouTube und ohne Metadaten in der Datei.

### 10. Was ich bewusst nicht bauen würde

- **Befehlspalette und Passkeys** — beides braucht JavaScript, das wir nicht
  haben wollen. Der Gewinn wiegt die Ausnahme nicht auf.
- **YouTube-Einbettung** — die Regel „nichts von Dritten zur Laufzeit" ist der
  Grund, warum dieses CMS ohne Cookie-Banner auskommt. Ein Einbettungscode
  kostet genau das.
- **GraphQL, OAuth, Git-Automatik, statischer Export** — jedes davon ist eine
  zweite Betriebsart neben der einen, die funktioniert.

---

## Zusammengefasst

Wir sind näher dran, als die Zahl 47 vermuten lässt: Fassungen, Medien,
Bildausschnitt, Bausteineditor, Vorschau, Filter, Schutz, Zwei-Faktor,
Formulare, Import/Export und eine Schreib-Schnittstelle sind da. Die eine
Sache, die anfangs fehlte — dass der Betreiber selbst bestimmen kann, woraus
seine Inhalte bestehen —, ist gebaut: eigene Felder, wiederholbare Gruppen,
Verweise zwischen Seiten, eigene Inhaltsarten, Abschnitte und Bedingungen. Der
Hofladen ist deshalb keine Sonderbehandlung im Kern mehr, sondern eine
Anwendung des Vorhandenen.

Damit ist die Lücke, mit der dieses Papier anfing, auf beiden Ebenen zu: an
der Seite und am Baustein. Was bleibt, ist keine Bauart mehr, sondern eine
Liste einzelner Sachen — Auswahl auch als Knopfreihe statt nur als Klappliste,
Schlagwörter als Feldart, Textbausteine mit anderen Typen als Text,
CSV-Import, statischer Export. Jedes davon ist ein Nachmittag, keines ändert,
wie das Programm gebaut ist.
