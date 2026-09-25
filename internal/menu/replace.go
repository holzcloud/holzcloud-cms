package menu

import (
	"context"
	"database/sql"
	"fmt"
)

// ItemInput is one entry of a menu as a whole tree is written: what the
// assistant connection sends when it sets a menu's entries in one go.
type ItemInput struct {
	Title    string
	ItemType string // "page", "url" or "custom"
	URL      string
	PageID   *int64
	Children []ItemInput
}

// Tree is the entries of a menu as a tree, in their order — the shape a
// reader outside the templates wants, and the same one GetMenuTreeIn builds.
func Tree(items []MenuItem) []MenuNode { return buildTree(items) }

// ReplaceItems swaps every entry of a menu for the given tree, in one
// transaction.
//
// One transaction because the alternative — delete, then insert entry by entry
// — leaves a website without navigation for as long as that takes, and without
// any at all if the tenth insert fails. The caller has validated the tree (the
// types, that a page belongs to the menu's website); this only stores it.
func (s *Store) ReplaceItems(ctx context.Context, menuID int64, items []ItemInput) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// The children go with their parents through ON DELETE CASCADE.
	if _, err := tx.ExecContext(ctx, `DELETE FROM menu_items WHERE menu_id = $1`, menuID); err != nil {
		return fmt.Errorf("clear menu items: %w", err)
	}
	if err := insertItems(ctx, tx, menuID, nil, items); err != nil {
		return err
	}
	return tx.Commit()
}

func insertItems(ctx context.Context, tx *sql.Tx, menuID int64, parentID *int64, items []ItemInput) error {
	for i, it := range items {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO menu_items (menu_id, parent_id, title, item_type, url, page_id, sort_order)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			menuID, parentID, it.Title, it.ItemType, it.URL, it.PageID, i)
		if err != nil {
			return fmt.Errorf("create menu item: %w", err)
		}
		if len(it.Children) == 0 {
			continue
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("create menu item: %w", err)
		}
		if err := insertItems(ctx, tx, menuID, &id, it.Children); err != nil {
			return err
		}
	}
	return nil
}
