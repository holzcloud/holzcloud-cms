package menu

import (
	"strings"
	"testing"
)

func node(itemType, title, url, pageSlug string) MenuNode {
	return MenuNode{MenuItem: MenuItem{
		Title:    title,
		ItemType: itemType,
		URL:      url,
		PageSlug: pageSlug,
	}}
}

func renderOne(n MenuNode) string {
	return string(RenderMenu(map[string][]MenuNode{"main": {n}}, "main"))
}

func TestRenderMenuEscapesTitlesAndSlugs(t *testing.T) {
	got := renderOne(node("page", `<script>alert(1)</script>`, "", `x" onmouseover="alert(1)`))

	if strings.Contains(got, "<script>") {
		t.Errorf("title was not escaped: %s", got)
	}
	// The slug must stay inside the href value: an unescaped quote followed by
	// an attribute name is what a breakout looks like.
	if strings.Contains(got, `" onmouseover`) {
		t.Errorf("slug broke out of the href attribute: %s", got)
	}
}

// A menu entry must not be able to become a script execution vector.
func TestRenderMenuRefusesActiveURLSchemes(t *testing.T) {
	dangerous := []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"  javascript:alert(1)",
		"\tjavascript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==",
		"vbscript:msgbox(1)",
		"file:///etc/passwd",
	}

	for _, url := range dangerous {
		got := renderOne(node("url", "Click me", url, ""))
		if strings.Contains(got, "<a href") {
			t.Errorf("URL %q was rendered as a link: %s", url, got)
		}
		if !strings.Contains(got, "Click me") {
			t.Errorf("URL %q lost its title entirely: %s", url, got)
		}
	}
}

func TestRenderMenuKeepsLegitimateURLs(t *testing.T) {
	safe := []string{
		"https://example.com/page",
		"http://example.com",
		"mailto:hello@example.com",
		"tel:+4915112345678",
		"/about",
		"about",
		"#section",
		"?page=2",
		"//cdn.example.com/asset",
	}

	for _, url := range safe {
		got := renderOne(node("url", "Link", url, ""))
		if !strings.Contains(got, "<a href=") {
			t.Errorf("URL %q was not rendered as a link: %s", url, got)
		}
	}
}

func TestRenderMenuStopsAtMaxDepth(t *testing.T) {
	// Four nested levels; only three may be rendered.
	deepest := node("custom", "level4", "", "")
	l3 := node("custom", "level3", "", "")
	l3.Children = []MenuNode{deepest}
	l2 := node("custom", "level2", "", "")
	l2.Children = []MenuNode{l3}
	l1 := node("custom", "level1", "", "")
	l1.Children = []MenuNode{l2}

	got := renderOne(l1)

	for _, want := range []string{"level1", "level2", "level3"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %s to be rendered: %s", want, got)
		}
	}
	if strings.Contains(got, "level4") {
		t.Errorf("depth limit not enforced: %s", got)
	}
}

func TestRenderMenuUnknownLocationIsEmpty(t *testing.T) {
	if got := RenderMenu(map[string][]MenuNode{"main": {node("custom", "x", "", "")}}, "footer"); got != "" {
		t.Errorf("expected empty output for unknown location, got %q", got)
	}
}

// The query orders by parent_id ascending, so a grandchild is linked after its
// parent has already been visited. Building the tree with value copies dropped
// every third-level item silently, even though README and RenderMenu both
// promise three levels.
func TestBuildTreeKeepsThirdLevel(t *testing.T) {
	id := func(n int64) *int64 { return &n }
	items := []MenuItem{
		{ID: 1, Title: "level1"},
		{ID: 2, Title: "level2", ParentID: id(1)},
		{ID: 3, Title: "level3", ParentID: id(2)},
	}

	roots := buildTree(items)
	if len(roots) != 1 {
		t.Fatalf("want 1 root, got %d", len(roots))
	}
	if len(roots[0].Children) != 1 {
		t.Fatalf("level 2 missing: %+v", roots[0])
	}
	if len(roots[0].Children[0].Children) != 1 {
		t.Fatalf("level 3 was dropped: %+v", roots[0].Children[0])
	}
	if got := roots[0].Children[0].Children[0].Title; got != "level3" {
		t.Errorf("level 3 title = %q; want level3", got)
	}
}

// An item whose parent no longer exists must still appear rather than vanish.
func TestBuildTreeKeepsOrphans(t *testing.T) {
	missing := int64(999)
	roots := buildTree([]MenuItem{{ID: 1, Title: "waise", ParentID: &missing}})
	if len(roots) != 1 || roots[0].Title != "waise" {
		t.Errorf("orphan lost: %+v", roots)
	}
}

// A parent cycle must not make the renderer recurse forever.
func TestBuildTreeSurvivesCycles(t *testing.T) {
	a, b := int64(1), int64(2)
	roots := buildTree([]MenuItem{
		{ID: 1, Title: "a", ParentID: &b},
		{ID: 2, Title: "b", ParentID: &a},
	})
	if len(roots) == 0 {
		t.Error("a cycle swallowed every item")
	}
}
