---
phase: 10-authentik
plan: 08
subsystem: infra
tags: [documentation, deploy, caddy, forward-auth, authentik, cve-2026-30851, cve-2026-52845, changelog, listen-address]

requires:
  - phase: 10-authentik
    provides: "plan 10-01's seven HOLZCLOUD_SSO_ settings with the defaults the code actually carries, its four start-up refusals, and HOLZCLOUD_LISTEN — whose loopback default is the one thing in this phase that changes behaviour for an installation that never switches single sign-on on"
  - phase: 10-authentik
    provides: "plan 10-02's ProxySecretHeader, the four-header contract and the unconditional strip — the Caddyfile's copy_headers list and the strip's normalised prefix are the same contract seen from two sides"
  - phase: 10-authentik
    provides: "plan 10-03's ASCII address rule, plan 10-04's provisioning refusal, plan 10-05's group synchronisation and its no-website refusal, plan 10-06's MustHaveSecondFactor(role, viaSSO) and its two admin screens, plan 10-07's relative sign-out path"
provides:
  - "deploy/Caddyfile.example: a commented (holzcloud-sso) snippet with the outpost route, forward_auth, and one explicit request_header delete per copied header in BOTH spellings — validated by adapting it with a real Caddy 2.11.4"
  - "deploy/DEPLOY.md: the single sign-on section — seven settings, the 2.11.2 floor with the affected range, the two operator-owned verifications, the acceptance test with the sentence that a refused connection is not a pass, the ASCII address rule, and SSO-07's DEPLOY half"
  - "HOLZCLOUD_LISTEN documented in three places, including the docker run command that would otherwise produce a container answering nobody"
  - "docs/security.md: the four layers for a reader who is not deploying, and the sentence that an identity provider's session satisfies the second step"
  - "docs/configuration.md: nine variables, pointing at DEPLOY.md for the reasoning rather than repeating it"
  - "CHANGELOG.md: the German entry that names the loopback default before anything else"
  - "The measured correction to the isTrustedProxy gate, and the measured fact that Caddy's own fix deletes only the canonical spelling"
affects: [10-09-translation, 10-10-browser-pass, 12-translation-sweep]

actuals:
  # chars/4 over the realized diff, the scale 10-07 fixed for this phase.
  # git diff 0c20f47..5b1bba7 -- deploy/ docs/ CHANGELOG.md is 30,575 characters.
  # NOT comparable with waves 3-5, which recorded approximately chars/1.
  tokens: 7644
  raw_diff_chars: 30575
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "A documentation claim about another program is measured against that program when it can be: caddy adapt on the shipped example proves what forward_auth generates, and proves it for the version an operator can actually install"
    - "A refactor of a shipped example is proved neutral by adapting the file before and after and diffing the JSON, not by reading it"
    - "Every claim in an operator-facing document is either checked against this repository or explicitly labelled as the operator's own verification; the SUMMARY says which is which"
    - "A Caddyfile whose ordering matters is written inside one route, because a route runs its directives in written order and nothing else in a Caddyfile guarantees that"

key-files:
  created: []
  modified:
    - deploy/Caddyfile.example
    - deploy/DEPLOY.md
    - docs/security.md
    - docs/configuration.md
    - CHANGELOG.md

key-decisions:
  - "The (holzcloud-sso) snippet is defined ABOVE the site blocks, not at the foot of the file as the plan's layout implied. Measured: Caddy resolves import at parse time in file order, so a snippet defined after its use adapts with 'File to import not found: holzcloud-sso'. The first draft of this file shipped that error and caddy adapt found it."
  - "The response-header block was extracted into a (holzcloud-headers) snippet so the plain site and the single sign-on site share one definition. A duplicated security-header block in an example file is a block that drifts. Proved neutral: caddy adapt produces byte-identical JSON before and after."
  - "Everything after the outpost handle sits inside one route including the terminal reverse_proxy, so no claim in this file depends on remembering Caddy's default directive order. The shared secret is set on the localhost hop and AFTER forward_auth, so it goes to the CMS and not to the identity provider — verified in the adapted JSON, where the outpost upstream carries only X-Forwarded-Method and X-Forwarded-Uri."
  - "docs/configuration.md gained nine variables, not seven: HOLZCLOUD_LISTEN and HOLZCLOUD_TRUSTED_PROXIES were both absent from it although both existed before this phase, and both decide whether a forwarded identity is read at all. The plan says nine and the two tables say seven; nine is the reading that makes the second description not a subset of the first."
  - "The DEPLOY.md sentence about an editor whose groups map to no website carries plan 10-05's caveat explicitly: the refusal only applies when HOLZCLOUD_SSO_WEBSITE_GROUPS is set. Without the caveat the document would describe a lockout that does not happen."
  - "The claim 'the same sentence is on the account screen and the user list' was weakened to 'the same dependency', because the two templates carry two different German sentences. A document that says 'the same sentence' about two different sentences is wrong in a way nothing would catch."

patterns-established:
  - "A gate must measure what its name claims — FIFTH instance in this phase, and the first one in a gate this plan was asked to run rather than one it wrote. The plan's isTrustedProxy gate excludes '^\\./\\.planning/'; grep on this machine emits paths without the './' prefix, so the exclusion never fires and the gate reads 14 where it wants 0. The corrected exclusion reads 0. Recorded in .planning/WINDOWS.md."
  - "A documentation plan can install the program it documents. Caddy 2.11.4 was installed specifically to adapt the shipped example, and it turned three unverifiable assertions into measurements — including one that changed what the file says."
  - "Cite functions, never line numbers — carried from waves 1-7. Nothing this plan wrote cites a line number in a Go file; DEPLOY.md cites HOLZCLOUD_ names and docs/security.md cites no symbol at all."

requirements-completed: [SSO-07, SSO-11]

coverage:
  - id: D1
    description: "The shipped Caddy example deletes the client's own identity headers explicitly, one request_header line per copied header, in both the hyphenated and the underscored spelling, placed before forward_auth"
    requirement: "SSO-11"
    verification:
      - kind: other
        ref: "grep -c 'request_header -X-authentik' deploy/Caddyfile.example == 4 and grep -c 'request_header -X_authentik' == 4"
        status: pass
      - kind: integration
        ref: "caddy adapt on the uncommented snippet (Caddy 2.11.4): the eight deletes appear FIRST and in written order — X-authentik-username, X_authentik_username, X-authentik-email, X_authentik_email, X-authentik-name, X_authentik_name, X-authentik-groups, X_authentik_groups — before Caddy's own four"
        status: pass
      - kind: integration
        ref: "caddy adapt exit 0 on the file as shipped and on the file with the snippet uncommented; caddy fmt produces no diff"
        status: pass
    human_judgment: false
  - id: D2
    description: "The underscored spelling is load-bearing even on a Caddy that carries the CVE-2026-30851 fix, and the shipped file says so"
    requirement: "SSO-11"
    verification:
      - kind: integration
        ref: "caddy adapt on Caddy 2.11.4 emits delete entries for X-Authentik-Username, -Email, -Name, -Groups only — the canonical hyphenated names. No underscore delete is generated by the fix."
        status: pass
      - kind: other
        ref: "grep -c 'CVE-2026-30851' deploy/Caddyfile.example == 2; the companion advisory GHSA-f59h-q822-g45g / CVE-2026-52845 is named in the same comment"
        status: pass
    human_judgment: false
  - id: D3
    description: "DEPLOY.md names a minimum Caddy version of 2.11.2 and says what is wrong with 2.10.0 through 2.11.1, so an operator on the stable apt package knows they are affected"
    requirement: "SSO-11"
    verification:
      - kind: other
        ref: "grep -c '2\\.11\\.2' deploy/DEPLOY.md == 4 (Caddy section and single sign-on section); grep -c 'CVE-2026-30851' deploy/DEPLOY.md == 1, with the affected range and 'latent since November 2024' beside it"
        status: pass
    human_judgment: false
  - id: D4
    description: "The two facts that are genuinely the operator's are written as verifications they perform once against their own instance, with a runnable check for each"
    requirement: "SSO-11"
    verification:
      - kind: other
        ref: "deploy/DEPLOY.md, '### Two things you verify once, against your own instance' — (1) that their Authentik emits X-authentik-username, with the Subject-mode warning; (2) `caddy version` plus `caddy adapt --config /etc/caddy/Caddyfile | grep -o '\"delete\":\\[[^]]*\\]'`"
        status: pass
      - kind: integration
        ref: "the caddy adapt | grep command in item 2 was run against the shipped example on Caddy 2.11.4 and printed the expected entries — the check an operator is told to run is a check that works"
        status: pass
    human_judgment: false
  - id: D5
    description: "Criterion 2's one command is in DEPLOY.md as the operator's own acceptance test, with the sentence that a connection refused is NOT a pass and instructions for making the port answer"
    requirement: "SSO-11"
    verification:
      - kind: other
        ref: "grep -ci 'connection refused' deploy/DEPLOY.md == 1, in '### The acceptance test, and the answer that is not a pass'"
        status: pass
      - kind: other
        ref: "the expected answer '303 to …/admin/login' checked against internal/auth/middleware.go:38 — http.Redirect(w, r, \"/admin/login\", http.StatusSeeOther)"
        status: pass
      - kind: e2e
        ref: "the same command was run live over IPv4 and IPv6 from an untrusted address in plan 10-02 (coverage D3) against the built binary; this plan documents that test rather than re-running it"
        status: pass
    human_judgment: false
  - id: D6
    description: "SSO-07's DEPLOY half: this installation's second factor is whatever the operator's Authentik enforces, stated unmissably, completing the requirement with plan 10-06's two admin screens"
    requirement: "SSO-07"
    verification:
      - kind: other
        ref: "deploy/DEPLOY.md, '### The second factor is whatever your Authentik enforces' — a section heading, not a sentence in a paragraph"
        status: pass
      - kind: other
        ref: "checked against internal/auth/twofactor.go:60 — MustHaveSecondFactor(role, viaSSO) returns role == \"admin\" && !viaSSO"
        status: pass
      - kind: other
        ref: "the admin half verified present: cmd/holzcloud/templates/admin/account.html:36 ({{if .ViaSSO}}) and cmd/holzcloud/templates/admin/user_list.html:8 ({{if .SSOEnabled}})"
        status: pass
    human_judgment: false
  - id: D7
    description: "HOLZCLOUD_LISTEN is in the environment table with its loopback default, and the container section and the shipped docker run command both set 0.0.0.0"
    verification:
      - kind: other
        ref: "grep -c 'HOLZCLOUD_LISTEN' deploy/DEPLOY.md == 4; grep -c 'HOLZCLOUD_LISTEN=0.0.0.0' == 3 (the container bullet, the docker run line, the acceptance test's temporary override)"
        status: pass
      - kind: other
        ref: "default checked against internal/config/config.go:175 — const defaultListen = \"127.0.0.1\""
        status: pass
    human_judgment: false
  - id: D8
    description: "The e-mail rule is documented: an address carrying a character above ASCII is refused at sign-in because COLLATE NOCASE folds ASCII only"
    verification:
      - kind: other
        ref: "deploy/DEPLOY.md, '### E-mail addresses above plain ASCII are refused'"
        status: pass
      - kind: other
        ref: "checked against internal/admin/forwardauth.go:160 (if !isASCII(email) -> refuseSSO with ssoRefuseNonASCII) and :635 (isASCII: every byte below 0x80)"
        status: pass
    human_judgment: false
  - id: D9
    description: "The stale reference to internal/admin/login.go:isTrustedProxy is corrected everywhere outside .planning/"
    verification:
      - kind: other
        ref: "grep -rn isTrustedProxy deploy/ docs/ CHANGELOG.md README.md | wc -l == 0; repository-wide outside .planning/ == 0. The replacement names HOLZCLOUD_TRUSTED_PROXIES and internal/web/clientip.go, both of which exist (config.go:259, clientip.go:50)"
        status: pass
    human_judgment: false
  - id: D10
    description: "Nothing in these documents tells a browser to fetch anything from a third party at runtime"
    verification:
      - kind: other
        ref: "grep -Ec 'https?://[a-z0-9.-]+' deploy/Caddyfile.example == 2, unchanged — the two Caddy-installation lines that were already there. The outpost is addressed as a host:port without a scheme and served from /outpost.goauthentik.io/* on the application's own domain."
        status: pass
    human_judgment: false
  - id: D11
    description: "The changelog entry is German whole sentences in the file's own voice, names the loopback default first among the things that change for an installation that does not use single sign-on, and names the second-factor consequence"
    verification:
      - kind: other
        ref: "sed -n '/## Unveröffentlicht/,/^## /p' CHANGELOG.md | grep -ci HOLZCLOUD_LISTEN == 2; grep -c '^- ' in the same range == 0 (no bullet fragments)"
        status: pass
    human_judgment: false
  - id: D12
    description: "This is a documentation plan: no Go file changed, the suite is as green as it was found, and the string count did not move"
    requirement: "QUAL-01"
    verification:
      - kind: other
        ref: "git diff --name-only 0c20f47..HEAD -- '*.go' | wc -l == 0"
        status: pass
      - kind: other
        ref: "go build ./... exit 0; go vet ./... exit 0; gofmt -l . empty; go test ./... exit 0, 44 packages ok, 0 FAIL"
        status: pass
      - kind: other
        ref: "go run ./tools/i18n: 1321 Zeichenketten im Quelltext, 2 offen — identical to the measurement taken immediately before the first commit of this plan"
        status: pass
    human_judgment: false

duration: 41min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 08: Deployment documentation Summary

**The half of this phase that runs on somebody else's machine: a Caddy example that deletes the visitor's own identity headers in both spellings and adapts cleanly on a real Caddy, a deployment section that names 2.11.2 and says what is wrong below it, and one acceptance test with the sentence that makes it a test rather than a ritual.**

## Performance

- **Duration:** 41 min
- **Started:** 2026-09-08T05:28:00Z
- **Completed:** 2026-09-08T06:09:00Z
- **Tasks:** 3 of 3
- **Files modified:** 5

## Accomplishments

- **`deploy/Caddyfile.example`** carries a commented `(holzcloud-sso)` snippet: the outpost route on the application's own domain, eight explicit `request_header` deletes, `forward_auth` copying only the four headers the CMS reads, and the shared secret on the hop to the CMS. **The file was validated by adapting it with a real Caddy 2.11.4**, both as shipped and with the snippet uncommented.
- **`deploy/DEPLOY.md`** gained `## Single sign-on (Authentik forward auth)` — 160 lines covering the seven settings with the defaults the code carries, the `HOLZCLOUD_SSO_PROVISION` / `HOLZCLOUD_SSO_DEFAULT_WEBSITE` refusal and what it protects, the 2.11.2 floor, the two operator verifications, the acceptance test, the ASCII address rule and the second-factor dependency — plus `HOLZCLOUD_LISTEN` in the variable table, in the container section and in the shipped `docker run` command.
- **`docs/security.md`** describes the four layers to a reader who is not deploying, and says plainly that this is the first place in the program that believes a claim about identity rather than verifying one.
- **`docs/configuration.md`** lists nine variables it did not have and points at `DEPLOY.md` for the reasoning.
- **`CHANGELOG.md`** carries the German entry, with the loopback default named before anything else and the container case spelled out.
- **Three assertions became measurements** because Caddy was installed to check them: what `forward_auth` generates, that the fix's delete covers the canonical spelling only, and that the header-snippet extraction is behaviour-neutral.

## Task Commits

1. **Task 1: the Caddy example** — `e6ad663` (docs)
2. **Task 2: DEPLOY.md, security.md, configuration.md** — `e2e244f` (docs)
3. **Task 3: the changelog entry** — `5b1bba7` (docs)

## Files Created/Modified

- `deploy/Caddyfile.example` — the `(holzcloud-sso)` snippet, the extracted `(holzcloud-headers)` snippet, the minimum-version note in the header comment, and the corrected trusted-proxy paragraph
- `deploy/DEPLOY.md` — the single sign-on section, the `HOLZCLOUD_LISTEN` row, the container bullet and `docker run` line, the Caddy-section version note
- `docs/security.md` — `## Signing in through an identity provider`, plus one paragraph inside `## Two-factor authentication` and a table-of-contents entry
- `docs/configuration.md` — nine variables and a short `## Single sign-on` pointer
- `CHANGELOG.md` — `### Hinzugefügt` under `## Unveröffentlicht`, four paragraphs

## What is checked against this repository, and what is the operator's

This is the acceptance question for a plan whose output no test can grade, so it is answered explicitly.

### Checked against this repository (a reader may act on these)

| Claim in DEPLOY.md / docs | Checked against |
|---|---|
| `HOLZCLOUD_SSO_ENABLED` default `false` | `internal/config/config.go:291` |
| SSO on without a secret refuses to start | `config.go:307` |
| `HOLZCLOUD_SSO_ADMIN_GROUP` has no default | `config.go:293`, and the field comment at `:123` |
| `HOLZCLOUD_SSO_WEBSITE_GROUPS` is `group=websiteID` pairs | `config.go:297`, `parseWebsiteGroups` |
| Provisioning without a default website refuses to start | `config.go:322`; the database check is `checkDefaultWebsite` in `cmd/holzcloud` |
| `HOLZCLOUD_SSO_SIGN_OUT_PATH` default and one-leading-slash validation | `config.go:178` (`defaultSignOutPath`), `:328` (`isLocalPath`) |
| `HOLZCLOUD_LISTEN` default `127.0.0.1` | `config.go:175` (`defaultListen`) |
| The secret header is `X-Holzcloud-Proxy-Secret`, compared in constant time | `internal/web/forwardauth.go` (`ProxySecretHeader`, `subtle.ConstantTimeCompare`) |
| Identity headers are stripped on every path before any handler | `internal/web/forwardauth.go`, `stripIdentityHeaders`, normalising `_` to `-` |
| The identity is the username and not `-uid` | the `Identity` doc comment in the same file |
| An address above plain ASCII is refused | `internal/admin/forwardauth.go:160`, `:635` |
| Groups map to no configured website ⇒ the sign-in is refused, but only when the mapping is set | `internal/admin/forwardauth.go:431` (`errSSONoWebsiteGroup`), plan 10-05's recorded unconfigured-mapping branch |
| An Authentik session satisfies the second factor | `internal/auth/twofactor.go:60` |
| The dependency is shown on two admin screens | `account.html:36`, `user_list.html:8` |
| The acceptance test's expected `303 …/admin/login` | `internal/auth/middleware.go:38` |
| `HOLZCLOUD_TRUSTED_PROXIES` is what a proxy on another host needs | `config.go:259`, `internal/web/clientip.go:50` |
| The Caddy example parses, formats and adapts | `caddy adapt` / `caddy fmt` on Caddy 2.11.4, exit 0 |
| Caddy's own fix deletes the canonical spelling only | measured: `caddy adapt` on 2.11.4 emits `"delete"` for `X-Authentik-Username/-Email/-Name/-Groups` and for no underscore name |
| The shared secret goes to the CMS and not to the outpost | measured in the adapted JSON: the outpost upstream carries only `X-Forwarded-Method` and `X-Forwarded-Uri` |
| Extracting `(holzcloud-headers)` changed nothing | measured: `caddy adapt` output is byte-identical before and after |

### Taken from this phase's research, not measured here (labelled as such)

- **That CVE-2026-30851 affects Caddy 2.10.0 through 2.11.1, was fixed in 2.11.2, and was latent since November 2024.** The *fixed* half is measured — 2.11.4 emits the delete. The *affected range* and the introduction date come from the advisory as recorded in `ROADMAP.md` and `10-CONTEXT.md`; no vulnerable Caddy was installed to reproduce the bypass. `DEPLOY.md` states the range as fact and names the advisory so a reader can check it, which is the right treatment for a published CVE.
- **That Authentik joins group names with `|` and emits the `-uid`, `-jwt`, `-entitlements` and `-meta-*` families.** Verified by plan 10-02 from Authentik's own source at two release tags, not re-verified here.

### Written as the operator's own verification (this project makes no claim)

1. **That their Authentik emits `X-authentik-username` with the value they expect**, and that the provider's Subject mode is not changed after users are mapped. Framed as a check they run, with two ways to read the header off their own instance.
2. **That their Caddy is 2.11.2 or newer and that the configuration it generates really carries a delete per copied header.** Two commands, `caddy version` and `caddy adapt … | grep -o '"delete":[…]'`. The second command was itself run here against the shipped example, so the check an operator is told to run is a check that works.

Beside those, the **acceptance test** is deliberately of the same kind: it tells the operator the truth about their own installation without this project having to know it, and it carries the sentence that a refused connection is not a pass.

## Decisions Made

1. **The `(holzcloud-sso)` snippet is defined above the site blocks.** Caddy resolves `import` at parse time in file order; a snippet defined at the foot of the file, as the plan's layout implied, adapts with `Error: File to import not found: holzcloud-sso`. The first draft shipped exactly that, and `caddy adapt` found it in one run. **This is the single most valuable thing installing Caddy bought** — the file would have been reviewed, gated and committed with an instruction that does not work.
2. **The response-header block was extracted into `(holzcloud-headers)`.** Two sites now share one definition instead of two that drift. Proved neutral by adapting the file before and after and diffing the JSON: byte-identical.
3. **Everything after the outpost `handle` is inside one `route`, including the terminal `reverse_proxy`.** A `route` runs its directives in written order; nothing else in a Caddyfile guarantees that, and this file's whole point is that the deletes stand before the question is asked. The shared secret sits after `forward_auth` so it reaches the CMS and not the identity provider.
4. **Nine variables in `docs/configuration.md`, not seven.** `HOLZCLOUD_LISTEN` and `HOLZCLOUD_TRUSTED_PROXIES` were both missing from that file although both predate this phase, and both decide whether a forwarded identity is read at all.
5. **The plan's own sentence about an editor with no matching website group was qualified.** Plan 10-05 recorded that with `HOLZCLOUD_SSO_WEBSITE_GROUPS` unset the website half does not run; without the caveat `DEPLOY.md` would describe a lockout that does not happen.
6. **"The same sentence" became "the same dependency."** The two admin screens carry two different German sentences saying the same thing. Nothing would have caught the stronger claim.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] The shipped Caddyfile's snippet was defined after its use and did not parse**

- **Found during:** Task 1
- **Issue:** The plan places the single sign-on block at the foot of the file, after `example.com { import holzcloud }`. Written that way and then uncommented as the file instructs, `caddy adapt` fails with `Error: File to import not found: holzcloud-sso, at …:47`. Caddy resolves `import` during parsing, in file order.
- **Fix:** The snippet definition and its explanation were moved above the primary domain block. The site blocks now follow the two snippet definitions, which is also the conventional Caddyfile layout.
- **Verification:** `caddy adapt` exit 0 on the file as shipped and on the file with the block uncommented and the site pointed at `holzcloud-sso`.
- **Committed in:** `e6ad663`

**2. [Rule 2 — Missing critical functionality] A duplicated security-header block would have shipped**

- **Found during:** Task 1
- **Issue:** The single sign-on site cannot `import holzcloud` — that snippet carries a `reverse_proxy localhost:8080` without the shared secret, and two terminal handlers in one site block is a directive-order question with no good answer. The alternative was to copy the `Strict-Transport-Security` / `nosniff` / `Referrer-Policy` / `-Server` block into the new snippet.
- **Fix:** The block was extracted into `(holzcloud-headers)`, imported by both. A duplicated set of security headers in an example file is a set that drifts, and drift here means one of two sites silently losing HSTS.
- **Verification:** `caddy adapt` output for the non-SSO path is byte-identical before and after the change.
- **Committed in:** `e6ad663`

**3. [Rule 1 — Bug] The stale `isTrustedProxy` reference, as the plan anticipated**

- **Found during:** Task 1 (recorded by the planner in `<flagged_assumptions>`, so **not** scope creep)
- **Issue:** `deploy/Caddyfile.example` told operators not to proxy from another host "without adjusting `internal/admin/login.go:isTrustedProxy`". That function does not exist anywhere in this repository and has not since the file was written in August. An instruction to adjust a function that does not exist is an instruction somebody follows by inventing one.
- **Fix:** Replaced with `HOLZCLOUD_TRUSTED_PROXIES` (the setting an operator actually changes) and `internal/web/clientip.go` (the code that reads it). The rest of the paragraph was kept — its advice was always right — and a sentence about `HOLZCLOUD_LISTEN` was added, because a proxy on another host also needs the CMS to bind something other than loopback.
- **Verification:** `grep -rn isTrustedProxy` outside `.planning/` returns 0.
- **Committed in:** `e6ad663`

**4. [Rule 1 — Bug] Two DEPLOY.md claims were true-ish and are now true**

- **Found during:** Task 2, verifying claims against the code
- **Issue:** (a) A draft sentence said an editor whose groups map to no configured website is refused, without plan 10-05's caveat that the website half does not run at all when the mapping is unset. (b) A draft sentence said "the same sentence is on the account screen and the user list"; the two templates carry two different sentences.
- **Fix:** Both rewritten. (a) now names `HOLZCLOUD_SSO_WEBSITE_GROUPS` and states both branches; (b) says "the same dependency".
- **Verification:** (a) against `internal/admin/forwardauth.go` and 10-05's recorded decision; (b) against `account.html:38` and `user_list.html:10`.
- **Committed in:** `e2e244f`

**5. [Rule 3 — Blocking] A version claim I could not check**

- **Found during:** Task 2
- **Issue:** A draft of the `HOLZCLOUD_LISTEN` table row said "Until version 1.10 the process listened on every interface". The unreleased version's number is not settled — `CHANGELOG.md`'s last release is 1.9 and `STATE.md` calls the milestone v1.6.
- **Fix:** Reworded to "The process used to listen on every interface; it no longer does", which is true whatever the release is numbered.
- **Committed in:** `e2e244f`

---

**Total deviations:** 5 auto-fixed (2 × Rule 1 on the Caddyfile, 1 × Rule 2, 1 × Rule 1 on prose, 1 × Rule 3)
**Impact on plan:** No scope creep. Three of the five are the plan's own file scope; deviation 3 was flagged by the planner in advance; deviation 2 is 6 lines moved with a proof that nothing changed. **Not one Go file was touched.**

## Findings to report

### 1. The plan's `isTrustedProxy` gate does not measure what its name claims — the fifth instance in this phase

The gate as written:

```bash
grep -rn isTrustedProxy . --include='*.go' --include='*.md' --include='Caddyfile.example' | grep -v '^\./\.planning/' | wc -l
```

On this machine it prints **14**, not 0, on a tree where the property fully holds. The exclusion anchors on `^\./`, and `grep` here (a shell function over `ugrep 7.8.4`) emits paths as `.planning/phases/…` with **no `./` prefix**, so the exclusion never fires and the gate counts the twelve occurrences in `10-08-PLAN.md` and the two in `10-10-PLAN.md` — the very files the exclusion exists to skip.

The check that actually holds the property:

```bash
grep -rn isTrustedProxy . --include='*.go' --include='*.md' --include='Caddyfile.example' | grep -v '^\(\./\)\?\.planning/' | wc -l   # 0
```

or, plainer and immune to the prefix question entirely:

```bash
grep -rn isTrustedProxy deploy/ docs/ internal/ cmd/ CHANGELOG.md README.md | wc -l   # 0
```

Both read 0. The stale reference is gone; only the gate was wrong. This is the phase's fifth instance of `10-CONTEXT.md`'s rule, and the first where the defective gate was one the plan asked the executor to *run* rather than one it wrote. The shape to remember: **an exclusion pattern is part of the gate, and a `^`-anchored path pattern is a bet on which tool prints the path.** Recorded in `.planning/WINDOWS.md` as a `deviation`.

There is a second, milder instance in the same plan. Task 3's changelog gate,

```bash
sed -n '/## Unveröffentlicht/,/^## [0-9]/p' CHANGELOG.md | grep -c '^- '
```

reads 0 both before and after this plan, because the section already contained no bullets. It cannot distinguish "this plan wrote prose" from "this plan wrote nothing" — it is a gate against a regression somebody else might cause, not a gate on this plan's own work. The property it names was held by reading the entry.

### 2. `go run ./tools/i18n` still reports 2 open strings, and this plan did not close them

**1321 Zeichenketten im Quelltext, 2 offen** per catalogue — identical to the measurement taken immediately before the first commit here. Both are plan 10-06's two new German sentences on the account screen and the user list. **Plan 10-09 owns writing them into the catalogues**; they were deliberately left alone and remain entry 10 in `.planning/WINDOWS.md`.

### 3. `docs/deployment.md` was not touched, and arguably should be

`docs/deployment.md` is the summary page that points at `deploy/DEPLOY.md`, and it now summarises a document that has a section it does not mention. It is outside this plan's `files_modified`, so nothing was written to it. It is a one-paragraph job for whoever runs the phase's documentation sweep.

### 4. Caddy 2.11.4 was installed on the development machine

`brew install caddy`. It is a build/verification tool, not a dependency of this project: `go.mod` is untouched, nothing in the binary or in CI references it, and every measurement it produced is recorded above so a reader does not have to reinstall it to trust them. It is the reason deviation 1 was found before the file shipped.

## The counting table, measured

Measured immediately before `e6ad663` and again after `5b1bba7`, as the plan requires.

| What | Command | Baseline | Predicted | Measured after |
|---|---|---|---|---|
| `request_header -X-authentik` lines | `grep -c 'request_header -X-authentik' deploy/Caddyfile.example` | 0 | +4 | **4** ✓ |
| `request_header -X_authentik` lines | `grep -c 'request_header -X_authentik' deploy/Caddyfile.example` | 0 | +4 | **4** ✓ |
| `CVE-2026-30851` in the Caddyfile | `grep -c CVE-2026-30851 deploy/Caddyfile.example` | 0 | >0 | **2** ✓ |
| `isTrustedProxy` in the Caddyfile | `grep -c isTrustedProxy deploy/Caddyfile.example` | 1 | −1 | **0** ✓ |
| `HOLZCLOUD_SSO_SECRET` in the Caddyfile | `grep -c HOLZCLOUD_SSO_SECRET deploy/Caddyfile.example` | 0 | 1 | **1** ✓ |
| absolute addresses in the Caddyfile | `grep -Ec 'https?://[a-z0-9.-]+' deploy/Caddyfile.example` | 2 | ≤2 | **2** ✓ |
| `2.11.2` in `DEPLOY.md` | `grep -c '2\.11\.2' deploy/DEPLOY.md` | 0 | ≥2 | **4** ✓ |
| `CVE-2026-30851` in `DEPLOY.md` | `grep -c CVE-2026-30851 deploy/DEPLOY.md` | 0 | >0 | **1** ✓ |
| `HOLZCLOUD_SSO_` in `DEPLOY.md` | `grep -c HOLZCLOUD_SSO_ deploy/DEPLOY.md` | 0 | ≥7 | **13** ✓ |
| `HOLZCLOUD_LISTEN` in `DEPLOY.md` | `grep -c HOLZCLOUD_LISTEN deploy/DEPLOY.md` | 0 | ≥3 | **4** ✓ |
| `HOLZCLOUD_LISTEN=0.0.0.0` in `DEPLOY.md` | `grep -c 'HOLZCLOUD_LISTEN=0.0.0.0' deploy/DEPLOY.md` | 0 | >0 | **3** ✓ |
| `connection refused` in `DEPLOY.md` | `grep -ci 'connection refused' deploy/DEPLOY.md` | 0 | >0 | **1** ✓ |
| `HOLZCLOUD_SSO_` in `configuration.md` | `grep -c HOLZCLOUD_SSO_ docs/configuration.md` | 0 | ≥7 | **8** ✓ |
| `authentik` in `security.md` | `grep -ci authentik docs/security.md` | 0 | >0 | **1** ✓ |
| `isTrustedProxy` in `deploy/ docs/ CHANGELOG README` | `grep -rn … \| wc -l` | 1 | 0 | **0** ✓ |
| `isTrustedProxy` repo-wide outside `.planning/` | corrected exclusion, see Findings | 1 | 0 | **0** ✓ (the plan's literal gate reads **14**; see Finding 1) |
| bullets in `## Unveröffentlicht` | `sed -n … \| grep -c '^- '` | 0 | 0 | **0** ✓ |
| Go files changed | `git diff --name-only 0c20f47..HEAD -- '*.go' \| wc -l` | — | 0 | **0** ✓ |
| strings in source | `go run ./tools/i18n \| head -1` | 1321 | 0 | **1321** ✓ |
| catalogue strings open | `go run ./tools/i18n` | 2 | 0 | **2** ✓ (10-06's, owned by 10-09) |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | 51 | 0 | **51** ✓ |

`caddy adapt` on the shipped file: exit 0. `caddy fmt` diff: empty. `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...`: all clean, 44 packages ok, 0 FAIL.

## Requirements Completed

- **SSO-11** — whole. The shipped example deletes the client's headers explicitly in both spellings and adapts cleanly on a real Caddy; `DEPLOY.md` names 2.11.2, the affected range, the CVE and the companion advisory; the two operator-owned facts are written as verifications with runnable checks; the acceptance test carries the sentence that separates it from a ritual.
- **SSO-07** — completed here. Plan 10-06 put the admin half on two screens; this plan writes the `DEPLOY.md` half, as its own section rather than a sentence in a paragraph.

## Known Stubs

None. Nothing in these five files is a placeholder, and no instruction points at something that does not exist — which is what deviations 1 and 3 were about.

## Threat Flags

None. This plan added no network endpoint, no auth path, no file access and no schema change. `deploy/Caddyfile.example`'s new content is inert: it is comment text until an operator uncomments it.

The register's own entries are covered:

| Threat ID | Disposition | Where |
|---|---|---|
| T-10-44 (CVE-2026-30851 on an operator's Caddy) | mitigate | Eight explicit deletes in the shipped example, 2.11.2 named as the floor with the affected range, and the CMS's own unconditional strip behind both |
| T-10-45 (the secret written into a Caddyfile) | mitigate | `{env.HOLZCLOUD_SSO_SECRET}` — a reference, never a value, exactly one occurrence, and measured to reach the CMS and not the outpost |
| T-10-46 (an acceptance test that passes for the wrong reason) | mitigate | "`Connection refused` is not a pass", with instructions for making the port answer |
| T-10-47 (a container that answers nothing after upgrading) | mitigate | The container bullet, the `docker run` line, the variable table and the changelog, which names it first |
| T-10-48 (a stale cross-reference) | mitigate | 0 occurrences outside `.planning/` |
| T-10-49 (the second-factor dependency in only one place) | mitigate | `DEPLOY.md` here plus 10-06's two screens, both verified present |

## Issues Encountered

Nothing beyond the five deviations. The only genuine surprise was how quickly `caddy adapt` disproved a file that read perfectly.

## Self-Check

- `deploy/Caddyfile.example` — FOUND
- `deploy/DEPLOY.md` — FOUND
- `docs/security.md` — FOUND
- `docs/configuration.md` — FOUND
- `CHANGELOG.md` — FOUND
- commit `e6ad663` — FOUND
- commit `e2e244f` — FOUND
- commit `5b1bba7` — FOUND

## Self-Check: PASSED
