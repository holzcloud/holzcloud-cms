package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The Privacy screen (Phase 33.2, plan 01). Every helper of this file starts
// with privacy, so the plans that follow in this package keep theirs apart.

// privacyAdmin is the test handler with the search and the activity log
// wired the way main.go wires them.
func privacyAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, *domain.Website) {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	h.SetDataRights(dsr.NewService(database))
	h.SetActivityStore(activity.NewStore(database))
	return h, sm, database, ws
}

// privacyOrder stores an order the way the shop writes one.
func privacyOrder(t *testing.T, database *db.DB, websiteID int64, number, email, name string) {
	t.Helper()
	res, err := database.Write.Exec(`INSERT INTO orders (website_id, number, token, audience, currency,
		email, name, street, postal_code, city,
		items_net, items_tax, items_gross, total_net, total_tax, total_gross,
		created_at, updated_at)
		VALUES ($1, $2, $3, 'private', 'CHF', $4, $5, 'Weg 1', '3000', 'Bern',
		2500, 0, 2500, 2500, 0, 2500, '2026-04-01T08:00:00Z', '2026-04-01T08:00:00Z')`,
		websiteID, number, "tok-"+number, email, name)
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}
	id, _ := res.LastInsertId()
	if _, err := database.Write.Exec(`INSERT INTO order_items (order_id, title, quantity, tax_bp,
		unit_gross, unit_net, line_net, line_tax, line_gross) VALUES ($1, 'Brett', 1, 0, 2500, 2500, 2500, 0, 2500)`, id); err != nil {
		t.Fatalf("insert order item: %v", err)
	}
}

func privacyScreen(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/datenschutz"
}

// privacyGet calls a handler of the screen with the path value set.
func privacyGet(t *testing.T, h *Handler, sm *scs.SessionManager, fn func(http.ResponseWriter, *http.Request) error, target string, websiteID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return serve(t, h, sm, fn, req)
}

func TestThePrivacyScreenFindsAnOrder(t *testing.T) {
	h, sm, database, ws := privacyAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0042", "Ada@Example.COM", "Ada Lovelace")

	rec := privacyGet(t, h, sm, h.HandlePrivacyPerson, privacyScreen(ws.ID)+"?email=ada@example.com", ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if want := `href="/admin/websites/` + strconv.FormatInt(ws.ID, 10) + `/bestellungen/2026-0042"`; !strings.Contains(body, want) {
		t.Errorf("the screen does not link the order: want %s", want)
	}
	if !strings.Contains(body, "Ada Lovelace") {
		t.Error("the screen does not show the customer's name")
	}
	if !strings.Contains(body, "datenschutz/export.json?email=ada%40example.com") {
		t.Error("the screen has no link to the JSON download of this address")
	}

	empty := privacyGet(t, h, sm, h.HandlePrivacyPerson, privacyScreen(ws.ID), ws.ID)
	if empty.Code != http.StatusOK {
		t.Fatalf("without an address: status %d, want 200", empty.Code)
	}
	eb := empty.Body.String()
	if !strings.Contains(eb, `name="email"`) {
		t.Error("without an address the screen has no search form")
	}
	if strings.Contains(eb, "export.json") || strings.Contains(eb, "Ada Lovelace") {
		t.Error("without an address the screen shows a result")
	}
}

func TestThePrivacyScreenRefusesWhatIsNotAnAddress(t *testing.T) {
	h, sm, _, ws := privacyAdmin(t)
	rec := privacyGet(t, h, sm, h.HandlePrivacyPerson, privacyScreen(ws.ID)+"?email=nobody", ws.ID)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "form-error") {
		t.Error("the screen does not say what is wrong with the address")
	}
}

func TestThePrivacyExportIsADownload(t *testing.T) {
	h, sm, database, ws := privacyAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0043", "ada@example.com", "Ada Lovelace")

	rec := privacyGet(t, h, sm, h.HandlePrivacyExport, privacyScreen(ws.ID)+"/export.json?email=ADA@example.com", ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	today := time.Now().UTC().Format("2006-01-02")
	want := `attachment; filename="personal-data-` + strconv.FormatInt(ws.ID, 10) + "-" + today + `.json"`
	if cd := rec.Header().Get("Content-Disposition"); cd != want {
		t.Errorf("Content-Disposition = %q, want %q", cd, want)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	var got dsr.Export
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("the body is not JSON: %v", err)
	}
	if orders := got.Records["orders"]; len(orders) != 1 || orders[0].Key != "2026-0043" {
		t.Errorf("records.orders = %+v, want the order 2026-0043", got.Records["orders"])
	}

	for _, bad := range []string{"", "?email=", "?email=nobody", "?email=ada@example.com%3Fbcc%3Dx"} {
		rec := privacyGet(t, h, sm, h.HandlePrivacyExport, privacyScreen(ws.ID)+"/export.json"+bad, ws.ID)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400", bad, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("%q: the refusal carries a body: %q", bad, rec.Body.String())
		}
	}
}

// privacyDrive runs one request through the chain main.go builds: the
// session with the given account, RequireWebsiteAccess, and the two routes
// registered as main.go registers them.
func privacyDrive(t *testing.T, h *Handler, sm *scs.SessionManager, database *db.DB, userID int64, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/websites/{id}/datenschutz", h.ErrHandler(h.HandlePrivacyPerson))
	mux.HandleFunc("GET /admin/websites/{id}/datenschutz/export.json", h.ErrHandler(h.HandlePrivacyExport))
	guarded := auth.RequireWebsiteAccess(sm, NewWebsiteAccessLookup(database))(mux)
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		guarded.ServeHTTP(w, r)
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestThePrivacyScreenOfAnotherWebsiteShowsNothing(t *testing.T) {
	h, sm, database, a := privacyAdmin(t)
	ctx := context.Background()
	b, err := h.domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	privacyOrder(t, database, a.ID, "2026-0501", "ada@example.com", "Ada auf A")
	privacyOrder(t, database, b.ID, "2026-0502", "ada@example.com", "Ada auf B")

	users := user.NewStore(database, cheapHashing)
	editorID, err := users.Create(ctx, "Nur B", "nur-b@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if err := users.SetRights(ctx, editorID, user.Rights{MayPublish: true, Websites: []int64{b.ID}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}

	for _, target := range []string{
		privacyScreen(a.ID) + "?email=ada@example.com",
		privacyScreen(a.ID) + "/export.json?email=ada@example.com",
	} {
		rec := privacyDrive(t, h, sm, database, editorID, target)
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d for an editor of B only, want a refusal", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "Ada auf A") || strings.Contains(rec.Body.String(), "2026-0501") {
			t.Errorf("%s: the refusal shows website A's order", target)
		}
	}

	// The negative control: the same editor on B gets B's order and nothing
	// of A's, through the same chain.
	for _, target := range []string{
		privacyScreen(b.ID) + "?email=ada@example.com",
		privacyScreen(b.ID) + "/export.json?email=ada@example.com",
	} {
		rec := privacyDrive(t, h, sm, database, editorID, target)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d, want 200", target, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "2026-0502") {
			t.Errorf("%s: B's order is missing", target)
		}
		if strings.Contains(body, "Ada auf A") || strings.Contains(body, "2026-0501") {
			t.Errorf("%s: B's screen shows website A's order", target)
		}
	}
}

func TestThePrivacyScreenNamesWhatItCouldNotSearch(t *testing.T) {
	h, sm, database, ws := privacyAdmin(t)
	if _, err := database.Write.Exec(`INSERT INTO plugins (id, name, version, manifest, sha256, installed_at, updated_at)
		VALUES ('kontaktformular', 'Kontaktformular', '1.0.0', '{}', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := database.Write.Exec(`INSERT INTO plugin_store (plugin_id, website_id, key, value, updated_at)
			VALUES ('kontaktformular', $1, $2, '{}', '2026-01-01T00:00:00Z')`, ws.ID, "nachricht:"+strconv.Itoa(i)); err != nil {
			t.Fatal(err)
		}
	}

	rec := privacyGet(t, h, sm, h.HandlePrivacyPerson, privacyScreen(ws.ID)+"?email=ada@example.com", ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, optoutSays("Not searched")) {
		t.Fatal("the screen has no Not searched section")
	}
	section := body[strings.Index(body, optoutSays("Not searched")):]
	if !strings.Contains(section, "Kontaktformular") {
		t.Error("the plugin's name is not under Not searched")
	}
	if !strings.Contains(section, optoutSays("4 entries")) && !strings.Contains(section, ">4 ") {
		t.Error("the plugin's entry count is not under Not searched")
	}
	if !strings.Contains(section, optoutSays("The plugin is not running on this server.")) {
		t.Error("the screen does not say why the plugin was not searched")
	}
	if strings.Index(body, optoutSays("Not searched")) > strings.Index(body, optoutSays("Download as JSON")) {
		t.Error("what was not searched comes after the result; it has to come first")
	}

	// The export says it too.
	exp := privacyGet(t, h, sm, h.HandlePrivacyExport, privacyScreen(ws.ID)+"/export.json?email=ada@example.com", ws.ID)
	var got dsr.Export
	if err := json.Unmarshal(exp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.NotCovered) != 1 || got.NotCovered[0].Plugin != "kontaktformular" || got.NotCovered[0].Entries != 4 {
		t.Errorf("not_covered = %+v", got.NotCovered)
	}
}

// T-33.2-05: the log records that a person's data was handed out, and how
// much, never whose. The row outlives an erasure, and a row that kept the
// address would keep exactly what was erased.
func TestAnExportLeavesNoAddressInTheLog(t *testing.T) {
	h, sm, database, ws := privacyAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0701", "ada.lovelace@example.com", "Ada Lovelace")
	privacyOrder(t, database, ws.ID, "2026-0702", "Ada.Lovelace@example.com", "Ada Lovelace")

	rec := privacyGet(t, h, sm, h.HandlePrivacyExport, privacyScreen(ws.ID)+"/export.json?email=ada.lovelace@example.com", ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}

	rows, err := database.Read.Query(`SELECT action, COALESCE(website_id, 0), CAST(id AS TEXT), COALESCE(CAST(user_id AS TEXT), ''),
		actor_email, entity_type, CAST(entity_id AS TEXT), metadata, created_at
		FROM activity_log WHERE action = $1`, activity.ActionPrivacyExport)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
		var action string
		var websiteID int64
		var id, userID, actor, entityType, entityID, metadata, created string
		if err := rows.Scan(&action, &websiteID, &id, &userID, &actor, &entityType, &entityID, &metadata, &created); err != nil {
			t.Fatal(err)
		}
		if websiteID != ws.ID {
			t.Errorf("the entry is for website %d, want %d", websiteID, ws.ID)
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
			t.Fatalf("metadata %q: %v", metadata, err)
		}
		if meta["records"] != float64(2) {
			t.Errorf("metadata = %s, want records = 2", metadata)
		}
		for _, col := range []string{action, id, userID, actor, entityType, entityID, metadata, created} {
			if strings.Contains(strings.ToLower(col), "lovelace") {
				t.Errorf("a column of the log entry carries the address: %q", col)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d privacy.export entries, want 1", n)
	}

	// Looking at the screen is not handing data out, and writes nothing.
	privacyGet(t, h, sm, h.HandlePrivacyPerson, privacyScreen(ws.ID)+"?email=ada.lovelace@example.com", ws.ID)
	var total int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM activity_log WHERE action = $1`, activity.ActionPrivacyExport).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("%d entries after viewing the screen, want still 1", total)
	}
}

// 33.1 D-16 carried over: a person's data stays in the admin. No assistant
// tool searches by address or reads a privacy request.
func TestThePrivacyScreenIsNotOverMCP(t *testing.T) {
	h, _, _, _ := newTestAdmin(t)
	tools := ai.Tools(ai.Deps{
		Domains: h.domains, Pages: h.pages, Media: h.mediaStore, Fields: h.fields, Ops: h,
		Limits: ai.Limits{DataDir: h.cfg.DataDir},
	})
	if len(tools) < 10 {
		t.Fatalf("%d tools; the registry was not built, the guard would prove nothing", len(tools))
	}
	for _, tool := range tools {
		name := strings.ToLower(tool.Name)
		for _, bad := range []string{"person", "personal", "privacy_request"} {
			if strings.Contains(name, bad) {
				t.Errorf("tool %q names %q: a person's data is not exposed over MCP", tool.Name, bad)
			}
		}
	}
}
