package admin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/design"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/wording"
)

// Websites, domains, templates, design and wording — the steps the screens in
// website.go, template.go and wording.go take, as functions of a context and
// plain values, so the screens and the MCP tools in internal/ai take the same
// steps. The Op… methods at the bottom are the tools' side; they return plain
// values and maps because internal/ai cannot import this package (this one
// imports it for the key store).

// errWebsiteNameMissing refuses a website without a name, as the form does.
var errWebsiteNameMissing = errors.New("the website needs a name")

// errNoSuchTemplate is a template id that names nothing.
var errNoSuchTemplate = errors.New("there is no such template")

// templateRefusal is a reason an archive is not installed or a template not
// deleted. key is a catalogue sentence the screen shows as it is; detail, when
// set, is what the checks found and the screen puts it after "Invalid
// template:".
type templateRefusal struct {
	key    string
	detail error
}

func (e templateRefusal) Error() string {
	if e.detail != nil {
		return "invalid template: " + e.detail.Error()
	}
	return e.key
}

// createWebsite creates a website the way the "new website" form does: the
// default template switched on and, unless asked not to, the starter pages and
// menus in place.
func (h *Handler) createWebsite(ctx context.Context, name, description string, starter bool, userID *int64) (*domain.Website, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errWebsiteNameMissing
	}
	ws, err := h.domains.CreateWebsite(ctx, name, strings.TrimSpace(description))
	if err != nil {
		return nil, err
	}

	// Auto-activate default template for new website
	defaultTmpl, err := h.tmplStore.GetBySlug(ctx, "default")
	if err == nil && defaultTmpl != nil {
		_ = h.tmplStore.ActivateForWebsite(ctx, ws.ID, defaultTmpl.ID)
	}

	if starter {
		h.createStarterContent(ctx, ws.ID, userID)
	}
	return ws, nil
}

// saveWebsite stores the website form, read through get.
func (h *Handler) saveWebsite(ctx context.Context, id int64, get func(string) string) error {
	name := strings.TrimSpace(get("name"))
	if name == "" {
		return errWebsiteNameMissing
	}
	description := strings.TrimSpace(get("description"))
	active := get("active") == "on" || get("active") == "1"

	if err := h.domains.UpdateWebsite(ctx, id, name, description, active); err != nil {
		return err
	}
	if err := h.domains.UpdateSettings(ctx, id, settingsFrom(get)); err != nil {
		return err
	}
	// The resolver caches the whole Website struct per host, including its
	// active flag and name. Without this, deactivating or renaming a site has no
	// effect on the public site until the process restarts.
	h.resolver.InvalidateCache()
	// The date helpers are baked into the parsed template set, so a language or
	// time-zone change has to drop it too.
	h.loader.InvalidateTemplateCache(id)
	return nil
}

// websiteFormValues is a stored website as the settings form would post it.
// Laying an assistant's changes over it and reading the result through
// settingsFrom is what makes a partial change go through every rule of the
// full form — and leaves every field nobody mentioned exactly as it was.
func websiteFormValues(ws *domain.Website) map[string]string {
	box := func(b bool) string {
		if b {
			return "on"
		}
		return ""
	}
	id := func(p *int64) string {
		if p == nil {
			return ""
		}
		return strconv.FormatInt(*p, 10)
	}
	return map[string]string{
		"name":               ws.Name,
		"description":        ws.Description,
		"active":             box(ws.Active),
		"locale":             ws.Locale,
		"extra_locales":      ws.ExtraLocales,
		"timezone":           ws.TimeZone,
		"meta_description":   ws.MetaDescription,
		"canonical_redirect": box(ws.CanonicalRedirect),
		"confirm_senders":    box(ws.ConfirmSenders),
		"offline_mode":       ws.OfflineMode,
		"offline_message":    ws.OfflineMessage,
		"favicon_media_id":   id(ws.FaviconMediaID),
		"logo_media_id":      id(ws.LogoMediaID),
		"contact_email":      ws.ContactEmail,
		"notify_email":       ws.NotifyEmail,
		"org_type":           ws.OrgType,
		"street":             ws.Street,
		"postal_code":        ws.PostalCode,
		"city":               ws.City,
		"country":            ws.Country,
		"phone":              ws.Phone,
		"opening_hours":      ws.OpeningHours,
		"blog_base":          ws.BlogBase,
		"posts_per_page":     strconv.Itoa(ws.PostsPerPage),
	}
}

// deleteWebsite removes a website and the files of its media.
func (h *Handler) deleteWebsite(ctx context.Context, id int64) error {
	if err := h.domains.DeleteWebsite(ctx, id); err != nil {
		return err
	}
	h.resolver.InvalidateCache()

	// The media rows go with the website through ON DELETE CASCADE, but the
	// files behind them do not. Without this the uploads of every deleted site
	// stay on the SD card forever, unreachable and uncountable.
	mediaDir := filepath.Join(h.cfg.DataDir, "media", strconv.FormatInt(id, 10))
	if err := os.RemoveAll(mediaDir); err != nil {
		// The website is already gone; failing the request now would suggest it
		// was not deleted. Log it so the leftover can be cleaned up by hand.
		slog.Error("remove media directory of deleted website", "err", err, "dir", mediaDir)
	}
	return nil
}

// activateTemplate switches a website to a template.
func (h *Handler) activateTemplate(ctx context.Context, websiteID, templateID int64) error {
	if err := h.tmplStore.ActivateForWebsite(ctx, websiteID, templateID); err != nil {
		return err
	}
	h.loader.InvalidateTemplateCache(websiteID)
	return nil
}

// saveDesign stores the design tokens of the design form, read through get,
// and returns what was kept.
//
// Sanitize drops anything the CSS could not use, so a mistyped value falls back
// to the theme instead of losing the settings that were right.
func (h *Handler) saveDesign(ctx context.Context, websiteID int64, get func(string) string) (design.Tokens, error) {
	tokens := tokensFromForm(get)
	if err := h.domains.UpdateDesignTokens(ctx, websiteID, domain.DesignTokens{
		Ink: tokens.Ink, Paper: tokens.Paper, Brand: tokens.Brand,
		Font: tokens.Font, Measure: tokens.Measure, Radius: tokens.Radius,
	}); err != nil {
		return tokens, err
	}
	h.invalidateWebsiteCaches(websiteID)
	return tokens, nil
}

// resetDesign drops every design token, so the template decides again.
func (h *Handler) resetDesign(ctx context.Context, websiteID int64) error {
	if err := h.domains.UpdateDesignTokens(ctx, websiteID, domain.DesignTokens{Radius: -1}); err != nil {
		return err
	}
	h.invalidateWebsiteCaches(websiteID)
	return nil
}

// installTemplate unpacks an uploaded archive, runs every check on it —
// structure, external subresources, scripts, the render check — and records
// it. A refusal is a templateRefusal; anything else is a fault.
func (h *Handler) installTemplate(ctx context.Context, name string, archive []byte) (*tmplmgr.Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, templateRefusal{key: i18n.N("Please give the template a name")}
	}
	slug := slugify(name)
	// A name with no usable characters would yield an empty slug, making destDir
	// the templates root — ExtractTemplate would then RemoveAll every installed
	// template before renaming its temp dir into place.
	if slug == "" {
		return nil, templateRefusal{key: i18n.N("The template name must contain letters or digits")}
	}

	existing, err := h.tmplStore.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, templateRefusal{key: i18n.N("A template with that name already exists")}
	}

	destDir := filepath.Join(h.cfg.DataDir, "templates", slug)
	if err := tmplmgr.ExtractTemplate(bytes.NewReader(archive), int64(len(archive)), destDir,
		h.cfg.MaxTemplateSize, h.loader.DefaultFS()); err != nil {
		return nil, templateRefusal{detail: err}
	}

	t, err := h.tmplStore.Create(ctx, name, slug)
	if err != nil {
		// Clean up disk on DB failure
		_ = os.RemoveAll(destDir)
		return nil, err
	}
	return t, nil
}

// deleteTemplate removes an uploaded template that no website uses.
func (h *Handler) deleteTemplate(ctx context.Context, id int64) error {
	t, err := h.tmplStore.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if t == nil {
		return errNoSuchTemplate
	}
	if t.IsBuiltin {
		return templateRefusal{key: i18n.N("Built-in templates cannot be deleted.")}
	}
	// Check if active anywhere (T-04-08)
	active, err := h.tmplStore.IsActiveAnywhere(ctx, id)
	if err != nil {
		return err
	}
	if active {
		return templateRefusal{key: i18n.N("A template that is active on a website cannot be deleted. Please activate a different template there first.")}
	}
	return h.tmplStore.Delete(ctx, id)
}

// --- for the MCP tools ------------------------------------------------------

// OpCreateWebsite creates a website exactly as the "new website" form does,
// starter pages and menus included unless starter is false.
func (h *Handler) OpCreateWebsite(ctx context.Context, name, description string, starter bool) (*domain.Website, error) {
	return h.createWebsite(ctx, name, description, starter, nil)
}

// OpUpdateWebsite changes the settings named in changes — keyed by the names
// of the settings form's fields — and leaves every other one as it is. The
// values go through the form's own validation.
func (h *Handler) OpUpdateWebsite(ctx context.Context, id int64, changes map[string]string) (*domain.Website, error) {
	ws, err := h.domains.GetWebsite(ctx, id)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, errors.New("there is no such website")
	}
	form := websiteFormValues(ws)
	for k, v := range changes {
		if _, ok := form[k]; !ok {
			return nil, fmt.Errorf("the website has no setting %q", k)
		}
		form[k] = v
	}
	// The screen only offers this website's images; an id from outside has to
	// be checked instead, or a favicon could point into another website.
	for _, field := range []string{"favicon_media_id", "logo_media_id"} {
		mid := optionalID(form[field])
		if mid == nil || h.mediaStore == nil {
			continue
		}
		m, err := h.mediaStore.GetByID(ctx, *mid)
		if err != nil || m == nil || m.WebsiteID != id || !strings.HasPrefix(m.MimeType, "image/") {
			return nil, fmt.Errorf("%s: %d is not an image of this website", field, *mid)
		}
	}
	if err := h.saveWebsite(ctx, id, func(k string) string { return form[k] }); err != nil {
		return nil, err
	}
	return h.domains.GetWebsite(ctx, id)
}

// OpDeleteWebsite deletes a website with everything in it, media files
// included.
func (h *Handler) OpDeleteWebsite(ctx context.Context, id int64) error {
	return h.deleteWebsite(ctx, id)
}

// OpLaunchChecklist is the "ready to launch?" list of the settings screen.
func (h *Handler) OpLaunchChecklist(ctx context.Context, websiteID int64) ([]map[string]any, error) {
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, errors.New("there is no such website")
	}
	domains, err := h.domains.ListDomains(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	checks := h.siteChecks(ctx, ws, domains)
	out := make([]map[string]any, 0, len(checks))
	for _, c := range checks {
		out = append(out, map[string]any{"check": c.Label, "ok": c.OK, "hint": c.Hint})
	}
	return out, nil
}

// OpTemplates lists the installed templates and, per website, the id of the
// active one.
func (h *Handler) OpTemplates(ctx context.Context) ([]tmplmgr.Template, map[int64]int64, error) {
	list, err := h.tmplStore.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	active, err := h.tmplStore.ActiveByWebsite(ctx)
	if err != nil {
		return nil, nil, err
	}
	return list, active, nil
}

// OpActivateTemplate switches a website to an installed template.
func (h *Handler) OpActivateTemplate(ctx context.Context, websiteID, templateID int64) error {
	t, err := h.tmplStore.GetByID(ctx, templateID)
	if err != nil {
		return err
	}
	if t == nil {
		return errNoSuchTemplate
	}
	return h.activateTemplate(ctx, websiteID, templateID)
}

// OpSetDesign stores design tokens given as the design form's fields
// (token_ink, token_paper, token_brand, token_font, token_measure,
// token_radius, use_colours) and returns what survived Sanitize.
func (h *Handler) OpSetDesign(ctx context.Context, websiteID int64, form map[string]string) (design.Tokens, error) {
	return h.saveDesign(ctx, websiteID, func(k string) string { return form[k] })
}

// OpResetDesign drops the website's design tokens.
func (h *Handler) OpResetDesign(ctx context.Context, websiteID int64) error {
	return h.resetDesign(ctx, websiteID)
}

// OpWording is the wording screen: per language every word the theme mints,
// what the theme says and what the operator chose; and the stray words.
func (h *Handler) OpWording(ctx context.Context, websiteID int64) (map[string]any, error) {
	if h.wording == nil {
		return nil, errors.New("the wording store is not available")
	}
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, errors.New("there is no such website")
	}
	languages, stray, used, err := h.wordingOf(ctx, ws)
	if err != nil {
		return nil, err
	}
	langs := make([]map[string]any, 0, len(languages))
	for _, l := range languages {
		words := make([]map[string]any, 0, len(l.Rows))
		for _, row := range l.Rows {
			w := map[string]any{"key": row.Key, "theme": row.Theme, "untranslated": row.Untranslated}
			if row.Own != "" {
				w["own"] = row.Own
			}
			words = append(words, w)
		}
		langs = append(langs, map[string]any{
			"language": l.Locale, "name": l.Name, "words": words,
			"changed": l.Changed, "untranslated": l.Untranslated,
		})
	}
	strays := make([]map[string]any, 0, len(stray))
	for _, e := range stray {
		strays = append(strays, map[string]any{"language": e.Locale, "key": e.Key, "own": e.Value})
	}
	return map[string]any{
		"languages": langs, "unused_own_words": strays,
		"used": used, "limit": wording.MaxKeys,
	}, nil
}

// OpSetWording writes the operator's words for one language; an empty value
// gives a word back to the theme. It returns how many were written, the keys
// that were skipped, and whether the website's limit was reached.
func (h *Handler) OpSetWording(ctx context.Context, websiteID int64, locale string, words map[string]string) (int, []string, bool, error) {
	if h.wording == nil {
		return 0, nil, false, errors.New("the wording store is not available")
	}
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return 0, nil, false, err
	}
	if ws == nil {
		return 0, nil, false, errors.New("there is no such website")
	}
	in := make([]wordInput, 0, len(words))
	for k, v := range words {
		in = append(in, wordInput{Locale: locale, Key: k, Value: v})
	}
	res, err := h.saveWords(ctx, ws, in)
	if err != nil {
		return 0, nil, false, err
	}
	skipped := make([]string, 0, len(res.Skipped))
	for _, s := range res.Skipped {
		skipped = append(skipped, s.Key)
	}
	return res.Saved, skipped, res.Full, nil
}

// OpUploadTemplate installs a template archive after the same checks as the
// upload screen.
func (h *Handler) OpUploadTemplate(ctx context.Context, name string, archive []byte) (*tmplmgr.Template, error) {
	if int64(len(archive)) > h.cfg.MaxTemplateSize {
		return nil, fmt.Errorf("the archive is larger than the %d bytes this installation accepts", h.cfg.MaxTemplateSize)
	}
	return h.installTemplate(ctx, name, archive)
}

// OpDeleteTemplate deletes an uploaded template no website uses.
func (h *Handler) OpDeleteTemplate(ctx context.Context, id int64) error {
	return h.deleteTemplate(ctx, id)
}
