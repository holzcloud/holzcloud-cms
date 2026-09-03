package auth

import (
	"sync"
	"time"
)

// LoginThrottle limits failed login attempts on two axes.
//
// A strict per-client limit stops an individual attacker quickly. A much looser
// per-account limit catches a distributed attack against one mailbox without
// handing anyone an easy lockout: if the account limit were as strict as the
// client limit, a stranger could disable a known admin account with a handful of
// requests from a single machine.
type LoginThrottle struct {
	perClient  *LoginLimiter
	perAccount *LoginLimiter
}

// NewLoginThrottle builds the two-tier throttle. clientMax should be small
// (individual attacker), accountMax considerably larger (distributed attack).
func NewLoginThrottle(clientMax, accountMax int, window time.Duration) *LoginThrottle {
	return &LoginThrottle{
		perClient:  NewLoginLimiter(clientMax, window),
		perAccount: NewLoginLimiter(accountMax, window),
	}
}

// Allowed reports whether another login attempt may be made.
func (t *LoginThrottle) Allowed(ip, email string) bool {
	return t.perClient.Allowed(ip) && t.perAccount.Allowed(email)
}

// RecordFailure counts a failed attempt on both axes.
func (t *LoginThrottle) RecordFailure(ip, email string) {
	t.perClient.RecordFailure(ip)
	t.perAccount.RecordFailure(email)
}

// Reset clears both counters after a successful login, so a user who just
// proved they know the password is not locked out by their earlier typos.
func (t *LoginThrottle) Reset(ip, email string) {
	t.perClient.Reset(ip)
	t.perAccount.Reset(email)
}

// RetryAfter reports how long the caller must wait before trying again.
func (t *LoginThrottle) RetryAfter(ip, email string) time.Duration {
	if wait := t.perClient.RetryAfter(ip); wait > 0 {
		return wait
	}
	return t.perAccount.RetryAfter(email)
}

// Cleanup drops fully expired entries on both axes.
func (t *LoginThrottle) Cleanup() {
	t.perClient.Cleanup()
	t.perAccount.Cleanup()
}

// LoginLimiter throttles repeated failed login attempts.
//
// It is a plain in-memory sliding-window counter — appropriate for a
// single-binary, single-host deployment. Only failures count, so a legitimate
// user is never limited by someone else's successful traffic.
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time

	max    int
	window time.Duration

	// now is injectable for tests.
	now func() time.Time
}

// NewLoginLimiter creates a limiter allowing max failures per key within window.
func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
		now:      time.Now,
	}
}

// Allowed reports whether another attempt may be made for every given key.
// Keys with no recorded failures are always allowed.
func (l *LoginLimiter) Allowed(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := l.now().Add(-l.window)
	for _, key := range keys {
		if len(l.pruneLocked(key, cutoff)) >= l.max {
			return false
		}
	}
	return true
}

// RecordFailure counts a failed attempt against every given key.
func (l *LoginLimiter) RecordFailure(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)
	for _, key := range keys {
		l.attempts[key] = append(l.pruneLocked(key, cutoff), now)
	}
}

// Reset clears the counters for the given keys, called after a successful login.
func (l *LoginLimiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		delete(l.attempts, key)
	}
}

// RetryAfter reports how long the caller must wait before the oldest recorded
// failure for key falls out of the window. Zero when the key is not limited.
func (l *LoginLimiter) RetryAfter(keys ...string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)
	var longest time.Duration
	for _, key := range keys {
		times := l.pruneLocked(key, cutoff)
		if len(times) < l.max {
			continue
		}
		if wait := times[0].Add(l.window).Sub(now); wait > longest {
			longest = wait
		}
	}
	return longest
}

// pruneLocked drops expired attempts for key and returns the remaining ones.
// The caller must hold l.mu.
func (l *LoginLimiter) pruneLocked(key string, cutoff time.Time) []time.Time {
	times := l.attempts[key]
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.attempts, key)
		return nil
	}
	l.attempts[key] = kept
	return kept
}

// Cleanup drops all entries whose attempts have fully expired. Call periodically
// so keys that are never retried do not accumulate.
func (l *LoginLimiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := l.now().Add(-l.window)
	for key := range l.attempts {
		l.pruneLocked(key, cutoff)
	}
}
