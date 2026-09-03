// Package sharelink signs the URLs that let someone see an unpublished page.
//
// A customer approving a draft should not need an account, and the alternative
// people reach for otherwise is publishing it "just for a minute" — which is
// how a half-finished price list ends up in a search index.
package sharelink

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// DefaultLifetime is how long a share link works.
//
// A week: long enough for a customer to get round to it over a weekend, short
// enough that a link forwarded and forgotten stops working before the page has
// changed beyond recognition.
const DefaultLifetime = 7 * 24 * time.Hour

// MaxLifetime bounds what the admin may choose.
const MaxLifetime = 90 * 24 * time.Hour

var (
	// ErrInvalid means the link is malformed or was tampered with.
	ErrInvalid = errors.New("this preview link is not valid")
	// ErrExpired means the link has run out.
	ErrExpired = errors.New("this preview link has expired")
)

// Signer issues and checks share links.
type Signer struct {
	key []byte
}

// New derives a signing key from the installation's existing secret.
//
// A distinct key rather than the secret itself, and a distinct label from the
// form guard's: a token minted for one purpose must never validate for the
// other, or a contact-form token would open an unpublished page.
func New(secret []byte) *Signer {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("holzcloud share link v1"))
	return &Signer{key: mac.Sum(nil)}
}

// Token mints a link for one page, valid until expires.
//
// The page id is in the token rather than in a separate path segment, so a
// signature cannot be lifted from one page's link and pasted onto another's.
func (s *Signer) Token(pageID int64, expires time.Time) string {
	payload := strconv.FormatInt(pageID, 10) + "-" + strconv.FormatInt(expires.UTC().Unix(), 10)
	return payload + "." + s.sign(payload)
}

// Check validates a token and returns the page it was minted for.
func (s *Signer) Check(token string, now time.Time) (pageID int64, err error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return 0, ErrInvalid
	}
	// Constant time: a comparison that stops at the first wrong byte tells an
	// attacker how much of the signature they have right.
	if !hmac.Equal([]byte(sig), []byte(s.sign(payload))) {
		return 0, ErrInvalid
	}

	idPart, expPart, ok := strings.Cut(payload, "-")
	if !ok {
		return 0, ErrInvalid
	}
	pageID, err = strconv.ParseInt(idPart, 10, 64)
	if err != nil || pageID <= 0 {
		return 0, ErrInvalid
	}
	expires, err := strconv.ParseInt(expPart, 10, 64)
	if err != nil {
		return 0, ErrInvalid
	}
	// Expiry is checked after the signature, so an expired-but-genuine link and
	// a forged one are told apart — the first deserves "ask for a new link",
	// the second does not deserve an explanation at all.
	if now.UTC().After(time.Unix(expires, 0).UTC()) {
		return 0, ErrExpired
	}
	return pageID, nil
}

func (s *Signer) sign(payload string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Path is the public address of a share link.
func Path(token string) string { return "/vorschau/" + token }
