---
phase: 10-authentik
plan: 01
subsystem: infra
tags: [config, sso, authentik, forward-auth, listen-address, argon2id, netip, slog]

requires:
  - phase: 01-foundation
    provides: "config.Load's collected-error contract (errs + errors.Join), envBool/envSize/getEnv, Config.LogValue, and the Payrexx pair as the precedent for refusing to start on half a configuration"
  - phase: 02-websites
    provides: "domain.Store.GetWebsite, which returns (nil, nil) for an id nobody created — the lookup the start-up refusal is built on"
provides:
  - "HOLZCLOUD_LISTEN, default 127.0.0.1: the process no longer binds every interface unless an operator says so, and the effective address is in the startup log"
  - "Seven SSO settings, all inert while HOLZCLOUD_SSO_ENABLED is false, with the shared secret environment-only and never logged"
  - "Four collected configuration refusals, each naming the variable to change"
  - "checkDefaultWebsite: the start-up refusal against the database, so an id nobody created stops the process instead of provisioning an account into nothing"
  - "config.Config.SSOEnabled/SSOSecret — the fields plan 10-02's forward-auth middleware is built from"
affects: [10-02-forward-auth, 10-03-identity-mapping, 10-04-provisioning, 10-07-admin-surface, 10-08-deploy-docs]

actuals:
  tokens: 7178
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Environment variable names single-sourced as unexported constants (envListen, envSSO*) so a refusal's sentence cannot drift from the name the operator has to change"
    - "A start-up check that needs the database is a function returning an error, called from main, which owns the slog.Error and the os.Exit — never inside newRouter, which is under test"
    - "isLocalPath: a configured redirect target is validated as one leading slash, never // and never /\\"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - cmd/holzcloud/main.go
    - cmd/holzcloud/main_test.go

key-decisions:
  - "The env var names are unexported constants rather than repeated literals — the only way the plan's own counting gate (7 HOLZCLOUD_SSO_ lines) and its own behaviour list (every refusal names its variables) can both hold"
  - "Startup refusals are plain English Go literals, not routed through the i18n catalogue — that is what the Payrexx and SMTP refusals already do, and cmd/ is outside the collector's reach anyway"
  - "checkDefaultWebsite was extracted as a named function taking a one-method websiteLookup interface, so the start-up refusal has a real test instead of an assertion about main()"
  - "isLocalPath also rejects /\\ (backslash), not only // — browsers treat both as protocol-relative, and the plan only named the double slash"

patterns-established:
  - "Refusals collect: every new check appends to errs and Load still returns errors.Join, proved by a test that sets three things wrong and counts three"
  - "Every guard added by this plan was mutation-verified: removed, watched a named test go red, restored"

requirements-completed: [SSO-04, SSO-05, SSO-10]

coverage:
  - id: D1
    description: "The server binds 127.0.0.1 unless HOLZCLOUD_LISTEN says otherwise, and the effective address is in the startup log"
    requirement: "SSO-10"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestSSOAndListenDefaults"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestListenAcceptsAnyAddressTheOperatorNames"
        status: pass
      - kind: integration
        ref: "netstat -an | grep '\\.8080.*LISTEN' against the built binary — default shows only 127.0.0.1.8080; HOLZCLOUD_LISTEN=0.0.0.0 and =:: show *.8080"
        status: pass
      - kind: other
        ref: "grep -c 'net.JoinHostPort(cfg.Listen, cfg.Port)' cmd/holzcloud/main.go == 1 and grep -c 'Addr:    \":\" + cfg.Port' == 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "The shared secret lives in the environment, never in the database, and never in the startup log — not even a prefix"
    requirement: "SSO-04"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestConfigLogValueCarriesTheSSOBlockButNotTheSecret"
        status: pass
      - kind: other
        ref: "ls internal/db/migrations/*.sql | wc -l unchanged at 50 — no schema was touched"
        status: pass
    human_judgment: false
  - id: D3
    description: "Single sign-on is off by default, and switched on without a secret the process refuses to start"
    requirement: "SSO-04"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestSSORefusalsNameTheirVariables/single_sign-on_without_a_secret"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestSSOAndListenDefaults"
        status: pass
    human_judgment: false
  - id: D4
    description: "Account creation is off by default, and switched on without HOLZCLOUD_SSO_DEFAULT_WEBSITE the process refuses to start with a sentence naming the website"
    requirement: "SSO-05"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestSSORefusalsNameTheirVariables/provisioning_without_a_default_website"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestSSORefusalsNameTheirVariables/provisioning_without_a_sign-on_path"
        status: pass
    human_judgment: false
  - id: D5
    description: "The default website is checked against the database at start-up, not at the first sign-in: an id nobody created is a startup failure naming it"
    requirement: "SSO-05"
    verification:
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestProvisioningRefusesToStartWithoutItsDefaultWebsite/an_id_nobody_ever_created"
        status: pass
      - kind: integration
        ref: "cmd/holzcloud/main_test.go#TestProvisioningRefusesToStartWithoutItsDefaultWebsite/provisioning_off_asks_the_database_nothing"
        status: pass
    human_judgment: false
  - id: D6
    description: "The sign-out target is validated as a path on this server, so a mistyped setting cannot become an open redirect out of the admin"
    requirement: "SSO-05"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestSSORefusalsNameTheirVariables/the_sign-out_target_is_an_absolute_address (and /is_protocol-relative, /is_a_backslash_escape, /is_not_a_path_at_all)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Every refusal is collected rather than fatal-on-first — three wrong settings produce three sentences from one Load()"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestThreeBadSettingsProduceThreeErrors"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/config/config.go | grep -c 'errs = append' == 24, up from 14"
        status: pass
    human_judgment: false
  - id: D8
    description: "HOLZCLOUD_SSO_WEBSITE_GROUPS parses group=websiteID pairs, and every malformed pair is its own named error"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestSSOWebsiteGroupsParsing"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestSSOWebsiteGroupsRejectsMalformedPairs (7 subtests)"
        status: pass
    human_judgment: false
  - id: D9
    description: "The loopback default is a behaviour change for every existing installation that does not front the service with a same-host proxy — including the shipped `docker run -p 8080:8080` instructions, which now need HOLZCLOUD_LISTEN=0.0.0.0"
    verification: []
    human_judgment: true
    rationale: "Whether an existing deployment breaks depends on the operator's own topology, which this repository cannot observe. deploy/DEPLOY.md is plan 10-08's file and was deliberately not touched here; until it is, an operator upgrading a container deployment loses the port. Recorded in the plan's own flagged_assumptions and repeated here so the phase cannot ship without 10-08 writing it down."

duration: 42 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 01: Settings and the two refusals Summary

**Nine environment settings, a loopback default that moves the admin port off every interface, and four collected refusals plus one database check that together make "provisioning on, default website unnamed" impossible to reach — the configuration that would otherwise hand the first stranger who authenticates editor access to every website.**

## Performance

- **Duration:** 42 min
- **Started:** 2026-09-07T22:50:00Z (approx — the executor did not capture a start stamp before reading context; first commit 2026-09-07T23:21:22Z)
- **Completed:** 2026-09-07T23:32:00Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- `HOLZCLOUD_LISTEN` with a `127.0.0.1` default. The one line that decides what the process binds is now `net.JoinHostPort(cfg.Listen, cfg.Port)`, verified by asking the operating system rather than the source: the built binary with no environment shows exactly `tcp4 127.0.0.1.8080 LISTEN` and nothing on `*.8080`.
- Seven SSO settings in one block, all inert while `HOLZCLOUD_SSO_ENABLED` is false. The shared secret is environment-only, for the reason already written above `PayrexxSecret`, and `LogValue` reports `sso_configured` as a boolean with the value absent — not even truncated.
- Four collected refusals in `config.Load`, and a fifth check in `main` against the database. The one that matters is the pair: `HOLZCLOUD_SSO_PROVISION` without `HOLZCLOUD_SSO_DEFAULT_WEBSITE` does not load, and a `HOLZCLOUD_SSO_DEFAULT_WEBSITE` naming a website nobody created does not start.
- `internal/admin/handler.go` was not touched. The inversion it contains is prevented at the other end, exactly as D-01 settled.
- Every guard was mutation-verified — removed, watched go red under a named test, restored. Eleven mutations, eleven reds, output below.

## Task Commits

1. **Task 1 (RED): failing tests for the SSO block and the listen address** — `63db4cf` (test)
2. **Task 1 (GREEN): the SSO block, the listen address, and the four refusals** — `1409e74` (feat)
3. **Task 2: bind the configured address, and refuse to start without the default website** — `cd6b542` (feat)
4. **Task 3: the arithmetic** — no code commit by design; the plan's own action says "No new code. This task is the arithmetic, written down." Its output is the table below.

**Plan metadata:** see the `docs(10-01)` commit that carries this file.

_Task 1 carried `tdd="true"`; the RED commit's tests do not compile against the pre-change `Config`, which is the intended failure._

## Files Created/Modified

- `internal/config/config.go` — the `Listen` field, the seven-field SSO block with its doc comments, the eight env-name constants, the four refusals, `isLocalPath`, `parseWebsiteGroups`, and nine new `LogValue` entries
- `internal/config/config_test.go` — nine new test functions, 26 named subtests, covering every bullet of the plan's behaviour list
- `cmd/holzcloud/main.go` — `net.JoinHostPort(cfg.Listen, cfg.Port)`, the `websiteLookup` interface, `checkDefaultWebsite`, and its call beside the other start-up checks
- `cmd/holzcloud/main_test.go` — `TestProvisioningRefusesToStartWithoutItsDefaultWebsite` with three subtests, and `refusingLookup`, which fails the test if the database is consulted with provisioning off

## Task 3: the counting gates, measured against the post-change tree

**The plan's baselines were measured on 2026-09-07, before phase 11 landed in this tree.** Four of them are stale. They are reported as-written and as-measured; the assertion this plan actually makes is the **delta**, and every delta is what the plan predicted.

| What | Command | Plan's baseline (2026-09-07) | Measured baseline (2026-09-08, pre-change) | This plan adds | Measured after | Verdict |
|---|---|---|---|---|---|---|
| `HOLZCLOUD_` reads in config.go | `grep -v '^[[:space:]]*//' internal/config/config.go \| grep -c HOLZCLOUD_` | 47 | **47** | 8 | **55** | exactly as predicted |
| `HOLZCLOUD_SSO_` reads | `grep -v '^[[:space:]]*//' internal/config/config.go \| grep -c HOLZCLOUD_SSO_` | 0 | **0** | 7 | **7** | exactly as predicted |
| collected errors | `grep -v '^[[:space:]]*//' internal/config/config.go \| grep -c 'errs = append'` | 14 | **14** | ≥4 | **24** | ≥18 satisfied (10 added: 6 in `Load`, 4 in `parseWebsiteGroups`) |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | 49 | **50** | 0 | **50** | **baseline stale**; delta 0 as required |
| packages under `internal/` | `ls -d internal/*/ \| wc -l` | 40 | **41** | 0 | **41** | **baseline stale**; delta 0 as required |
| files using `RemoteAddr` | `grep -rn RemoteAddr internal/ cmd/ --include='*.go' \| grep -v _test \| wc -l` | 3 | **3** | 0 | **3** | exactly as predicted |
| `IsTrustedPeer` mentions | `grep -rn IsTrustedPeer internal/ cmd/ --include='*.go' \| grep -v _test \| wc -l` | 3 | **3** | 0 | **3** | exactly as predicted |
| strings in source | `go run ./tools/i18n \| head -1` | 1277 | **1311** | 0 | **1311** | **baseline stale**; delta 0 as required |
| admin templates | `ls cmd/holzcloud/templates/admin/*.html \| wc -l` | 66 | **68** | 0 | **68** | **baseline stale**; delta 0 as required |
| `layoutPageNames` entries | the slice at `internal/web/render.go:51` | 50 | **50** | 0 | **50** | exactly as predicted |

### The four rows that are assertions of absence

- **Migrations: 50 before, 50 after.** D-05 holds. `00001:6` declares `password TEXT NOT NULL` with no CHECK, so an Argon2id hash of random bytes is a legal value, and `00001:7`'s table-head `CHECK (role IN ('admin','editor'))` is why there is no third role. Nothing in this plan reached for the schema. The plan predicted 49 because it was written before phase 11's migration; the invariant it was protecting is the **delta**, and the delta is 0.
- **`RemoteAddr`: 3 before, 3 after.** The three are `internal/web/clientip.go:33`, `clientip.go:51` and `internal/web/logging.go:130` — verified by name, not by count alone. No second trusted-peer check was written.
- **`IsTrustedPeer`: 3 before, 3 after.** `logging.go:35`, `clientip.go:47` (the doc comment's own line) and `clientip.go:50`. This plan adds no caller; plan 10-02 adds exactly one, and if `RemoteAddr` rises with it, that is a second check wearing the right name.
- **No new user-visible string.** `go run ./tools/i18n` reports `1311 Zeichenketten im Quelltext` before and after, and `34 offen, 0 verwaist` on `en`, `es`, `fr` and `it` before and after. **Those 34 are pre-existing and belong to plan 11-07** — this plan added none of them and closed none of them. The startup refusals are plain Go literals in `internal/config` (collected roots include `internal/`, so a translated helper would have shown here) and the `main.go` log lines are outside the collector's roots by design.

### Divergences, and their cause

Four baselines diverge, all with one cause: **the plan was authored on 2026-09-07 against a tree that phase 11 has since advanced.** Migrations 49→50, packages 40→41, admin templates 66→68, source strings 1277→1311. None of the four is a change this plan made; all four measured identical before and after my first commit. Per the plan's own instruction ("A divergence is a finding to write down, not a number to adjust") the stale numbers are written down rather than the plan being edited, and the numbers a later phase should carry forward are the **measured** column.

**Consequence for the plan's literal verify gates:** `go run ./tools/i18n | head -1` was specified to fail unless it prints `1277 Zeichenketten im Quelltext`, and `ls internal/db/migrations/*.sql | wc -l` was specified to fail unless it prints 49. Both gates fail as literally written against this tree, and would fail identically on an empty commit. They are reported here as **stale-baseline gate failures, not implementation failures**, and re-anchored to the deltas above. This is the same failure family `10-CONTEXT.md` records from phase 9 — a gate whose number was fixed against a tree that then moved.

## Decisions Made

- **Environment variable names are unexported constants** (`envListen`, `envSSOEnabled`, …), not repeated literals. This was forced by the plan itself: its counting gate demands exactly 7 non-comment lines containing `HOLZCLOUD_SSO_` in `config.go`, while its behaviour list demands that each refusal's sentence names its variables. With repeated literals those two are contradictory (four refusals plus `parseWebsiteGroups` would put the count at 11+). Constants satisfy both, and are better anyway: a renamed variable cannot drift from the message telling an operator what to rename. Both gates land on the plan's exact predicted numbers, 7 and 55.
- **Refusal sentences are English plain literals, not catalogue entries.** The precedent is what the Payrexx and SMTP refusals do — a Go literal in `errors.New`/`fmt.Errorf`, never `web.T`. The *language* of the file is mixed (Payrexx and SMTP refusals are German; the Argon2, megapixel and `parsePrefixes` refusals are English); English was chosen because `10-CONTEXT.md` records the 2026-09-06 decision that this project's code and new artifacts are English. The load-bearing half of the precedent — not routed through the catalogue — is followed exactly, which is why `tools/i18n` is unchanged.
- **`checkDefaultWebsite` is a function returning an error**, taking a one-method `websiteLookup` interface, so the start-up refusal has a real test. `main` keeps the `slog.Error` and the `os.Exit(1)`. This honours the plan's "a fatal exit does not belong in a function a test calls" while making the acceptance criterion ("a test proves an id nobody created is a startup failure naming it") actually provable.
- **`HOLZCLOUD_SSO_DEFAULT_WEBSITE` is read through the existing `envSize` helper**, which already appends "must be positive, got 0" for `0` and `-1`. That gives the plan's "`=0` and `=-1` are the same error as absent" for free and keeps the collected-error contract.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] `isLocalPath` also rejects a leading `/\`**

- **Found during:** Task 1
- **Issue:** The plan specifies the sign-out target must begin with exactly one `/` and names `//` as the failure. `/\evil.example` is the same open redirect with the separator browsers also accept — WHATWG URL parsing treats `\` as equivalent to `/` in the authority position, so `/\evil.example` navigates off-origin exactly as `//evil.example` does. A validator that only checks `//` is a check that is correct at every site the author remembered, which is the shape `10-CONTEXT.md` names as the one to fear.
- **Fix:** `isLocalPath` returns false for a leading `//` **and** a leading `/\`.
- **Files modified:** `internal/config/config.go`, `internal/config/config_test.go`
- **Verification:** `TestSSORefusalsNameTheirVariables/the_sign-out_target_is_a_backslash_escape`; mutation-verified (removing the guard turns four subtests red).
- **Committed in:** `1409e74`

**2. [Rule 3 - Blocking] Env var names hoisted into constants**

- **Found during:** Task 1
- **Issue:** The plan's `HOLZCLOUD_SSO_` gate (exactly 7) and its behaviour list (every refusal names its variables) cannot both hold if the names are repeated as literals — the refusals alone would push the count past 11.
- **Fix:** An eight-name `const` block; every read and every message formats a constant with `%s`.
- **Files modified:** `internal/config/config.go`
- **Verification:** Gate measures 7 and 55, the plan's exact predictions; `TestSSORefusalsNameTheirVariables` proves the runtime messages still name the variables.
- **Committed in:** `1409e74`

**3. [Rule 2 - Missing Critical] A test file added to Task 2's file set**

- **Found during:** Task 2
- **Issue:** The plan lists only `cmd/holzcloud/main.go` for Task 2, but the plan's own must-have truth ("checked against the database at start-up … a test proves an id nobody created is a startup failure naming it") cannot be met without a test.
- **Fix:** `TestProvisioningRefusesToStartWithoutItsDefaultWebsite` in `cmd/holzcloud/main_test.go`, with a `refusingLookup` stand-in proving the database is not consulted at all when provisioning is off.
- **Files modified:** `cmd/holzcloud/main_test.go`
- **Verification:** Three subtests pass; mutation-verified twice.
- **Committed in:** `cd6b542`

### Findings recorded rather than worked around

**4. `internal/admin/handler.go`'s inversion is at line 183, not 178 and not 173.**

`ROADMAP.md:474` cites 173; `10-CONTEXT.md` D-01 cites 173; the plan's `flagged_assumptions` corrects it to 178 and the executor prompt confirmed 178. Measured on this tree today:

```
$ grep -n 'return assigned == 0 || mine > 0' internal/admin/handler.go
183:		return assigned == 0 || mine > 0
```

The code is exactly as described in all three documents — `NewWebsiteAccessLookup` in `internal/admin/handler.go`, ending in that return. Only the citation drifted, twice, and phase 11 moved it a further five lines. **The doc comments this plan wrote deliberately cite the function by name (`NewWebsiteAccessLookup`) rather than by line number**, so they cannot rot again. Later plans in this phase should do the same; a line number in this file has now been wrong in three separate documents.

**5. The plan's `HOLZCLOUD_` count gate is satisfiable only with the constants of deviation 2.** Recorded above; not a defect in the code, a contradiction in the plan as written.

**6. Two stale verify gates.** `go run ./tools/i18n | head -1` (expects 1277) and `ls internal/db/migrations/*.sql | wc -l` (expects 49) fail on this tree for reasons that predate this plan. See "Divergences" above.

---

**Total deviations:** 3 auto-fixed (2 missing-critical, 1 blocking) + 3 findings recorded.
**Impact on plan:** No scope creep. Two of the three auto-fixes exist to make the plan's own stated must-haves provable; the third closes an open-redirect variant the plan named only half of.

## Mutation Verification

Every guard this plan adds was removed, run against a named test, and restored. **Actual red output, verbatim.**

**1. The refusal for SSO enabled with no secret**
```
--- FAIL: TestSSORefusalsNameTheirVariables/single_sign-on_without_a_secret (0.00s)
        config_test.go:289: map[HOLZCLOUD_SSO_ENABLED:true] was accepted; want a refusal
```

**2. The refusal for provisioning without a sign-on path**
```
--- FAIL: TestSSORefusalsNameTheirVariables/provisioning_without_a_sign-on_path (0.00s)
        config_test.go:293: the refusal must name HOLZCLOUD_SSO_ENABLED so the operator knows what to
        change: HOLZCLOUD_SSO_PROVISION is on but HOLZCLOUD_SSO_DEFAULT_WEBSITE names no website: a
        provisioned account with no website assignment is an account with access to every website
```
(Note what this red proves: with the guard gone the operator is told about the *website* and never about the *switch* — the collected-error contract still fired, but the sentence they needed was missing.)

**3. The refusal for provisioning without a default website — the one this phase exists for**
```
--- FAIL: TestSSORefusalsNameTheirVariables/provisioning_without_a_default_website (0.00s)
        config_test.go:289: map[HOLZCLOUD_SSO_ENABLED:true HOLZCLOUD_SSO_PROVISION:true
        HOLZCLOUD_SSO_SECRET:a-shared-secret] was accepted; want a refusal
```

**4. The sign-out path validation**
```
--- FAIL: TestSSORefusalsNameTheirVariables/the_sign-out_target_is_an_absolute_address (0.00s)
        config_test.go:289: map[HOLZCLOUD_SSO_SIGN_OUT_PATH:https://evil.example/sign_out] was accepted; want a refusal
--- FAIL: TestSSORefusalsNameTheirVariables/the_sign-out_target_is_protocol-relative (0.00s)
        config_test.go:289: map[HOLZCLOUD_SSO_SIGN_OUT_PATH://evil.example/sign_out] was accepted; want a refusal
--- FAIL: TestSSORefusalsNameTheirVariables/the_sign-out_target_is_a_backslash_escape (0.00s)
        config_test.go:289: map[HOLZCLOUD_SSO_SIGN_OUT_PATH:/\evil.example/sign_out] was accepted; want a refusal
--- FAIL: TestSSORefusalsNameTheirVariables/the_sign-out_target_is_not_a_path_at_all (0.00s)
        config_test.go:289: map[HOLZCLOUD_SSO_SIGN_OUT_PATH:outpost.goauthentik.io/sign_out] was accepted; want a refusal
```

**5. The loopback default reverted to `0.0.0.0`**
```
--- FAIL: TestSSOAndListenDefaults (0.00s)
    config_test.go:140: Listen: want 127.0.0.1, got "0.0.0.0"
```

**6. The listen address no longer parsed**
```
--- FAIL: TestSSORefusalsNameTheirVariables/listen_is_not_an_address (0.00s)
        config_test.go:289: map[HOLZCLOUD_LISTEN:nonsense] was accepted; want a refusal
```

**7. The duplicate-group refusal turned into last-one-wins**
```
--- FAIL: TestSSOWebsiteGroupsRejectsMalformedPairs/a_group_listed_twice (0.00s)
        config_test.go:357: "redaktion-a=1,redaktion-a=2" was accepted; want a refusal
```

**8. The shared secret written into the startup log**
```
--- FAIL: TestConfigLogValueCarriesTheSSOBlockButNotTheSecret (0.00s)
    config_test.go:414: the shared secret is in the startup log: [... sso_configured=true
    sso_secret_prefix=correct-horse-battery-staple sso_provision=false ...]
    config_test.go:418: a prefix of the shared secret is in the startup log: [...]
```

**9. The start-up existence check removed (`ws == nil` no longer refuses)**
```
--- FAIL: TestProvisioningRefusesToStartWithoutItsDefaultWebsite/an_id_nobody_ever_created (0.00s)
        main_test.go:635: a website that does not exist was accepted; want a refusal to start
```

**10. `checkDefaultWebsite` short-circuited to `return nil`**
```
--- FAIL: TestProvisioningRefusesToStartWithoutItsDefaultWebsite/an_id_nobody_ever_created (0.00s)
        main_test.go:635: a website that does not exist was accepted; want a refusal to start
```

**11. The every-interface address restored** (grep gate, not a test)
```
gate 'old address absent'  (want 0):  1   <- red
gate 'JoinHostPort present' (want 1): 0   <- red
# after restore
gate 'old address absent'  (want 0):  0
gate 'JoinHostPort present' (want 1): 1
```

After every mutation the file was restored with `git checkout --` and the suite re-run green.

## Verification Results

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | silent |
| `gofmt -l .` | silent |
| `go test ./...` | **0 failures** |
| `go test ./internal/config/ -v` | 9 new test functions, 26 named subtests, all PASS |
| `go run ./tools/i18n` | `1311 Zeichenketten im Quelltext`; `34 offen, 0 verwaist` on en/es/fr/it — **unchanged**, no new open strings |
| default bind address, asked of the OS | `tcp4 127.0.0.1.8080 LISTEN`, and `0` rows on `*.8080` |
| `HOLZCLOUD_LISTEN=0.0.0.0` | `tcp46 *.8080 LISTEN` |
| `HOLZCLOUD_LISTEN=::` | `tcp46 *.8080 LISTEN` — `JoinHostPort` bracketed it; `":" + Port` would not have |
| startup log, `configuration loaded` | `listen=127.0.0.1 sso_enabled=false sso_provision=false sso_configured=false sso_admin_group='' sso_default_website=0 sso_website_groups=0 sso_sign_out_path=/outpost.goauthentik.io/sign_out`; the secret is absent |

The `ss`/`netstat` gate: this is macOS, `ss` does not exist, `netstat -an` answered. The plan allows this explicitly.

## Issues Encountered

- **An uncommitted task was lost to a mutation revert and had to be re-applied.** During mutation testing of Task 2 I ran `git checkout -- cmd/holzcloud/main.go` to restore a mutated guard while Task 2 itself was still uncommitted — which restored the file to HEAD, i.e. to before Task 2. Caught immediately by the grep gates flipping to `0`/`1` on the restored file. Re-applied identically and verified; the mutation runs were then repeated against the committed version, which is what the output above records. Lesson for later plans in this phase: commit the task before mutating it.
- **Two other agents are committing in this tree concurrently** (`50fe048`, `6df5960` landed between my commits). Every commit here was staged file by file and verified with `git show --stat`; none of my three commits touches a file outside this plan's set.

## Threat Flags

None. No new network endpoint, no new auth path, no new file access pattern, no schema change. The one new trust-boundary surface — the environment block — is exactly what the plan's threat register (T-10-01 … T-10-06) already covers, and every `mitigate` disposition in that register has a passing test above.

## Known Stubs

None. Every setting this plan adds is read, validated, logged and tested. Four of them (`SSOAdminGroup`, `SSOWebsiteGroups`, `SSOSecret`, `SSOSignOutPath`) have no consumer yet — that is the plan's design, not a stub: they are the fields plans 10-02 through 10-07 are built from, and none of them can be nil or unvalidated when those plans read them.

## User Setup Required

None yet. The environment block is documented in code comments; `deploy/DEPLOY.md` is plan 10-08's file and was deliberately not touched.

**One warning that must not be lost before 10-08:** the loopback default is a behaviour change for any deployment that does not front the service with a proxy on the same host, and it **breaks the shipped `docker run -p 8080:8080` instructions** at `deploy/DEPLOY.md:34-57` — a process binding loopback inside its own network namespace answers nobody. The documented answer is `HOLZCLOUD_LISTEN=0.0.0.0` in the container, proved working above. Plan 10-08 owns writing that into the container section.

## Next Phase Readiness

- `cfg.SSOEnabled` and `cfg.SSOSecret` exist and cannot be half-set — plan 10-02's forward-auth middleware can be built on them without a nil check.
- `cfg.SSOWebsiteGroups` and `cfg.SSOAdminGroup` are parsed and validated for plan 10-03's identity mapping. `SSOAdminGroup` may legitimately be empty; that is a configuration, not a missing value.
- `cfg.SSODefaultWebsite` is guaranteed positive **and** guaranteed to exist in the database whenever `cfg.SSOProvision` is true. Plan 10-04 can assign to it without re-checking.
- `IsTrustedPeer` is still at 3 mentions and `RemoteAddr` still at 3 reads. Plan 10-02 must add exactly one `IsTrustedPeer` call site and zero `RemoteAddr` reads; a rise in both is a second trusted-peer check wearing the right name.
- **Carry the measured baselines forward, not the plan's:** migrations 50, packages 41, admin templates 68, source strings 1311, `layoutPageNames` 50.

## Self-Check: PASSED

- `internal/config/config.go` — FOUND, contains `HOLZCLOUD_SSO_ENABLED`
- `internal/config/config_test.go` — FOUND, contains `SSODefaultWebsite`
- `cmd/holzcloud/main.go` — FOUND, contains `JoinHostPort`
- `cmd/holzcloud/main_test.go` — FOUND, contains `checkDefaultWebsite`
- Commit `63db4cf` — FOUND
- Commit `1409e74` — FOUND
- Commit `cd6b542` — FOUND
- `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` — all clean at the time of writing

---
*Phase: 10-authentik*
*Completed: 2026-09-08*
