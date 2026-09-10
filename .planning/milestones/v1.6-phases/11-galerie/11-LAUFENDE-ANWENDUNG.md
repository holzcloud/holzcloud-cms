---
phase: 11-galerie
kind: evidence
ran: 2026-09-08
binary: built from HEAD f8052a3, CGO_ENABLED=0
data: fresh data dir, migrations 0 → 50, two bundles imported
status: GAL-03 bestätigt, Lichtkasten bestätigt, kein JavaScript
---

# Phase 11 gegen die laufende Anwendung

Kein Testlauf, sondern ein Durchgang durch das echte Programm: eigenes
Verzeichnis, Migrationen 0 → 50, zwei Archive eingespielt, ein Konto über
`holzcloud user create`, Anmeldung samt Pflicht-TOTP, dann alles über HTTP wie
ein Browser es schickt.

**Warum das hier steht und nicht in `.playwright-mcp/`:** Bildschirmfotos sind
in dieser Ablage ausgenommen (`.gitignore:52`), und ein Tor, das auf Prosa über
Bilder ruht, die niemand nachprüfen kann, ist kein Tor. Was unten steht, kann
jeder nachfahren.

## Was aufgebaut wurde

    Album "Sommer 2026" (Kennung sommer-2026), drei Bilder mit Alt und Unterschrift
    Galerie-Baustein auf Seite 1, der das Album benennt — b9.album=sommer-2026
    Seite veröffentlicht, Theme weide, Host seehof.localhost

## 1. Der Baustein überlebt das Speichern, das ihn anlegt

Der Fund, den weder `11-CONTEXT.md` noch `11-PATTERNS.md` genannt hatten:
`Block.Empty()` hielt eine Galerie ohne Einträge für leer, und `Set.Clean`
warf jeden leeren Baustein vor dem Kodieren weg. Ein Album-Baustein trägt
keine Einträge.

Nach dem Speichern, das ihn anlegt, steht er noch da:

```
name="b9.album"
```

## 2. Die Bindung ist spät — im Speicher steht die Marke, nicht das Bild

```
marker in content_html: [[album:sommer-2026:9]]
wollschwein in content_html: False
```

Wären die Bilder beim Speichern eingesetzt worden, stünden sie hier. Sie
stehen nicht hier.

## 3. GAL-03, das Kriterium, das still falsch sein kann

> „Ändert man ein Album, ändert sich jede Seite, die es trägt — **ohne dass
> diese Seiten angefasst werden**."

Eine Bildunterschrift im Album geändert. Die Seite wurde nicht geöffnet, nicht
gespeichert, nicht angefasst.

```
page before: sha256 5d324394cea78b85   updated_at 2026-09-07T23:25:36Z
album edited: 303
page after:  sha256 5d324394cea78b85   updated_at 2026-09-07T23:25:36Z
  -> die Zeile der Seite ist Byte für Byte dieselbe

öffentliche Seite vorher:  <figcaption>Unterschrift 1
öffentliche Seite nachher: <figcaption>GEAENDERT OHNE DIE SEITE ANZUFASSEN
```

Beide Hälften, und die zweite ist die, auf die es ankommt: dieselbe Zeile in
der Datenbank, eine andere Seite im Netz.

**Ein Zwischenfall, der hierher gehört.** Der erste Versuch änderte nichts, und
die naheliegende Erklärung wäre „die öffentliche Seite wird zwischengespeichert"
gewesen — ein Befund, den ich beinahe aufgeschrieben hätte. In der Datenbank
nachgesehen: die Bildunterschrift war gar nicht geändert worden, weil mein
Formular `bild.medium` nicht mitschickte. Nicht das Programm hat sich falsch
verhalten, sondern mein Aufruf. Der Umweg über die Datenbank vor der Deutung
ist der Grund, dass hier kein erfundener Befund steht.

## 4. Der Lichtkasten ist `:target`, und es gibt kein JavaScript

```
<script>-Vorkommen auf der öffentlichen Seite:  1
   davon <script type="application/ld+json">:   1
onclick= / javascript: :                        0
<dialog> :                                      0
```

Der einzige `<script>` ist der Datenblock, den ein Browser nie ausführt und den
`internal/tmplmgr/script.go` ausdrücklich ausnimmt.

Die Vergrösserung:

```html
<a class="hc-galerie__oeffnen" href="#hc-b10-p1"><img …></a>
…
<figure class="hc-galerie__gross" id="hc-b10-p1" tabindex="-1">
```

Ein Anker auf eine Kennung — reines HTML, die Zurück-Taste des Browsers
funktioniert, und `href="#hc-zu"` schliesst. Kein `showModal()`.

## 5. Die Bilder gehen durch den Grössen-Durchgang

```html
<img src="/media/1/baechlein-01.jpg" alt="Bild 1 im Album"
     sizes="(min-width: 50em) 30vw, 90vw" width="1200" height="1600"
     loading="lazy" decoding="async"
     srcset="…-thumb.jpg 400w, …-medium.jpg 800w, …-01.jpg 1200w">
```

`srcset`, `sizes`, `width` und `height` sind da. Das ist die Reihenfolge, die
11-05 verlangt: die Erweiterung läuft **vor** `h.responsive`, sonst hätte
`media.MakeResponsive` diese `<img>` nie gesehen. Und `alt` kommt aus dem
Album, nicht aus der Mediathek — der Wert, den ich beim Anlegen eingetragen
habe, steht da.

## Was hier NICHT geprüft ist

- **Der ungestylte Fall.** D-01 verlangt, dass eine Galerieseite mit
  blockiertem `/assets/bausteine.css` eine lesbare Bilderliste mit
  funktionierenden Ankern bleibt. Aus dem Aufbau spricht nichts dagegen — die
  Grossansichten sind `<figure>`-Elemente in der Reihenfolge —, aber
  „spricht nichts dagegen" ist keine Messung. Gehört zu 11-07.
- **Die Diashow** und die Breite eines waagerechten Schiebefelds bei drei
  Fensterbreiten. Gehört zu 11-07, und das ist die Messung, die dort am
  ehesten etwas findet.
- **Die Verwaltungsbildschirme im Bild.** Dass sie im Grundgerüst stecken, ist
  belegt (die Alben-Seite trägt die vollständige Navigation), das Aussehen
  nicht.

Der förmliche Durchgang gehört 11-07 und läuft **nach** der Behebungsrunde —
Phase 7, 8 und 9 haben je nach einem abgezeichneten Durchgang noch sichtbare
Änderungen bekommen, was die Unterschrift wertlos macht.

## Nachfahren

```bash
go build -o /tmp/holzcloud ./cmd/holzcloud
HOLZCLOUD_DATA_DIR=/tmp/hcdata go run ./build/devseed \
    build/pakete/seehof-seewen.zip:weide:seehof.localhost
printf 'ein sicheres passwort' | HOLZCLOUD_DATA_DIR=/tmp/hcdata \
    /tmp/holzcloud user create -email admin@test.local -name Admin -role admin
HOLZCLOUD_DATA_DIR=/tmp/hcdata HOLZCLOUD_PORT=8099 HOLZCLOUD_SECURE=false /tmp/holzcloud
```
