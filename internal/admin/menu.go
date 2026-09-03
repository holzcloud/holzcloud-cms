package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// MenuListData extends LayoutData for the menu list page.
type MenuListData struct {
	web.LayoutData
	WebsiteID int64
	Menus     []menu.Menu
	// Languages is the language picker, empty on a website with one language.
	// A menu belongs to a language: a French page with German menu titles is
	// not a French page.
	Languages []LanguageChoice
	// Website is kept for LanguageName, which has to know the main language to
	// give the empty tag a name.
	Website *domain.Website
}

// LanguageName is the name of a menu's language, for the listing.
func (d MenuListData) LanguageName(stored string) string {
	if name := rowLanguage(d.Website, stored); name != "" {
		return name
	}
	return "–"
}

// MenuEditData extends LayoutData for the menu edit page.
type MenuEditData struct {
	web.LayoutData
	WebsiteID int64
	Menu      *menu.Menu
	Items     []menuItemNode
	Pages     []page.Page
}

// menuItemNode is a flat representation of a menu item with depth for indentation.
type menuItemNode struct {
	menu.MenuItem
	Depth int
}

// HandleMenuList lists menus for a website.
func (h *Handler) HandleMenuList(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	menus, err := h.menuStore.ListMenus(r.Context(), websiteID)
	if err != nil {
		return err
	}

	data := MenuListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Menüs – %s", ws.Name)),
		WebsiteID:  websiteID,
		Menus:      menus,
		Languages:  languageChoices(ws, ""),
		Website:    ws,
	}
	data.ActiveNav = "menus"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "menu_list", data)
}

// HandleMenuCreate creates a new menu for a website.
func (h *Handler) HandleMenuCreate(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	locationKey := strings.TrimSpace(r.FormValue("location_key"))
	// The language is only trusted when the website actually has it: a menu in
	// a language nobody serves would be invisible and unexplainable.
	loc := ""
	if ws, err := h.domains.GetWebsite(r.Context(), websiteID); err == nil && ws != nil {
		loc = locale.Pick(r.FormValue("sprache"), ws.Locales())
	}

	if name == "" || locationKey == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte Name und Kennung angeben")
		http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/menus", websiteID), http.StatusSeeOther)
		return nil
	}

	// Validate location_key format (T-04-09)
	if !isValidLocationKey(locationKey) {
		web.SetFlashError(h.sm, r.Context(), "Die Kennung darf nur Kleinbuchstaben, Ziffern und Bindestriche enthalten")
		http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/menus", websiteID), http.StatusSeeOther)
		return nil
	}

	if _, err := h.menuStore.CreateMenu(r.Context(), websiteID, name, locationKey, loc); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			web.SetFlashError(h.sm, r.Context(), "Für diese Website gibt es in dieser Sprache bereits ein Menü mit dieser Kennung")
			http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/menus", websiteID), http.StatusSeeOther)
			return nil
		}
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Menü angelegt")
	redirect := fmt.Sprintf("/admin/websites/%d/menus", websiteID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleMenuEdit shows the menu editor with items.
func (h *Handler) HandleMenuEdit(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	menuID, err := strconv.ParseInt(r.PathValue("menuID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	m, err := h.menuStore.GetMenu(r.Context(), menuID)
	if err != nil {
		return err
	}
	if m == nil || m.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	items, err := h.menuStore.ListItems(r.Context(), menuID)
	if err != nil {
		return err
	}

	// Build flat list with depth for template indentation
	flat := buildFlatTree(items)

	// Load published pages for the page-link dropdown, in the menu's own
	// language: a French menu linking to a German page sends the visitor out of
	// the language they chose, and the address it would build does not exist.
	pages, _, err := h.pages.ListPages(r.Context(), websiteID,
		page.ListFilter{Status: "published", Locale: m.Locale, Sort: "title", Page: 1, PerPage: 500})
	if err != nil {
		return err
	}

	data := MenuEditData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Menü %s – %s", m.Name, ws.Name)),
		WebsiteID:  websiteID,
		Menu:       m,
		Items:      flat,
		Pages:      pages,
	}
	data.ActiveNav = "menus"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "menu_edit", data)
}

// HandleMenuUpdate updates a menu's name and location key.
func (h *Handler) HandleMenuUpdate(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	menuID, err := strconv.ParseInt(r.PathValue("menuID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	m, err := h.menuStore.GetMenu(r.Context(), menuID)
	if err != nil {
		return err
	}
	if m == nil || m.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	locationKey := strings.TrimSpace(r.FormValue("location_key"))
	if name == "" || locationKey == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte Name und Kennung angeben")
		http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID), http.StatusSeeOther)
		return nil
	}

	if err := h.menuStore.UpdateMenu(r.Context(), menuID, name, locationKey); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Menü gespeichert")
	redirect := fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleMenuDelete deletes a menu.
func (h *Handler) HandleMenuDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	menuID, err := strconv.ParseInt(r.PathValue("menuID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	m, err := h.menuStore.GetMenu(r.Context(), menuID)
	if err != nil {
		return err
	}
	if m == nil || m.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	if err := h.menuStore.DeleteMenu(r.Context(), menuID); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Menü gelöscht")
	redirect := fmt.Sprintf("/admin/websites/%d/menus", websiteID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleMenuItemCreate creates a new menu item.
func (h *Handler) HandleMenuItemCreate(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	menuID, err := strconv.ParseInt(r.PathValue("menuID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte einen Titel für den Eintrag angeben")
		http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID), http.StatusSeeOther)
		return nil
	}

	itemType := r.FormValue("item_type")
	// Validate item_type (T-04-09)
	if itemType != "page" && itemType != "url" && itemType != "custom" {
		web.SetFlashError(h.sm, r.Context(), "Ungültige Art des Menüeintrags")
		http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID), http.StatusSeeOther)
		return nil
	}

	url := strings.TrimSpace(r.FormValue("url"))
	var pageID *int64
	if itemType == "page" {
		if pid := r.FormValue("page_id"); pid != "" {
			p, err := strconv.ParseInt(pid, 10, 64)
			if err == nil {
				pageID = &p
			}
		}
	}

	var parentID *int64
	if pid := r.FormValue("parent_id"); pid != "" {
		p, err := strconv.ParseInt(pid, 10, 64)
		if err == nil && p > 0 {
			parentID = &p
		}
	}

	// Get next sort order
	items, err := h.menuStore.ListItems(r.Context(), menuID)
	if err != nil {
		return err
	}
	sortOrder := 0
	for _, item := range items {
		if (parentID == nil && item.ParentID == nil) || (parentID != nil && item.ParentID != nil && *parentID == *item.ParentID) {
			if item.SortOrder >= sortOrder {
				sortOrder = item.SortOrder + 1
			}
		}
	}

	if _, err := h.menuStore.CreateItem(r.Context(), menuID, parentID, title, itemType, url, pageID, sortOrder); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Eintrag hinzugefügt")
	redirect := fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleMenuItemUpdate updates a menu item.
func (h *Handler) HandleMenuItemUpdate(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	menuID, err := strconv.ParseInt(r.PathValue("menuID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	itemID, err := strconv.ParseInt(r.PathValue("itemID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	item, err := h.menuStore.GetItem(r.Context(), itemID)
	if err != nil {
		return err
	}
	if item == nil || item.MenuID != menuID {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	title := strings.TrimSpace(r.FormValue("title"))
	itemType := r.FormValue("item_type")
	if itemType != "page" && itemType != "url" && itemType != "custom" {
		itemType = item.ItemType
	}
	url := strings.TrimSpace(r.FormValue("url"))
	var pageID *int64
	if itemType == "page" {
		if pid := r.FormValue("page_id"); pid != "" {
			p, err := strconv.ParseInt(pid, 10, 64)
			if err == nil {
				pageID = &p
			}
		}
	}

	if err := h.menuStore.UpdateItem(r.Context(), itemID, title, itemType, url, pageID); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Eintrag gespeichert")
	redirect := fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID)
	_ = menuID // used in redirect
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleMenuItemDelete deletes a menu item.
func (h *Handler) HandleMenuItemDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	menuID, err := strconv.ParseInt(r.PathValue("menuID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	itemID, err := strconv.ParseInt(r.PathValue("itemID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	item, err := h.menuStore.GetItem(r.Context(), itemID)
	if err != nil {
		return err
	}
	if item == nil || item.MenuID != menuID {
		http.NotFound(w, r)
		return nil
	}

	if err := h.menuStore.DeleteItem(r.Context(), itemID); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Eintrag gelöscht")
	redirect := fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleMenuItemReorder moves a menu item up or down.
func (h *Handler) HandleMenuItemReorder(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	menuID, err := strconv.ParseInt(r.PathValue("menuID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	itemID, err := strconv.ParseInt(r.PathValue("itemID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	direction := r.URL.Query().Get("direction")
	if direction != "up" && direction != "down" {
		web.SetFlashError(h.sm, r.Context(), "Ungültige Richtung")
		http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID), http.StatusSeeOther)
		return nil
	}

	// Get the item to find its parent and sort order
	item, err := h.menuStore.GetItem(r.Context(), itemID)
	if err != nil {
		return err
	}
	if item == nil || item.MenuID != menuID {
		http.NotFound(w, r)
		return nil
	}

	// Get all siblings (same parent)
	allItems, err := h.menuStore.ListItems(r.Context(), menuID)
	if err != nil {
		return err
	}

	var siblings []menu.MenuItem
	for _, it := range allItems {
		sameParent := (item.ParentID == nil && it.ParentID == nil) ||
			(item.ParentID != nil && it.ParentID != nil && *item.ParentID == *it.ParentID)
		if sameParent {
			siblings = append(siblings, it)
		}
	}

	// Find adjacent item
	var adjacentID int64
	for i, sib := range siblings {
		if sib.ID == itemID {
			if direction == "up" && i > 0 {
				adjacentID = siblings[i-1].ID
			} else if direction == "down" && i < len(siblings)-1 {
				adjacentID = siblings[i+1].ID
			}
			break
		}
	}

	if adjacentID > 0 {
		if err := h.menuStore.SwapSortOrder(r.Context(), itemID, adjacentID); err != nil {
			return err
		}
		web.SetFlashSuccess(h.sm, r.Context(), "Reihenfolge geändert")
	}

	redirect := fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// buildFlatTree converts a flat list of menu items into a depth-annotated flat list.
func buildFlatTree(items []menu.MenuItem) []menuItemNode {
	// Build parent->children map
	childrenOf := make(map[int64][]menu.MenuItem)
	var roots []menu.MenuItem
	for _, item := range items {
		if item.ParentID == nil {
			roots = append(roots, item)
		} else {
			childrenOf[*item.ParentID] = append(childrenOf[*item.ParentID], item)
		}
	}

	var result []menuItemNode
	var walk func(items []menu.MenuItem, depth int)
	walk = func(items []menu.MenuItem, depth int) {
		for _, item := range items {
			result = append(result, menuItemNode{MenuItem: item, Depth: depth})
			if children, ok := childrenOf[item.ID]; ok {
				walk(children, depth+1)
			}
		}
	}
	walk(roots, 0)
	return result
}

// isValidLocationKey checks that a location key contains only lowercase, digits, hyphens.
func isValidLocationKey(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}
