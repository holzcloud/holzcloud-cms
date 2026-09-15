package block

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// The measurement WORD-04 asks for, in the one place that can answer it.
//
// Five words this program mints ride inside pages.content_html, written in the
// website's language at save: the lightbox's previous, next and close, the
// fallback between the <video> tags, and the accessible name of a gallery's
// scrolling region. Making them late is what closes window 34 — and it is a
// cost paid on every page view of every site, including the overwhelming
// majority that have one language and no gallery at all.
//
// So the numbers come before the mechanism. What each candidate costs is
// measured here against bodies the size real pages are, and 16-MEASUREMENT.md
// carries the result and the reading of it.
//
// Nothing in this file is a proposal. It is four ways of touching a stored body
// at delivery, written the cheapest way each could plausibly be written, so
// that the comparison flatters none of them.

// mdStub stands in for page.RenderMarkdown.
//
// The subject here is what it costs to walk a finished body, not what it costs
// to produce one, and a real Markdown parser in the corpus builder would put
// its own time into every number below. internal/block must not import
// internal/page in any case.
func mdStub(s string) (string, error) { return "<p>" + strings.TrimSpace(s) + "</p>", nil }

// para is one paragraph of a page, near enough to what an editor writes.
const para = `Die Werkstatt steht seit 1954 am selben Platz, und die Maschinen
darin sind zum Teil älter als das Haus. Wer hereinkommt, riecht zuerst das Holz
und hört dann die Absaugung. Wir bauen Möbel auf Maß, reparieren, was sich
reparieren lässt, und sagen es, wenn sich etwas nicht mehr lohnt.`

// benchBody builds a page body of roughly size bytes, optionally ending in a
// gallery of its own pictures (which is what freezes the five words) or in an
// album marker (which is what is already resolved late).
func benchBody(tb testing.TB, size int, gallery, album bool) string {
	tb.Helper()

	var blocks []Block
	for n := 0; n < size/len(para)+1; n++ {
		blocks = append(blocks, Block{Type: TypeText, Markdown: para})
	}
	if gallery {
		g := Block{Type: TypeGallery, Variant: "3", Display: "diashow"}
		for i := 1; i <= 8; i++ {
			g.Items = append(g.Items, Item{MediaID: int64(i)})
		}
		blocks = append(blocks, g)
	}
	if album {
		blocks = append(blocks, Block{Type: TypeGallery, Variant: "3", Display: "diashow", AlbumSlug: "werkstatt"})
	}

	look := func(id int64) (Image, bool) {
		return Image{
			URL:   fmt.Sprintf("/media/bild-%d.jpg", id),
			Alt:   "Ein Schrank aus Eiche",
			Width: 1600, Height: 1200,
		}, true
	}
	return Render(blocks, Builtin, look, mdStub)
}

// The corpus. Three sizes, because the cost of every candidate below is linear
// in the length of the body and a claim about "a page" is worth nothing without
// saying how big it is.
//
//	small  — a contact page or an imprint
//	medium — an article with pictures, which is what most pages are
//	large  — a long report, the size at which a per-request pass starts to show
var benchSizes = []struct {
	name string
	size int
}{
	{"small-2k", 2 << 10},
	{"medium-20k", 20 << 10},
	{"large-200k", 200 << 10},
}

// BenchmarkBodySize is not a benchmark. It reports what the corpus actually
// weighs, so the numbers beside it can be read as bytes per nanosecond rather
// than as three anonymous constants.
func BenchmarkBodySize(b *testing.B) {
	for _, s := range benchSizes {
		for _, withGallery := range []bool{false, true} {
			name := s.name
			if withGallery {
				name += "-gallery"
			}
			b.Run(name, func(b *testing.B) {
				body := benchBody(b, s.size, withGallery, false)
				b.ReportMetric(float64(len(body)), "bytes")
				b.ReportMetric(float64(strings.Count(body, "aria-label")), "aria-labels")
				for i := 0; i < b.N; i++ {
					_ = body
				}
			})
		}
	}
}

// Candidate A: the cheap question, already paid on every public request.
//
// strings.Contains for a prefix that is not there. This is the floor — whatever
// a mechanism costs, it costs at least one pass like this, and the point of
// measuring it separately is to know how much of the total is unavoidable.
func BenchmarkCheapQuestion(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) {
			body := benchBody(b, s.size, false, false)
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if hasAlbumMarker(body) {
					b.Fatal("a body with no album named one")
				}
			}
		})
	}
}

// Candidate B: a regular expression over the whole body, unconditionally.
//
// This is what "resolve everything late" costs if it is written the obvious
// way: one pattern, one pass, every request, whether or not the page has
// anything to resolve. ReplaceAlbumMarkers deliberately does NOT do this — it
// asks candidate A first — and this benchmark is why that guard exists.
var benchWordPattern = regexp.MustCompile(`\[\[w:([a-z-]+)\]\]`)

func BenchmarkRegexpOverWholeBody(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) {
			body := benchBody(b, s.size, false, false)
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if benchWordPattern.MatchString(body) {
					b.Fatal("a body with no word marker matched one")
				}
			}
		})
	}
}

// Candidate C: a sentinel substituted at delivery.
//
// The save would write [[w:gallery]] where it now writes "Galerie", and the
// public handler would replace the five sentinels with the words of the page's
// language. strings.Replacer is the cheapest honest way to write that: one
// pass, five needles, no regular expression.
//
// Measured twice on purpose — with and without candidate A in front of it —
// because the difference between those two IS the decision. If the guarded form
// costs what the guard costs, the common case pays nothing and the mechanism is
// affordable; if the replacer is already cheap enough, the guard is complexity
// for its own sake.
var benchReplacer = strings.NewReplacer(
	"[[w:previous]]", "Vorheriges Bild",
	"[[w:next]]", "Nächstes Bild",
	"[[w:close]]", "Große Ansicht schließen",
	"[[w:novideo]]", "Ihr Browser kann dieses Video nicht abspielen.",
	"[[w:gallery]]", "Galerie",
)

const benchSentinelPrefix = "[[w:"

func BenchmarkSentinelReplace(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) {
			body := benchBody(b, s.size, false, false)
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if out := benchReplacer.Replace(body); len(out) != len(body) {
					b.Fatal("a body with no sentinel changed length")
				}
			}
		})
	}
}

func BenchmarkSentinelReplaceGuarded(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) {
			body := benchBody(b, s.size, false, false)
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out := body
				if strings.Contains(body, benchSentinelPrefix) {
					out = benchReplacer.Replace(body)
				}
				if len(out) != len(body) {
					b.Fatal("a body with no sentinel changed length")
				}
			}
		})
	}
}

// The same two, on a page that DOES carry a gallery — the case the mechanism
// exists for. Here the replacer has work to do and the guard cannot skip it, so
// this is the honest upper bound of candidate C.
func BenchmarkSentinelReplaceHit(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) {
			body := benchBody(b, s.size, true, false)
			// Stand in for what the save would have written.
			// The source language is English since v2.0, and block.Builtin
			// translates nothing, so these are the words the corpus actually
			// carries — the same constants textGallery and friends hold.
			body = strings.ReplaceAll(body, "Gallery", "[[w:gallery]]")
			body = strings.ReplaceAll(body, "Previous image", "[[w:previous]]")
			body = strings.ReplaceAll(body, "Next image", "[[w:next]]")
			body = strings.ReplaceAll(body, "Close large view", "[[w:close]]")
			if !strings.Contains(body, benchSentinelPrefix) {
				b.Fatalf("the corpus carries no sentinel; the words the renderer mints have changed")
			}
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = benchReplacer.Replace(body)
			}
		})
	}
}

// Candidate D: render the blocks again, per request.
//
// The broad answer — store the blocks and never freeze anything. It needs no
// sentinel, no marker and no migration of stored HTML, and it is measured here
// so that "probably not the right one" (16-CONTEXT §4) is a number rather than
// an instinct. Media lookups are served from a map, which is the most
// favourable case: a real request would add a query per picture.
func BenchmarkRenderAgain(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) {
			var blocks []Block
			for n := 0; n < s.size/len(para)+1; n++ {
				blocks = append(blocks, Block{Type: TypeText, Markdown: para})
			}
			g := Block{Type: TypeGallery, Variant: "3", Display: "diashow"}
			for i := 1; i <= 8; i++ {
				g.Items = append(g.Items, Item{MediaID: int64(i)})
			}
			blocks = append(blocks, g)

			look := func(id int64) (Image, bool) {
				return Image{URL: fmt.Sprintf("/media/bild-%d.jpg", id), Alt: "Ein Schrank aus Eiche", Width: 1600, Height: 1200}, true
			}
			b.SetBytes(int64(len(Render(blocks, Builtin, look, mdStub))))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = Render(blocks, Builtin, look, mdStub)
			}
		})
	}
}

// What an album marker already costs, for scale.
//
// This is the one late mechanism the program already has, shipped in v2.2 and
// paid on every page that names an album. Any new mechanism that costs less
// than this on the common case is affordable by a standard this project has
// already accepted; one that costs more needs an argument.
func BenchmarkAlbumMarkerExpansion(b *testing.B) {
	for _, s := range benchSizes {
		b.Run(s.name, func(b *testing.B) {
			body := benchBody(b, s.size, false, true)
			if !hasAlbumMarker(body) {
				b.Fatal("the corpus carries no album marker")
			}
			expand := func(slug string, at, columns int, mod string) string {
				return GalleryWrapper(columns, mod, "<figure></figure>", nil)
			}
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ReplaceAlbumMarkers(body, expand)
			}
		})
	}
}
