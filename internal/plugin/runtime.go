package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// CallTimeout bounds one hook.
//
// A plugin runs inside a request. Without a deadline a module with an endless
// loop does not fail — it holds the connection, then the next one, and the site
// stops answering with nothing in the log to say why. Two seconds is far more
// than any hook needs and short enough that a visitor notices a slow page
// rather than a dead one.
const CallTimeout = 2 * time.Second

// QueueWait is how long a call waits for a busy plugin before it is answered
// with ErrBusy. Two seconds is the length of one call: a caller that would
// wait longer than a whole call would be waiting behind a plugin that is not
// going to get to it. A starting value, tunable by SetLimits.
const QueueWait = 2 * time.Second

// QueueLength is how many callers may wait for one plugin at the same time.
// A caller who would be the thirty-third is answered with ErrBusy at once
// rather than held as a connection and a goroutine. A starting value.
const QueueLength = 32

// BreakerFailures is how many calls in a row may fail before a plugin is held
// back. A starting value.
const BreakerFailures = 3

// BreakerCooldown is how long a plugin that failed BreakerFailures calls in a
// row is held back before one call is tried again. Fixed, not growing: a
// plugin that stays broken then costs one bounded call per cooldown. A
// starting value.
const BreakerCooldown = 30 * time.Second

// reviveTimeout bounds bringing a closed module back from its compiled module.
// Far above the seven milliseconds an instantiation takes, and the same kind of
// bound as LoadTimeout: a start function that never returns must not hold a
// call for ever.
const reviveTimeout = 10 * time.Second

// Limits are the bounds of a plugin call. A zero field keeps its default.
type Limits struct {
	// CallTimeout bounds one hook once the call holds the plugin.
	CallTimeout time.Duration
	// QueueWait is how long a call waits for the plugin before ErrBusy.
	QueueWait time.Duration
	// QueueLength is how many callers may wait for one plugin.
	QueueLength int
	// BreakerFailures is the number of failed calls in a row that holds a
	// plugin back.
	BreakerFailures int
	// BreakerCooldown is how long a held-back plugin is left alone.
	BreakerCooldown time.Duration
}

// SetLimits replaces the bounds of a call. It is meant for tests, which need
// short ones, and must be called before the first Load: the values are read on
// every call without a lock. They are fields of the Runtime and not package
// variables so that one test's bounds cannot reach another test's runtime
// under go test -race.
func (r *Runtime) SetLimits(l Limits) {
	if l.CallTimeout > 0 {
		r.callTimeout = l.CallTimeout
	}
	if l.QueueWait > 0 {
		r.queueWait = l.QueueWait
	}
	if l.QueueLength > 0 {
		r.queueLength = l.QueueLength
	}
	if l.BreakerFailures > 0 {
		r.breakerFailures = l.BreakerFailures
	}
	if l.BreakerCooldown > 0 {
		r.breakerCooldown = l.BreakerCooldown
	}
}

// LoadTimeout is the default bound of every Runtime on compiling a module and
// running its _initialize; the bound itself lives on the Runtime
// (loadTimeout). It is far above what a shipped plugin needs (well under a
// second) and exists so that a module whose start never returns cannot hold a
// load for ever.
const LoadTimeout = 60 * time.Second

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

	// loadTimeout is the bound Load applies. It is LoadTimeout unless a test
	// in this package shortens it before the first Load. It is a field rather
	// than a package variable so that one test's short bound cannot race
	// another test's runtime under go test -race.
	loadTimeout time.Duration

	// The bounds of a call, see Limits. Fields for the same reason as
	// loadTimeout.
	callTimeout     time.Duration
	queueWait       time.Duration
	queueLength     int
	breakerFailures int
	breakerCooldown time.Duration
	reviveTimeout   time.Duration
	// now is the clock of the breaker; a test moves it past a cooldown.
	now func() time.Time

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

	// lock holds the instance: one linear memory, one caller at a time. A
	// one-slot channel and not a sync.Mutex, because a caller has to be able to
	// give up its place in the line: when its context ends, when the wait is
	// longer than the queue wait, and never at all when the line is full. A
	// mutex can only be waited for.
	//
	// Everything below it is read and written only while it is held.
	lock    chan struct{}
	waiting atomic.Int32
	// closed marks an instance that Load or Unload replaced. It is set before
	// close waits for the lock, so that a caller already in the line does not
	// get the instance first and bring its compiled module back to life.
	closed atomic.Bool

	mod    api.Module
	alloc  api.Function
	handle api.Function
}

func newInstance(m *Manifest, compiled wazero.CompiledModule) *instance {
	return &instance{manifest: m, compiled: compiled, lock: make(chan struct{}, 1)}
}

// callCtx travels with a call so the host functions know who is asking and
// about which website. It is not on the Runtime because two requests can be
// inside two different plugins at the same moment.
type callCtx struct {
	pluginID  string
	manifest  *Manifest
	websiteID int64
	// files are the attachments the host is holding for this request, and the
	// only piece of callCtx that is not just identity.
	//
	// Held here rather than on the Runtime because two requests can be inside
	// two different plugins at the same moment — and because the moment the
	// call returns, whatever was not kept is gone, which is exactly the
	// lifetime a caller should have to think about.
	files FileKeeper
}

// FileKeeper stores the attachments of one request.
//
// An interface so this package keeps knowing nothing about the media store or
// the disk, the same way TemplateResolver keeps internal/template out of the
// database. Nil means no file came in, or the plugin has no business with one.
type FileKeeper interface {
	// List is what arrived, without the bytes.
	List() []AttachedFile
	// Keep writes them and answers with their ids. Called at most once: a
	// second call answers with the same ids and stores nothing further.
	Keep(ctx context.Context) ([]KeptFile, error)
}

type callCtxKey struct{}

// readOnlyCall marks a call during which the host refuses the plugin's
// writes, and remembers whether the plugin tried one.
type readOnlyCall struct{ refused atomic.Bool }

type readOnlyCallKey struct{}

// ReadOnly returns a context for a call during which the plugin may read its
// storage but not change it: store.set, store.delete and files.keep answer
// StatusDenied. The function it returns reports afterwards whether the plugin
// tried such a write.
//
// It is how the host keeps the promise of a plugin that did not declare
// respects_read_only (D-13) on a GET as well: the host refuses that plugin's
// POSTs for somebody without the publishing right before calling it, but a
// GET reaches the plugin — reading stays open — and a simple screen's
// "delete" link is a GET.
func ReadOnly(ctx context.Context) (context.Context, func() bool) {
	rc := &readOnlyCall{}
	return context.WithValue(ctx, readOnlyCallKey{}, rc), rc.refused.Load
}

// writeOps are the operations a read-only call refuses.
var writeOps = map[string]bool{OpStoreSet: true, OpStoreDelete: true, OpKeepFiles: true}

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

	r := &Runtime{
		store: store, log: log, engine: engine, loaded: map[string]*instance{},
		loadTimeout: LoadTimeout,
		callTimeout: CallTimeout, queueWait: QueueWait, queueLength: QueueLength,
		breakerFailures: BreakerFailures, breakerCooldown: BreakerCooldown,
		reviveTimeout: reviveTimeout, now: time.Now,
	}

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
	// A load is not the caller's to cancel. Switching a plugin on runs inside
	// the admin's request, and a browser that drops it (a navigation, a closed
	// tab) cancels that context while the module compiles or runs _initialize;
	// with WithCloseOnContextDone the engine then closes the module mid-start,
	// and the plugin stays switched on but not running. The load keeps the
	// caller's values, drops its cancellation and gets a bound of its own.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.loadTimeout)
	defer cancel()

	compiled, err := r.engine.CompileModule(ctx, module)
	if err != nil {
		return fmt.Errorf("the module cannot be compiled: %w", err)
	}

	// A guest that does not export what the convention requires is refused
	// here rather than at the first hook, where the failure would look like
	// the hook being broken.
	for _, name := range []string{GuestAlloc, GuestHandle} {
		if _, ok := compiled.ExportedFunctions()[name]; !ok {
			compiled.Close(ctx)
			return fmt.Errorf("the module does not export %q", name)
		}
	}

	mod, err := r.instantiate(ctx, compiled)
	if err != nil {
		compiled.Close(ctx)
		return fmt.Errorf("the module cannot be started: %w", err)
	}

	inst := newInstance(m, compiled)
	inst.setModule(mod)

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		mod.Close(ctx)
		compiled.Close(ctx)
		return errors.New("die Laufzeit ist beendet")
	}
	old := r.loaded[m.ID]
	r.loaded[m.ID] = inst
	r.mu.Unlock()

	// The old instance is closed after the map is released: closing waits for
	// the call that is running in it, and a Dispatch that wants the map in the
	// meantime would wait for that call too, whichever plugin it was for.
	if old != nil {
		old.close(ctx)
	}
	return nil
}

// instantiate starts a compiled module. A module config is used once, so every
// instantiation, the first and a revival alike, builds a fresh one.
func (r *Runtime) instantiate(ctx context.Context, compiled wazero.CompiledModule) (api.Module, error) {
	cfg := wazero.NewModuleConfig().
		WithName("").
		// _initialize rather than _start: a reactor module sets itself up and
		// stays, where a command module would run main and exit.
		WithStartFunctions("_initialize").
		WithSysNanotime().
		WithSysWalltime()
	return r.engine.InstantiateModule(ctx, compiled, cfg)
}

// setModule makes a freshly started module the instance's own.
func (i *instance) setModule(mod api.Module) {
	i.mod = mod
	i.alloc = mod.ExportedFunction(GuestAlloc)
	i.handle = mod.ExportedFunction(GuestHandle)
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

// close retires the instance. The flag is set before the lock is waited for:
// callers already in the line then find the instance closed when their turn
// comes, instead of getting it first and starting its module again. The wait
// itself is not subject to the queue (a replaced plugin must be released however
// long the line is) and is for the one call that is running.
func (i *instance) close(ctx context.Context) {
	i.closed.Store(true)
	i.lock <- struct{}{}
	defer i.release()
	i.closeModule()
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
		return fmt.Errorf("the payload for %q is larger than %d MB", hook, MaxPayloadBytes>>20)
	}

	// The call is the plugin's own and not the visitor's. A request that is
	// dropped, before the call or while it runs, would otherwise take the wasm
	// module down with it (the engine closes a module whose context is done),
	// and one visitor who closed a tab would switch the plugin off for every
	// website. The call keeps what the request carries, the language, the
	// read-only marker, the request and the attachments, and none of its
	// cancellation; its bound starts once it holds the plugin, see call.
	base := context.WithValue(context.WithoutCancel(ctx), callCtxKey{}, &callCtx{
		pluginID: id, manifest: inst.manifest, websiteID: websiteID,
		files: keeperFrom(ctx),
	})

	answer, err := r.call(ctx, inst, base, hook, payload)
	if err != nil {
		if failure(err) {
			r.fail(id, hook, err)
		}
		return err
	}
	if len(answer) == 0 || out == nil {
		return nil
	}
	if err := json.Unmarshal(answer, out); err != nil {
		err = fmt.Errorf("the answer to %q is not valid JSON: %w", hook, err)
		r.fail(id, hook, err)
		return err
	}
	return nil
}

// ErrNotLoaded is returned when a plugin is not running.
var ErrNotLoaded = errors.New("plugin nicht geladen")

// ErrBusy is the reason of an UnavailableError for a plugin that has no room
// for one more caller: the line behind it is full, or the caller waited as long
// as it may.
var ErrBusy = errors.New("the plugin is busy")

// ErrHeldBack is the reason of an UnavailableError for a plugin that failed
// call after call and is being left alone for a while.
var ErrHeldBack = errors.New("the plugin is held back after repeated failures")

// UnavailableError says that a plugin was not asked at all, and when it is
// worth asking again. It is an answer about the moment and not a failure of the
// plugin: nothing is recorded on the plugin for it.
//
// errors.Is(err, ErrBusy) and errors.Is(err, ErrHeldBack) tell the two apart;
// errors.As gives RetryAfter, which a route turns into a Retry-After header.
type UnavailableError struct {
	Reason     error
	RetryAfter time.Duration
}

func (e *UnavailableError) Error() string { return e.Reason.Error() }
func (e *UnavailableError) Unwrap() error { return e.Reason }

// leftError is a caller who stopped waiting for the plugin. It is the caller's
// own context that ended, and so it says nothing about the plugin.
type leftError struct{ err error }

func (e *leftError) Error() string { return e.err.Error() }
func (e *leftError) Unwrap() error { return e.err }

// failure reports whether err says something about the plugin. Busy and held
// back answers, a caller who left the line and a plugin that has been replaced
// do not: recording them would send the operator after a fault that is not
// there.
func failure(err error) bool {
	var u *UnavailableError
	var l *leftError
	return !errors.As(err, &u) && !errors.As(err, &l) && !errors.Is(err, ErrNotLoaded)
}

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
//
// The bound of the call starts once it holds the instance. A caller that waited
// behind another has its whole bound left to run in: starting the clock before
// the lock would hand a slow call's time to the one queued behind it.
func (r *Runtime) call(ctx context.Context, i *instance, base context.Context, hook string, payload []byte) ([]byte, error) {
	if err := r.acquire(ctx, i); err != nil {
		return nil, err
	}
	defer i.release()
	// An instance that Load or Unload replaced is gone, whatever the caller
	// queued for. Its compiled module is not brought back for a caller that
	// holds a reference nothing else holds.
	if i.closed.Load() || r.isClosed() {
		return nil, ErrNotLoaded
	}

	// A module that a timeout, a trap or the engine closed is brought back
	// from the compiled module under the lock, so that no two calls revive it.
	if i.mod == nil || i.mod.IsClosed() {
		if err := r.revive(i); err != nil {
			return nil, err
		}
	}

	cctx, cancel := context.WithTimeout(base, r.callTimeout)
	defer cancel()

	// A trap in the guest arrives as an error, but a panic in wazero's own
	// machinery would otherwise take the request with it.
	var res []byte
	var err error
	func() {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("the module crashed: %v", p)
			}
		}()
		res, err = i.callLocked(cctx, hook, payload)
	}()
	if err != nil {
		// A guest that trapped or was cut off is in an unknown state: a Go
		// runtime halfway through a goroutine switch cannot be resumed. Its
		// module is closed here, if the engine has not done so, and the next
		// call starts a fresh one.
		i.closeModule()
	}
	return res, err
}

// acquire takes the instance for one call, or says why the call cannot have it.
//
// A free instance is taken without a look at the caller's context: the call is
// detached from the request by design, and a submission that reached the plugin
// should complete even if its visitor has gone. Only waiting depends on the
// caller. The line is bounded in length, so that a slow plugin is not a place
// where every request of a busy site piles up as a goroutine and a connection,
// and in time, so that a caller is not held longer than a call takes.
func (r *Runtime) acquire(ctx context.Context, i *instance) error {
	select {
	case i.lock <- struct{}{}:
		return nil
	default:
	}

	busy := &UnavailableError{Reason: ErrBusy, RetryAfter: r.queueWait}
	if int(i.waiting.Add(1)) > r.queueLength {
		i.waiting.Add(-1)
		return busy
	}
	defer i.waiting.Add(-1)

	timer := time.NewTimer(r.queueWait)
	defer timer.Stop()
	select {
	case i.lock <- struct{}{}:
		return nil
	case <-ctx.Done():
		return &leftError{err: ctx.Err()}
	case <-timer.C:
		return busy
	}
}

func (i *instance) release() { <-i.lock }

// revive starts the instance's module again. The caller holds the instance.
func (r *Runtime) revive(i *instance) error {
	i.closeModule()
	if i.compiled == nil {
		return ErrNotLoaded
	}
	// A context of its own: the call has not begun, so there is no call
	// context yet, and a fresh Load does not run under one either.
	ctx, cancel := context.WithTimeout(context.Background(), r.reviveTimeout)
	defer cancel()
	mod, err := r.instantiate(ctx, i.compiled)
	if err != nil {
		return fmt.Errorf("the module cannot be started again: %w", err)
	}
	i.setModule(mod)
	return nil
}

// closeModule releases the instance's module and its memory. The caller holds
// the instance.
func (i *instance) closeModule() {
	if i.mod != nil {
		i.mod.Close(context.Background())
		i.mod = nil
		i.alloc, i.handle = nil, nil
	}
}

func (r *Runtime) isClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.closed
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
		return nil, fmt.Errorf("the answer is larger than %d MB", MaxPayloadBytes>>20)
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
		return 0, fmt.Errorf("%s returned nothing", GuestAlloc)
	}
	ptr := uint32(ret[0])
	if ptr == 0 {
		return 0, fmt.Errorf("%s hat keinen Speicher geliefert", GuestAlloc)
	}
	if !i.mod.Memory().Write(ptr, data) {
		return 0, errors.New("the memory handed over lies outside the module")
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
	if perm != PermNone && !cc.manifest.Allows(perm) {
		return r.answer(m, outPtr, outCap, StatusDenied,
			[]byte(fmt.Sprintf("the plugin does not have the permission %q", perm)))
	}
	if rc, _ := ctx.Value(readOnlyCallKey{}).(*readOnlyCall); rc != nil && writeOps[op] {
		rc.refused.Store(true)
		return r.answer(m, outPtr, outCap, StatusDenied,
			[]byte("read only: the person on this screen may not change what the public sees"))
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
			return nil, errors.New("the settings are not available")
		}
		s, err := r.settings(ctx, site)
		if err != nil {
			return nil, err
		}
		return json.Marshal(s)

	case OpPagesList, OpPagesGet, OpPagesSearch:
		if r.pages == nil {
			return nil, errors.New("the pages are not available")
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

	case OpKeepFiles:
		// Nothing was on the disk until this line. A plugin that refuses a
		// submission simply never gets here, and the files go away with the
		// request.
		if cc.files == nil {
			return json.Marshal(KeepFilesResult{})
		}
		kept, err := cc.files.Keep(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(KeepFilesResult{Kept: kept})

	case OpTranslate:
		// The language comes out of the request's context and not out of the
		// call: a plugin must not be able to choose which language an operator
		// is answered in, and the host already knows.
		var a TranslateArg
		if err := json.Unmarshal(arg, &a); err != nil {
			return nil, err
		}
		return json.Marshal(TranslateResult{Text: i18n.T(i18n.Lang(ctx), a.Text)})

	case OpNotify:
		if r.notify == nil {
			return json.Marshal(NotifyResult{Reason: "no mail server is set up"})
		}
		var a NotifyArg
		if err := json.Unmarshal(arg, &a); err != nil {
			return nil, err
		}
		if len(a.Subject)+len(a.Body) > MaxNotifyBytes {
			return nil, fmt.Errorf("the notification is larger than %d KB", MaxNotifyBytes>>10)
		}
		// The copy to the sender is asked for in the same call and granted by
		// a SECOND permission, so a plugin with "notify" alone cannot reach a
		// third party. cc.manifest is the plugin's own declaration, checked at
		// install; the operator's switch is checked by the host function.
		a.Confirm = a.Confirm && cc.manifest.Allows(PermConfirm)
		queued, confirmed, reason, err := r.notify(ctx, site, a)
		if err != nil {
			return nil, err
		}
		return json.Marshal(NotifyResult{Queued: queued, Reason: reason, Confirmed: confirmed})

	case OpRender:
		if r.render == nil {
			return nil, errors.New("rendering pages is not available")
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
			return nil, fmt.Errorf("there is no view %q", a.View)
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
// NotifyFunc sends the operator's notification and, when asked and allowed,
// one copy to the sender. It reports both separately: a plugin has to be able
// to tell "it arrived and they were told" from "it arrived".
type NotifyFunc func(ctx context.Context, websiteID int64, a NotifyArg) (queued, confirmed bool, reason string, err error)

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

// keeperFromKey carries the attachments of one request from the caller that
// parsed them down to the host call that may keep them.
type keeperFromKey struct{}

// WithFiles gives the attachments of a request to whatever plugin call follows.
//
// The caller is the HTTP layer, which is the only place that has a multipart
// form; this package is the only place that knows a plugin asked for them.
// Passing them through the context rather than through Dispatch's signature
// keeps every other hook — content, admin, event — unchanged.
func WithFiles(ctx context.Context, k FileKeeper) context.Context {
	return context.WithValue(ctx, keeperFromKey{}, k)
}

func keeperFrom(ctx context.Context) FileKeeper {
	k, _ := ctx.Value(keeperFromKey{}).(FileKeeper)
	return k
}
