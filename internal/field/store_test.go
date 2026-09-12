package field

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// newFieldStore creates a fresh database along with a website.
//
// A file and not an in-memory database: db.Open checks the WAL pragmas after
// opening, and an in-memory database cannot satisfy them.
func newFieldStore(t *testing.T) (*Store, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("Wanderungen: %v", err)
	}
	res, err := database.Write.Exec(
		`INSERT INTO websites (name, description) VALUES ('Prüfsite', '')`)
	if err != nil {
		t.Fatalf("Website anlegen: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Website-Nummer: %v", err)
	}
	return NewStore(database), id
}

// Three probe fields and not one, because validate empties every property that
// does not fit the chosen kind: the display lives on a choice, the maximum on a
// multi-choice, the two bounds on a range field. Not one field can carry all
// three. Together the three cover all four new columns, and that is the point
// here: each of the four has to arrive on every read path.
func auswahlFeld(websiteID int64, key string) Def {
	return Def{
		WebsiteID: websiteID, Key: key, Label: "Farbe", Kind: KindChoice,
		Choices: []string{"rot", "blau"},
		Display: DisplayButtons,
	}
}

func mehrfachFeld(websiteID int64, key string) Def {
	return Def{
		WebsiteID: websiteID, Key: key, Label: "Zutaten", Kind: KindMulti,
		Choices:   []string{"salz", "pfeffer"},
		MaxValues: 3,
	}
}

func bereichFeld(websiteID int64, key string) Def {
	return Def{
		WebsiteID: websiteID, Key: key, Label: "Menge", Kind: KindRange,
		RangeMin: "1", RangeMax: "9",
	}
}

func pruefeAuswahl(t *testing.T, wo string, d Def) {
	t.Helper()
	if d.Display != DisplayButtons {
		t.Errorf("%s: Darstellung = %q, erwartet %q", wo, d.Display, DisplayButtons)
	}
}

func pruefeMehrfach(t *testing.T, wo string, d Def) {
	t.Helper()
	if d.MaxValues != 3 {
		t.Errorf("%s: maximum = %d, expected 3", wo, d.MaxValues)
	}
}

func pruefeBereich(t *testing.T, wo string, d Def) {
	t.Helper()
	if d.RangeMin != "1" || d.RangeMax != "9" {
		t.Errorf("%s: Grenzen = %q/%q, erwartet \"1\"/\"9\"", wo, d.RangeMin, d.RangeMax)
	}
}

func finde(t *testing.T, wo string, defs []Def, key string) Def {
	t.Helper()
	for _, d := range defs {
		if d.Key == key {
			return d
		}
	}
	t.Fatalf("%s: field %q not found", wo, key)
	return Def{}
}

// TestNeueSpalten is the real gate on the nine SQL sites.
//
// page_field_defs' column list stands nine times in store.go: seven SELECTs
// (List, Sub, OfBlockType, OfBlockTypes, OfSnippet, OfSnippets, Get), the
// INSERT and the UPDATE. Forget one and a field loads silently with a zero
// value — a row of buttons appears as a drop-down, a bound is not enforced, and
// nowhere is there an error. Counting the occurrences would not find that: a
// SELECT can name the column and still never write it into the Def. So this
// reads rather than counts.
func TestNeueSpalten(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()

	// --- Get, for a field of the page itself --------------------------------
	oben1, err := store.Create(ctx, auswahlFeld(site, "farbe"))
	if err != nil {
		t.Fatalf("Auswahl anlegen: %v", err)
	}
	oben2, err := store.Create(ctx, mehrfachFeld(site, "zutaten"))
	if err != nil {
		t.Fatalf("Mehrfachauswahl anlegen: %v", err)
	}
	oben3, err := store.Create(ctx, bereichFeld(site, "menge"))
	if err != nil {
		t.Fatalf("Bereich anlegen: %v", err)
	}
	pruefeAuswahl(t, "Get (Seitenfeld)", *oben1)
	pruefeMehrfach(t, "Get (Seitenfeld)", *oben2)
	pruefeBereich(t, "Get (Seitenfeld)", *oben3)

	// --- List ---------------------------------------------------------------
	top, err := store.List(ctx, site)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	pruefeAuswahl(t, "List", finde(t, "List", top, "farbe"))
	pruefeMehrfach(t, "List", finde(t, "List", top, "zutaten"))
	pruefeBereich(t, "List", finde(t, "List", top, "menge"))

	// --- Sub, for a field inside a group ------------------------------------
	gruppe, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "zeiten", Label: "Öffnungszeiten", Kind: KindGroup})
	if err != nil {
		t.Fatalf("Gruppe anlegen: %v", err)
	}
	inGruppe1 := auswahlFeld(site, "gfarbe")
	inGruppe1.ParentID = gruppe.ID
	if _, err := store.Create(ctx, inGruppe1); err != nil {
		t.Fatalf("Auswahl in Gruppe: %v", err)
	}
	inGruppe2 := mehrfachFeld(site, "gzutaten")
	inGruppe2.ParentID = gruppe.ID
	if _, err := store.Create(ctx, inGruppe2); err != nil {
		t.Fatalf("Mehrfachauswahl in Gruppe: %v", err)
	}
	inGruppe3 := bereichFeld(site, "gmenge")
	inGruppe3.ParentID = gruppe.ID
	if _, err := store.Create(ctx, inGruppe3); err != nil {
		t.Fatalf("Bereich in Gruppe: %v", err)
	}
	sub, err := store.Sub(ctx, site, gruppe.ID)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	pruefeAuswahl(t, "Sub", finde(t, "Sub", sub, "gfarbe"))
	pruefeMehrfach(t, "Sub", finde(t, "Sub", sub, "gzutaten"))
	pruefeBereich(t, "Sub", finde(t, "Sub", sub, "gmenge"))

	// And the same path once more through List, which builds the tree in memory.
	top, err = store.List(ctx, site)
	if err != nil {
		t.Fatalf("List (zweites Mal): %v", err)
	}
	pruefeAuswahl(t, "List/Sub", finde(t, "List/Sub", finde(t, "List", top, "zeiten").Sub, "gfarbe"))
	pruefeBereich(t, "List/Sub", finde(t, "List/Sub", finde(t, "List", top, "zeiten").Sub, "gmenge"))

	// --- OfBlockType und OfBlockTypes ---------------------------------------
	res, err := store.DB.Write.ExecContext(ctx,
		`INSERT INTO block_types (website_id, key, name) VALUES ($1, 'karte', 'Karte')`, site)
	if err != nil {
		t.Fatalf("Bausteinart anlegen: %v", err)
	}
	bausteinart, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Bausteinart-Nummer: %v", err)
	}
	imBaustein1 := auswahlFeld(site, "bfarbe")
	imBaustein1.BlockTypeID = bausteinart
	if _, err := store.Create(ctx, imBaustein1); err != nil {
		t.Fatalf("Auswahl im Baustein: %v", err)
	}
	imBaustein2 := mehrfachFeld(site, "bzutaten")
	imBaustein2.BlockTypeID = bausteinart
	if _, err := store.Create(ctx, imBaustein2); err != nil {
		t.Fatalf("Mehrfachauswahl im Baustein: %v", err)
	}
	imBaustein3 := bereichFeld(site, "bmenge")
	imBaustein3.BlockTypeID = bausteinart
	if _, err := store.Create(ctx, imBaustein3); err != nil {
		t.Fatalf("Bereich im Baustein: %v", err)
	}
	derBaustein, err := store.OfBlockType(ctx, site, bausteinart)
	if err != nil {
		t.Fatalf("OfBlockType: %v", err)
	}
	pruefeAuswahl(t, "OfBlockType", finde(t, "OfBlockType", derBaustein, "bfarbe"))
	pruefeMehrfach(t, "OfBlockType", finde(t, "OfBlockType", derBaustein, "bzutaten"))
	pruefeBereich(t, "OfBlockType", finde(t, "OfBlockType", derBaustein, "bmenge"))

	alleBausteine, err := store.OfBlockTypes(ctx, site)
	if err != nil {
		t.Fatalf("OfBlockTypes: %v", err)
	}
	pruefeAuswahl(t, "OfBlockTypes", finde(t, "OfBlockTypes", alleBausteine[bausteinart], "bfarbe"))
	pruefeMehrfach(t, "OfBlockTypes", finde(t, "OfBlockTypes", alleBausteine[bausteinart], "bzutaten"))
	pruefeBereich(t, "OfBlockTypes", finde(t, "OfBlockTypes", alleBausteine[bausteinart], "bmenge"))

	// --- Update -------------------------------------------------------------
	geaendert := *oben1
	geaendert.Display = ""
	if err := store.Update(ctx, site, oben1.ID, geaendert); err != nil {
		t.Fatalf("Update (Auswahl): %v", err)
	}
	nach, err := store.Get(ctx, site, oben1.ID)
	if err != nil {
		t.Fatalf("Get nach Update: %v", err)
	}
	if nach.Display != "" {
		t.Errorf("Update: Darstellung = %q, erwartet leer", nach.Display)
	}

	geaendert2 := *oben2
	geaendert2.MaxValues = 7
	if err := store.Update(ctx, site, oben2.ID, geaendert2); err != nil {
		t.Fatalf("Update (Mehrfachauswahl): %v", err)
	}
	nach2, err := store.Get(ctx, site, oben2.ID)
	if err != nil {
		t.Fatalf("Get nach Update: %v", err)
	}
	if nach2.MaxValues != 7 {
		t.Errorf("Update: maximum = %d, expected 7", nach2.MaxValues)
	}

	geaendert3 := *oben3
	geaendert3.RangeMin = "10"
	geaendert3.RangeMax = "20"
	if err := store.Update(ctx, site, oben3.ID, geaendert3); err != nil {
		t.Fatalf("Update (Bereich): %v", err)
	}
	nach3, err := store.Get(ctx, site, oben3.ID)
	if err != nil {
		t.Fatalf("Get nach Update: %v", err)
	}
	if nach3.RangeMin != "10" || nach3.RangeMax != "20" {
		t.Errorf("Update: Grenzen = %q/%q, erwartet \"10\"/\"20\"", nach3.RangeMin, nach3.RangeMax)
	}

	// --- A field that sets none of the four ---------------------------------
	// The counter-check: the migration's default values must not equip a field
	// with a property nobody set.
	schlicht, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "notiz", Label: "Notiz", Kind: KindText})
	if err != nil {
		t.Fatalf("schlichtes Feld: %v", err)
	}
	if schlicht.Display != "" || schlicht.MaxValues != 0 ||
		schlicht.RangeMin != "" || schlicht.RangeMax != "" {
		t.Errorf("a plain field carries a property nobody set: %q/%d/%q/%q",
			schlicht.Display, schlicht.MaxValues, schlicht.RangeMin, schlicht.RangeMax)
	}

	// --- The third emptying rule: the bounds belong to the range field ------
	// The regression this clause stands against: converting a range field to
	// another kind would otherwise keep two bounds nobody reads any more — and
	// which would suddenly hold again on the next conversion back.
	fremd, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "umgestellt", Label: "Umgestellt", Kind: KindText,
		RangeMin: "1", RangeMax: "9"})
	if err != nil {
		t.Fatalf("umgestelltes Feld: %v", err)
	}
	if fremd.RangeMin != "" || fremd.RangeMax != "" {
		t.Errorf("ein Textfeld behielt die Grenzen %q/%q", fremd.RangeMin, fremd.RangeMax)
	}
	// And the mistake an over-eager emptying rule makes: a range field that
	// sets only one of the two bounds keeps it.
	halb, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "nachoben", Label: "Nach oben offen", Kind: KindRange,
		RangeMin: "1"})
	if err != nil {
		t.Fatalf("halb begrenztes Feld: %v", err)
	}
	if halb.RangeMin != "1" || halb.RangeMax != "" {
		t.Errorf("halb begrenztes Bereichsfeld = %q/%q, erwartet \"1\"/\"\"",
			halb.RangeMin, halb.RangeMax)
	}
}

// TestNeueSpaltenSindGebundeneParameter is the check for T-07-05: the four new
// values travel as $n parameters and are never glued into an SQL string. A
// quotation mark and a semicolon therefore come back unchanged instead of
// taking the statement apart.
func TestNeueSpaltenSindGebundeneParameter(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()

	boshaft := `O'Brien"; DROP TABLE page_field_defs; --`
	// A range field, because validate empties the bounds on every other kind —
	// and an emptied value could not take a statement apart, so the check would
	// have no teeth left.
	d, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "grenze", Label: "Grenze", Kind: KindRange,
		RangeMin: boshaft, RangeMax: boshaft})
	if err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if d.RangeMin != boshaft || d.RangeMax != boshaft {
		t.Fatalf("the bounds came back changed: %q / %q", d.RangeMin, d.RangeMax)
	}
	// And the table is still there.
	if _, err := store.List(ctx, site); err != nil {
		t.Fatalf("List after the malicious value: %v", err)
	}
}

// TestNeueSpaltenGeprueft deckt die drei Regeln in validate ab.
func TestNeueSpaltenGeprueft(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()

	t.Run("verdrehte Grenzen werden abgelehnt", func(t *testing.T) {
		_, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "verdreht", Label: "Verdreht", Kind: KindNumber,
			RangeMin: "10", RangeMax: "2"})
		if err == nil {
			t.Fatal("a lower bound above the upper one was accepted")
		}
		// errors.Is and not just the text: the screen hangs its detailed
		// reason off exactly this sentinel, and that Create passes it through
		// unwrapped is the condition for that.
		if !errors.Is(err, ErrRangeInverted) {
			t.Errorf("the refusal does not carry ErrRangeInverted: %v", err)
		}
		if !strings.Contains(err.Error(), "bound") {
			t.Errorf("the reason does not name the bound: %v", err)
		}
	})

	t.Run("keine Zahlen, also keine Ablehnung", func(t *testing.T) {
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "worte", Label: "Worte", Kind: KindText,
			RangeMin: "b", RangeMax: "a"}); err != nil {
			t.Fatalf("two words are not an inverted pair of numbers: %v", err)
		}
	})

	t.Run("negative Höchstzahl wird abgelehnt", func(t *testing.T) {
		_, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "negativ", Label: "Negativ", Kind: KindMulti,
			Choices: []string{"a"}, MaxValues: -1})
		if err == nil {
			t.Fatal("a negative maximum was accepted")
		}
	})

	// A multi-choice with no options draws a group with nothing in it but the
	// hidden sentinel — it can never carry a value. If it is required on top of
	// that, Check reports "has to be filled in" on every save of every page, and
	// the form offers nothing that could satisfy it: the page is unsaveable
	// until somebody changes the definition.
	t.Run("Mehrfachauswahl ohne Möglichkeiten wird abgelehnt", func(t *testing.T) {
		_, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "leerauswahl", Label: "Leerauswahl", Kind: KindMulti})
		if err == nil {
			t.Fatal("a multi-choice with not a single option was accepted")
		}
		if !strings.Contains(err.Error(), "option") {
			t.Errorf("the reason does not name the options: %v", err)
		}

		// The single-valued choice has always been bound — the same rule, now
		// on both kinds.
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "leereauswahl", Label: "Leere Auswahl", Kind: KindChoice}); err == nil {
			t.Error("a choice with not a single option was accepted")
		}

		// With one option both work.
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "hoelzer", Label: "Hölzer", Kind: KindMulti,
			Choices: []string{"Eiche"}}); err != nil {
			t.Errorf("a multi-choice with one option was refused: %v", err)
		}
	})

	t.Run("artfremde Eigenschaften werden geleert statt abgelehnt", func(t *testing.T) {
		// Converting an existing field to another kind should not mean
		// clearing boxes by hand first — the same bargain the heading already
		// makes.
		d, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "umgestellt", Label: "Umgestellt", Kind: KindText,
			Display: DisplayButtons, MaxValues: 5})
		if err != nil {
			t.Fatalf("anlegen: %v", err)
		}
		if d.Display != "" {
			t.Errorf("Darstellung an einem Textfeld = %q, erwartet leer", d.Display)
		}
		if d.MaxValues != 0 {
			t.Errorf("maximum on a text field = %d, expected 0", d.MaxValues)
		}
	})
}

// TestIsButtonRow: the row of buttons is a display mode of the choice, not a
// field type of its own. The predicate lives here so that neither the template
// nor switchOf spells the comparison out again.
func TestIsButtonRow(t *testing.T) {
	cases := []struct {
		art     string
		anzeige string
		will    bool
	}{
		{KindChoice, DisplayButtons, true},
		{KindChoice, "", false},
		{KindMulti, DisplayButtons, false},
		{KindText, DisplayButtons, false},
	}
	for _, f := range cases {
		d := Def{Kind: f.art, Display: f.anzeige}
		if got := d.IsButtonRow(); got != f.will {
			t.Errorf("IsButtonRow(%q, %q) = %v, erwartet %v", f.art, f.anzeige, got, f.will)
		}
	}
}

// The field key is the name every stored value sits under and the name the
// theme addresses the field by. validate used to derive it from the label only
// when none came along — a brought key went through unchecked. The one path on
// which a brought key comes in is the archive path
// (internal/bundle/import.go:351), which is to say a file from somebody else's
// machine.
//
// The second half is the counter-check for the unified upper bound: SlugifyKey
// truncates at maxKeyBytes, validKey reads the same number. If the two drifted
// apart, validate would refuse what SlugifyKey itself produced.
func TestFeldschluesselWirdAufSeineFormGeprueft(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()

	t.Run("Klammern im Schlüssel werden abgelehnt", func(t *testing.T) {
		// Literally the shape internal/bundle/import.go:351 hands over.
		_, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "farbe[]", Label: "Farbe", Kind: KindChoice,
			Choices: []string{"rot", "blau"}})
		if err == nil {
			t.Fatal("a key with brackets was accepted")
		}
		if !strings.Contains(err.Error(), "key") {
			t.Errorf("the reason does not name the key: %v", err)
		}

		// And nothing was created.
		defs, err := store.List(ctx, site)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, d := range defs {
			if d.Key == "farbe[]" {
				t.Error("the refused key is in the database regardless")
			}
		}
	})

	t.Run("Grossbuchstaben und Punkte werden abgelehnt", func(t *testing.T) {
		for _, key := range []string{"Farbe", "farbe.ton", "farbe-ton", "farbe ton", "fär be"} {
			if _, err := store.Create(ctx, Def{
				WebsiteID: site, Key: key, Label: "Farbe", Kind: KindText}); err == nil {
				t.Errorf("the key %q was accepted", key)
			}
		}
	})

	t.Run("eine 39 Zeichen lange Kennung bleibt speicherbar", func(t *testing.T) {
		// The same label as in TestKennungAusBeschriftung: SlugifyKey makes 39
		// characters of it. Before the unification validKey refused anything
		// over 30 — the derivation would have produced something the check in
		// the same function no longer let through.
		lang := "Sehr langer Name der weit über vierzig Zeichen hinausgeht"
		d, err := store.Create(ctx, Def{
			WebsiteID: site, Label: lang, Kind: KindText})
		if err != nil {
			t.Fatalf("a 39-character key was refused: %v", err)
		}
		if d.Key != "sehr_langer_name_der_weit_ueber_vierzig" {
			t.Errorf("Kennung = %q, wollte die vollen 39 Zeichen", d.Key)
		}
		if len(d.Key) != 39 {
			t.Errorf("the key is %d characters long, wanted 39", len(d.Key))
		}
	})

	t.Run("eine Bedingung auf ein langes Feld fällt nicht mehr still weg", func(t *testing.T) {
		// validKey(d.Condition) silently deleted a condition pointing at a
		// field with a key of 31 to 40 characters — the same difference of
		// numbers, one level further on.
		schalter := "sehr_langer_name_der_weit_ueber_vierzig"
		d, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "abhaengig", Label: "Abhängig", Kind: KindText,
			Condition: schalter})
		if err != nil {
			t.Fatalf("anlegen: %v", err)
		}
		if d.Condition != schalter {
			t.Errorf("Bedingung = %q, wollte %q", d.Condition, schalter)
		}
	})
}

// TestBausteinNamensraum is the gate on the fourth namespace.
//
// What counting would not find, and why this reads instead: a SELECT can name
// the new column and still never write it into the Def — the list is right, the
// field stays zero, and nothing fails. And a WHERE clause can stand on five
// statements and be missing from the sixth — the count adds up, and every
// snippet's field stands on every page's editing form. Both show up only when
// every read path is read back.
//
// The two fields deliberately carry the same key: that a page field "telefon"
// and a snippet field "telefon" of the same website may stand side by side is
// half of the promise, and that neither appears on the other's path is the
// other half.
func TestBausteinNamensraum(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()

	res, err := store.DB.Write.ExecContext(ctx,
		`INSERT INTO snippets (website_id, key, name) VALUES ($1, 'kontakt', 'Kontakt')`, site)
	if err != nil {
		t.Fatalf("Textbaustein anlegen: %v", err)
	}
	textbaustein, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Textbaustein-Nummer: %v", err)
	}
	res, err = store.DB.Write.ExecContext(ctx,
		`INSERT INTO snippets (website_id, key, name) VALUES ($1, 'impressum', 'Impressum')`, site)
	if err != nil {
		t.Fatalf("zweiten Textbaustein anlegen: %v", err)
	}
	zweiterBaustein, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Textbaustein-Nummer: %v", err)
	}
	res, err = store.DB.Write.ExecContext(ctx,
		`INSERT INTO block_types (website_id, key, name) VALUES ($1, 'karte', 'Karte')`, site)
	if err != nil {
		t.Fatalf("Bausteinart anlegen: %v", err)
	}
	bausteinart, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Bausteinart-Nummer: %v", err)
	}

	seitenfeld, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText})
	if err != nil {
		t.Fatalf("Seitenfeld anlegen: %v", err)
	}
	bausteinfeld, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: textbaustein})
	if err != nil {
		t.Fatalf("Textbausteinfeld anlegen: %v", err)
	}

	// --- List sieht nur die Seitenfelder -------------------------------------
	oben, err := store.List(ctx, site)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(oben) != 1 {
		t.Fatalf("List gibt %d Felder heraus, erwartet 1", len(oben))
	}
	if oben[0].ID != seitenfeld.ID {
		t.Errorf("List gibt Feld %d heraus, erwartet das Seitenfeld %d — "+
			"ohne AND snippet_id IS NULL steht jedes Textbausteinfeld auf jedem Seitenformular",
			oben[0].ID, seitenfeld.ID)
	}
	if oben[0].SnippetID != 0 {
		t.Errorf("List: SnippetID = %d, erwartet 0", oben[0].SnippetID)
	}

	// --- OfSnippet sieht nur die Felder dieses Textbausteins ------------------
	amBaustein, err := store.OfSnippet(ctx, site, textbaustein)
	if err != nil {
		t.Fatalf("OfSnippet: %v", err)
	}
	if len(amBaustein) != 1 {
		t.Fatalf("OfSnippet gibt %d Felder heraus, erwartet 1", len(amBaustein))
	}
	if amBaustein[0].ID != bausteinfeld.ID {
		t.Errorf("OfSnippet hands out field %d, expected %d", amBaustein[0].ID, bausteinfeld.ID)
	}
	if amBaustein[0].SnippetID != textbaustein {
		t.Errorf("OfSnippet: SnippetID = %d, erwartet %d", amBaustein[0].SnippetID, textbaustein)
	}
	// A snippet of a foreign id gets nothing, and this website's second snippet
	// has no field yet.
	leer, err := store.OfSnippet(ctx, site, zweiterBaustein)
	if err != nil {
		t.Fatalf("OfSnippet (zweiter Baustein): %v", err)
	}
	if len(leer) != 0 {
		t.Errorf("OfSnippet of the second block hands out %d fields, expected none", len(leer))
	}

	// --- Sub, OfBlockType and OfBlockTypes see neither of the two ------------
	gruppe, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "zeiten", Label: "Öffnungszeiten", Kind: KindGroup})
	if err != nil {
		t.Fatalf("Gruppe anlegen: %v", err)
	}
	sub, err := store.Sub(ctx, site, gruppe.ID)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if len(sub) != 0 {
		t.Errorf("Sub gibt %d Felder heraus, erwartet keines", len(sub))
	}
	imBaustein, err := store.OfBlockType(ctx, site, bausteinart)
	if err != nil {
		t.Fatalf("OfBlockType: %v", err)
	}
	if len(imBaustein) != 0 {
		t.Errorf("OfBlockType gibt %d Felder heraus, erwartet keines", len(imBaustein))
	}
	alleBausteinarten, err := store.OfBlockTypes(ctx, site)
	if err != nil {
		t.Fatalf("OfBlockTypes: %v", err)
	}
	if len(alleBausteinarten) != 0 {
		t.Errorf("OfBlockTypes hands out %d block kinds, expected none", len(alleBausteinarten))
	}

	// --- Get really writes the column into the Def ---------------------------
	//
	// This is the probe counting does not replace: what is read here is what
	// scanDef wrote into the Def, and not what stands in the SELECT.
	geholt, err := store.Get(ctx, site, bausteinfeld.ID)
	if err != nil {
		t.Fatalf("Get (Textbausteinfeld): %v", err)
	}
	if geholt.SnippetID != textbaustein {
		t.Errorf("Get: SnippetID = %d, erwartet %d — ein SELECT kann die Spalte "+
			"nennen und sie trotzdem nie in den Def schreiben", geholt.SnippetID, textbaustein)
	}
	if geholt.BlockTypeID != 0 {
		t.Errorf("Get: BlockTypeID = %d, erwartet 0", geholt.BlockTypeID)
	}
	geholtSeite, err := store.Get(ctx, site, seitenfeld.ID)
	if err != nil {
		t.Fatalf("Get (Seitenfeld): %v", err)
	}
	if geholtSeite.SnippetID != 0 || geholtSeite.BlockTypeID != 0 {
		t.Errorf("Get (Seitenfeld): SnippetID = %d, BlockTypeID = %d, erwartet 0/0",
			geholtSeite.SnippetID, geholtSeite.BlockTypeID)
	}

	// --- Die beiden Teilindizes ----------------------------------------------
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: textbaustein}); !errors.Is(err, ErrDuplicateKey) {
		t.Errorf("zweites „telefon“ am selben Textbaustein: Fehler = %v, erwartet ErrDuplicateKey", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: zweiterBaustein}); err != nil {
		t.Errorf("“telefon” on the second snippet: %v — every snippet is a namespace of its own", err)
	}

	// --- Update moves no field out of its namespace --------------------------
	geaendert := *bausteinfeld
	geaendert.SnippetID = zweiterBaustein
	geaendert.Label = "Telefon direkt"
	if err := store.Update(ctx, site, bausteinfeld.ID, geaendert); err != nil {
		t.Fatalf("Update: %v", err)
	}
	nach, err := store.Get(ctx, site, bausteinfeld.ID)
	if err != nil {
		t.Fatalf("Get nach Update: %v", err)
	}
	if nach.SnippetID != textbaustein {
		t.Errorf("Update: SnippetID = %d, erwartet unverändert %d — ein Formular "+
			"darf ein Feld nicht in einen anderen Namensraum schieben", nach.SnippetID, textbaustein)
	}
	if nach.Label != "Telefon direkt" {
		t.Errorf("Update: Beschriftung = %q, erwartet \"Telefon direkt\"", nach.Label)
	}

	// --- And the page fields were left untouched by it ------------------------
	obenDanach, err := store.List(ctx, site)
	if err != nil {
		t.Fatalf("List (danach): %v", err)
	}
	if len(obenDanach) != 2 {
		t.Errorf("List then hands out %d fields, expected 2 (page field and group)", len(obenDanach))
	}
}

// newSnippet creates a snippet and returns its id.
func newSnippet(t *testing.T, store *Store, websiteID int64, key, name string) int64 {
	t.Helper()
	res, err := store.DB.Write.Exec(
		`INSERT INTO snippets (website_id, key, name) VALUES ($1, $2, $3)`, websiteID, key, name)
	if err != nil {
		t.Fatalf("Textbaustein %q anlegen: %v", key, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Textbaustein-Nummer: %v", err)
	}
	return id
}

// newBlockType creates a block kind and returns its id.
func newBlockType(t *testing.T, store *Store, websiteID int64, key, name string) int64 {
	t.Helper()
	res, err := store.DB.Write.Exec(
		`INSERT INTO block_types (website_id, key, name) VALUES ($1, $2, $3)`, websiteID, key, name)
	if err != nil {
		t.Fatalf("Bausteinart %q anlegen: %v", key, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Bausteinart-Nummer: %v", err)
	}
	return id
}

// sameSlices compares element for element and Sub for Sub.
//
// Not len and done: the bulk read and the single read must disagree neither
// about the order nor about the tree, or the public assembly shows a different
// website from the admin screen.
func sameSlices(t *testing.T, wo string, massen, einzeln []Def) {
	t.Helper()
	if len(massen) != len(einzeln) {
		t.Fatalf("%s: OfSnippets gibt %d Felder heraus, OfSnippet %d",
			wo, len(massen), len(einzeln))
	}
	for i := range massen {
		if massen[i].ID != einzeln[i].ID {
			t.Errorf("%s: Feld %d: OfSnippets = %d, OfSnippet = %d — die Reihenfolge "+
				"läuft auseinander", wo, i, massen[i].ID, einzeln[i].ID)
			continue
		}
		if massen[i].Position != einzeln[i].Position {
			t.Errorf("%s: Feld %d: Position %d gegen %d", wo, i,
				massen[i].Position, einzeln[i].Position)
		}
		if len(massen[i].Sub) != len(einzeln[i].Sub) {
			t.Errorf("%s: field %d (%q): Sub has %d against %d entries", wo, i,
				massen[i].Key, len(massen[i].Sub), len(einzeln[i].Sub))
			continue
		}
		for j := range massen[i].Sub {
			if massen[i].Sub[j].ID != einzeln[i].Sub[j].ID {
				t.Errorf("%s: Feld %d, Unterfeld %d: %d gegen %d", wo, i, j,
					massen[i].Sub[j].ID, einzeln[i].Sub[j].ID)
			}
		}
	}
}

// TestBausteinNamensraumMassenleser puts OfSnippets beside OfSnippet.
//
// The bulk reader runs on every public assembly of a page, the single reader on
// the admin screen. If they handed out different things for the same website,
// the difference would be visible nowhere but in the browser — and there only
// if somebody held both screens side by side.
func TestBausteinNamensraumMassenleser(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()

	t.Run("ohne Textbausteinfelder eine leere Karte und keine nil-Karte", func(t *testing.T) {
		leerStore, leerSite := newFieldStore(t)
		if _, err := leerStore.Create(ctx, Def{
			WebsiteID: leerSite, Key: "titel", Label: "Titel", Kind: KindText}); err != nil {
			t.Fatalf("Seitenfeld anlegen: %v", err)
		}
		alle, err := leerStore.OfSnippets(ctx, leerSite)
		if err != nil {
			t.Fatalf("OfSnippets: %v", err)
		}
		if alle == nil {
			t.Fatal("OfSnippets hands out nil, expected an empty map")
		}
		if len(alle) != 0 {
			t.Errorf("OfSnippets hands out %d entries, expected none", len(alle))
		}
	})

	kontakt := newSnippet(t, store, site, "kontakt", "Kontakt")
	impressum := newSnippet(t, store, site, "impressum", "Impressum")
	karte := newBlockType(t, store, site, "karte", "Karte")

	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "titel", Label: "Titel", Kind: KindText}); err != nil {
		t.Fatalf("Seitenfeld anlegen: %v", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "beschriftung", Label: "Beschriftung", Kind: KindText,
		BlockTypeID: karte}); err != nil {
		t.Fatalf("Bausteinartfeld anlegen: %v", err)
	}
	ersts, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: kontakt})
	if err != nil {
		t.Fatalf("erstes Textbausteinfeld: %v", err)
	}
	zweits, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "fax", Label: "Fax", Kind: KindText,
		SnippetID: kontakt})
	if err != nil {
		t.Fatalf("zweites Textbausteinfeld: %v", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "firma", Label: "Firma", Kind: KindText,
		SnippetID: impressum}); err != nil {
		t.Fatalf("Feld am zweiten Textbaustein: %v", err)
	}

	alle, err := store.OfSnippets(ctx, site)
	if err != nil {
		t.Fatalf("OfSnippets: %v", err)
	}
	if len(alle) != 2 {
		t.Fatalf("OfSnippets gibt %d Textbausteine heraus, erwartet 2", len(alle))
	}
	for _, id := range []int64{kontakt, impressum} {
		einzeln, err := store.OfSnippet(ctx, site, id)
		if err != nil {
			t.Fatalf("OfSnippet(%d): %v", id, err)
		}
		sameSlices(t, "Textbaustein", alle[id], einzeln)
	}
	// Nothing belonging to a page or to a block kind.
	for id, defs := range alle {
		for _, d := range defs {
			if d.SnippetID != id {
				t.Errorf("OfSnippets: field %q lies under %d but carries SnippetID %d",
					d.Key, id, d.SnippetID)
			}
			if d.BlockTypeID != 0 {
				t.Errorf("OfSnippets: field %q carries BlockTypeID %d", d.Key, d.BlockTypeID)
			}
		}
	}

	// --- Two fields at the same position -------------------------------------
	//
	// The tie-breaker is the id, on both read paths. Without it SQLite would
	// decide freely, and the two paths would come out in different orders on
	// some days.
	if _, err := store.DB.Write.ExecContext(ctx,
		`UPDATE page_field_defs SET position = 0 WHERE id IN ($1, $2)`,
		ersts.ID, zweits.ID); err != nil {
		t.Fatalf("Positionen gleichstellen: %v", err)
	}
	alle, err = store.OfSnippets(ctx, site)
	if err != nil {
		t.Fatalf("OfSnippets (nach Gleichstand): %v", err)
	}
	einzeln, err := store.OfSnippet(ctx, site, kontakt)
	if err != nil {
		t.Fatalf("OfSnippet (nach Gleichstand): %v", err)
	}
	sameSlices(t, "gleiche Position", alle[kontakt], einzeln)
	if len(einzeln) != 2 || einzeln[0].ID != ersts.ID {
		t.Errorf("at equal position the id breaks the tie: first field = %d, expected %d",
			einzeln[0].ID, ersts.ID)
	}
}

// TestBausteinNamensraumGruppe checks the group on a snippet.
//
// A block kind can carry none, a snippet can — and its sub-fields carry both,
// parent_id and snippet_id. The bulk reader therefore has to fold them into the
// tree and must hand out no sub-field at the top level.
func TestBausteinNamensraumGruppe(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()
	kontakt := newSnippet(t, store, site, "kontakt", "Kontakt")

	gruppe, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "zeiten", Label: "Öffnungszeiten", Kind: KindGroup,
		SnippetID: kontakt})
	if err != nil {
		t.Fatalf("Gruppe am Textbaustein: %v", err)
	}
	for _, key := range []string{"tag", "von"} {
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: key, Label: key, Kind: KindText,
			ParentID: gruppe.ID, SnippetID: kontakt}); err != nil {
			t.Fatalf("Unterfeld %q: %v", key, err)
		}
	}

	sub, err := store.Sub(ctx, site, gruppe.ID)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if len(sub) != 2 {
		t.Fatalf("Sub gibt %d Unterfelder heraus, erwartet 2 — ein AND snippet_id IS NULL "+
			"in Sub liesse jede Gruppe an einem Textbaustein leer zurückkommen", len(sub))
	}
	for _, d := range sub {
		if d.ParentID != gruppe.ID || d.SnippetID != kontakt {
			t.Errorf("Unterfeld %q: ParentID = %d, SnippetID = %d, erwartet %d / %d",
				d.Key, d.ParentID, d.SnippetID, gruppe.ID, kontakt)
		}
	}

	alle, err := store.OfSnippets(ctx, site)
	if err != nil {
		t.Fatalf("OfSnippets: %v", err)
	}
	oben := alle[kontakt]
	if len(oben) != 1 {
		t.Fatalf("OfSnippets gibt %d Felder auf der obersten Ebene heraus, erwartet 1 "+
			"(die Gruppe) — ein Unterfeld gehört nicht dorthin", len(oben))
	}
	if len(oben[0].Sub) != 2 {
		t.Errorf("the group carries %d sub-fields, expected 2", len(oben[0].Sub))
	}
	einzeln, err := store.OfSnippet(ctx, site, kontakt)
	if err != nil {
		t.Fatalf("OfSnippet: %v", err)
	}
	sameSlices(t, "Gruppe", oben, einzeln)
}

// TestBausteinNamensraumValidate keeps the two arms apart.
//
// The block-kind arm forces "not required" and narrows the field kinds; the
// snippet arm deliberately does neither. If the two had ever run into each
// other, nothing would fail — a required field would quietly stop being one.
func TestBausteinNamensraumValidate(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()
	kontakt := newSnippet(t, store, site, "kontakt", "Kontakt")
	karte := newBlockType(t, store, site, "karte", "Karte")

	// --- Pflicht bleibt Pflicht, gilt_fuer wird gestellt, Bedingung geleert ---
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "anrede", Label: "Anrede", Kind: KindChoice,
		Choices: []string{"ja", "nein"}}); err != nil {
		t.Fatalf("Steuerfeld anlegen: %v", err)
	}
	amBaustein, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: kontakt, Required: true, AppliesTo: ForPage, Condition: "anrede"})
	if err != nil {
		t.Fatalf("Textbausteinfeld anlegen: %v", err)
	}
	if !amBaustein.Required {
		t.Error("Pflicht = false, erwartet true — ein Textbaustein hat ein eigenes " +
			"Formular, auf dem sich ein Pflichtfeld mit einer Begründung zurückweisen lässt")
	}
	if amBaustein.AppliesTo != ForBoth {
		t.Errorf("applies_to = %q, expected %q — a snippet belongs to no content kind",
			amBaustein.AppliesTo, ForBoth)
	}
	if amBaustein.Condition != "" {
		t.Errorf("Bedingung = %q, erwartet leer — checkCondition läuft über die "+
			"Feldliste der Seite, eine Bedingung hier würde nie beachtet", amBaustein.Condition)
	}

	// --- The block-kind arm is unchanged --------------------------------------
	imBaustein, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "beschriftung", Label: "Beschriftung", Kind: KindText,
		BlockTypeID: karte, Required: true})
	if err != nil {
		t.Fatalf("Bausteinartfeld anlegen: %v", err)
	}
	if imBaustein.Required {
		t.Error("Bausteinartfeld: Pflicht = true, erwartet false — eine Seite darf " +
			"nicht an einem halb geschriebenen Baustein scheitern")
	}

	// --- Reference and term: on a snippet yes, in a block kind no ------------
	for _, art := range []string{KindRef, KindTerm} {
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "b_" + art, Label: art, Kind: art,
			SnippetID: kontakt}); err != nil {
			t.Errorf("%q am Textbaustein: %v — die Werte eines Textbausteins werden "+
				"auf dem Weg nach draussen aufgelöst und erstarren nicht zu HTML", art, err)
		}
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "k_" + art, Label: art, Kind: art,
			BlockTypeID: karte}); !errors.Is(err, ErrNotInBlock) {
			t.Errorf("%q in der Bausteinart: Fehler = %v, erwartet ErrNotInBlock — "+
				"der bestehende Arm bleibt unangetastet", art, err)
		}
	}
}

// TestBausteinNamensraumMove keeps a field inside its own snippet.
func TestBausteinNamensraumMove(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()
	kontakt := newSnippet(t, store, site, "kontakt", "Kontakt")

	seiteEins, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "titel", Label: "Titel", Kind: KindText})
	if err != nil {
		t.Fatalf("erstes Seitenfeld: %v", err)
	}
	seiteZwei, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "untertitel", Label: "Untertitel", Kind: KindText})
	if err != nil {
		t.Fatalf("zweites Seitenfeld: %v", err)
	}
	blockOne, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: kontakt})
	if err != nil {
		t.Fatalf("erstes Textbausteinfeld: %v", err)
	}
	blockTwo, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "fax", Label: "Fax", Kind: KindText,
		SnippetID: kontakt})
	if err != nil {
		t.Fatalf("zweites Textbausteinfeld: %v", err)
	}

	// The second snippet field one place up.
	if err := store.Move(ctx, site, blockTwo.ID, true); err != nil {
		t.Fatalf("Move (Textbausteinfeld nach oben): %v", err)
	}
	nach, err := store.OfSnippet(ctx, site, kontakt)
	if err != nil {
		t.Fatalf("OfSnippet nach Move: %v", err)
	}
	if len(nach) != 2 || nach[0].ID != blockTwo.ID || nach[1].ID != blockOne.ID {
		t.Fatalf("after the Move: %v, expected %d before %d",
			nach, blockTwo.ID, blockOne.ID)
	}
	// The page fields have not moved: without the fourth arm Move would have
	// read the page's list and rewritten its positions.
	seitlich, err := store.List(ctx, site)
	if err != nil {
		t.Fatalf("List nach Move: %v", err)
	}
	if len(seitlich) != 2 || seitlich[0].ID != seiteEins.ID || seitlich[1].ID != seiteZwei.ID {
		t.Errorf("the page fields stand differently after the Move: %v", seitlich)
	}
	if seitlich[0].Position != 0 || seitlich[1].Position != 1 {
		t.Errorf("the positions of the page fields are %d/%d, expected 0/1",
			seitlich[0].Position, seitlich[1].Position)
	}

	// The now-first snippet field upwards: nothing happens, even though page
	// fields of the same website occupy lower positions.
	if err := store.Move(ctx, site, blockTwo.ID, true); err != nil {
		t.Fatalf("Move (first field upwards): %v", err)
	}
	danach, err := store.OfSnippet(ctx, site, kontakt)
	if err != nil {
		t.Fatalf("OfSnippet after the second Move: %v", err)
	}
	if len(danach) != 2 || danach[0].ID != blockTwo.ID || danach[1].ID != blockOne.ID {
		t.Errorf("the first field of a snippet has nothing to swap with upwards: %v", danach)
	}
}

// TestBausteinNamensraumFeldvorrat is D-05, and the second-to-last assertion is
// the whole decision.
//
// That the sixty-first field of a snippet is refused would be so with a shared
// allowance too. Only that in the same breath the first field of a SECOND
// snippet, a page field and a block-kind field are accepted distinguishes the
// change from merely raising the number — and makes the sentence "no more
// fields can be added" true.
func TestBausteinNamensraumFeldvorrat(t *testing.T) {
	ctx := context.Background()

	t.Run("der Vorrat eines Textbausteins verwehrt keinem anderen Träger", func(t *testing.T) {
		store, site := newFieldStore(t)
		kontakt := newSnippet(t, store, site, "kontakt", "Kontakt")
		impressum := newSnippet(t, store, site, "impressum", "Impressum")
		karte := newBlockType(t, store, site, "karte", "Karte")

		for i := 0; i < MaxFields; i++ {
			if _, err := store.Create(ctx, Def{
				WebsiteID: site, Key: fmt.Sprintf("f%02d", i), Label: fmt.Sprintf("Feld %d", i),
				Kind: KindText, SnippetID: kontakt}); err != nil {
				t.Fatalf("Feld %d am Textbaustein: %v", i, err)
			}
		}
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "zuviel", Label: "Zu viel", Kind: KindText,
			SnippetID: kontakt}); !errors.Is(err, ErrTooMany) {
			t.Errorf("the sixty-first field of the same snippet: error = %v, expected ErrTooMany", err)
		}
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "erstes", Label: "Erstes", Kind: KindText,
			SnippetID: impressum}); err != nil {
			t.Errorf("erstes Feld des zweiten Textbausteins: %v — das ist D-05 im Ganzen: "+
				"ohne diese Zusicherung wäre die Änderung von einem Heraufsetzen der Zahl "+
				"nicht zu unterscheiden", err)
		}
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "seitenfeld", Label: "Seitenfeld", Kind: KindText}); err != nil {
			t.Errorf("page field while a snippet stands at its limit: %v", err)
		}
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "bausteinfeld", Label: "Bausteinfeld", Kind: KindText,
			BlockTypeID: karte}); err != nil {
			t.Errorf("block-kind field while a snippet stands at its limit: %v", err)
		}
	})

	t.Run("der Vorrat der Seite ist unverändert, Gruppen eingerechnet", func(t *testing.T) {
		store, site := newFieldStore(t)
		newSnippet(t, store, site, "kontakt", "Kontakt")

		// A group and its sub-fields count against the page's allowance,
		// exactly as before: 1 group + 58 sub-fields + 1 page field = 60.
		gruppe, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "zeiten", Label: "Öffnungszeiten", Kind: KindGroup})
		if err != nil {
			t.Fatalf("Gruppe: %v", err)
		}
		for i := 0; i < MaxFields-2; i++ {
			if _, err := store.Create(ctx, Def{
				WebsiteID: site, Key: fmt.Sprintf("u%02d", i), Label: fmt.Sprintf("Unterfeld %d", i),
				Kind: KindText, ParentID: gruppe.ID}); err != nil {
				t.Fatalf("Unterfeld %d: %v", i, err)
			}
		}
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "letztes", Label: "Letztes", Kind: KindText}); err != nil {
			t.Fatalf("sechzigstes Seitenfeld: %v", err)
		}
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "zuviel", Label: "Zu viel", Kind: KindText}); !errors.Is(err, ErrTooMany) {
			t.Errorf("das einundsechzigste Seitenfeld: Fehler = %v, erwartet ErrTooMany — "+
				"der Vorrat der Seite bewegt sich nicht unter einer bestehenden Website weg", err)
		}
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "nochein", Label: "Noch eins", Kind: KindText,
			ParentID: gruppe.ID}); !errors.Is(err, ErrTooMany) {
			t.Errorf("ein weiteres Unterfeld derselben Gruppe: Fehler = %v, erwartet ErrTooMany — "+
				"Unterfelder zählen gegen den Vorrat der Seite", err)
		}
	})
}

// TestBausteinGruppenNamensraum puts the snippet beside the page.
//
// 00047 drew idx_page_field_defs_kennung_textbaustein as
// ON page_field_defs(snippet_id, kennung) WHERE snippet_id IS NOT NULL —
// without "AND parent_id IS NULL". Since a group's sub-fields inherit their
// carrier from the group (08-05) they carry snippet_id, and the partial index
// caught them too: the sub-fields fell into the same namespace as the top-level
// fields. Two groups on one snippet could then not both carry a sub-field
// "tag", and a top-level field could not share its key with a sub-field — on a
// page both have been allowed since 00029.
//
// That is why the case stands here twice: once on the page, where it has always
// gone through, and once on the snippet. The two have to give the same answer,
// or the screen promises the operator something the database does not keep
// (field_list.html:20).
func TestBausteinGruppenNamensraum(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()

	// --- The page: the yardstick ---------------------------------------------
	seiteGruppe1, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "oeffnungszeiten", Label: "Öffnungszeiten", Kind: KindGroup})
	if err != nil {
		t.Fatalf("Seite, erste Gruppe: %v", err)
	}
	seiteGruppe2, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "ferien", Label: "Ferien", Kind: KindGroup})
	if err != nil {
		t.Fatalf("Seite, zweite Gruppe: %v", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "tag", Label: "Tag", Kind: KindText,
		ParentID: seiteGruppe1.ID}); err != nil {
		t.Fatalf("page, “tag” in the first group: %v", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "tag", Label: "Tag", Kind: KindText,
		ParentID: seiteGruppe2.ID}); err != nil {
		t.Fatalf("Seite, „tag“ in der zweiten Gruppe: %v — zwei Gruppen einer "+
			"Seite dürfen dieselbe Unterkennung tragen", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "tag", Label: "Tag", Kind: KindText}); err != nil {
		t.Fatalf("Seite, „tag“ auf der obersten Ebene: %v — die oberste Ebene "+
			"und eine Gruppe sind zwei Namensräume", err)
	}

	// --- Der Textbaustein: dieselben vier Schritte ----------------------------
	baustein := newSnippet(t, store, site, "kontakt", "Kontakt")

	bausteinGruppe1, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "oeffnungszeiten", Label: "Öffnungszeiten", Kind: KindGroup,
		SnippetID: baustein})
	if err != nil {
		t.Fatalf("Textbaustein, erste Gruppe: %v", err)
	}
	bausteinGruppe2, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "ferien", Label: "Ferien", Kind: KindGroup,
		SnippetID: baustein})
	if err != nil {
		t.Fatalf("Textbaustein, zweite Gruppe: %v", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "tag", Label: "Tag", Kind: KindText,
		ParentID: bausteinGruppe1.ID, SnippetID: baustein}); err != nil {
		t.Fatalf("Textbaustein, „tag“ in der ersten Gruppe: %v", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "tag", Label: "Tag", Kind: KindText,
		ParentID: bausteinGruppe2.ID, SnippetID: baustein}); err != nil {
		t.Fatalf("Textbaustein, „tag“ in der zweiten Gruppe: %v — was die Seite "+
			"trägt, muss der Textbaustein auch tragen", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "tag", Label: "Tag", Kind: KindText,
		SnippetID: baustein}); err != nil {
		t.Fatalf("Textbaustein, „tag“ auf der obersten Ebene: %v — die oberste "+
			"Ebene des Textbausteins und seine Gruppen sind zwei Namensräume", err)
	}

	// --- And its own namespace stays narrow -----------------------------------
	//
	// The index is drawn narrower and not taken away: two top-level fields of
	// the same snippet still must not carry the same key, and nor may two
	// sub-fields of the same group — the second of those has been held by
	// idx_page_field_defs_kennung_gruppe since 00029.
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "tag", Label: "Noch ein Tag", Kind: KindText,
		SnippetID: baustein}); !errors.Is(err, ErrDuplicateKey) {
		t.Errorf("zweites „tag“ auf der obersten Ebene des Textbausteins: Fehler = %v, "+
			"erwartet ErrDuplicateKey", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "tag", Label: "Noch ein Tag", Kind: KindText,
		ParentID: bausteinGruppe1.ID, SnippetID: baustein}); !errors.Is(err, ErrDuplicateKey) {
		t.Errorf("zweites „tag“ in derselben Gruppe des Textbausteins: Fehler = %v, "+
			"erwartet ErrDuplicateKey", err)
	}
}

// Create no longer takes its carrier on trust.
//
// REFERENCES snippets(id) proves the snippet exists; nothing proved it belonged
// to d.WebsiteID, and idx_page_field_defs_kennung_textbaustein is drawn on
// snippet_id alone. The database would have filed a definition across the
// website boundary without complaint. Today the admin screen guards it with
// snippetOf, and the archive path hands in an id it has just created — both
// correct, both outside the store. The comment above importSnippetFields, by
// contrast, promised a "second layer" in the store, and there was none.
// Whoever believes the comment writes the hole.
//
// block_type_id has carried the same gap since 00038; both are closed here,
// because it is the same line.
func TestCreatePruefsDenTraegerGegenDieWebsite(t *testing.T) {
	store, site := newFieldStore(t)
	ctx := context.Background()

	res, err := store.DB.Write.Exec(
		`INSERT INTO websites (name, description) VALUES ('Fremde Site', '')`)
	if err != nil {
		t.Fatalf("zweite Website: %v", err)
	}
	fremd, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Website-Nummer: %v", err)
	}

	fremderBaustein := newSnippet(t, store, fremd, "kontakt", "Kontakt")
	fremdeArt := newBlockType(t, store, fremd, "zitat", "Zitat")

	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: fremderBaustein}); err == nil {
		t.Error("a field was created on another website's snippet")
	} else if !errors.Is(err, ErrNoSnippet) {
		t.Errorf("Fehler = %v, erwartet ErrNoSnippet", err)
	}

	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "quelle", Label: "Quelle", Kind: KindText,
		BlockTypeID: fremdeArt}); err == nil {
		t.Error("a field was created on another website's block kind")
	} else if !errors.Is(err, ErrNoBlockType) {
		t.Errorf("Fehler = %v, erwartet ErrNoBlockType", err)
	}

	// And its own stay untouched: the guard must not become a bar.
	eigenerBaustein := newSnippet(t, store, site, "kontakt", "Kontakt")
	eigeneArt := newBlockType(t, store, site, "zitat", "Zitat")
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: eigenerBaustein}); err != nil {
		t.Errorf("eigener Textbaustein: %v", err)
	}
	if _, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "quelle", Label: "Quelle", Kind: KindText,
		BlockTypeID: eigeneArt}); err != nil {
		t.Errorf("eigene Bausteinart: %v", err)
	}
}

// columnLists takes page_field_defs' SELECT column lists out of store.go.
//
// The FILE is read and not the store: the danger in question is a list that
// differs from the others, and that is invisible at run time — a SELECT can
// name a column and never write it into the Def, and the other way round a
// forgotten entry silently loads a zero value.
func columnLists(t *testing.T) []string {
	t.Helper()
	quelle, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("store.go lesen: %v", err)
	}
	var out []string
	for _, roh := range regexp.MustCompile("`[^`]*`").FindAllString(string(quelle), -1) {
		inhalt := strings.TrimSpace(strings.Trim(roh, "`"))
		if !strings.HasPrefix(inhalt, "SELECT id, website_id") {
			continue
		}
		bis := strings.Index(inhalt, "FROM")
		if bis < 0 {
			t.Fatalf("SELECT ohne FROM: %q", inhalt)
		}
		out = append(out, strings.Join(strings.Fields(inhalt[:bis]), " "))
	}
	return out
}

// columnCount counts the columns of a SELECT list.
//
// The commas inside a bracket belong to a call and not to the list, which is
// why the bracket depth is carried along.
func columnCount(liste string) int {
	tiefe, n := 0, 1
	for _, r := range liste {
		switch r {
		case '(':
			tiefe++
		case ')':
			tiefe--
		case ',':
			if tiefe == 0 {
				n++
			}
		}
	}
	return n
}

// The number in the comment above scanDef is counted out rather than believed.
//
// It said "five" for four migrations while there had long been seven — and it
// is the only thing in the file that tells the next author how many places they
// have to touch, on the danger this phase itself named as its most dangerous. A
// number that stands only in a sentence goes wrong again with the next change;
// one a test counts out does not.
//
// A fifth carrier turns this test red. That is the intention: the author should
// pass the lines of scanDef on their way to changing the number.
func TestSpaltenlistenSindAbschriften(t *testing.T) {
	lists := columnLists(t)
	if len(lists) != 7 {
		t.Fatalf("%d SELECT-Spaltenlisten in store.go, der Kommentar über scanDef "+
			"nennt sieben — stimmt die Zahl nicht mehr, sind beide zu berichtigen "+
			"und scanDefs Scan-Reihenfolge nachzusehen", len(lists))
	}
	for i, l := range lists[1:] {
		if l != lists[0] {
			t.Errorf("Spaltenliste %d weicht ab:\n  %s\n  %s", i+2, lists[0], l)
		}
	}
	// And it is the order scanDef scans: eighteen columns, snippet_id last.
	// What is counted are the top-level commas — the ones in
	// COALESCE(parent_id, 0) separate no column, and counting them gives
	// twenty-one. (Which is exactly what happened when this test was first
	// written.)
	if n := columnCount(lists[0]); n != 18 {
		t.Errorf("the column list holds %d columns, scanDef scans eighteen", n)
	}
	if !strings.HasSuffix(lists[0], "COALESCE(snippet_id, 0)") {
		t.Errorf("die Spaltenliste endet auf %q, scanDef scannt SnippetID zuletzt", lists[0])
	}
}
