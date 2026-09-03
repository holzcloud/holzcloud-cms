package page

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World":             "hello-world",
		"  Trimmed  ":             "trimmed",
		"Multiple   Spaces":       "multiple-spaces",
		"Ümläute & Sönderzeichen": "uemlaeute-soenderzeichen",
		"Möbelbau":                "moebelbau",
		"Grüße aus Köln":          "gruesse-aus-koeln",
		"Maß & Straße":            "mass-strasse",
		"Café Crème":              "cafe-creme",
		"---leading-trailing---":  "leading-trailing",
		"Already-A-Slug":          "already-a-slug",
		"123":                     "123",
		"!!!":                     "untitled",
		"":                        "untitled",
		"../../etc/passwd":        "etc-passwd",
	}

	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q; want %q", in, got, want)
		}
	}
}

// A slug ends up in URLs and is compared against path segments, so it must never
// contain path separators or spaces.
func TestSlugifyProducesURLSafeOutput(t *testing.T) {
	inputs := []string{
		"a/b",
		`a\b`,
		"a b",
		"a?b=c#d",
		"<script>alert(1)</script>",
		"..",
	}
	for _, in := range inputs {
		got := Slugify(in)
		if strings.ContainsAny(got, `/\ ?#<>"'%`) {
			t.Errorf("Slugify(%q) = %q contains an unsafe character", in, got)
		}
		if got == "" {
			t.Errorf("Slugify(%q) returned an empty slug", in)
		}
	}
}

// goldmark output must never reach a template without passing bluemonday.
func TestRenderMarkdownSanitizesOutput(t *testing.T) {
	cases := map[string][]string{
		"<script>alert(1)</script>":                  {"<script>", "alert(1)"},
		"<img src=x onerror=alert(1)>":               {"onerror"},
		"[click](javascript:alert(1))":               {"javascript:"},
		"<iframe src=\"https://evil.com\"></iframe>": {"<iframe"},
		"<a href=\"/ok\" onclick=\"steal()\">x</a>":  {"onclick"},
	}

	for source, forbidden := range cases {
		html, err := RenderMarkdown(source)
		if err != nil {
			t.Fatalf("RenderMarkdown(%q): %v", source, err)
		}
		for _, f := range forbidden {
			if strings.Contains(html, f) {
				t.Errorf("RenderMarkdown(%q) = %q still contains %q", source, html, f)
			}
		}
	}
}

func TestRenderMarkdownKeepsSafeFormatting(t *testing.T) {
	html, err := RenderMarkdown("# Title\n\nSome **bold** text and a [link](https://example.com).")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	for _, want := range []string{"<h1", "<strong>bold</strong>", `href="https://example.com"`} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in output:\n%s", want, html)
		}
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(errString("UNIQUE constraint failed: pages.slug")) {
		t.Error("expected a SQLite UNIQUE error to be recognised")
	}
	if !isUniqueViolation(errString("unique constraint violation")) {
		t.Error("expected a lowercase UNIQUE error to be recognised")
	}
	if isUniqueViolation(errString("database is locked")) {
		t.Error("an unrelated error was treated as a UNIQUE violation")
	}
	if isUniqueViolation(nil) {
		t.Error("nil must not be a UNIQUE violation")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// The create and edit handlers only run Slugify on an empty field, so a
// hand-typed slug used to reach the database verbatim. A reserved name or a
// slash produced a page that could never be opened, with no error anywhere.
func TestValidateSlugRejectsUnreachableAddresses(t *testing.T) {
	rejected := map[string]string{
		"empty":            "",
		"reserved admin":   "admin",
		"reserved sitemap": "sitemap.xml",
		"reserved media":   "media",
		"reserved assets":  "assets",
		"reserved t":       "t",
		"uppercase":        "Impressum",
		"slash":            "ueber/uns",
		"space":            "ueber uns",
		"leading hyphen":   "-start",
		"trailing hyphen":  "start-",
		"double hyphen":    "a--b",
		"umlaut":           "über-uns",
		"query":            "seite?x=1",
	}
	for label, slug := range rejected {
		if err := ValidateSlug(slug); err == nil {
			t.Errorf("%s: ValidateSlug(%q) accepted it", label, slug)
		}
	}

	accepted := []string{"home", "ueber-uns", "impressum", "seite-2", "a", "2026-rueckblick"}
	for _, slug := range accepted {
		if err := ValidateSlug(slug); err != nil {
			t.Errorf("ValidateSlug(%q) rejected a valid slug: %v", slug, err)
		}
	}
}

// Whatever Slugify produces must pass validation, or the automatic path would
// generate slugs the manual path rejects.
func TestSlugifyOutputAlwaysValidates(t *testing.T) {
	for _, title := range []string{
		"Über uns", "Grüße aus Köln", "!!!", "", "Maß & Straße",
		"---", "A very long title " + strings.Repeat("x ", 200),
	} {
		slug := Slugify(title)
		if err := ValidateSlug(slug); err != nil {
			t.Errorf("Slugify(%q) = %q which fails validation: %v", title, slug, err)
		}
	}
}

// goldmark now passes raw HTML through, which makes bluemonday the actual
// security boundary rather than a second line of defence. These cases are the
// contract that boundary has to hold.
func TestRenderMarkdownKeepsInlineHTMLButStripsDanger(t *testing.T) {
	kept := map[string]string{
		"HTML paragraph":    "<p>Importierter Text</p>",
		"inline formatting": "Text mit <strong>fett</strong> mittendrin",
		"a div":             `<div class="kasten">Kasten</div>`,
		"a table":           "<table><tr><td>Zelle</td></tr></table>",
		"a safe link":       `<a href="/impressum">Impressum</a>`,
		"an image":          `<img src="/media/1/foto.png" alt="Foto">`,
	}
	for label, src := range kept {
		out, err := RenderMarkdown(src)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if strings.TrimSpace(out) == "" {
			t.Errorf("%s: content was silently dropped (input %q)", label, src)
		}
	}

	dangerous := map[string][]string{
		"<script>alert(1)</script>":                          {"<script", "alert(1)"},
		`<img src=x onerror="alert(1)">`:                     {"onerror"},
		`<a href="javascript:alert(1)">x</a>`:                {"javascript:"},
		`<iframe src="https://evil.example"></iframe>`:       {"<iframe"},
		`<object data="evil.swf"></object>`:                  {"<object"},
		`<embed src="evil.swf">`:                             {"<embed"},
		`<form action="https://evil.example"><input></form>`: {"<form"},
		`<div onclick="steal()">x</div>`:                     {"onclick"},
		`<style>body{display:none}</style>`:                  {"<style"},
		`<a href="/ok" onmouseover="x()">y</a>`:              {"onmouseover"},
		`<svg><script>alert(1)</script></svg>`:               {"<script"},
		`<meta http-equiv="refresh" content="0;url=//evil">`: {"http-equiv"},
	}
	for src, forbidden := range dangerous {
		out, err := RenderMarkdown(src)
		if err != nil {
			t.Fatalf("RenderMarkdown(%q): %v", src, err)
		}
		for _, f := range forbidden {
			if strings.Contains(strings.ToLower(out), strings.ToLower(f)) {
				t.Errorf("RenderMarkdown(%q) = %q still contains %q", src, out, f)
			}
		}
	}
}

// Responsive images are the reason srcset is allowed through: without it every
// visitor downloads the full-size photo. The attribute is layout, not script.
func TestRenderMarkdownKeepsResponsiveImageAttributes(t *testing.T) {
	in := `<picture>
<source srcset="/media/1/foto-800.jpg 800w, /media/1/foto-1600.jpg 1600w" sizes="(max-width: 40rem) 100vw, 40rem" type="image/jpeg">
<img src="/media/1/foto.jpg" alt="Werkstatt" loading="lazy" decoding="async">
</picture>`
	out, err := RenderMarkdown(in)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	for _, want := range []string{"<picture", "<source", "srcset=", "sizes=", `loading="lazy"`, `decoding="async"`, `alt="Werkstatt"`} {
		if !strings.Contains(out, want) {
			t.Errorf("%s was stripped:\n%s", want, out)
		}
	}
}

// Opening srcset must not open a script vector with it.
func TestRenderMarkdownRejectsSchemeInSrcset(t *testing.T) {
	for _, in := range []string{
		`<img src="/media/1/a.jpg" srcset="javascript:alert(1) 1x">`,
		`<img src="/media/1/a.jpg" srcset="data:text/html;base64,PHNjcmlwdD4= 1x">`,
		`<source srcset="https://fremd.example/bild.jpg 1x">`,
	} {
		out, err := RenderMarkdown(in)
		if err != nil {
			t.Fatalf("RenderMarkdown: %v", err)
		}
		if strings.Contains(out, "srcset") {
			t.Errorf("a srcset with a scheme survived: %q -> %q", in, out)
		}
	}
}
