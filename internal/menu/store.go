package menu

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Store handles SQL operations for menus and menu items.
type Store struct {
	DB *db.DB
}

// NewStore creates a new menu store.
func NewStore(database *db.DB) *Store {
	return &Store{DB: database}
}

// CreateMenu creates a new menu.
func (s *Store) CreateMenu(ctx context.Context, websiteID int64, name, locationKey, loc string) (*Menu, error) {
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO menus (website_id, name, location_key, locale) VALUES ($1, $2, $3, $4)`,
		websiteID, name, locationKey, loc)
	if err != nil {
		return nil, fmt.Errorf("create menu: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetMenu(ctx, id)
}

// GetMenu returns a menu by ID.
func (s *Store) GetMenu(ctx context.Context, id int64) (*Menu, error) {
	var m Menu
	var createdAt string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, website_id, name, location_key, locale, created_at FROM menus WHERE id = $1`, id).
		Scan(&m.ID, &m.WebsiteID, &m.Name, &m.LocationKey, &m.Locale, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get menu: %w", err)
	}
	m.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return &m, nil
}

// ListMenus returns all menus for a website.
func (s *Store) ListMenus(ctx context.Context, websiteID int64) ([]Menu, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, name, location_key, locale, created_at FROM menus WHERE website_id = $1 ORDER BY name`,
		websiteID)
	if err != nil {
		return nil, fmt.Errorf("list menus: %w", err)
	}
	defer rows.Close()

	var menus []Menu
	for rows.Next() {
		var m Menu
		var createdAt string
		if err := rows.Scan(&m.ID, &m.WebsiteID, &m.Name, &m.LocationKey, &m.Locale, &createdAt); err != nil {
			return nil, fmt.Errorf("scan menu: %w", err)
		}
		m.CreatedAt, _ = time.Parse(timeLayout, createdAt)
		menus = append(menus, m)
	}
	return menus, rows.Err()
}

// UpdateMenu updates a menu's name and location key.
func (s *Store) UpdateMenu(ctx context.Context, id int64, name, locationKey string) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE menus SET name = $1, location_key = $2 WHERE id = $3`,
		name, locationKey, id)
	if err != nil {
		return fmt.Errorf("update menu: %w", err)
	}
	return nil
}

// DeleteMenu deletes a menu by ID.
func (s *Store) DeleteMenu(ctx context.Context, id int64) error {
	_, err := s.DB.Write.ExecContext(ctx, `DELETE FROM menus WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete menu: %w", err)
	}
	return nil
}

// CreateItem creates a new menu item.
func (s *Store) CreateItem(ctx context.Context, menuID int64, parentID *int64, title, itemType, url string, pageID *int64, sortOrder int) (*MenuItem, error) {
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO menu_items (menu_id, parent_id, title, item_type, url, page_id, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		menuID, parentID, title, itemType, url, pageID, sortOrder)
	if err != nil {
		return nil, fmt.Errorf("create menu item: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetItem(ctx, id)
}

// GetItem returns a menu item by ID.
func (s *Store) GetItem(ctx context.Context, id int64) (*MenuItem, error) {
	var mi MenuItem
	var parentID, pageID sql.NullInt64
	var createdAt string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT mi.id, mi.menu_id, mi.parent_id, mi.title, mi.item_type, mi.url,
		        mi.page_id, mi.sort_order, mi.created_at, COALESCE(p.slug, '') as page_slug
		 FROM menu_items mi
		 LEFT JOIN pages p ON p.id = mi.page_id
		 WHERE mi.id = $1`, id).
		Scan(&mi.ID, &mi.MenuID, &parentID, &mi.Title, &mi.ItemType, &mi.URL,
			&pageID, &mi.SortOrder, &createdAt, &mi.PageSlug)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get menu item: %w", err)
	}
	if parentID.Valid {
		mi.ParentID = &parentID.Int64
	}
	if pageID.Valid {
		mi.PageID = &pageID.Int64
	}
	mi.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return &mi, nil
}

// UpdateItem updates a menu item.
func (s *Store) UpdateItem(ctx context.Context, id int64, title, itemType, url string, pageID *int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE menu_items SET title = $1, item_type = $2, url = $3, page_id = $4 WHERE id = $5`,
		title, itemType, url, pageID, id)
	if err != nil {
		return fmt.Errorf("update menu item: %w", err)
	}
	return nil
}

// DeleteItem deletes a menu item by ID.
func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	_, err := s.DB.Write.ExecContext(ctx, `DELETE FROM menu_items WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete menu item: %w", err)
	}
	return nil
}

// ListItems returns all items for a menu in tree-building order.
func (s *Store) ListItems(ctx context.Context, menuID int64) ([]MenuItem, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT mi.id, mi.menu_id, mi.parent_id, mi.title, mi.item_type, mi.url,
		        mi.page_id, mi.sort_order, mi.created_at, COALESCE(p.slug, '') as page_slug
		 FROM menu_items mi
		 LEFT JOIN pages p ON p.id = mi.page_id
		 WHERE mi.menu_id = $1
		 ORDER BY mi.parent_id NULLS FIRST, mi.sort_order`,
		menuID)
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	defer rows.Close()

	var items []MenuItem
	for rows.Next() {
		var mi MenuItem
		var parentID, pageID sql.NullInt64
		var createdAt string
		if err := rows.Scan(&mi.ID, &mi.MenuID, &parentID, &mi.Title, &mi.ItemType, &mi.URL,
			&pageID, &mi.SortOrder, &createdAt, &mi.PageSlug); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		if parentID.Valid {
			mi.ParentID = &parentID.Int64
		}
		if pageID.Valid {
			mi.PageID = &pageID.Int64
		}
		mi.CreatedAt, _ = time.Parse(timeLayout, createdAt)
		items = append(items, mi)
	}
	return items, rows.Err()
}

// SwapSortOrder swaps the sort_order of two menu items in a transaction.
func (s *Store) SwapSortOrder(ctx context.Context, itemID1, itemID2 int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var order1, order2 int
	err = tx.QueryRowContext(ctx, `SELECT sort_order FROM menu_items WHERE id = $1`, itemID1).Scan(&order1)
	if err != nil {
		return fmt.Errorf("get sort_order 1: %w", err)
	}
	err = tx.QueryRowContext(ctx, `SELECT sort_order FROM menu_items WHERE id = $1`, itemID2).Scan(&order2)
	if err != nil {
		return fmt.Errorf("get sort_order 2: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE menu_items SET sort_order = $1 WHERE id = $2`, order2, itemID1)
	if err != nil {
		return fmt.Errorf("update sort_order 1: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE menu_items SET sort_order = $1 WHERE id = $2`, order1, itemID2)
	if err != nil {
		return fmt.Errorf("update sort_order 2: %w", err)
	}

	return tx.Commit()
}

// GetMenuTree returns menu items as a tree for a website and location key.
func (s *Store) GetMenuTree(ctx context.Context, websiteID int64, locationKey string) ([]MenuNode, error) {
	return s.GetMenuTreeIn(ctx, websiteID, locationKey, "")
}

// GetMenuTreeIn is the same for one language.
func (s *Store) GetMenuTreeIn(ctx context.Context, websiteID int64, locationKey, loc string) ([]MenuNode, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT mi.id, mi.menu_id, mi.parent_id, mi.title, mi.item_type, mi.url,
		        mi.page_id, mi.sort_order, mi.created_at, COALESCE(p.slug, '') as page_slug
		 FROM menu_items mi
		 JOIN menus m ON m.id = mi.menu_id
		 -- The same conditions as page.PublicPredicate. A menu entry pointing at
		 -- a draft, a scheduled or a trashed page renders as plain text, so the
		 -- navigation never links somewhere that answers 404.
		 LEFT JOIN pages p ON p.id = mi.page_id
		     AND p.status = 'published'
		     AND (p.publish_at IS NULL OR p.publish_at <= strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		     AND (p.unpublish_at IS NULL OR p.unpublish_at > strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		     AND p.deleted_at IS NULL
		 WHERE m.website_id = $1 AND m.location_key = $2 AND m.locale = $3
		 ORDER BY mi.parent_id NULLS FIRST, mi.sort_order`,
		websiteID, locationKey, loc)
	if err != nil {
		return nil, fmt.Errorf("get menu tree: %w", err)
	}
	defer rows.Close()

	var items []MenuItem
	for rows.Next() {
		var mi MenuItem
		var parentID, pageID sql.NullInt64
		var createdAt string
		if err := rows.Scan(&mi.ID, &mi.MenuID, &parentID, &mi.Title, &mi.ItemType, &mi.URL,
			&pageID, &mi.SortOrder, &createdAt, &mi.PageSlug); err != nil {
			return nil, fmt.Errorf("scan menu tree item: %w", err)
		}
		if parentID.Valid {
			mi.ParentID = &parentID.Int64
		}
		if pageID.Valid {
			mi.PageID = &pageID.Int64
		}
		// Every item of this menu carries the menu's prefix, so a page link in
		// a second language lands on that language's address.
		if loc != "" {
			mi.PathPrefix = "/" + loc
		}
		mi.CreatedAt, _ = time.Parse(timeLayout, createdAt)
		items = append(items, mi)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return buildTree(items), nil
}

// buildTree converts a flat sorted list of menu items into a tree.
//
// The linking pass works entirely on pointers and only dereferences at the very
// end. Attaching a value copy while linking would snapshot a child before its
// own children were attached, and since the query orders by parent_id ascending
// a grandchild is linked after its parent was already copied — which silently
// dropped every third-level item, even though README and RenderMenu both
// promise three levels.
type treeNode struct {
	item     MenuItem
	children []*treeNode
}

func buildTree(items []MenuItem) []MenuNode {
	nodes := make(map[int64]*treeNode, len(items))
	for _, item := range items {
		nodes[item.ID] = &treeNode{item: item}
	}

	var roots []*treeNode
	for _, item := range items {
		node := nodes[item.ID]
		if item.ParentID == nil {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodes[*item.ParentID]
		if !ok || parent == node || createsCycle(nodes, node, parent) {
			// Orphaned or self-referential: surface it rather than lose it.
			roots = append(roots, node)
			continue
		}
		parent.children = append(parent.children, node)
	}

	return materialize(roots)
}

// createsCycle reports whether making parent the parent of node would close a
// loop, which would make materialize recurse forever.
func createsCycle(nodes map[int64]*treeNode, node, parent *treeNode) bool {
	for cur := parent; cur != nil; {
		if cur == node {
			return true
		}
		if cur.item.ParentID == nil {
			return false
		}
		next, ok := nodes[*cur.item.ParentID]
		if !ok || next == cur {
			return false
		}
		cur = next
	}
	return false
}

// materialize turns the pointer tree into the value tree templates render.
func materialize(nodes []*treeNode) []MenuNode {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]MenuNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, MenuNode{MenuItem: n.item, Children: materialize(n.children)})
	}
	return out
}
