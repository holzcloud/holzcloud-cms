# Cross-website write in the product save handler — proof, fix, and shop audit

**Date:** 2026-09-07
**Debug session:** `.planning/debug/product-cross-website-write.md`
**Commits:** `071bead` (proof), `2e43bc1` (fix), `fb9c76a` (second proof), `25c134e` (second fix)

---

## 1. The claim, re-verified

All three legs of the report were confirmed by reading before anything was run,
and then confirmed again by execution.

| Leg | Verdict | Evidence |
|---|---|---|
| `handleProductSave` never checks the product's website | **confirmed** | `product.go:214-289` contains no `Get` and no `WebsiteID` comparison. Its siblings at `:164` (GET arm) and `:314` (delete) contain both. |
| `shop.Store.Update` cannot compensate | **confirmed** | `UPDATE products SET … WHERE id=$17`. `website_id` was in neither the `SET` list nor the `WHERE` clause. |
| The routes admit an editor | **confirmed** | `main.go:868-872` use plain `HandleFunc`; the `/shop` settings routes on lines `878-879` immediately below use `requireAdmin(...)`. |
| `term.SetForProduct` takes the categories with it | **confirmed, and worse than reported** | see §2. |

The AND-gate fired: the write needs all three conditions at once. Closing any one
stops it, which is the argument for closing the one that protects future callers
— the store.

## 2. The proof

`internal/admin/product_scope_test.go`, committed **failing** in `071bead`.
Literal output against the unfixed tree:

```
--- FAIL: TestProductSaveRefusesAForeignWebsitesProduct (0.56s)
    product_scope_test.go:184: save: status 303, want 404 or 403 — the write reached a foreign website
    product_scope_test.go:188: website B's product was retitled from "Fremder Tisch" to "Übernommen"
    product_scope_test.go:191: website B's product was repriced from 2490.00 to 0.05
    product_scope_test.go:195: website B's product was moved from "fremder-tisch" to "uebernommen"
--- FAIL: TestProductSaveDoesNotTouchAForeignWebsitesCategories (0.52s)
    product_scope_test.go:223: save: status 303, want 404 or 403 — the write reached a foreign website
    product_scope_test.go:230: website B's product went from 2 categories to 1
--- FAIL: TestProductSaveRefusesAForeignProductIdEvenForAnAdmin (0.52s)
    product_scope_test.go:253: save with a mismatched product id: status 303, want 404
    product_scope_test.go:258: website B's product was rewritten through website A's address: "Übernommen" at 0.05
```

The request went through the real chain — session, `auth.RequireWebsiteAccess`,
routed mux — as an editor whose `user_websites` grants website A only. The
fixture asserts up front that the guard admits that editor on A and refuses them
on B, so the test cannot pass for the wrong reason.

**The category line is the one that matters most, and it is worse than the
report said.** "2 categories to 1" is not erasure. `setProductTerms` is called
with `ws.ID` = website A alongside website B's product id, so the unscoped
`DELETE` cleared B's two labels and the `INSERT` attached a term belonging to
**A**. The result is a `product_terms` row whose two halves belong to different
websites — a corruption that **no code fix undoes**. The CHANGELOG therefore
carries a query for operators to run:

```sql
SELECT pt.product_id, pt.term_id FROM product_terms pt
  JOIN products p ON p.id = pt.product_id
  JOIN terms t ON t.id = pt.term_id
 WHERE p.website_id <> t.website_id;
```

Three control tests passed on that same run (save on the own website, create via
the `neu` sentinel, delete). Without them a fix that refuses everything would
have looked correct.

## 3. Where the check went, and why

**In the store — `internal/shop/product.go`.** `Update` and `Delete` now take
`websiteID` and carry `AND website_id = ?` in the `WHERE` clause, returning the
new `shop.ErrNotFound` when `RowsAffected() == 0`. The handler maps that to 404,
matching what the edit screen beside it already answers for the same
disagreement.

Three reasons, in order of weight:

1. **The compiler enumerates the callers.** The signature change named both
   production sites immediately — `product.go:279` and `:318`. `internal/menu`
   could not do this: `menu_items` has no `website_id` column, so its fix stayed
   in the handler with a comment on the store saying it enforces nothing. That
   comment is a note to a reader; a signature is a note to the build. `products`
   has the column, so this one does not have to settle.
2. **One statement cannot be raced.** `Get` → compare → `Update` has a window
   between the check and the write. `WHERE id = ? AND website_id = ?` has none.
3. **It is this codebase's dominant pattern already** — `internal/term`'s
   `Delete`/`Rename`, `internal/kind`, `internal/block`, `internal/field`, and
   `OrderStore.SetStatus`.

`RowsAffected() == 0` raises an error rather than passing silently, because
silence would let the handler flash "Produkt gespeichert" over a write that
never happened. `ErrNotFound` deliberately does not distinguish "no such
product" from "not your product": telling them apart would confirm that an id
names something real elsewhere.

`term.SetForProduct` was hardened too — the `DELETE` goes through a subquery on
`products` and the `INSERT` joins `products` to `terms` on the shared website,
so the cross-tenant row observed in §2 cannot be written at all. `SetForPage`
has the identical shape; all five of its callers create the page they then
label, so it was left alone and the code comment records that decision.

**Verification.** Every new guard was mutation-checked. Removing
`AND website_id=$18` from `Update`, `AND website_id = $2` from `Delete`, or the
subquery from `SetForProduct`'s `DELETE` turns the corresponding tests red. The
proof test file was **not touched** by the fix commit — `git show --stat 2e43bc1`
does not list it.

## 4. Second bug found by the audit — and fixed

`internal/admin/order.go` `HandleOrderDetail`. Three ids meet on the order
screen; only two were checked.

| id | source | scoped? |
|---|---|---|
| website | address | yes, by `RequireWebsiteAccess` |
| order number | address | yes, `ByNumber(ctx, ws.ID, …)` |
| **`mail_id`** | **posted form** | **no** |

`retryMail` passed it straight to `outbox.Store.Retry`, whose statement was
`UPDATE outbox … WHERE id = ? AND status <> 'sent'` — no website, although
`outbox.website_id` is `NOT NULL` (migration `00042:15`).

Proven in `fb9c76a`, `internal/admin/order_scope_test.go`:

```
--- FAIL: TestOrderRetryMailRefusesAForeignWebsitesMessage (0.54s)
    order_scope_test.go:164: website B's message was put back in the queue: status "pending", want "failed"
    order_scope_test.go:168: website B's attempt counter was reset from 5 to 0
    order_scope_test.go:171: website B's error text was cleared, so its operator can no longer see what went wrong
```

**This one is a delivery, not a defacement.** The other website's customer
receives their mail a second time, and the failure their operator was about to
investigate has had its error text and attempt counter wiped. Fixed in `25c134e`
by the same method: `Retry` takes the website, the compiler named its one
caller, mutation-checked.

## 5. Full audit

Remit: every handler and every store method in `internal/shop` (products,
orders, carts, payments), plus `term.SetForProduct`. Criterion: a second
resource id taken from a path or a form and written without being scoped to the
website.

### Broken (2 found, 2 fixed)

| Site | Shape | Status |
|---|---|---|
| `admin/product.go` `handleProductSave` → `shop.Store.Update` | `productID` from path, written unscoped | **fixed** `2e43bc1` |
| `admin/order.go` `retryMail` → `outbox.Store.Retry` | `mail_id` from form, written unscoped | **fixed** `25c134e` |

Plus `term.SetForProduct`, hardened in the same pass — reachable only through
the first of those, but it had written the cross-tenant row.

### Clean — `internal/shop` store methods

| Method | Why clean |
|---|---|
| `Store.Create` | `website_id` comes from the caller's authorised website and is written, not matched. |
| `Store.Get` | Id-only **read**. Both admin callers compare `WebsiteID` immediately; the cart caller is fed an id already proven by `GetPublished`. |
| `Store.List`, `ListPublished`, `CountPublished` | `WHERE website_id = ?`. |
| `Store.GetPublished` | `WHERE website_id = ? AND slug = ? AND status='published'` — the safe entry point the public side uses. |
| `Store.ListPublishedByTerm`, `CountPublishedByTerm` | `p.website_id = ?` in the join. |
| `Store.AdjustStock` | Id-only write, but the only caller is `OrderStore.Place` over a cart built by `GetPublished`. Clean **by construction**, not structurally. |
| `Store.SetTerms`, `Store.SetGallery` | Id-only writes on `product_terms` / `product_media`. **No production caller** — test-only today. Left unchanged deliberately; noted below. |
| `Store.TermIDs`, `Store.GalleryIDs` | Id-only **reads** of a product's own join rows. |
| `CartStore.Ensure`, `Get`, `byToken` | `WHERE website_id = ?`. |
| `CartStore.Add`, `SetQuantity`, `Remove`, `Clear`, `touch` | Keyed on `cartID`, which came from `Ensure`/`Get` scoped to the website; the `productID` beside it came from `GetPublished`. |
| `CartStore.items` | Read, keyed on an already-scoped cart. |
| `CartStore.Sweep` | Housekeeping across all websites by age, by design. |
| `OrderStore.Place` | Takes `websiteID` and writes it. Its stock `UPDATE` is id-only but iterates a cart built by `GetPublished`. |
| `OrderStore.ByNumber`, `List` | `WHERE website_id = ?`. |
| `OrderStore.itemsOf` | Read, keyed on an order already fetched by `ByNumber`. |
| `OrderStore.SetStatus` | Already `WHERE id = ? AND website_id = ?` — the target pattern, pre-existing. |
| `OrderStore.SetPayment` | Id-only write, but all five callers pass an order from `ByNumber(ctx, website.ID, …)`. Clean **by construction**. |

### Clean — handlers

| Handler | Why clean |
|---|---|
| `admin` `HandleProductList` | One id. `List(ctx, ws.ID)`. |
| `admin` `HandleProductForm` (GET arm) | `Get` then `p.WebsiteID != ws.ID` → 404. |
| `admin` `HandleProductDelete` | Same check, and now scoped in the store as well. Re-proved by a test rather than left to reading. |
| `admin` `HandleOrderList` | One id. |
| `admin` `HandleOrderDetail` — order lookup and `SetStatus` | `ByNumber(ctx, ws.ID, …)`; `SetStatus` scoped. (Its `mail_id` arm was the bug in §4.) |
| `admin` `HandleOrderDocument` | `ByNumber` scoped; `kind` validated against a two-value whitelist. |
| `admin` `recheckPayment` | Uses `order.PaymentReference` off the already-scoped order, never a form value. Amount is verified against the order before settling. |
| `admin` `announceShipment` | Builds from the scoped order. |
| `admin` `HandleShopSettings` | Settings live on the website row itself — only one id exists. |
| `public` `HandleCartAdd` | Resolves the article by **slug within the website**. |
| `public` `HandleCartUpdate` / `HandleCartRemove` / `cartLineChange` | Same, and it already carried the comment: *"A numeric id from the form would let a guessed number reach another site's catalogue."* |
| `public` `HandlePaymentReturn` | `ByNumber(ctx, website.ID, …)`; a guessed number causes one API call and leaks nothing. |
| `public` `HandlePaymentHook` | Reads only the gateway id from the body, then asks Payrexx directly; the order is resolved by `ByNumber` scoped to the website; amount **and** currency verified before settling. |
| `payrexx.Client` (all methods) | An outbound HTTP client with no database and no website concept. |

**The public shop side is not clean by luck.** It was written with exactly this
threat in mind and says so in a comment that predates this investigation. The
same author avoided the trap on the visitor-facing side and fell into it on the
admin side — worth knowing, because it says the gap is in the admin review
habit, not in the understanding.

## 6. Two things named but deliberately not changed

**1. `featured_media_id` is never validated against the website.** The product
form (`product.go:236`) and the page form (`page_form.go:128`) both take a media
id from the form and store it without checking which website owns that media.
So website A's product can display website B's picture.

Not fixed, because it is not an oversight: `HandleMediaServe`
(`admin/media.go:326`) documents the decision explicitly — *"Host-to-site
enforcement stays deliberately out: it would break the admin preview and
legitimate cross-site reuse of the same file."* Media at `/media/{websiteID}/…`
is already fetchable for any active website, so the reference discloses nothing
new.

**It does sit in tension with the project's stated isolation principle** (all
resources scoped to exactly one website, no sharing). That is a design decision
for the developer, not something to change inside a security fix — but it should
be decided rather than inherited.

**2. `shop.Store.SetTerms` and `SetGallery` remain id-only.** Both have no
production caller today. Changing a dead method's signature adds diff without
adding safety; this entry is the durable record so that whoever wires them up
scopes them first.

## 7. Gates

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | empty |
| `go test ./...` | **exit 0** |
| `go run ./tools/i18n` | `29 offen`, **`0 verwaist`** in every catalogue — byte-identical to the baseline at `071bead`, so these changes added no catalogue strings |

`offen` at 29 is the pre-existing state (phase 11 is mid-flight; plan 11-07 owns
the catalogues), verified by running the tool against a worktree at the proof
commit and getting the same 1306 / 29 / 0.

## 8. Why it was not caught, and what stops it returning

**Why not caught.** No gate existed for this class. `go vet` cannot see a missing
`WHERE` clause, and no test drove an admin write handler as a user restricted to
a *different* website — every existing admin test used one website. The menu fix
on 2026-09-06 introduced the first such test; this is the second, and the
pattern is now established.

**Recurrence guards, all concrete:**

- `internal/admin/product_scope_test.go` — 6 tests through the real middleware
- `internal/shop/product_test.go` — 4 store-level tests, including the
  unknown-id boundary neighbour
- `internal/term/store_test.go` — 2 tests on `SetForProduct`
- `internal/admin/order_scope_test.go` — 2 tests on the retry path
- `internal/outbox/outbox_test.go` — 1 store-level test
- **The signature change itself** — `Update`, `Delete` and `Retry` can no longer
  be called without naming a website. This is the guard that does not depend on
  anyone remembering to write a test.

**The standing recommendation:** whenever an admin route takes a website id and
a second resource id, put the scope in the store's `WHERE` clause and take the
website as a parameter. When the second resource's table has no `website_id`
column (`menu_items`, `page_terms`, `product_media`), that is the signal that
the check has to live in the handler — and that it needs a `*_scope_test.go` to
hold it there.
