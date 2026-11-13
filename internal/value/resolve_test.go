package value

import (
	"fmt"
	"html"
	"strings"
	"testing"
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
