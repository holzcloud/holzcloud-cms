package privacy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// The legal pages are composed, not written: one structure in Go, the words in
// files.
//
// The STRUCTURE is Go — which blocks a page has, in which order, and under
// which condition each one appears. It is written once for every language, so
// a translation can change how a sentence reads but never drop a block, swap
// two sections or show the shop paragraph on a website without a shop.
//
// The WORDS are data under texts/<lang>/<kind>/, one Markdown file per block
// and a meta.json for titles, slugs and table cells. That is not a matter of
// taste. tools/english reads Go source and fails the build on German in it,
// and French or Romansh prose would trip it just the same, because short
// French words are on its list of German ones; prose in embedded files is outside its reach by design,
// and a later plan adds a language by adding files, without touching Go.
//
// The INPUTS are booleans, the framework set and the language, and nothing
// else. Phase 30 held that a composed template carries no operator string, no
// link and no raw HTML, because the operator publishes it after editing and
// the page renders like any other; a field for, say, the company name would be
// the first way Markdown typed by somebody could reach it. Request has no such
// field, and a test fails if one is added.

// Kind is one sort of legal page.
type Kind string

// The kinds of legal page this package can compose.
const (
	KindPrivacy Kind = "privacy"
	KindImprint Kind = "imprint"
	KindDNS     Kind = "dns"
)

// Request is everything a composed page depends on.
type Request struct {
	// Lang is the language tag of the page: "de", "de-CH", "fr", "rm", "es".
	// TextLang decides which texts it gets.
	Lang string
	// Swiss asks for Swiss German spelling (D-15). It changes nothing for a
	// language other than German.
	Swiss bool
	// Set is the website's choice of legal frameworks.
	Set legal.Set
	// Features is what the website uses.
	Features Features
}

// block is one part of a page. on decides whether it appears; nil means
// always.
type block struct {
	name string
	on   func(Request) bool
}

// kindSpec is the structure of one kind of page.
type kindSpec struct {
	kind   Kind
	blocks []block
	// minFillins is how many fill-in markers every composition must carry at
	// least. A template with fewer than that looks finished when it is not.
	minFillins int
	// vars returns the placeholders of this kind for the text language and
	// the request. Their values may use the shared placeholders. It may be
	// nil.
	vars func(lang string, r Request) map[string]string
}

// registry holds every kind, registered from an init function in the kind's
// own file.
var registry = map[Kind]kindSpec{}

// register adds a kind. A kind registered twice is a programming error.
func register(s kindSpec) {
	if _, dup := registry[s.kind]; dup {
		panic(fmt.Sprintf("privacy: kind %q registered twice", s.kind))
	}
	registry[s.kind] = s
}

// Kinds lists the registered kinds, sorted.
func Kinds() []Kind {
	out := make([]Kind, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// compose writes the Markdown of one kind of page.
//
// The blocks whose condition holds are written in the registered order, one
// blank line between two of them, and the placeholders are expanded. A block
// whose file is missing in the language is skipped; the completeness test is
// what makes sure there is none.
func compose(k Kind, r Request) string {
	spec, ok := registry[k]
	if !ok {
		return ""
	}
	r.Features = r.Features.normalised()
	lang := TextLang(k, r.Lang)
	kt := texts().kinds[k][lang]
	if kt == nil {
		return ""
	}

	var parts []string
	for _, b := range spec.blocks {
		if b.on != nil && !b.on(r) {
			continue
		}
		if s, ok := kt.blocks[b.name]; ok {
			parts = append(parts, s)
		}
	}
	text := strings.Join(parts, "\n\n") + "\n"

	shared := sharedVars(lang)
	vars := map[string]string{}
	if spec.vars != nil {
		sharedReplacer := replacerOf(shared)
		for key, v := range spec.vars(lang, r) {
			vars[key] = sharedReplacer.Replace(v)
		}
	}
	for key, v := range shared {
		vars[key] = v
	}
	text = replacerOf(vars).Replace(text)
	if lang == "de" && r.Swiss {
		text = swissSpelling.Replace(text)
	}
	return text
}

// swissSpelling turns German into Swiss German spelling (D-15): ss for the
// sharp s, and guillemets for the low-high quotation marks. It is the rule
// tools/i18n applies to the de-CH catalogue, and internal/i18n/tag.go says
// why: a German text in the German spelling looks foreign on a Swiss
// website. The letters are written as escapes because tools/english reads
// this file.
var swissSpelling = strings.NewReplacer("\u00df", "ss", "\u201e", "\u00ab", "\u201c", "\u00bb")

// sharedVars are the placeholders every kind can use.
func sharedVars(lang string) map[string]string {
	w := texts().langs[lang]
	machine := ""
	if w.MachineNotice != "" {
		machine = " **" + w.MachineNotice + "**"
	}
	return map[string]string{
		"fillin":          w.Fillin,
		"notice":          w.TemplateNotice,
		"machine_notice":  machine,
		"no_legal_advice": w.NoLegalAdvice,
	}
}

// replacerOf turns placeholders into one replacer, built from the sorted keys
// so the same map always gives the same replacer.
func replacerOf(vars map[string]string) *strings.Replacer {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, 2*len(keys))
	for _, k := range keys {
		pairs = append(pairs, "{"+k+"}", vars[k])
	}
	return strings.NewReplacer(pairs...)
}
