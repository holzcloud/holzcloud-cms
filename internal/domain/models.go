package domain

import (
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/locale"
)

// Website represents a managed website.
type Website struct {
	ID          int64
	Name        string
	Description string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Locale drives the lang attribute and the date format of the public site.
	// It is the website's main language: its pages keep their addresses
	// unprefixed, whatever else is switched on.
	Locale string
	// ExtraLocales are the further languages, as stored: "fr, it". Empty for a
	// website in one language, which is every website until somebody adds one.
	ExtraLocales string
	// TimeZone is the zone published dates are rendered in. Stored as an IANA
	// name and resolved through the embedded tzdata, never through the host.
	TimeZone string
	// MetaDescription is the site-wide fallback for <meta name="description">.
	MetaDescription string

	FaviconMediaID *int64
	LogoMediaID    *int64

	// CanonicalRedirect sends visitors of a secondary domain to the primary one
	// with a 301. Off by default: a fresh install reachable only over a bare IP
	// would otherwise redirect itself somewhere unreachable.
	CanonicalRedirect bool

	// OfflineMode decides what an inactive website answers: "notfound" (404,
	// the old behaviour) or "maintenance" (503, which keeps the search index).
	OfflineMode    string
	OfflineMessage string

	// BlogBase is the address of the post archive, without slashes — "aktuelles"
	// serves it at /aktuelles. Empty switches the archive off, which is right
	// for a site that has no posts.
	BlogBase string
	// PostsPerPage is how many entries one archive page shows.
	PostsPerPage int

	// ContactEmail is shown beside the contact form as the direct alternative.
	// Empty leaves it out; nothing is ever mailed from here.
	ContactEmail string
	// NotifyEmail is where notifications about this website go — a new enquiry,
	// say. Separate from ContactEmail, which is published beside the form: the
	// person who reads the enquiries is not necessarily the address printed on
	// the site, and an address on a website collects spam one does not want
	// forwarded as well.
	NotifyEmail string

	// The organisation behind the site, as structured data. OrgType is a
	// schema.org type; empty leaves the whole block out, because a personal
	// blog should not claim to be a shop.
	OrgType      string
	Street       string
	PostalCode   string
	City         string
	Country      string
	Phone        string
	OpeningHours string

	// Design tokens. Each is empty or zero when the theme should decide; see
	// internal/design for the shapes they are validated against.
	TokenInk     string
	TokenPaper   string
	TokenBrand   string
	TokenFont    string
	TokenMeasure int
	TokenRadius  int

	// The shop. ShopBase is the path the catalogue lives under, next to
	// BlogBase; empty switches the shop off entirely, because a CMS hosting
	// several websites cannot assume they all sell something.
	ShopBase string
	// Currency is an ISO 4217 code. Per website rather than compiled in: the
	// same installation may run a Swiss and an Austrian site, and the two are
	// written differently — CHF 1’234.55 against 1.234,55 €.
	Currency string
	// ShippingGross is the flat delivery charge in the currency's smallest
	// unit. ShippingFreeFrom is the basket value from which it is waived, or
	// nil when it never is — 0 would mean "always free".
	ShippingGross    int64
	ShippingFreeFrom *int64
	// ShippingTaxBP is the rate on the delivery charge, in basis points.
	ShippingTaxBP int
	// PriceDisplay is "private", "business" or "both".
	PriceDisplay string
	// VATExempt is the Swiss small-business rule: below CHF 100'000 of annual
	// turnover no tax is shown or charged, and the invoice has to say why.
	VATExempt bool
	// VATNumber is the UID as it appears on invoices: CHE-123.456.789 MWST.
	VATNumber string
	// ReturnPolicy is what the shop promises about sending goods back.
	//
	// Switzerland has no statutory right of withdrawal for online orders, so
	// whatever stands here is a business decision rather than a legal
	// requirement. Empty means nothing is promised, which is lawful and has to
	// be said plainly in the confirmation rather than left to be assumed.
	ReturnPolicy string
	// OrderEmail is where a new order is reported. Deliberately separate from
	// ContactEmail: enquiries and orders are often read by different people,
	// and a shop that sends orders to the general enquiry mailbox buries them.
	OrderEmail string
	// PaymentDetails are the bank particulars for prepayment, as the operator
	// typed them. They go into the confirmation, which for prepayment is the
	// only place the customer ever learns where to transfer the money to.
	PaymentDetails string
}

// Locales returns the additional languages, cleaned.
func (w Website) Locales() []string { return locale.ParseList(w.ExtraLocales, w.Locale) }

// Multilingual reports whether this website has more than one language.
func (w Website) Multilingual() bool { return len(w.Locales()) > 0 }

// HasArchive reports whether this website publishes a post archive.
func (w Website) HasArchive() bool { return w.BlogBase != "" }

// ArchiveURL is the archive's public path.
func (w Website) ArchiveURL() string {
	if w.BlogBase == "" {
		return ""
	}
	return "/" + w.BlogBase
}

// HasShop reports whether this website sells anything.
func (w Website) HasShop() bool { return w.ShopBase != "" }

// ShopURL is the catalogue's public path.
func (w Website) ShopURL() string {
	if w.ShopBase == "" {
		return ""
	}
	return "/" + w.ShopBase
}

// InMaintenance reports whether an inactive website should answer 503 instead
// of 404.
func (w Website) InMaintenance() bool { return !w.Active && w.OfflineMode == "maintenance" }

// Domain represents a domain name assigned to a website.
//
// Domain is always the punycode form, because that is what a browser puts in the
// Host header and therefore what the resolver matches on. Use Display for the
// admin UI so an internationalised name reads the way the owner typed it.
type Domain struct {
	ID        int64
	WebsiteID int64
	Domain    string
	IsPrimary bool
	CreatedAt time.Time
}

// Display returns the human-readable Unicode form of the domain.
func (d Domain) Display() string { return DisplayDomain(d.Domain) }
