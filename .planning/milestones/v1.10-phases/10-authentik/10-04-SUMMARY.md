---
phase: 10-authentik
plan: 04
subsystem: auth
tags: [forward-auth, authentik, sso, provisioning, website-isolation, privilege-escalation, argon2, compensation]

requires:
  - phase: 10-authentik
    provides: "plan 10-01's config.SSOProvision and config.SSODefaultWebsite, which cannot both be set to a nonsense pair because the process refuses to start on one — so this plan's branch needs no second validation of its own"
  - phase: 10-authentik
    provides: "plan 10-03's ForwardAuthSignIn, whose `u == nil` refusal is the exact point this plan splits, and whose empty-address guard is the reason provisioning never sees an empty address in production"
provides:
  - "admin.(*Handler).provisionSSOUser: the only place in the codebase that creates an account without a person asking, and the only SetRights call site that is not a form"
  - "The same-function rule: Create and the website assignment are one function because between them there must be no request, no error path and no later plan"
  - "admin.errSSOEmptyAddress: a sentinel, so that provisioning's own empty-address guard can be asserted apart from the two other layers that also refuse it"
  - "admin.randomSecret: 32 bytes from crypto/rand, hashed by user.Store.Create and forgotten in the same statement"
  - "ssoRefuseProvisionFailed: the fourth refusal reason, which is how an operator whose default website was deleted learns that somebody was turned away"
affects: [10-05-group-rights, 10-06-second-factor, 10-07-admin-surface, 10-08-deploy-docs, 10-10-browser-pass]

actuals:
  tokens: 8400
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "A guard whose mutation stays green is investigated, not accepted — and when the investigation finds that other layers hold the property, the answer is to make the guard's own contribution assertable (a sentinel error + errors.Is) rather than to delete it or to wave it through"
    - "The load-bearing assertion is made against the function that authorises a real request, never against the row it reads: NewWebsiteAccessLookup is called, user_websites is only described"
    - "A control step that deletes the one row and watches the same assertion flip, so the test demonstrates the property it asserts is the one that would break"
    - "A half-finished write is compensated through the ordinary delete path, and the compensation's failure names the row an operator must remove by hand"

key-files:
  created: []
  modified:
    - internal/admin/forwardauth.go
    - internal/admin/forwardauth_test.go

key-decisions:
  - "provisionSSOUser takes *http.Request rather than the plan's context.Context, because the user.create protocol row is written inside the function that creates the account and h.LogActivity takes the request. Splitting the record from the creation is the exact failure this file's own header comment warns about."
  - "A failed provisioning DOES write an auth.login_fail row, unlike the failed account lookup one branch above it. The commonest way to arrive there is a HOLZCLOUD_SSO_DEFAULT_WEBSITE naming a deleted website — a healthy database and an operator who needs to see that somebody was turned away. activity.Store.Log never returns an error by design, so this cannot turn a refusal into a 500 even when the database really has stopped answering."
  - "errSSOEmptyAddress is a sentinel rather than an inline errors.New, because three independent layers refuse the empty address and no behavioural test could say which one did. Mutation 5 proved that by staying green."
  - "Two comments in ForwardAuthSignIn that this change made untrue were corrected in the same commit: the step list's 'the account lookup, which creates nothing' and step 5's 'provisioning is a later plan'."

patterns-established:
  - "The plan's own grep gate for the secret in the log reads a proxy and mutation 8 proved it: with the secret appended to a continuation line of slog.Info, the gate still printed 0 while the secret was in the log. The behavioural test caught it. Recorded as the phase's second instance of 10-CONTEXT's 'a gate must measure what its name claims'."
  - "Cite functions, never line numbers — carried from waves 1-3. Nothing this plan wrote cites a line in internal/admin/handler.go."
  - "Commit the task before mutating it — carried from wave 1, applied to all ten mutations here."

requirements-completed: [SSO-05]

coverage:
  - id: D1
    description: "An identity with no account is refused unless the operator switched creation on, and the refusal writes one auth.login_fail row and falls through to the password form"
    requirement: "SSO-05"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestProvisioningOffCreatesNothing — the users table is the same size after the refused sign-in as before it"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthRefusesAndFallsThrough (wave 3, still green: cfg.SSOProvision is false by default)"
        status: pass
      - kind: other
        ref: "mutation 4 — the provisioning switch removed: the users table grows, somebody is signed in, and 2 protocol rows are written. Red."
        status: pass
    human_judgment: false
  - id: D2
    description: "A provisioned account is created as an editor, never an administrator, so the role a group grants stays plan 10-05's decision"
    requirement: "SSO-05"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthProvisionsAnAccount/the_account_is_an_editor_and_never_an_administrator"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/admin/forwardauth.go | grep -c 'user.RoleAdmin' == 0"
        status: pass
      - kind: other
        ref: "mutation 2 — RoleEditor becomes RoleAdmin: the role subtest, the session-role subtest AND TestProvisionedAccountCannotReachASecondWebsite all red, because NewWebsiteAccessLookup returns true for an administrator two lines before it counts assignments. The gate flips 0 to 1."
        status: pass
    human_judgment: false
  - id: D3
    description: "A provisioned account is given HOLZCLOUD_SSO_DEFAULT_WEBSITE as its one assignment in the same request that creates it, and the claim is proved by the function that authorises requests rather than by a row count"
    requirement: "SSO-05"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestProvisionedAccountCannotReachASecondWebsite — calls NewWebsiteAccessLookup(database), the same constructor main.go hands auth.RequireWebsiteAccess, and requires false for the second website"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestProvisionedAccountCannotReachASecondWebsite — the control step: the one row deleted, the same call flips to true"
        status: pass
      - kind: other
        ref: "mutation 1 — the assignment never written: 'a freshly provisioned account reached a website it was never assigned to' — red"
        status: pass
      - kind: other
        ref: "mutation 7 — a row IS written and it names the other website: both directions red. A row-counting test would have passed."
        status: pass
    human_judgment: false
  - id: D4
    description: "A provisioning failure is a refused sign-in and never a partial account, because a surviving account with zero assignments is the inversion itself"
    requirement: "SSO-05"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestProvisioningLeavesNoAccountWhenTheAssignmentFails — the failure induced the way a real one arrives, a default website id no row carries"
        status: pass
      - kind: other
        ref: "sed the provisionSSOUser body | grep -c 'h.users.Delete' == 1"
        status: pass
      - kind: other
        ref: "mutation 3 — the compensation removed: 'the users table went from 0 to 1'. Red, and the gate flips 1 to 0."
        status: pass
      - kind: other
        ref: "mutation 10 — a failed provisioning answers 403: 'the next handler ran 0 times; want exactly 1 — a refusal is a fall-through'. Red, and wave 3's status gate flips 0 to 1."
        status: pass
    human_judgment: false
  - id: D5
    description: "Nobody can sign in to a provisioned account with a password, because no password was ever chosen; the account holds an Argon2id hash of 32 bytes from crypto/rand and no migration was needed"
    requirement: "SSO-05"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestNoPasswordSignsInAProvisionedAccount — the real HandleLogin, 4 subtests including the empty password and a control address no account carries"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestForwardAuthProvisionsAnAccount/the_stored_password_is_a_real_Argon2id_hash_and_not_the_empty_string"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestRandomSecretIsFreshAndLongEnough — 32 calls, all distinct, all at least auth.MinPasswordLength"
        status: pass
      - kind: other
        ref: "mutation 9 — randomSecret returns a constant: 'randomSecret repeated itself at call 1'. Red."
        status: pass
      - kind: other
        ref: "ls internal/db/migrations/*.sql | wc -l unchanged at 51 — 00001:6 declares password TEXT NOT NULL with no CHECK"
        status: pass
    human_judgment: false
  - id: D6
    description: "The random secret is never logged at any level"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestTheProvisioningSecretAppearsInNoLogLine — the log captured at Debug, every token of it offered to auth.VerifyPassword against the stored hash"
        status: pass
      - kind: other
        ref: "mutation 8 — the secret appended to slog.Info: the test names the leaked value verbatim. Red. The plan's own grep gate stayed at 0 under the same mutation; see Deviations."
        status: pass
    human_judgment: false
  - id: D7
    description: "An account created by a race resolves by re-reading rather than by failing"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestProvisioningResolvesADuplicateAddressByReReading — the second call returns the row the first one made and the users table does not grow"
        status: pass
      - kind: other
        ref: "mutation 6 — the duplicate branch disabled: 'the second provisioning refused instead of re-reading'. Red."
        status: pass
    human_judgment: false
  - id: D8
    description: "Provisioning never creates an account with an empty address — the constraint wave 3 discovered and handed forward"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestProvisioningNeverCreatesAnAccountWithAnEmptyAddress (2 subtests: through the middleware, and asked directly)"
        status: pass
      - kind: other
        ref: "mutation 5 — provisioning's guard removed: GREEN at first, investigated, and red after errSSOEmptyAddress made the guard's own contribution assertable"
        status: pass
      - kind: other
        ref: "mutation 5b — provisioning's guard AND user.Store.Create's guard removed: 'the users table went from 0 to 1 for an empty address'. Red. This is what proves the property is held at all."
        status: pass
    human_judgment: false
  - id: D9
    description: "Whether an operator wants provisioning at all, and whether one default website is the right shape for their installation rather than a per-group mapping from the first sign-in"
    verification: []
    human_judgment: true
    rationale: "Provisioning is off by default and a deliberate act. An operator who switches it on gets every person their Authentik authenticates as an editor of one website — which is the safe answer and may not be the useful one for an installation with several sites and several teams. Plan 10-05 is where a group decides more than that; plan 10-08's DEPLOY.md is where the operator is told what switching it on means, including that T-10-27 (uncontrolled account creation) is accepted rather than mitigated. Plan 10-10's browser pass is where a human watches a real first sign-in create an account."

duration: 15 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 04: The Seam, Closed and Asked About Summary

**One branch where wave 3 left a refusal, and one function that creates an account and writes its single website assignment without a request between them — because `NewWebsiteAccessLookup` reads zero rows in `user_websites` as access to every website, which is right for accounts made by hand and inverts the moment accounts are made automatically — proved by asking that very function whether the new account may enter a second website and requiring false, with a control step that deletes the one row and watches the same call flip to true.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-08T04:25Z (approximate: the first commit landed at 04:31:11Z, after the reading)
- **Completed:** 2026-09-08T04:41Z
- **Tasks:** 2
- **Files:** 0 created, 2 modified

## Accomplishments

- **The assignment is written in the same function as the creation, and the comment says why they cannot be separated.** `provisionSSOUser` calls `h.users.Create` and `h.users.SetRights` with nothing between them that can return, branch or hand control to a handler. The D-01 paragraph above the branch is carried whole rather than paraphrased: it names `NewWebsiteAccessLookup`, quotes `return assigned == 0 || mine > 0`, says that a new account has zero rows *by construction*, and says that this is why the assignment is not left to plan 10-05's group synchronisation.
- **The proof goes through the function that authorises requests, and mutation 7 is why that matters.** `TestProvisionedAccountCannotReachASecondWebsite` builds `NewWebsiteAccessLookup(database)` — the same constructor `main.go` hands `auth.RequireWebsiteAccess` — and asks it about a second website. Under mutation 7 the assignment *is* written, exactly one row exists, and it names the wrong website: **a row-counting test would have passed.** The lookup answered wrong in both directions and the test named both.
- **The control step is not decoration and it did its job.** Deleting the one row makes the same call flip to true. Without it the test would pass on a tree where provisioning never ran, where the assignment went to the wrong id, or where the lookup was stubbed.
- **`NewWebsiteAccessLookup` was not touched.** `git diff d22b87f..HEAD -- internal/admin/handler.go` is empty. It is cited by name eleven times across the two files and never by line number.
- **A provisioned account is an editor, and mutation 2 shows what an administrator would have cost.** With `user.RoleAdmin` in its place, the second-website test goes red too — because the lookup returns true for an administrator two lines before it counts any assignment. Creating an admin here would have skipped the entire defence this plan is about, silently, with every other test still green.
- **A failed assignment leaves nothing behind**, induced the way a real one arrives: a `HOLZCLOUD_SSO_DEFAULT_WEBSITE` naming no row, which `user_websites`' foreign key refuses.
- **The wave-3 constraint was honoured and then found to be already held three times over — which is not the same as being asserted.** See the mutation section; this is the plan's one real finding.
- **Ten mutations, ten reds**, each against a **committed** file, restored with `git checkout --`, one of them only after an investigation that changed the code.

## Task Commits

1. **Task 1 (RED): failing tests for provisioning and the inversion** — `80f6904` (test)
2. **Task 1 (GREEN): the branch, the helper, the compensation** — `570f7d6` (feat)
3. **Task 1 (follow-up): the empty-address guard asserted by name** — `5280ba8` (test)

_Task 1 carried `tdd="true"`. The RED commit's tests do not compile against the pre-change tree — `h.provisionSSOUser undefined`, `undefined: randomSecret` — which is the intended failure and the same shape wave 3 recorded._

**Task 2 produced no commit of its own.** Its own action says "No production code. One test, plus the arithmetic", and both of its tests (`TestProvisionedAccountCannotReachASecondWebsite`, `TestProvisioningOffCreatesNothing`) were written in the Task 1 RED commit, because the plan's Task 1 behaviour list asks for the same assertion in its fourth bullet. Splitting one test across two commits to match the task boundary would have made the RED commit incomplete. Its output is the two tables below.

Commit 3 is a follow-up to Task 1 rather than a new task: it was forced by mutation testing the committed Task 1, and it is the same shape wave 3's commits 4 and 5 were.

## Files Modified

- `internal/admin/forwardauth.go` (203 → 403 lines) — `errSSOEmptyAddress`, the fourth refusal reason `ssoRefuseProvisionFailed`, the D-01 paragraph and the provisioning branch inside `ForwardAuthSignIn`, `provisionSSOUser`, `randomSecret`, and two corrected comments
- `internal/admin/forwardauth_test.go` (619 → 1167 lines) — `newProvisioningAdmin`, `accountByEmail`, `assignedWebsites`, `inSession`, and 10 new test functions carrying 15 named subtests

## The one real finding: a mutation that stayed green, and what was under it

**Mutation 5 removed `provisionSSOUser`'s empty-address guard and the whole suite stayed green.** Both halves of `TestProvisioningNeverCreatesAnAccountWithAnEmptyAddress` passed, including the direct-call half written specifically to catch this.

Stopping there would have been defensible and wrong — it is the same first observation wave 3 made about *its* empty-address guard, and there the answer was that the guard was load-bearing and under-tested. Here the answer is different, and finding out which of the two it was is the whole point.

**Three independent layers refuse the empty address:**

1. `ForwardAuthSignIn` step 4, wave 3's guard — an identity with no address never reaches provisioning at all;
2. `provisionSSOUser`'s own guard, written by this plan;
3. `user.Store.Create` at `internal/user/store.go:174` — `if email == "" { return 0, errors.New("email is required") }`.

Remove any one and the other two still refuse, so **no behavioural assertion can say which one did the work.** The direct-call half stayed green on layer 3's validation error.

**Mutation 5b removed layers 2 and 3 together**, and the property really does break:

```
--- FAIL: TestProvisioningNeverCreatesAnAccountWithAnEmptyAddress
    --- FAIL: …/asked_directly,_provisioning_refuses_the_empty_address_itself (0.54s)
        forwardauth_test.go:997: provisionSSOUser accepted the empty address; it would create the row TestForwardAuthNeverMatchesAnEmptyAddress seeds, and the next identity with no e-mail header would sign in as it
        forwardauth_test.go:1002: provisionSSOUser returned account 1 for the empty address
        forwardauth_test.go:1005: the users table went from 0 to 1 for an empty address
```

So the constraint wave 3 handed forward **is** satisfied, and it is falsifiable. What was not assertable was provisioning's own contribution to it — and a guard whose removal nothing notices reads as tidiness and gets deleted by the next person tidying. The fix was neither to delete it nor to wave it through: `errSSOEmptyAddress` is now a sentinel, the direct-call subtest asserts `errors.Is`, and mutation 5 is red on its own:

```
--- FAIL: TestProvisioningNeverCreatesAnAccountWithAnEmptyAddress
    --- FAIL: …/asked_directly,_provisioning_refuses_the_empty_address_itself (0.53s)
        forwardauth_test.go:1008: err = provision: create the account: email is required; want errSSOEmptyAddress. Provisioning must refuse the empty address itself, not lean on user.Store.Create happening to validate it
```

That failure message is also the finding, stated as a test: **layer 3 is a validation message, not a security guard.** Nothing marks `email is required` as load-bearing for account safety, and an installation that one day wants an address-less service account would relax it without knowing what it was holding up.

## Mutation Verification

Every guard was removed, inverted or redirected on a **committed** file, run against a named test, restored with `git checkout --`, and `git status --short` confirmed empty afterwards. **Ten mutations, ten reds.** Verbatim output.

**1. The assignment is never written — the mutation this whole plan exists to prevent**
```
--- FAIL: TestForwardAuthProvisionsAnAccount (0.56s)
    --- FAIL: TestForwardAuthProvisionsAnAccount/the_account_belongs_to_the_default_website_and_to_nothing_else (0.00s)
        forwardauth_test.go:752: user_websites = []; want exactly [2]. An empty list is the inversion itself
--- FAIL: TestProvisionedAccountCannotReachASecondWebsite (0.55s)
    forwardauth_test.go:1101: a freshly provisioned account reached a website it was never assigned to ("Die fremde Seite", id 3) — NewWebsiteAccessLookup read zero assignments as every website
```

**2. Provisioning creates an administrator (`user.RoleEditor` → `user.RoleAdmin`)**
```
--- FAIL: TestForwardAuthProvisionsAnAccount (0.56s)
    --- FAIL: …/the_account_is_an_editor_and_never_an_administrator (0.00s)
        forwardauth_test.go:737: role = "admin"; want "editor" — what a group grants is plan 10-05's decision, and NewWebsiteAccessLookup returns true for an administrator before it counts any assignment
    --- FAIL: …/the_request_that_created_it_is_the_request_that_signs_it_in (0.00s)
        forwardauth_test.go:765: session user_role = "admin"; want "editor"
--- FAIL: TestProvisionedAccountCannotReachASecondWebsite (0.56s)
    forwardauth_test.go:1101: a freshly provisioned account reached a website it was never assigned to ("Die fremde Seite", id 3) — NewWebsiteAccessLookup read zero assignments as every website
```
```
and the plan's gate (want 0): 1
```
**The third failure is the one worth reading.** The assignment was written correctly under this mutation and the row is there; the account reaches every website anyway, because the role short-circuits the count. This is T-10-23 as a test rather than as an argument.

**3. The compensation removed (`h.users.Delete` disabled)**
```
--- FAIL: TestProvisioningLeavesNoAccountWhenTheAssignmentFails (0.58s)
    forwardauth_test.go:903: the users table went from 0 to 1; a failed provisioning leaves nothing behind, because what it would leave behind is an account with access to every website
```
```
and the plan's gate (want 1): 0
```

**4. The provisioning switch removed (`!h.cfg.SSOProvision` → `false`)**
```
--- FAIL: TestProvisioningOffCreatesNothing (0.55s)
    forwardauth_test.go:1135: the users table went from 0 to 1 with HOLZCLOUD_SSO_PROVISION off
    forwardauth_test.go:1138: provisioning is off and somebody was signed in as 1
    forwardauth_test.go:1149: wrote 2 protocol rows; want exactly 1 refusal
```

**5. Provisioning's own empty-address guard removed — the mutation that stayed GREEN**
```
ok  	github.com/holzcloud/holzcloud-cms/internal/admin	3.681s
```
Investigated rather than accepted; see the section above. Red after `errSSOEmptyAddress`, output quoted there.

**5b. Provisioning's guard AND `user.Store.Create`'s guard removed** — output quoted above. This is the mutation that proves the property is held at all rather than merely asserted.

**6. A duplicate address is refused instead of re-read**
```
--- FAIL: TestProvisioningResolvesADuplicateAddressByReReading (0.55s)
    forwardauth_test.go:943: the second provisioning refused instead of re-reading: provision: create the account: a user with that email already exists
```

**7. A row IS written, and it names the other website (`SSODefaultWebsite` → `SSODefaultWebsite + 1`)**
```
--- FAIL: TestForwardAuthProvisionsAnAccount (0.55s)
    --- FAIL: …/the_account_belongs_to_the_default_website_and_to_nothing_else (0.00s)
        forwardauth_test.go:752: user_websites = [3]; want exactly [2]. An empty list is the inversion itself
--- FAIL: TestProvisionedAccountCannotReachASecondWebsite (0.54s)
    forwardauth_test.go:1096: NewWebsiteAccessLookup refuses the provisioned account its own default website ("Die eigene Seite", id 2); it was assigned that website in the request that created it
    forwardauth_test.go:1101: a freshly provisioned account reached a website it was never assigned to ("Die fremde Seite", id 3) — NewWebsiteAccessLookup read zero assignments as every website
```
**This is the mutation that justifies the test's shape.** `SELECT COUNT(*) FROM user_websites WHERE user_id = ?` returns 1 under it. A test that counted rows would have been green while a provisioned account had access to exactly the website it must not have and none of the one it must.

**8. The provisioning secret written to the log**
```
--- FAIL: TestTheProvisioningSecretAppearsInNoLogLine (0.56s)
    forwardauth_test.go:1072: the provisioning secret was written to the log: "CavANF6kOTgBv4fxSeS-L8EwlawAmjonWHqMaQRdkdc"
```
```
and the plan's own grep gate (want 0): 0   <- GREEN under the mutation
```
See Deviations: the gate reads a proxy and the test does not.

**9. `randomSecret` returns a constant**
```
--- FAIL: TestRandomSecretIsFreshAndLongEnough (0.00s)
    forwardauth_test.go:1032: randomSecret repeated itself at call 1; every provisioned account gets its own
```

**10. A failed provisioning answers 403 instead of falling through**
```
--- FAIL: TestProvisioningLeavesNoAccountWhenTheAssignmentFails (0.56s)
    forwardauth_test.go:911: the next handler ran 0 times; want exactly 1 — a refusal is a fall-through
```
```
and wave 3's status gate (want 0): 1   <- red
```

After every mutation the file was restored with `git checkout --` and `git status --short` confirmed empty.

## The counting gates, measured against the post-change tree

**Baseline measured at 2026-09-08T04:29Z, on this tree, immediately before the first commit of this plan — not taken from the plan.** The plan's note above its own table says these rows are deltas, and two of its literal numbers had moved again since it was written.

| What | Command | Plan's number | Measured baseline | This plan adds | Measured after | Verdict |
|---|---|---|---|---|---|---|
| `SetRights` call sites | `grep -rn 'SetRights(' internal/ --include='*.go' \| grep -v _test \| grep -vc 'func (s \*Store) SetRights'` | 2 → 3 | **2** | 1 | **3** | exactly as predicted |
| `NewWebsiteAccessLookup` mentions | `grep -rn NewWebsiteAccessLookup internal/ cmd/ --include='*.go' \| wc -l` | 7 → 8 | **18** | 11 | **29** | baseline and delta both diverge; see below |
| `user.RoleAdmin` outside comments | `grep -v '^[[:space:]]*//' internal/admin/forwardauth.go \| grep -c user.RoleAdmin` | 0 → 0 | **0** | 0 | **0** | exactly as predicted |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | vorher (gate says 49) | **51** | 0 | **51** | delta 0; the literal 49 is stale twice over |
| files in `internal/admin` | `ls internal/admin/*.go \| grep -v _test \| wc -l` | vorher + 1 | **52** | 0 | **52** | delta 0 — this plan creates no file; see below |
| strings in source | `go run ./tools/i18n \| head -1` | vorher | **1318** | 0 | **1318** | delta 0 as required |
| `adminProtectedMux.Handle` | `grep -c adminProtectedMux.Handle cmd/holzcloud/main.go` | vorher | **159** | 0 | **159** | delta 0 as required |
| `completeLogin` call sites | wave 3's standing invariant | 4 | **4** | 0 | **4** | delta 0; the AST test still green |

**`SetRights` at 3 is the row the plan asked to be stated, and its description needs one correction.** The plan says the two baseline call sites are "the user form and the bundle import". They are both the user form:

```
internal/admin/user.go:230   h.users.SetRights(r.Context(), newID, rightsFromForm(r, role))   — the create form
internal/admin/user.go:321   h.users.SetRights(r.Context(), id,    rightsFromForm(r, role))   — the edit form
internal/admin/forwardauth.go:308  h.users.SetRights(ctx, id, rights)                        — provisioning
```

There is no bundle-import call site on this tree. The count and the invariant are unaffected: **a fourth appearing in this phase before plan 10-05 means the assignment is being written in two places.**

**`NewWebsiteAccessLookup` mentions diverge in both columns, and neither divergence is a defect.** The plan's baseline of 7 was measured on 2026-09-07; the tree carries **18**, because phase 11's `album_scope_test.go` (3) and the phase-9 `product_scope_test.go` / `order_scope_test.go` families cite it. The delta is **+11** rather than +1 because the command counts *lines mentioning the name* and the plan was counting the new *call*: this plan makes exactly **one** call (`NewWebsiteAccessLookup(database)` in the inversion test) and names the function ten more times in comments — deliberately, because waves 1-3 established that this function is cited by name and never by line number.

### `go run ./tools/i18n`, before and after

```
before:  1318 Zeichenketten im Quelltext
         en.json  1277 übersetzt, 41 offen, 0 verwaist   (es, fr, it identical)
after:   1318 Zeichenketten im Quelltext
         en.json  1318 übersetzt,  0 offen, 0 verwaist   (es, fr, it identical)
delta:   0 strings by this plan, 0 newly open, 0 orphaned
```

**The 41 open strings closing is not this plan's doing and must not be credited to it.** They were closed by `322f853 i18n(11-07): die einundvierzig Zeichenketten der Galerie, in vier Sprachen`, which the concurrent plan-11-07 agent committed between this plan's GREEN commit and its follow-up — exactly as the executor prompt predicted. **This plan neither added a string nor closed one.** Everything it does is invisible: a person provisioned on their first sign-in sees the same dashboard, a person refused sees the same login form, and `ssoRefuseProvisionFailed` is an `slog` field value that is never rendered.

## Decisions Made

- **`provisionSSOUser` takes `*http.Request`, not the plan's `context.Context`.** The `user.create` protocol row is written inside the function that creates the account, and `h.LogActivity` takes the request. Writing the row in the caller instead would put the record and the creation in different functions, which is precisely the drift this file's own header comment describes for the sign-in funnel. `ctx := r.Context()` on the first line.
- **A failed provisioning writes an `auth.login_fail` row; a failed account lookup still does not.** The difference is not inconsistency. The lookup branch is reached when the database has stopped answering a read, so a write is hopeless and the second failure would be the one in the log. The commonest way to reach the provisioning branch is a `HOLZCLOUD_SSO_DEFAULT_WEBSITE` naming a website that has since been deleted — a perfectly healthy database and an operator who has no other way to learn that people are being turned away. `activity.Store.Log` never returns an error by design, so this cannot turn a refusal into a 500 in the other case either.
- **`errSSOEmptyAddress` is a sentinel.** See the finding above; this was not a style choice but the only way the guard's own removal fails.
- **Two comments that this change made untrue were corrected in the same commit.** `ForwardAuthSignIn`'s step list said "5. the account lookup, which creates nothing", and step 5's inline comment said "provisioning is a later plan and lands as a second branch at this point". Both are now false. Wave 3's own rule: a comment that has quietly become untrue is worse than a failing test, because nothing reports it.
- **The failure in `TestProvisioningLeavesNoAccountWhenTheAssignmentFails` is induced through the foreign key rather than through a fake store.** `user_websites.website_id REFERENCES websites(id)` and `foreign_keys=ON`, so a default website id naming no row makes the insert fail — which is the shape a real operator error takes, not an invented one.
- **`newProvisioningAdmin` sets `h.users.Params = cheapHashing`.** The package's ordinary fixture hands the user store the zero `auth.Argon2Params`, and `argon2.IDKey` panics rather than hashing on `Iterations < 1`. `cheapHashing` already exists in `menu_scope_test.go` for the same reason.

## Deviations from Plan

### Auto-fixed

**1. [Rule 2 — Missing critical] Provisioning's empty-address guard had no test that could fail**

- **Found during:** Task 1 mutation testing (mutation 5, which stayed green), against the committed file
- **Issue:** Three layers refuse the empty address and no behavioural assertion could distinguish them, so removing the one this plan wrote changed nothing observable. The executor's constraint from wave 3 was to *assert* that provisioning can never create such an account; the assertion existed and was passing for the wrong reason.
- **Fix:** `errSSOEmptyAddress` as a package-level sentinel, returned by the guard; the direct-call subtest asserts `errors.Is(err, errSSOEmptyAddress)` with a message that names why "some error" is not enough.
- **Files modified:** `internal/admin/forwardauth.go`, `internal/admin/forwardauth_test.go`
- **Verification:** mutation 5, verbatim above — and mutation 5b, which proves the property itself is real.
- **Committed in:** `5280ba8`

**2. [Rule 1 — Bug] Two comments in `ForwardAuthSignIn` became untrue with this change**

- **Found during:** Task 1 GREEN
- **Issue:** The function's step list said step 5 "creates nothing", and step 5's own comment said provisioning "is a later plan". This plan is that later plan.
- **Fix:** Step 5 of the list now describes the creation and the assignment together; step 5's comment says the lookup creates nothing and that the branch below it decides, off by default.
- **Files modified:** `internal/admin/forwardauth.go`
- **Verification:** read against the body; `go build`, `go vet`, `gofmt -l` all clean.
- **Committed in:** `570f7d6`

### Divergences recorded rather than worked around

**3. `provisionSSOUser` takes `*http.Request` rather than the plan's `ctx context.Context`.** Reason under "Decisions Made" — the same family as wave 3's fourth argument to `refuseSSO`.

**4. A provisioning failure writes a protocol row, which the plan does not specify either way.** Reason under "Decisions Made". Asserted by `TestProvisioningLeavesNoAccountWhenTheAssignmentFails`.

**5. Task 2 produced no commit,** because both of its tests are also the fourth bullet of Task 1's behaviour list and were therefore written in the RED commit. Splitting one test across two commits to match the task boundary would have made the RED commit an incomplete RED.

**6. The plan's `secret`-in-the-log grep gate reads a proxy, and mutation 8 proved it.** The gate is

```
grep -rn 'secret\|password' internal/admin/forwardauth.go | grep -c 'slog.'
```

It counts lines that name the secret **and** name `slog.` on the same line. `gofmt` wraps a long `slog.Info` across three lines, so appending `"initial_credential", secret` to its last line puts the secret in the log while the gate still prints **0**. Verified: under mutation 8 the gate was green and `TestTheProvisioningSecretAppearsInNoLogLine` was red, naming the leaked value verbatim. This is 10-CONTEXT's "a gate that proves a proxy is not a gate" for the second time in this phase. **The gate is kept** — it is free and it catches the single-line case — but the behavioural test is what holds the property, and the summary says which is which rather than reporting the green gate as evidence.

**7. Two of the plan's literal `verify` gates are stale against this tree**, the family waves 1, 2 and 3 all recorded:
- `ls internal/db/migrations/*.sql | wc -l` is specified to fail unless it prints **49**; the tree has **51**. Delta 0.
- The counting table's `files in internal/admin` row says "This plan adds 0" and then "After: **vorher + 1**". The two cells contradict each other; the "adds 0" cell is right and the "after" cell is a copy of wave 3's row, where a file really was created. Measured 52 → 52.

**8. The `NewWebsiteAccessLookup` baseline of 7 is stale by 11 and its delta prediction measures a different thing.** Both numbers are written down above with the cause. Neither is adjusted away.

**9. The plan's description of the two existing `SetRights` call sites is wrong** — "the user form and the bundle import" are both the user form (create and edit). The count is right and the invariant is unaffected. Named above.

---

**Total deviations:** 2 auto-fixed (1 Rule 2, 1 Rule 1) + 7 divergences recorded.
**Impact on plan:** No scope creep. Deviation 1 adds a sentinel error and one assertion — no behaviour change on any path a request can take. Deviation 2 changes comments only. Nothing in `internal/admin/handler.go`, `internal/user/`, `internal/auth/` or `cmd/holzcloud/` was touched.

## Verification Results

**This run is from a tree whose `git status --short` was empty before and after and whose `HEAD` did not move during it** — `5280ba8` at 04:37:26Z and `5280ba8` at 04:39:12Z. Wave 3 recorded suites failing in packages it never touched because the concurrent plan-11-07 agent was mid-commit; `322f853` landed in this tree between this plan's second and third commits, so the caveat is live and the run below is the one being quoted.

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | silent |
| `gofmt -l .` | silent |
| `go test ./...` | **0 failures** |
| `go test ./internal/admin/ -run 'ForwardAuth\|Provision\|RandomSecret' -v` | 20 test functions, all PASS (10 from wave 3, 10 new, 15 named subtests) |
| `go test ./internal/admin/ -run 'ProvisionedAccountCannotReachASecondWebsite\|ProvisioningOffCreatesNothing' -v` | 2 tests, both PASS |
| `sed` the `provisionSSOUser` body `\| grep -v '//' \| grep -c user.RoleAdmin` | **0** ✓ |
| `sed` the `provisionSSOUser` body `\| grep -c SetRights` | **1** ✓ |
| `sed` the `provisionSSOUser` body `\| grep -c h.users.Delete` | **1** ✓ |
| `grep -v '//' internal/admin/forwardauth.go \| grep -c user.RoleAdmin` | **0** ✓ |
| `grep -n 'secret\|password' … \| grep -c 'slog.'` | **0** ✓ — but see deviation 6: this gate is green under mutation 8 as well |
| `grep -c NewWebsiteAccessLookup internal/admin/forwardauth_test.go` | **8** (≠ 0) ✓ |
| `grep -c user_websites internal/admin/forwardauth_test.go` | **9** (≠ 0) ✓ |
| `grep -rn 'SetRights(' … \| grep -vc 'func (s \*Store) SetRights'` | **3** ✓ |
| `git diff d22b87f..HEAD -- internal/admin/handler.go` | **empty** ✓ — `NewWebsiteAccessLookup` untouched |
| `ls internal/db/migrations/*.sql \| wc -l` | 51 — the plan's literal gate expects 49 and is stale twice over; **delta 0** |
| `go run ./tools/i18n \| head -1` | `1318 Zeichenketten im Quelltext` before **and** after; **delta 0** |

## Issues Encountered

- **A mutation that stayed green was the plan's most useful five minutes**, and it did not end where wave 3's did. Wave 3's green mutation revealed an under-tested guard; this one revealed a well-covered property with an unassertable guard. Both first observations look identical from outside — `ok`, no output — and only the investigation distinguishes them. Recording "mutation 5: green, guard appears redundant" would have been true, cheap, and would have left `errSSOEmptyAddress` unwritten and the next tidier free to delete the guard.
- **The plan's own log gate is green under the mutation it exists to catch.** Reported in full under deviation 6 rather than as a passing row in the table. This is the second gate-measures-a-proxy finding in this phase.
- **A concurrent agent committed into this tree during execution** (`322f853`, plan 11-07). Every commit here was staged file by file and verified with `git show --stat`; all three touch only this plan's two files. The final suite run is quoted with its HEAD before and after and a clean `git status` at both ends.
- **`newTestAdmin` hands the user store zero `Argon2Params`**, which panic rather than hash. Anything in package `admin` that reaches `user.Store.Create` in a test needs `cheapHashing`. Not a defect of this plan and worth knowing for plan 10-05, which will create and re-assign accounts in the same package.

## Threat Flags

None new. Every `mitigate` disposition in the plan's register has a passing test and a red mutation:

| Threat | Covered by |
|---|---|
| T-10-22 a provisioned account with zero rows in `user_websites` | mutations 1 and 7, both asked of `NewWebsiteAccessLookup`; the control step; mutation 3 for the half-finished case |
| T-10-23 provisioning creating an administrator | mutation 2 — and its third failure line, which shows the account reaching every website *with the assignment correctly written* |
| T-10-24 a provisioned account reachable by password | `TestNoPasswordSignsInAProvisionedAccount` through the real `HandleLogin`, 4 subtests; mutation 9 |
| T-10-25 the random secret in a log line | `TestTheProvisioningSecretAppearsInNoLogLine`; mutation 8. **The plan's grep gate does not cover this** — see deviation 6 |
| T-10-26 a race creating two accounts for one identity | mutation 6; `TestProvisioningResolvesADuplicateAddressByReReading` |
| T-10-27 uncontrolled account creation | accepted by the plan, unchanged. Belongs in `DEPLOY.md` (plan 10-08) |
| T-10-SC package-manager installs | no install task; every symbol is Go standard library or already in `go.mod` |

**One surface worth carrying forward that the register does not name.** `user.Store.Delete` refuses to remove the last administrator (`user.ErrLastAdmin`). The compensation therefore cannot fail *for that reason* here, because provisioning creates editors — but if plan 10-05 ever promotes an account on creation, the compensating delete would begin to fail exactly on the account that most needs removing, and the `slog.Error` naming the id would become the only record. Provisioning creating an editor is what keeps that path closed, and it is now gated twice.

## Known Stubs

None. Everything this plan builds is reachable, wired and tested.

`ssoRefuseProvisionFailed` is a new value in an existing set of reason codes and is written on a live path — `TestProvisioningLeavesNoAccountWhenTheAssignmentFails` reaches it. It is not a stub.

## User Setup Required

None from this plan. Three things it hands to plan 10-08 (`DEPLOY.md`), all of which an operator will otherwise meet as a surprise:

1. **`HOLZCLOUD_SSO_PROVISION` means every person the operator's Authentik authenticates gets an account here** — an editor of `HOLZCLOUD_SSO_DEFAULT_WEBSITE` and of nothing else. That is T-10-27, accepted rather than mitigated, and the boundary is the operator's own directory.
2. **If `HOLZCLOUD_SSO_DEFAULT_WEBSITE` names a website that is later deleted, every first sign-in is refused** and lands at the password form. The evidence is one `auth.login_fail` row per attempt in `/admin/protokoll` and a `forward auth provisioning` line in the server log. Plan 10-01 checks the website exists at start-up; nothing rechecks it afterwards, and nothing should — the alternative is a query per sign-in for a case that is an operator error.
3. **A provisioned account has no password and cannot be given one from the login form.** The operator sets one through the user administration if that person ever needs to sign in without the identity provider.

## Next Phase Readiness

- **Plan 10-05 (group rights)** is the one that must not undo this. It re-applies rights on every sign-in, so it owns D-01's *second* road — an existing account whose groups no longer map to any configured website getting an **empty** `user_websites` write and reaching "zero means every website" **by subtraction**. `SetRights` replaces wholesale, which is what makes 10-05 idempotent and also what makes that subtraction one line away. **The assertion shape is already written here:** `TestProvisionedAccountCannotReachASecondWebsite`'s control step is exactly the state 10-05 must refuse to produce, and `NewWebsiteAccessLookup` is exactly the function to ask.
- **`SetRights` must stay at exactly 3 call sites for the rest of this phase.** A fourth means the assignment is written in two places. 10-05 should re-use `provisionSSOUser`'s call or move it, not add one.
- **`completeLogin` is still at 4** and wave 3's AST test still holds it there.
- **`provisionSSOUser` creates an editor,** and plan 10-05's group mapping is where a role other than editor may be decided. Note the interaction under Threat Flags: `user.Store.Delete` refuses to remove the last administrator, so promoting on creation would give the compensation a way to fail.
- **Carry these baselines forward, measured on this tree at 04:40Z:** migrations **51**, admin templates **68**, source strings **1318** with **0 offen** (11-07 closed the 41), `adminProtectedMux.Handle` **159**, `internal/admin` non-test files **52**, `completeLogin` call sites **4**, `SetRights` call sites **3**, `NewWebsiteAccessLookup` mentions **29**.
- **`newTestAdmin`'s zero `Argon2Params` panic** is worth knowing before 10-05 writes its first test that creates an account: use `cheapHashing`, as `newProvisioningAdmin` and `menu_scope_test.go` both do.
- **For plan 10-10 (the browser pass):** drive a first sign-in with `HOLZCLOUD_SSO_PROVISION=true` and then try to reach a *second* website from the address bar. Every automated test asserts the lookup refuses it; what no test can assert is what the refused person sees, and whether it reads as an error or as a dead end.
- **Do not cite `internal/admin/handler.go` by line number.** `NewWebsiteAccessLookup`'s final line has been `:173`, `:178` and `:183` in three documents of this phase. Everything this plan wrote names the symbol.

## Self-Check: PASSED

- `internal/admin/forwardauth.go` — FOUND, contains `provisionSSOUser`, `randomSecret`, `errSSOEmptyAddress`, `SetRights` (1×), `h.users.Delete` (1×), `user.RoleAdmin` outside comments (0×)
- `internal/admin/forwardauth_test.go` — FOUND, contains `NewWebsiteAccessLookup` (8×), `user_websites` (9×)
- `internal/admin/handler.go` — FOUND and **unchanged**: `git diff d22b87f..HEAD -- internal/admin/handler.go` empty
- Commit `80f6904` — FOUND
- Commit `570f7d6` — FOUND
- Commit `5280ba8` — FOUND
- `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` — all clean at 04:39:12Z on an unmoving `5280ba8`

---
*Phase: 10-authentik*
*Completed: 2026-09-08*
