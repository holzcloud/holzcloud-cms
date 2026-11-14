package admin

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The states a cell of the legal card can be in.
const (
	legalMissing   = "missing"
	legalDraft     = "draft"
	legalPublished = "published"
	legalTemplate  = "template"
)

// legalCell is one page of one kind in one language, as the card shows it.
//
// It holds the kind, a state and an id. The title and the text of a page are
// the editors' own writing, and the card has no way to print them: a page that
// a lawyer rewrote is nobody else's business on the settings screen.
type legalCell struct {
	Kind   privacy.Kind
	State  string
	PageID int64
}

// legalRow is one language of the website.
type legalRow struct {
	// Tag is the language tag as the website names it, the main one included.
	Tag string
	// Main marks the website's main language, whose pages carry no locale.
	Main  bool
	Cells []legalCell
	// Hint is a catalogue key that says something about this language's
	// templates, or empty.
	Hint string
}

// legalCard is everything the "Legal pages" card on the settings screen shows.
type legalCard struct {
	Kinds []privacy.Kind
	Rows  []legalRow
	// Missing counts the cells without a page; the button that creates them
	// is only offered when there is something to create.
	Missing int
	// Frameworks are the chosen frameworks, for the sentence above the table.
	Frameworks []legal.Framework
	// NoFrameworks says that none is chosen, which the card reports instead of
	// naming them.
	NoFrameworks bool
}

// legalKinds are the pages a website's frameworks call for: the privacy page
// and the imprint always (D-09, D-13), the Do-Not-Sell page only with the
// United States.
func legalKinds(ws *domain.Website) []privacy.Kind {
	kinds := []privacy.Kind{privacy.KindPrivacy, privacy.KindImprint}
	if ws.HasUSFramework() {
		kinds = append(kinds, privacy.KindDNS)
	}
	return kinds
}

// legalComposers makes the template of each kind of page.
var legalComposers = map[privacy.Kind]func(privacy.Request) string{
	privacy.KindPrivacy: privacy.ComposePrivacy,
	privacy.KindImprint: privacy.ComposeImprint,
	privacy.KindDNS:     privacy.ComposeDNS,
}

// pageLocale is the locale a page in the language tag is stored with: empty
// for the website's main language, the tag for every other one.
func pageLocale(ws *domain.Website, tag string) string {
	if tag == ws.Locale {
		return ""
	}
	return tag
}

// legalCard looks at the legal pages of every language the card shows.
//
// An existing page is found by any of the slugs its kind has in any language
// (privacy.RoleSlugs), its own language's slug first. An operator who called
// the privacy page "datenschutzerklaerung" has a privacy page, and offering a
// second one beside it would be wrong.
func (h *Handler) legalCard(ctx context.Context, ws *domain.Website) (legalCard, error) {
	card := legalCard{
		Kinds:        legalKinds(ws),
		Frameworks:   ws.Frameworks().List(),
		NoFrameworks: ws.Frameworks().Empty(),
	}
	for _, tag := range []string{ws.Locale} {
		row := legalRow{Tag: tag, Main: tag == ws.Locale}
		for _, k := range card.Kinds {
			p, err := h.findLegalPage(ctx, ws, k, tag)
			if err != nil {
				return legalCard{}, err
			}
			cell := legalCell{Kind: k, State: legalMissing}
			if p != nil {
				cell.PageID = p.ID
				switch {
				case privacy.HasTemplateNotice(p.ContentMarkdown):
					cell.State = legalTemplate
				case p.Status == "published":
					cell.State = legalPublished
				default:
					cell.State = legalDraft
				}
			} else {
				card.Missing++
			}
			row.Cells = append(row.Cells, cell)
		}
		card.Rows = append(card.Rows, row)
	}
	return card, nil
}

// findLegalPage is the page of kind k in the language tag, or nil.
func (h *Handler) findLegalPage(ctx context.Context, ws *domain.Website, k privacy.Kind, tag string) (*page.Page, error) {
	loc := pageLocale(ws, tag)
	slugs := append([]string{privacy.Slug(k, tag)}, privacy.RoleSlugs(k)...)
	for _, s := range slugs {
		p, err := h.pages.GetPageBySlugIn(ctx, ws.ID, loc, s)
		if err != nil {
			return nil, fmt.Errorf("look up %s page: %w", k, err)
		}
		if p != nil {
			return p, nil
		}
	}
	return nil, nil
}

// HandleLegalDrafts creates legal pages as drafts for the website named in the
// path.
//
// Without fields it creates every page the card shows as missing, each in its
// language, with the title and the address of that language's texts. With art
// and sprache it creates the current template of that kind and language as one
// more draft under its own address, next to whatever page exists, so that the
// operator can compare the two (Phase 30's offer, now for every kind).
//
// An existing page is never changed (D-01). It may be published, it may have
// been rewritten by a lawyer; it is the operator's text. This file only looks
// pages up and creates new ones, and a test holds the list of page-store calls
// to exactly those two. Drafts publish nothing, so an editor without the
// publishing right may ask for them too.
func (h *Handler) HandleLegalDrafts(w http.ResponseWriter, r *http.Request) error {
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil
	}

	if r.PostForm.Has("art") || r.PostForm.Has("sprache") {
		return h.createCompareDraft(w, r, ws)
	}
	return h.createMissingLegalPages(w, r, ws)
}

// createCompareDraft makes the current template of one kind and language as a
// new draft next to the existing page.
func (h *Handler) createCompareDraft(w http.ResponseWriter, r *http.Request, ws *domain.Website) error {
	k := privacy.Kind(r.PostForm.Get("art"))
	tag := r.PostForm.Get("sprache")
	// Both come from the client. The kind has to be one this website's
	// frameworks call for and the language one it publishes in; anything else
	// is a page that cannot exist here.
	if !slices.Contains(legalKinds(ws), k) || !slices.Contains(ws.AllLocales(), tag) {
		http.NotFound(w, r)
		return nil
	}
	created, err := h.createLegalDraft(r, ws, k, tag,
		privacy.DraftTitle(k, tag), privacy.DraftSlug(k, tag), 0)
	if err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Legal template created as a draft")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", ws.ID, created.ID))
}

// createMissingLegalPages makes every page the card shows as missing.
func (h *Handler) createMissingLegalPages(w http.ResponseWriter, r *http.Request, ws *domain.Website) error {
	card, err := h.legalCard(r.Context(), ws)
	if err != nil {
		return err
	}
	var created []*page.Page
	for _, row := range card.Rows {
		for _, cell := range row.Cells {
			if cell.State != legalMissing {
				continue
			}
			p, err := h.createLegalDraft(r, ws, cell.Kind, row.Tag,
				privacy.Title(cell.Kind, row.Tag), privacy.Slug(cell.Kind, row.Tag), 0)
			if err != nil {
				return err
			}
			created = append(created, p)
		}
	}

	switch len(created) {
	case 0:
		web.SetFlashSuccess(h.sm, r.Context(), "No legal page was missing.")
	case 1:
		web.SetFlashSuccess(h.sm, r.Context(), "Legal template created as a draft")
		return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", ws.ID, created[0].ID))
	default:
		web.SetFlashSuccess(h.sm, r.Context(), web.Titlef(r, "%d legal pages created as drafts", len(created)))
	}
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d#legal-pages", ws.ID))
}

// createLegalDraft composes the template of kind k for the language tag and
// stores it as a new draft. translationOf names the main-language page it
// translates, or is zero.
func (h *Handler) createLegalDraft(r *http.Request, ws *domain.Website, k privacy.Kind, tag, title, slug string, translationOf int64) (*page.Page, error) {
	req, err := h.legalRequest(r.Context(), ws, tag)
	if err != nil {
		return nil, err
	}
	markdown := legalComposers[k](req)
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		return nil, fmt.Errorf("render %s draft: %w", k, err)
	}
	created, err := h.pages.CreatePage(r.Context(), page.PageCreate{
		WebsiteID:     ws.ID,
		Title:         title,
		Slug:          slug,
		Markdown:      markdown,
		HTML:          html,
		Status:        "draft",
		Locale:        pageLocale(ws, tag),
		TranslationOf: translationOf,
		Meta:          page.PageMeta{Excerpt: page.Excerpt(markdown)},
		UserID:        h.currentUserID(r),
	})
	if err != nil {
		return nil, err
	}
	h.emitPageSaved(r, ws.ID, created, true)
	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPageCreate,
		EntityType: "page",
		EntityID:   created.ID,
		WebsiteID:  &ws.ID,
		Metadata:   map[string]any{"slug": created.Slug, "titel": created.Title},
	})
	return created, nil
}
