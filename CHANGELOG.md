# Changelog

This file records what changed between two versions for somebody who runs this
software. It follows the shape of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) — newest release at the
top, one heading per version with a number and a date, groups below it — but
writes the entries in whole sentences rather than in bullet fragments. That is
deliberate: every other document in this project explains the why as well, and a
list of clipped half-sentences would read as though somebody else had written it.
Whoever writes the next entry, please join in.

The numbers are the same as the tags in the repository.

## 2.9 — 2026-09-26

**Anmelden über OpenID Connect.** Neben der Anmeldung über Authentik mit
Forward-Auth gibt es jetzt einen zweiten Weg über den eigenen
Identitätsanbieter: Das Anmeldeformular bekommt einen Knopf „Mit Authentik
anmelden“ (der Name ist einstellbar), und es funktioniert mit Authentik,
Keycloak, Zitadel und jedem anderen Anbieter, der OpenID Connect spricht. Ein
Reverse-Proxy mit Outpost und gemeinsamem Geheimnis ist dafür nicht nötig.

### Neu

**Der Server fragt den Anbieter nie.** Der übliche Weg tauscht einen Code beim
Anbieter gegen ein Token und holt dessen Schlüssel ab — beides Anfragen an einen
fremden Server zur Laufzeit, und genau das tut dieses Programm nicht. Stattdessen
schickt der Anbieter dem Browser ein signiertes ID-Token, der Browser bringt es
zurück (`response_type=id_token`, `response_mode=form_post`), und der Server
prüft die Signatur mit einem Schlüssel, den der Betreiber einmal ablegt: das
JWKS-Dokument oder ein PEM-Schlüssel des Anbieters (RS256, ES256), oder das
Client-Secret für einen Anbieter, der damit signiert (HS256). Geprüft werden
Aussteller, Empfänger, Ablauf und eine Nonce, die nur für diese eine Anmeldung
gilt; `none` und nicht eingestellte Verfahren werden abgelehnt. Die Prüfung ist
eigener Code mit der Standardbibliothek, ohne neue Abhängigkeit.

**Dieselben Regeln wie bei Forward-Auth.** Welches Konto, welche Rolle, welche
Websites und ob ein neues Konto angelegt werden darf, entscheidet derselbe Code
mit denselben Einstellungen (`HOLZCLOUD_SSO_ADMIN_GROUP`,
`HOLZCLOUD_SSO_WEBSITE_GROUPS`, `HOLZCLOUD_SSO_PROVISION`,
`HOLZCLOUD_SSO_DEFAULT_WEBSITE`). Verknüpft wird über den Benutzernamen
(`preferred_username`), also ist ein Konto, das schon für Forward-Auth verknüpft
war, auch hier dasselbe. Das Protokoll schreibt dieselben Zeilen, gekennzeichnet
mit `via: oidc`. Der zweite Faktor ist, wie bei Forward-Auth, der des Anbieters.

**Nach der Anmeldung geht es dorthin, wohin man wollte**, auch über den Umweg
zum Anbieter.

### Anders als Forward-Auth

Die Rechte werden bei der Anmeldung gelesen und nicht bei jedem Klick — bei
späteren Anfragen kommt nichts an, woran sie sich prüfen liessen. Eine Änderung
der Gruppen beim Anbieter wirkt ab der nächsten Anmeldung; eine Sitzung hält
höchstens einen Tag. Abmelden hier meldet beim Anbieter nicht ab. Schaltet man
OpenID Connect aus, enden alle Sitzungen, die darüber zustande kamen.

### Für Betreiber

Elf Einstellungen `HOLZCLOUD_OIDC_*`, alle wirkungslos, solange
`HOLZCLOUD_OIDC_ENABLED` aus ist; eine halbe Konfiguration verhindert den Start
und nennt die fehlende Variable. Die Anleitung für Authentik steht in
`deploy/DEPLOY.md`, die Liste in `docs/configuration.md`, die Begründung in
`docs/security.md`. Wechselt der Anbieter seinen Signaturschlüssel, muss die
Datei ersetzt werden; bis dahin wird die Anmeldung über den Anbieter abgelehnt,
das Passwort funktioniert weiter. Keine Datenbankänderung.

## 2.8.4 — 2026-09-26

**Bild und Text läuft im Theme *holzcloud* über die ganze Breite.** Seit 2.8.3
bekommt das Bild drei Fünftel des Bausteins, aber der Baustein selbst stand
noch in der Textspalte von 78 Zeichen, und rechts davon blieb ein Drittel der
Seite leer. Jetzt reicht er wie eine Galerie bis an den rechten Rand.

## 2.8.3 — 2026-09-26

**Die Themes *holzcloud* und *weide* zeigen wieder ihr eigenes Favicon.** Beide
Dateien trugen einen Kommentar, in dem `--` stand, und das ist in einem
XML-Kommentar nicht erlaubt. Der Browser verwarf das Bild ohne ein Wort; eine
Website auf diesen Themes hatte kein Zeichen im Tab, und wo die Verwaltung unter
derselben Domain läuft, erschien stattdessen deren Wolke. Ein Test parst jetzt
jedes eingebettete SVG. Wer ein eigenes Favicon hochlädt, war nie betroffen.

**Bild und Text taugt im Theme *holzcloud* für Bildschirmfotos.** Das Bild
bekommt drei Fünftel der Breite statt der Hälfte, eine grössere Rundung und
einen Schatten in der Akzentfarbe, und die Zeilen stehen mit mehr Luft
untereinander. Ein Bild in einer Karte trägt die Rundung der Karte und ist im
Seitenverhältnis 16:10 von oben links angeschnitten, wie ein Bildschirmfoto.

## 2.8.2 — 2026-09-26

**Ein langes Menü lässt sich am Telefon wieder ganz durchblättern.** In den
Themes *rudel*, *weide* und *default* klebt die Kopfleiste oben. Klappte man
auf einem Telefon ein Menü auf, das höher war als der Bildschirm, scrollte ein
Wischen die Seite dahinter — die unteren Punkte blieben unerreichbar. Jetzt
wird die offene Leiste selbst zum Bildlaufbereich, höchstens so hoch wie der
sichtbare Bildschirm, und der Bildlauf geht nicht an die Seite weiter.

## 2.8.1 — 2026-09-26

**Ein langer Website-Name bricht die Kopfleiste am Telefon nicht mehr.** Im
Theme *weide* rutschte der Menüknopf in eine eigene Zeile, sobald der Name nicht
mehr ganz neben ihn passte — bei „Kinderbauernhof Seehof“ schon auf einem
gewöhnlichen Telefon. Jetzt steht der Knopf immer rechts neben dem Namen; unter
600 px Breite ist der Name eine Stufe kleiner und bleibt so bis etwa 340 px auf
einer Zeile. Reicht der Platz auch dann nicht, bricht der Name in sich um, nicht
die Leiste.

## 2.8 — 2026-09-26

**Claude und ChatGPT verbinden sich jetzt selbst.** Wer das CMS in Claude (im
Browser oder in der App) oder in ChatGPT als MCP-Server einträgt, gibt nur noch
die Adresse `https://…/ai` an — Client-ID und Secret bleiben leer. Der Assistent
meldet sich über OAuth an, der Browser landet auf einer Seite der Verwaltung,
und dort wird zugestimmt. Bisher verlangten diese Assistenten eine Client-ID,
schickten dann auf eine Anmeldeseite, und die gab es nicht: 404.

### Neu

**Die Zustimmungsseite** unter *KI-Zugang* nennt, welcher Assistent fragt und
wohin der Browser danach zurückgeht, und lässt wählen, für welche Website die
Verbindung gilt und ob sie nur lesen oder auch schreiben darf. Sie ist nur für
Administratoren, und die Zustimmung verlangt das Passwort noch einmal — wie das
Ausstellen eines Schlüssels von Hand. Verwalten (Benutzer, Plugins, Schlüssel)
wird auf diesem Weg nie vergeben.

**Die Verbindung ist ein gewöhnlicher Schlüssel.** Sie steht in der Liste unter
*KI-Zugang*, gekennzeichnet als „selbst angemeldet“, und endet, wenn man sie
dort zurückzieht. Der Schlüssel selbst gilt eine Stunde und wird vom
Assistenten erneuert; jedes Erneuern ersetzt auch das Erneuerungsgeheimnis, so
dass eine Kopie davon genau einmal funktioniert. Eine Verbindung, die neunzig
Tage lang nicht benutzt wurde, erneuert sich nicht mehr.

**Nach der Anmeldung geht es dorthin, wohin man wollte.** Wer eine Adresse der
Verwaltung aufruft, ohne angemeldet zu sein, landet nach Passwort und zweitem
Faktor wieder dort und nicht auf der Startseite. Für die Zustimmungsseite ist
das nötig — sie trägt die ganze Anfrage des Assistenten in ihrer Adresse —, und
für jeden Link auf eine Seite der Verwaltung angenehm.

### Für Betreiber

Umgesetzt ist, was diese Assistenten brauchen, und nicht mehr: dynamische
Registrierung (RFC 7591), der Autorisierungscode mit PKCE (nur S256),
Erneuerung mit wechselndem Geheimnis und die beiden Metadaten-Dokumente unter
`/.well-known/` (RFC 8414, RFC 9728). Kein Weg führt an der Zustimmungsseite
vorbei. Registrieren kann sich jeder Client, das ist der Sinn der Sache; ohne
Zustimmung kann er nichts, und Registrierungen ohne Schlüssel verfallen nach
einem Tag. Die Adresse für die Rückkehr muss `https` sein (oder `http` auf
`localhost`) und wird ganz verglichen, nie als Anfang.

Wanderung 00058 legt zwei Tabellen an und ergänzt `ai_tokens` um drei Spalten;
sie ist umkehrbar. Wer sich nur über den Ausweisdienst anmeldet und kein
Passwort hat, kommt wie beim Ausstellen eines Schlüssels nicht über die
Passwortabfrage — der Weg darum herum steht in `deploy/DEPLOY.md`.

## 2.7 — 2026-09-25

**Alles, was die Verwaltung kann, geht jetzt auch über die KI-Verbindung.** Wer
einen eigenen Assistenten an `/ai` anschliesst, muss sich nie mehr in die
Web-Oberfläche einloggen: Websites anlegen, Seiten und Bausteine schreiben,
Bilder hochladen, Menüs, Design, Shop, Benutzer — rund 140 Werkzeuge statt
bisher neun.

### Neu

**Schlüssel ohne Web-Oberfläche.** `holzcloud ai key create -name "…" -level
admin` erzeugt auf dem Server einen Schlüssel, `list` zeigt sie, `revoke` zieht
einen zurück. Damit beginnt eine Installation ohne einen einzigen Klick im
Browser.

**Drei Stufen.** Ein Schlüssel liest (`read`), schreibt Inhalte (`content`) oder
verwaltet zusätzlich die Installation (`admin`): Benutzer, weitere Schlüssel,
Plugins, Sprachen, Marke. Einen Admin-Schlüssel gibt es nur auf dem Server, nie
über die Verbindung selbst und nie auf einem Bildschirm. Die Schlüsselseite im
Admin zeigt Admin-Schlüssel an und sagt, wie man einen erstellt.

**Derselbe Weg wie die Bildschirme.** Wo ein Bildschirm mehr tut als eine Zeile
speichern — Cache leeren, Dateien prüfen und verkleinern, Protokoll schreiben,
Mails verschicken —, ruft das Werkzeug dieselbe Funktion auf wie der Bildschirm.
Jede Änderung steht im Protokoll unter „KI: Name des Schlüssels“. Löschen
verlangt ein ausdrückliches `confirm: true`, und ein falsch geschriebener
Parameter wird mit seinem Namen abgelehnt statt still übergangen. Dateien kommen
als base64 mit; der Server holt nie selbst etwas aus dem Netz.

### Behoben

Beim Zusammenlegen der Wege sind drei Fehler der Web-Oberfläche aufgefallen:
**Eine frühere Fassung wiederherzustellen** hat Auszug, Beschreibung,
Vorschaubild, Zeitplan und die eigenen Felder geleert und eine Baustein-Seite in
reinen Text verwandelt — jetzt bleibt alles erhalten. **Eine Seite zu
duplizieren** hat bei einer Baustein-Seite nur den Text kopiert — jetzt auch die
Bausteine und Felder. **Eine Inhaltsart ohne Übersichtsadresse** landete unter
`/untitled`. Und der Link eines Benutzers mit unbekannter Nummer brachte einen
Fehler statt „nicht gefunden“.

## 2.6 — 2026-09-24

**Die Verwaltung ist neu geordnet.** Statt siebenundzwanzig Einträgen in der
Seitenleiste gibt es sechs Orte — Start, Seiten, Medien, Shop, Design,
Einstellungen — und unten das Konto. Jeder Ort ist ein Symbol über einem kurzen
Wort, auf dem Telefon eine Leiste am unteren Rand wie in jeder App. Eine Zahl an
einem Ort sagt, was dort wartet: Seiten zur Prüfung, Bilder ohne Beschreibung
(gelb), neue Bestellungen. Nichts ist weggefallen; es steht nur dort, wo man es
sucht.

### Geändert

**Einstellungen** sind eine Liste links und der gewählte Bereich rechts, wie die
Systemeinstellungen eines Rechners. Oben steht, was für diese Website gilt —
Allgemein, Menüs, Schlagworte, Textbausteine, Weiterleitungen, Inhaltsarten,
Felder, Bausteinarten —, darunter, was für die ganze Installation gilt. Man
springt von einem Bereich zum nächsten, ohne zurückzugehen.

**Der Seiteneditor** hat zwei Spalten: links wird geschrieben, rechts stehen
Vorschau, Einstellungen und Versionen als Reiter. Wer den Reiter wechselt,
verliert nichts, weil alles ein Formular bleibt. Unter dem Text steht
„+ Element“: Bild, Galerie, Karten, Zitat, Aufruf, Video und die eigenen
Bausteinarten der Website. Der Text bis dahin wird der erste Abschnitt, das
Element folgt ihm — ein Schritt statt zwei. Zwischen zwei Abschnitten steht
jederzeit ein weiteres „+ Element“, das genau dort einfügt und nicht am Ende.
Enter in einem Feld speichert, statt einen Baustein-Knopf auszulösen.

**Medien und Alben sind eine Bibliothek.** Links stehen die Sammlungen — alle
Medien, Bilder, Videos, Dokumente, unbenutzte, Bilder ohne Beschreibung — und
darunter die Alben, jeweils mit ihrer Anzahl. Es lassen sich bis zu zwanzig
Dateien auf einmal hochladen; wer Bilder ohne Beschreibung hochlädt, landet
danach gleich bei genau diesen. Bilder werden angekreuzt und mit einem Klick in
ein Album gelegt.

**Design** zeigt die Website neben ihren Werten. „In der Vorschau zeigen“ setzt
Farben, Schrift, Textbreite und Ecken in der Vorschau ein, ohne zu speichern,
und die Vorschau lässt sich auf Telefonbreite schalten. Neben Text- und
Akzentfarbe steht der Kontrast zum Hintergrund, grün ab 4,5:1. Die Vorschau im
Admin zeigt dabei zum ersten Mal die eigenen Farben der Website — bisher zeigte
sie das Theme, wie es ausgeliefert wird.

**Der Shop hat eine Übersicht**: Umsatz des Monats, offene Bestellungen, was fast
ausverkauft ist, Produkte online; darunter, was zu tun ist — eine Vorauskasse,
die seit Tagen offen ist, eine bezahlte Bestellung, die auf den Versand wartet,
ein Artikel, von dem noch zwei da sind —, und daneben der Umsatz der letzten
acht Wochen. Bestellungen, Produkte und Shop-Einstellungen sind Reiter darüber.

**Das Konto** zeigt auf einen Blick, wie sicher es steht: Zwei-Schritt-Anmeldung,
übrige Ersatzcodes, auf wie vielen Geräten man angemeldet ist. Die Geräte stehen
darunter, jedes mit „Abmelden“ — ein verlorenes Telefon meldet man vom
Schreibtisch aus ab. Geräte, auf denen man sich vor diesem Update angemeldet
hat, heissen dort „Eine frühere Anmeldung“.

### Neu

**Eine Website lässt sich als statische Dateien ausgeben.** `holzcloud export
-website <id|domain> <verzeichnis>` schreibt sie so, wie der Server sie
ausliefert, in ein leeres Verzeichnis — bereit für jeden Host, der nur Dateien
kann.

Der Export ist kein zweiter Renderer, sondern fragt denselben Router, den der
Server betreibt, und schreibt die Antworten auf. Eine Seite sieht darum im
Export genau so aus wie im Browser, und ein Theme muss nichts davon wissen. Er
beginnt bei der Startseite, der Sitemap, `robots.txt` und dem Feed und folgt
jedem Link, Stylesheet, Bild und `srcset`, das auf der Website bleibt. Die
zweite Seite einer Liste, `/blog?seite=2`, wird zu `/blog/seite/2/`, weil ein
statischer Host die Abfrage nicht ansieht, und die Links darauf werden
umgeschrieben. Eine Weiterleitung wird zu einer Seite, die ohne Skript
weiterleitet, und die 404-Seite der Website zu `404.html`.

Was nur ein laufender Server kann, fehlt mit Absicht und steht im Bericht, den
der Befehl ausgibt: die Suche, Formulare, geschützte Seiten sowie Warenkorb,
Kasse und Zahlung des Shops. Seiten, die ein Formular tragen, werden
geschrieben und einzeln genannt — das Formular steht darin und tut nichts.

Das Zielverzeichnis muss leer sein. Ein Export über einen alten hinweg würde
Seiten behalten, die inzwischen gelöscht sind.

## 2.5 — 2026-09-17

**Die Verwaltung ist auf dem Telefon benutzbar.** Nicht „geht auch irgendwie",
sondern gemessen: alle 29 Bildschirme auf einem 390 Pixel breiten Telefon, und
danach von Hand durchgefahren.

### Geändert

Vorher lief ein Drittel der Bildschirme über den Rand hinaus — das
Aktivitätsprotokoll um 338 Pixel, die Benutzerliste um 275, die Seitenliste um
187. Eine Seite, die man seitwärts schieben muss, um an einen Knopf zu kommen,
fühlt sich kaputt an, auch wenn der Knopf funktioniert. Jetzt läuft keine mehr
über. Die Listen bleiben Tabellen und scrollen in ihrer Karte, nicht mit der
ganzen Seite.

**Jedes Bedienelement ist mindestens 44 × 44 Pixel gross.** Das ist die Zahl,
auf die Apples Richtlinien und die Barrierefreiheitsregel WCAG 2.2 unabhängig
voneinander kommen; darunter trifft eine Fingerkuppe das Nachbarelement. Vorher
war auf jedem Bildschirm ein Dutzend bis drei Dutzend Elemente zu klein — das
Menüsymbol mass 32 × 32, das Benutzermenü 36 × 32, jede Zeile der Navigation 35
Pixel hoch. Jetzt keines mehr. Ein Ankreuzfeld ist die eine bewusste Ausnahme:
es ist 24 × 24 statt 13 × 13 und steht in einer 44 Pixel hohen Zeile, in der
auch die Beschriftung schaltet — ein Häkchen so gross wie ein Knopf wäre keins
mehr.

**Das grosse Schreibfeld des Seiteneditors ist nicht mehr zu klein beschriftet.**
Unter 16 Pixel zoomt ein iPhone die Seite beim Hineintippen und zoomt nicht
zurück: aus einem Tippen wird ein Zwei-Finger-Zug. Von allen Feldern ausgerechnet
das, in dem jemand einen Nachmittag verbringt, hatte 14.

Am Schreibtisch ändert sich nichts. Die grösseren Masse hängen daran, ob das
Gerät mit dem Finger bedient wird, nicht daran, wie breit der Bildschirm ist —
ein Telefon quer ist 926 Pixel breit und braucht sie trotzdem, eine Maus auf
einem schmalen Fenster braucht sie nicht. Ein Laptop mit Berührungsbildschirm
bekommt sie ebenfalls, und das ist beabsichtigt.

### Behoben

Das **Zeilenmenü einer Seite** — darin stehen Veröffentlichen, Duplizieren und
Löschen — erschien auf dem Telefon in der letzten Zeile einer Liste halb
abgeschnitten. Die Regel, die das verhindern sollte, stand seit ihrer
Niederschrift an einer Stelle, an der sie nie wirken konnte.

Die **Einstellungsseite einer Website** schob sich um 59 Pixel nach rechts
hinaus, sobald eine Domain einen längeren Namen hatte; der Knopf, der dabei
über den Rand ragte, war ausgerechnet „Entfernen".

Der **Markdown-Spickzettel** im Editor war 24 Pixel zu breit, sobald man ihn
aufklappte — also in dem einzigen Zustand, in dem ihn jemand liest. Er scrollt
jetzt in sich; die Beispiele brechen weiterhin nicht mitten im Wort.

## 2.4 — 2026-09-17

> Zu dieser Fassung gibt es **kein Tag und keine Ausgabe im Ausgabenverzeichnis**.
> Sie ist auf `main` eingeflossen und in 2.5 enthalten; wer 2.5 installiert, hat
> sie. Der Eintrag bleibt stehen, weil beschrieben gehört, was sich geändert hat
> — auch wenn niemand genau diesen Stand herunterladen kann.


Ein Meilenstein, der **nichts ändert, was Sie täglich sehen** — und deshalb
einer, dessen Liste ungewöhnlich aussieht. Er ist entstanden, weil das
Fensterbuch zum ersten Mal leer war, und ein leeres Fensterbuch ist kein
sauberer Baum, sondern einer, in dem nichts mehr steht, das jemand aufgeschrieben
hat. Also wurde nachgesehen, was niemand aufgeschrieben hatte.

Gefunden wurde es fast immer auf dieselbe Weise: **beim Schreiben eines Tests
für einen Bildschirm, nicht beim Lesen des Codes.** Das ist der eigentliche
Befund dieses Meilensteins.

### Behoben — was Ihnen hätte weh tun können

**Eine Domain liess sich über den falschen Bildschirm entfernen.** Der Knopf auf
der Einstellungsseite von Website A gab dem Speicher nur die Kennung der Domain
mit, und der löschte nach dieser Kennung allein. Ein Verklicken oder eine im
Browser liegengebliebene alte Seite genügte, um Website B eine Domain zu
nehmen — also einen Kundenauftritt vom Netz —, während der Bildschirm „Domain
entfernt" meldete und die eigene Liste unverändert dastand.

**Eine gelöschte Inhaltsart nahm ihre Einträge scheinbar mit.** Die Zählung sah
in der falschen Spalte nach: ein Eintrag einer eigenen Art ist intern eine Seite
und trägt den Schlüssel daneben. Drei Produkte zählten deshalb als *null*
Produkte — und zusätzlich als drei Seiten. Folge: Wer eine Art mit hundert
Einträgen entfernte, las „Inhaltsart entfernt" und sonst nichts, obwohl die
Warnung, dass die hundert Einträge bleiben, genau dafür da ist. Und die Zahl
„Seiten" auf demselben Bildschirm war um jeden Eintrag jeder eigenen Art zu
hoch.

**Ein Menü konnte sich beim Umbenennen selbst unsichtbar machen.** Der
Ortsschlüssel — das, wonach ein Theme das Menü sucht — wurde beim Anlegen
geprüft und beim Ändern nicht. „haupt" zu „Haupt Menü" zu machen wurde
angenommen, mit „Menü gespeichert" beantwortet, und die Navigation verschwand
von der Website, ohne dass irgendwo stand warum.

**Ein Logo-SVG wurde zu nachlässig geprüft.** Die Prüfung war eine Liste von
vier verbotenen Zeichenfolgen; `onerror=`, `onmouseover=`, ein Leerzeichen vor
dem Gleichheitszeichen und ein als XML-Entität geschriebener Doppelpunkt gingen
alle daran vorbei. Kein Einfallstor — die Sicherheitsrichtlinie des Browsers
verbietet Skript im Administrationsbereich ohnehin, und das hat sie die ganze
Zeit getan —, aber die Prüfung behauptete etwas, das sie nicht tat. Sie liest
das Dokument jetzt, statt darin zu suchen.

### Behoben — Sätze in der falschen Sprache

Elf Sätze, die ein Betreiber liest, gingen am Übersetzungs-Katalog vorbei. Das
Tückische daran: sie fehlten dort nicht, sie **existierten** dort nicht — der
Katalog meldete weder *offen* noch *verwaist*, weil er nichts von ihnen wusste.

Darunter zwei, die auf einer englischsprachigen Anlage deutsch blieben
(„%d Nachrichten werden erneut versucht.", „Zugeschnitten auf %d × %d Pixel."),
zwei deutsche Überschriften hinter Einladungs- und Zurücksetz-Links, die Meldung
nach dem Anlegen einer Website, und — auf jeder Anlage in jeder Sprache — das
Wort, das an eine Kopie angehängt wird: aus „Kontakt" wurde „Kontakt (Kopie)",
gleichgültig welche Sprache eingestellt war.

**Der Import-Bericht trug zwei Überschriften**, und die zweite log: Wer ein
Archiv einspielte, das dieses Programm selbst geschrieben hatte, las darunter
„WordPress-Import abgeschlossen".

### Neu für Entwickler

`tools/assembled` weist ab sofort jeden Satz ab, der mit `fmt.Sprintf` innerhalb
eines Aufrufs zusammengebaut wird, der sein Argument einem Betreiber zeigt. Die
Lücke steht seit jeher in CLAUDE.md; jetzt steht auch ein Tor davor, und die
Bauprüfung führt es aus.

### Was sonst geschah

Die Verwaltung ist von **35,9 % auf 59,3 %** durch Tests gedeckt, das Paket
`internal/branding` von 0 % auf 95,7 %. Jeder dieser Tests wurde erst rot
gefahren, bevor er grün sein durfte — 164 Mutationen, 158 gefangen, die sechs
übrigen mit Messung als gleichwertig festgehalten. Ausserdem wurden zwei seit
v2.0 offen festgehaltene Mängel geschlossen: die Bildfelder eines Textbausteins
überstehen jetzt ein Bündel, und eine Marke im Text wird nur noch dort ersetzt,
wo sie wirklich Text ist.

## 2.3 — 2026-09-15

### Behoben — die Sprache einer Seite

**Eine Galerie sprach die Sprache der Website und nicht die der Seite.** Auf
einer französischen Seite einer deutschen Website las ein Besucher deutsche
Bedienelemente im Lichtkasten und einen deutschen Vorlese-Namen am Schiebefeld.
Fünf Wörter, die dieses Programm selbst prägt — *Vorheriges Bild*, *Nächstes
Bild*, *Große Ansicht schließen*, der Satz zwischen den Video-Marken und der
Name der Galerie —, wurden beim **Speichern** übersetzt und standen seitdem in
der Sprache fest, die die Website in jenem Augenblick hatte.

Das hatte zwei Folgen, und beide waren zu sehen: eine Website, die ihre Sprache
wechselte, antwortete weiter in der alten, bis jede Seite noch einmal
gespeichert wurde. Und eine Seite in einer zweiten Sprache bekam die Wörter der
Website statt ihre eigenen.

Beim Speichern wird jetzt gar nichts mehr übersetzt. Die fünf Wörter stehen als
Marke in der Seite, und ein Durchgang bei der Auslieferung setzt sie in der
Sprache **der Seite** ein. **Eine vor dieser Fassung gespeicherte Seite wird
nicht angefasst**: sie trägt die Wörter weiterhin so, wie sie gespeichert
wurden, und heilt beim nächsten Speichern. Es gibt keine Migration und nichts zu
tun.

Vorher gemessen statt geraten: eine Seite ohne Galerie kostet das 311
Nanosekunden und kein einziges Byte zusätzlichen Speichers.

### Behoben — die Namen der Plugins

**Vier der fünf mitgelieferten Plugins nannten sich auf Deutsch, auch auf einer
englischen Installation.** Name, Beschreibung und der Eintrag in der
Seitenleiste stehen in der Datei, die ein Plugin über sich selbst mitbringt, und
wurden nie übersetzt — seit 2.0 die Quellsprache umgestellt hat, war das auf dem
ersten Bildschirm zu sehen, den jemand aufmacht.

Ein Plugin bringt seine Übersetzungen jetzt selbst mit, und die fünf
mitgelieferten tun das in Deutsch, Spanisch, Französisch und Italienisch. Wer
die Verwaltung auf Englisch liest, liest *Year*; wer sie auf Deutsch liest,
*Jahreszahl*.

### Hinzugefügt — die Fassungsnummer sagt, was sich geändert hat

Die Fassungsnummer unten links in der Verwaltung ist ein Verweis. Dahinter liegt
diese Datei: jede Ausgabe, die neueste oben, jede unter einer eigenen Adresse,
die sich weitergeben lässt.

Nichts meldet sich von selbst — kein Abzeichen, kein Fenster, das nach einer
Aktualisierung aufgeht. Wer wissen will, was neu ist, klickt auf die Nummer.

### Behoben — Kleinigkeiten

- Die Plugin-Liste und der Bildschirm eines Plugins schrieben ihre Überschrift
  zweimal untereinander.
- Der Rückweg vom Bildschirm eines Plugins stand neben der Überschrift statt
  dort, wo er auf jedem anderen Bildschirm steht.

### Für Betreiber

**Die Fassungen 2.0, 2.1 und 2.2 gibt es jetzt auch als Ausgabe zum
Herunterladen**, mit dem fertigen Linux-Programm und seiner Prüfsumme. Sie waren
im Changelog beschrieben und nie veröffentlicht worden.

**Vier Abhängigkeiten sind aktualisiert**, alle aus der festen Liste, auf der
dieses Programm steht: der SQLite-Treiber (1.57.0 → 1.58.0), die Migrationen
(goose 3.27.3 → 3.28.0), der Markdown-Übersetzer (goldmark 1.8.5 → 1.8.6) und
die Kryptografie, an der die Passwörter hängen (x/crypto 0.55.0 → 0.57.0). Der
Treiberwechsel ist der einzige, bei dem etwas schiefgehen könnte, und er wurde
nachgesehen statt angenommen: ein Programm aus diesem Stand hat eine Datenbank
geöffnet, die eine ältere Fassung geschrieben hatte, meldete Schemastand 56 und
antwortete auf die Prüfung mit „integrity: ok". **Es ist nichts zu tun**, aber
es steht hier, weil ein Treiber unter einer Datenbank kein Nebensatz ist.

Eine Warnung, die dabei herauskam und die eine eigene Zeile verdient: die
mitgelieferten Plugin-Module von 2.1 und 2.2 waren gegen eine ältere Fassung der
Plugin-Schnittstelle gebaut. Die Prüfung, die genau das findet, lief zwei Tage
lang rot, ohne dass es jemand gelesen hat. Sie läuft jetzt auch beim Erstellen
einer Ausgabe, damit ein veraltetes Modul nicht in einer Ausgabe mitreist.

### Für Entwickler von Erweiterungen

`plugin.json` kennt ein neues, freiwilliges Feld `lang`: je Sprache eine Tabelle
von der englischen Fassung eines Satzes auf die Übersetzung, für `name`,
`description` und `admin.label`. Ohne das Feld verhält sich ein Manifest genau
wie bisher. Die Regel ist dieselbe wie beim Katalog eines Themes: der englische
Satz **ist** der Schlüssel, ein fehlender Schlüssel fällt auf den Satz zurück,
und `de-CH` fragt zuerst `de`. Die Paketbeschreibung von `sdk` sagt es mit
einem Beispiel.

## 2.2 — 2026-09-14

### Behoben — eine geschützte Seite

**Eine passwortgeschützte Seite durfte von einem vorgelagerten Cache
aufbewahrt werden.** Wer das Passwort eingegeben hatte, bekam die Seite mit
`Cache-Control: public, max-age=300` — und mit einer `Vary`-Angabe, aus der das
Cookie verschwunden war, an dem der Zugang hängt. Ein CDN oder ein Firmenproxy
vor diesem Server konnte sie damit fünf Minuten lang jedem geben, der die
Adresse kannte. Das Formular *davor* war korrekt geschützt; die Seite dahinter
nicht.

Dieselbe Zeile gab einem Laden, der beide Preismodi anbietet, öffentliches
Zwischenspeichern für Preise, die einem Cookie folgen — Geschäftskundenpreise
konnten so an Privatkunden gehen.

Es gibt jetzt drei Antworten statt einer, und jede sagt, was sie ist: öffentlich
für das, was für alle gleich ist; privat für Preise, die dem Besucher folgen;
gar nicht zwischenspeicherbar für eine Seite hinter einem Passwort. Bei nur
einem Preismodus bleibt der Katalog teilbar — die stumpfe Behebung hätte jede
Seite jeder Website uncachebar gemacht.

`docs/security.md` sagt die Regel jetzt und sagt auch, was vorher falsch war.

### Behoben — Einmalanmeldung

- **Der Abmelde-Knopf meldete niemanden ab.** Wer sich bei eingeschalteter
  Einmalanmeldung mit Passwort angemeldet hatte, landete beim Abmelden auf dem
  Anmeldeformular — und der nächste Klick meldete ihn über den Ausweisdienst
  sofort wieder an. Er geht jetzt zum Ausweisdienst, wenn dieser gerade für
  jemanden bürgt. Ist der Proxy kaputt — der Fall, für den die Passwortanmeldung
  da ist —, bleibt alles, wie es war.
- **Eine abgewiesene Identität füllte das Tätigkeitsprotokoll.** Jede einzelne
  Anfrage schrieb eine Zeile; ein Proxy, der dauernd dieselbe abgewiesene
  Identität behauptet, liess die Tabelle unbegrenzt wachsen. Jetzt höchstens
  eine Zeile je Identität und Grund alle 15 Minuten. Abgewiesen wird weiterhin
  jede Anfrage.
- **Das Protokoll sagt jetzt, warum und auf welchem Weg.** Eine abgewiesene
  Anmeldung nennt ihren Grund, eine misslungene Sitzungserneuerung hinterlässt
  überhaupt erst eine Zeile, und eine Abmeldung unterscheidet den Knopf vom
  automatischen Ende. Zeilen, die bei der Bereitstellung eines Kontos entstehen,
  tragen jetzt dieses Konto und sind über den Benutzerfilter auffindbar.
- **Das Benutzerformular sagt, woher die Rechte kommen.** Bei einem verknüpften
  Konto und eingerichteten Gruppen steht neben den Häkchen, dass eine Änderung
  hier nur bis zur nächsten Anfrage hält.

### Behoben — Galerie

**Eine Albumgalerie sprach zwei Sprachen auf einmal.** Der Name ihres
Schiebefelds wurde beim Speichern eingefroren, die Bedienelemente daneben bei
der Auslieferung aufgelöst. Nach einem Sprachwechsel der Website antwortete sie
„Imagen siguiente" neben `aria-label="Galerie"`, bis jede Seite mit einer
Galerie neu gespeichert war. Beides entsteht jetzt in einem Zug. Alte Seiten
laufen unverändert weiter und heilen beim nächsten Speichern — es braucht keine
Migration.

### Behoben — Kleinigkeiten

- Verwaltungsantworten trugen `Vary: Cookie` zweimal.
- `holzcloud`s Übersetzungswerkzeug liess einen verwaisten Schweizer Eintrag
  stehen, auch beim zweiten Lauf.

### Für Betreiber

`release.yml` kann einen Release-Tag jetzt selbst anlegen: *Actions → Release →
Run workflow*, mit Tag-Name und Commit. Bisher musste der Tag von Hand gepusht
werden, bevor der Workflow überhaupt anlief.

---

## 2.1 — 2026-09-13

### Hinzugefügt

**Die öffentliche Seite spricht die Sprache des Besuchers.** Eine Website, die
auf Französisch erscheint, liest sich auf Französisch — nicht nur die Seiten,
sondern auch das Drumherum des Themes und alles, was ein Plugin einem Besucher
sagt. Bis 2.0 war das nicht so: Wer sich auf einer deutschen Seite bei der
E-Mail-Adresse vertippte, bekam *"The e-mail address does not look right."* zu
lesen. Das war ein Fehler von 2.0 und ist der erste Punkt, den diese Fassung
behebt.

**Ein Theme bringt seine eigenen Wörter mit.** Neben den Vorlagen darf jetzt ein
Verzeichnis `lang/` stehen, mit `de.json`, `fr.json` und so weiter. In der
Vorlage holt `{{t "Weiterlesen"}}` das Wort daraus. Die acht mitgelieferten
Themes bringen je vier Kataloge mit, auf Deutsch, Französisch, Italienisch und
Spanisch. Was ein Theme nicht übersetzt hat, nennt `holzcloud template check`
beim Namen; abgewiesen wird deswegen nichts.

**Und der Betreiber hat das letzte Wort.** Unter *Wörter* lässt sich je Website
und je Sprache überschreiben, wie das Theme etwas nennt. Die Reihenfolge ist:
das eigene Wort, dann das des Themes, dann der Schlüssel selbst.

TEMPLATE-SPEC §2.5 sagt seit dieser Fassung das Gegenteil von dem, was drei Tage
vorher darin stand. Die alte Begründung ist zitiert und beantwortet, nicht
gelöscht.

### Hinzugefügt — das Kontaktformular

Das Formular war schlicht hässlich und konnte wenig. Jetzt:

- **Es sieht in allen acht Themes nach etwas aus.** Drei von ihnen hatten vorher
  keine einzige Regel dafür und haben es in den Voreinstellungen des Browsers
  gezeichnet.
- **Ein Fehler steht an dem Feld, um das es geht** — nicht als ein Satz über dem
  ganzen Formular.
- **Wer schreibt, bekommt eine Eingangsbestätigung**, wenn du sie einschaltest.
  Eine Kopie der eben gesendeten Nachricht, an die Adresse, die ohnehin darin
  steht, und an keine andere.
- **Antworten geht aus der Verwaltung**, ohne Wechsel ins Mailprogramm. Die
  Antwort bleibt bei der Nachricht.
- **Eine Einwilligung, die du selbst formulierst**, mit einem Satz je Sprache.
  Was jemand angekreuzt hat, steht Wort für Wort bei seiner Nachricht, mit dem
  Zeitpunkt — auch in einem Jahr, wenn du den Satz umformuliert hast.
- **Abgewiesene Absendungen verschwinden nicht mehr.** Honigtopf, Zeitfalle und
  Stundengrenze legen sie in eine Quarantäne mit dem Grund. Ein Fehlalarm lässt
  sich dort finden und freigeben.
- **Keine Anfrage fällt mehr still weg.** Was niemand gelesen hat, wird nicht
  gelöscht, um Platz zu schaffen.
- **Dateien dürfen mitkommen**, wenn du es einschaltest. Höchstens drei, geprüft
  an ihrem Inhalt, und erst auf der Platte, wenn die Spamfallen durch sind.
- **Ein Feld, das nur manchmal gefragt wird.** Hängt es von einer früheren
  Antwort ab, erscheint es nach einem Klick auf *Weiter* — ohne JavaScript, wie
  alles hier.

### Behoben

**Eine Antwort auf ein abgesendetes Formular kam auf der Startseite nie an.**
Die Adresse wurde aus der Kennung der Seite gebaut, also `/home`, und `/home`
leitet auf `/` weiter — ohne die Angabe, worum es ging. Wer auf der Startseite
ein Formular abschickte, sah das Formular wieder und weder ein Danke noch einen
Grund. Dasselbe verlor bei einer Seite in einer zweiten Sprache deren Vorsatz.
Das Formular trägt die Adresse jetzt selbst mit.

### Für Entwickler von Erweiterungen

`ContentIn` und `RequestIn` tragen zwei neue Felder: `Path`, die Adresse, unter
der diese Seite ausgeliefert wird, und `Lang`, die Sprache, in der das
geschieht. Beides ist nicht aus dem Bisherigen abzuleiten — die Startseite hat
die Kennung `home` und liegt unter `/`, und eine Seite in der zweiten Sprache
der Website wird in dieser zweiten ausgeliefert und nicht in der ersten.
`Site()` beantwortet ausserdem, in welchen Sprachen eine Website erscheint.

---

## 2.0 — 2026-09-13

### Geändert — mit Bruch

**Der Vorlagen-Vertrag spricht Englisch. Ein Theme, das für 1.x geschrieben
wurde, hört auf zu funktionieren.** Sieben Namen, die ein Theme-Autor eintippt,
heissen ab dieser Fassung anders:

| bis 1.10 | ab 2.0 |
|---|---|
| `.Page.Felder` | `.Page.Fields` |
| `.Page.Feldliste` | `.Page.FieldList` |
| `.Page.Art` | `.Page.Kind` |
| `.Page.Uebersetzungen` | `.Page.Translations` |
| `.Site.Bausteinfelder` | `.Site.SnippetFields` |
| `.Site.Bausteinliste` | `.Site.SnippetList` |
| `.Site.Sprachen` | `.Site.Languages` |

Das ist Absicht und geschieht in genau einer Fassung, statt beide Schreibweisen
jahrelang nebeneinander zu führen. Der Grund ist der Zweck dieser Fassung: die
Quelle dieses Programms ist auf Englisch umgestellt, damit sie jemand lesen
kann, der kein Deutsch spricht — und der Vertrag ist der Teil davon, den nicht
nur wer hineinsieht, sondern jeder Theme-Autor tippt. Ein halber Umstieg wäre
dauerhaft schlechter als ein ganzer.

**Was zu tun ist.** In jeder `.html`-Datei des Themes die linke Spalte durch die
rechte ersetzen; die Bedeutung ändert sich an keiner Stelle, nur der Name. Alle
acht mitgelieferten Themes sind umgestellt. `holzcloud template check` nennt
eine übersehene Stelle beim Namen, bevor ein Upload angenommen wird.

**Was sich *nicht* ändert:** die gespeicherten Werte. Eine Feldart heisst
weiterhin `langtext`, eine Bausteinart weiterhin `zitat`, und die CSS-Klassen
der Bausteine heissen weiterhin `hc-aufruf`, `hc-zitat`, `hc-galerie`. Die
stehen in jeder Datenbank und reisen in jedem Archiv; sie umzubenennen wäre ein
zweiter Bruch ohne Gewinn für irgendjemanden. Ein Theme-Stylesheet bleibt also
unangetastet.

**Die Werkzeuge des KI-Zugangs sprechen Englisch. Ein gespeicherter Prompt, der
ein Werkzeug beim deutschen Namen nennt, hört auf zu funktionieren.** Aus
`seite_anlegen` wird `create_page`, aus `felder_auflisten` wird `list_fields`,
und ebenso heissen die Argumente und die Antwortschlüssel neu: `"titel"` wird
`"title"`, `"zustand": "entwurf"` wird `"status": "draft"`. Die ganze Liste
steht in [docs/ai-access.md](docs/ai-access.md).

Derselbe Grund wie beim Vorlagen-Vertrag: was eine Maschine liest, gehört zur
Quelle und nicht zu dem, was ein Betreiber tippt. Ein Assistent, der die
Werkzeugliste abfragt — und so ist MCP gedacht — merkt von der Umstellung
nichts. Wer einen Werkzeugnamen fest in einen Prompt geschrieben hat, bekommt
eine klare Absage (`there is no tool "seite_anlegen"`) statt eines stillen
Fehlverhaltens.

**Was sich *nicht* ändert:** alles, was ein Betreiber selbst eingetippt hat. Ein
Feldschlüssel heisst weiterhin `preis`, eine eigene Inhaltsart weiterhin
`produkt`, und `list_fields` meldet eine Feldart weiterhin als
`mehrfachauswahl`. Das sind Werte in der Datenbank, nicht Vokabular des
Protokolls.

**Die deutschen Spaltennamen der Datenbank heissen englisch** — Wanderung 00054
benennt vierundzwanzig Spalten um, darunter `pages.art` zu `pages.content_kind`.
Das geschieht beim Start von selbst und ist von aussen nicht zu sehen; es steht
hier, weil eine Sicherung aus 1.x mit einer Fassung 1.x zurückgespielt werden
muss und nicht mit dieser.

### Hinzugefügt

**Ein Plugin kann jetzt übersetzen.** Das SDK bekommt `T` und `Tf`, der Host
eine Operation `translate`, und `tools/i18n` liest `plugins/` als dritte Wurzel.
Ein Plugin schreibt seinen Satz auf Englisch, der Host schlägt ihn im eigenen
Katalog nach, und der Bildschirm erscheint in der Sprache, die der Betreiber
gerade liest. Es braucht dafür keine Berechtigung: nach einem Wort zu fragen
ist nicht, nach den Daten von jemandem zu fragen. Die beiden mitgelieferten
Plugin-Bildschirme sind umgestellt.

**TEMPLATE-SPEC §2.5 sagt jetzt, dass ein Theme einsprachig ist** — und wie eine
mehrsprachige Website stattdessen gebaut wird: die Wörter des Rahmens kommen aus
Textbausteinen, die Felder tragen, nicht aus der Vorlage. Das war vorher wahr
und stand nirgends.

### Behoben

**Vier Spaltenüberschriften der Seitenliste standen auf einer englischen
Oberfläche deutsch da** — „Art", „Sprache", „Adresse" und „Veröffentlicht". Sie
trugen ihr deutsches Wort noch als Katalogschlüssel, und nach der Umstellung der
Quellsprache fand die Übersetzung sie nicht mehr und fiel auf den Schlüssel
zurück. Dieselbe Ursache hatten die Beschriftungen der Schriftenliste im Design,
die Auswahl „Art" im Seitenformular und die Zuschnittformate der Mediathek.

**Das Einlesen eines Holzcloud-Archivs meldete „WordPress-Import
abgeschlossen".** Beide Wege teilen sich denselben Bericht, und die Überschrift
ist mitgereist.

**Rund vierzig Sätze, die ein Betreiber liest, standen in keinem Katalog** und
waren deshalb in keiner Sprache ausser Deutsch zu haben: die Meldungen der
WordPress-Einfuhr, die Sammelaktion der Seitenliste, die Hinweise beim
Hochladen eines Bildes, die Kontomails, der Konflikthinweis am Seitenformular
und die ganzen Verwaltungsbildschirme des Ladens. Sie waren mit `+` oder
`fmt.Sprintf` zusammengesetzt, und was so entsteht, sieht der Sammler nicht.
Jetzt sind es 1610 Einträge in de, es, fr und it — 250 mehr als in 1.10.

### Unter der Haube

**Die Quelle dieses Programms ist auf Englisch.** Jeder Kommentar, jeder
Bezeichner, jeder Testname und jeder Katalogschlüssel — rund 110 000 Zeilen. Der
Katalog hat die Richtung gewechselt: der englische Satz ist jetzt der Schlüssel,
und `de.json` ist eine Übersetzung wie `fr.json` auch. Für einen Betreiber ändert
sich dabei nichts; die Verwaltung spricht weiterhin Deutsch, sobald der Browser
Deutsch verlangt.

`go run ./tools/english` hält das fest und läuft in CI. Ein deutscher Kommentar
lässt den Bau scheitern — mit genau vier benannten Ausnahmen, die jede ihren
Grund an Ort und Stelle tragen: die Kataloge, die Vorrichtung, mit der eine
hochgeladene Vorlage geprüft wird, die Sätze, die ein *Kunde* liest (Kasse,
Bestellmails, Monatsnamen), und die Dateien, deren Kommentare von der deutschen
Sprache handeln und sie deshalb benennen müssen.

## 1.10 — 2026-09-11

### Hinzugefügt

**Anmeldung über den Ausweisdienst der Organisation.** Wer sich bei der eigenen
Anmeldung des Betriebs — Authentik — bereits ausgewiesen hat, kommt in die
Verwaltung, ohne ein zweites Mal ein Passwort einzugeben. Ein vorgeschalteter
Caddy fragt den Ausweisdienst, wer da klopft, und reicht die Antwort weiter.
Welche Gruppen jemand dort hat, entscheidet, ob er die Anlage verwaltet oder
Inhalte pflegt und welche Websites er betreten darf; das wird bei **jeder**
Anmeldung neu gelesen. Nimmt man jemandem beim Ausweisdienst eine Gruppe weg,
ist der Zugang hier beim nächsten Klick weg — auch mitten in einer laufenden
Sitzung, nicht erst, wenn sie abläuft. Das Ganze ist ausgeschaltet, solange es niemand einschaltet,
und der Weg über das Passwort bleibt unverändert daneben bestehen. Er ist auch
der Weg zurück, wenn der Ausweisdienst einmal nicht antwortet.

**Für eine Anlage, die das nicht benutzt, ändert sich genau eines, und das ist
das Wichtigste an diesem Eintrag: Der Dienst horcht jetzt auf `127.0.0.1` und
nicht mehr auf allen Netzwerkkarten.** Wer Caddy auf demselben Rechner
betreibt — die beschriebene Einrichtung — merkt davon nichts. Wer den Dienst von
einem anderen Rechner aus erreicht, braucht `HOLZCLOUD_LISTEN=0.0.0.0`. **Das
Container-Abbild setzt das selbst**: im Container ist die Rückschleife immer die
falsche Adresse, weil ein veröffentlichter Port, ein Kubernetes-Service und die
Proben alle über die Adresse des Containers kommen. Wer das Abbild betreibt,
muss nichts ändern.

**Wer die Anmeldung über den Ausweisdienst einschaltet, braucht Caddy in
Fassung 2.11.2 oder neuer.** Ältere Fassungen ab 2.10.0 tragen CVE-2026-30851:
`forward_auth` setzte die Kopfzeilen mit der Auskunft über die Person zwar,
löschte aber die gleichnamigen Kopfzeilen nicht, die der Besucher selbst
mitgeschickt hatte. Antwortete der Ausweisdienst einmal ohne eine davon, kam
die Angabe des Besuchers unverändert hinten an. Der mitgelieferte
`deploy/Caddyfile.example` löscht sie jetzt ausdrücklich, in beiden
Schreibweisen, und Holzcloud löscht sie unabhängig davon noch einmal selbst.
Was einzustellen ist, steht in `deploy/DEPLOY.md`.

**Ein Hinweis, der eine Sicherheitszusage verschiebt:** Eine Sitzung des
Ausweisdienstes erfüllt die Bestätigung in zwei Schritten. Diese Anlage verlangt
sie dann nicht noch einmal — welcher zweite Schritt tatsächlich verlangt wird,
entscheidet also ab sofort die Anmeldung der Organisation und nicht mehr
Holzcloud. Für Verwaltende, die sich mit Passwort anmelden, bleibt sie
unverändert Pflicht. Der Satz steht auch in der Verwaltung, unter *Mein Konto*
und über der Liste der Personen.

**Ein Konto gehört zu genau einer Identität beim Ausweisdienst, und eine
E-Mail-Adresse genügt nie.** Eine Anmeldung erreicht das Konto, das mit ihrem
Benutzernamen verknüpft ist — nicht das Konto, dessen Adresse zufällig mitkommt.
Ein Konto, das die Anlage beim ersten Besuch selbst anlegt, ist von Anfang an
verknüpft. **Ein von Hand angelegtes Konto ist über den Ausweisdienst nicht
erreichbar, bis es verknüpft wird:**

    holzcloud user sso -email ada@example.com -username ada

`-unlink` hebt die Verknüpfung auf. Ein Konto wird bewusst nicht beim ersten
Mal über die Adresse verknüpft: sonst entschiede, wer an dem Tag schneller ist,
an dem die Anmeldung eingeschaltet wird. Wer einen Benutzer beim Ausweisdienst
umbenennt, muss ihn hier neu verknüpfen.

**Mit eingeschalteter Anmeldung prüft die Anlage beim Start strenger.** Das
gemeinsame Geheimnis braucht mindestens 32 Zeichen (`openssl rand -hex 32`).
`HOLZCLOUD_TRUSTED_PROXIES` darf nicht jede Adresse zulassen — `0.0.0.0/0` oder
`::/0` halten den Dienst an, denn die vertrauten Proxys entscheiden, ob eine
Auskunft über eine Person überhaupt gelesen wird. Und jede Website-Nummer in
`HOLZCLOUD_SSO_WEBSITE_GROUPS` muss es geben; eine, die keine Website benennt,
hält den Dienst ebenfalls an, statt jede Person dieser Gruppe beim Anmelden
kommentarlos abzuweisen.

**Eine Sitzung des Ausweisdienstes gilt nur so lange, wie er dieselbe Person
bestätigt.** Meldet sich im selben Browser jemand anderes beim Ausweisdienst an,
endet die Sitzung, und diese Person wird angemeldet. Schaltet man die Anmeldung
über den Ausweisdienst ab, enden alle Sitzungen, die sie gemacht hat, bei ihrer
nächsten Anfrage — Sitzungen mit Passwort bleiben unberührt. Kommt eine
Auskunft über die Person mit zwei Werten an, wird sie nicht geglaubt.

**Drei Dinge, die die Anmeldung über den Ausweisdienst noch nicht kann**, stehen
in `deploy/DEPLOY.md` ausgeschrieben: Ein Konto, das nur so hereinkam, kennt kein
Passwort und kommt an den fünf Aktionen nicht vorbei, die es noch einmal
verlangen (`holzcloud user passwd` hilft). Abmelden aus einer Passwortsitzung
meldet beim Ausweisdienst niemanden ab. Und eine von Hand entzogene Website kehrt
beim nächsten Klick zurück, solange die Gruppen die Websites bestimmen.

### Behoben

**Ein Nein in einer eigenen Bausteinart erschien auf der Seite als Ja.** Eigene
Bausteinarten tragen dieselben Feldarten wie eine Seite, aber ihre Werte wurden
beim Speichern nur gekürzt, nie geprüft. Über ein eingespieltes Archiv oder ein
von Hand abgeschicktes Formular konnte so ein Ja/Nein „nein" heissen — und der
Baustein las alles, was nicht „0" war, als Ja. Ebenso standen eine Zahl
ausserhalb ihres Bereichs, eine Uhrzeit wie „25:99" oder eine Auswahl, die es
nicht gibt, unverändert auf der öffentlichen Seite. Wer nur im Bausteineditor
klickt, hat das nie gesehen: das Formular schickt für ein Häkchen immer „1".

Jetzt gilt für ein Bausteinfeld dieselbe Prüfung wie für ein Seitenfeld. Was sie
ablehnt, wird beim nächsten Speichern der Seite verworfen, ohne Meldung — so wie
bisher schon ein Wert, dessen Feld aus der Bausteinart entfernt wurde. Ein
bereits gespeichertes „nein" wird sofort richtig gelesen. Eine Mehrfachauswahl
steht in einem Baustein jetzt als Liste da, ein Wert je Punkt, mit der Klasse
`hc-eigen__liste`; bisher standen die Werte untereinander in einer Zeile.

**Der Feed zeigte nach einer Albumänderung das alte Datum.** Die Einträge trugen
die neuen Bilder schon, aber `<updated>` und `Last-Modified` des Feeds kannten
nur Seiten und Textbausteine. Ein Feedleser, der sich auf das Datum verlässt,
hatte keinen Grund nachzusehen. Die Seite selbst hatte das schon richtig.

**Das Löschen einer Website konnte einen Redakteur zum Redakteur aller Websites
machen.** Wer auf Websites eingeschränkt ist, war das bisher allein durch seine
Zuordnungen, und keine Zuordnung hiess: alle Websites. Wurde die einzige Website
eines Redakteurs gelöscht, verschwand mit ihr seine einzige Zuordnung — und aus
„nur diese eine" wurde „alle". An diesem Redakteur hatte niemand etwas geändert.
Dasselbe konnte ein Speichern der Rechte hinterlassen, das mittendrin
scheiterte. Betroffen seit Fassung 1.3, und nur wer Redakteure einschränkt.

Jetzt steht ausdrücklich da, ob jemand eingeschränkt ist. Wer eingeschränkt ist
und seine letzte Website verliert, erreicht **keine** mehr. Die Liste der
Personen zeigt das als „0 von N Websites", und das Formular fragt, was „nichts
angekreuzt" heissen soll — „alle Websites" oder „keine Website" —, damit ein
unverändertes Speichern nichts erweitert. Ein Häkchen schränkt immer ein.

**Bitte nach dem Aktualisieren einmal durchsehen.** Die Umstellung erkennt
eingeschränkte Personen an ihren Zuordnungen. Wer *vor* dem Aktualisieren die
einzige Website verloren hat, hat keine mehr — und ist darum für die Anlage
jemand, den nie jemand eingeschränkt hat. Kein Programm kann die beiden
unterscheiden. In der Liste der Personen stehen diese Redakteure bei „alle
Websites"; wer dort jemanden findet, der eingeschränkt sein sollte, setzt die
Häkchen neu.

**Ein Redakteur konnte die Navigation einer fremden Website ändern.** Die
Zugangsprüfung nimmt die Website-Nummer aus der Adresse und prüft, ob der
Angemeldete auf *diese* Website darf. Die Menü-Handler prüften danach korrekt,
dass das Menü zu dieser Website gehört — **die vier Handler für die einzelnen
Menüeinträge taten es nicht.** Sie prüften nur, dass der Eintrag zum genannten
Menü gehört, und nicht, dass das Menü zur genannten Website gehört.

Wer also nur für Website A freigeschaltet war, konnte über
`/admin/websites/A/menus/<B>/items/<B>` die Einträge von Website B anlegen,
ändern, löschen und umsortieren. Der Beweis steht als Prüffolge im Repository:
`internal/admin/menu_scope_test.go` schlägt gegen den Stand vor der Behebung
fehl und zeigt, wie die Navigation einer fremden Website auf eine fremde Adresse
umgebogen wird.

Betroffen ist nur, wer Redakteure auf einzelne Websites einschränkt
(`user_websites`, seit Fassung 1.3). Alle sieben Menü-Handler gehen jetzt durch
zwei Prüffunktionen; `internal/menu/store.go` trägt einen Hinweis, dass er die
Zuordnung selbst **nicht** erzwingt, damit die nächste Erweiterung nicht
dieselbe Lücke aufmacht.

Bei der Gelegenheit wurden 51 weitere Admin-Routen durchgesehen, die eine
Website-Nummer und eine zweite Kennung entgegennehmen. Alle anderen prüfen
korrekt — Seiten, Fassungen, Medien, Textbausteine, Schlagwörter,
Weiterleitungen, gespeicherte Ansichten, Produkte, Bestellungen, Inhaltsarten,
Bausteinarten und Felder. Das Menü war der einzige Ausreisser, und zwar weil es
als einziges die Zuordnung im Aufrufer statt im Speicher prüft.

**Ein Redakteur konnte das Produkt einer fremden Website überschreiben.**
Dieselbe Lücke wie oben, eine Ebene weiter: Das Formular zum Bearbeiten eines
Produkts nimmt die Produktnummer aus der Adresse und speichert, ohne zu prüfen,
zu welcher Website dieses Produkt gehört. Die Anzeige desselben Formulars und
das Löschen prüfen es korrekt — **allein das Speichern tat es nicht**, und der
Speicher konnte es nicht auffangen, weil `UPDATE products … WHERE id = ?` die
Website gar nicht erwähnte.

Wer nur für Website A freigeschaltet war, konnte über
`/admin/websites/A/produkte/<B>` Titel, Untertitel, Beschreibung,
Artikelnummer, **Preis**, Steuersatz, Lagerbestand, Gewicht, Lieferhinweis,
Adresse und Veröffentlichungsstatus eines Produkts von Website B neu schreiben.
Verschieben ging nicht — die Website-Spalte wird beim Speichern nicht angefasst
—, überschreiben schon. Der Beweis steht als eigene Fassung im Repository:
`internal/admin/product_scope_test.go` schlägt gegen den Stand davor fehl und
zeigt, wie ein Tisch für 2490.00 auf 0.05 gesetzt und veröffentlicht wird.

**Bitte im Bestand nachsehen.** Der Fehler hatte eine zweite Wirkung, die keine
Programmänderung rückgängig macht: Beim Speichern werden auch die Schlagwörter
neu gesetzt, und dabei wurden die des fremden Produkts gelöscht und ein
Schlagwort der *eigenen* Website angehängt. In `product_terms` konnte so eine
Zeile entstehen, deren beide Hälften zu verschiedenen Websites gehören. Wer
Redakteure auf einzelne Websites einschränkt, sollte einmal nachzählen:

    SELECT pt.product_id, pt.term_id FROM product_terms pt
      JOIN products p ON p.id = pt.product_id
      JOIN terms t ON t.id = pt.term_id
     WHERE p.website_id <> t.website_id;

Jede Zeile, die das ausgibt, ist von Hand zu löschen.

Betroffen ist wie oben nur, wer Redakteure auf einzelne Websites einschränkt
(`user_websites`, seit Fassung 1.3). Anders als beim Menü sitzt die Prüfung
diesmal **im Speicher**: `products` hat eine Website-Spalte, also tragen
`Update` und `Delete` sie in der `WHERE`-Bedingung und melden `ErrNotFound`,
wenn keine Zeile passt. Die geänderte Signatur zwingt den Übersetzer, jeden
Aufrufer zu nennen — ein Hinweis im Kommentar hätte das nicht getan.
`term.SetForProduct` bindet Produkt und Schlagwort jetzt über dieselbe Website
zusammen, damit die Zeile oben gar nicht mehr entstehen kann.

Auch hier wurde die Nachbarschaft durchgesehen: sämtliche Handler und
Speichermethoden für Produkte, Warenkörbe, Bestellungen und Zahlungen. Das
Speichern des Produkts war der einzige Fund. Der öffentliche Warenkorb ist
sauber, und zwar mit Absicht — er sucht den Artikel über die Adresse innerhalb
der Website und trägt seit jeher den Kommentar, dass eine Nummer aus dem
Formular in einen fremden Katalog reichen würde.

**Ein Redakteur konnte die E-Mail einer fremden Website erneut verschicken.**
Beim Durchsehen der Bestellhandler gefunden, gleiche Bauart, andere Tabelle.
Auf der Bestellseite treffen drei Kennungen aufeinander: die Website aus der
Adresse (geprüft), die Bestellnummer aus der Adresse (innerhalb dieser Website
gesucht) — und die Nachrichtennummer aus dem abgeschickten Formular, die
ungeprüft an den Postausgang weitergereicht wurde.

Das ist keine Verunstaltung, sondern eine Zustellung: Wer nur für Website A
freigeschaltet war, konnte über die Schaltfläche „Nochmals senden" eine
Nachricht von Website B wieder in die Warteschlange stellen. Deren Kundschaft
bekommt die Mail ein zweites Mal, und der Fehlertext samt Versuchszähler, den
der andere Betrieb gerade auswerten wollte, war gelöscht. Der Beweis liegt als
eigene Fassung bei: `internal/admin/order_scope_test.go`.

`outbox.Retry` nimmt jetzt die Website entgegen und trägt sie in der
`WHERE`-Bedingung; „bereits verschickt", „gibt es nicht" und „gehört nicht
Ihnen" sind absichtlich dieselbe Antwort, damit die Bestellseite nicht dazu
benutzt werden kann, die Existenz fremder Nachrichten abzufragen.

**Ein Redakteur konnte den Titel und den Entwurfsstand fremder Seiten lesen.**
Dieselbe Bauart, aber die zweite Kennung ist diesmal keine Ressource, die
jemand ändern will, sondern eine **Verknüpfung** — und eine Verknüpfung liest
sich in beide Richtungen.

Auf einer mehrsprachigen Website steht im Seitenformular ein verstecktes Feld
mit der Nummer der Seite, deren Übersetzung diese ist. Diese Nummer wurde
ungeprüft in die Spalte geschrieben, und die beiden Abfragen, die eine
Übersetzungsgruppe wieder auslesen, nannten die Website ebenfalls nicht. Wer
nur für Website A freigeschaltet war, konnte die Nummer von Hand auf eine Seite
von Website B setzen, speichern — und bekam beim nächsten Öffnen desselben
Bildschirms den Titel jener Seite angezeigt, mit dem Abzeichen „Entwurf", wenn
sie unveröffentlicht war. Eine Seite pro Speichervorgang, durch Abzählen der
Nummern.

War die fremde Seite veröffentlicht, kam etwas dazu: die Sprachumschalter
**beider** Websites zeigten fortan einen Eintrag, der auf die jeweils andere
zeigte, auf deren eigener Adresse.

`SetTranslation` nimmt jetzt die Website entgegen, verweigert eine fremde
Zielseite und schreibt in diesem Fall gar nichts; die Sprache der Seite wird
trotzdem gesetzt, denn das Speichern hat stattgefunden, und der abgewiesene
Versuch steht im Protokoll. Beide Leser tragen die Website in der Bedingung —
nicht nur der schreibende Weg, denn eine Verknüpfung, die schon in der Spalte
steht, darf auch nicht mehr zurückgelesen werden.

**Erweiterungen bekamen Seiten zu sehen, die noch nicht oder nicht mehr
öffentlich sind.** Wer eine Erweiterung schreibt, ist jemand anderes — deshalb
steht die Regel „nur veröffentlichte Seiten" im Wirt und nicht in der
Erweiterung. Sie stand dort auch, aber sie fragte die falsche Bedingung ab: die
Liste, die eine Erweiterung anfordert, wurde mit dem Filter des
Verwaltungsbereichs gebaut, und der kennt nur die Spalte `status`.

Also kam alles mit, was `status = 'published'` trägt und trotzdem nicht
öffentlich ist: eine Seite, deren Veröffentlichungsdatum noch in der Zukunft
liegt, eine, deren Ablaufdatum vorbei ist, und eine, die hinter einem Passwort
steht. Auf einer Seite mit der Bestell-Erweiterung stand damit das Produkt der
nächsten Saison samt Preis in der öffentlichen Liste. Die Einzelabfrage war
schlimmer: sie gab den **vollständigen Text** einer passwortgeschützten Seite
heraus, ohne dass das Passwort je eingegeben wurde und ohne eine Zeile im
Protokoll.

Beides ist zu. Für die Liste gibt es `page.ListPublic`, das dieselbe Bedingung
benutzt wie die Übersicht, der Feed, die Suche und beide Archive — „darf in
einer Liste stehen" ist eben etwas anderes als „darf unter der eigenen Adresse
ausgeliefert werden", und auf der eigenen Adresse steht das Passwortfenster
davor. Die Einzelabfrage antwortet auf eine geschützte Seite jetzt genau das,
was sie auf eine nicht vorhandene antwortet.

**Der Tabellenimport löschte die Schlagwörter der Seiten, die er aktualisierte.**
Eine Datei mit einer Schlagwörter-Spalte, in der nicht jede Zeile etwas stehen
hat, nahm jeder aktualisierten Seite mit leerer Zelle sämtliche Schlagwörter
weg — ohne eine Zeile im Bericht. Bei fünfhundert Zeilen, von denen dreihundert
in dieser Spalte leer sind, sind das dreihundert Seiten.

Die Regel dafür war längst getroffen und an drei anderen Stellen umgesetzt: eine
leere Zelle sagt **nichts** über ihr Fach, weil eine Tabelle den Unterschied
zwischen „ist jetzt leer" und „steht nicht drin" gar nicht ausdrücken kann. Der
Text, der Zustand und jedes eigene Feld folgten ihr bereits; die Schlagwörter
waren als einzige auf der alten Regel stehen geblieben. Eine Vorgabe, die auf
dem Zuordnungsbildschirm eingetragen wurde, zählt weiterhin als Aussage.

**Der Tabellenimport schrieb in Übersetzungen.** Seit Fassung 1.6 ist eine
Adresse je Sprache eindeutig, und die Verwaltung schlägt für eine Übersetzung
absichtlich dieselbe Adresse vor. Der Import legt seine Seiten immer in der
Hauptsprache an, suchte die Adresse aber **ohne** Sprache — und bekam
zurück, was die Datenbank zuerst hergab. Gibt es `/kontakt` nur auf
Französisch, so galt die deutsche Zeile als Aktualisierung: mit der Einstellung
„aktualisieren" wurden Titel, Text und Zustand der französischen Seite
überschrieben, eine Fassung angelegt und „1 aktualisiert" gemeldet; mit
„übergehen" wurde die deutsche Seite nie angelegt, obwohl ihre Adresse frei war.

**Ein Pflichtfeld für Beiträge machte jeden Tabellenimport unmöglich.** Der
Importer prüfte die Zeilen gegen *alle* eigenen Felder der Website statt gegen
die, die auf eine Seite gehören — er legt aber ausschliesslich Seiten an. Ein
Feld mit „gilt für: Beiträge" und Häkchen bei „Pflicht" wies damit jede Zeile
jeder Datei ab, mit einer Meldung, die ein Feld nennt, nach dem das
Seitenformular nie fragt. In die andere Richtung: ein Wert für ein solches Feld
wurde gespeichert, wo ihn nie jemand liest, und beim nächsten Speichern aus dem
Formular wortlos verworfen. Beim Aktualisieren entscheidet jetzt die Art der
**bestehenden** Seite — ein Beitrag wird gegen die Felder eines Beitrags
geprüft, sonst verlöre er beim Import genau die Werte, die er trägt.

**Ein gespeichertes „nein" wurde als „ja" gedruckt.** Die Prüfung der eigenen
Felder hatte für jede Feldart einen Zweig ausser für „Ja/Nein". Dort kam alles
durch, und gelesen wird beim Anzeigen alles als *ja*, was nicht „0" ist. Über
das Formular war das nicht zu erreichen, über ein Archiv oder den Assistenten
schon: ein Feld mit dem Wert „nein" erschien auf jedem ausgelieferten Theme als
„ja". Es gibt jetzt genau eine Lesart eines Ja/Nein — `field.NormalizeBool` —,
die Prüfung weist zurück, was sie nicht lesen kann, und beim Speichern wird die
Schreibweise festgelegt, damit ein Wort nicht sein Gegenteil bedeutet.

**Eine Uhrzeit erreichte das Theme als `09:30:00`.** Die Spezifikation
verspricht `HH:MM`; manche Browser schicken die Sekunden mit, und der
Tabellenimport reichte die Zelle roh durch. Beide Seiten sind behoben — beim
Lesen und beim Schreiben —, damit auch das stimmt, was schon in der Spalte
steht.

**`NaN` lag in jedem Bereich.** Ein Bereichsfeld hat zwei Grenzen, und für
`NaN` sind beide Vergleiche falsch, es liegt also zwischen allen. Eine
Tabellenzelle mit `NaN` — was Excel bei einem Rechenfehler schreibt — kam
durch die Prüfung und stand danach auf der Seite. Dieselbe Lesart machte auch
eine als `NaN` eingetragene *Grenze* zu einer Grenze, die nichts durchsetzt.

### Gehärtet

**Textbausteine und Produktgalerien tragen die Website jetzt im Speicher.** Bei
den Textbausteinen war nichts kaputt: alle vier Aufrufer verglichen die Website
selbst, und einer hatte den Vergleich sogar schon in eine eigene Funktion
gezogen — mit dem Kommentar, dass genau diese Ungleichheit der Grund dafür sei.
Er hatte recht, und der Vergleich ist jetzt dort, wo er hingehört: in der
`WHERE`-Bedingung, wo der Übersetzer jeden Aufrufer nennt. Richtig, weil sich
vier Leute erinnert haben, ist nicht dasselbe wie richtig.

`product_media` ist die dritte Tabelle dieses Projekts, deren Zeilen über eine
zweite Kennung allein angesprochen werden — die beiden anderen sind die, in
denen dieser Fehler schon zweimal ausgeliefert wurde. Sie hat noch keinen
Aufrufer, und genau deshalb steht der Wächter jetzt darin: vor dem ersten
Aufrufer statt nach dem ersten Bericht.


## 1.9 — 2026-09-05

### Fixed

**A changed template reached nobody who already knew the website.** Every
template asset went out with `Cache-Control: public, max-age=31536000, immutable`
— on an address that never changes. To a browser, `immutable` means: do not ask,
not even on a reload. A corrected stylesheet therefore reached only those who had
never opened the page, for a year.

Without a version in the address the rule is now one hour with a strong ETag: the
second request is a revalidation and comes back as a 304 with no body. Anyone who
versions the address (`/t/style.css?v=…`) still gets the long promise — and then
the address keeps it.

**Images from an imported archive had no dimensions.** The upload path measures
every image and creates the scaled-down versions; the archive path only created
the row. On a website built that way, no image had `width`/`height` in the HTML —
the layout jumps as things load — and there was **no `srcset`**, so a phone
downloaded every original at full size. On a website with eighty-five images that
is the difference between a few hundred kilobytes and several megabytes.

A background job now fills that in: at startup and hourly thereafter, a hundred
images per pass. It is self-limiting — an image that has its dimensions is never
looked at again — and needs no attention. The `thumbnails` subcommand does the
same by hand and remains for when you do not want to wait.

Filled in afterwards and not inside the import itself: a website with a hundred
images would otherwise decode a hundred images inside one HTTP request, and an
import that runs into a timeout is worse than images that come into focus a few
minutes later.

## 1.8 — 2026-09-05

### Fixed

**The Weide template now states its own gallery crop.** Its comment had always
described it — "the gallery image is cropped and fills its frame" — but the rule
for it came from `bausteine.css` and was dropped there in 1.7. Between 1.7 and
this version the images in a Weide gallery stood side by side in their own
shapes, which with a mix of portrait and landscape gives a crooked row. If you run
1.7 with this template, please apply this version.

## 1.7 — 2026-09-05

### Fixed

**The gallery cropped every image to 4:3.** A screenshot at 1.94:1 lost nearly a
third of its width that way — on the left and the right, which is exactly where an
application keeps its margins, its toolbars and its navigation. What remained
visible was a detail from the middle that looks like a mistake at upload.

The gallery now shows an image in its own shape, the way the three other image
places — image, image and text, and the images of a website's own block kinds —
always have. A grid stays even as long as the images in a gallery share a shape;
where they differ in height they align at the top. The card keeps its tile: there
the image is a teaser and not the content.

A template that wants a fixed tile still sets one itself. Rudel does that with
`aspect-ratio: 1` for its dog pictures — and since then also states the crop that
goes with it, which it previously inherited from here. **If you run a template of
your own that sets an aspect ratio in the gallery, please check that
`object-fit: cover` stands beside it**; without that line the image is now
squashed instead of cropped.

## 1.6 — 2026-09-04

### Fixed

**Editing a page discarded its blocks of the website's own kinds.** Saving an
existing page in the editor lost every block whose kind the website had created
for itself — silently, without a message, and with an apparently successful save.
The nine built-in kinds survived, everything else was discarded during cleanup,
because the save path lacked the list of the website's own kinds. From the same
cause, the button for one of your own kinds created no block, and the kinds
disappeared from the menu after every change, so the loss could not even be undone
in the editor.

The text was not lost in the process: the fields of a discarded block are still in
the page's body text. What was lost was the arrangement. If you use your own block
kinds, please look through the pages that have been saved in the editor since you
installed; a website's archive (`holzcloud.json`) contains the original blocks
with all their field values and is the reliable source for rebuilding.

Newly created pages were never affected — only editing.

## 1.5 — 2026-09-03

### Fixed

**An address now exists per language.** Until now an address was unique once per
website, across all languages. The French version of `/holzcloud-cms` silently got
a "-2" appended when it was created, and because translation links are resolved by
address, they all then pointed at whichever language was imported last. A
five-language website had five home pages, four of them wrongly linked, not a
single `hreflang` in the head, and a language picker that fell back to the home
page. Anyone running a multilingual website who uses product names as addresses
was certainly affected; migration 00045 tidies it up with nothing to do by hand.
Addresses already renamed do stay as they are, however — a "-2" is not taken back,
because by now it may have become a linked location.

**The home page had two addresses.** It was available at `/` and at `/home`, both
with themselves as the canonical address and both in the sitemap — with five
languages, ten addresses for five pages. `/home` now redirects permanently (301)
to the root of its language and is no longer in the sitemap.

**The button in the call-to-action block was invisible in the Holzcloud
template.** Brass lettering on a brass surface, because the rule for links in body
text comes later than the one for the button.

**Several paragraphs in one text block stood without spacing between them**, also
in the Holzcloud template.

**The import report announced text fields as missing images.** "the file 'Next.js'
is missing" — what was checked was the spelling of the value rather than the kind
of the field.

### Added

**The Holzcloud template dresses four of a website's own block kinds**, when a
website creates them: `vorspann`, `merkmal`, `stand` and `technik`. With those you
can set an opening sentence, a list of facts, a status line and a row of keywords,
for which the editor otherwise has no markup.

### Changed

**The publication date appears in the Holzcloud template only on a post**, no
longer on every page.

## 1.4 — 2026-09-03

This is the first public release and therefore the first entry in this file. The
project was developed in a private repository before that. There are no entries
here for that time, because there were no public releases in it; inventing some
would be worth less than nothing. The "Versioning" section in the README says the
same thing from the other direction.

### What 1.4 is

A self-hosted CMS as a single Go binary, without CGO, with SQLite as its store.
One installation carries several websites with several domains each, strictly
separated from one another. Pages are made in Markdown or out of blocks; which
fields, block kinds and content kinds a website knows it decides for itself,
without a new version of the program being needed for it. Along with that:
uploadable templates, a multilingual public website and a multilingual admin,
media, menus, SEO, a compulsory second factor for administrator accounts, and the
export and import of a whole website as a readable archive. The complete list is
under "Features" in the README; repeating it here would mean maintaining it in two
places.

### Added

The eighth built-in template is called "Holzcloud" and brings the design of
holzcloud.ch along as a theme, together with the fonts that belong to it. The
fonts are in the template itself and are shipped with it; this theme too loads
nothing from a foreign server while it runs.

### Changed

**Builds are for linux/amd64 only.** arm64 and the Raspberry Pi are out of the
build plan and out of the description. Anyone who expected an arm build gets none
any more and has to build it themselves. In exchange, a release workflow publishes
a finished binary with a checksum on every `v*` tag, instead of every installation
compiling it by hand.

The admin now shows the notice required by AGPL §13: the running version and the
reference to the source, in all five interface languages. Anyone running this
software for others thereby fulfils an obligation that was previously open.

### Security

Go is raised to 1.26.6. That closes eight vulnerabilities in the standard library
that this program actually reached; `govulncheck` reports none afterwards. The
call now runs as its own step in the security workflow, so that the next
vulnerability is not first noticed during a release check. That is the reason not
to keep running an older version.
