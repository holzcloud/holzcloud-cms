package auth

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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

// A wrong guess keeps its reservation, so a client is blocked after its limit
// and nobody else is.
func TestThrottleBlocksAClientAfterMaxFailures(t *testing.T) {
	th, _ := newTestThrottle(3, 100, time.Minute)

	for i := 0; i < 3; i++ {
		a, _ := th.Reserve("ip:1.2.3.4", "")
		if a == nil {
			t.Fatalf("attempt %d was blocked before the limit", i+1)
		}
		a.Fail()
	}

	if a, _ := th.Reserve("ip:1.2.3.4", ""); a != nil {
		t.Error("expected the key to be blocked after reaching the limit")
	}
	// A different client must be unaffected.
	if a, _ := th.Reserve("ip:5.6.7.8", ""); a == nil {
		t.Error("an unrelated key was blocked")
	}
}

func TestThrottleWindowExpires(t *testing.T) {
	th, clock := newTestThrottle(2, 100, time.Minute)

	for i := 0; i < 2; i++ {
		a, _ := th.Reserve("ip:1.2.3.4", "")
		a.Fail()
	}
	if a, _ := th.Reserve("ip:1.2.3.4", ""); a != nil {
		t.Fatal("expected the key to be blocked")
	}

	clock.advance(61 * time.Second)

	if a, _ := th.Reserve("ip:1.2.3.4", ""); a == nil {
		t.Error("expected the block to lift once the window passed")
	}
}

// Any one axis over its limit blocks: an account hammered from rotating
// addresses stays blocked for a fresh address.
func TestThrottleBlocksWhenTheAccountIsOverTheLimit(t *testing.T) {
	th, _ := newTestThrottle(10, 2, time.Minute)

	for _, client := range []string{"ip:1.1.1.1", "ip:2.2.2.2"} {
		a, _ := th.Reserve(client, "victim@example.com")
		a.Fail()
	}

	if a, _ := th.Reserve("ip:9.9.9.9", "victim@example.com"); a != nil {
		t.Error("expected a fresh IP to still be blocked by the account counter")
	}
}

// The refusal says how long until there is room: the oldest failure is 20s old,
// so 40s of the 60s window remain.
func TestThrottleRetryAfter(t *testing.T) {
	th, clock := newTestThrottle(2, 100, time.Minute)

	if got := th.RetryAfter("ip:1.2.3.4", ""); got != 0 {
		t.Errorf("RetryAfter on an unused key = %v; want 0", got)
	}

	a, _ := th.Reserve("ip:1.2.3.4", "")
	a.Fail()
	clock.advance(20 * time.Second)
	a, _ = th.Reserve("ip:1.2.3.4", "")
	a.Fail()

	if got := th.RetryAfter("ip:1.2.3.4", ""); got != 40*time.Second {
		t.Errorf("RetryAfter = %v; want 40s", got)
	}
	a, wait := th.Reserve("ip:1.2.3.4", "")
	if a != nil || wait != 40*time.Second {
		t.Errorf("a refused Reserve answered %v after %v; want nil after 40s", a, wait)
	}
}

// The per-account limit must be loose enough that a single attacker cannot lock
// a known admin address out just by failing the client limit.
func TestLoginThrottleClientLockoutDoesNotLockTheAccount(t *testing.T) {
	throttle := NewLoginThrottle(3, 100, time.Minute).Login()

	for i := 0; i < 3; i++ {
		a, _ := throttle.Reserve("1.2.3.4", "admin@example.com")
		a.Fail()
	}

	if a, _ := throttle.Reserve("1.2.3.4", "admin@example.com"); a != nil {
		t.Error("expected the attacking client to be blocked")
	}
	// The legitimate admin, coming from a different address, must still get in.
	if a, _ := throttle.Reserve("5.6.7.8", "admin@example.com"); a == nil {
		t.Error("a single attacker locked the account for everyone else")
	}
}

// A distributed attack against one account is still caught by the account limit.
func TestLoginThrottleBlocksAccountAfterManyFailures(t *testing.T) {
	throttle := NewLoginThrottle(3, 10, time.Minute).Login()

	for i := 0; i < 10; i++ {
		// Every attempt from a different address, so the client limit never trips.
		a, _ := throttle.Reserve(string(rune('a'+i)), "admin@example.com")
		if a == nil {
			t.Fatalf("attempt %d was blocked before the account limit", i+1)
		}
		a.Fail()
	}

	if a, _ := throttle.Reserve("fresh-address", "admin@example.com"); a != nil {
		t.Error("expected the account limit to catch a distributed attack")
	}
	if a, _ := throttle.Reserve("fresh-address", "other@example.com"); a == nil {
		t.Error("an unrelated account was blocked")
	}
}

// A successful login clears the account's key, so the next typo does not lock
// out somebody who just proved they know the password.
func TestLoginThrottleSuccessClearsTheAccount(t *testing.T) {
	throttle := NewLoginThrottle(10, 2, time.Minute).Login()

	a, _ := throttle.Reserve("1.2.3.4", "admin@example.com")
	a.Fail()
	a, _ = throttle.Reserve("5.6.7.8", "admin@example.com")
	a.Succeed()

	for i := 0; i < 2; i++ {
		a, _ := throttle.Reserve("9.9.9.9", "admin@example.com")
		if a == nil {
			t.Fatalf("attempt %d after a success was blocked: the account key was not cleared", i+1)
		}
		a.Fail()
	}
}

func TestLoginLimiterCleanupDropsExpiredKeys(t *testing.T) {
	l, clock := newTestLimiter(2, time.Minute)

	if _, _, ok := l.reserve("ip:1.2.3.4"); !ok {
		t.Fatal("the reservation was refused")
	}
	clock.advance(2 * time.Minute)
	l.Cleanup()

	l.mu.Lock()
	remaining := len(l.keys)
	l.mu.Unlock()

	if remaining != 0 {
		t.Errorf("expected expired keys to be dropped, %d remain", remaining)
	}
}

// count is how many attempts the limiter holds for key, for the tests.
func (l *LoginLimiter) count(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if el, ok := l.keys[storeKey(key)]; ok {
		return len(el.Value.(*limitedKey).times)
	}
	return 0
}

// newTestThrottle builds a throttle whose two limiters share one injected
// clock.
func newTestThrottle(clientMax, accountMax int, window time.Duration) (*Throttle, *fixedClock) {
	clock := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	th := NewThrottle(clientMax, accountMax, window)
	th.client.now = clock.now
	th.account.now = clock.now
	return th, clock
}

// The old pair, Allowed and then RecordFailure, lets every goroutine of a burst
// through the check before any of them has been counted. Reserve counts in the
// same step it checks, so the limit holds however the goroutines interleave.
func TestAParallelBurstCannotExceedTheLimit(t *testing.T) {
	th := NewThrottle(10, 100, time.Minute)

	start := make(chan struct{})
	var wg sync.WaitGroup
	var passed, refused atomic.Int64
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			a, wait := th.Reserve("203.0.113.7", "owner@example.com")
			if a == nil {
				if wait <= 0 {
					t.Errorf("a refusal reported a wait of %v", wait)
				}
				refused.Add(1)
				return
			}
			a.Fail()
			passed.Add(1)
		}()
	}
	close(start)
	wg.Wait()

	if passed.Load() != 10 || refused.Load() != 190 {
		t.Errorf("passed %d, refused %d; want 10 and 190", passed.Load(), refused.Load())
	}
}

func TestASuccessClearsOnlyTheAccountKey(t *testing.T) {
	th, _ := newTestThrottle(10, 100, time.Minute)

	for i := 0; i < 9; i++ {
		a, _ := th.Reserve("X", "b@example.com")
		if a == nil {
			t.Fatalf("failure %d was refused", i+1)
		}
		a.Fail()
	}
	a, _ := th.Reserve("X", "a@example.com")
	if a == nil {
		t.Fatal("the attempt for account A was refused")
	}
	a.Succeed()

	tenth, _ := th.Reserve("X", "b@example.com")
	if tenth == nil {
		t.Fatal("the tenth attempt of client X was refused; Succeed must give back only its own reservation")
	}
	tenth.Fail()
	if again, _ := th.Reserve("X", "b@example.com"); again != nil {
		t.Error("an eleventh attempt of client X passed: Succeed cleared the client key as well")
	}

	// And what Succeed clears is the account key: five failures on account C
	// fill it, a success on C clears it.
	th2, _ := newTestThrottle(100, 5, time.Minute)
	for i := 0; i < 4; i++ {
		f, _ := th2.Reserve("Y", "c@example.com")
		f.Fail()
	}
	ok, _ := th2.Reserve("Z", "c@example.com")
	ok.Succeed()
	if th2.account.count("c@example.com") != 0 {
		t.Error("Succeed left the account key counted")
	}
	if th2.client.count("Z") != 0 {
		t.Error("Succeed kept its own client reservation")
	}
	if th2.client.count("Y") != 4 {
		t.Error("Succeed touched another client's failures")
	}
}

func TestARefundGivesBackOnlyItsOwnReservation(t *testing.T) {
	th, clock := newTestThrottle(10, 100, time.Minute)

	for i := 0; i < 3; i++ {
		a, _ := th.Reserve("X", "u@example.com")
		a.Fail()
		clock.advance(time.Second)
	}
	a, _ := th.Reserve("X", "u@example.com")
	a.Refund()
	if n := th.client.count("X"); n != 3 {
		t.Errorf("the client key holds %d entries after a refund, want 3", n)
	}

	// Settling is once only.
	a.Refund()
	a.Fail()
	a.Succeed()
	if n := th.client.count("X"); n != 3 {
		t.Errorf("settling a second time changed the client key to %d entries", n)
	}
	b, _ := th.Reserve("X", "u@example.com")
	b.Fail()
	b.Refund()
	if n := th.client.count("X"); n != 4 {
		t.Errorf("a refund after Fail took the reservation back: %d entries, want 4", n)
	}

	// An attempt never settled keeps its reservation until somebody refunds it.
	c, _ := th.Reserve("X", "u@example.com")
	if n := th.client.count("X"); n != 5 {
		t.Errorf("an unsettled attempt holds %d entries, want 5", n)
	}
	c.Refund()
	if n := th.client.count("X"); n != 4 {
		t.Errorf("after Refund %d entries, want 4", n)
	}
}

func TestAnEmptyKeyIsNotCounted(t *testing.T) {
	th, _ := newTestThrottle(1, 1, time.Minute)

	a, _ := th.Reserve("", "u@example.com")
	a.Fail()
	if th.client.count("") != 0 {
		t.Error("an empty client key was counted")
	}
	if b, _ := th.Reserve("", "u@example.com"); b != nil {
		t.Error("the account axis did not count")
	}
	c, _ := th.Reserve("X", "")
	c.Fail()
	if d, _ := th.Reserve("X", ""); d != nil {
		t.Error("the client axis did not count")
	}
	if e, _ := th.Reserve("X", "other@example.com"); e != nil {
		t.Error("client X should be full")
	}
}

func TestAnAccountAxisRefusalGivesTheClientReservationBack(t *testing.T) {
	th, _ := newTestThrottle(10, 1, time.Minute)
	a, _ := th.Reserve("X", "u@example.com")
	a.Fail()
	if b, wait := th.Reserve("X", "u@example.com"); b != nil || wait <= 0 {
		t.Fatalf("got %v, %v; want a refusal with a wait", b, wait)
	}
	if n := th.client.count("X"); n != 1 {
		t.Errorf("a refused reservation left the client key at %d entries, want 1", n)
	}
}

// A full limiter does not make room by forgetting a key that still counts:
// that key could be an account somebody is guessing at. It refuses the new key
// instead, so the memory stays bounded and no budget is handed out again.
func TestAFullLimiterRefusesANewKeyRatherThanForgetALiveOne(t *testing.T) {
	l, clock := newTestLimiter(5, time.Hour)
	l.maxKeys = 100

	for i := 0; i < 100; i++ {
		if _, _, ok := l.reserve(fmt.Sprintf("user%d@example.com", i)); !ok {
			t.Fatalf("reservation %d refused below the cap", i)
		}
	}
	for i := 100; i < 1000; i++ {
		if _, wait, ok := l.reserve(fmt.Sprintf("user%d@example.com", i)); ok || wait <= 0 {
			t.Fatalf("reservation %d past the cap: ok=%v wait=%v, want a refusal with a wait", i, ok, wait)
		}
	}
	l.mu.Lock()
	held, ordered := len(l.keys), l.order.Len()
	l.mu.Unlock()
	if held != 100 || ordered != 100 {
		t.Errorf("the limiter holds %d keys (%d in order), want 100", held, ordered)
	}
	if l.count("user0@example.com") != 1 {
		t.Error("the oldest key, still in its window, was forgotten to make room")
	}
	// A key already held is not new: it is still counted while the cap is full.
	if _, _, ok := l.reserve("user0@example.com"); !ok || l.count("user0@example.com") != 2 {
		t.Error("a key already held was refused because the limiter is full")
	}

	// Once the window has passed, the keys are swept and there is room again.
	clock.advance(time.Hour + time.Second)
	if _, _, ok := l.reserve("fresh@example.com"); !ok {
		t.Error("a new key was refused after every held key had left its window")
	}
}

// CR-01: the account axis is keyed on whatever address is typed, so anybody
// can make fresh keys. Spending an account's budget and then filling the
// limiter with junk must not give that account a fresh budget, whether the
// junk is counted as wrong guesses or only held (a request waiting at the
// gate, then refunded as busy).
func TestAFloodOfFreshKeysDoesNotRestoreASpentAccount(t *testing.T) {
	for _, settle := range []string{"fail", "refund"} {
		t.Run(settle, func(t *testing.T) {
			th, _ := newTestThrottle(10, 100, 15*time.Minute)
			for i := 0; i < 100; i++ {
				a, _ := th.Reserve(fmt.Sprintf("203.0.113.%d", i/10), "victim@example.com")
				if a == nil {
					t.Fatalf("guess %d at the victim refused", i+1)
				}
				a.Fail()
			}
			if a, _ := th.Reserve("198.51.100.1", "victim@example.com"); a != nil {
				t.Fatal("the victim's budget is not spent")
			}

			// DefaultMaxKeys junk addresses from a thousand client keys, ten each.
			var held []*Attempt
			for i := 0; i < DefaultMaxKeys; i++ {
				a, _ := th.Reserve(fmt.Sprintf("2001:db8:%x::/64", i/10), fmt.Sprintf("junk%d@example.com", i))
				if a == nil {
					continue
				}
				if settle == "fail" {
					a.Fail()
				} else {
					held = append(held, a)
				}
			}
			for _, a := range held {
				a.Refund()
			}

			if a, wait := th.Reserve("198.51.100.2", "victim@example.com"); a != nil || wait <= 0 {
				t.Errorf("after %d junk keys the spent account could be guessed again (wait %v)", DefaultMaxKeys, wait)
			}
			th.account.mu.Lock()
			n := len(th.account.keys)
			th.account.mu.Unlock()
			if n > DefaultMaxKeys {
				t.Errorf("the account axis holds %d keys, more than its cap of %d", n, DefaultMaxKeys)
			}
		})
	}
}

func TestALongKeyIsStoredShort(t *testing.T) {
	l, _ := newTestLimiter(5, time.Hour)
	long1 := strings.Repeat("a", 10*1024)
	long2 := strings.Repeat("a", 10*1024-1) + "b"
	for _, k := range []string{long1, long2, "short@example.com", strings.Repeat("c", 64)} {
		if _, _, ok := l.reserve(k); !ok {
			t.Fatalf("reservation refused for a key of %d bytes", len(k))
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.keys) != 4 {
		t.Errorf("%d keys held, want 4 (two long keys must stay two keys)", len(l.keys))
	}
	for k := range l.keys {
		if len(k) > 71 {
			t.Errorf("a key of %d bytes is stored", len(k))
		}
	}
	if _, ok := l.keys["short@example.com"]; !ok {
		t.Error("a short key was not stored as given")
	}
	if _, ok := l.keys[strings.Repeat("c", 64)]; !ok {
		t.Error("a key of exactly 64 bytes was not stored as given")
	}
}

func TestExpiredKeysAreSweptOnReservation(t *testing.T) {
	l, clock := newTestLimiter(5, time.Minute)
	for i := 0; i < 50; i++ {
		l.reserve(fmt.Sprintf("k%d", i))
	}
	clock.advance(2 * time.Minute)
	if _, _, ok := l.reserve("fresh"); !ok {
		t.Fatal("reservation refused")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.keys) != 1 {
		t.Errorf("%d keys held after one reservation past the window, want 1", len(l.keys))
	}
}

func TestTheSecondFactorAndConfirmationHaveTheirOwnLimits(t *testing.T) {
	lt := NewLoginThrottle(10, 100, time.Minute)

	for name, th := range map[string]*Throttle{"second factor": lt.SecondFactor(), "confirmation": lt.Confirm()} {
		for i := 0; i < 5; i++ {
			a, _ := th.Reserve(fmt.Sprintf("10.0.0.%d", i), "one@example.com")
			if a == nil {
				t.Fatalf("%s: failure %d was refused", name, i+1)
			}
			a.Fail()
		}
		if a, _ := th.Reserve("10.0.0.99", "one@example.com"); a != nil {
			t.Errorf("%s: the sixth failure of one account passed", name)
		}
	}
	if a, _ := lt.Login().Reserve("10.0.0.99", "one@example.com"); a == nil {
		t.Error("the second step or the confirmation used up the login's budget")
	}
}

// A throttle that keeps a budget for a client and has no account axis may make
// room for a new key by forgetting the one reserved longest ago (CR-01 of the
// phase 38 review). Refusing the new key would let anybody with enough
// addresses fill the table and lock every other client out for the window;
// forgetting a key can only hand that key a fresh budget, and there is no
// account whose guess budget it could reset. The login's throttles keep
// refusing (the test above).
func TestAnEvictingThrottleMakesRoomForANewClient(t *testing.T) {
	clock := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	th := NewEvictingThrottle(3, time.Hour)
	th.client.now = clock.now
	th.client.maxKeys = 100

	for i := 0; i < 100; i++ {
		a, _ := th.Reserve(fmt.Sprintf("2001:db8:%x::/64", i), "")
		if a == nil {
			t.Fatalf("reservation %d refused below the cap", i)
		}
		a.Fail()
	}
	for i := 100; i < 1000; i++ {
		a, wait := th.Reserve(fmt.Sprintf("2001:db8:%x::/64", i), "")
		if a == nil {
			t.Fatalf("new client %d refused with %v while the table is full", i, wait)
		}
		a.Fail()
	}
	th.client.mu.Lock()
	held, ordered := len(th.client.keys), th.client.order.Len()
	th.client.mu.Unlock()
	if held != 100 || ordered != 100 {
		t.Errorf("the limiter holds %d keys (%d in order), want its cap of 100", held, ordered)
	}
	if th.client.count("2001:db8:0::/64") != 0 || th.client.count("2001:db8:3e7::/64") != 1 {
		t.Error("the key reserved longest ago should have gone, the newest stayed")
	}
	// A client still held is counted on as before, and its own limit holds.
	for i := 0; i < 2; i++ {
		if a, _ := th.Reserve("2001:db8:3e7::/64", ""); a == nil {
			t.Fatalf("a held client was refused below its limit at %d", i+2)
		} else {
			a.Fail()
		}
	}
	if a, wait := th.Reserve("2001:db8:3e7::/64", ""); a != nil || wait <= 0 {
		t.Errorf("the fourth from one client: %v, %v; want a refusal", a, wait)
	}
}
