package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The erasure page (Phase 33.2, plan 08). Every helper of this file starts
// with erase, so the plans beside it in this package keep theirs apart.

// eraseAdmin is the test handler with the privacy service wired as main.go
// wires it, on a German seller's website.
func eraseAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, *domain.Website) {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	s := dsr.NewService(database)
	s.SetUsers(user.NewStore(database, cheapHashing), sm)
	s.SetActivity(activity.NewStore(database))
	h.SetDataRights(s)
	h.SetActivityStore(activity.NewStore(database))
	if _, err := database.Write.Exec(`UPDATE websites SET country = 'DE' WHERE id = $1`, ws.ID); err != nil {
		t.Fatal(err)
	}
	ws.Country = "DE"
	return h, sm, database, ws
}

func erasePage(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/datenschutz/loeschen"
}

func erasedPage(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/datenschutz/geloescht"
}

// eraseClient is one browser: the session cookie travels from one request
// to the next, as the report the POST leaves for the result page does.
type eraseClient struct {
	t      *testing.T
	h      *Handler
	sm     *scs.SessionManager
	cookie *http.Cookie
	// user and role are put into the session of every request; 0 and ""
	// leave the session as the test fixture has it.
	user int64
	role string
	// fresh marks the password as just confirmed.
	fresh bool
}

// do runs the handler behind the password question, as main.go registers
// it, and returns the answer and the flashes.
func (c *eraseClient) do(fn func(http.ResponseWriter, *http.Request) error, req *http.Request) (rec *httptest.ResponseRecorder, bad, good string) {
	c.t.Helper()
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec = httptest.NewRecorder()
	var handlerErr error
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerErr = fn(w, r)
		bad = c.sm.GetString(r.Context(), "flash_error")
		good = c.sm.GetString(r.Context(), "flash_success")
	})
	guarded := auth.RequireFreshPassword(c.sm)(inner)
	c.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.user != 0 {
			c.sm.Put(r.Context(), auth.SessionKeyUserID, c.user)
		}
		if c.role != "" {
			c.sm.Put(r.Context(), auth.SessionKeyUserRole, c.role)
		}
		if c.fresh {
			auth.MarkElevated(c.sm, r.Context())
		}
		guarded.ServeHTTP(w, r)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		c.t.Fatalf("handler: %v", handlerErr)
	}
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == c.sm.Cookie.Name {
			c.cookie = ck
		}
	}
	return rec, bad, good
}

// get asks for a page of the website.
func (c *eraseClient) get(fn func(http.ResponseWriter, *http.Request) error, target string, websiteID int64) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	rec, _, _ := c.do(fn, req)
	return rec
}

// post sends the erasure form.
func (c *eraseClient) post(websiteID int64, form url.Values, htmx bool) (*httptest.ResponseRecorder, string) {
	c.t.Helper()
	req := albumPost(erasePage(websiteID), form, websiteRoute(websiteID), htmx)
	rec, bad, _ := c.do(c.h.HandlePrivacyErase, req)
	return rec, bad
}

// eraseOrderLeft reads what the order row holds after the request.
func eraseOrderLeft(t *testing.T, database *db.DB, number string) (email, erasedAt string) {
	t.Helper()
	var e string
	var at *string
	if err := database.Read.QueryRow(`SELECT email, erased_at FROM orders WHERE number = $1`, number).Scan(&e, &at); err != nil {
		t.Fatal(err)
	}
	if at != nil {
		return e, *at
	}
	return e, ""
}

func TestTheErasurePageAsksForThePassword(t *testing.T) {
	h, sm, database, ws := eraseAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0042", "ada@example.com", "Ada Lovelace")
	target := erasePage(ws.ID) + "?email=ada%40example.com"

	c := &eraseClient{t: t, h: h, sm: sm}
	rec := c.get(h.HandlePrivacyErase, target, ws.ID)
	want := auth.ConfirmPath + "?weiter=" + url.QueryEscape(target)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
		t.Fatalf("without a fresh password: %d to %q, want 303 to %q", rec.Code, rec.Header().Get("Location"), want)
	}
	if email, at := eraseOrderLeft(t, database, "2026-0042"); email == "" || at != "" {
		t.Fatal("asking for the page changed the order")
	}

	c.fresh = true
	rec = c.get(h.HandlePrivacyErase, target, ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("with a fresh password: status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"2026-0042",
		retentionSays("Promised returns policy"),
		retentionSays("Remark"),
		retentionSays("E-mail address"),
		retentionSays("An order's invoice record must stay readable until its period ends, so it is reduced, not deleted: name, company, address, items, amounts, payment state, return policy and the customer's note stay; the e-mail address and the phone number go."),
		retentionSays("Germany: booking vouchers and copies of invoices are kept for eight years (§ 147 AO, § 257 HGB, § 14b UStG)."),
		retentionSays("Kept until %s", "01.01.2035"),
		retentionSays("This cannot be undone."),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not show %q", want)
		}
	}
	if email, at := eraseOrderLeft(t, database, "2026-0042"); email == "" || at != "" {
		t.Error("showing the page changed the order")
	}
}

func TestErasingAnOrderFromThePage(t *testing.T) {
	for _, htmx := range []bool{false, true} {
		h, sm, database, ws := eraseAdmin(t)
		privacyOrder(t, database, ws.ID, "2026-0042", "ada@example.com", "Ada Lovelace")
		c := &eraseClient{t: t, h: h, sm: sm, fresh: true}

		rec, bad := c.post(ws.ID, url.Values{"email": {"ada@example.com"}, "quelle": {"orders"}}, htmx)
		if bad != "" {
			t.Fatalf("htmx %v: flash %q", htmx, bad)
		}
		if htmx {
			if rec.Header().Get("HX-Redirect") != erasedPage(ws.ID) {
				t.Fatalf("htmx: status %d, HX-Redirect %q, want %s", rec.Code, rec.Header().Get("HX-Redirect"), erasedPage(ws.ID))
			}
		} else if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != erasedPage(ws.ID) {
			t.Fatalf("status %d to %q, want 303 to %s", rec.Code, rec.Header().Get("Location"), erasedPage(ws.ID))
		}
		if email, at := eraseOrderLeft(t, database, "2026-0042"); email != "" || at == "" {
			t.Fatalf("htmx %v: the order was not reduced (email %q, erased_at %q)", htmx, email, at)
		}

		res := c.get(h.HandlePrivacyErased, erasedPage(ws.ID), ws.ID)
		if res.Code != http.StatusOK {
			t.Fatalf("the result page: status %d, want 200", res.Code)
		}
		body := res.Body.String()
		if !strings.Contains(body, "2026-0042") || !strings.Contains(body, retentionSays("Kept until %s", "01.01.2035")) {
			t.Errorf("the result page does not name the order as kept until 01.01.2035")
		}
		if !strings.Contains(body, retentionSays("One order reduced")) {
			t.Error("the result page does not count the reduced order")
		}
		if strings.Contains(body, "ada@example.com") {
			t.Error("the result page shows the erased address")
		}

		again := c.get(h.HandlePrivacyErased, erasedPage(ws.ID), ws.ID)
		if again.Code != http.StatusSeeOther || again.Header().Get("Location") != privacyScreen(ws.ID) {
			t.Errorf("the second visit: %d to %q, want 303 to the Privacy screen", again.Code, again.Header().Get("Location"))
		}
	}
}

func TestTheErasurePageHasNoConfirmAttribute(t *testing.T) {
	h, sm, database, ws := eraseAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0042", "ada@example.com", "Ada Lovelace")
	c := &eraseClient{t: t, h: h, sm: sm, fresh: true}
	body := c.get(h.HandlePrivacyErase, erasePage(ws.ID)+"?email=ada%40example.com", ws.ID).Body.String()

	if strings.Contains(body, "hx-confirm") {
		t.Error("the page carries an htmx confirmation; the page itself is the confirmation")
	}
	form := regexp.MustCompile(`<form[^>]*hx-post="([^"]*)"[^>]*>`).FindStringSubmatch(body)
	if form == nil {
		t.Fatal("the page has no erasure form with hx-post")
	}
	if !strings.Contains(form[0], `action="`+form[1]+`"`) || form[1] != erasePage(ws.ID) {
		t.Errorf("the form posts to %q with htmx and elsewhere without: %s", form[1], form[0])
	}
	if !strings.Contains(form[0], `method="post"`) {
		t.Errorf("the form is not a POST: %s", form[0])
	}
	button := regexp.MustCompile(`<button[^>]*type="submit"[^>]*>`).FindString(body[strings.Index(body, form[0]):])
	if !strings.Contains(button, `hx-disabled-elt="this"`) {
		t.Errorf("the erase button can be pressed twice: %s", button)
	}
	for _, field := range []string{`name="email" value="ada@example.com"`, `name="gorilla.csrf.Token"`, `name="quelle" value="orders"`} {
		if !strings.Contains(body, field) {
			t.Errorf("the form lacks %s", field)
		}
	}
}

func TestAnEditorWithoutThePublishingRightCannotErase(t *testing.T) {
	h, sm, database, ws := eraseAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0042", "ada@example.com", "Ada Lovelace")
	editor := editorWithoutPublishing(t, database, ws.ID)
	c := &eraseClient{t: t, h: h, sm: sm, user: editor, role: user.RoleEditor, fresh: true}

	rec, bad := c.post(ws.ID, url.Values{"email": {"ada@example.com"}, "quelle": {"orders"}}, false)
	wantRefused(t, "erasure", rec, bad, privacyScreen(ws.ID))
	if email, at := eraseOrderLeft(t, database, "2026-0042"); email == "" || at != "" {
		t.Error("an editor without the publishing right erased the order")
	}

	body := c.get(h.HandlePrivacyErase, erasePage(ws.ID)+"?email=ada%40example.com", ws.ID).Body.String()
	if !strings.Contains(body, "2026-0042") {
		t.Error("the editor does not see what an erasure would do")
	}
	if strings.Contains(body, retentionSays("Erase now")) || strings.Contains(body, `method="post"`) {
		t.Error("the page offers the editor a button that is refused")
	}
	if !strings.Contains(body, retentionSays(readOnlyText)) {
		t.Error("the page does not say why there is no button")
	}
}

func TestOnlyAnAdministratorMayEraseAnAccount(t *testing.T) {
	h, sm, database, ws := eraseAdmin(t)
	ctx := context.Background()
	users := user.NewStore(database, cheapHashing)
	staff, err := users.Create(ctx, "Ada", "ada@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}
	if err := users.SetRights(ctx, staff, user.Rights{MayPublish: true, Websites: []int64{ws.ID}}); err != nil {
		t.Fatal(err)
	}
	chef, err := users.Create(ctx, "Chef", "chef@example.com", "passwort123", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	exists := func() bool {
		u, err := users.GetByID(ctx, staff)
		return err == nil && u != nil
	}
	form := url.Values{"email": {"ada@example.com"}, "quelle": {"accounts"}}

	// An editor who may publish: the box is not offered, and a posted one is
	// ignored with a sentence.
	editor := &eraseClient{t: t, h: h, sm: sm, role: user.RoleEditor, fresh: true}
	body := editor.get(h.HandlePrivacyErase, erasePage(ws.ID)+"?email=ada%40example.com", ws.ID).Body.String()
	if strings.Contains(body, `value="accounts"`) {
		t.Error("an editor is offered the account box")
	}
	if rec, bad := editor.post(ws.ID, form, false); rec.Code != http.StatusSeeOther || bad != "" {
		t.Fatalf("the editor's erasure: %d, %q", rec.Code, bad)
	}
	if !exists() {
		t.Fatal("an editor erased a staff account")
	}
	result := editor.get(h.HandlePrivacyErased, erasedPage(ws.ID), ws.ID).Body.String()
	if !strings.Contains(result, retentionSays("Only an administrator can erase a staff account.")) {
		t.Error("the result does not say why the account stayed")
	}

	// An administrator: offered, unticked, and erased when ticked.
	admin := &eraseClient{t: t, h: h, sm: sm, user: chef, role: user.RoleAdmin, fresh: true}
	body = admin.get(h.HandlePrivacyErase, erasePage(ws.ID)+"?email=ada%40example.com", ws.ID).Body.String()
	box := regexp.MustCompile(`<input[^>]*value="accounts"[^>]*>`).FindString(body)
	if box == "" || strings.Contains(box, "checked") {
		t.Errorf("the administrator's account box: %q, want offered and not ticked", box)
	}
	if rec, bad := admin.post(ws.ID, form, false); rec.Code != http.StatusSeeOther || bad != "" {
		t.Fatalf("the administrator's erasure: %d, %q", rec.Code, bad)
	}
	if exists() {
		t.Error("the administrator's erasure left the account")
	}
}

// eraseBackupsSection is the part of a page from the backups heading on.
func eraseBackupsSection(body string) string {
	at := strings.Index(body, `id="erasure-backups"`)
	if at < 0 {
		return ""
	}
	return body[at:]
}

func TestTheResultNamesSnapshotsAndBackups(t *testing.T) {
	h, sm, database, ws := eraseAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0042", "ada@example.com", "Ada Lovelace")
	c := &eraseClient{t: t, h: h, sm: sm, fresh: true}
	preview := func() string {
		return eraseBackupsSection(c.get(h.HandlePrivacyErase, erasePage(ws.ID)+"?email=ada%40example.com", ws.ID).Body.String())
	}

	// No snapshot yet.
	if got := preview(); !strings.Contains(got, retentionSays("No pre-upgrade snapshot is in the data directory.")) {
		t.Errorf("without a snapshot the page does not say so:\n%s", got)
	}

	for _, name := range []string{"pre-upgrade-0.1.4-20261101-120000.sqlite", "pre-upgrade-0.1.5-20261110-080000.sqlite"} {
		if err := os.WriteFile(filepath.Join(h.cfg.DataDir, name), []byte("SQLite format 3"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Not a snapshot: it is not listed.
	if err := os.WriteFile(filepath.Join(h.cfg.DataDir, "notes.sqlite"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	check := func(what, section string) {
		t.Helper()
		for _, want := range []string{
			retentionSays("Backups and snapshots"),
			"pre-upgrade-0.1.4-20261101-120000.sqlite", "01.11.2026",
			"pre-upgrade-0.1.5-20261110-080000.sqlite", "10.11.2026",
			retentionSays("These pre-upgrade snapshots in the data directory still hold the erased data. They are kept until they are removed: the newest five, with no age limit."),
			retentionSays("Nightly backups made by deploy/backup.sh still hold the erased data until they age out: after 30 days, unless RETENTION_DAYS says otherwise. An off-site copy follows its own schedule. Backups are used only to restore."),
			retentionSays("After restoring a backup older than today, repeat this erasure."),
			retentionSays("Mail already sent stays in the recipient's mailbox and with the mail provider."),
		} {
			if !strings.Contains(section, want) {
				t.Errorf("%s does not show %q", what, want)
			}
		}
		if strings.Index(section, "pre-upgrade-0.1.5") > strings.Index(section, "pre-upgrade-0.1.4") {
			t.Errorf("%s does not list the newest snapshot first", what)
		}
		if strings.Contains(section, "notes.sqlite") || strings.Contains(section, retentionSays("No pre-upgrade snapshot is in the data directory.")) {
			t.Errorf("%s lists what is not a snapshot", what)
		}
	}
	check("the preview", preview())

	if rec, bad := c.post(ws.ID, url.Values{"email": {"ada@example.com"}, "quelle": {"orders"}}, false); rec.Code != http.StatusSeeOther || bad != "" {
		t.Fatalf("erase: %d %q", rec.Code, bad)
	}
	check("the result", eraseBackupsSection(c.get(h.HandlePrivacyErased, erasedPage(ws.ID), ws.ID).Body.String()))
}
