---
phase: quick-260903-rqq
plan: 01
subsystem: templates/public
tags: [theme, css, design-system, a11y, print]
status: complete

requires:
  - cmd/holzcloud/templates/public/holzcloud/  # Referenz für Aufbau und Tonfall
  - cmd/holzcloud/assets/bausteine.css         # die Brücke zu den neun Bausteinarten
  - internal/design/tokens.go                  # die sechs Regler der Verwaltung
provides:
  - "Vorlage `weide` in heller Fassung: warmes Papier, Erdbraun, Manrope"
  - "zwölf Ansichten in einer gemeinsamen Bauteil-Sprache (.hc-*)"
  - "Manrope als zweiter, byteweise identischer Ort im Repository"
affects:
  - cmd/holzcloud/assets/VENDOR.md

tech-stack:
  added: []
  patterns:
    - "Drei Stufen der Kaskade: vier @layer < schichtlose CMS-Schicht < .Site.Design"
    - "Alle Farben als color-mix(in srgb, …) aus vier Entscheidungen abgeleitet"
    - "Brücke zu den Reglern: vier Zeilen, Richtung immer vom Regler zum System"
    - "Brücke zu den Bausteinen: zehn generische Namen statt nachgebauter .hc-block-Regeln"
    - "Beschriftungen als gesperrte Versalien, Beträge über font-variant-numeric: tabular-nums — keine zweite Schriftfamilie"

key-files:
  created:
    - cmd/holzcloud/templates/public/weide/favicon.svg
    - cmd/holzcloud/templates/public/weide/fonts/manrope-latin.woff2
    - cmd/holzcloud/templates/public/weide/fonts/manrope-latin-ext.woff2
  modified:
    - cmd/holzcloud/templates/public/weide/style.css
    - cmd/holzcloud/templates/public/weide/layout.html
    - cmd/holzcloud/templates/public/weide/home.html
    - cmd/holzcloud/templates/public/weide/page.html
    - cmd/holzcloud/templates/public/weide/list.html
    - cmd/holzcloud/templates/public/weide/search.html
    - cmd/holzcloud/templates/public/weide/gate.html
    - cmd/holzcloud/templates/public/weide/404.html
    - cmd/holzcloud/templates/public/weide/maintenance.html
    - cmd/holzcloud/templates/public/weide/shop.html
    - cmd/holzcloud/templates/public/weide/product.html
    - cmd/holzcloud/templates/public/weide/cart.html
    - cmd/holzcloud/templates/public/weide/checkout.html
    - cmd/holzcloud/templates/public/weide/order.html
    - cmd/holzcloud/assets/VENDOR.md

decisions:
  - "Die schwächste Tinte steht auf 64 % (5.00:1 auf #FAF6EF), nicht auf den 52 % aus holzcloud — die gelten für helle Tinte auf dunklem Grund und ergäben hier 3.45:1"
  - "Die Kopfleiste bekommt als einzige Fläche einen backdrop-filter; bei blosser Deckung geisterte der Fliesstext von unten als graue Schrift durch"
  - "@media print wurde aus Aufgabe 3 in Aufgabe 1 vorgezogen: TestShippedThemesRenderEveryView verlangt für jede mitgelieferte Vorlage ein Druck-Stylesheet"
  - "Manrope liegt bewusst an zwei Orten statt einmal: eine Vorlage muss als hochladbares Archiv für sich stehen, und /t/fonts/ zeigt immer in die ausgelieferte Vorlage"
  - "Kein JetBrains Mono: Beschriftungen sind gesperrte Versalien, Beträge richtet tabular-nums aus — eine Schreibmaschine klingt auf einem Bauernhof technisch"
  - "Unter 1000 px MUSS die oberste Menüliste eine Spalte sein; als Zeile mit flex-wrap erzeugt ein <li> mit Untermenü ein Treppenmuster"
  - "Der Checkbox-Umschalter kommt zurück: oberhalb der Schwelle auf display: none, damit er dort weder im Tabulatorweg noch im Baum der Hilfsmittel steht"
  - "Ein Bild allein in seinem Absatz spannt bis breit-ende — ohne diese Zeile lässt eine Seite aus reinem Markdown 62 % der Breite leer"
  - "Überschriften: die Grösse gilt überall im Inhalt (Nachfahrenselektor), der grosse Abstand nur auf der obersten Ebene (Kindselektor) — zwei Reichweiten mit Absicht"
  - "Eine Überschrift über einem bildtext KANN nur im Markdown des Bausteins stehen: render.go gibt blk.Title allein beim aufruf aus, block_list.html bietet ein Titelfeld auch nur dort an"
  - ".hc-karten gehört in die Breit-Regel — eine Kartenreihe ist dieselbe Geste wie eine Galerie"
  - "Die Breit-Regel nennt jetzt die REGEL statt der Liste: breit ist, was von sich aus mehrspaltig ist oder ein Bild in natürlicher Grösse zeigt; schmal bleibt, was gelesen wird"
  - ".hc-aufruf bleibt absichtlich schmal — ein Kasten mit einem Satz und einem Knopf, mittig gesetzt, wäre über die volle Breite ein Halbsatz in einer leeren Fläche"
  - "Ein Bild wird in seiner eigenen Grösse gezeigt: kein Hochrechnen (inline-size: auto) und eine Höhengrenze, für die drei Bildarten, die nicht zugeschnitten werden"
  - "Kein object-fit: cover ausserhalb von Karte und Galerie — nur dort liefert render.go ein object-position aus dem Bildmittelpunkt; sonst schnitte es blind aus der Mitte"
  - "Ein Aufruf bleibt mittig, solange er ein Zuruf ist; ab dem zweiten Absatz oder einer Liste wird er linksbündig"
  - "Für ein Bild hat breit drei Stufen: Textspalte, --breit (Rasterspalte), --voll (Fensterbreite); ein Bild allein im Markdown entspricht --breit"

metrics:
  duration: 104m
  completed: 2026-09-03
  tasks: 3
  commits: 11
  files: 18

actuals:
  tokens: 40800
  tasks: 3
  commits: 11
---

# Quick-Aufgabe 260903-rqq: Vorlage `weide` neu gestalten — Summary

Die mitgelieferte öffentliche Vorlage `weide` trägt jetzt dieselbe Architektur
und dieselben Gesten wie die Vorlage `holzcloud`, aber ins Helle übersetzt:
warmes gebrochenes Weiss statt Nachtbraun, Erdbraun statt Messing, Manrope statt
Systemschriftstapel, und Karton mit einer Haarlinie statt Glas.

## Was gebaut wurde

**Das Gestaltungssystem.** `style.css` ist von 2332 auf 1657 Zeilen neu
geschrieben, in drei Stufen, und die Reihenfolge ist die ganze Mechanik: vier
`@layer weide.*` (tokens, base, components, motif), darunter die bewusst
schichtlose CMS-Schicht, und danach im `<style>` die Werte der Website. Jede
Stufe gewinnt gegen die darüber, ohne dass irgendwo ein Spezifitätsspiel oder
ein `!important` nötig wäre.

In `weide.tokens` wird genau viermal ein Farbwert entschieden — Papier `#FAF6EF`,
Tinte `#231A11`, Marke `#6E4D32`, dazu die Schrift auf der Marke. Alles andere
ist eine Ableitung über `color-mix(in srgb, …)`. Weil `var()` erst am Ende
aufgelöst wird und die CMS-Schicht die vier Reglernamen der Verwaltung auf
genau diese Namen legt, zieht eine eingestellte Marke die halbe Palette mit:
den Braunschleier im Grund, die Kante beim Überfahren, den Zitatbalken, den
Fokusring. Keine Ableitung musste dafür wiederholt werden.

**Die Kontraststufe, neu gemessen.** Die `.52` aus holzcloud ist dort mit dem
Vermerk versehen, dass `.42` einmal durchfiel — aber sie gilt für helle Tinte
auf dunklem Grund. Auf diesem Papier nachgerechnet: 52 % ergeben 3.45:1 und
fallen durch, 60 % ergeben 4.40:1 und fallen knapp durch, 64 % ergeben 5.00:1.
Das ist der Wert, den die Datei festschreibt, mit der Rechnung im Kommentar
daneben. `--hc-ink-2` bei 80 % ergibt 8.59:1, die Marke gegen Papier 7.04:1 in
beide Richtungen — sie trägt damit als Verweisfarbe und als Knopffüllung mit
heller Schrift. Alle vier Zustandsfarben stehen über 5.4:1 gegen Papier und
gegen eine Karte.

**Die Hülle.** `layout.html` hat eine klebende Kopfleiste mit Marke links, Menü
rechts, Sprachwahl, Warenkorb und Suchformular. Untermenüs öffnen über `:hover`
und `:focus-within` — keine Zeile JavaScript, und die Tastatur erreicht sie
genauso wie der Zeiger. Der Checkbox-Schalter der alten Fassung ist weg: unter
1000 px bricht die Leiste um und das Menü nimmt die zweite Zeile. Warenkorb und
Suche standen bisher gar nicht in der Leiste, obwohl die Vorlage `shop.html` und
`cart.html` mitbringt — vom Kopf der Seite kam damit niemand zum Korb.

Die Fusszeile behält alles, was die alte Fassung zeigte: das Schaf als
eingebauten Pfad in `currentColor`, Name, Beschreibung, `footer-kontakt`,
`schwesterseite`, Fussnavigation, Schlagwörter und die Copyright-Zeile.

**Der Textkörper.** `.page-content` ist ein Raster aus zwei Spalten: der
Fliesstext bleibt beim Mass aus `--hc-measure` (Vorgabe 74 Zeichen), eine
Galerie, ein Video und ein Bild über die volle Breite laufen bis an den rechten
Rand. Die zweite Spalte ist unter dem Mass null breit, also stapelt sich das auf
dem Telefon von selbst, ohne eine zweite Regel in einer Medienabfrage. Weil die
Seiten, für die diese Vorlage gedacht ist, reiner Markdown-Fluss sind, bekam
dieser Fluss eigene Regeln: 64 px vor jeder `h2`, ein Bild allein in seinem
Absatz wird über `:has()` zur Figur mit Luft davor und danach — sonst klebte
der Absatz nach einem Bild an ihm.

**Der Aufmacher.** Links Augenbrauen-Zeile aus `.Site.Name`, Schlagzeile aus
`.Page.Title` (nur wenn die Seite nicht schon mit einer eigenen Überschrift
beginnt), eine Zeile Anspruch aus `.Site.Description`, dann abgesicherte
Knöpfe. Rechts das Titelbild aus `.Meta.OGImage` mit `aspect-ratio: 4/3` und
`object-fit: cover`, damit ein Hochformat den Aufmacher nicht in die Länge
zieht; ohne Titelbild eine ruhige Hügelzeichnung, gebaut allein aus den Klassen
der Schicht `weide.motif` — kein `fill` und kein `stroke` als Literal im Markup,
damit sie einer eingestellten Marke folgt.

`.Page.Excerpt` steht ausdrücklich **nicht** im Aufmacher, mit einem Kommentar,
der sagt warum: der Anriss wird beim Speichern aus demselben Markdown
abgeleitet, das zwei Abschnitte weiter unten als Inhalt noch einmal kommt. Genau
das war der einzige Fehler, den die Sichtprüfung bei `holzcloud` fand.

**Die neun Bausteinarten** sind über die zehn Zeilen der Brücke gekleidet.
Gezielte Regeln stehen nur dort, wo auf hellem Papier aus der Brücke etwas
Falsches folgt: eine Haarlinie am Bild, damit ein helles Bild nicht ausfranst;
eine deckende Karte statt einer Glasfläche; gesperrte Versalien statt der
Mono-Beschriftung, mit `opacity: 1` zurückgesetzt, weil die Farbe die Abstufung
schon trägt; und die ausdrückliche Schriftfarbe am `.hc-knopf`, ohne die er als
erdbrauner Text auf erdbrauner Fläche stünde.

**Die zwölf Ansichten** sind alle erhalten und in die neue Sprache übersetzt.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 – Blockierend] `@media print` aus Aufgabe 3 in Aufgabe 1 vorgezogen**
- **Gefunden bei:** Aufgabe 1, beim Prüftor `go test ./internal/template/...`
- **Problem:** `TestShippedThemesRenderEveryView` (render_test.go:297) verlangt
  von jeder mitgelieferten Vorlage ein `@media print`. Der Plan sah den
  Druck-Abschnitt erst in Aufgabe 3 vor; die Testreihe war damit nach Aufgabe 1
  rot, und der Durchstich hätte kein grünes Tor gehabt.
- **Fix:** Der vollständige Druck-Abschnitt wie geplant, nur zwei Aufgaben
  früher geschrieben. Aufgabe 3 hat ihre Regeln davor eingefügt, damit er
  weiterhin die letzte Regel der Datei ist.
- **Datei:** `cmd/holzcloud/templates/public/weide/style.css`
- **Commit:** 62d022c

**2. [Rule 1 – Fehler] Die Kopfleiste liess den Text darunter durchgeistern**
- **Gefunden bei:** Aufgabe 3, bei der Durchsicht der Medienabfragen
- **Problem:** Der Plan setzte `--hc-pane-bar` auf 82 % Deckung und schloss
  einen `backdrop-filter` ausdrücklich aus. 18 % einer Tinte, die gegen Papier
  15.9:1 misst, sind aber noch gut zu sehen: unter der klebenden Leiste stand
  der scrollende Fliesstext als graue Geisterschrift.
- **Fix:** Deckung auf 88 % und ein `backdrop-filter: blur(12px) saturate(140%)`
  allein an `.hc-bar` — die einzige Fläche der Vorlage mit einem. Dazu ein
  `@supports not (backdrop-filter)`-Zweig und ein Zweig für
  `prefers-reduced-transparency`, die beide auf `--hc-pane-bar-solid` (98 %)
  schalten. Karten bleiben deckend wie in D-18 verlangt; die Leiste ist keine
  Karte, und die Vorlage `holzcloud` behandelt sie ebenso.
- **Datei:** `cmd/holzcloud/templates/public/weide/style.css`
- **Commit:** fef2554

**3. [Rule 1 – Fehler] Ein langer Betriebsname sprengte die Leiste auf dem Telefon**
- **Gefunden bei:** Aufgabe 3, bei der Durchsicht gegen 390 px
- **Problem:** `.site-mark` stand nach dem Muster von `holzcloud` auf
  `flex: none`. Dort trägt die Marke einen kurzen Namen; hier heisst ein Betrieb
  „Milchschäferei Seehof", und ein `flex: none`-Element besteht auf seiner
  `max-content`-Breite — die Leiste lief auf einem Telefon nach rechts hinaus.
- **Fix:** `flex: 0 1 auto` mit `min-width: 0`, dazu `overflow-wrap: break-word`
  am Namen.
- **Datei:** `cmd/holzcloud/templates/public/weide/style.css`
- **Commit:** fef2554

### Sonstige Abweichungen

- **`.hc-bildtext__bild img` bekam eine Haarlinie**, die der Plan nicht nennt.
  Sie folgt derselben Begründung wie beim Bild-Baustein: ein helles Bild neben
  einem Absatz auf hellem Papier braucht eine Kante, um als Bild anzufangen.
- **Der Druck-Abschnitt enthält drei Literalfarben** (`#fff`, `#ddd`, `#bbb`) in
  einem Bereich, in dem laut Plan kein Farbwert stehen soll. Das ist bewusst und
  entspricht der Vorlage `holzcloud`: auf Papier ist Weiss das Papier des
  Druckers und kein Wert der Palette.
- **Die gemessene Kontrastzahl heisst 5.00:1, nicht 5.02:1** wie im Plan. Der
  Plan rundete anders; der Kommentar in der Datei trägt die selbst nachgerechnete
  Zahl, weil eine geschönte Messung die nächste Entscheidung verdirbt.

## Threat Flags

Keine. Die Arbeit fügt keine Netzwerkschnittstelle, keinen Authentifizierungspfad
und keine Schemaänderung hinzu; `CheckNoExternalRefs` und `CheckNoScripts` laufen
über `template check` und melden nichts.

Die drei zugesagten Minderungen sind erfüllt:
- **T-rqq-01** — `@font-face` zeigt auf `/t/fonts/…`; kein Byte von einem
  fremden Ursprung. Bestätigt von `template check` und von
  `TestBuiltinTemplatesHaveNoExternalRefs`.
- **T-rqq-02** — kein `.js`, kein `<script>` ausser dem `application/ld+json`-
  Datenblock, kein `on*`, keine `javascript:`-Adresse. Bestätigt von
  `CheckNoScripts`.
- **T-rqq-03** — beide woff2 sind in jeder der drei Aufgaben mit `cmp` gegen die
  in VENDOR.md mit SHA-256 verzeichneten Originale geprüft worden.

## Known Stubs

Keine. Kein hartkodierter Leerwert, kein „coming soon", kein TODO in den
sechzehn berührten Dateien.

## Prüfungen

| Prüfung | Ergebnis |
|---|---|
| `holzcloud template check …/weide` | no problems found (zwölf Ansichten gegen SampleData UND MinimalData) |
| `go build ./...` | grün |
| `go test ./internal/template/... ./internal/tmplmgr/...` | grün |
| `go test ./...` | grün |
| `go run ./tools/i18n` | en/es/fr/it je 1128 übersetzt, 0 offen, 0 verwaist |
| Kaskade | genau vier `@layer` am Zeilenanfang, alle vor der CMS-Markierung |
| Regler | die Vorlage setzt keinen der vier gebrückten Namen (Zähler 0) |
| Schriften | beide woff2 byteweise identisch mit den geprüften Originalen |
| dreizehn HTML-Dateien | zwölf Ansichten plus `layout.html` |

**Nicht automatisiert und darum hinterher zu tun:** ein Blick im Browser auf
Startseite, Textseite, Archiv und ein schmales Fenster. Genau dieser Blick fand
bei `holzcloud` den einen Fehler, den keine der obigen Prüfungen sah.

## Nach der Sichtprüfung

Der Orchestrator hat die Vorlage im Browser gegen die echte Website der
Milchschäferei Seehof geprüft. Aufmacher, Schrift, Farbwelt und Textkörper
trugen; zwei Befunde kamen zurück und sind in denselben Quick-Task geflossen.

### Befund 1 — Die Navigation unter 1000 px war eine Treppe (Commit 5df88df)

Bei 390 px, fünf Hauptpunkte, zwei davon mit Untermenü: die Punkte standen auf
drei verschiedenen Einzügen, und die Reihenfolge war nicht mehr zu lesen.

**Die Ursache gehört festgehalten, weil sie unsichtbar ist und wiederkommt.**
Die oberste Liste `.site-menu ul` steht in der CMS-Schicht auf
`display: flex; flex-wrap: wrap` in ZEILENrichtung. Unter der Schwelle wurde
das Untermenü zwar von `position: absolute` auf `static` gestellt, aber die
oberste Liste blieb eine Zeile. Ein `<li>` mit Untermenü ist darin ein breites
Flex-Element, dessen Kinder — der `<a>` und das jetzt statische `<ul>` —
nebeneinander laufen; die Zeile bricht um dieses breite Element herum, und
daraus entsteht die Treppe. Das `flex-direction: row` auf `li > ul` verstärkte
es.

**Die Regel, die das verhindert:** unter 1000 px stehen BEIDE Menüebenen
ausdrücklich auf `flex-direction: column; align-items: stretch`, jedes `<li>`
über die volle Breite. Wer das eines Tages „aufräumt", holt das Treppenmuster
zurück.

Dazu kam die Menge: sieben Menüzeilen sind rund 320 px Leiste, bevor auf einem
Telefon der erste Inhalt kommt — auf jeder Seite. Der Umschalter, den die erste
Fassung dieser Arbeit gestrichen hatte, ist deshalb zurück, als Checkbox ohne
eine Zeile Skript.

Der ursprüngliche Einwand gegen ihn — „ein Schalter, den niemand sieht, steht
trotzdem im Tabulatorweg" — trifft nur eine Fassung, die ihn bloss optisch
versteckt. Hier stehen oberhalb von 1000 px Checkbox UND Beschriftung auf
`display: none`, und damit sind sie aus dem Tabulatorweg und aus dem Baum der
Hilfsmittel heraus. Unterhalb ist die Checkbox geklemmt, aber fokussierbar; der
Fokusring erscheint über `.nav-switch:focus-visible + .hc-bar .nav-button` an
der Beschriftung, die an ihrer Stelle steht. Zugeklappt heisst `display: none`,
also liegen auch die Punkte der Untermenüs nicht im Tabulatorweg. Der Sprunglink
bleibt erstes Kind von `<body>` und erstes Tabulatorziel; die Checkbox steht
danach.

Zwei Zugaben, die dabei anfielen:
- Hat eine Website gar kein Hauptmenü, gibt `menuFor` eine leere Zeichenkette
  zurück (`internal/menu/render.go:20`). Dann fällt der Knopf über
  `.hc-bar__inner:not(:has(.site-menu ul))` weg, statt ein leeres Fach zu
  öffnen.
- Die Suche verschwindet unter 600 px nicht mehr aus der Leiste, sie klappt mit
  dem Menü auf. Ganz wegzunehmen hiesse, dass ein Telefon die Suche der Website
  nur noch über die 404-Seite findet.

### Befund 2 — Reiner Fliesstext liess 62 % der Breite leer (Commit a69c3c1)

Auf `/hof` bei 1265 px: die Textspalte rund 440 px, jedes Bild darin ebenfalls
440 px, rechts daneben über 700 px Nichts.

Warum das bei `holzcloud` nicht auffällt: dort füllen Karten, Galerien und der
Aufmacher die zweite Rasterspalte. Diese neun Seiten sind reines Markdown —
Überschrift, Bild, Absatz, Link —, also griff `grid-column: text-start /
breit-ende` nur für `.hc-galerie`, `.hc-video` und `.hc-bild--voll`, und keines
davon kommt dort vor.

Ein Bild, das allein in seinem Absatz steht, spannt jetzt bis `breit-ende`. Der
Selektor dafür stand schon — `.page-content > p:has(> img:only-child)` war
bereits die Figur mit Luft davor und danach —, es fehlte nur die Spaltenangabe.
Eine `<figure>` im Inhalt, die kein `.hc-block` ist, geht mitsamt ihrer
Unterschrift denselben Weg; bluemonday lässt `figure` und `figcaption` durch
(`internal/page/markdown.go:62`), eine handgeschriebene Figur ist also echter
Inhalt und kein toter Selektor.

Die Grenzen:
- Nur bei `:only-child`. Ein Bild mitten im Satz bleibt, wo es ist.
- `inline-size: auto` mit `max-block-size: 70vh` statt `cover` oder `contain`.
  Ein Zuschnitt schneidet auf einer Hofseite Köpfe ab; `contain` legte leere
  Streifen INNERHALB der Haarlinie ab, und die Figur läse sich als schlecht
  sitzendes Bild. So umschliesst die Kante immer genau das Bild: ein Querformat
  füllt die breite Spalte, ein Hochformat schrumpft auf 70vh und steht mittig.
  Der Preis ist, dass ein Bild schmaler als die Spalte nicht hochgerechnet wird
  — das ist die richtige Seite des Handels, ein hochskaliertes Foto ist sichtbar
  weich.
- Auf schmalen Fenstern ändert sich nichts: die zweite Spalte ist dort ohnehin
  null breit.
- Ein Browser ohne `:has()` lässt beide Regeln fallen und zeigt das Bild in der
  Textspalte, also den Zustand von vorher. Tragbarer Rückfall.
- Die `.hc-bild`-Bausteine bleiben unberührt: der Selektor greift nur an
  direkten Kindern von `.page-content`, und `figure:not(.hc-block)` schliesst
  sie aus. (Ein erster Entwurf mit `:where(p, figure) > img:only-child` hätte
  ein `.hc-bild` ohne Unterschrift mitgenommen und ihm die volle Breite aus
  `bausteine.css` genommen — beim Gegenlesen gefunden und vor dem Commit
  eingeengt.)

### Ausdrücklich nicht angefasst

- Die Untermenüs stehen zugeklappt korrekt auf `visibility: hidden; opacity: 0`;
  was auf der Ganzseiten-Aufnahme wie durchscheinende Schrift aussah, ist ein
  Artefakt der Aufnahme.
- Dass das Aufmacherbild in der Entwicklungsumgebung nicht lud, liegt an
  `.Meta.OGImage`: der Wert ist die absolute Adresse aus `CanonicalBase` und
  trägt die Hauptdomain ohne Port. In Produktion stimmt es.
- Der lange Fliesstext bleibt Fliesstext. Die Umgliederung des Inhalts in
  Bausteine ist ein eigener Auftrag in einem anderen Repository.

### Prüfungen nach den zwei Befunden

`template check` no problems found · `go build ./...` grün ·
`go test ./internal/template/... ./internal/tmplmgr/...` grün · `go test ./...`
grün · Kaskade weiterhin genau vier `@layer` vor der CMS-Markierung · die
Vorlage setzt keinen der vier gebrückten Reglernamen.

## Nach dem Umstellen des Inhalts auf Bausteine

Der Orchestrator hat den Inhalt der Milchschäferei auf Bausteine umgestellt
(sechs von neun Seiten, 34 Bausteine) und dabei zwei Fehler gefunden, die die
Vorlage nur auf der obersten Ebene richtig machte. Beide bestätigt, beide in
diesen Quick-Task geflossen.

### Fehler A — Überschriften in Bausteinen blieben klein (Commit 0aef9ee)

`.page-content > :where(h2)` ist ein KINDselektor und traf damit nur direkte
Kinder. Eine Überschrift in `.hc-text` oder `.hc-bildtext__text` bekam weder
Grösse noch Abstand: gemessen 26,25 px im Baustein gegen 36,8 px auf einer
Fliesstext-Seite. Weil auf derselben Website Baustein- und Fliesstextseiten
nebeneinander stehen, war das ein sichtbarer Bruch von Seite zu Seite.

**Der Satz, ohne den die neue Regel später wie unnötige Grosszügigkeit aussieht
und „aufgeräumt" wird:** eine Überschrift über einem Bild-neben-Text KANN gar
nicht anders, als im Markdown des Bausteins zu stehen.
`internal/block/render.go` gibt `blk.Title` nur beim `aufruf` aus (dort als
`.hc-aufruf__titel`), und `templates/admin/block_list.html` bietet ein
Titelfeld auch nur dort an — plus je Karte einer Kartenreihe. Für `bildtext`
und `galerie` gibt es keines. Ein `## …` im Baustein landet deshalb
zwangsläufig in `.hc-bildtext__text`, also als Nachfahre und nicht als Kind.
Auf den sechs umgestellten Seiten betraf das jede einzelne Überschrift.

Grösse und Abstand sind jetzt getrennt, mit zwei verschiedenen Reichweiten:
- **Grösse:** Nachfahrenselektor, gilt überall im Inhalt.
- **Abstand davor:** bleibt Kindselektor. Dort eröffnet eine Überschrift einen
  Abschnitt DER SEITE, und der Absatz davor gehört noch zum vorigen. Innerhalb
  eines Bausteins, der über `.hc-block` schon `margin-block` um sich hat, wäre
  derselbe Abstand doppelt gesetzte Luft; dort trägt der Rhythmus des Bausteins
  (`.hc-text > * + *`, 24 px). Beide Begründungen stehen im Kommentar
  nebeneinander, damit die nächste Lesart sie nicht wieder zusammenführt.

Zwei Dinge dabei geprüft:
- `:first-child` greift weiter, und es addieren sich keine zwei Abstände: den
  Abstand setzt nach wie vor allein der Kindselektor.
- **Ein Fehler, den die Umstellung selbst erst erzeugt hätte:** ein
  Nachfahrenselektor `.page-content h2` misst (0,1,1) und gewinnt gegen
  `.hc-karte__titel` und `.hc-aufruf__titel` (je (0,1,0) aus `bausteine.css`).
  Ohne Gegenmassnahme wäre jeder Kartentitel auf Kapitelgrösse aufgebläht
  worden — in einer 17rem schmalen Karte. Die zwei Überschriften, die der Kern
  SELBST baut, sind deshalb über `:not(.hc-karte__titel, .hc-aufruf__titel)`
  ausgenommen: sie sind Bauteile und keine Gliederung des Textes, ihre Grösse
  kommt aus `--step-1` und `--step-2`.

### Fehler B — `.hc-karten` fehlte in der Breit-Regel (Commit 1307967)

Eine Kartenreihe blieb in der Textspalte. Die Rechnung, warum daraus 2+1
wurde: `bausteine.css` gibt `.hc-spalten-3` ein
`minmax(min(100%, 14rem), 1fr)`, die Fuge ist 24 px (`--hc-luft` über die
Brücke aus `--hc-space-5`), drei Spuren brauchen also
3 × 224 + 2 × 24 = **720 px** — und die Textspalte war bei 1265 px
Seitenbreite **705 px**. Es fehlten fünfzehn Pixel.

Dass im Inhalt daraufhin die Spaltenzahl auf 4 gezwungen wurde, ist eine
Notlösung gegen ein Problem der Vorlage, und sie steht danach in einem
Manifest, wo niemand sie mehr als solche erkennt. Eine Kartenreihe ist gerade
das Element, das die Breite füllen soll — dieselbe Geste wie eine Galerie.
`.hc-karten` steht jetzt in der Aufzählung.

Nachgerechnet mit der vollen Breite (`.hc-wide` 1320 px, Rinne
`clamp(20px, 4vw, 56px)`, Fuge 24 px):

| Fenster | Raster | 3 Karten (`spalten-3`) | 4 Karten (`spalten-4`) |
|---|---|---|---|
| 1280 px | 1178 px | 3 à 377 px | 4 à 276 px |
| 900 px | 828 px | 3 à 260 px | 4 à 189 px |

Keine der vier Reihen bricht um. Zum Vergleich der alte Zustand: in der 705 px
schmalen Textspalte standen drei Karten auf 2 à 340 px.

**Eine Beobachtung ohne Änderung:** vier Karten mit `hc-spalten-3` ergeben bei
900 px drei Spalten, also eine Reihe 3+1. Das ist das `auto-fit`-Verhalten von
`bausteine.css` und war schon vorher so; es ist kein Befund dieser Arbeit und
gehört, wenn überhaupt, in den Kern und nicht in eine Vorlage.

### Fehler C — `.hc-bildtext` fehlte in derselben Aufzählung (Commit c7297cf)

Dritter Befund derselben Familie, am laufenden Server bei 1280 px gemessen.
Ein `bildtext` ist ein Bild NEBEN einem Text, also zwei Spalten — er stand
aber in der 705 px schmalen Textspalte, dem Bild blieben 340 px und dem Text
Zeilen von rund 30 Zeichen. Die Auszeichnung versprach zwei Spalten, das
Raster gab ihr eine halbe.

Nachgerechnet (Fuge 24 px aus `--hc-luft`, Mass 705 px — das Modell trifft die
Messung des Orchestrators auf den Pixel):

| Fenster | Raster | vorher | nachher |
|---|---|---|---|
| 1280 px | 1178 px | 2 × 340 px | 2 × 577 px |
| 1000 px | 920 px | 2 × 340 px | 2 × 448 px |
| 900 px | 828 px | 2 × 340 px | 2 × 402 px |
| 760 px | 699 px | 2 × 338 px | 2 × 338 px |
| 700 px | 644 px | gestapelt | gestapelt |

**Zwei Berichtigungen zur Meldung.** Bei 900 px stapelt `bausteine.css` den
Baustein NICHT: die Abfrage steht bei `45em`, und `em` misst in einer
Medienabfrage immer gegen 16 px, also bei 720 px Fenster. Bei 900 px sind es
weiterhin zwei Spalten, und die Korrektur hilft dort ebenso (340 → 402 px).
Erst ab etwa 760 px fallen breit und schmal zusammen, weil das Mass dann
breiter ist als das Raster; gestapelt wird unter 720 px.

`.hc-bildtext--rechts` geht mit — es ist ein blosser Modifikator derselben
Klasse am selben Element (`internal/block/render.go:140-142`).

### Die Breit-Regel nennt jetzt die Regel statt der Liste

Drei Nachbesserungen an einer Zeile waren zwei zu viel. Der Kommentar davor
sagt deshalb nicht mehr, WAS drinsteht, sondern WARUM:

> Breit ist, was von sich aus MEHRSPALTIG ist oder ein BILD in seiner
> natürlichen Grösse zeigt. Schmal bleibt, was GELESEN wird.

Mehrspaltig: `.hc-galerie`, `.hc-karten`, `.hc-bildtext`. Bild in natürlicher
Grösse: `.hc-video`, `.hc-bild--voll`, dazu die Figur aus einem Bild allein im
Absatz. Gelesen und darum schmal: `.hc-text`, `.hc-zitat`, `.hc-eigen`.

**`.hc-aufruf` bleibt ausdrücklich schmal, und der Grund steht dabei.** Er ist
ein Kasten mit einem Satz und einem Knopf, sein Inhalt ist mittig gesetzt;
über 1178 px gezogen stünde ein Halbsatz in einer sehr breiten leeren Fläche.
Er ist das einzige Element, das absichtlich in der Textspalte bleibt — ohne
diesen Satz wird er beim nächsten Mal „der Vollständigkeit halber" nachgetragen
und dabei kaputtgemacht.

### Fehler D — Ein Bild richtete sich nach dem Rahmen statt umgekehrt (Commit ac0aeb5)

Zwei Meldungen derselben Absicht, in einem Commit und unter einem gemeinsamen
Kommentar: **der Rahmen richtet sich nach dem Bild, nicht das Bild nach dem
Rahmen.** Getrennt geschrieben laden die beiden Regeln dazu ein, später eine
davon zu entfernen — deshalb stehen sie als ein Block mit einer Begründung.

**Erste Hälfte, nicht hochrechnen.** `bausteine.css` sagt eine Ebene höher
schon das Richtige (`.hc-block img` auf `max-inline-size: 100%`, Z. 34), aber
`.hc-bild img` (Z. 50) und `.hc-bildtext__bild img` (Z. 96) überschreiben es
mit `inline-size: 100%` und zwingen damit auch ein kleines Bild auf
Spaltenbreite. Gemessen: die Bildmarke eines Hofes, 235 px breit, wurde in
einer 569 px breiten Bildtext-Spalte 2,4-fach hochgerechnet und franste
sichtbar aus. Jetzt `inline-size: auto` plus `max-inline-size: 100%`; ein
kleines Bild steht mittig in seiner Spalte, statt sie zu füllen. Eine
unscharfe Marke ist schlimmer als eine kleine.

**Zweite Hälfte, eine Höhengrenze.** Ein Hochformat 900 × 1600 stand im
Bildtext auf 533 × 945, der Text daneben war 143 px hoch, und weil
`bausteine.css` die zwei Spalten auf `align-items: center` stellt, begann er
405 px unter der Bildoberkante. Ein `bildtext` verspricht ein Bild NEBEN einem
Text; ist das Bild dreimal so hoch, ist es kein Paar mehr, sondern ein Bild mit
einer Fussnote. Vorher fiel das nicht auf, weil in der schmalen Textspalte
jedes Bild auf 340 px gedrückt wurde — die Verbreiterung (Fehler C) ist
richtig, sie legt den Fall nur offen.

**Warum die Grenze an der Höhe hängt und nicht an einem `cover`:**
`internal/block/render.go` ruft `imgTag` nur für Galerie (Z. 165) und Karte
(Z. 182) mit gesetztem Zuschnitt-Kennzeichen; nur dort entsteht aus dem
Bildmittelpunkt ein `object-position` (Z. 369). Ein `bildtext`-Bild bekommt
keines mitgeliefert (Z. 146, `cropped = false`), ein `object-fit: cover`
schnitte also blind aus der geometrischen Mitte — bei einem Tierbild der Kopf.

Zwei Zahlen für zwei verschiedene Fragen:
- `70vh` für `.hc-bild` und `.hc-eigen__bild`: sie stehen allein in der Spalte
  und sind nicht die Hälfte eines Paares — dieselbe Zahl wie die Figur im
  Fliesstext, aus demselben Grund.
- `min(100cqw, 70vh)` für `.hc-bildtext__bild`: nie höher, als die eigene
  Spalte breit ist. Das ist die Paar-Regel selbst, in eine Länge übersetzt —
  die Grenze folgt dem Layout und nicht dem Fenster. Dafür ist die Bildspalte
  ein Grössen-Container (`container-type: inline-size`).

Nachgerechnet, Bildtext-Spalte 569 px, Fenster 900 px hoch:

| Bild | natürlich | vorher | nachher |
|---|---|---|---|
| Bildmarke | 235 × 235 | 569 × 569 | 235 × 235 |
| hoch | 900 × 1600 | 569 × 1012 | 320 × 569 |
| quer | 1600 × 1200 | 569 × 427 | 569 × 427 |
| quer | 1600 × 900 | 569 × 320 | 569 × 320 |

Nur das Hochformat und das zu kleine Bild bewegen sich; die Querformate
bleiben, wo sie waren.

**Über die Meldung hinaus mitgenommen:** `.hc-eigen__bild` trägt dieselbe
Zeile aus `bausteine.css` (Z. 296) und wird ebenfalls nicht zugeschnitten — es
ist derselbe Fehler, und ohne Kenntnis des Schlüssels einer eigenen
Bausteinart ist Nicht-Hochrechnen das, was sicher richtig ist. Karte und
Galerie sind ausgenommen und behalten ihr `inline-size: 100%`: dort wird in
einen festen Rahmen zugeschnitten, und ein Bild MUSS ihn füllen, sonst klafft
er.

**`bausteine.css` ist nicht angefasst.** Die Datei gehört dem Kern und kleidet
alle mitgelieferten Vorlagen plus jede hochgeladene; eine Änderung dort hätte
eine andere Reichweite. Ihr Kommentarkopf hält ausdrücklich fest, dass ein
Theme alles davon überschreiben darf und die Angaben dafür nur eine Klasse
tief liegen — genau diese Freiheit ist hier benutzt.

### Befund E — Fliesstext im Aufruf stand zentriert (Commit f56f48b)

`bausteine.css` stellt `.hc-aufruf` auf `text-align: center` (Z. 196), und
`weide` überschrieb es nicht. An einer Schwestervorlage mit echtem Inhalt
gemessen: ein `aufruf` mit **vier Absätzen**, 705 px breit, alles mittig.
Fliesstext mit beidseitigem Flatterrand ist schwer zu lesen — das Auge findet
den Anfang der nächsten Zeile nicht, weil er jedesmal woanders liegt. Derselbe
Text stand als handgeschriebenes `<aside>` vorher linksbündig; die Umstellung
auf einen echten Baustein hat ihn also verschlechtert.

**Nicht grundsätzlich linksbündig gesetzt.** Der Kern hat für seinen Fall
recht: `block.go:54` beschreibt den Aufruf als „a boxed invitation with a
button", und ein Satz plus Knopf steht mittig besser. Den guten Fall für den
schlechten zu opfern wäre der falsche Handel. Stattdessen am Inhalt
entschieden, über zwei Merkmale, die beide mit `:has()` sichtbar sind:

- **mehr als ein Kind** im `.hc-aufruf__text` — ein zweiter Absatz, oder ein
  Absatz und eine Liste. Ein Zuruf ist genau ein Satz.
- **überhaupt eine Liste** — Aufzählungspunkte mittig sind in jeder Länge
  falsch; eine Liste hat eine linke Kante, das ist ihr Sinn.

`text-align: start` statt `left`, damit die Regel auch eine Sprache trägt, die
von rechts nach links läuft. Ohne `:has()` bleibt es mittig, also beim
heutigen Zustand — hier ausnahmsweise der schlechtere der beiden Fälle, und
darum im Kommentar ausdrücklich als solcher genannt.

### Befund F — `.hc-bild--breit` war schmaler als ein Bild ohne Auszeichnung (Commit a1d3447)

`bausteine.css` zieht `--breit` ab 60em auf `inline-size: min(115%, 100vw)`
mit `margin-inline: -7.5%` (Z. 58-70). Das ist der Kunstgriff einer Vorlage
OHNE Raster: ein Bild läuft prozentual aus der Textspalte heraus. Gegen unser
Zweispaltenraster ergab das 705 × 1.15 = **810 px**, während ein allein
stehendes Bild im Markdown **1163 px** bekam — der ausdrücklich als „breit"
ausgezeichnete Weg war der schmalere.

Jetzt eine Leiter, und so auch benannt:

| Stufe | Rahmen | bedeutet |
|---|---|---|
| ohne Zusatz | Textspalte | Lesebreite — die bewusste Wahl der Redaktion, die einzige Stufe, die man ansagen muss |
| `--breit` | Rasterspalte | `text-start / breit-ende`, dasselbe Mass wie `.hc-karten` und `.hc-bildtext` |
| `--voll` | Fensterbreite | unverändert aus `bausteine.css` |

Nachgemessen bei 1280 × 900 (Raster 1178 px, Textspalte 705 px):

| Weg | vorher | nachher |
|---|---|---|
| ohne Zusatz | 705 × 397 | unverändert |
| `--breit` | 810 × 456 | **1120 × 630** |
| `--voll` | 1280 × 720 | unverändert |
| Bild allein im Markdown | 1178 × 662 | **1120 × 630** |

Ein allein stehendes Bild im Markdown entspricht damit **genau** `--breit`.
Das ist richtig, weil Markdown keine Möglichkeit hat, „schmal" zu sagen, und
breit die sinnvollere Vorgabe ist. Die zwei Wege dürfen sich unterscheiden:
der eine wird ausgezeichnet, der andere geerbt.

**Dabei ein eigener Fehler aus Commit ac0aeb5 berichtigt.** Die dort
eingeführte Höhengrenze von `70vh` traf auch `--voll` und hätte ein
16:9-Bild auf einem 900er Fenster von 1280 auf 1120 px zusammengeschrumpft —
also gerade dem Modifikator die Breite genommen, die er ansagt.
`.hc-bild--voll img` steht jetzt auf `max-block-size: none`. Das
Nicht-Hochrechnen gilt dort weiter, und das ist kein Widerspruch: eine
hochgerechnete Aufnahme ist BESCHÄDIGT, eine hohe ist nur hoch. Das eine will
niemand, das andere kann man wollen.

**`bausteine.css` bleibt bei beiden Befunden unangetastet**, obwohl beide
dorthin gehörten: die Datei kleidet acht Vorlagen plus jede hochgeladene, und
diese Reichweite ist eine andere Entscheidung als eine Zeile in einer Vorlage.

### Zum Stand der Prüftore

`template check` meldet für `weide` „no problems found"; `go build ./...`,
`go test ./internal/template/... ./internal/tmplmgr/...` und `go test ./...`
sind grün.

Zwischenzeitlich war `go test ./internal/template/...` rot, aber **nicht wegen
`weide`**: `TestShippedThemesRenderEveryView` meldete genau eine Zeile,
`rudel has no print stylesheet`. An `cmd/holzcloud/templates/public/rudel/`
arbeitete im selben Arbeitsbaum ein anderer Agent. Der Test meldet je Vorlage
einzeln über `t.Errorf`, `weide` kam in der Ausgabe nie vor. Beim letzten Lauf
war auch `rudel` wieder grün. `rudel/` habe ich nicht angefasst — auch nicht,
um den Test grün zu machen.

## Self-Check: PASSED

Alle drei neuen Dateien liegen auf der Platte (`favicon.svg`, beide woff2), alle
fünfzehn geänderten ebenfalls, und alle drei Commits sind in `git log`
auffindbar: 62d022c, 84f42dd, fef2554, 5df88df, a69c3c1, 0aef9ee, 1307967, c7297cf, ac0aeb5, f56f48b, a1d3447.
