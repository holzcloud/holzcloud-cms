package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The structure of a website — menus, terms, snippets, redirects, content
// kinds, fields and block kinds — as the assistant connection reaches it.
//
// Each Op… method below either shares its steps with the screen (createMenu,
// storeSnippet, saveField, saveKind, redirectProblems, checkInternalLinks) or
// is the one store call the screen makes too, reached through here because the
// store is the handler's and not the connection's. Every method takes the
// website and checks that what an id names belongs to it: the ids come from an
// assistant, and an assistant repeats whatever it was told.

// refusal is input that was turned down, with the sentence that says why. The
// screens flash it and the assistant connection hands it back; neither has to
// know which check it came from.
type refusal struct{ msg string }

func (e refusal) Error() string { return e.msg }

func refuse(msg string) error { return refusal{msg: msg} }

func isRefusal(err error) bool {
	var r refusal
	return errors.As(err, &r)
}

// --- menus ------------------------------------------------------------------

// createMenu checks and stores a new menu. The language is only trusted when
// the website actually has it: a menu in a language nobody serves would be
// invisible and unexplainable.
func (h *Handler) createMenu(ctx context.Context, websiteID int64, name, locationKey, language string) (*menu.Menu, error) {
	name, locationKey = strings.TrimSpace(name), strings.TrimSpace(locationKey)
	loc := ""
	if ws, err := h.domains.GetWebsite(ctx, websiteID); err == nil && ws != nil {
		loc = locale.Pick(language, ws.Locales())
	}
	if err := checkMenu(name, locationKey); err != nil {
		return nil, err
	}
	m, err := h.menuStore.CreateMenu(ctx, websiteID, name, locationKey, loc)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, refuse(i18n.N("This website already has a menu with that key in this language"))
		}
		return nil, err
	}
	return m, nil
}

// updateMenu checks and stores a menu's name and key.
func (h *Handler) updateMenu(ctx context.Context, menuID int64, name, locationKey string) error {
	name, locationKey = strings.TrimSpace(name), strings.TrimSpace(locationKey)
	if err := checkMenu(name, locationKey); err != nil {
		return err
	}
	if err := h.menuStore.UpdateMenu(ctx, menuID, name, locationKey); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return refuse(i18n.N("This website already has a menu with that key in this language"))
		}
		return err
	}
	return nil
}

// checkMenu is the rule for a menu's name and key, on creating and on
// changing alike (T-04-09). The key is what a theme looks the menu up by, so a
// key outside the alphabet is a menu no template can reach: renaming "haupt" to
// "Haupt Menü" was once accepted on the update path, answered "Menu saved", and //nolint:german — the example key, which is the point
// took the navigation off the site with nothing anywhere to say why.
func checkMenu(name, locationKey string) error {
	if name == "" || locationKey == "" {
		return refuse(i18n.N("Please give a name and a key"))
	}
	if !isValidLocationKey(locationKey) {
		return refuse(i18n.N("The key may only contain lower-case letters, digits and hyphens"))
	}
	return nil
}

// OpMenus lists the menus of a website.
func (h *Handler) OpMenus(ctx context.Context, websiteID int64) ([]menu.Menu, error) {
	return h.menuStore.ListMenus(ctx, websiteID)
}

// OpMenu returns one menu with its entries as a tree, or nil when the menu is
// not this website's.
func (h *Handler) OpMenu(ctx context.Context, websiteID, menuID int64) (*menu.Menu, []menu.MenuNode, error) {
	m, err := h.menuOfWebsite(ctx, websiteID, menuID)
	if err != nil || m == nil {
		return nil, nil, err
	}
	items, err := h.menuStore.ListItems(ctx, menuID)
	if err != nil {
		return nil, nil, err
	}
	return m, menu.Tree(items), nil
}

// OpCreateMenu creates a menu, with the checks of the menu screen.
func (h *Handler) OpCreateMenu(ctx context.Context, websiteID int64, name, locationKey, language string) (*menu.Menu, error) {
	return h.createMenu(ctx, websiteID, name, locationKey, language)
}

// OpUpdateMenu changes a menu's name and key.
func (h *Handler) OpUpdateMenu(ctx context.Context, websiteID, menuID int64, name, locationKey string) error {
	m, err := h.menuOfWebsite(ctx, websiteID, menuID)
	if err != nil {
		return err
	}
	if m == nil {
		return errors.New("there is no such menu on this website")
	}
	return h.updateMenu(ctx, menuID, name, locationKey)
}

// MaxMenuDepth is how deep a menu written in one go may nest: the depth
// RenderMenu draws. A deeper entry would be stored and never shown.
const MaxMenuDepth = 3

// maxMenuItems bounds one menu. A navigation with two hundred entries is a
// mistake, and an assistant making one should hear so.
const maxMenuItems = 200

// OpSetMenuItems replaces every entry of a menu with the given tree.
//
// The same three kinds of entry the screen offers, and one check the screen
// never needed: a page entry must name a page of this website, because the id
// comes from an assistant and not out of a list the server drew itself.
func (h *Handler) OpSetMenuItems(ctx context.Context, websiteID, menuID int64, items []menu.ItemInput) error {
	m, err := h.menuOfWebsite(ctx, websiteID, menuID)
	if err != nil {
		return err
	}
	if m == nil {
		return errors.New("there is no such menu on this website")
	}
	count := 0
	var check func(list []menu.ItemInput, depth int) error
	check = func(list []menu.ItemInput, depth int) error {
		if depth > MaxMenuDepth {
			return fmt.Errorf("a menu nests at most %d levels deep; deeper entries would never be shown", MaxMenuDepth)
		}
		for i := range list {
			it := &list[i]
			count++
			if count > maxMenuItems {
				return fmt.Errorf("a menu holds at most %d entries", maxMenuItems)
			}
			it.Title = strings.TrimSpace(it.Title)
			it.URL = strings.TrimSpace(it.URL)
			if it.Title == "" {
				return errors.New("every menu entry needs a label")
			}
			switch it.ItemType {
			case "page":
				if it.PageID == nil || *it.PageID <= 0 {
					return fmt.Errorf("the entry %q links to a page but names none", it.Title)
				}
				p, err := h.pages.GetPage(ctx, *it.PageID)
				if err != nil {
					return err
				}
				if p == nil || p.WebsiteID != websiteID {
					return fmt.Errorf("the entry %q names page %d, which is not a page of this website", it.Title, *it.PageID)
				}
				it.URL = ""
			case "url":
				if it.URL == "" {
					return fmt.Errorf("the entry %q links to an address but names none", it.Title)
				}
				it.PageID = nil
			case "custom":
				it.URL, it.PageID = "", nil
			default:
				return fmt.Errorf("the entry %q has no valid kind: page, url or custom", it.Title)
			}
			if err := check(it.Children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := check(items, 1); err != nil {
		return err
	}
	return h.menuStore.ReplaceItems(ctx, menuID, items)
}

// OpDeleteMenu removes a menu with its entries.
func (h *Handler) OpDeleteMenu(ctx context.Context, websiteID, menuID int64) error {
	m, err := h.menuOfWebsite(ctx, websiteID, menuID)
	if err != nil {
		return err
	}
	if m == nil {
		return errors.New("there is no such menu on this website")
	}
	return h.menuStore.DeleteMenu(ctx, menuID)
}

// --- terms ------------------------------------------------------------------

// OpTerms lists every term of a website with how often it is used, drafts
// included — the view of the terms screen.
func (h *Handler) OpTerms(ctx context.Context, websiteID int64) ([]term.Term, error) {
	return h.terms.ListAll(ctx, websiteID)
}

// OpRenameTerm changes a term's name and keeps its address.
func (h *Handler) OpRenameTerm(ctx context.Context, websiteID, termID int64, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("a term needs a name")
	}
	if err := h.termOfWebsite(ctx, websiteID, termID); err != nil {
		return err
	}
	return h.terms.Rename(ctx, websiteID, termID, name)
}

// OpDeleteTerm removes a term from everything that carries it. The content
// itself stays.
func (h *Handler) OpDeleteTerm(ctx context.Context, websiteID, termID int64) error {
	if err := h.termOfWebsite(ctx, websiteID, termID); err != nil {
		return err
	}
	return h.terms.Delete(ctx, websiteID, termID)
}

// termOfWebsite says plainly when a term is not this website's. The store
// would simply change nothing, which a screen can live with and an assistant
// reporting "deleted" cannot.
func (h *Handler) termOfWebsite(ctx context.Context, websiteID, termID int64) error {
	all, err := h.terms.ListAll(ctx, websiteID)
	if err != nil {
		return err
	}
	for _, t := range all {
		if t.ID == termID {
			return nil
		}
	}
	return errors.New("there is no such term on this website")
}

// --- snippets ---------------------------------------------------------------

// OpSnippets lists the snippets of a website and, by key, on how many live
// pages each one is used.
func (h *Handler) OpSnippets(ctx context.Context, websiteID int64) ([]snippet.Snippet, map[string]int, error) {
	list, err := h.snippets.List(ctx, websiteID)
	if err != nil {
		return nil, nil, err
	}
	used := make(map[string]int, len(list))
	for _, sn := range list {
		n, err := h.snippets.CountUsage(ctx, websiteID, sn.Key)
		if err != nil {
			return nil, nil, err
		}
		used[sn.Key] = n
	}
	return list, used, nil
}

// OpSnippet returns one snippet of a website with the definitions of its own
// fields, or a nil snippet when it is not this website's.
func (h *Handler) OpSnippet(ctx context.Context, websiteID, id int64) (*snippet.Snippet, []field.Def, error) {
	sn, err := h.snippets.Get(ctx, websiteID, id)
	if err != nil || sn == nil {
		return nil, nil, err
	}
	return sn, h.snippetFieldDefs(ctx, websiteID, sn.ID), nil
}

// OpSaveSnippet creates a snippet (id 0) or changes one, with the checks of the
// snippet screen: the name, the shape of the key, and the snippet's own fields
// against the definitions the server holds. The fields are written whole, as
// the screen writes them — the caller carries forward what it keeps.
func (h *Handler) OpSaveSnippet(ctx context.Context, websiteID, id int64, key, name, markdown string, fields field.Data) (*snippet.Snippet, error) {
	values := snippetValues{
		ID:       id,
		Key:      strings.TrimSpace(strings.ToLower(key)),
		Name:     strings.TrimSpace(name),
		Markdown: markdown,
		Fields:   fields,
	}
	errs := web.FormErrors{}
	values.validate(errs)
	for _, k := range []string{"name", "key"} {
		if msg, ok := errs[k]; ok {
			return nil, refuse(msg)
		}
	}
	defs := h.snippetFieldDefs(ctx, websiteID, id)
	for _, reason := range reasonTexts(ctx, field.CheckAll(defs, values.Fields)) {
		return nil, refuse(reason)
	}
	saved, err := h.storeSnippet(ctx, websiteID, values, defs)
	switch {
	case errors.Is(err, snippet.ErrNotFound):
		return nil, errors.New("there is no such snippet on this website")
	case errors.Is(err, snippet.ErrKeyTaken):
		return nil, refuse(i18n.N("That key is already used by another snippet."))
	case err != nil:
		return nil, err
	}
	return h.snippets.Get(ctx, websiteID, saved)
}

// OpDeleteSnippet removes a snippet. A page carrying its marker shows nothing
// in its place from then on.
func (h *Handler) OpDeleteSnippet(ctx context.Context, websiteID, id int64) error {
	sn, err := h.snippets.Get(ctx, websiteID, id)
	if err != nil {
		return err
	}
	if sn == nil {
		return errors.New("there is no such snippet on this website")
	}
	return h.snippets.Delete(ctx, websiteID, id)
}

// --- redirects and links ----------------------------------------------------

// OpRedirects lists the redirects of a website with their hit counts.
func (h *Handler) OpRedirects(ctx context.Context, websiteID int64) ([]page.Redirect, error) {
	return h.pages.ListRedirects(ctx, websiteID)
}

// OpAddRedirect stores a redirect, normalising both addresses the way the
// screen does. It answers the two addresses as they were stored.
func (h *Handler) OpAddRedirect(ctx context.Context, websiteID int64, from, to string, code int) (string, string, error) {
	from, to = normalizeRedirectPath(from), normalizeRedirectPath(to)
	for _, p := range redirectProblems(from, to) {
		return "", "", refuse(p.message)
	}
	if code != 302 {
		code = 301
	}
	if err := h.pages.AddRedirect(ctx, websiteID, from, to, code); err != nil {
		return "", "", err
	}
	return from, to, nil
}

// OpDeleteRedirect removes one redirect of a website.
func (h *Handler) OpDeleteRedirect(ctx context.Context, websiteID, id int64) error {
	err := h.pages.DeleteRedirect(ctx, websiteID, id)
	if errors.Is(err, page.ErrNotFound) {
		return errors.New("there is no such redirect on this website")
	}
	return err
}

// OpBrokenLinks is the report the redirect screen shows: every root-relative
// link on the website's pages that resolves to no page, no redirect and no
// route. The type is the connection's own, because internal/ai cannot import
// this package.
func (h *Handler) OpBrokenLinks(ctx context.Context, websiteID int64) ([]ai.BrokenLink, error) {
	found, err := h.checkInternalLinks(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	out := make([]ai.BrokenLink, 0, len(found))
	for _, b := range found {
		out = append(out, ai.BrokenLink{PageID: b.OnPageID, PageTitle: b.OnPage, PageSlug: b.OnPageSlug, Target: b.Target})
	}
	return out, nil
}

// --- content kinds ----------------------------------------------------------

// OpKinds lists a website's own content kinds and, by key, how many entries
// carry each.
func (h *Handler) OpKinds(ctx context.Context, websiteID int64) ([]kind.Type, map[string]int, error) {
	types, err := h.kinds.List(ctx, websiteID)
	if err != nil {
		return nil, nil, err
	}
	counts := make(map[string]int, len(types))
	for _, t := range types {
		n, err := h.kinds.Count(ctx, websiteID, t.Key)
		if err != nil {
			return nil, nil, err
		}
		counts[t.Key] = n
	}
	return types, counts, nil
}

// OpSaveKind creates a content kind (t.ID 0) or changes one, with the checks
// of the content kind screen.
func (h *Handler) OpSaveKind(ctx context.Context, websiteID int64, t kind.Type) (kind.Type, error) {
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return kind.Type{}, err
	}
	if ws == nil {
		return kind.Type{}, errors.New("there is no such website")
	}
	t.Archive = archiveSlug(t.Archive)
	return h.saveKind(ctx, ws, t)
}

// saveKind creates a kind (t.ID 0) or changes one, after the checks the store
// cannot make because they need the website and its pages. The screen and the
// assistant connection both save through it.
func (h *Handler) saveKind(ctx context.Context, ws *domain.Website, t kind.Type) (kind.Type, error) {
	t.WebsiteID = ws.ID
	// The address of the overview is reserved like that of the archive: a page
	// with the same address would never get its turn, because the overview is
	// checked first. Better to refuse here than to win silently there.
	if t.Archive != "" {
		if t.Archive == ws.BlogBase {
			return kind.Type{}, refuse(i18n.N("This address already belongs to the archive of posts"))
		}
		if existing, err := h.pages.GetPageBySlug(ctx, ws.ID, t.Archive); err == nil && existing != nil {
			return kind.Type{}, refuse(i18n.N(
				"There is already a page at this address. Choose another one, otherwise one of the two would be unreachable."))
		}
	}
	if t.ID > 0 {
		if err := h.kinds.Update(ctx, t); err != nil {
			return kind.Type{}, err
		}
		return h.kinds.Get(ctx, ws.ID, t.ID)
	}
	return h.kinds.Create(ctx, t)
}

// OpDeleteKind removes a content kind. Its entries stay, keep the key and can
// be switched to another kind; the answer says how many there are.
func (h *Handler) OpDeleteKind(ctx context.Context, websiteID, id int64) (kind.Type, int, error) {
	t, err := h.kinds.Get(ctx, websiteID, id)
	if err != nil {
		return kind.Type{}, 0, err
	}
	n, err := h.kinds.Count(ctx, websiteID, t.Key)
	if err != nil {
		return kind.Type{}, 0, err
	}
	if err := h.kinds.Delete(ctx, websiteID, id); err != nil {
		return kind.Type{}, 0, err
	}
	return t, n, nil
}

// OpMoveKind shifts a content kind one place in the order.
func (h *Handler) OpMoveKind(ctx context.Context, websiteID, id int64, up bool) error {
	if _, err := h.kinds.Get(ctx, websiteID, id); err != nil {
		return err
	}
	return h.kinds.Move(ctx, websiteID, id, up)
}

// --- fields -----------------------------------------------------------------

// OpField returns one field definition of a website.
func (h *Handler) OpField(ctx context.Context, websiteID, id int64) (*field.Def, error) {
	def, err := h.fields.Get(ctx, websiteID, id)
	if err != nil || def == nil {
		return nil, errors.New("there is no such field on this website")
	}
	return def, nil
}

// OpSaveField creates a field (id 0) or changes one, with the checks of the
// field screen. The carrier — a group, a block kind or a snippet — is named in
// def and checked against the website.
func (h *Handler) OpSaveField(ctx context.Context, websiteID, id int64, def field.Def) (*field.Def, error) {
	if id > 0 {
		if _, err := h.OpField(ctx, websiteID, id); err != nil {
			return nil, err
		}
	}
	if def.BlockTypeID > 0 {
		if _, err := h.blockTypes.Get(ctx, websiteID, def.BlockTypeID); err != nil {
			return nil, errors.New("there is no such block kind on this website")
		}
	}
	saved, err := h.saveField(ctx, websiteID, id, def)
	if errors.Is(err, errNoCarrier) {
		return nil, errors.New("there is no such group or snippet on this website")
	}
	return saved, err
}

// OpDeleteField removes a field definition. The values stay on the pages until
// each is next saved.
func (h *Handler) OpDeleteField(ctx context.Context, websiteID, id int64) error {
	if _, err := h.OpField(ctx, websiteID, id); err != nil {
		return err
	}
	return h.fields.Delete(ctx, websiteID, id)
}

// OpMoveField shifts a field one place within its own level.
func (h *Handler) OpMoveField(ctx context.Context, websiteID, id int64, up bool) error {
	if _, err := h.OpField(ctx, websiteID, id); err != nil {
		return err
	}
	return h.fields.Move(ctx, websiteID, id, up)
}

// --- block kinds ------------------------------------------------------------

// OpBlockTypes lists a website's own block kinds, each with its fields, and by
// key on how many pages each one is used.
func (h *Handler) OpBlockTypes(ctx context.Context, websiteID int64) ([]block.Own, map[string]int, error) {
	types, err := h.blockTypes.List(ctx, websiteID)
	if err != nil {
		return nil, nil, err
	}
	used := make(map[string]int, len(types))
	for _, t := range types {
		n, err := h.blockTypes.Used(ctx, websiteID, t.Key)
		if err != nil {
			return nil, nil, err
		}
		used[t.Key] = n
	}
	return types, used, nil
}

// OpSaveBlockType creates a block kind (id 0) or changes its name and hint.
func (h *Handler) OpSaveBlockType(ctx context.Context, websiteID, id int64, name, hint string) (*block.Own, error) {
	if id == 0 {
		return h.blockTypes.Create(ctx, websiteID, name, hint)
	}
	if _, err := h.blockTypeOfWebsite(ctx, websiteID, id); err != nil {
		return nil, err
	}
	if err := h.blockTypes.Update(ctx, websiteID, id, name, hint); err != nil {
		return nil, err
	}
	return h.blockTypes.Get(ctx, websiteID, id)
}

// OpDeleteBlockType removes a block kind with its fields. Blocks of that kind
// disappear from each page the next time it is saved.
func (h *Handler) OpDeleteBlockType(ctx context.Context, websiteID, id int64) error {
	if _, err := h.blockTypeOfWebsite(ctx, websiteID, id); err != nil {
		return err
	}
	return h.blockTypes.Delete(ctx, websiteID, id)
}

// OpMoveBlockType shifts a block kind one place in the editor's menu.
func (h *Handler) OpMoveBlockType(ctx context.Context, websiteID, id int64, up bool) error {
	if _, err := h.blockTypeOfWebsite(ctx, websiteID, id); err != nil {
		return err
	}
	return h.blockTypes.Move(ctx, websiteID, id, up)
}

func (h *Handler) blockTypeOfWebsite(ctx context.Context, websiteID, id int64) (*block.Own, error) {
	t, err := h.blockTypes.Get(ctx, websiteID, id)
	if err != nil || t == nil {
		return nil, errors.New("there is no such block kind on this website")
	}
	return t, nil
}
