package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// The activity log: the screen that says who did what, and the one action in
// this administration that removes the record of the other actions.
//
// Driven red by thirteen mutations, all thirteen caught. One had to be chased:
// removing the "no date" guard let time.Parse("") fail instead, which is still
// a refusal — with the wrong sentence on it. The two-guard shape again, and
// again the fix is to assert what the operator reads.

func activityAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, *domain.Website) {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	// Nil leaves the screen out entirely (SetActivityStore says so), so a test
	// of the screen has to hand it one.
	h.SetActivityStore(activity.NewStore(database))
	return h, sm, database, ws
}

func activityRequest(query string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/admin/protokoll"+query, nil)
}

func TestTheActivityLogShowsWhatWasDoneAndNarrowsIt(t *testing.T) {
	h, sm, database, ws := activityAdmin(t)
	ctx := context.Background()
	store := activity.NewStore(database)

	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	store.Log(ctx, activity.Entry{
		ActorEmail: "anna@example.com", Action: activity.ActionPagePublish,
		EntityType: "page", EntityID: 1, WebsiteID: &ws.ID,
		Metadata: map[string]any{"titel": "Über uns"},
	})
	store.Log(ctx, activity.Entry{
		ActorEmail: "bruno@example.com", Action: activity.ActionDomainAdd,
		EntityType: "domain", EntityID: 2, WebsiteID: &other.ID,
	})

	body := serve(t, h, sm, h.HandleActivityList, activityRequest("")).Body.String()
	for _, want := range []string{"anna@example.com", "bruno@example.com", "Die andere"} {
		if !strings.Contains(body, want) {
			t.Errorf("the log does not show %q", want)
		}
	}

	// Narrowed to one website.
	body = serve(t, h, sm, h.HandleActivityList,
		activityRequest("?website_id="+strconv.FormatInt(other.ID, 10))).Body.String()
	if !strings.Contains(body, "bruno@example.com") {
		t.Error("the website filter left out the entry it should keep")
	}
	if strings.Contains(body, "anna@example.com") {
		t.Error("the website filter kept the other website's entry")
	}

	// Narrowed to one action.
	body = serve(t, h, sm, h.HandleActivityList,
		activityRequest("?action="+url.QueryEscape(activity.ActionDomainAdd))).Body.String()
	if !strings.Contains(body, "bruno@example.com") || strings.Contains(body, "anna@example.com") {
		t.Error("the action filter did not narrow to the action")
	}
}

// A hand-edited address shows an unfiltered protocol rather than an error page:
// somebody who trims a query string by hand should not be met with a 500.
func TestAnUnreadableFilterIsIgnoredRatherThanRefused(t *testing.T) {
	h, sm, database, ws := activityAdmin(t)
	ctx := context.Background()
	activity.NewStore(database).Log(ctx, activity.Entry{
		ActorEmail: "anna@example.com", Action: activity.ActionPagePublish,
		EntityType: "page", EntityID: 1, WebsiteID: &ws.ID,
	})

	for _, q := range []string{
		"?website_id=keine-zahl",
		"?user_id=keine-zahl",
		"?from=gestern",
		"?to=irgendwann",
		"?from=2026-13-45",
		"?website_id=&user_id=&from=&to=",
	} {
		rec := serve(t, h, sm, h.HandleActivityList, activityRequest(q))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", q, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), "anna@example.com") {
			t.Errorf("%s: an unreadable filter narrowed the log to nothing", q)
		}
	}

	// And the filters that ARE readable still narrow, or the test above would
	// pass against a version that ignores every filter.
	f := activityFilter(url.Values{"website_id": {"7"}, "user_id": {"3"},
		"action": {" seite.publizieren "}, "from": {"2026-01-01"}, "to": {"2026-01-31"}})
	if f.WebsiteID == nil || *f.WebsiteID != 7 {
		t.Errorf("website id = %v", f.WebsiteID)
	}
	if f.UserID == nil || *f.UserID != 3 {
		t.Errorf("user id = %v", f.UserID)
	}
	if f.Action != "seite.publizieren" {
		t.Errorf("action = %q", f.Action)
	}
	if f.From == nil || f.From.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("from = %v", f.From)
	}
	// "until 31 January" means the 31st of January with it, and not its first
	// moment — a day filter that stopped at midnight would hide everything
	// done on the last day asked for.
	if f.To == nil || f.To.Format("2006-01-02 15:04:05") != "2026-01-31 23:59:59" {
		t.Errorf("to = %v, want the end of the day", f.To)
	}
}

// The pager has to carry the filters, or the second page of a protocol somebody
// narrowed down is the unfiltered one.
func TestThePagerKeepsWhateverTheFiltersWere(t *testing.T) {
	got := activityPagerQuery(url.Values{
		"page":       {"3"},
		"website_id": {"7"},
		"action":     {"seite.publizieren"},
		"leer":       {"   "},
		"from":       {"2026-01-01"},
	})
	values, err := url.ParseQuery(got)
	if err != nil {
		t.Fatalf("the pager query is not a query: %v", err)
	}
	if values.Has("page") {
		t.Error("the pager carried the page number into its own links")
	}
	if values.Has("leer") {
		t.Error("an empty filter was carried along")
	}
	for k, want := range map[string]string{
		"website_id": "7", "action": "seite.publizieren", "from": "2026-01-01",
	} {
		if values.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, values.Get(k), want)
		}
	}
}

// Purging is the one action that removes the record of the other actions, so it
// is refused without a date, refused with a date that is not one, and recorded
// when it happens.
func TestPurgingNeedsADateAndLeavesItsOwnTrace(t *testing.T) {
	h, sm, database, ws := activityAdmin(t)
	ctx := context.Background()
	store := activity.NewStore(database)

	store.Log(ctx, activity.Entry{
		ActorEmail: "anna@example.com", Action: activity.ActionPagePublish,
		EntityType: "page", EntityID: 1, WebsiteID: &ws.ID,
	})

	// Each case is asserted on ITS OWN sentence. "No date" and "not a date" are
	// two different things to tell somebody, and they are caught by two
	// different guards — so asserting only "an error came back" passes with the
	// first guard removed, because time.Parse("") fails too and answers the
	// wrong one.
	for _, c := range []struct{ what, before, want string }{
		{"no date at all", "", "Please give a date"},
		{"only spaces", "   ", "Please give a date"},
		{"a date that is not one", "irgendwann", "That is not a valid date"},
		{"a date the wrong way round", "31.01.2026", "That is not a valid date"},
	} {
		rec, bad, _ := albumFlash(t, h, sm, h.HandleActivityPurge,
			postForm("/admin/protokoll/loeschen", url.Values{"before": {c.before}}, nil))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if bad != c.want {
			t.Errorf("%s: answered %q, want %q", c.what, bad, c.want)
		}
	}
	if _, total, _ := store.List(ctx, activity.Filter{}, 50, 0); total != 1 {
		t.Fatalf("a refused purge changed the log: %d entries", total)
	}

	// A date in the future takes everything that is there — and the purge
	// itself is written, because an audit trail that can be emptied without a
	// trace is not one.
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	rec, bad, good := albumFlash(t, h, sm, h.HandleActivityPurge,
		postForm("/admin/protokoll/loeschen", url.Values{"before": {tomorrow}}, nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	if !strings.Contains(good, "1") {
		t.Errorf("the message does not say how many went: %q", good)
	}

	left, total, err := store.List(ctx, activity.Filter{}, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("%d entries after the purge, want just the purge's own", total)
	}
	if left[0].Action != activity.ActionActivityPurge {
		t.Errorf("the entry left behind is %q, not the purge's own record", left[0].Action)
	}
}

// A build without a protocol leaves the screen out rather than failing, which
// is what the nil check in LogActivity promises.
func TestWithoutAProtocolNothingBreaks(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	if h.activityStore != nil {
		t.Fatal("newTestAdmin now has a protocol — this test proves nothing")
	}

	// Every handler that logs still works.
	if _, err := domain.NewStore(database).AddDomain(ctx, ws.ID, "holzbau.example", true); err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, sm, h.HandleDomainAdd, postForm("/admin/websites/1/domains",
		url.Values{"domain": {"zweit.example"}}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("a logging handler without a protocol: status %d", rec.Code)
	}
	list, _ := domain.NewStore(database).ListDomains(ctx, ws.ID)
	if len(list) != 2 {
		t.Errorf("%d domains — the work itself did not happen", len(list))
	}
}
