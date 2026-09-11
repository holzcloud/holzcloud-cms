// What this file guards is the sentence that a column is a position.
//
// A spreadsheet exported from a real business has two columns headed the same
// word more often than not, and it has headings written on a Mac, where an
// umlaut is two code points and not one. Both of those are silent failures:
// the second column vanishes without a word, or a heading fails to match the
// very field it names. Neither shows up in a test of the matching unless the
// test itself is written in the shape that breaks it, which is why the NFD
// input below is spelled as an escape and never as a literal umlaut.
//
// No database and no HTTP anywhere in this file. The mapping needs neither, and
// a test that reached for one would be evidence that the mapping had grown a
// dependency it must not have.
package csvimport

import (
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/field"
)

// def is a field definition as field.List would have returned it.
func def(id int64, position int, key, label, kind string) field.Def {
	return field.Def{ID: id, Position: position, Key: key, Label: label, Kind: kind}
}

// TestFoldHeaderComposesCombiningUmlauts: the same word, written in the two
// ways Unicode allows, folds to the same key. (IMP-06 / encoding, D-28.)
func TestFoldHeaderComposesCombiningUmlauts(t *testing.T) {
	// Written as escapes on purpose. A literal umlaut here would be whatever
	// normalisation this source file happens to be stored in, and the test
	// would then compare a string with itself and pass while proving nothing.
	const nfc = "Gr\u00F6sse"  // LATIN SMALL LETTER O WITH DIAERESIS, one rune
	const nfd = "Gro\u0308sse" // 'o' followed by COMBINING DIAERESIS, two runes

	if nfc == nfd {
		t.Fatal("the two spellings are the same string, so this test proves nothing")
	}
	if got, want := foldHeader(nfd), foldHeader(nfc); got != want {
		t.Errorf("foldHeader(NFD) = %q, foldHeader(NFC) = %q — a heading exported on a Mac would not find its own field", got, want)
	}
	if got := foldHeader(nfc); got != "groesse" {
		t.Errorf("foldHeader(%q) = %q, want %q — the key has to be the one field.SlugifyKey derives from the label", nfc, got, "groesse")
	}
	// The capital, because SlugifyKey lowercases the whole string before it
	// looks at a single rune, so the composition has to happen before that.
	if got := foldHeader("GRO\u0308SSE"); got != "groesse" {
		t.Errorf("foldHeader of the upper-case decomposed form = %q, want %q", got, "groesse")
	}
}

// TestFoldHeaderAgreesOnEveryOtherAccent (WR-01): the hole D-28 exists to
// close, at the accents that are not ä, ö or ü.
//
// One word must yield one key, and which key it is does not matter — only that
// the two spellings Unicode allows meet. They did not: settleMarks composed the
// diaeresis back for a, o and u and dropped every other mark while KEEPING its
// base, so "Café" written NFC folded to "caf" (SlugifyKey dropped the
// precomposed é whole) and the same word written NFD folded to "cafe". A
// spreadsheet exported by anything that normalises to NFD produced a heading
// that did not match its own field and landed unmapped with no note against it.
//
// The wanted keys changed in v2.0, and the property did not. SlugifyKey used to
// carry its own four-entry transliteration list and dropped every other accented
// letter; it now goes through page.Transliterate, which has known the full
// Latin-1 set all along. So the key for "Café" is "cafe" and not "caf" — the
// letter is transliterated instead of lost, in the header fold and in the label
// the field was created from alike. Only NEW field keys move: a key is minted
// once and then stands, which is what keeps a page's values when a label is
// reworded.
func TestFoldHeaderAgreesOnEveryOtherAccent(t *testing.T) {
	// Escapes and not literals: which normalisation a literal would carry is
	// the very question under test.
	for _, c := range []struct{ name, nfc, nfd, want string }{
		{"e acute", "Caf\u00E9", "Cafe\u0301", "cafe"},
		{"n tilde", "A\u00F1o", "An\u0303o", "ano"},
		{"c cedilla", "Fa\u00E7ade", "Fac\u0327ade", "facade"},
		{"a ring", "M\u00E5l", "Ma\u030Al", "mal"},
		{"s caron", "\u0160kola", "S\u030Ckola", "skola"},
	} {
		if c.nfc == c.nfd {
			t.Fatalf("%s: the two spellings are the same string, so this case proves nothing", c.name)
		}
		gotNFC, gotNFD := foldHeader(c.nfc), foldHeader(c.nfd)
		if gotNFC != gotNFD {
			t.Errorf("%s: foldHeader(NFC) = %q, foldHeader(NFD) = %q — two keys for one word", c.name, gotNFC, gotNFD)
		}
		// The key SlugifyKey itself derives from the label, because that is
		// the key the field of that name actually carries.
		if gotNFC != c.want {
			t.Errorf("%s: foldHeader(%q) = %q, want %q — the key field.SlugifyKey derives from the label", c.name, c.nfc, gotNFC, c.want)
		}
	}
}

// TestFoldCellKeepsTheBaseLetter (WR-01, the other consumer): a CELL is folded
// through page.Transliterate, which writes é out as "e", so there the base
// letter has to be KEPT.
//
// Since v2.0 the header fold keeps it too — SlugifyKey transliterates rather
// than drops, so there is nothing left for the two folds to do differently and
// they are one function. The test stays, because the cell path has its own
// consumers (the status vocabulary, the janein vocabulary) and a later change
// that reintroduced a second fold would silently move a whole status column
// outside its own closed vocabulary.
func TestFoldCellKeepsTheBaseLetter(t *testing.T) {
	const nfc = "Caf\u00E9"  // LATIN SMALL LETTER E WITH ACUTE, one rune
	const nfd = "Cafe\u0301" // 'e' followed by COMBINING ACUTE ACCENT, two runes
	if got, want := foldCell(nfd), foldCell(nfc); got != want {
		t.Errorf("foldCell(NFD) = %q, foldCell(NFC) = %q — one cell, two spellings", got, want)
	}
	if got := foldCell(nfc); got != "cafe" {
		t.Errorf("foldCell(%q) = %q, want %q — a cell keeps its letters", nfc, got, "cafe")
	}
}

// TestFoldHeaderIgnoresSpellingAndCase: case, surrounding space and the three
// separators an operator types all fall away. (IMP-06, D-17.)
func TestFoldHeaderIgnoresSpellingAndCase(t *testing.T) {
	for _, in := range []string{"  TITEL  ", "titel", "TiTel", "\tTitel\n"} {
		if got := foldHeader(in); got != "titel" {
			t.Errorf("foldHeader(%q) = %q, want %q", in, got, "titel")
		}
	}
	// The three separators an operator types are interchangeable but they do
	// not fall away: field.SlugifyKey collapses each of them to a single
	// underscore, and a field defined under the label "Ti-tel" carries the key
	// "ti_tel" for exactly the same reason. The two derivations agree, which is
	// what makes the match work; they do not agree on erasing the separator.
	for _, in := range []string{"Ti-tel", "Ti tel", "ti_tel", "Ti - tel"} {
		if got := foldHeader(in); got != "ti_tel" {
			t.Errorf("foldHeader(%q) = %q, want %q", in, got, "ti_tel")
		}
	}
	// A heading that carries no letter at all folds to nothing, and nothing is
	// not a key that anything may match.
	if got := foldHeader("\u2014"); got != "" {
		t.Errorf("foldHeader(em dash) = %q, want the empty string", got)
	}
}

// TestTwoEqualHeadingsDoNotCollide: a file with two columns headed Titel shows
// two entries, and the second neither overwrites the first nor disappears.
// (IMP-01 / adjacency, D-27.)
func TestTwoEqualHeadingsDoNotCollide(t *testing.T) {
	head := []string{"Titel", "Titel"}

	cols := Columns(head)
	if len(cols) != 2 {
		t.Fatalf("Columns returned %d entries for a two-column file, want 2", len(cols))
	}
	if cols[0].Index != 0 || cols[1].Index != 1 {
		t.Errorf("columns are addressed %d and %d, want 0 and 1 — a column is a position", cols[0].Index, cols[1].Index)
	}

	m := AutoMap(head, nil)
	if m.Targets[0].Kind != TargetTitle {
		t.Errorf("first column points at %q, want %q", m.Targets[0].Kind, TargetTitle)
	}
	if m.Targets[1].Kind != TargetNone {
		t.Errorf("second column points at %q, want %q — one target is filled once", m.Targets[1].Kind, TargetNone)
	}
	if got := m.ColumnFor(TargetTitle, ""); got != 0 {
		t.Errorf("ColumnFor(title) = %d, want 0", got)
	}
}

// TestSecondHeadingSaysWhyItStaysEmpty: the loser of a collision carries a
// reason code, so the mapping screen says why in place instead of leaving an
// unexplained blank that reads as an oversight. (IMP-06 / adjacency.)
func TestSecondHeadingSaysWhyItStaysEmpty(t *testing.T) {
	m := AutoMap([]string{"Titel", "Titel"}, nil)

	if m.Notes[0].Reason != "" {
		t.Errorf("the winning column carries the note %q, want none", m.Notes[0].Reason)
	}
	if m.Notes[1].Reason != ReasonColumnTaken {
		t.Fatalf("the second column carries the note %q, want %q", m.Notes[1].Reason, ReasonColumnTaken)
	}
	if len(m.Notes[1].Args) != 1 || m.Notes[1].Args[0] != "1" {
		t.Errorf("the note's arguments are %v, want the winning column's position [1]", m.Notes[1].Args)
	}
}

// TestFirstDefinitionWins: when two definitions could both take one heading,
// the earlier one in field.List's order does. (IMP-06 / ordering.)
func TestFirstDefinitionWins(t *testing.T) {
	defs := []field.Def{
		def(1, 1, "sorte", "Sorte", field.KindText),
		// A second definition whose LABEL folds to the same key as the first
		// one's key. Both could take the heading; the order decides.
		def(2, 2, "sorte_zwei", "sorte", field.KindText),
	}

	m := AutoMap([]string{"Sorte"}, defs)
	if m.Targets[0].Kind != TargetField || m.Targets[0].Key != "sorte" {
		t.Errorf("the heading went to %+v, want the definition at position 1", m.Targets[0])
	}
}

// TestEmptyHeadingStaysMappableByHand: a heading that folds to nothing matches
// nothing automatically, says so, and is still offered.
// (IMP-06 / empty, IMP-01 / empty.)
func TestEmptyHeadingStaysMappableByHand(t *testing.T) {
	head := []string{"\u2014", ""}

	cols := Columns(head)
	if len(cols) != 2 {
		t.Fatalf("Columns dropped a column: %d of 2", len(cols))
	}
	if cols[1].HasHeading() {
		t.Error("the empty heading reports as present")
	}
	if cols[1].Number != 2 {
		t.Errorf("the second column counts as %d, want 2", cols[1].Number)
	}

	m := AutoMap(head, nil)
	for i := range head {
		if m.Targets[i].Kind != TargetNone {
			t.Errorf("column %d matched %q automatically, want nothing", i, m.Targets[i].Kind)
		}
		if m.Notes[i].Reason != ReasonEmptyHeader {
			t.Errorf("column %d carries the note %q, want %q", i, m.Notes[i].Reason, ReasonEmptyHeader)
		}
	}
}

// TestColumnsInFileOrder: always the file's order, never a sorted one.
// (IMP-01 / ordering.)
func TestColumnsInFileOrder(t *testing.T) {
	head := []string{"C", "A", "B"}
	cols := Columns(head)
	for i, want := range head {
		if cols[i].Heading != want {
			t.Errorf("column %d is %q, want %q", i, cols[i].Heading, want)
		}
		if cols[i].Number != i+1 {
			t.Errorf("column %d counts as %d, want %d", i, cols[i].Number, i+1)
		}
	}
}

// TestImageAndRefAreNoTarget: the four refused kinds are refused as a target
// and are not matched automatically either. (D-18.)
func TestImageAndRefAreNoTarget(t *testing.T) {
	for _, kind := range []string{field.KindImage, field.KindRef, field.KindGroup, field.KindSection} {
		if Mappable(kind) {
			t.Errorf("Mappable(%q) = true, want false", kind)
		}
	}
	for _, kind := range []string{
		field.KindText, field.KindLong, field.KindCode, field.KindNumber, field.KindRange,
		field.KindDate, field.KindTime, field.KindBool, field.KindChoice, field.KindMulti,
		field.KindLink, field.KindTerm,
	} {
		if !Mappable(kind) {
			t.Errorf("Mappable(%q) = false, want true", kind)
		}
	}
	// A kind this version does not know is not offered either: a definition
	// written by a newer version must not be fed a cell nothing can validate.
	if Mappable("teleporter") {
		t.Error(`Mappable("teleporter") = true, want false`)
	}

	defs := []field.Def{
		def(1, 1, "bild", "Bild", field.KindImage),
		def(2, 2, "eltern", "Eltern", field.KindRef),
	}
	m := AutoMap([]string{"Bild", "Eltern"}, defs)
	for i := range m.Targets {
		if m.Targets[i].Kind != TargetNone {
			t.Errorf("column %d was matched to %+v, want nothing at all", i, m.Targets[i])
		}
	}
}

// TestWithoutFieldsTheFourFixedTargetsRemain: a website that has defined no
// field of its own still imports. (IMP-04 / empty.)
func TestWithoutFieldsTheFourFixedTargetsRemain(t *testing.T) {
	// The last heading is written with an escape for the umlaut so this file
	// stays free of them; it is a heading an operator types, not an identifier.
	head := []string{"Titel", "Adresse", "Text", "Zustand", "Schlagw\u00F6rter"}
	m := AutoMap(head, nil)

	want := []string{TargetTitle, TargetSlug, TargetBody, TargetStatus, TargetTerms}
	for i, w := range want {
		if m.Targets[i].Kind != w {
			t.Errorf("column %d (%q) points at %q, want %q", i, head[i], m.Targets[i].Kind, w)
		}
		if m.ColumnFor(w, "") != i {
			t.Errorf("ColumnFor(%q) = %d, want %d", w, m.ColumnFor(w, ""), i)
		}
	}

	// The English spellings of the same five, because an operator's spreadsheet
	// is written in whatever language they work in.
	english := []string{"Title", "Slug", "Body", "State", "Tags"}
	me := AutoMap(english, nil)
	for i, w := range want {
		if me.Targets[i].Kind != w {
			t.Errorf("column %d (%q) points at %q, want %q", i, english[i], me.Targets[i].Kind, w)
		}
	}

	// A target nothing points at answers -1 rather than a position that would
	// read as column one.
	if got := AutoMap([]string{"Titel"}, nil).ColumnFor(TargetBody, ""); got != -1 {
		t.Errorf("ColumnFor of an unmapped target = %d, want -1", got)
	}
}
