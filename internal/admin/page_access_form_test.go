package admin

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

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
			if tc.stored != "" && !page.CheckPagePassword(after, tc.stored) {
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
	if !after.Protected() || !page.CheckPagePassword(after, "geheim123") {
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
