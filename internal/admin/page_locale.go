package admin

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// LanguageChoice is one language in the page form's dropdown.
type LanguageChoice struct {
	// Code is the stored value: empty for the main language.
	Code string
	// Name is what an editor reads: "Deutsch", "Französisch".
	Name string
	// Selected marks the language this page is in.
	Selected bool
	// Primary marks the website's main language. The note beside it is added
	// in the template rather than glued on here: a sentence assembled in Go
	// cannot be looked up in the catalogue.
	Primary bool
}

// TranslationView is one sibling of a page in the translation panel.
type TranslationView struct {
	PageID int64
	Name   string
	Title  string
	Status string
	// Self marks the page being edited, so the panel can show where you are
	// rather than offering a link back to the same screen.
	Self bool
	// Missing marks a language the page has not been translated into yet. Such
	// a row carries a button instead of a link.
	Missing bool
	// Code is the language tag, needed by the "translate" button.
	Code string
}

// Draft reports whether this translation is still unpublished, so the panel can
// say so — a group where only the German is live is the normal state halfway
// through translating, and it should be visible at a glance.
func (t TranslationView) Draft() bool { return t.Status == "draft" }

// languageChoices builds the dropdown for one website.
//
// Nil when the website has one language: a select with a single option is a
// control that asks a question with only one answer.
func languageChoices(ws *domain.Website, current string) []LanguageChoice {
	if ws == nil || !ws.Multilingual() {
		return nil
	}
	out := []LanguageChoice{{
		Code: "", Name: locale.Name(ws.Locale), Selected: current == "", Primary: true,
	}}
	for _, tag := range ws.Locales() {
		out = append(out, LanguageChoice{Code: tag, Name: locale.Name(tag), Selected: current == tag})
	}
	return out
}

// translationViews lists the whole group a page belongs to, missing languages
// included.
//
// The gaps are the point: what an editor needs to see is not which translations
// exist but which ones are still missing.
func (h *Handler) translationViews(r *http.Request, ws *domain.Website, pg *page.Page) []TranslationView {
	if ws == nil || pg == nil || !ws.Multilingual() {
		return nil
	}
	group, err := h.pages.TranslationsForEditor(r.Context(), pg)
	if err != nil {
		slog.Error("load translation group", "err", err, "page", pg.ID)
		return nil
	}

	byLocale := map[string]page.Page{}
	for _, t := range group {
		byLocale[t.Locale] = t
	}

	var out []TranslationView
	for _, tag := range append([]string{""}, ws.Locales()...) {
		name := locale.Name(ws.Locale)
		if tag != "" {
			name = locale.Name(tag)
		}
		t, exists := byLocale[tag]
		if !exists {
			out = append(out, TranslationView{Name: name, Code: tag, Missing: true})
			continue
		}
		out = append(out, TranslationView{
			PageID: t.ID, Name: name, Title: t.Title, Status: t.Status,
			Self: t.ID == pg.ID, Code: tag,
		})
	}
	return out
}

// setLanguage files a page under its language after it has been saved.
//
// Separate from the save itself: it must not need the version token, and a page
// whose language is wrong is a page that still saved. A language the website
// does not have becomes the main one rather than an error — the form cannot
// offer it, so the only way to send it is by hand.
func (h *Handler) setLanguage(r *http.Request, ws *domain.Website, pageID int64, values PageValues) {
	if ws == nil || !ws.Multilingual() {
		return
	}
	loc := locale.Pick(values.Locale, ws.Locales())
	of := values.translationOf()
	// A page in the main language is the middle of its own star and never
	// points anywhere: allowing it to would let two pages point at each other
	// and make the group unreachable from either end.
	if loc == "" {
		of = 0
	}
	if of == pageID {
		of = 0
	}
	if err := h.pages.SetTranslation(r.Context(), pageID, loc, of); err != nil {
		slog.Error("set page language", "err", err, "page", pageID)
	}
}

// HandlePageTranslate starts the translation of a page into one language.
//
// It copies the original rather than opening an empty form: a translator works
// from the text, and retyping the structure of a page in order to translate it
// is the part nobody does twice.
//
// The copy is a draft. Everything about it is still the original's words, and a
// half-translated page must not be reachable from the internet.
func (h *Handler) HandlePageTranslate(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil || !ws.Multilingual() {
		http.NotFound(w, r)
		return nil
	}
	original, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if original == nil || original.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}
	// Out of the address, not out of the form. The button sits inside the page
	// form, which carries a "sprache" field of its own for the page's own
	// language — and on a POST the body wins over the query, so FormValue would
	// hand back the language of the page being translated instead of the one
	// the button asked for.
	tag := locale.Pick(r.URL.Query().Get("sprache"), ws.Locales())
	if tag == "" {
		web.SetFlashError(h.sm, r.Context(), "Diese Sprache gibt es auf dieser Website nicht")
		return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, pageID))
	}

	// The middle of the star: translating a translation still belongs to the
	// same group, not to a chain hanging off it.
	mitte := original.ID
	if original.TranslationOf != 0 {
		mitte = original.TranslationOf
	}

	created, err := h.pages.CreatePage(r.Context(), page.PageCreate{
		WebsiteID: websiteID,
		Title:     original.Title,
		// Addresses are unique per website across all languages, so the copy
		// cannot keep the original's. The tag is appended as a placeholder the
		// translator replaces with the real one — "kontakt-fr" becomes
		// "contact" as soon as somebody looks at the page.
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
		UserID:  h.currentUserID(r),
	})
	if err != nil {
		return err
	}
	if err := h.pages.SetTranslation(r.Context(), created.ID, tag, mitte); err != nil {
		return err
	}

	// Zusammengesetzt, also vorher übersetzt: SetFlashSuccess schlägt den ganzen
	// Satz nach, und einen in Go zusammengeklebten findet es nie.
	web.SetFlashSuccess(h.sm, r.Context(), web.Titlef(r,
		"%s: Entwurf aus der Vorlage angelegt – Text und Adresse jetzt übersetzen",
		web.T(r, locale.Name(tag))))
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, created.ID))
}

// translationSlug is the placeholder address of a fresh translation.
func translationSlug(slug, tag string) string {
	suffix := "-" + page.Slugify(tag)
	if len(slug)+len(suffix) > page.MaxSlugLength {
		slug = slug[:page.MaxSlugLength-len(suffix)]
	}
	return slug + suffix
}

// rowLanguage is the language shown in a listing row, empty when the website
// has one language and the column would say the same thing on every row.
func rowLanguage(ws *domain.Website, stored string) string {
	if ws == nil || !ws.Multilingual() {
		return ""
	}
	if stored == "" {
		return locale.Name(ws.Locale)
	}
	return locale.Name(stored)
}
