---
phase: 11-galerie
audited: 2026-09-08
status: OPEN_THREATS
verdict_note: >-
  Kein blockierender Befund. 36 von 37 Bedrohungen geschlossen, eine offen und
  unterhalb der Schwelle (medium; block_on steht auf high). Dazu sechs
  unregistrierte Flächen, die keiner Bedrohungszeile zugeordnet waren — die
  wertvollste Hälfte dieses Berichts.
threats_total: 37
threats_closed: 36
threats_open_nonblocking: 1
threats_open_blocking: 0
asvs_level: 2
block_on: high
---

# Phase 11 — Sicherheitsprüfung

43 Registerzeilen über sieben Pläne; `T-11-SC` wiederholt sich je Plan und zählt
einmal → **37 verschiedene Bedrohungen**. Jede Zeile unten ruht auf gelesenem
Code, einer ausgeführten Abfrage oder einem gefahrenen Test — nicht auf einer
Zusammenfassung.

## Der Isolationsbefund, um den es zuerst geht

Er wurde neu hergeleitet und nicht aus dem Code-Durchgang übernommen. Drei
Fragen, drei Antworten:

**Löst die Erweiterung eine Marke innerhalb der Website der Seite auf?** Ja, an
allen fünf Stellen, und die Websitenummer kommt jedes Mal aus der Anfrage und
nie aus der Seitenzeile oder aus der Marke: `pagedata.go:63` (Seite),
`feed.go:92` (Atom), `pluginhost.go:82` (Erweiterungen), `preview.go:298`
(Vorschau), `access.go:183` (Seite hinter dem Passwort). Und der Test legt
wirklich **auf beiden Websites ein Album mit demselben Slug** an und schlägt
laut fehl, wenn die beiden je auseinandergehen.

**Gehört ein Bild auf jedem Schreibweg zur Website des Albums?** Drei Wege,
alle gedeckt. Verwaltung anlegen/ändern erreichen `requireOwnPicture` im
Handler (der auch einen Film abfängt) **und** `requireOwnMedia` im Speicher.
Der Archivimport geht unter dem Handler durch, erreicht aber `requireOwnMedia`
weiterhin; den Film, den er einschmuggeln könnte, fängt stattdessen der
Renderer (`render.go:594`), was den eingebetteten Fall gleich mitdeckt. Der
Join `m.website_id = a.website_id` in `LoadFor` ist der Rückhalt beim Lesen für
jede Zeile, die vor diesen Prüfungen geschrieben wurde.

**Legt der Archivimport unter der Zielwebsite an?** Ja, auf jedem Pfad —
`import.go:391 Create(ctx, websiteID, …)` und `:413 AddItem(ctx, websiteID, …)`.

Alle 20 Aufrufstellen des Albenspeichers ausserhalb seines eigenen Pakets
wurden aufgezählt. Keine ungescopte. **Das hat nicht die Form der sechs
ausgelieferten Fehler**: die Prüfung steht in der `WHERE`-Bedingung und die
Websitenummer in der Signatur, also zählt der Übersetzer die Aufrufer auf —
genau das, was `.planning/debug/knowledge-base.md` vorschreibt.

## Geschlossen (36)

Auszug der tragenden Zeilen; die vollständige Herleitung stand im Prüfbericht
und ist hier zusammengefasst.

| ID | Art | Schwere | Beleg |
|---|---|---|---|
| T-11-01 | Tampering | high | `render.go:622`, `:639`, `:686-687` — alles `html.EscapeString`, ein Renderer, kein zweiter Weg |
| T-11-02 | Tampering | med | `render.go:603` baut die Kennung aus `strconv.Atoi` über `\d+`; keine Redakteurszeichenkette erreicht eine id |
| T-11-03 | EoP | high | `script-src 'self'` auf beiden CSPs; null `<script>` in allen acht Themes ausser `ld+json`; im Browser mit abgeschaltetem Skript bestätigt |
| T-11-05 | InfoDisc | low | `const closeTarget = "hc-zu"`, als `href` und nie als `id` geprüft |
| T-11-06 | EoP | high | alle ~22 Anweisungen in `internal/album/store.go` nennen `website_id`; die vier Eintragsanweisungen tragen ihre eigene Unterabfrage |
| T-11-07 | EoP | high | `requireOwnMedia`, von beiden `AddItem`-Aufrufern erreicht |
| T-11-09 | DoS | high | `store.go:638-700` — vier Anweisungen über vier Ganzzahlen, nichts wartet |
| T-11-10 | Tampering | high | ausschliesslich gebundene Parameter; die einzige Verkettung ist eine Konstante und eine `IN`-Liste aus Indizes |
| T-11-11 | EoP | high | `albumFromPath` auf allen neun Routen, 13 Speicheraufrufe mit `ws.ID` |
| T-11-13 | Spoofing | high | CSRF über `adminProtectedMux`; im Browser: POST ohne Marke → 403 |
| T-11-14 | EoP | high | `requireOwnPicture` an beiden Stellen, `requireOwnMedia` als zweiter Riemen |
| T-11-18 | Tampering | high | `DisplayClass()` bildet auf ein Literal ab; **ausgeführt**: `Display = "\" onload=alert(1) x=\""` ergibt `""` |
| T-11-21 | Tampering | high | `page.Slugify` auf dem Formularweg — **aber siehe UF-1** |
| T-11-22 | InfoDisc | high | `a.website_id = $1` **und** `m.website_id = a.website_id`; der Test legt denselben Slug auf beiden Websites an |
| T-11-27 | Tampering | high | `grep -c "INSERT INTO" internal/bundle/import.go` → **0**; alles über `Create` |
| T-11-31 | Tampering | high | beide Richtungen; die Mutation „erst umbenennen" ist rot festgehalten |
| T-11-33 | EoP | high | im Browser: vier fremde Albenrouten → 404 **mit gültiger CSRF-Marke von der eigenen Seite der anderen Website**, und die Wirkung nachgeprüft |
| T-11-36 | Spoofing | high | `album_list`, `album_edit` in `layoutPageNames`; im Browser bestätigt |

`T-11-04` und `T-11-SC` sind **akzeptiert** statt gemindert; zu `T-11-04` siehe
UF-3, dessen Zahl berichtigt gehört.

## Offen, nicht blockierend (1)

**T-11-17 — die Formularhälfte hält, die Manifesthälfte wurde nie gebaut.**

Die Registerzeile verlangt, dass „ein handgeschriebenes Formular **oder ein
handgeschriebenes Manifest** keinen dritten Wert ablegen kann".
`internal/bundle/blocks.go:108` kopiert `Display: b.Display` unverändert aus dem
Manifest, und `set.Clean` prüft `Type`, `Fields` und `Items` — aber nie
`Display`. Gemessen, nicht behauptet:

```
stored Display after Clean = "\" onload=alert(1) x=\""
encoded JSON: [{"typ":"galerie","darstellung":"\" onload=alert(1) x=\"", …}]
DisplayClass() = ""
```

**Warum es nicht blockiert:** T-11-18 ist der tragende Wächter und hält
uneingeschränkt — der Wert erreicht die Auszeichnung nie, es gibt also keine
Einschleusung. Was bleibt, ist ein gespeicherter Wert, den der Code nicht
kennt: genau die Klasse, über die `.planning/GLOSSARY.md` geschrieben wurde.
Der Flick ist eine Zeile neben `blocks.go:108`.

## Unregistrierte Flächen (6)

Die wertvollste Hälfte des Berichts: neue Fläche, der keine Bedrohungszeile
zugeordnet war.

**UF-1 — der Kommentar, auf dem zwei Bedrohungen ruhen, sagt das Gegenteil des
Gemessenen.** `internal/block/render.go:444-454` behauptet, die Marke werde
„von diesem Programm in HTML geschrieben und trifft weder goldmark noch
bluemonday". Durch die echte Kette gefahren:

```
in : Ein Absatz mit [[album:sommer-2025:1]] mittendrin.
out: <p>Ein Absatz mit [[album:sommer-2025:1]] mittendrin.</p>   slugs: [sommer-2025]

in : <a href="/x?q=[[album:sommer-2025:1]]">klick</a>
out: <p><a href="/x?q=[[album:sommer-2025:1]]" rel="nofollow">klick</a></p>   slugs: [sommer-2025]
```

Ein Redakteur, der rohes HTML in einen Textbaustein schreibt, legt eine Marke
**in einem Attributwert** ab. `ReplaceAlbumMarkers` ersetzt kontextfrei, also
beendet das erste `"` der Galerie zur Anfragezeit das `href`.

**Kein XSS, und es wurde versucht:** alles nach dem Bruch ist vom Renderer
erzeugt, Alt und Bildunterschrift sind maskiert, nachlaufender Redakteurstext
landet maskiert im Datenzustand, bluemonday verwirft ein `href` mit Leerzeichen,
und `script-src 'self'` verbietet Inline-Skript ohnehin. **Ergebnis:
Auszeichnungsschaden auf einer Seite, selbst zugefügt von einem angemeldeten
Redakteur auf seiner eigenen Website, keine Reichweite über die Websitegrenze.**
Der Befund ist der Kommentar, nicht der Schaden.

**UF-2 — die Zahl *verschiedener* Alben, die eine Seite nennt, ist unbegrenzt.**
`MaxBlocks = 60` begrenzt Galeriebausteine, aber ein einzelner Textbaustein kann
beliebig viele verschiedene `[[album:aN:1]]` tragen (oben bewiesen), und ein
Textbaustein vervielfacht das über Seiten hinweg. Degradiert sicher: jenseits
der Variablengrenze von SQLite scheitert die Abfrage, `albumsFor` gibt die
Null-Menge zurück, jede Marke erweitert zu nichts. Also Anfragekosten, kein Leck
— und 11-05s Zusage („nur die Alben, die das HTML nennt") ist eingehalten.

**UF-3 — die akzeptierte Zahl in T-11-04 steht für die falsche Konstante.** Die
Annahme rechnet mit `block.MaxItems = 24` und „48 `<img>` im schlimmsten Fall".
Eine **Album**-Galerie ist davon nicht begrenzt: `album.MaxItems = 120`, also
240 pro Galerie und rund 14 400 bei sechzig Bausteinen. Alle mit `loading=lazy`
und die Grossansichten `display:none`, die Annahme trägt also weiter — die Zahl
darin nicht. **Sie gehört berichtigt statt weitergetragen.**

**UF-4 — drei erklärte mechanische Tore gibt es nicht.** Kein Test liest
`internal/album/store.go` und behauptet, jede Anweisung nenne `website_id`
(T-11-06 und T-11-22 berufen sich darauf); keiner zählt `adminOnly` auf 19 und
editoroffen auf 6 (T-11-12, und `11-03-SUMMARY.md` wiederholt es); keiner zählt
`hx-disabled-elt` (T-11-16). **Die Substanz ist in allen drei Fällen da** und
wurde von Hand nachgeprüft — aber nach der eigenen Regel dieser Prüfung ist ein
Test der Beleg, dass etwas einmal galt, und diese drei sind nicht einmal das.
Die wirklichen Wächter sind `internal/admin/album_scope_test.go` (verhaltensnah,
in der von der Wissensbasis vorgeschriebenen Form) und die
Vollständigkeitsprüfung in `main_test.go`, die stärker ist als die Zählung, die
sie ersetzt hat.

**UF-5 — `internal/tmplmgr/script.go` nimmt einen Typ aus, den CLAUDE.md nicht
nennt.** CLAUDE.md erlaubt nur `application/ld+json`; der Code nimmt auch
`application/json` aus. Beide sind laut Spezifikation untätig und `importmap`
wird richtig abgewiesen — also eine Abweichung zwischen Dokument und Code und
kein Loch. Entweder CLAUDE.md erweitern oder die Abbildung verengen.

**UF-6 — T-11-25s Registertext beschreibt das Gegenteil des Ausgelieferten.**
Der Plan schrieb „der Rumpf wird unerweitert zurückgegeben"; WR-02 hat das
umgedreht. `albumsFor` gibt bei jedem Fehler die Null-Menge zurück und die
Erweiterung läuft unbedingt, eine scheiternde Abfrage kostet also die Galerie
und nicht die Seite — und leakt `[[album:<slug>:n]]` nicht mehr. Der berichtigte
Satz gehört hierher, nicht der des Plans.
