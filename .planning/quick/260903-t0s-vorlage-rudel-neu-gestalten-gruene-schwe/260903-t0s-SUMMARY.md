---
phase: quick-260903-t0s
plan: 01
subsystem: templates/public
tags: [theme, css, design-system, a11y, print, hochformat]
status: complete

requires:
  - cmd/holzcloud/templates/public/weide/    # Referenz für Architektur und Tonfall
  - cmd/holzcloud/assets/bausteine.css       # die Brücke zu den neun Bausteinarten
  - internal/design/tokens.go                # die sechs Regler der Verwaltung
  - internal/block/render.go                 # wo die Bildmitte je Kachelbild entsteht
provides:
  - "Vorlage `rudel` in grüner Fassung: gebrochenes Weiss, Waldgrün, Manrope"
  - "zwölf Ansichten in der gemeinsamen Bauteil-Sprache (.hc-*)"
  - "Radienfamilie, die über calc() an einer eingestellten Rundung hängt"
  - "Regeln für die Übergangsform: handgeschriebene <section>-Gruppen und <aside>"
  - "Manrope als dritter, byteweise identischer Ort im Repository"
affects:
  - cmd/holzcloud/assets/VENDOR.md

tech-stack:
  added: []
  patterns:
    - "Drei Stufen der Kaskade: vier @layer < schichtlose CMS-Schicht < .Site.Design"
    - "Alle Farben als color-mix(in srgb, …) aus vier Entscheidungen abgeleitet"
    - "Brücke zu den Reglern: vier Zeilen, Richtung immer vom Regler zum System"
    - "Radien über calc() aus der mittleren abgeleitet statt drei feste Werte"
    - "Höhengrenze statt Zuschnitt, wo der Kern keine Bildmitte mitgibt"

key-files:
  created:
    - cmd/holzcloud/templates/public/rudel/favicon.svg
    - cmd/holzcloud/templates/public/rudel/fonts/manrope-latin.woff2
    - cmd/holzcloud/templates/public/rudel/fonts/manrope-latin-ext.woff2
  modified:
    - cmd/holzcloud/templates/public/rudel/style.css
    - cmd/holzcloud/templates/public/rudel/layout.html
    - cmd/holzcloud/templates/public/rudel/home.html
    - cmd/holzcloud/templates/public/rudel/page.html
    - cmd/holzcloud/templates/public/rudel/list.html
    - cmd/holzcloud/templates/public/rudel/search.html
    - cmd/holzcloud/templates/public/rudel/gate.html
    - cmd/holzcloud/templates/public/rudel/404.html
    - cmd/holzcloud/templates/public/rudel/maintenance.html
    - cmd/holzcloud/templates/public/rudel/shop.html
    - cmd/holzcloud/templates/public/rudel/product.html
    - cmd/holzcloud/templates/public/rudel/cart.html
    - cmd/holzcloud/templates/public/rudel/checkout.html
    - cmd/holzcloud/templates/public/rudel/order.html
    - cmd/holzcloud/assets/VENDOR.md

decisions:
  - "Entwurfswert der Rundung ist 10px, nicht die 16px des Plans — das Manifest dieser Website stellt 10 ein, und ein Entwurfswert weiter davon entfernt als nötig macht die Vorschau in der Verwaltung irreführend"
  - "Die Radienfamilie hängt über calc() an --hc-r: eine eingestellte Rundung zieht Karte, Bild und Eingabefeld im selben Verhältnis mit; bei 0 wird alles 0, nichts wird negativ"
  - "--hc-measure ist 66ch statt der 74ch aus weide: beide Websites, die eine der zwei Vorlagen tragen, stellen 66 ein"
  - "Die schwächste Tinte steht auf 64 % (4.80:1 auf #F7F6F0), auf diesem Papier selbst nachgerechnet — dieselbe Prozentzahl wie weide, aber aus eigener Rechnung"
  - "Aufmacherbild und Produkt-Hauptbild werden NICHT zugeschnitten: ohne Bildmitte aus dem Kern trifft ein Querformat-Zuschnitt bei einem Hundeporträt den Kopf"
  - "Galeriebild auf 1/1 und Kartenbild auf 4/3 statt der Querformate aus bausteine.css; der Zuschnitt bleibt, damit die Bildmitte aus render.go weiter wirkt"
  - "Überschriften: die Grösse gilt überall im Inhalt, der grosse Abstand nur auf der obersten Ebene — zwei Reichweiten mit Absicht, ausgenommen die zwei Bausteintitel"
  - "Breit ist, was von sich aus mehrspaltig ist oder ein Bild in natürlicher Grösse zeigt; schmal bleibt, was gelesen wird"
  - "Handgeschriebene <div><section>-Gruppen bekommen die Breite einer Kartenreihe, ein <aside> die eines aufrufs — die Breite darf beim späteren Umzug in Bausteine nicht springen"
  - "Höhengrenze min(52vh, 500px) für das Bild im bildtext: strenger als die 70vh beim vollbreiten Bild, weil es nur die halbe Breite hat"
  - "Ein Bild wird in seiner Grösse gezeigt, der Rahmen richtet sich danach: max-inline-size statt inline-size für die drei ungeschnittenen Bausteinarten, dazu die Höhengrenze — ein Gedanke, ein Kommentar"

metrics:
  duration: 52m
  completed: 2026-09-03
  tasks: 3
  commits: 5
  files: 18

actuals:
  tokens: 68000
  tasks: 3
  commits: 5
---

# Quick-Aufgabe 260903-t0s: Vorlage `rudel` neu gestalten — Summary

Die mitgelieferte öffentliche Vorlage `rudel` trägt jetzt dieselbe Architektur
wie die soeben abgenommene Schwestervorlage `weide`, ins Waldgrüne übersetzt:
gebrochenes Weiss mit kühlerem Zug statt warmem Creme, Waldgrün statt Erdbraun,
Manrope statt System-Serifen. Sie kleidet die Website der Hundezucht Delnahida,
und alles, was diese Website besonders macht — 85 Bilder, viele im Hochformat,
zwölf Tierporträts, sechs Menüpunkte, handgeschriebenes HTML im Markdown —,
steht als Begründung in den Kommentaren.

## Was gebaut wurde

**Das Gestaltungssystem.** `style.css` ist von 2343 auf 2093 Zeilen neu
geschrieben, in drei Stufen: vier `@layer rudel.*` (tokens, base, components,
motif), darunter die bewusst schichtlose CMS-Schicht, danach im `<style>` die
Werte der Website. In `rudel.tokens` wird genau viermal ein Farbwert
entschieden — Papier `#F7F6F0`, Tinte `#1B211A`, Marke `#325737` und die
Schrift auf der Marke. Alles andere leitet über `color-mix(in srgb, …)` ab,
also zieht eine eingestellte Marke die halbe Palette mit.

**Die Kontraststufe, auf diesem Papier gemessen.** Die Tinte gegen das Papier
misst 15.16:1. Davon 52 % ergeben 3.36:1 und fallen durch, 60 % ergeben 4.26:1
und fallen durch, 62 % ergeben 4.52:1 und liegen auf der Messerschneide; 64 %
ergeben 4.80:1 und sind der Wert, den die Datei festschreibt — auf einer Karte
4.86:1. `--hc-ink-2` bei 80 % ergibt 8.12:1, die Marke gegen Papier 7.59:1 und
gegen eine Karte 7.72:1, trägt also als Verweisfarbe und als Knopffüllung. Alle
vier Zustandsfarben stehen zwischen 5.46:1 und 6.62:1 gegen Papier. Dass die
64 % auch in `weide` stehen, ist das Ergebnis einer eigenen Rechnung; die Zahl
fällt zufällig gleich aus.

**Die Rundung ist der Ort, an dem sich `rudel` vom Entwurf unterscheidet.**
Der Plan sah 16px vor; das Manifest dieser Website stellt 10 ein (die
Milchschäferei 0). Der Entwurfswert ist deshalb 10px, und die Familie hängt
über `calc()` an der mittleren: `--hc-r-sm` auf 0.55, `--hc-r-lg` auf 1.6. Eine
eingestellte Rundung zieht damit Karte, Bild und Eingabefeld im selben
Verhältnis mit — in `weide` bewegt sie nur die mittlere. Bei 0 wird alles 0,
nichts wird negativ.

**Die Hülle.** Klebende Kopfleiste mit Marke, Menü, Sprachwahl, Warenkorb und
Suche; Untermenüs über `:hover` und `:focus-within`, keine Zeile JavaScript.
Unter 1000 px stehen beide Menüebenen ausdrücklich auf `flex-direction: column`
— die Regel, die `weide` einen Nachbesserungs-Commit gekostet hat, steht hier
im ersten Wurf. Sechs Hauptpunkte mit zwei Untermenüs wären ohne Umschalter
rund 320 px Leiste vor dem ersten Inhalt, also klappt das Menü hinter einer
Checkbox zu; oberhalb der Schwelle stehen Checkbox und Beschriftung auf
`display: none` und sind damit aus dem Tabulatorweg heraus.

**Der Textkörper.** Zwei Rasterspalten, Text auf `--hc-measure`, daneben eine
Spalte bis `breit-ende`. Ein Bild allein in seinem Absatz spannt breit und ist
auf 70vh begrenzt — auch das der zweite `weide`-Befund, hier im ersten Wurf.

**Die Übergangsform.** Diese Website trägt heute handgeschriebenes HTML im
Markdown: die Startseite baut ihre Kartenreihen als
`<div><section>…</section></div>` und ihren Aktuell-Kasten als `<aside>`. Dass
bluemonday `div`, `section` und `aside` unverändert durchlässt, ist mit einer
Wegwerf-Prüfung gegen `SanitizeHTML` bestätigt worden, nicht angenommen. Die
Gruppen werden über `:has()` als Kartenraster gesetzt, das `<aside>` als
hervorgehobener Kasten — jeweils mit der Breite des Bausteins, der später aus
ihnen wird, damit beim Umzug nichts springt. Ein Browser ohne `:has()` lässt
beide Regeln fallen und zeigt den Zustand von vorher.

**Kein Zuschnitt ohne Bildmitte, und kein Hochrechnen.** `bausteine.css` schneidet das Galeriebild auf 4/3 und das
Kartenbild auf 3/2, beides Querformat. Die Vorlage setzt 1/1 und 4/3: das
Quadrat kostet Hoch- und Querformat denselben Anteil. Der Zuschnitt selbst
bleibt, weil `render.go` genau für diese zwei Bausteinarten je Bild einen
Mittelpunkt als `style`-Attribut mitgibt — und die Vorlage setzt diese
Eigenschaft im Baustein-Abschnitt an keiner Stelle selbst, weil eine Regel im
Stylesheet ein `style`-Attribut nur mit `!important` schlagen könnte. Wo der
Kern keine Bildmitte mitgibt — Aufmacher, Bild-Baustein, Produkt-Hauptbild,
bildtext, handgeschriebene `<section>` —, wird nicht zugeschnitten, sondern die
Höhe begrenzt.

**Die zwölf Ansichten** sind alle erhalten und übersetzt. Ein Feldvergleich
gegen die alte Fassung zeigte, dass `weide` in allen zehn übrigen Ansichten
eine Obermenge ist; kein Feld ist verlorengegangen, drei sind dazugekommen
(`Gate.Path`, die Schlagwortliste am Fuss des Archivs, der Bestellstatus).
Ziel, Feldnamen und verstecktes Feld aller Formulare sind unverändert.

## Deviations from Plan

### Vier Befunde des Orchestrators, mitten in der Arbeit gemeldet

Alle vier betreffen Stellen, an denen die Referenz `weide` danebenliegt. Sie
sind hier **nicht** mitkopiert worden. `weide` ist getrennt nachgebessert
worden.

**1. Überschriften in Bausteinen blieben klein**
- **Gefunden bei:** Aufgabe 1, gemeldet vom Orchestrator
- **Problem:** `weide` hängt Grösse und Abstand an dieselbe Regel mit dem
  Kindselektor. Eine Überschrift in `.hc-text`, in `.hc-bildtext__text` oder in
  einer handgeschriebenen `<section>` ist kein direktes Kind von
  `.page-content` und blieb auf der Grösse des Fliesstextes — gemessen 26 px
  im Baustein gegen 37 px auf einer Markdown-Seite.
- **Fix:** Zwei Regeln statt einer. Die GRÖSSE über den Nachfahrenselektor,
  also überall; der grosse Abstand DAVOR weiterhin nur auf der obersten Ebene,
  wo er einen Abschnitt der Seite eröffnet. Ausgenommen sind die zwei
  Überschriften, die einem Baustein als dessen Titel gehören
  (`.hc-aufruf__titel`, `.hc-karte__titel`, render.go:185 und :225) — ein
  Kartentitel ist kein Abschnittstitel.
- **Commit:** 7de5994

**2. `.hc-karten` und `.hc-bildtext` fehlten in der Breit-Regel**
- **Gefunden bei:** Aufgabe 1 und 2, gemeldet vom Orchestrator
- **Problem:** Am laufenden Server bei 1280 px gemessen: Raster 1178 px,
  Textspalte 705 px. Eine Kartenreihe blieb in der Textspalte und brach auf
  2+1 um; einem bildtext blieben 340 px fürs Bild und rund 30 Zeichen je Zeile
  daneben — die Auszeichnung verspricht zwei Spalten, das Raster gab ihr eine
  halbe.
- **Fix:** Beide in die Aufzählung aufgenommen. Der Kommentar steht als REGEL
  und nicht als Liste, damit ein künftiger Bausteintyp nicht wieder geraten
  werden muss: *breit ist, was von sich aus mehrspaltig ist oder ein Bild in
  seiner natürlichen Grösse zeigt — schmal bleibt, was gelesen wird.* Danach
  ist `.hc-aufruf` schmal, obwohl er eine Fläche ist, und `.hc-bildtext` breit,
  obwohl er Text enthält.
- **Commits:** 7de5994, 377b27d

**3. Ein Hochformat sprengte den bildtext-Baustein**
- **Gefunden bei:** Aufgabe 3, gemeldet vom Orchestrator
- **Problem:** An `weide` mit echtem Inhalt gemessen, Spalte 533 px: ein
  Hochformat 900×1600 wurde 533×945 gezeigt — höher als das Fenster. Der Text
  daneben war 143 px hoch und stand wegen `align-items: center` 405 px unter
  der Bildoberkante. Die Querformate derselben Seite sassen richtig.
- **Fix:** `max-block-size: min(52vh, 500px)`, Breite folgt. Kein Zuschnitt,
  weil der Kern dem bildtext keine Bildmitte mitgibt und ein `cover` blind aus
  der geometrischen Mitte schnitte — bei einem Hundeporträt nicht der Kopf.
  Strenger als die 70vh beim vollbreiten Bild, weil dieses nur die halbe Breite
  hat. Nachgerechnet bei 570 px Spalte: 900×1600 wird von 570×1013 auf 281×500
  begrenzt, 1600×1200 und 1600×900 bleiben unberührt, ein Quadrat verliert
  12 % Breite. Dieselbe Grenze für Bilder in handgeschriebenen `<section>`.
- **Commit:** f748711

**4. Ein Bild, das schmaler ist als seine Spalte, wurde hochgerechnet**
- **Gefunden bei:** Aufgabe 3, gemeldet vom Orchestrator
- **Problem:** `bausteine.css` setzt für `.hc-bild` (Z. 50),
  `.hc-bildtext__bild` (Z. 96) und `.hc-eigen__bild` (Z. 299)
  `inline-size: 100%` und überschreibt damit sein eigenes, richtiges
  `max-inline-size: 100%` aus Zeile 34. Keine der drei Arten wird
  zugeschnitten, also zieht die Zeile ein kleines Bild auf volle
  Spaltenbreite — eine 235 px breite Bildmarke wurde in einer 569-px-Spalte
  2,4-fach hochgerechnet und franste sichtbar aus. Bei Karte und Galerie ist
  `inline-size: 100%` dagegen richtig: dort wird in einen festen Rahmen
  zugeschnitten, und der muss gefüllt sein.
- **Fix:** `inline-size: auto` plus `max-inline-size: 100%` für alle drei, auf
  der Theme-Ebene. `bausteine.css` ist **nicht** angefasst worden — sie gehört
  dem Kern und kleidet alle acht mitgelieferten Vorlagen; ihr Kommentarkopf
  hält ausdrücklich fest, dass ein Theme alles davon überschreiben darf.
  Höhengrenze und Hochrechnen stehen jetzt als EIN Kommentar unter einem Satz:
  ein Bild wird in seiner Grösse gezeigt, der Rahmen richtet sich danach. Zwei
  getrennte Begründungen an derselben Stelle laden dazu ein, später eine davon
  zu entfernen.
- **Commit:** d9e2b4c

### Eigene Abweichungen vom Plan

**`.hc-eigen__bild` ist in den vierten Befund aufgenommen worden**, obwohl der
Orchestrator nur `.hc-bild` und `.hc-bildtext__bild` nannte. Ein Blick in
`bausteine.css` Zeile 297..302 zeigt denselben Fall: `inline-size: 100%` ohne
Zuschnitt. Eine Regel, die für zwei von drei gleichartigen Stellen gilt, ist
keine Regel.

**`--hc-r` steht auf 10px statt der 16px des Plans.** Das Manifest der Website
(im ausgezogenen Kundenrepository, vom Orchestrator gelesen) setzt
`"radius": 10`. Ein Entwurfswert, der weiter von der einzigen bekannten
Verwendung entfernt liegt als nötig, macht die Vorschau in der Verwaltung
irreführend. Die `calc()`-Familie des Plans bleibt und ist auf Vorzeichen
geprüft: bei 0 wird alles 0.

**`--hc-measure` steht auf 66ch statt der 74ch aus `weide`.** Beide Websites,
die eine der zwei Vorlagen tragen, stellen 66 ein; dieselbe Begründung wie bei
der Rundung, und 66 Zeichen liegen ohnehin in der Mitte dessen, was als lesbare
Zeile gilt.

**Das Aufmacherbild wird nicht zugeschnitten** — das sah der Plan schon so vor.
Dieselbe Überlegung ist auf das **Produkt-Hauptbild** übertragen worden, das
der Plan nicht nennt: auch dort ein einzelnes Bild ohne Angabe zur Bildmitte.

**Das Vorschaubild des Archivs, das Produktkartenbild, das Galeriebild des
Produkts und das Warenkorbbild bekommen eine feste Bildmitte im oberen
Drittel.** Der Plan nennt nur das Archivbild. Alle vier sind derselbe Fall:
blosser URL ohne Bildmitte, aber zugeschnitten, damit die Reihe eine Reihe
bleibt. Auf einem Hundefoto steht der Kopf fast immer oben. Alle vier liegen
ausserhalb des Baustein-Abschnitts, das Prüftor bleibt bei null Treffern.

**Ein Feld vom Typ „bild" in `.page-fields` bekam eine Höhengrenze von 50vh.**
Der Plan nennt sie nicht; die zwölf Tierporträts tragen ihre Angaben in dieser
Liste, und ein Ahnentafel- oder Wurfbild im Hochformat hätte sie sonst
gesprengt.

**Das Anheben der Karte beim Überfahren hängt an
`prefers-reduced-motion: no-preference`.** In `weide` steht die Bewegung
unbedingt; wer weniger Bewegung eingestellt hat, bekommt hier Kante und
Schatten, aber keinen Sprung.

**Der Druck-Abschnitt kennt die Übergangsform.** `<section>`-Kacheln und das
`<aside>` verlieren auf Papier Schatten und bekommen dieselbe graue Kante wie
eine Karte.

## Threat Flags

Keine. Die Arbeit fügt keine Netzwerkschnittstelle, keinen
Authentifizierungspfad und keine Schemaänderung hinzu.

Die vier zugesagten Minderungen sind erfüllt:
- **T-t0s-01** — `@font-face` zeigt auf `/t/fonts/…`; kein Byte von einem
  fremden Ursprung. Bestätigt von `CheckNoExternalRefs` über `template check`
  und von `TestBuiltinTemplatesHaveNoExternalRefs`.
- **T-t0s-02** — kein `.js`, kein `<script>` ausser dem
  `application/ld+json`-Datenblock, kein `on*`, keine `javascript:`-Adresse.
  Bestätigt von `CheckNoScripts`.
- **T-t0s-03** — beide woff2 sind in jeder der drei Aufgaben mit `cmp` gegen
  die Kopien in `weide` geprüft worden; ihre SHA-256 stimmen mit den in
  VENDOR.md verzeichneten Werten überein
  (`e310b55a…` und `ce093b34…`).
- **T-t0s-07** — der Baustein-Abschnitt setzt die Eigenschaft, die die
  Bildmitte trägt, an keiner Stelle selbst; das Prüftor liest ihn
  kommentarbereinigt und meldet null Treffer. Der Zuschnitt in Galerie und
  Karte bleibt erhalten, die Angabe des Kerns wirkt also weiter.

`T-t0s-04`, `T-t0s-05` und `T-t0s-06` sind wie geplant akzeptiert; an
`internal/design` und `internal/public` hat diese Arbeit nichts geändert.

## Known Stubs

Keine. Kein hartkodierter Leerwert, kein „coming soon", kein TODO in den
achtzehn berührten Dateien.

## Prüfungen

| Prüfung | Ergebnis |
|---|---|
| `holzcloud template check …/rudel` | no problems found (zwölf Ansichten gegen SampleData UND MinimalData) |
| `go build ./...` | grün |
| `go test ./internal/template/... ./internal/tmplmgr/...` | grün |
| `go test ./...` | grün, 39 Pakete |
| `go run ./tools/i18n` | en/es/fr/it je 1128 übersetzt, 0 offen, 0 verwaist |
| Kaskade | genau vier `@layer` am Zeilenanfang, alle vor der CMS-Markierung |
| Regler | die Vorlage setzt keinen der vier gebrückten Namen (Zähler 0) |
| Bildmitte | Baustein-Abschnitt kommentarbereinigt: null `object-position` |
| Schriften | beide woff2 byteweise identisch mit den Kopien in `weide` |
| Klammern in style.css | 431 offen, 431 zu |
| `bausteine.css` | unangetastet — `git status` zeigt sie nicht |
| dreizehn HTML-Dateien | zwölf Ansichten plus `layout.html` |
| Go-Code | `git status` zeigt nichts unter `internal/` oder `cmd/holzcloud/*.go` |

**Medienabfragen im Kopf geprüft**, wie verlangt:
- **1280 px** — Menü als Zeile, Aufmacher zweispaltig, Textspalte 66ch mit
  breiter zweiter Spalte daneben.
- **900 px** — unter 1000: Menü als Spalte hinter dem Umschalter, Aufmacher
  einspaltig; unter/bei 900: Produkt und Kasse stapeln, was klebte, steht still.
- **390 px** — Menü als Spalte, Suche aus der Leiste, klappt aber mit dem Menü
  auf; Warenkorbzeile auf 72 px Bild, Feldliste und Faktenzeilen einspaltig,
  Archiveintrag einspaltig mit 16/9-Vorschau.

**Nicht automatisiert und darum hinterher zu tun:** der Blick im Browser gegen
die echte Website — Startseite mit den handgeschriebenen `<section>`-Gruppen,
ein Tierporträt mit Feldliste, eine Galerie mit hochformatigen Bildern, das
schmale Fenster mit sechs Menüpunkten. Genau dieser Blick fand bei `holzcloud`
und bei `weide` je einen Fehler, den keine der obigen Prüfungen sah.

## Self-Check: PASSED

Alle drei neuen Dateien liegen auf der Platte (`favicon.svg`, beide woff2),
alle fünfzehn geänderten ebenfalls, und alle vier Commits sind in `git log`
auffindbar: 7de5994, 377b27d, 154d5de, f748711, d9e2b4c.
