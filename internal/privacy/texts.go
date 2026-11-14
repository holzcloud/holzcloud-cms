package privacy

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
)

// textFS holds the words of every legal page:
//
//	texts/<lang>/lang.json             the notice, the closing line, the fill-in marker
//	texts/<lang>/<kind>/meta.json      title, slug, draft title and slug, table cells
//	texts/<lang>/<kind>/<block>.md     one block of the page
//
//go:embed texts
var textFS embed.FS

// langWords are the four sentences every kind of page shares in one language.
type langWords struct {
	TemplateNotice string `json:"template_notice"`
	NoLegalAdvice  string `json:"no_legal_advice"`
	Fillin         string `json:"fillin"`
	// MachineNotice says the text was written by a machine and must be checked
	// by a speaker before it is published (D-07). Empty for every language a
	// person wrote.
	MachineNotice string `json:"machine_notice"`
}

// kindTexts are the words of one kind of page in one language.
type kindTexts struct {
	meta   map[string]string
	also   []string
	blocks map[string]string
}

type textTree struct {
	langs map[string]langWords
	kinds map[Kind]map[string]*kindTexts
}

var (
	treeOnce sync.Once
	tree     *textTree
)

// texts reads the embedded tree once. The files are part of the binary, so a
// file that does not parse is a build that should never have passed its
// tests, and it panics.
func texts() *textTree {
	treeOnce.Do(func() {
		t, err := loadTexts(textFS)
		if err != nil {
			panic(err)
		}
		tree = t
	})
	return tree
}

func loadTexts(fsys fs.FS) (*textTree, error) {
	t := &textTree{langs: map[string]langWords{}, kinds: map[Kind]map[string]*kindTexts{}}
	langs, err := fs.ReadDir(fsys, "texts")
	if err != nil {
		return nil, err
	}
	for _, l := range langs {
		if !l.IsDir() {
			continue
		}
		lang := l.Name()
		raw, err := fs.ReadFile(fsys, path.Join("texts", lang, "lang.json"))
		if err != nil {
			return nil, err
		}
		var w langWords
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("texts/%s/lang.json: %w", lang, err)
		}
		t.langs[lang] = w

		kinds, err := fs.ReadDir(fsys, path.Join("texts", lang))
		if err != nil {
			return nil, err
		}
		for _, kd := range kinds {
			if !kd.IsDir() {
				continue
			}
			kt, err := loadKind(fsys, path.Join("texts", lang, kd.Name()))
			if err != nil {
				return nil, err
			}
			k := Kind(kd.Name())
			if t.kinds[k] == nil {
				t.kinds[k] = map[string]*kindTexts{}
			}
			t.kinds[k][lang] = kt
		}
	}
	return t, nil
}

func loadKind(fsys fs.FS, dir string) (*kindTexts, error) {
	kt := &kindTexts{meta: map[string]string{}, blocks: map[string]string{}}
	raw, err := fs.ReadFile(fsys, path.Join(dir, "meta.json"))
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%s/meta.json: %w", dir, err)
	}
	for key, v := range m {
		if key == "also_slugs" {
			if err := json.Unmarshal(v, &kt.also); err != nil {
				return nil, fmt.Errorf("%s/meta.json %s: %w", dir, key, err)
			}
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, fmt.Errorf("%s/meta.json %s: %w", dir, key, err)
		}
		kt.meta[key] = s
	}
	files, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		name, ok := strings.CutSuffix(f.Name(), ".md")
		if !ok || f.IsDir() {
			continue
		}
		b, err := fs.ReadFile(fsys, path.Join(dir, f.Name()))
		if err != nil {
			return nil, err
		}
		kt.blocks[name] = strings.Trim(string(b), "\n")
	}
	return kt, nil
}

// baseOf is the language of a tag without its region, in lower case.
func baseOf(tag string) string {
	base, _, _ := strings.Cut(tag, "-")
	return strings.ToLower(strings.TrimSpace(base))
}

// TextLang is the language whose texts a page of kind k in the language tag
// gets.
//
// The region is dropped: de-CH reads the German texts, and the Swiss spelling
// is a separate switch (Request.Swiss). Spanish gets the English texts — the
// owner decided against Spanish legal templates for now (D-16), and English is
// the language a Spanish reader is likelier to read than German. Any language
// without texts of its own falls back the same way, to English and, while
// English has none for the kind, to German, which every kind has.
//
// The tag is only ever a key here. It is never part of a path and never part
// of the result, so an odd tag gets the fallback and nothing else.
func TextLang(k Kind, tag string) string {
	have := texts().kinds[k]
	base := baseOf(tag)
	if base == "es" {
		base = "en"
	}
	if _, ok := have[base]; ok {
		return base
	}
	if _, ok := have["en"]; ok {
		return "en"
	}
	return "de"
}

// meta is one value of the meta.json of a kind in a text language.
func meta(k Kind, lang, key string) string {
	kt := texts().kinds[k][lang]
	if kt == nil {
		return ""
	}
	return kt.meta[key]
}

// Title is the title of a page of kind k in the language tag.
func Title(k Kind, tag string) string { return meta(k, TextLang(k, tag), "title") }

// Slug is the slug of a page of kind k in the language tag.
func Slug(k Kind, tag string) string { return meta(k, TextLang(k, tag), "slug") }

// DraftTitle is the title of a new template offered next to an existing page.
func DraftTitle(k Kind, tag string) string { return meta(k, TextLang(k, tag), "draft_title") }

// DraftSlug is the slug of that draft.
func DraftSlug(k Kind, tag string) string { return meta(k, TextLang(k, tag), "draft_slug") }

// RoleSlugs are the slugs under which a page of kind k can stand, in every
// language that has texts for it: each language's slug and its other usual
// spellings. Sorted, without duplicates.
func RoleSlugs(k Kind) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, kt := range texts().kinds[k] {
		add(kt.meta["slug"])
		for _, s := range kt.also {
			add(s)
		}
	}
	sort.Strings(out)
	return out
}

// roleJoiners are what stands between two role slugs when one page serves
// several roles: "impressum-datenschutz", "impressum-und-datenschutz",
// "imprint-and-privacy", "mentions-legales-et-confidentialite",
// "note-legali-e-privacy". The bare hyphen last, so that "-und-" is read as
// a joiner and not as the start of a slug "und-…".
var roleJoiners = []string{"-und-", "-and-", "-et-", "-e-", "-"}

// IsRoleSlug reports whether a page under slug is a page of kind k (IN-02 of
// the second 33.1 review): one of RoleSlugs(k); or role slugs of the legal
// kinds joined into one page's address, at least one of them k's — one page
// that is imprint and privacy notice at once is common on small websites;
// and either of them with the numeric counter CreatePage appends to a taken
// address ("datenschutz-2").
//
// Nothing looser. A slug that merely contains or starts with a role slug —
// a post "privacy-tips", the published compare draft "datenschutz-vorlage" —
// is not the page (IN-06 of the first review). The variants a language uses
// are listed in its meta.json under also_slugs, and that list is the place
// to add one.
func IsRoleSlug(k Kind, slug string) bool {
	if base, counter, ok := cutLast(slug, "-"); ok && counter != "" && strings.Trim(counter, "0123456789") == "" {
		slug = base
	}
	own := RoleSlugs(k)
	if slices.Contains(own, slug) {
		return true
	}
	var all []string
	for _, kind := range []Kind{KindImprint, KindPrivacy, KindDNS} {
		all = append(all, RoleSlugs(kind)...)
	}
	return joinedRoles(slug, own, all, false)
}

// joinedRoles reports whether slug is a role slug of all followed by a
// joiner and the rest, down to a last role slug, with at least one of them
// in own. Only called on what follows a joiner or on a whole slug that is
// not itself a role slug of k, so one role slug alone never passes here
// unless it is k's.
func joinedRoles(slug string, own, all []string, sawOwn bool) bool {
	for _, r := range all {
		rest, ok := strings.CutPrefix(slug, r)
		if !ok {
			continue
		}
		mine := sawOwn || slices.Contains(own, r)
		for _, j := range roleJoiners {
			tail, ok := strings.CutPrefix(rest, j)
			if !ok || tail == "" {
				continue
			}
			if slices.Contains(all, tail) && (mine || slices.Contains(own, tail)) {
				return true
			}
			if joinedRoles(tail, own, all, mine) {
				return true
			}
		}
	}
	return false
}

// cutLast is strings.Cut at the last sep.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

// wordsOf is the lang.json of a tag, by the same fallback as TextLang: its
// base, Spanish read as English, then English, then German. It used to go
// straight to German, so a tag without texts (nl) composed an English page
// while TemplateNotice, Fillin and NoLegalAdvice answered in German (IN-05).
func wordsOf(tag string) langWords {
	base := baseOf(tag)
	if base == "es" {
		base = "en"
	}
	for _, lang := range []string{base, "en", "de"} {
		if w, ok := texts().langs[lang]; ok {
			return w
		}
	}
	return texts().langs["de"]
}

// TemplateNotice is the sentence that marks a page as an unfilled template, in
// the language of the tag. The German one is the sentence of Phase 30, byte
// for byte, so pages made since then are still recognised.
func TemplateNotice(tag string) string { return wordsOf(tag).TemplateNotice }

// NoLegalAdvice is the closing line of every composed page.
func NoLegalAdvice(tag string) string { return wordsOf(tag).NoLegalAdvice }

// Fillin is the visible marker for a fact only the operator knows.
func Fillin(tag string) string { return wordsOf(tag).Fillin }

// HasTemplateNotice reports whether the page still carries the sentence that
// marks it as an unfilled template, in any language and in the Swiss spelling
// too: a page made for a Swiss website carries the notice in that form.
func HasTemplateNotice(markdown string) bool {
	for _, w := range texts().langs {
		n := w.TemplateNotice
		if n == "" {
			continue
		}
		if strings.Contains(markdown, n) || strings.Contains(markdown, swissSpelling.Replace(n)) {
			return true
		}
	}
	return false
}
