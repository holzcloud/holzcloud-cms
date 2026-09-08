---
phase: 10-authentik
plan: 07
subsystem: auth
tags: [forward-auth, authentik, sso, sign-out, open-redirect, content-security-policy, form-action, htmx]

requires:
  - phase: 10-authentik
    provides: "plan 10-01's config.SSOSignOutPath, validated at load by isLocalPath as a path beginning with exactly one slash — and refusing the /\\ that a browser reads as protocol-relative just as it reads //. This plan reuses that validation and adds none of its own."
  - phase: 10-authentik
    provides: "plan 10-03's auth.SessionKeyViaSSO, written in exactly one place and now read in its second: the sign-out branch"
provides:
  - "admin.(*Handler).HandleLogout with the identity-provider branch: destroying the session here also sends the browser to the outpost, so authentik's own cookie goes too"
  - "The recorded non-decision: web.AdminCSP / web.AdminHeadersWith were NOT built, with the condition that would make them necessary and the two tests that fail first"
  - "TestLogoutTargetIsAlwaysARelativePath and TestLogoutRedirectIsPermittedByTheAdminFormActionPolicy: the two halves of the premise the absent CSP change rests on, asserted rather than argued"
  - "The first test coverage HandleLogout has ever had — it had none before this plan"
affects: [10-08-deploy-docs, 10-10-browser-pass, 12-translation-sweep]

actuals:
  # Scale note, because getting this wrong corrupts every later projection.
  # The template's rule is estimateTokens = chars/4 over the realized diff.
  # The realized diff (git diff c0d9a4b~1 -- login.go login_test.go) is
  # 21,866 characters, so chars/4 = 5,467. That is the number recorded here.
  #
  # It is NOT on the same scale wave 5 used: 10-05 recorded 34,000 against a
  # 35,457-character diff, i.e. approximately chars/1. Waves 3 and 4 look the
  # same way. So this phase's `actuals` are internally inconsistent, and
  # comparing this row against wave 5's measures the two measurement methods
  # rather than the two plans. Both numbers are given so a later calibration
  # can pick one and convert.
  tokens: 5467
  raw_diff_chars: 21866
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "A counting gate that does NOT strip comments is stricter than its name, and this time it caught the executor: a comment naming the forbidden token tripped it. The fix is to reword the comment, never to loosen the gate."
    - "A green mutation is a question — and the answer here was a third kind again: the guard's input is reachable (this package builds &Handler{} by literal) and nothing drove it, so the tests were too narrow"
    - "The premise an omission rests on is asserted on the very response that carries it: the Content-Security-Policy header and the Location header are read from the same recorder, so neither half can drift without the other noticing"
    - "A roadmap instruction is a hypothesis: this plan's mutation 5 IS the roadmap's own sentence, and it is red in five places"

key-files:
  created:
    - internal/admin/login_test.go
  modified:
    - internal/admin/login.go

key-decisions:
  - "web.AdminCSP / web.AdminHeadersWith were deliberately not built, and the reason is recorded below as a decision rather than left looking like a forgotten step of the build order."
  - "The redirect target is a relative path from the configuration and is never assembled from r.Host — which is the OPPOSITE of what ROADMAP.md line 481 instructs. r.Host is a header value, so 'build the target from the request's own host, never from a header' is self-contradictory, and following it is exactly mutation 5."
  - "auth.backTo / auth.SafeReturn are not reused, though the roadmap asks for it. Measured: SafeReturn(\"/outpost.goauthentik.io/sign_out\") returns \"/admin/\" — following that instruction would have made the sign-out button land the person back in the administration."
  - "The handler keeps h.cfg != nil, matching absoluteAdminURL's shape in the same package. Mutation 9 stayed green until a test drove a sign-out on a handler with no configuration; a Handler with a nil cfg is constructible in this package today."
  - "internal/admin/login_test.go was created, not extended. The plan says to add the tests 'beside the existing logout test' — there was no existing logout test and no login_test.go, and HandleLogout had no coverage at all."

patterns-established:
  - "A gate must measure what its name claims — fourth instance in this phase, and the first in the inverse direction: the gate was stricter than its wording and the executor's own comment was the thing it caught."
  - "Cite functions, never line numbers — carried from waves 1-5. The plan's own citation of the redirect helper at page.go:977 had already drifted to :1003 by the time this plan ran."
  - "Commit the task before mutating it — carried from wave 1, applied to all eleven mutations here."

requirements-completed: [SSO-08]

coverage:
  - id: D1
    description: "Signing out of a session the identity provider established sends the browser to the outpost's sign-out, so authentik's own cookie goes too and the next click does not silently sign the person back in"
    requirement: "SSO-08"
    verification:
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutOfAnSSOSessionSignsOutAtTheIdentityProvider — 303 and Location equal to cfg.SSOSignOutPath"
        status: pass
      - kind: other
        ref: "mutation 1 — the branch never taken: red in four tests"
        status: pass
      - kind: other
        ref: "mutation 11 — the helper answers 302: red, so the status is asserted and not incidental"
        status: pass
    human_judgment: false
  - id: D2
    description: "Signing out of a password session answers 303 to /admin/login exactly as it did before — same status, same target, byte for byte, with single sign-on switched ON in the handler so it is the session that decides and not the installation"
    requirement: "SSO-08"
    verification:
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutOfAPasswordSessionIsExactlyWhatItWas"
        status: pass
      - kind: other
        ref: "mutation 10 — the default target changed to /admin/: red in three tests, one of them this one"
        status: pass
      - kind: other
        ref: "mutation 11 — 302 instead of 303: red in this test too"
        status: pass
    human_judgment: false
  - id: D3
    description: "The Location value is always a path on this server: it is built from the configuration and never from r.Host, X-Forwarded-Host or anything else the request carries"
    requirement: "SSO-08"
    verification:
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutTargetIsAlwaysARelativePath — three values taken from config.Load and four config.Load refuses, with the four explicit conditions (leading /, not //, not /\\, no ://)"
        status: pass
      - kind: other
        ref: "sed the function body | grep -v '//' | grep -c 'r.Host|X-Forwarded|r.Header.Get' == 0"
        status: pass
      - kind: other
        ref: "mutation 5 — the target assembled as \"https://\" + r.Host + path, which is ROADMAP.md line 481's own instruction: red in five places, and the grep gate flips 0 to 1"
        status: pass
    human_judgment: false
  - id: D4
    description: "The premise the absent Content-Security-Policy change rests on, asserted on the very response that carries the redirect: the admin policy has form-action 'self' and the target is same-origin"
    requirement: "SSO-08"
    verification:
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutRedirectIsPermittedByTheAdminFormActionPolicy — web.AdminHeaders wrapped round the real handler, both branches"
        status: pass
      - kind: other
        ref: "mutation 8 — form-action removed from adminCSP: red in both subtests, naming the whole policy it read"
        status: pass
      - kind: other
        ref: "mutation 5 — the target made cross-origin: red with 'form-action 'self' will block it silently in Safari'"
        status: pass
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestAdminResponsesCarrySecurityHeaders — unchanged and green; frame-ancestors 'none', X-Frame-Options: DENY and Cache-Control: no-store are what the roadmap warned would be at risk if the policy were rebuilt"
        status: pass
    human_judgment: false
  - id: D5
    description: "The via_sso flag is read BEFORE sm.Destroy, because afterwards the session no longer knows how it was established"
    requirement: "SSO-08"
    verification:
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutReadsTheViaSSOFlagBeforeTheSessionIsDestroyed — part one asserts the branch, part two performs the same read after Destroy and requires it to answer false"
        status: pass
      - kind: other
        ref: "sed the function body | grep -n 'SessionKeyViaSSO|sm.Destroy' — line 18 before line 34"
        status: pass
      - kind: other
        ref: "mutation 2 — the whole branch physically moved below sm.Destroy: red in four tests, and the source-order gate flips (the read at 22, Destroy at 17)"
        status: pass
    human_judgment: false
  - id: D6
    description: "The activity row is still written before the session is destroyed and still carries the actor, and the session really is destroyed on both paths"
    requirement: "SSO-08"
    verification:
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutWritesTheProtocolRowOnBothPaths — exactly one auth.logout row, with actor_email and user_id"
        status: pass
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutDestroysTheSessionOnBothPaths — the same cookie is anonymous on a later request"
        status: pass
      - kind: other
        ref: "mutation 6 — LogActivity moved below Destroy: actor_email = \"\" and user_id = 0 on both paths. Red."
        status: pass
      - kind: other
        ref: "mutation 7 — sm.Destroy removed: the cookie still belongs to user 1 afterwards. Red."
        status: pass
    human_judgment: false
  - id: D7
    description: "Two shapes that must not reach the outpost: single sign-on switched off with a stale via_sso session, and a handler with no configuration at all"
    verification:
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutWithSSOSwitchedOffIgnoresAStaleViaSSOFlag"
        status: pass
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutSurvivesAHandlerWithNoConfiguration"
        status: pass
      - kind: other
        ref: "mutation 3 — the SSOEnabled term removed: red. mutation 9 — the h.cfg != nil term removed: GREEN at first, investigated, and now a nil dereference at the assertion."
        status: pass
    human_judgment: false
  - id: D8
    description: "An htmx sign-out would get HX-Redirect and no body, because the answer goes through the package's own redirect helper rather than a hand-written 303"
    verification:
      - kind: unit
        ref: "internal/admin/login_test.go#TestLogoutOfAnHtmxRequestAnswersWithHXRedirect"
        status: pass
      - kind: other
        ref: "sed the function body | grep -c 'http.Redirect' == 0; grep -c in login.go 6 -> 5"
        status: pass
      - kind: other
        ref: "mutation 4 — the helper replaced by a hand-written 303: red, and both counting gates flip (0 to 1 inside the function, 5 to 6 in the file)"
        status: pass
    human_judgment: false
  - id: D9
    description: "Nothing this plan does is read by a person on a screen: no route, no template, no translatable string, and internal/web/headers.go is untouched"
    verification:
      - kind: other
        ref: "adminProtectedMux.Handle 159 -> 159; adminOnly rows 19 -> 19; admin templates 68 -> 68; layoutPageNames 50 -> 50; migrations 51 -> 51; non-test files in internal/admin 52 -> 52"
        status: pass
      - kind: other
        ref: "git diff -- internal/web/headers.go | wc -l == 0, and git diff 1fff843 -- internal/web/headers.go == 0"
        status: pass
      - kind: other
        ref: "grep -c '{{t |i18n.|web.T(' on both files == 0; the i18n count moved 1319 -> 1321 and both strings are plan 10-06's, quoted verbatim below"
        status: pass
    human_judgment: false
  - id: D10
    description: "That a real authentik outpost, on the operator's own domain, actually answers /outpost.goauthentik.io/sign_out by clearing its cookie and sending the person somewhere sensible"
    verification: []
    human_judgment: true
    rationale: "This repository can assert that the browser is sent to the configured path and that the path is on this server. What happens at the other end belongs to the operator's Authentik and their reverse proxy, and no test here can observe it. Plan 10-08 owns writing the expectation into DEPLOY.md; plan 10-10's browser pass — run twice, once with forward auth on and once off — is where a human watches a sign-out actually end the session at both ends."

duration: 28 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 07: The Sign-Out That Signs Somebody Out Summary

**`HandleLogout` learns where the person came in and tells the identity provider they are leaving — four lines of branch, a relative target that no header can influence, and the Content-Security-Policy work the build order asked for recorded as unnecessary with the two tests that will say so first.**

## Performance

- **Duration:** 28 min
- **Started:** 2026-09-08T05:27Z
- **Completed:** 2026-09-08T05:55Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- A sign-out from a session the identity provider established now answers 303 to `cfg.SSOSignOutPath`, so authentik's cookie goes with the session and the next click under `/admin/` does not silently sign the person back in (SSO-08).
- The password path's sign-out is asserted byte for byte — same status, same target — with single sign-on switched **on** in the handler under test, so what the test proves is that the *session* decides and not the installation.
- The premise that made the roadmap's step ⑦ unnecessary is now two tests rather than a paragraph, and both go red under the mutations that would invalidate it.
- **`HandleLogout` had no test coverage at all before this plan.** `grep -rln 'HandleLogout'` returned three non-test files and nothing else.

## Task Commits

1. **Task 1 (RED)** — `c0d9a4b` `test(10-07): the sign-out an identity provider has to be told about`
2. **Task 1 (GREEN)** — `824592c` `feat(10-07): signing out signs the person out at the identity provider too`
3. **Task 1 (widening, after mutation 9)** — `bd0e65e` `test(10-07): the guard a green mutation asked about`

Task 2 produced no code by design; its output is the two sections below and the counting table.

## Files Created/Modified

- `internal/admin/login_test.go` — **created.** Nine tests over `HandleLogout`: the SSO branch, the untouched password path, the stale `via_sso` flag with the outpost switched off, the relative target in both directions, the `form-action` premise, the protocol row, the destroyed session, the htmx answer, the read-before-`Destroy` ordering, and a handler with no configuration.
- `internal/admin/login.go` — `HandleLogout` gains the branch, and three comments longer than the change: why signing out here is not enough, why the target is relative, and what was deliberately not built.

---

## Not built: `web.AdminCSP` / `web.AdminHeadersWith`

`ROADMAP.md`'s step ⑦ names them. **They are not needed in this shape, and building them would be a mechanism with no caller.** `adminCSP` (`internal/web/headers.go`) carries `form-action 'self'`; the sign-out target is `/outpost.goauthentik.io/sign_out`, a **relative path on this server**, and a same-origin target satisfies `'self'`.

The Safari behaviour the roadmap warns about — a redirect answering a form POST checked against `form-action` — is real, and is written up beside `PaymentFormAction` in the same file: *"the TWINT payment on an iPhone dies silently at the moment of the handover, and nothing in the server log says why."* It fires only for a **cross-origin** target.

**What holds that premise is two tests, not a convention.**

- `TestLogoutTargetIsAlwaysARelativePath` asserts the `Location` begins with one slash, is not `//`, is not `/\`, and carries no scheme — for three values `config.Load` accepts, and it asserts in the other direction that `config.Load` refuses four that would leave this origin.
- `TestLogoutRedirectIsPermittedByTheAdminFormActionPolicy` reads the `Content-Security-Policy` and the `Location` **off the same recorder**, so the policy and the target it permits cannot drift apart without one of them failing.

The day a **separate outpost host** is supported, both fail first and name what has to change: `PublicCSP`'s shape (`PublicCSP(extraFormAction ...string)` + `SecureHeadersWith(csp)`) mirrored for the admin policy and wired where `web.AdminHeaders` is wired today, keeping `frame-ancestors 'none'`, `X-Frame-Options: DENY` and `Cache-Control: no-store`, which `cmd/holzcloud/main_test.go#TestAdminResponsesCarrySecurityHeaders` asserts and which still passes unchanged.

Both directions were watched go red. Mutation 8 removed `form-action 'self'` from `adminCSP`; mutation 5 made the target cross-origin. `internal/web/headers.go` is byte-identical to its state at the wave-5 close (`git diff 1fff843 -- internal/web/headers.go` is empty).

## Three roadmap notes that do not describe this tree

**1. "logout is an htmx POST, so use `HX-Redirect` + `Vary: HX-Request`."** It is not. `cmd/holzcloud/templates/admin/base.html:69-72` is a plain `<form method="POST" action="/admin/logout">` with a CSRF hidden field and no `hx-` attribute, and `grep -rn hx-boost cmd/holzcloud/templates/` returns nothing. The treatment is applied anyway, because `(*Handler).redirect` already does both and using it costs one word — but a later reader should not conclude from the code that the note was ignored. `Vary: HX-Request` is set by the render layer and is not set a second time here.

**2. "build the target from the request's own host, never from a header."** (`ROADMAP.md` line 481.) This is self-contradictory, and this plan does the opposite of its first half deliberately. `r.Host` *is* a header value — Go fills it from the request line or the `Host:` header — so "from the request's own host" and "never from a header" cannot both be satisfied. A host taken from a request and put into a redirect is how an open redirect is built, which `auth.SafeReturn` already says in this codebase's own words. **Mutation 5 is the roadmap's sentence implemented literally, and it is red in five places**, including with the message `form-action 'self' will block it silently in Safari`. The target is built from `cfg.SSOSignOutPath`, which `config.Load` already refused unless it is a path beginning with exactly one slash.

**3. "reusing `auth.backTo` / `auth.SafeReturn`."** Measured, not reasoned:

```
SafeReturn("/outpost.goauthentik.io/sign_out") = "/admin/"
```

`SafeReturn` narrows anything outside `/admin/` back to the dashboard, by design and correctly for the confirmation screen it was written for. Following the roadmap here would have produced a sign-out button that lands the person back inside the administration — a silent failure of SSO-08 with a green build. Neither helper is used; the sign-out target is a fixed configured path and not a return address, so there is no untrusted string to narrow.

## The green mutation, and what it turned out to be

Mutation 9 removed `h.cfg != nil` from the branch condition. The whole `-run 'Logout|SignOut'` set stayed green:

```
ok  	github.com/holzcloud/holzcloud-cms/internal/admin	9.426s
```

This phase has now seen three different answers to that shape (wave 5's mutation 2: the property was held in another package; wave 5's mutation 10: nothing held it and the mutation was right). **This is a fourth: the guard's input is reachable, and nothing drove it.**

`admin.Handler` is constructed with a non-nil `cfg` at both of its two `NewHandler` call sites (`cmd/holzcloud/main.go:273` and `cmd/holzcloud/main_test.go:102`, plus `page_handler_test.go:63`) — which is what makes the guard look like dead defensiveness. But the package also builds the struct **by literal**: `internal/admin/page_fields_kinds_test.go:244` and `:257` construct `&Handler{}` and `&Handler{terms: …}`. A handler with a session manager and no configuration is therefore constructible in this package today, and no test drove a sign-out on one.

The fix follows waves 4 and 5: not deleting the guard, and not waving it through, but making its own contribution assertable. `TestLogoutSurvivesAHandlerWithNoConfiguration` drives the real handler with `h.cfg = nil`, and mutation 9 is now red:

```
--- FAIL: TestLogoutSurvivesAHandlerWithNoConfiguration (0.60s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
[signal SIGSEGV: segmentation violation code=0x2 addr=0x138 pc=0x10298034c]
	/Users/holz/Projects/holzcloud-cms/internal/admin/login_test.go:413 +0xf8
```

The guard also matches the package's existing shape — `absoluteAdminURL` reads `if h.cfg != nil && h.cfg.Secure` — so the two are now consistent *and* one of them is tested.

## Mutation Verification

Every mutation was applied to a **committed** file, run against the plan's own `-run 'Logout|SignOut'` pattern (which matches all nine tests — checked, after wave 5 lost a whole gate to a pattern that matched none), restored with `git checkout --`, and `git status --short -- internal/admin/login.go` confirmed empty afterwards. **Eleven mutations. Ten red on the first run; the eleventh green, investigated, and red after the tests were widened.** Verbatim, abridged where a failure repeats across subtests.

**1. The SSO branch never taken** (`if false && h.cfg != nil && …`)
```
--- FAIL: TestLogoutOfAnSSOSessionSignsOutAtTheIdentityProvider (0.58s)
    login_test.go:123: Location = "/admin/login"; want "/outpost.goauthentik.io/sign_out" — destroying the session here leaves authentik's own cookie in the browser, and the next click signs the person straight back in
--- FAIL: TestLogoutTargetIsAlwaysARelativePath/config.Load_accepts_/sign_out (0.55s)
        login_test.go:188: Location = "/admin/login"; want "/sign_out"
--- FAIL: TestLogoutOfAnHtmxRequestAnswersWithHXRedirect (0.57s)
--- FAIL: TestLogoutReadsTheViaSSOFlagBeforeTheSessionIsDestroyed (0.57s)
```

**2. The whole branch moved below `sm.Destroy`** — the ordering mutation, and the reason the plan asks for a test rather than only a source-order gate.
```
--- FAIL: TestLogoutOfAnSSOSessionSignsOutAtTheIdentityProvider (0.58s)
    login_test.go:123: Location = "/admin/login"; want "/outpost.goauthentik.io/sign_out" …
--- FAIL: TestLogoutTargetIsAlwaysARelativePath (2.25s)   [all three configured values]
--- FAIL: TestLogoutOfAnHtmxRequestAnswersWithHXRedirect (0.56s)
--- FAIL: TestLogoutReadsTheViaSSOFlagBeforeTheSessionIsDestroyed (0.59s)
```
The source-order gate flips with it: the read reports at line 22 and `sm.Destroy` at line 17.

**3. The `h.cfg.SSOEnabled` term removed**
```
--- FAIL: TestLogoutWithSSOSwitchedOffIgnoresAStaleViaSSOFlag (0.56s)
    login_test.go:159: Location = "/outpost.goauthentik.io/sign_out"; want "/admin/login" — with HOLZCLOUD_SSO_ENABLED off there is no outpost to reach
```

**4. `h.redirect` replaced by a hand-written 303**
```
--- FAIL: TestLogoutOfAnHtmxRequestAnswersWithHXRedirect (0.56s)
    login_test.go:344: HX-Redirect = ""; want "/outpost.goauthentik.io/sign_out" — a 303 answering an htmx POST is swapped into the page instead of followed
    login_test.go:347: Location = "/outpost.goauthentik.io/sign_out"; an htmx answer carries HX-Redirect and no Location
```
Both counting gates flip: the token inside the function 0 → 1, and the file 5 → 6.

**5. The target assembled from the request** (`"https://" + r.Host + h.cfg.SSOSignOutPath`) — this is `ROADMAP.md` line 481's instruction implemented literally.
```
--- FAIL: TestLogoutOfAnSSOSessionSignsOutAtTheIdentityProvider (0.58s)
    login_test.go:123: Location = "https://admin.test/outpost.goauthentik.io/sign_out"; want "/outpost.goauthentik.io/sign_out" …
--- FAIL: TestLogoutTargetIsAlwaysARelativePath/config.Load_accepts_/sign_out (0.54s)
        login_test.go:190: Location = "https://admin.test/sign_out"; want a path on this server, beginning with a slash
        login_test.go:190: Location = "https://admin.test/sign_out"; a sign-out target carrying a scheme is an open redirect
--- FAIL: TestLogoutRedirectIsPermittedByTheAdminFormActionPolicy/the_identity_provider's_sign-out (0.54s)
        login_test.go:266: Location = "https://admin.test/outpost.goauthentik.io/sign_out" is cross-origin; form-action 'self' will block it silently in Safari
--- FAIL: TestLogoutOfAnHtmxRequestAnswersWithHXRedirect (0.57s)
--- FAIL: TestLogoutReadsTheViaSSOFlagBeforeTheSessionIsDestroyed (0.56s)
```
Red in five places, and the `r.Host` grep gate flips 0 → 1.

**6. `LogActivity` moved below `sm.Destroy`** — the ordering the existing German comment already documents.
```
--- FAIL: TestLogoutWritesTheProtocolRowOnBothPaths/through_the_identity_provider (0.62s)
        login_test.go:299: actor_email = ""; want "sso@test"
        login_test.go:309: user_id = 0; want 1
--- FAIL: TestLogoutWritesTheProtocolRowOnBothPaths/through_the_password_form (0.59s)
        login_test.go:299: actor_email = ""; want "password@test"
```

**7. `sm.Destroy` removed**
```
--- FAIL: TestLogoutDestroysTheSessionOnBothPaths/through_the_identity_provider (0.56s)
        login_test.go:326: after the sign-out the same cookie still belongs to 1; want an anonymous session
--- FAIL: TestLogoutDestroysTheSessionOnBothPaths/through_the_password_form (0.54s)
```

**8. `form-action 'self'` removed from `adminCSP`** — the other half of the CSP premise.
```
--- FAIL: TestLogoutRedirectIsPermittedByTheAdminFormActionPolicy/the_identity_provider's_sign-out (0.59s)
        login_test.go:252: the admin policy no longer carries form-action 'self': "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'" …
--- FAIL: TestLogoutRedirectIsPermittedByTheAdminFormActionPolicy/the_login_form (0.55s)
```
This one touched a **shared** file. It was applied and restored inside a single command with one narrowly-scoped test run between: applied 05:37:19Z, restored 05:37:22Z, a three-second window, and `git diff -- internal/web/headers.go | wc -l` back to 0 immediately. Recorded rather than glossed, because two other agents were reading this tree.

**9. `h.cfg != nil` removed** — GREEN at first. See the section above.

**10. The password path's target changed** to `/admin/`
```
--- FAIL: TestLogoutOfAPasswordSessionIsExactlyWhatItWas (0.57s)
    login_test.go:143: Location = "/admin/"; want "/admin/login"
--- FAIL: TestLogoutWithSSOSwitchedOffIgnoresAStaleViaSSOFlag (0.56s)
--- FAIL: TestLogoutSurvivesAHandlerWithNoConfiguration (0.60s)
```

**11. The redirect helper answers 302 instead of 303** (`internal/admin/page.go`, restored immediately)
```
--- FAIL: TestLogoutOfAnSSOSessionSignsOutAtTheIdentityProvider (0.59s)
    login_test.go:120: status = 302; want 303
--- FAIL: TestLogoutOfAPasswordSessionIsExactlyWhatItWas (0.56s)
    login_test.go:140: status = 302; want 303
```
The "byte for byte" claim about the password path covers the status as well as the target, and this is what says so.

## The counting gates, measured against the post-change tree

**Baseline measured at 2026-09-08T05:28Z on this tree, immediately before the first commit of this plan — not taken from the plan.** The plan's own note above its table says these rows are deltas and that its literals had already moved; two of them had moved again.

| What | Command | Plan's number | Measured baseline | Adds | Measured after | Verdict |
|---|---|---|---|---|---|---|
| `internal/web/headers.go` diff | `git diff -- internal/web/headers.go \| wc -l` | 0 → 0 | **0** | 0 | **0** | exactly as predicted |
| `adminProtectedMux.Handle` | `grep -c adminProtectedMux.Handle cmd/holzcloud/main.go` | gate says 150 | **159** | 0 | **159** | delta 0 as required; the literal 150 is stale, and 159 is what wave 5 measured too |
| `adminOnly` rows | the table at `main_test.go:195` | 19 | **19** | 0 | **19** | exactly as predicted |
| admin templates | `ls cmd/holzcloud/templates/admin/*.html \| wc -l` | vorher | **68** | 0 | **68** | delta 0 — no `/admin/abgemeldet` page was built |
| `layoutPageNames` entries | the slice in `internal/web/render.go` | 50 | **50** | 0 | **50** | exactly as predicted |
| `http.Redirect` in `login.go` | `grep -c 'http.Redirect' internal/admin/login.go` | 6 → 5 | **6** | −1 | **5** | exactly as predicted |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | vorher | **51** | 0 | **51** | delta 0 as required |
| strings in source | `go run ./tools/i18n \| head -1` | vorher, +2 from 10-06 | **1319** | 0 | **1321** | +2, both plan 10-06's; this plan's own delta is 0 |
| non-test files in `internal/admin` | `ls internal/admin/*.go \| grep -v _test \| wc -l` | — | **52** | 0 | **52** | delta 0 — this plan creates no non-test file |

**No new route and no new template.** The earlier drafting of this phase considered answering the logout POST with a local `/admin/abgemeldet` page carrying a plain `<a>` — the roadmap offers it as the cheaper alternative to the CSP change — and it was rejected because it is a click between a person and being signed out, on the one screen where hesitating means staying signed in. Both rows are unchanged, so it was not built.

**`http.Redirect` in `login.go` fell by exactly one.** The five that remain are the login form's own paths — the rate-limit refusal, the two failure paths, the second-factor hand-off and the success — and they are untouched.

### `go run ./tools/i18n`, before and after

```
before (05:28Z):  1319 Zeichenketten im Quelltext
                  en.json  1319 übersetzt, 0 offen, 0 verwaist   (es, fr, it identical)
after  (05:42Z):  1321 Zeichenketten im Quelltext
                  en.json  1319 übersetzt, 2 offen, 0 verwaist   (es, fr, it identical)
delta:            +2 strings, +2 open, 0 orphaned
```

**The delta attributable to this plan is 0, and the two are plan 10-06's**, from commit `8d6df09 feat(10-06): the dependency on the identity provider, shown on two screens`:

```
+ {{t "Du bist über die Anmeldung deiner Organisation hier hereingekommen. Die Bestätigung in zwei Schritten verlangt dort, wer dich angemeldet hat – diese Anlage verlangt sie dann nicht noch einmal."}}
+ {{t "Wer sich über die Anmeldung der Organisation anmeldet, bringt seine Bestätigung in zwei Schritten von dort mit. Ob sie verlangt wird, entscheidet die Anmeldung der Organisation und nicht diese Anlage."}}
```

`grep -c '{{t \|i18n\.\|web\.T(' internal/admin/login.go internal/admin/login_test.go` is **0** for both files. Everything this plan does is invisible to a person: a sign-out that lands on the identity provider's own page needs no wording from here.

**The `2 offen` is plan 10-06's catalogue write, not a regression of this one.** Their strings are in the templates and not yet in `en/es/fr/it.json`; `go run ./tools/i18n -write` is theirs to run at their close-out. Flagging it here so it is not read as unattended drift, and so that whichever plan closes it does not have to re-derive where it came from.

## Decisions Made

1. **`web.AdminCSP` / `web.AdminHeadersWith` were not built.** Recorded above as a decision with its revisiting condition and the two tests that fail first, so it does not look like a step of the build order that somebody forgot.
2. **The target is never assembled from the request**, against `ROADMAP.md` line 481's explicit instruction. The instruction is self-contradictory (`r.Host` is a header) and implementing it is mutation 5.
3. **`auth.backTo` / `auth.SafeReturn` are not reused**, also against the roadmap. Measured: `SafeReturn` would have rewritten the outpost path to `/admin/`.
4. **The answer goes through `(*Handler).redirect`**, so the plan needs no opinion on whether the logout form will ever become htmx.
5. **`h.cfg != nil` stays**, and now has a test of its own.
6. **`internal/admin/login_test.go` was created rather than extended**, because there was no logout test to sit beside.
7. **The existing German comment at the top of `HandleLogout` was left exactly as it is.** It is correct, and translating it is Phase 12's sweep. Every comment this plan added is English.

## Deviations from Plan

### 1. [Rule 3 — Blocking] The plan's "existing logout test" does not exist

- **Found during:** Task 1, writing the tests.
- **Issue:** The plan says to add the tests "beside the existing logout test" in `internal/admin/login_test.go`, and to assert the password path "against the same expectation the existing test uses". Neither exists. `ls internal/admin/*_test.go` has 24 files and no `login_test.go`, and `grep -rln 'HandleLogout\|ActionAuthLogout'` returns only `cmd/holzcloud/main.go`, `internal/activity/entry.go` and `internal/admin/login.go`. **`HandleLogout` had no test coverage of any kind before this plan.**
- **Fix:** The file was created, and the password-path expectation (`303` to `/admin/login`) was read from the handler's own pre-change source rather than from a test. Mutations 10 and 11 are what make that expectation load-bearing rather than transcribed.
- **Files:** `internal/admin/login_test.go`
- **Committed in:** `c0d9a4b`

### 2. [Rule 1 — Bug in this plan's own work] A comment satisfied the gate's own prohibition

- **Found during:** Task 1, running the gates before the GREEN commit.
- **Issue:** The plan's third gate counts `http.Redirect` inside `HandleLogout` and must read 0. Unlike the `r.Host` gate beside it, **it does not strip comments** — deliberately, since it is a stricter reading. The comment explaining why the package helper was used named the standard-library function, so the gate read **1** and the file-level gate read **6** instead of 5. Both were failing at the moment the implementation was otherwise complete.
- **Fix:** The comment was reworded to describe the alternative without naming it, and now says so explicitly, so the next person does not reintroduce the token while tidying. **The gate was not loosened.** This is the phase's fourth instance of "a gate must measure what its name claims" — and the first in the inverse direction, where the gate was stricter than its wording and caught the executor.
- **Verification:** function-level count 1 → 0; file-level count 6 → 5.
- **Committed in:** `824592c`

### 3. [Recorded, not fixed] The plan's citation of the redirect helper had drifted

- **Issue:** The plan's `<read_first>` names `internal/admin/page.go:975-984` for `(*Handler).redirect`. The function is at **`page.go:1003-1010`**; lines 975-984 are inside `renderPageRow`. The plan's action text cites `page.go:977`.
- **Action:** Nothing to fix — the function was found by name. Recorded because it is the phase's standing observation, now in its fifth wave: **cite functions, never line numbers.** The plan's own `<flagged_assumptions>` block corrects `main_test.go:220-234` to `:223-238` for the same reason.

### 4. [Recorded, not fixed] A gate's literal is stale

- **Issue:** Task 2's gate `grep -c 'adminProtectedMux.Handle' cmd/holzcloud/main.go` fails when the count "is not 150". It is **159**, before and after, and wave 5 measured 159 as well.
- **Action:** The gate's *meaning* — no route was registered, so `TestRouteAuthorization` needs no new row — holds: the delta is 0. The literal was not edited into the plan; it is recorded here, which is what the plan's own note above its table asks for. A gate that fails on an empty commit gets skimmed the next time rather than read.

---

**Total deviations:** 2 acted on (1 blocking, 1 bug in this plan's own work), 2 recorded without action.
**Impact on plan:** No scope change. Deviation 1 made the plan's work larger than described — the plan expected to extend a test file and it had to establish the handler's first coverage — and deviation 2 was caught by the plan's own gate, which is the gate working.

## Verification Results

Run at 05:48-05:50Z, `HEAD` unchanged across the run (`f333a7b` at both ends), working tree carrying only another agent's `.planning/WINDOWS.md`:

```
go build ./...            clean
go vet ./...              silent
gofmt -l .                silent
go test -count=1 ./...    exit 0 — 44 packages ok, 0 FAIL lines
```

The packages this plan touches or leans on:

```
ok  	github.com/holzcloud/holzcloud-cms/cmd/holzcloud	9.095s
ok  	github.com/holzcloud/holzcloud-cms/internal/admin	128.069s
ok  	github.com/holzcloud/holzcloud-cms/internal/auth	1.752s
ok  	github.com/holzcloud/holzcloud-cms/internal/config	2.146s
ok  	github.com/holzcloud/holzcloud-cms/internal/web	4.694s
```

An earlier full run at 05:43-05:45Z also passed every package, but `HEAD` moved from `4fe4104` to `f333a7b` while it ran — another agent committed — so it is not quoted as the result. This is the third wave in a row to hit that, and re-running with the check at both ends is what makes the quoted run mean anything.

Plan-level checks:

| Check | Result |
|---|---|
| `go build ./...` and `go test ./...` | clean / 0 failures |
| SSO sign-out lands on the outpost's path; password sign-out on `/admin/login` | `TestLogoutOfAnSSOSessionSignsOutAtTheIdentityProvider`, `TestLogoutOfAPasswordSessionIsExactlyWhatItWas` |
| `Location` always begins with one slash and carries no scheme | `TestLogoutTargetIsAlwaysARelativePath`, four explicit conditions |
| `via_sso` read before `Destroy` | source-order gate (18 < 34) **and** `TestLogoutReadsTheViaSSOFlagBeforeTheSessionIsDestroyed`, plus mutation 2 |
| `internal/web/headers.go` unchanged, admin security-header test green | `git diff` 0 lines; `TestAdminResponsesCarrySecurityHeaders` PASS |
| No route, no template, no new string | 159/68/1321-with-both-strings-attributed |

`-run 'Logout|SignOut'` was checked against the test names it has to match: it selects all nine, after wave 5 lost a gate to a pattern that matched none of its twelve tests.

## Issues Encountered

**Package `admin` did not compile at 05:31Z, and the cause was not this plan.** `go vet ./internal/admin/` reported `not enough arguments in call to auth.MustHaveSecondFactor` at `internal/admin/twofactor.go:167`. Plan **10-06** was mid-flight in the same tree, changing that function's arity so the compiler would enumerate its five call sites (`10-CONTEXT.md`'s D-04 correction). `git log` said so before any diagnosis was attempted. Waited; it compiled again ten seconds later, once 10-06 had updated its call sites.

**A whole-package mutation run at 05:39Z showed six red tests belonging to 10-06** (`TestSecondFactorDisableLetsAnSSOAdminThrough`, `TestAccountScreenDoesNotCallTheSecondFactorCompulsoryForAnSSOAdmin` and four siblings, all in `internal/admin/twofactor_sso_test.go`). They were 10-06's RED phase, not a consequence of the mutation under test. The mutation was re-run under `-run 'Logout|SignOut'` and the finding stands. **Every mutation in this plan was scoped to that pattern for exactly this reason** — a package-wide mutation run in a shared tree measures the other agent as much as the mutation.

Both are recorded rather than smoothed over, because the phase now has three waves' worth of failures in packages nobody touched, and the cheapest defence is that each report says which agent owned the failure.

## Threat Flags

None. This plan adds no network endpoint, no auth path, no file access and no schema change. The one surface it touches — a redirect target reaching a browser — is `T-10-40` in the plan's own register, and it is mitigated by construction (`config.Load`'s `isLocalPath`) and asserted in both directions.

## Known Stubs

None. Every branch of `HandleLogout` is reached by a test, and no value is hardcoded that a later plan has to replace.

## User Setup Required

None from this plan. Two things belong in plan 10-08's `DEPLOY.md`, and are noted here so they are not lost:

- `HOLZCLOUD_SSO_SIGN_OUT_PATH` defaults to `/outpost.goauthentik.io/sign_out` and **must remain a path on this server**. The process refuses to start on anything else, including `//host` and `/\host`.
- If an installation ever routes the outpost on a **separate host**, this setting can no longer express it, and the Content-Security-Policy work recorded above becomes necessary at the same moment. The two tests named there are what will say so.

## Next Phase Readiness

- **SSO-08 is met in code and asserted.** What no test here can observe is the other end of the redirect; that is D10 above, and it belongs to plan 10-10's browser pass, which `10-CONTEXT.md` requires to be run twice — once with forward auth on and once off.
- **Plan 10-08 (deployment docs)** should carry the two `DEPLOY.md` notes above and the operator-side verification `10-CONTEXT.md` already names.
- **Plan 10-06** owes `go run ./tools/i18n -write` for its two strings; the catalogues currently read `2 offen`, and this plan minted none of it.
- **Phase 12's translation sweep** inherits one German comment at the top of `HandleLogout`, left deliberately intact.

---
*Phase: 10-authentik*
*Completed: 2026-09-08*

## Self-Check: PASSED

Every claim above was checked against disk rather than recalled:

- `internal/admin/login_test.go`, `internal/admin/login.go` and this file exist.
- `c0d9a4b`, `824592c` and `bd0e65e` are all reachable in `git log --oneline --all`.
- All ten test functions named in this summary are present by name in
  `internal/admin/login_test.go`, and `-run 'Logout|SignOut'` selects exactly
  ten `--- PASS` lines — the gate's pattern matches every test the gate is
  supposed to cover.
