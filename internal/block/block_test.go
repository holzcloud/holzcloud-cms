package block

import (
	"net/url"
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
		{Type: TypeGallery, Variant: "4", Items: []Item{{MediaID: 1}, {MediaID: 2, Caption: "Im Mai"}}},
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
		len(got[1].Items) != 2 || got[1].Items[1].Caption != "Im Mai" {
		t.Errorf("got %+v", got)
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
