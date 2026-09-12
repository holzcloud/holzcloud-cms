// The farm shop as a plugin.
//
// An order form over the operator's own products — and a product is a page that
// has filled in a price field. So the whole thing stands on what the core has
// been able to do since it grew fields of its own, and brings along only what
// nobody needs without orders: the form, the orders and the screen they sit on.
//
// No basket. Somebody ordering from a farm picks what they want once and sends
// it off; a basket would need a session per visitor, a cookie and a second page,
// and all of that would have to be right before the first order arrived. A form
// does the job.
//
// No payment. A payment provider would be a call to the outside at run time —
// exactly the rule that lets this CMS do without a cookie banner. Payment is on
// handover or by invoice, and that stands as a note above the form.
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

// The form's field names. As constants, because they stand in two places — when
// drawing and when receiving — and would otherwise drift apart.
const (
	fieldName     = "name"
	fieldEmail    = "email"
	fieldPhone    = "telefon"
	fieldAddress  = "adresse"
	fieldRemark   = "bemerkung"
	fieldPageKey  = "seite"
	fieldTime     = "gestellt"
	fieldHoneypot = "website"
	// quantityPrefix + the page's address is a product's quantity field.
	quantityPrefix = "menge_"
)

// submitAddress is fixed and not the page's: then every page stays a plain GET,
// and the form works the same everywhere.
const submitAddress = "/bestellung"

// An order's limits. They keep a single submission from filling a memory card,
// and are wide enough for any real order.
const (
	maxName      = 120
	maxEmail     = 254
	maxTelefon   = 40
	maxAddress   = 400
	maxBemerkung = 2000
	maxQuantity  = 999
	maxPosten    = 40
)

// maxProStunde ist die Grenze gegen Sturzfluten.
const maxProStunde = 30

func init() {
	plugin.OnContent(formularEinsetzen)
	plugin.OnRoute(acceptOrder)
	plugin.OnAdmin(verwaltung)
}

// --- die Marke im Text ------------------------------------------------------

var token = regexp.MustCompile(`(?i)\[\[bestellung\]\]`)

// markerInParagraph matches the marker when it stands alone in a paragraph —
// which it does as soon as it is on a line of its own in Markdown. A form inside
// a <p> is invalid HTML that a browser silently reorders; the paragraph ends up
// in the middle of the form and the spacing sits wrong.
var markerInParagraph = regexp.MustCompile(`(?i)<p>\s*\[\[bestellung\]\]\s*</p>`)

// formularEinsetzen ersetzt [[bestellung]] durch das Formular.
func formularEinsetzen(in plugin.ContentIn) (plugin.ContentOut, error) {
	if !token.MatchString(in.HTML) {
		return plugin.ContentOut{}, nil
	}

	e := einstellungenLaden()
	produkte, err := readProducts(e)
	if err != nil {
		plugin.Logf("warn", "Produkte nicht lesbar: %v", err)
		fehltext := `<p>The product list is not available at the moment.</p>`
		out := markerInParagraph.ReplaceAllLiteralString(in.HTML, fehltext)
		return plugin.ContentOut{
			HTML:    token.ReplaceAllLiteralString(out, fehltext),
			Changed: true,
		}, nil
	}

	stand, note := stateFromQuery(in.Query)
	formular := draw(produkte, e, in.Slug, stand, note)
	// The paragraph first, so that the <p> disappears along with its marker. In
	// one pass an empty <p></p> would be left behind.
	out := markerInParagraph.ReplaceAllLiteralString(in.HTML, formular)
	out = token.ReplaceAllLiteralString(out, formular)
	return plugin.ContentOut{HTML: out, Changed: true}, nil
}

// stateFromQuery reads what came back from the last submission.
func stateFromQuery(query string) (stand, note string) {
	q, err := url.ParseQuery(query)
	if err != nil {
		return "", ""
	}
	return q.Get("bestellung"), q.Get("hinweis")
}

// draw builds the form.
//
// Assembled by hand rather than with a template: the result goes through the
// host's filter, and every value coming from outside is escaped individually
// here. A template with automatic escaping does not come free in a WASM module,
// and half a one would be worse than none.
func draw(produkte []product, e settings, page, stand, note string) string {
	var b strings.Builder
	b.WriteString(`<div class="bestellung">`)

	switch stand {
	case "gesendet":
		b.WriteString(`<p class="bestellung__ok" role="status">Danke! Deine Bestellung ist angekommen. ` +
			`Wir melden uns bei dir.</p>`)
		b.WriteString(`</div>`)
		return b.String()
	case "fehler":
		text := note
		if text == "" {
			text = "The order could not be accepted."
		}
		b.WriteString(`<p class="bestellung__fehler" role="alert">` + html.EscapeString(text) + `</p>`)
	}

	orderable := 0
	for _, p := range produkte {
		if p.Orderable {
			orderable++
		}
	}
	if orderable == 0 {
		b.WriteString(`<p>There is nothing to order at the moment.</p></div>`)
		return b.String()
	}

	if e.Hint != "" {
		b.WriteString(`<p class="bestellung__hinweis">` + html.EscapeString(e.Hint) + `</p>`)
	}

	b.WriteString(`<form class="bestellung__form" method="POST" action="` + submitAddress + `">`)
	b.WriteString(`<input type="hidden" name="` + fieldPageKey + `" value="` + html.EscapeString(page) + `">`)
	b.WriteString(`<input type="hidden" name="` + fieldTime + `" value="` + html.EscapeString(timestamp()) + `">`)
	// The honeypot: a field that looks like one and is not. Hidden with CSS
	// only, so that a screen reader can skip it and a form filler in the
	// browser writes nothing into it.
	b.WriteString(`<p class="bestellung__falle" aria-hidden="true">` +
		`<label>Website<input type="text" name="` + fieldHoneypot + `" tabindex="-1" autocomplete="off"></label></p>`)

	b.WriteString(`<table class="bestellung__tabelle"><thead><tr>` +
		`<th scope="col">Produkt</th><th scope="col">Preis</th><th scope="col">Menge</th>` +
		`</tr></thead><tbody>`)
	for _, p := range produkte {
		b.WriteString(`<tr>`)
		b.WriteString(`<th scope="row"><a href="/` + html.EscapeString(p.Slug) + `">` +
			html.EscapeString(p.Titel) + `</a>`)
		if !p.Orderable && p.Status != "" {
			b.WriteString(` <span class="bestellung__aus">` + html.EscapeString(p.Status) + `</span>`)
		}
		b.WriteString(`</th>`)

		b.WriteString(`<td>` + html.EscapeString(e.Currency) + ` ` + html.EscapeString(p.Price))
		if p.Einheit != "" {
			b.WriteString(` <span class="bestellung__einheit">/ ` + html.EscapeString(p.Einheit) + `</span>`)
		}
		b.WriteString(`</td>`)

		b.WriteString(`<td>`)
		if p.Orderable {
			name := quantityPrefix + p.Slug
			b.WriteString(`<label class="sr-only" for="` + html.EscapeString(name) + `">Menge ` +
				html.EscapeString(p.Titel) + `</label>`)
			b.WriteString(`<input type="number" inputmode="numeric" min="0" max="` +
				strconv.Itoa(maxQuantity) + `" step="1" value="" id="` + html.EscapeString(name) +
				`" name="` + html.EscapeString(name) + `">`)
		} else {
			b.WriteString(`<span class="bestellung__aus">nicht bestellbar</span>`)
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)

	field := func(name, label, art string, required bool, hilfe string) {
		b.WriteString(`<p class="bestellung__feld"><label for="b_` + name + `">` +
			html.EscapeString(label))
		if required {
			b.WriteString(` <span aria-hidden="true">*</span>`)
		}
		b.WriteString(`</label>`)
		if art == "textarea" {
			b.WriteString(`<textarea id="b_` + name + `" name="` + name + `" rows="3"></textarea>`)
		} else {
			b.WriteString(`<input type="` + art + `" id="b_` + name + `" name="` + name + `"`)
			if required {
				b.WriteString(` required`)
			}
			b.WriteString(`>`)
		}
		if hilfe != "" {
			b.WriteString(`<span class="bestellung__hilfe">` + html.EscapeString(hilfe) + `</span>`)
		}
		b.WriteString(`</p>`)
	}

	field(fieldName, "Name", "text", true, "")
	field(fieldEmail, "E-Mail", "email", true, "")
	field(fieldPhone, "Telefon", "tel", false, "Optional — helps if we have to ask something.")
	field(fieldAddress, "Adresse", "textarea", false, "Only needed if it is to be delivered.")
	field(fieldRemark, "Bemerkung", "textarea", false, "")

	b.WriteString(`<p class="bestellung__abschicken">` +
		`<button type="submit">Bestellung abschicken</button></p>`)
	b.WriteString(`</form></div>`)
	return b.String()
}

// --- die Absendung ----------------------------------------------------------

// bestellungAnnehmen nimmt die Bestellung entgegen.
func acceptOrder(in plugin.RequestIn) (plugin.RequestOut, error) {
	if in.Method != "POST" {
		// A GET on this address is somebody who opened the link. To the start
		// page rather than onto an empty one.
		return plugin.RequestOut{Handled: true, Status: 303, Location: "/"}, nil
	}

	form, err := url.ParseQuery(in.Body)
	if err != nil {
		return back("", "fehler", "Die Bestellung war nicht lesbar."), nil
	}
	page := clean(form.Get(fieldPageKey), 200)

	// The honeypot first: whatever was written in here was not a human being.
	// The answer looks like a success, so that a script does not learn what it
	// failed on.
	if strings.TrimSpace(form.Get(fieldHoneypot)) != "" {
		plugin.Log("info", "order with a filled honeypot discarded")
		return back(page, "gesendet", ""), nil
	}
	if !timeTokenHolds(form.Get(fieldTime)) {
		return back(page, "fehler",
			"The form has expired. Please reload the page and send it again."), nil
	}

	e := einstellungenLaden()
	produkte, err := readProducts(e)
	if err != nil {
		return plugin.RequestOut{}, err
	}

	item, problem := readItems(form, produkte, e)
	if problem != "" {
		return back(page, "fehler", problem), nil
	}

	b := order{
		Name:      clean(form.Get(fieldName), maxName),
		Email:     clean(form.Get(fieldEmail), maxEmail),
		Telefon:   clean(form.Get(fieldPhone), maxTelefon),
		Address:   clean(form.Get(fieldAddress), maxAddress),
		Bemerkung: clean(form.Get(fieldRemark), maxBemerkung),
		Page:      page,
		Posten:    item,
		Currency:  e.Currency,
	}
	if b.Name == "" {
		return back(page, "fehler", "Bitte trage deinen Namen ein."), nil
	}
	if !addressLooksReal(b.Email) {
		return back(page, "fehler", "Please enter a valid e-mail address."), nil
	}
	b.Summe, b.SummeBekannt = total(item)

	if !underTheHourlyLimit() {
		return back(page, "fehler",
			"A great many orders are coming in just now. Please try again in an hour."), nil
	}
	if err := speichern(&b); err != nil {
		return plugin.RequestOut{}, err
	}
	notify(b)
	return back(page, "gesendet", ""), nil
}

// readItems collects the quantities that were ordered.
//
// Price and description come from the page and not from the form: otherwise the
// visitor would decide what something costs. What comes from the form is the
// quantity, and nothing else.
func readItems(form url.Values, produkte []product, e settings) ([]item, string) {
	var out []item
	for _, p := range produkte {
		raw := strings.TrimSpace(form.Get(quantityPrefix + p.Slug))
		if raw == "" || raw == "0" {
			continue
		}
		quantity, err := strconv.Atoi(raw)
		if err != nil || quantity < 0 {
			return nil, "Bei „" + p.Titel + "” there is no number."
		}
		if quantity == 0 {
			continue
		}
		if quantity > maxQuantity {
			return nil, "Bei „" + p.Titel + "“ ist die Menge zu gross. Bitte melde dich direkt bei uns."
		}
		if !p.Orderable {
			return nil, "„" + p.Titel + "” cannot be ordered at the moment."
		}
		out = append(out, item{
			Slug: p.Slug, Titel: p.Titel, Quantity: quantity,
			Price: p.Price, Einheit: p.Einheit,
		})
		if len(out) > maxPosten {
			return nil, "That is a great many different items. Please get in touch with us directly."
		}
	}
	if len(out) == 0 {
		return nil, "Please enter a quantity for at least one product."
	}
	_ = e
	return out, ""
}

// total adds up when every price can be read.
//
// If one cannot be read there is no total rather than a wrong one: the operator
// sees the items and works it out, and nobody is sent a confirmed amount that is
// wrong.
func total(item []item) (float64, bool) {
	total := 0.0
	for _, p := range item {
		value, ok := priceValue(p.Price)
		if !ok {
			return 0, false
		}
		total += value * float64(p.Quantity)
	}
	return total, true
}

// back sends the visitor back to the page, with the outcome in the address. A
// redirect rather than an answer in the body, so that a reload does not send the
// order a second time.
func back(page, stand, note string) plugin.RequestOut {
	ziel := "/"
	if page != "" {
		ziel = "/" + page
	}
	q := url.Values{}
	q.Set("bestellung", stand)
	if note != "" {
		q.Set("hinweis", note)
	}
	return plugin.RequestOut{Handled: true, Status: 303, Location: ziel + "?" + q.Encode()}
}

// notify tells the operator.
//
// After storing: an order that stands in the admin has arrived — whether the
// e-mail gets through changes nothing about that. A failure here must never
// reach the person ordering.
func notify(b order) {
	var t strings.Builder
	fmt.Fprintf(&t, "Neue Bestellung von %s\n\n", b.Name)
	for _, p := range b.Posten {
		fmt.Fprintf(&t, "  %d × %s", p.Quantity, p.Titel)
		if p.Price != "" {
			fmt.Fprintf(&t, "  (%s %s", b.Currency, p.Price)
			if p.Einheit != "" {
				fmt.Fprintf(&t, " / %s", p.Einheit)
			}
			t.WriteString(")")
		}
		t.WriteString("\n")
	}
	if b.SummeBekannt {
		fmt.Fprintf(&t, "\nSumme: %s %s\n", b.Currency, amountText(b.Summe))
	} else {
		t.WriteString("\nTotal: could not be worked out – please check.\n")
	}
	fmt.Fprintf(&t, "\nE-Mail: %s\n", b.Email)
	if b.Telefon != "" {
		fmt.Fprintf(&t, "Telefon: %s\n", b.Telefon)
	}
	if b.Address != "" {
		fmt.Fprintf(&t, "Adresse:\n%s\n", b.Address)
	}
	if b.Bemerkung != "" {
		fmt.Fprintf(&t, "\nBemerkung:\n%s\n", b.Bemerkung)
	}
	t.WriteString("\nThe order also stands in the admin under “Orders”.\n")

	// The orderer's address as the reply address: the operator presses reply
	// and writes to the customer without typing the address out.
	queued, reason, err := plugin.Notify("Neue Bestellung von "+b.Name, t.String(), b.Email)
	switch {
	case err != nil:
		plugin.Logf("warn", "the notification about the order did not go out: %v", err)
	case !queued:
		plugin.Logf("info", "keine Benachrichtigung verschickt: %s", reason)
	}
}

// --- Kleinkram --------------------------------------------------------------

// clean takes control characters out and truncates.
func clean(v string, max int) string {
	v = strings.TrimSpace(v)
	v = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, v)
	if len(v) > max {
		v = v[:max]
	}
	return v
}

// addressLooksReal checks as much as can be checked without writing to it.
func addressLooksReal(v string) bool {
	at := strings.LastIndex(v, "@")
	if at <= 0 || at == len(v)-1 || len(v) > maxEmail {
		return false
	}
	rest := v[at+1:]
	return strings.Contains(rest, ".") && !strings.ContainsAny(v, " \t\n")
}

// --- Zeitmarke gegen Skripte ------------------------------------------------

// timestamp is the moment the form was drawn, signed.
//
// Signed, because a value the sender can set freely is not a statement. The key
// lies in the plugin's storage and is created the first time: a hard-coded one
// would stand in every copy of this source.
func timestamp() string {
	jetzt := strconv.FormatInt(time.Now().Unix(), 10)
	return jetzt + "." + zeichen(jetzt)
}

func timeTokenHolds(v string) bool {
	teile := strings.SplitN(v, ".", 2)
	if len(teile) != 2 || !hmac.Equal([]byte(zeichen(teile[0])), []byte(teile[1])) {
		return false
	}
	gestellt, err := strconv.ParseInt(teile[0], 10, 64)
	if err != nil {
		return false
	}
	alter := time.Now().Unix() - gestellt
	// Downwards, because a human being does not fill in an order in two
	// seconds; upwards, because a form that has stood open for a day comes from
	// a page whose prices may have changed.
	return alter >= 2 && alter <= 12*60*60
}

func zeichen(v string) string {
	m := hmac.New(sha256.New, schluessel())
	m.Write([]byte(v))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

const keyToken = "zeitmarken-schluessel"

func schluessel() []byte {
	if raw, da, err := plugin.Get(keyToken); err == nil && da && raw != "" {
		if key, err := base64.RawStdEncoding.DecodeString(raw); err == nil && len(key) == 32 {
			return key
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// Without randomness there is no signature. A fixed value would be
		// worse than no marker at all, so there is none here.
		plugin.Logf("error", "no randomness for the key: %v", err)
		return nil
	}
	if err := plugin.Set(keyToken, base64.RawStdEncoding.EncodeToString(key)); err != nil {
		plugin.Logf("warn", "key not stored: %v", err)
	}
	return key
}

// --- Stundengrenze ----------------------------------------------------------

const schluesselZaehler = "zaehler"

// underTheHourlyLimit counts the current hour's orders.
func underTheHourlyLimit() bool {
	stunde := time.Now().UTC().Format("2006-01-02T15")
	raw, _, err := plugin.Get(schluesselZaehler)
	if err != nil {
		return true
	}
	var z struct {
		Stunde string `json:"stunde"`
		Count  int    `json:"anzahl"`
	}
	_ = json.Unmarshal([]byte(raw), &z)
	if z.Stunde != stunde {
		z.Stunde, z.Count = stunde, 0
	}
	if z.Count >= maxProStunde {
		return false
	}
	z.Count++
	if neu, err := json.Marshal(z); err == nil {
		_ = plugin.Set(schluesselZaehler, string(neu))
	}
	return true
}

// --- Speicher ---------------------------------------------------------------

// item is one line of the order.
type item struct {
	Slug     string `json:"slug"`
	Titel    string `json:"titel"`
	Quantity int    `json:"menge"`
	Price    string `json:"preis,omitempty"`
	Einheit  string `json:"einheit,omitempty"`
}

// order is what gets stored.
//
// The prices are stored with it and are not fetched from the page later:
// if the price changes tomorrow, this order keeps today's.
type order struct {
	ID           string  `json:"id"`
	Eingegangen  string  `json:"eingegangen"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	Telefon      string  `json:"telefon,omitempty"`
	Address      string  `json:"adresse,omitempty"`
	Bemerkung    string  `json:"bemerkung,omitempty"`
	Page         string  `json:"seite,omitempty"`
	Posten       []item  `json:"posten"`
	Summe        float64 `json:"summe,omitempty"`
	SummeBekannt bool    `json:"summe_bekannt,omitempty"`
	Currency     string  `json:"waehrung,omitempty"`
	// Done is set once the operator has ticked the order off.
	Done bool `json:"erledigt,omitempty"`
}

const prefixOrder = "bestellung:"

// speichern legt die Bestellung ab.
//
// The key starts with the point in time, so that the list comes back sorted
// by date, and ends in randomness, so that two orders in the same second do
// not overwrite each other.
func speichern(b *order) error {
	jetzt := time.Now().UTC()
	b.Eingegangen = jetzt.Format(time.RFC3339)
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	b.ID = jetzt.Format("20060102T150405") + "-" + base64.RawURLEncoding.EncodeToString(raw)

	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return plugin.Set(prefixOrder+b.ID, string(data))
}

// alleBestellungen liest sie, neueste zuerst.
func allOrders() ([]order, error) {
	values, err := plugin.List(prefixOrder, 500)
	if err != nil {
		return nil, err
	}
	out := make([]order, 0, len(values))
	for _, raw := range values {
		var b order
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			continue
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func loadOrder(id string) (order, bool) {
	raw, da, err := plugin.Get(prefixOrder + id)
	if err != nil || !da {
		return order{}, false
	}
	var b order
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		return order{}, false
	}
	return b, true
}

func saveOrder(b order) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return plugin.Set(prefixOrder+b.ID, string(data))
}

func main() {}
