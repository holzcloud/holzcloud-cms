package menu

import "time"

// Menu represents a named menu for a website location (e.g. "main", "footer").
type Menu struct {
	ID          int64
	WebsiteID   int64
	Name        string
	LocationKey string
	// Locale is the language this menu belongs to. Empty is the website's main
	// language: a French page with German menu titles is not a French page.
	Locale    string
	CreatedAt time.Time
}

// MenuItem represents a single item within a menu.
type MenuItem struct {
	ID        int64
	MenuID    int64
	ParentID  *int64
	Title     string
	ItemType  string // "page", "url", "custom"
	URL       string
	PageID    *int64
	PageSlug  string // joined from pages table
	SortOrder int
	CreatedAt time.Time

	// PathPrefix is the language prefix of the menu this item belongs to:
	// empty in the website's main language, "/fr" in a second one. It is put in
	// front of a page link so a French menu points at the French address.
	//
	// On the item and not worked out while rendering, because rendering happens
	// in a template helper that knows nothing about the website.
	PathPrefix string
}

// MenuNode is a MenuItem with its children, used for tree rendering.
type MenuNode struct {
	MenuItem
	Children []MenuNode
}
