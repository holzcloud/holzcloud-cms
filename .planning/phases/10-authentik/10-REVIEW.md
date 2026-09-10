---
phase: 10-authentik
reviewed: 2026-09-10T13:15:04Z
tree_reviewed: 480f21b
depth: deep
files_reviewed: 22
files_reviewed_list:
  - cmd/holzcloud/main.go
  - cmd/holzcloud/main_test.go
  - cmd/holzcloud/templates/admin/account.html
  - cmd/holzcloud/templates/admin/user_list.html
  - deploy/Caddyfile.example
  - deploy/DEPLOY.md
  - docs/configuration.md
  - docs/security.md
  - internal/admin/forwardauth.go
  - internal/admin/forwardauth_test.go
  - internal/admin/login.go
  - internal/admin/login_test.go
  - internal/admin/twofactor.go
  - internal/admin/twofactor_sso_test.go
  - internal/admin/user.go
  - internal/auth/session.go
  - internal/auth/twofactor.go
  - internal/auth/twofactor_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/web/forwardauth.go
  - internal/web/forwardauth_test.go
findings:
  critical: 3
  warning: 8
  info: 6
  total: 17
status: issues_found
---

# Phase 10: Code Review Report

> **Messhinweis.** Dieser Durchgang fehlte, obwohl die Roadmap Welle 9 ausdrücklich
> auf „the code-review fix round" sperrt; er ist am 2026-09-10 nachgeholt. Sein
> isolierter Worktree stand auf `d4ca500` (`origin/main`, ohne Phase 10). Der
> Prüfer hat das bemerkt und in einer `git archive 480f21b`-Kopie gemessen; im
> Hauptbaum wurde nichts geschrieben. Ein erster Anlauf ist am Wochenlimit
> gestorben, bevor er etwas schrieb. Der Bericht ist der des Prüfers, unverändert
> bis auf diesen Hinweis, `tree_reviewed` im Kopf und den Abschnitt „Stand der
> Behebung" am Ende.

**Reviewed:** 2026-09-10T13:15:04Z
**Depth:** deep (cross-file: request chain, forward-auth → sign-in → rights sync → user store → access lookup, config → start-up checks, session lifecycle)
**Files Reviewed:** 22 (the files carried by the phase-10 commits). Files called from them were also read: `internal/user/rights.go`, `internal/user/store.go`, `internal/admin/handler.go`, `internal/admin/website.go`, `internal/admin/confirm.go`, `internal/auth/middleware.go`, `internal/auth/elevate.go`, `internal/web/clientip.go`, `internal/web/logging.go`, `internal/db/migrations/00033_user_rights.sql`, `cmd/holzcloud/cli.go`, and `scs/v2@v2.9.0/data.go`.
**Status:** issues_found

## Measurement setup

The assigned worktree was at `d4ca500`, which predates phase 10. Every measurement ran on a `git archive 480f21b` copy in the scratchpad; the main checkout was never written. The baseline was green for `internal/web`, `internal/admin`, `internal/auth`, `internal/config` and `cmd/holzcloud`. Findings rest on measurement tests (M1–M7) and mutations (A–G). Each was run in the copy, reverted, and verified with `cmp`/`diff -r`. Anything not measured is labelled as unmeasured.

## Summary

The transport layers are sound, and the tests hold them:

- **Peer before header** (`IsTrustedPeer` removed → mutation E red).
- **The strip, including the underscore spelling** (normalisation removed → mutation C red in both `internal/web` and `cmd/holzcloud`).
- **Sign-out path validation** (`isLocalPath` always true → mutation D red).
- **The "no matching website group" refusal** (guard disabled → mutation G red).

The Caddyfile example deletes both spellings before `forward_auth`.

The phase fails one layer further in: at **which local account a believed identity becomes, and what that account may reach afterwards**.

1. The account is chosen by `X-authentik-email` alone. The username is only logged. The last-administrator safeguard then *keeps* the admin role for an identity that is in no group. Measured: an identity provider user "mallory" with no groups, carrying the sole administrator's address, is signed in as that administrator with role `admin` and no TOTP (CR-01).
2. The phase guards the "zero rows in `user_websites` = every website" inversion at creation and at "no matching group". It misses two other roads to the same state. Both are measured: a failing `SetRights` after its committed `DELETE` (CR-02), and the `ON DELETE CASCADE` when the default website is deleted at runtime (CR-03). This is the project's recurring pattern: correct at the known places, silently wrong at an overlooked one.

### Verdict on the claims of the aborted earlier run

| Claim | Verdict | Where |
|---|---|---|
| Deleting the default website → CASCADE → "every website" | **Confirmed, measured (M3)**. Critical | CR-03 |
| `SetRights` not transactional → empty assignment → "every website" | **Confirmed, measured (M2)**. Critical; the trigger is an unvalidated website-group id | CR-02 |
| Demoted admin → editor with every website | **Confirmed, measured (M4/M4b)**. Downgraded to Warning: it is less than the person had; the defect is a role write committed before the refusal | WR-01 |
| `via_sso` survives SSO off; 2FA readers ignore `cfg.SSOEnabled`, logout does not | **Confirmed, measured (M5)**. Also, a password login inherits the mark (code-read in scs) | WR-04 |
| Multiple values under one identity header | **Confirmed, measured (M6)**: the first value wins | WR-02 |
| `HOLZCLOUD_TRUSTED_PROXIES` implausible with SSO on | **Confirmed, measured (M7)**: `0.0.0.0/0,::/0` loads without error | WR-05 |
| No minimum length for `HOLZCLOUD_SSO_SECRET` | **Confirmed, measured (M7)**: a 1-character secret loads | WR-06 |
| `ConstantTimeCompare` and the chain position held by no test | **Confirmed by mutation** (A green, B green) | IN-01, IN-02 |
| `cli.go` restates the second-factor rule, wrong since phase 10 | **Partly confirmed**: a second predicate, correct for the password path, misleading for an SSO admin. File out of scope | IN-05 |
| Logout branches on `HX-Request` without `Vary` | **Confirmed** (code + grep: no test asserts `Vary`) | IN-03 |
| `RequireFreshPassword` locks a provisioned account out of five buttons | **Confirmed** (code-read; five `requireFresh` routes, a password nobody knows) | WR-07 |

New in this pass: CR-01, WR-03 and WR-08.

## Critical Issues

### CR-01: A believed identity becomes whichever local account has its e-mail address, including an administrator; the last-admin fallback then grants `admin` to an identity in no group

**File:** `internal/admin/forwardauth.go:154-169`, `internal/admin/forwardauth.go:327-341`, `internal/web/forwardauth.go:72-77`, `deploy/DEPLOY.md:269-276`

**Issue:**
- **How the account is chosen.** `ForwardAuthSignIn` resolves the account with `h.users.GetByEmail(r.Context(), email)` (line 169). Nothing ties the identity provider identity to the local row: no stored username, no subject, no "this account was provisioned/linked by SSO" marker. `ident.Username` is used only in log lines (174, 232, 258, 571, 621). The doc comment "The identity is pinned to the username" (`internal/web/forwardauth.go:72`) and DEPLOY.md's warning that renaming users "creates a new account here" (275-276) are both false. A rename changes nothing, and an address change switches accounts.
- **Existing accounts are taken over.** Every existing account is reachable this way, including hand-made administrators with TOTP, because `via_sso` switches the second factor off (`auth.MustHaveSecondFactor`).
- **What the group sync then does.** `syncRightsFromGroups` computes `want = editor` for an identity outside `SSOAdminGroup` and calls `Update`. For the last administrator that returns `ErrLastAdmin`, and the branch at 330-340 sets `want = u.Role` and *continues the sign-in as admin*.

**Measured (M1):**
- *Setup:* `newRightsSyncAdmin` (admin group and two website groups configured), and a sole admin `boss@example.com`.
- *Request:* from a trusted peer with the right secret: `X-authentik-username: mallory`, `X-authentik-email: boss@example.com`, `X-authentik-groups:` (empty).
- *Result:* `session user_id=1 (boss=1) session role="admin" stored role="admin" viaSSO=true`.

**Measured (M1b):**
- *Setup:* the same, plus a second admin; the request carries group `seite-a`.
- *Result:* `session user_id=1 (boss=1) role="editor" stored="editor"`. The real administrator is demoted, and mallory holds their account.

**Who can trigger it:**
- Anyone who can make the identity provider emit a chosen `X-authentik-email` for an identity they control. That includes self-service profile editing, enrollment flows without address verification, and identity provider operators with fewer rights than a CMS admin.
- On Caddy 2.10.0–2.11.1 without the example's `request_header` deletes, it also includes any identity provider account that has no address (see WR-03).

*Unmeasured:* whether Authentik lets users change their own address by default.

`TestSyncRightsKeepsTheLastAdministrator` asserts the fallback as intended behaviour, so the suite currently holds the hole in place.

**Fix:**
1. Bind on something the identity provider controls and the user does not: store the username on the account (new column, `UNIQUE`) and look it up by that. Sign in by address only for accounts SSO created or an administrator explicitly linked.
2. Until then, refuse at least these two cases:

```go
// after step 5, before syncRightsFromGroups
if u.Role == user.RoleAdmin &&
	(h.cfg.SSOAdminGroup == "" || !ident.HasGroup(h.cfg.SSOAdminGroup)) {
	h.refuseSSO(r, ident, email, "admin_without_admin_group")
	next.ServeHTTP(w, r)
	return
}
```

and in `syncRightsFromGroups` replace the `ErrLastAdmin` continuation with a refusal (`return "", errSSOLastAdmin`). A demotion that cannot be applied must not become a sign-in with the role it was meant to remove.

### CR-02: A `SetRights` failure during group sync leaves the account with an empty assignment — every website; the website-group ids are never checked against the database

**File:** `internal/user/rights.go:93-121`, `internal/admin/forwardauth.go:439-447`, `internal/config/config.go:297`, `cmd/holzcloud/main.go:654-669`

**Issue:** `SetRights` runs three independent autocommit statements: `UPDATE users` (98), `DELETE FROM user_websites` (102), then one `INSERT` per id (115). If an `INSERT` fails, the `DELETE` has already committed.

- **Sync path:** the error returns at forwardauth.go:446, the caller refuses the sign-in (247-261), and nothing restores the rows. The account is left with `assigned == 0`, which `NewWebsiteAccessLookup` (`internal/admin/handler.go:183`) reads as every website. Its existing sessions, and the password form, now reach every website.
- **Provisioning path:** there is a compensation (`Delete`, 553). The sync path has none.

**The trigger is plain configuration.** `parseWebsiteGroups` accepts any positive id, and `checkDefaultWebsite` checks only `SSODefaultWebsite`. M7: `HOLZCLOUD_SSO_WEBSITE_GROUPS=ghost=9999` loads with `err=<nil>`. The FK is enforced (`internal/db/db.go:18`, `foreign_keys(1)`), so the `INSERT` for a missing website fails.

**A valid mapping does not protect.** The ids are sorted ascending before insertion, so if the *lowest* mapped id no longer exists, the first `INSERT` fails and every row is lost, even when the person also carries valid groups.

A second trigger, *unmeasured*: the request context is cancelled (the client disconnects) between the `DELETE` and the `INSERT`.

**Measured (M2):**
- *Setup:* website groups `{seite-a→A, seite-b→B, ghost→9999}`, and an editor assigned `[A]`.
- *Before:* `siteB=false`.
- *Request:* SSO sign-in with groups `ghost`.
- *Result:* `sign-in user_id=0; assigned after=[]; siteB after=true`.

`TestLosingEveryWebsiteGroupDoesNotGrantEveryWebsite` (held, mutation G red) covers only the `len(ids)==0` branch, not this road.

**Fix:** make `SetRights` atomic and validate the mapping at start-up.

```go
func (s *Store) SetRights(ctx context.Context, id int64, rights Rights) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rights: %w", err)
	}
	defer tx.Rollback()
	// … the same three statements on tx …
	return tx.Commit()
}
```

In `checkDefaultWebsite`, call `websites.GetWebsite` for every value of `cfg.SSOWebsiteGroups` whenever `cfg.SSOEnabled`, and refuse to start on a missing one.

### CR-03: Deleting the default website at runtime cascades every provisioned account's only row away — every website — and the next SSO sign-in succeeds

**File:** `internal/admin/website.go:301-311`, `internal/db/migrations/00033_user_rights.sql:30`, `cmd/holzcloud/main.go:654-669`, `internal/admin/forwardauth.go:391-393`

**Issue:**
- **Where the guard stops.** The phase's own comment says "an account that exists with zero rows in user_websites is the vulnerability itself, for however long it exists" (forwardauth.go:209-213). It guards that state only at start-up (`checkDefaultWebsite`).
- **The cascade.** `user_websites.website_id` is `REFERENCES websites(id) ON DELETE CASCADE`, and `HandleWebsiteDelete` has no guard. Deleting the website named by `HOLZCLOUD_SSO_DEFAULT_WEBSITE` silently removes the single row of every account provisioned into it.
- **No self-repair.** With `HOLZCLOUD_SSO_WEBSITE_GROUPS` unset, `syncRightsFromGroups` returns before touching the assignment (391-393), so later sign-ins do not repair it either.
- **Discovered late.** The operator finds out at the next restart, when `checkDefaultWebsite` refuses to start the process. By then the accounts already reach every website.

**Measured (M3):**
- *Setup:* provisioning on, default website `home`.
- *After provisioning:* `user_id=1 assigned=[2] other=false`.
- *Action:* `DeleteWebsite(home)`.
- *Result:* `assigned=[] other=true; next SSO sign-in user_id=1`.

The trigger is an administrator action (`requireAdmin(requireFresh(...))`, main.go:858), not an attack. The beneficiaries are every stranger provisioning ever let in. The same cascade already affected hand-made single-website editors before phase 10; phase 10 is what creates such accounts automatically.

**Fix:** refuse the deletion while it would leave any editor with zero rows, and always refuse it for `cfg.SSODefaultWebsite`:

```go
if h.cfg != nil && h.cfg.SSOProvision && id == h.cfg.SSODefaultWebsite {
	web.SetFlashError(h.sm, r.Context(), "…")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d", id))
}
var orphaned int
_ = h.db.Read.QueryRowContext(r.Context(), `
	SELECT COUNT(*) FROM users u
	 WHERE u.role = 'editor'
	   AND EXISTS (SELECT 1 FROM user_websites w WHERE w.user_id = u.id AND w.website_id = $1)
	   AND NOT EXISTS (SELECT 1 FROM user_websites w WHERE w.user_id = u.id AND w.website_id <> $1)`,
	id).Scan(&orphaned)
```

(The flash text goes through the catalogue.)

## Warnings

### WR-01: Group sync writes a demotion before deciding to refuse, leaving a refused former admin as an editor with every website

**File:** `internal/admin/forwardauth.go:327-358`, `internal/admin/forwardauth.go:413-432`

**Issue:** `Update(..., want)` commits at 328. The "no matching website group" refusal is decided only at 413-431. A demoted admin whose groups carry no website group is refused, but the role change stays, and admins normally have no `user_websites` rows.

**Measured (M4):**
- *Setup:* two admins, website groups configured.
- *Request:* `a1` signs in with no groups.
- *Result:* `sign-in user_id=0 stored role="editor" assigned=[] siteB=true`.

**Measured (M4b):**
- *Setup:* website groups unset.
- *Result:* `sign-in user_id=1 role="editor" assigned=[] siteB=true`. This is consistent with "unset does not decide".

The identity provider operator meant to take access away. What remains is an editor on every website, through existing sessions and the password form. It is not an escalation beyond the prior admin role, hence Warning.

**Fix:** compute role and website ids first, refuse before any write, and only then call `Update` and `SetRights`, ideally in one transaction (see CR-02).

### WR-02: Identity headers with several values are believed; the first value wins

**File:** `internal/web/forwardauth.go:139-147`

**Issue:** `Header.Get` returns the first of several values, and nothing refuses the ambiguity. The strip at 153 removes both values only after they have been read.

**Measured (M6):**
- *Request:* from a trusted peer with the secret: `X-Authentik-Username: [mallory, alice]`, `X-Authentik-Email: [boss@example.com, alice@example.com]`.
- *Result:* identity `username="mallory" email="boss@example.com"`.

The shipped Caddyfile (delete + `copy_headers` set) cannot produce this. Any proxy configuration that appends (`header_up +X-authentik-…`), or a second hop that forwards a client's copy, can. Together with CR-01, the first value decides the account.

**Fix:**

```go
func one(h http.Header, name string) (string, bool) {
	v := h.Values(name)
	if len(v) > 1 {
		return "", false
	}
	if len(v) == 0 {
		return "", true
	}
	return strings.TrimSpace(v[0]), true
}
```

Believe no identity at all if any of the four headers is ambiguous.

### WR-03: The code and the docs claim the strip makes a proxy that forwards a client's copy "a misconfiguration and not a way in" — it does not

**File:** `internal/web/forwardauth.go:34-38`, `docs/security.md:164-166`, `deploy/DEPLOY.md:260-262`

**Issue:** `stripIdentityHeaders` runs on the request as it arrives *here*, after the headers have been read (139-147 before 153). It cannot tell the proxy's copy from a client's copy that the proxy passed along.

On Caddy 2.10.0–2.11.1 without the example's `request_header` deletes, the client's value arrives whenever the outpost omits a header, for example when the account has no address. It arrives from the trusted peer with the right secret, and layers 1 and 3 believe it. With CR-01 that becomes any account.

The text tells operators that layer 2 already covers exactly the case where only the Caddy version and the delete lines protect. Code order is verified; live Caddy behaviour is *unmeasured*.

**Fix:** correct all three passages. The strip protects handlers against direct clients and against header spellings. Against a proxy that forwards a client's copy, the Caddy floor and the delete lines are the only defence. In DEPLOY.md make the version check a requirement, not an explanation.

### WR-04: `via_sso` outlives SSO being switched off, and is inherited by a later password sign-in

**File:** `internal/auth/twofactor.go:90-91`, `internal/admin/twofactor.go:60-62` (read at 174, 204, 281, 414, 416), `internal/admin/login.go:80`, `internal/admin/login.go:106-110`, `internal/admin/login.go:160-161`

**Issue:** The logout reader checks `h.cfg.SSOEnabled`. `RequireSecondFactor` and every reader in `internal/admin/twofactor.go` do not.

**Measured (M5):**
- *Setup:* `cfg.SSOEnabled=false`, and a session carrying `via_sso=true` for an admin with a confirmed authenticator.
- *Request:* `POST /admin/2fa/aus`.
- *Result:* the second factor was removed (`still enabled=false`).

An operator who switches SSO off in an emergency (a compromised proxy) keeps every SSO-established admin session exempt from the second factor, and able to delete it.

The mark is also never removed. `HandleLogin` rotates with `RenewToken` (login.go:80), which in scs v2.9.0 changes only the token and deadline and keeps every value (`data.go:282-305`), and `completeLogin` removes `pending_user_id` but not `via_sso`. A session marked by SSO that then completes a password (+TOTP) sign-in as account B therefore carries the mark, and can switch off B's TOTP. *Unmeasured* end to end; code-read.

`internal/auth/session.go:23-27` ("read in exactly two") is also out of date: there are three readers.

**Fix:**
- Single-source the predicate: `h.viaSSO` returns `h.cfg != nil && h.cfg.SSOEnabled && h.sm.GetBool(...)`.
- Give `RequireSecondFactor` the switch.
- Clear the mark on every sign-in and set it only on the SSO path:

```go
func (h *Handler) completeLogin(r *http.Request, id int64, role, email string) {
	h.sm.Remove(r.Context(), auth.SessionKeyPendingUserID)
	h.sm.Remove(r.Context(), auth.SessionKeyViaSSO)
	…
}
// forwardauth.go: h.completeLogin(...) first, then h.sm.Put(..., SessionKeyViaSSO, true)
```

### WR-05: With SSO on, `HOLZCLOUD_TRUSTED_PROXIES` accepts `0.0.0.0/0` and `::/0`, and layer 1 disappears without a word

**File:** `internal/config/config.go:259-262`, `internal/config/config.go:307-332`

**Issue:** `parsePrefixes` accepts any CIDR. No check ties it to `SSOEnabled`.

**Measured (M7):**
- *Configuration:* `HOLZCLOUD_SSO_ENABLED=true`, `HOLZCLOUD_TRUSTED_PROXIES=0.0.0.0/0,::/0`, `HOLZCLOUD_LISTEN=0.0.0.0`.
- *Result:* `err=<nil>`.

Layer 1 then passes every peer, and the shared secret is the only remaining check, unthrottled (WR-06). The same setting also makes `ClientIP` believe anyone's `X-Forwarded-For`.

**Fix:**

```go
if cfg.SSOEnabled {
	for _, p := range cfg.TrustedProxies {
		if p.Bits() == 0 {
			errs = append(errs, fmt.Errorf("%s: %s trusts every address while %s is on",
				"HOLZCLOUD_TRUSTED_PROXIES", p, envSSOEnabled))
		}
	}
}
```

### WR-06: No minimum length for `HOLZCLOUD_SSO_SECRET`, and no trace of wrong secrets

**File:** `internal/config/config.go:292`, `internal/config/config.go:307-312`, `internal/web/forwardauth.go:135`, `internal/web/forwardauth.go:211-213`

**Issue:** Only emptiness is refused. M7: a 1-character secret loads (`secretLen=1`, `err=<nil>`).

A wrong secret is silently ignored: not counted, not logged. Any process that counts as a trusted peer can enumerate a short secret without leaving a trace. With the default `127.0.0.1` trust, that is every local process on the host. Constant time protects against timing, not against guessing.

**Fix:** refuse `len(cfg.SSOSecret) < 32` when SSO is on, and log a trusted-peer request that carries identity headers with a wrong secret at `Warn`, without the value.

### WR-07: `RequireFreshPassword` makes five admin actions unreachable for an account provisioned through SSO

**File:** `internal/admin/forwardauth.go:512-525`, `cmd/holzcloud/main.go:858`, `cmd/holzcloud/main.go:1017`, `cmd/holzcloud/main.go:1038`, `cmd/holzcloud/main.go:1044`, `cmd/holzcloud/main.go:1069`, `internal/admin/confirm.go:46`, `internal/admin/user.go:446-455`, `cmd/holzcloud/templates/admin/account.html:10`

**Issue:**
- **No password to confirm.** A provisioned account's password is a random value, hashed and forgotten (512, 525). The account can become an admin through the admin group (323-328).
- **Five actions become unreachable.** The five `requireFresh` routes (delete website, delete user, create AI key, remove plugin, purge activity log) send it to `/admin/bestaetigen`, which checks a password nobody knows (confirm.go:46).
- **No self-service way out.** Setting its own password needs the current one (user.go:446-455), so "Passwort ändern" on *My account* is a dead end.
- **Remaining exits:** another administrator sets a password, a reset link, or the CLI. An installation whose administrators all came through SSO has none of the five actions.

Code-read; no test covers it. There is no security effect.

**Fix:** for a session with the `via_sso` mark (WR-04), elevate by a fresh identity provider round-trip (a sign-out/sign-in through the outpost), or offer setting a first password without the current one while the account has never had a chosen password. At minimum, say so on the confirm screen and in DEPLOY.md.

### WR-08: A CMS session is not bound to the identity provider identity; the documented "takes effect at the next sign-in, not at the next session expiry" is the opposite of what happens

**File:** `internal/admin/forwardauth.go:126-136`, `internal/auth/session.go:61-62`, `deploy/DEPLOY.md:235-237`, `docs/security.md:181-183`

**Issue:**
- **The sync never re-runs during a session.** Step 3 passes any signed-in session through without comparing the incoming identity to the session. `RequireAuth` re-reads only the role. The group sync therefore runs again only when the CMS session is gone: 24 h lifetime, 4 h idle.
- **Two effects:**
  - Removing a group, or deactivating a user, at Authentik takes effect at CMS session expiry. That is exactly what DEPLOY.md:235-237 says does not happen.
  - A different person signing in to Authentik on the same browser keeps the previous person's CMS session.

Code-read; *unmeasured* by test.

**Fix:**
- When a session with the `via_sso` mark meets an identity whose address differs from `SessionKeyUserEmail`, destroy the session and continue as a fresh sign-in.
- Store a `sso_synced_at` in the session and re-run `syncRightsFromGroups` after a short interval (e.g. 15 min).
- Correct both documentation passages.

## Info

### IN-01: `subtle.ConstantTimeCompare` is held by no test

**File:** `internal/web/forwardauth.go:211-213`

**Issue:** Mutation A replaced the comparison with `r.Header.Get(ProxySecretHeader) == want`, and `go test ./internal/web/` stayed green. No behavioural test can tell the two apart; the comment is the only thing holding it.

**Fix:** an AST assertion alongside `TestForwardAuthChecksThePeerBeforeItReadsAHeader`, requiring a `subtle.ConstantTimeCompare` call in `secretMatches`.

### IN-02: ForwardAuth's position outside RequestID/AccessLog is held by no test

**File:** `cmd/holzcloud/main.go:1224-1244`

**Issue:** Mutation B moved `web.ForwardAuth` inside `AccessLog` and `RequestID`, and `go test ./cmd/holzcloud/` stayed green.

Today this changes nothing: `AccessLog` logs no header (`internal/web/logging.go:141-150`), and `RequestID` reads only `X-Request-ID` (`logging.go:35-36`). The comment at 1224-1227 is the only thing holding the order, and it becomes load-bearing the day either middleware logs a header.

**Fix:** an AST assertion in `main_test.go` that `web.ForwardAuth(...)` is the last assignment to `handler` in `newRouter`.

### IN-03: The logout response branches on `HX-Request` without `Vary: HX-Request`

**File:** `internal/admin/login.go:190`, `internal/admin/page.go:1003-1011`

**Issue:** `h.redirect` answers with `HX-Redirect` or with a 303 depending on `HX-Request`, and sets no `Vary`. That breaks the CLAUDE.md convention. The only setters are `internal/web/render.go:247,264`, and no test in `login_test.go` or `main_test.go` asserts `Vary`. It is practically harmless: a POST answered with `no-store`.

**Fix:** `w.Header().Add("Vary", "HX-Request")` in `redirect` (and in `redirectBack`, user.go:498-505).

### IN-04: The duplicate-address branch of provisioning can hand out an account that does not yet have its assignment

**File:** `internal/admin/forwardauth.go:526-538`

**Issue:** When two requests for the same new identity race, the loser returns the winner's row. The winner may still be between `Create` (store.go:209) and `SetRights` (forwardauth.go:547). The loser then signs in to an account with zero rows, which contradicts the guarantee at 209-213. The window is small (both requests hash Argon2 before inserting). *Unmeasured.*

**Fix:** in the duplicate branch, read `Rights` and refuse while an editor has no rows.

### IN-05: `holzcloud user 2fa disable` restates the second-factor rule as a second predicate

**File:** `cmd/holzcloud/cli.go:449-455` (outside the listed scope)

**Issue:** `if u.Role == "admin"` prints "it will be asked to set up a new authenticator immediately". `internal/auth/twofactor.go:56-59` says there is no second predicate. The message is true on the password path and false for an administrator who signs in through SSO.

**Fix:** `auth.MustHaveSecondFactor(u.Role, false)`, and word it as "when signing in with a password".

### IN-06: Signing out a password session while SSO is on signs the same person straight back in

**File:** `internal/admin/login.go:159-175`

**Issue:** Only sessions with the `via_sso` mark are sent to the outpost's sign-out. A password session behind the SSO proxy is sent to `/admin/login`, and the next `/admin/` request carries the outpost's headers, so `ForwardAuthSignIn` signs the person in again. It is the "sign-out that signs nobody out" the comment describes, reached from the other side. Code-read, *unmeasured*.

**Fix:** also send the session to `SSOSignOutPath` when `cfg.SSOEnabled` and the logout request itself carried an identity (`web.IdentityFromContext`).

---

_Reviewed: 2026-09-10T13:15:04Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_


## Stand der Behebung

Nachgetragen, nicht vom Prüfer. Jede geschlossene Zeile hat einen Rotbeweis, der
vor dem Flick eingecheckt wurde, und eine Mutationsprobe danach.

| ID | Stand | Rotbeweis | Flick |
|---|---|---|---|
| CR-01 | **offen** | — | — |
| CR-02 | Kern geschlossen: `SetRights` transaktional, und eine leere Zuordnung eines begrenzten Kontos heisst keine Website. **Offen:** die Gruppen-Website-IDs werden nicht gegen die Tabelle geprüft. | `88bab2e` | `e724cdd` |
| CR-03 | geschlossen | `0536f96` | `e724cdd` |
| WR-01 | geschlossen | `88bab2e` | `e391023` |
| WR-02 | geschlossen | `5a5e753` | `0c7b15b` |
| WR-03 | **offen** | — | — |
| WR-04 | **offen** | — | — |
| WR-05 | **offen** | — | — |
| WR-06 | **offen** | — | — |
| WR-07 | **offen** | — | — |
| WR-08 | **offen** | — | — |
| IN-01 bis IN-06 | **offen** | — | — |
