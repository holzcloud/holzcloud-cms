---
quick_id: 260915-w1n
slug: fassungsnummer-fuehrt-zu-den-neuerungen
date: 2026-09-15
source: Wunsch des Betreibers, im Lauf angemeldet
mode: direkt
commits: [fc702f6]
---

# Quick 260915-w1n — die Fassungsnummer in der Seitenleiste führt zu dem, was sich geändert hat

## Was gewünscht war

Wörtlich: *„ich will in der webui links unten immer eine versionsnummer haben,
und wenn ich auf die versionsnummer klicke, kommt ein screen mit allem was neu
ist."* Dazu ein Bild aus einem anderen Programm: ein Kasten „What's New" mit
Reitern je Fassung.

## Was gebaut wurde

Die Fassungsnummer **stand schon** unten links (`base.html`, `.sidebar-build`) —
sie ist seit der ersten Fassung dort, weil Abschnitt 13 der AGPL einen Verweis
auf den Quelltext verlangt. Neu ist, dass sie ein Verweis ist und wohin er
führt.

- `changelog.go` **im Wurzelverzeichnis**: ein Paket, dessen einzige Aufgabe
  `//go:embed CHANGELOG.md` ist. `go:embed` kommt nicht aus seinem eigenen
  Verzeichnis heraus und lehnt einen Pfad mit `..` ab, und CHANGELOG.md gehört
  an die Wurzel — dorthin schaut GitHub, und dorthin schickt CONTRIBUTING.md
  jeden, der einen Eintrag schreibt. Die Alternative — eine Kopie unter
  `internal/`, von einem `-check`-Werkzeug in Gleichschritt gehalten — ist die
  richtige Form für etwas Erzeugtes und die falsche für etwas von Hand
  Geschriebenes: eine zweite Kopie ist eine zweite Sache zum Vergessen.
  `internal/tmplspec` macht es umgekehrt, und das ist für ein Dokument richtig,
  das niemand an der Wurzel erwartet.
- `internal/changelog`: teilt die Datei an ihren `## `-Überschriften, rendert
  jeden Eintrag durch `page.RenderMarkdown` (goldmark, dann bluemonday — es gibt
  hier bewusst keinen zweiten Weg zu `template.HTML`), beim ersten Aufruf und
  nicht beim Start.
- `internal/admin/changelog.go` und `changelog.html`: der Bildschirm.
  `/admin/neuerungen` ist die neueste Fassung, `/admin/neuerungen/2.1` jene —
  eine Adresse, die man jemandem schicken kann, der noch auf der älteren sitzt.
- Angemeldet sein genügt. Was sich zwischen zwei Fassungen geändert hat, ist
  vor einer Redakteurin kein Geheimnis; sie ist diejenige, der der Unterschied
  auffällt.

## Zwei Entscheidungen, die anders ausfielen als das Vorbild

**Kein Kasten, der sich meldet.** Das Vorbild öffnet sich nach einer
Aktualisierung von selbst und hat einen „Got it"-Knopf. Hier meldet sich nichts:
kein Abzeichen, kein Punkt, kein Dialog. Ein selbst betriebenes Programm, das
seine Bedienerin unterbricht, um über sich selbst zu reden, lernt sie
wegzuklicken — und dann ist auch der Hinweis weg, der einmal wichtig gewesen
wäre.

**Verweise statt Reiter.** Die Fassungen sind gewöhnliche `<a>`-Elemente mit
eigener Adresse, keine Reiter aus Javascript. Ohne Skript funktioniert die Seite
genauso, was in diesem Projekt die Regel und nicht die Ausnahme ist.

## Was offen bleibt

Die Einträge sind auf Deutsch und werden nicht übersetzt. Das steht im
Paketkommentar von `internal/changelog` ausdrücklich da, statt verschwiegen zu
werden: ein Changelog ist ein Dokument und kein Katalog von
Oberflächenzeichenketten, und 44 KB Prosa durch `i18n.N` zu schicken wäre weder
machbar noch ehrlich. Die Umrandung — Überschrift, Fassungsliste, leerer Fall —
geht wie alles andere durch den Katalog.

## Gemessen

Im laufenden Programm gefahren (QUAL-02): Binär gebaut, Server gestartet, als
Redakteurin angemeldet, `/admin/neuerungen` im Browser aufgenommen. Zwei Fehler
fielen dabei auf und sind behoben:

1. Die Überschrift stand doppelt — `base.html` setzt sie schon aus `.Title`. Die
   Vorlage folgt jetzt `dashboard.html`: `{{define "content"}}` reicht an
   `{{define "changelog-content"}}` weiter, was zugleich der Weg ist, auf dem
   htmx den Teiltausch findet.
2. Das CSS griff nach `--color-primary`; dieses Projekt heisst den Farbton
   `--color-accent`. Eine unbekannte Eigenschaft fällt still aus, also hätte der
   Rahmen der gewählten Fassung einfach gefehlt.

`TestEveryAdminRouteIsClassified` verlangte ausserdem, dass die beiden neuen
Adressen eingeordnet werden — sie stehen in `editorOpenRoutes`.
