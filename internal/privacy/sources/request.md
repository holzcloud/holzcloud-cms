# Sources of the "request about my data" page

This file is not embedded and never reaches a website. It records, for every
block of the request kind (`internal/privacy/request.go`, texts under
`internal/privacy/texts/<lang>/request/`), what it says and which source it
was written against. The research is
`.planning/phases/33.2-data-subject-rights-and-retention/33.2-RESEARCH.md`,
§B.1 (deadlines, identity, format), §B.2 and §B.3 (what an erased order keeps)
and §Sources. A translation of a block says the same as the German block; a
translator who changes what a block says updates this table.

The texts are templates, not legal advice, and every composition says so
(33.1 D-07). The sources were read for the research on 2026-11-14.

The rights and deadlines repeat what the privacy page's framework sections
already publish (`texts/<lang>/privacy/{eu,ch,uk,us}.md`), with the same
citations, so the two pages of one website cannot disagree (T-33.2-54).

## Blocks

| Block | Shown when | What it says | Source |
|---|---|---|---|
| glance | always | template notice (33.1 D-07), rm machine notice; what the page is for: ask what is held, have it corrected or erased | 33.2 D-01 |
| rights | always | access with a copy, correction, erasure where the data is no longer needed and need not be kept; the privacy page's address as a fill-in for the details and exceptions | GDPR Art. 15(1) and (3), 16, 17(1) and (3)(b); DSG Art. 25, 32; UK GDPR Art. 15–17; Cal. Civ. Code § 1798.100 ff. (right to know, delete, correct). Read via gdpr-info.eu (EUR-Lex returned only navigation) and justement.ch (fedlex needs JavaScript), per the research's §Sources |
| deadline-eu | Set has EU | without undue delay, in any event within one month of receipt; extension by two further months with notice and reasons within the first month; free unless manifestly unfounded or excessive; additional information where there are reasonable doubts about the identity | GDPR Art. 12(3), (5), (6), via gdpr-info.eu/art-12-gdpr. The wording of the extension follows the published EU privacy section and the UK one; the research marks the extension's wording [ASSUMED] and its existence [CITED] |
| deadline-ch | Set has CH | access free of charge, as a rule within 30 days; if not possible, the person is told within the 30 days when; reasonable measures to identify the person | DSG (SR 235.1) Art. 25(6), 25(7); DSV (SR 235.11) Art. 16(5), Art. 18, via justement.ch. No deadline is named for correction and erasure (DSG Art. 32 sets none), so the block speaks of the access |
| deadline-uk | Set has UK | as the EU, with Art. 12A cited as the published UK section does | UK GDPR Art. 12(3), (5), (6), Art. 12A. Data (Use and Access) Act 2025: the commencement dates of its provisions conflict between the secondary sources the research read (Kennedys, VinciWorks, Clifford Chance), so no date and no "stop the clock" rule is encoded |
| deadline-us | Set has US | acknowledgement within 10 business days; answer within 45 calendar days of receipt; up to 45 further calendar days with notice and explanation; verification by matching the request's details against what is stored | CCPA regs (Cal. Code Regs. tit. 11) § 7021(a), (b), read via law.cornell.edu/regulations/california/11-CCR-7021. Verification: §§ 7060–7062 [ASSUMED by the research, not read]; the method stated is the one the published US privacy section already states ("compared with the details stored, for example the e-mail address of an order") |
| form | always | the marker [[datenanfrage]] alone on its line; no cookie; what is stored (kind, address, optional name and note, page language), no IP address and no browser id; the network address held in memory for at most an hour to limit abuse, never written; the operator is notified by mail when mail is set up; another way to send a request as a fill-in | `internal/public/datarequest.go` (HandleDataRequest, dataRequestWidget, dataRequestNotify) and `internal/public/optout.go` (takeRequest: the shared network budget); 33.1 D-03, 33.2 D-01 |
| identity | always | the operator checks the request is the person's before handing out, correcting or erasing anything and may ask for something that shows it; the answer comes from the operator, never from this page; the confirmation is the same for every address | 33.2 D-01; research §B.1 consequence 4 (identity is the operator's, not built). The provisions behind it are cited in the deadline block of each framework (GDPR and UK GDPR Art. 12(6), DSV Art. 16(5), CCPA regs § 7060 ff.), so a website is not told about a law it does not follow |
| kept | Features.Shop | e-mail address, phone and the order's mails are erased; the invoice fields stay (name, company, address, items, amounts, payment state, return policy, the customer's note) until the legal retention period of the seller's country ends, counted from the end of the order's calendar year; not used for anything else meanwhile; the answer names the date | 33.2 D-06, D-07; research §B.2 (AO § 147(3), HGB § 257(4), UStG § 14b; OR Art. 958f, MWSTG Art. 70; HMRC VAT Notice 700/21; IRS record keeping), §B.3 Rules 2–5; GDPR Art. 17(3)(b); CCPA regs § 7022(f) (kept data used for no other purpose); AO § 146(4) |
| closing | always | the no-legal-advice line, last line of every composition | Phase 30 decisions, 33.1 D-07 |

## meta.json

| Key | German | English | Why |
|---|---|---|---|
| title | Anfrage zu meinen Daten | Request about my data | the words of the form's own catalogue key "Request about my data", which the Requests screen and the legal pages card use for this page |
| slug | `meine-daten` | `my-data` | short; the German one is the address plan 03's cookie matrix serves |
| also_slugs | `anfrage-zu-meinen-daten` | `request-about-my-data` | the title as an address, so a page made by hand under it is found and not offered twice |
| draft_title, draft_slug | the title with "neue Vorlage", the slug with "vorlage" | "new template", "template" | a compare draft offered next to an existing page (33.1 D-01) |

## Languages

German and English (plan 33.2-05); French and Italian (plan 33.2-06).
Spanish reads English for good (33.1 D-16). German addresses the visitor with
du, as the Do-Not-Sell page does; the Swiss spelling is applied by compose
(33.1 D-15).

### French and Italian

Both say what the German block says, sentence for sentence, and carry the same
numbers in the same order (`TestEveryLanguageStatesTheSameDeadlines` compares
the digits of every composition with the English one). They address the
visitor as the privacy page of their language does: French with vous, Italian
with Lei.

- **Swiss terms** are the ones the French and Italian versions of the Swiss
  acts on fedlex use: *loi fédérale sur la protection des données* (LPD) and
  *ordonnance sur la protection des données* (OPDo), *legge federale sulla
  protezione dei dati* (LPD) and *ordinanza sulla protezione dei dati* (OPDa),
  SR/RS 235.1 and 235.11; *responsable du traitement* / *titolare del
  trattamento*; *renseignements* / *informazioni* for the information given
  on access (LPD art. 25, al. 6 and 7 / cpv. 6 e 7), *mesures adéquates* /
  *misure adeguate* for the identification (OPDo/OPDa art. 16, al./cpv. 5).
  Provisions are cited as the French and Italian privacy sections cite them:
  "art. 25, al. 7, LPD" and "art. 25 cpv. 7 LPD". The ordinance is named in
  full at its first citation, as the English block does, because the privacy
  page never names it.
- **GDPR and UK GDPR terms** follow the French and Italian versions of
  Regulation (EU) 2016/679, as the published EU and UK sections do: *dans les
  meilleurs délais et en tout état de cause dans un délai d'un mois* / *senza
  ingiustificato ritardo e comunque entro un mese*; *manifestement infondée ou
  excessive* / *manifestamente infondata o eccessiva*; *doutes raisonnables* /
  *ragionevoli dubbi*. Citation form "art. 12, par. 3, RGPD" / "art. 12 par.
  3 RGPD", as in the privacy sections.
- **CCPA**: the regulations are *règlements CCPA* / *regolamento CCPA*, as
  the US privacy sections call them; the English right names stay in brackets,
  as in German. "Business days" and "calendar days" are *jours ouvrables* /
  *giorni lavorativi* and *jours civils* / *giorni di calendario*; there is no
  official French or Italian text of the regulations to take them from.
- **French typography** as in the French privacy page: typographic
  apostrophe, U+00A0 before a colon and inside the angle quotes, U+202F before
  a semicolon.
- **Slugs**: `mes-donnees` and `i-miei-dati`, with the titles as also_slugs
  (`demande-mes-donnees`, `richiesta-miei-dati`), built as the German and
  English ones are.

## Open assumptions a block relies on

| Assumption | Where | What the text does about it |
|---|---|---|
| CCPA regs §§ 7060–7062 allow verification by matching the request's details against stored ones | deadline-us | the US privacy section says the same; the page names the section range, not a sentence of it |
| The extension wording under GDPR Art. 12(3) | deadline-eu | worded as the published EU and UK sections of the privacy page are |
| DUAA 2025 changes to UK deadlines | deadline-uk | no date and no clock-stopping rule is stated |
| The erasure of an order behaves as D-07 says | kept | plan 33.2-08 builds it; the block describes that decision, not today's code |
