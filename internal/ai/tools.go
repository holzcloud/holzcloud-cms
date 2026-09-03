package ai

import (
	"errors"
	"fmt"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// What an assistant can do.
//
// Few tools, each doing one thing an editor would recognise. The temptation is
// to expose the stores as they are — thirty methods, every option — and the
// result is an assistant that has to guess which of four ways to save a page is
// the right one. So: list, read, write, publish, and the two lookups needed to
// write anything sensible.
//
// Two rules run through all of them. Nothing is published unless somebody asked
// for it in so many words, and every write goes through the same store call the
// admin makes — same slug rules, same revision, same validation. A page from an
// assistant is a page, and a person can look at it, change it and revert it
// afterwards.

// Deps are the stores the tools work through.
type Deps struct {
	Domains *domain.Store
	Pages   *page.Store
	Media   *media.Store
	// Fields are the website's own page fields. Nil means an assistant sees
	// none, which is right for a build without them.
	Fields *field.Store
}

// Tools builds the tool list.
func Tools(d Deps) []Tool {
	return []Tool{
		websitesAuflisten(d),
		seitenAuflisten(d),
		seiteLesen(d),
		seiteSuchen(d),
		medienAuflisten(d),
		felderAuflisten(d),
		seiteAnlegen(d),
		seiteAendern(d),
		seiteVeroeffentlichen(d),
	}
}

// --- lesen ------------------------------------------------------------------

func websitesAuflisten(d Deps) Tool {
	return Tool{
		Name: "websites_auflisten",
		Description: "Listet die Websites dieser Installation mit Kennung, Name und Adresse. " +
			"Der erste Aufruf, wenn nicht klar ist, um welche Website es geht.",
		InputSchema: Schema{Type: "object"},
		Run: func(c Call) (any, error) {
			list, err := d.Domains.ListWebsites(c.Ctx)
			if err != nil {
				return nil, err
			}
			out := []map[string]any{}
			for _, ws := range list {
				if c.Scope.MaySee(ws.ID) != nil {
					continue
				}
				out = append(out, map[string]any{
					"id": ws.ID, "name": ws.Name, "beschreibung": ws.Description,
					"aktiv": ws.Active, "sprache": ws.Locale,
				})
			}
			return map[string]any{"websites": out}, nil
		},
	}
}

func seitenAuflisten(d Deps) Tool {
	return Tool{
		Name: "seiten_auflisten",
		Description: "Listet die Seiten einer Website mit Kennung, Titel, Adresse und Zustand. " +
			"Ohne Inhalt — den holt seite_lesen.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "Kennung der Website"},
				"zustand": {Type: "string", Description: "entwurf, veroeffentlicht oder alle",
					Enum: []string{"entwurf", "veroeffentlicht", "alle"}},
				"anzahl": {Type: "integer", Description: "höchstens so viele, Vorgabe 50"},
				"sprache": {Type: "string", Description: "Sprachkürzel wie fr; leer lassen " +
					"für alle Sprachen, \"haupt\" für die Hauptsprache"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Zustand string `json:"zustand"`
				Anzahl  int    `json:"anzahl"`
				Sprache string `json:"sprache"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			// Ohne Angabe alle Sprachen: eine Liste, die stillschweigend nur die
			// Hauptsprache zeigt, sieht vollständig aus und ist es nicht.
			filter := page.ListFilter{Locale: "*", Page: 1, PerPage: clampCount(a.Anzahl)}
			switch strings.TrimSpace(a.Sprache) {
			case "":
				// alle
			case "haupt":
				filter.Locale = ""
			default:
				filter.Locale = locale.Normalise(a.Sprache)
			}
			switch a.Zustand {
			case "entwurf":
				filter.Status = "draft"
			case "veroeffentlicht":
				filter.Status = "published"
			}

			pages, total, err := d.Pages.ListPages(c.Ctx, a.Website, filter)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(pages))
			for _, p := range pages {
				out = append(out, kurz(p))
			}
			return map[string]any{"seiten": out, "insgesamt": total}, nil
		},
	}
}

func seiteLesen(d Deps) Tool {
	return Tool{
		Name: "seite_lesen",
		Description: "Holt eine Seite mit ihrem vollständigen Text. Entweder über die Kennung " +
			"oder über Website und Adresse.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":      {Type: "integer", Description: "Kennung der Seite"},
				"website": {Type: "integer", Description: "Kennung der Website, zusammen mit adresse"},
				"adresse": {Type: "string", Description: "Adresse der Seite ohne Schrägstrich"},
			},
		},
		Run: func(c Call) (any, error) {
			p, err := findePage(c, d)
			if err != nil {
				return nil, err
			}
			out := kurz(*p)
			out["markdown"] = p.ContentMarkdown
			out["kurzfassung"] = p.Excerpt
			daten := field.Decode(p.Fields)
			if len(daten.Values) > 0 {
				out["felder"] = map[string]string(daten.Values)
			}
			if len(daten.Rows) > 0 {
				out["gruppen"] = daten.Rows
			}
			// Eine Seite aus Bausteinen hat kein Markdown, das sich sinnvoll
			// zurückschreiben liesse. Das muss dastehen, sonst schreibt ein
			// Assistent seinen Text hinein und wundert sich, dass die Bausteine
			// gewinnen.
			if p.Blocks != "" {
				out["aufbau"] = "bausteine"
				out["hinweis"] = "Diese Seite besteht aus Bausteinen. seite_aendern schreibt " +
					"Markdown und würde die Bausteine ersetzen — bitte nachfragen, bevor du das tust."
			} else {
				out["aufbau"] = "markdown"
			}
			return out, nil
		},
	}
}

func seiteSuchen(d Deps) Tool {
	return Tool{
		Name:        "seiten_durchsuchen",
		Description: "Sucht in den Seiten einer Website nach Wörtern und liefert die Fundstellen.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "Kennung der Website"},
				"suche":   {Type: "string", Description: "wonach gesucht wird"},
				"anzahl":  {Type: "integer", Description: "höchstens so viele, Vorgabe 20"},
			},
			Required: []string{"website", "suche"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Suche   string `json:"suche"`
				Anzahl  int    `json:"anzahl"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			anzahl := a.Anzahl
			if anzahl <= 0 {
				anzahl = 20
			}
			// Entwürfe dürfen mit: dies ist die Verwaltung, nicht die Website,
			// und ein Assistent, der einen halbfertigen Text überarbeiten soll,
			// muss ihn finden können.
			res, err := d.Pages.SearchPages(c.Ctx, a.Website, a.Suche, true, clampCount(anzahl))
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(res))
			for _, r := range res {
				e := kurz(r.Page)
				e["fundstelle"] = string(r.Snippet)
				out = append(out, e)
			}
			return map[string]any{"treffer": out}, nil
		},
	}
}

func medienAuflisten(d Deps) Tool {
	return Tool{
		Name: "medien_auflisten",
		Description: "Listet die Bilder und Dateien einer Website mit ihrer Adresse und ihrer " +
			"Beschreibung. Die Adresse ist das, was in einen Markdown-Verweis gehört.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "Kennung der Website"},
				"suche":   {Type: "string", Description: "Dateiname oder Beschreibung"},
				"anzahl":  {Type: "integer", Description: "höchstens so viele, Vorgabe 50"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Suche   string `json:"suche"`
				Anzahl  int    `json:"anzahl"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			items, _, err := d.Media.List(c.Ctx, a.Website,
				media.Filter{Query: a.Suche}, 1, clampCount(a.Anzahl))
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(items))
			for _, m := range items {
				e := map[string]any{
					"id": m.ID, "datei": m.OriginalName, "adresse": m.URL(),
					"art": m.MimeType, "beschreibung": m.AltText,
					"markdown": m.MarkdownRef(),
				}
				if m.NeedsAltText() {
					e["hinweis"] = "Diesem Bild fehlt eine Beschreibung. " +
						"Wer es einbaut, sollte eine schreiben."
				}
				out = append(out, e)
			}
			return map[string]any{"medien": out}, nil
		},
	}
}

func felderAuflisten(d Deps) Tool {
	return Tool{
		Name: "felder_auflisten",
		Description: "Listet die eigenen Felder einer Website — was diese Website über Titel und Text " +
			"hinaus an einer Seite kennt, etwa Preis oder Verfügbarkeit. Vor dem ersten Schreiben " +
			"aufrufen: nur so weiß man, welche Angaben eine Seite hier tragen kann.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "Kennung der Website"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			if d.Fields == nil {
				return map[string]any{"felder": []any{}}, nil
			}
			defs, err := d.Fields.List(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(defs))
			for _, def := range defs {
				e := map[string]any{
					"kennung": def.Key, "beschriftung": def.Label,
					"art": def.Kind, "pflicht": def.Required, "gilt_fuer": def.AppliesTo,
				}
				if def.Hint != "" {
					e["hinweis"] = def.Hint
				}
				if def.Condition != "" {
					// Sonst schreibt ein Assistent in ein Feld, das niemand
					// sieht, und wundert sich, dass die Seite es nicht zeigt.
					e["nur_wenn_ausgefuellt"] = def.Condition
				}
				if len(def.Choices) > 0 {
					e["auswahl"] = def.Choices
				}
				if def.IsGroup() {
					sub := make([]map[string]any, 0, len(def.Sub))
					for _, s := range def.Sub {
						se := map[string]any{
							"kennung": s.Key, "beschriftung": s.Label,
							"art": s.Kind, "pflicht": s.Required,
						}
						if len(s.Choices) > 0 {
							se["auswahl"] = s.Choices
						}
						sub = append(sub, se)
					}
					e["unterfelder"] = sub
				}
				out = append(out, e)
			}
			return map[string]any{"felder": out}, nil
		},
	}
}

// --- schreiben --------------------------------------------------------------

func seiteAnlegen(d Deps) Tool {
	return Tool{
		Name:   "seite_anlegen",
		Writes: true,
		Description: "Legt eine neue Seite an. Sie entsteht immer als Entwurf und ist erst " +
			"öffentlich, wenn sie jemand veröffentlicht.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website":  {Type: "integer", Description: "Kennung der Website"},
				"titel":    {Type: "string", Description: "Titel der Seite"},
				"markdown": {Type: "string", Description: "Inhalt in Markdown"},
				"adresse":  {Type: "string", Description: "Adresse ohne Schrägstrich; sonst aus dem Titel"},
				"art": {Type: "string", Description: "seite oder beitrag",
					Enum: []string{"seite", "beitrag"}},
				"felder": {Type: "object", Description: "die eigenen Felder dieser Website, " +
					"als {\"kennung\": \"wert\"}; welche es gibt, sagt felder_auflisten"},
				"gruppen": {Type: "object", Description: "die wiederholbaren Gruppen, als " +
					"{\"kennung\": [{\"unterfeld\": \"wert\"}, …]}"},
				"sprache": {Type: "string", Description: "Sprachkürzel wie fr, wenn die Website " +
					"mehrsprachig ist; leer heißt Hauptsprache"},
				"uebersetzung_von": {Type: "integer", Description: "Kennung der Seite in der " +
					"Hauptsprache, zu der diese Fassung gehört"},
			},
			Required: []string{"website", "titel", "markdown"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64                     `json:"website"`
				Titel     string                    `json:"titel"`
				Markdown  string                    `json:"markdown"`
				Adresse   string                    `json:"adresse"`
				Art       string                    `json:"art"`
				Felder    field.Values              `json:"felder"`
				Gruppen   map[string][]field.Values `json:"gruppen"`
				Sprache   string                    `json:"sprache"`
				GehoertZu int64                     `json:"uebersetzung_von"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			if strings.TrimSpace(a.Titel) == "" {
				return nil, errors.New("die Seite braucht einen Titel")
			}

			html, err := page.RenderMarkdown(a.Markdown)
			if err != nil {
				return nil, fmt.Errorf("der Text lässt sich nicht ausgeben: %w", err)
			}
			slug := strings.TrimSpace(strings.Trim(a.Adresse, "/"))
			if slug == "" {
				slug = page.Slugify(a.Titel)
			}

			felder, reason, err := pruefeFelder(c, d, a.Website, artZuKind(a.Art),
				field.Data{Values: a.Felder, Rows: a.Gruppen})
			if err != nil {
				return nil, err
			}
			if reason != "" {
				return nil, errors.New(reason)
			}

			created, err := d.Pages.CreatePage(c.Ctx, page.PageCreate{
				WebsiteID: a.Website,
				Title:     strings.TrimSpace(a.Titel),
				Slug:      slug,
				Markdown:  a.Markdown,
				HTML:      html,
				// Immer ein Entwurf. Ein Assistent, der versehentlich
				// veröffentlicht, stellt etwas Halbfertiges ins Netz, und das
				// merkt der Betreiber erst, wenn jemand es gelesen hat.
				Status: "draft",
				Fields: felder,
				Meta:   page.PageMeta{Excerpt: page.Excerpt(a.Markdown)},
				Kind:   artZuKind(a.Art),
			})
			if err != nil {
				return nil, err
			}
			c.Log.Info("ai created page", "key", c.Scope.Name, "page", created.ID, "website", a.Website)

			if hinweis := d.setzeSprache(c, a.Website, created.ID, a.Sprache, a.GehoertZu); hinweis != "" {
				out := kurz(*created)
				out["hinweis"] = hinweis
				return out, nil
			}

			out := kurz(*created)
			out["hinweis"] = "Als Entwurf angelegt. Zum Veröffentlichen seite_veroeffentlichen aufrufen."
			return out, nil
		},
	}
}

func seiteAendern(d Deps) Tool {
	return Tool{
		Name:   "seite_aendern",
		Writes: true,
		Description: "Ändert Titel oder Text einer Seite. Der bisherige Stand bleibt als Fassung " +
			"erhalten und lässt sich in der Verwaltung zurückholen. Am Zustand ändert sich nichts: " +
			"eine veröffentlichte Seite bleibt veröffentlicht.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":       {Type: "integer", Description: "Kennung der Seite"},
				"titel":    {Type: "string", Description: "neuer Titel; ohne Angabe bleibt der alte"},
				"markdown": {Type: "string", Description: "neuer Inhalt; ohne Angabe bleibt der alte"},
				"felder": {Type: "object", Description: "die eigenen Felder dieser Website, " +
					"als {\"kennung\": \"wert\"}; ohne Angabe bleiben die bisherigen"},
				"gruppen": {Type: "object", Description: "wiederholbare Gruppen als " +
					"{\"kennung\": [{\"unterfeld\": \"wert\"}, …]}; eine angegebene Gruppe " +
					"ersetzt ihre bisherigen Zeilen vollständig"},
			},
			Required: []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID        int64                     `json:"id"`
				Titel     *string                   `json:"titel"`
				Markdown  *string                   `json:"markdown"`
				Felder    field.Values              `json:"felder"`
				Gruppen   map[string][]field.Values `json:"gruppen"`
				Sprache   string                    `json:"sprache"`
				GehoertZu int64                     `json:"uebersetzung_von"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := d.Pages.GetPage(c.Ctx, a.ID)
			if err != nil || p == nil {
				return nil, errors.New("diese Seite gibt es nicht")
			}
			if err := c.Scope.MaySee(p.WebsiteID); err != nil {
				return nil, err
			}
			if p.Blocks != "" && a.Markdown != nil {
				return nil, errors.New("diese Seite besteht aus Bausteinen; " +
					"Markdown hineinzuschreiben würde sie ersetzen. Bitte in der Verwaltung ändern.")
			}

			titel, markdown := p.Title, p.ContentMarkdown
			if a.Titel != nil {
				titel = strings.TrimSpace(*a.Titel)
			}
			if a.Markdown != nil {
				markdown = *a.Markdown
			}
			if strings.TrimSpace(titel) == "" {
				return nil, errors.New("die Seite braucht einen Titel")
			}

			html := p.ContentHTML
			if a.Markdown != nil {
				if html, err = page.RenderMarkdown(markdown); err != nil {
					return nil, fmt.Errorf("der Text lässt sich nicht ausgeben: %w", err)
				}
			}

			// Angegebene Felder ergänzen die bisherigen, statt sie zu ersetzen:
			// wer den Preis ändert, will nicht die Verfügbarkeit verlieren.
			gespeicherte := field.Decode(p.Fields)
			for key, val := range a.Felder {
				gespeicherte.Values[key] = val
			}
			// Eine angegebene Gruppe ersetzt ihre Zeilen ganz: eine Liste
			// zeilenweise zu ergänzen hiesse zu raten, welche Zeile gemeint ist.
			for key, rows := range a.Gruppen {
				gespeicherte.Rows[key] = rows
			}
			felder, reason, ferr := pruefeFelder(c, d, p.WebsiteID, p.Kind, gespeicherte)
			if ferr != nil {
				return nil, ferr
			}
			if reason != "" {
				return nil, errors.New(reason)
			}

			err = d.Pages.UpdatePage(c.Ctx, p.ID, page.PageUpdate{
				Title: titel, Slug: p.Slug, Markdown: markdown, HTML: html,
				Blocks: p.Blocks, Fields: felder,
				// Der Zustand bleibt: Ändern ist nicht Veröffentlichen, und
				// ein Assistent, der beim Korrigieren eines Entwurfs die Seite
				// live schaltet, ist genau das, was niemand will.
				Status:          p.Status,
				Meta:            page.PageMeta{Excerpt: p.Excerpt, MetaDescription: p.MetaDescription, FeaturedMediaID: p.FeaturedMediaID, NoIndex: p.NoIndex},
				Schedule:        page.PageSchedule{PublishAt: p.PublishAt, UnpublishAt: p.UnpublishAt},
				Kind:            p.Kind,
				TypeKey:         p.TypeKey,
				ExpectedVersion: p.Version,
			})
			if err != nil {
				if errors.Is(err, page.ErrConflict) {
					return nil, errors.New("die Seite wurde inzwischen von jemand anderem gespeichert; " +
						"bitte noch einmal lesen und dann erneut ändern")
				}
				return nil, err
			}
			c.Log.Info("ai updated page", "key", c.Scope.Name, "page", p.ID)

			nach, _ := d.Pages.GetPage(c.Ctx, p.ID)
			return kurz(*nach), nil
		},
	}
}

func seiteVeroeffentlichen(d Deps) Tool {
	return Tool{
		Name:   "seite_veroeffentlichen",
		Writes: true,
		Description: "Schaltet eine Seite öffentlich oder zurück auf Entwurf. Rufe das nur auf, " +
			"wenn ausdrücklich darum gebeten wurde.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id": {Type: "integer", Description: "Kennung der Seite"},
				"zustand": {Type: "string", Description: "veroeffentlicht oder entwurf",
					Enum: []string{"veroeffentlicht", "entwurf"}},
			},
			Required: []string{"id", "zustand"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64  `json:"id"`
				Zustand string `json:"zustand"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := d.Pages.GetPage(c.Ctx, a.ID)
			if err != nil || p == nil {
				return nil, errors.New("diese Seite gibt es nicht")
			}
			if err := c.Scope.MaySee(p.WebsiteID); err != nil {
				return nil, err
			}

			status := "draft"
			if a.Zustand == "veroeffentlicht" {
				status = "published"
			}
			if err := d.Pages.SetPageStatus(c.Ctx, p.ID, status, nil); err != nil {
				return nil, err
			}
			c.Log.Info("ai changed page status", "key", c.Scope.Name, "page", p.ID, "status", status)

			nach, _ := d.Pages.GetPage(c.Ctx, p.ID)
			return kurz(*nach), nil
		},
	}
}

// --- gemeinsam --------------------------------------------------------------

// pruefeFelder validates the website's own fields and encodes them for storage.
//
// The same check the admin form runs, for the same reason: a price that is not
// a number has to be refused here too, or an assistant becomes the way around
// every rule an editor has to follow.
func pruefeFelder(c Call, d Deps, websiteID int64, pageKind string, daten field.Data) (string, string, error) {
	if d.Fields == nil || daten.Empty() {
		return "", "", nil
	}
	defs, err := d.Fields.List(c.Ctx, websiteID)
	if err != nil {
		return "", "", err
	}
	mine := field.For(defs, pageKind)
	for _, reason := range field.CheckAll(mine, daten) {
		return "", reason, nil
	}
	raw, err := field.Encode(field.Clean(mine, daten))
	return raw, "", err
}

// findePage löst die beiden Wege auf, eine Seite zu benennen.
func findePage(c Call, d Deps) (*page.Page, error) {
	var a struct {
		ID      int64  `json:"id"`
		Website int64  `json:"website"`
		Adresse string `json:"adresse"`
	}
	if err := c.Into(&a); err != nil {
		return nil, err
	}

	var p *page.Page
	var err error
	switch {
	case a.ID > 0:
		p, err = d.Pages.GetPage(c.Ctx, a.ID)
	case a.Website > 0 && a.Adresse != "":
		p, err = d.Pages.GetPageBySlug(c.Ctx, a.Website, strings.Trim(a.Adresse, "/"))
	default:
		return nil, errors.New("entweder id oder website und adresse angeben")
	}
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("diese Seite gibt es nicht")
	}
	if err := c.Scope.MaySee(p.WebsiteID); err != nil {
		return nil, err
	}
	return p, nil
}

// kurz ist eine Seite ohne ihren Text — was in eine Liste gehört und was nach
// einer Änderung zurückkommt.
// setzeSprache files a freshly created page under its language.
//
// Anything the website does not have becomes the main language and is said out
// loud: an assistant that writes a French page onto a German-only site should
// be told, not left with a page that quietly never appears.
func (d Deps) setzeSprache(c Call, websiteID, pageID int64, sprache string, gehoertZu int64) string {
	if strings.TrimSpace(sprache) == "" && gehoertZu == 0 {
		return ""
	}
	ws, err := d.Domains.GetWebsite(c.Ctx, websiteID)
	if err != nil || ws == nil {
		return ""
	}
	tag := locale.Pick(sprache, ws.Locales())
	if tag == "" && strings.TrimSpace(sprache) != "" {
		return "Als Entwurf angelegt – aber in der Hauptsprache: die Sprache " +
			strings.TrimSpace(sprache) + " ist auf dieser Website nicht eingeschaltet."
	}
	if tag == "" {
		gehoertZu = 0
	}
	if err := d.Pages.SetTranslation(c.Ctx, pageID, tag, gehoertZu); err != nil {
		return "Als Entwurf angelegt, die Sprache liess sich aber nicht setzen: " + err.Error()
	}
	return ""
}

func kurz(p page.Page) map[string]any {
	out := map[string]any{
		"id": p.ID, "website": p.WebsiteID, "titel": p.Title, "adresse": p.Slug,
		// Eine eigene Inhaltsart steht mit ihrer Kennung da: "seite" wäre für ein
		// Produkt zwar technisch richtig und für einen Assistenten irreführend.
		"zustand": zustandVon(p.Status), "art": artVon(p),
		"geaendert": p.UpdatedAt.UTC().Format(timeLayout),
	}
	if p.PublishedAt != nil {
		out["veroeffentlicht_am"] = p.PublishedAt.UTC().Format(timeLayout)
	}
	// Nur wenn es etwas zu sagen gibt: auf einer einsprachigen Website wäre
	// "sprache": "" bei jeder Seite ein Feld, das nichts unterscheidet.
	if p.Locale != "" {
		out["sprache"] = p.Locale
	}
	if p.TranslationOf != 0 {
		out["uebersetzung_von"] = p.TranslationOf
	}
	return out
}

func zustandVon(status string) string {
	if status == "published" {
		return "veroeffentlicht"
	}
	return "entwurf"
}

func kindZuArt(kind string) string {
	if kind == page.KindPost {
		return "beitrag"
	}
	return "seite"
}

func artZuKind(art string) string {
	if art == "beitrag" {
		return page.KindPost
	}
	return page.KindPage
}

// clampCount bounds a list. An assistant that asks for everything would
// otherwise pull a whole site through the server's memory and into a context
// window.
func clampCount(n int) int {
	if n <= 0 {
		return 50
	}
	if n > 200 {
		return 200
	}
	return n
}

// artVon ist die Art eines Eintrags, wie ein Assistent sie lesen soll: die
// Kennung der eigenen Inhaltsart, sonst "seite" oder "beitrag".
func artVon(p page.Page) string {
	if p.TypeKey != "" {
		return p.TypeKey
	}
	return kindZuArt(p.Kind)
}
