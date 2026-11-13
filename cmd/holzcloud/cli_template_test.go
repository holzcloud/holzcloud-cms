package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/tmplspec"
)

// minimalTheme writes the complete minimal template of TEMPLATE-SPEC §11 into
// a temporary directory, plus whatever extra files a test names. The
// specification's own example, so the test exercises what an author starts
// from rather than a shape invented here.
func minimalTheme(t *testing.T, extra map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"layout.html": specBlock(t, "Zum Inhalt springen"),
		"page.html":   specBlock(t, "Weitere Beiträge"),
	}
	for name, body := range extra {
		files[name] = body
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// specBlock returns the fenced code block of the specification holding marker.
func specBlock(t *testing.T, marker string) string {
	t.Helper()
	for i, block := range strings.Split(tmplspec.Markdown(), "```") {
		if i%2 == 0 || !strings.Contains(block, marker) {
			continue
		}
		if _, rest, ok := strings.Cut(block, "\n"); ok {
			return rest
		}
	}
	t.Fatalf("no code block in the specification contains %q", marker)
	return ""
}

// The check says which contract it read the theme against, and what the theme
// itself declares. A theme without theme.json is fine and is told so.
func TestTemplateCheckNamesTheContract(t *testing.T) {
	dir := minimalTheme(t, nil)

	var out, errOut bytes.Buffer
	if err := runTemplateCheck([]string{dir}, &out, &errOut); err != nil {
		t.Fatalf("the minimal theme failed the check: %v\nstderr:\n%s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "template contract 1.0; ") {
		t.Errorf("stdout does not name the contract:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "declares no contract version") {
		t.Errorf("stdout does not say the theme declares no version:\n%s", out.String())
	}
}
