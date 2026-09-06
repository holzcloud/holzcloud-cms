package field

import (
	"context"
	"errors"
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
		t.Errorf("%s: Höchstzahl = %d, erwartet 3", wo, d.MaxValues)
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
		t.Errorf("Update: Höchstzahl = %d, erwartet 7", nach2.MaxValues)
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
		t.Errorf("schlichtes Feld trägt eine Eigenschaft, die niemand setzte: %q/%d/%q/%q",
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
		// errors.Is und nicht nur der Text: der Bildschirm hängt seine
		// ausführliche Begründung an genau dieses Wächterzeichen, und dass
		// Create es unverpackt durchreicht, ist die Bedingung dafür.
		if !errors.Is(err, ErrRangeInverted) {
			t.Errorf("die Ablehnung trägt nicht ErrRangeInverted: %v", err)
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
			t.Fatal("eine Mehrfachauswahl ohne eine einzige Möglichkeit wurde angenommen")
		}
		if !strings.Contains(err.Error(), "Möglichkeit") {
			t.Errorf("die Begründung nennt die Möglichkeiten nicht: %v", err)
		}

		// Die einwertige Auswahl war schon immer gebunden — dieselbe Regel,
		// jetzt an beiden Arten.
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "leereauswahl", Label: "Leere Auswahl", Kind: KindChoice}); err == nil {
			t.Error("eine Auswahl ohne eine einzige Möglichkeit wurde angenommen")
		}

		// Mit einer Möglichkeit geht beides.
		if _, err := store.Create(ctx, Def{
			WebsiteID: site, Key: "hoelzer", Label: "Hölzer", Kind: KindMulti,
			Choices: []string{"Eiche"}}); err != nil {
			t.Errorf("eine Mehrfachauswahl mit einer Möglichkeit wurde abgelehnt: %v", err)
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
			t.Fatal("ein Schlüssel mit Klammern wurde angenommen")
		}
		if !strings.Contains(err.Error(), "Kennung") {
			t.Errorf("die Begründung nennt die Kennung nicht: %v", err)
		}

		// Und nichts wurde angelegt.
		defs, err := store.List(ctx, site)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, d := range defs {
			if d.Key == "farbe[]" {
				t.Error("der abgelehnte Schlüssel steht trotzdem in der Datenbank")
			}
		}
	})

	t.Run("Grossbuchstaben und Punkte werden abgelehnt", func(t *testing.T) {
		for _, key := range []string{"Farbe", "farbe.ton", "farbe-ton", "farbe ton", "fär be"} {
			if _, err := store.Create(ctx, Def{
				WebsiteID: site, Key: key, Label: "Farbe", Kind: KindText}); err == nil {
				t.Errorf("der Schlüssel %q wurde angenommen", key)
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
			t.Fatalf("eine 39 Zeichen lange Kennung wurde abgelehnt: %v", err)
		}
		if d.Key != "sehr_langer_name_der_weit_ueber_vierzig" {
			t.Errorf("Kennung = %q, wollte die vollen 39 Zeichen", d.Key)
		}
		if len(d.Key) != 39 {
			t.Errorf("Kennung ist %d Zeichen lang, wollte 39", len(d.Key))
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
		`INSERT INTO block_types (website_id, kennung, name) VALUES ($1, 'karte', 'Karte')`, site)
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
		t.Errorf("OfSnippet gibt Feld %d heraus, erwartet %d", amBaustein[0].ID, bausteinfeld.ID)
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
		t.Errorf("OfSnippet des zweiten Bausteins gibt %d Felder heraus, erwartet keines", len(leer))
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
		t.Errorf("OfBlockTypes gibt %d Bausteinarten heraus, erwartet keine", len(alleBausteinarten))
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
		t.Errorf("„telefon“ am zweiten Textbaustein: %v — jeder Textbaustein ist ein eigener Namensraum", err)
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
		t.Errorf("List gibt danach %d Felder heraus, erwartet 2 (Seitenfeld und Gruppe)", len(obenDanach))
	}
}
