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

// Die Kennung landet in pages.art, in einem Formularfeld und in einer Adresse.
// Was dort Ärger macht, wird hier abgelehnt.
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

// Was aus einem Formular kommt, ist eine Art dieser Website oder eine Seite.
func TestPick(t *testing.T) {
	types := []Type{{Key: "produkt"}, {Key: "termin"}}
	cases := map[string]string{
		"produkt": "produkt",
		"termin":  "termin",
		"post":    Post,
		"page":    Page,
		"rezept":  Page, // gibt es auf dieser Website nicht
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
	if got := NameOf(types, "produkt", true); got != "Produkte" {
		t.Errorf("Mehrzahl = %q", got)
	}
	if got := NameOf(types, "produkt", false); got != "Produkt" {
		t.Errorf("Einzahl = %q", got)
	}
	if got := NameOf(types, Post, true); got != "Beiträge" {
		t.Errorf("eingebaut = %q", got)
	}
	// Eine gelöschte Art: die Einträge tragen sie noch, und sie bekommen einen
	// Namen statt einer Leerstelle.
	if got := NameOf(types, "rezept", false); got != "rezept" {
		t.Errorf("verschwundene Art = %q", got)
	}
}

func TestByArchive(t *testing.T) {
	types := []Type{{Key: "produkt", Archive: "hofladen"}, {Key: "termin"}}
	if got, ok := ByArchive(types, "hofladen"); !ok || got.Key != "produkt" {
		t.Errorf("ByArchive(hofladen) = %+v, %v", got, ok)
	}
	if _, ok := ByArchive(types, ""); ok {
		t.Error("eine leere Adresse darf keine Übersicht treffen — sonst führte jede Seite ohne Slug dorthin")
	}
	if _, ok := ByArchive(types, "irgendwas"); ok {
		t.Error("eine fremde Adresse trifft eine Übersicht")
	}
}
