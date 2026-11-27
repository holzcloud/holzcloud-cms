package auth

import (
	"errors"
	"sync/atomic"
	"time"
)

// ErrBusy is the third outcome of a password check, beside "right" and
// "wrong": every slot of the Argon2 gate was taken and the check was not made.
//
// It must never be read as a wrong password. A caller that counted it as one
// would let an attacker who fills the gate lock the owner out through the
// defence, so every caller tests for it before any bookkeeping and answers
// "busy, try again" without touching a throttle.
var ErrBusy = errors.New("auth: too many password checks at once")

// The starting numbers of the gate. They are starting numbers: the first two
// are env-tunable (HOLZCLOUD_ARGON2_CONCURRENCY) and the rest are meant to be
// moved when a real server shows they are wrong.
const (
	// DefaultArgon2Slots is how many derivations may run at once. Two slots
	// are 128 MB at the default 64 MB per check, which a small server can
	// carry; fifty parallel checks, 3 GB, it could not.
	DefaultArgon2Slots = 2

	// AnonymousWait is how long a check by a stranger (a login, a page
	// password) waits for a slot. Short on purpose: a flood is answered
	// quickly, and a waiter holds no memory while it waits.
	AnonymousWait = 1500 * time.Millisecond

	// PrivilegedWait is how long a check by somebody entitled to the action
	// (hashing a new password, a signed-in person re-entering their own) waits.
	// It outlasts every anonymous waiter queued before it, so an
	// administrator's own action gets through an attack.
	PrivilegedWait = 10 * time.Second

	// BusyRetryAfter is the Retry-After a busy answer carries.
	BusyRetryAfter = 2 * time.Second
)

// Gate bounds how many Argon2 derivations run at once.
//
// It lives inside internal/auth, around the one place a key is derived, so
// every caller is bounded without a constructor change and no caller can
// forget it. A slot is held only around the derivation, never around a
// database call: the write pool has one connection, and a goroutine holding a
// slot while it waits for that connection, with the writer waiting for a slot,
// is a deadlock.
type Gate struct {
	slots     chan struct{}
	anonymous time.Duration
	privilege time.Duration
}

// NewGate builds a gate with the given number of slots (below 1 becomes 1)
// and the waits of the two classes of caller.
func NewGate(slots int, anonymousWait, privilegedWait time.Duration) *Gate {
	if slots < 1 {
		slots = 1
	}
	return &Gate{
		slots:     make(chan struct{}, slots),
		anonymous: anonymousWait,
		privilege: privilegedWait,
	}
}

// Slots is how many derivations the gate allows at once.
func (g *Gate) Slots() int { return cap(g.slots) }

// Acquire takes a slot, waiting up to wait for one. It returns ErrBusy when
// none became free in time. The caller must call release exactly once.
func (g *Gate) Acquire(wait time.Duration) (release func(), err error) {
	release = func() { <-g.slots }
	select {
	case g.slots <- struct{}{}:
		return release, nil
	default:
	}
	if wait <= 0 {
		return nil, ErrBusy
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case g.slots <- struct{}{}:
		return release, nil
	case <-timer.C:
		return nil, ErrBusy
	}
}

var argon2Gate atomic.Pointer[Gate]

func init() {
	argon2Gate.Store(NewGate(DefaultArgon2Slots, AnonymousWait, PrivilegedWait))
}

// SetArgon2Gate installs the gate every derivation runs in and returns the
// previous one. main calls it once at startup with the configured size; a test
// swaps one in and restores the previous one with t.Cleanup (only in tests that
// do not call t.Parallel, which never overlap parallel tests).
func SetArgon2Gate(g *Gate) (previous *Gate) {
	return argon2Gate.Swap(g)
}
