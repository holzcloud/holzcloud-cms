package plugin

// The calling convention between the host and a plugin.
//
// Two rules shape all of it. Everything crosses the boundary as bytes in the
// guest's own linear memory, because that is the only thing WebAssembly can
// pass; and the host never allocates inside the guest, because doing so means
// calling back into a module that is in the middle of calling out, and that
// re-entrancy is where this kind of bridge grows its hardest bugs.
//
// So: the guest hands the host a buffer it already owns, and the host either
// fills it or says how much room it would have needed. Two calls in the worst
// case, none of them re-entrant, and a guest that never has to trust a pointer
// it did not make.
//
// Payloads are JSON. Not because it is fast — it is not — but because a plugin
// author debugging why a hook does nothing can print the bytes and read them.
// The calls that matter run once per request, not once per row.

// HostModule is the import namespace the guest links against.
const HostModule = "holzcloud"

// Functions the host exports to the guest.
const (
	// FnCall is the only way into the host: an operation name, an argument,
	// and a buffer to write the answer into. One function rather than thirty
	// keeps the import section of every plugin identical, so the host can add
	// an operation without any existing module needing a rebuild.
	FnCall = "hc_call"
)

// Functions the guest must export.
const (
	// GuestAlloc reserves n bytes in guest memory and returns the offset. The
	// host uses it to hand over the input of a hook. The guest owns what it
	// returns and is free to reuse or drop it after the call.
	GuestAlloc = "hc_alloc"
	// GuestHandle is the single entry point. It receives the hook name and the
	// payload, and returns offset and length of its answer, packed.
	GuestHandle = "hc_handle"
)

// Status codes in the high half of a hc_call result.
const (
	// StatusOK means the low half is the number of bytes written.
	StatusOK = 0
	// StatusError means the low half is the length of an error message that
	// was written into the buffer.
	StatusError = 1
	// StatusShort means nothing was written and the low half is how many bytes
	// the answer needs. The guest allocates that much and calls again.
	StatusShort = 2
	// StatusDenied means the plugin did not declare the permission this
	// operation needs. It is deliberately distinct from a plain error: it is a
	// mistake in the manifest, not in the code, and the message says so.
	StatusDenied = 3
)

// pack puts a status and a length into the single i64 a wasm function returns.
func pack(status, n uint32) uint64 { return uint64(status)<<32 | uint64(n) }

// unpack splits what the guest returned from hc_handle: an offset and a length.
func unpack(v uint64) (ptr, n uint32) { return uint32(v >> 32), uint32(v) }

// Operations a guest may ask the host for.
//
// The name carries the permission it needs, so a reader of a manifest can tell
// what a plugin will be able to do without reading its code.
const (
	OpLog         = "log"
	OpStoreGet    = "store.get"
	OpStoreSet    = "store.set"
	OpStoreDelete = "store.delete"
	OpStoreList   = "store.list"
	OpSettings    = "settings"
	OpPagesList   = "pages.list"
	OpPagesGet    = "pages.get"
	OpPagesSearch = "pages.search"
	OpRender      = "render"
	OpNotify      = "notify"
)

// opPermission maps an operation to the permission that unlocks it.
var opPermission = map[string]string{
	OpLog:         PermLog,
	OpStoreGet:    PermStore,
	OpStoreSet:    PermStore,
	OpStoreDelete: PermStore,
	OpStoreList:   PermStore,
	OpSettings:    PermSettings,
	OpPagesList:   PermPagesRead,
	OpPagesGet:    PermPagesRead,
	OpPagesSearch: PermPagesRead,
	OpRender:      PermRender,
	OpNotify:      PermNotify,
}

// --- payloads ---------------------------------------------------------------

// LogArg is what OpLog takes.
type LogArg struct {
	// Level is "debug", "info", "warn" or "error". Anything else is logged at
	// info: a plugin should not be able to make a line disappear by inventing
	// a level, nor to raise one to error.
	Level   string `json:"level"`
	Message string `json:"message"`
}

// StoreArg is what the store operations take.
type StoreArg struct {
	Key string `json:"key"`
	// Value is only read by store.set.
	Value string `json:"value,omitempty"`
	// Prefix and Limit are only read by store.list.
	Prefix string `json:"prefix,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	// Global asks for the plugin's installation-wide space instead of the
	// current website's. A plugin that keeps a setting rather than content
	// says so here.
	Global bool `json:"global,omitempty"`
}

// StoreResult is what store.get returns.
type StoreResult struct {
	Value string `json:"value"`
	Found bool   `json:"found"`
}

// SettingsResult is what OpSettings returns: the parts of a website a plugin
// may see. Deliberately not the whole record — a plugin has no business with
// the offline message or the design tokens.
type SettingsResult struct {
	WebsiteID   int64  `json:"website_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Locale      string `json:"locale"`
	TimeZone    string `json:"timezone"`
	URL         string `json:"url,omitempty"`
	BlogBase    string `json:"blog_base,omitempty"`
	// ContactEmail is the address the operator publishes. It is on the site
	// already, so a plugin that draws a contact form may offer it too.
	ContactEmail string `json:"contact_email,omitempty"`
}

// MaxPagesLimit bounds how many pages one call may return.
//
// A plugin asking for everything on a site with two thousand pages would build
// a payload of megabytes and copy it into a sandbox with sixteen. The limit is
// enforced by the host, not suggested to the plugin.
const MaxPagesLimit = 100

// PagesArg is what the page operations take.
type PagesArg struct {
	// Slug selects one page, for pages.get.
	Slug string `json:"slug,omitempty"`
	// Query is the search terms, for pages.search.
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
	// Offset pages through pages.list.
	Offset int `json:"offset,omitempty"`
	// PostsOnly narrows pages.list to archive entries.
	PostsOnly bool `json:"posts_only,omitempty"`
	// WithFields asks for the website's own fields along with each page.
	//
	// Off by default and asked for explicitly: a shop needs the price, and a
	// search does not — sending them always would put a copy of every page's
	// extra data through a sandbox that has sixteen megabytes in total.
	WithFields bool `json:"with_fields,omitempty"`
}

// PageInfo is one page as a plugin sees it.
//
// Published pages only, in every operation and every hook, including the admin
// one. A plugin that could read drafts would be a way around the rule that an
// unfinished page is not public, and the place that rule has to hold is here —
// the host — not in the good intentions of whoever wrote the module.
type PageInfo struct {
	ID    int64  `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	// Excerpt is the short plain-text summary; HTML is the full body and is
	// filled only by pages.get, because sending it for a list of a hundred
	// pages would be the whole site in one payload.
	Excerpt string `json:"excerpt,omitempty"`
	HTML    string `json:"html,omitempty"`
	// Snippet is the matching passage with <mark> around the terms, from
	// pages.search.
	Snippet     string `json:"snippet,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	IsPost      bool   `json:"is_post,omitempty"`

	// Fields and Groups are the website's own fields, filled only when the
	// query asked for them. The values are as stored: a picture is its media
	// id, a number is the text that was typed.
	Fields map[string]string              `json:"fields,omitempty"`
	Groups map[string][]map[string]string `json:"groups,omitempty"`
}

// PagesResult is what the page operations return.
type PagesResult struct {
	Pages []PageInfo `json:"pages"`
	// Total is how many there are in all, so a plugin can page through.
	Total int `json:"total,omitempty"`
}

// RenderArg asks the host to draw a public page in the website's theme.
//
// This is what makes a public plugin look like part of the site rather than a
// bare page in the wrong font. The plugin supplies the content; the header, the
// menus, the stylesheet and the footer are the theme's, and the plugin never
// sees them.
type RenderArg struct {
	Title string `json:"title"`
	Slug  string `json:"slug,omitempty"`
	// HTML is the body. It is sanitised with the same policy as page content
	// before it reaches a template, so a plugin cannot put a script on the
	// public site any more than an editor can.
	HTML string `json:"html,omitempty"`
	// Description fills the meta description; NoIndex keeps the page out of
	// search engines, which is what a result list wants.
	Description string `json:"description,omitempty"`
	NoIndex     bool   `json:"no_index,omitempty"`

	// View selects a theme view. Empty means the ordinary page view. Only the
	// names below are accepted: a plugin naming a file would be a way to ask the
	// theme loader for something that is not a view at all.
	View string `json:"view,omitempty"`
	// Search fills the search view, which every theme already styles. Ignored
	// for any other view.
	Search *RenderSearch `json:"search,omitempty"`
}

// Views a plugin may ask for.
const (
	ViewPage   = "page.html"
	ViewSearch = "search.html"
)

// RenderSearch is the result list of the search view.
type RenderSearch struct {
	Query string `json:"query,omitempty"`
	// Submitted separates "nothing typed yet" from "nothing found", which need
	// different words on the page.
	Submitted bool        `json:"submitted,omitempty"`
	Results   []RenderHit `json:"results,omitempty"`
}

// RenderHit is one entry in a rendered result list.
type RenderHit struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	// Snippet may contain <mark>; it is escaped by the host except for that
	// one element, so a plugin cannot smuggle markup into a result row.
	Snippet string `json:"snippet,omitempty"`
}

// NotifyArg is one message to the operator.
//
// No recipient field, on purpose: the host decides where it goes, from the
// website's own settings. What a plugin controls is the subject, the text and
// where a reply should land.
type NotifyArg struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	// ReplyTo lets the operator answer the person the message is about — the
	// visitor who sent an enquiry, say. It is checked by the host and dropped
	// if it is not an address.
	ReplyTo string `json:"reply_to,omitempty"`
}

// NotifyResult says what happened.
type NotifyResult struct {
	// Queued false with no error means there was nowhere to send it: no mail
	// server, or no notification address on this website. That is a normal
	// state and not a failure, and a plugin should not treat it as one.
	Queued bool   `json:"queued"`
	Reason string `json:"reason,omitempty"`
}

// MaxNotifyBytes bounds one notification.
const MaxNotifyBytes = 16 << 10

// RenderResult is the finished page.
type RenderResult struct {
	HTML string `json:"html"`
}

// --- hook payloads ----------------------------------------------------------

// ContentIn is what the content hook receives.
type ContentIn struct {
	WebsiteID int64  `json:"website_id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	HTML      string `json:"html"`
	// Query is the current request's query string.
	//
	// A filter that puts a form on a page needs it: the outcome of the last
	// submission comes back in the address, and without it the plugin would
	// have to draw the form and the answer to it in two different places.
	Query string `json:"query,omitempty"`
}

// ContentOut is what it may change.
//
// A hook that returns nothing at all leaves the page alone, which is what makes
// it safe to call a plugin on every page: the cost of a plugin that has nothing
// to say is one JSON round trip, not a mangled page.
type ContentOut struct {
	HTML    string `json:"html,omitempty"`
	Changed bool   `json:"changed"`
}

// RequestIn is what the request, route and notfound hooks receive.
type RequestIn struct {
	WebsiteID int64             `json:"website_id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Query     string            `json:"query,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	// Body is present only for a route hook on a POST, and only up to the
	// host's limit. A plugin cannot ask for more.
	Body string `json:"body,omitempty"`
}

// RequestOut is how a plugin answers, or declines to.
type RequestOut struct {
	// Handled false means the host carries on as if the plugin had not run.
	Handled bool `json:"handled"`
	Status  int  `json:"status,omitempty"`
	// Location turns the answer into a redirect.
	Location string            `json:"location,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	// ContentType defaults to text/html; charset=utf-8.
	ContentType string `json:"content_type,omitempty"`
}

// AdminIn is what the admin hook receives.
type AdminIn struct {
	WebsiteID int64 `json:"website_id,omitempty"`
	// Method, Path and Form let a plugin's screen be a form that posts to
	// itself. The host has already checked the CSRF token and the role.
	Method string              `json:"method"`
	Path   string              `json:"path"`
	Form   map[string][]string `json:"form,omitempty"`
	// Query is the address's query string. Without it a plugin screen has no
	// way to have a second view: every link back to itself would look the
	// same, so "edit this one" would have to be a button, and a list of ten
	// things would be ten buttons that all say "edit".
	Query string `json:"query,omitempty"`
}

// AdminOut is the plugin's screen.
//
// HTML rather than a template: the plugin does not get to reach into the
// admin's own templates, and what it returns is sanitised before it is placed.
type AdminOut struct {
	Title string `json:"title,omitempty"`
	HTML  string `json:"html"`
	// Redirect sends the operator somewhere after a successful form post, so a
	// reload does not repeat it.
	Redirect string `json:"redirect,omitempty"`
	// Flash is shown once above the screen.
	Flash      string `json:"flash,omitempty"`
	FlashError bool   `json:"flash_error,omitempty"`
	// Download hands the operator a file instead of drawing the screen.
	Download *Download `json:"download,omitempty"`
}

// Download is a file a screen hands over instead of drawing itself — a list
// exported as CSV, most likely.
//
// It is deliberately narrow. The host forces the response to be an attachment
// with a content type from its own short list, because a plugin that could
// serve text/html from the administration's own address could put a page there
// that looks like the administration and is not.
type Download struct {
	// Filename is what the browser saves it as. The host strips anything that
	// is not a plain name.
	Filename string `json:"filename"`
	// ContentType has to be one the host allows; anything else becomes
	// application/octet-stream.
	ContentType string `json:"content_type,omitempty"`
	Body        string `json:"body"`
}

// EventIn is what the event hook receives. It cannot change anything, which is
// what makes it safe to deliver after the fact.
type EventIn struct {
	Name      string            `json:"name"`
	WebsiteID int64             `json:"website_id,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

// Event names the host emits.
const (
	EventPageSaved    = "page.saved"
	EventPageDeleted  = "page.deleted"
	EventNotFound     = "request.notfound"
	EventMediaAdded   = "media.added"
	EventFormReceived = "form.received"
)
