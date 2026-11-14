package admin

import (
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
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
	// A new website is German, follows EU and Switzerland (D-05) and uses
	// none of the functions yet.
	want := privacy.ComposePrivacy(privacy.Request{Lang: "de", Set: legal.Default(), Features: privacy.Features{}})
	if p.ContentMarkdown != want {
		t.Error("the stored Markdown is not the composed template of a new website")
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
	if p.ContentMarkdown != privacy.ComposePrivacy(privacy.Request{Lang: "de", Set: legal.Default(), Features: features}) {
		t.Error("the stored Markdown is not the composed template of the website's functions")
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

// The start page opens as before. The imprint is now composed from the
// website's frameworks (D-09) like the privacy page, so it no longer opens
// with the § 5 DDG heading: that line is now the heading of the EU block
// further down, which a new website still gets because EU and Switzerland
// are its default set.
func TestStarterLegalPagesAreUnchanged(t *testing.T) {
	h, _, _, _ := newTestAdmin(t)
	ctx := context.Background()

	ws, err := h.OpCreateWebsite(ctx, "Hofladen", "", true)
	if err != nil {
		t.Fatal(err)
	}
	home, err := h.pages.GetPageBySlug(ctx, ws.ID, "home")
	if err != nil || home == nil {
		t.Fatalf("home: %v", err)
	}
	if first, _, _ := strings.Cut(home.ContentMarkdown, "\n"); first != "Willkommen!" {
		t.Errorf("home opens with %q, want %q", first, "Willkommen!")
	}
	if home.Status != "published" {
		t.Errorf("home status = %q, want published", home.Status)
	}

	imp, err := h.pages.GetPageBySlug(ctx, ws.ID, "impressum")
	if err != nil || imp == nil {
		t.Fatalf("impressum: %v", err)
	}
	if imp.Status != "draft" {
		t.Errorf("impressum status = %q, want draft", imp.Status)
	}
	if imp.Title != "Impressum" {
		t.Errorf("impressum title = %q, want Impressum", imp.Title)
	}
	want := privacy.ComposeImprint(privacy.Request{Lang: "de", Set: legal.Default()})
	if imp.ContentMarkdown != want {
		t.Error("the stored imprint is not the composed imprint of a new website")
	}
	if !strings.Contains(imp.ContentMarkdown, "## Angaben gemäß § 5 DDG") {
		t.Error("the imprint of a new website lacks the EU block")
	}
	html, err := page.RenderMarkdown(want)
	if err != nil {
		t.Fatal(err)
	}
	if imp.ContentHTML != html {
		t.Error("the stored imprint HTML is not the rendering of the stored Markdown")
	}
}

// A website that follows only US law gets the lean imprint: who it is and how
// to reach it, without the EU and Swiss items (D-09).
func TestAUSOnlyWebsiteGetsTheLeanStarterImprint(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if _, err := database.Write.Exec(`UPDATE websites SET legal_frameworks = 'us' WHERE id = ?`, ws.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := h.domains.GetWebsite(ctx, ws.ID)
	if err != nil || stored == nil {
		t.Fatalf("GetWebsite: %v", err)
	}

	h.createStarterContent(ctx, ws.ID, nil)

	imp, err := h.pages.GetPageBySlug(ctx, ws.ID, "impressum")
	if err != nil || imp == nil {
		t.Fatalf("the imprint is missing: %v", err)
	}
	want := privacy.ComposeImprint(privacy.Request{Lang: "de", Swiss: stored.Country == "CH", Set: legal.ParseSet("us")})
	if imp.ContentMarkdown != want {
		t.Error("the stored imprint is not the lean composition of a US-only website")
	}
	for _, bad := range []string{"§ 5 DDG", "SR 241", "2002/2013"} {
		if strings.Contains(imp.ContentMarkdown, bad) {
			t.Errorf("the lean imprint carries %q", bad)
		}
	}
	if imp.Status != "draft" {
		t.Errorf("status = %q, want draft", imp.Status)
	}
}

// The two legal starter pages carry the title and the slug of the website's
// main language, the same ones the Legal pages card uses (privacy.Title,
// privacy.Slug), so a French website does not start with a page called
// "Impressum" at /impressum that the card then fails to recognise as its
// French imprint. A German website keeps its German titles and the slugs
// impressum and datenschutz (TestStarterLegalPagesAreUnchanged).
func TestStarterLegalPagesFollowTheMainLanguage(t *testing.T) {
	for _, lang := range []string{"en", "fr", "it", "de-CH"} {
		t.Run(lang, func(t *testing.T) {
			h, _, database, ws := newTestAdmin(t)
			ctx := context.Background()
			if _, err := database.Write.Exec(`UPDATE websites SET locale = ? WHERE id = ?`, lang, ws.ID); err != nil {
				t.Fatal(err)
			}

			h.createStarterContent(ctx, ws.ID, nil)

			var footer []string
			for _, k := range []privacy.Kind{privacy.KindImprint, privacy.KindPrivacy} {
				slug, title := privacy.Slug(k, lang), privacy.Title(k, lang)
				p, err := h.pages.GetPageBySlug(ctx, ws.ID, slug)
				if err != nil || p == nil {
					t.Fatalf("no %s page at /%s: %v", k, slug, err)
				}
				if p.Title != title {
					t.Errorf("%s title = %q, want %q", k, p.Title, title)
				}
				if p.Status != "draft" {
					t.Errorf("%s status = %q, want draft", k, p.Status)
				}
				footer = append(footer, title)
			}
			if !strings.HasPrefix(lang, "de") {
				for _, german := range []string{"impressum", "datenschutz"} {
					if p, _ := h.pages.GetPageBySlug(ctx, ws.ID, german); p != nil {
						t.Errorf("a %s website still gets a page at /%s", lang, german)
					}
				}
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
				if len(items) != 2 || items[0].Title != footer[0] || items[1].Title != footer[1] {
					t.Errorf("footer menu = %+v, want %q then %q", items, footer[0], footer[1])
				}
				checked = true
			}
			if !checked {
				t.Error("the website has no footer menu")
			}
		})
	}
}
