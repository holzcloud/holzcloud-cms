package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// Pages beyond create, change and publish: the wastebasket, the history, the
// blocks, the settings of the editor's side panel, languages, preview links,
// review and the bulk menu of the page list.
//
// Everything that a screen does with more than one store call goes through
// the admin handler (pageOps), so an assistant's save renders blocks, keeps the
// media bookkeeping and tells the plugins exactly as a person's save does.
// Single store calls — list the wastebasket, name a revision — are made here.
//
// What an editor account may not do, a key is not asked about: a content key
// is the operator's own assistant and publishes when it is told to, which is
// what publish_page already does. The one exception is erasing a page for
// good, which the screens leave to an administrator and which is left here to
// an admin key.

// pageOps is what the page tools need from the admin handler.
type pageOps interface {
	OpBlockSet(ctx context.Context, websiteID int64) block.Set
	OpEditPage(ctx context.Context, websiteID, pageID, version int64, edit func(p *page.Page, u *page.PageUpdate) error) (*page.Page, error)
	OpSetPageBlocks(ctx context.Context, websiteID, pageID, version int64, blocks []block.Block) (*page.Page, error)
	OpTrashPage(ctx context.Context, websiteID, pageID int64) (*page.Page, error)
	OpRestoreRevision(ctx context.Context, websiteID, pageID, revisionID int64) (*page.Page, *page.Revision, error)
	OpDuplicatePage(ctx context.Context, websiteID, pageID int64) (*page.Page, error)
	OpTranslatePage(ctx context.Context, websiteID, pageID int64, language string) (*page.Page, error)
	OpShareLink(ctx context.Context, websiteID, pageID int64, days int) (string, time.Time, error)
	OpSetPageAccess(ctx context.Context, websiteID, pageID int64, protected bool, password, hint string) (*page.Page, error)
	OpPageTerms(ctx context.Context, websiteID, pageID int64) ([]string, error)
	OpSetPageTerms(ctx context.Context, websiteID, pageID int64, names []string) ([]string, error)
	OpSetPageKind(ctx context.Context, websiteID, pageID, version int64, key string) (*page.Page, error)
	OpSetPageSlug(ctx context.Context, websiteID, pageID, version int64, slug string) (*page.Page, error)
}

var errNoPageOps = errors.New("this function is not available on this installation")

func (d Deps) pageOps() (pageOps, error) {
	ops, ok := d.Ops.(pageOps)
	if !ok {
		return nil, errNoPageOps
	}
	return ops, nil
}

func pagesTools(d Deps) []Tool {
	return []Tool{
		getPageSettings(d),
		listTrash(d), trashPage(d), restorePage(d), purgePage(d),
		listRevisions(d), readRevision(d), restoreRevision(d), labelRevision(d),
		duplicatePage(d),
		listBlockKinds(d), getPageBlocks(d), setPageBlocks(d), insertBlock(d),
		setPageSchedule(d), setPageProtection(d), setPageSEO(d), setPageTerms(d),
		setPageKind(d), setPageSlug(d),
		listTranslations(d), createTranslation(d),
		createShareLink(d), setReview(d), bulkPages(d),
	}
}

// --- shared -----------------------------------------------------------------

// idArgs is the argument most of these tools take.
type idArgs struct {
	ID int64 `json:"id"`
}

var idOnly = Schema{
	Type:       "object",
	Properties: map[string]Property{"id": {Type: "integer", Description: "id of the page"}},
	Required:   []string{"id"},
}

// versionProperty lets an assistant say which state it read. With it, a save
// over somebody else's newer one is refused instead of overwriting it.
var versionProperty = Property{Type: "integer", Description: "the version you read (from " +
	"get_page_settings or get_page_blocks); the change is refused when somebody saved in " +
	"between. Leave out to change the page as it is now"}

// pageOf fetches a page by id and checks the key may reach its website. A
// page in the wastebasket is found too; livePageOf refuses it.
func pageOf(c Call, d Deps, id int64) (*page.Page, error) {
	p, err := d.Pages.GetPage(c.Ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("there is no such page")
	}
	if err := c.Scope.MaySee(p.WebsiteID); err != nil {
		return nil, err
	}
	return p, nil
}

func livePageOf(c Call, d Deps, id int64) (*page.Page, error) {
	p, err := pageOf(c, d, id)
	if err != nil {
		return nil, err
	}
	if p.InTrash() {
		return nil, errors.New("this page is in the wastebasket; restore_page brings it back")
	}
	return p, nil
}

// pageErr turns the store's errors into a sentence an assistant can act on.
func pageErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, page.ErrNotFound):
		return errors.New("there is no such page")
	case errors.Is(err, page.ErrConflict):
		return errors.New("somebody else has saved the page in the meantime; " +
			"please read it again and then change it once more")
	case errors.Is(err, page.ErrSlugTaken):
		return errors.New("that address is already used by another page")
	}
	return err
}

// pageEntry is the activity log entry for one page.
func pageEntry(action string, p *page.Page, extra map[string]any) activity.Entry {
	meta := map[string]any{"slug": p.Slug, "titel": p.Title}
	for k, v := range extra {
		meta[k] = v
	}
	return activity.Entry{Action: action, EntityType: "page", EntityID: p.ID, Metadata: meta}
}

// withNote is brief with a sentence added.
func withNote(p page.Page, note string) map[string]any {
	out := brief(p)
	if note != "" {
		out["note"] = note
	}
	return out
}

func stamp(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(timeLayout)
}

// --- settings ---------------------------------------------------------------

func getPageSettings(d Deps) Tool {
	return Tool{
		Name: "get_page_settings",
		Description: "Fetches what the editor's side panel shows about a page: version, kind, " +
			"language, schedule, password protection, search-engine settings, labels, review " +
			"state and how many older versions exist. The body is fetched by read_page.",
		InputSchema: idOnly,
		Run: func(c Call) (any, error) {
			var a idArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := pageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			out := brief(*p)
			out["version"] = p.Version
			out["in_trash"] = p.InTrash()
			out["review"] = "none"
			if p.ReviewState == "pending" {
				out["review"] = "pending"
			}
			out["schedule"] = map[string]any{
				"publish_at": stamp(p.PublishAt), "unpublish_at": stamp(p.UnpublishAt),
			}
			out["protection"] = map[string]any{
				"protected": p.Protected(), "hint": p.AccessHint,
				"has_password": p.AccessPassword != "",
			}
			var featured any
			if p.FeaturedMediaID != nil {
				featured = *p.FeaturedMediaID
			}
			out["seo"] = map[string]any{
				"excerpt": p.Excerpt, "meta_description": p.MetaDescription,
				"featured_media": featured, "noindex": p.NoIndex,
			}
			out["built_from"] = "markdown"
			if p.Blocks != "" {
				out["built_from"] = "blocks"
			}
			if revs, err := d.Pages.ListRevisions(c.Ctx, p.ID); err == nil {
				out["revisions"] = len(revs)
			}
			if ops, err := d.pageOps(); err == nil && !p.InTrash() {
				if labels, err := ops.OpPageTerms(c.Ctx, p.WebsiteID, p.ID); err == nil {
					if labels == nil {
						labels = []string{}
					}
					out["labels"] = labels
				}
			}
			return out, nil
		},
	}
}

// --- wastebasket ------------------------------------------------------------

func listTrash(d Deps) Tool {
	return Tool{
		Name: "list_trash",
		Description: "Lists the pages in a website's wastebasket, most recently deleted first, " +
			"with the address they had. They are erased for good after " +
			fmt.Sprint(int(page.TrashRetention.Hours()/24)) + " days.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": {Type: "integer", Description: "id of the website"}},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			pages, err := d.Pages.ListTrash(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(pages))
			for _, p := range pages {
				e := brief(p)
				e["deleted_at"] = stamp(p.DeletedAt)
				out = append(out, e)
			}
			return map[string]any{"pages": out}, nil
		},
	}
}

func trashPage(d Deps) Tool {
	return Tool{
		Name:   "trash_page",
		Writes: true,
		Description: "Moves a page to the wastebasket. It disappears from the website at once " +
			"and can be brought back with restore_page for 30 days.",
		InputSchema: idOnly,
		Run: func(c Call) (any, error) {
			var a idArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			if _, err := ops.OpTrashPage(c.Ctx, p.WebsiteID, p.ID); err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageDelete, p, nil))
			return withNote(*p, "Moved to the wastebasket. restore_page brings it back."), nil
		},
	}
}

func restorePage(d Deps) Tool {
	return Tool{
		Name:   "restore_page",
		Writes: true,
		Description: "Brings a page back out of the wastebasket, with its old address when that " +
			"is still free. Its status is what it was before.",
		InputSchema: idOnly,
		Run: func(c Call) (any, error) {
			var a idArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := pageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			if !p.InTrash() {
				return nil, errors.New("this page is not in the wastebasket")
			}
			if err := d.Pages.RestorePage(c.Ctx, p.ID); err != nil {
				return nil, pageErr(err)
			}
			after, err := d.Pages.GetPage(c.Ctx, p.ID)
			if err != nil || after == nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, after,
				map[string]any{"restored": true}))
			return brief(*after), nil
		},
	}
}

func purgePage(d Deps) Tool {
	return Tool{
		Name:   "purge_page",
		Writes: true,
		Admin:  true,
		Description: "Erases a page in the wastebasket for good, with its whole history. This " +
			"cannot be undone. Requires confirm: true; call it only when you were expressly " +
			"asked to erase the page.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":      {Type: "integer", Description: "id of a page in the wastebasket"},
				"confirm": {Type: "boolean", Description: "must be true"},
			},
			Required: []string{"id", "confirm"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64 `json:"id"`
				Confirm bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errors.New("erasing needs confirm: true")
			}
			p, err := pageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			if !p.InTrash() {
				return nil, errors.New("only a page in the wastebasket can be erased; " +
					"trash_page puts it there")
			}
			if err := d.Pages.PurgePage(c.Ctx, p.ID); err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageDelete, p,
				map[string]any{"purged": true}))
			return map[string]any{"id": p.ID, "erased": true}, nil
		},
	}
}

// --- history ----------------------------------------------------------------

func listRevisions(d Deps) Tool {
	return Tool{
		Name: "list_revisions",
		Description: "Lists the older versions of a page, newest first. Every save that changed " +
			"title, address or content left one; the last 20 are kept.",
		InputSchema: idOnly,
		Run: func(c Call) (any, error) {
			var a idArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := pageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			revs, err := d.Pages.ListRevisions(c.Ctx, p.ID)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(revs))
			for _, r := range revs {
				out = append(out, revisionBrief(r))
			}
			return map[string]any{"page": p.ID, "revisions": out}, nil
		},
	}
}

func revisionBrief(r page.Revision) map[string]any {
	e := map[string]any{
		"id": r.ID, "page": r.PageID, "title": r.Title, "slug": r.Slug,
		"status": wireStatus(r.Status), "saved_at": r.CreatedAt.UTC().Format(timeLayout),
		"built_from": "markdown",
	}
	if r.Blocks != "" {
		e["built_from"] = "blocks"
	}
	if r.Label != "" {
		e["label"] = r.Label
	}
	if r.UserEmail != "" {
		e["by"] = r.UserEmail
	}
	return e
}

// revisionOf fetches a revision and checks the key may reach its page.
func revisionOf(c Call, d Deps, id int64) (*page.Revision, *page.Page, error) {
	rev, err := d.Pages.GetRevision(c.Ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if rev == nil {
		return nil, nil, errors.New("there is no such revision")
	}
	p, err := pageOf(c, d, rev.PageID)
	if err != nil {
		return nil, nil, err
	}
	return rev, p, nil
}

var revisionOnly = Schema{
	Type:       "object",
	Properties: map[string]Property{"revision": {Type: "integer", Description: "id of the revision"}},
	Required:   []string{"revision"},
}

func readRevision(d Deps) Tool {
	return Tool{
		Name: "read_revision",
		Description: "Fetches one older version of a page with its complete body — markdown, " +
			"and the blocks when the page was built from blocks at the time.",
		InputSchema: revisionOnly,
		Run: func(c Call) (any, error) {
			var a struct {
				Revision int64 `json:"revision"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			rev, p, err := revisionOf(c, d, a.Revision)
			if err != nil {
				return nil, err
			}
			out := revisionBrief(*rev)
			out["markdown"] = rev.ContentMarkdown
			if rev.Blocks != "" {
				set := block.Builtin
				if ops, err := d.pageOps(); err == nil {
					set = ops.OpBlockSet(c.Ctx, p.WebsiteID)
				}
				if blocks, err := block.Decode(rev.Blocks, set); err == nil {
					out["blocks"] = wireBlocks(blocks)
				}
			}
			return out, nil
		},
	}
}

func restoreRevision(d Deps) Tool {
	return Tool{
		Name:   "restore_revision",
		Writes: true,
		Description: "Puts an older version back: title and content (text or blocks) return, " +
			"address, status and settings stay. The state it replaces becomes a revision itself, " +
			"so the restore can be undone the same way.",
		InputSchema: revisionOnly,
		Run: func(c Call) (any, error) {
			var a struct {
				Revision int64 `json:"revision"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			rev, p, err := revisionOf(c, d, a.Revision)
			if err != nil {
				return nil, err
			}
			if p.InTrash() {
				return nil, errors.New("this page is in the wastebasket; restore_page brings it back")
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			after, _, err := ops.OpRestoreRevision(c.Ctx, p.WebsiteID, p.ID, rev.ID)
			if err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageRevisionRestore, after,
				map[string]any{"fassung": rev.ID, "stand": rev.CreatedAt.Format("02.01.2006 15:04")}))
			return brief(*after), nil
		},
	}
}

func labelRevision(d Deps) Tool {
	return Tool{
		Name:   "label_revision",
		Writes: true,
		Description: "Names an older version — \"before the rebuild\" — so it can be found " +
			"again. An empty label removes the name. The page itself does not change.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"revision": {Type: "integer", Description: "id of the revision"},
				"label": {Type: "string", Description: fmt.Sprintf("the name, at most %d "+
					"characters; empty removes it", page.RevisionLabelMax)},
			},
			Required: []string{"revision", "label"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Revision int64  `json:"revision"`
				Label    string `json:"label"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			rev, _, err := revisionOf(c, d, a.Revision)
			if err != nil {
				return nil, err
			}
			if err := d.Pages.LabelRevision(c.Ctx, rev.ID, a.Label); err != nil {
				return nil, err
			}
			// A note about the history, not an edit: the log is left alone as
			// the screen leaves it, and nothing the website shows has changed.
			after, err := d.Pages.GetRevision(c.Ctx, rev.ID)
			if err != nil || after == nil {
				return nil, errors.New("the revision has gone")
			}
			return revisionBrief(*after), nil
		},
	}
}

// --- copies -----------------------------------------------------------------

func duplicatePage(d Deps) Tool {
	return Tool{
		Name:   "duplicate_page",
		Writes: true,
		Description: "Copies a page as a new draft — body, blocks, fields, kind and search " +
			"settings — with \"(copy)\" after the title and a free address.",
		InputSchema: idOnly,
		Run: func(c Call) (any, error) {
			var a idArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			created, err := ops.OpDuplicatePage(c.Ctx, p.WebsiteID, p.ID)
			if err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageCreate, created,
				map[string]any{"copy_of": p.ID}))
			return withNote(*created, "Created as a draft."), nil
		},
	}
}

// --- blocks -----------------------------------------------------------------

// wireBlock is a block as an assistant reads and writes it. The stored form
// uses German keys for historical reasons; these are the same fields in
// English, and the type keys are the stored ones, which list_block_kinds
// names.
type wireBlock struct {
	Type     string            `json:"type"`
	Markdown string            `json:"markdown,omitempty"`
	MediaID  int64             `json:"media_id,omitempty"`
	Alt      string            `json:"alt,omitempty"`
	Caption  string            `json:"caption,omitempty"`
	Variant  string            `json:"variant,omitempty"`
	Display  string            `json:"display,omitempty"`
	Title    string            `json:"title,omitempty"`
	Text     string            `json:"text,omitempty"`
	Source   string            `json:"source,omitempty"`
	LinkText string            `json:"link_text,omitempty"`
	LinkURL  string            `json:"link_url,omitempty"`
	PosterID int64             `json:"poster_id,omitempty"`
	Album    string            `json:"album,omitempty"`
	Fields   map[string]string `json:"fields,omitempty"`
	Items    []wireItem        `json:"items,omitempty"`
}

type wireItem struct {
	MediaID  int64  `json:"media_id,omitempty"`
	Alt      string `json:"alt,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Title    string `json:"title,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	LinkURL  string `json:"link_url,omitempty"`
}

func wireBlocks(blocks []block.Block) []wireBlock {
	out := make([]wireBlock, 0, len(blocks))
	for _, b := range blocks {
		w := wireBlock{
			Type: b.Type, Markdown: b.Markdown, MediaID: b.MediaID, Alt: b.Alt,
			Caption: b.Caption, Variant: b.Variant, Display: b.Display, Title: b.Title,
			Text: b.Text, Source: b.Source, LinkText: b.LinkText, LinkURL: b.LinkURL,
			PosterID: b.PosterID, Album: b.AlbumSlug, Fields: b.Fields,
		}
		for _, it := range b.Items {
			w.Items = append(w.Items, wireItem(it))
		}
		out = append(out, w)
	}
	return out
}

func storedBlock(w wireBlock) block.Block {
	b := block.Block{
		Type: strings.TrimSpace(w.Type), Markdown: w.Markdown, MediaID: w.MediaID, Alt: w.Alt,
		Caption: w.Caption, Variant: w.Variant, Display: w.Display, Title: w.Title,
		Text: w.Text, Source: w.Source, LinkText: w.LinkText, LinkURL: w.LinkURL,
		PosterID: w.PosterID, AlbumSlug: w.Album, Fields: w.Fields,
	}
	for _, it := range w.Items {
		b.Items = append(b.Items, block.Item(it))
	}
	return b
}

var blockSchema = Property{
	Type: "object",
	Description: "one block; which fields a type reads is what list_block_kinds says, " +
		"the others are ignored",
	Properties: map[string]Property{
		"type":      {Type: "string", Description: "the block type, e.g. text, bild, galerie"},
		"markdown":  {Type: "string", Description: "prose in markdown"},
		"media_id":  {Type: "integer", Description: "id of an image or video from list_media"},
		"alt":       {Type: "string", Description: "description of the image"},
		"caption":   {Type: "string", Description: "caption under the image or video"},
		"variant":   {Type: "string", Description: "layout choice, depends on the type"},
		"display":   {Type: "string", Description: "galerie only: empty for a grid, diashow for a slideshow"},
		"title":     {Type: "string", Description: "heading of a call to action"},
		"text":      {Type: "string", Description: "the quotation"},
		"source":    {Type: "string", Description: "who said the quotation"},
		"link_text": {Type: "string", Description: "button text of a call to action"},
		"link_url":  {Type: "string", Description: "button target of a call to action"},
		"poster_id": {Type: "integer", Description: "video only: id of the still image"},
		"album":     {Type: "string", Description: "galerie only: slug of an album instead of items"},
		"fields":    {Type: "object", Description: "a website's own block kind: {\"field key\": \"value\"}"},
		"items": {Type: "array", Description: "the pictures of a galerie or the panels of karten",
			Items: &Property{Type: "object", Properties: map[string]Property{
				"media_id": {Type: "integer"}, "alt": {Type: "string"}, "caption": {Type: "string"},
				"title": {Type: "string"}, "markdown": {Type: "string"}, "link_url": {Type: "string"},
			}}},
	},
	Required: []string{"type"},
}

// builtinUses says which wire fields a built-in type reads. The renderer is
// the authority; this is its summary for an assistant.
var builtinUses = map[string]string{
	block.TypeText:      "markdown",
	block.TypeImage:     "media_id (an image), alt, caption, variant: normal, breit or voll",
	block.TypeImageText: "media_id (an image), alt, caption, markdown, variant: links or rechts (side of the image)",
	block.TypeGallery: "items [{media_id, alt, caption}] or album (slug of an album), variant: 2, 3 or 4 " +
		"columns, display: empty or diashow",
	block.TypeCards:   "items [{media_id, alt, title, markdown, link_url}], variant: 2, 3 or 4 columns",
	block.TypeQuote:   "text, source",
	block.TypeCallout: "title, markdown, link_text, link_url",
	block.TypeVideo:   "media_id (an MP4 from the library), poster_id (an image), caption, variant: normal, breit or voll",
	block.TypeDivider: "nothing",
}

func listBlockKinds(d Deps) Tool {
	return Tool{
		Name: "list_block_kinds",
		Description: "Lists the block types a website's pages can be built from — the built-in " +
			"ones and the website's own — and which fields each one reads. Call it before " +
			"set_page_blocks.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": {Type: "integer", Description: "id of the website"}},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			set := ops.OpBlockSet(c.Ctx, a.Website)
			out := []map[string]any{}
			for _, k := range block.Kinds {
				out = append(out, map[string]any{
					"type": k.Type, "name": k.Name, "hint": k.Hint, "reads": builtinUses[k.Type],
				})
			}
			for _, own := range set.Own {
				fields := make([]map[string]any, 0, len(own.Fields))
				for _, def := range own.Fields {
					e := map[string]any{
						"key": def.Key, "label": def.Label, "kind": def.Kind, "required": def.Required,
					}
					if len(def.Choices) > 0 {
						e["choices"] = def.Choices
					}
					fieldProperties(e, def)
					fields = append(fields, e)
				}
				out = append(out, map[string]any{
					"type": own.Key, "name": own.Name, "hint": own.Hint, "own": true,
					"reads": "fields, as {\"key\": \"value\"}", "fields": fields,
				})
			}
			return map[string]any{"kinds": out, "max_blocks": block.MaxBlocks, "max_items": block.MaxItems}, nil
		},
	}
}

func getPageBlocks(d Deps) Tool {
	return Tool{
		Name: "get_page_blocks",
		Description: "Fetches the block list of a page as JSON, with the version to hand back " +
			"to set_page_blocks. A page written as plain markdown has no blocks; setting some " +
			"turns it into a block page.",
		InputSchema: idOnly,
		Run: func(c Call) (any, error) {
			var a idArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			blocks, err := block.Decode(p.Blocks, ops.OpBlockSet(c.Ctx, p.WebsiteID))
			if err != nil {
				return nil, err
			}
			out := brief(*p)
			out["version"] = p.Version
			out["blocks"] = wireBlocks(blocks)
			if blocks == nil {
				out["built_from"] = "markdown"
				out["note"] = "This page is plain markdown (see read_page). set_page_blocks " +
					"would make it a block page; put its text into a text block to keep it."
			} else {
				out["built_from"] = "blocks"
			}
			return out, nil
		},
	}
}

// saveBlocks writes a block list through the editor's save and reports what
// did not survive the cleaning, which drops empty blocks and invalid values.
func saveBlocks(c Call, d Deps, p *page.Page, version int64, blocks []block.Block) (any, error) {
	ops, err := d.pageOps()
	if err != nil {
		return nil, err
	}
	if len(blocks) > block.MaxBlocks {
		return nil, fmt.Errorf("a page takes at most %d blocks", block.MaxBlocks)
	}
	after, err := ops.OpSetPageBlocks(c.Ctx, p.WebsiteID, p.ID, version, blocks)
	if err != nil {
		return nil, pageErr(err)
	}
	changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, after, map[string]any{"blocks": true}))

	stored, _ := block.Decode(after.Blocks, ops.OpBlockSet(c.Ctx, p.WebsiteID))
	out := brief(*after)
	out["version"] = after.Version
	out["blocks"] = len(stored)
	if len(stored) < len(blocks) {
		out["note"] = fmt.Sprintf("%d of %d blocks were empty and left out; get_page_blocks "+
			"shows what was stored.", len(blocks)-len(stored), len(blocks))
	}
	return out, nil
}

func setPageBlocks(d Deps) Tool {
	return Tool{
		Name:   "set_page_blocks",
		Writes: true,
		Description: "Replaces the whole block list of a page and renders it, exactly as a save " +
			"in the block editor does. Empty blocks are left out. The previous state is kept as " +
			"a revision; the status does not change. list_block_kinds says which types and " +
			"fields there are.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":      {Type: "integer", Description: "id of the page"},
				"blocks":  {Type: "array", Description: "the blocks, in order", Items: &blockSchema},
				"version": versionProperty,
			},
			Required: []string{"id", "blocks"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64       `json:"id"`
				Blocks  []wireBlock `json:"blocks"`
				Version int64       `json:"version"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			blocks := make([]block.Block, 0, len(a.Blocks))
			for _, w := range a.Blocks {
				blocks = append(blocks, storedBlock(w))
			}
			return saveBlocks(c, d, p, a.Version, blocks)
		},
	}
}

func insertBlock(d Deps) Tool {
	return Tool{
		Name:   "insert_block",
		Writes: true,
		Description: "Inserts one block into a page at a position, keeping the others. " +
			"A markdown page becomes a block page whose first block is its text.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":    {Type: "integer", Description: "id of the page"},
				"block": blockSchema,
				"position": {Type: "integer", Description: "0 puts it first, 1 after the first " +
					"block, and so on; leave out to append it at the end"},
				"version": versionProperty,
			},
			Required: []string{"id", "block"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID       int64     `json:"id"`
				Block    wireBlock `json:"block"`
				Position *int      `json:"position"`
				Version  int64     `json:"version"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			blocks, err := block.Decode(p.Blocks, ops.OpBlockSet(c.Ctx, p.WebsiteID))
			if err != nil {
				return nil, err
			}
			if blocks == nil && strings.TrimSpace(p.ContentMarkdown) != "" {
				// The editor's "+ Element" under the markdown editor: the text
				// becomes the first block and the new one follows.
				blocks = block.FromMarkdown(p.ContentMarkdown)
			}
			at := len(blocks)
			if a.Position != nil {
				if *a.Position < 0 || *a.Position > len(blocks) {
					return nil, fmt.Errorf("position must lie between 0 and %d", len(blocks))
				}
				at = *a.Position
			}
			next := make([]block.Block, 0, len(blocks)+1)
			next = append(next, blocks[:at]...)
			next = append(next, storedBlock(a.Block))
			next = append(next, blocks[at:]...)
			version := a.Version
			if version == 0 {
				// The list was read here, so the save must be against the page
				// as it was read — not whatever it is a moment later.
				version = p.Version
			}
			return saveBlocks(c, d, p, version, next)
		},
	}
}

// --- side-panel settings ----------------------------------------------------

// parseWhen reads a schedule bound: RFC 3339, or the editor's own
// "2006-01-02T15:04", which is UTC.
func parseWhen(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00", "2006-01-02T15:04", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			utc := t.UTC()
			return &utc, nil
		}
	}
	return nil, fmt.Errorf("%q is not a date; write it as 2026-10-01T08:00Z", raw)
}

func setPageSchedule(d Deps) Tool {
	return Tool{
		Name:   "set_page_schedule",
		Writes: true,
		Description: "Sets when a published page is visible: from publish_at and until " +
			"unpublish_at, both UTC. An empty string removes a bound, a bound left out stays. " +
			"The page still has to be published for either to matter.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":           {Type: "integer", Description: "id of the page"},
				"publish_at":   {Type: "string", Description: "visible from, e.g. 2026-10-01T08:00Z; empty removes it"},
				"unpublish_at": {Type: "string", Description: "visible until, UTC; empty removes it"},
				"version":      versionProperty,
			},
			Required: []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID          int64   `json:"id"`
				PublishAt   *string `json:"publish_at"`
				UnpublishAt *string `json:"unpublish_at"`
				Version     int64   `json:"version"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			after, err := ops.OpEditPage(c.Ctx, p.WebsiteID, p.ID, a.Version, func(_ *page.Page, u *page.PageUpdate) error {
				if a.PublishAt != nil {
					t, err := parseWhen(*a.PublishAt)
					if err != nil {
						return err
					}
					u.Schedule.PublishAt = t
				}
				if a.UnpublishAt != nil {
					t, err := parseWhen(*a.UnpublishAt)
					if err != nil {
						return err
					}
					u.Schedule.UnpublishAt = t
				}
				// The editor's rule, in the editor's words.
				if s := u.Schedule; s.PublishAt != nil && s.UnpublishAt != nil && !s.UnpublishAt.After(*s.PublishAt) {
					return errors.New("the end must come after the start")
				}
				return nil
			})
			if err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, after, map[string]any{"schedule": true}))
			out := brief(*after)
			out["publish_at"], out["unpublish_at"] = stamp(after.PublishAt), stamp(after.UnpublishAt)
			if after.Status != "published" {
				out["note"] = "The page is a draft; the schedule applies once it is published."
			}
			return out, nil
		},
	}
}

func setPageProtection(d Deps) Tool {
	return Tool{
		Name:   "set_page_protection",
		Writes: true,
		Description: "Puts a page behind a password, changes the password or the hint, or " +
			"removes the protection. Without a new password the existing one stays; switching " +
			"it on needs a password the first time.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":        {Type: "integer", Description: "id of the page"},
				"protected": {Type: "boolean", Description: "true puts it behind a password, false removes the protection"},
				"password": {Type: "string", Description: fmt.Sprintf("a new password, at least %d "+
					"characters; leave out to keep the current one", page.MinPagePasswordLength)},
				"hint": {Type: "string", Description: "the line visitors read above the password form"},
			},
			Required: []string{"id", "protected"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID        int64   `json:"id"`
				Protected bool    `json:"protected"`
				Password  string  `json:"password"`
				Hint      *string `json:"hint"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			hint := p.AccessHint
			if a.Hint != nil {
				hint = *a.Hint
			}
			after, err := ops.OpSetPageAccess(c.Ctx, p.WebsiteID, p.ID, a.Protected, a.Password, hint)
			switch {
			case errors.Is(err, page.ErrNoPagePassword):
				return nil, errors.New("this page has no password yet; give one")
			case err != nil:
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, after, map[string]any{"protection": a.Protected}))
			out := brief(*after)
			out["protected"], out["hint"] = after.Protected(), after.AccessHint
			return out, nil
		},
	}
}

func setPageSEO(d Deps) Tool {
	return Tool{
		Name:   "set_page_seo",
		Writes: true,
		Description: "Changes what search engines and link previews see: the excerpt, the meta " +
			"description, the preview image and whether search engines should leave the page " +
			"out. A value left out stays as it is.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id": {Type: "integer", Description: "id of the page"},
				"excerpt": {Type: "string", Description: "short summary for listings; empty " +
					"derives it from the text"},
				"meta_description": {Type: "string", Description: "text for search engines; empty uses the excerpt"},
				"featured_media":   {Type: "integer", Description: "id of an image from list_media; 0 removes it"},
				"noindex":          {Type: "boolean", Description: "true asks search engines to leave the page out"},
				"version":          versionProperty,
			},
			Required: []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID              int64   `json:"id"`
				Excerpt         *string `json:"excerpt"`
				MetaDescription *string `json:"meta_description"`
				FeaturedMedia   *int64  `json:"featured_media"`
				NoIndex         *bool   `json:"noindex"`
				Version         int64   `json:"version"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			if a.FeaturedMedia != nil && *a.FeaturedMedia != 0 {
				m, err := d.Media.GetByID(c.Ctx, *a.FeaturedMedia)
				if err != nil || m == nil || m.WebsiteID != p.WebsiteID || !m.IsImage() {
					return nil, errors.New("the preview image has to be an image from this website's library")
				}
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			after, err := ops.OpEditPage(c.Ctx, p.WebsiteID, p.ID, a.Version, func(cur *page.Page, u *page.PageUpdate) error {
				if a.Excerpt != nil {
					u.Meta.Excerpt = strings.TrimSpace(*a.Excerpt)
					// The editor's rule: an empty excerpt is derived, never
					// stored empty.
					if u.Meta.Excerpt == "" {
						u.Meta.Excerpt = page.Excerpt(cur.ContentMarkdown)
					}
				}
				if a.MetaDescription != nil {
					u.Meta.MetaDescription = strings.TrimSpace(*a.MetaDescription)
				}
				if a.FeaturedMedia != nil {
					u.Meta.FeaturedMediaID = nil
					if id := *a.FeaturedMedia; id > 0 {
						u.Meta.FeaturedMediaID = &id
					}
				}
				if a.NoIndex != nil {
					u.Meta.NoIndex = *a.NoIndex
				}
				return nil
			})
			if err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, after, map[string]any{"seo": true}))
			var featured any
			if after.FeaturedMediaID != nil {
				featured = *after.FeaturedMediaID
			}
			out := brief(*after)
			out["seo"] = map[string]any{
				"excerpt": after.Excerpt, "meta_description": after.MetaDescription,
				"featured_media": featured, "noindex": after.NoIndex,
			}
			return out, nil
		},
	}
}

func setPageTerms(d Deps) Tool {
	return Tool{
		Name:   "set_page_terms",
		Writes: true,
		Description: "Replaces the labels (tags) of a page. Labels that do not exist yet are " +
			"created; an empty list removes them all. At most 12.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":     {Type: "integer", Description: "id of the page"},
				"labels": {Type: "array", Description: "the label names", Items: &Property{Type: "string"}},
			},
			Required: []string{"id", "labels"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID     int64    `json:"id"`
				Labels []string `json:"labels"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			stored, err := ops.OpSetPageTerms(c.Ctx, p.WebsiteID, p.ID, a.Labels)
			if err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, p, map[string]any{"labels": stored}))
			if stored == nil {
				stored = []string{}
			}
			out := brief(*p)
			out["labels"] = stored
			return out, nil
		},
	}
}

func setPageKind(d Deps) Tool {
	return Tool{
		Name:   "set_page_kind",
		Writes: true,
		Description: "Files a page under another kind: page, post (listed in the archive by " +
			"date) or one of the website's own kinds. The address stays. Fields the new kind " +
			"does not have are dropped, as on a save in the editor.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":      {Type: "integer", Description: "id of the page"},
				"kind":    {Type: "string", Description: "page, post or the key of an own kind"},
				"version": versionProperty,
			},
			Required: []string{"id", "kind"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64  `json:"id"`
				Kind    string `json:"kind"`
				Version int64  `json:"version"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			after, err := ops.OpSetPageKind(c.Ctx, p.WebsiteID, p.ID, a.Version, a.Kind)
			if err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, after, map[string]any{"kind": wireKindOf(*after)}))
			return brief(*after), nil
		},
	}
}

func setPageSlug(d Deps) Tool {
	return Tool{
		Name:   "set_page_slug",
		Writes: true,
		Description: "Gives a page a new address (slug). The old address forwards to the new " +
			"one automatically, so existing links keep working.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":      {Type: "integer", Description: "id of the page"},
				"slug":    {Type: "string", Description: "the new slug, without a leading slash"},
				"version": versionProperty,
			},
			Required: []string{"id", "slug"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64  `json:"id"`
				Slug    string `json:"slug"`
				Version int64  `json:"version"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			after, err := ops.OpSetPageSlug(c.Ctx, p.WebsiteID, p.ID, a.Version, a.Slug)
			if err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, after, map[string]any{"previous_slug": p.Slug}))
			note := ""
			if after.Slug != p.Slug {
				note = "/" + p.Slug + " now forwards to /" + after.Slug + "."
			}
			return withNote(*after, note), nil
		},
	}
}

// --- languages --------------------------------------------------------------

func listTranslations(d Deps) Tool {
	return Tool{
		Name: "list_translations",
		Description: "Lists the versions of a page in the website's languages, the missing " +
			"ones included — create_translation fills a gap.",
		InputSchema: idOnly,
		Run: func(c Call) (any, error) {
			var a idArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ws, err := d.Domains.GetWebsite(c.Ctx, p.WebsiteID)
			if err != nil || ws == nil {
				return nil, errors.New("the website cannot be read")
			}
			if !ws.Multilingual() {
				return map[string]any{"page": p.ID, "languages": []any{},
					"note": "This website has one language."}, nil
			}
			group, err := d.Pages.TranslationsForEditor(c.Ctx, ws.ID, p)
			if err != nil {
				return nil, err
			}
			byLocale := map[string]page.Page{}
			for _, t := range group {
				byLocale[t.Locale] = t
			}
			out := []map[string]any{}
			for _, tag := range append([]string{""}, ws.Locales()...) {
				e := map[string]any{"language": tag, "main": tag == ""}
				if tag == "" {
					e["language"] = ws.Locale
				}
				e["name"] = locale.Name(e["language"].(string))
				if t, ok := byLocale[tag]; ok {
					e["page"], e["title"], e["slug"], e["status"] = t.ID, t.Title, t.Slug, wireStatus(t.Status)
				} else {
					e["missing"] = true
				}
				out = append(out, e)
			}
			return map[string]any{"page": p.ID, "languages": out}, nil
		},
	}
}

func createTranslation(d Deps) Tool {
	return Tool{
		Name:   "create_translation",
		Writes: true,
		Description: "Creates the version of a page in another language of the website, as a " +
			"draft copy of the original with a placeholder address (\"contact-fr\"). Translate " +
			"its text with update_page or set_page_blocks and its address with set_page_slug.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":       {Type: "integer", Description: "id of the page to translate"},
				"language": {Type: "string", Description: "a language tag of the website, such as fr"},
			},
			Required: []string{"id", "language"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID       int64  `json:"id"`
				Language string `json:"language"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			created, err := ops.OpTranslatePage(c.Ctx, p.WebsiteID, p.ID, a.Language)
			if err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageCreate, created,
				map[string]any{"translation_of": p.ID, "language": created.Locale}))
			return withNote(*created, "Created as a draft copy of the original — now translate "+
				"the text and the address."), nil
		},
	}
}

// --- sharing, review, bulk --------------------------------------------------

func createShareLink(d Deps) Tool {
	return Tool{
		Name: "create_share_link",
		// A write although it stores nothing: the link shows a draft to anybody
		// who has it, which is not something a read-only key may hand out.
		Writes: true,
		Description: "Makes a preview link that shows the page, draft or not, to somebody " +
			"without an account, until it expires. Only hand it to whom it is meant for.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":   {Type: "integer", Description: "id of the page"},
				"days": {Type: "integer", Description: "how long it works; 7 by default, at most 90"},
			},
			Required: []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID   int64 `json:"id"`
				Days int   `json:"days"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, err := d.pageOps()
			if err != nil {
				return nil, err
			}
			link, expires, err := ops.OpShareLink(c.Ctx, p.WebsiteID, p.ID, a.Days)
			if err != nil {
				return nil, pageErr(err)
			}
			out := map[string]any{"page": p.ID, "url": link, "expires": expires.Format(timeLayout)}
			if strings.HasPrefix(link, "/") {
				out["note"] = "The website has no primary domain, so this is only the path; " +
					"put the website's address in front of it."
			}
			return out, nil
		},
	}
}

func setReview(d Deps) Tool {
	return Tool{
		Name:   "set_review",
		Writes: true,
		Description: "Marks a page as waiting for review, so it shows up under \"waiting for " +
			"review\" in the page list, or removes the mark. The next save removes it too.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":      {Type: "integer", Description: "id of the page"},
				"pending": {Type: "boolean", Description: "true submits it for review, false removes the mark"},
			},
			Required: []string{"id", "pending"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64 `json:"id"`
				Pending bool  `json:"pending"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := livePageOf(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			state := "none"
			if a.Pending {
				state = "pending"
			}
			if err := d.Pages.SetReviewState(c.Ctx, p.ID, state); err != nil {
				return nil, pageErr(err)
			}
			changed(d, c, p.WebsiteID, pageEntry(activity.ActionPageUpdate, p, map[string]any{"review": state}))
			out := brief(*p)
			out["review"] = state
			return out, nil
		},
	}
}

func bulkPages(d Deps) Tool {
	return Tool{
		Name:   "bulk_pages",
		Writes: true,
		Description: "Publishes, withdraws (back to draft) or moves to the wastebasket several " +
			"pages of one website at once, like the bulk menu of the page list. Pages of " +
			"another website or already in the wastebasket are skipped and named. Publish " +
			"only when you were expressly asked to.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"action": {Type: "string", Description: "publish, unpublish or trash",
					Enum: []string{"publish", "unpublish", "trash"}},
				"ids": {Type: "array", Description: "ids of the pages", Items: &Property{Type: "integer"}},
			},
			Required: []string{"website", "action", "ids"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64   `json:"website"`
				Action  string  `json:"action"`
				IDs     []int64 `json:"ids"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			if a.Action != "publish" && a.Action != "unpublish" && a.Action != "trash" {
				return nil, errors.New("action is publish, unpublish or trash")
			}
			if len(a.IDs) == 0 {
				return nil, errors.New("no page given")
			}
			if len(a.IDs) > 200 {
				return nil, errors.New("at most 200 pages at once")
			}
			var ops pageOps
			if a.Action == "trash" {
				var err error
				if ops, err = d.pageOps(); err != nil {
					return nil, err
				}
			}

			done := []int64{}
			skipped := []map[string]any{}
			for _, id := range a.IDs {
				p, err := d.Pages.GetPage(c.Ctx, id)
				if err != nil {
					return nil, err
				}
				// The guard the screen has: a page of another website is not
				// touched, whatever the key could otherwise reach.
				if p == nil || p.WebsiteID != a.Website || p.InTrash() {
					skipped = append(skipped, map[string]any{"id": id, "reason": "not a live page of this website"})
					continue
				}
				action := activity.ActionPageDelete
				switch a.Action {
				case "publish":
					action = activity.ActionPagePublish
					err = d.Pages.SetPageStatus(c.Ctx, id, "published", nil)
				case "unpublish":
					action = activity.ActionPageUnpublish
					err = d.Pages.SetPageStatus(c.Ctx, id, "draft", nil)
				case "trash":
					_, err = ops.OpTrashPage(c.Ctx, a.Website, id)
				}
				if err != nil {
					skipped = append(skipped, map[string]any{"id": id, "reason": pageErr(err).Error()})
					continue
				}
				changed(d, c, a.Website, pageEntry(action, p, nil))
				done = append(done, id)
			}
			return map[string]any{"action": a.Action, "done": done, "skipped": skipped}, nil
		},
	}
}
