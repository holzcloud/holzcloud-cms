package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Manager is what the rest of the application talks to.
//
// It holds the three pieces together: the record in the database, the files on
// disk, and the running modules. Every one of them can be right while another
// is wrong — a row for a plugin whose module was deleted, a module for a plugin
// nobody enabled — so the states are reconciled in one place rather than in
// every caller.
type Manager struct {
	store   *Store
	rt      *Runtime
	dataDir string
	log     *slog.Logger

	// mu guards the snapshot below.
	mu   sync.RWMutex
	snap snapshot
}

// snapshot is the routing table, rebuilt whenever something is installed,
// enabled, disabled or removed.
//
// Kept in memory because the alternative is a database round trip on every
// request to ask which plugins care about this page. That is the difference
// between a plugin system that costs nothing and one an operator can feel.
type snapshot struct {
	// byHook maps a hook to the plugin ids that want it, in a stable order.
	byHook map[string][]string
	// sites maps a plugin id to the websites it acts on. Empty means none.
	sites map[string]map[int64]bool
	// routes maps a claimed path to the plugin that owns it.
	routes map[string]string
	// admin is the sidebar, in display order.
	admin []AdminLink
}

// AdminLink is one plugin's entry in the admin.
type AdminLink struct {
	PluginID   string
	Label      string
	PerWebsite bool
	AdminOnly  bool
}

// NewManager wires the pieces together and loads what is enabled.
func NewManager(ctx context.Context, store *Store, rt *Runtime, dataDir string, log *slog.Logger) (*Manager, error) {
	if log == nil {
		log = slog.Default()
	}
	m := &Manager{store: store, rt: rt, dataDir: dataDir, log: log}
	if err := os.MkdirAll(m.root(), 0o755); err != nil {
		return nil, fmt.Errorf("plugin-verzeichnis anlegen: %w", err)
	}
	if err := m.Sync(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) root() string         { return filepath.Join(m.dataDir, "plugins") }
func (m *Manager) dir(id string) string { return filepath.Join(m.root(), id) }

// Sync brings the running modules in line with the database.
//
// A plugin that cannot be loaded is recorded and skipped, never fatal: one
// broken module must not stop a server that is serving four other websites
// perfectly well.
func (m *Manager) Sync(ctx context.Context) error {
	installed, err := m.store.List(ctx)
	if err != nil {
		return err
	}

	want := map[string]bool{}
	for _, p := range installed {
		if !p.Enabled || p.Manifest == nil {
			continue
		}
		want[p.ID] = true
		if m.rt.Loaded(p.ID) {
			continue
		}
		module, err := os.ReadFile(filepath.Join(m.dir(p.ID), ModuleName))
		if err != nil {
			m.note(ctx, p.ID, fmt.Errorf("das Modul fehlt auf der Platte: %w", err))
			continue
		}
		if err := m.rt.Load(ctx, p.Manifest, module); err != nil {
			m.note(ctx, p.ID, err)
			continue
		}
		m.log.Info("plugin loaded", "plugin", p.ID, "version", p.Version)
	}

	// Anything running that should not be.
	for _, id := range m.rt.LoadedIDs() {
		if !want[id] {
			m.rt.Unload(ctx, id)
		}
	}

	m.rebuild(installed)
	return nil
}

// note records why a plugin is not running.
func (m *Manager) note(ctx context.Context, id string, err error) {
	m.log.Error("plugin not loaded", "plugin", id, "err", err)
	_ = m.store.SetError(ctx, id, err.Error())
}

// rebuild recomputes the routing table.
func (m *Manager) rebuild(installed []Installed) {
	s := snapshot{
		byHook: map[string][]string{},
		sites:  map[string]map[int64]bool{},
		routes: map[string]string{},
	}
	// Sorted by id so two plugins filtering the same page always run in the
	// same order. Content filters are not commutative, and an order that
	// depended on map iteration would produce a page that differs between
	// restarts for no reason anyone could find.
	sort.Slice(installed, func(i, j int) bool { return installed[i].ID < installed[j].ID })

	for _, p := range installed {
		if !p.Enabled || p.Manifest == nil || !m.rt.Loaded(p.ID) {
			continue
		}
		for _, h := range p.Manifest.Hooks {
			s.byHook[h] = append(s.byHook[h], p.ID)
		}
		set := map[int64]bool{}
		for _, w := range p.Websites {
			set[w] = true
		}
		s.sites[p.ID] = set
		for _, r := range p.Manifest.Routes {
			s.routes[r] = p.ID
		}
		if p.Manifest.Admin != nil {
			s.admin = append(s.admin, AdminLink{
				PluginID:   p.ID,
				Label:      p.Manifest.Admin.Label,
				PerWebsite: p.Manifest.Admin.PerWebsite,
				AdminOnly:  p.Manifest.Admin.AdminOnly,
			})
		}
	}

	m.mu.Lock()
	m.snap = s
	m.mu.Unlock()
}

// for returns the plugins that want a hook on a website, cheapest path first.
func (m *Manager) forHook(hook string, websiteID int64) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := m.snap.byHook[hook]
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if m.snap.sites[id][websiteID] {
			out = append(out, id)
		}
	}
	return out
}

// Active reports whether any plugin wants this hook here.
//
// Called before assembling a payload: building a copy of a page's HTML to hand
// to nobody is the kind of cost that only shows up under load.
func (m *Manager) Active(hook string, websiteID int64) bool {
	return len(m.forHook(hook, websiteID)) > 0
}

// AdminLinks returns the sidebar entries.
func (m *Manager) AdminLinks() []AdminLink {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AdminLink, len(m.snap.admin))
	copy(out, m.snap.admin)
	return out
}

// RouteOwner returns the plugin that claims a path, if any.
func (m *Manager) RouteOwner(path string, websiteID int64) (string, bool) {
	m.mu.RLock()
	id, ok := m.snap.routes[path]
	onSite := ok && m.snap.sites[id][websiteID]
	m.mu.RUnlock()
	if !onSite {
		return "", false
	}
	return id, true
}

// --- the hooks the application calls ----------------------------------------

// FilterContent runs every content filter over a page.
//
// The plugins run in turn, each seeing what the one before it produced. A
// plugin that fails is skipped and the text it was given carries on: a broken
// filter costs its own feature, never the page.
func (m *Manager) FilterContent(ctx context.Context, websiteID int64, in ContentIn) string {
	html := in.HTML
	for _, id := range m.forHook(HookContent, websiteID) {
		var out ContentOut
		in.HTML = html
		if err := m.rt.Dispatch(ctx, id, HookContent, websiteID, in, &out); err != nil {
			continue
		}
		if out.Changed && out.HTML != "" {
			html = out.HTML
		}
	}
	return html
}

// HandleRequest offers a request to the plugins that asked to see one first.
//
// The first plugin to say it handled the request wins and the rest are not
// asked. That makes the outcome depend on the order, which is why the order is
// the plugin id and not something incidental.
func (m *Manager) HandleRequest(ctx context.Context, websiteID int64, in RequestIn) (*RequestOut, bool) {
	for _, id := range m.forHook(HookRequest, websiteID) {
		var out RequestOut
		if err := m.rt.Dispatch(ctx, id, HookRequest, websiteID, in, &out); err != nil {
			continue
		}
		if out.Handled {
			return &out, true
		}
	}
	return nil, false
}

// HandleNotFound offers a request nothing else answered to the plugins.
//
// The last word before the 404 page. A plugin that says nothing costs one call
// per miss, which is a price only broken links and scanners pay.
func (m *Manager) HandleNotFound(ctx context.Context, websiteID int64, in RequestIn) (*RequestOut, bool) {
	for _, id := range m.forHook(HookNotFound, websiteID) {
		var out RequestOut
		if err := m.rt.Dispatch(ctx, id, HookNotFound, websiteID, in, &out); err != nil {
			continue
		}
		if out.Handled {
			return &out, true
		}
	}
	return nil, false
}

// HandleRoute lets the plugin that owns a path answer it.
func (m *Manager) HandleRoute(ctx context.Context, websiteID int64, in RequestIn) (*RequestOut, bool) {
	id, ok := m.RouteOwner(in.Path, websiteID)
	if !ok {
		return nil, false
	}
	var out RequestOut
	if err := m.rt.Dispatch(ctx, id, HookRoute, websiteID, in, &out); err != nil {
		return nil, false
	}
	if !out.Handled {
		return nil, false
	}
	return &out, true
}

// Admin renders one plugin's screen.
func (m *Manager) Admin(ctx context.Context, id string, in AdminIn) (*AdminOut, error) {
	var out AdminOut
	if err := m.rt.Dispatch(ctx, id, HookAdmin, in.WebsiteID, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Emit tells the plugins that something happened.
//
// It does not wait and it does not report: an event is delivered after the
// thing already succeeded, and a plugin that is slow to hear about a saved page
// must not make saving a page slow. The context is deliberately not the
// request's, which may be cancelled the moment the response is written.
func (m *Manager) Emit(name string, websiteID int64, data map[string]string) {
	ids := m.forHook(HookEvent, websiteID)
	if len(ids) == 0 {
		return
	}
	in := EventIn{Name: name, WebsiteID: websiteID, Data: data}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, id := range ids {
			_ = m.rt.Dispatch(ctx, id, HookEvent, websiteID, in, nil)
		}
	}()
}

// --- lifecycle --------------------------------------------------------------

// Install reads an uploaded archive, writes it to disk and records it.
//
// Nothing is enabled by it. The operator uploads, looks, and then decides.
func (m *Manager) Install(ctx context.Context, r io.ReaderAt, size int64) (*Manifest, error) {
	pkg, err := ReadPackage(r, size)
	if err != nil {
		return nil, err
	}
	if err := m.store.Install(ctx, pkg); err != nil {
		return nil, err
	}
	if err := m.write(pkg); err != nil {
		return nil, err
	}
	// After the files, so a migration that fails leaves an installed plugin
	// the operator can remove rather than a directory nothing knows about.
	if err := m.store.ApplyMigrations(ctx, pkg.Manifest.ID, pkg.Migrations); err != nil {
		_ = m.store.SetError(ctx, pkg.Manifest.ID, err.Error())
		return pkg.Manifest, err
	}

	// A plugin that was already running has just been replaced on disk; reload
	// so the running module is the one that was installed.
	if m.rt.Loaded(pkg.Manifest.ID) {
		m.rt.Unload(ctx, pkg.Manifest.ID)
	}
	return pkg.Manifest, m.Sync(ctx)
}

// write puts a package on disk, replacing what was there.
func (m *Manager) write(pkg *Package) error {
	dir := m.dir(pkg.Manifest.ID)
	// Removed first: a new version with fewer assets would otherwise keep
	// serving the ones the old version had.
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(pkg.Manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestName), raw, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ModuleName), pkg.Module, 0o644); err != nil {
		return err
	}
	for name, data := range pkg.Assets {
		dst := filepath.Join(dir, filepath.FromSlash(AssetDir+name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Enable switches a plugin on and loads it.
func (m *Manager) Enable(ctx context.Context, id string) error {
	if err := m.store.SetEnabled(ctx, id, true); err != nil {
		return err
	}
	if err := m.Sync(ctx); err != nil {
		return err
	}
	// Sync records the reason on the plugin; the caller needs to know it did
	// not come up so the admin can be told rather than shown a green tick.
	if !m.rt.Loaded(id) {
		p, err := m.store.Get(ctx, id)
		if err == nil && p.LastError != "" {
			return errors.New(p.LastError)
		}
		return fmt.Errorf("das Plugin %q liess sich nicht laden", id)
	}
	return nil
}

// Disable switches a plugin off and unloads it.
func (m *Manager) Disable(ctx context.Context, id string) error {
	if err := m.store.SetEnabled(ctx, id, false); err != nil {
		return err
	}
	return m.Sync(ctx)
}

// SetWebsites changes which sites a plugin acts on.
func (m *Manager) SetWebsites(ctx context.Context, id string, sites []int64) error {
	if err := m.store.SetWebsites(ctx, id, sites); err != nil {
		return err
	}
	return m.Sync(ctx)
}

// Remove unloads a plugin and deletes everything belonging to it.
func (m *Manager) Remove(ctx context.Context, id string) error {
	m.rt.Unload(ctx, id)
	if err := m.store.Remove(ctx, id); err != nil {
		return err
	}
	// After the database: a directory left behind is untidy, a row pointing at
	// files that are gone is a plugin the operator cannot switch off.
	if err := os.RemoveAll(m.dir(id)); err != nil {
		m.log.Error("plugin directory not removed", "plugin", id, "err", err)
	}
	return m.Sync(ctx)
}

// List returns what is installed, with the running state filled in.
func (m *Manager) List(ctx context.Context) ([]Status, error) {
	installed, err := m.store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Status, 0, len(installed))
	for _, p := range installed {
		out = append(out, Status{Installed: p, Running: m.rt.Loaded(p.ID)})
	}
	return out, nil
}

// Get returns one plugin with its running state.
func (m *Manager) Get(ctx context.Context, id string) (*Status, error) {
	p, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Status{Installed: *p, Running: m.rt.Loaded(id)}, nil
}

// Status is a plugin as the admin sees it.
//
// Enabled and Running are two different things and both are shown: a plugin
// that is switched on and not running is the case the operator has to be able
// to see, and LastError says why.
type Status struct {
	Installed
	Running bool
}

// AssetPath is where a plugin's static file lives on disk, or empty if the
// name is not one this plugin may serve.
func (m *Manager) AssetPath(id, name string) string {
	rel, err := safeRelative(AssetDir+name, AssetDir)
	if err != nil || !reID.MatchString(id) {
		return ""
	}
	return filepath.Join(m.dir(id), AssetDir, filepath.FromSlash(rel))
}
