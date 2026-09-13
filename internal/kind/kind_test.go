package kind

import "testing"

func TestKeyAusDemNamen(t *testing.T) {
	cases := map[string]string{
		"Produkt":            "produkt",
		"Termin im Kalender": "termin_im_kalender",
		"Käse & Molke":       "kaese_molke",
		"Grüße":              "gruesse",
		"  Tier  ":           "tier",
		"---":                "",
	}
	for in, want := range cases {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q; want %q", in, got, want)
		}
	}
}

// The key lands in pages.content_kind, in a form field and in an address. What
// causes trouble there is refused here.
func TestValidKey(t *testing.T) {
	for _, gut := range []string{"produkt", "termin_2026", "tier"} {
		if !ValidKey(gut) {
			t.Errorf("ValidKey(%q) = false", gut)
		}
	}
	for _, schlecht := range []string{"", "a", "Produkt", "produkt-2", "1produkt", "seite", "beitrag", "page", "post", "ärger"} {
		if ValidKey(schlecht) {
			t.Errorf("ValidKey(%q) = true", schlecht)
		}
	}
}

// What comes out of a form is a kind of this website or a page.
func TestPick(t *testing.T) {
	types := []Type{{Key: "produkt"}, {Key: "termin"}}
	cases := map[string]string{
		"produkt": "produkt",
		"termin":  "termin",
		"post":    Post,
		"page":    Page,
		"rezept":  Page, // does not exist on this website
		"":        Page,
	}
	for in, want := range cases {
		if got := Pick(in, types); got != want {
			t.Errorf("Pick(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestNameOf(t *testing.T) {
	types := []Type{{Key: "produkt", Name: "Produkt", Plural: "Produkte"}}
	// An own kind's name is what the operator typed and is never translated.
	if got := NameOf(types, "produkt", true); got != "Produkte" {
		t.Errorf("plural = %q", got)
	}
	if got := NameOf(types, "produkt", false); got != "Produkt" {
		t.Errorf("singular = %q", got)
	}
	// A built-in kind's name is a catalogue KEY — the screens draw it through
	// {{t .Name}} — so what comes back here is the key and not a German word.
	if got := NameOf(types, Post, true); got != "Posts" {
		t.Errorf("built-in = %q", got)
	}
	// A deleted kind: the entries still carry it, and they get a name instead of
	// a blank.
	if got := NameOf(types, "rezept", false); got != "rezept" {
		t.Errorf("vanished kind = %q", got)
	}
}

func TestByArchive(t *testing.T) {
	types := []Type{{Key: "produkt", Archive: "hofladen"}, {Key: "termin"}}
	if got, ok := ByArchive(types, "hofladen"); !ok || got.Key != "produkt" {
		t.Errorf("ByArchive(hofladen) = %+v, %v", got, ok)
	}
	if _, ok := ByArchive(types, ""); ok {
		t.Error("an empty address must not hit an overview — otherwise every page without a slug would lead there")
	}
	if _, ok := ByArchive(types, "irgendwas"); ok {
		t.Error("a foreign address hits an overview")
	}
}
