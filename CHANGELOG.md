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
