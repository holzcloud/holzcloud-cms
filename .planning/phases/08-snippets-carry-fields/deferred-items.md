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

Was zu tun ist: in den drei Zeilen `&#8592;` durch das Zeichen `←` ersetzen,
`go run ./tools/i18n -write` fahren, die drei neuen Schlüssel übersetzen, die
drei alten aus `en.json`, `es.json`, `fr.json` und `it.json` streichen,
`-schweiz` fahren und auf `0 offen, 0 verwaist` prüfen.

**Zwei verwaiste Unterfelder im Wegwerf-Datenverzeichnis** — kein Produktcode.
Vor dem Flick dieses Plans angelegt (`snippet_id NULL` unter einer Gruppe mit
`snippet_id`); das Verzeichnis ist mit dem Browserdurchgang gelöscht worden. Auf
einer echten Installation, die zwischen 08-03 und diesem Flick eine Gruppe an
einem Textbaustein angelegt hat, bleiben solche Zeilen unsichtbar stehen. Sie
schaden nichts — kein Leseweg gibt sie heraus —, sind aber auch nicht
erreichbar; wer sie loswerden will, legt die Unterfelder neu an.
