---
phase: 11-galerie
kind: evidence
subject: CR-01
ran: 2026-09-08
tree: 5f8fb4e, in einem eigenen Arbeitsbaum, damit die laufenden Agenten nichts merken
---

# Die vier kritischen Befunde nachgeprüft, weil der Beweis nicht mitgeliefert wurde

Der Fixer wurde vom Nutzungslimit abgebrochen, nachdem er alle vier kritischen
Befunde behoben hatte und **bevor** er den Bericht schrieb. Die Mutationsproben,
auf denen die Behebungskultur dieses Projekts ruht, sind damit für CR-01 nicht
festgehalten worden. Hier sind sie, unabhängig gefahren — für alle vier.

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


---

# CR-02: zwei Alben mit einem Namen

Der Wächter ist die Namensprüfung in `album.Store.Rename`. Neun Zeilen
entfernt:

```
album_collision_test.go:50: Rename produced a second album called
    "Werkstatt 2025" — an archive of this website can no longer say which of
    them a gallery means
--- FAIL: TestRenameRefusesANameAnotherAlbumAlreadyHas
```

**Der Kommentar über der Behebung ist der Grund, dass ich sie erwähne.** Er
zitiert seine eigene frühere Fassung, die genau das Gegenteil behauptete:

> „Die Umbenennung kann nicht kollidieren, weil der Zwang auf dem Slug liegt
> und der Slug sich nicht bewegt. Zwei Alben können darum denselben sichtbaren
> Namen bekommen und verschiedene Adressen haben, was ein Betreiber sehen und
> rückgängig machen kann."

Und dann, warum das falsch war: **der Betreiber ist nicht der einzige Leser
eines Namens.** Ein Archiv trägt ein Album über seinen Namen und über sonst
nichts. Zwei Alben mit einem Namen schreiben zwei ununterscheidbare Einträge,
auf der anderen Seite entsteht eines davon nicht, seine Bilder sind weg, und
jede Galerie, die darauf zeigte, bindet an das überlebende — eine Albenmenge
Fotografien durch die von jemand anderem ersetzt, mit einer Warnung, die eine
Nummer in einer Liste nennt und nie eine Seite.

Ein Satz, der stimmte, und ein Schluss daraus, der nicht stimmte.

# CR-03: die Marke im Feed

`expandForFeed` erweitert wieder nur die Textbausteine, so wie vorher:

```
feed_album_test.go:64: the feed carries the raw marker [[album:moebel:0]];
    every subscriber's reader shows it, and it leaks the album's address
feed_album_test.go:71: the feed does not carry the album's pictures either —
    the marker was dropped rather than expanded
--- FAIL: TestTheFeedDoesNotShipTheRawAlbumMarker
--- FAIL: TestTheFeedSurvivesAnUnwiredAlbumStore
```

Die dritte Zeile ist die, die ein reiner Vorhandenseinstest übersehen hätte:
es reicht nicht, dass die Marke weg ist — die Bilder müssen da sein.

# CR-04: der Baustein, den das nächste Speichern löscht

`albumChoicesFor` gibt wieder nur zurück, was der Speicher aufgelistet hat:

```
block_album_survival_test.go:77: the block's own album has no selected option,
    so the browser will submit the first one (value="") and the next save
    deletes the gallery
block_album_survival_test.go:124: with no albums on the website the select was
    not drawn, so nothing posts b0.album and the next save deletes the gallery
block_album_survival_test.go:178: a gallery naming an unknown album got 2
    options, want 3
--- FAIL: TestAGalleryKeepsItsAlbumWhenTheAlbumIsGone
--- FAIL: TestAGalleryKeepsItsAlbumWhenThereIsNoListAtAll
--- FAIL: TestAlbumChoicesForLeavesAKnownAlbumAlone
```

Die Fehlertexte schreiben die ganze Kette hin: keine Auswahl gezeichnet →
nichts schickt `bN.album` → `Empty()` hält die Galerie für leer → `Clean` wirft
den Baustein weg, mit einer Erfolgsmeldung darüber.

Und `TestAGalleryWithNoAlbumStillGetsNoSelect` bleibt unter derselben Mutation
grün — die Behebung hat die Auswahl nicht einfach immer gezeichnet.

---

## Alle vier, nach der Wiederherstellung

```
go build ./...   sauber
go test ./...    0 Fehlschläge
```

Jede Mutation wurde einzeln gesetzt und einzeln zurückgenommen, in einem
eigenen Arbeitsbaum auf `5f8fb4e`, damit die beiden laufenden Agenten nichts
davon merken.

---

## Nachtrag: meine CR-02-Probe war richtig und unvollständig

Der Browserdurchgang von 11-07 hat am selben Tag gefunden, was diese
Nachprüfung nicht gesucht hat (`a3a5fe8`):

> Album umbenennen, dann ein zweites unter dem alten Namen anlegen — die Liste
> zeigt den Namen zweimal.

Meine Probe hat gefragt: **hält der Wächter?** Antwort ja — `Rename` weist eine
Namenskollision ab, und ohne die neun Zeilen wird der Test rot. Das stimmt
weiterhin.

Sie hat nicht gefragt: **reicht der Wächter?** Und da lautet die Antwort nein,
denn CR-02s Behebung stützte sich auf einen Satz, der ab der ersten Umbenennung
nicht mehr gilt:

> „Create refuses a duplicate through the UNIQUE constraint on the slug."

Die Adresse bewegt sich beim Umbenennen **absichtlich nicht** — das ist GAL-04,
und es ist der Grund, aus dem ein Album überhaupt umbenannt werden darf, ohne
jede Seite zu verlieren, die es trägt. Also ist der **alte** Name danach unter
einer anderen Adresse wieder frei, und das `INSERT` läuft an der Bedingung
vorbei, auf die sich die Behebung verliess. Zwei Alben, ein Name, genau der
Zustand, den `album_collision_test.go` verbietet — über die andere Tür.

**Das ist die Lehre und nicht die Fussnote.** Eine Mutationsprobe misst, ob ein
Wächter trägt. Sie kann nicht messen, ob er an der richtigen Stelle steht, und
sie stellt die Frage gar nicht, ob es eine zweite Tür gibt. Dafür braucht es
jemanden, der die Anwendung benutzt — hier: umbenennen und dann anlegen, was
kein Test tat, weil kein Test auf die Idee kam.

Die Projektgeschichte sagt genau das über sich selbst (`docs/offene-punkte.md`):
die Fehler, die dieses Projekt ausgeliefert hat, fand ein Browserdurchgang und
nie die Testreihe. Hier hat er einen Fehler in der **Behebung eines kritischen
Befunds** gefunden, zwei Stunden nachdem sie abgesegnet war.
