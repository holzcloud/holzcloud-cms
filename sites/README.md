# Websites als Quelltext

Hier liegt eine fertige Website in der Form, in der man sie lesen und ändern
kann: das Manifest als JSON, die Bilder als gewöhnliche Dateien.
`tools/mkbundle` packt daraus das Zip-Archiv, das der Import im Admin erwartet.

Ein Bundle wird nicht als Zip eingecheckt, obwohl das die Form ist, in der es
am Ende gebraucht wird. Ein Zip lässt sich nicht im Diff lesen, und eine
korrigierte Zeile Text schreibt dreissig Megabyte Binärdaten neu. Unverpackt
ist ein Tippfehler eine Zeile im Diff.

## Was hier liegt

`beispiel/` — die Velowerkstatt Beispiel in Musterhausen. Es gibt sie nicht.
Adresse, Preise, Öffnungszeiten und beide Bilder sind erfunden, und
`example.com` ist eine Domain, die nach RFC 2606 nie jemandem gehört.

Sie steht hier, damit man das Format an etwas Vollständigem sieht statt an
einer Aufzählung von Feldern. Jedes Stück kommt genau einmal vor: vier
veröffentlichte Seiten, ein Menü mit einem Unterpunkt, ein Textbaustein, zwei
Bilder mit Beschreibung, ein Titelbild und zwei Pfade der Form `/media/0/`.
Wer eine eigene Website anlegt, kopiert das Verzeichnis und schreibt es um.

Welche Felder es überhaupt gibt, steht nicht hier, sondern in
`internal/bundle/format.go`. Die JSON-Namen an den Strukturen sind die einzige
Wahrheit darüber, und `mkbundle` weist jeden Namen zurück, der dort nicht
vorkommt. Eine Aufzählung an dieser Stelle wäre eine zweite Wahrheit, die
veraltet, sobald jemand ein Feld hinzufügt.

## Bauen und einspielen

```sh
go run ./tools/mkbundle sites/beispiel
```

Das schreibt `sites/beispiel.zip`. Das Archiv gehört nicht ins Repository und
steht in `.gitignore`. Danach im Admin unter **Websites → Website importieren**
hochladen. Der Import legt jedes Mal eine **neue** Website an; er führt nichts
zusammen. Wer eine Fassung ersetzen will, importiert neu, vergleicht und löscht
die alte.

Nach dem Import fehlt noch die Domain: eine importierte Website hat keine, und
ohne Domain ist sie nicht erreichbar. Unter **Domains** eintragen. Die Domains
stehen bewusst nicht im Manifest — ein Bundle soll anderswo landen können, ohne
einen Hostnamen zu beanspruchen, der dieser Maschine nicht gehört.

Ebenfalls von Hand zu setzen ist das Theme. Es steht nicht im Manifest, weil
ein Bundle auf einer Maschine landen kann, die dieses Theme gar nicht hat.

## Was `mkbundle` prüft

Es weigert sich, ein Archiv zu bauen, das beim Import stillschweigend Inhalt
verlöre:

- **Unbekannte Felder.** Der Import ist absichtlich nachsichtig — er nimmt, was
  er kennt, damit ein altes Bundle noch landet. Hier gilt das Gegenteil:
  `meta_desc` statt `meta_description` würde jede Beschreibung der Website
  kosten, ohne eine einzige Fehlermeldung.
- **Bilder in beide Richtungen.** Jede Datei in `media/` steht im Manifest, und
  jeder Eintrag im Manifest liegt in `media/`. Ein nicht eingetragenes Bild
  reist nicht mit; die Seite, die es zeigt, bleibt auf der Zielmaschine leer.
- **Fehlende Alt-Texte.** Eine Bildbeschreibung lässt sich später nachtragen,
  aber niemand tut es. Beim Bauen ist der Moment, in dem es noch jemanden gibt,
  der das Bild vor Augen hat.
- **Zeichen, die eine Bildbeschreibung ganz verlieren.** Ein Doppelpunkt, ein
  Fragezeichen, ein Gedankenstrich, ein kaufmännisches Und — bei jedem davon
  entfernt die Bereinigung nicht das Zeichen, sondern das ganze Attribut. Das
  Bild erscheint weiterhin, die Seite sieht fertig aus, und für einen
  Screenreader ist sie stumm. Komma und Punkt sind erlaubt und reichen.
- **Verweise ins Leere.** Menüpunkte auf Seiten, die es nicht gibt.
  Titelbilder, die in keiner Medienliste stehen. `/media/…`-Pfade im Text, zu
  denen keine Datei gehört.

Die Prüfsummen schreibt es beim Packen selbst. Eine eingecheckte Prüfsumme wäre
eine zweite Kopie der Wahrheit, die veraltet, sobald jemand ein Foto neu
zuschneidet — und dann scheitert der Import mit einer Meldung über eine
Beschädigung, die es gar nicht gibt. Dasselbe gilt für `version`,
`exported_at` und `generated_by`: von Hand geschrieben werden sie nicht.

## Bilder im Text

Bilder werden als `/media/0/dateiname.jpg` geschrieben. Die Null ist ein
Platzhalter: welche Nummer die Website bekommt, entscheidet sich erst beim
Import, und der schreibt die Pfade dann auf die richtige um. Das gilt für jede
Zahl an dieser Stelle, auch für die einer echten Website — genau das ist es,
was einen Export von einem Server auf einem anderen wieder funktionieren lässt.

Umgeschrieben werden nur Dateien, die im Bundle stecken. Ein Verweis auf etwas
anderes bleibt stehen.

## Die zwei Bilder im Beispiel

Sie zeigen nichts. Es sind zwei flächige JPEGs, 1200 mal 800, mit ein paar
Rechtecken darauf, damit man sie auseinanderhalten kann. Erzeugt hat sie ein
Wegwerfprogramm aus `image`, `image/color`, `image/draw` und `image/jpeg` —
nur Standardbibliothek, weshalb sie JFIF ohne EXIF-Block sind und nichts über
die Maschine verraten, auf der sie entstanden.

Das Programm ist nicht eingecheckt. Es hätte genau eine Aufgabe, die sich nie
wiederholt, und ein zweiter Lauf würde die Bytes und damit jede Prüfsumme still
neu schreiben. Ein Bild ist Inhalt, kein Bauergebnis — ein Foto erzeugt auch
niemand neu. Wer die zwei doch einmal ersetzen muss, schreibt das Programm in
zehn Minuten wieder.

## Ändern

Text und Struktur stehen in `holzcloud.json`. Wer dort etwas ändert, baut neu
und importiert erneut — es gibt bewusst keinen Weg, eine laufende Website aus
dieser Datei nachzuziehen. Das wäre eine zweite Quelle der Wahrheit neben dem
Admin, und die redaktionellen Änderungen der letzten Wochen wären still
überschrieben.

Nach dem ersten Import ist der Admin die Quelle. Diese Dateien sind der
Startpunkt und das Archiv, aus dem man wieder anfangen kann.

## Wo die echten Websites liegen

Hier standen bis zur Freigabe des Quelltextes zwei Websites von Kunden, mit
ihren Fotos, ihren Texten, einer Mailadresse und einer Postadresse. Das ist
nicht das Material dieses Projekts, und dieses Repository steht unter der
AGPL-3.0 — mit veröffentlicht wird also alles, was darin liegt.

Beide sind deshalb ins private Repository `holzcloud/holzcloud-sites`
umgezogen. Dort gehören sie hin, und dort werden sie weiter gepflegt. Was hier
bleibt, ist das Format und ein Beispiel dafür.
