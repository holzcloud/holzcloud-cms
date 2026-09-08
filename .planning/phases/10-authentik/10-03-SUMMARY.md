---
phase: 10-authentik
plan: 03
subsystem: auth
tags: [forward-auth, authentik, sso, session-fixation, renewtoken, collate-nocase, activity-log, middleware]

requires:
  - phase: 10-authentik
    provides: "plan 10-01's config.SSOEnabled, which cannot be true without a secret, so the middleware's first condition needs no nil check beyond the handler's own cfg"
  - phase: 10-authentik
    provides: "plan 10-02's web.IdentityFromContext and the unconditional header strip — the identity arrives in the context because there is no header left to read"
provides:
  - "admin.(*Handler).ForwardAuthSignIn: the fourth caller of the sign-in funnel, and the only place a proxy's claim becomes a session"
  - "auth.SessionKeyViaSSO: the session records HOW it was established, so plan 10-06's second-factor decision has one input rather than a re-derivation"
  - "The ASCII address rule: an address carrying a byte above 0x7F is refused with a named reason rather than matched"
  - "One insertion between setupGuard and requireAuth, so every authorisation middleware behind it runs bit-for-bit unchanged"
  - "Two source-order gates: RenewToken before any session write, and completeLogin at exactly four call sites"
affects: [10-04-provisioning, 10-05-group-rights, 10-06-second-factor, 10-07-admin-surface, 10-08-deploy-docs, 10-10-browser-pass]

actuals:
  tokens: 10897
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "A property that no observable answer distinguishes is asserted against the source with go/ast, not against behaviour — the second use of wave 2's precedent, and this time it caught a live gap"
    - "A refusal is a fall-through: the middleware writes no status of its own on any path and calls the next handler exactly once, so the password form survives the proxy"
    - "The identity and the account key are two different things (X-authentik-username vs users.email), and the source says why rather than the plan"
    - "A guard whose removal no test catches is either dead or under-tested — the empty-address branch was under-tested, and the test that closes it seeds the row the schema permits"

key-files:
  created:
    - internal/admin/forwardauth.go
    - internal/admin/forwardauth_test.go
  modified:
    - internal/auth/session.go
    - cmd/holzcloud/main.go
    - cmd/holzcloud/main_test.go

key-decisions:
  - "refuseSSO takes the normalised address as a fourth argument rather than the plan's three, because the attempted address that goes into the protocol row is the lowercased and trimmed one, and recomputing it inside the helper would put the address rule in two places"
  - "A failed account lookup writes no protocol row, only slog.Error — the store that would write it sits on the database that just failed to answer. The plan's action says slog + next for this branch, and its behaviour list never asks for a row here."
  - "The empty-address refusal was given its own test after mutation testing showed its removal was invisible: users.email permits the empty string, so without the guard an identity with no e-mail header is matched against such a row rather than refused."
  - "The RenewToken ordering is proved by a go/ast test, because scs preserves session values across a renew — a middleware that rotated the token after writing user_id answers identically to one that rotated before, and all ten behavioural tests stay green under that reordering."

patterns-established:
  - "Cite functions, never line numbers: the roadmap's :968 for the admin chain became :1036 in the plan and :1118 today. Everything this plan wrote names symbols."
  - "Commit the task before mutating it — carried from wave 1, applied to all fourteen mutations here."

requirements-completed: [SSO-01, SSO-09]

coverage:
  - id: D1
    description: "A person the identity provider already signed in reaches the admin through the same funnel a password reaches it through, with the session naming the account rather than the header"
    requirement: "SSO-01"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthSignsInAnExistingAccount"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthUsesTheStoredSpellingOfTheAddress"
        status: pass
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestForwardAuthSignsInThroughTheOrdinaryChain — GET /admin/ answers 200 through the real router"
        status: pass
      - kind: other
        ref: "mutation 9 — the header's address stored instead of the account's: red"
        status: pass
    human_judgment: false
  - id: D2
    description: "completeLogin has exactly four callers, so an SSO sign-in appears in /admin/protokoll and moves the last-login stamp because the same function writes both"
    requirement: "SSO-01"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestCompleteLoginHasExactlyFourCallers — go/ast over every non-test file in package admin"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthWritesTheSameActivityRowAPasswordSignInWrites"
        status: pass
      - kind: other
        ref: "grep -rn completeLogin internal/ --include='*.go' | grep -v _test | grep -vc 'func (h \\*Handler) completeLogin\\|// completeLogin' == 4"
        status: pass
      - kind: other
        ref: "mutation 8 — the funnel bypassed with three sm.Put calls: 0 protocol rows written, and the AST gate reports 3 call sites. Both red."
        status: pass
    human_judgment: false
  - id: D3
    description: "The session token is rotated before anything is put in the session, so an attacker who fixed the session id before the sign-in does not own it after"
    requirement: "SSO-01"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthRotatesTheSessionToken — the Set-Cookie value before and after are compared, not the call"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthRotatesTheTokenBeforeItWritesAnything — go/ast source order"
        status: pass
      - kind: other
        ref: "mutation 3 (RenewToken removed) and mutation 4 (RenewToken moved after the writes): both red; mutation 4 leaves every behavioural test green, which is the whole argument for the gate"
        status: pass
    human_judgment: false
  - id: D4
    description: "An address carrying any byte above 0x7F is refused with a named reason rather than matched, because SQLite's COLLATE NOCASE folds ASCII only and a Go-side match would find a row the database considers distinct"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthRefusesAndFallsThrough/an_address_carrying_a_byte_above_0x7F,_even_though_a_row_would_match_it"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestIsASCII (7 cases)"
        status: pass
      - kind: other
        ref: "mutation 5 — the ASCII rule removed: Müller@example.com signs in the müller@example.com administrator, red"
        status: pass
    human_judgment: false
  - id: D5
    description: "An identity with no matching account falls through to the ordinary password form, writes one auth.login_fail row carrying the attempted address, creates nothing and never answers 403"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthRefusesAndFallsThrough (3 subtests: no account, non-ASCII, no address)"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthNeverMatchesAnEmptyAddress"
        status: pass
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestUntrustedPeerWithIdentityHeaderGetsTheLoginForm/a_trusted_peer_with_the_right_secret,_whose_identity_names_no_account — 303 to /admin/login"
        status: pass
      - kind: other
        ref: "sed the function body | grep -c 'http.Error|StatusForbidden|http.Redirect' == 0; mutation 10 (a refusal answers 403) red and the gate flips to 1"
        status: pass
    human_judgment: false
  - id: D6
    description: "A request that already carries a signed-in session passes straight through: the middleware signs in, it does not re-sign-in, and it never overwrites a session reached by password"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthNeverOverwritesASignedInSession"
        status: pass
      - kind: other
        ref: "mutation 2 — the short circuit removed: the password session is replaced by the proxy's identity and a further protocol row is written, red"
        status: pass
    human_judgment: false
  - id: D7
    description: "With HOLZCLOUD_SSO_ENABLED false no line of the middleware body executes past its first condition, and the password path is provably what it was"
    requirement: "SSO-09"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthDoesNothingWhileSwitchedOff — an identity really is in the context and is ignored entirely"
        status: pass
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestPasswordPathIsUnchangedWithSSOOff — same 303, same /admin/, same session keys, and no via_sso"
        status: pass
      - kind: other
        ref: "mutation 1 — the master switch removed: sign-in, protocol row, via_sso and last-login stamp all appear, red"
        status: pass
    human_judgment: false
  - id: D8
    description: "The middleware sits between setupGuard and requireAuth and nowhere else; the public admin chain is untouched, so /admin/login can sign nobody in"
    requirement: "SSO-09"
    verification:
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestForwardAuthDoesNotTouchTheLoginPage"
        status: pass
      - kind: other
        ref: "the nesting gate reads the nesting itself: sed the mux.Handle line | grep -o 'setupGuard(adminHandler.ForwardAuthSignIn(requireAuth' | wc -l == 1; ForwardAuthSignIn in main.go == 1; forward auth on adminPublic == 0"
        status: pass
      - kind: other
        ref: "mutations 11, 12 and 14 — removed, also hung on the public chain, and moved inside requireAuth: all three red, and the nesting gate flips on 11 and 14"
        status: pass
    human_judgment: false
  - id: D9
    description: "Seven named things are unchanged, each measured rather than assumed"
    requirement: "SSO-09"
    verification:
      - kind: other
        ref: "git diff --stat 5f8fb4e -- internal/auth/middleware.go internal/auth/twofactor.go internal/admin/login.go internal/user/rights.go internal/db/migrations/ — empty; the four middleware declarations and Rights/SetRights verified present by name"
        status: pass
    human_judgment: false
  - id: D10
    description: "Nothing this plan does is read by a person on a screen: no new route, no new template, no new string"
    verification:
      - kind: other
        ref: "go run ./tools/i18n: 1318 Zeichenketten im Quelltext before and after, 41 offen / 0 verwaist on en/es/fr/it before and after — delta 0"
        status: pass
      - kind: other
        ref: "adminProtectedMux.Handle 159 → 159; adminOnly rows 19 → 19; admin templates 68 → 68; layoutPageNames 50 → 50; migrations 51 → 51"
        status: pass
    human_judgment: false
  - id: D11
    description: "Whether an operator's real Authentik emits X-authentik-username with the address that matches a users.email row, and whether refusing a non-ASCII address is the behaviour they want rather than a lockout"
    verification: []
    human_judgment: true
    rationale: "Both depend on the operator's own directory, which this repository cannot observe. An installation whose accounts are keyed on addresses carrying umlauts will find every such person refused and sent to the password form — correct by this plan's design, and a surprise unless DEPLOY.md says so. Plan 10-08 owns writing it down; plan 10-10's browser pass is where a human sees the refusal path end at the login form rather than at a 403."

duration: 23 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 03: The Fourth Caller Summary

**One middleware that turns the proxy's identity into a session by calling the same unexported funnel the password form calls — so the sign-in appears in `/admin/protokoll` because the same function writes the row — with the session token rotated before anything is written, an address the database cannot promise is one account refused rather than guessed, and one insertion between `setupGuard` and `requireAuth` that leaves every authorisation middleware behind it bit-for-bit unchanged.**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-08T03:54:19Z
- **Completed:** 2026-09-08T04:17:12Z
- **Tasks:** 3
- **Files:** 2 created, 3 modified

## Accomplishments

- **`ForwardAuthSignIn` is the fourth caller and adds nothing of its own.** The body is seven steps in an order the source says is the design: the switch, the identity from the context, the already-signed-in short circuit, the address rule, the lookup that creates nothing, `RenewToken`, then the funnel. It writes no status on any path and calls the next handler exactly once.
- **The ordering gap wave 2 warned about was live here, and was found by mutation testing rather than by reading.** `scs` preserves session values across a renew, so moving `RenewToken` *after* the session writes answers every request identically — same status, same session, and a token that changed either way. **All ten behavioural tests stayed green under that reordering.** The property is a source-order property, so a `go/ast` test now asserts it, and the reordering is red.
- **The empty-address guard turned out to be load-bearing for a reason the plan did not name.** `users.email` is `NOT NULL UNIQUE COLLATE NOCASE` and nothing there forbids `''`, so an account row with an empty address is legal. Without the guard an identity carrying no e-mail header is not refused but *looked up*, and it matches that row. Proved by seeding one as an administrator.
- **The ASCII rule was demonstrated, not argued.** Mutation 5 removes it and `Müller@example.com` signs in the `müller@example.com` **administrator** — a row SQLite considers a different account from the one Go's `ToLower` found.
- **The seven that must not move are each empty**, measured against the wave-3 starting commit and verified by symbol name as well as by diff.
- **Fourteen mutations, fourteen reds**, each against a **committed** file, restored with `git checkout --` and the suite re-run green. Verbatim output below.

## Task Commits

1. **Task 1 (RED): failing tests for the fourth caller** — `4b0e944` (test)
2. **Task 1 (GREEN): the fourth caller of the sign-in funnel** — `7899c58` (feat)
3. **Task 2: one line in the hand-nested admin chain** — `73e98fb` (feat)
4. **Task 1 (follow-up): the token rotation and the four callers, read from the source** — `5bae5b0` (test)
5. **Task 1 (follow-up): an empty address is refused, never looked up** — `57d10d1` (test)
6. **Task 3: the seven and the counting gates** — no code commit by design. Its own action says "No new code beyond a comment", and the comment it points at (the D-02 paragraph on the address rule) was written in Task 1. Its output is the two tables below.

_Task 1 carried `tdd="true"`. The RED commit's tests do not compile against the pre-change tree — `undefined: auth.SessionKeyViaSSO`, `h.ForwardAuthSignIn undefined`, `undefined: isASCII` — which is the intended failure._

Commits 4 and 5 are follow-ups to Task 1 rather than new tasks: both were forced by mutation testing the committed Task 1, and both add only tests. Committing them separately rather than amending keeps the reason each exists in the history.

## Files Created/Modified

- `internal/admin/forwardauth.go` (new, 203 lines) — the file-level note on what this file decides that `internal/web` does not, the three refusal reason codes, `ForwardAuthSignIn` with the seven-step body, `refuseSSO`, and `isASCII` with the D-02 paragraph
- `internal/admin/forwardauth_test.go` (new, 619 lines) — 13 test functions, 3 named subtests, the `web.ForwardAuth`-driven harness, and the two `go/ast` gates
- `internal/auth/session.go` — `SessionKeyViaSSO` beside the five keys, with its comment naming both readers
- `cmd/holzcloud/main.go` — one insertion in the protected chain and the extended comment above it
- `cmd/holzcloud/main_test.go` — `ssoRouter`, `forwardAuthRequest`, `insertUser`, the three named tests, and the corrected reason on wave 2's control subtest

## Task 3, part one: the seven that must not move

**Diff range: `5f8fb4e..HEAD`** — the wave-3 starting commit, which is `docs(10-02)`, the last commit of wave 2.

The plan says to diff against **the phase's** starting commit (`cdcfbab`). Both ranges are reported, because they disagree and the disagreement is not mine:

| Must not change | Command | Against `5f8fb4e` (wave 3) | Against `cdcfbab` (phase 10) |
|---|---|---|---|
| `auth.RequireAuth` | `git diff --stat -- internal/auth/middleware.go` | **empty** | empty |
| `auth.RequireAdmin` | same file | **empty** | empty |
| `auth.RequireWebsiteAccess` | same file | **empty** | empty |
| `auth.RequireSecondFactor` | `git diff --stat -- internal/auth/twofactor.go` | **empty** | empty |
| `completeLogin`'s body | `git diff -- internal/admin/login.go` | **empty** | empty |
| `user.Rights` / `SetRights` | `git diff --stat -- internal/user/rights.go` | **empty** | empty |
| the `users` schema | `git diff -- internal/db/migrations/` | **empty** | `00051_album_updated_at.sql, +69` |
| migration **count** | `ls internal/db/migrations/*.sql \| wc -l` | 51 → **51** | 50 → 51 |

**The one non-empty cell is not a finding against this design.** `00051_album_updated_at.sql` was added by `2e02be3 fix(11): CR-01 a warm browser no longer keeps the old gallery for ever` — the concurrent Phase 11 fix round, attributed by `git log --diff-filter=A`. `00001_initial.sql`, which is where the `users` schema lives, is byte-identical against both ranges. **Recorded as a defect of the range, not of the code:** the phase-start range cannot answer this question while another phase commits into the same tree, and the wave-start range can.

Verified by **name** as well as by diff, because a file diff would not catch a symbol that moved out of it:

```
internal/auth/middleware.go:33  func RequireAuth(sm *scs.SessionManager, lookup UserLookup) Middleware
internal/auth/middleware.go:67  func RequireAdmin(sm *scs.SessionManager) Middleware
internal/auth/middleware.go:99  func RequireWebsiteAccess(sm *scs.SessionManager, allowed WebsiteAccess) Middleware
internal/auth/twofactor.go:44   func MustHaveSecondFactor(role string) bool
internal/auth/twofactor.go:50   func RequireSecondFactor(sm *scs.SessionManager, lookup SecondFactorLookup) Middleware
internal/user/rights.go:59      func (s *Store) Rights(...)
internal/user/rights.go:93      func (s *Store) SetRights(...)
```

`internal/admin/handler.go` was **not touched** — `git diff --stat 5f8fb4e -- internal/admin/handler.go` is empty. `NewWebsiteAccessLookup` is cited by name and never by line number anywhere this plan wrote, for the reason waves 1 and 2 recorded. (For the record and not as a citation: `return assigned == 0 || mine > 0` is at `:183` today, which is where wave 1 measured it.)

## Task 3, part two: the counting gates, measured against the post-change tree

**Baseline measured at 2026-09-08T03:54:38Z, immediately before the first commit of this plan, on this tree — not taken from the plan.** The plan's note above its own table says these rows are deltas; three of its literal numbers had moved again since it was written, and the Phase 11 fix round committed three more times (`4e462b1`, `4c32f5c`, `a4af157`) while this plan ran.

| What | Command | Plan's number | Measured baseline | This plan adds | Measured after | Verdict |
|---|---|---|---|---|---|---|
| `completeLogin` call sites | `grep -rn completeLogin internal/ --include='*.go' \| grep -v _test \| grep -vc '…'` | 3 → 4 | **3** | 1 | **4** | exactly as predicted |
| session keys in `session.go` | `grep -c 'SessionKey[A-Za-z]* *=' internal/auth/session.go` | 5 → 6 | **5** | 1 | **6** | exactly as predicted |
| files in `internal/admin` | `ls internal/admin/*.go \| grep -v _test \| wc -l` | vorher + 1 | **51** | 1 | **52** | exactly as predicted |
| `adminProtectedMux.Handle` | `grep -c 'adminProtectedMux.Handle' cmd/holzcloud/main.go` | vorher (gate says 150) | **159** | 0 | **159** | delta 0; the literal 150 is stale |
| `adminOnly` rows | the table in `main_test.go` | 19 | **19** | 0 | **19** | exactly as predicted |
| admin templates | `ls cmd/holzcloud/templates/admin/*.html \| wc -l` | vorher | **68** | 0 | **68** | delta 0 as required |
| `layoutPageNames` entries | the slice in `internal/web/render.go` | 50 | **50** | 0 | **50** | exactly as predicted |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | vorher (gate says 49) | **51** | 0 | **51** | delta 0; the literal 49 is stale twice over |
| files using `RemoteAddr` | `grep -rn RemoteAddr internal/ cmd/ --include='*.go' \| grep -v _test \| wc -l` | 3 | **3** | 0 | **3** | exactly as predicted |
| `IsTrustedPeer` mentions | same shape | 4 (wave 2) | **4** | 0 | **4** | delta 0 as required |
| packages under `internal/` | `ls -d internal/*/ \| wc -l` | 41 (wave 2) | **41** | 0 | **41** | delta 0 as required |
| strings in source | `go run ./tools/i18n \| head -1` | vorher | **1318** | 0 | **1318** | delta 0 as required |

### `go run ./tools/i18n`, before and after

```
before:  1318 Zeichenketten im Quelltext
         en.json  1277 übersetzt, 41 offen, 0 verwaist   (es, fr, it identical)
after:   1318 Zeichenketten im Quelltext
         en.json  1277 übersetzt, 41 offen, 0 verwaist   (es, fr, it identical)
delta:   0 strings, 0 newly open, 0 orphaned
```

The 41 open strings are pre-existing and belong to plan 11-07. Waves 1 and 2 recorded 34; the Phase 11 fix round raised it to 41 before this plan started, exactly as the executor prompt predicted. **This plan neither added one nor closed one.** Everything it does is invisible: a person who signs in through the identity provider sees the same dashboard, and a person who is refused sees the same login form with no new sentence on it. The three refusal reason codes are `slog` field values, never rendered — and `cmd/` is outside the collector's roots in any case, while `internal/admin/forwardauth.go` is inside them, so a translated helper there would have shown up.

### The four rows that stay flat, and why each matters

- **No new route.** `adminProtectedMux.Handle` 159 → 159 and `TestRouteAuthorization`'s `adminOnly` table 19 → 19. Forward auth adds no path to the router; it is a layer on the path that already existed. Had either moved, a route would exist that the table's own comment says must be in it.
- **No new template**, 68 → 68, and no new `layoutPageNames` entry, 50 → 50.
- **No new migration**, 51 → 51. D-05 holds and the `users` schema was not reached for — which is exactly why the account key had to be `users.email` and why the ASCII rule exists at all.
- **`RemoteAddr` still 3 and `IsTrustedPeer` still 4.** This plan reads neither. `refuseSSO` gets the client address through `h.clientIP.ClientIP(r)`, which is the resolver, not a second peer check. Wave 2's rule — a fourth `RemoteAddr` read is a second trusted-peer check whatever it is named — is intact.

## Decisions Made

- **`refuseSSO` takes four arguments, not the plan's three.** The address that goes into the protocol row is the lowercased, trimmed one, and it has already been computed at every call site. Passing it keeps the address rule in one place; recomputing it inside the helper would put `strings.ToLower(strings.TrimSpace(...))` in two.
- **A failed account lookup writes no protocol row.** The plan's action for that branch says `slog.Error` + `next`, and its behaviour list never asks for a row there. The reason is worth stating: the store that would write the row sits on the database that just failed to answer, so the write is likely to fail too and the second failure would be the one in the log.
- **The `RenewToken` ordering is an AST test.** See below; this was not a stylistic choice but the only kind of test that fails.
- **The empty-address test seeds an empty-address account.** Anything weaker cannot distinguish the guard from the "no such account" branch that follows it.
- **`main_test.go`'s wave-2 control subtest keeps its verdict and gets a new reason.** It was named `a trusted peer with the right secret, because nothing signs anybody in yet`; that sentence is now false, though the assertion still holds. It reads `whose identity names no account` and the comment says which half of SSO-02 each of the two tests proves. A comment that has quietly become untrue is worse than a failing test, because nothing reports it.

## Deviations from Plan

### Auto-fixed

**1. [Rule 2 — Missing critical] The `RenewToken`-before-anything ordering had no test that could fail**

- **Found during:** Task 1 mutation testing, against the committed file
- **Issue:** `scs`'s `RenewToken` preserves the values already in the session. A middleware that rotates the token *after* writing `user_id` therefore produces the same status, the same session contents **and a changed token** — every one of the ten behavioural tests stays green. Probed before fixing: the reordered file built clean and `go test ./internal/admin/ -run ForwardAuth` answered `ok`. The plan's `verify` block gates this with a `sed | grep -n` on line numbers, which is a gate on the file rather than a test, and the executor prompt requires "a test that would fail if the call moved".
- **Fix:** `TestForwardAuthRotatesTheTokenBeforeItWritesAnything` parses `forwardauth.go` with `go/parser`, walks `ForwardAuthSignIn`, and asserts that every `sm.Put` and the `completeLogin` call have a source position greater than the single `RenewToken` call. Same shape as wave 2's operand-order gate on `internal/web`.
- **Files modified:** `internal/admin/forwardauth_test.go`
- **Verification:** mutation 4 — red on the AST gate, green on every behavioural test, which is precisely the gap it closes.
- **Committed in:** `5bae5b0`

**2. [Rule 2 — Missing critical] The four-caller claim had no test, only a grep**

- **Found during:** Task 1 gate run
- **Issue:** The plan asserts the four callers with a shell pipeline in its `verify` block. A pipeline in a plan document runs once, when somebody remembers to run it; the claim is a standing invariant of this package for the rest of the phase.
- **Fix:** `TestCompleteLoginHasExactlyFourCallers` walks every non-test `.go` file in `internal/admin` with `go/ast` and counts `CallExpr` selectors named `completeLogin`, printing every call site when the count is wrong.
- **Files modified:** `internal/admin/forwardauth_test.go`
- **Verification:** mutation 8 — the funnel bypassed with three `sm.Put` calls; the gate reports `3 call sites; want exactly 4` and names all three, while `TestForwardAuthWritesTheSameActivityRowAPasswordSignInWrites` reports `wrote 0 protocol rows`. The two together are the phase's central claim, red.
- **Committed in:** `5bae5b0`

**3. [Rule 2 — Missing critical] The empty-address refusal had no test that could fail**

- **Found during:** Task 1 mutation testing (mutation 6, which stayed green)
- **Issue:** With `if email == ""` removed, the empty address falls through `isASCII("")` (true) into `GetByEmail(ctx, "")`, which finds nothing and refuses through the no-account branch — same status, same one `auth.login_fail` row, same empty `actor_email`. The guard looked like tidiness. It is not: `users.email` is `TEXT NOT NULL UNIQUE COLLATE NOCASE`, which permits `''`, verified by inserting one. Without the guard, an identity carrying no e-mail header is **matched against that row**.
- **Fix:** `TestForwardAuthNeverMatchesAnEmptyAddress` seeds an account with an empty address **and the admin role**, then asserts that an identity with no address signs in as nobody.
- **Files modified:** `internal/admin/forwardauth_test.go`
- **Verification:** mutation 6b, verbatim below.
- **Committed in:** `57d10d1`

### Divergences recorded rather than worked around

**4. `refuseSSO` has a fourth parameter.** The plan's signature is `refuseSSO(r, ident, reason)`; the implementation is `refuseSSO(r, ident, email, reason)`. Reason under "Decisions Made".

**5. Task 3 produced no code commit,** as in waves 1 and 2 and for the same reason: its own action says "No new code beyond a comment", and the comment it points at was written in Task 1.

**6. The plan's admin-chain citation drifted again, for the third time.** `ROADMAP.md` says `main.go:968`; the plan's `flagged_assumptions` corrects it to `:1036` and says so explicitly; the line is at **`:1118`** on this tree. The chain is otherwise exactly as described — hand-nested, not built with `auth.Chain`, and the described insertion point is right. **Nothing this plan wrote cites a line number.** The nesting gate reads the nesting itself (`setupGuard(adminHandler.ForwardAuthSignIn(requireAuth`), which is why it survived the drift.

**7. Two of the plan's literal `verify` gates are stale against this tree**, in the family waves 1 and 2 both recorded:
- `ls internal/db/migrations/*.sql | wc -l` is specified to fail unless it prints **49**; the tree has **51**, stale twice over from Phase 11's migration and its fix round. Delta 0.
- `grep -c 'adminProtectedMux.Handle' cmd/holzcloud/main.go` is specified to fail unless it prints **150**; the tree has **159**, which is what wave 2 measured. Delta 0.

Both fail as literally written and would fail identically on an empty commit. Re-anchored to the deltas, per the plan's own note and the two waves' precedent.

**8. The "diff against the phase's starting commit" range does not survive a concurrent phase.** Against `cdcfbab` the migrations directory shows `00051_album_updated_at.sql`, added by Phase 11's `2e02be3`. Against the wave-3 start `5f8fb4e` it is empty. Both are reported above rather than one being chosen quietly.

---

**Total deviations:** 3 auto-fixed (all Rule 2, all missing tests for guards the plan itself specifies) + 5 divergences recorded.
**Impact on plan:** No scope creep — the three auto-fixes add tests only, no production behaviour. All three exist because mutation testing showed the plan's own must-have truths were not provable as written: one was provable only by reading, one only by a shell pipeline, and one not at all.

## Mutation Verification

Every guard was removed, inverted or moved on a **committed** file, run against a named test, restored with `git checkout --`, and the suite re-run green. **Fourteen mutations, fourteen reds. Verbatim output.**

**1. The master switch removed (`!h.cfg.SSOEnabled` → `false`)**
```
--- FAIL: TestForwardAuthDoesNothingWhileSwitchedOff (0.56s)
    forwardauth_test.go:413: single sign-on is off and somebody was signed in anyway: user_id = 1
    forwardauth_test.go:416: single sign-on is off and via_sso was written anyway
    forwardauth_test.go:419: single sign-on is off and 1 protocol rows were written; want 0
    forwardauth_test.go:425: single sign-on is off and the account was still looked up and stamped
```

**2. The already-signed-in short circuit removed**
```
--- FAIL: TestForwardAuthNeverOverwritesASignedInSession (0.56s)
    forwardauth_test.go:319: the signed-in session was replaced: user_id = 2, want 1 (2 is the identity the proxy asserted)
    forwardauth_test.go:323: session user_email = "grace@example.com"; the existing session must be left exactly as it is
    forwardauth_test.go:326: a session reached by password was marked as established through single sign-on
    forwardauth_test.go:329: wrote 1 further protocol rows for an already signed-in request; one row per page view is what makes the protocol useless
```

**3. `RenewToken` removed entirely — the session-fixation mutation**
```
--- FAIL: TestForwardAuthRotatesTheSessionToken (0.56s)
    forwardauth_test.go:243: the session token did not change across the sign-in ("xNFXCA0-OCYZ0GsLf1wTZKC1Yn3KjQWRJKmLxutzN8o"); whoever fixed the session id before the sign-in still owns the session after it
```

**4. `RenewToken` moved *after* the session writes — the ordering mutation**
```
--- FAIL: TestForwardAuthRotatesTheTokenBeforeItWritesAnything (0.00s)
    forwardauth_test.go:542: Put is called at forwardauth.go:147:8, before RenewToken at forwardauth.go:150:18; the token must be rotated before anything goes into the session, or whoever fixed the session id before the sign-in still owns it after
    forwardauth_test.go:542: completeLogin is called at forwardauth.go:148:5, before RenewToken at forwardauth.go:150:18; the token must be rotated before anything goes into the session, or whoever fixed the session id before the sign-in still owns it after
```
**Under this mutation every behavioural test stayed green:**
```
ok  	github.com/holzcloud/holzcloud-cms/internal/admin	5.729s
```
That is the whole argument for the gate, and it is the same argument wave 2 made about the peer check — this time verified against a tree where the plan's own must-have truth would otherwise have been unprovable.

**5. The ASCII rule removed (`!isASCII(email)` → `false && …`) — the D-02 mutation**
```
--- FAIL: TestForwardAuthRefusesAndFallsThrough (1.62s)
    --- FAIL: TestForwardAuthRefusesAndFallsThrough/an_address_carrying_a_byte_above_0x7F,_even_though_a_row_would_match_it (0.52s)
        forwardauth_test.go:386: a refused identity was signed in as 1
        forwardauth_test.go:397: action = "auth.login_success"; want "auth.login_fail"
```
The seeded account is `müller@example.com` with role `admin`. With the rule gone, the header `Müller@example.com` signs in that administrator — an account SQLite's `COLLATE NOCASE` considers distinct from the address that was asked for.

**6. The empty-address refusal removed — the mutation that stayed GREEN, and what was done about it**
```
(no output — TestForwardAuthRefusesAndFallsThrough passed under the mutation)
```
Investigated rather than accepted. `users.email` permits `''`, verified by inserting a row. Test added; see deviation 3.

**6b. The same mutation against the new test**
```
--- FAIL: TestForwardAuthNeverMatchesAnEmptyAddress (0.57s)
    forwardauth_test.go:612: an identity with no address signed in as user 1 (role "admin"); the empty string is a legal value in users.email and must never be looked up
    forwardauth_test.go:617: an identity with no address was marked as signed in through single sign-on
```

**7. A refusal writes no protocol row**
```
--- FAIL: TestForwardAuthRefusesAndFallsThrough (1.63s)
    --- FAIL: …/an_address_no_account_carries (0.55s)
        forwardauth_test.go:394: wrote 0 protocol rows; want exactly 1 refusal
    --- FAIL: …/an_address_carrying_a_byte_above_0x7F,_even_though_a_row_would_match_it (0.53s)
        forwardauth_test.go:394: wrote 0 protocol rows; want exactly 1 refusal
    --- FAIL: …/an_identity_with_no_address_at_all (0.54s)
        forwardauth_test.go:394: wrote 0 protocol rows; want exactly 1 refusal
```

**8. The funnel bypassed — `user_id`, `user_role` and `user_email` written directly**
```
--- FAIL: TestForwardAuthWritesTheSameActivityRowAPasswordSignInWrites (0.58s)
    forwardauth_test.go:268: wrote 0 protocol rows; want exactly 1
--- FAIL: TestCompleteLoginHasExactlyFourCallers (0.01s)
    forwardauth_test.go:584: completeLogin has 3 call sites; want exactly 4
      login.go:97:4
      setup.go:121:4
      twofactor.go:118:4
```
```
and the plan's own grep gate (want 4): 3
```
**This is the mutation this whole plan exists to prevent.** The middleware works perfectly under it — the person is signed in, the dashboard renders — and the sign-in is invisible in `/admin/protokoll`.

**9. The header's address stored instead of the account's**
```
--- FAIL: TestForwardAuthUsesTheStoredSpellingOfTheAddress (0.58s)
    forwardauth_test.go:232: session user_email = "ADA@Example.COM"; the header only claimed an address, the database names the account
```

**10. A refusal answers 403 instead of falling through**
```
--- FAIL: TestForwardAuthRefusesAndFallsThrough/an_address_no_account_carries (0.57s)
    forwardauth_test.go:379: the next handler ran 0 times; want exactly 1 — a refusal is a fall-through
```
```
and the plan's status gate (want 0): 1
```

**11. The middleware removed from the protected chain**
```
--- FAIL: TestForwardAuthSignsInThroughTheOrdinaryChain (0.49s)
    main_test.go:937: GET /admin/ with an identity for a seeded editor answered 303; want 200 — a person the identity provider already signed in must not be asked again

gate 'setupGuard(adminHandler.ForwardAuthSignIn(requireAuth' (want 1): 0   <- red
```

**12. The middleware also hung on the public admin chain**
```
--- FAIL: TestForwardAuthDoesNotTouchTheLoginPage (0.47s)
    main_test.go:979: /admin/login signed in user 1; the public admin chain has no forward-auth layer

gate 'ForwardAuthSignIn in main.go'   (want 1): 2   <- red
gate 'forward auth on adminPublic'   (want 0): 1   <- red
```

**13. `via_sso` is no longer written**
```
--- FAIL: TestForwardAuthSignsInAnExistingAccount (0.58s)
    forwardauth_test.go:218: via_sso is false after a forward-auth sign-in; plan 10-06 has no input to read
```

**14. The middleware moved *inside* `requireAuth`**
```
--- FAIL: TestForwardAuthSignsInThroughTheOrdinaryChain (0.48s)
    main_test.go:937: GET /admin/ with an identity for a seeded editor answered 303; want 200 — a person the identity provider already signed in must not be asked again

gate 'setupGuard(adminHandler.ForwardAuthSignIn(requireAuth' (want 1): 0   <- red
```
It compiles and it is one nesting level away from correct: `RequireAuth` reads the session before forward auth has written it, so the person is redirected to the login form they should never have seen. This is the position argument as a test.

After every mutation the file was restored with `git checkout --`, `git status --short` confirmed clean, and the suite re-run green.

## Verification Results

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | silent |
| `gofmt -l .` | silent |
| `go test ./...` | **0 failures** |
| `go test ./internal/admin/ -run ForwardAuth -v` | 10 test functions + 3 named subtests, all PASS |
| `go test ./cmd/holzcloud/ -run 'ForwardAuth\|PasswordPath' -v` | 3 named tests, all PASS |
| `grep -rn completeLogin … \| grep -v _test \| grep -vc …` | **4** ✓ |
| `grep -c 'SessionKey[A-Za-z]* *=' internal/auth/session.go` | **6** ✓ |
| `sed` the function body `\| grep -n 'RenewToken\|completeLogin'` | `79: RenewToken`, `89: completeLogin` — rotation first ✓ |
| `sed` the function body `\| grep -c 'http.Error\|StatusForbidden\|http.Redirect'` | **0** ✓ |
| nesting gate `setupGuard(adminHandler.ForwardAuthSignIn(requireAuth` | **1** ✓ |
| `grep -c ForwardAuthSignIn cmd/holzcloud/main.go` | **1** ✓ |
| `sed -n '/adminPublic := /p' \| grep -c ForwardAuth` | **0** ✓ |
| the seven, `git diff --stat 5f8fb4e` | all **empty** ✓ |
| `go run ./tools/i18n \| head -1` | `1318 Zeichenketten im Quelltext` before **and** after; `41 offen, 0 verwaist` on en/es/fr/it, unchanged ✓ |
| `ls internal/db/migrations/*.sql \| wc -l` | 51 — the plan's literal gate expects 49 and is stale twice over; **delta 0**, which is the assertion that matters |
| `grep -c adminProtectedMux.Handle cmd/holzcloud/main.go` | 159 — the plan's literal gate expects 150 and is stale; **delta 0** |

## Issues Encountered

- **`go test ./...` failed twice with failures that were not mine, in two different packages, and passed on re-run.** The first run reported 14 failures across `internal/admin` (album scope tests) and `internal/album`; running the named tests alone passed. The second run reported 3 failures in `internal/block`, which I have never touched, and the same command re-run in the same breath reported 0. The cause is the concurrent Phase 11 agent committing into this tree: `4e462b1`, `4c32f5c` and `a4af157` all landed between my Task 2 commit and my final run. **A suite result taken while another agent is mid-commit is not evidence** — this is wave 2's "prove which process answered" lesson in a different costume. The run recorded above is from a tree whose `git status --short` was clean and whose `git log` did not move during it.
- **Every commit was staged file by file and verified with `git show --stat`.** All five touch only this plan's five files; the per-commit file list is above.
- **The empty-address guard was one investigation away from being recorded as unfalsifiable.** The honest first observation was "mutation 6 stayed green". Writing that down and stopping would have been defensible and wrong: the guard is load-bearing, and finding out why took one `INSERT`.

## Threat Flags

None new. Every `mitigate` disposition in the plan's register has a passing test and a red mutation:

| Threat | Covered by |
|---|---|
| T-10-15 session fixation across the sign-in | mutations 3 and 4; `TestForwardAuthRotatesTheSessionToken` (the cookie, not the call) + the AST ordering gate |
| T-10-16 a case-folding mismatch matching the wrong account | mutation 5 — an administrator signed in through a row the database considers distinct |
| T-10-17 an SSO sign-in invisible in the activity log | mutation 8 — the funnel bypassed, 0 rows written, call sites at 3 |
| T-10-18 a refused attempt leaving no trace | mutation 7 |
| T-10-19 a refusal answering 403 | mutation 10; the status gate at 0 |
| T-10-20 forward auth feeding the login throttle | `loginThrottle` is not named anywhere in `forwardauth.go`; the reason is in `refuseSSO`'s doc comment, not only in the plan |
| T-10-21 an existing session overwritten by an identity | mutation 2 |
| T-10-SC package-manager installs | no install task; every symbol is Go standard library or already in `go.mod` |

**One surface this plan adds that the register did not name, and it is closed:** `users.email` permits the empty string, so an account row with an empty address is reachable by an identity carrying no e-mail header. Covered by `TestForwardAuthNeverMatchesAnEmptyAddress` and mutation 6b. Worth carrying to plan 10-04: **provisioning must not create an account with an empty address**, or it will create exactly the row this test seeds.

## Known Stubs

None. Everything this plan builds is reachable, wired and tested. Two things have **no consumer yet, by design and by the build order**:

- `auth.SessionKeyViaSSO` is written here and read by nothing. Plan 10-06's `MustHaveSecondFactor` is its first reader and plan 10-07's sign-out its second; the key exists now so that neither has to re-derive "was this an SSO session" from something that only looks equivalent.
- `web.Identity.HasGroup` is still called only from its own test. Plan 10-05 is its first caller, as wave 2 recorded.

**No account is created on any path in this plan.** That is not a stub either — it is the boundary between SSO-02 and SSO-05, and plan 10-04 lands provisioning as a second branch at step 5 of `ForwardAuthSignIn`.

## User Setup Required

None from this plan. Two things it hands to plan 10-08 (`DEPLOY.md`), both of which an operator will otherwise meet as a surprise:

1. **An address carrying any byte above 0x7F cannot sign in through single sign-on** and is sent to the password form. An installation whose accounts are keyed on addresses with umlauts will find those people refused. The reason is that the database's own uniqueness rule cannot promise such an address names one account; the answer is to key those accounts on ASCII addresses, not to widen the rule here.
2. **The identity provider's `X-authentik-username` is the identity, but `users.email` is the account key.** An account exists in this CMS or the person reaches the password form; nothing is created for them in this plan.

## Next Phase Readiness

- **Plan 10-04 (provisioning)** lands as a second branch where `u == nil` is handled today, inside `ForwardAuthSignIn` between the lookup and `RenewToken`. It must not create an account with an empty address — see Threat Flags. `cfg.SSODefaultWebsite` is guaranteed positive and guaranteed to exist in the database whenever `cfg.SSOProvision` is true, per wave 1.
- **Plan 10-05 (group rights)** gets `web.Identity.HasGroup` and `cfg.SSOWebsiteGroups`, both parsed and validated. D-01's second road — an empty `user_websites` write reaching "zero means every website" by subtraction — is still open and is that plan's to close; nothing here writes `user_websites`.
- **Plan 10-06 (the second factor)** reads `auth.SessionKeyViaSSO` and must change exactly one line of `internal/auth/twofactor.go`. That file is empty in this plan's diff, so 10-06's own gate has a clean baseline. Note wave 1's correction: `MustHaveSecondFactor` has **five** call sites, not one.
- **Plan 10-07 (sign-out)** is the second reader of `SessionKeyViaSSO`, and `cfg.SSOSignOutPath` is already validated as a local path.
- **Carry these baselines forward, measured on this tree at 04:17Z:** migrations **51**, packages **41**, admin templates **68**, source strings **1318** with **41 offen**, `adminProtectedMux.Handle` **159**, `adminOnly` rows **19**, `layoutPageNames` **50**, `internal/admin` non-test files **52**, `RemoteAddr` **3**, `IsTrustedPeer` **4**, session keys **6**, `completeLogin` call sites **4**.
- **`completeLogin` must stay at exactly 4 for the rest of this phase.** It is now asserted by a test rather than only by a pipeline, so a fifth funnel fails the suite.
- **Do not cite `cmd/holzcloud/main.go` by line number.** The admin chain has now been cited wrongly in three documents. The nesting gate reads the nesting.
- **For plan 10-10 (the browser pass):** the refusal path is the one to drive by hand. Every automated test asserts it ends at the login form; what no test can assert is that the person understands why they are looking at it, since this plan deliberately mints no message. SSO-07's one on-screen sentence belongs to plan 10-06.

## Self-Check: PASSED

- `internal/admin/forwardauth.go` — FOUND, contains `ForwardAuthSignIn` and `RenewToken`
- `internal/admin/forwardauth_test.go` — FOUND
- `internal/auth/session.go` — FOUND, contains `SessionKeyViaSSO`
- `cmd/holzcloud/main.go` — FOUND, contains `ForwardAuthSignIn` exactly once
- `cmd/holzcloud/main_test.go` — FOUND
- Commit `4b0e944` — FOUND
- Commit `7899c58` — FOUND
- Commit `73e98fb` — FOUND
- Commit `5bae5b0` — FOUND
- Commit `57d10d1` — FOUND
- `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` — all clean at the time of writing

---
*Phase: 10-authentik*
*Completed: 2026-09-08*
