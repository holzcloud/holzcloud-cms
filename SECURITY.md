# Reporting a vulnerability

Please do **not** report a vulnerability through a public issue.

Use GitHub's private channel instead:
[**Report a vulnerability**](https://github.com/holzcloud/holzcloud-cms/security/advisories/new).
Only the maintainer sees the report, and you can keep writing in it until the
matter is fixed.

If that is not possible for you, open an issue titled "Security — requesting
contact" and **without details**; we will then agree on another way.

## What happens next

This is a project maintained by a single person in their spare time. I aim for a
first reply within a week and for a fix as soon as I have understood what needs
doing. I cannot promise deadlines and I pay no bounties.

If you would like to set a deadline for disclosure: 90 days is customary and fine
by me. Just say so in the report.

You will be named in the fix if you would like to be — tell me under which name.

## What counts as a vulnerability

Anything that lets somebody do more than they are entitled to is of interest:

- Access to other websites in an installation that hosts several
- Content reachable without signing in that should not be — drafts, protected
  pages, enquiries, orders, uploaded files
- Executing code through an uploaded template or an uploaded file
- Bypassing the login, the second factor or the separation of roles
- Scripts that come to execution in the admin
- Manipulating prices or stock — an order that stores a price other than the one
  on the page at the time

Not of interest, even when a scanner flags it:

- Missing security headers on `/healthz` or `/readyz`
- User enumeration through the login form — there is none; every failed attempt
  answers the same. If you find otherwise, that is a vulnerability.
- Disclosure of the version in use. It is in the startup log on purpose.
- Reports consisting only of a tool's output, without anybody having checked
  whether something can actually be achieved with it

## What the project does by itself

So you can judge what has already been dealt with:

- Template archives are refused at upload if they contain JavaScript or
  references to foreign servers, and are rendered before being accepted
- The `Content-Security-Policy` forbids foreign sources on every response; in the
  admin it additionally forbids every script in the document
- Passwords with Argon2id; the second factor is compulsory for administrator
  accounts
- CSRF on all changing requests, including those over htmx
- Zip-slip on template upload is prevented
- Mail goes out over STARTTLS or implicit TLS unless the operator explicitly
  configures `HOLZCLOUD_SMTP_TLS=none`, and header values are stripped of line
  breaks

Payment is optional and goes through one provider, Payrexx, and only when
`HOLZCLOUD_PAYREXX_INSTANCE` and `HOLZCLOUD_PAYREXX_SECRET` are both set; without
them the shop offers invoice and prepayment, and the two payment routes answer
404. This is what the program does with it, and what it does not:

- The customer is redirected to Payrexx's own page and comes back. Card and
  account data never reach this server. The API key lives in the environment,
  not in the database or its backups.
- The return address (`/zahlung/zurueck/<token>`) carries the order's private
  token, not its number. The notification address (`/zahlung/payrexx`) is open
  to the internet and has no shared secret, so **nothing in a notification is
  believed**: the one thing read from it is the gateway id, and the provider is
  then asked directly over the API. A forged notification can cause one lookup.
- A verdict is written to an order only if it is still open or its earlier attempt
  failed, only if the gateway is the one recorded for that order, and for "paid"
  only if amount and currency match the order. A payment already settled or a
  refund entered by hand is not overwritten by a late or replayed message.
- What is not verified: the program does not check a signature on the
  notification itself (it asks the provider instead), and the API signature
  scheme is written against the published description and tested against a
  stand-in, not against the live service.

A way around any of these is exactly what I would like to hear about.
