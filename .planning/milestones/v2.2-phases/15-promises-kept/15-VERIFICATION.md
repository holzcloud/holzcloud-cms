# Verification: Phase 15 — What the server promises, it keeps

Measured 2026-09-14 against `e9db77d`, by running the gates, by reverting each
fix to see its test go red, and by driving a binary at `127.0.0.1:8080`.

**420 Go files, 119 305 lines, 1341 test functions, 1825 catalogue entries in
each of de, es, fr and it.** Eight commits since the v2.1 close; 39 files
changed, +1 635 / −261.

---

## KEEP-01 — a protected page is never cacheable by a shared proxy

**Met.** Measured before the work, on `32d545d`: `serveCached` wrote `Vary` with
`Set`, deleting the `Vary: Cookie` the session middleware adds, and answered
`Cache-Control: public, max-age=300`. A password-protected page, once unlocked,
went out under those headers — so a reverse proxy or a CDN in front of the
binary could hold it for five minutes and hand it to anybody who asked for the
address.

`access.go` had already written that danger down, above the gate it guards
correctly:

> Never cached: a shared proxy holding this would hand the form to someone who
> has already unlocked, **or worse, the page to someone who has not.**

The form was guarded. The page behind it was not.

Three answers now, each saying what it is:

| | Cache-Control | Vary | for |
|---|---|---|---|
| `serveCached` | `public, max-age=300` | `HX-Request` | the same for everybody |
| `servePersonal` | `private, max-age=300` | `Cookie, HX-Request` | prices that follow a cookie |
| `servePrivate` | `no-store, private` | `Cookie, HX-Request` | behind a password |

Live, on the running binary:

```
locked     401  Cache-Control: no-store, private   Vary: Cookie
unlocked   200  Cache-Control: no-store, private   Vary: Cookie, HX-Request
           ("Netto-Preisliste" present in the body)
ordinary   200  Cache-Control: public, max-age=300 Vary: HX-Request
```

Held by `TestAnUnlockedPageIsNotHandedToAProxy`; red without `servePrivate`.

## KEEP-02 — an answer that depends on a cookie says so

**Met.** The same `Set` gave a shop offering both price modes `public` caching
for figures that follow the audience cookie, so trade prices could reach a
consumer. `servePricedFor` decides by `Settings.Display`: with **one** price
mode the figures are the same for everybody and the catalogue stays shareable.

That second half is asserted too, and it is the point — the blunt fix, adding
`Cookie` to every answer, would have made every page of every site uncacheable
in exchange for nothing. Held by `TestShopPricesThatDependOnACookieAreNotShared`
with all three display modes; red without `servePersonal`.

## KEEP-03 — `docs/security.md` says what is true

**Met.** The protected-page section carries the cache rule and says plainly what
was wrong before it.

## WIN-01 — the eight forward-auth windows

**Met.** Five of the eight were one sentence said five ways: the protocol cannot
answer the question an operator brings to it.

| window | was | is |
|---|---|---|
| 26 | a row on **every** request | one per identity and reason per 15 minutes |
| 28a | the row did not say why | `via` and `reason` in the metadata |
| 28b | a failed rotation refused with **no row** | `session_renew_failed` |
| 28c | the sign-out row could not say which way | `by_hand`, `via: password`/`sso` |
| 22 | the provisioning row carried `user_id` NULL | it names the account it created |
| 20 | a sign-out button that signed nobody out | it reaches the outpost when the proxy is vouching |
| 21 | no screen said where the rights come from | the user form says it |

Live, with single sign-on configured:

```
12 refused requests  -> 1 protocol row, {"reason":"no_account","via":"sso"}
sign-out, linked     -> 303 /outpost.goauthentik.io/sign_out
                        row: user_id 1, entity_id 1,
                        {"reason":"by_hand","via":"password"}
user form            -> "This account signs in through the identity provider,
                        and its websites are re-derived from its groups there
                        on every request."
```

**19 and 27 are waived**, with their reasons moved from the ledger to the code —
above `RequireFreshPassword` and above `provisionSSOUser`. Both need a
conversation with an identity provider that nothing in this repository can
exercise. Window 19's note also records what must **not** be done, because it is
the tempting shortcut: exempting a single sign-on session from the password
confirmation would take the one question standing in front of the irreversible
and remove it for exactly the accounts no operator created by hand.

## WIN-02 — the five plan-document windows

**Met, by waiver.** Entries 4, 7, 9, 11 and 12 are miscounting gates inside
*executed and archived* plan documents, and in every case the property the gate
was meant to protect held. Correcting an archived plan rewrites a report rather
than correcting one — the same rule under which a migration that has run is
never edited. Each entry already carries the corrected form of its gate, and the
lesson is where it does work: `RETROSPECTIVE.md` under *What Was Inefficient*
and again as the first of the *Key Lessons*.

## WIN-03 — window 8

**Met.** An album gallery's wrapper was written at save, in whatever language
the website had then, while the controls inside the same region resolve at
delivery. Change a site from German to Spanish and it answers "Imagen siguiente"
beside `aria-label="Galerie"` until every page carrying a gallery is saved again.

`block.GalleryWrapper` is now the one writer for both sources, and the album
marker carries the wrapper's columns and display class as an **optional** tail —
so `[[album:slug:at]]` still parses, its wrapper is still in the stored HTML,
and its expansion still returns the tiles alone. No migration, no broken page,
and a page heals on its next save.

Live: a page carrying the long marker renders
`role="region" aria-label="Galerie"` with `Nächstes Bild` beside it, from one
translator, and no raw marker left on the page.

## WIN-04 — window 23

**Met.** `writeSwiss` folded the whole existing file back in to protect the
hand-written Swiss deviations and protected the orphans with them. It now keeps
only what the source still has and prints every key it lets go. Proven on the
real catalogue: orphan planted, first run names and removes it, second run
reports `0 let go`, and the file is byte-identical to what it was.

## WIN-05 — window 24

**Met, as a side effect.** The duplicate `Vary: Cookie` came from the session
library and the CSRF library each adding it — both correctly, neither knowing
about the other. `web.AddVary` folds duplicates. Before and after, on the
running binary:

```
before  Vary: Cookie
        Vary: Cookie
after   Vary: Cookie, HX-Request
```

## WIN-06 — the ledger's front matter is true

**Met, and the ledger needed repairing to make it so.** Its markdown table and
its JSON block had drifted: entries 6, 14, 15 and 16 were closed in the table in
v2.0 and left open in the JSON, and entry 8 carried an unescaped `|` inside a
cell, so it rendered with an extra column and every parser dropped it. The table
is the record and the block now derives from it; the file says so.

Asserted rather than eyeballed at the close:

```
json: {'open': 1, 'fixed': 24, 'waived': 9} total 34
front matter: open_count 1, fixed_count 24, waived_count 9, total_count 34
WIN-06 holds: counts agree, every waiver has a reason, every fix has a date
```

**Fifteen open at the milestone's start; one at the end**, and that one —
window 34 — was opened during this work.

## QUAL-01 — the gates

```
no German in the Go source outside the catalogues
de.json      1825 translated, 0 open, 0 orphaned
es.json      1825 translated, 0 open, 0 orphaned
fr.json      1825 translated, 0 open, 0 orphaned
it.json      1825 translated, 0 open, 0 orphaned
every shipped theme's catalogue matches tools/themewords/words.json
gofmt: clean   go vet: clean   go test ./...: clean
```

The suite also ran on a GitHub runner at v2.0's commit, green, as a by-product
of the release attempt.

## QUAL-02 — driven in a browser

**Met.** The headers on every kind of public answer; an album gallery for its
region name; the user form with single sign-on configured; the sign-out of a
linked password session; twelve refused identities in a row.

## What this phase found that it did not plan for

- **Two defects nobody had recorded**, both reached by chasing an entry that
  looked trivial. Window 24 says only that a header appears twice.
- **Twice the conservative-looking change was the wrong one.** KEEP-01's blunt
  fix would have made every page uncacheable; window 20's first shape broke
  SSO-09's fallback, which is the path an operator takes when the proxy is
  broken. Both times the honest cure was narrower, not broader, and the second
  was forced by an existing test that was right.
- **One finding that was mine to correct twice.** The 403 on
  `git push origin refs/tags/v2.0` was recorded for two milestones as "this
  credential cannot write tag refs". It is nothing of the kind: until
  2026-09-11 this environment worked under the operator's own account, and
  since `5a46c18` it is an App integration — and an App may not create a tag ref
  whose tree carries workflow files. Written up in `STATE.md` and in
  `release.yml`, with what was measured kept apart from what is merely
  documented behaviour.
