package admin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// The Op… methods of ops_site.go are what an AI key reaches instead of the
// website, design, template and wording screens. They share their steps with
// those screens; these tests hold that the shared steps do what the screens
// promise when there is no request behind them.

// A website created through the tool gets what the form gives: the default
// template switched on, and the start page, the two legal drafts and both
// menus — or nothing at all when asked for nothing.
func TestOpCreateWebsiteSeedsWhatTheFormSeeds(t *testing.T) {
	h, _, _, _ := newTestAdmin(t)
	ctx := context.Background()

	ws, err := h.OpCreateWebsite(ctx, "  Hofladen  ", "Eier und Kartoffeln", true)
	if err != nil {
		t.Fatalf("OpCreateWebsite: %v", err)
	}
	if ws.Name != "Hofladen" {
		t.Errorf("name = %q", ws.Name)
	}
	pages, _, err := h.pages.ListPages(ctx, ws.ID, page.ListFilter{Locale: "*", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	slugs := map[string]string{}
	for _, p := range pages {
		slugs[p.Slug] = p.Status
	}
	for _, sp := range starterPages {
		if slugs[sp.slug] != sp.status {
			t.Errorf("starter page %q: status %q, want %q", sp.slug, slugs[sp.slug], sp.status)
		}
	}
	for _, loc := range []string{"main", "footer"} {
		if tree, err := h.menuStore.GetMenuTree(ctx, ws.ID, loc); err != nil || len(tree) == 0 {
			t.Errorf("menu %s: %d entries, %v", loc, len(tree), err)
		}
	}

	empty, err := h.OpCreateWebsite(ctx, "Import", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if pages, _, _ := h.pages.ListPages(ctx, empty.ID, page.ListFilter{Locale: "*", Page: 1, PerPage: 50}); len(pages) != 0 {
		t.Errorf("without starter content the website has %d pages", len(pages))
	}

	if _, err := h.OpCreateWebsite(ctx, "   ", "", true); !errors.Is(err, errWebsiteNameMissing) {
		t.Errorf("a website without a name: %v", err)
	}
}

// A partial change leaves every other setting where it was, and the values go
// through the form's rules.
func TestOpUpdateWebsiteChangesOnlyWhatIsGiven(t *testing.T) {
	h, _, _, ws := newTestAdmin(t)
	ctx := context.Background()

	if _, err := h.OpUpdateWebsite(ctx, ws.ID, map[string]string{
		"city": "Lüneburg", "contact_email": "hof@example.org", "timezone": "Europe/Vienna",
		"canonical_redirect": "on",
	}); err != nil {
		t.Fatalf("OpUpdateWebsite: %v", err)
	}
	got, err := h.OpUpdateWebsite(ctx, ws.ID, map[string]string{"phone": "0123", "locale": "tlh"})
	if err != nil {
		t.Fatalf("OpUpdateWebsite: %v", err)
	}
	if got.City != "Lüneburg" || got.ContactEmail != "hof@example.org" || got.TimeZone != "Europe/Vienna" ||
		!got.CanonicalRedirect || got.Name != ws.Name {
		t.Errorf("a setting nobody mentioned changed: %+v", got)
	}
	if got.Phone != "0123" {
		t.Errorf("phone = %q", got.Phone)
	}
	if got.Locale != tmpl.DefaultLocale {
		t.Errorf("an unknown language was stored as %q, want the default", got.Locale)
	}

	if _, err := h.OpUpdateWebsite(ctx, ws.ID, map[string]string{"name": ""}); !errors.Is(err, errWebsiteNameMissing) {
		t.Errorf("an empty name: %v", err)
	}
	if _, err := h.OpUpdateWebsite(ctx, ws.ID, map[string]string{"shoe_size": "44"}); err == nil {
		t.Error("an unknown setting was accepted")
	}
}

// A favicon is an image of this website — never one of another.
func TestOpUpdateWebsiteRefusesAnotherWebsitesImage(t *testing.T) {
	h, _, _, ws := newTestAdmin(t)
	ctx := context.Background()
	other, err := h.domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := h.mediaStore.Create(ctx, other.ID, "a.png", "a.png", "image/png", 10, "h1")
	if err != nil {
		t.Fatal(err)
	}
	mine, err := h.mediaStore.Create(ctx, ws.ID, "b.png", "b.png", "image/png", 10, "h2")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := h.OpUpdateWebsite(ctx, ws.ID, map[string]string{
		"favicon_media_id": strconv.FormatInt(theirs.ID, 10),
	}); err == nil {
		t.Error("another website's image became the favicon")
	}
	got, err := h.OpUpdateWebsite(ctx, ws.ID, map[string]string{
		"favicon_media_id": strconv.FormatInt(mine.ID, 10),
	})
	if err != nil {
		t.Fatalf("own image: %v", err)
	}
	if got.FaviconMediaID == nil || *got.FaviconMediaID != mine.ID {
		t.Errorf("favicon = %v", got.FaviconMediaID)
	}
}

func TestOpDeleteWebsiteRemovesItAndItsFiles(t *testing.T) {
	h, _, _, ws := newTestAdmin(t)
	ctx := context.Background()
	dir := filepath.Join(h.cfg.DataDir, "media", strconv.FormatInt(ws.ID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := h.OpDeleteWebsite(ctx, ws.ID); err != nil {
		t.Fatalf("OpDeleteWebsite: %v", err)
	}
	if got, _ := h.domains.GetWebsite(ctx, ws.ID); got != nil {
		t.Error("the website is still there")
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("the media directory is still there")
	}
}

func TestOpLaunchChecklistIsTheSettingsScreensList(t *testing.T) {
	h, _, _, ws := newTestAdmin(t)
	ctx := context.Background()

	checks, err := h.OpLaunchChecklist(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(h.siteChecks(ctx, ws, nil)); len(checks) != want || want == 0 {
		t.Fatalf("%d checks, want %d", len(checks), want)
	}
	if ok, _ := checks[0]["ok"].(bool); ok {
		t.Error("a website without a domain passes \"at least one domain\"")
	}
	if _, err := h.OpLaunchChecklist(ctx, 9999); err == nil {
		t.Error("a website that does not exist has a checklist")
	}
}

// The design goes through the form's reader and Sanitize: a value the CSS
// cannot use is dropped, the right ones stay, and a reset drops everything.
func TestOpSetDesignKeepsOnlyWhatTheRulesAllow(t *testing.T) {
	h, _, _, ws := newTestAdmin(t)
	ctx := context.Background()

	kept, err := h.OpSetDesign(ctx, ws.ID, map[string]string{
		"token_ink": "#112233", "token_paper": "red", "token_brand": "#ABC",
		"token_font": "serif", "token_measure": "70", "token_radius": "99",
		"use_colours": "on",
	})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Ink != "#112233" || kept.Paper != "" || kept.Brand != "#abc" || kept.Font != "serif" ||
		kept.Measure != 70 || kept.Radius != -1 {
		t.Errorf("kept %+v", kept)
	}
	stored, _ := h.domains.GetWebsite(ctx, ws.ID)
	if stored.TokenInk != "#112233" || stored.TokenMeasure != 70 {
		t.Errorf("stored %+v", stored)
	}

	if err := h.OpResetDesign(ctx, ws.ID); err != nil {
		t.Fatal(err)
	}
	stored, _ = h.domains.GetWebsite(ctx, ws.ID)
	if stored.TokenInk != "" || stored.TokenFont != "" || stored.TokenMeasure != 0 || stored.TokenRadius != -1 {
		t.Errorf("after reset %+v", stored)
	}
}

// Uploading through the tool is uploading through the screen: the same
// refusals, the same record, and an archive too large is refused before it is
// opened.
func TestOpUploadTemplateRunsTheUploadChecks(t *testing.T) {
	h, _, _, wsID := templateAdmin(t)
	ctx := context.Background()

	bad := themeZip(t, map[string]string{"page.html": `{{define "content"}}<script>alert(1)</script>{{end}}`})
	if _, err := h.OpUploadTemplate(ctx, "Kaputt", bad); err == nil || !strings.Contains(err.Error(), "JavaScript") {
		t.Errorf("a template with a script: %v", err)
	}
	if stored, _ := h.tmplStore.GetBySlug(ctx, "kaputt"); stored != nil {
		t.Error("the refused template was recorded")
	}

	got, err := h.OpUploadTemplate(ctx, "Werkstatt", themeZip(t, nil))
	if err != nil {
		t.Fatalf("OpUploadTemplate: %v", err)
	}
	if _, err := h.OpUploadTemplate(ctx, "Werkstatt", themeZip(t, nil)); err == nil {
		t.Error("a second template of the same name was accepted")
	}

	h.cfg.MaxTemplateSize = 10
	if _, err := h.OpUploadTemplate(ctx, "Gross", themeZip(t, nil)); err == nil {
		t.Error("an archive over the limit was accepted")
	}

	if err := h.OpActivateTemplate(ctx, wsID, 99999); !errors.Is(err, errNoSuchTemplate) {
		t.Errorf("activating a template that does not exist: %v", err)
	}
	if err := h.OpActivateTemplate(ctx, wsID, got.ID); err != nil {
		t.Fatal(err)
	}
	list, active, err := h.OpTemplates(ctx)
	if err != nil || len(list) == 0 {
		t.Fatalf("OpTemplates: %d, %v", len(list), err)
	}
	if active[wsID] != got.ID {
		t.Errorf("active template = %d, want %d", active[wsID], got.ID)
	}

	if err := h.OpDeleteTemplate(ctx, got.ID); err == nil {
		t.Error("a template in use was deleted")
	}
}

// The words go through the same filter as the screen's form: a key the theme
// does not use is skipped, a real one is stored and read back.
func TestOpSetWordingWritesWhatTheThemeAsksFor(t *testing.T) {
	h, _, _, ws := newTestAdmin(t)
	ctx := context.Background()

	keys, _ := h.themeVocabulary(ctx, ws)
	if len(keys) == 0 {
		t.Fatal("the shipped theme asks for no words")
	}
	saved, skipped, full, err := h.OpSetWording(ctx, ws.ID, ws.Locale, map[string]string{
		keys[0]: "Mein Wort", "Gibt es nicht": "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved != 1 || full || len(skipped) != 1 || skipped[0] != "Gibt es nicht" {
		t.Errorf("saved %d, skipped %v, full %v", saved, skipped, full)
	}

	view, err := h.OpWording(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	langs := view["languages"].([]map[string]any)
	found := false
	for _, w := range langs[0]["words"].([]map[string]any) {
		if w["key"] == keys[0] && w["own"] == "Mein Wort" {
			found = true
		}
	}
	if !found {
		t.Error("the own word does not appear in the wording")
	}
}
