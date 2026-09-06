package template

import (
	"html/template"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
)

// SampleData and MinimalData are the two renderings every template has to
// survive. They are exported because three things depend on being able to name
// exactly what a template is handed:
//
//   - the upload check, which renders an archive before accepting it,
//   - the test suite, which renders the shipped themes,
//   - the authoring specification, which documents the contract for whoever —
//     or whatever — writes a template.
//
// Keeping them in one place is the point. A fixture that lives in a test file
// cannot be shown to a template author, and a specification written separately
// from the fixture drifts away from it silently.

// SampleData fills every field a template may touch.
//
// Every value is deliberately non-empty, which sample_test.go enforces by
// reflection: a field left at its zero value here would make the upload check
// pass a template that has never had that field rendered even once.
func SampleData() PageData {
	published := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 4, 2, 16, 30, 0, 0, time.UTC)

	// The own-field values, declared once because Felder and Feldliste are two
	// views of the same data and a fixture where they disagree would document a
	// contract the renderer never produces.
	//
	// A time of day carries no date and no zone — this is exactly what
	// field.ParseTimeOfDay hands back for "16:30", down to the year.
	abholzeit := time.Date(0, time.January, 1, 16, 30, 0, 0, time.UTC)
	ausstattung := []string{"Schublade", "Kabelauslass", "Verlängerung"}
	material := field.Term{Name: "Eiche", Slug: "eiche", URL: "/tag/eiche"}
	sitzplaetze := field.Number{Value: 8, Raw: "8"}
	// A code field holds what somebody typed, tags and all. It is a plain
	// string on purpose: html/template escapes it, and that escaping is the
	// whole promise of the kind.
	abbund := `<balken laenge="240">Eiche</balken>`

	return PageData{
		Site: SiteData{
			Name:            "Holzbau Schmidt",
			Description:     "Möbel nach Maß",
			MetaDescription: "Schreinerei aus dem Schwarzwald",
			Locale:          "de",
			TimeZone:        "Europe/Berlin",
			FaviconURL:      "/media/1/favicon.png",
			LogoURL:         "/media/1/logo.png",
			URL:             "https://example.de",
			Snippets: map[string]template.HTML{
				"footer-kontakt": "<p>Telefon 07721 123456</p>",
			},
			// A snippet's own fields, under the same key its body has: the
			// body and the fields are two halves of one snippet, exactly as a
			// page is content plus fields. Bausteinfelder and Bausteinliste are
			// two views of the same data and are filled so they agree, for the
			// reason the page's two are.
			Bausteinfelder: map[string]map[string]any{
				"footer-kontakt": {
					"telefon": "07721 123456",
					"strasse": "Hauptstraße 4",
				},
			},
			Bausteinliste: map[string][]field.Entry{
				"footer-kontakt": {
					{Key: "telefon", Label: "Telefon", Kind: field.KindText, Value: "07721 123456", Text: "07721 123456"},
					{Key: "strasse", Label: "Straße", Kind: field.KindText, Value: "Hauptstraße 4", Text: "Hauptstraße 4"},
				},
			},
			Terms: []TermLink{
				{Name: "Eiche", URL: "/tag/eiche", Count: 7},
				{Name: "Möbel", URL: "/tag/moebel", Count: 3},
			},
			Design:    ":root{--hc-brand:oklch(55% 0.12 45)}",
			HasSearch: true,
			FeedURL:   "/feed.atom",
			Sprachen: []LanguageLink{
				{Code: "de", Name: "Deutsch", URL: "/ueber-uns", Active: true},
				{Code: "fr", Name: "Français", URL: "/fr/a-propos"},
			},
		},
		Page: PageContent{
			Title:       "Über uns",
			ContentHTML: "<p>Inhalt</p>",
			Slug:        "ueber-uns",
			PublishedAt: &published,
			UpdatedAt:   &updated,
			Excerpt:     "Wir bauen Möbel.",
			// True and false both have to be exercised; MinimalData carries the
			// false case, where a theme is expected to print the title itself.
			HasOwnHeading: true,
			IsPost:        true,
			ArchiveURL:    "/aktuelles",
			Terms:         []TermLink{{Name: "Möbel", URL: "/tag/moebel", Count: 3}},
			Prev:          &PageLink{Title: "Voriger Beitrag", URL: "/vorig"},
			Next:          &PageLink{Title: "Nächster Beitrag", URL: "/naechst"},
			Art:           "produkt",
			Felder: map[string]any{
				"holzart":     "Eiche",
				"lieferzeit":  "4 Wochen",
				"ausstattung": ausstattung,
				"abholzeit":   &abholzeit,
				"sitzplaetze": sitzplaetze,
				"abbundzeile": abbund,
				"material":    &material,
			},
			Feldliste: []field.Entry{
				{Key: "holzart", Label: "Holzart", Kind: field.KindText, Value: "Eiche", Text: "Eiche"},
				{Key: "lieferzeit", Label: "Lieferzeit", Kind: field.KindText, Value: "4 Wochen", Text: "4 Wochen"},
				// One entry per kind this version can put in the list beyond a
				// plain string, so a template's handling of each is rendered at
				// least once before an upload is accepted. Values, Term and Text
				// are filled the way field.List fills them — Values is the same
				// slice as Value, Term the same pointer, and Text the readable
				// form of both.
				{
					Key: "ausstattung", Label: "Ausstattung", Kind: field.KindMulti,
					Value: ausstattung, Values: ausstattung,
					Text: "Schublade, Kabelauslass, Verlängerung",
				},
				{Key: "abholzeit", Label: "Abholzeit", Kind: field.KindTime, Value: &abholzeit, Text: "16:30"},
				{Key: "sitzplaetze", Label: "Sitzplätze", Kind: field.KindRange, Value: sitzplaetze, Text: "8"},
				{Key: "abbundzeile", Label: "Abbundzeile", Kind: field.KindCode, Value: abbund, Text: abbund},
				{
					Key: "material", Label: "Material", Kind: field.KindTerm,
					Value: &material, Term: &material, Text: material.Name,
				},
			},
			Uebersetzungen: []LanguageLink{
				{Code: "de", Name: "Deutsch", URL: "/ueber-uns", Active: true},
				{Code: "fr", Name: "Français", URL: "/fr/a-propos"},
			},
		},
		Menus: map[string][]menu.MenuNode{
			"main":   {{MenuItem: menu.MenuItem{Title: "Start", ItemType: "page", PageSlug: "home"}}},
			"footer": {{MenuItem: menu.MenuItem{Title: "Impressum", ItemType: "url", URL: "/impressum"}}},
		},
		Search: SearchData{
			Query:     "Möbel",
			Submitted: true,
			Results:   []SearchHit{{Title: "Leistungen", URL: "/leistungen", Snippet: "…<mark>Möbel</mark>…"}},
		},
		Archive: ArchiveData{
			Entries: []ArchiveEntry{{
				Title:       "Neue Werkbank",
				URL:         "/neue-werkbank",
				Excerpt:     "Endlich fertig.",
				PublishedAt: &published,
				ImageURL:    "/media/1/werkbank.jpg",
				Terms:       []TermLink{{Name: "Eiche", URL: "/tag/eiche", Count: 7}},
			}},
			Page:       2,
			TotalPages: 3,
			Total:      25,
			PrevURL:    "/aktuelles",
			NextURL:    "/aktuelles?seite=3",
			Term:       "Eichenholz",
			Terms:      []TermLink{{Name: "Eiche", URL: "/tag/eiche", Count: 7}},
		},
		Gate: GateData{
			Hint:  "Das Passwort steht in unserem Anschreiben.",
			Path:  "/preisliste",
			Wrong: true,
		},
		Preview: PreviewData{Active: true, Status: "draft"},
		Shop: ShopData{
			Enabled:           true,
			URL:               "/shop",
			Audience:          "private",
			CanSwitchAudience: true,
			TaxNote:           "inkl. 8.1 % MWST",
			ShippingNote:      "Versandkostenfrei ab CHF 200.00",
			Categories:        []TermLink{{Name: "Tische", URL: "/shop/kategorie/tische", Count: 4}},
		},
		Catalogue: CatalogueData{
			Products: []ProductEntry{{
				Title:        "Esstisch Adlisberg",
				Subtitle:     "Eiche massiv, geölt",
				URL:          "/shop/esstisch-adlisberg",
				Excerpt:      "Vier Meter, aus einem Stamm.",
				ImageURL:     "/media/1/tisch.jpg",
				Price:        "CHF 4’900.00",
				PriceNote:    "inkl. 8.1 % MWST",
				Available:    true,
				SoldOutLabel: "Ausverkauft",
				Terms:        []TermLink{{Name: "Tische", URL: "/shop/kategorie/tische", Count: 4}},
			}},
			Page: 1, TotalPages: 2, Total: 14,
			PrevURL: "/shop", NextURL: "/shop?seite=2",
			Term: "Tische",
		},
		Product: ProductData{
			Title:           "Esstisch Adlisberg",
			Subtitle:        "Eiche massiv, geölt",
			Slug:            "esstisch-adlisberg",
			DescriptionHTML: "<p>Aus einem Stamm.</p>",
			SKU:             "TI-4900",
			Price:           "CHF 4’900.00",
			PriceNote:       "inkl. 8.1 % MWST",
			PriceOther:      "CHF 4’532.84 zzgl. MWST",
			ImageURL:        "/media/1/tisch.jpg",
			Gallery:         []string{"/media/1/tisch-detail.jpg"},
			Available:       true,
			StockNote:       "Noch 2 an Lager",
			DeliveryNote:    "Lieferzeit 3–4 Wochen",
			Terms:           []TermLink{{Name: "Tische", URL: "/shop/kategorie/tische", Count: 4}},
			AddURL:          "/warenkorb/hinzufuegen",
		},
		Cart: CartData{
			Count: 3,
			URL:   "/warenkorb",
			Total: "CHF 5’157.00",
			Lines: []CartLine{{
				Title:     "Esstisch Adlisberg",
				Subtitle:  "Eiche massiv, geölt",
				URL:       "/shop/esstisch-adlisberg",
				Slug:      "esstisch-adlisberg",
				ImageURL:  "/media/1/tisch.jpg",
				Quantity:  1,
				UnitPrice: "CHF 4’900.00",
				LinePrice: "CHF 4’900.00",
				Available: true,
			}},
			Totals: CartTotals{
				Items:        "CHF 5’145.00",
				Shipping:     "CHF 12.00",
				ShippingFree: true,
				Total:        "CHF 5’157.00",
				TaxLines: []CartTaxLine{{
					Label: "MWST 8.1 %", Net: "CHF 4’770.58", Tax: "CHF 386.42",
				}},
				TaxNote: "keine MWST (Kleinunternehmen)",
			},
			CheckoutURL: "/kasse",
			Blocked:     "Ein Artikel im Warenkorb ist nicht mehr verfügbar.",
			UpdateURL:   "/warenkorb/menge",
			RemoveURL:   "/warenkorb/entfernen",
		},
		Checkout: CheckoutData{
			Action:       "/kasse",
			Notice:       "Esstisch Adlisberg ist nicht mehr in der benötigten Menge verfügbar.",
			Business:     true,
			ReturnPolicy: "Rückgabe innert 14 Tagen, ungebraucht.",
			Accepted:     true,
			Methods: []PaymentMethod{
				{Value: "invoice", Label: "Rechnung", Note: "Rechnung mit der Ware."},
			},
			Values: map[string]string{
				"email": "kundin@example.ch", "name": "Anna Meier", "firma": "Meier AG",
				"uid": "CHE-123.456.789 MWST", "telefon": "044 123 45 67",
				"strasse": "Seestrasse 4", "plz": "8002", "ort": "Zürich",
				"land": "CH", "bemerkung": "Bitte vormittags liefern.",
				"zahlungsart": "invoice",
			},
			Errors: map[string]string{"plz": "Eine Schweizer Postleitzahl hat vier Ziffern."},
		},
		Order: OrderData{
			Number:         "2026-0007",
			Email:          "kundin@example.ch",
			Name:           "Anna Meier",
			Company:        "Meier AG",
			Address:        "Seestrasse 4, 8002 Zürich, CH",
			Note:           "Bitte vormittags liefern.",
			Status:         "new",
			PaymentLabel:   "Rechnung",
			PaymentNote:    "Die Rechnung liegt der Sendung bei.",
			PaymentPending: true,
			ReturnPolicy:   "Rückgabe innert 14 Tagen, ungebraucht.",
			Lines: []CartLine{{
				Title: "Esstisch Adlisberg", Subtitle: "Eiche massiv, geölt",
				Quantity: 1, UnitPrice: "CHF 4’900.00", LinePrice: "CHF 4’900.00",
				Available: true, Slug: "esstisch-adlisberg", URL: "/shop/esstisch-adlisberg",
				ImageURL: "/media/1/tisch.jpg",
			}},
			Totals: CartTotals{
				Items: "CHF 4’900.00", Shipping: "CHF 12.00", ShippingFree: true,
				Total: "CHF 4’912.00", TaxNote: "keine MWST (Kleinunternehmen)",
				TaxLines: []CartTaxLine{{Label: "MWST", Net: "CHF 4’543.94", Tax: "CHF 368.06"}},
			},
		},
		Meta: MetaData{
			CanonicalURL:   "https://example.de/ueber-uns",
			Description:    "Wir bauen Möbel.",
			OGImage:        "https://example.de/media/1/vorschau.jpg",
			NoIndex:        true,
			Message:        "Wir sind gleich wieder da.",
			StructuredData: `{"@context":"https://schema.org","@type":"WebPage"}`,
		},
	}
}

// MinimalData is the same page with everything optional left out.
//
// This is the case that actually breaks templates. `{{.Page.Prev.Title}}`
// renders perfectly against SampleData and fails at "nil pointer evaluating
// *template.PageLink.Title" the first time a visitor opens the oldest post —
// and a check that only ever renders the full fixture would have called that
// template good. Everything that can legitimately be absent is absent here:
// no dates, no neighbours, no image, no menus, no labels, no snippets.
func MinimalData() PageData {
	return PageData{
		Site: SiteData{
			Name:   "Holzbau Schmidt",
			Locale: "de",
		},
		Page: PageContent{
			Title:       "Über uns",
			ContentHTML: "<p>Inhalt</p>",
			Slug:        "ueber-uns",
			// The website has defined its fields; nobody has filled any of them
			// in on this page. That is where the empty case of an own field
			// lives: Resolve puts every defined field in the map, filled or
			// not, so {{.Page.Felder.abholzeit}} is a nil time rather than a
			// missing key — and a theme that reaches through it unguarded fails
			// here, which is the point of this fixture.
			//
			// Feldliste stays empty on purpose, and it is not the same
			// omission: field.List drops an entry whose value is empty — every
			// arm of its switch does — so a list carrying an empty entry is a
			// state the program cannot produce. A fixture that invented one
			// would reject a template for handling the list the way the
			// specification tells it to.
			Felder: map[string]any{
				"holzart":     "",
				"lieferzeit":  "",
				"ausstattung": []string{},
				"abholzeit":   (*time.Time)(nil),
				"sitzplaetze": field.Number{},
				"abbundzeile": "",
				"material":    (*field.Term)(nil),
			},
		},
		// An empty archive still renders list.html: a label nobody has used yet,
		// or a blog before the first post.
		Archive: ArchiveData{Page: 1, TotalPages: 1},
		// A search page before anything has been typed into it.
		Search: SearchData{},
		Gate:   GateData{Path: "/preisliste"},
		// A website that sells nothing. Everything a shop would fill stays
		// zero, which is what a theme with a basket in its header has to
		// survive — most websites in a multi-site install have no shop.
		Shop:      ShopData{},
		Catalogue: CatalogueData{Page: 1, TotalPages: 1},
		Product:   ProductData{},
		// An empty basket on a website that sells nothing: no lines, no total,
		// no checkout link. A layout with a basket badge has to survive it.
		Cart: CartData{},
		// The order form before anything has been typed, and a page that is
		// not an order confirmation at all.
		Checkout: CheckoutData{Action: "/kasse", Values: map[string]string{}},
		Order:    OrderData{},
	}
}
