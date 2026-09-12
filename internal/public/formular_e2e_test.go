package public

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The contact form as a plugin, through the whole chain: the token in the text
// becomes the form, /formular accepts the submission, and the message lies
// afterwards in the plugin's own store.
func TestKontaktformularPluginNimmtNachrichtenAn(t *testing.T) {
	h, database, ws, manager := formularAufbau(t)

	// --- the token becomes the form ---
	page := h.plugins.FilterContent(context.Background(), ws.ID, plugin.ContentIn{
		WebsiteID: ws.ID, Slug: "kontakt", Title: "Kontakt",
		HTML: "<p>Schreib uns:</p><p>[[formular:Rohwolle]]</p>",
	})
	if !strings.Contains(page, `<form class="contact-form"`) {
		t.Fatalf("the marker did not become a form:\n%s", page)
	}
	if !strings.Contains(page, `value="Rohwolle"`) {
		t.Errorf("the subject from the marker is not in the field:\n%s", page)
	}
	// A <form> inside a <p> is invalid HTML that the browser reorders.
	if strings.Contains(page, "<p><form") || strings.Contains(page, "<p></p>") {
		t.Errorf("the paragraph around the marker was not replaced with it:\n%s", page)
	}

	// --- der Honigtopf ---
	rec := submit(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {timeToken(t, database, -10*time.Second)},
		"name":      {"Ein Roboter"},
		"email":     {"bot@example.test"},
		"nachricht": {"Günstige Uhren."},
		"website":   {"https://spam.example"}, // die Falle
	})
	// Answered exactly like a success: a robot that learns it was refused
	// learns by that how to get past the filter.
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") {
		t.Errorf("the honeypot was not answered like a success: %q", ort)
	}
	if n := messages(t, database); n != 0 {
		t.Errorf("der Roboter hat %d Nachrichten hinterlassen", n)
	}

	// --- eine zu schnelle Absendung ---
	rec = submit(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {timeToken(t, database, 0)},
		"name":      {"Zu schnell"},
		"email":     {"schnell@example.test"},
		"nachricht": {"Sofort abgeschickt."},
	})
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") {
		t.Errorf("the too-fast submission was not answered like a success: %q", ort)
	}
	if n := messages(t, database); n != 0 {
		t.Errorf("die zu schnelle Absendung wurde gespeichert (%d)", n)
	}

	// --- an incomplete submission ---
	rec = submit(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {timeToken(t, database, -10*time.Second)},
		"name":      {""},
		"email":     {"eva@example.test"},
		"nachricht": {"Ohne Namen."},
	})
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=fehler") {
		t.Errorf("the incomplete submission was accepted: %q", ort)
	}

	// --- eine echte Absendung ---
	rec = submit(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {timeToken(t, database, -10*time.Second)},
		"name":      {"Eva Muster"},
		"email":     {"eva@example.test"},
		"betreff":   {"Rohwolle"},
		"nachricht": {"Habt ihr noch braune Wolle?"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("Status %d, want 303", rec.Code)
	}
	if ort := rec.Header().Get("Location"); ort != "/kontakt?formular=gesendet" {
		t.Errorf("Location = %q", ort)
	}
	if n := messages(t, database); n != 1 {
		t.Fatalf("%d Nachrichten gespeichert, want 1", n)
	}

	// --- and it stands on the admin side ---
	out, err := manager.Admin(context.Background(), "kontaktformular",
		plugin.AdminIn{WebsiteID: ws.ID, Method: "GET"})
	if err != nil {
		t.Fatalf("Admin: %v", err)
	}
	for _, wanted := range []string{"Eva Muster", "Rohwolle", "braune Wolle"} {
		if !strings.Contains(out.HTML, wanted) {
			t.Errorf("%q fehlt auf dem Verwaltungsbildschirm:\n%s", wanted, out.HTML)
		}
	}
}

// What a visitor writes is read on the operator's screen. A message with markup
// in it must not become markup there — or the contact form is the way to put
// something into the operator's admin.
func TestNachrichtWirdInDerVerwaltungMaskiert(t *testing.T) {
	h, database, ws, manager := formularAufbau(t)

	submit(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {timeToken(t, database, -10*time.Second)},
		"name":      {`<img src=x onerror="alert(1)">`},
		"email":     {"boese@example.test"},
		"nachricht": {`<script>alert(2)</script>`},
	})

	out, err := manager.Admin(context.Background(), "kontaktformular",
		plugin.AdminIn{WebsiteID: ws.ID, Method: "GET"})
	if err != nil {
		t.Fatalf("Admin: %v", err)
	}
	// The angle bracket is the difference: &lt;script&gt; is text somebody
	// wrote, <script> would be a script in the operator's browser. The host
	// filters once more afterwards — but a plugin that relies on that is a
	// plugin that gets it wrong somewhere else.
	for _, roh := range []string{"<script", "<img"} {
		if strings.Contains(out.HTML, roh) {
			t.Errorf("%s kam ungefiltert durch:\n%s", roh, out.HTML)
		}
	}
}

// --- Aufbau -----------------------------------------------------------------

func formularAufbau(t *testing.T) (*Handler, *db.DB, *domain.Website, *plugin.Manager) {
	t.Helper()
	modul := wasmtest.Modul(t, "../../plugins/kontaktformular/plugin.wasm")
	roh, err := os.ReadFile("../../plugins/kontaktformular/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := plugin.ParseManifest(roh)
	if err != nil {
		t.Fatalf("the shipped manifest is invalid: %v", err)
	}

	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")
	manager := loadPlugin(t, h, database, manifest, modul, ws.ID)
	h.SetPlugins(manager)

	// Have the token filled in once: in doing so the plugin draws its signing
	// key, which the tests need afterwards.
	manager.FilterContent(context.Background(), ws.ID, plugin.ContentIn{
		WebsiteID: ws.ID, Slug: "kontakt", Title: "Kontakt", HTML: "<p>[[formular]]</p>",
	})
	return h, database, ws, manager
}

// submit sends a form through the same middleware as the server.
func submit(t *testing.T, h *Handler, ws *domain.Website, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "http://velowerkstatt.test/formular",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))

	rec := httptest.NewRecorder()
	h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("die Absendung lief am Plugin vorbei bis zum Kern")
	})).ServeHTTP(rec, req)
	return rec
}

// timeToken builds a valid token with the key the plugin drew itself.
//
// The test thereby reaches into the plugin's store. The reason is the time
// trap: it demands three seconds between drawing and submitting, and waiting
// those three seconds out in every test run would be the kind of cost that ends
// with nobody running the tests any more.
func timeToken(t *testing.T, database *db.DB, alter time.Duration) string {
	t.Helper()
	store := plugin.NewStore(database)
	roh, ok, err := store.StoreGet(context.Background(), "kontaktformular", 0, "signaturschluessel")
	if err != nil || !ok {
		t.Fatalf("the plugin has not drawn a signing key yet (ok=%v, err=%v)", ok, err)
	}
	key, err := base64.RawStdEncoding.DecodeString(roh)
	if err != nil {
		t.Fatalf("the signing key is not readable: %v", err)
	}

	stempel := strconv.FormatInt(time.Now().Add(alter).UTC().Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(stempel))
	return stempel + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// messages counts what lies in the plugin's store.
func messages(t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRow(
		`SELECT COUNT(*) FROM plugin_store WHERE plugin_id = 'kontaktformular' AND key LIKE 'nachricht:%'`).
		Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// An enquiry should reach the operator, not merely lie in the admin. The plugin
// may name no address in doing so — that stands in the website's settings, and
// the host puts it in.
func TestAnfrageLandetImPostausgang(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)

	// A mail server that can do nothing but exist: what is checked is what gets
	// queued, not what gets delivered.
	queue := mail.NewQueue(database, mail.NewSender(mail.Config{
		Host: "mail.example.test", From: "cms@example.test",
	}), slog.New(slog.DiscardHandler))
	domains := domain.NewStore(database)
	if err := domains.UpdateSettings(context.Background(), ws.ID, domain.Settings{
		NotifyEmail: "eva@example.test", OfflineMode: "notfound", PostsPerPage: 10,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	// Read afresh, so that the handler sees the newly set address.
	h.SetNotify(domains, queue)

	submit(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {timeToken(t, database, -10*time.Second)},
		"name":      {"Eva Muster"},
		"email":     {"besucher@example.test"},
		"betreff":   {"Rohwolle"},
		"nachricht": {"Habt ihr noch braune Wolle?"},
	})

	var empfaenger, betreff, rumpf, antwortAn string
	if err := database.Read.QueryRow(
		`SELECT recipient, subject, body, reply_to FROM mail_outbox`).
		Scan(&empfaenger, &betreff, &rumpf, &antwortAn); err != nil {
		t.Fatalf("nichts im Postausgang: %v", err)
	}
	if empfaenger != "eva@example.test" {
		t.Errorf("recipient = %q — the address comes from the settings", empfaenger)
	}
	if !strings.Contains(betreff, "Rohwolle") || !strings.Contains(betreff, ws.Name) {
		t.Errorf("Betreff = %q, erwartet Website-Name und Anliegen", betreff)
	}
	// Replying is one click and not a move into the admin.
	if antwortAn != "besucher@example.test" {
		t.Errorf("Antwortadresse = %q", antwortAn)
	}
	if !strings.Contains(rumpf, "braune Wolle") {
		t.Errorf("der Text fehlt:\n%s", rumpf)
	}
}

// Without a stored address nothing is sent — and that is not a fault but a
// decision of the operator's.
func TestOhneBenachrichtigungsadresseKeineMail(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)
	h.SetNotify(domain.NewStore(database), mail.NewQueue(database, mail.NewSender(mail.Config{
		Host: "mail.example.test", From: "cms@example.test",
	}), slog.New(slog.DiscardHandler)))

	rec := submit(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {timeToken(t, database, -10*time.Second)},
		"name":      {"Eva Muster"},
		"email":     {"besucher@example.test"},
		"nachricht": {"Eine Frage."},
	})
	// For the visitor nothing changes: their message has arrived.
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") {
		t.Errorf("die Absendung schlug fehl: %q", ort)
	}
	if n := messages(t, database); n != 1 {
		t.Errorf("the message was not stored (%d)", n)
	}

	var offen int
	database.Read.QueryRow(`SELECT COUNT(*) FROM mail_outbox`).Scan(&offen)
	if offen != 0 {
		t.Errorf("%d messages in the outbox although no address is configured", offen)
	}
}

// An assembled form: create it, put it into a page, fill it in, and the answers
// stand named on the admin side.
func TestEigenesFormularVonEndeZuEnde(t *testing.T) {
	h, database, ws, manager := formularAufbau(t)
	ctx := context.Background()

	// --- anlegen ---
	admin := func(form url.Values) plugin.AdminOut {
		t.Helper()
		in := plugin.AdminIn{WebsiteID: ws.ID, Method: "GET"}
		if form != nil {
			in.Method = "POST"
			in.Form = form
		}
		out, err := manager.Admin(ctx, "kontaktformular", in)
		if err != nil {
			t.Fatalf("Admin: %v", err)
		}
		return *out
	}

	if out := admin(url.Values{"neues_formular": {"Anmeldung zum Hoffest"}}); out.FlashError {
		t.Fatalf("Anlegen fehlgeschlagen: %s", out.Flash)
	}

	// Create two fields and fill them in. The editor sends the whole form along
	// with every action, so the same is done here.
	admin(url.Values{"kennung": {"anmeldung-zum-hoffest"}, "name": {"Anmeldung zum Hoffest"},
		"feldaktion": {"neu"}})
	admin(url.Values{"kennung": {"anmeldung-zum-hoffest"}, "name": {"Anmeldung zum Hoffest"},
		"fe0.beschriftung": {"Dein Name"}, "fe0.art": {"text"}, "fe0.pflicht": {"1"},
		"feldaktion": {"neu"}})
	admin(url.Values{"kennung": {"anmeldung-zum-hoffest"}, "name": {"Anmeldung zum Hoffest"},
		"fe0.beschriftung": {"Dein Name"}, "fe0.art": {"text"}, "fe0.pflicht": {"1"},
		"fe1.beschriftung": {"E-Mail"}, "fe1.art": {"email"}, "fe1.pflicht": {"1"},
		"sichern": {"1"}})

	liste := admin(nil)
	_ = liste

	// --- the token becomes the form of one's own ---
	page := manager.FilterContent(ctx, ws.ID, plugin.ContentIn{
		WebsiteID: ws.ID, Slug: "hoffest", Title: "Hoffest",
		HTML: "<p>[[formular:anmeldung-zum-hoffest]]</p>",
	})
	for _, wanted := range []string{"Dein Name", "E-Mail", `name="f_dein-name"`, `type="email"`} {
		if !strings.Contains(page, wanted) {
			t.Errorf("%q fehlt im gezeichneten Formular:\n%s", wanted, page)
		}
	}

	// --- ein Pflichtfeld fehlt ---
	rec := submit(t, h, ws, url.Values{
		"seite": {"hoffest"}, "formular": {"anmeldung-zum-hoffest"},
		"gestellt":    {timeToken(t, database, -10*time.Second)},
		"f_dein-name": {"Eva"},
	})
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=fehler") {
		t.Errorf("the incomplete submission was accepted: %q", ort)
	}

	// --- complete ---
	rec = submit(t, h, ws, url.Values{
		"seite": {"hoffest"}, "formular": {"anmeldung-zum-hoffest"},
		"gestellt":    {timeToken(t, database, -10*time.Second)},
		"f_dein-name": {"Eva Muster"},
		"f_e-mail":    {"eva@example.test"},
	})
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") ||
		!strings.Contains(ort, "welches=anmeldung-zum-hoffest") {
		t.Errorf("Location = %q", ort)
	}
	if n := messages(t, database); n != 1 {
		t.Fatalf("%d Nachrichten gespeichert", n)
	}

	// --- and the answers stand named on the admin side ---
	out := admin(nil)
	for _, wanted := range []string{"Dein Name", "Eva Muster", "eva@example.test", "Anmeldung zum Hoffest"} {
		if !strings.Contains(out.HTML, wanted) {
			t.Errorf("%q fehlt auf dem Bildschirm:\n%s", wanted, out.HTML)
		}
	}
}

// The token stays backwards compatible: what names no form is, as before, a
// pre-filled subject.
func TestMarkeMitUnbekanntemArgumentBleibtDerBetreff(t *testing.T) {
	_, _, ws, manager := formularAufbau(t)

	page := manager.FilterContent(context.Background(), ws.ID, plugin.ContentIn{
		WebsiteID: ws.ID, Slug: "wolle", Title: "Wolle",
		HTML: "<p>[[formular:Rohwolle]]</p>",
	})
	if !strings.Contains(page, `value="Rohwolle"`) {
		t.Errorf("the subject was not pre-filled:\n%s", page)
	}
	// What the second check means: the classic renderer ran, the assembled one
	// did not. Looking for two characters somewhere in the document was a poor
	// stand-in for that, and in both directions. The hidden field "gestellt"
	// carries a base64url signature whose alphabet contains f and _, so "f_"
	// stood in it by chance about once in eighty runs and turned CI red without
	// anything being broken. And an assembled form without fields has no field
	// name at all; it would have gone through unnoticed. So what is meant is
	// what is asked: once positively, that the classic form is there, and three
	// times negatively for the traces that zeichnenEigen inevitably leaves.
	if !strings.Contains(page, `<form class="contact-form" method="POST"`) {
		t.Errorf("the plain form was not drawn:\n%s", page)
	}
	for _, trace := range []string{
		`contact-form--`,  // the class zeichnenEigen opens with
		`name="formular"`, // the hidden field that says which form it was
		`name="f_`,        // a field name, in the only position it can have
	} {
		if strings.Contains(page, trace) {
			t.Errorf("an assembled form was drawn, %q is in the page:\n%s", trace, page)
		}
	}
}

// A plugin's screen goes through the host's filter before it lands in the
// admin. An editor whose fields fall away in the process is an editor that
// sends empty values on saving — and you cannot tell that by looking at it.
func TestFormulareditorUeberstehtDenFilterDesHosts(t *testing.T) {
	_, _, ws, manager := formularAufbau(t)
	ctx := context.Background()

	if _, err := manager.Admin(ctx, "kontaktformular", plugin.AdminIn{
		WebsiteID: ws.ID, Method: "POST",
		Form: map[string][]string{"neues_formular": {"Anmeldung zum Hoffest"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Admin(ctx, "kontaktformular", plugin.AdminIn{
		WebsiteID: ws.ID, Method: "POST",
		Form: map[string][]string{
			"kennung": {"anmeldung-zum-hoffest"}, "name": {"Anmeldung zum Hoffest"},
			"feldaktion": {"neu"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	out, err := manager.Admin(ctx, "kontaktformular", plugin.AdminIn{
		WebsiteID: ws.ID, Method: "GET",
		Query: "ansicht=formular&kennung=anmeldung-zum-hoffest",
	})
	if err != nil {
		t.Fatal(err)
	}
	sauber := string(web.SanitizeAdminHTML(out.HTML))

	// Every control without which the editor does not work.
	for _, wanted := range []string{
		`<form method="POST"`,       // the form itself
		`name="kennung"`,            // which form is being edited
		`name="fe0.beschriftung"`,   // the question
		`<select`, `name="fe0.art"`, // the field kind
		`<textarea`, `name="fe0.auswahl"`, // the options
		`type="checkbox"`, `name="fe0.pflicht"`,
		`name="feldaktion" value="neu"`, // add a field
		`name="sichern"`,                // save
		`href="?ansicht=formulare"`,     // back to the list
	} {
		if !strings.Contains(sauber, wanted) {
			t.Errorf("%q did not survive the filter", wanted)
		}
	}
	// And nothing that could execute.
	for _, darfNicht := range []string{"<script", "onclick", "javascript:"} {
		if strings.Contains(sauber, darfNicht) {
			t.Errorf("%q steht im gefilterten Bildschirm", darfNicht)
		}
	}
}
