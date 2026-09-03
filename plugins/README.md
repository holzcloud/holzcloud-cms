# Plugins

Was hier liegt, sind die mitgelieferten Beispiele. Ein Plugin ist ein eigenes
Go-Modul und muss nicht in diesem Repository stehen — genau das ist der Punkt:
wer eine Funktion braucht, die es nicht gibt, schreibt sie, ohne das CMS zu
forken.

| Verzeichnis | Was es tut | Haken |
|---|---|---|
| `jahreszahl/` | ersetzt `[[jahr]]` im Text durch das laufende Jahr | `content`, `admin` |
| `nicht-gefunden/` | führt Buch über Adressen, die angefragt und nicht gefunden wurden | `event`, `admin` |
| `suche/` | die Volltextsuche unter `/suche` | `route` |
| `kontaktformular/` | `[[formular]]` wird zum Formular, `/formular` nimmt es entgegen; eigene Formulare mit `[[formular:kennung]]` | `content`, `route`, `admin` |
| `bestellung/` | der Hofladen: `[[bestellung]]` wird zur Produktliste mit Mengenfeldern, `/bestellung` nimmt sie entgegen | `content`, `route`, `admin` |

Die letzten drei standen einmal im Kern. Sie sind ausgezogen, weil eine Website
sie nicht braucht, um eine Website zu sein — und was man nicht braucht, soll man
nicht mitschleppen müssen. Wer sie will, spielt sie ein; wer nicht, hat sie
nirgends: keine Route, keine Tabelle, kein Bildschirm, kein Aufruf.

Drei Dinge sehen nach Zusatzfunktion aus und sind bewusst im Kern geblieben:

- **Weiterleitungen** werden beim Umbenennen einer Seite in derselben
  Transaktion geschrieben wie die Umbenennung selbst. Ein Plugin erfährt erst
  hinterher davon und könnte die Meldung verpassen. Das ist kein Merkmal,
  sondern der Schutz davor, dass das CMS seine eigenen Adressen zerbricht.
- **Schlagwörter** sind Teil davon, wie eine Seite geschrieben wird: das Feld
  steht im Editor, die Werte erreichen jedes Theme, das Archiv gruppiert
  danach, der Export trägt sie mit. Ein Plugin erreicht keine dieser Stellen.
- **Export/Import** bräuchte Schreibrechte auf Seiten, Medien, Schlagwörter,
  Menüs, Textbausteine und Einstellungen — also auf alles. Eine Sandbox, die
  alles herausgibt, ist keine.

Die Regel dahinter: ausgelagert wird, was eine Website nicht braucht, um eine
Website zu sein. Nicht, was sich technisch auslagern lässt.

Der **Hofladen** ist das Beispiel dafür, wie weit das trägt: er bringt kein
Produktmodell mit, sondern liest die eigenen Felder der Website. Ein Produkt ist
eine Seite, die ein Preisfeld ausgefüllt hat — Bild, Beschreibung und Adresse
hat sie ohnehin. Das Plugin steuert nur bei, was ohne Bestellungen niemand
braucht: das Formular, die Bestellungen und den Bildschirm, auf dem sie liegen.

## Ein Plugin bauen

```sh
cd plugins/jahreszahl
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o plugin.wasm .
zip -j jahreszahl.zip plugin.json plugin.wasm
```

Das Zip wird im Admin unter *Plugins* hochgeladen. Es enthält:

| Datei | Pflicht | Wozu |
|---|---|---|
| `plugin.json` | ja | Kennung, Haken, Adressen, Berechtigungen |
| `plugin.wasm` | ja | das Modul |
| `assets/…` | nein | Dateien, die unter `/plugin-assets/<kennung>/` ausgeliefert werden |
| `migrations/*.sql` | nein | eigene Tabellen, einmalig angewendet |

## Eine Datei übergeben

Ein Verwaltungsbildschirm kann statt HTML eine Datei zurückgeben:

```go
return plugin.AdminOut{Download: &plugin.Download{
    Filename:    "nachrichten-2026-08-30.csv",
    ContentType: "text/csv",
    Body:        string(tabelle),
}}, nil
```

Der Host reicht sie als Anhang weiter und nicht zum Anzeigen. Erlaubt sind
`text/csv`, `text/plain`, `text/markdown` und `application/json`; alles andere
wird zu `application/octet-stream`. Kein `text/html` — ein Plugin, das unter der
Adresse der Verwaltung eine Seite ausliefern könnte, könnte dort eine Seite
hinstellen, die wie die Verwaltung aussieht und es nicht ist. Der Dateiname wird
auf einen einfachen Namen zurückgeschnitten, die Grösse ist auf 8 MB begrenzt.

## Was ein Plugin darf

Nur, was im Manifest steht. Ein Haken, der nicht genannt ist, wird nie
aufgerufen; eine Berechtigung, die nicht dasteht, wird verweigert. Das ist
keine Dokumentation, sondern eine Prüfung bei jedem Aufruf.

### Haken

| Haken | Wann | SDK |
|---|---|---|
| `content` | vor dem Ausliefern einer Seite, darf den Text ändern | `OnContent` |
| `request` | vor dem Zuordnen **jeder** öffentlichen Anfrage | `OnRequest` |
| `route` | für die Adressen, die das Manifest beansprucht | `OnRoute` |
| `notfound` | erst wenn der Kern nichts gefunden hat, vor der 404-Seite | `OnNotFound` |
| `admin` | für den eigenen Bildschirm in der Verwaltung | `OnAdmin` |
| `event` | nachdem etwas passiert ist; ändert nichts mehr | `OnEvent` |

`notfound` statt `request`, wann immer es geht: `request` läuft bei jedem
Seitenaufruf, `notfound` nur bei einem Fehlgriff. Eine Weiterleitungstabelle
gehört an den zweiten Haken, nicht an den ersten.

### Berechtigungen

| Berechtigung | Erlaubt | SDK |
|---|---|---|
| `store` | der eigene Schlüssel/Wert-Speicher, je Website getrennt | `Get`, `Set`, `Delete`, `List` |
| `pages:read` | die **veröffentlichten** Seiten lesen und durchsuchen, wahlweise mit den eigenen Feldern der Website | `Pages`, `Posts`, `GetPage`, `SearchPages`, `PagesWithFields` |
| `settings` | die Einstellungen der Website lesen | `Site` |
| `render` | eine öffentliche Seite im Theme der Website ausgeben | `Render` |
| `notify` | eine Benachrichtigung an den Betreiber schicken — nie an eine selbst gewählte Adresse | `Notify` |
| `log` | ins Server-Protokoll schreiben | `Log`, `Logf` |

Entwürfe sieht ein Plugin nie — auch nicht mit `pages:read`, auch nicht auf
seinem eigenen Verwaltungsbildschirm. Diese Grenze zieht der Host, nicht das
Plugin.

`render` ist die Berechtigung, die ein Plugin auf die öffentliche Website
bringt: es liefert Titel und Rumpf, Kopf, Menü, Schriften und Fuss kommen vom
Theme. Was es liefert, geht durch denselben Filter wie der Text eines
Redakteurs — ein Skript überlebt ihn nicht.

`notify` schickt an die Adresse, die in den Einstellungen der Website steht,
und an keine andere. Ein Plugin, das den Empfänger nennen dürfte, wäre ein
Mail-Relay mit Web-Oberfläche.

Ein Bildschirm in der Verwaltung darf mehrere Ansichten haben: das Plugin
bekommt die Abfragezeichenkette und kann auf sich selbst verlinken. Sein HTML
wird gefiltert, bevor es angezeigt wird — Formulare, Tabellen und Auswahlfelder
überstehen das, alles Ausführbare nicht.

Ein Formular auf diesem Bildschirm sendet an dieselbe Adresse zurück; den
Sitzungsschlüssel setzt der Host nach der Filterung in jedes Formular ein. Ein
Plugin bekommt ihn nie zu sehen und braucht ihn auch nicht — es schreibt
einfach `<form method="POST">`.

`PagesWithFields` liefert zusätzlich die eigenen Felder einer Website, so wie
sie gespeichert sind: ein Bild als Kennzahl, eine Zahl als der Text, den jemand
getippt hat. Ausdrücklich anzufordern und nicht immer dabei — eine Liste von
hundert Seiten mit allen Zusatzangaben wäre ein zweiter Nutzinhalt durch eine
Sandbox, die sechzehn Megabyte insgesamt hat.

Ein Plugin läuft in einer Sandbox. Es kann die Platte nicht lesen, keinen
Socket öffnen und die Datenbank nur durch die Host-Funktionen erreichen, die
das SDK anbietet. Eine Endlosschleife wird nach zwei Sekunden abgebrochen, ein
Absturz abgefangen — in beiden Fällen läuft die Anfrage weiter, als hätte das
Plugin nichts gesagt, und der Grund steht im Admin beim Plugin.

## Die Grösse

Ein Go-Plugin wiegt rund drei Megabyte, fast vollständig Go-Laufzeit. Das ist
der Preis dafür, dass ein Autor keine zweite Werkzeugkette braucht. Mit TinyGo
wären es etwa 150 KB; das SDK funktioniert dort ebenso, verlangt aber eine
zusätzliche Installation.
