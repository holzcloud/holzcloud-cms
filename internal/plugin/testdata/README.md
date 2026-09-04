# Das Prüf-Plugin

`echo.wasm` ist aus `echo/main.go` gebaut und liegt gebaut im Repository.

```sh
cd internal/plugin/testdata/echo
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -buildvcs=false -ldflags="-s -w" -o ../echo.wasm .
```

`go run ./tools/wasm echo` baut dasselbe Modul mit der festgelegten Go-Fassung
und legt es an dieselbe Stelle — der Weg, der ohne Nachdenken die Bytes ergibt,
die die Prüfstrecke erwartet. `-buildvcs=false` ist dabei Bedingung und nicht
Geschmack: ohne die Angabe trägt das Modul den Git-Stand des Augenblicks, und
ein Bau an einem anderen Commit ergibt andere Bytes.

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
