# Zurückgestellt — Phase 8

## Aus Plan 08-04

**`07-SECURITY.md:183` nennt „T-07-26 (Plan 05)", die Nummer gehört aber Plan 06.**

Gefunden beim Schliessen von T-07-26. Die Stelle steht in **W-4** („Die
Bezeichnungsmenge ist von 12 auf 32 MiB gewachsen") und redet über die
Massnahme zur Prägung von Bezeichnungen aus Plan 05 — eine andere Bedrohung als
T-07-26, die Information Disclosure aus Plan 06 über `field.Hidden` ist. Die
Nummer dort ist also eine Verwechslung, und sie stand schon vor diesem Plan da.

Nicht angefasst, aus zwei Gründen: Plan 08-04 verlangt ausdrücklich, keinen
anderen Eintrag jenes Dokuments zu ändern, und welche Nummer dort richtig wäre,
liesse sich nur raten — das Register führt für Plan 05 mehrere Kandidaten.

**Folge:** das Zähltor `grep -c 'T-07-26' 07-SECURITY.md` misst nach dem
Schliessen **2** statt der vom Plan erwarteten 1. Eine zu lang, und die
Ursache ist diese Fremdnennung, nicht ein unvollständiges Schliessen.

**Zu tun:** beim nächsten `/gsd-secure-phase 7` die Nummer in W-4 gegen das
Register prüfen und richtigstellen.

## Aus Plan 08-05

> **Erledigt am 2026-09-06 im Schnellauftrag `260906-m9z`.** Der Stempel gilt
> der Feststellung zu `field_list.html` unmittelbar darunter; der zweite Befund
> dieses Abschnitts (die zwei verwaisten Unterfelder) bleibt davon unberührt.
>
> Der Text darunter bleibt stehen, weil er belegt, was damals geglaubt wurde.
> **Die Begründung des Aufschubs ruhte jedoch auf einer falschen Annahme**
> darüber, worin der Flick besteht: sie gilt allein für den unten
> vorgeschlagenen Flick, die Entität durch das Zeichen `←` zu ersetzen. Der
> tatsächlich gefahrene Flick wechselt an denselben drei Stellen nur die
> aufrufende Funktion von `t` auf die HTML-durchlassende Fassung `th`. Die
> Zeichenkette bleibt byte-für-byte dieselbe, kein Schlüssel verwaist, kein
> Katalog wurde angefasst, und `go run ./tools/i18n` stand vorher wie nachher
> auf `1158 Zeichenketten im Quelltext` mit `0 offen, 0 verwaist` für
> en/es/fr/it.

**`field_list.html` druckt `&#8592;` als Text statt als Pfeil — auf allen drei Rückwegen.**

Im Browserdurchgang gesehen (Schritt 1): der Rückweg über dem Feldbildschirm
liest wörtlich „&#8592; Alle Textbausteine" statt „← Alle Textbausteine". Die
Ursache ist, dass die Zeichenkette als HTML-Entität im Quelltext steht und
`{{t "…"}}` ihr Ergebnis kontextabhängig maskiert — `&` wird zu `&amp;`,
also erscheint die Entität selbst.

Nicht hier geflickt, und der Grund ist der Umfang: die Stelle steht dreimal
(`field_list.html:8`, `:16`, `:25`), zwei der drei Arme sind älter als diese
Phase, und die Zeichenkette **ist der Katalogschlüssel** — sie zu ändern legt in
`en/es/fr/it.json` drei neue Schlüssel an und lässt die drei alten verwaisen.
Das Tor „0 verwaist" fällt dann, bis die alten von Hand aus vier Katalogen
entfernt sind. Das ist eine i18n-Aufräumarbeit mit eigenem Commit, keine
Nebenwirkung eines Vorrichtungsplans.

**Überholt — diese Schrittfolge nicht fahren** (Stempel vom 2026-09-06,
Schnellauftrag `260906-m9z`). Genau sie hätte die Verwaisung, die zu vermeiden
das Ziel war, überhaupt erst erzeugt: sie tastet den Schlüssel an und muss
deshalb hinterher vier Kataloge aufräumen. Gefahren wurde stattdessen der
Wechsel von `t` auf `th` an denselben drei Stellen, ohne ein Zeichen der
Zeichenkette zu berühren. Der Absatz steht hier nur noch als Beleg:

~~Was zu tun ist: in den drei Zeilen `&#8592;` durch das Zeichen `←` ersetzen,
`go run ./tools/i18n -write` fahren, die drei neuen Schlüssel übersetzen, die
drei alten aus `en.json`, `es.json`, `fr.json` und `it.json` streichen,
`-schweiz` fahren und auf `0 offen, 0 verwaist` prüfen.~~

**Zwei verwaiste Unterfelder im Wegwerf-Datenverzeichnis** — kein Produktcode.
Vor dem Flick dieses Plans angelegt (`snippet_id NULL` unter einer Gruppe mit
`snippet_id`); das Verzeichnis ist mit dem Browserdurchgang gelöscht worden. Auf
einer echten Installation, die zwischen 08-03 und diesem Flick eine Gruppe an
einem Textbaustein angelegt hat, bleiben solche Zeilen unsichtbar stehen. Sie
schaden nichts — kein Leseweg **des Textbausteins** gibt sie heraus; der
Gruppenbildschirm (`?gruppe=<id>`) zeigt sie weiterhin, weil `Store.Sub` allein
nach `website_id` und `parent_id` sucht und keine Textbausteinklausel trägt.
Unsichtbar sind sie also nur auf dem Formular des Textbausteins, und das ist
genau, was `OfSnippet`/`OfSnippets` bewirken. Wer sie loswerden will, legt die
Unterfelder neu an. *(Satz berichtigt im Code-Review zu Phase 8, IN-01: die
Feststellung „kein Leseweg" war zu weit gefasst. Die Verfügung — keine
Reparaturwanderung — steht unverändert, denn 08-03 und der Flick liegen in
derselben Phase.)*

## Aus dem Code-Review zu Phase 8

**Bild-, Verweis- und Schlagwortwerte eines Textbausteins überleben die
Archivreise nicht** (WR-03).

Der Wert eines solchen Feldes ist eine Nummer *dieser* Anlage. Auf dem
Seitenweg werden sie beim Ausfahren übersetzt — eine Mediennummer in einen
Dateinamen, eine Seitennummer in eine Adresse (`exportFieldValues`) — und beim
Einfahren zurück (`translateIn`). Die Werte eines Textbausteins gehen roh
hinaus und roh hinein (`exportSnippets`), drüben gehört die Nummer einer
anderen Website, und `fieldImages`/`fieldRefs` weisen sie zurück: das Feld kommt
an, das Bild nicht.

Das ist keine Randlage: `internal/admin/field.go` bietet diese Feldarten am
Textbaustein ausdrücklich an (`kinds = field.Kinds`), und `field_list.html:20`
verspricht sie dem Betreiber. Das Versprechen hält beim Anzeigen und bricht
beim Ausfahren.

**Was im Review geflickt wurde:** nur die Lautstärke. `ortsgebundeneWerte`
meldet beim Import jeden solchen Wert im Bericht, damit die Wahl drüben zu
wiederholen ist, statt dass sie stillschweigend fehlt. Der Wert reist weiterhin
mit; nichts verschwindet.

**Was offen bleibt:** die Übersetzung selbst. `translateOut`/`translateIn`
sitzen im Seitenweg und leben von Nachschlagewerken, die dort gebaut werden; sie
von dort zu lösen und am Textbaustein wiederzuverwenden ist eine Änderung am
Seitenweg und damit eine eigene Arbeit. **Gehört in die Roadmap, nicht in einen
Kommentar** — Phase 7 hat genau diese Sorte Verlust auf genau diesem Weg schon
einmal ausgeliefert.
