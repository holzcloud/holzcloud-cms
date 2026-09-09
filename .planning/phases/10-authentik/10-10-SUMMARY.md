---
phase: 10-authentik
plan: 10
subsystem: verification
tags: [browser-pass, sso, forward-auth, second-factor, milestone-close-out, i18n, no-javascript]

requires:
  - phase: 10-01
    provides: "HOLZCLOUD_LISTEN and the seven SSO settings this pass configured"
  - phase: 10-02
    provides: "the peer check, the header strip and the constant-time secret this pass drove from an untrusted address"
  - phase: 10-03
    provides: "ForwardAuthSignIn, the fourth caller of completeLogin, and the seven deliberately-unchanged things this pass re-measured"
  - phase: 10-04
    provides: "provisionSSOUser, whose one-website result is the pass's most important screenshot"
  - phase: 10-05
    provides: "syncRightsFromGroups and the refusal that closes D-01's second road"
  - phase: 10-06
    provides: "MustHaveSecondFactor(role, viaSSO) and the two admin screens that carry SSO-07"
  - phase: 10-07
    provides: "HandleLogout's identity-provider branch"
  - phase: 10-08
    provides: "deploy/DEPLOY.md, whose single sign-on section this pass followed as an operator would"
  - phase: 10-09
    provides: "the two catalogue sentences this pass compared word for word against the screen"
  - phase: 11-07
    provides: "the method: a stdlib-only CDP client against a scratch installation, every step written in words"
provides:
  - "Three driven passes: forward authentication on (11 steps), forward authentication off (10 steps), and the v1.6 close-out (18 steps) — 67 screenshots, every outcome in words"
  - "Criterion 2 driven over IPv4 and IPv6 from non-loopback addresses WITH a positive control, so the refusals are proved to be about the peer and not about a gate that refuses everything"
  - "D-01's second road seen: losing every website group produces the login form AND leaves user_websites untouched — the empty write never happens"
  - "Criterion 5 answered twice: driven in a browser, and measured as a source diff against the phase-10 baseline"
  - "A QUAL-01 finding the i18n gate is structurally blind to: five German sentences printed to an English admin, one of them Phase 8's own, while `go run ./tools/i18n` reports 0 offen, 0 verwaist"
  - "The eighth and ninth instance of 'a gate must measure what its name claims' in this phase — both of them in gates this plan itself ran"
  - "SSO-03 and SSO-04 marked complete: both were delivered by 10-02, both are driven in this pass, and both still read Pending in REQUIREMENTS.md — the fifth occurrence of the ledger drifting behind the code"
affects: [verify-work, complete-milestone, ship, 12-umbenennung]

actuals:
  tokens: 11000
  tasks: 3
  commits: 1

tech-stack:
  added: []
  patterns:
    - "A stand-in for an authentik outpost: twenty lines of net/http/httputil in the session scratch directory, never committed, with what it does NOT cover stated before what it does"
    - "The counterfactual as the strongest evidence a stand-in can give: the same proxy restarted with no identity headers is what a real outpost looks like after sign-out, and it produced the login form"
    - "A browser pass driven through DOM.* only when scripting is off, because Runtime.evaluate is itself script and a measurement made through it is not a measurement of a scriptless page"

key-files:
  created:
    - .planning/phases/10-authentik/10-10-SUMMARY.md
  modified:
    - .planning/WINDOWS.md

key-decisions:
  - "The pass ran without a separate phase-10 review-fix round, because there was none: the nine waves each carried their own mutation discipline. The precondition's actual concern — that no user-visible change landed after the pass — is answered by naming every commit that touched the tree while it ran and showing that none of them is on a driven surface."
  - "Five untranslated German sentences are recorded in WINDOWS.md and NOT fixed. The class is pre-existing and milestone-wide, this plan has no production-code task, and the remedy for at least one of them (a fmt.Sprintf-assembled sentence) changes how the string is built rather than what it says."
  - "The German-literal gate is reported as a FLOOR of 8 with three shapes that slip past it named, rather than as a count. The wide form reads 59 and is too noisy to be a gate; three of the five sentences I actually saw are invisible to the narrow one."

patterns-established:
  - "A stand-in is worth more when its limit is driven than when it is disclaimed: step 9 drove the anonymous counterfactual instead of writing a paragraph about what a real outpost would do"
  - "Every harness error of my own is recorded beside the finding it nearly became — four of them here, each caught by measuring the consequence rather than trusting the API"

requirements-completed: [SSO-01, SSO-02, SSO-08, SSO-09, QUAL-01, QUAL-02]

coverage:
  - id: D1
    description: "A person authenticated at the identity provider reaches the admin without a second sign-in"
    requirement: SSO-01
    verification:
      - kind: automated_ui
        ref: "task 1 step 1: http://localhost:9090/admin/ -> Overview, no login form in between, redakteurin@example.com in the top bar, website Redaktion A, 15 nav items"
        status: pass
    human_judgment: false
  - id: D2
    description: "A provisioned account gets exactly its own website and is refused the other — D-01's inversion, seen"
    requirement: SSO-01
    verification:
      - kind: automated_ui
        ref: "task 1 step 2: the switcher offers only Redaktion A; /admin/websites/2/pages -> 403; the control /admin/websites/1/pages -> 200"
        status: pass
    human_judgment: false
  - id: D3
    description: "An SSO sign-in writes both the account-creation row and the sign-in row to the activity log, and a rights change writes its own"
    requirement: SSO-01
    verification:
      - kind: automated_ui
        ref: "task 1 steps 3 and 6/7: user.create + auth.login_success for redakteurin@example.com; two user.update rows carrying {\"field\":\"role\",\"from\":\"editor\",\"to\":\"admin\",\"via\":\"sso\"} and its reverse"
        status: pass
    human_judgment: false
  - id: D4
    description: "A demotion at the identity provider takes effect, not only a promotion"
    requirement: SSO-01
    verification:
      - kind: automated_ui
        ref: "task 1 step 7: groups back to redaktion-a alone -> 25 nav items become 15, /admin/users answers 403, the account screen reads Editor again"
        status: pass
    human_judgment: false
  - id: D5
    description: "Losing every website group produces the login form and does NOT write an empty rights assignment"
    requirement: SSO-01
    verification:
      - kind: automated_ui
        ref: "task 1 step 8: /admin/ -> the login form; server log reason=no_website_group; user_websites still holds (2, 1) afterwards"
        status: pass
    human_judgment: false
  - id: D6
    description: "An untrusted peer writing the headers by hand gets a login form, over IPv4 and over IPv6, against the finished binary"
    requirement: SSO-02
    verification:
      - kind: other
        ref: "task 1 step 10: 303 -> /admin/login from 192.168.0.63 and from [2a02:21b4:b04f:5500:96:4223:a182:78ba], five header shapes each incl. the underscore spelling and the real secret; positive control: the identical headers from 127.0.0.1 with the secret -> 200"
        status: pass
    human_judgment: false
  - id: D7
    description: "Signing out sends the browser to the identity provider's sign-out path"
    requirement: SSO-08
    verification:
      - kind: automated_ui
        ref: "task 1 step 9: POST /admin/logout -> 303 -> GET /outpost.goauthentik.io/sign_out -> 404 from the stand-in, which is the expected answer"
        status: pass
    human_judgment: false
  - id: D8
    description: "The next click does not silently sign the person back in once the provider has stopped asserting them"
    requirement: SSO-08
    verification:
      - kind: automated_ui
        ref: "task 1 step 9, the counterfactual: the same proxy restarted with -anon (no identity headers, which is what an outpost emits after sign-out) -> /admin/ lands on the login form"
        status: pass
    human_judgment: true
    rationale: "The stand-in cannot clear a real authentik cookie. What is proved is the CMS's half — the redirect — plus the counterfactual. The remaining half is the operator's verification in DEPLOY.md."
  - id: D9
    description: "With single sign-on off, the password path, the second factor, the recovery codes and the command line are what they were"
    requirement: SSO-09
    verification:
      - kind: automated_ui
        ref: "task 2 steps 1-10: login form with 2 inputs / 1 button / 0 links and no mention of any provider; password -> /admin/2fa -> Overview; 'As an administrator you cannot switch it off' with no control that does; the full enrolment; a recovery code taking 10 to 9; `holzcloud user 2fa disable` then password-alone then re-enrolment; sign-out to /admin/login and not to any outpost path"
        status: pass
      - kind: unit
        ref: "go test ./internal/auth/ ./internal/admin/ -run 'SecondFactor|Login|Logout|PasswordPath' -> 69 test entries, 0 FAIL"
        status: pass
      - kind: other
        ref: "source diff cdcfbab..HEAD over the password path: MustHaveSecondFactor's body is `role == \"admin\" && !viaSSO`, identical when viaSSO is false; login.html unchanged; VerifyDummyPassword hardened for callers config.Load cannot produce"
        status: pass
    human_judgment: false
  - id: D10
    description: "The provisioned account cannot be reached with a password, and the failure is indistinguishable from an unknown address"
    requirement: SSO-09
    verification:
      - kind: automated_ui
        ref: "task 2 step 8: four cases incl. an empty password posted past the client-side `required` — all four answer 'Email address or password is not right'"
        status: pass
      - kind: unit
        ref: "internal/admin#TestNoPasswordSignsInAProvisionedAccount"
        status: pass
    human_judgment: false
  - id: D11
    description: "Every one of the sixteen field kinds defined, filled, saved and read back"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "task 3 steps 1-2: the chooser offers all 16 in the Kinds order; 14 controls round-tripped byte for byte; gruppe carries a row, abschnitt is a heading"
        status: pass
    human_judgment: false
  - id: D12
    description: "The button row, its explicit no-answer choice, and a dependent field appearing and disappearing — with JavaScript off"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "task 3 step 3: 4 radio inputs incl. value=\"\", labelled '– not given –'; clicking it removes #feld_f_link's box entirely (display:none via :has()), clicking 'gruen' brings it back"
        status: pass
    human_judgment: false
  - id: D13
    description: "A renamed label changes every public page carrying it, with no page touched"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "task 3 step 5: pages row byte-identical (updated_at 2026-09-08T07:05:46Z, content_html 853 bytes, before and after) while the served page went Sommerweide -> Winterweide; /tag/sommerweide kept"
        status: pass
    human_judgment: false
  - id: D14
    description: "The snippet fields: five kinds filled and read back, absent from a page's edit form, absent from a block kind's list"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "task 3 steps 8-9: the fourth mode ?textbaustein=2 offers all 16 kinds; 5 values round-tripped; feld_s_* count on a page edit form is 0 while its own feld_f_* count is 20"
        status: pass
    human_judgment: false
  - id: D15
    description: "Both import paths, both collision answers, and the dry run agreeing with the write"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "task 3 steps 12-14: new website 3/0/0 predicted and written; existing+skip 1/0/2 predicted and written; existing+update 0/3/0 predicted and written with the title actually changing; the hostile file's report table equals the dry run's, row numbers 5 and 6 in both"
        status: pass
    human_judgment: false
  - id: D16
    description: "Everything 09-GAPS.md changed after Phase 9's own browser pass, driven again"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "task 3 step 15: the five default boxes on the mapping card; the dry run naming labels on both arms; the within-file duplicate address sentence; a default-only Schlagwörter target predicting 2 and creating exactly 2"
        status: pass
    human_judgment: false
  - id: D17
    description: "Every screen touched in A, B and C driven once with JavaScript switched off"
    requirement: QUAL-02
    verification:
      - kind: automated_ui
        ref: "task 3 step 17: field screen (create), block-kind and snippet field screens, snippet save, and the whole CSV path — panel <details>, file, mapping, sample stepper, dry run, import — all through DOM.* and real input events with Emulation.setScriptExecutionDisabled on"
        status: pass
    human_judgment: false
  - id: D18
    description: "`go run ./tools/i18n` reports 0 offen, 0 verwaist across everything v1.6 added"
    requirement: QUAL-01
    verification:
      - kind: other
        ref: "1321 strings; en/es/fr/it each '1321 übersetzt, 0 offen, 0 verwaist'; de-CH 73 Abweichungen 0 ohne Gegenstück; fr-CH 4, it-CH 9"
        status: pass
    human_judgment: false
  - id: D19
    description: "The catalogues are green and five German sentences still reach an English admin"
    verification: []
    human_judgment: true
    rationale: "Open finding. Recorded as WINDOWS.md entries 12-16, not fixed here: the class is pre-existing and milestone-wide, this plan carries no production-code task, and one remedy changes how a sentence is assembled rather than what it says."

duration: 55 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 10: The half no test can hold, and the milestone's close-out — Summary

**Three passes driven against a scratch installation in a real browser — forward authentication on, forward authentication off, and everything v1.6 added — 67 screenshots and 39 numbered steps, which confirmed every criterion this phase claims and found five German sentences printed to an English admin that `go run ./tools/i18n` cannot see and reports 0 offen about.**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-09-08T06:33:00Z
- **Completed:** 2026-09-08T07:28:00Z
- **Tasks:** 3 of 3
- **Steps driven:** 11 + 10 + 18 = 39, with 67 screenshots
- **Files modified:** 2 (this summary, `.planning/WINDOWS.md`)

## The precondition, and why it is met differently

Task 1's precondition names a **code-review fix round for phase 10**. There is none, and there was never going to be one: the nine waves each carried their own mutation discipline, and the phase has produced 60+ mutations with the reds recorded in the wave summaries. Noted rather than halted on, as instructed.

What the precondition is *actually* about is the failure that cost Phases 7, 8 and 9 their signatures — **a user-visible change landing after the pass was signed off**. That question is answerable, and the answer is on the record rather than asserted:

```
HEAD when the binary was built     24fa2ee   git status --porcelain: empty
HEAD when the gates were run       34df8f8   git status --porcelain: empty
```

Four commits landed in the tree while the passes ran. Every one of them:

```
34df8f8  docs(06)     .planning/phases/06-aufr-umen/06-SECURITY.md
3aca298  chore        plugins/*/plugin.wasm, plugins/*/*.zip  (11 rebuilt artefacts)
539a3ca  docs         plugins/README.md, sdk/plugin.go, tools/wasm/main.go
4580143  chore        plugins/kontaktformular/*
```

`git diff --stat 24fa2ee..HEAD` touches **no** file under `internal/`, `cmd/holzcloud/templates/`, `cmd/holzcloud/assets/` or `internal/i18n/`. No plugin was installed on any of the three scratch installations, so none of the rebuilt WebAssembly ran. The pass was driven against `24fa2ee`; the mechanical gates below were run against `34df8f8`. The condition the precondition protects held.

## The instrument, and the stand-in

**The browser.** Chrome 152.0.7977.82 headless, driven over the DevTools Protocol through a WebSocket client written with the Python standard library only — no npm, no Node, no pip. Screenshots were captured for every numbered step into the session scratch directory. **They are not the evidence.** `.playwright-mcp/` and the scratch path are both outside the repository, so every claim below is a number or a string somebody else can re-measure. That is Phase 11's method and this document copies it deliberately.

**Where scripting is off, the DOM is read through `DOM.*` and never through `Runtime.evaluate`** — evaluating an expression *is* script execution, and a page measured through it is not a page without JavaScript. Clicks are `Input.dispatchMouseEvent` at a box from `DOM.getBoxModel`; typing is `Input.dispatchKeyEvent`.

**The stand-in for the identity provider.** There is no Authentik on this machine and installing one is not what criterion 1 asks for. What forward authentication consumes is a header set from a trusted peer carrying a shared secret, so the stand-in is a twenty-line Go reverse proxy in the scratch directory (`httputil.NewSingleHostReverseProxy`) listening on `127.0.0.1:9090`, deleting every inbound `X-authentik…`/`X_authentik…` spelling the way a fixed Caddy does, then setting `X-authentik-username`, `-email`, `-name`, `-groups` and `X-Holzcloud-Proxy-Secret` from its own flags. Restarting it with different flags is what steps 6, 7, 8 and 9 need. It is never committed.

**What it does not cover, stated before what it does:**

- the outpost's own authentication — whether Authentik actually signed the person in;
- the `copy_headers` behaviour and the explicit `request_header -X-Authentik-…` deletes that `deploy/Caddyfile.example` addresses;
- CVE-2026-30851 itself, which lives in Caddy and not here;
- clearing a real provider cookie at sign-out (see step 9).

Those four are the operator's verification in `DEPLOY.md`, which is where `10-CONTEXT.md` puts them. A stand-in honest about its edges is worth more than one described as the real thing.

---

# Task 1 — the sign-on pass, forward authentication ON

Setup: a scratch data directory, migrations 0 → 51, first-run setup through `/admin/setup` creating `admin@test.local` with a **password** and the compulsory TOTP, then two websites created through `/admin/websites/new`. Then the server restarted with `HOLZCLOUD_SSO_ENABLED=true`, a secret, `HOLZCLOUD_SSO_ADMIN_GROUP=holzcloud-admins`, `HOLZCLOUD_SSO_WEBSITE_GROUPS=redaktion-a=1,redaktion-b=2`, `HOLZCLOUD_SSO_PROVISION=true`, `HOLZCLOUD_SSO_DEFAULT_WEBSITE=1`.

**One thing worth reading in the start-up log before any step:**

```
"listen":"127.0.0.1"  "sso_enabled":true  "sso_configured":true  "sso_provision":true
"sso_admin_group":"holzcloud-admins"  "sso_default_website":1  "sso_website_groups":2
"sso_sign_out_path":"/outpost.goauthentik.io/sign_out"
```

There is no `sso_secret` key in that dump, and no value resembling the secret anywhere in the log. SSO-04's "environment rather than database, and never logged" is visible in the one place an operator would notice it going wrong.

### Step 1 — straight in *(t1-01-dashboard-ohne-anmeldung.png)*

`http://localhost:9090/admin/` as `redakteurin@example.com`, groups `redaktion-a`, from a browser profile with no cookies.

```
final URL          http://localhost:9090/admin/       (no redirect to /admin/login)
<title>            Overview — Holzcloud
<h1>               Overview
login form?        False
top bar            SPAN :: redakteurin@example.com
current website    Redaktion A
nav items          15
```

Criterion 1 in one screenshot: a dashboard, with nothing in between.

### Step 2 — the account belongs to one website *(t1-02a, t1-02b)*

**This is the most important measurement in the phase.** `NewWebsiteAccessLookup` reads `assigned == 0 || mine > 0` — "no assignment means every website" — so a provisioned account with zero `user_websites` rows would be an editor of everything.

```
the website switcher offers      Redaktion A         (one entry, and "All websites …")
/admin/websites lists            Redaktion A         (one row)
/admin/websites/2/pages          403   body: "Diese Website gehört nicht zu deinem Zugang."
/admin/websites/1/pages          200   Pages – Redaktion A — Holzcloud     (the control)
```

The provisioning wrote the row. The inversion did not happen.

*(The 403's body is German on an English admin — see Findings.)*

### Step 3 — the activity log *(t1-03-protokoll.png)*

As the password administrator in a second browser profile, `/admin/protokoll`:

```
08.09.2026 06:40 | redakteurin@example.com | auth.login_success | user #2
08.09.2026 06:40 | redakteurin@example.com | auth.login_success | user #2
08.09.2026 06:40 | redakteurin@example.com | user.create        | user #2
```

Both rows are there, so the SSO path went through `completeLogin` and everything after this is trustworthy. Two `auth.login_success` rows because the pass reached the proxy twice from cookie-less clients (a `curl` probe and then the browser); the *third* visit, with the session cookie, wrote no row — which is `ForwardAuthSignIn`'s step 3 working.

### Step 4 — the second factor, on the person's own screen *(t1-04-konto-sso.png)*

`/admin/konto` as the SSO person. The sentence plan 10-06 added, compared word for word against `10-09-SUMMARY.md`'s English value:

> You came in here through your organisation's sign-in. Two-step verification is required there by whoever signed you in – this installation does not ask for it a second time.

Identical, en-dash included. Role reads **Editor**. The word "compulsory" does not occur on the page. Below the sentence sits "off / Recommended: a guessed password alone is then no longer enough. Set it up now" — consistent with *not compulsory*, and they may still enrol.

### Step 5 — the second factor, on the administrator's screen *(t1-05-users-hinweis.png)*

`/admin/users` as the password administrator.

```
notice index in the HTML   13445        <table> index   13653       notice above the table: True
```

> Whoever signs in through the organisation's sign-in brings their two-step verification with them from there. Whether it is required is decided by the organisation's sign-in and not by this installation.

Also word for word from the catalogue. And the row:

```
Rita Redakteurin | redakteurin@example.com | Editor | 1 of 2 websites
```

### Step 6 — the promotion *(t1-06, t1-06b)*

Stand-in restarted with `holzcloud-admins|redaktion-a`.

```
reload with the SAME session   nav items 15    (unchanged)
after dropping the cookie      nav items 25    Users, Activity log, Plugins, Email,
                                               AI access, Languages, Brand: all present
/admin/konto                   Role Administrator
```

**Named because "reload" as the plan words it is not enough, and that is correct rather than a defect.** `internal/admin/forwardauth.go:126-136` leaves an already-signed-in session exactly as it is, and says why in the source: otherwise every request would rotate the token and write a sign-in row, and somebody signed in by password on a shared machine would be silently swapped for whoever the proxy last asserted. So SSO-06's "re-applied at **every sign-in**" is exactly the guarantee — a rights change at the identity provider reaches this installation at the person's next sign-in, not on their next click. An operator should know that; it is the difference between "revoked" and "revoked when their session ends".

The log rows, read from `activity_log.metadata`:

```
{"field":"role","from":"editor","to":"admin","via":"sso"}
```

### Step 7 — the demotion *(t1-07-degradierung.png)*

The half that is easy to skip. Stand-in restarted with `redaktion-a` alone.

```
fresh sign-in     nav items 25 -> 15
                  Users, Activity log, Plugins, Email, AI access, Languages, Brand,
                  Fields, Content kinds, Block kinds:  all ABSENT
/admin/users      403
/admin/konto      Role Editor
activity_log      {"field":"role","from":"admin","to":"editor","via":"sso"}
```

The direction nobody worries about, and the direction SSO-06 exists for, both work.

### Step 8 — losing every website group *(t1-08-keine-website-gruppe-login.png)*

Stand-in restarted with no groups at all.

```
/admin/       ->  http://localhost:9090/admin/login
                  <title> Sign in — Holzcloud     login form present: True
                  nav items: []      (not a dashboard, and not a dashboard of every website)

server log:  WARN "forward auth rights synchronisation refused the sign-in"
                  err="sync: no group maps to a configured website" reason="no_website_group"
             WARN "forward auth sign-in refused" reason="no_website_group" ip="127.0.0.1"
```

And the half that closes D-01's **second** road — the one neither `ROADMAP.md` nor `10-CONTEXT.md` named until the planner found it:

```
select user_id, website_id from user_websites   ->   2 | 1
```

The refusal did not write an empty assignment. The row that would have meant "every website" was never created.

### Step 9 — signing out *(t1-09a, t1-09b, t1-09c)*

The redirect chain, from Network events:

```
POST http://localhost:9090/admin/logout
GET  http://localhost:9090/outpost.goauthentik.io/sign_out        (redirected from 303)
     -> 404 page not found
```

The 404 is the correct and expected outcome against a stand-in: the CMS's job is to issue the redirect to the outpost's path, and it does.

**Then the second click, which is the whole of SSO-08.** With the stand-in still asserting headers unconditionally, `/admin/` signs the person straight back in — the honest limit of the stand-in, exactly as `<flagged_assumptions>` predicted. Rather than write a paragraph about what a real outpost would do, I drove the counterfactual: the same proxy restarted with `-anon`, which is what an outpost emits once its own cookie is gone.

```
proxy asserting headers   -> /admin/          signed in, no login form
proxy asserting nothing   -> /admin/login     login form present: True
```

That is the strongest thing this stand-in can say, and it is stronger than the plan asked for. The remaining half — that a real authentik outpost actually stops asserting after `/sign_out` — is the operator's verification in `DEPLOY.md`.

*An incident of mine, recorded because the first attempt produced a wrong-looking result.* The first sign-out click did nothing: the form lives inside a **closed** `<details class="switcher">`, and there are **two** such elements on the page, so my selector opened the website switcher and clicked into empty space. Caught because the URL did not change, not because anything told me. Fixed by selecting `details.switcher:has(form[action$="/admin/logout"]) > summary`. My harness, not the application.

### Step 10 — criterion 2's one command, against the finished binary

The server restarted with `HOLZCLOUD_LISTEN=0.0.0.0` exactly as `deploy/DEPLOY.md:308` instructs, and put back afterwards as the same paragraph instructs. **The addresses used, recorded as the plan requires:**

```
IPv4   192.168.0.63                                    (en0, non-loopback)
IPv6   2a02:21b4:b04f:5500:96:4223:a182:78ba           (en0, global, autoconf secured)
listener as the OS sees it:  holzcloud … IPv6  TCP *:8099 (LISTEN)
```

The documented command, and four more shapes beside it:

```
                                             from 192.168.0.63          from [2a02:…:78ba]
no header at all                             303 …/admin/login          303 …/admin/login
X-authentik-email: stranger@example.com      303 …/admin/login          303 …/admin/login
X-authentik-username: redakteurin            303 …/admin/login          —
X_authentik_username: redakteurin  (underscore) 303 …/admin/login       —
username + the REAL shared secret            303 …/admin/login          303 …/admin/login
```

**Neither leg is a `Connection refused`.** Both are `303` to `…/admin/login` — a login form, not a dashboard.

**And the positive control, without which every line above would also be produced by a gate that refuses everything.** The *identical* complete header set — username, email, name, groups and the real secret:

```
from 127.0.0.1                                  200          (signed in)
from 192.168.0.63                               303 …/admin/login
from [2a02:21b4:b04f:5500:96:4223:a182:78ba]    303 …/admin/login
from 127.0.0.1, WRONG secret                    303 …/admin/login
from 127.0.0.1, NO secret                       303 …/admin/login
```

The refusals are about the peer and about the secret, not about a blanket refusal. And the server logged **nothing** for the untrusted attempts — correct, because the peer check happens before any header is read, so there is nothing to log.

**Why the IPv6 leg answered at all, since `0.0.0.0` is an IPv4 literal.** Measured rather than assumed, then explained: `lsof` shows the listener as `IPv6 *:8099`. `$GOROOT/src/net/ipsock_posix.go`'s `favoriteAddrFamily` returns `AF_INET6` with `ipv6only=false` for `mode == "listen"` when the local address `isWildcard()` and `supportsIPv4map()` — and `0.0.0.0` is a wildcard. So Go gives a dual-stack socket, and `DEPLOY.md:308`'s advice to set `0.0.0.0` for the both-families test is correct on the linux/amd64 target too, with the one caveat of a kernel set to `net.ipv6.bindv6only=1`. The settings table at `DEPLOY.md:124` already names `::` for both families; the two lines do not contradict each other.

### Step 11 — scripting off *(t1-11a, t1-11b, t1-11c)*

Steps 1, 4 and 9 repeated with `Emulation.setScriptExecutionDisabled`, read through `DOM.*` only.

```
step 1   /admin/  ->  Overview — Holzcloud, no login form, identity in the DOM, 15 nav items
                      <script> tags on the page: 1   ->  <script src="/assets/htmx.min.js" defer>
step 4   /admin/konto  ->  the organisation sentence, word for word; Role Editor;
                      the language form submitted by a real click -> flash "Language saved"
step 9   the <details> opened by a real click on its <summary> (plain HTML, no script)
         POST /admin/logout -> 303 -> /outpost.goauthentik.io/sign_out -> 404
         hx- attributes on the logout form: []      (it needs no htmx and has none)
```

Every control works. htmx is enhancement, as claimed.

---

# Task 2 — the same server with forward authentication OFF

Same data directory, `HOLZCLOUD_SSO_ENABLED` unset and every other `HOLZCLOUD_SSO_` variable removed, the stand-in stopped (`port 9090: Connection refused`). Start-up log: `"sso_enabled":false, "sso_configured":false`.

Driven in the order 1, 2, 3, 7, 5, 6, 4, 8, 9, 10 rather than 1…10, because the administrator was already enrolled from task 1: step 3 is therefore the `/admin/konto` variant the plan explicitly allows, and step 4's full enrolment is walked after step 6's command-line disable, where it doubles as the proof that step 6 leaves the account asking to enrol again.

### Step 1 — the login form *(t2-01-login-unveraendert.png)*

```
/admin/  ->  /admin/login          <title> Sign in — Holzcloud
inputs   ['gorilla.csrf.Token', 'email', 'password']
buttons  ['Sign in']
links    []                        (none at all)
visible  "Holzcloud  Please sign in  Email  Password  Sign in"

the words organisation / authentik / identity / Single / SSO / provider / outpost,
anywhere in the HTML:   all False
```

Nothing new on it, and nothing hidden in it either.

### Step 2 — signing in with a password *(t2-02a, t2-02b)*

```
password  ->  /admin/2fa   "Two-step verification  Enter the six-digit code from your
                            authenticator app.  Code  Continue
                            No access to the device? Use a recovery code · Cancel"
TOTP      ->  /admin/       <h1> Overview
```

### Step 3 — the second factor is compulsory again *(t2-03-konto-pflicht.png)*

`/admin/konto`, the block in full:

> Two-step verification **switched on**. Signing in also asks for a code from your authenticator app. Recovery codes left: 10. New recovery codes · Set up a new device. **As an administrator you cannot switch it off.** If the device is gone and no recovery code is left, this helps on the server: `holzcloud user 2fa disable -email admin@test.local`

```
forms on the page        /admin/logout, /admin/konto/sprache, /admin/2fa/codes, /admin/2fa/einrichten/neu
any control that turns it off   []
the organisation sentence present   False
```

`MustHaveSecondFactor(role, false)` seen rather than asserted, and the SSO wording does not leak onto the password path.

**The guard behind it, driven directly.** There is no form for switching it off, so I posted to the route:

```
POST /admin/2fa/aus  as an administrator
  -> 200, back on /admin/konto, flash:
     "Für Administratoren ist die Bestätigung in zwei Schritten Pflicht. Wenn das Gerät
      verloren ist, hilft ein Wiederherstellungscode oder „holzcloud user 2fa disable"
      auf dem Server."
```

That is `internal/admin/twofactor.go:281` — the call site `10-CONTEXT.md` singles out as "not a screen flag" — refusing correctly. *In German, on an English admin: see Findings.*

### Step 4 — the whole enrolment *(t2-04-2fa-einrichten.png, t2-04-wiederherstellungscodes.png)*

Walked after step 6's disable, so it is also the proof that the way back in works.

```
the QR    an inline <svg viewBox="0 0 61 61">, 19 330 bytes, one <rect> and one <path>
          — a real 57-module code with a 2-module quiet zone, no <img>, no data: URI,
            nothing fetched from anywhere
the key   "XH6G EL2O JV65 IAQU IOAT KGIO GYBN DYJP"
the first code  derived from that key with hmac-sha1 and accepted, which proves the key
                printed on the screen IS the account's secret
recovery  10 codes shown, e.g. nnpmr-j9g42 … , with the sentence
          "These codes are shown this once only. Afterwards they exist nowhere — not in
           the database either, which holds only their checksums."
```

**One observation.** The screen says "Print them out" and the stylesheet carries **no** `@media print` block (`grep -c "@media print" cmd/holzcloud/assets/admin.css` → 0). The page is printable in the plain sense — any page is — but nothing is done to make the printout readable. An observation, not a defect claim.

### Step 5 — a recovery code *(t2-05a, t2-05b)*

```
/admin/2fa  ->  "Use a recovery code"  ->  /admin/2fa?wiederherstellung=1
                "Enter a recovery code. Every recovery code works exactly once."
used c3rdx-qf98x   ->  /admin/  <h1> Overview
/admin/konto       ->  Recovery codes left: 9        (was 10)
```

### Step 6 — the command line *(t2-06-nach-cli-neu-einrichten.png)*

Server stopped, as an operator with shell access would.

```
holzcloud user 2fa status
  ID  ROLE    EMAIL                    SECOND FACTOR  RECOVERY CODES LEFT
  1   admin   admin@test.local         on             9
  2   editor  redakteurin@example.com  off            0

holzcloud user 2fa disable -email admin@test.local
  Second factor removed for admin@test.local.
  The account can now sign in with its password alone.
  As an administrator it will be asked to set up a new authenticator immediately.

holzcloud user 2fa status
  1   admin   admin@test.local         off            0
```

Restarted, then:

```
password alone  ->  http://127.0.0.1:8099/admin/2fa/einrichten
```

Exactly what `DEPLOY.md:501-503` promises, sentence for sentence.

### Step 7 — signing out *(t2-07-abmeldung-login.png)*

```
sign-out       ->  /admin/login       <title> Sign in — Holzcloud, login form present
second visit to /admin/  ->  /admin/login
```

No outpost path anywhere in the chain.

### Step 8 — the SSO account is inert *(t2-08-sso-konto-inert.png)*

```
redakteurin@example.com  + 'irgendein passwort'    "Email address or password is not right"
gibtesnicht@example.com  + 'irgendein passwort'    "Email address or password is not right"
admin@test.local         + a wrong password        "Email address or password is not right"
```

The empty-password case is blocked client-side by `required`, so it was posted past that check with the CSRF token:

```
redakteurin@example.com  + ''   200  "Email address or password is not right"  (1646 bytes)
gibtesnichtaber@…        + ''   200  "Email address or password is not right"  (1654 bytes)
```

Same message; the responses differ only by the echoed address. Nothing on the screen says the account exists. `internal/admin#TestNoPasswordSignsInAProvisionedAccount` holds the same property mechanically.

### Step 9 — the user list *(t2-09-users-ohne-hinweis.png)*

```
the SSO notice ("Whoever signs in through …")   False
the word "organisation" anywhere on the page    False
table headers   Name | Email | Role | Rights | Last signed in | Actions
rows            admin@test.local (Administrator), redakteurin@example.com (Editor, 1 of 2 websites)
```

Absent with single sign-on off, present with it on (task 1 step 5). SSO-07's conditional is real.

### Step 10 — scripting off *(t2-10a, t2-10b, t2-10c)*

```
step 1  /admin/ -> /admin/login, inputs [csrf, email, password],
        hx- attributes on the login form: ['hx-disabled']   (a hint, not a dependency)
step 2  typed by real key events -> /admin/2fa -> a recovery code -> /admin/, 25 nav items
step 7  POST /admin/logout -> 303 -> /admin/login;  second visit -> the login form
```

### And criterion 5 measured a second way

A browser pass answers "does it still behave the same". A source diff answers "did anything change at all". Both, because criterion 5 is a claim about absence. Against the phase-10 baseline `cdcfbab`:

| file | what moved |
|---|---|
| `cmd/holzcloud/templates/admin/login.html` | **nothing** |
| `internal/auth/twofactor.go` | `MustHaveSecondFactor(role)` → `(role, viaSSO)`, body `role == "admin" && !viaSSO`. With `viaSSO` false the expression is the old one, character for character |
| `internal/auth/session.go` | `SessionKeyViaSSO` **added**; nothing removed or changed. `scs` answers false for an absent key, so a password session takes no new branch |
| `internal/auth/password.go` | `VerifyDummyPassword` now defaults all four Argon2 parameters, not only the key length. `config.Load` cannot produce a struct that reaches the new branches — it floors memory at 8 KB and refuses zero iterations and zero parallelism. A guard for other callers, not a behaviour change on the password path, and step 8 confirms it empirically |
| `internal/user/store.go` | a doc comment plus the empty-address refusal that phase 10 wave 3's green mutation uncovered |

Nothing else on the password path moved.

---

# Task 3 — the milestone close-out

A fresh scratch data directory, the same binary, single sign-on off, one website, one administrator.

## A — Phase 7: every field kind

### Step 1 — all sixteen, defined *(t3-01-felder.png, t3-A1-alle-16-feldarten.png)*

The chooser on `/admin/websites/1/felder`, read from the DOM:

```
['text','langtext','code','zahl','bereich','datum','zeit','janein','auswahl',
 'mehrfachauswahl','bild','link','verweis','schlagwort','gruppe','abschnitt']
equal to field.Kinds, in order:  True
```

One field of each was created — sixteen "Field created" flashes, sixteen rows on the list, each showing its kind by name: Short text, Long text, Code, Number, Range, Date, Time, Yes/no, Choice, Multiple choice, Image, Link, Reference, Label, Group (0), Section.

**And the exclusion** *(t3-A1b-bausteinart-feldauswahl.png)*. A block kind `Preisblock` created, then its field chooser:

```
offers   text langtext code zahl bereich datum zeit janein auswahl mehrfachauswahl bild link
absent   verweis  schlagwort  gruppe  abschnitt
```

`verweis` and `schlagwort` absent, as the plan asks — and `gruppe`/`abschnitt` with them, which is `BlockKinds()`'s "four exclusions" in `internal/field/field.go`.

### Step 2 — filled, saved, reloaded *(t3-A2c-alle-werte-zurueck.png)*

The step that catches an encoding that writes and does not read. Every value compared after a reload:

```
feld_f_text             'Bergkaese'                    OK
feld_f_langtext         'Zwei Zeilen\nueber den Hof.'  OK
feld_f_code             '<b>x</b>'                     OK
feld_f_zahl             '42'                           OK
feld_f_bereich          '7'                            OK
feld_f_datum            '2026-03-14'                   OK
feld_f_zeit             '06:30'                        OK
feld_f_auswahl          'gruen'                        OK
feld_f_bild             '1'                            OK
feld_f_link             'https://example.com/hof'      OK
feld_f_verweis          '2'                            OK
feld_f_schlagwort       'sommerweide'                  OK
feld_f_janein           checked                        OK
feld_f_mehrfachauswahl  ['rot','gruen','blau']         OK
```

Fourteen of the sixteen carry a scalar; `abschnitt` is a heading and `gruppe` is a container, driven separately *(t3-A2d)*: two sub-fields (`Tag` text, `Von` zeit) created, "Add a row" pressed, and the row's inputs appeared as `gruppe.f_gruppe.0.tag` and `gruppe.f_gruppe.0.von`.

### Step 3 — the button row and the condition, with JavaScript off *(t3-A3, t3-A3b, t3-A3c)*

`F auswahl` set to `knopfreihe`, `F link` made conditional on it. Then scripting switched off:

```
element used      radio inputs   (not a <select>)
radio count       4
values offered    ['', 'rot', 'gruen', 'blau']
the empty one's label   "– not given –"
wrapper classes   feld-schalter feld-schalter--knopfreihe / form-knopfreihe

start (F auswahl = gruen):        #feld_f_link  laid out 833x42
click "– not given –"             #feld_f_link  NO BOX  ->  display:none
click "gruen"                     #feld_f_link  laid out 833x42
```

Both clicks were real mouse events at boxes from `DOM.getBoxModel`. No `data-` attribute names the condition anywhere in the markup; the mechanism is the CSS rule at `admin.css:1115`, `:has(> .form-group input[type="radio"][value=""]:checked)`. The comment beside it explains why a button row always carries an empty radio — it is what that rule reads — and the pass confirms the reasoning holds in a browser.

### Steps 4, 5, 6 — on the public page

The home page is rendered by `home.html`, which prints no field list, so the fields were put on `/impressum`, which `page.html` renders. Public site reached with `Host: hof.localhost`.

```
F text              Impressum-Feld
F code              &lt;b&gt;x&lt;/b&gt;
F mehrfachauswahl   rot, gruen, blau
F schlagwort        Sommerweide
```

- **Step 4** — three values in one save, three after reload, three on the public page.
- **Step 6** — `<b>x</b>` printed **verbatim**: `&lt;b&gt;x&lt;/b&gt;` is in the source, a live `<b>x</b>` is not.
- **Step 5, first half** — the label prints its **name** (`Sommerweide`), not its address.

**Step 5, second half — the term renamed, the page untouched** *(t3-C…, measured on the pages row):*

```
page row BEFORE   id 2  updated_at 2026-09-08T07:05:46Z  content_html 853 bytes
rename            "Renamed. The address stays as it was so existing links keep working."
                  the table then reads: Winterweide  /tag/sommerweide  1 in use
page row AFTER    id 2  updated_at 2026-09-08T07:05:46Z  content_html 853 bytes   ← identical

public page before   <dd>Sommerweide</dd>
public page after    <dd>Winterweide</dd>
```

The page was not opened, not saved, not touched. Same row in the database, different page on the net — and the address held, so no link broke.

### Step 7 — `zeit` and `bereich`

```
#feld_f_zeit      <input type="time" …>
#feld_f_bereich   <input type="number" step="any" min="0" max="10">   ← not a slider
```

`bereich` is a number box, so the chosen number is readable before saving with no JavaScript at all — which is what `field.go`'s own comment promises ("ausdrücklich kein Schieber"). And empty is distinguishable from midnight at every layer:

```
saved 00:00   editor reads '00:00'   public page:  00:00
saved ''      editor reads ''        public page:  the field is not printed at all
```

## B — Phase 8: the snippet fields

### Step 8 — the fourth mode *(t3-B8, t3-B8b)*

`/admin/websites/1/felder?textbaustein=2` — the mode plan 08-03 added.

```
the chooser offers all 16 kinds     (the full palette, not BlockKinds() — as field.go:322 says)
five fields created:  S text, S code, S bereich, S zeit, S langtext
                      three of them are kinds Phase 7 added (210a6b9: zeit, bereich, code)
```

Filled, saved, reloaded — all five came back byte for byte, including `'<i>roh</i>'` and `'<script>alert(1)</script> und normaler Text'`.

### Step 9 — they appear nowhere else

Phase 8 criterion 3 asks for this in a browser and not only in a test:

```
on a page's edit form            feld_s_* controls: []   ABSENT
   (its own fields still there:  feld_f_* controls: 20)
in a block kind's field list     only "B code" — its own
in the website's field list      no S-prefixed row       ABSENT
```

### Step 10 — sanitisation

**The plan's step as written could not be reached, and that is a finding rather than a skip.** No shipped theme prints snippet field values. `internal/template/loader.go:390-398` exposes `.Site.Bausteinliste` with the comment *"The list is what a shipped theme can use"* — and `grep -rn "Bausteinliste" cmd/holzcloud/templates/public/` finds nothing. So there is no public page on which a snippet's `langtext` field can be seen at all. See *Not driven*.

What **is** reachable is the same sanitisation on the snippet's own body, through a shipped theme, and it was driven:

```
snippet body:  Rufe uns an. <script>alert(1)</script> <img src=x onerror=alert(2)>
               <a href="javascript:alert(3)">klick</a>

on the public page:  Rufe uns an. <img src="x"> klick
live <script>alert(1)</script>   False
onerror= anywhere                False
javascript: anywhere             False
every <script> on the page       ['<script type="application/ld+json">']
```

The single `<script>` is the data block a browser never executes and `internal/tmplmgr/script.go` exempts by name — the shape CLAUDE.md requires.

### Step 11 — the snippet that predates its fields

`Alt ohne Felder`, a plain Markdown body and no fields at all, called from a page with `[[snippet:alt]]`:

```
on the public page:   Ein <strong>alter</strong> Textbaustein.
raw "[[snippet:" left anywhere:   False
```

## C — Phase 9: both import paths

### Step 12 — a good file into a new website *(t3-C12a … t3-C12e)*

The CSV panel on `/admin/websites`, then the mapping screen:

```
5 columns auto-mapped by header:  target_0=title  target_1=slug  target_2=body
                                  target_3=status target_4=terms
five default boxes:  default_title default_slug default_body default_status default_terms
the sample row stepper:
   Sample: row 2 of 4   ['Erste Seite','erste','Ein Satz.','veroeffentlicht','hof']
   Sample: row 3 of 4   ['Zweite Seite','zweite','Noch ein Satz.','entwurf','hof|weide']
   Sample: row 4 of 4   ['Dritte Seite','dritte','Und noch einer.','veroeffentlicht','']
```

The dry run:

> Nothing has been written. What stands here is what would happen on import — not a single page has come into being yet, and none has been changed. File "gut.csv", 3 rows checked. **3 to create / 0 to update / 0 to skip. 2 labels will be created along the way, as far as they do not exist yet.** Nothing to report: every row of this file goes through.

The report:

> Import finished. File "gut.csv", 3 rows read, website "Aus der Tabelle". **3 created / 0 updated / 0 skipped.** Nothing to report: every row of the file went through.

And what is actually there:

```
Erste Seite   /erste   Published
Zweite Seite  /zweite  Draft          ← the file said entwurf
Dritte Seite  /dritte  Published
labels:  hof (2 in use)   weide (1 in use)      ← the 2 the dry run promised
```

### Step 13 — the same file into an existing website, both answers *(t3-C13a, t3-C13b)*

A second file changing row 1's title and text, keeping row 2, adding row 4.

```
collision = uebergehen      dry run  1 to create / 0 to update / 2 to skip
                            report   1 created  / 0 updated  / 2 skipped
                            "Erste Seite" still reads Erste Seite  (untouched)
                            "Vierte Seite" created

collision = aktualisieren   dry run  0 to create / 3 to update / 0 to skip
                            report   0 created  / 3 updated  / 0 skipped
                            "Erste Seite" now reads "Erste Seite GEAENDERT"
```

Both required by the roadmap's own planning note; the existing-website arm is the deliberate departure from `wordpress.go`'s rule. Dry run and write agreed on both, which is the property `TestCSVProbeAndStartAgreeOnEveryVerdict` holds mechanically.

### Step 14 — a hostile file *(t3-C14a … t3-C14d)*

Five hostile shapes in one file, imported into the website that has all sixteen fields:

| what was in the file | what the screen said |
|---|---|
| a header `Schlagwörter` with a **decomposed** umlaut (`o` + U+0308, `b'Schlagwo\xcc\x88rter'`) | recognised and auto-mapped to **Labels** |
| a **second column headed `Text`** | *"The same heading already stands over column 3. This one stays without a target until you choose one."* — target `none` |
| a `janein` cell reading **`nein`** | no error; `F janein` came back **unchecked** on the imported page |
| a **duplicate address within the file** (row 5 repeats row 2) | *"One row of this file wants an address that a row above it already takes. The first one creates; the later ones update or are skipped — the counts above already say so."* and, in the table, `skip · The page already exists at "h-eins". It was not touched. · One row: 5` |
| a row with **no title** | `skip · This row has no title. Without a title nobody will find the page again later. · One row: 6` |
| a **short row** (3 cells of 6) | **nothing** — see below |

The mapping screen also said, unprompted: *"These fields are not on offer, because a spreadsheet cell cannot fill them: F bild F verweis F gruppe F abschnitt."*

**The row numbers line up.** The stepper labels the first data row "row 2 of 6"; the report names the fourth data row "row 5" and the fifth "row 6". Both count the header as row 1 — the operator opens the spreadsheet at the number they were given.

**The report table is the dry run's table, unchanged:**

```
skip | The page already exists at “h-eins”. It was not touched.                        | One row: 5
skip | This row has no title. Without a title nobody will find the page again later.   | One row: 6
```

**The short row, recorded as an observation.** `internal/csv/csv.go:209-211` sets `FieldsPerRecord = -1` with the comment *"A short row becomes a reported row carrying its own row number instead of an error carrying a line number."* Driven: the short row did **not** blow up the file, which is what the comment promises, and it also produced no message of any kind — it simply became a page whose missing cells are empty (`Kurze Zeile`, body `nur drei Felder`, `F janein` unchecked, no labels). That is defensible, and an operator is not told that row 4 had three of six columns. Whether it should be is a design question, not a defect I am asserting.

And the `janein` trap in full, which is the one that would matter:

```
cell 'nein'                       ->  F janein checked = False
cell 'ja'                         ->  F janein checked = True
the cell missing entirely         ->  F janein checked = False
```

`nein` is not read as a non-empty truthy string.

### Step 15 — what Phase 9 changed after its own pass, driven again *(t3-C15a … t3-C15c)*

`09-GAPS.md` records four things that landed after Phase 9's browser pass was signed off. All four were re-driven, and each is above or here:

1. **The dry run predicting addresses a row above will take** (gap 2). Seen in step 14: the sentence *"One row of this file wants an address that a row above it already takes"*, and driven on **both** collision answers in step 13.
2. **The term pre-pass on the dry-run arm** (gap 2). Seen in steps 12 and 13: *"N labels will be created along the way, as far as they do not exist yet"* on both arms. The qualifier is deliberate — knowing which already exist would cost a query per name.
3. **The five default boxes on the mapping card** (gap 3). Seen in step 12: `default_title`, `default_slug`, `default_body`, `default_status`, `default_terms`.
4. **The default-only labels defect found while closing** (the dry run promised two and four were created). Driven directly, with a file that has **no** `Schlagwörter` column and `importiert|hofladen` typed into the Vorgaben box:

```
mapping     targets: title, slug, body   —   a labels column: none
dry run     2 to create / 0 to update / 0 to skip
            2 labels will be created along the way, as far as they do not exist yet
report      2 created / 0 updated / 0 skipped
afterwards  the new website's labels:  hofladen (2 in use)   importiert (2 in use)
```

**Promised 2, created 2, attached to both pages.** The defect does not recur, and the orphan its own fix could have manufactured was not manufactured.

## D — Phase 10: the sign-on path

Criterion 6's fourth item is satisfied by tasks 1 and 2 above, and is not repeated here. The steps that cover it: task 1 steps 1–11 (forward authentication on, both directions of a group change, the loss of every website group, the sign-out, and criterion 2's command over both address families) and task 2 steps 1–10 (the password path, the second factor, the recovery codes and the command-line way back in).

Phase 11's own pass is recorded in `.planning/phases/11-galerie/11-07-SUMMARY.md` and is cited rather than repeated, as instructed.

## E — the gate itself

### Step 16 — `go run ./tools/i18n`, on the finished tree

```
1321 Zeichenketten im Quelltext
de-CH.json   73 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1321 übersetzt, 0 offen, 0 verwaist
es.json      1321 übersetzt, 0 offen, 0 verwaist
fr-CH.json    4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1321 übersetzt, 0 offen, 0 verwaist
it-CH.json    9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1321 übersetzt, 0 offen, 0 verwaist
```

**Criterion 6's mechanical half is MET.** Read the Findings section before believing it means what it sounds like.

### Step 17 — every screen with scripting off

Beyond task 1 step 11 and task 2 step 10, driven here *(t3-E17a … t3-E17f)*:

```
the fields screen        a field created by real typing and a real click -> "Field created."
                         <script> tags on the page: 1 (the deferred htmx include)
                         the chooser still offers 16 kinds
the block-kind screen    12 kinds
the snippet screen       16 kinds; a snippet saved -> "Snippet saved"
the websites screen      the CSV panel present; hx- attributes on its form: ['hx-disabled']
the whole CSV path       <details> opened by clicking its <summary>  (plain HTML)
                         file chosen, website named by real key events
                         "On to the mapping"  -> Map the columns, targets 0..2
                         "next row"           -> Sample: row 2 of 3  ->  row 3 of 3
                         "Start the dry run"  -> /probe, "File \"ohne-skript.csv\", 2 rows checked"
                         "Import now"         -> /start, "Import finished"
```

A whole spreadsheet import, end to end, with JavaScript switched off. Every control is a plain form submit.

### Step 18 — the counting table, measured against the finished tree

| What | Expected | Measured | |
|---|---|---|---|
| strings in source | 1279 | **1321** | drift, see below |
| `0 offen, 0 verwaist` lines | 4 | 4 | ✔ |
| migrations | 49 | **51** | drift |
| packages under `internal/` | 40 | **41** | drift |
| files in `internal/web` | 14 | 14 | ✔ |
| files in `internal/admin` | 51 | **52** | drift |
| admin templates | 66 | **68** | drift |
| `layoutPageNames` entries | 50 | 50 | ✔ (2 of them `album_`, 4 `csv_`) |
| `adminProtectedMux.Handle` | 150 | **159** | drift |
| `adminOnly` rows | 19 | 19 | ✔ |
| files using `RemoteAddr` | 3 | 3 | ✔ |
| `IsTrustedPeer` mentions | 4 | 4 | ✔ |
| `completeLogin` call sites | 4 | 4 | ✔ |
| `MustHaveSecondFactor` non-test | 6 | **8 lines** | see below |
| `SetRights` call sites | 4 | 4 | ✔ |
| files using `BeginTx` | 14 | **15** | drift |
| `isTrustedProxy` outside `.planning/` | 0 | 0 | ✔ *with the corrected exclusion* |

**Every drift is Phase 11 landing in the same tree between this plan being written and being run**, and each is attributable:

- migrations 49 → 51: `00050_albums.sql` and `00051_album_updated_at.sql`. `11-07-SUMMARY.md` recorded the same divergence one number earlier.
- packages 40 → 41: `internal/album`.
- `internal/admin` 51 → 52: `album.go`. Admin templates 66 → 68: `album_list.html`, `album_edit.html`.
- `adminProtectedMux.Handle` 150 → 159: nine album routes. `11-07` measured 159/159 against its own expectation.
- `BeginTx` 14 → 15: `internal/album/store.go`.
- strings 1279 → 1321: Phase 11's 41 (1278 → 1318, then 1319 after its own fix), plus 10-09's 2 = 1321. Commit `24fa2ee` documents the mechanism by which this number can move without a sentence being written, and its package comment is worth reading beside the Findings below.

`layoutPageNames`, `adminOnly`, `RemoteAddr`, `IsTrustedPeer`, `completeLogin`, `SetRights` and `isTrustedProxy` — **the seven this phase must not have moved** — all read exactly what plans 10-02 through 10-05 recorded.

## Which check actually holds each property

Per the knowledge base's rule, for the gates where two things could be load-bearing:

- **"one trusted-peer check in this codebase"** — `grep -rn RemoteAddr … | wc -l` reads 3 (`web/clientip.go`, `web/forwardauth.go`, `web/logging.go`). The count is a tripwire; the property is held by `web.ForwardAuth` reading only `IsTrustedPeer`, which `TestForwardAuthRefusesAndFallsThrough` and step 10's positive control both exercise. The grep would not notice a fourth file that used `RemoteAddr` for something harmless.
- **`MustHaveSecondFactor` = 6** — the number is 8 lines, decomposed by hand rather than by a filter: **2 comments** (`auth/twofactor.go:38`, `admin/twofactor.go:383`), **1 definition** (`auth/twofactor.go:60`) and **5 call sites** (`auth:91`, `admin:174, 204, 281, 414`). Five is exactly D-04's corrected count, and the plan's "6" is definition + calls. The property — "exactly one home" — is held by the **signature**, not by the count: the parameter was added rather than a variant, so the compiler enumerates the callers.
- **`isTrustedProxy` = 0** — held by the corrected exclusion, see below.

---

# Findings

## 1. The catalogues are green and the admin speaks German

**Five sentences, seen with my own eyes on an English admin, while `go run ./tools/i18n` reports `0 offen, 0 verwaist` on all four catalogues.**

| where | what stood on the screen | when |
|---|---|---|
| the flash after creating a website | *Website angelegt – mit Startseite sowie Impressum und Datenschutzerklärung als Entwurf (3 Seiten)* | task 1 setup |
| the 403 body when an editor opens a website outside their access | *Diese Website gehört nicht zu deinem Zugang.* | task 1 step 2 |
| the `<title>` and `<h1>` of the field screen, all four modes | *Felder – Milchschaeferei* · *Baustein „Preisblock" – …* · *Textbaustein „Kontakt" – …* · *Gruppe „F gruppe" – …* | task 3 steps 1, 8 |
| the flash after uploading an image | *Datei hochgeladen – bitte noch eine Bildbeschreibung eintragen* | task 3 step 2 |
| the refusal that stops an administrator switching off their second factor | *Für Administratoren ist die Bestätigung in zwei Schritten Pflicht. Wenn das Gerät verloren ist, hilft ein Wiederherstellungscode oder „holzcloud user 2fa disable" auf dem Server.* | task 2 step 3 |

**One of these is v1.6's own.** `git log -S 'Textbaustein „' -- internal/admin/field.go` returns `48e5b1d feat(08-03): der vierte Modus des Feldbildschirms`, 2026-09-06 — Phase 8. Criterion 6 says *"0 offen, 0 verwaist across everything v1.6 added"*, and the screen Phase 8 added prints German to a reader whose admin is English. The other four are pre-existing (`0e6d7af`), but they were found on the same screens, in the same session, by the same method.

**Why no gate in this repository can see them.** Each sentence is written in a shape the collector cannot read:

```
internal/admin/field.go:311      title = "Felder – " + websiteName            string concatenation
internal/admin/field.go:317-329  title = "Baustein „" + …                     string concatenation
internal/auth/middleware.go:109  http.Error(w, "Diese Website …", 403)         never reaches a template
internal/admin/starter.go:181    return fmt.Sprintf("Website angelegt – …")    returned from a helper
internal/admin/media.go:176-183  message := "Datei hochgeladen"; message += …  assembled in pieces
internal/admin/twofactor.go:283  "…Pflicht. " + "…oder " + "„holzcloud…"        three fragments
```

`tools/i18n`'s own package comment — added in commit `24fa2ee`, *while this pass was running* — already describes the mechanism exactly:

> "N Zeichenketten im Quelltext" counts what this tool can see, which is not the same as what a person wrote. A sentence that has always been in the code but sat in a shape the collector cannot read — assembled with `fmt.Sprintf`, returned from a helper, surfaced through `err.Error()` — is absent from that number.

The comment stops one step short of the consequence, and the consequence is what a browser shows: those sentences are not merely *uncounted*, they are **printed in German to a reader whose admin is English**. This is the same class as Phase 11's finding (`textGallery`, marked, collected, translated in four catalogues, and still German on screen) — but worse in blast radius: that one was an `aria-label`, these include an `<h1>`, a `<title>` and three flashes.

**How large is the class.** Reported as a **floor**, not a count, and with what escapes it named first. A gate over `internal/admin`, `internal/auth`, `internal/web` and `internal/public` for German-marked literals (`äöüÄÖÜß„"`) absent from `en.json`, excluding comments, `slog.*` and error wrapping, reads:

```
narrow (only lines carrying title= / http.Error / flash)      8
wide   (every such literal in those four packages)           59
```

**Three shapes I know slip past the narrow one, because I saw all three on the screen and none of them is in its 8:**

1. `"Felder – "` — **ASCII only**. No umlaut, no sharp s, no German quotation marks. A German sentence written in plain ASCII is invisible to this gate by construction.
2. `starterContentSummary()` — the literal is `return`ed from a helper, so its line matches neither `title=` nor `flash`.
3. `media.go:176` — `message :=` followed by `+=`, matching neither.

The wide 59 includes rows that are not user-facing at all (starter *content* written into the database, CSV sample text, WordPress import diagnostics), so it is not a gate either. **Neither number is the answer**; the five sentences above are, and they were found by looking.

**Not fixed here.** The class is pre-existing and milestone-wide, this plan has no production-code task, and at least one remedy (the `fmt.Sprintf`-assembled sentence) changes how a string is built rather than what it says — which is a shape decision, not phase-close work in a tree another agent is committing to. **Recorded as `.planning/WINDOWS.md` entries 12-16**, so it blocks `/gsd-ship` rather than scrolling out of context.

## 2. Two more gates measuring something other than their name

This phase has now hit **nine**. The eighth and ninth are both in gates this plan ran.

**Eighth — the plan's own `isTrustedProxy` exclusion, confirmed against the tree.**

```
grep -rn isTrustedProxy . --include='*.go' --include='*.md' | grep -v '^\./\.planning/' | wc -l   ->  32
grep -rn isTrustedProxy . --include='*.go' --include='*.md' | grep -v '^\(\./\)\?\.planning/' | wc -l   ->   0
```

`grep` here is a shell function over `ugrep` (`type grep` → *"a shell function from /Users/holz/.claude/shell-snapshots/…"*) and prints paths **without** a `./` prefix, so the `^\./` anchor never fires. `WINDOWS.md` entry 11 already recorded this from plan 10-08 and gives the corrected form; running it confirmed both the failure and the fix. **The property itself holds: 0 outside `.planning/`.**

**Ninth — my own comment filter, which read one too high.** Measuring `MustHaveSecondFactor`, I stripped comments with `grep -v ':[0-9]*: *//'` and got 7 instead of the 6 the plan expects. The line it missed is `admin/twofactor.go:383`, which is indented with a **tab**, and ` *` matches spaces only. The honest answer came from reading all eight lines by hand: 2 comments, 1 definition, 5 call sites. Worth recording precisely because it is the same shape one step inward — I wrote the gate, in the plan that exists to check gates, and it still read the wrong number.

**A tenth, reported as a narrowing rather than a failure.** Task 2's verify is `go test ./internal/auth/ ./internal/admin/ -run 'SecondFactor|Login|Logout|PasswordPath'`. Checked before being believed: it matches **69** test entries and reports **0 FAIL** — not the "matches nothing" shape 10-09 found. But it excludes the entire `TestForwardAuth*` family and `TestNoPasswordSignsInAProvisionedAccount`. For *its* claim ("nothing about the password path changed") that exclusion is correct; named here so nobody later reads its 0 as covering the sign-in machinery this phase built. `go test ./...` covers those and is green.

## 3. A rights change is logged, and the screen does not say what changed

`/admin/protokoll` shows five columns — Zeitpunkt, Wer, Aktion, Betrifft, Website — and a role change appears as:

```
08.09.2026 06:42 | redakteurin@example.com | user.update | user #2 | —
```

The `metadata` column holds `{"field":"role","from":"editor","to":"admin","via":"sso"}` and the screen never prints it, for any action. So criterion 1's *"every change of rights is written to the activity log"* is true at the storage layer and an administrator reading the screen cannot tell a promotion from a demotion, or either from any other `user.update`. Pre-existing (the template has always had five columns); named because this phase is the first to write rights changes that nobody triggered by hand.

## 4. The register drifted behind the code again — SSO-03 and SSO-04 read Pending

Checked while closing, because the milestone close-out gate is graded from these documents and the knowledge base already carries *"the ledger drifts behind the code, and the gate reads the ledger"* with four occurrences.

```
before:  SSO-03 | Phase 10 | Pending          - [ ] **SSO-03**: the header strip …
         SSO-04 | Phase 10 | Pending          - [ ] **SSO-04**: the shared secret …
         the other nine SSO requirements:  Complete
```

Both are delivered by plan **10-02**, whose summary exists and whose `provides` names them in as many words (*"An unconditional strip of every inbound identity header … driven by a scan of the header map's own keys"*, and the constant-time secret). SSO-04 is additionally declared by 10-01, so the shared-ID gate held it until the second declaring plan finished — and nothing marked it afterwards. All ten plans of the phase have summaries, so nothing was in flight.

**And this pass drove both of them**, which is why marking them is a measurement and not bookkeeping:

```
SSO-03  X_authentik_username: redakteurin  from 192.168.0.63   ->  303 …/admin/login
        (the underscore spelling Go canonicalises differently, stripped like every other)
SSO-04  the complete identity from 127.0.0.1 with the real secret   ->  200
        the same from 127.0.0.1 with a WRONG secret                 ->  303 …/admin/login
        the same from 127.0.0.1 with NO secret                      ->  303 …/admin/login
```

Marked complete. `grep -cE "\| (SSO|QUAL)-[0-9]+ \|" … | grep -c Pending` now reads **0** for the whole phase. Fifth occurrence of this pattern; recorded so the count in the knowledge base stays honest.

## 5. Observations, offered as such

- **No `@media print` block exists** while the recovery-codes screen says "Print them out" and "Print them or write them down before going on".
- **A short CSV row produces no message.** `internal/csv/csv.go`'s comment promises it becomes "a reported row carrying its own row number instead of an error carrying a line number", and it does become a row rather than an error — but there was nothing else wrong with it, so nothing was reported, and a page was created with empty trailing columns.
- **The rights sync is per sign-in, not per request**, and the source says why (`forwardauth.go:126-136`). Named under step 6 because "reload" does not demonstrate it and an operator revoking access should know when it takes effect.

---

# Not driven

Recorded as *not driven* rather than as passed, because this project has been bitten by both mistakes.

- **A snippet field's value on a public page (task 3 step 10, first half).** No shipped theme prints `.Site.Bausteinliste` or `.Site.Bausteinfelder` — `grep -rn "Bausteinliste\|Bausteinfelder" cmd/holzcloud/templates/public/` finds nothing, though `loader.go:392` says the list "is what a shipped theme can use". So there is no public page on which `<script>alert(1)</script>` in a snippet's `langtext` field can be seen at all, sanitised or not. The **snippet body** carries the same goldmark→bluemonday pipeline and was driven in full (script removed, `onerror` stripped, `javascript:` href stripped). Writing and uploading a template of my own to reach the field values would have proved a theme I wrote, not one that ships.
- **A real Authentik outpost.** Everything the stand-in does not cover, listed at the top of this document, and the sign-out cookie in particular. Step 9's `-anon` counterfactual is the closest a stand-in gets.
- **CVE-2026-30851 and the `copy_headers` behaviour.** They live in Caddy. `deploy/DEPLOY.md` carries the operator's two verifications, and plan 10-08 validated the shipped Caddyfile by adapting it with a real Caddy 2.11.4.
- **`0.0.0.0` on linux/amd64.** The dual-stack listener was measured on darwin and *explained* from `net/ipsock_posix.go`'s wildcard branch, which is platform-independent. It was not run on Linux, and a kernel with `net.ipv6.bindv6only=1` would behave differently.
- **The `verweis` field's rendered link and the `bild` field's rendered image on a public page.** Both round-tripped in the editor (step 2) and the default theme has branches for both, but the values happened to sit on the home page, which `home.html` renders without a field list; the public checks were done on `/impressum`, where only four fields were set.

---

# Deviations from Plan

## Auto-fixed

None. This plan carries no production-code task and none was needed: every criterion it drove held.

## Departures from the plan's own wording, each deliberate

**1. [Rule 3 — Blocker] The precondition names a review-fix round that does not exist.**
Reported at the top rather than halted on, as instructed. The condition the precondition protects — no user-visible change after the pass — was checked directly and is on the record with the four commits named.

**2. Task 2's steps were driven in the order 1, 2, 3, 7, 5, 6, 4, 8, 9, 10.**
The administrator was already enrolled from task 1, so step 3 is the `/admin/konto` variant the plan explicitly allows, and step 4's full enrolment is walked after step 6's `2fa disable`, where it simultaneously proves step 6's promise. Every step was driven; only their order moved.

**3. Task 3 step 10's first half could not be reached.** Recorded under *Not driven* with the reason and with the reachable half driven instead.

**4. The public-page checks were moved from the home page to `/impressum`.** `home.html` prints no field list; `page.html` does. Not a defect — a theme decides what it prints — but it is why the field evidence is on the second page.

## Found and deliberately not fixed

**5. [Rule 4 — Architectural] Five German sentences on an English admin.** See Findings 1. `WINDOWS.md` entries 12-16.

---

**Total deviations:** 0 auto-fixed. 4 deliberate departures in method, each named. 1 finding class recorded under Rule 4.
**Impact on plan:** none on scope. The plan's central claim — that a browser pass finds what the suite cannot — held for the fourth phase running.

---

# Issues Encountered

**1. Four harness errors of my own, each recorded beside the finding it nearly became.**

- *The wrong `<details>`.* The sign-out form is inside one of **two** `details.switcher` elements; my first selector opened the other one and the click landed in empty space. The sign-out looked broken. Caught because the URL did not change.
- *Coordinates.* I "corrected" `DOM.getBoxModel` by subtracting `Page.getLayoutMetrics`' `pageY`, and every click then landed one scroll-height above its target. Measured, not reasoned: on this Chrome the box model already returns **viewport**-relative coordinates and `Input.dispatchMouseEvent` wants exactly those. The subtraction was removed and the reason written into `cdp.py`.
- *Newlines in key events.* `Input.dispatchKeyEvent` with `text` carries no newline, so `'rot\ngruen\nblau'` typed into the options textarea became **one** option named `rotgruenblau`. It looked like the choice field had lost its options. Repaired by setting the value and firing `input`/`change`, then re-driven.
- *The first form on any admin page is `/admin/logout`.* A `form` selector without an anchor submits the sign-out. It cost one sign-in.

None of these is an application defect and none is written up as one — but each was one wrong conclusion away from being reported as a finding, which is the argument for checking the consequence rather than the API. Phase 11's summary records the same lesson from `Network.setBlockedURLs`.

**2. The tree moved under the pass.** Four commits landed while it ran. Analysed rather than ignored: see *The precondition*. No `git clean`, no `git stash`, no `git checkout -- .`, no blanket reset and no `git worktree` was used at any point, and `git status --porcelain` was empty at both ends.

---

# Final gates, on the finished tree

```
HEAD before        34df8f897005c2f67b5a388d6481f0896ff822f5
git status         (empty)
go build ./...     exit 0
go vet ./...       exit 0, silent
gofmt -l .         silent
go test ./...      0 failures
go run ./tools/i18n   1321 strings; en/es/fr/it each "1321 übersetzt, 0 offen, 0 verwaist";
                      de-CH 73 Abweichungen, 0 ohne Gegenstück;
                      fr-CH 4, it-CH 9, both read-only
HEAD after         34df8f897005c2f67b5a388d6481f0896ff822f5
git status         (empty)
```

Environment: Go 1.26.6 darwin/arm64, Chrome 152.0.7977.82, binary `sha256 047997fb…` built `CGO_ENABLED` default from `24fa2ee`.

**Teardown.** Both scratch data directories removed (`data1` gone, `data3` gone), no listener on 8099 or 9090, the stand-in binary and its source never committed, the scratch path outside the repository, and no real data directory used at any point.

---

# The milestone close-out gate: does it hold?

Criterion 6 asks two things.

**"`go run ./tools/i18n` reports `0 offen, 0 verwaist` across everything v1.6 added" — MET as written, and the reader must be told what that sentence does not mean.** All four catalogues are green on 1321 strings. And a screen Phase 8 added prints its `<title>` and `<h1>` in German to an English admin, because the sentence was never collectable. The gate is honest about what it measures — `tools/i18n`'s own package comment says so in as many words — and criterion 6 was written expecting that number to mean "nothing v1.6 added is untranslated". It does not mean that. **The gate passes; the intent behind it is not fully met**, and the shortfall is recorded in `WINDOWS.md` rather than argued away.

**"every field kind from Phase 7, the snippet fields from Phase 8, both import paths from Phase 9 and the sign-on path here have each been driven once through the running application in a browser" — MET.** Sixteen kinds defined, filled, saved, read back and rendered; the snippet fields in the fourth mode, filled, read back and proved absent everywhere they should be; both import paths and both collision answers, with the dry run agreeing with the write on every one; and the sign-on path in both configurations. Phase 11's own pass is recorded and cited.

**The other four criteria of Phase 10 — 1, 2, 3 and 5 — are met and were seen**, each with the step numbers above. Criterion 4's second half (signing out signs the person out at Authentik too) is met on the CMS's side and rests on the operator's outpost for the rest, which is stated in `DEPLOY.md` and stated here.

So: the milestone's close-out gate **holds**, with one qualification that is written down rather than smoothed over — the translation gate is green on a tree where five German sentences reach a non-German admin, one of them added by this milestone. That is a defect to fix, not a reason to withhold the milestone; but a reader of this document should not be able to miss it, which is why it is the first finding and the last paragraph.

---

# Next Phase Readiness

Phase 10 is complete: ten plans, ten summaries. With Phase 11 already closed, **v1.6 is complete** — 48 requirements across six phases.

Carried forward for whoever picks up next:

- **`.planning/WINDOWS.md` entries 12-16** — the five untranslated sentences. Entry 16 (`twofactor.go:283`) sits on a Phase 10 call site; entry 12 (`field.go:311`) covers a Phase 8 screen. A Phase 12 "Umbenennung" pass is the natural home for the class, together with entry 8 (Phase 11's `textGallery`), which is the same failure by a different route.
- **The generalisation the knowledge base should carry**: *a translation gate counts what a collector can read, and a sentence assembled at runtime is invisible to it in both directions — it is neither counted nor translated, and only a browser shows the difference.* Nine gates in this phase measured something other than their name; this is the tenth and the largest, because it is the gate a whole milestone criterion rests on.
- The `SupportedLocales` gap Phase 11 named (four catalogues, two selectable website languages) is unchanged.

## Self-Check: PASSED

- `.planning/phases/10-authentik/10-10-SUMMARY.md` present on disk.
- `.planning/WINDOWS.md` carries the five new entries (`node gsd-tools.cjs windows append` returned `ok: true` five times).
- Both scratch data directories removed; no listener on 8099 or 9090; nothing of the stand-in committed.
- `git status --porcelain` empty before the pass, before the gates and after them; HEAD `34df8f8` unchanged across the gate run.
- `go build ./...` exit 0, `go vet ./...` silent, `gofmt -l .` silent, `go test ./...` 0 failures.
- `go run ./tools/i18n` → four lines reading `0 offen, 0 verwaist`.
- `REQUIREMENTS.md`: all 13 Phase 10 requirements read Complete; `grep` for Pending among SSO/QUAL rows returns 0.
- 67 screenshots captured under the session scratch path (`t1-` 16, `t2-` 16, `t3-` 35), one per numbered step, each named in this document by what it showed.
