# Context: Phase 15 — What the server promises, it keeps

Opened 2026-09-14.

---

## §1 Why this phase exists, and how it started

The ledger (`.planning/WINDOWS.md`) has carried **fifteen open entries** since
v1.10 and v2.0. Two milestones have closed over the top of them. That is long
enough: a register nobody works is a register nobody reads.

The plan was to work the fifteen. The first hour did something else. Window 24
says only that admin answers carry `Vary: Cookie` twice — trivial, harmless, a
line of bookkeeping. Chasing *why* led to `internal/public/handler.go`:

```go
w.Header().Set("Cache-Control", "public, max-age=300")
w.Header().Set("Vary", "HX-Request")
```

`Set` replaces. The session middleware names `Cookie` on every request, at the
top of `LoadAndSave`; this line deleted it. And `serveCached` is what serves a
**password-protected page** once the visitor has entered the password.

So: a protected page went out as `public, max-age=300`, with a `Vary` that no
longer mentioned the cookie the access depends on. Any shared cache in front of
the binary — a CDN, a company proxy, a Varnish — could hold it for five minutes
and hand it to somebody who never entered the password.

`access.go` had already written the danger down, in the comment above the gate
it guards correctly:

> Never cached: a shared proxy holding this would hand the form to someone who
> has already unlocked, **or worse, the page to someone who has not.**

The form was guarded. The page was not.

**That is the shape of this phase and the reason for its name.** Every item here
is a place where what the program says and what it does have come apart.

## §2 Three answers, not one

The fix is not "add Cookie to Vary". That would be the safe-looking change and
it would be wrong: with `Cookie` in the list, a cache keyed on cookies stores
one copy per visitor, so declaring it on every page makes the five-minute cache
worthless for the whole site. The honest shape is three kinds of answer, each
saying what it is:

| | Cache-Control | Vary | for |
|---|---|---|---|
| `serveCached` | `public, max-age=300` | `HX-Request` | the same for everybody |
| `servePersonal` | `private, max-age=300` | `Cookie, HX-Request` | prices that follow the visitor's price mode |
| `servePrivate` | `no-store, private` | `Cookie, HX-Request` | a page behind a password |

`serveCached` still *replaces* `Vary`, and now says in its comment that this is a
**claim about the content** rather than a convenience. An answer that is not the
same for everybody must take one of the other two doors. Before this, there were
no other doors, so everything rode along.

A shop is only personal when it offers **both** price modes — with one mode the
figures are the same for everybody and the catalogue stays shareable. `Display`
decides, in `servePricedFor`.

## §3 web.AddVary, and window 24 closing itself

Six places wrote `Set("Vary", …)`. `web.AddVary` keeps what is there, folds
duplicates and leaves `*` alone. It is also, without being aimed at it, the fix
for window 24: the duplicate `Vary: Cookie` came from the session library and
the CSRF library each adding it — both correct, neither knowing about the other
— and any answer that now passes through `AddVary` says it once.

Measured on the running binary before and after:

```
before  Vary: Cookie
        Vary: Cookie
after   Vary: Cookie, HX-Request
```

## §4 What is deliberately NOT changed

`serveCached` keeps `public, max-age=300` for ordinary pages. They carry no
shop data (`shopData` is called only from the shop, cart and checkout handlers,
whatever its own comment claims) and nothing else on them follows a cookie. Had
the blunt fix been taken, every page of every site would have become
uncacheable, which is a real cost paid for nothing.

---

## §5 The forward-auth cluster, read once

Eight of the fifteen entries named `internal/admin/forwardauth.go`. The plan
said to read that subsystem once rather than eight times, and that was right:
five of them turned out to be the same sentence said five ways — *the protocol
cannot answer the question an operator brings to it.*

| window | what it was | what it is |
|---|---|---|
| 26 | a refused identity wrote a row on **every** request | one row per identity and reason per 15 minutes |
| 28a | the row did not say **why** | `via` and `reason` in the metadata |
| 28b | a failed session rotation refused with **no row at all** | `session_renew_failed`, through the same path |
| 28c | the sign-out row could not say **which way** | `by_hand` plus `via: password`/`sso` |
| 22 | the provisioning row carried **user_id NULL** | it names the account it created |

**Window 26 is the one with teeth, and its cure is constrained.** The obvious
answer — feed the sign-in brake — is barred by T-10-20: a proxy asserting a
wrong identity would then lock out the person whose address it names. So the
brake sits on the **writing**. Every request is still refused, still logged to
the file that rotates, and still costs the visitor nothing but a password form.
The bound on the map matters too: the key is an identity the *proxy* chose, so
a brake against an unbounded table must not itself be an unbounded map.

**Window 20 was a sign-out button that signed nobody out.** The middleware
leaves a password session alone while it lives — but the button has just
destroyed it, so the next click arrives with no session, the outpost's cookie is
still in the browser, and the person is signed straight back in. The question is
not how the session began; it is whether the identity provider will simply make
another one.

**Window 21 was never about the behaviour.** Re-deriving a website assignment
from the directory's groups on every request is SSO-06 working exactly as
designed. What the entry actually reports is *"kein Bildschirm sagt das"* — an
operator watching their own change undo itself with nothing to read. So the
change is one sentence on the user form, shown when three things hold together.

## §6 What stays open, and why that is not the same as unfinished

**Windows 19 and 27 are waived, and their reasons now stand at the code** rather
than only in the ledger — above `RequireFreshPassword` and above
`provisionSSOUser`. Both need a conversation with the identity provider that
nothing in this repository can exercise: there is no directory in the test
environment. Window 19's entry also records what must **not** be done, because
it is the tempting shortcut: exempting a single sign-on session from the
password confirmation would take the one question standing in front of the
irreversible and remove it for exactly the accounts no operator created by hand.

**Windows 4, 7, 9, 11 and 12 are waived together.** All five are miscounting
gates inside *executed and archived* plan documents, and in every case the
property the gate was meant to protect held. Correcting an archived plan would
rewrite a report rather than correct one — the same rule under which a migration
that has run is never edited, and under which Phase 11 stayed in the milestone
that built it. Each entry already carries the corrected form of its gate, and
the lesson is where it does work: `RETROSPECTIVE.md` records it under *What Was
Inefficient* and again as the first of the *Key Lessons* — "A gate is a claim
about what it reads."

## §7 Where the ledger stands

Fifteen open at the milestone's start; **one** at the end of this pass, and that
one — window 34 — was opened during it. It is the album gallery answering in the
website's language rather than the page's, which is PUB-01's principle one level
down, and closing it means making the inline gallery's frozen words late too.
That is a wave of its own.

A ledger worked to one entry is worth something only if its own numbers are
true. They were not: the markdown table and the JSON block beside it had drifted
by four rows, and one row carried an unescaped `|` that made every parser drop
it. Both repaired in §3's commit, with the file now saying which of the two is
the record.

## §8 One fix had to be sharpened, by a test that was right

The first shape of window 20's cure asked whether the ACCOUNT leaving was
linked to an identity. `TestLogoutOfAPasswordSessionIsExactlyWhatItWas` went
red, and it was right to: it encodes **SSO-09** — *"nothing about the password
path changed; it is the session that decides, not the installation"*.

That rule is not bureaucracy, it is the fallback. When the proxy is broken an
operator signs in with a password to go and fix it — and on most installations
their account IS linked, so the first shape sent them to an address served by
the thing that is broken.

The question is therefore asked of the **request in hand**, not of the account:
an identity in the context means the proxy is alive and vouching for somebody
right now, and that identity resolving to a linked account means the next click
really does sign somebody in. Both true, and the diversion is the only reading
under which the button tells the truth. Either false — proxy down, nobody
asserted, nothing linked — and the password path is byte for byte what it was.

The new test drives the identity through the real `web.ForwardAuth` middleware
rather than writing it into a context by hand, so the case it protects is the
one a browser produces.

Worth writing down because it is the second time in this phase that the
conservative-looking change was the wrong one: **KEEP-01's** blunt fix would
have made every page of every site uncacheable, and this one would have broken
the fallback path. Both times the honest cure was narrower, not broader.
