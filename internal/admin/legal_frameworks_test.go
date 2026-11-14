package admin

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// LEG-01: the legal frameworks a website follows are its own (D-04), start as
// EU + CH (D-05), may be empty (D-16), and no settings writer that does not
// mean to change them — a form without the group, a partial assistant update —
// resets them on the way past.

// legalSetBox matches one checkbox of the group, whatever order its
// attributes stand in.
var legalSetBox = regexp.MustCompile(`<input[^>]*name="legal_frameworks"[^>]*>`)

var legalSetValue = regexp.MustCompile(`value="([a-z]+)"`)

// legalSetTicked reads the settings screen and answers which boxes are ticked,
// keyed by framework id. Every box of the group is in the map.
func legalSetTicked(t *testing.T, body string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, tag := range legalSetBox.FindAllString(body, -1) {
		if !strings.Contains(tag, `type="checkbox"`) {
			continue
		}
		m := legalSetValue.FindStringSubmatch(tag)
		if m == nil {
			t.Fatalf("a framework box without a value: %s", tag)
		}
		out[m[1]] = regexp.MustCompile(`\schecked(\s|>|=)`).MatchString(tag)
	}
	if len(out) != 4 {
		t.Fatalf("%d framework boxes on the screen, want 4: %v", len(out), out)
	}
	return out
}

// legalSetWant fails unless exactly the given ids are ticked.
func legalSetWant(t *testing.T, what, body string, ids ...string) {
	t.Helper()
	ticked := legalSetTicked(t, body)
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for id, on := range ticked {
		if on != want[id] {
			t.Errorf("%s: box %s ticked = %v, want %v", what, id, on, want[id])
		}
	}
}

// legalSetStored is the website's set as the database holds it.
func legalSetStored(t *testing.T, database *db.DB, id int64) string {
	t.Helper()
	ws, err := domain.NewStore(database).GetWebsite(context.Background(), id)
	if err != nil || ws == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return ws.LegalFrameworks
}

// legalSetPut writes a set past every Go rule, the way an earlier save left it.
func legalSetPut(t *testing.T, database *db.DB, id int64, set string) {
	t.Helper()
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE websites SET legal_frameworks = $1 WHERE id = $2`, set, id); err != nil {
		t.Fatalf("set legal frameworks: %v", err)
	}
}

// legalSetForm is the settings form as the browser posts it, with the boxes
// given and the marker that says the group was on the screen.
func legalSetForm(boxes ...string) url.Values {
	v := url.Values{
		"name": {"Testseite"}, "locale": {"de"}, "timezone": {"Europe/Zurich"},
		"offline_mode": {"notfound"}, "legal_frameworks_shown": {"1"},
	}
	for _, b := range boxes {
		v.Add("legal_frameworks", b)
	}
	return v
}

func TestTickingEUAndUSStoresExactlyThat(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	rec, bad, _ := albumFlash(t, h, sm, h.HandleWebsiteEdit,
		postForm(websiteSettingsAddress(ws.ID), legalSetForm("eu", "us"), websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther || bad != "" {
		t.Fatalf("status %d, flash %q", rec.Code, bad)
	}
	// Two boxes, because a test with one would pass against r.FormValue,
	// which hands back the first value only.
	if got := legalSetStored(t, database, ws.ID); got != "eu,us" {
		t.Errorf("stored %q, want eu,us", got)
	}
	body := websiteGet(t, h, sm, h.HandleWebsiteEdit, ws.ID).Body.String()
	legalSetWant(t, "after saving eu and us", body, "eu", "us")
}

func TestNoBoxTickedIsTheEmptySet(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	rec, bad, _ := albumFlash(t, h, sm, h.HandleWebsiteEdit,
		postForm(websiteSettingsAddress(ws.ID), legalSetForm(), websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther || bad != "" {
		t.Fatalf("status %d, flash %q", rec.Code, bad)
	}
	if got := legalSetStored(t, database, ws.ID); got != "" {
		t.Errorf("stored %q, want the empty set", got)
	}
	legalSetWant(t, "after saving none", websiteGet(t, h, sm, h.HandleWebsiteEdit, ws.ID).Body.String())
}

// A post that never drew the group — an older screen, a hand-built request,
// the settings test of this package — says nothing about the frameworks, and
// four unticked boxes look exactly like that on the wire without the marker.
func TestAPostWithoutTheGroupKeepsTheSet(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	legalSetPut(t, database, ws.ID, "ch,uk")

	form := legalSetForm()
	form.Del("legal_frameworks_shown")
	form.Set("city", "Chur")
	rec, bad, _ := albumFlash(t, h, sm, h.HandleWebsiteEdit,
		postForm(websiteSettingsAddress(ws.ID), form, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther || bad != "" {
		t.Fatalf("status %d, flash %q", rec.Code, bad)
	}
	if got := legalSetStored(t, database, ws.ID); got != "ch,uk" {
		t.Errorf("a post without the group changed the set to %q", got)
	}
	if got := storedWebsite(t, h, ws.ID).City; got != "Chur" {
		t.Errorf("the rest of the post was not saved: town %q", got)
	}
}

func TestOneWebsitesSetIsNotAnothers(t *testing.T) {
	h, sm, database, a := newTestAdmin(t)
	b, err := h.domains.CreateWebsite(context.Background(), "Zweite", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	rec, bad, _ := albumFlash(t, h, sm, h.HandleWebsiteEdit,
		postForm(websiteSettingsAddress(a.ID), legalSetForm("us"), websiteRoute(a.ID)))
	if rec.Code != http.StatusSeeOther || bad != "" {
		t.Fatalf("status %d, flash %q", rec.Code, bad)
	}
	if got := legalSetStored(t, database, a.ID); got != "us" {
		t.Fatalf("A stored %q, want us", got)
	}
	if got := legalSetStored(t, database, b.ID); got != "eu,ch" {
		t.Errorf("B's set became %q by a change to A", got)
	}
	legalSetWant(t, "website B", websiteGet(t, h, sm, h.HandleWebsiteEdit, b.ID).Body.String(), "eu", "ch")
	legalSetWant(t, "website A", websiteGet(t, h, sm, h.HandleWebsiteEdit, a.ID).Body.String(), "us")
}

// The frameworks decide what the public site says about the law it follows,
// so they belong to whoever may change what the public sees.
func TestAnEditorWithoutPublishingCannotChangeTheSet(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	route := websiteRoute(ws.ID)

	rec, flash := serveAs(t, h, sm, editor, h.HandleWebsiteEdit,
		postForm(websiteSettingsAddress(ws.ID), legalSetForm("us"), route))
	wantRefused(t, "framework save", rec, flash, websiteSettingsAddress(ws.ID))
	if got := legalSetStored(t, database, ws.ID); got != "eu,ch" {
		t.Errorf("an editor without the right changed the set to %q", got)
	}

	body := getAs(t, sm, putUser(sm, editor), h.HandleWebsiteEdit, websiteSettingsAddress(ws.ID), route)
	if !insideDisabledFieldset(body, `name="legal_frameworks"`) {
		t.Error("the framework boxes are not inside the disabled fieldset")
	}
	legalSetWant(t, "read-only screen", body, "eu", "ch")
}

// Pitfall 8: "uk" is Ukrainian to the locale helpers. A framework is labelled
// by its own name, never through a language name.
func TestTheFrameworkLabelsAreNotLanguageNames(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	body := websiteGet(t, h, sm, h.HandleWebsiteEdit, ws.ID).Body.String()
	for _, label := range []string{
		"European Union (GDPR, ePrivacy)",
		"Switzerland (revised FADP)",
		"United Kingdom (UK GDPR, PECR)",
		"United States (CCPA/CPRA and other state laws)",
	} {
		if !strings.Contains(body, label) {
			t.Errorf("the label %q is not on the screen", label)
		}
	}
	if strings.Contains(body, "Ukrainian") {
		t.Error("the screen names Ukrainian — a framework went through a locale helper")
	}
}

// Pitfall 2: an assistant's change is laid over the stored website as the
// form would post it, so a key missing there reads as the empty set.
func TestAPartialMCPUpdateKeepsTheSet(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	legalSetPut(t, database, ws.ID, "ch,us")

	got, err := h.OpUpdateWebsite(context.Background(), ws.ID, map[string]string{"city": "Chur"})
	if err != nil {
		t.Fatalf("OpUpdateWebsite: %v", err)
	}
	if got.City != "Chur" {
		t.Errorf("town = %q, want Chur", got.City)
	}
	if got := legalSetStored(t, database, ws.ID); got != "ch,us" {
		t.Errorf("an update of the town changed the set to %q", got)
	}
}

func TestMCPChangesTheSetThroughTheFormsRules(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()

	for _, c := range []struct{ given, want string }{
		{"US, eu, xx", "eu,us"},
		{"", ""},
	} {
		if _, err := h.OpUpdateWebsite(ctx, ws.ID, map[string]string{"legal_frameworks": c.given}); err != nil {
			t.Fatalf("OpUpdateWebsite(%q): %v", c.given, err)
		}
		if got := legalSetStored(t, database, ws.ID); got != c.want {
			t.Errorf("given %q, stored %q, want %q", c.given, got, c.want)
		}
	}
}

func TestMCPChangesOnlyItsOwnWebsite(t *testing.T) {
	h, _, database, a := newTestAdmin(t)
	ctx := context.Background()
	b, err := h.domains.CreateWebsite(ctx, "Zweite", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	if _, err := h.OpUpdateWebsite(ctx, a.ID, map[string]string{"legal_frameworks": "uk"}); err != nil {
		t.Fatalf("OpUpdateWebsite: %v", err)
	}
	if got := legalSetStored(t, database, a.ID); got != "uk" {
		t.Fatalf("A stored %q, want uk", got)
	}
	if got := legalSetStored(t, database, b.ID); got != "eu,ch" {
		t.Errorf("B's set became %q by a change to A", got)
	}
}
