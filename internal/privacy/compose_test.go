package privacy

import (
	"encoding/json"
	"io/fs"
	"path"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// These tests hold for every kind that registers itself and for every language
// that has files. A later plan adds a language or a kind by adding files and
// one registration; it never edits a test here.

// combos returns all 32 combinations of the five booleans, including the
// inconsistent ones (Payment or PriceChoice without Shop).
func combos() []Features {
	var out []Features
	for i := 0; i < 32; i++ {
		out = append(out, Features{
			Shop:           i&1 != 0,
			PriceChoice:    i&2 != 0,
			Payment:        i&4 != 0,
			ProtectedPages: i&8 != 0,
			Forms:          i&16 != 0,
		})
	}
	return out
}

// sets returns all 16 subsets of the four frameworks.
func sets() []legal.Set {
	var out []legal.Set
	all := legal.All()
	for i := 0; i < 1<<len(all); i++ {
		var ids []string
		for j, f := range all {
			if i&(1<<j) != 0 {
				ids = append(ids, string(f))
			}
		}
		out = append(out, legal.ParseSet(strings.Join(ids, ",")))
	}
	return out
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n "), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// languagesOf lists the languages with a directory for the kind, read from
// the embedded tree rather than from the code under test.
func languagesOf(t *testing.T, k Kind) []string {
	t.Helper()
	entries, err := fs.ReadDir(textFS, "texts")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := fs.Stat(textFS, path.Join("texts", e.Name(), string(k))); err == nil {
			out = append(out, e.Name())
		}
	}
	return out
}

// allLanguages lists every language with a lang.json.
func allLanguages(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(textFS, "texts")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// langFile reads one language's lang.json straight from the tree.
func langFile(t *testing.T, lang string) map[string]string {
	t.Helper()
	raw, err := fs.ReadFile(textFS, path.Join("texts", lang, "lang.json"))
	if err != nil {
		t.Fatalf("%s: %v", lang, err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s/lang.json: %v", lang, err)
	}
	return m
}

// metaKeys reads the key set of one meta.json.
func metaKeys(t *testing.T, lang string, k Kind) []string {
	t.Helper()
	raw, err := fs.ReadFile(textFS, path.Join("texts", lang, string(k), "meta.json"))
	if err != nil {
		t.Fatalf("%s/%s: %v", lang, k, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s/%s/meta.json: %v", lang, k, err)
	}
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var leftover = regexp.MustCompile(`\{[a-z_]+\}`)

func TestEveryCompositionIsAMarkedTemplate(t *testing.T) {
	if len(Kinds()) == 0 {
		t.Fatal("no kind is registered")
	}
	for _, k := range Kinds() {
		spec := registry[k]
		langs := languagesOf(t, k)
		if len(langs) == 0 {
			t.Fatalf("%s: no language has files", k)
		}
		for _, lang := range langs {
			words := langFile(t, lang)
			notice, closing, fill, machine := words["template_notice"], words["no_legal_advice"], words["fillin"], words["machine_notice"]
			for _, set := range sets() {
				for _, f := range combos() {
					r := Request{Lang: lang, Set: set, Features: f}
					a, b := compose(k, r), compose(k, r)
					where := string(k) + "/" + lang + " " + set.String() + " " + featureString(f)
					if a != b {
						t.Fatalf("%s: two calls gave different text", where)
					}
					if n := strings.Count(a, notice); n != 1 {
						t.Errorf("%s: template notice occurs %d times, want 1", where, n)
					}
					if i := strings.Index(a, notice); i < 0 || i > 400 {
						t.Errorf("%s: template notice at byte %d, want within the first 400", where, i)
					}
					if got := lastNonEmptyLine(a); got != closing {
						t.Errorf("%s: last line is %q, want the no-legal-advice sentence", where, got)
					}
					if n := strings.Count(a, fill); n < spec.minFillins {
						t.Errorf("%s: only %d fill-in markers, want at least %d", where, n, spec.minFillins)
					}
					for _, bad := range []string{"http://", "https://", "<"} {
						if strings.Contains(a, bad) {
							t.Errorf("%s: text contains %q: no link and no raw HTML belong in it", where, bad)
						}
					}
					if m := leftover.FindString(a); m != "" {
						t.Errorf("%s: placeholder %s was not expanded", where, m)
					}
					if machine != "" {
						if n := strings.Count(a, machine); n != 1 {
							t.Errorf("%s: machine notice occurs %d times, want 1", where, n)
						}
						if i := strings.Index(a, machine); i < 0 || i > 600 {
							t.Errorf("%s: machine notice at byte %d, want within the first 600", where, i)
						}
					}
					for _, other := range allLanguages(t) {
						if other == lang {
							continue
						}
						if m := langFile(t, other)["machine_notice"]; m != "" && strings.Contains(a, m) {
							t.Errorf("%s: carries the machine notice of %s", where, other)
						}
					}
				}
			}
		}
	}
}

func featureString(f Features) string {
	var on []string
	for name, v := range map[string]bool{"shop": f.Shop, "price": f.PriceChoice, "payment": f.Payment, "protected": f.ProtectedPages, "forms": f.Forms} {
		if v {
			on = append(on, name)
		}
	}
	sort.Strings(on)
	return "{" + strings.Join(on, ",") + "}"
}

func TestEveryLanguageHasEveryBlock(t *testing.T) {
	for _, lang := range allLanguages(t) {
		words := langFile(t, lang)
		var keys []string
		for k := range words {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if want := []string{"fillin", "machine_notice", "no_legal_advice", "template_notice"}; !slices.Equal(keys, want) {
			t.Errorf("%s/lang.json has keys %v, want exactly %v", lang, keys, want)
		}
		for _, k := range []string{"template_notice", "no_legal_advice", "fillin"} {
			if strings.TrimSpace(words[k]) == "" {
				t.Errorf("%s/lang.json: %s is empty", lang, k)
			}
		}
		// A kind directory nobody registered is text that can never be composed.
		entries, err := fs.ReadDir(textFS, path.Join("texts", lang))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() {
				if _, ok := registry[Kind(e.Name())]; !ok {
					t.Errorf("texts/%s/%s belongs to no registered kind", lang, e.Name())
				}
			}
		}
	}

	for _, k := range Kinds() {
		langs := languagesOf(t, k)
		if !slices.Contains(langs, "de") {
			t.Errorf("%s: German has no directory; German is the language every other one falls back to", k)
			continue
		}
		deKeys := metaKeys(t, "de", k)
		for _, key := range []string{"title", "slug", "also_slugs", "draft_title", "draft_slug"} {
			if !slices.Contains(deKeys, key) {
				t.Errorf("%s/de/meta.json lacks %q", k, key)
			}
		}
		var want []string
		for _, b := range registry[k].blocks {
			want = append(want, b.name+".md")
		}
		want = append(want, "meta.json")
		sort.Strings(want)
		for _, lang := range langs {
			entries, err := fs.ReadDir(textFS, path.Join("texts", lang, string(k)))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, e := range entries {
				got = append(got, e.Name())
			}
			sort.Strings(got)
			if !slices.Equal(got, want) {
				t.Errorf("texts/%s/%s holds %v, want exactly %v", lang, k, got, want)
			}
			if keys := metaKeys(t, lang, k); !slices.Equal(keys, deKeys) {
				t.Errorf("texts/%s/%s/meta.json has keys %v, want the German set %v", lang, k, keys, deKeys)
			}
		}
	}
}

// allSet is every framework chosen.
func allSet() legal.Set { return legal.ParseSet("eu,ch,uk,us") }

// frameworkBlocks are the framework sections of the privacy page, in their
// order.
var frameworkBlocks = []legal.Framework{legal.EU, legal.CH, legal.UK, legal.US}

// firstLine is the first line of a block file of a language.
func firstLine(t *testing.T, lang string, k Kind, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(textFS, path.Join("texts", lang, string(k), name+".md"))
	if err != nil {
		t.Fatalf("%s/%s/%s: %v", lang, k, name, err)
	}
	first, _, _ := strings.Cut(strings.TrimLeft(string(raw), "\n"), "\n")
	return first
}

func TestEachChosenFrameworkHasItsSectionOnce(t *testing.T) {
	for _, lang := range languagesOf(t, KindPrivacy) {
		heading := map[legal.Framework]string{}
		for _, fw := range frameworkBlocks {
			h := firstLine(t, lang, KindPrivacy, string(fw))
			if !strings.HasPrefix(h, "## ") {
				t.Errorf("%s/%s.md opens with %q, want a ## heading", lang, fw, h)
			}
			heading[fw] = "\n" + h + "\n"
		}
		media := "\n" + firstLine(t, lang, KindPrivacy, "media") + "\n"
		for _, set := range sets() {
			text := ComposePrivacy(Request{Lang: lang, Set: set, Features: allFeatures()})
			last := strings.Index(text, media)
			if last < 0 {
				t.Fatalf("%s %s: no media heading", lang, set)
			}
			end := strings.LastIndex(text, "\n---\n")
			for _, fw := range frameworkBlocks {
				n := strings.Count(text, heading[fw])
				want := 0
				if set.Has(fw) {
					want = 1
				}
				if n != want {
					t.Errorf("%s %q: section %s occurs %d times, want %d", lang, set, fw, n, want)
					continue
				}
				if n == 1 {
					i := strings.Index(text, heading[fw])
					if i < last {
						t.Errorf("%s %q: section %s stands before the media block or before an earlier section", lang, set, fw)
					}
					if i > end {
						t.Errorf("%s %q: section %s stands after the closing line", lang, set, fw)
					}
					last = i
				}
			}
		}
	}
}

func TestFrameworkSectionsCiteTheirLaw(t *testing.T) {
	anchors := map[legal.Framework][]string{
		legal.EU: {"2016/679", "2002/58", "art. 21", "art. 77"},
		legal.CH: {"235.1", "45c", "art. 25", "art. 49"},
		legal.UK: {"uk gdpr", "164a", "pecr"},
		legal.US: {"1798", "7025", "22575", "sec-gpc"},
	}
	for _, lang := range languagesOf(t, KindPrivacy) {
		for _, fw := range frameworkBlocks {
			text := strings.ToLower(ComposePrivacy(Request{Lang: lang, Set: legal.ParseSet(string(fw)), Features: allFeatures()}))
			for _, a := range anchors[fw] {
				if !strings.Contains(text, a) {
					t.Errorf("%s with %s chosen: the text does not cite %q", lang, fw, a)
				}
			}
		}
	}
}

func TestNoOperatorStringCanReachATemplate(t *testing.T) {
	// The inputs are booleans, a set and the language. A string field added
	// to the request would be a way for an operator's words into a template.
	rt := reflect.TypeOf(Request{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Type.Kind() == reflect.String && f.Name != "Lang" {
			t.Errorf("Request.%s is a string: only the language may be one", f.Name)
		}
	}
	ft := reflect.TypeOf(Features{})
	for i := 0; i < ft.NumField(); i++ {
		if ft.Field(i).Type.Kind() != reflect.Bool {
			t.Errorf("Features.%s is not a boolean", ft.Field(i).Name)
		}
	}
	for _, k := range Kinds() {
		for _, bad := range []string{"<script>", `de"><b>`, "../../etc"} {
			text := compose(k, Request{Lang: bad, Set: allSet(), Features: allFeatures()})
			if strings.Contains(text, bad) || strings.Contains(text, "<") {
				t.Errorf("%s with Lang %q echoes it", k, bad)
			}
			if lang := TextLang(k, bad); !slices.Contains(languagesOf(t, k), lang) {
				t.Errorf("%s: Lang %q falls back to %q, which has no files", k, bad, lang)
			}
			if text != compose(k, Request{Lang: TextLang(k, bad), Set: allSet(), Features: allFeatures()}) {
				t.Errorf("%s with Lang %q is not the text of the language it falls back to", k, bad)
			}
		}
	}
}

// swissForm is the Swiss spelling the test expects, written here and not
// taken from the code under test.
var swissForm = strings.NewReplacer("\u00df", "ss", "\u201e", "\u00ab", "\u201c", "\u00bb")

// A Swiss website gets the German text in Swiss spelling (D-15): the same
// text, with ss for the sharp s and with guillemets.
func TestSwissGermanHasNoEszett(t *testing.T) {
	plainPrivacy := ComposePrivacy(Request{Lang: "de", Set: allSet(), Features: allFeatures()})
	if !strings.Contains(plainPrivacy, "\u00df") {
		t.Fatal("the German privacy text has no sharp s, so this test would pass without the Swiss spelling")
	}
	for _, k := range Kinds() {
		for _, set := range sets() {
			for _, f := range []Features{{}, allFeatures()} {
				plain := compose(k, Request{Lang: "de", Set: set, Features: f})
				swiss := compose(k, Request{Lang: "de", Swiss: true, Set: set, Features: f})
				if swiss != swissForm.Replace(plain) {
					t.Errorf("%s %s: the Swiss text is not the German text in Swiss spelling", k, set)
				}
				for _, bad := range []string{"\u00df", "\u201e", "\u201c"} {
					if strings.Contains(swiss, bad) {
						t.Errorf("%s %s: the Swiss text still contains %q", k, set, bad)
					}
				}
				if de := compose(k, Request{Lang: "de-CH", Swiss: true, Set: set, Features: f}); de != swiss {
					t.Errorf("%s %s: de-CH with Swiss spelling differs from de with Swiss spelling", k, set)
				}
			}
		}
		// For a language with texts of its own the switch changes nothing.
		for _, lang := range []string{"fr", "it", "en", "rm"} {
			if TextLang(k, lang) == "de" {
				continue
			}
			r := Request{Lang: lang, Set: allSet(), Features: allFeatures()}
			plain := compose(k, r)
			r.Swiss = true
			if compose(k, r) != plain {
				t.Errorf("%s: Swiss spelling changed the %s text", k, lang)
			}
		}
	}
}

func TestTheTextLanguageFallsBack(t *testing.T) {
	for _, k := range Kinds() {
		have := map[string]bool{}
		for _, l := range languagesOf(t, k) {
			have[l] = true
		}
		fallback := "de"
		if have["en"] {
			fallback = "en"
		}
		cases := map[string]string{
			"de":    "de",
			"de-CH": "de",
			"DE-ch": "de",
			"es":    fallback,
			"es-MX": fallback,
			"pt":    fallback,
			"":      fallback,
			"xx-YY": fallback,
		}
		for _, l := range []string{"rm", "fr", "it", "en"} {
			if have[l] {
				cases[l] = l
				cases[l+"-CH"] = l
			} else {
				cases[l] = fallback
			}
		}
		for tag, want := range cases {
			if got := TextLang(k, tag); got != want {
				t.Errorf("TextLang(%s, %q) = %q, want %q", k, tag, got, want)
			}
			if got, want := Title(k, tag), meta(k, want, "title"); got != want || got == "" {
				t.Errorf("Title(%s, %q) = %q, want %q", k, tag, got, want)
			}
			if got, want := Slug(k, tag), meta(k, want, "slug"); got != want || got == "" {
				t.Errorf("Slug(%s, %q) = %q, want %q", k, tag, got, want)
			}
		}
	}
	if got := Slug(KindPrivacy, "de-CH"); got != "datenschutz" {
		t.Errorf("the German privacy slug is %q, want datenschutz", got)
	}
}

// The page's words and the page's text fall back the same way (IN-05): a tag
// without texts of its own composes the English page, so its notice, its
// closing line and its fill-in are the English ones too — not German words
// about an English page.
func TestTheWordsFallBackLikeTheText(t *testing.T) {
	for _, tag := range []string{"nl", "pt-BR", "xx", ""} {
		if TemplateNotice(tag) != TemplateNotice("en") || NoLegalAdvice(tag) != NoLegalAdvice("en") || Fillin(tag) != Fillin("en") {
			t.Errorf("%q: the words are not the English ones (notice %q)", tag, TemplateNotice(tag))
		}
		text := ComposePrivacy(Request{Lang: tag, Set: allSet(), Features: allFeatures()})
		if !strings.Contains(text, TemplateNotice(tag)) {
			t.Errorf("%q: the composed page does not carry the notice TemplateNotice gives", tag)
		}
		if !strings.HasSuffix(strings.TrimSpace(text), NoLegalAdvice(tag)) {
			t.Errorf("%q: the composed page does not end with the line NoLegalAdvice gives", tag)
		}
	}
}

func TestEveryLanguagesNoticeIsRecognised(t *testing.T) {
	for _, lang := range allLanguages(t) {
		words := langFile(t, lang)
		n := words["template_notice"]
		if !HasTemplateNotice("Intro\n\n**" + n + "** Rest") {
			t.Errorf("the %s notice is not recognised", lang)
		}
		if !HasTemplateNotice(swissForm.Replace(n)) {
			t.Errorf("the %s notice in Swiss spelling is not recognised", lang)
		}
		if HasTemplateNotice(words["no_legal_advice"]) {
			t.Errorf("the %s no-legal-advice line alone counts as a template notice", lang)
		}
		if TemplateNotice(lang) != n {
			t.Errorf("TemplateNotice(%s) = %q, want %q", lang, TemplateNotice(lang), n)
		}
	}
	if HasTemplateNotice("Datenschutz") {
		t.Error("a word that is no notice counts as one")
	}
	// An unknown language gets the English notice, as it gets the English
	// page (TextLang); it used to get the German one (IN-05 of the 33.1
	// review, TestTheWordsFallBackLikeTheText). A regional tag gets its base.
	if TemplateNotice("pt") != TemplateNotice("en") || TemplateNotice("de-CH") != TemplateNotice("de") {
		t.Error("an unknown language does not get the English notice, or a regional tag not its base's")
	}
}

// IsRoleSlug is what the launch checklist asks of a footer link (IN-02 of
// the second 33.1 review): a role slug, a variant the texts list, several
// role slugs joined into one page's address, each with the counter
// CreatePage appends — and not a slug that merely starts with one.
func TestIsRoleSlug(t *testing.T) {
	for _, tc := range []struct {
		slug string
		kind Kind
		want bool
	}{
		{"datenschutz", KindPrivacy, true},
		{"datenschutz-2", KindPrivacy, true},
		{"datenschutzhinweise", KindPrivacy, true},
		{"privacy-statement", KindPrivacy, true},
		{"impressum-datenschutz", KindPrivacy, true},
		{"impressum-datenschutz", KindImprint, true},
		{"impressum-und-datenschutz", KindImprint, true},
		{"impressum-und-datenschutz-3", KindPrivacy, true},
		{"imprint-and-privacy-policy", KindPrivacy, true},
		{"mentions-legales-et-confidentialite", KindPrivacy, true},
		{"note-legali-e-privacy", KindImprint, true},
		{"datenschutz", KindImprint, false},
		{"datenschutz-datenschutzhinweise", KindImprint, false},
		{"datenschutz-vorlage", KindPrivacy, false},
		{"privacy-tips", KindPrivacy, false},
		{"impressum-und-kontakt", KindImprint, false},
		{"impressum-", KindImprint, false},
		{"impressum-und-", KindImprint, false},
		{"-impressum", KindImprint, false},
		{"", KindImprint, false},
	} {
		if got := IsRoleSlug(tc.kind, tc.slug); got != tc.want {
			t.Errorf("IsRoleSlug(%s, %q) = %v, want %v", tc.kind, tc.slug, got, tc.want)
		}
	}
}

func TestRoleSlugsCoverEveryLanguage(t *testing.T) {
	got := RoleSlugs(KindPrivacy)
	if !slices.Contains(got, "datenschutz") {
		t.Errorf("RoleSlugs = %v, want datenschutz among them", got)
	}
	if !slices.IsSorted(got) {
		t.Errorf("RoleSlugs = %v, not sorted", got)
	}
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s] {
			t.Errorf("RoleSlugs lists %q twice", s)
		}
		seen[s] = true
	}
	for _, lang := range languagesOf(t, KindPrivacy) {
		raw, err := fs.ReadFile(textFS, path.Join("texts", lang, string(KindPrivacy), "meta.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Slug string   `json:"slug"`
			Also []string `json:"also_slugs"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		for _, s := range append([]string{m.Slug}, m.Also...) {
			if !seen[s] {
				t.Errorf("%s: slug %q is not among the role slugs", lang, s)
			}
		}
	}
}
