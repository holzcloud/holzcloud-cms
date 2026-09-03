# Zurückgestellt — gefunden am 2026-09-03 während 260903-da5

## Flatternder Test: TestMarkeMitUnbekanntemArgumentBleibtDerBetreff

**Datei:** `internal/public/formular_e2e_test.go:414`
**Ausserhalb des Umfangs.** Die Datei steht nicht in `files_modified` dieses Plans und
wurde hier nicht angefasst (`git diff` über sie ist leer). Der Fehlschlag besteht schon
vorher; er hat mit dem Austausch der Fixture-Adressen nichts zu tun.

**Was passiert.** Die Zusicherung lautet

```go
if strings.Contains(seite, "f_") {
        t.Errorf("es wurde ein zusammengestelltes Formular gezeichnet:\n%s", seite)
}
```

Sie soll zeigen, dass kein zusammengesetztes Formular gezeichnet wurde, und sucht dafür
nach dem Präfix `f_`. Auf derselben Seite steht aber das Feld

```html
<input type="hidden" name="gestellt" value="1788421711.0-Rzf0WFcJQLOuSiX0wjIoUFY3f_HqBVz76r9QuwHio">
```

Der zufällige Teil dieser Zeitmarke ist base64url; sein Alphabet enthält `f` und `_`.
Enthält der Zufallsteil zufällig die Folge `f_`, schlägt der Test fehl, obwohl nichts
kaputt ist.

**Gemessen.** `go test ./internal/public/ -run TestMarkeMitUnbekanntemArgumentBleibtDerBetreff -count=400`
ergab 5 Fehlschläge, also rund 1,25 %. Das deckt sich mit der Rechnung: etwa 39
Positionen mal 1/64 mal 1/64 ≈ 0,95 %.

**Was zu tun wäre.** Nicht nach `f_` im ganzen Dokument suchen, sondern dort, wo die
Aussage gilt — etwa auf `name="f_` oder `id="f_` prüfen, oder das Feld `gestellt` vor
der Prüfung aus dem HTML schneiden. Eine eigene Aufgabe, kein Nachtrag zu dieser.
