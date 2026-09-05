package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return database
}

// aufbau baut einen Server mit einer Website und liefert beide Schlüssel.
func aufbau(t *testing.T) (*httptest.Server, *db.DB, int64, string, string) {
	t.Helper()
	database := newTestDB(t)

	domains := domain.NewStore(database)
	ws, err := domains.CreateWebsite(context.Background(), "Testhof", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	tokens := NewStore(database)
	schreiben, _, err := tokens.Issue(context.Background(), "schreibend", 0, true, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	lesen, _, err := tokens.Issue(context.Background(), "lesend", 0, false, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	srv := NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains,
		Pages:   page.NewStore(database),
		Media:   media.NewStore(database),
	}))
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts, database, ws.ID, schreiben, lesen
}

// ruf schickt eine JSON-RPC-Nachricht und liefert die Antwort.
func ruf(t *testing.T, ts *httptest.Server, key, method string, params any) map[string]any {
	t.Helper()
	body := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(raw))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s: Status %d", method, res.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("%s: Antwort nicht lesbar: %v", method, err)
	}
	return out
}

// werkzeug ruft ein Werkzeug auf und liefert das ausgepackte Ergebnis.
func werkzeug(t *testing.T, ts *httptest.Server, key, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res := ruf(t, ts, key, "tools/call", map[string]any{"name": name, "arguments": args})
	if res["error"] != nil {
		t.Fatalf("%s: Protokollfehler %v", name, res["error"])
	}
	result, _ := res["result"].(map[string]any)
	fehler, _ := result["isError"].(bool)

	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("%s: keine Antwort", name)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if fehler {
		return map[string]any{"text": text}, true
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s: %q ist kein JSON: %v", name, text, err)
	}
	return out, false
}

// Ohne Schlüssel kommt niemand herein. Das ist die eine Prüfung, die alles
// andere trägt: fällt sie, ist jede weitere Grenze in dieser Datei gegenstandslos.
func TestOhneSchluesselAbgewiesen(t *testing.T) {
	ts, _, _, key, _ := aufbau(t)

	for _, fall := range []struct{ name, key string }{
		{"gar keiner", ""},
		{"erfunden", "hc_dasgibtesnicht"},
		{"fast richtig", key + "x"},
	} {
		t.Run(fall.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, ts.URL,
				strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
			if fall.key != "" {
				req.Header.Set("Authorization", "Bearer "+fall.key)
			}
			res, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusUnauthorized {
				t.Errorf("Status = %d, want 401", res.StatusCode)
			}
		})
	}
}

// Ein abgelaufener Schlüssel ist ein toter Schlüssel.
func TestAbgelaufenerSchluessel(t *testing.T) {
	database := newTestDB(t)
	tokens := NewStore(database)
	secret, tok, err := tokens.Issue(context.Background(), "alt", 0, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// Ablauf in die Vergangenheit rücken statt eine Stunde zu warten.
	if _, err := database.Write.Exec(`UPDATE ai_tokens SET expires_at = $1 WHERE id = $2`,
		time.Now().UTC().Add(-time.Minute).Format(timeLayout), tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Verify(context.Background(), secret); err != ErrExpired {
		t.Errorf("Verify = %v, want %v", err, ErrExpired)
	}
}

// Ein zurückgezogener Schlüssel wirkt sofort und nicht erst beim Neustart.
func TestZurueckgezogenerSchluessel(t *testing.T) {
	database := newTestDB(t)
	tokens := NewStore(database)
	secret, tok, err := tokens.Issue(context.Background(), "weg", 0, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := tokens.Revoke(context.Background(), tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Verify(context.Background(), secret); err != ErrBadToken {
		t.Errorf("Verify = %v, want %v", err, ErrBadToken)
	}
}

// Der Schlüssel selbst darf nirgends in der Datenbank stehen. Wer einen Abzug
// bekommt, soll damit nichts anfangen können.
func TestNurDerAbdruckWirdGespeichert(t *testing.T) {
	database := newTestDB(t)
	tokens := NewStore(database)
	secret, _, err := tokens.Issue(context.Background(), "geheim", 0, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	var treffer int
	if err := database.Read.QueryRow(
		`SELECT COUNT(*) FROM ai_tokens WHERE token_hash = $1`, secret).Scan(&treffer); err != nil {
		t.Fatal(err)
	}
	if treffer != 0 {
		t.Error("der Schlüssel steht im Klartext in der Datenbank")
	}
}

// Ein nur lesender Schlüssel bekommt die schreibenden Werkzeuge nicht einmal zu
// sehen — und wenn er sie doch aufruft, wird er abgewiesen. Beides, weil das
// eine Bequemlichkeit ist und das andere die Grenze.
func TestNurLesenSiehtUndDarfNichtSchreiben(t *testing.T) {
	ts, _, wsID, _, lesen := aufbau(t)

	res := ruf(t, ts, lesen, "tools/list", nil)
	result := res["result"].(map[string]any)
	for _, roh := range result["tools"].([]any) {
		name := roh.(map[string]any)["name"].(string)
		if strings.HasPrefix(name, "seite_anlegen") || strings.HasPrefix(name, "seite_aendern") ||
			strings.HasPrefix(name, "seite_veroeffentlichen") {
			t.Errorf("ein lesender Schlüssel sieht %q", name)
		}
	}

	_, fehler := werkzeug(t, ts, lesen, "seite_anlegen", map[string]any{
		"website": wsID, "titel": "Verboten", "markdown": "Text",
	})
	if !fehler {
		t.Error("ein lesender Schlüssel konnte eine Seite anlegen")
	}
}

// Ein Schlüssel für eine Website darf die andere nicht sehen.
func TestSchluesselBleibtBeiSeinerWebsite(t *testing.T) {
	database := newTestDB(t)
	domains := domain.NewStore(database)
	eins, _ := domains.CreateWebsite(context.Background(), "Eins", "")
	zwei, _ := domains.CreateWebsite(context.Background(), "Zwei", "")

	tokens := NewStore(database)
	secret, _, err := tokens.Issue(context.Background(), "nur eins", eins.ID, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains, Pages: page.NewStore(database), Media: media.NewStore(database),
	})))
	defer ts.Close()

	liste, fehler := werkzeug(t, ts, secret, "websites_auflisten", nil)
	if fehler {
		t.Fatalf("websites_auflisten: %v", liste["text"])
	}
	sites := liste["websites"].([]any)
	if len(sites) != 1 {
		t.Fatalf("%d Websites sichtbar, erwartet 1", len(sites))
	}

	if _, fehler := werkzeug(t, ts, secret, "seite_anlegen", map[string]any{
		"website": zwei.ID, "titel": "Fremd", "markdown": "Text",
	}); !fehler {
		t.Error("auf einer fremden Website konnte geschrieben werden")
	}
}

// Der ganze Weg, den ein Assistent geht: verbinden, Werkzeuge holen, eine Seite
// anlegen, sie ändern und am Ende veröffentlichen.
func TestDerGanzeWeg(t *testing.T) {
	ts, database, wsID, key, _ := aufbau(t)

	res := ruf(t, ts, key, "initialize", map[string]any{})
	if got := res["result"].(map[string]any)["protocolVersion"]; got != ProtocolVersion {
		t.Errorf("protocolVersion = %v, want %v", got, ProtocolVersion)
	}

	angelegt, fehler := werkzeug(t, ts, key, "seite_anlegen", map[string]any{
		"website": wsID, "titel": "Unsere Schafe", "markdown": "# Hallo\n\nText.",
	})
	if fehler {
		t.Fatalf("seite_anlegen: %v", angelegt["text"])
	}
	// Alles Neue ist ein Entwurf. Das ist die Regel, an der am meisten hängt:
	// ein Assistent, der aus Versehen veröffentlicht, stellt etwas Halbfertiges
	// ins Netz, und gemerkt wird das erst, wenn jemand es gelesen hat.
	if angelegt["zustand"] != "entwurf" {
		t.Fatalf("zustand = %v, want entwurf", angelegt["zustand"])
	}
	id := int64(angelegt["id"].(float64))

	gelesen, _ := werkzeug(t, ts, key, "seite_lesen", map[string]any{"id": id})
	if !strings.Contains(gelesen["markdown"].(string), "Hallo") {
		t.Errorf("markdown = %q", gelesen["markdown"])
	}

	geaendert, fehler := werkzeug(t, ts, key, "seite_aendern", map[string]any{
		"id": id, "markdown": "# Hallo\n\nMehr Text.",
	})
	if fehler {
		t.Fatalf("seite_aendern: %v", geaendert["text"])
	}
	// Ändern ist nicht Veröffentlichen.
	if geaendert["zustand"] != "entwurf" {
		t.Errorf("nach dem Ändern zustand = %v, want entwurf", geaendert["zustand"])
	}

	// Der alte Stand bleibt als Fassung erhalten, genau wie bei einer Änderung
	// aus der Verwaltung — sonst wäre ein Text, den eine KI überschreibt, weg.
	fassungen, err := page.NewStore(database).ListRevisions(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(fassungen) == 0 {
		t.Error("die Änderung hat keine Fassung hinterlassen")
	}

	oeffentlich, fehler := werkzeug(t, ts, key, "seite_veroeffentlichen", map[string]any{
		"id": id, "zustand": "veroeffentlicht",
	})
	if fehler {
		t.Fatalf("seite_veroeffentlichen: %v", oeffentlich["text"])
	}
	if oeffentlich["zustand"] != "veroeffentlicht" {
		t.Errorf("zustand = %v, want veroeffentlicht", oeffentlich["zustand"])
	}
}

// Eine Seite aus Bausteinen darf nicht durch Markdown ersetzt werden. Ohne
// diese Sperre schreibt ein Assistent seinen Text hinein und der Aufbau der
// Seite ist still verloren.
func TestBausteinseiteWirdNichtUeberschrieben(t *testing.T) {
	ts, database, wsID, key, _ := aufbau(t)

	pages := page.NewStore(database)
	p, err := pages.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: wsID, Title: "Bausteine", Slug: "bausteine",
		Markdown: "", HTML: "<p>gebaut</p>", Status: "draft",
		Blocks: `[{"kind":"text","text":"Hallo"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}

	gelesen, _ := werkzeug(t, ts, key, "seite_lesen", map[string]any{"id": p.ID})
	if gelesen["aufbau"] != "bausteine" {
		t.Errorf("aufbau = %v, want bausteine", gelesen["aufbau"])
	}

	antwort, fehler := werkzeug(t, ts, key, "seite_aendern", map[string]any{
		"id": p.ID, "markdown": "alles neu",
	})
	if !fehler {
		t.Fatal("die Bausteine wurden überschrieben")
	}
	if !strings.Contains(antwort["text"].(string), "Bausteine") {
		t.Errorf("Begründung = %q", antwort["text"])
	}

	// Nur den Titel zu ändern muss trotzdem gehen.
	if _, fehler := werkzeug(t, ts, key, "seite_aendern", map[string]any{
		"id": p.ID, "titel": "Neuer Titel",
	}); fehler {
		t.Error("der Titel einer Bausteinseite liess sich nicht ändern")
	}
}

// GET ist die Anfrage, die ein Client stellt, wenn er einen Ereignisstrom
// erwartet. Eine klare Absage schlägt einen 404, der nach falscher Adresse
// aussieht.
func TestNurPost(t *testing.T) {
	ts, _, _, key, _ := aufbau(t)
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Status = %d, want 405", res.StatusCode)
	}
}

// Eine Benachrichtigung hat keine Kennung und will keine Antwort. Wer darauf
// antwortet, bringt manche Clients zum Klagen.
func TestBenachrichtigungOhneAntwort(t *testing.T) {
	ts, _, _, key, _ := aufbau(t)
	req, _ := http.NewRequest(http.MethodPost, ts.URL,
		strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Errorf("Status = %d, want 202", res.StatusCode)
	}
}

// Jedes Werkzeug braucht ein Schema, sonst rät der Assistent die Argumente.
func TestJedesWerkzeugHatEinSchema(t *testing.T) {
	for _, w := range Tools(Deps{}) {
		if w.Description == "" {
			t.Errorf("%s hat keine Beschreibung", w.Name)
		}
		if w.InputSchema.Type != "object" {
			t.Errorf("%s: Schema-Typ %q", w.Name, w.InputSchema.Type)
		}
		for _, pflicht := range w.InputSchema.Required {
			if _, ok := w.InputSchema.Properties[pflicht]; !ok {
				t.Errorf("%s verlangt %q, beschreibt es aber nicht", w.Name, pflicht)
			}
		}
	}
}

// --- Was ein Assistent über die neuen Feldeigenschaften erfährt -----------

// aufbauMitFeldern ist aufbau, nur mit den eigenen Feldern der Website dabei.
// Ein Server ohne Fields meldet gar keine Felder — richtig für einen Bau ohne
// sie, unbrauchbar für diese Prüfungen.
func aufbauMitFeldern(t *testing.T) (*httptest.Server, *field.Store, int64, string) {
	t.Helper()
	database := newTestDB(t)

	domains := domain.NewStore(database)
	ws, err := domains.CreateWebsite(context.Background(), "Testhof", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	tokens := NewStore(database)
	schreiben, _, err := tokens.Issue(context.Background(), "schreibend", 0, true, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	fields := field.NewStore(database)
	srv := NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains,
		Pages:   page.NewStore(database),
		Media:   media.NewStore(database),
		Fields:  fields,
	}))
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts, fields, ws.ID, schreiben
}

// feldNamens sucht ein Feld in der Antwort von felder_auflisten.
func feldNamens(t *testing.T, liste []any, kennung string) map[string]any {
	t.Helper()
	for _, roh := range liste {
		e, _ := roh.(map[string]any)
		if e["kennung"] == kennung {
			return e
		}
	}
	t.Fatalf("kein Feld %q in %v", kennung, liste)
	return nil
}

// Ein Assistent schreibt in die Felder, die diese Liste beschreibt. Steht die
// Höchstzahl nicht da, schreibt es drei Werte in ein Feld, das zwei nimmt;
// steht die Schreibweise nicht da, schreibt es „Mo, Di" in eines, das eine
// Zeile je Wert erwartet. Beides wird abgelehnt, und der Assistent erfährt
// den Grund erst danach — die Liste ist die Stelle, an der es vorher steht.
func TestFelderAuflistenBeschreibtDieNeuenEigenschaften(t *testing.T) {
	ts, fields, wsID, key := aufbauMitFeldern(t)
	ctx := context.Background()

	anlegen := func(d field.Def) field.Def {
		t.Helper()
		d.WebsiteID = wsID
		got, err := fields.Create(ctx, d)
		if err != nil {
			t.Fatalf("Feld %q anlegen: %v", d.Key, err)
		}
		return *got
	}

	anlegen(field.Def{Key: "sorten", Label: "Sorten", Kind: field.KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"}, MaxValues: 2})
	anlegen(field.Def{Key: "menge", Label: "Menge", Kind: field.KindRange,
		RangeMin: "1", RangeMax: "10"})
	anlegen(field.Def{Key: "stil", Label: "Stil", Kind: field.KindChoice,
		Choices: []string{"hell", "dunkel"}, Display: field.DisplayButtons})
	anlegen(field.Def{Key: "form", Label: "Form", Kind: field.KindChoice,
		Choices: []string{"rund", "eckig"}})
	anlegen(field.Def{Key: "herkunft", Label: "Herkunft", Kind: field.KindText})
	gruppe := anlegen(field.Def{Key: "zeiten", Label: "Zeiten", Kind: field.KindGroup})
	anlegen(field.Def{ParentID: gruppe.ID, Key: "tage", Label: "Tage", Kind: field.KindMulti,
		Choices: []string{"Mo", "Di", "Mi"}, MaxValues: 2})
	anlegen(field.Def{ParentID: gruppe.ID, Key: "von", Label: "Von", Kind: field.KindRange,
		RangeMin: "0", RangeMax: "24"})

	res, fehler := werkzeug(t, ts, key, "felder_auflisten", map[string]any{"website": wsID})
	if fehler {
		t.Fatalf("felder_auflisten: %v", res["text"])
	}
	liste, _ := res["felder"].([]any)
	if len(liste) == 0 {
		t.Fatalf("keine Felder gemeldet: %v", res)
	}

	// Die Höchstzahl steht da, und zwar als Zahl.
	sorten := feldNamens(t, liste, "sorten")
	if got, will := sorten["max_werte"], float64(2); got != will {
		t.Errorf("max_werte = %v (%T), wollte %v", got, got, will)
	}
	// Und die eine Zeile, die sagt, wie mehrere Werte geschrieben werden.
	hinweis, _ := sorten["mehrere_werte"].(string)
	if hinweis == "" {
		t.Errorf("das mehrwertige Feld sagt nicht, wie sein Wert geschrieben wird: %v", sorten)
	}

	// Beide Grenzen, jede für sich.
	menge := feldNamens(t, liste, "menge")
	if menge["min_wert"] != "1" || menge["max_wert"] != "10" {
		t.Errorf("die Grenzen fehlen oder stimmen nicht: %v", menge)
	}

	// Die Darstellung nur dort, wo sie von der Ausklappliste abweicht, die
	// jedes bestehende Feld ist.
	if got := feldNamens(t, liste, "stil")["darstellung"]; got != field.DisplayButtons {
		t.Errorf("darstellung = %v, wollte %q", got, field.DisplayButtons)
	}
	if _, da := feldNamens(t, liste, "form")["darstellung"]; da {
		t.Error("die Ausklappliste meldet eine Darstellung, obwohl sie die gewöhnliche ist")
	}

	// Ein gewöhnliches Textfeld trägt nichts davon: eine gemeldete Null läse
	// sich als „keiner erlaubt", und eine gemeldete leere Grenze als „die
	// Grenze ist leer".
	herkunft := feldNamens(t, liste, "herkunft")
	for _, schluessel := range []string{"darstellung", "max_werte", "min_wert", "max_wert", "mehrere_werte"} {
		if _, da := herkunft[schluessel]; da {
			t.Errorf("das Textfeld meldet %q: %v", schluessel, herkunft)
		}
	}

	// Und dasselbe eine Ebene tiefer: ein Assistent, der über ein Unterfeld
	// weniger erfährt, schreibt in genau dieses falsch hinein.
	unter, _ := feldNamens(t, liste, "zeiten")["unterfelder"].([]any)
	if len(unter) == 0 {
		t.Fatalf("die Gruppe meldet keine Unterfelder: %v", feldNamens(t, liste, "zeiten"))
	}
	tage := feldNamens(t, unter, "tage")
	if got, will := tage["max_werte"], float64(2); got != will {
		t.Errorf("das Unterfeld meldet max_werte = %v, wollte %v", got, will)
	}
	if h, _ := tage["mehrere_werte"].(string); h == "" {
		t.Errorf("das mehrwertige Unterfeld sagt nicht, wie sein Wert geschrieben wird: %v", tage)
	}
	von := feldNamens(t, unter, "von")
	if von["min_wert"] != "0" || von["max_wert"] != "24" {
		t.Errorf("das Unterfeld meldet die Grenzen nicht: %v", von)
	}
}

// Die Rundreise: geschrieben, wie die Notiz es beschreibt, und unverändert
// zurückgelesen. Das ist die einzige Zusicherung hier, die eine Notiz auffliegen
// liesse, die eine Schreibweise beschreibt, welche der Schreibweg nicht nimmt.
func TestMehrwertigesFeldGehtDurchDieWerkzeugeUndZurueck(t *testing.T) {
	ts, fields, wsID, key := aufbauMitFeldern(t)
	if _, err := fields.Create(context.Background(), field.Def{
		WebsiteID: wsID, Key: "sorten", Label: "Sorten", Kind: field.KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"}, MaxValues: 2,
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	angelegt, fehler := werkzeug(t, ts, key, "seite_anlegen", map[string]any{
		"website": wsID, "titel": "Bretter", "markdown": "Text.",
		"felder": map[string]any{"sorten": "Eiche\nBuche"},
	})
	if fehler {
		t.Fatalf("seite_anlegen: %v", angelegt["text"])
	}
	id := int64(angelegt["id"].(float64))

	gelesen, fehler := werkzeug(t, ts, key, "seite_lesen", map[string]any{"id": id})
	if fehler {
		t.Fatalf("seite_lesen: %v", gelesen["text"])
	}
	felder, _ := gelesen["felder"].(map[string]any)
	if got, will := felder["sorten"], "Eiche\nBuche"; got != will {
		t.Errorf("zurückgelesen %q, wollte %q", got, will)
	}

	// Und die Höchstzahl gilt auch hier: der Schreibweg läuft durch dasselbe
	// CheckAll wie das Formular, ein Assistent kommt also nicht an einer
	// Regel vorbei, an die sich eine Person halten muss.
	zuViel, fehler := werkzeug(t, ts, key, "seite_anlegen", map[string]any{
		"website": wsID, "titel": "Zu viel", "markdown": "Text.",
		"felder": map[string]any{"sorten": "Eiche\nBuche\nEsche"},
	})
	if !fehler {
		t.Errorf("drei Werte wurden angenommen, obwohl höchstens zwei erlaubt sind: %v", zuViel)
	}
}
