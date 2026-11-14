# Sources of the Romansh privacy page

This file is not embedded and never reaches a website. It records where the
Romansh (Rumantsch Grischun) words of `internal/privacy/texts/rm/privacy/`
come from. The blocks cover the same checklist items as the German blocks;
which items those are, and which primary source each block was written
against, stands in `sources/privacy.md`.

## What this text is, and what it is not

- **It was written by a machine.** No Romansh speaker has checked it. Every
  composition says so in bold, right after the template notice: the
  `machine_notice` of `texts/rm/lang.json` (D-07), asserted by
  `TestRomanshCarriesTheMachineNotice` and by the data-driven
  `TestEveryCompositionIsAMarkedTemplate`. The human review of the Romansh
  templates is deferred (33.1-CONTEXT.md, Deferred Ideas). Until it has
  happened, the notice is what makes publishing the page an informed choice.
- **No official Romansh text of the Swiss data protection act exists to quote.**
  The fedlex filestore served the DSG (SR 235.1) in de, fr, it and en, but the
  rm URL gave only the no-JavaScript shell (33.1-RESEARCH.md, assumption A11).
  The non-official bilingual corpus droit-bilingue.ch has no Romansh DSG
  either: its Romansh collection under chapter 23 holds only SR 231.1 (checked
  2026-11-14). The legal terms below are therefore borrowed from other Romansh
  federal texts and from the dictionary, not quoted from the act they describe.
- **Romansh is never a website's main language.** `tmpl.SupportedLocales` does
  not offer it (`TestRomanshIsNeverAMainLanguage`), so a Romansh privacy page
  only ever arrives as an extra-language draft beside an original in the main
  language, which the operator can compare it with.

## How the text is written

- Short, plain sentences. Where German packs two facts into one sentence, the
  Romansh text uses two.
- The visitor is addressed with the courtesy form **Vus** (Voss, Vossa, As),
  capitalised, as Romansh administrative texts do.
- **Statute names stay as abbreviations**: LPD, LTC, RGPD, UK GDPR, DPA 2018,
  PECR, CCPA/CPRA, CalOPPA, TDDDG. Statute titles that have a Romansh form in
  a source (LPD, LTC, RGPD) give it once in the section heading or the first
  sentence; English and Californian statute titles stay in English. The
  provisions themselves stay as cited in German (art., al., lit., section,
  regulation, Schedule, §).
- The English CCPA terms stay in brackets after the Romansh word
  (identifiers, commercial information, right to know, …), like the German
  text does, so a reader can find them in the statute.
- Where a term has no source-backed form, the text uses a plain description
  built from sourced words instead of a long coined term. Those combinations
  are listed below as "built".

## Sources

| Short | Source | Used for |
|---|---|---|
| PG | Pledari Grond, the Rumantsch Grischun dictionary of the Lia Rumantscha, https://www.pledarigrond.ch/rumantschgrischun/ (searched 2026-11-14) | most terms |
| LTrans | Lescha federala davart il princip da la transparenza da l’administraziun (LTrans, SR 152.3), Romansh text in the non-official corpus droit-bilingue.ch, arts. 9, 11–14, e.g. https://www.droit-bilingue.ch/rm-de/1/15/152.3-14-17.html | the authority's long form, "Lescha federala davart la protecziun da datas", "SR 235.1", "datas da persunas", "entaifer … dis", "en scrit", "prevesa", "refusar", "scadenza" |
| rm/lang.json | the notices written by plan 33.1-02 | "model", "cumplettar" |

## Terms

| Romansh, as used | German | Source |
|---|---|---|
| Decleraziun davart la protecziun da datas (title) | Datenschutzerklärung | PG: "decleraziun davart la protecziun da (las) datas" |
| protecziun da datas | Datenschutz | PG |
| nov model (draft title) | neue Vorlage | PG: Vorlage → "model"; rm/lang.json |
| datas persunalas | personenbezogene Daten, Personendaten | PG: "datas persunalas" (LTrans has "datas da persunas"; PG lists both) |
| Lescha federala davart la protecziun da datas (LPD), SR 235.1 | Datenschutzgesetz (DSG), SR 235.1 | LTrans art. 9 and its footnote; PG: "lescha davart la protecziun da (las) datas [LPD]" |
| Incumbensà federal per la protecziun da datas e per la transparenza (IFPDT) | Eidgenössischer Datenschutz- und Öffentlichkeitsbeauftragter (EDÖB) | LTrans arts. 13 and 14: "l’incumbensà federal per la protecziun da datas e per la transparenza"; PG: "Incumbensada federala per la protecziun da (las) datas e per la transparenza", abbreviation IFPDT. The masculine form follows the statute, like the German one. This settles assumption A9 of 33.1-RESEARCH.md for the long form and the abbreviation. Both sources are non-official. |
| lescha da telecommunicaziun (LTC) | Fernmeldegesetz (FMG) | PG |
| Reglament general davart la protecziun da datas (RGPD) | Datenschutz-Grundverordnung (DSGVO) | PG |
| directiva | Richtlinie | PG |
| Uniun europeica (UE); Reginavel Unì; Stadis Unids; California; Svizra; Germania | the states and unions named | PG (Germania and Svizra: common names, not looked up) |
| stadi commember; stadi terz; stadi federal | Mitgliedstaat; Drittstaat; Bundesstaat | PG |
| Cussegl federal | Bundesrat | PG |
| responsabel | Verantwortlicher | PG |
| elavuraziun; elavurar | Bearbeitung; bearbeiten | PG |
| elavuratur d’incumbensa; contract d’elavuraziun d’incumbensa | Auftragsverarbeiter; Vertrag über die Auftragsverarbeitung | built: PG "elavuratur" (Bearbeiter) and "incumbensa" (Auftrag, also legal); PG has no entry for the compound |
| incumbensà per la protecziun da datas | Datenschutzbeauftragter | PG: the entry for the former federal commissioner, "incumbensà federal per la protecziun da (las) datas", without "federal" |
| represchentant; represchentanza; filiala | Vertreter; Vertretung; Niederlassung | PG |
| retschavider | Empfänger | PG |
| basa legala | Rechtsgrundlage | PG |
| interess legitim | berechtigtes Interesse | PG |
| interess privat predominant | überwiegendes privates Interesse | PG: überwiegend → "predominant" |
| motiv da giustificaziun; giustifitgà | Rechtfertigungsgrund; gerechtfertigt | PG |
| consentiment | Einwilligung | PG |
| opposiziun; far opposiziun | Widerspruch; widersprechen | PG |
| dretg d’infurmaziun | Recht auf Auskunft | PG |
| dretg da correctura | Recht auf Berichtigung | PG |
| dretg d’annullaziun da las datas | Recht auf Löschung | PG |
| dretg da restrenscher l’elavuraziun | Recht auf Einschränkung der Verarbeitung | PG |
| dretg a la portabladad da las datas | Recht auf Datenübertragbarkeit | PG |
| reclamaziun | Beschwerde | PG |
| autoritad da surveglianza | Aufsichtsbehörde | PG |
| denunzia; d’uffizi; investigaziun | Anzeige; von Amtes wegen; Untersuchung | PG |
| violaziun | Verletzung | PG |
| communicaziun da datas; a l’exteriur | Bekanntgabe der Daten; im Ausland | PG |
| garanzia; excepziun | Garantie; Ausnahme | PG |
| decisiun d’adequatezza | Angemessenheitsbeschluss | built: PG "adequatezza" (Angemessenheit); no entry for the compound |
| clausulas da contract standard | Standardvertragsklauseln | built, no source found |
| profiling | Profiling | PG ("profiling [angl.]") |
| conservar; termin | aufbewahren; Frist | PG |
| stizzar | löschen | PG |
| memorisar | speichern | PG |
| protocol; datoteca | Protokoll; Datei | PG ("datotecas da protocol" for server log files) |
| server; navigatur; adressa IP; rait; banca da datas | Server; Browser; IP-Adresse; Netz; Datenbank | PG |
| scursanir | kürzen | PG |
| website | Website | PG ("website [info.] f") |
| tabella; chapitel | Tabelle; Abschnitt | PG |
| formular; pled-clav; endatar; debloccar | Formular; Passwort; eingeben; freischalten | PG |
| object; messadi | Betreff; Nachricht | PG |
| chanaster da products; empustaziun; empustar; butia online; cassa | Warenkorb; Bestellung; bestellen; Onlineshop; Kasse | PG |
| pajament; import; valuta; intent d’utilisaziun | Zahlung; Betrag; Währung; Verwendungszweck | PG |
| numer postal d’aviament; numer da telefon; adressa dad e-mail; numer da la taglia sin la plivalur | Postleitzahl; Telefonnummer; E-Mail-Adresse; Mehrwertsteuernummer | PG |
| remartga; furniziun; client | Anmerkung; Lieferung; Kunde | PG |
| mussament | Beleg | PG |
| chascha dad e-mail | E-Mail-Postfach | PG |
| purschider; purschider da servetschs; gestiunari | Anbieter; Dienstleister; Betreiber | PG |
| conto | Konto | PG |
| sectur da l’administraziun | Verwaltungsbereich | PG |
| sesida; connexiun | Sitzung; Anmeldung (info.) | PG; "sesidas da connexiun" for login sessions is built from the two |
| maletg; chargiar si | Bild; hochladen | PG |
| manaschi; segirezza | Betrieb; Sicherheit | PG |
| visitader, visitadra | Besucher, Besucherin | PG |
| consument; chasada; svieuta | Verbraucher; Haushalt; Umsatz | PG |
| vender; parter (partì, partida) | verkaufen; teilen | PG ("parter", also for sharing in social media) |
| partiziun | das Teilen (CCPA "share") | built from "parter"; the English "share" stands beside it |
| dischavantatg | Nachteil | PG |
| procura | Vollmacht | PG |
| minoren | Minderjährige(r) | PG (not used; the text says "persunas sut 16 onns") |
| domicil | Wohnsitz | PG |
| modificaziun, midada; actualisaziun | Änderung; Aktualisierung | PG |
| acziun; funcziun; configurar | Handlung; Funktion; einstellen | PG (configurar: PG has the noun configuraziun for Einstellung) |
| lingia d’entestaziun (header) | Kopfzeile (of an HTTP request) | built, no source found; PG "lingia dal chau-pagina" is the header of a printed page. The English "header" stands beside it |
| marca da temp | Zeitstempel | built, no source found |
| {n} di / dis, ura / uras, minuta / minutas | Tag(e), Stunde(n), Minute(n) | LTrans ("entaifer 20 dis"); common words |

## Terms that rest on no source

The following are built or chosen without a source and are the first things a
Romansh reviewer should look at:

- elavuratur d’incumbensa (and contract d’elavuraziun d’incumbensa)
- decisiun d’adequatezza
- clausulas da contract standard
- sesidas da connexiun
- partiziun
- lingia d’entestaziun
- marca da temp
- memoria da lavur (Arbeitsspeicher) and bloc d’adressas (Adressblock), in the
  sentence about the opt-out form's network budget (us)
- the slug `protecziun-da-datas` and the draft slug `protecziun-da-datas-model`
  are built from the sourced title; `also_slugs` is empty because no other
  usual Romansh spelling was found

## The request page (`texts/rm/request/`)

The "request about my data" page of phase 33.2 (plan 06) is written by the
same method and carries the same machine notice
(`TestRomanshRequestPageIsMarkedAsMachineWritten`); its human review is
deferred with the rest. What each block says and which source it rests on
stands in `sources/request.md`; it says sentence for sentence what the German
block says and carries the same numbers in the same order
(`TestEveryLanguageStatesTheSameDeadlines`). It addresses the visitor with
Vus, as the privacy page does. Where the privacy page already has a word for
something (responsabel, elavurar, stizzar, rectifitgar, dretg d’infurmaziun,
dretg da correctura, dretg d’annullaziun, retschavida, termin, prolungar,
memoria da lavur, adressa da rait, gestiunari, empustaziun, remartga,
prescripziuns CCPA), the request page uses that word. The Swiss ordinance is
named in words and not by an abbreviation, because no Romansh abbreviation
for it was found and an invented one would look like a quotation; it is not
cited by its SR number either, so that the page carries the same numbers as
the other languages.

Words the request page adds that rest on no source checked for this plan
(the Pledari Grond was not searched again), the first things a Romansh
reviewer should look at:

- ordinaziun davart la protecziun da datas (Datenschutzverordnung, DSV) and
  "la medema ordinaziun" for its second citation
- mesiras adequatas (angemessene Massnahmen)
- nun ch’ina dumonda saja evidentamain nunfundada u exagerada (es sei denn,
  ein Antrag ist offenkundig unbegründet oder exzessiv)
- dubis fundads (begründete Zweifel)
- dis da lavur (Geschäftstage) and dis chalendars (Kalendertage); onn
  chalendar (Kalenderjahr)
- motivaziun (Begründung)
- copia (Kopie); en detagl (im Einzelnen)
- dar ora (herausgeben)
- quint (Rechnung); venditur (Verkäufer); cundiziuns da return
  (Rückgabebedingungen); durada legala da conservaziun (gesetzliche
  Aufbewahrungsfrist)
- vegnir a savair (erfahren); independentamain da (unabhängig von)
- the title "Dumonda davart mias datas", the slug `mias-datas`, the also_slug
  `dumonda-mias-datas` and the draft slug `mias-datas-model`, built from the
  title as the privacy page's slugs are

## Open assumptions

| Assumption | Where | What the text does about it |
|---|---|---|
| A9 Romansh terms (IFPDT form etc.) | ch | the long form and IFPDT are taken from LTrans and the Pledari Grond, both non-official; the masculine form follows the statute |
| A11 No Romansh DSG on fedlex | all | no term is quoted from the DSG; the abbreviation LPD and the SR number carry the reference |
| The text is unchecked | all | the bold machine notice (D-07); human review deferred |
