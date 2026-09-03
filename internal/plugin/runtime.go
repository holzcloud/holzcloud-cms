package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// CallTimeout bounds one hook.
//
// A plugin runs inside a request. Without a deadline a module with an endless
// loop does not fail — it holds the connection, then the next one, and the site
// stops answering with nothing in the log to say why. Two seconds is far more
// than any hook needs and short enough that a visitor notices a slow page
// rather than a dead one.
const CallTimeout = 2 * time.Second

// MemoryPages bounds one instance's linear memory, in 64 KiB pages.
//
// 256 pages is 16 MB. A Go guest needs a few megabytes for its runtime before
// it does anything; this leaves room to work and still stops a plugin from
// eating a small node's memory on its own.
const MemoryPages = 256

// MaxPayloadBytes bounds what crosses the boundary in either direction.
const MaxPayloadBytes = 8 << 20

// Runtime compiles and runs the installed modules.
//
// One instance per plugin, kept alive: instantiating costs about seven
// milliseconds, which is too much to pay on every request. Calls into one
// plugin are serialised by its own lock — a wasm instance has one linear memory
// and no notion of two callers at once.
type Runtime struct {
	store  *Store
	log    *slog.Logger
	engine wazero.Runtime

	mu      sync.RWMutex
	loaded  map[string]*instance
	closed  bool
	hostMod api.Module

	// settings, pages and render are supplied by the caller; a nil one means
	// the matching operation is refused rather than silently answered with
	// nothing, so a missing wire-up is visible in the plugin's error line.
	settings SettingsFunc
	pages    PagesFunc
	render   RenderFunc
	notify   NotifyFunc
}

type instance struct {
	manifest *Manifest
	compiled wazero.CompiledModule

	// mu guards mod: one linear memory, one caller at a time.
	mu  sync.Mutex
	mod api.Module

	alloc  api.Function
	handle api.Function
}

// callCtx travels with a call so the host functions know who is asking and
// about which website. It is not on the Runtime because two requests can be
// inside two different plugins at the same moment.
type callCtx struct {
	pluginID  string
	manifest  *Manifest
	websiteID int64
}

type callCtxKey struct{}

// NewRuntime prepares the engine. Nothing is loaded yet.
func NewRuntime(ctx context.Context, store *Store, log *slog.Logger) (*Runtime, error) {
	if log == nil {
		log = slog.Default()
	}
	cfg := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(MemoryPages).
		// Without this a module that ignores its deadline keeps running after
		// the context is done, and the timeout above would be a suggestion.
		WithCloseOnContextDone(true)

	engine := wazero.NewRuntimeWithConfig(ctx, cfg)
	// Go's wasip1 target links against WASI even for a module that never
	// touches a file. The functions are present; what they can reach is
	// nothing, because the module config below grants no filesystem, no
	// environment and no clock beyond the wall time.
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, engine); err != nil {
		engine.Close(ctx)
		return nil, fmt.Errorf("wasi bereitstellen: %w", err)
	}

	r := &Runtime{store: store, log: log, engine: engine, loaded: map[string]*instance{}}

	host, err := engine.NewHostModuleBuilder(HostModule).
		NewFunctionBuilder().WithFunc(r.hostCall).Export(FnCall).
		Instantiate(ctx)
	if err != nil {
		engine.Close(ctx)
		return nil, fmt.Errorf("host-schnittstelle bereitstellen: %w", err)
	}
	r.hostMod = host
	return r, nil
}

// Close releases every instance and the engine.
func (r *Runtime) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	r.loaded = map[string]*instance{}
	return r.engine.Close(ctx)
}

// Load compiles a module and instantiates it.
//
// Compiling takes the better part of a second, so it happens here — at startup
// or when a plugin is switched on — and never in a request.
func (r *Runtime) Load(ctx context.Context, m *Manifest, module []byte) error {
	compiled, err := r.engine.CompileModule(ctx, module)
	if err != nil {
		return fmt.Errorf("das Modul lässt sich nicht übersetzen: %w", err)
	}

	// A guest that does not export what the convention requires is refused
	// here rather than at the first hook, where the failure would look like
	// the hook being broken.
	for _, name := range []string{GuestAlloc, GuestHandle} {
		if _, ok := compiled.ExportedFunctions()[name]; !ok {
			compiled.Close(ctx)
			return fmt.Errorf("das Modul exportiert %q nicht", name)
		}
	}

	cfg := wazero.NewModuleConfig().
		WithName("").
		// _initialize rather than _start: a reactor module sets itself up and
		// stays, where a command module would run main and exit.
		WithStartFunctions("_initialize").
		WithSysNanotime().
		WithSysWalltime()

	mod, err := r.engine.InstantiateModule(ctx, compiled, cfg)
	if err != nil {
		compiled.Close(ctx)
		return fmt.Errorf("das Modul lässt sich nicht starten: %w", err)
	}

	inst := &instance{
		manifest: m, compiled: compiled, mod: mod,
		alloc: mod.ExportedFunction(GuestAlloc), handle: mod.ExportedFunction(GuestHandle),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		mod.Close(ctx)
		compiled.Close(ctx)
		return errors.New("die Laufzeit ist beendet")
	}
	if old := r.loaded[m.ID]; old != nil {
		old.close(ctx)
	}
	r.loaded[m.ID] = inst
	return nil
}

// Unload stops a plugin and frees its memory.
func (r *Runtime) Unload(ctx context.Context, id string) {
	r.mu.Lock()
	inst := r.loaded[id]
	delete(r.loaded, id)
	r.mu.Unlock()
	if inst != nil {
		inst.close(ctx)
	}
}

func (i *instance) close(ctx context.Context) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.mod != nil {
		i.mod.Close(ctx)
		i.mod = nil
	}
	if i.compiled != nil {
		i.compiled.Close(ctx)
		i.compiled = nil
	}
}

// Loaded reports whether a plugin is currently running.
func (r *Runtime) Loaded(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded[id] != nil
}

// LoadedIDs returns the running plugins, for a status screen.
func (r *Runtime) LoadedIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.loaded))
	for id := range r.loaded {
		out = append(out, id)
	}
	return out
}

// Dispatch calls one hook on one plugin and unmarshals the answer into out.
//
// A plugin that traps, times out or answers with nonsense is recorded and
// treated as having said nothing. The alternative — letting it break the
// request — would mean one bad plugin takes the site down, which is exactly the
// thing a sandbox is for.
func (r *Runtime) Dispatch(ctx context.Context, id, hook string, websiteID int64, in, out any) error {
	r.mu.RLock()
	inst := r.loaded[id]
	r.mu.RUnlock()
	if inst == nil {
		return fmt.Errorf("%w: %s", ErrNotLoaded, id)
	}
	if !inst.manifest.Declares(hook) {
		return nil // not an error: the plugin simply does not want this hook
	}

	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	if len(payload) > MaxPayloadBytes {
		return fmt.Errorf("die Nutzlast für %q ist größer als %d MB", hook, MaxPayloadBytes>>20)
	}

	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ctx = context.WithValue(ctx, callCtxKey{},
		&callCtx{pluginID: id, manifest: inst.manifest, websiteID: websiteID})

	answer, err := inst.call(ctx, hook, payload)
	if err != nil {
		r.fail(id, hook, err)
		return err
	}
	if len(answer) == 0 || out == nil {
		return nil
	}
	if err := json.Unmarshal(answer, out); err != nil {
		err = fmt.Errorf("die Antwort auf %q ist kein gültiges JSON: %w", hook, err)
		r.fail(id, hook, err)
		return err
	}
	return nil
}

// ErrNotLoaded is returned when a plugin is not running.
var ErrNotLoaded = errors.New("plugin nicht geladen")

// fail records why a plugin misbehaved, in the log and on the plugin itself.
func (r *Runtime) fail(id, hook string, err error) {
	r.log.Error("plugin call failed", "plugin", id, "hook", hook, "err", err)
	// Best effort and deliberately not on the request's context: the request
	// may already be cancelled, and losing the reason would leave the operator
	// with a plugin that quietly does nothing.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.store.SetError(ctx, id, fmt.Sprintf("%s: %v", hook, err))
}

// call moves the bytes across and back.
func (i *instance) call(ctx context.Context, hook string, payload []byte) ([]byte, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.mod == nil {
		return nil, ErrNotLoaded
	}

	// A trap in the guest arrives as an error, but a panic in wazero's own
	// machinery would otherwise take the request with it.
	var res []byte
	var err error
	func() {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("das Modul ist abgestürzt: %v", p)
			}
		}()
		res, err = i.callLocked(ctx, hook, payload)
	}()
	return res, err
}

func (i *instance) callLocked(ctx context.Context, hook string, payload []byte) ([]byte, error) {
	// Hook and payload travel in one buffer, hook first.
	//
	// Two allocations would mean the guest has to find its own memory again
	// from an integer the host hands back, and turning an integer into a
	// pointer is exactly the operation Go does not allow. With one buffer the
	// guest keeps the slice it made and cuts it in two.
	joined := make([]byte, 0, len(hook)+len(payload))
	joined = append(joined, hook...)
	joined = append(joined, payload...)

	ptr, err := i.write(ctx, joined)
	if err != nil {
		return nil, err
	}

	ret, err := i.handle.Call(ctx, uint64(ptr), uint64(len(hook)), uint64(len(joined)))
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, nil
	}
	ptr, n := unpack(ret[0])
	if n == 0 {
		return nil, nil
	}
	if n > MaxPayloadBytes {
		return nil, fmt.Errorf("die Antwort ist größer als %d MB", MaxPayloadBytes>>20)
	}
	buf, ok := i.mod.Memory().Read(ptr, n)
	if !ok {
		return nil, errors.New("die Antwort liegt ausserhalb des Speichers des Moduls")
	}
	// Copied: the guest owns that memory and may reuse it on the next call.
	out := make([]byte, len(buf))
	copy(out, buf)
	return out, nil
}

// write asks the guest for room and puts data there.
func (i *instance) write(ctx context.Context, data []byte) (uint32, error) {
	if len(data) == 0 {
		return 0, nil
	}
	ret, err := i.alloc.Call(ctx, uint64(len(data)))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", GuestAlloc, err)
	}
	if len(ret) == 0 {
		return 0, fmt.Errorf("%s hat nichts zurückgegeben", GuestAlloc)
	}
	ptr := uint32(ret[0])
	if ptr == 0 {
		return 0, fmt.Errorf("%s hat keinen Speicher geliefert", GuestAlloc)
	}
	if !i.mod.Memory().Write(ptr, data) {
		return 0, errors.New("der gelieferte Speicher liegt ausserhalb des Moduls")
	}
	return ptr, nil
}

// hostCall is the one function the guest may call.
//
// Everything the host offers goes through here, dispatched by name. A single
// import means a plugin built today still links against a host that has since
// grown ten new operations.
func (r *Runtime) hostCall(ctx context.Context, m api.Module,
	opPtr, opLen, argPtr, argLen, outPtr, outCap uint32) uint64 {

	cc, _ := ctx.Value(callCtxKey{}).(*callCtx)
	if cc == nil {
		// Only reachable if a guest keeps a goroutine alive past its call.
		return r.answerError(m, outPtr, outCap, errors.New("Aufruf ausserhalb eines Hakens"))
	}

	op, ok := readString(m, opPtr, opLen)
	if !ok {
		return pack(StatusError, 0)
	}
	perm, known := opPermission[op]
	if !known {
		return r.answerError(m, outPtr, outCap, fmt.Errorf("unbekannte Operation %q", op))
	}
	if !cc.manifest.Allows(perm) {
		return r.answer(m, outPtr, outCap, StatusDenied,
			[]byte(fmt.Sprintf("das Plugin hat die Berechtigung %q nicht", perm)))
	}

	arg, ok := readBytes(m, argPtr, argLen)
	if !ok {
		return pack(StatusError, 0)
	}

	res, err := r.runOp(ctx, cc, op, arg)
	if err != nil {
		return r.answerError(m, outPtr, outCap, err)
	}
	return r.answer(m, outPtr, outCap, StatusOK, res)
}

// runOp performs one host operation.
func (r *Runtime) runOp(ctx context.Context, cc *callCtx, op string, arg []byte) ([]byte, error) {
	site := cc.websiteID

	switch op {
	case OpLog:
		var a LogArg
		if err := json.Unmarshal(arg, &a); err != nil {
			return nil, err
		}
		// The plugin's name is added by the host, not taken from the message:
		// a plugin cannot write a line that looks like it came from somewhere
		// else.
		level := slog.LevelInfo
		switch a.Level {
		case "debug":
			level = slog.LevelDebug
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
		r.log.Log(ctx, level, "plugin", "plugin", cc.pluginID, "message", truncate(a.Message, 2000))
		return nil, nil

	case OpStoreGet, OpStoreSet, OpStoreDelete, OpStoreList:
		var a StoreArg
		if err := json.Unmarshal(arg, &a); err != nil {
			return nil, err
		}
		if a.Global {
			site = 0
		}
		switch op {
		case OpStoreGet:
			v, found, err := r.store.StoreGet(ctx, cc.pluginID, site, a.Key)
			if err != nil {
				return nil, err
			}
			return json.Marshal(StoreResult{Value: v, Found: found})
		case OpStoreSet:
			return nil, r.store.StoreSet(ctx, cc.pluginID, site, a.Key, a.Value)
		case OpStoreDelete:
			return nil, r.store.StoreDelete(ctx, cc.pluginID, site, a.Key)
		default:
			m, err := r.store.StoreList(ctx, cc.pluginID, site, a.Prefix, a.Limit)
			if err != nil {
				return nil, err
			}
			return json.Marshal(m)
		}

	case OpSettings:
		if r.settings == nil {
			return nil, errors.New("die Einstellungen sind nicht verfügbar")
		}
		s, err := r.settings(ctx, site)
		if err != nil {
			return nil, err
		}
		return json.Marshal(s)

	case OpPagesList, OpPagesGet, OpPagesSearch:
		if r.pages == nil {
			return nil, errors.New("die Seiten sind nicht verfügbar")
		}
		var a PagesArg
		if err := json.Unmarshal(arg, &a); err != nil {
			return nil, err
		}
		// Clamped rather than rejected: a plugin asking for a thousand pages is
		// not misbehaving, it just does not know the limit, and an error would
		// turn a working feature into a broken one over a number.
		if a.Limit <= 0 || a.Limit > MaxPagesLimit {
			a.Limit = MaxPagesLimit
		}
		if a.Offset < 0 {
			a.Offset = 0
		}
		res, err := r.pages(ctx, site, PagesQuery{
			Op: op, Slug: a.Slug, Query: a.Query,
			Limit: a.Limit, Offset: a.Offset, PostsOnly: a.PostsOnly,
			WithFields: a.WithFields,
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)

	case OpNotify:
		if r.notify == nil {
			return json.Marshal(NotifyResult{Reason: "es ist kein Mailserver eingerichtet"})
		}
		var a NotifyArg
		if err := json.Unmarshal(arg, &a); err != nil {
			return nil, err
		}
		if len(a.Subject)+len(a.Body) > MaxNotifyBytes {
			return nil, fmt.Errorf("die Benachrichtigung ist größer als %d KB", MaxNotifyBytes>>10)
		}
		queued, reason, err := r.notify(ctx, site, a)
		if err != nil {
			return nil, err
		}
		return json.Marshal(NotifyResult{Queued: queued, Reason: reason})

	case OpRender:
		if r.render == nil {
			return nil, errors.New("das Ausgeben von Seiten ist nicht verfügbar")
		}
		var a RenderArg
		if err := json.Unmarshal(arg, &a); err != nil {
			return nil, err
		}
		switch a.View {
		case "", ViewPage:
			a.View = ViewPage
		case ViewSearch:
		default:
			return nil, fmt.Errorf("die Ansicht %q gibt es nicht", a.View)
		}
		out, err := r.render(ctx, site, a)
		if err != nil {
			return nil, err
		}
		return json.Marshal(RenderResult{HTML: out})
	}
	return nil, fmt.Errorf("unbekannte Operation %q", op)
}

// SettingsFunc reads the parts of a website a plugin may see.
//
// Injected rather than imported: internal/plugin must not depend on the domain
// package, or the two could not be tested apart and a plugin would be able to
// reach whatever the domain store grows next.
type SettingsFunc func(ctx context.Context, websiteID int64) (SettingsResult, error)

// PagesQuery is one page request from a plugin, already checked and clamped.
type PagesQuery struct {
	// Op is which of the three operations was asked for.
	Op         string
	Slug       string
	Query      string
	Limit      int
	Offset     int
	PostsOnly  bool
	WithFields bool
}

// PagesFunc reads published pages. Injected for the same reason as the
// settings reader, and with the same rule: published only, never a draft.
type PagesFunc func(ctx context.Context, websiteID int64, q PagesQuery) (PagesResult, error)

// RenderFunc draws a page in the website's theme and returns the finished HTML.
type RenderFunc func(ctx context.Context, websiteID int64, a RenderArg) (string, error)

// NotifyFunc queues one message to the operator of a website.
//
// It returns whether anything was queued and, if not, why — no mail server, or
// no notification address on this site. Neither is a failure: both are ordinary
// states of an installation that has not asked for mail.
type NotifyFunc func(ctx context.Context, websiteID int64, a NotifyArg) (queued bool, reason string, err error)

// WithSettings supplies the reader for the settings operation.
func (r *Runtime) WithSettings(f SettingsFunc) { r.settings = f }

// WithPages supplies the reader for the page operations.
func (r *Runtime) WithPages(f PagesFunc) { r.pages = f }

// WithRender supplies the theme renderer.
func (r *Runtime) WithRender(f RenderFunc) { r.render = f }

// WithNotify supplies the notifier.
func (r *Runtime) WithNotify(f NotifyFunc) { r.notify = f }

// answer writes a result into the guest's buffer, or says how much room it
// needs. Nothing is written in the short case: a partial answer that the guest
// then parses is worse than no answer.
func (r *Runtime) answer(m api.Module, outPtr, outCap uint32, status int, data []byte) uint64 {
	if uint32(len(data)) > outCap {
		return pack(StatusShort, uint32(len(data)))
	}
	if len(data) > 0 && !m.Memory().Write(outPtr, data) {
		return pack(StatusError, 0)
	}
	return pack(uint32(status), uint32(len(data)))
}

func (r *Runtime) answerError(m api.Module, outPtr, outCap uint32, err error) uint64 {
	return r.answer(m, outPtr, outCap, StatusError, []byte(truncate(err.Error(), 1000)))
}

func readBytes(m api.Module, ptr, n uint32) ([]byte, bool) {
	if n == 0 {
		return nil, true
	}
	if n > MaxPayloadBytes {
		return nil, false
	}
	return m.Memory().Read(ptr, n)
}

func readString(m api.Module, ptr, n uint32) (string, bool) {
	b, ok := readBytes(m, ptr, n)
	return string(b), ok
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
