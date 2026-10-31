---
phase: 10-authentik
plan: 06
subsystem: auth
tags: [forward-auth, authentik, sso, two-factor, totp, session, i18n, admin-ui]

requires:
  - phase: 10-authentik
    provides: "plan 10-03's ForwardAuthSignIn, which writes auth.SessionKeyViaSSO in exactly one place — this plan reads it and never re-derives it from a header, because the headers are gone by the time any handler runs"
  - phase: 10-authentik
    provides: "plan 10-01's config.SSOEnabled, the master switch the user-list notice is gated on"
provides:
  - "auth.MustHaveSecondFactor(role string, viaSSO bool): the whole of D-04 in one function, with the arity changed so the compiler enumerates the callers rather than a reader"
  - "auth.RequireSecondFactor reading SessionKeyViaSSO itself — it already holds sm, so the decision needed no new dependency"
  - "admin.(*Handler).viaSSO: one reader per screen in package admin, so the four screen call sites ask the session and not a header"
  - "admin.AccountData.ViaSSO and admin.UserListData.SSOEnabled: the two fields that carry SSO-07's dependency onto a screen instead of into a source comment"
  - "The measured correction to D-04: FIVE call sites, not the one ROADMAP.md and 10-CONTEXT.md both record — with the compiler's own verbatim enumeration"
affects: [10-07-sign-out, 10-08-deploy-docs, 10-09-translation, 10-10-browser-pass]

actuals:
  tokens: 21000
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "Change the arity, do not add a variant: a second predicate is what lets one call site keep the old answer for a year, and four of this plan's five call sites were in no planning document"
    - "A grep gate that counts a symbol cannot see where the symbol sits — the ViaSSO gate prints 1 whether the notice is above the card's two branches or inside one of them, and mutation 13 is what found the hole"
    - "The predicate's old behaviour asserted as a table over both roles, so a later change to the password path breaks a test named after the password path"
    - "A German literal inside {{t}} with English identifiers and comments around it: the sentence is a catalogue key, not an identifier"

key-files:
  created:
    - internal/auth/twofactor_test.go
    - internal/admin/twofactor_sso_test.go
  modified:
    - internal/auth/twofactor.go
    - internal/admin/twofactor.go
    - internal/admin/user.go
    - cmd/holzcloud/templates/admin/account.html
    - cmd/holzcloud/templates/admin/user_list.html

key-decisions:
  - "internal/auth/twofactor_test.go did not exist. The plan says 'extend' it; RequireSecondFactor and MustHaveSecondFactor had no test of any kind before this plan, so the file was created and the middleware's three unchanged early returns were asserted along with the new behaviour."
  - "internal/admin/twofactor_sso_test.go was created although the plan's <files> does not list it. The plan's own behaviour list demands a named test for the :274 disable guard in both directions, and package auth cannot reach a handler."
  - "The RED commit could not contain the four-row predicate table. The table names the new arity, so on the pre-change tree it is a compile error rather than a red test; it went in as its own commit immediately after GREEN. The RED commit holds the behavioural half, which does compile against the old tree and does fail."
  - "The two new sentences are left untranslated, as the plan instructs. `go run ./tools/i18n` reports 2 offen per catalogue and CLAUDE.md's gate asks for 0 — the divergence is deliberate, recorded here with the exact keys, entered in .planning/WINDOWS.md, and closed by plan 10-09's `-write` pass over the whole phase."
  - "The account notice sits above the card's {{if .Enabled}} split, not inside a branch, and the test table now covers both branches. The plan says why; nothing in the plan's gates could tell the difference."

patterns-established:
  - "A gate that counts a symbol is not a gate for where the symbol is: the plan's `grep -c ViaSSO account.html == 1` is green under mutation 13, which shows the notice to half the people it was written for. Fourth instance in this phase of 10-CONTEXT's 'a gate must measure what its name claims'."
  - "Check that a -run pattern matches the tests it claims to run, and say so — carried from wave 5. Both filters were counted here: 8 of 8 in auth, 9 of 9 in admin."
  - "Cite functions, never line numbers — carried from waves 1-5. Every function this plan changed moved: :44 → :60, :70 → :91, :167 → :174, :197 → :204, :274 → :281, :402 → :414."
  - "Commit the task before mutating it — carried from wave 1, applied to all thirteen mutations here."

requirements-completed: [SSO-07]

coverage:
  - id: D1
    description: "An Authentik session satisfies the second-factor requirement unconditionally: MustHaveSecondFactor(role, viaSSO) returns role == \"admin\" && !viaSSO, and an administrator whose session carries via_sso reaches /admin/ without being redirected to the setup page"
    requirement: "SSO-07"
    verification:
      - kind: unit
        ref: "internal/auth/twofactor_test.go#TestMustHaveSecondFactorOverBothRolesAndBothWaysIn — all four combinations of role and viaSSO"
        status: pass
      - kind: unit
        ref: "internal/auth/twofactor_test.go#TestRequireSecondFactorLetsAnSSOSessionThrough — driven through a live scs session, not against the predicate"
        status: pass
      - kind: other
        ref: "mutation 1 — the !viaSSO term removed: 2 red in auth, 3 red in admin. mutation 3 — RequireSecondFactor never reads the flag: red."
        status: pass
    human_judgment: false
  - id: D2
    description: "A password session is bit-for-bit what it was: MustHaveSecondFactor(role, false) is the old MustHaveSecondFactor(role) for both roles, and a password administrator with no second factor is still redirected to the setup page"
    requirement: "SSO-09"
    verification:
      - kind: unit
        ref: "internal/auth/twofactor_test.go#TestMustHaveSecondFactorIsUnchangedForAPasswordSession — a table over both roles, asserted against the literal it replaced"
        status: pass
      - kind: unit
        ref: "internal/auth/twofactor_test.go#TestRequireSecondFactorStillRedirectsAPasswordAdmin"
        status: pass
      - kind: unit
        ref: "internal/admin/twofactor_sso_test.go#TestAccountScreenStillCallsTheSecondFactorCompulsoryForAPasswordAdmin"
        status: pass
      - kind: other
        ref: "mutation 2 — the predicate inverted to role == \"admin\" && viaSSO: 4 red in auth including the password-path table, 6 red in admin. mutation 4 — the middleware always believes viaSSO: red."
        status: pass
    human_judgment: false
  - id: D3
    description: "An administrator who signed in with a password is still forced to set a second factor up and still cannot switch it off — the :274 guard keeps saying yes for them and keeps naming `holzcloud user 2fa disable`"
    requirement: "SSO-09"
    verification:
      - kind: unit
        ref: "internal/admin/twofactor_sso_test.go#TestSecondFactorDisableStillRefusesAPasswordAdmin — asked of users.totp_confirmed_at, never of the flash message"
        status: pass
      - kind: unit
        ref: "internal/admin/twofactor_sso_test.go#TestSecondFactorDisableNamesTheWayBackForAPasswordAdmin"
        status: pass
      - kind: other
        ref: "mutation 6 — the guard handed true: 'an administrator who signed in with a password switched their second factor off'. Red in both tests. This is the mutation that would remove a protection for password users while adding a convenience for SSO ones."
        status: pass
    human_judgment: false
  - id: D4
    description: "An administrator signed in through the identity provider CAN switch a second factor off if they set one up here, because this installation no longer requires one of them — the decision, not a side effect"
    requirement: "SSO-07"
    verification:
      - kind: unit
        ref: "internal/admin/twofactor_sso_test.go#TestSecondFactorDisableLetsAnSSOAdminThrough"
        status: pass
      - kind: unit
        ref: "internal/admin/twofactor_sso_test.go#TestSecondFactorDisableStillWorksForAnEditor — the negative control on a path that was already open"
        status: pass
      - kind: other
        ref: "mutation 5 — the guard handed false: red. mutation 9 — (*Handler).viaSSO reads a key nothing writes: red in three places."
        status: pass
    human_judgment: false
  - id: D5
    description: "The decision has exactly one home. MustHaveSecondFactor is one definition and five call sites, with no second predicate and no wrapper — and the compiler, not a reader, enumerated them"
    requirement: "SSO-07"
    verification:
      - kind: other
        ref: "`go build ./...` after the arity change listed internal/admin/twofactor.go:167, :197, :274 and :402 verbatim — quoted in full below"
        status: pass
      - kind: other
        ref: "grep for MustHaveSecondFactor in internal/, non-test, comments excluded == 6 (one definition + five callers), before AND after; delta 0"
        status: pass
      - kind: other
        ref: "grep for SessionKeyViaSSO writers in internal/, non-test == 1 — the flag still has one writer, in internal/admin/forwardauth.go"
        status: pass
    human_judgment: false
  - id: D6
    description: "The dependency on the operator's identity provider is shown on the person's own account screen — for somebody who has a second factor here and for somebody who does not"
    requirement: "SSO-07"
    verification:
      - kind: unit
        ref: "internal/admin/twofactor_sso_test.go#TestAccountScreenTellsAnSSOPersonWhereTheirSecondFactorIsEnforced — 7 rows over role × viaSSO × enrolled, read from the rendered screen"
        status: pass
      - kind: unit
        ref: "internal/admin/twofactor_sso_test.go#TestAccountScreenDoesNotCallTheSecondFactorCompulsoryForAnSSOAdmin"
        status: pass
      - kind: other
        ref: "mutation 10 — the {{if .ViaSSO}} guard removed: the two password rows red. mutation 11 — AccountData.ViaSSO never set: the two SSO rows red. mutation 13 — the notice moved inside the else-branch: the two enrolled SSO rows red, and the plan's own grep gate GREEN throughout."
        status: pass
    human_judgment: false
  - id: D7
    description: "The dependency is shown to the administrator reading the account list when HOLZCLOUD_SSO_ENABLED is on, and with single sign-on off the screen is what it was"
    requirement: "SSO-07"
    verification:
      - kind: unit
        ref: "internal/admin/twofactor_sso_test.go#TestUserListTellsAnAdministratorTheInstallationDependsOnTheIdentityProvider — both settings"
        status: pass
      - kind: other
        ref: "mutation 12 — SSOEnabled hardcoded true: the switched-off row red"
        status: pass
    human_judgment: false
  - id: D8
    description: "Both new sentences are German literals inside {{t}}, visible to tools/i18n: 1319 → 1321 strings, exactly +2"
    verification:
      - kind: other
        ref: "`go run ./tools/i18n | head -1` — 1319 before, 1321 after, delta +2"
        status: pass
    human_judgment: false
  - id: D9
    description: "Whether an operator who has NOT configured a second factor at their Authentik understands that switching HOLZCLOUD_SSO_ENABLED on removes this installation's own second-factor requirement for every administrator who signs in that way — and whether the two new sentences say that clearly enough to a person who is not the operator"
    verification: []
    human_judgment: true
    rationale: "This is the transfer T-10-35 records, and it is the one thing in this plan no test can judge. The tests prove the sentences are on the screens and that they are there for the right people; whether an operator reads 'die Bestätigung in zwei Schritten verlangt dort, wer dich angemeldet hat' and then goes and checks their Authentik is exactly the question plan 10-10's browser pass and plan 10-08's DEPLOY.md paragraph exist for. The German wording is also untranslated in en/es/fr/it until plan 10-09, so a non-German-reading operator currently sees German."

duration: 25 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 06: One Predicate, Five Call Sites Summary

**`auth.MustHaveSecondFactor` gains a `viaSSO` parameter and returns `role == "admin" && !viaSSO`, so an Authentik session satisfies the second-factor requirement in exactly one place — with the arity changed rather than a variant added, which made the compiler print the four call sites no planning document named, and with the transferred dependency put on two admin screens instead of into a source comment.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-09-08T05:22Z (approximate; the first commit landed at 05:31:06Z, after the reading)
- **Completed:** 2026-09-08T05:47Z
- **Tasks:** 3
- **Files:** 2 created, 5 modified

## The call-site count: the plan was right, the roadmap was wrong

`ROADMAP.md`'s D-04 note and `10-CONTEXT.md`'s D-04 bullet both say
`MustHaveSecondFactor` is *"one function, one caller at `:70`"*. The plan says
five. **Measured on the tree, the plan is right and both source documents are
wrong** — and the way it was measured is the point: the signature was changed
first and the compiler was allowed to enumerate them.

```
# github.com/holzcloud/holzcloud-cms/internal/admin
internal/admin/twofactor.go:167:44: not enough arguments in call to auth.MustHaveSecondFactor
	have (string)
	want (string, bool)
internal/admin/twofactor.go:197:45: not enough arguments in call to auth.MustHaveSecondFactor
	have (string)
	want (string, bool)
internal/admin/twofactor.go:274:31: not enough arguments in call to auth.MustHaveSecondFactor
	have (string)
	want (string, bool)
internal/admin/twofactor.go:402:40: not enough arguments in call to auth.MustHaveSecondFactor
	have (string)
	want (string, bool)
```

Plus `internal/auth/twofactor.go:70`, which was updated in the same edit as the
definition. **Five, exactly the plan's list, exactly the plan's line numbers.**

Cited by name rather than by line, because every one of them has since moved:

| Call site | Function | Was | Is now | What it decides |
|---|---|---|---|---|
| 1 | `auth.RequireSecondFactor` | `:70` | `:91` | whether a request is redirected to the setup page |
| 2 | `admin.(*Handler).HandleTwoFactorSetup` | `:167` | `:174` | whether the setup screen says the step is compulsory |
| 3 | `admin.(*Handler).confirmTwoFactor` | `:197` | `:204` | the same screen after a wrong code |
| 4 | `admin.(*Handler).HandleTwoFactorDisable` | `:274` | `:281` | **the guard** — whether a second factor may be switched off |
| 5 | `admin.(*Handler).HandleAccount` | `:402` | `:414` | whether the account screen says "Pflicht" |

The definition itself moved `:44` → `:60`. **Four of these five were in no
planning document**, and one of the four is a behavioural guard rather than a
screen flag. A reader who believed the roadmap would have changed the signature,
met four unexpected compile errors, and had every incentive to add a wrapper to
make them go away — which is precisely what D-04's "exactly one home" forbids
and what this plan's insistence on the arity change prevents.

## Accomplishments

- **The predicate has one home and one input.** `MustHaveSecondFactor(role, viaSSO)` is the whole decision. `RequireSecondFactor` already held `sm`, so it reads `SessionKeyViaSSO` itself and needed no new dependency; `scs.GetBool` answers false for an absent key, so the password path takes **no new branch at all**, which is what makes "nothing changed" checkable rather than argued.
- **The password path is asserted as a table over both roles,** twice: once against the four-combination predicate table and once against the literal it replaced. Mutation 2 inverts the predicate and reddens both, plus the disable guard and both screens.
- **The `:274` guard still refuses a password administrator and still names `holzcloud user 2fa disable`.** Its behaviour for an SSO-signed administrator changes on purpose, and both directions have their own named test. Mutation 6 — the guard handed `true` — is the mutation that removes a protection for password users while adding a convenience for SSO ones, and it goes red in two places.
- **`internal/auth/twofactor.go` had no test file before this plan.** `MustHaveSecondFactor` and `RequireSecondFactor` were untested. The middleware's three unchanged early returns (nil lookup, a second-factor path, no session user) are now asserted too — not because this plan touches them, but because each is a way a future change could start reading a session flag it must not need.
- **The transferred dependency is on two screens, and it is gated by a test rather than by a grep.** The plan's own `grep -c ViaSSO` gate is green under mutation 13.
- **Thirteen mutations, thirteen reds,** every one against a **committed** file, restored with `git checkout --`, with `git status --short -- internal/ cmd/` verified empty afterwards.
- **No green mutation this wave.** Wave 5's two green mutations had two different answers and the knowledge base is right that the observation is indistinguishable; here there was nothing to investigate, and that is said plainly rather than dressed up.

## Task Commits

1. **Task 1 (RED): the behavioural half, which compiles against the pre-change tree** — `85233ed` (test)
2. **Task 1 (GREEN): the arity change and the five call sites** — `82ae14f` (feat)
3. **Task 1 (follow-up): the four-row predicate table, which could not be RED** — `103603f` (test)
4. **Task 2: the dependency on two screens** — `8d6df09` (feat)
5. **Task 2 (follow-up): the notice asserted in both branches of the card** — `f333a7b` (test)

Task 1 carried `tdd="true"`. **Four assertions failed in the RED state** and the
rest are negative or unchanged assertions whose value shows only under mutation:

```
--- FAIL: TestRequireSecondFactorLetsAnSSOSessionThrough
    twofactor_test.go:58: status = 303; want 200 — an administrator signed in through the identity
    provider was sent to "/admin/2fa/einrichten" …
--- FAIL: TestSecondFactorDisableLetsAnSSOAdminThrough
    twofactor_sso_test.go:135: an administrator signed in through the identity provider could not
    switch their second factor off …
--- FAIL: TestAccountScreenDoesNotCallTheSecondFactorCompulsoryForAnSSOAdmin
    twofactor_sso_test.go:166: the account screen tells an administrator signed in through the
    identity provider that a second factor is compulsory here …
--- FAIL: TestSecondFactorSetupScreenDropsTheCompulsoryWarningForAnSSOAdmin/through_the_identity_provider
    twofactor_sso_test.go:207: the setup screen's compulsory warning = true; want false
```

**Task 3 produced no commit,** as its own `<action>` says: it is arithmetic and
one correction, and both live in this document.

## Files Created/Modified

- `internal/auth/twofactor.go` (85 → 108 lines) — the arity change, three new doc-comment paragraphs, and one new line in `RequireSecondFactor`
- `internal/auth/twofactor_test.go` (**new**, 201 lines) — 8 test functions, 11 named subtests
- `internal/admin/twofactor.go` (455 → 470 lines) — `(*Handler).viaSSO`, four updated call sites, `AccountData.ViaSSO`
- `internal/admin/twofactor_sso_test.go` (**new**, 281 lines) — 9 test functions, 12 named subtests, and the helpers `newSecondFactorAdmin`, `seedSecondFactorAccount`, `enableSecondFactor`, `secondFactorEnabled`, `serveSignedIn`
- `internal/admin/user.go` (498 → 505 lines) — `UserListData.SSOEnabled`
- `cmd/holzcloud/templates/admin/account.html` — one German sentence in a `callout`, above the card's two branches
- `cmd/holzcloud/templates/admin/user_list.html` — one German sentence, inside `user_list-content` so an htmx fragment carries it

## The counting table, measured

**The plan's own preamble warns that these rows are deltas and that a gate
checking an absolute a foreign phase moves fails on an empty commit.** Four of
the eight rows were already stale before this plan started, and the first row is
stale for a different and more interesting reason.

| What | Plan's baseline | Measured before | This plan adds | Measured after | Verdict |
|---|---|---|---|---|---|
| `MustHaveSecondFactor` non-test, plan's literal gate | 6 | **7** | +1 | **8** | ✗ **the gate counts its own explanation** — see below |
| `MustHaveSecondFactor` non-test, comments excluded | — | **6** | 0 | **6** | ✓ one definition, five callers, delta 0 |
| callers (`grep -vc 'func MustHave…'`) | 5 | **5** | 0 | **5** | ✓ exactly as the plan predicts |
| `SessionKeyViaSSO` writers | 1 | **1** | 0 | **1** | ✓ one writer, in `forwardauth.go` |
| admin templates | 66 | **68** | 0 | **68** | ✓ delta 0; the plan's absolute is stale |
| `layoutPageNames` entries | 50 | **52** | 0 | **52** | ✓ delta 0; the plan's absolute is stale |
| `adminProtectedMux.Handle` | vorher | **159** | 0 | **159** | ✓ no new route |
| migrations | 49 | **51** | 0 | **51** | ✓ delta 0; the plan's absolute is stale, five waves running |
| `ViaSSO` in `account.html` | 0 | **0** | +1 | **1** | ✓ — but see mutation 13 |
| `SSOEnabled` in `user_list.html` | 0 | **0** | +1 | **1** | ✓ |
| strings in source | vorher | **1319** | +2 | **1321** | ✓ exactly +2 |
| open keys per catalogue (en, es, fr, it) | 0 | **0** | +2 | **2** | expected; plan 10-09 closes them |

### The first row is a gate that counts its own explanation

The plan specifies

```
grep -rn 'MustHaveSecondFactor' internal/ --include='*.go' | grep -v _test | wc -l
```

and says it must print **6** — "one definition and five call sites". It printed
**7 before this plan touched anything**, because the function's own doc comment
line (`// MustHaveSecondFactor decides who is required to set one up.`) is a line
naming `MustHaveSecondFactor`. It prints **8** after, because the doc comment on
the new `AccountData.ViaSSO` field explains that it carries the same fact
*"MustHaveSecondFactor decides"*.

The property the gate is *named* for — no second predicate, no wrapper — is
intact and measurable. With comments excluded it prints exactly the plan's 6,
before and after:

```
grep -rn 'MustHaveSecondFactor' internal/ --include='*.go' | grep -v _test \
  | sed 's/^[^:]*:[0-9]*://' | grep -vc '^[[:space:]]*//'
before (1fff843): 6
after  (f333a7b): 6
```

**The comment was not reworded to make the number come out right.** The sentence
explaining that the screen and the middleware read one value is worth more than
a green count, and the honest correction is the corrected gate.

This is the **fourth** instance in phase 10 of `10-CONTEXT.md`'s *"a gate must
measure what its name claims"*, and the second of the exact sub-shape wave 5
recorded: the plan applies `grep -v '//'` in one place and forgets it in the one
beside it.

## The i18n position, with the number plan 10-09 is closing

```
before:  1319 Zeichenketten im Quelltext
         en.json 1319 übersetzt, 0 offen, 0 verwaist   (es, fr, it identical)
after:   1321 Zeichenketten im Quelltext
         en.json 1319 übersetzt, 2 offen, 0 verwaist   (es, fr, it identical)
delta:   +2 strings, +2 newly open per catalogue, 0 orphaned
```

**This plan leaves four catalogues with two open keys each, and CLAUDE.md's gate
asks for `0 offen`.** That divergence is deliberate, instructed by the plan, and
stated here rather than hidden: translating two of the phase's strings by hand
would put them through a different process from the rest, and plan 10-09 runs
`go run ./tools/i18n -write` over the whole phase and rebuilds `de-CH.json` with
`-schweiz`. It is also entered in `.planning/WINDOWS.md` so the ship gate can see
it without reading this file.

**The two keys, verbatim, so plan 10-09 has a list rather than a search:**

1. `Du bist über die Anmeldung deiner Organisation hier hereingekommen. Die Bestätigung in zwei Schritten verlangt dort, wer dich angemeldet hat – diese Anlage verlangt sie dann nicht noch einmal.`
2. `Wer sich über die Anmeldung der Organisation anmeldet, bringt seine Bestätigung in zwei Schritten von dort mit. Ob sie verlangt wird, entscheidet die Anmeldung der Organisation und nicht diese Anlage.`

Both are German literals inside `{{t}}` in `cmd/holzcloud/templates/admin`, which
is one of the two roots `tools/i18n` reads — so they are *offen*, which is a
reported state, and not invisible, which is the state CLAUDE.md's Translation
section warns about. `de-CH.json` and the two hand-maintained `*-CH` catalogues
are unchanged (73, 4 and 9 divergences, as before).

## Mutation Verification

Every guard was removed, inverted or redirected on a **committed** file, run
against this plan's own `-run` patterns, restored with `git checkout --`, and
`git status --short -- internal/ cmd/` confirmed to carry none of this plan's
files afterwards. **Thirteen mutations, thirteen reds.** Verbatim, abridged only
where a failure repeats across subtests.

**1. The `!viaSSO` term removed from the predicate** — the old one-argument behaviour restored
```
--- FAIL: TestMustHaveSecondFactorOverBothRolesAndBothWaysIn
    --- FAIL: …/an_administrator_who_signed_in_through_the_identity_provider
--- FAIL: TestRequireSecondFactorLetsAnSSOSessionThrough
    twofactor_test.go:110: status = 303; want 200 — an administrator signed in through the identity
    provider was sent to "/admin/2fa/einrichten", and asking the same person for a second factor
    twice is what this plan removes
    twofactor_test.go:114: the next handler did not run; an SSO session must reach the administration
--- FAIL: TestSecondFactorDisableLetsAnSSOAdminThrough
--- FAIL: TestAccountScreenDoesNotCallTheSecondFactorCompulsoryForAnSSOAdmin
--- FAIL: TestSecondFactorSetupScreenDropsTheCompulsoryWarningForAnSSOAdmin/through_the_identity_provider
```
Red in five places across two packages, which is the shape that shows the
predicate really is the one home: one edit, every screen and the middleware.

**2. The predicate inverted — `role == "admin" && viaSSO`** — the protection-removing direction
```
--- FAIL: TestMustHaveSecondFactorOverBothRolesAndBothWaysIn
    --- FAIL: …/an_administrator_who_signed_in_with_a_password
    --- FAIL: …/an_administrator_who_signed_in_through_the_identity_provider
--- FAIL: TestMustHaveSecondFactorIsUnchangedForAPasswordSession
    twofactor_test.go:84: MustHaveSecondFactor("admin", false) = false; want true — passing false
    must be bit-for-bit the old one-argument predicate
--- FAIL: TestRequireSecondFactorStillRedirectsAPasswordAdmin
    twofactor_test.go:126: status = 200; want 303 …
--- FAIL: TestSecondFactorDisableStillRefusesAPasswordAdmin
    twofactor_sso_test.go:95: an administrator who signed in with a password switched their second
    factor off; the guard that refuses it is the one protection a password session has left
--- FAIL: TestSecondFactorDisableNamesTheWayBackForAPasswordAdmin
--- FAIL: TestAccountScreenStillCallsTheSecondFactorCompulsoryForAPasswordAdmin
    twofactor_sso_test.go:182: an administrator who signed in with a password is no longer told a
    second factor is compulsory; nothing about the password path may move in this plan
```
Ten failures. This is the mutation SSO-09 exists for.

**3. `RequireSecondFactor` never reads the flag (`viaSSO := false`)**
```
--- FAIL: TestRequireSecondFactorLetsAnSSOSessionThrough
    twofactor_test.go:110: status = 303; want 200 …
```

**4. `RequireSecondFactor` always believes the session came via SSO (`viaSSO := true`)**
```
--- FAIL: TestRequireSecondFactorStillRedirectsAPasswordAdmin
    twofactor_test.go:126: status = 200; want 303 — an administrator who signed in with a password
    and has no second factor is still sent to the setup page
    twofactor_test.go:129: Location = ""; want "/admin/2fa/einrichten"
    twofactor_test.go:132: the next handler ran; a redirect is not a fall-through
```

**5. The disable guard handed `false`** — the SSO administrator stays locked in
```
--- FAIL: TestSecondFactorDisableLetsAnSSOAdminThrough
    twofactor_sso_test.go:135: an administrator signed in through the identity provider could not
    switch their second factor off; this installation does not require one of them, so it may not refuse
```

**6. The disable guard handed `true`** — **the password administrator can switch it off**
```
--- FAIL: TestSecondFactorDisableStillRefusesAPasswordAdmin
    twofactor_sso_test.go:95: an administrator who signed in with a password switched their second
    factor off; the guard that refuses it is the one protection a password session has left
--- FAIL: TestSecondFactorDisableNamesTheWayBackForAPasswordAdmin
    twofactor_sso_test.go:119: flash = ""; want it to name „holzcloud user 2fa disable“ — the refusal
    has to say what the way back is
```
The `:274` call site is the one the plan singles out, and this is what a mistake
in it costs.

**7. The account screen's `Required` handed `false`**
```
--- FAIL: TestAccountScreenDoesNotCallTheSecondFactorCompulsoryForAnSSOAdmin
```

**8. The setup screen's two `Required` flags handed `false`**
```
--- FAIL: TestSecondFactorSetupScreenDropsTheCompulsoryWarningForAnSSOAdmin/through_the_identity_provider
    twofactor_sso_test.go:207: the setup screen's compulsory warning = true; want false
```
Both `:167` and `:197` are covered by this one subtest pair; they build the same
screen from two entry points.

**9. `(*Handler).viaSSO` reads a key nothing writes (`"via_single_sign_on"`)**
```
--- FAIL: TestSecondFactorDisableLetsAnSSOAdminThrough
--- FAIL: TestAccountScreenDoesNotCallTheSecondFactorCompulsoryForAnSSOAdmin
--- FAIL: TestSecondFactorSetupScreenDropsTheCompulsoryWarningForAnSSOAdmin/through_the_identity_provider
```
Red in three places — the one-reader helper is load-bearing for all four screen
call sites.

**10. The account notice's `{{if .ViaSSO}}` guard removed** — everybody sees it
```
--- FAIL: TestAccountScreenTellsAnSSOPersonWhereTheirSecondFactorIsEnforced
    --- FAIL: …/an_administrator_with_a_password
    --- FAIL: …/an_editor_with_a_password
```

**11. `AccountData.ViaSSO` never set** — nobody sees it
```
--- FAIL: TestAccountScreenTellsAnSSOPersonWhereTheirSecondFactorIsEnforced
    --- FAIL: …/an_administrator_through_the_identity_provider
    --- FAIL: …/an_editor_through_the_identity_provider
```

**12. `UserListData.SSOEnabled` hardcoded `true`** — the notice on an installation with no single sign-on
```
--- FAIL: TestUserListTellsAnAdministratorTheInstallationDependsOnTheIdentityProvider
    --- FAIL: …/with_single_sign-on_switched_off
    twofactor_sso_test.go:266: the user list names the identity provider = true; want false — with
    single sign-on off the screen must be byte-for-byte what it was
```

**13. The account notice moved inside the `{{else}}` branch** — *the plan's grep gate stays green*
```
--- FAIL: TestAccountScreenTellsAnSSOPersonWhereTheirSecondFactorIsEnforced
    --- FAIL: …/an_administrator_through_the_identity_provider_who_set_one_up_here_anyway
    --- FAIL: …/an_editor_through_the_identity_provider_who_set_one_up_here_anyway
```
```
and the plan's gate throughout:  grep -c 'ViaSSO' account.html → 1  ✓
```
**Before the follow-up commit this mutation was GREEN.** See Deviations 3.

## Decisions Made

- **`internal/auth/twofactor_test.go` was created, not extended.** The plan says extend and lists it under `files_modified`; it did not exist. `MustHaveSecondFactor` and `RequireSecondFactor` had **no test of any kind** on this tree — which is itself the finding, because the middleware that decides whether an administrator may reach the administration without a second factor was the untested one.
- **`internal/admin/twofactor_sso_test.go` was created although the plan's `<files>` does not list it.** The plan's behaviour list demands a named test for the `:274` guard in both directions and says so explicitly ("worth its own named test rather than being left to follow from the predicate"), and package `auth` cannot reach a handler. Recorded as Deviation 2.
- **The RED commit holds the behavioural half only.** The four-row table names the new arity, so on the pre-change tree it is a compile error, and a compile error is not a red test. It went in as its own commit immediately after GREEN, which is the honest ordering: the behavioural failure is what proved the change was needed, and the table is what stops any row of it changing silently later.
- **The account notice sits above the card's `{{if .Enabled}}` split** — the plan says why, and mutation 13 is what made the reason assertable.
- **`h.cfg != nil && h.cfg.SSOEnabled` rather than `h.cfg.SSOEnabled`.** `internal/admin/forwardauth.go` already guards `h.cfg == nil`, and `newTestAdmin` builds a handler with a config that other suites may leave unset. A nil dereference in the user list would be a 500 on a screen every administrator opens.
- **The plan's stale absolutes were not adjusted away.** Four of the eight counting rows have absolutes that a foreign phase moved; all four have delta 0 and are reported with both numbers.

## Deviations from Plan

### Auto-fixed

**1. [Rule 3 — Blocking] `internal/auth/twofactor_test.go` did not exist**

- **Found during:** Task 1, before the first edit.
- **Issue:** The plan says "Extend `internal/auth/twofactor_test.go`" and lists it as modified. There is no such file, and `grep -rln 'RequireSecondFactor' --include='*_test.go'` returns nothing: neither the predicate nor the middleware had a test.
- **Fix:** created it, with the plan's table plus the middleware's three unchanged early returns.
- **Files:** `internal/auth/twofactor_test.go` (new)
- **Verification:** mutations 1, 2, 3 and 4, all red against it.
- **Committed in:** `85233ed` and `103603f`

**2. [Rule 2 — Missing critical] The `:274` guard needed a test in a package the plan's file list does not name**

- **Found during:** Task 1 RED.
- **Issue:** The plan's `<files>` for task 1 is `internal/auth/twofactor.go, internal/auth/twofactor_test.go, internal/admin/twofactor.go` — no admin test file. But three of its behaviour bullets are about handlers (`the :274 guard still refuses`, `an SSO administrator can switch one off`, `the account screen does not say compulsory`), and package `auth` cannot reach a handler. Without an admin test file, the guard whose behaviour a mistake would actually change would have had no assertion at all.
- **Fix:** `internal/admin/twofactor_sso_test.go`, 9 test functions driving the real handlers over a real migrated database and the real on-disk templates.
- **Verification:** mutations 5, 6, 7, 8, 9, 10, 11, 12 and 13 are all red against this file and would all have been green without it. Mutation 6 in particular — a password administrator switching their second factor off — would have shipped unobserved.
- **Committed in:** `85233ed`, `8d6df09`, `f333a7b`

**3. [Rule 2 — Missing critical] The account notice was asserted in only one of the card's two branches**

- **Found during:** Task 3, applying wave 4's third method finding (*a grep gate can read a proxy*) to this plan's own gates.
- **Issue:** The plan's gate is `grep -c 'ViaSSO' cmd/holzcloud/templates/admin/account.html` and must print 1. It prints 1 whether the notice sits above the `{{if .Enabled}}` split — where the plan says it belongs, "because it applies to a person who has a second factor here and to one who does not" — or inside one of the branches. **And the behavioural table had the same hole**: every seeded account had no second factor, so only the `{{else}}` branch was ever rendered. Mutation 13 moves the notice into that branch: the gate stays at 1, every test stays green, and an SSO administrator who set an authenticator up here reads nothing.
- **Fix:** three rows added to `TestAccountScreenTellsAnSSOPersonWhereTheirSecondFactorIsEnforced`, covering somebody signed in through the identity provider who has enrolled here anyway.
- **Verification:** mutation 13, verbatim above — green before this commit, red after, with the plan's grep gate green throughout.
- **Committed in:** `f333a7b`

### Divergences recorded rather than worked around

**4. The plan's `MustHaveSecondFactor == 6` gate counts its own explanation.** Reported in full under "The counting table, measured". It printed 7 before this plan started and 8 after. The property is intact; the corrected gate prints 6 before and after; the doc comment was **not** reworded to make the number come out right.

**5. Three of the plan's absolutes are stale** — admin templates specified 66, measured 68; `layoutPageNames` specified 50, measured 52; migrations specified 49, measured 51. All three have delta 0. The migrations divergence is now the sixth consecutive wave to record it, which makes it a property of the planning template rather than of any one plan.

**6. The RED commit could not contain the predicate table.** Reason under "Decisions Made". Wave 5 recorded the mirror-image case (a test split across two commits to match a task boundary); this is the other direction — a test that cannot exist until the production change does.

**7. Task 3 produced no commit.** Its `<action>` says "No new code. The arithmetic, and one correction written down." Both are in this document, which is Task 3's deliverable.

**8. The plan's `<verify>` for Task 3 specifies `ls cmd/holzcloud/templates/admin/*.html | wc -l` must print 66 and `migrations` 49.** Both fail on this tree for reasons that predate the phase. Named rather than adjusted; the delta each row actually asserts is 0 and holds.

---

**Total deviations:** 3 auto-fixed (1 Rule 3, 2 Rule 2) + 5 divergences recorded.
**Impact on plan:** No scope creep. Deviations 1 and 2 are test files the plan's own behaviour list requires and its file list omits — no production code was added for either. Deviation 3 is three test rows. Nothing in `internal/admin/handler.go`, `internal/admin/forwardauth.go`, `internal/user/`, `internal/web/` or `cmd/holzcloud/*.go` was touched.

## Verification Results

**This run is from a tree whose `git status --short -- internal/ cmd/ tools/` was
empty at both ends and whose `HEAD` did not move during it** — `f333a7b` before
and `f333a7b` after.

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | silent |
| `gofmt -l .` | silent |
| `go test ./...` | **44 packages ok, 0 failures** |
| `go test ./internal/auth/ -run SecondFactor -v` | 8 test functions, all PASS, 11 named subtests |
| `go test ./internal/admin/ -run 'SecondFactor\|AccountScreen\|UserList' -v` | 9 test functions, all PASS, 12 named subtests |
| **the `-run` filters checked against the names they claim to run** | auth: 8 of 8 defined test functions matched. admin: 9 of 9 matched. **Wave 5's finding does not recur here.** |
| `grep MustHaveSecondFactor` non-test, plan's literal gate | **8** — the plan wants 6; it counts two comment lines; see Deviation 4 |
| the same, comments excluded | **6** ✓ before and after — one definition, five callers |
| callers only | **5** ✓ |
| `SessionKeyViaSSO` writers | **1** ✓ — `internal/admin/forwardauth.go` |
| `grep -c ViaSSO cmd/holzcloud/templates/admin/account.html` | **1** ✓ — and green under mutation 13; see Deviation 3 |
| `grep -c SSOEnabled cmd/holzcloud/templates/admin/user_list.html` | **1** ✓ |
| `ls cmd/holzcloud/templates/admin/*.html \| wc -l` | 68 — the plan's literal gate expects 66 and is stale; **delta 0** |
| `layoutPageNames` entries | 52 — the plan expects 50 and is stale; **delta 0** |
| `ls internal/db/migrations/*.sql \| wc -l` | 51 — the plan expects 49 and is stale; **delta 0** |
| `grep -c adminProtectedMux.Handle cmd/holzcloud/main.go` | 159 — **delta 0**, no new route |
| `go run ./tools/i18n \| head -1` | 1319 before, **1321** after — **delta +2**, exactly the two sentences |
| open keys per catalogue | 0 before, **2** after in en/es/fr/it — expected, handed to plan 10-09 |
| `git diff --diff-filter=D --name-only 1fff843..f333a7b` | **empty** — no file was deleted by any commit of this plan |

## Issues Encountered

- **Both source documents were wrong about the size of the change, and the plan was right.** Recorded above with the compiler's own output. The instructive part is the method rather than the number: had the change been made by adding a `MustHaveSecondFactorForSession` variant, the four unlisted call sites would have compiled untouched and the `:274` guard would still be asking the old question — which is exactly the "correct because every caller remembered" shape this project's knowledge base already has six entries about.
- **The function under change had no test file.** `internal/auth/twofactor.go` is the middleware that decides whether an administrator may reach the administration without a second factor, and nothing tested it. That is not a defect this plan introduced, but it does mean the SSO-09 claim "the password path is bit-for-bit unchanged" had nothing to rest on before this plan wrote it down.
- **No green mutation this wave, and that is stated rather than embellished.** Wave 5's two green mutations produced two different answers and the knowledge base is right that the observation is indistinguishable at the moment it happens. Here all thirteen went red on the first run. The one thing that *looked* like a green mutation — the plan's `grep -c ViaSSO` gate staying at 1 under mutation 13 — is a gate reading a proxy, not a covered property, and it was investigated to a conclusion (Deviation 3) rather than waved through.
- **Three concurrent agents committed into this tree throughout.** Plan 10-07 (`internal/admin/login.go`, `login_test.go`), a `internal/block` fix and a `internal/public/pluginhost.go` fix. At one point a full-suite run showed four failures in `internal/admin` — `TestLogoutOfAnSSOSessionSignsOutAtTheIdentityProvider` and three siblings — all of them plan 10-07's own RED commit mid-flight in `login_test.go`, none of them touched by this plan. They were green again by the final run. Every commit here was staged file by file and verified with `git show --stat`; all five touch only this plan's files.
- **`newTestAdmin`'s zero `Argon2Params`** bit for the third consecutive wave. `newSecondFactorAdmin` sets `cheapHashing`, as waves 4 and 5 both predicted the next plan would have to.

## Threat Flags

None new. Every `mitigate` disposition in the plan's register has a passing test and a red mutation; the one `transfer` is what this whole plan is.

| Threat | Disposition | Covered by |
|---|---|---|
| T-10-35 an administrator with no second factor anywhere | **transfer** | Not mitigated and not claimed to be. The condition SSO-07 attaches to the transfer is discharged here for the admin half — two screens, both gated by tests, one of them found by mutation 13 — and by plan 10-08 for `DEPLOY.md`. `MustHaveSecondFactor`'s doc comment names both places so a reader who changes the function knows what else must change. |
| T-10-36 the `via_sso` flag set by something other than the sign-in | mitigate | One writer, gated at 1. The flag is in the server-side `scs` session in SQLite, not a client-editable cookie. Mutation 9 shows what a second, wrongly-named reader costs. |
| T-10-37 a password-signed administrator escaping the requirement | mitigate | Mutations 2, 4 and 6, ten failures between them; the four-row table; `TestMustHaveSecondFactorIsUnchangedForAPasswordSession`; `scs.GetBool` returning false for an absent key so the password path takes no new branch. |
| T-10-38 the dependency living only in a source comment | mitigate | Mutations 10, 11, 12 and 13. Note that the plan's own grep gates are green under 13, so the tests and not the gates are what hold this. |
| T-10-SC package-manager installs | accept | No install task; every symbol is Go standard library or already in `go.mod`. |

**One surface the register does not name, found while reading `:274`.** The
disable guard now says yes to an SSO-signed administrator, which means a person
who signed in through the identity provider **can remove their own TOTP
enrolment here**. If the operator later switches `HOLZCLOUD_SSO_ENABLED` off,
that administrator falls back to the password form with no second factor and is
sent to the setup page by `RequireSecondFactor` — correct, and a surprise. It is
not a hole: `MustHaveSecondFactor("admin", false)` is true again the moment the
session is a password one, so they cannot reach the administration until they
enrol. It belongs in `DEPLOY.md` (plan 10-08) as a sentence about what switching
single sign-on **off** does to accounts that were using it.

## Known Stubs

None. Every field, helper and sentence this plan adds is reachable, wired and
tested. `AccountData.ViaSSO` and `UserListData.SSOEnabled` are read by templates
whose rendering is asserted; `(*Handler).viaSSO` has four callers and a mutation
that reddens three tests.

**One tracked open item, which is not a stub:** the two new catalogue keys are
untranslated in `en`, `es`, `fr` and `it` — `2 offen` each — by the plan's
instruction, closed by plan 10-09. Recorded in `.planning/WINDOWS.md`. A
non-German-reading operator sees German on those two lines until then.

## User Setup Required

None from this plan. Two things it hands to plan 10-08 (`DEPLOY.md`), on top of
waves 4's and 5's six:

1. **Switching `HOLZCLOUD_SSO_ENABLED` on removes this installation's own
   second-factor requirement for every administrator who signs in through the
   proxy.** Whether those administrators are protected by two factors is from
   that moment a question about the operator's Authentik. The two admin screens
   say so; `DEPLOY.md` is where the operator is told to go and check that their
   Authentik actually enforces one.
2. **Switching it back off is not symmetric.** An administrator who removed
   their TOTP enrolment while single sign-on was on will be sent to the setup
   page at their next password sign-in and cannot reach the administration until
   they enrol again. That is correct behaviour and it will read as a lock-out to
   somebody who does not expect it.

## Next Phase Readiness

- **`MustHaveSecondFactor` is at exactly 5 call sites.** A sixth for the rest of this phase means a screen is deciding "is this compulsory" somewhere a reader of the five would not find it. A second *definition* — a wrapper, a variant, a `…ForSession` — is what D-04 forbids outright.
- **`SessionKeyViaSSO` now has one writer and three readers**: `RequireSecondFactor`, `(*Handler).viaSSO`, and plan 10-07's sign-out. `internal/auth/session.go`'s own comment predicted exactly two readers and is now one short; whoever touches it next should update that sentence rather than leave a comment that undercounts.
- **Plan 10-07 (sign-out)** was executing in this tree concurrently and reads the same flag. No file overlaps this plan.
- **Plan 10-08 (`DEPLOY.md`)** owes the two paragraphs above plus wave 4's three and wave 5's three — eight in total, and the first of this plan's two is the whole reason SSO-07 exists.
- **Plan 10-09 (translation)** closes **2 open keys in each of `en`, `es`, `fr`, `it`** — the exact strings are quoted above — and rebuilds `de-CH.json` with `-schweiz`. The source-string count it should see going in is **1321**.
- **Carry these baselines forward, measured on this tree at 05:47Z:** migrations **51**, admin templates **68**, source strings **1321** with **2 offen** per catalogue, `adminProtectedMux.Handle` **159**, `layoutPageNames` **52**, `MustHaveSecondFactor` call sites **5**, `SessionKeyViaSSO` writers **1**.
- **For plan 10-10 (the browser pass):** sign in through the proxy as an administrator with no second factor and confirm you land on the dashboard rather than the setup page; then read the account screen and judge whether the sentence explains what is happening. Then sign out, sign in with a password as the same account, and confirm the setup page is compulsory again. Every one of those is asserted by a test; none of them is asserted to be *comprehensible*, which is the whole of D9.
- **Do not cite `internal/admin/twofactor.go` or `internal/auth/twofactor.go` by line number.** All six sites this plan touched moved within it, and the phase's documents have now been wrong about a line number in four separate places.

## Self-Check: PASSED

- `internal/auth/twofactor.go` — FOUND (108 lines), contains `func MustHaveSecondFactor(role string, viaSSO bool)`, `sm.GetBool(r.Context(), SessionKeyViaSSO)`
- `internal/auth/twofactor_test.go` — FOUND (201 lines), 8 test functions, all matched by `-run SecondFactor`
- `internal/admin/twofactor.go` — FOUND (470 lines), contains `func (h *Handler) viaSSO`, `auth.MustHaveSecondFactor(role, h.viaSSO(r))` (4×), `ViaSSO bool`
- `internal/admin/twofactor_sso_test.go` — FOUND (281 lines), 9 test functions, all matched by `-run 'SecondFactor|AccountScreen|UserList'`
- `internal/admin/user.go` — FOUND (505 lines), contains `SSOEnabled bool` and `h.cfg != nil && h.cfg.SSOEnabled`
- `cmd/holzcloud/templates/admin/account.html` — FOUND, `ViaSSO` 1×
- `cmd/holzcloud/templates/admin/user_list.html` — FOUND, `SSOEnabled` 1×
- Commit `85233ed` — FOUND
- Commit `82ae14f` — FOUND
- Commit `103603f` — FOUND
- Commit `8d6df09` — FOUND
- Commit `f333a7b` — FOUND
- `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` — all clean at 05:47Z on an unmoving `f333a7b`, with `git status --short -- internal/ cmd/ tools/` empty at both ends

---
*Phase: 10-authentik*
*Completed: 2026-09-08*
