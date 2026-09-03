package auth

import (
	"testing"
	"time"
)

// fixedClock lets the tests advance time without sleeping.
type fixedClock struct{ t time.Time }

func (c *fixedClock) now() time.Time          { return c.t }
func (c *fixedClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(max int, window time.Duration) (*LoginLimiter, *fixedClock) {
	clock := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	l := NewLoginLimiter(max, window)
	l.now = clock.now
	return l, clock
}

func TestLoginLimiterBlocksAfterMaxFailures(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !l.Allowed("ip:1.2.3.4") {
			t.Fatalf("attempt %d was blocked before the limit", i+1)
		}
		l.RecordFailure("ip:1.2.3.4")
	}

	if l.Allowed("ip:1.2.3.4") {
		t.Error("expected the key to be blocked after reaching the limit")
	}
	// A different client must be unaffected.
	if !l.Allowed("ip:5.6.7.8") {
		t.Error("an unrelated key was blocked")
	}
}

func TestLoginLimiterWindowExpires(t *testing.T) {
	l, clock := newTestLimiter(2, time.Minute)

	l.RecordFailure("ip:1.2.3.4")
	l.RecordFailure("ip:1.2.3.4")
	if l.Allowed("ip:1.2.3.4") {
		t.Fatal("expected the key to be blocked")
	}

	clock.advance(61 * time.Second)

	if !l.Allowed("ip:1.2.3.4") {
		t.Error("expected the block to lift once the window passed")
	}
}

// A successful login must clear the counters so the next typo does not lock out
// a user who just proved they know the password.
func TestLoginLimiterResetClearsCounters(t *testing.T) {
	l, _ := newTestLimiter(2, time.Minute)

	l.RecordFailure("ip:1.2.3.4", "email:a@example.com")
	l.RecordFailure("ip:1.2.3.4", "email:a@example.com")
	if l.Allowed("ip:1.2.3.4") {
		t.Fatal("expected the key to be blocked")
	}

	l.Reset("ip:1.2.3.4", "email:a@example.com")

	if !l.Allowed("ip:1.2.3.4", "email:a@example.com") {
		t.Error("expected Reset to unblock both keys")
	}
}

// Allowed takes several keys; any one of them being over the limit blocks.
func TestLoginLimiterBlocksWhenAnyKeyIsOverLimit(t *testing.T) {
	l, _ := newTestLimiter(2, time.Minute)

	// The email is hammered from rotating addresses.
	l.RecordFailure("email:victim@example.com")
	l.RecordFailure("email:victim@example.com")

	if l.Allowed("ip:9.9.9.9", "email:victim@example.com") {
		t.Error("expected a fresh IP to still be blocked by the email counter")
	}
}

func TestLoginLimiterRetryAfter(t *testing.T) {
	l, clock := newTestLimiter(2, time.Minute)

	if got := l.RetryAfter("ip:1.2.3.4"); got != 0 {
		t.Errorf("RetryAfter on an unused key = %v; want 0", got)
	}

	l.RecordFailure("ip:1.2.3.4")
	clock.advance(20 * time.Second)
	l.RecordFailure("ip:1.2.3.4")

	// The oldest failure is 20s old, so 40s of the 60s window remain.
	if got := l.RetryAfter("ip:1.2.3.4"); got != 40*time.Second {
		t.Errorf("RetryAfter = %v; want 40s", got)
	}
}

// The per-account limit must be loose enough that a single attacker cannot lock
// a known admin address out just by failing the client limit.
func TestLoginThrottleClientLockoutDoesNotLockTheAccount(t *testing.T) {
	throttle := NewLoginThrottle(3, 100, time.Minute)

	for i := 0; i < 3; i++ {
		throttle.RecordFailure("1.2.3.4", "admin@example.com")
	}

	if throttle.Allowed("1.2.3.4", "admin@example.com") {
		t.Error("expected the attacking client to be blocked")
	}
	// The legitimate admin, coming from a different address, must still get in.
	if !throttle.Allowed("5.6.7.8", "admin@example.com") {
		t.Error("a single attacker locked the account for everyone else")
	}
}

// A distributed attack against one account is still caught by the account limit.
func TestLoginThrottleBlocksAccountAfterManyFailures(t *testing.T) {
	throttle := NewLoginThrottle(3, 10, time.Minute)

	for i := 0; i < 10; i++ {
		// Every attempt from a different address, so the client limit never trips.
		throttle.RecordFailure(string(rune('a'+i)), "admin@example.com")
	}

	if throttle.Allowed("fresh-address", "admin@example.com") {
		t.Error("expected the account limit to catch a distributed attack")
	}
	if !throttle.Allowed("fresh-address", "other@example.com") {
		t.Error("an unrelated account was blocked")
	}
}

func TestLoginThrottleResetAfterSuccess(t *testing.T) {
	throttle := NewLoginThrottle(2, 10, time.Minute)

	throttle.RecordFailure("1.2.3.4", "admin@example.com")
	throttle.RecordFailure("1.2.3.4", "admin@example.com")
	if throttle.Allowed("1.2.3.4", "admin@example.com") {
		t.Fatal("expected the client to be blocked")
	}

	throttle.Reset("1.2.3.4", "admin@example.com")

	if !throttle.Allowed("1.2.3.4", "admin@example.com") {
		t.Error("a successful login did not clear the counters")
	}
}

func TestLoginLimiterCleanupDropsExpiredKeys(t *testing.T) {
	l, clock := newTestLimiter(2, time.Minute)

	l.RecordFailure("ip:1.2.3.4")
	clock.advance(2 * time.Minute)
	l.Cleanup()

	l.mu.Lock()
	remaining := len(l.attempts)
	l.mu.Unlock()

	if remaining != 0 {
		t.Errorf("expected expired keys to be dropped, %d remain", remaining)
	}
}
