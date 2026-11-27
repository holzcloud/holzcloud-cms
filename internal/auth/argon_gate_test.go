package auth

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// None of these tests calls t.Parallel: they swap the package's gate and its
// key-derivation seam, and a test that does not call t.Parallel never overlaps
// another one.

var gateTestParams = Argon2Params{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

// useGate installs g for the test and restores the previous gate afterwards.
func useGate(t *testing.T, g *Gate) {
	t.Helper()
	previous := SetArgon2Gate(g)
	t.Cleanup(func() { SetArgon2Gate(previous) })
}

// countDerivations replaces the key function with one that counts its calls
// and defers to the real one.
func countDerivations(t *testing.T) *atomic.Int64 {
	t.Helper()
	var n atomic.Int64
	real := argon2Key
	argon2Key = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		n.Add(1)
		return real(password, salt, time, memory, threads, keyLen)
	}
	t.Cleanup(func() { argon2Key = real })
	return &n
}

func TestAFullGateRefusesAPasswordCheckWithoutDerivingAKey(t *testing.T) {
	hash, err := HashPassword("geheim-geheim", gateTestParams)
	if err != nil {
		t.Fatal(err)
	}
	g := NewGate(1, 20*time.Millisecond, 20*time.Millisecond)
	useGate(t, g)
	derivations := countDerivations(t)

	release, err := g.Acquire(0)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ok, err := VerifyPassword("geheim-geheim", hash)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("VerifyPassword with a full gate returned %v, want ErrBusy", err)
	}
	if ok {
		t.Error("a busy check reported the password as right")
	}
	if n := derivations.Load(); n != 0 {
		t.Errorf("a refused check derived %d keys, want none", n)
	}
}

func TestEveryArgon2PathGoesThroughTheGate(t *testing.T) {
	hash, err := HashPassword("geheim-geheim", gateTestParams)
	if err != nil {
		t.Fatal(err)
	}
	g := NewGate(1, 10*time.Millisecond, 10*time.Millisecond)
	useGate(t, g)

	release, err := g.Acquire(0)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]func() error{
		"HashPassword": func() error {
			_, err := HashPassword("neues-passwort", gateTestParams)
			return err
		},
		"VerifyPassword": func() error {
			_, err := VerifyPassword("geheim-geheim", hash)
			return err
		},
		"VerifyOwnPassword": func() error {
			_, err := VerifyOwnPassword("geheim-geheim", hash)
			return err
		},
		"VerifyDummyPassword": func() error {
			return VerifyDummyPassword("irgendwas", gateTestParams)
		},
	}
	for name, call := range paths {
		if err := call(); !errors.Is(err, ErrBusy) {
			t.Errorf("%s with a full gate returned %v, want ErrBusy", name, err)
		}
	}

	release()
	for name, call := range paths {
		if err := call(); err != nil {
			t.Errorf("%s with a free gate returned %v, want nil", name, err)
		}
	}
}

func TestTheGateNeverRunsMoreDerivationsThanItHasSlots(t *testing.T) {
	hash, err := HashPassword("geheim-geheim", gateTestParams)
	if err != nil {
		t.Fatal(err)
	}
	// Long waits: every goroutine queues for a slot rather than giving up.
	useGate(t, NewGate(2, 30*time.Second, 30*time.Second))

	var running, highest atomic.Int64
	entered := make(chan struct{}, 20)
	block := make(chan struct{})
	real := argon2Key
	argon2Key = func(password, salt []byte, tm, memory uint32, threads uint8, keyLen uint32) []byte {
		now := running.Add(1)
		for {
			h := highest.Load()
			if now <= h || highest.CompareAndSwap(h, now) {
				break
			}
		}
		entered <- struct{}{}
		<-block
		running.Add(-1)
		return real(password, salt, tm, memory, threads, keyLen)
	}
	t.Cleanup(func() { argon2Key = real })

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := VerifyPassword("geheim-geheim", hash); err != nil {
				t.Errorf("VerifyPassword: %v", err)
			}
		}()
	}
	// Two derivations are inside; the other eighteen are waiting.
	<-entered
	<-entered
	select {
	case <-entered:
		t.Error("a third derivation started while two slots were held")
	case <-time.After(50 * time.Millisecond):
	}
	close(block)
	wg.Wait()
	if h := highest.Load(); h != 2 {
		t.Errorf("highest number of derivations at once = %d, want 2", h)
	}
}

func TestPrivilegedChecksWaitLongerThanAnonymousOnes(t *testing.T) {
	hash, err := HashPassword("geheim-geheim", gateTestParams)
	if err != nil {
		t.Fatal(err)
	}
	g := NewGate(1, 20*time.Millisecond, 2*time.Second)
	useGate(t, g)

	release, err := g.Acquire(0)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		release()
	}()

	if _, err := VerifyPassword("geheim-geheim", hash); !errors.Is(err, ErrBusy) {
		t.Fatalf("an anonymous check should give up within 20 ms, got %v", err)
	}
	// The privileged ones outwait the 200 ms; the first takes the slot and
	// frees it again, so the second finds it free.
	if ok, err := VerifyOwnPassword("geheim-geheim", hash); err != nil || !ok {
		t.Errorf("VerifyOwnPassword = %v, %v, want true, nil", ok, err)
	}
	if _, err := HashPassword("neues-passwort", gateTestParams); err != nil {
		t.Errorf("HashPassword: %v", err)
	}
}

func TestAHashWithALargerMemoryCostTakesOneSlot(t *testing.T) {
	big := gateTestParams
	big.Memory = 2 * gateTestParams.Memory
	hash, err := HashPassword("geheim-geheim", big)
	if err != nil {
		t.Fatal(err)
	}
	useGate(t, NewGate(1, time.Second, time.Second))

	var running, highest atomic.Int64
	var storedMemory atomic.Uint32
	real := argon2Key
	argon2Key = func(password, salt []byte, tm, memory uint32, threads uint8, keyLen uint32) []byte {
		now := running.Add(1)
		highest.Store(max(highest.Load(), now))
		storedMemory.Store(memory)
		defer running.Add(-1)
		return real(password, salt, tm, memory, threads, keyLen)
	}
	t.Cleanup(func() { argon2Key = real })

	if ok, err := VerifyPassword("geheim-geheim", hash); err != nil || !ok {
		t.Fatalf("VerifyPassword = %v, %v", ok, err)
	}
	if h := highest.Load(); h != 1 {
		t.Errorf("a larger stored cost held %d derivations at once, want 1", h)
	}
	if m := storedMemory.Load(); m != big.Memory {
		t.Errorf("derived with m=%d, want the stored m=%d", m, big.Memory)
	}
}
