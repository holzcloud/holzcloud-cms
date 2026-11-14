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
	// perms maps a plugin id to the permissions it declared, for a caller that
	// has to know BEFORE the plugin is called — see Manager.Allows.
	perms map[string]map[string]bool
}

// AdminLink is one plugin's entry in the admin.
type AdminLink struct {
	PluginID string
	// Label is the source string, in the language the plugin declared it in.
	// LabelIn is what the sidebar actually prints.
	Label      string
	PerWebsite bool
	AdminOnly  bool
	// lang is the plugin's own catalogue, carried along so that the sidebar can
	// resolve the label without a second trip to the store. The snapshot is
	// built once per reload and read on every request; looking the manifest up
	// again for one string would be a lock per menu entry per page view.
	lang map[string]map[string]string
}

// LabelIn is the sidebar entry in the operator's language.
//
// The operator's and not the visitor's: this string only ever appears in the
// admin, so the question sdk.T has to ask both ways round has only one answer
// here.
func (l AdminLink) LabelIn(lang string) string {
	return Manifest{Lang: l.lang}.Text(lang, l.Label)
}

// NewManager wires the pieces together and loads what is enabled.
func NewManager(ctx context.Context, store *Store, rt *Runtime, dataDir string, log *slog.Logger) (*Manager, error) {
	if log == nil {
		log = slog.Default()
	}
	m := &Manager{store: store, rt: rt, dataDir: dataDir, log: log}
	if err := os.MkdirAll(m.root(), 0o755); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
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
			m.note(ctx, p.ID, fmt.Errorf("the module is missing from disk: %w", err))
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
		perms:  map[string]map[string]bool{},
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
		s.perms[p.ID] = map[string]bool{}
		for _, perm := range p.Manifest.Permissions {
			s.perms[p.ID][perm] = true
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
				lang:       p.Manifest.Lang,
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

// Serves reports whether one plugin is loaded, acts on the website and
// declares the hook. A nil manager serves nothing.
//
// This is the question the privacy search asks before it calls a plugin
// (Phase 33.2). It is asked per plugin rather than per hook, unlike Active,
// because the answer the search needs is not "does anybody answer here" but
// "does THIS plugin, which keeps data for the website, answer": a plugin that
// holds a person's data and does not answer has to be named on the screen,
// not silently skipped.
func (m *Manager) Serves(id, hook string, websiteID int64) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.snap.sites[id][websiteID] {
		return false
	}
	for _, p := range m.snap.byHook[hook] {
		if p == id {
			return true
		}
	}
	return false
}

// ActsOn reports whether a plugin is running and switched on for the
// website, whatever hooks it declares. A nil manager acts on nothing.
func (m *Manager) ActsOn(id string, websiteID int64) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snap.sites[id][websiteID]
}

// Allows reports whether one plugin declared a permission.
//
// For the caller that has to prepare something BEFORE the plugin is called and
// only if it may have it — an attachment, whose bytes the host reads off the
// wire and holds. Asking the manifest is the same check the runtime makes on
// the call itself; this one only avoids doing the work for a plugin that would
// be refused anyway.
func (m *Manager) Allows(pluginID, permission string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snap.perms[pluginID][permission]
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

// ErrNotServed is Privacy asked of a plugin that does not answer the privacy
// hook on the website: not installed, switched off, not on the website, or
// without the hook in its manifest.
var ErrNotServed = errors.New("the plugin does not answer the privacy hook on this website")

// ErrWroteDuringFind is a plugin that tried to change its store while it was
// only asked to find. The write was refused; the answer is not trusted.
var ErrWroteDuringFind = errors.New("the plugin tried to write while it was asked to find")

// ErrPrivacyFailed is a plugin that answered the privacy hook with an error:
// a refused key, a value that is not an address, a store it could not read.
// The wrapped message is the plugin's own.
var ErrPrivacyFailed = errors.New("the plugin could not answer")

// ErrNoPrivacyAnswer is a plugin that declares the privacy hook and answered
// nothing at all — most likely it registered no handler. Nothing is not "no
// records": it is an answer nobody gave.
var ErrNoPrivacyAnswer = errors.New("the plugin declares the privacy hook and answered nothing")

// privacyOps are the operations Privacy passes on.
var privacyOps = map[string]bool{
	PrivacyFind: true, PrivacyErase: true, PrivacyCorrect: true, PrivacyExpire: true,
}

// Privacy asks one plugin one privacy question (Phase 33.2, D-09).
//
// Synchronous, unlike Emit, because the answer is the point: the operator is
// waiting for a person's records, an erasure or a retention run. And every
// error is returned rather than swallowed as the other hooks do, because a
// privacy answer that silently lost a plugin would look complete — the
// operator would hand a person an answer with their contact-form messages
// missing and never know.
//
// find runs inside ReadOnly: a plugin that tries to write while it is only
// asked to look is refused the write, and its answer is ErrWroteDuringFind
// rather than records. Each call is bounded by CallTimeout like every hook;
// expire answers in batches for that reason (PrivacyIn.Limit).
func (m *Manager) Privacy(ctx context.Context, websiteID int64, id string, in PrivacyIn) (*PrivacyOut, error) {
	if !m.Serves(id, HookPrivacy, websiteID) {
		return nil, fmt.Errorf("%w: %s", ErrNotServed, id)
	}
	if !privacyOps[in.Op] {
		return nil, fmt.Errorf("unknown privacy operation %q", in.Op)
	}
	var tried func() bool
	if in.Op == PrivacyFind {
		ctx, tried = ReadOnly(ctx)
	}
	// Read raw first: an empty answer and "{}" are different things here.
	var raw json.RawMessage
	if err := m.rt.Dispatch(ctx, id, HookPrivacy, websiteID, in, &raw); err != nil {
		return nil, err
	}
	if tried != nil && tried() {
		return nil, fmt.Errorf("%w: %s", ErrWroteDuringFind, id)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoPrivacyAnswer, id)
	}
	var out PrivacyOut
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("the privacy answer of %s is not valid JSON: %w", id, err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("%w: %s: %s", ErrPrivacyFailed, id, out.Error)
	}
	return &out, nil
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
		return fmt.Errorf("the plugin %q could not be loaded", id)
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
