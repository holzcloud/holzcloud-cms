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

// Drei Prüffelder und nicht eines, weil validate jede Eigenschaft leert, die
// zur gewählten Art nicht passt: die Darstellung lebt an einer Auswahl, die
// Höchstzahl an einer Mehrfachauswahl, die beiden Grenzen an einem
// Bereichsfeld. Kein einziges Feld kann alle drei tragen. Zusammen belegen die
// drei alle vier neuen Spalten, und darum geht es hier: jede der vier muss auf
// jedem Leseweg ankommen.
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

// TestNeueSpalten ist das eigentliche Tor auf die neun SQL-Stellen.
//
// Die Spaltenliste von page_field_defs steht neun Mal in store.go: sieben
// SELECTs (List, Sub, OfBlockType, OfBlockTypes, OfSnippet, OfSnippets, Get),
// das INSERT und das UPDATE. Wird eine davon vergessen, lädt ein Feld
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

	// Und derselbe Weg noch einmal über List, die den Baum im Speicher baut.
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
		t.Errorf("a plain field carries a property nobody set: %q/%d/%q/%q",
			schlicht.Display, schlicht.MaxValues, schlicht.RangeMin, schlicht.RangeMax)
	}

	// --- Die dritte Leerregel: die Grenzen gehören dem Bereichsfeld ---------
	// Der Rückfall, gegen den diese Klausel steht: wer ein Bereichsfeld auf
	// eine andere Art umstellt, behielte sonst zwei Grenzen, die niemand mehr
	// liest — und die beim nächsten Umstellen zurück plötzlich wieder gälten.
	fremd, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "umgestellt", Label: "Umgestellt", Kind: KindText,
		RangeMin: "1", RangeMax: "9"})
	if err != nil {
		t.Fatalf("umgestelltes Feld: %v", err)
	}
	if fremd.RangeMin != "" || fremd.RangeMax != "" {
		t.Errorf("ein Textfeld behielt die Grenzen %q/%q", fremd.RangeMin, fremd.RangeMax)
	}
	// Und der Fehler, den eine zu eifrige Leerregel macht: ein Bereichsfeld,
	// das nur eine der beiden Grenzen setzt, behält sie.
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

// TestNeueSpaltenSindGebundeneParameter ist die Prüfung zu T-07-05: die vier
// neuen Werte reisen als $n-Parameter und werden nie in eine SQL-Zeichenkette
// geklebt. Ein Anführungszeichen und ein Semikolon kommen deshalb unverändert
// zurück, statt die Anweisung zu zerlegen.
func TestNeueSpaltenSindGebundeneParameter(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()

	boshaft := `O'Brien"; DROP TABLE page_field_defs; --`
	// Ein Bereichsfeld, weil validate die Grenzen an jeder anderen Art leert —
	// und ein geleerter Wert könnte keine Anweisung zerlegen, die Prüfung
	// hätte also keine Zähne mehr.
	d, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "grenze", Label: "Grenze", Kind: KindRange,
		RangeMin: boshaft, RangeMax: boshaft})
	if err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if d.RangeMin != boshaft || d.RangeMax != boshaft {
		t.Fatalf("the bounds came back changed: %q / %q", d.RangeMin, d.RangeMax)
	}
	// Und die Tabelle steht noch.
	if _, err := store.List(ctx, site); err != nil {
		t.Fatalf("List after the malicious value: %v", err)
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
			t.Fatal("a lower bound above the upper one was accepted")
		}
		// errors.Is und nicht nur der Text: der Bildschirm hängt seine
		// ausführliche Begründung an genau dieses Wächterzeichen, und dass
		// Create es unverpackt durchreicht, ist die Bedingung dafür.
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

	// Eine Mehrfachauswahl ohne Möglichkeiten zeichnet eine Gruppe, in der
	// nichts steht als der versteckte Wächter — sie kann nie einen Wert
	// tragen. Ist sie zusätzlich Pflicht, meldet Check bei jedem Speichern
	// jeder Seite „muss ausgefüllt werden“, und das Formular bietet nichts an,
	// womit sich das erfüllen liesse: die Seite ist unspeicherbar, bis jemand
	// die Definition ändert.
	t.Run("Mehrfachauswahl ohne Möglichkeiten wird abgelehnt", func(t *testing.T) {
		_, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "leerauswahl", Label: "Leerauswahl", Kind: KindMulti})
		if err == nil {
			t.Fatal("a multi-choice with not a single option was accepted")
		}
		if !strings.Contains(err.Error(), "option") {
			t.Errorf("the reason does not name the options: %v", err)
		}

		// Die einwertige Auswahl war schon immer gebunden — dieselbe Regel,
		// jetzt an beiden Arten.
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "leereauswahl", Label: "Leere Auswahl", Kind: KindChoice}); err == nil {
			t.Error("a choice with not a single option was accepted")
		}

		// Mit einer Möglichkeit geht beides.
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "hoelzer", Label: "Hölzer", Kind: KindMulti,
			Choices: []string{"Eiche"}}); err != nil {
			t.Errorf("a multi-choice with one option was refused: %v", err)
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
			t.Errorf("maximum on a text field = %d, expected 0", d.MaxValues)
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

// Der Feldschlüssel ist der Name, unter dem jeder gespeicherte Wert steht und
// unter dem das Theme das Feld anspricht. validate leitete ihn bisher nur aus
// der Beschriftung ab, wenn keiner mitkam — mitgebrachte Schlüssel gingen
// ungeprüft durch. Der einzige Weg, auf dem ein mitgebrachter Schlüssel herein
// kommt, ist der Archivweg (internal/bundle/import.go:351), also eine Datei
// von einem fremden Rechner.
//
// Die zweite Hälfte ist die Gegenprobe zur vereinheitlichten Obergrenze:
// SlugifyKey schneidet bei maxKeyBytes ab, validKey liest dieselbe Zahl. Liefen
// die beiden auseinander, lehnte validate ab, was SlugifyKey selbst erzeugt hat.
func TestFeldschluesselWirdAufSeineFormGeprueft(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()

	t.Run("Klammern im Schlüssel werden abgelehnt", func(t *testing.T) {
		// Wörtlich die Gestalt, die internal/bundle/import.go:351 übergibt.
		_, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "farbe[]", Label: "Farbe", Kind: KindChoice,
			Choices: []string{"rot", "blau"}})
		if err == nil {
			t.Fatal("a key with brackets was accepted")
		}
		if !strings.Contains(err.Error(), "key") {
			t.Errorf("the reason does not name the key: %v", err)
		}

		// Und nichts wurde angelegt.
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
		// Dieselbe Beschriftung wie in TestKennungAusBeschriftung: SlugifyKey
		// macht daraus 39 Zeichen. Vor der Vereinheitlichung lehnte validKey
		// alles über 30 ab — die Ableitung hätte etwas erzeugt, das die
		// Prüfung derselben Funktion nicht mehr passiert.
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
		// validKey(d.Condition) löschte wortlos eine Bedingung, die auf ein
		// Feld mit 31 bis 40 Zeichen langer Kennung zeigte — derselbe
		// Zahlenunterschied, eine Ebene weiter.
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

// TestBausteinNamensraum ist das Tor auf den vierten Namensraum.
//
// Was eine Zählung nicht fände, und darum steht hier eine Lesung: ein SELECT
// kann die neue Spalte nennen und sie trotzdem nie in den Def schreiben — die
// Liste stimmt, das Feld bleibt null, und nichts schlägt fehl. Und eine
// WHERE-Bedingung kann an fünf Anweisungen stehen und an der sechsten fehlen —
// die Zählung geht auf, und jedes Feld eines Textbausteins steht auf dem
// Bearbeitungsformular jeder Seite. Beides fällt nur auf, wenn über jeden
// Leseweg zurückgelesen wird.
//
// Die beiden Felder tragen absichtlich dieselbe Kennung: dass ein Seitenfeld
// „telefon" und ein Textbausteinfeld „telefon" derselben Website nebeneinander
// stehen dürfen, ist die Hälfte der Zusage, und dass keines von beiden auf dem
// Weg des anderen erscheint, die andere.
func TestBausteinNamensraum(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
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
	// Ein Textbaustein einer fremden Nummer bekommt nichts, und der zweite
	// Textbaustein dieser Website hat noch kein Feld.
	leer, err := store.OfSnippet(ctx, site, zweiterBaustein)
	if err != nil {
		t.Fatalf("OfSnippet (zweiter Baustein): %v", err)
	}
	if len(leer) != 0 {
		t.Errorf("OfSnippet of the second block hands out %d fields, expected none", len(leer))
	}

	// --- Sub, OfBlockType und OfBlockTypes sehen keines von beiden ------------
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

	// --- Get schreibt die Spalte wirklich in den Def -------------------------
	//
	// Das ist die Probe, die eine Zählung nicht ersetzt: hier wird gelesen, was
	// scanDef in den Def geschrieben hat, und nicht, was im SELECT steht.
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

	// --- Update verschiebt kein Feld aus seinem Namensraum -------------------
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

	// --- Und die Seitenfelder sind davon unberührt geblieben ------------------
	obenDanach, err := store.List(ctx, site)
	if err != nil {
		t.Fatalf("List (danach): %v", err)
	}
	if len(obenDanach) != 2 {
		t.Errorf("List then hands out %d fields, expected 2 (page field and group)", len(obenDanach))
	}
}

// neuerTextbaustein legt einen Textbaustein an und gibt seine Nummer zurück.
func neuerTextbaustein(t *testing.T, store *Store, websiteID int64, key, name string) int64 {
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

// neueBausteinart legt eine Bausteinart an und gibt ihre Nummer zurück.
func neueBausteinart(t *testing.T, store *Store, websiteID int64, key, name string) int64 {
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

// gleicheScheiben vergleicht Element für Element und Sub für Sub.
//
// Nicht len und fertig: die Massenlesung und die Einzellesung dürfen sich weder
// über die Reihenfolge noch über den Baum uneins sein, sonst zeigt der
// öffentliche Aufbau eine andere Website als der Verwaltungsbildschirm.
func gleicheScheiben(t *testing.T, wo string, massen, einzeln []Def) {
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

// TestBausteinNamensraumMassenleser stellt OfSnippets neben OfSnippet.
//
// Der Massenleser läuft auf jedem öffentlichen Aufbau einer Seite, der
// Einzelleser auf dem Verwaltungsbildschirm. Gäben sie für dieselbe Website
// Verschiedenes heraus, wäre der Unterschied nirgends zu sehen ausser im
// Browser — und auch dort nur, wenn jemand beide Bildschirme nebeneinander
// hält.
func TestBausteinNamensraumMassenleser(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()

	t.Run("ohne Textbausteinfelder eine leere Karte und keine nil-Karte", func(t *testing.T) {
		leerStore, leerSite := neuerFeldSpeicher(t)
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

	kontakt := neuerTextbaustein(t, store, site, "kontakt", "Kontakt")
	impressum := neuerTextbaustein(t, store, site, "impressum", "Impressum")
	karte := neueBausteinart(t, store, site, "karte", "Karte")

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
		gleicheScheiben(t, "Textbaustein", alle[id], einzeln)
	}
	// Nichts, was einer Seite oder einer Bausteinart gehört.
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

	// --- Zwei Felder auf derselben Position ----------------------------------
	//
	// Der Gleichstandsbrecher ist die Nummer, in beiden Lesewegen. Ohne ihn
	// entschiede SQLite frei, und die zwei Wege kämen an manchen Tagen in
	// verschiedener Reihenfolge heraus.
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
	gleicheScheiben(t, "gleiche Position", alle[kontakt], einzeln)
	if len(einzeln) != 2 || einzeln[0].ID != ersts.ID {
		t.Errorf("at equal position the id breaks the tie: first field = %d, expected %d",
			einzeln[0].ID, ersts.ID)
	}
}

// TestBausteinNamensraumGruppe prüft die Gruppe an einem Textbaustein.
//
// Eine Bausteinart kann keine tragen, ein Textbaustein schon — und ihre
// Unterfelder tragen beides, parent_id und snippet_id. Der Massenleser muss sie
// deshalb zum Baum fügen und darf kein Unterfeld auf der obersten Ebene
// herausgeben.
func TestBausteinNamensraumGruppe(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()
	kontakt := neuerTextbaustein(t, store, site, "kontakt", "Kontakt")

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
	gleicheScheiben(t, "Gruppe", oben, einzeln)
}

// TestBausteinNamensraumValidate hält die zwei Arme auseinander.
//
// Der Bausteinart-Arm erzwingt „keine Pflicht" und verengt die Feldarten; der
// Textbaustein-Arm tut beides ausdrücklich nicht. Wären die beiden je
// ineinandergeraten, fiele nichts aus — ein Pflichtfeld hörte still auf, eines
// zu sein.
func TestBausteinNamensraumValidate(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()
	kontakt := neuerTextbaustein(t, store, site, "kontakt", "Kontakt")
	karte := neueBausteinart(t, store, site, "karte", "Karte")

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

	// --- Der Bausteinart-Arm ist unverändert ---------------------------------
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

	// --- Verweis und Schlagwort: am Textbaustein ja, in der Bausteinart nein --
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

// TestBausteinNamensraumMove hält ein Feld in seinem eigenen Textbaustein.
func TestBausteinNamensraumMove(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()
	kontakt := neuerTextbaustein(t, store, site, "kontakt", "Kontakt")

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
	bausteinEins, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "telefon", Label: "Telefon", Kind: KindText,
		SnippetID: kontakt})
	if err != nil {
		t.Fatalf("erstes Textbausteinfeld: %v", err)
	}
	bausteinZwei, err := store.Create(ctx, Def{
		WebsiteID: site, Key: "fax", Label: "Fax", Kind: KindText,
		SnippetID: kontakt})
	if err != nil {
		t.Fatalf("zweites Textbausteinfeld: %v", err)
	}

	// Das zweite Textbausteinfeld eine Stelle nach oben.
	if err := store.Move(ctx, site, bausteinZwei.ID, true); err != nil {
		t.Fatalf("Move (Textbausteinfeld nach oben): %v", err)
	}
	nach, err := store.OfSnippet(ctx, site, kontakt)
	if err != nil {
		t.Fatalf("OfSnippet nach Move: %v", err)
	}
	if len(nach) != 2 || nach[0].ID != bausteinZwei.ID || nach[1].ID != bausteinEins.ID {
		t.Fatalf("after the Move: %v, expected %d before %d",
			nach, bausteinZwei.ID, bausteinEins.ID)
	}
	// Die Seitenfelder haben sich nicht bewegt: ohne den vierten Arm hätte Move
	// die Liste der Seite gelesen und deren Positionen neu geschrieben.
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

	// Das nun erste Textbausteinfeld nach oben: nichts geschieht, obwohl
	// Seitenfelder derselben Website tiefere Positionen belegen.
	if err := store.Move(ctx, site, bausteinZwei.ID, true); err != nil {
		t.Fatalf("Move (first field upwards): %v", err)
	}
	danach, err := store.OfSnippet(ctx, site, kontakt)
	if err != nil {
		t.Fatalf("OfSnippet after the second Move: %v", err)
	}
	if len(danach) != 2 || danach[0].ID != bausteinZwei.ID || danach[1].ID != bausteinEins.ID {
		t.Errorf("the first field of a snippet has nothing to swap with upwards: %v", danach)
	}
}

// TestBausteinNamensraumFeldvorrat ist D-05, und die vorletzte Zusicherung ist die
// ganze Entscheidung.
//
// Dass das einundsechzigste Feld eines Textbausteins abgelehnt wird, wäre auch
// bei einem geteilten Vorrat so. Erst dass im selben Atemzug das erste Feld
// eines zweiten Textbausteins, ein Seitenfeld und ein Bausteinartfeld
// angenommen werden, unterscheidet die Änderung vom blossen Heraufsetzen der
// Zahl — und macht den Satz „mehr Felder gehen nicht" wahr.
func TestBausteinNamensraumFeldvorrat(t *testing.T) {
	ctx := context.Background()

	t.Run("der Vorrat eines Textbausteins verwehrt keinem anderen Träger", func(t *testing.T) {
		store, site := neuerFeldSpeicher(t)
		kontakt := neuerTextbaustein(t, store, site, "kontakt", "Kontakt")
		impressum := neuerTextbaustein(t, store, site, "impressum", "Impressum")
		karte := neueBausteinart(t, store, site, "karte", "Karte")

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
		store, site := neuerFeldSpeicher(t)
		neuerTextbaustein(t, store, site, "kontakt", "Kontakt")

		// Eine Gruppe und ihre Unterfelder zählen gegen den Vorrat der Seite,
		// genau wie bisher: 1 Gruppe + 58 Unterfelder + 1 Seitenfeld = 60.
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

// TestBausteinGruppenNamensraum stellt den Textbaustein neben die Seite.
//
// 00047 zog idx_page_field_defs_kennung_textbaustein als
// ON page_field_defs(snippet_id, kennung) WHERE snippet_id IS NOT NULL — ohne
// „AND parent_id IS NULL". Seit die Unterfelder einer Gruppe ihren Träger von
// der Gruppe erben (08-05), tragen sie snippet_id, und der Teilindex fasste sie
// mit ein: die Unterfelder fielen in denselben Namensraum wie die Felder der
// obersten Ebene. Zwei Gruppen an einem Textbaustein konnten dann nicht beide
// ein Unterfeld „tag" tragen, und ein Feld der obersten Ebene konnte seine
// Kennung nicht mit einem Unterfeld teilen — auf einer Seite ist beides seit
// 00029 erlaubt.
//
// Deshalb steht der Fall hier zweimal: einmal an der Seite, wo er seit jeher
// durchgeht, und einmal am Textbaustein. Die beiden müssen dieselbe Antwort
// geben, sonst sagt der Bildschirm dem Bedienenden etwas zu, was die Datenbank
// nicht hält (field_list.html:20).
func TestBausteinGruppenNamensraum(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
	ctx := context.Background()

	// --- Die Seite: der Massstab ---------------------------------------------
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
	baustein := neuerTextbaustein(t, store, site, "kontakt", "Kontakt")

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

	// --- Und der eigene Namensraum bleibt eng --------------------------------
	//
	// Der Index wird enger gezogen und nicht weggenommen: zwei Felder der
	// obersten Ebene desselben Textbausteins dürfen weiterhin nicht dieselbe
	// Kennung tragen, und zwei Unterfelder derselben Gruppe auch nicht — das
	// zweite hält seit 00029 idx_page_field_defs_kennung_gruppe.
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

// Create nimmt seinen Träger nicht mehr auf Treu und Glauben.
//
// REFERENCES snippets(id) beweist, dass es den Textbaustein gibt; nichts
// bewies, dass er zu d.WebsiteID gehört, und
// idx_page_field_defs_kennung_textbaustein ist auf snippet_id allein gezogen.
// Die Datenbank hätte eine Definition über die Websitegrenze hinweg
// klaglos abgelegt. Heute wacht der Verwaltungsbildschirm mit snippetOf davor,
// und der Archivweg reicht eine eben angelegte Nummer herein — beide richtig,
// beide ausserhalb des Speichers. Der Kommentar über importSnippetFields
// versprach dagegen eine „zweite Schicht" im Speicher, und die gab es nicht.
// Wer dem Kommentar glaubt, schreibt das Loch.
//
// Dieselbe Lücke trägt block_type_id seit 00038; beide werden hier geschlossen,
// weil es dieselbe Zeile ist.
func TestCreatePruefsDenTraegerGegenDieWebsite(t *testing.T) {
	store, site := neuerFeldSpeicher(t)
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

	fremderBaustein := neuerTextbaustein(t, store, fremd, "kontakt", "Kontakt")
	fremdeArt := neueBausteinart(t, store, fremd, "zitat", "Zitat")

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

	// Und die eigenen bleiben unangetastet: die Wache darf nicht zur Sperre
	// werden.
	eigenerBaustein := neuerTextbaustein(t, store, site, "kontakt", "Kontakt")
	eigeneArt := neueBausteinart(t, store, site, "zitat", "Zitat")
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

// spaltenlisten holt die SELECT-Spaltenlisten von page_field_defs aus store.go.
//
// Gelesen wird die Datei und nicht der Speicher: die Gefahr, um die es geht,
// ist eine Liste, die von den anderen abweicht, und die ist zur Laufzeit nicht
// zu sehen — ein SELECT kann eine Spalte nennen und sie nie in den Def
// schreiben, und andersherum lädt ein vergessener Eintrag still einen Nullwert.
func spaltenlisten(t *testing.T) []string {
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

// spaltenzahl zählt die Spalten einer SELECT-Liste.
//
// Die Kommata innerhalb einer Klammer gehören zu einem Aufruf und nicht zur
// Liste, deshalb wird die Klammertiefe mitgeführt.
func spaltenzahl(liste string) int {
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

// Die Zahl im Kommentar über scanDef wird ausgezählt statt geglaubt.
//
// Sie stand vier Wanderungen lang auf „fünf", während es längst sieben waren —
// und sie ist das Einzige in der Datei, das dem nächsten Autor sagt, wie viele
// Stellen er anzufassen hat, auf der Gefahr, die diese Phase selbst als ihre
// gefährlichste benannt hat. Eine Zahl, die nur in einem Satz steht, geht mit
// der nächsten Änderung wieder schief; eine, die ein Test auszählt, nicht.
//
// Ein fünfter Träger macht diesen Test rot. Das ist die Absicht: der Autor soll
// beim Ändern der Zahl an den Zeilen von scanDef vorbeikommen.
func TestSpaltenlistenSindAbschriften(t *testing.T) {
	listen := spaltenlisten(t)
	if len(listen) != 7 {
		t.Fatalf("%d SELECT-Spaltenlisten in store.go, der Kommentar über scanDef "+
			"nennt sieben — stimmt die Zahl nicht mehr, sind beide zu berichtigen "+
			"und scanDefs Scan-Reihenfolge nachzusehen", len(listen))
	}
	for i, l := range listen[1:] {
		if l != listen[0] {
			t.Errorf("Spaltenliste %d weicht ab:\n  %s\n  %s", i+2, listen[0], l)
		}
	}
	// Und sie ist die Reihenfolge, die scanDef scannt: achtzehn Spalten,
	// snippet_id zuletzt. Gezählt werden die Kommata der obersten Ebene — die
	// in COALESCE(parent_id, 0) trennen keine Spalte, und wer sie mitzählt,
	// bekommt einundzwanzig heraus. (Genau das ist beim ersten Schreiben
	// dieses Tests passiert.)
	if n := spaltenzahl(listen[0]); n != 18 {
		t.Errorf("the column list holds %d columns, scanDef scans eighteen", n)
	}
	if !strings.HasSuffix(listen[0], "COALESCE(snippet_id, 0)") {
		t.Errorf("die Spaltenliste endet auf %q, scanDef scannt SnippetID zuletzt", listen[0])
	}
}
