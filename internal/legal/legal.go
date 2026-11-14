// Package legal is the set of legal frameworks a website follows.
//
// A framework here is a body of law the operator says applies to one website:
// the EU's GDPR with the ePrivacy rules, Switzerland's revised FADP, the UK's
// GDPR with PECR, and the privacy laws of the US states. Which of them a
// website follows decides which sections its privacy page carries, what its
// imprint asks for, and whether the US signals exist at all — the "Do Not Sell
// or Share" page and the honouring of Global Privacy Control. The set belongs
// to exactly one website (D-04).
//
// The ids are deliberately not language tags, though two of them look like
// one. "uk" is Ukrainian to internal/locale, so a Framework is never handed to
// a locale helper: what it is called on a screen comes from its own label in
// the catalogue, not from a language name.
//
// The Global Privacy Control signal is read per request and nothing more. It
// is never stored and never logged: recording which visitor sent it would be a
// processing of personal data with no purpose, under exactly the laws that
// make the signal matter.
package legal

import (
	"net/http"
	"strings"
)

// Framework is one body of law a website can follow.
type Framework string

// The four frameworks, by their stored id.
const (
	EU Framework = "eu"
	CH Framework = "ch"
	UK Framework = "uk"
	US Framework = "us"
)

// All lists every framework in the one fixed order the canonical form uses.
func All() []Framework { return []Framework{EU, CH, UK, US} }

// Set is a website's choice of frameworks. The zero value is the empty set,
// which is allowed (D-16): the launch checklist reports it rather than the
// store refusing it. A Set is comparable, so two of them compare with ==.
type Set struct{ eu, ch, uk, us bool }

// ParseSet reads a set from what a person or a file wrote: commas or white
// space between the ids, any case, any spacing. Anything that is not one of
// the four ids drops out silently — the caller stores the canonical String,
// so the reader of a stored set never meets a spelling it does not know.
func ParseSet(raw string) Set {
	var s Set
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		switch Framework(strings.ToLower(strings.TrimSpace(part))) {
		case EU:
			s.eu = true
		case CH:
			s.ch = true
		case UK:
			s.uk = true
		case US:
			s.us = true
		}
	}
	return s
}

// String is the canonical form, as stored: the ids in the order of All,
// separated by commas without spaces, and "" for the empty set. The CHECK of
// migration 00063 accepts exactly the sixteen strings this can produce.
func (s Set) String() string {
	return strings.Join(frameworkStrings(s.List()), ",")
}

// Has reports whether the set contains f.
func (s Set) Has(f Framework) bool {
	switch f {
	case EU:
		return s.eu
	case CH:
		return s.ch
	case UK:
		return s.uk
	case US:
		return s.us
	}
	return false
}

// List is the frameworks of the set in the order of All.
func (s Set) List() []Framework {
	var out []Framework
	for _, f := range All() {
		if s.Has(f) {
			out = append(out, f)
		}
	}
	return out
}

// Empty reports whether no framework is chosen.
func (s Set) Empty() bool { return s == Set{} }

// Default is the set every website starts with: EU and Switzerland, because
// that is what the privacy template of Phase 30 was written for (D-05).
func Default() Set { return Set{eu: true, ch: true} }

// GPCHeader is the request header that carries the Global Privacy Control
// signal.
const GPCHeader = "Sec-GPC"

// GPC reports whether the request carries the Global Privacy Control signal.
//
// The W3C specification is strict about the value: a server must ignore the
// header "unless the value is exactly the character '1'", and a request that
// carries the header more than once counts as sending the signal when any one
// of them is "1". Surrounding white space is not part of a header value, so it
// is trimmed; "true", "yes" or "11" are not the signal.
func GPC(h http.Header) bool {
	for _, v := range h.Values(GPCHeader) {
		if strings.TrimSpace(v) == "1" {
			return true
		}
	}
	return false
}

func frameworkStrings(fs []Framework) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = string(f)
	}
	return out
}
