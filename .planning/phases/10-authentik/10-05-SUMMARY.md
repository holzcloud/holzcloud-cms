---
phase: 10-authentik
plan: 05
subsystem: auth
tags: [forward-auth, authentik, sso, group-mapping, rbac, website-isolation, privilege-escalation, activity-log]

requires:
  - phase: 10-authentik
    provides: "plan 10-02's web.Identity.HasGroup — the whole-element comparison, written one wave early so that this plan could not reach for strings.Contains — and splitGroups, which drops empty elements"
  - phase: 10-authentik
    provides: "plan 10-01's config.SSOAdminGroup (no default) and config.SSOWebsiteGroups (an explicit group=websiteID map, chosen because the websites table has no slug column)"
  - phase: 10-authentik
    provides: "plan 10-03's ForwardAuthSignIn, whose step 6 this plan inserts itself into, and plan 10-04's provisionSSOUser, whose freshly assigned account this plan must not immediately refuse"
provides:
  - "admin.(*Handler).syncRightsFromGroups: the whole of SSO-06 in one function — the role from the groups, the websites from the configured map, one protocol row per actual change"
  - "admin.errSSONoWebsiteGroup: the sentinel that makes the zero-website refusal distinguishable from the four refusals that look identical from outside"
  - "The refusal that closes D-01's SECOND road: an editor whose groups map to no configured website is refused the sign-in, never written as an empty assignment"
  - "ssoRefuseNoWebsiteGroup and ssoRefuseSyncFailed: the fifth and sixth refusal reason codes"
  - "The unconfigured-mapping branch: with HOLZCLOUD_SSO_WEBSITE_GROUPS unset the website half does not run at all, because refusing on 'your groups match no configured website' when nothing is configured refuses everybody"
affects: [10-06-second-factor, 10-07-admin-surface, 10-08-deploy-docs, 10-10-browser-pass]

actuals:
  tokens: 34000
  tasks: 2
  commits: 6

tech-stack:
  added: []
  patterns:
    - "A green mutation is a question, not a pass — twice in one plan, and each time the answer was a different one: mutation 2 found a property held in another package, mutation 10 found a property nothing held at all"
    - "The load-bearing assertion is made against NewWebsiteAccessLookup, the function that authorises a real request, and never against a row count in user_websites"
    - "A control step that empties the rows by hand and watches the same lookup flip, so the test demonstrates that the property it asserts is the one that would break"
    - "A sentinel error where several layers refuse the same input, so that one layer's own contribution is assertable with errors.Is rather than only visible when all of them go"
    - "A configured-but-empty setting grants to nobody; an unconfigured setting does not decide at all. The two are different questions and this plan needed both answers."

key-files:
  created: []
  modified:
    - internal/admin/forwardauth.go
    - internal/admin/forwardauth_test.go

key-decisions:
  - "The role is written BEFORE the zero-website refusal, not after. Refusing first would leave an operator who removed somebody from every group — administration and websites alike — with a stale administrator in users.role forever, reachable from the password form. The demotion is real and correct on its own; the refusal is about the websites."
  - "With HOLZCLOUD_SSO_WEBSITE_GROUPS unset the website half of the synchronisation does not run. The plan's literal rule — an empty resulting slice is a refusal — locks out every editor on an installation that configures no group mapping, including the account plan 10-04's provisioning had assigned one line earlier. The invariant that actually matters is narrower and still holds: this function never WRITES an empty assignment, and writing nothing cannot."
  - "syncRightsFromGroups takes both ctx and *http.Request, as the plan's signature specifies: the store calls take a context and h.LogActivity takes the request. This is one convention away from provisionSSOUser, which takes only the request; the divergence is the plan's and is recorded rather than silently harmonised."
  - "Two new refusal reason codes rather than one. errSSONoWebsiteGroup is an operator's misconfiguration or a deliberate removal of access; ssoRefuseSyncFailed is a database that stopped answering. They land in different places in an operator's day and the server log says which."
  - "The tests were renamed from TestGroupSync… to TestSyncRights… because the plan's own verification gate runs -run 'ForwardAuth|SyncRights|Groups' and matched none of them."

patterns-established:
  - "A counting gate that its own explanation satisfies is not a gate — the plan applied grep -v '//' to the strings.Contains gate and forgot it on the HasGroup gate beside it. Third instance in this phase of 10-CONTEXT's 'a gate must measure what its name claims'."
  - "A gate's -run pattern is part of the gate. This plan's pattern matched none of the twelve tests the plan asked for, and the tests were renamed rather than the divergence merely recorded."
  - "Cite functions, never line numbers — carried from waves 1-4. Nothing this plan wrote cites a line in internal/admin/handler.go."
  - "Commit the task before mutating it — carried from wave 1, applied to all twelve mutations here."

requirements-completed: [SSO-06]

coverage:
  - id: D1
    description: "Group membership decides the role and is re-applied on EVERY sign-in, so a demotion at the identity provider takes effect here at the next sign-in rather than at the next session expiry"
    requirement: "SSO-06"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsAppliesADemotionAtTheNextSignIn — users.role, the session role and exactly one user.update row"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsPromotionWritesOneProtocolRowNamingFromAndTo — the metadata names role, editor, admin and sso"
        status: pass
      - kind: other
        ref: "mutation 8 — completeLogin handed u.Role instead of the synchronised role: three subtests red. Without the compiler silenced it does not even build."
        status: pass
    human_judgment: false
  - id: D2
    description: "A group is matched as a WHOLE element of the pipe-separated header: the account in not-holzcloud-admins is not an administrator"
    requirement: "SSO-06"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsMatchesAGroupAsAWholeElement — 9 cases, of which not-holzcloud-admins, holzcloud-admins-x and xholzcloud-adminsx are the three a strings.Contains implementation gets wrong"
        status: pass
      - kind: other
        ref: "mutation 1 — strings.Contains over the joined header: exactly those three subtests red, the other six green. The plan's gate flips 0 to 1."
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/admin/forwardauth.go | grep -c 'strings.Contains' == 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "HOLZCLOUD_SSO_ADMIN_GROUP has no default, and an empty value grants administration to nobody rather than to everybody"
    requirement: "SSO-06"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsMatchesAGroupAsAWholeElement — three cases with the configured group empty, including a header carrying empty elements"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsRefusesAnEmptyAdministrationGroupOnItsOwn — a direct call with an Identity built by hand, the shape splitGroups never produces"
        status: pass
      - kind: other
        ref: "mutation 2 — the SSOAdminGroup != \"\" term removed: GREEN at first, investigated, and red once the guard's own contribution was assertable"
        status: pass
      - kind: other
        ref: "mutation 2b — this guard AND web.splitGroups' empty-element drop removed together: a header of \"|seite-a|\" makes an administrator, and so does provisioning. Red in five places. This is what proves the property is held at all."
        status: pass
    human_judgment: false
  - id: D4
    description: "An editor whose groups match NO configured website is REFUSED the sign-in rather than signed in with an empty assignment — D-01's inversion reached by subtraction instead of by creation"
    requirement: "SSO-06"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestLosingEveryWebsiteGroupDoesNotGrantEveryWebsite — asked of NewWebsiteAccessLookup, the constructor main.go hands auth.RequireWebsiteAccess, with the control step of the plan's step 4"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsRefusesAnEditorWithNoMatchingWebsiteGroup — 3 cases, each asserting that the existing user_websites rows survive the refusal"
        status: pass
      - kind: other
        ref: "mutation 3 — the refusal removed: 'the editor lost their last website group and gained access to \"Seite B\"'. Red in four places."
        status: pass
    human_judgment: false
  - id: D5
    description: "There is no third role: users.role carries a table-level CHECK (role IN ('admin','editor')) at 00001:7 and the sync writes one of exactly those two"
    requirement: "SSO-06"
    verification:
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/admin/forwardauth.go | grep -oE 'user\\.Role[A-Za-z]+' | sort -u == 'user.RoleAdmin user.RoleEditor'"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/admin/forwardauth.go | grep -cE '\"(admin|editor)\"' == 0 — a role is never a bare string, so a typo is a compile error"
        status: pass
      - kind: other
        ref: "ls internal/db/migrations/*.sql | wc -l unchanged at 51 — a third role would need a table rebuild and there is no third role"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every change of rights writes one activity row naming what changed from and to; an unchanged sign-in writes none"
    requirement: "SSO-06"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestUnchangedGroupsWriteNoActivityRow and #TestSyncRightsIsIdempotent"
        status: pass
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsIsIdempotentWhateverShapeTheHeaderArrivesIn — descending header order and two groups mapped to one website"
        status: pass
      - kind: other
        ref: "mutation 6 — SetRights and the row written on every sign-in: red in five places including the two tests that were passing trivially before"
        status: pass
      - kind: other
        ref: "grep -c 'Action[A-Za-z]* *= \"' internal/activity/entry.go unchanged at 19 — metadata added, no action name minted"
        status: pass
    human_judgment: false
  - id: D7
    description: "The publishing right is NOT overwritten: may_publish is read and carried through"
    requirement: "SSO-06"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsCarriesThePublishingRight — may_publish set to 0, then a sign-in that really does change the websites"
        status: pass
      - kind: other
        ref: "mutation 4 — MayPublish: true: 'may_publish went from false to true across a sign-in'. Red."
        status: pass
    human_judgment: false
  - id: D8
    description: "user.Store.Update refusing to demote the last administrator is handled rather than ignored: the sign-in continues and the refusal is logged loudly"
    requirement: "SSO-06"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsKeepsTheLastAdministrator — the role kept, the sign-in completed, the account named in the log, and no protocol row for a change that did not happen"
        status: pass
      - kind: other
        ref: "mutation 7 — the ErrLastAdmin case removed: the sign-in is refused and nothing is logged. Red."
        status: pass
    human_judgment: false
  - id: D9
    description: "An administrator's user_websites rows are left exactly as they are"
    requirement: "SSO-06"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsLeavesAnAdministratorsAssignmentAlone"
        status: pass
      - kind: other
        ref: "mutation 5 — the early return removed: an administrator whose only group is the administration group has no website group, so the sign-in is refused outright and a spurious protocol row appears. Red in two places."
        status: pass
    human_judgment: false
  - id: D10
    description: "With no group=website mapping configured the website half does not run, so SSO-06 does not lock every editor out of an installation that never opted into group-driven website access"
    verification:
      - kind: unit
        ref: "internal/admin/forwardauth_test.go#TestSyncRightsLeavesTheAssignmentAloneWithNoGroupMappingConfigured — the rows untouched, both lookups unchanged, and the role half still running"
        status: pass
      - kind: other
        ref: "mutation 9 — the branch removed: five of wave 3's and wave 4's own tests red, including the whole of TestForwardAuthProvisionsAnAccount. This is the deviation, stated as a mutation."
        status: pass
    human_judgment: false
  - id: D11
    description: "Whether an installation should refuse to start when single sign-on is on and HOLZCLOUD_SSO_WEBSITE_GROUPS is empty, and whether an operator understands that removing somebody's last website group turns their sign-in into the password form rather than into an error"
    verification: []
    human_judgment: true
    rationale: "Two operator-facing consequences no test can judge. First: with no group mapping configured, single sign-on grants websites to nobody and takes them from nobody — safe, and possibly not what an operator who set HOLZCLOUD_SSO_ADMIN_GROUP expected. Plan 10-01 already chose not to refuse to start on an empty administration group; the same argument applies here and the same DEPLOY.md paragraph (plan 10-08) is where it belongs. Second: a refused person sees the ordinary login form and no explanation, by design — the evidence is one auth.login_fail row and one 'no_website_group' line in the server log. Plan 10-10's browser pass is where a human watches that happen and says whether it reads as a dead end or as a bug."

duration: 40 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 05: The Rights, Re-applied Every Time Summary

**One function that re-derives role and website access from the identity provider's groups on every sign-in, and refuses the sign-in of an editor whose groups map to no configured website rather than writing the empty assignment `NewWebsiteAccessLookup` reads as access to every website — D-01's inversion reached by subtraction instead of by creation, proved by asking that same authorisation function and never by counting rows, with a control step that empties the rows by hand and watches the answer flip.**

## Performance

- **Duration:** ~40 min
- **Started:** 2026-09-08T04:47Z (approximate; the first commit landed at 04:54Z, after the reading)
- **Completed:** 2026-09-08T05:20Z
- **Tasks:** 2
- **Files:** 0 created, 2 modified

## The inversion, reached a third way

D-01 says a new account must not have zero rows in `user_websites`, because `NewWebsiteAccessLookup` reads zero as **every** website. Wave 4 closed that road by writing the assignment in the same function that creates the account. **The inversion has a second road and this plan closed it:** SSO-06 re-applies the rights on *every* sign-in, so an existing editor whose groups no longer map to any configured website would have their assignment reduced to nothing — the same state, arrived at by subtraction. An operator removing somebody's last website group, meaning to take their access away, would grant them everything. Neither `ROADMAP.md` nor `10-CONTEXT.md`'s original text named it; the planner found it and this plan refuses the sign-in instead of writing the empty list.

## Accomplishments

- **`syncRightsFromGroups` runs before `RenewToken` and before `completeLogin`,** so the role that reaches the session is the synchronised one. Mutation 8 shows what the stale one costs, and without silencing the compiler that mutation does not build at all.
- **The whole-element match is asked of `Identity.HasGroup` and of nothing else.** `not-holzcloud-admins`, `holzcloud-admins-x` and `xholzcloud-adminsx` are written out as literals in the table, guarded by one assertion that `fwdAdminGroup` still spells the name they were built on. Under mutation 1 exactly those three go red and the six matching cases stay green — which is the shape that proves the table is testing the trap and not the happy path.
- **The refusal is proved through `NewWebsiteAccessLookup`,** the constructor `main.go` hands `auth.RequireWebsiteAccess`, in both directions and with the control step. Wave 4's mutation 7 is the reason: it wrote one `user_websites` row naming the *wrong* website, so `SELECT COUNT(*)` returned 1 and a row-counting test would have been green while the account reached a site it must not have.
- **A refusal is not a way of emptying an assignment.** All four refusal cases assert that the existing rows survive, because a refusal that also cleared them would be this very bug wearing a different hat.
- **`may_publish` is read through `Rights` and carried into `SetRights` unchanged.**
- **The last administrator is not demoted,** the refusal is logged at warn level naming the account, and the sign-in continues. Locking an installation out because a group changed on somebody else's server is the worse outcome.
- **`internal/admin/handler.go` was not touched.** `git diff 13a0ee4..HEAD -- internal/admin/handler.go` is empty. `NewWebsiteAccessLookup` is cited by name throughout and never by line number.
- **Twelve mutations, twelve reds** — two of them only after the green ones had been investigated and the code or the tests changed. Every mutation was run against a **committed** file and restored with `git checkout --`.

## Task Commits

1. **Task 1 (RED): the four traps, before the function exists** — `be45d1b` (test)
2. **Task 1 (GREEN): `syncRightsFromGroups`, wired into step 6** — `aeabfe4` (feat)
3. **Task 1 (follow-up): the empty administration group asserted by this guard alone** — `f2cc46d` (test)
4. **Task 1 (follow-up): the sort and the deduplication made falsifiable** — `4b6ec5e` (test)
5. **Task 2: the trap headers written out as literals** — `09e670f` (test)
6. **Task 2: the tests named after the function they test** — `2cfca3c` (test)

Task 1 carried `tdd="true"`. The RED commit's tests compile against the pre-change tree — they are behavioural and name no new symbol — and 20 of 28 assertions failed, which is the intended failure. Seven subtests passed in the RED state, all of them negative assertions (*this group does not make an administrator*), and their value shows only under mutation 1; that is stated here rather than counted as coverage.

Commits 3 and 4 are follow-ups to Task 1 forced by mutation testing the committed file, the same shape wave 3's commits 4-5 and wave 4's commit 3 took. Commits 5 and 6 belong to Task 2, which the plan says produces no production code.

## Files Modified

- `internal/admin/forwardauth.go` (403 → 642 lines) — `errSSONoWebsiteGroup`, the reason codes `ssoRefuseNoWebsiteGroup` and `ssoRefuseSyncFailed`, step 6 of `ForwardAuthSignIn` and its renumbered step list, `syncRightsFromGroups` (163 lines including comments) and `sameWebsites`
- `internal/admin/forwardauth_test.go` (1167 → 1877 lines) — `newRightsSyncAdmin`, `fwdGroupRequest`, `assign`, `storedRole`, `storedMayPublish`, `sameIDList`, `countAction`, and 12 new test functions carrying 20 named subtests

## The two green mutations, and why they were different questions

The knowledge base says a green mutation means *something else is covering this input* — find out what, and whether it covers every input the guard was written for. It happened twice here and the answer was different both times.

### Mutation 2: the property was held, in another package

Removing the `h.cfg.SSOAdminGroup != ""` term left the whole suite green:

```
ok  	github.com/holzcloud/holzcloud-cms/internal/admin	12.851s
```

**What was covering it:** `web.splitGroups` drops empty elements, so `Identity.Groups` never carries `""`, so `HasGroup("")` is false for every input a real request can produce. **Does it cover every input?** For a request, yes. But `web.Identity` is an exported struct with an exported `Groups` field, `splitGroups` lives in another package and is not called from here, and the first thing that builds an `Identity` another way — a second reader, plan 10-07's admin surface, a fake in a test — arrives with that layer gone.

Removing **both** layers proves the property is real and falsifiable:

```
--- FAIL: TestSyncRightsMatchesAGroupAsAWholeElement
    --- FAIL: …/an_empty_configured_group_is_not_matched_by_an_empty_header_element
        forwardauth_test.go:1334: users.role = "admin"; want "editor" — the header was "|seite-a|" and the configured administration group was ""
--- FAIL: TestForwardAuthProvisionsAnAccount/the_account_is_an_editor_and_never_an_administrator
        forwardauth_test.go:738: role = "admin"; want "editor"
```

The fix, following wave 4's answer to the identical shape, was neither to delete the guard nor to wave it through: `TestSyncRightsRefusesAnEmptyAdministrationGroupOnItsOwn` calls `syncRightsFromGroups` directly with an `Identity` carrying `[]string{"", "seite-a", ""}` — the shape `splitGroups` never produces — and mutation 2 is now red on its own:

```
--- FAIL: TestSyncRightsRefusesAnEmptyAdministrationGroupOnItsOwn (0.54s)
    forwardauth_test.go:1813: role = "admin"; want "editor" — the configured administration group is the empty string, and an unset HOLZCLOUD_SSO_ADMIN_GROUP grants administration to nobody
```

### Mutation 10: nothing was covering it, and the mutation was right

Removing the `sort.Slice` also left everything green. That looked like the same finding and it was not. `SetRights` sorts again before inserting, so **no row in the database changes** — which is why it reads as redundant. What changes is the comparison against the stored assignment: `user.Store.Rights` returns its rows `ORDER BY website_id`, so an unsorted collected list never equals the stored one, `SetRights` runs on every sign-in, and **one `user.update` row is written per sign-in for a person whose rights never changed** — which is precisely the property `TestUnchangedGroupsWriteNoActivityRow` exists to hold, for the one input it did not use.

Nothing covered it. The mutation was right and the tests were too narrow: every existing idempotence test used a single group, where order is trivially stable. `TestSyncRightsIsIdempotentWhateverShapeTheHeaderArrivesIn` adds the two shapes that produce it — groups arriving in descending website order, and two groups the operator mapped to one website — and mutations 10 and 11 are now red:

```
--- FAIL: …/two_groups_in_descending_website_order
    forwardauth_test.go:1850: "user.update" rows went from 1 to 2 across a second identical sign-in with the header "seite-b|seite-a"
--- FAIL: …/two_groups_the_operator_mapped_to_one_website
    forwardauth_test.go:1871: "user.update" rows went from 1 to 2 across a second identical sign-in with the header "redaktion-a|leitung-a"
```

**The two findings look identical from outside — `ok`, no output — and only the investigation tells them apart.** Recording "mutation 10: green, the sort is redundant because SetRights sorts" would have been true, cheap, and would have left the sort free for the next person tidying to delete, and `/admin/protokoll` growing a row per sign-in for the first person whose groups arrive in the wrong order.

## Mutation Verification

Every guard was removed, inverted or redirected on a **committed** file, run against the plan's own test pattern, restored with `git checkout --`, and `git status --short -- internal/` confirmed empty afterwards. **Twelve mutations, twelve reds.** Verbatim output, abridged only where a failure repeats across subtests.

**1. The substring group match** (`ident.HasGroup(g)` → `strings.Contains(strings.Join(ident.Groups, "|"), g)`)
```
--- FAIL: TestSyncRightsMatchesAGroupAsAWholeElement (4.86s)
    --- FAIL: …/a_group_the_configured_name_is_a_suffix_of_is_not_the_group (0.57s)
        forwardauth_test.go:1334: users.role = "admin"; want "editor" — the header was "not-holzcloud-admins|seite-a" and the configured administration group was "holzcloud-admins". A whole element is compared, not a substring: strings.Contains over the raw header is how "not-holzcloud-admins|seite-a" becomes an administrator
    --- FAIL: …/a_group_the_configured_name_is_a_prefix_of_is_not_the_group (0.53s)
        forwardauth_test.go:1334: users.role = "admin"; want "editor" — the header was "holzcloud-admins-x|seite-a" …
    --- FAIL: …/a_group_carrying_the_configured_name_inside_it_is_not_the_group (0.53s)
        forwardauth_test.go:1334: users.role = "admin"; want "editor" — the header was "xholzcloud-adminsx|seite-a" …
```
```
and the plan's gate (want 0): 1
```
Exactly the three trap rows go red; the six matching rows stay green. A table that only used a matching group would have been green in full.

**2. The empty configured administration group — GREEN at first.** Investigated above; red after `TestSyncRightsRefusesAnEmptyAdministrationGroupOnItsOwn`, output quoted there.

**2b. This guard AND `web.splitGroups`' empty-element drop, together** — output quoted above. Red in five places, including `TestForwardAuthProvisionsAnAccount`: a provisioned account would be created as an administrator. This is what proves the property is held at all.

**3. The zero-website refusal removed — the mutation this whole plan exists to prevent**
```
--- FAIL: TestSyncRightsRefusesAnEditorWithNoMatchingWebsiteGroup (1.62s)
    --- FAIL: …/no_groups_at_all (0.53s)
        forwardauth_test.go:1602: an editor whose groups match no configured website signed in as 1
        forwardauth_test.go:1612: wrote 0 "auth.login_fail" rows; want exactly 1 naming the refusal
        forwardauth_test.go:1617: user_websites = [] after the refusal; want the untouched [2] — a refusal that empties the assignment produces exactly the state it refuses
--- FAIL: TestLosingEveryWebsiteGroupDoesNotGrantEveryWebsite (0.53s)
    forwardauth_test.go:1669: the editor lost every website group and was signed in anyway (as 1) — taking somebody's last website group away has to take their access away
    forwardauth_test.go:1673: wrote 0 "auth.login_fail" rows; want exactly 1 — an operator has no other way to learn that somebody was turned away
    forwardauth_test.go:1677: the editor lost their last website group and gained access to "Seite B" (id 3) — the sync wrote an empty assignment and NewWebsiteAccessLookup read it as "all"
```
All three subtests of the refusal test fail identically. **The third line of the second failure is the plan.**

**4. `may_publish` derived instead of carried** (`MayPublish: current.MayPublish` → `MayPublish: true`)
```
--- FAIL: TestSyncRightsCarriesThePublishingRight (0.56s)
    forwardauth_test.go:1508: may_publish went from false to true across a sign-in; no group grants the publishing right and nothing here decides it — an operator decided it about a person
```

**5. An administrator's assignment is written after all** (the `want == user.RoleAdmin` early return removed)
```
--- FAIL: TestSyncRightsMatchesAGroupAsAWholeElement/the_configured_group_alone_makes_an_administrator (0.54s)
    forwardauth_test.go:1340: the sign-in did not happen (session user_id = 0, account 1); the role above was not the one that reached the session
--- FAIL: TestSyncRightsKeepsTheLastAdministrator (0.53s)
    forwardauth_test.go:1573: wrote 1 "user.update" rows for a demotion that did not happen; want 0
```
**The first failure is the one worth reading.** An administrator whose only group is the administration group has no website group either, so with the early return gone they fall into the zero-website refusal and are **locked out**. The early return is not tidiness; it is what keeps administration reachable.

**6. `SetRights` and the protocol row on every sign-in** (`if !sameWebsites(…)` → `if true`)
```
--- FAIL: TestSyncRightsAppliesADemotionAtTheNextSignIn (0.54s)
    forwardauth_test.go:1372: wrote 2 "user.update" rows for the demotion; want exactly 1 naming what changed
--- FAIL: TestSyncRightsIsIdempotent (0.53s)
    forwardauth_test.go:1531: wrote 2 "user.update" rows across two identical sign-ins; want exactly 1 — the first one, from what changed then. SetRights is idempotent; logging every call is not
--- FAIL: TestUnchangedGroupsWriteNoActivityRow (0.54s)
    forwardauth_test.go:1716: "user.update" rows went from 1 to 2 across a sign-in that changed nothing; a protocol with one row per sign-in is a protocol nobody reads
--- FAIL: TestSyncRightsIsIdempotentWhateverShapeTheHeaderArrivesIn (1.07s)  [both subtests]
```
`TestUnchangedGroupsWriteNoActivityRow` passed trivially in the RED state (0 rows to 0 rows). This is where it earns its keep.

**7. The last-administrator refusal treated as a database failure** (`case errors.Is(err, user.ErrLastAdmin):` → `case false:`)
```
--- FAIL: TestSyncRightsKeepsTheLastAdministrator (0.53s)
    forwardauth_test.go:1561: the sign-in did not happen (session user_id = 0); a refused demotion is not a refused sign-in
    forwardauth_test.go:1565: session user_role = ""; want "admin" — the role that reaches the session is the one the account still has
    forwardauth_test.go:1569: the refused demotion was not logged with the account it concerns; log was:
```

**8. `completeLogin` handed the stale role.** Without `_ = role` it is a **compile error** — `declared and not used: role` — which is the strongest form this gate can take. With the compiler silenced:
```
--- FAIL: TestSyncRightsMatchesAGroupAsAWholeElement/the_configured_group_alone_makes_an_administrator
    forwardauth_test.go:1344: session user_role = "editor"; want "admin" — the synchronised role has to be the one handed to completeLogin, not the stale one read before it
--- FAIL: TestSyncRightsAppliesADemotionAtTheNextSignIn (0.53s)
    forwardauth_test.go:1369: session user_role = "admin"; want "editor"
```

**9. The unconfigured group mapping refuses everybody** (the `len(h.cfg.SSOWebsiteGroups) == 0` branch removed) — this is the plan's literal rule, run as a mutation:
```
--- FAIL: TestForwardAuthSignsInAnExistingAccount (0.56s)
    forwardauth_test.go:215: session user_id = 0; want 1
--- FAIL: TestForwardAuthUsesTheStoredSpellingOfTheAddress (0.53s)
--- FAIL: TestForwardAuthRotatesTheSessionToken (0.52s)
--- FAIL: TestForwardAuthWritesTheSameActivityRowAPasswordSignInWrites (0.53s)
    forwardauth_test.go:277: action = "auth.login_fail"; want "auth.login_success"
--- FAIL: TestForwardAuthProvisionsAnAccount (0.53s)   [3 subtests]
    forwardauth_test.go:763: session user_id = 0; want the account just created (1)
--- FAIL: TestSyncRightsLeavesTheAssignmentAloneWithNoGroupMappingConfigured (0.54s)
    forwardauth_test.go:1742: the editor was refused (session user_id = 0) although this installation configures no group-to-website mapping at all; every editor would be locked out of single sign-on, silently, because a refusal looks like the password form
```
Five of wave 3's and wave 4's tests fail, including the account provisioning had assigned one line earlier.

**10. The collected ids are not sorted — GREEN at first.** Investigated above; red after `TestSyncRightsIsIdempotentWhateverShapeTheHeaderArrivesIn`, output quoted there.

**11. The deduplication removed** — red on the second subtest of the same test, output quoted above.

After every mutation the files were restored with `git checkout --` and `git status --short -- internal/` confirmed empty.

## The counting gates, measured against the post-change tree

**Baseline measured at 2026-09-08T04:52Z, on this tree, immediately before the first commit of this plan — not taken from the plan.** The plan's note above its own table says these rows are deltas and that its own literals had already moved once; they had moved again.

| What | Command | Plan's number | Measured baseline | Adds | Measured after | Verdict |
|---|---|---|---|---|---|---|
| `SetRights` call sites | `grep -rn 'SetRights(' internal/ --include='*.go' \| grep -v _test \| grep -vc 'func (s \*Store) SetRights'` | 3 → 4 | **3** | 1 | **4** | exactly as predicted |
| `strings.Contains` in `forwardauth.go` | `grep -v '^[[:space:]]*//' … \| grep -c strings.Contains` | 0 → 0 | **0** | 0 | **0** | exactly as predicted |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | vorher (gate says 49) | **51** | 0 | **51** | delta 0; the literal 49 is stale three times over |
| activity action constants | `grep -c 'Action[A-Za-z]* *= "' internal/activity/entry.go` | 19 → 19 | **19** | 0 | **19** | exactly as predicted |
| files in `internal/admin` | `ls internal/admin/*.go \| grep -v _test \| wc -l` | vorher + 1 | **52** | 0 | **52** | delta 0 — this plan creates no file; the "adds 0"/"after +1" contradiction is wave 4's, unchanged |
| `adminProtectedMux.Handle` | `grep -c adminProtectedMux.Handle cmd/holzcloud/main.go` | vorher | **159** | 0 | **159** | delta 0 as required |
| admin templates | `ls cmd/holzcloud/templates/admin/*.html \| wc -l` | vorher | **68** | 0 | **68** | delta 0 as required |
| strings in source | `go run ./tools/i18n \| head -1` | vorher | **1319** | 0 | **1319** | delta 0 as required |
| `completeLogin` call sites | wave 3's standing invariant | 4 | **4** | 0 | **4** | delta 0; the AST test still green |
| `NewWebsiteAccessLookup` in `forwardauth_test.go` | `grep -c …` | ≥ 2 | **8** | 7 | **15** | gate satisfied; the number is mentions, not calls (see below) |

**`SetRights` at 4 is the row the plan asked to be stated,** and the four are now: the create form (`user.go:230`), the edit form (`user.go:321`), provisioning (`forwardauth.go`), and this sync. Wave 4 already corrected the plan's description of the first two — there is no bundle-import call site on this tree. **A fifth in this phase would mean the assignment is decided in a place nobody looking at these four would find.**

**The activity action constants are unchanged at 19,** measured on this tree rather than trusted from the plan. `entry.go`'s own comment says adding a name makes existing rows unfindable by a new filter and that metadata is free; this plan added metadata (`field`, `from`, `to`, `via`) and no names.

**The `NewWebsiteAccessLookup` count is mentions, not calls,** the same divergence wave 4 recorded. This plan makes **two** new calls — one in `TestLosingEveryWebsiteGroupDoesNotGrantEveryWebsite`, one in `TestSyncRightsLeavesTheAssignmentAloneWithNoGroupMappingConfigured`, plus one inside a subtest of `TestSyncRightsWritesExactlyTheMatchingWebsites` — and names the function four more times in comments, deliberately, because the phase cites it by name and never by line.

### `go run ./tools/i18n`, before and after

```
before:  1319 Zeichenketten im Quelltext
         en.json  1319 übersetzt, 0 offen, 0 verwaist   (es, fr, it identical)
after:   1319 Zeichenketten im Quelltext
         en.json  1319 übersetzt, 0 offen, 0 verwaist   (es, fr, it identical)
delta:   0 strings, 0 newly open, 0 orphaned
```

**The baseline is 1319 and `0 offen`, not the plan's 1311 and not wave 4's 41 open.** Plan 11-07 closed the forty-one and added one more string while this plan was reading. This plan minted no string and closed none: everything it does is invisible to a person. A promoted account sees the same dashboard; a refused person sees the same login form; `ssoRefuseNoWebsiteGroup` and `ssoRefuseSyncFailed` are `slog` field values and `activity_log` metadata, neither of which is rendered on a translated screen. The protocol screen shows the action name `user.update`, which already exists and is already translated.

## Decisions Made

- **The role is written before the zero-website refusal.** The plan's ordering, kept deliberately after weighing the alternative. Refusing first would be tidier — a refused sign-in would then write nothing at all — but an operator who removes somebody from every group, administration and websites alike, would leave a stale `admin` in `users.role` forever, reachable from the password form and from `auth.RequireAuth`, which re-reads the role on every request. The demotion is correct on its own evidence; the refusal is about the websites. The comment in the function says so.
- **The website half does not run where the operator configured no mapping.** See Deviations 1 — this is the plan's one substantive change and it has its own mutation.
- **Two refusal reason codes, not one.** `errSSONoWebsiteGroup` is recognised with `errors.Is` at the call site and becomes `no_website_group`; anything else becomes `rights_sync_failed`. From outside they are the same refusal — no session, one `auth.login_fail` row, the password form — and the server log is the only place the difference can live. An operator meets them on completely different days.
- **`errSSONoWebsiteGroup` is a sentinel** for the same reason `errSSOEmptyAddress` is: five refusals are observably identical, so without a name the caller could not pick the reason code and no test could say which branch ran.
- **`syncRightsFromGroups` takes both `ctx` and `*http.Request`,** as the plan's signature specifies. `provisionSSOUser` one screen below takes only the request and derives `ctx` from it. Two conventions in one file is not ideal; the plan wrote the signature explicitly and it is kept rather than quietly harmonised. Recorded as Deviation 4.
- **The tests were renamed to `TestSyncRights…`.** See Deviation 3.

## Deviations from Plan

### Auto-fixed

**1. [Rule 3 — Blocking] The plan's literal refusal rule locks every editor out of an installation with no group mapping**

- **Found during:** Task 1 GREEN — `go test ./internal/admin/` went from green to seven failing tests, all of them wave 3's and wave 4's.
- **Issue:** The plan says *if the resulting slice is empty, refuse*. With `HOLZCLOUD_SSO_WEBSITE_GROUPS` unset there is no mapping at all, so "your groups match no configured website" is true of everybody and says nothing about anybody. Every editor would be refused single sign-on — **including the account plan 10-04's provisioning had just created and correctly assigned one line earlier** — and silently, because a refusal here is the ordinary password form.
- **Fix:** `if len(h.cfg.SSOWebsiteGroups) == 0 { return want, nil }` before the ids are collected, with the longest comment in the function after the refusal's own. The invariant that matters is narrower than the plan's rule and still holds exactly: **this function never *writes* an empty assignment, and writing nothing cannot.** An account that already had no rows keeps none and keeps whatever it could reach before; no SSO path creates that state, because provisioning writes its one row in the same function that creates the account. Once the operator has configured even one `group=website` pair they have opted into group-driven website access, and from then on "no matching group" is an answer rather than the absence of a question.
- **Files modified:** `internal/admin/forwardauth.go`, `internal/admin/forwardauth_test.go`
- **Verification:** `TestSyncRightsLeavesTheAssignmentAloneWithNoGroupMappingConfigured`, and **mutation 9**, which removes the branch and reproduces the failure verbatim.
- **Committed in:** `aeabfe4`

**2. [Rule 2 — Missing critical] The sort and the deduplication had no test that could fail**

- **Found during:** Task 1 mutation testing (mutation 10, green), against the committed file.
- **Issue:** `SetRights` sorts and deduplicates again, so removing either changes no row. What it changes is the comparison against the stored assignment, and therefore whether a `user.update` row is written on **every** sign-in for somebody whose rights never changed. Every existing idempotence test used a single group, where order is trivially stable.
- **Fix:** `TestSyncRightsIsIdempotentWhateverShapeTheHeaderArrivesIn`, two subtests: groups in descending website order, and two groups mapped to one website.
- **Verification:** mutations 10 and 11, both red, output quoted above.
- **Committed in:** `4b6ec5e`

**3. [Rule 1 — Bug] The plan's own verification gate ran none of the tests the plan asked for**

- **Found during:** Task 2, running the plan's gate `go test ./internal/admin/ -run 'ForwardAuth|SyncRights|Groups' -v`.
- **Issue:** The twelve new tests were named `TestGroupSync…`. That matches neither `SyncRights` nor `Groups` (the name reads *Group* then *Sync*), so the gate ran 21 tests and not one of them was new. A gate that cannot see the work it verifies passes for the wrong reason forever.
- **Fix:** renamed `TestGroupSync…` → `TestSyncRights…` and `newGroupSyncAdmin` → `newRightsSyncAdmin`. The names now say which function is under test, which is the better name anyway. The gate runs all 24.
- **Committed in:** `2cfca3c`

### Divergences recorded rather than worked around

**4. `syncRightsFromGroups` takes both `ctx` and `r`,** as the plan specifies, where the adjacent `provisionSSOUser` takes only `r`. Wave 4 dropped the context parameter for exactly this reason and recorded it; this plan keeps it for exactly the opposite reason — the plan wrote the signature out. Two conventions now sit in one file. Neither is wrong; the drift is named here so the next plan can settle it deliberately.

**5. The plan's `HasGroup` gate counts its own explanation.** The gate is

```
sed -n '/func (h \*Handler) syncRightsFromGroups/,/^}/p' internal/admin/forwardauth.go | grep -c 'HasGroup'
```

and it is specified to fail unless it prints **1**. It prints **3**, because the comment naming trap one has to say `ident.HasGroup` to explain it. With `grep -v '^[[:space:]]*//'` added it prints exactly **1**. The plan applied that very exclusion to the `strings.Contains` gate one line above — *"a gate that its own explanation satisfies is not a gate"* — and did not apply it here. **Third instance in this phase of a gate measuring a proxy.** The property holds and the corrected gate is in the table below; the plan's literal gate is stale.

**6. The plan's `-run` pattern and the `migrations == 49` gate are stale.** Migrations are 51 on this tree (delta 0) — the same divergence waves 1, 2, 3 and 4 all recorded, now four phases old. Both are named rather than adjusted.

**7. Seven subtests of `TestSyncRightsMatchesAGroupAsAWholeElement` and `TestUnchangedGroupsWriteNoActivityRow` passed in the RED state.** They are negative assertions — *this does not make an administrator*, *this writes no row* — and a negative assertion is trivially true on a tree where nothing happens. Their value is entirely in mutations 1 and 6, and that is said here rather than counted as RED coverage.

**8. Task 2's first test and Task 1's refusal test overlap.** `TestLosingEveryWebsiteGroupDoesNotGrantEveryWebsite` (Task 2) and `TestSyncRightsRefusesAnEditorWithNoMatchingWebsiteGroup` (Task 1, eighth behaviour bullet) assert the same refusal from two sides — the lookup and the rows. Both were written in the RED commit, as wave 4's were and for the same reason: splitting one property across two commits to match a task boundary makes the RED commit an incomplete RED.

---

**Total deviations:** 3 auto-fixed (1 Rule 3, 1 Rule 2, 1 Rule 1) + 5 divergences recorded.
**Impact on plan:** No scope creep. Deviation 1 narrows the refusal to the case where the operator configured the mapping, and its own mutation shows what the unnarrowed version costs; it does not weaken the invariant the plan is about. Deviations 2 and 3 are tests and names. Nothing in `internal/admin/handler.go`, `internal/user/`, `internal/auth/`, `internal/web/` or `cmd/holzcloud/` was touched.

## Verification Results

**This run is from a tree whose `git status --short -- internal/ cmd/ tools/` was empty before and after and whose `HEAD` did not move during it** — `062c002` at 05:17:39Z and `062c002` at 05:19:56Z. A concurrent plan-11-07 agent committed `2bfbe23`, `a3a5fe8` and `062c002` into this tree during execution, and `.planning/STATE.md` and `.planning/WINDOWS.md` were dirty in its working tree at several points; no source file outside this plan's two was ever touched by either of us.

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | silent |
| `gofmt -l .` | silent |
| `go test ./...` | **44 packages ok, 0 failures** |
| `go test ./internal/admin/ -run 'ForwardAuth\|SyncRights\|Groups' -v` | 24 test functions, all PASS (11 forward-auth, 12 new, 1 shared), 38 named subtests |
| `go test ./internal/admin/ -run 'LosingEveryWebsiteGroup\|UnchangedGroupsWriteNoActivityRow' -v` | 2 tests, both PASS |
| `grep -c 'not-holzcloud-admins' internal/admin/forwardauth_test.go` | **3** (≠ 0) ✓ |
| `grep -v '//' internal/admin/forwardauth.go \| grep -c 'strings.Contains'` | **0** ✓ |
| `sed` the sync body `\| grep -c HasGroup` | **3** — the plan's gate wants 1 and counts its own comment; see deviation 5 |
| `sed` the sync body `\| grep -v '//' \| grep -c HasGroup` | **1** ✓ — the corrected gate |
| `sed` the sync body `\| grep -c 'MayPublish: current.MayPublish'` | **1** ✓ |
| `grep -v '//' … \| grep -oE 'user\.Role[A-Za-z]+' \| sort -u` | `user.RoleAdmin user.RoleEditor` ✓ — exactly two, no third role |
| `grep -v '//' … \| grep -cE '"(admin\|editor)"'` | **0** ✓ — a role is never a bare string |
| `grep -c NewWebsiteAccessLookup internal/admin/forwardauth_test.go` | **15** (≥ 2) ✓ |
| `grep -rn 'SetRights(' … \| grep -vc 'func (s \*Store) SetRights'` | **4** ✓ |
| `grep -c 'Action[A-Za-z]* *= "' internal/activity/entry.go` | **19** ✓ — unchanged |
| `git diff 13a0ee4..HEAD -- internal/admin/handler.go` | **empty** ✓ — `NewWebsiteAccessLookup` untouched |
| `ls internal/db/migrations/*.sql \| wc -l` | 51 — the plan's literal gate expects 49 and is stale; **delta 0** |
| `go run ./tools/i18n \| head -1` | `1319 Zeichenketten im Quelltext` before **and** after; **delta 0** |
| `git diff --diff-filter=D --name-only 2bfbe23..2cfca3c` | **empty** — no file was deleted by any commit of this plan |

## Issues Encountered

- **Two green mutations in one plan, with two different answers.** Reported in full above. The instructive part is that they are indistinguishable at the moment of observation, and that the second one — where nothing was covering the property — is the one that would most easily have been written off, because "SetRights sorts again anyway" is a true sentence that happens not to be the reason the sort is there.
- **The plan's own refusal rule was a lock-out.** It is not a planning error so much as a case the plan's own frame did not include: it was written about the state *after* an operator has configured a group mapping, and an installation before that point is a different state with the same predicate. Found in three seconds by running the existing suite, which is the argument for running it after every GREEN rather than at the end.
- **A gate whose `-run` pattern matches none of the plan's own tests** is a new shape for this phase's collection. The previous two were gates that read a proxy for the property; this one reads nothing at all and reports success. Renaming was cheaper than recording it.
- **A concurrent agent committed into this tree throughout.** Every commit here was staged file by file and verified with `git show --stat`; all six touch only this plan's two files.
- **`newTestAdmin`'s zero `Argon2Params`** bit again — `newRightsSyncAdmin` sets `cheapHashing`, as wave 4's note predicted it would need to.

## Threat Flags

None new. Every `mitigate` disposition in the plan's register has a passing test and a red mutation:

| Threat | Covered by |
|---|---|
| T-10-28 a substring match on the group header | mutation 1, exactly three subtests; the `strings.Contains` gate at 0 with comments excluded |
| T-10-29 an editor losing every website group and gaining all of them | mutation 3, asked of `NewWebsiteAccessLookup` in both directions; the control step; the surviving-rows assertion in all four refusal cases |
| T-10-30 an empty configured administration group matching an empty header element | mutations 2 and 2b — and the finding that only one of the three guards was assertable |
| T-10-31 a stale role surviving a demotion | mutation 8, which does not even compile without silencing; `TestSyncRightsAppliesADemotionAtTheNextSignIn` |
| T-10-32 a rights change with no record | `TestSyncRightsPromotionWritesOneProtocolRowNamingFromAndTo`; mutation 6 for the other direction |
| T-10-33 the last administrator demoted by a group change | mutation 7; `TestSyncRightsKeepsTheLastAdministrator` |
| T-10-34 `may_publish` reset on every sign-in | mutation 4; the grep gate at 1 |
| T-10-SC package-manager installs | no install task; every symbol is Go standard library or already in `go.mod` |

**One surface the register does not name, found by mutation 5 and worth carrying forward.** An administrator's rights come entirely from the role, so the website half of the sync is skipped for them — and it *has* to be, because an administrator typically carries no website group at all. That means **an account promoted to administrator by a group is authorised by exactly one string in `users.role`**, and `NewWebsiteAccessLookup` returns true two lines before it counts anything. The administration group name is therefore the whole of the trust boundary between the operator's directory and this installation, which is why plan 10-01 gave `HOLZCLOUD_SSO_ADMIN_GROUP` no default and why an empty one grants to nobody. It belongs in `DEPLOY.md` (plan 10-08) as a sentence about who may create groups in the operator's Authentik.

## Known Stubs

None. Everything this plan builds is reachable, wired and tested.

`ssoRefuseNoWebsiteGroup` and `ssoRefuseSyncFailed` are new values in an existing set of reason codes, both written on live paths — mutation 3's output shows the first one's `auth.login_fail` row being counted. Neither is a stub.

## User Setup Required

None from this plan. Three things it hands to plan 10-08 (`DEPLOY.md`), each of which an operator would otherwise meet as a surprise:

1. **With `HOLZCLOUD_SSO_WEBSITE_GROUPS` unset, single sign-on decides the role and nothing about websites.** Editors keep whatever assignment they have and nobody's access changes. This is safe and it is probably not what an operator who has just configured `HOLZCLOUD_SSO_ADMIN_GROUP` expects. The process does **not** refuse to start on it, for the same reason plan 10-01 chose not to refuse on an empty administration group.
2. **Removing somebody's last website group at the identity provider turns their sign-in into the password form, not into an error message.** That is the design — no refusal here is ever shown to anybody — and the operator's evidence is one `auth.login_fail` row per attempt in `/admin/protokoll` and a `reason=no_website_group` line in the server log. An operator who does not know this will hear "single sign-on is broken".
3. **The administration group is the whole trust boundary.** Anybody who can create a group with that name in the operator's Authentik can make themselves an administrator here at their next sign-in. The name has no default and must be one only the directory's own administrators can create.

## Next Phase Readiness

- **`SetRights` is now at exactly 4 call sites** — the create form, the edit form, provisioning and the sync. **A fifth for the rest of this phase means the assignment is being written somewhere a reader of those four would not find.** Wave 4's invariant, advanced by one.
- **`completeLogin` is still at 4** and wave 3's AST test still holds it there. `ForwardAuthSignIn` now has eight numbered steps rather than seven.
- **Plan 10-06 (second factor)** inserts itself after the sync and before `completeLogin`, or the other way round; note that the role handed to `completeLogin` is now a local `role` and not `u.Role`, and that `u.Role` is deliberately stale from that point on. Mutation 8 is the test that will catch a reordering that loses it.
- **Plan 10-07 (admin surface)** is the most likely place to build a `web.Identity` without `web.splitGroups` — a preview of what a group would grant, for instance. If it does, mutation 2's finding becomes live: `Identity.Groups` would carry an empty element and only `syncRightsFromGroups`' own `SSOAdminGroup != ""` term would stand between that and an administrator. `TestSyncRightsRefusesAnEmptyAdministrationGroupOnItsOwn` is the test that already asserts it.
- **Plan 10-08 (`DEPLOY.md`)** owes the three paragraphs under "User Setup Required" plus wave 4's three.
- **Carry these baselines forward, measured on this tree at 05:19Z:** migrations **51**, admin templates **68**, source strings **1319** with **0 offen**, `adminProtectedMux.Handle` **159**, `internal/admin` non-test files **52**, `completeLogin` call sites **4**, `SetRights` call sites **4**, activity action constants **19**.
- **For plan 10-10 (the browser pass):** sign in as an editor whose group maps to one website, then remove that group at the identity provider and sign in again. Every automated test asserts the refusal and the surviving rows; what no test can assert is whether the person in front of the password form has any idea why they are looking at it.
- **Do not cite `internal/admin/handler.go` by line number.** `NewWebsiteAccessLookup`'s final line has been `:173`, `:178` and `:183` in three documents of this phase, and it moved again while this one was being written.

## Self-Check: PASSED

- `internal/admin/forwardauth.go` — FOUND (642 lines), contains `syncRightsFromGroups`, `sameWebsites`, `errSSONoWebsiteGroup`, `ssoRefuseNoWebsiteGroup`, `HasGroup` (1× outside comments), `MayPublish: current.MayPublish` (1×), `strings.Contains` outside comments (0×), `user.Role*` set = {`RoleAdmin`, `RoleEditor`}
- `internal/admin/forwardauth_test.go` — FOUND (1877 lines), contains `not-holzcloud-admins` (3×), `NewWebsiteAccessLookup` (15×)
- `internal/admin/handler.go` — FOUND and **unchanged**: `git diff 13a0ee4..HEAD -- internal/admin/handler.go` empty
- Commit `be45d1b` — FOUND
- Commit `aeabfe4` — FOUND
- Commit `f2cc46d` — FOUND
- Commit `4b6ec5e` — FOUND
- Commit `09e670f` — FOUND
- Commit `2cfca3c` — FOUND
- `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` — all clean at 05:19:56Z on an unmoving `062c002`

---
*Phase: 10-authentik*
*Completed: 2026-09-08*
