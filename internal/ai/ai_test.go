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

// setUp builds a server with one website and hands back both keys.
func setUp(t *testing.T) (*httptest.Server, *db.DB, int64, string, string) {
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

// call sends a JSON-RPC message and hands back the answer.
func call(t *testing.T, ts *httptest.Server, key, method string, params any) map[string]any {
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

// callTool calls a tool and hands back the unwrapped result.
func callTool(t *testing.T, ts *httptest.Server, key, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res := call(t, ts, key, "tools/call", map[string]any{"name": name, "arguments": args})
	if res["error"] != nil {
		t.Fatalf("%s: protocol error %v", name, res["error"])
	}
	result, _ := res["result"].(map[string]any)
	failed, _ := result["isError"].(bool)

	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("%s: keine Antwort", name)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if failed {
		return map[string]any{"text": text}, true
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s: %q is not JSON: %v", name, text, err)
	}
	return out, false
}

// Without a key nobody gets in. This is the one check everything else rests on:
// if it falls, every further boundary in this file is beside the point.
func TestARequestWithoutAKeyIsRefused(t *testing.T) {
	ts, _, _, key, _ := setUp(t)

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

// An expired key is a dead key.
func TestAnExpiredKeyIsRefused(t *testing.T) {
	database := newTestDB(t)
	tokens := NewStore(database)
	secret, tok, err := tokens.Issue(context.Background(), "alt", 0, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// Move the expiry into the past rather than wait an hour.
	if _, err := database.Write.Exec(`UPDATE ai_tokens SET expires_at = $1 WHERE id = $2`,
		time.Now().UTC().Add(-time.Minute).Format(timeLayout), tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Verify(context.Background(), secret); err != ErrExpired {
		t.Errorf("Verify = %v, want %v", err, ErrExpired)
	}
}

// A revoked key takes effect at once and not at the next restart.
func TestARevokedKeyIsRefused(t *testing.T) {
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

// The key itself must stand nowhere in the database. Whoever gets a copy of it
// should be able to do nothing with that.
func TestOnlyTheFingerprintIsStored(t *testing.T) {
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
		t.Error("the key is in the database in clear text")
	}
}

// A read-only key does not even get to see the writing tools — and if it calls
// them anyway, it is refused. Both, because the one is a convenience and the
// other is the boundary.
func TestAReadOnlyKeySeesAndMayNotWrite(t *testing.T) {
	ts, _, wsID, _, lesen := setUp(t)

	res := call(t, ts, lesen, "tools/list", nil)
	result := res["result"].(map[string]any)
	for _, raw := range result["tools"].([]any) {
		name := raw.(map[string]any)["name"].(string)
		if strings.HasPrefix(name, "create_page") || strings.HasPrefix(name, "update_page") ||
			strings.HasPrefix(name, "publish_page") {
			t.Errorf("a read-only key sees %q", name)
		}
	}

	_, failed := callTool(t, ts, lesen, "create_page", map[string]any{
		"website": wsID, "title": "Verboten", "markdown": "Text",
	})
	if !failed {
		t.Error("a read-only key was able to create a page")
	}
}

// A key for one website must not see the other.
func TestAKeyStaysWithItsOwnWebsite(t *testing.T) {
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

	list, failed := callTool(t, ts, secret, "list_websites", nil)
	if failed {
		t.Fatalf("list_websites: %v", list["text"])
	}
	sites := list["websites"].([]any)
	if len(sites) != 1 {
		t.Fatalf("%d Websites sichtbar, erwartet 1", len(sites))
	}

	if _, failed := callTool(t, ts, secret, "create_page", map[string]any{
		"website": zwei.ID, "title": "Fremd", "markdown": "Text",
	}); !failed {
		t.Error("it was possible to write on somebody else's website")
	}
}

// The whole way an assistant goes: connect, fetch the tools, create a page,
// change it and publish it at the end.
func TestTheWholeWayThrough(t *testing.T) {
	ts, database, wsID, key, _ := setUp(t)

	res := call(t, ts, key, "initialize", map[string]any{})
	if got := res["result"].(map[string]any)["protocolVersion"]; got != ProtocolVersion {
		t.Errorf("protocolVersion = %v, want %v", got, ProtocolVersion)
	}

	angelegt, failed := callTool(t, ts, key, "create_page", map[string]any{
		"website": wsID, "title": "Unsere Schafe", "markdown": "# Hallo\n\nText.",
	})
	if failed {
		t.Fatalf("create_page: %v", angelegt["text"])
	}
	// Everything new is a draft. This is the rule most hangs on: an assistant
	// that publishes by accident puts something half-finished on the net, and
	// that is noticed only once somebody has read it.
	if angelegt["status"] != "draft" {
		t.Fatalf("status = %v, want draft", angelegt["status"])
	}
	id := int64(angelegt["id"].(float64))

	gelesen, _ := callTool(t, ts, key, "read_page", map[string]any{"id": id})
	if !strings.Contains(gelesen["markdown"].(string), "Hallo") {
		t.Errorf("markdown = %q", gelesen["markdown"])
	}

	changed, failed := callTool(t, ts, key, "update_page", map[string]any{
		"id": id, "markdown": "# Hallo\n\nMehr Text.",
	})
	if failed {
		t.Fatalf("update_page: %v", changed["text"])
	}
	// Changing is not publishing.
	if changed["status"] != "draft" {
		t.Errorf("after the change status = %v, want draft", changed["status"])
	}

	// The old state is kept as a revision, exactly as with a change made from
	// the admin side — otherwise a text an AI overwrites would be gone.
	fassungen, err := page.NewStore(database).ListRevisions(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(fassungen) == 0 {
		t.Error("the change left no version behind")
	}

	oeffentlich, failed := callTool(t, ts, key, "publish_page", map[string]any{
		"id": id, "status": "published",
	})
	if failed {
		t.Fatalf("publish_page: %v", oeffentlich["text"])
	}
	if oeffentlich["status"] != "published" {
		t.Errorf("status = %v, want published", oeffentlich["status"])
	}
}

// A page made of blocks must not be replaced by markdown. Without this bar an
// assistant writes its text into it and the build of the page is silently lost.
func TestABlockPageIsNotOverwritten(t *testing.T) {
	ts, database, wsID, key, _ := setUp(t)

	pages := page.NewStore(database)
	p, err := pages.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: wsID, Title: "Bausteine", Slug: "bausteine",
		Markdown: "", HTML: "<p>gebaut</p>", Status: "draft",
		Blocks: `[{"kind":"text","text":"Hallo"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}

	gelesen, _ := callTool(t, ts, key, "read_page", map[string]any{"id": p.ID})
	if gelesen["built_from"] != "blocks" {
		t.Errorf("built_from = %v, want blocks", gelesen["built_from"])
	}

	answer, failed := callTool(t, ts, key, "update_page", map[string]any{
		"id": p.ID, "markdown": "alles neu",
	})
	if !failed {
		t.Fatal("the blocks were overwritten")
	}
	if !strings.Contains(answer["text"].(string), "blocks") {
		t.Errorf("reason = %q", answer["text"])
	}

	// Changing only the title has to keep working all the same.
	if _, failed := callTool(t, ts, key, "update_page", map[string]any{
		"id": p.ID, "title": "Neuer Titel",
	}); failed {
		t.Error("the title of a block page could not be changed")
	}
}

// GET is the request a client makes when it expects an event stream. A clear
// refusal beats a 404, which looks like a wrong address.
func TestOnlyPostIsAccepted(t *testing.T) {
	ts, _, _, key, _ := setUp(t)
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

// A notification has no id and wants no answer. Whoever answers one makes some
// clients complain.
func TestANotificationGetsNoAnswer(t *testing.T) {
	ts, _, _, key, _ := setUp(t)
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

// Every tool needs a schema, or the assistant guesses the arguments.
func TestEveryToolHasASchema(t *testing.T) {
	for _, w := range Tools(Deps{}) {
		if w.Description == "" {
			t.Errorf("%s hat keine Beschreibung", w.Name)
		}
		if w.InputSchema.Type != "object" {
			t.Errorf("%s: Schema-Typ %q", w.Name, w.InputSchema.Type)
		}
		for _, required := range w.InputSchema.Required {
			if _, ok := w.InputSchema.Properties[required]; !ok {
				t.Errorf("%s demands %q but does not describe it", w.Name, required)
			}
		}
	}
}

// --- What an assistant learns about the new field properties --------------

// setUpWithFields is setUp, only with the website's own fields along. A server
// without Fields reports no fields at all — right for a build without them,
// useless for these checks.
func setUpWithFields(t *testing.T) (*httptest.Server, *field.Store, int64, string) {
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

// fieldNamed finds one field in the answer of list_fields.
func fieldNamed(t *testing.T, list []any, key string) map[string]any {
	t.Helper()
	for _, raw := range list {
		e, _ := raw.(map[string]any)
		if e["key"] == key {
			return e
		}
	}
	t.Fatalf("no field %q in %v", key, list)
	return nil
}

// An assistant writes into the fields this list describes. If the maximum is
// not there, it writes three values into a field that takes two; if the
// spelling is not there, it writes "Mo, Di" into one that expects one line per
// value. Both are refused, and the assistant learns the reason only afterwards
// — the list is the place where it stands beforehand.
func TestListFieldsDescribesTheNewProperties(t *testing.T) {
	ts, fields, wsID, key := setUpWithFields(t)
	ctx := context.Background()

	create := func(d field.Def) field.Def {
		t.Helper()
		d.WebsiteID = wsID
		got, err := fields.Create(ctx, d)
		if err != nil {
			t.Fatalf("Feld %q anlegen: %v", d.Key, err)
		}
		return *got
	}

	create(field.Def{Key: "sorten", Label: "Sorten", Kind: field.KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"}, MaxValues: 2})
	create(field.Def{Key: "menge", Label: "Menge", Kind: field.KindRange,
		RangeMin: "1", RangeMax: "10"})
	create(field.Def{Key: "stil", Label: "Stil", Kind: field.KindChoice,
		Choices: []string{"hell", "dunkel"}, Display: field.DisplayButtons})
	create(field.Def{Key: "form", Label: "Form", Kind: field.KindChoice,
		Choices: []string{"rund", "eckig"}})
	create(field.Def{Key: "herkunft", Label: "Herkunft", Kind: field.KindText})
	group := create(field.Def{Key: "zeiten", Label: "Zeiten", Kind: field.KindGroup})
	create(field.Def{ParentID: group.ID, Key: "tage", Label: "Tage", Kind: field.KindMulti,
		Choices: []string{"Mo", "Di", "Mi"}, MaxValues: 2})
	create(field.Def{ParentID: group.ID, Key: "von", Label: "Von", Kind: field.KindRange,
		RangeMin: "0", RangeMax: "24"})

	res, failed := callTool(t, ts, key, "list_fields", map[string]any{"website": wsID})
	if failed {
		t.Fatalf("list_fields: %v", res["text"])
	}
	list, _ := res["fields"].([]any)
	if len(list) == 0 {
		t.Fatalf("no fields reported: %v", res)
	}

	// The maximum is there, and as a number at that.
	sorten := fieldNamed(t, list, "sorten")
	if got, will := sorten["max_values"], float64(2); got != will {
		t.Errorf("max_werte = %v (%T), wollte %v", got, got, will)
	}
	// And the one line that says how several values are written.
	note, _ := sorten["multiple_values"].(string)
	if note == "" {
		t.Errorf("the multi-valued field does not say how its value is written: %v", sorten)
	}

	// Both bounds, each on its own.
	quantity := fieldNamed(t, list, "menge")
	if quantity["min_value"] != "1" || quantity["max_value"] != "10" {
		t.Errorf("the bounds are missing or wrong: %v", quantity)
	}

	// The presentation only where it differs from the dropdown that every
	// existing field is.
	if got := fieldNamed(t, list, "stil")["presentation"]; got != field.DisplayButtons {
		t.Errorf("darstellung = %v, wollte %q", got, field.DisplayButtons)
	}
	if _, da := fieldNamed(t, list, "form")["presentation"]; da {
		t.Error("the drop-down reports a display although it is the ordinary one")
	}

	// An ordinary text field carries none of it: a reported zero would read as
	// "none allowed", and a reported empty bound as "the bound is empty".
	herkunft := fieldNamed(t, list, "herkunft")
	for _, schluessel := range []string{"presentation", "max_values", "min_value", "max_value", "multiple_values"} {
		if _, da := herkunft[schluessel]; da {
			t.Errorf("das Textfeld meldet %q: %v", schluessel, herkunft)
		}
	}

	// And the same one level down: an assistant that learns less about a
	// subfield is an assistant that writes into exactly that one wrongly.
	unter, _ := fieldNamed(t, list, "zeiten")["subfields"].([]any)
	if len(unter) == 0 {
		t.Fatalf("die Gruppe meldet keine Unterfelder: %v", fieldNamed(t, list, "zeiten"))
	}
	tage := fieldNamed(t, unter, "tage")
	if got, will := tage["max_values"], float64(2); got != will {
		t.Errorf("das Unterfeld meldet max_werte = %v, wollte %v", got, will)
	}
	if h, _ := tage["multiple_values"].(string); h == "" {
		t.Errorf("the multi-valued sub-field does not say how its value is written: %v", tage)
	}
	von := fieldNamed(t, unter, "von")
	if von["min_value"] != "0" || von["max_value"] != "24" {
		t.Errorf("the sub-field does not report the bounds: %v", von)
	}
}

// The round trip: written the way the note describes it, and read back
// unchanged. This is the only assurance here that would expose a note
// describing a spelling the writing path does not take.
func TestAMultiValuedFieldGoesThroughTheToolsAndBack(t *testing.T) {
	ts, fieldStore, wsID, key := setUpWithFields(t)
	if _, err := fieldStore.Create(context.Background(), field.Def{
		WebsiteID: wsID, Key: "sorten", Label: "Sorten", Kind: field.KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"}, MaxValues: 2,
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	angelegt, failed := callTool(t, ts, key, "create_page", map[string]any{
		"website": wsID, "title": "Bretter", "markdown": "Text.",
		"fields": map[string]any{"sorten": "Eiche\nBuche"},
	})
	if failed {
		t.Fatalf("create_page: %v", angelegt["text"])
	}
	id := int64(angelegt["id"].(float64))

	gelesen, failed := callTool(t, ts, key, "read_page", map[string]any{"id": id})
	if failed {
		t.Fatalf("read_page: %v", gelesen["text"])
	}
	fields, _ := gelesen["fields"].(map[string]any)
	if got, will := fields["sorten"], "Eiche\nBuche"; got != will {
		t.Errorf("read back %q, wanted %q", got, will)
	}

	// And the maximum holds here too: the writing path runs through the same
	// CheckAll as the form, so an assistant does not get past a rule a person
	// has to keep.
	zuViel, failed := callTool(t, ts, key, "create_page", map[string]any{
		"website": wsID, "title": "Zu viel", "markdown": "Text.",
		"fields": map[string]any{"sorten": "Eiche\nBuche\nEsche"},
	})
	if !failed {
		t.Errorf("three values were accepted although at most two are allowed: %v", zuViel)
	}
}
