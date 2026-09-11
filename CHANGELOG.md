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
