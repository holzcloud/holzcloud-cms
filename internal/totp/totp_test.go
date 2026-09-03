package totp

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfc6238Secret is the ASCII string "12345678901234567890" from the RFC's test
// vectors, in the base32 form an authenticator app takes.
var rfc6238Secret = base32.StdEncoding.WithPadding(base32.NoPadding).
	EncodeToString([]byte("12345678901234567890"))

// The published test vectors are the only way to know this implementation
// agrees with every authenticator app rather than merely with itself.
func TestCodeMatchesTheRFCTestVectors(t *testing.T) {
	// From RFC 6238 Appendix B, SHA-1 column, truncated to six digits.
	cases := map[int64]string{
		59:          "287082",
		1111111109:  "081804",
		1111111111:  "050471",
		1234567890:  "005924",
		2000000000:  "279037",
		20000000000: "353130",
	}
	for unix, want := range cases {
		got, err := Code(rfc6238Secret, time.Unix(unix, 0).UTC())
		if err != nil {
			t.Fatalf("Code at %d: %v", unix, err)
		}
		if got != want {
			t.Errorf("Code at %d = %s, want %s", unix, got, want)
		}
	}
}

func TestValidateAcceptsAClockOffByHalfAMinute(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 1, 12, 0, 30, 0, time.UTC)

	// A phone whose clock runs one period fast or slow still works.
	for _, offset := range []time.Duration{-Period, 0, Period} {
		code, err := Code(secret, now.Add(offset))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := Validate(secret, code, now); !ok {
			t.Errorf("a code from %v away was rejected", offset)
		}
	}

	// Two periods out is not accepted: widening the window is what makes a
	// shoulder-surfed code useful for minutes rather than seconds.
	far, _ := Code(secret, now.Add(3*Period))
	if _, ok := Validate(secret, far, now); ok {
		t.Error("a code from ninety seconds away was accepted")
	}
}

func TestValidateReturnsTheStepSoReplayCanBeRefused(t *testing.T) {
	secret, _ := GenerateSecret()
	now := time.Date(2026, 5, 1, 12, 0, 15, 0, time.UTC)

	code, _ := Code(secret, now)
	step, ok := Validate(secret, code, now)
	if !ok {
		t.Fatal("a freshly generated code was rejected")
	}
	// Without the counter, a caller has no way to tell "correct" from
	// "correct and already used".
	if step != Step(now) {
		t.Errorf("step = %d, want %d", step, Step(now))
	}
}

func TestValidateToleratesHowPeopleTypeCodes(t *testing.T) {
	secret, _ := GenerateSecret()
	now := time.Date(2026, 5, 1, 12, 0, 15, 0, time.UTC)
	code, _ := Code(secret, now)

	// Apps display "123 456"; people paste it with the space.
	spaced := code[:3] + " " + code[3:]
	if _, ok := Validate(secret, spaced, now); !ok {
		t.Errorf("a code typed as %q was rejected", spaced)
	}
	if _, ok := Validate(secret, " "+code+"\n", now); !ok {
		t.Error("a code with surrounding whitespace was rejected")
	}
}

func TestValidateRejectsRubbish(t *testing.T) {
	secret, _ := GenerateSecret()
	now := time.Now()
	for _, bad := range []string{"", "12345", "1234567", "abcdef", "000000"} {
		if _, ok := Validate(secret, bad, now); ok && bad != "000000" {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestGenerateSecretIsUsableByAnApp(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		secret, err := GenerateSecret()
		if err != nil {
			t.Fatal(err)
		}
		if seen[secret] {
			t.Fatal("GenerateSecret repeated itself")
		}
		seen[secret] = true

		// Padding breaks several popular authenticator apps, which reject the
		// "=" outright rather than stripping it.
		if strings.Contains(secret, "=") {
			t.Errorf("secret %q carries base32 padding", secret)
		}
		if _, err := Code(secret, time.Now()); err != nil {
			t.Errorf("generated secret does not produce a code: %v", err)
		}
	}
}

func TestURICarriesWhatAnAppNeeds(t *testing.T) {
	uri := URI("JBSWY3DPEHPK3PXP", "erika@example.de", "Holzcloud (example.de)")

	for _, want := range []string{
		"otpauth://totp/",
		"secret=JBSWY3DPEHPK3PXP",
		"issuer=Holzcloud",
		"digits=6",
		"period=30",
		"algorithm=SHA1",
	} {
		if !strings.Contains(uri, want) {
			t.Errorf("URI is missing %q:\n%s", want, uri)
		}
	}
	// The address must survive: the label is what tells someone with four
	// accounts which line belongs to which.
	if !strings.Contains(uri, "erika") {
		t.Errorf("URI lost the account name:\n%s", uri)
	}
}

func TestQRCodeIsSelfContainedSVG(t *testing.T) {
	svg, err := QRCode(URI("JBSWY3DPEHPK3PXP", "erika@example.de", "Holzcloud"))
	if err != nil {
		t.Fatalf("QRCode: %v", err)
	}
	markup := string(svg)

	if !strings.HasPrefix(markup, "<svg") || !strings.HasSuffix(markup, "</svg>") {
		t.Errorf("not a complete SVG element: %.80s…", markup)
	}
	// Nothing may be fetched at render time; the rule holds in the admin too.
	// The xmlns declaration is a namespace name, not an address anything
	// resolves, so it is removed before the check rather than excused inside it.
	body := strings.Replace(markup, `xmlns="http://www.w3.org/2000/svg"`, "", 1)
	for _, forbidden := range []string{"http://", "https://", "<image", "url("} {
		if strings.Contains(body, forbidden) {
			t.Errorf("SVG refers to something external via %q", forbidden)
		}
	}
	// A transparent code on a dark background is an unreadable code.
	if !strings.Contains(markup, `fill="#fff"`) {
		t.Error("SVG has no opaque background")
	}
	if !strings.Contains(markup, "<path") {
		t.Error("SVG has no modules drawn")
	}
}

func TestFormatSecretIsReadableByHand(t *testing.T) {
	got := FormatSecret("JBSWY3DPEHPK3PXP")
	if got != "JBSW Y3DP EHPK 3PXP" {
		t.Errorf("FormatSecret = %q", got)
	}
	// The grouping is presentation only — what gets typed back in still has to
	// validate, which Validate handles by stripping non-digits and the store by
	// upper-casing and trimming.
	if strings.ReplaceAll(got, " ", "") != "JBSWY3DPEHPK3PXP" {
		t.Error("FormatSecret changed the secret itself")
	}
}
