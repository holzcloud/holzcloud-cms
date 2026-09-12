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
	fieldName     = "name"
	fieldEmail    = "email"
	fieldSubject  = "betreff"
	fieldText     = "nachricht"
	fieldPage     = "seite"
	fieldTime     = "gestellt"
	fieldHoneypot = "website"
	// fieldForm says on receipt which assembled form was submitted. Without it
	// the receiving side would have to guess from the field names.
	fieldForm = "formular"
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

const (
	prefixMessage  = "nachricht:"
	praefixZaehler = "zaehler:"
	schluesselName = "signaturschluessel"
)

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
	// Nothing typed: what a visitor entered is cached nowhere, and sending it
	// back through the address would mean writing somebody else's text into an
	// address that lands in the history and in every server log. A refused
	// submission therefore says what is missing; the browser still has the
	// entries when they go back.
	var values url.Values
	// The paragraph first, so that the <p> disappears along with its marker. In
	// one pass an empty <p></p> would be left behind.
	out := ersetzen(markerInParagraph, in.HTML, data, values)
	out = ersetzen(marker, out, data, values)
	return plugin.ContentOut{HTML: out, Changed: out != in.HTML}, nil
}

func ersetzen(re *regexp.Regexp, page string, d data, values url.Values) string {
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
					values = nil
				}
				return zeichnenEigen(f, eigen, values)
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
	// Form is the key of the form whose submission is being answered. Only that
	// one shows the message — on a page with two forms it would otherwise stand
	// twice.
	Form      string
	Timestamp string
	Kontakt   string
	Hint      string
	IsError   bool
	Name      string
	Email     string
	Subject   string
	Text      string
}

func formData(in plugin.ContentIn) data {
	d := data{Page: in.Slug, Timestamp: timestamp(time.Now())}
	if s, err := plugin.Site(); err == nil {
		d.Kontakt = s.ContactEmail
	}

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
		d.Hint = hintText(q.Get("hinweis"))
	}
	return d
}

// hintText bounds what the query string may bring onto the page.
//
// The output is escaped anyway; the limit is aimed at a link that puts a page
// of somebody else's text into a website's layout — which is how a plain
// contact form becomes a phishing page.
func hintText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len([]rune(raw)) > 160 {
		return "The message could not be sent. Please check what you entered."
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
	fmt.Fprintf(&b, `<form class="contact-form" method="POST" action="%s">`, submitAddress)
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldTime, e(d.Timestamp))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldPage, e(d.Page))

	if d.Hint != "" {
		class := "contact-form__notice"
		if d.IsError {
			class += " contact-form__notice--error"
		}
		fmt.Fprintf(&b, `<p class="%s" role="status">%s</p>`, class, e(d.Hint))
	}

	field := func(id, label, typ, name, value string, max int, extra string) {
		fmt.Fprintf(&b, `<div class="contact-form__field"><label for="%s">%s</label>`+
			`<input type="%s" id="%s" name="%s" maxlength="%d" value="%s" %s></div>`,
			id, label, typ, id, name, max, e(value), extra)
	}
	field("cf-name", "Name", "text", fieldName, d.Name, maxName, `required autocomplete="name"`)
	field("cf-email", "E-Mail", "email", fieldEmail, d.Email, maxEmail, `required autocomplete="email"`)
	field("cf-subject", "Betreff", "text", fieldSubject, d.Subject, maxBetreff, "")

	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="cf-body">Nachricht</label>`+
		`<textarea id="cf-body" name="%s" rows="8" required maxlength="%d">%s</textarea></div>`,
		fieldText, maxText, e(d.Text))

	// No visible field. The stylesheet hides it from people, aria-hidden and
	// tabindex from screen readers; whoever does not see it either way leaves
	// it alone. A program that fills in every input it finds fills this one in
	// too — which is exactly what it is for.
	fmt.Fprintf(&b, `<div class="contact-form__trap" aria-hidden="true">`+
		`<label for="cf-website">Website (bitte leer lassen)</label>`+
		`<input type="text" id="cf-website" name="%s" tabindex="-1" autocomplete="off"></div>`,
		fieldHoneypot)

	b.WriteString(`<button type="submit" class="contact-form__submit">Nachricht senden</button>`)
	if d.Kontakt != "" {
		fmt.Fprintf(&b, `<p class="contact-form__alternative">Lieber direkt schreiben? `+
			`<a href="mailto:%s">%s</a></p>`, e(d.Kontakt), e(d.Kontakt))
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
		return back("", "fehler", "The form could not be read."), nil
	}
	form, err := url.ParseQuery(in.Body)
	if err != nil {
		return back("", "fehler", "The form could not be read."), nil
	}
	page := pageName(form.Get(fieldPage))

	// The two traps, in the order that costs least. A filled honeypot and a
	// form that comes back in under three seconds are both answered exactly
	// like a success: a robot that learns which of its submissions were refused
	// learns how to get past the filter.
	if strings.TrimSpace(form.Get(fieldHoneypot)) != "" {
		plugin.Log("info", "honeypot triggered")
		return back(page, "gesendet", ""), nil
	}
	switch reason := checkTimestamp(form.Get(fieldTime), time.Now()); reason {
	case "":
	case "abgelaufen":
		// Somebody who had a tab open for a day deserves an answer and not
		// silence — the message is real and still stands in the field.
		return back(page, "fehler",
			"The form was open for too long. Please reload the page and send again."), nil
	default:
		plugin.Logf("info", "Absendung abgewiesen: %s", reason)
		return back(page, "gesendet", ""), nil
	}

	// An assembled form says which one it is itself. If the key there no longer
	// exists, the submission is refused rather than read as the built-in form:
	// the fields would not match, and an empty message would come out.
	welches := strings.TrimSpace(form.Get(fieldForm))
	var n message
	if welches != "" {
		f, ok := formularLaden(welches)
		if !ok {
			return backTo(page, welches, "fehler",
				"This form no longer exists. Please reload the page."), nil
		}
		var problem string
		n, problem = empfangenEigen(f, form, page)
		if problem != "" {
			return backTo(page, welches, "fehler", problem), nil
		}
	} else {
		n = message{
			Name:    strings.TrimSpace(form.Get(fieldName)),
			Email:   strings.TrimSpace(form.Get(fieldEmail)),
			Subject: strings.TrimSpace(form.Get(fieldSubject)),
			Text:    strings.TrimSpace(form.Get(fieldText)),
			Page:    page,
		}
		if problem := check(n); problem != "" {
			return back(page, "fehler", problem), nil
		}
	}

	if !roomThisHour() {
		return backTo(page, welches, "fehler",
			"A great many messages have just come in. Please try again in an hour."), nil
	}
	if err := speichern(n); err != nil {
		return plugin.RequestOut{}, err
	}
	notify(n)
	return backTo(page, welches, "gesendet", ""), nil
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
		subject = "Neue Anfrage"
	}
	if n.FormName != "" && !strings.Contains(subject, n.FormName) {
		subject = n.FormName + ": " + subject
	}
	from := n.Page
	if from == "" {
		from = "Startseite"
	} else {
		from = "/" + from
	}

	text := fmt.Sprintf(`An enquiry has come in through the form on %s.

From:     %s <%s>
Subject:  %s
At:       %s

%s

--
Replying to this message goes straight to the sender.
`, from, n.Name, n.Email, subject, shortDate(n.Time), n.Text)

	// The sender's address as the reply address: then replying is one click and
	// not a switch to the admin, copy, paste.
	queued, reason, err := plugin.Notify(subject, text, n.Email)
	switch {
	case err != nil:
		plugin.Logf("error", "Benachrichtigung fehlgeschlagen: %v", err)
	case !queued && reason != "":
		// Not an error: no mail server or no address is a decision and not a
		// mishap. Once at debug, so that somebody searching finds it.
		plugin.Logf("debug", "keine Benachrichtigung verschickt: %s", reason)
	}
}

// back sends the visitor back to the page and carries the outcome along in the
// address. Without JavaScript, and a reload does not send twice.
func back(page, stand, hint string) plugin.RequestOut {
	return backTo(page, "", stand, hint)
}

// backTo is the same but also says which form is meant — on a page with two
// forms the message would otherwise stand under both.
func backTo(page, welches, stand, hint string) plugin.RequestOut {
	ziel := "/"
	if page != "" {
		ziel = "/" + page
	}
	q := url.Values{}
	q.Set("formular", stand)
	if welches != "" {
		q.Set("welches", welches)
	}
	if hint != "" {
		q.Set("hinweis", hint)
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

// check returns what is missing from a submission first.
//
// The sentences are for the visitor, so plain language that says what to do —
// an enquiry turned away with "validation error" is a lost enquiry.
func check(n message) string {
	switch {
	case n.Name == "":
		return "Bitte trage deinen Namen ein."
	case len([]rune(n.Name)) > maxName:
		return "The name is too long."
	case n.Email == "":
		return "Please enter an e-mail address so that we can answer."
	case !plausibleAddress(n.Email):
		return "The e-mail address does not look right."
	case len(n.Email) > maxEmail:
		return "Die E-Mail-Adresse ist zu lang."
	case len([]rune(n.Subject)) > maxBetreff:
		return "The subject is too long."
	case n.Text == "":
		return "Please write a message as well."
	case len([]rune(n.Text)) > maxText:
		return "The message is too long. Please keep it a little shorter."
	}
	return ""
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
	for i := 0; i < len(keys)-maxMessages; i++ {
		_ = plugin.Delete(keys[i])
	}
	plugin.Logf("info", "%d alte Nachrichten entfernt", len(keys)-maxMessages)
}

// --- die Verwaltung ---------------------------------------------------------

func screen(in plugin.AdminIn) (plugin.AdminOut, error) {
	if in.Method == "POST" {
		switch {
		case len(in.Form["loeschen"]) > 0:
			_ = plugin.Delete(prefixMessage + in.Form["loeschen"][0])
			return plugin.AdminOut{Redirect: ".", Flash: "Message deleted."}, nil
		case len(in.Form["gelesen"]) > 0:
			mark(in.Form["gelesen"][0], true)
			return plugin.AdminOut{Redirect: "."}, nil
		case len(in.Form["ungelesen"]) > 0:
			mark(in.Form["ungelesen"][0], false)
			return plugin.AdminOut{Redirect: "."}, nil
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
			return plugin.AdminOut{Redirect: ".", Flash: "There is nothing to output yet.", FlashError: true}, nil
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
		b.WriteString(`<p class="empty">No message has come in yet. ` +
			`Put <code>[[formular]]</code> into a page and the form stands there.</p>`)
		return plugin.AdminOut{Title: "Nachrichten", HTML: b.String()}, nil
	}

	fmt.Fprintf(&b, `<p>%d Nachrichten, davon %d ungelesen. `+
		`<a class="btn btn--sm" href="?ansicht=csv">Als Tabelle herunterladen</a></p>`,
		len(liste), ungelesen)

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
			fmt.Fprintf(&b, ` · von <code>/%s</code>`, escape(n.Page))
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

		b.WriteString(`<p class="table-actions">`)
		if n.Read {
			button(&b, "ungelesen", n.Key, "Als ungelesen markieren", "")
		} else {
			button(&b, "gelesen", n.Key, "Als gelesen markieren", "")
		}
		button(&b, "loeschen", n.Key, "Delete", "btn--danger")
		b.WriteString(`</p></article>`)
	}

	return plugin.AdminOut{Title: "Nachrichten", HTML: b.String()}, nil
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
	return "Ohne Betreff"
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
