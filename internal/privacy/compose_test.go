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
