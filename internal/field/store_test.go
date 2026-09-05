package field

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// neuerFeldSpeicher legt eine frische Datenbank samt einer Website an.
//
// Eine Datei und keine Datenbank im Arbeitsspeicher: db.Open prüft die
// WAL-Pragmata nach dem Öffnen, und die kann eine Datenbank im Arbeitsspeicher
// nicht erfüllen.
func neuerFeldSpeicher(t *testing.T) (*Store, int64) {
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

// Zwei Prüffelder und nicht eines, weil validate eine Eigenschaft leert, die
// zur gewählten Art nicht passt: die Darstellung lebt an einer Auswahl, die
// Höchstzahl an einer Mehrfachauswahl. Kein einziges Feld kann also beide
// tragen. Zusammen belegen die beiden alle vier neuen Spalten, und darum geht
// es hier: jede der vier muss auf jedem Leseweg ankommen.
func auswahlFeld(websiteID int64, key string) Def {
	return Def{
		WebsiteID: websiteID, Key: key, Label: "Farbe", Kind: KindChoice,
		Choices: []string{"rot", "blau"},
		Display: DisplayButtons, RangeMin: "1", RangeMax: "9",
	}
}

func mehrfachFeld(websiteID int64, key string) Def {
	return Def{
		WebsiteID: websiteID, Key: key, Label: "Zutaten", Kind: KindMulti,
		Choices:   []string{"salz", "pfeffer"},
		MaxValues: 3, RangeMin: "2", RangeMax: "8",
	}
}

func pruefeAuswahl(t *testing.T, wo string, d Def) {
	t.Helper()
	if d.Display != DisplayButtons {
		t.Errorf("%s: Darstellung = %q, erwartet %q", wo, d.Display, DisplayButtons)
	}
	if d.RangeMin != "1" || d.RangeMax != "9" {
		t.Errorf("%s: Grenzen = %q/%q, erwartet \"1\"/\"9\"", wo, d.RangeMin, d.RangeMax)
	}
}

func pruefeMehrfach(t *testing.T, wo string, d Def) {
	t.Helper()
	if d.MaxValues != 3 {
		t.Errorf("%s: Höchstzahl = %d, erwartet 3", wo, d.MaxValues)
	}
	if d.RangeMin != "2" || d.RangeMax != "8" {
		t.Errorf("%s: Grenzen = %q/%q, erwartet \"2\"/\"8\"", wo, d.RangeMin, d.RangeMax)
	}
}

func finde(t *testing.T, wo string, defs []Def, key string) Def {
	t.Helper()
	for _, d := range defs {
		if d.Key == key {
			return d
		}
	}
	t.Fatalf("%s: Feld %q nicht gefunden", wo, key)
	return Def{}
}

// TestNeueSpalten ist das eigentliche Tor auf die sieben SQL-Stellen.
//
// Die Spaltenliste von page_field_defs steht sieben Mal in store.go: fünf
// SELECTs, das INSERT und das UPDATE. Wird eine davon vergessen, lädt ein Feld
// still mit einem Nullwert — eine Knopfreihe erscheint als Klappliste, eine
// Grenze wird nicht durchgesetzt, und nirgends steht ein Fehler. Eine Zählung
// der Vorkommen fände das nicht: ein SELECT kann die Spalte nennen und sie
// trotzdem nie in den Def schreiben. Deshalb wird hier gelesen, nicht gezählt.
func TestNeueSpalten(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()

	// --- Get, für ein Feld der Seite selbst ---------------------------------
	oben1, err := store.Create(ctx, auswahlFeld(site, "farbe"))
	if err != nil {
		t.Fatalf("Auswahl anlegen: %v", err)
	}
	oben2, err := store.Create(ctx, mehrfachFeld(site, "zutaten"))
	if err != nil {
		t.Fatalf("Mehrfachauswahl anlegen: %v", err)
	}
	pruefeAuswahl(t, "Get (Seitenfeld)", *oben1)
	pruefeMehrfach(t, "Get (Seitenfeld)", *oben2)

	// --- List ---------------------------------------------------------------
	top, err := store.List(ctx, site)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	pruefeAuswahl(t, "List", finde(t, "List", top, "farbe"))
	pruefeMehrfach(t, "List", finde(t, "List", top, "zutaten"))

	// --- Sub, für ein Feld in einer Gruppe ----------------------------------
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
	sub, err := store.Sub(ctx, site, gruppe.ID)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	pruefeAuswahl(t, "Sub", finde(t, "Sub", sub, "gfarbe"))
	pruefeMehrfach(t, "Sub", finde(t, "Sub", sub, "gzutaten"))

	// Und derselbe Weg noch einmal über List, die den Baum im Speicher baut.
	top, err = store.List(ctx, site)
	if err != nil {
		t.Fatalf("List (zweites Mal): %v", err)
	}
	pruefeAuswahl(t, "List/Sub", finde(t, "List/Sub", finde(t, "List", top, "zeiten").Sub, "gfarbe"))

	// --- OfBlockType und OfBlockTypes ---------------------------------------
	res, err := store.DB.Write.ExecContext(ctx,
		`INSERT INTO block_types (website_id, kennung, name) VALUES ($1, 'karte', 'Karte')`, site)
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
	derBaustein, err := store.OfBlockType(ctx, site, bausteinart)
	if err != nil {
		t.Fatalf("OfBlockType: %v", err)
	}
	pruefeAuswahl(t, "OfBlockType", finde(t, "OfBlockType", derBaustein, "bfarbe"))
	pruefeMehrfach(t, "OfBlockType", finde(t, "OfBlockType", derBaustein, "bzutaten"))

	alleBausteine, err := store.OfBlockTypes(ctx, site)
	if err != nil {
		t.Fatalf("OfBlockTypes: %v", err)
	}
	pruefeAuswahl(t, "OfBlockTypes", finde(t, "OfBlockTypes", alleBausteine[bausteinart], "bfarbe"))
	pruefeMehrfach(t, "OfBlockTypes", finde(t, "OfBlockTypes", alleBausteine[bausteinart], "bzutaten"))

	// --- Update -------------------------------------------------------------
	geaendert := *oben1
	geaendert.Display = ""
	geaendert.RangeMin = "10"
	geaendert.RangeMax = "20"
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
	if nach.RangeMin != "10" || nach.RangeMax != "20" {
		t.Errorf("Update: Grenzen = %q/%q, erwartet \"10\"/\"20\"", nach.RangeMin, nach.RangeMax)
	}

	geaendert2 := *oben2
	geaendert2.MaxValues = 7
	geaendert2.RangeMin = "30"
	geaendert2.RangeMax = "40"
	if err := store.Update(ctx, site, oben2.ID, geaendert2); err != nil {
		t.Fatalf("Update (Mehrfachauswahl): %v", err)
	}
	nach2, err := store.Get(ctx, site, oben2.ID)
	if err != nil {
		t.Fatalf("Get nach Update: %v", err)
	}
	if nach2.MaxValues != 7 {
		t.Errorf("Update: Höchstzahl = %d, erwartet 7", nach2.MaxValues)
	}
	if nach2.RangeMin != "30" || nach2.RangeMax != "40" {
		t.Errorf("Update: Grenzen = %q/%q, erwartet \"30\"/\"40\"", nach2.RangeMin, nach2.RangeMax)
	}

	// --- Ein Feld, das keine der vier setzt ---------------------------------
	// Die Gegenprobe: die Standardwerte der Wanderung dürfen kein Feld mit
	// einer Eigenschaft ausstatten, die niemand gesetzt hat.
	schlicht, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "notiz", Label: "Notiz", Kind: KindText})
	if err != nil {
		t.Fatalf("schlichtes Feld: %v", err)
	}
	if schlicht.Display != "" || schlicht.MaxValues != 0 ||
		schlicht.RangeMin != "" || schlicht.RangeMax != "" {
		t.Errorf("schlichtes Feld trägt eine Eigenschaft, die niemand setzte: %q/%d/%q/%q",
			schlicht.Display, schlicht.MaxValues, schlicht.RangeMin, schlicht.RangeMax)
	}
}

// TestNeueSpaltenSindGebundeneParameter ist die Prüfung zu T-07-05: die vier
// neuen Werte reisen als $n-Parameter und werden nie in eine SQL-Zeichenkette
// geklebt. Ein Anführungszeichen und ein Semikolon kommen deshalb unverändert
// zurück, statt die Anweisung zu zerlegen.
func TestNeueSpaltenSindGebundeneParameter(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()

	boshaft := `O'Brien"; DROP TABLE page_field_defs; --`
	d, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "grenze", Label: "Grenze", Kind: KindText,
		RangeMin: boshaft, RangeMax: boshaft})
	if err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if d.RangeMin != boshaft || d.RangeMax != boshaft {
		t.Fatalf("Grenzen kamen verändert zurück: %q / %q", d.RangeMin, d.RangeMax)
	}
	// Und die Tabelle steht noch.
	if _, err := store.List(ctx, site); err != nil {
		t.Fatalf("List nach dem boshaften Wert: %v", err)
	}
}

// TestNeueSpaltenGeprueft deckt die drei Regeln in validate ab.
func TestNeueSpaltenGeprueft(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()

	t.Run("verdrehte Grenzen werden abgelehnt", func(t *testing.T) {
		_, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "verdreht", Label: "Verdreht", Kind: KindNumber,
			RangeMin: "10", RangeMax: "2"})
		if err == nil {
			t.Fatal("eine untere Grenze über der oberen wurde angenommen")
		}
		if !strings.Contains(err.Error(), "Grenze") {
			t.Errorf("die Begründung nennt die Grenze nicht: %v", err)
		}
	})

	t.Run("keine Zahlen, also keine Ablehnung", func(t *testing.T) {
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "worte", Label: "Worte", Kind: KindText,
			RangeMin: "b", RangeMax: "a"}); err != nil {
			t.Fatalf("zwei Wörter sind kein verdrehtes Zahlenpaar: %v", err)
		}
	})

	t.Run("negative Höchstzahl wird abgelehnt", func(t *testing.T) {
		_, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "negativ", Label: "Negativ", Kind: KindMulti,
			Choices: []string{"a"}, MaxValues: -1})
		if err == nil {
			t.Fatal("eine negative Höchstzahl wurde angenommen")
		}
	})

	t.Run("artfremde Eigenschaften werden geleert statt abgelehnt", func(t *testing.T) {
		// Wer ein bestehendes Feld auf eine andere Art umstellt, soll nicht
		// erst von Hand Kästchen ausräumen müssen — dieselbe Abmachung, die
		// die Überschrift schon macht.
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
			t.Errorf("Höchstzahl an einem Textfeld = %d, erwartet 0", d.MaxValues)
		}
	})
}

// TestIsButtonRow: die Knopfreihe ist ein Anzeigemodus der Auswahl, kein
// eigener Feldtyp. Das Prädikat lebt hier, damit weder die Vorlage noch
// switchOf den Vergleich noch einmal ausschreibt.
func TestIsButtonRow(t *testing.T) {
	faelle := []struct {
		art     string
		anzeige string
		will    bool
	}{
		{KindChoice, DisplayButtons, true},
		{KindChoice, "", false},
		{KindMulti, DisplayButtons, false},
		{KindText, DisplayButtons, false},
	}
	for _, f := range faelle {
		d := Def{Kind: f.art, Display: f.anzeige}
		if got := d.IsButtonRow(); got != f.will {
			t.Errorf("IsButtonRow(%q, %q) = %v, erwartet %v", f.art, f.anzeige, got, f.will)
		}
	}
}
