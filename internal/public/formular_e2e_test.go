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
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Das Kontaktformular als Plugin, durch die ganze Kette: die Marke im Text wird
// zum Formular, /formular nimmt die Absendung an, und die Nachricht liegt
// danach im eigenen Speicher des Plugins.
func TestKontaktformularPluginNimmtNachrichtenAn(t *testing.T) {
	h, database, ws, manager := formularAufbau(t)

	// --- die Marke wird zum Formular ---
	seite := h.plugins.FilterContent(context.Background(), ws.ID, plugin.ContentIn{
		WebsiteID: ws.ID, Slug: "kontakt", Title: "Kontakt",
		HTML: "<p>Schreib uns:</p><p>[[formular:Rohwolle]]</p>",
	})
	if !strings.Contains(seite, `<form class="contact-form"`) {
		t.Fatalf("aus der Marke wurde kein Formular:\n%s", seite)
	}
	if !strings.Contains(seite, `value="Rohwolle"`) {
		t.Errorf("der Betreff aus der Marke steht nicht im Feld:\n%s", seite)
	}
	// Ein <form> in einem <p> ist ungültiges HTML, das der Browser umsortiert.
	if strings.Contains(seite, "<p><form") || strings.Contains(seite, "<p></p>") {
		t.Errorf("der Absatz um die Marke wurde nicht mit ersetzt:\n%s", seite)
	}

	// --- der Honigtopf ---
	rec := absenden(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {zeitmarke(t, database, -10*time.Second)},
		"name":      {"Ein Roboter"},
		"email":     {"bot@example.test"},
		"nachricht": {"Günstige Uhren."},
		"website":   {"https://spam.example"}, // die Falle
	})
	// Genau wie ein Erfolg beantwortet: ein Roboter, der erfährt, dass er
	// abgewiesen wurde, erfährt damit, wie er am Filter vorbeikommt.
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") {
		t.Errorf("der Honigtopf wurde nicht wie ein Erfolg beantwortet: %q", ort)
	}
	if n := nachrichten(t, database); n != 0 {
		t.Errorf("der Roboter hat %d Nachrichten hinterlassen", n)
	}

	// --- eine zu schnelle Absendung ---
	rec = absenden(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {zeitmarke(t, database, 0)},
		"name":      {"Zu schnell"},
		"email":     {"schnell@example.test"},
		"nachricht": {"Sofort abgeschickt."},
	})
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") {
		t.Errorf("die zu schnelle Absendung wurde nicht wie ein Erfolg beantwortet: %q", ort)
	}
	if n := nachrichten(t, database); n != 0 {
		t.Errorf("die zu schnelle Absendung wurde gespeichert (%d)", n)
	}

	// --- eine unvollständige Absendung ---
	rec = absenden(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {zeitmarke(t, database, -10*time.Second)},
		"name":      {""},
		"email":     {"eva@example.test"},
		"nachricht": {"Ohne Namen."},
	})
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=fehler") {
		t.Errorf("die unvollständige Absendung wurde angenommen: %q", ort)
	}

	// --- eine echte Absendung ---
	rec = absenden(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {zeitmarke(t, database, -10*time.Second)},
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
	if n := nachrichten(t, database); n != 1 {
		t.Fatalf("%d Nachrichten gespeichert, want 1", n)
	}

	// --- und sie steht in der Verwaltung ---
	out, err := manager.Admin(context.Background(), "kontaktformular",
		plugin.AdminIn{WebsiteID: ws.ID, Method: "GET"})
	if err != nil {
		t.Fatalf("Admin: %v", err)
	}
	for _, wollte := range []string{"Eva Muster", "Rohwolle", "braune Wolle"} {
		if !strings.Contains(out.HTML, wollte) {
			t.Errorf("%q fehlt auf dem Verwaltungsbildschirm:\n%s", wollte, out.HTML)
		}
	}
}

// Was ein Besucher schreibt, wird auf dem Bildschirm des Betreibers gelesen.
// Eine Nachricht mit Markup darin darf dort kein Markup werden — sonst ist das
// Kontaktformular der Weg, dem Betreiber etwas in die Verwaltung zu legen.
func TestNachrichtWirdInDerVerwaltungMaskiert(t *testing.T) {
	h, database, ws, manager := formularAufbau(t)

	absenden(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {zeitmarke(t, database, -10*time.Second)},
		"name":      {`<img src=x onerror="alert(1)">`},
		"email":     {"boese@example.test"},
		"nachricht": {`<script>alert(2)</script>`},
	})

	out, err := manager.Admin(context.Background(), "kontaktformular",
		plugin.AdminIn{WebsiteID: ws.ID, Method: "GET"})
	if err != nil {
		t.Fatalf("Admin: %v", err)
	}
	// Die spitze Klammer ist der Unterschied: &lt;script&gt; ist Text, den
	// jemand geschrieben hat, <script> wäre ein Skript im Browser des
	// Betreibers. Der Host filtert danach noch einmal — aber ein Plugin, das
	// sich darauf verlässt, ist ein Plugin, das anderswo daneben liegt.
	for _, roh := range []string{"<script", "<img"} {
		if strings.Contains(out.HTML, roh) {
			t.Errorf("%s kam ungefiltert durch:\n%s", roh, out.HTML)
		}
	}
}

// --- Aufbau -----------------------------------------------------------------

func formularAufbau(t *testing.T) (*Handler, *db.DB, *domain.Website, *plugin.Manager) {
	t.Helper()
	modul, err := os.ReadFile("../../plugins/kontaktformular/plugin.wasm")
	if err != nil {
		t.Skipf("plugins/kontaktformular/plugin.wasm fehlt: %v", err)
	}
	roh, err := os.ReadFile("../../plugins/kontaktformular/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := plugin.ParseManifest(roh)
	if err != nil {
		t.Fatalf("das mitgelieferte Manifest ist ungültig: %v", err)
	}

	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")
	manager := loadPlugin(t, h, database, manifest, modul, ws.ID)
	h.SetPlugins(manager)

	// Einmal die Marke ausfüllen lassen: dabei zieht das Plugin seinen
	// Signaturschlüssel, den die Tests danach brauchen.
	manager.FilterContent(context.Background(), ws.ID, plugin.ContentIn{
		WebsiteID: ws.ID, Slug: "kontakt", Title: "Kontakt", HTML: "<p>[[formular]]</p>",
	})
	return h, database, ws, manager
}

// absenden schickt ein Formular durch dieselbe Middleware wie der Server.
func absenden(t *testing.T, h *Handler, ws *domain.Website, form url.Values) *httptest.ResponseRecorder {
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

// zeitmarke baut eine gültige Marke mit dem Schlüssel, den das Plugin selbst
// gezogen hat.
//
// Der Test greift damit in den Speicher des Plugins hinein. Der Grund ist die
// Zeitfalle: sie verlangt drei Sekunden zwischen Zeichnen und Absenden, und
// diese drei Sekunden in jedem Testlauf abzuwarten wäre die Art von Kosten, die
// am Ende dazu führt, dass niemand die Tests mehr laufen lässt.
func zeitmarke(t *testing.T, database *db.DB, alter time.Duration) string {
	t.Helper()
	store := plugin.NewStore(database)
	roh, ok, err := store.StoreGet(context.Background(), "kontaktformular", 0, "signaturschluessel")
	if err != nil || !ok {
		t.Fatalf("das Plugin hat noch keinen Signaturschlüssel gezogen (ok=%v, err=%v)", ok, err)
	}
	key, err := base64.RawStdEncoding.DecodeString(roh)
	if err != nil {
		t.Fatalf("der Signaturschlüssel ist nicht lesbar: %v", err)
	}

	stempel := strconv.FormatInt(time.Now().Add(alter).UTC().Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(stempel))
	return stempel + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// nachrichten zählt, was im Speicher des Plugins liegt.
func nachrichten(t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRow(
		`SELECT COUNT(*) FROM plugin_store WHERE plugin_id = 'kontaktformular' AND key LIKE 'nachricht:%'`).
		Scan(&n); err != nil {
		t.Fatalf("zählen: %v", err)
	}
	return n
}

// Eine Anfrage soll den Betreiber erreichen, nicht nur in der Verwaltung
// liegen. Das Plugin darf dabei keine Adresse nennen — die steht in den
// Einstellungen der Website, und der Host setzt sie ein.
func TestAnfrageLandetImPostausgang(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)

	// Ein Mailserver, der nichts kann ausser existieren: geprüft wird, was
	// eingereiht wird, nicht was zugestellt wird.
	queue := mail.NewQueue(database, mail.NewSender(mail.Config{
		Host: "mail.example.test", From: "cms@example.test",
	}), slog.New(slog.DiscardHandler))
	domains := domain.NewStore(database)
	if err := domains.UpdateSettings(context.Background(), ws.ID, domain.Settings{
		NotifyEmail: "eva@example.test", OfflineMode: "notfound", PostsPerPage: 10,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	// Neu gelesen, damit der Handler die frisch gesetzte Adresse sieht.
	h.SetNotify(domains, queue)

	absenden(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {zeitmarke(t, database, -10*time.Second)},
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
		t.Errorf("Empfänger = %q — die Adresse kommt aus den Einstellungen", empfaenger)
	}
	if !strings.Contains(betreff, "Rohwolle") || !strings.Contains(betreff, ws.Name) {
		t.Errorf("Betreff = %q, erwartet Website-Name und Anliegen", betreff)
	}
	// Antworten ist ein Klick und kein Wechsel in die Verwaltung.
	if antwortAn != "besucher@example.test" {
		t.Errorf("Antwortadresse = %q", antwortAn)
	}
	if !strings.Contains(rumpf, "braune Wolle") {
		t.Errorf("der Text fehlt:\n%s", rumpf)
	}
}

// Ohne hinterlegte Adresse wird nichts verschickt — und das ist kein Fehler,
// sondern eine Entscheidung des Betreibers.
func TestOhneBenachrichtigungsadresseKeineMail(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)
	h.SetNotify(domain.NewStore(database), mail.NewQueue(database, mail.NewSender(mail.Config{
		Host: "mail.example.test", From: "cms@example.test",
	}), slog.New(slog.DiscardHandler)))

	rec := absenden(t, h, ws, url.Values{
		"seite":     {"kontakt"},
		"gestellt":  {zeitmarke(t, database, -10*time.Second)},
		"name":      {"Eva Muster"},
		"email":     {"besucher@example.test"},
		"nachricht": {"Eine Frage."},
	})
	// Für den Besucher ändert sich nichts: seine Nachricht ist angekommen.
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") {
		t.Errorf("die Absendung schlug fehl: %q", ort)
	}
	if n := nachrichten(t, database); n != 1 {
		t.Errorf("die Nachricht wurde nicht gespeichert (%d)", n)
	}

	var offen int
	database.Read.QueryRow(`SELECT COUNT(*) FROM mail_outbox`).Scan(&offen)
	if offen != 0 {
		t.Errorf("%d Nachrichten im Postausgang, obwohl keine Adresse hinterlegt ist", offen)
	}
}

// Ein zusammengestelltes Formular: anlegen, in eine Seite setzen, ausfüllen,
// und die Antworten stehen benannt in der Verwaltung.
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

	// Zwei Felder anlegen und ausfüllen. Der Editor schickt bei jeder Aktion
	// das ganze Formular mit, also wird hier genauso vorgegangen.
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

	// --- die Marke wird zum eigenen Formular ---
	seite := manager.FilterContent(ctx, ws.ID, plugin.ContentIn{
		WebsiteID: ws.ID, Slug: "hoffest", Title: "Hoffest",
		HTML: "<p>[[formular:anmeldung-zum-hoffest]]</p>",
	})
	for _, wollte := range []string{"Dein Name", "E-Mail", `name="f_dein-name"`, `type="email"`} {
		if !strings.Contains(seite, wollte) {
			t.Errorf("%q fehlt im gezeichneten Formular:\n%s", wollte, seite)
		}
	}

	// --- ein Pflichtfeld fehlt ---
	rec := absenden(t, h, ws, url.Values{
		"seite": {"hoffest"}, "formular": {"anmeldung-zum-hoffest"},
		"gestellt":    {zeitmarke(t, database, -10*time.Second)},
		"f_dein-name": {"Eva"},
	})
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=fehler") {
		t.Errorf("die unvollständige Absendung wurde angenommen: %q", ort)
	}

	// --- vollständig ---
	rec = absenden(t, h, ws, url.Values{
		"seite": {"hoffest"}, "formular": {"anmeldung-zum-hoffest"},
		"gestellt":    {zeitmarke(t, database, -10*time.Second)},
		"f_dein-name": {"Eva Muster"},
		"f_e-mail":    {"eva@example.test"},
	})
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") ||
		!strings.Contains(ort, "welches=anmeldung-zum-hoffest") {
		t.Errorf("Location = %q", ort)
	}
	if n := nachrichten(t, database); n != 1 {
		t.Fatalf("%d Nachrichten gespeichert", n)
	}

	// --- und die Antworten stehen benannt in der Verwaltung ---
	out := admin(nil)
	for _, wollte := range []string{"Dein Name", "Eva Muster", "eva@example.test", "Anmeldung zum Hoffest"} {
		if !strings.Contains(out.HTML, wollte) {
			t.Errorf("%q fehlt auf dem Bildschirm:\n%s", wollte, out.HTML)
		}
	}
}

// Die Marke bleibt rückwärtskompatibel: was kein Formular benennt, ist wie
// bisher ein vorausgefüllter Betreff.
func TestMarkeMitUnbekanntemArgumentBleibtDerBetreff(t *testing.T) {
	_, _, ws, manager := formularAufbau(t)

	seite := manager.FilterContent(context.Background(), ws.ID, plugin.ContentIn{
		WebsiteID: ws.ID, Slug: "wolle", Title: "Wolle",
		HTML: "<p>[[formular:Rohwolle]]</p>",
	})
	if !strings.Contains(seite, `value="Rohwolle"`) {
		t.Errorf("der Betreff wurde nicht vorausgefüllt:\n%s", seite)
	}
	// Gemeint ist mit der zweiten Prüfung: der klassische Zeichner lief, der
	// zusammengestellte nicht. Zwei Zeichen irgendwo im Dokument zu suchen war
	// dafür ein schlechter Stellvertreter, und zwar in beide Richtungen. Das
	// versteckte Feld "gestellt" trägt eine base64url-Signatur, deren Alphabet
	// f und _ enthält, also stand "f_" rund einmal in achtzig Läufen zufällig
	// darin und färbte die CI rot, ohne dass etwas kaputt war. Und ein
	// zusammengestelltes Formular ohne Felder hat gar keinen Feldnamen; es wäre
	// unbemerkt durchgegangen. Also wird gefragt, was gemeint ist: einmal
	// positiv, dass das klassische Formular dasteht, und dreimal negativ nach
	// den Spuren, die zeichnenEigen unvermeidlich hinterlässt.
	if !strings.Contains(seite, `<form class="contact-form" method="POST"`) {
		t.Errorf("das klassische Formular wurde nicht gezeichnet:\n%s", seite)
	}
	for _, spur := range []string{
		`contact-form--`,  // die Klasse, mit der zeichnenEigen öffnet
		`name="formular"`, // das versteckte Feld, das sagt, welches Formular es war
		`name="f_`,        // ein Feldname, in der einzigen Stellung, die er haben kann
	} {
		if strings.Contains(seite, spur) {
			t.Errorf("es wurde ein zusammengestelltes Formular gezeichnet, %q steht in der Seite:\n%s", spur, seite)
		}
	}
}

// Der Bildschirm eines Plugins geht durch den Filter des Hosts, bevor er in der
// Verwaltung landet. Ein Editor, dessen Felder dabei wegfallen, ist ein Editor,
// der beim Speichern leere Werte schickt — und das sieht man ihm nicht an.
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

	// Jedes Bedienelement, ohne das der Editor nicht funktioniert.
	for _, wollte := range []string{
		`<form method="POST"`,       // das Formular selbst
		`name="kennung"`,            // welches Formular bearbeitet wird
		`name="fe0.beschriftung"`,   // die Frage
		`<select`, `name="fe0.art"`, // die Feldart
		`<textarea`, `name="fe0.auswahl"`, // die Möglichkeiten
		`type="checkbox"`, `name="fe0.pflicht"`,
		`name="feldaktion" value="neu"`, // Feld hinzufügen
		`name="sichern"`,                // speichern
		`href="?ansicht=formulare"`,     // zurück zur Liste
	} {
		if !strings.Contains(sauber, wollte) {
			t.Errorf("%q hat den Filter nicht überstanden", wollte)
		}
	}
	// Und nichts, was ausführen könnte.
	for _, darfNicht := range []string{"<script", "onclick", "javascript:"} {
		if strings.Contains(sauber, darfNicht) {
			t.Errorf("%q steht im gefilterten Bildschirm", darfNicht)
		}
	}
}
