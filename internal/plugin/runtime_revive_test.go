package plugin

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// reviveRuntime is neueLaufzeit with short bounds: the limits are set before
// the first Load, as SetLimits requires, and the echo module is loaded after.
func reviveRuntime(t *testing.T, l Limits) (*Runtime, *Store) {
	t.Helper()
	return reviveRuntimeWith(t, l, echoModule(t))
}

// reviveRuntimeWith loads another module than the echo guest under the id
// "echo", so that the helpers below keep working on it.
func reviveRuntimeWith(t *testing.T, l Limits, module []byte) (*Runtime, *Store) {
	t.Helper()
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	r, err := NewRuntime(ctx, s, nil)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	r.SetLimits(l)

	m := goodManifest()
	m.ID = "echo"
	m.Name = "Echo"
	m.Hooks = []string{HookContent, HookEvent, HookRequest, HookAdmin}
	m.Permissions = []string{PermStore}
	p := &Package{Manifest: &m, Module: module, SHA256: strings.Repeat("a", 64)}
	if err := s.Install(ctx, p); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := r.Load(ctx, &m, p.Module); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return r, s
}

// spin is the call that never returns: the echo guest loops for ever.
func spin(ctx context.Context, r *Runtime) error {
	return r.Dispatch(ctx, "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "endlos"}}, nil)
}

// content is an ordinary call that has to succeed.
func content(ctx context.Context, r *Runtime) error {
	var out ContentOut
	err := r.Dispatch(ctx, "echo", HookContent, 1,
		ContentIn{WebsiteID: 1, Slug: "home", HTML: "<p>Hallo</p>"}, &out)
	if err == nil && !out.Changed {
		return errors.New("the plugin answered but did not run")
	}
	return err
}

func lastError(t *testing.T, s *Store) string {
	t.Helper()
	p, err := s.Get(context.Background(), "echo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return p.LastError
}

// A visitor who is gone when the plugin is called, or goes while it runs, must
// not switch the plugin off for everybody else. The call is detached from the
// request: it is the plugin's own, with a bound of its own.
func TestACancelledCallerDoesNotCloseThePlugin(t *testing.T) {
	r, s := reviveRuntime(t, Limits{})

	gone, cancel := context.WithCancel(context.Background())
	cancel()
	if err := content(gone, r); err != nil {
		t.Fatalf("a call whose caller is already gone: %v", err)
	}
	if err := content(context.Background(), r); err != nil {
		t.Fatalf("the next call, after a caller left: %v", err)
	}
	if e := lastError(t, s); e != "" {
		t.Errorf("a caller who left was recorded as the plugin's failure: %q", e)
	}
}

func TestACancelMidCallDoesNotCutTheCall(t *testing.T) {
	r, _ := reviveRuntime(t, Limits{CallTimeout: 300 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	err := spin(ctx, r)
	took := time.Since(start)
	if err == nil {
		t.Fatal("the endless loop ran through")
	}
	if took < 250*time.Millisecond {
		t.Errorf("the caller's cancel cut the call after %v, its own bound is 300 ms", took)
	}
	if err := content(context.Background(), r); err != nil {
		t.Fatalf("the plugin does not answer after the cut call: %v", err)
	}
}

// A call that runs into its bound closes the module. The next call brings it
// back from the compiled module, which stays valid after its instances close.
func TestAPluginComesBackAfterATimeout(t *testing.T) {
	r, s := reviveRuntime(t, Limits{CallTimeout: 100 * time.Millisecond})

	err := spin(context.Background(), r)
	if err == nil {
		t.Fatal("the endless loop ran through")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("expected a deadline error, got: %v", err)
	}
	if e := lastError(t, s); !strings.Contains(e, "deadline") {
		t.Errorf("the timeout was not recorded on the plugin: %q", e)
	}
	if err := content(context.Background(), r); err != nil {
		t.Fatalf("the plugin did not come back after a timeout: %v", err)
	}
}

// instanceOf is the running instance of the echo plugin, for a test that has to
// look at the lock.
func instanceOf(r *Runtime) *instance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded["echo"]
}

// waitFor polls until cond holds. The tests below need a call to hold the
// instance, or to wait for it, before the next one is made; sleeping a guessed
// time would make them depend on the machine.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("gave up waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func holds(r *Runtime) func() bool {
	return func() bool { return len(instanceOf(r).lock) == 1 }
}

func waiters(r *Runtime, n int32) func() bool {
	return func() bool { return instanceOf(r).waiting.Load() == n }
}

// async runs f in a goroutine and returns what it will have returned.
func async(f func() error) func() error {
	var wg sync.WaitGroup
	var err error
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = f()
	}()
	return func() error {
		wg.Wait()
		return err
	}
}

// The bound of a call is its own once it holds the plugin. A call that queued
// behind a slow one is not charged the time it waited.
func TestTheBoundStartsAfterTheLock(t *testing.T) {
	r, s := reviveRuntime(t, Limits{CallTimeout: 300 * time.Millisecond})

	first := async(func() error { return spin(context.Background(), r) })
	waitFor(t, "the first call to hold the plugin", holds(r))

	start := time.Now()
	if err := content(context.Background(), r); err != nil {
		t.Fatalf("a call that waited for the plugin: %v", err)
	}
	if took := time.Since(start); took < 100*time.Millisecond {
		t.Errorf("the second call did not wait for the first: %v", took)
	}
	if first() == nil {
		t.Error("the endless loop ran through")
	}
	if e := lastError(t, s); strings.Contains(e, "content") {
		t.Errorf("the waiting call was recorded as a failure: %q", e)
	}
}

// With a line of one, the third caller does not stand behind the second: it is
// told so at once, and what it is told is not held against the plugin.
func TestAFullQueueAnswersAtOnce(t *testing.T) {
	r, s := reviveRuntime(t, Limits{CallTimeout: 500 * time.Millisecond, QueueWait: 2 * time.Second, QueueLength: 1})

	first := async(func() error { return spin(context.Background(), r) })
	waitFor(t, "the first call to hold the plugin", holds(r))
	second := async(func() error { return content(context.Background(), r) })
	waitFor(t, "one caller in the line", waiters(r, 1))

	start := time.Now()
	err := content(context.Background(), r)
	took := time.Since(start)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got: %v", err)
	}
	var u *UnavailableError
	if !errors.As(err, &u) || u.RetryAfter <= 0 {
		t.Errorf("a busy answer says nothing about when to ask again: %v", err)
	}
	if took > 50*time.Millisecond {
		t.Errorf("the busy answer took %v", took)
	}

	if first() == nil {
		t.Error("the endless loop ran through")
	}
	if err := second(); err != nil {
		t.Errorf("the caller in the line was not served: %v", err)
	}
	e := lastError(t, s)
	if !strings.HasPrefix(e, "event:") || strings.Contains(e, "busy") {
		t.Errorf("LastError should carry the timeout of the endless loop only: %q", e)
	}
}

func TestAQueueWaitEndsInBusy(t *testing.T) {
	r, _ := reviveRuntime(t, Limits{CallTimeout: 500 * time.Millisecond, QueueWait: 50 * time.Millisecond})

	first := async(func() error { return spin(context.Background(), r) })
	waitFor(t, "the first call to hold the plugin", holds(r))

	start := time.Now()
	err := content(context.Background(), r)
	took := time.Since(start)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got: %v", err)
	}
	if took < 40*time.Millisecond || took > 300*time.Millisecond {
		t.Errorf("busy after %v, the wait is 50 ms", took)
	}
	first()
}

// A caller who stops waiting leaves the line and nothing else. It is not the
// plugin's failure, and it does not close the module of the call in front of it.
func TestAQueuedCallerWhoLeavesIsNotRecorded(t *testing.T) {
	r, s := reviveRuntime(t, Limits{CallTimeout: 500 * time.Millisecond})

	start := time.Now()
	first := async(func() error { return spin(context.Background(), r) })
	waitFor(t, "the first call to hold the plugin", holds(r))

	ctx, cancel := context.WithCancel(context.Background())
	second := async(func() error { return content(ctx, r) })
	waitFor(t, "one caller in the line", waiters(r, 1))
	time.AfterFunc(20*time.Millisecond, cancel)

	left := time.Now()
	if err := second(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if took := time.Since(left); took > 300*time.Millisecond {
		t.Errorf("the caller was kept in the line for %v after it left", took)
	}
	if n := instanceOf(r).waiting.Load(); n != 0 {
		t.Errorf("%d callers still counted in the line", n)
	}

	if first() == nil {
		t.Error("the endless loop ran through")
	}
	if took := time.Since(start); took < 450*time.Millisecond {
		t.Errorf("the call in front was cut after %v by a caller who left", took)
	}
	if e := lastError(t, s); !strings.HasPrefix(e, "event:") {
		t.Errorf("a caller who left was recorded: %q", e)
	}
}

// A call queued on an instance that Unload (or Load) replaced must not bring
// the old module back. It would run, once, on an instance nothing refers to and
// nothing will ever close again.
//
// Unload rather than Load: both swap the entry and close the old instance by
// the same path, and Load compiles for the better part of a second first, which
// would make the order of events depend on the machine.
func TestAReplacedPluginIsNotRevived(t *testing.T) {
	r, s := reviveRuntime(t, Limits{CallTimeout: 600 * time.Millisecond})
	old := instanceOf(r)

	first := async(func() error { return spin(context.Background(), r) })
	waitFor(t, "the first call to hold the plugin", holds(r))
	second := async(func() error { return content(context.Background(), r) })
	waitFor(t, "one caller in the line", waiters(r, 1))

	r.Unload(context.Background(), "echo")
	if r.Loaded("echo") {
		t.Fatal("still loaded after Unload")
	}

	if first() == nil {
		t.Error("the endless loop ran through")
	}
	if err := second(); !errors.Is(err, ErrNotLoaded) {
		t.Errorf("a call queued on a replaced instance should find it gone, got: %v", err)
	}
	if old.mod != nil || old.compiled != nil {
		t.Error("the old instance was brought back or not released")
	}
	if e := lastError(t, s); strings.HasPrefix(e, "content:") {
		t.Errorf("the call on the replaced instance was recorded: %q", e)
	}

	// The replacement is served by a new instance.
	m := goodManifest()
	m.ID = "echo"
	m.Name = "Echo"
	m.Hooks = []string{HookContent, HookEvent, HookRequest, HookAdmin}
	m.Permissions = []string{PermStore}
	if err := r.Load(context.Background(), &m, echoModule(t)); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if instanceOf(r) == old {
		t.Fatal("Load did not make a new instance")
	}
	if err := content(context.Background(), r); err != nil {
		t.Fatalf("the replacement does not answer: %v", err)
	}
}

// trappingModule is a hand-assembled wasm module whose hc_handle executes
// unreachable when the hook's name is five bytes long ("event") and answers
// with nothing otherwise. Like spinningModule it needs no toolchain; unlike it,
// it keeps running after a call has failed, which is what a trap is about.
func trappingModule() []byte {
	return []byte{
		// header: magic and version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		// type section: three types
		//   0: () -> ()                       _initialize
		//   1: (i32) -> i32                   hc_alloc
		//   2: (i32, i32, i32) -> i64         hc_handle
		0x01, 0x10, 0x03,
		0x60, 0x00, 0x00,
		0x60, 0x01, 0x7f, 0x01, 0x7f,
		0x60, 0x03, 0x7f, 0x7f, 0x7f, 0x01, 0x7e,
		// function section: one function of each type
		0x03, 0x04, 0x03, 0x00, 0x01, 0x02,
		// memory section: one memory of one page (64 KiB), no maximum
		0x05, 0x03, 0x01, 0x00, 0x01,
		// export section: _initialize, hc_alloc, hc_handle and the memory
		0x07, 0x2f, 0x04,
		0x0b, '_', 'i', 'n', 'i', 't', 'i', 'a', 'l', 'i', 'z', 'e', 0x00, 0x00,
		0x08, 'h', 'c', '_', 'a', 'l', 'l', 'o', 'c', 0x00, 0x01,
		0x09, 'h', 'c', '_', 'h', 'a', 'n', 'd', 'l', 'e', 0x00, 0x02,
		0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00,
		// code section: three bodies
		0x0a, 0x18, 0x03,
		// _initialize: end
		0x02, 0x00, 0x0b,
		// hc_alloc: i32.const 1024 (a fixed offset inside the one page), end
		0x05, 0x00, 0x41, 0x80, 0x08, 0x0b,
		// hc_handle: if the second parameter (the length of the hook's name) is
		// 5 then unreachable; i64.const 0 (an empty answer); end
		0x0d, 0x00, 0x20, 0x01, 0x41, 0x05, 0x46, 0x04, 0x40, 0x00, 0x0b, 0x42, 0x00, 0x0b,
	}
}

// A guest that trapped is in an unknown state. It is not resumed: the next call
// is served by a module of its own.
func TestATrappingPluginComesBack(t *testing.T) {
	r, s := reviveRuntimeWith(t, Limits{}, trappingModule())

	// The module has to work before it is allowed to trap.
	if err := r.Dispatch(context.Background(), "echo", HookContent, 1, ContentIn{HTML: "x"}, nil); err != nil {
		t.Fatalf("the hand-assembled module does not answer: %v", err)
	}
	before := instanceOf(r).mod

	err := r.Dispatch(context.Background(), "echo", HookEvent, 1, EventIn{Name: "boom"}, nil)
	if err == nil {
		t.Fatal("the trap was not reported")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("expected the trap in the error, got: %v", err)
	}
	if e := lastError(t, s); !strings.HasPrefix(e, "event:") {
		t.Errorf("the trap was not recorded: %q", e)
	}

	if err := r.Dispatch(context.Background(), "echo", HookContent, 1, ContentIn{HTML: "x"}, nil); err != nil {
		t.Fatalf("the plugin does not answer after a trap: %v", err)
	}
	if after := instanceOf(r).mod; after == before {
		t.Error("the module that trapped was used again")
	}
}
