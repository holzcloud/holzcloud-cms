package value

import "testing"

// The passages of a full-text search come from the Markdown source and are cut
// at token boundaries, so a marker can stand in one whole, with a highlighted
// term inside it, or cut in half at either end. Every case must come out as
// the value's text or as nothing, and never as a piece of the marker.
func TestResolvePlain(t *testing.T) {
	texts := map[string]string{"version-holzcloud-cms": "0.1.5"}
	for _, tc := range []struct {
		name, in, want string
	}{
		{"a complete marker", "Alpha [[wert:version-holzcloud-cms]]: neu", "Alpha 0.1.5: neu"},
		{"a highlighted term inside the marker",
			"Alpha [[wert:\x02version\x03-holzcloud-cms]]: neu", "Alpha 0.1.5: neu"},
		{"the highlighted word is the marker's own name",
			"Alpha [[\x02wert\x03:version-holzcloud-cms]]: neu", "Alpha 0.1.5: neu"},
		{"a highlight outside the marker stays",
			"\x02Alpha\x03 [[wert:version-holzcloud-cms]]: neu", "\x02Alpha\x03 0.1.5: neu"},
		{"cut off at the end", "…Alpha [[wert:version-holz…", "…Alpha …"},
		{"cut off at the end with a highlight", "…Alpha [[wert:\x02version\x03…", "…Alpha …"},
		{"a bare opening at the end", "…Alpha [[…", "…Alpha …"},
		{"a bare opening at the very end", "…Alpha [[", "…Alpha "},
		{"only the name at the end", "…Alpha [[wert…", "…Alpha …"},
		{"the name and the colon at the end", "…Alpha [[wert:", "…Alpha "},
		{"cut off at the start", "…cms]]: neu…", "…: neu…"},
		{"cut off at the start inside the name", "…ert:version-holzcloud-cms]]: neu…", "…: neu…"},
		{"cut off at the start with a highlight", "…\x02cms\x03]]: neu…", "…: neu…"},
		{"an unknown key", "Alpha [[wert:gibt-es-nicht]]: neu", "Alpha : neu"},
		{"two markers", "[[wert:version-holzcloud-cms]] und [[wert:version-holzcloud-cms]]", "0.1.5 und 0.1.5"},
		{"another marker family is left alone", "Fuss [[snippet:fuss]] hier", "Fuss [[snippet:fuss]] hier"},
		{"not escaped here", "a < b [[wert:version-holzcloud-cms]] & c", "a < b 0.1.5 & c"},
		{"no marker at all", "Alpha \x02neu\x03 und alt", "Alpha \x02neu\x03 und alt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolvePlain(tc.in, texts); got != tc.want {
				t.Errorf("ResolvePlain(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// The value's text is plain text: no markup, and a text cannot carry a
// sentinel byte, because CleanText refuses every control character — so what
// Highlight later turns into <mark> can only have come from the index.
func TestResolvePlainPutsTheTextInAsItStands(t *testing.T) {
	texts := map[string]string{"v": "<b>&</b>"}
	if got := ResolvePlain("x [[wert:v]] y", texts); got != "x <b>&</b> y" {
		t.Errorf("ResolvePlain = %q; want the raw text, escaped later by the caller", got)
	}
}

// A passage without a bracket pair is returned as it is, and that is the path
// nearly every search hit takes.
func TestResolvePlainLeavesAPassageWithoutBracketsAlone(t *testing.T) {
	in := "…Alpha \x02neu\x03 und alt…"
	allocs := testing.AllocsPerRun(100, func() {
		if ResolvePlain(in, nil) != in {
			t.Fatal("changed")
		}
	})
	if allocs != 0 {
		t.Errorf("a passage without brackets cost %v allocations; want 0", allocs)
	}
}
