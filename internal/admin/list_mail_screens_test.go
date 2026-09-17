package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The list's own furniture — which columns a person sees and which filters they
// have given a name to — and the outbox screen.
//
// Both belong to a PERSON rather than to a website, which is the property worth
// holding: a number in an address must not reach anybody else's view.
//
// Driven red by twelve mutations, all twelve caught on the first pass — the
// first slice of this phase where nothing had to be chased. The reason is worth
// naming, because it is the counterpart of every other slice's lesson: every
// assertion here is on the sentence the operator reads or the row that is
// stored, and none on "something happened".

func TestTheColumnsAPersonChoosesAreTheirsAndBounded(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	users := user.NewStore(database, cheapHashing)
	me, err := users.Create(ctx, "Ich", "ich@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}

	stored := func() string {
		var s string
		if err := database.Read.QueryRowContext(ctx,
			`SELECT COALESCE(page_columns, '') FROM users WHERE id = $1`, me).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// Only known keys survive, in the fixed order, whatever the form sends:
	// what comes back is a form and is therefore anything at all.
	rec, _ := serveAs(t, h, sm, me, h.HandlePageColumns, postForm(
		"/admin/websites/1/spalten",
		url.Values{"spalten": {"gibtesnicht", columnNames[1].Key, columnNames[0].Key}},
		websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if got, want := stored(), columnNames[0].Key+","+columnNames[1].Key; got != want {
		t.Errorf("stored %q, want %q — unknown keys dropped and the order fixed", got, want)
	}

	// Nothing ticked is a legitimate answer — a list of titles and nothing else
	// — and it is stored as a marker so it does not read as "never chose".
	rec, _ = serveAs(t, h, sm, me, h.HandlePageColumns,
		postForm("/admin/websites/1/spalten", url.Values{}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("nothing ticked: status %d", rec.Code)
	}
	if got := stored(); got != "-" {
		t.Errorf("nothing ticked stored %q, want the marker", got)
	}

	// Without a signed-in person there is nobody whose columns these are.
	rec = serve(t, h, sm, h.HandlePageColumns,
		postForm("/admin/websites/1/spalten", url.Values{}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusNotFound {
		t.Errorf("without a person: status %d, want 404", rec.Code)
	}
}

func TestASavedViewIsRememberedCorrectedAndForgotten(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	users := user.NewStore(database, cheapHashing)
	me, err := users.Create(ctx, "Ich", "ich@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}

	views := func(userID int64) []savedView {
		return h.savedViews(ctx, userID, ws.ID, "")
	}

	rec, flash := serveAs(t, h, sm, me, h.HandleSavedViewCreate, postForm(
		"/admin/websites/1/ansichten",
		url.Values{"name": {"  Meine Entwürfe  "}, "filter": {"?status=draft"}},
		websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, flash)
	}
	mine := views(me)
	if len(mine) != 1 {
		t.Fatalf("%d views, want 1", len(mine))
	}
	if mine[0].Name != "Meine Entwürfe" {
		t.Errorf("the name was not trimmed: %q", mine[0].Name)
	}
	if !strings.Contains(mine[0].Query, "status=draft") {
		t.Errorf("the filter was not remembered: %q", mine[0].Query)
	}

	// The same name twice CORRECTS rather than doubling: a second chip with the
	// same label helps nobody.
	serveAs(t, h, sm, me, h.HandleSavedViewCreate, postForm("/admin/websites/1/ansichten",
		url.Values{"name": {"Meine Entwürfe"}, "filter": {"?status=published"}},
		websiteRoute(ws.ID)))
	mine = views(me)
	if len(mine) != 1 {
		t.Fatalf("%d views after correcting one, want 1", len(mine))
	}
	if !strings.Contains(mine[0].Query, "status=published") {
		t.Errorf("the correction did not arrive: %q", mine[0].Query)
	}

	// Refused: no name, and a "view" that is the whole list.
	for _, c := range []struct{ what, name, filter, want string }{
		{"no name", "  ", "?status=draft", "Please give the view a name"},
		{"the whole list", "Alles", "", "This view is the whole list — set a filter first."},
	} {
		_, flash := serveAs(t, h, sm, me, h.HandleSavedViewCreate,
			postForm("/admin/websites/1/ansichten",
				url.Values{"name": {c.name}, "filter": {c.filter}}, websiteRoute(ws.ID)))
		if flash != c.want {
			t.Errorf("%s: answered %q, want %q", c.what, flash, c.want)
		}
	}
	if len(views(me)) != 1 {
		t.Errorf("a refused view was stored anyway")
	}

	// A long name is cut rather than refused.
	serveAs(t, h, sm, me, h.HandleSavedViewCreate, postForm("/admin/websites/1/ansichten",
		url.Values{"name": {strings.Repeat("a", 60)}, "filter": {"?status=draft"}},
		websiteRoute(ws.ID)))
	for _, v := range views(me) {
		if len([]rune(v.Name)) > 40 {
			t.Errorf("a name of %d characters was stored", len([]rune(v.Name)))
		}
	}

	// And forgotten again.
	first := views(me)[0]
	rec, _ = serveAs(t, h, sm, me, h.HandleSavedViewDelete,
		postForm("/admin/websites/1/ansichten/1/delete", nil,
			websiteRoute(ws.ID, "viewID", strconv.FormatInt(first.ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: status %d", rec.Code)
	}
	for _, v := range views(me) {
		if v.ID == first.ID {
			t.Error("the view is still there")
		}
	}
}

// A view belongs to one person, and the user id in the delete's condition is
// the whole access check.
func TestOneSavedViewCannotBeDeletedByAnotherPerson(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	users := user.NewStore(database, cheapHashing)
	mine, err := users.Create(ctx, "Ich", "ich@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := users.Create(ctx, "Andere", "andere@example.com", "passwort123", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	serveAs(t, h, sm, mine, h.HandleSavedViewCreate, postForm("/admin/websites/1/ansichten",
		url.Values{"name": {"Meine"}, "filter": {"?status=draft"}}, websiteRoute(ws.ID)))
	own := h.savedViews(ctx, mine, ws.ID, "")
	if len(own) != 1 {
		t.Fatalf("%d views", len(own))
	}

	// Somebody else, an administrator at that, aiming at the id.
	rec, _ := serveAs(t, h, sm, theirs, h.HandleSavedViewDelete,
		postForm("/admin/websites/1/ansichten/1/delete", nil,
			websiteRoute(ws.ID, "viewID", strconv.FormatInt(own[0].ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if len(h.savedViews(ctx, mine, ws.ID, "")) != 1 {
		t.Error("somebody else's view was deleted through an id in an address")
	}
	// And they cannot see it either.
	if len(h.savedViews(ctx, theirs, ws.ID, "")) != 0 {
		t.Error("another person's views are on this person's list")
	}
}

// The outbox screen, and the two buttons on it.
func TestTheMailScreenSaysWhatItCanAndCannotDo(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)

	rec := serve(t, h, sm, h.HandleMailStatus,
		httptest.NewRequest(http.MethodGet, "/admin/mail", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	// Without a mail server the test button says so, and says which two
	// settings are missing — a message that only said "not possible" would
	// leave the operator reading the deployment guide.
	rec, bad, _ := albumFlashWithEmail(t, h, sm, "chefin@example.com", h.HandleMailTest,
		postForm("/admin/mail/test", nil, nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("test: status %d", rec.Code)
	}
	if !strings.Contains(bad, "HOLZCLOUD_SMTP_HOST") {
		t.Errorf("the refusal does not name what is missing: %q", bad)
	}

	// Without an address on the account there is nowhere to send it, and that
	// is a different sentence.
	rec, bad, _ = albumFlashWithEmail(t, h, sm, "", h.HandleMailTest,
		postForm("/admin/mail/test", nil, nil))
	if bad != "No address is on file for your account." {
		t.Errorf("without an address: answered %q", bad)
	}

	// Retrying an empty queue says so rather than claiming to have done
	// something.
	rec, _, good := albumFlashWithEmail(t, h, sm, "chefin@example.com", h.HandleMailRetry,
		postForm("/admin/mail/retry", nil, nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("retry: status %d", rec.Code)
	}
	if good != "Nothing is waiting any more." {
		t.Errorf("retrying an empty queue said %q", good)
	}
}

// albumFlashWithEmail runs a handler as somebody with (or without) an address
// on file, which is what the mail screen asks about.
func albumFlashWithEmail(t *testing.T, h *Handler, sm *scs.SessionManager, email string,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request,
) (rec *httptest.ResponseRecorder, bad, good string) {
	t.Helper()
	rec = httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if email != "" {
			sm.Put(r.Context(), auth.SessionKeyUserEmail, email)
		}
		handlerErr = fn(w, r)
		bad = sm.GetString(r.Context(), auth.SessionKeyFlashError)
		good = sm.GetString(r.Context(), auth.SessionKeyFlashSuccess)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec, bad, good
}
