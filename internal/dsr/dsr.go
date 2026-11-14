// Package dsr keeps the requests people make about their personal data.
//
// A request is one row of privacy_requests: who asked (as much as they chose
// to say), what kind of request it is, in which language the page was, whether
// the browser sent the Global Privacy Control signal, when it arrived and when
// the operator answered it. Nothing else — no IP address and no user agent,
// because the request is used only to record and answer it (CCPA regs
// § 7025(c)(2)).
//
// Phase 33.1 writes one kind, KindOptOut, from the "Do Not Sell or Share"
// page. Phase 33.2 adds the data requests (access, correction, erasure) to the
// same table and this package (33.2 D-09); the kind is validated here rather
// than by a CHECK list in the schema so that adding one needs no table
// rebuild.
//
// This package never deletes. The record of a US opt-out has to be kept for
// at least 24 months (§ 7101), and retention — deciding when a row may go —
// is Phase 33.2's (D-14). A delete method here would be the one way to break
// that by accident, so there is none.
package dsr

import (
	"errors"
	"net/mail"
	"strings"
)

// ValidEmail reports whether s is one bare e-mail address: what net/mail
// reads as a single address with no display name, no comment, no quoting, and
// none of the characters that mean something in a mailto: link or a header
// (? & # % / , ; < > and white space).
//
// Strict on purpose. The address is what the operator answers the request
// at, and the admin list makes it a mailto: link: an address that carries
// "?bcc=…" is a link that sends a copy wherever the stranger said (WR-02 of
// the 33.1 review). A real address never needs any of these characters, and
// the visitor who mistyped one is told and can send again.
//
// The character list is what net/mail lets through and a link must not
// have: ? & # % / are ordinary characters of an address's local part there.
// What net/mail refuses by itself is not checked again (IN-05 of the second
// 33.1 review): an empty string, a second @, an empty local part or domain,
// and a "mailto:" in front — the colon is no character of an address, so
// that is not one either. TestAnAddressIsOneAddress holds every one of them.
func ValidEmail(s string) bool {
	if strings.ContainsAny(s, "?&#%/,;<>\"()[]\\ \t\r\n") {
		return false
	}
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Name == "" && addr.Address == s
}

// KindOptOut is a request not to sell or share personal information.
const KindOptOut = "optout"

// The longest values the table accepts, in characters. The schema enforces
// the same numbers.
const (
	MaxName  = 200
	MaxEmail = 254
	MaxNote  = 2000
	// maxLocale is a BCP 47 tag's practical length.
	maxLocale = 35
)

// kinds are the kinds this program writes. Phase 33.2 adds its own here.
var kinds = map[string]bool{
	KindOptOut: true,
}

var (
	// ErrEmpty means the request says nothing: no name, no address and no
	// signal from the browser.
	ErrEmpty = errors.New("dsr: the request names nobody")
	// ErrTooLong means a value is longer than the table allows.
	ErrTooLong = errors.New("dsr: a value is too long")
	// ErrBadKind means the kind is not one this program knows.
	ErrBadKind = errors.New("dsr: unknown kind of request")
	// ErrNotFound means there is no such request on this website.
	ErrNotFound = errors.New("dsr: no such request")
	// ErrBadEmail means the e-mail field holds something other than one bare
	// address (ValidEmail).
	ErrBadEmail = errors.New("dsr: not one e-mail address")
	// ErrNoTime means a restored request does not say when it arrived.
	ErrNoTime = errors.New("dsr: the request has no arrival time")
	// ErrFutureTime means a restored request arrived in the future
	// (Restore).
	ErrFutureTime = errors.New("dsr: the request arrives in the future")
	// ErrAnswerTime means a restored request was answered in the future, or
	// before it arrived (Restore).
	ErrAnswerTime = errors.New("dsr: the request is answered in the future or before it arrived")
	// ErrLimit means the website already received as many requests of the
	// kind as AddWithin allows in its window, and this one was not stored.
	ErrLimit = errors.New("dsr: too many requests in the window")
)
