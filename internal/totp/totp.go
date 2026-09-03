// Package totp implements the time-based one-time passwords of RFC 6238, the
// six digits an authenticator app shows.
//
// Written out rather than pulled in: the algorithm is HMAC, a counter and a
// modulo, and the interesting parts of getting it right — the replay window,
// the constant-time comparison, the clock skew — are decisions this application
// has to make anyway.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Period is the length of one code's life. Thirty seconds is what every
// authenticator app assumes and none of them let the user change.
const Period = 30 * time.Second

// Digits is the length of a code.
const Digits = 6

// Skew is how many periods either side of now are accepted.
//
// One period, so a phone whose clock is up to thirty seconds off still works
// and a code stays valid while the user finishes typing it. Wider would extend
// the window in which a code read over someone's shoulder is still good.
const Skew = 1

// secretBytes is the length of a generated shared secret.
//
// Twenty bytes is what RFC 4226 requires as a minimum and what the apps expect;
// longer secrets produce a QR code that some readers struggle with.
const secretBytes = 20

// encoding is base32 without padding, which is what otpauth:// URIs use — a
// secret with "=" on the end is rejected by several popular apps.
var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret returns a new shared secret in the encoding the apps expect.
func GenerateSecret() (string, error) {
	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return encoding.EncodeToString(buf), nil
}

// Code returns the six digits valid at a given moment.
func Code(secret string, at time.Time) (string, error) {
	return codeForStep(secret, Step(at))
}

// Step is the counter a moment falls into.
//
// It is exported because the counter of an accepted code has to be stored: the
// same code stays valid for thirty seconds, and without remembering it, anyone
// who reads it over a shoulder can sign in for the rest of that window.
func Step(at time.Time) int64 {
	return at.UTC().Unix() / int64(Period/time.Second)
}

func codeForStep(secret string, step int64) (string, error) {
	key, err := encoding.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("decode totp secret: %w", err)
	}
	if len(key) == 0 {
		return "", fmt.Errorf("totp secret is empty")
	}

	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))

	// SHA-1, because that is what RFC 6238 specifies and what the apps
	// implement. Its weakness is collisions, which an HMAC does not depend on.
	mac := hmac.New(sha1.New, key)
	mac.Write(counter[:])
	sum := mac.Sum(nil)

	// Dynamic truncation, RFC 4226 §5.4: the low nibble of the last byte picks
	// where in the digest to read from.
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	return fmt.Sprintf("%0*d", Digits, value%pow10(Digits)), nil
}

func pow10(n int) uint32 {
	out := uint32(1)
	for i := 0; i < n; i++ {
		out *= 10
	}
	return out
}

// Validate checks a submitted code and returns the counter it belongs to.
//
// The caller must refuse a step it has already accepted. Returning the step
// rather than a bare bool is what makes that possible — a validator that only
// says yes or no cannot be made replay-proof by its caller.
func Validate(secret, submitted string, at time.Time) (step int64, ok bool) {
	submitted = normalize(submitted)
	if len(submitted) != Digits {
		return 0, false
	}

	now := Step(at)
	for delta := int64(-Skew); delta <= Skew; delta++ {
		candidate, err := codeForStep(secret, now+delta)
		if err != nil {
			return 0, false
		}
		// Constant time: a comparison that stops at the first wrong digit
		// leaks, over enough attempts, which digits were right.
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(submitted)) == 1 {
			return now + delta, true
		}
	}
	return 0, false
}

// normalize strips what people type in when reading digits off a screen.
func normalize(code string) string {
	var b strings.Builder
	for _, r := range code {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// URI builds the otpauth:// URL an authenticator app reads from a QR code.
//
// The issuer appears twice on purpose: once in the label, which is what old
// apps display, and once as a parameter, which is what current ones read.
func URI(secret, account, issuer string) string {
	label := issuer + ":" + account
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", Digits))
	q.Set("period", fmt.Sprintf("%d", int(Period/time.Second)))

	return "otpauth://totp/" + url.PathEscape(label) + "?" + q.Encode()
}

// FormatSecret groups the secret into blocks of four for typing it in by hand.
//
// Manual entry is the fallback when the camera will not focus, and a
// thirty-two-character run with no breaks is where people lose their place.
func FormatSecret(secret string) string {
	var b strings.Builder
	for i, r := range secret {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
