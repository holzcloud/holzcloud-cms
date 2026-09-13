package template

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The words a theme mints itself.
//
// A theme writes "Cart" somewhere, and no catalogue of this program can hold
// it: the theme is uploadable content and this program does not ship it. That
// was the reasoning behind TEMPLATE-SPEC.md §2.5, which decided a theme is
// single-language and left t out of the public FuncMap deliberately. The
// reasoning was right about where the sentence cannot live and wrong about the
// conclusion, because it left all eight shipped themes hard-wired to German —
// "Warenkorb" 48 times — so a website published in French had no theme it
// could use.
//
// The answer is that the sentence lives in the theme. A theme carries
// lang/<tag>.json beside its templates, t resolves against that and against
// nothing else, and a key it does not carry is reported by
// `holzcloud template check` rather than swallowed. The objection §2.5 raised —
// that a t working for our themes and silently failing for yours is worse than
// none — is answered by both halves of that sentence, not just the first.

// langDir is where a theme keeps its catalogues. One directory, so an archive's
// file list says at a glance whether it is translated at all.
const langDir = "lang"

// Catalog is one theme's words in one language.
//
// The zero value is usable and translates nothing, which is what an untranslated
// theme gets: every key comes back as itself. Since the keys are the English
// sentences the theme author wrote, an untranslated theme renders exactly as it
// did before this existed.
type Catalog struct {
	entries map[string]string
}

// T translates one key.
//
// A missing key comes back as itself rather than as an empty string or a
// marker. A visitor then reads the theme author's own English, which is a page;
// the alternatives are a blank button and a page full of ‹‹cart››. What makes
// this defensible rather than silent is that Check reports every such key
// before the theme is ever installed.
func (c Catalog) T(key string) string {
	if s, ok := c.entries[key]; ok && s != "" {
		return s
	}
	return key
}

// with lays one set of words over this catalogue and returns the result.
//
// A copy, because a Catalog is cached inside a parsed template set and two
// websites on the same theme must not be able to write into each other's. The
// map is small — 150 keys across all eight shipped themes together — so copying
// it per template set is not worth a lock.
//
// An empty value in the overlay is ignored rather than blanking the word. The
// store deletes an emptied entry instead of storing "", but a row from an older
// build or a hand-edited database must not be able to put a blank button on a
// page.
func (c Catalog) with(over map[string]string) Catalog {
	if len(over) == 0 {
		return c
	}
	merged := make(map[string]string, len(c.entries)+len(over))
	for k, v := range c.entries {
		merged[k] = v
	}
	for k, v := range over {
		if v == "" {
			continue
		}
		merged[k] = v
	}
	return Catalog{entries: merged}
}

// Has reports whether the catalogue carries a key at all. CheckCatalogs uses it; T does
// not need it, because its answer to both cases is the same.
func (c Catalog) Has(key string) bool {
	s, ok := c.entries[key]
	return ok && s != ""
}

// Len is how many keys the catalogue translates.
func (c Catalog) Len() int { return len(c.entries) }

// loadCatalog reads one theme's catalogue for a language.
//
// The fallback chain is exact tag, then base language: a site published as
// "de-CH" gets lang/de-CH.json if the theme has one and lang/de.json if it does
// not. It stops there. There is no fall back to another language, because a
// French page carrying German chrome is the defect this whole thing exists to
// remove, and quietly substituting one language for another is how it would
// come back.
func loadCatalog(read fileReader, locale string) Catalog {
	for _, tag := range candidates(locale) {
		raw, err := read(path.Join(langDir, tag+".json"))
		if err != nil || len(raw) == 0 {
			continue
		}
		var entries map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			// A catalogue that does not parse is treated as absent. The upload
			// check says so in terms the author can act on; refusing to render
			// here would take the site down over a comma.
			continue
		}
		return Catalog{entries: entries}
	}
	return Catalog{}
}

// candidates is the tags to try, most specific first.
func candidates(locale string) []string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return nil
	}
	if base, _, found := strings.Cut(locale, "-"); found && base != "" {
		return []string{locale, base}
	}
	return []string{locale}
}

// --- what a theme asks for --------------------------------------------------

// keyCall finds the keys a template asks for.
//
// It reads the source rather than executing it, because a key inside {{if}} is
// asked for on some pages and not on others, and a report that depended on
// which page was rendered would be worse than no report. The cost is that a key
// built at run time — {{t (printf "cart.%s" .X)}} — is invisible here, and that
// is the same trap tools/i18n documents for the admin side: assemble the
// sentence in the catalogue, not in the template.
var keyCall = regexp.MustCompile(`\{\{-?\s*t(?:h|f)?\s+"((?:[^"\\]|\\.)*)"`)

// KeysUsed lists every key the theme's own template files ask for, sorted and
// without repeats.
func KeysUsed(theme fs.FS) []string {
	seen := map[string]bool{}
	for _, name := range append([]string{layoutFile}, viewFiles...) {
		raw, err := fs.ReadFile(theme, name)
		if err != nil {
			continue
		}
		for _, m := range keyCall.FindAllStringSubmatch(string(raw), -1) {
			key, err := strconv.Unquote(`"` + m[1] + `"`)
			if err != nil {
				continue
			}
			seen[key] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// LanguagesOffered lists the languages a theme carries a catalogue for.
func LanguagesOffered(theme fs.FS) []string {
	entries, err := fs.ReadDir(theme, langDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(out)
	return out
}

// CheckCatalogs reports the keys a theme mints and does not translate.
//
// This is the half that makes t defensible. Without it a theme author ships a
// French catalogue missing four keys, four English words stand in the middle of
// a French page, every test is green and nobody finds out until a visitor
// mentions it.
//
// It is deliberately NOT part of Check, and therefore refuses nothing. Check
// answers "will this template work?", and a theme with four untranslated words
// works — it shows the author's own English in those four places. Refusing the
// upload over it would make a half-translated theme unusable, which is worse
// than the thing being reported and is not this program's decision to take.
// `holzcloud template check` prints these, and the admin's template screen
// shows them; neither stands between the operator and their own site.
func CheckCatalogs(theme fs.FS) []Problem {
	used := KeysUsed(theme)
	offered := LanguagesOffered(theme)
	if len(used) == 0 && len(offered) == 0 {
		return nil
	}

	var problems []Problem
	if len(used) > 0 && len(offered) == 0 {
		return []Problem{{
			File:    langDir + "/",
			Message: fmt.Sprintf("the theme asks for %d translatable words and carries no catalogue", len(used)),
			Hint: "put lang/<language>.json beside the templates, e.g. " +
				langDir + "/en.json, with one entry per key. Without it every word shows as itself.",
		}}
	}

	for _, tag := range offered {
		raw, err := fs.ReadFile(theme, path.Join(langDir, tag+".json"))
		if err != nil {
			continue
		}
		var entries map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			problems = append(problems, Problem{
				File:    path.Join(langDir, tag+".json"),
				Message: "the catalogue is not valid JSON: " + err.Error(),
				Hint:    "an object of key to translation, both strings",
			})
			continue
		}
		cat := Catalog{entries: entries}

		var missing []string
		for _, key := range used {
			if !cat.Has(key) {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, Problem{
				File:    path.Join(langDir, tag+".json"),
				Message: fmt.Sprintf("%d of %d words are not translated", len(missing), len(used)),
				Hint:    "missing: " + list(missing),
			})
		}

		// A key in the catalogue that no template asks for is the other
		// direction of the same mistake — usually a word that was reworded in
		// the template and left behind here. Worth saying, never worth
		// refusing: an archive is allowed to carry more than it uses.
		var spare []string
		for key := range entries {
			if !contains(used, key) {
				spare = append(spare, key)
			}
		}
		if len(spare) > 0 {
			sort.Strings(spare)
			problems = append(problems, Problem{
				File:    path.Join(langDir, tag+".json"),
				Message: fmt.Sprintf("%d entries are for words no template asks for", len(spare)),
				Hint:    "left over: " + list(spare),
			})
		}
	}
	return problems
}

// list prints at most five of them, because a hint nobody reads to the end is
// the same as no hint.
func list(keys []string) string {
	const max = 5
	if len(keys) <= max {
		return strings.Join(quoteAll(keys), ", ")
	}
	return fmt.Sprintf("%s and %d more",
		strings.Join(quoteAll(keys[:max]), ", "), len(keys)-max)
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = `"` + s + `"`
	}
	return out
}

func contains(haystack []string, needle string) bool {
	i := sort.SearchStrings(haystack, needle)
	return i < len(haystack) && haystack[i] == needle
}

// CatalogFor is one theme's words in one language, as a plain map.
//
// For the admin screen, which has to show what the theme offers beside what the
// operator made of it. Catalog itself stays unexported in its shape: this hands
// out a copy, so a screen cannot write into a catalogue that a render is using.
func CatalogFor(theme fs.FS, locale string) map[string]string {
	cat := loadCatalog(func(name string) ([]byte, error) {
		return fs.ReadFile(theme, name)
	}, locale)
	out := make(map[string]string, len(cat.entries))
	for k, v := range cat.entries {
		out[k] = v
	}
	return out
}
