// Package plugin reads, validates and stores the extensions a site runs.
//
// A plugin is a WebAssembly module plus a manifest, packed in a zip. It is
// uploaded in the admin, switched on and off, and removed again — no compiler,
// no rebuilt binary, no forked source tree.
//
// WebAssembly rather than Go's own plugin package: that one needs cgo, dynamic
// linking and a guest built by the exact same toolchain, none of which survives
// a static cross-compile without cgo. A wasm module has no such tie, and it
// runs in a sandbox that cannot read the disk, open a socket or call the
// database except through the host functions this package hands it.
package plugin

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ABIVersion is the calling convention this build speaks.
//
// A plugin names the version it was built against and is refused if it does not
// match. Loading a module that expects a different memory layout would not fail
// with an error — it would read the wrong bytes and behave strangely, which is
// the one outcome worth any amount of strictness to avoid.
const ABIVersion = 1

// ManifestName is the file every package must open with.
const ManifestName = "plugin.json"

// ModuleName is the WebAssembly module inside the package.
const ModuleName = "plugin.wasm"

// AssetDir holds files served verbatim under /plugin-assets/<id>/.
const AssetDir = "assets/"

// MigrationDir holds the plugin's own SQL, applied when it is installed.
const MigrationDir = "migrations/"

// Manifest is what the package declares about itself.
type Manifest struct {
	// ID is the stable name. It is the key everywhere: the table prefix, the
	// asset path, the admin route. Renaming one is installing a different
	// plugin, which is why it may never be derived from the display name.
	ID string `json:"id"`
	// ABI is the calling convention the module was built against.
	ABI int `json:"abi"`

	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	// URL points at the plugin's own page. It is shown as a link and never
	// fetched: this server loads nothing from anywhere at runtime.
	URL string `json:"url,omitempty"`
	// License is informational, for the admin who has to answer for what runs
	// on their machine.
	License string `json:"license,omitempty"`

	// Hooks are the points the module wants to be called at. A hook that is
	// not declared is never dispatched, so a plugin costs nothing at the
	// places it does not touch.
	Hooks []string `json:"hooks,omitempty"`
	// Routes are public paths the plugin answers. They are claimed at install
	// time so a collision is refused while it can still be undone, rather than
	// discovered when two plugins both think they own /suche.
	Routes []string `json:"routes,omitempty"`
	// Permissions are what the plugin may reach. Anything not listed is
	// refused by the host, not merely undocumented.
	Permissions []string `json:"permissions,omitempty"`

	// Admin describes the entry in the admin sidebar, if the plugin has one.
	Admin *AdminEntry `json:"admin,omitempty"`
}

// AdminEntry is a plugin's own screen in the admin.
type AdminEntry struct {
	// Label is what the sidebar says.
	Label string `json:"label"`
	// PerWebsite puts the screen under a website rather than beside the
	// global settings. Most plugins are about one site's content.
	PerWebsite bool `json:"per_website,omitempty"`
	// AdminOnly keeps editors out. A plugin that touches settings rather than
	// content says so here.
	AdminOnly bool `json:"admin_only,omitempty"`
}

// Hooks the host dispatches. A plugin declares the ones it wants.
const (
	// HookContent filters a page's HTML before it is sent. This is where a
	// marker in the text becomes something else.
	HookContent = "content"
	// HookRequest runs before a public request is routed and may answer it
	// outright — a redirect, a 410, a page of its own.
	HookRequest = "request"
	// HookNotFound runs only when the core found nothing, just before the 404
	// page, and may answer instead.
	//
	// Separate from HookRequest because of what it costs: a request hook is
	// dispatched on every page view, so a plugin that only cares about addresses
	// that do not exist would pay a call per visitor to say nothing. Here it
	// pays one per miss. It is the hook a redirect table wants.
	HookNotFound = "notfound"
	// HookRoute answers one of the paths the manifest claims.
	HookRoute = "route"
	// HookAdmin renders the plugin's own admin screen.
	HookAdmin = "admin"
	// HookEvent is told that something happened. It cannot change the outcome,
	// which is what makes it safe to run after the fact.
	HookEvent = "event"
)

// Permissions a plugin can ask for.
const (
	// PermStore is the plugin's own key/value space. Every plugin that keeps
	// anything needs it, and it can never see another plugin's keys.
	PermStore = "store"
	// PermPagesRead allows reading published pages of the current website.
	PermPagesRead = "pages:read"
	// PermPagesWrite allows creating and changing pages.
	PermPagesWrite = "pages:write"
	// PermMediaRead allows listing and reading media metadata.
	PermMediaRead = "media:read"
	// PermSettings allows reading the website's settings.
	PermSettings = "settings"
	// PermLog allows writing to the server log.
	PermLog = "log"
	// PermNotify allows sending one notification to the operator of the
	// website — and to nobody else.
	//
	// The recipient is never the plugin's to choose. A plugin that could name
	// an address would be a mail relay with a web interface, and the first
	// thing found on the internet would use it to send someone else's spam.
	PermNotify = "notify"
	// PermRender allows drawing a public page in the website's own theme.
	//
	// Its own permission rather than a free operation, because it is the one
	// that puts a plugin's markup in front of visitors: a manifest asking for it
	// is a plugin that appears on the public site, and that should be visible
	// before it is switched on, not after.
	PermRender = "render"
)

var (
	knownHooks = map[string]bool{
		HookContent: true, HookRequest: true, HookRoute: true,
		HookAdmin: true, HookEvent: true, HookNotFound: true,
	}
	knownPermissions = map[string]bool{
		PermStore: true, PermPagesRead: true, PermPagesWrite: true,
		PermMediaRead: true, PermSettings: true, PermLog: true,
		PermRender: true, PermNotify: true,
	}
	// reID is deliberately narrower than a slug: it becomes a table prefix and
	// a path segment, and a name that needs quoting in either is a name that
	// will eventually be quoted wrong.
	reID      = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	reVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([.\-+][0-9A-Za-z.\-]+)?$`)
)

// reservedIDs are names the host owns. A plugin taking one would shadow a
// built-in route or an admin screen and be unreachable, or worse, reachable.
var reservedIDs = map[string]bool{
	"admin": true, "core": true, "holzcloud": true, "system": true,
	"media": true, "assets": true, "plugin": true, "plugins": true,
}

// reservedRoutes are the paths the server already answers. A plugin claiming
// one would never be called, because the built-in route is registered first —
// a failure with no error message anywhere, which is the kind worth refusing.
var reservedRoutes = map[string]bool{
	"/": true, "/admin": true, "/media": true, "/assets": true, "/t": true,
	"/healthz": true, "/readyz": true, "/sitemap.xml": true, "/robots.txt": true,
	"/feed.xml": true, "/freischalten": true, "/vorschau": true,
	"/plugin-assets": true,
}

// MaxManifestBytes bounds the manifest inside a package.
const MaxManifestBytes = 64 << 10

// ParseManifest reads and checks a manifest.
//
// Unknown fields are refused. The manifest is written by hand by whoever built
// the plugin, and a mistyped key would otherwise be dropped in silence: a
// plugin that declared "permissons" would install, run, and fail at the first
// call with a permission error nobody can explain.
func ParseManifest(data []byte) (*Manifest, error) {
	if len(data) > MaxManifestBytes {
		return nil, fmt.Errorf("%s ist größer als %d Bytes", ManifestName, MaxManifestBytes)
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()

	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", ManifestName, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks everything that can be checked without running the module.
func (m *Manifest) Validate() error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if !reID.MatchString(m.ID) {
		add("die Kennung %q ist nicht zulässig: erlaubt sind Kleinbuchstaben, "+
			"Ziffern und Bindestriche, beginnend mit einem Buchstaben", m.ID)
	}
	if reservedIDs[m.ID] {
		add("die Kennung %q ist für das System reserviert", m.ID)
	}
	if m.ABI != ABIVersion {
		add("das Plugin ist für Schnittstelle %d gebaut, diese Fassung spricht %d",
			m.ABI, ABIVersion)
	}
	if strings.TrimSpace(m.Name) == "" {
		add("der Anzeigename fehlt")
	}
	if len(m.Name) > 80 {
		add("der Anzeigename ist länger als 80 Zeichen")
	}
	if !reVersion.MatchString(m.Version) {
		add("die Fassung %q ist keine Versionsnummer der Form 1.2.3", m.Version)
	}
	if len(m.Description) > 500 {
		add("die Beschreibung ist länger als 500 Zeichen")
	}
	// http and https only: the value ends up in an href, and javascript: in a
	// link the admin clicks is the oldest trick against an admin.
	if m.URL != "" && !strings.HasPrefix(m.URL, "https://") && !strings.HasPrefix(m.URL, "http://") {
		add("die Adresse %q ist keine http- oder https-Adresse", m.URL)
	}

	for _, h := range m.Hooks {
		if !knownHooks[h] {
			add("den Haken %q gibt es nicht", h)
		}
	}
	for _, p := range m.Permissions {
		if !knownPermissions[p] {
			add("die Berechtigung %q gibt es nicht", p)
		}
	}
	if len(m.Hooks) == 0 && len(m.Routes) == 0 {
		add("das Plugin nennt weder einen Haken noch eine Adresse und könnte nie aufgerufen werden")
	}

	seen := map[string]bool{}
	for _, r := range m.Routes {
		switch {
		case !strings.HasPrefix(r, "/"):
			add("die Adresse %q beginnt nicht mit einem Schrägstrich", r)
		case strings.Contains(r, ".."), strings.Contains(r, "//"):
			add("die Adresse %q ist nicht zulässig", r)
		case reservedRoutes[strings.TrimSuffix(r, "/")]:
			add("die Adresse %q gehört bereits dem Server", r)
		case seen[r]:
			add("die Adresse %q steht doppelt", r)
		}
		seen[r] = true
	}

	if m.Admin != nil {
		if strings.TrimSpace(m.Admin.Label) == "" {
			add("der Eintrag für die Verwaltung hat keine Beschriftung")
		}
		if len(m.Admin.Label) > 40 {
			add("die Beschriftung in der Verwaltung ist länger als 40 Zeichen")
		}
		if !m.Declares(HookAdmin) {
			add("das Plugin will einen Eintrag in der Verwaltung, hat aber den Haken %q nicht", HookAdmin)
		}
	}
	if len(m.Routes) > 0 && !m.Declares(HookRoute) {
		add("das Plugin beansprucht Adressen, hat aber den Haken %q nicht", HookRoute)
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s: %s", ManifestName, strings.Join(problems, "; "))
}

// Declares reports whether the plugin asked for a hook.
func (m *Manifest) Declares(hook string) bool {
	for _, h := range m.Hooks {
		if h == hook {
			return true
		}
	}
	return false
}

// Allows reports whether the plugin asked for a permission.
func (m *Manifest) Allows(perm string) bool {
	for _, p := range m.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}
