# GSD Debug Knowledge Base

Resolved debug sessions. Used by `gsd-debugger` to surface known-pattern
hypotheses at the start of new investigations.

---

## menu-item-cross-website-write — an editor could rewrite another website's navigation
- **Date:** 2026-09-06
- **Error patterns:** cross-website, cross-tenant, website isolation, RequireWebsiteAccess, second path id, menuID, itemID, 303 instead of 404, silent write
- **Root cause(s):** The four menu-ITEM handlers authorised on the websiteID taken from the address but never verified that the menu named by menuID belonged to that website; they compared item.MenuID != menuID only. auth.RequireWebsiteAccess reads only the leading /admin/websites/<number> by documented design and cannot compensate.
- **Fix:** Two helpers in internal/admin/menu.go — menuOfWebsite and itemOfWebsite — with all seven menu handlers routed through them. Handler level rather than store level because menu_items has no website_id column.
- **Files changed:** internal/admin/menu_scope_test.go, internal/admin/menu.go, internal/menu/store.go
- **Why not caught:** No gate existed for this class. No test drove an admin write handler as a user restricted to a *different* website — every admin test used one website.
- **Recurrence guard:** Regression test internal/admin/menu_scope_test.go (6 tests through the real middleware chain). The store still enforces nothing and carries a comment saying so — a weaker guard than a signature, and the reason the product fix went into the store instead.
---

## product-cross-website-write — an editor could rewrite and reprice another website's product
- **Date:** 2026-09-07
- **Error patterns:** cross-website, cross-tenant, website isolation, RequireWebsiteAccess, second path id, productID, unscoped UPDATE, WHERE id only, 303 instead of 404, silent write, repriced, product_terms cross-tenant row
- **Root cause(s):** internal/admin/product.go handleProductSave took productID from the address and passed it to shop.Store.Update without checking ownership; shop.Store.Update's WHERE named only the id so it could not catch the omission; and the produkte routes are registered without requireAdmin, so editors reach them. All three were required simultaneously (AND-gate). Separately, term.SetForProduct's unscoped DELETE FROM product_terms wrote a join row whose two halves belonged to different websites.
- **Fix:** shop.Store.Update and Delete take websiteID and carry AND website_id in the WHERE clause, returning shop.ErrNotFound on RowsAffected()==0; the handler maps that to 404 before setProductTerms runs. term.SetForProduct scopes its DELETE by subquery and joins products to terms on the shared website. The signature change made the compiler enumerate the callers.
- **Files changed:** internal/admin/product_scope_test.go, internal/shop/product.go, internal/shop/product_test.go, internal/admin/product.go, internal/term/store.go, internal/term/store_test.go, internal/shop/order_test.go, internal/shop/cart_test.go, CHANGELOG.md
- **Why not caught:** No gate existed for this class — go vet cannot see a missing WHERE clause, and the menu-scope test written the day before covered only menus. The sibling handlers in the same file were correct, which made the file look reviewed.
- **Recurrence guard:** internal/admin/product_scope_test.go (6 tests), internal/shop/product_test.go (4 store-level tests incl. the unknown-id boundary), internal/term/store_test.go (2 tests) — and above all the changed signature: Update and Delete can no longer be called without naming a website, which does not depend on anyone remembering to write a test. CHANGELOG carries a SQL query for operators to find cross-tenant product_terms rows, which no code fix repairs.
---

## order-retry-mail-cross-website-write — an editor could re-send another website's customer email
- **Date:** 2026-09-07
- **Error patterns:** cross-website, cross-tenant, website isolation, third id from a form, mail_id, outbox, unscoped UPDATE, WHERE id only, attempt counter reset, last_error cleared, duplicate delivery
- **Root cause(s):** internal/admin/order.go HandleOrderDetail scopes the website and the order correctly but passes mail_id straight from the posted form to outbox.Store.Retry, whose statement was UPDATE outbox … WHERE id = ? AND status <> 'sent' — no website_id, though outbox.website_id is NOT NULL.
- **Fix:** outbox.Store.Retry takes websiteID and carries AND website_id = $3; "already sent", "no such message" and "not your message" stay one answer so the screen cannot be used to probe for foreign messages. Handler passes ws.ID.
- **Files changed:** internal/admin/order_scope_test.go, internal/outbox/outbox.go, internal/outbox/outbox_test.go, internal/admin/order.go, CHANGELOG.md
- **Why not caught:** No gate existed for this class, and the id was the *third* on the screen — the two ids in the address were both checked, which made the handler look scoped.
- **Recurrence guard:** internal/admin/order_scope_test.go (2 tests), internal/outbox/outbox_test.go (1 store-level test), and the changed Retry signature.
---

## Pattern note — the recurring defect shape in this codebase

**Six occurrences now**, over four days, all identical. Beside the three below:
the page **translation group** (`page.Store.SetTranslation` plus both readers —
and there the second id is a *link*, so the damage was a read: a foreign page's
title and draft badge printed on the editor's own screen), the **snippets**
(correct only because four separate callers each remembered; moved into the
store), and **`product_media`** (latent — `SetGallery` has no caller yet, so the
guard went in before the first one rather than after the first report).

The shape: **an admin route takes a website id
and a second resource id, the middleware authorises only the first, and the
second is written without being scoped.** `auth.RequireWebsiteAccess` states in
its own doc comment that it reads only the leading path segment, so it can never
help; the guarantee must be re-established per resource.

**Where to put the check.** In the store's WHERE clause, taking the website as a
parameter, so the compiler enumerates the callers and one statement cannot be
raced. When the second resource's table has no `website_id` column
(`menu_items`, `page_terms`, `product_media`, `cart_items`), that absence is the
signal that the check must live in the handler instead — and that it needs a
`*_scope_test.go` to hold it there, because nothing else will.

**How to test it.** Two websites, a resource on B, an editor whose
`user_websites` grants only A, drive the request through the real middleware
chain, and assert the **effect** (the row is unchanged) as well as the status.
Always include the negative controls — the same action on the own website, and
the create path — or a fix that refuses everything passes.


## Pattern note — the ledger drifts behind the code, and the gate reads the ledger

**Four occurrences, found on 2026-09-08 by reading the verification documents
against the tree.** In every case the code was correct and the record was not:

- `.planning/STATE.md` carried Phase 6's plugin-guest browser pass as **not
  run**. `06-VERIFICATION.md:212-260` records it run in full, with the archives
  uploaded, a migration applied and its sha256 recorded, and criterion 6 marked
  MET.
- The same file carried Phase 7's `code`-inside-a-block as **never driven**.
  `07-VERIFICATION.md:212-219` shows it driven, with the rendered HTML.
- `09-VERIFICATION.md` carried two gaps as open. Both were closed —
  `csvEnsureTerms` bundles the transaction the first one named, and the
  `planned` map closes the second — and both guards are green.
- The `ROADMAP.md` progress table listed Phases 6, 7 and 8 as *In Progress* with
  every plan complete, and **Phase 9 as "0/TBD, Not started"** while six plans,
  six waves, a security finding and four browser-found defects were done in it.

**Why it matters more here than in most projects.** The milestone close-out gate
is graded from these documents. `gsd-tools` derives `completed_phases` from the
ROADMAP table, so the stale table also produced a wrong progress counter — and a
phase recorded as "not started" is a phase a close-out gate will either block on
or, worse, skip.

**The rule that came out of it.** A verification document is amended, never
rewritten: keep the original verdict, add what closed it underneath, and say why
the amendment exists. Phase 6's report does this and it is the model — *"a
verification report that silently upgrades its own verdict is worth less than
one that shows what changed."* The stale entries were not wrong when written;
they were never revisited.

**How to detect it.** Count `*-SUMMARY.md` against `*-PLAN.md` per phase
directory and compare with the ROADMAP table; then read each verification
document's `gaps:` block and run the named guard. Four of four gaps in this
sweep were already closed.

## Method note — a mutation that stays green is a question, not a pass

The rule in this project is that a guard is only believed once somebody has
watched its test go red without it. The interesting case is the other one.

**2026-09-08, phase 10 wave 3.** The executor removed its empty-address guard
from the forward-auth middleware and *nothing changed* — same status, same
behaviour, all ten tests green. The easy conclusion is that the guard is
redundant. It kept going instead, and found that `users.email` is declared
`NOT NULL UNIQUE COLLATE NOCASE` and **nothing in that declaration forbids the
empty string**; it inserted one to be sure. So without the guard an identity
carrying no e-mail header is not refused but *looked up*, and it matches that
row. The guard was covered by the no-account branch for the inputs the tests
happened to use, and not for the input that mattered.

The test that came out of it seeds that row as an **administrator**, and the
finding became a written constraint on the next plan: provisioning must never
create an account with an empty address, or it creates exactly that row.

**The same day, one step further.** Reading that, the obvious next worry is
that `user.Store.Create` lets a caller make such a row and only the two
callers refuse it — the "correct because every caller remembered" shape this
knowledge base already has three entries about. Checked: `store.go:172-175`
refuses it in the store. The instinct was wrong and checking cost a minute.
Worth recording, because a pattern-match is a hypothesis and not a finding.

**So:**

- A green mutation means *something else is covering this input*. Find out what,
  and whether it covers every input the guard was written for.
- If nothing else covers it, the mutation was wrong — usually too narrow.
- If something does, say so in the comment: two guards where one would do is
  fine, two guards where nobody knows which one is load-bearing is not.
