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
