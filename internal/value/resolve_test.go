package value

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// v is the one website's texts most tests resolve against.
var v = map[string]string{"v": "0.1.5"}

func TestResolveReplacesAMarkerInText(t *testing.T) {
	cases := []struct{ in, want string }{
		{`<p>Alpha [[wert:v]]: neu</p>`, `<p>Alpha 0.1.5: neu</p>`},
		{`<h2>Version [[wert:v]]</h2>`, `<h2>Version 0.1.5</h2>`},
		{`<li>Punkt [[wert:v]]</li>`, `<li>Punkt 0.1.5</li>`},
		// Code is text too (D-13, A9): a version written as code is still the
		// version, and the reader would not expect the marker there.
		{`<code>[[wert:v]]</code>`, `<code>0.1.5</code>`},
	}
	for _, c := range cases {
		if got := Resolve(c.in, v); got != c.want {
			t.Errorf("Resolve(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestResolveEscapesTheTextItPutsIn(t *testing.T) {
	text := `<b>"x" & y</b>`
	got := Resolve(`<p>[[wert:k]]</p>`, map[string]string{"k": text})
	want := `<p>` + html.EscapeString(text) + `</p>`
	if got != want {
		t.Fatalf("Resolve = %q; want %q — a value is plain text and never an element", got, want)
	}
	if strings.Contains(got, "<b>") {
		t.Errorf("the value's markup arrived as markup: %q", got)
	}
}

func TestAnUnknownKeyBecomesNothing(t *testing.T) {
	in := `<p>Alpha [[wert:nope]]: neu</p>`
	if got := Resolve(in, v); got != `<p>Alpha : neu</p>` {
		t.Errorf("unknown key: %q; want %q", got, `<p>Alpha : neu</p>`)
	}
	if got := Resolve(`<p>Alpha [[wert:v]]: neu</p>`, nil); got != `<p>Alpha : neu</p>` {
		t.Errorf("nil map: %q; want every marker empty", got)
	}
}

func TestAMarkerWhereTextIsNotReadIsRemoved(t *testing.T) {
	cases := []string{
		`<script>var a = "[[wert:v]]";</script>`,
		`<style>p::after{content:"[[wert:v]]"}</style>`,
		`<title>[[wert:v]]</title>`,
		`<!-- [[wert:v]] -->`,
	}
	for _, in := range cases {
		got := Resolve(in, v)
		if strings.Contains(got, "[[wert:") {
			t.Errorf("Resolve(%q) = %q; the marker is still there", in, got)
		}
		if strings.Contains(got, "0.1.5") {
			t.Errorf("Resolve(%q) = %q; expanded where the browser does not read text", in, got)
		}
	}
}

func TestResolveLeavesADocumentWithoutAMarkerAlone(t *testing.T) {
	in := `<p>Alpha <a href="/x%20y">x</a> [[w:next]] [[snippet:a]]</p>`
	if got := Resolve(in, v); got != in {
		t.Errorf("Resolve changed a marker-free document:\n got %q\nwant %q", got, in)
	}
}

func TestResolveInsideAnHref(t *testing.T) {
	in := `<a href="https://x.ch/tag/v[[wert:v]]" rel="nofollow">Release</a>`
	want := `<a href="https://x.ch/tag/v0.1.5" rel="nofollow">Release</a>`
	if got := Resolve(in, v); got != want {
		t.Errorf("Resolve = %q; want %q", got, want)
	}
}

// hrefsOf parses a document and returns, per <a> start tag, its href values.
func hrefsOf(t *testing.T, doc string) [][]string {
	t.Helper()
	var out [][]string
	z := xhtml.NewTokenizer(strings.NewReader(doc))
	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			return out
		}
		if tt != xhtml.StartTagToken && tt != xhtml.SelfClosingTagToken {
			continue
		}
		tok := z.Token()
		if tok.Data != "a" {
			continue
		}
		var hrefs []string
		for _, a := range tok.Attr {
			if a.Key == "href" {
				hrefs = append(hrefs, a.Val)
			}
		}
		out = append(out, hrefs)
	}
}

func TestAValueCannotChangeALinksSchemeOrHost(t *testing.T) {
	cases := []struct {
		name, href, text string
		check            func(t *testing.T, href string)
	}{
		{"javascript after a path", "https://x.ch/v[[wert:k]]", "javascript:alert(1)", func(t *testing.T, href string) {
			u, err := url.Parse(href)
			if err != nil || u.Scheme != "https" || u.Host != "x.ch" {
				t.Errorf("href %q: scheme or host moved", href)
			}
		}},
		{"protocol-relative", "/[[wert:k]]", "/evil.example", func(t *testing.T, href string) {
			if href != "/" {
				t.Errorf("href %q; want the value dropped and \"/\" left", href)
			}
		}},
		{"backslash", "/[[wert:k]]", `\evil.example`, func(t *testing.T, href string) {
			if href != "/" {
				t.Errorf("href %q; want the value dropped and \"/\" left", href)
			}
		}},
		{"host", "https://[[wert:k]]/", "evil.example", func(t *testing.T, href string) {
			if strings.Contains(href, "evil.example") {
				t.Errorf("href %q: the value became the host", href)
			}
		}},
		{"whole address", "[[wert:k]]", "javascript:alert(1)", func(t *testing.T, href string) {
			if href != "" {
				t.Errorf("href %q; want the value dropped", href)
			}
		}},
		{"space and umlaut", "/suche/[[wert:k]]", "zwei wörter", func(t *testing.T, href string) {
			if href != "/suche/zwei%20w%C3%B6rter" {
				t.Errorf("href %q; want %q", href, "/suche/zwei%20w%C3%B6rter")
			}
		}},
		{"markup", "https://x.ch/v[[wert:k]]", `"><script>alert(1)</script>`, func(t *testing.T, href string) {
			u, err := url.Parse(href)
			if err != nil || u.Scheme != "https" || u.Host != "x.ch" {
				t.Errorf("href %q: scheme or host moved", href)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := Resolve(`<p><a href="`+c.href+`" rel="nofollow">L</a></p>`, map[string]string{"k": c.text})
			if strings.Contains(strings.ToLower(out), "<script") {
				t.Fatalf("the value added an element: %s", out)
			}
			links := hrefsOf(t, out)
			if len(links) != 1 || len(links[0]) != 1 {
				t.Fatalf("want exactly one <a> with exactly one href, got %v in %s", links, out)
			}
			if strings.HasPrefix(strings.ToLower(links[0][0]), "javascript:") {
				t.Fatalf("href %q starts with javascript:", links[0][0])
			}
			c.check(t, links[0][0])
		})
	}
}

func TestResolveInATextAttribute(t *testing.T) {
	texts := map[string]string{"k": `Version "1" & <2>`}
	for _, attr := range []string{"aria-label", "alt", "title"} {
		in := `<img src="/a.png" ` + attr + `="Bild [[wert:k]]">`
		out := Resolve(in, texts)
		z := xhtml.NewTokenizer(strings.NewReader(out))
		z.Next()
		tok := z.Token()
		got := ""
		for _, a := range tok.Attr {
			if a.Key == attr {
				got = a.Val
			}
		}
		if got != `Bild Version "1" & <2>` {
			t.Errorf("%s: read back %q from %s", attr, got, out)
		}
		if strings.Contains(out, "<2>") {
			t.Errorf("%s: the text was not escaped: %s", attr, out)
		}
	}
	for _, attr := range []string{"class", "id", "style", "data-x"} {
		in := `<span ` + attr + `="a[[wert:k]]b">x</span>`
		out := Resolve(in, texts)
		if strings.Contains(out, "Version") || strings.Contains(out, "[[wert:") {
			t.Errorf("%s: the marker must be removed, got %s", attr, out)
		}
	}
}

func TestOnlyTheMarkedTagChanges(t *testing.T) {
	before := markerFreeBody(8 << 10)
	after := `<p>Ende &amp; Schluss <a title="x" href="/a%20b">x</a></p>` + markerFreeBody(4<<10)
	in := before + `<a class="k" href="https://x.ch/tag/v[[wert:v]]" rel="nofollow">R</a>` + after
	out := Resolve(in, v)
	if !strings.HasPrefix(out, before) {
		t.Errorf("bytes before the marked tag changed")
	}
	if !strings.HasSuffix(out, after) {
		t.Errorf("bytes after the marked tag changed")
	}
	if !strings.Contains(out, `href="https://x.ch/tag/v0.1.5"`) {
		t.Errorf("the marked href was not resolved")
	}
}

// markerFreeBody is a page of the size frozen_bench_test.go calls medium,
// paragraphs with %20-encoded addresses like the corpus 33-RESEARCH.md F2
// measured on.
func markerFreeBody(size int) string {
	const para = `<p>Die Werkstatt steht seit 1954 am selben Platz, und die Maschinen darin sind ` +
		`zum Teil älter als das Haus. <a href="/werkstatt/unsere%20maschinen">Mehr</a> und ` +
		`<a href="https://example.ch/a%20b?c=d">dort</a>.</p>` + "\n"
	var b strings.Builder
	for b.Len() < size {
		b.WriteString(para)
	}
	return b.String()
}

func TestTheNoMarkerPathAllocatesNothing(t *testing.T) {
	body := markerFreeBody(23 << 10)
	allocs := testing.AllocsPerRun(100, func() {
		if Resolve(body, v) != body {
			t.Fatal("changed")
		}
	})
	if allocs != 0 {
		t.Fatalf("Resolve on a marker-free body allocates %v times per call; want 0", allocs)
	}
}

func BenchmarkResolveWithoutAMarker(b *testing.B) {
	for _, size := range []int{4 << 10, 23 << 10, 100 << 10} {
		body := markerFreeBody(size)
		b.Run(fmt.Sprintf("%dk", size>>10), func(b *testing.B) {
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = Resolve(body, v)
			}
		})
	}
}
