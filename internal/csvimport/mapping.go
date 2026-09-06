package csvimport

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/holzcloud/holzcloud-cms/internal/field"
)

// combiningDiaeresis is U+0308, the mark that stands behind a bare letter in a
// decomposed spelling and turns it into an umlaut.
const combiningDiaeresis = '\u0308'

// precomposed maps the three base letters whose diaeresis form field.SlugifyKey
// knows by name onto that single rune.
//
// Written as escapes rather than as literal umlauts, because which of the two
// spellings a literal would carry is exactly the question this table answers,
// and a source file stored in the other normalisation would silently make the
// table map a rune onto itself.
var precomposed = map[rune]rune{
	'a': '\u00E4', // LATIN SMALL LETTER A WITH DIAERESIS
	'o': '\u00F6', // LATIN SMALL LETTER O WITH DIAERESIS
	'u': '\u00FC', // LATIN SMALL LETTER U WITH DIAERESIS
	'A': '\u00C4', // LATIN CAPITAL LETTER A WITH DIAERESIS
	'O': '\u00D6', // LATIN CAPITAL LETTER O WITH DIAERESIS
	'U': '\u00DC', // LATIN CAPITAL LETTER U WITH DIAERESIS
}

// foldHeader turns a column heading into the key a field definition is matched
// against. It is the one place in this phase that folds a heading, and nothing
// else may call field.SlugifyKey on one.
//
// The rule is: settle the combining marks first, then field.SlugifyKey. Never
// the other way round, and never SlugifyKey alone.
//
// Why the marks have to be settled at all. SlugifyKey (field.go:876) matches
// the single rune U+00F6 and writes "oe" for it. A heading typed on macOS, or
// exported by a tool that normalises to NFD, does not carry that rune: it
// carries 'o' followed by U+0308. Handed to SlugifyKey unchanged, the mark is
// not a letter, not a digit and not a separator, so it falls away without a
// word: the heading spelled Gr + U+00F6 + sse folds to "groesse", and the same
// heading spelled Gro + U+0308 + sse folds to "grosse". Two keys for one word,
// and only one of them is the key the very field of that name carries. The
// heading would then fail to match its own field, with nothing anywhere
// reporting it. That is the class of silent failure IMP-06 exists to prevent,
// and no test of the matching finds it unless the test's own input is written
// in the decomposed form.
//
// Why composing and not merely stripping. Stripping the mark is what D-28
// proposed, and measured against SlugifyKey it does not close the hole: the
// two spellings above still fold to "grosse" and to "groesse", two keys for one
// word, and they still do not meet. The mark has to be put back onto its base
// letter for the three vowels SlugifyKey knows.
//
// Every OTHER mark takes its base letter with it. That is not prettier and it
// is not meant to be: SlugifyKey has cases for ä, ö, ü and ß and DROPS every
// other non-ASCII letter whole, so the precomposed U+00E9 in "Café" folds to
// "caf". Keeping the base for the decomposed spelling would fold the very same
// word to "cafe" — two keys for one word, which is verbatim the failure this
// function exists to prevent, moved from ö to é. A field whose label is "Café"
// carries the key "caf", so it is the spelling that agrees with SlugifyKey that
// finds its own field; the VALUE of the key does not matter, only that one word
// yields one key. The same holds for ñ, ç, å, š and the rest.
//
// Header-scoped, and that scope is the point. settleHeaderMarks drops the base
// because its output is handed to SlugifyKey. foldCell (row.go:37) hands its
// output to page.Transliterate instead, which writes é out as "e", so there the
// two spellings already agree and dropping the base would BREAK that agreement:
// the decomposed "Café" would fold to "caf" while the composed one folds to
// "cafe". Two folds because there are two consumers with two answers, and each
// is written against the one it feeds.
//
// And no dependency. golang.org/x/text is an indirect entry in go.mod and stays
// one: internal/template/dates.go:24 records this project deliberately
// declining it once before, for a job of the same size. unicode is in the
// standard library and the whole of this is one loop.
func foldHeader(header string) string {
	return field.SlugifyKey(settleHeaderMarks(header))
}

// The two answers settle gives a combining mark it cannot compose away.
//
// A bool at a call site says nothing; these two names say which consumer the
// fold is written for and why the answers differ. See foldHeader.
const (
	keepMarkedBase = false
	dropMarkedBase = true
)

// settleMarks writes a decomposed spelling out as a composed one, for a CELL.
//
// A combining diaeresis standing behind a, o or u is put back onto its base
// letter, because those three are the ones field.SlugifyKey and
// page.Transliterate know as single runes. Every other combining mark is
// dropped and its base kept, because the caller hands the result to
// page.Transliterate, which writes the precomposed é out as "e": keeping the
// base is what makes the two spellings of one cell agree here.
//
// Separate from foldHeader because a heading is not the only thing an operator
// types that has to be recognised however their editor normalised it: the
// status vocabulary and the janein vocabulary in row.go fold the same way and
// must not spell the rule a second time.
func settleMarks(s string) string { return settle(s, keepMarkedBase) }

// settleHeaderMarks is the same fold for a HEADING, which is handed to
// field.SlugifyKey rather than to page.Transliterate. The one difference is
// what happens to a mark that is not a diaeresis on a, o or u: the base letter
// goes with it. foldHeader carries the argument.
func settleHeaderMarks(s string) string { return settle(s, dropMarkedBase) }

// settle is the one loop both folds are.
func settle(s string, dropMarked bool) string {
	runes := []rune(s)

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if unicode.Is(unicode.Mn, r) {
			// A mark that reaches this point either had no base letter before
			// it or its base was written out unchanged. Either way it carries
			// no letter of its own and SlugifyKey would drop it anyway.
			continue
		}
		if i+1 < len(runes) && unicode.Is(unicode.Mn, runes[i+1]) {
			if runes[i+1] == combiningDiaeresis {
				if composed, ok := precomposed[r]; ok {
					b.WriteRune(composed)
					i++
					continue
				}
			}
			if dropMarked {
				// The base goes with the mark, because the precomposed
				// spelling of this letter is dropped whole by SlugifyKey.
				i++
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// The targets a column may be pointed at.
//
// Six of them plus "nothing", and "nothing" is one of the six rather than the
// absence of a target: IMP-01 asks for a column to be pointable at "explicitly
// nothing", and a target with a name is the difference between a decision and
// an omission on the mapping screen.
const (
	// TargetNone is a column that is imported nowhere.
	TargetNone = "none"
	// TargetTitle is the page's title.
	TargetTitle = "title"
	// TargetSlug is the page's address.
	TargetSlug = "slug"
	// TargetBody is the page's Markdown source.
	TargetBody = "body"
	// TargetStatus is draft or published.
	TargetStatus = "status"
	// TargetTerms are the page's own terms, the analogue of what the WordPress
	// importer does at wordpress.go:147.
	TargetTerms = "terms"
	// TargetField is one of the website's own field definitions, named by Key.
	TargetField = "field"
)

// Target is where one column goes.
type Target struct {
	// Kind is one of the seven constants above.
	Kind string
	// Key is the field's key, and is only read when Kind is TargetField.
	Key string
}

// String is the target as one string, so it can be a map key and a form value.
//
// A field's key cannot collide with one of the fixed targets here because the
// prefix keeps the two apart: a website may define a field whose key is
// literally "title" without taking the page's own title away from it.
func (t Target) String() string {
	if t.Kind == TargetField {
		return TargetField + ":" + t.Key
	}
	return t.Kind
}

// Column is one column of the uploaded file.
//
// It is addressed by its position and never by its heading, and that is the
// whole of D-27. A file with two columns headed "Titel" is an ordinary file —
// a product list with a name in two languages, a sheet somebody widened twice —
// and addressing a column by its heading would silently drop the second or let
// it overwrite the first. Two entries at two positions can be pointed at two
// different targets, which is what an operator looking at such a file expects.
type Column struct {
	// Index is the position in the file, counted from zero, and is the address
	// every other part of this package uses.
	Index int
	// Number is the same position counted from one, which is what an operator
	// counts and what a screen prints.
	Number int
	// Heading is the cell of the header row exactly as the file spelled it,
	// with nothing folded and nothing trimmed away.
	Heading string
}

// HasHeading says whether this column's header cell holds anything at all.
//
// The label a screen shows for a column without one is not minted here.
// Building "Spalte %d" or "Column %d" in Go would put a sentence a person reads
// where tools/i18n cannot see it — its Go collector reads the first argument of
// eight named functions and nothing else — so the fallback wording is a {{tf}}
// literal in the template, fed with Number. Same rule as a row's verdict, for
// the same measured reason (D-32).
func (c Column) HasHeading() bool { return strings.TrimSpace(c.Heading) != "" }

// Columns lists the file's columns, in the file's order, always.
//
// Every column of the header row appears, including one whose heading cell is
// empty: it matches nothing automatically but stays pointable by hand.
func Columns(header []string) []Column {
	out := make([]Column, 0, len(header))
	for i, heading := range header {
		out = append(out, Column{Index: i, Number: i + 1, Heading: heading})
	}
	return out
}

// Mappable says whether a column may be pointed at a field of this kind.
//
// Admitted, and why: text, langtext, code, zahl, bereich, datum and zeit
// because a cell is text and field.Check validates it unchanged; janein
// because the importer parses a closed vocabulary for it, which it must, since
// Check has no case for the kind at all; auswahl and mehrfachauswahl because
// the cell has to match an option exactly and the dry run is what makes that
// survivable; link because checkLink accepts a path or an http(s) address,
// which is what a cell holds; schlagwort because the cell holds a name and the
// importer derives the slug from it.
//
// Refused, and why: bild and verweis, because both store a numeric id that
// exists only in this installation, so a cell naming a file or a page would
// have to be fetched — the third-party fetch this program does not make — or
// guessed, which is worse than refusing; gruppe, because a group is rows and a
// CSV row is flat; abschnitt, because a section holds no value to import into.
//
// A kind this version does not know is refused too. A definition written by a
// newer version of the program would otherwise be fed a cell that nothing here
// can validate.
//
// The kind values are German because they are what stands in
// page_field_defs.art on every website's every field. They are data and not
// identifiers, and translating one here would compare against a string no row
// carries.
func Mappable(kind string) bool {
	switch kind {
	case field.KindImage, field.KindRef, field.KindGroup, field.KindSection:
		return false
	}
	return field.KnownKind(kind)
}

// Note says why a column was left unmapped by the automatic match.
//
// A code plus its arguments and never a finished sentence, for the same
// measured reason a row's verdict is one (D-32): a sentence built here would be
// invisible to tools/i18n, and the mapping screen would be German-only while
// the translation gate reported green.
type Note struct {
	Reason Reason
	Args   []string
}

// fixedSpellings is the closed table of headings that name one of the fixed
// targets.
//
// Closed on purpose. Every spelling in it is a decision somebody took; guessing
// a further one is how a column silently lands on the wrong target, which is
// worse than leaving it unmapped and letting the operator point it themselves.
//
// Both languages, because the program is open source and an operator's
// spreadsheet is written in whatever language they work in. These are headings
// a person types into a spreadsheet — data, not identifiers — so the German
// ones stay German. All of them are already folded, which is why none carries
// an umlaut: field.SlugifyKey has written "schlagwoerter" for one by the time a
// lookup reaches this table.
var fixedSpellings = map[string]string{
	"titel":         TargetTitle,
	"title":         TargetTitle,
	"adresse":       TargetSlug,
	"slug":          TargetSlug,
	"address":       TargetSlug,
	"text":          TargetBody,
	"inhalt":        TargetBody,
	"body":          TargetBody,
	"content":       TargetBody,
	"zustand":       TargetStatus,
	"status":        TargetStatus,
	"state":         TargetStatus,
	"schlagwoerter": TargetTerms,
	"schlagworte":   TargetTerms,
	"tags":          TargetTerms,
}

// Mapping is where every column of one file goes.
type Mapping struct {
	// Targets holds one entry per column, addressed by the column's index.
	Targets []Target
	// Notes holds one entry per column, addressed the same way: why the column
	// was left unmapped, or the zero Note when there is nothing to say.
	Notes []Note
	// Defaults is a default value per TARGET and not per column.
	//
	// Keyed by Target.String(), because IMP-08 gives the default to the field:
	// two columns pointed at one field would otherwise carry two defaults for
	// one slot, and nothing would decide between them.
	Defaults map[string]string
}

// ColumnFor is the index of the column pointed at a target, or -1.
//
// Every consumer asks this question, and none of them should walk the slice
// itself: -1 rather than a zero that would read as the first column is the
// whole reason this is a function.
func (m Mapping) ColumnFor(kind, key string) int {
	for i, t := range m.Targets {
		if t.Kind == kind && (kind != TargetField || t.Key == key) {
			return i
		}
	}
	return -1
}

// AutoMap proposes a mapping for a header row.
//
// The order of the steps is the requirement and not an implementation detail:
//
//  1. Every heading is folded once.
//  2. The fixed targets are matched from their own closed spelling table
//     first, so a website that happens to define a field called "Titel" does
//     not take the title column away from the page's own title.
//  3. Then the field definitions, in the order field.List returned them —
//     ORDER BY position, id, which is the order the operator sees on the field
//     screen. For each definition the columns are walked left to right and the
//     first match takes it, so when two definitions could both take one
//     heading the earlier one wins.
//  4. A definition is matched at most once, and a target is filled at most
//     once. A further column folding to a key that has already been taken is
//     left unmapped and says so, because an unexplained blank on the mapping
//     screen reads as an oversight rather than as a decision.
//  5. A heading that folds to nothing matches nothing and says that too. It
//     stays pointable by hand.
//
// Nothing here is final: every column of the returned mapping is meant to be
// overridden on the screen, and the automatic match is a proposal.
func AutoMap(header []string, defs []field.Def) Mapping {
	m := Mapping{
		Targets:  make([]Target, len(header)),
		Notes:    make([]Note, len(header)),
		Defaults: map[string]string{},
	}

	folded := make([]string, len(header))
	for i, heading := range header {
		folded[i] = foldHeader(heading)
		m.Targets[i] = Target{Kind: TargetNone}
	}

	// winner records which column took each folded key, so the column that
	// arrives second at the same key can name the one that beat it.
	winner := map[string]int{}
	claim := func(column int, t Target) {
		m.Targets[column] = t
		winner[folded[column]] = column
	}

	for i, key := range folded {
		if key == "" {
			continue
		}
		kind, ok := fixedSpellings[key]
		if !ok || m.ColumnFor(kind, "") >= 0 {
			continue
		}
		claim(i, Target{Kind: kind})
	}

	for _, d := range defs {
		if !Mappable(d.Kind) {
			continue
		}
		labelKey := foldHeader(d.Label)
		for i, key := range folded {
			if key == "" || m.Targets[i].Kind != TargetNone {
				continue
			}
			if key != d.Key && key != labelKey {
				continue
			}
			claim(i, Target{Kind: TargetField, Key: d.Key})
			break
		}
	}

	for i, key := range folded {
		if m.Targets[i].Kind != TargetNone {
			continue
		}
		if key == "" {
			m.Notes[i] = Note{Reason: ReasonEmptyHeader}
			continue
		}
		if first, ok := winner[key]; ok {
			m.Notes[i] = Note{
				Reason: ReasonColumnTaken,
				Args:   []string{strconv.Itoa(Columns(header)[first].Number)},
			}
		}
	}

	return m
}
