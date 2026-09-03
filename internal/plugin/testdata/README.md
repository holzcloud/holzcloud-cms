# Das Prüf-Plugin

`echo.wasm` ist aus `echo/main.go` gebaut und liegt gebaut im Repository.

```sh
cd internal/plugin/testdata/echo
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o ../echo.wasm .
```

Es liegt gebaut hier, obwohl es drei Megabyte sind, weil ein Test, der erst
einen Compiler-Lauf braucht, irgendwann übersprungen wird — und ein
übersprungener Test ist einer, der nie wieder ausgeführt wird. Der Bau
bräuchte ausserdem die wasip1-Standardbibliothek, die auf einem frischen
Rechner erst geladen werden müsste.

Die drei Megabyte sind fast vollständig die Go-Laufzeit. Ein Plugin in TinyGo
wäre ein Zehntel so gross, verlangte dem Autor aber eine zweite Werkzeugkette
ab; deshalb ist Gos eigenes Ziel die Vorgabe.

Das Modul tut gerade genug, um die Aufrufkonvention zu beweisen: es hallt
wider, schreibt und liest im eigenen Speicher, greift nach einer Berechtigung,
die es nicht hat, gibt ungültiges JSON zurück und hängt sich auf Befehl auf.
