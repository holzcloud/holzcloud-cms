// Package textdiff compares two texts line by line.
//
// It exists because the alternative was a dependency for sixty lines of
// well-understood algorithm. The one candidate — go-difflib — has not had a
// release since 2016, and a diff is not the place in this program where a
// stale dependency earns its keep: the input is one page of prose, the output
// is read by a person, and nothing about it is going to change.
//
// The algorithm is the usual longest-common-subsequence walk. It is quadratic,
// which is the reason for MaxLines: a page of text is a few hundred lines and
// costs nothing, and something pathological should degrade into an honest
// "these are different" rather than into a slow request.
package textdiff

import "strings"

// MaxLines is the point past which the comparison gives up on being precise.
//
// Two thousand lines is far more than a page of prose ever has. Past it the
// result is still correct — it says the old text went and the new one came —
// it just stops being useful, which is better than being slow.
const MaxLines = 2000

// Kind is what happened to a line.
type Kind string

const (
	// KindContext is a line both texts have.
	KindContext Kind = "ctx"
	// KindAdd is a line only the new text has.
	KindAdd Kind = "add"
	// KindDelete is a line only the old text has.
	KindDelete Kind = "del"
	// KindSkip stands for the run of unchanged lines left out between two
	// changes. Count says how many.
	KindSkip Kind = "skip"
)

// Line is one line of the comparison.
//
// OldNo and NewNo are the line numbers in the respective text, or zero where
// the line does not exist there. They are what makes a diff readable next to
// the actual document.
type Line struct {
	Kind  Kind
	Text  string
	OldNo int
	NewNo int
	// Count is the number of lines a KindSkip stands for. Zero otherwise.
	Count int
}

// Lines compares two texts and returns every line, unchanged ones included.
//
// Use Compact to drop the long stretches in between; keeping them here means
// the caller decides how much context to show rather than this package.
func Lines(oldText, newText string) []Line {
	a := splitLines(oldText)
	b := splitLines(newText)

	if len(a) > MaxLines || len(b) > MaxLines {
		return wholesale(a, b)
	}

	// The table of common-subsequence lengths, built from the end backwards so
	// the walk below can go forwards and read the better choice straight off.
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var out []Line
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, Line{Kind: KindContext, Text: a[i], OldNo: i + 1, NewNo: j + 1})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, Line{Kind: KindDelete, Text: a[i], OldNo: i + 1})
			i++
		default:
			out = append(out, Line{Kind: KindAdd, Text: b[j], NewNo: j + 1})
			j++
		}
	}
	for ; i < len(a); i++ {
		out = append(out, Line{Kind: KindDelete, Text: a[i], OldNo: i + 1})
	}
	for ; j < len(b); j++ {
		out = append(out, Line{Kind: KindAdd, Text: b[j], NewNo: j + 1})
	}
	return out
}

// Compact keeps context lines within reach of a change and replaces the rest
// with a single KindSkip standing for the lines left out.
//
// context is how many unchanged lines stay on each side of a change. A run
// short enough to be worth less than the gap that would replace it is kept
// whole — a "3 unchanged lines" marker between two changes tells the reader
// less than the three lines would.
func Compact(lines []Line, context int) []Line {
	if context < 0 {
		context = 0
	}
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if l.Kind == KindContext {
			continue
		}
		for k := i - context; k <= i+context; k++ {
			if k >= 0 && k < len(lines) {
				keep[k] = true
			}
		}
	}

	var out []Line
	for i := 0; i < len(lines); {
		if keep[i] {
			out = append(out, lines[i])
			i++
			continue
		}
		start := i
		for i < len(lines) && !keep[i] {
			i++
		}
		run := lines[start:i]
		if len(run) <= 2 {
			out = append(out, run...)
			continue
		}
		out = append(out, Line{Kind: KindSkip, Count: len(run)})
	}
	return out
}

// Changed reports whether the two texts differ at all. Cheaper than building
// the comparison, and it is what an empty state wants to ask.
func Changed(oldText, newText string) bool { return oldText != newText }

// wholesale is the answer for inputs too large to compare line by line: the
// old text went, the new one came. Correct, and it says so honestly.
func wholesale(a, b []string) []Line {
	out := make([]Line, 0, len(a)+len(b))
	for i, l := range a {
		out = append(out, Line{Kind: KindDelete, Text: l, OldNo: i + 1})
	}
	for i, l := range b {
		out = append(out, Line{Kind: KindAdd, Text: l, NewNo: i + 1})
	}
	return out
}

// splitLines splits on newlines and drops the empty piece a trailing newline
// leaves behind — otherwise every text ends with a phantom line that shows up
// in the comparison as a change nobody made.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
