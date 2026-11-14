package public

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/marker"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
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

// The anti-spam numbers mirror the contact-form plugin's, and for its
// reasons (plugins/kontaktformular): the honeypot and the time trap stop the
// ordinary robot, the hourly limit stops the one that gets through anyway from
// filling a small server's disk and the operator's inbox overnight. None of
// them needs a cookie, which is the point — the public side sets none before
// a visitor asks for one (D-03).
const (
	// optoutMinAge is what a human being needs at least to press the button.
	// Far above what a script needs, and short, because a browser that fills
	// in name and address lets a person send within a second or two; one who
	// is quicker still is asked to send again, not turned away.
	optoutMinAge = 2 * time.Second
	// optoutMaxAge is how long a drawn form's time stamp stays valid: long
	// enough to read the page and fill in the form, short enough that one
	// stamp fetched once does not feed a robot for a day. A person whose tab
	// stood open longer is asked to send again (stampStale), so the price of
	// a short lifetime is one more click and never a lost request.
	optoutMaxAge = 2 * time.Hour
	// optoutSiteLimit is how many opt-outs a website stores per hour,
	// counted inside the insert (dsr.Store.AddWithin) so that a restart does
	// not reset it and a burst does not slip past it. It protects the disk —
	// three hundred requests of at most 2500 characters are under a megabyte
	// an hour — and it is high on purpose, because reaching it closes the
	// form for everybody.
	optoutSiteLimit = 300
	// optoutNetworkLimit is how many of them one network may send per hour
	// (optout_limit.go). A person sends one; an office behind one address a
	// few. Reaching it closes the form for that network only.
	optoutNetworkLimit = 10
	// optoutMailLimit is how many notifications a website's operator gets
	// per hour. A request above it is stored and listed, not mailed: the
	// record matters more than the inbox, and a flood should fill neither.
	optoutMailLimit = 30
	// optoutMaxBody bounds what the form may send. The three fields together
	// are under 2500 characters; sixteen KiB leaves room for any encoding of
	// them and none for anything else.
	optoutMaxBody = 16 << 10
)

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
	case "spaeter":
		result, isError = i18n.T(lang, "Too many requests at the moment. Please try again in an hour."), true
	case "abgelaufen":
		result, isError = i18n.T(lang, "The form was open too long or sent too quickly, so the request has not been recorded. Please send it again."), true
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
	fmt.Fprintf(&b, `<input type="hidden" name="stempel" value="%s">`,
		e(OptOutStamp(h.optoutSigner, ws.ID, now)))

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
//
// The order of the checks is the order of their cost. A cross-site POST is
// refused before anything is read: with no cookie there is nothing for a
// classic CSRF to ride, but a forged opt-out would still fill the operator's
// list and inbox. The body is bounded before it is parsed. The honeypot and
// a stamp this page did not draw answer exactly like a success and store
// nothing, because a robot must not learn which trap caught it. A stamp this
// page did draw, but too long ago or a moment ago, belongs to a person, who
// is asked to send again rather than told "recorded" about a request that
// was not (CCPA regs § 7026(a)(1) makes this form a way the request must be
// taken). The hourly limit is the other refusal a visitor sees, because a
// person can act on it.
func (h *Handler) HandleOptOut(w http.ResponseWriter, r *http.Request) error {
	ws := domain.WebsiteFromContext(r.Context())
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}
	if !h.optoutOffered(ws) {
		return h.serve404(w, r, ws)
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, optoutMaxBody)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return nil
		}
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil
	}
	ctx := r.Context()
	now := time.Now()
	back, err := h.optoutBack(r, ws, r.PostFormValue("seite"))
	if err != nil {
		return err
	}

	// The honeypot before the stamp's age: a robot that kept a real stamp
	// must still learn nothing from the answer.
	verdict := h.optoutStampCheck(ws, r.PostFormValue("stempel"), now)
	if r.PostFormValue("website") != "" || verdict == stampForged {
		optoutRedirect(w, r, back, "eingegangen")
		return nil
	}
	if verdict == stampStale {
		optoutRedirect(w, r, back, "abgelaufen")
		return nil
	}
	// The network's budget before the website's ceiling, so that one
	// network that spent its own never reaches the one everybody shares.
	if network, ok := h.optoutNetwork(r); ok && h.optoutNets != nil && !h.optoutNets.take(network, now) {
		optoutRedirect(w, r, back, "spaeter")
		return nil
	}
	lang := LocaleFrom(ctx)
	if lang == "" {
		lang = ws.Locale
	}
	req := dsr.Request{
		WebsiteID: ws.ID,
		Kind:      dsr.KindOptOut,
		Locale:    lang,
		Name:      strings.TrimSpace(r.PostFormValue("name")),
		Email:     strings.TrimSpace(r.PostFormValue("email")),
		Note:      strings.TrimSpace(r.PostFormValue("note")),
		GPC:       legal.GPC(r.Header),
	}
	// The hourly limit is counted inside the insert, not before it: a count
	// read first lets a burst of parallel posts all read the same number.
	hourAgo := now.Add(-time.Hour)
	_, err = h.requests.AddWithin(ctx, req, hourAgo, optoutSiteLimit)
	switch {
	case errors.Is(err, dsr.ErrLimit):
		optoutRedirect(w, r, back, "spaeter")
	case errors.Is(err, dsr.ErrEmpty):
		optoutRedirect(w, r, back, "leer")
	case errors.Is(err, dsr.ErrTooLong):
		optoutRedirect(w, r, back, "zulang")
	case err != nil:
		return fmt.Errorf("store opt-out: %w", err)
	default:
		h.optoutNotifyWithin(ctx, ws, req, hourAgo)
		optoutRedirect(w, r, back, "eingegangen")
	}
	return nil
}

// optoutNotifyWithin mails the operator about a stored request unless the
// website already took optoutMailLimit of them this hour. The count is read
// after this request's own insert, so it is never below this request's place
// in the hour: a burst can only send fewer mails than the limit, never more.
func (h *Handler) optoutNotifyWithin(ctx context.Context, ws *domain.Website, req dsr.Request, since time.Time) {
	n, err := h.requests.CountSince(ctx, ws.ID, dsr.KindOptOut, since)
	if err != nil {
		slog.Error("count opt-outs for the notification", "err", err, "website", ws.ID)
		return
	}
	if n > optoutMailLimit {
		return
	}
	h.optoutNotify(ctx, ws, req)
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

// What the form's time stamp says about who sent it.
type stampVerdict int

const (
	// stampFresh is a stamp this page drew, inside its time window.
	stampFresh stampVerdict = iota
	// stampStale is a stamp this page drew, but too long ago or a moment
	// ago: a tab left open, or a form the browser filled in and the visitor
	// sent at once. A person, who is asked to send again.
	stampStale
	// stampForged is no stamp, or one this page did not draw. A robot, which
	// gets the success answer and nothing stored.
	stampForged
)

// OptOutStamp is the time stamp the opt-out form of a website carries, drawn
// at issued: the issue time in the id slot of a share-link token, signed so
// that a robot cannot back-date it, and checked for its age when the form
// comes back.
//
// The signer is derived per website, so a stamp drawn on one website is a
// forgery on every other: one signer for all of them let a single GET feed
// posts to every website of the installation (WR-01).
func OptOutStamp(signer *sharelink.Signer, websiteID int64, issued time.Time) string {
	return optoutStampSigner(signer, websiteID).Token(issued.Unix(), issued.Add(optoutMaxAge))
}

func optoutStampSigner(signer *sharelink.Signer, websiteID int64) *sharelink.Signer {
	return signer.Derive("website:" + strconv.FormatInt(websiteID, 10))
}

// optoutStampCheck tells the three kinds of stamp apart.
//
// The signature decides between a person and a robot, and the age only
// between "now" and "please again": a stamp that verifies proves it came from
// this website's page, so answering its sender honestly tells a robot nothing
// it did not already know. sharelink checks expiry after the signature for
// exactly this distinction.
func (h *Handler) optoutStampCheck(ws *domain.Website, stamp string, now time.Time) stampVerdict {
	issued, err := optoutStampSigner(h.optoutSigner, ws.ID).Check(stamp, now)
	switch {
	case errors.Is(err, sharelink.ErrExpired):
		return stampStale
	case err != nil:
		return stampForged
	}
	age := now.Sub(time.Unix(issued, 0))
	if age < optoutMinAge || age > optoutMaxAge {
		return stampStale
	}
	return stampFresh
}

// optoutNotify tells the operator about a stored opt-out, by the rule every
// notification of this program follows (NotifyForPlugin): only when mail is
// set up for the website and the website has a notification address. Neither
// is a failure — the request is stored and listed either way.
//
// In the website's main language, because the reader is the operator and not
// the visitor (D-16). The visitor's address stays in the body and never
// becomes a Reply-To: a header is built from what the operator configured and
// from nothing a stranger typed. A mail error is logged and the visitor still
// gets their confirmation, because their request is recorded.
func (h *Handler) optoutNotify(ctx context.Context, ws *domain.Website, req dsr.Request) {
	if h.mail == nil || ws.NotifyEmail == "" || !h.mail.EnabledFor(ctx, ws.ID) {
		return
	}
	lang := ws.Locale
	var b strings.Builder
	// Label and value, one per line: a list, not a sentence.
	line := func(label, value string) {
		if value != "" {
			b.WriteString(label + ": " + value + "\n")
		}
	}
	line(i18n.T(lang, "Name"), req.Name)
	line(i18n.T(lang, "E-mail address"), req.Email)
	line(i18n.T(lang, "Note"), req.Note)
	line(i18n.T(lang, "Language"), req.Locale)
	b.WriteString("\n")
	if req.GPC {
		b.WriteString(i18n.T(lang, "The browser sent the Global Privacy Control signal."))
	} else {
		b.WriteString(i18n.T(lang, "The browser did not send the Global Privacy Control signal."))
	}
	b.WriteString("\n\n")
	b.WriteString(i18n.T(lang, "The request is listed under Settings → Opt-outs of this website."))
	b.WriteString("\n")

	msg := mail.Message{
		To: ws.NotifyEmail,
		// The site's name in front, as on every notification: an operator
		// running four websites gets four kinds into one inbox.
		Subject: "[" + ws.Name + "] " + i18n.T(lang, "New opt-out request (Do Not Sell or Share)"),
		Body:    b.String(),
	}
	if err := h.mail.Enqueue(ctx, ws.ID, msg); err != nil {
		slog.Error("queue opt-out notification", "err", err, "website", ws.ID)
	}
}

// HandleGPCJSON answers /.well-known/gpc.json for a website that follows US
// law: the resource the Global Privacy Control specification defines for a
// site to say that it honours the signal. lastUpdate is optional there, and
// nothing stores a date for it, so it is left out rather than invented.
//
// A website without US in its set answers 404 — it makes no such statement.
func (h *Handler) HandleGPCJSON(w http.ResponseWriter, r *http.Request) error {
	ws := domain.WebsiteFromContext(r.Context())
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}
	if !ws.Frameworks().Has(legal.US) {
		return h.serve404(w, r, ws)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, err := io.WriteString(w, `{"gpc": true}`)
	return err
}

// optoutVary names the request headers an answer depends on beyond
// HX-Request: Sec-GPC when the widget is in it, because its first line does,
// and nothing otherwise. Every other page of a US website is the same with
// and without the signal (D-12), and naming Sec-GPC there would split a
// shared cache's entry in two for nothing.
func (h *Handler) optoutVary(content []byte) []string {
	if bytes.Contains(content, []byte(optoutWidgetMark)) {
		return []string{legal.GPCHeader}
	}
	return nil
}
