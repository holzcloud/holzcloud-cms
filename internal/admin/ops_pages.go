package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The page editor, the page list and the wastebasket, for the MCP tools.
//
// Each Op here is what a screen does minus the request: the same store calls,
// the same rendering of blocks, the same plugin events. Where a screen had its
// steps inline they have moved into a small function both call, so an
// assistant's save and an editor's save cannot come to differ.
//
// The errors are plain English and meant for a machine. The screens keep their
// own flash messages, which go through the catalogue.

// ErrUnknownBlockType is returned for a block whose type this website does not
// have.
var ErrUnknownBlockType = errors.New("unknown block type")

// ErrForeignMedia is returned for a media id that is not in this website's
// library, or is not the kind of file the block needs.
var ErrForeignMedia = errors.New("media file not in this website's library")

// ErrNoBlocks is returned when nothing of a block list survives cleaning.
var ErrNoBlocks = errors.New("no block is left that would show anything")

// ErrNoLanguage is returned for a language the website does not have.
var ErrNoLanguage = errors.New("this website does not have that language")

// ErrTranslationExists is returned when the language is already covered.
var ErrTranslationExists = errors.New("a version in that language exists already")

// ErrUnknownKind is returned for a content kind the website does not have.
var ErrUnknownKind = errors.New("this website has no such kind")

// ErrNotConfigured is returned when a part of the installation is not wired.
var ErrNotConfigured = errors.New("not available on this installation")

// livePage loads a page of one website that is not in the wastebasket, or
// page.ErrNotFound. The id comes from outside, so the website is checked here
// and not trusted.
func (h *Handler) livePage(ctx context.Context, websiteID, pageID int64) (*page.Page, error) {
	p, err := h.pages.GetPage(ctx, pageID)
	if err != nil {
		return nil, err
	}
	if p == nil || p.WebsiteID != websiteID || p.InTrash() {
		return nil, page.ErrNotFound
	}
	return p, nil
}

// keepUpdate is a save that changes nothing: every column the edit form
// carries, as it stands. A tool that changes one setting starts from here, so
// the schedule survives a new excerpt and the fields survive a new schedule.
func keepUpdate(p *page.Page) page.PageUpdate {
	return page.PageUpdate{
		Title: p.Title, Slug: p.Slug, Markdown: p.ContentMarkdown, HTML: p.ContentHTML,
		Status: p.Status, Blocks: p.Blocks, Fields: p.Fields,
		Meta: page.PageMeta{
			Excerpt: p.Excerpt, MetaDescription: p.MetaDescription,
			FeaturedMediaID: p.FeaturedMediaID, NoIndex: p.NoIndex,
		},
		Schedule:        page.PageSchedule{PublishAt: p.PublishAt, UnpublishAt: p.UnpublishAt},
		Kind:            p.Kind,
		TypeKey:         p.TypeKey,
		ExpectedVersion: p.Version,
	}
}

// OpBlockSet is the block kinds a website may use: the built-in nine and its
// own.
func (h *Handler) OpBlockSet(ctx context.Context, websiteID int64) block.Set {
	return h.blockSet(ctx, websiteID)
}

// OpEditPage saves one page the way the editor's save does, after edit has
// changed what it wants.
//
// edit receives the stored page and a save that would change nothing. version
// is the version the caller read, or zero for "the one there is now" — a
// mismatch is page.ErrConflict, exactly as in the editor. Afterwards the media
// bookkeeping is redone and the plugins hear of the save.
func (h *Handler) OpEditPage(ctx context.Context, websiteID, pageID, version int64, edit func(p *page.Page, u *page.PageUpdate) error) (*page.Page, error) {
	p, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return nil, err
	}
	u := keepUpdate(p)
	if version != 0 {
		u.ExpectedVersion = version
	}
	if err := edit(p, &u); err != nil {
		return nil, err
	}
	if err := h.pages.UpdatePage(ctx, p.ID, u); err != nil {
		return nil, err
	}
	h.recordMediaUsageCtx(ctx, websiteID, p.ID, u.HTML)
	h.emitPageSaved(nil, websiteID, &page.Page{ID: p.ID, Slug: u.Slug, Status: u.Status}, false)
	return h.pages.GetPage(ctx, p.ID)
}

// OpSetPageBlocks replaces the block list of a page and renders it, exactly as
// the editor's save does: cleaned against the website's kinds, encoded, the
// HTML and the plain text derived from it.
//
// Media ids are checked against the website before anything is written. The
// renderer would silently leave a foreign picture out; an assistant should be
// told instead, or it reports a gallery that nobody can see.
func (h *Handler) OpSetPageBlocks(ctx context.Context, websiteID, pageID, version int64, blocks []block.Block) (*page.Page, error) {
	set := h.blockSet(ctx, websiteID)
	look := h.blockImages(ctx, websiteID)
	for i := range blocks {
		b := &blocks[i]
		if _, ok := set.KindOf(b.Type); !ok {
			return nil, fmt.Errorf("block %d: %w: %q", i+1, ErrUnknownBlockType, b.Type)
		}
		if err := checkBlockMedia(i, *b, look); err != nil {
			return nil, err
		}
		// The same derivation the form applies: the slug is what the marker
		// carries, and nothing else may reach it.
		if strings.TrimSpace(b.AlbumSlug) != "" {
			b.AlbumSlug = page.Slugify(b.AlbumSlug)
		}
	}

	markdown, html, encoded, err := h.blockContent(ctx, websiteID, pageValues{BlockSet: set, Blocks: blocks})
	if err != nil {
		return nil, err
	}
	if encoded == "" {
		return nil, ErrNoBlocks
	}
	return h.OpEditPage(ctx, websiteID, pageID, version, func(_ *page.Page, u *page.PageUpdate) error {
		u.Markdown, u.HTML, u.Blocks = markdown, html, encoded
		return nil
	})
}

// checkBlockMedia refuses a picture or a film of another website, or of the
// wrong kind for its place.
func checkBlockMedia(at int, b block.Block, look block.Lookup) error {
	want := func(id int64, film bool, what string) error {
		if id == 0 {
			return nil
		}
		img, ok := look(id)
		if !ok || img.Film != film {
			return fmt.Errorf("block %d, %s %d: %w", at+1, what, id, ErrForeignMedia)
		}
		return nil
	}
	film := b.Type == block.TypeVideo
	if err := want(b.MediaID, film, "media"); err != nil {
		return err
	}
	if err := want(b.PosterID, false, "poster"); err != nil {
		return err
	}
	for _, it := range b.Items {
		if err := want(it.MediaID, false, "item media"); err != nil {
			return err
		}
	}
	return nil
}

// OpTrashPage moves a page to the wastebasket.
func (h *Handler) OpTrashPage(ctx context.Context, websiteID, pageID int64) (*page.Page, error) {
	p, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return nil, err
	}
	if err := h.trashPage(ctx, websiteID, p); err != nil {
		return nil, err
	}
	return p, nil
}

// trashPage is the delete button: into the wastebasket, and the plugins told.
func (h *Handler) trashPage(ctx context.Context, websiteID int64, p *page.Page) error {
	if err := h.pages.TrashPage(ctx, p.ID); err != nil && !errors.Is(err, page.ErrNotFound) {
		return err
	}
	h.emitPageDeleted(websiteID, p)
	return nil
}

// OpRestoreRevision puts an older version back as the current one.
func (h *Handler) OpRestoreRevision(ctx context.Context, websiteID, pageID, revisionID int64) (*page.Page, *page.Revision, error) {
	p, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return nil, nil, err
	}
	rev, err := h.pages.GetRevision(ctx, revisionID)
	if err != nil {
		return nil, nil, err
	}
	if rev == nil || rev.PageID != p.ID {
		return nil, nil, page.ErrNotFound
	}
	if err := h.restoreRevision(ctx, p, rev, nil); err != nil {
		return nil, nil, err
	}
	after, err := h.pages.GetPage(ctx, p.ID)
	return after, rev, err
}

// restoreRevision writes a revision's title and content over the page.
//
// Only what a revision holds moves back: title, text and blocks. Everything a
// revision does not record — the excerpt, the preview image, the schedule, the
// website's own fields — stays as it is. The restore used to send an update
// with those left empty, which cleared all of them, and it rendered the
// Markdown of a block page, which turned the blocks back into their plain text.
//
// The revision keeps the slug it had, but the address may have been taken over
// since, so the current one stays.
func (h *Handler) restoreRevision(ctx context.Context, p *page.Page, rev *page.Revision, userID *int64) error {
	u := keepUpdate(p)
	u.Title, u.Markdown, u.Blocks = rev.Title, rev.ContentMarkdown, ""
	u.UserID = userID

	set := h.blockSet(ctx, p.WebsiteID)
	blocks, err := block.Decode(rev.Blocks, set)
	if err != nil {
		return err
	}
	if blocks != nil {
		markdown, html, encoded, err := h.blockContent(ctx, p.WebsiteID, pageValues{BlockSet: set, Blocks: blocks})
		if err != nil {
			return err
		}
		u.Markdown, u.HTML, u.Blocks = markdown, html, encoded
	} else {
		if u.HTML, err = page.RenderMarkdown(rev.ContentMarkdown); err != nil {
			return err
		}
	}
	if err := h.pages.UpdatePage(ctx, p.ID, u); err != nil {
		return err
	}
	h.recordMediaUsageCtx(ctx, p.WebsiteID, p.ID, u.HTML)
	return nil
}

// OpDuplicatePage copies a page as a new draft.
func (h *Handler) OpDuplicatePage(ctx context.Context, websiteID, pageID int64) (*page.Page, error) {
	src, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return nil, err
	}
	return h.duplicatePage(ctx, src, nil)
}

// duplicatePage is the copy the page list's "Duplicate" makes.
//
// The copy is always a draft, whatever the original was: duplicating a live
// page and instantly publishing an unedited copy of it is never what anyone
// meant. Blocks and the website's own fields come along — a copy of a block
// page that arrives as its plain text is not a copy.
func (h *Handler) duplicatePage(ctx context.Context, src *page.Page, userID *int64) (*page.Page, error) {
	created, err := h.pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: src.WebsiteID,
		// Through the catalogue and not glued on in Go: the suffix is a word
		// the operator reads and then edits.
		Title: i18n.Tf(i18n.Lang(ctx), "%s (copy)", src.Title),
		// CreatePage uniquifies the slug, so the copy lands on "kontakt-2"
		// rather than failing on the constraint.
		Slug:     src.Slug,
		Markdown: src.ContentMarkdown,
		HTML:     src.ContentHTML,
		Blocks:   src.Blocks,
		Fields:   src.Fields,
		Status:   "draft",
		Meta: page.PageMeta{
			Excerpt:         src.Excerpt,
			MetaDescription: src.MetaDescription,
			FeaturedMediaID: src.FeaturedMediaID,
			NoIndex:         src.NoIndex,
		},
		// A copy of a product is a product.
		Kind:    src.Kind,
		TypeKey: src.TypeKey,
		UserID:  userID,
	})
	if err != nil {
		return nil, err
	}
	h.recordMediaUsageCtx(ctx, src.WebsiteID, created.ID, created.ContentHTML)
	h.emitPageSaved(nil, src.WebsiteID, created, true)
	return created, nil
}

// OpTranslatePage starts the version of a page in another language, the way
// the editor's "Create version" does: a draft copy, filed under the language
// and in the page's translation group.
func (h *Handler) OpTranslatePage(ctx context.Context, websiteID, pageID int64, language string) (*page.Page, error) {
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	if ws == nil || !ws.Multilingual() {
		return nil, ErrNoLanguage
	}
	original, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return nil, err
	}
	tag := locale.Pick(language, ws.Locales())
	if tag == "" {
		return nil, ErrNoLanguage
	}
	// The screen only offers the button for a missing language. Here the
	// language arrives as a word, so the check the screen makes by leaving the
	// button out is made in so many words.
	group, err := h.pages.TranslationsForEditor(ctx, websiteID, original)
	if err != nil {
		return nil, err
	}
	for _, t := range group {
		if t.Locale == tag {
			return nil, fmt.Errorf("%w (page %d)", ErrTranslationExists, t.ID)
		}
	}
	return h.translatePage(ctx, ws, original, tag, nil)
}

// translatePage copies the original into a new draft in one language.
//
// It copies rather than opening an empty page: a translator works from the
// text. The copy is a draft, because everything about it is still the
// original's words.
func (h *Handler) translatePage(ctx context.Context, ws *domain.Website, original *page.Page, tag string, userID *int64) (*page.Page, error) {
	// The middle of the star: translating a translation still belongs to the
	// same group, not to a chain hanging off it.
	mitte := original.ID
	if original.TranslationOf != 0 {
		mitte = original.TranslationOf
	}
	created, err := h.pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID,
		Title:     original.Title,
		// Addresses are unique per website across all languages; the tag is a
		// placeholder the translator replaces.
		Slug:     translationSlug(original.Slug, tag),
		Markdown: original.ContentMarkdown,
		HTML:     original.ContentHTML,
		Blocks:   original.Blocks,
		Fields:   original.Fields,
		Status:   "draft",
		Meta: page.PageMeta{
			Excerpt:         original.Excerpt,
			MetaDescription: original.MetaDescription,
			FeaturedMediaID: original.FeaturedMediaID,
			NoIndex:         original.NoIndex,
		},
		Kind:    original.Kind,
		TypeKey: original.TypeKey,
		UserID:  userID,
	})
	if err != nil {
		return nil, err
	}
	if err := h.pages.SetTranslation(ctx, ws.ID, created.ID, tag, mitte); err != nil {
		return nil, err
	}
	return h.pages.GetPage(ctx, created.ID)
}

// OpShareLink mints a preview link for somebody without an account.
//
// days is bounded as on the screen; zero is the default lifetime. The address
// is absolute when the website has a primary domain, and only the path when it
// has none — there is no request here whose host could stand in for it.
func (h *Handler) OpShareLink(ctx context.Context, websiteID, pageID int64, days int) (string, time.Time, error) {
	p, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return "", time.Time{}, err
	}
	if h.share == nil {
		return "", time.Time{}, fmt.Errorf("share links: %w", ErrNotConfigured)
	}
	lifetime := sharelink.DefaultLifetime
	if days > 0 {
		lifetime = min(time.Duration(days)*24*time.Hour, sharelink.MaxLifetime)
	}
	expires := time.Now().UTC().Add(lifetime)
	path := sharelink.Path(h.share.Token(p.ID, expires))

	primary, err := h.domains.PrimaryDomain(ctx, websiteID)
	if err != nil || primary == "" {
		return path, expires, nil
	}
	scheme := "http"
	if h.cfg != nil && h.cfg.Secure {
		scheme = "https"
	}
	return scheme + "://" + primary + path, expires, nil
}

// OpSetPageAccess switches a page's password protection, with the same rules
// as the editor: an empty password keeps the one there is, and protection
// without any password is refused (page.ErrNoPagePassword).
func (h *Handler) OpSetPageAccess(ctx context.Context, websiteID, pageID int64, protected bool, password, hint string) (*page.Page, error) {
	p, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return nil, err
	}
	if err := h.pages.SetAccess(ctx, p.ID, page.AccessUpdate{
		Protected: protected, Password: password, Hint: strings.TrimSpace(hint),
	}, h.argon2Params); err != nil {
		return nil, err
	}
	return h.pages.GetPage(ctx, p.ID)
}

// OpPageTerms reads the labels of a page.
func (h *Handler) OpPageTerms(ctx context.Context, websiteID, pageID int64) ([]string, error) {
	p, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return nil, err
	}
	if h.terms == nil {
		return nil, nil
	}
	terms, err := h.terms.ForPage(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		out = append(out, t.Name)
	}
	return out, nil
}

// OpSetPageTerms replaces the labels of a page, read the way the editor's
// comma-separated field is read, and reports what was stored.
func (h *Handler) OpSetPageTerms(ctx context.Context, websiteID, pageID int64, names []string) ([]string, error) {
	p, err := h.livePage(ctx, websiteID, pageID)
	if err != nil {
		return nil, err
	}
	if h.terms == nil {
		return nil, fmt.Errorf("labels: %w", ErrNotConfigured)
	}
	if err := h.terms.SetForPage(ctx, websiteID, p.ID, term.Parse(strings.Join(names, ","))); err != nil {
		return nil, err
	}
	return h.OpPageTerms(ctx, websiteID, p.ID)
}

// OpSetPageKind files a page under another kind: "page", "post" or one of the
// website's own.
//
// The website's own fields are checked against the new kind the way the editor
// checks them on save, and the ones the new kind does not have are dropped —
// again as the editor's save does. A required field the new kind has and the
// page lacks refuses the change, with the reason.
func (h *Handler) OpSetPageKind(ctx context.Context, websiteID, pageID, version int64, key string) (*page.Page, error) {
	key = strings.TrimSpace(key)
	var types []kind.Type
	if h.kinds != nil {
		list, err := h.kinds.List(ctx, websiteID)
		if err != nil {
			return nil, err
		}
		types = list
	}
	values := pageValues{TypeKey: key}
	values.setKind(types)
	// setKind files anything unknown under "page", which is right for a form
	// and wrong for a word an assistant typed: it would believe it had made a
	// product.
	if values.KindValue() != key {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, key)
	}

	defs := h.fieldDefs(ctx, websiteID)
	return h.OpEditPage(ctx, websiteID, pageID, version, func(p *page.Page, u *page.PageUpdate) error {
		data := field.Decode(p.Fields)
		for _, reason := range checkFields(defs, values.KindValue(), data) {
			return errors.New(reason.Text(i18n.Lang(ctx)))
		}
		stored, err := field.Encode(field.Clean(field.For(defs, values.KindValue()), data))
		if err != nil {
			return err
		}
		u.Kind, u.TypeKey, u.Fields = values.Kind, values.TypeKey, stored
		return nil
	})
}

// OpSetPageSlug gives a page a new address, checked as the editor checks it.
// The store records the redirect from the old address in the same
// transaction.
func (h *Handler) OpSetPageSlug(ctx context.Context, websiteID, pageID, version int64, slug string) (*page.Page, error) {
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, page.ErrNotFound
	}
	return h.OpEditPage(ctx, websiteID, pageID, version, func(p *page.Page, u *page.PageUpdate) error {
		values := pageValues{Title: p.Title, Slug: strings.TrimSpace(strings.Trim(slug, "/"))}
		errs := web.FormErrors{}
		clean := values.validateOn(nil, errs, ws.BlogBase, ws.Locales())
		for _, msg := range errs {
			return errors.New(msg)
		}
		u.Slug = clean
		return nil
	})
}
