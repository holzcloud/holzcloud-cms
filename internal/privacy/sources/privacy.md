# Sources of the privacy page

This file is not embedded and never reaches a website. It records, for every
block of the privacy kind (`internal/privacy/privacy_page.go`, texts under
`internal/privacy/texts/<lang>/privacy/`), which items of the legal checklists
in `.planning/phases/33.1-legal-frameworks-per-website/33.1-RESEARCH.md` it
covers and which primary source it was written against. The checklist ids are
those of the research: E1–E13 (EU), C1–C8 (Switzerland), K1–K5 (UK), U1–U12
(US). A translation of a block covers the same items as the German block; a
translator who changes what a block says updates this table.

The texts are templates, not legal advice, and every composition says so
(D-07). The sources were read for the research on 2026-11-14; the research
names how long they stay current (UK and CPPA rule-making move).

## Blocks

| Block | Shown when | Checklist items | Primary source |
|---|---|---|---|
| glance | always | template notice (D-07); "no third-party content, no analytics" of the shared core | Phase 30 decisions; CLAUDE.md "Nothing loads at runtime" |
| controller | always | E1 and C1 (identity and contact, as a fill-in); U9 contact for questions | GDPR Art. 13(1)(a), http://publications.europa.eu/resource/celex/32016R0679 ; DSG Art. 19 Abs. 2 lit. a, fedlex `eli/cc/2022/491/20230901` |
| log | always | shared core: server log, truncation to /24 and /48, retention from `web.LogRetentionDays`; E6 storage period for the log | `internal/web/logip.go`, `deploy/journald-holzcloud.conf` (tied by TestRetentionComesFromTheDropIn) |
| cookies | always | shared core: cookie table from the register, processors table with the hosting country as a fill-in (E4, E5, C3, C4, U5); why no consent is needed, framework-neutral | `internal/privacy/cookies.go` register; the provisions themselves stand in each framework section |
| orders | Shop | shared core: what the checkout stores; E10 (data needed for the contract); retention as a fill-in (E6) | GDPR Art. 13(2)(e); `internal/public/checkout.go` |
| payment | Shop and Payment | shared core: what goes to Payrexx (`internal/public/payment.go` startPayment); processing contract as a fill-in | GDPR Art. 28 (named in the EU section); A10 |
| protected | ProtectedPages | shared core: the unlock cookie holds no personal data | `internal/page` UnlockCookiePrefix, UnlockLifetime |
| forms | Forms | shared core: what a form stores, no IP and no browser id, anti-spam without a cookie; retention as a fill-in | `plugins/kontaktformular`, `plugins/bestellung` |
| staff | always | shared core: accounts of the people who work on the website (name, e-mail, Argon2id hash, second factor, sessions, activity log) | `internal/auth/password.go`, `internal/activity` |
| media | always | shared core: files served from the own server, EXIF/XMP/IPTC removed at upload | `internal/media/intake.go` Keep |
| eu | Set has EU | E1 representative (Art. 27) and E2 DPO as fill-ins; E3 bases per processing with the legitimate interest named (`{bases_eu}`); E4 recipients and Art. 28 contracts; E5 third-country transfer, hosting country as a fill-in, Switzerland adequacy; E6 storage by reference; E7 rights of Art. 15–18, 20; Art. 21 objection under its own heading (Art. 21(4)); E8 no consent-based processing; E9 complaint (Art. 77) with the competent authority as a fill-in; E10 in `basis_eu_orders`; E11 no automated decision (Art. 22); E12 one month (Art. 12(3)); E13 cookies under Art. 5(3) ePrivacy and § 25 Abs. 2 Nr. 2 TDDDG | GDPR Arts. 4, 6, 12, 13, 15–18, 20–22, 27, 28, 45, 77: http://publications.europa.eu/resource/celex/32016R0679 ; ePrivacy Art. 5(3): http://publications.europa.eu/resource/celex/02002L0058-20091219 ; § 25 TDDDG: https://www.gesetze-im-internet.de/ttdsg/__25.html |
| ch | Set has CH | C1 by reference to the controller block; C2 purposes with the Art. 31 justification, contract under Art. 31 Abs. 2 lit. a (`{bases_ch}`); C3 recipients; C4 disclosure abroad naming the state (Art. 19 Abs. 4) with Art. 16 Abs. 2 / Art. 17 as fill-ins; C5 Swiss representative (Art. 14) as a fill-in; C6 access free and as a rule within 30 days (Art. 25), handover and transfer (Art. 28), rectification (Art. 32 Abs. 1), prohibition, non-disclosure and deletion (Art. 32 Abs. 2), objection as processing against the expressed will (Art. 30 Abs. 2 lit. b); C7 report to the EDÖB (Art. 49), not a GDPR-style complaint; C8 cookies under FMG Art. 45c lit. b with how to refuse | DSG SR 235.1 Arts. 14, 16, 17, 19, 25, 28, 30, 31, 32, 49: fedlex filestore `eli/cc/2022/491/20230901` (de/fr/it/en); FMG Art. 45c: fedlex `eli/cc/1997/2187_2187_2187/20230901` |
| uk | Set has UK | E1–E11 as for the EU in UK wording, with these differences: K1 transfers under regulations of Art. 45A; K2 complaint to the controller first, electronic route as a fill-in, acknowledgement within 30 days (s. 164A DPA 2018); K3 complaint to the Information Commission, formerly the ICO (s. 165); K4 UK representative (Art. 27) as a fill-in; K5 cookies under PECR regulation 6 with Schedule A1 paragraph 4 | UK GDPR Art. 13: https://www.legislation.gov.uk/eur/2016/679/article/13 ; DPA 2018 s. 164A: https://www.legislation.gov.uk/ukpga/2018/12/section/164A ; Information Commission succession: https://www.gov.uk/government/news/information-commission-succeeds-the-ico-as-uks-data-protection-regulator ; PECR reg. 6: https://www.legislation.gov.uk/uksi/2003/2426/regulation/6 , Sch. A1: https://www.legislation.gov.uk/uksi/2003/2426/schedule/A1 |
| us | Set has US | applicability (three CCPA thresholds, $26,625,000 since 1 January 2025; CalOPPA without a threshold) as an operator note; U1 categories in the last 12 months (`{categories_us}`); U2 sources; U3 purposes; U4 nothing sold or shared in the preceding 12 months, no actual knowledge for under-16s, sensitive data; U5 service providers by reference to the processors table; U6 rights to know, delete, correct, opt out, non-discrimination; U7 e-mail as a fill-in plus the form on the Do Not Sell or Share page, verification without an account, authorised agents with signed permission, 45 days (§ 1798.130); U8 GPC as an opt-out of sale and sharing for that browser (§ 7025), nothing changes, the DNS page confirms it (D-10, D-12); the opt-out form's data, no IP and no browser id stored, the sender's network (IPv4 address or IPv6 /64) and its block (/24 or /48) held in memory for at most an hour to limit the form and never written (`internal/public/optout_limit.go`), retention as a fill-in, records at least 24 months (§ 7101, D-11, U12); U9 date of the last update and how changes are announced as fill-ins; U10 Do Not Track (§ 22575(b)(5)–(6)); other states and the appeal as a fill-in | CCPA statute: https://cppa.ca.gov/regulations/pdf/ccpa_statute.pdf ; CCPA regulations eff. 1/1/2026 (§§ 7011, 7025, 7026, 7101): https://cppa.ca.gov/regulations/pdf/ccpa_statute_eff_20260101.pdf ; thresholds: https://www.cppa.ca.gov/regulations/cpi_adjustment.html ; CalOPPA § 22575 (mirror): https://california.public.law/codes/ca_bus_and_prof_code_section_22575 ; GPC: https://w3c.github.io/gpc/ |
| closing | always | D-07 no-legal-advice line, last line of every composition | Phase 30 decisions |

## Rows of meta.json that carry law

| Key | Framework | Checklist items | Primary source |
|---|---|---|---|
| basis_eu_* | EU | E3, E10 (orders), legitimate interest named (Art. 13(1)(d)) | GDPR Art. 6(1)(b), (c), (f) |
| basis_ch_* | CH | C2, Art. 31 justification | DSG Art. 31 |
| basis_uk_* | UK | E3 in UK wording | UK GDPR Art. 6(1)(b), (f) |
| category_us_* | US | U1 statutory category names | Cal. Civ. Code § 1798.140; regs § 7011(e)(1)(A) |
| processor_hosting, processor_payment | all | E4, E5, C3, C4, U5: the hosting country is a fill-in; Payrexx sits in Switzerland | GDPR Art. 13(1)(e)–(f); DSG Art. 19 Abs. 4 |

## Open assumptions a block relies on

Named as in the research's assumptions log. Each one is worded so that a wrong
assumption costs a sentence, not the structure, and most stand behind a
fill-in.

| Assumption | Where | What the text does about it |
|---|---|---|
| A3 The Commission's adequacy decision for Switzerland (2000/518/EC) still stands | eu: transfers | names the adequacy decision for Switzerland without its number; the hosting country stays a fill-in |
| A4 The Federal Council lists the EU/EEA states as adequate (DSV Annex 1) | ch: disclosure abroad | does not name a list; asks for safeguards only when the state is not adequate |
| A6 The UK representative rule mirrors the EU's | uk: representative | representative as a fill-in "falls zutreffend" |
| (checked) The UK deadline: UK GDPR Art. 12(3) as amended on 5 February 2026 says "without undue delay and in any event before the end of the applicable time period", and Art. 12A (inserted by the Data (Use and Access) Act 2025, s. 76) makes that one month from the relevant time, extendable by two further months for complex or numerous requests, by a notice with reasons given within the first month. Read at https://www.legislation.gov.uk/eur/2016/679/article/12 and https://www.legislation.gov.uk/ukpga/2025/18/section/76 on 2026-11-14 | uk: rights | said as "spätestens innerhalb eines Monats", with the extension and its notice, citing Art. 12(3) and 12A, in all five languages (IN-07 of the 33.1 review; it read "in der Regel" before) |
| A7 ico.org.uk still serves the Information Commission | uk: complaints | prints no URL (templates carry no links) |
| A8 The list of states that require honouring opt-out signals, and appeal duties | us: other states | one generic paragraph; the appeal route is a fill-in |
| A10 Payrexx is a processor (Art. 28) rather than an independent controller | payment, cookies (processor row), eu | kept from Phase 30; the payment block also points to the provider's own privacy notice |
