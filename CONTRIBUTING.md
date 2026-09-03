# Mitarbeiten

Holzcloud CMS ist ein CMS für einen kleinen Server im Regal oder im Cluster.
Was hier gebaut wird, soll auf dem billigsten Knoten flüssig laufen und ohne
Betreuung jahrelang durchhalten. Das ist keine Einschränkung, die man nebenbei
erwähnt — es ist die Entscheidung, aus der die meisten anderen folgen.

## Was Sie brauchen

Go 1.22 oder neuer. Sonst nichts.

```bash
go build ./cmd/holzcloud
go test ./...
./holzcloud
# http://localhost:8080/admin öffnen — dort wird das erste Konto angelegt
```

Kein npm, kein Bundler, keine Datenbank zum Aufsetzen: SQLite entsteht beim
ersten Start, die Migrationen laufen von selbst.

## Die harten Grenzen

Diese vier Regeln sind nicht verhandelbar. Ein Vorschlag, der eine davon
verletzt, wird abgelehnt, egal wie gut er sonst ist.

**Kein JavaScript ausser htmx.** htmx 2.0 liegt im Repository und wird vom
eigenen Server ausgeliefert. Sonst nichts. Jede Seite muss ohne JavaScript
vollständig bedienbar sein — htmx macht sie angenehmer, nicht benutzbar.

**Zur Laufzeit wird nichts nachgeladen.** Kein Skript, kein Stylesheet, keine
Schrift, kein Bild von einem fremden Server. Alles, was der Browser anfordert,
kommt vom eigenen Ursprung. Beim Übersetzen dürfen zwei Dinge geholt werden:
Go-Module und Schriften — und eine Schrift wird ins Repository gelegt und über
`embed.FS` mitgeliefert, nie über eine Adresse eingebunden.

Durchgesetzt an drei Stellen, die alle funktionieren müssen:
`internal/web/headers.go` (die Inhaltsrichtlinie), `internal/tmplmgr/external.go`
(fremde Quellen in hochgeladenen Vorlagen) und `internal/tmplmgr/script.go`
(Skripte darin).

**Ein Programm, keine Beilagen.** Vorlagen, Anlagen und Migrationen stecken
über `embed.FS` im Programm. Eine Datei, die zur Laufzeit danebenliegen muss,
ist eine Datei, die beim Aktualisieren vergessen wird.

**Kein CGO.** SQLite kommt über `modernc.org/sqlite`, damit sich ohne
C-Werkzeugkette ein statisches Programm übersetzen lässt.

## Wie hier Code aussieht

**Kommentare sagen warum, nicht was.** Der Code sagt schon, was er tut. Was er
nicht sagen kann, ist, welcher Fehler diese Zeile nötig gemacht hat. Solche
Kommentare stehen überall im Projekt, und sie sind der Grund, warum man nach
einem halben Jahr noch versteht, warum etwas so und nicht anders ist.

**Tests, die beissen.** Ein Test, der auch dann grün bleibt, wenn man die
geprüfte Zeile kaputt macht, prüft nichts. Machen Sie sie kaputt und sehen Sie
nach. Mehrere Tests in diesem Projekt sind erst dadurch entstanden, dass der
erste Entwurf einen absichtlich eingebauten Fehler nicht bemerkt hat.

**Migrationen werden nie geändert, sobald sie angewendet wurden.** Neue Spalte,
neue Datei. Eine geänderte Migration läuft auf einer bestehenden Installation
nicht noch einmal — dort fehlt die Spalte dann einfach.

**Fehler werden angezeigt, nicht verschluckt.** Eine leere Auswahl, eine
stillschweigend übersprungene Vorlage, ein verschluckter Rückgabewert: das sind
die Fehler, die es bis in den Betrieb schaffen. Lieber ein Start, der abbricht.

## Wer diesen Code geschrieben hat

Ein grosser Teil dieses Projekts ist von einem KI-Agenten geschrieben worden,
unter Anleitung und Durchsicht einer einzelnen Person. Sie sollen es hier lesen
und nicht selbst herausfinden müssen.

Nachzählen lässt es sich hier nicht. Die Commits, die den Agenten als Autor
tragen, liegen im privaten Repository, aus dem dieses hier veröffentlicht wurde;
öffentlich beginnt die Aufzeichnung bei `v1.4` — der Abschnitt „Versionen" in
der README sagt, warum. Es steht hier also die Aussage und kein Beleg dazu. Das
ist genau der Grund, warum sie überhaupt dasteht.

Am Massstab ändert das nichts. Es gilt, was zwei Abschnitte weiter oben steht:
Kommentare sagen warum, und ein Test, der einen absichtlich eingebauten Fehler
nicht bemerkt, prüft nichts. Ein Modell hält sich daran nicht von allein — das
ist der Teil, den die Durchsicht leistet.

Ihre Beiträge dürfen ebenso mit einem Modell entstehen. Wer einen Pull Request
öffnet, steht dafür ein, dass er tut, was er behauptet, und dass die Rechte
daran wie im Abschnitt „Lizenz" beschrieben übertragen werden können. Ein
Beitrag, den vor dem Abschicken niemand gelesen hat, fällt auf.

## Vorlagen

Wer eine Vorlage bauen will, braucht keinen Beitrag zum Code. Die vollständige
Beschreibung des Datenvertrags steht in `internal/tmplspec/TEMPLATE-SPEC.md`,
und das Programm gibt sie selbst aus:

```bash
./holzcloud template spec           # die Beschreibung
./holzcloud template check ./meine-vorlage   # prüfen, ohne etwas zu installieren
```

Die Beschreibung ist ausdrücklich auch für KI-Agenten gedacht: sie ist so
geschrieben, dass ein Modell sie wörtlich befolgen kann. Tests binden sie an den
Code — ein Feld, das im Vertrag steht, aber nicht in der Beschreibung, lässt die
Testsuite fehlschlagen.

## Bevor Sie einen Pull Request öffnen

```bash
gofmt -l internal/ cmd/     # muss leer sein
go vet ./...
go test ./...
for t in default journal magazine midnight schlicht; do
    go run ./cmd/holzcloud template check cmd/holzcloud/templates/public/$t
done
```

Für eine grössere Änderung: bitte vorher einen Issue aufmachen. Es ist für alle
angenehmer, sich über die Richtung zu einigen, bevor jemand einen Abend
investiert hat.

## Sprache

Alles, was Nutzerinnen und Nutzer zu sehen bekommen, ist auf Deutsch —
Beschriftungen, Fehlermeldungen, E-Mails. Die Vorlagen-Beschreibung ist auf
Englisch, weil sie sich an ein internationales Publikum und an Sprachmodelle
richtet. Kommentare im Code: beides kommt vor, schreiben Sie in der Sprache, in
der Sie sich genauer ausdrücken können.

## Lizenz

Mit einem Beitrag stellen Sie ihn unter die
[GNU AGPL-3.0](LICENSE), unter der auch das übrige Projekt steht.
