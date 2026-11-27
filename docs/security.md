# Security

- [The measures](#the-measures)
- [No runtime dependencies on third parties](#no-runtime-dependencies-on-third-parties)
- [Why a holzcloud-CMS site needs no cookie banner](#why-a-holzcloud-cms-site-needs-no-cookie-banner)
- [Requests about personal data](#requests-about-personal-data)
- [Retention](#retention)
- [Record of processing activities](#record-of-processing-activities)
- [Two-factor authentication](#two-factor-authentication)
- [Signing in through an identity provider](#signing-in-through-an-identity-provider)
- [Rights per person](#rights-per-person)
- [The password again, before the irreversible](#the-password-again-before-the-irreversible)
- [Protected pages and preview links](#protected-pages-and-preview-links)
- [Access links and getting locked out](#access-links-and-getting-locked-out)
- [The brand of the admin](#the-brand-of-the-admin)

## The measures

- **Argon2id** password hashing, tunable to the server's performance, with a
  minimum of eight characters everywhere a password is set. Password checks run
  in a bounded pool: at most `HOLZCLOUD_ARGON2_CONCURRENCY` of them at once
  (default 2, each one takes `HOLZCLOUD_ARGON2_MEMORY`), and the start-up line
  *password checks bounded* names the slot count and the worst-case memory. A
  stranger's check (a sign-in, a page password) waits 1.5 seconds for a slot, a
  signed-in person's own (hashing a new password, confirming their own) 10
  seconds. A check that finds every slot taken is answered "busy, try again"
  (503 with `Retry-After`) and is **never counted as a wrong password**, so a
  flood cannot use the defence to lock the owner out: it only makes the server
  say so.
- **CSRF** protection on all state-changing requests, through gorilla/csrf, with
  the token in `hx-headers` on `<body>` for htmx requests
- **Session rotation** on login, to prevent fixation
- **Session revalidation** — every admin request re-checks the user against the
  database, so deleting a user or changing a role takes effect immediately
- **Session revocation** — changing a password ends that user's other sessions
- **Throttling of every password check**, all within 15 minutes and all in memory
  only (a restart clears them): the sign-in allows 10 failures per client
  address and 100 per account; the second sign-in step 10 per client and 5 per
  account, and a sign-in ends after 3 wrong codes, also when they are sent in
  parallel, so the password has to be typed again (that costs no attempt, so
  the 5 per account are the bound on guessing a code); the confirmation step ("please confirm") and the "current
  password" field of a password change 10 per client and 5 per user; the
  password form of a protected page 10 per visitor and page and 100 per page.
  An IPv6 address counts by its /64, not as 18 quintillion budgets. A check is
  reserved before it is made and given back when the server was too busy to make
  it, so parallel guesses cannot get past a limit and a busy answer costs
  nothing. Each limit remembers at most 10,000 addresses or accounts and never
  forgets one that is still counting to make room for another, so a flood of
  invented addresses cannot hand an account under attack a fresh budget; while
  all 10,000 are still counting, a new one has to wait (a restart clears it). The per-account limits are deliberately loose, or accepted as the
  price: whoever knows an account's password can spend its second-step budget
  and keep the owner out of the second step for up to 15 minutes. A sign-in code
  (TOTP) works once, even when several requests send it at the same moment
- **Nothing loads at runtime** — a `default-src 'self'` CSP on every response,
  stricter still on `/admin`; template archives are refused at upload if they
  reference an external stylesheet, script, font or image
- **Security headers** — `nosniff`, `Referrer-Policy`, `X-Frame-Options`,
  `Cross-Origin-Opener-Policy`
- **bluemonday** sanitisation on all Markdown output
- **Zip-slip prevention** and an enforced uncompressed-size budget on template upload
- **Magic-byte MIME validation** on media upload; an SVG must really be an SVG,
  and SVG and PDF are served with a sandboxing CSP
- **Path validation** on template assets — `http.ServeMux` cleans the *escaped*
  URL, so `%2e%2e` reaches handlers as `..` and has to be rejected explicitly
- **Draft isolation** — draft pages never leak into public queries
- **Public form attachments are bounded.** A plugin that has the `attach`
  permission (the contact form, once its attachment field is switched on) takes
  files from strangers, so the host bounds what a submission may cost before the
  plugin is called. The kind of a file is read from its first bytes; each file
  is then written to the spool directory `.upload-spool` in the data directory,
  up to the limit for its kind and no further, and is **not held in memory**
  while it arrives and is checked. A submission may carry at most 64 parts and
  256 KB of text, at most three files, and only one of them may use the video
  limit (the others are held to the image limit), so the whole body is at most
  about 74 MiB at the default limits (video + two images + 320 KB); a body that
  is larger, or that ends before its closing boundary, is refused whole (413 or
  400) and the plugin never sees it. A file the form keeps is read once more to
  remove its metadata, which costs up to about twice its kind's limit while it
  is kept: the worst case for four submissions at once is about 512 MB at the
  default 64 MB video limit, and four contact-form messages that each kept a
  60 MB video at the same moment measured 300 to 363 MB more resident memory
  (300, 321 and 363 MB in three runs; it depends on when the Go runtime reuses
  what the first keep freed, so it is a measurement and not a bound). A spam
  message that keeps nothing costs no memory to speak of: four at once with 60
  MB attachments grew the process by 1 MB, against 700 to 811 MB on 0.1.5.
  At most **four** submissions with files are taken in at once on the whole
  server, **two** at once and **thirty an hour** from one network (an IPv4
  address, an IPv6 /64); the next is told to come back (503 for the server, 429
  for a network, both with `Retry-After`) before a byte of its body is read. A
  request that arrives through a proxy that `HOLZCLOUD_TRUSTED_PROXIES` does not
  cover names no visitor, so it meets only the server's four places and no
  per-network bound. When the free space in the data directory is below
  `HOLZCLOUD_MIN_FREE_BYTES` plus room for four uploads (the floor plus four
  times the largest submission: about 809 MiB at the defaults), files are
  refused with a sentence and the message itself still arrives. The spool of a
  request is emptied when its answer is written, whatever happened, and the
  whole spool at every start and, for what is older than an hour, every hour.
  What is accepted: four slow senders can keep a form's attachments busy for up
  to two minutes (the server's read timeout), and the rest of the site goes on
  working; the levers are a proxy in front of the server that limits the time
  and the rate of an upload, and a restart.
- **An animated GIF is never decoded frame by frame.** The upload counts a GIF's
  frames by walking its blocks and skipping the picture data, so a GIF of
  hundreds of frames costs no more memory to upload than one of two (a 240-frame
  GIF of 2600 by 1300 pixels grew the process by 2 to 4 MB, against 720 to 743 MB
  on 0.1.5). An animated or unreadable GIF is stored as it is and is not offered for
  scaling.

## No runtime dependencies on third parties

Nothing is fetched from another server while the application runs — no CDN
scripts, no external stylesheets, no web fonts, no remote images, no analytics.
Every subresource comes from this server's own origin, which is what makes the
`default-src 'self'` CSP possible and keeps an instance fully self-contained and
offline-capable.

Only two things may be downloaded **at build time**:

1. **Go modules** — ordinary `go get` / `go mod tidy`
2. **Fonts** — fetched once, committed to the repository, embedded with
   `embed.FS` and served from `/assets/`. A font is never referenced by URL.

The single exception at runtime is SMTP, and only if you configure it: see
[configuration](configuration.md#e-mail). That includes the mail accounts a
website can have of its own; their passwords are stored encrypted under
`HOLZCLOUD_SECRET_KEY`, which never enters the database.

## Why a holzcloud-CMS site needs no cookie banner

This follows from the rule above and is worth stating plainly. The public side
of a website never sets a cookie because somebody opened a page: the session
manager is only ever touched by admin handlers, and the CSRF cookie is scoped to
`Path("/admin")`. A site without a shop or a protected page sets **no cookies at
all** on the public side. A site with either sets up to three cookies, each only
as the result of something the visitor does, and none of them needs consent. They
are listed below. The site also loads nothing from a third party, so there is no
embedded service that could set one on your behalf and nothing to obtain consent
for under § 25 TDDDG.

Consent is about storing or reading information on the visitor's device and about
passing data to third parties. A holzcloud-CMS site does neither, so the banner has
nothing to ask about. You still need an imprint (§ 5 DDG) and a privacy notice
(Art. 13 GDPR) — a new website is created with both as drafts, linked from the
footer menu, ready to fill in.

This stops being true the moment a feature is added that stores something in the
browser or talks to another server. Any such feature has to say so here.

**The contact form is the one feature that collects personal data**, so it gets
its entry. Placing `[[formular]]` on a page adds a form whose submissions are
stored in the site's own database and read in the admin under *Messages*.
`[[formular:Raw wool]]` places the same form with *Raw wool* already in the
subject line — for a page that sells or offers one thing.

What that does and does not change:

- It still sets **no cookie**. The spam protection is a honeypot field and a
  signed timestamp in the form itself, so nothing is stored on the visitor's
  device.
- It still talks to **no third party** — unless you configured SMTP, in which case
  a notification goes to the address you entered and nowhere else.
- It **does** store what the visitor typed — name, address, subject, message —
  under Art. 6(1)(b)/(f) GDPR. The sender's IP address and user agent are
  deliberately *not* stored.
- Your privacy notice therefore has to mention the form: what is collected, why,
  and for how long you keep it. Nothing else in holzcloud-CMS needs an entry there.

Answered enquiries are deleted on their own once the website's period for
form submissions has passed — twelve months unless you set another, under
Settings → Privacy → Retention ([below](#retention)). How long an enquiry stays
relevant is your decision; the period is where you write it down.

### The three public cookies

All three are `HttpOnly` and `SameSite=Lax`, and `Secure` on an installation
served over TLS. None is set by opening a page, and none is a tracking cookie.

| Cookie | Holds | Lifetime | Set when |
|--------|-------|----------|----------|
| `hc_warenkorb` | a random token that points to the basket in the site's own database | 30 days; deleted when the order is placed | the visitor puts an item in the basket |
| `hc_preise` | the choice between private and business prices | 30 days (it was a year until 0.1.1) | the visitor switches the view, on shops that show both |
| `hc_frei_N` | a signed token that unlocks the protected page with the number N, and nothing about the person | 12 hours, scoped to that page's path | the visitor enters the right password |

The basket cannot work without its cookie and a page cannot stay unlocked without
its own, so both are strictly necessary for what the visitor asked for and
consent-exempt under § 25(2) TDDDG. The price view is set only by the visitor's
own switch and remembers nothing but that choice; the privacy text lists it with
the other two.

None of the four legal frameworks a website can follow (see
[below](#legal-frameworks-the-privacy-page-and-the-imprint)) asks for a banner
for these three, because each one exempts storage that is strictly necessary
for something the visitor asked for:

| Framework | Exemption |
|---|---|
| European Union | ePrivacy Directive Art. 5(3), second sentence; in Germany § 25 Abs. 2 Nr. 2 TDDDG |
| Switzerland | FMG Art. 45c lit. b — allowed when the visitor is told about the processing, its purpose and that they can refuse. The Swiss section of the privacy page says how: don't do the thing that sets the cookie, delete it, or block cookies in the browser |
| United Kingdom | PECR Schedule A1 para 4, in particular (e)(ii), "maintaining a record of selections made on a website" |
| United States | no consent rule for such storage; what the law asks for is a truthful privacy policy (CalOPPA, Bus. & Prof. Code § 22575), including how the site answers Do-Not-Track — it tracks nobody, so the signal changes nothing |

This is the reading of the research for this release, written into templates;
it is not legal advice.

Four tests hold the promise that opening a page sets nothing.
`TestNoPublicGetSetsACookie` drives all eight shipped themes through the real
router, over two dozen public addresses (home page, archive, search, 404, shop
lists, basket, feeds, the Do Not Sell or Share page, the request page and
more; the list is `publicGetMatrix`), and fails if a plain GET sets a cookie.
`TestNoPublicGetSetsACookieWithGPC` runs the same addresses again with
`Sec-GPC: 1`, `TestTheOptOutPostSetsNoCookie` sends the opt-out form of a
US website (see [below](#opt-out-requests)) and checks that its answer sets
none either, and `TestTheDataRequestPostSetsNoCookie` does the same for the
request about one's own data ([below](#the-request-form)). A source test, `TestEveryCookieWriterIsAccountedFor`, fails when a
file that writes a cookie appears that the privacy text does not list, so a
fourth cookie cannot go out unmentioned.

### The request log

The program writes one line per request to the journal. The visitor's address in
it is cut to the first 24 bits (IPv4) or 48 bits (IPv6) before it is written; an
address that cannot be read is written as `unknown`. Three limits belong with
that:

- The rate limit holds the whole IPv4 address, or the /64 of an IPv6 address, in
  memory only, and forgets it.
- The journal itself is kept for 14 days by `deploy/journald-holzcloud.conf`. That
  file is a journald drop-in; it is system-wide, because journald has no
  per-unit retention, it is installed by hand (the command is in the file), and
  it was not confirmed on a real host at the time of writing. The check command
  in the file is the confirmation.
- Failed sign-ins and refused single sign-on attempts at the admin still write the
  whole address, under the same retention.

### Pictures

On upload the EXIF, XMP and IPTC data and the comments of a picture are removed.
Only the orientation is kept, so a photograph taken sideways still stands
upright. A test sends a photograph with a GPS position and camera data through
the real upload path and checks the stored file, and a source test keeps that
path the only one by which an upload is stored.

### Legal frameworks, the privacy page and the imprint

Every website says which laws it follows. Settings → General has four boxes:

| Box | Stands for |
|---|---|
| European Union | GDPR and the ePrivacy rules |
| Switzerland | the revised Federal Act on Data Protection (FADP/DSG), with UWG and FMG |
| United Kingdom | UK GDPR, the Data Protection Act 2018 and PECR |
| United States | the US state privacy laws: CCPA/CPRA, CalOPPA, and comparable laws of other states in one general paragraph |

A website starts with **European Union and Switzerland** ticked, because that is
what the privacy text was written for before the choice existed; the migration
that added the choice gave every existing website the same two
(`legal.Default`, migration `00063_legal_frameworks.sql`). The choice belongs to
one website and is never seen by another. Unticking all four is allowed — the
launch checklist then reports "Legal frameworks chosen" as open, and the
templates carry no section of any law. The same set is read and written over
the assistant connection ([AI access](ai-access.md#tools)) and travels with a
website bundle.

**The privacy page** is one page per website, not one per framework. A shared
core describes what this program does with data, derived from the functions the
website uses — a shop, the price view, online payment, protected pages, a
form — with a table of the cookies and one of the recipients. Below the
core comes one section per chosen framework, with what only that law asks for:
the legal bases and the right to object under the GDPR, the Swiss naming of the
country a disclosure goes to and the report to the FDPIC, the complaint to the
controller and to the Information Commission in the UK, and for the United
States the categories of information, the statement that nothing is sold or
shared, how Global Privacy Control is treated, and the Do-Not-Track answer
CalOPPA asks for (`privacy.ComposePrivacy`).

**The imprint** is composed the same way, for every website whatever it
follows: who runs the website and how to reach them, e-mail included, and then
the items the European (§ 5 DDG for providers in Germany, Art. 5 of Directive
2000/31/EC otherwise), Swiss (UWG Art. 3 para. 1 let. s) and UK rules
(Electronic Commerce (EC Directive) Regulations 2002 reg. 6, and for a
company the Trading Disclosures Regulations 2015, SI 2015/17, reg. 25) add when chosen. A website that follows only US law,
or none, gets the lean imprint: who and how to reach. No imprint links the EU
online dispute resolution platform, because the regulation behind it
(No 524/2013) was repealed by Regulation (EU) 2024/3228 with effect from
20 July 2025 (`privacy.ComposeImprint`).

**Languages.** The templates exist in German, French, Italian, Romansh and
English (`internal/privacy/texts/<language>/`), and a page gets the texts of its
own language (`privacy.TextLang`). A Spanish page gets the English template, and
the Legal pages card says so under the Spanish row. Romansh is never a
website's main language — the settings form does not offer it
(`SupportedLocales` in `internal/template`) — so a Romansh template only ever
arrives as a further-language draft next to an original in the main language.
It was written by a machine, says so in a second bold notice at its top, and
has to be checked by a Romansh speaker before it is published. A German page of
a website whose country is Switzerland, or a `de-CH` page, is written in Swiss
spelling, without ß (`legalRequest` in `internal/admin`).

**Templates, not legal advice.** Every composed page opens with the notice that
it is an unfilled template and closes with the sentence that it replaces no
legal advice; what only the operator knows — the responsible body, the
addresses, the hosting provider, the competent authority — is a visible fill-in
marker (`TestEveryCompositionIsAMarkedTemplate`). A page that still carries the
notice must not be published, and the Legal pages card marks it "Unfilled
template". Which provision each block of text was written against is recorded,
block by block, in `internal/privacy/sources/` (`privacy.md`, `imprint.md`,
`dns.md`, and `rm.md` for the Romansh vocabulary); those files are not
embedded and never reach a website.

**Nothing changes a page that exists.** A new website is created with the
imprint and the privacy page as drafts, in its main language, linked from the
footer menu. After that the **Legal pages** card (Settings → General) shows
one row per language of the website and one column per page its frameworks
call for — privacy page and imprint always, Do Not Sell or Share with the
United States — and creates what is missing as drafts in their language, filed
as translations of the main-language page. A cell with a page offers "New
draft to compare": the current template beside the page, never over it
(`HandleLegalDrafts`; `TestLegalPagesOnlyCreatePages` pins the page-store calls
to creating). Ticking a framework later therefore never rewrites a published
privacy page; the operator compares and copies.

### Global Privacy Control and the Do Not Sell or Share page

Global Privacy Control is a signal a browser can send with every request, as
the header `Sec-GPC: 1`; only exactly `1` counts (`legal.GPC`). On a website
with the United States among its frameworks the signal is treated as the
visitor's request not to have their personal information sold or shared, for
that browser (CCPA regulations § 7025). Since this program sells and shares
nothing, honouring it changes nothing but one line:

- A page that carries `[[widerspruch]]` (the
  [opt-out widget](content-model.md#the-opt-out-widget)) shows whether this
  browser sent the signal — "Opt-Out Request Honored" when it did, as § 7025(c)(6)
  asks. That page answers with `Vary: Sec-GPC`, so a shared cache does not
  hand one visitor's line to another (`optoutVary`,
  `TestTheWidgetPageVariesOnGPC`).
- Every other page is byte for byte the same with and without the signal, and
  does not name it in `Vary` (`TestGPCChangesNothingElse`). There is no
  pop-up, no interstitial and no fee (§ 7025(f)).
- The signal is neither logged nor stored. Only when a visitor sends the
  opt-out form is it recorded, with that request, whether the browser sent it.
- `/.well-known/gpc.json` answers `{"gpc": true}` on a US website, the
  resource the GPC specification defines for a site to say it honours the
  signal, and 404 on any other (`HandleGPCJSON`). `lastUpdate` is left out
  rather than invented. A shared cache may keep it for an hour, so the
  statement is gone at most an hour after the United States is unticked.

**The Do Not Sell or Share page** ("Do Not Sell or Share My Personal
Information", the statutory title, in English on every language's page) says
that nothing is sold or shared, names the recipients the website does involve
(the hosting provider, the mail provider, Payrexx with online payment),
explains the signal and carries the opt-out form (`privacy.ComposeDNS`). A
business that neither sells nor shares is not required to offer it (CCPA
regulations § 7010(e)), and the page says so; it is offered anyway, so a
visitor who looks for it finds it and an opt-out is recorded.

**Who the US laws apply to.** The CCPA binds a for-profit "business" doing
business in California that meets one of three thresholds: annual revenue over
$26,625,000 (the adjusted figure since 1 January 2025), buying, selling or
sharing the personal information of 100,000 or more consumers or households,
or half its revenue or more from selling or sharing it (Cal. Civ. Code
§ 1798.140(d)(1)). Most websites run on this program are below all three.
CalOPPA has no threshold: any commercial website that collects personally
identifiable information from California residents has to post a privacy
policy (Bus. & Prof. Code § 22575). The US section of the privacy page is
written to serve both.

### Opt-out requests

The opt-out form of a US website stores a request in the website's own
database, table `privacy_requests` (migration `00064_privacy_requests.sql`,
`dsr.Request`):

- what the visitor typed, as they typed it: a name, an e-mail address, a
  note — each optional, but a request must carry a name, an address or the
  signal. An address must be one bare address (`dsr.ValidEmail`): no display
  name, and none of `? & # % / , ;` or white space, which in a `mailto:` link
  would add a copy recipient or a prefilled text. The admin list links only
  an address that passes the same check and shows anything else as text;
- the language of the page it was sent from;
- whether the browser sent Global Privacy Control;
- when it arrived, and when the operator marked it handled.

It stores **no IP address and no user agent**, and the form sets no cookie
(`TestTheOptOutPostSetsNoCookie`). The spam protection needs none either: a
honeypot field, a signed time stamp in the form that has to be between two
seconds and two hours old and is valid only on the website that drew it, and
three limits per hour. One network — an IPv4 address, or an IPv6 /64 — may send
10 requests an hour, and the block around it — the IPv4 /24, the IPv6 /48 —
30, so that a home connection's /56 or a tunnel broker's free /48 is not
hundreds of budgets (`TestOneDelegationCannotCloseTheForm`). These budgets
are kept in memory only, for at most an hour and for at most 50 000 networks,
so no address is stored; spending one closes the form to that network and
nobody else (`TestOneNetworkCannotLockOutTheOthers`). A request the website's
limit refuses spends nothing (`TestARefusalByTheCeilingLeavesNoBudgetBehind`).
The website as a whole stores at
most 300 an hour, counted inside the insert so that neither a restart nor a
burst of parallel posts gets past it (`TestTheHourlyCapHoldsUnderABurst`).
The operator is mailed about the first 30 of an hour; any further request is
stored and listed, not mailed. Behind a reverse proxy the per-network budget
needs `HOLZCLOUD_TRUSTED_PROXIES`, as the login throttle does; a request whose
address is loopback or private gets no network budget and meets only the
website's limit. A cross-site POST is
refused before anything is read (`optoutSameOrigin`): `Sec-Fetch-Site` must
say `same-origin`, or `same-site` with an `Origin` of this very host; a
browser that sends no `Sec-Fetch-Site` is judged by its `Origin`, and without
one by its `Referer`, and either must name the host the form was posted to
(`TestAForeignOriginIsRefusedWithoutFetchMetadata`). A request caught by the
honeypot, or carrying a stamp this page did not sign, gets the same answer as
an accepted one, so a robot does not learn which trap caught it. A stamp this
page did sign but that is too old or too young belongs to a person — a tab
left open, a form the browser filled in — and gets "please send it again"
with a fresh form, never "recorded" for a request that was not stored
(`TestAGenuineButStaleStampAsksToSendAgain`).

When mail is set up for the website and it has a notification address, the
operator gets an e-mail in the website's main language with the request and
whether the signal was sent; the visitor's address stays in the body and never
becomes a Reply-To (`optoutNotify`).

The requests are listed under **Settings → Opt-outs** (German
"Widersprüche"), per website, newest first; the entry is in the settings
navigation of a website with the United States. "Mark as handled" stores the
date once (`TestMarkingAnOptOutHandledStoresTheDateOnce`). There is no delete:
neither the screen nor the store can remove a request
(`TestTheOptOutScreenCannotDelete`, `TestTheStoreCannotDelete`), and the
assistant connection cannot read them ([AI access](ai-access.md#tools)).
The [retention job](#retention) deletes them, and nothing else does: an
opt-out goes once the website's period for privacy requests has passed,
counted from the later of its arrival and the day it was marked handled, and
that period is never shorter than the 24 months CCPA regulations § 7101 ask
a request's record to be kept (`expireRequests`,
`TestRequestsAreKeptAtLeast24Months`). A data request nobody has answered is
never deleted: it is the evidence that an answer is owed. A request entered by
hand cannot be dated further back than those 24 months, so a year typed wrong
is refused at the field instead of being expired at the next run
(`TestARequestByPostIsEntered`, `TestARequestByMailIsEnteredByHand`). The job and the deletion lock below
count the same way, so neither lets go of a request the other still holds.

Deleting the website does not get round that. A website that received or
answered a request in the last 24 months cannot be deleted — not from the
settings screen and not by the assistant — because its requests would go with
it; the database refuses (trigger `websites_keep_privacy_requests`, migration
`00065`), and the screen says why and suggests deactivating the website
instead (`TestAWebsiteWithRecentPrivacyRequestsIsKept`). Requests older than
that go with their website. Moving a website by export and import takes its
requests along with the dates they arrived and were answered
(`TestPrivacyRequestsTravelWithTheWebsite`), so an export archive of such a
website carries names, addresses and notes and is to be kept like the
database. A request in an archive that arrived in the future, or was answered
in the future or before it arrived, is not restored and is named in the
import report (`TestAPrivacyRequestWithImpossibleDatesIsReported`): every
limit above counts from those dates, so one dated 2099 would close the form
and lock the website until then.

## Requests about personal data

A person may ask a website what it holds about them, for a copy of it, for a
correction or for its erasure. This program finds the data, hands it over,
corrects it and erases it. Whether a request really comes from the person,
which law applies and what the answer says are the operator's decisions;
nothing on these screens and nothing in this section is legal advice.

All of it lives under **Settings → Privacy** of one website (German
"Datenschutz", `/admin/websites/<number>/datenschutz`) and reaches that website
and no other. Everybody who may enter the website may search and download;
correcting and erasing need the right to publish, and a person without it is
refused with the read-only sentence (`TestAnEditorWithoutThePublishingRightCannotCorrect`,
`TestAnEditorWithoutThePublishingRightCannotErase`). None of it is reachable
over the assistant connection: a person's data, the requests and the opt-outs
stay in the admin, and the one privacy tool an AI key has is
`get_processing_record` ([below](#record-of-processing-activities);
`TestNoToolReturnsAPersonsData`, `TestThePrivacyScreenIsNotOverMCP`).

### Finding one person's data

The Privacy screen takes one e-mail address and searches everything the
website keeps under it (`dsr.Service.Find`):

- the shop's orders, and the mails sent about them;
- the contact messages from before the contact-form plugin (`form_messages`);
- the website's privacy requests and opt-outs;
- the mail queue, by recipient and by Reply-To — not the operator's own
  notification about a data request or an opt-out, which carries the
  visitor's address in its text only, never in a header; the request itself
  holds the same details and is listed and kept, and the notification leaves
  the queue a week after it was sent, or 90 days after it was queued when it
  never was;
- the staff accounts that work on this website — for an administrator only:
  the list of accounts is an administrator's screen, so an editor's search,
  export and printable answer leave them out
  (`TestOnlyAnAdministratorSeesAStaffAccount`);
- what the plugins keep, asked through their `privacy` hook
  ([plugins/README.md](../plugins/README.md#the-privacy-hook)): the contact
  form from version 1.6.0 — its messages, the quarantine, the answers of
  assembled forms and their attachments — and the farm shop from 1.1.0, its
  orders.

The address has to name somebody — one line with an @ in it — and is searched
as typed, so an address the shop or a plugin stored in a laxer shape (with a
name, a slash, two of them) is still found, handed over and erased
(`TestThePrivacyScreenRefusesWhatIsNotAnAddress`, `TestFindFindsAnAddressAsStored`);
the checkout itself now takes one bare address only
(`TestTheCheckoutTakesOneBareAddress`);
its letter case does not matter (`TestFindIgnoresLetterCase`), and the search
never reaches another website's records (`TestFindStaysOnItsWebsite`,
`TestThePrivacyScreenOfAnotherWebsiteShowsNothing`). An answer that leaves
something out says so. A plugin that keeps data for the website and cannot be
asked — it has no privacy hook, or it is not running — is listed under "Not
searched" with the number of its entries, and a store that failed to answer is
listed under "Could not be searched" (`TestPluginsWithDataAreReported`,
`TestAFailingSourceIsNamedAndTheWalkGoesOn`,
`TestThePrivacyScreenNamesWhatItCouldNotSearch`). A plugin's own error never
reads as "nothing found" (`TestAPrivacyErrorIsNotAnEmptyAnswer`), and a plugin
asked to find cannot write (`TestFindCannotWrite`).

Above the search the screen says the one thing it cannot do: check that the
request really comes from the person the address belongs to, before anything
is sent (GDPR Art. 12(6), Swiss DSV Art. 16(5), CCPA regulations § 7060 ff.).

### Handing it over: the JSON file and the printable answer

**Download as JSON** writes one file with every record found and, beside the
records, what GDPR Art. 15(1) and DSG Art. 25(2) ask an answer to carry: the
controller, the frameworks the website follows, and for each kind of record
found its purpose, legal basis, recipients, retention and source, and the
person's rights with where to complain. That part comes from the
[record of processing](#record-of-processing-activities), so the answer and
the record cannot disagree (`dsr.BuildExport`, `Export.Describe`,
`TestTheExportDescribesWhatItHolds`). **Printable answer** is the same as a
page to print or save as a PDF, with what an erasure kept and until when
(`HandlePrivacyAnswerPrint`, `TestThePrintableAnswer`).

Both speak the website's main language, not the operator's — they are for the
person; a Spanish website and one in a language without a catalogue answer in
English (`TestTheExportIsInTheWebsitesLanguage`). Neither carries a secret: no
password hash, no second-factor secret or recovery code, no order's
confirmation token (`TestTheExportCarriesNoSecret`). An order mail is handed
over as it was written, except for the link to the order's page: the order
confirmation ends with that link, and the part after `/bestellung/` is the
token, so it is cut out and the link reads `/bestellung/…`. Send them to the
person only, and treat them like the database. Both are answered with
`Cache-Control: no-store` (`TestThePrivacyExportIsADownload`), and each writes
one `privacy.export` entry in the activity log with the number of records and
never the address (`TestAnExportLeavesNoAddressInTheLog`).

### The request form

`[[datenanfrage]]` on a page of any website, whatever law it follows, places a
form ([content model](content-model.md#the-request-form)): what the visitor
asks for — access, correction or erasure —, the address the request is about,
which is required, and a name and a note, which are not. The Legal pages card
creates a page that carries it, "Request about my data", in every language of
the website ([multilingual](multilingual.md#legal-pages-in-several-languages)).

A request is stored in `privacy_requests`, beside the opt-outs: its kind, the
address, the name and note as typed, the page's language, when it arrived and
that it came by the form. Like the opt-out it stores **no IP address and no
user agent**, and sending it sets no cookie
(`TestTheDataRequestPostSetsNoCookie`). Whether the browser sent Global Privacy
Control is not stored with it: the signal says nothing about a request for
access, correction or erasure.

The form is guarded exactly as the [opt-out form](#opt-out-requests) is, by
the same code (`takeRequest`): the same-origin check, the bounded body, the
honeypot, the signed time stamp — drawn for this form, so a stamp of one form
is a forgery on the other —, the per-network budget, which the two forms
share, the website's hourly ceiling counted inside the insert, and the cap on
mails to the operator (`TestTheRequestFormRefusesLikeTheOptOut`). The opt-outs
count against their own ceiling, the three kinds of data request against one.

**The form never reveals anything.** Whatever address was typed, the visitor
reads the same sentence — recorded, the operator answers after checking that
it is theirs — and nothing about any person is ever on the page
(`TestARequestIsStored`); a form that said "we hold nothing for this address"
would tell anybody whether its owner is a customer. With mail set up and a
notification address, the operator gets an e-mail in the website's main
language naming the kind and the date by which it is due, and the visitor's
address never becomes a Reply-To (`TestTheOperatorIsToldAboutTheRequest`,
`TestTheOperatorMailNamesTheDueDate`).

### Requests and when they are due

The **Requests** tab lists every request for access, correction or erasure,
newest first: those from the form, and those that arrived by e-mail, letter,
telephone or otherwise, which the operator enters by hand with the day they
came (`TestARequestByMailIsEnteredByHand`). Each row links to the search for
its address.

Each request shows when it is due, by the frameworks the website follows
(`dsr.DueFor`):

| Framework | Answer due | Law |
|---|---|---|
| European Union, United Kingdom | one month, the same day of the next month | GDPR and UK GDPR Art. 12(3) |
| Switzerland | 30 days | DSG Art. 25(7), DSV Art. 18 |
| United States | 45 calendar days; an acknowledgement within 10 business days | CCPA regulations § 7021 |

With several frameworks the earliest date is shown, because answering by it
meets all of them; a request past it is marked overdue
(`TestDueDatesFollowTheFrameworks`, `TestTheRequestsScreenShowsWhenEachIsDue`).
The dates are a reminder and enforce nothing. Each law allows an extension when
the person is told in time, which is the operator's to decide, and public
holidays are not counted, so the acknowledgement date says "about".

**The answer is recorded once**: granted, partly or refused, with its basis,
which a refusal and a partial answer must give. A second answer is refused and
changes nothing (`TestTheAnswerIsRecordedOnce`, `Store.Answer`). Together with
the date, the kind and the manner it arrived in, that is the record CCPA
regulations § 7101 ask for — "the date of request, nature of request, manner in
which the request was made, the date of the business's response, the nature of
the response, and the basis for the denial". The record is kept at least 24
months from the later of arrival and answer ([retention](#retention)), is not
erased by the person's own erasure, because it is what keeps that erasure from
being undone by mistake (§ 7022(e)), and is never shared: not over the
assistant connection, and in a [bundle](import-export.md#what-travels-and-what-does-not)
only with the website it belongs to.

### Correcting

Every record found carries a "Correct…" disclosure with one small form per
field it allows, or the sentence why it allows none (`dsr.Service.Correct`,
`TestTheScreenOffersWhatEachKindAllows`). What may be corrected depends on what
the record is:

| Record | Corrected | How |
|---|---|---|
| Order: e-mail, phone | freely | the invoice does not print them (`TestAnOrdersEmailIsCorrected`) |
| Order: name, company, VAT number, street, postal code, city, country | yes, with the original kept | the old and the new value go into the activity log, because AO § 146(4) forbids changing a record "in a way that leaves its original content no longer ascertainable"; the notice says that an invoice already issued may need a corrected one (`TestAnInvoiceFieldIsCorrectedWithItsOriginalKept`, `TestAnInvoiceCorrectionKeepsTheOriginalInTheLog`); when the person is erased and the order reduced, both values go from those entries, which keep the field and the order (`TestErasureStripsTheCorrectedValuesOfTheOrder`); the same happens when the order's period ends and the retention run deletes it — every invoice correction older than the order cut-off loses both values (`TestExpiryStripsTheCorrectedValuesOfExpiredOrders`) |
| An erased order | no | the erasure is one-way (`TestAnErasedOrderCannotBeCorrected`) |
| Contact message from before the plugin | name, e-mail | what the person wrote stays what they wrote (`TestALegacyFormMessageIsCorrected`) |
| A plugin's record | the fields the plugin lists | through its privacy hook; a farm-shop order's address counts as an invoice field (`TestAPluginRecordIsCorrectedThroughTheHook`) |
| Staff account | on its own screen | an administrator gets the link to *Users → Edit* |
| Privacy request, mail | no | the record of a request and a copy of what was sent (`TestLogsAndAccountsAreNotCorrectedHere`) |

Only an invoice field's entry in the activity log carries values; any other
correction names the field and nothing else. After an e-mail correction the
screen comes back under the new address. The forms post the same request with
or without JavaScript.

### Erasing

**Erase…** on the Privacy screen opens a page, not a dialog: what an erasure of
this address would do on this website, in three groups — what is deleted, what
is reduced and what stays of it, and what is kept — with the reason and the law
for everything that stays, the plugins that could not be asked, and the
backups and snapshots that still hold the data
([below](#after-an-erasure-backups-and-snapshots)). The page changes nothing
(`TestPreviewChangesNothing`). It and the button on it, "Erase now", sit behind
the [password asked again](#the-password-again-before-the-irreversible) and the
right to publish (`TestTheErasurePageAsksForThePassword`). There is no
`hx-confirm`: a dialog that disappears without JavaScript is no question
before an act that cannot be undone, so the page is the question
(`TestTheErasurePageHasNoConfirmAttribute`). The erasure re-finds the records
itself on the website of the address; whatever the form posts names only the
person's address and the groups ticked (`TestErasureNeverCrossesWebsites`).

**What goes:** the contact messages from before the plugin; the person's mails,
the order mails and the queue; the contact form's messages, the quarantine
included, and their attachment files (`TestErasureDeletesFormMessagesAndMail`,
`TestErasureOfPluginRecords`). A plugin can only name files stored for it:
an attachment is recorded as the plugin's when it is kept, and a file of
another website, of the operator's own — the logo, an image on a page — or of
another plugin is refused and reported, never deleted on the plugin's word
(`TestAForeignMediaIDIsRefused`, `TestAPluginCannotDeleteTheOperatorsFile`,
`TestDeleteOwnedRefusesAFileThePluginDidNotStore`). A file that stays on the
disk is named on the result page (`TestAFileThatStaysIsNamed`).

**What an order keeps.** An order is the seller's copy of an invoice, which
bookkeeping and tax law make the seller keep (UStG § 14b in Germany, Art. 958f
CO in Switzerland), and AO § 146(4) forbids changing it so that the original
can no longer be read. GDPR Art. 17(3)(b) lets exactly what that obligation
covers stay, "to the extent that processing is necessary … for compliance with
a legal obligation", and CCPA regulations § 7022(f) ask the rest to go. So an
erased order keeps the **name, company, VAT number, address, items, amounts,
payment state, return policy and the customer's note** — what the invoice
prints — and loses its **e-mail address and phone number**; its mails are
deleted; its confirmation link's token is replaced by a fresh random one, so
the old link answers 404 from then on (`TestErasingAnOrderReducesIt`,
`TestTwoErasedOrdersKeepUniqueTokens`). The order is marked erased with the
date until which it is kept: 1 January after the period of the
[seller's country](#how-long-an-order-is-kept) has run from the end of the
order's year, never earlier than the website's own order period. That date is
stored with the order, so what the person was told stays true if the period is
changed later, and the retention job deletes the rest of the order on it and
not before. The kept fields are used for nothing else. The order screens show
the order as erased, with both dates and no address to write to, and changing
its status sends no mail (`TestAnErasedOrderSaysSoAndGetsNoMail`). A farm-shop
order is reduced the same way by the plugin itself — e-mail and phone go, name,
address, items, sum and note stay, with the same date
(`TestTheFarmShopReducesAnErasedOrder`).

**What is kept whole:** the person's privacy requests, as said
[above](#requests-and-when-they-are-due) (`TestRequestsAreKeptAndSaySo`).

**Staff accounts** are erased only by an administrator who ticks the box, never
the account of the person erasing, and never the last administrator
(`TestAccountsOnlyWhenChosenAndNeverYourOwn`,
`TestOnlyAnAdministratorMayEraseAnAccount`). The activity log keeps the staff
member's address, because what an account did has to stay attributable
(GDPR Art. 17(3)(e)); the log's own purge removes it.

When the erasure was opened from an erasure request, that request is answered
— "granted", or "partly" when something had to stay, with the basis — and a
request for access or correction is left for its own answer
(`TestTheRequestIsAnswered`). Whatever the erasure leaves is listed as kept
with its reason — a group that was not ticked, a staff account only an
administrator may erase, whether the box was offered or not — and makes the
answer "partly", never "granted" (`TestWhatTheErasureLeavesIsKeptAndPartly`). The erasure writes one `privacy.erase` entry in
the activity log with counts and never the address
(`TestErasureLogsNoAddress`). The result page is shown once; it names what was
reduced, until when and why, what failed, and the backups.

### After an erasure: backups and snapshots

An erasure changes the live database and the media directory. It does not
rewrite the copies, because a copy that an erasure could edit would no longer
be the way back that it exists to be. The erasure page and its result page
list what still holds the data (`TestTheResultNamesSnapshotsAndBackups`):

- **Pre-upgrade snapshots** (`pre-upgrade-<version>-<stamp>.sqlite` in the data
  directory), written before an update that migrates the database. The newest
  five are kept **with no age limit**, so erased data can survive in them for
  months or years. The pages list each by name and date.
- **Nightly backups** made by `deploy/backup.sh`, until they age out: after 30
  days, unless `RETENTION_DAYS` says otherwise. An off-site copy follows its own
  schedule.
- **Mail already sent**, which stays in the recipient's mailbox and with the
  mail provider. It is out of reach.

Keep backups and snapshots for restoring and for nothing else — the ICO calls
it "beyond use". And **after restoring a backup or snapshot that is older than
an erasure, repeat the erasure**: the restored database has the data back, and
its request records are just as old. The program keeps no list of erasures
outside the database, so it cannot repeat them for you: keep your own note of
each erasure — the day and the address — until every backup and snapshot older
than it is gone.

## Retention

What a website no longer needs is deleted on its own, after a period the
operator sets per website and kind of data, under **Settings → Privacy →
Retention** (`dsr.Service.Periods`, `HandlePrivacyRetention`):

| Kind | Default | Shortest | Set |
|---|---|---|---|
| Form submissions: the contact messages from before the plugin, and the contact form's messages with their attachment files | 12 months | 1 month | per website |
| Privacy requests and opt-outs, from the later of arrival and answer; a data request nobody has answered stays until it is | 24 months | 24 months (CCPA regulations § 7101) | per website |
| Orders, the shop's and the farm shop's | the period of the seller's country | the same | per website, in whole years |
| Staff accounts without a sign-in | 24 months | 1 month | for the whole installation, by an administrator |
| Failed mail, in both queues | 90 days after it was queued | — | fixed |
| The server's log | 14 days | — | read-only |

A period the website never chose follows its default, so a default changed in a
later release reaches it; a chosen one is stored and kept
(`TestPeriodsHaveDefaults`). Changing a period needs the right to publish and
the [password asked again](#the-password-again-before-the-irreversible),
because a shorter period deletes data at the next run without another question
(`TestChangingAPeriodAsksForThePasswordAndThePublishingRight`). A period below
its floor is refused at the field; a stored one that a later change of the
country left below the law's is applied as the law's.

### How long an order is kept

The period follows the **seller's country** — the country of the website's own
address under Settings → General — because bookkeeping and tax law bind the
business where it sits, whoever its customers are (`legal.OrderPeriodFor`):

| Seller's country | Period | Law |
|---|---|---|
| Switzerland | 10 years | Art. 958f CO; Art. 70(2) with Art. 42(6) VAT Act (MWSTG) |
| Germany | 8 years | § 147(3) AO, § 257(4) HGB, § 14b UStG, as amended for 2025 |
| United Kingdom | 6 years | HMRC VAT Notice 700/21 |
| United States | 3 years | the federal period of limitations the IRS names as the default |
| any other country | 10 years | none researched; the screen says so |

The period runs from the end of the calendar year of the order: an order may be
deleted from 1 January of its year plus the period plus one — an order of 2026
on a German website on 1 January 2035 (`legal.OrderDeletableFrom`,
`TestOrdersFollowTheSellersCountry`). That is exact in Germany; where the
business year is not the calendar year it can end up to a year early, and the
screen says to set a year more. A website may keep orders longer, never
shorter. What was not researched is not encoded: the periods of other EU member
states, US state sales-tax rules, and the German rule that the period does not
end while a tax assessment is open (§ 147(3) AO). Where one of them applies,
set the longer period yourself. These are the readings of the research for this
release, not legal advice.

### The job

The job "retention" ticks every hour and runs when a run is due: the last
successful run started at least 23 hours ago and the hour is a quiet one
(01:00–05:00 UTC), or it started 30 hours ago or longer whatever the hour, or a
run since then failed or never finished (`RetentionTick`,
`TestTheGateRunsOnceADay`). The gate is stored in the database, so a service
restarted more often than daily still runs once a day.

A run walks every website and kind and deletes in batches, keyed by the
website (`RunRetention`). It records itself in `retention_runs` — when it
started and finished, whether it succeeded, how much it deleted — and one row
per website and kind in `retention_deletions`, with the count and any error;
the installation's own kinds have no website. One failing kind does not stop
the others, but the run is then marked failed, the error goes to the journal,
and the Retention screen names the kind and the cause in a sentence
(`TestAFailedKindDoesNotStopTheOthers`, `TestTheRetentionScreenShowsAFailedKind`).
A run interrupted by a restart is closed as failed at the next one, and runs
older than 90 days are pruned (`TestAnInterruptedRunIsClosed`,
`TestOldRunsArePruned`). Each run that deleted something leaves one
`privacy.retention` entry in the activity log per website and one for the
installation, with counts only (`TestARunLeavesOneActivityEntryPerWebsite`);
neither the journal nor the stored error ever carries an address, a name or a
file name (`TestTheJobLogsNoPersonalData`).

**A failure is never silent.** When the job has not finished a run
successfully for more than 48 hours — counted from the later of its last
success and the moment the installation first checked for it
(`retention.since`), so a job that never ran is late too — every administrator's dashboard shows a warning that names
`journalctl -u holzcloud`, and every Retention screen shows it to whoever opens
it (`RetentionStatus`, `TestAnAdministratorIsWarnedAboutAStalledRetention`,
`TestTheRetentionScreenWarnsWhenOverdue`). The Retention screen also shows the
last run, what it deleted per kind, and the plugins that keep data and do not
answer the privacy hook: the job cannot delete what they keep, and the screen
says so (`TestTheRetentionScreenNamesPluginsTheJobCannotReach`).

**Plugins.** The contact form deletes its messages with their attachments and
the farm shop its orders when the job asks, through the privacy hook; the farm
shop never deletes an order before a date an erasure stored
(`TestPluginFormSubmissionsExpireWithTheirFiles`,
`TestTheFarmShopExpiresOnlyAfterTheRetentionDate`).

**Accounts** are installation-wide, so their period is one setting for the
whole installation, changed by an administrator with the password asked again
(`HandleRetentionAccounts`, `TestOnlyAnAdministratorWithAFreshPasswordSetsTheAccountPeriod`).
A staff account that has not signed in for longer — or was never used since it
was created — is deleted with its sessions ended. **An administrator is never
deleted automatically**; the screen counts the administrators who are past the
period instead (`TestInactiveAccountsAreDeletedButNeverAdministrators`).

**Failed mail** — a mail of either queue that was never sent — is deleted 90
days after it was queued (`TestFailedMailIsDeletedAfter90Days`).

**Logs** are not the program's to delete. The journal keeps them for 14 days by
the journald drop-in `deploy/journald-holzcloud.conf`, which is system-wide
([the request log](#the-request-log)); the Retention screen shows the period
read-only.

## Record of processing activities

GDPR Art. 30(1) and the Swiss DSG Art. 12(2) ask a controller to keep a record
of what it processes: the controller, the purposes, the categories of people
and of data, the recipients, transfers abroad, when data is erased and how it
is protected. **Settings → Privacy → Record of processing** generates it per website on
every visit, from what the website uses, the plugins that keep data for it,
the frameworks it follows and the periods it keeps — the same facts its privacy
page is composed from, so the record does not drift from the notice
(`dsr.BuildRecord`, `TestTheRecordFollowsWhatTheWebsiteUses`,
`TestTheRecordStatesThePeriodsItKeeps`). It is shown on screen in the
operator's language, as a page to print or save as a PDF, and as JSON
(`HandlePrivacyRecord`, `HandlePrivacyRecordPrint`, `HandlePrivacyRecordJSON`,
`TestTheRecordScreenAndItsExports`).

- **Activities.** Always: delivering the website (the server's log), requests
  and opt-outs, staff accounts and the activity log, e-mail sent by the
  website, backups. When the website uses them: contact forms, shop orders,
  orders through the farm shop, online payment, protected pages. Each with its
  purpose, the people and data concerned, the legal basis per framework
  (GDPR and UK GDPR Art. 6(1), FADP Art. 31), recipients, transfers and
  retention. A plugin the record cannot describe gets an activity of its own,
  to be filled in.
- **Security measures**, from what this repository enforces and this
  document describes: Argon2id, the second factor for administrators,
  sessions renewed at every sign-in, CSRF protection, the `default-src 'self'`
  policy and nothing loaded from other servers, the password asked again,
  backups with SHA-256 checksums, shortened addresses in the request log — and
  the TLS of the proxy in front, to be filled in.
- **Exemptions**, per framework: GDPR and UK GDPR Art. 30(5) exempt an
  organisation under 250 people unless the processing is not occasional, which
  an online shop's is read to be; DSV Art. 24 exempts one under 250 in
  Switzerland unless it processes sensitive data on a large scale or profiles
  at high risk; US law asks for no such record. The record is offered either
  way (`TestTheRecordNamesTheExemptions`).
- **Fill-ins.** What the program cannot know is marked "(to be filled in)" and
  never guessed: the hosting, mail and backup providers, the country the
  server stands in, a data protection officer, a representative, the TLS of
  the proxy in front. The one provider it names is Payrexx, because that is the
  one the program integrates (`TestTheRecordInventsNoProvider`).

The person's export and printable answer take their purposes, recipients and
retention from this record ([above](#handing-it-over-the-json-file-and-the-printable-answer)).
An AI key reads the record with `get_processing_record`
([AI access](ai-access.md#tools)); it is the only privacy tool, and it takes
no address.

## Two-factor authentication

Administrators **must** use a second factor; editors may. The reasoning is that an
administrator can change every password on the installation, upload a template and
reach every site, so a guessed password there costs everything.

It is ordinary TOTP (RFC 6238) — any authenticator app works, and the
implementation is checked against the RFC's published test vectors so it agrees
with the apps rather than merely with itself. The QR code is generated and drawn
as inline SVG; nothing is fetched at page load, and the shared secret is also
printed in blocks of four for anyone whose camera will not focus.

Making it compulsory would turn the phone into a single point of failure for the
whole installation, so there are two ways back:

1. **Recovery codes.** Ten single-use codes, shown once at setup and never again —
   only their hashes are stored. *My account* counts down how many are left and
   issues a fresh list on request.
2. **The command line.** `holzcloud user 2fa disable -email …` removes the second
   factor from an account. It needs shell access to the machine, which is exactly
   what an attacker with the password but not the phone does not have.
   `holzcloud user 2fa status` shows who has one and how many recovery codes they
   have left.

A code is refused once it has been used, even inside the thirty seconds it stays
arithmetically valid: that window is long enough for somebody who read the digits
over a shoulder to type them in afterwards.

There is one way this requirement is satisfied elsewhere: an installation wired
to an identity provider accepts that provider's session as the second step, so
what is asked for is then whatever the provider asks for. That is configured at
the provider and not here, it is stated on *My account* and on the user list of
any installation where it is switched on, and the section below says what it
costs.

## Signing in through an identity provider

An installation may be wired to an organisation's own sign-in — Authentik, behind
a reverse proxy that asks it who the visitor is and passes the answer on. It is
off unless somebody switches it on, and while it is off not one line of it runs.
It is worth describing here because it is **the first place in this program that
believes a claim about who somebody is**, rather than verifying one. Everything
else either checks a password, checks a secret it holds itself, or trusts nothing
at all. A forwarded identity header is an ordinary line of text, and anything
that can open a socket to this program can write one.

So the believing is made narrow, in four layers, each covering a different
failure of the one before it.

**The address the connection came from.** Read from the accepted socket, and
therefore not something a client can choose. It is checked *before* any identity
header is read, so a claim from an untrusted peer is never even fetched — and
that ordering is asserted against the source, because no answer the program gives
could tell the two orderings apart.

**Deleting the headers.** Every inbound header whose name looks like an identity
header is removed on every request, whoever the peer was, before any handler
runs. It is driven by a scan of the request's own header names rather than by a
list somebody has to keep in step with a program on another machine, and it folds
underscores to hyphens first, because those are two distinct names to Go and one
name to a careless reader. What this layer does **not** do is make a reverse proxy
that forwards a visitor's own copy harmless — which a defect in Caddy made the
default for a year. By the time such a request arrives it comes from the trusted
proxy with the right secret, and a forwarded value is indistinguishable from one
the identity provider sent. A header arriving with two values is refused, because
that is the shape an appending proxy leaves; against a single forwarded value the
proxy's version and its own delete lines are the only defence. This paragraph
said the opposite until the Phase 10 code review.

**A shared secret.** The proxy adds a header this installation names itself, and
this program compares it in constant time. It has nothing to do with the identity
provider; it exists so that a mistake in the first layer is not on its own
enough. It lives in the environment, never in the database, and it is stripped
from the request before any handler could log it.

**A signed assertion** would be the fourth, and it is deliberately **not** built.
The reason is written in the source rather than left to be rediscovered: the
three layers above are what a forward-auth arrangement can actually rest on, and
a signature that the same proxy could mint would add ceremony and no guarantee.

What it costs, plainly: the installation now depends on the operator's identity
provider and on the reverse proxy in front of it being configured correctly. What
it does not cost: the password login is untouched and stays the way back in when
the provider is down; group membership is re-read on every request a single
sign-on session makes, so a removal there takes effect here at that person's next
click rather than when the session expires — which is what this sentence used to
promise less exactly, and what the code did not do until the Phase 10 code review;
a session established this way ends when single sign-on is switched off or when
the provider vouches for somebody else on the same browser; and an account the provider vouches for
but this installation has never seen is refused unless somebody deliberately
switched account creation on — and then only into a website named in advance.
`deploy/DEPLOY.md` has the settings and the one command an operator can use to
check their own installation from the outside.

### The second way in: OpenID Connect

Since 2.9 an installation may instead — or as well — accept a sign-in through
OpenID Connect. This one believes a **signature**, not a header: the provider
signs an ID token, the browser posts it back, and this program checks it
against the provider's public key before it reads a single claim. Issuer,
audience, expiry and a nonce made for this one sign-in are all checked, `none`
and every algorithm nobody configured are refused, and a secret-signed token
(HS256) is only accepted where no public key is configured — the other way
round is the textbook confusion of using a public key as a secret.

It keeps the rule at the top of this document to the letter. The usual flow
trades a code for a token at the provider and fetches the provider's keys from
it, which is a request to another server while the program runs. This one asks
for the token directly and has the operator put the key on disk once; nothing
is fetched, ever. The price is manual: when the provider rotates its key, the
file has to be replaced.

The post back is the one form in the administration that is not under CSRF
protection, because it comes from the provider's page and cannot carry this
installation's token. What stands in its place is the state: a random value
made at the start of the sign-in and kept in a cookie only this origin can set
(`__Host-`), compared in constant time, and good for one answer. Everything
after the token — which account, which rights, whether a new account may be
created — goes through the same code as forward authentication, and writes the
same rows in the activity log, marked `via: oidc`. What it cannot do is re-read
the groups on every request: nothing arrives on later requests to re-read. A
change at the provider takes effect at the next sign-in, and a session lasts at
most a day. [`deploy/DEPLOY.md`](../deploy/DEPLOY.md#single-sign-on-openid-connect)
has the settings.

## Rights per person

Two roles remain, because two are right: **administrator** runs the installation,
**editor** runs the content. What was missing in practice is not more roles but
two limits *inside* the one — both under *Users → Edit*:

| Right | Meaning |
|---|---|
| **Websites** | Nothing ticked: all of them. Otherwise exactly these — everywhere, including through a hand-typed address. |
| **May publish** | Without this right, drafts are written and submitted for review; somebody else puts them online. Everything a visitor sees the moment it is stored opens read-only: published pages, menus, text snippets, albums, labels, redirects, files in use, the website's settings, theme and wording, published products, and plugin screens that change the website. A refused change says whom to ask — somebody who may publish — and does not park the change for later. |

What that covers in the corners, since version 0.1.5:

- **Products** follow the page rule. A draft product can be created, changed and
  deleted, and whatever such a person saves stays a draft. A published product —
  its text, price, stock and pictures — is read-only.
- **Media**: uploading stays open. A file nobody uses can still be described,
  cropped and deleted; a file something on the website shows — a page, an album,
  a text snippet, a product, the logo — cannot.
- **When the rights cannot be read**, the change is refused rather than allowed,
  and the error is logged. The same holds when the question "is this file in
  use?" fails: the file counts as in use. An administrator is not affected.
- **Plugin screens**: a plugin that declares `respects_read_only` in its manifest
  is told and refuses its own changes to the website, so orders, messages and
  the not-found log stay workable. Any other plugin's screen opens read-only, and
  the server refuses every change made on it by such a person.
- **Privacy** (Settings → Privacy): finding a person's data and downloading
  it stay open, so the person can be answered; correcting it, erasing it and
  changing a retention period are refused
  (`TestAnEditorWithoutThePublishingRightCannotCorrect`,
  `TestAnEditorWithoutThePublishingRightCannotErase`,
  `TestChangingAPeriodAsksForThePasswordAndThePublishingRight`). A staff
  account is erased only by an administrator.
- **AI keys are not limited by this right.** An AI key belongs to no person, and
  only an administrator can issue one, so there is nobody's publishing right to
  apply to it. The MCP tools answer exactly as they did before.

### How "including through a hand-typed address" is actually kept

That clause is the whole promise, and it is worth saying plainly how it is held
— and where it has failed, because it has.

Every address under the administration begins `/admin/websites/<number>/`, and a
middleware checks that number against the signed-in person's list before the
request reaches any handler. That part has always worked. The failures were all
the same shape and none of them was in that check:

**A screen that takes a second number.** A menu item, a product, an order's
outbound mail, a page's translation link, a snippet. The middleware reads the
website out of the *address*; the handler then reads a menu item — or a product,
or a message — out of the address or out of the submitted form, and nothing
joins the two. Type a website you may enter and a resource you may not, and the
guard is satisfied by the first half.

It happened six times between the 6th and the 8th of September 2026: the menu
items, the product form's save arm, an order's "send again" button, a page's
translation group, and — as hardening rather than a live defect — the snippets
and a product's picture list. Each is written up in the changelog with what an
operator should check.

**What changed as a result, and it is a rule rather than six patches.** The
website now belongs *in the database query*, not in a comparison the handler
remembers to make:

```sql
UPDATE products SET … WHERE id = ? AND website_id = ?
```

A query written that way cannot be got wrong by a later caller, because changing
the function's signature makes the compiler name every place that calls it. Where
a table genuinely cannot carry the website — `menu_items` and `page_terms` hang
off their parent and have no column of their own — the check stays in the handler
**and** carries a test of its own, because in that position nothing else holds it.

**What this means for you.** If you have never restricted an editor to particular
websites, none of it ever applied: an unrestricted account may enter every
website by design. If you have (`user_websites`, since version 1.3), the
changelog entry for each fix names what to look for — one of them left rows
behind that no program change undoes, and it carries the query to find them.

## The password again, before the irreversible

A session lasts a working day, because anything shorter leads to passwords on
scraps of paper. The price is the open laptop in a shared office. The answer to
that is not a shorter session but **a question in front of the few buttons that
destroy something**:

- deleting a website
- deleting a user
- issuing an AI key
- removing a plugin
- erasing a person's data ([above](#erasing)) — the erasure page itself, so
  the question comes before the list of what will go
- changing a website's retention periods, or the installation's period for
  staff accounts ([above](#retention)), because a shorter period deletes at
  the next run of the job

Then comes a screen asking for the password; afterwards you are confirmed for 15
minutes and land back where you were. The button is not silently completed for
you — you press it again, and this time it works. That saves a session full of
parked forms and is more honest.

Deliberately few buttons. A prompt that comes up everywhere is one nobody reads,
and then it protects nothing. `TestSensitiveRoutesAskForThePasswordAgain` holds
the whole list, which also covers accounts, the second factor, uploads and
imports.

## Protected pages and preview links

Two small answers to two questions every site eventually asks.

A page can be put **behind a password** — the trade price list a joiner sends to
dealers and does not want in a search index. It is not a substitute for accounts:
everyone who gets the word gets in, and there is no record of who did. What it
does guarantee is that the page never appears in a listing, an archive, the
search, the feed or the sitemap, because the title and the excerpt are exactly
what the password was meant to hold back.

A protected page works **in every language** of the website: the password form
posts the address with its language prefix (`fr/secret`), the cookie is scoped to
that address and the visitor returns to the page in the language they came from.
Before 0.1.6 a protected page in a further language could not be unlocked at all.

An unlock is **bound to the page's current password**. Changing the password,
also to the same words, ends every earlier unlock of that page; changing only the
hint does not. With the update to 0.1.6 every unlock made before it ended once,
and visitors type the password again. The form is throttled (10 attempts per
visitor and page and 100 per page within 15 minutes, counted by the /64 for IPv6);
a visitor who tries more often sees the form again with status 429 and a
`Retry-After` until the wait is over, and a server that is busy says 503 and does
not count the attempt. The body of the form is read up to 4 KiB. Behind a proxy
that is not listed in `HOLZCLOUD_TRUSTED_PROXIES` every visitor shares one
address: such a visitor has no budget of their own and only the page's 100 apply
(see [DEPLOY.md](../deploy/DEPLOY.md#locked-out-of-the-admin-interface)).

**And that no cache in front of this server keeps it.** The page a visitor gets
after entering the password is answered `Cache-Control: no-store, private`, so a
reverse proxy or a CDN may not hold it and hand it to the next person who asks
for that address. This was not true before 2.2: the form in front of the page
said `no-store` and the page behind it said `public, max-age=300`, because the
line that wrote its `Vary` header replaced the list instead of adding to it and
so deleted the cookie the access depends on. A shared cache could have served
the page for five minutes to anybody. The same line did the same to a shop that
offers both price modes, where trade prices could have reached a consumer; those
pages are now `private`.

A **preview link** shows an unpublished page to somebody without an account. The
alternative people otherwise reach for is publishing a draft "just for a minute"
so a customer can look at it, which is how a half-finished price list ends up in
a search index for a year. The link is an HMAC over the page id and an expiry, so
a signature cannot be moved to another page or stretched to a later date, and the
page it opens carries a visible banner saying it is a preview.

## Access links and getting locked out

An administrator does not have to invent a password and read it out: under *Users*
there are a **reset link** and an **invitation link**. The link is shown exactly
once, expires (72 h for invitations, 1 h for resets) and works once. Only its
checksum is stored — the same discipline as for the CSRF key. With SMTP
configured the link is additionally sent by mail; it is still shown on screen,
because mail can be delayed or filtered.

*Locked out?* If nobody can get in any more, the CLI on the machine is the way
back: `holzcloud user passwd`.

## The brand of the admin

Under *Brand* the admin takes on the name of the installation: the word in the
corner, the second half of every window title and the heading on the login
screen. Plus a mark (one or two letters for the square) and optionally a logo —
PNG, WebP or SVG, at most 512 KB.

An SVG is a document that can carry script, and it is served from this server.
It is therefore **checked and not defused**: anything containing `<script>`,
`onload=` or `javascript:` is refused rather than cleaned. And the check looks at
the bytes, not at the file name.

That concerns the admin only. How the websites themselves look is under *Design*.
