package value

import "testing"

func TestResolveSummary(t *testing.T) {
	texts := map[string]string{"v": "0.1.5", "name": "<Holz & Co>"}
	for _, tc := range []struct{ name, in, want string }{
		{"no marker comes back as it is", "Alpha: neu", "Alpha: neu"},
		{"a known key is its text", "Alpha [[wert:v]]: neu", "Alpha 0.1.5: neu"},
		{"the text is not escaped here; the template does that", "Von [[wert:name]]", "Von <Holz & Co>"},
		{"an unknown key before punctuation leaves no orphan space", "Alpha [[wert:x]]: neu", "Alpha: neu"},
		{"an unknown key between words leaves one space", "Alpha [[wert:x]] neu", "Alpha neu"},
		{"an unknown key at the end", "Stand [[wert:x]]", "Stand"},
		{"an unknown key at the start", "[[wert:x]] Stand", "Stand"},
		{"two unknown keys side by side", "A [[wert:x]] [[wert:y]] B", "A B"},
		{"a marker of another family is left alone", "A [[snippet:s]] [[wert:v]]", "A [[snippet:s]] 0.1.5"},
		{"half a marker at the end is removed", "Alpha [[wert:vers…", "Alpha …"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveSummary(tc.in, texts); got != tc.want {
				t.Errorf("ResolveSummary(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
	if got := ResolveSummary("Alpha [[wert:v]]: neu", nil); got != "Alpha: neu" {
		t.Errorf("without texts every marker is nothing: got %q", got)
	}
}
