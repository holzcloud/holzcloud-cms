package admin

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/web"
	"github.com/holzcloud/holzcloud-cms/internal/wording"
)

// The screen where an operator says what their site calls things.
//
// The theme decides what words exist — "Cart", "Search", "Page not found" — and
// ships a translation for each. This screen is where the farm shop says "Korb"
// instead of "Warenkorb" without forking the theme, and the only place the two
// can be told apart: what the theme offers stands beside what the operator made
// of it, in every language the website is published in.

// WordRow is one word on the screen.
type WordRow struct {
	// Key is the theme author's own sentence, which is also what stands in the
	// template. It is shown, because it is the only name the word has.
	Key string
	// Theme is what the theme's own catalogue says in this language, or empty
	// when the theme does not translate it and the key itself is shown.
	Theme string
	// Own is the operator's word, empty when they have not chosen one.
	Own string
	// Untranslated marks a word the theme mints and does not translate in this
	// language. It is the thing worth acting on, so the screen can lead with it.
	Untranslated bool
}

// WordLanguage is one language's worth of rows.
type WordLanguage struct {
	Locale string
	// Name is the language as it calls itself, for somebody who does not read
	// tags.
	Name string
	Rows []WordRow
	// Changed and Untranslated count the rows worth noticing.
	Changed      int
	Untranslated int
}

// WordingData is the screen.
type WordingData struct {
	web.LayoutData
	Website   *domain.Website
	Languages []WordLanguage
	// Stray are overrides for words the current theme does not ask for. They
	// are kept, not deleted: an operator who tries another theme and comes back
	// must find their words again. Saying so is better than either tidying them
	// away or pretending they do the nothing they currently do.
	Stray []wording.Entry
	Limit int
	Used  int
}

// HandleWebsiteWording renders the wording screen.
func (h *Handler) HandleWebsiteWording(w http.ResponseWriter, r *http.Request) error {
	ws, err := h.wordingWebsite(r)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	keys, themeWords := h.themeVocabulary(r, ws)
	own, err := h.wording.All(r.Context(), ws.ID)
	if err != nil {
		return err
	}
	ownBy := map[string]map[string]string{}
	for _, e := range own {
		if ownBy[e.Locale] == nil {
			ownBy[e.Locale] = map[string]string{}
		}
		ownBy[e.Locale][e.Key] = e.Value
	}

	known := map[string]bool{}
	for _, k := range keys {
		known[k] = true
	}

	data := WordingData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Wording – %s", ws.Name)),
		Website:    ws,
		Limit:      wording.MaxKeys,
		Used:       len(own),
	}

	for _, loc := range websiteLocales(ws) {
		lang := WordLanguage{Locale: loc, Name: locale.Native(loc)}
		for _, key := range keys {
			theme, translated := themeWords[loc][key]
			row := WordRow{Key: key, Theme: theme, Own: ownBy[loc][key]}
			// The theme translates a key when its catalogue CARRIES it — not
			// when the translation differs from the key. Plenty of words are
			// the same in two languages ("Menu", "Page %d"), and calling those
			// untranslated would nag an operator about work that is done.
			row.Untranslated = !translated || theme == ""
			if row.Own != "" {
				lang.Changed++
			}
			if row.Untranslated {
				lang.Untranslated++
			}
			lang.Rows = append(lang.Rows, row)
		}
		data.Languages = append(data.Languages, lang)
	}

	for _, e := range own {
		if !known[e.Key] {
			data.Stray = append(data.Stray, e)
		}
	}

	data.ActiveNav = "website-design"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "website_wording", data)
}

// HandleWebsiteWordingSave takes the screen's form.
//
// Every box is written, including the empty ones: an emptied box is how a
// person says "use the theme's word again", and the store turns that into a
// removal. Doing it the other way round — writing only what is filled in —
// would make a word impossible to give back.
func (h *Handler) HandleWebsiteWordingSave(w http.ResponseWriter, r *http.Request) error {
	ws, err := h.wordingWebsite(r)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		web.SetFlashError(h.sm, r.Context(), "The form could not be read.")
		return h.redirectToWording(w, r, ws.ID)
	}

	keys, _ := h.themeVocabulary(r, ws)
	known := map[string]bool{}
	for _, k := range keys {
		known[k] = true
	}
	allowed := map[string]bool{}
	for _, l := range websiteLocales(ws) {
		allowed[l] = true
	}

	full := false
	saved := 0
	for name, values := range r.Form {
		locale, key, ok := splitWordField(name)
		// A field for a language the website does not publish in, or for a word
		// the theme does not ask for, is not saved. Both mean the form was
		// built against a different state than the one being saved into —
		// another tab, another theme — and guessing which would write words
		// nobody chose.
		if !ok || !allowed[locale] || !known[key] {
			continue
		}
		err := h.wording.Set(r.Context(), ws.ID, locale, key, values[0])
		switch {
		case err == wording.ErrTooMany:
			full = true
		case err != nil:
			return err
		default:
			saved++
		}
	}

	// The render caches a parsed set per website and language with the words
	// baked into its FuncMap. Without this the operator saves, reloads the
	// site, and sees the old word.
	h.loader.InvalidateTemplateCache(ws.ID)

	if full {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r,
			"Not everything was saved: a website may have at most %d words of its own.",
			wording.MaxKeys))
	} else {
		web.SetFlashSuccess(h.sm, r.Context(), "Wording saved.")
	}
	_ = saved
	return h.redirectToWording(w, r, ws.ID)
}

func (h *Handler) redirectToWording(w http.ResponseWriter, r *http.Request, id int64) error {
	http.Redirect(w, r, "/admin/websites/"+strconv.FormatInt(id, 10)+"/wording",
		http.StatusSeeOther)
	return nil
}

// wordingWebsite reads the website out of the path, or nil when there is none.
func (h *Handler) wordingWebsite(r *http.Request) (*domain.Website, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, nil
	}
	return h.domains.GetWebsite(r.Context(), id)
}

// splitWordField reads "w:<locale>:<key>" back apart.
//
// The locale cannot contain a colon and the key can, so the split is on the
// first two and the rest is the key — a sentence with a colon in it is an
// ordinary sentence and must survive the round trip.
func splitWordField(name string) (locale, key string, ok bool) {
	rest, found := strings.CutPrefix(name, "w:")
	if !found {
		return "", "", false
	}
	locale, key, found = strings.Cut(rest, ":")
	if !found || locale == "" || key == "" {
		return "", "", false
	}
	return locale, key, true
}

// themeVocabulary is the words the website's theme mints, and what its own
// catalogues make of them in each language the site is published in.
func (h *Handler) themeVocabulary(r *http.Request, ws *domain.Website) ([]string, map[string]map[string]string) {
	themeFS := h.loader.ActiveThemeFS(r.Context(), ws.ID)
	if themeFS == nil {
		return nil, nil
	}
	keys := tmpl.KeysUsed(themeFS)
	sort.Strings(keys)

	words := map[string]map[string]string{}
	for _, loc := range websiteLocales(ws) {
		words[loc] = tmpl.CatalogFor(themeFS, loc)
	}
	return keys, words
}

// websiteLocales is every language the website is published in, its main one
// first.
//
// Website.Locales() is the EXTRA languages — that is what Multilingual() counts
// — so a screen that used it alone would leave out the language most of the
// site is written in. This one was found in the browser: the wording screen
// offered French and not German on a German website with French beside it.
func websiteLocales(ws *domain.Website) []string { return ws.AllLocales() }
