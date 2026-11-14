package admin

import (
	"context"
	"slices"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
)

// The labels of the legal items, as the catalogue knows them.
const (
	legalCheckFrameworks = "Legal frameworks chosen"
	legalCheckImprint    = "Imprint linked in the footer menu"
	legalCheckPrivacy    = "Privacy page linked in the footer menu"
	legalCheckDNS        = "Do Not Sell or Share page linked in the footer menu"
)

// legalCheckWebsite stores the frameworks and the main language and reads the
// website back.
func legalCheckWebsite(t *testing.T, h *Handler, database *db.DB, ws *domain.Website, set, mainLang string) *domain.Website {
	t.Helper()
	if _, err := database.Write.Exec(`UPDATE websites SET legal_frameworks = ?, locale = ? WHERE id = ?`, set, mainLang, ws.ID); err != nil {
		t.Fatalf("store the website: %v", err)
	}
	return readWebsite(t, h, ws.ID)
}

// legalCheckFooter puts a page with the slug and status into the footer menu
// of the main language. linked false creates the page without the menu entry.
func legalCheckFooter(t *testing.T, h *Handler, database *db.DB, websiteID int64, slug, status string, linked bool) {
	t.Helper()
	legalCheckFooterText(t, h, database, websiteID, slug, status, "text", linked)
}

// legalCheckFooterText is legalCheckFooter with the page's Markdown.
func legalCheckFooterText(t *testing.T, h *Handler, database *db.DB, websiteID int64, slug, status, text string, linked bool) {
	t.Helper()
	ctx := context.Background()
	p := seedPage(t, database, websiteID, "Page "+slug, slug, text, status)
	if !linked {
		return
	}
	menus, err := h.menuStore.ListMenus(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListMenus: %v", err)
	}
	var menuID int64
	for _, m := range menus {
		if m.LocationKey == "footer" && m.Locale == "" {
			menuID = m.ID
		}
	}
	if menuID == 0 {
		m, err := h.menuStore.CreateMenu(ctx, websiteID, "Footer", "footer", "")
		if err != nil {
			t.Fatalf("CreateMenu: %v", err)
		}
		menuID = m.ID
	}
	if _, err := h.menuStore.CreateItem(ctx, menuID, nil, p.Title, "page", "", &p.ID, 0); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
}

// legalCheckFind is the item with the label, or nil.
func legalCheckFind(checks []siteCheck, label string) *siteCheck {
	for i := range checks {
		if checks[i].Label == label {
			return &checks[i]
		}
	}
	return nil
}

func TestTheChecklistAsksForAFramework(t *testing.T) {
	for _, tc := range []struct {
		set  string
		want bool
	}{{"", false}, {"eu", true}, {"us", true}} {
		t.Run("set="+tc.set, func(t *testing.T) {
			h, _, database, ws := newTestAdmin(t)
			stored := legalCheckWebsite(t, h, database, ws, tc.set, "de")
			c := legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckFrameworks)
			if c == nil {
				t.Fatalf("no item %q", legalCheckFrameworks)
			}
			if c.OK != tc.want {
				t.Errorf("done = %v, want %v", c.OK, tc.want)
			}
			if c.Link != "/admin/websites/"+idOf(ws.ID)+"#legal-frameworks" {
				t.Errorf("link %q, want the frameworks on the settings screen", c.Link)
			}
			if c.Hint == "" {
				t.Errorf("no hint")
			}
		})
	}
}

func TestTheImprintCheckKnowsEveryLanguage(t *testing.T) {
	for _, tc := range []struct {
		name, lang, slug, status string
		linked, want             bool
	}{
		{"French", "fr", "mentions-legales", "published", true, true},
		{"English", "en", "legal-notice", "published", true, true},
		{"Italian", "it", "note-legali", "published", true, true},
		{"German", "de", "impressum", "published", true, true},
		{"draft", "fr", "mentions-legales", "draft", true, false},
		{"not linked", "fr", "mentions-legales", "published", false, false},
		{"other page", "fr", "contact", "published", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, database, ws := newTestAdmin(t)
			stored := legalCheckWebsite(t, h, database, ws, "eu", tc.lang)
			legalCheckFooter(t, h, database, ws.ID, tc.slug, tc.status, tc.linked)
			c := legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckImprint)
			if c == nil {
				t.Fatalf("no item %q", legalCheckImprint)
			}
			if c.OK != tc.want {
				t.Errorf("%s %s (linked %v): done = %v, want %v", tc.status, tc.slug, tc.linked, c.OK, tc.want)
			}
		})
	}
}

func TestThePrivacyCheckIsAlwaysThere(t *testing.T) {
	for _, set := range []string{"", "eu", "us"} {
		t.Run("set="+set, func(t *testing.T) {
			h, _, database, ws := newTestAdmin(t)
			stored := legalCheckWebsite(t, h, database, ws, set, "de")
			c := legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckPrivacy)
			if c == nil {
				t.Fatalf("no item %q", legalCheckPrivacy)
			}
			if c.OK {
				t.Errorf("done without a privacy page")
			}
		})
	}
	for _, tc := range []struct {
		lang, slug, status string
		want               bool
	}{
		{"de", "datenschutz", "published", true},
		{"fr", "protection-des-donnees", "published", true},
		{"en", "privacy-policy", "published", true},
		{"de", "datenschutz", "draft", false},
	} {
		t.Run(tc.slug+"/"+tc.status, func(t *testing.T) {
			h, _, database, ws := newTestAdmin(t)
			stored := legalCheckWebsite(t, h, database, ws, "eu", tc.lang)
			legalCheckFooter(t, h, database, ws.ID, tc.slug, tc.status, true)
			c := legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckPrivacy)
			if c == nil || c.OK != tc.want {
				t.Errorf("%+v, want done = %v", c, tc.want)
			}
		})
	}
}

// The legal checks want the page itself (IN-06 of the 33.1 review). A slug
// that merely contains a role slug — a post "privacy-tips", the compare draft
// "datenschutz-vorlage" once published — used to pass, and so did a page that
// still carries the template notice, which the legal card calls an unfilled
// template. A role slug with the counter CreatePage appends ("datenschutz-2")
// is still the page.
func TestTheLegalCheckWantsTheRealPage(t *testing.T) {
	notice := "**" + privacy.TemplateNotice("de") + "** Rest"
	for _, tc := range []struct {
		name, slug, text string
		want             bool
	}{
		{"the role slug", "datenschutz", "Unser Text.", true},
		{"the role slug with a counter", "datenschutz-2", "Unser Text.", true},
		{"a post about privacy", "privacy-tips", "Tipps.", false},
		{"the published compare draft", "datenschutz-vorlage", "Unser Text.", false},
		{"an unfilled template", "datenschutz", notice, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, database, ws := newTestAdmin(t)
			stored := legalCheckWebsite(t, h, database, ws, "eu", "de")
			legalCheckFooterText(t, h, database, ws.ID, tc.slug, "published", tc.text, true)
			c := legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckPrivacy)
			if c == nil || c.OK != tc.want {
				t.Errorf("%s (%q): %+v, want done = %v", tc.name, tc.slug, c, tc.want)
			}
		})
	}
}

func TestTheDoNotSellCheckOnlyWithUS(t *testing.T) {
	for _, set := range []string{"", "eu", "eu,ch,uk"} {
		h, _, database, ws := newTestAdmin(t)
		stored := legalCheckWebsite(t, h, database, ws, set, "de")
		if c := legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckDNS); c != nil {
			t.Errorf("set %q: the Do-Not-Sell item is there", set)
		}
	}

	h, _, database, ws := newTestAdmin(t)
	stored := legalCheckWebsite(t, h, database, ws, "us", "en")
	c := legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckDNS)
	if c == nil {
		t.Fatalf("with US there is no Do-Not-Sell item")
	}
	if c.OK {
		t.Errorf("done without the page")
	}
	if c.Hint != "With the United States chosen, this page says that nothing is sold or shared and shows whether a visitor's browser sent the Global Privacy Control signal (Cal. Civ. Code § 1798.135)." {
		t.Errorf("hint %q", c.Hint)
	}
	legalCheckFooter(t, h, database, ws.ID, "do-not-sell-or-share", "published", true)
	c = legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckDNS)
	if c == nil || !c.OK {
		t.Errorf("a published Do-Not-Sell page in the footer: %+v", c)
	}
	// The privacy check does not take the Do-Not-Sell page for a privacy page.
	if p := legalCheckFind(h.siteChecks(context.Background(), stored, nil), legalCheckPrivacy); p == nil || p.OK {
		t.Errorf("the Do-Not-Sell page passes the privacy check: %+v", p)
	}
}

func TestTheHintsFollowTheFrameworks(t *testing.T) {
	const (
		imprintEU      = "German law (§ 5 DDG) requires an imprint reachable from every page; the footer menu does that."
		imprintCH      = "Swiss law (Art. 3 para. 1 let. s UWG) requires your identity and contact address, e-mail included; the footer menu makes it reachable from every page."
		imprintUK      = "UK law (Electronic Commerce Regulations 2002, reg. 6) requires your name, address and e-mail on the website; the footer menu makes them reachable from every page."
		imprintGeneral = "Say who runs this website and how to reach them; the footer menu makes it reachable from every page."
		privacyEU      = "The GDPR (Art. 13) requires a privacy notice at the moment data is collected; the footer menu links it from every page."
		privacyCH      = "The Swiss FADP (Art. 19) requires a privacy notice; the footer menu links it from every page."
		privacyUK      = "UK GDPR (Art. 13) requires a privacy notice; the footer menu links it from every page."
		privacyUS      = "California law (CalOPPA) requires a conspicuous link containing the word “privacy”; the footer menu provides it."
		privacyGeneral = "A privacy notice tells visitors what this website does with their data; the footer menu links it from every page."
	)
	for _, tc := range []struct {
		set, imprint, privacy string
	}{
		{"eu", imprintEU, privacyEU},
		{"eu,ch,uk,us", imprintEU, privacyEU},
		{"ch", imprintCH, privacyCH},
		{"ch,us", imprintCH, privacyCH},
		{"uk", imprintUK, privacyUK},
		{"uk,us", imprintUK, privacyUK},
		{"us", imprintGeneral, privacyUS},
		{"", imprintGeneral, privacyGeneral},
	} {
		t.Run("set="+tc.set, func(t *testing.T) {
			h, _, database, ws := newTestAdmin(t)
			stored := legalCheckWebsite(t, h, database, ws, tc.set, "de")
			checks := h.siteChecks(context.Background(), stored, nil)
			if c := legalCheckFind(checks, legalCheckImprint); c == nil || c.Hint != tc.imprint {
				t.Errorf("imprint hint %+v, want %q", c, tc.imprint)
			}
			if c := legalCheckFind(checks, legalCheckPrivacy); c == nil || c.Hint != tc.privacy {
				t.Errorf("privacy hint %+v, want %q", c, tc.privacy)
			}
		})
	}
}

func TestOpLaunchChecklistCarriesTheLegalItems(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()
	stored := legalCheckWebsite(t, h, database, ws, "eu,us", "de")

	screen := h.siteChecks(ctx, stored, nil)
	var want []string
	for _, c := range screen {
		want = append(want, c.Label)
	}
	ops, err := h.OpLaunchChecklist(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range ops {
		label, _ := c["check"].(string)
		got = append(got, label)
	}
	if !slices.Equal(got, want) {
		t.Errorf("MCP labels %v, want the screen's %v", got, want)
	}

	// The four legal items stand together, in this order, where the imprint
	// item stood.
	i := slices.Index(want, legalCheckFrameworks)
	if i < 0 || i+3 >= len(want) || !slices.Equal(want[i:i+4], []string{legalCheckFrameworks, legalCheckImprint, legalCheckPrivacy, legalCheckDNS}) {
		t.Errorf("the legal items are %v", want)
	}
}
