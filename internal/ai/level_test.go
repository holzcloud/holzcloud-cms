package ai

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
)

// The three levels, end to end: an admin tool is invisible and refused for a
// content key, offered and callable for an admin key.
func TestAdminToolsBelongToAdminKeys(t *testing.T) {
	database := newTestDB(t)
	tokens := NewStore(database)
	ctx := context.Background()
	content, _, err := tokens.IssueLevel(ctx, "inhalt", 0, LevelContent, 0)
	if err != nil {
		t.Fatal(err)
	}
	admin, tok, err := tokens.IssueLevel(ctx, "verwaltung", 0, LevelAdmin, 0)
	if err != nil {
		t.Fatal(err)
	}
	if tok.LevelOf() != LevelAdmin || !tok.CanWrite {
		t.Fatalf("an admin key is %v, may write %v", tok.LevelOf(), tok.CanWrite)
	}

	probe := Tool{
		Name: "probe_admin", Description: "x", InputSchema: Schema{Type: "object"},
		Writes: true, Admin: true,
		Run: func(Call) (any, error) { return map[string]any{"ok": true}, nil },
	}
	ts := httptest.NewServer(NewServer(tokens, "Test", slog.New(slog.DiscardHandler), []Tool{probe}))
	t.Cleanup(ts.Close)

	listed := func(key string) bool {
		res := call(t, ts, key, "tools/list", nil)
		for _, raw := range res["result"].(map[string]any)["tools"].([]any) {
			if raw.(map[string]any)["name"] == "probe_admin" {
				return true
			}
		}
		return false
	}
	if listed(content) {
		t.Error("a content key is offered an admin tool")
	}
	if !listed(admin) {
		t.Error("an admin key is not offered the admin tool")
	}
	if _, isErr := callTool(t, ts, content, "probe_admin", nil); !isErr {
		t.Error("a content key could call an admin tool")
	}
	if _, isErr := callTool(t, ts, admin, "probe_admin", nil); isErr {
		t.Error("an admin key could not call an admin tool")
	}
}

// An admin key limited to one website would promise what it cannot keep:
// users and plugins belong to the whole installation.
func TestAnAdminKeyReachesEveryWebsite(t *testing.T) {
	tokens := NewStore(newTestDB(t))
	if _, _, err := tokens.IssueLevel(context.Background(), "x", 7, LevelAdmin, 0); err != ErrAdminNeedsAll {
		t.Fatalf("err = %v, want ErrAdminNeedsAll", err)
	}
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]Level{"read": LevelRead, "content": LevelContent, "": LevelContent, "ADMIN": LevelAdmin} {
		got, err := ParseLevel(in)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := ParseLevel("root"); err == nil {
		t.Error("an unknown level was accepted")
	}
}

// Six areas were written side by side. Two tools of one name would leave one of
// them silently unreachable — the map keeps the last — so the names are checked.
func TestToolNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range Tools(Deps{}) {
		if seen[tool.Name] {
			t.Errorf("two tools are called %q", tool.Name)
		}
		seen[tool.Name] = true
	}
}
