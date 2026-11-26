package field

import (
	"strings"
	"testing"
)

// A browser reads "\" as "/" and drops a tab before it resolves, so "/\host"
// and "/<TAB>/host" are "//host": the editor's link leaves the site while
// looking like a path (RDR-03).
func TestLinkFieldRefusesABackslashPath(t *testing.T) {
	d := Def{Label: "Ziel", Kind: KindLink}

	for _, bad := range []string{`/\evil.example`, "/\tevil.example"} {
		reason := Check(d, bad)
		if reason.Empty() {
			t.Errorf("Check(%q) was let through", bad)
			continue
		}
		if !strings.Contains(reason.String(), "backslash") {
			t.Errorf("Check(%q) = %q, want the backslash reason", bad, reason)
		}
	}

	if reason := Check(d, "//evil.example"); reason.Empty() || !strings.Contains(reason.String(), "two slashes") {
		t.Errorf("Check(//evil.example) = %q, want the two-slashes reason", reason)
	}

	for _, good := range []string{"/kontakt", "https://example.ch/x", "mailto:a@example.ch"} {
		if reason := Check(d, good); !reason.Empty() {
			t.Errorf("Check(%q) = %q, want it accepted", good, reason)
		}
	}
}

// A value stored before the check existed must not reach a theme as an href.
func TestResolveEmptiesALinkThatLeavesTheSite(t *testing.T) {
	defs := []Def{{Key: "ziel", Label: "Ziel", Kind: KindLink}}
	resolve := func(v string) any {
		return Resolve(defs, Data{Values: map[string]string{"ziel": v}}, Links{})["ziel"]
	}

	for _, bad := range []string{`/\evil.example`, "/\tevil.example", "//evil.example"} {
		if got := resolve(bad); got != "" {
			t.Errorf("Resolve(%q) = %v, want an empty value", bad, got)
		}
	}
	for _, good := range []string{"/kontakt", "https://example.ch/x"} {
		if got := resolve(good); got != good {
			t.Errorf("Resolve(%q) = %v, want it unchanged", good, got)
		}
	}

	entries := List(defs, Data{Values: map[string]string{"ziel": `/\evil.example`}}, Links{})
	if len(entries) != 0 {
		t.Errorf("List kept %d entries for a link that leaves the site, want none", len(entries))
	}
}
