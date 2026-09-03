// Package plugin is what a Holzcloud plugin is written against.
//
// A plugin is an ordinary Go program compiled to WebAssembly. It registers the
// hooks it wants and then does nothing until the host calls it:
//
//	package main
//
//	import plugin "github.com/holzcloud/holzcloud-cms/sdk"
//
//	func init() {
//		plugin.OnContent(func(in plugin.ContentIn) (plugin.ContentOut, error) {
//			html := strings.ReplaceAll(in.HTML, "[[jahr]]", "2026")
//			return plugin.ContentOut{HTML: html, Changed: html != in.HTML}, nil
//		})
//	}
//
//	func main() {}
//
// Registration goes in init and not in main, and the empty main is not a
// mistake. A plugin is a WASI reactor: the host starts it with _initialize,
// which runs package initialisation and returns, and main never runs at all. A
// plugin that registered in main would compile, install, load, and then do
// nothing on every hook with no error anywhere — so this package says so out
// loud the first time a hook finds no handler. Registering in init is the same
// shape a database driver uses, so it is not as odd as it first looks.
//
// Build it with:
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
//
// The two functions the host needs — the allocator and the entry point — are
// exported by this package. An author never writes an unsafe pointer, and the
// one place in the whole arrangement that does is here, written once and
// tested, rather than copied into every plugin with a different bug in it.
//
// Everything a plugin may reach goes through this package, and everything it
// reaches has to be declared in plugin.json first. A call to Set without
// "store" in the permissions returns ErrDenied, not a silent no-op: a plugin
// that appears to save and does not is worse than one that stops.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ABIVersion is the calling convention this SDK speaks. It has to match the
// "abi" field in plugin.json and the host that will run the module.
const ABIVersion = 1

// ErrDenied is returned when the manifest did not ask for the permission an
// operation needs.
var ErrDenied = errors.New("das Plugin hat diese Berechtigung nicht")

// --- what a hook receives and returns ---------------------------------------

// ContentIn is a page about to be sent to a visitor.
type ContentIn struct {
	WebsiteID int64  `json:"website_id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	HTML      string `json:"html"`
	// Query is the current request's query string, so a filter can react to
	// what came back in the address — the outcome of a form submission, say.
	Query string `json:"query,omitempty"`
}

// ContentOut is the page after the plugin looked at it.
//
// Changed false leaves the page exactly as it was, which is the answer a plugin
// should give whenever it has nothing to do — it costs one round trip instead
// of a copy of every page.
type ContentOut struct {
	HTML    string `json:"html,omitempty"`
	Changed bool   `json:"changed"`
}

// RequestIn is a public request, for the request and route hooks.
type RequestIn struct {
	WebsiteID int64             `json:"website_id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Query     string            `json:"query,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
}

// RequestOut answers a request, or declines to.
type RequestOut struct {
	// Handled false means the host carries on as if the plugin had not run.
	Handled     bool              `json:"handled"`
	Status      int               `json:"status,omitempty"`
	Location    string            `json:"location,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
}

// AdminIn is a visit to the plugin's own admin screen. The host has already
// checked the operator's role and the CSRF token.
type AdminIn struct {
	WebsiteID int64               `json:"website_id,omitempty"`
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	Form      map[string][]string `json:"form,omitempty"`
	// Query is the address's query string, so a plugin screen can have more
	// than one view and link between them.
	Query string `json:"query,omitempty"`
}

// AdminOut is the screen. The HTML is sanitised by the host before it is
// placed, so a plugin cannot put a script into the admin.
type AdminOut struct {
	Title      string `json:"title,omitempty"`
	HTML       string `json:"html"`
	Redirect   string `json:"redirect,omitempty"`
	Flash      string `json:"flash,omitempty"`
	FlashError bool   `json:"flash_error,omitempty"`
	// Download hands the operator a file instead of drawing the screen. Nil
	// for every ordinary answer.
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

// EventIn is something that happened. A plugin cannot change the outcome,
// which is what makes it safe to be told after the fact.
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

// Page is a published page as a plugin sees it.
//
// Published only, always. There is no way to ask for a draft: an unfinished
// page is not public, and the host holds that line rather than trusting each
// plugin to.
type Page struct {
	ID    int64  `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	// Excerpt is the short plain-text summary. HTML is the full body and is
	// filled only by GetPage — a list of a hundred pages with their bodies
	// would be the whole site in one payload.
	Excerpt string `json:"excerpt,omitempty"`
	HTML    string `json:"html,omitempty"`
	// Snippet is the matching passage with <mark> around the terms, from
	// SearchPages.
	Snippet string `json:"snippet,omitempty"`
	// PublishedAt is RFC 3339 in UTC, or empty.
	PublishedAt string `json:"published_at,omitempty"`
	IsPost      bool   `json:"is_post,omitempty"`

	// Felder and Gruppen are the website's own fields, filled only by the
	// calls that ask for them. As stored: a picture is its media id, a number
	// is the text somebody typed — with a comma, if that is what they typed.
	Felder  map[string]string              `json:"fields,omitempty"`
	Gruppen map[string][]map[string]string `json:"groups,omitempty"`
}

// Feld returns one of the website's own fields, or the empty string.
func (p Page) Feld(key string) string { return p.Felder[key] }

// RenderArg asks the host to draw a public page in the website's theme.
//
// This is what makes a plugin's page look like part of the site: the header,
// the menus, the stylesheet and the footer are the theme's, and the plugin
// supplies only what goes in the middle. Needs "render".
type RenderArg struct {
	Title string `json:"title"`
	Slug  string `json:"slug,omitempty"`
	// HTML is the body. The host sanitises it with the same policy as page
	// content, so anything that executes is removed on the way through.
	HTML        string `json:"html,omitempty"`
	Description string `json:"description,omitempty"`
	// NoIndex keeps the page out of search engines, which is what a result
	// list or a form response wants.
	NoIndex bool `json:"no_index,omitempty"`

	// View selects a theme view: empty or ViewPage for an ordinary page,
	// ViewSearch for the result list every theme already styles.
	View string `json:"view,omitempty"`
	// Search fills the search view and is ignored for any other.
	Search *RenderSearch `json:"search,omitempty"`
}

// Views a plugin may ask for.
const (
	ViewPage   = "page.html"
	ViewSearch = "search.html"
)

// RenderSearch is a result list in the theme's own styling.
type RenderSearch struct {
	Query string `json:"query,omitempty"`
	// Submitted separates "nothing typed yet" from "nothing found", which need
	// different words on the page.
	Submitted bool        `json:"submitted,omitempty"`
	Results   []RenderHit `json:"results,omitempty"`
}

// RenderHit is one entry in a result list.
type RenderHit struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	// Snippet may contain <mark> and nothing else: the host escapes it and
	// puts back only that one element.
	Snippet string `json:"snippet,omitempty"`
}

// Settings are the parts of a website a plugin may see.
type Settings struct {
	WebsiteID   int64  `json:"website_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Locale      string `json:"locale"`
	TimeZone    string `json:"timezone"`
	URL         string `json:"url,omitempty"`
	BlogBase    string `json:"blog_base,omitempty"`
	// ContactEmail is the address the operator publishes.
	ContactEmail string `json:"contact_email,omitempty"`
}

// --- registering hooks ------------------------------------------------------

var (
	onContent  func(ContentIn) (ContentOut, error)
	onRequest  func(RequestIn) (RequestOut, error)
	onRoute    func(RequestIn) (RequestOut, error)
	onNotFound func(RequestIn) (RequestOut, error)
	onAdmin    func(AdminIn) (AdminOut, error)
	onEvent    func(EventIn) error
)

// OnContent filters a page's HTML before it is sent.
func OnContent(f func(ContentIn) (ContentOut, error)) { onContent = f }

// OnRequest runs before a public request is routed and may answer it outright.
func OnRequest(f func(RequestIn) (RequestOut, error)) { onRequest = f }

// OnRoute answers one of the paths declared in plugin.json.
func OnRoute(f func(RequestIn) (RequestOut, error)) { onRoute = f }

// OnNotFound runs only when the core found nothing, just before the 404 page.
//
// Prefer it over OnRequest whenever the plugin only cares about addresses that
// do not exist: a request hook is called on every page view, this one only on a
// miss. It is what a redirect table wants.
func OnNotFound(f func(RequestIn) (RequestOut, error)) { onNotFound = f }

// OnAdmin renders the plugin's screen in the admin.
func OnAdmin(f func(AdminIn) (AdminOut, error)) { onAdmin = f }

// OnEvent is told that something happened.
func OnEvent(f func(EventIn) error) { onEvent = f }

// dispatch routes one call from the host to the registered handler.
//
// A hook nobody registered returns nothing rather than an error: the manifest
// may declare a hook the plugin only handles in a later version, and a hard
// failure there would be a plugin that looks broken while working as intended.
func dispatch(hook string, payload []byte) ([]byte, error) {
	warnIfEmpty()
	switch hook {
	case "content":
		return call(payload, onContent)
	case "request":
		return call(payload, onRequest)
	case "route":
		return call(payload, onRoute)
	case "notfound":
		return call(payload, onNotFound)
	case "admin":
		return call(payload, onAdmin)
	case "event":
		if onEvent == nil {
			return nil, nil
		}
		var in EventIn
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		return nil, onEvent(in)
	}
	return nil, fmt.Errorf("unbekannter Haken %q", hook)
}

// warnIfEmpty says something once when a hook arrives and nothing is
// registered.
//
// This is the mistake every plugin author makes exactly once: registering in
// main, which a reactor module never runs. Without this the plugin is silent
// and correct-looking, and the only symptom is a feature that does not happen.
var warned bool

func warnIfEmpty() {
	if warned || onContent != nil || onRequest != nil || onRoute != nil ||
		onNotFound != nil || onAdmin != nil || onEvent != nil {
		return
	}
	warned = true
	Log("error", "kein Haken registriert — die Registrierung gehört in func init(), "+
		"nicht in func main(): ein Reaktor-Modul führt main nie aus")
}

// call unmarshals, runs a handler and marshals the answer.
func call[In, Out any](payload []byte, f func(In) (Out, error)) ([]byte, error) {
	if f == nil {
		return nil, nil
	}
	var in In
	if err := json.Unmarshal(payload, &in); err != nil {
		return nil, err
	}
	out, err := f(in)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

// --- what a plugin may ask the host for -------------------------------------

// Log writes a line to the server's log. Needs the "log" permission.
//
// The plugin's name is added by the host, so a line can never look like it came
// from somewhere else.
func Log(level, message string) {
	_, _ = hostJSON("log", map[string]string{"level": level, "message": message})
}

// Logf is Log with a format string.
func Logf(level, format string, args ...any) { Log(level, fmt.Sprintf(format, args...)) }

// Get reads one value from the plugin's own space for the current website.
// Needs "store". A missing key is not an error.
func Get(key string) (string, bool, error) { return get(key, false) }

// GlobalGet reads from the plugin's installation-wide space, for settings that
// are not about a particular site.
func GlobalGet(key string) (string, bool, error) { return get(key, true) }

func get(key string, global bool) (string, bool, error) {
	raw, err := hostJSON("store.get", storeArg{Key: key, Global: global})
	if err != nil {
		return "", false, err
	}
	var r struct {
		Value string `json:"value"`
		Found bool   `json:"found"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", false, err
	}
	return r.Value, r.Found, nil
}

// Set writes one value. Needs "store".
func Set(key, value string) error {
	_, err := hostJSON("store.set", storeArg{Key: key, Value: value})
	return err
}

// GlobalSet writes to the installation-wide space.
func GlobalSet(key, value string) error {
	_, err := hostJSON("store.set", storeArg{Key: key, Value: value, Global: true})
	return err
}

// Delete removes one value.
func Delete(key string) error {
	_, err := hostJSON("store.delete", storeArg{Key: key})
	return err
}

// List returns the keys under a prefix with their values, at most limit of
// them. The host caps the limit at a thousand: a plugin should not pull its
// whole storage through the boundary in one call.
func List(prefix string, limit int) (map[string]string, error) {
	raw, err := hostJSON("store.list", storeArg{Prefix: prefix, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	if len(raw) == 0 {
		return out, nil
	}
	return out, json.Unmarshal(raw, &out)
}

// Site returns the current website's settings. Needs "settings".
func Site() (Settings, error) {
	raw, err := hostJSON("settings", struct{}{})
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	return s, json.Unmarshal(raw, &s)
}

// Pages lists published pages, newest first, at most limit of them. Needs
// "pages:read". The host caps the limit at a hundred.
func Pages(limit, offset int) ([]Page, int, error) {
	return pages("pages.list", pagesArg{Limit: limit, Offset: offset})
}

// PagesWithFields is Pages with the website's own fields attached.
//
// A separate call rather than an option on Pages: the fields are a second
// payload per page, and a plugin that does not need them should not pay for
// them on every call.
func PagesWithFields(limit, offset int) ([]Page, int, error) {
	return pages("pages.list", pagesArg{Limit: limit, Offset: offset, WithFields: true})
}

// Posts is Pages narrowed to archive entries.
func Posts(limit, offset int) ([]Page, int, error) {
	return pages("pages.list", pagesArg{Limit: limit, Offset: offset, PostsOnly: true})
}

// GetPage returns one published page with its body, or ok false if there is no
// such page — which is also the answer for a draft.
func GetPage(slug string) (Page, bool, error) {
	list, _, err := pages("pages.get", pagesArg{Slug: slug})
	if err != nil || len(list) == 0 {
		return Page{}, false, err
	}
	return list[0], true, nil
}

// SearchPages runs the site search over published pages. Each result carries a
// Snippet with <mark> around the matching terms.
func SearchPages(query string, limit int) ([]Page, error) {
	list, _, err := pages("pages.search", pagesArg{Query: query, Limit: limit})
	return list, err
}

func pages(op string, arg pagesArg) ([]Page, int, error) {
	raw, err := hostJSON(op, arg)
	if err != nil {
		return nil, 0, err
	}
	var r struct {
		Pages []Page `json:"pages"`
		Total int    `json:"total"`
	}
	if len(raw) == 0 {
		return nil, 0, nil
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, 0, err
	}
	return r.Pages, r.Total, nil
}

// Render draws a page in the website's theme and returns the finished HTML,
// ready to be put in RequestOut.Body. Needs "render".
//
// It only works while a public request is being answered: there is no site to
// draw from an admin screen, and asking there is an error rather than a page.
func Render(a RenderArg) (string, error) {
	raw, err := hostJSON("render", a)
	if err != nil {
		return "", err
	}
	var r struct {
		HTML string `json:"html"`
	}
	return r.HTML, json.Unmarshal(raw, &r)
}

// Notify sends one message to the operator of the current website. Needs
// "notify".
//
// The recipient is not the plugin's to choose: it goes to the notification
// address in the website's settings and nowhere else. A plugin that could name
// an address would be a mail relay with a public web interface.
//
// queued false with a nil error means there was nowhere to send it — no mail
// server, or no address on this website. That is an ordinary state and not a
// failure; reason says which.
func Notify(subject, body, replyTo string) (queued bool, reason string, err error) {
	raw, err := hostJSON("notify", notifyArg{Subject: subject, Body: body, ReplyTo: replyTo})
	if err != nil {
		return false, "", err
	}
	var r struct {
		Queued bool   `json:"queued"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return false, "", err
	}
	return r.Queued, r.Reason, nil
}

type notifyArg struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	ReplyTo string `json:"reply_to,omitempty"`
}

type pagesArg struct {
	Slug       string `json:"slug,omitempty"`
	Query      string `json:"query,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	PostsOnly  bool   `json:"posts_only,omitempty"`
	WithFields bool   `json:"with_fields,omitempty"`
}

type storeArg struct {
	Key    string `json:"key,omitempty"`
	Value  string `json:"value,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Global bool   `json:"global,omitempty"`
}

// hostJSON marshals an argument, calls the host and returns the raw answer.
func hostJSON(op string, arg any) ([]byte, error) {
	a, err := json.Marshal(arg)
	if err != nil {
		return nil, err
	}
	return hostCall(op, a)
}
