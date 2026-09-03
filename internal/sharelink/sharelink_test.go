package sharelink

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestATokenOpensTheOnePageItWasMintedFor(t *testing.T) {
	s := New([]byte("installation secret"))
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	token := s.Token(42, now.Add(DefaultLifetime))
	id, err := s.Check(token, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if id != 42 {
		t.Errorf("page id = %d, want 42", id)
	}
}

func TestASignatureCannotBeMovedToAnotherPage(t *testing.T) {
	s := New([]byte("installation secret"))
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	token := s.Token(42, now.Add(DefaultLifetime))
	payload, sig, _ := strings.Cut(token, ".")
	_ = payload

	// The page id is inside the signed payload, so pasting the signature onto
	// another page's token must not work — otherwise one shared draft would
	// hand out every draft on the site.
	forged := "43-" + strings.SplitN(payload, "-", 2)[1] + "." + sig
	if _, err := s.Check(forged, now); !errors.Is(err, ErrInvalid) {
		t.Errorf("a signature was moved to another page: %v", err)
	}
}

func TestAnExpiryCannotBeExtended(t *testing.T) {
	s := New([]byte("installation secret"))
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	token := s.Token(42, now.Add(time.Hour))
	payload, sig, _ := strings.Cut(token, ".")
	id := strings.SplitN(payload, "-", 2)[0]

	later := now.Add(365 * 24 * time.Hour).Unix()
	stretched := id + "-" + time.Unix(later, 0).UTC().Format("20060102") + "." + sig
	if _, err := s.Check(stretched, now); !errors.Is(err, ErrInvalid) {
		t.Errorf("an expiry was rewritten: %v", err)
	}
}

func TestAnExpiredLinkIsToldApartFromAForgedOne(t *testing.T) {
	s := New([]byte("installation secret"))
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	token := s.Token(42, now.Add(time.Hour))
	// The person holding an expired link deserves "ask for a new one"; the one
	// holding a forgery deserves no explanation at all.
	if _, err := s.Check(token, now.Add(2*time.Hour)); !errors.Is(err, ErrExpired) {
		t.Errorf("err = %v, want ErrExpired", err)
	}
	if _, err := s.Check("42-99999999999.wasauchimmer", now); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestOnePurposeCannotStandInForAnother(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	secret := []byte("installation secret")

	// The application derives two signers from the same secret with different
	// labels. A token from one must be worthless to the other, or an unlock
	// cookie would open a draft.
	share := New(append([]byte("share:"), secret...))
	unlock := New(append([]byte("unlock:"), secret...))

	token := share.Token(42, now.Add(time.Hour))
	if _, err := unlock.Check(token, now); !errors.Is(err, ErrInvalid) {
		t.Errorf("a share token validated as an unlock token: %v", err)
	}
}

func TestMalformedTokensAreRefused(t *testing.T) {
	s := New([]byte("installation secret"))
	now := time.Now()

	for _, bad := range []string{
		"", ".", "keinpunkt", "42.", ".sig", "42-abc.sig", "abc-123.sig",
		"0-99999999999.sig", "-1-99999999999.sig",
	} {
		if _, err := s.Check(bad, now); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestPathIsTheAddressAVisitorGets(t *testing.T) {
	if got := Path("42-1234.sig"); got != "/vorschau/42-1234.sig" {
		t.Errorf("Path = %q", got)
	}
}
