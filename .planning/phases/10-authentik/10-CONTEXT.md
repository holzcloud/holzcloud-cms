# Phase 10: Authentik Forward-Auth - Context

**Gathered:** 2026-09-07
**Status:** Ready for planning

> Written in **English**: the developer decided on 2026-09-06 that this
> open-source project's code and its new planning artifacts are English.
> `.planning/GLOSSARY.md` is binding, including its rule that a German word
> *stored* in the database is a value and not an identifier.
>
> **The developer asked for the milestone to be carried out autonomously and
> explicitly not to be waited for.** This phase's research flag says every
> remaining question is a *policy decision, not a lookup*. Three of them are
> settled below with a recommended default and the reasoning, so the phase is
> buildable without an answer. **Two facts remain genuinely the operator's and
> cannot be looked up from here** — they are isolated in their own section, they
> are deployment-time verifications rather than planning blockers, and the build
> is correct under the defaults either way.

<domain>
## Phase Boundary

Whoever the operator's Authentik has already signed in reaches the admin without
a second sign-in — and every route by which that trust could be forged is closed
before the first header is read, with the password path untouched behind it.

Requirements: SSO-01 … SSO-11, plus the milestone close-out gate QUAL-01/QUAL-02.

**`ROADMAP.md`'s planning notes for this phase are unusually complete** — the
four-layer trust boundary, the authentik header contract verified from source at
two release tags, the Caddy CVE, the `form-action` trap, the eight-step build
order. **They are not restated here.** This file records only what they leave
open.

</domain>

<decisions>
## The three questions the research flag named

### D-01: an SSO identity with no matching website group

**Settled: auto-provisioning is off by default, and with it on the service
refuses to start until it is told which website a new account belongs to.**

This is the phase's single most likely way to ship a real vulnerability, and it
is dangerous precisely because every part looks correct alone.
`internal/admin/handler.go:173` reads

```go
return assigned == 0 || mine > 0
```

„no assignment means every website" — correct for accounts an operator creates by
hand, and it **inverts** under auto-provisioning: a freshly provisioned SSO
account has zero `user_websites` rows *by construction*, so the first stranger
who authenticates at the identity provider gets editor access to **every website
in the installation**.

`handler.go:173` must **not** change — that would lock out every existing editor.
The fix is at the other end: provisioning off unless switched on, and when on, a
required default website. The project already refuses to start on a
half-configured Payrexx pair; copy that shape, and make the failure loud at
start-up rather than quiet at the first sign-in.

### D-02: what `X-authentik-uid` looks like in the operator's instance

**Settled: pin the identity to `X-authentik-username`, not `-uid`.**

`-uid` is the OIDC `sub`, and its format depends on the provider's Subject mode —
the default is a hashed identifier. The roadmap records that at MEDIUM confidence
and says to pin to username if unsure. **Unsure is the honest state**, so
username it is, and `DEPLOY.md` states that the subject mode must not change
after users are mapped.

A companion trap that must be in the plan: **e-mail is not a stable identity.**
SQLite's `COLLATE NOCASE` folds ASCII only, so `Müller@…` and `müller@…` are two
rows — and therefore two admins.

### D-03: which Caddy the operator runs

**Settled: `DEPLOY.md` states a minimum of 2.11.2 and the example Caddyfile
deletes every copied header explicitly.**

This is not a lookup either — it is a floor the documentation sets.
CVE-2026-30851 (GHSA-7r4p-vjf4-gxv4) makes `forward_auth … { copy_headers X-Foo }`
generate a *conditional* set with **no delete** for the client's inbound `X-Foo`.
When the outpost answers 200 without that header — an anonymous route, a user
with no e-mail, any header authentik chose not to emit — the client's own value
reaches the backend verbatim. Affected 2.10.0–2.11.1, fixed in 2.11.2, latent
since November 2024, so it is in whatever Caddy a typical operator has from the
stable apt repository.

**And layer 2 of the trust boundary exists so that this is a misconfiguration on
someone else's server rather than a bypass here:** the application strips every
inbound identity header itself, including the underscore alias
(`X_authentik_email`) — Go canonicalises `-` but not `_`, so they are two
distinct map keys. Three lines that cannot fail.

## What is genuinely the operator's, and is a deployment step

Neither blocks planning or building. Both belong in `DEPLOY.md` as a verification
the operator performs once, against their own instance:

1. **That their Authentik emits `X-authentik-username`** with the value they
   expect, and that the Subject mode will not be changed afterwards.
2. **That their Caddy is 2.11.2 or newer**, and that the generated Caddyfile
   carries an explicit `request_header -X-Authentik-…` per copied header.

**The acceptance test in criterion 2 is what makes both checkable without
trusting either:** from a second machine, over IPv4 **and** IPv6,
`curl -H 'X-authentik-email: …' http://<server>:8080/admin/` must return a
**login form, not a dashboard**. If the operator's setup is wrong, that command
says so.

## Two decisions this phase must not drift on

- **D-04: the second factor.** The developer decided on 2026-09-03 that an
  Authentik session satisfies it unconditionally. It has **exactly one home** —
  `auth.MustHaveSecondFactor` (`internal/auth/twofactor.go:44`, one function, one
  caller at `:70`). Changed there or nowhere. Because that makes this
  installation's second factor depend on the operator's Authentik enforcing one,
  the dependency is **stated in `DEPLOY.md` and shown in the admin** (SSO-07) —
  not left in a source comment where nobody reads it.
- **D-05: no third role.** `users.role` carries a table-level
  `CHECK (role IN ('admin','editor'))` at `00001:7`, and loosening a table-head
  CHECK in SQLite means rebuilding a table that has foreign-key children.

</decisions>

<insights>
## What this milestone has learned that applies here

- **A gate must measure what its name claims.** Phase 9 produced four failures of
  that one shape. The worst: a `BeginTx` grep measured 0 while a ten-second
  write-connection stall sat one package away. **A gate that proves a proxy is not
  a gate** — and in a security phase that distinction is the whole job.
- **The browser pass runs after the code-review fix round.** Phases 7, 8 and 9 all
  showed why. Here it must run **twice**: once with forward-auth on, once with it
  **off**, because criterion 5 is precisely that nothing about the password path
  changed.
- **A cross-website hole of exactly this family was found and fixed in this
  codebase on 2026-09-06** (`de4a1ce` proves it, `5e453a9` fixes it): four menu
  handlers checked that an item belonged to its menu and never that the menu
  belonged to the website. **The shape to fear is a check that is correct at every
  site the author remembered.** This phase adds the first place in the codebase
  that trusts a header's *claim* about identity — every existing header use is
  either a secret the server verifies (`/ai`'s bearer token) or a value whose only
  guarantee is who the peer is.
- **Counting gates are measured against the post-change tree, never estimated.**

</insights>

<discretion>
## Claude's Discretion

| # | Question | Settled | Evidence |
|---|---|---|---|
| 1 | SSO user with no website group | **Provisioning off by default; with it on, refuse to start without a default website** (D-01) | `handler.go:173`'s „no assignment means every website" inverts under provisioning; the Payrexx pair is the precedent for refusing to start |
| 2 | Which header is the identity | **`X-authentik-username`** (D-02) | `-uid` is the OIDC `sub` and its shape depends on Subject mode; the roadmap itself says pin to username if unsure, and unsure is honest |
| 3 | Which Caddy | **A documented floor of 2.11.2, plus explicit header deletes** (D-03) | CVE-2026-30851 is latent in 2.10.0–2.11.1; layer 2 makes a wrong Caddyfile elsewhere a misconfiguration and not a bypass |
| 4 | Verify against the operator's instance | **A deployment step, not a planning blocker** | Criterion 2's one-command test says whether their setup is wrong, without this phase having to know it |

</discretion>
