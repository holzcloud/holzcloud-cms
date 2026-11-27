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

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

func countPages(t *testing.T, database *db.DB, websiteID int64) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM pages WHERE website_id = $1`, websiteID).Scan(&n); err != nil {
		t.Fatalf("count pages: %v", err)
	}
	return n
}

func pageRoute(websiteID, pageID int64) map[string]string {
	r := map[string]string{"id": strconv.FormatInt(websiteID, 10)}
	if pageID != 0 {
		r["pageID"] = strconv.FormatInt(pageID, 10)
	}
	return r
}

// Protection asked for without a usable password refuses the save, before
// anything is stored. It used to store the page — published, if that was
// asked for — and only then report that the protection had failed, so a page
// meant for a few people stood public with a flash beside it.
func TestProtectingAPageWithoutAUsablePasswordRefusesTheSave(t *testing.T) {
	const (
		missing  = "Please set a password, or switch the protection off."
		tooShort = "A page password needs at least 6 characters."
	)

	for _, tc := range []struct {
		name, password, want string
	}{
		{"create without a password", "", missing},
		{"create with a short password", "abc", tooShort},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database, ws := newTestAdmin(t)
			before := countPages(t, database, ws.ID)
			rec := serve(t, h, sm, h.HandlePageCreate, postForm("/admin/websites/1/pages/new", url.Values{
				"title":            {"Intern"},
				"slug":             {"intern"},
				"content_markdown": {"Nur für Mitglieder."},
				"status":           {"published"},
				"kind":             {"page"},
				"protected":        {"1"},
				"page_password":    {tc.password},
			}, pageRoute(ws.ID, 0)))

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `id="page-password-error"`) || !strings.Contains(body, tc.want) {
				t.Errorf("the password field does not say %q", tc.want)
			}
			if n := countPages(t, database, ws.ID); n != before {
				t.Errorf("%d pages after a refused create, want %d", n, before)
			}
		})
	}

	for _, tc := range []struct {
		name, stored, password, want string
	}{
		{"edit without a stored password", "", "", missing},
		{"edit with a short password over a stored one", "geheim123", "abc", tooShort},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database, ws := newTestAdmin(t)
			store := page.NewStore(database)
			p := seedPage(t, database, ws.ID, "Intern", "intern", "vorher", "published")
			if tc.stored != "" {
				if err := store.SetAccess(context.Background(), p.ID,
					page.AccessUpdate{Protected: true, Password: tc.stored}, cheapHashing); err != nil {
					t.Fatalf("SetAccess: %v", err)
				}
			}
			p, _ = store.GetPage(context.Background(), p.ID)

			v := url.Values{
				"title":            {"Intern"},
				"slug":             {"intern"},
				"content_markdown": {"nachher"},
				"status":           {"draft"},
				"version":          {strconv.FormatInt(p.Version, 10)},
				"protected":        {"1"},
				"page_password":    {tc.password},
			}
			rec := serve(t, h, sm, h.HandlePageEdit, postForm("/x", v, pageRoute(ws.ID, p.ID)))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `id="page-password-error"`) || !strings.Contains(body, tc.want) {
				t.Errorf("the password field does not say %q", tc.want)
			}

			after, err := store.GetPage(context.Background(), p.ID)
			if err != nil || after == nil {
				t.Fatalf("GetPage: %v", err)
			}
			if after.ContentMarkdown != "vorher" || after.Status != "published" || after.Version != p.Version {
				t.Errorf("a refused save changed the page: %q %q v%d", after.ContentMarkdown, after.Status, after.Version)
			}
			if tc.stored == "" && after.Protected() {
				t.Error("a refused save protected the page")
			}
			if ok, _ := page.CheckPagePassword(after, tc.stored); tc.stored != "" && !ok {
				t.Error("the stored password no longer opens the page")
			}
		})
	}
}

// The one case that looks like "protection without a password" and is not:
// the field is empty because a password is already stored.
func TestAProtectedPageKeepsItsPasswordWhenTheFieldIsLeftEmpty(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	store := page.NewStore(database)
	p := seedPage(t, database, ws.ID, "Intern", "intern", "vorher", "draft")
	if err := store.SetAccess(context.Background(), p.ID,
		page.AccessUpdate{Protected: true, Password: "geheim123"}, cheapHashing); err != nil {
		t.Fatalf("SetAccess: %v", err)
	}
	p, _ = store.GetPage(context.Background(), p.ID)

	rec := serve(t, h, sm, h.HandlePageEdit, postForm("/x", url.Values{
		"title":            {"Intern"},
		"slug":             {"intern"},
		"content_markdown": {"nachher"},
		"status":           {"draft"},
		"version":          {strconv.FormatInt(p.Version, 10)},
		"protected":        {"1"},
		"page_password":    {""},
		"has_password":     {"1"},
		"access_hint":      {"Steht im Rundbrief"},
	}, pageRoute(ws.ID, p.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	after, err := store.GetPage(context.Background(), p.ID)
	if err != nil || after == nil {
		t.Fatalf("GetPage: %v", err)
	}
	if after.ContentMarkdown != "nachher" || after.AccessHint != "Steht im Rundbrief" {
		t.Errorf("text %q, hint %q not stored", after.ContentMarkdown, after.AccessHint)
	}
	if ok, _ := page.CheckPagePassword(after, "geheim123"); !after.Protected() || !ok {
		t.Error("the page lost its password")
	}
}

func TestProtectingAPageWithAPasswordStillSaves(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	rec := serve(t, h, sm, h.HandlePageCreate, postForm("/admin/websites/1/pages/new", url.Values{
		"title":         {"Intern"},
		"slug":          {"intern"},
		"status":        {"draft"},
		"kind":          {"page"},
		"protected":     {"1"},
		"page_password": {"geheim123"},
	}, pageRoute(ws.ID, 0)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	p, err := page.NewStore(database).GetPageBySlug(context.Background(), ws.ID, "intern")
	if err != nil || p == nil {
		t.Fatalf("the page was not created: %v", err)
	}
	if !p.Protected() {
		t.Error("the page is not protected")
	}
}

// serveThroughErrHandler runs a handler the way the router does, so an error it
// returns is answered by ErrHandler (a busy gate as 503) instead of failing the
// test.
func serveThroughErrHandler(h *Handler, sm interface {
	LoadAndSave(http.Handler) http.Handler
}, fn func(http.ResponseWriter, *http.Request) error, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	sm.LoadAndSave(h.ErrHandler(fn)).ServeHTTP(rec, req)
	return rec
}

// WR-01: hashing a page password can find the Argon2 gate full. That used to
// happen after the page was stored, so a page created as published and
// protected went live without its password, with only a flash to say so. Now
// the password is hashed before anything is stored, and a busy gate refuses
// the whole save with 503.
func TestABusyGateRefusesTheWholeSaveOfAProtectedPage(t *testing.T) {
	fullGate := func(t *testing.T) {
		t.Helper()
		gate := auth.NewGate(1, time.Millisecond, time.Millisecond)
		release, err := gate.Acquire(0)
		if err != nil {
			t.Fatal(err)
		}
		previous := auth.SetArgon2Gate(gate)
		t.Cleanup(func() {
			auth.SetArgon2Gate(previous)
			release()
		})
	}

	t.Run("create", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		before := countPages(t, database, ws.ID)
		fullGate(t)
		rec := serveThroughErrHandler(h, sm, h.HandlePageCreate, postForm("/admin/websites/1/pages/new", url.Values{
			"title":            {"Intern"},
			"slug":             {"intern"},
			"content_markdown": {"Nur für Mitglieder."},
			"status":           {"published"},
			"kind":             {"page"},
			"protected":        {"1"},
			"page_password":    {"geheim123"},
		}, pageRoute(ws.ID, 0)))
		if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
			t.Fatalf("status %d, Retry-After %q; want 503 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
		}
		if n := countPages(t, database, ws.ID); n != before {
			t.Errorf("%d pages after a busy save, want %d: the page was stored without its password", n, before)
		}
	})

	t.Run("edit", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		store := page.NewStore(database)
		p := seedPage(t, database, ws.ID, "Intern", "intern", "vorher", "published")
		fullGate(t)
		rec := serveThroughErrHandler(h, sm, h.HandlePageEdit, postForm("/x", url.Values{
			"title":            {"Intern"},
			"slug":             {"intern"},
			"content_markdown": {"nachher"},
			"status":           {"published"},
			"version":          {strconv.FormatInt(p.Version, 10)},
			"protected":        {"1"},
			"page_password":    {"geheim123"},
		}, pageRoute(ws.ID, p.ID)))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status %d, want 503", rec.Code)
		}
		after, err := store.GetPage(context.Background(), p.ID)
		if err != nil || after == nil {
			t.Fatalf("GetPage: %v", err)
		}
		if after.ContentMarkdown != "vorher" || after.Version != p.Version {
			t.Errorf("a busy save stored the page: %q v%d", after.ContentMarkdown, after.Version)
		}
	})
}
