package block

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/field"
)

// markdown steht für den Renderer des Hosts. Er wird hineingereicht, damit
// dieses Paket ohne Datenbank prüfbar bleibt.
func markdown(src string) (string, error) { return "<p>" + src + "</p>", nil }

func bilder(m map[int64]Image) Lookup {
	return func(id int64) (Image, bool) {
		img, ok := m[id]
		return img, ok
	}
}

// Der Editor schickt flache Feldnamen. Was zusammengehört, erkennt der Parser
// an der Nummer — und die Nummern dürfen Lücken haben, weil ein gelöschter
// Baustein sonst alle folgenden umbenennen müsste.
func TestFormularWirdInBausteineGelesen(t *testing.T) {
	form := url.Values{
		"b0.typ":      {"text"},
		"b0.markdown": {"Guten Tag."},
		"b5.typ":      {"bild"},
		"b5.medium":   {"7"},
		"b5.alt":      {"Ein Schaf"},
		"b5.variante": {"voll"},
		"titel":       {"nicht mein Feld"},
	}

	blocks := FromForm(form)
	if len(blocks) != 2 {
		t.Fatalf("%d Bausteine, want 2: %+v", len(blocks), blocks)
	}
	if blocks[0].Type != TypeText || blocks[0].Markdown != "Guten Tag." {
		t.Errorf("erster Baustein: %+v", blocks[0])
	}
	if blocks[1].MediaID != 7 || blocks[1].Alt != "Ein Schaf" || blocks[1].Variant != "voll" {
		t.Errorf("zweiter Baustein: %+v", blocks[1])
	}
}

// Verschachtelte Einträge einer Galerie oder Kartenreihe behalten ihre
// Reihenfolge, auch wenn das Formular sie in beliebiger Ordnung liefert — eine
// Map hat keine.
func TestVerschachtelteEintraegeBehaltenDieReihenfolge(t *testing.T) {
	form := url.Values{
		"b0.typ":         {"karten"},
		"b0.e2.titel":    {"Drittens"},
		"b0.e0.titel":    {"Erstens"},
		"b0.e1.titel":    {"Zweitens"},
		"b0.e1.linkziel": {"/laden"},
	}

	blocks := FromForm(form)
	if len(blocks) != 1 || len(blocks[0].Items) != 3 {
		t.Fatalf("got %+v", blocks)
	}
	want := []string{"Erstens", "Zweitens", "Drittens"}
	for i, w := range want {
		if blocks[0].Items[i].Title != w {
			t.Errorf("Eintrag %d = %q, want %q", i, blocks[0].Items[i].Title, w)
		}
	}
	if blocks[0].Items[1].LinkURL != "/laden" {
		t.Errorf("das Linkziel ging verloren: %+v", blocks[0].Items[1])
	}
}

// Ein Knopf kann gegen eine Liste gezeichnet worden sein, die es so nicht mehr
// gibt. Die ehrliche Antwort darauf ist die Liste, wie sie jetzt ist.
func TestUnsinnigeAktionenAendernNichts(t *testing.T) {
	start := []Block{{Type: TypeText, Markdown: "eins"}, {Type: TypeText, Markdown: "zwei"}}
	for _, aktion := range []string{"hoch:0", "runter:1", "weg:9", "hoch:abc", "neu:gibtsnicht", ""} {
		got := Apply(append([]Block(nil), start...), aktion, Builtin)
		if len(got) != 2 || got[0].Markdown != "eins" || got[1].Markdown != "zwei" {
			t.Errorf("%q hat die Liste verändert: %+v", aktion, got)
		}
	}
}

func TestVerschiebenUndLoeschen(t *testing.T) {
	start := []Block{
		{Type: TypeText, Markdown: "eins"},
		{Type: TypeText, Markdown: "zwei"},
		{Type: TypeText, Markdown: "drei"},
	}

	got := Apply(append([]Block(nil), start...), "hoch:2", Builtin)
	if got[1].Markdown != "drei" || got[2].Markdown != "zwei" {
		t.Errorf("hoch:2 = %+v", got)
	}

	got = Apply(append([]Block(nil), start...), "runter:0", Builtin)
	if got[0].Markdown != "zwei" || got[1].Markdown != "eins" {
		t.Errorf("runter:0 = %+v", got)
	}

	got = Apply(append([]Block(nil), start...), "weg:1", Builtin)
	if len(got) != 2 || got[0].Markdown != "eins" || got[1].Markdown != "drei" {
		t.Errorf("weg:1 = %+v", got)
	}
}

// Eine Galerie oder Kartenreihe beginnt mit einem Eintrag: sonst fügt jemand
// den Baustein ein und findet nichts, was er ausfüllen könnte.
func TestNeueGalerieHatEinenEintrag(t *testing.T) {
	got := Apply(nil, "neu:galerie", Builtin)
	if len(got) != 1 || len(got[0].Items) != 1 {
		t.Fatalf("got %+v", got)
	}
	got = Apply(got, "neu-e:0", Builtin)
	if len(got[0].Items) != 2 {
		t.Errorf("ein Eintrag kam nicht dazu: %+v", got[0])
	}
	got = Apply(got, "weg-e:0:0", Builtin)
	if len(got[0].Items) != 1 {
		t.Errorf("der Eintrag ging nicht weg: %+v", got[0])
	}
}

// Ein Baustein, den jemand hinzugefügt und dann in Ruhe gelassen hat, soll
// nicht als leerer Kasten auf der Website landen.
func TestLeereBausteineFallenBeimSichernWeg(t *testing.T) {
	blocks := []Block{
		{Type: TypeText, Markdown: "  "},
		{Type: TypeText, Markdown: "Bleibt."},
		{Type: TypeImage},
		{Type: TypeDivider},
		{Type: "gibtsnicht", Markdown: "x"},
	}
	got := Builtin.Clean(blocks)
	if len(got) != 2 || got[0].Markdown != "Bleibt." || got[1].Type != TypeDivider {
		t.Errorf("got %+v", got)
	}
}

// Was ein Redakteur tippt, ist Text und nie Markup. Der Rahmen darum ist
// unserer — deshalb darf er Klassen tragen.
func TestTextWirdMaskiertUndDerRahmenNicht(t *testing.T) {
	html := Render([]Block{{
		Type: TypeQuote, Text: `<script>alert(1)</script>`, Source: `Eva & Co`,
	}}, Builtin, bilder(nil), markdown)

	if strings.Contains(html, "<script") {
		t.Errorf("das Skript kam durch:\n%s", html)
	}
	if !strings.Contains(html, "Eva &amp; Co") {
		t.Errorf("die Quelle wurde nicht maskiert:\n%s", html)
	}
	if !strings.Contains(html, `class="hc-block hc-zitat"`) {
		t.Errorf("der Rahmen fehlt:\n%s", html)
	}
}

// Ein Redakteur, der etwas Merkwürdiges einfügt, soll eine Karte ohne Link
// bekommen — keine Seite, die es ausführt.
func TestNurBrauchbareLinkzieleUeberleben(t *testing.T) {
	for _, boese := range []string{
		"javascript:alert(1)", "//fremde.example/laden", "data:text/html,<script>",
	} {
		html := Render([]Block{{
			Type: TypeCards, Items: []Item{{Title: "Wolle", LinkURL: boese}},
		}}, Builtin, bilder(nil), markdown)
		if strings.Contains(html, "href=") {
			t.Errorf("%q wurde zu einem Link:\n%s", boese, html)
		}
		if !strings.Contains(html, "Wolle") {
			t.Errorf("mit dem Link ging auch die Karte verloren:\n%s", html)
		}
	}
	for _, gut := range []string{"/laden", "https://beispiel.ch", "mailto:eva@beispiel.ch", "#unten"} {
		html := Render([]Block{{
			Type: TypeCards, Items: []Item{{Title: "Wolle", LinkURL: gut}},
		}}, Builtin, bilder(nil), markdown)
		if !strings.Contains(html, `href="`+gut+`"`) {
			t.Errorf("%q wurde verworfen:\n%s", gut, html)
		}
	}
}

// Ein Bild, das aus der Mediathek gelöscht wurde, kostet seinen eigenen
// Baustein — nie den Artikel darum herum.
func TestFehlendesBildKostetNurSeinenBaustein(t *testing.T) {
	html := Render([]Block{
		{Type: TypeText, Markdown: "Vorher."},
		{Type: TypeImage, MediaID: 999},
		{Type: TypeText, Markdown: "Nachher."},
	}, Builtin, bilder(nil), markdown)

	if !strings.Contains(html, "Vorher.") || !strings.Contains(html, "Nachher.") {
		t.Errorf("der Text um das fehlende Bild ist weg:\n%s", html)
	}
	if strings.Contains(html, "<img") {
		t.Errorf("es wurde ein Bild ausgegeben:\n%s", html)
	}
}

// Die Beschreibung aus der Mediathek gilt, solange der Baustein keine eigene
// hat — sonst müsste sie an jeder Stelle neu getippt werden.
func TestBildbeschreibungFaelltAufDieMediathekZurueck(t *testing.T) {
	look := bilder(map[int64]Image{
		1: {URL: "/media/1/schaf.jpg", Alt: "Ein Schaf auf der Weide", Width: 800, Height: 600},
	})

	html := Render([]Block{{Type: TypeImage, MediaID: 1}}, Builtin, look, markdown)
	if !strings.Contains(html, `alt="Ein Schaf auf der Weide"`) {
		t.Errorf("die Beschreibung der Mediathek fehlt:\n%s", html)
	}
	if !strings.Contains(html, `width="800" height="600"`) {
		t.Errorf("die Masse fehlen, die Seite springt beim Laden:\n%s", html)
	}

	html = Render([]Block{{Type: TypeImage, MediaID: 1, Alt: "Hier: die Blesse"}}, Builtin, look, markdown)
	if !strings.Contains(html, `alt="Hier: die Blesse"`) {
		t.Errorf("die eigene Beschreibung wurde nicht genommen:\n%s", html)
	}
}

// Der Auszug, die Suche und die Kurzfassung lasen bisher die Markdown-Spalte.
// Eine Seite aus Bausteinen hätte dort nichts stehen und wäre für die eigene
// Suche unsichtbar.
func TestReinerTextSammeltAlleWorte(t *testing.T) {
	text := PlainText([]Block{
		{Type: TypeText, Markdown: "Wir haben Wolle."},
		{Type: TypeCards, Items: []Item{
			{Title: "Rohwolle", Markdown: "Ungewaschen."},
			{Title: "Gekardet", Markdown: "Zum Spinnen."},
		}},
		{Type: TypeQuote, Text: "Schöne Tiere.", Source: "Eine Kundin"},
	}, Builtin)
	for _, wort := range []string{"Wolle", "Rohwolle", "Ungewaschen", "Schöne Tiere", "Eine Kundin"} {
		if !strings.Contains(text, wort) {
			t.Errorf("%q fehlt im reinen Text:\n%s", wort, text)
		}
	}
	if strings.Contains(text, "<") {
		t.Errorf("es steht Markup im reinen Text:\n%s", text)
	}
}

// Hin und zurück durch die Datenbank darf nichts verändern.
func TestKodierenUndLesenIstVerlustfrei(t *testing.T) {
	blocks := []Block{
		{Type: TypeImageText, MediaID: 3, Alt: "Der Hof", Markdown: "Text daneben.", Variant: "rechts"},
		{Type: TypeGallery, Variant: "4", Display: DisplaySlideshow,
			Items: []Item{{MediaID: 1}, {MediaID: 2, Caption: "Im Mai"}}},
	}
	raw, err := Encode(blocks, Builtin)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(raw, Builtin)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 2 || got[0].Variant != "rechts" || got[1].Columns() != 4 ||
		len(got[1].Items) != 2 || got[1].Items[1].Caption != "Im Mai" ||
		got[1].Display != DisplaySlideshow {
		t.Errorf("got %+v", got)
	}
}

// A block that made no display choice must encode to JSON without the key at
// all. The field is omitempty for exactly this: every page saved before this
// field existed keeps the shape it has in the blocks column, and a diff of two
// stored pages does not sprout a line nobody asked for.
func TestAnEmptyDisplayIsNotWrittenToTheJSON(t *testing.T) {
	raw, err := Encode([]Block{
		{Type: TypeGallery, Items: []Item{{MediaID: 1}}},
	}, Builtin)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(raw, "darstellung") {
		t.Errorf("the key is in the JSON of a block that made no choice:\n%s", raw)
	}

	raw, err = Encode([]Block{
		{Type: TypeGallery, Display: DisplaySlideshow, Items: []Item{{MediaID: 1}}},
	}, Builtin)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(raw, `"darstellung":"`+DisplaySlideshow+`"`) {
		t.Errorf("the chosen display is not in the JSON:\n%s", raw)
	}
}

// The value arrives from a form, and a form is a text field anybody can post.
// A display outside the closed vocabulary is not stored at all — the block
// keeps what it had. .planning/GLOSSARY.md's closing section records the run
// time failure that follows when the stored vocabulary and the code disagree.
func TestSetBlockFieldIgnoresADisplayOutsideTheVocabulary(t *testing.T) {
	b := Block{Type: TypeGallery}
	setBlockField(&b, "darstellung", []string{DisplaySlideshow})
	if b.Display != DisplaySlideshow {
		t.Fatalf("the constant was not stored: %q", b.Display)
	}
	setBlockField(&b, "darstellung", []string{"karussell"})
	if b.Display != DisplaySlideshow {
		t.Errorf("an unknown value was stored: %q", b.Display)
	}
	setBlockField(&b, "darstellung", []string{""})
	if b.Display != "" {
		t.Errorf("the grid is the empty value and must be storable: %q", b.Display)
	}
}

// Variant is the gallery's column count and the display is a second axis. The
// whole reason the display got a field of its own is that one string cannot
// mean both, so the two are proved independent rather than assumed so.
func TestTheDisplayDoesNotChangeTheColumnCount(t *testing.T) {
	for _, variant := range []string{"2", "", "3", "4"} {
		grid := Block{Type: TypeGallery, Variant: variant}
		show := Block{Type: TypeGallery, Variant: variant, Display: DisplaySlideshow}
		if grid.Columns() != show.Columns() {
			t.Errorf("variant %q: %d columns as a grid, %d as a slideshow",
				variant, grid.Columns(), show.Columns())
		}
	}
	if got := (Block{Type: TypeGallery, Variant: DisplaySlideshow}).Columns(); got != 3 {
		t.Errorf("the display value put in Variant must stay an unknown column count, got %d", got)
	}
}

// The modifier class is minted from the constant and never concatenated from
// the stored string, so nothing a hand-edited archive carries reaches the
// class attribute of the wrapper (T-11-18).
func TestDisplayClassIsMintedAndNotConcatenated(t *testing.T) {
	if got := (Block{Type: TypeGallery}).DisplayClass(); got != "" {
		t.Errorf("the grid must add no modifier, got %q", got)
	}
	if got := (Block{Type: TypeGallery, Display: DisplaySlideshow}).DisplayClass(); got == "" {
		t.Error("the slideshow must add a modifier")
	}
	if got := (Block{Type: TypeGallery, Display: `x" onload="alert(1)`}).DisplayClass(); got != "" {
		t.Errorf("a value outside the vocabulary must add no modifier, got %q", got)
	}
}

// Keine Bausteine ist ein Wert und nicht zwei, die sich gleich verhalten, bis
// sie jemand vergleicht.
func TestKeineBausteineIstDieLeereZeichenkette(t *testing.T) {
	raw, err := Encode(nil, Builtin)
	if err != nil || raw != "" {
		t.Errorf("Encode(nil, Builtin) = %q, %v", raw, err)
	}
	raw, err = Encode([]Block{{Type: TypeText, Markdown: "   "}}, Builtin)
	if err != nil || raw != "" {
		t.Errorf("Encode(leerer Baustein, Builtin) = %q, %v", raw, err)
	}
	got, err := Decode("", Builtin)
	if err != nil || got != nil {
		t.Errorf("Decode(\"\", Builtin) = %+v, %v", got, err)
	}
}

// Der Weg zurück in den einfachen Editor steht offen, solange nichts verloren
// ginge — und ist zu, sobald etwas verloren ginge.
func TestZurueckZuMarkdownNurWennNichtsVerlorenGeht(t *testing.T) {
	md, ok := ToMarkdown([]Block{
		{Type: TypeText, Markdown: "Erster Absatz."},
		{Type: TypeText, Markdown: "Zweiter Absatz."},
	})
	if !ok || !strings.Contains(md, "Erster") || !strings.Contains(md, "Zweiter") {
		t.Errorf("got %q, %v", md, ok)
	}
	if _, ok := ToMarkdown([]Block{{Type: TypeText}, {Type: TypeImage, MediaID: 1}}); ok {
		t.Error("ein Bild würde verloren gehen, der Weg zurück darf nicht offen sein")
	}
}

// Wo ein Baustein ein Bild in eine feste Form presst, entscheidet der
// Fokuspunkt, was übrig bleibt. Ohne ihn schneidet der Browser stur aus der
// Mitte — bei einem Tier am linken Bildrand jedes Mal daneben.
func TestFokusPunktWirkNurWoZugeschnittenWird(t *testing.T) {
	look := bilder(map[int64]Image{
		1: {URL: "/media/1/schaf.jpg", Alt: "Ein Schaf", Focus: "20% 40%"},
	})

	// Eine Galeriekachel wird in ein festes Seitenverhältnis gepresst.
	html := Render([]Block{{Type: TypeGallery, Items: []Item{{MediaID: 1}}}}, Builtin, look, markdown)
	if !strings.Contains(html, `object-position:20% 40%`) {
		t.Errorf("der Fokus fehlt in der Galerie:\n%s", html)
	}

	// Ein einzelnes Bild behält seine eigene Form; dort gibt es nichts zu
	// verschieben, und ein Attribut, das nichts tut, gehört nicht auf die Seite.
	html = Render([]Block{{Type: TypeImage, MediaID: 1}}, Builtin, look, markdown)
	if strings.Contains(html, "object-position") {
		t.Errorf("der Fokus steht an einer Stelle, an der er nichts bewirkt:\n%s", html)
	}
}

// Ein Video ist eine eigene Datei dieser Website in einem <video>, kein
// eingebetteter Rahmen von einem fremden Server.
func TestVideoBaustein(t *testing.T) {
	look := func(id int64) (Image, bool) {
		switch id {
		case 1:
			return Image{URL: "/media/1/film.mp4", Film: true}, true
		case 2:
			return Image{URL: "/media/1/standbild.jpg"}, true
		}
		return Image{}, false
	}
	html := Render([]Block{{
		Type: TypeVideo, MediaID: 1, PosterID: 2, Variant: "breit",
		Caption: "Die Schafe im Frühling",
	}}, Builtin, look, markdown)

	for _, want := range []string{
		`<video controls playsinline preload="metadata"`,
		`poster="/media/1/standbild.jpg"`,
		`<source src="/media/1/film.mp4" type="video/mp4">`,
		`hc-video--breit`,
		`<figcaption>Die Schafe im Frühling</figcaption>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("%q fehlt in:\n%s", want, html)
		}
	}
	if strings.Contains(html, "<iframe") || strings.Contains(html, "autoplay") {
		t.Errorf("das Video wird eingebettet oder spielt von allein:\n%s", html)
	}
}

// Ein Bildbaustein, der auf einen Film zeigt, ergibt kein kaputtes <img> —
// und ein Videobaustein mit einem Foto darin kein <video> ohne Film.
func TestVerwechselteDateiartFaelltWeg(t *testing.T) {
	look := func(id int64) (Image, bool) {
		if id == 1 {
			return Image{URL: "/media/1/film.mp4", Film: true}, true
		}
		return Image{URL: "/media/1/bild.jpg"}, true
	}
	if html := Render([]Block{{Type: TypeImage, MediaID: 1}}, Builtin, look, markdown); html != "" {
		t.Errorf("ein Film im Bildbaustein wurde gezeichnet: %s", html)
	}
	if html := Render([]Block{{Type: TypeVideo, MediaID: 2}}, Builtin, look, markdown); html != "" {
		t.Errorf("ein Bild im Videobaustein wurde gezeichnet: %s", html)
	}
}

// --- eigene Bausteinarten ----------------------------------------------------

// eigeneArt ist ein "Rezeptschritt": eine Zeile, ein Absatz, ein Bild, ein
// Häkchen.
func eigeneArt() Set {
	return Set{Own: []Own{{
		ID: 1, Key: "rezeptschritt", Name: "Rezeptschritt",
		Fields: []field.Def{
			{Key: "nummer", Label: "Nummer", Kind: field.KindText},
			{Key: "anleitung", Label: "Anleitung", Kind: field.KindLong},
			{Key: "bild", Label: "Bild", Kind: field.KindImage},
			{Key: "wichtig", Label: "Hervorheben", Kind: field.KindBool},
			{Key: "quelle", Label: "Zum Rezept", Kind: field.KindLink},
		},
	}}}
}

// Eine eigene Art wird zu Auszeichnung, die das Theme ansprechen kann: eine
// Klasse für die Art, eine je Feld.
func TestEigeneArtWirdZuKlassen(t *testing.T) {
	set := eigeneArt()
	html := Render([]Block{{
		Type: "rezeptschritt",
		Fields: map[string]string{
			"nummer":    "3",
			"anleitung": "Den Teig **kneten**.",
			"bild":      "1",
			"wichtig":   "1",
			"quelle":    "/rezepte/brot",
		},
	}}, set, bilder(map[int64]Image{1: {URL: "/media/1/teig.jpg", Alt: "Teig"}}), markdown)

	for _, teil := range []string{
		`hc-eigen--rezeptschritt`,
		`hc-ja--wichtig`,
		`hc-eigen__zeile--nummer`,
		`hc-eigen__text--anleitung`,
		`hc-eigen__bild--bild`,
		`Den Teig **kneten**.`,
		`href="/rezepte/brot"`,
		`Zum Rezept`,
	} {
		if !strings.Contains(html, teil) {
			t.Errorf("%q fehlt in der Ausgabe:\n%s", teil, html)
		}
	}
}

// Der Rahmen ist unserer, der Inhalt nicht: was jemand tippt, wird maskiert.
func TestEigeneArtMaskiertDenInhalt(t *testing.T) {
	html := Render([]Block{{
		Type:   "rezeptschritt",
		Fields: map[string]string{"nummer": `<img src=x onerror=alert(1)>`, "quelle": "javascript:alert(1)"},
	}}, eigeneArt(), bilder(nil), markdown)

	if strings.Contains(html, "<img") {
		t.Errorf("das Bild kam durch:\n%s", html)
	}
	if !strings.Contains(html, "&lt;img") {
		t.Errorf("es wurde nicht maskiert:\n%s", html)
	}
	if strings.Contains(html, "javascript:") {
		t.Errorf("die Adresse kam durch:\n%s", html)
	}
}

// Eine Art, die diese Website nicht hat, ist kein Baustein — sonst könnte eine
// von Hand geschriebene Zeile in der Datenbank einen Typ erfinden.
func TestUnbekannteArtVerschwindet(t *testing.T) {
	blocks := Builtin.Clean([]Block{
		{Type: "rezeptschritt", Fields: map[string]string{"nummer": "3"}},
		{Type: TypeText, Markdown: "Bleibt."},
	})
	if len(blocks) != 1 || blocks[0].Type != TypeText {
		t.Errorf("die fremde Art blieb stehen: %+v", blocks)
	}
	// Mit der Art dagegen bleibt sie.
	if blocks := eigeneArt().Clean([]Block{
		{Type: "rezeptschritt", Fields: map[string]string{"nummer": "3"}},
	}); len(blocks) != 1 {
		t.Errorf("die eigene Art verschwand: %+v", blocks)
	}
}

// Ein Wert, dessen Feld aus der Art entfernt wurde, geht mit ihm — beim
// nächsten Speichern, nicht sofort.
func TestWertOhneFeldWirdAufgeraeumt(t *testing.T) {
	blocks := eigeneArt().Clean([]Block{{
		Type:   "rezeptschritt",
		Fields: map[string]string{"nummer": "3", "gabsmalgibtsnichtmehr": "Rest"},
	}})
	if len(blocks) != 1 {
		t.Fatalf("der Baustein verschwand: %+v", blocks)
	}
	if _, da := blocks[0].Fields["gabsmalgibtsnichtmehr"]; da {
		t.Errorf("der verwaiste Wert blieb: %+v", blocks[0].Fields)
	}
	if blocks[0].Fields["nummer"] != "3" {
		t.Errorf("der gültige Wert ging verloren: %+v", blocks[0].Fields)
	}
}

// Ein Baustein, in dem nichts steht, ist keiner: sonst hinterlässt jeder
// Fehlklick im Menü einen leeren Kasten auf der Seite.
func TestLeereEigeneArtVerschwindet(t *testing.T) {
	if blocks := eigeneArt().Clean([]Block{
		{Type: "rezeptschritt", Fields: map[string]string{"nummer": "   "}},
	}); len(blocks) != 0 {
		t.Errorf("der leere Baustein blieb: %+v", blocks)
	}
}

// Nur die Felder mit Worten landen im Suchtext. Eine Bildnummer im Anriss
// eines Rezepts wäre schlimmer als gar keiner.
func TestNurWorteImReinenText(t *testing.T) {
	text := PlainText([]Block{{
		Type: "rezeptschritt",
		Fields: map[string]string{
			"nummer": "Schritt drei", "anleitung": "Kneten.", "bild": "42", "quelle": "/rezepte/brot",
		},
	}}, eigeneArt())

	if !strings.Contains(text, "Schritt drei") || !strings.Contains(text, "Kneten.") {
		t.Errorf("die Worte fehlen:\n%s", text)
	}
	if strings.Contains(text, "42") || strings.Contains(text, "/rezepte") {
		t.Errorf("was keine Worte sind, steht im Text:\n%s", text)
	}
}

// Die Felder kommen unter einem eigenen Vorzeichen aus dem Formular, damit eine
// Art ein Feld "text" oder "typ" haben darf, ohne dem Baustein selbst ins
// Gehege zu kommen.
func TestEigeneFelderAusDemFormular(t *testing.T) {
	blocks := FromForm(map[string][]string{
		"b0.typ":      {"rezeptschritt"},
		"b0.f.nummer": {"3"},
		"b0.f.typ":    {"Vorspeise"},
		"b0.f.text":   {"Etwas Text."},
		"b0.markdown": {"gehört dem Baustein"},
	})
	if len(blocks) != 1 {
		t.Fatalf("aus dem Formular kam: %+v", blocks)
	}
	b := blocks[0]
	if b.Type != "rezeptschritt" {
		t.Errorf("die Art ist %q", b.Type)
	}
	if b.Fields["typ"] != "Vorspeise" || b.Fields["text"] != "Etwas Text." || b.Fields["nummer"] != "3" {
		t.Errorf("die eigenen Felder kamen falsch an: %+v", b.Fields)
	}
	if b.Markdown != "gehört dem Baustein" {
		t.Errorf("das eingebaute Feld wurde überschrieben: %q", b.Markdown)
	}
}

// Das Menü bietet die eingebauten zuerst und die eigenen dahinter.
func TestMenuStelltEingebauteVoran(t *testing.T) {
	menu := eigeneArt().Menu()
	if len(menu) != len(Kinds)+1 {
		t.Fatalf("das Menü hat %d Einträge", len(menu))
	}
	if menu[0].Type != TypeText {
		t.Errorf("vorn steht %q", menu[0].Type)
	}
	if menu[len(menu)-1].Type != "rezeptschritt" {
		t.Errorf("hinten steht %q", menu[len(menu)-1].Type)
	}
}

// artMitCode ist eine eigene Bausteinart mit einem Codefeld und einer
// Mehrfachauswahl — die beiden Arten, um die es in dieser Datei geht.
func artMitCode() Set {
	return Set{Own: []Own{{
		ID: 2, Key: "hinweis", Name: "Hinweis",
		Fields: []field.Def{
			{Key: "schnipsel", Label: "Schnipsel", Kind: field.KindCode},
			{Key: "sorten", Label: "Sorten", Kind: field.KindMulti,
				Choices: []string{"Eiche", "Buche"}},
		},
	}}}
}

// Der schärfste Satz dieser Phase: was in ein Codefeld getippt wird, erscheint
// wörtlich und wird nicht ausgeführt — auch in einem Baustein.
//
// „Auch in einem Baustein" ist die ganze Schwierigkeit. Ein Baustein wird beim
// Speichern der Seite zu HTML eingefroren, und dieses HTML bekommt der
// Besucher. Im Theme zu maskieren wäre zu spät: dann stehen die Bytes längst
// in der Datenbank.
func TestCodeImBausteinWirdMaskiert(t *testing.T) {
	roh := `<script>alert("x" & 1)</script>`
	html := Render([]Block{{
		Type:   "hinweis",
		Fields: map[string]string{"schnipsel": roh},
	}}, artMitCode(), bilder(nil), markdown)

	if strings.Contains(html, "<script") {
		t.Errorf("das Skript kam durch:\n%s", html)
	}
	for _, will := range []string{"&lt;script&gt;", "&amp;", "&#34;"} {
		if !strings.Contains(html, will) {
			t.Errorf("%q fehlt — es wurde nicht maskiert:\n%s", will, html)
		}
	}
	// Ein eigenes Element und nicht die Zeile, die jede unbekannte Art
	// bekommt: ein Codefeld ist vorformatiert, und das Theme muss es
	// ansprechen können.
	for _, will := range []string{"<pre", "<code", "hc-eigen__code--schnipsel"} {
		if !strings.Contains(html, will) {
			t.Errorf("%q fehlt in der Ausgabe:\n%s", will, html)
		}
	}
	// Und niemals durch den Markdown-Renderer: der Prüfdoppelgänger oben legt
	// um alles ein <p>, ein <p> hier wäre also der Beweis, dass der Wert den
	// Markdown-Weg genommen hat. Der Baustein trägt nur dieses eine Feld, ein
	// <p> kann also von nirgendwo sonst kommen.
	// „<p" allein wäre zu grob — das trifft auch das <pre>, das hier stehen
	// soll.
	for _, absatz := range []string{"<p>", "<p "} {
		if strings.Contains(html, absatz) {
			t.Errorf("der Code lief durch den Markdown-Renderer:\n%s", html)
		}
	}
}

// Ein leeres Codefeld hinterlässt keinen leeren Kasten auf der Seite.
func TestCodeImBausteinLeerErgibtNichts(t *testing.T) {
	html := Render([]Block{{
		Type:   "hinweis",
		Fields: map[string]string{"schnipsel": "   "},
	}}, artMitCode(), bilder(nil), markdown)
	if html != "" {
		t.Errorf("ein leeres Codefeld ergab Auszeichnung:\n%s", html)
	}
	// Auch neben einem gefüllten Feld: kein Element für das leere.
	html = Render([]Block{{
		Type:   "hinweis",
		Fields: map[string]string{"schnipsel": "", "sorten": "Eiche"},
	}}, artMitCode(), bilder(nil), markdown)
	if strings.Contains(html, "hc-eigen__code") {
		t.Errorf("das leere Codefeld bekam trotzdem ein Element:\n%s", html)
	}
}

// Was die Suche der Website sieht. Die Liste ist eine Entscheidung und keine
// Aufzählung: ein Codefeld hält Worte — eine Adresse, eine Zeile Einstellung —
// und eine Seite aus Bausteinen wäre für ihre eigene Suche sonst gerade dort
// unsichtbar, wo der Verfasser sich am meisten Mühe gab.
func TestPlainTextNimmtCodeUndMehrfachauswahl(t *testing.T) {
	text := PlainText([]Block{{
		Type: "hinweis",
		Fields: map[string]string{
			"schnipsel": "Musterweg 3, 3000 Bern",
			"sorten":    "Eiche\nBuche",
		},
	}}, artMitCode())

	if !strings.Contains(text, "Musterweg 3, 3000 Bern") {
		t.Errorf("der Code fehlt im Suchtext:\n%s", text)
	}
	// Mit einem Leerzeichen verbunden und nicht mit den gespeicherten
	// Zeilenumbrüchen: ein Anriss soll sich wie ein Satz lesen und nicht wie
	// eine Spalte.
	if !strings.Contains(text, "Eiche Buche") {
		t.Errorf("die Mehrfachauswahl steht nicht als Worte im Suchtext:\n%s", text)
	}
	if strings.Contains(text, "Eiche\nBuche") {
		t.Errorf("die Mehrfachauswahl steht als Spalte im Suchtext:\n%s", text)
	}
}

// Ein mehrwertiges Feld einer eigenen Bausteinart trägt dieselbe Markierung
// wie eines auf der Seite selbst, und der Parser muss sie hier genauso lesen.
// Ohne diesen Zweig bliebe von drei Häkchen der erste Wert übrig — und weil
// der Wächter vor der Gruppe steht, wäre das der leere String: jedes Häkchen
// verschwände beim Speichern, ohne dass irgendwo etwas gemeldet würde.
func TestMehrfachauswahlImBausteinBehaeltAlleHaken(t *testing.T) {
	blocks := FromForm(url.Values{
		"b0.typ":         {"merkmal"},
		"b0.f.hoelzer[]": {"", "Eiche", "Buche"},
		"b0.f.bemerkung": {"einwertig"},
	})
	if len(blocks) != 1 {
		t.Fatalf("%d Bausteine, want 1: %+v", len(blocks), blocks)
	}
	if got, will := blocks[0].Fields["hoelzer"], "Eiche\nBuche"; got != will {
		t.Errorf("die Häkchen ergaben %q, wollte %q", got, will)
	}
	if _, da := blocks[0].Fields["hoelzer[]"]; da {
		t.Error("die Markierung steht noch in der Kennung")
	}
	if got := blocks[0].Fields["bemerkung"]; got != "einwertig" {
		t.Errorf("das einwertige Feld ergab %q, wollte %q", got, "einwertig")
	}
}

// Und derselbe Unterschied wie oben auf der Seite: mit dem Wächter allein ist
// die Kennung da und leer, ganz ohne das Feld ist sie gar nicht da.
func TestBausteinfeldGeleertOderAbwesend(t *testing.T) {
	geleert := FromForm(url.Values{
		"b0.typ":         {"merkmal"},
		"b0.f.hoelzer[]": {""},
	})
	if len(geleert) != 1 {
		t.Fatalf("%d Bausteine, want 1", len(geleert))
	}
	val, da := geleert[0].Fields["hoelzer"]
	if !da {
		t.Error("nach dem Wächter allein fehlt die Kennung ganz")
	}
	if val != "" {
		t.Errorf("nach dem Wächter allein steht %q da, wollte leer", val)
	}

	ohne := FromForm(url.Values{"b0.typ": {"merkmal"}, "b0.f.notiz": {"x"}})
	if _, da := ohne[0].Fields["hoelzer"]; da {
		t.Error("die Kennung steht im Baustein, obwohl das Formular sie nie trug")
	}

	// Eine Markierung ohne Kennung ist kein Feld.
	leer := FromForm(url.Values{"b0.typ": {"merkmal"}, "b0.f.[]": {"Eiche"}})
	if len(leer[0].Fields) != 0 {
		t.Errorf("b0.f.[] ergab %+v, wollte nichts", leer[0].Fields)
	}
}

// --- The lightbox --------------------------------------------------------
//
// From here down the tests are English while everything above them is German.
// That is deliberate, not drift: this project's code became English on
// 2026-09-06 and the file is mixed-language on purpose. Do not "fix" one half.

// galleryLook is the three pictures the lightbox tests share.
func galleryLook() Lookup {
	return bilder(map[int64]Image{
		1: {URL: "/media/1/eins.jpg", Alt: "Eins", Width: 1200, Height: 800},
		2: {URL: "/media/1/zwei.jpg", Alt: "Zwei", Width: 1200, Height: 800},
		3: {URL: "/media/1/drei.jpg", Alt: "Drei", Width: 1200, Height: 800},
	})
}

// threePictures is one gallery block of three pictures, the middle one with a
// caption.
func threePictures() Block {
	return Block{Type: TypeGallery, Items: []Item{
		{MediaID: 1},
		{MediaID: 2, Caption: "Das zweite Bild"},
		{MediaID: 3},
	}}
}

// largeViews cuts the rendered gallery into one string per large view, in
// document order.
func largeViews(t *testing.T, html string) []string {
	t.Helper()
	parts := strings.Split(html, `<figure class="hc-galerie__gross"`)
	if len(parts) < 2 {
		t.Fatalf("no large view was rendered at all:\n%s", html)
	}
	return parts[1:]
}

// A tile is the way into its own large view, and the fragment in the URL is the
// whole mechanism — no script is involved on any path.
func TestGalleryTileLinksToItsOwnLargeView(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, galleryLook(), markdown)

	for _, want := range []string{`href="#hc-b1-p1"`, `href="#hc-b1-p2"`, `href="#hc-b1-p3"`} {
		if !strings.Contains(html, want) {
			t.Errorf("no tile links %s:\n%s", want, html)
		}
	}
	for _, want := range []string{`id="hc-b1-p1"`, `id="hc-b1-p2"`, `id="hc-b1-p3"`} {
		if !strings.Contains(html, want) {
			t.Errorf("no large view carries %s:\n%s", want, html)
		}
	}
	// The tile's link has to come before the target it names, or the anchor
	// would move the viewport backwards over the grid.
	if strings.Index(html, `href="#hc-b1-p1"`) > strings.Index(html, `id="hc-b1-p1"`) {
		t.Errorf("the tile's link stands after its own large view:\n%s", html)
	}
}

// Every tile first, every large view after: the grid stays a grid, and the
// enlargements are siblings underneath it rather than interleaved with it.
func TestGalleryLargeViewsFollowEveryTile(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, galleryLook(), markdown)

	lastTile := strings.LastIndex(html, `<figure class="hc-galerie__bild">`)
	firstLarge := strings.Index(html, `<figure class="hc-galerie__gross"`)
	if lastTile < 0 || firstLarge < 0 {
		t.Fatalf("tiles or large views are missing:\n%s", html)
	}
	if lastTile > firstLarge {
		t.Errorf("a tile stands after the first large view:\n%s", html)
	}
	if got := len(largeViews(t, html)); got != 3 {
		t.Errorf("%d large views, want 3:\n%s", got, html)
	}
}

// An absent control at each end, never a wrap-around: a list of four holiday
// photos that jumps back to the first is a surprise, and the browser's own back
// button is the way out.
func TestGalleryFirstHasNoPreviousAndLastHasNoNext(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, galleryLook(), markdown)
	views := largeViews(t, html)

	if strings.Contains(views[0], "hc-galerie__zurueck") {
		t.Errorf("the first picture offers a previous control:\n%s", views[0])
	}
	if !strings.Contains(views[0], `class="hc-galerie__weiter" href="#hc-b1-p2"`) {
		t.Errorf("the first picture does not step to the second:\n%s", views[0])
	}
	if !strings.Contains(views[1], `class="hc-galerie__zurueck" href="#hc-b1-p1"`) ||
		!strings.Contains(views[1], `class="hc-galerie__weiter" href="#hc-b1-p3"`) {
		t.Errorf("the middle picture is missing a neighbour:\n%s", views[1])
	}
	if strings.Contains(views[2], "hc-galerie__weiter") {
		t.Errorf("the last picture offers a next control:\n%s", views[2])
	}
	if !strings.Contains(views[2], `class="hc-galerie__zurueck" href="#hc-b1-p2"`) {
		t.Errorf("the last picture does not step back to the second:\n%s", views[2])
	}
}

// Closing is a link to a fragment that matches nothing, which returns :target
// to no match. It is not href="#", which would scroll to the top of the
// document and add a history entry the back button then has to walk back
// through.
func TestGalleryCloseTargetsNothingOnThePage(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, galleryLook(), markdown)

	if !strings.Contains(html, `href="#`+closeTarget+`"`) {
		t.Errorf("no close control points at %q:\n%s", closeTarget, html)
	}
	if strings.Contains(html, `id="`+closeTarget+`"`) {
		t.Errorf("the close target %q is also an id on the page, so it would "+
			"open a large view instead of closing one:\n%s", closeTarget, html)
	}
	if strings.Contains(html, `href="#"`) {
		t.Errorf("the bare hash is used somewhere:\n%s", html)
	}
}

// Two galleries on one page would otherwise mint #bild-1 twice and the browser
// would jump to whichever came first. The assertion is that the ids are
// distinct, not how they are spelled.
func TestTwoGalleriesOnOnePageMintDistinctIds(t *testing.T) {
	blocks := []Block{
		{Type: TypeText, Markdown: "Guten Tag."},
		threePictures(),
		{Type: TypeText, Markdown: "Und weiter."},
		threePictures(),
	}
	html := Render(blocks, Builtin, galleryLook(), markdown)

	ids := regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	for _, m := range ids {
		seen[m[1]] = true
	}
	if len(ids) != 6 || len(seen) != 6 {
		t.Errorf("%d ids, %d of them distinct, want 6 and 6: %v", len(ids), len(seen), seen)
	}
}

// The large view links no variant and builds no path. A hand-built
// "-large.jpg" carries no ?v= cache-busting and 404s on an original too small
// to have that size; media.MakeResponsive adds the candidate list at request
// time instead.
func TestGalleryLargeViewNamesNoVariant(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, galleryLook(), markdown)

	if strings.Contains(html, "srcset") {
		t.Errorf("the renderer wrote a srcset:\n%s", html)
	}
	if strings.Contains(html, "-large") || strings.Contains(html, "-medium") ||
		strings.Contains(html, "-thumb") {
		t.Errorf("a variant is named in a path:\n%s", html)
	}
	view := largeViews(t, html)[0]
	if !strings.Contains(view, `src="/media/1/eins.jpg"`) {
		t.Errorf("the large view does not serve the tile's own address:\n%s", view)
	}
	if !strings.Contains(view, `sizes="100vw"`) {
		t.Errorf("the large view is missing sizes=\"100vw\":\n%s", view)
	}
	// Nothing squeezes the large view into a fixed shape, so the focus point
	// would change nothing there.
	if strings.Contains(view, "object-position") {
		t.Errorf("the large view carries an inline object-position:\n%s", view)
	}
}

// With /assets/bausteine.css blocked the new rules simply do not exist, so the
// markup has to read as a page on its own: the same picture again, at natural
// size, with its caption, in the flow, with working anchors. Anything whose
// unstyled rendering is a stack of grey boxes fails here.
func TestGalleryUnstyledMarkupIsAFigureSibling(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, galleryLook(), markdown)

	if !strings.HasPrefix(html, `<div class="hc-block hc-galerie hc-spalten-3">`) ||
		!strings.HasSuffix(html, `</div>`) {
		t.Fatalf("the gallery is no longer one div:\n%s", html)
	}
	if got := strings.Count(html, "<div"); got != 1 {
		t.Errorf("%d div elements, want 1 — a wrapper whose only purpose is to "+
			"be positioned:\n%s", got, html)
	}
	for _, forbidden := range []string{"aria-modal", `role="dialog"`, "hintergrund", "backdrop"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("the markup carries %q, which is chrome and not content:\n%s", forbidden, html)
		}
	}
	// The large view is a figure, and it carries its caption where a figure
	// carries one.
	view := largeViews(t, html)[1]
	if !strings.Contains(view, `<figcaption>Das zweite Bild</figcaption>`) {
		t.Errorf("the large view lost the caption the tile has:\n%s", view)
	}
	// A keyboard user who followed the tile anchor continues from the picture
	// they just opened rather than from where they were, and there is no script
	// here to move focus for them.
	if !strings.Contains(view, `tabindex="-1"`) {
		t.Errorf("the large view is not focusable:\n%s", view)
	}
}

// The behaviour that exists today and must survive the change.
func TestGalleryWithNoResolvablePicturesRendersNothing(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, bilder(map[int64]Image{}), markdown)
	if html != "" {
		t.Errorf("a gallery of nothing rendered %q", html)
	}
}

// block.Builtin has no translator, and every test and every caller that does
// not supply one must still get a page.
func TestSetWithoutTranslatorKeepsTheGermanSource(t *testing.T) {
	if Builtin.T != nil {
		t.Fatalf("block.Builtin arrived with a translator")
	}
	html := Render([]Block{threePictures()}, Builtin, galleryLook(), markdown)
	for _, want := range []string{textPrevious, textNext, textClose} {
		if !strings.Contains(html, want) {
			t.Errorf("the German source %q is missing without a translator:\n%s", want, html)
		}
	}
}

// The gate the rest of the i18n work is worthless without.
//
// Marking with i18n.N proves only that the string reaches the catalogue.
// render.go carries no locale of its own, so a string can be marked, collected,
// translated into four languages and printed in German anyway — with every
// other gate green. The same gallery is rendered twice: once without a
// translator, once with one that maps each control name to a token no German
// sentence contains.
func TestLightboxControlsGoThroughTheInjectedTranslator(t *testing.T) {
	german := Render([]Block{threePictures()}, Builtin, galleryLook(), markdown)
	for _, want := range []string{textPrevious, textNext, textClose} {
		if !strings.Contains(german, want) {
			t.Fatalf("the German source %q is missing before translation:\n%s", want, german)
		}
	}

	tokens := map[string]string{
		textPrevious: "QQ-previous-QQ",
		textNext:     "QQ-next-QQ",
		textClose:    "QQ-close-QQ",
	}
	set := Set{T: func(s string) string {
		if out, ok := tokens[s]; ok {
			return out
		}
		return s
	}}

	translated := Render([]Block{threePictures()}, set, galleryLook(), markdown)
	for source, token := range tokens {
		if !strings.Contains(translated, token) {
			t.Errorf("%q was not translated:\n%s", source, translated)
		}
		if strings.Contains(translated, source) {
			t.Errorf("the German literal %q survived the translator — the string "+
				"is collected and translated in four catalogues and printed in "+
				"German anyway:\n%s", source, translated)
		}
	}
}
