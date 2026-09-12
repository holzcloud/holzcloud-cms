package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// partialFiles are shared fragment templates. They are parsed into every page
// set (so a full page can embed them) and can also be rendered on their own as
// an htmx swap response via RenderPartial.
//
// Every fragment returned to the browser must live here rather than being built
// with string concatenation in Go: hand-built HTML bypasses html/template's
// contextual escaping and turns any user-controlled field into stored XSS.
var partialFiles = []string{
	"page_row.html",
	"domain_list.html",
	"page_inline_edit.html",
	"pager.html",
	"preview_pane.html",
	"markdown_help.html",
	"media_picker.html",
	"block_list.html",
	// The icons. Here, because a fragment needs them just as much as a whole
	// page: a row that htmx reloads should carry the same menu as before.
	"icons.html",
	// One input field of the website — once in the field list, once per row
	// einer Gruppe.
	"field_input.html",
	// One field of the form together with everything hanging off it. Calls itself.
	"field_top.html",
	// The reason for a row as a sentence, and the outcome as a badge. A partial,
	// because the dry run and the report are two separately parsed templates:
	// the same reason has to read the same on both screens, and two copies of it
	// would drift apart.
	"csv_reason.html",
}

// standalonePages are full HTML documents with no base layout: they are what a
// visitor sees before there is a session to hang a layout on.
var standalonePages = []string{"login", "setup", "set_password", "two_factor_verify", "order_print"}

// layoutPageNames are the pages that supply a "content" block to base.html.
var layoutPageNames = []string{"dashboard", "website_list", "website_form", "website_design", "page_list", "page_form", "page_revisions", "page_revision_compare", "trash", "snippet_list", "term_list", "redirect_list", "user_link", "template_list", "template_upload", "plugin_list", "plugin_screen", "mail_status", "ai_keys", "field_list", "blocktype_list", "menu_list", "menu_edit", "album_list", "album_edit", "media_list", "media_crop", "user_list", "user_form", "user_password", "two_factor_setup", "two_factor_codes", "account", "share_link", "import_report", "language_list", "translation_matrix", "confirm", "branding", "kind_list", "activity_log", "product_list", "product_form", "shop_settings", "order_list", "order_detail", "csv_mapping", "csv_expired", "csv_dryrun", "csv_report"}

// PageTemplates holds per-page template sets, each with its own "content"
// block — one complete set per language.
//
// Parsed per language rather than translated while rendering, because the
// translation function is baked into the FuncMap at parse time. The alternative
// — cloning the set on every request to bind a different function — would pay
// for the whole template tree on every page view to save a few hundred
// kilobytes once at startup.
type PageTemplates struct {
	// mu guards byLang, which Reload replaces wholesale while requests are
	// being served.
	mu sync.RWMutex
	// byLang holds one parsed world per language, keyed by tag.
	byLang map[string]*langSet
	// layoutPages tracks which pages use the base layout (render via "base" entry point)
	layoutPages map[string]bool
	// fsys is kept so a language dropped into the folder on disk can be parsed
	// without restarting the process.
	fsys fs.FS
}

type langSet struct {
	pages map[string]*template.Template
	// partials is the standalone set used for fragment-only responses
	partials *template.Template
}

// adminFuncs are the helpers every admin template gets, bound to one language.
func adminFuncs(lang string) template.FuncMap {
	return template.FuncMap{
		// t translates one German string. It is the only thing a template
		// author has to remember: wrap the words a person reads.
		"t": func(s string) string { return i18n.T(lang, s) },
		// th is t for a sentence that carries its own inline markup — a <code>
		// or a <strong> in the middle of it. The alternative is chopping the
		// sentence into three fragments a translator cannot make sense of.
		//
		// Safe because of what it is fed: every argument is a literal from a
		// template compiled into this binary, and the catalogues are compiled
		// in beside them. It never sees user data — a sentence with a value in
		// it contains a template action, and an action cannot be a string
		// literal here.
		"th": func(s string) template.HTML { return template.HTML(i18n.T(lang, s)) },
		// tf is t for a sentence with something in it: {{tf "%d Seiten" .Count}}.
		// The format string is what gets translated, so a language that needs
		// the parts in another order can say so.
		"tf": func(format string, args ...any) string { return i18n.Tf(lang, format, args...) },
		// thf is th and tf at once: a sentence that carries BOTH inline markup
		// and a value.
		//
		// It exists because two sentences in this admin were chopped in three
		// for want of it, and the pieces reached the catalogue as keys. One of
		// them was the fragment "an." — a full stop, whose English translation
		// is a full stop, which is the point at which a catalogue entry has
		// stopped carrying meaning. The other was "fest.". Both are now one
		// sentence, and a translator can see what they are translating.
		//
		// Every ARGUMENT is escaped, and that is the difference from th.
		// th is safe because it only ever sees literals compiled into this
		// binary; thf sees whatever the caller substitutes, which on the screen
		// that needed it is an e-mail address somebody typed. The markup of the
		// FORMAT is trusted for the same reason th's is; the values filled into
		// it never are.
		"thf": func(format string, args ...any) template.HTML {
			safe := make([]any, len(args))
			for i, a := range args {
				safe[i] = template.HTMLEscapeString(fmt.Sprint(a))
			}
			return template.HTML(i18n.Tf(lang, format, safe...))
		},
		// lang is the tag itself, for <html lang="…">.
		"lang": func() string { return lang },
	}
}

// ParseAdminTemplates builds per-page template sets from an fs.FS, once for
// every language this build can show.
func ParseAdminTemplates(fsys fs.FS) (*PageTemplates, error) {
	pt := &PageTemplates{
		byLang:      make(map[string]*langSet),
		layoutPages: make(map[string]bool),
		fsys:        fsys,
	}
	for _, name := range layoutPageNames {
		pt.layoutPages[name] = true
	}
	if err := pt.Reload(); err != nil {
		return nil, err
	}
	return pt, nil
}

// Reload parses the whole admin template world again, once per language this
// installation now has.
//
// It exists because a language can arrive after start-up: an operator drops a
// file into <data>/sprachen and presses a button. The translation function is
// baked into the FuncMap at parse time, so a new language means a new parsed
// set — there is no way around re-parsing, and re-parsing is a few hundred
// milliseconds once, not per request.
//
// A failure leaves the previous sets in place: half a world would be worse
// than an old one.
func (pt *PageTemplates) Reload() error {
	built := make(map[string]*langSet)
	for _, l := range i18n.Languages() {
		set, err := parseFor(pt.fsys, l.Code)
		if err != nil {
			return fmt.Errorf("%s: %w", l.Code, err)
		}
		built[l.Code] = set
	}
	if built[i18n.Source] == nil {
		return fmt.Errorf("no templates for the source language")
	}

	pt.mu.Lock()
	pt.byLang = built
	pt.mu.Unlock()
	return nil
}

// parseFor parses the whole admin template world for one language.
func parseFor(fsys fs.FS, lang string) (*langSet, error) {
	set := &langSet{pages: make(map[string]*template.Template)}
	funcs := adminFuncs(lang)

	partials, err := template.New("partials").Funcs(funcs).ParseFS(fsys, partialFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse partials: %w", err)
	}
	set.partials = partials

	for _, name := range standalonePages {
		t, err := template.New(name).Funcs(funcs).ParseFS(fsys, name+".html")
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		set.pages[name] = t
	}

	for _, name := range layoutPageNames {
		files := append([]string{"base.html", name + ".html"}, partialFiles...)
		t, err := template.New(name).Funcs(funcs).ParseFS(fsys, files...)
		if err != nil {
			return nil, fmt.Errorf("parse %s with base: %w", name, err)
		}
		set.pages[name] = t
	}

	// Every template has to stand in exactly one of the two lists. If it is
	// missing from both, it is skipped without complaint at start-up and fails
	// only when it is called, with "template not found" — on a page that perhaps
	// nobody opens before the customers do.
	if err := checkAllPagesRegistered(fsys, set); err != nil {
		return nil, err
	}

	return set, nil
}

// forLang is the parsed world for a request's language, falling back to German.
//
// The fallback is not defensive dressing: a language could be removed from a
// build while a user still has it stored in their account, and the answer to
// that is a German screen, not a 500.
func (pt *PageTemplates) forLang(lang string) *langSet {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	if set, ok := pt.byLang[lang]; ok {
		return set
	}
	return pt.byLang[i18n.Source]
}

// checkAllPagesRegistered fails the start when a template file was added and
// nobody put it in one of the two lists above.
func checkAllPagesRegistered(fsys fs.FS, set *langSet) error {
	entries, err := fs.Glob(fsys, "*.html")
	if err != nil {
		return fmt.Errorf("list admin templates: %w", err)
	}

	known := map[string]bool{"base.html": true}
	for _, f := range partialFiles {
		known[f] = true
	}

	var missing []string
	for _, entry := range entries {
		if known[entry] {
			continue
		}
		name := strings.TrimSuffix(entry, ".html")
		if set.pages[name] == nil {
			missing = append(missing, entry)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("these admin templates stand in no list in render.go and would not be findable at run time: %s",
			strings.Join(missing, ", "))
	}
	return nil
}

// RenderPartial renders a shared fragment template (see partialFiles) as a
// standalone htmx response.
func RenderPartial(w http.ResponseWriter, pt *PageTemplates, r *http.Request, name string, data any) error {
	return RenderPartialFor(w, pt, i18n.Lang(r.Context()), name, data)
}

// RenderPartialFor is RenderPartial in one language. Fragments are answers to
// the same person as the page they land in, so they have to speak the same
// language — a row swapped by htmx that comes back in German is the most
// visible way a translation can fail.
func RenderPartialFor(w http.ResponseWriter, pt *PageTemplates, lang, name string, data any) error {
	w.Header().Set("Vary", "HX-Request")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return pt.forLang(lang).partials.ExecuteTemplate(w, name, data)
}

// RenderAdmin renders an admin page. For htmx partial requests, renders
// the "-content" variant. For full page requests on layout pages, renders
// "base". For standalone pages, renders the page name directly.
// Always sets Vary: HX-Request.
func RenderAdmin(w http.ResponseWriter, pt *PageTemplates, r *http.Request, name string, data any) error {
	return RenderAdminStatus(w, pt, r, name, data, http.StatusOK)
}

// RenderAdminStatus is RenderAdmin with an explicit status code. The status has
// to be written after the headers and before the body, so a caller that needs a
// non-200 cannot simply call WriteHeader around RenderAdmin.
func RenderAdminStatus(w http.ResponseWriter, pt *PageTemplates, r *http.Request, name string, data any, status int) error {
	w.Header().Set("Vary", "HX-Request")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}

	tmpl := pt.forLang(i18n.Lang(r.Context())).pages[name]
	if tmpl == nil {
		return fmt.Errorf("template %q not found", name)
	}

	if r.Header.Get("HX-Request") == "true" {
		return tmpl.ExecuteTemplate(w, name+"-content", data)
	}

	// Layout pages execute "base"; standalone pages execute their own name
	execName := name
	if pt.layoutPages[name] {
		execName = "base"
	}
	return tmpl.ExecuteTemplate(w, execName, data)
}
