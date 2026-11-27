package auth

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"
)

// The limits of the throttles that are not the login. The login's own are given
// to NewLoginThrottle by main (10 per client, 100 per account, 15 minutes).
const (
	// SecondFactorClientMax is how many second-step guesses one client may make
	// in a window.
	SecondFactorClientMax = 10

	// SecondFactorAccountMax is how many second-step guesses an account takes
	// in a window. It is low because the code has six digits and whoever is
	// guessing it already holds the password. It is not lower because whoever
	// holds the password can spend the budget and lock the owner's second step
	// for the window; a restart clears it, and deploy/DEPLOY.md says so.
	SecondFactorAccountMax = 5

	// ConfirmClientMax is how many wrong re-entered passwords one client may
	// make in a window, and ConfirmUserMax how many one signed-in person's own
	// confirmations take.
	ConfirmClientMax = 10
	ConfirmUserMax   = 5
)

// DefaultMaxKeys is how many keys a limiter keeps. Each key holds at most its
// limit of timestamps, so a full limiter is bounded at DefaultMaxKeys times its
// limit: about 40 MiB for the largest, the login's account axis (100 each),
// and a few megabytes for every other one.
//
// A key that still has an attempt in its window is never forgotten to make
// room. The account axis of the login is keyed on whatever address is typed,
// so anybody can mint fresh keys; if a flood of them could push out a key at
// its limit, that account would get a fresh budget each time (code review
// CR-01). When every held key is still live, a NEW key is refused instead,
// with the wait until the oldest one leaves its window, and a key already held
// goes on being counted as before. Filling a limiter therefore takes
// DefaultMaxKeys attempts inside one window, each of which went through the
// Argon2 gate or is waiting at it; a restart clears it, like every throttle.
const DefaultMaxKeys = 10000

// maxStoredKey is the longest key kept as given. A longer one is stored as a
// short digest, so a submitted e-mail address cannot make a map key as large as
// the request body allows.
const maxStoredKey = 64

// storeKey is the form of key the limiter keeps. Callers pass account keys
// already lower-cased.
func storeKey(key string) string {
	if len(key) <= maxStoredKey {
		return key
	}
	sum := sha256.Sum256([]byte(key))
	return "sha256:" + hex.EncodeToString(sum[:])[:32]
}

// Throttle limits one kind of attempt on two axes, the client and the account.
//
// A strict per-client limit stops an individual attacker quickly. A much looser
// per-account limit catches a distributed attack against one account without
// handing anyone an easy lockout: if the account limit were as strict as the
// client limit, a stranger could disable a known admin account with a handful of
// requests from a single machine.
//
// An attempt is reserved before any work is done: Reserve checks and counts in
// one step under the lock, so a parallel burst of N requests against a limit of
// M passes at most M. The caller then settles what it reserved (see Attempt).
type Throttle struct {
	client  *LoginLimiter
	account *LoginLimiter
}

// NewThrottle builds the two-axis throttle. clientMax should be small
// (individual attacker), accountMax considerably larger (distributed attack).
func NewThrottle(clientMax, accountMax int, window time.Duration) *Throttle {
	return &Throttle{
		client:  NewLoginLimiter(clientMax, window),
		account: NewLoginLimiter(accountMax, window),
	}
}

// Reserve counts one attempt for the client and the account, or refuses.
//
// It returns an Attempt to settle, or nil and how long to wait when either axis
// is full; a refusal counts nothing. An empty key skips its axis. The caller
// should defer attempt.Refund() right away, so that every exit that does not
// settle the attempt gives its reservation back.
func (t *Throttle) Reserve(client, account string) (*Attempt, time.Duration) {
	a := &Attempt{t: t, client: client, account: account}
	if client != "" {
		at, wait, ok := t.client.reserve(client)
		if !ok {
			if account != "" {
				wait = max(wait, t.account.RetryAfter(account))
			}
			return nil, wait
		}
		a.clientAt, a.hasClient = at, true
	}
	if account != "" {
		at, wait, ok := t.account.reserve(account)
		if !ok {
			if a.hasClient {
				t.client.refund(client, a.clientAt)
			}
			return nil, wait
		}
		a.accountAt, a.hasAccount = at, true
	}
	return a, 0
}

// RetryAfter reports how long the caller must wait before an attempt may be
// reserved again. Zero when neither axis is full.
func (t *Throttle) RetryAfter(client, account string) time.Duration {
	return max(t.client.RetryAfter(client), t.account.RetryAfter(account))
}

// Cleanup drops fully expired entries on both axes.
func (t *Throttle) Cleanup() {
	t.client.Cleanup()
	t.account.Cleanup()
}

// Attempt is one reservation. It is settled once; later calls do nothing.
//
//   - Fail keeps the reservations: the attempt was a wrong guess.
//   - Succeed gives back this attempt's client reservation and clears the
//     account key. The account's owner proved the password; the client's
//     earlier failures stand, so a thief who owns one account cannot reset
//     their own budget by logging in to it between guesses at another.
//   - Refund gives back both reservations: the attempt was not a guess (a busy
//     gate, a server error, a cancelled request).
type Attempt struct {
	t          *Throttle
	client     string
	account    string
	clientAt   time.Time
	accountAt  time.Time
	hasClient  bool
	hasAccount bool
	settled    atomic.Bool
}

// Fail keeps the reservations.
func (a *Attempt) Fail() { a.settled.Store(true) }

// Succeed settles the attempt as a right password.
func (a *Attempt) Succeed() {
	if !a.settled.CompareAndSwap(false, true) {
		return
	}
	if a.hasClient {
		a.t.client.refund(a.client, a.clientAt)
	}
	if a.account != "" {
		a.t.account.clear(a.account)
	}
}

// Refund gives back both reservations.
func (a *Attempt) Refund() {
	if !a.settled.CompareAndSwap(false, true) {
		return
	}
	if a.hasClient {
		a.t.client.refund(a.client, a.clientAt)
	}
	if a.hasAccount {
		a.t.account.refund(a.account, a.accountAt)
	}
}

// LoginThrottle owns the throttles of the admin: the login, the second step of
// a sign-in, the wrong codes of one pending sign-in and the re-entering of
// one's own password.
type LoginThrottle struct {
	login        *Throttle
	secondFactor *Throttle
	pending      *Throttle
	confirm      *Throttle
}

// NewLoginThrottle builds the throttles. clientMax and accountMax are the
// login's limits; the second step and the confirmation have their own (the
// constants above), all on the same window.
func NewLoginThrottle(clientMax, accountMax int, window time.Duration) *LoginThrottle {
	return &LoginThrottle{
		login:        NewThrottle(clientMax, accountMax, window),
		secondFactor: NewThrottle(SecondFactorClientMax, SecondFactorAccountMax, window),
		pending:      NewThrottle(PendingSecondFactorFailures, PendingSecondFactorFailures, PendingSignInWindow),
		confirm:      NewThrottle(ConfirmClientMax, ConfirmUserMax, window),
	}
}

// Login is the throttle of the sign-in form.
func (t *LoginThrottle) Login() *Throttle { return t.login }

// SecondFactor is the throttle of the second step of a sign-in.
func (t *LoginThrottle) SecondFactor() *Throttle { return t.secondFactor }

// PendingSignIn counts the wrong codes of one pending sign-in, keyed by its
// nonce (SessionKeyPendingNonce) on the account axis; callers pass no client
// key. It allows PendingSecondFactorFailures within PendingSignInWindow, so
// the limit holds as long as the session can, and a refused reservation ends
// the pending sign-in. Only someone who has the password reaches it, and each
// of its keys holds at most three attempts.
func (t *LoginThrottle) PendingSignIn() *Throttle { return t.pending }

// Confirm is the throttle of a signed-in person re-entering their password.
func (t *LoginThrottle) Confirm() *Throttle { return t.confirm }

// Cleanup drops fully expired entries on every axis of every throttle.
func (t *LoginThrottle) Cleanup() {
	t.login.Cleanup()
	t.secondFactor.Cleanup()
	t.pending.Cleanup()
	t.confirm.Cleanup()
}

// LoginLimiter throttles repeated attempts by key.
//
// It is a plain in-memory sliding-window counter, appropriate for a
// single-binary, single-host deployment, and it is bounded: it keeps at most
// maxKeys keys and sweeps the keys whose window has passed on every
// reservation. A live key is never forgotten to make room; a new key is refused
// while the limiter is full of them (see DefaultMaxKeys). A restart clears it.
type LoginLimiter struct {
	mu   sync.Mutex
	keys map[string]*list.Element // of *limitedKey
	// order is by the newest reservation, most recent at the front, so the
	// sweep can stop at the first key still in its window. A refused
	// reservation does not move its key: no live key is ever evicted, so the
	// order decides only what the sweep looks at, and moving a key whose newest
	// attempt is old to the front would hide it from the sweep.
	order *list.List

	max     int
	maxKeys int
	window  time.Duration

	// now is injectable for tests.
	now func() time.Time
}

// limitedKey is one key's attempts in the window, oldest first.
type limitedKey struct {
	key   string
	times []time.Time
}

// NewLoginLimiter creates a limiter allowing max attempts per key within window.
func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{
		keys:    make(map[string]*list.Element),
		order:   list.New(),
		max:     max,
		maxKeys: DefaultMaxKeys,
		window:  window,
		now:     time.Now,
	}
}

// reserve counts an attempt for key when there is room and reports the moment
// it was counted at; otherwise it counts nothing and reports how long until
// there will be room. The check and the count are one step under the lock.
//
// There is no room when the key is at its limit, and, for a key the limiter
// does not hold yet, when it already holds maxKeys live keys.
func (l *LoginLimiter) reserve(key string) (at time.Time, wait time.Duration, ok bool) {
	key = storeKey(key)
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)
	times := l.recentLocked(key, now)
	if l.max <= 0 {
		return time.Time{}, l.window, false
	}
	if len(times) >= l.max {
		return time.Time{}, times[0].Add(l.window).Sub(now), false
	}
	if len(times) == 0 && l.order.Len() >= l.maxKeys {
		return time.Time{}, l.fullWaitLocked(now), false
	}
	l.addLocked(key, now)
	return now, 0, true
}

// fullWaitLocked is how long a new key waits while the limiter is full: until
// the key at the back, the one reserved longest ago, leaves its window. The
// sweep has just run, so that key is still in it.
func (l *LoginLimiter) fullWaitLocked(now time.Time) time.Duration {
	wait := l.window
	if el := l.order.Back(); el != nil {
		if times := el.Value.(*limitedKey).times; len(times) > 0 {
			wait = times[len(times)-1].Add(l.window).Sub(now)
		}
	}
	return max(wait, time.Second)
}

// refund removes exactly one reservation made at the given moment, and forgets
// a key left with none.
func (l *LoginLimiter) refund(key string, at time.Time) {
	key = storeKey(key)
	l.mu.Lock()
	defer l.mu.Unlock()

	el, ok := l.keys[key]
	if !ok {
		return
	}
	k := el.Value.(*limitedKey)
	for i := len(k.times) - 1; i >= 0; i-- {
		if k.times[i].Equal(at) {
			k.times = append(k.times[:i], k.times[i+1:]...)
			break
		}
	}
	if len(k.times) == 0 {
		l.forgetLocked(el)
	}
}

// clear forgets a key altogether.
func (l *LoginLimiter) clear(key string) {
	key = storeKey(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	if el, ok := l.keys[key]; ok {
		l.forgetLocked(el)
	}
}

// addLocked records an attempt at now and makes key the most recently
// reserved. reserve has checked that a new key fits under maxKeys.
func (l *LoginLimiter) addLocked(key string, now time.Time) {
	if el, ok := l.keys[key]; ok {
		k := el.Value.(*limitedKey)
		k.times = append(k.times, now)
		l.order.MoveToFront(el)
		return
	}
	l.keys[key] = l.order.PushFront(&limitedKey{key: key, times: []time.Time{now}})
}

func (l *LoginLimiter) forgetLocked(el *list.Element) {
	delete(l.keys, el.Value.(*limitedKey).key)
	l.order.Remove(el)
}

// sweepLocked forgets the keys at the back of the order whose newest attempt
// the window has passed. It stops at the first key still in the window:
// everything in front of it was touched later.
func (l *LoginLimiter) sweepLocked(now time.Time) {
	cutoff := now.Add(-l.window)
	for el := l.order.Back(); el != nil; el = l.order.Back() {
		k := el.Value.(*limitedKey)
		if len(k.times) > 0 && k.times[len(k.times)-1].After(cutoff) {
			return
		}
		l.forgetLocked(el)
	}
}

// recentLocked drops the expired attempts of key and returns the remaining
// ones; a key with none left is forgotten.
func (l *LoginLimiter) recentLocked(key string, now time.Time) []time.Time {
	el, ok := l.keys[key]
	if !ok {
		return nil
	}
	k := el.Value.(*limitedKey)
	cutoff := now.Add(-l.window)
	kept := k.times[:0]
	for _, t := range k.times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	k.times = kept
	if len(kept) == 0 {
		l.forgetLocked(el)
		return nil
	}
	return kept
}

// RetryAfter reports how long the caller must wait before the oldest recorded
// attempt for key falls out of the window. Zero when the key is not limited.
func (l *LoginLimiter) RetryAfter(keys ...string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	var longest time.Duration
	for _, key := range keys {
		times := l.recentLocked(storeKey(key), now)
		if len(times) < l.max {
			continue
		}
		if wait := times[0].Add(l.window).Sub(now); wait > longest {
			longest = wait
		}
	}
	return longest
}

// Cleanup drops all entries whose attempts have fully expired. Call periodically
// so keys that are never retried do not accumulate beyond the sweep.
func (l *LoginLimiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	for el := l.order.Back(); el != nil; {
		prev := el.Prev()
		l.recentLocked(el.Value.(*limitedKey).key, now)
		el = prev
	}
}
