package public

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// The request about one's own data (Phase 33.2, D-01).
//
// A page carries [[datenanfrage]], and at delivery the marker becomes a form:
// a visitor asks for a copy of their data, for it to be corrected, or for it
// to be erased, names the address the request is about, and sends it without
// an account and without a cookie. The request lands in privacy_requests,
// beside the opt-outs (33.2 D-09), and the operator reads it under Settings →
// Privacy → Requests.
//
// The form never answers the request. Whatever was typed, the visitor is
// told the same sentence — recorded, the operator answers after checking it
// is theirs — and nothing about any person is ever on the page: a form that
// said "we hold nothing for this address" would tell anybody who types an
// address whether its owner is a customer here. Finding the data, checking
// the identity and sending the answer stay with the operator.
//
// Every website may carry it, whatever law it follows: the right of access
// is in all four frameworks, and a website that answers no such request is
// not one this program helps build.
//
// It stands on the opt-out form's machinery (optout.go), not beside it: the
// same-origin check, the bounded body, the honeypot, the signed time stamp,
// the per-network budget, the hourly ceiling counted inside the insert and the
// hourly mail cap are takeRequest's, and the two forms share them. What is
// this form's own is its marker, its fields, its stamp's derivation and its
// mail.

// dataRequestMarker is what a page writes where the form goes.
const dataRequestMarker = "[[datenanfrage]]"

// dataRequestWidgetMark is the attribute the form carries. optoutModTime
// looks for it in a rendered answer, so it has to be something no editor's
// content produces by accident.
const dataRequestWidgetMark = "data-hc-datarequest"

var (
	// dataRequestParagraph is the marker on a line of its own, wrapped in a
	// paragraph that goes with it (optoutParagraph says why).
	dataRequestParagraph = regexp.MustCompile(`<p>\s*\[\[datenanfrage\]\]\s*</p>`)
	// dataRequestPattern is the marker anywhere else.
	dataRequestPattern = regexp.MustCompile(`\[\[datenanfrage\]\]`)
)

// dataRequestOffered reports whether this website takes requests here: any
// website, when the handler has somewhere to store them and a way to sign
// the form.
func (h *Handler) dataRequestOffered(ws *domain.Website) bool {
	return ws != nil && h.requests != nil && h.optoutSigner != nil
}

// expandDataRequest puts the request form where a page says [[datenanfrage]],
// by expandOptOut's rules (placeWidget): only a marker in text, the paragraph
// around it taken with it, one form per page. At delivery, because the form
// carries a stamp drawn for this answer.
func (h *Handler) expandDataRequest(r *http.Request, ws *domain.Website, pg *page.Page, body string) string {
	if !strings.Contains(body, dataRequestMarker) {
		return body
	}
	widget := ""
	if h.dataRequestOffered(ws) {
		widget = h.dataRequestWidget(r, ws, pg)
	}
	return placeWidget(body, dataRequestParagraph, dataRequestPattern, widget)
}

// withoutDataRequest removes the marker from a body that is not a page being
// served — a feed entry, what a plugin reads — as withoutOptOut does for the
// opt-out's: neither can send the form, and nobody may see the marker.
func withoutDataRequest(body string) string {
	return withoutWidget(body, dataRequestMarker, dataRequestParagraph, dataRequestPattern)
}

// dataRequestStampLabel is the request form's derivation on one website:
// not the opt-out's, so a stamp drawn for one form is a forgery on the other.
func dataRequestStampLabel(websiteID int64) string {
	return "datenanfrage:website:" + strconv.FormatInt(websiteID, 10)
}

// DataRequestStamp is the time stamp the request form of a website carries,
// drawn at issued (formStamp).
func DataRequestStamp(signer *sharelink.Signer, websiteID int64, issued time.Time) string {
	return formStamp(signer, dataRequestStampLabel(websiteID), issued)
}

// dataRequestWidget draws the form. Every word goes through the catalogue
// with its key at the call site and is escaped, as in optoutWidget, in the
// language optoutLang chooses. The classes are the contact form's, which all
// shipped themes style; no hx- attribute, because the public site loads no
// script.
func (h *Handler) dataRequestWidget(r *http.Request, ws *domain.Website, pg *page.Page) string {
	lang := optoutLang(r)
	e := html.EscapeString

	var b strings.Builder
	fmt.Fprintf(&b, `<form class="contact-form" %s method="post" action="%s">`,
		dataRequestWidgetMark, e(h.localePath(r, ws, "/datenanfrage")))

	// What the last submission came to, by name and never by sentence, so
	// nobody can put words on this page through a link. The recorded answer
	// is the same for every address: it says nothing about stored data.
	result, isError := "", false
	switch r.URL.Query().Get("datenanfrage") {
	case "eingegangen":
		result = i18n.T(lang, "Your request has been recorded. The operator of this website answers it after checking that it is yours.")
	case "leer", "adresse":
		result, isError = i18n.T(lang, "Please give the e-mail address your request is about."), true
	case "art":
		result, isError = i18n.T(lang, "Please choose what you would like."), true
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
		e(DataRequestStamp(h.optoutSigner, ws.ID, time.Now())))

	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="hc-datarequest-kind">%s</label>`+
		`<select id="hc-datarequest-kind" name="art" required>`+
		`<option value="%s">%s</option><option value="%s">%s</option><option value="%s">%s</option>`+
		`</select></div>`,
		e(i18n.T(lang, "What would you like?")),
		dsr.KindAccess, e(i18n.T(lang, "A copy of my data (access)")),
		dsr.KindCorrection, e(i18n.T(lang, "Correct my data")),
		dsr.KindErasure, e(i18n.T(lang, "Erase my data")))
	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="hc-datarequest-email">%s</label>`+
		`<input type="email" id="hc-datarequest-email" name="email" required maxlength="%d" autocomplete="email"></div>`,
		e(i18n.T(lang, "E-mail address the request is about")), dsr.MaxEmail)
	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="hc-datarequest-name">%s</label>`+
		`<input type="text" id="hc-datarequest-name" name="name" maxlength="%d" autocomplete="name"></div>`,
		e(i18n.T(lang, "Name (optional)")), dsr.MaxName)
	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="hc-datarequest-note">%s</label>`+
		`<textarea id="hc-datarequest-note" name="note" rows="4" maxlength="%d"></textarea></div>`,
		e(i18n.T(lang, "Note (optional)")), dsr.MaxNote)

	// The honeypot, as the opt-out form has it.
	fmt.Fprintf(&b, `<div class="contact-form__trap" aria-hidden="true">`+
		`<label for="hc-datarequest-website">%s</label>`+
		`<input type="text" id="hc-datarequest-website" name="website" tabindex="-1" autocomplete="off"></div>`,
		e(i18n.T(lang, "Website (please leave empty)")))

	fmt.Fprintf(&b, `<p class="contact-form__alternative">%s</p>`,
		e(i18n.T(lang, "What you enter here is stored only to record and answer this request. Nothing about you is shown on this page, and no account is needed.")))
	fmt.Fprintf(&b, `<button type="submit" class="contact-form__submit">%s</button>`,
		e(i18n.T(lang, "Send request")))
	b.WriteString(`</form>`)
	return b.String()
}

// HandleDataRequest takes the request form (POST /datenanfrage), through
// takeRequest's steps and order. What the form adds is the kind: one of the
// three data requests, and nothing else — an opt-out posted here is refused
// like any unknown kind, because it has its own form and its own family in
// the hourly ceiling.
func (h *Handler) HandleDataRequest(w http.ResponseWriter, r *http.Request) error {
	ws := domain.WebsiteFromContext(r.Context())
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}
	if !h.dataRequestOffered(ws) {
		return h.serve404(w, r, ws)
	}
	return h.takeRequest(w, r, ws, requestForm{
		query:      "datenanfrage",
		stampLabel: dataRequestStampLabel(ws.ID),
		build: func(r *http.Request) (dsr.Request, error) {
			kind := r.PostFormValue("art")
			if !dsr.IsDataKind(kind) {
				return dsr.Request{}, dsr.ErrBadKind
			}
			return dsr.Request{
				Kind:   kind,
				Manner: dsr.MannerForm,
				Name:   strings.TrimSpace(r.PostFormValue("name")),
				Email:  strings.TrimSpace(r.PostFormValue("email")),
				Note:   strings.TrimSpace(r.PostFormValue("note")),
			}, nil
		},
		notify: h.dataRequestNotify,
	})
}

// dataRequestNotify tells the operator about a stored request, by
// optoutNotify's rules: only when mail is set up for the website and it has a
// notification address, in the website's main language, the visitor's
// address in the body and never in a header, a mail error logged without it.
func (h *Handler) dataRequestNotify(ctx context.Context, ws *domain.Website, req dsr.Request) {
	if h.mail == nil || ws.NotifyEmail == "" || !h.mail.EnabledFor(ctx, ws.ID) {
		return
	}
	lang := ws.Locale
	var subject string
	switch req.Kind {
	case dsr.KindAccess:
		subject = i18n.T(lang, "New request for access to personal data")
	case dsr.KindCorrection:
		subject = i18n.T(lang, "New request to correct personal data")
	default:
		subject = i18n.T(lang, "New request to erase personal data")
	}
	var b strings.Builder
	// Label and value, one per line: a list, not a sentence.
	line := func(label, value string) {
		if value != "" {
			b.WriteString(label + ": " + value + "\n")
		}
	}
	line(i18n.T(lang, "Kind"), i18n.T(lang, dsr.KindName(req.Kind)))
	line(i18n.T(lang, "Name"), req.Name)
	line(i18n.T(lang, "E-mail address"), req.Email)
	line(i18n.T(lang, "Note"), req.Note)
	line(i18n.T(lang, "Language"), req.Locale)
	received := time.Now().In(tmpl.LoadLocation(ws.TimeZone))
	line(i18n.T(lang, "Received"), received.Format("02.01.2006 15:04 MST"))
	// The deadline of the website's frameworks, in its zone (dsr.DueFor):
	// the operator who reads only the inbox learns it here.
	if due, ok := dsr.DueFor(received, ws.Frameworks()); ok {
		line(i18n.T(lang, "Answer due by"), due.Answer.Format("02.01.2006")+" ("+strings.ToUpper(string(due.By))+")")
		if !due.Acknowledge.IsZero() {
			line(i18n.T(lang, "Acknowledge by about"), due.Acknowledge.Format("02.01.2006"))
		}
	}
	b.WriteString("\n")
	b.WriteString(i18n.T(lang, "The request is listed under Settings → Privacy → Requests of this website."))
	b.WriteString("\n")

	msg := mail.Message{
		To:      ws.NotifyEmail,
		Subject: "[" + ws.Name + "] " + subject,
		Body:    b.String(),
	}
	if err := h.mail.Enqueue(ctx, ws.ID, msg); err != nil {
		slog.Error("queue data request notification", "err", err, "website", ws.ID)
	}
}
