---
status: verifying
trigger: "A second cross-website write vulnerability, same family as the menu one fixed on 2026-09-06. internal/admin/product.go handleProductSave (the POST arm of HandleProductForm) reads productID from the path, builds a shop.Product, and calls h.products.Update — without ever checking the product belongs to the website in the path. Prove it first with a failing test in its own commit, then fix it in the store."
created: 2026-09-07T00:00:00Z
updated: 2026-09-07T00:00:00Z
---

## Current Focus

reasoning_checkpoint:
  hypothesis: "handleProductSave passes a path-supplied productID to shop.Store.Update, whose WHERE clause is id-only, so an editor confined to website A rewrites website B's product in place."
  confirming_evidence:
    - "Direct observation: the committed test 071bead returns 303 and website B's product is retitled, repriced 2490.00 -> 0.05 and moved to a new slug."
    - "Direct read: handleProductSave (product.go:214-289) contains no Get and no WebsiteID comparison; its two siblings at :164 and :314 contain both."
    - "Direct read: UPDATE products ... WHERE id=$17 — website_id is in neither SET nor WHERE."
    - "Direct observation: the three control tests (own website, create, delete) pass on the same run, so the fixture is not simply refusing everything."
  falsification_test: "If the store gained AND website_id = $2 and the attack still landed, the hypothesis about where the check belongs would be wrong. Conversely if the controls went red after the fix, the fix would be over-broad rather than correct."
  fix_rationale: "The root cause is an unscoped WHERE clause reachable by an unchecked caller. Adding the check to the handler alone would fix this caller and leave the store as willing as before for the next one — the compromise the menu fix had to make and explicitly flagged. products.website_id exists, so the WHERE clause can carry the scope itself and the signature change makes the compiler enumerate every caller."
  blind_spots:
    - "product_terms rows already written across the tenant boundary in a live database are not repaired by any code change; that needs a data fix and the CHANGELOG must say so."
    - "SetTerms, SetGallery, AdjustStock, TermIDs and GalleryIDs stay id-only after this fix; they are clean by construction only for as long as their callers prove the product first."
  candidate_causes:
    - "code: the save arm was written without the WebsiteID comparison its siblings have (confirmed)"
    - "code: the store's WHERE clause cannot enforce what the caller skipped (confirmed)"
    - "config: the routes are registered without requireAdmin, so the reachable population is editors, not admins (confirmed — main.go:868-872 beside requireAdmin-wrapped rows at :878-879)"
    - "data: product_terms has no website_id of its own and inherits scope from the product, so a wrong product id silently produces a cross-tenant row (confirmed by the 2-categories-to-1 observation)"
  and_gate: "yes. The write requires all three simultaneously: the missing handler check, the unscoped store WHERE, and a route that admits a non-admin. Close any one and the editor cannot reach a foreign product — which is why the fix goes in the store, the leg that protects every future caller rather than only this route."

next_action: "Change shop.Store.Update and Delete to take websiteID, add shop.ErrNotFound on RowsAffected()==0, map it to 404 in internal/admin/product.go, and harden the product_id-keyed writes in term.SetForProduct / shop.SetTerms / shop.SetGallery."

## Symptoms

expected: |
  An editor granted access to website A only must not be able to write ANY
  resource belonging to website B. Every admin write handler that takes both a
  {id} website path value and a second resource id must verify the second
  resource belongs to the website named in the path — the sibling arms already
  do: the GET arm of HandleProductForm (product.go:164) and
  HandleProductDelete (:314) both compare p.WebsiteID != ws.ID.

actual: |
  handleProductSave does not. It parses productID from the path and calls
  h.products.Update with a shop.Product carrying that id. shop.Store.Update
  (internal/shop/product.go:165-186) is UPDATE products SET ... WHERE id=$17
  with no website_id in the WHERE clause. website_id is not in the SET list
  either, so the product cannot be MOVED between websites, but every other
  column can be rewritten: title, subtitle, description, sku, price_gross,
  tax_bp, stock, weight_grams, delivery_note, status, featured_media_id,
  position. h.setProductTerms -> term.Store.SetForProduct
  (internal/term/store.go:364) opens with an unscoped
  DELETE FROM product_terms WHERE product_id = $1, so the categories go too.

errors: |
  No runtime error. Silent cross-tenant write — the request succeeds with 303.

reproduction: |
  1. Create websites A and B.
  2. Create a product on B.
  3. Create an editor granted access to A only (row in user_websites for A).
  4. Sign in as that editor.
  5. POST /admin/websites/<A>/produkte/<B productID> with a product form.
  6. Observe B's product rewritten.

started: |
  Since the product save handler was written. Same shape as the menu bug: the
  mechanism applied correctly at every sibling site and silently missing at one.

## Eliminated

<!-- APPEND only -->

## Evidence

- timestamp: 2026-09-07T00:00:00Z
  checked: "Knowledge base / prior session .planning/debug/menu-item-cross-website-write.md"
  found: "Identical defect family resolved 2026-09-06 in internal/admin/menu.go. Root cause: handlers authorize on path websiteID, never verify the tail id belongs to that website; auth.RequireWebsiteAccess reads only the leading /admin/websites/<number> by documented design. Fix went in the handler because menu_items has no website_id column; the resolution notes a follow-up to move it into the store."
  implication: "Strong known-pattern candidate. products DOES have a website_id column, so this one can and must be fixed in the store, per the task brief and per the term/kind/block/field precedent."

## Resolution

root_cause: "internal/admin/product.go handleProductSave took productID from the address and passed it to shop.Store.Update without ever establishing that the product belongs to the website in the address; shop.Store.Update's WHERE clause named only the id, so it could not catch the omission; and the four produkte routes are registered without requireAdmin, so the reachable population is editors. All three are required simultaneously — the AND-gate fired. PROVEN by test: the write returned 303 and rewrote website B's product while authenticated as an editor assigned only to website A."

fix: "The check went into internal/shop/product.go, not the handler. Update and Delete now take websiteID and carry AND website_id in the WHERE clause, returning the new shop.ErrNotFound when RowsAffected()==0. The signature change made the compiler name both production callers (product.go:279 and :318) rather than leaving that to a reader — the property the menu fix could not have, because menu_items has no website_id column and its store fix was left as a comment. internal/admin/product.go maps ErrNotFound to 404, before setProductTerms runs. internal/term/store.go SetForProduct scopes its DELETE by subquery and joins products to terms on the shared website, so a product_terms row spanning two websites is now structurally impossible."

verification: |
  guardrail_verdict: accepted
  signal_repro: "The six tests of internal/admin/product_scope_test.go, committed failing in 071bead, pass unmodified against the fix. The test file was not touched in the fix commit."
  signal_regression: "go test ./... exit 0. go build ./... and go vet ./... clean, gofmt -l . empty."
  signal_mutation: "Removing AND website_id=$18 from Update and AND website_id = $2 from Delete turns 3 admin tests and 2 shop tests red. Removing the subquery from SetForProduct's DELETE turns TestSetForProductLeavesAForeignProductAlone red ('the foreign product went from 2 labels to 0'). Every new guard is load-bearing."
  signal_not_deletion_only: "The diff adds a WHERE condition, a sentinel error, a RowsAffected check and a 404 branch. Nothing was removed."
  signal_controls: "Six negative controls pass and passed before the fix as well: save on the own website, create via the 'neu' sentinel, delete of the own product, store-level update and delete on the owning website, and SetForProduct on its own product. An over-broad fix would have turned these red."
  oracle_type: specified
  boundary_neighbours: "unknown id (999) answers ErrNotFound like a foreign id; the 'neu' create sentinel is unaffected; the admin role is covered separately because RequireWebsiteAccess cannot help there at all."

files_changed: ["internal/admin/product_scope_test.go", "internal/shop/product.go", "internal/shop/product_test.go", "internal/admin/product.go", "internal/term/store.go", "internal/term/store_test.go", "internal/shop/order_test.go", "internal/shop/cart_test.go", "CHANGELOG.md"]

## Second finding — outbox retry

- timestamp: 2026-09-07T01:00:00Z
  checked: "The audit of internal/admin/order.go, the order half of the brief's remit — HandleOrderDetail's POST arm at :198-223"
  found: "Three ids meet on that screen. The website comes from the address and is authorised. The order comes from the address and is fetched with ByNumber(ctx, ws.ID, ...), which is scoped. The third — `mail_id` — comes from the posted form and is passed to retryMail (:344) and on to outbox.Store.Retry (internal/outbox/outbox.go:241), whose statement is `UPDATE outbox SET status='pending', attempts=0, last_error='', next_attempt_at=$1 WHERE id = $2 AND status <> 'sent'`. No website_id, although outbox.website_id is NOT NULL (migration 00042:15)."
  implication: "Same family, same remit, different table. Proven by internal/admin/order_scope_test.go: as an editor assigned only to website A, posting aktion=mail-erneut with website B's message id re-queues it — status failed -> pending, attempts 5 -> 0, last_error cleared. Consequence is a delivery, not a defacement: B's customer receives the message again, and B's operator loses the error text they were about to act on. The control test (the same action on the own website) passes, so the handler is not simply broken."
