package ai_test

// The page tools, end to end: a real admin handler as Ops, a real database, and
// the JSON-RPC server in front. An external test package because internal/admin
// imports this one — the handler cannot be built from inside package ai.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/admin"
	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// pagesRig is one installation with two websites and four keys.
type pagesRig struct {
	ts       *httptest.Server
	pages    *page.Store
	database *db.DB
	site     int64 // the website the tests work on
	other    int64 // a second website
	content  string
	readOnly string
	admin    string
	foreign  string // a content key for the other website only
}

func newPagesRig(t *testing.T) pagesRig {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	ctx := context.Background()

	domains := domain.NewStore(database)
	site, err := domains.CreateWebsite(ctx, "Testhof", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := domains.CreateWebsite(ctx, "Nachbar", "")
	if err != nil {
		t.Fatal(err)
	}
	pages := page.NewStore(database)
	mediaStore := media.NewStore(database)

	// Cheap hashing: a page password is hashed with these.
	params := auth.Argon2Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	h := admin.NewHandler(database, nil, nil, params, domains, domain.NewResolver(domains), pages,
		nil, menu.NewStore(database), mediaStore, snippet.NewStore(database), term.NewStore(database),
		sharelink.New([]byte("test")),
		tmpl.NewLoader(dir, os.DirFS("../../cmd/holzcloud/templates/public/default"), nil, nil),
		&config.Config{DataDir: dir}, nil, nil)

	tokens := ai.NewStore(database)
	issue := func(name string, websiteID int64, level ai.Level) string {
		secret, _, err := tokens.IssueLevel(ctx, name, websiteID, level, 0)
		if err != nil {
			t.Fatalf("IssueLevel: %v", err)
		}
		return secret
	}
	rig := pagesRig{
		pages: pages, database: database, site: site.ID, other: other.ID,
		content:  issue("inhalt", 0, ai.LevelContent),
		readOnly: issue("lesen", 0, ai.LevelRead),
		admin:    issue("verwaltung", 0, ai.LevelAdmin),
		foreign:  issue("nachbar", other.ID, ai.LevelContent),
	}
	srv := ai.NewServer(tokens, "Test", slog.New(slog.DiscardHandler), ai.Tools(ai.Deps{
		Domains: domains, Pages: pages, Media: mediaStore, Ops: h,
	}))
	rig.ts = httptest.NewServer(srv)
	t.Cleanup(rig.ts.Close)
	return rig
}

// call runs one tool and hands back its result, or its error text and true.
func (r pagesRig) call(t *testing.T, key, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res := r.rpc(t, key, "tools/call", map[string]any{"name": name, "arguments": args})
	result, _ := res["result"].(map[string]any)
	if result == nil {
		t.Fatalf("%s: no result: %v", name, res)
	}
	failed, _ := result["isError"].(bool)
	text, _ := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if failed {
		return map[string]any{"text": text}, true
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s: %q is not JSON: %v", name, text, err)
	}
	return out, false
}

// must is call for a step that has to work.
func (r pagesRig) must(t *testing.T, key, name string, args map[string]any) map[string]any {
	t.Helper()
	out, failed := r.call(t, key, name, args)
	if failed {
		t.Fatalf("%s: %v", name, out["text"])
	}
	return out
}

func (r pagesRig) rpc(t *testing.T, key, method string, params any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, _ := http.NewRequest(http.MethodPost, r.ts.URL, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := r.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (r pagesRig) newPage(t *testing.T, websiteID int64, title, markdown string) *page.Page {
	t.Helper()
	html, _ := page.RenderMarkdown(markdown)
	p, err := r.pages.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: page.Slugify(title),
		Markdown: markdown, HTML: html, Status: "draft",
		Meta: page.PageMeta{Excerpt: "Eigene Zusammenfassung"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (r pagesRig) reload(t *testing.T, id int64) *page.Page {
	t.Helper()
	p, err := r.pages.GetPage(context.Background(), id)
	if err != nil || p == nil {
		t.Fatalf("GetPage(%d): %v", id, err)
	}
	return p
}

func num(v any) int64 { f, _ := v.(float64); return int64(f) }

// Blocks written through the tools are rendered by the editor's own save: the
// HTML carries the block markup, the plain text feeds the excerpt and search,
// and the state before is a revision.
func TestBlocksAreRenderedLikeTheEditorSaves(t *testing.T) {
	r := newPagesRig(t)
	p := r.newPage(t, r.site, "Hof", "Willkommen auf dem Hof.")

	// Inserting into a markdown page turns its text into the first block.
	out := r.must(t, r.content, "insert_block", map[string]any{
		"id": p.ID, "block": map[string]any{"type": "zitat", "text": "Gutes Holz", "source": "Oma"},
	})
	if num(out["blocks"]) != 2 {
		t.Fatalf("blocks = %v, want 2", out["blocks"])
	}
	got := r.reload(t, p.ID)
	if !strings.Contains(got.ContentHTML, "hc-zitat") || !strings.Contains(got.ContentHTML, "Willkommen") {
		t.Errorf("rendered HTML = %q", got.ContentHTML)
	}
	if !strings.Contains(got.ContentMarkdown, "Gutes Holz") {
		t.Errorf("plain text for search = %q", got.ContentMarkdown)
	}
	if got.Excerpt != "Eigene Zusammenfassung" {
		t.Errorf("the excerpt was touched: %q", got.Excerpt)
	}

	read := r.must(t, r.content, "get_page_blocks", map[string]any{"id": p.ID})
	blocks, _ := read["blocks"].([]any)
	if len(blocks) != 2 || blocks[1].(map[string]any)["source"] != "Oma" {
		t.Fatalf("get_page_blocks = %v", read["blocks"])
	}

	// A stale version is refused, not overwritten.
	if _, failed := r.call(t, r.content, "set_page_blocks", map[string]any{
		"id": p.ID, "version": p.Version, "blocks": []any{map[string]any{"type": "trenner"}},
	}); !failed {
		t.Error("a save against an old version went through")
	}

	// Unknown types and pictures of another website are refused with a reason.
	if res, failed := r.call(t, r.content, "set_page_blocks", map[string]any{
		"id": p.ID, "blocks": []any{map[string]any{"type": "karussell"}},
	}); !failed || !strings.Contains(res["text"].(string), "karussell") {
		t.Errorf("unknown type: %v", res)
	}
	foreign, err := media.NewStore(r.database).Create(context.Background(), r.other,
		"x.jpg", "x.jpg", "image/jpeg", 10, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if _, failed := r.call(t, r.content, "set_page_blocks", map[string]any{
		"id": p.ID, "blocks": []any{map[string]any{"type": "bild", "media_id": foreign.ID}},
	}); !failed {
		t.Error("a picture of another website was accepted")
	}

	revs := r.must(t, r.content, "list_revisions", map[string]any{"id": p.ID})
	if list, _ := revs["revisions"].([]any); len(list) == 0 {
		t.Error("setting blocks left no revision")
	}
}

// The wastebasket: in, listed, out again, and erasing is an admin's call that
// has to be confirmed.
func TestTheWastebasket(t *testing.T) {
	r := newPagesRig(t)
	p := r.newPage(t, r.site, "Alt", "Text")

	r.must(t, r.content, "trash_page", map[string]any{"id": p.ID})
	if !r.reload(t, p.ID).InTrash() {
		t.Fatal("the page is not in the wastebasket")
	}
	list := r.must(t, r.content, "list_trash", map[string]any{"website": r.site})
	if pages, _ := list["pages"].([]any); len(pages) != 1 || pages[0].(map[string]any)["slug"] != "alt" {
		t.Errorf("list_trash = %v", list)
	}

	r.must(t, r.content, "restore_page", map[string]any{"id": p.ID})
	if back := r.reload(t, p.ID); back.InTrash() || back.Slug != "alt" {
		t.Errorf("after restore: in trash %v, slug %q", back.InTrash(), back.Slug)
	}

	r.must(t, r.content, "trash_page", map[string]any{"id": p.ID})
	if _, failed := r.call(t, r.content, "purge_page", map[string]any{"id": p.ID, "confirm": true}); !failed {
		t.Error("a content key erased a page")
	}
	if _, failed := r.call(t, r.admin, "purge_page", map[string]any{"id": p.ID}); !failed {
		t.Error("erased without confirm")
	}
	r.must(t, r.admin, "purge_page", map[string]any{"id": p.ID, "confirm": true})
	if gone, _ := r.pages.GetPage(context.Background(), p.ID); gone != nil {
		t.Error("the page is still there")
	}
}

// Restoring a revision brings back title and text and leaves the settings a
// revision does not record — the excerpt here — alone.
func TestRestoreRevisionKeepsTheSettings(t *testing.T) {
	r := newPagesRig(t)
	p := r.newPage(t, r.site, "Preise", "Alte Preise")

	r.must(t, r.content, "update_page", map[string]any{"id": p.ID, "markdown": "Neue Preise"})
	revs := r.must(t, r.content, "list_revisions", map[string]any{"id": p.ID})["revisions"].([]any)
	rev := num(revs[0].(map[string]any)["id"])

	labelled := r.must(t, r.content, "label_revision", map[string]any{"revision": rev, "label": "vor 2027"})
	if labelled["label"] != "vor 2027" {
		t.Errorf("label = %v", labelled["label"])
	}
	old := r.must(t, r.content, "read_revision", map[string]any{"revision": rev})
	if old["markdown"] != "Alte Preise" {
		t.Errorf("read_revision markdown = %v", old["markdown"])
	}

	r.must(t, r.content, "restore_revision", map[string]any{"revision": rev})
	got := r.reload(t, p.ID)
	if got.ContentMarkdown != "Alte Preise" {
		t.Errorf("markdown = %q", got.ContentMarkdown)
	}
	if got.Excerpt != "Eigene Zusammenfassung" {
		t.Errorf("the restore cleared the excerpt: %q", got.Excerpt)
	}
}

// The side panel: schedule, protection, search settings, labels, kind and
// address, each through the save the editor makes, and get_page_settings
// reporting them back.
func TestSidePanelSettings(t *testing.T) {
	r := newPagesRig(t)
	p := r.newPage(t, r.site, "Kontakt", "Ruf an.")

	r.must(t, r.content, "set_page_schedule", map[string]any{
		"id": p.ID, "publish_at": "2030-01-01T08:00Z", "unpublish_at": "2030-02-01T08:00:00Z",
	})
	if _, failed := r.call(t, r.content, "set_page_schedule", map[string]any{
		"id": p.ID, "unpublish_at": "2029-01-01T08:00Z",
	}); !failed {
		t.Error("an end before the start was accepted")
	}

	if _, failed := r.call(t, r.content, "set_page_protection", map[string]any{
		"id": p.ID, "protected": true,
	}); !failed {
		t.Error("protection without any password was accepted")
	}
	if _, failed := r.call(t, r.content, "set_page_protection", map[string]any{
		"id": p.ID, "protected": true, "password": "abc",
	}); !failed {
		t.Error("a too short password was accepted")
	}
	r.must(t, r.content, "set_page_protection", map[string]any{
		"id": p.ID, "protected": true, "password": "geheim123", "hint": "Nur für Kunden",
	})

	r.must(t, r.content, "set_page_seo", map[string]any{
		"id": p.ID, "meta_description": "Wie man uns erreicht", "noindex": true,
	})
	r.must(t, r.content, "set_page_terms", map[string]any{"id": p.ID, "labels": []string{"Hof", "hof", "Holz"}})
	r.must(t, r.content, "set_page_kind", map[string]any{"id": p.ID, "kind": "post"})
	if _, failed := r.call(t, r.content, "set_page_kind", map[string]any{"id": p.ID, "kind": "produkt"}); !failed {
		t.Error("a kind the website does not have was accepted")
	}
	moved := r.must(t, r.content, "set_page_slug", map[string]any{"id": p.ID, "slug": "anfahrt"})
	if moved["slug"] != "anfahrt" {
		t.Errorf("slug = %v", moved["slug"])
	}
	redirect, err := r.pages.LookupRedirect(context.Background(), r.site, "/kontakt")
	if err != nil || redirect == nil || redirect.ToPath != "/anfahrt" {
		t.Errorf("no redirect from the old address: %+v %v", redirect, err)
	}

	s := r.must(t, r.content, "get_page_settings", map[string]any{"id": p.ID})
	sched := s["schedule"].(map[string]any)
	if sched["publish_at"] != "2030-01-01T08:00:00Z" || sched["unpublish_at"] != "2030-02-01T08:00:00Z" {
		t.Errorf("schedule = %v", sched)
	}
	if prot := s["protection"].(map[string]any); prot["protected"] != true || prot["hint"] != "Nur für Kunden" {
		t.Errorf("protection = %v", prot)
	}
	seo := s["seo"].(map[string]any)
	if seo["meta_description"] != "Wie man uns erreicht" || seo["noindex"] != true ||
		seo["excerpt"] != "Eigene Zusammenfassung" {
		t.Errorf("seo = %v", seo)
	}
	if labels, _ := s["labels"].([]any); len(labels) != 2 {
		t.Errorf("labels = %v, want Hof and Holz", s["labels"])
	}
	if s["type"] != "post" {
		t.Errorf("type = %v", s["type"])
	}
	if s["status"] != "draft" {
		t.Errorf("a settings change published the page: %v", s["status"])
	}
}

// Copies, languages, preview links, review and the bulk menu.
func TestCopiesLanguagesLinksReviewAndBulk(t *testing.T) {
	r := newPagesRig(t)
	p := r.newPage(t, r.site, "Angebot", "Bretter")
	if _, err := r.database.Write.Exec(`UPDATE websites SET extra_locales = 'fr' WHERE id = $1`, r.site); err != nil {
		t.Fatal(err)
	}

	cp := r.must(t, r.content, "duplicate_page", map[string]any{"id": p.ID})
	if cp["status"] != "draft" || cp["slug"] == p.Slug || !strings.Contains(cp["title"].(string), "Angebot") {
		t.Errorf("duplicate = %v", cp)
	}

	fr := r.must(t, r.content, "create_translation", map[string]any{"id": p.ID, "language": "fr"})
	if fr["language"] != "fr" || num(fr["translation_of"]) != p.ID || fr["status"] != "draft" {
		t.Errorf("translation = %v", fr)
	}
	if _, failed := r.call(t, r.content, "create_translation", map[string]any{"id": p.ID, "language": "fr"}); !failed {
		t.Error("a second French version was created")
	}
	if _, failed := r.call(t, r.content, "create_translation", map[string]any{"id": p.ID, "language": "it"}); !failed {
		t.Error("a language the website does not have was accepted")
	}
	langs := r.must(t, r.content, "list_translations", map[string]any{"id": p.ID})["languages"].([]any)
	if len(langs) != 2 || num(langs[1].(map[string]any)["page"]) != num(fr["id"]) {
		t.Errorf("list_translations = %v", langs)
	}

	link := r.must(t, r.content, "create_share_link", map[string]any{"id": p.ID, "days": 3})
	if !strings.HasPrefix(link["url"].(string), "/vorschau/") {
		t.Errorf("share link = %v", link["url"])
	}

	r.must(t, r.content, "set_review", map[string]any{"id": p.ID, "pending": true})
	if r.reload(t, p.ID).ReviewState != "pending" {
		t.Error("the page is not waiting for review")
	}

	neighbour := r.newPage(t, r.other, "Fremd", "Text")
	bulk := r.must(t, r.content, "bulk_pages", map[string]any{
		"website": r.site, "action": "publish", "ids": []int64{p.ID, neighbour.ID},
	})
	if done, _ := bulk["done"].([]any); len(done) != 1 {
		t.Errorf("done = %v", bulk["done"])
	}
	if r.reload(t, neighbour.ID).Status != "draft" {
		t.Error("the bulk action reached a page of another website")
	}
	if r.reload(t, p.ID).Status != "published" {
		t.Error("the page was not published")
	}
	r.must(t, r.content, "bulk_pages", map[string]any{"website": r.site, "action": "trash", "ids": []int64{p.ID}})
	if !r.reload(t, p.ID).InTrash() {
		t.Error("bulk trash did not trash")
	}
}

// The boundaries: a key for another website reaches nothing here, and a
// read-only key neither sees nor calls the writing tools.
func TestPageToolsKeepTheirBoundaries(t *testing.T) {
	r := newPagesRig(t)
	p := r.newPage(t, r.site, "Meins", "Text")

	for name, args := range map[string]map[string]any{
		"trash_page":        {"id": p.ID},
		"get_page_settings": {"id": p.ID},
		"get_page_blocks":   {"id": p.ID},
		"set_page_blocks":   {"id": p.ID, "blocks": []any{map[string]any{"type": "trenner"}}},
		"duplicate_page":    {"id": p.ID},
		"list_trash":        {"website": r.site},
		"bulk_pages":        {"website": r.site, "action": "trash", "ids": []int64{p.ID}},
	} {
		if _, failed := r.call(t, r.foreign, name, args); !failed {
			t.Errorf("%s: a key for another website got through", name)
		}
	}
	if r.reload(t, p.ID).InTrash() {
		t.Fatal("a foreign key trashed the page")
	}

	listed := r.rpc(t, r.readOnly, "tools/list", nil)["result"].(map[string]any)["tools"].([]any)
	for _, raw := range listed {
		switch name := raw.(map[string]any)["name"].(string); name {
		case "set_page_blocks", "trash_page", "purge_page", "create_share_link", "bulk_pages":
			t.Errorf("a read-only key is offered %s", name)
		}
	}
	if _, failed := r.call(t, r.readOnly, "set_page_blocks", map[string]any{
		"id": p.ID, "blocks": []any{map[string]any{"type": "trenner"}},
	}); !failed {
		t.Error("a read-only key wrote blocks")
	}
	if _, failed := r.call(t, r.readOnly, "get_page_settings", map[string]any{"id": p.ID}); failed {
		t.Error("a read-only key could not read the settings")
	}
}
