# Sources of the "Do Not Sell or Share" page

This file is not embedded and never reaches a website. It records, for every
block of the DNS kind (`internal/privacy/dns.go`, texts under
`internal/privacy/texts/<lang>/dns/`), which items of the checklist in
`.planning/phases/33.1-legal-frameworks-per-website/33.1-RESEARCH.md` it covers
and which primary source it was written against. The checklist ids are those of
the research: D1–D7 for this page, U4, U5 and U8 of the US section. A
translation of a block covers the same items as the German block; a translator
who changes what a block says updates this table.

The texts are templates, not legal advice, and every composition says so
(D-07). The sources were read for the research on 2026-11-14; CPPA rule-making
moves, and the regulations cited are those effective 1 January 2026.

## Blocks

| Block | Shown when | Checklist items | Primary source |
|---|---|---|---|
| glance | always | template notice (D-07), rm machine notice; what the page is for and for whom; D1 why the title is the statutory English wording | Cal. Civ. Code § 1798.135(a)(1): https://cppa.ca.gov/regulations/pdf/ccpa_statute.pdf |
| nothing-sold | always | D2 and U4: nothing is sold and nothing is shared for cross-context behavioural advertising, in the CCPA's sense of the two words; no third-party content and no analytics (the shared core of the privacy page); D2 note: a business that neither sells nor shares need not offer the page, offered anyway (D-10) | CCPA regs § 7010(e): https://cppa.ca.gov/regulations/pdf/ccpa_statute_eff_20260101.pdf ; "sell" and "share" in Cal. Civ. Code § 1798.140; CLAUDE.md "Nothing loads at runtime" (enforced by `internal/web/headers.go` and `internal/tmplmgr/external.go`) |
| processors | always | D2 and U5: service providers under a written contract are neither a sale nor sharing; the hosting provider as a fill-in; the operator's e-mail provider as a fill-in, because the opt-out notification of plan 03 goes through it when mail is set up | CCPA regs § 7026(f)(1); `internal/public/optout.go` (operator mail) |
| payment | Shop and Payment | D2 and U5: Payrexx and what it receives, the same list as the processor row of the privacy page | `internal/public/payment.go` startPayment; the privacy page's `processor_payment` row |
| gpc | always | U8 and D3: what Global Privacy Control is, Sec-GPC: 1 treated as an opt-out for this browser, the status line of the widget shows whether it was sent; D7: no fee, no change of experience, no pop-up or interstitial; the signal is not logged, a submitted form records whether it was sent | CCPA regs § 7025 (in particular (c)(6) and (f)); GPC: https://w3c.github.io/gpc/ ; `internal/legal` GPC, `internal/dsr` (the `gpc` column) |
| form | always | D4: the form as an online method besides the signal, no account, no verifiable consumer request; D5: what is entered is used only for the request; the marker [[widerspruch]] alone on its line; no cookie, no IP address, no browser id (D-03, plan 03) | CCPA regs § 7025(c)(2), § 7026(a)(1), (c), (d); `internal/public/optout.go` |
| agents | always | D6: authorised agents with signed permission; D4: a request denied as fraudulent is explained to the requester; the contact e-mail address as a fill-in; the statute and the sections the page follows | CCPA regs § 7026(e), (j); Cal. Civ. Code § 1798.135 |
| closing | always | D-07 no-legal-advice line, last line of every composition | Phase 30 decisions |

## meta.json

| Key | Value | Why |
|---|---|---|
| title | "Do Not Sell or Share My Personal Information" in every language | D1, Cal. Civ. Code § 1798.135(a)(1); what a California visitor looks for |
| slug | `do-not-sell-or-share` in every language | slugs are unique per website and language (`UNIQUE(website_id, locale, slug)`), so one address serves every language; it is also the slug plan 03's cookie matrix serves |
| also_slugs | `do-not-sell`, `do-not-sell-my-personal-information` | the usual other spellings, for the launch checklist (D-13) |
| draft_title, draft_slug | the title with "new template" in the page's language; the slug with that word | a draft offered next to an existing page (D-01) |

## Open assumptions a block relies on

| Assumption | Where | What the text does about it |
|---|---|---|
| The section numbers of the CCPA regulations effective 1 January 2026 stay as cited | every block that cites one | the citations name the section, not a page or a line, and sources/dns.md names the version |
| Most operators are below the CCPA thresholds | nothing-sold | the page claims no duty: it says it need not be offered and is offered anyway |
| Payrexx acts as a service provider | payment | kept as on the privacy page (assumption A10 there) |
