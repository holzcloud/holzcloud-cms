---
phase: 11-galerie
review_path: .planning/phases/11-galerie/11-REVIEW.md
fixed: 2026-09-08
fix_scope: critical+warning
iteration: 1
status: complete
findings:
  addressed: 12
  remaining: 5
  out_of_scope: 5
evidence:
  - .planning/phases/11-galerie/11-KRITISCH-NACHPRUEFUNG.md
---

# Phase 11 — was aus dem Code-Review behoben wurde

Zwei Läufe. Der erste behob die vier kritischen Befunde und WR-01 und wurde vom
Nutzungslimit abgebrochen, **bevor** er den Bericht schrieb. Der zweite — dieser
— behob WR-02 bis WR-07 und trägt beide Hälften hier zusammen.

**Alle siebzehn Befunde sind angefasst.** Elf behoben, einer war beim Nachsehen
schon durch eine andere Behebung geschlossen, fünf sind Hinweise ausserhalb des
Umfangs und stehen unten mit dem Grund.

## Wo gearbeitet wurde, und warum nicht im eigenen Arbeitsbaum

Im Hauptbaum, **jeder Commit Datei für Datei gestaget**, jeder mit
`git show --stat` nachgesehen. Ein zweiter Agent führte im selben Baum Phase 10
Welle 3 aus (`internal/admin/forwardauth.go`, `internal/auth/session.go`,
`cmd/holzcloud/main.go`, `cmd/holzcloud/main_test.go`); vier seiner Commits
liegen zwischen meinen. Kein `git add -A`, kein `git add .`, kein `git stash`,
kein `git clean`. Keine seiner Dateien steht in einem meiner sechs Commits.

Ein eigener Arbeitsbaum wäre die übliche Absonderung gewesen und war hier die
schlechtere: er hätte am Ende auf einen Zweig zurückgeführt werden müssen, der
sich währenddessen um vier fremde Commits weiterbewegt hat, und ein
`--ff-only`, das dann scheitert, lässt die Arbeit auf einem Nebenzweig liegen.

## Behoben

### Die vier kritischen Befunde (erster Lauf, hier nachgetragen)

Die Beschreibungen sind aus `git show` rekonstruiert. **Ihre Mutationsproben
sind zum Zeitpunkt der Behebung nicht festgehalten worden** — die Commit-Texte
nennen sie, der Bericht, in dem sie gestanden hätten, wurde nie geschrieben.
Sie wurden inzwischen unabhängig nachgefahren, zweimal: von einem eigenen
Durchgang auf `5f8fb4e`
(`.planning/phases/11-galerie/11-KRITISCH-NACHPRUEFUNG.md`, alle vier) und von
diesem Lauf heute auf dem fertigen Baum (CR-01 beide Hälften, CR-03, CR-04;
CR-02 fiel bei der Prüfung von WR-04 mit ab). Was unten als *heute
nachgeprüft* steht, ist von heute; nichts davon ist als Beweis von damals
ausgegeben.

| Befund | Commit | Was geändert wurde |
|---|---|---|
| **CR-01** Ein warmer Browser behält die alte Galerie für immer | `2e02be3` | Zwei unabhängige Fehler auf dem zwischengespeicherten Weg. (1) `serveCached` fiel bei **nicht passender** `If-None-Match` auf `If-Modified-Since` durch — RFC 7232 §3.3 sagt, die Marke entscheidet allein. Vergleich jetzt `web.MatchesETag`, also mit Liste und `*`, was der Vorschlag des Reviews (`match == etag`) nicht gehabt hätte. (2) `contentModTime` sah die Alben nicht. **Migration `00051_album_updated_at.sql`** gibt `albums` und `album_items` ein `updated_at` (00050 ist veröffentlicht und wird nicht angefasst); `internal/album` setzt den Stempel des **Elternalbums** aus jeder ändernden Methode heraus, in deren eigener Transaktion; `album.Set.Latest` trägt ihn zu `contentModTime`. Der Elternstempel und kein `MAX` über die Kinder, weil eine entfernte Zeile ihren Zeitstempel mitnimmt. Neu: `internal/album/stamp_test.go`, `internal/public/cache_test.go`. |
| **CR-02** Zwei Alben mit einem Namen fallen beim Archiv-Umlauf zusammen | `ca85c63` | Drei Änderungen in drei Tiefen. `album.Store.Rename` verweigert einen Namen, den ein anderes Album derselben Website schon trägt (`ErrDuplicateName`, den `albumSaid` längst in einen Satz übersetzt), Lesen und Schreiben in einer Transaktion auf dem Schreibvorrat. `importAlbums` gibt zurück, welchen Slug jedes Album **wirklich bekommen hat**, beim Speicher erfragt statt ein zweites Mal abgeleitet, und lässt einen doppelt genannten Namen weg. `missingAlbum` misst gegen dieselbe Karte, meldet also, was **angelegt** wurde, statt was **behauptet** war. Ein mehrdeutiger Name bindet nichts, statt an das zuerst Gekommene zu binden. Neu: `internal/bundle/album_collision_test.go`. |
| **CR-03** Der Atom-Feed liefert die rohe `[[album:…]]`-Marke aus | `373675c` | `expandForFeed` erweitert jetzt Textbausteine **und** Alben; die Alben des ganzen Feeds werden einmal geladen, aus der Verkettung der bereits erweiterten Rümpfe, weil ein Textbaustein selbst eine Galeriemarke tragen kann. `h.albumsFor` ist der nichtfehlschlagende, gegen `nil` sichere Lader, den beide Aufrufer wollen. Ausdrücklich nicht behoben und im Kommentar gesagt: der Feed läuft weiterhin ohne Plugin-Filter und ohne Grössen-Durchgang. Neu: `internal/public/feed_album_test.go`. |
| **CR-04** Eine an ein Album gebundene Galerie löscht das nächste Speichern | `fb32335` | `albumChoicesFor` beantwortet beide Türen an einer Stelle: die Liste, wie sie ist, plus — wenn der Baustein ein Album nennt, das nicht darin steht — eine Möglichkeit, die für dieses Album steht, als *fehlend* gekennzeichnet und darum ausgewählt. Damit ist die Liste im Fall „keine Alben mehr" auch nicht mehr leer, die Auswahl wird also genau dann gezeichnet, wenn es einen Wert zu tragen gibt. Ein verstecktes Feld wurde erwogen und verworfen: ein zweiter Träger für einen Wert ist ein zweites Ding, das man mitführen muss. Und `siteAlbums` macht aus einem Lesefehler keine leere Liste mehr — `newPageFormData` gibt jetzt einen Fehler zurück, durch alle fünf Aufrufer. Neu: `internal/admin/block_album_survival_test.go`. |

### Die sieben Warnungen

| Befund | Commit | Was geändert wurde |
|---|---|---|
| **WR-01** Sechs Sätze der Alben-Verwaltung sind für `tools/i18n` unsichtbar | `6efb3ba` | `albumSaid` und `requireOwnPicture` geben ihre Sätze aus einem Helfer zurück, der Sammler sah an der Aufrufstelle eine Variable. Jetzt `i18n.N` an der Stelle, wo der Satz **geschrieben** steht — so wie `internal/block/render.go` es tut —, ohne `web.T` an der Aufrufstelle, weil `SetFlashError` schon übersetzt. `tools/i18n` ging von 35 auf 41 offen, also genau die sechs. `TestNoAlbumSentenceIsReturnedUnmarked` hält die **Form** und keine Liste: es liest `album.go` und verweigert jedes nackt zurückgegebene Literal, ein siebter Satz fällt also nächstes Jahr hier durch statt auszuliefern. Übersetzt wird nicht hier — Plan 11-07. |
| **WR-02** Ein fehlgeschlagener Speicher liefert die rohe Marke statt gar nichts | `b96becb` | **Das Review hat recht, und die alte Begründung war die richtige Regel falsch herum gedreht.** Sie lautete „ein fehlschlagendes Album muss seinen eigenen Baustein kosten und nie die Seite". Die Marke stehenzulassen kostet die **Seite**: ein Klammerzeichen mitten im Artikel, auf jeder Galerie der Website, bei jedem Aufruf, ohne Erholung, plus die interne Adresse des Albums. Zu nichts erweitern kostet den **Baustein**: eine Galerie fehlt, der Artikel drumherum liest sich. Der Zwilling in derselben Datei macht seit jeher das andere: `loadSnippets` meldet den Fehler und gibt eine **leere Karte** zurück, und `snippet.Expand` läuft trotzdem. Auch die Analogie zu `h.responsive`, die der Kommentar anbot, trägt nicht: ein Rumpf ohne Grössen-Durchgang ist eine lesbare Seite ohne `srcset`, ein Rumpf ohne diese Erweiterung ist keine. `albumSet` ist in `albumsFor` aufgegangen — den Lader, den CR-03 für den Feed schon angelegt hatte —, Seite, Feed und Vorschau gehen jetzt einen Weg. Kostet für eine Seite ohne Album nichts: `LoadFor` liest die Slugs zuerst aus dem HTML, `Expand` gibt einen markenfreien Rumpf unverändert zurück. Neu: `internal/public/album_failure_test.go`. |
| **WR-03** Die Vorschau zeigt die Marke statt der Galerie | `4e462b1` | `previewPageContent` ist eine Methode geworden und erweitert die Albenmarken; `previewAlbums` daneben ist derselbe nichtfehlschlagende Lader wie in `internal/public`, gegen `nil` sicher und mit der leeren Menge bei Lesefehler. Bewusst nicht geteilt: `internal/admin` importiert `internal/public` nicht, und eine kleine Funktion auf jeder Seite ist billiger als diese Abhängigkeit. Die Sprache der Website geht mit hinein, damit die drei Steuernamen des Lichtkastens in der Sprache der vorgeschauten Seite stehen. Neu: `internal/admin/preview_album_test.go`. |
| **WR-04** `importAlbumSlug` leitet die Adresse ein zweites Mal ab | *(kein eigener Commit — durch `ca85c63` geschlossen)* | Die zweite Ableitung ist weg: `importBlocks` schlägt in der Karte nach, die `importAlbums` mit dem Slug jedes tatsächlich angelegten Albums füllt, und `page.Slugify` kommt in `internal/bundle/blocks.go` nicht mehr vor. Der Satz im Paketkommentar von `internal/album` ist damit wieder wahr und wurde in `2e02be3` entsprechend umgeschrieben — **er wurde allerdings umgeschrieben, bevor `ca85c63` ihn wahr machte**, es gab also ein Fenster von sechs Minuten, in dem der Kommentar dem Code vorauslief. Heute stimmen beide. Gehalten wird das nicht von einem Zähl-Tor, sondern von drei Verhaltenstests, siehe die Mutationsproben unten. |
| **WR-05** Kein Handler-Test für Website-Grenzen bei den acht schreibenden Alben-Handlern | `4c32f5c` | `internal/admin/album_scope_test.go`, in der Form, die die Wissensdatenbank vorschreibt: zwei Websites, die Sache auf B, eine Bearbeiterin, deren `user_websites` nur A umfasst, der Aufruf durch die **echte** Kette (Sitzung, `RequireWebsiteAccess`, die neun Routen wie `main.go` sie einträgt), und die **Wirkung** neben dem Status geprüft — Name, Slug, Bilderzahl, Reihenfolge, Beschreibungen und Unterschriften von Bs Album, auf Bs eigener Website-Kennung nachgelesen. Mit den Gegenproben: Anlegen, Umbenennen, Bild hinzufügen, Bild löschen, Album löschen und Umsortieren auf der **eigenen** Website gelingen weiterhin. **Kein lebender Fehler** — der Speicher ist wirklich sauber gefasst, und ich konnte durch keine der neun Routen einen Zugriff bauen. Das ist der Wächter, nicht der Befund. |
| **WR-06** Der Archiv-Import kann einen Film in ein Album legen | `a4af157` | `GalleryItems` überspringt jetzt `!ok || img.Film`, genau wie der Bildzweig oben in derselben Datei es seit jeher tut. Erreichbar, ohne dass etwas schiefgeht: `AddItem` prüft die Website und bewusst nicht die Dateiart — `requireOwnPicture` im Handler ist die Stelle dafür —, aber der Archiv-Import ruft `AddItem` unter dem Handler hindurch und löst das Bild über `mediaByName` auf, was einen MP4 nennen kann. Der Wächter steht im Zeichner und nicht im Speicher: das ist die billigere der beiden Antworten des Reviews **und** die weitere, weil sie Album und eingebaute Galerie zugleich abdeckt, und der Speicher darf weiter keine Meinung haben. Der Film fällt weg wie eine nicht auflösbare Kennung: die Kennungen bleiben am eigenen Listenplatz, die Schrittlinks laufen über die **gezeichnete** Liste. Neu: `internal/block/gallery_film_test.go`, `internal/public/album_film_test.go`. |
| **WR-07** `AddItem` liest die Reihenfolge vom Lesevorrat und schreibt auf dem Schreibvorrat | `23e0208` | Zählung und Maximum laufen jetzt **auf der Transaktion**, über dem `INSERT`. Das ist die zweite der beiden Antworten des Reviews und der ersten vorgezogen, weil der `INSERT` über `RowsAffected() == 0` schon „nicht das Album dieser Website" meldet: die Obergrenze in dieselbe `WHERE` zu nehmen, hiesse eine Antwort auf zwei Fragen. `ErrTooManyItems` und `ErrNotFound` sind für eine Betreiberin verschiedene Sätze. Neu: `internal/album/order_test.go`. |

### Ein Hinweis, mitgenommen

| Befund | Commit | Warum hier |
|---|---|---|
| **IN-01** `block.HasAlbumMarker` ist ausgeführt und hat nur Aufrufer im eigenen Paket | `b96becb` (mit WR-02) | Der Hinweis liess offen: unexportieren **oder** in WR-02s Behebung verwenden, „wo endlich ein öffentlicher Aufrufer entsteht". WR-02 entscheidet das: weil jetzt bedingungslos erweitert wird, kann dieser Aufrufer nicht entstehen — genau das Nicht-Erweitern war der Fehler. Also `hasAlbumMarker`. `AlbumMarkerSlugs` und `ReplaceAlbumMarkers` stellen die Frage selbst, ausserhalb des Pakets muss sie niemand vorher stellen. |

## Mutationsproben

Jede Behebung dieses Laufs wurde rückgängig gemacht, der benannte Test rot
gesehen und wiederhergestellt. Der wörtliche rote Ausdruck steht hier, weil
eine Überdeckung, die nie rot war, kein Beweis ist.

**WR-02** — die Vorbedingung wiederhergestellt (`if h.albumStore != nil` mit
Überspringen bei Fehler):

```
--- FAIL: TestAFailingAlbumQueryLeavesNoMarkerOnThePage (0.09s)
    album_failure_test.go:49: the visitor is served the raw marker although the
    album query failed:
--- FAIL: TestAPageWithAnAlbumMarkerAndNoStoreLeavesNoMarker (0.07s)
    album_failure_test.go:73: a build with no album store printed the raw marker
    to a visitor:
```

**WR-03** — `ContentHTML` wieder unerweitert geschrieben:

```
--- FAIL: TestThePreviewShowsTheAlbumAndNotTheMarker (0.55s)
    preview_album_test.go:84: the preview shows the internal syntax where the
    pictures belong:
    preview_album_test.go:87: the preview does not show the album's picture:
--- FAIL: TestThePreviewOfAnEmptyAlbumShowsNoMarkerEither (0.54s)
    preview_album_test.go:99: an empty album left its marker on the preview:
--- FAIL: TestThePreviewWithoutAnAlbumStoreShowsNoMarker (0.52s)
    preview_album_test.go:112: a handler with no album store printed the marker
    into the preview:
```

**WR-04** — `importAlbumSlug` mit `page.Slugify` wiederhergestellt. Das ist
zugleich die heutige Nachprüfung von CR-02:

```
--- FAIL: TestTwoAlbumsWithOneNameDoNotCollapseIntoOne (0.08s)
    album_collision_test.go:144: block 0 points at album "werkstatt-2025"
    although the archive named two albums "Werkstatt 2025" and only one of them
    can exist
    album_collision_test.go:144: block 1 points at album "werkstatt-2025" …
--- FAIL: TestAnAlbumThatCouldNotBeCreatedBindsNoGallery (0.07s)
    album_collision_test.go:209: block 0 points at album "untitled" although no
    album was created at all
--- FAIL: TestAnOverlongAlbumNameLandsWhereTheStorePutIt (0.07s)
    album_collision_test.go:274: the gallery points at "werkstatt-werkstatt-…-
    werkstatt" and the album the store made is at "werkstatt-…-werkstatt" — two
    derivations of one key, and nothing anywhere reports the empty gallery
```

**WR-05** — in zwei Stufen, weil ein grüner Wächter für einen Fehler, den es
nicht gibt, für sich nichts beweist.

(a) `Store.Get` hört auf zu fassen — genau die Form, die die Wissensdatenbank
beschreibt. Alle sieben Verweigerungen werden am Status rot:

```
album_scope_test.go:214: edit: status 200, want 404 or 403
album_scope_test.go:228: update: status 303, want 404 or 403
album_scope_test.go:237: delete: status 303, want 404 or 403
album_scope_test.go:246: item create: status 303, want 404 or 403
album_scope_test.go:256: item update: status 303, want 404 or 403
album_scope_test.go:265: item delete: status 303, want 404 or 403
album_scope_test.go:275: item reorder: status 303, want 404 or 403
```

(b) zusätzlich jede `WHERE` im Speicher entfasst — der menü-förmige Speicher,
in dem nichts mehr hält. Jetzt greifen die Wirkungsprüfungen, und das ist die
Hälfte, die alle sechs ausgelieferten Fehler dieser Familie gefangen hätte:

```
update: website B's album became "Übernommen"/"geheime-referenzen"
delete: website B's album is gone
item create: website B's album has 3 pictures, had 2
item update: website B's descriptions are now "Übernommen", "B zwei"
item delete: website B's album has 1 pictures, had 2
item reorder: website B's pictures are now 2, 1 — the order was rewritten
```

**WR-06** — `|| img.Film` entfernt:

```
--- FAIL: TestAFilmInAGalleryIsNotDrawnAsAPicture (0.00s)
    gallery_film_test.go:43: a film was rendered into a picture grid:
--- FAIL: TestAFilmInAGalleryLeavesTheStepLinksWhole (0.00s)
    gallery_film_test.go:62: 3 large views, want 2 — the film was counted as a
    picture
--- FAIL: TestAGalleryOfNothingButFilmsRendersNothing (0.00s)
    gallery_film_test.go:90: a gallery of nothing but films rendered
    "…<img src=\"/media/1/a.mp4\" alt=\"\" …>…"
--- FAIL: TestAFilmInAnAlbumIsNotServedAsAPicture (0.09s)
    album_film_test.go:40: the album's film reached the page:
```

**WR-07** — gegen den wörtlichen Quelltext vor der Behebung
(`git show HEAD:internal/album/store.go`):

```
--- FAIL: TestConcurrentAddsGetDistinctSortOrders (0.09s)
    order_test.go:82: pictures 2 and 3 both sit at sort_order 1 — the next place
    was read on a different snapshot than the one it was written on, and the
    arrow button between them will now report success and move nothing
    (und drei weitere Paare im selben Lauf)
--- FAIL: TestTheItemCapIsNotExceededByConcurrentAdds (0.10s)
    order_test.go:189: attempt 1: the album holds 121 pictures and the cap is
    120 — the count was read on a snapshot older than the insert that acted on it
```

### Was WR-07s Proben nebenbei zeigten, und es steht jetzt im Kommentar

Die erste Mutation, die ich versuchte, verschob nur die Lesung zurück auf
`s.DB.Read` und liess sie **unter** `BeginTx` stehen. **Sie blieb grün.** Was
die Aufrufe hintereinanderreiht, ist nicht der Vorrat, aus dem gelesen wird,
sondern `BeginTx`: der Schreibvorrat hält eine einzige Verbindung
(`SetMaxOpenConns(1)`), ein zweites `AddItem` kommt an dieser Zeile nicht
vorbei. Die Lesung auf `tx` ist trotzdem die richtige — sobald über der Zählung
je eine Anweisung dazukommt, läse die andere Verbindung einen Zustand, den
diese Transaktion schon verlassen hat. Beides steht jetzt in `AddItem`, damit
die nächste Person es nicht andersherum wieder auseinandernimmt.

Die Obergrenzen-Probe fährt **zwanzig Versuche** und sagt das in ihrem eigenen
Kommentar: ein einzelner Versuch traf das Fenster etwa in einem von vier
Läufen, ein Ein-Schuss-Test wäre also in drei von vier Läufen aus dem falschen
Grund grün gewesen. Sie füllt das Album mit rohem SQL und nicht über `AddItem`,
weil 119 eigene Schreibtransaktionen vor dem Rennen die Schreibverbindung warm
laufen lassen und genau das Fenster verengen, um das es geht.

`TestASwapBetweenTwoEqualOrdersIsWhatTheDuplicateCosts` ist ein
festschreibender Test und vor wie nach der Behebung grün: er erzwingt die
Doppelung mit rohem SQL und prüft, dass der Tausch dann wirklich nichts tut.
Er steht dort, damit der Grund, warum der erste Test zählt, in der Datei liegt
und nicht nur in einer Commit-Nachricht. **Er war nie rot und wird hier nicht
als Beweis geführt.**

### Heutige Nachprüfung der kritischen Befunde

Zusätzlich zu `11-KRITISCH-NACHPRUEFUNG.md`, auf dem fertigen Baum:

```
CR-01a  Durchfall auf If-Modified-Since wiederhergestellt
        --- FAIL: TestAWarmBrowserIsNotKeptOnAnOldAlbum
            cache_test.go:104: 304: the visitor keeps the old gallery although
            the album changed — GAL-03 is false on the cached path
        --- FAIL: TestAMismatchingIfNoneMatchEndsTheNegotiation
            cache_test.go:134: 304 for a request whose If-None-Match did not
            match — If-Modified-Since decided a negotiation RFC 7232 §3.3 says
            it may not take part in

CR-01b  contentModTime vergisst die Alben
        --- FAIL: TestChangingAnAlbumMovesThePagesLastModified
            cache_test.go:170: Last-Modified stayed at "…" although the album
            changed — the validator does not see the albums

CR-02   siehe WR-04 oben, dieselbe Mutation

CR-03   expandForFeed wieder nur Textbausteine
        --- FAIL: TestTheFeedDoesNotShipTheRawAlbumMarker
            feed_album_test.go:64: the feed carries the raw marker
            [[album:moebel:0]]; every subscriber's reader shows it, and it leaks
            the album's address:
            feed_album_test.go:71: the feed does not carry the album's pictures
            either — the marker was dropped rather than expanded:

CR-04   albumChoicesFor gibt wieder nur die Speicherliste zurück
        --- FAIL: TestAGalleryKeepsItsAlbumWhenTheAlbumIsGone
            block_album_survival_test.go:77: the block's own album has no
            selected option, so the browser will submit the first one (value="")
            and the next save deletes the gallery:
        --- FAIL: TestAGalleryKeepsItsAlbumWhenThereIsNoListAtAll
            block_album_survival_test.go:124: with no albums on the website the
            select was not drawn, so nothing posts b0.album and the next save
            deletes the gallery:
```

Alle wiederhergestellt, danach grün.

## Was offen bleibt, und warum

### Fünf Hinweise, ausserhalb des Umfangs (`critical+warning`)

- **IN-02** `album.Store.BySlug` hat keinen Aufrufer ausserhalb der Tests.
  **Nach CR-02 immer noch wahr, und der Hinweis nennt genau den Aufrufer, der
  nicht entstanden ist**: CR-02 löst die Referenz eines Bausteins gegen die
  Karte auf, die `importAlbums` aus `Create` füllt, und braucht `BySlug` dafür
  nicht. Die Methode ist damit endgültig ohne Zweck ausserhalb der Tests. Ein
  Löschen ist eine Aufräumarbeit ohne Verhaltensänderung und gehört nicht in
  eine Behebungsrunde.
- **IN-03** `TestSpecRequiresTheBlockStylesheet` zählt ein Vorkommen und keine
  Platzierung. Betrifft `internal/tmplspec` und ist eine Verschärfung eines
  Tores, kein Fehler.
- **IN-04** `Vary: HX-Request` fehlt an den Alben-POSTs. Der Hinweis nennt
  selbst die richtige Behebung — im gemeinsamen `ErrHandler` —, und die trifft
  jeden Handler des Programms, nicht nur die Alben. `internal/admin/menu.go`
  hat dieselbe Auslassung. Das ist eine eigene, kleine, geplante Änderung.
- **IN-05** Eine Bausteinposition ausserhalb des Zahlenbereichs in einer Marke
  lässt die Galerie still verschwinden. Nur über von Hand geändertes
  `content_html` erreichbar, und Verschwinden ist der sichere Ausgang; ein
  `slog.Warn` wäre die ganze Änderung.
- **IN-06** Nichts prüft, dass von fünfzig Alben genau eines geladen wird. Das
  Verhalten hält die `IN (…)`-Klausel; es fehlt ein Test, der die Eigenschaft
  beobachtbar statt erschlossen macht.

**IN-01 wurde mitgenommen** (siehe oben), weil WR-02 die Frage entschied, die
der Hinweis offenliess.

### Eine Lücke, die WR-03 absichtlich nicht schliesst

Die Vorschau erweitert weiterhin **keine Textbaustein-Marken**, und tat es auch
vorher nicht. Diese Lücke ist älter als Phase 11 und breiter als ein Baustein:
sie zu schliessen heisst zu entscheiden, was eine Vorschau für einen
bearbeiteten, aber noch nicht gespeicherten Textbaustein zeigt — eine Frage über
die Vorschau, nicht über das Album. Eine Seite mit unerweitertem Textbaustein
ist eine Seite, der ein Satz fehlt; eine Seite mit unerweitertem Album war eine
Seite mit einem Klammerzeichen dort, wo eine ganze Galerie hingehört. Der Grund
steht im Kommentar über `previewPageContent` und nicht nur hier.

### Ein Befund, der beim Nachsehen schon geschlossen war

**WR-04** hat keinen eigenen Commit, weil `ca85c63` (CR-02) ihn mitgeschlossen
hat — die zweite Ableitung des Schlüssels und die Namenskollision sind
derselbe Fehler von zwei Seiten. Der Commit-Text von `ca85c63` sagt das
ausdrücklich. Ich habe es nicht geglaubt, sondern die Mutation gefahren; sie
steht oben.

## Was nicht behoben wurde, obwohl es hätte sein können

Nichts. Kein Befund im Umfang wurde als „das Review hat unrecht" abgewiesen,
und keiner wurde weitergeschoben. **WR-02 war der einzige, bei dem das Review
gegen eine ausdrückliche Begründung im Quelltext argumentierte** — dort hat es
recht behalten, und der Grund steht jetzt in der Datei, damit die alte Regel
nicht in einem Jahr wieder zurückkommt.

## Prüfstand

```
go build ./...        sauber
go vet ./...          still
gofmt -l .            still
go test ./...         exit 0, 44 Pakete ok, 0 FAIL
go run ./tools/i18n   1318 Zeichenketten, en/es/fr/it je 41 offen, 0 verwaist
                      de-CH 73 Abweichungen, 0 ohne Gegenstück
```

**Die Zahl der offenen Zeichenketten steht vor und nach diesem Lauf bei 41.**
Keine der sechs Behebungen prägt einen Satz, den eine Betreiberin liest — es
sind Zeichner, Speicher und Erweiterung, dazu Tests. Die 41 sind unverändert
Plan 11-07s Arbeit.

---

_Behoben: 2026-09-08_
_Erster Lauf: CR-01…CR-04, WR-01 (vom Nutzungslimit abgebrochen, ohne Bericht)_
_Zweiter Lauf: WR-02…WR-07, IN-01, dieser Bericht_
