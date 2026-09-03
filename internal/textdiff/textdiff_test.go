package textdiff_test

import (
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/textdiff"
)

func render(lines []textdiff.Line) string {
	var b strings.Builder
	for _, l := range lines {
		switch l.Kind {
		case textdiff.KindAdd:
			b.WriteString("+" + l.Text + "\n")
		case textdiff.KindDelete:
			b.WriteString("-" + l.Text + "\n")
		case textdiff.KindContext:
			b.WriteString(" " + l.Text + "\n")
		case textdiff.KindSkip:
			b.WriteString("…\n")
		}
	}
	return b.String()
}

func TestIdenticalTextsHaveNoChangedLines(t *testing.T) {
	lines := textdiff.Lines("eins\nzwei\ndrei\n", "eins\nzwei\ndrei\n")
	for _, l := range lines {
		if l.Kind != textdiff.KindContext {
			t.Fatalf("unveränderter Text ergibt %q: %s", l.Kind, l.Text)
		}
	}
	if len(lines) != 3 {
		t.Fatalf("3 Zeilen erwartet, %d bekommen", len(lines))
	}
}

func TestChangedLineShowsAsDeleteThenAdd(t *testing.T) {
	got := render(textdiff.Lines("eins\nzwei\ndrei\n", "eins\nZWEI\ndrei\n"))
	want := " eins\n-zwei\n+ZWEI\n drei\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestInsertionKeepsSurroundingLinesAsContext(t *testing.T) {
	got := render(textdiff.Lines("eins\ndrei\n", "eins\nzwei\ndrei\n"))
	want := " eins\n+zwei\n drei\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Eine leere Seite gegen eine volle: alles ist neu, nichts ist gelöscht.
func TestEmptyOldTextIsAllAdditions(t *testing.T) {
	lines := textdiff.Lines("", "eins\nzwei\n")
	if len(lines) != 2 {
		t.Fatalf("2 Zeilen erwartet, %d bekommen", len(lines))
	}
	for _, l := range lines {
		if l.Kind != textdiff.KindAdd {
			t.Fatalf("%q statt add", l.Kind)
		}
	}
}

// Der abschliessende Zeilenumbruch darf keine Geisterzeile erzeugen — sonst
// meldet jeder Vergleich eine Änderung, die niemand gemacht hat.
func TestTrailingNewlineIsNotALine(t *testing.T) {
	withNewline := textdiff.Lines("eins\n", "eins\n")
	without := textdiff.Lines("eins", "eins")
	if len(withNewline) != 1 || len(without) != 1 {
		t.Fatalf("je 1 Zeile erwartet, %d und %d bekommen", len(withNewline), len(without))
	}
}

func TestLineNumbersFollowBothTexts(t *testing.T) {
	lines := textdiff.Lines("a\nb\n", "a\nx\nb\n")
	for _, l := range lines {
		switch {
		case l.Kind == textdiff.KindAdd && l.OldNo != 0:
			t.Fatalf("neue Zeile trägt eine alte Nummer: %+v", l)
		case l.Kind == textdiff.KindDelete && l.NewNo != 0:
			t.Fatalf("gelöschte Zeile trägt eine neue Nummer: %+v", l)
		}
	}
	last := lines[len(lines)-1]
	if last.OldNo != 2 || last.NewNo != 3 {
		t.Fatalf("letzte Zeile: alt %d neu %d, erwartet 2 und 3", last.OldNo, last.NewNo)
	}
}

func TestCompactReplacesLongUnchangedRunsWithOneMarker(t *testing.T) {
	var oldB, newB strings.Builder
	for i := 0; i < 30; i++ {
		oldB.WriteString("gleich\n")
		newB.WriteString("gleich\n")
	}
	oldB.WriteString("alt\n")
	newB.WriteString("neu\n")

	compact := textdiff.Compact(textdiff.Lines(oldB.String(), newB.String()), 3)
	var skips, contexts int
	for _, l := range compact {
		switch l.Kind {
		case textdiff.KindSkip:
			skips++
			if l.Count != 27 {
				t.Fatalf("Marke steht für %d Zeilen, 27 erwartet", l.Count)
			}
		case textdiff.KindContext:
			contexts++
		}
	}
	if skips != 1 {
		t.Fatalf("%d Marken, 1 erwartet", skips)
	}
	if contexts != 3 {
		t.Fatalf("%d Kontextzeilen, 3 erwartet", contexts)
	}
}

// Eine kurze unveränderte Strecke bleibt stehen: "2 Zeilen ausgelassen" sagt
// weniger als die zwei Zeilen selbst.
func TestCompactKeepsRunsShorterThanTheMarker(t *testing.T) {
	oldText := "a\ngleich\ngleich\nb\n"
	newText := "A\ngleich\ngleich\nB\n"
	for _, l := range textdiff.Compact(textdiff.Lines(oldText, newText), 0) {
		if l.Kind == textdiff.KindSkip {
			t.Fatalf("kurze Strecke wurde durch eine Marke ersetzt")
		}
	}
}

func TestOversizedInputDegradesToWholesaleReplacement(t *testing.T) {
	var big strings.Builder
	for i := 0; i < textdiff.MaxLines+1; i++ {
		big.WriteString("zeile\n")
	}
	for _, l := range textdiff.Lines(big.String(), "kurz\n") {
		if l.Kind == textdiff.KindContext {
			t.Fatalf("übergrosse Eingabe liefert Kontextzeilen statt einer klaren Ersetzung")
		}
	}
}
