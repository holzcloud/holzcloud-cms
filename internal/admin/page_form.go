package admin

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// PageValues holds exactly what was submitted, so a rejected form can be
// re-rendered with the user's own text rather than the database row.
//
// Templates read .Values, never .Page — reading the row is what made a failed
// save discard a long article and show an empty form with a flash message.
type PageValues struct {
	Title    string
	Slug     string
	Markdown string
	Status   string
	Version  int64
	// TypeKey is the "Art" the form sent: "page", "post" or one of the
	// website's own kinds. It is resolved against that website's kinds by
	// setKind, after which Kind is the built-in behaviour and TypeKey is the
	// own kind or empty.
	TypeKey string
	// Kind is "page" or "post". A post is listed in the archive by date; a page
	// is not.
	Kind string
	// Tags is the comma-separated label field, kept as typed so a rejected form
	// gives it back unchanged.
	Tags string
	// Protected, PagePassword and AccessHint drive the password gate. The
	// password field is never filled from the database — echoing a stored one
	// back would put it in the page source of every edit screen.
	Protected    bool
	PagePassword string
	AccessHint   string
	// HasPassword says a password is already set, so the form can offer "leave
	// blank to keep it" instead of demanding it again.
	HasPassword bool

	Excerpt         string
	MetaDescription string
	FeaturedMediaID string
	NoIndex         bool

	// PublishAt and UnpublishAt are the raw <input type="datetime-local">
	// values, kept as strings so a rejected form shows exactly what was typed.
	PublishAt   string
	UnpublishAt string

	// Blocks is the page as building blocks. Nil means this page is written as
	// one piece of Markdown, which is what every page was until an editor
	// switched this one over.
	Blocks []block.Block

	// Fields are the answers to the website's own fields, as typed. Kept raw so
	// a rejected form gives back exactly what was entered — a price typed with
	// a comma should come back with the comma and a reason beside it.
	Fields field.Data

	// Locale is the language this page is written in, empty for the website's
	// main one.
	Locale string
	// TranslationOf is the page in the main language this one belongs to, as a
	// string because it comes out of a hidden form field. Zero or empty means
	// the page stands on its own.
	TranslationOf string

	// BlockSet is the block kinds this website may use. It travels with the
	// form values because everything that reads the block list — the menu, the
	// switch back to Markdown, the save — has to know what a stored type means,
	// and a website's own kinds are not something a free function can look up.
	BlockSet block.Set
}

// UsesBlocks reports which editor this page opens in.
func (v PageValues) UsesBlocks() bool { return v.Blocks != nil }

// BlockKinds is the menu of block types, for the template: the built-in ones
// and whatever this website has defined for itself.
func (v PageValues) BlockKinds() []block.Kind { return v.BlockSet.Menu() }

// CanReturnToMarkdown reports whether switching back would lose nothing.
func (v PageValues) CanReturnToMarkdown() bool {
	_, ok := block.ToMarkdown(v.Blocks)
	return ok
}

// pageValuesFromRequest reads the submitted fields verbatim.
//
// The set is a parameter and not something the caller may attach afterwards,
// the way pageValuesFromPage takes it. It was optional once, and one of the two
// callers forgot it: saving an existing page then ran Clean against the nine
// built-in kinds alone, and every block of a kind the website had defined for
// itself was dropped without a word. Six feature cards came back as prose, and
// the editor no longer offered the kind that would have restored them.
func pageValuesFromRequest(r *http.Request, set block.Set) PageValues {
	version, _ := strconv.ParseInt(r.FormValue("version"), 10, 64)
	status := r.FormValue("status")
	if status != "published" {
		status = "draft"
	}
	return PageValues{
		BlockSet:        set,
		Kind:            page.NormalizeKind(r.FormValue("kind")),
		TypeKey:         strings.TrimSpace(r.FormValue("kind")),
		Tags:            strings.TrimSpace(r.FormValue("tags")),
		Protected:       r.FormValue("protected") != "",
		PagePassword:    r.FormValue("page_password"),
		AccessHint:      strings.TrimSpace(r.FormValue("access_hint")),
		HasPassword:     r.FormValue("has_password") != "",
		Title:           strings.TrimSpace(r.FormValue("title")),
		Slug:            strings.TrimSpace(r.FormValue("slug")),
		Markdown:        r.FormValue("content_markdown"),
		Status:          status,
		Version:         version,
		Excerpt:         strings.TrimSpace(r.FormValue("excerpt")),
		MetaDescription: strings.TrimSpace(r.FormValue("meta_description")),
		FeaturedMediaID: strings.TrimSpace(r.FormValue("featured_media_id")),
		NoIndex:         r.FormValue("noindex") != "",
		PublishAt:       strings.TrimSpace(r.FormValue("publish_at")),
		UnpublishAt:     strings.TrimSpace(r.FormValue("unpublish_at")),
		Blocks:          blocksFromRequest(r),
		Fields:          fieldsFromRequest(r),
		Locale:          strings.TrimSpace(r.FormValue("sprache")),
		TranslationOf:   strings.TrimSpace(r.FormValue("uebersetzung_von")),
	}
}

// fieldsFromRequest reads every "feld_*" and "gruppe.*" value.
//
// Read by prefix rather than by definition list: this runs before the
// definitions are loaded, and a value whose field was deleted while the form
// was open is dropped later by field.Clean rather than silently here.
//
// A group's rows are named gruppe.<kennung>.<nummer>.<unterfeld>. The number is
// the position in the form, not an identity — after a row is removed the
// numbers are handed out again from the top, which is exactly what makes
// removing a row a plain form submission.
func fieldsFromRequest(r *http.Request) field.Data {
	out := field.Data{Values: field.Values{}, Rows: map[string][]field.Values{}}
	rows := map[string]map[int]field.Values{}

	for name, values := range r.Form {
		if len(values) == 0 {
			continue
		}
		if key, ok := strings.CutPrefix(name, "feld_"); ok && key != "" {
			out.Values[key] = values[0]
			continue
		}
		group, index, sub, ok := parseRowName(name)
		if !ok {
			continue
		}
		if rows[group] == nil {
			rows[group] = map[int]field.Values{}
		}
		if rows[group][index] == nil {
			rows[group][index] = field.Values{}
		}
		rows[group][index][sub] = values[0]
	}

	for group, byIndex := range rows {
		indexes := make([]int, 0, len(byIndex))
		for i := range byIndex {
			indexes = append(indexes, i)
		}
		sort.Ints(indexes)
		for _, i := range indexes {
			out.Rows[group] = append(out.Rows[group], byIndex[i])
		}
	}
	return out
}

// parseRowName splits gruppe.<kennung>.<nummer>.<unterfeld>.
func parseRowName(name string) (group string, index int, sub string, ok bool) {
	rest, found := strings.CutPrefix(name, "gruppe.")
	if !found {
		return "", 0, "", false
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return "", 0, "", false
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil || i < 0 || i >= field.MaxRows {
		return "", 0, "", false
	}
	return parts[0], i, parts[2], true
}

// blocksFromRequest reads the block list, or nil when this page is Markdown.
//
// The hidden marker rather than "are there any b0. fields": a page whose last
// block was just deleted has no block fields at all, and without the marker it
// would silently fall back into the Markdown editor and lose the switch.
func blocksFromRequest(r *http.Request) []block.Block {
	if r.FormValue("bausteine") == "" {
		return nil
	}
	blocks := block.FromForm(r.Form)
	if blocks == nil {
		// Not nil: an empty list still means "this page uses blocks".
		blocks = []block.Block{}
	}
	return blocks
}

// pageValuesFromPage fills the form from a stored page, used for the initial GET.
func pageValuesFromPage(p *page.Page, set block.Set) PageValues {
	if p == nil {
		return PageValues{Status: "draft", Kind: page.KindPage, BlockSet: set}
	}
	v := PageValues{
		BlockSet:        set,
		Kind:            page.NormalizeKind(p.Kind),
		TypeKey:         p.TypeKey,
		Title:           p.Title,
		Slug:            p.Slug,
		Markdown:        p.ContentMarkdown,
		Status:          p.Status,
		Version:         p.Version,
		Excerpt:         p.Excerpt,
		MetaDescription: p.MetaDescription,
		NoIndex:         p.NoIndex,
		Protected:       p.Protected(),
		AccessHint:      p.AccessHint,
		HasPassword:     p.AccessPassword != "",
		Locale:          p.Locale,
	}
	if p.TranslationOf != 0 {
		v.TranslationOf = strconv.FormatInt(p.TranslationOf, 10)
	}
	if p.FeaturedMediaID != nil {
		v.FeaturedMediaID = strconv.FormatInt(*p.FeaturedMediaID, 10)
	}
	v.PublishAt = formatLocalInput(p.PublishAt)
	v.UnpublishAt = formatLocalInput(p.UnpublishAt)
	if blocks, err := block.Decode(p.Blocks, set); err == nil && blocks != nil {
		v.Blocks = blocks
	}
	v.Fields = field.Decode(p.Fields)
	return v
}

// localInputLayout is what <input type="datetime-local"> sends and expects.
const localInputLayout = "2006-01-02T15:04"

// formatLocalInput renders a stored UTC instant for the datetime-local field.
//
// The field carries no time zone, so both directions use UTC and the form says
// so next to the input. Presenting the website's zone here would need the
// website in scope and would still be guesswork about the editor's own clock —
// a stated UTC is honest, a silently shifted local time is not.
func formatLocalInput(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(localInputLayout)
}

// parseLocalInput reads a datetime-local value as UTC.
func parseLocalInput(raw string) (*time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	// Some browsers include seconds.
	for _, layout := range []string{localInputLayout, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			utc := t.UTC()
			return &utc, true
		}
	}
	return nil, false
}

// schedule turns the submitted values into the store's schedule, reporting a
// bad date rather than silently dropping it — a page that quietly failed to be
// scheduled would just never appear.
func (v PageValues) schedule(errs web.FormErrors) page.PageSchedule {
	var s page.PageSchedule
	if t, ok := parseLocalInput(v.PublishAt); ok {
		s.PublishAt = t
	} else {
		errs.Add("publish_at", "Bitte ein gültiges Datum angeben.")
	}
	if t, ok := parseLocalInput(v.UnpublishAt); ok {
		s.UnpublishAt = t
	} else {
		errs.Add("unpublish_at", "Bitte ein gültiges Datum angeben.")
	}
	if s.PublishAt != nil && s.UnpublishAt != nil && !s.UnpublishAt.After(*s.PublishAt) {
		errs.Add("unpublish_at", "Das Ende muss nach dem Start liegen.")
	}
	return s
}

// meta turns the submitted values into what the store writes.
//
// An empty excerpt is derived from the Markdown rather than stored empty, so a
// listing or a meta description always has something to show. An editor who
// types one keeps it verbatim.
func (v PageValues) meta() page.PageMeta {
	excerpt := v.Excerpt
	if excerpt == "" {
		excerpt = page.Excerpt(v.Markdown)
	}
	m := page.PageMeta{
		Excerpt:         excerpt,
		MetaDescription: v.MetaDescription,
		NoIndex:         v.NoIndex,
	}
	if id, err := strconv.ParseInt(v.FeaturedMediaID, 10, 64); err == nil && id > 0 {
		m.FeaturedMediaID = &id
	}
	return m
}

// access turns the submitted values into what the store writes.
func (v PageValues) access() page.AccessUpdate {
	return page.AccessUpdate{
		Protected: v.Protected,
		Password:  v.PagePassword,
		Hint:      v.AccessHint,
	}
}

// IsPost reports whether the form is editing an archive entry.
func (v PageValues) IsPost() bool { return v.Kind == page.KindPost }

// KindValue is what the "Art" dropdown should have selected: the own kind when
// there is one, otherwise the built-in.
func (v PageValues) KindValue() string {
	if v.TypeKey != "" {
		return v.TypeKey
	}
	return v.Kind
}

// validate checks the submitted values and normalises the slug.
func (v *PageValues) validate(errs web.FormErrors) string {
	return v.validateFor(errs, "")
}

// validateFor is validate with the website's own reserved address.
//
// The archive lives at a slug the operator chose, and a page taking the same
// one would be permanently shadowed by it — the same failure ValidateSlug
// already prevents for the router's own routes, just per website.
func (v *PageValues) validateFor(errs web.FormErrors, archiveSlug string) string {
	return v.validateOn(nil, errs, archiveSlug, nil)
}

// validateOn is validateFor plus the website's additional languages.
//
// A page whose address is "fr" on a site that has French would never be
// reachable: /fr belongs to the language. Saying so here is the only moment
// where it can still be corrected — afterwards the page exists and is simply
// invisible, with nothing on the screen to explain why.
//
// The request is only here for the language: two of the messages have
// something in them, and a sentence glued together in Go can never be looked up
// in the catalogue. It may be nil, and then the messages stay German.
func (v *PageValues) validateOn(r *http.Request, errs web.FormErrors, archiveSlug string, extras []string) string {
	if v.Title == "" {
		errs.Add("title", "Bitte einen Titel angeben.")
	}

	slug := v.Slug
	if slug == "" {
		slug = page.Slugify(v.Title)
	} else {
		slug = page.Slugify(slug)
	}
	if v.Title != "" {
		if err := page.ValidateSlug(slug); err != nil {
			errs.Add("slug", tr(r, "Ungültige Adresse: %s", err))
		} else if archiveSlug != "" && slug == archiveSlug {
			errs.Add("slug", "Diese Adresse gehört dem Archiv und wäre nicht erreichbar.")
		} else if locale.Reserved(slug, extras) {
			errs.Add("slug", tr(r, "Diese Adresse gehört der Sprache %s und wäre nicht erreichbar.",
				trs(r, locale.Name(slug))))
		}
	}
	return slug
}

// tr and trs are Titlef and T that survive a nil request, for the validator —
// which is also called from places that have none.
func tr(r *http.Request, format string, args ...any) string {
	if r == nil {
		return fmt.Sprintf(format, args...)
	}
	return web.Titlef(r, format, args...)
}

func trs(r *http.Request, s string) string {
	if r == nil {
		return s
	}
	return web.T(r, s)
}

// translationOf is the page this one belongs to, or zero.
func (v PageValues) translationOf() int64 {
	id, _ := strconv.ParseInt(v.TranslationOf, 10, 64)
	if id < 0 {
		return 0
	}
	return id
}
