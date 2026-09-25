package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// Menus, terms, snippets, redirects, content kinds, fields, block kinds.
//
// The permission split is the web admin's: what an editor reaches there —
// menus, terms, snippets, redirects — a content key may change here, and what
// sits behind requireAdmin there — content kinds, fields, block kinds — is
// Admin here. The structure of a website is the operator's decision; an
// assistant working for an editor should not be able to give every page a new
// required field.
//
// Every write goes through the admin handler (internal/admin/ops_structure.go),
// which shares its checks with the screen. Nothing here decides what a valid
// menu key or snippet key is; it only translates between the wire and those
// functions, and checks first that the key may touch the website at all.

// The activity log's names for what these tools do. Local rather than in
// internal/activity, where the screens' names stand: the screens do not log
// these changes (yet), and the day they do the names can move there unchanged.
const (
	actMenuCreate       = "menu.create"
	actMenuUpdate       = "menu.update"
	actMenuDelete       = "menu.delete"
	actTermRename       = "term.rename"
	actTermDelete       = "term.delete"
	actSnippetCreate    = "snippet.create"
	actSnippetUpdate    = "snippet.update"
	actSnippetDelete    = "snippet.delete"
	actRedirectAdd      = "redirect.add"
	actRedirectDelete   = "redirect.delete"
	actKindSave         = "kind.save"
	actKindDelete       = "kind.delete"
	actFieldSave        = "field.save"
	actFieldDelete      = "field.delete"
	actBlockKindSave    = "blockkind.save"
	actBlockKindDelete  = "blockkind.delete"
	actStructureReorder = "structure.reorder"
)

// BrokenLink is an internal link on a page that leads nowhere — one row of the
// report check_links hands back. Declared here because the admin handler that
// builds it imports this package and not the other way round.
type BrokenLink struct {
	PageID    int64
	PageTitle string
	PageSlug  string
	Target    string
}

// The admin handler's methods, one interface per part so a build without one
// part loses those tools only.
type menuOps interface {
	OpMenus(ctx context.Context, websiteID int64) ([]menu.Menu, error)
	OpMenu(ctx context.Context, websiteID, menuID int64) (*menu.Menu, []menu.MenuNode, error)
	OpCreateMenu(ctx context.Context, websiteID int64, name, locationKey, language string) (*menu.Menu, error)
	OpUpdateMenu(ctx context.Context, websiteID, menuID int64, name, locationKey string) error
	OpSetMenuItems(ctx context.Context, websiteID, menuID int64, items []menu.ItemInput) error
	OpDeleteMenu(ctx context.Context, websiteID, menuID int64) error
}

type termOps interface {
	OpTerms(ctx context.Context, websiteID int64) ([]term.Term, error)
	OpRenameTerm(ctx context.Context, websiteID, termID int64, name string) error
	OpDeleteTerm(ctx context.Context, websiteID, termID int64) error
}

type snippetOps interface {
	OpSnippets(ctx context.Context, websiteID int64) ([]snippet.Snippet, map[string]int, error)
	OpSnippet(ctx context.Context, websiteID, id int64) (*snippet.Snippet, []field.Def, error)
	OpSaveSnippet(ctx context.Context, websiteID, id int64, key, name, markdown string, fields field.Data) (*snippet.Snippet, error)
	OpDeleteSnippet(ctx context.Context, websiteID, id int64) error
}

type redirectOps interface {
	OpRedirects(ctx context.Context, websiteID int64) ([]page.Redirect, error)
	OpAddRedirect(ctx context.Context, websiteID int64, from, to string, code int) (string, string, error)
	OpDeleteRedirect(ctx context.Context, websiteID, id int64) error
	OpBrokenLinks(ctx context.Context, websiteID int64) ([]BrokenLink, error)
}

type kindOps interface {
	OpKinds(ctx context.Context, websiteID int64) ([]kind.Type, map[string]int, error)
	OpSaveKind(ctx context.Context, websiteID int64, t kind.Type) (kind.Type, error)
	OpDeleteKind(ctx context.Context, websiteID, id int64) (kind.Type, int, error)
	OpMoveKind(ctx context.Context, websiteID, id int64, up bool) error
}

type fieldOps interface {
	OpField(ctx context.Context, websiteID, id int64) (*field.Def, error)
	OpSaveField(ctx context.Context, websiteID, id int64, def field.Def) (*field.Def, error)
	OpDeleteField(ctx context.Context, websiteID, id int64) error
	OpMoveField(ctx context.Context, websiteID, id int64, up bool) error
}

type blockKindOps interface {
	OpBlockTypes(ctx context.Context, websiteID int64) ([]block.Own, map[string]int, error)
	OpSaveBlockType(ctx context.Context, websiteID, id int64, name, hint string) (*block.Own, error)
	OpDeleteBlockType(ctx context.Context, websiteID, id int64) error
	OpMoveBlockType(ctx context.Context, websiteID, id int64, up bool) error
}

// structOps asserts the handler to the part a tool needs.
func structOps[T any](d Deps) (T, error) {
	ops, ok := d.Ops.(T)
	if !ok {
		var none T
		return none, errors.New("this function is not available in this installation")
	}
	return ops, nil
}

func structureTools(d Deps) []Tool {
	return []Tool{
		listMenus(d), getMenu(d), createMenu(d), updateMenu(d), deleteMenu(d),
		listTerms(d), renameTerm(d), deleteTerm(d),
		listSnippets(d), getSnippet(d), createSnippet(d), updateSnippet(d), deleteSnippet(d),
		listRedirects(d), createRedirect(d), deleteRedirect(d), checkLinks(d),
		listKinds(d), createKind(d), updateKind(d), deleteKind(d), moveKind(d),
		createField(d), updateField(d), deleteField(d), moveField(d),
		listBlockKinds(d), createBlockKind(d), updateBlockKind(d), deleteBlockKind(d), moveBlockKind(d),
	}
}

// --- shared -----------------------------------------------------------------

var (
	websiteProp = Property{Type: "integer", Description: "id of the website"}
	confirmProp = Property{Type: "boolean", Description: "must be true: this cannot be undone, " +
		"so ask the person before you set it"}
	directionProp = Property{Type: "string", Description: "up or down, one place",
		Enum: []string{"up", "down"}}
)

// needConfirm is the bar in front of every delete: an assistant that deletes
// because a sentence could be read that way has to say so out loud first.
func needConfirm(confirm bool, what string) error {
	if !confirm {
		return fmt.Errorf("deleting %s cannot be undone: ask the person, then call again with confirm: true", what)
	}
	return nil
}

// withWebsite reads the arguments into args and checks that the key may touch
// the website they name — website points into args, at that id.
func withWebsite(c Call, args any, website *int64) error {
	if err := c.Into(args); err != nil {
		return err
	}
	if *website <= 0 {
		return errors.New("give the id of the website")
	}
	return c.Scope.MaySee(*website)
}

// --- menus ------------------------------------------------------------------

// menuItemWire is one entry of a menu as an assistant writes it.
type menuItemWire struct {
	Label    string         `json:"label"`
	Page     int64          `json:"page"`
	URL      string         `json:"url"`
	Children []menuItemWire `json:"children"`
}

// menuItemProp describes one entry; children repeat the same shape. The schema
// stops one level down rather than referring to itself, which the small schema
// here cannot express — the description says the rest.
var menuItemProp = Property{
	Type: "object",
	Description: "one entry: a label and either page (the id of a page of this website) or url " +
		"(an address such as https://… or /contact); with neither it is a plain heading without a link",
	Properties: map[string]Property{
		"label": {Type: "string", Description: "the text shown in the menu"},
		"page":  {Type: "integer", Description: "id of a page of this website to link to"},
		"url":   {Type: "string", Description: "an address to link to, instead of a page"},
		"children": {Type: "array", Description: "entries below this one, in the same shape; " +
			"at most three levels in all", Items: &Property{Type: "object"}},
	},
	Required: []string{"label"},
}

func menuInput(in []menuItemWire) ([]menu.ItemInput, error) {
	out := make([]menu.ItemInput, 0, len(in))
	for _, w := range in {
		it := menu.ItemInput{Title: w.Label}
		switch {
		case w.Page > 0 && strings.TrimSpace(w.URL) != "":
			return nil, fmt.Errorf("the entry %q names both a page and an address; give one", w.Label)
		case w.Page > 0:
			id := w.Page
			it.ItemType, it.PageID = "page", &id
		case strings.TrimSpace(w.URL) != "":
			it.ItemType, it.URL = "url", w.URL
		default:
			it.ItemType = "custom"
		}
		children, err := menuInput(w.Children)
		if err != nil {
			return nil, err
		}
		it.Children = children
		out = append(out, it)
	}
	return out, nil
}

func menuOut(nodes []menu.MenuNode) []map[string]any {
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		e := map[string]any{"id": n.ID, "label": n.Title}
		switch n.ItemType {
		case "page":
			e["type"] = "page"
			if n.PageID != nil {
				e["page"] = *n.PageID
			}
			if n.PageSlug != "" {
				e["page_slug"] = n.PageSlug
			} else {
				// Otherwise the entry reads as a link and is plain text on the
				// website, and nobody can tell why.
				e["note"] = "The page of this entry no longer exists; it shows as plain text."
			}
		case "url":
			e["type"], e["url"] = "url", n.URL
		default:
			e["type"] = "text"
		}
		if len(n.Children) > 0 {
			e["children"] = menuOut(n.Children)
		}
		out = append(out, e)
	}
	return out
}

func menuBrief(m menu.Menu) map[string]any {
	out := map[string]any{"id": m.ID, "name": m.Name, "key": m.LocationKey}
	if m.Locale != "" {
		out["language"] = m.Locale
	}
	return out
}

func listMenus(d Deps) Tool {
	return Tool{
		Name: "list_menus",
		Description: "Lists the menus of a website: id, name, the key a theme places it by " +
			"(main, footer, …) and, on a multilingual website, its language.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"website": websiteProp}, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[menuOps](d)
			if err != nil {
				return nil, err
			}
			list, err := ops.OpMenus(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, m := range list {
				out = append(out, menuBrief(m))
			}
			return map[string]any{"menus": out}, nil
		},
	}
}

func getMenu(d Deps) Tool {
	return Tool{
		Name:        "get_menu",
		Description: "Fetches one menu with all its entries as a tree.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"menu":    {Type: "integer", Description: "id of the menu"},
			}, Required: []string{"website", "menu"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Menu    int64 `json:"menu"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[menuOps](d)
			if err != nil {
				return nil, err
			}
			m, tree, err := ops.OpMenu(c.Ctx, a.Website, a.Menu)
			if err != nil {
				return nil, err
			}
			if m == nil {
				return nil, errors.New("there is no such menu on this website")
			}
			out := menuBrief(*m)
			out["items"] = menuOut(tree)
			return out, nil
		},
	}
}

func createMenu(d Deps) Tool {
	return Tool{
		Name:   "create_menu",
		Writes: true,
		Description: "Creates a menu, optionally with its entries. The key is what the theme places " +
			"the menu by — usually main or footer; lower-case letters, digits and hyphens.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":  websiteProp,
				"name":     {Type: "string", Description: "name of the menu, for the admin"},
				"key":      {Type: "string", Description: "where the theme shows it, such as main or footer"},
				"language": {Type: "string", Description: "a language tag such as fr on a multilingual website; empty means the main language"},
				"items":    {Type: "array", Description: "the entries, in order", Items: &menuItemProp},
			}, Required: []string{"website", "name", "key"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64          `json:"website"`
				Name     string         `json:"name"`
				Key      string         `json:"key"`
				Language string         `json:"language"`
				Items    []menuItemWire `json:"items"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[menuOps](d)
			if err != nil {
				return nil, err
			}
			items, err := menuInput(a.Items)
			if err != nil {
				return nil, err
			}
			m, err := ops.OpCreateMenu(c.Ctx, a.Website, a.Name, a.Key, a.Language)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actMenuCreate, EntityType: "menu", EntityID: m.ID})
			var note string
			if strings.TrimSpace(a.Language) != "" && m.Locale == "" {
				note = "The language " + strings.TrimSpace(a.Language) + " is not switched on for " +
					"this website, so the menu belongs to the main language."
			}
			if len(items) > 0 {
				if err := ops.OpSetMenuItems(c.Ctx, a.Website, m.ID, items); err != nil {
					// The menu exists; say so, or the next attempt fails on
					// the key and the assistant does not know why.
					return nil, fmt.Errorf("the menu was created (id %d) but its entries were refused: %w", m.ID, err)
				}
			}
			_, tree, err := ops.OpMenu(c.Ctx, a.Website, m.ID)
			if err != nil {
				return nil, err
			}
			out := menuBrief(*m)
			out["items"] = menuOut(tree)
			if note != "" {
				out["note"] = note
			}
			return out, nil
		},
	}
}

func updateMenu(d Deps) Tool {
	return Tool{
		Name:   "update_menu",
		Writes: true,
		Description: "Changes a menu's name or key and/or replaces its entries. Given items replace " +
			"ALL existing entries — read the menu with get_menu first and send the whole list " +
			"back with your change in it. Without items the entries stay as they are.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"menu":    {Type: "integer", Description: "id of the menu"},
				"name":    {Type: "string", Description: "new name; without one the old stays"},
				"key":     {Type: "string", Description: "new key; without one the old stays"},
				"items":   {Type: "array", Description: "the complete new list of entries, in order", Items: &menuItemProp},
			}, Required: []string{"website", "menu"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64           `json:"website"`
				Menu    int64           `json:"menu"`
				Name    *string         `json:"name"`
				Key     *string         `json:"key"`
				Items   *[]menuItemWire `json:"items"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[menuOps](d)
			if err != nil {
				return nil, err
			}
			m, _, err := ops.OpMenu(c.Ctx, a.Website, a.Menu)
			if err != nil {
				return nil, err
			}
			if m == nil {
				return nil, errors.New("there is no such menu on this website")
			}
			if a.Name != nil || a.Key != nil {
				name, key := m.Name, m.LocationKey
				if a.Name != nil {
					name = *a.Name
				}
				if a.Key != nil {
					key = *a.Key
				}
				if err := ops.OpUpdateMenu(c.Ctx, a.Website, m.ID, name, key); err != nil {
					return nil, err
				}
			}
			if a.Items != nil {
				items, err := menuInput(*a.Items)
				if err != nil {
					return nil, err
				}
				if err := ops.OpSetMenuItems(c.Ctx, a.Website, m.ID, items); err != nil {
					return nil, err
				}
			}
			changed(d, c, a.Website, activity.Entry{Action: actMenuUpdate, EntityType: "menu", EntityID: m.ID})
			after, tree, err := ops.OpMenu(c.Ctx, a.Website, m.ID)
			if err != nil || after == nil {
				return nil, err
			}
			out := menuBrief(*after)
			out["items"] = menuOut(tree)
			return out, nil
		},
	}
}

func deleteMenu(d Deps) Tool {
	return Tool{
		Name:   "delete_menu",
		Writes: true,
		Description: "Deletes a menu with all its entries. The theme shows nothing in its place. " +
			"Requires confirm: true.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"menu":    {Type: "integer", Description: "id of the menu"},
				"confirm": confirmProp,
			}, Required: []string{"website", "menu", "confirm"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Menu    int64 `json:"menu"`
				Confirm bool  `json:"confirm"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			if err := needConfirm(a.Confirm, "a menu"); err != nil {
				return nil, err
			}
			ops, err := structOps[menuOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteMenu(c.Ctx, a.Website, a.Menu); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actMenuDelete, EntityType: "menu", EntityID: a.Menu})
			return map[string]any{"deleted": a.Menu}, nil
		},
	}
}

// --- terms ------------------------------------------------------------------

func listTerms(d Deps) Tool {
	return Tool{
		Name: "list_terms",
		Description: "Lists the terms (tags) of a website with their address and on how many " +
			"entries each is used, drafts included. Useful to find two spellings of one term. " +
			"A term comes into being by tagging a page; there is no separate way to create one.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"website": websiteProp}, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[termOps](d)
			if err != nil {
				return nil, err
			}
			list, err := ops.OpTerms(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, t := range list {
				out = append(out, map[string]any{
					"id": t.ID, "name": t.Name, "slug": t.Slug, "url": t.URL(), "used": t.Count,
				})
			}
			return map[string]any{"terms": out}, nil
		},
	}
}

func renameTerm(d Deps) Tool {
	return Tool{
		Name:        "rename_term",
		Writes:      true,
		Description: "Renames a term. Its address stays as it was, so existing links keep working.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"term":    {Type: "integer", Description: "id of the term"},
				"name":    {Type: "string", Description: "the new name"},
			}, Required: []string{"website", "term", "name"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Term    int64  `json:"term"`
				Name    string `json:"name"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[termOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpRenameTerm(c.Ctx, a.Website, a.Term, a.Name); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actTermRename, EntityType: "term", EntityID: a.Term})
			return map[string]any{"id": a.Term, "name": strings.Join(strings.Fields(a.Name), " "),
				"note": "Renamed. The address stays as it was."}, nil
		},
	}
}

func deleteTerm(d Deps) Tool {
	return Tool{
		Name:   "delete_term",
		Writes: true,
		Description: "Deletes a term and takes it off every entry that carries it. The entries " +
			"themselves stay. Requires confirm: true.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"term":    {Type: "integer", Description: "id of the term"},
				"confirm": confirmProp,
			}, Required: []string{"website", "term", "confirm"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Term    int64 `json:"term"`
				Confirm bool  `json:"confirm"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			if err := needConfirm(a.Confirm, "a term"); err != nil {
				return nil, err
			}
			ops, err := structOps[termOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteTerm(c.Ctx, a.Website, a.Term); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actTermDelete, EntityType: "term", EntityID: a.Term})
			return map[string]any{"deleted": a.Term}, nil
		},
	}
}

// --- snippets ---------------------------------------------------------------

func snippetBrief(sn snippet.Snippet) map[string]any {
	return map[string]any{
		"id": sn.ID, "key": sn.Key, "name": sn.Name,
		// The marker is what goes into a page's text; an assistant told the key
		// alone has to guess the brackets.
		"marker":  "[[snippet:" + sn.Key + "]]",
		"updated": sn.UpdatedAt.UTC().Format(timeLayout),
	}
}

func listSnippets(d Deps) Tool {
	return Tool{
		Name: "list_snippets",
		Description: "Lists the snippets of a website: reusable pieces of text placed into a page " +
			"with their marker, such as [[snippet:opening-hours]]. With how many live pages use each.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"website": websiteProp}, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[snippetOps](d)
			if err != nil {
				return nil, err
			}
			list, used, err := ops.OpSnippets(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, sn := range list {
				e := snippetBrief(sn)
				e["used_on_pages"] = used[sn.Key]
				out = append(out, e)
			}
			return map[string]any{"snippets": out}, nil
		},
	}
}

func getSnippet(d Deps) Tool {
	return Tool{
		Name: "get_snippet",
		Description: "Fetches one snippet with its markdown, the values of its own fields and " +
			"the definitions of those fields.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"snippet": {Type: "integer", Description: "id of the snippet"},
			}, Required: []string{"website", "snippet"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Snippet int64 `json:"snippet"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[snippetOps](d)
			if err != nil {
				return nil, err
			}
			sn, defs, err := ops.OpSnippet(c.Ctx, a.Website, a.Snippet)
			if err != nil {
				return nil, err
			}
			if sn == nil {
				return nil, errors.New("there is no such snippet on this website")
			}
			out := snippetBrief(*sn)
			out["markdown"] = sn.ContentMarkdown
			data := field.Decode(sn.Fields)
			if len(data.Values) > 0 {
				out["fields"] = map[string]string(data.Values)
			}
			if len(data.Rows) > 0 {
				out["groups"] = data.Rows
			}
			out["field_definitions"] = fieldDefsOut(defs)
			return out, nil
		},
	}
}

func createSnippet(d Deps) Tool {
	return Tool{
		Name:   "create_snippet",
		Writes: true,
		Description: "Creates a snippet. The key is what its marker carries: lower-case letters, " +
			"digits, hyphen and underscore. A new snippet has no own fields yet; an admin key " +
			"adds them with create_field and the snippet's id.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":  websiteProp,
				"key":      {Type: "string", Description: "the key, such as opening-hours"},
				"name":     {Type: "string", Description: "name of the snippet, for the admin"},
				"markdown": {Type: "string", Description: "the text in markdown"},
			}, Required: []string{"website", "key", "name"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64  `json:"website"`
				Key      string `json:"key"`
				Name     string `json:"name"`
				Markdown string `json:"markdown"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[snippetOps](d)
			if err != nil {
				return nil, err
			}
			sn, err := ops.OpSaveSnippet(c.Ctx, a.Website, 0, a.Key, a.Name, a.Markdown, field.Data{})
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actSnippetCreate, EntityType: "snippet", EntityID: sn.ID})
			return snippetBrief(*sn), nil
		},
	}
}

func updateSnippet(d Deps) Tool {
	return Tool{
		Name:   "update_snippet",
		Writes: true,
		Description: "Changes a snippet: key, name, markdown, and the values of its own fields. " +
			"What is not given stays. Changing the key breaks every marker that uses the old one.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":  websiteProp,
				"snippet":  {Type: "integer", Description: "id of the snippet"},
				"key":      {Type: "string", Description: "new key"},
				"name":     {Type: "string", Description: "new name"},
				"markdown": {Type: "string", Description: "new text in markdown"},
				"fields": {Type: "object", Description: "the snippet's own fields as {\"key\": \"value\"}; " +
					"given ones are changed, the others stay"},
				"groups": {Type: "object", Description: "repeatable groups as {\"key\": [{\"subfield\": \"value\"}, …]}; " +
					"a group that is given replaces its rows entirely"},
			}, Required: []string{"website", "snippet"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64                     `json:"website"`
				Snippet  int64                     `json:"snippet"`
				Key      *string                   `json:"key"`
				Name     *string                   `json:"name"`
				Markdown *string                   `json:"markdown"`
				Fields   field.Values              `json:"fields"`
				Groups   map[string][]field.Values `json:"groups"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[snippetOps](d)
			if err != nil {
				return nil, err
			}
			sn, _, err := ops.OpSnippet(c.Ctx, a.Website, a.Snippet)
			if err != nil {
				return nil, err
			}
			if sn == nil {
				return nil, errors.New("there is no such snippet on this website")
			}
			key, name, markdown := sn.Key, sn.Name, sn.ContentMarkdown
			if a.Key != nil {
				key = *a.Key
			}
			if a.Name != nil {
				name = *a.Name
			}
			if a.Markdown != nil {
				markdown = *a.Markdown
			}
			// The store writes the fields whole, so what is not given is
			// carried forward — the same merge update_page makes.
			data := field.Decode(sn.Fields)
			for k, v := range a.Fields {
				data.Values[k] = v
			}
			for k, rows := range a.Groups {
				data.Rows[k] = rows
			}
			saved, err := ops.OpSaveSnippet(c.Ctx, a.Website, sn.ID, key, name, markdown, data)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actSnippetUpdate, EntityType: "snippet", EntityID: sn.ID})
			out := snippetBrief(*saved)
			if saved.Key != sn.Key {
				out["note"] = "The key changed: pages still carrying [[snippet:" + sn.Key +
					"]] now show nothing there."
			}
			return out, nil
		},
	}
}

func deleteSnippet(d Deps) Tool {
	return Tool{
		Name:   "delete_snippet",
		Writes: true,
		Description: "Deletes a snippet. Pages carrying its marker show nothing in its place. " +
			"Requires confirm: true.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"snippet": {Type: "integer", Description: "id of the snippet"},
				"confirm": confirmProp,
			}, Required: []string{"website", "snippet", "confirm"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Snippet int64 `json:"snippet"`
				Confirm bool  `json:"confirm"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			if err := needConfirm(a.Confirm, "a snippet"); err != nil {
				return nil, err
			}
			ops, err := structOps[snippetOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteSnippet(c.Ctx, a.Website, a.Snippet); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actSnippetDelete, EntityType: "snippet", EntityID: a.Snippet})
			return map[string]any{"deleted": a.Snippet}, nil
		},
	}
}

// --- redirects and links ----------------------------------------------------

func listRedirects(d Deps) Tool {
	return Tool{
		Name: "list_redirects",
		Description: "Lists the redirects of a website: old address, target, code (301 permanent, " +
			"302 temporary), whether a rename wrote it or a person, and how often it was followed.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"website": websiteProp}, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[redirectOps](d)
			if err != nil {
				return nil, err
			}
			list, err := ops.OpRedirects(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, r := range list {
				out = append(out, map[string]any{
					"id": r.ID, "from": r.FromPath, "to": r.ToPath, "code": r.Code,
					"source": r.Source, "hits": r.Hits,
				})
			}
			return map[string]any{"redirects": out}, nil
		},
	}
}

func createRedirect(d Deps) Tool {
	return Tool{
		Name:   "create_redirect",
		Writes: true,
		Description: "Sends visitors of an old address to a new one. Full URLs are cut down to their " +
			"path. An existing redirect from the same address is replaced. Permanent (301) " +
			"unless temporary is true.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":   websiteProp,
				"from":      {Type: "string", Description: "the old address, such as /kontakt.html"},
				"to":        {Type: "string", Description: "where it should lead, such as /contact"},
				"temporary": {Type: "boolean", Description: "true for a temporary redirect (302)"},
			}, Required: []string{"website", "from", "to"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64  `json:"website"`
				From      string `json:"from"`
				To        string `json:"to"`
				Temporary bool   `json:"temporary"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[redirectOps](d)
			if err != nil {
				return nil, err
			}
			code := 301
			if a.Temporary {
				code = 302
			}
			from, to, err := ops.OpAddRedirect(c.Ctx, a.Website, a.From, a.To, code)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actRedirectAdd, EntityType: "redirect",
				Metadata: map[string]any{"from": from, "to": to}})
			return map[string]any{"from": from, "to": to, "code": code}, nil
		},
	}
}

func deleteRedirect(d Deps) Tool {
	return Tool{
		Name:        "delete_redirect",
		Writes:      true,
		Description: "Deletes a redirect; its old address answers 404 again. Requires confirm: true.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":  websiteProp,
				"redirect": {Type: "integer", Description: "id of the redirect"},
				"confirm":  confirmProp,
			}, Required: []string{"website", "redirect", "confirm"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64 `json:"website"`
				Redirect int64 `json:"redirect"`
				Confirm  bool  `json:"confirm"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			if err := needConfirm(a.Confirm, "a redirect"); err != nil {
				return nil, err
			}
			ops, err := structOps[redirectOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteRedirect(c.Ctx, a.Website, a.Redirect); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actRedirectDelete, EntityType: "redirect", EntityID: a.Redirect})
			return map[string]any{"deleted": a.Redirect}, nil
		},
	}
}

func checkLinks(d Deps) Tool {
	return Tool{
		Name: "check_links",
		Description: "Reports the internal links on a website's pages that lead nowhere: which page " +
			"carries the link and where it points. Fix one by changing the page or with create_redirect.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"website": websiteProp}, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[redirectOps](d)
			if err != nil {
				return nil, err
			}
			list, err := ops.OpBrokenLinks(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, b := range list {
				out = append(out, map[string]any{
					"page": b.PageID, "page_title": b.PageTitle, "page_slug": b.PageSlug, "target": b.Target,
				})
			}
			return map[string]any{"broken_links": out}, nil
		},
	}
}

// --- content kinds (admin) --------------------------------------------------

// The sort orders as an assistant names them, and as they are stored.
func kindSortToWire(s string) string {
	if s == kind.SortTitle {
		return "title"
	}
	return "newest"
}

func kindSortFromWire(s string) string {
	if s == "title" || s == kind.SortTitle {
		return kind.SortTitle
	}
	return kind.SortNewest
}

func kindOut(t kind.Type, entries int) map[string]any {
	out := map[string]any{
		"id": t.ID, "key": t.Key, "name": t.Name, "plural": t.Plural,
		"sort": kindSortToWire(t.Sort), "entries": entries,
	}
	if t.Archive != "" {
		out["archive"] = "/" + t.Archive
	}
	return out
}

var kindProps = map[string]Property{
	"name":    {Type: "string", Description: "the singular, such as Product"},
	"plural":  {Type: "string", Description: "the plural, such as Products"},
	"archive": {Type: "string", Description: "address of the overview page listing all entries, such as products; empty for none"},
	"sort": {Type: "string", Description: "order of the overview: newest first or by title",
		Enum: []string{"newest", "title"}},
}

func withProps(base map[string]Property, extra map[string]Property) map[string]Property {
	out := make(map[string]Property, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func listKinds(d Deps) Tool {
	return Tool{
		Name:  "list_kinds",
		Admin: true,
		Description: "Lists a website's own content kinds (beside the built-in page and post), such " +
			"as products or events, with how many entries each holds. The key is what list_pages " +
			"reports as the type.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"website": websiteProp}, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[kindOps](d)
			if err != nil {
				return nil, err
			}
			list, counts, err := ops.OpKinds(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, t := range list {
				out = append(out, kindOut(t, counts[t.Key]))
			}
			return map[string]any{"kinds": out}, nil
		},
	}
}

func createKind(d Deps) Tool {
	return Tool{
		Name:   "create_kind",
		Writes: true,
		Admin:  true,
		Description: "Creates a content kind of its own, such as products. Its key is made from the " +
			"name and cannot change afterwards.",
		InputSchema: Schema{Type: "object",
			Properties: withProps(kindProps, map[string]Property{"website": websiteProp}),
			Required:   []string{"website", "name", "plural"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Name    string `json:"name"`
				Plural  string `json:"plural"`
				Archive string `json:"archive"`
				Sort    string `json:"sort"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[kindOps](d)
			if err != nil {
				return nil, err
			}
			t, err := ops.OpSaveKind(c.Ctx, a.Website, kind.Type{
				Name: a.Name, Plural: a.Plural, Archive: strings.Trim(a.Archive, "/ "),
				Sort: kindSortFromWire(a.Sort),
			})
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actKindSave, EntityType: "kind", EntityID: t.ID})
			return kindOut(t, 0), nil
		},
	}
}

func updateKind(d Deps) Tool {
	return Tool{
		Name:        "update_kind",
		Writes:      true,
		Admin:       true,
		Description: "Changes a content kind's name, plural, overview address or order. The key stays.",
		InputSchema: Schema{Type: "object",
			Properties: withProps(kindProps, map[string]Property{
				"website": websiteProp,
				"kind":    {Type: "integer", Description: "id of the content kind"},
			}),
			Required: []string{"website", "kind"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64   `json:"website"`
				Kind    int64   `json:"kind"`
				Name    *string `json:"name"`
				Plural  *string `json:"plural"`
				Archive *string `json:"archive"`
				Sort    *string `json:"sort"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[kindOps](d)
			if err != nil {
				return nil, err
			}
			list, counts, err := ops.OpKinds(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			var t *kind.Type
			for i := range list {
				if list[i].ID == a.Kind {
					t = &list[i]
				}
			}
			if t == nil {
				return nil, errors.New("there is no such content kind on this website")
			}
			if a.Name != nil {
				t.Name = *a.Name
			}
			if a.Plural != nil {
				t.Plural = *a.Plural
			}
			if a.Archive != nil {
				t.Archive = strings.Trim(*a.Archive, "/ ")
			}
			if a.Sort != nil {
				t.Sort = kindSortFromWire(*a.Sort)
			}
			saved, err := ops.OpSaveKind(c.Ctx, a.Website, *t)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actKindSave, EntityType: "kind", EntityID: saved.ID})
			return kindOut(saved, counts[saved.Key]), nil
		},
	}
}

func deleteKind(d Deps) Tool {
	return Tool{
		Name:   "delete_kind",
		Writes: true,
		Admin:  true,
		Description: "Deletes a content kind. Its entries stay and keep its key; they can be switched " +
			"to another kind, or the kind created again. Requires confirm: true.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"kind":    {Type: "integer", Description: "id of the content kind"},
				"confirm": confirmProp,
			}, Required: []string{"website", "kind", "confirm"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Kind    int64 `json:"kind"`
				Confirm bool  `json:"confirm"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			if err := needConfirm(a.Confirm, "a content kind"); err != nil {
				return nil, err
			}
			ops, err := structOps[kindOps](d)
			if err != nil {
				return nil, err
			}
			t, n, err := ops.OpDeleteKind(c.Ctx, a.Website, a.Kind)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actKindDelete, EntityType: "kind", EntityID: t.ID,
				Metadata: map[string]any{"key": t.Key}})
			out := map[string]any{"deleted": t.ID, "key": t.Key}
			if n > 0 {
				out["note"] = fmt.Sprintf("%d entries still carry the key %q. Switch them to another "+
					"kind, or create the kind again.", n, t.Key)
			}
			return out, nil
		},
	}
}

func moveKind(d Deps) Tool {
	return Tool{
		Name:        "move_kind",
		Writes:      true,
		Admin:       true,
		Description: "Moves a content kind one place up or down in the order the admin offers them.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":   websiteProp,
				"kind":      {Type: "integer", Description: "id of the content kind"},
				"direction": directionProp,
			}, Required: []string{"website", "kind", "direction"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64  `json:"website"`
				Kind      int64  `json:"kind"`
				Direction string `json:"direction"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[kindOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpMoveKind(c.Ctx, a.Website, a.Kind, a.Direction == "up"); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actStructureReorder, EntityType: "kind", EntityID: a.Kind})
			return map[string]any{"moved": a.Kind, "direction": a.Direction}, nil
		},
	}
}

// --- fields (admin) ---------------------------------------------------------

// fieldKindsText lists the kinds of field for a description, generated from
// the one list so it cannot fall behind.
func fieldKindsText() string {
	parts := make([]string, 0, len(field.Kinds))
	for _, k := range field.Kinds {
		parts = append(parts, k.Kind+" ("+k.Name+")")
	}
	return strings.Join(parts, ", ")
}

func fieldKindEnum() []string {
	out := make([]string, 0, len(field.Kinds))
	for _, k := range field.Kinds {
		out = append(out, k.Kind)
	}
	return out
}

// appliesFromWire accepts the English words and the stored ones.
func appliesFromWire(s string) string {
	switch strings.TrimSpace(s) {
	case "", "both", field.ForBoth:
		return field.ForBoth
	case "page", field.ForPage:
		return field.ForPage
	case "post", field.ForPost:
		return field.ForPost
	}
	return strings.TrimSpace(s)
}

func displayFromWire(s string) string {
	switch strings.TrimSpace(s) {
	case "buttons", field.DisplayButtons:
		return field.DisplayButtons
	}
	return ""
}

// fieldDefsOut describes definitions with their ids, which the writing tools
// need, and the same particulars list_fields gives.
func fieldDefsOut(defs []field.Def) []map[string]any {
	out := make([]map[string]any, 0, len(defs))
	for _, def := range defs {
		e := map[string]any{
			"id": def.ID, "key": def.Key, "label": def.Label, "kind": def.Kind, "required": def.Required,
		}
		if def.Hint != "" {
			e["hint"] = def.Hint
		}
		if len(def.Choices) > 0 {
			e["choices"] = def.Choices
		}
		fieldProperties(e, def)
		if len(def.Sub) > 0 {
			e["subfields"] = fieldDefsOut(def.Sub)
		}
		out = append(out, e)
	}
	return out
}

var fieldProps = map[string]Property{
	"label":    {Type: "string", Description: "the label; on creating, the key is made from it and never changes"},
	"required": {Type: "boolean", Description: "whether a page cannot be saved without it"},
	"hint":     {Type: "string", Description: "a line of help under the input"},
	"choices": {Type: "array", Description: "the options of a choice or multiple choice",
		Items: &Property{Type: "string"}},
	"applies_to": {Type: "string", Description: "which entries carry the field: both, page, post, " +
		"or the key of an own content kind; only for a field of the page itself"},
	"condition": {Type: "string", Description: "key of another field; this one is only asked for " +
		"once that one is filled in"},
	"presentation": {Type: "string", Description: "for a choice: buttons for a row of buttons, " +
		"empty for a drop-down"},
	"max_values": {Type: "integer", Description: "for a multiple choice: how many may be picked; 0 for no limit"},
	"min_value":  {Type: "string", Description: "for a range: the lower bound, empty for none"},
	"max_value":  {Type: "string", Description: "for a range: the upper bound, empty for none"},
}

// fieldWire is what an assistant sends for a field. Pointers, so an update
// can tell "not given" from "emptied".
type fieldWire struct {
	Label        *string   `json:"label"`
	Kind         *string   `json:"kind"`
	Required     *bool     `json:"required"`
	Hint         *string   `json:"hint"`
	Choices      *[]string `json:"choices"`
	AppliesTo    *string   `json:"applies_to"`
	Condition    *string   `json:"condition"`
	Presentation *string   `json:"presentation"`
	MaxValues    *int      `json:"max_values"`
	MinValue     *string   `json:"min_value"`
	MaxValue     *string   `json:"max_value"`
}

// apply writes what was given over def.
func (w fieldWire) apply(def *field.Def) {
	if w.Label != nil {
		def.Label = *w.Label
	}
	if w.Kind != nil {
		def.Kind = *w.Kind
	}
	if w.Required != nil {
		def.Required = *w.Required
	}
	if w.Hint != nil {
		def.Hint = *w.Hint
	}
	if w.Choices != nil {
		def.Choices = *w.Choices
	}
	if w.AppliesTo != nil {
		def.AppliesTo = appliesFromWire(*w.AppliesTo)
	}
	if w.Condition != nil {
		def.Condition = strings.TrimSpace(*w.Condition)
	}
	if w.Presentation != nil {
		def.Display = displayFromWire(*w.Presentation)
	}
	if w.MaxValues != nil {
		def.MaxValues = *w.MaxValues
	}
	if w.MinValue != nil {
		def.RangeMin = strings.TrimSpace(*w.MinValue)
	}
	if w.MaxValue != nil {
		def.RangeMax = strings.TrimSpace(*w.MaxValue)
	}
}

func createField(d Deps) Tool {
	return Tool{
		Name:   "create_field",
		Writes: true,
		Admin:  true,
		Description: "Creates a field. Without a carrier it is a field of the website's pages " +
			"(list_fields shows those); with group it goes inside that group, with block_kind into " +
			"an own block kind, with snippet into a snippet. A field of kind gruppe is a group: " +
			"create it first, then its subfields with group set to its id. Kinds: " + fieldKindsText() + ".",
		InputSchema: Schema{Type: "object",
			Properties: withProps(fieldProps, map[string]Property{
				"website":    websiteProp,
				"kind":       {Type: "string", Description: "the kind of input", Enum: fieldKindEnum()},
				"group":      {Type: "integer", Description: "id of the group this field goes into"},
				"block_kind": {Type: "integer", Description: "id of the own block kind this field belongs to"},
				"snippet":    {Type: "integer", Description: "id of the snippet this field belongs to"},
			}),
			Required: []string{"website", "label", "kind"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64 `json:"website"`
				Group     int64 `json:"group"`
				BlockKind int64 `json:"block_kind"`
				Snippet   int64 `json:"snippet"`
				fieldWire
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[fieldOps](d)
			if err != nil {
				return nil, err
			}
			carriers := 0
			for _, id := range []int64{a.Group, a.BlockKind, a.Snippet} {
				if id > 0 {
					carriers++
				}
			}
			if carriers > 1 {
				return nil, errors.New("give at most one of group, block_kind and snippet")
			}
			def := field.Def{ParentID: a.Group, BlockTypeID: a.BlockKind, SnippetID: a.Snippet,
				AppliesTo: field.ForBoth}
			a.fieldWire.apply(&def)
			saved, err := ops.OpSaveField(c.Ctx, a.Website, 0, def)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actFieldSave, EntityType: "field", EntityID: saved.ID})
			return fieldDefsOut([]field.Def{*saved})[0], nil
		},
	}
}

func updateField(d Deps) Tool {
	return Tool{
		Name:   "update_field",
		Writes: true,
		Admin:  true,
		Description: "Changes a field's label, kind, choices and other particulars. What is not given " +
			"stays. The key never changes, and a group cannot become a plain field or back.",
		InputSchema: Schema{Type: "object",
			Properties: withProps(fieldProps, map[string]Property{
				"website": websiteProp,
				"field":   {Type: "integer", Description: "id of the field"},
				"kind":    {Type: "string", Description: "the kind of input", Enum: fieldKindEnum()},
			}),
			Required: []string{"website", "field"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Field   int64 `json:"field"`
				fieldWire
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[fieldOps](d)
			if err != nil {
				return nil, err
			}
			def, err := ops.OpField(c.Ctx, a.Website, a.Field)
			if err != nil {
				return nil, err
			}
			a.fieldWire.apply(def)
			def.Sub = nil
			saved, err := ops.OpSaveField(c.Ctx, a.Website, def.ID, *def)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actFieldSave, EntityType: "field", EntityID: saved.ID})
			return fieldDefsOut([]field.Def{*saved})[0], nil
		},
	}
}

func deleteField(d Deps) Tool {
	return Tool{
		Name:   "delete_field",
		Writes: true,
		Admin:  true,
		Description: "Deletes a field (a group with its subfields). The values stay on the pages " +
			"until each one is next saved; creating the field again with the same label brings " +
			"them back until then. Requires confirm: true.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"field":   {Type: "integer", Description: "id of the field"},
				"confirm": confirmProp,
			}, Required: []string{"website", "field", "confirm"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Field   int64 `json:"field"`
				Confirm bool  `json:"confirm"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			if err := needConfirm(a.Confirm, "a field"); err != nil {
				return nil, err
			}
			ops, err := structOps[fieldOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteField(c.Ctx, a.Website, a.Field); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actFieldDelete, EntityType: "field", EntityID: a.Field})
			return map[string]any{"deleted": a.Field}, nil
		},
	}
}

func moveField(d Deps) Tool {
	return Tool{
		Name:   "move_field",
		Writes: true,
		Admin:  true,
		Description: "Moves a field one place up or down within its own level — the page's fields, " +
			"one group, one block kind or one snippet. Call it again to move further.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":   websiteProp,
				"field":     {Type: "integer", Description: "id of the field"},
				"direction": directionProp,
			}, Required: []string{"website", "field", "direction"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64  `json:"website"`
				Field     int64  `json:"field"`
				Direction string `json:"direction"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[fieldOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpMoveField(c.Ctx, a.Website, a.Field, a.Direction == "up"); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actStructureReorder, EntityType: "field", EntityID: a.Field})
			return map[string]any{"moved": a.Field, "direction": a.Direction}, nil
		},
	}
}

// --- block kinds (admin) ----------------------------------------------------

func blockKindOut(t block.Own, used int) map[string]any {
	out := map[string]any{"id": t.ID, "key": t.Key, "name": t.Name, "used_on_pages": used,
		"fields": fieldDefsOut(t.Fields)}
	if t.Hint != "" {
		out["hint"] = t.Hint
	}
	return out
}

func listBlockKinds(d Deps) Tool {
	return Tool{
		Name:  "list_block_kinds",
		Admin: true,
		Description: "Lists the block kinds a page can be built from: the built-in ones by key, and " +
			"the website's own with their fields and on how many pages each is used.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"website": websiteProp}, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[blockKindOps](d)
			if err != nil {
				return nil, err
			}
			list, used, err := ops.OpBlockTypes(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			builtin := make([]string, 0, len(block.Kinds))
			for _, k := range block.Kinds {
				builtin = append(builtin, k.Type)
			}
			own := make([]map[string]any, 0, len(list))
			for _, t := range list {
				own = append(own, blockKindOut(t, used[t.Key]))
			}
			return map[string]any{"built_in": builtin, "own": own}, nil
		},
	}
}

func createBlockKind(d Deps) Tool {
	return Tool{
		Name:   "create_block_kind",
		Writes: true,
		Admin:  true,
		Description: "Creates an own block kind, such as a recipe step. Its key is made from the name " +
			"and never changes. It starts without fields: add them with create_field and block_kind " +
			"set to the id this returns.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": websiteProp,
				"name":    {Type: "string", Description: "the name shown in the editor"},
				"hint":    {Type: "string", Description: "a line saying what the block is for"},
			}, Required: []string{"website", "name"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Name    string `json:"name"`
				Hint    string `json:"hint"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[blockKindOps](d)
			if err != nil {
				return nil, err
			}
			t, err := ops.OpSaveBlockType(c.Ctx, a.Website, 0, a.Name, a.Hint)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actBlockKindSave, EntityType: "block_kind", EntityID: t.ID})
			out := blockKindOut(*t, 0)
			out["note"] = "Created without fields. Add them with create_field and block_kind: " +
				fmt.Sprint(t.ID) + "."
			return out, nil
		},
	}
}

func updateBlockKind(d Deps) Tool {
	return Tool{
		Name:        "update_block_kind",
		Writes:      true,
		Admin:       true,
		Description: "Changes an own block kind's name or hint. The key stays.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":    websiteProp,
				"block_kind": {Type: "integer", Description: "id of the block kind"},
				"name":       {Type: "string", Description: "new name; without one the old stays"},
				"hint":       {Type: "string", Description: "new hint; without one the old stays"},
			}, Required: []string{"website", "block_kind"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64   `json:"website"`
				BlockKind int64   `json:"block_kind"`
				Name      *string `json:"name"`
				Hint      *string `json:"hint"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[blockKindOps](d)
			if err != nil {
				return nil, err
			}
			list, used, err := ops.OpBlockTypes(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			var current *block.Own
			for i := range list {
				if list[i].ID == a.BlockKind {
					current = &list[i]
				}
			}
			if current == nil {
				return nil, errors.New("there is no such block kind on this website")
			}
			name, hint := current.Name, current.Hint
			if a.Name != nil {
				name = *a.Name
			}
			if a.Hint != nil {
				hint = *a.Hint
			}
			t, err := ops.OpSaveBlockType(c.Ctx, a.Website, current.ID, name, hint)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actBlockKindSave, EntityType: "block_kind", EntityID: t.ID})
			return blockKindOut(*t, used[t.Key]), nil
		},
	}
}

func deleteBlockKind(d Deps) Tool {
	return Tool{
		Name:   "delete_block_kind",
		Writes: true,
		Admin:  true,
		Description: "Deletes an own block kind with its fields. Blocks of this kind disappear from " +
			"each page the next time it is saved. Requires confirm: true.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":    websiteProp,
				"block_kind": {Type: "integer", Description: "id of the block kind"},
				"confirm":    confirmProp,
			}, Required: []string{"website", "block_kind", "confirm"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64 `json:"website"`
				BlockKind int64 `json:"block_kind"`
				Confirm   bool  `json:"confirm"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			if err := needConfirm(a.Confirm, "a block kind"); err != nil {
				return nil, err
			}
			ops, err := structOps[blockKindOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteBlockType(c.Ctx, a.Website, a.BlockKind); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actBlockKindDelete, EntityType: "block_kind", EntityID: a.BlockKind})
			return map[string]any{"deleted": a.BlockKind}, nil
		},
	}
}

func moveBlockKind(d Deps) Tool {
	return Tool{
		Name:        "move_block_kind",
		Writes:      true,
		Admin:       true,
		Description: "Moves an own block kind one place up or down in the editor's menu.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website":    websiteProp,
				"block_kind": {Type: "integer", Description: "id of the block kind"},
				"direction":  directionProp,
			}, Required: []string{"website", "block_kind", "direction"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64  `json:"website"`
				BlockKind int64  `json:"block_kind"`
				Direction string `json:"direction"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[blockKindOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpMoveBlockType(c.Ctx, a.Website, a.BlockKind, a.Direction == "up"); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{Action: actStructureReorder, EntityType: "block_kind", EntityID: a.BlockKind})
			return map[string]any{"moved": a.BlockKind, "direction": a.Direction}, nil
		},
	}
}
