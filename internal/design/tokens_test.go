package design

import (
	"strings"
	"testing"
)

func TestOnlyHexColoursSurvive(t *testing.T) {
	good := map[string]string{
		"#fff":      "#fff",
		"#FFFFFF":   "#ffffff",
		"#1a6dd4":   "#1a6dd4",
		"#1a6dd4cc": "#1a6dd4cc",
		"  #abc  ":  "#abc",
	}
	for in, want := range good {
		if got := hexColour(in); got != want {
			t.Errorf("hexColour(%q) = %q, want %q", in, got, want)
		}
	}

	// Anything that is not a hex colour is dropped rather than parsed. Each
	// extra syntax would need its own parser, and each parser is a place a
	// closing brace could slip through into the stylesheet.
	for _, bad := range []string{
		"", "red", "rgb(1,2,3)", "oklch(0.5 0.1 20)", "#12", "#12345",
		"#gggggg", "#fff;}", "#fff}", "url(x)", "#fff /*", "var(--x)",
		"#fff;color:red", "javascript:alert(1)",
	} {
		if got := hexColour(bad); got != "" {
			t.Errorf("hexColour(%q) = %q, want it dropped", bad, got)
		}
	}
}

func TestNothingChosenProducesNoRule(t *testing.T) {
	// A theme that received an empty :root rule would emit an empty <style>
	// element on every page for nothing.
	if css := (Tokens{Radius: -1}).CSS(); css != "" {
		t.Errorf("CSS() = %q, want empty", css)
	}
	if !(Tokens{Radius: -1}).Empty() {
		t.Error("untouched tokens do not report themselves empty")
	}
}

func TestCSSCarriesOnlyWhatWasChosen(t *testing.T) {
	css := string(Tokens{Brand: "#1a6dd4", Radius: -1}.CSS())

	if !strings.Contains(css, "--hc-brand:#1a6dd4;") {
		t.Errorf("the chosen colour is missing:\n%s", css)
	}
	// A token nobody set must not appear: it would override a theme that had a
	// better answer for it.
	for _, unwanted := range []string{"--hc-ink", "--hc-paper", "--hc-font", "--hc-measure", "--hc-radius"} {
		if strings.Contains(css, unwanted) {
			t.Errorf("%s was emitted although it was never chosen:\n%s", unwanted, css)
		}
	}
	if !strings.HasPrefix(css, ":root{") || !strings.HasSuffix(css, "}") {
		t.Errorf("not a single :root rule:\n%s", css)
	}
}

func TestAnUnknownFontIsDropped(t *testing.T) {
	// A name typed by hand would either not be installed — silently falling
	// back to something else — or be a URL, which would load a font from
	// another server.
	css := string(Tokens{Font: `Comic Sans MS", url(https://evil.example/f.woff2), "`, Radius: -1}.CSS())
	if css != "" {
		t.Errorf("a hand-typed font produced %q", css)
	}

	css = string(Tokens{Font: "serif", Radius: -1}.CSS())
	if !strings.Contains(css, "--hc-font:Georgia") {
		t.Errorf("an offered stack was not emitted:\n%s", css)
	}
	// The stack is one of ours, so it cannot end the declaration early.
	if strings.Count(css, "{") != 1 || strings.Count(css, "}") != 1 {
		t.Errorf("the rule has extra braces:\n%s", css)
	}
}

func TestMeasureAndRadiusAreBounded(t *testing.T) {
	for _, n := range []int{-5, 0, 39, 111, 5000} {
		if got := Sanitize(Tokens{Measure: n}).Measure; got != 0 {
			t.Errorf("Measure %d survived as %d", n, got)
		}
	}
	if got := Sanitize(Tokens{Measure: 70}).Measure; got != 70 {
		t.Errorf("a usable measure was dropped: %d", got)
	}

	for _, n := range []int{-2, 33, 9999} {
		if got := Sanitize(Tokens{Radius: n}).Radius; got != -1 {
			t.Errorf("Radius %d survived as %d", n, got)
		}
	}
	// Zero is a real choice — square corners on purpose — and must not be
	// confused with "not set".
	if got := Sanitize(Tokens{Radius: 0}).Radius; got != 0 {
		t.Errorf("a deliberate 0 radius was treated as unset: %d", got)
	}
	css := string(Tokens{Radius: 0}.CSS())
	if !strings.Contains(css, "--hc-radius:0px;") {
		t.Errorf("square corners were not emitted:\n%s", css)
	}
}

func TestOneBadValueDoesNotLoseTheGoodOnes(t *testing.T) {
	// Refusing the whole form over a mistyped colour would cost the settings
	// that were right.
	got := Sanitize(Tokens{
		Ink: "nicht-eine-farbe", Paper: "#ffffff", Brand: "#1a6dd4",
		Font: "erfunden", Measure: 70, Radius: 8,
	})
	if got.Ink != "" {
		t.Errorf("the bad colour survived as %q", got.Ink)
	}
	if got.Font != "" {
		t.Errorf("the bad font survived as %q", got.Font)
	}
	if got.Paper != "#ffffff" || got.Brand != "#1a6dd4" || got.Measure != 70 || got.Radius != 8 {
		t.Errorf("a good value was lost with the bad one: %+v", got)
	}
}

func TestCSSCannotBeMadeToEscapeItsRule(t *testing.T) {
	css := string(Tokens{
		Ink:    "#fff;} body{display:none} :root{",
		Paper:  "#000",
		Font:   `x"; background:url(https://evil.example/p.png); "`,
		Radius: -1,
	}.CSS())

	if strings.Count(css, "{") != 1 || strings.Count(css, "}") != 1 {
		t.Errorf("the rule was broken open:\n%s", css)
	}
	if strings.Contains(css, "url(") || strings.Contains(css, "display:none") {
		t.Errorf("an injected declaration survived:\n%s", css)
	}
	if !strings.Contains(css, "--hc-paper:#000;") {
		t.Errorf("the one valid value was lost:\n%s", css)
	}
}
