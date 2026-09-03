// Package i18n is the language of the administration.
//
// One decision runs through everything here: **the German text is the key.**
// A template says {{t "Seite gespeichert"}}, not {{t "page.saved"}}, and a
// catalogue maps that German sentence onto the other language.
//
// Two things follow, and both matter more than the tidiness of a key scheme:
//
//   - A missing translation degrades to German, which is a sentence somebody
//     can read, rather than to "page.saved", which is a defect on the screen.
//   - The templates stay readable. Anyone can see what a screen says without
//     looking anything up, which is the difference between a translation that
//     stays current and one that rots.
//
// Adding a language is dropping a JSON file in: into internal/i18n/locales for
// one that ships with the binary, or into <data>/sprachen on a running
// installation, where it needs no build at all. See disk.go.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Source is the language the strings in the source code are written in. It
// needs no catalogue: a lookup that finds nothing returns the text unchanged,
// which is already German.
const Source = "de"

//go:embed locales/*.json
var catalogFS embed.FS

// Language is one language the administration can be shown in.
type Language struct {
	// Code is the tag, "de" or "en".
	Code string
	// Name is what the language calls itself, because it is chosen by somebody
	// who wants to read it: "English", not "Englisch".
	Name string
}

// names are the languages as they call themselves. A catalogue whose language
// is missing here is shown by its tag rather than being left out — a language
// nobody can select is a language that might as well not ship.
var names = map[string]string{
	"de": "Deutsch",
	"en": "English",
	"fr": "Français",
	"it": "Italiano",
	"nl": "Nederlands",
	"es": "Español",
	"pl": "Polski",
	"pt": "Português",
	"tr": "Türkçe",
	// Rätoromanisch has no second country to be distinguished from, so it is a
	// language and not a fassung. It ships no catalogue — see the note in
	// Stats — but it is named here so an rm.json dropped into the folder turns
	// up as "Rumantsch" rather than as "rm".
	"rm": "Rumantsch",
}

// regions name a regional fassung in its own language, because the account
// picker is read by the person who wants that fassung.
//
// Only the ones that mean something here: the languages of Switzerland, where
// the difference is real enough to be worth a file.
var regions = map[string]string{
	"de-CH": "Schweiz",
	"fr-CH": "Suisse",
	"it-CH": "Svizzera",
	"rm-CH": "Svizra",
	"de-AT": "Österreich",
	"de-DE": "Deutschland",
	"fr-FR": "France",
	"it-IT": "Italia",
	"en-GB": "UK",
	"en-US": "US",
	"es-ES": "España",
	"pt-BR": "Brasil",
}

var (
	// mu guards the three below. They are rebuilt wholesale by Reload, which an
	// administrator can trigger while requests are being served.
	mu        sync.RWMutex
	once      sync.Once
	catalogs  map[string]map[string]string
	onDisk    map[string]bool
	languages []Language
)

// ensure loads once, for the readers that may run before SetDir.
func ensure() {
	once.Do(func() {
		mu.Lock()
		defer mu.Unlock()
		if catalogs == nil {
			loadLocked()
		}
	})
}

// loadLocked reads the embedded catalogues and then the folder on disk. The
// caller holds mu.
//
// A broken catalogue is logged and skipped rather than fatal: a typo in a
// translation must not stop the server from starting, because the German
// administration underneath it works perfectly.
func loadLocked() {
	catalogs = map[string]map[string]string{}
	onDisk = map[string]bool{}

	entries, err := fs.Glob(catalogFS, "locales/*.json")
	if err != nil {
		slog.Error("read message catalogues", "err", err)
	}
	for _, name := range entries {
		code := Normalise(strings.TrimSuffix(strings.TrimPrefix(name, "locales/"), ".json"))
		data, err := catalogFS.ReadFile(name)
		if err != nil {
			slog.Error("read message catalogue", "err", err, "file", name)
			continue
		}
		var msgs map[string]string
		if err := json.Unmarshal(data, &msgs); err != nil {
			slog.Error("message catalogue is not valid JSON", "err", err, "file", name)
			continue
		}
		// An empty translation is not a translation. Dropping it here means the
		// German shows through, which is what a half-finished catalogue should
		// look like.
		for key, value := range msgs {
			if strings.TrimSpace(value) == "" {
				delete(msgs, key)
			}
		}
		catalogs[code] = msgs
	}

	// The folder on disk, on top of what is compiled in.
	readDir(catalogs, onDisk)

	languages = []Language{{Code: Source, Name: nameOf(Source)}}
	for code := range catalogs {
		if code == Source {
			continue
		}
		languages = append(languages, Language{Code: code, Name: nameOf(code)})
	}
	sort.Slice(languages, func(i, j int) bool { return languages[i].Name < languages[j].Name })
}

// nameOf is what a language calls itself: "Français", "Français (Suisse)".
//
// A regional fassung is named after its base language plus the region, so the
// two sort next to each other in the picker and nobody has to guess what
// "fr-CH" was. An unknown tag is shown as it is — a wrong name is worse.
func nameOf(code string) string {
	if name, ok := names[code]; ok {
		return name
	}
	base := Base(code)
	if base == "" {
		return code
	}
	name, ok := names[base]
	if !ok {
		return code
	}
	if region, ok := regions[code]; ok {
		return name + " (" + region + ")"
	}
	return name + " (" + Region(code) + ")"
}

// Languages are the languages the administration can be shown in, German
// first-class among them.
func Languages() []Language {
	ensure()
	mu.RLock()
	defer mu.RUnlock()
	return languages
}

// Known reports whether a tag is a language this build can show.
func Known(code string) bool {
	if code == Source {
		return true
	}
	ensure()
	mu.RLock()
	defer mu.RUnlock()
	_, ok := catalogs[code]
	return ok
}

// T translates one string.
//
// Unknown language, unknown string, empty translation: the German comes back.
// There is no error case, because there is nothing a caller could usefully do
// about it — and a page that renders in German is a page.
//
// A regional fassung is asked first and its base language second, so fr-CH only
// has to carry what it says differently from fr.
func T(lang, s string) string {
	if lang == "" || lang == Source || s == "" {
		return s
	}
	ensure()
	mu.RLock()
	defer mu.RUnlock()
	for _, code := range chain(lang) {
		msgs, ok := catalogs[code]
		if !ok {
			continue
		}
		// An empty translation is not a translation. load() already drops
		// those, and the check is repeated here because this is the function
		// everything goes through: a blank label on a screen is worse than a
		// German one, and worth guarding twice.
		if out, ok := msgs[s]; ok && out != "" {
			return out
		}
	}
	return s
}

// N marks a string for translation without translating it.
//
// For the tables of labels that are built once at start-up, long before there
// is a request and therefore a language: the kinds of block, the kinds of
// field, the readiness checks. The value travels through the program in German
// and is translated where it is rendered, with {{t}}.
//
// It returns its argument unchanged. Its whole job is to be a name the
// extraction tool can find, so a label added to one of those tables turns up in
// the catalogue instead of quietly staying German.
func N(s string) string { return s }

// Tf translates a format string and then fills it in.
//
// The format is what gets translated, so a language that puts the parts in a
// different order can say so: "Seiten – %s" against "%s – pages".
func Tf(lang, format string, args ...any) string {
	return fmt.Sprintf(T(lang, format), args...)
}

// FromAcceptLanguage picks a language out of the browser's header.
//
// It is the fallback for the screens where nobody is logged in yet — the login
// form, the first-run setup — because those are exactly the screens where being
// shown a language you cannot read is worst: there is no way in to change it.
//
// Deliberately simple: quality values are read, everything else about RFC 4647
// is not. A header that asks for nothing this build has yields German.
func FromAcceptLanguage(header string) string {
	type wish struct {
		code string
		q    float64
	}
	var wishes []wish

	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		code, rest, _ := strings.Cut(part, ";")
		q := 1.0
		if value, ok := strings.CutPrefix(strings.TrimSpace(rest), "q="); ok {
			if parsed, err := parseQ(value); err == nil {
				q = parsed
			}
		}
		wishes = append(wishes, wish{code: Normalise(code), q: q})
	}

	sort.SliceStable(wishes, func(i, j int) bool { return wishes[i].q > wishes[j].q })
	for _, w := range wishes {
		if w.q <= 0 {
			continue
		}
		// The full tag first, then the language without its region. A Swiss
		// browser asks for de-CH and gets the Swiss fassung when there is one;
		// a German one asks for de-DE, finds nothing under that name, and gets
		// German rather than nothing at all.
		if Known(w.code) {
			return w.code
		}
		if base := Base(w.code); base != "" && Known(base) {
			return base
		}
	}
	return Source
}

func parseQ(s string) (float64, error) {
	var q float64
	_, err := fmt.Sscanf(s, "%g", &q)
	return q, err
}

// FromRequest is FromAcceptLanguage for a request.
func FromRequest(r *http.Request) string {
	return FromAcceptLanguage(r.Header.Get("Accept-Language"))
}
