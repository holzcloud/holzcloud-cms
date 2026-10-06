package seo

import (
	"strings"
	"testing"
)

// good is a page that earns no finding but the ones the test is about.
func good() Input {
	return Input{
		ID:              1,
		Title:           strings.Repeat("t", 30),
		Slug:            "gute-seite",
		Description:     strings.Repeat("d", 100),
		Status:          "published",
		Kind:            "page",
		Markdown:        "## Titel\n\n" + strings.Repeat("wort ", 200) + "\n\n[Kontakt](/kontakt)",
		HasFeatured:     true,
		Host:            "example.org",
		SiteDescription: "x",
	}
}

func has(fs []Finding, code string) (Finding, bool) {
	for _, f := range fs {
		if f.Code == code {
			return f, true
		}
	}
	return Finding{}, false
}

func TestAnalyzeGoodPageIsClean(t *testing.T) {
	if fs := Analyze(good()); len(fs) != 0 {
		t.Fatalf("a good page has findings: %+v", fs)
	}
}

func TestAnalyzeTitleLengthInRunes(t *testing.T) {
	cases := []struct {
		name  string
		title string
		code  string
	}{
		{"empty", "", CodeTitleMissing},
		{"blank", "   ", CodeTitleMissing},
		{"19", strings.Repeat("a", 19), CodeTitleShort},
		{"20", strings.Repeat("a", 20), ""},
		{"60", strings.Repeat("a", 60), ""},
		{"61", strings.Repeat("a", 61), CodeTitleLong},
		{"20 umlauts are 40 bytes and fine", strings.Repeat("ä", 20), ""},
		{"19 umlauts", strings.Repeat("ä", 19), CodeTitleShort},
		{"20 emoji are fine", strings.Repeat("😀", 20), ""},
		{"60 emoji are fine", strings.Repeat("😀", 60), ""},
		{"61 emoji", strings.Repeat("😀", 61), CodeTitleLong},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := good()
			in.Title = c.title
			fs := Analyze(in)
			for _, code := range []string{CodeTitleMissing, CodeTitleShort, CodeTitleLong} {
				_, got := has(fs, code)
				if got != (code == c.code) {
					t.Errorf("%s present=%v, want %v (%+v)", code, got, code == c.code, fs)
				}
			}
		})
	}
	in := good()
	in.Title = ""
	if f, _ := has(Analyze(in), CodeTitleMissing); f.Severity != SeverityError {
		t.Errorf("a missing title is an error, got %q", f.Severity)
	}
}

func TestAnalyzeDescription(t *testing.T) {
	cases := []struct {
		name, desc, site, code string
		sev                    Severity
	}{
		{"empty with site description", "", "Site", CodeDescriptionFallback, SeverityInfo},
		{"empty without", "", "", CodeDescriptionMissing, SeverityWarn},
		{"69", strings.Repeat("a", 69), "", CodeDescriptionShort, SeverityWarn},
		{"70", strings.Repeat("a", 70), "", "", ""},
		{"160", strings.Repeat("a", 160), "", "", ""},
		{"161", strings.Repeat("a", 161), "", CodeDescriptionLong, SeverityWarn},
		{"160 umlauts", strings.Repeat("ü", 160), "", "", ""},
		{"161 emoji", strings.Repeat("🙂", 161), "", CodeDescriptionLong, SeverityWarn},
		{"69 emoji", strings.Repeat("🙂", 69), "", CodeDescriptionShort, SeverityWarn},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := good()
			in.Description, in.SiteDescription = c.desc, c.site
			fs := Analyze(in)
			for _, code := range []string{CodeDescriptionMissing, CodeDescriptionFallback, CodeDescriptionShort, CodeDescriptionLong} {
				f, got := has(fs, code)
				if got != (code == c.code) {
					t.Errorf("%s present=%v, want %v (%+v)", code, got, code == c.code, fs)
				}
				if got && f.Severity != c.sev {
					t.Errorf("%s severity %q, want %q", code, f.Severity, c.sev)
				}
			}
		})
	}
}

func TestAnalyzeDuplicates(t *testing.T) {
	title := strings.Repeat("t", 30)
	desc := strings.Repeat("d", 100)
	pub := func(id int64, loc string) Other {
		return Other{ID: id, Title: "  " + strings.ToUpper(title), Description: strings.ToUpper(desc), Locale: loc, Status: "published"}
	}
	cases := []struct {
		name   string
		others []Other
		count  int
	}{
		{"none", nil, 0},
		{"case-insensitive and trimmed", []Other{pub(2, "")}, 1},
		{"two others", []Other{pub(2, ""), pub(3, "")}, 2},
		{"self is excluded", []Other{pub(1, "")}, 0},
		{"other language is no duplicate", []Other{pub(2, "fr")}, 0},
		{"draft is ignored", []Other{{ID: 2, Title: title, Description: desc, Status: "draft"}}, 0},
		{"trash is ignored", []Other{{ID: 2, Title: title, Description: desc, Status: "published", Deleted: true}}, 0},
		{"different text", []Other{{ID: 2, Title: "anders", Description: "auch anders", Status: "published"}}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := good()
			in.Others = c.others
			fs := Analyze(in)
			for _, code := range []string{CodeDuplicateTitle, CodeDuplicateDesc} {
				f, got := has(fs, code)
				if got != (c.count > 0) || f.Count != c.count {
					t.Errorf("%s: present=%v count=%d, want count %d", code, got, f.Count, c.count)
				}
			}
		})
	}
	// An empty description is not a duplicate of another empty one.
	in := good()
	in.Description = ""
	in.Others = []Other{{ID: 2, Title: "x", Status: "published"}}
	if _, got := has(Analyze(in), CodeDuplicateDesc); got {
		t.Error("two empty descriptions are not duplicates")
	}
	// A page in a language is compared within that language.
	in = good()
	in.Locale = "fr"
	in.Others = []Other{pub(2, "fr")}
	if _, got := has(Analyze(in), CodeDuplicateTitle); !got {
		t.Error("same title in the same language is a duplicate")
	}
}

func TestAnalyzeHeadings(t *testing.T) {
	long := strings.Repeat("wort ", 200)
	cases := []struct {
		name, md, blocks string
		h1, skip         bool
	}{
		{"clean", "## A\n\n### B\n\n" + long, "", false, false},
		{"h1 in markdown", "# A\n\n" + long, "", true, false},
		{"h1 tag in markdown", "<h1>A</h1>\n\n" + long, "", true, false},
		{"h1 in blocks", long, "# Kopf\n\ntext", true, false},
		{"hash without space is no heading", "#hashtag\n\n" + long, "", false, false},
		{"h1 in code fence is ignored", "```\n# kommentar\n```\n\n" + long, "", false, false},
		{"h2 then h4", "## A\n\n#### B\n\n" + long, "", false, true},
		{"h2 then h4 tags", "<h2>A</h2><h4>B</h4> " + long, "", false, true},
		{"h2 then h4 across blocks", long, "## A\n\n#### B", false, true},
		{"h4 then h2 is fine", "#### B\n\n## A\n\n" + long, "", false, false},
		{"h2 h3 h4 is fine", "## A\n\n### B\n\n#### C\n\n" + long, "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := good()
			in.Markdown, in.BlocksText = c.md+"\n\n[x](/y)", c.blocks
			fs := Analyze(in)
			if _, got := has(fs, CodeH1InBody); got != c.h1 {
				t.Errorf("h1-in-body present=%v, want %v", got, c.h1)
			}
			if _, got := has(fs, CodeHeadingSkip); got != c.skip {
				t.Errorf("heading-skip present=%v, want %v", got, c.skip)
			}
		})
	}
	in := good()
	in.Markdown = "# A\n\n" + strings.Repeat("wort ", 200) + "[x](/y)"
	if f, _ := has(Analyze(in), CodeH1InBody); f.Severity != SeverityWarn {
		t.Errorf("h1-in-body is a warning, got %q", f.Severity)
	}
}

func TestAnalyzeImagesWithoutAlt(t *testing.T) {
	in := good()
	in.ImagesWithoutAlt = 2
	if f, got := has(Analyze(in), CodeImagesNoAlt); !got || f.Count != 2 || f.Severity != SeverityWarn {
		t.Errorf("images from the caller: %+v %v", f, got)
	}
	in = good()
	in.Markdown += "\n\n![](/media/1/a.jpg) ![mit Text](/media/1/b.jpg) ![ ](/media/1/c.jpg)"
	if f, got := has(Analyze(in), CodeImagesNoAlt); !got || f.Count != 2 {
		t.Errorf("empty markdown alt: %+v %v", f, got)
	}
	in = good()
	in.Markdown += "\n\n![Beschreibung](/media/1/b.jpg)"
	if _, got := has(Analyze(in), CodeImagesNoAlt); got {
		t.Error("an image with alt text is fine")
	}
}

func TestAnalyzeThinText(t *testing.T) {
	short := "## A\n\n" + strings.Repeat("wort ", 147) + "[x](/y)" // 147 + "A" + "x" = 149 words
	enough := "## A\n\n" + strings.Repeat("wort ", 150) + "[x](/y)"
	cases := []struct {
		name, md, kind, typeKey, blocks string
		thin                            bool
		words                           int
	}{
		{"149 words", short, "page", "", "", true, 149},
		{"150 words", enough, "page", "", "", false, 0},
		{"post", short, "post", "", "", true, 149},
		{"own kind is skipped", short, "page", "produkt", "", false, 0},
		{"blocks win over plain markdown", "", "page", "", strings.Repeat("wort ", 200) + "[x](/y)", false, 0},
		{"thin blocks", strings.Repeat("wort ", 400), "page", "", "[x](/y) kurz", true, 2},
		{"code fence is not text", "```\n" + strings.Repeat("wort ", 300) + "\n```\n[x](/y)", "page", "", "", true, 1},
		{"image targets are not words", strings.Repeat("wort ", 148) + "![](/a/b/c/d/e.jpg) [x](/y)", "page", "", "", true, 149},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := good()
			in.Markdown, in.Kind, in.TypeKey, in.BlocksText = c.md, c.kind, c.typeKey, c.blocks
			f, got := has(Analyze(in), CodeThinText)
			if got != c.thin {
				t.Fatalf("thin-text present=%v, want %v (%+v)", got, c.thin, f)
			}
			if got && f.Count != c.words {
				t.Errorf("words = %d, want %d", f.Count, c.words)
			}
		})
	}
}

func TestAnalyzeInternalLinks(t *testing.T) {
	body := func(link string) string { return "## A\n\n" + strings.Repeat("wort ", 200) + "\n\n" + link }
	cases := []struct {
		name, link string
		want       bool // finding present
	}{
		{"none", "", true},
		{"root-relative", "[a](/kontakt)", false},
		{"protocol-relative is external", "[a](//cdn.example.com/x)", true},
		{"external", "[a](https://other.example/x)", true},
		{"own host absolute", "[a](https://example.org/kontakt)", false},
		{"own host case", "[a](https://EXAMPLE.org/kontakt)", false},
		{"mailto", "[a](mailto:x@example.org)", true},
		{"anchor only", "[a](#oben)", true},
		{"relative", "[a](kontakt)", false},
		{"html anchor", `<a href="/kontakt">a</a>`, false},
		{"image is no link", "![a](/media/1/a.jpg)", true},
		{"link inside code fence", "```\n[a](/kontakt)\n```", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := good()
			in.Markdown = body(c.link)
			if _, got := has(Analyze(in), CodeNoInternalLinks); got != c.want {
				t.Errorf("no-internal-links present=%v, want %v", got, c.want)
			}
		})
	}
	// A link that only a block carries counts.
	in := good()
	in.Markdown = body("")
	in.BlocksText = "[Mehr](/mehr)"
	if _, got := has(Analyze(in), CodeNoInternalLinks); got {
		t.Error("a link in a block is a link")
	}
}

func TestAnalyzeSlug(t *testing.T) {
	cases := []struct {
		name, slug  string
		long, noise bool
	}{
		{"fine", "gute-seite", false, false},
		{"75", strings.Repeat("a", 75), false, false},
		{"76", strings.Repeat("a", 76), true, false},
		{"76 umlauts", strings.Repeat("ä", 76), true, false},
		{"75 umlauts", strings.Repeat("ä", 75), false, false},
		{"und", "kaffee-und-kuchen", false, true},
		{"the", "the-big-day", false, true},
		{"a", "a-day", false, true},
		{"inside a word is fine", "gruendung-andacht-ofen", false, false},
		{"single word", "die", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := good()
			in.Slug = c.slug
			fs := Analyze(in)
			if _, got := has(fs, CodeSlugLong); got != c.long {
				t.Errorf("slug-long present=%v, want %v", got, c.long)
			}
			if _, got := has(fs, CodeSlugNoise); got != c.noise {
				t.Errorf("slug-noise present=%v, want %v", got, c.noise)
			}
		})
	}
}

func TestAnalyzeStates(t *testing.T) {
	in := good()
	in.NoIndex, in.Protected, in.Status, in.HasFeatured = true, true, "draft", false
	fs := Analyze(in)
	for _, code := range []string{CodeNoIndex, CodeProtected, CodeDraft, CodeNoFeaturedImage} {
		f, got := has(fs, code)
		if !got || f.Severity != SeverityInfo {
			t.Errorf("%s: present=%v severity=%q, want an info", code, got, f.Severity)
		}
	}
	for _, code := range []string{CodeNoIndex, CodeProtected, CodeDraft, CodeNoFeaturedImage} {
		in := good()
		_, got := has(Analyze(in), code)
		if got {
			t.Errorf("%s on a plain page", code)
		}
	}
}

func TestAnalyzeOrderAndWorst(t *testing.T) {
	in := good()
	in.Title = ""                            // error
	in.Description = strings.Repeat("a", 10) // warn
	in.NoIndex = true                        // info
	in.Slug = "kaffee-und-kuchen"            // info
	fs := Analyze(in)
	for i := 1; i < len(fs); i++ {
		a, b := fs[i-1], fs[i]
		if a.Severity.Rank() < b.Severity.Rank() ||
			(a.Severity.Rank() == b.Severity.Rank() && a.Code > b.Code) {
			t.Fatalf("not ordered: %+v before %+v", a, b)
		}
	}
	if fs[0].Code != CodeTitleMissing {
		t.Errorf("first = %s, want the error", fs[0].Code)
	}
	if got := Worst(fs); got != SeverityError {
		t.Errorf("Worst = %q", got)
	}
	if got := Worst(nil); got != "" {
		t.Errorf("Worst(nil) = %q", got)
	}
	if got := Worst([]Finding{{Severity: SeverityInfo}, {Severity: SeverityWarn}}); got != SeverityWarn {
		t.Errorf("Worst = %q", got)
	}
	e, w, i := Count(fs)
	if e != 1 || w != 1 || i != 2 {
		t.Errorf("Count = %d %d %d", e, w, i)
	}
}

func TestCodesAreKnownAndAnalyzeOnlyReturnsKnown(t *testing.T) {
	in := Input{Markdown: "# a\n\n#### b\n\n![](x)", NoIndex: true, Protected: true, Status: "draft", Slug: "a-und-b",
		Others: []Other{{ID: 9, Title: "x", Status: "published"}}}
	for _, f := range Analyze(in) {
		if !Known(f.Code) {
			t.Errorf("Analyze returned %q, which Codes() does not list", f.Code)
		}
	}
	if Known("nonsense") {
		t.Error("nonsense is not a code")
	}
}
