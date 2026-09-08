---
phase: 11-galerie
kind: evidence
subject: CR-01
ran: 2026-09-08
tree: 5f8fb4e, in einem eigenen Arbeitsbaum, damit die laufenden Agenten nichts merken
---

# CR-01 nachgeprüft, weil der Beweis nicht mitgeliefert wurde

Der Fixer wurde vom Nutzungslimit abgebrochen, nachdem er alle vier kritischen
Befunde behoben hatte und **bevor** er den Bericht schrieb. Die Mutationsproben,
auf denen die Behebungskultur dieses Projekts ruht, sind damit für CR-01 nicht
festgehalten worden. Hier sind sie, unabhängig gefahren.

## Die Behebung ist besser als der Vorschlag

Der Durchgang schlug vor:

```go
if match := r.Header.Get("If-None-Match"); match != "" {
    if match == etag { … }
}
```

Umgesetzt wurde stattdessen `web.MatchesETag`, und die Begründung steht im
Kommentar: **`If-None-Match` ist eine Liste und kann `*` sein.** Ein
Gleichheitsvergleich hätte einen Browser, der zwei Marken schickt, wie einen
ohne Marke behandelt. Der Vorschlag war zu einfach; die Behebung nicht.

## Zwei Proben, zwei verschiedene Zusagen

**A — der Durchfall von einer nicht passenden Marke auf das Datum wird
wiederhergestellt:**

```
cache_test.go:134: 304 for a request whose If-None-Match did not match —
    If-Modified-Since decided a negotiation RFC 7232 §3.3 says it may not
    take part in
--- FAIL: TestAMismatchingIfNoneMatchEndsTheNegotiation
cache_test.go:104: 304: the visitor keeps the old gallery although the album
    changed — GAL-03 is false on the cached path
--- FAIL: TestAWarmBrowserIsNotKeptOnAnOldAlbum
```

**B — `contentModTime` vergisst die Alben wieder:**

```
cache_test.go:170: Last-Modified stayed at "Tue, 08 Sep 2026 03:54:59 GMT"
    although the album changed — the validator does not see the albums, and a
    cache that sends only a date keeps the old gallery
--- FAIL: TestChangingAnAlbumMovesThePagesLastModified
```

Beide wiederhergestellt, danach `go test ./internal/public/` grün.

## Was Probe A nebenbei zeigt, und der Durchgang hat es nicht benannt

Unter Mutation A wird **auch** `TestAWarmBrowserIsNotKeptOnAnOldAlbum` rot —
obwohl `contentModTime` die Alben zu diesem Zeitpunkt kennt. Das ist kein
Zufall und keine Überdeckung:

**Ein HTTP-Datum hat Sekundenauflösung.** Ändert jemand ein Album in derselben
Sekunde, in der die Seite zuletzt gespeichert wurde — was in einem Test die
Regel und im Betrieb der schlimmste Fall ist —, dann ist der neue Zeitstempel
nicht *nach* dem alten, und `If-Modified-Since` kann die Änderung
grundsätzlich nicht sehen. Die Marke sieht sie, weil sie über den Inhalt
gebildet wird.

Die beiden Hälften der Behebung sind also **nicht redundant, sondern für
verschiedene Fälle zuständig**:

| Fall | Was ihn auffängt |
|---|---|
| Änderung in derselben Sekunde | nur die Marke |
| Browser schickt beides | die Marke, jetzt weil sie das Datum abschneidet |
| Zwischenspeicher schickt nur ein Datum | nur `contentModTime` |

Wer eine der beiden für überflüssig hält, hat die Zeile in der Tabelle
übersehen, die nur die andere hält.
