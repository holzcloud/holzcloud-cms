---
status: fixing
trigger: "A second cross-website write vulnerability, same family as the menu one fixed on 2026-09-06. internal/admin/product.go handleProductSave (the POST arm of HandleProductForm) reads productID from the path, builds a shop.Product, and calls h.products.Update — without ever checking the product belongs to the website in the path. Prove it first with a failing test in its own commit, then fix it in the store."
created: 2026-09-07T00:00:00Z
updated: 2026-09-07T00:00:00Z
---

## Current Focus

hypothesis: "handleProductSave authorizes on the websiteID taken from the address but writes the product named by productID without checking product.WebsiteID == ws.ID. shop.Store.Update's WHERE clause is id-only, so the store cannot catch what the caller forgot. An editor granted website A can therefore rewrite website B's product through /admin/websites/<A>/produkte/<B productID>."
test: "A Go test in internal/admin/ modelled on menu_scope_test.go: websites A and B, a product on B, an editor whose user_websites grants only A, POST to /admin/websites/<A>/produkte/<B productID> and assert refusal AND that B's product's title and price are unchanged."
expecting: "The test FAILS before the fix — the write lands, the price and title change, a 303 comes back. That failure is the proof."
next_action: "Read internal/admin/product.go in full, internal/shop/product.go, internal/term/store.go SetForProduct, cmd/holzcloud/main.go:860-880 route registration."

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

root_cause: ""
fix: ""
verification: ""
files_changed: []

- timestamp: 2026-09-07T00:10:00Z
  checked: "internal/admin/product.go handleProductSave (:214-289) against its siblings HandleProductForm GET arm (:158-170) and HandleProductDelete (:299-317)"
  found: "Both siblings do `p, err := h.products.Get(ctx, id)` then `if p == nil || p.WebsiteID != ws.ID { http.NotFound }`. handleProductSave does neither: it reads `r.PathValue(\"productID\")` into values.ID at :228-230, builds `&shop.Product{ID: values.ID, WebsiteID: ws.ID, ...}` at :250 and calls h.products.Update at :277. No Get, no WebsiteID comparison anywhere in the function."
  implication: "Leg 1 confirmed by reading. The handler trusts an id it never checked."

- timestamp: 2026-09-07T00:10:00Z
  checked: "internal/shop/product.go Store.Update (:166-186) and Store.Delete (:189-192) and Store.Get (:195-202)"
  found: "Update is `UPDATE products SET slug,title,subtitle,description_markdown,description_html,excerpt,sku,price_gross,tax_bp,stock,weight_grams,delivery_note,status,featured_media_id,position,updated_at WHERE id=$17`. website_id appears in neither the SET list nor the WHERE clause. Delete is `DELETE FROM products WHERE id = $1`. Get is `SELECT ... WHERE id = $1`."
  implication: "Leg 2 confirmed. The store cannot catch what the caller forgot, and because website_id is not in SET the product cannot be moved — only rewritten in place, which is the worse outcome for the victim: their own product, at their own address, with the attacker's price."

- timestamp: 2026-09-07T00:10:00Z
  checked: "cmd/holzcloud/main.go:868-872 shop route registration against the requireAdmin-wrapped rows at :878-879"
  found: "The four produkte routes use plain adminProtectedMux.HandleFunc. The two /shop settings routes on the very next lines use adminProtectedMux.Handle(..., requireAdmin(...)). So the product routes are reachable by any signed-in user who passes RequireWebsiteAccess, editors included."
  implication: "Leg 3 confirmed. The attack does not need an administrator; role editor suffices, which is what makes this a privilege boundary rather than an admin footgun."

- timestamp: 2026-09-07T00:10:00Z
  checked: "internal/db/migrations/00039_shop_catalogue.sql:11-62"
  found: "products.website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE, plus UNIQUE (website_id, slug)."
  implication: "The column needed for a store-level WHERE id = $1 AND website_id = $2 exists. Unlike menu_items, this fix belongs in the store — no JOIN, no subquery."

- timestamp: 2026-09-07T00:10:00Z
  checked: "internal/term/store.go SetForProduct (:364-402) and internal/shop/product.go SetTerms (:289-308), SetGallery (:331-350), TermIDs (:311), GalleryIDs (:353), AdjustStock (:378)"
  found: "SetForProduct opens `DELETE FROM product_terms WHERE product_id = $1` with no website scoping, though it does take websiteID and uses it correctly for the terms rows themselves. SetTerms and SetGallery have the identical unscoped DELETE. AdjustStock, TermIDs and GalleryIDs are all id-only."
  implication: "Leg 4 confirmed. The categories are collateral damage of the same call. These are all product_id-keyed join tables with no website_id of their own, so they inherit scoping from whoever proves the product; that proof is exactly what the handler skips."

- timestamp: 2026-09-07T00:20:00Z
  checked: "The new internal/admin/product_scope_test.go run against the unfixed tree — go test ./internal/admin/ -run TestProduct"
  found: |
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
    The three control tests (own website, create, delete) passed on the same run.
  implication: |
    Hypothesis CONFIRMED, and worse than reported in one respect. "2 categories
    to 1" is not simply erasure: setProductTerms is called with ws.ID = website
    A while productID names website B's product, so the DELETE removed B's two
    labels and the INSERT attached a term belonging to A. The join table
    product_terms ends up holding a row that crosses the tenant boundary — a
    corruption that survives the fix and would need a data repair, not just a
    code one. The control tests passing on the same run rules out a broken
    fixture: the same request shape on the own website returns 303 and takes.

- timestamp: 2026-09-07T00:20:00Z
  checked: "Whether the store or the middleware could be the better place, given the menu precedent"
  found: "products.website_id exists (migration 00039:13). internal/term/store.go Delete/Rename, internal/kind, internal/block and internal/field all already take (ctx, websiteID, id) and put website_id in the WHERE clause. The menu fix could not do this because menu_items reaches its website only through menus."
  implication: "The store is the right place here and the menu compromise does not apply. Changing Update's signature to (ctx, websiteID, p) makes the compiler enumerate the call sites, which is the property a comment cannot provide."
