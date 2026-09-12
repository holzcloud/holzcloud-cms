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
	pickupTime := time.Date(0, time.January, 1, 16, 30, 0, 0, time.UTC)
	fittings := []string{"Schublade", "Kabelauslass", "Verlängerung"}
	material := field.Term{Name: "Eiche", Slug: "eiche", URL: "/tag/eiche"}
	seats := field.Number{Value: 8, Raw: "8"}
	// A code field holds what somebody typed, tags and all. It is a plain
	// string on purpose: html/template escapes it, and that escaping is the
	// whole promise of the kind.
	joinery := `<balken laenge="240">Eiche</balken>`
	// The nine kinds the fixture did not carry until the gate was measured.
	// template.Check renders this and nothing else, so a kind that is absent
	// here is a kind whose branch in an uploaded theme has never once been
	// executed — and the comment below the list claimed the opposite for as
	// long as it was untrue. sample_test.go now walks field.Kinds, so the claim
	// is checked rather than asserted.
	//
	// Every value is the shape field.Resolve produces, not a shape that merely
	// looks like it: a fixture that got that wrong would document a contract
	// the renderer never fulfils.
	description := "Massive Eiche, von Hand geölt.\nZwei Zeilen, ohne Formatierung."
	price := field.Number{Value: 1290.5, Raw: "1290.50"}
	// A date carries a day and no time of day, which is what time.Parse gives
	// back for "2026-05-01" — down to the zone.
	opened := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	view := field.Image{
		URL: "/media/1/tisch.jpg", Alt: "Ein Eichentisch in der Werkstatt",
		// Width, Height and Focus are filled because internal/public fills
		// them, and every shipped theme reads two of the three. A fixture that
		// left them at zero would let an upload through whose width attribute
		// has never been rendered.
		Width: 1600, Height: 1067, Focus: "50% 35%",
	}
	workshop := field.Ref{Title: "Die Werkstatt", URL: "/werkstatt", Kind: "page"}
	// A group's rows are what Resolve makes of them: one map per row, each
	// holding the sub-fields' resolved values. This is the shape the map side
	// of the contract has never carried.
	mondayFrom := time.Date(0, time.January, 1, 8, 0, 0, 0, time.UTC)
	mondayTo := time.Date(0, time.January, 1, 12, 0, 0, 0, time.UTC)
	thursdayFrom := time.Date(0, time.January, 1, 13, 30, 0, 0, time.UTC)
	thursdayTo := time.Date(0, time.January, 1, 18, 0, 0, 0, time.UTC)
	openingHours := []map[string]any{
		{"tag": "Montag", "von": &mondayFrom, "bis": &mondayTo},
		{"tag": "Donnerstag", "von": &thursdayFrom, "bis": &thursdayTo},
	}

	// A snippet's own values are declared here for the reason the page's are:
	// its two views have to hold the same value, not two that look alike.
	// A time of day again carries no date and no zone.
	opens := time.Date(0, time.January, 1, 7, 30, 0, 0, time.UTC)

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
			// One value that is not a string, spelled the way Page.Felder
			// spells it: a snippet carries every kind a page carries, and a
			// fixture of nothing but strings would let an upload through whose
			// handling of a typed value has never once run.
			SnippetFields: map[string]map[string]any{
				"footer-kontakt": {
					"telefon": "07721 123456",
					"strasse": "Hauptstraße 4",
					"oeffnet": &opens,
				},
			},
			SnippetList: map[string][]field.Entry{
				"footer-kontakt": {
					{Key: "telefon", Label: "Telefon", Kind: field.KindText, Value: "07721 123456", Text: "07721 123456"},
					{Key: "strasse", Label: "Straße", Kind: field.KindText, Value: "Hauptstraße 4", Text: "Hauptstraße 4"},
					{Key: "oeffnet", Label: "Öffnet", Kind: field.KindTime, Value: &opens, Text: "07:30"},
				},
			},
			Terms: []TermLink{
				{Name: "Eiche", URL: "/tag/eiche", Count: 7},
				{Name: "Möbel", URL: "/tag/moebel", Count: 3},
			},
			Design:    ":root{--hc-brand:oklch(55% 0.12 45)}",
			HasSearch: true,
			FeedURL:   "/feed.atom",
			Languages: []LanguageLink{
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
			Kind:          "produkt",
			Fields: map[string]any{
				"holzart":     "Eiche",
				"lieferzeit":  "4 Wochen",
				"ausstattung": fittings,
				"abholzeit":   &pickupTime,
				"sitzplaetze": seats,
				"abbundzeile": joinery,
				"material":    &material,

				"beschreibung":    description,
				"preis":           price,
				"eroeffnet":       &opened,
				"lieferbar":       true,
				"oberflaeche":     "geölt",
				"ansicht":         &view,
				"prospekt":        "https://example.de/prospekt.pdf",
				"werkstatt":       &workshop,
				"oeffnungszeiten": openingHours,
			},
			FieldList: []field.Entry{
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
					Value: fittings, Values: fittings,
					Text: "Schublade, Kabelauslass, Verlängerung",
				},
				{Key: "abholzeit", Label: "Abholzeit", Kind: field.KindTime, Value: &pickupTime, Text: "16:30"},
				{Key: "sitzplaetze", Label: "Sitzplätze", Kind: field.KindRange, Value: seats, Text: "8"},
				{Key: "abbundzeile", Label: "Abbundzeile", Kind: field.KindCode, Value: joinery, Text: joinery},
				{
					Key: "material", Label: "Material", Kind: field.KindTerm,
					Value: &material, Term: &material, Text: material.Name,
				},
				{
					Key: "beschreibung", Label: "Beschreibung", Kind: field.KindLong,
					Value: description, Text: description,
				},
				{Key: "preis", Label: "Preis", Kind: field.KindNumber, Value: price, Text: price.Raw},
				// A date leaves Text empty: the theme prints it with its own
				// formatDate, and field.List says so where it decides not to
				// fill it in.
				{Key: "eroeffnet", Label: "Eröffnet", Kind: field.KindDate, Value: &opened},
				{Key: "lieferbar", Label: "Lieferbar", Kind: field.KindBool, Value: true, Yes: true},
				{
					Key: "oberflaeche", Label: "Oberfläche", Kind: field.KindChoice,
					Value: "geölt", Text: "geölt",
				},
				{Key: "ansicht", Label: "Ansicht", Kind: field.KindImage, Value: &view, Image: &view},
				{
					Key: "prospekt", Label: "Prospekt", Kind: field.KindLink,
					Value: "https://example.de/prospekt.pdf", Text: "https://example.de/prospekt.pdf",
				},
				{
					Key: "werkstatt", Label: "Werkstatt", Kind: field.KindRef,
					Value: &workshop, Ref: &workshop, Text: workshop.Title,
				},
				{
					Key: "oeffnungszeiten", Label: "Öffnungszeiten", Kind: field.KindGroup,
					Value: openingHours,
					Rows: [][]field.Entry{
						{
							{Key: "tag", Label: "Tag", Kind: field.KindText, Value: "Montag", Text: "Montag"},
							{Key: "von", Label: "Von", Kind: field.KindTime, Value: &mondayFrom, Text: "08:00"},
							{Key: "bis", Label: "Bis", Kind: field.KindTime, Value: &mondayTo, Text: "12:00"},
						},
						{
							{Key: "tag", Label: "Tag", Kind: field.KindText, Value: "Donnerstag", Text: "Donnerstag"},
							{Key: "von", Label: "Von", Kind: field.KindTime, Value: &thursdayFrom, Text: "13:30"},
							{Key: "bis", Label: "Bis", Kind: field.KindTime, Value: &thursdayTo, Text: "18:00"},
						},
					},
				},
			},
			Translations: []LanguageLink{
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
// no dates, no neighbours, no image, no menus, no labels, no snippet bodies —
// and one snippet whose fields are defined and empty, which is the state a
// theme that indexes into .Site.SnippetFields actually breaks on.
func MinimalData() PageData {
	return PageData{
		Site: SiteData{
			Name:   "Holzbau Schmidt",
			Locale: "de",
			// The same for a snippet: its fields are defined and nobody has
			// filled any of them in. Resolve puts every defined field in the
			// map here too, so {{index .Site.SnippetFields "footer-kontakt"
			// "oeffnet"}} is a nil time rather than a missing key.
			//
			// There is deliberately no Bausteinliste, and it is the same
			// omission Feldliste is below and not a second one: field.List
			// drops an entry whose value is empty, so a snippet nobody has
			// filled in is missing from that map entirely. A fixture that
			// invented an entry there would reject a template for ranging the
			// list the way the specification tells it to.
			//
			// The body is absent as well: a snippet is a body plus optional
			// fields, and a theme has to survive either half being away.
			SnippetFields: map[string]map[string]any{
				"footer-kontakt": {
					"telefon": "",
					"strasse": "",
					"oeffnet": (*time.Time)(nil),
				},
			},
		},
		Page: PageContent{
			Title:       "Über uns",
			ContentHTML: "<p>Inhalt</p>",
			Slug:        "ueber-uns",
			// The website has defined its fields; nobody has filled any of them
			// in on this page. That is where the empty case of an own field
			// lives: Resolve puts every defined field in the map, filled or
			// not, so {{.Page.Fields.abholzeit}} is a nil time rather than a
			// missing key — and a theme that reaches through it unguarded fails
			// here, which is the point of this fixture.
			//
			// Feldliste stays empty on purpose, and it is not the same
			// omission: field.List drops an entry whose value is empty — every
			// arm of its switch does — so a list carrying an empty entry is a
			// state the program cannot produce. A fixture that invented one
			// would reject a template for handling the list the way the
			// specification tells it to.
			Fields: map[string]any{
				"holzart":     "",
				"lieferzeit":  "",
				"ausstattung": []string{},
				"abholzeit":   (*time.Time)(nil),
				"sitzplaetze": field.Number{},
				"abbundzeile": "",
				"material":    (*field.Term)(nil),

				"beschreibung": "",
				"preis":        field.Number{},
				"eroeffnet":    (*time.Time)(nil),
				"lieferbar":    false,
				"oberflaeche":  "",
				"ansicht":      (*field.Image)(nil),
				"prospekt":     "",
				"werkstatt":    (*field.Ref)(nil),
				// A group with no rows resolves to an empty slice and not to
				// nil: Resolve builds it with make, so a theme's {{range}} sees
				// a list of nothing rather than a missing key.
				"oeffnungszeiten": []map[string]any{},
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
