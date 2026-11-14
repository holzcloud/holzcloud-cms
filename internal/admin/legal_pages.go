package admin

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
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
	// Name is the language's name, a catalogue key.
	Name string
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

// Columns is the number of columns of the card's table: the language and
// one per kind.
func (c legalCard) Columns() int { return len(c.Kinds) + 1 }

// legalRowHint is what the card says under a language whose templates are not
// what its name suggests, or empty.
//
// A Spanish page gets the English texts (D-16), and the operator should know
// that before publishing an English page on a Spanish website. The Romansh
// texts were written by a machine (D-07); the page itself says so as well, but
// the card is where the decision to create it is made.
func legalRowHint(tag string) string {
	base, _, _ := strings.Cut(tag, "-")
	switch strings.ToLower(base) {
	case "es":
		return i18n.N("Spanish pages get the English template.")
	case "rm":
		return i18n.N("The Romansh template was written by a machine — have it checked by a Romansh speaker before publishing.")
	}
	return ""
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

// legalCard looks at the legal pages of every language of the website, the
// main one first (D-08).
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
	for _, tag := range ws.AllLocales() {
		row := legalRow{Tag: tag, Name: locale.Name(tag), Main: tag == ws.Locale, Hint: legalRowHint(tag)}
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
//
// In a further language the translation group comes first: whatever page in
// that language is linked to the main-language page of the kind is that
// language's page, whatever its address. Looking only at role slugs missed an
// English page called "privacy-statement" in the group of "datenschutz", and
// "create the missing pages" then filed a second English page into the same
// group (WR-04 of the 33.1 review) — a group holds one page per language, and
// with two the language switcher and hreflang would have to pick one.
func (h *Handler) findLegalPage(ctx context.Context, ws *domain.Website, k privacy.Kind, tag string) (*page.Page, error) {
	loc := pageLocale(ws, tag)
	if loc != "" {
		mainPage, err := h.findLegalPage(ctx, ws, k, ws.Locale)
		if err != nil {
			return nil, err
		}
		if mainPage != nil {
			group, err := h.pages.TranslationsForEditor(ctx, ws.ID, mainPage)
			if err != nil {
				return nil, fmt.Errorf("look up %s translations: %w", k, err)
			}
			for i := range group {
				if group[i].Locale == loc {
					return &group[i], nil
				}
			}
		}
	}
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
//
// The draft is not filed into the translation group: the group already has its
// page in that language, and the draft is there to be compared and thrown
// away, not to stand in for it.
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
//
// It walks the main language first, so that a page in a further language can
// be filed as a translation of the main-language page of the same kind (D-08):
// the one that was found, or the one this request just made. With neither the
// draft stands alone. The link is set through PageCreate.TranslationOf rather
// than the page store's separate linking call, which checks a target that
// arrives from a form: here the target is a page of the same website that this
// handler has just found or made, and the list of page-store calls that a test
// holds stays at two.
func (h *Handler) createMissingLegalPages(w http.ResponseWriter, r *http.Request, ws *domain.Website) error {
	card, err := h.legalCard(r.Context(), ws)
	if err != nil {
		return err
	}
	mainPage := map[privacy.Kind]int64{}
	var created []*page.Page
	for _, row := range card.Rows {
		for _, cell := range row.Cells {
			if cell.State != legalMissing {
				if row.Main {
					mainPage[cell.Kind] = cell.PageID
				}
				continue
			}
			var of int64
			if !row.Main {
				of = mainPage[cell.Kind]
			}
			p, err := h.createLegalDraft(r, ws, cell.Kind, row.Tag,
				privacy.Title(cell.Kind, row.Tag), privacy.Slug(cell.Kind, row.Tag), of)
			if err != nil {
				return err
			}
			if row.Main {
				mainPage[cell.Kind] = p.ID
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
