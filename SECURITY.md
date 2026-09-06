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

There is no payment processing in this program: the farm-shop plugin takes an
order and nothing else, and money changes hands outside it. So there is no
payment provider, no callback and no notification to be forged.

A way around any of these is exactly what I would like to hear about.
