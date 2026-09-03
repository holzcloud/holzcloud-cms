package i18n

import (
	"strings"
	"unicode"
)

// Language tags, and the one thing that makes them worth having: a region.
//
// "de" is German, "de-CH" is German as it is written in Switzerland. That is
// not decoration. Swiss German has no ß at all, and it quotes with «Anführung»
// where Germany writes „Anführung“ — a Swiss administration that spells
// "Grösse" with a ß looks, to the person reading it, like a foreign product.
//
// A regional fassung is a catalogue like any other, with one rule on top:
// **what it does not say, its base language says.** de-CH carries the three
// dozen sentences that are actually spelled differently and nothing else; the
// remaining nine hundred come from German. That is what keeps a variant
// maintainable — a new sentence in the source appears in every regional
// fassung the moment it is translated once, and nobody has to keep two nearly
// identical files in step by hand.

// Normalise cleans a tag as somebody typed it or named a file: "DE_ch" and
// "de-ch" both become "de-CH".
func Normalise(tag string) string {
	tag = strings.TrimSpace(strings.ReplaceAll(tag, "_", "-"))
	base, region, hasRegion := strings.Cut(tag, "-")
	base = strings.ToLower(base)
	if !hasRegion {
		return base
	}
	return base + "-" + strings.ToUpper(region)
}

// ValidTag reports whether a tag is one this can work with: two or three
// letters, optionally a two-letter region after a hyphen.
//
// A small subset of BCP 47 on purpose. It is enough for every language a
// website is written in and every regional fassung of one, and narrow enough
// that a tag can be a file name and an address segment without escaping.
func ValidTag(tag string) bool {
	base, region, hasRegion := strings.Cut(tag, "-")
	if len(base) < 2 || len(base) > 3 || !onlyLetters(base) || base != strings.ToLower(base) {
		return false
	}
	if !hasRegion {
		return true
	}
	return len(region) == 2 && onlyLetters(region) && region == strings.ToUpper(region)
}

func onlyLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) || r > unicode.MaxASCII {
			return false
		}
	}
	return s != ""
}

// Base is the language a regional fassung falls back on: "de-CH" is German,
// "de" is nothing. Empty means the tag stands on its own.
func Base(tag string) string {
	base, _, hasRegion := strings.Cut(tag, "-")
	if !hasRegion {
		return ""
	}
	return strings.ToLower(base)
}

// Region is the region of a tag, "CH" for de-CH and empty for de.
func Region(tag string) string {
	_, region, _ := strings.Cut(tag, "-")
	return strings.ToUpper(region)
}

// chain is the catalogues to ask, in order: the tag itself, then what it falls
// back on. German is not in it — a lookup that finds nothing returns the German
// it was given.
func chain(tag string) []string {
	if base := Base(tag); base != "" && base != Source {
		return []string{tag, base}
	}
	return []string{tag}
}
