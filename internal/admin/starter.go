package admin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// starterPage is one of the pages a new website gets.
type starterPage struct {
	title    string
	slug     string
	status   string
	markdown string
}

// starterPages are what a brand-new website starts with.
//
// A site with no pages answers its own domain with "Seite nicht gefunden",
// which is the first thing the owner sees after pointing DNS at it. The two
// legal pages are drafts: they are scaffolding to fill in, and publishing an
// empty imprint would be worse than having none.
var starterPages = []starterPage{
	{
		title:  "Startseite",
		slug:   "home",
		status: "published",
		markdown: `Willkommen!

Diese Startseite wurde automatisch angelegt. Klicke auf **Bearbeiten**, um sie
durch deinen eigenen Text zu ersetzen.
`,
	},
	{
		title:  "Impressum",
		slug:   "impressum",
		status: "draft",
		// The headings follow § 5 DDG. The placeholders are deliberately
		// obvious: half-filled legal text that looks finished is the dangerous
		// state, so every line says what belongs there.
		markdown: `## Angaben gemäß § 5 DDG

**Diese Seite ist eine Vorlage und noch nicht ausgefüllt.** Trage deine
eigenen Angaben ein und veröffentliche die Seite erst danach.

Name des Unternehmens
Straße und Hausnummer
Postleitzahl und Ort

**Vertreten durch:** Vor- und Nachname der vertretungsberechtigten Person

## Kontakt

Telefon: (eintragen)
E-Mail: (eintragen)

## Registereintrag

Registergericht und Registernummer, falls vorhanden.

## Umsatzsteuer-Identifikationsnummer

USt-IdNr. gemäß § 27 a Umsatzsteuergesetz, falls vorhanden.

## Verantwortlich für den Inhalt

Name und Anschrift der verantwortlichen Person.

---

Diese Vorlage ersetzt keine Rechtsberatung. Welche Angaben für dein Gewerbe
verpflichtend sind, hängt von Rechtsform und Tätigkeit ab.
`,
	},
	{
		title:  "Datenschutzerklärung",
		slug:   "datenschutz",
		status: "draft",
		markdown: `## Datenschutz auf einen Blick

**Diese Seite ist eine Vorlage und noch nicht ausgefüllt.** Prüfe die Angaben
und veröffentliche die Seite erst danach.

## Verantwortliche Stelle

Name, Anschrift und Kontaktdaten der verantwortlichen Stelle im Sinne von
Art. 4 Abs. 7 DSGVO.

## Welche Daten beim Besuch dieser Website anfallen

Diese Website setzt keine Cookies, bindet nichts von fremden Servern ein und
verwendet keine Analysewerkzeuge. Beim Aufruf einer Seite verarbeitet der
Server die technisch notwendigen Verbindungsdaten (IP-Adresse, Zeitpunkt,
aufgerufene Adresse) — trage hier ein, wie lange diese Protokolle aufbewahrt
werden.

## Deine Rechte

Auskunft (Art. 15 DSGVO), Berichtigung (Art. 16), Löschung (Art. 17),
Einschränkung der Verarbeitung (Art. 18), Datenübertragbarkeit (Art. 20) und
Widerspruch (Art. 21). Außerdem besteht ein Beschwerderecht bei einer
Datenschutz-Aufsichtsbehörde.

---

Diese Vorlage ersetzt keine Rechtsberatung.
`,
	},
}

// createStarterContent fills a new website with a published start page, the two
// legal drafts, a main menu and a footer menu linking them.
//
// Errors are logged rather than returned: the website itself is already
// created, and failing the request would leave the operator staring at an error
// for a site that does exist.
func (h *Handler) createStarterContent(ctx context.Context, websiteID int64, userID *int64) {
	created := make(map[string]int64, len(starterPages))

	for _, sp := range starterPages {
		html, err := page.RenderMarkdown(sp.markdown)
		if err != nil {
			slog.Error("render starter page", "err", err, "slug", sp.slug)
			continue
		}
		p, err := h.pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: websiteID,
			Title:     sp.title,
			Slug:      sp.slug,
			Markdown:  sp.markdown,
			HTML:      html,
			Status:    sp.status,
			Meta:      page.PageMeta{Excerpt: page.Excerpt(sp.markdown)},
			UserID:    userID,
		})
		if err != nil {
			slog.Error("create starter page", "err", err, "slug", sp.slug)
			continue
		}
		created[sp.slug] = p.ID
	}

	h.createStarterMenu(ctx, websiteID, "Hauptmenü", "main", created, []string{"home"})
	// § 5 DDG wants the imprint reachable from every page; a footer menu is how
	// a theme does that.
	h.createStarterMenu(ctx, websiteID, "Fußzeile", "footer", created, []string{"impressum", "datenschutz"})
}

func (h *Handler) createStarterMenu(ctx context.Context, websiteID int64, name, location string, pages map[string]int64, slugs []string) {
	if h.menuStore == nil {
		return
	}
	// The starter menus are in the website's main language: a brand-new website
	// has no other one yet.
	m, err := h.menuStore.CreateMenu(ctx, websiteID, name, location, "")
	if err != nil {
		slog.Error("create starter menu", "err", err, "location", location)
		return
	}
	for i, slug := range slugs {
		id, ok := pages[slug]
		if !ok {
			continue
		}
		title := slug
		for _, sp := range starterPages {
			if sp.slug == slug {
				title = sp.title
			}
		}
		if _, err := h.menuStore.CreateItem(ctx, m.ID, nil, title, "page", "", &id, i); err != nil {
			slog.Error("create starter menu item", "err", err, "slug", slug)
		}
	}
}

// starterContentSummary is what the flash message says after creating a site.
func starterContentSummary() string {
	return fmt.Sprintf("Website angelegt – mit Startseite sowie Impressum und Datenschutzerklärung als Entwurf (%d Seiten)", len(starterPages))
}
