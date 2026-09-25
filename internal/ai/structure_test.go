package ai

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The happy paths of these tools run against the real admin handler in
// internal/admin/ops_structure_test.go — this package cannot import that one.
// What is held here is what does not need the handler at all: which key may
// call what, and that a refusal comes before anything is looked up.

// The web admin's split, mirrored: content kinds, fields and block kinds sit
// behind requireAdmin there (cmd/holzcloud/main.go), so they are Admin here;
// menus, terms, snippets and redirects are an editor's, so they are not.
func TestStructureToolsFollowTheScreensPermissionSplit(t *testing.T) {
	admin := map[string]bool{
		"list_kinds": true, "create_kind": true, "update_kind": true, "delete_kind": true, "move_kind": true,
		"create_field": true, "update_field": true, "delete_field": true, "move_field": true,
		"list_block_kinds": true, "create_block_kind": true, "update_block_kind": true,
		"delete_block_kind": true, "move_block_kind": true,
	}
	reads := map[string]bool{
		"list_menus": true, "get_menu": true, "list_terms": true, "list_snippets": true,
		"get_snippet": true, "list_redirects": true, "check_links": true,
		"list_kinds": true, "list_block_kinds": true,
	}
	seen := 0
	for _, tool := range structureTools(Deps{}) {
		seen++
		if tool.Admin != admin[tool.Name] {
			t.Errorf("%s: Admin = %v, want %v", tool.Name, tool.Admin, admin[tool.Name])
		}
		if tool.Writes == reads[tool.Name] {
			t.Errorf("%s: Writes = %v, but it is a %s tool", tool.Name, tool.Writes,
				map[bool]string{true: "reading", false: "writing"}[reads[tool.Name]])
		}
		// Every delete demands the confirmation, in the schema where an
		// assistant reads it and not only in the code.
		if strings.HasPrefix(tool.Name, "delete_") {
			found := false
			for _, r := range tool.InputSchema.Required {
				found = found || r == "confirm"
			}
			if !found {
				t.Errorf("%s does not require confirm", tool.Name)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no structure tools")
	}
}

func TestAReadOnlyKeyCannotChangeTheStructure(t *testing.T) {
	ts, _, wsID, _, lesen := setUp(t)
	for name, args := range map[string]map[string]any{
		"create_menu":     {"website": wsID, "name": "Haupt", "key": "main"},
		"create_snippet":  {"website": wsID, "key": "k", "name": "K"},
		"create_redirect": {"website": wsID, "from": "/a", "to": "/b"},
		"rename_term":     {"website": wsID, "term": 1, "name": "x"},
	} {
		answer, failed := callTool(t, ts, lesen, name, args)
		if !failed {
			t.Errorf("%s: a read-only key was let through", name)
			continue
		}
		if answer["text"] != ErrReadOnly.Error() {
			t.Errorf("%s: refused for the wrong reason: %v", name, answer["text"])
		}
	}
}

// A content key writes menus but not the structure an admin decides.
func TestAContentKeyCannotChangeKindsFieldsOrBlockKinds(t *testing.T) {
	ts, _, wsID, schreiben, _ := setUp(t)
	for name, args := range map[string]map[string]any{
		"list_kinds":        {"website": wsID},
		"create_kind":       {"website": wsID, "name": "Produkt", "plural": "Produkte"},
		"create_field":      {"website": wsID, "label": "Preis", "kind": "zahl"},
		"delete_field":      {"website": wsID, "field": 1, "confirm": true},
		"create_block_kind": {"website": wsID, "name": "Schritt"},
	} {
		answer, failed := callTool(t, ts, schreiben, name, args)
		if !failed {
			t.Errorf("%s: a content key was let through", name)
			continue
		}
		if answer["text"] != ErrNotAdmin.Error() {
			t.Errorf("%s: refused for the wrong reason: %v", name, answer["text"])
		}
	}

	// And it does not even see them offered.
	adminOnly := map[string]bool{}
	for _, tool := range structureTools(Deps{}) {
		adminOnly[tool.Name] = tool.Admin
	}
	res := call(t, ts, schreiben, "tools/list", nil)
	for _, raw := range res["result"].(map[string]any)["tools"].([]any) {
		if name := raw.(map[string]any)["name"].(string); adminOnly[name] {
			t.Errorf("a content key is offered %s", name)
		}
	}
}

// A key for one website is refused on another before anything is looked up —
// the refusal is the scope's, not a "not found" that would say the id exists.
func TestAStructureToolStaysWithItsOwnWebsite(t *testing.T) {
	database := newTestDB(t)
	domains := domain.NewStore(database)
	eins, _ := domains.CreateWebsite(context.Background(), "Eins", "")
	zwei, _ := domains.CreateWebsite(context.Background(), "Zwei", "")
	tokens := NewStore(database)
	secret, _, err := tokens.IssueLevel(context.Background(), "nur eins", eins.ID, LevelContent, 0)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains, Pages: page.NewStore(database), Media: media.NewStore(database),
	})))
	defer ts.Close()

	for name, args := range map[string]map[string]any{
		"list_menus":      {"website": zwei.ID},
		"get_menu":        {"website": zwei.ID, "menu": 1},
		"create_menu":     {"website": zwei.ID, "name": "Fremd", "key": "main"},
		"delete_menu":     {"website": zwei.ID, "menu": 1, "confirm": true},
		"list_terms":      {"website": zwei.ID},
		"delete_term":     {"website": zwei.ID, "term": 1, "confirm": true},
		"get_snippet":     {"website": zwei.ID, "snippet": 1},
		"update_snippet":  {"website": zwei.ID, "snippet": 1, "name": "x"},
		"list_redirects":  {"website": zwei.ID},
		"create_redirect": {"website": zwei.ID, "from": "/a", "to": "/b"},
		"check_links":     {"website": zwei.ID},
	} {
		answer, failed := callTool(t, ts, secret, name, args)
		if !failed {
			t.Errorf("%s reached another website", name)
			continue
		}
		if answer["text"] != ErrNotForYou.Error() {
			t.Errorf("%s: refused for the wrong reason: %v", name, answer["text"])
		}
	}
}

// Without confirm nothing is deleted, and the answer says what to do.
func TestADeleteWithoutConfirmIsRefused(t *testing.T) {
	ts, _, wsID, schreiben, _ := setUp(t)
	answer, failed := callTool(t, ts, schreiben, "delete_menu", map[string]any{"website": wsID, "menu": 1})
	if !failed {
		t.Fatal("a menu was deleted without confirm")
	}
	if !strings.Contains(answer["text"].(string), "confirm: true") {
		t.Errorf("the refusal does not say how to confirm: %v", answer["text"])
	}
}

// A build whose Ops lacks the methods answers with a sentence, never a panic.
func TestAMissingHandlerIsSaidAndNotAPanic(t *testing.T) {
	ts, _, wsID, schreiben, _ := setUp(t)
	answer, failed := callTool(t, ts, schreiben, "list_menus", map[string]any{"website": wsID})
	if !failed || !strings.Contains(answer["text"].(string), "not available") {
		t.Errorf("list_menus without a handler = %v", answer)
	}
}

// The item list of a menu turns into the three kinds the screen knows.
func TestMenuEntriesAreReadIntoTheirKinds(t *testing.T) {
	items, err := menuInput([]menuItemWire{
		{Label: "Start", Page: 4},
		{Label: "Extern", URL: "https://example.org", Children: []menuItemWire{{Label: "Nur Text"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ItemType != "page" || items[0].PageID == nil || *items[0].PageID != 4 {
		t.Errorf("page entry = %+v", items[0])
	}
	if items[1].ItemType != "url" || items[1].URL != "https://example.org" {
		t.Errorf("url entry = %+v", items[1])
	}
	if got := items[1].Children[0].ItemType; got != "custom" {
		t.Errorf("an entry without a target is %q, want custom", got)
	}
	if _, err := menuInput([]menuItemWire{{Label: "Beides", Page: 1, URL: "/x"}}); err == nil {
		t.Error("an entry naming a page and an address was accepted")
	}
}
