package tmplmgr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTheme puts entries into a fresh directory, the shape CheckTemplateDir
// reads after an archive has been extracted.
func writeTheme(t *testing.T, entries []zipEntry) string {
	t.Helper()
	dir := t.TempDir()
	for _, e := range entries {
		if err := os.WriteFile(filepath.Join(dir, e.name), e.body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// An author who wrote theme.json said something; when it cannot be read, the
// upload says so, naming the file, instead of installing a theme whose
// declaration nobody will ever see.
func TestCheckTemplateDirRefusesAMalformedThemeJSON(t *testing.T) {
	for _, manifest := range []string{`{"contract": "one"}`, `not json`} {
		t.Run(manifest, func(t *testing.T) {
			files := validTemplate(zipEntry{name: "theme.json", body: []byte(manifest)})

			problems := CheckTemplateDir(writeTheme(t, files), nil)
			if len(problems) != 1 || !strings.Contains(problems[0], "theme.json") {
				t.Fatalf("CheckTemplateDir = %q, want one problem naming theme.json", problems)
			}

			r := buildZip(t, files...)
			err := ExtractTemplate(r, int64(r.Len()), filepath.Join(t.TempDir(), "theme"), 1<<20, nil)
			if err == nil || !strings.Contains(err.Error(), "theme.json") {
				t.Errorf("ExtractTemplate = %v, want a refusal naming theme.json", err)
			}
		})
	}
}

// Declaring nothing is allowed, and so is any well-formed version — an older
// one, a newer one, one beside keys nobody reads. The check reports a version;
// it does not decide what a difference means.
func TestCheckTemplateDirAcceptsAnyWellFormedContract(t *testing.T) {
	cases := map[string][]zipEntry{
		"absent":         validTemplate(),
		"current":        validTemplate(zipEntry{name: "theme.json", body: []byte(`{"contract": "1.0"}`)}),
		"older":          validTemplate(zipEntry{name: "theme.json", body: []byte(`{"contract": "0.9"}`)}),
		"newer and more": validTemplate(zipEntry{name: "theme.json", body: []byte(`{"contract": "2.3", "author": "x"}`)}),
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			if problems := CheckTemplateDir(writeTheme(t, files), nil); len(problems) > 0 {
				t.Errorf("a well-formed theme was refused: %q", problems)
			}
		})
	}
}
