package plugin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// reviveRuntime is neueLaufzeit with short bounds: the limits are set before
// the first Load, as SetLimits requires, and the echo module is loaded after.
func reviveRuntime(t *testing.T, l Limits) (*Runtime, *Store) {
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
	p := &Package{Manifest: &m, Module: echoModule(t), SHA256: strings.Repeat("a", 64)}
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
