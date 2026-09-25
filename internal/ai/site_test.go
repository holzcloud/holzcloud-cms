package ai

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/design"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
)

// The website tools, wired to a stand-in for the admin handler. What the
// handler does with the values is tested in internal/admin (ops_site_test.go);
// here it is the wiring: who may call what, which website a key reaches, and
// that the tool hands the handler what the form would.

// fakeSite records what the tools hand the admin handler.
type fakeSite struct {
	domains  *domain.Store
	created  []string
	updates  []map[string]string
	deleted  []int64
	design   []map[string]string
	resets   int
	uploads  [][]byte
	words    map[string]string
	activate [2]int64
	logged   []string
}

func (f *fakeSite) OpLog(_ context.Context, _ string, e activity.Entry) {
	f.logged = append(f.logged, e.Action)
}
func (f *fakeSite) OpChanged(int64) {}

func (f *fakeSite) OpCreateWebsite(ctx context.Context, name, desc string, starter bool) (*domain.Website, error) {
	f.created = append(f.created, name)
	return f.domains.CreateWebsite(ctx, name, desc)
}
func (f *fakeSite) OpUpdateWebsite(ctx context.Context, id int64, ch map[string]string) (*domain.Website, error) {
	f.updates = append(f.updates, ch)
	return f.domains.GetWebsite(ctx, id)
}
func (f *fakeSite) OpDeleteWebsite(_ context.Context, id int64) error {
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeSite) OpLaunchChecklist(context.Context, int64) ([]map[string]any, error) {
	return []map[string]any{{"check": "At least one domain", "ok": false, "hint": "x"}}, nil
}
func (f *fakeSite) OpTemplates(context.Context) ([]tmplmgr.Template, map[int64]int64, error) {
	return []tmplmgr.Template{{ID: 1, Name: "Default", Slug: "default", IsBuiltin: true}}, map[int64]int64{}, nil
}
func (f *fakeSite) OpActivateTemplate(_ context.Context, ws, t int64) error {
	f.activate = [2]int64{ws, t}
	return nil
}
func (f *fakeSite) OpSetDesign(_ context.Context, _ int64, form map[string]string) (design.Tokens, error) {
	f.design = append(f.design, form)
	return design.Tokens{Ink: form["token_ink"], Radius: -1}, nil
}
func (f *fakeSite) OpResetDesign(context.Context, int64) error { f.resets++; return nil }
func (f *fakeSite) OpWording(context.Context, int64) (map[string]any, error) {
	return map[string]any{"languages": []any{}}, nil
}
func (f *fakeSite) OpSetWording(_ context.Context, _ int64, _ string, w map[string]string) (int, []string, bool, error) {
	f.words = w
	return len(w), nil, false, nil
}
func (f *fakeSite) OpUploadTemplate(_ context.Context, name string, archive []byte) (*tmplmgr.Template, error) {
	f.uploads = append(f.uploads, archive)
	return &tmplmgr.Template{ID: 7, Name: name, Slug: strings.ToLower(name)}, nil
}
func (f *fakeSite) OpDeleteTemplate(context.Context, int64) error { return nil }

// siteSetUp builds a server with two websites and four keys: admin, content,
// read-only, and content for the first website only.
type siteKeys struct{ admin, content, read, onlyFirst string }

func siteSetUp(t *testing.T) (*httptest.Server, *fakeSite, *domain.Store, int64, int64, siteKeys) {
	t.Helper()
	database := newTestDB(t)
	domains := domain.NewStore(database)
	ctx := context.Background()
	first, _ := domains.CreateWebsite(ctx, "Eins", "")
	second, _ := domains.CreateWebsite(ctx, "Zwei", "")

	tokens := NewStore(database)
	var k siteKeys
	var err error
	if k.admin, _, err = tokens.IssueLevel(ctx, "admin", 0, LevelAdmin, 0); err != nil {
		t.Fatal(err)
	}
	if k.content, _, err = tokens.Issue(ctx, "inhalt", 0, true, 0); err != nil {
		t.Fatal(err)
	}
	if k.read, _, err = tokens.Issue(ctx, "lesen", 0, false, 0); err != nil {
		t.Fatal(err)
	}
	if k.onlyFirst, _, err = tokens.Issue(ctx, "eins", first.ID, true, 0); err != nil {
		t.Fatal(err)
	}

	fake := &fakeSite{domains: domains}
	ts := httptest.NewServer(NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains, Pages: page.NewStore(database), Media: media.NewStore(database), Ops: fake,
	})))
	t.Cleanup(ts.Close)
	return ts, fake, domains, first.ID, second.ID, k
}

func TestGetWebsiteShowsSettingsAndStaysWithItsKey(t *testing.T) {
	ts, _, domains, first, second, k := siteSetUp(t)
	if _, err := domains.AddDomain(context.Background(), first, "eins.example", true); err != nil {
		t.Fatal(err)
	}

	got, failed := callTool(t, ts, k.read, "get_website", map[string]any{"website": first})
	if failed {
		t.Fatalf("get_website: %v", got["text"])
	}
	if got["name"] != "Eins" || got["time_zone"] == nil {
		t.Errorf("answer %v", got)
	}
	if doms := got["domains"].([]any); len(doms) != 1 {
		t.Errorf("domains %v", doms)
	}

	if _, failed := callTool(t, ts, k.onlyFirst, "get_website", map[string]any{"website": second}); !failed {
		t.Error("a key for the first website read the second")
	}
}

func TestUpdateWebsiteHandsTheFormItsFieldNames(t *testing.T) {
	ts, fake, _, first, second, k := siteSetUp(t)

	got, failed := callTool(t, ts, k.content, "update_website", map[string]any{
		"website": first, "city": "Lüneburg", "canonical_redirect": true,
		"time_zone": "Europe/Vienna", "favicon_media_id": 0,
	})
	if failed {
		t.Fatalf("update_website: %v", got["text"])
	}
	if len(fake.updates) != 1 {
		t.Fatalf("%d updates", len(fake.updates))
	}
	want := map[string]string{"city": "Lüneburg", "canonical_redirect": "on", "timezone": "Europe/Vienna", "favicon_media_id": ""}
	for k, v := range want {
		if fake.updates[0][k] != v {
			t.Errorf("%s = %q, want %q", k, fake.updates[0][k], v)
		}
	}
	if len(fake.updates[0]) != len(want) {
		t.Errorf("more was handed on than was given: %v", fake.updates[0])
	}
	if len(fake.logged) != 1 || fake.logged[0] != activity.ActionWebsiteUpdate {
		t.Errorf("logged %v", fake.logged)
	}

	if _, failed := callTool(t, ts, k.content, "update_website", map[string]any{"website": first, "shoe": 1}); !failed {
		t.Error("an unknown setting was accepted")
	}
	if _, failed := callTool(t, ts, k.read, "update_website", map[string]any{"website": first, "city": "X"}); !failed {
		t.Error("a read-only key changed a website")
	}
	if _, failed := callTool(t, ts, k.onlyFirst, "update_website", map[string]any{"website": second, "city": "X"}); !failed {
		t.Error("a key for the first website changed the second")
	}
	if len(fake.updates) != 1 {
		t.Errorf("a refused call reached the handler: %d updates", len(fake.updates))
	}
}

// What the web puts behind requireAdmin is Admin here: a content key neither
// sees nor calls it.
func TestWebsiteManagementNeedsAnAdminKey(t *testing.T) {
	ts, fake, _, first, _, k := siteSetUp(t)

	res := call(t, ts, k.content, "tools/list", nil)
	offered := map[string]bool{}
	for _, raw := range res["result"].(map[string]any)["tools"].([]any) {
		offered[raw.(map[string]any)["name"].(string)] = true
	}
	for _, name := range []string{"create_website", "delete_website", "add_domain", "remove_domain",
		"set_primary_domain", "set_design", "reset_design", "upload_template", "delete_template",
		"get_template_spec"} {
		if offered[name] {
			t.Errorf("a content key is offered %s", name)
		}
	}
	for _, name := range []string{"get_website", "update_website", "activate_template", "set_wording",
		"launch_checklist", "translation_matrix", "list_templates", "get_design", "get_wording"} {
		if !offered[name] {
			t.Errorf("a content key is not offered %s", name)
		}
	}

	if _, failed := callTool(t, ts, k.content, "create_website", map[string]any{"name": "Neu"}); !failed {
		t.Error("a content key created a website")
	}
	if _, failed := callTool(t, ts, k.content, "add_domain", map[string]any{"website": first, "domain": "x.example"}); !failed {
		t.Error("a content key added a domain")
	}
	if len(fake.created) != 0 {
		t.Error("a refused call reached the handler")
	}

	got, failed := callTool(t, ts, k.admin, "create_website", map[string]any{"name": "Neu"})
	if failed {
		t.Fatalf("create_website: %v", got["text"])
	}
	if len(fake.created) != 1 || got["name"] != "Neu" {
		t.Errorf("created %v, answer %v", fake.created, got)
	}
}

func TestDeleteWebsiteWantsConfirmation(t *testing.T) {
	ts, fake, _, first, _, k := siteSetUp(t)

	if _, failed := callTool(t, ts, k.admin, "delete_website", map[string]any{"website": first}); !failed {
		t.Error("deleted without confirm")
	}
	if len(fake.deleted) != 0 {
		t.Fatal("the handler was called without confirm")
	}
	if got, failed := callTool(t, ts, k.admin, "delete_website", map[string]any{"website": first, "confirm": true}); failed {
		t.Fatalf("delete_website: %v", got["text"])
	}
	if len(fake.deleted) != 1 || fake.deleted[0] != first {
		t.Errorf("deleted %v", fake.deleted)
	}
}

// Domains go through the store; an id from outside must not reach another
// website's domain.
func TestDomainsStayWithTheirWebsite(t *testing.T) {
	ts, _, domains, first, second, k := siteSetUp(t)
	ctx := context.Background()

	got, failed := callTool(t, ts, k.admin, "add_domain", map[string]any{"website": first, "domain": "eins.example"})
	if failed {
		t.Fatalf("add_domain: %v", got["text"])
	}
	if _, failed := callTool(t, ts, k.admin, "add_domain", map[string]any{"website": second, "domain": "eins.example"}); !failed {
		t.Error("the same domain was added to a second website")
	}
	theirs, err := domains.AddDomain(ctx, second, "zwei.example", false)
	if err != nil {
		t.Fatal(err)
	}

	if _, failed := callTool(t, ts, k.admin, "remove_domain", map[string]any{
		"website": first, "domain_id": theirs.ID, "confirm": true,
	}); !failed {
		t.Error("the second website's domain was removed through the first")
	}
	if _, failed := callTool(t, ts, k.admin, "set_primary_domain", map[string]any{
		"website": first, "domain_id": theirs.ID,
	}); !failed {
		t.Error("the second website's domain was made primary for the first")
	}
	if list, _ := domains.ListDomains(ctx, second); len(list) != 1 {
		t.Errorf("the second website has %d domains", len(list))
	}

	if got, failed := callTool(t, ts, k.admin, "set_primary_domain", map[string]any{
		"website": second, "domain_id": theirs.ID,
	}); failed {
		t.Fatalf("set_primary_domain: %v", got["text"])
	}
	if p, _ := domains.PrimaryDomain(ctx, second); p != "zwei.example" {
		t.Errorf("primary = %q", p)
	}
	if got, failed := callTool(t, ts, k.admin, "remove_domain", map[string]any{
		"website": second, "domain_id": theirs.ID, "confirm": true,
	}); failed {
		t.Fatalf("remove_domain: %v", got["text"])
	}
	if list, _ := domains.ListDomains(ctx, second); len(list) != 0 {
		t.Errorf("the domain is still there: %v", list)
	}
}

func TestSetDesignLaysChangesOverWhatIsStored(t *testing.T) {
	ts, fake, domains, first, _, k := siteSetUp(t)
	if err := domains.UpdateDesignTokens(context.Background(), first, domain.DesignTokens{
		Ink: "#111111", Font: "serif", Measure: 70, Radius: -1,
	}); err != nil {
		t.Fatal(err)
	}

	if _, failed := callTool(t, ts, k.content, "set_design", map[string]any{"website": first, "ink": "#222"}); !failed {
		t.Error("a content key changed the design tokens")
	}
	got, failed := callTool(t, ts, k.admin, "set_design", map[string]any{"website": first, "brand": "#123456", "radius": 4})
	if failed {
		t.Fatalf("set_design: %v", got["text"])
	}
	form := fake.design[0]
	if form["token_ink"] != "#111111" || form["token_font"] != "serif" || form["token_measure"] != "70" ||
		form["token_brand"] != "#123456" || form["token_radius"] != "4" || form["use_colours"] != "on" {
		t.Errorf("form %v", form)
	}

	if _, failed := callTool(t, ts, k.admin, "set_design", map[string]any{"website": first, "template_colours": true}); failed {
		t.Fatal("set_design with template_colours failed")
	}
	if fake.design[1]["use_colours"] != "" {
		t.Error("template_colours did not drop the colours")
	}
}

func TestUploadTemplateDecodesTheArchive(t *testing.T) {
	ts, fake, _, _, _, k := siteSetUp(t)

	if _, failed := callTool(t, ts, k.admin, "upload_template", map[string]any{"name": "W", "archive": "%%%"}); !failed {
		t.Error("an archive that is not base64 was accepted")
	}
	got, failed := callTool(t, ts, k.admin, "upload_template", map[string]any{
		"name": "Werkstatt", "archive": base64.StdEncoding.EncodeToString([]byte("PK-zip")),
	})
	if failed {
		t.Fatalf("upload_template: %v", got["text"])
	}
	if len(fake.uploads) != 1 || string(fake.uploads[0]) != "PK-zip" {
		t.Errorf("uploads %q", fake.uploads)
	}
	if _, failed := callTool(t, ts, k.content, "upload_template", map[string]any{"name": "W", "archive": "UEs="}); !failed {
		t.Error("a content key uploaded a template")
	}
}

func TestSetWordingOnlyInTheWebsitesLanguages(t *testing.T) {
	ts, fake, _, first, second, k := siteSetUp(t)

	if _, failed := callTool(t, ts, k.content, "set_wording", map[string]any{
		"website": first, "language": "ja", "words": map[string]any{"Cart": "Korb"},
	}); !failed {
		t.Error("a word was set in a language the website is not published in")
	}
	got, failed := callTool(t, ts, k.content, "set_wording", map[string]any{
		"website": first, "language": "de", "words": map[string]any{"Cart": "Korb"},
	})
	if failed {
		t.Fatalf("set_wording: %v", got["text"])
	}
	if fake.words["Cart"] != "Korb" {
		t.Errorf("words %v", fake.words)
	}
	if _, failed := callTool(t, ts, k.onlyFirst, "set_wording", map[string]any{
		"website": second, "language": "de", "words": map[string]any{"Cart": "Korb"},
	}); !failed {
		t.Error("a key for the first website set words on the second")
	}
}

// The matrix comes straight from the page store, main language by its tag.
func TestTranslationMatrixNamesTheGaps(t *testing.T) {
	ts, _, domains, first, _, k := siteSetUp(t)
	ctx := context.Background()
	ws, _ := domains.GetWebsite(ctx, first)
	if err := domains.UpdateSettings(ctx, first, domain.Settings{Locale: ws.Locale, TimeZone: ws.TimeZone,
		ExtraLocales: "fr", OfflineMode: "notfound", PostsPerPage: 10, Country: "DE"}); err != nil {
		t.Fatal(err)
	}
	if got, failed := callTool(t, ts, k.content, "create_page", map[string]any{
		"website": first, "title": "Hallo", "markdown": "x",
	}); failed {
		t.Fatalf("create_page: %v", got["text"])
	}

	got, failed := callTool(t, ts, k.read, "translation_matrix", map[string]any{"website": first})
	if failed {
		t.Fatalf("translation_matrix: %v", got["text"])
	}
	langs := got["languages"].([]any)
	if len(langs) != 2 {
		t.Fatalf("languages %v", langs)
	}
	fr := langs[1].(map[string]any)
	if fr["language"] != "fr" || fr["missing"].(float64) != 1 {
		t.Errorf("french column %v", fr)
	}
}
