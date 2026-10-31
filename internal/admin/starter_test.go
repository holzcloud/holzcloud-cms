package admin

import (
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
)

// A website created with starter content gets the composed template for a
// website that uses none of the functions, as a draft, second in the footer.
func TestStarterPrivacyPageIsTheComposedTemplate(t *testing.T) {
	h, _, _, _ := newTestAdmin(t)
	ctx := context.Background()

	ws, err := h.OpCreateWebsite(ctx, "Hofladen", "", true)
	if err != nil {
		t.Fatalf("OpCreateWebsite: %v", err)
	}
	p, err := h.pages.GetPageBySlug(ctx, ws.ID, "datenschutz")
	if err != nil || p == nil {
		t.Fatalf("the privacy page is missing: %v", err)
	}
	if p.Status != "draft" {
		t.Errorf("status = %q, want draft (the page holds fill-in markers)", p.Status)
	}
	want := privacy.Compose(privacy.Features{})
	if p.ContentMarkdown != want {
		t.Error("the stored Markdown is not privacy.Compose of the zero Features")
	}
	html, err := page.RenderMarkdown(want)
	if err != nil {
		t.Fatal(err)
	}
	if p.ContentHTML != html {
		t.Error("the stored HTML is not the rendering of the stored Markdown")
	}

	menus, err := h.menuStore.ListMenus(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	checked := false
	for _, m := range menus {
		if m.LocationKey != "footer" {
			continue
		}
		items, err := h.menuStore.ListItems(ctx, m.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || items[0].PageSlug != "impressum" || items[1].PageSlug != "datenschutz" {
			t.Errorf("footer menu = %+v, want impressum then datenschutz", items)
		}
		checked = true
	}
	if !checked {
		t.Error("the website has no footer menu")
	}
}

// A website that already runs a shop and a protected page when its starter
// content is made gets the cookie rows for them.
func TestStarterPrivacyPageFollowsTheWebsiteFunctions(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if _, err := database.Write.Exec(
		`UPDATE websites SET shop_base = 'laden', price_display = 'both' WHERE id = ?`, ws.ID); err != nil {
		t.Fatal(err)
	}
	prot := seedPage(t, database, ws.ID, "Preisliste", "preisliste", "x", "published")
	if err := h.pages.SetAccess(ctx, prot.ID, page.AccessUpdate{Protected: true, Password: "holz2026"}, h.argon2Params); err != nil {
		t.Fatal(err)
	}

	h.createStarterContent(ctx, ws.ID, nil)

	p, err := h.pages.GetPageBySlug(ctx, ws.ID, "datenschutz")
	if err != nil || p == nil {
		t.Fatalf("the privacy page is missing: %v", err)
	}
	features := privacy.Features{Shop: true, PriceChoice: true, ProtectedPages: true}
	if p.ContentMarkdown != privacy.Compose(features) {
		t.Error("the stored Markdown is not privacy.Compose of the website's functions")
	}
	if p.Status != "draft" {
		t.Errorf("status = %q, want draft", p.Status)
	}
	cookies := privacy.Cookies(features)
	if len(cookies) != 3 {
		t.Fatalf("the register lists %d cookies for this website, want 3", len(cookies))
	}
	for _, c := range cookies {
		if !strings.Contains(p.ContentMarkdown, c.Name) {
			t.Errorf("the page does not name the cookie %q", c.Name)
		}
	}
}

// Only the privacy page changed: the other two starter pages open as before.
func TestStarterLegalPagesAreUnchanged(t *testing.T) {
	h, _, _, _ := newTestAdmin(t)
	ctx := context.Background()

	ws, err := h.OpCreateWebsite(ctx, "Hofladen", "", true)
	if err != nil {
		t.Fatal(err)
	}
	for slug, wantFirst := range map[string]string{
		"impressum": "## Angaben gemäß § 5 DDG",
		"home":      "Willkommen!",
	} {
		p, err := h.pages.GetPageBySlug(ctx, ws.ID, slug)
		if err != nil || p == nil {
			t.Fatalf("%s: %v", slug, err)
		}
		first, _, _ := strings.Cut(p.ContentMarkdown, "\n")
		if first != wantFirst {
			t.Errorf("%s opens with %q, want %q", slug, first, wantFirst)
		}
	}
	imp, _ := h.pages.GetPageBySlug(ctx, ws.ID, "impressum")
	if imp.Status != "draft" {
		t.Errorf("impressum status = %q, want draft", imp.Status)
	}
	home, _ := h.pages.GetPageBySlug(ctx, ws.ID, "home")
	if home.Status != "published" {
		t.Errorf("home status = %q, want published", home.Status)
	}
}
