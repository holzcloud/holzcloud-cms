package template

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
)

// SafeAssetPath validates a request-supplied asset path before it is joined to
// a directory on disk. It returns the cleaned slash-separated path and true
// only when the path stays inside its base directory.
//
// This must be applied to every path that reaches os.ReadFile or filepath.Join:
// http.ServeMux runs its path cleaning on the *escaped* URL, so a percent-encoded
// "%2e%2e" survives routing and is handed to the handler as a literal "..".
func SafeAssetPath(p string) (string, bool) {
	if p == "" || strings.ContainsAny(p, "\\\x00") || path.IsAbs(p) {
		return "", false
	}
	clean := path.Clean(p)
	if !fs.ValidPath(clean) || clean == "." {
		return "", false
	}
	return clean, true
}

// TemplateResolver resolves the active template slug for a website.
// The tmplmgr.Store implements this interface via ActiveTemplateSlug.
type TemplateResolver interface {
	ActiveTemplateSlug(ctx context.Context, websiteID int64) (string, error)
}

// PageData is the data passed to public templates for rendering.
type PageData struct {
	Site  SiteData
	Page  PageContent
	Menus map[string][]menu.MenuNode
	Meta  MetaData
	// Search is filled only for search.html. It is zero-valued everywhere else,
	// so a theme that ignores it is unaffected.
	Search SearchData
	// Archive is filled only for list.html, on the same terms.
	Archive ArchiveData
	// Gate is filled only for gate.html, the password form.
	Gate GateData
	// Preview marks a page reached through a share link, so a theme can say so
	// on the page itself.
	Preview PreviewData
	// Shop is filled on every page of a website that sells something, so a
	// layout can show the basket and the price-mode switch anywhere. Its
	// Enabled field is false on a site without a shop, and a theme that
	// ignores it is unaffected.
	Shop ShopData
	// Catalogue is filled only for shop.html, Product only for product.html.
	Catalogue CatalogueData
	Product   ProductData
	// Cart is filled on every page of a selling website, so a layout can show
	// what is in the basket from anywhere. Its Lines are only filled on
	// cart.html — a header badge needs the count, not the contents.
	Cart CartData
	// Checkout is filled only for checkout.html, Order only for order.html.
	Checkout CheckoutData
	Order    OrderData
}

// CheckoutData is the order form.
type CheckoutData struct {
	// Action is where the form posts — always back to itself.
	Action string
	// Values holds what was typed, by field name, so a rejected form comes
	// back filled in rather than blank.
	Values map[string]string
	// Errors are the messages by field name. Print them beside their field.
	Errors map[string]string
	// Notice is a message about the order as a whole, such as an article that
	// ran out between the basket and the button.
	Notice string
	// Business is true when this visitor is ordering as a company, which is
	// what makes the company and VAT fields required.
	Business bool
	// Methods are the payment options offered.
	Methods []PaymentMethod
	// ReturnPolicy is what the shop promises about sending goods back, or
	// empty. Switzerland grants no statutory right of withdrawal for online
	// orders, so an empty value is lawful — and has to be left unsaid rather
	// than filled with a promise nobody made.
	ReturnPolicy string
	// Accepted is the state of the confirmation box.
	Accepted bool
}

// PaymentMethod is one option on the order form.
type PaymentMethod struct {
	Value string
	Label string
	Note  string
}

// OrderData is a placed order, as its confirmation shows it.
type OrderData struct {
	Number  string
	Email   string
	Name    string
	Company string
	// Address is the delivery address on one line.
	Address string
	Note    string
	// Status is "new", "paid", "shipped" or "cancelled".
	Status string
	// PaymentLabel is the chosen method in words.
	PaymentLabel string
	// PaymentNote says where the payment stands, in a sentence the customer
	// can act on: what to transfer, or that nothing more is owed. Empty when
	// there is nothing to say.
	PaymentNote string
	// PaymentPending is true while the money has not arrived. A theme can use
	// it to mark the order visibly as not yet settled; PaymentNote already
	// says so in words, so ignoring this field is fine.
	PaymentPending bool
	ReturnPolicy   string
	Lines          []CartLine
	Totals         CartTotals
}

// CartData is the basket.
type CartData struct {
	// Count is the number of articles, summed over the lines. Zero means empty.
	Count int
	// URL is where the basket lives.
	URL string
	// Total is the amount as it must be printed, currency and all.
	Total string
	// Lines are filled only for cart.html.
	Lines []CartLine
	// Totals is the summary under the lines, only on cart.html.
	Totals CartTotals
	// CheckoutURL is where the order form lives, empty when the basket is
	// empty or something in it cannot be bought.
	CheckoutURL string
	// Blocked names why checkout is not possible, or is empty.
	Blocked string
	// UpdateURL and RemoveURL are where the quantity forms post to.
	UpdateURL string
	RemoveURL string
}

// CartLine is one row of the basket.
type CartLine struct {
	Title    string
	Subtitle string
	URL      string
	// Slug identifies the line in the quantity and remove forms.
	Slug     string
	ImageURL string
	Quantity int
	// UnitPrice and LinePrice are printed strings, never numbers.
	UnitPrice string
	LinePrice string
	// Available is false when the article ran out while the basket sat there.
	Available bool
}

// CartTotals is the summary under the basket.
type CartTotals struct {
	Items    string
	Shipping string
	// ShippingFree is true when the delivery charge was waived, so a theme can
	// say "kostenlos" rather than "CHF 0.00".
	ShippingFree bool
	Total        string
	// TaxLines are the per-rate rows an invoice must show. Empty in a shop
	// that is not liable for VAT.
	TaxLines []CartTaxLine
	// TaxNote is the sentence under the total.
	TaxNote string
}

// CartTaxLine is one rate's share of the total.
type CartTaxLine struct {
	Label string
	Net   string
	Tax   string
}

// ShopData is what every page of a selling website knows about its shop.
type ShopData struct {
	// Enabled is false when the website sells nothing. Everything else in this
	// struct is then zero.
	Enabled bool
	// URL is the catalogue's address, e.g. "/shop".
	URL string
	// Audience is "private" or "business" — which prices are being shown.
	Audience string
	// CanSwitchAudience is true only where the operator offers both. A consumer
	// shop must not offer to show net prices: the price a consumer is shown is
	// the one the Preisbekanntgabeverordnung regulates.
	CanSwitchAudience bool
	// TaxNote is the sentence under a price: "inkl. 8.1 % MWST", "zzgl. …", or
	// the small-business note. Already worded for the audience.
	TaxNote string
	// ShippingNote is "Versandkostenfrei ab CHF 200" or empty.
	ShippingNote string
	// Categories are the labels products are filed under, for a shop menu.
	Categories []TermLink
}

// CatalogueData is the product list rendered by shop.html.
type CatalogueData struct {
	Products []ProductEntry
	// Page and TotalPages drive the pager; TotalPages is 1 when everything
	// fits, so a theme can hide it with one comparison.
	Page, TotalPages int
	PrevURL, NextURL string
	Total            int
	// Term is the category this listing is filtered by, empty on the full
	// catalogue.
	Term string
}

// ProductEntry is one product in a listing.
type ProductEntry struct {
	Title    string
	Subtitle string
	URL      string
	Excerpt  string
	// ImageURL is a same-origin /media/ path, or empty.
	ImageURL string
	// Price is the amount as it must be printed, currency and all:
	// "CHF 1’234.55". Never a number for the theme to format.
	Price string
	// PriceNote is the tax sentence for this product's rate.
	PriceNote string
	// Available is false when the product is tracked and sold out.
	Available bool
	// SoldOutLabel is what to print instead of a buy button, or empty.
	SoldOutLabel string
	Terms        []TermLink
}

// ProductData is the single product rendered by product.html.
type ProductData struct {
	Title    string
	Subtitle string
	Slug     string
	// DescriptionHTML has been through the same sanitising as page content.
	DescriptionHTML template.HTML
	SKU             string
	Price           string
	PriceNote       string
	// PriceOther is the same price for the other audience — a business shop
	// still has to state the gross amount somewhere. Empty when only one
	// audience is served.
	PriceOther string
	// ImageURL is the main picture; Gallery holds the rest.
	ImageURL  string
	Gallery   []string
	Available bool
	// StockNote is "Noch 2 an Lager" or "Ausverkauft", or empty when the
	// operator does not track quantities.
	StockNote    string
	DeliveryNote string
	Terms        []TermLink
	// AddURL is where the buy form posts to.
	AddURL string
}

// GateData is the password form in front of a protected page.
type GateData struct {
	// Hint is the operator's line above the field — "the password is in our
	// letter" — so someone who has it knows they are in the right place.
	Hint string
	// Path is the page being unlocked, posted back with the password.
	Path string
	// Wrong is true after a failed attempt.
	Wrong bool
}

// PreviewData marks a page served through a share link.
type PreviewData struct {
	Active bool
	// Status is the page's own status, so the banner can say "draft" rather
	// than only "preview".
	Status string
}

// ArchiveData is the entry list rendered by list.html.
type ArchiveData struct {
	// Entries are the posts on this page of the archive, newest first.
	Entries []ArchiveEntry
	// Page and TotalPages drive the pager. TotalPages is 1 when everything fits
	// on one page, so a theme can hide the pager with a single comparison.
	Page, TotalPages int
	// PrevURL and NextURL are empty at the ends of the archive, so a theme does
	// not have to compute whether a link would lead anywhere.
	PrevURL, NextURL string
	// Total is the number of entries in the whole archive.
	Total int
	// Term is the label this archive is filtered by, empty on the post archive.
	Term string
	// Terms are the labels in use on the site, for a tag cloud in the layout.
	Terms []TermLink
}

// TermLink is one label, with how often it is used.
type TermLink struct {
	Name  string
	URL   string
	Count int
}

// ArchiveEntry is one row of the archive.
type ArchiveEntry struct {
	Title       string
	URL         string
	Excerpt     string
	PublishedAt *time.Time
	// ImageURL is the entry's preview image as a same-origin path, or empty.
	ImageURL string
	// Terms are the entry's labels.
	Terms []TermLink
}

// SearchData is the result list rendered by search.html.
type SearchData struct {
	Query string
	// Submitted separates "no query yet" from "no results", which need
	// different words on the page.
	Submitted bool
	Results   []SearchHit
}

// SearchHit is one result.
type SearchHit struct {
	Title string
	URL   string
	// Snippet is the matching passage with <mark> around the terms. It is built
	// from escaped text in internal/page/search.go, never from stored HTML.
	Snippet template.HTML
}

// SiteData holds website-level information for template rendering.
//
// Fields are only ever added, never renamed or removed: an uploaded theme is
// written against this struct and a missing field is a parse error at render
// time, on the visitor's request. New fields are zero-valued for old themes.
type SiteData struct {
	Name        string
	Description string
	// MetaDescription is the site-wide fallback for <meta name="description">.
	MetaDescription string
	// Locale is the BCP-47-ish language tag for <html lang="…">.
	Locale string
	// TimeZone is the IANA name dates are rendered in.
	TimeZone string
	// FaviconURL and LogoURL are same-origin /media/ paths, or empty.
	FaviconURL string
	LogoURL    string
	// URL is the canonical base of the site, e.g. "https://example.de".
	URL string
	// Snippets are the reusable blocks, by key, so a layout can place one
	// outside the page body: {{index .Site.Snippets "footer-kontakt"}}.
	//
	// The values are already sanitised — they went through the same
	// goldmark-then-bluemonday pipeline as page content.
	Snippets map[string]template.HTML
	// Terms are the labels in use on this website, most used first, so a layout
	// can offer them as a way into the content.
	Terms []TermLink
	// Design is the website's own :root rule, or empty when the operator has
	// changed nothing. A theme places it after its own stylesheet so the
	// custom properties win.
	Design template.CSS
	// HasSearch says whether this site answers /suche at all.
	//
	// The search is a plugin, so it may simply not be installed. A theme that
	// offers a search box regardless would be pointing visitors at a page that
	// does not exist — the one link on a site that must never be broken is the
	// one the site itself draws.
	HasSearch bool

	// FeedURL is the address of the Atom feed in the language being served:
	// "/feed.xml", or "/fr/feed.xml" under a second language. A theme that
	// hard-codes "/feed.xml" offers French readers the German feed.
	FeedURL string
	// Sprachen are the site's languages, main one first, each pointing at the
	// current address in that language. Empty on a website with one language,
	// so a theme can hide the switcher with a single check.
	//
	// German, like Page.Felder, because it is a name a theme author types.
	Sprachen []LanguageLink
}

// LanguageLink is one language in the switcher.
type LanguageLink struct {
	// Code is the tag for hreflang and lang, always filled — the main language
	// is stored as the empty string, but printing lang="" would be worse than
	// printing a wrong language.
	Code string
	// Name is the language as it calls itself: "Français", "Italiano". A
	// switcher is read by somebody who does not speak the page they are on.
	Name string
	// URL is the address of this page, or of the site, in that language.
	URL string
	// Active marks the language being shown right now.
	Active bool
}

// PageContent holds page-level information for template rendering.
type PageContent struct {
	Title       string
	ContentHTML template.HTML
	Slug        string
	PublishedAt *time.Time
	UpdatedAt   *time.Time
	// Excerpt is a short plain-text summary, used for listings and as the
	// fallback for the meta description.
	Excerpt string
	// HasOwnHeading is true when the content already opens with an <h1>, which
	// happens whenever an editor starts the Markdown with "# ".
	//
	// A theme that prints the page title as an <h1> too would then show the
	// heading twice — the most visible defect on a freshly created start page.
	HasOwnHeading bool

	// IsPost marks an archive entry, so a theme can print the date on a post
	// and leave it off an "Über uns" that has no meaningful one.
	IsPost bool
	// Art is the key of the website's own content kind — "produkt", "termin" —
	// and empty for the built-in page and post. A theme written for one website
	// uses it to lay a product out differently from an ordinary page without
	// having to guess from which fields happen to be filled.
	Art string
	// Prev and Next are the neighbouring entries of a post, oldest-wards and
	// newest-wards. Both are nil on a page and at the ends of the archive.
	Prev *PageLink
	Next *PageLink
	// ArchiveURL is the address of the post archive, or empty when the site has
	// none — a post can then link back to the list it came from.
	ArchiveURL string
	// Terms are the labels on this page, so a theme can print them under the
	// text and let a reader find related content.
	Terms []TermLink

	// Felder are the website's own fields, resolved to the types they mean:
	// {{ .Page.Felder.preis }} prints a price, {{ if .Page.Felder.verfuegbar }}
	// asks a question. German, unlike the rest of this struct, because it is
	// the one part a person writing a theme types out — and they type the
	// field names in German too.
	Felder map[string]any
	// Feldliste are the same fields in their defined order, with their labels.
	//
	// The list is what a shipped theme can use: it cannot know that this
	// website calls a field "Preis pro Kilo", but it can print label and value.
	// Felder is for a theme written for one particular site.
	Feldliste []field.Entry

	// Uebersetzungen are the languages this page really exists in, this one
	// included. A language the page has not been translated into is missing
	// from the list: a switcher that offers French and then answers 404 is
	// worse than one that offers nothing.
	//
	// This is what <link rel="alternate" hreflang> is built from. Site.Sprachen
	// is the whole site's list and is the right thing on the archive or the
	// search, where there is no single page to translate.
	Uebersetzungen []LanguageLink
}

// PageLink is a reference to another page, for prev/next navigation.
type PageLink struct {
	Title string
	URL   string
}

// MetaData holds meta information for the page.
type MetaData struct {
	CanonicalURL string
	// Description is what ends up in <meta name="description">: the page's own
	// value, else its excerpt, else the site-wide default.
	Description string
	// OGImage is an absolute URL to the preview image, or empty.
	OGImage string
	// NoIndex asks search engines to leave this page out.
	NoIndex bool
	// Message carries the operator's text on the maintenance page.
	Message string
	// StructuredData is the schema.org JSON-LD for this page, ready to be
	// placed inside <script type="application/ld+json">. Empty when the site
	// has nothing worth saying, so a theme emits no empty element.
	//
	// It is built in Go rather than in the theme: a theme author who gets one
	// field wrong produces a result that looks fine and is silently ignored,
	// and the failure is invisible from the site itself.
	StructuredData template.JS
}

// BuiltinTemplates is the list of templates shipped inside the binary.
var BuiltinTemplates = []struct {
	Name string
	Slug string
}{
	{Name: "Werkstatt", Slug: "default"},
	{Name: "Schlicht", Slug: "schlicht"},
	{Name: "Magazine", Slug: "magazine"},
	{Name: "Midnight", Slug: "midnight"},
	{Name: "Journal", Slug: "journal"},
	{Name: "Rudel", Slug: "rudel"},
	{Name: "Weide", Slug: "weide"},
	{Name: "Holzcloud", Slug: "holzcloud"},
}

// Loader loads and caches public templates with disk-first, embed-fallback resolution.
type Loader struct {
	dataDir   string
	defaultFS fs.FS
	publicFS  fs.FS // full templates/public FS with all built-in templates
	resolver  TemplateResolver
	cache     sync.Map // cacheKey -> *template.Template
}

// NewLoader creates a template loader.
// dataDir is the runtime data directory (e.g., "data").
// defaultFS should be the embedded default public template FS (sub'd to "public/default").
// publicFS should be the embedded FS for "templates/public" (all built-in templates).
// resolver can be nil for backward compat (falls back to embedded default).
func NewLoader(dataDir string, defaultFS fs.FS, publicFS fs.FS, resolver TemplateResolver) *Loader {
	return &Loader{
		dataDir:   dataDir,
		defaultFS: defaultFS,
		publicFS:  publicFS,
		resolver:  resolver,
	}
}

// funcMap returns template helper functions for one website's locale and zone.
//
// It takes parameters rather than being a package-level constant because
// formatDate has to speak the site's language; the locale is part of cacheKey
// so two websites with different settings cannot share a parsed set.
func funcMap(locale, timezone string) template.FuncMap {
	dates := newDateFormatter(locale, timezone)
	return template.FuncMap{
		// safeHTML marks a string as safe HTML for template rendering.
		// This is SAFE because content_html is always pre-sanitized by bluemonday
		// on page save (see internal/page/markdown.go). The raw goldmark output
		// is never stored -- only the bluemonday-sanitized version reaches here.
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		// formatDate writes the date out in the site's language, e.g.
		// "2. April 2026". It used to be hard-wired to "January 2, 2006", so a
		// German site published its articles with US dates.
		"formatDate":      dates.formatLong,
		"formatDateShort": dates.formatShort,
		// formatDateISO is for the datetime attribute of <time>, which must stay
		// machine-readable regardless of language.
		"formatDateISO": dates.formatISO,
		"formatWeekday": dates.formatWeekday,
		// menu renders a menu by location key as nested <ul><li> HTML.
		// Called in templates as {{menu .Menus "main"}}.
		"menu": func(menus map[string][]menu.MenuNode, locationKey string) template.HTML {
			return menu.RenderMenu(menus, locationKey)
		},
		// menuFor is menu with the current path, so the active entry can carry
		// aria-current="page".
		"menuFor": func(menus map[string][]menu.MenuNode, locationKey, currentPath string) template.HTML {
			return menu.RenderMenuCurrent(menus, locationKey, currentPath)
		},
	}
}

// viewFiles are the per-view template files. Each one supplies the "content"
// block that layout.html pulls in, so they all define a template of the same
// name and must never be parsed into one set — the last one parsed would win and
// every view would render the same body.
var viewFiles = []string{"home.html", "page.html", "404.html", "maintenance.html",
	"search.html", "list.html", "gate.html", "shop.html", "product.html", "cart.html",
	"checkout.html", "order.html"}

// layoutFile is the entry point of a public template; it is the template that
// gets executed, with the view supplying "content".
const layoutFile = "layout.html"

// requiredViews must come from the winning source. Everything else falls back
// to the embedded default per file, which is what tmplmgr's upload validation
// and the README both promise: an archive only has to carry layout.html and
// page.html.
//
// layout.html itself always stays bound to the winning source, so a theme can
// never end up with its own layout and a foreign body.
var requiredViews = map[string]bool{
	layoutFile:  true,
	"page.html": true,
}

// cacheKey identifies one parsed template set.
//
// locale and timezone are part of the key because the date helpers are baked
// into the FuncMap at parse time. Without them, two websites with different
// settings would share a set and the second one would render the first one's
// language.
type cacheKey struct {
	websiteID int64
	view      string
	locale    string
	timezone  string
}

// fileReader reads a template file by name from whichever source won resolution.
type fileReader func(name string) ([]byte, error)

// resolveSource picks the template source for a website: a user-uploaded
// template on disk, a built-in template from the embedded FS, or the embedded
// default. Resolution happens once per template set so a website cannot end up
// with layout.html from one source and its view from another.
func (l *Loader) resolveSource(ctx context.Context, websiteID int64) fileReader {
	slug := ""
	if l.resolver != nil {
		if s, err := l.resolver.ActiveTemplateSlug(ctx, websiteID); err == nil {
			slug = s
		}
	}

	// 1. Disk (user-uploaded template for the active slug)
	if safeSlug(slug) {
		diskPath := filepath.Join(l.dataDir, "templates", slug)
		if _, err := os.Stat(filepath.Join(diskPath, layoutFile)); err == nil {
			return func(name string) ([]byte, error) {
				return os.ReadFile(filepath.Join(diskPath, name))
			}
		}
	}

	// 2. Built-in template of that slug from the embedded FS
	if safeSlug(slug) && l.publicFS != nil {
		if content, err := fs.ReadFile(l.publicFS, slug+"/"+layoutFile); err == nil && len(content) > 0 {
			return func(name string) ([]byte, error) {
				return fs.ReadFile(l.publicFS, slug+"/"+name)
			}
		}
	}

	// 3. Embedded default
	return func(name string) ([]byte, error) {
		return fs.ReadFile(l.defaultFS, name)
	}
}

// loadTemplates parses the template set for one view of a website: layout.html
// plus the view file that provides its "content" block.
func (l *Loader) loadTemplates(ctx context.Context, websiteID int64, view, locale, timezone string) (*template.Template, error) {
	key := cacheKey{websiteID: websiteID, view: view, locale: locale, timezone: timezone}
	if cached, ok := l.cache.Load(key); ok {
		return cached.(*template.Template), nil
	}

	read := l.resolveSource(ctx, websiteID)
	// The root is left unnamed; layout.html and the view are added to the set as
	// named associates. Naming the root layout.html would leave an unparsed
	// template of that name in the set and break escaping.
	tmpl := template.New("").Funcs(funcMap(locale, timezone))

	for _, name := range []string{layoutFile, view} {
		content, err := read(name)
		if err != nil {
			// tmplmgr only requires layout.html and page.html in an archive, and
			// README promises the rest falls back. Without this a conforming
			// upload made GET / fail with 500 on the missing home.html.
			if !requiredViews[name] {
				content, err = fs.ReadFile(l.defaultFS, name)
			}
			if err != nil {
				return nil, fmt.Errorf("read template %s: %w", name, err)
			}
		}
		if _, err := tmpl.New(name).Parse(string(content)); err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
	}

	l.cache.Store(key, tmpl)
	return tmpl, nil
}

// RenderPage renders one view ("home.html", "page.html" or "404.html") into a
// buffer and returns the bytes. The caller is responsible for writing the
// response with appropriate caching headers.
//
// The view file only defines the "content" block, so layout.html is what gets
// executed — executing the view directly produces an empty document.
func (l *Loader) RenderPage(ctx context.Context, websiteID int64, view string, data PageData) ([]byte, error) {
	if !slices.Contains(viewFiles, view) {
		return nil, fmt.Errorf("unknown template view %q", view)
	}

	// Normalise here rather than trusting every caller: an empty locale would
	// render lang="" — worse than a wrong language, because assistive software
	// treats it as "unknown" and falls back to the reader's own setting.
	if data.Site.Locale == "" {
		data.Site.Locale = DefaultLocale
	}
	if data.Site.TimeZone == "" {
		data.Site.TimeZone = DefaultTimeZone
	}

	tmpl, err := l.loadTemplates(ctx, websiteID, view, data.Site.Locale, data.Site.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, layoutFile, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", view, err)
	}
	return buf.Bytes(), nil
}

// Render404 renders the 404 template for a website.
func (l *Loader) Render404(ctx context.Context, websiteID int64, site SiteData) ([]byte, error) {
	title := "Seite nicht gefunden"
	if normalizeLocale(site.Locale) == "en" {
		title = "Page not found"
	}
	data := PageData{
		Site: site,
		Page: PageContent{Title: title},
		Meta: MetaData{NoIndex: true},
	}
	return l.RenderPage(ctx, websiteID, "404.html", data)
}

// RenderMaintenance renders the maintenance page of a deactivated website.
func (l *Loader) RenderMaintenance(ctx context.Context, websiteID int64, site SiteData, message string) ([]byte, error) {
	title := "Wartungsarbeiten"
	if normalizeLocale(site.Locale) == "en" {
		title = "Down for maintenance"
	}
	data := PageData{
		Site: site,
		Page: PageContent{Title: title},
		Meta: MetaData{NoIndex: true, Message: message},
	}
	return l.RenderPage(ctx, websiteID, "maintenance.html", data)
}

// DefaultFS is the built-in default theme.
//
// The upload check needs it: an archive is allowed to omit most views, and the
// ones it omits are served from here — wrapped in the uploaded layout. That
// combination has to be rendered before the archive is accepted, or it is first
// rendered for a visitor.
func (l *Loader) DefaultFS() fs.FS { return l.defaultFS }

// InvalidateTemplateCache removes every cached view for a website.
//
// Call this when the active template changes, its files are modified, or the
// website's locale or time zone is edited — the date helpers are baked into the
// parsed set.
func (l *Loader) InvalidateTemplateCache(websiteID int64) {
	l.cache.Range(func(key, _ any) bool {
		if k, ok := key.(cacheKey); ok && k.websiteID == websiteID {
			l.cache.Delete(key)
		}
		return true
	})
}

// BuiltinAsset returns the content of a built-in template asset by slug and path.
// Returns nil if not found.
func (l *Loader) BuiltinAsset(slug, assetPath string) ([]byte, error) {
	if l.publicFS == nil || !safeSlug(slug) {
		return nil, nil
	}
	clean, ok := SafeAssetPath(assetPath)
	if !ok {
		return nil, nil
	}
	content, err := fs.ReadFile(l.publicFS, slug+"/"+clean)
	if err != nil {
		return nil, nil
	}
	return content, nil
}

// Asset returns a template asset by website ID, checking disk (user-uploaded
// template for the website's active slug), then the built-in template of that
// slug, then the embedded default. Returns nil when the asset does not exist or
// the requested path is not safe.
//
// Both the public site and the admin preview resolve assets through this single
// path so the two cannot drift apart.
func (l *Loader) Asset(ctx context.Context, websiteID int64, assetPath string) ([]byte, error) {
	clean, ok := SafeAssetPath(assetPath)
	if !ok {
		return nil, nil
	}

	slug := l.ResolveSlug(ctx, websiteID)

	// 1. Disk (user-uploaded templates by slug)
	if safeSlug(slug) {
		diskPath := filepath.Join(l.dataDir, "templates", slug, filepath.FromSlash(clean))
		if content, err := os.ReadFile(diskPath); err == nil {
			return content, nil
		}
	}

	// 2. Built-in template by slug
	if slug != "" {
		if content, err := l.BuiltinAsset(slug, clean); err == nil && content != nil {
			return content, nil
		}
	}

	// 3. Embedded default
	content, err := fs.ReadFile(l.defaultFS, clean)
	if err != nil {
		return nil, nil
	}
	return content, nil
}

// safeSlug reports whether a template slug is a single safe path segment.
// Slugs are generated by the admin layer, but they end up in filesystem paths,
// so they are re-validated at every use site.
func safeSlug(slug string) bool {
	if slug == "" || len(slug) > 100 {
		return false
	}
	for _, c := range slug {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}

// ResolveSlug returns the active template slug for a website.
func (l *Loader) ResolveSlug(ctx context.Context, websiteID int64) string {
	if l.resolver == nil {
		return ""
	}
	slug, err := l.resolver.ActiveTemplateSlug(ctx, websiteID)
	if err != nil {
		return ""
	}
	return slug
}

// ViewFiles are the per-view template names an archive may supply.
//
// Exported because the authoring specification has to name all of them, and a
// list maintained separately from this one would eventually stop matching it.
func ViewFiles() []string { return slices.Clone(viewFiles) }
