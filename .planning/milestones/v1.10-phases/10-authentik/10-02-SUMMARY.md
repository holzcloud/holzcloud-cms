---
phase: 10-authentik
plan: 02
subsystem: auth
tags: [forward-auth, authentik, middleware, trusted-peer, header-stripping, constant-time, context, cve-2026-30851, cve-2026-52845]

requires:
  - phase: 10-authentik
    provides: "plan 10-01's config.SSOEnabled and config.SSOSecret, which cannot be half-set — SSO on without a secret does not load — so the middleware is built on them without a nil check"
  - phase: 01-foundation
    provides: "web.ClientIPResolver.IsTrustedPeer and the web.RequestID precedent (IsTrustedPeer -> sanitise -> use), which this file extends from a claim about a request id to a claim about a person"
provides:
  - "web.ForwardAuth: the first place in this codebase that believes a header's claim about identity, and the four layers that make the believing narrow"
  - "web.Identity, web.IdentityFromContext, web.ProxySecretHeader, web.ForwardAuthOptions and Identity.HasGroup — the surface plans 10-03 through 10-05 are built on"
  - "An unconditional strip of every inbound identity header, driven by a scan of the header map's own keys, wired outermost in cmd/holzcloud so it is a property of the binary and not of one route prefix"
  - "Layer 4 written down and deliberately not built, in the source, so that 'we should parse the JWT' is answered where it will be proposed"
  - "Criterion 2 run live over IPv4 and IPv6 from a genuinely untrusted address against a binary proved to be the one answering"
affects: [10-03-identity-mapping, 10-04-provisioning, 10-05-group-rights, 10-07-admin-surface, 10-08-deploy-docs, 10-10-browser-pass]

actuals:
  tokens: 9516
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "A guard whose source order is the requirement: IsTrustedPeer stands to the left of every header read in one short-circuiting && chain, and an AST test asserts the operand order because no observable behaviour can"
    - "Deletion driven by a scan of the header map's own keys, normalising _ to - before comparing, rather than by a list of names somebody has to keep in step with a program on another machine"
    - "The identity leaves the middleware in the request context and never in a header, so the strip is what makes 'no handler can read a claim' true rather than a convention"
    - "routerTweaks: a zero-value-is-the-old-behaviour options struct for testRouter, so a new test can change SSO settings, the resolver or install a far-side probe without touching the existing suite"

key-files:
  created:
    - internal/web/forwardauth.go
    - internal/web/forwardauth_test.go
  modified:
    - cmd/holzcloud/main.go
    - cmd/holzcloud/main_test.go

key-decisions:
  - "Only the four headers that constitute the identity are read (username, email, name, groups). The plan's action says 'read the seven headers', which contradicts its own prose that -uid is read into nothing and -jwt is read by nothing; reading a header into a discarded variable is dead code that would also break the r.Header[ gate's intent. Divergence recorded."
  - "The strip deletes with `delete(r.Header, name)` and never `Header.Del`, because Del canonicalises the name it is given and would not remove the underscore spelling — the exact CVE-2026-52845 shape."
  - "The ordering claim is proved by an AST test over ForwardAuth's own source, not by behaviour. No observable answer distinguishes 'peer checked first' from 'peer checked second' — both refuse — so the property is a source-order property and the test asserts source order."
  - "The far-side strip assertion at router level uses two observables of unequal strength, and the summary says which is which rather than presenting the weaker one as the stronger."
  - "IsTrustedPeer is named once in code and nowhere in a comment of the new file, because the counting gate greps non-test files without stripping comments and the pair (RemoteAddr 3, IsTrustedPeer 4) is load-bearing."

patterns-established:
  - "Commit the task before mutating it. Every guard in this plan was mutation-verified against a committed file, which is what wave 1's lost task taught."
  - "A live gate must first prove which process answered it. Three server runs in this plan silently failed to bind and were answered by a foreign listener; the transcript that stands carries the listener PID beside the launched PID."
  - "A live trust gate carries a negative control: the same address, the same command, one configuration change, and the answer flips."

requirements-completed: [SSO-02, SSO-03, SSO-04]

coverage:
  - id: D1
    description: "The peer address is checked before any identity header is read, and the source order is the proof: IsTrustedPeer stands to the left of every header read in one short-circuiting expression"
    requirement: "SSO-02"
    verification:
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestForwardAuthChecksThePeerBeforeItReadsAHeader"
        status: pass
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestForwardAuthDisbelievesAnUntrustedPeer/over_IPv4 and /over_IPv6"
        status: pass
      - kind: other
        ref: "mutation 9 — the peer check moved right of secretMatches: the ordering test goes red while every behavioural test stays green, which is the point"
        status: pass
    human_judgment: false
  - id: D2
    description: "The trusted-peer check is web.ClientIPResolver.IsTrustedPeer and there is no second one: RemoteAddr is still read in exactly the three places it was read before this plan"
    requirement: "SSO-02"
    verification:
      - kind: other
        ref: "grep -rn RemoteAddr internal/ cmd/ --include='*.go' | grep -v _test | wc -l == 3, named: clientip.go:33, clientip.go:51, logging.go:130"
        status: pass
      - kind: other
        ref: "grep -rn IsTrustedPeer internal/ cmd/ --include='*.go' | grep -v _test | wc -l == 4, named: logging.go:35, clientip.go:47 (doc), clientip.go:50 (decl), forwardauth.go:135 (the one new call site)"
        status: pass
    human_judgment: false
  - id: D3
    description: "An untrusted peer carrying no identity header falls through to the ordinary password form and never to a 403 — the way back in does not die with the proxy"
    requirement: "SSO-02"
    verification:
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestForwardAuthLetsAnUntrustedPeerThroughUnharmed"
        status: pass
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestUntrustedPeerWithIdentityHeaderGetsTheLoginForm (5 subtests, IPv4 and IPv6)"
        status: pass
      - kind: e2e
        ref: "curl -H 'X-authentik-email: …' http://192.168.0.63:8123/admin/ and http://[2a02:21b4:b04f:5500:96:4223:a182:78ba]:8123/admin/ against the built binary — 303 to /admin/login, both families, with and without the correct proxy secret"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every inbound header whose name normalises to the x-authentik- prefix is deleted before the next handler runs, whoever the peer was, driven by a scan of the header map's own keys"
    requirement: "SSO-03"
    verification:
      - kind: unit
        ref: "internal/web/forwardauth_test.go — assertNoIdentityHeaderSurvives walks the header map the probe handler was given; called from 6 test functions across 12 wire spellings and 3 raw map keys"
        status: pass
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestIdentityHeadersAreStrippedEverywhere/an_admin_route,_seen_from_the_far_side_of_the_chain"
        status: pass
      - kind: other
        ref: "mutation 5 — the strip given a condition: 10 keys reach the handler, red"
        status: pass
    human_judgment: false
  - id: D5
    description: "The underscore alias is covered because the scan normalises _ to - before comparing, and a fixed list of hyphenated names would not be"
    requirement: "SSO-03"
    verification:
      - kind: other
        ref: "mutation 6 — the key scan replaced by a fixed hyphenated list using Header.Del: X_authentik_email, X_authentik_groups, x-authentik-email, X-AUTHENTIK-EMAIL and x_authentik_username all survive, red"
        status: pass
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestForwardAuthDisbelievesAnUntrustedPeer (the spelling table includes both underscore spellings)"
        status: pass
    human_judgment: false
  - id: D6
    description: "The proxy proves it is the proxy with crypto/subtle.ConstantTimeCompare against a secret read from the environment, and an empty configured secret can never match"
    requirement: "SSO-04"
    verification:
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestForwardAuthRefusesTheWrongSecret (3 subtests) and #TestForwardAuthNeverMatchesAnEmptyConfiguredSecret (3 subtests)"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/web/forwardauth.go | grep -c ConstantTimeCompare == 1"
        status: pass
    human_judgment: false
  - id: D7
    description: "The identity leaves the middleware in the request context and never in a header, and the shared secret is stripped with the rest so no handler can read the secret protecting it"
    requirement: "SSO-03"
    verification:
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestForwardAuthStripsTheProxySecretItself"
        status: pass
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestIdentityFromContextOnAContextThatNeverSawTheMiddleware"
        status: pass
    human_judgment: false
  - id: D8
    description: "X-authentik-groups is split on the pipe with whole elements compared, empty elements dropped, so an empty header yields no groups rather than one group named the empty string"
    verification:
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestSplitGroups (6 subtests) and #TestHasGroupComparesWholeElements"
        status: pass
    human_judgment: false
  - id: D9
    description: "With HOLZCLOUD_SSO_ENABLED false the middleware still strips and never populates: the strip is unconditional and the trust is not"
    requirement: "SSO-03"
    verification:
      - kind: unit
        ref: "internal/web/forwardauth_test.go#TestForwardAuthStripsWhileSwitchedOff"
        status: pass
    human_judgment: false
  - id: D10
    description: "The middleware is wrapped outermost, above RequestID and around the whole mux, so no handler in the binary — not even the access log — sees an unstripped identity header"
    requirement: "SSO-03"
    verification:
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestIdentityHeadersAreStrippedEverywhere (4 subtests: an admin route from the far side, /healthz, a public route, and /healthz still answering)"
        status: pass
      - kind: other
        ref: "grep -n web.ForwardAuth cmd/holzcloud/main.go == 1232, greater than web.RequestID at 1231; sed -n '/mux.Handle(\"\\/admin\\/\"/p' | grep -c ForwardAuth == 0"
        status: pass
      - kind: other
        ref: "mutation 12 — the middleware moved inside /admin/: /healthz keeps 8 identity headers, red, and the grep gate flips to 1"
        status: pass
    human_judgment: false
  - id: D11
    description: "The signed assertion is deliberately not built and explains itself in the source, so 'we should parse the JWT' is answered where it will be proposed"
    verification:
      - kind: other
        ref: "internal/web/forwardauth.go, the closing block comment: JWKS at runtime is the one rule this project does not break; without a signing key authentik signs symmetrically with the client secret; the signature protects nothing the transport does not; parsing without verifying is strictly worse than not having it"
        status: pass
    human_judgment: true
    rationale: "That the reason is adequate and lives where a later reader will look is a judgment about prose, not something a test can assert. The mechanical half — X-authentik-jwt is stripped and read by nothing — is covered by D4."

duration: 40 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 02: The Security Core Summary

**One middleware in which the peer address is checked to the left of every header read in a single short-circuiting expression, an unconditional key-scan strip that removes every spelling of every identity header including the underscore alias, a constant-time shared-secret comparison, and the fourth layer written down and deliberately not built — wired outermost so that all of it is a property of the binary rather than of one route prefix.**

Nothing in this plan signs anybody in. After it lands a correct request from the proxy carries an identity in its context and every handler ignores it, which is exactly what the build order's step ② is supposed to look like.

## Performance

- **Duration:** ≈ 40 min. The start was not stamped before context reading; the first commit is `fb8daab` at 2026-09-08 01:42:27 +0200 and the last verification finished 01:58.
- **Tasks:** 3
- **Files:** 2 created, 2 modified

## Accomplishments

- **`internal/web/forwardauth.go` — the four layers, and the order is the requirement.** The guard is one `&&` chain: `opts.Enabled && opts.Secret != "" && resolver != nil && resolver.IsTrustedPeer(r) && secretMatches(r, opts.Secret)`. Go stops at the first false, so an untrusted peer's headers are never fetched — not the secret, not one identity header. The comment says the ordering is the mechanism rather than a style choice.
- **The strip is a scan, not a list.** `stripIdentityHeaders` ranges over `r.Header`'s own keys, normalises each with `strings.ToLower(strings.ReplaceAll(k, "_", "-"))`, collects the doomed keys and then deletes them with `delete(r.Header, name)` — never `Header.Del`, which canonicalises the name it is given and would leave `X_authentik_email` behind. It runs on every path with no condition; that absence of a condition is its whole value.
- **The proxy secret is stripped with the identity headers.** A handler cannot read the secret it is being protected by out of the request it is serving.
- **Wired outermost at `cmd/holzcloud/main.go:1232`**, above `RequestID` and around the whole mux. Not inside `/admin/` — that is the difference between "the admin strips identity headers" and "this program strips identity headers", and the mutation that scopes it to `/admin/` leaves eight identity headers on `/healthz`.
- **Layer 4 is written down and not built**, in a block comment at the foot of the file, with the three reasons: JWKS at runtime is the one rule this project does not break; without a signing key authentik signs proxy tokens symmetrically with the client secret; and parsing a JWT without verifying it is strictly worse than not having it, because it looks like a defence.
- **Criterion 2 was run for real** against the built binary, over IPv4 and IPv6, from an address that is genuinely not in `HOLZCLOUD_TRUSTED_PROXIES` — after three earlier runs were discovered to have been answered by a foreign process. See "The finding that nearly became evidence".
- **Every guard mutation-verified**: twelve mutations, twelve reds, verbatim output below. Each was applied to a **committed** file, restored with `git checkout --`, and the suite re-run green — which is wave 1's lesson applied.

## Task Commits

1. **Task 1 (RED): failing tests for the forward-auth middleware** — `fb8daab` (test)
2. **Task 1 (GREEN): the four layers of forward authentication, in order** — `b5a4285` (feat)
3. **Task 2: wire the forward-auth strip as the outermost middleware** — `56607a0` (feat)
4. **Task 3: criterion 2 as a runnable gate, and the counting gates** — no code commit. The plan's own action says "no new production code beyond a comment", and the comment it alludes to (why the JWT is not parsed, why the ordering is the mechanism) was already written in Task 1. Task 3's output is the transcript and the table below. Recorded as a divergence rather than a gratuitous comment added to satisfy a file list.

_Task 1 carried `tdd="true"`. The RED commit's tests do not compile against the pre-change package — `undefined: Identity`, `undefined: ForwardAuth`, `undefined: ProxySecretHeader` — which is the intended failure._

## Files Created/Modified

- `internal/web/forwardauth.go` (new, 249 lines) — the three-paragraph package doc block, `ProxySecretHeader`, `Identity` with D-02 written into its comment, `HasGroup`, `ForwardAuthOptions`, `IdentityFromContext`, `ForwardAuth`, `stripIdentityHeaders`, `isIdentityHeader`, `secretMatches`, `splitGroups`, and the layer-4 non-decision
- `internal/web/forwardauth_test.go` (new, 420 lines) — 13 test functions, 24 named subtests, the twelve-spelling wire table plus three raw map keys, and the AST ordering gate
- `cmd/holzcloud/main.go` — one wrap at `:1232` and the extended comment above the chain explaining why it is above `RequestID`
- `cmd/holzcloud/main_test.go` — `routerTweaks` and `testRouterWith`, plus `TestIdentityHeadersAreStrippedEverywhere` and `TestUntrustedPeerWithIdentityHeaderGetsTheLoginForm`

## Task 3, part one: criterion 2, run against a running binary

### The finding that nearly became evidence

**Three of my server runs never bound the port, and I collected a full transcript from a foreign process before noticing.**

The first run was launched on port 8099 with `HOLZCLOUD_LISTEN=::`. `netstat` showed a listener, curl answered `303 → /admin/login` from both address families, and the `X-Request-ID` probes discriminated exactly as expected. Every one of those answers came from something else. The launch log ends:

```
{"msg":"server starting","addr":"[::]:8099"}
{"level":"ERROR","msg":"server error","err":"listen tcp 0.0.0.0:8099: bind: address already in use"}
```

Something — a leftover dev server, `lsof` named PID 37911 running `holzcloud` — already held 8099 and answered everything. It was caught only because a later run on a genuinely free port answered `303 → /admin/setup` instead of `/admin/login`: an empty database redirects to the setup form, so the earlier `/admin/login` could only have come from a database that had users, which mine did not.

This is exactly the shape `10-CONTEXT.md` names — *a gate that proves a proxy is not a gate* — and it was one paragraph away from being written into this summary as proof. The run recorded below therefore begins by proving **which process answered**: the PID returned by `lsof -nP -iTCP:8123 -sTCP:LISTEN -t` is compared with the PID of the process that was launched, and the log is checked for zero bind errors.

**Rule for the rest of this phase:** a live gate against a running binary states the listener PID beside the launched PID, or it is not evidence.

### The run that stands

Built with `CGO_ENABLED=0 go build`, started on port **8123** (verified free first) with:

```
HOLZCLOUD_LISTEN=::  HOLZCLOUD_PORT=8123
HOLZCLOUD_DATA_DIR=<scratch>/crit2/data
HOLZCLOUD_SSO_ENABLED=true  HOLZCLOUD_SSO_SECRET=the-real-secret
```

`HOLZCLOUD_TRUSTED_PROXIES` left at its default `127.0.0.1/32,::1/128`.

```
started pid=49491; bind errors=0
listener pid: 49491          <- the same process, so the answers below are this binary's
"msg":"server starting","addr":"[::]:8123"
```

Binding every interface is deliberate. The obvious substitute — leave the server on loopback and curl it from elsewhere — answers `Connection refused`, which is a pass in the sense that no dashboard came back and proves **nothing at all** about layers 1 to 3. It proves the listener.

**What stood in for a second machine:** the host's own non-loopback interface addresses on `en0` — **`192.168.0.63`** for IPv4 and the global **`2a02:21b4:b04f:5500:96:4223:a182:78ba`** for IPv6. A packet addressed to either leaves the loopback path, and `RemoteAddr` carries the interface address, which is exactly what a second machine would produce. Neither is in the default trusted range. **A non-loopback IPv6 address exists on this machine, so the finding the plan asks to hand to 10-10 does not arise.**

The installation was first set up through its own `POST /admin/setup` (with the CSRF token from the form), so that an unauthenticated `/admin/` answers the **login** form rather than the setup form. Without that step the redirect is `/admin/setup` for every caller and the gate would have proved nothing about authentication.

**The four commands criterion 2 asks for, and two more:**

```
$ curl -sS -o /dev/null -w '%{http_code} %{redirect_url}\n' \
    -H 'X-authentik-email: stranger@example.com' -H 'X-authentik-username: stranger' \
    http://<addr>:8123/admin/

0. control, loopback (trusted), no identity header:
   303 http://127.0.0.1:8123/admin/login
1. IPv4 untrusted, identity headers:
   303 http://192.168.0.63:8123/admin/login
2. IPv6 untrusted, identity headers:
   303 http://[2a02:21b4:b04f:5500:96:4223:a182:78ba]:8123/admin/login
3. IPv4 untrusted, identity headers AND the correct proxy secret:
   303 http://192.168.0.63:8123/admin/login
4. IPv6 untrusted, identity headers AND the correct proxy secret:
   303 http://[2a02:21b4:b04f:5500:96:4223:a182:78ba]:8123/admin/login
5. IPv4 untrusted, the underscore alias (X_authentik_email, X_authentik_username):
   303 http://192.168.0.63:8123/admin/login

6. and what /admin/login actually is, fetched from the untrusted IPv4 address
   with the identity headers set:
   <title>Anmelden — Holzcloud
   name="email"
   type="password"
```

Steps 3 and 4 are the ones worth stating plainly: **layer 1 alone is enough, and a secret that leaked does not buy a peer the right to be believed.**

### What that run does and does not prove, stated honestly

It proves the port answered from a non-loopback address, that the answer is the login form and not a dashboard, and that adding the correct shared secret does not change it.

It does **not** on its own distinguish "the header was disbelieved" from "nothing reads the header yet" — and after this plan nothing does, by construction. So the run was given a discriminator using the binary's own trusted-peer machinery, which answers out loud: `web.RequestID` adopts a client-chosen `X-Request-ID` **only from a trusted peer**, and it calls the same `IsTrustedPeer` the forward-auth guard calls.

```
$ curl -H 'X-Request-ID: chosen-by-the-client' http://<addr>:8123/healthz   (response header)

   loopback (trusted)     chosen-by-the-client
   IPv4 non-loopback      b7bce346f8042dcb        <- a fresh id: the claim was discarded
   IPv6 non-loopback      ae8f1c260489b9b2        <- likewise
```

**Negative control** — the same binary, the same addresses, the same command, one configuration change (`HOLZCLOUD_TRUSTED_PROXIES=192.168.0.0/16`, replacing the loopback default), listener PID 49777, zero bind errors:

```
   loopback (no longer listed)  1c5fc886feeae338      <- now discarded
   IPv4 (now listed)            chosen-by-the-client  <- now adopted
   IPv6 (still not listed)      7ce4b43b30b04a9d      <- still discarded
```

The classification flips exactly where the configuration says it should. That is what turns "the port answered 303" into "both non-loopback addresses are genuinely untrusted peers to this binary, over both families, and the trust is configuration-driven rather than an accident of the address".

The scratch data directory was removed afterwards; the repository's own `data/` was never opened (its mtime is unchanged at Sep 4 18:51).

## Task 3, part two: the counting gates, measured against the post-change tree

**Baseline measured immediately before the first commit of this plan, on 2026-09-08, not taken from the plan.** The plan's own note says these rows are deltas; four of its absolute numbers had already moved again between the plan being written and this wave running, and the migration count moved once more *during* it (50 → 51, the Phase 11 fix round's `00051_album_updated_at.sql`).

| What | Command | Plan's baseline | Measured baseline (pre-change) | This plan adds | Measured after | Verdict |
|---|---|---|---|---|---|---|
| files using `RemoteAddr` | `grep -rn RemoteAddr internal/ cmd/ --include='*.go' \| grep -v _test \| wc -l` | 3 | **3** | 0 | **3** | exactly as predicted |
| `IsTrustedPeer` mentions | `grep -rn IsTrustedPeer internal/ cmd/ --include='*.go' \| grep -v _test \| wc -l` | 3 | **3** | 1 | **4** | exactly as predicted |
| files in `internal/web` | `ls internal/web/*.go \| grep -v _test \| wc -l` | 13 | **13** | 1 | **14** | exactly as predicted |
| packages under `internal/` | `ls -d internal/*/ \| wc -l` | vorher | **41** | 0 | **41** | delta 0 as required |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | vorher (49 as written) | **51** | 0 | **51** | delta 0 as required; the literal gate value 49 is stale twice over |
| `adminProtectedMux.Handle` | `grep -c 'adminProtectedMux.Handle' cmd/holzcloud/main.go` | vorher | **159** | 0 | **159** | delta 0 as required |
| admin templates | `ls cmd/holzcloud/templates/admin/*.html \| wc -l` | vorher | **68** | 0 | **68** | delta 0 as required |
| `adminOnly` rows | the table in `main_test.go` | 19 | **19** | 0 | **19** | exactly as predicted |
| strings in source | `go run ./tools/i18n \| head -1` | vorher (1277 as written) | **1311** | 0 | **1311** | delta 0 as required |

**The load-bearing pair.** `RemoteAddr` 3 → 3 and `IsTrustedPeer` 3 → 4 is the reuse the roadmap asked for. Verified by name and not by count alone:

- `RemoteAddr`: `internal/web/clientip.go:33`, `clientip.go:51`, `internal/web/logging.go:130`. Unchanged.
- `IsTrustedPeer`: `logging.go:35` (the `RequestID` call), `clientip.go:47` (the doc-comment line), `clientip.go:50` (the declaration), and **`forwardauth.go:135`** — the single new call site, inside the guard.

**One thing this cost.** The `IsTrustedPeer` gate greps non-test files **without stripping comments**, so the doc comment above `ForwardAuth` originally pushed the count to 5. The comment now reads "the trusted-peer check standing to the left of secretMatches" instead of naming the function. The prose is unharmed; it is worth recording that the gate as written forbids a comment from mentioning the function it documents, which is a cost the next plan touching this file should know about.

**i18n, before and after:** `1311 Zeichenketten im Quelltext`, and `34 offen, 0 verwaist` on `en`, `es`, `fr`, `it`. **Delta: 0.** Nothing in this plan is read by a person on a screen. Those 34 are pre-existing and belong to plan 11-07, exactly as wave 1 recorded.

## Decisions Made

- **Only four headers are read, not seven.** The plan's action text says "read the seven headers with `r.Header.Get`", while its own prose says `-uid` "is read into nothing" and `-jwt` is "read by nothing". Reading a header into a discarded variable is dead code and would weaken rather than strengthen the file. `X-authentik-username`, `-email`, `-name` and `-groups` are read; `-uid`, `-entitlements`, `-jwt` and `-meta-*` are read by nothing and stripped like the rest. Recorded as a divergence below.
- **`delete(r.Header, name)`, never `Header.Del`.** `Del` canonicalises the name it is given. Deleting `X_authentik_email` through `Del` would look right and do nothing. The comment says so at the call site; mutation 6 proves it.
- **The ordering gate is an AST test, not a behavioural one.** No observable answer distinguishes "the peer was checked first" from "the peer was checked second", because both paths refuse. The test parses `forwardauth.go`, flattens the guard's `&&` chain, and asserts that the `IsTrustedPeer` operand precedes the `secretMatches` operand and that no `Header` selector inside `ForwardAuth` appears before it. Mutation 9 — swapping the two operands — turns that test red while every behavioural test stays green, which is precisely the gap it exists to close.
- **`routerTweaks` rather than a second `testRouter`.** The zero value reproduces the old behaviour exactly (nil resolver, SSO off, pass-through setup guard), so the whole existing suite is untouched, and a new test changes only what it cares about.
- **The router-level control subtest is deliberately fragile.** `a trusted peer with the right secret, because nothing signs anybody in yet` asserts 303 today. Plan 10-03 will have to change that line, and it should have to — it is where the build order is written into the suite.

## Deviations from Plan

### Auto-fixed and documented

**1. [Rule 3 — Blocking] Four headers read, not seven**

- **Found during:** Task 1
- **Issue:** The plan's action says "read the seven headers"; its own prose one paragraph earlier says `-uid` is read into nothing and its layer-4 block says `-jwt` is read by nothing. Both cannot hold.
- **Fix:** The four headers that constitute the identity are read; the rest are read by nothing and stripped.
- **Files:** `internal/web/forwardauth.go`
- **Verification:** every behavioural bullet of the plan's own list has a passing named subtest; `grep -c 'r.Header\['` is 0 as the plan requires.
- **Committed in:** `b5a4285`

**2. [Rule 3 — Blocking] The doc comment may not name `IsTrustedPeer`**

- **Found during:** Task 1 gate run
- **Issue:** The counting gate greps non-test files without stripping comments. Naming the function in `ForwardAuth`'s doc comment put the count at 5, which the plan defines as "it checks twice".
- **Fix:** the comment says "the trusted-peer check" instead. The gate reads 4.
- **Files:** `internal/web/forwardauth.go`
- **Committed in:** `b5a4285`

**3. [Rule 2 — Missing critical] The proxy secret header is deleted by the same scan**

- **Found during:** Task 1
- **Issue:** The plan asks for this in its behaviour list but the scan's prefix rule alone would not have covered it — `X-Holzcloud-Proxy-Secret` does not start with `x-authentik-`.
- **Fix:** `isIdentityHeader` also matches the normalised `ProxySecretHeader`.
- **Verification:** `TestForwardAuthStripsTheProxySecretItself`; mutation 11.
- **Committed in:** `b5a4285`

**4. [Rule 2 — Missing critical] `cmd/holzcloud/main_test.go` needed a harness change**

- **Found during:** Task 2
- **Issue:** `testRouter` passes no `clientIP` and no SSO configuration, so a test of "an untrusted peer is disbelieved" would have passed vacuously — with a nil resolver the guard short-circuits before the peer is even considered.
- **Fix:** `routerTweaks` / `testRouterWith`, zero value identical to the previous behaviour.
- **Verification:** the whole `cmd/holzcloud` suite passes unchanged; the new tests build a router with SSO on and a real loopback resolver.
- **Committed in:** `56607a0`

### Divergences recorded rather than worked around

**5. Task 3 produced no code commit.** Its own action says "no new production code beyond a comment", and the comment it points at was already written in Task 1. Adding a comment purely to make a file appear in a commit would be worse than recording this.

**6. The strip is proved at router level by two observables of unequal strength, and this is which.** The plan explicitly asks for this to be written down.

- **The strong one, used for an admin route:** `setupGuard` is a seam `newRouter` already takes from its caller. A test installs a recorder there; it sits inside every middleware wrapped around the mux, so the header map it records is the one an admin handler is given. That is a true far-side read.
- **The weaker one, used for `/healthz` and a public route:** there is no seam on those paths, and registering a probe route would mean changing `newRouter`'s shape for a test's benefit. The assertion is made on the request object the router was handed. `stripIdentityHeaders` deletes from `r.Header` **in place**, and no middleware re-adds a header, so a key still present afterwards is a key no strip removed — which is exactly the claim under test ("the strip is a property of the binary, not of `/admin/`"), and mutation 12 turns it red. What it cannot prove is that the deletion happened **before** `next` ran. `internal/web/forwardauth_test.go` carries that ordering proof from a genuine downstream probe handler.

**7. Three live-gate runs were answered by a foreign process.** Full account above. No code defect; a procedural one, and the most useful thing in this summary.

**8. The plan's literal `ls internal/db/migrations/*.sql | wc -l` gate expects 49; the tree has 51.** Stale twice over — once from Phase 11's first migration and once from its fix round, which landed `00051_album_updated_at.sql` while this plan was executing. Re-anchored to the delta, which is 0, per the plan's own instruction and wave 1's precedent.

---

**Total deviations:** 4 auto-fixed (2 blocking, 2 missing-critical) + 4 divergences recorded.
**Impact:** No scope creep. Three of the four auto-fixes exist to make the plan's own stated must-haves provable; the fourth resolves a contradiction between the plan's action text and its own prose.

## Mutation Verification

Every guard was removed or inverted on a **committed** file, run against a named test, and restored with `git checkout --`. Twelve mutations, twelve reds. Verbatim output.

**1. Layer 1 removed — the peer is no longer checked**
```
--- FAIL: TestForwardAuthDisbelievesAnUntrustedPeer (0.00s)
    --- FAIL: TestForwardAuthDisbelievesAnUntrustedPeer/over_IPv4 (0.00s)
        forwardauth_test.go:142: an untrusted peer was believed: &{Username:stranger Email:stranger@example.com Name:stranger@example.com Groups:[stranger@example.com]}
    --- FAIL: TestForwardAuthDisbelievesAnUntrustedPeer/over_IPv6 (0.00s)
        forwardauth_test.go:142: an untrusted peer was believed: &{Username:stranger Email:stranger@example.com Name:stranger@example.com Groups:[stranger@example.com]}
```

**2. Layer 3 removed — the secret no longer has to match**
```
--- FAIL: TestForwardAuthRefusesTheWrongSecret (0.00s)
    --- FAIL: TestForwardAuthRefusesTheWrongSecret/a_trusted_peer_with_the_wrong_secret (0.00s)
        forwardauth_test.go:189: the secret did not have to match: &{Username:ada Email:stranger@example.com Name:stranger@example.com Groups:[stranger@example.com]}
    --- FAIL: TestForwardAuthRefusesTheWrongSecret/a_trusted_peer_with_no_secret_header_at_all (0.00s)
        forwardauth_test.go:189: the secret did not have to match: &{Username:ada ...}
    --- FAIL: TestForwardAuthRefusesTheWrongSecret/a_trusted_peer_with_an_empty_secret_header (0.00s)
        forwardauth_test.go:189: the secret did not have to match: &{Username:ada ...}
```

**3. The master switch removed — single sign-on off no longer means no identity**
```
--- FAIL: TestForwardAuthStripsWhileSwitchedOff (0.00s)
    forwardauth_test.go:205: single sign-on is off and an identity was produced anyway: &{Username:ada Email:stranger@example.com Name:stranger@example.com Groups:[stranger@example.com]}
```

**4. The empty-secret guard removed — an empty configured secret becomes matchable**
```
--- FAIL: TestForwardAuthNeverMatchesAnEmptyConfiguredSecret (0.00s)
    --- FAIL: TestForwardAuthNeverMatchesAnEmptyConfiguredSecret/no_secret_header (0.00s)
        forwardauth_test.go:227: an empty configured secret was matchable: &{Username:ada ...}
    --- FAIL: TestForwardAuthNeverMatchesAnEmptyConfiguredSecret/an_empty_secret_header (0.00s)
        forwardauth_test.go:227: an empty configured secret was matchable: &{Username:ada ...}
```
(`ConstantTimeCompare` of two empty slices returns 1. The guard is one character wide and this is why plan 10-01 also refuses to start in that configuration.)

**5. Layer 2 given a condition — the strip only runs when an identity was built**
```
--- FAIL: TestForwardAuthDisbelievesAnUntrustedPeer/over_IPv4 (0.00s)
        forwardauth_test.go:144: the handler was given "X-Authentik-Username"; every identity header must be gone before it runs
        forwardauth_test.go:144: the handler was given "X-Authentik-Uid"; ...
        forwardauth_test.go:144: the handler was given "X-Authentik-Groups"; ...
        forwardauth_test.go:144: the handler was given "X_authentik_email"; ...
        forwardauth_test.go:144: the handler was given "x_authentik_username"; ...
        forwardauth_test.go:144: the handler was given "X-Holzcloud-Proxy-Secret"; a handler must not be able to read the secret protecting it
        forwardauth_test.go:144: the handler was given "X-Authentik-Entitlements"; ...
        forwardauth_test.go:144: the handler was given "X-Authentik-Jwt"; ...
        forwardauth_test.go:144: the handler was given "X-AUTHENTIK-EMAIL"; ...
        forwardauth_test.go:144: the handler was given "X-Authentik-Email"; ...
```

**6. The key scan replaced by a fixed hyphenated list using `Header.Del` — the CVE-2026-52845 shape, exactly**
```
--- FAIL: TestForwardAuthDisbelievesAnUntrustedPeer/over_IPv4 (0.00s)
        forwardauth_test.go:144: the handler was given "X_authentik_email"; every identity header must be gone before it runs
        forwardauth_test.go:144: the handler was given "X_authentik_groups"; ...
        forwardauth_test.go:144: the handler was given "x-authentik-email"; ...
        forwardauth_test.go:144: the handler was given "X-AUTHENTIK-EMAIL"; ...
        forwardauth_test.go:144: the handler was given "x_authentik_username"; ...
    --- FAIL: TestForwardAuthDisbelievesAnUntrustedPeer/over_IPv6 (0.00s)
        (the same five)
```
The list deleted all eight hyphenated names it knew about and left every underscore and non-canonical spelling standing. This is the mutation the whole layer exists for.

**7. `splitGroups` no longer drops empty elements**
```
--- FAIL: TestSplitGroups/an_empty_element_is_dropped (0.00s)
        forwardauth_test.go:296: splitGroups("a||b") = []string{"a", "", "b"}; want []string{"a", "b"}
--- FAIL: TestSplitGroups/an_empty_header_is_no_groups,_not_one_empty_group (0.00s)
        forwardauth_test.go:296: splitGroups("") = []string{""}; want []string(nil)
--- FAIL: TestSplitGroups/only_separators_is_no_groups (0.00s)
        forwardauth_test.go:296: splitGroups("||") = []string{"", "", ""}; want []string(nil)
--- FAIL: TestSplitGroups/spaces_are_trimmed (0.00s)
        forwardauth_test.go:300: splitGroups(" a | b ") = []string{" a ", " b "}; want []string{"a", "b"}
```

**8. `HasGroup` compares by substring**
```
--- FAIL: TestHasGroupComparesWholeElements (0.00s)
    forwardauth_test.go:310: not-holzcloud-admins was accepted as holzcloud-admins
```

**9. The peer check moved to the right of the secret comparison — the ordering mutation**
```
--- FAIL: TestForwardAuthChecksThePeerBeforeItReadsAHeader (0.00s)
    forwardauth_test.go:384: the secret is compared at operand 3 and the peer at 4; the peer must be checked first, because && short-circuits left to right and a secret comparison reads a header
```
**Every behavioural test stayed green under this mutation.** That is the whole argument for the gate: the property is not observable from outside, so a suite without it would have shipped the reordering.

**10. An identity with no username is accepted**
```
--- FAIL: TestForwardAuthNeedsAUsername (0.00s)
    forwardauth_test.go:245: an identity with no username was accepted: &{Username: Email:ada@example.com Name: Groups:[]}
```

**11. The shared secret is left on the request**
```
--- FAIL: TestForwardAuthStripsTheProxySecretItself (0.00s)
    forwardauth_test.go:257: the handler was given "X-Holzcloud-Proxy-Secret"; a handler must not be able to read the secret protecting it
    forwardauth_test.go:257: the handler was given "x-holzcloud-proxy-secret"; ...
    forwardauth_test.go:259: the handler can read the shared secret: "correct-horse-battery-staple"
```

**12. The middleware moved inside `/admin/` instead of around the whole mux**
```
--- FAIL: TestIdentityHeadersAreStrippedEverywhere/the_liveness_probe (0.00s)
        main_test.go:793: GET /healthz: "X-Authentik-Entitlements" survived; stripping identity headers is a property of this binary
        main_test.go:793: GET /healthz: "X-Authentik-Jwt" survived; ...
        main_test.go:793: GET /healthz: "X-Authentik-Meta-Provider" survived; ...
        main_test.go:793: GET /healthz: "X_authentik_groups" survived; ...
        main_test.go:793: GET /healthz: "X-AUTHENTIK-EMAIL" survived; ...
        main_test.go:793: GET /healthz: "x_authentik_username" survived; ...
        main_test.go:793: GET /healthz: "X-Authentik-Email" survived; ...
        main_test.go:793: GET /healthz: "X-Authentik-Name" survived; ...

gate 'ForwardAuth scoped to /admin/' (want 0): 1   <- red
```

**13. The outermost wrap removed entirely** (bonus)
```
--- FAIL: TestIdentityHeadersAreStrippedEverywhere (0.91s)
gate 'web.ForwardAuth present in main.go' (want 1): 0   <- red
```

After every mutation the file was restored with `git checkout --` and the suite re-run green.

## Verification Results

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | silent |
| `gofmt -l .` | silent (empty) |
| `go test ./...` | **0 failures** |
| `go test ./internal/web/ -run ForwardAuth -v` | 13 test functions, 24 named subtests, all PASS — including `over_IPv4` and `over_IPv6` as separate subtests, which criterion 2 names separately |
| `go test ./cmd/holzcloud/ -run 'Stripped\|UntrustedPeer' -v` | 9 subtests, all PASS (4 strip + 5 untrusted-peer), more than the 4 the plan requires |
| `grep -rn RemoteAddr … \| grep -v _test \| wc -l` | 3 ✓ |
| `grep -rn IsTrustedPeer … \| grep -v _test \| wc -l` | 4 ✓ |
| `ls internal/web/*.go \| grep -v _test \| wc -l` | 14 ✓ |
| `grep -v '^\s*//' forwardauth.go \| grep -c ConstantTimeCompare` | 1 ✓ |
| `grep -v '^\s*//' forwardauth.go \| grep -c 'r.Header\['` | 0 ✓ |
| `go list -deps ./internal/web/ \| … \| grep -c internal/config` | 0 ✓ — `internal/web` did not gain a dependency on `internal/config` |
| `grep -n web.ForwardAuth cmd/holzcloud/main.go` | 1232, greater than `web.RequestID` at 1231 ✓ (bottom-up wrapping: the last wrap is outermost) |
| `sed -n '/mux.Handle("\/admin\/"/p' \| grep -c ForwardAuth` | 0 ✓ |
| `go run ./tools/i18n \| head -1` | `1311 Zeichenketten im Quelltext` before **and** after; `34 offen, 0 verwaist` on en/es/fr/it, unchanged |
| `ls internal/db/migrations/*.sql \| wc -l` | 51 — the plan's literal gate expects 49 and is stale twice over; delta 0, which is the assertion that matters |
| criterion 2, live, IPv4 + IPv6, untrusted address | 303 → `/admin/login` in all four required forms, listener PID proved |

## Issues Encountered

- **Three server runs never bound the port and a full transcript was collected from a foreign process.** Documented in full above; caught by a redirect target that could not have come from an empty database. The recorded run proves its own listener.
- **A concurrent agent's uncommitted work broke `go build ./...` for about thirty seconds** (`internal/bundle/import.go`: `albumSlugs` map type mid-refactor). Waited for it to settle rather than touching their files. Every commit here was staged file by file and verified with `git show --stat`; the three commits touch only `internal/web/forwardauth.go`, `internal/web/forwardauth_test.go`, `cmd/holzcloud/main.go` and `cmd/holzcloud/main_test.go`.
- **The `IsTrustedPeer` counting gate forbids a comment from naming the function it documents.** Not a defect; a cost, recorded so the next plan touching this file is not surprised by it.

## Threat Flags

None new. Every `mitigate` disposition in the plan's register has a passing test and a red mutation above:

| Threat | Covered by |
|---|---|
| T-10-07 spoofing a header directly | mutations 1, 2; live criterion 2; `TestForwardAuthDisbelievesAnUntrustedPeer` |
| T-10-08 the client's own header surviving Caddy 2.10.0–2.11.1 | mutation 5; `TestIdentityHeadersAreStrippedEverywhere` |
| T-10-09 the underscore alias | mutation 6 — the decisive one |
| T-10-10 timing on the secret comparison | one `ConstantTimeCompare`, gate-counted |
| T-10-11 a handler reading the shared secret | mutation 11 |
| T-10-12 an unverified JWT parsed as a defence | stripped by the same scan, read by nothing, reason in the source |
| T-10-13 a refusal where a fall-through belongs | `TestForwardAuthLetsAnUntrustedPeerThroughUnharmed`; `next` asserted to run exactly once on every path; live 303 |
| T-10-14 an empty configured secret matching | mutation 4, plus plan 10-01's start-up refusal |

The one new surface — the request context now carries an `*Identity` on some requests — is read by nothing in this plan and is the interface 10-03 consumes.

## Known Stubs

None, in the sense that matters: everything this plan builds is reachable and tested. Two things have **no consumer yet, by design and by the build order**:

- `IdentityFromContext` is never called outside its own test. Plan 10-03 is its first caller.
- `Identity.HasGroup` is never called outside its own test. Plan 10-05 is its first caller; the function exists now precisely so that plan cannot get whole-element comparison wrong in a hurry.

Neither can be nil or unvalidated when those plans reach them. This is step ② of the build order looking exactly as it is supposed to: the gate exists before there is anything behind it worth reaching.

## Next Phase Readiness

- **Plan 10-03** puts its sign-in middleware at `cmd/holzcloud/main.go:1036`, between `setupGuard` and `requireAuth`, and reads the identity with `web.IdentityFromContext`. It must not read a header — there are none left by then. It will also have to change the deliberately fragile control subtest `a trusted peer with the right secret, because nothing signs anybody in yet` in `main_test.go`, and that is the point of it.
- **Carry these baselines forward, measured today:** migrations **51**, packages **41**, admin templates **68**, source strings **1311**, `adminProtectedMux.Handle` **159**, `internal/web` non-test files **14**, `RemoteAddr` **3**, `IsTrustedPeer` **4**.
- **`RemoteAddr` must stay at 3 for the rest of this phase.** A fourth read is a second trusted-peer check whatever it is named.
- **For plan 10-08 (`DEPLOY.md`):** the shared-secret header is named `X-Holzcloud-Proxy-Secret` — this installation's name, not authentik's — and the example Caddyfile must add it. It is also stripped before any handler, so it can never appear in a template or a log.
- **For plan 10-10 (the browser pass):** a non-loopback IPv6 address exists on this machine, so the fallback finding the plan asks about does not arise. The criterion-2 command is recorded above in a form that can be re-run; note the procedural rule that came out of it — prove which process answered before believing the answer.
- **Nothing in `internal/admin/handler.go` was touched.** `NewWebsiteAccessLookup` is unchanged, cited by name and never by line number, for the reason wave 1 recorded.

## Self-Check: PASSED

- `internal/web/forwardauth.go` — FOUND, contains `ConstantTimeCompare`
- `internal/web/forwardauth_test.go` — FOUND, contains `X_authentik_email`
- `cmd/holzcloud/main.go` — FOUND, contains `web.ForwardAuth`
- `cmd/holzcloud/main_test.go` — FOUND, contains `ForwardAuth`
- Commit `fb8daab` — FOUND
- Commit `b5a4285` — FOUND
- Commit `56607a0` — FOUND
- `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` — all clean at the time of writing

---
*Phase: 10-authentik*
*Completed: 2026-09-08*
