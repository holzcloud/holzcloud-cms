# Changelog

Diese Datei hält fest, was sich zwischen zwei Fassungen für jemanden geändert
hat, der diese Software betreibt. Sie folgt dem Aufbau von
[Keep a Changelog](https://keepachangelog.com/de/1.1.0/) — neuste Freigabe
zuoberst, eine Überschrift je Fassung mit Nummer und Datum, darunter Gruppen —
schreibt die Einträge aber in ganzen Sätzen statt in Stichworten. Das ist
Absicht: jedes andere Dokument in diesem Projekt erklärt auch das Warum, und
eine Liste abgehackter Halbsätze läse sich, als hätte sie jemand anderes
verfasst. Wer den nächsten Eintrag schreibt, macht bitte mit.

Die Nummern sind dieselben wie die Tags im Repository.

## 1.6 — 2026-09-04

### Behoben

**Das Bearbeiten einer Seite verwarf ihre Bausteine eigener Arten.** Wer eine
bestehende Seite im Editor speicherte, verlor jeden Baustein, dessen Art die
Website sich selbst angelegt hatte — still, ohne Meldung, und mit einem
augenscheinlich erfolgreichen Speichern. Die eingebauten neun Arten blieben,
alles andere wurde beim Aufräumen verworfen, weil dem Speicherpfad die Liste
der eigenen Arten fehlte. Aus derselben Ursache legte der Knopf für eine eigene
Art keinen Baustein an und die Arten verschwanden nach jeder Änderung aus dem
Menü, sodass sich der Verlust im Editor nicht einmal rückgängig machen liess.

Der Text ging dabei nicht verloren: die Felder eines verworfenen Bausteins
stehen weiterhin im Fliesstext der Seite. Verloren ging die Gestaltung. Wer
eigene Bausteinarten benutzt, sieht bitte die Seiten durch, die seit dem
Einspielen einmal im Editor gespeichert wurden; das Archiv einer Website
(`holzcloud.json`) enthält die ursprünglichen Bausteine mit allen Feldwerten
und ist die verlässliche Quelle für den Wiederaufbau.

Angelegte Seiten waren nie betroffen — nur das Bearbeiten.

## 1.5 — 2026-09-03

### Behoben

**Eine Adresse gibt es jetzt je Sprache.** Bisher galt eine Adresse einmal pro
Website, quer über alle Sprachen. Die französische Fassung von
`/holzcloud-cms` bekam beim Anlegen still ein „-2" angehängt, und weil die
Übersetzungsverweise über die Adresse aufgelöst werden, zeigten sie danach
alle auf die Sprache, die zuletzt eingespielt wurde. Eine fünfsprachige
Website hatte fünf Startseiten, von denen vier falsch verknüpft waren, kein
einziges `hreflang` im Kopf und eine Sprachwahl, die auf die Startseite
zurückfiel. Wer eine mehrsprachige Website betreibt und Produktnamen als
Adresse benutzt, war davon sicher betroffen; die Migration 00045 räumt es auf,
ohne dass etwas von Hand nachzuziehen wäre. Bereits umbenannte Adressen bleiben
allerdings, wie sie sind — ein „-2" wird nicht zurückgenommen, weil daraus
inzwischen ein verlinkter Ort geworden sein kann.

**Die Startseite hatte zwei Adressen.** Sie war unter `/` und unter `/home` zu
haben, beide mit sich selbst als kanonischer Adresse und beide im Sitemap — bei
fünf Sprachen zehn Adressen für fünf Seiten. `/home` leitet jetzt dauerhaft
(301) auf die Wurzel seiner Sprache um und steht nicht mehr im Sitemap.

**Der Knopf im Aufruf-Baustein war in der Vorlage Holzcloud unsichtbar.**
Messingfarbene Schrift auf messingfarbener Fläche, weil die Regel für Verweise
im Fliesstext später steht als die für den Knopf.

**Mehrere Absätze in einem Textbaustein standen ohne Abstand untereinander**,
ebenfalls in der Vorlage Holzcloud.

**Der Importbericht meldete Textfelder als fehlende Bilder.** „die Datei
‚Next.js' fehlt" — geprüft wurde die Schreibweise des Wertes statt der Art des
Feldes.

### Hinzugefügt

**Die Vorlage Holzcloud kleidet vier eigene Bausteinarten**, wenn eine Website
sie anlegt: `vorspann`, `merkmal`, `stand` und `technik`. Damit lassen sich ein
Aufmacher-Satz, eine Faktenliste, eine Statuszeile und eine Reihe Stichworte
setzen, für die es im Editor sonst keine Auszeichnung gibt.

### Geändert

**Das Veröffentlichungsdatum steht in der Vorlage Holzcloud nur noch an einem
Beitrag**, nicht mehr an jeder Seite.

## 1.4 — 2026-09-03

Dies ist die erste öffentliche Freigabe und darum der erste Eintrag in dieser
Datei. Entwickelt wurde das Projekt vorher in einem privaten Repository. Für
diese Zeit stehen hier keine Einträge, weil es in ihr keine öffentlichen
Freigaben gab; welche zu erfinden, wäre weniger wert als nichts. Der Abschnitt
„Versionen" in der README sagt dasselbe noch einmal aus der anderen Richtung.

### Was 1.4 ist

Ein selbst gehostetes CMS als einzelnes Go-Binär, ohne CGO, mit SQLite als
Ablage. Eine Installation trägt mehrere Websites mit je mehreren Domains, streng
voneinander getrennt. Seiten entstehen in Markdown oder aus Bausteinen; welche
Felder, Bausteinarten und Inhaltsarten eine Website kennt, bestimmt sie selbst,
ohne dass dafür eine neue Programmfassung nötig wäre. Dazu Vorlagen zum
Hochladen, eine mehrsprachige öffentliche Website und eine mehrsprachige
Verwaltung, Medien, Menüs, SEO, ein zwingender zweiter Faktor für
Verwaltungskonten sowie Ausfuhr und Einfuhr einer ganzen Website als lesbares
Archiv. Die vollständige Aufzählung steht unter „Features" in der README; sie
hier zu wiederholen hiesse, sie an zwei Orten zu pflegen.

### Hinzugefügt

Die achte eingebaute Vorlage heisst „Holzcloud" und bringt das Design von
holzcloud.ch als Theme mit, samt der Schriften, die dazugehören. Die Schriften
liegen in der Vorlage selbst und werden mit ihr ausgeliefert; auch dieses Theme
lädt im Betrieb nichts von einem fremden Server nach.

### Geändert

**Gebaut wird nur noch für linux/amd64.** arm64 und der Raspberry Pi sind aus
dem Bauplan und aus der Beschreibung entfernt. Wer bisher eine arm-Fassung
erwartet hat, bekommt keine mehr und muss selbst bauen. Dafür veröffentlicht ein
Freigabe-Ablauf auf jedem `v*`-Tag ein fertiges Binär samt Prüfsumme, statt dass
jede Installation es von Hand übersetzt.

Die Verwaltung zeigt jetzt den nach AGPL §13 vorgeschriebenen Hinweis: die
laufende Fassung und den Verweis auf den Quelltext, in allen fünf
Oberflächensprachen. Wer diese Software für andere betreibt, erfüllt damit eine
Pflicht, die vorher offen war.

### Sicherheit

Go ist auf 1.26.6 angehoben. Das schliesst acht Lücken in der
Standardbibliothek, die dieses Programm tatsächlich erreicht hat; `govulncheck`
meldet danach keine mehr. Der Aufruf läuft ab jetzt als eigener Schritt im
Sicherheits-Ablauf mit, damit die nächste Lücke nicht erst bei einer
Freigabeprüfung auffällt. Das ist der Grund, eine ältere Fassung nicht
weiterzubetreiben.
