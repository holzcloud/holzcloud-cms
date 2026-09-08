package auth

import (
	"testing"
	"time"
)

// VerifyDummyPassword exists to close a user-enumeration oracle: an unknown
// e-mail must cost the same as a known one with a wrong password. It had no
// test of any kind — found while sweeping internal/auth for exported functions
// no test names, which is how Phase 10 wave 6 discovered that
// MustHaveSecondFactor had none either.
//
// # The zero-value trap
//
// The function already defends ONE of its four parameters: keyLen falls back to
// the default when zero. The other three are handed to argon2.IDKey raw, and
// argon2 panics on zero rounds — "argon2: number of rounds too small".
//
// Production cannot reach that: config.Load floors memory at 8 KB and refuses
// zero iterations and zero parallelism, collecting all three as errors rather
// than defaulting. But a caller with a partially-filled struct can, and one
// exists — internal/admin/page_handler_test.go builds its handler with
// auth.Argon2Params{}.
//
// The consequence is worse than the timing difference the function was written
// to remove. A panic in the login handler on the "no such e-mail" path is a
// 500 where a known address gets a 200, which tells an attacker the same thing
// the timing would have, instantly and without measurement. The defence would
// have become the oracle.
func TestVerifyDummyPasswordSurvivesAZeroValueParamsStruct(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked on a zero-value Argon2Params: %v — the login "+
				"handler calls this on the unknown-address path, so this is a 500 "+
				"where a known address gets a 200", r)
		}
	}()
	VerifyDummyPassword("hunter2", Argon2Params{})
}

// And the property the function is actually for: it must cost roughly what a
// real verification costs. Asserted as an order of magnitude and not a ratio —
// a machine under load makes any tighter bound flaky, and the failure this
// guards against is not "10% faster" but "returns immediately".
func TestVerifyDummyPasswordCostsWhatAVerificationCosts(t *testing.T) {
	p := Argon2Params{Memory: 8192, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

	hash, err := HashPassword("hunter2", p)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	start := time.Now()
	if _, err := VerifyPassword("falsch", hash); err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	real := time.Since(start)

	start = time.Now()
	VerifyDummyPassword("falsch", p)
	dummy := time.Since(start)

	if dummy*10 < real {
		t.Errorf("the dummy derivation took %v against the real %v — more than ten "+
			"times faster is not a constant-time answer, it is the enumeration "+
			"oracle this function exists to close", dummy, real)
	}
}
