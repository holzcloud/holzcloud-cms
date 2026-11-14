# Security

- [The measures](#the-measures)
- [No runtime dependencies on third parties](#no-runtime-dependencies-on-third-parties)
- [Why a holzcloud-CMS site needs no cookie banner](#why-a-holzcloud-cms-site-needs-no-cookie-banner)
- [Two-factor authentication](#two-factor-authentication)
- [Signing in through an identity provider](#signing-in-through-an-identity-provider)
- [Rights per person](#rights-per-person)
- [The password again, before the irreversible](#the-password-again-before-the-irreversible)
- [Protected pages and preview links](#protected-pages-and-preview-links)
- [Access links and getting locked out](#access-links-and-getting-locked-out)
- [The brand of the admin](#the-brand-of-the-admin)

## The measures

- **Argon2id** password hashing, tunable to the server's performance, with a
  minimum of eight characters everywhere a password is set
- **CSRF** protection on all state-changing requests, through gorilla/csrf, with
  the token in `hx-headers` on `<body>` for htmx requests
- **Session rotation** on login, to prevent fixation
- **Session revalidation** — every admin request re-checks the user against the
  database, so deleting a user or changing a role takes effect immediately
- **Session revocation** — changing a password ends that user's other sessions
- **Login throttling** — 10 failures per client address and 100 per account
  within 15 minutes; the per-account limit is deliberately loose so it cannot be
  used to lock somebody out
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

Delete answered enquiries when you no longer need them; nothing expires them
automatically, because how long an enquiry stays relevant is a decision only the
operator can make.

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

Three tests hold the promise that opening a page sets nothing.
`TestNoPublicGetSetsACookie` drives all eight shipped themes through the real
router, over nineteen public addresses (home page, archive, search, 404, shop
lists, basket, feeds and more), and fails if a plain GET sets a cookie.
`TestNoPublicGetSetsACookieWithGPC` runs the same addresses again with
`Sec-GPC: 1`, and `TestTheOptOutPostSetsNoCookie` sends the opt-out form of a
US website (see [below](#opt-out-requests)) and checks that its answer sets
none either. A source test, `TestEveryCookieWriterIsAccountedFor`, fails when a
file that writes a cookie appears that the privacy text does not list, so a
fourth cookie cannot go out unmentioned.

### The request log

The program writes one line per request to the journal. The visitor's address in
it is cut to the first 24 bits (IPv4) or 48 bits (IPv6) before it is written; an
address that cannot be read is written as `unknown`. Three limits belong with
that:

- The rate limit holds the whole address, in memory only, and forgets it.
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
  rather than invented.

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
two limits per hour. One network — an IPv4 address, or an IPv6 /64 — may send
10 requests an hour; that budget is kept in memory only, so no address is
stored, and spending it closes the form to that network and nobody else
(`TestOneNetworkCannotLockOutTheOthers`). The website as a whole stores at
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
assistant connection cannot read them ([AI access](ai-access.md#tools)). In
this release they are kept until the website itself is deleted. Retention and
deletion arrive with the data-subject requests of the next phase, which must
keep a US opt-out for at least 24 months (CCPA regulations § 7101).

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

Then comes a screen asking for the password; afterwards you are confirmed for 15
minutes and land back where you were. The button is not silently completed for
you — you press it again, and this time it works. That saves a session full of
parked forms and is more honest.

Deliberately four buttons. A prompt that comes up everywhere is one nobody reads,
and then it protects nothing.

## Protected pages and preview links

Two small answers to two questions every site eventually asks.

A page can be put **behind a password** — the trade price list a joiner sends to
dealers and does not want in a search index. It is not a substitute for accounts:
everyone who gets the word gets in, and there is no record of who did. What it
does guarantee is that the page never appears in a listing, an archive, the
search, the feed or the sitemap, because the title and the excerpt are exactly
what the password was meant to hold back.

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
