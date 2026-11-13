package main

import (
	"archive/zip"
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

// zipOf packs a theme directory into an archive with its files at the root, as
// TEMPLATE-SPEC §1 asks, and returns the archive's path.
func zipOf(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "theme.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// What the theme declares in theme.json is reported beside the contract the
// check reads against. A different version is a note and never a failure; a
// theme.json that cannot be read is a problem naming the file. The same for a
// directory and for an archive of it, because an author may check either.
func TestTemplateCheckReportsTheDeclaredContract(t *testing.T) {
	forms := map[string]func(t *testing.T, dir string) string{
		"directory": func(_ *testing.T, dir string) string { return dir },
		"zip":       zipOf,
	}
	for form, target := range forms {
		t.Run(form, func(t *testing.T) {
			t.Run("current", func(t *testing.T) {
				path := target(t, minimalTheme(t, map[string]string{"theme.json": `{"contract": "1.0"}`}))
				var out, errOut bytes.Buffer
				if err := runTemplateCheck([]string{path}, &out, &errOut); err != nil {
					t.Fatalf("check failed: %v\nstderr:\n%s", err, errOut.String())
				}
				if !strings.Contains(out.String(), "template contract 1.0; "+path+" declares 1.0") {
					t.Errorf("stdout does not report the declared 1.0:\n%s", out.String())
				}
				if strings.Contains(errOut.String(), "note:") {
					t.Errorf("a theme declaring the current contract got a note:\n%s", errOut.String())
				}
			})

			t.Run("another version", func(t *testing.T) {
				path := target(t, minimalTheme(t, map[string]string{"theme.json": `{"contract": "0.9"}`}))
				var out, errOut bytes.Buffer
				if err := runTemplateCheck([]string{path}, &out, &errOut); err != nil {
					t.Fatalf("a different well-formed version failed the check: %v\nstderr:\n%s", err, errOut.String())
				}
				if !strings.Contains(out.String(), "declares 0.9") {
					t.Errorf("stdout does not report the declared 0.9:\n%s", out.String())
				}
				note := errOut.String()
				if !strings.Contains(note, "note: ") || !strings.Contains(note, "0.9") || !strings.Contains(note, "1.0") {
					t.Errorf("stderr carries no note naming 0.9 and 1.0:\n%s", note)
				}
			})

			t.Run("quiet", func(t *testing.T) {
				path := target(t, minimalTheme(t, map[string]string{"theme.json": `{"contract": "1.0"}`}))
				var out, errOut bytes.Buffer
				if err := runTemplateCheck([]string{"-quiet", path}, &out, &errOut); err != nil {
					t.Fatalf("check failed: %v\nstderr:\n%s", err, errOut.String())
				}
				if out.Len() != 0 {
					t.Errorf("-quiet printed on stdout:\n%s", out.String())
				}
			})

			t.Run("malformed", func(t *testing.T) {
				path := target(t, minimalTheme(t, map[string]string{"theme.json": `{"contract": "one"}`}))
				var out, errOut bytes.Buffer
				if err := runTemplateCheck([]string{path}, &out, &errOut); err == nil {
					t.Fatalf("a malformed theme.json passed the check\nstdout:\n%s", out.String())
				}
				if !strings.Contains(errOut.String(), "theme.json") {
					t.Errorf("stderr does not name theme.json:\n%s", errOut.String())
				}
			})
		})
	}
}
