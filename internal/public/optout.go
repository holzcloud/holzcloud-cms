package public

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/marker"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The opt-out of a website that follows US law (LEG-03).
//
// A page carries [[widerspruch]], and at delivery the marker becomes a widget:
// one line saying whether this visitor's browser sends the Global Privacy
// Control signal, and a form that takes a request not to sell or share
// personal information without an account and without a cookie (D-10, D-11).
// The "Do Not Sell or Share" page that plan 07 composes carries the marker;
// any other page may carry it too.
//
// It is core and not part of the contact-form plugin (D-16): a plugin can be
// switched off, and a website that says it honours opt-outs must keep taking
// them.

// optoutMarker is what a page writes where the widget goes.
const optoutMarker = "[[widerspruch]]"

// optoutWidgetMark is the attribute the widget's outer element carries. The
// cache headers look for it in a rendered answer (optoutVary), so it has to be
// something no editor's content produces by accident.
const optoutWidgetMark = "data-hc-optout"

// optoutMaxAge is how long a drawn form's time stamp stays valid. It mirrors
// the contact-form plugin's maxAge: a page can stand in a tab for a long time,
// and the point is only that a stamp copied once is not reusable for weeks.
const optoutMaxAge = 12 * time.Hour

var (
	// optoutParagraph is the marker on a line of its own, which goldmark wraps
	// in a paragraph. The paragraph goes with it: a form inside a <p> is markup
	// the browser repairs by closing the paragraph early and leaving an empty
	// one behind.
	optoutParagraph = regexp.MustCompile(`<p>\s*\[\[widerspruch\]\]\s*</p>`)
	// optoutPattern is the marker anywhere else.
	optoutPattern = regexp.MustCompile(`\[\[widerspruch\]\]`)
)

// optoutOffered reports whether this website takes opt-outs here: its set has
// US, and the handler has somewhere to store them and a way to sign the form.
func (h *Handler) optoutOffered(ws *domain.Website) bool {
	return ws != nil && ws.Frameworks().Has(legal.US) && h.requests != nil && h.optoutSigner != nil
}

// expandOptOut puts the opt-out widget where a page says [[widerspruch]].
//
// At delivery and never at save, because the widget's first line depends on
// the request: whether this browser sent Sec-GPC: 1 (CCPA regs § 7025(c)(6)).
// A text frozen into content_html could only ever say one of the two.
//
// After the values and before the plugins, the same slot as the value pass
// and for the same reasons: a plugin filter sees the page as a visitor would,
// form included, and the responsive rewrite at the end sees the final markup.
//
// marker.OnlyInText first, because the expansion is HTML. An editor can type
// the marker into a link address, and expanding it there would splice a form
// into an attribute (UF-1, internal/marker's package doc). A marker that is
// not in text is removed, which is what it rendered as before.
//
// A website without US in its set gets nothing where the marker stood — not
// the marker, not a form. A page that loses US keeps its text without a form
// that would promise something the website no longer offers. The same holds
// when the handler was built without a request store or a signer.
//
// One widget per page: a second marker renders as nothing, because two forms
// with the same field ids are one form too many.
func (h *Handler) expandOptOut(r *http.Request, ws *domain.Website, pg *page.Page, body string) string {
	if !strings.Contains(body, optoutMarker) {
		return body
	}
	body = marker.OnlyInText(body, optoutParagraph, optoutPattern)
	widget := ""
	if h.optoutOffered(ws) {
		widget = h.optoutWidget(r, ws, pg)
	}
	placed := false
	place := func(string) string {
		if placed {
			return ""
		}
		placed = true
		return widget
	}
	body = optoutParagraph.ReplaceAllStringFunc(body, place)
	return optoutPattern.ReplaceAllStringFunc(body, place)
}

// withoutOptOut removes the marker from a body that is not a page being
// served — a feed entry, what a plugin reads. Neither has a request to answer
// and neither should carry a form, but a visitor must never see the marker.
func withoutOptOut(body string) string {
	if !strings.Contains(body, optoutMarker) {
		return body
	}
	body = optoutParagraph.ReplaceAllString(body, "")
	return optoutPattern.ReplaceAllString(body, "")
}

// optoutLang is the language the widget speaks: the page's, except Spanish.
//
// A Spanish website gets the English legal templates (D-16), so the widget on
// its "Do Not Sell or Share" page speaks English too, and does not switch
// language halfway down the page. Romansh has no catalogue and falls back to
// the English source by itself.
func optoutLang(r *http.Request) string {
	lang := i18n.Lang(r.Context())
	if base, _, _ := strings.Cut(lang, "-"); base == "es" {
		return i18n.Source
	}
	return lang
}

// optoutWidget draws the widget. Every word goes through the catalogue with
// its key at the call site, so tools/i18n collects it; every word is escaped,
// because a translation is text and not markup.
//
// The classes are the contact-form plugin's, which all eight shipped themes
// style: the widget looks like the form a visitor already knows from the
// contact page. No hx- attribute anywhere — the public site loads no script,
// and the form is an ordinary POST with an ordinary 303 back.
func (h *Handler) optoutWidget(r *http.Request, ws *domain.Website, pg *page.Page) string {
	lang := optoutLang(r)
	e := html.EscapeString
	now := time.Now()

	var b strings.Builder
	fmt.Fprintf(&b, `<form class="contact-form" %s method="post" action="%s">`,
		optoutWidgetMark, e(h.localePath(r, ws, "/widerspruch")))

	// The status line, and the only thing the signal changes (D-12).
	status := i18n.T(lang, "Your browser does not send the Global Privacy Control signal. You can switch it on in your browser, or use this form.")
	if legal.GPC(r.Header) {
		status = i18n.T(lang, "Your browser sends the Global Privacy Control signal. Opt-Out Request Honored: this website treats it as your request not to sell or share your personal information — and it sells and shares none.")
	}
	fmt.Fprintf(&b, `<p class="contact-form__notice" role="status">%s</p>`, e(status))

	// What the last submission came to. The query names the outcome and not
	// the sentence, so nobody can put words on this page through a link.
	result, isError := "", false
	switch r.URL.Query().Get("widerspruch") {
	case "eingegangen":
		result = i18n.T(lang, "Your request has been recorded.")
	case "leer":
		result, isError = i18n.T(lang, "Please give a name or an e-mail address so that the request can be recorded."), true
	case "zulang":
		result, isError = i18n.T(lang, "An entry is longer than the form allows."), true
	}
	if result != "" {
		class := "contact-form__notice"
		if isError {
			class += " contact-form__notice--error"
		}
		fmt.Fprintf(&b, `<p class="%s" role="status">%s</p>`, class, e(result))
	}

	slug := ""
	if pg != nil {
		slug = pg.Slug
	}
	fmt.Fprintf(&b, `<input type="hidden" name="seite" value="%s">`, e(slug))
	// The issue time in the id slot of a share-link token: signed, so a robot
	// cannot back-date it, and checked for its age when the form comes back.
	fmt.Fprintf(&b, `<input type="hidden" name="stempel" value="%s">`,
		e(h.optoutSigner.Token(now.Unix(), now.Add(optoutMaxAge))))

	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="hc-optout-name">%s</label>`+
		`<input type="text" id="hc-optout-name" name="name" maxlength="%d" autocomplete="name"></div>`,
		e(i18n.T(lang, "Name (optional)")), dsr.MaxName)
	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="hc-optout-email">%s</label>`+
		`<input type="email" id="hc-optout-email" name="email" maxlength="%d" autocomplete="email"></div>`,
		e(i18n.T(lang, "E-mail address (optional)")), dsr.MaxEmail)
	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="hc-optout-note">%s</label>`+
		`<textarea id="hc-optout-note" name="note" rows="4" maxlength="%d"></textarea></div>`,
		e(i18n.T(lang, "Note (optional)")), dsr.MaxNote)

	// The honeypot, as the contact form has it: hidden from people by the
	// stylesheet and from screen readers by aria-hidden and tabindex. A program
	// that fills in every input fills in this one too.
	fmt.Fprintf(&b, `<div class="contact-form__trap" aria-hidden="true">`+
		`<label for="hc-optout-website">%s</label>`+
		`<input type="text" id="hc-optout-website" name="website" tabindex="-1" autocomplete="off"></div>`,
		e(i18n.T(lang, "Website (please leave empty)")))

	// The small print under the fields. .contact-form__alternative and not
	// .contact-form__why: the themes paint __why in the error colour, because
	// the contact form uses it for the sentence under a field that was wrong.
	fmt.Fprintf(&b, `<p class="contact-form__alternative">%s</p>`,
		e(i18n.T(lang, "What you enter here is used only to record and answer this request. No account is needed.")))
	fmt.Fprintf(&b, `<button type="submit" class="contact-form__submit">%s</button>`,
		e(i18n.T(lang, "Send opt-out request")))
	b.WriteString(`</form>`)
	return b.String()
}

// HandleOptOut takes the opt-out form (POST /widerspruch).
//
// No session, no cookie, no CSRF token — gorilla/csrf needs a cookie, and the
// public side sets none before a visitor asks for one (D-03). The answer is
// always a 303 back to the page the form stood on, with the outcome in the
// query, so a reload never posts twice.
func (h *Handler) HandleOptOut(w http.ResponseWriter, r *http.Request) error {
	ws := domain.WebsiteFromContext(r.Context())
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}
	if !h.optoutOffered(ws) {
		return h.serve404(w, r, ws)
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil
	}
	ctx := r.Context()
	back, err := h.optoutBack(r, ws, r.PostFormValue("seite"))
	if err != nil {
		return err
	}

	lang := LocaleFrom(ctx)
	if lang == "" {
		lang = ws.Locale
	}
	_, err = h.requests.Add(ctx, dsr.Request{
		WebsiteID: ws.ID,
		Kind:      dsr.KindOptOut,
		Locale:    lang,
		Name:      r.PostFormValue("name"),
		Email:     r.PostFormValue("email"),
		Note:      r.PostFormValue("note"),
		GPC:       legal.GPC(r.Header),
	})
	switch {
	case errors.Is(err, dsr.ErrEmpty):
		optoutRedirect(w, r, back, "leer")
	case errors.Is(err, dsr.ErrTooLong):
		optoutRedirect(w, r, back, "zulang")
	case err != nil:
		return fmt.Errorf("store opt-out: %w", err)
	default:
		optoutRedirect(w, r, back, "eingegangen")
	}
	return nil
}

// optoutBack is the address the form came from: the published page in the
// language being served, or the start page of that language when the form
// named nothing that exists.
func (h *Handler) optoutBack(r *http.Request, ws *domain.Website, seite string) (string, error) {
	slug := strings.Trim(strings.TrimSpace(seite), "/")
	if slug == "" {
		return h.localePath(r, ws, "/"), nil
	}
	pg, err := h.pageStore.GetPublishedPageIn(r.Context(), ws.ID, LocaleFrom(r.Context()), slug)
	if err != nil {
		return "", fmt.Errorf("get page for opt-out: %w", err)
	}
	if pg == nil || pg.Slug == page.HomeSlug {
		return h.localePath(r, ws, "/"), nil
	}
	return h.localePath(r, ws, "/"+pg.Slug), nil
}

func optoutRedirect(w http.ResponseWriter, r *http.Request, back, outcome string) {
	http.Redirect(w, r, back+"?widerspruch="+outcome, http.StatusSeeOther)
}
