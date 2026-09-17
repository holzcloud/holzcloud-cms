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

	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// The website settings screen and the domains on it. Nine handlers, none of
// them tested, and the domain half of them is what decides whether a site
// answers at all.
//
// Driven red by fourteen mutations, all fourteen caught. One test was red
// against the code as it stood and found a real defect: HandleDomainRemove
// passed the store a domain id and nothing else, so website A's screen could
// take a domain off website B. Fixed in internal/domain/store.go, which says
// why at the statement.
//
// Three of the tests had to be sharpened, and it is the same shape every time —
// a second guard one layer down makes the first one's removal invisible:
//
//   - blog_base has a column default of "aktuelles" (migration 00014), so
//     asserting on it passed against a handler that never saved the settings.
//     Two fields with no default are asserted instead.
//   - NormalizeDomain trims and refuses an empty host, so the screen's own
//     trim and its own guard could both go without a test noticing. The
//     assertion is on the sentence the operator reads.

func websiteGet(t *testing.T, h *Handler, sm *scs.SessionManager,
	fn func(http.ResponseWriter, *http.Request) error, websiteID int64,
	extra ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1", nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	for i := 0; i+1 < len(extra); i += 2 {
		req.SetPathValue(extra[i], extra[i+1])
	}
	return serve(t, h, sm, fn, req)
}

func TestTheWebsiteSettingsAreSavedAndTheCachesDropped(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	rec, bad, _ := albumFlash(t, h, sm, h.HandleWebsiteEdit, postForm("/admin/websites/1",
		url.Values{
			"name": {"  Holzbau Schmidt  "}, "description": {"  Möbel nach Mass  "},
			"active": {"on"}, "locale": {"de"}, "timezone": {"Europe/Zurich"},
			"offline_mode": {"notfound"}, "blog_base": {"aktuelles"},
			// Two fields with no column default, because blog_base has one
			// ("aktuelles", migration 00014) and asserting on it would pass
			// against a handler that never saved the settings at all.
			"meta_description": {"  Möbel nach Mass aus Pforzheim  "},
			"city":             {"  Pforzheim  "},
		}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}

	stored, err := domain.NewStore(database).GetWebsite(ctx, ws.ID)
	if err != nil || stored == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	if stored.Name != "Holzbau Schmidt" {
		t.Errorf("the name was not trimmed: %q", stored.Name)
	}
	if stored.Description != "Möbel nach Mass" {
		t.Errorf("description = %q", stored.Description)
	}
	if !stored.Active {
		t.Error("the website was switched off by a save that said active")
	}
	if stored.BlogBase != "aktuelles" {
		t.Errorf("the settings did not arrive: blog base = %q", stored.BlogBase)
	}
	if stored.MetaDescription != "Möbel nach Mass aus Pforzheim" {
		t.Errorf("the settings did not arrive: meta description = %q", stored.MetaDescription)
	}
	if stored.City != "Pforzheim" {
		t.Errorf("the settings did not arrive: city = %q", stored.City)
	}

	// An empty name is refused and changes nothing.
	rec, bad, _ = albumFlash(t, h, sm, h.HandleWebsiteEdit, postForm("/admin/websites/1",
		url.Values{"name": {"   "}}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther || bad == "" {
		t.Errorf("an empty name: status %d, flash %q", rec.Code, bad)
	}
	again, _ := domain.NewStore(database).GetWebsite(ctx, ws.ID)
	if again == nil || again.Name != "Holzbau Schmidt" {
		t.Errorf("the refused save changed the name to %q", again.Name)
	}
}

// Switching a website off has to reach the public side at once. The resolver
// caches the whole Website struct per host, including the active flag, so
// without dropping that cache a deactivated site went on answering until the
// process was restarted.
func TestSwitchingAWebsiteOffDropsTheResolverCache(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if _, err := domain.NewStore(database).AddDomain(ctx, ws.ID, "holzbau.example", true); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}

	// The visitor's side, through the middleware itself rather than through a
	// helper that would not be the thing being tested.
	visit := func() int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "holzbau.example"
		rec := httptest.NewRecorder()
		h.resolver.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rec, req)
		return rec.Code
	}

	// Warm the cache the way a visitor's first request does.
	if code := visit(); code != http.StatusOK {
		t.Fatalf("the site does not answer before the change: %d", code)
	}

	// No "active" in the form is a switched-off website.
	rec := serve(t, h, sm, h.HandleWebsiteEdit, postForm("/admin/websites/1",
		url.Values{"name": {ws.Name}, "locale": {"de"}, "timezone": {"Europe/Zurich"},
			"offline_mode": {"notfound"}},
		websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if stored, _ := domain.NewStore(database).GetWebsite(ctx, ws.ID); stored == nil || stored.Active {
		t.Fatalf("the website was not switched off — the test would prove nothing")
	}

	if code := visit(); code != http.StatusNotFound {
		t.Errorf("the switched-off website still answers with %d — the resolver cache "+
			"was not dropped, so it would go on serving until the process restarts", code)
	}
}

func TestADomainIsAddedRefusedTwiceAndRemoved(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	route := websiteRoute(ws.ID)

	rec, bad, _ := albumFlash(t, h, sm, h.HandleDomainAdd, postForm("/admin/websites/1/domains",
		url.Values{"domain": {"  Holzbau.Example  "}, "is_primary": {"on"}}, route))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	list, err := domain.NewStore(database).ListDomains(ctx, ws.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListDomains: %v (%d)", err, len(list))
	}
	// Normalised on the way in: a host name is lower case and has no spaces
	// around it, whatever the form sent. Both the screen and NormalizeDomain
	// trim, so this holds the outcome; what the screen's own trim is for is the
	// empty-name case below, and that is asserted on the sentence.
	if list[0].Domain != "holzbau.example" {
		t.Errorf("the domain was stored as %q", list[0].Domain)
	}
	if !list[0].IsPrimary {
		t.Error("the primary flag did not arrive")
	}

	// The same host twice is refused with a sentence rather than a 500 out of
	// the unique index — and a host belonging to somebody else is the case that
	// matters, because two sites answering one name is not resolvable.
	rec, bad, _ = albumFlash(t, h, sm, h.HandleDomainAdd, postForm("/admin/websites/1/domains",
		url.Values{"domain": {"holzbau.example"}}, route))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("the duplicate: status %d", rec.Code)
	}
	if !strings.Contains(bad, "already") && !strings.Contains(bad, "bereits") {
		t.Errorf("the duplicate was answered with %q", bad)
	}
	if list, _ := domain.NewStore(database).ListDomains(ctx, ws.ID); len(list) != 1 {
		t.Errorf("%d domains after the refused duplicate", len(list))
	}

	// An empty name is refused HERE, by the screen, with the screen's own
	// sentence. NormalizeDomain refuses it too, so asserting only "an error
	// came back" would pass with the screen's guard removed and leave the
	// operator reading a wrapped parse error instead of an instruction.
	rec, bad, _ = albumFlash(t, h, sm, h.HandleDomainAdd,
		postForm("/admin/websites/1/domains", url.Values{"domain": {"   "}}, route))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("an empty domain: status %d", rec.Code)
	}
	if bad != "Domain name is required" {
		t.Errorf("an empty domain was answered with %q", bad)
	}

	// And it goes again.
	rec = serve(t, h, sm, h.HandleDomainRemove, postForm("/admin/websites/1/domains/1/delete",
		nil, websiteRoute(ws.ID, "domainID", strconv.FormatInt(list[0].ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("remove: status %d", rec.Code)
	}
	if list, _ := domain.NewStore(database).ListDomains(ctx, ws.ID); len(list) != 0 {
		t.Error("the domain is still there")
	}
}

// A domain is what decides whether a site answers at all, so removing one
// through the wrong website's screen must do nothing.
//
// This test was red: HandleDomainRemove passed the domain id to the store and
// nothing else, and the store deleted by id alone. Its sibling
// HandleDomainSetPrimary hands down both ids and the store checks both — so the
// two handlers on the same screen disagreed, and the one that disagreed was the
// destructive one.
//
// Not a privilege hole: both routes are behind requireAdmin, and an
// administrator may enter every website by design (NewWebsiteAccessLookup says
// why). What it is, is a mis-click or a stale page taking another customer's
// site off the air while the screen says "Domain removed" and this website's
// list is unchanged.
func TestRemovingADomainThroughTheWrongWebsiteDoesNothing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	domains := domain.NewStore(database)

	other, err := domains.CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	fremd, err := domains.AddDomain(ctx, other.ID, "fremd.example", true)
	if err != nil {
		t.Fatalf("AddDomain: %v", err)
	}

	route := websiteRoute(ws.ID, "domainID", strconv.FormatInt(fremd.ID, 10))
	rec := serve(t, h, sm, h.HandleDomainRemove,
		postForm("/admin/websites/1/domains/1/delete", nil, route))
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}

	still, err := domains.ListDomains(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(still) != 1 {
		t.Fatalf("the other website's domain was removed from this website's screen — "+
			"%d left", len(still))
	}
	if !still[0].IsPrimary || still[0].Domain != "fremd.example" {
		t.Errorf("the other website's domain was changed: %+v", still[0])
	}

	// The same for the primary flag, which already checked both ids. Asserted
	// here so the pair keeps agreeing.
	second, err := domains.AddDomain(ctx, other.ID, "zweit.example", false)
	if err != nil {
		t.Fatal(err)
	}
	serve(t, h, sm, h.HandleDomainSetPrimary, postForm("/admin/websites/1/domains/1/primary",
		nil, websiteRoute(ws.ID, "domainID", strconv.FormatInt(second.ID, 10))))
	after, _ := domains.ListDomains(ctx, other.ID)
	for _, d := range after {
		if d.ID == second.ID && d.IsPrimary {
			t.Error("the other website's primary domain was moved from this website's screen")
		}
	}
}

// The settings screen renders, with what is on it and with the checks that tell
// an operator what is still missing.
func TestTheWebsiteSettingsScreenShowsWhatIsThere(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if _, err := domain.NewStore(database).AddDomain(ctx, ws.ID, "holzbau.example", true); err != nil {
		t.Fatal(err)
	}
	rec := websiteGet(t, h, sm, h.HandleWebsiteEdit, ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{ws.Name, "holzbau.example"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not show %q", want)
		}
	}

	// An unknown website is a 404 and not a 500.
	for _, fn := range []func(http.ResponseWriter, *http.Request) error{
		h.HandleWebsiteEdit, h.HandleWebsiteDesign,
	} {
		if rec := websiteGet(t, h, sm, fn, 999); rec.Code != http.StatusNotFound {
			t.Errorf("an unknown website: status %d, want 404", rec.Code)
		}
	}
	// And an id that is not a number at all.
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/x", nil)
	req.SetPathValue("id", "keine-zahl")
	if rec := serve(t, h, sm, h.HandleWebsiteEdit, req); rec.Code != http.StatusNotFound {
		t.Errorf("a non-numeric id: status %d, want 404", rec.Code)
	}
}
