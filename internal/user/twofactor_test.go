package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/totp"
)

// newTestUserStore is newTestStore with the seeded account discarded: these
// tests create their own accounts with the roles they need.
func newTestUserStore(t *testing.T) (*Store, int64) {
	return newTestStore(t)
}

// seedUser creates an account and returns its id.
func seedUser(t *testing.T, s *Store, email, role string) int64 {
	t.Helper()
	id, err := s.Create(context.Background(), "Test", email, "passwort123", role)
	if err != nil {
		t.Fatalf("Create %s: %v", email, err)
	}
	return id
}

// advance moves the store's clock, so a sign-in "a minute later" can be tested
// without the test taking a minute.
func advance(s *Store, d time.Duration) {
	base := time.Now().Add(d)
	s.now = func() time.Time { return base }
}

// enrol runs a full setup and returns the secret and the recovery codes.
func enrol(t *testing.T, s *Store, userID int64) (string, []string) {
	t.Helper()
	secret, err := s.BeginTwoFactor(context.Background(), userID)
	if err != nil {
		t.Fatalf("BeginTwoFactor: %v", err)
	}
	code, err := totp.Code(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	codes, err := s.ConfirmTwoFactor(context.Background(), userID, code)
	if err != nil {
		t.Fatalf("ConfirmTwoFactor: %v", err)
	}
	return secret, codes
}

func TestEnrolmentNeedsAWorkingCode(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	if _, err := s.BeginTwoFactor(ctx, id); err != nil {
		t.Fatalf("BeginTwoFactor: %v", err)
	}
	// A wrong code must not switch it on: an account that enabled two-factor
	// without proving the app works has locked itself out.
	if _, err := s.ConfirmTwoFactor(ctx, id, "000000"); !errors.Is(err, ErrBadCode) {
		t.Fatalf("err = %v, want ErrBadCode", err)
	}
	tf, _ := s.GetTwoFactor(ctx, id)
	if tf.Enabled() {
		t.Fatal("a wrong code enabled the second factor")
	}
}

func TestBeginDoesNotBreakAWorkingFactor(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	secret, _ := enrol(t, s, id)

	// Someone opens the setup page again — to move to a new phone, or by
	// misclicking. The old app has to keep working until the new one proves
	// itself, or a stray click locks an administrator out of the installation.
	if _, err := s.BeginTwoFactor(ctx, id); err != nil {
		t.Fatalf("BeginTwoFactor: %v", err)
	}

	tf, _ := s.GetTwoFactor(ctx, id)
	if !tf.Enabled() {
		t.Fatal("starting a new enrolment switched the working one off")
	}
	later := time.Now().Add(2 * totp.Period)
	advance(s, 2*totp.Period)
	code, _ := totp.Code(secret, later)
	if err := s.VerifyCode(ctx, id, code); err != nil {
		t.Errorf("the old authenticator stopped working: %v", err)
	}
}

func TestConfirmingANewDeviceReplacesTheOldOne(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	oldSecret, oldCodes := enrol(t, s, id)
	newSecret, newCodes := enrol(t, s, id)

	if oldSecret == newSecret {
		t.Fatal("the second enrolment reused the first secret")
	}
	// The old phone must stop working the moment the new one is confirmed.
	later := time.Now().Add(4 * totp.Period)
	advance(s, 4*totp.Period)
	oldCode, _ := totp.Code(oldSecret, later)
	if err := s.VerifyCode(ctx, id, oldCode); !errors.Is(err, ErrBadCode) {
		t.Errorf("the replaced authenticator still works: %v", err)
	}
	// And so must the recovery codes printed against it.
	if err := s.UseRecoveryCode(ctx, id, oldCodes[0]); !errors.Is(err, ErrBadCode) {
		t.Errorf("a recovery code from the old setup still works: %v", err)
	}
	if err := s.UseRecoveryCode(ctx, id, newCodes[0]); err != nil {
		t.Errorf("a fresh recovery code does not work: %v", err)
	}
}

func TestTheSameCodeCannotBeUsedTwice(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	secret, _ := enrol(t, s, id)
	// A code lives for thirty seconds — long enough for someone who read the
	// digits over a shoulder to type them in after their owner did.
	later := time.Now().Add(2 * totp.Period)
	advance(s, 2*totp.Period)
	code, _ := totp.Code(secret, later)
	if err := s.VerifyCode(ctx, id, code); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := s.VerifyCode(ctx, id, code); !errors.Is(err, ErrBadCode) {
		t.Errorf("the same code was accepted twice: %v", err)
	}
}

func TestConfirmationCodeCannotBeReplayedAtSignIn(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	secret, err := s.BeginTwoFactor(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := totp.Code(secret, time.Now())
	if _, err := s.ConfirmTwoFactor(ctx, id, code); err != nil {
		t.Fatalf("ConfirmTwoFactor: %v", err)
	}

	// Confirmation records the step it consumed, so the very code that turned
	// the factor on cannot be replayed to sign in with it.
	if err := s.VerifyCode(ctx, id, code); !errors.Is(err, ErrBadCode) {
		t.Errorf("the setup code worked as a sign-in code: %v", err)
	}
}

func TestRecoveryCodesAreSingleUse(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	_, codes := enrol(t, s, id)
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("got %d recovery codes, want %d", len(codes), RecoveryCodeCount)
	}

	// People type them off paper, so case and the grouping dash must not matter.
	if err := s.UseRecoveryCode(ctx, id, "  "+codes[0]+"  "); err != nil {
		t.Fatalf("a code with surrounding whitespace was refused: %v", err)
	}
	if err := s.UseRecoveryCode(ctx, id, codes[0]); !errors.Is(err, ErrBadCode) {
		t.Errorf("a recovery code worked twice: %v", err)
	}

	tf, _ := s.GetTwoFactor(ctx, id)
	if tf.RecoveryLeft != RecoveryCodeCount-1 {
		t.Errorf("%d codes left, want %d", tf.RecoveryLeft, RecoveryCodeCount-1)
	}
	if tf.RecoveryTotal != RecoveryCodeCount {
		t.Errorf("total = %d, want %d", tf.RecoveryTotal, RecoveryCodeCount)
	}
}

func TestRecoveryCodesAreNotStoredInTheClear(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	_, codes := enrol(t, s, id)

	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT code_hash FROM user_recovery_codes WHERE user_id = $1`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			t.Fatal(err)
		}
		for _, code := range codes {
			// A stolen copy of the database file must not yield working codes.
			if stored == code {
				t.Fatalf("recovery code %q is stored in the clear", code)
			}
		}
	}
}

func TestDisableRemovesEverything(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	secret, codes := enrol(t, s, id)
	if err := s.DisableTwoFactor(ctx, id); err != nil {
		t.Fatalf("DisableTwoFactor: %v", err)
	}

	tf, _ := s.GetTwoFactor(ctx, id)
	if tf.Enabled() || tf.Secret != "" || tf.PendingSecret != "" {
		t.Errorf("state survived the reset: %+v", tf)
	}
	if tf.RecoveryTotal != 0 {
		t.Errorf("%d recovery codes survived", tf.RecoveryTotal)
	}

	later := time.Now().Add(2 * totp.Period)
	advance(s, 2*totp.Period)
	code, _ := totp.Code(secret, later)
	if err := s.VerifyCode(ctx, id, code); !errors.Is(err, ErrNoSecondFactor) {
		t.Errorf("err = %v, want ErrNoSecondFactor", err)
	}
	if err := s.UseRecoveryCode(ctx, id, codes[0]); !errors.Is(err, ErrBadCode) {
		t.Errorf("a recovery code survived the reset: %v", err)
	}
}

func TestOneAccountCannotSpendAnothersRecoveryCode(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	first := seedUser(t, s, "eins@example.de", RoleAdmin)
	second := seedUser(t, s, "zwei@example.de", RoleAdmin)

	_, codes := enrol(t, s, first)
	enrol(t, s, second)

	if err := s.UseRecoveryCode(ctx, second, codes[0]); !errors.Is(err, ErrBadCode) {
		t.Errorf("one account's recovery code worked for another: %v", err)
	}
	// And it is still unspent for its owner.
	if err := s.UseRecoveryCode(ctx, first, codes[0]); err != nil {
		t.Errorf("the failed attempt consumed the code anyway: %v", err)
	}
}

func TestRegenerateVoidsTheOldList(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	_, old := enrol(t, s, id)
	fresh, err := s.RegenerateRecoveryCodes(ctx, id)
	if err != nil {
		t.Fatalf("RegenerateRecoveryCodes: %v", err)
	}

	if err := s.UseRecoveryCode(ctx, id, old[0]); !errors.Is(err, ErrBadCode) {
		t.Errorf("an old code still works after regenerating: %v", err)
	}
	if err := s.UseRecoveryCode(ctx, id, fresh[0]); err != nil {
		t.Errorf("a new code does not work: %v", err)
	}
	// The authenticator itself is untouched — only the paper list changed.
	tf, _ := s.GetTwoFactor(ctx, id)
	if !tf.Enabled() {
		t.Error("regenerating the codes switched the second factor off")
	}
}

// The setup page is rendered again on every reload and after every mistyped
// code. A new secret each time would invalidate the QR the user just scanned
// and then reject their app's correct answer.
func TestEnsurePendingSecretSurvivesAReload(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	first, err := s.EnsurePendingSecret(ctx, id)
	if err != nil {
		t.Fatalf("EnsurePendingSecret: %v", err)
	}
	second, err := s.EnsurePendingSecret(ctx, id)
	if err != nil {
		t.Fatalf("second EnsurePendingSecret: %v", err)
	}
	if first != second {
		t.Fatal("reloading the setup page changed the secret behind the QR code")
	}

	// A code from the scanned QR still confirms after a reload.
	code, _ := totp.Code(first, time.Now())
	if _, err := s.ConfirmTwoFactor(ctx, id, code); err != nil {
		t.Errorf("the scanned secret stopped working across a reload: %v", err)
	}
}

// Starting over deliberately is a different action from opening the page.
func TestBeginTwoFactorReplacesThePendingSecret(t *testing.T) {
	s, _ := newTestUserStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "admin@example.de", RoleAdmin)

	first, _ := s.EnsurePendingSecret(ctx, id)
	second, err := s.BeginTwoFactor(ctx, id)
	if err != nil {
		t.Fatalf("BeginTwoFactor: %v", err)
	}
	if first == second {
		t.Fatal("starting over reused the old secret")
	}
	old, _ := totp.Code(first, time.Now())
	if _, err := s.ConfirmTwoFactor(ctx, id, old); !errors.Is(err, ErrBadCode) {
		t.Errorf("the discarded enrolment still confirms: %v", err)
	}
}
