// Package value keeps the values of a website: a named line of text that a page
// writes as [[wert:key]] inside a sentence, a heading, a list item or a link
// address, and that the public side puts in its place when the page is read.
//
// The case that asked for it is a version number. holzcloud.ch names the
// released version of each project in a dozen sentences and link addresses, and
// every release rewrote those pages one by one. A value is kept once; every
// sentence that names it reads it, and changing it changes the next response of
// every page without saving any of them (VAL-04).
//
// # What a value is
//
// One line of plain text (D-05 of phase 33). Never Markdown, never HTML: it is
// escaped wherever it is put — as text in a paragraph, percent-encoded inside a
// link address — so an editor's angle bracket is an angle bracket and not an
// element. CleanText is the one place that says what a text may hold, and every
// Store method that writes calls it, so no caller can store what the screen
// would refuse.
//
// One text per website, for every language (D-04). Words that differ by
// language stay the page's own text or the wording catalogue's.
//
// # The fourth marker resolved late
//
// The snippet, the album gallery and the late-resolved word already stand in
// stored HTML as markers and are replaced at delivery. A value is the fourth,
// and the first whose text is written by an editor and may stand inside a link
// address — which is why Resolve tokenizes instead of replacing over the whole
// body the way block.ResolveWords does.
//
// # Isolation
//
// A value belongs to exactly one website (D-03). Every statement in store.go
// carries the website id, and Resolve is handed one website's texts and nothing
// else; a key of website A never resolves on website B.
package value

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Prefix is what every value marker starts with, and the one literal the
// delivery guard asks about.
//
// One literal and not two: goldmark percent-encodes the brackets of a marker
// inside a Markdown link destination, and guarding on both spellings costs
// three and a half times what one costs on every page view (33-RESEARCH.md F2).
// So page.RenderMarkdown restores the encoded spelling once, at save, through
// marker.RestoreEncodedValueMarkers, and delivery only ever looks for this.
const Prefix = "[[wert:"

// MaxKeyBytes is the longest key, the same limit a snippet key has.
const MaxKeyBytes = 60

// MaxTextBytes is the longest text, in bytes after trimming.
//
// Bytes and not characters, like every other limit here: 400 bytes are about
// 200 characters even when every one of them is an umlaut, which is far more
// than a version number or a phone number needs and short enough that a value
// stays a value and does not become a paragraph.
const MaxTextBytes = 400

// pattern matches a complete marker. The key alphabet is the snippet key's: a-z
// and 0-9 first, then also - and _ (CheckKey refuses one that ends in either).
var pattern = regexp.MustCompile(`\[\[wert:([a-z0-9][a-z0-9_-]*)\]\]`)

// keyPattern is the whole of a valid key.
var keyPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$`)

// Sentinel errors. The screen and the assistant map each to a sentence of
// their own; the store returns them and nothing else for a refused input.
var (
	ErrBadKey      = errors.New("value key must be a-z, 0-9, - and _ (not first or last), at most 60 bytes")
	ErrTextEmpty   = errors.New("value text is empty")
	ErrTextTooLong = errors.New("value text is longer than 400 bytes")
	ErrTextInvalid = errors.New("value text must be one line of valid UTF-8 without control characters")
	ErrTextMarker  = errors.New("value text must not contain [[")
	ErrKeyTaken    = errors.New("another value already uses that key")
	ErrNotFound    = errors.New("value not found")
)

// InUseError refuses to delete a value that a page or a snippet still names.
//
// Users are the names of those pages and snippets, ready to show, in the order
// Report gives them. The shape is media.InUseError's, so the screen and the
// assistant answer a value in use the way they answer a picture in use (D-02).
type InUseError struct {
	Users []string
}

func (e *InUseError) Error() string {
	return fmt.Sprintf("value is used by %d page(s) or snippet(s)", len(e.Users))
}

// Marker is what a page writes for a value.
//
// Built from the prefix rather than spelled out, so the writer and the reader
// cannot disagree — the rule block.Word follows.
func Marker(key string) string { return Prefix + key + "]]" }

// Has reports whether a document carries any value marker.
//
// The cheap question, asked before anything else: a page without a marker pays
// one strings.Contains and no allocation, the same as the word guard
// (block.HasWordMarker) it sits beside.
func Has(doc string) bool { return strings.Contains(doc, Prefix) }

// Keys returns the distinct keys of the complete markers in s, in the order
// they first appear. It reads Markdown, a blocks document and HTML alike,
// because all three carry the literal marker.
func Keys(s string) []string {
	if !Has(s) {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range pattern.FindAllStringSubmatch(s, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// CheckKey refuses a key that is not a-z and 0-9 with - and _ inside, or that
// is longer than MaxKeyBytes. internal/admin keeps its own validKey for
// snippets; the rule is the same and this is the one values use.
func CheckKey(key string) error {
	if key == "" || len(key) > MaxKeyBytes || !keyPattern.MatchString(key) {
		return ErrBadKey
	}
	return nil
}

// CleanText trims a text and refuses one a value may not hold.
//
// No line break and no tab, because a value stands inside a sentence and a
// link address, and neither has room for one. No "[[", so a value can never
// carry marker syntax of its own and the public page can never print one.
func CleanText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ErrTextEmpty
	}
	if len(text) > MaxTextBytes {
		return "", ErrTextTooLong
	}
	if !utf8.ValidString(text) {
		return "", ErrTextInvalid
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return "", ErrTextInvalid
		}
	}
	if strings.Contains(text, "[[") {
		return "", ErrTextMarker
	}
	return text, nil
}
