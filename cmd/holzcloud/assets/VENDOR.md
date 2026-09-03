# Vendored assets

Nothing in this directory may be fetched at runtime (see CLAUDE.md). Everything
a browser loads is served out of the embedded filesystem, so it has to be
committed to this repository. Most of it goes out under `/assets/` and lives
here; the two typefaces are the exception and live in the theme that uses them
— see `## Schriften` below for where and why.

## htmx.min.js

| | |
|---|---|
| Version | 2.0.10 |
| Source | `https://registry.npmjs.org/htmx.org/-/htmx.org-2.0.10.tgz`, path `package/dist/htmx.min.js` |
| Size | 51238 bytes |
| SHA-256 | `71ea67185bfa8c98c39d31717c6fce5d852370fcdfd129db4543774d3145c0de` |
| Lizenz | BSD-2-Clause (`Zero-Clause` für die Dokumentation), siehe das Paket |
| Tarball SHA-512 | `kdeJe7ZVwaS6QMz/ebBIVtZdpwen6L0OQ5GOhPV9MKBb196TCZeZu4yA7ZIQsaLKv7EpXz+So7KSXNuHXhj7Cw==` |

The tarball hash above is the `dist.integrity` value published in the npm
registry metadata — the same check `npm install` performs. It was verified
against the downloaded archive before the file was extracted.

This file was a 47-byte placeholder comment from the commit that created it
until it was vendored, which meant every `hx-*` attribute in the admin UI was
inert in a browser while looking correct in the source. The admin therefore does
not depend on it: every state-changing control carries a plain `method`/`action`
pair and htmx is progressive enhancement on top. `internal/web/nojs_test.go`
enforces that, so this file can be removed again without breaking anything.

### Updating

```bash
V=2.0.11   # target version
curl -sS "https://registry.npmjs.org/htmx.org/${V}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["dist"]["integrity"])'

curl -sSfL -o htmx.tgz "https://registry.npmjs.org/htmx.org/-/htmx.org-${V}.tgz"
openssl dgst -sha512 -binary htmx.tgz | openssl base64 -A   # must match the integrity value

tar xzf htmx.tgz package/dist/htmx.min.js
cp package/dist/htmx.min.js cmd/holzcloud/assets/htmx.min.js
sha256sum cmd/holzcloud/assets/htmx.min.js                  # record it in this file
```

Then update the table above and run `go test ./...`.

## Schriften

**Die vier Dateien liegen nicht in diesem Verzeichnis.** Sie gehören zur
Vorlage `holzcloud` und stehen in
`cmd/holzcloud/templates/public/holzcloud/fonts/`; verzeichnet sind sie hier,
weil hier steht, was von dritter Seite in dieses Repository gekommen ist.

Der Grund für den Ort: `/assets/` wird von `http.FileServer` bedient. Gos
eingebaute MIME-Tabelle kennt `.woff2` nicht, und eine Datei aus einer
`embed.FS` meldet einen ModTime von null — dieselben vier Dateien gingen von
dort also ohne Typ und ohne Zwischenspeicher-Kopfzeilen hinaus und würden bei
jedem Seitenwechsel neu geholt, 83 KB, auf einer schmalen Leitung. Über `/t/` geht
`internal/web/asset.go` und setzt `font/woff2`, `X-Content-Type-Options:
nosniff` und `Cache-Control: public, max-age=31536000, immutable`. Zudem
dokumentiert TEMPLATE-SPEC §2.1 genau diese Form für eine Vorlage mit eigener
Schrift, und die Vorlage bleibt damit ein gültiges Archiv, das sich auf jeder
Installation hochladen lässt.

Beide Familien sind variabel: eine Datei je Teilmenge trägt jedes Gewicht der
Achse. Deshalb steht in `style.css` ein `font-weight`-BEREICH und nicht ein
`@font-face` je Schnitt. Nur `latin` und `latin-ext` sind übernommen.

**Manrope liegt an ZWEI Orten**, byteweise identisch und mit denselben SHA-256
aus der Tabelle unten: in `cmd/holzcloud/templates/public/holzcloud/fonts/` und
in `cmd/holzcloud/templates/public/weide/fonts/`. Das ist Absicht und keine
vergessene Kopie. Eine Vorlage muss als Archiv für sich stehen: wer sie
herunterlädt und auf einer anderen Installation hochlädt, bekommt sonst eine
Vorlage, die ihre eigene Schrift nicht mitbringt — und `/t/fonts/…` zeigt immer
in die Vorlage, die gerade ausgeliefert wird, nie in eine andere. Ein Symlink
oder ein Verweis über Verzeichnisgrenzen hinweg wäre im Archiv genauso kaputt.
Wer die Dateien austauscht, muss BEIDE Orte anfassen; der Prüfbefehl dafür
steht unter *Updating*. JetBrains Mono bleibt bei `holzcloud` allein — die
Vorlage `weide` benutzt keine Schrift fester Breite.

### Manrope

| | |
|---|---|
| Version | variabel, `wght` 200–800 |
| Anfrage | `https://fonts.googleapis.com/css2?family=Manrope:wght@200..800&display=swap` |
| Teilmenge `latin` | `https://fonts.gstatic.com/s/manrope/v20/xn7gYHE41ni1AdIRggexSvfedN4.woff2` |
| Teilmenge `latin-ext` | `https://fonts.gstatic.com/s/manrope/v20/xn7gYHE41ni1AdIRggmxSvfedN62Zw.woff2` |
| `manrope-latin.woff2` | 24576 bytes, SHA-256 `e310b55a7fd9677f5e3555e6c6c4d064fa1f1d24393f0ddbe217cea12a8c432f` |
| `manrope-latin-ext.woff2` | 15240 bytes, SHA-256 `ce093b341d9c10658ee1eaa85c5f8042ff3307bc6ccfc5f405616eb437f0009e` |
| Lizenz | SIL Open Font License 1.1 |

### JetBrains Mono

| | |
|---|---|
| Version | variabel, `wght` 400–500 |
| Anfrage | `https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400..500&display=swap` |
| Teilmenge `latin` | `https://fonts.gstatic.com/s/jetbrainsmono/v24/tDbv2o-flEEny0FZhsfKu5WU4zr3E_BX0PnT8RD8yKwBNntkaToggR7BYRbKPxDcwgknk-4.woff2` |
| Teilmenge `latin-ext` | `https://fonts.gstatic.com/s/jetbrainsmono/v24/tDbv2o-flEEny0FZhsfKu5WU4zr3E_BX0PnT8RD8yKwBNntkaToggR7BYRbKPx7cwgknk-6nFg.woff2` |
| `jetbrains-mono-latin.woff2` | 31340 bytes, SHA-256 `2c32b9b3ee358c119e210f6f5195f9bd34894d78a785ff2e95d60e718e400af4` |
| `jetbrains-mono-latin-ext.woff2` | 11596 bytes, SHA-256 `9c38cb2d0d2d93c1ee6e21fa78db76f13ea7e15e15cc64214c7ca89b6aaa35c4` |
| Lizenz | SIL Open Font License 1.1 |

### Updating

Der User-Agent ist nicht optional. Ohne einen Chrome-artigen liefert
`fonts.googleapis.com` ein Stylesheet mit TTF-Verweisen statt woff2, und die
Dateien wären dreimal so gross.

```bash
UA='Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36'
T=cmd/holzcloud/templates/public/holzcloud/fonts
W=cmd/holzcloud/templates/public/weide/fonts   # zweiter Ort, nur Manrope

# 1. Das Stylesheet holen. Es enthält einen @font-face je Teilmenge; gebraucht
#    werden genau die beiden Kommentare /* latin */ und /* latin-ext */.
curl -sS -A "$UA" 'https://fonts.googleapis.com/css2?family=Manrope:wght@200..800&display=swap'      > manrope.css
curl -sS -A "$UA" 'https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400..500&display=swap' > jbmono.css

# 2. Die beiden URLs je Familie herausziehen und laden.
#    grep -A liefert den Block nach dem Kommentar der Teilmenge.
grep -A6 '/\* latin \*/'     manrope.css | grep -o 'https://[^)]*\.woff2'   # -> $T/manrope-latin.woff2
grep -A6 '/\* latin-ext \*/' manrope.css | grep -o 'https://[^)]*\.woff2'   # -> $T/manrope-latin-ext.woff2
grep -A6 '/\* latin \*/'     jbmono.css  | grep -o 'https://[^)]*\.woff2'   # -> $T/jetbrains-mono-latin.woff2
grep -A6 '/\* latin-ext \*/' jbmono.css  | grep -o 'https://[^)]*\.woff2'   # -> $T/jetbrains-mono-latin-ext.woff2

# 3. Manrope an den zweiten Ort kopieren. Wer das vergisst, lässt die
#    Vorlage `weide` auf der alten Fassung stehen — sie rendert weiter und
#    fällt nirgends auf, bis jemand die beiden Seiten nebeneinander sieht.
cp "$T"/manrope-latin.woff2 "$T"/manrope-latin-ext.woff2 "$W"/
cmp "$T"/manrope-latin.woff2     "$W"/manrope-latin.woff2
cmp "$T"/manrope-latin-ext.woff2 "$W"/manrope-latin-ext.woff2

# 4. Grösse und Prüfsumme in dieses Verzeichnis übertragen, und die
#    unicode-range aus dem Stylesheet in die @font-face-Blöcke von
#    templates/public/holzcloud/style.css UND templates/public/weide/style.css.
( cd "$T" && wc -c *.woff2 && shasum -a 256 *.woff2 )
```

Danach die beiden Tabellen oben nachführen und `go test ./...` laufen lassen.

## Lizenzen der Fremdbestandteile

htmx steht unter der BSD-2-Clause-Lizenz und ist damit mit der AGPL, unter der
Holzcloud CMS steht, verträglich. Manrope und JetBrains Mono stehen unter der
SIL Open Font License 1.1, die ebenfalls mit der AGPL verträglich ist: sie
bindet nur die Schriftdateien selbst und stellt an das Programm, das sie
ausliefert, keine Bedingung ausser der, dass sie nicht einzeln verkauft werden. Die Lizenztexte der Go-Module stehen im
Modul-Cache (`go mod download` und dann `go list -m -f '{{.Dir}}' all`); sie
werden hier nicht kopiert, weil sie nicht mit ausgeliefert werden — sie stecken
übersetzt im Programm, und ihre Bedingungen gelten unverändert weiter.
