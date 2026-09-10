# Security

- [The measures](#the-measures)
- [No runtime dependencies on third parties](#no-runtime-dependencies-on-third-parties)
- [Why a Holzcloud site needs no cookie banner](#why-a-holzcloud-site-needs-no-cookie-banner)
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
[configuration](configuration.md#e-mail).

## Why a Holzcloud site needs no cookie banner

This follows from the rule above and is worth stating plainly. The public side
sets **no cookies at all**: the session manager is only ever touched by admin
handlers, and the CSRF cookie is scoped to `Path("/admin")`. It also loads
nothing from a third party, so there is no embedded service that could set one on
your behalf and nothing to obtain consent for under § 25 TDDDG.

Consent is about storing or reading information on the visitor's device and about
passing data to third parties. A Holzcloud site does neither, so the banner has
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
  and for how long you keep it. Nothing else in Holzcloud needs an entry there.

Delete answered enquiries when you no longer need them; nothing expires them
automatically, because how long an enquiry stays relevant is a decision only the
operator can make.

**A password-protected page sets one cookie**, and only that page does. It
remembers that the visitor entered the password, is scoped to that page's path,
lasts twelve hours and contains a signed token rather than anything about the
person. It is strictly necessary for a service the visitor explicitly asked for —
seeing the page they just unlocked — and therefore consent-exempt under
§ 25(2) TDDDG. A site with no protected page still sets no cookie at all.

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

## Rights per person

Two roles remain, because two are right: **administrator** runs the installation,
**editor** runs the content. What was missing in practice is not more roles but
two limits *inside* the one — both under *Users → Edit*:

| Right | Meaning |
|---|---|
| **Websites** | Nothing ticked: all of them. Otherwise exactly these — everywhere, including through a hand-typed address. |
| **May publish** | Without this right, work is written and submitted for review; somebody else puts it online. |

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
