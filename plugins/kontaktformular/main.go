// The contact form as a plugin.
//
// Three parts that belong together: the marker [[formular]] in the text becomes
// the form, /formular receives the submission, and the messages sit in the
// admin. All three in one module, because they share the same idea of field
// names, traps and storage — spread over three plugins the first change to a
// field name would be a silent break.
//
// Why this is not core: a website that receives no messages needs neither the
// form nor the table behind it nor the screen that never has anything on it.
// Whoever publishes only a phone number should not have to carry the whole
// thing along.
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// Field names. They stand in the drawn form and are read again on receipt; as
// constants, so that the two places do not drift apart.
const (
	fieldName    = "name"
	fieldEmail   = "email"
	fieldSubject = "betreff"
	fieldText    = "nachricht"
	fieldPage    = "seite"
	// fieldPath carries the address the page is served under, which is where a
	// refusal and a thank-you have to go back to. The slug alone is not it: the
	// start page's slug is "home" and the start page is at "/", so every
	// message about a form on the start page went to /home, was redirected to
	// /, and lost its query — the visitor saw the form again and no answer.
	fieldPath = "pfad"
	// fieldLang is the language the form was DRAWN in.
	//
	// The submission goes to /formular, which carries no language: the host
	// answers it in the website's first one, and the consent recorded with the
	// message would then be the sentence in a language the sender never saw.
	// What has to be stored is the sentence they actually read, and only the
	// form they read it on knows which that was.
	fieldLang     = "sprache"
	fieldTime     = "gestellt"
	fieldHoneypot = "website"
	// fieldForm says on receipt which assembled form was submitted. Without it
	// the receiving side would have to guess from the field names.
	fieldForm = "formular"

	// fieldConsent is the tick box, present only when the operator wrote a
	// sentence for it.
	fieldConsent = "einwilligung"

	// fieldAttach is the file field, present only when the operator asked for
	// it.
	fieldAttach = "anhang"
)

// submitAddress is a fixed address and not the page's: then every page stays a
// plain GET, and the form works the same everywhere.
const submitAddress = "/formular"

// The fields' limits. They are what keeps a single submission from filling a
// memory card, and wide enough for any real enquiry.
const (
	maxName    = 120
	maxEmail   = 254 // the longest address RFC 5321 allows
	maxBetreff = 200
	maxText    = 8000
)

// minimumDuration is what a human being needs at least.
//
// Three seconds are below what anybody needs to read and type, and far above
// what a script needs. Longer would start refusing people who paste in a
// message they prepared.
const minimumDuration = 3 * time.Second

// maxAge is how long a drawn form stays valid. A page can stand in a tab for a
// long time; the point is only that a timestamp copied once is not reusable for
// weeks.
const maxAge = 12 * time.Hour

// hourlyLimit is how many messages a website accepts per hour.
//
// The honeypot and the time trap stop the ordinary spam robot. This stops the
// one that gets through anyway from filling a small server's disk overnight —
// and no amount of filtering afterwards makes that good again.
const hourlyLimit = 30

// maxMessages is how many messages are kept.
const maxMessages = 500

// keyConsent is where the operator's consent sentence is kept.
//
// Empty means no tick box: a form that asks for nothing must not grow one
// because the plugin was updated. Switching it on is the operator writing the
// sentence, which is also the only way this can be right — what a visitor has
// to agree to is a matter for the person running the site, not for a CMS.
const keyConsent = "einwilligungstext"

// keyAttach switches the attachment field on.
//
// Off until the operator asks for it, and for the same reason as the receipt:
// switching it on means strangers may put files on this server. An operator who
// reads the setting and switches it on has weighed that.
const keyAttach = "anhang"

// maxAttachments is what one submission may bring. The host bounds it too; this
// is the number the form offers.
const maxAttachments = 3

// maxConsent bounds it. A consent nobody reads to the end is not consent, and a
// tick box with a page of text beside it is exactly that.
const maxConsent = 400

const (
	prefixMessage  = "nachricht:"
	praefixZaehler = "zaehler:"
	schluesselName = "signaturschluessel"
)

// attachment is one file kept with a message.
//
// An id and a name. The plugin never had the bytes and cannot read them back;
// what it can do is put the id into a link, which is what the admin does.
type attachment struct {
	MediaID int64 `json:"medien_id"`
	// Name is what the sender called it; Filename is what it is called here,
	// which is the handle /media/ takes. Both are kept, because a link needs
	// the second and a person needs the first.
	Name     string `json:"name"`
	Filename string `json:"dateiname"`
}

// message is an enquiry that has come in.
type message struct {
	// Key is the storage key without its prefix, so that a form in the admin
	// can point at a single message.
	Key     string `json:"kennung"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Subject string `json:"betreff,omitempty"`
	Text    string `json:"text"`
	Page    string `json:"seite,omitempty"`
	Time    string `json:"zeit"`
	Read    bool   `json:"gelesen,omitempty"`
	// Form and FormName say where the message came from. Empty for the built-in
	// contact form.
	Form     string `json:"formular,omitempty"`
	FormName string `json:"formularname,omitempty"`
	// Fields are the answers of an assembled form, in the order they were
	// asked.
	Fields []answer `json:"felder,omitempty"`
	// Consent is the sentence the sender ticked, word for word, and when.
	//
	// The wording is stored and not only a yes, because a consent nobody can
	// reconstruct is not one: an operator who changes the sentence next year
	// would otherwise have no way to say what this person actually agreed to.
	// Empty on every message from before the operator asked for consent, which
	// is how it should read — they were not asked, so they did not agree.
	Consent     string `json:"einwilligung,omitempty"`
	ConsentTime string `json:"einwilligungszeit,omitempty"`
	// Attachments are the files that came with the message, as media ids.
	//
	// Ids and not bytes: the plugin never had the file and cannot read it back.
	// What it can do is put the id into a link, which is what the admin does.
	Attachments []attachment `json:"anhaenge,omitempty"`
	// RefusedFor is why a submission was turned away, empty on a real message.
	// It is the one field that tells the two lists apart, so that a submission
	// released into the messages cannot keep looking refused.
	RefusedFor string `json:"abgewiesen,omitempty"`
	// Reply is what the operator wrote back, and when.
	//
	// Kept with the message rather than only sent, because the point of
	// answering from here is that the next person to open the screen can see
	// that it was answered. A reply that exists only in somebody's sent folder
	// is a reply the colleague cannot see.
	Reply     string `json:"antwort,omitempty"`
	ReplyTime string `json:"antwortzeit,omitempty"`
}

func init() {
	plugin.OnContent(formularEinsetzen)
	plugin.OnRoute(absendungAnnehmen)
	plugin.OnAdmin(verwaltung)
}

// --- die Marke im Text ------------------------------------------------------

// marker matches the marker with or without a subject:
//
//	[[formular]]              a plain contact form
//	[[formular:Rohwolle]]     the same, with "Rohwolle" already in the subject
//
// The subject is for a page that offers one thing. Without it every enquiry
// from every product page arrives with an empty subject line, and whoever reads
// them has to open each one to see which of five things it is about.
//
// The square bracket is excluded from the subject, so that a marker with no
// closing bracket does not swallow the rest of the page.
var marker = regexp.MustCompile(`\[\[formular(?::([^\]\[]{1,` + strconv.Itoa(maxBetreff) + `}))?\]\]`)

// markerInParagraph is the same marker with the paragraph goldmark wraps it in.
//
// A marker alone in a paragraph replaces the whole paragraph: a <form> inside a
// <p> is invalid HTML that a browser silently reorders — it pulls the form out
// and leaves the fields behind.
var markerInParagraph = regexp.MustCompile(`<p>` + marker.String() + `</p>`)

func formularEinsetzen(in plugin.ContentIn) (plugin.ContentOut, error) {
	if !marker.MatchString(in.HTML) {
		return plugin.ContentOut{}, nil
	}

	data := formData(in)
	// This program puts nothing a visitor typed back into an address of its own
	// accord: that would be writing somebody else's text into the history and
	// into every server log. A refused submission therefore says what is
	// missing and no more; the browser still has the entries when they go back.
	//
	// What IS read back is a query the visitor's own browser wrote, which is
	// what the Continue button of a two-step form is: formmethod="get" means
	// exactly this. Reading it is not the same act as writing it.
	query, err := url.ParseQuery(in.Query)
	if err != nil {
		query = nil
	}
	// The paragraph first, so that the <p> disappears along with its marker. In
	// one pass an empty <p></p> would be left behind.
	out := ersetzen(markerInParagraph, in.HTML, data, query)
	out = ersetzen(marker, out, data, query)
	return plugin.ContentOut{HTML: out, Changed: out != in.HTML}, nil
}

func ersetzen(re *regexp.Regexp, page string, d data, query url.Values) string {
	return re.ReplaceAllStringFunc(page, func(treffer string) string {
		arg := ""
		if m := marker.FindStringSubmatch(treffer); m != nil {
			arg = strings.TrimSpace(m[1])
		}

		// An argument naming an assembled form brings that one; every other is
		// a pre-filled subject as before. So [[formular:Rohwolle]] stays
		// exactly what it was, and [[formular:hoffest]] is something new.
		if arg != "" {
			if f, ok := formularLaden(arg); ok {
				eigen := d
				if d.Form != "" && d.Form != f.Key {
					// The answer belongs to another form on the same page; this
					// one shows no foreign message.
					eigen.Hint, eigen.IsError = "", false
				}
				return zeichnenEigen(f, eigen, answersFromQuery(f, query))
			}
		}

		eigen := d
		if arg != "" && eigen.Subject == "" {
			// What the visitor typed themselves wins: their submission was
			// refused for another reason, and overwriting their subject on top
			// of that would be the second thing that happens to them.
			eigen.Subject = arg
		}
		if d.Form != "" {
			eigen.Hint, eigen.IsError = "", false
		}
		return draw(eigen)
	})
}

// data is everything the drawn form needs.
type data struct {
	Page string
	// Path is the address the page is served under, which is where a two-step
	// form points its Continue button. Not "/"+Page: the start page has the
	// slug "home" and is served at the root.
	Path string
	// Lang is the language the form is drawn in.
	Lang string
	// Form is the key of the form whose submission is being answered. Only that
	// one shows the message — on a page with two forms it would otherwise stand
	// twice.
	Form      string
	Timestamp string
	Kontakt   string
	Hint      string
	IsError   bool
	// ErrorField is the form field the message belongs under, empty when it is
	// about the submission as a whole.
	ErrorField string
	// Attach says the form offers a file field.
	Attach bool
	// Consent is the operator's sentence, or empty when they ask for none.
	//
	// It is the operator's own HTML, cleaned by the host the way any of their
	// text is, so that a link to their privacy page can stand inside it.
	Consent string
	Name    string
	Email   string
	Subject string
	Text    string
}

func formData(in plugin.ContentIn) data {
	d := data{Page: in.Slug, Path: in.Path, Lang: in.Lang, Timestamp: timestamp(time.Now())}
	if s, err := plugin.Site(); err == nil {
		d.Kontakt = s.ContactEmail
	}
	d.Consent = consentText(in.Lang)
	d.Attach = attachWanted()

	q, err := url.ParseQuery(in.Query)
	if err != nil {
		return d
	}
	d.Form = q.Get("welches")
	switch q.Get("formular") {
	case "gesendet":
		f, ok := formularLaden(d.Form)
		d.Hint = hintFor(f, ok)
	case "fehler":
		d.IsError = true
		d.Hint, d.ErrorField = refusalText(q.Get("grund"), q.Get("feld"), q.Get("wort"))
	}
	return d
}

// refusalText turns what the address bar brought back into a sentence.
//
// Everything is checked against what this program knows: an unknown code gives
// the general sentence, and a field name that is not one of this form's fields
// is dropped so the message goes above the form instead of being attached to
// something that is not there. Nothing from the address bar reaches the page as
// text except the label, which is bounded and escaped where it is written.
func refusalText(code, field, word string) (text, at string) {
	frame, known := reasons[code]
	if !known {
		return plugin.T("The message could not be sent. Please check what you entered."), ""
	}
	if strings.Contains(frame, "%s") {
		word = strings.TrimSpace(word)
		if r := []rune(word); len(r) > maxLabel {
			word = string(r[:maxLabel])
		}
		text = plugin.Tf(frame, word)
	} else {
		text = plugin.T(frame)
	}
	return text, fieldName2(field)
}

// maxLabel bounds the operator's own word on its way through the address bar.
// The editor already bounds a label to 120; this is the same number said again
// where the value comes back from outside.
const maxLabel = 120

// fieldName2 keeps only a field name this form could actually have.
func fieldName2(raw string) string {
	switch raw {
	case fieldName, fieldEmail, fieldSubject, fieldText, fieldConsent, fieldAttach:
		return raw
	}
	// An assembled form's field: the prefix plus a key, and a key is the narrow
	// shape reKey allows.
	if rest, ok := strings.CutPrefix(raw, fieldPrefix); ok && reKey.MatchString(rest) {
		return raw
	}
	return ""
}

// hintText bounds what the query string may bring onto the page.
//
// The output is escaped anyway; the limit is aimed at a link that puts a page
// of somebody else's text into a website's layout — which is how a plain
// contact form becomes a phishing page.
func hintText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len([]rune(raw)) > 160 {
		return plugin.T("The message could not be sent. Please check what you entered.")
	}
	return raw
}

// draw builds the form.
//
// In Go and not in the theme: the markup carries the honeypot, the timestamp
// and the field names the receiving side expects. A theme that got one of them
// wrong would produce a form that silently refuses every real visitor. The
// styling goes through the classes.
func draw(d data) string {
	e := html.EscapeString
	var b strings.Builder
	// No novalidate. The browser catching an empty required field on the spot
	// is better than a round trip, and it needs no JavaScript; everything it
	// cannot judge — the shape of an address, a length, an hour that was too
	// busy — comes back from the server with a sentence in the page's language
	// under the field it is about.
	enctype := ""
	if d.Attach {
		// Without this the browser sends only the file NAMES, and the form
		// silently arrives with no attachment at all — the sort of defect that
		// looks like the server losing files.
		enctype = ` enctype="multipart/form-data"`
	}
	fmt.Fprintf(&b, `<form class="contact-form" method="POST" action="%s"%s>`, submitAddress, enctype)
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldTime, e(d.Timestamp))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldPage, e(d.Page))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldPath, e(d.Path))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldLang, e(d.Lang))

	// The notice above the form is for what is not about one field: a success,
	// a form that expired, an hour that was too busy. A refusal that names a
	// field goes under that field instead — reading "the e-mail address does
	// not look right" above a form of four boxes leaves the visitor to work out
	// which box, and they are the one person who cannot see the code.
	if d.Hint != "" && d.ErrorField == "" {
		class := "contact-form__notice"
		if d.IsError {
			class += " contact-form__notice--error"
		}
		fmt.Fprintf(&b, `<p class="%s" role="status">%s</p>`, class, e(d.Hint))
	}

	// required is a word and not only a star. A star alone is a convention that
	// has to be learnt, and it is read out as "star" or skipped entirely.
	required := ` <span class="contact-form__required">` + e(plugin.T("(required)")) + `</span>`

	field := func(id, label, typ, name, value string, max int, req bool, extra string) {
		wrong := d.ErrorField == name && d.IsError
		class := "contact-form__field"
		if wrong {
			class += " contact-form__field--wrong"
			extra += ` aria-invalid="true" aria-describedby="` + id + `-why"`
		}
		fmt.Fprintf(&b, `<div class="%s"><label for="%s">%s`, class, id, e(label))
		if req {
			b.WriteString(required)
		}
		b.WriteString(`</label>`)
		if wrong {
			// role="alert" so a screen reader says it on arrival. The visitor
			// came back to this page BECAUSE of this sentence; it is the reason
			// the page was loaded.
			fmt.Fprintf(&b, `<p class="contact-form__why" id="%s-why" role="alert">%s</p>`,
				id, e(d.Hint))
		}
		fmt.Fprintf(&b, `<input type="%s" id="%s" name="%s" maxlength="%d" value="%s" %s></div>`,
			typ, id, name, max, e(value), extra)
	}
	field("cf-name", plugin.T("Name"), "text", fieldName, d.Name, maxName, true,
		`required autocomplete="name"`)
	field("cf-email", plugin.T("E-mail"), "email", fieldEmail, d.Email, maxEmail, true,
		`required autocomplete="email"`)
	field("cf-subject", plugin.T("Subject"), "text", fieldSubject, d.Subject, maxBetreff, false, "")

	textWrong := d.ErrorField == fieldText && d.IsError
	textClass := "contact-form__field"
	textExtra := ""
	if textWrong {
		textClass += " contact-form__field--wrong"
		textExtra = ` aria-invalid="true" aria-describedby="cf-body-why"`
	}
	fmt.Fprintf(&b, `<div class="%s"><label for="cf-body">%s%s</label>`,
		textClass, e(plugin.T("Message")), required)
	if textWrong {
		fmt.Fprintf(&b, `<p class="contact-form__why" id="cf-body-why" role="alert">%s</p>`,
			e(d.Hint))
	}
	// The limit is said before it is reached rather than after. Without
	// JavaScript there is no counter that runs; what there can be is a number
	// the visitor sees before writing two thousand and one characters and being
	// sent back.
	fmt.Fprintf(&b, `<textarea id="cf-body" name="%s" rows="8" required maxlength="%d"%s>%s</textarea>`+
		`<span class="contact-form__limit">%s</span></div>`,
		fieldText, maxText, textExtra, e(d.Text),
		e(plugin.Tf("at most %d characters", maxText)))

	// No visible field. The stylesheet hides it from people, aria-hidden and
	// tabindex from screen readers; whoever does not see it either way leaves
	// it alone. A program that fills in every input it finds fills this one in
	// too — which is exactly what it is for.
	fmt.Fprintf(&b, `<div class="contact-form__trap" aria-hidden="true">`+
		`<label for="cf-website">%s</label>`+
		`<input type="text" id="cf-website" name="%s" tabindex="-1" autocomplete="off"></div>`,
		e(plugin.T("Website (please leave empty)")), fieldHoneypot)

	if d.Attach {
		wrong := d.ErrorField == fieldAttach && d.IsError
		class := "contact-form__field"
		extra := ""
		if wrong {
			class += " contact-form__field--wrong"
			extra = ` aria-invalid="true" aria-describedby="cf-attach-why"`
		}
		fmt.Fprintf(&b, `<div class="%s"><label for="cf-attach">%s</label>`,
			class, e(plugin.T("Attachment")))
		if wrong {
			fmt.Fprintf(&b, `<p class="contact-form__why" id="cf-attach-why" role="alert">%s</p>`,
				e(d.Hint))
		}
		// multiple, and the host bounds the number as well: a form that offers
		// three and takes one is a form that loses files without saying so.
		fmt.Fprintf(&b, `<input type="file" id="cf-attach" name="%s" multiple%s>`+
			`<span class="contact-form__limit">%s</span></div>`,
			fieldAttach, extra, e(plugin.Tf("at most %d files", maxAttachments)))
	}

	if d.Consent != "" {
		wrong := d.ErrorField == fieldConsent && d.IsError
		class := "contact-form__consent"
		extra := ""
		if wrong {
			class += " contact-form__field--wrong"
			extra = ` aria-invalid="true" aria-describedby="cf-consent-why"`
		}
		fmt.Fprintf(&b, `<div class="%s">`, class)
		if wrong {
			fmt.Fprintf(&b, `<p class="contact-form__why" id="cf-consent-why" role="alert">%s</p>`,
				e(d.Hint))
		}
		// The sentence stands beside the box and is not a link to a sentence.
		// Whoever has to follow a link to find out what they are agreeing to
		// has not agreed to it; an operator who wants a link puts one inside
		// their own sentence, which is why this goes through th.
		fmt.Fprintf(&b, `<label for="cf-consent"><input type="checkbox" id="cf-consent" `+
			`name="%s" value="1" required%s> <span>%s</span></label></div>`,
			fieldConsent, extra, d.Consent)
	}

	fmt.Fprintf(&b, `<button type="submit" class="contact-form__submit">%s</button>`,
		e(plugin.T("Send message")))
	if d.Kontakt != "" {
		fmt.Fprintf(&b, `<p class="contact-form__alternative">%s `+
			`<a href="mailto:%s">%s</a></p>`,
			e(plugin.T("Would you rather write directly?")), e(d.Kontakt), e(d.Kontakt))
	}
	b.WriteString(`</form>`)
	return b.String()
}

// --- die Zeitmarke ----------------------------------------------------------

// timestamp signs the moment the form was drawn.
//
// Signed, because an unsigned hidden field is one a robot simply back-dates —
// the time trap would then be a comment in the source and nothing else.
func timestamp(jetzt time.Time) string {
	stempel := strconv.FormatInt(jetzt.UTC().Unix(), 10)
	return stempel + "." + signature(stempel)
}

// checkTimestamp returns a reason, or "" when everything is right.
func checkTimestamp(marker string, jetzt time.Time) string {
	stempel, sig, ok := strings.Cut(marker, ".")
	if !ok {
		return "gefaelscht"
	}
	// Compared over the full length: a comparison that stops at the first wrong
	// byte gives away how much of the signature is already right.
	if !hmac.Equal([]byte(sig), []byte(signature(stempel))) {
		return "gefaelscht"
	}
	sekunden, err := strconv.ParseInt(stempel, 10, 64)
	if err != nil {
		return "gefaelscht"
	}
	alter := jetzt.UTC().Sub(time.Unix(sekunden, 0).UTC())
	switch {
	case alter < 0:
		// Only something whose clock was set back can come from the future —
		// or a forgery that hit the signature regardless.
		return "gefaelscht"
	case alter < minimumDuration:
		return "zu schnell"
	case alter > maxAge:
		return "abgelaufen"
	}
	return ""
}

func signature(stempel string) string {
	mac := hmac.New(sha256.New, signaturschluessel())
	mac.Write([]byte(stempel))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// key is drawn once and kept in the installation-wide storage.
//
// Its own key and not the server's: the plugin does not get to see the server's
// secret, and that is right. It has to hold across restarts, or every open form
// would be invalid after one.
var schluesselCache []byte

func signaturschluessel() []byte {
	if schluesselCache != nil {
		return schluesselCache
	}
	if raw, ok, _ := plugin.GlobalGet(schluesselName); ok && raw != "" {
		if b, err := base64.RawStdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
			schluesselCache = b
			return schluesselCache
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Without randomness there is no signature worth having. A fixed
		// fallback would be worse than no time trap at all, because it looks
		// like one.
		plugin.Log("error", "no randomness available, the time trap stays off")
		return nil
	}
	if err := plugin.GlobalSet(schluesselName, base64.RawStdEncoding.EncodeToString(b)); err != nil {
		plugin.Logf("error", "the signing key could not be stored: %v", err)
	}
	schluesselCache = b
	return schluesselCache
}

// --- der Empfang ------------------------------------------------------------

func absendungAnnehmen(in plugin.RequestIn) (plugin.RequestOut, error) {
	if in.Method != "POST" {
		return back("", "fehler", refusal{Code: "unreadable"}), nil
	}
	form, err := url.ParseQuery(in.Body)
	if err != nil {
		return back("", "fehler", refusal{Code: "unreadable"}), nil
	}
	page := pageName(form.Get(fieldPage))
	// Where the answer goes. It comes out of the submission like everything
	// else here, so it is checked to be an address on this website and nothing
	// more; anything else falls back to the start page.
	ziel := pagePath(form.Get(fieldPath))
	// And the language the sender read the form in, checked against the ones
	// this website publishes in: a tag out of a submission is a stranger's
	// word, and one that is not offered here falls back to the first language
	// rather than reaching for a sentence that does not exist.
	sprache := knownLocale(form.Get(fieldLang))

	// The two traps, in the order that costs least. A filled honeypot and a
	// form that comes back in under three seconds are both answered exactly
	// like a success: a robot that learns which of its submissions were refused
	// learns how to get past the filter.
	if strings.TrimSpace(form.Get(fieldHoneypot)) != "" {
		plugin.Log("info", "honeypot triggered")
		quarantine(form, page, "honeypot")
		return back(ziel, "gesendet", refusal{}), nil
	}
	switch reason := checkTimestamp(form.Get(fieldTime), time.Now()); reason {
	case "":
	case "abgelaufen":
		// Somebody who had a tab open for a day deserves an answer and not
		// silence — the message is real and still stands in the field.
		return back(ziel, "fehler", refusal{Code: "expired"}), nil
	default:
		plugin.Logf("info", "submission refused: %s", reason)
		quarantine(form, page, reason)
		return back(ziel, "gesendet", refusal{}), nil
	}

	// An assembled form says which one it is itself. If the key there no longer
	// exists, the submission is refused rather than read as the built-in form:
	// the fields would not match, and an empty message would come out.
	welches := strings.TrimSpace(form.Get(fieldForm))
	var n message
	if welches != "" {
		f, ok := formularLaden(welches)
		if !ok {
			return backTo(ziel, welches, "fehler", refusal{Code: "gone"}), nil
		}
		var problem refusal
		n, problem = empfangenEigen(f, form, page)
		if !problem.ok() {
			return backTo(ziel, welches, "fehler", problem), nil
		}
	} else {
		n = message{
			Name:    strings.TrimSpace(form.Get(fieldName)),
			Email:   strings.TrimSpace(form.Get(fieldEmail)),
			Subject: strings.TrimSpace(form.Get(fieldSubject)),
			Text:    strings.TrimSpace(form.Get(fieldText)),
			Page:    page,
		}
		if problem := check(n); !problem.ok() {
			return back(ziel, "fehler", problem), nil
		}
	}

	// The consent, checked here for both kinds of form.
	//
	// Read from the store and not from the submission: what the sender ticked
	// is what the form showed them, and a POST that carries its own sentence
	// would let anybody write their own consent and have it stored as the
	// visitor's.
	// The sentence in the language the form was drawn in, which is the one the
	// sender actually read. Storing any other with the message would record a
	// consent to words nobody was shown.
	if wording := consentText(sprache); wording != "" {
		if strings.TrimSpace(form.Get(fieldConsent)) == "" {
			return backTo(ziel, welches, "fehler",
				refusal{Field: fieldConsent, Code: "consent-missing"}), nil
		}
		n.Consent = wording
		n.ConsentTime = time.Now().UTC().Format(time.RFC3339)
	}

	// A file the host would not take is said so before anything else is done
	// with the submission: the sender has to be able to send it again with a
	// smaller picture, and telling them after the message was stored would mean
	// an enquiry with half its attachments and nobody the wiser.
	for _, f := range in.Files {
		if f.Refused != "" {
			return backTo(ziel, welches, "fehler",
				refusal{Field: fieldAttach, Code: "attach-refused", Arg: f.Refused}), nil
		}
	}

	if !roomThisHour() {
		return backTo(ziel, welches, "fehler", refusal{Code: "too-many"}), nil
	}

	// Only here. Every trap has been past, the message is real, and this is the
	// first moment a stranger's file is allowed to reach the disk.
	if len(in.Files) > 0 {
		kept, err := plugin.KeepFiles()
		if err != nil {
			plugin.Logf("error", "the attachments could not be kept: %v", err)
		}
		for _, k := range kept {
			n.Attachments = append(n.Attachments, attachment{
				MediaID: k.MediaID, Name: k.Name, Filename: k.Filename,
			})
		}
	}

	if err := speichern(n); err != nil {
		return plugin.RequestOut{}, err
	}
	notify(n)
	return backTo(ziel, welches, "gesendet", refusal{}), nil
}

// notify tells the operator when they have set that up.
//
// After storing and not before: an enquiry that stands in the admin has
// arrived — whether the notification gets through changes nothing about that.
// A failure here must never reach the visitor; for them the message is sent,
// and that is true.
//
// The recipient is in the website's settings, not here. The plugin cannot name
// an address, and that is the reason it may be given this permission at all.
func notify(n message) {
	subject := n.Subject
	if subject == "" {
		subject = plugin.T("New enquiry")
	}
	if n.FormName != "" && !strings.Contains(subject, n.FormName) {
		subject = n.FormName + ": " + subject
	}
	from := plugin.T("the home page")
	if n.Page != "" {
		from = "/" + n.Page
	}

	// One letter, one catalogue entry. Assembling it from six translated lines
	// would let a language reorder the lines and not the letter, and whoever
	// translates it would be looking at six fragments with no way to see how
	// they sit together.
	text := plugin.Tf(`An enquiry has come in through the form on %s.

From:     %s <%s>
Subject:  %s
At:       %s

%s

--
Replying to this message goes straight to the sender.
`, from, n.Name, n.Email, subject, shortDate(n.Time), n.Text)

	// What the person who wrote reads. Their own words come back, because a
	// receipt that does not say what was received is worth very little — and
	// because it is the copy they will look for when they want to know what
	// they actually asked.
	confirmSubject := plugin.Tf("Your enquiry: %s", subject)
	confirmText := plugin.Tf(`Thank you — your message has arrived and we will be in touch.

This is what you sent us:

%s

--
This is an automatic receipt. Replying to it reaches us.
`, n.Text)

	// The sender's address as the reply address: then replying is one click and
	// not a switch to the admin, copy, paste. The same address is where the
	// receipt goes — the host allows one copy and only to that address, which
	// is what stops this being a mail relay. See PermConfirm.
	queued, confirmed, reason, err := plugin.NotifyAndConfirm(
		subject, text, n.Email, confirmSubject, confirmText)
	switch {
	case err != nil:
		plugin.Logf("error", "notification failed: %v", err)
	case !queued && reason != "":
		// Not an error: no mail server or no address is a decision and not a
		// mishap. Once at debug, so that somebody searching finds it.
		plugin.Logf("debug", "no notification sent: %s", reason)
	case queued && !confirmed:
		// Also not an error, and worth a line: the operator has not switched
		// receipts on, or the plugin was installed without the permission.
		plugin.Log("debug", "no receipt sent to the sender")
	}
}

func back(path, stand string, why refusal) plugin.RequestOut {
	return backTo(path, "", stand, why)
}

// backTo is the same but also says which form is meant — on a page with two
// forms the message would otherwise stand under both.
func backTo(path, welches, stand string, why refusal) plugin.RequestOut {
	ziel := path
	if ziel == "" {
		ziel = "/"
	}
	q := url.Values{}
	q.Set("formular", stand)
	if welches != "" {
		q.Set("welches", welches)
	}
	if !why.ok() {
		q.Set("grund", why.Code)
		if why.Field != "" {
			q.Set("feld", why.Field)
		}
		if why.Arg != "" {
			q.Set("wort", why.Arg)
		}
	}
	return plugin.RequestOut{
		Handled: true,
		// 303: the browser has to follow with GET, or a reload sends the
		// message a second time.
		Status:   303,
		Location: ziel + "?" + q.Encode(),
	}
}

// pageName cleans the page reference the form brought along.
//
// It comes out of the request and lands in a Location header. Everything that
// could leave the website or append a header is discarded and not escaped —
// then the worst outcome is a redirect to the start page.
func pageName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 200 {
		return ""
	}
	for _, r := range raw {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return ""
		}
	}
	return raw
}

// pagePath cleans the address the form brought along.
//
// It comes out of the request and lands in a Location header, so it is checked
// against what an address on this website can look like and nothing wider: a
// leading slash, then segments of the same narrow alphabet a slug has, which
// is also what a language prefix is made of. No scheme, no host, no "//" that
// a browser reads as a host, no "..", no percent sign, nothing to append a
// header with. Whatever fails falls back to the start page, which is the same
// worst outcome pageName already had.
func pagePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 200 || raw[0] != '/' {
		return ""
	}
	if raw == "/" {
		return raw
	}
	for _, teil := range strings.Split(strings.TrimSuffix(raw[1:], "/"), "/") {
		if teil == "" {
			return ""
		}
		for _, r := range teil {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return ""
			}
		}
	}
	return raw
}

// refusal is what a submission was turned away for.
//
// A field and a code, never a finished sentence. The sentence is made where the
// form is drawn, which has three consequences and all three are the point:
//
//   - The message can stand AT the field it is about, which one string handed
//     back through the address bar can never do.
//   - The set of things the address bar may say is closed. Until now the
//     sentence itself travelled in ?hinweis=, escaped and bounded to 160
//     characters — so a crafted link could put a stranger's 160 characters into
//     the operator's own layout, which is how a contact form becomes a phishing
//     page. A code the table below does not know now yields the ordinary
//     "please check what you entered".
//   - The sentence is translated when the page is drawn, in the language of
//     that page, rather than at the moment the submission was refused.
type refusal struct {
	// Field is the form field the message belongs under, empty when the message
	// is about the submission as a whole.
	Field string
	// Code names the reason. Nobody reads it.
	Code string
	// Arg fills the one %s a few of the sentences have. It is the operator's
	// own field label and never a sentence: what travels is a word they typed
	// into their own form, bounded by the editor to 120 characters, and the
	// sentence around it comes from the table.
	Arg string
}

// ok reports that there is nothing to say.
func (r refusal) ok() bool { return r.Code == "" }

// reasons maps a code to the sentence a visitor reads.
//
// The closed set. N marks each sentence for the catalogue; T translates it in
// formData, where the language of the page is known.
var reasons = map[string]string{
	"name-missing":    plugin.N("Please enter your name."),
	"name-long":       plugin.N("The name is too long."),
	"email-missing":   plugin.N("Please enter an e-mail address so that we can answer."),
	"email-shape":     plugin.N("The e-mail address does not look right."),
	"email-long":      plugin.N("The e-mail address is too long."),
	"subject-long":    plugin.N("The subject is too long."),
	"text-missing":    plugin.N("Please write a message as well."),
	"text-long":       plugin.N("The message is too long. Please keep it a little shorter."),
	"unreadable":      plugin.N("The form could not be read."),
	"expired":         plugin.N("The form was open for too long. Please reload the page and send again."),
	"gone":            plugin.N("This form no longer exists. Please reload the page."),
	"too-many":        plugin.N("A great many messages have just come in. Please try again in an hour."),
	"form-incomplete": plugin.N("Please fill in the form."),
	"consent-missing": plugin.N("Please agree before sending."),
	"attach-refused":  plugin.N("The attachment was not accepted: %s"),
	// The five below are about one field of an assembled form and carry its
	// label in their %s.
	"field-tick":  plugin.N("Please tick “%s”."),
	"field-fill":  plugin.N("Please fill in “%s”."),
	"field-long":  plugin.N("“%s” is too long."),
	"field-email": plugin.N("The address in “%s” does not look right."),
	"field-digit": plugin.N("“%s” has to be a number."),
	"field-date":  plugin.N("“%s” has to be a date."),
	"field-pick":  plugin.N("Please choose one of the offered values for “%s”."),
}

// check returns what is missing from a submission first.
//
// The sentences are for the visitor, so plain language that says what to do —
// an enquiry turned away with "validation error" is a lost enquiry.
func check(n message) refusal {
	switch {
	case n.Name == "":
		return refusal{Field: fieldName, Code: "name-missing"}
	case len([]rune(n.Name)) > maxName:
		return refusal{Field: fieldName, Code: "name-long"}
	case n.Email == "":
		return refusal{Field: fieldEmail, Code: "email-missing"}
	case !plausibleAddress(n.Email):
		return refusal{Field: fieldEmail, Code: "email-shape"}
	case len(n.Email) > maxEmail:
		return refusal{Field: fieldEmail, Code: "email-long"}
	case len([]rune(n.Subject)) > maxBetreff:
		return refusal{Field: fieldSubject, Code: "subject-long"}
	case n.Text == "":
		return refusal{Field: fieldText, Code: "text-missing"}
	case len([]rune(n.Text)) > maxText:
		return refusal{Field: fieldText, Code: "text-long"}
	}
	return refusal{}
}

// plausibleAddress is a check of shape, not a judgement. Stricter would refuse
// real addresses; anything that promises more does not keep it.
func plausibleAddress(s string) bool {
	name, wirt, ok := strings.Cut(s, "@")
	if !ok || name == "" || wirt == "" {
		return false
	}
	if strings.Contains(wirt, "@") || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	punkt := strings.LastIndex(wirt, ".")
	return punkt > 0 && punkt < len(wirt)-1
}

// roomThisHour counts along and says no when the hour is full.
func roomThisHour() bool {
	key := praefixZaehler + time.Now().UTC().Format("2006-01-02T15")
	n := 0
	if raw, ok, _ := plugin.Get(key); ok {
		n, _ = strconv.Atoi(raw)
	}
	if n >= hourlyLimit {
		return false
	}
	_ = plugin.Set(key, strconv.Itoa(n+1))
	dropOldCounters(key)
	return true
}

// dropOldCounters sweeps away the counters of past hours. They are never read
// again, and without this the storage would grow by one row per hour.
func dropOldCounters(aktuell string) {
	alle, err := plugin.List(praefixZaehler, 100)
	if err != nil {
		return
	}
	for k := range alle {
		if k != aktuell {
			_ = plugin.Delete(k)
		}
	}
}

func speichern(n message) error {
	n.Time = time.Now().UTC().Format(time.RFC3339)
	n.Key = n.Time + "-" + zufallsende()

	raw, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if err := plugin.Set(prefixMessage+n.Key, string(raw)); err != nil {
		return err
	}
	sweep()
	return nil
}

// zufallsende trennt zwei Nachrichten aus derselben Sekunde.
func zufallsende() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// sweep throws away the oldest messages when there are too many.
func sweep() {
	alle, err := plugin.List(prefixMessage, 1000)
	if err != nil || len(alle) <= maxMessages {
		return
	}
	keys := make([]string, 0, len(alle))
	for k := range alle {
		keys = append(keys, k)
	}
	// The key starts with the timestamp, so it sorts chronologically.
	sort.Strings(keys)

	// Only what somebody has read.
	//
	// This used to delete the oldest, full stop, with nothing but a log line to
	// say so. This plugin's own migration 0001 writes the reason it must not:
	// "an enquiry somebody made that nobody reads is a lost enquiry". An unread
	// message is precisely the one that has not been dealt with, so it is
	// precisely the one that may not fall.
	//
	// The consequence is that the bound can be exceeded, by an operator who
	// does not read their messages. That is the right way round: a full store
	// is a problem the operator can see and fix, and a deleted enquiry is one
	// nobody can.
	over := len(keys) - maxMessages
	removed, kept := 0, 0
	for _, key := range keys {
		if removed >= over {
			break
		}
		raw, ok, err := plugin.Get(key)
		if err != nil {
			continue
		}
		if ok {
			var n message
			if err := json.Unmarshal([]byte(raw), &n); err == nil && !n.Read {
				kept++
				continue
			}
		}
		if err := plugin.Delete(key); err == nil {
			removed++
		}
	}
	plugin.Logf("info", "%d old messages removed, %d kept because nobody has read them",
		removed, kept)
	if kept > 0 && removed < over {
		// Said once and loudly: the store is over its bound and staying there
		// until somebody reads what is in it.
		plugin.Logf("warn",
			"%d messages are stored, %d above the limit of %d — %d of the oldest are unread and were not removed",
			len(keys), len(keys)-maxMessages, maxMessages, kept)
	}
}

// --- die Verwaltung ---------------------------------------------------------

func screen(in plugin.AdminIn) (plugin.AdminOut, error) {
	if in.Method == "POST" {
		switch {
		case len(in.Form["loeschen"]) > 0:
			_ = plugin.Delete(prefixMessage + in.Form["loeschen"][0])
			return plugin.AdminOut{Redirect: ".", Flash: plugin.T("Message deleted.")}, nil
		case len(in.Form["gelesen"]) > 0:
			mark(in.Form["gelesen"][0], true)
			return plugin.AdminOut{Redirect: "."}, nil
		case len(in.Form["ungelesen"]) > 0:
			mark(in.Form["ungelesen"][0], false)
			return plugin.AdminOut{Redirect: "."}, nil
		case len(in.Form["antworten"]) > 0:
			return reply(in.Form["antworten"][0], firstValue(in.Form, "antwort"))

		}
	}

	alle, err := plugin.List(prefixMessage, 1000)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	liste := lesen(alle)
	// Newest first: whoever opens the screen is almost always looking for the last one.
	sort.Slice(liste, func(i, j int) bool { return liste[i].Key > liste[j].Key })

	// The table to download. It sits behind the same address with ?ansicht=csv,
	// because a plugin has only one screen and the query is the only thing it
	// has to tell views apart with.
	if strings.Contains(in.Query, "ansicht=csv") {
		if len(liste) == 0 {
			return plugin.AdminOut{Redirect: ".", Flash: plugin.T("There is nothing to output yet."), FlashError: true}, nil
		}
		return csvAusgabe(liste)
	}

	var b strings.Builder
	b.WriteString(navigation(ansichtNachrichten))
	ungelesen := 0
	for _, n := range liste {
		if !n.Read {
			ungelesen++
		}
	}

	if len(liste) == 0 {
		fmt.Fprintf(&b, `<p class="empty">%s</p>`, plugin.T("No message has come in yet. "+
			"Put <code>[[formular]]</code> into a page and the form stands there."))
		return plugin.AdminOut{Title: plugin.T("Messages"), HTML: b.String()}, nil
	}

	// One whole sentence in the catalogue, both numbers inside it: a language
	// that counts differently or orders the clauses the other way round can say
	// so, which it could not if this were pieces joined together.
	fmt.Fprintf(&b, `<p>%s `+
		`<a class="btn btn--sm" href="?ansicht=csv">%s</a></p>`,
		escape(plugin.Tf("%d messages, %d of them unread.", len(liste), ungelesen)),
		escape(plugin.T("Download as a table")))

	for _, n := range liste {
		class := "card"
		if !n.Read {
			class += " card--unread"
		}
		fmt.Fprintf(&b, `<article class="%s">`, class)
		fmt.Fprintf(&b, `<h3>%s</h3>`, escape(betreffOder(n)))
		fmt.Fprintf(&b, `<p class="text-muted">%s &lt;<a href="mailto:%s">%s</a>&gt; · %s`,
			escape(n.Name), escape(n.Email), escape(n.Email), escape(shortDate(n.Time)))
		if n.Page != "" {
			fmt.Fprintf(&b, ` · %s`, plugin.Tf("from <code>/%s</code>", escape(n.Page)))
		}
		if n.FormName != "" {
			fmt.Fprintf(&b, ` · %s`, escape(n.FormName))
		}
		b.WriteString(`</p>`)
		if len(n.Fields) > 0 {
			// An assembled form has named answers. As a table rather than as
			// flowing text: somebody going through twenty enquiries is always
			// looking for the same field, and in a column they find it.
			b.WriteString(`<table class="table"><tbody>`)
			for _, a := range n.Fields {
				fmt.Fprintf(&b, `<tr><th scope="row">%s</th><td>%s</td></tr>`,
					escape(a.Label), escape(a.Value))
			}
			b.WriteString(`</tbody></table>`)
		} else {
			// The text comes from outside. It is printed escaped, and the line
			// breaks are made by the stylesheet, not by inserted markup.
			fmt.Fprintf(&b, `<pre class="message-body">%s</pre>`, escape(n.Text))
		}

		if len(n.Attachments) > 0 {
			b.WriteString(`<p class="table-actions">`)
			for _, a := range n.Attachments {
				// The media route is the host's, and it is the only way to
				// reach the file: the plugin has an id and nothing else.
				fmt.Fprintf(&b, `<a class="btn btn--sm" href="/media/%d/%s" download>%s</a> `,
					in.WebsiteID, escape(a.Filename), escape(a.Name))
			}
			b.WriteString(`</p>`)
		}

		// What they agreed to, exactly as it stood beside the box. Not
		// "consent given" — that is a claim, and this is the evidence.
		if n.Consent != "" {
			// The sentence is written as it STOOD, markup and all, not escaped
			// into <a href="…">. The record is meant to answer "what did this
			// person see and agree to", and a record nobody can read does not
			// answer it. It is the operator's own text either way — the same
			// text the public form already renders.
			fmt.Fprintf(&b, `<p class="text-muted">%s</p>`,
				plugin.Tf("Agreed on %s: “%s”", escape(shortDate(n.ConsentTime)), n.Consent))
		}

		// What was already answered, and when. It stands above the box so that
		// somebody opening the screen sees the answer before they write a
		// second one.
		if n.Reply != "" {
			fmt.Fprintf(&b, `<div class="card card--reply"><p class="text-muted">%s</p><pre class="message-body">%s</pre></div>`,
				escape(plugin.Tf("Answered on %s", shortDate(n.ReplyTime))), escape(n.Reply))
		}

		if n.Email != "" {
			// A form of its own, because a <form> inside a <form> is invalid
			// HTML and the browser throws the inner one away — the same reason
			// the buttons below each carry their own.
			fmt.Fprintf(&b, `<form method="POST" class="stack">`+
				`<input type="hidden" name="antworten" value="%s">`+
				`<label for="a-%s">%s</label>`+
				`<textarea id="a-%s" name="antwort" rows="4" maxlength="%d" `+
				`placeholder="%s"></textarea>`+
				`<p><button type="submit" class="btn btn--sm btn--primary">%s</button></p>`+
				`</form>`,
				escape(n.Key), escape(n.Key),
				escape(plugin.Tf("Answer %s", n.Name)),
				escape(n.Key), maxText,
				escape(plugin.T("Your answer goes to the address above.")),
				escape(plugin.T("Send answer")))
		}

		b.WriteString(`<p class="table-actions">`)
		if n.Read {
			button(&b, "ungelesen", n.Key, plugin.T("Mark as unread"), "")
		} else {
			button(&b, "gelesen", n.Key, plugin.T("Mark as read"), "")
		}
		button(&b, "loeschen", n.Key, plugin.T("Delete"), "btn--danger")
		b.WriteString(`</p></article>`)
	}

	return plugin.AdminOut{Title: plugin.T("Messages"), HTML: b.String()}, nil
}

// button draws a form with one button in it. It posts back to the same address;
// the host inserts the session key, and the plugin never sees it. actionButton
// is a button without a form of its own: it submits the one it stands in. A
// <form> inside a <form> is invalid HTML, and a browser throws the inner one
// away — the button would then do nothing at all.
func actionButton(b *strings.Builder, name, value, label, class string) {
	fmt.Fprintf(b, `<button type="submit" name="%s" value="%s" class="btn btn--sm %s">%s</button> `,
		name, escape(value), class, label)
}

func button(b *strings.Builder, name, value, label, class string) {
	fmt.Fprintf(b, `<form method="POST" class="inline-form">`+
		`<input type="hidden" name="%s" value="%s">`+
		`<button type="submit" class="btn btn--sm %s">%s</button></form>`,
		name, escape(value), class, label)
}

func mark(key string, read bool) {
	storeKey := prefixMessage + key
	raw, ok, _ := plugin.Get(storeKey)
	if !ok {
		return
	}
	var n message
	if json.Unmarshal([]byte(raw), &n) != nil {
		return
	}
	n.Read = read
	if updated, err := json.Marshal(n); err == nil {
		_ = plugin.Set(storeKey, string(updated))
	}
}

func lesen(raw map[string]string) []message {
	out := make([]message, 0, len(raw))
	for _, v := range raw {
		var n message
		if json.Unmarshal([]byte(v), &n) == nil && n.Key != "" {
			out = append(out, n)
		}
	}
	return out
}

func betreffOder(n message) string {
	if n.Subject != "" {
		return n.Subject
	}
	return plugin.T("No subject")
}

func shortDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("02.01.2006 15:04")
}

func escape(s string) string { return html.EscapeString(s) }

func main() {}

// reply answers one enquiry by e-mail and keeps the answer with it.
//
// The address is the sender's, and it is the address stored with the message —
// not one that came in with the form just now. That distinction is the whole
// safety of this: an operator can answer the person who wrote and nobody else,
// and a crafted POST cannot turn the reply screen into a way to send mail to a
// third party.
func reply(key, text string) (plugin.AdminOut, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return plugin.AdminOut{Redirect: ".",
			Flash: plugin.T("An empty answer is not sent."), FlashError: true}, nil
	}
	if len([]rune(text)) > maxText {
		return plugin.AdminOut{Redirect: ".",
			Flash: plugin.T("The answer is too long."), FlashError: true}, nil
	}

	raw, ok, err := plugin.Get(prefixMessage + key)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	if !ok {
		return plugin.AdminOut{Redirect: ".",
			Flash: plugin.T("This message no longer exists."), FlashError: true}, nil
	}
	var n message
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return plugin.AdminOut{}, err
	}
	if n.Email == "" {
		return plugin.AdminOut{Redirect: ".",
			Flash: plugin.T("This message carries no address to answer."), FlashError: true}, nil
	}

	// Notify goes to the operator; the copy goes to the sender. Here the copy
	// IS the message, and the operator's own goes out as the record that an
	// answer was sent — which is also what makes this work through one
	// permitted path instead of a second way to send mail.
	subject := plugin.Tf("Re: %s", betreffOder(n))
	body := plugin.Tf(`%s

--
This is the answer to your enquiry of %s.

> %s
`, text, shortDate(n.Time), n.Text)

	queued, confirmed, reason, err := plugin.NotifyAndConfirm(
		plugin.Tf("Answered: %s", betreffOder(n)),
		plugin.Tf("An answer went to %s <%s>:\n\n%s\n", n.Name, n.Email, text),
		n.Email, subject, body)
	if err != nil {
		return plugin.AdminOut{}, err
	}

	// Stored whatever the mail did. An answer the operator wrote is a record of
	// what they said, and losing it because a mail server was down would be the
	// worse of the two failures.
	//
	// Written back under the SAME key, not through speichern: that function
	// mints a fresh key and a fresh timestamp for a message that has just come
	// in, and using it here would leave the original standing and put a second
	// copy of the enquiry beside it, dated today.
	n.Reply = text
	n.ReplyTime = time.Now().UTC().Format(time.RFC3339)
	n.Read = true
	updated, err := json.Marshal(n)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	if err := plugin.Set(prefixMessage+key, string(updated)); err != nil {
		return plugin.AdminOut{}, err
	}

	switch {
	case confirmed:
		return plugin.AdminOut{Redirect: ".", Flash: plugin.T("Answer sent.")}, nil
	case !queued && reason != "":
		return plugin.AdminOut{Redirect: ".", FlashError: true,
			Flash: plugin.T("The answer is stored with the message, and no mail went out: no mail server is set up.")}, nil
	default:
		return plugin.AdminOut{Redirect: ".", FlashError: true,
			Flash: plugin.T("The answer is stored with the message, and no mail went out. Switch “Tell the sender that their message arrived” on in the website settings.")}, nil
	}
}

// --- die Quarantaene --------------------------------------------------------

// The submissions that were turned away without being told.
//
// The honeypot and the time trap answer exactly like a success, because a robot
// that learns which of its submissions were refused learns how to get past the
// filter. The cost of that is a visitor who is refused wrongly and never finds
// out — and neither does the operator, because the submission went nowhere.
// Somebody whose browser fills in every field it finds, or who has a clock an
// hour out, simply stops being able to write to this website.
//
// So a refused submission is kept, with the reason, and the operator can see
// it. What is kept is bounded and what falls out of it falls silently: unlike
// an enquiry, nobody has promised to read these, and what matters is the recent
// false positive rather than a year of robots.
const (
	prefixQuarantine = "abgewiesen:"
	maxQuarantine    = 50
)

// quarantineReasons names what turned a submission away, for the screen.
var quarantineReasons = map[string]string{
	"honeypot":   plugin.N("The hidden field was filled in"),
	"zu schnell": plugin.N("Sent less than three seconds after the page was drawn"),
	"gefaelscht": plugin.N("The timestamp was missing or does not match"),
	"abgelaufen": plugin.N("The form had been open for more than twelve hours"),
}

// quarantine keeps a refused submission so that a wrong refusal is visible.
//
// It stores what was typed and not a message: a submission that never became
// one has no key, no subject and possibly no address, and pretending otherwise
// would put half-built messages into the list the operator reads.
func quarantine(form url.Values, page, reason string) {
	n := message{
		Name:    clip(form.Get(fieldName), maxName),
		Email:   clip(form.Get(fieldEmail), maxEmail),
		Subject: clip(form.Get(fieldSubject), maxBetreff),
		Text:    clip(form.Get(fieldText), maxText),
		Page:    page,
		Form:    strings.TrimSpace(form.Get(fieldForm)),
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
	n.Key = n.Time + "-" + zufallsende()
	n.RefusedFor = reason

	raw, err := json.Marshal(n)
	if err != nil {
		return
	}
	if err := plugin.Set(prefixQuarantine+n.Key, string(raw)); err != nil {
		return
	}
	sweepQuarantine()
}

// clip bounds one field on its way into the quarantine.
//
// The same limits the real form applies, said again here because these values
// did NOT go through check(): they were refused before it, and a robot is
// exactly the sort of caller that sends a megabyte.
func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}

// sweepQuarantine keeps the newest and lets the rest go.
//
// Unlike sweep() for real messages this does not spare the unread, and the
// difference is the point: an enquiry nobody has read is one nobody has dealt
// with, while a refused submission nobody has read is almost always a robot.
func sweepQuarantine() {
	alle, err := plugin.List(prefixQuarantine, 200)
	if err != nil || len(alle) <= maxQuarantine {
		return
	}
	keys := make([]string, 0, len(alle))
	for k := range alle {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i := 0; i < len(keys)-maxQuarantine; i++ {
		_ = plugin.Delete(keys[i])
	}
}

// release moves a refused submission into the messages.
//
// For the false positive: the operator reads it, sees a real person, and the
// enquiry becomes an enquiry. It gets a fresh key from speichern, so it sorts
// where it is being read rather than where it was refused — and it does NOT
// notify, because the moment has passed and a notification about a message the
// operator is looking at is noise.
func release(key string) (plugin.AdminOut, error) {
	raw, ok, err := plugin.Get(prefixQuarantine + key)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	if !ok {
		return plugin.AdminOut{Redirect: "?ansicht=abgewiesen",
			Flash: plugin.T("This submission no longer exists."), FlashError: true}, nil
	}
	var n message
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return plugin.AdminOut{}, err
	}
	n.RefusedFor = ""
	if err := speichern(n); err != nil {
		return plugin.AdminOut{}, err
	}
	_ = plugin.Delete(prefixQuarantine + key)
	return plugin.AdminOut{Redirect: "?ansicht=abgewiesen",
		Flash: plugin.T("Moved into the messages.")}, nil
}

// consentText is the sentence the operator asks visitors to agree to, or empty.
// consentText is the sentence a visitor of THIS page has to agree to.
//
// The operator writes one per language the website publishes in. A language
// they left empty falls back to the one they wrote first, which is better than
// an empty tick box: a form that silently stops asking for consent because a
// language was added is the failure that costs something.
//
// Falling back is not the same as being right, and the admin screen says which
// languages are still missing.
func consentText(lang string) string {
	if lang != "" {
		if raw, ok, err := plugin.Get(consentKey(lang)); err == nil && ok {
			if text := clip(raw, maxConsent); text != "" {
				return text
			}
		}
	}
	raw, ok, err := plugin.Get(keyConsent)
	if err != nil || !ok {
		return ""
	}
	return clip(raw, maxConsent)
}

// knownLocale keeps only a language this website actually publishes in.
func knownLocale(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 12 {
		return ""
	}
	for _, l := range siteLocales() {
		if l.Tag == raw {
			return raw
		}
	}
	return ""
}

// consentTextExact reads one language's sentence without the fallback, which is
// what the admin screen needs: a box that showed the fallback would be saved
// back as that language's own sentence the next time somebody pressed Save.
func consentTextExact(lang string) string {
	raw, ok, err := plugin.Get(consentKey(lang))
	if err != nil || !ok {
		return ""
	}
	return clip(raw, maxConsent)
}

// consentKey is where one language's sentence is kept. The bare key stays what
// it always was, so a website that was set up before this existed loses
// nothing and needs no migration.
func consentKey(lang string) string {
	if lang == "" {
		return keyConsent
	}
	return keyConsent + ":" + lang
}

// siteLocales are the languages this website publishes in, the first one first.
//
// Empty when the settings cannot be read, which is the same as a website with
// one language: then there is one sentence and no list.
func siteLocales() []plugin.Locale {
	s, err := plugin.Site()
	if err != nil {
		return nil
	}
	return s.Locales
}

// attachWanted reports whether the operator asked for a file field.
func attachWanted() bool {
	raw, ok, err := plugin.Get(keyAttach)
	return err == nil && ok && raw == "1"
}
