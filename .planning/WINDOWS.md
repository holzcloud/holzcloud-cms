---
schema_version: 1
open_count: 19
waived_count: 2
fixed_count: 10
total_count: 31
last_updated: 2026-09-11T00:00:00.000Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | quick-260903-bsk | deviation | internal/i18n/locales/en.json |  | tools/i18n writeCatalog emits flush-left JSON while the four full catalogues carried a two-space indent; -write reformatted ~2250 lines each. Tool format kept as canonical. | waived | Accepted during execution, not an open defect: the tool's flush-left format is canonical (de-CH, fr-CH and it-CH were already flush-left); the two-space indent in the four full catalogues was drift from a hand-translation pass. | 2026-09-03T06:49:26.839Z | 2026-09-03T06:49:41.839Z |
| 2 | 07 | deviation | internal/field/field.go |  | trimTo schneidet einen Wert bei MaxValueBytes still ab; bei einem mehrwertigen Feld halbiert das einen Wert. Vorbestehend, D-13, gehoert Plan 07-04 (melden statt abschneiden) | fixed |  | 2026-09-05T13:53:52.821Z | 2026-09-05T14:51:59.731Z |
| 3 | 07 | deviation | internal/field/field.go | 674 | Die Ablehnungsgruende in field.go (rangeReason 674-682, die Laengen- und Mehrwert-Meldungen in Check 710 und 778-781) werden durch blosse Zeichenkettenverkettung gebaut, ohne i18n.N. Sie werden deshalb nie extrahiert und nie uebersetzt: bei englischer UI erschien 'Ausstattung: hoechstens 3 Werte, ausgewaehlt sind 4.' auf Deutsch, waehrend die Artnamen daneben uebersetzt waren. Vorbestehend, kein Regress aus Phase 7 (gegen 60ff5b2 geprueft: field.go gab dort schon rohes Deutsch zurueck); Phase 7 ist der Konvention gefolgt und hat die Flaeche verbreitert. Relevant, weil go run ./tools/i18n '0 offen, 0 verwaist' meldet, ohne diese Zeichenketten je zu sehen — der QUAL-01-Zaehler misst sie nicht. Zurueckgestellt: jede Validierungsrueckgabe in field.go umzuschreiben ist eigene Arbeit. | fixed | Geschlossen in Welle 12-04 von v2.0. field.Check, rangeReason, tooLong und die Mehrwert-Meldungen geben kein fertiges Deutsch mehr zurueck, sondern ein field.Reason — ein mit i18n.N markiertes Format plus seine Argumente. Die Beschriftung des Betreibers reist als %s und wird nie uebersetzt; der Satz entsteht dort, wo die Sprache des Lesers bekannt ist. 20 vorher unsichtbare Zeichenketten stehen jetzt im Katalog, in en/es/fr/it uebersetzt. Gehalten von TestAFieldRefusalIsTranslatedAndTheLabelIsNot und TestEveryKindsRefusalIsAFormatPlusArgumentsAndNotASentence; Gegenprobe gefahren (Katalogeintrag geleert -> rot). | 2026-09-05T16:13:58.341Z | 2026-09-11T00:00:00.000Z |
| 4 | 08 | deviation | .planning/phases/07-field-kinds/07-SECURITY.md | 183 | W-4 nennt 'T-07-26 (Plan 05)', die Nummer gehoert aber Plan 06 (Information Disclosure ueber field.Hidden). Vorbestehende Verwechslung, beim Schliessen von T-07-26 in Plan 08-04 gefunden und bewusst nicht angefasst (der Plan verbietet Aenderungen an anderen Eintraegen; welche Nummer richtig waere, liesse sich nur raten). Folge: das Zaehltor grep -c 'T-07-26' misst 2 statt 1. | open |  | 2026-09-06T10:24:52.821Z |  |
| 5 | 08 | deviation | cmd/holzcloud/templates/admin/field_list.html | 16 | field_list.html druckt &#8592; als Text statt als Pfeil (alle drei Rueckwege); Aenderung verwaist drei Katalogschluessel, darum zurueckgestellt — BERICHTIGT: diese Begruendung des Aufschubs ruhte auf einer falschen Annahme darueber, worin der Flick besteht. Sie gilt allein fuer den Flick, den deferred-items.md vorschlaegt (die Entitaet durch das Zeichen ersetzen). Der tatsaechlich gefahrene Flick wechselt an denselben drei Stellen nur die aufrufende Funktion von t auf die HTML-durchlassende Fassung th; die Zeichenkette bleibt byte-gleich, kein Schluessel verwaist, kein Katalog wurde angefasst, und der Zaehler stand vorher wie nachher auf 1158 Zeichenketten mit 0 offen, 0 verwaist fuer en/es/fr/it. Geschlossen im Schnellauftrag 260906-m9z am 2026-09-06. | fixed |  | 2026-09-06T10:55:06.562Z | 2026-09-06T14:14:47.478Z |
| 6 | 11 | deviation | internal/bundle/import.go |  | Report.Warnings baut jeden Satz mit fmt.Sprintf und rohem Deutsch. CLAUDE.md haelt seit 7e0c834 ausdruecklich fest, dass ein mit fmt.Sprintf gebauter Satz fuer tools/i18n unsichtbar ist. Vorbestehend: rund 30 solche Warnungen standen schon vor Plan 11-06 in dieser Datei; 11-06 hat vier weitere in derselben Form ergaenzt (importAlbums, missingAlbum), weil die Alternative den Locale des Bedieners durch bundle.Import zu faedeln waere und der Bericht sonst in der Sprache der importierten Website erschiene statt in der des Bedieners. Der richtige Flick ist die Form, die .planning/GLOSSARY.md fuer csvimport schon vorschreibt: Code plus Argumente statt fertigem Satz (D-32). Folge: der Zaehler go run ./tools/i18n sieht keine dieser Zeilen. | open |  | 2026-09-07T23:07:58.770Z |  |
| 7 | 10 | deviation | internal/admin/forwardauth.go |  | The plan 10-04 verify gate 'grep secret\|password \| grep -c slog.' reads a proxy: gofmt wraps slog.Info across lines, so a secret appended to a continuation line keeps the gate at 0. Held by TestTheProvisioningSecretAppearsInNoLogLine instead. | open |  | 2026-09-08T04:44:33.668Z |  |
| 8 | 11 | deviation | internal/block/render.go | 212 | Eine Album-Galerie mit Diashow-Darstellung zeigt ihre Lichtkasten-Bedienelemente in der Sprache des Besuchers und den Namen ihres Schiebefelds auf Deutsch — auf derselben Seite, im selben Durchgang. Im Browser gemessen am 2026-09-08: Website auf Englisch, /: 'Next image \| Previous image \| Close large view' neben aria-label="Galerie"; auf Spanisch: 'Imagen siguiente \| Imagen anterior \| Cerrar la vista grande' neben aria-label="Galerie". Ursache: render.go:212 uebersetzt den Regionsnamen mit s.text (block.Set.T, das internal/admin/page_blocks.go NUR beim Speichern setzt), waehrend die Bedienelemente in GalleryItems bei einer Album-Galerie ueber internal/album/expand.go:133 set.t bekommen, den Uebersetzer der Anfrage. Der Kommentar ueber textGallery behauptet, die Wiederverwendung von 'Galerie' koste nichts, weil der Schluessel 'in en, es, fr und it heute uebersetzt ist' — er wird nie uebersetzt gerendert. Genau die Klasse Fehler, die das i18n-Tor nicht sieht: markiert, gesammelt, viermal uebersetzt, 0 offen 0 verwaist, und trotzdem deutsch beim Besucher. Betrifft nur den Vorlese-Namen des Schiebefelds. Die Behebung verschiebt die Grenze zwischen dem, was eine Galerie beim Speichern einfriert, und dem, was sie bei der Anfrage aufloest — eine Architekturfrage (Regel 4), deshalb hier festgehalten und nicht am Phasenende gemacht. | open |  | 2026-09-08T05:10:24.057Z |  |
| 9 | 10 | deviation | internal/admin/forwardauth.go |  | 10-05: the plan's HasGroup counting gate counts its own explanatory comment (prints 3, wants 1); the corrected gate adds grep -v '//' and prints 1 | open |  | 2026-09-08T05:24:33.848Z |  |
| 10 | 10 | unmet-truth | cmd/holzcloud/templates/admin/account.html |  | Neue Zeichenkette noch nicht uebersetzt: en/es/fr/it je 2 offen (Kontobildschirm + Benutzerliste). Plan 10-09 schliesst sie mit tools/i18n -write. | fixed |  | 2026-09-08T05:47:40.320Z | 2026-09-08T06:26:25.942Z |
| 11 | 10 | deviation | .planning/phases/10-authentik/10-08-PLAN.md |  | 10-08: the isTrustedProxy gate excludes '^\./\.planning/', but grep on this machine emits paths without a './' prefix, so the exclusion never fires — the gate reads 14 instead of 0. Corrected form: grep -v '^\(\./\)\?\.planning/', which reads 0. Fifth instance of 'a gate must measure what its name claims' in this phase. | open |  | 2026-09-08T06:09:24.781Z |  |
| 12 | 10 | deviation | .planning/phases/10-authentik/10-09-PLAN.md |  | 10-09: two more counting gates measure something other than their name, sixth and seventh instance in this phase. (a) completeLogin: the gate excludes 'func (h *Handler) completeLogin' and '// completeLogin' but forwardauth.go:241 mentions the symbol mid-sentence inside a comment, so the gate prints 5 where the plan wants 4; grep 'h\.completeLogin(' prints 4 (3 at the phase baseline, +1 from 10-03) and is the form that measures call sites. (b) MustHaveSecondFactor: 'grep -rn ... \| wc -l' counts a doc comment, the func declaration and, since 10-06, one new prose comment at admin/twofactor.go:383 — it prints 7 -> 8 while the actual call sites are unchanged at 5 -> 5, so the plan's 'Phase adds 0' is true of the property and false of the number. Same family as entry 9. Additionally the plan's absolute gates for migrations (49) and BeginTx files (14) were overtaken by Phase 11 landing between waves: measured 51 and 15, with 00050_albums.sql (11-02) and 00051_album_updated_at.sql (11-CR-01) attributed by git log --diff-filter=A, and zero migrations added by Phase 10. | open |  | 2026-09-08T06:26:43.345Z |  |
| 13 | 10 | deviation | internal/admin/field.go:311 |  | 10-10 browser pass: the field screen's four page titles are raw German literals (Felder – %s, Baustein "…" – %s, Textbaustein "…" – %s, Gruppe "…" – %s). The snippet one is Phase 8's own (48e5b1d). They are never collected, so 'go run ./tools/i18n' reports 0 offen while <title> and <h1> read German on an English admin. Seen on screen in all four modes. | fixed | Geschlossen 2026-09-09 im Commit 46e0722: alle vier Titel gehen jetzt durch web.Titlef, vier neue Schluessel in en/es/fr/it uebersetzt, de-CH per -schweiz neu erzeugt (drei Abweichungen). Rot vorher durch TestTheFieldScreenTitleIsTranslated, gruen nachher, wieder rot beim Leeren von "Felder – %s" in en.json. Die vier uebrigen deutschen Saetze dieses Durchgangs (Eintraege 12, 14, 15, 16) sind vorbestehend und gehoeren Phase 12. | 2026-09-08T07:27:26.865Z | 2026-09-09T00:00:00.000Z |
| 14 | 10 | deviation | internal/auth/middleware.go:109 |  | 10-10 browser pass: RequireWebsiteAccess refuses with http.Error("Diese Website gehoert nicht zu deinem Zugang."), an uncollected German literal. Seen as the 403 body when an SSO editor opened a website outside their access on an English admin. The sibling sentence 'Veroeffentlichen gehoert nicht zu deinem Zugang' IS in all four catalogues. | open |  | 2026-09-08T07:27:26.996Z |  |
| 15 | 10 | deviation | internal/admin/starter.go:181 |  | 10-10 browser pass: starterContentSummary() returns a German sentence built with fmt.Sprintf and never marked; the flash after creating a website reads German on an English admin. Invisible to a German-character gate anchored on 'title=' or 'flash', because it is returned from a helper. | open |  | 2026-09-08T07:27:27.127Z |  |
| 16 | 10 | deviation | internal/admin/media.go:176 |  | 10-10 browser pass: the media-upload flash is concatenated from three uncollected German fragments (message := "Datei hochgeladen" + ' – ' + warning + ' – bitte noch eine Bildbeschreibung eintragen'). Seen in German on an English admin after uploading an image. | open |  | 2026-09-08T07:27:27.260Z |  |
| 17 | 10 | deviation | internal/admin/twofactor.go:283 |  | 10-10 browser pass: the refusal that stops an administrator switching off their own second factor (the MustHaveSecondFactor call site 10-CONTEXT names) flashes three concatenated uncollected German fragments. The guard itself is correct and was driven; only its wording is untranslated. | fixed | Geschlossen in c2dc0b3 (Rotbeweis 029660d): die Ablehnung ist ein Literal, in en/es/fr/it uebersetzt, de-CH per -schweiz; TestSecondFactorRefusalIsTranslated haelt die Uebersetzung, das i18n-Tor die Sichtbarkeit fuer den Sammler (Probe G50: Verkettung -> 1 verwaist). | 2026-09-08T07:27:27.391Z | 2026-09-10T18:00:00.000Z |
| 18 | 10 | deviation | internal/field/field.go |  | v1.6s eigene deutsche Saetze am Katalog vorbei, gemessen im Phase-10-Abschluss: neun Ablehnungsgruende in field.Check (d.Label + Literal) erreichen ueber field.CheckAll Seitenformular, Textbausteinformular, CSV-Importbericht und KI-Werkzeuge; sechs errors.New-Saetze in internal/field/store.go erreichen die Meldezeile ueber den err.Error()-Rueckfall in internal/admin/field.go; die Kopfzeile der CSV-Beispieltabelle ist deutsch. Sonde mit lang=en: 422 und 'Preis muss eine Zahl sein.'. Das i18n-Tor sieht keinen davon. Bewusst an Phase 12 uebergeben (Kriterium 9, dieselbe Klasse wie Eintrag 3): field.CheckAll hat sechs Aufrufer bis in den CSV-Import, und ein Umbau am Rand eines Meilenstein-Abschlusses waere genau die Aenderung, die nach dem letzten Browserdurchgang landet. | fixed | Geschlossen in Welle 12-04 von v2.0, alle drei Teile. Die neun Ablehnungsgruende in field.Check sind field.Reason (siehe Eintrag 3). Die errors.New-Saetze in field/store.go, block/store.go und kind/store.go sind mit i18n.N markiert — web.SetFlashError laesst sie ohnehin durch i18n.T, also uebersetzen sie jetzt. Der err.Error()-Rueckfall in admin/field.go und admin/blocktype.go ist durch ein gesammeltes 'Speichern fehlgeschlagen.' ersetzt, der deutsche fmt.Errorf-Wrap geht nach slog.Error, wo sein Detail etwas wert ist; ErrNoGroup, ErrNoSnippet und ErrNoBlockType werden namentlich beantwortet, statt ueber fmt.Errorf('%w: %s') zusammengesetzt auf den Bildschirm zu fallen. Die Kopfzeile der CSV-Beispieltabelle wird in der Sprache des Betreibers geschrieben, und csvimport.fixedSpellings kennt die uebersetzten Schreibweisen, damit die Datei zurueckkommt. Die Musterzeile bleibt bewusst deutsch: 'entwurf' gehoert zum geschlossenen Statuswortschatz, den der Einleser zurueckliest. | 2026-09-10T18:00:00.000Z | 2026-09-11T00:00:00.000Z |
| 19 | 10 | deviation | cmd/holzcloud/main.go |  | Code-Durchgang WR-07: RequireFreshPassword verlangt vor fuenf Aktionen (Website loeschen, Nutzer loeschen, KI-Schluessel anlegen, Plugin entfernen, Protokoll aufraeumen — im Code nachgesehen) ein Passwort, das ein nur ueber SSO bereitgestelltes Konto nicht kennt. Keine Sicherheitswirkung. In deploy/DEPLOY.md 'Three things single sign-on does not do yet' beschrieben, Ausweg holzcloud user passwd. Ein Flick (Bestaetigung ueber eine frische Anmeldung beim Ausweisdienst) ist offen. | open |  | 2026-09-10T18:00:00.000Z |  |
| 20 | 10 | deviation | internal/admin/login.go |  | Code-Durchgang IN-06: wer bei eingeschaltetem SSO eine Passwortsitzung abmeldet, wird beim naechsten Klick ueber den Ausweisdienst wieder angemeldet, wenn der Browser dort noch eine Sitzung hat und das Konto verknuepft ist. In deploy/DEPLOY.md beschrieben. | open |  | 2026-09-10T18:00:00.000Z |  |
| 21 | 10 | deviation | internal/admin/forwardauth.go |  | Eine von Hand im Nutzerformular entzogene Website kehrt bei der naechsten Anfrage der SSO-Sitzung zurueck, solange HOLZCLOUD_SSO_WEBSITE_GROUPS gesetzt ist; kein Bildschirm sagt das. In deploy/DEPLOY.md beschrieben. | open |  | 2026-09-10T18:00:00.000Z |  |
| 22 | 10 | deviation | internal/admin/forwardauth.go |  | Rechtezeilen der SSO-Abgleichung tragen user_id NULL, wenn sie bei der Bereitstellung entstehen (noch keine Sitzung) und sind ueber den Benutzerfilter des Protokolls nicht auffindbar. Im Browserdurchgang gemessen: in einer laufenden Sitzung traegt die Zeile das Konto (A3/A4), NULL nur bei der Bereitstellung (A5) — der Befund ist enger als im Nachlauf formuliert. | open |  | 2026-09-10T18:00:00.000Z |  |
| 23 | 10 | deviation | tools/i18n/main.go |  | go run ./tools/i18n -schweiz entfernt einen verwaisten de-CH-Eintrag nicht, auch nicht beim zweiten Lauf. Nicht still: TestFassungKeysExistInTheSource wird rot. Einmal von Hand entfernt (e306eeb). | open |  | 2026-09-10T18:00:00.000Z |  |
| 24 | 10 | deviation | internal/web/headers.go |  | Verwaltungsantworten tragen 'Vary: Cookie' zweimal (im Browserdurchgang an POST /admin/logout gesehen: 'Cookie, Cookie, HX-Request'). Vorbestehend, harmlos, nicht aus Phase 10. | open |  | 2026-09-10T18:00:00.000Z |  |
| 25 | 10 | deviation | internal/i18n/locales/fr-CH.json |  | T-10-52: fr-CH und it-CH werden von Hand gepflegt; eine formgerechte Handaenderung besteht Werkzeug, Tests und CI. Konstruktionsbedingt. | waived | Konstruktionsbedingt: die Regionalkataloge fr-CH und it-CH sind Abweichungslisten, die bewusst von Hand gepflegt werden; ein maschineller Pruefer muesste die Sprache verstehen. Ein Schluessel ohne Gegenstueck im Quelltext wird weiterhin von TestFassungKeysExistInTheSource gefangen. | 2026-09-10T18:00:00.000Z |  |
| 26 | 10 | deviation | internal/admin/forwardauth.go |  | Eine verweigerte SSO-Identitaet schreibt bei JEDER Anfrage eine auth.login_fail-Zeile, ungebremst (im Browserdurchgang A6: vier Zeilen fuer zwei Laeufe). Ein Proxy, der dieselbe verweigerte Identitaet dauernd behauptet, laesst das Taetigkeitsprotokoll unbegrenzt wachsen. Die Anmeldebremse bewusst nicht zu fuettern (T-10-20, gehalten) schliesst die naheliegende Loesung aus. | open |  | 2026-09-10T18:00:00.000Z |  |
| 27 | 10 | deviation | internal/admin/forwardauth.go |  | Kein Abbau: ein durch SSO angelegtes Konto ueberlebt die Identitaet, die es erzeugt hat. Nimmt der Ausweisdienst jemanden heraus, bleibt das Konto hier bestehen (erreichbar nur noch ueber ein gesetztes Passwort oder eine neue Verknuepfung). | open |  | 2026-09-10T18:00:00.000Z |  |
| 28 | 10 | deviation | internal/admin/forwardauth.go |  | Protokolllücken bei SSO-Verweigerungen: die auth.login_fail-Zeile traegt den Grund (not_linked, no_website_group, …) nicht, nur das Serverlog; ein misslungenes RenewToken verweigert ohne Protokollzeile; die Abmeldezeile unterscheidet nicht, welcher Weg gegangen wurde. | open |  | 2026-09-10T18:00:00.000Z |  |
| 29 | audit-v1.6 | deviation | internal/field/field.go | 1093 | Meilenstein-Audit v1.6: field.CheckAll baut die Zeilenmeldung einer Gruppe mit fmt.Sprintf("%s, Zeile %d: %s", def.Label, i+1, reason) an zwei Stellen (field.go:1093 Laengenpruefung verborgener Zeilen, :1113 Check pro Unterfeld). Der Rahmen 'Zeile %d' ist fuer tools/i18n unsichtbar, auch wenn reason selbst eines Tages katalogisiert ist. Dieselbe Klasse wie Eintrag 18, aber eine eigene Stelle, die 18 nicht nennt. Gefunden vom Integrationspruefer, im Code gelesen. Gehoert zu Phase 12 Kriterium 9 (v2.0). | fixed | Geschlossen in Welle 12-04 von v2.0. Der Rahmen ist keine fmt.Sprintf mehr, sondern field.inRow — ein Reason, der einen Reason traegt; Reason.Text loest verschachtelt auf, in einer Sprache. Gehalten von TestTheRowFrameAroundAGroupReasonIsTranslatedToo, Gegenprobe gefahren: Rahmenschluessel geleert -> 'Oeffnungszeiten, Zeile 1: Von has to be a time' wird gemeldet, also genau die halb uebersetzte Zeile, die dieser Eintrag beschreibt. | 2026-09-10T15:24:10.774Z | 2026-09-11T00:00:00.000Z |
| 30 | 12 | deviation | internal/field/field.go |  | field.SlugifyKey trug eine eigene Transliterationsliste mit vier Eintraegen (ae, oe, ue, ss) und liess jeden anderen Akzentbuchstaben ganz fallen: 'Titulo' mit Akut wurde 'ttulo', 'Etat' wurde 'tat', 'Cafe' mit Akut wurde 'caf'. page.Transliterate kennt den vollen Latin-1-Satz seit jeher, und field.go argumentiert das Prinzip in seinem eigenen KindTerm-Zweig: zwei Niederschriften einer Regel sind zwei Regeln, die auseinanderlaufen. Gefunden am 2026-09-11 beim Uebersetzen der CSV-Beispielkopfzeile und im selben Commit behoben. Nur NEUE Feldkennungen aendern sich; eine Kennung wird einmal gepraegt und steht dann. csvimport.settleHeaderMarks war auf genau diesen Fehler geeicht und faellt damit weg — die zweite Faltung war nie ueber Kopfzeilen, sie war ein Spiegel des Fehlers. | fixed | Im selben Commit behoben, in dem er gefunden wurde. TestFoldHeaderAgreesOnEveryOtherAccent haelt die Eigenschaft weiter und traegt jetzt die richtigen Schluessel statt der verlustbehafteten. | 2026-09-11T00:00:00.000Z | 2026-09-11T00:00:00.000Z |
| 31 | 12 | deviation | internal/i18n/locales/en.json |  | Der Katalog warf 'Schlagwort'/'Schlagwoerter' und 'Beschriftung' auf dasselbe englische Wort (Label/Labels), ebenso im Spanischen (Etiqueta) und Italienischen (Etichetta). GLOSSARY.md fuehrt das seit 2026-09-06 als vorbestehenden Uebersetzungsfehler, den das Umdrehen der Quellsprache aufdecken wuerde. Aufgedeckt hat ihn etwas anderes: die uebersetzte Kopfzeile der CSV-Beispieldatei traegt 'Labels', und csvimport.fixedSpellings kennt das Wort nicht — die heruntergeladene Datei waere beim Hochladen mit einer unzugeordneten Spalte zurueckgekommen. Behoben: Schlagwort/Schlagwoerter heissen Term/Terms, Termino/Terminos, Termine/Termini; Beschriftung behaelt Label. Franzoesisch hatte die Kollision nicht. | fixed | Im selben Commit behoben. Gehalten von TestEveryTranslatedExampleHeadingIsRecognisedOnTheWayBackIn, Gegenprobe gefahren: 'Labels' wieder eingesetzt -> rot. | 2026-09-11T00:00:00.000Z | 2026-09-11T00:00:00.000Z |

````json
[
  {
    "id": 1,
    "kind": "deviation",
    "phase": "quick-260903-bsk",
    "file": "internal/i18n/locales/en.json",
    "line": null,
    "description": "tools/i18n writeCatalog emits flush-left JSON while the four full catalogues carried a two-space indent; -write reformatted ~2250 lines each. Tool format kept as canonical.",
    "status": "waived",
    "reason": "Accepted during execution, not an open defect: the tool's flush-left format is canonical (de-CH, fr-CH and it-CH were already flush-left); the two-space indent in the four full catalogues was drift from a hand-translation pass.",
    "recorded_at": "2026-09-03T06:49:26.839Z",
    "resolved_at": "2026-09-03T06:49:41.839Z"
  },
  {
    "id": 2,
    "kind": "deviation",
    "phase": "07",
    "file": "internal/field/field.go",
    "line": null,
    "description": "trimTo schneidet einen Wert bei MaxValueBytes still ab; bei einem mehrwertigen Feld halbiert das einen Wert. Vorbestehend, D-13, gehoert Plan 07-04 (melden statt abschneiden)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-05T13:53:52.821Z",
    "resolved_at": "2026-09-05T14:51:59.731Z"
  },
  {
    "id": 3,
    "kind": "deviation",
    "phase": "07",
    "file": "internal/field/field.go",
    "line": 674,
    "description": "Die Ablehnungsgruende in field.go (rangeReason 674-682, die Laengen- und Mehrwert-Meldungen in Check 710 und 778-781) werden durch blosse Zeichenkettenverkettung gebaut, ohne i18n.N. Sie werden deshalb nie extrahiert und nie uebersetzt: bei englischer UI erschien 'Ausstattung: hoechstens 3 Werte, ausgewaehlt sind 4.' auf Deutsch, waehrend die Artnamen daneben uebersetzt waren. Vorbestehend, kein Regress aus Phase 7 (gegen 60ff5b2 geprueft: field.go gab dort schon rohes Deutsch zurueck); Phase 7 ist der Konvention gefolgt und hat die Flaeche verbreitert. Relevant, weil go run ./tools/i18n '0 offen, 0 verwaist' meldet, ohne diese Zeichenketten je zu sehen — der QUAL-01-Zaehler misst sie nicht. Zurueckgestellt: jede Validierungsrueckgabe in field.go umzuschreiben ist eigene Arbeit.",
    "status": "fixed",
    "reason": "Geschlossen in Welle 12-04 von v2.0. field.Check, rangeReason, tooLong und die Mehrwert-Meldungen geben kein fertiges Deutsch mehr zurueck, sondern ein field.Reason — ein mit i18n.N markiertes Format plus seine Argumente. Die Beschriftung des Betreibers reist als %s und wird nie uebersetzt; der Satz entsteht dort, wo die Sprache des Lesers bekannt ist. 20 vorher unsichtbare Zeichenketten stehen jetzt im Katalog, in en/es/fr/it uebersetzt. Gehalten von TestAFieldRefusalIsTranslatedAndTheLabelIsNot und TestEveryKindsRefusalIsAFormatPlusArgumentsAndNotASentence; Gegenprobe gefahren (Katalogeintrag geleert -> rot).",
    "recorded_at": "2026-09-05T16:13:58.341Z",
    "resolved_at": "2026-09-11T00:00:00.000Z"
  },
  {
    "id": 4,
    "kind": "deviation",
    "phase": "08",
    "file": ".planning/phases/07-field-kinds/07-SECURITY.md",
    "line": 183,
    "description": "W-4 nennt 'T-07-26 (Plan 05)', die Nummer gehoert aber Plan 06 (Information Disclosure ueber field.Hidden). Vorbestehende Verwechslung, beim Schliessen von T-07-26 in Plan 08-04 gefunden und bewusst nicht angefasst (der Plan verbietet Aenderungen an anderen Eintraegen; welche Nummer richtig waere, liesse sich nur raten). Folge: das Zaehltor grep -c 'T-07-26' misst 2 statt 1.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-06T10:24:52.821Z",
    "resolved_at": null
  },
  {
    "id": 5,
    "kind": "deviation",
    "phase": "08",
    "file": "cmd/holzcloud/templates/admin/field_list.html",
    "line": 16,
    "description": "field_list.html druckt &#8592; als Text statt als Pfeil (alle drei Rueckwege); Aenderung verwaist drei Katalogschluessel, darum zurueckgestellt — BERICHTIGT: diese Begruendung des Aufschubs ruhte auf einer falschen Annahme darueber, worin der Flick besteht. Sie gilt allein fuer den Flick, den deferred-items.md vorschlaegt (die Entitaet durch das Zeichen ersetzen). Der tatsaechlich gefahrene Flick wechselt an denselben drei Stellen nur die aufrufende Funktion von t auf die HTML-durchlassende Fassung th; die Zeichenkette bleibt byte-gleich, kein Schluessel verwaist, kein Katalog wurde angefasst, und der Zaehler stand vorher wie nachher auf 1158 Zeichenketten mit 0 offen, 0 verwaist fuer en/es/fr/it. Geschlossen im Schnellauftrag 260906-m9z am 2026-09-06.",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-06T10:55:06.562Z",
    "resolved_at": "2026-09-06T14:14:47.478Z"
  },
  {
    "id": 6,
    "kind": "deviation",
    "phase": "11",
    "file": "internal/bundle/import.go",
    "line": null,
    "description": "Report.Warnings baut jeden Satz mit fmt.Sprintf und rohem Deutsch. CLAUDE.md haelt seit 7e0c834 ausdruecklich fest, dass ein mit fmt.Sprintf gebauter Satz fuer tools/i18n unsichtbar ist. Vorbestehend: rund 30 solche Warnungen standen schon vor Plan 11-06 in dieser Datei; 11-06 hat vier weitere in derselben Form ergaenzt (importAlbums, missingAlbum), weil die Alternative den Locale des Bedieners durch bundle.Import zu faedeln waere und der Bericht sonst in der Sprache der importierten Website erschiene statt in der des Bedieners. Der richtige Flick ist die Form, die .planning/GLOSSARY.md fuer csvimport schon vorschreibt: Code plus Argumente statt fertigem Satz (D-32). Folge: der Zaehler go run ./tools/i18n sieht keine dieser Zeilen.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-07T23:07:58.770Z",
    "resolved_at": null
  },
  {
    "id": 7,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "The plan 10-04 verify gate 'grep secret|password | grep -c slog.' reads a proxy: gofmt wraps slog.Info across lines, so a secret appended to a continuation line keeps the gate at 0. Held by TestTheProvisioningSecretAppearsInNoLogLine instead.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T04:44:33.668Z",
    "resolved_at": null
  },
  {
    "id": 8,
    "kind": "deviation",
    "phase": "11",
    "file": "internal/block/render.go",
    "line": 212,
    "description": "Eine Album-Galerie mit Diashow-Darstellung zeigt ihre Lichtkasten-Bedienelemente in der Sprache des Besuchers und den Namen ihres Schiebefelds auf Deutsch — auf derselben Seite, im selben Durchgang. Im Browser gemessen am 2026-09-08: Website auf Englisch, /: 'Next image | Previous image | Close large view' neben aria-label=\"Galerie\"; auf Spanisch: 'Imagen siguiente | Imagen anterior | Cerrar la vista grande' neben aria-label=\"Galerie\". Ursache: render.go:212 uebersetzt den Regionsnamen mit s.text (block.Set.T, das internal/admin/page_blocks.go NUR beim Speichern setzt), waehrend die Bedienelemente in GalleryItems bei einer Album-Galerie ueber internal/album/expand.go:133 set.t bekommen, den Uebersetzer der Anfrage. Der Kommentar ueber textGallery behauptet, die Wiederverwendung von 'Galerie' koste nichts, weil der Schluessel 'in en, es, fr und it heute uebersetzt ist' — er wird nie uebersetzt gerendert. Genau die Klasse Fehler, die das i18n-Tor nicht sieht: markiert, gesammelt, viermal uebersetzt, 0 offen 0 verwaist, und trotzdem deutsch beim Besucher. Betrifft nur den Vorlese-Namen des Schiebefelds. Die Behebung verschiebt die Grenze zwischen dem, was eine Galerie beim Speichern einfriert, und dem, was sie bei der Anfrage aufloest — eine Architekturfrage (Regel 4), deshalb hier festgehalten und nicht am Phasenende gemacht.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T05:10:24.057Z",
    "resolved_at": null
  },
  {
    "id": 9,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "10-05: the plan's HasGroup counting gate counts its own explanatory comment (prints 3, wants 1); the corrected gate adds grep -v '//' and prints 1",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T05:24:33.848Z",
    "resolved_at": null
  },
  {
    "id": 10,
    "kind": "unmet-truth",
    "phase": "10",
    "file": "cmd/holzcloud/templates/admin/account.html",
    "line": null,
    "description": "Neue Zeichenkette noch nicht uebersetzt: en/es/fr/it je 2 offen (Kontobildschirm + Benutzerliste). Plan 10-09 schliesst sie mit tools/i18n -write.",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-08T05:47:40.320Z",
    "resolved_at": "2026-09-08T06:26:25.942Z"
  },
  {
    "id": 11,
    "kind": "deviation",
    "phase": "10",
    "file": ".planning/phases/10-authentik/10-08-PLAN.md",
    "line": null,
    "description": "10-08: the isTrustedProxy gate excludes '^\\./\\.planning/', but grep on this machine emits paths without a './' prefix, so the exclusion never fires — the gate reads 14 instead of 0. Corrected form: grep -v '^\\(\\./\\)\\?\\.planning/', which reads 0. Fifth instance of 'a gate must measure what its name claims' in this phase.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T06:09:24.781Z",
    "resolved_at": null
  },
  {
    "id": 12,
    "kind": "deviation",
    "phase": "10",
    "file": ".planning/phases/10-authentik/10-09-PLAN.md",
    "line": null,
    "description": "10-09: two more counting gates measure something other than their name, sixth and seventh instance in this phase. (a) completeLogin: the gate excludes 'func (h *Handler) completeLogin' and '// completeLogin' but forwardauth.go:241 mentions the symbol mid-sentence inside a comment, so the gate prints 5 where the plan wants 4; grep 'h\\.completeLogin(' prints 4 (3 at the phase baseline, +1 from 10-03) and is the form that measures call sites. (b) MustHaveSecondFactor: 'grep -rn ... | wc -l' counts a doc comment, the func declaration and, since 10-06, one new prose comment at admin/twofactor.go:383 — it prints 7 -> 8 while the actual call sites are unchanged at 5 -> 5, so the plan's 'Phase adds 0' is true of the property and false of the number. Same family as entry 9. Additionally the plan's absolute gates for migrations (49) and BeginTx files (14) were overtaken by Phase 11 landing between waves: measured 51 and 15, with 00050_albums.sql (11-02) and 00051_album_updated_at.sql (11-CR-01) attributed by git log --diff-filter=A, and zero migrations added by Phase 10.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T06:26:43.345Z",
    "resolved_at": null
  },
  {
    "id": 13,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/field.go:311",
    "line": null,
    "description": "10-10 browser pass: the field screen's four page titles are raw German literals (Felder – %s, Baustein \"…\" – %s, Textbaustein \"…\" – %s, Gruppe \"…\" – %s). The snippet one is Phase 8's own (48e5b1d). They are never collected, so 'go run ./tools/i18n' reports 0 offen while <title> and <h1> read German on an English admin. Seen on screen in all four modes.",
    "status": "fixed",
    "reason": "Geschlossen 2026-09-09 im Commit 46e0722: alle vier Titel gehen jetzt durch web.Titlef, vier neue Schluessel in en/es/fr/it uebersetzt, de-CH per -schweiz neu erzeugt (drei Abweichungen). Rot vorher durch TestTheFieldScreenTitleIsTranslated, gruen nachher, wieder rot beim Leeren von \"Felder – %s\" in en.json. Die vier uebrigen deutschen Saetze dieses Durchgangs (Eintraege 12, 14, 15, 16) sind vorbestehend und gehoeren Phase 12.",
    "recorded_at": "2026-09-08T07:27:26.865Z",
    "resolved_at": "2026-09-09T00:00:00.000Z"
  },
  {
    "id": 14,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/auth/middleware.go:109",
    "line": null,
    "description": "10-10 browser pass: RequireWebsiteAccess refuses with http.Error(\"Diese Website gehoert nicht zu deinem Zugang.\"), an uncollected German literal. Seen as the 403 body when an SSO editor opened a website outside their access on an English admin. The sibling sentence 'Veroeffentlichen gehoert nicht zu deinem Zugang' IS in all four catalogues.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T07:27:26.996Z",
    "resolved_at": null
  },
  {
    "id": 15,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/starter.go:181",
    "line": null,
    "description": "10-10 browser pass: starterContentSummary() returns a German sentence built with fmt.Sprintf and never marked; the flash after creating a website reads German on an English admin. Invisible to a German-character gate anchored on 'title=' or 'flash', because it is returned from a helper.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T07:27:27.127Z",
    "resolved_at": null
  },
  {
    "id": 16,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/media.go:176",
    "line": null,
    "description": "10-10 browser pass: the media-upload flash is concatenated from three uncollected German fragments (message := \"Datei hochgeladen\" + ' – ' + warning + ' – bitte noch eine Bildbeschreibung eintragen'). Seen in German on an English admin after uploading an image.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-08T07:27:27.260Z",
    "resolved_at": null
  },
  {
    "id": 17,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/twofactor.go:283",
    "line": null,
    "description": "10-10 browser pass: the refusal that stops an administrator switching off their own second factor (the MustHaveSecondFactor call site 10-CONTEXT names) flashes three concatenated uncollected German fragments. The guard itself is correct and was driven; only its wording is untranslated.",
    "status": "fixed",
    "reason": "Geschlossen in c2dc0b3 (Rotbeweis 029660d): die Ablehnung ist ein Literal, in en/es/fr/it uebersetzt, de-CH per -schweiz; TestSecondFactorRefusalIsTranslated haelt die Uebersetzung, das i18n-Tor die Sichtbarkeit fuer den Sammler (Probe G50: Verkettung -> 1 verwaist).",
    "recorded_at": "2026-09-08T07:27:27.391Z",
    "resolved_at": "2026-09-10T18:00:00.000Z"
  },
  {
    "id": 18,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/field/field.go",
    "line": null,
    "description": "v1.6s eigene deutsche Saetze am Katalog vorbei, gemessen im Phase-10-Abschluss: neun Ablehnungsgruende in field.Check (d.Label + Literal) erreichen ueber field.CheckAll Seitenformular, Textbausteinformular, CSV-Importbericht und KI-Werkzeuge; sechs errors.New-Saetze in internal/field/store.go erreichen die Meldezeile ueber den err.Error()-Rueckfall in internal/admin/field.go; die Kopfzeile der CSV-Beispieltabelle ist deutsch. Sonde mit lang=en: 422 und 'Preis muss eine Zahl sein.'. Das i18n-Tor sieht keinen davon. Bewusst an Phase 12 uebergeben (Kriterium 9, dieselbe Klasse wie Eintrag 3): field.CheckAll hat sechs Aufrufer bis in den CSV-Import, und ein Umbau am Rand eines Meilenstein-Abschlusses waere genau die Aenderung, die nach dem letzten Browserdurchgang landet.",
    "status": "fixed",
    "reason": "Geschlossen in Welle 12-04 von v2.0, alle drei Teile. Die neun Ablehnungsgruende in field.Check sind field.Reason (siehe Eintrag 3). Die errors.New-Saetze in field/store.go, block/store.go und kind/store.go sind mit i18n.N markiert — web.SetFlashError laesst sie ohnehin durch i18n.T, also uebersetzen sie jetzt. Der err.Error()-Rueckfall in admin/field.go und admin/blocktype.go ist durch ein gesammeltes 'Speichern fehlgeschlagen.' ersetzt, der deutsche fmt.Errorf-Wrap geht nach slog.Error, wo sein Detail etwas wert ist; ErrNoGroup, ErrNoSnippet und ErrNoBlockType werden namentlich beantwortet, statt ueber fmt.Errorf('%w: %s') zusammengesetzt auf den Bildschirm zu fallen. Die Kopfzeile der CSV-Beispieltabelle wird in der Sprache des Betreibers geschrieben, und csvimport.fixedSpellings kennt die uebersetzten Schreibweisen, damit die Datei zurueckkommt. Die Musterzeile bleibt bewusst deutsch: 'entwurf' gehoert zum geschlossenen Statuswortschatz, den der Einleser zurueckliest.",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": "2026-09-11T00:00:00.000Z"
  },
  {
    "id": 19,
    "kind": "deviation",
    "phase": "10",
    "file": "cmd/holzcloud/main.go",
    "line": null,
    "description": "Code-Durchgang WR-07: RequireFreshPassword verlangt vor fuenf Aktionen (Website loeschen, Nutzer loeschen, KI-Schluessel anlegen, Plugin entfernen, Protokoll aufraeumen — im Code nachgesehen) ein Passwort, das ein nur ueber SSO bereitgestelltes Konto nicht kennt. Keine Sicherheitswirkung. In deploy/DEPLOY.md 'Three things single sign-on does not do yet' beschrieben, Ausweg holzcloud user passwd. Ein Flick (Bestaetigung ueber eine frische Anmeldung beim Ausweisdienst) ist offen.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 20,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/login.go",
    "line": null,
    "description": "Code-Durchgang IN-06: wer bei eingeschaltetem SSO eine Passwortsitzung abmeldet, wird beim naechsten Klick ueber den Ausweisdienst wieder angemeldet, wenn der Browser dort noch eine Sitzung hat und das Konto verknuepft ist. In deploy/DEPLOY.md beschrieben.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 21,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "Eine von Hand im Nutzerformular entzogene Website kehrt bei der naechsten Anfrage der SSO-Sitzung zurueck, solange HOLZCLOUD_SSO_WEBSITE_GROUPS gesetzt ist; kein Bildschirm sagt das. In deploy/DEPLOY.md beschrieben.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 22,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "Rechtezeilen der SSO-Abgleichung tragen user_id NULL, wenn sie bei der Bereitstellung entstehen (noch keine Sitzung) und sind ueber den Benutzerfilter des Protokolls nicht auffindbar. Im Browserdurchgang gemessen: in einer laufenden Sitzung traegt die Zeile das Konto (A3/A4), NULL nur bei der Bereitstellung (A5) — der Befund ist enger als im Nachlauf formuliert.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 23,
    "kind": "deviation",
    "phase": "10",
    "file": "tools/i18n/main.go",
    "line": null,
    "description": "go run ./tools/i18n -schweiz entfernt einen verwaisten de-CH-Eintrag nicht, auch nicht beim zweiten Lauf. Nicht still: TestFassungKeysExistInTheSource wird rot. Einmal von Hand entfernt (e306eeb).",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 24,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/web/headers.go",
    "line": null,
    "description": "Verwaltungsantworten tragen 'Vary: Cookie' zweimal (im Browserdurchgang an POST /admin/logout gesehen: 'Cookie, Cookie, HX-Request'). Vorbestehend, harmlos, nicht aus Phase 10.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 25,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/i18n/locales/fr-CH.json",
    "line": null,
    "description": "T-10-52: fr-CH und it-CH werden von Hand gepflegt; eine formgerechte Handaenderung besteht Werkzeug, Tests und CI. Konstruktionsbedingt.",
    "status": "waived",
    "reason": "Konstruktionsbedingt: die Regionalkataloge fr-CH und it-CH sind Abweichungslisten, die bewusst von Hand gepflegt werden; ein maschineller Pruefer muesste die Sprache verstehen. Ein Schluessel ohne Gegenstueck im Quelltext wird weiterhin von TestFassungKeysExistInTheSource gefangen.",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 26,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "Eine verweigerte SSO-Identitaet schreibt bei JEDER Anfrage eine auth.login_fail-Zeile, ungebremst (im Browserdurchgang A6: vier Zeilen fuer zwei Laeufe). Ein Proxy, der dieselbe verweigerte Identitaet dauernd behauptet, laesst das Taetigkeitsprotokoll unbegrenzt wachsen. Die Anmeldebremse bewusst nicht zu fuettern (T-10-20, gehalten) schliesst die naheliegende Loesung aus.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 27,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "Kein Abbau: ein durch SSO angelegtes Konto ueberlebt die Identitaet, die es erzeugt hat. Nimmt der Ausweisdienst jemanden heraus, bleibt das Konto hier bestehen (erreichbar nur noch ueber ein gesetztes Passwort oder eine neue Verknuepfung).",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 28,
    "kind": "deviation",
    "phase": "10",
    "file": "internal/admin/forwardauth.go",
    "line": null,
    "description": "Protokolllücken bei SSO-Verweigerungen: die auth.login_fail-Zeile traegt den Grund (not_linked, no_website_group, …) nicht, nur das Serverlog; ein misslungenes RenewToken verweigert ohne Protokollzeile; die Abmeldezeile unterscheidet nicht, welcher Weg gegangen wurde.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T18:00:00.000Z",
    "resolved_at": null
  },
  {
    "id": 29,
    "kind": "deviation",
    "phase": "audit-v1.6",
    "file": "internal/field/field.go",
    "line": 1093,
    "description": "Meilenstein-Audit v1.6: field.CheckAll baut die Zeilenmeldung einer Gruppe mit fmt.Sprintf(\"%s, Zeile %d: %s\", def.Label, i+1, reason) an zwei Stellen (field.go:1093 Laengenpruefung verborgener Zeilen, :1113 Check pro Unterfeld). Der Rahmen 'Zeile %d' ist fuer tools/i18n unsichtbar, auch wenn reason selbst eines Tages katalogisiert ist. Dieselbe Klasse wie Eintrag 18, aber eine eigene Stelle, die 18 nicht nennt. Gefunden vom Integrationspruefer, im Code gelesen. Gehoert zu Phase 12 Kriterium 9 (v2.0).",
    "status": "fixed",
    "reason": "Geschlossen in Welle 12-04 von v2.0. Der Rahmen ist keine fmt.Sprintf mehr, sondern field.inRow — ein Reason, der einen Reason traegt; Reason.Text loest verschachtelt auf, in einer Sprache. Gehalten von TestTheRowFrameAroundAGroupReasonIsTranslatedToo, Gegenprobe gefahren: Rahmenschluessel geleert -> 'Oeffnungszeiten, Zeile 1: Von has to be a time' wird gemeldet, also genau die halb uebersetzte Zeile, die dieser Eintrag beschreibt.",
    "recorded_at": "2026-09-10T15:24:10.774Z",
    "resolved_at": "2026-09-11T00:00:00.000Z"
  },
  {
    "id": 30,
    "kind": "deviation",
    "phase": "12",
    "file": "internal/field/field.go",
    "line": null,
    "description": "field.SlugifyKey trug eine eigene Transliterationsliste mit vier Eintraegen (ae, oe, ue, ss) und liess jeden anderen Akzentbuchstaben ganz fallen: 'Titulo' mit Akut wurde 'ttulo', 'Etat' wurde 'tat', 'Cafe' mit Akut wurde 'caf'. page.Transliterate kennt den vollen Latin-1-Satz seit jeher, und field.go argumentiert das Prinzip in seinem eigenen KindTerm-Zweig: zwei Niederschriften einer Regel sind zwei Regeln, die auseinanderlaufen. Gefunden am 2026-09-11 beim Uebersetzen der CSV-Beispielkopfzeile und im selben Commit behoben. Nur NEUE Feldkennungen aendern sich; eine Kennung wird einmal gepraegt und steht dann. csvimport.settleHeaderMarks war auf genau diesen Fehler geeicht und faellt damit weg — die zweite Faltung war nie ueber Kopfzeilen, sie war ein Spiegel des Fehlers.",
    "status": "fixed",
    "reason": "Im selben Commit behoben, in dem er gefunden wurde. TestFoldHeaderAgreesOnEveryOtherAccent haelt die Eigenschaft weiter und traegt jetzt die richtigen Schluessel statt der verlustbehafteten.",
    "recorded_at": "2026-09-11T00:00:00.000Z",
    "resolved_at": "2026-09-11T00:00:00.000Z"
  },
  {
    "id": 31,
    "kind": "deviation",
    "phase": "12",
    "file": "internal/i18n/locales/en.json",
    "line": null,
    "description": "Der Katalog warf 'Schlagwort'/'Schlagwoerter' und 'Beschriftung' auf dasselbe englische Wort (Label/Labels), ebenso im Spanischen (Etiqueta) und Italienischen (Etichetta). GLOSSARY.md fuehrt das seit 2026-09-06 als vorbestehenden Uebersetzungsfehler, den das Umdrehen der Quellsprache aufdecken wuerde. Aufgedeckt hat ihn etwas anderes: die uebersetzte Kopfzeile der CSV-Beispieldatei traegt 'Labels', und csvimport.fixedSpellings kennt das Wort nicht — die heruntergeladene Datei waere beim Hochladen mit einer unzugeordneten Spalte zurueckgekommen. Behoben: Schlagwort/Schlagwoerter heissen Term/Terms, Termino/Terminos, Termine/Termini; Beschriftung behaelt Label. Franzoesisch hatte die Kollision nicht.",
    "status": "fixed",
    "reason": "Im selben Commit behoben. Gehalten von TestEveryTranslatedExampleHeadingIsRecognisedOnTheWayBackIn, Gegenprobe gefahren: 'Labels' wieder eingesetzt -> rot.",
    "recorded_at": "2026-09-11T00:00:00.000Z",
    "resolved_at": "2026-09-11T00:00:00.000Z"
  }
]
````
