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

## Unveröffentlicht

### Behoben

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
