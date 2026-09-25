package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/design"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/structured"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/tmplspec"
)

// Websites, domains, settings, templates, design tokens, wording.
//
// The permission split is the web's: what the admin puts behind requireAdmin —
// creating and deleting a website, its domains, the design tokens, uploading
// and deleting a template, the template specification — is Admin here too. The
// rest a content key may do on its own website, as an editor may on the
// settings, design and wording screens.
//
// Every step beyond a single store call goes through the admin handler (see
// internal/admin/ops_site.go), so a setting an assistant changes meets the
// same validation as the form, and a website it creates gets the same start
// page, legal drafts and menus.

// siteOps are the admin handler's methods these tools use.
type siteOps interface {
	OpCreateWebsite(ctx context.Context, name, description string, starter bool) (*domain.Website, error)
	OpUpdateWebsite(ctx context.Context, id int64, changes map[string]string) (*domain.Website, error)
	OpDeleteWebsite(ctx context.Context, id int64) error
	OpLaunchChecklist(ctx context.Context, websiteID int64) ([]map[string]any, error)
	OpTemplates(ctx context.Context) ([]tmplmgr.Template, map[int64]int64, error)
	OpActivateTemplate(ctx context.Context, websiteID, templateID int64) error
	OpSetDesign(ctx context.Context, websiteID int64, form map[string]string) (design.Tokens, error)
	OpResetDesign(ctx context.Context, websiteID int64) error
	OpWording(ctx context.Context, websiteID int64) (map[string]any, error)
	OpSetWording(ctx context.Context, websiteID int64, locale string, words map[string]string) (int, []string, bool, error)
	OpUploadTemplate(ctx context.Context, name string, archive []byte) (*tmplmgr.Template, error)
	OpDeleteTemplate(ctx context.Context, id int64) error
}

// errNoSiteOps is what a tool says when the build has no admin handler behind
// it — a test build, say. Never a panic.
var errNoSiteOps = errors.New("managing websites is not available on this server")

func siteOpsOf(d Deps) (siteOps, error) {
	ops, ok := d.Ops.(siteOps)
	if !ok {
		return nil, errNoSiteOps
	}
	return ops, nil
}

func siteTools(d Deps) []Tool {
	return []Tool{
		getWebsite(d), updateWebsite(d), createWebsite(d), deleteWebsite(d),
		addDomain(d), removeDomain(d), setPrimaryDomain(d),
		launchChecklist(d), translationMatrix(d),
		listTemplates(d), activateTemplate(d), uploadTemplate(d), deleteTemplate(d),
		getTemplateSpec(),
		getDesign(d), setDesign(d), resetDesign(d),
		getWording(d), setWording(d),
	}
}

// websiteArg is the one argument almost every tool here takes.
var websiteArg = Property{Type: "integer", Description: "id of the website"}

// visibleWebsite reads the website a call names, after asking the key.
func visibleWebsite(c Call, d Deps, id int64) (*domain.Website, error) {
	if err := c.Scope.MaySee(id); err != nil {
		return nil, err
	}
	ws, err := d.Domains.GetWebsite(c.Ctx, id)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, errors.New("there is no such website")
	}
	return ws, nil
}

// --- settings ---------------------------------------------------------------

// settingArgs maps the wire name of each setting to the settings form's field
// and its JSON type. The form's names are what OpUpdateWebsite takes; the wire
// names are what an assistant reads, so they say what they are.
var settingArgs = []struct {
	wire, form, kind, doc string
}{
	{"name", "name", "string", "name of the website"},
	{"description", "description", "string", "a short description, shown in the admin and used as the search fallback"},
	{"active", "active", "boolean", "false takes the website offline (see offline_mode)"},
	{"language", "locale", "string", "main language, one of: " + supportedLanguages()},
	{"extra_languages", "extra_locales", "string", "further languages, comma separated, e.g. \"fr, it\"; empty for one language"},
	{"time_zone", "timezone", "string", "IANA zone such as Europe/Berlin; published dates are shown in it"},
	{"meta_description", "meta_description", "string", "site-wide description for search engines"},
	{"favicon_media_id", "favicon_media_id", "integer", "id of an image of this website (list_media); 0 removes it"},
	{"logo_media_id", "logo_media_id", "integer", "id of an image of this website (list_media); 0 removes it"},
	{"canonical_redirect", "canonical_redirect", "boolean", "send visitors of other domains to the primary domain with a 301"},
	{"confirm_senders", "confirm_senders", "boolean", "tell a person who writes through a form that it arrived (sends e-mail to visitor addresses)"},
	{"offline_mode", "offline_mode", "string", "what an inactive website answers: notfound (404) or maintenance (503)"},
	{"offline_message", "offline_message", "string", "text shown while in maintenance"},
	{"contact_email", "contact_email", "string", "address published beside the contact form"},
	{"notify_email", "notify_email", "string", "address notifications about this website go to (not published)"},
	{"org_type", "org_type", "string", "schema.org type of the business, one of: " + orgTypes() + "; empty for none"},
	{"street", "street", "string", "street and number of the business"},
	{"postal_code", "postal_code", "string", "postal code"},
	{"city", "city", "string", "city"},
	{"country", "country", "string", "two-letter country code, DE when empty"},
	{"phone", "phone", "string", "phone number"},
	{"opening_hours", "opening_hours", "string", "opening hours, as the form takes them"},
	{"blog_base", "blog_base", "string", "address of the post archive without slashes, e.g. aktuelles; empty switches it off"},
	{"posts_per_page", "posts_per_page", "integer", "entries per archive page, 1 to 100"},
}

func supportedLanguages() string {
	codes := make([]string, 0, len(tmpl.SupportedLocales))
	for _, l := range tmpl.SupportedLocales {
		codes = append(codes, l.Code)
	}
	return strings.Join(codes, ", ")
}

func orgTypes() string {
	out := []string{}
	for _, t := range structured.OrgTypes {
		if t.Value != "" {
			out = append(out, t.Value)
		}
	}
	return strings.Join(out, ", ")
}

// websiteView is a website with every setting, under the wire names.
func websiteView(ws *domain.Website) map[string]any {
	id := func(p *int64) any {
		if p == nil {
			return nil
		}
		return *p
	}
	return map[string]any{
		"id": ws.ID, "name": ws.Name, "description": ws.Description, "active": ws.Active,
		"language": ws.Locale, "extra_languages": ws.ExtraLocales, "time_zone": ws.TimeZone,
		"meta_description": ws.MetaDescription,
		"favicon_media_id": id(ws.FaviconMediaID), "logo_media_id": id(ws.LogoMediaID),
		"canonical_redirect": ws.CanonicalRedirect, "confirm_senders": ws.ConfirmSenders,
		"offline_mode": ws.OfflineMode, "offline_message": ws.OfflineMessage,
		"contact_email": ws.ContactEmail, "notify_email": ws.NotifyEmail,
		"org_type": ws.OrgType, "street": ws.Street, "postal_code": ws.PostalCode,
		"city": ws.City, "country": ws.Country, "phone": ws.Phone,
		"opening_hours": ws.OpeningHours, "blog_base": ws.BlogBase,
		"posts_per_page": ws.PostsPerPage,
	}
}

func domainsView(list []domain.Domain) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, dm := range list {
		out = append(out, map[string]any{"id": dm.ID, "domain": dm.Domain, "primary": dm.IsPrimary})
	}
	return out
}

func getWebsite(d Deps) Tool {
	return Tool{
		Name: "get_website",
		Description: "Fetches every setting of a website — name, languages, time zone, favicon and logo, " +
			"contact addresses, business details, archive, offline mode — and its domains. " +
			"update_website takes the same names.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteArg},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			domains, err := d.Domains.ListDomains(c.Ctx, ws.ID)
			if err != nil {
				return nil, err
			}
			out := websiteView(ws)
			out["domains"] = domainsView(domains)
			return out, nil
		},
	}
}

func updateWebsite(d Deps) Tool {
	props := map[string]Property{"website": websiteArg}
	for _, s := range settingArgs {
		p := Property{Type: s.kind, Description: s.doc}
		if s.wire == "offline_mode" {
			p.Enum = []string{"notfound", "maintenance"}
		}
		props[s.wire] = p
	}
	return Tool{
		Name:   "update_website",
		Writes: true,
		Description: "Changes settings of a website. Only the settings given change; every other stays. " +
			"The values go through the settings form's own checks: an unknown language, time zone or " +
			"business type falls back to the default, and the answer lists under \"adjusted\" what was " +
			"not stored as given. Setting active to false takes the website offline — only when asked.",
		InputSchema: Schema{Type: "object", Properties: props, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var raw map[string]json.RawMessage
			if err := c.Into(&raw); err != nil {
				return nil, err
			}
			var id int64
			if err := json.Unmarshal(raw["website"], &id); err != nil || id <= 0 {
				return nil, errors.New("give the id of the website")
			}
			if _, err := visibleWebsite(c, d, id); err != nil {
				return nil, err
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}

			changes := map[string]string{}
			given := map[string]string{}
			for _, s := range settingArgs {
				v, ok := raw[s.wire]
				if !ok {
					continue
				}
				val, err := formValue(s.kind, v)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", s.wire, err)
				}
				changes[s.form] = val
				given[s.wire] = val
			}
			for k := range raw {
				if k != "website" && !knownSetting(k) {
					return nil, fmt.Errorf("there is no setting %q; get_website shows them all", k)
				}
			}
			if len(changes) == 0 {
				return nil, errors.New("nothing to change was given")
			}

			ws, err := ops.OpUpdateWebsite(c.Ctx, id, changes)
			if err != nil {
				return nil, err
			}
			changed(d, c, id, activity.Entry{
				Action: activity.ActionWebsiteUpdate, EntityType: "website", EntityID: id,
				Metadata: map[string]any{"settings": keysOf(given)},
			})

			out := websiteView(ws)
			if adj := adjusted(out, given); len(adj) > 0 {
				out["adjusted"] = adj
			}
			return out, nil
		},
	}
}

func knownSetting(wire string) bool {
	for _, s := range settingArgs {
		if s.wire == wire {
			return true
		}
	}
	return false
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// formValue turns a JSON argument into what the form would have posted.
func formValue(kind string, raw json.RawMessage) (string, error) {
	if string(raw) == "null" {
		return "", nil
	}
	switch kind {
	case "boolean":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return "", errors.New("expected true or false")
		}
		if b {
			return "on", nil
		}
		return "", nil
	case "integer":
		var n int64
		if err := json.Unmarshal(raw, &n); err != nil {
			return "", errors.New("expected a whole number")
		}
		if n <= 0 {
			return "", nil
		}
		return strconv.FormatInt(n, 10), nil
	default:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", errors.New("expected a string")
		}
		return s, nil
	}
}

// adjusted names the settings the form's rules stored differently from how
// they were given. An assistant that asked for "Klingon" and got German should
// know, rather than report success.
func adjusted(stored map[string]any, given map[string]string) map[string]any {
	out := map[string]any{}
	for _, wire := range []string{"language", "time_zone", "org_type", "blog_base", "posts_per_page", "offline_mode"} {
		g, ok := given[wire]
		if !ok {
			continue
		}
		have := fmt.Sprint(stored[wire])
		if strings.TrimSpace(g) != have {
			out[wire] = map[string]any{"given": g, "stored": stored[wire]}
		}
	}
	return out
}

func createWebsite(d Deps) Tool {
	return Tool{
		Name:   "create_website",
		Writes: true,
		Admin:  true,
		Description: "Creates a new website, exactly as the admin's \"new website\" form does: the default " +
			"template is switched on and, unless starter_content is false, it gets a published start " +
			"page, an imprint and a privacy statement as drafts, a main menu and a footer menu. It has " +
			"no domain yet — add_domain gives it one.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"name":        {Type: "string", Description: "name of the website"},
				"description": {Type: "string", Description: "a short description"},
				"starter_content": {Type: "boolean", Description: "false for an empty website " +
					"(for an import that brings its own pages); true by default"},
			},
			Required: []string{"name"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Starter     *bool  `json:"starter_content"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			starter := a.Starter == nil || *a.Starter
			ws, err := ops.OpCreateWebsite(c.Ctx, a.Name, a.Description, starter)
			if err != nil {
				return nil, err
			}
			changed(d, c, ws.ID, activity.Entry{
				Action: activity.ActionWebsiteCreate, EntityType: "website", EntityID: ws.ID,
				Metadata: map[string]any{"name": ws.Name, "starter_content": starter},
			})
			out := websiteView(ws)
			out["note"] = "Created. It has no domain yet; add_domain gives it one."
			return out, nil
		},
	}
}

func deleteWebsite(d Deps) Tool {
	return Tool{
		Name:   "delete_website",
		Writes: true,
		Admin:  true,
		Description: "Deletes a website with all its pages, media files, menus and domains. This cannot be " +
			"undone. Requires confirm: true, and should only be called when the operator asked for exactly " +
			"this website to be deleted.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteArg,
				"confirm": {Type: "boolean", Description: "must be true"},
			},
			Required: []string{"website", "confirm"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Confirm bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errors.New("deleting a website cannot be undone; call again with confirm: true " +
					"if that is really what was asked for")
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteWebsite(c.Ctx, ws.ID); err != nil {
				return nil, err
			}
			// Logged without the website: its row is gone, and the entry would
			// point at nothing. The id and the name stay in the entry itself.
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionWebsiteDelete, EntityType: "website", EntityID: ws.ID,
				Metadata: map[string]any{"name": ws.Name},
			})
			return map[string]any{"deleted": ws.ID, "name": ws.Name}, nil
		},
	}
}

// --- domains ----------------------------------------------------------------

func addDomain(d Deps) Tool {
	return Tool{
		Name:   "add_domain",
		Writes: true,
		Admin:  true,
		Description: "Adds a domain (host name such as example.org) to a website. With primary: true it " +
			"becomes the canonical address, and the previous primary domain stops being one.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteArg,
				"domain":  {Type: "string", Description: "host name, without https:// and without a path"},
				"primary": {Type: "boolean", Description: "make it the primary domain"},
			},
			Required: []string{"website", "domain"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Domain  string `json:"domain"`
				Primary bool   `json:"primary"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(a.Domain) == "" {
				return nil, errors.New("give the domain")
			}
			added, err := d.Domains.AddDomain(c.Ctx, ws.ID, strings.TrimSpace(a.Domain), a.Primary)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
					return nil, errors.New("that domain is already assigned to a website")
				}
				return nil, err
			}
			changed(d, c, ws.ID, activity.Entry{
				Action: activity.ActionDomainAdd, EntityType: "domain", EntityID: added.ID,
				Metadata: map[string]any{"domain": added.Domain, "haupt": a.Primary},
			})
			return domainsAnswer(c, d, ws.ID)
		},
	}
}

func domainsAnswer(c Call, d Deps, websiteID int64) (any, error) {
	list, err := d.Domains.ListDomains(c.Ctx, websiteID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"website": websiteID, "domains": domainsView(list)}, nil
}

// ownDomain finds a domain of a website by its id, so an id from outside
// cannot reach another website's domain.
func ownDomain(c Call, d Deps, websiteID, domainID int64) (*domain.Domain, error) {
	list, err := d.Domains.ListDomains(c.Ctx, websiteID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == domainID {
			return &list[i], nil
		}
	}
	return nil, errors.New("this website has no domain with that id; get_website lists them")
}

var domainIDArg = Property{Type: "integer", Description: "id of the domain, as get_website lists it"}

func removeDomain(d Deps) Tool {
	return Tool{
		Name:   "remove_domain",
		Writes: true,
		Admin:  true,
		Description: "Removes a domain from a website. The website is no longer reachable under it. " +
			"Requires confirm: true.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteArg, "domain_id": domainIDArg,
				"confirm": {Type: "boolean", Description: "must be true"},
			},
			Required: []string{"website", "domain_id", "confirm"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64 `json:"website"`
				DomainID int64 `json:"domain_id"`
				Confirm  bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			dm, err := ownDomain(c, d, ws.ID, a.DomainID)
			if err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errors.New("removing a domain takes the website off that address; " +
					"call again with confirm: true if that is what was asked for")
			}
			if err := d.Domains.RemoveDomain(c.Ctx, ws.ID, dm.ID); err != nil {
				return nil, err
			}
			changed(d, c, ws.ID, activity.Entry{
				Action: activity.ActionDomainRemove, EntityType: "domain", EntityID: dm.ID,
				Metadata: map[string]any{"domain": dm.Domain},
			})
			return domainsAnswer(c, d, ws.ID)
		},
	}
}

func setPrimaryDomain(d Deps) Tool {
	return Tool{
		Name:   "set_primary_domain",
		Writes: true,
		Admin:  true,
		Description: "Makes one of a website's domains the primary one — the canonical address in the " +
			"sitemap and the page source, and the target of canonical_redirect.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteArg, "domain_id": domainIDArg},
			Required:   []string{"website", "domain_id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64 `json:"website"`
				DomainID int64 `json:"domain_id"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			dm, err := ownDomain(c, d, ws.ID, a.DomainID)
			if err != nil {
				return nil, err
			}
			if err := d.Domains.SetPrimaryDomain(c.Ctx, ws.ID, dm.ID); err != nil {
				return nil, err
			}
			changed(d, c, ws.ID, activity.Entry{
				Action: activity.ActionDomainPrimary, EntityType: "domain", EntityID: dm.ID,
				Metadata: map[string]any{"domain": dm.Domain},
			})
			return domainsAnswer(c, d, ws.ID)
		},
	}
}

// --- overviews --------------------------------------------------------------

func launchChecklist(d Deps) Tool {
	return Tool{
		Name: "launch_checklist",
		Description: "The \"ready to launch?\" list of the settings screen: domain, primary domain, a " +
			"published start page, an activated template, the imprint in the footer menu, a description " +
			"for search engines, a favicon. Each item says whether it is done and what to do if not.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteArg},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			checks, err := ops.OpLaunchChecklist(c.Ctx, ws.ID)
			if err != nil {
				return nil, err
			}
			open := 0
			for _, ch := range checks {
				if ok, _ := ch["ok"].(bool); !ok {
					open++
				}
			}
			return map[string]any{"website": ws.ID, "checks": checks, "open": open, "ready": open == 0}, nil
		},
	}
}

func translationMatrix(d Deps) Tool {
	return Tool{
		Name: "translation_matrix",
		Description: "Shows which pages of a website exist in which of its languages, and how many are " +
			"missing per language. create_page with language and translation_of fills a gap.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteArg},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			rows, err := d.Pages.TranslationMatrix(c.Ctx, ws.ID)
			if err != nil {
				return nil, err
			}
			// The main language is stored as "" on a page; the answer names it
			// by its tag, like every other language.
			codes := append([]string{""}, ws.Locales()...)
			tag := func(code string) string {
				if code == "" {
					return ws.Locale
				}
				return code
			}
			missing := map[string]int{}
			pages := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				langs := map[string]any{}
				var gaps []string
				for _, code := range codes {
					cell, ok := row.ByLocale[code]
					if !ok {
						missing[tag(code)]++
						gaps = append(gaps, tag(code))
						continue
					}
					langs[tag(code)] = map[string]any{
						"id": cell.ID, "title": cell.Title, "status": wireStatus(cell.Status),
					}
				}
				e := map[string]any{"id": row.ID, "title": row.Title, "slug": row.Slug, "languages": langs}
				if len(gaps) > 0 {
					e["missing"] = gaps
				}
				pages = append(pages, e)
			}
			columns := make([]map[string]any, 0, len(codes))
			for _, code := range codes {
				columns = append(columns, map[string]any{
					"language": tag(code), "name": locale.Native(tag(code)),
					"main": code == "", "missing": missing[tag(code)],
				})
			}
			return map[string]any{"website": ws.ID, "languages": columns, "pages": pages}, nil
		},
	}
}

// --- templates --------------------------------------------------------------

var templateArg = Property{Type: "integer", Description: "id of the template, as list_templates lists it"}

func listTemplates(d Deps) Tool {
	return Tool{
		Name: "list_templates",
		Description: "Lists the installed templates (themes) with their id, name and whether they are " +
			"built in. With a website, says which one is active there.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of a website, to see which template is active there"},
			},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if a.Website > 0 {
				if _, err := visibleWebsite(c, d, a.Website); err != nil {
					return nil, err
				}
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			list, active, err := ops.OpTemplates(c.Ctx)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, t := range list {
				e := map[string]any{"id": t.ID, "name": t.Name, "slug": t.Slug, "builtin": t.IsBuiltin}
				if a.Website > 0 {
					e["active"] = active[a.Website] == t.ID
				} else {
					// Only the websites this key may see: a website key learns
					// nothing about the others by way of a template.
					on := []int64{}
					for wsID, tID := range active {
						if tID == t.ID && c.Scope.MaySee(wsID) == nil {
							on = append(on, wsID)
						}
					}
					e["active_on_websites"] = on
				}
				out = append(out, e)
			}
			res := map[string]any{"templates": out}
			if a.Website > 0 && active[a.Website] == 0 {
				res["note"] = "No template is activated for this website; the built-in default template is used."
			}
			return res, nil
		},
	}
}

func activateTemplate(d Deps) Tool {
	return Tool{
		Name:   "activate_template",
		Writes: true,
		Description: "Switches a website to another installed template. The public website looks " +
			"different at once; only when asked.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteArg, "template": templateArg},
			Required:   []string{"website", "template"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64 `json:"website"`
				Template int64 `json:"template"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpActivateTemplate(c.Ctx, ws.ID, a.Template); err != nil {
				return nil, err
			}
			changed(d, c, ws.ID, activity.Entry{
				Action: activity.ActionTemplateActivate, EntityType: "template", EntityID: a.Template,
			})
			return map[string]any{"website": ws.ID, "active_template": a.Template}, nil
		},
	}
}

func uploadTemplate(d Deps) Tool {
	return Tool{
		Name:   "upload_template",
		Writes: true,
		Admin:  true,
		Description: "Installs a template from a .zip archive, after the same checks as the admin's upload: " +
			"layout.html and page.html present, no external subresources, no JavaScript, and a render of " +
			"a full and of an empty page. A refused archive answers with what the checks found. " +
			"get_template_spec is the contract a template must meet. Installing does not activate it.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"name":    {Type: "string", Description: "name of the template; its slug is made from it"},
				"archive": {Type: "string", Description: "the .zip file, base64 encoded"},
			},
			Required: []string{"name", "archive"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Name    string `json:"name"`
				Archive string `json:"archive"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			archive, err := base64.StdEncoding.DecodeString(strings.TrimSpace(a.Archive))
			if err != nil {
				return nil, errors.New("the archive is not valid base64")
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			t, err := ops.OpUploadTemplate(c.Ctx, a.Name, archive)
			if err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionTemplateUpload, EntityType: "template", EntityID: t.ID,
				Metadata: map[string]any{"name": t.Name, "slug": t.Slug},
			})
			return map[string]any{
				"id": t.ID, "name": t.Name, "slug": t.Slug,
				"note": "Installed. activate_template switches a website to it.",
			}, nil
		},
	}
}

func deleteTemplate(d Deps) Tool {
	return Tool{
		Name:   "delete_template",
		Writes: true,
		Admin:  true,
		Description: "Deletes an uploaded template that no website uses. Built-in templates cannot be " +
			"deleted. Requires confirm: true.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"template": templateArg,
				"confirm":  {Type: "boolean", Description: "must be true"},
			},
			Required: []string{"template", "confirm"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Template int64 `json:"template"`
				Confirm  bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errors.New("deleting a template cannot be undone; call again with confirm: true")
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteTemplate(c.Ctx, a.Template); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionTemplateDelete, EntityType: "template", EntityID: a.Template,
			})
			return map[string]any{"deleted": a.Template}, nil
		},
	}
}

func getTemplateSpec() Tool {
	return Tool{
		Name:  "get_template_spec",
		Admin: true,
		Description: "Returns the template authoring specification (Markdown): the files a template " +
			"consists of, the data every view receives, the functions, the rules. Read it before " +
			"writing a template for upload_template.",
		InputSchema: Schema{Type: "object"},
		Run: func(c Call) (any, error) {
			return map[string]any{"specification": tmplspec.Markdown()}, nil
		},
	}
}

// --- design -----------------------------------------------------------------

func designView(ws *domain.Website) map[string]any {
	tokens := map[string]any{
		"ink": ws.TokenInk, "paper": ws.TokenPaper, "brand": ws.TokenBrand,
		"font": ws.TokenFont, "measure": ws.TokenMeasure, "radius": ws.TokenRadius,
	}
	fonts := make([]map[string]any, 0, len(design.FontStacks))
	for _, f := range design.FontStacks {
		fonts = append(fonts, map[string]any{"value": f.Value, "label": f.Label})
	}
	out := map[string]any{
		"website":          ws.ID,
		"tokens":           tokens,
		"template_colours": ws.TokenInk == "" && ws.TokenPaper == "" && ws.TokenBrand == "",
		"fonts":            fonts,
		"rules": fmt.Sprintf("colours are #rgb, #rrggbb or #rrggbbaa; measure is %d to %d characters "+
			"(0: the template decides); radius is 0 to %d pixels (-1: the template decides)",
			design.MinMeasure, design.MaxMeasure, design.MaxRadius),
	}
	// The same line the design screen draws: 4.5:1 is the WCAG minimum for
	// running text.
	if ws.TokenInk != "" && ws.TokenPaper != "" {
		contrast := map[string]any{}
		if r := design.Contrast(ws.TokenInk, ws.TokenPaper); r > 0 {
			contrast["text"] = map[string]any{"ratio": round1(r), "ok": r >= 4.5}
		}
		if r := design.Contrast(ws.TokenBrand, ws.TokenPaper); r > 0 {
			contrast["links"] = map[string]any{"ratio": round1(r), "ok": r >= 4.5}
		}
		out["contrast"] = contrast
	}
	return out
}

func round1(f float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(f, 'f', 1, 64), 64)
	return v
}

func getDesign(d Deps) Tool {
	return Tool{
		Name: "get_design",
		Description: "Fetches a website's own design choices laid over its template: text, background and " +
			"link colour, typeface, text width and corner rounding, with the contrast of the colours and " +
			"the typefaces on offer. Empty or -1 values mean the template decides.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteArg},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			return designView(ws), nil
		},
	}
}

func setDesign(d Deps) Tool {
	return Tool{
		Name:   "set_design",
		Writes: true,
		Admin:  true,
		Description: "Changes a website's design choices. Only what is given changes. A value the design " +
			"rules cannot use is dropped (the template decides there) and listed under \"not_accepted\". " +
			"template_colours: true drops the three colours. The answer carries the contrast: keep text " +
			"and links at 4.5:1 or more.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website":          websiteArg,
				"ink":              {Type: "string", Description: "text colour, e.g. #1a1a1a"},
				"paper":            {Type: "string", Description: "background colour, e.g. #fafafa"},
				"brand":            {Type: "string", Description: "link and accent colour, e.g. #1a6dd4"},
				"font":             {Type: "string", Description: "typeface: " + fontValues() + "; empty for the template's"},
				"measure":          {Type: "integer", Description: fmt.Sprintf("text width in characters, %d to %d; 0 for the template's", design.MinMeasure, design.MaxMeasure)},
				"radius":           {Type: "integer", Description: fmt.Sprintf("corner rounding in pixels, 0 to %d; -1 for the template's", design.MaxRadius)},
				"template_colours": {Type: "boolean", Description: "true drops the own colours, so the template's apply"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website         int64   `json:"website"`
				Ink             *string `json:"ink"`
				Paper           *string `json:"paper"`
				Brand           *string `json:"brand"`
				Font            *string `json:"font"`
				Measure         *int    `json:"measure"`
				Radius          *int    `json:"radius"`
				TemplateColours bool    `json:"template_colours"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}

			// The stored choices as the design form would post them, with what
			// was given laid over — read by the form's own reader.
			form := map[string]string{
				"token_ink": ws.TokenInk, "token_paper": ws.TokenPaper, "token_brand": ws.TokenBrand,
				"token_font":    ws.TokenFont,
				"token_measure": strconv.Itoa(ws.TokenMeasure),
				"token_radius":  strconv.Itoa(ws.TokenRadius),
			}
			given := map[string]string{}
			set := func(field, wire string, v *string) {
				if v != nil {
					form[field] = *v
					given[wire] = *v
				}
			}
			set("token_ink", "ink", a.Ink)
			set("token_paper", "paper", a.Paper)
			set("token_brand", "brand", a.Brand)
			set("token_font", "font", a.Font)
			if a.Measure != nil {
				form["token_measure"] = strconv.Itoa(*a.Measure)
				given["measure"] = form["token_measure"]
			}
			if a.Radius != nil {
				form["token_radius"] = strconv.Itoa(*a.Radius)
				given["radius"] = form["token_radius"]
			}
			if !a.TemplateColours && (form["token_ink"] != "" || form["token_paper"] != "" || form["token_brand"] != "") {
				form["use_colours"] = "on"
			}

			kept, err := ops.OpSetDesign(c.Ctx, ws.ID, form)
			if err != nil {
				return nil, err
			}
			changed(d, c, ws.ID, activity.Entry{
				Action: activity.ActionDesignSave, EntityType: "website", EntityID: ws.ID,
			})

			after, err := d.Domains.GetWebsite(c.Ctx, ws.ID)
			if err != nil || after == nil {
				return nil, errors.New("the design was saved but cannot be read back")
			}
			out := designView(after)
			if no := notAccepted(given, kept, a.TemplateColours); len(no) > 0 {
				out["not_accepted"] = no
			}
			return out, nil
		},
	}
}

func fontValues() string {
	out := []string{}
	for _, f := range design.FontStacks {
		if f.Value != "" {
			out = append(out, f.Value)
		}
	}
	return strings.Join(out, ", ")
}

// notAccepted names the given values Sanitize dropped.
func notAccepted(given map[string]string, kept design.Tokens, templateColours bool) map[string]string {
	out := map[string]string{}
	colour := func(wire, stored string) {
		g, ok := given[wire]
		if !ok || templateColours || strings.TrimSpace(g) == "" {
			return
		}
		if !strings.EqualFold(strings.TrimSpace(g), stored) {
			out[wire] = g
		}
	}
	colour("ink", kept.Ink)
	colour("paper", kept.Paper)
	colour("brand", kept.Brand)
	if g, ok := given["font"]; ok && g != kept.Font {
		out["font"] = g
	}
	if g, ok := given["measure"]; ok && g != "0" && g != strconv.Itoa(kept.Measure) {
		out["measure"] = g
	}
	if g, ok := given["radius"]; ok && g != "-1" && g != strconv.Itoa(kept.Radius) {
		out["radius"] = g
	}
	return out
}

func resetDesign(d Deps) Tool {
	return Tool{
		Name:   "reset_design",
		Writes: true,
		Admin:  true,
		Description: "Drops all of a website's own design choices — colours, typeface, width, rounding — " +
			"so the template decides everything again.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteArg},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpResetDesign(c.Ctx, ws.ID); err != nil {
				return nil, err
			}
			changed(d, c, ws.ID, activity.Entry{
				Action: activity.ActionDesignReset, EntityType: "website", EntityID: ws.ID,
			})
			after, err := d.Domains.GetWebsite(c.Ctx, ws.ID)
			if err != nil || after == nil {
				return nil, errors.New("the design was reset but cannot be read back")
			}
			return designView(after), nil
		},
	}
}

// --- wording ----------------------------------------------------------------

func getWording(d Deps) Tool {
	return Tool{
		Name: "get_wording",
		Description: "Lists, per language of a website, every word its template uses (\"Cart\", \"Search\", " +
			"\"Page not found\"…): the key, what the template says, and the operator's own word where " +
			"one was chosen. Marks the words the template does not translate into that language.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteArg},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			out, err := ops.OpWording(c.Ctx, ws.ID)
			if err != nil {
				return nil, err
			}
			out["website"] = ws.ID
			return out, nil
		},
	}
}

func setWording(d Deps) Tool {
	return Tool{
		Name:   "set_wording",
		Writes: true,
		Description: "Sets the operator's own words for one language of a website, e.g. {\"Cart\": \"Korb\"}. " +
			"Keys are the template's keys as get_wording lists them; an empty value gives the word back " +
			"to the template. Keys the template does not use, and languages the website is not published " +
			"in, are skipped and listed.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website":  websiteArg,
				"language": {Type: "string", Description: "language tag, one of the website's languages"},
				"words":    {Type: "object", Description: "{\"key\": \"own word\"}; \"\" removes the own word"},
			},
			Required: []string{"website", "language", "words"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website  int64             `json:"website"`
				Language string            `json:"language"`
				Words    map[string]string `json:"words"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ws, err := visibleWebsite(c, d, a.Website)
			if err != nil {
				return nil, err
			}
			if len(a.Words) == 0 {
				return nil, errors.New("give at least one word")
			}
			lang := locale.Normalise(a.Language)
			published := false
			for _, l := range ws.AllLocales() {
				if l == lang {
					published = true
				}
			}
			if !published {
				return nil, fmt.Errorf("the website is not published in %q; its languages are %s",
					a.Language, strings.Join(ws.AllLocales(), ", "))
			}
			ops, err := siteOpsOf(d)
			if err != nil {
				return nil, err
			}
			saved, skipped, full, err := ops.OpSetWording(c.Ctx, ws.ID, lang, a.Words)
			if err != nil {
				return nil, err
			}
			if saved > 0 {
				changed(d, c, ws.ID, activity.Entry{
					Action: activity.ActionWordingSave, EntityType: "website", EntityID: ws.ID,
					Metadata: map[string]any{"language": lang, "words": saved},
				})
			}
			out := map[string]any{"website": ws.ID, "language": lang, "saved": saved}
			if len(skipped) > 0 {
				out["skipped"] = skipped
				out["note"] = "Skipped keys are not used by the active template; get_wording lists the ones that are."
			}
			if full {
				out["limit_reached"] = true
				out["note"] = "Not everything was saved: the website has reached its limit of own words."
			}
			return out, nil
		},
	}
}
