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
// page. Phase 33.2 adds the data requests of D-01 — access, correction and
// erasure — to the same table (33.2 D-09): a visitor sends one through the
// [[datenanfrage]] form, or the operator enters one that arrived by e-mail,
// letter or telephone, and records how it was answered. The kind is
// validated here rather than by a CHECK list in the schema so that adding
// one needs no table rebuild.
//
// Since Phase 33.2 the package also finds a person's data across the stores
// of one website (Service, source.go). It does so through one list of
// sources that every feature of the phase walks — the search, the export,
// erasure, correction, retention and the record of processing — so a store
// added later is covered by all of them at once. That does not change the
// rule below: Store still has no way to delete a privacy request.
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

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
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

// The data requests of Phase 33.2 (D-01): a person asks for a copy of their
// data, for it to be corrected, or for it to be erased.
const (
	KindAccess     = "access"
	KindCorrection = "correction"
	KindErasure    = "erasure"
)

// DataKinds are the kinds of data request, in the order a form offers them.
var DataKinds = []string{KindAccess, KindCorrection, KindErasure}

// kindNames are the names of the data request kinds, catalogue keys.
var kindNames = map[string]string{
	KindAccess:     i18n.N("Access"),
	KindCorrection: i18n.N("Correction"),
	KindErasure:    i18n.N("Erasure"),
}

// KindName is the name of a kind of request, a catalogue key; an unknown
// kind is its own name.
func KindName(kind string) string {
	if name, ok := kindNames[kind]; ok {
		return name
	}
	return kind
}

// IsDataKind reports whether kind is one of the data requests, and not an
// opt-out.
func IsDataKind(kind string) bool {
	return kind == KindAccess || kind == KindCorrection || kind == KindErasure
}

// The manners in which a request is made (CCPA regs § 7101(b)): through a
// form of the website, or by e-mail, letter, telephone or otherwise, entered
// by the operator. The schema checks the same list.
const (
	MannerForm  = "form"
	MannerEmail = "email"
	MannerPost  = "post"
	MannerPhone = "phone"
	MannerOther = "other"
)

// manners are the values the manner column takes.
var manners = map[string]bool{
	MannerForm: true, MannerEmail: true, MannerPost: true, MannerPhone: true, MannerOther: true,
}

// The answers a data request gets (§ 7101(b), "the nature of the response").
// An empty outcome means nobody has answered yet. The schema checks the same
// list.
const (
	OutcomeGranted = "granted"
	OutcomePartly  = "partly"
	OutcomeRefused = "refused"
)

// outcomes are the answers Answer records.
var outcomes = map[string]bool{OutcomeGranted: true, OutcomePartly: true, OutcomeRefused: true}

// Outcomes are the answers, in the order a form offers them.
var Outcomes = []string{OutcomeGranted, OutcomePartly, OutcomeRefused}

// ReceivedManners are the manners an operator enters a request with: every
// one but the form, which writes its own.
var ReceivedManners = []string{MannerEmail, MannerPost, MannerPhone, MannerOther}

// mannerNames and outcomeNames are the screen's names, catalogue keys. A
// letter is "Letter" and not "Post", which the catalogue already holds for a
// blog post.
var (
	mannerNames = map[string]string{
		MannerForm:  i18n.N("Form"),
		MannerEmail: i18n.N("E-mail"),
		MannerPost:  i18n.N("Letter"),
		MannerPhone: i18n.N("Phone"),
		MannerOther: i18n.N("Other"),
	}
	outcomeNames = map[string]string{
		OutcomeGranted: i18n.N("Granted"),
		OutcomePartly:  i18n.N("Partly granted"),
		OutcomeRefused: i18n.N("Refused"),
	}
)

// MannerName is the name of a manner, a catalogue key; an unknown manner is
// its own name.
func MannerName(manner string) string {
	if name, ok := mannerNames[manner]; ok {
		return name
	}
	return manner
}

// OutcomeName is the name of an outcome, a catalogue key; an unknown outcome
// is its own name.
func OutcomeName(outcome string) string {
	if name, ok := outcomeNames[outcome]; ok {
		return name
	}
	return outcome
}

// family is the kinds a form's hourly ceiling and mail cap count together:
// the data request form takes all three data kinds, so a ceiling per kind
// would be three ceilings; the opt-out form takes opt-outs. One form flooded
// does not close the other.
func family(kind string) []string {
	if IsDataKind(kind) {
		return DataKinds
	}
	return []string{kind}
}

// The longest values the table accepts, in characters. The schema enforces
// the same numbers.
const (
	MaxName  = 200
	MaxEmail = 254
	MaxNote  = 2000
	// MaxBasis is the longest basis of an answer.
	MaxBasis = 2000
	// maxLocale is a BCP 47 tag's practical length.
	maxLocale = 35
)

// kinds are the kinds this program writes. Phase 33.2 adds its own here.
var kinds = map[string]bool{
	KindOptOut:     true,
	KindAccess:     true,
	KindCorrection: true,
	KindErasure:    true,
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
	// ErrNeedsEmail means a data request names no address: the operator
	// answers it there, and has nothing else to find the person's data by.
	ErrNeedsEmail = errors.New("dsr: a data request needs an e-mail address")
	// ErrBadManner means the manner is not one the record knows, or a
	// request entered by hand claims to have come through the form.
	ErrBadManner = errors.New("dsr: unknown manner of request")
	// ErrBadOutcome means the answer is not granted, partly or refused.
	ErrBadOutcome = errors.New("dsr: unknown outcome")
	// ErrAnswered means the request already has its answer, and the record
	// keeps the first one.
	ErrAnswered = errors.New("dsr: the request is already answered")
)
