package main

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The gate has to catch what it was built for after the fact.
//
// PUB-01 is the case: v2.0 put English refusals into two plugins, in front of
// German visitors, and the gate ran green over them for two days. It asked "is
// this German?", which an English sentence passes. The question it was missing
// is the one below.
func TestASentenceAPluginCannotTranslateIsReported(t *testing.T) {
	cases := []struct {
		name, src string
		want      bool
	}{
		{
			name: "a bare refusal",
			src:  `func f() string { return "The e-mail address does not look right." }`,
			want: true,
		},
		{
			name: "the same refusal through T",
			src:  `func f() string { return plugin.T("The e-mail address does not look right.") }`,
			want: false,
		},
		{
			name: "through Tf, with the label as an argument",
			src:  `func f(l string) string { return plugin.Tf("“%s” has to be a number.", l) }`,
			want: false,
		},
		{
			name: "a sentence built with + across lines, inside T",
			src: `func f() string { return plugin.T("No field for an e-mail address: " +
				"an enquiry through this form cannot be answered by mail.") }`,
			want: false,
		},
		{
			// The trap the glossary records for Sprintf: only the format string
			// travels. A whole sentence handed to a %s is never translated.
			name: "a sentence handed to the %s of a Tf",
			src:  `func f() string { return plugin.Tf("%s", "The form could not be read.") }`,
			want: true,
		},
		{
			name: "a log line, which is English the way the source is",
			src:  `func f() { plugin.Logf("info", "%d old messages removed.", 3) }`,
			want: false,
		},
		{
			name: "a label, which this gate deliberately lets through",
			src:  `func f() string { return "Subject" }`,
			want: false,
		},
		{
			name: "a field name, which is not a sentence",
			src:  `const fieldEmail = "email"`,
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The path decides the rule: the same literal in internal/ is the
			// collector's business, not this gate's.
			path := filepath.Join("plugins", "demo", "main.go")
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, "package main\n"+c.src, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := checkPlugins(path, file, fset, map[int]bool{})
			if (len(got) > 0) != c.want {
				t.Fatalf("reported %d findings, want reported=%v\n%s", len(got), c.want, c.src)
			}
			for _, f := range got {
				if !strings.Contains(f.why, "T/Tf") {
					t.Errorf("the reason does not say what to do: %q", f.why)
				}
			}
		})
	}
}

// Outside plugins/ and sdk/ this half of the gate says nothing, because
// tools/i18n already reports an untranslated sentence there as open.
func TestTheSentenceRuleAppliesOnlyWhereTheCollectorCannotSee(t *testing.T) {
	src := `package main
func f() string { return "The e-mail address does not look right." }`
	for _, path := range []string{
		filepath.Join("internal", "admin", "page.go"),
		filepath.Join("cmd", "holzcloud", "main.go"),
		filepath.Join("plugins", "demo", "main_test.go"),
	} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := checkPlugins(path, file, fset, map[int]bool{}); len(got) != 0 {
			t.Errorf("%s: %d findings, want none", path, len(got))
		}
	}
}
