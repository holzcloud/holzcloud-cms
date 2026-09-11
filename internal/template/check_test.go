package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

const goodLayout = `<html lang="{{.Site.Locale}}"><head><title>{{.Page.Title}}</title></head>` +
	`<body>{{template "content" .}}</body></html>`

func themeFS(files map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

// The point of the check is what it says when a template is wrong. Each case
// here is a mistake someone writing a template actually makes; the assertion is
// that the message names the file, and where possible the way out.
func TestCheckCatchesAuthoringMistakes(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		// want are fragments that must all appear in the reported problems.
		want []string
	}{{
		name: "misspelt field",
		files: map[string]string{
			"layout.html": goodLayout,
			"page.html":   `{{define "content"}}<h1>{{.Page.Titel}}</h1>{{end}}`,
		},
		// The list of real field names is what turns this from a report into
		// a fix.
		want: []string{"page.html", "Titel", "PageContent has:", "Title"},
	}, {
		name: "unguarded optional field",
		files: map[string]string{
			"layout.html": goodLayout,
			"page.html":   `{{define "content"}}<a href="{{.Page.Next.URL}}">weiter</a>{{end}}`,
		},
		want: []string{"page.html", "optional field empty", "{{with .Page.Next}}"},
	}, {
		name: "unknown helper",
		files: map[string]string{
			"layout.html": goodLayout,
			"page.html":   `{{define "content"}}{{prettyDate .Page.PublishedAt}}{{end}}`,
		},
		want: []string{"page.html", "not defined", "formatDate"},
	}, {
		name: "syntax error",
		files: map[string]string{
			"layout.html": goodLayout,
			"page.html":   `{{define "content"}}{{if .Page.IsPost}}Beitrag{{end}}`,
		},
		want: []string{"page.html"},
	}, {
		name: "view forgets the content block",
		files: map[string]string{
			"layout.html": goodLayout,
			"page.html":   `<h1>{{.Page.Title}}</h1>`,
		},
		want: []string{"page.html", `"content"`, `{{define "content"}}`},
	}, {
		// Renders without any error and produces a site with no content on it.
		name: "layout never includes the body",
		files: map[string]string{
			"layout.html": `<html><body><h1>{{.Site.Name}}</h1></body></html>`,
			"page.html":   `{{define "content"}}<p>{{.Page.Title}}</p>{{end}}`,
		},
		want: []string{"layout.html", "never includes the page body", `{{template "content" .}}`},
	}, {
		name: "layout is only a fragment",
		files: map[string]string{
			"layout.html": `<div>{{template "content" .}}</div>`,
			"page.html":   `{{define "content"}}<p>{{.Page.Title}}</p>{{end}}`,
		},
		want: []string{"layout.html", "whole HTML document"},
	}, {
		name: "required file missing",
		files: map[string]string{
			"layout.html": goodLayout,
		},
		want: []string{"page.html", "missing"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := Check(themeFS(tc.files), nil)
			if len(problems) == 0 {
				t.Fatalf("no problem reported for a template with a %s", tc.name)
			}

			var report strings.Builder
			for _, p := range problems {
				report.WriteString(p.String() + "\n")
			}
			for _, want := range tc.want {
				if !strings.Contains(report.String(), want) {
					t.Errorf("the message does not mention %q:\n%s", want, report.String())
				}
			}
		})
	}
}

// A mistake in the layout breaks every view, including the ones the archive did
// not bring — those come from the default theme and are still wrapped in this
// layout. An archive with only layout.html and page.html is explicitly allowed,
// so this combination has to be checked or it is checked by the first visitor.
func TestCheckRendersFallbackViewsThroughTheOwnLayout(t *testing.T) {
	fallback := os.DirFS(filepath.Join("..", "..", "cmd", "holzcloud", "templates", "public", "default"))

	theme := themeFS(map[string]string{
		"layout.html": `<html><body>{{.Site.Naem}}{{template "content" .}}</body></html>`,
		"page.html":   `{{define "content"}}<p>{{.Page.Title}}</p>{{end}}`,
	})

	problems := Check(theme, fallback)
	if len(problems) == 0 {
		t.Fatal("a layout that names a field which does not exist was accepted")
	}

	var report strings.Builder
	blamesLayout := false
	for _, p := range problems {
		report.WriteString(p.String() + "\n")
		// The fault is in the layout. A report that only named home.html would
		// send the author to a file they never wrote.
		if p.File == "layout.html" {
			blamesLayout = true
		}
		if p.File != "layout.html" && !strings.Contains(p.Hint, "fault is in layout.html") {
			t.Errorf("problem in %s does not say the cause is the layout: %s", p.File, p)
		}
	}
	if !blamesLayout {
		t.Errorf("no problem points at the layout as the cause:\n%s", report.String())
	}
	if !strings.Contains(report.String(), "SiteData has:") {
		t.Errorf("no problem lists the fields SiteData does have:\n%s", report.String())
	}

	// One fault, reported once — not once per view that the layout breaks.
	if len(problems) != 1 {
		t.Errorf("one typo in the layout produced %d problems:\n%s", len(problems), report.String())
	}
}

// A template that is fine must produce no problems at all — a check that cries
// wolf on good input is one whose output nobody reads.
func TestCheckAcceptsAGoodTemplate(t *testing.T) {
	theme := themeFS(map[string]string{
		"layout.html": goodLayout,
		"page.html": `{{define "content"}}
			<article>
			  {{if not .Page.HasOwnHeading}}<h1>{{.Page.Title}}</h1>{{end}}
			  {{.Page.ContentHTML}}
			  {{with .Page.PublishedAt}}<time>{{formatDate .}}</time>{{end}}
			  {{with .Page.Next}}<a href="{{.URL}}">{{.Title}}</a>{{end}}
			  {{range .Page.Terms}}<a href="{{.URL}}">{{.Name}}</a>{{end}}
			</article>{{end}}`,
	})

	if problems := Check(theme, nil); len(problems) > 0 {
		for _, p := range problems {
			t.Errorf("good template rejected: %s", p)
		}
	}
}

// Every shipped theme has to pass the check an uploaded one has to pass. If it
// did not, the specification would be describing rules the project's own work
// does not follow.
func TestShippedThemesPassTheCheck(t *testing.T) {
	root := filepath.Join("..", "..", "cmd", "holzcloud", "templates", "public")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read shipped themes: %v", err)
	}

	var themes int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		themes++
		for _, p := range CheckDir(filepath.Join(root, e.Name()), nil) {
			t.Errorf("shipped theme %q would be rejected on upload: %s", e.Name(), p)
		}
	}
	if themes == 0 {
		t.Fatalf("no shipped themes found under %s", root)
	}
}

// The 2.0 break, held from the outside. CHANGELOG.md promises that a theme
// written against 1.x stops working — deliberately, in one release, rather than
// both spellings living side by side for years. A promise that nothing asserts
// is a sentence, not a behaviour: without this test the old names could quietly
// come back as aliases and the changelog would be lying.
//
// It is written against all seven renamed names rather than one, because the
// cheap mistake is to convert six and leave the seventh working by accident.
func TestATemplateWrittenAgainstTheOldGermanContractIsRefused(t *testing.T) {
	for _, old := range []string{
		".Page.Felder.preis",
		".Page.Feldliste",
		".Page.Art",
		".Page.Uebersetzungen",
		".Site.Bausteinfelder",
		".Site.Bausteinliste",
		".Site.Sprachen",
	} {
		theme := themeFS(map[string]string{
			"layout.html": goodLayout,
			"page.html":   `{{define "content"}}<article>{{` + old + `}}</article>{{end}}`,
		})

		problems := Check(theme, nil)
		if len(problems) == 0 {
			t.Errorf("a theme using the 1.x name %s was accepted; the break "+
				"CHANGELOG.md announces under 2.0 did not happen for this name", old)
		}
	}
}

// The other half, and the one a theme author actually meets: the refusal has to
// name the English field, or converting a theme means guessing. Go's own error
// lists the struct's fields, so this costs nothing to keep true — but only as
// long as nobody wraps it in a friendlier message that drops the list.
func TestTheRefusalNamesTheEnglishFieldsSoAThemeCanBeConverted(t *testing.T) {
	theme := themeFS(map[string]string{
		"layout.html": goodLayout,
		"page.html":   `{{define "content"}}<article>{{.Page.Feldliste}}</article>{{end}}`,
	})

	problems := Check(theme, nil)
	if len(problems) == 0 {
		t.Fatalf("the old name was accepted")
	}
	var joined string
	for _, p := range problems {
		joined += p.String() + "\n"
	}
	for _, want := range []string{"Fields", "FieldList", "Translations"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the refusal does not name %q, so a theme author is left "+
				"guessing what to write instead:\n%s", want, joined)
		}
	}
}
