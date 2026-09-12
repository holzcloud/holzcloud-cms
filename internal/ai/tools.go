package ai

import (
	"errors"
	"fmt"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
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
		listWebsites(d),
		listPages(d),
		readPage(d),
		searchPages(d),
		listMedia(d),
		listFields(d),
		createPage(d),
		changePage(d),
		publishPage(d),
	}
}

// --- reading ----------------------------------------------------------------

func listWebsites(d Deps) Tool {
	return Tool{
		Name: "list_websites",
		Description: "Lists the websites of this installation with their id, name and locale. " +
			"The first call to make when it is not clear which website is meant.",
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
					"id": ws.ID, "name": ws.Name, "description": ws.Description,
					"active": ws.Active, "language": ws.Locale,
				})
			}
			return map[string]any{"websites": out}, nil
		},
	}
}

func listPages(d Deps) Tool {
	return Tool{
		Name: "list_pages",
		Description: "Lists the pages of a website with their id, title, slug and status. " +
			"Without the body — read_page fetches that.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"status": {Type: "string", Description: "draft, published or all",
					Enum: []string{"draft", "published", "all"}},
				"limit": {Type: "integer", Description: "at most this many, 50 by default"},
				"language": {Type: "string", Description: "a language tag such as fr; leave " +
					"empty for all languages, \"main\" for the main language"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64  `json:"website"`
				Status   string `json:"status"`
				Count    int    `json:"limit"`
				Language string `json:"language"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			// Without a value, all languages: a list that silently shows only
			// the main language looks complete and is not.
			filter := page.ListFilter{Locale: "*", Page: 1, PerPage: clampCount(a.Count)}
			switch strings.TrimSpace(a.Language) {
			case "":
				// all of them
			case "main":
				filter.Locale = ""
			default:
				filter.Locale = locale.Normalise(a.Language)
			}
			switch a.Status {
			case "draft":
				filter.Status = "draft"
			case "published":
				filter.Status = "published"
			}

			pages, total, err := d.Pages.ListPages(c.Ctx, a.Website, filter)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(pages))
			for _, p := range pages {
				out = append(out, brief(p))
			}
			return map[string]any{"pages": out, "total": total}, nil
		},
	}
}

func readPage(d Deps) Tool {
	return Tool{
		Name: "read_page",
		Description: "Fetches a page with its complete body. Either by its id or by " +
			"website and slug.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":      {Type: "integer", Description: "id of the page"},
				"website": {Type: "integer", Description: "id of the website, together with slug"},
				"slug":    {Type: "string", Description: "slug of the page, without a leading slash"},
			},
		},
		Run: func(c Call) (any, error) {
			p, err := findPage(c, d)
			if err != nil {
				return nil, err
			}
			out := brief(*p)
			out["markdown"] = p.ContentMarkdown
			out["excerpt"] = p.Excerpt
			data := field.Decode(p.Fields)
			if len(data.Values) > 0 {
				out["fields"] = map[string]string(data.Values)
			}
			if len(data.Rows) > 0 {
				out["groups"] = data.Rows
			}
			// A page made of blocks has no markdown that could sensibly be
			// written back. That has to be stated, or an assistant writes its
			// text into it and is surprised that the blocks win.
			if p.Blocks != "" {
				out["built_from"] = "blocks"
				out["note"] = "This page is made of blocks. update_page writes markdown and " +
					"would replace the blocks — please ask before you do that."
			} else {
				out["built_from"] = "markdown"
			}
			return out, nil
		},
	}
}

func searchPages(d Deps) Tool {
	return Tool{
		Name:        "search_pages",
		Description: "Searches the pages of a website for words and hands back the matches.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"query":   {Type: "string", Description: "what is searched for"},
				"limit":   {Type: "integer", Description: "at most this many, 20 by default"},
			},
			Required: []string{"website", "query"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Query   string `json:"query"`
				Count   int    `json:"limit"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			count := a.Count
			if count <= 0 {
				count = 20
			}
			// Drafts may come along: this is the admin side, not the website,
			// and an assistant asked to rework a half-finished text has to be
			// able to find it.
			res, err := d.Pages.SearchPages(c.Ctx, a.Website, a.Query, true, clampCount(count))
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(res))
			for _, r := range res {
				e := brief(r.Page)
				e["match"] = string(r.Snippet)
				out = append(out, e)
			}
			return map[string]any{"matches": out}, nil
		},
	}
}

func listMedia(d Deps) Tool {
	return Tool{
		Name: "list_media",
		Description: "Lists the images and files of a website with their URL and their " +
			"description. The URL is what belongs inside a markdown reference.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"query":   {Type: "string", Description: "file name or description"},
				"limit":   {Type: "integer", Description: "at most this many, 50 by default"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Query   string `json:"query"`
				Count   int    `json:"limit"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			items, _, err := d.Media.List(c.Ctx, a.Website,
				media.Filter{Query: a.Query}, 1, clampCount(a.Count))
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(items))
			for _, m := range items {
				e := map[string]any{
					"id": m.ID, "file": m.OriginalName, "url": m.URL(),
					"mime": m.MimeType, "description": m.AltText,
					"markdown": m.MarkdownRef(),
				}
				if m.NeedsAltText() {
					e["note"] = "This image has no description. " +
						"Whoever puts it into a page should write one."
				}
				out = append(out, e)
			}
			return map[string]any{"media": out}, nil
		},
	}
}

func listFields(d Deps) Tool {
	return Tool{
		Name: "list_fields",
		Description: "Lists a website's own fields — what this website knows about a page beyond " +
			"title and body, a price or an availability say. Call it before the first write: it is " +
			"the only way to learn which particulars a page can carry here.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
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
				return map[string]any{"fields": []any{}}, nil
			}
			defs, err := d.Fields.List(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(defs))
			for _, def := range defs {
				e := map[string]any{
					"key": def.Key, "label": def.Label,
					"kind": def.Kind, "required": def.Required, "applies_to": def.AppliesTo,
				}
				if def.Hint != "" {
					e["hint"] = def.Hint
				}
				if def.Condition != "" {
					// Otherwise an assistant writes into a field nobody sees
					// and is surprised that the page does not show it.
					e["only_if_filled"] = def.Condition
				}
				if len(def.Choices) > 0 {
					e["choices"] = def.Choices
				}
				fieldProperties(e, def)
				if def.IsGroup() {
					sub := make([]map[string]any, 0, len(def.Sub))
					for _, s := range def.Sub {
						se := map[string]any{
							"key": s.Key, "label": s.Label,
							"kind": s.Kind, "required": s.Required,
						}
						if len(s.Choices) > 0 {
							se["choices"] = s.Choices
						}
						// The same particulars one level down, out of the same
						// place: an assistant that learns less about a subfield
						// is an assistant that writes into exactly that one
						// wrongly.
						fieldProperties(se, s)
						sub = append(sub, se)
					}
					e["subfields"] = sub
				}
				out = append(out, e)
			}
			return map[string]any{"fields": out}, nil
		},
	}
}

// fieldProperties enters into the description of a field whatever an assistant
// needs in order to write a value that will actually be accepted: the
// presentation, the maximum number of values, the two bounds of a number — and,
// for a multi-valued field, how several values are written into the one string
// that the writing tools take. The kind itself already stands under "kind"; what
// is added here is the shape of an acceptable value.
//
// One place for the field on the page and for the subfield inside a group, so
// that the two cannot drift apart.
//
// Every entry is absent where there is none, and that is not thrift: a reported
// zero would read as "none allowed" where it means "no limit", and a reported
// empty bound as "the bound is empty". "No bound" and "the bound is zero" are
// two different facts — the same reason the two columns were given a text type
// in 07-02.
//
// These particulars are read by a machine that acts on them: an ambiguous
// wording here is a wrongly written page over there.
func fieldProperties(e map[string]any, d field.Def) {
	if d.Display != "" {
		e["presentation"] = d.Display
	}
	if d.MaxValues > 0 {
		e["max_values"] = d.MaxValues
	}
	if d.RangeMin != "" {
		e["min_value"] = d.RangeMin
	}
	if d.RangeMax != "" {
		e["max_value"] = d.RangeMax
	}
	if d.IsMultiValued() {
		e["multiple_values"] = "Several values stand inside the same string, one per " +
			"line, separated by a line break; only the options named under choices " +
			"are allowed."
	}
}

// --- writing ----------------------------------------------------------------

func createPage(d Deps) Tool {
	return Tool{
		Name:   "create_page",
		Writes: true,
		Description: "Creates a new page. It is always born a draft and is public only once " +
			"somebody publishes it.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website":  {Type: "integer", Description: "id of the website"},
				"title":    {Type: "string", Description: "title of the page"},
				"markdown": {Type: "string", Description: "body in markdown"},
				"slug":     {Type: "string", Description: "slug without a slash; otherwise from the title"},
				"type": {Type: "string", Description: "page or post",
					Enum: []string{"page", "post"}},
				"fields": {Type: "object", Description: "this website's own fields, as " +
					"{\"key\": \"value\"}; list_fields says which ones there are"},
				"groups": {Type: "object", Description: "the repeatable groups, as " +
					"{\"key\": [{\"subfield\": \"value\"}, …]}"},
				"language": {Type: "string", Description: "a language tag such as fr, when the " +
					"website is multilingual; empty means the main language"},
				"translation_of": {Type: "integer", Description: "id of the page in the main " +
					"language this version belongs to"},
			},
			Required: []string{"website", "title", "markdown"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64                     `json:"website"`
				Title     string                    `json:"title"`
				Markdown  string                    `json:"markdown"`
				Address   string                    `json:"slug"`
				Type      string                    `json:"type"`
				Fields    field.Values              `json:"fields"`
				Groups    map[string][]field.Values `json:"groups"`
				Language  string                    `json:"language"`
				BelongsTo int64                     `json:"translation_of"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			if strings.TrimSpace(a.Title) == "" {
				return nil, errors.New("the page needs a title")
			}

			html, err := page.RenderMarkdown(a.Markdown)
			if err != nil {
				return nil, fmt.Errorf("the text cannot be rendered: %w", err)
			}
			slug := strings.TrimSpace(strings.Trim(a.Address, "/"))
			if slug == "" {
				slug = page.Slugify(a.Title)
			}

			fields, reason, err := checkFields(c, d, a.Website, kindFromWire(a.Type),
				field.Data{Values: a.Fields, Rows: a.Groups})
			if err != nil {
				return nil, err
			}
			if reason != "" {
				return nil, errors.New(reason)
			}

			created, err := d.Pages.CreatePage(c.Ctx, page.PageCreate{
				WebsiteID: a.Website,
				Title:     strings.TrimSpace(a.Title),
				Slug:      slug,
				Markdown:  a.Markdown,
				HTML:      html,
				// Always a draft. An assistant that publishes by accident puts
				// something half-finished on the net, and the operator notices
				// only once somebody has read it.
				Status: "draft",
				Fields: fields,
				Meta:   page.PageMeta{Excerpt: page.Excerpt(a.Markdown)},
				Kind:   kindFromWire(a.Type),
			})
			if err != nil {
				return nil, err
			}
			c.Log.Info("ai created page", "key", c.Scope.Name, "page", created.ID, "website", a.Website)

			if note := d.setLanguage(c, a.Website, created.ID, a.Language, a.BelongsTo); note != "" {
				out := brief(*created)
				out["note"] = note
				return out, nil
			}

			out := brief(*created)
			out["note"] = "Created as a draft. Call publish_page to publish it."
			return out, nil
		},
	}
}

func changePage(d Deps) Tool {
	return Tool{
		Name:   "update_page",
		Writes: true,
		Description: "Changes the title or the body of a page. The previous state is kept as a " +
			"revision and can be fetched back from the admin side. The status does not change: " +
			"a published page stays published.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":       {Type: "integer", Description: "id of the page"},
				"title":    {Type: "string", Description: "new title; without one the old stays"},
				"markdown": {Type: "string", Description: "new body; without one the old stays"},
				"fields": {Type: "object", Description: "this website's own fields, as " +
					"{\"key\": \"value\"}; without them the existing ones stay"},
				"groups": {Type: "object", Description: "repeatable groups as " +
					"{\"key\": [{\"subfield\": \"value\"}, …]}; a group that is given " +
					"replaces its existing rows entirely"},
			},
			Required: []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID       int64                     `json:"id"`
				Title    *string                   `json:"title"`
				Markdown *string                   `json:"markdown"`
				Fields   field.Values              `json:"fields"`
				Groups   map[string][]field.Values `json:"groups"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := d.Pages.GetPage(c.Ctx, a.ID)
			if err != nil || p == nil {
				return nil, errors.New("there is no such page")
			}
			if err := c.Scope.MaySee(p.WebsiteID); err != nil {
				return nil, err
			}
			if p.Blocks != "" && a.Markdown != nil {
				return nil, errors.New("this page is made of blocks; writing markdown into it " +
					"would replace them. Please change it on the admin side.")
			}

			title, markdown := p.Title, p.ContentMarkdown
			if a.Title != nil {
				title = strings.TrimSpace(*a.Title)
			}
			if a.Markdown != nil {
				markdown = *a.Markdown
			}
			if strings.TrimSpace(title) == "" {
				return nil, errors.New("the page needs a title")
			}

			html := p.ContentHTML
			if a.Markdown != nil {
				if html, err = page.RenderMarkdown(markdown); err != nil {
					return nil, fmt.Errorf("the text cannot be rendered: %w", err)
				}
			}

			// Given fields add to the existing ones instead of replacing them:
			// whoever changes the price does not want to lose the availability.
			stored := field.Decode(p.Fields)
			for key, val := range a.Fields {
				stored.Values[key] = val
			}
			// A given group replaces its rows entirely: adding to a list row by
			// row would mean guessing which row is meant.
			for key, rows := range a.Groups {
				stored.Rows[key] = rows
			}
			fields, reason, ferr := checkFields(c, d, p.WebsiteID, p.Kind, stored)
			if ferr != nil {
				return nil, ferr
			}
			if reason != "" {
				return nil, errors.New(reason)
			}

			err = d.Pages.UpdatePage(c.Ctx, p.ID, page.PageUpdate{
				Title: title, Slug: p.Slug, Markdown: markdown, HTML: html,
				Blocks: p.Blocks, Fields: fields,
				// The status stays: changing is not publishing, and an
				// assistant that puts a page live while correcting a draft is
				// exactly what nobody wants.
				Status:          p.Status,
				Meta:            page.PageMeta{Excerpt: p.Excerpt, MetaDescription: p.MetaDescription, FeaturedMediaID: p.FeaturedMediaID, NoIndex: p.NoIndex},
				Schedule:        page.PageSchedule{PublishAt: p.PublishAt, UnpublishAt: p.UnpublishAt},
				Kind:            p.Kind,
				TypeKey:         p.TypeKey,
				ExpectedVersion: p.Version,
			})
			if err != nil {
				if errors.Is(err, page.ErrConflict) {
					return nil, errors.New("somebody else has saved the page in the meantime; " +
						"please read it again and then change it once more")
				}
				return nil, err
			}
			c.Log.Info("ai updated page", "key", c.Scope.Name, "page", p.ID)

			after, _ := d.Pages.GetPage(c.Ctx, p.ID)
			return brief(*after), nil
		},
	}
}

func publishPage(d Deps) Tool {
	return Tool{
		Name:   "publish_page",
		Writes: true,
		Description: "Puts a page public or back to draft. Call this only when you were " +
			"expressly asked to.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id": {Type: "integer", Description: "id of the page"},
				"status": {Type: "string", Description: "published or draft",
					Enum: []string{"published", "draft"}},
			},
			Required: []string{"id", "status"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			p, err := d.Pages.GetPage(c.Ctx, a.ID)
			if err != nil || p == nil {
				return nil, errors.New("there is no such page")
			}
			if err := c.Scope.MaySee(p.WebsiteID); err != nil {
				return nil, err
			}

			status := "draft"
			if a.Status == "published" {
				status = "published"
			}
			if err := d.Pages.SetPageStatus(c.Ctx, p.ID, status, nil); err != nil {
				return nil, err
			}
			c.Log.Info("ai changed page status", "key", c.Scope.Name, "page", p.ID, "status", status)

			after, _ := d.Pages.GetPage(c.Ctx, p.ID)
			return brief(*after), nil
		},
	}
}

// --- shared -----------------------------------------------------------------

// checkFields validates the website's own fields and encodes them for storage.
//
// The same check the admin form runs, for the same reason: a price that is not
// a number has to be refused here too, or an assistant becomes the way around
// every rule an editor has to follow.
func checkFields(c Call, d Deps, websiteID int64, pageKind string, data field.Data) (string, string, error) {
	if d.Fields == nil || data.Empty() {
		return "", "", nil
	}
	defs, err := d.Fields.List(c.Ctx, websiteID)
	if err != nil {
		return "", "", err
	}
	mine := field.For(defs, pageKind)
	for _, reason := range field.CheckAll(mine, data) {
		return "", reason.Text(i18n.Lang(c.Ctx)), nil
	}
	raw, err := field.Encode(field.Clean(mine, data))
	return raw, "", err
}

// findPage resolves the two ways of naming a page.
func findPage(c Call, d Deps) (*page.Page, error) {
	var a struct {
		ID      int64  `json:"id"`
		Website int64  `json:"website"`
		Address string `json:"slug"`
	}
	if err := c.Into(&a); err != nil {
		return nil, err
	}

	var p *page.Page
	var err error
	switch {
	case a.ID > 0:
		p, err = d.Pages.GetPage(c.Ctx, a.ID)
	case a.Website > 0 && a.Address != "":
		p, err = d.Pages.GetPageBySlug(c.Ctx, a.Website, strings.Trim(a.Address, "/"))
	default:
		return nil, errors.New("give either id, or website and slug")
	}
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("there is no such page")
	}
	if err := c.Scope.MaySee(p.WebsiteID); err != nil {
		return nil, err
	}
	return p, nil
}

// setLanguage files a freshly created page under its language.
//
// Anything the website does not have becomes the main language and is said out
// loud: an assistant that writes a French page onto a German-only site should
// be told, not left with a page that quietly never appears.
func (d Deps) setLanguage(c Call, websiteID, pageID int64, language string, belongsTo int64) string {
	if strings.TrimSpace(language) == "" && belongsTo == 0 {
		return ""
	}
	ws, err := d.Domains.GetWebsite(c.Ctx, websiteID)
	if err != nil || ws == nil {
		return ""
	}
	tag := locale.Pick(language, ws.Locales())
	if tag == "" && strings.TrimSpace(language) != "" {
		return "Created as a draft – but in the main language: the language " +
			strings.TrimSpace(language) + " is not switched on for this website."
	}
	if tag == "" {
		belongsTo = 0
	}
	if err := d.Pages.SetTranslation(c.Ctx, websiteID, pageID, tag, belongsTo); err != nil {
		return "Created as a draft, but the language could not be set: " + err.Error()
	}
	return ""
}

// brief is a page without its body — what belongs in a list and what comes
// back after a change.
func brief(p page.Page) map[string]any {
	out := map[string]any{
		"id": p.ID, "website": p.WebsiteID, "title": p.Title, "slug": p.Slug,
		// A content type of its own stands there with its own key: "page"
		// would be technically right for a product and misleading to an
		// assistant.
		"status": wireStatus(p.Status), "type": wireKindOf(p),
		"updated": p.UpdatedAt.UTC().Format(timeLayout),
	}
	if p.PublishedAt != nil {
		out["published_at"] = p.PublishedAt.UTC().Format(timeLayout)
	}
	// Only when there is something to say: on a single-language website
	// "language": "" on every page would be a field that distinguishes nothing.
	if p.Locale != "" {
		out["language"] = p.Locale
	}
	if p.TranslationOf != 0 {
		out["translation_of"] = p.TranslationOf
	}
	return out
}

// wireStatus, wireKind and kindFromWire are the vocabulary an assistant sees.
// Since 2.0 that vocabulary is the same English as the stored one, so the three
// read almost like identity — they are not. They NORMALISE: anything that is
// not "published" is a draft to a reader, and anything that is not "post" is a
// page. The wire is written by a machine that guesses; the column is not.
func wireStatus(status string) string {
	if status == "published" {
		return "published"
	}
	return "draft"
}

func wireKind(kind string) string {
	if kind == page.KindPost {
		return page.KindPost
	}
	return page.KindPage
}

func kindFromWire(kind string) string {
	if kind == page.KindPost {
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

// wireKindOf is the kind of an entry as an assistant should read it: the key of
// its own content type, otherwise "page" or "post".
func wireKindOf(p page.Page) string {
	if p.TypeKey != "" {
		return p.TypeKey
	}
	return wireKind(p.Kind)
}
