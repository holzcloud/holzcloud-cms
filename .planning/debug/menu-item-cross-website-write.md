---
status: resolved
trigger: "Cross-website write vulnerability: internal/admin/menu.go menu ITEM handlers (HandleMenuItemCreate :285, HandleMenuItemUpdate :364, HandleMenuItemDelete :426) parse websiteID from path but never verify the menu named by menuID belongs to that website. Prove it with a failing test first, then fix."
created: 2026-09-06T17:27:40Z
updated: 2026-09-06T17:27:40Z
---

## Current Focus

hypothesis: "The three menu-item handlers in internal/admin/menu.go authorize on the path websiteID (via auth.RequireWebsiteAccess) but then operate on menuID/itemID without ever checking menu.WebsiteID == websiteID. An editor granted access to website A can therefore create/update/delete menu items belonging to website B by using A's id in the path and B's menuID/itemID in the tail."
test_ORIGINAL: "Write a Go test in internal/admin/ that provisions websites A and B, a menu with an item on B, and a user whose user_websites grants ONLY A; then drive POST /admin/websites/<A>/menus/<B menuID>/items[...] for create, update and delete and assert each is refused."
expecting: "The test FAILS before any fix (writes succeed / return 2xx-3xx instead of being refused). That failure is the proof. If the writes are already refused, the hypothesis is wrong and the report must say so."
next_action: "Read internal/admin/menu.go in full (all menu + menu-item handlers), internal/menu/store.go, internal/admin/handler.go:160-190, internal/auth/middleware.go, and the existing test harness in internal/admin/page_handler_test.go plus any menu tests, to learn how to build websites/users/sessions/CSRF in a test."

## Symptoms

expected: |
  A signed-in user who is granted access to website A only must not be able to
  read or write ANY resource belonging to website B. Every admin write handler
  that takes both a {id} website path value and a second resource id must
  verify that the second resource belongs to the website named in the path.
  The sibling menu handlers already do this: HandleMenuEdit (:162),
  HandleMenuUpdate (:213-218) and HandleMenuDelete (:261) all check
  m.WebsiteID != websiteID.

actual: |
  The three menu-ITEM handlers do not. They check only item.MenuID != menuID,
  which proves the item belongs to the named menu but says nothing about which
  website that menu belongs to. Reported call chain:
    - cmd/holzcloud/main.go:1034 wraps the admin mux in requireWebsite =
      auth.RequireWebsiteAccess (main.go:716)
    - internal/auth/middleware.go:102 takes the website id from the URL path
      (websiteIDInPath); :108 checks the signed-in user may access THAT website
      and returns 403 otherwise
    - per-website rights are real: user_websites
      (internal/db/migrations/00033_user_rights.sql:30-31), resolved by
      internal/admin/handler.go:173-174
  So for an editor allowed on website A only:
    POST /admin/websites/<A>/menus/<B menuID>/items/<B itemID>
  passes the middleware (it sees website A) and the handler then edits website
  B's menu item. Same shape for create and delete.

errors: |
  No runtime error. Silent cross-tenant write — the request succeeds.

reproduction: |
  1. Create websites A and B.
  2. Create a menu on B with at least one item.
  3. Create a user granted access to A only (row in user_websites for A, none for B).
  4. Sign in as that user.
  5. POST /admin/websites/<A>/menus/<B menuID>/items            (create)
     POST /admin/websites/<A>/menus/<B menuID>/items/<B itemID> (update)
     POST .../items/<B itemID>/delete                           (delete)
  6. Observe the write landing on website B's menu.

started: |
  Unknown / likely since the menu-item handlers were written. The sibling menu
  handlers one level up got the check; the item handlers were overlooked. This
  is the project's recurring defect shape: a mechanism applied correctly at
  every known site and silently missing at one overlooked site.

## Eliminated

<!-- APPEND only -->

## Evidence

- timestamp: 2026-09-06T17:27:40Z
  checked: "Orchestrator pre-flight read of internal/admin/menu.go lines 275-300, 355-380, 420-445"
  found: "All three item handlers parse websiteID from PathValue(\"id\") and menuID from PathValue(\"menuID\"). In the ~20 lines following each parse there is no lookup of the menu and no comparison of a menu's WebsiteID against websiteID. HandleMenuItemDelete goes straight to h.menuStore.GetItem(ctx, itemID) — an id-only lookup."
  implication: "Structurally consistent with the reported hypothesis. websiteID appears to be used only to build the redirect URL. Needs a runtime test to prove exploitability rather than mere untidiness."

- timestamp: 2026-09-06T17:27:40Z
  checked: "internal/auth/middleware.go RequireWebsiteAccess"
  found: "websiteID := websiteIDInPath(r.URL.Path); if userID == 0 || !allowed(ctx, userID, websiteID) -> 403. The doc comment states explicitly: 'It therefore reads the address rather than the route: everything under /admin/websites/<number> belongs to that website, whatever the rest of the path turns out to be.'"
  implication: "The middleware authorizes ONLY the path website. It is by design incapable of noticing that a tail id belongs to a different website. That guarantee must therefore be re-established inside each handler (or its store). Confirms the middleware is not a mitigating control here."

- timestamp: 2026-09-06T17:27:40Z
  checked: "internal/term/store.go:284-311 (Delete, Rename) as the comparison pattern"
  found: "Both take (ctx, websiteID, id) and put website_id in the WHERE clause; Rename additionally errors when RowsAffected()==0."
  implication: "A safer store-level shape already exists in this codebase, so the fix has a local precedent to point at. Blast radius of adopting it in internal/menu/store.go must be weighed against a handler-level fix that matches the already-correct sibling menu handlers."

## Resolution

root_cause: "internal/admin/menu.go's four menu-ITEM handlers (Create :285, Update :364, Delete :426, Reorder :467) authorize on the websiteID taken from the address but never verify that the menu named by menuID belongs to that website. They compare item.MenuID != menuID only, which establishes item-in-menu and nothing about menu-in-website. auth.RequireWebsiteAccess cannot compensate: by documented design it reads only the leading path segment. PROVEN by test: all four writes landed on website B while authenticated as an editor assigned only to website A, each returning 303."
fix: "Two helpers in internal/admin/menu.go — menuOfWebsite(ctx, websiteID, menuID) and itemOfWebsite(ctx, websiteID, menuID, itemID) — with all seven menu handlers routed through them. HandleMenuItemCreate gained an explicit menu lookup it never had. Handler level rather than store level because menu_items has no website_id column (it reaches the website only through menus), so the term-store shape would need a JOIN or subquery in five methods; internal/menu/store.go now documents on the Store type that it enforces no scoping and that callers must."
verification: "go build ./... clean; go vet ./... clean; gofmt -l . empty; go test ./... exit 0; go run ./tools/i18n reports 0 verwaist in every catalogue. All six menu_scope tests green after the fix, four of them red before it. The own-website control test passed both before and after, ruling out an over-broad fix. Revert signal satisfied by construction: the tests were committed failing in de4a1ce before the fix in 5e453a9."
files_changed: ["internal/admin/menu_scope_test.go", "internal/admin/menu.go", "internal/menu/store.go"]
commits: ["de4a1ce (failing test)", "5e453a9 (fix)"]
audit: "51 admin routes with a website id plus a second resource id checked. Only the four menu item handlers were dirty (Create, Update, Delete and Reorder — Reorder was not in the original report). Pages, page revisions, media, snippets, terms, redirects, saved views, products, orders, preview, content kinds, block kinds and fields all verified clean. Full list in .planning/quick/260906-menu-scope/REPORT.md."
