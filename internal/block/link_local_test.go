package block

import "testing"

// A block link is typed by an editor and goes onto a public page as an href.
func TestBlockLinkDropsABackslashPath(t *testing.T) {
	for _, bad := range []string{`/\evil.example`, "/\tevil.example", "//evil.example"} {
		if got := safeURL(bad); got != "" {
			t.Errorf("safeURL(%q) = %q, want an empty link", bad, got)
		}
	}
	for _, good := range []string{"/x", "#anker", "https://example.ch", "/aktion-50%"} {
		if got := safeURL(good); got != good {
			t.Errorf("safeURL(%q) = %q, want it kept", good, got)
		}
	}
}
