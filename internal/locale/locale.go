// Package locale is what a website's languages are made of.
//
// A language is a tag like "de", "fr-CH" or "rm". This package validates them,
// gives them a name a person recognises, and works out the address prefix. It
// deliberately does not translate anything: what is translated is content, and
// content lives in the database.
//
// One rule runs through everything here: the main language has no prefix.
// Switching a second language on must not move a single existing address.
package locale

import (
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"strings"
	"unicode"
)

// MaxExtra bounds how many additional languages a website may have.
//
// Not a technical limit. Every language multiplies the pages, the menus and the
// work; a website with a dozen is a website nobody keeps current.
const MaxExtra = 8

// Default is the language a website has when nothing is set.
const Default = "de"

// Valid reports whether a tag is one this can work with.
//
// A small subset of BCP 47: two or three letters, optionally a region after a
// hyphen. Enough for every language a website is written in, and narrow enough
// that the tag can go into an address without escaping.
//
// The same rule the administration's own languages follow, and deliberately the
// same code: a tag that names a language file has to name a website language
// too, or fr-CH would mean two different things in one program.
func Valid(tag string) bool { return i18n.ValidTag(tag) }

// Normalise cleans a tag as somebody typed it: "DE_ch" becomes "de-CH".
func Normalise(tag string) string { return i18n.Normalise(tag) }

// names are the languages a Swiss or German website is likely to use, plus the
// ones a visitor might. An unknown tag is shown as it is — a wrong name would
// be worse than the tag.
var names = map[string]string{
	"de": i18n.N("Deutsch"), "fr": i18n.N("Französisch"), "it": i18n.N("Italienisch"), "rm": i18n.N("Rätoromanisch"),
	"en": i18n.N("Englisch"), "es": i18n.N("Spanisch"), "pt": i18n.N("Portugiesisch"), "nl": i18n.N("Niederländisch"),
	"pl": i18n.N("Polnisch"), "tr": i18n.N("Türkisch"), "ru": i18n.N("Russisch"), "uk": i18n.N("Ukrainisch"),
	"cs": i18n.N("Tschechisch"), "hu": i18n.N("Ungarisch"), "da": i18n.N("Dänisch"), "sv": i18n.N("Schwedisch"),
	"no": i18n.N("Norwegisch"), "fi": i18n.N("Finnisch"), "hr": i18n.N("Kroatisch"), "sr": i18n.N("Serbisch"),
	"sq": i18n.N("Albanisch"), "ar": i18n.N("Arabisch"),
}

// Name is the language's name in German, with the region appended when there is
// one: "Französisch (CH)".
func Name(tag string) string {
	base, region, hasRegion := strings.Cut(tag, "-")
	name, known := names[strings.ToLower(base)]
	if !known {
		name = tag
	}
	if hasRegion && known {
		name += " (" + region + ")"
	}
	return name
}

// natives are the same languages as they call themselves.
//
// A switcher is read by somebody who does not speak the page they are on. A
// German site offering "Französisch" tells a French visitor nothing they can
// use; "Français" is the one word on the page they are certain to recognise.
var natives = map[string]string{
	"de": "Deutsch", "fr": "Français", "it": "Italiano", "rm": "Rumantsch",
	"en": "English", "es": "Español", "pt": "Português", "nl": "Nederlands",
	"pl": "Polski", "tr": "Türkçe", "ru": "Русский", "uk": "Українська",
	"cs": "Čeština", "hu": "Magyar", "da": "Dansk", "sv": "Svenska",
	"no": "Norsk", "fi": "Suomi", "hr": "Hrvatski", "sr": "Srpski",
	"sq": "Shqip", "ar": "العربية",
}

// Native is the language's name in itself, for a switcher a visitor reads.
// Name is the German one, for the admin.
func Native(tag string) string {
	base, region, hasRegion := strings.Cut(tag, "-")
	name, known := natives[strings.ToLower(base)]
	if !known {
		return Name(tag)
	}
	if hasRegion {
		name += " (" + region + ")"
	}
	return name
}

// ParseList reads the additional languages as they are stored and as an
// operator types them: separated by commas, spaces or new lines.
//
// The primary language is dropped if it turns up again, duplicates are dropped,
// and anything that is not a language tag is dropped. What comes back is a list
// that can be trusted everywhere else.
func ParseList(raw, primary string) []string {
	primary = Normalise(primary)
	seen := map[string]bool{primary: true}

	var out []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	}) {
		tag := Normalise(part)
		if !Valid(tag) || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
		if len(out) >= MaxExtra {
			break
		}
	}
	return out
}

// JoinList writes the list back for storage and for the form.
func JoinList(tags []string) string { return strings.Join(tags, ", ") }

// Prefix is the address prefix of a language: "" for the main one, "/fr"
// otherwise.
func Prefix(tag, primary string) string {
	if tag == "" || Normalise(tag) == Normalise(primary) {
		return ""
	}
	return "/" + tag
}

// Path puts a prefix in front of a path: Path("fr", "de", "/kontakt") is
// "/fr/kontakt".
func Path(tag, primary, path string) string {
	if path == "" {
		path = "/"
	}
	prefix := Prefix(tag, primary)
	if prefix == "" {
		return path
	}
	if path == "/" {
		return prefix
	}
	return prefix + path
}

// Split takes a language prefix off a request path.
//
// It only accepts a language the website actually has: without that check
// /it/… on a German-and-French site would silently serve the German page under
// a made-up address, and a search engine would index both.
func Split(path string, extras []string) (tag, rest string) {
	trimmed := strings.TrimPrefix(path, "/")
	first, remainder, _ := strings.Cut(trimmed, "/")
	for _, tag := range extras {
		if first == tag {
			if remainder == "" {
				return tag, "/"
			}
			return tag, "/" + remainder
		}
	}
	return "", path
}

// Pick takes a language out of a form field.
//
// Anything that is not one of the website's additional languages becomes the
// main language — the empty string. A form can be sent with any value in it,
// and a menu or a page filed under a language the site does not serve would be
// invisible without anyone being able to see why.
func Pick(raw string, extras []string) string {
	tag := Normalise(strings.TrimSpace(raw))
	for _, e := range extras {
		if tag == e {
			return tag
		}
	}
	return ""
}

// Reserved reports whether a slug would be swallowed by a language prefix.
//
// A page called "fr" on a site that has French would never be reachable: the
// address /fr belongs to the language. The editor is told before saving rather
// than afterwards, when the page is already invisible.
func Reserved(slug string, extras []string) bool {
	for _, tag := range extras {
		if slug == tag {
			return true
		}
	}
	return false
}
